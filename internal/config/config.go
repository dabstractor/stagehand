package config

import (
	"time"
)

func boolPtr(b bool) *bool { return &b }

// Config is the fully-resolved Stagehand configuration: the single value produced by the 7-layer
// precedence resolver (PRD §16.1, FR34) and read by every consumer — the TOML/git/env/CLI loaders
// (P1.M1.T4.S2-S4), the provider registry (P1.M2.T3), and the generation pipeline.
//
// DESIGN CALL: flat + plain-typed + RESOLVED. Every field holds a concrete value (Timeout is already
// a time.Duration). This struct is NOT unmarshaled directly from the §16.2 file: that file uses
// [defaults]/[generation] subtables and string durations ("120s"). The loaders (S2-S4) decode into
// their own intermediate structs (pointer or map-based — see arch §5.4) and merge field-by-field
// INTO this plain Config. Keeping Config plain means consumers read cfg.Timeout / cfg.Verbose with
// zero dereferencing. The toml tags use §16.2 snake_case leaf names; section grouping is S2's concern.
type Config struct {
	// [defaults] (PRD §16.2)
	Provider     string        `toml:"provider"`       // "" => auto-detect (PRD §15.2)
	Model        string        `toml:"model"`          // "" => provider manifest default_model
	Timeout      time.Duration `toml:"timeout"`        // generation timeout; Defaults: 120s
	AutoStageAll bool          `toml:"auto_stage_all"` // git add -A when nothing staged (PRD §9.4)
	Verbose      bool          `toml:"verbose"`        // print resolved cmd, raw output, retries

	// CLI / UI only — NOT in the §16.2 config file (PRD §15.2: --no-color / STAGEHAND_NO_COLOR / NO_COLOR)
	NoColor bool `toml:"-"` // TTY-aware at runtime; set by UI layer (P1.M4.T3.S1)

	// [generation] (PRD §16.2)
	MaxDiffBytes        int    `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int    `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int    `toml:"subject_target_chars"`  // target subject length for truncation
	Output              string `toml:"output"`                // "raw" | "json"
	StripCodeFence      *bool  `toml:"strip_code_fence"`      // strip ``` fences from agent output; nil ⇒ true

	// [provider.<name>] user-defined / override provider definitions (PRD §16.2, §12.8).
	// Carried as a RAW map: the provider MANIFEST type is defined later (P1.M2.T1), so config must not
	// import it (import-cycle risk). The registry (P1.M2.T3) consumes this map — for each name it
	// re-encodes the entry to TOML and unmarshals into a Manifest, then field-merges with the built-in
	// manifest per PRD §16.1. toml:"-" => excluded from flat marshal (no clash with `Provider` string)
	// and from flat unmarshal (Config is never decoded from §16.2; fileConfig is). Populated by the
	// file loaders (P1.M1.T4.S2); nil means "no user-defined providers".
	Providers map[string]map[string]any `toml:"-"`
}

// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 120s, auto_stage_all
// true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output "raw",
// strip_code_fence true, subject_target_chars 50. Provider and Model are "" (Layer 1 does not pin
// them): empty Provider => auto-detect (PRD §15.2); empty Model => use the manifest default_model
// (PRD §16.2). Verbose/NoColor are false (NoColor is ultimately TTY-aware in the UI layer).
//
// Returned BY VALUE: Config is an immutable resolved snapshot after Load(); a value return avoids
// nil-pointer hazards and lets callers copy freely.
func Defaults() Config {
	return Config{
		Provider:            "",
		Model:               "",
		Timeout:             120 * time.Second,
		AutoStageAll:        true,
		Verbose:             false,
		NoColor:             false,
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
		Output:              "raw",
		StripCodeFence:      boolPtr(true),
	}
}
