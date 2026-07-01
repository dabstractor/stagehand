package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dustin/stagehand/internal/exitcode"
)

// ---------------------------------------------------------------------------
// Test helpers (copied from internal/config/load_test.go and internal/git/git_test.go
// — _test.go helpers are not importable across packages).
// ---------------------------------------------------------------------------

// initRepo creates a minimal git repo in dir for testing.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Set repo-local user identity so every subsequent git operation in this repo
	// (commit, config, etc.) works even without a global ~/.gitconfig.
	cfgCmd := exec.Command("git", "-C", dir, "config", "user.name", "Test")
	cfgCmd.Env = os.Environ()
	if out, err := cfgCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v\n%s", err, out)
	}
	emailCmd := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com")
	emailCmd.Env = os.Environ()
	if out, err := emailCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v\n%s", err, out)
	}
}

// setGitConfig writes a git config key=value in the repo at dir (repo-local).
func setGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "config", key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config %s=%s failed: %v\n%s", key, value, err, out)
	}
}

// writeConfigFile creates a file at dir/relPath with body. MkdirAll ensures parent dirs exist.
func writeConfigFile(t *testing.T, dir, relPath, body string) string {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("writeConfigFile MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatalf("writeConfigFile WriteFile: %v", err)
	}
	return full
}

// chdir changes CWD to dir and registers a cleanup to restore the original.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("chdir restore failed: %v", err)
		}
	})
}

// loadEnvSetup creates isolated temp dirs for global config + repo, and returns paths.
// Caller should use chdir(t, repo) to exercise the repo-local layer.
// Returns: home (for XDG/HOME isolation), repo (for git config), globalDir (for global TOML).
func loadEnvSetup(t *testing.T) (home, repo, globalDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home) // globalConfigPath will use XDG
	repo = t.TempDir()
	initRepo(t, repo) // initialize git repo for layer 4
	globalDir = filepath.Join(home, "stagehand")
	return home, repo, globalDir
}

// saveRootState captures the current rootCmd mutable state for restoration in t.Cleanup.
func saveRootState(t *testing.T) (_ []string, origOut, origErr io.Writer, origRunE func(*cobra.Command, []string) error) {
	t.Helper()
	return nil, rootCmd.OutOrStdout(), rootCmd.ErrOrStderr(), rootCmd.RunE
}

// restoreRootState restores rootCmd state from a previous saveRootState snapshot.
func restoreRootState(t *testing.T, _ []string, origOut, origErr io.Writer, origRunE func(*cobra.Command, []string) error) {
	t.Helper()
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(origOut)
	rootCmd.SetErr(origErr)
	rootCmd.RunE = origRunE
	loadedCfg = nil
	// Reset all changed local and persistent flags (pflag doesn't reset between Parses,
	// so a --version parse leaves the flag true, short-circuiting subsequent Executes).
	resetFlags(rootCmd.Flags())
	resetFlags(rootCmd.PersistentFlags())
}

// resetFlags resets all changed flags in fs back to their DefValue and clears Changed.
func resetFlags(fs *pflag.FlagSet) {
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			f.Changed = false
			f.Value.Set(f.DefValue)
		}
	})
}

// ---------------------------------------------------------------------------
// TestFlags_RegisteredAndDefaults — flag surface + shorthands + zero defaults
// ---------------------------------------------------------------------------

func TestFlags_RegisteredAndDefaults(t *testing.T) {
	pf := rootCmd.PersistentFlags()

	// All §15.2 flags must be registered
	requiredFlags := []struct {
		name      string
		shorthand string
		defValue  string // expected DefValue
	}{
		{"provider", "", ""},
		{"model", "", ""},
		{"config", "", ""},
		{"timeout", "", ""},
		{"verbose", "v", "false"},
		{"no-color", "", "false"},
		{"all", "a", "false"},
		{"no-auto-stage", "", "false"},
		{"dry-run", "", "false"},
		// §15.2 decompose/per-role flags (P4.M1.T1.S1)
		{"commits", "", "0"},
		{"single", "", "false"},
		{"no-decompose", "", "false"},
		{"max-commits", "", "12"},
		{"planner-provider", "", ""},
		{"planner-model", "", ""},
		{"stager-provider", "", ""},
		{"stager-model", "", ""},
		{"arbiter-provider", "", ""},
		{"arbiter-model", "", ""},
	}
	for _, f := range requiredFlags {
		t.Run("flag_"+f.name, func(t *testing.T) {
			flag := pf.Lookup(f.name)
			if flag == nil {
				t.Fatalf("persistent flag %q not found", f.name)
			}
			if f.shorthand != "" && flag.Shorthand != f.shorthand {
				t.Errorf("flag %q shorthand = %q, want %q", f.name, flag.Shorthand, f.shorthand)
			}
			if flag.DefValue != f.defValue {
				t.Errorf("flag %q DefValue = %q, want %q", f.name, flag.DefValue, f.defValue)
			}
		})
	}

	// --help/-h is cobra's built-in (lazily initialized on first Execute)
	// Asserting -h before Execute would fail; --help works correctly at runtime.
}

// ---------------------------------------------------------------------------
// TestVersion_PrintsAndSkipsConfig — --version short-circuits before PersistentPreRunE
// ---------------------------------------------------------------------------

func TestVersion_PrintsAndSkipsConfig(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	Version = "test-v"
	rootCmd.Version = "test-v"
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf) // suppress any error output
	rootCmd.SetArgs([]string{"--version"})

	err := Execute(context.Background())

	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("test-v")) {
		t.Errorf("version output = %q, want to contain 'test-v'", buf.String())
	}
	if loadedCfg != nil {
		t.Error("loadedCfg should be nil (config NOT loaded for --version)")
	}
}

// ---------------------------------------------------------------------------
// TestRoot_LoadsConfigAndRunsStub — PersistentPreRunE loads config, RunE fires
// ---------------------------------------------------------------------------

func TestRoot_LoadsConfigAndRunsStub(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Stub: just return nil so we can assert Config()
		return nil
	}
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (stub RunE)", err)
	}
	cfg := Config()
	if cfg == nil {
		t.Fatal("Config() returned nil, want non-nil")
	}
	// FR-B3 (P1.M4.T4.S1): the bootstrap fires on first run with no config file,
	// so Provider is now the bootstrap target ("pi" or auto-detected) instead of "".
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout=%v, want 120s (default)", cfg.Timeout)
	}
	if cfg.Provider == "" {
		t.Errorf("Provider=%q, want non-empty (FR-B3 bootstrap writes provider)", cfg.Provider)
	}
}

// ---------------------------------------------------------------------------
// TestRoot_FlagOverridesEnvOverridesGit — full Layer 7 > Layer 5 > Layer 4
// ---------------------------------------------------------------------------

func TestRoot_FlagOverridesEnvOverridesGit(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	setGitConfig(t, repo, "stagehand.provider", "git-p")
	t.Setenv("STAGEHAND_PROVIDER", "env-p")

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	// Sub-case A: CLI flag wins over env > git
	t.Run("cli_wins", func(t *testing.T) {
		loadedCfg = nil
		rootCmd.SetArgs([]string{"--provider", "cli-p"})
		_ = Execute(context.Background()) // RunE may return an error (nothing-to-commit on clean repo)
		cfg := Config()
		if cfg == nil || cfg.Provider != "cli-p" {
			t.Errorf("Provider=%v, want cli-p (CLI > env > git)", cfg)
		}
	})

	// Sub-case B: env wins over git (no CLI flag)
	t.Run("env_wins", func(t *testing.T) {
		loadedCfg = nil
		// Reset provider flag Changed state (pflag doesn't reset between Parse calls)
		if f := rootCmd.PersistentFlags().Lookup("provider"); f != nil {
			f.Changed = false
		}
		flagProvider = "" // reset bound var
		rootCmd.SetArgs([]string{})
		_ = Execute(context.Background()) // RunE may return an error (nothing-to-commit on clean repo)
		cfg := Config()
		if cfg == nil || cfg.Provider != "env-p" {
			t.Errorf("Provider=%v, want env-p (env > git)", cfg)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRoot_ConfigLoadErrorMapsToExit1 — bad config → exitcode.Error
// ---------------------------------------------------------------------------

func TestRoot_ConfigLoadErrorMapsToExit1(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	// Write malformed TOML
	writeConfigFile(t, globalDir, "config.toml", "bad {toml")

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (bad config)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

// ---------------------------------------------------------------------------
// TestRoot_SilenceErrors — cobra prints nothing on error
// ---------------------------------------------------------------------------

func TestRoot_SilenceErrors(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--bogus-flag"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown flag)")
	}
	// SilenceErrors+SilenceUsage → cobra printed nothing
	if outBuf.Len() > 0 {
		t.Errorf("cobra printed to stdout: %q (should be silent)", outBuf.String())
	}
	if errBuf.Len() > 0 {
		t.Errorf("cobra printed to stderr: %q (should be silent)", errBuf.String())
	}
}

// ---------------------------------------------------------------------------
// TestRoot_TimeoutIsString — --timeout DefValue is "" (not a duration)
// ---------------------------------------------------------------------------

func TestRoot_TimeoutIsString(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("timeout")
	if f == nil {
		t.Fatal("timeout flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("timeout DefValue = %q, want empty string", f.DefValue)
	}
}
