package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// Decode structs — mirrors PRD §16.2 file shape (nested: [defaults], [generation],
// [provider.X]).  UNEXPORTED: only file.go decodes into these.
// ---------------------------------------------------------------------------

// fileRoleConfig is the FILE decode twin of config.RoleConfig (§16.4). A [role.planner] table decodes into
// fc.Role["planner"] EXACTLY as a [provider.pi] table decodes into fc.Provider["pi"]. materialize converts
// each to a typed RoleConfig. Both fields "" ⇒ the role inherits the global [defaults] (FR-R2).
type fileRoleConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"`
}

// fileConfig is the §16.2 file decode target: NESTED (matches [defaults]/[generation]/[role.X]/[provider.X]),
// with Timeout as a STRING ("120s") because go-toml/v2 cannot decode "120s" into time.Duration and the
// resolved Config is flat/plain (S1). loadTOML materializes this into a *Config. UNEXPORTED.
type fileConfig struct {
	ConfigVersion int                       `toml:"config_version"` // V2 — top-level metadata key (§9.17 FR-B4)
	Defaults      fileDefaults              `toml:"defaults"`
	Generation    fileGeneration            `toml:"generation"`
	Role          map[string]fileRoleConfig `toml:"role"`     // V2 — [role.<role>] per-role tables (§16.4)
	Provider      map[string]map[string]any `toml:"provider"` // nil if the file has no [provider] table
}

type fileDefaults struct {
	Provider     string `toml:"provider"`
	Model        string `toml:"model"`
	Reasoning    string `toml:"reasoning"`
	Timeout      string `toml:"timeout"` // §16.2 duration string, e.g. "120s"; parsed in loadTOML
	AutoStageAll bool   `toml:"auto_stage_all"`
	Verbose      bool   `toml:"verbose"`
}

type fileGeneration struct {
	MaxDiffBytes        int      `toml:"max_diff_bytes"`
	MaxMdLines          int      `toml:"max_md_lines"`
	TokenLimit          int      `toml:"token_limit"`  // FR3d — plumbed in S2 (materialize/overlay)
	DiffContext         *int     `toml:"diff_context"` // FR3f — *int (0-vs-unset); nil ⇒ user omitted (S2 contract correction)
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
	SubjectTargetChars  int      `toml:"subject_target_chars"`
	Output              string   `toml:"output"`
	StripCodeFence      *bool    `toml:"strip_code_fence"`
	MaxCommits          int      `toml:"max_commits"`       // V2 — safety cap on auto-decompose (§9.14 FR-M4)
	BinaryExtensions    []string `toml:"binary_extensions"` // V2 — extra non-text exts to filter (§9.1 FR3a)
	Exclude             []string `toml:"exclude"`           // V2.1 — §9.18 FR-X1 exclusion globs; UNION-merged in overlay()
	Format              string   `toml:"format"`            // V2.1 — §9.19 FR-F1 message format (validated at Load)
	Locale              string   `toml:"locale"`            // V2.1 — §9.19 FR-F6 message locale (free-form, never validated)
	Template            string   `toml:"template"`          // V2.1 — §9.19 FR-F8 message template (validated at Load)
	Push                bool     `toml:"push"`              // §9.22 FR-P1 — push after clean run (default false)
}

// ---------------------------------------------------------------------------
// noticeOut — swappable §19 notice destination (default os.Stderr).
// ---------------------------------------------------------------------------

// noticeOut is the destination for the §19 repo-local provider-redirect notice. Swappable for tests;
// defaults to os.Stderr. (PRD §19: a repo-local config redirecting the provider is surfaced to the user.)
var noticeOut io.Writer = os.Stderr

// SetNoticeOut sets the destination for the §19 repo-local provider-redirect notice (default os.Stderr).
// Intended for tests that need to observe/capture the notice. Pair with NoticeOut to restore.
// Non-test code should leave it at os.Stderr. (PRD §19; system_context §6 lists noticeOut as a swappable test sink.)
func SetNoticeOut(w io.Writer) { noticeOut = w }

// NoticeOut returns the current §19 notice destination (default os.Stderr). Pair with SetNoticeOut.
func NoticeOut() io.Writer { return noticeOut }

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

// globalConfigPath returns the GLOBAL config path (PRD §16.1 layer 2):
// $XDG_CONFIG_HOME/stagehand/config.toml when XDG_CONFIG_HOME is set AND absolute
// (XDG Base Dir Spec: a relative/empty value is ignored); otherwise
// ~/.config/stagehand/config.toml via os.UserHomeDir().
// GlobalConfigPath returns the resolved GLOBAL Stagehand config path (PRD §16.1 layer 2):
// the file `config init` writes and `config path` prints. It delegates to the unexported
// globalConfigPath() so there is a SINGLE source of truth for the global config location.
func GlobalConfigPath() string { return globalConfigPath() }

func globalConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "stagehand", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml" // last-resort fallback (CWD); matches arch §2.8
	}
	return filepath.Join(home, ".config", "stagehand", "config.toml")
}

// ResolveConfigPath returns the config file path, honoring overrides in the SAME precedence as
// config.Load: flagConfig (--config) > STAGEHAND_CONFIG env > GlobalConfigPath() discovery. It is the
// shared resolver for config.Load and the config init/upgrade/path subcommands (bugfix-001 Issue 4).
func ResolveConfigPath(flagConfig string) string {
	if flagConfig != "" {
		return flagConfig
	}
	if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
		return env
	}
	return GlobalConfigPath()
}

// repoLocalConfigPath returns the REPO-LOCAL config path (PRD §16.1 layer 3):
// the file ./.stagehand.toml.
// (Contract + PRD §16.1; NOT arch §2.8's .stagehand/config.toml directory.)
func repoLocalConfigPath() string { return ".stagehand.toml" }

// ---------------------------------------------------------------------------
// Defense-in-depth agent→provider textual remap (PRD §9.17 FR-B7 "first")
// ---------------------------------------------------------------------------

// agentKeyRe matches a bare `agent =` KEY at line start (after optional indent) — the abandoned
// intermediate terminology's defaults key. Line-oriented (multiline); captures indent + the ws+'='
// so the rewrite preserves them. Does NOT match comments, values, or [agent.*] headers.
var agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)

// remapAgentTerminology defense-in-depth-remaps the abandoned intermediate agent/[agent.*]
// terminology to provider/[provider.*] in raw TOML text BEFORE the typed decode (PRD §9.17 FR-B7
// "first"). Two transforms: (a) [agent. → [provider. table headers; (b) a bare `agent =` key →
// `provider =` (line-oriented, key name only). Pure + idempotent (a no-op on provider-terminology
// files). fileConfig has no Agent field, so without this go-toml silently drops [agent.*] tables.
func remapAgentTerminology(data []byte) []byte {
	s := string(data)
	s = strings.ReplaceAll(s, "[agent.", "[provider.")     // (a) table headers
	s = agentKeyRe.ReplaceAllString(s, "${1}provider${2}") // (b) bare key, key-name only
	return []byte(s)
}

// ---------------------------------------------------------------------------
// loadTOML — decode a §16.2 TOML file into a partial *Config
// ---------------------------------------------------------------------------

// loadTOML reads and decodes a TOML file into a partial *Config (PRD §16.2). A MISSING file is the
// normal "no override" condition: it returns (nil, nil). Other read errors and parse errors are
// returned wrapped (with the path). Only NON-ZERO fields from the file are materialized (arch §2.4
// non-zero overlay semantics — see the v1 limitation note in Config.Providers / the PRP).
func loadTOML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // not an error: layer simply absent
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Defense-in-depth (PRD §9.17 FR-B7): remap abandoned [agent.*]/agent terminology → provider
	// BEFORE the typed decode, so a v2 file using the old terminology loads with its provider block
	// preserved (otherwise go-toml silently drops [agent.*] — fileConfig has no Agent field).
	data = remapAgentTerminology(data)

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Validate the duration string up front (a malformed "timeout" must fail at LOAD, not at generation).
	var timeout time.Duration
	if fc.Defaults.Timeout != "" {
		timeout, err = time.ParseDuration(fc.Defaults.Timeout)
		if err != nil {
			return nil, fmt.Errorf("parse config %s: invalid timeout %q: %w", path, fc.Defaults.Timeout, err)
		}
	}

	return materialize(&fc, timeout), nil
}

// ---------------------------------------------------------------------------
// materialize — copy non-zero fields from fileConfig into a fresh *Config
// ---------------------------------------------------------------------------

// materialize copies the NON-ZERO fields of a fileConfig into a fresh *Config.
// Pure: receives an already-parsed duration. Non-zero overlay semantics mean
// a file cannot override a field to its zero value (false, 0, ""). See the v1
// limitation documented on Config.Providers and in the PRP design call #3.
func materialize(fc *fileConfig, timeout time.Duration) *Config {
	c := &Config{Timeout: timeout} // zero if file didn't set one (overlay skips zero — correct)
	d, g := &fc.Defaults, &fc.Generation

	if d.Provider != "" {
		c.Provider = d.Provider
	}
	if d.Model != "" {
		c.Model = d.Model
	}
	if d.Reasoning != "" {
		c.Reasoning = d.Reasoning
	}
	if d.AutoStageAll {
		c.AutoStageAll = true // v1 limitation: cannot set false via file
	}
	if d.Verbose {
		c.Verbose = true
	}
	if g.MaxDiffBytes != 0 {
		c.MaxDiffBytes = g.MaxDiffBytes
	}
	if g.MaxMdLines != 0 {
		c.MaxMdLines = g.MaxMdLines
	}
	// FR3d: TokenLimit is a plain int; 0 = unset ⇒ legacy caps (no meaningful "explicit 0").
	if g.TokenLimit != 0 {
		c.TokenLimit = g.TokenLimit
	}
	// FR3f: DiffContext is *int so an explicit 0 (changed-lines-only) is distinguishable from unset (nil).
	// NOTE: the guard is != nil, NOT != 0 — overlay() sits between every layer and the final config
	// (load.go:82 Defaults → :100/:123/:138 overlay), so a != 0 guard would silently clobber an
	// explicit diff_context=0 back to the default 1 (the contract's broken-overlay bug fixed in S2).
	if g.DiffContext != nil {
		c.DiffContext = g.DiffContext // *int: nil ⇒ unset; non-nil (incl. *0) ⇒ override
	}
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
	if g.Output != "" {
		o := g.Output
		c.Output = &o
	}
	if g.StripCodeFence != nil {
		c.StripCodeFence = g.StripCodeFence
	}
	// V2 [generation] scalars — non-zero/non-empty copy (matches every existing materialize field).
	if g.MaxCommits != 0 {
		c.MaxCommits = g.MaxCommits
	}
	if len(g.BinaryExtensions) > 0 {
		c.BinaryExtensions = g.BinaryExtensions
	}
	// §9.18 FR-X1 — single-file copy (union across layers happens in overlay(), not here).
	if len(g.Exclude) > 0 {
		c.Exclude = g.Exclude
	}
	// §9.19 FR-F1/FR-F6 — format/locale are SCALARS: non-zero/non-empty copy.
	if g.Format != "" {
		c.Format = g.Format
	}
	if g.Locale != "" {
		c.Locale = g.Locale
	}
	if g.Template != "" {
		c.Template = g.Template
	}
	// §9.22 FR-P1 — push from file (mirrors AutoStageAll/Verbose bool pattern).
	if g.Push {
		c.Push = true
	}
	// V2 top-level metadata — non-zero copy (the §9.17 advisory is P1.M4.T1's job, not here).
	if fc.ConfigVersion != 0 {
		c.ConfigVersion = fc.ConfigVersion
	}
	// V2 per-role table — convert map[string]fileRoleConfig → map[string]RoleConfig, copying every present
	// role (an all-empty [role.X] ⇒ "inherit global", harmless — mirrors Providers' whole-map copy).
	if len(fc.Role) > 0 {
		c.Roles = make(map[string]RoleConfig, len(fc.Role))
		for role, frc := range fc.Role {
			c.Roles[role] = RoleConfig(frc)
		}
	}
	c.Providers = fc.Provider // nil-safe: nil if no [provider] table
	return c
}

// ---------------------------------------------------------------------------
// overlay — field-by-field non-zero merge
// ---------------------------------------------------------------------------

// overlay merges src into dst field-by-field (arch §2.4): each NON-ZERO scalar in src overrides dst.
// The Providers map is field-merged PER PROVIDER: a field in src's [provider.X] overrides that one
// field of dst's [provider.X]; fields src omits are inherited from dst. This corrects the v1
// "key-level whole-block replace" (plan/.../P1M1T4S2 design-decisions), which silently dropped
// cross-layer provider pins — e.g. a repo [provider.pi] setting only default_model erased a global
// default_provider, leaving a bare --model that misrouted (PRD §9.8 FR37a). Field-merge with the
// BUILT-IN manifest remains the registry's job (P1.M2.T3). Nil-safe: nil src / nil src.Providers = no-op.
func overlay(dst, src *Config) {
	if src == nil {
		return
	}
	// [defaults]
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Reasoning != "" {
		dst.Reasoning = src.Reasoning
	}
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.AutoStageAll {
		dst.AutoStageAll = true
	}
	if src.Verbose {
		dst.Verbose = true
	}
	if src.Edit {
		dst.Edit = true
	}
	// §9.22 FR-P1 — push
	if src.Push {
		dst.Push = true
	}
	// [generation]
	if src.MaxDiffBytes != 0 {
		dst.MaxDiffBytes = src.MaxDiffBytes
	}
	if src.MaxMdLines != 0 {
		dst.MaxMdLines = src.MaxMdLines
	}
	// FR3d: TokenLimit plain int + != 0 (0 IS its unset sentinel per FR3d — no meaningful "explicit 0").
	if src.TokenLimit != 0 {
		dst.TokenLimit = src.TokenLimit
	}
	// FR3f: DiffContext *int + != nil. CRITICAL: this guard MUST be != nil, NOT != 0. overlay() is
	// between EVERY layer (global file / repo file / git config) and the final config (load.go:82→
	// :100/:123/:138; git.go:106), so a != 0 guard would fail `0 != 0` and silently revert an explicit
	// diff_context=0 to the -U1 default — making FR3f's 0 (changed-lines-only) unconfigurable via any
	// layer. The *int lets nil = "inherit lower layer" coexist with *0 = "explicit override to 0".
	if src.DiffContext != nil {
		dst.DiffContext = src.DiffContext // *int: nil ⇒ inherit lower layer; non-nil ⇒ override (incl. *0)
	}
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
	if src.Output != nil {
		dst.Output = src.Output
	}
	if src.StripCodeFence != nil {
		dst.StripCodeFence = src.StripCodeFence
	}
	// [provider.X] — field-level merge across config layers (PRD §9.8 FR37a). A field src sets overrides
	// that one field only; fields src omits survive from lower layers. Reverses the v1 key-level
	// "whole-block replace", which dropped cross-layer provider pins and caused bare-model misroutes.
	if len(src.Providers) > 0 {
		if dst.Providers == nil {
			dst.Providers = make(map[string]map[string]any, len(src.Providers))
		}
		for name, entry := range src.Providers {
			if dst.Providers[name] == nil {
				dst.Providers[name] = make(map[string]any, len(entry))
			}
			for k, v := range entry {
				dst.Providers[name][k] = v // field-level override; lower-layer fields survive
			}
		}
	}
	// V2 [role.<role>] — per-role FIELD-MERGE across config layers (PRD §16.4 FR-R3). A field a higher layer
	// sets overrides that one field only; fields the higher layer omits survive from lower layers. This is
	// the typed (Provider/Model) analog of the [provider.X] field-merge above (PRD §9.8 FR37a): a repo
	// [role.planner] model="X" must NOT erase a global [role.planner] provider="agy". Nil-safe.
	if len(src.Roles) > 0 {
		if dst.Roles == nil {
			dst.Roles = make(map[string]RoleConfig, len(src.Roles))
		}
		for role, rc := range src.Roles {
			existing := dst.Roles[role] // zero value if absent — fine (inherit-global sentinel)
			if rc.Provider != "" {
				existing.Provider = rc.Provider
			}
			if rc.Model != "" {
				existing.Model = rc.Model
			}
			if rc.Reasoning != "" {
				existing.Reasoning = rc.Reasoning
			}
			dst.Roles[role] = existing
		}
	}
	// V2 scalars — non-zero/non-empty wins (matches every existing overlay field).
	if src.ConfigVersion != 0 {
		dst.ConfigVersion = src.ConfigVersion
	}
	if src.MaxCommits != 0 {
		dst.MaxCommits = src.MaxCommits
	}
	if len(src.BinaryExtensions) > 0 {
		dst.BinaryExtensions = src.BinaryExtensions // REPLACE, not append (runtime denylist merge is P2.M1)
	}
	// §9.18 FR-X1: exclude UNIONS across layers (global → repo), the ONE list key that accumulates
	// rather than replaces (§16.1). A repo must not be able to DROP a globally-excluded glob. This is
	// a DELIBERATE exception to the REPLACE pattern used by BinaryExtensions above.
	if len(src.Exclude) > 0 {
		dst.Exclude = append(dst.Exclude, src.Exclude...)
	}
	// §9.19 FR-F1/FR-F6 — format/locale are SCALARS: standard non-zero REPLACE (the rule), NOT union
	// (only Exclude unions, FR-X1). overlay is called global→repo→gitconfig; highest non-empty layer wins.
	if src.Format != "" {
		dst.Format = src.Format
	}
	if src.Locale != "" {
		dst.Locale = src.Locale
	}
	if src.Template != "" {
		dst.Template = src.Template
	}
}

// ---------------------------------------------------------------------------
// loadRepoLocalConfig — repo-local file loader with §19 provider notice
// ---------------------------------------------------------------------------

// loadRepoLocalConfig loads the repo-local ./.stagehand.toml. If it sets the default provider, a
// one-line notice is written to noticeOut (default os.Stderr) per PRD §19 (a repo file redirecting
// the provider is surfaced to the user). Returns (nil, nil) if the file is absent.
func loadRepoLocalConfig() (*Config, error) {
	cfg, err := loadTOML(repoLocalConfigPath())
	if err != nil {
		return nil, err
	}
	if msg := repoProviderNotice(cfg); msg != "" {
		fmt.Fprint(noticeOut, msg)
	}
	return cfg, nil
}

// repoProviderNotice returns the §19 notice text iff cfg is non-nil and sets Provider != ""; else "".
// Pure (no I/O) so it is trivially unit-testable. (The notice flags the FILE's setting, not the final
// provider — higher layers may still override.)
func repoProviderNotice(cfg *Config) string {
	if cfg == nil || cfg.Provider == "" {
		return ""
	}
	return fmt.Sprintf("stagehand: repo-local config (.stagehand.toml) sets provider to %q\n", cfg.Provider)
}
