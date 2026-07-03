// Package main implements the stagehand CLI entrypoint.
//
// This is the root of the build DAG. It wires a cobra root command with the
// module version injected at link time via -ldflags "-X main.version=<VERSION>".
//
// The root command carries the default-action Run (maybeAutoStage +
// GenerateCommit) plus the ten PRD §15.2 global flags as persistent flags. The
// bare invocation `stagehand` runs the default action; `stagehand --version`
// short-circuits via cobra's Version field BEFORE Run; `stagehand providers` /
// `stagehand config` dispatch to the subcommand trees that self-register via
// their own package init() in providers.go / config.go.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/ui"
)

// version is the build version. It is overridden at link time with
// -ldflags "-X main.version=$(VERSION)". The "dev" default is only used when
// building without ldflags injection (e.g. `go run`, `go build` without flags).
var version = "dev"

// rootCmd is the stagehand root command. Setting Version (rather than
// registering a manual --version flag) lets cobra auto-add a --version flag
// that prints "stagehand version <version>" to stdout and exits 0 — and,
// crucially, cobra handles --version BEFORE invoking Run, so the short-circuit
// works even once Run is wired. A manual StringVar --version flag would require
// an argument and break bare `stagehand --version`, so the built-in field is
// used. Run (and the persistent flags) are attached in init() below so the
// wiring is explicit and reachable from tests.
var rootCmd = &cobra.Command{
	Use:     "stagehand",
	Short:   "Conventional, AI-friendly Git commits staged from natural language",
	Long:    rootLong,
	Version: version,
	Run: func(cmd *cobra.Command, _ []string) {
		// runDefault returns the PRD §15.4 exit code; the single os.Exit lives
		// here (NOT in runDefault) so the helpers stay os.Exit-free and
		// unit-testable, mirroring generate.handleSignal(...) int. Run (not
		// RunE) is used because cobra's RunE maps every error to exit 1, which
		// cannot carry the §15.4 codes 2/3/124.
		os.Exit(runDefault(cmd))
	},
}

// rootLong is the multi-line synopsis printed by `stagehand --help` and the
// bare-without-version path (PRD §15.1). It summarizes the default action and
// the global flags; the canonical per-flag/env reference lives in
// docs/CONFIGURATION.md (PRD §15.2).
const rootLong = `stagehand — conventional, AI-friendly Git commits from staged changes.

With no subcommand, stagehand runs the default action: it resolves the provider
+ config (flag > env > git-config > file > defaults), optionally auto-stages
all changes when nothing is staged, asks the resolved agent for a conventional
commit message, and creates the commit.

Usage:
  stagehand [flags]              Default action: generate + commit.
  stagehand <command> [flags]    providers | config subcommands.

Global flags (flag > env > git-config > file > default):
      --provider <name>   Provider/agent to use   (STAGEHAND_PROVIDER)
      --model <name>      Model override          (STAGEHAND_MODEL)
      --config <path>     Config file (overrides discovery) (STAGEHAND_CONFIG)
      --timeout <dur>     Generation timeout      (STAGEHAND_TIMEOUT, e.g. 120s)
  -a, --all                git add -A before snapshotting, even if staged
      --no-auto-stage      If nothing staged, exit 2 instead of auto-staging
      --dry-run            Generate + print the message; do not commit
  -v, --verbose            Print resolved command, raw output, retries
      --no-color           Disable color (also respects NO_COLOR)
      --version            Print version and exit
  -h, --help               Help

See docs/CONFIGURATION.md for the full flag/env/exit-code reference.`

// init wires the ten PRD §15.2 global flags as PERSISTENT flags on rootCmd so
// they are available to the root default action AND inherited by the
// providers/config subcommand trees. --version is NOT registered here (cobra's
// Version field adds it); --help/-h is cobra's built-in. The action flags
// (--all/--no-auto-stage/--dry-run) are read directly off cmd.Flags() in
// runDefault — they are NOT part of config.Flags (config.FlagsLayer has no
// field for them). A zero --timeout default (0s) means "use the config default"
// (120s); it flows into config only when Changed. Registration is centralized
// in registerPersistentFlags so tests can build an equivalent flag set on a
// fresh *cobra.Command without touching the global rootCmd.
func init() {
	registerPersistentFlags(rootCmd)
}

// registerPersistentFlags registers the PRD §15.2 persistent flags on cmd. It
// is a free function (not a closure) so a test can apply the exact same set to
// a freshly-constructed command, keeping buildFlags tests hermetic and
// independent of the package-global rootCmd's flag-parse state.
func registerPersistentFlags(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.String("provider", "", "Provider/agent to use (env: STAGEHAND_PROVIDER)")
	pf.String("model", "", "Model override (env: STAGEHAND_MODEL)")
	pf.String("config", "", "Path to a config file, overriding discovery (env: STAGEHAND_CONFIG)")
	pf.Duration("timeout", 0, "Generation timeout; 0 means use config default 120s (env: STAGEHAND_TIMEOUT)")
	pf.BoolP("all", "a", false, "Run `git add -A` before snapshotting, even if something is already staged")
	pf.Bool("no-auto-stage", false, "If nothing is staged, exit instead of auto-staging")
	pf.Bool("dry-run", false, "Generate and print the message; do not commit")
	pf.BoolP("verbose", "v", false, "Print the resolved command, raw agent output, and retries")
	pf.Bool("no-color", false, "Disable color output (also respects NO_COLOR)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(ui.ExitError)
	}
}
