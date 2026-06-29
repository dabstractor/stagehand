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
default_model = "glm-5-turbo"
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
// Test 1: KeysAndCount — exactly 4 keys: pi, claude, gemini, opencode
// ---------------------------------------------------------------------------

func TestBuiltinManifests_KeysAndCount(t *testing.T) {
	m := BuiltinManifests()
	if len(m) != 4 {
		t.Fatalf("BuiltinManifests() returned %d keys, want 4", len(m))
	}
	for _, k := range []string{"pi", "claude", "gemini", "opencode"} {
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
	assertStr(t, "DefaultModel", m.DefaultModel, "glm-5-turbo")
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "--system-prompt")
	assertStr(t, "ProviderFlag", m.ProviderFlag, "--provider")

	// DefaultProvider: NON-NIL empty (§12.3 writes default_provider = "")
	if m.DefaultProvider == nil {
		t.Fatal("DefaultProvider = nil, want non-nil *\"\" (explicit empty)")
	}
	if *m.DefaultProvider != "" {
		t.Errorf("DefaultProvider = %q, want \"\"", *m.DefaultProvider)
	}

	// BareFlags: 6 tokens in order
	wantBare := []string{
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
	}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
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
//         nil DefaultProvider, two "" bare-flag tokens)
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

	// DefaultProvider: NIL (§12.4 OMITS the key)
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)

	// BareFlags: 5 tokens, TWO of them "" (the value args to --tools and --setting-sources)
	wantBare := []string{
		"--tools", "", "--setting-sources", "", "--no-session-persistence",
	}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
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
// Test 5: Validate — both built-ins pass Validate()
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
// Test 6: DecodeParity — built-in == decode of verbatim §12.3/§12.4/§12.5/§12.6 TOML
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
		{"gemini", builtinGemini(), geminiTOML},       // geminiTOML = §12.5 with stdin revision
		{"opencode", builtinOpenCode(), opencodeTOML}, // opencodeTOML = verbatim §12.6
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
// Test 7: RenderedCommand_Pi_MatchesCommitPi — byte-for-byte commit-pi check
//         (THE work-item headline requirement)
// ---------------------------------------------------------------------------

func TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi(t *testing.T) {
	argv := renderArgs(builtinPi(), "zai", "", "<sys>") // model="" → default glm-5-turbo
	want := []string{
		"pi", "--provider", "zai",
		"--model", "glm-5-turbo",
		"--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // §12.2: print_flag LAST (matches §12.3 + commit-pi)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("pi rendered argv:\n got %v\nwant %v", argv, want)
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
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT in §12.5 → nil
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
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT → nil
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
