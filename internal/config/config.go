package config

import (
	"time"
)

func boolPtr(b bool) *bool { return &b }

func strPtr(s string) *string { return &s }

// CurrentConfigVersion is the config-schema version this binary understands (PRD §9.17 FR-B4).
// Bumped on any breaking config change. On load, stagehand compares a config file's
// config_version to this constant: older files are auto-migrated in memory (FR-B7) with
// a one-time deprecation notice pointing at `config upgrade`; ahead files emit an advisory.
// config_version is metadata, NOT a precedence layer (PRD §16.1). v3 = inference provider
// folded into model slash-prefix (FR-B7, FR-R5b); v2 = per-role models + multi-commit
// decomposition + binary filtering.
const CurrentConfigVersion = 3

// RoleConfig holds a per-role provider/model/reasoning override (PRD §16.4, §9.15 FR-R1–R6).
// A role is one of "planner", "stager", "message", "arbiter" (§13.6.2). Any field "" ⇒
// the role inherits the global [defaults] (FR-R2); a non-empty value overrides just that
// field (FR-R3 field-merge across layers). Model strings are provider-specific (FR-R5):
// a role's Model is interpreted by that role's resolved Provider's manifest, so changing
// a role's Provider without updating its Model is a configuration error stagehand surfaces.
// For multi-provider agents (pi/opencode/agy) Provider is required when Model is set (FR-R5b).
// Reasoning controls thinking effort (off|low|medium|high; FR-R6); "" ⇒ inherit global ⇒ shipped
// default (planner=high, others=off).
//
// Config.Roles (below) carries the RESOLVED per-role table; it is toml:"-" because the
// [role.<role>] FILE tables decode into fileConfig's fileRoleConfig map (S2) and
// materialize/overlay into this typed map — the same raw-map→typed-field pattern
// Config.Providers uses.
type RoleConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"` // off|low|medium|high (FR-R6); "" ⇒ inherit global [defaults].reasoning ⇒ shipped default
}

// Config is the fully-resolved Stagehand configuration: the single value produced by the 7-layer
// precedence resolver (PRD §16.1, FR34) and read by every consumer — the TOML/git/env/CLI loaders
// (P1.M1.T4.S2-S4), the provider registry (P1.M2.T3), and the generation pipeline.
//
// DESIGN CALL: flat + plain-typed + RESOLVED. Every field holds a concrete value (Timeout is already
// a time.Duration). This struct is NOT unmarshaled directly from the §16.2 file: that file uses
// [defaults]/[generation]/[role.<role>]/[provider.<name>] subtables and string durations ("120s").
// The loaders (file.go's fileConfig intermediate structs + materialize/overlay; load.go's
// env/flag layers) decode into their own intermediate structs and merge field-by-field INTO
// this plain Config. Keeping Config plain means consumers read cfg.Timeout /
// cfg.Roles["planner"].Model with zero dereferencing (except the deliberately-pointer
// Output/StripCodeFence, whose nil ⇒ "defer to the manifest"). The toml tags use §16.2
// snake_case leaf names for the file-backed fields; toml:"-" marks loader-populated maps
// (Providers, Roles) and CLI/runtime-only fields (NoColor, Commits, Single) that NEVER
// appear in a file.
//
// V2 FIELDS (this subtask, P1.M3.T1.S1):
//   - Roles / ConfigVersion / MaxCommits / BinaryExtensions / Commits / Single (see inline comments).
//   - RoleConfig (above) + CurrentConfigVersion (above) are the supporting type/const.
//   - File→Config plumbing for Roles/ConfigVersion/MaxCommits/BinaryExtensions lands in S2 (file.go);
//     env/flag wiring for Commits/Single/Roles + ResolveRoleModel land in P1.M3.T2 (load.go).
type Config struct {
	// [defaults] (PRD §16.2)
	Provider     string        `toml:"provider"`       // "" => auto-detect (PRD §15.2)
	Model        string        `toml:"model"`          // "" => provider manifest default_model
	Reasoning    string        `toml:"reasoning"`      // off|low|medium|high (FR-R6); "" ⇒ ResolveRoleModel's shipped fallback (planner=high)
	Timeout      time.Duration `toml:"timeout"`        // generation timeout; Defaults: 120s
	AutoStageAll bool          `toml:"auto_stage_all"` // git add -A when nothing staged (PRD §9.4)
	Verbose      bool          `toml:"verbose"`        // print resolved cmd, raw output, retries

	// CLI / UI only — NOT in the §16.2 config file (toml:"-"). Set by flags/env at runtime, never by a file.
	NoColor bool `toml:"-"` // --no-color / STAGEHAND_NO_COLOR / NO_COLOR; TTY-aware at runtime (UI layer)
	// V2 decompose mode flags (PRD §9.14 FR-M2) — set by --commits/--single (P1.M3.T2/P4.M1.T1), not files.
	Commits int  `toml:"-"` // --commits N (N≥2 forces exactly N commits); 0 = auto-decompose (planner decides); --commits 1 ⇒ Single
	Single  bool `toml:"-"` // --single/--no-decompose: bypass the planner entirely (v1 single-commit path)

	// [generation] (PRD §16.2)
	MaxDiffBytes        int     `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int     `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	MaxDuplicateRetries int     `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int     `toml:"subject_target_chars"`  // target subject length for truncation
	Output              *string `toml:"output"`                // nil ⇒ honor manifest (S2 bridge); non-nil ⇒ override
	StripCodeFence      *bool   `toml:"strip_code_fence"`      // strip ``` fences from agent output; nil ⇒ true
	// V2 generation tuning (PRD §16.2, §9.1 FR3a, §9.14 FR-M4) — decoded from [generation] in S2.
	MaxCommits       int      `toml:"max_commits"`       // safety cap on auto-decompose (default 12; FR-M4)
	BinaryExtensions []string `toml:"binary_extensions"` // extra non-text exts to filter (FR3a); nil ⇒ built-in denylist only

	// [provider.<name>] user-defined / override provider definitions (PRD §16.2, §12.8).
	// Carried as a RAW map: the provider MANIFEST type lives in internal/provider, so config must not import
	// it (import-cycle risk). The registry (P1.M2.T3) consumes this map — for each name it re-encodes the
	// entry to TOML and unmarshals into a Manifest, then field-merges with the built-in manifest per §16.1.
	// toml:"-" => excluded from flat marshal/unmarshal (Config is never decoded from §16.2; fileConfig is).
	// Populated by the file loaders (P1.M1.T4.S2); nil means "no user-defined providers".
	Providers map[string]map[string]any `toml:"-"`

	// V2 per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5). Keyed by role name
	// ("planner", "stager", "message", "arbiter"). toml:"-" — populated by the file loaders (S2)
	// from the [role.<role>] tables (field-merged across layers exactly like Providers); nil means
	// "no per-role overrides → every role inherits the global [defaults]" (FR-R2). On the single-commit
	// path the only active role is "message", so a nil Roles is exactly equivalent to v1 (back-compatible).
	Roles map[string]RoleConfig `toml:"-"`

	// V2 schema version (PRD §9.17 FR-B4). Metadata, NOT a precedence layer (§16.1): on load it is
	// compared to CurrentConfigVersion for an advisory warning; it does not participate in value
	// resolution. Decoded from the top-level config_version key in S2; Defaults() leaves it 0 (unset;
	// the load-time advisory (P1.M4.T1.S1) compares the resolved value to CurrentConfigVersion. 0 ⇒
	// no source declared a schema version).
	ConfigVersion int `toml:"config_version"`
}

// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 120s, auto_stage_all
// true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, subject_target_chars 50,
// max_commits 12. Output and StripCodeFence are nil (deferred to the manifest's Resolve() — §12.1).
// Provider and Model are "" (Layer 1 does not pin them): empty Provider => auto-detect (PRD §15.2);
// empty Model => the manifest default_model (§16.2). Verbose/NoColor/Single are false; Commits is 0
// (auto-decompose). Roles and BinaryExtensions are nil (no per-role overrides → all roles use the
// global; binary filtering uses the built-in denylist only). ConfigVersion is 0 (unset; a
// Defaults() Config has no schema version until a file declares one). NoColor is ultimately
// TTY-aware in the UI layer.
//
// Returned BY VALUE: Config is an immutable resolved snapshot after Load(); a value return avoids
// nil-pointer hazards and lets callers copy freely.
func Defaults() Config {
	return Config{
		Provider:            "",
		Model:               "",
		Reasoning:           "", // FR-R6: "" ⇒ fall through to the per-role shipped default (planner=high) in ResolveRoleModel
		Timeout:             120 * time.Second,
		AutoStageAll:        true,
		Verbose:             false,
		NoColor:             false,
		Commits:             0, // auto-decompose (PRD §9.14 FR-M2); set by --commits in P1.M3.T2/P4.M1.T1
		Single:              false,
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
		Output:              nil,
		StripCodeFence:      nil,
		MaxCommits:          12,  // §9.14 FR-M4 default safety cap on auto-decompose
		BinaryExtensions:    nil, // nil ⇒ built-in denylist only (§9.1 FR3a)
		Providers:           nil,
		Roles:               nil, // no per-role overrides → all roles use the global (§16.4 FR-R2)
		ConfigVersion:       0,   // UNSET sentinel — the load-time advisory (P1.M4.T1.S1) compares the resolved
		//                              value to CurrentConfigVersion; 0 ⇒ no source declared a schema version.
	}
}
