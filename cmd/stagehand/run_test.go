package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
	"github.com/dustin/stagehand/pkg/stagehand"
)

// These white-box tests cover the MOCKING contract for the stagehand default
// action (PRD §15.1/§15.2/§15.4, FR42/FR49–FR51). They mirror the repo's
// testing conventions: white-box package main, stdlib + cobra + internal/* +
// pkg/stagehand only, NO testify. The hermetic targets are the pure helpers in
// run.go (buildFlags / resolveAndCheckProvider / mapErrorToExitCode /
// buildOptions), driven with a freshly-constructed *cobra.Command (via
// registerPersistentFlags) and t.Setenv for the STAGEHAND_* env — never the
// package-global rootCmd, so each test's flag-parse state is independent. The
// FR16–FR20 staging-policy tests (maybeAutoStage) live in stage_test.go.

// newTestCmd returns a fresh *cobra.Command carrying the exact PRD §15.2
// persistent flag set (via the same registerPersistentFlags rootCmd uses).
// Using a fresh command (rather than the global rootCmd) keeps buildFlags tests
// hermetic: each test parses its own args without leaking state.
func newTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "stagehand"}
	registerPersistentFlags(cmd)
	return cmd
}

// TestBuildFlags_FlagOverEnv asserts the FR34 precedence MOCKING contract: a
// CLI flag ALWAYS beats its env var. --provider=claude (changed) + env
// STAGEHAND_PROVIDER=pi → both FlagsLayer pointers are set, but config.Load
// (driven on a temp repoDir so no real config/git-config interferes) resolves
// the provider to "claude" (flag layer wins). This is the binding precedence
// assertion the work item requires.
func TestBuildFlags_FlagOverEnv(t *testing.T) {
	t.Setenv("STAGEHAND_PROVIDER", "pi")

	cmd := newTestCmd(t)
	if err := cmd.ParseFlags([]string{"--provider=claude"}); err != nil {
		t.Fatalf("ParseFlags error: %v", err)
	}

	flags, err := buildFlags(cmd)
	if err != nil {
		t.Fatalf("buildFlags error: %v", err)
	}

	// Both layers carry the provider pointer.
	if flags.Env.Provider == nil || *flags.Env.Provider != "pi" {
		t.Errorf("Env.Provider = %v, want &\"pi\"", flags.Env.Provider)
	}
	if flags.Flag.Provider == nil || *flags.Flag.Provider != "claude" {
		t.Errorf("Flag.Provider = %v, want &\"claude\"", flags.Flag.Provider)
	}

	// Precedence: flag > env > ... resolved by Load. Temp repoDir keeps this
	// hermetic (no global/repo file, no git-config, not a git repo).
	cfg, _, _, err := config.Load(flags, t.TempDir())
	if err != nil {
		t.Fatalf("config.Load error: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("cfg.Provider = %q, want %q (flag must beat env)", cfg.Provider, "claude")
	}
}

// TestBuildFlags_PresentButZero asserts the present-but-zero rule for a CLI
// flag: an explicit `--model=` (Changed=true, value "") sets Flags.Flag.Model
// to a NON-NIL pointer to "", which Load treats as "set" and uses to overwrite
// a lower layer. This is the distinction that lets a user clear a config-set
// model with `--model ""`.
func TestBuildFlags_PresentButZero(t *testing.T) {
	cmd := newTestCmd(t)
	if err := cmd.ParseFlags([]string{"--model="}); err != nil {
		t.Fatalf("ParseFlags error: %v", err)
	}

	flags, err := buildFlags(cmd)
	if err != nil {
		t.Fatalf("buildFlags error: %v", err)
	}
	if flags.Flag.Model == nil {
		t.Fatal("Flag.Model == nil, want non-nil pointer to \"\" (present-but-zero)")
	}
	if *flags.Flag.Model != "" {
		t.Errorf("Flag.Model = %q, want \"\"", *flags.Flag.Model)
	}

	// And it overwrites a file-set model via Load: write a repo .stagehand.toml
	// with model="from-file", then Load with the flag layer → cfg.Model == "".
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.stagehand.toml", []byte("[defaults]\nmodel = \"from-file\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, _, err := config.Load(flags, dir)
	if err != nil {
		t.Fatalf("config.Load error: %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("cfg.Model = %q, want \"\" (present-but-zero flag must overwrite file model)", cfg.Model)
	}
}

// TestBuildFlags_EnvAbsent asserts an unset (or empty) STAGEHAND_MODEL does NOT
// set the env pointer (string scalars: empty env == "not set" per FR35; the
// present-but-zero override applies to CLI flags and env booleans, not to env
// string scalars).
func TestBuildFlags_EnvAbsent(t *testing.T) {
	// Explicitly empty — equivalent to absent for a string scalar.
	t.Setenv("STAGEHAND_MODEL", "")

	cmd := newTestCmd(t)
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("ParseFlags error: %v", err)
	}

	flags, err := buildFlags(cmd)
	if err != nil {
		t.Fatalf("buildFlags error: %v", err)
	}
	if flags.Env.Model != nil {
		t.Errorf("Env.Model = %v, want nil (empty/unset env must not set pointer)", flags.Env.Model)
	}
}

// TestBuildFlags_EnvTimeoutParsed asserts STAGEHAND_TIMEOUT is parsed to a
// time.Duration in the env layer, and that an unparseable value surfaces as an
// error (which runDefault maps to ExitError) rather than a silent zero.
func TestBuildFlags_EnvTimeoutParsed(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("STAGEHAND_TIMEOUT", "90s")
		cmd := newTestCmd(t)
		flags, err := buildFlags(cmd)
		if err != nil {
			t.Fatalf("buildFlags error: %v", err)
		}
		if flags.Env.Timeout == nil {
			t.Fatal("Env.Timeout == nil, want non-nil")
		}
		if want := 90 * 1000_000_000; int(*flags.Env.Timeout) != want { // 90s in ns
			t.Errorf("Env.Timeout = %v, want 90s", *flags.Env.Timeout)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("STAGEHAND_TIMEOUT", "not-a-duration")
		cmd := newTestCmd(t)
		_, err := buildFlags(cmd)
		if err == nil {
			t.Fatal("buildFlags err = nil, want error for unparseable STAGEHAND_TIMEOUT")
		}
		if !strings.Contains(err.Error(), "STAGEHAND_TIMEOUT") {
			t.Errorf("error %q does not mention STAGEHAND_TIMEOUT", err.Error())
		}
	})
}

// TestResolveAndCheckProvider_MissingFriendlyMsg asserts the friendly
// provider-missing message: a resolved provider (named but not on $PATH)
// yields exit Error (1) and a message naming the command and prompting the
// user to install the agent. This is the CLI's friendlier validation that runs
// BEFORE GenerateCommit (whose own message is less helpful).
func TestResolveAndCheckProvider_MissingFriendlyMsg(t *testing.T) {
	reg := provider.NewRegistry(nil, map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	cfg := config.Config{Provider: "definitely-not-an-agent-xyz"}

	name, code, msg := resolveAndCheckProvider(cfg, reg)
	if code != ui.ExitError {
		t.Errorf("code = %d, want %d (ExitError)", code, ui.ExitError)
	}
	if name != "" {
		t.Errorf("name = %q, want \"\" on failure", name)
	}
	for want := range map[string]bool{
		"definitely-not-an-agent-xyz": true,
		"not found":                   true,
		"Is the agent installed?":     true,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

// TestResolveAndCheckProvider_FirstDetected asserts cfg.Provider="" auto-resolves
// to the first detected provider in sorted reg.List() order. The fabricated
// providers use the test binary's own path (always executable) so detection is
// hermetic and does not depend on a particular agent being installed in the
// environment.
func TestResolveAndCheckProvider_FirstDetected(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable error: %v", err)
	}
	// Two detected providers; sorted order puts "aaa-..." first.
	reg := provider.NewRegistry(nil, map[string]provider.Manifest{
		"zzz-test-detected": {Command: exe},
		"aaa-test-detected": {Command: exe},
	})
	cfg := config.Config{Provider: ""}

	name, code, msg := resolveAndCheckProvider(cfg, reg)
	if code != ui.ExitSuccess {
		t.Fatalf("code = %d (msg=%q), want ExitSuccess", code, msg)
	}
	if name != "aaa-test-detected" {
		t.Errorf("name = %q, want %q (first in sorted List order)", name, "aaa-test-detected")
	}
}

// TestResolveAndCheckProvider_NoneDetected asserts that when no provider is
// configured and nothing is detected, the message says so (not the
// "command not found" path, which only applies to a named-but-missing provider).
func TestResolveAndCheckProvider_NoneDetected(t *testing.T) {
	reg := provider.NewRegistry(nil, map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	cfg := config.Config{Provider: ""}

	name, code, msg := resolveAndCheckProvider(cfg, reg)
	if code != ui.ExitError {
		t.Errorf("code = %d, want %d", code, ui.ExitError)
	}
	if name != "" {
		t.Errorf("name = %q, want \"\"", name)
	}
	if !strings.Contains(msg, "no provider") || !strings.Contains(msg, "PATH") {
		t.Errorf("message %q does not mention no provider / PATH", msg)
	}
}

// TestMapErrorToExitCode asserts the exact PRD §15.4 sentinel→exit-code mapping
// via a table. ErrHeadMoved and an arbitrary error both collapse to ExitError
// (1); timeout is NOT a separate branch (it collapses to ErrRescue→3 inside
// CommitStaged, so 124 is reserved/unused in v1).
func TestMapErrorToExitCode(t *testing.T) {
	arbitrary := errors.New("something exploded")
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ui.ExitSuccess},
		{"ErrNothingToCommit", stagehand.ErrNothingToCommit, ui.ExitNothingToCommit},
		{"ErrNothingStaged", ErrNothingStaged, ui.ExitNothingToCommit},
		{"ErrRescue", stagehand.ErrRescue, ui.ExitRescue},
		{"ErrHeadMoved", stagehand.ErrHeadMoved, ui.ExitError},
		{"arbitrary", arbitrary, ui.ExitError},
		{"wrapped-rescue", fmt.Errorf("outer: %w", stagehand.ErrRescue), ui.ExitRescue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapErrorToExitCode(tc.err); got != tc.want {
				t.Errorf("mapErrorToExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// TestBuildOptions_DryRunWired asserts the FR49 seam: buildOptions sets
// opts.DryRun from the flag and threads the resolved cfg provider/model/timeout.
func TestBuildOptions_DryRunWired(t *testing.T) {
	cfg := config.Config{Provider: "pi", Model: "glm-5.2", Timeout: 42}
	t.Run("dry-run", func(t *testing.T) {
		opts := buildOptions(cfg, true)
		if !opts.DryRun {
			t.Error("opts.DryRun = false, want true")
		}
		if opts.Provider != "pi" {
			t.Errorf("opts.Provider = %q, want %q", opts.Provider, "pi")
		}
		if opts.Model != "glm-5.2" {
			t.Errorf("opts.Model = %q, want %q", opts.Model, "glm-5.2")
		}
		if opts.Timeout != 42 {
			t.Errorf("opts.Timeout = %v, want 42", opts.Timeout)
		}
	})
	t.Run("commit", func(t *testing.T) {
		opts := buildOptions(cfg, false)
		if opts.DryRun {
			t.Error("opts.DryRun = true, want false")
		}
	})
}

// TestBuildOptions_VerboseWired asserts the FR50 seam: buildOptions threads the
// resolved cfg.Verbose (from --verbose/-v/STAGEHAND_VERBOSE via config.Load) into
// opts.Verbose, which GenerateCommit fans out to the shared *ui.Output driving
// the executor and the generate orchestrator. A sibling of DryRunWired so the
// FR49 dry-run assertions stay focused on the dry-run field.
func TestBuildOptions_VerboseWired(t *testing.T) {
	t.Run("verbose on", func(t *testing.T) {
		cfg := config.Config{Verbose: true}
		if opts := buildOptions(cfg, false); !opts.Verbose {
			t.Error("opts.Verbose = false, want true (must thread cfg.Verbose)")
		}
	})
	t.Run("verbose off", func(t *testing.T) {
		cfg := config.Config{Verbose: false}
		if opts := buildOptions(cfg, false); opts.Verbose {
			t.Error("opts.Verbose = true, want false (zero value must be a no-op)")
		}
	})
}

// TestVersionShortCircuit asserts --version short-circuits via cobra's Version
// field BEFORE Run is invoked: a command with Version set and args=["--version"]
// prints "stagehand version <v>" to stdout and exits 0 without calling Run.
// This is the mechanism main.go relies on (no manual --version flag).
func TestVersionShortCircuit(t *testing.T) {
	var runCalled bool
	cmd := &cobra.Command{
		Use:     "stagehand",
		Version: "9.9.9-test",
		Run: func(*cobra.Command, []string) {
			runCalled = true
		},
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if runCalled {
		t.Error("Run was invoked, want --version to short-circuit before Run")
	}
	out := stdout.String()
	if !strings.Contains(out, "stagehand version") {
		t.Errorf("stdout = %q, want it to contain \"stagehand version\"", out)
	}
	if !strings.Contains(out, "9.9.9-test") {
		t.Errorf("stdout = %q, want it to contain the version %q", out, "9.9.9-test")
	}
}

// TestPersistentFlags_Registered asserts the ten PRD §15.2 flags are present on
// the package-global rootCmd (registered via init()), with the -a/-v shorthands
// bound, and that --version is handled by the Version field (not a manual flag).
func TestPersistentFlags_Registered(t *testing.T) {
	for _, name := range []string{
		"provider", "model", "config", "timeout",
		"all", "no-auto-stage", "dry-run", "verbose", "no-color",
	} {
		if rootCmd.Flags().Lookup(name) == nil {
			t.Errorf("persistent flag --%s is not registered on rootCmd", name)
		}
	}
	// Shorthands.
	for _, sh := range []string{"a", "v"} {
		if f := rootCmd.Flags().ShorthandLookup(sh); f == nil {
			t.Errorf("shorthand -%s is not registered on rootCmd", sh)
		}
	}
	// Version is handled by the Version field, not a manual --version flag.
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version is empty; --version short-circuit relies on it")
	}
}

// TestRootCmdLongSet asserts rootCmd.Long is non-empty (the PRD §15.1 synopsis
// promised by Mode A docs is materialized in the command help).
func TestRootCmdLongSet(t *testing.T) {
	if strings.TrimSpace(rootCmd.Long) == "" {
		t.Error("rootCmd.Long is empty, want the PRD §15.1 multi-line synopsis")
	}
	for _, want := range []string{"--provider", "--dry-run", "stagehand"} {
		if !strings.Contains(rootCmd.Long, want) {
			t.Errorf("rootCmd.Long missing %q", want)
		}
	}
}
