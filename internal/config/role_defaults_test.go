package config

import "testing"

// TestDefaultModelsForProvider_PerProvider asserts each of the 7 built-in providers returns its
// expected 4-role column (hardcoded — NOT derived from the table, so the test is meaningful).
// PINS stager="" for the 5 non-stager-capable providers and non-empty stager for pi/claude.
func TestDefaultModelsForProvider_PerProvider(t *testing.T) {
	want := map[string]map[string]string{
		"pi": {
			"planner": "gpt-5.4", "stager": "gpt-5.4-mini", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-mini",
		},
		"claude": {
			"planner": "opus", "stager": "sonnet", "message": "haiku", "arbiter": "sonnet",
		},
		"gemini": {
			"planner": "gemini-3.5-pro", "stager": "", "message": "gemini-3.1-flash-lite", "arbiter": "gemini-3.5-flash",
		},
		"agy": {
			"planner": "gemini-3.5-pro", "stager": "", "message": "gemini-3.1-flash-lite", "arbiter": "gemini-3.5-flash",
		},
		"opencode": {
			"planner": "openai/gpt-5.4", "stager": "", "message": "openai/gpt-5.4-nano", "arbiter": "openai/gpt-5.4-mini",
		},
		"codex": {
			"planner": "gpt-5.1-codex-max", "stager": "", "message": "gpt-5.4-nano", "arbiter": "gpt-5.1-codex-mini",
		},
		"cursor": {
			"planner": "gpt-5.4", "stager": "", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-mini",
		},
	}
	for name, exp := range want {
		got := DefaultModelsForProvider(name)
		if got == nil {
			t.Errorf("DefaultModelsForProvider(%q) = nil, want a column", name)
			continue
		}
		for role, m := range exp {
			if got[role] != m {
				t.Errorf("DefaultModelsForProvider(%q)[%q] = %q, want %q", name, role, got[role], m)
			}
		}
	}
}

// TestDefaultModelsForProvider_AllRolesPresent asserts every known provider column has exactly the
// 4 canonical role keys (planner/stager/message/arbiter), including stager when its value is "".
func TestDefaultModelsForProvider_AllRolesPresent(t *testing.T) {
	roles := []string{"planner", "stager", "message", "arbiter"}
	for _, name := range []string{"pi", "claude", "gemini", "agy", "opencode", "codex", "cursor"} {
		col := DefaultModelsForProvider(name)
		if col == nil {
			t.Errorf("DefaultModelsForProvider(%q) = nil, want a column", name)
			continue
		}
		if len(col) != 4 {
			t.Errorf("DefaultModelsForProvider(%q) has %d roles, want 4", name, len(col))
		}
		for _, role := range roles {
			if _, ok := col[role]; !ok {
				t.Errorf("DefaultModelsForProvider(%q) missing role key %q", name, role)
			}
		}
	}
}

// TestDefaultModelsForProvider_StagerCapability isolates the stager="" signal: pi+claude have
// non-empty stager (TooledFlags set in builtin.go); the other 5 have stager=="" (nil TooledFlags).
func TestDefaultModelsForProvider_StagerCapability(t *testing.T) {
	for _, capable := range []string{"pi", "claude"} {
		if m := DefaultModelsForProvider(capable)["stager"]; m == "" {
			t.Errorf("%q should be stager-capable (non-empty stager), got %q", capable, m)
		}
	}
	for _, incapable := range []string{"gemini", "agy", "opencode", "codex", "cursor"} {
		if m := DefaultModelsForProvider(incapable)["stager"]; m != "" {
			t.Errorf("%q must have stager==\"\" (not stager-capable), got %q", incapable, m)
		}
	}
}

// TestDefaultModelsForProvider_UnknownReturnsNil asserts an unknown provider name returns nil.
func TestDefaultModelsForProvider_UnknownReturnsNil(t *testing.T) {
	if got := DefaultModelsForProvider("nonexistent"); got != nil {
		t.Errorf("DefaultModelsForProvider(\"nonexistent\") = %v, want nil", got)
	}
}

// TestDefaultModelsForProvider_CopySemantics asserts that mutating a returned map does NOT affect
// the package-level table (DefaultModelsForProvider must return a defensive copy).
func TestDefaultModelsForProvider_CopySemantics(t *testing.T) {
	first := DefaultModelsForProvider("pi")
	first["stager"] = "MUTATED"
	second := DefaultModelsForProvider("pi")
	if second["stager"] != "gpt-5.4-mini" {
		t.Errorf("table was mutated via returned map: second call stager = %q, want gpt-5.4-mini (must return a copy)", second["stager"])
	}
}

// TestRoleDefaults_KeySanity asserts the table has exactly the 7 built-in provider keys and no
// provider column contains a role key outside the canonical set {planner, stager, message, arbiter}.
func TestRoleDefaults_KeySanity(t *testing.T) {
	expectedProviders := map[string]bool{
		"pi": true, "claude": true, "gemini": true, "opencode": true,
		"codex": true, "cursor": true, "agy": true,
	}
	validRoles := map[string]bool{
		"planner": true, "stager": true, "message": true, "arbiter": true,
	}

	if len(roleDefaults) != len(expectedProviders) {
		t.Errorf("roleDefaults has %d providers, want %d", len(roleDefaults), len(expectedProviders))
	}
	for p := range roleDefaults {
		if !expectedProviders[p] {
			t.Errorf("roleDefaults has unexpected provider key %q", p)
		}
		for role := range roleDefaults[p] {
			if !validRoles[role] {
				t.Errorf("roleDefaults[%q] has unexpected role key %q", p, role)
			}
		}
	}
}
