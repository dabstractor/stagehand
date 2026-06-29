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
// Test 1: KeysAndCount — exactly 2 keys, "pi" and "claude"
// ---------------------------------------------------------------------------

func TestBuiltinManifests_KeysAndCount(t *testing.T) {
	m := BuiltinManifests()
	if len(m) != 2 {
		t.Fatalf("BuiltinManifests() returned %d keys, want 2", len(m))
	}
	if _, ok := m["pi"]; !ok {
		t.Error(`missing key "pi"`)
	}
	if _, ok := m["claude"]; !ok {
		t.Error(`missing key "claude"`)
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
// Test 6: DecodeParity — built-in == decode of verbatim §12.3/§12.4 TOML
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
