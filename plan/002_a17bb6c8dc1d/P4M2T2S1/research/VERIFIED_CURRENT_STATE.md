# P4.M2.T2.S1 — Verified Current State of the Codebase

> Captured 2026-07-01 via runtime execution of a freshly-built `./cmd/stagecoach` binary
> (`go build -o /tmp/stagecoach-verify ./cmd/stagecoach`) and source inspection. This is the
> EVIDENCE base for the PRP. All three contract items are ALREADY satisfied — the task's
> deliverable value is VERIFICATION + REGRESSION-TEST HARDENING (locking the behavior in).

## Contract item 1 — `config upgrade` registration → ✅ ALREADY DONE

**Source** (`internal/cmd/config.go`, `init()`):
```go
func init() {
    configInitCmd.Flags().String("provider", "", "...")
    configInitCmd.Flags().Bool("force", false, "...")
    configInitCmd.Flags().Bool("template", false, "...")
    configCmd.AddCommand(configInitCmd)
    configCmd.AddCommand(configPathCmd)
    configCmd.AddCommand(configUpgradeCmd)   // ← registration PRESENT
    rootCmd.AddCommand(configCmd)
}
```

**Runtime evidence** — `stagecoach config --help` "Available Commands" section:
```
Available Commands:
  init        Bootstrap a working config (auto-detects your agent)
  path        Print the resolved global config path
  upgrade     Upgrade an existing config to the current schema version
```
`stagecoach config upgrade --help` prints its Long help and exits 0 (command is reachable + executable).

**Existing test coverage**: `TestConfigUpgrade_*` (8 tests in config_test.go) execute the command via
`rootCmd.SetArgs([]string{"config", "upgrade"})` and it runs — which *implicitly* proves registration.
But NO test explicitly asserts "upgrade" appears in `config`'s Available Commands list.

## Contract item 2 — FR-B6 help de-duplication (parent commands) → ✅ ALREADY DONE

**`grep -rn "Subcommands" internal/cmd/` → (none found)**. No manual "Subcommands:" block exists anywhere.

`configCmd.Long`:
```go
Long: `Inspect, bootstrap, or upgrade the Stagecoach global config file.`,
```
`providersCmd.Long`:
```go
Long: `Inspect the built-in and user-defined provider manifests Stagecoach uses to generate commits.
User-defined providers (from the global or repo-local config file) override built-ins of the same
name; new names add new providers (PRD §12.8).`,
```
Both contain ONLY prose — cobra's auto-generated "Available Commands" is the single source. Runtime
confirms each leaf appears EXACTLY ONCE (init/path/upgrade for config; list/show for providers).

**GAP (the deliverable)**: The existing `TestConfigGroup_NoSubcommandPrintsHelp` (config_test.go:554) and
`TestProvidersGroup_NoSubcommandPrintsHelp` (providers_test.go:344) assert the help *contains* the leaf
names, but do NOT assert:
  - the manual "Subcommands:" block is ABSENT (the FR-B6 negative check), and
  - cobra's "Available Commands:" is the single source (the FR-B6 positive check).
Nothing locks FR-B6 in — a future edit that re-adds a manual block would pass all current tests.

## Contract item 3 — `config init` populated flag wiring → ✅ ALREADY DONE

**Source** (`internal/cmd/config.go`, `init()`): the three flags are registered on `configInitCmd`
(see item 1 source). They are LOCAL flags on the leaf command (NOT root persistent flags) — correct,
because `config init` is in `shouldSkipConfigLoad` (root.go) and never runs `config.Load`, so it reads
them via `cmd.Flags().GetBool/GetString` in `runConfigInit` (NOT via the root flag set).

**Runtime evidence** — `stagecoach config init --help` "Flags:" section:
```
Flags:
      --force             Overwrite an existing config file
  -h, --help              help for init
      --provider string   Target a specific provider instead of auto-detecting
      --template          Write the inert all-commented reference config (v1 behavior)
```
All three flags (`--provider`, `--force`, `--template`) are present and functional.

**NOTE on contract wording**: the contract says "needs its flags wired to root.go's flag set so
config.Load can read them (or they're local to configInitCmd)". The implemented design chose the SECOND
option — they are LOCAL to configInitCmd (correct: config init must NOT go through config.Load, which
requires a git repo for the git-config layer; the leaves work outside a repo via shouldSkipConfigLoad).
This is the right call; do NOT move them to root's persistent flags.

## Full lifecycle — init → use → upgrade → ✅ WORKS

Run in an isolated `HOME=/tmp/sh-lifecycle`:
1. `config path` → `…/config.toml` ✓
2. `config init` → "Wrote config to …" (populated, NOT inert) ✓
3. `config init` again → "config file already exists … (not overwritten)" exit 1 ✓
4. `config init --force` → overwrites, exit 0 ✓
5. written config is POPULATED (`[defaults] provider=…`, per-role models uncommented) ✓
6. written config contains `config_version = 2` (bootstrap.go:117 `fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)`) ✓
7. `config upgrade` on fresh-init'd config → "already at version 2 (no changes)" exit 0 ✓
8. rewrite `config_version = 1` → `config upgrade` → "Upgraded … to version 2" exit 0 ✓
9. `config upgrade` with no file → "no config file … (run 'stagecoach config init' first)" exit 1 ✓

`CurrentConfigVersion = 2` (config.go:18). `GenerateBootstrapConfig` (bootstrap.go:20) writes the
populated config INCLUDING the uncommented `config_version` line.

## Discovered observation (OUT of strict FR-B6 scope — do NOT change in this task)

`configInitCmd.Long` embeds a prose "Flags:" block listing `--provider`/`--force`/`--template`, AND cobra
auto-generates a "Flags:" section — a minor redundancy visible in `config init --help`. This is a LEAF
command, NOT a parent, so FR-B6 (which targets the `config`/`providers` PARENT "Subcommands:" blocks)
does NOT cover it. No existing test asserts on this prose block, so removing it would be safe — but it
is explicitly OUT OF SCOPE for this task (scope discipline). Do not touch it here.

## Test-helper signatures (for the regression tests)

From `internal/cmd/config_test.go` + `root_test.go`:
- `func setupNoRepo(t *testing.T) (home, plainDir, globalDir string)` — sets `HOME`+`XDG_CONFIG_HOME`
  to a temp `home`, chdir into `plainDir`; returns `globalDir = filepath.Join(home, "stagecoach")`.
  (XDG=home → global config path = `home/stagecoach/config.toml`.)
- `func writeConfigFile(t *testing.T, dir, relPath, body string) string` (root_test.go:61).
- `func saveRootState(t *testing.T) (_ []string, origOut, origErr io.Writer, origRunE func(*cobra.Command, []string) error)` (root_test.go:105).
- `func restoreRootState(t *testing.T, _ []string, origOut, origErr io.Writer, origRunE func(*cobra.Command, []string) error)` (root_test.go:111).

Standard test scaffold (used by ALL existing config/providers tests):
```go
_, origOut, origErr, origRunE := saveRootState(t)
defer restoreRootState(t, nil, origOut, origErr, origRunE)
setupNoRepo(t)               // or setupRepo(t) — temp HOME/XDG + chdir
var buf bytes.Buffer
rootCmd.SetOut(&buf)
rootCmd.SetErr(io.Discard)
rootCmd.SetArgs([]string{"config"})   // the args under test
err := Execute(context.Background())
// ... assertions on buf.String(), err, exitcode.For(err) ...
```
