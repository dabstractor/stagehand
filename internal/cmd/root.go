// Package cmd implements the cobra CLI scaffold for Stagehand (PRD §15.1/§15.2/§15.4/§21.1).
// It provides the root command with all eleven §15.2 global flags (persistent, inherited by every
// future subcommand), a PersistentPreRunE that resolves config once via config.Load(), an Execute()
// function that returns the command error (for exit-code mapping in main), and a Config() accessor
// for RunE consumers. The default action body, subcommands, signal handling, UI, and dry-run logic
// are added by sibling subtasks (S2/S3/S4/P1.M4.T2/T3/T4).
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
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
	Use:           "stagehand",
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
			return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: getwd: %w", err))
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
	RunE: runDefault,
}

func init() {
	pf := rootCmd.PersistentFlags()
	// §15.2 config-backed flags (zero defaults; config.Load owns Layer-7 precedence via fs.Changed).
	pf.StringVar(&flagProvider, "provider", "", "Provider/agent to use (env STAGEHAND_PROVIDER, git stagehand.provider; default auto-detected)")
	pf.StringVar(&flagModel, "model", "", "Model override (env STAGEHAND_MODEL, git stagehand.model; default per-manifest default_model)")
	pf.StringVar(&flagConfig, "config", "", "Path to a config file, overrides discovery (env STAGEHAND_CONFIG)")
	pf.StringVar(&flagTimeout, "timeout", "", "Generation timeout, e.g. \"120s\" or 120 (env STAGEHAND_TIMEOUT, git stagehand.timeout; default 120s)")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "Print resolved command, raw output, retries (env STAGEHAND_VERBOSE)")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable color (env STAGEHAND_NO_COLOR, NO_COLOR; default TTY-aware)")
	// §15.2 behavioral flags (read by S2/S4 RunE; not Config fields).
	pf.BoolVarP(&flagAll, "all", "a", false, "Run git add -A before snapshotting, even if something is staged")
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
	rootCmd.Version = Version // sync from package var (set by main before Execute)
	if ctx != nil {
		rootCmd.SetContext(ctx)
	}
	return rootCmd.Execute()
}
