// Package cmd implements the hook command group for Stagehand (PRD §9.20 FR-H1/H2/H3/H5).
// It provides a `hook` cobra command with three leaf subcommands: `install` (write the
// prepare-commit-msg hook, with optional --strict/--print), `uninstall` (remove a stagehand-owned
// hook), and `status` (report none|stagehand (v1)|foreign).
//
// The group defines a no-op PersistentPreRunE that OVERRIDES root's config.Load — hook commands
// need only the repo's hooks dir, never the resolved config, and must not trigger config.Load's
// first-run bootstrap write (FR-B3).
//
// Registered via init() — ZERO edits to root.go (providers.go/config.go pattern).
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/hook"
)

var (
	flagHookPrint  bool
	flagHookStrict bool
)

// hookCmd is the PRD §9.20 hook command group. No RunE → bare `stagehand hook` prints help.
// Its no-op PersistentPreRunE OVERRIDES root's (cobra runs only the nearest): install/uninstall/status
// need only the repo's hooks dir, never the resolved config — and must not trigger config.Load's
// first-run bootstrap write (FR-B3). P1.M3.T2.S1 adds `hook exec` as a sibling leaf to this group.
var hookCmd = &cobra.Command{
	Use:               "hook",
	Short:             "Manage the per-repo prepare-commit-msg hook",
	Long:              `Install, remove, or inspect stagehand's per-repo prepare-commit-msg hook (PRD §9.20).`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
}

var hookInstallCmd = &cobra.Command{
	Use:           "install",
	Short:         "Install the prepare-commit-msg hook in this repo",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookInstall,
}

var hookUninstallCmd = &cobra.Command{
	Use:           "uninstall",
	Short:         "Remove the stagehand prepare-commit-msg hook",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookUninstall,
}

var hookStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Report the prepare-commit-msg hook state (none|stagehand (v1)|foreign)",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookStatus,
}

func init() {
	hookInstallCmd.Flags().BoolVar(&flagHookPrint, "print", false,
		"Write the hook script to stdout instead of installing it")
	hookInstallCmd.Flags().BoolVar(&flagHookStrict, "strict", false,
		"Bake --strict into the hook so generation failures abort the commit (default: never block)")
	hookCmd.AddCommand(hookInstallCmd, hookUninstallCmd, hookStatusCmd)
	rootCmd.AddCommand(hookCmd) // register on root — NO edit to root.go (providers.go pattern)
}

// hooksDir resolves this repo's absolute hooks directory via S1's git.HooksPath.
func hooksDir(ctx context.Context) (string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return git.New(repoDir).HooksPath(ctx)
}

func runHookInstall(cmd *cobra.Command, _ []string) error {
	// Bake an explicit --config into the installed script so `hook exec` at commit time resolves the
	// SAME config the user selected at install time (report Finding 4). When --config is unset (""), no
	// STAGEHAND_CONFIG line is emitted and `hook exec` falls back to env/discovery as before.
	configPath := ""
	if cmd.Flags().Changed("config") {
		configPath = flagConfig
	}
	if flagHookPrint { // FR-H2: --print bypasses disk entirely (works outside a repo)
		fmt.Fprint(cmd.OutOrStdout(), hook.Script(flagHookStrict, configPath))
		return nil
	}
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}
	prev, err := hook.Install(dir, flagHookStrict, configPath)
	if errors.Is(err, hook.ErrForeignHook) { // FR-H2 never-clobber refusal
		fmt.Fprintf(cmd.ErrOrStderr(),
			"stagehand: a foreign prepare-commit-msg hook already exists; refusing to overwrite it.\n"+
				"To use stagehand, add this line to your existing hook:\n\n    %s\n",
			hook.InvocationLine(flagHookStrict))
		return exitcode.New(exitcode.Error, nil) // silent exit 1 — message already printed
	}
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: install hook: %w", err))
	}
	verb := "Installed"
	if prev == hook.StatusStagecoach {
		verb = "Updated"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s stagehand prepare-commit-msg hook.\n", verb)
	return nil
}

func runHookUninstall(cmd *cobra.Command, _ []string) error {
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}
	switch _, err := hook.Uninstall(dir); {
	case errors.Is(err, hook.ErrForeignHook):
		fmt.Fprintln(cmd.ErrOrStderr(),
			"stagehand: prepare-commit-msg hook is foreign; refusing to remove it.")
		return exitcode.New(exitcode.Error, nil) // exit 1
	case errors.Is(err, hook.ErrNoHook):
		fmt.Fprintln(cmd.OutOrStdout(), "No stagehand prepare-commit-msg hook to remove.")
		return nil // idempotent — exit 0
	case err != nil:
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: uninstall hook: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Removed stagehand prepare-commit-msg hook.")
	return nil
}

func runHookStatus(cmd *cobra.Command, _ []string) error {
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}
	st, err := hook.Detect(dir)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: hook status: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), st.String()) // "none" / "stagehand (v1)" / "foreign"
	return nil
}
