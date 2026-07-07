---
name: "P1.M4.T1.S1 — Root command, global flags, and custom exit-code wiring (cobra) — PRD §15.2 / §15.4 / §21.1"
description: |

  Build the cobra CLI scaffold for Stagecoach: the ROOT command (PRD §15.1), ALL §15.2 global flags,
  a `PersistentPreRunE` that loads the fully-resolved config via `config.Load()` (skipping for the
  future `config init`/`config path` + auto-skipped help/version), a new `internal/exitcode` package
  that maps any error to a PRD §15.4 exit code (0/1/2/3/124), and a real `cmd/stagecoach/main.go`
  that wires Execute → `exitcode.For` → `os.Exit` + declares `var version string` (the Makefile's
  `-ldflags -X main.version=…` target). Add `github.com/spf13/cobra` to go.mod.

  The default-action BODY (auto-stage-all → `stagecoach.GenerateCommit` → report) and the
  `providers`/`config` SUBCOMMANDS land in S2/S3/S4 respectively; signal handling in P1.M4.T2;
  UI/color/verbose in P1.M4.T3; the `--dry-run` RunE in P1.M4.T4. S1 ships the SCAFFOLD they hang
  on: a binary that parses every global flag, loads config once, prints help by default, prints
  `--version`, and exits with the correct code on any error.

  DELIVERABLES (NEW files + targeted edits; nothing under `internal/{config,generate,git,prompt,
  provider}` or `pkg/stagecoach` is touched):
    1. CREATE `internal/exitcode/exitcode.go`      — ExitError + New + For + §15.4 constants.
    2. CREATE `internal/exitcode/exitcode_test.go` — For() mapping matrix (all §15.4 codes).
    3. CREATE `internal/cmd/root.go`               — rootCmd + all §15.2 persistent flags +
       PersistentPreRunE (config load + skip) + Execute(ctx) + Version/config store + stub RunE.
    4. CREATE `internal/cmd/root_test.go`          — flag registration, --version, config loading
       through cobra (precedence), SilenceErrors.
    5. REWRITE `cmd/stagecoach/main.go`             — `var version string`; ctx; cmd.Execute(ctx);
       exitcode.For; os.Exit; baseline error print.
    6. MODIFY `go.mod`/`go.sum`                    — add `github.com/spf13/cobra`.

  CONTRACT (PRD §15.2 — every global flag below MUST exist on root, persistent, inheriting to all
  subcommands; §15.4 — the 5 exit codes; §21.1 — `make build` injects version):
    - `--provider <name>` (env `STAGECOACH_PROVIDER`, git `stagecoach.provider`, default auto-detected)
    - `--model <name>`    (env `STAGECOACH_MODEL`,    git `stagecoach.model`,    default manifest `default_model`)
    - `--config <path>`   (env `STAGECOACH_CONFIG`,   no git, default resolved path) → `LoadOpts.ConfigPathOverride`
    - `--timeout <dur>`   (env `STAGECOACH_TIMEOUT`,  git `stagecoach.timeout`,  default 120s)  [STRING flag]
    - `--all`, `-a`; `--no-auto-stage`; `--dry-run`; `--verbose`, `-v`; `--no-color`; `--version`; `--help`, `-h`
    Config-backed flags (provider/model/config/timeout/verbose/no-color) are registered with ZERO
    defaults; `config.Load` applies the real precedence via `fs.Changed` (Layer 7). Behavioral flags
    (all/no-auto-stage/dry-run) are package vars read by S2/S4. `--version` uses cobra's `Version`
    field (short-circuits before PersistentPreRunE). `--help/-h` is cobra's built-in.

  SCOPE NOTE (forward dependency, load-bearing): the task INPUT lists "ExitError type from
  P1.M4.T3.S3", but P1.M4.T3.S3 is Planned/NOT built and runs AFTER S1, while S1's `main()` MUST
  call `exitcode.For(err)`. There is no `internal/exitcode` package in the tree today. THEREFORE
  **S1 CREATES `internal/exitcode`** (decision §0 in research/design-decisions.md). When
  P1.M4.T3.S3 runs it must NOT recreate the package (it is FROZEN after S1); it should instead own
  the UI-layer error→ExitError wrapping at the RunE boundary. This PRP documents that; it does NOT
  edit tasks.json (read-only on the plan).

  SCOPE BOUNDARY (what S1 does NOT do — owned by siblings): default commit action (S2 → rootCmd.RunE
  body); providers list/show + config init/path subcommands (S3/S4 → rootCmd.AddCommand); signal
  handling (P1.M4.T2 → swaps context.Background for signal.NotifyContext); color/TTY/verbose UI
  (P1.M4.T3 → refines main's error print, NoColor TTY detection); dry-run RunE (P1.M4.T4 → reads
  flagDryRun). S1 REGISTERS the --all/--no-auto-stage/--dry-run flags (so help is complete) but
  implements NONE of their behavior.

  INPUT (upstream — READ-ONLY contracts): `config.Load(ctx, LoadOpts{ConfigPathOverride, RepoDir,
  Flags}) (*Config, error)` + `Config`/`Defaults()` (internal/config, P1.M1.T4 — load.go reads
  flags via `fs.Changed`+`fs.GetString("timeout")`; timeout is a STRING flag, FINDING 7).
  `generate.ErrNothingToCommit`/`ErrTimeout`/`ErrRescue`/`ErrCASFailed` + `*generate.RescueError`
  (Unwrap→Kind) + `*generate.CASError` (Unwrap→git.ErrCASFailed) (internal/generate, P1.M3.T4.S2 —
  drives exitcode.For's mapping). `pkg/stagecoach.GenerateCommit` (P1.M3.T5.S1, parallel — the seam
  S2 will call; S1 does not import it). `Makefile` (`-X main.version=$(VERSION)` already wired).

  OUTPUT (downstream consumers): S2 sets `rootCmd.RunE` to the default action and reads
  `cmd.Config()` + the behavioral flag vars; S3/S4 `rootCmd.AddCommand(...)`; P1.M4.T2 swaps
  `cmd.Execute(ctx)`'s ctx; P1.M4.T3 refines main's error output; P1.M4.T4 reads `flagDryRun`.

  ⚠️ Follow PRD §15.4 exit codes, NOT architecture/go_ecosystem_patterns.md §1.2's generic table
  (which says 2=usage, 3=config). PRD: 2=nothing-to-commit, 3=rescue. (design §1)
  ⚠️ `--timeout` MUST be a StringVar, NOT pflag.Duration — config.Load reads it via
  fs.GetString("timeout"); a Duration flag silently breaks `--timeout`. (design §2)
  ⚠️ Register the 6 config-backed flags with ZERO pflag defaults (""/false) so fs.Changed reflects
  "user passed it"; config.Load owns the real Layer-7 precedence. (design §3)
  ⚠️ Cobra short-circuits `--help`/`--version` BEFORE PersistentPreRunE — config does NOT load for
  those. The only explicit skip needed is `cmd.Name()=="init"||cmd.Name()=="path"` (for S4). (design §5/§6)

  Deliverable: 4 NEW files + rewritten main.go + go.mod/go.sum. `make build` produces
  `./bin/stagecoach` that parses flags, loads config, prints `--version`, and exits with §15.4 codes.

---

## Goal

**Feature Goal**: Ship Stagecoach's cobra CLI foundation (PRD §15.1/§15.2/§15.4/§21.1): a root command
holding all eleven §15.2 global flags (persistent, inherited by every future subcommand), a
`PersistentPreRunE` that resolves config exactly once via the existing `config.Load()` 7-layer
precedence, a centralized `internal/exitcode` package mapping any error to the precise PRD §15.4
exit code (0/1/2/3/124), and a real `main.go` that wires `Execute(ctx)` → `exitcode.For(err)` →
`os.Exit` and declares the `var version string` the Makefile's `-ldflags -X main.version=` targets.
This is the scaffold the default action (S2), subcommands (S3/S4), signals (P1.M4.T2), UI (P1.M4.T3),
and dry-run (P1.M4.T4) hang on.

**Deliverable** (4 NEW files + rewritten main.go + go.mod/go.sum; NO edits under internal/{config,
generate,git,prompt,provider} or pkg/stagecoach):
1. `internal/exitcode/exitcode.go` — `package exitcode`. `type ExitError{Code; Err}`, `func New(code,
   err) *ExitError`, `func For(err) int` (full §15.4 matrix), constants `Success/Error/
   NothingToCommit/Rescue/Timeout`.
2. `internal/exitcode/exitcode_test.go` — `package exitcode`. Pure unit tests: every error shape → code.
3. `internal/cmd/root.go` — `package cmd`. `rootCmd` (`SilenceErrors`+`SilenceUsage`), all §15.2
   persistent flags (bound to package vars), `PersistentPreRunE` (config load + init/path/help skip),
   `func Execute(ctx) error`, `var Version string`, config store + `Config()` accessor, stub RunE.
4. `internal/cmd/root_test.go` — `package cmd`. Flag registration/defaults, `--version` output,
   config loading through cobra (Layer precedence), `SilenceErrors` behavior. Own git/CWD helpers.
5. `cmd/stagecoach/main.go` — `package main`. `var version string`; `ctx := context.Background()`;
   `cmd.Version = version`; `err := cmd.Execute(ctx)`; `os.Exit(exitcode.For(err))`; baseline stderr
   error print.
6. `go.mod`/`go.sum` — `go get github.com/spf13/cobra@latest` (+ transitive deps in go.sum).

**Success Definition**: `make build` → `./bin/stagecoach`; `./bin/stagecoach --version` prints the
version; `./bin/stagecoach --help` lists ALL §15.2 flags with descriptions matching PRD §15.2/FR35;
`./bin/stagecoach` (inside a git repo) loads config and prints help (stub RunE); any error returns
the correct §15.4 code via `exitcode.For`. `go test -race ./internal/exitcode/ ./internal/cmd/`
green; `go test -race ./...` shows NO regression; `go vet ./...` clean; `gofmt -l` empty; only
go.mod/go.sum + the listed files changed.

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 primary persona "the plan-holder") who runs
`stagecoach` at the terminal, and transitively lazygit/CI integrators who invoke `stagecoach`
non-interactively (PRD §15.5 lazygit example). For S1 specifically: a user who can already run the
binary, see the complete global-flag help surface, set any flag/env/git-config, get `--version`,
and observe correct exit codes — even though the default commit action is a stub until S2.

**Use Case**: `stagecoach --version` (release sanity); `stagecoach --help` (discover flags); `stagecoach
--provider claude --model sonnet --dry-run` (flags parse + config resolves; the action body is S2/S4);
`stagecoach` with a broken config file exits `1` with a clear message.

**User Journey**: user runs `stagecoach [<flags>]` → cobra parses flags → `PersistentPreRunE` resolves
config (defaults→file→git→env→flags) → (stub) RunE runs → main maps any error to a §15.4 exit code.
For `--version`/`--help`, cobra short-circuits before config load and exits 0.

**Pain Points Addressed**: (1) discoverability — the full §15.2 flag table is in `--help` (Mode A docs);
(2) deterministic exit codes — scripts/lazygit/CI can branch on 0/1/2/3/124 (PRD §15.4); (3) one
config-resolution path — every future subcommand inherits the same `config.Load` via the persistent
pre-run.

## Why

- **It IS the CLI scaffold.** Everything in P1.M4 hangs on a working root command + persistent flags
  + one config-load path + exit-code wiring. Shipping it first means S2/S3/S4 are thin additions
  (`rootCmd.RunE = …`, `rootCmd.AddCommand(…)`).
- **Closes the "CLI owns exit codes" loop (PRD §15.4).** The pipeline (`generate.CommitStaged` /
  `pkg/stagecoach.GenerateCommit`) returns typed errors; the CLI is the ONLY layer that may call
  `os.Exit`. `exitcode.For` centralizes that mapping so it can't drift across commands.
- **Honors the build contract (§21.1).** `make build` already passes `-X main.version=$(VERSION)`; it
  is a silent no-op until `main.go` declares `var version string`. S1 is the natural owner of main.go.
- **Resolves the S3 forward dependency (design §0).** `internal/exitcode` is a hard dependency of
  main(); creating it in S1 (the first consumer) avoids a blocking ordering gap.

## What

A cobra root command with `SilenceErrors`+`SilenceUsage`, eleven persistent global flags (six
config-backed registered at zero default so `config.Load`'s `fs.Changed` precedence works; three
behavioral package-vars; `--version` via cobra's `Version` field; `--help` built-in), a
`PersistentPreRunE` that calls `config.Load(ctx, LoadOpts{ConfigPathOverride: flagConfig,
RepoDir: os.Getwd(), Flags: cmd.Flags()})` and stores the result (skipping for the future
`config init`/`config path`), a stub RunE that prints help, an `internal/exitcode` package mapping
errors → §15.4 codes, and a `main.go` that executes + exits. No business logic; no signals; no color;
no verbose output; no commit; no subcommand bodies.

### Success Criteria

- [ ] `internal/exitcode/exitcode.go` exists, `package exitcode`, imports `errors`+`context`+
      `github.com/dustin/stagecoach/internal/generate` ONLY. Exports `ExitError`, `New`, `For`,
      `Success`, `Error`, `NothingToCommit`, `Rescue`, `Timeout`. Has a `// Package exitcode …` doc.
- [ ] `For(nil)==0`; `For(ExitError{Code:2,…})==2`; `For(generate.ErrNothingToCommit)==2`;
      `For(&generate.RescueError{Kind:ErrRescue})==3`; `For(&generate.RescueError{Kind:ErrTimeout})==124`;
      `For(context.DeadlineExceeded)==124`; `For(generate.ErrCASFailed)==1`; `For(errors.New("x"))==1`.
- [ ] `internal/cmd/root.go` exists, `package cmd`, imports `cobra`+`pflag`+`context`+`fmt`+`os`+
      `github.com/dustin/stagecoach/internal/{config,exitcode}`. `rootCmd` has `SilenceErrors`+
      `SilenceUsage` true. ALL eleven §15.2 flags exist on `rootCmd.PersistentFlags()` with the exact
      names + shorthands from §15.2 (`-a` for all, `-v` for verbose, `-h` help built-in).
- [ ] `PersistentPreRunE` calls `config.Load` with `ConfigPathOverride=flagConfig`,
      `RepoDir=os.Getwd()`, `Flags=cmd.Flags()`; stores the result; returns `nil` when
      `cmd.Name()=="init"||cmd.Name()=="path"`. On `config.Load` error returns
      `exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err))`.
- [ ] `--version` prints the version (cobra `Version` field) and exits 0 WITHOUT loading config.
- [ ] `cmd/stagecoach/main.go` declares `var version string`, sets `cmd.Version = version`, calls
      `cmd.Execute(ctx)`, and `os.Exit(exitcode.For(err))`. Prints `stagecoach: <err>\n` to stderr for
      a non-empty error.
- [ ] `go.mod` adds `github.com/spf13/cobra`; `go build ./...` + `make build` succeed;
      `go test -race ./...` green; `go vet ./...` clean; `gofmt -l` empty; only listed files changed.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
upstream signatures (all quoted below), the design decisions (research/design-decisions.md — esp.
§0 the S3 forward-dep, §1 PRD-over-arch exit codes, §2 timeout-as-string, §3 zero flag defaults), the
PRD §15.2/§15.4 contracts (in `selected_prd_content`), the test conventions to mirror
(`internal/config/load_test.go`), and the copy-ready skeletons in the Implementation Blueprint. No
default-action/signal/UI/subcommand knowledge required.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T1S1/research/design-decisions.md
  why: the 11 decisions specific to this subtask. §0 (S1 creates internal/exitcode — forward dep on
       S3), §1 (PRD §15.4 OVERRIDES the arch doc's exit table), §2 (--timeout is a STRING flag),
       §3 (zero flag defaults; config.Load owns Layer 7 via fs.Changed), §4 (behavioral flag vars),
       §5 (--version via cobra.Version short-circuits), §6 (PersistentPreRunE skip = init/path),
       §7 (SilenceErrors; main prints baseline error), §8 (context.Background; signals are T2),
       §9 (root RunE is a stub for S2), §10 (cobra to go.mod), §11 (test strategy + copied helpers).
  critical: §0 (WHY S1 owns the exitcode package — read FIRST), §1 (do NOT copy arch doc's 2/3
       meanings), §2 (timeout-as-string is a silent-failure trap), §3 (zero defaults or Changed breaks).

- docfile: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: §1 (cobra command tree, persistent flags, PersistentPreRunE), §1.2 (ExitError/New/For shape),
       §1.4 (PersistentPreRunE config-loading pattern + the init/path skip), Appendix A (main.go +
       context wiring), Appendix B (go.mod deps).
  why: the idiomatic cobra scaffolding to follow (rootCmd layout, PersistentFlags, Execute, the
       *ExitError + errors.As pattern). The exit-code TABLE in §1.2 is a GENERIC reference — OVERRIDE
       its 2/3 meanings with PRD §15.4 (design §1).
  pattern: SilenceErrors+SilenceUsage on root; Execute() returns the error (no os.Exit inside cobra);
       the CALLER (main) maps it to an exit code. PersistentPreRunE loads config once for all subcommands.
  gotcha: cobra runs a child's PersistentPreRunE INSTEAD of the root's if the child defines one — so
       S3/S4 subcommands must NOT define their own PersistentPreRunE (they rely on root's). cobra
       short-circuits --help/--version before PersistentPreRunE (so no skip code needed for those).

- file: internal/config/load.go   (P1.M1.T4.S4 — READ for Load + LoadOpts; do NOT edit)
  section: `type LoadOpts struct{ ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet }` +
       `func Load(ctx context.Context, opts LoadOpts) (*Config, error)` + `func loadFlags(cfg, fs)`
       + `func parseTimeout(s string) (time.Duration, error)`.
  why: PersistentPreRunE calls `config.Load(ctx, config.LoadOpts{ConfigPathOverride: flagConfig,
       RepoDir: repoDir, Flags: cmd.Flags()})`. Path resolution is INSIDE Load (--config >
       STAGECOACH_CONFIG > discovery) — the CLI only forwards the --config string. loadFlags reads ONLY
       flags where `fs.Changed(name)` via `fs.GetString`/`fs.GetBool` (Layer 7, highest precedence).
  pattern: Flags==nil skips the flag layer (programmatic callers); the CLI passes cmd.Flags(). The 5
       config-backed flags loadFlags reads are EXACTLY: provider, model, timeout, verbose, no-color.
  gotcha: loadFlags reads timeout via fs.GetString("timeout") (FINDING 7) → --timeout MUST be a
       StringVar; a Duration flag silently no-ops it. parseTimeout accepts "120s" AND bare "120".

- file: internal/config/config.go   (P1.M1.T4.S1 — READ for Config fields; do NOT edit)
  section: `type Config struct { Provider, Model string; Timeout time.Duration; AutoStageAll, Verbose,
       NoColor bool; MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars int; Output
       string; StripCodeFence bool; Providers map[string]map[string]any }` + `func Defaults() Config`.
  why: confirms which flags ARE config fields (Provider/Model/Timeout/Verbose/NoColor) vs NOT
       (there is NO All / NoAutoStage / DryRun / Version field — those are behavioral, flag-only).
       NoColor is `toml:"-"` (CLI/UI-only). Defaults() = Layer 1 (Timeout 120s, AutoStageAll true, …).
  gotcha: the config-backed flags are registered at ZERO default so fs.Changed works; the effective
       defaults (120s etc.) come from Defaults() applied by Load, NOT from pflag defaults.

- file: internal/generate/generate.go   (P1.M3.T4.S2 — READ for the error types; do NOT edit)
  section: `var ErrNothingToCommit/ErrTimeout/ErrRescue` (sentinels) + `var ErrCASFailed = git.ErrCASFailed`
       + `type RescueError struct{ Kind error; TreeSHA, ParentSHA, Candidate string; Cause error }`
       (Unwrap→Kind) + `type CASError struct{…}` (Unwrap→git.ErrCASFailed).
  why: exitcode.For() maps these to §15.4 codes. errors.Is(err, generate.ErrNothingToCommit)→2;
       *RescueError with Kind==ErrRescue (errors.Is ErrRescue)→3; Kind==ErrTimeout→124;
       ErrCASFailed/*CASError→1. context.DeadlineExceeded→124 (bare, no snapshot).
  pattern: RescueError.Unwrap()==Kind, so errors.Is(err, generate.ErrTimeout) works on a *RescueError
       whose Kind is ErrTimeout. CASError.Unwrap()==git.ErrCASFailed==generate.ErrCASFailed.
  gotcha: a *RescueError is returned for BOTH timeout (Kind=ErrTimeout) and rescue (Kind=ErrRescue);
       For() must distinguish via errors.Is(err, generate.ErrTimeout) BEFORE the generic rescue check.

- file: internal/config/load_test.go   (P1.M1.T4.S4 — READ for the TEST PATTERN + helpers; do NOT edit)
  section: `loadEnvSetup(t)` (HOME/XDG isolation + temp git repo) + `chdir(t, dir)` + `newFlagSet(t)`
       (the 5 config-backed flags at zero default) + `writeConfigFile(t, dir, rel, body)` + the
       precedence-matrix tests.
  why: root_test.go MIRRORS this file's approach (real git + temp repo + CWD isolation + flag
       overrides) at the cobra boundary. The helpers are package-private to internal/config/git →
       UNIMPORTABLE from package cmd — copy the ~25-line set (initRepo from git_test.go,
       setGitConfig from config/git_test.go, writeConfigFile/chdir/loadEnvSetup from load_test.go).
  gotcha: initRepo sets GIT_AUTHOR/COMMITTER identity via env. chdir() restores CWD in t.Cleanup.
       newFlagSet uses STRING for timeout — mirror that exact registration in root.go.

- file: cmd/stagecoach/main.go   (P1.M1.T1.S1 stub — REWRITE this file)
  section: currently `package main\n\nfunc main() {}` — a 29-byte placeholder.
  why: S1 replaces it with the real main: `var version string`, ctx, cmd.Execute(ctx), exitcode.For,
       os.Exit. The Makefile's `-ldflags "-X main.version=…"` targets this `var version string`.
  gotcha: the `var version string` MUST be in package main (the binary's main pkg) for -X to work;
       cmd's Version is set FROM it (main sets cmd.Version = version before Execute).

- file: Makefile   (P1.M1.T1.S2 — READ; do NOT edit)
  section: `VERSION ?= dev`, `LDFLAGS := -X main.version=$(VERSION)`, `build:` target → `./bin/stagecoach`.
  why: confirms the build injects version into `main.version`; the `var version string` S1 adds in
       main.go is exactly what makes this effective (currently a silent no-op per the Makefile NOTE).
  pattern: `make build` is the validation command; `make test` runs `go test -race ./...`.

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1, parallel/Implementing — READ the contract; do NOT edit)
  section: `func GenerateCommit(ctx context.Context, opts Options) (Result, error)` + `type Options`
       + `type Result`.
  why: this is the seam S2's default action will call ("parse flags → maybe auto-stage →
       GenerateCommit → print result"). S1 does NOT import it (no default action yet), but the
       stub RunE + config store + behavioral flag vars are S1's contribution toward S2.
  gotcha: GenerateCommit is signal/exit-agnostic (returns Result/errors). The CLI's job (S2) is to
       map those errors via exitcode.For. S1 wires that mapping; S2 feeds it real errors.

- url: (PRD §15.1 synopsis, §15.2 global flags, §15.4 exit codes, §16.1/§16.3 precedence, §21.1 build
       — already in context as selected_prd_content `h3.52`/`h3.53`/`h3.55`/`h3.72` + `h2.15`;
       ALSO plan/001_f1f80943ac34/prd_snapshot.md §15, §16)
  why: §15.2 is the AUTHORITATIVE flag table (names, env, git-config, defaults, descriptions — Mode A
       docs ride with this subtask: help text must match §15.2/FR35). §15.4 is the AUTHORITATIVE
       exit-code table (drives exitcode.For + the constants). §21.1 confirms `make build` + ldflags.
  critical: §15.2 lists `--all/-a`, `--no-auto-stage`, `--dry-run`, `--verbose/-v`, `--no-color`,
       `--version`, `--help/-h` — ALL must be registered even though their RunE logic is S2/S4.
       §15.4 codes 0/1/2/3/124 (NOT the arch doc's generic 2=usage/3=config).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (cobra ADDED by this subtask)
go.sum                          # grows: cobra + transitive deps
cmd/stagecoach/main.go           # 29-byte stub (P1.M1.T1) — REWRITTEN by this subtask
internal/
  config/{config,file,git,load}.go   # P1.M1.T4 — Load/LoadOpts/Config (read-only ref)
  config/load_test.go           # P1.M1.T4 — TEST PATTERN + helpers to mirror (NOT import)
  config/git_test.go            # setGitConfig helper to copy (package-private)
  generate/generate.go          # P1.M3.T4.S2 — error types for exitcode.For (read-only ref)
  git/git_test.go               # initRepo helper to copy (package-private)
  exitcode/                     # EMPTY — this subtask creates exitcode.go + exitcode_test.go here
  cmd/                          # EMPTY — this subtask creates root.go + root_test.go here
  ui/                           # EMPTY — P1.M4.T3 (untouched)
  {git,generate,provider,prompt,stubtest}/  # untouched by S1
pkg/stagecoach/stagecoach.go      # P1.M3.T5.S1 (parallel) — GenerateCommit contract (read-only ref)
Makefile                        # build/test(-race)/coverage/lint/clean/help + -X main.version  (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/exitcode/exitcode.go      # NEW — package exitcode. ExitError{Code;Err} + New(code,err) +
                                   #        For(err) int (full §15.4 matrix) + Success/Error/
                                   #        NothingToCommit/Rescue/Timeout constants. Package doc.
internal/exitcode/exitcode_test.go # NEW — For() matrix: nil→0, ExitError→.Code, ErrNothingToCommit→2,
                                   #        *RescueError{ErrRescue}→3, *RescueError{ErrTimeout}→124,
                                   #        DeadlineExceeded→124, ErrCASFailed/*CASError→1, generic→1.
internal/cmd/root.go               # NEW — package cmd. rootCmd (SilenceErrors+SilenceUsage), all 11
                                   #        §15.2 persistent flags (bound to vars; config-backed at ZERO
                                   #        default), PersistentPreRunE (config.Load + init/path skip +
                                   #        store), Execute(ctx) error, var Version string, Config()
                                   #        accessor + loadedCfg, stub RunE (cmd.Help → replaced by S2).
internal/cmd/root_test.go          # NEW — package cmd. flag registration/defaults/shorthands; --version
                                   #        output; config loading through cobra (precedence); SilenceErrors.
                                   #        Own copied helpers (initRepo/setGitConfig/writeConfigFile/chdir/loadEnvSetup).
cmd/stagecoach/main.go              # REWRITE — var version string; ctx; cmd.Version=version; cmd.Execute(ctx);
                                   #        os.Exit(exitcode.For(err)); baseline stderr error print.
go.mod / go.sum                    # MODIFY — add github.com/spf13/cobra (+ transitive in go.sum).
# All other files UNCHANGED. internal/{config,generate,git,prompt,provider}, pkg/stagecoach UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 creates internal/exitcode — forward dep on S3, design §0): there is NO exitcode package
// in the tree today, and P1.M4.T3.S3 (its nominal owner) runs AFTER S1. S1's main() MUST call
// exitcode.For(err). So S1 CREATES internal/exitcode (ExitError+New+For+§15.4 constants). When S3 runs
// it must NOT recreate the package (FROZEN); it owns UI-layer error→ExitError wrapping instead.
// For() does the FULL §15.4 mapping so callers don't re-wrap. exitcode imports generate (one-way; no cycle).

// CRITICAL (PRD §15.4 OVERRIDES arch §1.2, design §1): arch/go_ecosystem_patterns.md §1.2 is a GENERIC
// reference whose exit table says 2=usage, 3=config. The PRD is authoritative: 2=nothing-to-commit,
// 3=rescue. Constants = Success(0), Error(1), NothingToCommit(2), Rescue(3), Timeout(124). cobra's own
// arg/flag-parse errors fall through to Error(1) — consistent with PRD §15.4 code 1.

// CRITICAL (--timeout is a STRING flag, design §2): config.Load's loadFlags reads it via
// fs.GetString("timeout"). A pflag.Duration registration makes fs.GetString error → loadFlags silently
// skips --timeout (silent no-op bug). Register PersistentFlags().StringVar(&flagTimeout, "timeout", "", …).
// config.parseTimeout accepts "120s" and bare "120". The 120s default is config.Defaults() Layer 1,
// NOT the pflag default (which is "").

// CRITICAL (zero flag defaults, design §3): register the 6 config-backed flags (provider/model/config/
// timeout/verbose/no-color) with pflag defaults ""/false. config.Load applies Layer 7 via fs.Changed —
// a non-zero pflag default would make Changed() false even when the user MEANT the default, breaking
// the "explicit flag beats lower layers" contract. The PRD's documented defaults (120s, auto-detected,
// TTY-aware) are produced by the precedence layers, not pflag.

// GOTCHA (--version via cobra.Version short-circuits, design §5): rootCmd.Version = <ver> makes cobra
// add --version (NO -v shorthand → no clash with --verbose -v) and print + exit BEFORE
// PersistentPreRunE. So config does NOT load for --version/--help (the task's "skip for help" is
// automatic). The only explicit skip is cmd.Name()=="init"||"path" (S4's config subcommands).

// GOTCHA (cobra PersistentPreRunE shadowing, arch §1.4 gotcha): if a CHILD command defines its own
// PersistentPreRunE, cobra runs ONLY the child's (root's is skipped). So S3/S4 subcommands must NOT
// define PersistentPreRunE — they rely on root's to load config. config init/path are exempt because
// they WANT to skip config load (the root pre-run's cmd.Name() guard handles it).

// GOTCHA (loadGitConfig fails outside a repo): `git -C <nonrepo> config` exits 128 → loadGitConfig
// returns a wrapped error → config.Load fails → exit 1. For S1 the root command thus requires being
// inside a git repo (the default action needs one anyway). S4's config init/path skip is the escape
// hatch for non-repo use. Tests must run inside a temp git repo (chdir).

// GOTCHA (SilenceErrors means main must print, design §7): with SilenceErrors+SilenceUsage true, cobra
// prints NOTHING on error. main() prints a baseline `stagecoach: <err>\n` to stderr so failures aren't
// silent. Guard: if err.Error()=="" (ExitError with nil Err, e.g. a clean non-zero exit) print nothing.
// P1.M4.T3 (UI) refines this (color, verbosity-aware). Do NOT call os.Exit from inside cobra/RunE.

// GOTCHA (rootCmd is a package-level singleton): root_test.go reuses rootCmd across tests via
// SetArgs/SetOut/SetErr + swapping RunE. RESTORE state in t.Cleanup (SetArgs(nil)→reset, restore
// original RunE/Out/Err) or later tests see stale state. -race: config is loaded in PersistentPreRunE
// (before RunE) and read in RunE — sequential within one Execute; tests are sequential → no mutex needed.

// GOTCHA (helpers are package-private): initRepo (internal/git/git_test.go), setGitConfig
// (internal/config/git_test.go), writeConfigFile/chdir/loadEnvSetup/newFlagSet
// (internal/config/load_test.go) are unimportable from package cmd. COPY the ~25-line set into
// root_test.go. initRepo sets GIT_AUTHOR/COMMITTER identity via env. chdir restores CWD in t.Cleanup.

// GOTCHA (version is package main): the Makefile's -X main.version=… injects into package main, so
// `var version string` lives in cmd/stagecoach/main.go (NOT internal/cmd). main sets cmd.Version =
// version before Execute. cobra's default version template prints "stagecoach version <version>".
```

## Implementation Blueprint

### Data models and structure

```go
// internal/exitcode/exitcode.go
package exitcode

import (
	"context"
	"errors"

	"github.com/dustin/stagecoach/internal/generate"
)

// PRD §15.4 exit codes (AUTHORITATIVE — overrides arch/go_ecosystem_patterns.md §1.2's generic table,
// which says 2=usage/3=config; PRD says 2=nothing-to-commit/3=rescue).
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)

// ExitError lets a command force a specific exit code for an error that For()'s domain mapping
// would otherwise default. Return from any RunE: `return exitcode.New(exitcode.Error, err)`.
// errors.As(err, &ee) recovers Code; Unwrap() returns Err (errors.Is chains through).
type ExitError struct {
	Code int    // the exit code to use
	Err  error  // underlying cause; may be nil for a clean non-zero exit
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}
func (e *ExitError) Unwrap() error { return e.Err }

// New wraps err with a forced exit code.
func New(code int, err error) *ExitError { return &ExitError{Code: code, Err: err} }

// For returns the PRD §15.4 exit code for err. Order: nil→0; explicit *ExitError→its Code; then the
// generate-domain mapping (NothingToCommit→2, Rescue→3, Timeout/Deadline→124, CAS→1); else 1.
// A *generate.RescueError whose Kind is ErrTimeout maps to 124 (checked BEFORE the generic rescue→3).
func For(err error) int {
	if err == nil {
		return Success
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return NothingToCommit
	}
	// *RescueError.Unwrap()==Kind; check timeout BEFORE rescue (a timeout IS a rescue with Kind=ErrTimeout).
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if errors.Is(err, generate.ErrCASFailed) {
		return Error // CAS is a general (non-rescue) failure per PRD §13.5/§15.4
	}
	return Error
}
```

```go
// internal/cmd/root.go
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
)

// Version is set by main.go from the ldflags-injected `var version string` before Execute.
// cobra's Version field auto-registers --version (no -v shorthand) and prints+exits BEFORE
// PersistentPreRunE, so config does NOT load for --version.
var Version string

// Config-backed flags (resolved by config.Load via fs.Changed; registered at ZERO default so Changed
// reflects "user passed it"). See design §2 (timeout is a STRING) and §3 (zero defaults).
var (
	flagProvider string
	flagModel    string
	flagConfig   string // --config → LoadOpts.ConfigPathOverride (NOT a Config field)
	flagTimeout  string // STRING — config.Load reads via fs.GetString("timeout") (FINDING 7)
	flagVerbose  bool
	flagNoColor  bool
)

// Behavioral flags (NOT Config fields; read directly by the default-action RunE in S2 / dry-run in S4).
var (
	flagAll         bool
	flagNoAutoStage bool
	flagDryRun      bool
)

// loadedCfg holds the config resolved in PersistentPreRunE; nil until then. Read by Config().
var loadedCfg *config.Config

// rootCmd is the cobra root. SilenceErrors+SilenceUsage → the CLI (main) controls all output.
var rootCmd = &cobra.Command{
	Use:           "stagecoach",
	Short:         "AI-assisted commit message generator",
	SilenceErrors: true,
	SilenceUsage:  true,
	Version:       Version,
	// PersistentPreRunE runs before any RunE (root or subcommand) EXCEPT --help/--version (cobra
	// short-circuits those first). It resolves config once and stores it for RunE access.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if shouldSkipConfigLoad(cmd) {
			return nil
		}
		repoDir, err := os.Getwd()
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
		}
		cfg, err := config.Load(cmd.Context(), config.LoadOpts{
			ConfigPathOverride: flagConfig,
			RepoDir:            repoDir,
			Flags:              cmd.Flags(),
		})
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err))
		}
		loadedCfg = cfg
		return nil
	},
	// STUB: prints help until S2 implements the default commit action
	// (auto-stage-all → stagecoach.GenerateCommit → report). TODO(P1.M4.T1.S2).
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	pf := rootCmd.PersistentFlags()
	// §15.2 config-backed flags (zero defaults; config.Load owns Layer-7 precedence via fs.Changed).
	pf.StringVar(&flagProvider, "provider", "", "Provider/agent to use (env STAGECOACH_PROVIDER, git stagecoach.provider; default auto-detected)")
	pf.StringVar(&flagModel, "model", "", "Model override (env STAGECOACH_MODEL, git stagecoach.model; default per-manifest default_model)")
	pf.StringVar(&flagConfig, "config", "", "Path to a config file, overrides discovery (env STAGECOACH_CONFIG)")
	pf.StringVar(&flagTimeout, "timeout", "", "Generation timeout, e.g. \"120s\" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 120s)")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "Print resolved command, raw output, retries (env STAGECOACH_VERBOSE)")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable color (env STAGECOACH_NO_COLOR, NO_COLOR; default TTY-aware)")
	// §15.2 behavioral flags (read by S2/S4 RunE; not Config fields).
	pf.BoolVarP(&flagAll, "all", "a", false, "Run `git add -A` before snapshotting, even if something is staged")
	pf.BoolVar(&flagNoAutoStage, "no-auto-stage", false, "If nothing is staged, exit instead of auto-staging")
	pf.BoolVar(&flagDryRun, "dry-run", false, "Generate and print the message; do not commit")
	// --version is auto-added by cobra (Version field above); --help/-h is cobra's built-in.
}

// shouldSkipConfigLoad returns true for commands that operate on the config PATH itself, not the
// resolved config — so they work outside a git repo and never need the git-config layer. Matches the
// task's "skip for config init/path/help" (help/version are already short-circuited by cobra).
// Forward-compatible: config init/path arrive in S4; for S1 (root only) this always returns false.
func shouldSkipConfigLoad(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "init" || name == "path"
}

// Config returns the config resolved by PersistentPreRunE, or nil if it was skipped/hasn't run.
// Used by the default action (S2) and subcommands (S3/S4). Safe to call from any RunE.
func Config() *config.Config { return loadedCfg }

// Execute runs the root command with the given context (set on rootCmd so PersistentPreRunE can read
// it via cmd.Context() for config.Load's cancellation seam). Returns the command error (main maps it
// to an exit code via exitcode.For). Does NOT call os.Exit.
func Execute(ctx context.Context) error {
	if ctx != nil {
		rootCmd.SetContext(ctx)
	}
	return rootCmd.Execute()
}
```

```go
// cmd/stagecoach/main.go  (REWRITE the 29-byte stub)
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dustin/stagecoach/internal/cmd"
	"github.com/dustin/stagecoach/internal/exitcode"
)

// version is injected at build time via -ldflags "-X main.version=…" (Makefile VERSION, default "dev").
// P1.M4.T2 will replace context.Background() with a signal-aware context; S1 uses the baseline.
var version = "dev"

func main() {
	cmd.Version = version // cobra's --version prints this (short-circuits before config load)
	err := cmd.Execute(context.Background())
	code := exitcode.For(err)
	if err != nil && err.Error() != "" {
		fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)
	}
	os.Exit(code)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD cobra to go.mod (do this FIRST so imports resolve)
  - RUN: `go get github.com/spf13/cobra@latest` then `go mod tidy`.
  - VERIFY: `grep cobra go.mod` shows `github.com/spf13/cobra vX.Y.Z`; go.sum grows with cobra's
      transitive deps (inconsh,insh, pflag already present). pflag is ALREADY a direct dep (config).
  - GOTCHA: do NOT bump the go directive (stay go 1.22). cobra v1.8.x supports go 1.22.

Task 2: CREATE internal/exitcode/exitcode.go (no dependency on cobra; pure)
  - FILE: NEW internal/exitcode/exitcode.go. PACKAGE: `package exitcode`.
  - DOC: `// Package exitcode maps Stagecoach errors to PRD §15.4 process exit codes (0/1/2/3/124).
      For() is the single source of truth used by the CLI's main(); it covers explicit *ExitError
      overrides, the generate-domain mapping (nothing-to-commit/rescue/timeout/CAS), and a default
      of 1. §15.4 overrides arch/go_ecosystem_patterns.md §1.2's generic table (2=nothing-to-commit,
      not usage; 3=rescue, not config).`
  - DEFINE: the 5 constants (Success/Error/NothingToCommit/Rescue/Timeout), ExitError{Code;Err} with
      Error()/Unwrap(), New(code,err), and For(err) — EXACTLY as in "Data models".
  - GOTCHA: For() checks `errors.Is(err, generate.ErrTimeout)` BEFORE `errors.Is(err, generate.ErrRescue)`
      — a *RescueError with Kind==ErrTimeout must map to 124, not 3. (RescueError.Unwrap()==Kind.)

Task 3: CREATE internal/exitcode/exitcode_test.go (pure unit; no git/cobra)
  - FILE: NEW internal/exitcode/exitcode_test.go. PACKAGE: `package exitcode`.
  - IMPORT: context, errors, fmt, testing, github.com/dustin/stagecoach/internal/generate.
  - CASES (table-driven, one t.Run each):
      * For(nil) == Success(0)
      * For(New(NothingToCommit, errors.New("x"))) == 2  (explicit ExitError wins)
      * For(New(7, errors.New("custom"))) == 7            (caller-chosen code)
      * For(generate.ErrNothingToCommit) == 2
      * For(&generate.RescueError{Kind: generate.ErrRescue}) == 3
      * For(&generate.RescueError{Kind: generate.ErrTimeout}) == 124  (timeout-before-rescue!)
      * For(fmt.Errorf("wrap: %w", generate.ErrTimeout)) == 124       (errors.Is unwraps)
      * For(context.DeadlineExceeded) == 124
      * For(generate.ErrCASFailed) == 1
      * For(&generate.CASError{Expected: "a", Actual: "b"}) == 1
      * For(errors.New("anything else")) == 1
      * For(New(Error, nil)) → ExitError.Error()=="" (clean non-zero exit, no message) — assert code==1
  - COVERAGE: every branch of For(). Verify the timeout-before-rescue ordering explicitly.

Task 4: CREATE internal/cmd/root.go (the cobra scaffold)
  - FILE: NEW internal/cmd/root.go. PACKAGE: `package cmd`. Follow "Data models" skeleton VERBATIM.
  - DEFINE: Version var; the 9 flag vars (6 config-backed + 3 behavioral); loadedCfg; rootCmd with
      SilenceErrors+SilenceUsage+Version+PersistentPreRunE+stub RunE; the init() registering all
      §15.2 flags on PersistentFlags(); shouldSkipConfigLoad; Config(); Execute(ctx).
  - NAMING: rootCmd (unexported, package-level); Execute/Config/Version (exported); flag* vars +
      loadedCfg + shouldSkipConfigLoad (unexported). PLACEMENT: all in internal/cmd/root.go.
  - GOTCHA: --timeout is StringVar (NOT Duration). -a on --all, -v on --verbose. --version via the
      Version FIELD (cobra auto-adds it; do NOT also register a "version" bool flag — double registration
      panics). --help/-h is automatic. PersistentPreRunE skips when cmd.Name()=="init"||"path".
  - GOTCHA: cmd.Flags() passed to config.Load returns the FULL merged flagset (persistent+local) —
      correct for both root and (future) subcommands. flagConfig is read directly (it's not a Config field).

Task 5: CREATE internal/cmd/root_test.go (cobra + git integration)
  - FILE: NEW internal/cmd/root_test.go. PACKAGE: `package cmd`.
  - COPY HELPERS (~25 lines, package-private upstream): initRepo (from internal/git/git_test.go),
      setGitConfig (from internal/config/git_test.go), writeConfigFile + chdir + loadEnvSetup (from
      internal/config/load_test.go). Keep their bodies verbatim (initRepo sets GIT_AUTHOR/COMMITTER env).
  - STATE HYGIENE: rootCmd is a package-level singleton. Each test that mutates it (SetArgs/SetOut/
      SetErr/RunE swap/loadedCfg) MUST restore in t.Cleanup: `rootCmd.SetArgs([]string{})`, restore the
      original io.Writer / RunE, and set loadedCfg=nil. Else tests poison each other (and -race).
  - CASES:
      * TestFlags_RegisteredAndDefaults: assert rootCmd.PersistentFlags().Lookup exists for EVERY §15.2
        flag (provider, model, config, timeout, verbose, no-color, all, no-auto-stage, dry-run,
        version, help). Assert shorthands: all→"a", verbose→"v", help→"h". Assert config-backed flags
        have zero DefValue (""/false). Assert --timeout DefValue is "" (string, not a duration).
      * TestVersion_PrintsAndSkipsConfig: set cmd.Version="test-v"; SetArgs(["--version"]); capture
        stdout via rootCmd.SetOut(&buf); Execute(ctx). Assert buf contains "test-v"; assert loadedCfg==nil
        (config NOT loaded — cobra short-circuited). Assert err==nil (exit 0).
      * TestRoot_LoadsConfigAndRunsStub: loadEnvSetup + chdir(repo); SetArgs([]) (no args); swap RunE
        to a capture fn that reads Config() and returns nil; Execute(ctx). Assert Config()!=nil,
        Config().Provider=="" (Defaults), Timeout==120s. (Restores RunE in cleanup.)
      * TestRoot_FlagOverridesEnvOverridesGit: loadEnvSetup + chdir(repo); setGitConfig(repo,
        "stagecoach.provider","git-p"); t.Setenv("STAGECOACH_PROVIDER","env-p"); SetArgs(["--provider",
        "cli-p"]); capture RunE; Execute. Assert Config().Provider=="cli-p" (CLI > env > git). Then a
        second sub-case with no --provider → Config().Provider=="env-p" (env > git).
      * TestRoot_ConfigLoadErrorMapsToExit1: writeConfigFile(globalDir,"config.toml","bad {toml");
        chdir(repo); SetArgs([]); Execute. Assert err != nil AND exitcode.For(err)==1 (config load
        failure). (This proves PersistentPreRunE propagates the load error as exitcode.Error.)
      * TestRoot_SilenceErrors: SetArgs(["--bogus-flag"]); Execute → assert err != nil (unknown flag)
        AND rootCmd's Out/Err captured nothing from cobra (SilenceErrors+SilenceUsage) — main is
        responsible for printing (tested at the exitcode level, not here).
  - COVERAGE: flag surface, --version short-circuit, config load + store, full precedence through the
        CLI, error→exit-code propagation, SilenceErrors. All behavioral flags' PRESENCE asserted (their
        RunE logic is S2/S4 — not tested here).

Task 6: REWRITE cmd/stagecoach/main.go
  - FILE: OVERWRITE cmd/stagecoach/main.go (currently the 29-byte stub). Follow "Data models" skeleton.
  - BODY: `var version = "dev"` (default literal; -X overrides); main() sets cmd.Version=version;
      err := cmd.Execute(context.Background()); code := exitcode.For(err); print baseline error to
      stderr if err != nil && err.Error() != ""; os.Exit(code).
  - GOTCHA: `var version string` (not `= "dev"`)? Use `var version = "dev"` so `stagecoach` run WITHOUT
      ldflags (e.g. `go run`) prints "dev" instead of empty. -X replaces it on `make build`.
  - GOTCHA: do NOT import cobra in main (main only touches cmd + exitcode). context.Background() is the
      S1 baseline; P1.M4.T2 swaps it.

Task 7: VALIDATE (run all gates; fix before declaring done)
  - `make build` → ./bin/stagecoach exists; `./bin/stagecoach --version` prints a version string.
  - `./bin/stagecoach --help` → lists ALL §15.2 flags with the §15.2/FR35 descriptions.
  - `go test -race ./internal/exitcode/ ./internal/cmd/` → green.
  - `go test -race ./...` → green (NO regression in config/generate/git/provider/prompt).
  - `go vet ./...` clean; `gofmt -l internal/ cmd/` empty.
  - Confirm `git status` shows ONLY: new internal/exitcode/{exitcode.go,exitcode_test.go}, new
    internal/cmd/{root.go,root_test.go}, modified cmd/stagecoach/main.go, modified go.mod/go.sum.
```

### Implementation Patterns & Key Details

```go
// PATTERN: PersistentPreRunE — resolve config ONCE, store it, skip for path-oriented commands.
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    if shouldSkipConfigLoad(cmd) { return nil }      // config init/path (S4); help/version auto-skipped
    repoDir, err := os.Getwd()
    if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("getwd: %w", err)) }
    cfg, err := config.Load(cmd.Context(), config.LoadOpts{
        ConfigPathOverride: flagConfig,                // --config > STAGECOACH_CONFIG > discovery (inside Load)
        RepoDir:            repoDir,                   // git -C <cwd> config walks up to repo root
        Flags:              cmd.Flags(),               // Layer 7: only fs.Changed flags apply
    })
    if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err)) }
    loadedCfg = cfg
    return nil
}

// PATTERN: main() exit-code wiring — the ONLY place os.Exit is called.
func main() {
    cmd.Version = version
    err := cmd.Execute(context.Background())
    code := exitcode.For(err)            // centralized §15.4 mapping
    if err != nil && err.Error() != "" {
        fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)  // baseline; P1.M4.T3 refines (color/verbose)
    }
    os.Exit(code)
}

// GOTCHA: For()'s ordering matters — timeout-before-rescue.
//   A *generate.RescueError{Kind: generate.ErrTimeout} unwraps to ErrTimeout, so it satisfies BOTH
//   errors.Is(ErrTimeout) and errors.Is(ErrRescue)? NO — Unwrap()==ErrTimeout only, so errors.Is(ErrRescue)
//   is FALSE for it. But check ErrTimeout FIRST anyway for clarity and to guard future Unwrap chains.

// GOTCHA: double-registering --version panics. cobra's Version field adds the flag automatically in
// Execute(); do NOT call PersistentFlags().BoolVar(..., "version", ...). Only register the 9 explicit
// flags in init(). --help/-h is likewise automatic.

// GOTCHA: testing a package-level singleton (rootCmd). Restore in t.Cleanup or tests leak state:
//   origOut, origErr, origRunE := rootCmd.OutOrStdout(), rootCmd.ErrOrStderr(), rootCmd.RunE
//   rootCmd.SetOut(&buf); rootCmd.SetArgs(...); rootCmd.RunE = capture
//   t.Cleanup(func(){ rootCmd.SetOut(origOut); rootCmd.SetArgs(nil); rootCmd.RunE = origRunE; loadedCfg=nil })
```

### Integration Points

```yaml
GO.MODULE:
  - add: "github.com/spf13/cobra vX.Y.Z" to go.mod require (via `go get`/`go mod tidy`); pflag already present
  - gotcha: "stay go 1.22; cobra v1.8.x is go-1.22-compatible"

COMMAND.TREE (forward — S2/S3/S4 hang here):
  - rootCmd.RunE: "STUB now (cmd.Help); S2 sets it to the default commit action"
  - rootCmd.AddCommand: "S3 adds providers{list,show}; S4 adds config{init,path}"
  - gotcha: "S3/S4 subcommands MUST NOT define their own PersistentPreRunE (cobra runs only the
             child's, skipping root's config load); config init/path rely on root's cmd.Name() guard"

CONFIG.STORE:
  - accessor: "cmd.Config() *config.Config — S2/S3/S4 read the resolved config from any RunE"
  - gotcha: "nil if PersistentPreRunE was skipped (config init/path) or hasn't run"

FLAG.VARS (forward — S2/S4 read these):
  - flagAll, flagNoAutoStage: "S2 default action (--all → git add -A; --no-auto-stage → exit 2)"
  - flagDryRun: "P1.M4.T4 dry-run RunE reads this; S1 only registers it"

CONTEXT (forward — P1.M4.T2):
  - main: "context.Background() now; P1.M4.T2 swaps for signal.NotifyContext + child-kill/rescue"
  - gotcha: "Execute(ctx) already sets ctx on rootCmd; PersistentPreRunE reads cmd.Context()"

MAKEFILE:
  - "unchanged; -X main.version=$(VERSION) now effective (main.go declares var version)"
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation - fix before proceeding
go build ./internal/exitcode/ ./internal/cmd/ ./cmd/stagecoach/
gofmt -w internal/exitcode/ internal/cmd/ cmd/stagecoach/
go vet ./internal/exitcode/ ./internal/cmd/ ./cmd/stagecoach/

# Expected: zero errors. gofmt rewrites formatting; govet reports none.
# GOTCHA: `go build ./cmd/stagecoach/` will FAIL with "version flag registered" panic at TEST time only
#         if you double-register --version — `go build` itself won't catch it; the --version test will.
```

### Level 2: Unit Tests (Component Validation)

```bash
# exitcode — pure unit, no git/cobra
go test -race ./internal/exitcode/ -v

# cmd — cobra + git (needs a temp repo; tests set up their own)
go test -race ./internal/cmd/ -v

# Expected: all green. If root_test.go flakes, check rootCmd state restoration in t.Cleanup.
```

### Level 3: Integration Testing (Binary Validation)

```bash
# Build the real binary
make build

# --version prints the injected version (VERSION=dev by default)
./bin/stagecoach --version
# Expected: a line containing "dev" (cobra default template: "stagecoach version dev")

# --help lists ALL §15.2 global flags (Mode A docs surface)
./bin/stagecoach --help
# Expected: --provider, --model, --config, --timeout, --all (-a), --no-auto-stage, --dry-run,
#           --verbose (-v), --no-color, --version, --help (-h) all present with §15.2 descriptions.

# Inside a git repo: stub RunE prints help (config loads first; no error)
cd /tmp/emptyrepo  # any git repo; create with `git init`
/home/dustin/projects/stagecoach/bin/stagecoach
# Expected: help text printed, exit 0.

# Outside a git repo: config.Load fails (git-config layer exits 128) → exit 1
cd /tmp && /home/dustin/projects/stagecoach/bin/stagecoach
echo $?   # Expected: 1

# Bad config file → exit 1 with a clear message
echo 'bad {toml' > /tmp/bad.toml && /home/dustin/projects/stagecoach/bin/stagecoach --config /tmp/bad.toml
# Expected: "stagecoach: config: global config: ..." on stderr, exit 1.

# Verify the version injection path the Makefile already wires
make build VERSION=v9.9.9 && ./bin/stagecoach --version
# Expected: contains "v9.9.9" (proves var version is the -X target main.go declared).
```

### Level 4: Domain-Specific Validation (Exit-Code Contract)

```bash
# Programmatic exit-code mapping sanity (no real agent needed for S1 — exitcode.For is unit-tested).
# The §15.4 contract is verified by exitcode_test.go's table; this is the human-readable confirmation:
go test -race ./internal/exitcode/ -run TestFor -v
# Expected: 2 (nothing-to-commit), 3 (rescue), 124 (timeout), 1 (CAS/general), 0 (success) all pass.

# Confirm no regression across the WHOLE module (config/generate/git/provider/prompt untouched)
go test -race ./...
# Expected: all green. If a config/generate test broke, S1 over-reached — recheck that internal/*
# files were NOT edited (only internal/exitcode + internal/cmd are new; cmd/stagecoach rewritten).
```

## Final Validation Checklist

### Technical Validation

- [ ] `make build` succeeds → `./bin/stagecoach` exists.
- [ ] `./bin/stagecoach --version` prints a version string; `make build VERSION=vX` injects it.
- [ ] `./bin/stagecoach --help` lists ALL eleven §15.2 flags with §15.2/FR35 descriptions.
- [ ] `go test -race ./...` green (exitcode + cmd new; NO regression elsewhere).
- [ ] `go vet ./...` clean; `gofmt -l internal/ cmd/` empty.
- [ ] `git status` shows ONLY: 4 new files (exitcode ×2, cmd ×2), rewritten main.go, go.mod/go.sum.

### Feature Validation

- [ ] All §15.2 flags registered (names + shorthands: `-a`, `-v`, `-h`) on root PersistentFlags.
- [ ] `--timeout` is a String flag (DefValue "" — verified in TestFlags_RegisteredAndDefaults).
- [ ] Config-backed flags have zero pflag defaults (fs.Changed contract intact).
- [ ] `PersistentPreRunE` loads config via `config.Load` and stores it; `Config()` returns it.
- [ ] `PersistentPreRunE` skips for `cmd.Name()=="init"||"path"` (forward-compatible; tested by intent).
- [ ] `--version`/`--help` do NOT load config (cobra short-circuit; TestVersion_PrintsAndSkipsConfig).
- [ ] `exitcode.For` maps every §15.4 code (exitcode_test.go table green).
- [ ] main() calls `os.Exit(exitcode.For(err))` exactly once; prints baseline error on non-empty err.
- [ ] Full precedence works through the CLI (TestRoot_FlagOverridesEnvOverridesGit).

### Code Quality Validation

- [ ] Follows cobra idioms from arch/go_ecosystem_patterns.md §1 (PersistentFlags, Execute returns err).
- [ ] Exit codes follow PRD §15.4 (NOT the arch doc's generic §1.2 table).
- [ ] File placement matches the desired tree (internal/exitcode/, internal/cmd/).
- [ ] No os.Exit/Print inside cobra RunE or internal/exitcode (only main exits/prints).
- [ ] No edits to internal/{config,generate,git,prompt,provider} or pkg/stagecoach (read-only contracts).
- [ ] cobra is the only new dependency; go directive stays 1.22.

### Documentation & Deployment

- [ ] Help text (auto-generated) matches PRD §15.2/FR35 (Mode A docs ride with this subtask).
- [ ] Every exported symbol (ExitError, New, For, Execute, Config, Version, the 5 constants) has a
      Go-doc comment.
- [ ] No new env vars beyond the documented STAGECOACH_* set (config.Load already handles them).

---

## Anti-Patterns to Avoid

- ❌ Don't copy architecture/go_ecosystem_patterns.md §1.2's exit-code table (2=usage, 3=config) —
  PRD §15.4 is authoritative (2=nothing-to-commit, 3=rescue).
- ❌ Don't register `--timeout` as pflag.Duration — config.Load reads it via fs.GetString; a Duration
  flag silently no-ops `--timeout`.
- ❌ Don't give config-backed flags non-zero pflag defaults — config.Load's `fs.Changed` precedence
  relies on zero defaults meaning "not user-set".
- ❌ Don't register a separate `--version` bool flag — cobra's `Version` field already adds it;
  double-registration panics at Execute time.
- ❌ Don't define `PersistentPreRunE` on S3/S4 subcommands (forward note) — it shadows root's and
  breaks the one-config-load-path invariant.
- ❌ Don't call `os.Exit` or print inside cobra RunE / internal/exitcode — only `main` exits/prints.
- ❌ Don't create the default action / subcommands / signal handler / verbose output / dry-run RunE —
  those are S2/S3/S4/P1.M4.T2/T3/T4. S1 is the SCAFFOLD only.
- ❌ Don't edit any file under internal/{config,generate,git,prompt,provider} or pkg/stagecoach — they
  are READ-ONLY upstream contracts (P1.M3.T5.S1 is running in parallel on pkg/stagecoach).
- ❌ Don't forget to restore rootCmd state in root_test.go's t.Cleanup — it's a package-level singleton
  reused across tests; leaking SetArgs/SetOut/RunE poisons siblings (and trips -race).
