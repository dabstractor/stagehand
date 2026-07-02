// load.go is the resolution layer of the FR34 precedence chain (PRD §16.1;
// decisions.md §6). It owns the single Load() entry point that starts from
// Default() (P1.M5.T1.S1, the lowest-precedence floor) and layers the source
// readers from file.go (P1.M5.T2.S1: global TOML, repo TOML, repo git-config)
// under the env and CLI-flag layers, producing the resolved Config threaded
// through the whole binary plus a provider.Registry built by field-merging the
// resolved ProviderOverrides onto the six built-in manifests. It also emits the
// §19 repo-local-config trust notice (PRD §19) when a repo-local source
// redirects the provider.
//
// This file is a sibling of config.go (which OWNS the "// Package config" doc)
// and therefore carries a plain "package config" line, mirroring how the
// sibling git package's log.go defers the package doc to its git.go, and how
// defaults.go/file.go already defer to config.go. It imports ONLY fmt, time,
// and internal/provider — it deliberately does NOT import the git package
// (readGitConfig already shells out via os/exec in file.go), preserving the
// one-way import edge config → provider as the only internal import
// (plan_overview key decision 1).
package config

import (
	"fmt"
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// Flags is the env+CLI layer consumed by Load (FR34 layers 6 and 7). It is
// populated by the CLI layer (P1.M7.T2.S1): that layer reads the six
// STAGEHAND_* environment variables (FR35/§15.2) into Env and the matching
// command-line flags into Flag, each as a pointer-per-scalar where nil means
// "not set by this source" and a non-nil pointer (even to the zero value)
// means "set by this source". Load never reads os.Getenv itself — the entire
// env surface is handed to it here, which keeps Load pure and testable without
// fiddling the process environment.
//
// Env is applied BELOW Flag so a CLI flag ALWAYS beats its env var (FR34:
// flag > env). FlagsLayer carries exactly the six STAGEHAND_* settings
// (ConfigPath, Provider, Model, Timeout, Verbose, NoColor); auto_stage_all and
// the generation scalars have NO env var or flag — AutoStageAll is governed by
// the --all/--no-auto-stage ACTION flags in the CLI (M7.T2), not by a config
// setter, and the generation caps are file/git-config only.
type Flags struct {
	Env  FlagsLayer
	Flag FlagsLayer
}

// FlagsLayer is the pointer-per-scalar shape of one of the two highest FR34
// layers (env or CLI flag). Every field is a pointer: nil = the source did not
// set it; non-nil (even to the zero value — Model="", Verbose=false) = set by
// this source. The present-but-zero distinction is what lets a higher layer
// override a lower layer's non-empty value with an explicit zero.
type FlagsLayer struct {
	// ConfigPath is STAGEHAND_CONFIG / --config: an explicit config-file path
	// that OVERRIDES discovery (§15.2). When non-nil on the Flag (or Env)
	// layer, Load parses THAT file in place of the global+repo file layers
	// (resolvedConfigPath picks Flag over Env). It is NOT written into Config
	// by applyFlagsLayer — Load sets ConfigPath itself from the resolved path.
	ConfigPath *string
	// Provider is STAGEHAND_PROVIDER / --provider (FR35, §15.2).
	Provider *string
	// Model is STAGEHAND_MODEL / --model (FR35, §15.2).
	Model *string
	// Timeout is STAGEHAND_TIMEOUT / --timeout, already parsed to a
	// time.Duration by the CLI layer (FR35, §15.2).
	Timeout *time.Duration
	// Verbose is STAGEHAND_VERBOSE / --verbose (FR35, §15.2).
	Verbose *bool
	// NoColor is STAGEHAND_NO_COLOR / --no-color (FR35, §15.2). Note the
	// underscore in the env-var name.
	NoColor *bool
}

// Load resolves the full FR34 precedence chain (PRD §16.1; decisions.md §6)
// into a Config and a provider.Registry, plus the §19 repo-local trust notice.
// The chain, applied lowest→highest, is: Default() (floor) → global TOML file
// → repo TOML file → repo git-config → env layer (flags.Env) → CLI-flag layer
// (flags.Flag). The six built-in manifests are NOT layered into Config scalars
// — they are injected by NewRegistry(provider.Builtins(), cfg.ProviderOverrides),
// which field-merges each override onto its matching built-in.
//
// Every layer is OPTIONAL: a missing file or an unset git-config key yields an
// empty overlay and a nil error, never a failure. A real read/parse error from
// any reader is propagated immediately as (cfg, nil, "", err).
//
// ConfigPath diagnostic: Load sets cfg.ConfigPath to the actually-loaded file
// path — the explicit --config / STAGEHAND_CONFIG path when that override is in
// effect; otherwise GlobalConfigPath() when the global file parsed non-empty
// (overlay had any set field or provider overrides); "" when no file was
// loaded (only defaults, or only repo/git/env/flag). The repo-file path is
// intentionally NOT recorded here (its location is fixed at
// <repoDir>/.stagehand.toml and obvious); only the user-resolvable global or
// explicit path is surfaced for diagnostics.
//
// The §19 trust notice (third return value) is non-empty ONLY when a
// REPO-LOCAL source — the repo .stagehand.toml file OR a stagehand.provider
// git-config key — set the provider. Global file, env, CLI flag, and an
// explicit --config path are user-chosen (not attacker-committable) and never
// trigger the notice. When repo-local set the provider but a higher layer
// (env/flag) overrode the final value, the notice still fires and names the
// final resolved cfg.Provider (the redirection was still visible at
// repo-local time). See docs/CONFIGURATION.md and PRD §19.
func Load(flags Flags, repoDir string) (cfg Config, reg *provider.Registry, trustNotice string, err error) {
	cfg = Default()
	repoLocalProviderSet := false

	// File layer(s). An explicit --config / STAGEHAND_CONFIG path OVERRIDES
	// discovery (§15.2): that single file replaces BOTH the global and the
	// repo file layers. Otherwise the normal global→repo discovery runs.
	if cp := resolvedConfigPath(flags); cp != "" {
		ov, perr := parseFile(cp)
		if perr != nil {
			return cfg, nil, "", perr
		}
		applyOverlay(&cfg, ov)
		mergeProviderOverrides(&cfg, ov)
		cfg.ConfigPath = cp
	} else {
		// Global file overlay (layer 3).
		gov, perr := readGlobalFile()
		if perr != nil {
			return cfg, nil, "", perr
		}
		globalHadContent := overlayHasContent(gov)
		applyOverlay(&cfg, gov)
		mergeProviderOverrides(&cfg, gov)
		if globalHadContent {
			// Record the resolved global path only when a file actually
			// parsed non-empty; "" when the global file is absent.
			if gp, gerr := GlobalConfigPath(); gerr == nil {
				cfg.ConfigPath = gp
			}
		}

		// Repo file overlay (layer 4). A repo-local provider set is tracked
		// for the §19 trust notice.
		rov, perr := readRepoFile(repoDir)
		if perr != nil {
			return cfg, nil, "", perr
		}
		if rov.Provider != nil {
			repoLocalProviderSet = true
		}
		applyOverlay(&cfg, rov)
		mergeProviderOverrides(&cfg, rov)
	}

	// Repo git-config overlay (layer 5). git-config CANNOT express
	// [provider.<name>] tables, so mergeProviderOverrides is intentionally
	// NOT called here. A repo-local provider set is tracked for the §19
	// trust notice.
	gcov, perr := readGitConfig(repoDir)
	if perr != nil {
		return cfg, nil, "", perr
	}
	if gcov.Provider != nil {
		repoLocalProviderSet = true
	}
	applyOverlay(&cfg, gcov)

	// Env layer (layer 6) then CLI-flag layer (layer 7, highest). applyFlagsLayer
	// writes only Provider/Model/Timeout/Verbose/NoColor — ConfigPath is
	// resolved+set above, not written from a FlagsLayer.
	applyFlagsLayer(&cfg, flags.Env)
	applyFlagsLayer(&cfg, flags.Flag)

	// Build the registry by field-merging the resolved overrides onto the six
	// built-in manifests (decisions.md §6). nil ProviderOverrides is correct:
	// Default leaves it nil and NewRegistry handles nil.
	reg = provider.NewRegistry(provider.Builtins(), cfg.ProviderOverrides)

	// §19 trust notice: only repo-local sources are attacker-committable, so
	// the notice is gated to them. The <name> is the FINAL resolved provider.
	if repoLocalProviderSet && cfg.Provider != "" {
		trustNotice = fmt.Sprintf("stagehand: repo-local config changed provider to %s", cfg.Provider)
	}
	return cfg, reg, trustNotice, nil
}

// resolvedConfigPath returns the explicit --config / STAGEHAND_CONFIG path when
// set, with the CLI flag winning over the env var (FR34: flag > env). Empty
// means "use normal global+repo discovery" (no override).
func resolvedConfigPath(flags Flags) string {
	if flags.Flag.ConfigPath != nil {
		return *flags.Flag.ConfigPath
	}
	if flags.Env.ConfigPath != nil {
		return *flags.Env.ConfigPath
	}
	return ""
}

// applyOverlay writes each scalar field from o onto cfg when o's pointer for
// that field is non-nil (a non-nil pointer to the ZERO value — Model="",
// Verbose=false, MaxDiffBytes=0 — counts as "set" and overwrites). It covers
// all 12 scalar fields. It does NOT touch ConfigPath (Load sets that itself)
// or ProviderOverrides (mergeProviderOverrides handles those). git-config and
// every other overlay flow through here unchanged.
func applyOverlay(cfg *Config, o overlay) {
	if o.Provider != nil {
		cfg.Provider = *o.Provider
	}
	if o.Model != nil {
		cfg.Model = *o.Model
	}
	if o.Timeout != nil {
		cfg.Timeout = *o.Timeout
	}
	if o.AutoStageAll != nil {
		cfg.AutoStageAll = *o.AutoStageAll
	}
	if o.Verbose != nil {
		cfg.Verbose = *o.Verbose
	}
	if o.NoColor != nil {
		cfg.NoColor = *o.NoColor
	}
	if o.MaxDiffBytes != nil {
		cfg.MaxDiffBytes = *o.MaxDiffBytes
	}
	if o.MaxMdLines != nil {
		cfg.MaxMdLines = *o.MaxMdLines
	}
	if o.MaxDuplicateRetries != nil {
		cfg.MaxDuplicateRetries = *o.MaxDuplicateRetries
	}
	if o.SubjectTargetChars != nil {
		cfg.SubjectTargetChars = *o.SubjectTargetChars
	}
	if o.Output != nil {
		cfg.Output = *o.Output
	}
	if o.StripCodeFence != nil {
		cfg.StripCodeFence = *o.StripCodeFence
	}
}

// applyFlagsLayer writes the five scalar fields a FlagsLayer can carry
// (Provider/Model/Timeout/Verbose/NoColor) onto cfg when non-nil. ConfigPath is
// deliberately NOT written here: it is consumed by resolvedConfigPath (and Load
// records the loaded path into cfg.ConfigPath itself).
func applyFlagsLayer(cfg *Config, l FlagsLayer) {
	if l.Provider != nil {
		cfg.Provider = *l.Provider
	}
	if l.Model != nil {
		cfg.Model = *l.Model
	}
	if l.Timeout != nil {
		cfg.Timeout = *l.Timeout
	}
	if l.Verbose != nil {
		cfg.Verbose = *l.Verbose
	}
	if l.NoColor != nil {
		cfg.NoColor = *l.NoColor
	}
}

// mergeProviderOverrides layers o's [provider.<name>] tables onto cfg's via a
// per-key SHALLOW map merge: a higher source's whole [provider.<name>] entry
// REPLACES a lower source's same-named entry, while DIFFERENT-named providers
// from lower layers survive. This per-key replace (NOT a field-merge of two
// user manifests) is deliberate: provider.mergeManifest is unexported, so
// config cannot field-merge two user manifests together — the single
// field-merge over the BUILT-IN happens once, inside NewRegistry. cfg's map is
// allocated on first contribution; a nil o.ProviderOverrides is a no-op.
func mergeProviderOverrides(cfg *Config, o overlay) {
	if o.ProviderOverrides == nil {
		return
	}
	if cfg.ProviderOverrides == nil {
		cfg.ProviderOverrides = make(map[string]provider.Manifest)
	}
	for k, v := range o.ProviderOverrides {
		cfg.ProviderOverrides[k] = v
	}
}

// overlayHasContent reports whether o carries any set field (a non-nil scalar
// pointer or a non-nil ProviderOverrides map). It is the "did this source
// actually parse a non-empty file" signal Load uses to decide whether to
// record the global config path as cfg.ConfigPath: an empty overlay means
// either a missing file or an effectively-empty one, neither of which should
// surface as the loaded config path.
func overlayHasContent(o overlay) bool {
	return o.Provider != nil ||
		o.Model != nil ||
		o.Timeout != nil ||
		o.AutoStageAll != nil ||
		o.Verbose != nil ||
		o.NoColor != nil ||
		o.MaxDiffBytes != nil ||
		o.MaxMdLines != nil ||
		o.MaxDuplicateRetries != nil ||
		o.SubjectTargetChars != nil ||
		o.Output != nil ||
		o.StripCodeFence != nil ||
		o.ProviderOverrides != nil
}
