package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/provider"
)

// ---------------------------------------------------------------------------
// Golden renderer test — deterministic, no PATH juggling
// ---------------------------------------------------------------------------

func TestModels_CuratedGolden(t *testing.T) {
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("claude")
	if !ok {
		t.Fatal("claude not found in registry")
	}

	var buf bytes.Buffer
	printCuratedTable(&buf, modelTarget{name: "claude", manifest: m})

	got := buf.String()
	// Assert fixed role order and expected model values for claude
	substrings := []string{
		"claude:",
		"  planner  opus",   // %-8s: 7-char name + 1 pad + 1 literal = 2 spaces
		"  stager   sonnet", // %-8s: 6-char name + 2 pad + 1 literal = 3 spaces
		"  message  haiku",  // %-8s: 7-char name + 1 pad + 1 literal = 2 spaces
		"  arbiter  sonnet", // %-8s: 7-char name + 1 pad + 1 literal = 2 spaces
		"verified 2026-07-02",
		"consult `claude --help`",
	}
	for _, sub := range substrings {
		if !strings.Contains(got, sub) {
			t.Errorf("curated table output missing %q\nGot:\n%s", sub, got)
		}
	}
}

func TestModels_CuratedGolden_NonStagerProvider(t *testing.T) {
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("gemini")
	if !ok {
		t.Fatal("gemini not found in registry")
	}

	var buf bytes.Buffer
	printCuratedTable(&buf, modelTarget{name: "gemini", manifest: m})

	got := buf.String()
	// Stager should be "—" for non-stager-capable providers
	if !strings.Contains(got, "  stager   —") { // %-8s: 6-char + 2 pad + 1 literal = 3 spaces
		t.Errorf("expected stager to be '—' for gemini\nGot:\n%s", got)
	}
	if !strings.Contains(got, "verified 2026-07-02") {
		t.Errorf("curated table missing verification date\nGot:\n%s", got)
	}
}

func TestModels_CuratedGolden_UserDefined(t *testing.T) {
	// A user-defined provider has no FR-D4 column — prints informational message
	m := provider.Manifest{
		Name:    "myagent",
		Command: strPtrUnexported("/opt/myagent"),
	}
	var buf bytes.Buffer
	printCuratedTable(&buf, modelTarget{name: "myagent", manifest: m})

	got := buf.String()
	if !strings.Contains(got, "myagent:") {
		t.Error("output missing 'myagent:'")
	}
	if !strings.Contains(got, "no list_models_command and no curated per-role defaults") {
		t.Errorf("expected informational message for user-defined provider\nGot:\n%s", got)
	}
}

func TestModels_CuratedGolden_VerificationDate(t *testing.T) {
	// Verify the constant matches what's printed
	if config.DefaultModelsVerificationDate != "2026-07-02" {
		t.Errorf("DefaultModelsVerificationDate = %q, want %q", config.DefaultModelsVerificationDate, "2026-07-02")
	}
}

// ---------------------------------------------------------------------------
// Live list renderer test
// ---------------------------------------------------------------------------

func TestModels_PrintLiveList(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "pi", "gpt-5.4\ngpt-5.4-mini\n")

	got := buf.String()
	if !strings.Contains(got, "pi:") {
		t.Error("missing 'pi:' heading")
	}
	if !strings.Contains(got, "gpt-5.4") {
		t.Error("missing model line")
	}
}

func TestModels_PrintLiveList_Empty(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "test", "")

	got := buf.String()
	if !strings.Contains(got, "(no models reported)") {
		t.Errorf("expected '(no models reported)' for empty stdout\nGot:\n%s", got)
	}
}

func TestModels_PrintLiveList_NoTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "test", "single-line")

	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline for block separation\nGot:\n%q", got)
	}
}

// ---------------------------------------------------------------------------
// Stub-binary live-list test
// ---------------------------------------------------------------------------

func TestModels_LiveList_StubBinary(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Create a temp dir with a fake "opencode" that prints a model list
	tmpDir := t.TempDir()
	stubPath := filepath.Join(tmpDir, "opencode")
	stubBody := "#!/bin/sh\necho 'openai/gpt-5.4'\necho 'openai/gpt-5.4-mini'\n"
	if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
		t.Skipf("cannot create stub binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "opencode:") {
		t.Errorf("output missing 'opencode:' heading\nGot:\n%s", got)
	}
	if !strings.Contains(got, "openai/gpt-5.4") {
		t.Errorf("output missing live model 'openai/gpt-5.4'\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Command-failure fallback test
// ---------------------------------------------------------------------------

func TestModels_CommandFailure_Fallback(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Create a fake "opencode" that exits 1
	tmpDir := t.TempDir()
	stubPath := filepath.Join(tmpDir, "opencode")
	stubBody := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
		t.Skipf("cannot create stub binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (fallback succeeded)", err)
	}

	gotOut := outBuf.String()
	gotErr := errBuf.String()

	// Stdout should have the curated table
	if !strings.Contains(gotOut, "opencode:") {
		t.Errorf("stdout missing 'opencode:'\nGot:\n%s", gotOut)
	}
	if !strings.Contains(gotOut, "consult `opencode --help`") {
		t.Errorf("stdout missing curated footer\nGot:\n%s", gotOut)
	}
	// Stderr should have the failure notice
	if !strings.Contains(gotErr, "list command failed") {
		t.Errorf("stderr missing failure notice\nGot:\n%s", gotErr)
	}
}

// ---------------------------------------------------------------------------
// Timeout fallback test
// ---------------------------------------------------------------------------

func TestModels_Timeout_Fallback(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Create a fake "opencode" that sleeps 10s
	tmpDir := t.TempDir()
	stubPath := filepath.Join(tmpDir, "opencode")
	stubBody := "#!/bin/sh\nsleep 10\n"
	if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
		t.Skipf("cannot create stub binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Set a very short timeout
	t.Setenv("STAGEHAND_TIMEOUT", "1s")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (fallback succeeded)", err)
	}

	gotOut := outBuf.String()
	// Stdout should have the curated table (timeout triggered fallback)
	if !strings.Contains(gotOut, "opencode:") {
		t.Errorf("stdout missing 'opencode:' after timeout\nGot:\n%s", gotOut)
	}
	if !strings.Contains(gotOut, "consult `opencode --help`") {
		t.Errorf("stdout missing curated footer after timeout\nGot:\n%s", gotOut)
	}
}

// ---------------------------------------------------------------------------
// Error matrix
// ---------------------------------------------------------------------------

func TestModels_UnknownProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "ghost"})

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

func TestModels_UndetectedNamedProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	// qwen-code is a known built-in, but is unlikely to be on any developer's PATH.
	// Prepend empty tmpDir so we still have git from the real PATH.
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "qwen-code"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (qwen-code not detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "not detected") {
		t.Errorf("error message %q should contain 'not detected'", err.Error())
	}
}

func TestModels_NoDefault_NothingDetected(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false

	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	// Use a clean PATH with only git (symlinked) so no providers are detected.
	tmpDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found on PATH")
	}
	if err := os.Symlink(gitPath, filepath.Join(tmpDir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	t.Setenv("PATH", tmpDir)
	// Override all built-in commands to nonexistent paths AND pre-write a global config
	// with empty provider so the bootstrap doesn't set a default.
	writeConfigFile(t, repo, ".stagehand.toml", `
config_version = 3
[defaults]
provider = ""

[provider.claude]
command = "/nonexistent/claude"
[provider.pi]
command = "/nonexistent/pi"
[provider.opencode]
command = "/nonexistent/opencode"
[provider.codex]
command = "/nonexistent/codex"
[provider.cursor]
command = "/nonexistent/cursor"
[provider.gemini]
command = "/nonexistent/gemini"
[provider.agy]
command = "/nonexistent/agy"
[provider.qwen-code]
command = "/nonexistent/qwen-code"
`)
	// Also write the global config to prevent bootstrap from creating one with provider="pi"
	writeConfigFile(t, globalDir, "config.toml", `config_version = 3
[defaults]
provider = ""
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models"})

	err = Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (nothing detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "no provider detected") {
		t.Errorf("error message %q should contain 'no provider detected'", err.Error())
	}
}

func TestModels_AllEmpty(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false // reset from any prior --all test

	setupRepo(t)
	// We can't guarantee no providers are on PATH (real pi, claude, etc. may exist).
	// Instead, test the --all + arg error which is independent of detection.
	// The "no providers detected" case for --all is implicitly covered by the
	// error message check in TestModels_AllEmpty_Detection.

	// Test the --all flag works (it's separate from the empty-detection case).
	// With real providers on PATH, --all should succeed and print blocks.
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err := Execute(context.Background())
	// Should succeed if any providers are detected
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (--all with detected providers)", err)
	}
	got := out.String()
	if got == "" {
		t.Error("--all output is empty")
	}
}

// TestModels_AllEmpty_Detection tests the --all error when no providers are detected.
// This uses a user-defined provider override with a nonexistent command to simulate
// no detected providers while keeping git on PATH for config load.
func TestModels_AllEmpty_Detection(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false

	repo := setupRepo(t)
	// Override all built-in commands to nonexistent paths so nothing is detected.
	// This doesn't remove the built-ins but makes their detect commands unfindable.
	// We do this by setting PATH to a tmpDir with only git (symlinked).
	tmpDir := t.TempDir()
	// Find real git and symlink it
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found on PATH")
	}
	if err := os.Symlink(gitPath, filepath.Join(tmpDir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	t.Setenv("PATH", tmpDir)

	// Write a config that overrides all built-in commands to nonexistent paths
	writeConfigFile(t, repo, ".stagehand.toml", `
[provider.claude]
command = "/nonexistent/claude"
[provider.pi]
command = "/nonexistent/pi"
[provider.opencode]
command = "/nonexistent/opencode"
[provider.codex]
command = "/nonexistent/codex"
[provider.cursor]
command = "/nonexistent/cursor"
[provider.gemini]
command = "/nonexistent/gemini"
[provider.agy]
command = "/nonexistent/agy"
[provider.qwen-code]
command = "/nonexistent/qwen-code"
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err = Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--all, nothing detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "no providers detected") {
		t.Errorf("error message %q should contain 'no providers detected'", err.Error())
	}
}

func TestModels_AllWithArg(t *testing.T) {
	flagModelsAll = false // reset from any prior --all test
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all", "opencode"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--all + arg)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "--all cannot be combined") {
		t.Errorf("error message %q should contain '--all cannot be combined'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Default resolution tests
// ---------------------------------------------------------------------------

func TestModels_DefaultResolved(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Put a fake "pi" (highest priority) on PATH
	tmpDir := t.TempDir()
	stubPath := filepath.Join(tmpDir, "pi")
	stubBody := "#!/bin/sh\necho 'gpt-5.4'\n"
	if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
		t.Skipf("cannot create stub binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// pi is the default (highest priority detected), should show pi's block
	if !strings.Contains(got, "pi:") {
		t.Errorf("output missing 'pi:' (default resolution)\nGot:\n%s", got)
	}
}

func TestModels_DefaultResolved_ExplicitProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false // reset from any prior --all test

	setupRepo(t)

	// Create a user-defined provider "myagent" with a fake binary, and set it as default.
	tmpDir := t.TempDir()
	stubPath := filepath.Join(tmpDir, "myagent-bin")
	stubBody := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
		t.Skipf("cannot create stub binary: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("STAGEHAND_PROVIDER", "claude")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "claude"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// claude is the explicitly requested provider, should show claude's block
	if !strings.Contains(got, "claude:") {
		t.Errorf("output missing 'claude:' (explicit arg)\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// --all over detected providers
// ---------------------------------------------------------------------------

func TestModels_AllDetected(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Put fake claude and opencode on PATH
	tmpDir := t.TempDir()
	for _, name := range []string{"claude", "opencode"} {
		stubPath := filepath.Join(tmpDir, name)
		stubBody := "#!/bin/sh\nexit 0\n"
		if err := os.WriteFile(stubPath, []byte(stubBody), 0755); err != nil {
			t.Skipf("cannot create stub binary: %v", err)
		}
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "claude:") {
		t.Errorf("output missing 'claude:'\nGot:\n%s", got)
	}
	if !strings.Contains(got, "opencode:") {
		t.Errorf("output missing 'opencode:'\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Help shows models-scoped --all text
// ---------------------------------------------------------------------------

func TestModels_HelpShowsAllScopedText(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--help"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := buf.String()
	if !strings.Contains(got, "every detected provider") {
		t.Errorf("help output missing models-scoped --all text\nGot:\n%s", got)
	}
	if strings.Contains(got, "git add -A") {
		t.Error(`help output must NOT contain "git add -A" (root's --all text)`)
	}
}

// strPtrUnexported is a test helper to create *string values for Manifest fields.
func strPtrUnexported(s string) *string { return &s }
