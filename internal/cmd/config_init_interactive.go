package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// interactiveStdinIsTTY is the TTY gate for --interactive. Overridable in tests so the happy-path
// Execute test can force true while piping answers via rootCmd.SetIn(reader).
var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }

// wizardResult is the output of the interactive wizard: a chosen provider + optional per-role model
// overrides (role→model). overrides is nil when the user accepts every default (→ byte-identical to
// GenerateBootstrapConfig(chosen)).
type wizardResult struct {
	provider  string
	overrides map[string]string // role ("planner"|"stager"|"message"|"arbiter") → model; nil = no edits
}

// needsInferencePrefix reports whether the provider requires a slash-prefix on models (inference/model
// form). Includes pi (provider_flag set), opencode (correct form even without provider_flag), and any
// provider with a non-empty provider_flag. The wizard re-prompts on a bare edited model for these.
func needsInferencePrefix(name string, m provider.Manifest) bool {
	pf := ""
	if m.ProviderFlag != nil {
		pf = *m.ProviderFlag
	}
	return name == "pi" || name == "opencode" || pf != ""
}

// runConfigInitInteractive is the RunE entry for `config init --interactive` (FR-L3, PRD §9.23/§15.3).
// It checks the TTY gate, validates composition flags, runs the pure wizard, and writes the result
// via the shared writeBootstrapFile.
func runConfigInitInteractive(cmd *cobra.Command, _ []string) error {
	// --interactive --template is a usage error (they write different things).
	if tmpl, _ := cmd.Flags().GetBool("template"); tmpl {
		return exitcode.New(exitcode.Error, fmt.Errorf(
			"--interactive writes a populated config; --template writes the inert reference — choose one"))
	}

	// TTY gate: non-TTY stdin → exit 1 pointing at plain config init.
	if !interactiveStdinIsTTY() {
		return exitcode.New(exitcode.Error, fmt.Errorf(
			"--interactive requires a terminal on stdin; run plain 'stagehand config init' instead (it stays non-interactive for post-install scripts, FR-B3)"))
	}

	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}

	installed := installedNames(reg)
	if len(installed) == 0 {
		return exitcode.New(exitcode.Error, fmt.Errorf(
			"no providers detected on $PATH; run plain 'stagehand config init' to default to pi, or install one of: %s",
			strings.Join(reg.PreferredBuiltins(), ", ")))
	}

	// --provider pre-select: validate via reg.Get; detection NOT required for an explicit pin.
	pinName, _ := cmd.Flags().GetString("provider")
	if pinName != "" {
		if _, ok := reg.Get(pinName); !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q (use a built-in: %s)",
				pinName, strings.Join(reg.PreferredBuiltins(), ", ")))
		}
	}

	defaultName := resolvedDefault(Config(), reg, installed)

	res, err := runInteractiveWizard(cmd.InOrStdin(), cmd.OutOrStdout(), reg, installed, defaultName, pinName)
	if err != nil {
		return exitcode.New(exitcode.Error, err)
	}

	content := config.GenerateBootstrapConfigWithOverrides(res.provider, res.overrides)

	path := config.ResolveConfigPath(flagConfig)
	force, _ := cmd.Flags().GetBool("force")
	if err := writeBootstrapFile(cmd, path, content, force); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path)
	return nil
}

// runInteractiveWizard is the PURE interactive wizard: reads answers from r, writes prompts to w.
// No os.Stdin / IsTerminal inside — fully testable with bytes.Buffer and strings.NewReader.
// Steps: (1) choose provider (skip if pinName != ""), (2) accept-or-edit per-role models,
// (3) re-prompt bare models on multi-backend providers.
func runInteractiveWizard(r io.Reader, w io.Writer, reg *provider.Registry, installed []string, defaultName, pinName string) (wizardResult, error) {
	br := bufio.NewReader(r)
	readLine := func() (string, error) {
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return "", io.EOF
		}
		return strings.TrimSpace(line), nil
	}

	installedSet := make(map[string]struct{}, len(installed))
	for _, n := range installed {
		installedSet[n] = struct{}{}
	}

	chosen := pinName
	if chosen == "" {
		// List detected providers in FR-D1 priority order, highlight the default.
		preferred := reg.PreferredBuiltins()
		fmt.Fprintln(w, "Detected providers:")
		detected := make([]string, 0, len(installed))
		for _, name := range preferred {
			if _, ok := installedSet[name]; ok {
				detected = append(detected, name)
			}
		}
		for i, name := range detected {
			marker := ""
			if name == defaultName {
				marker = " (default)"
			}
			fmt.Fprintf(w, "  %d. %s%s\n", i+1, name, marker)
		}

		// Prompt: loop until valid or EOF.
		for {
			prompt := fmt.Sprintf("Pick a provider [%s]: ", defaultName)
			fmt.Fprint(w, prompt)
			choice, err := readLine()
			if err != nil {
				return wizardResult{}, fmt.Errorf("unexpected end of input")
			}
			if choice == "" {
				chosen = defaultName
				break
			}
			// Accept a number or a name.
			if idx, ok := parseProviderIndex(choice, len(detected)); ok && idx < len(detected) {
				chosen = detected[idx]
				break
			}
			if _, ok := installedSet[choice]; ok {
				chosen = choice
				break
			}
			fmt.Fprintf(w, "unknown/undetected provider %q; choose from the list above\n", choice)
		}
	}

	m, _ := reg.Get(chosen)
	multi := needsInferencePrefix(chosen, m)

	// Get per-role defaults. For pi, show blanked defaults (what gets written on accept).
	overrides := map[string]string{}
	roles := []string{"planner", "stager", "message", "arbiter"}
	models := config.DefaultModelsForProvider(chosen)

	for _, role := range roles {
		// Display default: for pi, the bootstrap blanks models → show "".
		display := ""
		if chosen != "pi" && models != nil {
			display = models[role]
			// For non-stager-capable providers, stager table value is "" — show fallback hint.
			if role == "stager" && display == "" {
				_, fbModel := config.StagerFallback(chosen, models)
				if fbModel != "" {
					display = fmt.Sprintf("%s (routed to fallback)", fbModel)
				}
			}
		}

		prompt := fmt.Sprintf("%s model [%s]: ", role, display)
		if multi {
			prompt = fmt.Sprintf("%s model [%s]; include the inference/ prefix, e.g. zai/glm-5.2: ",
				role, display)
		}

		// Loop until valid or EOF.
		for {
			fmt.Fprint(w, prompt)
			value, err := readLine()
			if err != nil {
				return wizardResult{}, fmt.Errorf("unexpected end of input")
			}
			if value == "" {
				break // accept default (no override added)
			}
			// Multi-backend: reject bare edited models (no "/").
			if multi && !strings.Contains(value, "/") {
				fmt.Fprintf(w, "multi-backend provider: include the inference backend as a prefix, e.g. zai/glm-5.2\n")
				continue
			}
			overrides[role] = value
			break
		}
	}

	// Empty-map discipline: return nil so GenerateBootstrapConfigWithOverrides(chosen, nil) is byte-identical.
	if len(overrides) == 0 {
		overrides = nil
	}

	return wizardResult{provider: chosen, overrides: overrides}, nil
}

// parseProviderIndex tries to parse s as a 1-based index. Returns (index, true) on success.
func parseProviderIndex(s string, max int) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n - 1, n >= 1 && n <= max
}

// ensure the file is valid Go (no trailing syntax issues).
var _ = io.EOF
