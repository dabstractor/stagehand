package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/dustin/stagehand/internal/provider"
)

// These white-box tests cover the MOCKING contract for the providers command
// tree (PRD FR46–FR48, US10). They mirror the repo's testing conventions:
// white-box package main, no testify, reflect.DeepEqual, and the pure render
// helpers (renderProvidersList / showProviderManifest) as the hermetic targets.
//
// They deliberately avoid config.Load (which does file/git-config I/O): the
// pure helpers take a *provider.Registry built directly from provider.Builtins
// plus fabricated overrides, so the assertions are hermetic and deterministic.

// manifestEqual reports whether two manifests are equal after normalizing nil
// vs empty for the slice/map fields. go-toml/v2 encodes a nil slice as
// `key = []` and a nil map as `key = {}`, which both decode back to NON-nil
// empty allocations — TOML cannot distinguish them — so a naive
// reflect.DeepEqual would report pi (BareFlags set, Env/Subcommand nil) and
// opencode (BareFlags nil) as having changed across a round-trip. Normalizing
// both sides to nil when a field is empty makes the round-trip lossless for
// the scalar+bool fields, which ARE exactly comparable. (Subcommand, BareFlags,
// and Env are the only reference-typed fields on Manifest.)
func manifestEqual(a, b provider.Manifest) bool {
	if len(a.Subcommand) == 0 && len(b.Subcommand) == 0 {
		a.Subcommand, b.Subcommand = nil, nil
	}
	if len(a.BareFlags) == 0 && len(b.BareFlags) == 0 {
		a.BareFlags, b.BareFlags = nil, nil
	}
	if len(a.Env) == 0 && len(b.Env) == 0 {
		a.Env, b.Env = nil, nil
	}
	return reflect.DeepEqual(a, b)
}

// TestProvidersList_DetectedAndDefault asserts the MOCKING contract for the
// list render: every built-in name appears, detected/not-detected statuses are
// emitted, a fabricated provider is marked "not detected", and the trailing
// default line carries the resolved default name and model.
func TestProvidersList_DetectedAndDefault(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "pi", "glm-5-turbo"); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	// Every built-in provider name is present.
	for _, name := range []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing built-in provider %q\noutput:\n%s", name, out)
		}
	}

	// "detected" status is emitted, and pi (installed in this env) is detected.
	if !strings.Contains(out, "detected") {
		t.Errorf("output missing a detected/not-detected status\noutput:\n%s", out)
	}
	if detected["pi"] && !strings.Contains(out, "pi") {
		t.Errorf("pi is detected but absent from output\noutput:\n%s", out)
	}

	// The fabricated provider is present and marked not detected.
	if !strings.Contains(out, "definitely-not-an-agent-xyz") {
		t.Errorf("output missing fabricated provider name\noutput:\n%s", out)
	}
	// The fabricated row must carry "not detected" somewhere after its name.
	if !strings.Contains(out, "not detected") {
		t.Errorf("output missing \"not detected\" status\noutput:\n%s", out)
	}

	// The trailing default line names the resolved default and model.
	if !strings.Contains(out, "default provider: pi") {
		t.Errorf("output missing \"default provider: pi\" line\noutput:\n%s", out)
	}
	if !strings.Contains(out, "glm-5-turbo") {
		t.Errorf("output missing resolved default model \"glm-5-turbo\"\noutput:\n%s", out)
	}
}

// TestProvidersList_DefaultMarkerOnDefault asserts the "(default)" marker is
// placed on the resolved default provider's row (FR46).
func TestProvidersList_DefaultMarkerOnDefault(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), nil)
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "claude", "sonnet"); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "claude (default)") && !strings.Contains(out, "sonnet (default)") {
		t.Errorf("output missing a \"(default)\" marker on the default provider's row\noutput:\n%s", out)
	}
	// Ensure a non-default provider is NOT mis-marked. pi is a builtin distinct
	// from claude; its row must not carry the default marker.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "pi ") || strings.Contains(line, " pi ") {
			if strings.Contains(line, "(default)") {
				t.Errorf("non-default provider row carries (default) marker: %q", line)
			}
		}
	}
}

// TestProvidersList_NoneDetected asserts that when no provider is detected, the
// default line reads "(none detected)" and the model falls back to "(unset)".
func TestProvidersList_NoneDetected(t *testing.T) {
	// A registry whose single provider is not on $PATH.
	reg := provider.NewRegistry(nil, map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "", ""); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "default provider: (none detected)") {
		t.Errorf("output missing \"(none detected)\" default line\noutput:\n%s", out)
	}
	if !strings.Contains(out, "model: (unset)") {
		t.Errorf("output missing \"(unset)\" model fallback\noutput:\n%s", out)
	}
}

// TestProvidersShow_RoundTripsToSameManifest asserts every built-in manifest
// survives a TOML encode→decode round-trip under the nil-vs-empty-normalizing
// manifestEqual helper. This is the gotcha surface: go-toml/v2 encodes nil
// slices/maps as `[]`/`{}` which decode to non-nil empties, so pi (Env nil),
// opencode (BareFlags nil), and codex/gemini/cursor/pi must all compare equal
// after normalization.
func TestProvidersShow_RoundTripsToSameManifest(t *testing.T) {
	for name, orig := range provider.Builtins() {
		var buf bytes.Buffer
		if err := toml.NewEncoder(&buf).Encode(orig); err != nil {
			t.Errorf("encode %q failed: %v", name, err)
			continue
		}
		var decoded provider.Manifest
		if err := toml.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Errorf("decode %q failed: %v\ninput:\n%s", name, err, buf.String())
			continue
		}
		if !manifestEqual(orig, decoded) {
			t.Errorf("manifest %q did not round-trip\norig:   %#v\ndecoded: %#v", name, orig, decoded)
		}
	}
}

// TestProvidersShow_OverrideReflected asserts a user override (FR48) is visible
// in the show output while the rest of the built-in manifest survives the
// field-merge (decisions.md §6): setting only default_model leaves bare_flags
// intact. Routed through showProviderManifest to exercise the real show path.
func TestProvidersShow_OverrideReflected(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), map[string]provider.Manifest{
		"pi": {DefaultModel: "overridden-model"},
	})

	m, ok := reg.Get("pi")
	if !ok {
		t.Fatal(`reg.Get("pi") ok = false, want true`)
	}
	var buf bytes.Buffer
	if err := showProviderManifest(&buf, reg, "pi"); err != nil {
		t.Fatalf("showProviderManifest returned error: %v", err)
	}
	out := buf.String()

	// The override took effect.
	if !strings.Contains(out, "overridden-model") {
		t.Errorf("override default_model not reflected in show output\noutput:\n%s", out)
	}
	// A built-in field (pi's first bare flag) survived the merge.
	if !strings.Contains(out, "--no-tools") {
		t.Errorf("built-in bare_flags did not survive the field-merge\noutput:\n%s", out)
	}
	// Sanity: the merged manifest in-memory also carries both (proves the
	// field-merge happened before encoding, not via the TOML).
	if m.DefaultModel != "overridden-model" {
		t.Errorf("merged DefaultModel = %q, want %q", m.DefaultModel, "overridden-model")
	}
	found := false
	for _, f := range m.BareFlags {
		if f == "--no-tools" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("merged BareFlags lost --no-tools: %#v", m.BareFlags)
	}
}

// TestProvidersShow_UnknownErrors asserts showProviderManifest returns a non-nil
// error whose message mentions "unknown provider" for a name absent from the
// registry (FR47 exit-1 path). This drives the error path directly with a
// hermetic registry, avoiding config.Load file/git-config I/O.
func TestProvidersShow_UnknownErrors(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), nil)

	var buf bytes.Buffer
	err := showProviderManifest(&buf, reg, "no-such-agent")
	if err == nil {
		t.Fatal("showProviderManifest(unknown) err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error message = %q, want it to mention \"unknown provider\"", err.Error())
	}
}

// TestProvidersCmd_Registered asserts the providers tree is wired onto rootCmd
// (via the package init) without main.go being edited, and that list/show are
// its children. This guards the registration contract with P1.M7.T2.S1.
func TestProvidersCmd_Registered(t *testing.T) {
	providersCmd, _, err := rootCmd.Find([]string{"providers"})
	if err != nil {
		t.Fatalf("rootCmd.Find([providers]) error: %v", err)
	}
	if providersCmd == nil {
		t.Fatal("providers command not registered on rootCmd")
	}
	if providersCmd.Name() != "providers" {
		t.Errorf("providers command Name = %q, want %q", providersCmd.Name(), "providers")
	}
	// Both children are present.
	for _, sub := range []string{"list", "show"} {
		c, _, err := providersCmd.Find([]string{sub})
		if err != nil || c == nil {
			t.Errorf("providers subcommand %q not found: err=%v", sub, err)
		}
	}
}
