package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// piManifestTOML is the §12.3 pi built-in provider manifest, used as the primary test fixture.
const piManifestTOML = `name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "glm-5-turbo"
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"
bare_flags = ["--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"]
output = "raw"
strip_code_fence = true
`

// ---------------------------------------------------------------------------
// TestUnmarshal_FullManifest — decode the §12.3 pi manifest (all fields present)
// ---------------------------------------------------------------------------

func TestUnmarshal_FullManifest(t *testing.T) {
	var m Manifest
	if err := toml.Unmarshal([]byte(piManifestTOML), &m); err != nil {
		t.Fatalf("unmarshal pi manifest: %v", err)
	}

	// Name (plain string)
	if m.Name != "pi" {
		t.Errorf("Name = %q, want %q", m.Name, "pi")
	}

	// Required scalars (non-nil)
	assertStr(t, "Detect", m.Detect, "pi")
	assertStr(t, "Command", m.Command, "pi")

	// Slices
	// Subcommand is absent in the pi TOML → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}

	// Prompt delivery
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	// PromptFlag absent → nil
	if m.PromptFlag != nil {
		t.Errorf("PromptFlag = %q, want nil", *m.PromptFlag)
	}

	// Print mode
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")

	// Model
	assertStr(t, "ModelFlag", m.ModelFlag, "--model")
	assertStr(t, "DefaultModel", m.DefaultModel, "glm-5-turbo")

	// System prompt
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "--system-prompt")

	// Sub-provider
	assertStr(t, "ProviderFlag", m.ProviderFlag, "--provider")
	// Bare flags
	wantBare := []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"}
	if len(m.BareFlags) != len(wantBare) {
		t.Fatalf("BareFlags len = %d, want %d", len(m.BareFlags), len(wantBare))
	}
	for i, v := range wantBare {
		if m.BareFlags[i] != v {
			t.Errorf("BareFlags[%d] = %q, want %q", i, m.BareFlags[i], v)
		}
	}

	// Output
	assertStr(t, "Output", m.Output, "raw")
	// JsonField absent → nil
	if m.JsonField != nil {
		t.Errorf("JsonField = %q, want nil", *m.JsonField)
	}

	// StripCodeFence
	if m.StripCodeFence == nil {
		t.Fatal("StripCodeFence = nil, want non-nil")
	}
	if *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want true", *m.StripCodeFence)
	}

	// RetryInstruction absent → nil
	if m.RetryInstruction != nil {
		t.Errorf("RetryInstruction = %q, want nil", *m.RetryInstruction)
	}

	// Env absent → nil
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// TestUnmarshal_PartialManifest_NilPointers — absent keys → nil (FINDING C)
// ---------------------------------------------------------------------------

func TestUnmarshal_PartialManifest_NilPointers(t *testing.T) {
	tomlSrc := `name = "x"
print_flag = "-p"
bare_flags = ["a"]
`
	var m Manifest
	if err := toml.Unmarshal([]byte(tomlSrc), &m); err != nil {
		t.Fatalf("unmarshal partial: %v", err)
	}

	if m.Name != "x" {
		t.Errorf("Name = %q, want %q", m.Name, "x")
	}

	// Absent scalar fields → nil
	assertNilStr(t, "Detect", m.Detect)
	assertNilStr(t, "Command", m.Command)
	assertNilStr(t, "PromptDelivery", m.PromptDelivery)
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "ModelFlag", m.ModelFlag)
	assertNilStr(t, "DefaultModel", m.DefaultModel)
	assertNilStr(t, "SystemPromptFlag", m.SystemPromptFlag)
	assertNilStr(t, "ProviderFlag", m.ProviderFlag)
	assertNilStr(t, "Output", m.Output)
	assertNilStr(t, "JsonField", m.JsonField)
	if m.StripCodeFence != nil {
		t.Errorf("StripCodeFence = %v, want nil", *m.StripCodeFence)
	}
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)

	// Present fields → non-nil
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")

	// BareFlags present → non-nil slice
	if m.BareFlags == nil || len(m.BareFlags) != 1 || m.BareFlags[0] != "a" {
		t.Errorf("BareFlags = %v, want [\"a\"]", m.BareFlags)
	}

	// Absent slices/map → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// ---------------------------------------------------------------------------
// TestUnmarshal_ExplicitZeroNonNil — present zero values → non-nil (FINDING D)
// ---------------------------------------------------------------------------

func TestUnmarshal_ExplicitZeroNonNil(t *testing.T) {
	tomlSrc := `name = "z"
print_flag = ""
strip_code_fence = false
subcommand = []
`
	var m Manifest
	if err := toml.Unmarshal([]byte(tomlSrc), &m); err != nil {
		t.Fatalf("unmarshal explicit-zero: %v", err)
	}

	// print_flag="" → non-nil, empty
	if m.PrintFlag == nil {
		t.Fatal("PrintFlag = nil, want non-nil")
	}
	if *m.PrintFlag != "" {
		t.Errorf("PrintFlag = %q, want \"\"", *m.PrintFlag)
	}

	// strip_code_fence=false → non-nil, false
	if m.StripCodeFence == nil {
		t.Fatal("StripCodeFence = nil, want non-nil")
	}
	if *m.StripCodeFence != false {
		t.Errorf("StripCodeFence = %v, want false", *m.StripCodeFence)
	}

	// subcommand=[] → non-nil empty slice
	if m.Subcommand == nil {
		t.Fatal("Subcommand = nil, want non-nil (empty)")
	}
	if len(m.Subcommand) != 0 {
		t.Errorf("Subcommand = %v, want []", m.Subcommand)
	}
}

// ---------------------------------------------------------------------------
// TestMarshal_OmitsNilPointers — nil pointers omitted on marshal (FINDING A)
// ---------------------------------------------------------------------------

func TestMarshal_OmitsNilPointers(t *testing.T) {
	m := Manifest{Name: "gemini"}
	data, err := toml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)

	// Name must be present
	if !strings.Contains(out, "name") {
		t.Error("marshaled output missing 'name'")
	}

	// Nil pointer fields must be OMITTED (FINDING A)
	// Use a prefix check on each line to avoid false positives (e.g. "command" in "subcommand").
	for _, key := range []string{
		"command", "print_flag", "strip_code_fence",
		"prompt_delivery", "model_flag", "detect",
		"system_prompt_flag", "provider_flag", "default_provider",
		"json_field", "retry_instruction", "prompt_flag",
		"default_model", "output",
	} {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, key+" ") || strings.HasPrefix(line, key+"=") {
				t.Errorf("marshaled output contains nil-pointer field %q (should be omitted)", key)
			}
		}
	}

	// nil slices MAY appear as `[]` (FINDING B) — do not assert their absence
	// (subcommand = [] / bare_flags = [] are acceptable).
}

// ---------------------------------------------------------------------------
// TestValidate_* — valid + each failure mode
// ---------------------------------------------------------------------------

func TestValidate_ValidManifest_Passes(t *testing.T) {
	var m Manifest
	if err := toml.Unmarshal([]byte(piManifestTOML), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("pi manifest Validate: %v", err)
	}
}

func TestValidate_MissingName_Errors(t *testing.T) {
	m := Manifest{Command: strPtr("x")}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for empty Name")
	} else if !strings.Contains(err.Error(), "name") {
		t.Errorf("error %q does not mention 'name'", err.Error())
	}
}

func TestValidate_MissingCommand_Errors(t *testing.T) {
	// Command nil
	m := Manifest{Name: "x"}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for nil Command")
	} else if !strings.Contains(err.Error(), "command") {
		t.Errorf("error %q does not mention 'command'", err.Error())
	}

	// Command=&"" (non-nil but empty)
	m2 := Manifest{Name: "x", Command: strPtr("")}
	if err := m2.Validate(); err == nil {
		t.Fatal("expected error for empty *Command")
	} else if !strings.Contains(err.Error(), "command") {
		t.Errorf("error %q does not mention 'command'", err.Error())
	}
}

func TestValidate_BadPromptDelivery_Errors(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x"), PromptDelivery: strPtr("weird")}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for bad PromptDelivery")
	} else if !strings.Contains(err.Error(), "prompt_delivery") {
		t.Errorf("error %q does not mention 'prompt_delivery'", err.Error())
	}
}

func TestValidate_BadOutput_Errors(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x"), Output: strPtr("xml")}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for bad Output")
	} else if !strings.Contains(err.Error(), "output") {
		t.Errorf("error %q does not mention 'output'", err.Error())
	}
}

func TestValidate_NilEnumsAreOK(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x")}
	if err := m.Validate(); err != nil {
		t.Errorf("nil enums should be OK: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestDetectCommand_* — Detect > Command > ""
// ---------------------------------------------------------------------------

func TestDetectCommand_ReturnsDetectWhenSet(t *testing.T) {
	m := Manifest{Detect: strPtr("mydetect"), Command: strPtr("mycmd")}
	if got := m.DetectCommand(); got != "mydetect" {
		t.Errorf("DetectCommand = %q, want %q", got, "mydetect")
	}
}

func TestDetectCommand_FallsBackToCommandWhenDetectEmpty(t *testing.T) {
	m := Manifest{Detect: strPtr(""), Command: strPtr("mycmd")}
	if got := m.DetectCommand(); got != "mycmd" {
		t.Errorf("DetectCommand = %q, want %q", got, "mycmd")
	}
}

func TestDetectCommand_FallsBackToCommandWhenDetectNil(t *testing.T) {
	m := Manifest{Command: strPtr("mycmd")}
	if got := m.DetectCommand(); got != "mycmd" {
		t.Errorf("DetectCommand = %q, want %q", got, "mycmd")
	}
}

func TestDetectCommand_EmptyWhenBothNil(t *testing.T) {
	m := Manifest{}
	if got := m.DetectCommand(); got != "" {
		t.Errorf("DetectCommand = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// TestResolve_* — defaults applied, explicit zeros preserved, non-nil guarantee
// ---------------------------------------------------------------------------

func TestResolve_AppliesDefaultsToNilOptionals(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x")}
	r := m.Resolve()

	assertStr(t, "PromptDelivery", r.PromptDelivery, DefaultPromptDelivery)
	assertStr(t, "Output", r.Output, DefaultOutput)
	if r.StripCodeFence == nil || *r.StripCodeFence != DefaultStripCodeFence {
		t.Errorf("StripCodeFence = %v, want %v", r.StripCodeFence, DefaultStripCodeFence)
	}
	assertStr(t, "RetryInstruction", r.RetryInstruction, DefaultRetryInstruction)
	if r.Experimental == nil || *r.Experimental != false {
		t.Errorf("Experimental = %v, want non-nil *false (default non-experimental)", r.Experimental)
	}
}

func TestResolve_PreservesExplicitValues(t *testing.T) {
	m := Manifest{
		Name:           "x",
		Command:        strPtr("x"),
		StripCodeFence: boolPtr(false), // explicit false — must NOT become the true default
		Output:         strPtr("json"), // explicit json — must NOT become the raw default
		Experimental:   boolPtr(true),  // explicit true — must survive Resolve
	}
	r := m.Resolve()

	if r.StripCodeFence == nil || *r.StripCodeFence != false {
		t.Errorf("Resolve clobbered explicit strip_code_fence=false (got %v)", r.StripCodeFence)
	}
	if r.Output == nil || *r.Output != "json" {
		t.Errorf("Resolve clobbered explicit output=json (got %v)", r.Output)
	}
	if r.Experimental == nil || *r.Experimental != true {
		t.Errorf("Experimental = %v, want non-nil *true (explicit value preserved)", r.Experimental)
	}
}

func TestResolve_OptionalStringsBecomeEmpty(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x")}
	r := m.Resolve()

	for _, field := range []struct {
		name string
		ptr  *string
	}{
		{"Detect", r.Detect},
		{"PromptFlag", r.PromptFlag},
		{"PrintFlag", r.PrintFlag},
		{"ModelFlag", r.ModelFlag},
		{"DefaultModel", r.DefaultModel},
		{"SystemPromptFlag", r.SystemPromptFlag},
		{"ProviderFlag", r.ProviderFlag},
		{"JsonField", r.JsonField},
	} {
		if field.ptr == nil {
			t.Errorf("Resolve left %s nil, want non-nil *\"\"", field.name)
		} else if *field.ptr != "" {
			t.Errorf("Resolve set %s = %q, want \"\"", field.name, *field.ptr)
		}
	}
}

func TestResolve_SlicesLeftNil(t *testing.T) {
	m := Manifest{Name: "x", Command: strPtr("x")}
	r := m.Resolve()

	if r.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil (left as-is)", r.Subcommand)
	}
	if r.BareFlags != nil {
		t.Errorf("BareFlags = %v, want nil (left as-is)", r.BareFlags)
	}
	if r.TooledFlags != nil {
		t.Errorf("TooledFlags = %v, want nil (left as-is, slice regime)", r.TooledFlags)
	}
	if r.Env != nil {
		t.Errorf("Env = %v, want nil (left as-is)", r.Env)
	}
}

func TestResolve_CommandLeftNilIfAbsent(t *testing.T) {
	m := Manifest{Name: "x"}
	r := m.Resolve()

	if r.Command != nil {
		t.Errorf("Command = %q, want nil (not fabricated)", *r.Command)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertStr(t *testing.T, field string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want non-nil %q", field, want)
	} else if *got != want {
		t.Errorf("%s = %q, want %q", field, *got, want)
	}
}

func assertNilStr(t *testing.T, field string, got *string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %q, want nil", field, *got)
	}
}

// Verify the 4 Default* constants at compile time.
var _ = fmt.Sprintf("%s %s %v %s", DefaultPromptDelivery, DefaultOutput, DefaultStripCodeFence, DefaultRetryInstruction)
