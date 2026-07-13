package decompose

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/provider"
)

// goRegistry builds a Registry with overrides that set Command to "go" on the named providers,
// making them "installed" (exec.LookPath("go") succeeds in CI). Other built-ins keep their
// default Command (e.g. "pi", "claude") which are typically absent → not installed.
func goRegistry(t *testing.T, names []string, extraOverrides map[string]provider.Manifest) *provider.Registry {
	t.Helper()
	overrides := make(map[string]provider.Manifest, len(names)+len(extraOverrides))
	for _, name := range names {
		cmd := "go"
		overrides[name] = provider.Manifest{Command: &cmd}
	}
	for name, ov := range extraOverrides {
		if base, ok := overrides[name]; ok {
			// Merge extra fields onto the install override WITHOUT dropping
			// Command="go" (which makes the agent "installed"). Models a correctly-configured user.
			overrides[name] = provider.MergeManifest(base, ov)
		} else {
			overrides[name] = ov
		}
	}
	return provider.NewRegistry(overrides)
}

// bogusRegistry builds a Registry where all built-in providers are overridden to a bogus command
// (not on $PATH), making none of them installed. Then `installed` is overridden to "go" only for the
// named providers. This is used when a test needs to control exactly which providers appear installed
// regardless of what binaries happen to be on the dev machine (e.g. pi may be present).
func bogusRegistry(t *testing.T, installed []string) *provider.Registry {
	t.Helper()
	overrides := make(map[string]provider.Manifest)
	// Override ALL built-in names to a bogus command AND detect (DetectCommand returns Detect first).
	bogus := "definitely-not-a-real-command-xyzzy"
	for _, name := range []string{"pi", "opencode", "cursor", "agy", "codex", "claude"} {
		overrides[name] = provider.Manifest{Command: &bogus, Detect: &bogus}
	}
	// Then override the "installed" ones back to "go".
	goCmd := "go"
	for _, name := range installed {
		overrides[name] = provider.Manifest{Command: &goCmd, Detect: &goCmd}
	}
	return provider.NewRegistry(overrides)
}

// piManifest returns the merged pi manifest from a fresh registry (no overrides).
func piManifest(t *testing.T) provider.Manifest {
	t.Helper()
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("pi")
	if !ok {
		t.Fatal("pi not found in built-in manifests")
	}
	return m
}

// claudeManifest returns the merged claude manifest from a fresh registry (no overrides).
func claudeManifest(t *testing.T) provider.Manifest {
	t.Helper()
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("claude")
	if !ok {
		t.Fatal("claude not found in built-in manifests")
	}
	return m
}

// agyManifest returns the merged agy manifest from a fresh registry (no overrides).
func agyManifest(t *testing.T) provider.Manifest {
	t.Helper()
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("agy")
	if !ok {
		t.Fatal("agy not found in built-in manifests")
	}
	return m
}

// ---------------------------------------------------------------------------
// TestResolveRoles_HappyPath_AllPi
// ---------------------------------------------------------------------------

func TestResolveRoles_HappyPath_AllPi(t *testing.T) {
	// All 4 roles resolve to pi (global provider=pi, no per-role override). pi is multi-provider, so a
	// pinned model MUST use the slash-prefix form (FR-R5b) — model a correctly-configured user here.
	reg := bogusRegistry(t, []string{"pi"})
	wantPi := piManifest(t)

	cfg := config.Config{
		Provider: "pi",
		Model:    "zai/gpt-5.4-nano", // slash-prefix: inference backend + model (FR-R5b)
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}

	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		// All manifests should be pi's manifest.
		got := roleManifest(rm, role)
		if got.Name != wantPi.Name {
			t.Errorf("role %q manifest.Name = %q, want %q", role, got.Name, wantPi.Name)
		}
		// All providers should be "pi".
		rc := roleModel(rmodels, role)
		if rc.Provider != "pi" {
			t.Errorf("role %q provider = %q, want pi", role, rc.Provider)
		}
		// Model should be the global model (inherited via ResolveRoleModel).
		if rc.Model != "zai/gpt-5.4-nano" {
			t.Errorf("role %q model = %q, want zai/gpt-5.4-nano", role, rc.Model)
		}
	}

	// Stager TooledFlags should be non-empty (pi is stager-capable — no fallback needed).
	if len(rm.Stager.TooledFlags) == 0 {
		t.Error("Stager.TooledFlags is empty, want non-empty (pi is capable)")
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_StagerFallback
// ---------------------------------------------------------------------------

func TestResolveRoles_StagerFallback(t *testing.T) {
	// Stager is set to agy (TooledFlags nil → cannot stage); fallback to claude (first tooled-capable
	// installed after agy). Pi is NOT installed in this fixture (so fallback skips it). Claude is
	// single-backend (ProviderFlag="") so the FR-R5b guard does not fire on the fallback model.
	reg := bogusRegistry(t, []string{"agy", "claude"})
	wantClaude := claudeManifest(t)

	cfg := config.Config{
		Provider: "agy",
		Roles: map[string]config.RoleConfig{
			"stager": {Provider: "agy", Model: "agy-2.5-pro"},
		},
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}

	// Stager should have fallen back to claude.
	if rm.Stager.Name != wantClaude.Name {
		t.Errorf("Stager manifest.Name = %q, want %q", rm.Stager.Name, wantClaude.Name)
	}
	if rmodels.Stager.Provider != "claude" {
		t.Errorf("Stager provider = %q, want claude", rmodels.Stager.Provider)
	}

	// Stager model should be claude's stager default from the FR-D4 table.
	wantStagerModel := config.DefaultModelsForProvider("claude")["stager"]
	if rmodels.Stager.Model != wantStagerModel {
		t.Errorf("Stager model = %q, want %q", rmodels.Stager.Model, wantStagerModel)
	}

	// Stager TooledFlags should be non-empty (fallback to claude which is capable).
	if len(rm.Stager.TooledFlags) == 0 {
		t.Error("Stager.TooledFlags is empty after fallback, want non-empty")
	}

	// Other roles should be agy (global default).
	for _, role := range []string{"planner", "message", "arbiter"} {
		rc := roleModel(rmodels, role)
		if rc.Provider != "agy" {
			t.Errorf("role %q provider = %q, want agy", role, rc.Provider)
		}
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_StagerFallback_PiNotInstalled_FallsToClaude
// ---------------------------------------------------------------------------

func TestResolveRoles_StagerFallback_PiNotInstalled_FallsToClaude(t *testing.T) {
	// Pi NOT installed; agy is the global (not stager-capable); claude IS installed and capable.
	// Stager set to agy → fallback should go to claude (pi is not installed).
	reg := bogusRegistry(t, []string{"agy", "claude"})
	wantClaude := claudeManifest(t)

	cfg := config.Config{
		Provider: "agy",
		Roles: map[string]config.RoleConfig{
			"stager": {Provider: "agy"},
		},
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}

	if rm.Stager.Name != wantClaude.Name {
		t.Errorf("Stager manifest.Name = %q, want %q (claude)", rm.Stager.Name, wantClaude.Name)
	}
	if rmodels.Stager.Provider != "claude" {
		t.Errorf("Stager provider = %q, want claude", rmodels.Stager.Provider)
	}
	wantStagerModel := config.DefaultModelsForProvider("claude")["stager"]
	if rmodels.Stager.Model != wantStagerModel {
		t.Errorf("Stager model = %q, want %q", rmodels.Stager.Model, wantStagerModel)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_StagerFallback_ToPi_MultiProviderModel
// ---------------------------------------------------------------------------

func TestResolveRoles_StagerFallback_ToPi_MultiProviderModel(t *testing.T) {
	// Stager is set to agy (TooledFlags nil → cannot stage); fallback to pi (first
	// tooled-capable installed). Pi is multi-provider (ProviderFlag="--provider").
	// Tests two sub-cases via subtests:
	//   - bare model from the old provider's config → cleared (invalid on pi)
	//   - no explicit model → roleDefaults["pi"]["stager"] bare default also skipped
	// In both cases mdl must remain "" so the manifest DefaultModel resolves it.

	tests := []struct {
		name     string
		stagerRC config.RoleConfig
	}{
		{
			name:     "bare_model_from_old_provider",
			stagerRC: config.RoleConfig{Provider: "agy", Model: "agy-2.5-pro"},
		},
		{
			name:     "no_explicit_model",
			stagerRC: config.RoleConfig{Provider: "agy"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := bogusRegistry(t, []string{"agy", "pi"})

			cfg := config.Config{
				Provider: "agy",
				Roles: map[string]config.RoleConfig{
					"stager": tc.stagerRC,
				},
			}

			rm, rmodels, err := ResolveRoles(cfg, reg)
			if err != nil {
				t.Fatalf("ResolveRoles: %v", err)
			}

			// Stager should have fallen back to pi.
			if rm.Stager.Name != "pi" {
				t.Errorf("Stager manifest.Name = %q, want pi", rm.Stager.Name)
			}
			if rmodels.Stager.Provider != "pi" {
				t.Errorf("Stager provider = %q, want pi", rmodels.Stager.Provider)
			}

			// mdl must be "" — bare models are cleared when falling back to a
			// multi-provider agent (FR-R5b violation avoided).
			if rmodels.Stager.Model != "" {
				t.Errorf("Stager model = %q, want empty string (bare model cleared for multi-provider pi)", rmodels.Stager.Model)
			}

			// Other roles should remain agy (global default).
			for _, role := range []string{"planner", "message", "arbiter"} {
				rc := roleModel(rmodels, role)
				if rc.Provider != "agy" {
					t.Errorf("role %q provider = %q, want agy", role, rc.Provider)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_NoStagerCapable
// ---------------------------------------------------------------------------

func TestResolveRoles_NoStagerCapable(t *testing.T) {
	// Stager set to agy (not capable); pi and claude NOT installed → no fallback possible.
	// Only agy is installed (via Command="go" override); all others have bogus commands.
	reg := bogusRegistry(t, []string{"agy"})

	cfg := config.Config{
		Roles: map[string]config.RoleConfig{
			"stager": {Provider: "agy"},
		},
	}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error, want stager-capable error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "cannot stage") || !strings.Contains(errMsg, "stager-capable") {
		t.Errorf("error = %q, want stager-capable message", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_FR5b_BareModelOnPi
// ---------------------------------------------------------------------------

func TestResolveRoles_FR5b_BareModelOnPi(t *testing.T) {
	// Planner has a model set but NO provider; auto-detect picks pi (multi-provider) → FR-R5b error
	// because "glm-5-turbo" has no slash-prefix.
	reg := bogusRegistry(t, []string{"pi"})

	cfg := config.Config{
		Roles: map[string]config.RoleConfig{
			"planner": {Model: "glm-5-turbo"},
		},
	}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error, want FR-R5b model-prefix error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "must be inference/model") || !strings.Contains(errMsg, "planner") {
		t.Errorf("error = %q, want role-named must be inference/model error", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_FR5b_BareModelOnClaude_NoError
// ---------------------------------------------------------------------------

func TestResolveRoles_FR5b_BareModelOnClaude_NoError(t *testing.T) {
	// Planner has model but no provider; auto-detect picks claude (NOT multi-provider by
	// ProviderFlag signal) → no error. Claude is installed; pi is not.
	reg := bogusRegistry(t, []string{"claude"})

	cfg := config.Config{
		Roles: map[string]config.RoleConfig{
			"planner": {Model: "haiku"},
		},
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}

	// Planner should resolve to claude.
	if rmodels.Planner.Provider != "claude" {
		t.Errorf("Planner provider = %q, want claude", rmodels.Planner.Provider)
	}
	if rmodels.Planner.Model != "haiku" {
		t.Errorf("Planner model = %q, want haiku", rmodels.Planner.Model)
	}
	// Verify pi manifest is NOT multi-provider — just sanity check.
	if rm.Planner.Name != "claude" {
		t.Errorf("Planner manifest.Name = %q, want claude", rm.Planner.Name)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_FR5b_ProviderSet_NoInferenceProvider
// ---------------------------------------------------------------------------

func TestResolveRoles_FR5b_BareModelOnExplicitPi(t *testing.T) {
	// Provider="pi" (the AGENT name, multi-provider) + a bare model (no slash-prefix).
	// This is a misconfiguration: pi requires "inference/model" form. FR-R5b forbids it.
	reg := bogusRegistry(t, []string{"pi"})

	cfg := config.Config{
		Provider: "pi",
		Roles: map[string]config.RoleConfig{
			"planner": {Provider: "pi", Model: "gpt-5.4"},
		},
	}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error; want FR-R5b model-prefix error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "must be inference/model") || !strings.Contains(errMsg, "planner") {
		t.Errorf("error = %q, want role-named must be inference/model error", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_FR5b_InferenceProviderSet_NoError
// ---------------------------------------------------------------------------

func TestResolveRoles_FR5b_SlashPrefixModel_NoError(t *testing.T) {
	// A slash-prefix model on pi → no error. This is the correctly-configured pi user
	// (the fix path FR-R5b is meant to guide users TO).
	reg := bogusRegistry(t, []string{"pi"})

	cfg := config.Config{
		Provider: "pi",
		Roles: map[string]config.RoleConfig{
			"planner": {Provider: "pi", Model: "zai/gpt-5.4"},
		},
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}
	if rmodels.Planner.Provider != "pi" {
		t.Errorf("Planner provider = %q, want pi", rmodels.Planner.Provider)
	}
	if rmodels.Planner.Model != "zai/gpt-5.4" {
		t.Errorf("Planner model = %q, want zai/gpt-5.4", rmodels.Planner.Model)
	}
	if rm.Planner.Name != "pi" {
		t.Errorf("Planner manifest.Name = %q, want pi", rm.Planner.Name)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_MissingProvider
// ---------------------------------------------------------------------------

func TestResolveRoles_MissingProvider(t *testing.T) {
	// No provider set anywhere; no built-in installed.
	reg := bogusRegistry(t, nil)

	cfg := config.Config{}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error, want missing-provider error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no provider configured") {
		t.Errorf("error = %q, want no-provider-configured error", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_UnknownProvider
// ---------------------------------------------------------------------------

func TestResolveRoles_UnknownProvider(t *testing.T) {
	reg := goRegistry(t, []string{"pi"}, nil)

	cfg := config.Config{
		Provider: "nope",
	}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error, want unknown-provider error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unknown provider") {
		t.Errorf("error = %q, want unknown-provider error", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_Uninstalled
// ---------------------------------------------------------------------------

func TestResolveRoles_Uninstalled(t *testing.T) {
	// Provider set to "pi" but pi is NOT installed (overridden to bogus command).
	reg := bogusRegistry(t, nil)

	cfg := config.Config{
		Provider: "pi",
	}

	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles returned nil error, want uninstalled error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("error = %q, want command-not-found error", errMsg)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_PerRoleOverrides
// ---------------------------------------------------------------------------

func TestResolveRoles_PerRoleOverrides(t *testing.T) {
	// Per-role overrides: planner=claude, stager=pi (slash-prefix model, FR-R5b), message=claude,
	// arbiter=pi (no model → use manifest default). Claude is single-backend (no provider needed);
	// pi roles use slash-prefix models (FR-R5b).
	reg := bogusRegistry(t, []string{"pi", "claude"})

	cfg := config.Config{
		Provider: "claude", // global default
		Roles: map[string]config.RoleConfig{
			"planner": {Provider: "claude", Model: "opus"},
			"stager":  {Provider: "pi", Model: "zai/gpt-5.4-mini"},
			"message": {Provider: "claude", Model: "haiku"},
			"arbiter": {Provider: "pi"},
		},
	}

	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}

	// Verify per-role resolution.
	tests := []struct {
		role         string
		wantProvider string
		wantModel    string
	}{
		{"planner", "claude", "opus"},
		{"stager", "pi", "zai/gpt-5.4-mini"},
		{"message", "claude", "haiku"},
		{"arbiter", "pi", ""},
	}
	for _, tt := range tests {
		rc := roleModel(rmodels, tt.role)
		if rc.Provider != tt.wantProvider {
			t.Errorf("role %q provider = %q, want %q", tt.role, rc.Provider, tt.wantProvider)
		}
		if rc.Model != tt.wantModel {
			t.Errorf("role %q model = %q, want %q", tt.role, rc.Model, tt.wantModel)
		}
		m := roleManifest(rm, tt.role)
		if m.Name != tt.wantProvider {
			t.Errorf("role %q manifest.Name = %q, want %q", tt.role, m.Name, tt.wantProvider)
		}
	}
}

// ---------------------------------------------------------------------------
// TestResolveRoles_NoShippedReasoningDefault
// ---------------------------------------------------------------------------

func TestResolveRoles_NoShippedReasoningDefault(t *testing.T) {
	// FR-R6: NO role has a non-off shipped reasoning default — not even the planner. With nothing
	// set, every role resolves reasoning to "" (off). Claude is single-backend ⇒ no FR-R5b guard.
	reg := bogusRegistry(t, []string{"claude"})

	cfg := config.Config{} // nothing set

	_, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}
	if rmodels.Planner.Reasoning != "" {
		t.Errorf("planner reasoning = %q, want \"\" (off — no shipped default)", rmodels.Planner.Reasoning)
	}
	if rmodels.Stager.Reasoning != "" {
		t.Errorf("stager reasoning = %q, want \"\" (off)", rmodels.Stager.Reasoning)
	}
	if rmodels.Message.Reasoning != "" {
		t.Errorf("message reasoning = %q, want \"\" (off)", rmodels.Message.Reasoning)
	}
	if rmodels.Arbiter.Reasoning != "" {
		t.Errorf("arbiter reasoning = %q, want \"\" (off)", rmodels.Arbiter.Reasoning)
	}
}

func TestResolveRoles_ReasoningPerRoleSet(t *testing.T) {
	// Per-role reasoning override: message sets reasoning="low"; planner stays "" (off — no shipped default).
	reg := bogusRegistry(t, []string{"claude"})

	cfg := config.Config{
		Roles: map[string]config.RoleConfig{
			"message": {Reasoning: "low"},
		},
	}

	_, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil {
		t.Fatalf("ResolveRoles: %v", err)
	}
	if rmodels.Planner.Reasoning != "" {
		t.Errorf("planner reasoning = %q, want \"\" (off — no shipped default)", rmodels.Planner.Reasoning)
	}
	if rmodels.Message.Reasoning != "low" {
		t.Errorf("message reasoning = %q, want low (per-role override)", rmodels.Message.Reasoning)
	}
}

// ---------------------------------------------------------------------------
// TestComputeInstalled
// ---------------------------------------------------------------------------

func TestComputeInstalled(t *testing.T) {
	reg := goRegistry(t, []string{"pi", "claude"}, nil)
	installed := computeInstalled(reg)

	// "go" is on $PATH, so pi and claude (both overridden to Command="go") are installed.
	found := map[string]bool{}
	for _, name := range installed {
		found[name] = true
	}
	if !found["pi"] {
		t.Error("pi not in computeInstalled result")
	}
	if !found["claude"] {
		t.Error("claude not in computeInstalled result")
	}
}

// ---------------------------------------------------------------------------
// TestIsMultiProvider
// ---------------------------------------------------------------------------

func TestIsMultiProvider(t *testing.T) {
	pi := piManifest(t)
	claude := claudeManifest(t)
	agy := agyManifest(t)

	if !isMultiProvider(pi) {
		t.Error("pi should be multi-provider (ProviderFlag=\"--provider\")")
	}
	if isMultiProvider(claude) {
		t.Error("claude should NOT be multi-provider (ProviderFlag=\"\")")
	}
	if isMultiProvider(agy) {
		t.Error("agy should NOT be multi-provider (ProviderFlag=\"\")")
	}

	// Nil ProviderFlag (hypothetical override).
	var empty provider.Manifest
	if isMultiProvider(empty) {
		t.Error("empty manifest should NOT be multi-provider (nil ProviderFlag)")
	}
}

// ---------------------------------------------------------------------------
// roleManifest / roleModel — test helpers to index into RoleManifests/RoleModels by role name
// ---------------------------------------------------------------------------

func roleManifest(rm RoleManifests, role string) provider.Manifest {
	switch role {
	case "planner":
		return rm.Planner
	case "stager":
		return rm.Stager
	case "message":
		return rm.Message
	case "arbiter":
		return rm.Arbiter
	default:
		panic(fmt.Sprintf("unknown role %q", role))
	}
}

func roleModel(rmodels RoleModels, role string) config.RoleConfig {
	switch role {
	case "planner":
		return rmodels.Planner
	case "stager":
		return rmodels.Stager
	case "message":
		return rmodels.Message
	case "arbiter":
		return rmodels.Arbiter
	default:
		panic(fmt.Sprintf("unknown role %q", role))
	}
}
