---
name: "P1.M4.T1.S3 — providers list/show subcommands — PRD §9.11 (FR46/FR47/FR48) / §15.3 / §12.8"
description: |

  Add a `providers` command group with two leaf subcommands (PRD §15.3): `stagecoach providers list`
  (FR46 — list built-in + user-defined providers, mark each detected-on-$PATH with ✓/✗, and mark the
  resolved default) and `stagecoach providers show <name>` (FR47 — print the fully-resolved manifest
  for a provider, built-in merged with user overrides, as TOML). Both are thin READ-only views over the
  `provider.Registry` shipped in P1.M2.T3.S1 (`List`, `IsInstalled`, `DefaultProvider`, `MarshalTOML`,
  + `DecodeUserOverrides`/`NewRegistry` to bridge config). The command group is registered via an
  `init()` in a NEW file — ZERO edits to any existing file (parallel-safe with S2).

  This subtask does NOT generate, parse, commit, or touch git. It is a pure inspection/display layer
  over the already-merged registry. The registry's merge semantics (FR48: user overrides merge
  field-by-field onto a built-in of the same name; brand-new names add new providers) are inherited
  unchanged — S3 only DISPLAYS them.

  DELIVERABLES (2 NEW files; NOTHING under internal/{config,generate,git,prompt,provider},
  pkg/stagecoach, exitcode, or cmd/root.go is touched — all frozen READ-ONLY contracts):
    1. CREATE `internal/cmd/providers.go`      — `package cmd`. `providersCmd` (group) +
       `providersListCmd`/`providersShowCmd` (leaves) + `init()` registering them on `rootCmd` +
       `runProvidersList`/`runProvidersShow` RunE + helpers `newRegistry`, `installedNames`,
       `resolvedDefault`, `printProvidersList`.
    2. CREATE `internal/cmd/providers_test.go` — `package cmd`. Tests via the FULL CLI
       (`rootCmd`/`Execute`) reusing root_test.go helpers: built-in list, ✓/✗ detection, default
       marker, user-override appears, show TOML round-trip, override-merge reflected, unknown→exit 1,
       arg-count validation.

  CONTRACT (PRD §9.11 FR46/FR47/FR48, §15.3, §12.8, Mode-A docs):
    - `providers list` → a table (stdout) with columns NAME / DETECTED(✓|✗) / DEFAULT. One row per
      `Registry.List()` (sorted ascending by Name). DETECTED = ✓ iff `Registry.IsInstalled(m)`.
      DEFAULT = `(default)` on the row whose Name == the RESOLVED default. Header row included.
    - "resolved default" (FR46 "show the resolved default") = `cfg.Provider` if non-empty, else
      `Registry.DefaultProvider(installed)` (mirrors pkg/stagecoach.buildDeps). `installed` =
      `installedNames(reg)` (List+IsInstalled, mirrors buildDeps).
    - `providers show <name>` → `Registry.MarshalTOML(name)` printed to stdout AS-IS (it already
      ends in `\n`). Unknown name → exit 1.
    - Exit codes: success 0; any failure (config-load, decode, unknown provider, cobra arg error) 1.
      Routed via `exitcode.New(exitcode.Error, err)`; NEVER `os.Exit` (main owns that via exitcode.For).

  SCOPE BOUNDARY (owned by siblings — do NOT implement): the default commit action (S2 — root RunE);
  `config init/path` subcommands (S4 — owns `shouldSkipConfigLoad`); signal handling (P1.M4.T2);
  color/TTY restyling of the ✓/✢ glyphs + progress lines (P1.M4.T3 — S3 keeps printProvidersList
  taking an `io.Writer` so it can restyle); dry-run (P1.M4.T4). S3 does NOT modify
  `shouldSkipConfigLoad` — it NEEDS config loaded (user overrides for FR46/FR47).

  INPUT (upstream — READ-ONLY contracts, all on disk):
    - `provider.Registry` (P1.M2.T3.S1 — registry.go): `NewRegistry(map[string]Manifest)*Registry`,
      `(*Registry).List()[]Manifest` (sorted asc by Name), `Get(name)(Manifest,bool)`,
      `IsInstalled(m)bool` (exec.LookPath(m.DetectCommand())), `DefaultProvider(installed[]string)string`,
      `MarshalTOML(name)(string,error)` (merged manifest as TOML; unknown→error). +
      `provider.DecodeUserOverrides(raw map[string]map[string]any)(map[string]Manifest,error)` +
      `provider.Manifest` (manifest.go) with `.Name` + `.DetectCommand()`.
    - `config.Config.Providers` (P1.M1.T4 — config.go): the raw `map[string]map[string]any` carrying
      user `[provider.<name>]` overrides; nil if none. Bridged to the Registry by DecodeUserOverrides.
    - S1's `internal/cmd/root.go`: `rootCmd` (package-level singleton), `Config() *config.Config`
      (PersistentPreRunE result — runs for list/show because they're NOT in `shouldSkipConfigLoad`),
      `Execute(ctx) error`, `loadedCfg`. S1's `internal/cmd/root_test.go` helpers (same package —
      REUSABLE, do NOT re-copy): `saveRootState`/`restoreRootState`/`resetFlags`, `loadEnvSetup`,
      `initRepo`, `setGitConfig`, `writeConfigFile`, `chdir`.
    - S1's `internal/exitcode`: `New(code,err)*ExitError`, `For(err)int`, constant `Error=1`.

  OUTPUT (downstream consumers): the `providers` command group is the user-facing provider
  inspection surface (PRD §15.3). P1.M4.T3 may colorize the ✓/✗ glyphs (S3 writes plain glyphs to an
  io.Writer). P1.M5.T4 (README) may show example output. The shipped `providers/*.toml` reference
  files (P1.M5.T2.S1) are the hand-written equivalents of `providers show <name>` output.

  ⚠️ S3 does NOT edit root.go — registration is via `init()` in the NEW providers.go (design §0/§12).
     This is parallel-safe with S2 (S2 edits root.go's RunE; disjoint files, same package).
  ⚠️ Config DOES load for list/show (design §3) — they are NOT added to `shouldSkipConfigLoad`
     (S4 owns that). User overrides (FR46/FR47) require `cfg.Providers`. Consequence: list/show run
     where `config.Load` succeeds (any git repo; outside a repo Layer-4 git-config hard-fails → exit 1).
  ⚠️ "resolved default" (design §4) = cfg.Provider if set, ELSE DefaultProvider(installed) — the
     SAME resolution pkg/stagecoach.buildDeps uses. Honors an explicit cfg.Provider (incl. a §12.8 name).
  ⚠️ MarshalTOML output already ends in `\n` (verified empirically) → use `fmt.Fprint`, NOT `Fprintln`
     (design §6). nil pointers are omitted (free omitempty); `subcommand=[]` for empty slices;
     explicit-empty `*""` marshals as `''`.
  ⚠️ Do NOT add a PersistentPreRunE to providers/list/show — it would SHADOW root's and skip config
     load (design §3). Root's is inherited; leave it that way.

  Deliverable: 2 NEW files. `make build` → `./bin/stagecoach providers list` (inside a repo) prints the
  NAME/DETECTED/DEFAULT table; `./bin/stagecoach providers show pi` prints the merged pi manifest as
  TOML; `./bin/stagecoach providers show ghost` exits 1. `go test -race ./internal/cmd/` green; no
  regression in `go test -race ./...`.

---

## Goal

**Feature Goal**: Ship Stagecoach's provider-inspection CLI surface (PRD §9.11 / §15.3) — a `providers`
command group whose `list` subcommand renders a NAME/DETECTED/DEFAULT table over the merged registry
(FR46, including the resolved default) and whose `show <name>` subcommand prints a provider's
fully-resolved manifest as TOML (FR47), both as thin read-only views over the P1.M2.T3.S1 Registry,
with user overrides from config reflected (FR48), Mode-A help-text documentation, and §15.4 exit codes
(0 success / 1 any failure) routed through S1's centralized `exitcode`.

**Deliverable** (2 NEW files; ZERO edits to any existing file):
1. `internal/cmd/providers.go` — `package cmd`. The `providersCmd` group + `providersListCmd` +
   `providersShowCmd` cobra commands; `init()` registering them on `rootCmd`; `runProvidersList`/
   `runProvidersShow` RunE functions; helpers `newRegistry()(*provider.Registry,error)`,
   `installedNames(*provider.Registry)[]string`, `resolvedDefault(*config.Config,*provider.Registry,
   []string)string`, `printProvidersList(io.Writer,*provider.Registry,string)`. Mode-A `Short`/`Long`
   help text on all three commands.
2. `internal/cmd/providers_test.go` — `package cmd`. Integration tests driving the FULL CLI
   (`rootCmd`/`Execute`) reusing root_test.go helpers: built-in listing, ✓/✗ detection (via
   installed `go` vs a bogus binary), default marker (STAGECOACH_PROVIDER + auto-detect), user-override
   appearance + merge, show TOML round-trip, unknown→exit 1, arg-count validation.

**Success Definition**: `make build` → `./bin/stagecoach providers list` (inside a git repo) prints a
table whose rows are the 6 built-ins (claude, codex, cursor, gemini, opencode, pi) sorted ascending,
each with ✓ or ✗ in DETECTED, and `(default)` on the resolved-default row (pi when pi is installed and
no provider is configured); `./bin/stagecoach providers show pi` prints `name = 'pi'`, `command = 'pi'`,
`default_model = 'glm-5-turbo'`, etc. (the merged manifest) to stdout, exit 0; a user `[provider.pi]
default_model="glm-5.2"` override is reflected in BOTH `list` (pi still listed) and `show pi`
(`default_model = 'glm-5.2'`); `./bin/stagecoach providers show ghost` exits 1 with
`stagecoach: unknown provider "ghost"`; `./bin/stagecoach providers` (no subcommand) prints help.
`go test -race ./internal/cmd/` green; `go test -race ./...` shows NO regression; `go vet ./...` clean;
`gofmt -l internal/cmd/` empty; only the 2 listed files added (root.go UNCHANGED).

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 "the plan-holder" / "the multi-agent tinkerer") who
wants to know WHICH agents Stagecoach can drive, whether each is installed on this machine, and which
one will be used by default — plus the "API-key refusenik" (§7.2) defining a custom `[provider.X]` who
needs to confirm their override merged correctly.

**Use Case**: `stagecoach providers list` (what's available + what's detected + what's the default);
`stagecoach providers show pi` (see the exact command Stagecoach will run for pi, after my overrides);
`stagecoach providers show myagent` (confirm my custom provider manifest is well-formed).

**User Journey**: user runs `stagecoach providers list` → sees the table → runs `stagecoach providers
show <name>` to inspect a manifest → adjusts their config → re-runs `show` to confirm the merge.

**Pain Points Addressed**: (1) discoverability — the 6 built-ins aren't obvious without docs (FR46);
(2) "is X installed?" — one column answers it without `which` (FR46 ✓/✗); (3) "what's the default?"
— no need to reason about preference order (FR46 resolved default); (4) override verification —
`show` prints the MERGED manifest so a user sees exactly what the pipeline will use (FR47/FR48).

## Why

- **Closes the provider-management surface (PRD §9.11).** The Registry (M2) is invisible without a
  CLI; `providers list/show` are the only user-facing windows into it. This is the P1 ship-list item
  "providers list/show" (PRD §10.1).
- **Makes overrides debuggable (FR48).** A user who sets `[provider.pi] default_model="glm-5.2"` has
  no other way to confirm the merge took effect. `show` prints the merged result; `list` shows the
  custom name appeared. This is the difference between "I think my config works" and "I can see it."
- **Reuses proven merge logic, zero new semantics.** S3 DISPLAYS `Registry.MarshalTOML`/`List`/
  `IsInstalled`/`DefaultProvider` output verbatim — it adds no merge/resolution logic of its own. The
  generate pipeline (M3) and this CLI now read the SAME registry built the SAME way
  (`DecodeUserOverrides`+`NewRegistry`), so what `show` prints is exactly what `GenerateCommit` uses.
- **Parallel-safe with S2 (design §0/§12).** Registered via `init()` in a new file; S2 edits root.go's
  RunE. Disjoint files, same package, no merge hazard.

## What

A `providers` cobra command group (parent shows help) with two leaves:
- `list` (`cobra.NoArgs`): builds the registry from `Config().Providers`, computes `installedNames`
  and `resolvedDefault`, and prints a tabwriter table (NAME / DETECTED / DEFAULT + header) to stdout.
- `show <name>` (`cobra.ExactArgs(1)`): builds the registry, calls `MarshalTOML(name)`, prints the
  TOML to stdout as-is; unknown name → `exitcode.New(exitcode.Error, …)` (exit 1).

Both RunE funcs read `Config()` (S1's PersistentPreRunE loaded it — list/show are NOT in
`shouldSkipConfigLoad`), build the registry via `newRegistry()` (DecodeUserOverrides + NewRegistry,
mirroring `pkg/stagecoach.buildDeps`), and return `nil` on success / an `*exitcode.ExitError` on
failure. Neither calls `os.Exit`.

### Success Criteria

- [ ] `internal/cmd/providers.go` exists, `package cmd`, imports `fmt`+`io`+`os`+`text/tabwriter`+
      `github.com/spf13/cobra` + `github.com/dustin/stagecoach/internal/{config,exitcode,provider}`.
- [ ] `providersCmd` (Use "providers"), `providersListCmd` (Use "list", Args NoArgs), `providersShowCmd`
      (Use "show <name>", Args ExactArgs(1)) are defined; each has a `Short` and a `Long`.
- [ ] `func init()` does `providersCmd.AddCommand(providersListCmd, providersShowCmd)` then
      `rootCmd.AddCommand(providersCmd)`. root.go is NOT edited.
- [ ] `runProvidersList(cmd, args) error`: builds registry; computes installed + resolvedDefault;
      prints the NAME/DETECTED/DEFAULT table (header + one sorted row per `List()`) to stdout; the row
      with Name==resolvedDefault carries `(default)`; returns nil.
- [ ] `runProvidersShow(cmd, args) error`: name=args[0]; builds registry; `MarshalTOML(name)`→stdout
      via `fmt.Fprint` (no extra newline); on error returns `exitcode.New(exitcode.Error,
      fmt.Errorf("unknown provider %q", name))`; else nil.
- [ ] `newRegistry()` reads `Config()`; on nil cfg uses no overrides; `DecodeUserOverrides`→`NewRegistry`;
      decode error wrapped + returned (caller maps to exit 1).
- [ ] `resolvedDefault(cfg, reg, installed)` = cfg.Provider if non-empty, else `reg.DefaultProvider(installed)`.
- [ ] `installedNames(reg)` iterates `reg.List()`, appends Name where `reg.IsInstalled(m)`.
- [ ] Neither RunE calls `os.Exit`; both return errors consumable by `exitcode.For` (S1's main).
- [ ] No `PersistentPreRunE` added to any of the three commands (root's is inherited — config loads).
- [ ] `go test -race ./internal/cmd/` green; `go test -race ./...` NO regression; `go vet ./...` clean;
      `gofmt -l internal/cmd/` empty; only `providers.go` + `providers_test.go` NEW (root.go UNCHANGED).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
upstream signatures (all quoted below + in research/design-decisions.md), the 12 design decisions, the
PRD §9.11/§15.3 contracts (in `selected_prd_content`), the verified MarshalTOML output shape (§6), the
copy-ready skeletons in the Implementation Blueprint, and the test conventions to mirror
(`internal/cmd/root_test.go` integration pattern). No generation/commit/signal/UI knowledge required
(those are explicitly out of scope — S3 only reads the Registry + exec.LookPath).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T1S3/research/design-decisions.md
  why: the 12 decisions specific to this subtask. §0 (2 NEW files + init() registration; ZERO root.go
       edits — parallel-safe with S2), §1 (cobra group shape), §2 (newRegistry mirrors buildDeps:
       DecodeUserOverrides+NewRegistry), §3 (config DOES load for list/show; NOT in shouldSkipConfigLoad;
       consequence: runs where config.Load succeeds), §4 (resolvedDefault = cfg.Provider else
       DefaultProvider(installed)), §5 (list table format: tabwriter, NAME/DETECTED✓✗/DEFAULT header),
       §6 (show: MarshalTOML as-is, output already ends in \n — use Fprint not Fprintln), §7 (exit
       codes via exitcode.New/Error; never os.Exit; unknown→1), §8 (Mode-A Short/Long help text), §9
       (testing: mirror root_test.go; installed-detection via `go` vs bogus binary), §10 (stdout=data,
       stderr=diagnostics), §11 (do NOT touch shouldSkipConfigLoad), §12 (no conflict with S2).
  critical: §0 (WHY no root.go edit), §3 (config MUST load — the repo-requirement consequence), §4
       (the EXACT default-resolution formula), §6 (Fprint not Fprintln + the verified output shape).

- docfile: plan/001_f1f80943ac34/P1M4T1S1/PRP.md   (the SCAFFOLD contract — S3 hangs on it)
  section: "Data models" (internal/cmd/root.go skeleton: rootCmd, PersistentPreRunE, Config(),
       Execute, shouldSkipConfigLoad) + "Implementation Tasks" (root.go + root_test.go helpers).
  why: S1's root.go is the file S3's `init()` calls `rootCmd.AddCommand(...)` on (S3 does NOT edit
       root.go — it adds a sibling file). The `Config()` accessor, `Execute(ctx)`, and
       `shouldSkipConfigLoad` (returns true only for "init"/"path" — NOT list/show) are S1's
       deliverables S3 consumes. root_test.go's helpers are package-private to `cmd` → S3's test
       (same package) REUSES them directly (no re-copy).
  pattern: rootCmd is a package-level singleton; PersistentPreRunE loads config for any command not in
       the skip-list; main calls os.Exit(exitcode.For(err)) once and prints `stagecoach: <err>` only
       when err.Error() != "". S3 returns errors that interplay with BOTH (non-empty err → main prints).
  gotcha: do NOT add list/show to shouldSkipConfigLoad (they NEED config). do NOT edit root.go at all.

- file: internal/provider/registry.go   (P1.M2.T3.S1 — the Registry; S3's PRIMARY input; READ, do NOT edit)
  section: `func NewRegistry(userOverrides map[string]Manifest) *Registry` (built-ins ⊕ overrides),
       `func (r *Registry) List() []Manifest` (sorted ASC by Name), `func (r *Registry) Get(name string)
       (Manifest, bool)`, `func (r *Registry) IsInstalled(m Manifest) bool` (exec.LookPath(m.DetectCommand());
       "" → false), `func (r *Registry) DefaultProvider(installed []string) string` (first preferred
       built-in in installed; "" if none; user-defined names never auto-selected), `func (r *Registry)
       MarshalTOML(name string) (string, error)` (merged manifest as TOML; unknown→wrapped error),
       `func DecodeUserOverrides(raw map[string]map[string]any) (map[string]Manifest, error)` (bridges
       config.Providers → typed Manifests; nil→empty non-nil map, no error).
  why: THESE are the only provider-side functions S3 calls. list = List+IsInstalled+DefaultProvider;
       show = MarshalTOML. newRegistry = DecodeUserOverrides+NewRegistry. Nothing else from the package.
  pattern: List() returns a fresh sorted slice (deterministic for the table). IsInstalled probes
       DetectCommand() (Detect if set & non-empty, else Command) — cursor is the ONLY built-in where
       Detect≠Name (Detect="agent"), so cursor shows ✓ iff `agent` is on PATH (correct). MarshalTOML
       marshals the STORED (merged) manifest — go-toml OMITS nil *string/*bool pointers (free
       omitempty), so output shows what's actually configured with absent fields suppressed.
  gotcha: MarshalTOML output ALREADY ends in '\n' (verified — design §6); use fmt.Fprint, NOT Fprintln.
       DefaultProvider only considers BUILT-IN names (preferredBuiltins: pi,claude,gemini,opencode,
       codex,cursor) — a §12.8 user provider is never the auto-default (but an explicit cfg.Provider
       pointing at one IS the resolved default per §4).

- file: internal/provider/manifest.go   (P1.M2.T1.S1 — the Manifest struct; READ, do NOT edit)
  section: `type Manifest struct { Name string; Detect, Command *string; Subcommand []string; … }` +
       `func (m Manifest) DetectCommand() string` (Detect if set&non-empty, else Command, else "").
  why: `m.Name` is the table key/identity (registry sets it from the [provider.<name>] key); it's the
       NAME column. `m.DetectCommand()` is what IsInstalled probes (shown as ✓/✗). S3 reads ONLY
       these two (Name via List(); DetectCommand via IsInstalled) — it never touches the other fields.
  pattern: pointer scalars are *string/*bool (nil ⇒ absent ⇒ omitted on marshal; non-nil ⇒ present,
       even if "" or false). S3 does NOT need to dereference any pointer — MarshalTOML handles that.
  gotcha: Validate/Resolve are NOT called by S3 — the registry stores merged-but-unresolved manifests
       and MarshalTOML prints them as-is (letting `show` display a partially-defined provider for
       debugging, per registry.go's doc comment). Do NOT add Validate/Resolve calls.

- file: internal/provider/builtin.go   (P1.M2.T2 — the 6 built-in manifests; READ for test expectations)
  section: `BuiltinManifests()` returns pi/claude/gemini/opencode/codex/cursor. Each has Name + Detect
       + Command (+ DefaultModel where set: pi=glm-5-turbo, claude=sonnet, gemini=gemini-2.5-pro;
       opencode/codex/cursor have DefaultModel="" explicit-empty).
  why: tests assert `show pi` output contains `command = 'pi'` and `default_model = 'glm-5-turbo'`;
       `list` shows exactly these 6 names sorted. Knowing the DefaultModel values makes the show-merge
       test deterministic (`[provider.pi] default_model="glm-5.2"` → `default_model = 'glm-5.2'`).
  gotcha: cursor's Detect/Command = "agent" (≠ Name "cursor") — so `show cursor` prints `command =
       'agent'` and `detect = 'agent'`. codex Subcommand=["exec"], opencode Subcommand=["run"], cursor
       Subcommand=[] (non-nil empty → marshals as `subcommand = []`).

- file: internal/config/config.go   (P1.M1.T4.S1 — Config.Providers; READ, do NOT edit)
  section: `type Config struct { …; Providers map[string]map[string]any \`toml:"-"\` }` — the raw
       user-provider overrides from TOML files; nil if none. `toml:"-"` ⇒ excluded from flat marshal.
  why: `newRegistry()` reads `Config().Providers` and feeds it to DecodeUserOverrides. This is the
       ONLY config field S3 touches (it does NOT read Provider/Model/Timeout except Provider for the
       default marker). nil Providers → DecodeUserOverrides(nil) → empty map → NewRegistry = built-ins.
  gotcha: Config is a resolved snapshot (plain types); Providers is the raw map because the Manifest
       type lives in a later package (config must not import provider). DecodeUserOverrides is the
       bridge — do NOT try to decode Providers yourself.

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1 — buildDeps; READ the pattern, do NOT edit)
  section: `buildDeps(cfg, repoDir)` lines ~136-150: `overrides,err := provider.DecodeUserOverrides
       (cfg.Providers)` → `reg := provider.NewRegistry(overrides)` → `name := cfg.Provider; if name==""
       { for _,m := range reg.List() { if reg.IsInstalled(m) { installed=append(installed,m.Name) } };
       name = reg.DefaultProvider(installed) }`.
  why: this is the AUTHORITATIVE registry-build + default-resolution sequence. S3's `newRegistry()` and
       `resolvedDefault()`/`installedNames()` copy it verbatim so `list`/`show` reflect EXACTLY what
       GenerateCommit would use. Do not invent a different sequence.
  gotcha: buildDeps then calls Get/Validate/Resolve and errors if the resolved name is unknown — S3
       does NOT (list just shows the table; show delegates to MarshalTOML which errors on unknown).
       S3 is a pure VIEW; it does not enforce "default must be valid".

- file: internal/cmd/root.go   (P1.M4.T1.S1 — S3 calls rootCmd.AddCommand via init(); do NOT edit)
  section: `var rootCmd = &cobra.Command{Use:"stagecoach", SilenceErrors:true, SilenceUsage:true, …,
       PersistentPreRunE: <loads config>, RunE: <S1 stub, replaced by S2>}` + `func Config()
       *config.Config` + `func Execute(ctx context.Context) error` + `func shouldSkipConfigLoad(cmd)
       bool` (returns true ONLY for name=="init"||name=="path").
  why: S3's `init()` calls `rootCmd.AddCommand(providersCmd)`. The three RunE funcs read `Config()`
       (PersistentPreRunE set it — list/show aren't skipped). cobra inherits root's PersistentPreRunE
       to the new subcommands (none of them define their own). main maps the returned error via
       exitcode.For and prints `stagecoach: <err>` when non-empty.
  gotcha: do NOT edit root.go. do NOT add list/show to shouldSkipConfigLoad. do NOT give
       providers/list/show their own PersistentPreRunE (it would shadow root's → config wouldn't load).

- file: internal/cmd/root_test.go   (P1.M4.T1.S1 — READ; reuse its helpers, do NOT edit)
  section: the helpers `saveRootState(t)`/`restoreRootState(t, …)` (capture/restore rootCmd's
       Out/Err/RunE + resetFlags), `loadEnvSetup(t) (home, repo, globalDir)`, `initRepo(t, dir)`,
       `setGitConfig(t, dir, key, value)`, `writeConfigFile(t, dir, relPath, body) string`, `chdir(t, dir)`.
       All `package cmd` (same package as S3's test) → directly callable.
  why: providers_test.go drives the FULL CLI in a temp repo: it needs a git repo (initRepo — config.Load
       Layer-4 needs one), CWD/HOME/XDG isolation (chdir + loadEnvSetup), and config files
       (writeConfigFile for .stagecoach.toml overrides). Reuse these — do NOT re-copy (same package).
  gotcha: rootCmd is a package-level singleton — each test MUST restore state in t.Cleanup via
       restoreRootState (SetArgs(nil), Out/Err, loadedCfg=nil, resetFlags) or tests poison each other
       (and trip -race). loadEnvSetup sets HOME+XDG to a temp dir (global config isolation).

- file: internal/exitcode/exitcode.go   (P1.M4.T1.S1 — READ; do NOT edit)
  section: `const Error = 1` + `func New(code int, err error) *ExitError` + `func For(err error) int`
       (nil→0; *ExitError→Code; generate-domain; else 1).
  why: S3 returns `exitcode.New(exitcode.Error, err)` on every failure; main calls exitcode.For → 1.
       A cobra arg-validation error (show with ≠1 args) is a plain error → For's default → 1. No
       NothingToCommit/Rescue/Timeout outcomes exist for providers list/show.
  gotcha: ExitError.Error()=="" when Err==nil → main skips printing. S3 always passes a NON-nil err
       (descriptive messages) so main prints `stagecoach: <msg>` — there's no detailed pre-print to
       de-duplicate, so the silent pattern (S2 §4) is NOT needed here.

- url: (PRD §9.11 FR46/FR47/FR48, §15.3, §12.8 — in context as selected_prd_content `h3.27`/`h3.54`/
       `h2.10`; ALSO plan/001_f1f80943ac34/prd_snapshot.md §9.11, §15.3, §12)
  why: §9.11 is the AUTHORITATIVE providers-list/show spec (FR46 list+detected+default; FR47 show TOML;
       FR48 override semantics). §15.3 restates the two subcommands. §12.8 documents user-defined
       providers (override built-in OR add new name).
  critical: FR46's "show the resolved default" = the provider that would be used (cfg.Provider else
       auto-detect) — NOT just DefaultProvider (design §4). FR47's "fully-resolved manifest" = the
       MERGED (built-in ⊕ override) manifest — which is exactly what Registry.MarshalTOML returns.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; cobra+pflag+go-toml/v2 (UNCHANGED)
cmd/stagecoach/main.go               # P1.M4.T1.S1 — os.Exit(exitcode.For(err)) (UNCHANGED by S3)
internal/
  cmd/root.go                       # P1.M4.T1.S1 — rootCmd + PersistentPreRunE + Config() + Execute + shouldSkipConfigLoad + STUB RunE  (S3 does NOT edit; init() in providers.go calls rootCmd.AddCommand)
  cmd/root_test.go                  # P1.M4.T1.S1 — helpers: saveRootState/restoreRootState/resetFlags/loadEnvSetup/initRepo/setGitConfig/writeConfigFile/chdir (REUSED by S3)
  exitcode/exitcode.go              # P1.M4.T1.S1 — For/New/ExitError + Error=1 (read-only ref)
  config/{config,file,git,load}.go  # P1.M1.T4 — Config.Providers + Load (read-only ref)
  provider/{registry,manifest,builtin,…}.go  # P1.M2 — Registry + Manifest + built-ins (read-only ref; S3's PRIMARY input)
  {generate,git,prompt,stubtest}/   # untouched by S3 (no generation/commit in scope)
pkg/stagecoach/stagecoach.go          # P1.M3.T5.S1 — buildDeps pattern to MIRROR (read-only ref)
Makefile                            # build/test(-race)/coverage/lint/clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/providers.go        # NEW — package cmd. providersCmd (group) + providersListCmd +
                                  #        providersShowCmd (leaves) + init() registering on rootCmd +
                                  #        runProvidersList/runProvidersShow RunE + helpers
                                  #        (newRegistry, installedNames, resolvedDefault,
                                  #        printProvidersList). Mode-A Short/Long help on all 3 cmds.
internal/cmd/providers_test.go   # NEW — package cmd. Integration tests via rootCmd/Execute reusing
                                  #        root_test.go helpers: built-in list, ✓/✗ detection, default
                                  #        marker, override appears+merges, show TOML round-trip,
                                  #        unknown→exit 1, arg-count validation.
# All other files UNCHANGED. root.go, internal/{config,generate,git,prompt,provider}, pkg/stagecoach,
# exitcode UNCHANGED. (S3 is parallel-safe with S2: disjoint files, same package.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (register via init(), design §0/§12): S3 does NOT edit root.go. providers.go defines
// `func init() { providersCmd.AddCommand(providersListCmd, providersShowCmd); rootCmd.AddCommand
// (providersCmd) }`. Go runs package-level init()s in source-file order, but order is irrelevant
// here (AddCommand is idempotent-enough: it just appends). This is parallel-safe with S2 (S2 edits
// root.go's RunE field; S3 adds a sibling file). Both compile into package cmd.

// CRITICAL (config MUST load, design §3): list/show are NOT in shouldSkipConfigLoad, so root's
// PersistentPreRunE runs and loads config (cfg.Providers = user overrides). SKIPPING config would
// show only built-ins — violating FR46/FR47. CONSEQUENCE: list/show run where config.Load succeeds.
// config.Load Layer-4 (loadGitConfig) shells `git -C <cwd> config --get <key>` per key: exit 1 =
// "missing key" (NOT an error, fine inside any repo even with zero stagecoach.* keys); exit 128 (not a
// repo) = hard error → config.Load fails → PersistentPreRunE returns exitcode.Error → subcommand
// never runs (exit 1). So run `providers list/show` INSIDE a git repo. This matches the rest of the
// CLI and the feature's need for user overrides. Do NOT add list/show to the skip-list to "fix" this.

// CRITICAL (resolved default formula, design §4): "show the resolved default" (FR46) = the provider
// stagecoach would USE with no --provider = cfg.Provider if non-empty, ELSE reg.DefaultProvider(
// installed). This is EXACTLY pkg/stagecoach.buildDeps's resolution. Do NOT show only
// DefaultProvider (ignores an explicit cfg.Provider). installed = installedNames(reg) (List+IsInstalled).

// CRITICAL (MarshalTOML ends in \n, design §6 — VERIFIED empirically): provider.Registry.MarshalTOML
// returns a TOML document whose LAST byte is '\n'. Print with `fmt.Fprint(os.Stdout, s)` — NOT
// Fprintln (which would add a second blank line). go-toml OMITS nil *string/*bool pointers (free
// omitempty: pi output has NO prompt_flag/json_field/retry_instruction/env rows). Empty slices →
// `subcommand = []`; explicit-empty *"" → `''` (e.g. pi `default_provider = ''`); single-quoted strings.

// GOTCHA (cobra inherits PersistentPreRunE): root has PersistentPreRunE; providers/list/show do NOT
// define their own → root's runs for them (cobra fires the nearest ancestor's). DO NOT add a
// PersistentPreRunE to any new command — it would SHADOW root's and skip config load.

// GOTCHA (cobra arg validation runs before RunE): show uses cobra.ExactArgs(1). With ≠1 args, cobra
// returns an arg error BEFORE RunE (and before PersistentPreRunE for the leaf). exitcode.For on a
// plain cobra error → default 1. SilenceErrors+SilenceUsage → cobra prints nothing; main prints
// `stagecoach: <msg>`. list uses cobra.NoArgs (it takes none).

// GOTCHA (IsInstalled probes DetectCommand, not Name): cursor's Detect="agent" (≠ Name "cursor") →
// cursor shows ✓ iff `agent` is on PATH. IsInstalled("") (a manifest with neither Detect nor Command)
// → false. A §12.8 user provider with only command="..." set → DetectCommand falls back to Command.

// GOTCHA (rootCmd singleton state): providers_test.go drives rootCmd directly via SetArgs/SetOut/SetErr.
// RESTORE state in t.Cleanup via restoreRootState (the existing helper) — SetArgs(nil), Out/Err,
// loadedCfg=nil, resetFlags — or tests poison each other (and trip -race). Mirror root_test.go hygiene.

// GOTCHA (stdout=data, stderr=diagnostics, design §10): list table + show TOML → STDOUT (scriptable:
// `stagecoach providers show pi > pi.toml`). Errors reach the user via main's `stagecoach: <err>` to
// STDERR (returned err is non-empty). Nothing on stdout on failure.

// GOTCHA (DecodeUserOverrides(nil) is safe): nil/empty cfg.Providers → DecodeUserOverrides returns
// (empty non-nil map, nil error) → NewRegistry = built-ins only. So `list`/`show` work with zero
// config. Guard Config()==nil defensively (use no overrides) though PersistentPreRunE guarantees
// non-nil for list/show.

// GOTCHA (default marker edge case, design §4): if cfg.Provider is set to a name NOT in List() (a
// misconfiguration), no row matches → no marker. Do NOT special-case; the generate path (buildDeps)
// is the authority on bad config. list just shows the table.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/cmd/providers.go
package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/provider"
)

// providersCmd is the PRD §15.3 "providers" command group. It has NO RunE → bare `stagecoach providers`
// prints help (cobra default). list/show are its leaves (registered in init()). Root's PersistentPreRunE
// is INHERITED (none of the three define their own) so config loads for both (FR46/FR47 need cfg.Providers).
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage AI provider manifests",
	Long: `Inspect the built-in and user-defined provider manifests Stagecoach uses to generate commits.

User-defined providers (from the global or repo-local config file) override built-ins of the same
name; new names add new providers (PRD §12.8).

Subcommands:
  list          List all known providers with detection + default status.
  show <name>   Print a provider's fully-resolved manifest as TOML.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List providers",
	Long: `List all known providers (built-in and user-defined).

Each provider is shown with:
  NAME      the provider name (a built-in, or from [provider.<name>] in config)
  DETECTED  ✓ if the provider's command is found on $PATH, ✗ otherwise
  DEFAULT   (default) marks the provider that will be used when no --provider is given
            (the configured provider, or the first detected built-in in preference order)

User-defined providers override built-ins of the same name; new names add new providers (PRD §12.8).`,
	Args:           cobra.NoArgs,
	SilenceErrors:  true,
	SilenceUsage:   true,
	RunE:           runProvidersList,
}

var providersShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a provider manifest",
	Long: `Print the fully-resolved manifest for <name> as TOML.

The manifest is the built-in definition merged with any user overrides from config (PRD §12.8).
Unknown provider names exit with code 1.`,
	Args:           cobra.ExactArgs(1),
	SilenceErrors:  true,
	SilenceUsage:   true,
	RunE:           runProvidersShow,
}

func init() {
	providersCmd.AddCommand(providersListCmd)
	providersCmd.AddCommand(providersShowCmd)
	rootCmd.AddCommand(providersCmd) // register on S1's root — NO edit to root.go (design §0)
}

// runProvidersList implements `stagecoach providers list` (FR46). It builds the merged registry from
// config, computes which providers are installed and which is the resolved default, and prints a
// NAME/DETECTED/DEFAULT table to stdout. Returns nil on success; exitcode.New(Error, …) on a registry-
// build failure (main maps to exit 1). Never calls os.Exit.
func runProvidersList(cmd *cobra.Command, args []string) error {
	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	installed := installedNames(reg)
	defaultName := resolvedDefault(Config(), reg, installed)
	printProvidersList(os.Stdout, reg, defaultName)
	return nil
}

// runProvidersShow implements `stagecoach providers show <name>` (FR47). It builds the merged registry
// and prints the TOML for <name> (built-in ⊕ overrides) to stdout. Unknown name → exitcode.New(Error,
// …) (exit 1). cobra.ExactArgs(1) guarantees args[0] exists. Never calls os.Exit.
func runProvidersShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	s, err := reg.MarshalTOML(name)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name))
	}
	fmt.Fprint(os.Stdout, s) // MarshalTOML output already ends in '\n' (design §6) — no extra newline
	return nil
}

// newRegistry builds the merged provider.Registry from config the SAME way pkg/stagecoach.buildDeps
// does (design §2): DecodeUserOverrides(cfg.Providers) → NewRegistry. This guarantees list/show
// reflect EXACTLY what GenerateCommit would use. A decode error (malformed [provider.X]) is returned
// (caller maps to exit 1). If Config() is nil (defensive; PersistentPreRunE guarantees non-nil for
// list/show), uses no overrides → built-ins only.
func newRegistry() (*provider.Registry, error) {
	var raw map[string]map[string]any
	if cfg := Config(); cfg != nil {
		raw = cfg.Providers
	}
	overrides, err := provider.DecodeUserOverrides(raw)
	if err != nil {
		return nil, fmt.Errorf("provider overrides: %w", err)
	}
	return provider.NewRegistry(overrides), nil
}

// installedNames returns the Names of providers whose command is on $PATH (FR46 detection). Mirrors
// pkg/stagecoach.buildDeps verbatim. reg.List() is sorted ascending, so the result is too.
func installedNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// resolvedDefault returns the provider stagecoach would use with no --provider (FR46 "show the resolved
// default"): cfg.Provider if explicitly configured (Layer 1-7), else reg.DefaultProvider(installed)
// (first preferred built-in on PATH). Mirrors pkg/stagecoach.buildDeps. A nil cfg (defensive) → auto.
func resolvedDefault(cfg *config.Config, reg *provider.Registry, installed []string) string {
	if cfg != nil && cfg.Provider != "" {
		return cfg.Provider
	}
	return reg.DefaultProvider(installed)
}

// printProvidersList renders the FR46 table to w (stdout): a header + one row per provider (sorted by
// Name via List()), with ✓/✗ in DETECTED and "(default)" on the row whose Name == defaultName. Uses
// text/tabwriter for aligned columns. Takes an io.Writer so P1.M4.T3 can restyle/recolor without
// touching the resolver (design §5). Exact spacing is tabwriter's concern; tests assert on NAME +
// marker substrings.
func printProvidersList(w io.Writer, reg *provider.Registry, defaultName string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tDETECTED\tDEFAULT")
	for _, m := range reg.List() {
		detected := "✗"
		if reg.IsInstalled(m) {
			detected = "✓"
		}
		marker := ""
		if m.Name == defaultName {
			marker = "(default)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Name, detected, marker)
	}
	tw.Flush()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/providers.go (the command group + RunE + helpers)
  - FILE: NEW internal/cmd/providers.go. PACKAGE: `package cmd`. Follow "Data models" skeleton.
  - DEFINE: providersCmd, providersListCmd, providersShowCmd (cobra.Command); func init() (registers
      them on rootCmd — NO edit to root.go); runProvidersList(cmd, args) error; runProvidersShow(cmd,
      args) error; newRegistry() (*provider.Registry, error); installedNames(*provider.Registry)
      []string; resolvedDefault(*config.Config, *provider.Registry, []string) string;
      printProvidersList(io.Writer, *provider.Registry, string).
  - IMPORTS: fmt, io, os, text/tabwriter, github.com/spf13/cobra,
      github.com/dustin/stagecoach/internal/config (ONLY for the resolvedDefault param type — or use
      *config.Config), github.com/dustin/stagecoach/internal/exitcode,
      github.com/dustin/stagecoach/internal/provider. (Confirm module path github.com/dustin/stagecoach.)
  - NAMING: providersCmd/providersListCmd/providersShowCmd/runProvidersList/runProvidersShow/newRegistry/
      installedNames/resolvedDefault/printProvidersList (unexported, package-level). PLACEMENT: all in
      internal/cmd/providers.go.
  - GOTCHA: init() calls rootCmd.AddCommand(providersCmd) — rootCmd is S1's package-level var (same
      package, directly visible). Do NOT edit root.go. Use `fmt.Fprint` for show output (MarshalTOML
      already ends in \n). Use `cobra.NoArgs` for list, `cobra.ExactArgs(1)` for show. ✓/✗ glyphs are
      literal Unicode (U+2713 / U+2717).
  - GOTCHA: read Config() (S1's store); guard nil defensively in newRegistry/resolvedDefault. Do NOT
      add PersistentPreRunE to any of the three commands (root's is inherited).

Task 2: CREATE internal/cmd/providers_test.go (integration tests through the FULL CLI)
  - FILE: NEW internal/cmd/providers_test.go. PACKAGE: `package cmd` (same as root_test.go).
  - REUSE root_test.go helpers (same package — do NOT re-copy): saveRootState, restoreRootState,
      loadEnvSetup, initRepo, setGitConfig, writeConfigFile, chdir. (resetFlags is called inside
      restoreRootState.)
  - STATE HYGIENE: each test wraps its body in `origArgs, origOut, origErr, origRunE := saveRootState(t);
      defer restoreRootState(t, origArgs, origOut, origErr, origRunE)`. Capture stdout/stderr via
      `var out bytes.Buffer; rootCmd.SetOut(&out); rootCmd.SetErr(&out-or-discard)`. Derive exit code via
      `exitcode.For(err)` (import internal/exitcode in the test).
  - COMMON SETUP helper (local to this file): `setupRepo(t) string` → loadEnvSetup(t) for HOME/XDG
      isolation, chdir(t, repo) into a fresh git repo (config.Load Layer-4 needs a repo). Returns repo
      dir. (Built-in list/show need NO config file; override tests use writeConfigFile(t, repo,
      ".stagecoach.toml", body).)
  - CASES (drive rootCmd via SetArgs + Execute(context.Background()); assert exitcode.For(err),
      captured stdout, and key substrings):
      * TestProvidersList_Builtins: setupRepo; SetArgs(["providers","list"]); Execute. Assert exit 0;
        stdout contains the header "NAME" + all 6 names (claude,codex,cursor,gemini,opencode,pi), each
        once; names appear in ascending order (index(claude)<index(codex)<…<index(pi)).
      * TestProvidersList_DetectedGlyphs: setupRepo; write .stagecoach.toml with TWO user providers:
        [provider.realbin] command="go" (installed in go test env) + [provider.fakebin]
        command="no-such-binary-xyz" ; SetArgs(["providers","list"]); Execute. Assert exit 0; the
        realbin row contains "✓" and the fakebin row contains "✗". (Avoids depending on whether
        pi/claude/etc. are installed on the host.)
      * TestProvidersList_DefaultMarker_Explicit: setupRepo; t.Setenv("STAGECOACH_PROVIDER","pi");
        SetArgs(["providers","list"]); Execute. Assert exit 0; the pi row contains "(default)" and it
        is the ONLY row with "(default)". (resolvedDefault honors cfg.Provider — design §4.)
      * TestProvidersList_DefaultMarker_Auto: setupRepo; write .stagecoach.toml [provider.detectedpi]
        command="pi" is NOT reliable (pi may be absent). Instead: assert that when STAGECOACH_PROVIDER
        is unset and NO preferred built-in is installed, NO row has "(default)" (DefaultProvider
        returns "" — design §4 edge case). SetArgs(["providers","list"]); Execute; assert exit 0 and
        stdout does NOT contain "(default)". (Deterministic on any host: if no built-in is on PATH,
        there's no auto-default. If the host DOES have a built-in, this assertion may flake — so
        instead assert: at most ONE "(default)" appears. Prefer the explicit-marker test above for
        determinism; keep this one lenient.)
      * TestProvidersList_OverrideAppears: setupRepo; write .stagecoach.toml [provider.myagent]
        command="/opt/agent"; SetArgs(["providers","list"]); Execute. Assert exit 0; stdout contains
        "myagent" (a brand-new §12.8 provider appears, sorted among the built-ins).
      * TestProvidersShow_BuiltInTOML: setupRepo; SetArgs(["providers","show","pi"]); Execute. Assert
        exit 0; stdout contains `name = 'pi'`, `command = 'pi'`, `default_model = 'glm-5-turbo'`,
        `output = 'raw'`, `strip_code_fence = true`. (The verified §6 shape.)
      * TestProvidersShow_OverrideMerged: setupRepo; write .stagecoach.toml `[provider.pi]\ndefault_model
        = "glm-5.2"`; SetArgs(["providers","show","pi"]); Execute. Assert exit 0; stdout contains
        `default_model = 'glm-5.2'` (FR47 — the override is reflected in the MERGED manifest); still
        contains `command = 'pi'` (untouched built-in field survives the merge).
      * TestProvidersShow_NewProviderTOML: setupRepo; write .stagecoach.toml `[provider.myagent]\ncommand
        = "/opt/agent"\nprompt_delivery = "stdin"`; SetArgs(["providers","show","myagent"]); Execute.
        Assert exit 0; stdout contains `name = 'myagent'` and `command = '/opt/agent'`.
      * TestProvidersShow_UnknownExits1: setupRepo; SetArgs(["providers","show","ghost"]); Execute.
        Assert exitcode.For(err)==1 (Error); the returned err's message contains `ghost`.
      * TestProvidersShow_MissingArgExits1: setupRepo; SetArgs(["providers","show"]); Execute. Assert
        exitcode.For(err)==1 (cobra ExactArgs(1) rejects 0 args).
      * TestProvidersShow_ExtraArgsExits1: setupRepo; SetArgs(["providers","show","a","b"]); Execute.
        Assert exitcode.For(err)==1 (cobra ExactArgs(1) rejects 2 args).
      * TestProvidersGroup_NoSubcommandPrintsHelp: setupRepo; SetArgs(["providers"]); var buf bytes.Buffer;
        rootCmd.SetOut(&buf); Execute. Assert exit 0; buf contains "list" and "show" (help lists the
        subcommands). (providersCmd has no RunE → cobra prints help.)
  - COVERAGE: list (builtins, glyphs, default explicit/auto, override appears) + show (builtin TOML,
      override merged, new provider, unknown→1, arg-count). Use bytes.Contains for substring checks
      (robust to tabwriter spacing). No stubtest/agent dependency (pure registry + LookPath).

Task 3: VALIDATE (run all gates; fix before declaring done)
  - `make build` → ./bin/stagecoach exists; `./bin/stagecoach providers list` (inside a git repo) prints
      the table; `./bin/stagecoach providers show pi` prints the pi TOML; `./bin/stagecoach providers
      show ghost` exits 1; `./bin/stagecoach providers` prints help.
  - `go test -race ./internal/cmd/ -v` → green (root_test.go + default_action_test.go [if S2 merged]
      + providers_test.go).
  - `go test -race ./...` → green (NO regression — internal/{config,generate,git,provider,prompt},
      pkg/stagecoach, exitcode untouched).
  - `go vet ./...` clean; `gofmt -l internal/cmd/` empty.
  - `git status` shows ONLY: new internal/cmd/providers.go, new internal/cmd/providers_test.go.
      (root.go UNCHANGED — verify with `git diff internal/cmd/root.go` = empty.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: build the registry the SAME way the generate pipeline does (design §2 — mirror buildDeps).
func newRegistry() (*provider.Registry, error) {
    var raw map[string]map[string]any
    if cfg := Config(); cfg != nil {
        raw = cfg.Providers // user [provider.<name>] overrides (raw map; nil => none)
    }
    overrides, err := provider.DecodeUserOverrides(raw) // raw map -> typed Manifests
    if err != nil {
        return nil, fmt.Errorf("provider overrides: %w", err)
    }
    return provider.NewRegistry(overrides), nil // built-ins ⊕ overrides
}

// PATTERN: resolved default = what stagecoach would use (design §4 — mirror buildDeps).
func resolvedDefault(cfg *config.Config, reg *provider.Registry, installed []string) string {
    if cfg != nil && cfg.Provider != "" {
        return cfg.Provider // explicit (Layer 1-7) wins
    }
    return reg.DefaultProvider(installed) // auto: first preferred built-in on PATH
}
// installed = installedNames(reg): for _, m := range reg.List() { if reg.IsInstalled(m) { append } }

// PATTERN: exit codes via returned errors; never os.Exit (design §7).
func runProvidersShow(cmd *cobra.Command, args []string) error {
    name := args[0]
    reg, err := newRegistry()
    if err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) // exit 1; main prints
    }
    s, err := reg.MarshalTOML(name)
    if err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name)) // exit 1; main prints
    }
    fmt.Fprint(os.Stdout, s) // ALREADY ends in '\n' (verified) — NOT Fprintln
    return nil // exit 0
}

// GOTCHA: register via init() in the NEW file — do NOT edit root.go (design §0/§12). Parallel-safe
// with S2 (disjoint files, same package). rootCmd is S1's package-level var, visible in providers.go.
//   func init() { providersCmd.AddCommand(providersListCmd, providersShowCmd); rootCmd.AddCommand(providersCmd) }

// GOTCHA: config loads for list/show (NOT in shouldSkipConfigLoad) — needed for cfg.Providers (FR46/47).
// Do NOT add a PersistentPreRunE to the new commands (it would shadow root's).

// GOTCHA: MarshalTOML omits nil pointers, prints `subcommand = []` for empty slices, `''` for *"",
// single-quoted strings, and ENDS in '\n'. Assert on key substrings in tests, not exact bytes.
```

### Integration Points

```yaml
ROOT.COMMAND (S1 → S3 registers via init(); NO edit):
  - rootCmd: "S1's package-level singleton. providers.go's init() calls rootCmd.AddCommand(providersCmd).
    root.go is UNCHANGED. Verified by `git diff internal/cmd/root.go` == empty."
  - gotcha: "do NOT edit root.go, do NOT add list/show to shouldSkipConfigLoad (S3 needs config)."

CONFIG.STORE (S1 → S3 reads):
  - Config(): "S1's PersistentPreRunE result (runs for list/show). newRegistry reads cfg.Providers;
    resolvedDefault reads cfg.Provider. nil-guarded defensively (unreachable in practice)."

PERSISTENT.PRERUNE (S1 → S3 inherits):
  - root's PersistentPreRunE: "INHERITED by providers/list/show (none define their own) → config loads.
    DO NOT add a PersistentPreRunE to the new commands — it would shadow root's and skip config."

REGISTRY (P1.M2.T3.S1 → S3 consumes, READ-ONLY):
  - provider.Registry: "List (sorted asc), IsInstalled (LookPath DetectCommand), DefaultProvider
    (first preferred built-in in installed), MarshalTOML (merged manifest, unknown→err). +
    DecodeUserOverrides (config.Providers → Manifests) + NewRegistry. S3 calls ONLY these."

EXIT.CODE (S1 → S3 returns errors it maps):
  - exitcode.New/Error/For: "S3 returns exitcode.New(exitcode.Error, err) on failure; main calls
    os.Exit(exitcode.For(err)) and prints `stagecoach: <err>`. S3 never calls os.Exit. Only 0/1 occur."

CLI.HELP (Mode-A docs, S3 owns):
  - Short/Long: "providers/providers list/providers show each have Short (shown in parent --help) +
    Long (shown on own --help). The list Long documents the 3 columns + default resolution; the show
    Long states TOML output + exit-1-on-unknown. THIS is the user-facing docs deliverable."

UI (forward — P1.M4.T3):
  - printProvidersList(w io.Writer, …): "S3 writes plain ✓/✗ glyphs to an io.Writer. P1.M4.T3 may
    colorize (green ✓/red ✗) when TTY + !NoColor by wrapping/restyling the writer. S3 = the DATA."
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating providers.go - fix before proceeding
go build ./internal/cmd/ ./cmd/stagecoach/
gofmt -w internal/cmd/providers.go
go vet ./internal/cmd/

# Expected: zero errors. If `go build` reports rootCmd undefined, you're not in package cmd / wrong
# import path. If gofmt rewrites the ✓/✗ lines, that's fine (it preserves Unicode). govet: none.
# CRITICAL self-check: `git diff --stat internal/cmd/root.go` MUST be empty (S3 does not edit root.go).
```

### Level 2: Unit/Integration Tests (the FULL CLI)

```bash
# providers list/show tests drive the real cobra rootCmd (no agent/stub needed).
go test -race ./internal/cmd/ -run TestProviders -v

# Full cmd package (S1 root_test.go + [S2 default_action_test.go if merged] + S3 providers_test.go)
go test -race ./internal/cmd/ -v

# Expected: all green. If TestProvidersList_DetectedGlyphs flakes, ensure `go` is on PATH (it is in
# `go test`); the fakebin command is deliberately absent. If a test poisons another, check
# restoreRootState is deferred in EVERY test (singleton hygiene).
```

### Level 3: Integration Testing (Binary Validation)

```bash
# Build the real binary
make build

# Inside a git repo (config.Load Layer-4 needs one):
cd /tmp/scratchrepo && git init -q && git config user.name t && git config user.email t@e && \
  /home/dustin/projects/stagecoach/bin/stagecoach providers list
# Expected: a NAME/DETECTED/DEFAULT table with the 6 built-ins (claude,codex,cursor,gemini,opencode,pi)
# sorted ascending, each ✓ or ✗, and "(default)" on the resolved-default row (pi if pi is on PATH and
# no provider configured). Exit 0.

# show a built-in manifest as TOML
/home/dustin/projects/stagecoach/bin/stagecoach providers show pi
# Expected: TOML on stdout: `name = 'pi'`, `command = 'pi'`, `default_model = 'glm-5-turbo'`, …,
# ending in a newline. Exit 0.

# unknown provider → exit 1
/home/dustin/projects/stagecoach/bin/stagecoach providers show ghost; echo "exit=$?"
# Expected: stderr `stagecoach: unknown provider "ghost"`, exit=1.

# bare `providers` → help (lists list/show)
/home/dustin/projects/stagecoach/bin/stagecoach providers
# Expected: help text naming the list and show subcommands. Exit 0.

# override is reflected (FR47/FR48): write a repo-local override and re-show
printf '[provider.pi]\ndefault_model = "glm-5.2"\n' > /tmp/scratchrepo/.stagecoach.toml
/home/dustin/projects/stagecoach/bin/stagecoach providers show pi | grep default_model
# Expected: `default_model = 'glm-5.2'` (the override merged onto the built-in).

# (Outside a git repo, list/show exit 1 — config.Load Layer-4 hard-fails. Expected per design §3.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Pipe-ability: show output is valid TOML (round-trips through a TOML parser).
/home/dustin/projects/stagecoach/bin/stagecoach providers show pi > /tmp/pi.toml && \
  go run -mod=mod github.com/pelletier/go-toml/v2/cmd/tomljson@latest /tmp/pi.toml >/dev/null && \
  echo "valid TOML"
# (Or assert with a tiny Go snippet using toml.Unmarshal — the providers_test.go show tests already do
# substring checks; this is a human sanity check that the doc is machine-parseable.)

# list is grep-able by name (NAME column is first):
/home/dustin/projects/stagecoach/bin/stagecoach providers list | grep '^pi'
# Expected: the pi row (NAME in column 1; the leading-name design makes `^name` work despite the header).

# Default precedence: an explicit provider changes the marker.
STAGECOACH_PROVIDER=claude /home/dustin/projects/stagecoach/bin/stagecoach providers list | grep claude
# Expected: the claude row now carries "(default)" (resolvedDefault honors cfg.Provider).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./internal/cmd/ -v` (and `go test -race ./...` shows no regression)
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/cmd/` empty
- [ ] **root.go UNCHANGED**: `git diff --stat internal/cmd/root.go` is empty (registration via init())

### Feature Validation

- [ ] All success criteria from "What" section met
- [ ] `stagecoach providers list` prints the NAME/DETECTED/DEFAULT table (header + 6 built-ins sorted)
- [ ] `stagecoach providers show pi` prints the merged pi manifest as TOML (ends in newline)
- [ ] ✓/✗ reflect exec.LookPath detection; cursor probes `agent` (Detect≠Name)
- [ ] `(default)` marks the resolved default (cfg.Provider if set, else auto-detect)
- [ ] User `[provider.X]` overrides appear in list and are reflected in show (FR48 merge)
- [ ] Unknown provider name exits 1 with `stagecoach: unknown provider "<name>"`
- [ ] `show` with ≠1 args exits 1 (cobra ExactArgs); bare `providers` prints help
- [ ] Mode-A help text documents the output format on each command's --help

### Code Quality Validation

- [ ] Follows existing codebase patterns (registry-build mirrors buildDeps; exitcode centralization;
      rootCmd singleton hygiene in tests)
- [ ] File placement matches desired codebase tree (2 NEW files in internal/cmd/)
- [ ] Anti-patterns avoided (no os.Exit in RunE; no shadowing PersistentPreRunE; no root.go edit)
- [ ] Dependencies properly managed (only stdlib + cobra + internal/{config,exitcode,provider})
- [ ] No new external dependencies

### Documentation & Deployment

- [ ] Short/Long help text is self-documenting (Mode-A deliverable)
- [ ] No new environment variables or config keys introduced
- [ ] stdout = data (table/TOML), stderr = diagnostics (via main) — preserves pipe use cases

---

## Anti-Patterns to Avoid

- ❌ Don't edit root.go to register the command — use `init()` in the new file (design §0/§12; avoids a
  parallel-merge hazard with S2).
- ❌ Don't add `list`/`show` to `shouldSkipConfigLoad` — they NEED config (user overrides, FR46/FR47).
- ❌ Don't add a `PersistentPreRunE` to providers/list/show — it shadows root's and skips config load.
- ❌ Don't invent a new registry-build or default-resolution sequence — mirror `pkg/stagecoach.buildDeps`
  verbatim (DecodeUserOverrides+NewRegistry; cfg.Provider else DefaultProvider(installed)).
- ❌ Don't call `os.Exit` in a RunE — return an error; main maps it via `exitcode.For`.
- ❌ Don't use `fmt.Fprintln` for the show output — MarshalTOML already ends in `\n` (double newline).
- ❌ Don't call Validate/Resolve on manifests — the registry stores merged-but-unresolved manifests and
  MarshalTOML prints them as-is (lets `show` display a partially-defined provider for debugging).
- ❌ Don't assert on exact byte output in tests — use substring checks (tabwriter spacing + go-toml
  formatting are not part of the contract; NAME + key substrings are).
- ❌ Don't depend on pi/claude/etc. being installed in tests — use `go` (guaranteed in `go test`) vs a
  bogus binary for deterministic ✓/✗ assertions.
