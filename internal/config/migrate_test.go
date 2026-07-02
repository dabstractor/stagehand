package config

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// migrateV2ToV3 — table-driven tests (all migration branches)
// ---------------------------------------------------------------------------

func TestMigrateV2ToV3(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		wantFn func(t *testing.T, cfg *Config) // assertions on the MUTATED cfg
	}{
		{
			name: "global pi model folded",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: map[string]map[string]any{"pi": {"default_provider": "zai"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "zai/glm-5.2" {
					t.Errorf("Model=%q want zai/glm-5.2", cfg.Model)
				}
				if _, ok := cfg.Providers["pi"]["default_provider"]; ok {
					t.Error("default_provider should be deleted")
				}
			},
		},
		{
			name: "per-role folded with explicit provider",
			cfg: Config{
				Provider: "claude",
				Roles: map[string]RoleConfig{
					"planner": {Provider: "pi", Model: "glm-5.2"},
				},
				Providers: map[string]map[string]any{"pi": {"default_provider": "zai"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if rc := cfg.Roles["planner"]; rc.Model != "zai/glm-5.2" {
					t.Errorf("Roles[planner].Model=%q want zai/glm-5.2", rc.Model)
				}
			},
		},
		{
			name: "per-role inherits global provider",
			cfg: Config{
				Provider: "pi",
				Roles: map[string]RoleConfig{
					"message": {Model: "glm-5.2"}, // no Provider — inherits global "pi"
				},
				Providers: map[string]map[string]any{"pi": {"default_provider": "zai"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if rc := cfg.Roles["message"]; rc.Model != "zai/glm-5.2" {
					t.Errorf("Roles[message].Model=%q want zai/glm-5.2", rc.Model)
				}
			},
		},
		{
			name: "raw map default_model folded and default_provider deleted",
			cfg: Config{
				Provider: "pi",
				Providers: map[string]map[string]any{
					"pi": {"default_provider": "zai", "default_model": "glm-5.2"},
				},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Providers["pi"]["default_model"] != "zai/glm-5.2" {
					t.Errorf("pi.default_model=%q want zai/glm-5.2", cfg.Providers["pi"]["default_model"])
				}
				if _, ok := cfg.Providers["pi"]["default_provider"]; ok {
					t.Error("default_provider should be deleted")
				}
			},
		},
		{
			name: "idempotent — already prefixed model untouched",
			cfg: Config{
				Provider:  "pi",
				Model:     "zai/glm-5.2",
				Providers: map[string]map[string]any{"pi": {"default_provider": "zai", "default_model": "zai/glm-5.2"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "zai/glm-5.2" {
					t.Errorf("Model=%q want zai/glm-5.2 (unchanged)", cfg.Model)
				}
				if cfg.Providers["pi"]["default_model"] != "zai/glm-5.2" {
					t.Errorf("pi.default_model=%q want zai/glm-5.2 (unchanged)", cfg.Providers["pi"]["default_model"])
				}
			},
		},
		{
			name: "single-backend claude untouched — key dropped, model NOT prefixed",
			cfg: Config{
				Provider: "claude",
				Model:    "opus",
				Providers: map[string]map[string]any{
					"claude": {"default_provider": "anthropic", "default_model": "opus"},
				},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "opus" {
					t.Errorf("Model=%q want opus (single-backend untouched)", cfg.Model)
				}
				if _, ok := cfg.Providers["claude"]["default_provider"]; ok {
					t.Error("default_provider should be deleted (dead key)")
				}
				if cfg.Providers["claude"]["default_model"] != "opus" {
					t.Errorf("claude.default_model=%q want opus (NOT prefixed)", cfg.Providers["claude"]["default_model"])
				}
			},
		},
		{
			name: "empty default_provider — key dropped, no fold",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: map[string]map[string]any{"pi": {"default_provider": ""}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "glm-5.2" {
					t.Errorf("Model=%q want glm-5.2 (no fold for empty default_provider)", cfg.Model)
				}
				if _, ok := cfg.Providers["pi"]["default_provider"]; ok {
					t.Error("default_provider should be deleted")
				}
			},
		},
		{
			name: "bare pi model with NO default_provider — stays bare (no-invent)",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: map[string]map[string]any{"pi": {"default_model": "glm-5.2"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "glm-5.2" {
					t.Errorf("Model=%q want glm-5.2 (no-invent: no default_provider, stays bare)", cfg.Model)
				}
			},
		},
		{
			name: "nil Providers — no panic",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: nil,
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "glm-5.2" {
					t.Errorf("Model=%q want glm-5.2 (nil Providers — no-op)", cfg.Model)
				}
			},
		},
		{
			name: "nil Roles — no panic",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Roles:     nil,
				Providers: map[string]map[string]any{"pi": {"default_provider": "zai"}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "zai/glm-5.2" {
					t.Errorf("Model=%q want zai/glm-5.2 (global folded, nil Roles OK)", cfg.Model)
				}
			},
		},
		{
			name: "no providers at all — no panic",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: map[string]map[string]any{},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "glm-5.2" {
					t.Errorf("Model=%q want glm-5.2 (empty Providers — no-op)", cfg.Model)
				}
			},
		},
		{
			name: "non-string default_provider — key dropped, no fold",
			cfg: Config{
				Provider:  "pi",
				Model:     "glm-5.2",
				Providers: map[string]map[string]any{"pi": {"default_provider": 42}},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "glm-5.2" {
					t.Errorf("Model=%q want glm-5.2 (non-string dp — no fold)", cfg.Model)
				}
				if _, ok := cfg.Providers["pi"]["default_provider"]; ok {
					t.Error("default_provider should be deleted even for non-string")
				}
			},
		},
		{
			name: "user-defined multi-backend via provider_flag",
			cfg: Config{
				Provider: "myagent",
				Model:    "my-model",
				Providers: map[string]map[string]any{
					"myagent": {"default_provider": "custom-backend", "provider_flag": "--backend"},
				},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "custom-backend/my-model" {
					t.Errorf("Model=%q want custom-backend/my-model (user-defined multi-backend)", cfg.Model)
				}
			},
		},
		{
			name: "user-defined single-backend via empty provider_flag — no fold",
			cfg: Config{
				Provider: "myagent",
				Model:    "my-model",
				Providers: map[string]map[string]any{
					"myagent": {"default_provider": "custom-backend", "provider_flag": ""},
				},
			},
			wantFn: func(t *testing.T, cfg *Config) {
				if cfg.Model != "my-model" {
					t.Errorf("Model=%q want my-model (empty provider_flag — single-backend, no fold)", cfg.Model)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg // copy
			migrateV2ToV3(&cfg)
			tc.wantFn(t, &cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// migrationNotice — pure output tests
// ---------------------------------------------------------------------------

func TestMigrationNotice(t *testing.T) {
	tests := []struct {
		name     string
		version  int
		contains []string
	}{
		{"version 0 (no config_version)", 0, []string{"no config_version", "config upgrade"}},
		{"version 2", 2, []string{"schema version 2", "current 3", "config upgrade"}},
		{"version 1", 1, []string{"schema version 1", "current 3", "config upgrade"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := migrationNotice(tc.version)
			if got == "" {
				t.Fatal("migrationNotice returned empty")
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("notice = %q, want \\n-terminated", got)
			}
			for _, sub := range tc.contains {
				if !strings.Contains(got, sub) {
					t.Errorf("notice = %q, want to contain %q", got, sub)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isMultiBackend — classifier tests
// ---------------------------------------------------------------------------

func TestIsMultiBackend(t *testing.T) {
	tests := []struct {
		name string
		prov string
		raw  map[string]any
		want bool
	}{
		{"pi is multi-backend", "pi", map[string]any{}, true},
		{"claude is NOT multi-backend", "claude", map[string]any{}, false},
		{"user-defined with provider_flag", "myagent", map[string]any{"provider_flag": "--x"}, true},
		{"user-defined with empty provider_flag", "myagent", map[string]any{"provider_flag": ""}, false},
		{"unknown with no provider_flag", "unknown", map[string]any{}, false},
		{"nil raw map", "pi", nil, true}, // builtin check still works
		{"provider_flag non-string (int)", "myagent", map[string]any{"provider_flag": 42}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMultiBackend(tc.prov, tc.raw)
			if got != tc.want {
				t.Errorf("isMultiBackend(%q, %v) = %v, want %v", tc.prov, tc.raw, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// v2MultiBackendBuiltins — exported for test inspection
// ---------------------------------------------------------------------------

func TestV2MultiBackendBuiltins(t *testing.T) {
	if !v2MultiBackendBuiltins["pi"] {
		t.Error("v2MultiBackendBuiltins should contain pi")
	}
	if v2MultiBackendBuiltins["claude"] {
		t.Error("v2MultiBackendBuiltins should NOT contain claude")
	}
}
