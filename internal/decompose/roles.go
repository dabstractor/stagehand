// Package decompose implements the multi-commit decomposition pipeline (PRD §13.6.2): given an
// un-staged working tree, it produces N logically-coherent commits by running a four-agent pipeline
// (planner → stager → message → arbiter) with per-role provider/model resolution.
//
// This file (roles.go) is the role-resolution foundation: ResolveRoles turns a fully-resolved
// config.Config + *provider.Registry into four validated, install-checked provider.Manifests
// (RoleManifests) and four (provider, model) pairs (RoleModels). It is the four-role generalization
// of pkg/stagecoach.buildDeps (which resolves one role: message). The decompose orchestrator
// (P3.M4.T1.S1) calls ResolveRoles to build the injectable Deps struct.
//
// Subsequent tasks add planner.go, stager.go, message.go, arbiter.go, chain.go, and decompose.go
// (P3.M2.T2/T3/T4 + P3.M3.* + P3.M4.*).
package decompose

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/prompt"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// RoleManifests holds the four resolved (merged-but-unresolved — Render Validates+Resolves) provider
// manifests for the decomposition pipeline (PRD §13.6.2). Built by ResolveRoles; consumed by the
// orchestrator (P3.M4.T1.S1) via RoleManifests.X.Render(...). The stager field carries the TOOLED
// manifest post-FR-D4 fallback (non-empty TooledFlags guaranteed); the other three are bare manifests.
type RoleManifests struct {
	Planner provider.Manifest // bare
	Stager  provider.Manifest // tooled (TooledFlags non-empty after fallback)
	Message provider.Manifest // bare
	Arbiter provider.Manifest // bare
}

// RoleModels holds the four resolved (provider, model, reasoning) triples (one config.RoleConfig
// per role) produced by ResolveRoles. Post-auto-detect and post-stager-fallback. The orchestrator passes
// RoleModels.X.Model + RoleModels.X.Provider as the Render model/provider params, and RoleModels.X.Reasoning
// as the Render reasoning param (FR-R6). config.RoleConfig is {Provider, Model, Reasoning}.
type RoleModels struct {
	Planner config.RoleConfig
	Stager  config.RoleConfig
	Message config.RoleConfig
	Arbiter config.RoleConfig
}

// Deps carries the runtime collaborators the decompose orchestrator (P3.M4.T1.S1) threads through
// the pipeline. Injectable for testing with stub manifests: a test sets Deps.Roles directly and skips
// ResolveRoles (mirrors generate.Deps{Manifest: stub}). The orchestrator ALSO retains RoleModels
// (ResolveRoles's 2nd return value) locally for Render params (Deps carries RoleManifests only, per the
// contract — P3.M4.T1.S1 may extend Deps with a Models field if preferred).
type Deps struct {
	Git      git.Git
	Registry *provider.Registry
	Config   config.Config
	Roles    RoleManifests
	Verbose  *ui.Verbose

	Excludes []string // resolved user exclude pathspecs (from exclude.ResolveExcludePathspecs); nil ⇒ none

	// Out is where the loop prints the §18.3 multi-commit rescue + the §13.5 CAS message (stderr in
	// prod via cmd.ErrOrStderr; *bytes.Buffer in tests). nil → rescue/CAS messages are skipped
	// (library-safe; the loop guards nil). S2 (P3.M4.T1.S2).
	Out io.Writer

	// stager is an OPTIONAL test seam. When non-nil, the orchestrator (decompose.go) calls it instead
	// of the package-level stageConcept (the real tooled-agent invocation). nil in production (the CLI
	// builds Deps without it). Lets orchestrator tests inject a stager that actually stages files via
	// git (the stubtest agent cannot run git), exercising the full happy-path loop end-to-end. The
	// signature matches stageConcept exactly. See decompose.invokeStager.
	stager func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error
}

// ResolveRoles resolves the four agent roles (planner/stager/message/arbiter, PRD §13.6.2) from
// the fully-resolved config + registry into validated, install-checked manifests and (provider, model)
// pairs. For each role it applies the pipeline:
//
//  1. config.ResolveRoleModel — per-field merge (role → global → manifest-default sentinel)
//  2. auto-detect provider via reg.DefaultProvider if provider==""
//  3. reg.Get → Validate → IsInstalled
//  4. FR-R5b guard: bare model + no provider + multi-provider manifest → config error
//  5. FR-D4 stager fallback: TooledFlags-less stager → first installed TooledFlags-capable provider
//
// It stores the merged-but-unresolved manifest (Render calls Validate+Resolve itself — same as
// buildDeps). On the stager fallback, BOTH the provider AND the model are switched (models are
// provider-specific per FR-R5; the fallback model comes from config.DefaultModelsForProvider).
//
// Returns zero-value structs on any error (all errors are wrapped with the role name).
func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error) {
	installed := computeInstalled(reg) // computed ONCE — shared by all 4 roles + FirstTooledProvider

	var rm RoleManifests
	var rmodels RoleModels

	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		prov, mdl, rsn := config.ResolveRoleModel(role, cfg)

		if prov == "" {
			prov = reg.DefaultProvider(installed) // auto-detect (mirrors buildDeps)
		}
		if prov == "" {
			return RoleManifests{}, RoleModels{}, fmt.Errorf(
				"role %q: no provider configured and none of the preferred built-ins are installed", role)
		}

		m, ok := reg.Get(prov)
		if !ok {
			return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: unknown provider %q", role, prov)
		}
		if err := m.Validate(); err != nil {
			return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: provider %q: %w", role, prov, err)
		}
		if !reg.IsInstalled(m) {
			return RoleManifests{}, RoleModels{}, fmt.Errorf(
				"role %q: provider %q: command %q not found. Is the agent installed?",
				role, prov, m.DetectCommand())
		}

		// (The FR-R5b check lives AFTER the FR-D4 stager fallback below, so it validates the FINAL
		// resolved (agent, model) pair — including a fallback onto a multi-provider agent.)

		// FR-D4 stager fallback: a TooledFlags-less stager cannot stage → fall back to a capable one.
		if role == "stager" && len(m.TooledFlags) == 0 {
			fb := reg.FirstTooledProvider(installed)
			if fb == "" {
				return RoleManifests{}, RoleModels{}, fmt.Errorf(
					"role %q: provider %q cannot stage (tooled_flags empty) and no other installed "+
						"provider is stager-capable", role, prov)
			}
			fbm, ok := reg.Get(fb)
			if !ok {
				return RoleManifests{}, RoleModels{}, fmt.Errorf(
					"role %q: stager fallback provider %q not found", role, fb)
			}
			prov = fb
			m = fbm
			// Models are provider-specific (FR-R5). The old provider's model (user-configured
			// or default-table) may be bare and invalid on the new multi-provider fallback target.
			// Clear a bare model when falling back to a multi-provider agent, then apply the
			// default-table model for the fallback provider only if it is FR-R5b-compatible.
			if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") {
				mdl = "" // bare model from old provider — invalid on multi-provider fallback
			}
			if col := config.DefaultModelsForProvider(fb); col != nil {
				candidate := col["stager"]
				if candidate != "" && !(isMultiProvider(m) && !strings.Contains(candidate, "/")) {
					mdl = candidate
				}
			}
		}

		// FR-R5b (role-named early check; Render re-enforces at the chokepoint for ALL paths). A model PINNED on a
		// multi-provider (provider_flag set, e.g. pi) MUST carry its inference backend as a slash-prefix
		// ("zai/glm-5.2"); a bare model is an unroutable config error, never a silent bare --model. Mirrors
		// Render's guard (render.go) but fires earlier with the role name. mdl=="" is NOT pinned ⇒ no guard
		// (Render uses the manifest DefaultModel + its own guard). Single-backend providers (ProviderFlag=="")
		// are never guarded.
		if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") {
			return RoleManifests{}, RoleModels{}, fmt.Errorf(
				"role %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", role, mdl, m.Name)
		}

		setRole(&rm, &rmodels, role, m, prov, mdl, rsn)
	}

	return rm, rmodels, nil
}

// computeInstalled returns the names of providers whose discovery command is on $PATH.
// Mirrors pkg/stagecoach.buildDeps' installed computation. Computed ONCE per ResolveRoles call
// (shared by all 4 roles + FirstTooledProvider).
func computeInstalled(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// isMultiProvider reports whether the manifest represents a multi-provider agent (FR-R5b).
// Only pi today (ProviderFlag="--provider"); opencode/agy have ProviderFlag="" and are excluded
// because they have no separate --provider flag to omit (opencode encodes the provider in the model
// string; agy is single-backend). The check is nil-safe for hypothetical user overrides.
func isMultiProvider(m provider.Manifest) bool {
	return m.ProviderFlag != nil && *m.ProviderFlag != ""
}

// setRole assigns the resolved manifest and RoleConfig (provider, model, reasoning) to the correct
// struct field for the given role. A 4-case switch (no reflection — vet-friendly, clear, fast).
func setRole(rm *RoleManifests, rmodels *RoleModels, role string, m provider.Manifest, prov, mdl, rsn string) {
	rc := config.RoleConfig{Provider: prov, Model: mdl, Reasoning: rsn}
	switch role {
	case "planner":
		rm.Planner, rmodels.Planner = m, rc
	case "stager":
		rm.Stager, rmodels.Stager = m, rc
	case "message":
		rm.Message, rmodels.Message = m, rc
	case "arbiter":
		rm.Arbiter, rmodels.Arbiter = m, rc
	}
}
