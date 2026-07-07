// Package cmd implements the providers command group for Stagecoach (PRD §15.3, §9.11 FR46/FR47/FR48).
// It provides a `providers` cobra command with two leaf subcommands: `list` (show all providers with
// detection + default status) and `show <name>` (print a provider's merged manifest as TOML). Both are
// thin READ-only views over the provider.Registry (P1.M2.T3.S1), with user overrides from config reflected.
//
// Registered via init() in this file — ZERO edits to root.go (parallel-safe with S2, design §0/§12).
package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/provider"
)

// providersCmd is the PRD §15.3 "providers" command group. It has NO RunE → bare `stagecoach providers`
// prints help (cobra default). list/show are its leaves (registered in init()). Root's PersistentPreRunE
// is INHERITED (none of the three define their own) so config loads for both (FR46/FR47 need cfg.Providers).
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage AI provider manifests",
	Long: `Inspect the built-in and user-defined provider manifests Stagecoach uses to generate commits.

User-defined providers (from the global or repo-local config file) override built-ins of the same
name; new names add new providers (PRD §12.8).`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// providersListCmd implements `stagecoach providers list` (FR46).
var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List providers",
	Long: `List all known providers (built-in and user-defined).

Each provider is shown with:
  NAME      the provider name (a built-in, or from [provider.<name>] in config)
  DETECTED  ✓ if the provider's command is found on $PATH, ✗ otherwise
  DEFAULT   (default) marks the provider that will be used when no --provider is given
            (the configured provider, or the first detected built-in in preference order)

User-defined providers override built-ins of the same name; new names add new providers (PRD §12.8).`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runProvidersList,
}

// providersShowCmd implements `stagecoach providers show <name>` (FR47).
var providersShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a provider manifest",
	Long: `Print the fully-resolved manifest for <name> as TOML.

The manifest is the built-in definition merged with any user overrides from config (PRD §12.8).
Unknown provider names exit with code 1.`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runProvidersShow,
}

func init() {
	providersCmd.AddCommand(providersListCmd)
	providersCmd.AddCommand(providersShowCmd)
	rootCmd.AddCommand(providersCmd) // register on S1's root — NO edit to root.go (design §0)
}

// runProvidersList implements `stagecoach providers list` (FR46). It builds the merged registry from
// config, computes which providers are installed and which is the resolved default, and prints a
// NAME/DETECTED/DEFAULT table to stdout. Returns nil on success; exitcode.New(Error, …) on a registry-
// build failure (main maps to exit 1). Never calls os.Exit.
func runProvidersList(cmd *cobra.Command, args []string) error {
	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	installed := installedNames(reg)
	defaultName := resolvedDefault(Config(), reg, installed)
	printProvidersList(cmd.OutOrStdout(), reg, defaultName)
	return nil
}

// runProvidersShow implements `stagecoach providers show <name>` (FR47). It builds the merged registry
// and prints the TOML for <name> (built-in ⊕ overrides) to stdout. Unknown name → exitcode.New(Error,
// …) (exit 1). cobra.ExactArgs(1) guarantees args[0] exists. Never calls os.Exit.
func runProvidersShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	s, err := reg.MarshalTOML(name)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name))
	}
	fmt.Fprint(cmd.OutOrStdout(), s) // MarshalTOML output already ends in '\n' (design §6) — no extra newline
	return nil
}

// newRegistry builds the merged provider.Registry from config the SAME way pkg/stagecoach.buildDeps
// does (design §2): DecodeUserOverrides(cfg.Providers) → NewRegistry. This guarantees list/show
// reflect EXACTLY what GenerateCommit would use. A decode error (malformed [provider.X]) is returned
// (caller maps to exit 1). If Config() is nil (defensive; PersistentPreRunE guarantees non-nil for
// list/show), uses no overrides → built-ins only.
func newRegistry() (*provider.Registry, error) {
	var raw map[string]map[string]any
	if cfg := Config(); cfg != nil {
		raw = cfg.Providers
	}
	overrides, err := provider.DecodeUserOverrides(raw)
	if err != nil {
		return nil, fmt.Errorf("provider overrides: %w", err)
	}
	return provider.NewRegistry(overrides), nil
}

// installedNames returns the Names of providers whose command is on $PATH (FR46 detection). Mirrors
// pkg/stagecoach.buildDeps verbatim. reg.List() is sorted ascending, so the result is too.
func installedNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// resolvedDefault returns the provider stagecoach would use with no --provider (FR46 "show the resolved
// default"): cfg.Provider if explicitly configured (Layer 1-7), else reg.DefaultProvider(installed)
// (first preferred built-in on PATH). Mirrors pkg/stagecoach.buildDeps. A nil cfg (defensive) → auto.
func resolvedDefault(cfg *config.Config, reg *provider.Registry, installed []string) string {
	if cfg != nil && cfg.Provider != "" {
		return cfg.Provider
	}
	return reg.DefaultProvider(installed)
}

// printProvidersList renders the FR46 table to w (stdout): a header + one row per provider (sorted by
// Name via List()), with ✓/✗ in DETECTED and "(default)" on the row whose Name == defaultName. Uses
// text/tabwriter for aligned columns. Takes an io.Writer so P1.M4.T3 can restyle/recolor without
// touching the resolver (design §5). Exact spacing is tabwriter's concern; tests assert on NAME +
// marker substrings.
func printProvidersList(w io.Writer, reg *provider.Registry, defaultName string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tDETECTED\tDEFAULT")
	for _, m := range reg.List() {
		detected := "✗"
		if reg.IsInstalled(m) {
			detected = "✓"
		}
		marker := ""
		if m.Name == defaultName {
			marker = "(default)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Name, detected, marker)
	}
	tw.Flush()
}
