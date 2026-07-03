// Package cmd implements the integrate command group for Stagehand (PRD §9.21 FR-I1/I2).
// It provides an `integrate` cobra command with three leaf subcommands: `list` (show
// integration targets with detection + status + config path), `install <target>…`
// (apply one or more integrations with detection gating), and `remove <target>…`
// (uninstall symmetry).
//
// The group defines a no-op PersistentPreRunE that OVERRIDES root's config.Load —
// integrate edits user dotfiles (gitconfig, lazygit config) and works OUTSIDE a git
// repo, so it must NOT trigger config.Load's first-run bootstrap write (FR-B3).
// Same rationale as hook.go.
//
// Registered via init() — ZERO edits to root.go (providers.go/hook.go pattern).
package cmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/integrate"
)

// defaultEntries is the SINGLE registration seam (PRD §9.21). S2 ships NONE — the
// command surface + registry + detection gating are provably correct with fakes
// before any concrete target exists. T2.S1 (git-alias) and T2.S2 (lazygit) APPEND
// their integrate.Entry impls here (one line each). A function-var (not init/Register)
// so the Registry is always freshly built (mirrors provider.NewRegistry's discipline)
// and tests can swap it.
var defaultEntries = func() []integrate.Entry {
	return []integrate.Entry{newGitAliasEntry()} // T2.S2 appends &lazygitEntry{...} here later
}

var flagIntegrateYes bool // --yes (persistent on integrateCmd; install+remove honor it, list ignores it)

// integrateCmd is the PRD §9.21 integrate command group. No RunE → bare `stagehand
// integrate` prints help. Its no-op PersistentPreRunE OVERRIDES root's (cobra runs only
// the nearest): list/install/remove do NOT need config — they edit user dotfiles and
// work outside a git repo, and must not trigger config.Load's first-run bootstrap
// write (FR-B3). Same rationale as hook.go.
var integrateCmd = &cobra.Command{
	Use:   "integrate",
	Short: "Wire stagehand into installed git tools",
	Long: `Install or remove stagehand integrations for the git tools you already run (PRD §9.21).

Targets are explicit — name one or more of the supported tools (see 'stagehand integrate list').
Every file edit runs the no-mangle protocol: a unified-diff preview, a y/N prompt (use --yes to skip),
a timestamped backup, and a post-write re-parse with automatic restore on failure.`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // SKIP config.Load (like hook)
}

var integrateListCmd = &cobra.Command{
	Use:           "list",
	Short:         "List integration targets and their status",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runIntegrateList,
}

var integrateInstallCmd = &cobra.Command{
	Use:           "install <target>…",
	Short:         "Install one or more stagehand integrations",
	Args:          cobra.MinimumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runIntegrateInstall,
}

var integrateRemoveCmd = &cobra.Command{
	Use:           "remove <target>…",
	Short:         "Remove one or more stagehand integrations",
	Args:          cobra.MinimumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runIntegrateRemove,
}

func init() {
	integrateCmd.PersistentFlags().BoolVar(&flagIntegrateYes, "yes", false,
		"Skip the preview prompt and apply changes directly (for scripts)")
	integrateCmd.AddCommand(integrateListCmd, integrateInstallCmd, integrateRemoveCmd)
	rootCmd.AddCommand(integrateCmd) // NO edit to root.go (providers.go/hook.go pattern)
}

// ---------------------------------------------------------------------------
// RunE functions — build registry, delegate to pure dispatch
// ---------------------------------------------------------------------------

// runIntegrateList — RunE: build the registry, delegate to the pure printer.
func runIntegrateList(cmd *cobra.Command, _ []string) error {
	reg := integrate.NewRegistry(defaultEntries())
	printIntegrateList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), reg)
	return nil
}

// runIntegrateInstall — RunE: build registry + opts, delegate to pure dispatch.
func runIntegrateInstall(cmd *cobra.Command, args []string) error {
	reg := integrate.NewRegistry(defaultEntries())
	opts := integrate.InstallOptions{Yes: flagIntegrateYes, Out: cmd.ErrOrStderr(), Confirm: nil /* ⇒ Default */}
	return dispatchInstall(cmd.Context(), reg, args, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

// runIntegrateRemove — RunE: build registry + opts, delegate to pure dispatch.
func runIntegrateRemove(cmd *cobra.Command, args []string) error {
	reg := integrate.NewRegistry(defaultEntries())
	opts := integrate.RemoveOptions{Yes: flagIntegrateYes, Out: cmd.ErrOrStderr(), Confirm: nil /* ⇒ Default */}
	return dispatchRemove(cmd.Context(), reg, args, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

// ---------------------------------------------------------------------------
// Pure dispatch functions (testable without cobra)
// ---------------------------------------------------------------------------

// printIntegrateList — PURE: the FR-I1 table (TARGET/DETECTED/STATUS/CONFIG),
// one row per List() (sorted). Deterministic; takes io.Writer.
func printIntegrateList(ctx context.Context, stdout, stderr io.Writer, reg *integrate.Registry) {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tDETECTED\tSTATUS\tCONFIG")
	for _, e := range reg.List() {
		detected := "✗"
		if err := e.Detect(ctx); err == nil {
			detected = "✓"
		}
		status := "not installed"
		if s, err := e.Status(ctx); err == nil {
			status = s.String()
		}
		cfg := "—"
		if p, err := e.ConfigPath(ctx); err == nil && p != "" {
			cfg = p
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name(), detected, status, cfg)
	}
	tw.Flush()
}

// dispatchInstall — PURE: per-target Detect gate → Install; batch continue-on-failure;
// exit 1 iff any failed/unknown/gated. Decline/NoChange are NOT failures.
func dispatchInstall(ctx context.Context, reg *integrate.Registry, targets []string, opts integrate.InstallOptions, stdout, stderr io.Writer) error {
	failed := false
	for _, name := range targets {
		e, ok := reg.Get(name)
		if !ok {
			fmt.Fprintf(stderr, "stagehand: unknown target %q; see `stagehand integrate list`.\n", name)
			failed = true
			continue
		}
		if err := e.Detect(ctx); err != nil { // FR-I2 detection gating
			fmt.Fprintf(stderr, "stagehand: %s requires its tool on $PATH, which was not detected (%v); skipping.\n", name, err)
			failed = true
			continue
		}
		res, err := e.Install(ctx, opts)
		if err != nil {
			fmt.Fprintf(stderr, "stagehand: install %s failed: %v\n", name, err)
			failed = true
			continue
		}
		fmt.Fprintln(stdout, formatInstallResult(res))
	}
	if failed {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: one or more targets failed"))
	}
	return nil
}

// dispatchRemove — PURE: per-target Detect gate → Remove; batch continue-on-failure;
// exit 1 iff any failed/unknown/gated. Decline/NoChange are NOT failures.
func dispatchRemove(ctx context.Context, reg *integrate.Registry, targets []string, opts integrate.RemoveOptions, stdout, stderr io.Writer) error {
	failed := false
	for _, name := range targets {
		e, ok := reg.Get(name)
		if !ok {
			fmt.Fprintf(stderr, "stagehand: unknown target %q; see `stagehand integrate list`.\n", name)
			failed = true
			continue
		}
		if err := e.Detect(ctx); err != nil { // FR-I2 detection gating
			fmt.Fprintf(stderr, "stagehand: %s requires its tool on $PATH, which was not detected (%v); skipping.\n", name, err)
			failed = true
			continue
		}
		res, err := e.Remove(ctx, opts)
		if err != nil {
			fmt.Fprintf(stderr, "stagehand: remove %s failed: %v\n", name, err)
			failed = true
			continue
		}
		fmt.Fprintln(stdout, formatRemoveResult(res))
	}
	if failed {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: one or more targets failed"))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Result formatters
// ---------------------------------------------------------------------------

// formatInstallResult maps S1's Outcome to the per-target stdout verb. Decline/NoChange
// are informational, NOT errors (exit 0).
func formatInstallResult(r integrate.InstallResult) string {
	switch r.Outcome {
	case integrate.OutcomeCreated:
		return fmt.Sprintf("Installed %s integration (created %s).", r.Target, r.Path)
	case integrate.OutcomeUpdated:
		s := fmt.Sprintf("Updated %s integration (%s).", r.Target, r.Path)
		if r.Backup != "" {
			s += fmt.Sprintf(" Backup: %s", r.Backup)
		}
		return s
	case integrate.OutcomeRemoved:
		return fmt.Sprintf("Removed %s integration.", r.Target)
	case integrate.OutcomeDeclined:
		return fmt.Sprintf("Declined %s — nothing was changed.", r.Target)
	default: // OutcomeNoChange
		return fmt.Sprintf("No changes for %s (already installed).", r.Target)
	}
}

// formatRemoveResult maps S1's Outcome to the per-target stdout verb for remove.
func formatRemoveResult(r integrate.RemoveResult) string {
	switch r.Outcome {
	case integrate.OutcomeRemoved:
		s := fmt.Sprintf("Removed %s integration (%s).", r.Target, r.Path)
		if r.Backup != "" {
			s += fmt.Sprintf(" Backup: %s", r.Backup)
		}
		return s
	case integrate.OutcomeDeclined:
		return fmt.Sprintf("Declined %s remove — nothing was changed.", r.Target)
	default: // OutcomeNoChange
		return fmt.Sprintf("No changes for %s (not installed).", r.Target)
	}
}
