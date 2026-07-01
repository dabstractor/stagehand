package config

// FR-D4 / FR-D5 verification block (PRD §9.16).
//
// Verification date: 2026-07
// Primary source:   PRD §9.16 FR-D4 table + work-item exemplars (P1.M3.T3.S1 item_description §1).
// FR-D5 mandate:    Model lineups change fast. The implementing agent MUST re-verify each provider's
//                   current flagship/mid/fast model names against that provider's live docs / --help
//                   and record verified names + date here. Defaults are authored trivially-refreshable
//                   (one cell per provider×role).
//
// Per-provider status (update on re-verification):
//   pi      — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — PRD baseline 2026-07 (bare; sub-provider set
//             separately via --provider; verify pi's OpenAI-routing sub-provider, FR-D4 note).
//   opencode— openai/gpt-5.4 / -mini / -nano — PRD baseline 2026-07 (provider-prefixed; verify upstream).
//   cursor  — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — UNVERIFIED: PRD gives tier names (flagship/mid/
//             nano); resolved to best-guess OpenAI tokens (cursor is OpenAI-backed). VERIFY `agent --help`.
//   agy     — gemini-3.5-pro / gemini-3.5-flash / gemini-3.1-flash-lite — PRD baseline 2026-07.
//   gemini  — same as agy — PRD baseline 2026-07.
//   codex   — gpt-5.1-codex-max / gpt-5.1-codex-mini / gpt-5.4-nano — PRD baseline 2026-07.
//   claude  — opus / sonnet / haiku — PRD baseline 2026-07 (bare aliases; opus=4.8, sonnet=5 per FR-D4).
//
// Stager-capability basis: a provider's stager cell is non-empty IFF its built-in manifest
// (internal/provider/builtin.go) has non-empty TooledFlags. As of 2026-07 that is ONLY pi + claude.
// gemini/agy/opencode/codex/cursor have stager="" (nil TooledFlags ⇒ RenderTooled errors ⇒ cannot be
// stager). The bootstrap (P1.M4.T2) applies the FR-D4 fallback (next TooledFlags-capable provider) on
// stager=="". VERIFY the TooledFlags state in builtin.go at implementation — if a provider has since
// gained TooledFlags, give it the mid-tier stager model.

// RoleModelDefaults is the PRD §9.16 FR-D4 per-provider × per-role default-model table, keyed
// provider → role → model. The four roles are planner/stager/message/arbiter (FR-R1). A stager value
// of "" means the provider cannot serve as the stager (its built-in manifest has nil/empty TooledFlags
// — only pi and claude are stager-capable); the bootstrap (P1.M4.T2) applies the FR-D4 fallback on
// that signal. See the FR-D5 block above for model-name provenance + the re-verification mandate.
type RoleModelDefaults map[string]map[string]string

// roleDefaults is the compiled-in FR-D4 table (unexported; access via DefaultModelsForProvider, which
// returns copies). Stager cells: non-empty IFF the provider's manifest has non-empty TooledFlags
// (pi, claude); "" otherwise (gemini, agy, opencode, codex, cursor) — the bootstrap applies the fallback.
var roleDefaults = RoleModelDefaults{
	"pi": {
		"planner": "gpt-5.4",      // flagship/smart tier (FR-D3)
		"stager":  "gpt-5.4-mini", // stager-capable (TooledFlags set in builtin.go)
		"message": "gpt-5.4-nano", // fast tier
		"arbiter": "gpt-5.4-mini", // mid tier
	},
	"claude": {
		"planner": "opus",   // flagship/smart (bare alias → current gen, opus=4.8)
		"stager":  "sonnet", // stager-capable (TooledFlags set); bare alias (sonnet=5)
		"message": "haiku",  // fast tier
		"arbiter": "sonnet", // mid tier
	},
	"gemini": {
		"planner": "gemini-3.5-pro",
		"stager":  "", // NOT stager-capable (TooledFlags nil) — bootstrap applies FR-D4 fallback
		"message": "gemini-3.1-flash-lite",
		"arbiter": "gemini-3.5-flash",
	},
	"agy": {
		"planner": "gemini-3.5-pro",
		"stager":  "", // NOT stager-capable (TooledFlags nil)
		"message": "gemini-3.1-flash-lite",
		"arbiter": "gemini-3.5-flash",
	},
	"opencode": {
		"planner": "openai/gpt-5.4", // provider-prefixed (opencode ProviderFlag empty)
		"stager":  "",               // NOT stager-capable (TooledFlags nil)
		"message": "openai/gpt-5.4-nano",
		"arbiter": "openai/gpt-5.4-mini",
	},
	"codex": {
		"planner": "gpt-5.1-codex-max",
		"stager":  "", // NOT stager-capable (TooledFlags nil)
		"message": "gpt-5.4-nano",
		"arbiter": "gpt-5.1-codex-mini",
	},
	"cursor": {
		"planner": "gpt-5.4",      // FR-D5: PRD tier-name "flagship" → best-guess OpenAI token; VERIFY agent --help
		"stager":  "",             // NOT stager-capable (TooledFlags nil)
		"message": "gpt-5.4-nano", // FR-D5: PRD tier-name "nano" → best-guess; VERIFY
		"arbiter": "gpt-5.4-mini", // FR-D5: PRD tier-name "mid" → best-guess; VERIFY
	},
}

// DefaultModelsForProvider returns a COPY of the named provider's role→model column from the FR-D4 table
// (PRD §9.16 FR-D4), or nil if name is not a built-in provider. The copy is defensive — callers (the
// config bootstrap, P1.M4.T2) may mutate it freely without affecting the package-level table (mirrors
// provider.BuiltinManifests' fresh-per-call discipline).
//
// The bootstrap writes the detected provider's [role.*] block from this column (FR-B1 step 3) and other
// installed providers' blocks commented (step 4). A stager value of "" means the provider cannot serve
// as the stager (nil TooledFlags) — the bootstrap applies the FR-D4 fallback (next TooledFlags-capable
// provider) on that signal. See roleDefaults' FR-D5 block for model-name provenance.
func DefaultModelsForProvider(name string) map[string]string {
	if col, ok := roleDefaults[name]; ok {
		out := make(map[string]string, len(col))
		for role, model := range col {
			out[role] = model
		}
		return out
	}
	return nil
}
