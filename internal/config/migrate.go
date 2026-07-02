package config

import (
	"fmt"
	"strings"
)

// v2MultiBackendBuiltins names the v2 built-in providers whose manifests carried a default_provider (a
// non-empty provider_flag) — the only providers a v2 default_provider could meaningfully apply to. In the
// v3 tree ONLY builtinPi() has ProviderFlag != "" (internal/provider/builtin.go); opencode/agy route their
// inference backend via the model slash-prefix WITHOUT a provider_flag and never carried a default_provider
// in v2. A user-defined provider is multi-backend iff its raw cfg.Providers entry sets a non-empty
// "provider_flag" (isMultiBackend checks both). LOCAL to config so the v3 migration can classify providers
// WITHOUT importing internal/provider (the raw-map decoupling invariant). Migration shim — add a name here
// if a future built-in gains a provider_flag.
var v2MultiBackendBuiltins = map[string]bool{"pi": true}

// migrateV2ToV3 performs the PRD §9.17 FR-B7 IN-MEMORY migration on a resolved *Config whose ConfigVersion
// predates 3. It folds the removed default_provider field into a slash-PREFIX on model for multi-backend
// providers, in three places: the global Config.Model, each per-role Config.Roles[r].Model, and the raw
// Config.Providers[name]["default_model"] entry; then deletes the default_provider key. IDEMPOTENT and
// INVENTS NOTHING (FR-B7): folds only when default_provider X is non-empty AND the target model is
// non-empty and bare (no "/"). A bare model with no resolvable prefix STAYS bare (becomes an FR-R5b error
// the user resolves). Single-backend providers are UNTOUCHED (a meaningless default_provider just drops).
//
// Load calls this BEFORE the caller's provider.DecodeUserOverrides (registry.go): DecodeUserOverrides
// re-encodes Config.Providers to TOML and unmarshals into the v3 Manifest — which no longer has a
// default_provider field (removed in P1.M1.T1.S1) — so go-toml would SILENTLY DROP the value. The fold must
// happen first, while default_provider is still in the raw map.
//
// AGENT TERMINOLOGY (FR-B7 "first map agent/[agent.*] → provider/[provider.*]"): handled
// UPSTREAM in loadTOML, which calls remapAgentTerminology on the raw TOML text BEFORE the typed
// decode — so agent-keyed data reaches the typed Config already remapped to provider. This
// function therefore needs no agent logic; it only folds default_provider (below). The on-disk
// `config upgrade` command (P1.M3.T1.S2) performs the same remap when persisting to the file.
func migrateV2ToV3(cfg *Config) {
	// (0) agent→provider: handled UPSTREAM by loadTOML's remapAgentTerminology (before decode), so
	// cfg already uses provider terminology here. No agent-specific work in this function.

	// (1) Collect the default_provider prefix per multi-backend provider; drop the dead key; fold the
	// provider's own raw default_model.
	prefix := map[string]string{} // provider name -> former default_provider value X
	for name, raw := range cfg.Providers {
		dp, ok := raw["default_provider"]
		if !ok {
			continue
		}
		delete(raw, "default_provider") // the field is gone in v3 — drop the dead key regardless
		x, _ := dp.(string)             // go-toml decodes a TOML string to Go string
		if x == "" || !isMultiBackend(name, raw) {
			continue // empty value, or single-backend/unknown ⇒ no fold (FR-B7)
		}
		prefix[name] = x
		if dm, ok := raw["default_model"]; ok { // raw model field is "default_model" (manifest.go:52)
			if s, ok := dm.(string); ok && s != "" && !strings.Contains(s, "/") {
				raw["default_model"] = x + "/" + s
			}
		}
	}
	if len(prefix) == 0 {
		return // nothing to fold into global/per-role models
	}

	// (2) Global model — folds only if cfg.Provider is a multi-backend provider with a prefix.
	if cfg.Model != "" && !strings.Contains(cfg.Model, "/") {
		if x, ok := prefix[cfg.Provider]; ok {
			cfg.Model = x + "/" + cfg.Model
		}
	}

	// (3) Per-role models — effective provider = role override if set, else the global.
	for r, rc := range cfg.Roles {
		if rc.Model == "" || strings.Contains(rc.Model, "/") {
			continue
		}
		ep := rc.Provider
		if ep == "" {
			ep = cfg.Provider
		}
		if x, ok := prefix[ep]; ok {
			rc.Model = x + "/" + rc.Model
			cfg.Roles[r] = rc // map values are copies — write back
		}
	}
}

// isMultiBackend reports whether provider `name` is a multi-backend (provider_flag) provider per v3
// semantics, WITHOUT importing internal/provider. True if name is a known v2 built-in multi-backend
// (v2MultiBackendBuiltins) OR the raw provider map explicitly sets a non-empty "provider_flag". Used only
// by the v3 migration.
func isMultiBackend(name string, raw map[string]any) bool {
	if v2MultiBackendBuiltins[name] {
		return true
	}
	if pf, ok := raw["provider_flag"]; ok {
		if s, ok := pf.(string); ok && s != "" {
			return true
		}
	}
	return false
}

// migrationNotice returns the ONE-TIME PRD §9.17 FR-B7 deprecation notice emitted when a <v3 config is
// auto-migrated in memory. originalVersion is the file's declared version (0 ⇒ none declared). PURE (no
// I/O); Load writes it to noticeOut. Points the user at `config upgrade` to persist the migration.
func migrationNotice(originalVersion int) string {
	if originalVersion == 0 {
		return "stagehand: config file has no config_version — treated as legacy and auto-migrated in " +
			"memory (the `default_provider` field was folded into the `model` slash-prefix, FR-B7). " +
			"Run 'stagehand config upgrade' to persist this to the file.\n"
	}
	return fmt.Sprintf("stagehand: config schema version %d (current %d) — auto-migrated in memory "+
		"(the `default_provider` field was folded into the `model` slash-prefix, FR-B7). "+
		"Run 'stagehand config upgrade' to persist this to the file.\n",
		originalVersion, CurrentConfigVersion)
}
