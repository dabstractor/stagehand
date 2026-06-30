package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// ---------------------------------------------------------------------------
// Helpers (load_test.go's OWN — reuses initRepo/setGitConfig from git_test.go)
// ---------------------------------------------------------------------------

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

// newFlagSet returns a fresh *pflag.FlagSet with the 5 Config-backed flags pre-registered:
// provider/model as String "", timeout as String "", verbose as Bool false, no-color as Bool false.
func newFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("provider", "", "")
	fs.String("model", "", "")
	fs.String("timeout", "", "")
	fs.Bool("verbose", false, "")
	fs.Bool("no-color", false, "")
	return fs
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

// ---------------------------------------------------------------------------
// parseTimeout tests
// ---------------------------------------------------------------------------

func TestParseTimeout_DurationAndSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"120s", 120 * time.Second},
		{"120", 120 * time.Second},
		{"2m", 2 * time.Minute},
		{"90", 90 * time.Second},
		{"1h30m", 90 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseTimeout(tc.input)
			if err != nil {
				t.Fatalf("parseTimeout(%q) err=%v, want nil", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseTimeout(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseTimeout_Invalid(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"abc"},
		{""},
		{"12.3.4"}, // not a valid duration or integer
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			_, err := parseTimeout(tc.input)
			if err == nil {
				t.Fatalf("parseTimeout(%q) err=nil, want error", tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// loadEnv tests
// ---------------------------------------------------------------------------

func TestLoadEnv_StringsTimeoutBools(t *testing.T) {
	cfg := Defaults()
	t.Setenv("STAGEHAND_PROVIDER", "pi")
	t.Setenv("STAGEHAND_MODEL", "glm-5.2")
	t.Setenv("STAGEHAND_TIMEOUT", "60s")
	t.Setenv("STAGEHAND_VERBOSE", "true")
	t.Setenv("STAGEHAND_NO_COLOR", "true")

	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi", cfg.Provider)
	}
	if cfg.Model != "glm-5.2" {
		t.Errorf("Model=%q want glm-5.2", cfg.Model)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout=%v want 60s", cfg.Timeout)
	}
	if !cfg.Verbose {
		t.Errorf("Verbose=false want true")
	}
	if !cfg.NoColor {
		t.Errorf("NoColor=false want true")
	}
}

func TestLoadEnv_BoolFalseEscape(t *testing.T) {
	cfg := Config{Verbose: true, NoColor: true} // start with true
	t.Setenv("STAGEHAND_VERBOSE", "false")
	t.Setenv("STAGEHAND_NO_COLOR", "false")

	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if cfg.Verbose {
		t.Errorf("Verbose=true want false (DIRECT set escape hatch)")
	}
	if cfg.NoColor {
		t.Errorf("NoColor=true want false (DIRECT set escape hatch)")
	}
}

func TestLoadEnv_NoColorResolvable(t *testing.T) {
	cfg := Defaults()
	// NoColor absent → unchanged
	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if cfg.NoColor {
		t.Errorf("NoColor=true want false (no env set)")
	}

	// NoColor present → set
	t.Setenv("STAGEHAND_NO_COLOR", "true")
	cfg2 := Defaults()
	if err := loadEnv(&cfg2); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if !cfg2.NoColor {
		t.Errorf("NoColor=false want true (STAGEHAND_NO_COLOR=true)")
	}
}

func TestLoadEnv_BadBoolErrors(t *testing.T) {
	cfg := Defaults()
	t.Setenv("STAGEHAND_VERBOSE", "notabool")

	err := loadEnv(&cfg)
	if err == nil {
		t.Fatal("loadEnv err=nil, want error for bad bool")
	}
	if !strings.Contains(err.Error(), "STAGEHAND_VERBOSE") {
		t.Errorf("err=%v, want it to contain 'STAGEHAND_VERBOSE'", err)
	}
}

func TestLoadEnv_BadTimeoutErrors(t *testing.T) {
	cfg := Defaults()
	t.Setenv("STAGEHAND_TIMEOUT", "abc")

	err := loadEnv(&cfg)
	if err == nil {
		t.Fatal("loadEnv err=nil, want error for bad timeout")
	}
	if !strings.Contains(err.Error(), "STAGEHAND_TIMEOUT") {
		t.Errorf("err=%v, want it to contain 'STAGEHAND_TIMEOUT'", err)
	}
}

func TestLoadEnv_EmptyStringsSkipped(t *testing.T) {
	cfg := Config{Provider: "original", Model: "original"}
	t.Setenv("STAGEHAND_PROVIDER", "")
	t.Setenv("STAGEHAND_MODEL", "")

	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if cfg.Provider != "original" {
		t.Errorf("Provider=%q want original (empty env skipped)", cfg.Provider)
	}
	if cfg.Model != "original" {
		t.Errorf("Model=%q want original (empty env skipped)", cfg.Model)
	}
}

// ---------------------------------------------------------------------------
// loadFlags tests
// ---------------------------------------------------------------------------

func TestLoadFlags_ChangedOnly(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t)
	if err := fs.Set("provider", "gemini"); err != nil {
		t.Fatal(err)
	}

	loadFlags(&cfg, fs)

	if cfg.Provider != "gemini" {
		t.Errorf("Provider=%q want gemini", cfg.Provider)
	}
	// model was NOT Changed — must stay at default ("")
	if cfg.Model != "" {
		t.Errorf("Model=%q want \"\" (not Changed, should not override)", cfg.Model)
	}
}

func TestLoadFlags_BoolDirect(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t)
	if err := fs.Set("no-color", "true"); err != nil {
		t.Fatal(err)
	}

	loadFlags(&cfg, fs)

	if !cfg.NoColor {
		t.Errorf("NoColor=false want true (--no-color set)")
	}
}

func TestLoadFlags_NoneChanged(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t) // nothing Set

	loadFlags(&cfg, fs)

	// cfg should equal Defaults()
	d := Defaults()
	if cfg.Provider != d.Provider || cfg.Model != d.Model ||
		cfg.Timeout != d.Timeout || cfg.Verbose != d.Verbose || cfg.NoColor != d.NoColor {
		t.Errorf("cfg=%+v, want Defaults() (no flags changed)", cfg)
	}
}

func TestLoadFlags_TimeoutString(t *testing.T) {
	cfg := Config{}
	fs := newFlagSet(t)
	if err := fs.Set("timeout", "60s"); err != nil {
		t.Fatal(err)
	}

	loadFlags(&cfg, fs)

	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout=%v want 60s", cfg.Timeout)
	}
}

func TestLoadFlags_TimeoutInteger(t *testing.T) {
	cfg := Config{}
	fs := newFlagSet(t)
	if err := fs.Set("timeout", "90"); err != nil {
		t.Fatal(err)
	}

	loadFlags(&cfg, fs)

	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout=%v want 90s", cfg.Timeout)
	}
}

func TestLoadFlags_VerboseFalse(t *testing.T) {
	cfg := Config{Verbose: true}
	fs := newFlagSet(t)
	if err := fs.Set("verbose", "false"); err != nil {
		t.Fatal(err)
	}

	loadFlags(&cfg, fs)

	if cfg.Verbose {
		t.Errorf("Verbose=true want false (DIRECT set via flag)")
	}
}

// ---------------------------------------------------------------------------
// Load — precedence tests
// ---------------------------------------------------------------------------

func TestLoad_DefaultsOnly(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	d := Defaults()
	if cfg.Provider != d.Provider || cfg.Model != d.Model ||
		cfg.Timeout != d.Timeout || cfg.Verbose != d.Verbose || cfg.NoColor != d.NoColor {
		t.Errorf("Load defaults-only: got %+v, want Defaults()", cfg)
	}
}

func TestLoad_GlobalFileOverridesDefaults(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\n")

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi (global file)", cfg.Provider)
	}
	// Other fields should be Defaults()
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout=%v want 120s (default)", cfg.Timeout)
	}
}

func TestLoad_RepoFileOverridesGlobal(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\n")
	// .stagehand.toml in repo dir (CWD)
	writeConfigFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")

	// Redirect notice output so it doesn't pollute test output
	origNoticeOut := noticeOut
	noticeOut = &strings.Builder{}
	defer func() { noticeOut = origNoticeOut }()

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("Provider=%q want claude (repo file > global file)", cfg.Provider)
	}
}

func TestLoad_GitOverridesRepoFile(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\n")
	writeConfigFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")
	setGitConfig(t, repo, "stagehand.provider", "gemini")

	// Redirect notice
	origNoticeOut := noticeOut
	noticeOut = &strings.Builder{}
	defer func() { noticeOut = origNoticeOut }()

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "gemini" {
		t.Errorf("Provider=%q want gemini (git > repo file)", cfg.Provider)
	}
}

func TestLoad_EnvOverridesGit(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	setGitConfig(t, repo, "stagehand.provider", "gemini")
	t.Setenv("STAGEHAND_PROVIDER", "pi")

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi (env > git)", cfg.Provider)
	}
}

func TestLoad_CLIOverridesEnv(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGEHAND_PROVIDER", "gemini")
	fs := newFlagSet(t)
	if err := fs.Set("provider", "claude"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("Provider=%q want claude (CLI > env)", cfg.Provider)
	}
	// Model is unset everywhere — should remain ""
	if cfg.Model != "" {
		t.Errorf("Model=%q want \"\" (nobody set it)", cfg.Model)
	}
}

func TestLoad_UnsetCLIFlagDoesNotOverride(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGEHAND_PROVIDER", "pi")
	fs := newFlagSet(t) // provider NOT Set — Changed("provider")==false

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi (unset CLI flag should not override env)", cfg.Provider)
	}
}

func TestLoad_EnvBoolFalseEscape(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	// Global file sets verbose=true (overlay can set true — non-zero)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nverbose = true\n")
	t.Setenv("STAGEHAND_VERBOSE", "false") // env DIRECT set -> Verbose=false

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Verbose {
		t.Errorf("Verbose=true want false (env DIRECT set must override file's true)")
	}
}

func TestLoad_NoColorFromCLI(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	fs := newFlagSet(t)
	if err := fs.Set("no-color", "true"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if !cfg.NoColor {
		t.Errorf("NoColor=false want true (--no-color set; toml:\"-\" field)")
	}
}

func TestLoad_CLIBoolFalseEscape(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nverbose = true\n")
	fs := newFlagSet(t)
	if err := fs.Set("verbose", "false"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Verbose {
		t.Errorf("Verbose=true want false (CLI DIRECT set overrides file true)")
	}
}

// ---------------------------------------------------------------------------
// Load — path resolution tests
// ---------------------------------------------------------------------------

func TestLoad_ConfigPathOverride(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	// Write a custom config file at an arbitrary path
	customPath := writeConfigFile(t, t.TempDir(), "custom.toml", "[defaults]\nprovider = \"custom\"\n")

	cfg, err := Load(context.Background(), LoadOpts{ConfigPathOverride: customPath, RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "custom" {
		t.Errorf("Provider=%q want custom (ConfigPathOverride)", cfg.Provider)
	}
}

func TestLoad_STAGEHAND_CONFIG_EnvPath(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)

	// Global discovery: provider=A
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"discovery\"\n")

	// STAGEHAND_CONFIG points to a different file: provider=B
	envConfig := writeConfigFile(t, t.TempDir(), "env-config.toml", "[defaults]\nprovider = \"envpath\"\n")
	t.Setenv("STAGEHAND_CONFIG", envConfig)

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "envpath" {
		t.Errorf("Provider=%q want envpath (STAGEHAND_CONFIG beats discovery)", cfg.Provider)
	}

	// ConfigPathOverride beats STAGEHAND_CONFIG
	cliConfig := writeConfigFile(t, t.TempDir(), "cli-config.toml", "[defaults]\nprovider = \"clipath\"\n")
	cfg2, err := Load(context.Background(), LoadOpts{ConfigPathOverride: cliConfig, RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg2.Provider != "clipath" {
		t.Errorf("Provider=%q want clipath (ConfigPathOverride beats STAGEHAND_CONFIG)", cfg2.Provider)
	}
}

// ---------------------------------------------------------------------------
// Load — explicit-vs-discovery path tests (S1 bugfix)
// ---------------------------------------------------------------------------

func TestLoad_ConfigPathOverride_MissingFileFails(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	missing := filepath.Join(t.TempDir(), "does-not-exist.toml")
	_, err := Load(context.Background(), LoadOpts{ConfigPathOverride: missing, RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for missing ConfigPathOverride file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("err=%v, want it to contain 'config file not found'", err)
	}
	if !strings.Contains(err.Error(), "does-not-exist.toml") {
		t.Errorf("err=%v, want it to contain the missing file path", err)
	}
}

func TestLoad_STAGEHAND_CONFIG_EnvPath_MissingFileFails(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	missing := filepath.Join(t.TempDir(), "nope.toml")
	t.Setenv("STAGEHAND_CONFIG", missing)
	_, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for missing STAGEHAND_CONFIG file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("err=%v, want it to contain 'config file not found'", err)
	}
	if !strings.Contains(err.Error(), "nope.toml") {
		t.Errorf("err=%v, want it to contain the missing file path", err)
	}
}

func TestLoad_DiscoveryMissingFileOK(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	// No --config, no STAGEHAND_CONFIG, no global file written to globalDir.
	// Discovery should tolerate absence (contract preserved).
	os.Unsetenv("STAGEHAND_CONFIG")

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v, want nil (discovery tolerates absent global file)", err)
	}
	if cfg == nil {
		t.Fatal("Load cfg=nil, want non-nil (Defaults() still returned)")
	}
}

func TestLoad_ConfigPathOverride_MissingBeatsEnv(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	// Both ConfigPathOverride (missing) and STAGEHAND_CONFIG (set but irrelevant).
	// ConfigPathOverride wins precedence; the resolved path is the missing override → error.
	missing := filepath.Join(t.TempDir(), "override-missing.toml")
	t.Setenv("STAGEHAND_CONFIG", filepath.Join(t.TempDir(), "env-set.toml"))

	_, err := Load(context.Background(), LoadOpts{ConfigPathOverride: missing, RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for missing ConfigPathOverride (override beats env)")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("err=%v, want it to contain 'config file not found'", err)
	}
	if !strings.Contains(err.Error(), "override-missing.toml") {
		t.Errorf("err=%v, want it to contain the override missing file path", err)
	}
}

// ---------------------------------------------------------------------------
// Load — error propagation tests
// ---------------------------------------------------------------------------

func TestLoad_BadRepoLocalFile(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	// Write invalid TOML to .stagehand.toml
	if err := os.WriteFile(filepath.Join(repo, ".stagehand.toml"), []byte("this is [not valid {toml"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for bad repo-local TOML")
	}
	if !strings.Contains(err.Error(), "repo config") {
		t.Errorf("err=%v, want it to contain 'repo config'", err)
	}
}

func TestLoad_BadGlobalFileErrors(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	// Write malformed TOML
	writeConfigFile(t, globalDir, "config.toml", "this is [not valid {toml\n")

	_, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for bad global file")
	}
	if !strings.Contains(err.Error(), "global config") {
		t.Errorf("err=%v, want it to contain 'global config'", err)
	}
}

func TestLoad_BadEnvBoolErrors(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGEHAND_VERBOSE", "notabool")

	_, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for bad env bool")
	}
	if !strings.Contains(err.Error(), "env config") {
		t.Errorf("err=%v, want it to contain 'env config'", err)
	}
	if !strings.Contains(err.Error(), "STAGEHAND_VERBOSE") {
		t.Errorf("err=%v, want it to contain 'STAGEHAND_VERBOSE'", err)
	}
}

func TestLoad_GitConfigErrorPropagates(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	// Set a non-integer timeout in git config
	setGitConfig(t, repo, "stagehand.timeout", "notanumber")

	_, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err == nil {
		t.Fatal("Load err=nil, want error for bad git timeout")
	}
	if !strings.Contains(err.Error(), "git config") {
		t.Errorf("err=%v, want it to contain 'git config'", err)
	}
}

func TestLoad_NilFlagsSkipped(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGEHAND_PROVIDER", "pi")

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: nil})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi (nil Flags should not panic, env still applies)", cfg.Provider)
	}
}

// ---------------------------------------------------------------------------
// Load — ctx cancellation
// ---------------------------------------------------------------------------

func TestLoad_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := Load(ctx, LoadOpts{})
	if err == nil {
		t.Fatal("Load err=nil, want error for cancelled context")
	}
	if !strings.Contains(err.Error(), "config load") {
		t.Errorf("err=%v, want it to contain 'config load'", err)
	}
}

// ---------------------------------------------------------------------------
// Load — full precedence matrix (one field across all layers)
// ---------------------------------------------------------------------------

func TestLoad_FullPrecedenceMatrix(t *testing.T) {
	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)

	// Layer 2: global file sets provider=pi
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\ntimeout = \"60s\"\n")

	// Layer 3: repo file overrides provider
	writeConfigFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")

	// Redirect notice
	origNoticeOut := noticeOut
	noticeOut = &strings.Builder{}
	defer func() { noticeOut = origNoticeOut }()

	// Layer 4: git overrides provider
	setGitConfig(t, repo, "stagehand.provider", "gemini")
	setGitConfig(t, repo, "stagehand.model", "git-model")

	// Layer 5: env overrides provider + model
	t.Setenv("STAGEHAND_PROVIDER", "env-pi")
	t.Setenv("STAGEHAND_MODEL", "env-model")

	// Layer 7: CLI overrides provider
	fs := newFlagSet(t)
	if err := fs.Set("provider", "cli-claude"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}

	// CLI flag wins for provider
	if cfg.Provider != "cli-claude" {
		t.Errorf("Provider=%q want cli-claude (CLI > env > git > repo > global > default)", cfg.Provider)
	}
	// Env wins for model (no CLI model set)
	if cfg.Model != "env-model" {
		t.Errorf("Model=%q want env-model (env > git > default)", cfg.Model)
	}
	// Git sets model only at layer 4, env overwrites at layer 5
	// Timeout comes from global file (no env/CLI override)
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout=%v want 60s (from global file)", cfg.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Load — timeout via env (integer form)
// ---------------------------------------------------------------------------

func TestLoad_TimeoutViaEnvInteger(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGEHAND_TIMEOUT", "45") // bare integer

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout=%v want 45s (from STAGEHAND_TIMEOUT=45)", cfg.Timeout)
	}
}
