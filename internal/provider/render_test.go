package provider

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures for mode tests
// ---------------------------------------------------------------------------

// dualModeManifest returns a minimal manifest with BOTH BareFlags and TooledFlags set,
// used by the 5 new Render-mode tests.
func dualModeManifest() Manifest {
	return Manifest{
		Name:        "dual",
		Command:     strPtr("agent"),
		BareFlags:   []string{"--no-tools"},
		TooledFlags: []string{"--allowed-tools", "git:*", "approval-mode", "auto"},
	}
}

// ---------------------------------------------------------------------------
// Test 1 (THE KEYSTONE): Golden Args + Stdin for ALL 6 built-in providers
// (byte-compatible with builtin_test.go's renderArgs outputs; pi is byte-for-byte commit-pi).
// ---------------------------------------------------------------------------

func TestRender_GoldenPerProvider(t *testing.T) {
	pi := builtinPi()
	claude := builtinClaude()
	gemini := builtinGemini()
	opencode := builtinOpenCode()
	codex := builtinCodex()
	cursor := builtinCursor()
	cases := []struct {
		name      string
		m         Manifest
		model     string
		provider  string
		wantCmd   string
		wantArgs  []string
		wantStdin string
	}{
		{"pi", pi, "", "", "pi", // FR-D2: shipped default — no --model/--provider
			[]string{"--system-prompt", "<sys>",
				"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session", "-p"},
			"<user>"}, // stdin; sys via flag → only user via stdin
		{"claude", claude, "sonnet", "", "claude",
			[]string{"--model", "sonnet", "--system-prompt", "<sys>",
				"--tools", "", "--setting-sources", "", "--no-session-persistence", "-p"}, // -p LAST
			"<user>"},
		{"gemini", gemini, "", "", "gemini",
			[]string{"-m", "gemini-2.5-pro", "--approval-mode", "default"},
			"<sys>\n\n<user>"}, // stdin; no sys flag → sys PREPENDED
		{"opencode", opencode, "anthropic/claude-sonnet-4", "", "opencode",
			[]string{"run", "-m", "anthropic/claude-sonnet-4", "<sys>\n\n<user>"}, // positional → payload trailing
			""},
		{"codex", codex, "gpt-5", "", "codex",
			[]string{"exec", "-m", "gpt-5", "--sandbox", "read-only", "--ephemeral"},
			"<sys>\n\n<user>"}, // stdin (REVISED builtin); no sys flag → PREPENDED
		{"cursor", cursor, "gpt-5", "", "agent", // Command="agent" (≠ Name "cursor")
			[]string{"--model", "gpt-5", "--mode", "ask", "--trust", "-p", "<sys>\n\n<user>"}, // -p LAST; positional
			""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := tc.m.Render(tc.model, tc.provider, "<sys>", "<user>")
			if err != nil {
				t.Fatalf("%s: Render error: %v", tc.name, err)
			}
			if spec.Command != tc.wantCmd {
				t.Errorf("%s: Command = %q, want %q", tc.name, spec.Command, tc.wantCmd)
			}
			if !reflect.DeepEqual(spec.Args, tc.wantArgs) {
				t.Errorf("%s: Args =\n got %v\nwant %v", tc.name, spec.Args, tc.wantArgs)
			}
			if spec.Stdin != tc.wantStdin {
				t.Errorf("%s: Stdin = %q, want %q", tc.name, spec.Stdin, tc.wantStdin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2 (THE headline): pi is byte-for-byte the commit-pi invocation
// (FR-D2: this is now the PERSONAL OVERRIDE path — explicit model+provider).
// ---------------------------------------------------------------------------

func TestRender_Pi_ByteForByteCommitPi(t *testing.T) {
	spec, err := builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>") // explicit personal override
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	wantArgs := []string{"--provider", "zai", "--model", "glm-5-turbo", "--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates",
		"--no-context-files", "--no-session", "-p"}
	if spec.Command != "pi" || !reflect.DeepEqual(spec.Args, wantArgs) || spec.Stdin != "<user>" {
		t.Errorf("pi not byte-for-byte commit-pi:\n Command=%q\n Args=%v\n Stdin=%q",
			spec.Command, spec.Args, spec.Stdin)
	}
}

// ---------------------------------------------------------------------------
// Test 3: System-prompt-prepend fallback: no sys flag → sys prepended to payload
// (delimiter "\n\n"); with flag → not prepended.
// ---------------------------------------------------------------------------

func TestRender_SystemPromptPrependFallback(t *testing.T) {
	// gemini has NO sys flag (SystemPromptFlag resolves to "") → sys prepended to stdin payload.
	got, _ := builtinGemini().Render("", "", "SYS", "USER")
	if got.Stdin != "SYS\n\nUSER" {
		t.Errorf("gemini prepend: Stdin = %q, want SYS\\n\\nUSER", got.Stdin)
	}
	// pi HAS a sys flag → sys via flag, payload = user only.
	got2, _ := builtinPi().Render("", "zai", "SYS", "USER")
	if got2.Stdin != "USER" {
		t.Errorf("pi (sys flag): Stdin = %q, want USER", got2.Stdin)
	}
	// Empty sys + no flag → no prepend (no leading newlines).
	got3, _ := builtinGemini().Render("", "", "", "USER")
	if got3.Stdin != "USER" {
		t.Errorf("empty sys: Stdin = %q, want USER", got3.Stdin)
	}
}

// ---------------------------------------------------------------------------
// Test 4: model default fallback: claude→sonnet (default model honored);
// explicit wins. Pi (FR-D2: empty default) emits NO --model.
// ---------------------------------------------------------------------------

func TestRender_ModelDefaultFallback(t *testing.T) {
	// claude: model="" → DefaultModel="sonnet"
	byDefault, _ := builtinClaude().Render("", "", "", "")
	if !containsPair(byDefault.Args, "--model", "sonnet") {
		t.Errorf("claude model default not applied: %v", byDefault.Args)
	}
	// claude: explicit model wins over default
	explicit, _ := builtinClaude().Render("custom-model", "", "", "")
	if !containsPair(explicit.Args, "--model", "custom-model") {
		t.Errorf("claude explicit model lost: %v", explicit.Args)
	}
	// pi (FR-D2: empty default) emits NO --model
	piNoModel, _ := builtinPi().Render("", "", "", "")
	if containsToken(piNoModel.Args, "--model") {
		t.Errorf("pi should emit no --model by default (FR-D2): %v", piNoModel.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 5: provider default fallback: provider="" → DefaultProvider (all
// built-ins "" → no --provider); a §12.8 default honored.
// ---------------------------------------------------------------------------

func TestRender_ProviderDefaultFallback(t *testing.T) {
	got, _ := builtinPi().Render("", "", "", "USER") // provider="" → DefaultProvider="" → no --provider
	for i, a := range got.Args {
		if a == "--provider" {
			t.Errorf("unexpected --provider at %d: %v", i, got.Args)
		}
	}
	// A §12.8 user manifest with default_provider="zai" + provider_flag → honored when caller passes "".
	user := Manifest{Name: "test", Command: strPtr("agent"), ProviderFlag: strPtr("--provider"), DefaultProvider: strPtr("zai")}
	got2, err := user.Render("", "", "", "USER")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !containsPair(got2.Args, "--provider", "zai") {
		t.Errorf("default_provider not honored: %v", got2.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Env: os.Environ() + manifest entries as "KEY=VAL"; manifest wins on
// collision; membership-only assertions.
// ---------------------------------------------------------------------------

func TestRender_Env(t *testing.T) {
	osEnvLen := len(os.Environ())
	m := Manifest{Name: "test", Command: strPtr("pi"), Env: map[string]string{"PI_OFFLINE": "1", "DEBUG": "x"}}
	spec, err := m.Render("", "", "", "USER")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(spec.Env) < osEnvLen+2 {
		t.Errorf("Env len = %d, want >= %d", len(spec.Env), osEnvLen+2)
	}
	set := map[string]bool{}
	for _, e := range spec.Env {
		set[e] = true
	}
	if !set["PI_OFFLINE=1"] {
		t.Errorf("manifest env PI_OFFLINE=1 missing: %v", spec.Env)
	}
	if !set["DEBUG=x"] {
		t.Errorf("manifest env DEBUG=x missing: %v", spec.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 7: flag delivery: payload appended as (prompt_flag, payload).
// ---------------------------------------------------------------------------

func TestRender_FlagDelivery(t *testing.T) {
	m := Manifest{Name: "test", Command: strPtr("agent"), PromptDelivery: strPtr("flag"), PromptFlag: strPtr("--prompt")}
	spec, _ := m.Render("", "", "", "PAYLOAD")
	if !containsPair(spec.Args, "--prompt", "PAYLOAD") {
		t.Errorf("flag delivery: %v", spec.Args)
	}
	if spec.Stdin != "" {
		t.Errorf("flag delivery Stdin = %q, want empty", spec.Stdin)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Validate error propagation: missing Command → error; invalid
// prompt_delivery → error.
// ---------------------------------------------------------------------------

func TestRender_ValidateErrors(t *testing.T) {
	if _, err := (Manifest{}).Render("", "", "", "U"); err == nil {
		t.Error("want error for empty manifest (no command)")
	}
	if _, err := (Manifest{Name: "x", Command: strPtr("pi"), PromptDelivery: strPtr("bogus")}).Render("", "", "", "U"); err == nil {
		t.Error("want error for invalid prompt_delivery")
	}
}

// ---------------------------------------------------------------------------
// Test 8b: FR-R5b backstop — Render NEVER emits a bare --model for a multi-provider agent.
// This is the single chokepoint every call path (v1 generate + all four decompose roles) flows
// through; all pass "" for the provider param and rely on the manifest's default_provider. A pinned
// model with no inference provider is rejected here so no path can emit an unroutable command.
// Exempt: no model, or a single-backend / combined-form agent (no provider_flag).
// ---------------------------------------------------------------------------

func TestRender_FR5b_RejectsBareModelOnMultiProvider(t *testing.T) {
	pi := builtinPi() // ProviderFlag="--provider", DefaultProvider="" (shipped)

	// Pinned model + NO inference provider → ERROR (the bug: was silently `pi --model glm-5.2`).
	if _, err := pi.Render("glm-5.2", "", "<sys>", "<user>"); err == nil {
		t.Fatal("Render accepted a bare --model on pi (no inference provider); want FR-R5b error")
	}

	// Same via the manifest default_model path (param "" → DefaultModel) when DefaultModel is set:
	// build a pi-shaped manifest with a default_model but no default_provider.
	m := Manifest{
		Name: "pi", Command: strPtr("pi"), PromptDelivery: strPtr("stdin"),
		ProviderFlag: strPtr("--provider"), ModelFlag: strPtr("--model"),
		DefaultModel: strPtr("glm-5.2"), DefaultProvider: strPtr(""),
	}
	if _, err := m.Render("", "", "<sys>", "<user>"); err == nil {
		t.Fatal("Render accepted a bare default_model on a multi-provider agent; want FR-R5b error")
	}

	// Inference provider supplied as the Render param → OK, emits --provider.
	spec, err := pi.Render("glm-5.2", "zai", "<sys>", "<user>")
	if err != nil {
		t.Fatalf("Render with explicit provider: %v", err)
	}
	if !containsPair(spec.Args, "--provider", "zai") || !containsPair(spec.Args, "--model", "glm-5.2") {
		t.Errorf("want --provider zai + --model glm-5.2; got %v", spec.Args)
	}

	// No model at all → OK (pi picks its own backend default; the shipped, blank-model path).
	if _, err := pi.Render("", "", "<sys>", "<user>"); err != nil {
		t.Errorf("Render with no model should be allowed (pi picks its own default): %v", err)
	}

	// Single-backend agent (claude, no provider_flag) + model + no provider → OK.
	claude := builtinClaude()
	if _, err := claude.Render("sonnet", "", "<sys>", "<user>"); err != nil {
		t.Errorf("claude (single-backend) should allow a bare --model: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 9: Resolve is non-mutating: the caller's manifest is untouched after
// Render.
// ---------------------------------------------------------------------------

func TestRender_DoesNotMutateManifest(t *testing.T) {
	m := builtinPi()
	origSys := m.SystemPromptFlag
	_, _ = m.Render("", "zai", "<sys>", "<user>")
	if m.SystemPromptFlag != origSys {
		t.Errorf("Render mutated SystemPromptFlag: %v vs %v", m.SystemPromptFlag, origSys)
	}
}

// ---------------------------------------------------------------------------
// Test 10: Render is byte-compatible with the test-only renderArgs for the
// flags portion.
// ---------------------------------------------------------------------------

func TestRender_CompatWithRenderArgs(t *testing.T) {
	// renderArgs returns Command as element[0]; CmdSpec splits Command out. Same tokens, same order.
	flags := renderArgs(builtinCodex(), "", "gpt-5", "<sys>")
	spec, _ := builtinCodex().Render("gpt-5", "", "<sys>", "<user>")
	if spec.Command != flags[0] {
		t.Errorf("Command mismatch: %q vs %q", spec.Command, flags[0])
	}
	if !reflect.DeepEqual(spec.Args, flags[1:]) {
		t.Errorf("Args != renderArgs flags:\n got %v\nwant %v", spec.Args, flags[1:])
	}
}

// ---------------------------------------------------------------------------
// Test 11: Default mode (no mode arg) is bare
// ---------------------------------------------------------------------------

func TestRender_DefaultModeIsBare(t *testing.T) {
	m := dualModeManifest()
	spec, err := m.Render("", "", "", "U")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	// BareFlags present, TooledFlags absent
	if !containsPair(spec.Args, "--no-tools", "") && !containsToken(spec.Args, "--no-tools") {
		t.Errorf("bare flag --no-tools missing from args: %v", spec.Args)
	}
	if containsToken(spec.Args, "--allowed-tools") {
		t.Errorf("tooled flag --allowed-tools should NOT appear in bare mode: %v", spec.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 12: Explicit bare mode is identical to default (no mode arg)
// ---------------------------------------------------------------------------

func TestRender_ExplicitBareMode(t *testing.T) {
	m := dualModeManifest()
	specDefault, _ := m.Render("", "", "", "U")
	specBare, _ := m.Render("", "", "", "U", RenderBare)
	if !reflect.DeepEqual(specDefault.Args, specBare.Args) {
		t.Errorf("explicit bare differs from default:\n default=%v\n bare   =%v", specDefault.Args, specBare.Args)
	}
	if specDefault.Stdin != specBare.Stdin {
		t.Errorf("Stdin differs: default=%q bare=%q", specDefault.Stdin, specBare.Stdin)
	}
}

// ---------------------------------------------------------------------------
// Test 13: Tooled mode appends TooledFlags (not BareFlags)
// ---------------------------------------------------------------------------

func TestRender_TooledModeAppendsTooledFlags(t *testing.T) {
	m := dualModeManifest()
	spec, err := m.Render("", "", "", "U", RenderTooled)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !containsToken(spec.Args, "--allowed-tools") {
		t.Errorf("tooled flag --allowed-tools missing: %v", spec.Args)
	}
	if containsToken(spec.Args, "--no-tools") {
		t.Errorf("bare flag --no-tools should NOT appear in tooled mode: %v", spec.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 14: Tooled mode with empty/nil TooledFlags returns an error
// ---------------------------------------------------------------------------

func TestRender_TooledModeEmptyFlagsErrors(t *testing.T) {
	bareOnly := Manifest{Name: "stager", Command: strPtr("agent"), BareFlags: []string{"--no-tools"}} // TooledFlags nil
	_, err := bareOnly.Render("", "", "", "U", RenderTooled)
	if err == nil {
		t.Fatal("expected error for tooled mode with nil TooledFlags, got nil")
	}
	if !strings.Contains(err.Error(), "tooled mode requires non-empty tooled_flags") {
		t.Errorf("error message missing expected text: %v", err)
	}
	if !strings.Contains(err.Error(), "stager") {
		t.Errorf("error message missing provider name: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 15: All golden providers still render in default bare mode (regression)
// ---------------------------------------------------------------------------

func TestRender_AllGoldenProvidersStillBareDefault(t *testing.T) {
	for _, b := range []Manifest{builtinPi(), builtinClaude(), builtinGemini(), builtinOpenCode(), builtinCodex(), builtinCursor()} {
		spec, err := b.Render("", "", "<sys>", "<user>") // no mode
		if err != nil {
			t.Errorf("provider %q: no-mode Render error: %v", b.Name, err)
		}
		if spec.Command == "" {
			t.Errorf("provider %q: empty Command", b.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// containsPair checks whether a flag-value pair appears consecutively in args.
func containsPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

// containsToken checks whether a single token appears anywhere in args.
func containsToken(args []string, token string) bool {
	for _, a := range args {
		if a == token {
			return true
		}
	}
	return false
}
