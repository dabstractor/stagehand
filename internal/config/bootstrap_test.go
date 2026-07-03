package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// buildBootstrapConfig — pure unit tests (no Execute, no $PATH)
// Moved from internal/cmd/config_test.go (P1.M4.T4.S1 — byte-identical).
// ---------------------------------------------------------------------------

func TestBuildBootstrapConfig_Pi(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi"}, nil)

	// config_version = 3 uncommented (CurrentConfigVersion)
	if !strings.Contains(content, fmt.Sprintf("config_version = %d", CurrentConfigVersion)) {
		t.Errorf("missing config_version = %d", CurrentConfigVersion)
	}

	// provider = "pi" uncommented
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\"")
	}

	// reasoning = "off" uncommented in [defaults] (FR-B1 — emitted so the field is discoverable
	// in the generated file rather than hidden; off is the shipped default for every role, FR-R6)
	if !strings.Contains(content, `reasoning = "off"`) {
		t.Error("missing uncommented reasoning = \"off\" in [defaults]")
	}

	// pi's four role models blanked (no sub-provider in bootstrap — pi picks its own backend default)
	assertContains(t, content, "[role.planner]", `model = ""`)
	assertContains(t, content, "[role.stager]", `model = ""`)
	assertContains(t, content, "[role.message]", `model = ""`)
	assertContains(t, content, "[role.arbiter]", `model = ""`)

	// Negative: a pi-only config must contain NO gpt-5.4 anywhere (catches the stager re-pull bug)
	if strings.Contains(content, "gpt-5.4") {
		t.Errorf("pi bootstrap must not ship un-routable gpt-5.4* models; got:\n%s", content)
	}

	// Multi-backend model-prefix annotation present
	if !strings.Contains(content, "prefix the model with your inference backend") {
		t.Error("pi bootstrap missing the model-prefix annotation")
	}

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
	content := buildBootstrapConfig("gemini", nil, nil)

	// provider = "gemini"
	if !strings.Contains(content, `provider = "gemini"`) {
		t.Error("missing provider = \"gemini\"")
	}

	// gemini's planner model
	assertContains(t, content, "[role.planner]", `model = "gemini-3.1-pro"`)

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
	content := buildBootstrapConfig("pi", []string{"pi", "claude"}, nil)

	// UNCOMMENTED role blocks are pi's (blanked — no sub-provider in bootstrap)
	assertContains(t, content, "[role.planner]", `model = ""`)
	assertContains(t, content, "[role.message]", `model = ""`)

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
	content := buildBootstrapConfig("pi", nil, nil)

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
			content := buildBootstrapConfig(tc.target, tc.installed, nil)
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

	// config_version = 3 (CurrentConfigVersion)
	if cv, ok := m["config_version"]; !ok || cv != int64(CurrentConfigVersion) {
		t.Errorf("config_version = %v, want %d", cv, CurrentConfigVersion)
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

// TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars guards Issue 4: the generated config
// header must document the FR-R6 reasoning env vars (global + per-role), matching docs/cli.md.
func TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars(t *testing.T) {
	content := buildBootstrapConfig("pi", nil, nil)
	assertContains(t, content,
		"STAGEHAND_REASONING",
		"STAGEHAND_<ROLE>_REASONING",
	)
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
