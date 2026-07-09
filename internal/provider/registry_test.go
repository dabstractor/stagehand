package provider

import (
	"os/exec"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// 1. preferredBuiltins sanity
// ---------------------------------------------------------------------------

func TestPreferredBuiltins_MatchesBuiltinKeys(t *testing.T) {
	bk := BuiltinManifests()
	set := map[string]struct{}{}
	for _, n := range preferredBuiltins {
		set[n] = struct{}{}
	}
	if len(set) != len(bk) {
		t.Errorf("preferredBuiltins has %d, builtins %d", len(set), len(bk))
	}
	for k := range bk {
		if _, ok := set[k]; !ok {
			t.Errorf("built-in %q not in preferredBuiltins", k)
		}
	}
	if len(preferredBuiltins) == 0 || preferredBuiltins[0] != "pi" {
		t.Errorf("pi must be first; got %v", preferredBuiltins)
	}
	// Exact FR-D1 order assertion (§9.16 FR-D1: open/self-hostable first, closed last).
	wantOrder := []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}
	if !reflect.DeepEqual(preferredBuiltins, wantOrder) {
		t.Errorf("preferredBuiltins order = %v, want FR-D1 %v", preferredBuiltins, wantOrder)
	}
}

// ---------------------------------------------------------------------------
// 2. NewRegistry with no overrides → exactly the 7 built-ins, each Get-able
// ---------------------------------------------------------------------------

func TestNewRegistry_NoOverrides_HasAllBuiltins(t *testing.T) {
	r := NewRegistry(nil)
	if got := len(r.manifests); got != len(BuiltinManifests()) {
		t.Fatalf("len = %d, want %d", got, len(BuiltinManifests()))
	}
	for name := range BuiltinManifests() {
		m, ok := r.Get(name)
		if !ok {
			t.Errorf("Get(%q) missing", name)
		}
		if m.Name != name {
			t.Errorf("%q: Name = %q, want %q", name, m.Name, name)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. §16.1 KEYSTONE: override pi's default_model → ONLY default_model changes
// ---------------------------------------------------------------------------

func TestNewRegistry_OverrideExisting_OnlyTouchedFieldChanges(t *testing.T) {
	base, _ := NewRegistry(nil).Get("pi") // the built-in pi
	r := NewRegistry(map[string]Manifest{"pi": {DefaultModel: strPtr("glm-5.2")}})
	got, ok := r.Get("pi")
	if !ok {
		t.Fatal("pi missing")
	}
	if got.DefaultModel == nil || *got.DefaultModel != "glm-5.2" {
		t.Errorf("DefaultModel = %v, want glm-5.2", got.DefaultModel)
	}
	// Untouched fields survive from the built-in:
	if *got.Command != *base.Command {
		t.Errorf("Command changed: %q vs %q", *got.Command, *base.Command)
	}
	if !reflect.DeepEqual(got.BareFlags, base.BareFlags) {
		t.Errorf("BareFlags changed: %v vs %v", got.BareFlags, base.BareFlags)
	}
	if *got.PrintFlag != *base.PrintFlag {
		t.Errorf("PrintFlag changed")
	}
	if got.Name != "pi" {
		t.Errorf("Name = %q, want pi", got.Name)
	}
}

// ---------------------------------------------------------------------------
// 4. Explicit-zero override wins at the registry level
// ---------------------------------------------------------------------------

func TestNewRegistry_OverrideExisting_ExplicitZeroWins(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"pi": {StripCodeFence: boolPtr(false), PrintFlag: strPtr("")}})
	got, _ := r.Get("pi")
	if got.StripCodeFence == nil || *got.StripCodeFence != false {
		t.Errorf("StripCodeFence = %v, want false", got.StripCodeFence)
	}
	if got.PrintFlag == nil || *got.PrintFlag != "" {
		t.Errorf("PrintFlag = %v, want \"\"", got.PrintFlag)
	}
}

// ---------------------------------------------------------------------------
// 5. Brand-new §12.8 provider is ADDED verbatim
// ---------------------------------------------------------------------------

func TestNewRegistry_NewName_AddedVerbatim(t *testing.T) {
	my := Manifest{Command: strPtr("/opt/agent"), PromptDelivery: strPtr("stdin"), BareFlags: []string{"--x"}}
	r := NewRegistry(map[string]Manifest{"myagent": my})
	got, ok := r.Get("myagent")
	if !ok {
		t.Fatal("myagent missing")
	}
	if got.Name != "myagent" {
		t.Errorf("Name = %q, want myagent", got.Name)
	}
	if *got.Command != "/opt/agent" {
		t.Errorf("Command lost")
	}
	if !reflect.DeepEqual(got.BareFlags, []string{"--x"}) {
		t.Errorf("BareFlags = %v", got.BareFlags)
	}
	if got := len(r.manifests); got != len(BuiltinManifests())+1 {
		t.Errorf("count = %d, want %d", got, len(BuiltinManifests())+1)
	}
}

// ---------------------------------------------------------------------------
// 6. Get missing → (zero, false)
// ---------------------------------------------------------------------------

func TestGet_MissingReturnsFalse(t *testing.T) {
	r := NewRegistry(nil)
	if _, ok := r.Get("nope"); ok {
		t.Error("want false for unknown name")
	}
}

// ---------------------------------------------------------------------------
// 7. List sorted ascending by Name
// ---------------------------------------------------------------------------

func TestList_SortedByName(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"zzz": {Command: strPtr("x")}, "aaa": {Command: strPtr("y")}})
	list := r.List()
	for i := 1; i < len(list); i++ {
		if !(list[i-1].Name < list[i].Name) {
			t.Errorf("not sorted at %d: %q >= %q", i, list[i-1].Name, list[i].Name)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. IsInstalled: false for bogus + empty; true for "go"; Detect wins over Command
// ---------------------------------------------------------------------------

func TestIsInstalled(t *testing.T) {
	r := NewRegistry(nil)
	if r.IsInstalled(Manifest{Command: strPtr("definitely-not-a-real-binary-xyz123")}) {
		t.Error("bogus reported installed")
	}
	if r.IsInstalled(Manifest{}) {
		t.Error("empty DetectCommand reported installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH (unexpected in go test env)")
	}
	if !r.IsInstalled(Manifest{Command: strPtr("go")}) {
		t.Error("go reported not installed")
	}
	// Detect overrides Command for detection:
	if r.IsInstalled(Manifest{Detect: strPtr("definitely-not-real-abc"), Command: strPtr("go")}) {
		t.Error("Detect=bogus should be false despite Command=go")
	}
}

// ---------------------------------------------------------------------------
// 9. MarshalTOML: round-trip + unknown error
// ---------------------------------------------------------------------------

func TestMarshalTOML_RoundTrip(t *testing.T) {
	r := NewRegistry(nil)
	s, err := r.MarshalTOML("pi")
	if err != nil {
		t.Fatalf("MarshalTOML(pi): %v", err)
	}
	// Double round-trip: marshal→unmarshal→marshal should produce identical TOML.
	// (nil slices marshal as `[]` and unmarshal as non-nil empty — pointer addresses
	// differ too — so struct-level DeepEqual would fail on cosmetic differences.)
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	s2, err := toml.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(s2) != s {
		t.Errorf("double round-trip mismatch:\nfirst=\n%s\nsecond=\n%s", s, string(s2))
	}
}

func TestMarshalTOML_UnknownErrors(t *testing.T) {
	r := NewRegistry(nil)
	if _, err := r.MarshalTOML("nope"); err == nil {
		t.Error("want error for unknown name")
	}
}

// ---------------------------------------------------------------------------
// 10. MarshalTOML reflects a merged override
// ---------------------------------------------------------------------------

func TestMarshalTOML_ReflectsMerge(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"pi": {DefaultModel: strPtr("glm-5.2")}})
	s, _ := r.MarshalTOML("pi")
	// Decode and check the overridden field instead of string search (avoids strings import).
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if decoded.DefaultModel == nil || *decoded.DefaultModel != "glm-5.2" {
		t.Errorf("merged default_model missing from TOML: got %v", decoded.DefaultModel)
	}
}

// TestMarshalTOML_ListModelsCommandPopulated asserts that MarshalTOML emits list_models_command
// for a populated built-in (opencode). Proves MarshalTOML needs no body edit — toml.Marshal is reflective.
func TestMarshalTOML_ListModelsCommandPopulated(t *testing.T) {
	r := NewRegistry(nil)
	s, err := r.MarshalTOML("opencode")
	if err != nil {
		t.Fatalf("MarshalTOML(opencode): %v", err)
	}
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if !reflect.DeepEqual(decoded.ListModelsCommand, []string{"opencode", "models"}) {
		t.Errorf("opencode list_models_command = %v, want [\"opencode\",\"models\"]", decoded.ListModelsCommand)
	}
}

// TestMarshalTOML_ListModelsCommandNil asserts that MarshalTOML emits a nil-compatible list_models_command
// for a built-in that has no verified listing (claude). go-toml marshals nil []string as `key = []`;
// this is display-only and harmless — matches how nil subcommand already renders.
func TestMarshalTOML_ListModelsCommandNil(t *testing.T) {
	r := NewRegistry(nil)
	s, err := r.MarshalTOML("claude")
	if err != nil {
		t.Fatalf("MarshalTOML(claude): %v", err)
	}
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	// Nil slices marshal as [] then decode to non-nil empty — both are "absent" semantically.
	if len(decoded.ListModelsCommand) != 0 {
		t.Errorf("claude list_models_command = %v, want empty/nil", decoded.ListModelsCommand)
	}
}

// TestMarshalTOML_UserOverrideListModelsCommand asserts that a user [provider.x] block setting
// list_models_command is honored by the existing merge path (slice regime 2).
func TestMarshalTOML_UserOverrideListModelsCommand(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"myagent": {
		Command:           strPtr("/opt/agent"),
		PromptDelivery:    strPtr("stdin"),
		ListModelsCommand: []string{"myagent", "list"},
	}})
	s, err := r.MarshalTOML("myagent")
	if err != nil {
		t.Fatalf("MarshalTOML(myagent): %v", err)
	}
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if !reflect.DeepEqual(decoded.ListModelsCommand, []string{"myagent", "list"}) {
		t.Errorf("myagent list_models_command = %v, want [\"myagent\",\"list\"]", decoded.ListModelsCommand)
	}
}

// ---------------------------------------------------------------------------
// 11. DefaultProvider: pi preferred; falls through; "" if none; ignores user-defined
// ---------------------------------------------------------------------------

func TestDefaultProvider(t *testing.T) {
	r := NewRegistry(nil)
	cases := []struct {
		installed []string
		want      string
	}{
		{[]string{"pi", "claude"}, "pi"},       // pi always wins (rank 1)
		{[]string{"codex", "claude"}, "codex"}, // codex(6) before claude(7)
		{[]string{"cursor", "agy"}, "cursor"},  // cursor(3) before agy(4)
		{[]string{"opencode", "pi"}, "pi"},     // pi still tops opencode(2)
		{[]string{"myagent"}, ""},              // user-defined never auto-selected
		{nil, ""},                              // nothing installed
	}
	for _, c := range cases {
		if got := r.DefaultProvider(c.installed); got != c.want {
			t.Errorf("DefaultProvider(%v) = %q, want %q", c.installed, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 12. DecodeUserOverrides: bridges raw config map → typed Manifests
// ---------------------------------------------------------------------------

func TestDecodeUserOverrides(t *testing.T) {
	raw := map[string]map[string]any{
		"myagent": {"command": "/opt/agent", "prompt_delivery": "stdin", "bare_flags": []any{"--no-mcp"}, "default_model": "m1"},
		"pi":      {"default_model": "glm-5.2"}, // override a built-in name
	}
	got, err := DecodeUserOverrides(raw)
	if err != nil {
		t.Fatalf("DecodeUserOverrides: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	my := got["myagent"]
	if my.Name != "myagent" {
		t.Errorf("myagent.Name = %q", my.Name)
	}
	if *my.Command != "/opt/agent" {
		t.Errorf("Command = %q", *my.Command)
	}
	if !reflect.DeepEqual(my.BareFlags, []string{"--no-mcp"}) {
		t.Errorf("BareFlags = %v", my.BareFlags)
	}
	if my.Detect != nil {
		t.Errorf("Detect = %v, want nil (absent)", my.Detect)
	}
	if my.StripCodeFence != nil {
		t.Errorf("StripCodeFence = %v, want nil (absent)", my.StripCodeFence)
	}
	pi := got["pi"]
	if *pi.DefaultModel != "glm-5.2" {
		t.Errorf("pi.DefaultModel = %q", *pi.DefaultModel)
	}
	if pi.Command != nil {
		t.Errorf("pi.Command = %v, want nil (absent in override)", pi.Command)
	}
	// nil input → empty non-nil map, no error.
	got0, err0 := DecodeUserOverrides(nil)
	if err0 != nil || got0 == nil || len(got0) != 0 {
		t.Errorf("nil input: got=%v err=%v", got0, err0)
	}
}

// ---------------------------------------------------------------------------
// FirstTooledProvider (FR-D4)
// ---------------------------------------------------------------------------

func TestFirstTooledProvider(t *testing.T) {
	r := NewRegistry(nil)
	cases := []struct {
		installed []string
		want      string
	}{
		{[]string{"pi", "claude"}, "pi"},      // pi is first capable (priority order)
		{[]string{"claude", "pi"}, "pi"},      // pi still wins regardless of input order
		{[]string{"claude", "agy"}, "claude"}, // only claude capable; agy is not
		{[]string{"agy", "qwen-code"}, ""},    // neither agy nor qwen-code is stager-capable
		{[]string{"claude"}, "claude"},        // claude alone is capable
		{[]string{"agy"}, ""},                 // agy is NOT stager-capable (nil TooledFlags)
		{[]string{"myagent"}, ""},             // user-defined never auto-selected
		{nil, ""},                             // nothing installed
	}
	for _, c := range cases {
		if got := r.FirstTooledProvider(c.installed); got != c.want {
			t.Errorf("FirstTooledProvider(%v) = %q, want %q", c.installed, got, c.want)
		}
	}
}
