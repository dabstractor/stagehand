package config

import (
	"testing"
	"time"
)

func TestResolveRoleModel_GlobalFallbackRolesNil(t *testing.T) {
	cfg := Defaults() // Roles == nil, Provider/Model/Reasoning == ""
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [global fallback, Roles nil]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off, no per-role or global set)", r)
	}
}

func TestResolveRoleModel_GlobalFallbackRoleAbsent(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	cfg.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}} // other roles set, but not "message"
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [role absent ⇒ global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_FullOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "codex-2.5-pro"},
	}
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "agy" || m != "codex-2.5-pro" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (agy,codex-2.5-pro) [full override]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"\" (off — no shipped default)", r)
	}
}

func TestResolveRoleModel_ModelOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global provider
	cfg.Roles = map[string]RoleConfig{
		"message": {Provider: "", Model: "gpt-5.4-nano"}, // model-only override
	}
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4-nano" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4-nano) [model-only: provider inherits global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_ProviderOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4" // global model
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy", Model: ""}, // provider-only override
	}
	p, m, r := ResolveRoleModel("stager", cfg)
	if p != "agy" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(stager) = (%q,%q), want (agy,gpt-5.4) [provider-only: model inherits global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_BothEmptyManifestSentinel(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model/Reasoning ""
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (\"\",\"\") [manifest-default sentinel]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"\" (off — no shipped default)", r)
	}
}

func TestResolveRoleModel_UnknownRoleFallsBackToGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m, r := ResolveRoleModel("palnner", cfg) // typo / non-canonical name
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(palnner) = (%q,%q), want (pi,gpt-5.4) [unknown role ⇒ global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(palnner) reasoning = %q, want \"\" (no shipped default for unknown role)", r)
	}
}

func TestResolveRoleModel_AllCanonicalRoles(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	// Override only planner + stager; leave message + arbiter on the global.
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "codex-2.5-pro"},
		"stager":  {Provider: "agy", Model: "codex-2.5-flash"},
	}
	want := map[string][3]string{
		"planner": {"agy", "codex-2.5-pro", ""},   // overridden provider/model; reasoning off (no shipped default)
		"stager":  {"agy", "codex-2.5-flash", ""}, // overridden provider/model; reasoning = shipped off
		"message": {"pi", "gpt-5.4", ""},          // global; reasoning = shipped off
		"arbiter": {"pi", "gpt-5.4", ""},          // global; reasoning = shipped off
	}
	for _, role := range roleNames { // roleNames is load.go's package-level canonical list (same package)
		p, m, r := ResolveRoleModel(role, cfg)
		w := want[role]
		if p != w[0] || m != w[1] || r != w[2] {
			t.Errorf("ResolveRoleModel(%s) = (%q,%q,%q), want (%q,%q,%q)", role, p, m, r, w[0], w[1], w[2])
		}
	}
}

// --- New reasoning-specific tests ---

func TestResolveRoleModel_ReasoningPerRole(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "medium" // global reasoning
	cfg.Roles = map[string]RoleConfig{
		"planner": {Reasoning: "high"}, // per-role override beats global
	}
	_, _, r := ResolveRoleModel("planner", cfg)
	if r != "high" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"high\" [per-role beats global]", r)
	}
	_, _, rm := ResolveRoleModel("stager", cfg)
	if rm != "medium" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"medium\" [global fallback, no per-role]", rm)
	}
}

func TestResolveRoleModel_ReasoningGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "low"
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy"}, // no reasoning override
	}
	_, _, r := ResolveRoleModel("stager", cfg)
	if r != "low" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"low\" [global fallback]", r)
	}
	// Planner: per-role reasoning not set, so it inherits the global "low" (no shipped planner default anymore).
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "low" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"low\" [global fallback]", rp)
	}
}

func TestResolveRoleModel_NoShippedReasoningDefault(t *testing.T) {
	// FR-R6: NO role has a non-off shipped reasoning default — not even the planner. With nothing
	// set (no per-role override, no global), every role resolves reasoning to "" (off).
	cfg := Defaults()                // Roles nil, Provider/Model/Reasoning all ""
	for _, role := range roleNames { // roleNames: load.go's package-level canonical list (same package)
		_, _, r := ResolveRoleModel(role, cfg)
		if r != "" {
			t.Errorf("ResolveRoleModel(%s) reasoning = %q, want \"\" (off — no shipped default)", role, r)
		}
	}
}

func TestResolveRoleModel_ReasoningOffIsNonZero(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "off" // explicitly set off — non-empty, so it's a real override
	_, _, r := ResolveRoleModel("planner", cfg)
	// Explicit "off" is a real (non-empty) value, so it is respected (planner inherits the global "off").
	if r != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [explicit global off respected]", r)
	}
	// Per-role "off" beats a global "high"
	cfg.Roles = map[string]RoleConfig{
		"planner": {Reasoning: "off"},
	}
	cfg.Reasoning = "high"
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [per-role off beats global high]", rp)
	}
}

// --- ResolveRoleTimeout tests (FR-R7 per-role generation timeout) ---

func TestResolveRoleTimeout_PerRoleOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from the 480s built-in so the assertion is unambiguous
	cfg.Roles = map[string]RoleConfig{"planner": {Timeout: 600 * time.Second}}
	got := ResolveRoleTimeout("planner", cfg)
	if got != 600*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 600s [per-role beats built-in 480s AND global 120s]", got)
	}
}

func TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // DISTINCT from the 480s built-in — makes the assertion unambiguous
	// Roles nil ⇒ no per-role override; planner must take its 480s BUILT-IN, NOT the 120s global.
	got := ResolveRoleTimeout("planner", cfg)
	if got != 480*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 480s (built-in beats 120s global)", got)
	}
}

func TestResolveRoleTimeout_NonPlannerGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from the 480s planner built-in
	for _, role := range []string{"stager", "message", "arbiter"} {
		got := ResolveRoleTimeout(role, cfg)
		if got != 120*time.Second {
			t.Errorf("ResolveRoleTimeout(%s) = %v, want 120s [no built-in ⇒ global]", role, got)
		}
	}
}

func TestResolveRoleTimeout_FieldMergeTimeoutOnly(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second
	cfg.Roles = map[string]RoleConfig{"message": {Provider: "pi", Timeout: 0}} // Timeout 0 ⇒ inherit global
	got := ResolveRoleTimeout("message", cfg)
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 120s [Timeout 0 inherits global; Provider is irrelevant to timeout]", got)
	}
}

func TestResolveRoleTimeout_UnknownRoleGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second
	got := ResolveRoleTimeout("palnner", cfg) // typo / non-canonical name
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(palnner) = %v, want 120s [unknown role ⇒ global, no built-in]", got)
	}
}

func TestResolveRoleTimeout_RolesNilGlobalFallback(t *testing.T) {
	cfg := Defaults() // Roles is nil from Defaults()
	cfg.Timeout = 120 * time.Second
	got := ResolveRoleTimeout("message", cfg)
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 120s [Roles nil ⇒ global fallback]", got)
	}
}

// TestResolveRoleTimeout_MessagePerRoleOverride closes GAP1 (FR-R7 clause (a)): NO LANDED test sets a
// NON-ZERO cfg.Roles["message"].Timeout. TestResolveRoleTimeout_FieldMergeTimeoutOnly uses message with
// Timeout==0 (inherit); _PerRoleOverride uses planner. The message role is the ONLY active role on the
// single-commit path, so its non-zero per-role-override branch is the highest-value untested case — a
// refactor that dropped the non-planner per-role-override path would pass every LANDED test but fail here.
func TestResolveRoleTimeout_MessagePerRoleOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from the 90s override so the assertion is unambiguous
	cfg.Roles = map[string]RoleConfig{"message": {Timeout: 90 * time.Second}}
	got := ResolveRoleTimeout("message", cfg)
	if got != 90*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 90s [message has no built-in; per-role override beats 120s global]", got)
	}
}

// TestResolveRoleTimeout_AllCanonicalRoles closes GAP2 (FR-R7 consolidation): the timeout axis had no
// "all canonical roles" table, unlike TestResolveRoleModel_AllCanonicalRoles (line 108) for the model
// axis. One table over roleNames proves the full FR-R7 resolution matrix in a single readable place:
// planner → per-role override (beats the 480s built-in); stager → per-role override (beats the 120s
// global); message/arbiter (absent from cfg.Roles, no built-in) → 120s global. The 6 LANDED ad-hoc tests
// cover these branches in isolation; this is the missing consolidated twin.
func TestResolveRoleTimeout_AllCanonicalRoles(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from 480s so the planner built-in is unambiguous
	cfg.Roles = map[string]RoleConfig{
		"planner": {Timeout: 600 * time.Second}, // override beats the 480s built-in
		"stager":  {Timeout: 60 * time.Second},  // override beats the 120s global
		// message, arbiter: no entry ⇒ no built-in ⇒ 120s global
	}
	want := map[string]time.Duration{
		"planner": 600 * time.Second, // per-role override
		"stager":  60 * time.Second,  // per-role override
		"message": 120 * time.Second, // no built-in, no override ⇒ global
		"arbiter": 120 * time.Second, // no built-in, no override ⇒ global
	}
	for _, role := range roleNames { // roleNames: load.go package-level canonical list
		got := ResolveRoleTimeout(role, cfg)
		if got != want[role] {
			t.Errorf("ResolveRoleTimeout(%s) = %v, want %v", role, got, want[role])
		}
	}
}

// TestResolveRoleTimeout_PlannerDefaultFromDefaults closes GAP3 (FR-R7 clause (e) explicit): the default
// change (P1.M2.T2.S1 flipped the global 480s→120s) is proven functionally by _PlannerBuiltinBeatsGlobal
// (Defaults()+set Timeout=120s — identical, since Defaults().Timeout IS 120s) but no LANDED test calls
// ResolveRoleTimeout with the literal Defaults(). This one-liner makes "the planner still gets 480s after
// the global dropped to 120s" a named, greppable assertion.
func TestResolveRoleTimeout_PlannerDefaultFromDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Timeout != 120*time.Second {
		t.Fatalf("Defaults().Timeout = %v, want 120s (the FR-R7 global default)", cfg.Timeout)
	}
	if got := ResolveRoleTimeout("planner", cfg); got != 480*time.Second {
		t.Errorf("ResolveRoleTimeout(planner, Defaults()) = %v, want 480s (built-in beats 120s global)", got)
	}
}
