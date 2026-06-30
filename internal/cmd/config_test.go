package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupNoRepo creates isolated temp dirs for HOME/XDG and a plain (non-git) temp dir,
// then chdir's into it. Returns home, plainDir, globalDir. Use for tests proving config init/path
// work OUTSIDE a git repo (shouldSkipConfigLoad returns true for init/path).
func setupNoRepo(t *testing.T) (home, plainDir, globalDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	plainDir = t.TempDir()
	chdir(t, plainDir)
	globalDir = filepath.Join(home, "stagehand")
	return home, plainDir, globalDir
}

// ---------------------------------------------------------------------------
// config path tests
// ---------------------------------------------------------------------------

func TestConfigPath_PrintsGlobalPath(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := strings.TrimSpace(out.String())
	expected := config.GlobalConfigPath()
	if got != expected {
		t.Errorf("config path output = %q, want %q", got, expected)
	}
	// Must end with stagehand/config.toml
	if !strings.HasSuffix(got, filepath.Join("stagehand", "config.toml")) {
		t.Errorf("config path output = %q, want to end with stagehand/config.toml", got)
	}
}

func TestConfigPath_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path", "x"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (extra args)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

func TestConfigPath_WorksOutsideGitRepo(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (works outside git repo)", err)
	}
	if out.Len() == 0 {
		t.Error("expected output on stdout")
	}
}

// ---------------------------------------------------------------------------
// config init tests
// ---------------------------------------------------------------------------

func TestConfigInit_WritesTemplate(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// stdout should contain the confirmation
	if !strings.Contains(out.String(), "Wrote example config") {
		t.Errorf("stdout = %q, want to contain 'Wrote example config'", out.String())
	}

	// The file should exist at the global config path
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read written config at %s: %v", path, err)
	}

	// Content should match the template exactly
	got := string(data)
	if got != exampleConfigTemplate {
		t.Errorf("written config does not match template (length %d vs %d)", len(got), len(exampleConfigTemplate))
	}

	// Parent dir should exist
	if _, err := os.Stat(globalDir); err != nil {
		t.Errorf("parent dir %s should exist: %v", globalDir, err)
	}
}

func TestConfigInit_TemplateIsInert(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read config at %s: %v", path, err)
	}

	content := string(data)

	// NO line should be an un-commented TOML table header: ^[[a-z]
	uncommentedSection := regexp.MustCompile(`^[[a-z]`)
	for i, line := range strings.Split(content, "\n") {
		if uncommentedSection.MatchString(line) {
			t.Errorf("line %d is an uncommented TOML header: %q (template must be inert)", i+1, line)
		}
	}

	// But the commented headers MUST be present (as guidance)
	for _, section := range []string{"[defaults]", "[generation]", "[provider.pi]", "[provider.myagent]"} {
		if !strings.Contains(content, section) {
			t.Errorf("template missing commented section %q", section)
		}
	}

	// Env-var and git-key docs must be present
	if !strings.Contains(content, "STAGEHAND_PROVIDER") {
		t.Error("template missing STAGEHAND_PROVIDER env-var doc")
	}
	if !strings.Contains(content, "stagehand.provider") {
		t.Error("template missing stagehand.provider git-key doc")
	}
}

func TestConfigInit_RefusesOverwrite(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	// Pre-create the config file with some content
	writeConfigFile(t, globalDir, "config.toml", `provider = "mine"
`)
	prePath := filepath.Join(globalDir, "config.toml")
	preContent, _ := os.ReadFile(prePath)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (file already exists)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error message %q should contain 'already exists'", err.Error())
	}

	// File must be UNCHANGED
	afterContent, _ := os.ReadFile(prePath)
	if string(afterContent) != string(preContent) {
		t.Error("config file was modified (should be unchanged — non-destructive)")
	}
}

func TestConfigInit_MkdirAllParent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	// The parent dir (<home>/stagehand) should NOT exist yet (loadEnvSetup doesn't create it)
	if _, err := os.Stat(globalDir); err == nil {
		t.Fatalf("parent dir %s already exists (test setup issue)", globalDir)
	}

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// Parent dir should now exist
	if _, err := os.Stat(globalDir); err != nil {
		t.Errorf("parent dir %s should exist after init: %v", globalDir, err)
	}
	// File should exist
	path := config.GlobalConfigPath()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file %s should exist after init: %v", path, err)
	}
}

func TestConfigInit_WorksOutsideGitRepo(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)

	// config init should succeed outside a git repo
	rootCmd.SetArgs([]string{"config", "init"})
	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (works outside git repo)", err)
	}
	if !strings.Contains(out.String(), "Wrote example config") {
		t.Errorf("stdout = %q, want to contain 'Wrote example config'", out.String())
	}

	// A second init should fail (refuse overwrite)
	out.Reset()
	rootCmd.SetArgs([]string{"config", "init"})
	err = Execute(context.Background())
	if err == nil {
		t.Fatal("second Execute err=nil, want error (refuse overwrite)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

func TestConfigInit_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "x"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (extra args)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

// ---------------------------------------------------------------------------
// config group (no subcommand → help)
// ---------------------------------------------------------------------------

func TestConfigGroup_NoSubcommandPrintsHelp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (prints help)", err)
	}

	got := buf.String()
	if !strings.Contains(got, "init") {
		t.Error(`help output missing "init" subcommand`)
	}
	if !strings.Contains(got, "path") {
		t.Error(`help output missing "path" subcommand`)
	}
}
