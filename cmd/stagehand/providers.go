// providers.go implements the `stagehand providers` command tree (PRD §15.3,
// FR46–FR48, US10). It is pure CLI glue over the already-built dependencies:
// it consumes internal/config.Load (P1.M5.T3.S1: resolved default + the
// field-merge registry) and internal/provider.Registry (P1.M2.T3.S2:
// List/Detect/Get) read-only, and renders two subcommands:
//
//   - `providers list` (FR46): every built-in + user provider, each marked
//     detected-on-$PATH vs not via reg.Detect, plus the resolved default
//     provider+model.
//   - `providers show <name>` (FR47): the provider's FULLY-RESOLVED manifest
//     (built-in field-merged with user overrides via the registry) as TOML,
//     using the Manifest's existing toml tags (no DTO).
//
// The tree self-registers onto the package-level rootCmd (defined in main.go)
// via a package-level init() below, so main.go stays untouched. That keeps
// this task (P1.M7.T3.S1) conflict-free with the sibling task P1.M7.T2.S1,
// which will later own rootCmd.Run + the persistent flags. No Run/RunE is
// added to rootCmd here (that is P1.M7.T2.S1's job); the bare `stagehand`
// and `stagehand providers` invocations therefore print help.
//
// This file is intentionally thin: it imports ONLY internal/{config,provider}
// (plus fmt/io, cobra, and go-toml/v2 for the show encoder). It deliberately
// does NOT import internal/git or internal/generate — the providers command
// needs neither, and keeping the leaf thin preserves the layering invariant.
package main

import (
	"fmt"
	"io"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/provider"
)

// init registers the providers command tree onto rootCmd at package init time.
// Registering here (rather than by editing main.go) means this task does not
// touch main.go, avoiding a conflict with the sibling task P1.M7.T2.S1 that
// owns rootCmd.Run and the persistent flags. No golangci-lint config exists in
// the repo, so the use of init() is safe.
func init() {
	rootCmd.AddCommand(newProvidersCmd())
}

// newProvidersCmd builds the `providers` parent command with its `list` and
// `show` children (FR46/FR47/FR48). The parent itself has no Run, so a bare
// `stagehand providers` prints the command's help (cobra default).
func newProvidersCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "providers",
		Short: "List providers and show resolved manifests",
		Long: "Inspect stagehand's agent providers.\n\n" +
			"  providers list         List every built-in and user provider, mark each\n" +
			"                         detected-on-$PATH vs not, and show the resolved\n" +
			"                         default provider and model (FR46).\n\n" +
			"  providers show <name>  Print a provider's fully-resolved manifest (built-in\n" +
			"                         field-merged with any user override) as TOML (FR47).",
	}
	parent.AddCommand(newProvidersListCmd(), newProvidersShowCmd())
	return parent
}

// newProvidersListCmd builds the `providers list` subcommand (FR46). It resolves
// cfg+reg via config.Load (defaults→global file→repo file→repo git-config; the
// flags layer is empty because the persistent flags are not yet wired by
// P1.M7.T2.S1), detects installed providers via reg.Detect, resolves the
// default provider+model (Default() leaves both empty, so auto-resolution is
// the common path), and delegates rendering to the pure renderProvidersList.
func newProvidersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known providers, mark detected, show the default",
		Long: "List every built-in and user provider. Each row marks whether the\n" +
			"provider's executable is detected on $PATH, and a trailing line names the\n" +
			"resolved default provider and model (FR46).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, reg, _, err := config.Load(config.Flags{}, "")
			if err != nil {
				return err
			}
			detected := reg.Detect()
			name, model := resolveDefault(cfg, reg, detected)
			return renderProvidersList(cmd.OutOrStdout(), reg, detected, name, model)
		},
	}
}

// newProvidersShowCmd builds the `providers show <name>` subcommand (FR47). It
// resolves reg via config.Load, looks up the named provider's fully-resolved
// manifest (built-in ⊕ user override), and writes it as TOML. The Manifest
// struct already carries toml tags on every field, so no DTO is needed — the
// emitted TOML is the exact manifest the executor will render a command from,
// which is the debugging surface promised by US10.
func newProvidersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Print a provider's fully-resolved manifest as TOML (FR47)",
		Long: "Print the fully-resolved manifest (built-in field-merged with any user\n" +
			"override via the registry) for the named provider as TOML, so you can see\n" +
			"the exact command stagehand will render (FR47).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg, _, err := config.Load(config.Flags{}, "")
			if err != nil {
				return err
			}
			return showProviderManifest(cmd.OutOrStdout(), reg, args[0])
		},
	}
}

// resolveDefault picks the provider+model to flag as the default for the list
// command. Default() leaves cfg.Provider and cfg.Model empty, so the common
// path auto-resolves: the default provider is cfg.Provider when set, otherwise
// the first detected provider in reg.List() order (reg.List is sorted, so this
// is a deterministic, stable choice); the default model is cfg.Model when set,
// otherwise the resolved provider's manifest DefaultModel. When nothing is
// detected the name is "" and renderProvidersList prints "(none detected)".
func resolveDefault(cfg config.Config, reg *provider.Registry, detected map[string]bool) (string, string) {
	name := cfg.Provider
	if name == "" {
		for _, n := range reg.List() {
			if detected[n] {
				name = n
				break
			}
		}
	}
	model := cfg.Model
	if model == "" {
		if m, ok := reg.Get(name); ok {
			model = m.DefaultModel
		}
	}
	return name, model
}

// renderProvidersList writes the providers-list report to w. It is PURE (its
// only I/O is writing to w) so it is a hermetic test target: the detected map
// and default name/model are passed in, never computed here. The report has a
// header row, one row per provider in reg.List() order (sorted) carrying the
// name, a detected/not-detected status, the manifest's DefaultModel (or
// "(unset)"), and a "(default)" marker on the resolved default, followed by a
// trailing "default provider: <name> (model: <model>)" line. Exact column
// widths are not contractual; the content (names, detected/not detected, the
// default line) is.
func renderProvidersList(w io.Writer, reg *provider.Registry, detected map[string]bool, defaultName, defaultModel string) error {
	fmt.Fprintln(w, "PROVIDER       STATUS         DEFAULT MODEL")
	for _, name := range reg.List() {
		status := "not detected"
		if detected[name] {
			status = "detected"
		}
		m, _ := reg.Get(name)
		model := m.DefaultModel
		if model == "" {
			model = "(unset)"
		}
		marker := ""
		if name == defaultName {
			marker = " (default)"
		}
		fmt.Fprintf(w, "%-15s %-14s %s%s\n", name, status, model, marker)
	}
	dn, dm := defaultName, defaultModel
	if dn == "" {
		dn = "(none detected)"
	}
	if dm == "" {
		dm = "(unset)"
	}
	fmt.Fprintf(w, "\ndefault provider: %s (model: %s)\n", dn, dm)
	return nil
}

// showProviderManifest looks up name in reg and writes its fully-resolved
// manifest as TOML (FR47). An unknown name returns an error mentioning
// "unknown provider" so cobra surfaces it and main.go's os.Exit(1) fires. The
// Manifest struct's existing toml tags do all the encoding work; the output is
// plain TOML with no wrapping table header, which round-trips back to the same
// manifest (modulo nil-vs-empty slice/map normalization — see providers_test).
func showProviderManifest(w io.Writer, reg *provider.Registry, name string) error {
	m, ok := reg.Get(name)
	if !ok {
		return fmt.Errorf("unknown provider %q: run 'stagehand providers list' to see available providers", name)
	}
	return toml.NewEncoder(w).Encode(m)
}
