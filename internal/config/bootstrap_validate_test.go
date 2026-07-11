package config

import (
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/provider"
	"github.com/pelletier/go-toml/v2"
)

// bootstrapValidateAllBuiltins mirrors internal preferredBuiltins (bootstrap.go) — the canonical
// ordered set of built-in provider names. Used as the `installed=[all]` case. Using the ordered
// literal (NOT provider.BuiltinManifests() map keys — map iteration order is non-deterministic)
// keeps the `[all]` case stable across runs.
var bootstrapValidateAllBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}

// TestBootstrapValidateModels is the post-bootstrap FR-R5b regression net (PRD §9.15 FR-R5b,
// §9.17 FR-B1; architecture/bootstrap_pi_model_bug.md §Post-Bootstrap ValidateModel Regression Net).
// For every (target, installed) combination it generates the bootstrap config, parses the ACTIVE
// [role.*] blocks into a Config via the production fileConfig→materialize path (commented blocks
// are inert TOML comments and are NOT parsed), resolves each role's effective (provider, model)
// via ResolveRoleModel, and calls Manifest.ValidateModel. A bare model on a provider_flag provider
// (pi) fails; a blank model is skipped (FR-D2 — the user fills it in). This would have caught
// Issue 1 (a bare pi stager model) immediately. GREEN on the post-S1 tree; RED on the pre-S1 tree.
//
// NOTE: this is `package config` (not config_test) because it needs the UNEXPORTED
// buildBootstrapConfig (for deterministic installed=nil/[all] control — the exported
// GenerateBootstrapConfig auto-detects installed via $PATH, which is non-deterministic and can't
// express the nil/[all] cases) AND the UNEXPORTED materialize (Config.Roles is toml:"-", so direct
// toml.Unmarshal into *Config leaves Roles nil and ResolveRoleModel finds no roles). The
// "config must not import provider" invariant is already relaxed — bootstrap.go imports it (no
// cycle: provider does not import config), so a `package config` test importing provider compiles.
// Following the item's literal config_test recommendation would stall at both compile walls.
func TestBootstrapValidateModels(t *testing.T) {
	manifests := provider.BuiltinManifests()
	roles := []string{"planner", "stager", "message", "arbiter"}

	type tc struct {
		target    string
		installed []string
	}
	cases := []tc{
		{"pi", []string{"pi"}},
		{"pi", []string{"pi", "claude"}},
		{"claude", []string{"claude"}},
		{"claude", []string{"claude", "pi"}},
		{"agy", []string{"agy", "pi", "claude"}},
	}
	for _, tgt := range bootstrapValidateAllBuiltins {
		cases = append(cases, tc{tgt, nil})                          // no-detection case
		cases = append(cases, tc{tgt, bootstrapValidateAllBuiltins}) // everything-detected case
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.target+"_installed_"+installedLabel(tc.installed), func(t *testing.T) {
			content := buildBootstrapConfig(tc.target, tc.installed, nil)

			// Parse the ACTIVE blocks (fileConfig) and materialize into a Config (populates Roles).
			// Commented `# [role.*]` blocks are TOML comments → not decoded → not validated (Issue 2's
			// scope is P1.M2.T1; this test validates ACTIVE blocks only). toml:"-" on Config.Roles
			// means direct Unmarshal into *Config would leave Roles nil — materialize is required.
			var fc fileConfig
			if err := toml.Unmarshal([]byte(content), &fc); err != nil {
				t.Fatalf("buildBootstrapConfig(%q, %v): invalid TOML: %v\n%s", tc.target, tc.installed, err, content)
			}
			cfg, err := materialize(&fc, 120*time.Second, 10*time.Minute)
			if err != nil {
				t.Fatalf("materialize: %v", err)
			}

			for _, role := range roles {
				prov, model, _ := ResolveRoleModel(role, *cfg)
				if model == "" {
					continue // blank is valid (FR-D2 — pi ships blank; user fills it in); skip
				}
				m, ok := manifests[prov]
				if !ok {
					t.Errorf("target=%s installed=%v role=%s: provider %q has no built-in manifest",
						tc.target, tc.installed, role, prov)
					continue
				}
				if err := m.ValidateModel(model); err != nil {
					t.Errorf("target=%s installed=%v role=%s: ValidateModel(%q on %q) = %v",
						tc.target, tc.installed, role, model, prov, err)
				}
			}
		})
	}
}

// installedLabel renders the installed slice for readable subtest names.
func installedLabel(installed []string) string {
	if len(installed) == 0 {
		return "nil"
	}
	return strings.Join(installed, ",")
}
