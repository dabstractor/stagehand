package config

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// buildBootstrapConfig — pure unit tests (no Execute, no $PATH)
// Moved from internal/cmd/config_test.go (P1.M4.T4.S1 — byte-identical).
// ---------------------------------------------------------------------------

func TestBuildBootstrapConfig_Pi(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi"})

	// config_version = 2 uncommented
	if !strings.Contains(content, "config_version = 2") {
		t.Error("missing config_version = 2")
	}

	// provider = "pi" uncommented
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\"")
	}

	// pi's four role models uncommented
	assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
	assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
	assertContains(t, content, "[role.arbiter]", `model = "gpt-5.4-mini"`)

	// pi IS stager-capable — no fallback annotation
	if strings.Contains(content, "cannot serve as the stager") {
		t.Error("pi config should NOT have stager fallback annotation")
	}

	// No other-provider commented blocks (only pi in installed)
	if strings.Contains(content, "=== claude (installed)") {
		t.Error("pi-only config should not have claude commented block")
	}
}

func TestBuildBootstrapConfig_GeminiStagerFallback(t *testing.T) {
	content := buildBootstrapConfig("gemini", nil)

	// provider = "gemini"
	if !strings.Contains(content, `provider = "gemini"`) {
		t.Error("missing provider = \"gemini\"")
	}

	// gemini's planner model
	assertContains(t, content, "[role.planner]", `model = "gemini-3.5-pro"`)

	// stager routed to pi
	assertContains(t, content, "[role.stager]", `provider = "pi"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)

	// annotation
	if !strings.Contains(content, "cannot serve as the stager") {
		t.Error("gemini config should have stager fallback annotation")
	}
	if !strings.Contains(content, "routed to pi") {
		t.Error("gemini config should mention routed to pi")
	}

	// gemini's message and arbiter
	assertContains(t, content, "[role.message]", `model = "gemini-3.1-flash-lite"`)
	assertContains(t, content, "[role.arbiter]", `model = "gemini-3.5-flash"`)
}

func TestBuildBootstrapConfig_OtherInstalledCommented(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi", "claude"})

	// UNCOMMENTED role blocks are pi's
	assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
	assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)

	// claude appears as commented block
	if !strings.Contains(content, "=== claude (installed)") {
		t.Error("missing claude commented block header")
	}
	if !strings.Contains(content, `# provider = "claude"`) {
		t.Error("missing commented claude provider line")
	}
	if !strings.Contains(content, `# model = "haiku"`) {
		t.Error("missing commented claude haiku model")
	}

	// claude's uncommented role blocks should NOT appear (only pi is the target)
	// Count uncommented [role.message] — should be exactly 1 (pi's)
	count := strings.Count(content, "\n[role.message]")
	if count != 1 {
		t.Errorf("expected exactly 1 uncommented [role.message], got %d", count)
	}
}

func TestBuildBootstrapConfig_NoInstallFallback(t *testing.T) {
	content := buildBootstrapConfig("pi", nil)

	// Should have the fallback annotation on the provider line
	if !strings.Contains(content, "no built-in agent detected on $PATH") {
		t.Error("missing no-install fallback annotation")
	}
}

func TestBuildBootstrapConfig_ValidTOML(t *testing.T) {
	cases := []struct {
		target    string
		installed []string
	}{
		{"pi", []string{"pi"}},
		{"pi", []string{"pi", "claude"}},
		{"claude", []string{"claude"}},
		{"gemini", nil},
		{"claude", []string{"claude", "pi"}},
		{"agy", []string{"agy", "pi", "claude"}},
	}
	for _, tc := range cases {
		t.Run(tc.target+"_"+strings.Join(tc.installed, ","), func(t *testing.T) {
			content := buildBootstrapConfig(tc.target, tc.installed)
			var m map[string]any
			if err := toml.Unmarshal([]byte(content), &m); err != nil {
				t.Errorf("buildBootstrapConfig(%q, %v) produced invalid TOML: %v", tc.target, tc.installed, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GenerateBootstrapConfig — shared entry point tests (P1.M4.T4.S1)
// ---------------------------------------------------------------------------

func TestGenerateBootstrapConfig_AutoDetectPi(t *testing.T) {
	content := GenerateBootstrapConfig("")

	// Should contain provider = "pi" (nothing on $PATH in CI → fallback)
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("expected provider = \"pi\" (auto-detect fallback)")
	}

	// Should be valid TOML
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("GenerateBootstrapConfig(\"\") produced invalid TOML: %v", err)
	}

	// config_version = 2
	if cv, ok := m["config_version"]; !ok || cv != int64(2) {
		t.Errorf("config_version = %v, want 2", cv)
	}
}

func TestGenerateBootstrapConfig_NamedProvider(t *testing.T) {
	content := GenerateBootstrapConfig("claude")

	if !strings.Contains(content, `provider = "claude"`) {
		t.Error("expected provider = \"claude\"")
	}

	// claude's role models
	assertContains(t, content, "[role.planner]", `model = "opus"`)
	assertContains(t, content, "[role.stager]", `provider = "claude"`)
	assertContains(t, content, "[role.stager]", `model = "sonnet"`)
	assertContains(t, content, "[role.message]", `model = "haiku"`)

	// Valid TOML
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("GenerateBootstrapConfig(\"claude\") produced invalid TOML: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertContains checks that content contains all the specified substrings.
func assertContains(t *testing.T, content string, substrs ...string) {
	t.Helper()
	for _, s := range substrs {
		if !strings.Contains(content, s) {
			t.Errorf("content missing %q", s)
		}
	}
}
