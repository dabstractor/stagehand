---
name: "P1.M4.T2.S1 — Rewrite `config init` to populated bootstrap + add --provider/--force/--template flags (PRD §9.17 FR-B1/B2, §9.16 FR-D1/D4, §16.4)"
description: |

  Rewrite `stagecoach config init` (internal/cmd/config.go) so its DEFAULT behavior writes a POPULATED,
  working config (not the inert commented template). It runs cascading detection (FR-D1: highest-priority
  installed built-in in order pi, opencode, cursor, agy, gemini, codex, claude), writes `[defaults]
  provider = <detected>` UNCOMMENTED, writes that provider's `[role.*]` per-role default models
  (FR-D4 table from P1.M3.T3.S1's DefaultModelsForProvider) UNCOMMENTED so the tool works immediately,
  applies the stager fallback (FR-D4: a provider whose manifest has nil TooledFlags cannot serve as the
  stager → route [role.stager] to the first stager-capable provider, annotated), and writes other
  INSTALLED providers as COMMENTED-OUT `[role.*]` blocks (switching agents = one-line uncomment). Parent
  dirs are created; an existing file is NOT overwritten unless `--force`; the written path is always
  printed. `config_version = CurrentConfigVersion` is written UNCOMMENTED.

  Three new LOCAL flags on `configInitCmd`: `--provider <name>` (target a specific provider instead of
  auto-detecting; validated against the built-in registry), `--force` (overwrite an existing file),
  `--template` (retain the v1 all-commented inert-template behavior — writes exampleConfigTemplate).

  CONTRACT (PRD §9.17 FR-B1/B2):
    - `config init`              → populated bootstrap (auto-detect default provider + its role models).
    - `config init --provider X` → populated bootstrap pinned to provider X (validated built-in).
    - `config init --force`      → overwrite an existing global config (both populated and --template).
    - `config init --template`   → the v1 inert exampleConfigTemplate (all-commented; the Mode-A reference).
    - Parent dirs created (MkdirAll); refuse overwrite unless --force (keep the existing "already exists"
      message); the written path is ALWAYS printed on success.

  INPUT (upstream — all EXIST, READ/CONSUME only, do NOT modify):
    - `config.DefaultModelsForProvider(name) map[string]string` — P1.M3.T3.S1 (internal/config/role_defaults.go).
      Returns the role→model column (COPY) or nil for an unknown name. A `stager == ""` value means the
      provider is NOT stager-capable (nil TooledFlags) → apply the FR-D4 fallback.
    - `provider.NewRegistry(nil) / reg.List() / reg.IsInstalled(m) / reg.Get(name) / reg.DefaultProvider(
      installed)` + `preferredBuiltins` order — P1.M2.T3 (internal/provider/registry.go). The FR-D1 cascade.
    - `config.CurrentConfigVersion = 2` (const) — P1.M3.T1.S1 (internal/config/config.go). Written
      UNCOMMENTED in the populated output (read-only; do NOT touch the const or Defaults()).
    - `config.GlobalConfigPath()` — P1.M1.T4.S2 (internal/config/file.go). The write target.
    - `exampleConfigTemplate` (this file) — the v1 inert template; P1.M4.T1.S1 added a config_version
      header block to it. KEEP it as the `--template` output (do NOT remove/rewrite the const).

  PARALLEL-EXECUTION NOTE: P1.M4.T1.S1 (config_version advisory) is implemented FIRST. Assume its outputs
  exist: `Defaults().ConfigVersion == 0`, the load.go advisory, and the `exampleConfigTemplate`
  config_version header. This subtask CONSUMES those read-only (CurrentConfigVersion const; the template
  const) and restructures ONLY runConfigInit + adds flags + the builder. It does NOT touch config.go/
  file.go/load.go (owned by T1.S1). Both edit internal/cmd/config.go — sequencing (T1 before T2) resolves
  the overlap; KEEP exampleConfigTemplate intact so T1.S1's header lands in the --template output.

  DELIVERABLES (2 files EDITED, 1 file EDITED for docs):
    EDIT internal/cmd/config.go        — +3 local flags on configInitCmd; rewrite runConfigInit
      (--template → inert; else populated); +pure buildBootstrapConfig(target, installed) string builder;
      +installedNames(reg) + stagerFallback() helpers; +provider import; updated configInitCmd Long/Short
      (Mode A: populated bootstrap + 3 flags + cascading detection). KEEP exampleConfigTemplate + runConfigPath.
    EDIT internal/cmd/config_test.go   — update tests for populated-default + --template/--force/--provider;
      add buildBootstrapConfig unit tests (deterministic) + TOML-validity check.
    EDIT docs/configuration.md         — document the populated config format + the 3 flags (Mode A).

  SCOPE BOUNDARY (owned by siblings — do NOT implement/edit):
    - `config upgrade` (P1.M4.T3), first-run auto-bootstrap (P1.M4.T4), FR-B6 help de-dup (P4.M2.T2.S1 —
      do NOT remove configCmd's "Subcommands:" block; only update the init line).
    - Defaults()/CurrentConfigVersion/load.go advisory (P1.M4.T1.S1 — consume read-only).
    - root.go, providers.go, default_action.go (P1.M4.T1/P4.M1 — do NOT edit).
    - role_defaults.go / RoleModelDefaults / DefaultModelsForProvider (P1.M3.T3.S1 — consume read-only).

  Deliverable: `stagecoach config init` writes a working config (auto-detected provider + role models);
  `--provider`/`--force`/`--template` work; `go build ./... && go test ./...` green; go.mod/go.sum
  unchanged; only the 3 listed files differ.

---

## Goal

**Feature Goal**: Make `stagecoach config init` produce a **populated, immediately-working** global config
(PRD §9.17 FR-B1) by running cascading provider detection (FR-D1) and writing the detected (or
`--provider`-pinned) provider's per-role default models (FR-D4) UNCOMMENTED, with the stager fallback
applied, other installed providers as commented `[role.*]` blocks, `config_version` uncommented, parent
dirs created, overwrite refused unless `--force`. Add `--provider`/`--force`/`--template` local flags,
retaining the v1 inert template behind `--template`.

**Deliverable** (2 source/test edits + 1 docs edit):
1. EDIT `internal/cmd/config.go` — (a) `--provider`/`--force`/`--template` LOCAL flags on `configInitCmd`;
   (b) rewrite `runConfigInit` to branch: `--template` → write `exampleConfigTemplate`; else populated
   bootstrap (build registry from built-ins → detect installed → resolve target → `buildBootstrapConfig` →
   write); (c) add pure `buildBootstrapConfig(target string, installed []string) string`; (d) add
   `installedNames(reg *provider.Registry) []string` + `stagerFallback() (name, model string)` helpers;
   (e) update `configInitCmd` Short/Long + `configCmd` init line (Mode A). KEEP `exampleConfigTemplate`,
   `runConfigPath`, `configPathCmd`, the `init()` registration, and the existing imports.
2. EDIT `internal/cmd/config_test.go` — update existing init tests for populated-default + the new flags;
   add deterministic `buildBootstrapConfig` unit tests + a TOML-validity check.
3. EDIT `docs/configuration.md` — document the populated format + the 3 flags.

**Success Definition**: `make build && ./bin/stagecoach config init` (no flags, in a fresh HOME) writes a
config whose `config_version = 2`, `[defaults] provider = <an installed built-in or "pi">`, and four
uncommented `[role.*]` blocks with models from the FR-D4 table (stager routed to a stager-capable
provider with an annotation when the default provider can't stage); re-running `config init` exits 1
"already exists … (not overwritten)"; `config init --force` overwrites; `config init --provider claude`
writes a claude-pinned populated config; `config init --template` writes the inert `exampleConfigTemplate`;
`config init --provider bogus` exits 1 ("unknown provider"). The written populated file is VALID TOML.
`go test -race ./internal/cmd/` green; `go test ./...` no regression; `go vet ./...` clean; only the 3
listed files changed.

## User Persona

**Target User**: A new Stagecoach user running `config init` for the first time (PRD §7 personas). They
want a config that **works immediately** — not a 200-line commented reference they have to study before
the tool does anything.

**Use Case**: `stagecoach config init` → stagecoach detects their installed agent (e.g. claude) → writes a
config with `[defaults] provider = "claude"` + claude's role models → next `stagecoach` run "just works."
Later: `config init --provider pi --force` to repin; or `config init --template` to get the full reference.

**User Journey**: install stagecoach → `stagecoach config init` → sees "Wrote config to ~/.config/.../
config.toml" → opens it → sees their agent + sensible per-role models already filled in → runs `stagecoach`
→ works. If they have TWO agents installed, the other appears as a commented block → uncomment one
`[role.*]` to route that role to it.

**Pain Points Addressed**: (1) "config init gave me a wall of comments and the tool still isn't
configured" — solved by populated defaults; (2) "which model for which role?" — solved by the FR-D4 table
materialized per provider; (3) "I picked the wrong agent" — solved by `--provider`/`--force` and the
commented alternatives.

## Why

- **Closes PRD §9.17 FR-B1/B2 (P0).** The v1 `config init` wrote an inert all-commented template —
  functional as documentation but not as a working config. FR-B1 mandates a populated bootstrap so the
  tool is "never unconfigured" and works immediately.
- **Materializes the FR-D4 per-provider × per-role table.** Users get concrete, tier-appropriate models
  (planner=flagship, message=fast, etc.) per their detected agent — the single highest-leverage config
  decision (which model for which job) made for them, editably.
- **Reuses frozen upstream (zero new domain logic).** Detection = the registry (P1.M2.T3); role models =
  DefaultModelsForProvider (P1.M3.T3.S1); schema version = CurrentConfigVersion (P1.M3.T1.S1). This task
  is the WIRING + a string builder.
- **Back-compatible.** `--template` retains the exact v1 behavior; the populated path only ADDS uncommented
  values a user could have written by hand.

## What

`configInitCmd` gains three LOCAL flags: `--provider <string>` (default `""`), `--force` (bool, default
false), `--template` (bool, default false). `runConfigInit`:
1. Resolve `path := config.GlobalConfigPath()`. Existence check: if the file exists AND NOT `--force` →
   return the existing `exitcode.New(exitcode.Error, "config file already exists at %s (not overwritten)")`
   (exit 1). (Stat-error handling unchanged.)
2. `MkdirAll(filepath.Dir(path), 0o755)`.
3. Determine the bytes to write:
   - `--template` → `content = exampleConfigTemplate` (the inert reference; unchanged).
   - else → build registry `reg := provider.NewRegistry(nil)`; `installed := installedNames(reg)`;
     resolve `target` (`--provider` validated via `reg.Get` else `reg.DefaultProvider(installed)` else
     `"pi"` fallback); `content = buildBootstrapConfig(target, installed)`.
4. `WriteFile(path, []byte(content), 0o644)`.
5. Print the path: `--template` → "Wrote example config to %s"; else → "Wrote config to %s".

`buildBootstrapConfig(target string, installed []string) string` (PURE) returns the populated TOML:
header (precedence/env/git-key/flag docs — reused from the template's header text), `config_version =
CurrentConfigVersion` (uncommented), `[defaults]` with `provider = "<target>"` uncommented (others
commented), the four `[role.*]` blocks for the target UNCOMMENTED (models from
`DefaultModelsForProvider(target)`; `[role.stager]` routed to the stager fallback when target's stager is
`""`, with an annotation), then each OTHER installed provider as a commented `[role.*]` block group, then
a commented `[generation]` defaults section.

### Success Criteria

- [ ] `configInitCmd` defines LOCAL flags `--provider` (string), `--force` (bool), `--template` (bool)
      via `configInitCmd.Flags()`; they do NOT appear on `config path` or the root.
- [ ] `runConfigInit`: refuse-overwrite unless `--force` (keep the exact "already exists" message);
      `MkdirAll` parent; `--template` writes `exampleConfigTemplate`; else writes
      `buildBootstrapConfig(target, installed)`; prints the path on success; never `os.Exit`.
- [ ] `--provider <name>`: validated against `provider.NewRegistry(nil).Get(name)`; unknown →
      `exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q …))` (exit 1).
- [ ] Auto-detect: `target = reg.DefaultProvider(installedNames(reg))`; if `""` (nothing installed) →
      `"pi"` with a header comment noting the fallback.
- [ ] `buildBootstrapConfig(target, installed)` is PURE (no I/O, no detection) and returns VALID TOML
      containing: `config_version = CurrentConfigVersion` (uncommented), `[defaults] provider = "<target>"`
      (uncommented), four `[role.*]` blocks with models from `DefaultModelsForProvider(target)` (stager
      routed to the fallback when target stager is `""`, annotated), and a commented `[role.*]` group for
      each `installed` provider != target.
- [ ] `buildBootstrapConfig`'s output parses as valid TOML (`toml.Unmarshal` into `map[string]any` succeeds).
- [ ] `configInitCmd.Long` (Mode A) describes the populated bootstrap, the three flags, and cascading
      detection; `configCmd`'s init line updated (FR-B6 help de-dup NOT done — owned by P4.M2.T2.S1).
- [ ] `docs/configuration.md` documents the populated format + the 3 flags.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; go.mod/go.sum unchanged;
      only `internal/cmd/config.go`, `internal/cmd/config_test.go`, `docs/configuration.md` differ.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact flag additions +
runConfigInit branch logic (skeletons below), the pure `buildBootstrapConfig` signature + output format
(skeleton below), the upstream signatures (all quoted + in research/design-decisions.md F1–F9), the
provider-detection pattern to mirror (providers.go's `installedNames`), the FR-D4 stager-fallback rule,
the test strategy (deterministic via `--provider`/direct builder calls; validity-only for auto-detect),
and the docs target. No git/prompt/decompose knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/002_a17bb6c8dc1d/P1M4T2S1/research/design-decisions.md
  why: the 9 decisions + 9 findings. F1 (config init skips config load → Config() is nil → build registry
       from built-ins only), F2 (the pure builder vs detection+I/O split — the testability crux),
       F3 (DefaultModelsForProvider + the stager "" sentinel + FR-D4 fallback), F4 (target resolution
       --provider→detect→"pi" fallback), F5 (KEEP exampleConfigTemplate = --template output; carries
       T1.S1's header), F6 (config_version UNCOMMENTED = CurrentConfigVersion), F7 (cmd→provider already
       a dep, no cycle), F8 (--force for both paths; keep "already exists" message), F9 (Mode-A help +
       docs; do NOT do FR-B6 de-dup).
  critical: F2 (buildBootstrapConfig is PURE — test deterministically), F1 (NewRegistry(nil)), F3 (stager
       fallback), F4 (the "pi" no-install fallback).

# MUST READ — the file being rewritten
- file: internal/cmd/config.go   (EDIT — rewrite runConfigInit + add flags/builder/helpers; KEEP template+path)
  section: `runConfigInit` (writes exampleConfigTemplate, refuses overwrite) — the function to rewrite.
       `exampleConfigTemplate` const (the inert template, incl. T1.S1's config_version header) — KEEP as
       the --template output. `configInitCmd`/`configCmd` cobra.Command defs. `init()` registration.
  why: this is THE file. The rewrite restructures runConfigInit into (--template | populated) branches and
       adds the builder + helpers + flags. KEEP runConfigPath, configPathCmd, exampleConfigTemplate, init().
  pattern: the existing refuse-overwrite (os.Stat → "already exists") + MkdirAll + WriteFile(0o644) +
       exitcode.New routing is REUSED (only the content source + the --force bypass change).
  gotcha: do NOT remove exampleConfigTemplate (T1.S1's header lives there; it's the --template output).
       Do NOT touch root.go (flags are LOCAL on configInitCmd via configInitCmd.Flags(), not persistent).
       Do NOT remove configCmd's "Subcommands:" block (FR-B6 de-dup is P4.M2.T2.S1).

# MUST READ — the detection pattern to mirror (read providers.go; do NOT edit)
- file: internal/cmd/providers.go   (READ — mirror installedNames + the registry build)
  section: `installedNames(reg *provider.Registry) []string` (loops reg.List(), appends m.Name if
       reg.IsInstalled(m)) and the `newRegistry()` helper (DecodeUserOverrides → NewRegistry). resolvedDefault
       is NOT used (config init has no loaded cfg).
  why: the populated bootstrap needs the SAME detection (IsInstalled over List()) to find installed
       built-ins. Re-implement the 5-line installedNames loop locally in config.go (it's unexported in
       providers.go; do NOT export it — local copy keeps the change self-contained).
  pattern: `reg := provider.NewRegistry(nil)` (nil overrides — Config() is nil in init, F1); then
       `for _, m := range reg.List() { if reg.IsInstalled(m) { installed = append(installed, m.Name) } }`.
  gotcha: do NOT call providers.go's newRegistry() (it reads Config().Providers — nil here; misleading).
       Build with NewRegistry(nil) directly. Detection is the ONLY exec.LookPath in this task.

# MUST READ — the registry API (consume; do NOT edit)
- file: internal/provider/registry.go   (READ)
  section: `preferredBuiltins = []string{"pi","opencode","cursor","agy","gemini","codex","claude"}` (the
       FR-D1 order — use for the no-install "pi" fallback + stager-fallback scan). `NewRegistry(overrides)`,
       `reg.Get(name) (Manifest, bool)`, `reg.List() []Manifest`, `reg.IsInstalled(m) bool`,
       `reg.DefaultProvider(installed) string` (returns "" if none of preferredBuiltins is in installed).
  why: the populated bootstrap's detection + target resolution + --provider validation all use these.
       DefaultProvider is the FR-D1 cascade (pure, takes the installed list).
  gotcha: DefaultProvider returns "" when nothing is installed → apply the "pi" fallback (F4). reg.Get
       validates --provider. preferredBuiltins is the stager-fallback scan order (F3).

# MUST READ — the FR-D4 role-model table (consume; do NOT edit)
- file: internal/config/role_defaults.go   (READ)
  section: `func DefaultModelsForProvider(name string) map[string]string` — returns a COPY of the
       role→model column (planner/stager/message/arbiter → model) or nil for an unknown name. A `stager`
       value of "" means NOT stager-capable (nil TooledFlags). The `roleDefaults` var + the FR-D5
       re-verification mandate block.
  why: THIS supplies the per-role models written into the [role.*] blocks. The bootstrap writes the
       target provider's column UNCOMMENTED and other providers' columns COMMENTED.
  gotcha: stager=="" ⇒ apply the fallback (F3): scan preferredBuiltins for the first with non-empty
       stager; always resolves to "pi" today. The returned map is a copy — safe to read. nil ⇒ unknown
       provider (only happens if --provider validation was skipped — it isn't; validate first).

# MUST READ — the schema-version const (consume read-only; do NOT edit)
- file: internal/config/config.go   (READ CurrentConfigVersion; do NOT edit)
  section: `const CurrentConfigVersion = 2`. (P1.M4.T1.S1 changes Defaults().ConfigVersion → 0; the CONST
       is unchanged at 2.)
  why: the populated config writes `config_version = CurrentConfigVersion` UNCOMMENTED (F6) so the
       T1.S1 load-time advisory is SILENT for a freshly-bootstrapped config. Use the CONST, not Defaults().
  gotcha: do NOT read Defaults().ConfigVersion (it's 0 after T1.S1; meaningless here). Use the const.

# MUST READ — the global path resolver (consume; do NOT edit)
- file: internal/config/file.go   (READ GlobalConfigPath)
  section: `func GlobalConfigPath() string` (the exported wrapper over globalConfigPath — XDG or
       ~/.config/stagecoach/config.toml). Tests set HOME=XDG_CONFIG_HOME=t.TempDir() so the path is
       <tmp>/stagecoach/config.toml (globalDir = <tmp>/stagecoach).
  why: the write target + the always-printed path. Already used by runConfigPath (unchanged).

# MUST READ — the config tests + helpers (EDIT; reuse root_test.go helpers)
- file: internal/cmd/config_test.go   (EDIT — update for populated default + flags; add builder tests)
  section: `setupNoRepo(t)` (isolates HOME/XDG, chdir to a plain dir, returns globalDir). The existing
       `TestConfigInit_*` tests (WritesTemplate/TemplateIsInert/RefusesOverwrite/MkdirAllParent/
       WorksOutsideGitRepo/ExtraArgsExits1). `saveRootState`/`restoreRootState` (rootCmd singleton hygiene).
  why: these tests assert the v1 behavior (default init writes exampleConfigTemplate). After the rewrite
       the DEFAULT writes the POPULATED config — so WritesTemplate/TemplateIsInert move to the `--template`
       path; RefusesOverwrite/MkdirAllParent/WorksOutsideGitRepo stay (now test populated default);
       add --force overwrite + --provider pinning + --provider-bogus error + buildBootstrapConfig unit
       tests + a TOML-validity check.
  pattern: drive rootCmd via SetArgs + Execute(context.Background()); assert via exitcode.For(err) +
       os.ReadFile(config.GlobalConfigPath()). For buildBootstrapConfig unit tests, call it DIRECTLY
       (same package) with fixed args — no Execute, no $PATH dependence.

# MUST READ — the DOCS target (Mode A)
- file: docs/configuration.md   (EDIT — document populated format + 3 flags)
  section: "Config file paths" (says "Written by `stagecoach config init`") + "File format" (says "Every
       line in the `config init` template is commented out"). Update both: `config init` now writes a
       POPULATED config by default; `--template` writes the inert reference; add `--provider`/`--force`.
  why: the work item's DOCS (Mode A) requirement. The current docs describe ONLY the v1 inert behavior.

- url: (PRD §9.17 FR-B1/B2 + §9.16 FR-D1/D4 + §16.4 — in context as selected_prd_content h3.33/h2.9;
       ALSO plan/002_a17bb6c8dc1d/prd_snapshot.md §9.16/§9.17/§16.4)
  why: FR-B1/B2 is the AUTHORITATIVE bootstrap contract (populated, cascading, role models uncommented,
       others commented, --force/--template/--provider). FR-D1 is the cascade order; FR-D4 is the table +
       stager fallback. §16.4 is the [role.*] semantics.
  critical: FR-B1 "writes a populated, working config (not an inert commented template)"; FR-D4 "A provider
       whose tooled_flags is empty cannot serve as the stager … the bootstrap config annotates the fallback."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go            # THE file to EDIT: rewrite runConfigInit, +flags, +buildBootstrapConfig, +helpers. KEEP template/path/init().
  config_test.go       # EDIT: update init tests for populated default + flags; add builder unit tests.
  providers.go         # READ: mirror installedNames + the NewRegistry pattern. DO NOT EDIT.
  root_test.go         # READ: reuse setupNoRepo/saveRootState/restoreRootState/writeConfigFile/chdir. DO NOT EDIT.
  root.go / default_action.go  # P1.M4.T1/P4.M1. UNCHANGED (do NOT edit; flags are LOCAL on configInitCmd).
internal/config/
  config.go            # READ CurrentConfigVersion(=2). (T1.S1 edits Defaults(); do NOT edit here.)
  file.go              # READ GlobalConfigPath. DO NOT EDIT.
  role_defaults.go     # READ DefaultModelsForProvider (FR-D4 table). DO NOT EDIT.
  load.go              # READ roleNames order (planner,stager,message,arbiter). DO NOT EDIT.
internal/provider/
  registry.go          # READ NewRegistry/Get/List/IsInstalled/DefaultProvider/preferredBuiltins. DO NOT EDIT.
  builtin.go           # READ (TooledFlags state — pi+claude stager-capable). DO NOT EDIT.
docs/configuration.md  # EDIT: populated format + 3 flags (Mode A).
go.mod / go.sum        # UNCHANGED (go-toml/v2 already a dep — used for the validity check; provider already imported).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/config.go            # EDIT — +3 local flags; rewrite runConfigInit; +buildBootstrapConfig (pure);
                                   #        +installedNames + stagerFallback helpers; +provider import; Mode-A help.
internal/cmd/config_test.go       # EDIT — populated-default tests; --template/--force/--provider tests;
                                   #        buildBootstrapConfig unit tests; TOML-validity check.
docs/configuration.md             # EDIT — populated format + 3 flags.
# NO new files. go.mod/go.sum UNCHANGED. root.go/providers.go/default_action.go/config.go(config pkg)/
# file.go/load.go/role_defaults.go/registry.go/builtin.go UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (config init skips config load — F1): shouldSkipConfigLoad returns true for cmd.Name()=="init",
// so PersistentPreRunE returns nil → Config() is NIL inside runConfigInit. Build the registry from BUILT-INS
// ONLY: provider.NewRegistry(nil). Do NOT call providers.go's newRegistry() (it reads Config().Providers).
// CONSEQUENCE: --provider can only target a BUILT-IN name (no user-defined providers are visible here).

// CRITICAL (buildBootstrapConfig must be PURE — F2): IsInstalled does exec.LookPath (non-deterministic per
// machine). Factor the TOML generation into buildBootstrapConfig(target, installed) — no detection, no I/O.
// runConfigInit does detection + I/O and DELEGATES the string. Tests call buildBootstrapConfig DIRECTLY
// with fixed args (deterministic) OR use --provider to pin the target end-to-end.

// CRITICAL (stager "" sentinel — F3): DefaultModelsForProvider(target)["stager"] == "" means the provider
// CANNOT be the stager (nil TooledFlags — gemini/agy/opencode/codex/cursor). Apply the FR-D4 fallback:
// scan preferredBuiltins for the first with non-empty stager (always "pi" today) → [role.stager] carries
// provider="<fallback>" + the fallback's stager model + an annotation comment. Do NOT leave [role.stager]
// with an empty model.

// CRITICAL (no-install "pi" fallback — F4): reg.DefaultProvider(installed) returns "" when NOTHING is
// installed. Default target to "pi" (preferredBuiltins[0]) with a header comment noting the guess, so the
// config is VALID (not an empty provider) and works once pi is installed.

// GOTCHA (config_version UNCOMMENTED in populated output — F6): write `config_version = CurrentConfigVersion`
// (the const, =2) UNCOMMENTED, top-level. This makes the P1.M4.T1.S1 advisory SILENT (version matches).
// The --template path keeps config_version COMMENTED (it's the inert reference).

// GOTCHA (KEEP exampleConfigTemplate — F5): do NOT remove/rewrite the const. It IS the --template output
// and carries P1.M4.T1.S1's config_version header. The populated path does NOT use it.

// GOTCHA (flags are LOCAL, not persistent): register via configInitCmd.Flags(), NOT rootCmd.PersistentFlags()
// (else they'd leak onto config path + the root command). shouldSkipConfigLoad stays true for "init".

// GOTCHA (overwrite message unchanged — F8): keep the EXACT "config file already exists at %s (not
// overwritten)" error (TestConfigInit_RefusesOverwrite asserts Contains "already exists"). --force bypasses
// the os.Stat existence check for BOTH --template and populated.

// GOTCHA (success message): populated → "Wrote config to %s"; --template → keep "Wrote example config to %s".
// The path is ALWAYS printed (FR-B1).

// GOTCHA (do NOT do FR-B6 help de-dup): configCmd's manual "Subcommands:" block stays (P4.M2.T2.S1 owns its
// removal). Only update the `init` line's description in configCmd.Long if needed.

// GOTCHA (role order): write [role.*] blocks in the canonical order planner, stager, message, arbiter
// (internal/config/load.go var roleNames) for readability.

// GOTCHA (valid TOML): the populated output must parse. String-build carefully (quote strings, no trailing
// commas in arrays). Verify with a toml.Unmarshal into map[string]any in a test. Commented lines (# ...) are
// ignored by TOML, so the commented other-provider blocks don't affect validity.

// GOTCHA (cursor detect ≠ name): IsInstalled probes DetectCommand() — cursor's is "agent" (not "cursor").
// Don't special-case; reg.IsInstalled handles it. The installed list carries Names ("cursor"), not detect cmds.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/cmd/config.go — ADD these (KEEP all existing: configCmd/configInitCmd/configPathCmd/
// runConfigPath/exampleConfigTemplate/init()). Add import "github.com/dustin/stagecoach/internal/provider"
// and "strings" (strings already imported? check — add if missing).

// Local flags on configInitCmd (NOT persistent — must not leak to `config path` or root).
// In a new init() or inline after configInitCmd's def, OR via configInitCmd.Flags() in the existing init():
func init() { // extend the EXISTING init() — keep configCmd.AddCommand + rootCmd.AddCommand(configCmd)
	configInitCmd.Flags().String("provider", "", "Target a specific provider instead of auto-detecting")
	configInitCmd.Flags().Bool("force", false, "Overwrite an existing config file")
	configInitCmd.Flags().Bool("template", false, "Write the inert all-commented reference config (v1 behavior)")
}

// runConfigInit (REWRITE): --template → inert; else populated bootstrap. Reuse the stat/mkdir/write/exitcode
// skeleton; only the content source + --force bypass change.
func runConfigInit(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if _, err := os.Stat(path); err == nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path))
		} else if !os.IsNotExist(err) {
			return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err))
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
	}

	tmpl, _ := cmd.Flags().GetBool("template")
	var content string
	if tmpl {
		content = exampleConfigTemplate
	} else {
		providerName, _ := cmd.Flags().GetString("provider")
		reg := provider.NewRegistry(nil) // built-ins only (config load is skipped for init — F1)
		installed := installedNames(reg)
		target, err := resolveBootstrapTarget(reg, providerName, installed)
		if err != nil {
			return exitcode.New(exitcode.Error, err)
		}
		content = buildBootstrapConfig(target, installed)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}
	if tmpl {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote example config to %s\n", path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path)
	}
	return nil
}

// resolveBootstrapTarget resolves the bootstrap provider: --provider (validated) > cascade > "pi" fallback.
func resolveBootstrapTarget(reg *provider.Registry, providerName string, installed []string) (string, error) {
	if providerName != "" {
		if _, ok := reg.Get(providerName); !ok {
			return "", fmt.Errorf("unknown provider %q (use a built-in: %s)", providerName, strings.Join(preferredBuiltins, ", "))
		}
		return providerName, nil
	}
	if det := reg.DefaultProvider(installed); det != "" {
		return det, nil
	}
	return "pi", nil // nothing installed — valid default; annotated in buildBootstrapConfig
}

// installedNames mirrors providers.go's helper (unexported there) — Names of providers on $PATH.
func installedNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// buildBootstrapConfig is the PURE populated-config generator (PRD §9.17 FR-B1). NO detection, NO I/O —
// takes an already-resolved target + the installed list, returns the exact TOML. Deterministic ⇒ unit-
// testable. Writes: header docs, config_version (uncommented), [defaults] provider=<target> (uncommented),
// four [role.*] blocks for target (models from DefaultModelsForProvider; stager routed to the fallback
// when target can't stage, annotated), each OTHER installed provider as a commented [role.*] group, then a
// commented [generation] section.
func buildBootstrapConfig(target string, installed []string) string {
	var b strings.Builder
	// --- header (reuse the template's header text: precedence, env vars, git keys, cli flags) ---
	b.WriteString(bootstrapHeader) // a const with the shared header (see Task 2)

	// config_version (UNCOMMENTED — F6)
	fmt.Fprintf(&b, "config_version = %d\n", config.CurrentConfigVersion)

	// [defaults] — provider uncommented, rest commented
	b.WriteString("\n[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")

	// [role.*] for the target (UNCOMMENTED), canonical order
	models := config.DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
	stagerName, stagerModel := stagerFallback(target, models)
	b.WriteString("\n# --- per-role models for the default provider " + fmt.Sprintf("%q", target) + " (PRD §16.4, §9.15) ---\n")
	writeRoleBlock(&b, "planner", target, models["planner"], "")
	writeRoleBlock(&b, "stager", stagerName, stagerModel,
		"# "+target+" cannot serve as the stager (no tooled_flags); routed to "+stagerName+" (the first stager-capable provider).")
	// (omit the annotation when target IS stager-capable — stagerName==target)
	writeRoleBlock(&b, "message", target, models["message"], "")
	writeRoleBlock(&b, "arbiter", target, models["arbiter"], "")

	// other installed providers as COMMENTED [role.*] groups
	for _, name := range preferredBuiltins {
		if name == target || !isInstalledName(name, installed) {
			continue
		}
		other := config.DefaultModelsForProvider(name)
		b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
		writeCommentedRoleBlock(&b, "planner", name, other["planner"])
		writeCommentedRoleBlock(&b, "stager", name, other["stager"])
		writeCommentedRoleBlock(&b, "message", name, other["message"])
		writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
	}

	// commented [generation] defaults
	b.WriteString(generationCommented) // a const (see Task 2)
	return b.String()
}

// writeRoleBlock writes an UNCOMMENTED [role.<r>] block: provider line omitted when == target (inherits
// [defaults]); optional annotation comment printed first when non-empty.
func writeRoleBlock(b *strings.Builder, role, provider, model, annotation string) {
	fmt.Fprintf(b, "[role.%s]\n", role)
	if annotation != "" {
		fmt.Fprintf(b, "%s\n", annotation)
	}
	if provider != "" /* && provider != target-inherit */ {
		fmt.Fprintf(b, "provider = %q\n", provider)
	}
	fmt.Fprintf(b, "model = %q\n", model)
}

// writeCommentedRoleBlock writes a fully-commented [role.<r>] block for an alternate provider.
func writeCommentedRoleBlock(b *strings.Builder, role, provider, model string) {
	fmt.Fprintf(b, "# [role.%s]\n", role)
	fmt.Fprintf(b, "# provider = %q\n", provider)
	fmt.Fprintf(b, "# model = %q\n", model)
}

// stagerFallback returns the (provider, model) for the [role.stager] block: target's own if stager-capable
// (models["stager"] != ""), else the first stager-capable provider in preferredBuiltins order (always "pi"
// today). F3.
func stagerFallback(target string, models map[string]string) (string, string) {
	if m := models["stager"]; m != "" {
		return target, m
	}
	for _, name := range preferredBuiltins {
		if col := config.DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
			return name, col["stager"]
		}
	}
	return target, models["stager"] // unreachable (pi is always stager-capable) — defensive
}

func isInstalledName(name string, installed []string) bool {
	for _, n := range installed {
		if n == name {
			return true
		}
	}
	return false
}
```

```go
// bootstrapHeader + generationCommented are CONSTS (the shared header/footer text). Factor the header from
// the EXISTING exampleConfigTemplate (precedence/env/git-key/cli-flags blocks — identical text, since both
// configs share the same docs). Place them near exampleConfigTemplate.
//
// bootstrapHeader: the "# Stagecoach configuration file …" intro + precedence + env vars + git keys + cli
// flags blocks, ending with a blank line. Adapt the intro line to say "bootstrap (populated)".
//
// generationCommented: the "# [generation]\n# max_diff_bytes = 300000\n …" block (commented defaults).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/config.go — add the 3 local flags + imports
  - FILE: internal/cmd/config.go. ADD import "github.com/dustin/stagecoach/internal/provider" (cmd→provider
      already a dep — F7) and "strings" (if not present). KEEP existing imports (fmt, os, path/filepath,
      cobra, config, exitcode).
  - EXTEND the existing `func init()` (which does configCmd.AddCommand + rootCmd.AddCommand(configCmd)) with:
        configInitCmd.Flags().String("provider", "", "Target a specific provider instead of auto-detecting")
        configInitCmd.Flags().Bool("force", false, "Overwrite an existing config file")
        configInitCmd.Flags().Bool("template", false, "Write the inert all-commented reference config (v1 behavior)")
  - GOTCHA: LOCAL flags (configInitCmd.Flags()), NOT PersistentFlags (would leak to `config path`/root).
      shouldSkipConfigLoad stays true for "init".

Task 2: EDIT internal/cmd/config.go — factor bootstrapHeader + generationCommented consts
  - EXTRACT the shared header text from exampleConfigTemplate (the precedence/env-var/git-key/cli-flag doc
      blocks — identical for both configs) into `const bootstrapHeader = \`…\``. Adapt the intro to
      "bootstrap (populated)". EXTRACT the commented [generation] block into `const generationCommented`.
  - WHY: buildBootstrapConfig reuses the SAME docs (DRY) — only the intro + which lines are commented differ.
  - GOTCHA: keep exampleConfigTemplate intact (it still embeds its own copy of the header — T1.S1's
      config_version block lives there). Do NOT refactor exampleConfigTemplate to reference bootstrapHeader
      (risk of touching T1.S1's work); just copy the text into the new const.

Task 3: EDIT internal/cmd/config.go — add helpers (installedNames, resolveBootstrapTarget, stagerFallback,
        isInstalledName, writeRoleBlock, writeCommentedRoleBlock) + buildBootstrapConfig
  - FILE: internal/cmd/config.go. ADD the functions in the "Data models" skeleton. buildBootstrapConfig is
      PURE (F2). stagerFallback implements F3. resolveBootstrapTarget implements F4 (--provider→cascade→"pi").
  - GOTCHA: writeRoleBlock OMITS the `provider =` line when the role inherits [defaults] (planner/message/
      arbiter for the target); for the stager fallback it ALWAYS writes provider=<fallback>. Commented blocks
      always include provider. Role order = planner, stager, message, arbiter.

Task 4: EDIT internal/cmd/config.go — REWRITE runConfigInit
  - FILE: internal/cmd/config.go. REPLACE the existing runConfigInit body with the branch logic in the
      "Data models" skeleton: read --force/--template/--provider; existence check SKIPPED when --force;
      MkdirAll; content = (--template ? exampleConfigTemplate : buildBootstrapConfig(resolveBootstrapTarget…));
      WriteFile(0o644); print path ("Wrote example config to" vs "Wrote config to").
  - GOTCHA: keep the EXACT refuse-overwrite message (F8). never os.Exit. route errors via exitcode.New.

Task 5: EDIT internal/cmd/config.go — Mode-A help (configInitCmd Short/Long + configCmd init line)
  - UPDATE configInitCmd.Short (e.g. "Bootstrap a working config (auto-detects your agent)") and .Long to
      describe the populated bootstrap, cascading detection (FR-D1), and the three flags (--provider/--force/
      --template). Update configCmd.Long's init line to match (do NOT remove the "Subcommands:" block — FR-B6
      is P4.M2.T2.S1).

Task 6: EDIT internal/cmd/config_test.go — update existing tests + add new ones
  - FILE: internal/cmd/config_test.go. UPDATE/ADD:
  - TestConfigInit_Populated_WritesWorkingConfig (DEFAULT, no flags): setupNoRepo; SetArgs(["config","init"]);
    Execute. Assert exit 0; stdout contains "Wrote config to"; the file exists; content CONTAINS
    "config_version = 2", an uncommented "[defaults]" with "provider =" (uncommented), and an uncommented
    "[role.message]" (auto-detect provider is env-dependent — assert STRUCTURE not the exact provider; the
    provider value must be one of the built-in names or "pi"). Use `--provider pi` for EXACT assertions
    (next test).
  - TestConfigInit_ProviderPin_ExactOutput: SetArgs(["config","init","--provider","pi"]); Execute; read file;
    assert EXACTLY: `provider = "pi"`, `[role.planner]` + `model = "gpt-5.4"`, `[role.message]` +
    `model = "gpt-5.4-nano"`, `[role.stager]` + `model = "gpt-5.4-mini"` (pi is stager-capable — no fallback),
    `[role.arbiter]` + `model = "gpt-5.4-mini"`, and `config_version = 2`. (Deterministic — pi's column is fixed.)
  - TestConfigInit_ProviderStagerFallback: SetArgs(["config","init","--provider","gemini"]); read file; assert
    `[defaults] provider = "gemini"`, `[role.planner] model = "gemini-3.5-pro"`, and `[role.stager]` contains
    `provider = "pi"` + `model = "gpt-5.4-mini"` + an annotation mentioning gemini cannot stage. (gemini's
    stager is "" → FR-D4 fallback to pi.)
  - TestConfigInit_Template_WritesInert (was TestConfigInit_WritesTemplate): SetArgs(["config","init",
    "--template"]); assert file content == exampleConfigTemplate EXACTLY; stdout "Wrote example config to".
  - TestConfigInit_TemplateIsInert: move to --template path (asserts no uncommented TOML header). Same body,
    SetArgs adds "--template".
  - TestConfigInit_Force_Overwrites: pre-create the config ("provider = \"mine\""); SetArgs(["config","init",
    "--force","--provider","pi"]); Execute; assert exit 0; file content is now the POPULATED pi config (not
    "mine"). Also a --template --force variant (overwrites with exampleConfigTemplate).
  - TestConfigInit_RefusesOverwrite (EXISTING — keep, now tests populated default): unchanged body (no flags
    → populated; pre-existing file → exit 1 "already exists"; file unchanged).
  - TestConfigInit_UnknownProvider: SetArgs(["config","init","--provider","bogus"]); Execute; assert exit 1
    (exitcode.Error); err contains "unknown provider".
  - TestConfigInit_MkdirAllParent / _WorksOutsideGitRepo / _ExtraArgsExits1: KEEP (still valid; populated
    default). Update the second-init message check in _WorksOutsideGitRepo if the stdout wording changed.
  - TestBuildBootstrapConfig_* (UNIT — call buildBootstrapConfig DIRECTLY, no Execute, no $PATH):
      * TestBuildBootstrapConfig_Pi: buildBootstrapConfig("pi", []string{"pi"}) → assert the exact strings
        (provider="pi"; 4 role blocks with pi's models; NO fallback annotation; config_version=2).
      * TestBuildBootstrapConfig_GeminiStagerFallback: buildBootstrapConfig("gemini", nil) → [role.stager]
        has provider="pi" + the annotation; planner/message/arbiter are gemini's models.
      * TestBuildBootstrapConfig_OtherInstalledCommented: buildBootstrapConfig("pi", []string{"pi","claude"})
        → contains a commented `# [role.message]` + `# provider = "claude"` + `# model = "haiku"` block; the
        UNCOMMENTED [role.*] are pi's (not claude's).
      * TestBuildBootstrapConfig_ValidTOML: for several (target, installed), toml.Unmarshal(buildBootstrapConfig(...),
        &map[string]any) returns nil (valid TOML). Import github.com/pelletier/go-toml/v2 (already in go.mod).
  - GOTCHA: buildBootstrapConfig is deterministic → assert EXACT substrings. The Execute-driven auto-detect
    test asserts STRUCTURE only (provider ∈ built-ins ∪ {"pi"}), not the exact value. Each Execute test wraps
    in saveRootState/restoreRootState (rootCmd singleton hygiene). setupNoRepo isolates HOME/XDG.

Task 7: EDIT docs/configuration.md — populated format + 3 flags (Mode A)
  - UPDATE "Config file paths": `config init` writes a POPULATED working config by default; `--template`
    writes the inert reference; `--provider`/`--force` documented.
  - UPDATE "File format": replace "Every line … is commented out" with: the DEFAULT `config init` writes a
    populated config (detected provider + per-role models uncommented; config_version uncommented); show a
    short populated example (provider + [role.*]). Note `--template` for the all-commented reference.
  - ADD a short "Bootstrap (`config init`)" subsection: cascading detection (FR-D1 order), the stager
    fallback, the 3 flags, and "the written path is always printed."

Task 8: VERIFY (run all gates; fix before declaring done)
  - `go build ./... && go vet ./...` clean.
  - `go test -race ./internal/cmd/ -v` → all PASS (updated + new tests).
  - `go test ./...` → GREEN (no regression — config/provider/generate unaffected; the populated output is
      read by future Load calls but init itself skips Load).
  - `gofmt -l internal/cmd/ docs/` empty (docs is .md — gofmt skips it; just internal/cmd/).
  - `git diff --exit-code go.mod go.sum` → empty (go-toml/v2 + provider already deps).
  - `git status` shows EXACTLY 3 files: internal/cmd/config.go, internal/cmd/config_test.go,
      docs/configuration.md. Verify UNCHANGED: root.go, providers.go, default_action.go, config/config.go,
      config/file.go, config/load.go, config/role_defaults.go, provider/registry.go, provider/builtin.go.
```

### Implementation Patterns & Key Details

```go
// PATTERN: pure builder + detect/IO split (F2). buildBootstrapConfig(target, installed) is the ONLY thing
// that knows the TOML shape; runConfigInit is the ONLY thing that touches the filesystem/registry. Tests
// exercise the builder directly for determinism.

// PATTERN: mirror providers.go's installedNames (5-line loop over reg.List()+IsInstalled). Do NOT export the
// shared helper (local copy in config.go keeps the change self-contained; avoids editing providers.go).

// PATTERN: reuse the existing refuse-overwrite + MkdirAll + WriteFile(0o644) + exitcode.New skeleton — only
// the content source + the --force bypass are new.

// CRITICAL (stager fallback, F3): when DefaultModelsForProvider(target)["stager"] == "", the [role.stager]
// block MUST route to a stager-capable provider (provider=<fallback> + its stager model + annotation). Never
// write an empty `model = ""` for the stager. The fallback is the first preferredBuiltins with non-empty
// stager ("pi" today).

// CRITICAL (no-install fallback, F4): DefaultProvider("") == "" ⇒ target = "pi" + a header comment. Never
// write `provider = ""` in the populated config.

// GOTCHA: writeRoleBlock omits `provider =` when the role inherits [defaults] (so planner/message/arbiter
// for the target have only `model =`). The stager-fallback block always includes `provider =` (it overrides
// [defaults]). Commented blocks always include `provider =` (they're for switching).

// GOTCHA: config_version is UNCOMMENTED in the populated output (F6) but COMMENTED in exampleConfigTemplate
// (the --template path). Use config.CurrentConfigVersion (the const), not Defaults().

// GOTCHA: the populated output must be valid TOML — quote strings, no trailing commas, commented lines are
// harmless. Verify with toml.Unmarshal in a test.
```

### Integration Points

```yaml
DETECTION (provider.NewRegistry + IsInstalled — mirror providers.go):
  - build: "runConfigInit: reg := provider.NewRegistry(nil) (built-ins only — Config() is nil, F1);
    installed := installedNames(reg)."
  - target: "resolveBootstrapTarget: --provider (reg.Get-validated) > reg.DefaultProvider(installed) > 'pi'."

ROLE.MODELS (config.DefaultModelsForProvider — FR-D4 table):
  - consume: "buildBootstrapConfig reads DefaultModelsForProvider(target) for the uncommented [role.*]
    blocks and DefaultModelsForProvider(other) for the commented ones. stager=="" ⇒ stagerFallback (F3)."

SCHEMA.VERSION (config.CurrentConfigVersion — read-only):
  - write: "populated output: `config_version = CurrentConfigVersion` UNCOMMENTED (F6) → the T1.S1 advisory
    is silent for a fresh bootstrap. --template keeps it commented (inert)."

PATH (config.GlobalConfigPath):
  - write: "runConfigInit writes to GlobalConfigPath(); MkdirAll parent; refuse/--force; always print path."

FLAGS (configInitCmd.Flags — LOCAL):
  - add: "--provider <string>, --force <bool>, --template <bool> on configInitCmd (NOT persistent)."

DOCS (Mode A):
  - edit: "docs/configuration.md — populated format + 3 flags + cascading detection + stager fallback."

GO.MODULE: change NONE. go-toml/v2 (validity test) + internal/provider already deps. `go mod tidy` no-op.

FROZEN/LEAVE (do NOT edit):
  - root.go, providers.go, default_action.go (P1.M4.T1/P4.M1).
  - config/config.go (CurrentConfigVersion/Defaults — T1.S1), config/file.go, config/load.go,
    config/role_defaults.go (P1.M3), provider/registry.go, provider/builtin.go (P1.M2).
  - exampleConfigTemplate const (KEEP — it's the --template output; carries T1.S1's header).
  - FR-B6 help de-dup (P4.M2.T2.S1) — do NOT remove configCmd's "Subcommands:" block.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/cmd/config.go internal/cmd/config_test.go
go vet ./internal/cmd/
# Confirm the 3 flags are LOCAL (not persistent) — must NOT appear under `stagecoach config path --help`:
go run ./cmd/stagecoach config init --help   | grep -E -- "--provider|--force|--template"   # expect all 3
go run ./cmd/stagecoach config path --help   | grep -E -- "--provider|--force|--template"   # expect NONE
go run ./cmd/stagecoach --help               | grep -E -- "--template"                       # expect NONE (local to init)
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; flags appear ONLY under `config init --help`.
```

### Level 2: Unit + CLI tests (the cmd suite)

```bash
# The pure builder — deterministic, all branches:
go test -race ./internal/cmd/ -run TestBuildBootstrapConfig -v
# Expected PASS: Pi (exact models, no fallback), GeminiStagerFallback ([role.stager]→pi + annotation),
#                OtherInstalledCommented (claude as # block), ValidTOML (parses).

# The CLI-driven init tests:
go test -race ./internal/cmd/ -run TestConfigInit -v
# Expected PASS: Populated_WritesWorkingConfig (structure), ProviderPin_ExactOutput (pi exact),
#                ProviderStagerFallback (gemini→pi stager), Template_WritesInert (==exampleConfigTemplate),
#                TemplateIsInert (--template), Force_Overwrites, RefusesOverwrite, UnknownProvider,
#                MkdirAllParent, WorksOutsideGitRepo, ExtraArgsExits1.

# config path + group tests unchanged:
go test -race ./internal/cmd/ -run 'TestConfigPath|TestConfigGroup' -v

# Full cmd suite (no regression):
go test -race ./internal/cmd/ -v
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — config/provider/generate unaffected (init skips Load).
# Confirm ONLY the 3 target files differ:
git status --porcelain
# Expected: exactly 3: internal/cmd/config.go, internal/cmd/config_test.go, docs/configuration.md.
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/cmd/root.go internal/cmd/providers.go internal/cmd/default_action.go \
  internal/config/config.go internal/config/file.go internal/config/load.go internal/config/role_defaults.go \
  internal/provider/registry.go internal/provider/builtin.go go.mod go.sum \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Manual end-to-end (real $PATH detection)

```bash
make build
TMP=$(mktemp -d); export HOME=$TMP XDG_CONFIG_HOME=$TMP; cd "$TMP"
# Default (auto-detect — provider depends on what's installed):
./bin/stagecoach config init && cat "$(./bin/stagecoach config path)"
#   → expect: config_version = 2; [defaults] provider = <an installed built-in or "pi">; four [role.*]
#     blocks uncommented (stager possibly routed to pi with an annotation); other installed providers
#     as commented [role.*] blocks.
# Pin a provider (deterministic):
./bin/stagecoach config init --force --provider claude && cat "$(./bin/stagecoach config path)"
#   → [defaults] provider = "claude"; [role.planner] model="opus"; [role.message] model="haiku";
#     [role.stager] model="sonnet" (claude IS stager-capable — no fallback).
# Stager fallback (gemini cannot stage):
./bin/stagecoach config init --force --provider gemini && grep -A2 '\[role.stager\]' "$(./bin/stagecoach config path)"
#   → provider = "pi" + model = "gpt-5.4-mini" + the annotation.
# Inert template:
./bin/stagecoach config init --force --template && head -5 "$(./bin/stagecoach config path)"
#   → the all-commented exampleConfigTemplate.
# Errors:
./bin/stagecoach config init --provider bogus; echo "EXIT=$?"   # → exit 1, "unknown provider"
# Verify the populated config LOADS cleanly (advisory silent — version matches):
./bin/stagecoach config path    # (no stderr advisory when version==2)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/cmd/` clean.
- [ ] `go test ./...` PASS (cmd suite incl. updated + new tests; no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] `git status` shows EXACTLY 3 files; every LEAVE file byte-unchanged.

### Feature Validation
- [ ] `config init` (default) writes a populated config: `config_version = 2`, `[defaults] provider =
      <detected|"pi">`, four uncommented `[role.*]` blocks (FR-D4 models; stager fallback applied).
- [ ] `config init --provider <built-in>` writes that provider's exact role models (deterministic).
- [ ] `config init --provider bogus` → exit 1 "unknown provider".
- [ ] `config init --force` overwrites (populated and `--template`).
- [ ] `config init --template` writes `exampleConfigTemplate` verbatim (all-commented; inert).
- [ ] Refuse-overwrite without `--force` (exact "already exists" message); parent dirs created; path always printed.
- [ ] Other INSTALLED providers appear as commented `[role.*]` blocks.
- [ ] The populated output is valid TOML (`toml.Unmarshal` succeeds).

### Code Quality Validation
- [ ] `buildBootstrapConfig` is PURE (no I/O/detection) — deterministic + unit-tested.
- [ ] Detection mirrors providers.go's `installedNames`; registry built from built-ins only (`NewRegistry(nil)`).
- [ ] Stager fallback (F3) + no-install "pi" fallback (F4) implemented and annotated.
- [ ] Flags are LOCAL to `configInitCmd` (not persistent); shouldSkipConfigLoad unchanged for "init".
- [ ] `exampleConfigTemplate`, `runConfigPath`, `configPathCmd`, `init()` registration preserved.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] `configInitCmd.Long` (Mode A) describes populated bootstrap + 3 flags + cascading detection.
- [ ] `docs/configuration.md` documents the populated format + the 3 flags + the stager fallback.
- [ ] No new env vars (the flags are CLI-only; --provider/--model env already exist).

---

## Anti-Patterns to Avoid

- ❌ **Don't make `buildBootstrapConfig` do detection or I/O.** IsInstalled is non-deterministic (exec.LookPath
  per machine); folding it into the builder makes the output un-unit-testable. Keep it PURE(target, installed)
  (F2); runConfigInit does detection + I/O.
- ❌ **Don't call `providers.go`'s `newRegistry()` or `Config()` in init.** Config() is nil (config load is
  skipped for init — F1). Build the registry from built-ins: `provider.NewRegistry(nil)`.
- ❌ **Don't leave `[role.stager]` with an empty model.** When the target can't stage (stager==""), apply the
  FR-D4 fallback (route to the first stager-capable provider, annotated). Never write `model = ""` for stager.
- ❌ **Don't write `provider = ""` in the populated config.** When nothing is installed, default to "pi" +
  a comment (F4). An empty provider is not a "populated, working" config.
- ❌ **Don't remove or rewrite `exampleConfigTemplate`.** It's the `--template` output and carries P1.M4.T1.S1's
  config_version header (F5). The populated path does not use it.
- ❌ **Don't make the flags persistent.** Register on `configInitCmd.Flags()`, not `rootCmd.PersistentFlags()`
  (else they leak to `config path` and the root command).
- ❇ **Don't change the refuse-overwrite message.** Keep the exact "config file already exists at %s (not
  overwritten)" (the existing test asserts `Contains "already exists"`); `--force` bypasses the check.
- ❌ **Don't read `Defaults().ConfigVersion`.** It's 0 after T1.S1. Use the `CurrentConfigVersion` const (=2)
  for the populated output's `config_version`.
- ❌ **Don't do FR-B6 help de-dup.** Removing configCmd's "Subcommands:" block is P4.M2.T2.S1's job — leave it;
  only update the init line.
- ❌ **Don't touch root.go/providers.go/default_action.go or the config/provider packages.** This task edits
  ONLY internal/cmd/config.go, internal/cmd/config_test.go, docs/configuration.md.
- ❌ **Don't assert the exact auto-detected provider in an Execute test.** It's $PATH-dependent. Pin with
  `--provider` for exact assertions; assert STRUCTURE (provider ∈ built-ins ∪ {"pi"}) for the auto-detect path.
