---
name: "P1.M4.T1.S4 — config init/path subcommands — PRD §9.8 (FR38) / §15.3 / §16.2"
description: |

  Add a `config` command group with two leaf subcommands (PRD §15.3): `stagecoach config init`
  (FR38 — writes a fully-commented example TOML config to the resolved GLOBAL config path, creating
  parent dirs, refusing to overwrite an existing file) and `stagecoach config path` (FR38 — prints the
  resolved GLOBAL config path, one line, to stdout). The written template IS the Mode-A user-facing
  config documentation: it documents §16.2 sections ([defaults], [generation], [provider.X]), the
  §9.8 precedence order (FR34), STAGECOACH_* env vars (FR35), and `stagecoach.*` git-config keys
  (FR36/§16.3), with every option line commented out so the file is INERT until edited.

  This subtask does NOT generate, parse, commit, touch git, or resolve a Config. Both leaves SKIP
  config loading entirely (S1's `shouldSkipConfigLoad` already returns true for cmd names "init"/"path"),
  so they work ANYWHERE — including OUTSIDE a git repo and on a machine with no config.

  INPUT (upstream — `globalConfigPath()` from P1.M1.T4.S2): the path resolver is UNEXPORTED in
  `internal/config/file.go`, so the ONE edit to an existing file is to ADD a thin exported wrapper
  `GlobalConfigPath()` there (delegating to the existing unexported func — zero behavior change). This
  keeps `config path` and `config.Load` sharing a single source of truth for "where is the global file?"
  (DRY-critical). The `cmd` package then calls `config.GlobalConfigPath()`.

  DELIVERABLES (1 small EDIT + 2 NEW files):
    1. EDIT   `internal/config/file.go`  — ADD `func GlobalConfigPath() string { return globalConfigPath() }`
       (one exported wrapper; ~5 lines incl. doc comment). ADDITIVE — no existing func changes.
    2. CREATE `internal/cmd/config.go`    — `package cmd`. `configCmd` (group) + `configInitCmd`/
       `configPathCmd` (leaves) + `init()` registering them on `rootCmd` + `runConfigInit`/
       `runConfigPath` RunE + the `exampleConfigTemplate` `const` string (Mode-A docs). NO root.go edit.
    3. CREATE `internal/cmd/config_test.go` — `package cmd`. Tests via the FULL CLI (`rootCmd`/`Execute`)
       reusing root_test.go helpers: `path` prints the resolved global path (temp HOME/XDG); `init`
       writes the template + creates parent dir; written template is inert (all lines `#`-commented);
       `init` refuses to overwrite an existing file (exit 1); both work OUTSIDE a git repo (exit 0).

  CONTRACT (PRD §9.8 FR38, §15.3, §16.2, Mode-A docs):
    - `config path` → print `config.GlobalConfigPath()` to STDOUT, one line + newline. Exit 0.
    - `config init` → compute `config.GlobalConfigPath()`; if the file ALREADY EXISTS, return
      `exitcode.New(exitcode.Error, …("already exists…not overwritten"))` → exit 1 (non-destructive);
      else `os.MkdirAll(filepath.Dir(path), 0o755)`, `os.WriteFile(path, template, 0o644)`, print
      "Wrote example config to <path>" to STDOUT. Exit 0.
    - The written template: every option commented (`#`); header documents precedence (FR34), env
      vars (FR35), git keys (FR36/§16.3); sections [defaults]/[generation]/[provider.X] per §16.2 with
      accurate field names (from manifest.go toml tags) and documented default values.
    - Exit codes: success 0; any failure (mkdir/write/already-exists/arg-error) 1. Routed via
      `exitcode.New(exitcode.Error, err)`; NEVER `os.Exit` (main owns that via exitcode.For).

  SCOPE BOUNDARY (owned by siblings — do NOT implement): the default commit action (S2 — root RunE);
  `providers list/show` (S3); signal handling (P1.M4.T2); color/TTY restyling of the init/path output
  (P1.M4.T3 — S4 keeps runConfigPath/runConfigInit writing plain text so they can be restyled later);
  dry-run (P1.M4.T4). S4 does NOT modify `shouldSkipConfigLoad` — it is already correct for init/path.

  ⚠️ S4 does NOT edit root.go — registration is via `init()` in the NEW config.go (mirrors S3's
     providers.go; design D2). shouldSkipConfigLoad already returns true for "init"/"path" (S1 wired it).
  ⚠️ `globalConfigPath()` is UNEXPORTED → S4 adds `GlobalConfigPath()` to file.go (the one additive
     edit). Do NOT duplicate the XDG/home path logic in cmd (two sources of truth would drift).
  ⚠️ `config path` uses the DISCOVERED global path (GlobalConfigPath = XDG or ~/.config/...), NOT the
     --config/STAGECOACH_CONFIG override (those select a READ path for Load; FR38 targets the global
     location). `config init` writes to that same global path.
  ⚠️ `config init` is NON-DESTRUCTIVE: an existing file is NEVER overwritten (exit 1 with a message).
     No --force in scope.

  Deliverable: 1 edited file + 2 NEW files. `make build` → `./bin/stagecoach config path` prints
  `/home/.../.config/stagecoach/config.toml` (or `$XDG_CONFIG_HOME/stagecoach/config.toml`); `./bin/stagecoach
  config init` writes the commented template there and prints "Wrote example config to …"; a second
  `config init` exits 1 with "stagecoach: config file already exists at … (not overwritten)". Both work
  outside any git repo. `go test -race ./internal/cmd/` green; `go test -race ./...` no regression.

---

## Goal

**Feature Goal**: Ship Stagecoach's config-file bootstrap + discovery CLI surface (PRD §9.8 FR38 /
§15.3) — a `config` command group whose `init` leaf writes a fully-commented, INERT example TOML
config (documenting §16.2 sections, §9.8 precedence, STAGECOACH_* env vars, and stagecoach.* git keys)
to the resolved GLOBAL config path (creating parent dirs, refusing to overwrite), and whose `path`
leaf prints that resolved global path (one line, scriptable), both as thin views over the P1.M1.T4.S2
`globalConfigPath()` resolver (newly exported as `config.GlobalConfigPath()`), with Mode-A help text,
§15.4 exit codes (0 success / 1 failure) routed through S1's centralized `exitcode`, and the ability
to run OUTSIDE any git repository.

**Deliverable** (1 small EDIT + 2 NEW files):
1. EDIT `internal/config/file.go` — add exported `func GlobalConfigPath() string` (one-line wrapper
   delegating to the existing unexported `globalConfigPath()`; additive, zero behavior change).
2. CREATE `internal/cmd/config.go` — `package cmd`. `configCmd` (group) + `configInitCmd` +
   `configPathCmd` cobra commands; `init()` registering them on `rootCmd`; `runConfigInit`/
   `runConfigPath` RunE functions; the `exampleConfigTemplate` `const` string (the Mode-A user-facing
   config documentation). Mode-A `Short`/`Long` help text on all three commands.
3. CREATE `internal/cmd/config_test.go` — `package cmd`. Integration tests driving the FULL CLI
   (`rootCmd`/`Execute`) reusing root_test.go helpers: `path` prints the resolved global path;
   `init` writes the inert template + creates the parent dir; written file is all-commented;
   `init` refuses to overwrite (exit 1); both run outside a git repo (exit 0); arg validation.

**Success Definition**: `make build` → `./bin/stagecoach config path` prints exactly one line ending in
`/config.toml` (the XDG or `~/.config/stagecoach/config.toml` path), exit 0; `./bin/stagecoach config init`
prints "Wrote example config to <that path>" and creates the file at that path whose content is the
commented template (every option line begins with `#`); `cat $(./bin/stagecoach config path)` shows the
`[defaults]`, `[generation]`, and `[provider.X]` sections all commented out, plus a header explaining
precedence, STAGECOACH_* env vars, and `stagecoach.*` git keys; a second `./bin/stagecoach config init`
exits 1 with `stagecoach: config file already exists at <path> (not overwritten)` and leaves the file
UNCHANGED; both `config path` and `config init` succeed when run from a directory that is NOT a git
repo; `./bin/stagecoach config` (no subcommand) prints help naming `init` and `path`. `go test -race
./internal/cmd/` green; `go test -race ./...` shows NO regression; `go vet ./...` clean; `gofmt -l
internal/cmd/ internal/config/` empty; only the 3 listed files changed (root.go UNCHANGED).

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 "the plan-holder" / "the API-key refusenik" / "the
multi-agent tinkerer") who wants to (a) discover WHERE Stagecoach reads its global config from, and
(b) bootstrap a well-documented, commented starting config they can edit — without reading the docs.

**Use Case**: `stagecoach config path` (where do I put my global config?); `stagecoach config init`
(scaffold a commented config I can uncomment options into); then `vi $(stagecoach config path)`.

**User Journey**: user runs `stagecoach config path` → sees the path → runs `stagecoach config init` →
opens the written file → uncomments the lines they want (e.g. `[provider.pi] default_model = …`) →
re-runs `stagecoach config init` to confirm it refused to clobber their edits (exit 1, unchanged).

**Pain Points Addressed**: (1) discoverability of the config location (XDG vs ~/.config is non-obvious
— `config path` answers it); (2) "what options exist?" — the written template is the documentation
(FR38); (3) "will init destroy my config?" — no, it refuses to overwrite (D4, safety).

## Why

- **Closes the config-management surface (PRD §9.8 FR38).** Without `config init`/`config path`, a user
  has no in-CLI way to find the global path or scaffold a valid config. This is the P1 ship-list item
  "config init/path" (PRD §10.1) and the last piece of P1.M4.T1's command structure.
- **The template IS the documentation (Mode-A).** Per the work item's DOCS requirement, the written
  commented config is the primary user-facing config reference — it documents every §16.2 section, the
  §9.8 precedence, env vars, and git keys inline, where the user edits. This is "docs at the point of
  action."
- **Reuses the proven path resolver, zero new semantics.** S4 only exports + calls
  `globalConfigPath()`; it adds no new path logic. What `config path` prints and `config init` writes
  is EXACTLY where `config.Load` reads the global layer from (D1 — single source of truth).
- **Non-destructive by design (D4).** `config init` refuses to overwrite, so a user's hand-edited
  config is never clobbered by a re-run. This is the safe default; the only way to "reset" is to
  delete the file first.
- **Runs anywhere (D8).** Both leaves skip config loading (shouldSkipConfigLoad), so they work before
  any git repo exists, in any directory, on a fresh machine — exactly when a user first bootstraps.

## What

A `config` cobra command group (parent shows help) with two leaves:
- `path` (`cobra.NoArgs`): computes `config.GlobalConfigPath()` and prints it to stdout (one line +
  newline). Returns nil. Exit 0.
- `init` (`cobra.NoArgs`): computes `config.GlobalConfigPath()`; if the file already exists, returns
  `exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)",
  path))` (exit 1, non-destructive); else `os.MkdirAll(filepath.Dir(path), 0o755)`, writes the
  `exampleConfigTemplate` to the path with `0o644`, prints "Wrote example config to <path>" to stdout.
  Returns nil on success. Exit 0.

Both RunE funcs return `nil` on success or an `*exitcode.ExitError` on failure. Neither calls
`os.Exit`. Neither reads `Config()` (config load is skipped for them).

### Success Criteria

- [ ] `internal/config/file.go` has an exported `func GlobalConfigPath() string` that returns
      `globalConfigPath()` (delegates; the unexported func + all its callers are UNCHANGED).
- [ ] `internal/cmd/config.go` exists, `package cmd`, imports `fmt`+`os`+`path/filepath`+
      `github.com/spf13/cobra` + `github.com/dustin/stagecoach/internal/{config,exitcode}`.
- [ ] `configCmd` (Use "config"), `configInitCmd` (Use "init", Args NoArgs), `configPathCmd` (Use
      "path", Args NoArgs) are defined; each has a `Short` and a `Long`.
- [ ] `func init()` does `configCmd.AddCommand(configInitCmd, configPathCmd)` then
      `rootCmd.AddCommand(configCmd)`. root.go is NOT edited.
- [ ] `runConfigPath(cmd, args) error`: prints `config.GlobalConfigPath()` to stdout (one line);
      returns nil.
- [ ] `runConfigInit(cmd, args) error`: resolves path; if `os.Stat` shows it exists → returns
      `exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)",
      path))`; else `MkdirAll(filepath.Dir(path), 0o755)` + `WriteFile(path, template, 0o644)` → on
      error returns `exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))`;
      else prints "Wrote example config to <path>" to stdout; returns nil.
- [ ] `exampleConfigTemplate` is a `const string`: header comments (precedence FR34, STAGECOACH_* env
      vars FR35, `stagecoach.*` git keys FR36/§16.3); `[defaults]`, `[generation]`, `[provider.X]`
      sections per §16.2 with EVERY option line `#`-commented; provider field names match
      `internal/provider/manifest.go` toml tags.
- [ ] Neither RunE calls `os.Exit`; both return errors consumable by `exitcode.For`.
- [ ] No `PersistentPreRunE` added to any of the three commands (root's is inherited — but skipped via
      shouldSkipConfigLoad for init/path; irrelevant either way).
- [ ] `go test -race ./internal/cmd/` green; `go test -race ./...` NO regression; `go vet ./...` clean;
      `gofmt -l internal/cmd/ internal/config/` empty; only `file.go` (1-line edit) + `config.go` +
      `config_test.go` changed (root.go UNCHANGED).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
upstream signatures (all quoted below + in research/design-decisions.md), the 9 design decisions, the
PRD §9.8/§15.3/§16.2 contracts (in `selected_prd_content`), the copy-ready template + skeletons in the
Implementation Blueprint, and the test conventions to mirror (`internal/cmd/root_test.go`). No
generation/commit/signal/UI knowledge required (explicitly out of scope).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T1S4/research/design-decisions.md
  why: the 9 decisions + 6 findings specific to this subtask. F1 (globalConfigPath is UNEXPORTED → add
       GlobalConfigPath wrapper — the ONE edit to file.go), F2 (shouldSkipConfigLoad ALREADY wired for
       init/path by S1 → no root.go edit, works outside a repo), F3 (manifest toml tags for the template),
       F4 (config-file decode shape + default values), F5 (test helpers reused from root_test.go), F6
       (exit-code + stdout/stderr conventions). D1–D9 map to the contract clauses.
  critical: F1 (WHY export, not duplicate), F2 (WHY no root.go edit + works outside a repo), D4
       (refuse-overwrite), D5 (path → stdout, discovered not override).

- docfile: plan/001_f1f80943ac34/P1M4T1S3/PRP.md   (the SIBLING PATTERN — S4 mirrors it 1:1)
  section: "Implementation Blueprint > Data models and structure" (providers.go skeleton: command group
       + leaves + init() + RunE + helpers) and "Known Gotchas" (register via init() in NEW file; do NOT
       edit root.go; do NOT add a PersistentPreRunE; SilenceErrors+SilenceUsage; never os.Exit; exit via
       exitcode.New(exitcode.Error, …)).
  why: S3's `providers.go` is the EXACT template S4's `config.go` copies — same shape (group + 2 leaves
       registered via init() on rootCmd), same exit-code routing, same Mode-A help-text approach, same
       test pattern (drive rootCmd/Execute + reuse root_test.go helpers). S4 differs only in: (a) it also
       ADDS GlobalConfigPath to file.go; (b) its RunE bodies write a file / print a path instead of
       querying a registry; (c) its leaves are in shouldSkipConfigLoad (skip config load) whereas S3's
       list/show are NOT (they need config).
  pattern: copy providers.go's cobra.Command struct shape (Use/Short/Long/Args/SilenceErrors/
       SilenceUsage/RunE), the `func init()` registration, the RunE-signature `func(cmd *cobra.Command,
       args []string) error`, and the `exitcode.New(exitcode.Error, fmt.Errorf(…))` failure path.
  gotcha: S4's init/path ARE skipped by shouldSkipConfigLoad (they must NOT load config — they work
       outside a repo), UNLIKE S3's list/show. Do NOT remove them from the skip-list; S1 already put
       them there for S4.

- file: internal/config/file.go   (P1.M1.T4.S2 — globalConfigPath; S4 EDITS this ONE file additively)
  section: `func globalConfigPath() string` (lines ~56-70): XDG_CONFIG_HOME (if set AND absolute) →
       `$XDG/stagecoach/config.toml`; else `os.UserHomeDir()` → `$home/.config/stagecoach/config.toml`;
       last-resort `"config.toml"` (CWD) if UserHomeDir errors. UNEXPORTED.
  why: THIS is the func S4 must consume but cannot (cross-package). S4 ADDS directly above or below it:
       `// GlobalConfigPath returns the resolved global Stagecoach config path (PRD §16.1 layer 2) — the
       // file `config init` writes and `config path` prints. func GlobalConfigPath() string { return
       globalConfigPath() }`. The unexported func STAYS (file_test.go + load.go call it).
  pattern: ADDITIVE edit only. Do NOT move/rename/rewrite globalConfigPath — just add the exported alias.
  gotcha: `load.go` calls `globalConfigPath()` (unexported) for Layer-2 discovery; keep that working.
       The export wrapper MUST return the SAME value (delegate, don't re-implement) or `config path`
       and Load will disagree on the global location.

- file: internal/cmd/root.go   (P1.M4.T1.S1 — S4 calls rootCmd.AddCommand via init(); do NOT edit)
  section: `var rootCmd = &cobra.Command{Use:"stagecoach", SilenceErrors:true, SilenceUsage:true, …,
       PersistentPreRunE: <if shouldSkipConfigLoad(cmd) return nil; else config.Load…>}` + `func
       shouldSkipConfigLoad(cmd) bool { name := cmd.Name(); return name == "init" || name == "path" }`
       + `func Execute(ctx) error`.
  why: S4's `init()` calls `rootCmd.AddCommand(configCmd)`. Because configInitCmd.Name()=="init" and
       configPathCmd.Name()=="path", shouldSkipConfigLoad returns true for BOTH → PersistentPreRunE
       returns nil immediately (no config.Load, no git shell-out). So init/path run ANYWHERE.
  pattern: rootCmd is a package-level singleton; init() registration is idempotent-enough (AddCommand
       appends). SilenceErrors+SilenceUsage mean cobra prints nothing on error; main prints
       `stagecoach: <err>` when err.Error() != "". S4 returns errors that interplay with both.
  gotcha: do NOT edit root.go. do NOT add init/path to shouldSkipConfigLoad (already there). do NOT
       give config/init/path their own PersistentPreRunE.

- file: internal/cmd/root_test.go   (P1.M4.T1.S1 — READ; reuse its helpers, do NOT edit)
  section: the helpers `saveRootState(t)`/`restoreRootState(t, …)` (capture/restore rootCmd Out/Err/RunE
       + resetFlags), `loadEnvSetup(t) (home, repo, globalDir)` (sets HOME=XDG_CONFIG_HOME=temp dir →
       GlobalConfigPath()==`<tmp>/stagecoach/config.toml`), `writeConfigFile`, `chdir`.
  why: config_test.go drives the FULL CLI in isolation: it needs CWD/HOME/XDG isolation (chdir +
       loadEnvSetup or t.Setenv). Because init/path SKIP config load, tests do NOT need a git repo —
       chdir into a plain `t.TempDir()` (no initRepo) to prove the outside-a-repo case.
  pattern: each test wraps its body in `origArgs, origOut, origErr, origRunE := saveRootState(t); defer
       restoreRootState(t, origArgs, origOut, origErr, origRunE)`. Capture stdout via `var out bytes.Buffer;
       rootCmd.SetOut(&out)`. Derive exit code via `exitcode.For(err)`.
  gotcha: rootCmd is a package-level singleton — restore state in t.Cleanup via restoreRootState or tests
       poison each other (and trip -race). loadEnvSetup returns globalDir=`<home>/stagecoach` (the parent
       of config.toml) — handy for asserting where init wrote.

- file: internal/provider/manifest.go   (P1.M2.T1.S1 — toml tags for the template; READ, do NOT edit)
  section: `type Manifest struct` toml tags: name, detect, command, subcommand, prompt_delivery
       (stdin|positional|flag), prompt_flag, print_flag, model_flag, default_model, system_prompt_flag,
       provider_flag, default_provider, bare_flags, output (raw|json), json_field, strip_code_fence,
       retry_instruction, env.
  why: the `exampleConfigTemplate`'s commented `[provider.X]` example MUST use these EXACT field names
       (the template is user-facing docs — wrong names would mislead users into invalid configs). Use the
       §16.2 subset (command, prompt_delivery, print_flag, model_flag, default_model, system_prompt_flag,
       default_provider, bare_flags, output) — all valid against manifest.go.
  pattern: the §16.2 `[provider.myagent]` example is the canonical "define a new provider" block; copy
       its fields into the commented template verbatim (they all match manifest.go tags).
  gotcha: do NOT invent field names not in manifest.go (e.g. there is no `args` or `timeout` in a
       provider manifest). `[provider.pi]` override example: `default_model`, `default_provider` only.

- file: internal/config/config.go   (P1.M1.T4.S1 — Defaults() for the template's documented values; READ)
  section: `func Defaults() Config` returns: Timeout 120s, AutoStageAll true, MaxDiffBytes 300000,
       MaxMdLines 100, MaxDuplicateRetries 3, SubjectTargetChars 50, Output "raw", StripCodeFence true,
       Provider/Model/Verbose/NoColor "" or false. `fileConfig` (file.go) decode shape: [defaults]
       {provider, model, timeout(string), auto_stage_all, verbose}; [generation] {max_diff_bytes,
       max_md_lines, max_duplicate_retries, subject_target_chars, output, strip_code_fence}.
  why: the template's commented `[defaults]`/`[generation]` lines should show these as the documented
       default values (so a user uncommenting gets sane defaults), matching §16.2 exactly.
  gotcha: `timeout` in the FILE is a STRING ("120s"), NOT a bare number — the template must show
       `timeout = "120s"` (quoted), because fileConfig.Defaults.Timeout is a string decoded by
       time.ParseDuration in loadTOML. (The resolved Config.Timeout is a time.Duration, but the FILE
       shape is a string.)

- file: internal/exitcode/exitcode.go   (P1.M4.T1.S1 — READ; do NOT edit)
  section: `const Error = 1` + `func New(code int, err error) *ExitError` + `func For(err error) int`
       (nil→0; *ExitError→Code; else 1).
  why: S4 returns `exitcode.New(exitcode.Error, err)` on every failure; main calls exitcode.For → 1.
       A cobra arg-validation error (init/path with args) is a plain error → For's default → 1. Only 0/1
       occur for config init/path (no NothingToCommit/Rescue/Timeout outcomes).
  gotcha: ExitError.Error()=="" when Err==nil → main skips printing. S4 always passes a NON-nil err
       (descriptive messages) so main prints `stagecoach: <msg>`.

- url: (PRD §9.8 FR38, §15.3, §16.2 — in context as selected_prd_content `h3.24`/`h3.54`/`h3.58`;
       ALSO plan/001_f1f80943ac34/prd_snapshot.md §9.8, §15.3, §16)
  why: §9.8 FR38 is the AUTHORITATIVE spec (config init writes a commented example to the global path;
       config path prints the resolved global config path). §15.3 restates the two subcommands. §16.2 is
       the example config STRUCTURE the template must mirror ([defaults]/[generation]/[provider.X]).
  critical: FR38 "commented example config" ⇒ every option line `#`-commented (INERT until edited).
       §16.2's `[provider.pi]` + `[provider.myagent]` blocks are the provider examples to include.
       The header must also document §9.8 precedence (FR34), env vars (FR35), git keys (FR36) per the
       work item's DOCS requirement.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; cobra+pflag+go-toml/v2 (UNCHANGED)
cmd/stagecoach/main.go               # P1.M4.T1.S1 — os.Exit(exitcode.For(err)) (UNCHANGED by S4)
internal/
  config/file.go                    # P1.M1.T4.S2 — globalConfigPath() UNEXPORTED  (S4 EDITS: +GlobalConfigPath wrapper)
  config/{config,git,load}.go       # P1.M1.T4 — Config + Load + git reader (UNCHANGED by S4)
  cmd/root.go                       # P1.M4.T1.S1 — rootCmd + shouldSkipConfigLoad (already true for init/path)  (S4 does NOT edit; init() in config.go calls rootCmd.AddCommand)
  cmd/root_test.go                  # P1.M4.T1.S1 — helpers: saveRootState/restoreRootState/resetFlags/loadEnvSetup/writeConfigFile/chdir (REUSED by S4)
  cmd/providers.go                  # P1.M4.T1.S3 — the PATTERN to mirror (group + leaves + init())  (read-only ref)
  exitcode/exitcode.go              # P1.M4.T1.S1 — For/New/ExitError + Error=1 (read-only ref)
  provider/manifest.go              # P1.M2.T1.S1 — toml tags for the template (read-only ref)
  {generate,git,prompt,stubtest}/   # untouched by S4 (no generation/commit in scope)
pkg/stagecoach/stagecoach.go          # P1.M3.T5.S1 (untouched by S4)
Makefile                            # build/test(-race)/coverage/lint/clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/config/file.go          # EDIT (additive): +func GlobalConfigPath() string { return globalConfigPath() }
internal/cmd/config.go           # NEW — package cmd. configCmd (group) + configInitCmd/configPathCmd
                                  #        (leaves) + init() registering on rootCmd + runConfigInit/
                                  #        runConfigPath RunE + exampleConfigTemplate const (Mode-A docs).
internal/cmd/config_test.go      # NEW — package cmd. Integration tests via rootCmd/Execute reusing
                                  #        root_test.go helpers: path prints global path; init writes
                                  #        inert template + parent dir; refuse-overwrite (exit 1);
                                  #        both run outside a git repo; arg validation.
# All other files UNCHANGED. root.go UNCHANGED. providers.go UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (export the resolver, design D1/F1): globalConfigPath() is UNEXPORTED in internal/config.
// The cmd package CANNOT see it. S4 ADDS `func GlobalConfigPath() string { return globalConfigPath() }`
// to file.go — the ONE additive edit. Do NOT duplicate the XDG/home logic in cmd (two sources of truth
// would let `config path` and `config.Load` disagree on the global location). The unexported func stays.

// CRITICAL (skip config load, design D2/F2): shouldSkipConfigLoad returns true for cmd.Name()=="init"
// and =="path" (S1 wired this for S4). So PersistentPreRunE returns nil — NO config.Load, NO Layer-4
// `git config` shell-out. CONSEQUENCE: config init/path run ANYWHERE (outside a repo, fresh machine).
// Do NOT edit root.go, do NOT remove init/path from the skip-list, do NOT add a PersistentPreRunE to
// the new commands (it would shadow root's).

// CRITICAL (refuse overwrite, design D4): config init MUST NOT clobber an existing file. Check
// os.Stat(path) first; if no error (exists), return exitcode.New(exitcode.Error, fmt.Errorf("config
// file already exists at %s (not overwritten)", path)). os.IsNotExist(err) means "safe to write".
// Non-destructive by design; no --force in scope.

// CRITICAL (timeout is a STRING in the file, F4): the template's [defaults] must show
//   # timeout = "120s"
// (QUOTED), because fileConfig.Defaults.Timeout is a string parsed by time.ParseDuration. A bare
// `timeout = 120` in the file would fail to parse at Load time. (The resolved Config.Timeout is a
// time.Duration; the FILE shape is a string.)

// GOTCHA (path → stdout, scriptable, design D5/F6): config path prints GlobalConfigPath() to STDOUT
// via fmt.Fprintln — ONE line. Use `$(stagecoach config path)`. Errors reach the user via main's
// `stagecoach: <err>` to STDERR. The path is the DISCOVERED global path, NOT --config/STAGECOACH_CONFIG.

// GOTCHA (MkdirAll parent, design D7): the global path's parent may not exist (e.g. fresh
// ~/.config/stagecoach/). config init MUST os.MkdirAll(filepath.Dir(path), 0o755) before WriteFile.
// WriteFile perm 0o644 (matches writeConfigFile test helper + standard config perms).

// GOTCHA (cobra inherits PersistentPreRunE, but it's skipped): root has PersistentPreRunE; config/
// init/path do NOT define their own → root's runs (and returns nil immediately via shouldSkipConfigLoad).
// Do NOT add a PersistentPreRunE to any new command.

// GOTCHA (cobra arg validation runs before RunE): init/path use cobra.NoArgs. With any args, cobra
// returns an arg error BEFORE RunE. exitcode.For on a plain cobra error → default 1. SilenceErrors+
// SilenceUsage → cobra prints nothing; main prints `stagecoach: <msg>`.

// GOTCHA (rootCmd singleton state): config_test.go drives rootCmd directly via SetArgs/SetOut/SetErr.
// RESTORE state in t.Cleanup via restoreRootState (the existing helper) — SetArgs(nil), Out/Err,
// loadedCfg=nil, resetFlags — or tests poison each other (and trip -race). Mirror root_test.go hygiene.

// GOTCHA (template is INERT): EVERY option line in exampleConfigTemplate must begin with `#`. A user
// who runs config init and never edits the file must have ZERO behavior change (config.Load reads an
// all-commented file as "no overrides" — loadTOML returns a zero fileConfig). Verify in a test by
// asserting the written file has NO un-commented `[` section header.

// GOTCHA (provider field names must match manifest.go, F3): the template's [provider.X] example uses
// the §16.2 subset — command, prompt_delivery, print_flag, model_flag, default_model,
// system_prompt_flag, default_provider, bare_flags, output — ALL of which are valid manifest.go toml
// tags. Do NOT invent fields (no `args`, `timeout`, etc. in a provider manifest).

// GOTCHA (stat-then-write is not atomic): there's a TOCTOU window between os.Stat and WriteFile, but
// it's acceptable here (config init is an interactive bootstrap, not a concurrent service). Do NOT add
// O_EXCL locking — out of scope.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/file.go  (EDIT — add this ONE exported wrapper next to the existing globalConfigPath)
//
// GlobalConfigPath returns the resolved GLOBAL Stagecoach config path (PRD §16.1 layer 2): the file
// `config init` writes and `config path` prints. It delegates to the existing unexported
// globalConfigPath() so there is a SINGLE source of truth for the global config location (what
// config.Load reads and what the CLI reports are always the same).
func GlobalConfigPath() string { return globalConfigPath() }
```

```go
// internal/cmd/config.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
)

// configCmd is the PRD §15.3 "config" command group. It has NO RunE → bare `stagecoach config` prints
// help (cobra default). init/path are its leaves (registered in init()). Both leaves are in
// shouldSkipConfigLoad (cmd.Name()=="init"/"path") so root's PersistentPreRunE returns nil immediately
// — they work OUTSIDE a git repo and never need config.Load.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the Stagecoach config file",
	Long: `Inspect or bootstrap the Stagecoach global config file.

Subcommands:
  init   Write a commented example config to the global config path.
  path   Print the resolved global config path.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a commented example config to the global path",
	Long: `Write a fully-commented example config to Stagecoach's global config path.

The written file documents every available option (defaults, generation tuning, provider overrides)
with all lines commented out, so it changes no behavior until you uncomment the lines you want. Parent
directories are created as needed.

If a config file already exists at the global path, it is NOT overwritten (exit code 1) to protect
your edits. Delete the file first to regenerate it.

See ` + "`stagecoach config path`" + ` for the target location.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved global config path",
	Long: `Print the resolved global config path (the file ` + "`config init`" + ` writes and Stagecoach
reads as its global config layer).

This is the DISCOVERED global location ($XDG_CONFIG_HOME/stagecoach/config.toml, or
~/.config/stagecoach/config.toml by default) — not a --config/STAGECOACH_CONFIG override, which selects
a separate read path.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigPath,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd) // register on S1's root — NO edit to root.go (design D2)
}

// runConfigPath implements `stagecoach config path` (FR38). Prints the resolved global config path to
// stdout (one line). Returns nil. Never calls os.Exit. Works outside a git repo (config load skipped).
func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stdout, config.GlobalConfigPath())
	return nil
}

// runConfigInit implements `stagecoach config init` (FR38). Writes the commented exampleConfigTemplate
// to the global config path (creating parent dirs). REFUSES to overwrite an existing file (exit 1,
// non-destructive). Prints a confirmation to stdout on success. Never calls os.Exit.
func runConfigInit(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()
	if _, err := os.Stat(path); err == nil {
		// File exists — do NOT clobber the user's config.
		return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path))
	} else if !os.IsNotExist(err) {
		// Unable to stat (permissions, etc.) — surface it rather than guessing.
		return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
	}
	if err := os.WriteFile(path, []byte(exampleConfigTemplate), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}
	fmt.Fprintf(os.Stdout, "Wrote example config to %s\n", path)
	return nil
}

// exampleConfigTemplate is the commented example config written by `config init` (PRD §16.2 / FR38).
// EVERY option line is commented out (#), so the file is INERT until the user uncomments it. This
// template IS the Mode-A user-facing config documentation: the header explains the §9.8 precedence
// order, STAGECOACH_* env vars, and `stagecoach.*` git-config keys; the [defaults]/[generation]/
// [provider.X] sections mirror §16.2 with documented default values and (for providers) field names
// that match internal/provider/manifest.go toml tags.
const exampleConfigTemplate = `# Stagecoach configuration file (PRD §16.2).
#
# Generated by ` + "`stagecoach config init`" + `. Every option below is COMMENTED OUT (#), so this file
# is inert — it documents the available options without changing any defaults. To use an option,
# copy its line to a new (uncommented) line and adjust the value.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGECOACH_* env vars  >  repo git config (stagecoach.*)  >
#   repo-local .stagecoach.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagecoach.toml) and repo git config (stagecoach.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGECOACH_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGECOACH_MODEL      model override ("" -> provider manifest default_model)
#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGECOACH_CONFIG     path to a config file, overrides discovery
#   STAGECOACH_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGECOACH_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#
# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagecoach.provider pi
#   git config stagecoach.model ""
#   git config stagecoach.timeout 120s
#   git config stagecoach.auto_stage_all true
#   (read via ` + "`git config --get stagecoach.<key>`" + `)

# ---------------------------------------------------------------------------
# [defaults] — top-level Stagecoach behavior (PRD §16.2)
# ---------------------------------------------------------------------------
# [defaults]
# provider       = "pi"     # default agent; "" -> auto-detect (first installed built-in)
# model          = ""       # "" -> use the provider manifest's default_model
# timeout        = "120s"   # generation timeout (Go duration string, e.g. "2m", or bare seconds)
# auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged
# verbose        = false    # print the resolved command, raw agent output, and retries

# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json"
# strip_code_fence      = true    # remove ` + "`" + ` fences from agent output

# ---------------------------------------------------------------------------
# [provider.<name>] — override a built-in or define a new provider (PRD §16.2, §12.8)
# ---------------------------------------------------------------------------
# A [provider.<name>] section FIELD-MERGES onto a built-in of the same name. A brand-new <name>
# adds a new provider. Use ` + "`stagecoach providers show <name>`" + ` to inspect the merged result.
#
# Override a built-in (e.g. pin pi to a different model/provider):
# [provider.pi]
# default_model    = "glm-5.2"
# default_provider = "zai"
#
# Define a brand-new provider (PRD §12.8):
# [provider.myagent]
# command            = "/opt/myagent/bin/agent"
# prompt_delivery    = "stdin"          # stdin | positional | flag
# print_flag         = "--once"
# model_flag         = "--model"
# default_model      = "my-model-7b"
# system_prompt_flag = "--system"
# default_provider   = "zai"
# bare_flags         = ["--no-mcp", "--ephemeral"]
# output             = "raw"            # raw | json
`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/file.go (add the GlobalConfigPath export wrapper)
  - FILE: internal/config/file.go. PACKAGE: config. Add `func GlobalConfigPath() string { return
      globalConfigPath() }` with a doc comment, adjacent to the existing unexported globalConfigPath
      (~line 60-70). ADDITIVE ONLY — do NOT move/rename/rewrite globalConfigPath.
  - WHY: the cmd package cannot see the unexported func (F1). This is the ONE edit to an existing file.
  - GOTCHA: the wrapper MUST delegate (return globalConfigPath()), never re-implement — or config path
      and config.Load will disagree. file_test.go + load.go still call the unexported one (unchanged).

Task 2: CREATE internal/cmd/config.go (the command group + RunE + template)
  - FILE: NEW internal/cmd/config.go. PACKAGE: `package cmd`. Follow "Data models" skeleton.
  - DEFINE: configCmd, configInitCmd, configPathCmd (cobra.Command); func init() (registers them on
      rootCmd — NO edit to root.go); runConfigInit(cmd, args) error; runConfigPath(cmd, args) error;
      exampleConfigTemplate const string.
  - IMPORTS: fmt, os, path/filepath, github.com/spf13/cobra,
      github.com/dustin/stagecoach/internal/config, github.com/dustin/stagecoach/internal/exitcode.
  - NAMING: configCmd/configInitCmd/configPathCmd/runConfigInit/runConfigPath/exampleConfigTemplate
      (unexported, package-level). PLACEMENT: all in internal/cmd/config.go.
  - GOTCHA: init() calls rootCmd.AddCommand(configCmd) — rootCmd is S1's package-level var (same
      package, directly visible). Use `cobra.NoArgs` for both leaves. The template const's EVERY option
      line must start with `#` (INERT). Provider field names must match manifest.go toml tags (F3).
      `timeout = "120s"` is QUOTED (file shape is a string, F4).
  - GOTCHA: do NOT add PersistentPreRunE to any of the three commands (root's is inherited + skipped
      for init/path via shouldSkipConfigLoad). Do NOT read Config() (init/path don't load config).

Task 3: CREATE internal/cmd/config_test.go (integration tests through the FULL CLI)
  - FILE: NEW internal/cmd/config_test.go. PACKAGE: `package cmd` (same as root_test.go).
  - REUSE root_test.go helpers (same package — do NOT re-copy): saveRootState, restoreRootState,
      loadEnvSetup, writeConfigFile, chdir. (resetFlags is called inside restoreRootState.)
  - STATE HYGIENE: each test wraps its body in `origArgs, origOut, origErr, origRunE := saveRootState(t);
      defer restoreRootState(t, origArgs, origOut, origErr, origRunE)`. Capture stdout via
      `var out bytes.Buffer; rootCmd.SetOut(&out)`. Derive exit code via `exitcode.For(err)` (import
      internal/exitcode in the test). Import internal/config too (to call config.GlobalConfigPath for
      expected-path assertions).
  - CASES (drive rootCmd via SetArgs + Execute(context.Background()); assert exitcode.For(err),
      captured stdout, and filesystem state):
      * TestConfigPath_PrintsGlobalPath: loadEnvSetup(t) for HOME/XDG isolation; chdir(t, t.TempDir())
        (plain dir — NO repo, to prove outside-a-repo works); SetArgs(["config","path"]); Execute.
        Assert exit 0; stdout (trimmed) == config.GlobalConfigPath(); and it ends with
        filepath.Join("stagecoach","config.toml").
      * TestConfigInit_WritesTemplate: loadEnvSetup(t); chdir(t, t.TempDir()) (NO repo);
        SetArgs(["config","init"]); Execute. Assert exit 0; stdout contains "Wrote example config";
        the file at config.GlobalConfigPath() EXISTS; its content == exampleConfigTemplate (read it
        back with os.ReadFile + string compare); the parent dir (filepath.Dir) was created.
      * TestConfigInit_TemplateIsInert: read the file written by TestConfigInit_WritesTemplate (or write
        it inline via config.GlobalConfigPath); assert NO line is an un-commented TOML table header —
        i.e. no line matches `^\[[a-z]` (every `[defaults]`/`[generation]`/`[provider.X]` header is
        `#`-commented). Also assert it contains "[defaults]", "[generation]", "[provider.pi]",
        "[provider.myagent]" (as commented guidance) and "STAGECOACH_PROVIDER" + "stagecoach.provider"
        (env + git-key docs present).
      * TestConfigInit_RefusesOverwrite: loadEnvSetup(t); chdir(t, t.TempDir()); pre-create the config
        file with writeConfigFile(t, globalDir, "config.toml", "provider = \"mine\"\n") (globalDir from
        loadEnvSetup = <home>/stagecoach); SetArgs(["config","init"]); Execute. Assert
        exitcode.For(err)==1 (Error); the returned err's message contains "already exists"; the file
        content is UNCHANGED (still "provider = \"mine\""). (Non-destructive — D4.)
      * TestConfigInit_MkdirAllParent: loadEnvSetup(t); chdir(t, t.TempDir()); assert the parent dir
        (<home>/stagecoach) does NOT exist yet (os.Stat fails); SetArgs(["config","init"]); Execute.
        Assert exit 0 AND the parent dir now exists (os.Stat succeeds) AND the file exists. (D7.)
      * TestConfigInit_PathWorksOutsideGitRepo: loadEnvSetup(t); chdir(t, t.TempDir()); do NOT call
        initRepo; SetArgs(["config","path"]); Execute → exit 0; SetArgs(["config","init"]); Execute →
        exit 0. (Proves shouldSkipConfigLoad works — no git needed. This is the distinguishing S4
        property vs S3's list/show, which REQUIRE a repo.)
      * TestConfigInit_ExtraArgsExits1: loadEnvSetup(t); chdir(t, t.TempDir()); SetArgs(["config",
        "init","x"]); Execute; assert exitcode.For(err)==1 (cobra NoArgs rejects 1 arg).
      * TestConfigPath_ExtraArgsExits1: as above with ["config","path","x"] → exit 1.
      * TestConfigGroup_NoSubcommandPrintsHelp: loadEnvSetup(t); chdir(t, t.TempDir()); var buf
        bytes.Buffer; rootCmd.SetOut(&buf); SetArgs(["config"]); Execute; assert exit 0; buf contains
        "init" and "path" (help lists the subcommands). (configCmd has no RunE → cobra prints help.)
  - COVERAGE: path (prints path, extra-args); init (writes, inert, refuse-overwrite, mkdirall,
      outside-repo, extra-args); group (help). Use bytes.Contains / strings for substring checks. No
      stubtest/agent/git dependency (pure path + file I/O).

Task 4: VALIDATE (run all gates; fix before declaring done)
  - `make build` → ./bin/stagecoach exists; `./bin/stagecoach config path` prints the global path;
      `./bin/stagecoach config init` writes it; `./bin/stagecoach config init` again exits 1 (refuse);
      `./bin/stagecoach config` prints help. Run all from a NON-repo dir to prove outside-a-repo.
  - `go test -race ./internal/config/ -v` → green (the GlobalConfigPath edit is additive; file_test.go
      still passes; OPTIONALLY add a tiny `TestGlobalConfigPath_Exported` asserting
      GlobalConfigPath()==globalConfigPath() — optional, in file_test.go or skip).
  - `go test -race ./internal/cmd/ -v` → green (root_test.go + default_action_test.go + providers_test.go
      + config_test.go).
  - `go test -race ./...` → green (NO regression).
  - `go vet ./...` clean; `gofmt -l internal/cmd/ internal/config/` empty.
  - `git status` shows: modified internal/config/file.go, new internal/cmd/config.go, new
      internal/cmd/config_test.go. (root.go UNCHANGED — verify with `git diff internal/cmd/root.go` =
      empty; providers.go UNCHANGED — `git diff internal/cmd/providers.go` = empty.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: register via init() in the NEW file — do NOT edit root.go (design D2; mirrors S3).
//   func init() { configCmd.AddCommand(configInitCmd, configPathCmd); rootCmd.AddCommand(configCmd) }
// rootCmd is S1's package-level var, visible in config.go (same package). Parallel-safe with all siblings.

// PATTERN: exit codes via returned errors; never os.Exit (design F6).
func runConfigInit(cmd *cobra.Command, args []string) error {
    path := config.GlobalConfigPath()
    if _, err := os.Stat(path); err == nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path)) // exit 1; main prints
    } else if !os.IsNotExist(err) {
        return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err)) // exit 1; main prints
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
    }
    if err := os.WriteFile(path, []byte(exampleConfigTemplate), 0o644); err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
    }
    fmt.Fprintf(os.Stdout, "Wrote example config to %s\n", path)
    return nil // exit 0
}

// PATTERN: path → stdout (one line, scriptable — design D5).
func runConfigPath(cmd *cobra.Command, args []string) error {
    fmt.Fprintln(os.Stdout, config.GlobalConfigPath()) // `$(stagecoach config path)` works
    return nil
}

// GOTCHA: config loads is SKIPPED for init/path (shouldSkipConfigLoad returns true for cmd.Name()
// "init"/"path"). So they work outside a git repo and never read Config(). Do NOT read Config().

// GOTCHA: the export wrapper delegates — never re-implement globalConfigPath (design D1/F1).
//   func GlobalConfigPath() string { return globalConfigPath() }

// GOTCHA: refuse-overwrite via os.Stat (design D4). os.IsNotExist => safe to write; any other stat
// error => surface it. Non-destructive; no --force.

// GOTCHA: timeout in the FILE is a STRING — template shows `# timeout = "120s"` (quoted). Provider
// field names in the template MUST match manifest.go toml tags (F3). Every option line is `#`-commented.
```

### Integration Points

```yaml
CONFIG.PATH (P1.M1.T4.S2 → S4 exports + consumes):
  - globalConfigPath: "UNEXPORTED in internal/config/file.go. S4 ADDS `func GlobalConfigPath() string
    { return globalConfigPath() }` (the ONE additive edit). cmd/config.go calls config.GlobalConfigPath().
    Verified by `git diff internal/config/file.go` showing ONLY the new wrapper (no rewrite)."
  - gotcha: "delegate, never re-implement. file_test.go + load.go still call the unexported one."

ROOT.COMMAND (S1 → S4 registers via init(); NO edit):
  - rootCmd: "S1's package-level singleton. config.go's init() calls rootCmd.AddCommand(configCmd).
    root.go is UNCHANGED. Verified by `git diff internal/cmd/root.go` == empty."
  - shouldSkipConfigLoad: "ALREADY returns true for cmd.Name()=='init'/'path' (S1 wired it for S4).
    So PersistentPreRunE returns nil — no config.Load, no git shell-out. config init/path run ANYWHERE."

EXIT.CODE (S1 → S4 returns errors it maps):
  - exitcode.New/Error/For: "S4 returns exitcode.New(exitcode.Error, err) on failure; main calls
    os.Exit(exitcode.For(err)) and prints `stagecoach: <err>`. S4 never calls os.Exit. Only 0/1 occur."

CLI.HELP (Mode-A docs, S4 owns):
  - Short/Long: "config/config init/config path each have Short (shown in parent --help) + Long (shown
    on own --help). The `config init` Long states the refuse-overwrite behavior + that dirs are created.
    The `config path` Long states the path is the discovered global location (not an override)."
  - exampleConfigTemplate: "the const IS the user-facing config documentation deliverable (Mode-A).
    It documents precedence (FR34), env vars (FR35), git keys (FR36), and every §16.2 section."

UI (forward — P1.M4.T3):
  - runConfigPath/runConfigInit: "S4 writes plain text to os.Stdout. P1.M4.T3 may colorize the
    'Wrote example config to …' line when TTY + !NoColor. S4 = the DATA/logic."
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating config.go + editing file.go - fix before proceeding
go build ./internal/cmd/ ./internal/config/ ./cmd/stagecoach/
gofmt -w internal/cmd/config.go internal/config/file.go
go vet ./internal/cmd/ ./internal/config/

# Expected: zero errors. If `go build` reports config.GlobalConfigPath undefined, you didn't add the
# wrapper to file.go (Task 1). If it reports rootCmd undefined, you're not in package cmd / wrong import.
# CRITICAL self-checks:
#   git diff --stat internal/cmd/root.go          # MUST be empty (S4 does not edit root.go)
#   git diff --stat internal/cmd/providers.go     # MUST be empty (S4 does not touch S3's file)
#   git diff internal/config/file.go | head -40   # ONLY the new GlobalConfigPath wrapper (additive)
```

### Level 2: Unit/Integration Tests (the FULL CLI)

```bash
# config path/init tests drive the real cobra rootCmd (no agent/stub/git needed).
go test -race ./internal/cmd/ -run TestConfig -v

# Full cmd package (S1 root_test.go + S2 default_action_test.go + S3 providers_test.go + S4 config_test.go)
go test -race ./internal/cmd/ -v

# The config edit is additive — config tests still pass (OPTIONALLY add a 1-line export-parity test).
go test -race ./internal/config/ -v

# Expected: all green. If TestConfigInit_RefusesOverwrite fails, check that the pre-created file is at
# config.GlobalConfigPath() (loadEnvSetup sets XDG=<home> => path = <home>/stagecoach/config.toml; the
# pre-create must write THERE, via writeConfigFile(t, globalDir, "config.toml", …)). If a test poisons
# another, check restoreRootState is deferred in EVERY test (singleton hygiene).
```

### Level 3: Integration Testing (Binary Validation)

```bash
# Build the real binary
make build

# Run from a NON-git directory to prove config init/path work anywhere:
cd "$(mktemp -d)"
export XDG_CONFIG_HOME="$(pwd)/.config"   # isolate so you don't clobber your real config
BIN=/home/dustin/projects/stagecoach/bin/stagecoach

# config path prints the resolved global path
$BIN config path
# Expected: <tmp>/.config/stagecoach/config.toml (one line). Exit 0. Works WITHOUT a git repo.

# config init writes the commented template + creates parent dir
$BIN config init
# Expected: "Wrote example config to <tmp>/.config/stagecoach/config.toml". Exit 0.

# the written file is INERT (all option lines commented) and documents everything
cat "$($BIN config path)"
# Expected: header (precedence, STAGECOACH_*, stagecoach.*) + commented [defaults]/[generation]/
# [provider.pi]/[provider.myagent]. NO un-commented [ section header.

# a second config init REFUSES to overwrite (non-destructive)
$BIN config init; echo "exit=$?"
# Expected: stderr `stagecoach: config file already exists at … (not overwritten)`, exit=1. File unchanged.

# round-trip: uncomment a section and confirm config.Load parses it (proves the template is valid TOML)
sed 's/^# \[defaults\]/[defaults]/; s/^# provider       = "pi"/provider = "pi"/' "$($BIN config path)" \
  > /tmp/uncommented.toml && go run -mod=mod github.com/pelletier/go-toml/v2/cmd/tomljson@latest \
  /tmp/uncommented.toml >/dev/null && echo "valid TOML"
# Expected: "valid TOML" (the field names + types in the template parse).

# bare `config` → help (lists init/path)
$BIN config
# Expected: help text naming the init and path subcommands. Exit 0.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scriptability: config path is usable in $(...) and | xargs
cp /dev/null "$($BIN config path).bak" && rm "$($BIN config path).bak" && echo "path is scriptable"

# The template's documented defaults match config.Defaults() (grep a few):
grep -q 'max_diff_bytes        = 300000' "$($BIN config path)" && echo "defaults documented correctly"
grep -q 'timeout        = "120s"' "$($BIN config path)" && echo "timeout is a quoted string"

# Determinism: two fresh inits in two different XDG dirs produce byte-identical files
A=$(mktemp -d); B=$(mktemp -d)
XDG_CONFIG_HOME=$A $BIN config init >/dev/null
XDG_CONFIG_HOME=$B $BIN config init >/dev/null
diff "$A/stagecoach/config.toml" "$B/stagecoach/config.toml" && echo "deterministic template"

# Provider field names are valid: uncomment [provider.myagent] fully and round-trip
sed 's/^# //; /^$/d; /^Stagecoach/d' "$($BIN config path)" 2>/dev/null | head >/dev/null || true
# (Human sanity check that the provider example uses real manifest.go field names.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./internal/cmd/ -v` AND `go test -race ./internal/config/ -v`
      (and `go test -race ./...` shows no regression)
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/cmd/ internal/config/` empty
- [ ] **root.go UNCHANGED**: `git diff --stat internal/cmd/root.go` is empty (registration via init())
- [ ] **providers.go UNCHANGED**: `git diff --stat internal/cmd/providers.go` is empty
- [ ] **file.go edit is additive**: `git diff internal/config/file.go` shows ONLY the new
      `GlobalConfigPath` wrapper (no rewrite of globalConfigPath)

### Feature Validation

- [ ] All success criteria from "What" section met
- [ ] `stagecoach config path` prints the resolved global path (one line, ends in /config.toml)
- [ ] `stagecoach config init` writes the commented template to that path + creates parent dirs
- [ ] The written template is INERT (every option line `#`-commented; no un-commented `[` header)
- [ ] The template documents precedence (FR34), STAGECOACH_* env vars (FR35), stagecoach.* git keys (FR36)
- [ ] The template's [defaults]/[generation]/[provider.X] mirror §16.2 with valid field names (manifest.go)
- [ ] `config init` REFUSES to overwrite an existing file (exit 1, message, file unchanged)
- [ ] Both `config path` and `config init` run OUTSIDE a git repo (exit 0)
- [ ] `config path`/`config init` with extra args exit 1 (cobra NoArgs); bare `config` prints help
- [ ] Mode-A help text documents behavior on each command's --help

### Code Quality Validation

- [ ] Follows existing codebase patterns (mirrors S3's providers.go shape; exitcode centralization;
      rootCmd singleton hygiene in tests; init() registration in a new file)
- [ ] File placement matches desired codebase tree (1 edited + 2 NEW files)
- [ ] Anti-patterns avoided (no os.Exit in RunE; no shadowing PersistentPreRunE; no root.go edit; no
      duplicated path logic; no destructive overwrite)
- [ ] Dependencies properly managed (only stdlib + cobra + internal/{config,exitcode})
- [ ] No new external dependencies

### Documentation & Deployment

- [ ] exampleConfigTemplate is self-documenting (Mode-A deliverable) — the user-facing config reference
- [ ] Short/Long help text on config/config init/config path
- [ ] No new environment variables or config keys introduced (only documents existing ones)

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate the XDG/home path logic in cmd — export `GlobalConfigPath()` and delegate (design
  D1/F1). Two sources of truth would let `config path` and `config.Load` disagree.
- ❌ Don't edit root.go to register the command or to add init/path to shouldSkipConfigLoad — use
  `init()` in the new file (D2); shouldSkipConfigLoad already handles init/path (S1 wired it).
- ❌ Don't add a `PersistentPreRunE` to config/init/path — it would shadow root's (irrelevant here since
  the skip-list short-circuits, but don't add one regardless).
- ❌ Don't overwrite an existing config file — refuse with exit 1 (D4). Non-destructive by design.
- ❌ Don't call `os.Exit` in a RunE — return an error; main maps it via `exitcode.For`.
- ❌ Don't read `Config()` in init/path — config load is skipped for them; they don't need it.
- ❌ Don't write `timeout = 120` (bare) in the template — the file shape is a STRING; use `timeout =
  "120s"` (quoted), or config.Load will fail to parse an uncommented timeout.
- ❌ Don't invent provider field names not in `internal/provider/manifest.go` (no `args`, `timeout`,
  etc. in a provider manifest) — the template is user-facing docs; wrong names mislead.
- ❌ Don't leave any option line UN-commented in the template — the written file must be INERT (zero
  behavior change) until the user edits it.
- ❌ Don't assert on the exact resolved path bytes in tests (host-dependent) — assert via
  `config.GlobalConfigPath()` under an isolated temp HOME/XDG (loadEnvSetup / t.Setenv).
