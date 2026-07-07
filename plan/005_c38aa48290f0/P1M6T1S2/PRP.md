name: "P1.M6.T1.S2 — `stagecoach models [<provider>]` command with curated-table fallback"
description: |

---

## Goal

**Feature Goal**: Implement the `stagecoach models [<provider>]` cobra command (PRD §9.23 FR-L1, §15.3)
that prints the models reachable by a provider. Source-of-truth order per provider (FR-L1): **(a)** if
the manifest's `list_models_command` (delivered by S1 as `Manifest.ListModelsCommand []string`) is
non-empty, run it as a subprocess (inherited env, bounded timeout) and print its stdout under a provider
heading; **(b)** if the field is absent OR the command fails (non-zero exit / timeout / not found), print
the curated per-role tier table from the EXISTING `config.DefaultModelsForProvider(name)` (FR-D4),
annotated with its verification date + "consult `<command> --help` for the live list". **Never an HTTP
call** (§6.2 N2 / FR-L1) — the agent CLI is the only model authority. Default target = the resolved
default provider; `--all` = every DETECTED provider; an unknown or undetected named provider is a clear
error with a nonzero exit.

**Deliverable**:
1. `internal/cmd/models.go` (NEW) — `modelsCmd` cobra leaf on `rootCmd` (`Use: "models [<provider>]"`,
   `Args: cobra.MaximumNArgs(1)`, `RunE: runModels`), a LOCAL `--all`/`-a` flag (`flagModelsAll`), the
   `runModels` resolver, the `runListModels` executor call, and the two pure renderers
   (`printLiveList`, `printCuratedTable`). Registered on root in `init()` — **NO edit to root.go** (the
   `providers.go` precedent: "register on S1's root — NO edit to root.go").
2. `internal/config/role_defaults.go` (EDIT — one exported constant) — add
   `DefaultModelsVerificationDate = "2026-07-02"` so the curated-table annotation can surface the date as
   DATA (today it lives only in a comment). Keeps the date + the table refreshable together (FR-D5).
3. `docs/cli.md` (EDIT) — add the `### \`models [<provider>]\`` subcommand section (source order,
   `--all`, default = resolved default provider, never-HTTP note), placed before `## Exit codes`.
4. `internal/cmd/models_test.go` (NEW) — stub-binary live-list test, deterministic golden renderer test,
   command-FAILURE fallback test, and the error-case matrix (unknown / undetected / no-default /
   `--all`+arg / `--all`-empty), following `providers_test.go`'s harness.

**Success Definition**:
- `go build ./...`, `go test ./internal/cmd/... ./internal/config/... -v`, `go vet ./...`,
  `golangci-lint run`, `gofmt -l` all green.
- `stagecoach models claude` (claude detected, no `list_models_command`) prints the curated per-role table
  with the verification-date footer + `consult \`claude --help\`` hint (FR-L1 (b)).
- A detected provider whose `list_models_command` succeeds (e.g. opencode on PATH) prints the CLI's live
  stdout under a heading (FR-L1 (a)).
- A detected provider whose `list_models_command` FAILS (non-zero / timeout / not found) prints the
  curated table on stdout AND a one-line "list command failed; using curated table" notice on stderr.
- `stagecoach models --all` prints one block per detected provider; `stagecoach models` (no arg) prints
  the resolved default provider's block.
- `stagecoach models ghost` (unknown) and `stagecoach models <known-but-undetected>` exit 1 with a clear
  message; `stagecoach models` with nothing detected exits 1; `stagecoach models --all <p>` is a usage
  error (exit 1).
- `stagecoach models --help` shows the `--all` help text scoped to models (NOT the root's "git add -A"
  text) — proving the local flag overrides the inherited persistent one cleanly.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) and "multi-agent tinkerer" (§7.3) deciding which model to
pin per role before running `config init` or editing `[role.*]`. Also the "API-key refusenik" (§7.2) who
wants to see reachable models WITHOUT handing stagecoach an API key.

**Use Case**: `stagecoach models` (or `stagecoach models opencode`) shows what the installed agent CLI can
reach — read straight from the agent CLI itself (never stagecoach hitting a provider HTTP API with a key,
which is exactly what the incumbents do and stagecoach refuses to require, §6.2 N2).

**Pain Points Addressed**: incumbents (aicommits/opencommit) list models via provider HTTP APIs with the
user's key. Stagecoach has no key and asks the agent CLI instead — same reason the product exists. When a
CLI exposes no listing surface, stagecoach falls back to its own curated FR-D4 tier table rather than
leaving the user blind.

## Why

- **FR-L1 (PRD §9.23 / §15.3)**: the feature contract — source-of-truth order (a) `list_models_command`
  stdout, then (b) curated FR-D4 table on absent/failure; default = resolved default provider; `--all` =
  every detected provider; never an HTTP call.
- **FR-L2 (PRD §9.23 / §12.1)**: `list_models_command` is the optional argv this command runs —
  DELIVERED by P1.M6.T1.S1 (the parallel task; treat as the input contract). S2 is its only consumer.
- **§6.2 N2**: never an HTTP call — stagecoach has no key; the agent CLI is the only model authority.
  `models` routes discovery through the installed CLI the user already has.
- **architecture/system_context.md §3 (internal/provider seam)**: "`list_models_command` … the one new
  §12.1 field"; and (internal/config seam) "**FR-D4 curated tier table already exists as
  `DefaultModelsForProvider(name)`** (role_defaults.go) — reuse it as the `models` fallback (FR-L1b)."
  This PRP reuses BOTH — no new data table.
- **Scope fences**: S2 CONSUMES S1's `Manifest.ListModelsCommand`, the existing `provider.Execute`
  executor seam, the existing `Registry` (Get/IsInstalled/DefaultProvider), the existing
  `DefaultModelsForProvider`, and the existing `internal/cmd/providers.go` helpers
  (`newRegistry`/`installedNames`/`resolvedDefault`). S2 PROVIDES the `models` command surface for
  P1.M6.T2.S1 (`config init --interactive` may surface the listing). S2 does NOT add a config key or
  CLI flag outside the command's own local `--all`; it does NOT touch the manifest schema (S1 owns that);
  it does NOT implement the interactive wizard (P1.M6.T2.S1).

## What

A read-only `models [<provider>]` cobra leaf on root, with a local `--all`/`-a` flag, that resolves a
target set (one provider, or every detected provider), and for each target prints either the CLI's live
model list (when `list_models_command` is set and succeeds) or stagecoach's curated per-role tier table
(when the field is absent or the command fails). Output goes to stdout (pipeable); the "falling back to
curated table" notice goes to stderr. Errors are nonzero exits via `exitcode.New(exitcode.Error, …)`.

### Success Criteria

- [ ] `internal/cmd/models.go`: `modelsCmd` registered on `rootCmd` in `init()`; `Use: "models
      [<provider>]"`; `Args: cobra.MaximumNArgs(1)`; local `--all`/`-a` flag (`flagModelsAll`) overriding
      the inherited persistent `--all`; `RunE: runModels`.
- [ ] `runModels` resolves the target set: `--all` → `installedNames(reg)`; explicit arg → that provider
      (must be known AND detected, else error); no arg → `resolvedDefault(Config(), reg, installed)` (if
      "" → error). `--all` + an arg is a usage error.
- [ ] Per provider: if `len(m.ListModelsCommand) > 0` → `runListModels` runs it (inherited env, bounded
      timeout = `cfg.Timeout`); on success `printLiveList`; on ANY failure (non-zero / timeout / not
      found) → stderr notice + `printCuratedTable`. Else (`ListModelsCommand` absent) → `printCuratedTable`.
- [ ] `printCuratedTable` renders `config.DefaultModelsForProvider(name)` in FIXED role order
      `[planner, stager, message, arbiter]`, empty stager → `—`, with the
      `DefaultModelsVerificationDate` footer + `consult \`<command> --help\`` hint (`<command>` =
      `m.DetectCommand()`).
- [ ] `internal/config/role_defaults.go`: add exported `DefaultModelsVerificationDate = "2026-07-02"`.
- [ ] `docs/cli.md`: `### \`models [<provider>]\`` section before `## Exit codes` (source order, `--all`,
      default, never-HTTP).
- [ ] `internal/cmd/models_test.go`: stub-binary live-list test, golden renderer test, command-FAILURE
      fallback test, and the error matrix — all pass.
- [ ] All build/test/vet/lint/fmt green; `stagecoach models --help` shows the models-scoped `--all` text.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT cobra leaf to add (mirroring `providers.go`), the EXACT reused helpers
(`newRegistry`/`installedNames`/`resolvedDefault` in the SAME package), the EXACT executor seam
(`provider.Execute` + `provider.CmdSpec`), the EXACT fallback function (`config.DefaultModelsForProvider`),
the EXACT one-constant addition (`DefaultModelsVerificationDate`) and its value, the proven mechanism for
the `--all` collision (the `config init --provider` precedent + root.go's own comment), the EXACT
error/exit matrix, the EXACT output format for both render paths, and the EXACT docs insertion point. An
implementer with no prior codebase knowledge can build it from this document + codebase access._

### Documentation & References

```yaml
- file: internal/cmd/providers.go
  why: THE template. A cobra command leaf on rootCmd registered in init(); RunE builds the registry via
       newRegistry(), computes installedNames(reg) + resolvedDefault(Config(), reg, installed), prints to
       cmd.OutOrStdout(), and returns exitcode.New(exitcode.Error, err) on failure. Mirror this exactly.
  pattern: |
       var providersListCmd = &cobra.Command{ Use, Short, Long, Args: cobra.NoArgs, RunE: runProvidersList }
       func init() { rootCmd.AddCommand(providersCmd) }
       func runProvidersList(cmd *cobra.Command, args []string) error { reg, err := newRegistry(); ... }
  gotcha: |
    REUSE newRegistry()/installedNames()/resolvedDefault() — they are unexported helpers in package cmd
    (providers.go). Do NOT re-implement them. models.go is in the SAME package, so call them directly.

- file: internal/cmd/providers_test.go
  why: THE test harness to copy. setupRepo(t), saveRootState/restoreRootState, writeConfigFile,
       rootCmd.SetArgs + Execute(context.Background()), exitcode.For(err) assertions. models_test.go is
       its sibling; reuse ALL these helpers verbatim (they live in root_test.go / providers_test.go,
       same package — no import).
  pattern: TestProvidersShow_UnknownExits1 (assert exitcode.Error + err string contains the name).
  gotcha: Tests run in a git repo (setupRepo) because PersistentPreRunE → config.Load reads Layer 4. A
          test that needs a provider DETECTED must put a fake binary on a temp PATH (see stub pattern).

- file: internal/provider/executor.go
  why: THE executor seam to reuse. Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose)
       (stdout, stderr string, err error). Sets up the process group, context timeout, stdout/stderr
       capture. timeout>0 ⇒ internal context.WithTimeout; on expiry err IS context.DeadlineExceeded.
  pattern: |
       spec := provider.CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}   // Env nil ⇒ inherit parent env
       out, errb, err := provider.Execute(ctx, spec, timeout, vb)
  gotcha: |
    (1) Env nil ⇒ child inherits the parent env (executor.go: "nil Env ⇒ the child inherits the parent
    env") — exactly "inherited env"; do NOT pass os.Environ() (works but redundant). (2) Execute calls
    signal.RegisterChild/ClearChild — harmless for a non-generate-flow subcommand (the rescue signal
    handler is armed only in the generate pipeline). (3) Stdin MUST be "" (not nil-missing) so the child
    gets /dev/null and does not hang waiting on stdin.

- file: internal/provider/render.go
  why: CmdSpec definition (the struct Execute consumes). Command=executable, Args=flag portion AFTER
       command, Stdin="" ⇒ /dev/null, Env nil ⇒ inherit. Build it directly from ListModelsCommand — do
       NOT call Manifest.Render (Render is for the commit-generation argv; ListModelsCommand is already
       the full argv: binary + args).
  pattern: spec := provider.CmdSpec{Command: argv[0], Args: argv[1:]}
  gotcha: ListModelsCommand is the FULL argv (e.g. ["pi","--list-models"]). argv[0] is the binary; the
          rest are args. A single-element argv (["mybin"]) is legal → Args is empty.

- file: internal/provider/registry.go
  why: THE registry surface. Get(name) (Manifest, bool); IsInstalled(m) (probes DetectCommand via
       LookPath); DefaultProvider(installed) (first preferred built-in on PATH); List() (sorted). Use
       Get to read ListModelsCommand + DetectCommand(); use IsInstalled/installedNames for detection.
  pattern: m, ok := reg.Get(name); cmd := m.DetectCommand()
  gotcha: The registry stores MERGED-but-UNRESOLVED manifests. ListModelsCommand is a slice (no Resolve
          needed) and DetectCommand() works on unresolved pointers — so call reg.Get directly, no
          Validate/Resolve. (providers show does the same.)

- file: internal/config/role_defaults.go
  why: THE curated fallback. config.DefaultModelsForProvider(name) returns map[string]string
       {planner,stager,message,arbiter → model} for any built-in name, or nil for a user-defined name.
       Stager "" = not stager-capable. This is FR-D4 already in code — REUSE, do not duplicate the table.
       ALSO the file to add the verification-date constant to.
  pattern: col := config.DefaultModelsForProvider("claude")  // {"planner":"opus","stager":"sonnet",...}
  gotcha: |
    (1) Iterate a FIXED [planner,stager,message,arbiter] slice — map iteration order is random and would
    break the golden test. (2) DefaultModelsForProvider returns a COPY (safe to read). (3) For a
    user-defined provider (no table) it returns nil — render an informational "no curated defaults"
    message, exit 0. (4) The "Verification date: 2026-07-02" is a COMMENT today; S2 adds an exported
    CONSTANT DefaultModelsVerificationDate with the same value and prints THAT.

- file: internal/cmd/config.go
  why: THE precedent for the --all flag collision. config.go:142 defines a LOCAL "provider" flag that
       overrides root's persistent --provider; root.go's comment documents the mechanism: "pflag's
       AddFlagSet skips this inherited persistent flag on `config init` since a local name already
       exists." So a LOCAL --all on modelsCmd overrides the inherited persistent --all/-a (no panic,
       models-scoped help text wins). models never runs the default action, so flagAll being skipped is
       irrelevant.
  pattern: configInitCmd.Flags().String("provider", "", "...")  // overrides root persistent --provider
  gotcha: |
    Define BOTH the long name AND the -a shorthand on modelsCmd.Flags().BoolVarP(&flagModelsAll, "all",
    "a", false, ...). If you define only the long name, pflag skips the parent's whole --all flag (name
    collision) and -a becomes unbound → `stagecoach models -a` errors "unknown shorthand flag". Defining
    -a keeps it working.

- file: internal/cmd/root.go
  why: Confirms (a) PersistentPreRunE loads config for every command except init/path/upgrade + help/version
       (shouldSkipConfigLoad) — models is NOT excluded, so Config() is non-nil in runModels; (b) --all/-a
       is a PersistentFlag (flagAll, "git add -A") — the collision §3 resolves; (c) NO edit to root.go is
       needed (register modelsCmd on rootCmd inside models.go's init()).
  gotcha: Do NOT add models to shouldSkipConfigLoad — it NEEDS the resolved config (cfg.Providers overrides
          + cfg.Provider default + cfg.Timeout). It must stay in the normal load path, like providers.

- file: internal/exitcode/exitcode.go
  why: Error→exit-code mapping. exitcode.New(exitcode.Error, err) forces exit 1; main maps via
       exitcode.For. Use this for ALL models error paths (matches providers.go). Never os.Exit.
  pattern: return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name))

- url: PRD §9.23 FR-L1/L2 + §9.16 FR-D4 (in plan/005_c38aa48290f0/prd_snapshot.md)
  why: THE feature contract. FR-L1 = source-of-truth order (a list_models_command stdout, (b curated
       table on absent/failure) + default + --all + never-HTTP. FR-L2 = the field (S1). FR-D4 = the
       curated per-role tier table (DefaultModelsForProvider).
  section: "9.23 Discovery: model listing & interactive bootstrap" + "9.16 Default provider".

- docfile: plan/005_c38aa48290f0/P1M6T1S1/PRP.md
  why: THE S1 contract (the parallel task). It delivers Manifest.ListModelsCommand + the 4 populated
       built-ins + MarshalTOML surfacing. Treat it as delivered; consume m.ListModelsCommand from the
       merged registry manifest. S2 does NOT touch the manifest schema.
  section: Goal + Implementation Tasks (Task 1 field, Task 4 the 4 argv values).

- docfile: plan/005_c38aa48290f0/P1M6T1S2/research/design-decisions.md
  why: THE reasoning behind every decision above (executor choice, --all collision, error matrix, test
       strategy). Read it if a task seems under-specified — the WHY is there.
  section: all.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go               # rootCmd + PersistentPreRunE (config load) + persistent --all/-a flag (flagAll). NO EDIT.
  providers.go          # TEMPLATE: cobra leaf on root + newRegistry/installedNames/resolvedDefault helpers. REUSE.
  providers_test.go     # TEST HARNESS: setupRepo/saveRootState/writeConfigFile/Execute. REUSE in models_test.go.
  config.go             # precedent: local --provider flag overrides persistent (the --all collision mechanism).
  (hook.go, integrate.go, ...)   # other leaves — not touched.
internal/provider/
  executor.go           # Execute(ctx, spec, timeout, vb) — THE executor seam. REUSE.
  render.go             # CmdSpec struct. REUSE.
  registry.go           # Get/IsInstalled/DefaultProvider/List. REUSE.
  manifest.go           # Manifest.ListModelsCommand (DELIVERED by S1). READ-ONLY consume.
internal/config/
  role_defaults.go      # DefaultModelsForProvider(name) + (S2 adds) DefaultModelsVerificationDate constant.
docs/
  cli.md                # Subcommands section (lines 62–331). EDIT: add ### models before ## Exit codes (333).
```

### Desired Codebase tree with files to be added/edited

```bash
internal/cmd/models.go         # NEW — modelsCmd + flagModelsAll + runModels + runListModels + printLiveList + printCuratedTable
internal/cmd/models_test.go    # NEW — stub-binary, golden renderer, failure-fallback, error matrix
internal/config/role_defaults.go  # EDIT — +const DefaultModelsVerificationDate = "2026-07-02"
docs/cli.md                    # EDIT — +### `models [<provider>]` section before ## Exit codes
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (--all collision): root.go registers --all/-a as a PERSISTENT flag (flagAll, "git add -A").
//   FR-L1 needs `models --all`. Define a LOCAL modelsCmd.Flags().BoolVarP(&flagModelsAll, "all", "a",
//   false, <models text>). pflag's AddFlagSet SKIPS the inherited persistent flag when a local same-name
//   flag exists (proven by config.go:142's local --provider; documented in root.go's comment). MUST
//   define the -a shorthand too, else -a is left unbound and `stagecoach models -a` errors. NO panic.

// CRITICAL (ListModelsCommand is the FULL argv): it is [binary, args...] (e.g. ["pi","--list-models"]).
//   Build provider.CmdSpec{Command: argv[0], Args: argv[1:]}. Do NOT call Manifest.Render (that is the
//   commit-generation argv builder; ListModelsCommand is already complete). A 1-element argv is legal.

// CRITICAL (inherited env = nil Env): pass CmdSpec.Env as nil (NOT os.Environ()). executor.go: "nil Env
//   ⇒ the child inherits the parent env" — exactly "inherited env". Stdin MUST be "" so the child gets
//   /dev/null (else it blocks on stdin). Do NOT pass a non-empty Stdin.

// CRITICAL (fallback order, FR-L1): try the command FIRST; fall back to the curated table ONLY on (field
//   absent) OR (command fails: non-zero exit / context.DeadlineExceeded / start error). On fallback, the
//   curated table goes to STDOUT (it IS the answer); a one-line "list command failed; using curated
//   table" notice goes to STDERR (progress). A timeout/non-zero is NOT a hard error for the command as a
//   whole — exit 0 (the fallback succeeded). Only unknown/undetected/no-default are hard errors (exit 1).

// CRITICAL (fixed role order): iterate [planner, stager, message, arbiter] — NOT the map (random order
//   breaks the golden test). Empty stager cell (DefaultModelsForProvider returns "" for non-stager-
//   capable providers) renders as "—".

// CRITICAL (verification date is DATA now): role_defaults.go has the date only as a comment. S2 adds the
//   exported const DefaultModelsVerificationDate = "2026-07-02" and prints THAT in the footer. Keep the
//   const + roleDefaults in sync on every FR-D5 re-verification.

// CRITICAL (config loads for models): PersistentPreRunE runs for models (it is NOT in shouldSkipConfigLoad),
//   so Config() is non-nil in runModels. Do NOT add models to shouldSkipConfigLoad — it needs cfg.Providers
//   (user overrides) + cfg.Provider (default) + cfg.Timeout (bound). It runs in a git repo like providers.

// CRITICAL (never os.Exit): return exitcode.New(exitcode.Error, err) for every error path (exit 1); main
//   maps via exitcode.For. Matches providers.go exactly.
```

## Implementation Blueprint

### Data models and structure

No new data model. The command consumes existing types: `provider.CmdSpec`, `provider.Manifest`
(`ListModelsCommand`, via `DetectCommand()`), `config`'s `DefaultModelsForProvider` map. The only data
addition is ONE exported string constant (`DefaultModelsVerificationDate`).

```go
// internal/config/role_defaults.go — add near the FR-D5 verification block:
// DefaultModelsVerificationDate is the date the FR-D4 roleDefaults table was last verified (FR-D5).
// Surfaced by `stagecoach models` in the curated-fallback annotation (FR-L1). Update this AND roleDefaults
// together on each re-verification.
const DefaultModelsVerificationDate = "2026-07-02"

// internal/cmd/models.go — the command + helpers (sketch):
type modelTarget struct { name string; manifest provider.Manifest }
// (pure renderers take an io.Writer so they are unit-testable with a *bytes.Buffer — golden tests.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/role_defaults.go — add the verification-date constant
  - IMPLEMENT: `const DefaultModelsVerificationDate = "2026-07-02"` near the existing
    "Verification date: 2026-07-02" comment (keep them in lock-step; update both on FR-D5 refresh).
  - FOLLOW pattern: the file's existing FR-D5 comment block. The constant is exported (capital D) so
    internal/cmd can read config.DefaultModelsVerificationDate.
  - DO NOT: change roleDefaults itself, DefaultModelsForProvider, or any model value. ONE constant only.
  - DEPENDENCIES: none (the annotation in Task 4 reads it).

Task 2: CREATE internal/cmd/models.go — the command + resolver + executor call + renderers
  - IMPLEMENT: package cmd. The modelsCmd cobra leaf (Use "models [<provider>]", Short, Long quoting
    FR-L1 source order + never-HTTP; Args cobra.MaximumNArgs(1); SilenceErrors+SilenceUsage true;
    RunE runModels). A package var flagModelsAll bool. In init(): modelsCmd.Flags().BoolVarP(
    &flagModelsAll, "all", "a", false, "List models for every detected provider (default: the resolved
    default provider)") then rootCmd.AddCommand(modelsCmd).
  - IMPLEMENT runModels(cmd, args):
      1. if flagModelsAll && len(args) > 0 → return exitcode.New(exitcode.Error, fmt.Errorf(
         "--all cannot be combined with a provider argument")).
      2. reg, err := newRegistry(); if err != nil → exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)).
      3. installed := installedNames(reg); cfg := Config().
      4. Determine targets ([]modelTarget):
           - flagModelsAll: if len(installed)==0 → exitcode.New(exitcode.Error, fmt.Errorf(
             "no providers detected on $PATH; install one of stagecoach's supported agents")).
             else: for each name in installed → {name, manifest=reg.Get}.
           - len(args)==1 (name=args[0]): m, ok := reg.Get(name); if !ok → error "unknown provider %q".
             if name not in installed → error "provider %q is not detected on $PATH; install it or run
             'stagecoach models --all' for detected providers". targets=[{name,m}].
           - else (no arg): dflt := resolvedDefault(cfg, reg, installed); if dflt=="" → error "no
             provider detected on $PATH; pass a provider name or install one of stagecoach's supported
             agents". m,_ := reg.Get(dflt); targets=[{dflt,m}].
      5. For each target (in order): renderModelBlock(cmd, target, cfg). Separate blocks with a blank
         line when len(targets)>1.
      6. return nil.
  - IMPLEMENT renderModelBlock(cmd, t, cfg):
      argv := t.manifest.ListModelsCommand
      if len(argv) > 0 {
          timeout := 120*time.Second; if cfg != nil { timeout = cfg.Timeout }   // bound (FR25 knob)
          vb := (*ui.Verbose)(nil); if cfg != nil && cfg.Verbose { vb = ui.NewVerbose(cmd.ErrOrStderr(), true) }
          out, _, err := provider.Execute(cmd.Context(), provider.CmdSpec{Command: argv[0],
              Args: argv[1:], Stdin: ""}, timeout, vb)   // Env nil ⇒ inherit
          if err == nil { printLiveList(cmd.OutOrStdout(), t.name, out); return }
          // FR-L1 (b): command failed → curated fallback
          fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach: %s list command failed (%v); using curated table\n", t.name, err)
          printCuratedTable(cmd.OutOrStdout(), t)   // falls through to curated
      } else {
          printCuratedTable(cmd.OutOrStdout(), t)
      }
  - IMPLEMENT printCuratedTable(w io.Writer, t modelTarget):
      col := config.DefaultModelsForProvider(t.name)
      if col == nil {   // user-defined provider, no FR-D4 column
          fmt.Fprintf(w, "%s:\n  no list_models_command and no curated per-role defaults.\n  Add list_models_command to [provider.%s] or set [role.*] models in config.\n", t.name, t.name)
          return
      }
      fmt.Fprintf(w, "%s:\n", t.name)
      for _, role := range []string{"planner", "stager", "message", "arbiter"} {
          m := col[role]; if m == "" { m = "—" }
          fmt.Fprintf(w, "  %-8s %s\n", role, m)
      }
      cmd_ := t.manifest.DetectCommand()
      fmt.Fprintf(w, "\nStagecoach's curated per-role defaults (verified %s). The live list may differ — consult `%s --help`.\n",
          config.DefaultModelsVerificationDate, cmd_)
  - IMPLEMENT printLiveList(w io.Writer, name, stdout string):
      fmt.Fprintf(w, "%s:\n", name)
      if stdout == "" { fmt.Fprintf(w, "  (no models reported)\n"); return }
      fmt.Fprint(w, stdout)   // verbatim; do not trim (preserve the CLI's own formatting)
      if !strings.HasSuffix(stdout, "\n") { fmt.Fprintln(w) }   // tidy trailing for block separation
  - FOLLOW pattern: internal/cmd/providers.go (cobra leaf on root in init(); newRegistry/installedNames/
    resolvedDefault reuse; exitcode.New; cmd.OutOrStdout/ErrOrStderr; tabwriter NOT needed for the
    %-8s-aligned role table, but acceptable — keep it simple with %-8s).
  - NAMING: modelsCmd, flagModelsAll, runModels, renderModelBlock, printCuratedTable, printLiveList,
    modelTarget. CamelCase funcs, snake_case-free.
  - PLACEMENT: single new file internal/cmd/models.go.
  - DEPENDENCIES: Task 1 (DefaultModelsVerificationDate). Imports: fmt, io, strings, time, cobra,
    internal/config, internal/exitcode, internal/provider, internal/ui. (provider + config already
    imported by providers.go in the same package — no cycle; cmd→provider→(signal,ui), cmd→config.)

Task 3: EDIT docs/cli.md — the models subcommand section (Mode A)
  - IMPLEMENT: insert a `### \`models [<provider>]\`` section immediately BEFORE the `## Exit codes`
    heading (after the integrate section ends ~line 331). Content must cover: source-of-truth order
    ((a) run list_models_command → stdout; (b) curated FR-D4 table on absent/failure), default = the
    resolved default provider, --all = every detected provider, the local --all flag, the never-an-HTTP-
    call guarantee (§6.2 N2), unknown/undetected → exit 1, and one example block.
  - FOLLOW pattern: the existing `### \`providers show <name>\`` and `### \`providers list\`` sections
    (prose + a ```bash example block).
  - NAMING/PLACEMENT: H3 heading, snake/cobra-accurate (`models [<provider>]`), before `## Exit codes`.
  - DO NOT: edit the Global flags table (--all is documented there for the default action; the models
    local --all is described in THIS new section). Do NOT duplicate the FR-D4 table here (it lives in
    docs/providers.md already).
  - DEPENDENCIES: Task 2 (docs describe the shipped behavior).

Task 4: CREATE internal/cmd/models_test.go — stub-binary + golden + failure-fallback + error matrix
  - IMPLEMENT (reuse setupRepo/saveRootState/restoreRootState/writeConfigFile/Execute from the package;
    they live in root_test.go/providers_test.go):
    A. TestModels_CuratedGolden (DETERMINISTIC, no PATH juggling): call printCuratedTable(&buf,
       modelTarget{name:"claude", manifest:<claude manifest via reg or a minimal Manifest with
       Command/Name set>}) directly; assert the buf contains "claude:", "  planner   opus",
       "  stager    sonnet", "  message   haiku", "  arbiter   sonnet",
       "verified 2026-07-02", "consult `claude --help`". This is the golden test the contract names.
    B. TestModels_LiveList_StubBinary: write a fake `opencode` shell script (printf 'STAGECOACH_FAKE\n')
       to a temp dir; t.Setenv("PATH", tempDir+os.PathListSeparator+os.Getenv("PATH")) so opencode is
       DETECTED; Execute(["models","opencode"]); assert stdout contains "opencode:" +
       "STAGECOACH_FAKE". (opencode's built-in list_models_command=["opencode","models"] is run.)
       Guard: t.Skip if the script can't be made executable (Windows). Alternative: a user-defined
       [provider.stublist] with command + list_models_command pointing at a stub binary name.
    C. TestModels_CommandFailure_Fallback: fake `opencode` that exits 1; Execute(["models","opencode"]);
       assert STDOUT contains the curated footer ("consult `opencode --help`") AND STDERR contains the
       "list command failed" notice; exit code 0 (fallback succeeded).
    D. TestModels_Timeout_Fallback: fake `opencode` that sleeps 10s; set cfg via STAGECOACH_TIMEOUT=1s
       (or a tiny bound); assert curated table on stdout + exit 0 within a few seconds (proves the
       context timeout fired and the fallback ran).
    E. Error matrix (each asserts exitcode.For(err)==exitcode.Error + err string):
       TestModels_UnknownProvider ("ghost"), TestModels_UndetectedNamedProvider (a built-in not on
       PATH — e.g. claude with no fake binary), TestModels_NoDefault_NothingDetected (bare "models",
       nothing detected), TestModels_AllEmpty ("models","--all", nothing detected),
       TestModels_AllWithArg ("models","--all","opencode" → usage error).
    F. TestModels_DefaultResolved: with a fake `pi` (highest priority) on PATH, bare "models" prints
       pi's block (default resolution); with STAGECOACH_PROVIDER=claude + fake claude on PATH, bare
       "models" prints claude's block (explicit default).
    G. TestModels_HelpShowsAllScopedText: "models","--help" → output contains the models --all help
       text and does NOT contain "git add -A" (proves the local flag override).
  - FOLLOW pattern: internal/cmd/providers_test.go (the setup/save/restore/Execute/exitcode.For shape).
  - NAMING: TestModels_<Scenario>. COVERAGE: live success, curated success, command-failure fallback,
    timeout fallback, unknown, undetected, no-default, --all empty, --all+arg, default resolution, help.
  - PLACEMENT: internal/cmd/models_test.go (sibling of models.go; same package cmd).
  - DEPENDENCIES: Task 2 (the command + renderers).
```

### Implementation Patterns & Key Details

```go
// === models.go: the cobra leaf + local --all (overrides persistent; config.go:142 precedent) ===
var modelsCmd = &cobra.Command{
	Use:   "models [<provider>]",
	Short: "List models reachable by a provider",
	Long: `List the models a provider's CLI can reach — read straight from the agent CLI itself
(never an HTTP call: stagecoach has no API key; §6.2 N2 / FR-L1).

Source of truth, in order:
  (a) the manifest's list_models_command — run it, print its stdout; or
  (b) if absent or it fails — stagecoach's curated per-role tier table, annotated with its
      verification date and "consult '<command> --help' for the live list".

Default target is the resolved default provider; --all covers every detected provider.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runModels,
}
var flagModelsAll bool

func init() {
	modelsCmd.Flags().BoolVarP(&flagModelsAll, "all", "a", false,
		"List models for every detected provider (default: the resolved default provider)")
	rootCmd.AddCommand(modelsCmd) // NO edit to root.go (providers.go precedent)
}

// === runModels: resolve the target set, then render each block ===
func runModels(cmd *cobra.Command, args []string) error {
	if flagModelsAll && len(args) > 0 {
		return exitcode.New(exitcode.Error, fmt.Errorf("--all cannot be combined with a provider argument"))
	}
	reg, err := newRegistry() // reuse providers.go helper (same package)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	installed := installedNames(reg)      // reuse
	cfg := Config()                        // non-nil (PersistentPreRunE loaded it)
	targets, err := resolveModelTargets(args, reg, installed, cfg)
	if err != nil {
		return err // already exitcode-wrapped
	}
	for i, t := range targets {
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout()) // blank line between --all blocks
		}
		renderModelBlock(cmd, t, cfg)
	}
	return nil
}

// === renderModelBlock: FR-L1 (a) then (b) ===
func renderModelBlock(cmd *cobra.Command, t modelTarget, cfg *config.Config) {
	argv := t.manifest.ListModelsCommand
	if len(argv) > 0 {
		timeout := 120 * time.Second
		if cfg != nil {
			timeout = cfg.Timeout // bound (FR25 knob; default 120s)
		}
		var vb *ui.Verbose
		if cfg != nil && cfg.Verbose {
			vb = ui.NewVerbose(cmd.ErrOrStderr(), true)
		}
		out, _, err := provider.Execute(cmd.Context(),
			provider.CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}, // Env nil ⇒ inherit
			timeout, vb)
		if err == nil {
			printLiveList(cmd.OutOrStdout(), t.name, out)
			return
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach: %s list command failed (%v); using curated table\n", t.name, err)
		// fall through to curated
	}
	printCuratedTable(cmd.OutOrStdout(), t)
}

// === printCuratedTable: FIXED role order; empty stager → "—" ===
func printCuratedTable(w io.Writer, t modelTarget) {
	col := config.DefaultModelsForProvider(t.name)
	if col == nil {
		fmt.Fprintf(w, "%s:\n  no list_models_command and no curated per-role defaults.\n  Add list_models_command to [provider.%s] or set [role.*] models in config.\n", t.name, t.name)
		return
	}
	fmt.Fprintf(w, "%s:\n", t.name)
	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		m := col[role]
		if m == "" {
			m = "—" // e.g. non-stager-capable providers
		}
		fmt.Fprintf(w, "  %-8s %s\n", role, m)
	}
	fmt.Fprintf(w, "\nStagecoach's curated per-role defaults (verified %s). The live list may differ — consult `%s --help`.\n",
		config.DefaultModelsVerificationDate, t.manifest.DetectCommand())
}

// === printLiveList: heading + verbatim stdout ===
func printLiveList(w io.Writer, name, stdout string) {
	fmt.Fprintf(w, "%s:\n", name)
	if stdout == "" {
		fmt.Fprintf(w, "  (no models reported)\n")
		return
	}
	fmt.Fprint(w, stdout)
	if !strings.HasSuffix(stdout, "\n") {
		fmt.Fprintln(w)
	}
}
```

### Integration Points

```yaml
COMMAND REGISTRATION (models.go init()):
  - add to root: "rootCmd.AddCommand(modelsCmd) — NO edit to root.go (providers.go precedent)"
  - flag: "modelsCmd.Flags().BoolVarP(&flagModelsAll, 'all', 'a', false, <models text>) — LOCAL flag
           overrides the inherited persistent --all (config.go:142 mechanism)"

CONFIG (NO resolver change):
  - consumed: "cfg.Providers (user [provider.*] overrides via newRegistry), cfg.Provider (default),
    cfg.Timeout (bound), cfg.Verbose (verbose exec)"
  - added: "config.DefaultModelsVerificationDate constant (one exported string; printed in the footer)"

PROVIDER EXECUTOR (REUSE, no change):
  - seam: "provider.Execute(ctx, CmdSpec{Command:argv[0], Args:argv[1:], Stdin:''}, timeout, vb)"
  - env: "CmdSpec.Env nil ⇒ child inherits parent env (executor.go contract)"

REGISTRY (REUSE, no change):
  - "reg.Get(name) → merged manifest (read ListModelsCommand + DetectCommand()); reg.IsInstalled /
    installedNames for detection; resolvedDefault for the default"

FALLBACK (REUSE, no change):
  - "config.DefaultModelsForProvider(name) → FR-D4 per-role column (nil for user-defined)"

DOCS (docs/cli.md):
  - add: "### `models [<provider>]` section before ## Exit codes (source order, --all, never-HTTP)"

DOWNSTREAM CONSUMERS (NOT this task):
  - P1.M6.T2.S1: "config init --interactive wizard MAY surface the listing (reads ListModelsCommand /
    DefaultModelsForProvider the same way this command does)"
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file edit — fix before proceeding.
gofmt -w internal/cmd/models.go internal/cmd/models_test.go internal/config/role_defaults.go
go vet ./internal/cmd/... ./internal/config/...
golangci-lint run ./internal/cmd/... ./internal/config/...

# Expected: zero errors. gofmt -l prints nothing for edited files.
gofmt -l internal/cmd/ internal/config/
```

### Level 2: Unit Tests (Component Validation)

```bash
# The models command tests (Task 4): golden renderer, stub-binary live list, failure+timeout fallback,
# and the error matrix.
go test ./internal/cmd/ -run TestModels -v

# The verification-date constant is present + correct (a focused assertion or via TestModels_CuratedGolden).
go test ./internal/config/ -run TestRoleDefaults -v

# Full cmd + config packages (no regression in providers/config/etc.).
go test ./internal/cmd/... ./internal/config/... -v

# Expected: all pass. If TestModels_CuratedGolden fails on a role/model line, the renderer format drifted
# from the golden string — fix the renderer (do NOT change DefaultModelsForProvider values).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
go build ./...

# Curated fallback for a detected provider with no list_models_command (put a fake claude on PATH).
mkdir -p /tmp/m-bin && printf '#!/bin/sh\nexit 0\n' >/tmp/m-bin/claude && chmod +x /tmp/m-bin/claude
PATH=/tmp/m-bin:$PATH ./stagecoach models claude
# Expected stdout: "claude:" + the 4 role lines + the "verified 2026-07-02 ... consult `claude --help`" footer.

# Live list for a detected provider whose list_models_command succeeds (fake opencode prints a list).
printf '#!/bin/sh\necho "gpt-5.4"; echo "gpt-5.4-mini"\n' >/tmp/m-bin/opencode && chmod +x /tmp/m-bin/opencode
PATH=/tmp/m-bin:$PATH ./stagecoach models opencode
# Expected stdout: "opencode:" + the two model lines.

# Command-FAILURE fallback (fake opencode exits 1).
printf '#!/bin/sh\nexit 1\n' >/tmp/m-bin/opencode2 && ... ; PATH=... ./stagecoach models opencode
# Expected: curated table on stdout + "opencode list command failed ... using curated table" on stderr; exit 0.

# --all over detected providers.
PATH=/tmp/m-bin:$PATH ./stagecoach models --all
# Expected: a block per detected provider, blank-line separated.

# Error cases (exit 1 each).
./stagecoach models ghost                      # unknown provider
./stagecoach models nonexistent-cli            # (a known builtin not on PATH, e.g. with a clean PATH)
./stagecoach models --all opencode             # --all + arg → usage error

# Help shows the models-scoped --all text (NOT "git add -A").
./stagecoach models --help | grep -i "every detected provider"

# Expected: all behave as documented; nonzero exits are 1 (exitcode.Error).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Never-an-HTTP-call audit (§6.2 N2): confirm models.go + its transitive imports make NO net/http call.
grep -rn "net/http" internal/cmd/models.go internal/provider/executor.go || echo "OK: no net/http in the models path"
# Expected: no net/http usage in the models command path (provider.Execute uses only os/exec).

# Verify the list_models_command argv actually run are S1's verified values (FR-D5 live re-confirm).
# (Re-run each populated CLI per S1's research; if a CLI dropped its listing, the failure path covers it.)
opencode models 2>/dev/null | head -2 || echo "opencode not installed → curated fallback will run"

# Golden renderer stability: re-run TestModels_CuratedGolden after a `gofmt -w` to ensure formatting
# did not perturb the asserted substrings (the %-8s alignment is format-sensitive).
go test ./internal/cmd/ -run TestModels_CuratedGolden -v

# Expected: no net/http in the path; the golden renderer is stable under gofmt.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed.
- [ ] `go build ./...` succeeds.
- [ ] `go test ./internal/cmd/... ./internal/config/... -v` — all pass (incl. TestModels_* + TestRoleDefaults).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean.
- [ ] `gofmt -l internal/cmd/ internal/config/` prints nothing.

### Feature Validation

- [ ] `modelsCmd` registered on root in `init()`; `Use "models [<provider>]"`; `MaximumNArgs(1)`; local `--all`/`-a`.
- [ ] `runModels` resolves `--all` / explicit arg (known+detected) / default; `--all`+arg is a usage error.
- [ ] Live-list path runs `provider.Execute` on `ListModelsCommand` (inherited env, `cfg.Timeout` bound).
- [ ] Command-failure/timeout path falls back to the curated table on stdout + a stderr notice; exit 0.
- [ ] `printCuratedTable` uses FIXED role order, empty stager → "—", prints the verification date + consult hint.
- [ ] `DefaultModelsVerificationDate` constant added (= "2026-07-02") and printed.
- [ ] Unknown/undetected/no-default/`--all`-empty errors exit 1 via `exitcode.New(exitcode.Error, …)`.
- [ ] `stagecoach models --help` shows the models-scoped `--all` text (NOT "git add -A").
- [ ] `docs/cli.md` has the `### \`models [<provider>]\`` section before `## Exit codes`.

### Code Quality Validation

- [ ] Follows the `providers.go` cobra-leaf pattern exactly (init() registration, newRegistry reuse, exitcode).
- [ ] Reuses `provider.Execute`, `Registry.Get/IsInstalled/DefaultProvider`, `DefaultModelsForProvider` — no duplication.
- [ ] Local `--all` override uses the proven `config.go:142` mechanism (both long + shorthand defined).
- [ ] Renderers are pure (take `io.Writer`) so they are golden-testable with a `*bytes.Buffer`.
- [ ] No `os.Exit`; no `net/http`; no new dependencies; no manifest-schema change (S1 owns that).

### Documentation & Deployment

- [ ] `docs/cli.md` section states source order (a)/(b), default, `--all`, and the never-HTTP guarantee (N2).
- [ ] `modelsCmd.Long` help text states the same (FR-L1 + N2) for `--help`.

---

## Anti-Patterns to Avoid

- ❌ Don't re-implement `newRegistry`/`installedNames`/`resolvedDefault` — they are unexported helpers in
  package `cmd` (providers.go); call them directly (same package).
- ❌ Don't call `Manifest.Render` to build the list argv — `ListModelsCommand` is ALREADY the full argv
  (binary + args); build `CmdSpec{Command: argv[0], Args: argv[1:]}` directly.
- ❌ Don't pass `CmdSpec.Env = os.Environ()` — nil already means "inherit parent env" (executor.go); pass nil.
- ❌ Don't pass a non-empty `Stdin` — the child would block reading it; Stdin MUST be "".
- ❌ Don't treat a `list_models_command` failure (non-zero/timeout/not-found) as a hard command error — it
  is the FR-L1(b) fallback trigger; print the curated table (stdout) + a stderr notice, exit 0. Only
  unknown/undetected/no-default are hard errors (exit 1).
- ❌ Don't iterate `DefaultModelsForProvider`'s map directly — map order is random and breaks the golden
  test. Iterate a fixed `[planner, stager, message, arbiter]` slice.
- ❌ Don't redefine `--all` without the `-a` shorthand — pflag skips the parent's whole `--all` on a
  name collision, leaving `-a` unbound (use `BoolVarP(..., "all", "a", ...)`).
- ❌ Don't add `models` to `shouldSkipConfigLoad` — it NEEDS the resolved config; keep it on the normal
  load path like `providers`.
- ❌ Don't duplicate the FR-D4 table — `config.DefaultModelsForProvider` IS the table; reuse it.
- ❌ Don't touch the manifest schema, `providers/*.toml`, `builtin.go`, or `merge.go` — that is S1's
  contract (the parallel task); S2 only CONSUMES `Manifest.ListModelsCommand`.
- ❌ Don't add a config key or persistent flag for models — its only flag is the local `--all` on the
  command itself.
- ❌ Don't use `net/http` anywhere in the models path — never an HTTP call (§6.2 N2 / FR-L1).
