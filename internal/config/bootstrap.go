package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/stagehand/internal/provider"
)

// preferredBuiltins is the FR-D1 cascading provider priority order (local copy — mirrors
// internal/provider/registry.go's unexported preferredBuiltins). Used by stagerFallback + commented-block
// ordering. (Moved from internal/cmd/config.go; P1.M4.T4.S1.)
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}

// GenerateBootstrapConfig returns the populated bootstrap TOML (PRD §9.17 FR-B1/B3). provider != "" is
// used directly (caller validates); "" ⇒ cascading auto-detect (FR-D1) ⇒ "pi" fallback. NO I/O; $PATH
// detection via the registry. Shared by `config init` and the Load() first-run fallback. (P1.M4.T4.S1.)
func GenerateBootstrapConfig(prov string) string {
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
	return buildBootstrapConfig(target, installed)
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
func stagerFallback(target string, models map[string]string) (string, string) {
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

// buildBootstrapConfig is the PURE populated-config generator (PRD §9.17 FR-B1). NO detection, NO I/O —
// takes an already-resolved target + the installed list, returns the exact TOML. Deterministic ⇒ unit-
// testable. Writes: header docs, config_version (uncommented), [defaults] provider=<target> (uncommented),
// four [role.*] blocks for target (models from DefaultModelsForProvider; stager routed to the fallback
// when target can't stage, annotated), each OTHER installed provider as a commented [role.*] group, then a
// commented [generation] section.
func buildBootstrapConfig(target string, installed []string) string {
	var b strings.Builder

	// --- header (precedence/env/git/cli docs — shared with the inert template) ---
	b.WriteString(bootstrapHeader)

	// config_version (UNCOMMENTED — F6)
	fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)

	// [defaults] — provider uncommented, rest commented
	b.WriteString("\n# [defaults] — top-level Stagehand behavior (PRD §16.2)\n")
	b.WriteString("[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")

	// [role.*] for the target (UNCOMMENTED), canonical order: planner, stager, message, arbiter
	models := DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
	stagerName, stagerModel := stagerFallback(target, models)

	fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (PRD §16.4, §9.15) ---\n", target)

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
const bootstrapHeader = `# Stagehand configuration file (populated bootstrap).
#
# Generated by ` + "`stagehand config init`" + `. This file contains a WORKING config with
# a detected (or --provider-pinned) agent and per-role model defaults UNCOMMENTED.
# Edit freely; uncomment any commented section to activate it.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
#   repo-local .stagehand.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagehand.toml) and repo git config (stagehand.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGEHAND_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGEHAND_MODEL      model override ("" -> provider manifest default_model)
#   STAGEHAND_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGEHAND_CONFIG     path to a config file, overrides discovery
#   STAGEHAND_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGEHAND_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGEHAND_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGEHAND_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGEHAND_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGEHAND_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGEHAND_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
#
# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagehand.provider pi
#   git config stagehand.model ""
#   git config stagehand.timeout 120s
#   git config stagehand.auto_stage_all true
#   (read via ` + "`git config --get stagehand.<key>`" + `)
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
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
`
