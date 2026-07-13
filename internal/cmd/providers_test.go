package cmd

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/exitcode"
)

// setupRepo creates isolated temp dirs for HOME/XDG and a fresh git repo, then chdir's into it.
// Returns the repo dir. This is the common setup for every providers test (list/show need config.Load,
// which requires a git repo for Layer 4).
func setupRepo(t *testing.T) string {
	t.Helper()
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	return repo
}

// ---------------------------------------------------------------------------
// providers list tests
// ---------------------------------------------------------------------------

func TestProvidersList_Builtins(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	names := []string{"claude", "codex", "cursor", "opencode", "pi"}
	for _, name := range names {
		if !strings.Contains(got, name) {
			t.Errorf("list output missing provider %q", name)
		}
	}
	// Assert ascending sort: index of each name is < the next.
	prevIdx := -1
	for _, name := range names {
		idx := strings.Index(got, name)
		if idx < 0 {
			t.Fatalf("provider %q not found", name)
		}
		if idx <= prevIdx {
			t.Errorf("provider %q at index %d is not after previous at %d (ascending order violation)", name, idx, prevIdx)
		}
		prevIdx = idx
	}
}

func TestProvidersList_DetectedGlyphs(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	repo := setupRepo(t)
	// Write config with a provider whose command IS on PATH ("go" is guaranteed in go test)
	// and one whose command is NOT on PATH.
	writeConfigFile(t, repo, ".stagecoach.toml", `
[provider.realbin]
command = "go"

[provider.fakebin]
command = "no-such-binary-xyz"
`)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// realbin should be detected (go is on PATH)
	if !strings.Contains(got, "✓") {
		t.Error(`list output missing "✓" for realbin (go is on PATH)`)
	}
	// fakebin should NOT be detected
	if !strings.Contains(got, "✗") {
		t.Error(`list output missing "✗" for fakebin`)
	}
}

func TestProvidersList_DefaultMarker_Explicit(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	t.Setenv("STAGECOACH_PROVIDER", "pi")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// The pi row should have "(default)"
	piIdx := strings.Index(got, "pi")
	if piIdx < 0 {
		t.Fatal(`list output missing "pi"`)
	}
	// Find "(default)" occurrences — there should be exactly one
	count := strings.Count(got, "(default)")
	if count != 1 {
		t.Errorf(`expected exactly 1 "(default)" marker, got %d`, count)
	}
	// Verify the "(default)" is near the pi line
	defaultIdx := strings.Index(got, "(default)")
	// They should be close (same line in tabwriter output)
	if defaultIdx < 0 || defaultIdx < piIdx-10 || defaultIdx > piIdx+50 {
		t.Errorf(`"(default)" at index %d is not near "pi" at index %d`, defaultIdx, piIdx)
	}
}

func TestProvidersList_DefaultMarker_Auto(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	// STAGECOACH_PROVIDER is NOT set; default is auto-detected.
	// Assert: at most ONE "(default)" marker (could be 0 if no built-in is on PATH).
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	count := strings.Count(got, "(default)")
	if count > 1 {
		t.Errorf(`expected at most 1 "(default)" marker, got %d`, count)
	}
}

func TestProvidersList_OverrideAppears(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	repo := setupRepo(t)
	writeConfigFile(t, repo, ".stagecoach.toml", `
[provider.myagent]
command = "/opt/agent"
`)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "myagent") {
		t.Error(`list output missing "myagent" — user-defined provider should appear`)
	}
}

// ---------------------------------------------------------------------------
// providers show tests
// ---------------------------------------------------------------------------

func TestProvidersShow_BuiltInTOML(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show", "pi"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	substrings := []string{
		"name = 'pi'",
		"command = 'pi'",
		"default_model = ''",
		"output = 'raw'",
		"strip_code_fence = true",
	}
	for _, sub := range substrings {
		if !strings.Contains(got, sub) {
			t.Errorf("show pi output missing %q", sub)
		}
	}
}

func TestProvidersShow_OverrideMerged(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	repo := setupRepo(t)
	writeConfigFile(t, repo, ".stagecoach.toml", `
[provider.pi]
default_model = "glm-5.2"
`)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show", "pi"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// The override should be reflected
	if !strings.Contains(got, "default_model = 'glm-5.2'") {
		t.Error(`show pi output missing overridden "default_model = 'glm-5.2'"`)
	}
	// Untouched built-in field should survive
	if !strings.Contains(got, "command = 'pi'") {
		t.Error(`show pi output missing "command = 'pi'" (built-in field should survive merge)`)
	}
}

func TestProvidersShow_NewProviderTOML(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	repo := setupRepo(t)
	writeConfigFile(t, repo, ".stagecoach.toml", `
[provider.myagent]
command = "/opt/agent"
prompt_delivery = "stdin"
`)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show", "myagent"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "name = 'myagent'") {
		t.Error(`show myagent output missing "name = 'myagent'"`)
	}
	if !strings.Contains(got, "command = '/opt/agent'") {
		t.Error(`show myagent output missing "command = '/opt/agent'"`)
	}
}

func TestProvidersShow_UnknownExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show", "ghost"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown provider)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error message %q should contain 'ghost'", err.Error())
	}
}

func TestProvidersShow_MissingArgExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (missing arg)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

func TestProvidersShow_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers", "show", "a", "b"})

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
// providers group (no subcommand → help)
// ---------------------------------------------------------------------------

func TestProvidersGroup_NoSubcommandPrintsHelp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"providers"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (prints help)", err)
	}

	got := buf.String()
	if !strings.Contains(got, "list") {
		t.Error(`help output missing "list" subcommand`)
	}
	if !strings.Contains(got, "show") {
		t.Error(`help output missing "show" subcommand`)
	}
	if strings.Contains(got, "Subcommands:") {
		t.Error(`help output must NOT contain a manual "Subcommands:" block (FR-B6)`)
	}
	if !strings.Contains(got, "Available Commands:") {
		t.Error(`help output missing cobra "Available Commands:" (FR-B6 single source)`)
	}
}
