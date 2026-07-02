package provider

import (
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// Verbatim PRD §12.3 / §12.4 TOML — the decode-parity oracle.
// These are the AUTHORITATIVE manifest definitions; decoding them must produce
// a Manifest identical to the literal construction in builtin.go.
// ---------------------------------------------------------------------------

const piTOML = `name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = ""                          # FR-D2: empty in the shipped default; config init fills per-role (§9.16 FR-D4)
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"
default_provider = ""
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
tooled_flags = [
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
output = "raw"
strip_code_fence = true
`

const claudeTOML = `name = "claude"
detect = "claude"
command = "claude"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "sonnet"
system_prompt_flag = "--system-prompt"
provider_flag = ""
bare_flags = [
  "--tools", "",
  "--setting-sources", "",
  "--no-session-persistence",
]
tooled_flags = [
  "--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit",
  "--setting-sources", "",
  "--no-session-persistence",
]
output = "raw"
strip_code_fence = true
`

// geminiTOML — PRD §12.5 VERBATIM EXCEPT prompt_delivery is REVISED to "stdin" (the work-item contract;
// external_deps.md §gemini recommendation; Appendix E #1: stdin avoids arg-length limits on ~300 KB diffs).
// This is the ONE intentional deviation from the verbatim PRD TOML; decoding it must match builtinGemini().
const geminiTOML = `name = "gemini"
detect = "gemini"
command = "gemini"
prompt_delivery = "stdin"   # REVISED from §12.5 "positional" (work-item + external_deps.md + Appx E #1)
print_flag = ""
model_flag = "-m"
default_model = "gemini-2.5-pro"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true
`

// opencodeTOML — PRD §12.6 VERBATIM (no revision). Decoding it must match builtinOpenCode().
// Note bare_flags = [] decodes to a NON-NIL empty slice (FINDING D) — builtinOpenCode sets []string{}.
const opencodeTOML = `name = "opencode"
detect = "opencode"
command = "opencode"
subcommand = ["run"]
prompt_delivery = "positional"
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = []
output = "raw"
strip_code_fence = true
`

// codexTOML — PRD §12.7 codex VERBATIM EXCEPT two lines revised per the work-item contract +
// external_deps.md §codex (the discrepancy resolution). This is the ONLY intentional deviation set from
// the verbatim PRD codex TOML; decoding it must match builtinCodex().
//
//	(1) prompt_delivery = "stdin"      (§12.7 said "positional"; codex exec reads stdin via "-")
//	(2) bare_flags = ["--sandbox","read-only","--ephemeral"]  (§12.7 had "--ask-for-approval","never" —
//	    NOT a codex exec flag; dropped; --ephemeral added).
const codexTOML = `name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]
prompt_delivery = "stdin"   # REVISED #1 from §12.7 "positional" (codex exec reads stdin via "-")
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--sandbox", "read-only", "--ephemeral"]   # REVISED #2: dropped --ask-for-approval; added --ephemeral
output = "raw"
strip_code_fence = true
`

// cursorTOML — PRD §12.7 cursor VERBATIM (no revision). Decoding it must match builtinCursor().
// Note: detect/command = "agent" (≠ name "cursor"); subcommand = [] decodes to a NON-NIL empty slice
// (FINDING D) — builtinCursor sets Subcommand: []string{}.

// qwenCodeTOML — PRD §12.5.2 (experimental=true, # TO CONFIRM FR-D5). Decoding it must match builtinQwenCode().
const qwenCodeTOML = `name = "qwen-code"
detect = "qwen-code"
command = "qwen-code"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "-m"
default_model = "qwen3-coder-plus"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true
experimental = true
`

const agyTOML = `name = "agy"
detect = "agy"
command = "agy"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "-m"
default_model = "gemini-2.5-pro"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true
experimental = true
`

const cursorTOML = `name = "cursor"
detect = "agent"
command = "agent"
subcommand = []
prompt_delivery = "positional"
print_flag = "-p"
model_flag = "--model"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--mode", "ask", "--trust"]
output = "raw"
strip_code_fence = true
`

// ---------------------------------------------------------------------------
// renderArgs — local §12.2 argv-builder (test-only scaffolding, NOT the P1.M2.T4 renderer).
// Faithful port of PRD §12.2 "Command rendering algorithm".
// ---------------------------------------------------------------------------

func renderArgs(m Manifest, provider, model, sys string) []string {
	r := m.Resolve() // safe deref for every pointer
	modelToUse := model
	if modelToUse == "" && r.DefaultModel != nil {
		modelToUse = *r.DefaultModel
	}
	args := []string{}
	args = append(args, r.Subcommand...) // nil-safe no-op
	if *r.ProviderFlag != "" && provider != "" {
		args = append(args, *r.ProviderFlag, provider)
	}
	if *r.ModelFlag != "" && modelToUse != "" {
		args = append(args, *r.ModelFlag, modelToUse)
	}
	if *r.SystemPromptFlag != "" && sys != "" {
		args = append(args, *r.SystemPromptFlag, sys)
	}
	args = append(args, r.BareFlags...)
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag)
	}
	return append([]string{*r.Command}, args...)
}

// ---------------------------------------------------------------------------
// Test 1: KeysAndCount — exactly 7 keys: pi, claude, gemini, opencode, codex, cursor, agy
// ---------------------------------------------------------------------------

func TestBuiltinManifests_KeysAndCount(t *testing.T) {
	m := BuiltinManifests()
	if len(m) != 8 {
		t.Fatalf("BuiltinManifests() returned %d keys, want 8", len(m))
	}
	for _, k := range []string{"pi", "claude", "gemini", "opencode", "codex", "cursor", "agy", "qwen-code"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: NameMatchesKey — .Name equals its map key
// ---------------------------------------------------------------------------

func TestBuiltinManifests_NameMatchesKey(t *testing.T) {
	m := BuiltinManifests()
	for key, manifest := range m {
		if manifest.Name != key {
			t.Errorf("manifest key %q has .Name = %q", key, manifest.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: PiFields — every pi field asserted (explicit-empty + absent-nil pattern)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_PiFields(t *testing.T) {
	m := builtinPi()

	assertStr(t, "Detect", m.Detect, "pi")
	assertStr(t, "Command", m.Command, "pi")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "--model")
	assertStr(t, "DefaultModel", m.DefaultModel, "") // FR-D2: decoupled from any one subscription
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "--system-prompt")
	assertStr(t, "ProviderFlag", m.ProviderFlag, "--provider")

	// BareFlags: 6 tokens in order
	wantBare := []string{
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
	}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}

	// TooledFlags: 5 tokens (bare minus --no-tools — pi has no git allowlist flag)
	wantTooled := []string{
		"--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
	}
	if !reflect.DeepEqual(m.TooledFlags, wantTooled) {
		t.Errorf("TooledFlags = %v, want %v", m.TooledFlags, wantTooled)
	}

	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}

	// Absent fields → nil
	assertNilStr(t, "Subcommand-as-nil", nil) // not a slice field, but check the slice fields
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 4: ClaudeFields — every claude field asserted (explicit-empty ProviderFlag,
//         two "" bare-flag tokens)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_ClaudeFields(t *testing.T) {
	m := builtinClaude()

	assertStr(t, "Detect", m.Detect, "claude")
	assertStr(t, "Command", m.Command, "claude")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "--model")
	assertStr(t, "DefaultModel", m.DefaultModel, "sonnet")
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "--system-prompt")

	// ProviderFlag: NON-NIL empty (§12.4 writes provider_flag = "" # n/a)
	if m.ProviderFlag == nil {
		t.Fatal("ProviderFlag = nil, want non-nil *\"\" (explicit empty)")
	}
	if *m.ProviderFlag != "" {
		t.Errorf("ProviderFlag = %q, want \"\"", *m.ProviderFlag)
	}

	// BareFlags: 5 tokens, TWO of them "" (the value args to --tools and --setting-sources)
	wantBare := []string{
		"--tools", "", "--setting-sources", "", "--no-session-persistence",
	}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}

	// TooledFlags: 5 tokens (tools ENABLED + staging-only git allowlist; --allowed-tools TO CONFIRM at integration)
	wantTooled := []string{
		"--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit",
		"--setting-sources", "", "--no-session-persistence",
	}
	if !reflect.DeepEqual(m.TooledFlags, wantTooled) {
		t.Errorf("TooledFlags = %v, want %v", m.TooledFlags, wantTooled)
	}

	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}

	// Absent fields → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Validate — all built-ins pass Validate()
// ---------------------------------------------------------------------------

func TestBuiltinManifests_Validate(t *testing.T) {
	m := BuiltinManifests()
	for name, manifest := range m {
		if err := manifest.Validate(); err != nil {
			t.Errorf("%s Validate() = %v, want nil", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 6: DecodeParity — built-in == decode of verbatim §12.3–§12.7 TOML
//         (THE byte-faithfulness keystone)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_DecodeParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Manifest
		toml string
	}{
		{"pi", builtinPi(), piTOML},
		{"claude", builtinClaude(), claudeTOML},
		{"gemini", builtinGemini(), geminiTOML},        // geminiTOML = §12.5 with stdin revision
		{"opencode", builtinOpenCode(), opencodeTOML},  // opencodeTOML = verbatim §12.6
		{"codex", builtinCodex(), codexTOML},           // codexTOML = §12.7 codex with BOTH revisions
		{"cursor", builtinCursor(), cursorTOML},        // cursorTOML = verbatim §12.7 cursor
		{"agy", builtinAgy(), agyTOML},                 // agyTOML = §12.5.1 (experimental=true)
		{"qwen-code", builtinQwenCode(), qwenCodeTOML}, // qwenCodeTOML = §12.5.2 (experimental=true)
	} {
		var decoded Manifest
		if err := toml.Unmarshal([]byte(tc.toml), &decoded); err != nil {
			t.Fatalf("%s: decode failed: %v", tc.name, err)
		}
		if !reflect.DeepEqual(tc.got, decoded) {
			t.Errorf("%s: built-in != decoded TOML\n built-in: %+v\n decoded:  %+v", tc.name, tc.got, decoded)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 7a: RenderedCommand_Pi_ShippedDefault — FR-D2: no --model/--provider emitted
//         when both model and provider are empty (the shipped default).
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Pi_ShippedDefault(t *testing.T) {
	argv := renderArgs(builtinPi(), "", "", "<sys>") // model="" → default "" (FR-D2), provider="" → no flag
	want := []string{
		"pi",
		"--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // §12.2: print_flag LAST
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("pi shipped-default argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 7b: RenderedCommand_Pi_PersonalOverride — byte-for-byte commit-pi check
//         with EXPLICIT model+provider (FR-D2: the old default is now an override).
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride(t *testing.T) {
	argv := renderArgs(builtinPi(), "zai", "glm-5-turbo", "<sys>") // explicit personal override
	want := []string{
		"pi", "--provider", "zai",
		"--model", "glm-5-turbo",
		"--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // §12.2: print_flag LAST (matches commit-pi)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("pi personal-override argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 8: FreshEachCall — no shared mutable state across calls (design call #4)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_FreshEachCall(t *testing.T) {
	a := BuiltinManifests()
	b := BuiltinManifests()

	// Mutate a's pi BareFlags backing array in place.
	if len(a["pi"].BareFlags) > 0 {
		a["pi"].BareFlags[0] = "MUTATED"
	}

	// b's pi must be unaffected.
	wantBare := []string{
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
	}
	if !reflect.DeepEqual(b["pi"].BareFlags, wantBare) {
		t.Errorf("BuiltinManifests() shares state across calls (b corrupted by a): got %v", b["pi"].BareFlags)
	}
}

// ---------------------------------------------------------------------------
// Test 9: GeminiFields — every gemini field asserted (stdin revision + explicit-empty + absent-nil)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_GeminiFields(t *testing.T) {
	m := builtinGemini()
	assertStr(t, "Detect", m.Detect, "gemini")
	assertStr(t, "Command", m.Command, "gemini")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin") // REVISED from §12.5 "positional"
	assertStr(t, "PrintFlag", m.PrintFlag, "")                // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "gemini-2.5-pro")
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	wantBare := []string{"--approval-mode", "default"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 10: OpenCodeFields — every opencode field asserted (Subcommand=["run"], NON-NIL-empty BareFlags)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_OpenCodeFields(t *testing.T) {
	m := builtinOpenCode()
	assertStr(t, "Detect", m.Detect, "opencode")
	assertStr(t, "Command", m.Command, "opencode")
	wantSub := []string{"run"}
	if !reflect.DeepEqual(m.Subcommand, wantSub) {
		t.Errorf("Subcommand = %v, want %v", m.Subcommand, wantSub)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "positional")
	assertStr(t, "PrintFlag", m.PrintFlag, "") // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "")         // NON-NIL explicit empty (user must set)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	// BareFlags: §12.6 writes bare_flags = [] → NON-NIL empty slice (FINDING D). Assert BOTH non-nil AND len 0.
	if m.BareFlags == nil {
		t.Fatal("BareFlags = nil, want NON-NIL empty []string{} (§12.6 bare_flags = [] per FINDING D)")
	}
	if len(m.BareFlags) != 0 {
		t.Errorf("BareFlags = %v, want empty", m.BareFlags)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 11: RenderedCommand_Gemini — stdin delivery: argv has NO payload (piped)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Gemini(t *testing.T) {
	argv := renderArgs(builtinGemini(), "", "", "<sys>") // model="" → default gemini-2.5-pro
	want := []string{
		"gemini", "-m", "gemini-2.5-pro",
		"--approval-mode", "default",
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin (NOT in argv). No print/sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("gemini rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 12: RenderedCommand_OpenCode — positional delivery: payload IS trailing arg
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_OpenCode(t *testing.T) {
	flags := renderArgs(builtinOpenCode(), "", "anthropic/claude-sonnet-4", "") // explicit model (default is "")
	argv := append(flags, "<sys>\n\n<payload>")                                 // positional: payload appended per §12.2
	want := []string{
		"opencode", "run", // command + subcommand
		"-m", "anthropic/claude-sonnet-4", // model_flag + user-set model
		"<sys>\n\n<payload>", // positional payload (sys prepended — no sys flag on `run`)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("opencode rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 13: CodexFields — every codex field asserted (TWO revisions + explicit-empty + absent-nil)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_CodexFields(t *testing.T) {
	m := builtinCodex()
	assertStr(t, "Detect", m.Detect, "codex")
	assertStr(t, "Command", m.Command, "codex")
	wantSub := []string{"exec"}
	if !reflect.DeepEqual(m.Subcommand, wantSub) {
		t.Errorf("Subcommand = %v, want %v", m.Subcommand, wantSub)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin") // REVISED #1 from §12.7 "positional"
	assertStr(t, "PrintFlag", m.PrintFlag, "")                // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "")              // NON-NIL explicit empty (model from ~/.codex/config.toml)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "")      // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")              // NON-NIL explicit empty
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")              // NON-NIL explicit empty
	wantBare := []string{"--sandbox", "read-only", "--ephemeral"} // REVISED #2
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 14: CursorFields — every cursor field asserted (Detect/Command="agent" ≠ Name,
//          NON-NIL-EMPTY Subcommand, explicit-empty + absent-nil)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_CursorFields(t *testing.T) {
	m := builtinCursor()

	if m.Name != "cursor" {
		t.Errorf("Name = %q, want %q", m.Name, "cursor")
	}
	assertStr(t, "Detect", m.Detect, "agent") // §12.7 detect = "agent" — the binary is `agent` (≠ Name "cursor")
	assertStr(t, "Command", m.Command, "agent")
	// Subcommand: §12.7 writes subcommand = [] → NON-NIL empty slice (FINDING D). Assert BOTH non-nil AND len 0.
	if m.Subcommand == nil {
		t.Fatal("Subcommand = nil, want NON-NIL empty []string{} (§12.7 subcommand = [] per FINDING D)")
	}
	if len(m.Subcommand) != 0 {
		t.Errorf("Subcommand = %v, want empty", m.Subcommand)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "positional")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "--model")
	assertStr(t, "DefaultModel", m.DefaultModel, "")         // NON-NIL explicit empty (user must set)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	wantBare := []string{"--mode", "ask", "--trust"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 15: RenderedCommand_Codex — stdin delivery: argv has NO payload (piped via "-");
//         sys prepended to stdin payload; no print/sys/provider flag.
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Codex(t *testing.T) {
	argv := renderArgs(builtinCodex(), "", "gpt-5", "<sys>") // model explicit (default is "")
	want := []string{
		"codex", "exec", // command + subcommand
		"-m", "gpt-5", // model_flag + user-set model
		"--sandbox", "read-only", "--ephemeral", // REVISED bare_flags (read-only + session-clean)
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin via "-" (NOT in argv). No print/sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("codex rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 16: RenderedCommand_Cursor — positional delivery: payload IS the trailing positional arg (§12.2).
// NOTE: this is the §12.2 ALGORITHM order (renderArgs). §12.7's illustrative "Rendered" block shows
// `agent -p --mode ask --trust --model gpt-5 "<…>"` (different token order). Same tokens; cursor parses
// flags in any order → identical semantics. §12.2 is authoritative (the real P1.M2.T4 renderer).
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Cursor(t *testing.T) {
	flags := renderArgs(builtinCursor(), "", "gpt-5", "") // model explicit (default is "")
	argv := append(flags, "<sys>\n\n<payload>")           // positional: payload appended per §12.2
	want := []string{
		"agent",            // command (§12.7 command = "agent")
		"--model", "gpt-5", // model_flag + user-set model
		"--mode", "ask", "--trust", // bare_flags (read-only + skip ws-trust)
		"-p",                 // print_flag LAST per §12.2
		"<sys>\n\n<payload>", // positional payload (sys prepended — no sys flag on agent)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("cursor rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 17: AgyFields — every agy field asserted (experimental=true, nil tooled_flags)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_AgyFields(t *testing.T) {
	m := builtinAgy()
	assertStr(t, "Detect", m.Detect, "agy")
	assertStr(t, "Command", m.Command, "agy")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p") // NON-NIL (agy HAS a -p print flag per §12.5.1)
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "gemini-2.5-pro")
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	wantBare := []string{"--approval-mode", "default"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Experimental: NON-NIL true (ships experimental per §12.5.1.1)
	if m.Experimental == nil || *m.Experimental != true {
		t.Errorf("Experimental = %v, want non-nil true", m.Experimental)
	}
	// TooledFlags: nil — agy cannot stager until §12.5.1.1 item 4 is verified
	if m.TooledFlags != nil {
		t.Errorf("TooledFlags = %v, want nil", m.TooledFlags)
	}
	// Absent → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test 18: RenderedCommand_Agy — stdin delivery: argv has NO payload (piped);
//         sys prepended to stdin payload; print_flag "-p" last.
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Agy(t *testing.T) {
	argv := renderArgs(builtinAgy(), "", "", "<sys>") // model="" → default gemini-2.5-pro
	want := []string{
		"agy", "-m", "gemini-2.5-pro",
		"--approval-mode", "default",
		"-p", // print_flag LAST per §12.2 (agy has -p unlike gemini)
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin (NOT in argv). No sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("agy rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test: QwenCodeFields — every qwen-code field asserted (experimental=true, nil tooled_flags)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_QwenCodeFields(t *testing.T) {
	m := builtinQwenCode()
	assertStr(t, "Detect", m.Detect, "qwen-code")
	assertStr(t, "Command", m.Command, "qwen-code")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "qwen3-coder-plus") // # TO CONFIRM FR-D5
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "")         // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")                 // NON-NIL explicit empty (single-backend)
	wantBare := []string{"--approval-mode", "default"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	if m.Experimental == nil || *m.Experimental != true {
		t.Errorf("Experimental = %v, want non-nil true (§12.5.2 ships experimental)", m.Experimental)
	}
	if m.TooledFlags != nil {
		t.Errorf("TooledFlags = %v, want nil (cannot stager until verified)", m.TooledFlags)
	}
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// Test: RenderedCommand_QwenCode — stdin delivery: argv has NO payload (piped);
//       sys prepended to stdin payload; print_flag "-p" last.
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_QwenCode(t *testing.T) {
	argv := renderArgs(builtinQwenCode(), "", "", "<sys>") // model="" → default qwen3-coder-plus
	want := []string{
		"qwen-code", "-m", "qwen3-coder-plus",
		"--approval-mode", "default",
		"-p", // print_flag LAST per §12.2
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin (NOT in argv). No sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("qwen-code rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// ---------------------------------------------------------------------------
// Test 19: RenderedCommand_Pi_Tooled — pi rendered in RenderTooled mode (stager role).
//         Uses the REAL Render (not the bare-only renderArgs helper). Proves pi's
//         TooledFlags are non-empty (no error) and the tooled argv is bare MINUS --no-tools.
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Pi_Tooled(t *testing.T) {
	spec, err := builtinPi().Render("zai/glm-5-turbo", "<sys>", "<user>", "off", RenderTooled)
	if err != nil {
		t.Fatalf("pi tooled render error: %v", err)
	}
	want := []string{
		"--provider", "zai",
		"--model", "glm-5-turbo",
		"--system-prompt", "<sys>",
		"--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // print_flag LAST; NO --no-tools (tools on)
	}
	if !reflect.DeepEqual(spec.Args, want) {
		t.Errorf("pi tooled Args:\n got %v\nwant %v", spec.Args, want)
	}
	if spec.Stdin != "<user>" { // sys via --system-prompt flag → only user payload on stdin
		t.Errorf("pi tooled Stdin = %q, want %q", spec.Stdin, "<user>")
	}
}

// ---------------------------------------------------------------------------
// Test 20: RenderedCommand_Claude_Tooled — claude rendered in RenderTooled mode (stager role).
//         Uses the REAL Render. Proves claude's TooledFlags are non-empty and the tooled
//         argv uses --allowed-tools (NOT --tools "").
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Claude_Tooled(t *testing.T) {
	spec, err := builtinClaude().Render("sonnet", "<sys>", "<user>", "off", RenderTooled)
	if err != nil {
		t.Fatalf("claude tooled render error: %v", err)
	}
	want := []string{
		"--model", "sonnet",
		"--system-prompt", "<sys>",
		"--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit", // staging-only allowlist (NOT --tools "")
		"--setting-sources", "",
		"--no-session-persistence",
		"-p",
	}
	if !reflect.DeepEqual(spec.Args, want) {
		t.Errorf("claude tooled Args:\n got %v\nwant %v", spec.Args, want)
	}
	if spec.Stdin != "<user>" {
		t.Errorf("claude tooled Stdin = %q, want %q", spec.Stdin, "<user>")
	}
}
