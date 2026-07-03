// run.go implements the stagehand DEFAULT ACTION (PRD §15.1 synopsis, the
// `stagehand [flags]` path) — the runnable heart of the CLI. It owns the full
// flow resolved by this task (P1.M7.T2.S1): build config.Flags from the
// STAGEHAND_* env + the cobra persistent flags → config.Load → §19 trust
// notice → provider existence/PATH validation → maybeAutoStage (FR16–FR20) →
// stagehand.GenerateCommit → sentinel→exit-code mapping (PRD §15.4).
//
// The flow is decomposed into PURE, os.Exit-free helpers so the MOCKING
// contract is unit-testable without a real agent or a subprocess. This mirrors
// the house pattern in providers.go (dense doc comments; pure helpers as the
// hermetic test targets) and the return-code seam in
// internal/generate/signal.go `handleSignal(...) int`: runDefault RETURNS the
// exit code, and main.go's rootCmd.Run closure performs the single os.Exit.
// Using Run + os.Exit (NOT RunE) is mandatory because cobra's RunE maps every
// non-nil error to exit 1, which cannot carry the §15.4 codes 2/3/124.
//
// Staging POLICY lives in the sibling stage.go and ONLY there (decisions.md §1:
// the v2 seam — CommitStaged/generate assume the index is already staged;
// staging decisions are a CLI concern). maybeAutoStage takes a minimal `stager`
// interface so *git.Git satisfies it in production AND a stub satisfies it in
// tests, keeping the FR16–FR20 logic hermetic. runDefault calls it here and
// routes its result through mapErrorToExitCode.
//
// This file is a plain `package main` sibling of main.go (which OWNS the
// // Package doc comment), mirroring how providers.go/config.go defer the
// package doc to main.go.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
	"github.com/dustin/stagehand/pkg/stagehand"
)

// buildFlags translates the resolved env+CLI-flag sources into the two
// FlagsLayer structs config.Load consumes (FR34 layers 6 and 7). It is the
// single MOCKING surface for the precedence chain: a CLI flag ALWAYS beats its
// env var (Flag is applied ABOVE Env inside Load).
//
// Wiring rules (the present-but-zero trap, decisions.md §6):
//   - ENV string scalars (Provider/Model/ConfigPath): the pointer is set ONLY
//     when os.LookupEnv reports the var present AND non-empty (an empty env
//     value means "not set" per FR35). time.ParseDuration is applied to
//     STAGEHAND_TIMEOUT here; a parse failure is returned as an error so
//     runDefault can surface it as ExitError BEFORE Load.
//   - ENV booleans (Verbose/NoColor): the pointer is set whenever the var is
//     PRESENT (os.LookupEnv ok=true), even when it parses to false — a
//     present-but-false STAGEHAND_VERBOSE still overrides a lower layer's true.
//     "1"/"true"/"yes" (case-insensitive) → true; anything else → false.
//   - CLI flag layer: the pointer is set ONLY when cmd.Flags().Changed(name) is
//     true, so an explicit `--model ""` (Changed, value "") honors the
//     present-but-zero rule and overwrites a lower layer's model. cobra already
//     parses --timeout (Duration) and the booleans, so no re-parsing is needed.
//
// buildFlags never reads files or git-config (Load does that); it is hermetic
// given a *cobra.Command with the persistent flags registered and the process
// environment.
func buildFlags(cmd *cobra.Command) (config.Flags, error) {
	f := config.Flags{}

	// --- ENV layer (FR34 layer 6) ---
	if v, ok := os.LookupEnv("STAGEHAND_PROVIDER"); ok && v != "" {
		f.Env.Provider = &v
	}
	if v, ok := os.LookupEnv("STAGEHAND_MODEL"); ok && v != "" {
		f.Env.Model = &v
	}
	if v, ok := os.LookupEnv("STAGEHAND_CONFIG"); ok && v != "" {
		f.Env.ConfigPath = &v
	}
	if v, ok := os.LookupEnv("STAGEHAND_TIMEOUT"); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return config.Flags{}, fmt.Errorf("invalid STAGEHAND_TIMEOUT %q: %w", v, err)
		}
		f.Env.Timeout = &d
	}
	if v, ok := os.LookupEnv("STAGEHAND_VERBOSE"); ok {
		b := boolish(v)
		f.Env.Verbose = &b
	}
	if v, ok := os.LookupEnv("STAGEHAND_NO_COLOR"); ok {
		b := boolish(v)
		f.Env.NoColor = &b
	}

	// --- CLI FLAG layer (FR34 layer 7, highest) ---
	if cmd.Flags().Changed("provider") {
		v, _ := cmd.Flags().GetString("provider")
		f.Flag.Provider = &v
	}
	if cmd.Flags().Changed("model") {
		v, _ := cmd.Flags().GetString("model")
		f.Flag.Model = &v
	}
	if cmd.Flags().Changed("config") {
		v, _ := cmd.Flags().GetString("config")
		f.Flag.ConfigPath = &v
	}
	if cmd.Flags().Changed("timeout") {
		d, _ := cmd.Flags().GetDuration("timeout")
		f.Flag.Timeout = &d
	}
	if cmd.Flags().Changed("verbose") {
		b, _ := cmd.Flags().GetBool("verbose")
		f.Flag.Verbose = &b
	}
	if cmd.Flags().Changed("no-color") {
		b, _ := cmd.Flags().GetBool("no-color")
		f.Flag.NoColor = &b
	}

	return f, nil
}

// boolish parses an env-var boolean per FR35's convention: "1", "true", and
// "yes" (case-insensitive, surrounding whitespace trimmed) → true; any other
// value (including "0", "false", "no", "") → false. It is the parsing used for
// STAGEHAND_VERBOSE / STAGEHAND_NO_COLOR, where PRESENCE of the var is the
// "set" signal and the parsed value is the boolean. (NO_COLOR itself is read
// directly by ui.NewOutput, which treats any non-empty value as "disable
// color" per https://no-color.org — this helper is only for the STAGEHAND_*
// booleans, which use the 1/true/yes convention.)
func boolish(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// resolveAndCheckProvider resolves the provider name (cfg.Provider, else the
// first detected in reg.List() order) and validates it is known + installed on
// $PATH, returning (name, ui.ExitSuccess, "") on success or
// ("", ui.ExitError, friendlyMsg) on failure. Doing this validation at the CLI
// (rather than letting GenerateCommit fail later) yields the friendly
// "Provider X: command Y not found. Is the agent installed?" message instead
// of generate's less-friendly "no provider configured..." — PRD §15.4 exit 1.
//
// The target executable probed is the manifest's Detect field, falling back to
// Command (matching reg.Detect's own logic), so the error names the exact
// binary the user should install.
func resolveAndCheckProvider(cfg config.Config, reg *provider.Registry) (string, int, string) {
	detected := reg.Detect()
	name := cfg.Provider
	if name == "" {
		// cfg.Provider unset → first detected in sorted reg.List() order
		// (deterministic). Mirrors providers.go resolveDefault + GenerateCommit.
		for _, n := range reg.List() {
			if detected[n] {
				name = n
				break
			}
		}
	}
	if name == "" {
		// Nothing configured AND nothing detected on PATH.
		return "", ui.ExitError, "stagehand: no provider configured and no built-in agent detected on PATH"
	}
	m, ok := reg.Get(name)
	if !ok || !detected[name] {
		target := m.Detect
		if target == "" {
			target = m.Command
		}
		if target == "" {
			// Provider is unknown to the registry (no manifest), so there is no
			// command token to name — fall back to the provider name itself so the
			// message reads naturally instead of "command  not found".
			target = name
		}
		return "", ui.ExitError, fmt.Sprintf(
			"Provider %s: command %s not found. Is the agent installed?", name, target)
	}
	return name, ui.ExitSuccess, ""
}

// buildOptions is the pure cfg+dryRun → stagehand.Options translation. It exists
// as a seam so the dry-run wiring (FR49) is unit-testable without driving the
// full GenerateCommit pipeline. opts.Provider/Model/Timeout carry the resolved
// cfg values (which may be ""/0 for auto-resolution by GenerateCommit); opts is
// the HIGHEST-precedence source inside GenerateCommit, so a non-empty value
// here overrides its own internal config.Load(config.Flags{}). opts.Verbose
// threads the resolved cfg.Verbose (FR50: --verbose/-v/STAGEHAND_VERBOSE) into
// GenerateCommit so the shared *ui.Output driving the executor and the generate
// orchestrator emits the resolved command, raw agent stdout, and retry traces.
func buildOptions(cfg config.Config, dryRun bool) stagehand.Options {
	return stagehand.Options{
		Provider: cfg.Provider,
		Model:    cfg.Model,
		Timeout:  cfg.Timeout,
		DryRun:   dryRun,
		Verbose:  cfg.Verbose,
	}
}

// mapErrorToExitCode maps a staging or GenerateCommit return error to the PRD
// §15.4 exit code via the shipped sentinel aliases. It uses ui.Exit* constants
// — NEVER hardcoded ints. Resolution:
//   - nil                                  → ui.ExitSuccess (0)
//   - stagehand.ErrNothingToCommit         → ui.ExitNothingToCommit (2)
//   - ErrNothingStaged (FR19, CLI-layer)   → ui.ExitNothingToCommit (2)
//   - stagehand.ErrRescue                  → ui.ExitRescue (3)
//   - anything else (ErrHeadMoved, etc.)   → ui.ExitError (1)
//
// Timeout note (research §2): a per-invocation timeout is enforced INSIDE
// generate.CommitStaged via its own context.WithTimeout, which collapses to
// ErrRescue → exit 3 on expiry (the snapshot-rescue contract, decisions.md §3 /
// PRD §18.2). ui.ExitTimeout (124) is therefore RESERVED for a future
// CLI-level deadline and is not returned in v1; documenting 124 as a possible
// code without this branch ever firing today keeps the §15.4 table honest.
func mapErrorToExitCode(err error) int {
	if err == nil {
		return ui.ExitSuccess
	}
	if errors.Is(err, stagehand.ErrNothingToCommit) {
		return ui.ExitNothingToCommit
	}
	if errors.Is(err, stagehand.ErrRescue) {
		return ui.ExitRescue
	}
	if errors.Is(err, ErrNothingStaged) {
		// FR19: nothing staged + auto-stage declined (same exit code as the FR17
		// clean-after-add path; the distinction is the sentinel for errors.Is).
		return ui.ExitNothingToCommit
	}
	return ui.ExitError
}

// runDefault is the full default-action flow (PRD §15.1; research §5). It is
// the testable, os.Exit-free core: main.go's rootCmd.Run closure calls it and
// performs the single os.Exit on its return value. --version short-circuits
// BEFORE this is reached (cobra's Version field handles it in Execute).
//
// Flow: buildFlags → config.Load (.) → §19 trust notice (stderr) → provider
// validation → maybeAutoStage (FR16–FR20) → stagehand.GenerateCommit → exit-code
// mapping. stdout is touched ONLY by GenerateCommit (the FR42 success block and
// the FR49 dry-run message); runDefault writes notices to stderr via
// out.Progressf so stdout stays byte-clean for piping (FR51). cfg.Verbose is
// threaded into GenerateCommit via buildOptions, which honors it end-to-end
// (FR50: the shared *ui.Output driving the executor and generate orchestrator
// emits the resolved command, raw agent stdout, and retries to stderr).
func runDefault(cmd *cobra.Command) int {
	flags, err := buildFlags(cmd)
	if err != nil {
		// e.g. unparseable STAGEHAND_TIMEOUT — surface before Load.
		fmt.Fprintln(os.Stderr, err)
		return ui.ExitError
	}

	cfg, reg, notice, err := config.Load(flags, ".")
	// out is built from the RESOLVED cfg so verbose/color honor the full
	// precedence chain (ui.NewOutput also reads NO_COLOR itself).
	out := ui.NewOutput(os.Stdout, os.Stderr, cfg.Verbose, cfg.NoColor)
	if err != nil {
		out.Progressf("stagehand: %s\n", err)
		return ui.ExitError
	}
	if notice != "" {
		// §19 repo-local trust notice (stderr only).
		out.Progressf("%s\n", notice)
	}

	// Validate the resolved provider exists + is on PATH (friendly exit 1).
	if _, code, msg := resolveAndCheckProvider(cfg, reg); code != ui.ExitSuccess {
		out.Progressf("%s\n", msg)
		return code
	}

	// Staging policy (FR16–FR20), owned HERE — never inside CommitStaged.
	g, err := git.New(".")
	if err != nil {
		out.Progressf("stagehand: %s\n", err)
		return ui.ExitError
	}
	allFlag, _ := cmd.Flags().GetBool("all")
	noAutoStage, _ := cmd.Flags().GetBool("no-auto-stage")
	if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
		return mapErrorToExitCode(err)
	}

	// Generate (and, unless --dry-run, commit). GenerateCommit owns stdout
	// (FR42 block + FR49 message); runDefault must NOT re-print either.
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	opts := buildOptions(cfg, dryRun)
	_, err = stagehand.GenerateCommit(context.Background(), opts)
	return mapErrorToExitCode(err)
}
