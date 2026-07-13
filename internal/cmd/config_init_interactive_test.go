package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/provider"
)

// ---------------------------------------------------------------------------
// Pure wizard tests — no Execute, no root state juggling
// ---------------------------------------------------------------------------

// TestWizard_AcceptDefaults_ByteIdentical is the KEYSTONE test: accept-all answers → overrides nil
// → GenerateBootstrapConfigWithOverrides(chosen, nil) == GenerateBootstrapConfig(chosen) (byte-identical).
func TestWizard_AcceptDefaults_ByteIdentical(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude"}
	defaultName := "claude"

	// Accept-all: "\n" for provider pick (accept default), "\n" for each role (4 roles).
	input := "\n\n\n\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.provider != defaultName {
		t.Errorf("provider = %q, want %q", res.provider, defaultName)
	}
	if res.overrides != nil {
		t.Errorf("overrides = %v, want nil (accept-all → no edits)", res.overrides)
	}

	// Byte-identity: nil overrides must produce the same output as GenerateBootstrapConfig.
	withOverrides := config.GenerateBootstrapConfigWithOverrides(res.provider, res.overrides)
	without := config.GenerateBootstrapConfig(res.provider)
	if withOverrides != without {
		t.Errorf("byte-identity broken: GenerateBootstrapConfigWithOverrides(%q, nil) != GenerateBootstrapConfig(%q)\n"+
			"  len(with)=%d, len(without)=%d", res.provider, res.provider, len(withOverrides), len(without))
	}
}

// TestWizard_EditsSingleBackend tests editing models for a single-backend provider (claude).
func TestWizard_EditsSingleBackend(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude"}
	defaultName := "claude"

	// Pick claude (default), edit planner+message, accept stager+arbiter.
	input := "\nmy-plan\n\nmy-msg\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.provider != "claude" {
		t.Errorf("provider = %q, want %q", res.provider, "claude")
	}
	if res.overrides == nil {
		t.Fatal("overrides nil, want edits")
	}
	if res.overrides["planner"] != "my-plan" {
		t.Errorf("planner override = %q, want %q", res.overrides["planner"], "my-plan")
	}
	if res.overrides["message"] != "my-msg" {
		t.Errorf("message override = %q, want %q", res.overrides["message"], "my-msg")
	}
	// stager and arbiter should NOT be in overrides (accepted default).
	if _, ok := res.overrides["stager"]; ok {
		t.Errorf("stager should not be in overrides (accepted default)")
	}
	if _, ok := res.overrides["arbiter"]; ok {
		t.Errorf("arbiter should not be in overrides (accepted default)")
	}

	// Verify generated config has the edits and defaults elsewhere.
	content := config.GenerateBootstrapConfigWithOverrides("claude", res.overrides)
	if !strings.Contains(content, `model = "my-plan"`) {
		t.Error("generated config missing planner override")
	}
	if !strings.Contains(content, `model = "my-msg"`) {
		t.Error("generated config missing message override")
	}
	// Stager should still have claude's default.
	if !strings.Contains(content, `model = "sonnet"`) {
		t.Error("generated config should still have claude's stager default")
	}
}

// TestWizard_MultiBackend_RePrompt tests that a bare model for pi is rejected with a re-prompt.
func TestWizard_MultiBackend_RePrompt(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"pi"}
	defaultName := "pi"

	// Provider: accept pi (default). Planner: bare "gpt-5.4" → re-prompt → valid "zai/gpt-5.4".
	// Then accept remaining roles.
	input := "\ngpt-5.4\nzai/gpt-5.4\nzai/gpt-5.4-mini\nzai/gpt-5.4-nano\nzai/gpt-5.4-mini\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}

	// Output should contain the re-prompt message.
	if !strings.Contains(out.String(), "include the inference backend as a prefix") {
		t.Errorf("expected re-prompt message in output; got:\n%s", out.String())
	}
	if res.overrides["planner"] != "zai/gpt-5.4" {
		t.Errorf("planner override = %q, want %q", res.overrides["planner"], "zai/gpt-5.4")
	}
}

// TestWizard_MultiBackend_BlankAccepted tests that accept-all on pi → overrides nil → byte-identical.
func TestWizard_MultiBackend_BlankAccepted(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"pi"}
	defaultName := "pi"

	input := "\n\n\n\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.overrides != nil {
		t.Errorf("overrides = %v, want nil (accept-all on pi)", res.overrides)
	}

	// Byte-identity.
	with := config.GenerateBootstrapConfigWithOverrides("pi", res.overrides)
	without := config.GenerateBootstrapConfig("pi")
	if with != without {
		t.Error("byte-identity broken for pi accept-all")
	}
}

// TestWizard_InvalidProvider_RePrompt tests that an unknown provider name is re-prompted.
func TestWizard_InvalidProvider_RePrompt(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude"}
	defaultName := "claude"

	// Type "ghost" → re-prompt, then accept default "", then 4 role accepts.
	input := "ghost\n\n\n\n\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.provider != "claude" {
		t.Errorf("provider = %q, want %q (default after invalid)", res.provider, "claude")
	}
	if !strings.Contains(out.String(), "unknown/undetected provider") {
		t.Errorf("expected re-prompt for unknown provider; got:\n%s", out.String())
	}
}

// TestWizard_EOF_IsError tests that immediate EOF produces an error.
func TestWizard_EOF_IsError(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude"}
	defaultName := "claude"

	// Empty input → immediate EOF on first readLine.
	_, err := runInteractiveWizard(strings.NewReader(""), &bytes.Buffer{}, reg, installed, defaultName, "")
	if err == nil {
		t.Fatal("expected error on immediate EOF")
	}
	if !strings.Contains(err.Error(), "unexpected end of input") {
		t.Errorf("error = %q, want 'unexpected end of input'", err.Error())
	}
}

// TestWizard_ProviderPreSelect skips the provider prompt when pinName is set.
func TestWizard_ProviderPreSelect(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude", "pi"}
	defaultName := "claude"

	// 4 role prompts only (no provider prompt).
	input := "\n\n\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "pi")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.provider != "pi" {
		t.Errorf("provider = %q, want %q", res.provider, "pi")
	}
	if strings.Contains(out.String(), "Pick a provider") {
		t.Errorf("provider prompt should be skipped with --provider; got:\n%s", out.String())
	}
}

// TestWizard_NumberIndex tests numeric provider selection.
func TestWizard_NumberIndex(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"claude", "pi"}
	defaultName := "claude"

	// Pick #1 (pi, first in FR-D1 detected list: [pi, claude]).
	input := "1\n\n\n\n\n"
	var out bytes.Buffer

	res, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	if res.provider != "pi" {
		t.Errorf("provider = %q, want %q (selected #1 in FR-D1 order)", res.provider, "pi")
	}
}

// TestWizard_PiDisplayDefaultsAreBlank verifies that pi shows blank defaults (not the raw table values).
func TestWizard_PiDisplayDefaultsAreBlank(t *testing.T) {
	reg := provider.NewRegistry(nil)
	installed := []string{"pi"}
	defaultName := "pi"

	input := "\n\n\n\n\n"
	var out bytes.Buffer

	_, err := runInteractiveWizard(strings.NewReader(input), &out, reg, installed, defaultName, "")
	if err != nil {
		t.Fatalf("runInteractiveWizard err=%v", err)
	}
	// The prompt for pi planner should show "" not "gpt-5.4".
	if strings.Contains(out.String(), "gpt-5.4") {
		t.Errorf("pi display defaults should be blank, not raw table values; got:\n%s", out.String())
	}
}

// ---------------------------------------------------------------------------
// Execute-driven tests (TTY gate, flag composition, file writes)
// ---------------------------------------------------------------------------

// TestInteractive_NonTTY_Exits1 verifies non-TTY stdin → exit 1 with message pointing at plain config init.
func TestInteractive_NonTTY_Exits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	// Force non-TTY for this test (the test runner's stdin may be a real TTY).
	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return false }
	defer func() { interactiveStdinIsTTY = origTTY }()

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (non-TTY)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "terminal") {
		t.Errorf("error %q should contain 'terminal'", errMsg)
	}
	if !strings.Contains(errMsg, "config init") {
		t.Errorf("error %q should contain 'config init'", errMsg)
	}
}

// TestInteractive_Template_Conflict verifies --interactive --template is a usage error.
func TestInteractive_Template_Conflict(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)

	// Force TTY so we get past the TTY gate to the template check.
	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive", "--template"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--interactive --template conflict)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "choose one") {
		t.Errorf("error %q should contain 'choose one'", err.Error())
	}
}

// TestInteractive_Force_Overwrites verifies --interactive --force overwrites an existing config.
func TestInteractive_Force_Overwrites(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)

	// Pre-create the config file.
	writeConfigFile(t, globalDir, "config.toml", "provider = \"old\"\n")

	// Force TTY and pipe accept-all answers.
	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	rootCmd.SetIn(strings.NewReader("\n\n\n\n\n"))
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive", "--force"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (--interactive --force should overwrite)", err)
	}

	// File should be overwritten (not "old").
	data, _ := os.ReadFile(config.GlobalConfigPath())
	if strings.Contains(string(data), "old") {
		t.Error("file was not overwritten by --interactive --force")
	}
}

// TestInteractive_ProviderPreSelect tests --interactive --provider skips the provider prompt.
func TestInteractive_ProviderPreSelect(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)

	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	var out bytes.Buffer
	// 4 role prompts only (no provider prompt).
	rootCmd.SetIn(strings.NewReader("my-plan\n\nmy-msg\n\n"))
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive", "--provider", "claude", "--force"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// Provider prompt should NOT appear.
	if strings.Contains(out.String(), "Pick a provider") {
		t.Errorf("provider prompt should be skipped with --provider; got:\n%s", out.String())
	}

	// File should contain the edit.
	data, _ := os.ReadFile(config.GlobalConfigPath())
	if !strings.Contains(string(data), `model = "my-plan"`) {
		t.Error("file should contain planner override from --provider claude pre-select")
	}
}

// TestInteractive_NothingDetected_Exits1 verifies that no detected providers → exit 1.
func TestInteractive_NothingDetected_Exits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PATH", "") // no agents on PATH
	plainDir := t.TempDir()
	chdir(t, plainDir)

	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (no providers detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "no providers detected") {
		t.Errorf("error %q should contain 'no providers detected'", err.Error())
	}
}

// TestPlainConfigInit_Unchanged verifies plain `config init` (no --interactive) output is
// byte-identical to the pre-refactor output (regression test).
func TestPlainConfigInit_Unchanged(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--provider", "pi"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// Must have uncommented config_version = 3.
	if !strings.Contains(content, "config_version = 3") {
		t.Error("missing uncommented config_version = 3")
	}

	// pi's role models should be blank.
	assertContains(t, content, "[role.planner]", `model = ""`)
	assertContains(t, content, "[role.stager]", `model = ""`)
	assertContains(t, content, "[role.message]", `model = ""`)
	assertContains(t, content, "[role.arbiter]", `model = ""`)

	// Must match GenerateBootstrapConfig("pi") byte-for-byte.
	expected := config.GenerateBootstrapConfig("pi")
	if content != expected {
		t.Errorf("plain config init --provider pi output does not match GenerateBootstrapConfig(\"pi\")\n"+
			"  got len=%d, want len=%d", len(content), len(expected))
	}
}

// TestInteractive_UnknownProviderPin_Exits1 verifies --interactive --provider bogus → exit 1.
func TestInteractive_UnknownProviderPin_Exits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)

	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive", "--provider", "bogus"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown provider)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error %q should contain 'unknown provider'", err.Error())
	}
}

// TestInteractive_WritesFileWithEdits verifies the full Execute path writes a correct config with edits.
func TestInteractive_WritesFileWithEdits(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)

	origTTY := interactiveStdinIsTTY
	interactiveStdinIsTTY = func() bool { return true }
	defer func() { interactiveStdinIsTTY = origTTY }()

	// --provider claude + edit planner.
	rootCmd.SetIn(strings.NewReader("my-plan\n\n\n\n"))
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--interactive", "--provider", "claude"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// Must contain the edited planner model.
	if !strings.Contains(content, `model = "my-plan"`) {
		t.Error("file should contain planner override")
	}
	// Stager should still have claude's default (accepted).
	if !strings.Contains(content, `model = "sonnet"`) {
		t.Error("file should still have claude's stager default")
	}
}

// ---------------------------------------------------------------------------
// PreferredBuiltins accessor test
// ---------------------------------------------------------------------------

func TestPreferredBuiltins_ContainsKnownProviders(t *testing.T) {
	reg := provider.NewRegistry(nil)
	builtins := reg.PreferredBuiltins()

	if len(builtins) == 0 {
		t.Fatal("PreferredBuiltins returned empty slice")
	}
	// Must contain the known built-in names.
	expected := []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}
	for _, name := range expected {
		found := false
		for _, b := range builtins {
			if b == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PreferredBuiltins missing %q", name)
		}
	}
	// pi must be first (FR-D1).
	if builtins[0] != "pi" {
		t.Errorf("PreferredBuiltins[0] = %q, want %q (FR-D1 order)", builtins[0], "pi")
	}
}

// ---------------------------------------------------------------------------
// GenerateBootstrapConfigWithOverrides — pure unit test
// ---------------------------------------------------------------------------

func TestGenerateBootstrapConfigWithOverrides_NilOverrides_IsIdentity(t *testing.T) {
	for _, prov := range []string{"pi", "claude", "opencode", "codex"} {
		t.Run(prov, func(t *testing.T) {
			withNil := config.GenerateBootstrapConfigWithOverrides(prov, nil)
			plain := config.GenerateBootstrapConfig(prov)
			if withNil != plain {
				t.Errorf("GenerateBootstrapConfigWithOverrides(%q, nil) != GenerateBootstrapConfig(%q) — byte-identity broken", prov, prov)
			}
		})
	}
}

func TestGenerateBootstrapConfigWithOverrides_EmptyOverrides_IsIdentity(t *testing.T) {
	empty := map[string]string{}
	withEmpty := config.GenerateBootstrapConfigWithOverrides("claude", empty)
	plain := config.GenerateBootstrapConfig("claude")
	if withEmpty != plain {
		t.Error("empty map overrides != nil overrides — byte-identity broken")
	}
}

func TestGenerateBootstrapConfigWithOverrides_PiEditsModel(t *testing.T) {
	overrides := map[string]string{"planner": "zai/glm-5.2"}
	content := config.GenerateBootstrapConfigWithOverrides("pi", overrides)

	// Must contain the override.
	if !strings.Contains(content, `model = "zai/glm-5.2"`) {
		t.Error("missing planner override in pi config")
	}
	// Must have the format-focused note (NOT the "empty" note).
	if !strings.Contains(content, "slash-prefix") {
		t.Error("pi config with overrides should have format-focused note, not 'empty' note")
	}
	if strings.Contains(content, "The shipped per-role models are empty") {
		t.Error("pi config with overrides should NOT have the 'empty' note")
	}
}

func TestGenerateBootstrapConfigWithOverrides_ClaudeEditsStagerModel(t *testing.T) {
	overrides := map[string]string{"stager": "opus"}
	content := config.GenerateBootstrapConfigWithOverrides("claude", overrides)

	// Stager model should be overridden, but stager provider should still be "claude".
	if !strings.Contains(content, `model = "opus"`) {
		t.Error("missing stager model override")
	}
	if !strings.Contains(content, `provider = "claude"`) {
		t.Error("stager provider should still be claude")
	}
}
