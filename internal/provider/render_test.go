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
// used by the Render-mode tests.
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
	opencode := builtinOpenCode()
	codex := builtinCodex()
	cursor := builtinCursor()
	cases := []struct {
		name      string
		m         Manifest
		model     string
		wantCmd   string
		wantArgs  []string
		wantStdin string
	}{
		{"pi", pi, "", "pi", // FR-D2: shipped default — no --model/--provider
			[]string{"--system-prompt", "<sys>",
				"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session", "-p"},
			"<user>"}, // stdin; sys via flag → only user via stdin
		{"claude", claude, "sonnet", "claude",
			[]string{"--model", "sonnet", "--system-prompt", "<sys>",
				"--tools", "", "--setting-sources", "", "--no-session-persistence", "-p"}, // -p LAST
			"<user>"},
		{"opencode", opencode, "anthropic/claude-sonnet-4", "opencode",
			[]string{"run", "-m", "anthropic/claude-sonnet-4"}, // stdin (REVISED) → payload piped, NOT in argv
			"<sys>\n\n<user>"}, // stdin; no sys flag on `run` → sys PREPENDED
		{"codex", codex, "gpt-5", "codex",
			[]string{"exec", "-m", "gpt-5", "--sandbox", "read-only", "--ephemeral"},
			"<sys>\n\n<user>"}, // stdin (REVISED builtin); no sys flag → PREPENDED
		{"cursor", cursor, "gpt-5", "agent", // Command="agent" (≠ Name "cursor")
			[]string{"--model", "gpt-5", "--mode", "ask", "--trust", "-p", "<sys>\n\n<user>"}, // -p LAST; positional
			""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := tc.m.Render(tc.model, "<sys>", "<user>", "off")
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
// (FR-R5b: model-prefix fold — the inference provider is now the slash-prefix on model).
// ---------------------------------------------------------------------------

func TestRender_Pi_ByteForByteCommitPi(t *testing.T) {
	spec, err := builtinPi().Render("zai/glm-5-turbo", "<sys>", "<user>", "off") // model-prefix fold: "zai/glm-5-turbo" → --provider zai --model glm-5-turbo
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
	// agy has NO sys flag (SystemPromptFlag resolves to "") → sys prepended to stdin payload.
	got, _ := builtinAgy().Render("", "SYS", "USER", "off")
	if got.Stdin != "SYS\n\nUSER" {
		t.Errorf("agy prepend: Stdin = %q, want SYS\\n\\nUSER", got.Stdin)
	}
	// pi HAS a sys flag → sys via flag, payload = user only.
	got2, _ := builtinPi().Render("", "SYS", "USER", "off")
	if got2.Stdin != "USER" {
		t.Errorf("pi (sys flag): Stdin = %q, want USER", got2.Stdin)
	}
	// Empty sys + no flag → no prepend (no leading newlines).
	got3, _ := builtinAgy().Render("", "", "USER", "off")
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
	byDefault, _ := builtinClaude().Render("", "", "", "off")
	if !containsPair(byDefault.Args, "--model", "sonnet") {
		t.Errorf("claude model default not applied: %v", byDefault.Args)
	}
	// claude: explicit model wins over default
	explicit, _ := builtinClaude().Render("custom-model", "", "", "off")
	if !containsPair(explicit.Args, "--model", "custom-model") {
		t.Errorf("claude explicit model lost: %v", explicit.Args)
	}
	// pi (FR-D2: empty default) emits NO --model
	piNoModel, _ := builtinPi().Render("", "", "", "off")
	if containsToken(piNoModel.Args, "--model") {
		t.Errorf("pi should emit no --model by default (FR-D2): %v", piNoModel.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 5: FR-R5b model-prefix fold — pi (provider_flag) splits "backend/model";
// opencode (no provider_flag) passes model VERBATIM.
// ---------------------------------------------------------------------------

func TestRender_ModelPrefixFold(t *testing.T) {
	// pi + "zai/glm-5.2" (provider_flag="--provider") → fold to --provider zai --model glm-5.2
	s, _ := builtinPi().Render("zai/glm-5.2", "<sys>", "<user>", "off")
	if !containsPair(s.Args, "--provider", "zai") || !containsPair(s.Args, "--model", "glm-5.2") || containsToken(s.Args, "zai/glm-5.2") {
		t.Errorf("fold: %v", s.Args)
	}
	// opencode (no provider_flag) + "openai/gpt-5.4" → VERBATIM, NOT split
	o, _ := builtinOpenCode().Render("openai/gpt-5.4", "<sys>", "<user>", "off")
	if !containsPair(o.Args, "-m", "openai/gpt-5.4") || containsToken(o.Args, "--provider") {
		t.Errorf("opencode should pass model verbatim (no --provider): %v", o.Args)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Env: os.Environ() + manifest entries as "KEY=VAL"; manifest wins on
// collision; membership-only assertions.
// ---------------------------------------------------------------------------

func TestRender_Env(t *testing.T) {
	osEnvLen := len(os.Environ())
	m := Manifest{Name: "test", Command: strPtr("pi"), Env: map[string]string{"PI_OFFLINE": "1", "DEBUG": "x"}}
	spec, err := m.Render("", "", "USER", "off")
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
	spec, _ := m.Render("", "", "PAYLOAD", "off")
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
	if _, err := (Manifest{}).Render("", "", "U", "off"); err == nil {
		t.Error("want error for empty manifest (no command)")
	}
	if _, err := (Manifest{Name: "x", Command: strPtr("pi"), PromptDelivery: strPtr("bogus")}).Render("", "", "U", "off"); err == nil {
		t.Error("want error for invalid prompt_delivery")
	}
}

// ---------------------------------------------------------------------------
// Test 8b: FR-R5b v3 matrix — model-prefix fold + no-slash error + verbatim exemptions.
// Render is the single chokepoint enforcing the contract: a provider_flag provider (pi) splits
// "backend/model" → --provider <backend> --model <rest>; a bare model (no "/") is a HARD ERROR.
// Providers without a provider_flag (claude, opencode) pass the model VERBATIM.
// ---------------------------------------------------------------------------

func TestRender_FR5b_RejectsBareModelOnMultiProvider(t *testing.T) {
	pi := builtinPi() // ProviderFlag="--provider"

	// (1) bare model, no slash → ERROR
	if _, err := pi.Render("glm-5.2", "<sys>", "<user>", "off"); err == nil {
		t.Fatal("want no-slash error")
	}

	// (2) default_model path, no slash → ERROR (pi-shaped manifest with DefaultModel set)
	m := Manifest{
		Name: "pi", Command: strPtr("pi"), PromptDelivery: strPtr("stdin"),
		ProviderFlag: strPtr("--provider"), ModelFlag: strPtr("--model"),
		DefaultModel: strPtr("glm-5.2"),
	}
	if _, err := m.Render("", "<sys>", "<user>", "off"); err == nil {
		t.Fatal("want no-slash error on default_model")
	}

	// (3) fold success: "zai/glm-5.2" → --provider zai --model glm-5.2
	s, err := pi.Render("zai/glm-5.2", "<sys>", "<user>", "off")
	if err != nil || !containsPair(s.Args, "--provider", "zai") || !containsPair(s.Args, "--model", "glm-5.2") {
		t.Errorf("fold: err=%v args=%v", err, s.Args)
	}

	// (4) no model → OK (skips the split)
	if _, err := pi.Render("", "<sys>", "<user>", "off"); err != nil {
		t.Errorf("no model should be OK: %v", err)
	}

	// (5) single-backend (claude) + bare model → OK (verbatim, no provider_flag)
	if _, err := builtinClaude().Render("sonnet", "<sys>", "<user>", "off"); err != nil {
		t.Errorf("claude bare model should be OK: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 9: Resolve is non-mutating: the caller's manifest is untouched after
// Render.
// ---------------------------------------------------------------------------

func TestRender_DoesNotMutateManifest(t *testing.T) {
	m := builtinPi()
	origSys := m.SystemPromptFlag
	_, _ = m.Render("", "<sys>", "<user>", "off")
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
	spec, _ := builtinCodex().Render("gpt-5", "<sys>", "<user>", "off")
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
	spec, err := m.Render("", "", "U", "off")
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
	specDefault, _ := m.Render("", "", "U", "off")
	specBare, _ := m.Render("", "", "U", "off", RenderBare)
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
	spec, err := m.Render("", "", "U", "off", RenderTooled)
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
	_, err := bareOnly.Render("", "", "U", "off", RenderTooled)
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
	for _, b := range []Manifest{builtinPi(), builtinClaude(), builtinOpenCode(), builtinCodex(), builtinCursor()} {
		spec, err := b.Render("", "<sys>", "<user>", "off") // no mode
		if err != nil {
			t.Errorf("provider %q: no-mode Render error: %v", b.Name, err)
		}
		if spec.Command == "" {
			t.Errorf("provider %q: empty Command", b.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 16: FR-R6 — reasoning tokens appended when declared (incl. tooled mode);
//           absent level / nil table → silent no-op, never an error.
// ---------------------------------------------------------------------------

func TestRender_ReasoningTokensAppended(t *testing.T) {
	m := Manifest{Name: "r", Command: strPtr("agent"), ModelFlag: strPtr("--model"),
		ReasoningLevels: map[string][]string{"high": {"--thinking", "high"}}}
	// declared level → tokens appended after the model flag
	s, err := m.Render("m", "", "", "high")
	if err != nil {
		t.Fatalf("high: %v", err)
	}
	if !containsPair(s.Args, "--thinking", "high") {
		t.Errorf("reasoning tokens missing: %v", s.Args)
	}
	// "off" → no tokens, no error (off has no entry → nil slice → len 0)
	so, _ := m.Render("m", "", "", "off")
	if containsToken(so.Args, "--thinking") {
		t.Errorf("off should append no tokens: %v", so.Args)
	}
	// undeclared level → silent no-op, NEVER an error
	if _, err := m.Render("m", "", "", "medium"); err != nil {
		t.Errorf("undeclared level errored: %v", err)
	}
}

func TestRender_PiReasoningThinkingTokens(t *testing.T) {
	m := builtinPi() // the REAL built-in (not synthetic)
	// high/medium/low → --thinking <level> appended after the model flag (FR-R5b fold first)
	for _, lvl := range []string{"high", "medium", "low"} {
		s, err := m.Render("zai/glm-5.2", "", "", lvl) // folds to --provider zai --model glm-5.2
		if err != nil {
			t.Fatalf("%s: %v", lvl, err)
		}
		if !containsPair(s.Args, "--thinking", lvl) {
			t.Errorf("pi %s: want --thinking %s in %v", lvl, lvl, s.Args)
		}
	}
	// off / "" → no --thinking token, never an error (FR-R6 no-op)
	for _, lvl := range []string{"off", ""} {
		s, err := m.Render("zai/glm-5.2", "", "", lvl)
		if err != nil {
			t.Fatalf("%q: %v", lvl, err)
		}
		if containsToken(s.Args, "--thinking") {
			t.Errorf("pi %q: want NO --thinking token in %v", lvl, s.Args)
		}
	}
}

func TestRender_ClaudeReasoningEffortTokens(t *testing.T) {
	m := builtinClaude() // the REAL built-in (not synthetic)
	// high/medium/low → --effort <level> appended after the model flag
	for _, lvl := range []string{"high", "medium", "low"} {
		s, err := m.Render("sonnet", "", "", lvl)
		if err != nil {
			t.Fatalf("%s: %v", lvl, err)
		}
		if !containsPair(s.Args, "--effort", lvl) {
			t.Errorf("claude %s: want --effort %s in %v", lvl, lvl, s.Args)
		}
	}
	// off / "" → no --effort token, never an error (FR-R6 no-op)
	for _, lvl := range []string{"off", ""} {
		s, err := m.Render("sonnet", "", "", lvl)
		if err != nil {
			t.Fatalf("%q: %v", lvl, err)
		}
		if containsToken(s.Args, "--effort") {
			t.Errorf("claude %q: want NO --effort token in %v", lvl, s.Args)
		}
	}
}

func TestRender_ReasoningNilTableNoOp(t *testing.T) {
	// nil ReasoningLevels + any level → no-op, no error (FR-R6 graceful; nil map reads are safe)
	m := Manifest{Name: "n", Command: strPtr("agent"), ModelFlag: strPtr("--model")}
	if _, err := m.Render("m", "", "", "high"); err != nil {
		t.Errorf("nil table + high errored: %v", err)
	}
}

func TestRender_ReasoningTooledMode(t *testing.T) {
	// reasoning tokens append in TOOLED mode too (RenderTooled path)
	m := dualModeManifest()
	m.ReasoningLevels = map[string][]string{"high": {"--reason", "high"}}
	s, err := m.Render("", "", "U", "high", RenderTooled)
	if err != nil {
		t.Fatalf("tooled+reasoning: %v", err)
	}
	if !containsPair(s.Args, "--reason", "high") {
		t.Errorf("reasoning tokens missing in tooled mode: %v", s.Args)
	}
}

// ---------------------------------------------------------------------------
// RenderMultiTurn tests (S3 — multi-turn fallback renderer, PRD §9.24 FR-T6 / FR-T8 / FR-T9)
// ---------------------------------------------------------------------------

// mtPiManifest is a pi-shape Manifest literal with SessionMode="append" (the gate-passing capability).
// S3's unit tests set SessionMode directly in a literal — they are code-independent of S2 (merge) / S4
// (the shipped pi builtin value).
func mtPiManifest() Manifest {
	return Manifest{
		Name:             "pi",
		Command:          strPtr("pi"),
		ProviderFlag:     strPtr("--provider"),
		ModelFlag:        strPtr("--model"),
		SystemPromptFlag: strPtr("--system-prompt"),
		PrintFlag:        strPtr("-p"),
		PromptDelivery:   strPtr("stdin"),
		SessionMode:      strPtr("append"),
		BareFlags:        []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"},
	}
}

// TestRenderMultiTurn_PiTurn1_Golden is the byte-for-byte FR-T9 pin: --no-session dropped, --session-id
// <id> added (before -p), --system-prompt <sys> present on turn 1, Stdin = payload only.
func TestRenderMultiTurn_PiTurn1_Golden(t *testing.T) {
	spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-test", 1)
	if err != nil {
		t.Fatalf("RenderMultiTurn: %v", err)
	}
	wantArgs := []string{
		"--provider", "zai", "--model", "glm-5.2",
		"--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
		// NOTE: "--no-session" is ABSENT (filtered).
		"--session-id", "stagecoach-test",
		"-p", // print_flag LAST
	}
	if spec.Command != "pi" {
		t.Errorf("Command = %q, want "+"\"pi\"", spec.Command)
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Errorf("turn1 Args =\n got %v\nwant %v", spec.Args, wantArgs)
	}
	if spec.Stdin != "<payload>" {
		t.Errorf("turn1 Stdin = %q, want <payload>", spec.Stdin)
	}
	// Belt + suspenders on the FR-T6 swap.
	if !containsToken(spec.Args, "--session-id") {
		t.Errorf("--session-id missing: %v", spec.Args)
	}
	if containsToken(spec.Args, "--no-session") {
		t.Errorf("--no-session present: %v", spec.Args)
	}
}

// TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend proves turn>1 ⇒ no --system-prompt flag AND no
// sys prepend — even though sysPrompt is passed non-empty. The single turnSys local makes both guards
// turn-correct; this is the load-bearing turn-1-only assertion.
func TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend(t *testing.T) {
	spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-test", 2)
	if err != nil {
		t.Fatalf("RenderMultiTurn: %v", err)
	}
	wantArgs := []string{
		"--provider", "zai", "--model", "glm-5.2",
		// NOTE: NO "--system-prompt","<sys>" (turn>1 ⇒ turnSys="" ⇒ flag suppressed).
		"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
		"--session-id", "stagecoach-test",
		"-p",
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Errorf("turn2 Args =\n got %v\nwant %v", spec.Args, wantArgs)
	}
	if containsToken(spec.Args, "--system-prompt") {
		t.Errorf("turn2 leaked --system-prompt: %v", spec.Args)
	}
	if spec.Stdin != "<payload>" {
		t.Errorf("turn2 Stdin = %q, want <payload> (no prepend)", spec.Stdin)
	}
}

// TestRenderMultiTurn_NonAppendProviderErrors proves the capability gate fires for a provider whose
// SessionMode is not "append" (nil → Resolve defaults to "" → gate fires).
func TestRenderMultiTurn_NonAppendProviderErrors(t *testing.T) {
	m := Manifest{
		Name:             "pi",
		Command:          strPtr("pi"),
		ProviderFlag:     strPtr("--provider"),
		ModelFlag:        strPtr("--model"),
		SystemPromptFlag: strPtr("--system-prompt"),
		PrintFlag:        strPtr("-p"),
		PromptDelivery:   strPtr("stdin"),
		SessionMode:      strPtr(""), // explicit "" (non-append) — gate must fire
		BareFlags:        []string{"--no-session"},
	}
	spec, err := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<p>", "", "id", 1)
	if err == nil {
		t.Fatal("want error for non-append provider, got nil")
	}
	if !strings.Contains(err.Error(), "session_mode") {
		t.Errorf("error message missing expected text: %v", err)
	}
	if spec != nil {
		t.Errorf("spec = %v, want nil on error", spec)
	}
}

// TestRenderMultiTurn_DoesNotMutateManifest proves the filtered session-flags block builds a FRESH slice
// (did not assign into / re-slice r.BareFlags). m.BareFlags must still contain "--no-session" after the
// call.
func TestRenderMultiTurn_DoesNotMutateManifest(t *testing.T) {
	m := mtPiManifest()
	wantBare := append([]string(nil), m.BareFlags...)
	_, _ = m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-test", 1)
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags mutated:\n got %v\nwant %v", m.BareFlags, wantBare)
	}
	if !containsToken(m.BareFlags, "--no-session") {
		t.Errorf("m.BareFlags should still contain --no-session: %v", m.BareFlags)
	}
}

// TestRenderMultiTurn_GoldenTable is the §5 golden/table consolidation. PRESENCE-based (containsPair/
// containsToken), NOT byte-exact: S3's individual tests own byte-exact; this table owns the contract
// surface in the codebase's documented table idiom. Per the work item: "assert presence + the
// system-prompt turn-1-only distinction"; "Do NOT assert exact arg position for --session-id".
func TestRenderMultiTurn_GoldenTable(t *testing.T) {
	cases := []struct {
		name                 string
		turn                 int
		sessionID            string
		wantSysPromptPresent bool
	}{
		{"pi_turn1_session_id_and_sys_prompt_no_no_session", 1, "stagecoach-gt-t1", true},
		{"pi_turn2_session_id_present_sys_prompt_absent", 2, "stagecoach-gt-t1", false},
		{"pi_turn3_session_id_still_present_sys_prompt_absent", 3, "stagecoach-gt-t1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", tc.sessionID, tc.turn)
			if err != nil {
				t.Fatalf("turn %d: %v", tc.turn, err)
			}
			if !containsPair(spec.Args, "--session-id", tc.sessionID) {
				t.Errorf("turn %d: --session-id %q not present as a pair: %v", tc.turn, tc.sessionID, spec.Args)
			}
			if containsToken(spec.Args, "--no-session") {
				t.Errorf("turn %d: --no-session should be filtered out: %v", tc.turn, spec.Args)
			}
			if containsToken(spec.Args, "--system-prompt") != tc.wantSysPromptPresent {
				t.Errorf("turn %d: --system-prompt presence = %v, want %v",
					tc.turn, containsToken(spec.Args, "--system-prompt"), tc.wantSysPromptPresent)
			}
			if !containsToken(spec.Args, "-p") {
				t.Errorf("turn %d: -p (print_flag) missing: %v", tc.turn, spec.Args)
			}
			if spec.Stdin != "<payload>" {
				t.Errorf("turn %d: Stdin = %q, want <payload> (no sys prepend via stdin)", tc.turn, spec.Stdin)
			}
		})
	}
}

// TestRenderMultiTurn_SessionIDStableAcrossTurns pins FR-T6's invariant: the orchestrator mints ONE
// session id and re-invokes it every turn. S3's per-turn tests use a shared literal but never EXPLICITLY
// assert the id renders identically across turns. A regression that mutated the id per turn (e.g.
// appending a counter) would fail here.
func TestRenderMultiTurn_SessionIDStableAcrossTurns(t *testing.T) {
	const sid = "stagecoach-stability-probe"
	for turn := 1; turn <= 3; turn++ {
		spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", sid, turn)
		if err != nil {
			t.Fatalf("turn %d: %v", turn, err)
		}
		if !containsPair(spec.Args, "--session-id", sid) {
			t.Errorf("turn %d: expected --session-id %q in args, got %v", turn, sid, spec.Args)
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
