// Package cmd implements the models command for Stagehand (PRD §9.23 FR-L1/L2, §15.3, §6.2 N2).
// It provides a `models` cobra leaf on root that lists models reachable by a provider.
// Source of truth per provider (FR-L1): (a) if the manifest's list_models_command is set and
// succeeds, run it and print its stdout; (b) if absent or the command fails, print the curated
// FR-D4 per-role tier table from config.DefaultModelsForProvider, annotated with its verification
// date and a "consult <command> --help" hint. Never an HTTP call (§6.2 N2).
//
// Default target is the resolved default provider; --all covers every detected provider.
// Registered via init() in this file — ZERO edits to root.go (providers.go precedent).
package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/ui"
)

// modelsCmd implements `stagehand models [<provider>]` (FR-L1).
var modelsCmd = &cobra.Command{
	Use:   "models [<provider>]",
	Short: "List models reachable by a provider",
	Long: `List the models a provider's CLI can reach — read straight from the agent CLI itself
(never an HTTP call: stagehand has no API key; §6.2 N2 / FR-L1).

Source of truth, in order:
  (a) the manifest's list_models_command — run it, print its stdout; or
  (b) if absent or it fails — stagehand's curated per-role tier table, annotated with its
      verification date and "consult '<command> --help' for the live list".

Default target is the resolved default provider; --all covers every detected provider.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runModels,
}

// flagModelsAll is the local --all/-a flag on modelsCmd. It overrides the inherited persistent
// --all/-a from root (the "git add -A" flag) via pflag's AddFlagSet collision mechanism
// (config.go:142 precedent: a local same-name flag causes the inherited persistent to be skipped).
// MUST define both the long name AND the -a shorthand, else -a is left unbound.
var flagModelsAll bool

// modelTarget pairs a provider name with its merged manifest for rendering.
type modelTarget struct {
	name     string
	manifest provider.Manifest
}

func init() {
	modelsCmd.Flags().BoolVarP(&flagModelsAll, "all", "a", false,
		"List models for every detected provider (default: the resolved default provider)")
	rootCmd.AddCommand(modelsCmd) // NO edit to root.go (providers.go precedent)
}

// runModels resolves the target provider set and renders each block. Target resolution:
//   - --all → every detected provider
//   - explicit arg → that provider (must be known AND detected)
//   - no arg → the resolved default provider
//
// Returns nil on success; exitcode.New(exitcode.Error, …) on errors. Never calls os.Exit.
func runModels(cmd *cobra.Command, args []string) error {
	if flagModelsAll && len(args) > 0 {
		return exitcode.New(exitcode.Error, fmt.Errorf("--all cannot be combined with a provider argument"))
	}

	reg, err := newRegistry() // reuse providers.go helper (same package)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}
	installed := installedNames(reg) // reuse
	cfg := Config()                  // non-nil (PersistentPreRunE loaded it)

	var targets []modelTarget

	switch {
	case flagModelsAll:
		if len(installed) == 0 {
			return exitcode.New(exitcode.Error, fmt.Errorf(
				"no providers detected on $PATH; install one of stagehand's supported agents"))
		}
		for _, name := range installed {
			m, _ := reg.Get(name)
			targets = append(targets, modelTarget{name: name, manifest: m})
		}

	case len(args) == 1:
		name := args[0]
		m, ok := reg.Get(name)
		if !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name))
		}
		// Check detection: the provider must be on $PATH
		detected := false
		for _, inst := range installed {
			if inst == name {
				detected = true
				break
			}
		}
		if !detected {
			return exitcode.New(exitcode.Error, fmt.Errorf(
				"provider %q is not detected on $PATH; install it or run 'stagehand models --all' for detected providers", name))
		}
		targets = append(targets, modelTarget{name: name, manifest: m})

	default: // no arg, no --all
		dflt := resolvedDefault(cfg, reg, installed)
		if dflt == "" {
			return exitcode.New(exitcode.Error, fmt.Errorf(
				"no provider detected on $PATH; pass a provider name or install one of stagehand's supported agents"))
		}
		m, _ := reg.Get(dflt)
		targets = append(targets, modelTarget{name: dflt, manifest: m})
	}

	for i, t := range targets {
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout()) // blank line between --all blocks
		}
		renderModelBlock(cmd, t, cfg)
	}
	return nil
}

// renderModelBlock implements FR-L1 source-of-truth order for one provider:
//   - (a) if ListModelsCommand is set, run it via provider.Execute; on success print the live list.
//   - (b) if absent or the command fails, print the curated FR-D4 table.
//
// A command failure (non-zero exit / timeout / not found) is NOT a hard error — it triggers the
// curated fallback (stdout) + a one-line notice (stderr), exit 0.
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
		// FR-L1 (b): command failed → curated fallback
		fmt.Fprintf(cmd.ErrOrStderr(), "stagehand: %s list command failed (%v); using curated table\n", t.name, err)
		// fall through to curated
	}
	printCuratedTable(cmd.OutOrStdout(), t)
}

// printCuratedTable renders the FR-D4 per-role tier table for a provider to w. Uses FIXED role
// order [planner, stager, message, arbiter] — NOT map iteration (random order breaks golden tests).
// Empty stager cell (non-stager-capable providers) renders as "—". Prints the verification date
// footer + consult hint. Pure (takes io.Writer) — golden-testable with *bytes.Buffer.
func printCuratedTable(w io.Writer, t modelTarget) {
	col := config.DefaultModelsForProvider(t.name)
	if col == nil {
		// User-defined provider with no FR-D4 column
		fmt.Fprintf(w, "%s:\n  no list_models_command and no curated per-role defaults.\n  Add list_models_command to [provider.%s] or set [role.*] models in config.\n",
			t.name, t.name)
		return
	}
	fmt.Fprintf(w, "%s:\n", t.name)
	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		m := col[role]
		if m == "" {
			m = "—" // non-stager-capable providers
		}
		fmt.Fprintf(w, "  %-8s %s\n", role, m)
	}
	fmt.Fprintf(w, "\nStagehand's curated per-role defaults (verified %s). The live list may differ — consult `%s --help`.\n",
		config.DefaultModelsVerificationDate, t.manifest.DetectCommand())
}

// printLiveList prints the heading + the CLI's verbatim stdout for a provider. Pure (takes io.Writer).
func printLiveList(w io.Writer, name, stdout string) {
	fmt.Fprintf(w, "%s:\n", name)
	if stdout == "" {
		fmt.Fprintf(w, "  (no models reported)\n")
		return
	}
	fmt.Fprint(w, stdout) // verbatim; preserve the CLI's own formatting
	if !strings.HasSuffix(stdout, "\n") {
		fmt.Fprintln(w) // tidy trailing newline for block separation
	}
}
