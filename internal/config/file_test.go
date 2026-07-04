package config

import (
	"bytes"
	"fmt"
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

// --- TestLoadRepoLocalConfig_BadTOML ---

func TestLoadRepoLocalConfig_BadTOML(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.WriteFile(filepath.Join(dir, ".stagehand.toml"), []byte("this is [not valid {toml"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = loadRepoLocalConfig()
	if err == nil {
		t.Fatal("loadRepoLocalConfig err=nil, want error for bad TOML")
	}
}

// --- TestLoadTOMLValid ---

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
	if cfg.Output == nil || *cfg.Output != "json" {
		t.Errorf("Output=%v want strPtr(json)", cfg.Output)
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
	dst := Defaults()                                                 // Layer-1 baseline (AutoStageAll=true, MaxDiffBytes=300000, Timeout=120s, …)
	src := &Config{Timeout: 90 * time.Second, Output: strPtr("json")} // PARTIAL: only 2 fields set
	overlay(&dst, src)
	if dst.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s", dst.Timeout)
	}
	if dst.Output == nil || *dst.Output != "json" {
		t.Errorf("Output = %v, want strPtr(json)", dst.Output)
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
	if dst.StripCodeFence != nil {
		t.Errorf("StripCodeFence clobbered: %v, want nil", dst.StripCodeFence)
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

// --- Test E: TestOverlayProvidersFieldMerge ---

func TestOverlayProvidersFieldMerge(t *testing.T) {
	dst := Defaults()
	dst.Providers = map[string]map[string]any{
		"pi":     {"default_model": "A", "default_provider": "zai"},
		"claude": {"api_key": "key1"},
	}
	src := &Config{
		Providers: map[string]map[string]any{
			"pi": {"default_model": "B"}, // higher layer sets model only
		},
	}
	overlay(&dst, src)

	// pi.default_model overridden by src (higher layer wins, per-field)
	if got := dst.Providers["pi"]["default_model"]; got != "B" {
		t.Errorf("pi.default_model=%v, want B", got)
	}
	// pi.default_provider SURVIVES — the v1 key-level replace would have dropped it (PRD §9.8 FR37a).
	// This is the regression that let a repo [provider.pi] default_model erase a global default_provider,
	// leaving a bare --model that misrouted to the wrong upstream.
	if got := dst.Providers["pi"]["default_provider"]; got != "zai" {
		t.Errorf("pi.default_provider=%v, want zai (field-merge must preserve lower-layer fields)", got)
	}
	// a different provider key is untouched
	if dst.Providers["claude"] == nil || dst.Providers["claude"]["api_key"] != "key1" {
		t.Errorf("claude mutated: %v", dst.Providers["claude"])
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
	defer func() { _ = os.Chdir(origDir) }()

	// Save/restore noticeOut
	origNoticeOut := noticeOut
	defer func() { noticeOut = origNoticeOut }()

	// Case 1: no .stagehand.toml → nil, nil
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
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

// --- TestGlobalConfigPath_Wrapper ---

func TestGlobalConfigPath_Wrapper(t *testing.T) {
	// GlobalConfigPath() is just a wrapper — exercise it to cover the exported function.
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	absTmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", absTmp)
	got := GlobalConfigPath()
	want := filepath.Join(absTmp, "stagehand", "config.toml")
	if got != want {
		t.Errorf("GlobalConfigPath() = %q, want %q", got, want)
	}
}

// --- TestResolveConfigPath ---

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		envVal   string // value for STAGEHAND_CONFIG (set via t.Setenv)
		setupXDG bool   // if true, set XDG_CONFIG_HOME to a temp dir before calling
		wantPath string // expected result; empty means "use GlobalConfigPath() with XDG temp dir"
	}{
		{
			name:     "flag_only",
			flag:     "/tmp/my-config.toml",
			envVal:   "",
			wantPath: "/tmp/my-config.toml",
		},
		{
			name:     "env_only",
			flag:     "",
			envVal:   "/tmp/env-config.toml",
			wantPath: "/tmp/env-config.toml",
		},
		{
			name:     "flag_beats_env",
			flag:     "/tmp/flag-config.toml",
			envVal:   "/tmp/env-config.toml",
			wantPath: "/tmp/flag-config.toml",
		},
		{
			name:     "neither_global",
			flag:     "",
			envVal:   "",
			setupXDG: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Always clear STAGEHAND_CONFIG to prevent ambient env leaking
			t.Setenv("STAGEHAND_CONFIG", tc.envVal)

			var want string
			if tc.setupXDG {
				xdg := t.TempDir()
				t.Setenv("XDG_CONFIG_HOME", xdg)
				want = filepath.Join(xdg, "stagehand", "config.toml")
			} else {
				want = tc.wantPath
			}

			got := ResolveConfigPath(tc.flag)
			if got != want {
				t.Errorf("ResolveConfigPath(%q) = %q, want %q", tc.flag, got, want)
			}
		})
	}
}

// --- TestGlobalConfigPath_UserHomeDirFails ---

func TestGlobalConfigPath_UserHomeDirFails(t *testing.T) {
	// When both XDG_CONFIG_HOME is empty/relative AND os.UserHomeDir fails,
	// globalConfigPath falls back to "config.toml" (last-resort, CWD).
	// We force UserHomeDir failure by removing all home-env vars.
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	origHome := os.Getenv("HOME")
	origUserprofile := os.Getenv("USERPROFILE")
	defer func() {
		os.Setenv("XDG_CONFIG_HOME", origXDG)
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserprofile)
	}()

	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	os.Setenv("XDG_CONFIG_HOME", "") // empty → skip XDG path

	got := globalConfigPath()
	// On most systems UserHomeDir still succeeds (reads /etc/passwd on Linux).
	// If it does succeed, just verify the result is well-formed.
	if got == "config.toml" {
		// The fallback path was exercised.
		return
	}
	// Otherwise UserHomeDir succeeded — verify it ends with config.toml
	if !strings.HasSuffix(got, "config.toml") {
		t.Errorf("globalConfigPath() = %q, want ending with config.toml", got)
	}
}

// --- TestSetNoticeOut_NoticeOut ---

func TestSetNoticeOut_NoticeOut(t *testing.T) {
	// Save and restore the global noticeOut
	orig := NoticeOut()
	defer SetNoticeOut(orig)

	buf := &bytes.Buffer{}
	SetNoticeOut(buf)

	// NoticeOut should return the buffer we just set
	if got := NoticeOut(); got != buf {
		t.Errorf("NoticeOut() = %p, want %p (the buffer we set)", got, buf)
	}

	// Write through noticeOut (via fmt.Fprint) and verify it lands in the buffer
	fmt.Fprint(NoticeOut(), "hello notice")
	if buf.String() != "hello notice" {
		t.Errorf("buffer = %q, want %q", buf.String(), "hello notice")
	}
}

// --- TestOverlayStripCodeFenceFalse ---

func TestOverlayStripCodeFenceFalse(t *testing.T) {
	// Regression test for Finding 2: StripCodeFence=false must survive overlay.
	dst := Defaults() // StripCodeFence = true

	// Case 1: src sets false via pointer — must override dst's true
	f := false
	src := &Config{StripCodeFence: &f}
	overlay(&dst, src)
	if dst.StripCodeFence == nil {
		t.Fatal("StripCodeFence is nil after overlay")
	}
	if *dst.StripCodeFence {
		t.Errorf("StripCodeFence = true, want false (overlay should honor explicit false)")
	}

	// Case 2: src has nil StripCodeFence — must NOT override a set dst
	dst = Defaults()
	dst.StripCodeFence = boolPtr(true)
	src = &Config{Output: strPtr("json")} // StripCodeFence left nil (unset)
	overlay(&dst, src)
	if dst.StripCodeFence == nil || !*dst.StripCodeFence {
		t.Errorf("StripCodeFence = %v, want true (nil src must not clobber)", dst.StripCodeFence)
	}
}

// --- TestMaterializeStripCodeFenceFalse ---

func TestMaterializeStripCodeFenceFalse(t *testing.T) {
	// Regression test for Finding 2: a TOML file with strip_code_fence = false must
	// produce a Config with StripCodeFence=false (not drop it).
	body := `
[generation]
strip_code_fence = false
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if cfg.StripCodeFence == nil {
		t.Fatal("StripCodeFence is nil, want non-nil")
	}
	if *cfg.StripCodeFence {
		t.Errorf("StripCodeFence = true, want false")
	}

	// Verify overlay preserves the false
	dst := Defaults() // StripCodeFence = nil (defaults no longer set it)
	overlay(&dst, cfg)
	if dst.StripCodeFence == nil || *dst.StripCodeFence {
		t.Errorf("after overlay: StripCodeFence = %v, want false", dst.StripCodeFence)
	}
}

// --- TestLoadTOMLInvalidTOML ---

func TestLoadTOMLInvalidTOML(t *testing.T) {
	body := `this is [not valid {toml`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if cfg != nil {
		t.Errorf("invalid TOML: cfg=%v, want nil", cfg)
	}
	if err == nil {
		t.Fatal("invalid TOML: err=nil, want error")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("err=%q, want 'parse config'", err.Error())
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

// --- TestLoadTOML_V2Fields ---

// TestLoadTOML_V2Fields proves the v2 file keys decode + materialize: config_version, [generation]
// max_commits/binary_extensions, and [role.<role>] tables (incl. a PARTIAL role whose Provider is "" — the
// field-level decode, not a whole-block). Mirrors TestLoadTOMLValid.
func TestLoadTOML_V2Fields(t *testing.T) {
	body := `
config_version = 2

[generation]
max_commits = 5
binary_extensions = ["foo", "bar"]

[role.planner]
provider = "agy"
model = "gemini-2.5-pro"

[role.stager]
model = "gemini-2.5-flash"
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if cfg.ConfigVersion != 2 {
		t.Errorf("ConfigVersion=%d want 2", cfg.ConfigVersion)
	}
	if cfg.MaxCommits != 5 {
		t.Errorf("MaxCommits=%d want 5", cfg.MaxCommits)
	}
	if len(cfg.BinaryExtensions) != 2 || cfg.BinaryExtensions[0] != "foo" || cfg.BinaryExtensions[1] != "bar" {
		t.Errorf("BinaryExtensions=%v want [foo bar]", cfg.BinaryExtensions)
	}
	if len(cfg.Roles) != 2 {
		t.Fatalf("Roles len=%d want 2", len(cfg.Roles))
	}
	if rc := cfg.Roles["planner"]; rc.Provider != "agy" || rc.Model != "gemini-2.5-pro" {
		t.Errorf("Roles[planner]=%+v want {agy gemini-2.5-pro}", rc)
	}
	// Partial role: only model set → Provider decodes "" (field-level, not whole-block).
	if rc := cfg.Roles["stager"]; rc.Provider != "" || rc.Model != "gemini-2.5-flash" {
		t.Errorf("Roles[stager]=%+v want {\"\" gemini-2.5-flash}", rc)
	}
}

// --- TestOverlayRolesFieldMerge ---

// TestOverlayRolesFieldMerge is the FR-R3 regression guard — MIRRORS TestOverlayProvidersFieldMerge. A
// higher layer setting only [role.planner].model must NOT erase a lower layer's [role.planner].provider
// (the per-role analog of the FR37a provider field-merge). Plus: a src-only role is added; an untouched
// dst role survives.
func TestOverlayRolesFieldMerge(t *testing.T) {
	dst := &Config{
		Roles: map[string]RoleConfig{
			"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
			"message": {Provider: "pi", Model: "gpt-5.4-nano"},
		},
	}
	src := &Config{
		Roles: map[string]RoleConfig{
			"planner": {Model: "gemini-3.5-pro"},                        // higher layer sets MODEL only
			"arbiter": {Provider: "codex", Model: "gpt-5.1-codex-mini"}, // new role
		},
	}
	overlay(dst, src)

	// planner.provider SURVIVES (lower-layer field not clobbered by a higher-layer partial):
	if rc := dst.Roles["planner"]; rc.Provider != "agy" {
		t.Errorf("planner.provider=%q want agy (field-merge must preserve lower-layer provider)", rc.Provider)
	}
	// planner.model OVERRIDDEN by the higher layer:
	if rc := dst.Roles["planner"]; rc.Model != "gemini-3.5-pro" {
		t.Errorf("planner.model=%q want gemini-3.5-pro (higher layer wins)", rc.Model)
	}
	// new role added:
	if rc, ok := dst.Roles["arbiter"]; !ok {
		t.Errorf("arbiter role missing (src-only role must be added)")
	} else if rc.Provider != "codex" || rc.Model != "gpt-5.1-codex-mini" {
		t.Errorf("arbiter=%+v want {codex gpt-5.1-codex-mini}", rc)
	}
	// untouched dst role survives:
	if rc := dst.Roles["message"]; rc.Provider != "pi" || rc.Model != "gpt-5.4-nano" {
		t.Errorf("message=%+v want {pi gpt-5.4-nano} (untouched role must survive)", rc)
	}
}

// --- TestOverlay_V2Scalars ---

// TestOverlay_V2Scalars proves non-zero-wins + partial-merge preservation for ConfigVersion/MaxCommits/
// BinaryExtensions — mirrors TestOverlayPartial.
func TestOverlay_V2Scalars(t *testing.T) {
	// (a) src sets all three → overridden.
	dst := Defaults() // ConfigVersion=0, MaxCommits=12, BinaryExtensions=nil
	src := &Config{ConfigVersion: 3, MaxCommits: 7, BinaryExtensions: []string{"x", "y"}}
	overlay(&dst, src)
	if dst.ConfigVersion != 3 {
		t.Errorf("ConfigVersion=%d want 3 (src non-zero wins)", dst.ConfigVersion)
	}
	if dst.MaxCommits != 7 {
		t.Errorf("MaxCommits=%d want 7 (src non-zero wins)", dst.MaxCommits)
	}
	if len(dst.BinaryExtensions) != 2 {
		t.Errorf("BinaryExtensions=%v want [x y] (src non-empty wins = REPLACE)", dst.BinaryExtensions)
	}

	// (b) src OMITS them (zero/nil) → Defaults() baseline preserved (partial merge, no clobber).
	dst = Defaults()
	src = &Config{Provider: "pi"} // none of the v2 scalars set
	overlay(&dst, src)
	if dst.ConfigVersion != 0 {
		t.Errorf("ConfigVersion=%d want 0 (Defaults pins 0; nil src must not clobber)", dst.ConfigVersion)
	}
	if dst.MaxCommits != 12 {
		t.Errorf("MaxCommits=%d want 12 (zero src must not clobber)", dst.MaxCommits)
	}
	if dst.BinaryExtensions != nil {
		t.Errorf("BinaryExtensions=%v want nil (empty src must not clobber)", dst.BinaryExtensions)
	}
}

// --- TestLoadTOML_AgentTerminologyRemapped ---

// TestLoadTOML_AgentTerminologyRemapped proves that a v2 config using the abandoned intermediate
// agent/[agent.*] terminology loads with its provider block preserved in memory (cfg.Provider +
// cfg.Providers["pi"] populated) WITHOUT requiring the on-disk `config upgrade`.
func TestLoadTOML_AgentTerminologyRemapped(t *testing.T) {
	body := `
config_version = 2

[defaults]
agent = "pi"

[agent.pi]
default_model = "glm-5.2"
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want \"pi\" (agent→provider remap lost the default)", cfg.Provider)
	}
	m, ok := cfg.Providers["pi"]
	if !ok {
		t.Fatalf("Providers[\"pi\"] missing (agent→provider remap lost the [agent.pi] block)")
	}
	if m["default_model"] != "glm-5.2" {
		t.Errorf("pi.default_model=%v want glm-5.2", m["default_model"])
	}
}

// --- TestRemapAgentTerminology ---

// TestRemapAgentTerminology is a focused table-driven helper unit test for remapAgentTerminology.
// Pins idempotency (already-provider + double-run) and key-name-only precision (comments, values,
// prefixed keys untouched).
func TestRemapAgentTerminology(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"table header", "[agent.pi]", "[provider.pi]"},
		{"key spaced", `agent = "pi"`, `provider = "pi"`},
		{"key tight", `agent="pi"`, `provider="pi"`},
		{"indented key", "  agent = \"pi\"", "  provider = \"pi\""},
		{"comment untouched", "# agent = keep", "# agent = keep"},
		{"value untouched", `model = "agent"`, `model = "agent"`},
		{"prefixed key untouched", "my_agent = \"x\"", "my_agent = \"x\""},
		{"already-provider idempotent", "[provider.pi]\nprovider = \"pi\"", "[provider.pi]\nprovider = \"pi\""},
		{"header + key together", "[agent.pi]\nagent = \"pi\"", "[provider.pi]\nprovider = \"pi\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(remapAgentTerminology([]byte(tc.in)))
			if got != tc.want {
				t.Errorf("remapAgentTerminology(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	// double-run idempotency: remap(remap(x)) == remap(x) on a mixed input
	mixed := "[agent.pi]\nagent = \"pi\"\nmodel = \"agent\"\n"
	once := string(remapAgentTerminology([]byte(mixed)))
	twice := string(remapAgentTerminology([]byte(once)))
	if twice != once {
		t.Errorf("remap not idempotent on mixed input:\n once=%q\n twice=%q", once, twice)
	}
}

// --- TestMaterializeExclude ---

// TestMaterializeExclude proves the single-file copy of [generation].exclude (P1.M1.T1.S1).
// materialize() just copies (union across LAYERS happens in overlay(), not here).
func TestMaterializeExclude(t *testing.T) {
	body := `
[generation]
exclude = ["*.lock"]
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "*.lock" {
		t.Errorf("Exclude=%v want [*.lock]", cfg.Exclude)
	}
}

// --- TestOverlayExcludeUnion ---

// TestOverlayExcludeUnion is the load-bearing distinction of P1.M1.T1.S1: overlay() must UNION
// (append) Exclude across layers, unlike BinaryExtensions which REPLACES (see TestOverlay_V2Scalars).
func TestOverlayExcludeUnion(t *testing.T) {
	// (a) dst has globs, src adds more → APPEND, not replace.
	dst := &Config{Exclude: []string{"a"}}
	src := &Config{Exclude: []string{"b"}}
	overlay(dst, src)
	want := []string{"a", "b"}
	if len(dst.Exclude) != len(want) || dst.Exclude[0] != want[0] || dst.Exclude[1] != want[1] {
		t.Errorf("Exclude=%v want %v (overlay must UNION, not replace)", dst.Exclude, want)
	}

	// (b) nil src.Exclude → dst unchanged.
	dst2 := &Config{Exclude: []string{"a"}}
	src2 := &Config{Provider: "pi"} // Exclude left nil
	overlay(dst2, src2)
	if len(dst2.Exclude) != 1 || dst2.Exclude[0] != "a" {
		t.Errorf("Exclude=%v want [a] (nil src.Exclude must not clobber)", dst2.Exclude)
	}

	// (c) BinaryExtensions still REPLACES (regression guard — the two lists must not share behavior).
	dst3 := &Config{BinaryExtensions: []string{"x"}}
	src3 := &Config{BinaryExtensions: []string{"y"}}
	overlay(dst3, src3)
	if len(dst3.BinaryExtensions) != 1 || dst3.BinaryExtensions[0] != "y" {
		t.Errorf("BinaryExtensions=%v want [y] (must still REPLACE, not union)", dst3.BinaryExtensions)
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

// --- TestMaterializeOverlay_DiffContext_TokenLimit ---

// TestMaterializeOverlay_DiffContext_TokenLimit is the load-bearing proof of S2's contract correction:
// Config.DiffContext is *int (nil = unset) so an explicit diff_context=0 (FR3f: changed-lines-only)
// survives the overlay chain, while an omitted key inherits the -U1 default (1). TokenLimit stays
// plain int (FR3d: 0 IS its unset sentinel) and uses the standard != 0 guard everywhere.
//
// Table coverage: DiffContext unset⇒*1, explicit 1⇒*1, explicit 0⇒*0, explicit 3⇒*3, across:
//
//	(a) materialize-only  (file → Config)
//	(b) global-only       (Defaults → overlay global)
//	(c) repo-only         (Defaults → overlay repo)
//	(d) global+repo       (Defaults → overlay global → overlay repo)
//
// The explicit-0 rows PASS under *int + != nil and would FAIL under the contract's literal
// `!= 0` overlay guard (0 != 0 is false → clobbered to *1) — that failure is exactly the bug S2 fixes.
func TestMaterializeOverlay_DiffContext_TokenLimit(t *testing.T) {
	// intp returns a pointer to v (test-local alias of intPtr for table brevity).
	intp := func(v int) *int { return &v }

	// ---- (a) materialize-only: file → Config ----
	// A file that sets diff_context = N decodes into a *int (nil when omitted) and materialize copies
	// the pointer through (nil ⇒ materialize leaves c.DiffContext nil; non-nil ⇒ copies the pointer).
	materializeCases := []struct {
		name   string
		fileDC *int // nil = key omitted in the file
		fileTL int  // 0 = key omitted
		wantDC *int // nil ⇒ expect c.DiffContext == nil (materialize does NOT seed a default)
		wantTL int
	}{
		{"unset", nil, 0, nil, 0},
		{"explicit_1", intp(1), 0, intp(1), 0},
		{"explicit_0", intp(0), 0, intp(0), 0}, // THE key row: explicit 0 survives materialize
		{"explicit_3", intp(3), 0, intp(3), 0},
		{"token_limit_set", nil, 120000, nil, 120000},
	}
	for _, tc := range materializeCases {
		t.Run("materialize/"+tc.name, func(t *testing.T) {
			fc := &fileConfig{Generation: fileGeneration{DiffContext: tc.fileDC, TokenLimit: tc.fileTL}}
			c := materialize(fc, 0)
			if tc.wantDC == nil {
				if c.DiffContext != nil {
					t.Errorf("DiffContext = %v, want nil (materialize must not seed a default)", c.DiffContext)
				}
			} else {
				if c.DiffContext == nil {
					t.Fatalf("DiffContext = nil, want non-nil *%d", *tc.wantDC)
				}
				if *c.DiffContext != *tc.wantDC {
					t.Errorf("*DiffContext = %d, want *%d", *c.DiffContext, *tc.wantDC)
				}
			}
			if c.TokenLimit != tc.wantTL {
				t.Errorf("TokenLimit = %d, want %d", c.TokenLimit, tc.wantTL)
			}
		})
	}

	// ---- (b)/(c)/(d) overlay chain: Defaults (DiffContext=intPtr(1)) → overlay(file) ----
	// This is the load.go step the contract's broken guard sat in. overlay MUST use != nil so an
	// explicit 0 propagates and an omitted key inherits the -U1 default.
	type fileSpec struct {
		dc *int
		tl int
	}
	overlayCases := []struct {
		name   string
		global fileSpec // applied first (Defaults → overlay global)
		repo   fileSpec // applied next (→ overlay repo); dc=nil ⇒ repo omits the key
		wantDC int      // expected *cfg.DiffContext (Defaults guarantees non-nil after overlay)
		wantTL int
	}{
		// (b) global-only (repo omits both)
		{"global_only/unset", fileSpec{nil, 0}, fileSpec{nil, 0}, 1, 0},
		{"global_only/explicit_1", fileSpec{intp(1), 0}, fileSpec{nil, 0}, 1, 0},
		{"global_only/explicit_0", fileSpec{intp(0), 0}, fileSpec{nil, 0}, 0, 0}, // explicit 0 ⇒ *0 end-to-end
		{"global_only/explicit_3", fileSpec{intp(3), 0}, fileSpec{nil, 0}, 3, 0},
		// (c) repo-only (global omits both)
		{"repo_only/unset", fileSpec{nil, 0}, fileSpec{nil, 0}, 1, 0},
		{"repo_only/explicit_0", fileSpec{nil, 0}, fileSpec{intp(0), 0}, 0, 0}, // explicit 0 ⇒ *0 end-to-end
		{"repo_only/explicit_3", fileSpec{nil, 0}, fileSpec{intp(3), 0}, 3, 0},
		// (d) global+repo interactions
		{"global3_repo0_repo_wins_0", fileSpec{intp(3), 0}, fileSpec{intp(0), 0}, 0, 0}, // repo explicit-0 overrides global-3
		{"global3_repo_unset_inherits_3", fileSpec{intp(3), 0}, fileSpec{nil, 0}, 3, 0}, // repo omits ⇒ inherits global-3
		{"global0_repo3_repo_wins_3", fileSpec{intp(0), 0}, fileSpec{intp(3), 0}, 3, 0},
		// TokenLimit propagation (plain int, != 0)
		{"token_limit_global", fileSpec{nil, 120000}, fileSpec{nil, 0}, 1, 120000},
		{"token_limit_repo_overrides", fileSpec{nil, 120000}, fileSpec{nil, 80000}, 1, 80000},
	}
	for _, tc := range overlayCases {
		t.Run("overlay/"+tc.name, func(t *testing.T) {
			cfg := Defaults() // DiffContext = intPtr(1); TokenLimit = 0
			g := materialize(&fileConfig{Generation: fileGeneration{DiffContext: tc.global.dc, TokenLimit: tc.global.tl}}, 0)
			overlay(&cfg, g)
			r := materialize(&fileConfig{Generation: fileGeneration{DiffContext: tc.repo.dc, TokenLimit: tc.repo.tl}}, 0)
			overlay(&cfg, r)
			if cfg.DiffContext == nil {
				t.Fatalf("DiffContext = nil after overlay; Defaults() must seed intPtr(1) so nil is impossible here")
			}
			if *cfg.DiffContext != tc.wantDC {
				t.Errorf("*DiffContext = %d, want %d", *cfg.DiffContext, tc.wantDC)
			}
			if cfg.TokenLimit != tc.wantTL {
				t.Errorf("TokenLimit = %d, want %d", cfg.TokenLimit, tc.wantTL)
			}
		})
	}

	// ---- End-to-end via loadTOML (proves the TOML decode → *int path) ----
	// A real TOML file with [generation] diff_context = 0 must yield *cfg.DiffContext == 0 after
	// the full Defaults() → overlay(loadTOML) chain (the contract's stated end-to-end requirement).
	t.Run("loadTOML/explicit_0_end_to_end", func(t *testing.T) {
		body := `
[generation]
diff_context = 0
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.DiffContext == nil || *cfg.DiffContext != 0 {
			t.Fatalf("loadTOML DiffContext = %v, want non-nil *0 (changed-lines-only)", cfg.DiffContext)
		}
		// Now run the load.go overlay step (Defaults → overlay) — the step the contract's guard broke.
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.DiffContext == nil || *dst.DiffContext != 0 {
			t.Fatalf("after overlay: *DiffContext = %v, want *0 (explicit 0 must survive the overlay chain)", dst.DiffContext)
		}
	})

	t.Run("loadTOML/omitted_inherits_default", func(t *testing.T) {
		body := `
[generation]
max_diff_bytes = 1000
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.DiffContext != nil {
			t.Errorf("loadTOML DiffContext = %v, want nil (key omitted ⇒ materialize leaves it nil)", cfg.DiffContext)
		}
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.DiffContext == nil || *dst.DiffContext != 1 {
			t.Fatalf("after overlay: DiffContext = %v, want non-nil *1 (-U1 default inherited)", dst.DiffContext)
		}
	})
}
