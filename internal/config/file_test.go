package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTempTOML creates a TOML file in t.TempDir() with the given body and returns its path.
func writeTempTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("writeTempTOML: %v", err)
	}
	return path
}

// --- Test A: TestLoadTOMLValid ---

func TestLoadTOMLValid(t *testing.T) {
	body := `
[defaults]
provider = "pi"
timeout = "90s"
auto_stage_all = true

[generation]
max_diff_bytes = 12345
output = "json"

[provider.pi]
default_model = "glm-5.2"

[provider.myagent]
command = "/opt/myagent/bin/agent"
bare_flags = ["--no-mcp", "--ephemeral"]
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi", cfg.Provider)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout=%v want 90s", cfg.Timeout)
	}
	if !cfg.AutoStageAll {
		t.Errorf("AutoStageAll=false want true")
	}
	if cfg.MaxDiffBytes != 12345 {
		t.Errorf("MaxDiffBytes=%d want 12345", cfg.MaxDiffBytes)
	}
	if cfg.Output != "json" {
		t.Errorf("Output=%q want json", cfg.Output)
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("Providers len=%d want 2", len(cfg.Providers))
	}
	if m, ok := cfg.Providers["pi"]; !ok {
		t.Errorf("Providers[\"pi\"] missing")
	} else if m["default_model"] != "glm-5.2" {
		t.Errorf("pi.default_model=%v want glm-5.2", m["default_model"])
	}
	if _, ok := cfg.Providers["myagent"]; !ok {
		t.Errorf("Providers[\"myagent\"] missing")
	}
}

// --- Test B: TestLoadTOMLMissing ---

func TestLoadTOMLMissing(t *testing.T) {
	cfg, err := loadTOML(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Errorf("loadTOML(missing): err=%v, want nil", err)
	}
	if cfg != nil {
		t.Errorf("loadTOML(missing): cfg=%v, want nil", cfg)
	}
}

// --- Test C: TestOverlayPartial (CONTRACT CASE) ---

func TestOverlayPartial(t *testing.T) {
	dst := Defaults()                                         // Layer-1 baseline (AutoStageAll=true, MaxDiffBytes=300000, Timeout=120s, …)
	src := &Config{Timeout: 90 * time.Second, Output: "json"} // PARTIAL: only 2 fields set
	overlay(&dst, src)
	if dst.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s", dst.Timeout)
	}
	if dst.Output != "json" {
		t.Errorf("Output = %q, want json", dst.Output)
	}
	// Everything else MUST be untouched (NOT a wholesale replace):
	if !dst.AutoStageAll {
		t.Errorf("AutoStageAll clobbered: false, want true (partial merge broken)")
	}
	if dst.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes clobbered: %d, want 300000", dst.MaxDiffBytes)
	}
	if dst.Provider != "" {
		t.Errorf("Provider clobbered: %q, want empty", dst.Provider)
	}
	if dst.Model != "" {
		t.Errorf("Model clobbered: %q, want empty", dst.Model)
	}
	if dst.MaxMdLines != 100 {
		t.Errorf("MaxMdLines clobbered: %d, want 100", dst.MaxMdLines)
	}
	if dst.SubjectTargetChars != 50 {
		t.Errorf("SubjectTargetChars clobbered: %d, want 50", dst.SubjectTargetChars)
	}
	if !dst.StripCodeFence {
		t.Errorf("StripCodeFence clobbered: false, want true")
	}
}

// --- Test D: TestGlobalConfigPath ---

func TestGlobalConfigPath(t *testing.T) {
	// Save and restore original env
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	// Case 1: XDG set AND absolute
	absTmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", absTmp)
	got := globalConfigPath()
	want := filepath.Join(absTmp, "stagehand", "config.toml")
	if got != want {
		t.Errorf("XDG set: globalConfigPath() = %q, want %q", got, want)
	}

	// Case 2: XDG empty → falls back to home/.config/stagehand/config.toml
	os.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir: %v", err)
	}
	got = globalConfigPath()
	want = filepath.Join(home, ".config", "stagehand", "config.toml")
	if got != want {
		t.Errorf("XDG unset: globalConfigPath() = %q, want %q", got, want)
	}

	// Case 3: XDG set but RELATIVE → ignored (falls back to home)
	os.Setenv("XDG_CONFIG_HOME", "relative/path")
	got = globalConfigPath()
	if got != want {
		t.Errorf("XDG relative: globalConfigPath() = %q, want %q (home fallback)", got, want)
	}
}

// --- Test E: TestOverlayProvidersKeyReplace ---

func TestOverlayProvidersKeyReplace(t *testing.T) {
	dst := Defaults()
	dst.Providers = map[string]map[string]any{
		"pi":     {"default_model": "A"},
		"claude": {"api_key": "key1"},
	}
	src := &Config{
		Providers: map[string]map[string]any{
			"pi": {"default_model": "B"},
		},
	}
	overlay(&dst, src)

	// pi replaced entirely
	if dst.Providers["pi"]["default_model"] != "B" {
		t.Errorf("pi not replaced: default_model=%v, want B", dst.Providers["pi"]["default_model"])
	}
	// claude still present
	if dst.Providers["claude"] == nil {
		t.Errorf("claude missing after overlay")
	}
	if dst.Providers["claude"]["api_key"] != "key1" {
		t.Errorf("claude mutated: %v", dst.Providers["claude"]["api_key"])
	}
}

// --- Test F: TestRepoProviderNotice ---

func TestRepoProviderNotice(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"provider set", &Config{Provider: "pi"}, "sets provider to \"pi\""},
		{"nil config", nil, ""},
		{"empty config", &Config{}, ""},
		{"empty provider", &Config{Provider: ""}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := repoProviderNotice(tc.cfg)
			if tc.want == "" {
				if got != "" {
					t.Errorf("repoProviderNotice(%v) = %q, want empty", tc.cfg, got)
				}
			} else if !strings.Contains(got, tc.want) {
				t.Errorf("repoProviderNotice() = %q, want substring %q", got, tc.want)
			}
			// All non-empty notices must contain .stagehand.toml
			if got != "" && !strings.Contains(got, ".stagehand.toml") {
				t.Errorf("notice missing .stagehand.toml: %q", got)
			}
		})
	}
}

// --- Test G: TestLoadRepoLocalConfig ---

func TestLoadRepoLocalConfig(t *testing.T) {
	// Use a temp dir as CWD to isolate the .stagehand.toml lookup
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Save/restore noticeOut
	origNoticeOut := noticeOut
	defer func() { noticeOut = origNoticeOut }()

	// Case 1: no .stagehand.toml → nil, nil
	os.Chdir(dir)
	buf := &bytes.Buffer{}
	noticeOut = buf
	cfg, err := loadRepoLocalConfig()
	if err != nil {
		t.Fatalf("no file: err=%v", err)
	}
	if cfg != nil {
		t.Errorf("no file: cfg=%v, want nil", cfg)
	}
	if buf.Len() != 0 {
		t.Errorf("no file: notice=%q, want empty", buf.String())
	}

	// Case 2: .stagehand.toml with provider set → notice emitted
	tomlBody := `[defaults]
provider = "pi"
`
	if err := os.WriteFile(filepath.Join(dir, ".stagehand.toml"), []byte(tomlBody), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf.Reset()
	cfg, err = loadRepoLocalConfig()
	if err != nil {
		t.Fatalf("with provider: err=%v", err)
	}
	if cfg == nil {
		t.Fatalf("with provider: cfg=nil, want non-nil")
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q, want pi", cfg.Provider)
	}
	if !strings.Contains(buf.String(), "sets provider to \"pi\"") {
		t.Errorf("notice=%q, want to contain 'sets provider to \"pi\"'", buf.String())
	}

	// Case 3: .stagehand.toml without provider → no notice
	tomlBody = `[defaults]
timeout = "60s"
`
	if err := os.WriteFile(filepath.Join(dir, ".stagehand.toml"), []byte(tomlBody), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf.Reset()
	cfg, err = loadRepoLocalConfig()
	if err != nil {
		t.Fatalf("no provider: err=%v", err)
	}
	if cfg == nil {
		t.Fatalf("no provider: cfg=nil, want non-nil")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout=%v, want 60s", cfg.Timeout)
	}
	if buf.Len() != 0 {
		t.Errorf("no provider: notice=%q, want empty", buf.String())
	}
}

// --- TestLoadTOMLInvalidTimeout ---

func TestLoadTOMLInvalidTimeout(t *testing.T) {
	body := `
[defaults]
timeout = "120"
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if cfg != nil {
		t.Errorf("invalid timeout: cfg=%v, want nil", cfg)
	}
	if err == nil {
		t.Fatal("invalid timeout: err=nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("err=%q, want 'invalid timeout'", err.Error())
	}
}

// --- TestOverlayNilSrc ---

func TestOverlayNilSrc(t *testing.T) {
	dst := Defaults()
	overlay(&dst, nil) // must not panic
	// Compare scalar fields; Providers is nil in both (defaults has no Providers)
	if dst.Provider != "" {
		t.Errorf("Provider changed: %q", dst.Provider)
	}
	if dst.Timeout != 120*time.Second {
		t.Errorf("Timeout changed: %v", dst.Timeout)
	}
	if dst.AutoStageAll != true {
		t.Errorf("AutoStageAll changed")
	}
	if dst.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes changed: %d", dst.MaxDiffBytes)
	}
}
