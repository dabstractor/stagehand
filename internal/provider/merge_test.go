package provider

import (
	"reflect"
	"testing"
)

// sampleBase returns a fully-populated Manifest resembling the pi built-in provider.
// Used as the "built-in" that overrides merge onto in the tests below.
func sampleBase() Manifest {
	return Manifest{
		Name:             "pi",
		Detect:           strPtr("pi"),
		Command:          strPtr("pi"),
		PromptDelivery:   strPtr("stdin"),
		PromptFlag:       strPtr(""),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("glm-5-turbo"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr("--provider"),

		Output:           strPtr("raw"),
		JsonField:        strPtr(""),
		StripCodeFence:   boolPtr(true),
		RetryInstruction: strPtr(""),
		Subcommand:       nil,
		BareFlags:        []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"},
		TooledFlags:      []string{"--allowed-tools", "git:*", "--approval-mode", "auto"},
		Experimental:     boolPtr(true),
		Env:              map[string]string{"A": "1", "B": "2"},
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges — THE §16.1 KEYSTONE
// ---------------------------------------------------------------------------

func TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges(t *testing.T) {
	base := sampleBase()
	override := Manifest{DefaultModel: strPtr("glm-5.2")}
	merged := MergeManifest(base, override)

	// The one overridden field changed.
	if merged.DefaultModel == nil || *merged.DefaultModel != "glm-5.2" {
		t.Errorf("DefaultModel = %v, want \"glm-5.2\"", merged.DefaultModel)
	}

	// Every OTHER scalar pointer field must match base exactly.
	for _, tc := range []struct {
		name string
		got  *string
		want *string
	}{
		{"Detect", merged.Detect, base.Detect},
		{"Command", merged.Command, base.Command},
		{"PromptDelivery", merged.PromptDelivery, base.PromptDelivery},
		{"PromptFlag", merged.PromptFlag, base.PromptFlag},
		{"PrintFlag", merged.PrintFlag, base.PrintFlag},
		{"ModelFlag", merged.ModelFlag, base.ModelFlag},
		{"SystemPromptFlag", merged.SystemPromptFlag, base.SystemPromptFlag},
		{"ProviderFlag", merged.ProviderFlag, base.ProviderFlag},
		{"Output", merged.Output, base.Output},
		{"JsonField", merged.JsonField, base.JsonField},
		{"RetryInstruction", merged.RetryInstruction, base.RetryInstruction},
	} {
		if tc.got == nil && tc.want == nil {
			continue
		}
		if tc.got == nil || tc.want == nil || *tc.got != *tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	// StripCodeFence (*bool) must match base.
	if merged.StripCodeFence == nil || base.StripCodeFence == nil || *merged.StripCodeFence != *base.StripCodeFence {
		t.Errorf("StripCodeFence = %v, want %v", merged.StripCodeFence, base.StripCodeFence)
	}

	// Slices must match base.
	if !reflect.DeepEqual(merged.BareFlags, base.BareFlags) {
		t.Errorf("BareFlags = %v, want %v", merged.BareFlags, base.BareFlags)
	}
	if !reflect.DeepEqual(merged.TooledFlags, base.TooledFlags) {
		t.Errorf("TooledFlags = %v, want %v", merged.TooledFlags, base.TooledFlags)
	}
	if !reflect.DeepEqual(merged.Subcommand, base.Subcommand) {
		t.Errorf("Subcommand = %v, want %v", merged.Subcommand, base.Subcommand)
	}

	// Experimental must match base.
	if merged.Experimental == nil || base.Experimental == nil || *merged.Experimental != *base.Experimental {
		t.Errorf("Experimental = %v, want %v", merged.Experimental, base.Experimental)
	}

	// ListModelsCommand must match base.
	if !reflect.DeepEqual(merged.ListModelsCommand, base.ListModelsCommand) {
		t.Errorf("ListModelsCommand = %v, want %v", merged.ListModelsCommand, base.ListModelsCommand)
	}

	// Env must match base.
	if !reflect.DeepEqual(merged.Env, base.Env) {
		t.Errorf("Env = %v, want %v", merged.Env, base.Env)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_ExplicitZeroPointerWins — payoff of S1's pointer design
// ---------------------------------------------------------------------------

func TestMergeManifest_ExplicitZeroPointerWins(t *testing.T) {
	base := sampleBase() // StripCodeFence=true, PrintFlag="-p"
	merged := MergeManifest(base, Manifest{
		StripCodeFence: boolPtr(false), // explicit false — must NOT inherit base's true
		PrintFlag:      strPtr(""),     // explicit empty — must NOT inherit base's "-p"
		Experimental:   boolPtr(false), // base has true → explicit false must win
	})

	if merged.StripCodeFence == nil || *merged.StripCodeFence != false {
		t.Errorf("explicit strip_code_fence=false lost (got %v)", merged.StripCodeFence)
	}
	if merged.PrintFlag == nil || *merged.PrintFlag != "" {
		t.Errorf("explicit print_flag=\"\" lost (got %v)", merged.PrintFlag)
	}
	if merged.Experimental == nil || *merged.Experimental != false {
		t.Errorf("explicit experimental=false lost (got %v)", merged.Experimental)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_NonEmptySliceReplacesWholesale
// ---------------------------------------------------------------------------

func TestMergeManifest_NonEmptySliceReplacesWholesale(t *testing.T) {
	base := sampleBase()
	override := Manifest{
		BareFlags:         []string{"--x"},
		Subcommand:        []string{"run"},
		ListModelsCommand: []string{"myagent", "list"},
	}
	merged := MergeManifest(base, override)

	if !reflect.DeepEqual(merged.BareFlags, []string{"--x"}) {
		t.Errorf("BareFlags = %v, want [\"--x\"] (wholesale replace)", merged.BareFlags)
	}
	if !reflect.DeepEqual(merged.Subcommand, []string{"run"}) {
		t.Errorf("Subcommand = %v, want [\"run\"] (wholesale replace)", merged.Subcommand)
	}
	if !reflect.DeepEqual(merged.ListModelsCommand, []string{"myagent", "list"}) {
		t.Errorf("ListModelsCommand = %v, want [\"myagent\",\"list\"] (wholesale replace)", merged.ListModelsCommand)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_TooledFlagsReplacedWholesale
// ---------------------------------------------------------------------------

func TestMergeManifest_TooledFlagsReplacedWholesale(t *testing.T) {
	base := sampleBase()
	override := Manifest{TooledFlags: []string{"--yolo"}}
	merged := MergeManifest(base, override)

	if !reflect.DeepEqual(merged.TooledFlags, []string{"--yolo"}) {
		t.Errorf("TooledFlags = %v, want [\"--yolo\"] (wholesale replace)", merged.TooledFlags)
	}
	// The OTHER flag slice must be untouched.
	if !reflect.DeepEqual(merged.BareFlags, base.BareFlags) {
		t.Errorf("BareFlags = %v, want %v (untouched)", merged.BareFlags, base.BareFlags)
	}
	if !reflect.DeepEqual(merged.Subcommand, base.Subcommand) {
		t.Errorf("Subcommand = %v, want %v (untouched)", merged.Subcommand, base.Subcommand)
	}
	if !reflect.DeepEqual(merged.ListModelsCommand, base.ListModelsCommand) {
		t.Errorf("ListModelsCommand = %v, want %v (untouched)", merged.ListModelsCommand, base.ListModelsCommand)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_EmptyOrNilSlicePreservesBase
// ---------------------------------------------------------------------------

func TestMergeManifest_EmptyOrNilSlicePreservesBase(t *testing.T) {
	base := sampleBase()

	// (a) nil override slice → keep base
	mergedNil := MergeManifest(base, Manifest{})
	if !reflect.DeepEqual(mergedNil.BareFlags, base.BareFlags) {
		t.Errorf("nil override: BareFlags = %v, want %v", mergedNil.BareFlags, base.BareFlags)
	}
	if !reflect.DeepEqual(mergedNil.Subcommand, base.Subcommand) {
		t.Errorf("nil override: Subcommand = %v, want %v", mergedNil.Subcommand, base.Subcommand)
	}
	if !reflect.DeepEqual(mergedNil.TooledFlags, base.TooledFlags) {
		t.Errorf("nil override: TooledFlags = %v, want %v", mergedNil.TooledFlags, base.TooledFlags)
	}
	if !reflect.DeepEqual(mergedNil.ListModelsCommand, base.ListModelsCommand) {
		t.Errorf("nil override: ListModelsCommand = %v, want %v", mergedNil.ListModelsCommand, base.ListModelsCommand)
	}

	// (b) non-nil empty slice → treated as "not overridden" (keep base)
	mergedEmpty := MergeManifest(base, Manifest{
		Subcommand:        []string{}, // non-nil but empty
		TooledFlags:       []string{},
		ListModelsCommand: []string{},
	})
	if !reflect.DeepEqual(mergedEmpty.Subcommand, base.Subcommand) {
		t.Errorf("empty override: Subcommand = %v, want %v", mergedEmpty.Subcommand, base.Subcommand)
	}
	if !reflect.DeepEqual(mergedEmpty.TooledFlags, base.TooledFlags) {
		t.Errorf("empty override: TooledFlags = %v, want %v", mergedEmpty.TooledFlags, base.TooledFlags)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_EnvKeyByKeyMerge
// ---------------------------------------------------------------------------

func TestMergeManifest_EnvKeyByKeyMerge(t *testing.T) {
	base := sampleBase() // Env = {"A":"1", "B":"2"}
	override := Manifest{Env: map[string]string{"B": "3", "C": "4"}}
	merged := MergeManifest(base, override)

	want := map[string]string{"A": "1", "B": "3", "C": "4"}
	if !reflect.DeepEqual(merged.Env, want) {
		t.Errorf("Env = %v, want %v", merged.Env, want)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_EnvNilOverridePreservesBase
// ---------------------------------------------------------------------------

func TestMergeManifest_EnvNilOverridePreservesBase(t *testing.T) {
	base := sampleBase()                      // Env = {"A":"1", "B":"2"}
	merged := MergeManifest(base, Manifest{}) // Env nil

	if !reflect.DeepEqual(merged.Env, base.Env) {
		t.Errorf("Env = %v, want %v", merged.Env, base.Env)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_DoesNotMutateInputs — THE ALIASING GUARD
// ---------------------------------------------------------------------------

func TestMergeManifest_DoesNotMutateInputs(t *testing.T) {
	base := sampleBase()

	// Snapshot base.Env and base.BareFlags before the call.
	envBefore := map[string]string{}
	for k, v := range base.Env {
		envBefore[k] = v
	}
	bareBefore := append([]string(nil), base.BareFlags...)
	tooledBefore := append([]string(nil), base.TooledFlags...)

	override := Manifest{Env: map[string]string{"X": "9", "B": "overridden"}}
	_ = MergeManifest(base, override) // discard result; we care about base's integrity

	if !reflect.DeepEqual(base.Env, envBefore) {
		t.Errorf("MergeManifest mutated base.Env (aliasing bug): got %v, want %v", base.Env, envBefore)
	}
	if !reflect.DeepEqual(base.BareFlags, bareBefore) {
		t.Errorf("MergeManifest mutated base.BareFlags: got %v, want %v", base.BareFlags, bareBefore)
	}
	if !reflect.DeepEqual(base.TooledFlags, tooledBefore) {
		t.Errorf("MergeManifest mutated base.TooledFlags: got %v, want %v", base.TooledFlags, tooledBefore)
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_EmptyOverrideIsIdentity
// ---------------------------------------------------------------------------

func TestMergeManifest_EmptyOverrideIsIdentity(t *testing.T) {
	base := sampleBase()
	merged := MergeManifest(base, Manifest{})

	// reflect.DeepEqual compares pointer targets, which works here since both sides share
	// the same pointers in the identity case (no override applied).
	if !reflect.DeepEqual(merged, base) {
		t.Errorf("MergeManifest(base, Manifest{}) != base")
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_NamePreservedFromBase
// ---------------------------------------------------------------------------

func TestMergeManifest_NamePreservedFromBase(t *testing.T) {
	base := sampleBase() // Name = "pi"

	// override with a non-empty Name (should still be ignored)
	merged := MergeManifest(base, Manifest{Name: "ignored"})
	if merged.Name != "pi" {
		t.Errorf("Name = %q, want %q (base.Name preserved)", merged.Name, "pi")
	}

	// override with empty Name (the typical case)
	merged2 := MergeManifest(base, Manifest{})
	if merged2.Name != "pi" {
		t.Errorf("Name = %q, want %q (base.Name preserved)", merged2.Name, "pi")
	}
}

// ---------------------------------------------------------------------------
// TestMergeManifest_MergedResultValidates — S1↔S2 COMPOSITION
// ---------------------------------------------------------------------------

func TestMergeManifest_MergedResultValidates(t *testing.T) {
	base := sampleBase() // a complete, valid manifest

	// Sanity: base itself validates.
	if err := base.Validate(); err != nil {
		t.Fatalf("sampleBase() should validate: %v", err)
	}

	// Merged result with a partial override must still validate.
	merged := MergeManifest(base, Manifest{DefaultModel: strPtr("glm-5.2")})
	if err := merged.Validate(); err != nil {
		t.Errorf("merged manifest should validate: %v", err)
	}
}
