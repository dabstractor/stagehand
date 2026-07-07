# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`. cobra-based root command, all §15.2 global flags, PersistentPreRunE
> config loading, and custom exit-code wiring. The default action body + subcommands land in
> S2/S3/S4; signal handling in P1.M4.T2; UI/verbose in P1.M4.T3.

## 0. The forward dependency on P1.M4.T3.S3 (the `internal/exitcode` package) — RESOLUTION

The task INPUT lists "ExitError type from P1.M4.T3.S3." But `plan_status` shows P1.M4.T3.S3
("Exit-code constants and ExitError type") is **Planned / not built**, while S1 (this item) runs
**first** (T1 before T3) and its main() **must** call `exitcode.For(err)` to wire exit codes.
There is no `internal/exitcode` package in the tree today (`ls internal/exitcode` → ENOENT).

**Resolution:** **S1 CREATES `internal/exitcode`** (ExitError type + New + For + the PRD §15.4
constants). It is a hard, load-bearing dependency of main() and is fully specified by
`architecture/go_ecosystem_patterns.md` §1.2 + PRD §15.4. P1.M4.T3.S3 should be **re-scoped** when
it runs: it will NOT recreate the package (FROZEN after S1) but instead own the **UI-layer error→
ExitError wrapping at the RunE boundary** (e.g. the default action mapping `generate.ErrNothingToCommit`
→ `exitcode.New(exitcode.NothingToCommit, err)`). This is documented as a SCOPE NOTE in the PRP; it
does NOT modify tasks.json (research agent is read-only on the plan).

`exitcode.For(err)` does the FULL PRD §15.4 mapping so any error flowing up from anywhere (config
load, generate, a command) lands on the right code without each caller re-wrapping:

| error shape (errors.Is / errors.As)                       | code | constant            |
|-----------------------------------------------------------|------|---------------------|
| `nil`                                                     | 0    | Success             |
| `*exitcode.ExitError` (explicit `.Code`)                  | .Code| (caller-chosen)      |
| `errors.Is(err, generate.ErrNothingToCommit)`             | 2    | NothingToCommit     |
| `*generate.RescueError` && `errors.Is(err, generate.ErrRescue)`  | 3 | Rescue              |
| `*generate.RescueError` && `errors.Is(err, generate.ErrTimeout)` | 124 | Timeout            |
| `errors.Is(err, context.DeadlineExceeded)`                | 124  | Timeout             |
| `errors.Is(err, generate.ErrCASFailed)` (`*CASError`)     | 1    | GeneralError        |
| anything else                                             | 1    | GeneralError        |

`exitcode` imports `generate` (one-way; `generate` does NOT import `exitcode` → no cycle) and
`context` + `errors`. This centralizes the mapping DRY-ly while keeping the `*ExitError` escape
hatch for explicit overrides.

## 1. PRD §15.4 OVERRIDES architecture/go_ecosystem_patterns.md §1.2's exit-code table

The architecture doc is a GENERIC pattern reference. Its §1.2 table says `2 = usage error`,
`3 = config error`. **The PRD (authoritative product spec) says `2 = nothing to commit`,
`3 = rescue`.** This PRP follows the PRD. Constants are therefore:
`Success=0, GeneralError=1, NothingToCommit=2, Rescue=3, Timeout=124`. Usage/arg/flag errors from
cobra fall through to GeneralError (1) — which is consistent with PRD §15.4 code 1 ("parse failed …").
Do NOT copy the doc's 2/3 meanings.

## 2. `--timeout` MUST be a STRING flag, not `pflag.Duration`

`internal/config/load.go` `loadFlags()` reads it via `fs.GetString("timeout")` (coordination note
"FINDING 7" in load.go). If S1 registers `--timeout` as a Duration flag, `fs.GetString("timeout")`
errors and `loadFlags` **silently skips it** → `--timeout` would be a no-op. So register
`PersistentFlags().StringVar(&flagTimeout, "timeout", "", ...)`. `config.parseTimeout` accepts both
`"120s"` and bare `"120"` (seconds). The effective default (120s) comes from `config.Defaults()`
Layer 1 — NOT from the flag default (which is `""`).

## 3. Config-backed flags use ZERO defaults; `config.Load` owns precedence (Layer 7 = fs.Changed)

The 5 config-backed flags (`--provider`, `--model`, `--config`, `--timeout`, `--verbose`,
`--no-color`) are registered with zero defaults (`""`/`false`). The CLI passes `cmd.Flags()` to
`config.Load(LoadOpts{Flags: …})`; `loadFlags` overlays ONLY flags where `fs.Changed(name)` is true
(explicit user intent). So pflag defaults must be zero so `Changed` correctly means "user passed it".
The PRD's documented defaults (120s, auto-detected, TTY-aware) are produced by the precedence layers
(Layer 1 defaults / auto-detect in S2 / NoColor TTY-aware in P1.M4.T3.S1), NOT by the flag default.
Help-text descriptions must match PRD §15.2/FR35 (Mode A docs requirement).

## 4. Behavioral flags are package vars read by S2/S4; not config fields

`--all/-a`, `--no-auto-stage`, `--dry-run` are NOT `Config` fields (confirmed: `Config` has no such
fields). They are bound to package-level `internal/cmd` vars (`flagAll`, `flagNoAutoStage`,
`flagDryRun`) and read directly by the default-action RunE (S2) and dry-run RunE (S4). S1 registers
them (so help text is complete) but does NOT implement their logic. `--version` is handled by
cobra's `Version` field (see §5). `--help/-h` is cobra's built-in.

## 5. `--version` via cobra's `Version` field (short-circuits before PersistentPreRunE)

`rootCmd.Version = <version>` makes cobra auto-add `--version` (NO `-v` shorthand → no clash with
`--verbose -v`). Cobra checks `--version`/`--help` in `Command.execute()` BEFORE running
`PersistentPreRunE`, so **config is NOT loaded** for `stagecoach --version` / `--help` (this is why
the task's "skip for … help" is automatic and needs no code). The version STRING comes from the
Makefile's `-X main.version=$(VERSION)` (package `main`), so `main.go` sets `cmd.Version = version`
before `cmd.Execute(ctx)`. The `var version string` declaration belongs to S1 (the Makefile comment
says "a later subtask adds it" — that's S1, the owner of main.go).

## 6. `PersistentPreRunE` config-load skip = `cmd.Name()=="init"||cmd.Name()=="path"`

The CLI's `PersistentPreRunE` calls `config.Load(ctx, LoadOpts{ConfigPathOverride: flagConfig,
RepoDir: os.Getwd(), Flags: cmd.Flags()})` and stores the result. It must SKIP for `config init`
and `config path` (S4) — those operate on the config PATH itself, not the resolved config, and must
work outside a git repo. `--help`/`--version` are auto-skipped by cobra (§5). Skip check:
`if cmd.Name()=="init"||cmd.Name()=="path" { return nil }`. These commands don't exist in S1, so the
guard never matches now (root always loads) — but it is forward-correct for S4. **S3/S4 subcommands
must NOT define their own `PersistentPreRunE`** (cobra runs only the child's, shadowing root's).
Documented for downstream.

RepoDir = `os.Getwd()` (git `-C <cwd> config` walks up to the repo root — same choice pkg/stagecoach
made). NOTE: `loadGitConfig` FAILS (exit 128 → wrapped error) outside a git repo, so for S1 the
root command requires being inside a repo — acceptable (the default action needs one anyway; S4's
config init/path skip is exactly the escape hatch for non-repo use).

## 7. `SilenceErrors`+`SilenceUsage` on root; main() prints a baseline error line

Root sets both to true so cobra does NOT print errors/usage (the CLI controls output). For S1,
`main()` prints a minimal `stagecoach: <err>\n` to stderr on non-zero exit so failures aren't silent.
This baseline is REFINED by P1.M4.T3 (UI layer: color, verbosity-aware formatting). `ExitError` with
`Err==nil` (clean non-zero exit, e.g. nothing-to-commit surfacing) must NOT print an "error" line —
guard `if err != nil && err.Error() != ""`.

## 8. main.go context = `context.Background()` for S1 (signal handling is P1.M4.T2)

`main()` builds `ctx := context.Background()` and passes it to `cmd.Execute(ctx)`; the root's
`PersistentPreRunE` reads it via `cmd.Context()` for `config.Load(ctx, …)`. P1.M4.T2 replaces this
with `signal.NotifyContext` + child-kill/rescue. S1 does NOT install any signal handler (out of
scope). The `Execute(ctx context.Context)` signature is forward-compatible with that swap.

## 9. The root RunE is a STUB replaced by S2

S1 gives root a minimal RunE so `PersistentPreRunE` fires (config loads → testable). The stub:
`return cmd.Help()` (prints usage, exit 0), clearly commented `// TODO(P1.M4.T1.S2): default commit
action`. S2 swaps `rootCmd.RunE` for the real action (auto-stage-all → `stagecoach.GenerateCommit` →
print result). No default-action logic in S1.

## 10. cobra + pflag to go.mod (pflag already present)

`go.mod` has `github.com/spf13/pflag v1.0.10` already (config uses it). S1 ADDS
`github.com/spf13/cobra` (v1.8.x) via `go get github.com/spf13/cobra@latest && go mod tidy`. cobra
transitively depends on pflag + other small pkgs (go.sum grows). This is the ONLY dependency change.
`go build ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l` must all be clean.

## 11. Testing strategy (mirrors `internal/config/load_test.go` conventions)

cmd/exitcode tests need git-repo + CWD isolation because the cobra command shells out to git for
Layer 4. Helpers `initRepo`/`setGitConfig`/`writeConfigFile`/`chdir` are package-private in
`internal/{git,config}` → **copy** the ~25-line set into `internal/cmd/root_test.go` (same approach
P1.M3.T5.S1 documented). Isolate `HOME`+`XDG_CONFIG_HOME` to a temp dir (`loadEnvSetup` pattern).
Redirect `config.noticeOut` is NOT needed from package cmd (it's package-private to config; the
notice goes to os.Stderr which we capture via a buffer on root's OutOrStderr).

Test shape:
- **exitcode_test.go** (pure unit): feed each error shape → assert code. No git/cobra needed.
- **root_test.go** (cobra + git): `rootCmd.SetArgs(...)`; swap `rootCmd.RunE` to capture `Config()`;
  capture stdout/stderr via `rootCmd.SetOut/SetErr(buf)`; assert flags parsed, config precedence
  through the CLI, `--version` output, SilenceErrors (cobra didn't print). Run inside a temp repo
  (`chdir`). Restore rootCmd state in `t.Cleanup` (SetArgs(""), reset RunE) because rootCmd is a
  package-level singleton reused across tests.

## Sources

- `architecture/go_ecosystem_patterns.md` §1 (cobra setup, persistent flags, PersistentPreRunE,
  custom exit codes), §1.2 (ExitError/New/For shape), Appendix A (main.go + context wiring),
  Appendix B (go.mod deps). Exit-code TABLE overridden by PRD §15.4 (decision §1).
- `internal/config/load.go` — `Load`/`LoadOpts` signature, `loadFlags` reads `fs.Changed`+`fs.GetString`
  (FINDING 7: timeout as string). READ-ONLY contract.
- `internal/config/config.go` — `Config` fields (no DryRun/All/Version fields → those are flag-only).
- `internal/config/file.go` — path resolution is internal to Load (`globalConfigPath`/`repoLocalConfigPath`
  are unexported; CLI just passes `ConfigPathOverride`). READ-ONLY.
- `internal/generate/generate.go` — `ErrNothingToCommit`/`ErrTimeout`/`ErrRescue`/`ErrCASFailed`,
  `*RescueError{Kind,Unwrap→Kind}`, `*CASError{Unwrap→git.ErrCASFailed}`. READ-ONLY; drives exitcode.For.
- `internal/config/load_test.go` — test conventions to MIRROR (loadEnvSetup/chdir/newFlagSet/writeConfigFile).
- `pkg/stagecoach/stagecoach.go` (P1.M3.T5.S1, parallel/Implementing) — the public `GenerateCommit`
  contract the default action (S2) will call; S1 does NOT touch it but the stub is S1's seam toward it.
- `Makefile` — already wires `-X main.version=$(VERSION)`; needs `var version string` in main (S1 adds it).
- PRD §15.2 (global flags), §15.4 (exit codes), §16.1/§16.3 (precedence), §21.1 (build/ldflags).
