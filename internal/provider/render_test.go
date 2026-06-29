package provider

import (
	"os"
	"reflect"
	"testing"
)

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
		{"pi", pi, "", "zai", "pi",
			[]string{"--provider", "zai", "--model", "glm-5-turbo", "--system-prompt", "<sys>",
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
// ---------------------------------------------------------------------------

func TestRender_Pi_ByteForByteCommitPi(t *testing.T) {
	spec, err := builtinPi().Render("", "zai", "<sys>", "<user>")
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
// Test 4: model default fallback: model="" → DefaultModel (pi→glm-5-turbo);
// explicit wins.
// ---------------------------------------------------------------------------

func TestRender_ModelDefaultFallback(t *testing.T) {
	byDefault, _ := builtinPi().Render("", "zai", "", "") // model="" → glm-5-turbo
	if !containsPair(byDefault.Args, "--model", "glm-5-turbo") {
		t.Errorf("model default not applied: %v", byDefault.Args)
	}
	explicit, _ := builtinPi().Render("custom-model", "zai", "", "") // explicit wins
	if !containsPair(explicit.Args, "--model", "custom-model") {
		t.Errorf("explicit model lost: %v", explicit.Args)
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
