package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// Decode structs — mirrors PRD §16.2 file shape (nested: [defaults], [generation],
// [provider.X]).  UNEXPORTED: only file.go decodes into these.
// ---------------------------------------------------------------------------

// fileConfig is the §16.2 file decode target: NESTED (matches [defaults]/[generation]/[provider.X]),
// with Timeout as a STRING ("120s") because go-toml/v2 cannot decode "120s" into time.Duration and the
// resolved Config is flat/plain (S1). loadTOML materializes this into a *Config. UNEXPORTED.
type fileConfig struct {
	Defaults   fileDefaults              `toml:"defaults"`
	Generation fileGeneration            `toml:"generation"`
	Provider   map[string]map[string]any `toml:"provider"` // nil if the file has no [provider] table
}

type fileDefaults struct {
	Provider     string `toml:"provider"`
	Model        string `toml:"model"`
	Timeout      string `toml:"timeout"` // §16.2 duration string, e.g. "120s"; parsed in loadTOML
	AutoStageAll bool   `toml:"auto_stage_all"`
	Verbose      bool   `toml:"verbose"`
}

type fileGeneration struct {
	MaxDiffBytes        int    `toml:"max_diff_bytes"`
	MaxMdLines          int    `toml:"max_md_lines"`
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"`
	SubjectTargetChars  int    `toml:"subject_target_chars"`
	Output              string `toml:"output"`
	StripCodeFence      bool   `toml:"strip_code_fence"`
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

// repoLocalConfigPath returns the REPO-LOCAL config path (PRD §16.1 layer 3):
// the file ./.stagehand.toml.
// (Contract + PRD §16.1; NOT arch §2.8's .stagehand/config.toml directory.)
func repoLocalConfigPath() string { return ".stagehand.toml" }

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
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
	if g.Output != "" {
		c.Output = g.Output
	}
	if g.StripCodeFence {
		c.StripCodeFence = true // v1 limitation: cannot set false via file
	}
	c.Providers = fc.Provider // nil-safe: nil if no [provider] table
	return c
}

// ---------------------------------------------------------------------------
// overlay — field-by-field non-zero merge
// ---------------------------------------------------------------------------

// overlay merges src into dst field-by-field (arch §2.4): each NON-ZERO scalar in src overrides dst;
// the Providers map is merged KEY-BY-KEY (a key in src replaces dst's whole entry for that key — no
// sub-field merge within a provider at the file↔file boundary; field-merge with BUILT-IN manifests is
// the registry's job, P1.M2.T3). Nil-safe: a nil src (or nil src.Providers) is a no-op for that part.
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
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.AutoStageAll {
		dst.AutoStageAll = true
	}
	if src.Verbose {
		dst.Verbose = true
	}
	// [generation]
	if src.MaxDiffBytes != 0 {
		dst.MaxDiffBytes = src.MaxDiffBytes
	}
	if src.MaxMdLines != 0 {
		dst.MaxMdLines = src.MaxMdLines
	}
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
	if src.Output != "" {
		dst.Output = src.Output
	}
	if src.StripCodeFence {
		dst.StripCodeFence = true
	}
	// [provider.X]
	if len(src.Providers) > 0 {
		if dst.Providers == nil {
			dst.Providers = make(map[string]map[string]any, len(src.Providers))
		}
		for name, entry := range src.Providers {
			dst.Providers[name] = entry // key-level replace (arch §2.4)
		}
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
