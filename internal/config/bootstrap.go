package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/stagecoach/internal/provider"
)

// preferredBuiltins is the FR-D1 cascading provider priority order (local copy — mirrors
// internal/provider/registry.go's unexported preferredBuiltins). Used by stagerFallback + commented-block
// ordering. (Moved from internal/cmd/config.go; P1.M4.T4.S1.)
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "qwen-code", "codex", "claude"}

// GenerateBootstrapConfig returns the populated bootstrap TOML (PRD §9.17 FR-B1/B3). provider != "" is
// used directly (caller validates); "" ⇒ cascading auto-detect (FR-D1) ⇒ "pi" fallback. NO I/O; $PATH
// detection via the registry. Shared by `config init` and the Load() first-run fallback. (P1.M4.T4.S1.)
// Delegates to GenerateBootstrapConfigWithOverrides(prov, nil) — byte-identical to the pre-refactor
// output (nil overrides = no edits → golden test).
func GenerateBootstrapConfig(prov string) string {
	return GenerateBootstrapConfigWithOverrides(prov, nil)
}

// GenerateBootstrapConfigWithOverrides returns the populated bootstrap TOML with optional per-role
// model overrides applied (role→model: "planner"|"stager"|"message"|"arbiter" → model string).
// overrides is applied AFTER the pi-blank + stagerFallback computation (so structural routing is
// preserved; only MODEL values change). nil/empty overrides ⇒ byte-identical to GenerateBootstrapConfig.
// The interactive wizard (FR-L3, PRD §9.23/§15.3) calls this seam.
func GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string {
	reg := provider.NewRegistry(nil) // built-ins only
	installed := bootstrapProviderNames(reg)
	target := prov
	if target == "" {
		if det := reg.DefaultProvider(installed); det != "" {
			target = det
		} else {
			target = "pi" // nothing on $PATH — valid default; annotated by buildBootstrapConfig
		}
	}
	return buildBootstrapConfig(target, installed, overrides)
}

// bootstrapWriteConfig writes the populated bootstrap config to path (MkdirAll + WriteFile), used by the
// Load() first-run fallback (FR-B3). Returns a wrapped error on failure. (P1.M4.T4.S1.)
func bootstrapWriteConfig(path string) error {
	content := GenerateBootstrapConfig("")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// bootstrapProviderNames returns built-in provider names whose command is on $PATH (moved from cmd's
// configInitInstalledNames). reg.List() is sorted ascending.
func bootstrapProviderNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// stagerFallback returns the (provider, model) for the [role.stager] block: target's own if
// stager-capable (models["stager"] != ""), else the first stager-capable provider in preferredBuiltins
// order. Always resolves to "pi" today (pi and claude are the only stager-capable providers; pi is first).
// StagerFallback returns the (provider, model) for the [role.stager] block: target's own if
// stager-capable (models["stager"] != ""), else the first stager-capable provider in preferredBuiltins
// order. Exported for use by the interactive wizard (P1.M6.T2.S1).
func StagerFallback(target string, models map[string]string) (string, string) {
	if m := models["stager"]; m != "" {
		return target, m
	}
	for _, name := range preferredBuiltins {
		if col := DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
			return name, col["stager"]
		}
	}
	return target, models["stager"] // unreachable (pi is always stager-capable) — defensive
}

// isInstalledName reports whether name is in the installed list.
func isInstalledName(name string, installed []string) bool {
	for _, n := range installed {
		if n == name {
			return true
		}
	}
	return false
}

// writeRoleBlock writes an UNCOMMENTED [role.<r>] block. provider is omitted when "" (role inherits
// [defaults]); annotation is printed as a comment before the key=value lines when non-empty.
func writeRoleBlock(b *strings.Builder, role, prov, model, annotation string) {
	fmt.Fprintf(b, "\n[role.%s]\n", role)
	if annotation != "" {
		fmt.Fprintf(b, "# %s\n", annotation)
	}
	if prov != "" {
		fmt.Fprintf(b, "provider = %q\n", prov)
	}
	fmt.Fprintf(b, "model = %q\n", model)
}

// writeCommentedRoleBlock writes a fully-commented [role.<r>] block for an alternate provider.
func writeCommentedRoleBlock(b *strings.Builder, role, prov, model string) {
	fmt.Fprintf(b, "# [role.%s]\n", role)
	fmt.Fprintf(b, "# provider = %q\n", prov)
	fmt.Fprintf(b, "# model = %q\n", model)
}

// applyOverrides applies per-role model overrides onto the computed models map and stager model.
// overrides control only MODEL values — stagerName (structural routing) is untouched.
// A nil overrides map is a no-op (byte-identity contract: GenerateBootstrapConfig delegates with nil).
func applyOverrides(models map[string]string, stagerModel *string, overrides map[string]string) {
	if overrides == nil {
		return
	}
	for _, role := range []string{"planner", "message", "arbiter"} {
		if v, ok := overrides[role]; ok {
			models[role] = v
		}
	}
	if v, ok := overrides["stager"]; ok {
		*stagerModel = v
	}
}

// buildBootstrapConfig is the PURE populated-config generator (PRD §9.17 FR-B1). NO detection, NO I/O —
// takes an already-resolved target + the installed list + optional per-role model overrides, returns the
// exact TOML. Deterministic ⇒ unit-testable. Writes: header docs, config_version (uncommented),
// [defaults] provider=<target> (uncommented), four [role.*] blocks for target (models from
// DefaultModelsForProvider, overridden by overrides; stager routed to the fallback when target can't
// stage, annotated), each OTHER installed provider as a commented [role.*] group, then a commented
// [generation] section. overrides is applied AFTER pi-blank + stagerFallback (structural routing
// preserved; only MODEL values change). nil/empty overrides ⇒ no edits.
func buildBootstrapConfig(target string, installed []string, overrides map[string]string) string {
	var b strings.Builder

	// --- header (precedence/env/git/cli docs — shared with the inert template) ---
	b.WriteString(bootstrapHeader)

	// config_version (UNCOMMENTED — F6)
	fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)

	// [defaults] — provider uncommented, rest commented
	b.WriteString("\n# [defaults] — top-level Stagecoach behavior (PRD §16.2)\n")
	b.WriteString("[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below\n")
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")

	// [role.*] for the target (UNCOMMENTED), canonical order: planner, stager, message, arbiter
	models := DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
	piBlanked := target == "pi"
	if piBlanked {
		// pi is a multi-backend provider: the model must carry the inference backend as a
		// slash-prefix (FR-R5b). The bootstrap writes per-role models blank so the user supplies
		// their own backend/model.
		for role := range models {
			models[role] = ""
		}
	}
	stagerName, stagerModel := StagerFallback(target, models)
	if piBlanked {
		// stagerFallback re-pulls pi’s stager model from the FR-D4 table (a fresh copy); force
		// it blank so all four roles stay empty. pi remains the stager (stager-capable).
		stagerModel = ""
	}

	// Apply overrides AFTER pi-blank + stagerFallback (structural routing preserved; only MODEL values).
	piHasOverrides := piBlanked && len(overrides) > 0
	applyOverrides(models, &stagerModel, overrides)

	fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (PRD §16.4, §9.15) ---\n", target)
	if piBlanked && !piHasOverrides {
		b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
		b.WriteString("# e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
		b.WriteString("# The shipped per-role models are empty so you can supply your own backend/model.\n")
	} else if piBlanked && piHasOverrides {
		b.WriteString("# NOTE: pi is a multi-backend provider — each model carries the inference backend as a\n")
		b.WriteString("# slash-prefix (e.g. model = \"zai/glm-5.2\"). A bare model (no '/') on pi is a config\n")
		b.WriteString("# error (FR-R5b).\n")
	}

	// planner — inherits [defaults] provider
	writeRoleBlock(&b, "planner", "", models["planner"], "")

	// stager — may fall back to a different provider
	var stagerAnnotation string
	if stagerName != target {
		stagerAnnotation = target + " cannot serve as the stager (no tooled_flags); routed to " + stagerName + " (the first stager-capable provider)."
	}
	writeRoleBlock(&b, "stager", stagerName, stagerModel, stagerAnnotation)

	// message — inherits [defaults] provider
	writeRoleBlock(&b, "message", "", models["message"], "")

	// arbiter — inherits [defaults] provider
	writeRoleBlock(&b, "arbiter", "", models["arbiter"], "")

	// other installed providers as COMMENTED [role.*] groups
	for _, name := range preferredBuiltins {
		if name == target || !isInstalledName(name, installed) {
			continue
		}
		other := DefaultModelsForProvider(name)
		if other == nil {
			continue
		}
		b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
		writeCommentedRoleBlock(&b, "planner", name, other["planner"])
		writeCommentedRoleBlock(&b, "stager", name, other["stager"])
		writeCommentedRoleBlock(&b, "message", name, other["message"])
		writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
	}

	// commented [generation] defaults
	b.WriteString(generationCommented)

	return b.String()
}

// bootstrapHeader is the shared config-file header (precedence, env vars, git keys, CLI flags).
// Used by buildBootstrapConfig for the populated output. exampleConfigTemplate has its own copy.
const bootstrapHeader = `# Stagecoach configuration file (populated bootstrap).
#
# Generated by ` + "`stagecoach config init`" + `. This file contains a WORKING config with
# a detected (or --provider-pinned) agent and per-role model defaults UNCOMMENTED.
# Edit freely; uncomment any commented section to activate it.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGECOACH_* env vars  >  repo git config (stagecoach.*)  >
#   repo-local .stagecoach.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagecoach.toml) and repo git config (stagecoach.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGECOACH_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGECOACH_MODEL      model override ("" -> provider manifest default_model)
#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGECOACH_CONFIG     path to a config file, overrides discovery
#   STAGECOACH_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGECOACH_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGECOACH_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGECOACH_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGECOACH_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)
#   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
#   STAGECOACH_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
#
# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagecoach.provider pi
#   git config stagecoach.model ""
#   git config stagecoach.timeout 120s
#   git config stagecoach.auto_stage_all true
#   (read via ` + "`git config --get stagecoach.<key>`" + `)
#
# ---------------------------------------------------------------------------
# CLI flags (PRD §15.2) — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles (§16.4)
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single (§9.14)
# --single / --no-decompose                  bypass decomposition; force the single-commit path (§9.14)
# --max-commits <N>                          safety cap on auto-decompose (default 12; §9.14 FR-M4)

`

// generationCommented is the commented [generation] defaults section appended to the populated config.
const generationCommented = `
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# The [generation] header below is intentionally UNCOMMENTED (while every key stays
# commented). Reason: in TOML a key attaches to the most-recent active table header, so
# if this header were commented, a user uncommenting a single key (e.g. token_limit)
# would silently drop it under the WRONG table ([role.arbiter]) and it would be ignored.
# Keeping the header active means uncommenting any one key lands it in the right place.
# An all-commented-keys section is an inert empty table — built-in defaults still apply.
[generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section; ignored when token_limit is set (FR3d)
# max_md_lines          = 100     # per-file line cap for markdown diffs; ignored when token_limit is set (FR3d)
# token_limit           = 0       # holistic token budget for the WHOLE payload (prompt+examples+diff); 0 = unset ⇒ use the legacy caps above. Set to your model's context window, e.g. 120000, so the payload always fits without a per-model registry (FR3d)
# diff_context          = 1       # unchanged context lines around each hunk: 0 = changed lines only (max savings), 1 = one anchor line (default), 3 = git's default (FR3f); valid 0–3 (out-of-range rejected at config load)
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
# multi_turn_fallback     = true   # lossless multi-turn fallback on one-shot exhaustion (§9.24 FR-T1c); CANNOT disable via file — set session_mode="" on the provider instead (see docs/configuration.md)
# multi_turn_chunk_tokens = 32000  # per-turn chunk budget in tokens for multi-turn (§9.24 FR-T3); does NOT interact with token_limit (FR-T12)
`
