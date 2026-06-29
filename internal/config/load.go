package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/pflag"
)

// LoadOpts configures the Load() resolver. Populated by the caller (cobra PersistentPreRunE in
// P1.M4.T1.S1): ConfigPathOverride from the --config flag ("" if not passed); RepoDir = resolved repo
// root (for git config); Flags = cmd.Flags() (nil for programmatic callers -> no flag overlay).
type LoadOpts struct {
	ConfigPathOverride string         // from --config (CLI); "" => fall back to STAGEHAND_CONFIG, then discovery
	RepoDir            string         // repo root for git config (passed to loadGitConfig); "" is valid for tests
	Flags              *pflag.FlagSet // cobra/pflag set; nil => skip the CLI-flag layer
}

// Load resolves the full Stagehand configuration by applying PRD §16.1 layers in precedence order
// (lowest → highest): (1) built-in Defaults(); (2) global TOML; (3) repo-local TOML; (4) repo git
// config; (5) STAGEHAND_* env vars; (7) CLI flags (only explicitly-set ones). Higher wins. Returns one
// fully-resolved *Config (never nil on success). Any layer's hard error (unreadable file, bad parse,
// git failure) is wrapped with its layer context and returned, failing load.
//
// The global-file PATH itself is resolved FIRST: opts.ConfigPathOverride (--config) > STAGEHAND_CONFIG
// (env) > globalConfigPath() discovery (FINDING 4). loadEnv/loadFlags set bool fields DIRECTLY (not via
// overlay) so a boolean can be forced false — the documented escape hatch the non-zero overlay layers
// cannot provide (FINDING 3). loadGitConfig(opts.RepoDir) is layer 4; loadRepoLocalConfig() reads CWD.
func Load(ctx context.Context, opts LoadOpts) (*Config, error) {
	// Honor ctx minimally — frozen loaders take no ctx; this is the cancellation seam.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	cfg := Defaults() // Layer 1 (by value)

	// Resolve the global-file path: --config > STAGEHAND_CONFIG > discovery.
	globalPath := opts.ConfigPathOverride
	if globalPath == "" {
		if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
			globalPath = env
		} else {
			globalPath = globalConfigPath()
		}
	}

	// Layer 2: global TOML (or --config/STAGEHAND_CONFIG override). nil => absent (no error).
	if g, err := loadTOML(globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	} else if g != nil {
		overlay(&cfg, g)
	}

	// Layer 3: repo-local TOML (CWD .stagehand.toml; emits the §19 notice). nil => absent.
	if r, err := loadRepoLocalConfig(); err != nil {
		return nil, fmt.Errorf("repo config: %w", err)
	} else if r != nil {
		overlay(&cfg, r)
	}

	// Layer 4: repo git config (stagehand.* keys). Non-nil partial *Config; errors propagate.
	gc, err := loadGitConfig(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("git config: %w", err)
	}
	if gc != nil {
		overlay(&cfg, gc)
	}

	// Layer 5: STAGEHAND_* env vars (DIRECT set — booleans can be false).
	if err := loadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("env config: %w", err)
	}

	// Layer 7 (6): CLI flags — ONLY explicitly-set ones (flags.Changed). Skipped if opts.Flags == nil.
	if opts.Flags != nil {
		loadFlags(&cfg, opts.Flags)
	}

	return &cfg, nil
}

// loadEnv overlays STAGEHAND_* environment variables (PRD §15.2/FR35, §16.1 layer 5). Presence-semantic:
// a PRESENT, non-empty value overrides; an unset/empty var is a no-op. Booleans are set DIRECTLY (not
// via overlay) so STAGEHAND_VERBOSE=false / STAGEHAND_NO_COLOR=true work — the escape hatch the non-zero
// overlay layers cannot provide (FINDING 3). STAGEHAND_CONFIG is NOT handled here (it selects the file
// path, resolved in Load). A present-but-unparseable bool/timeout is a wrapped error (fail at load).
func loadEnv(cfg *Config) error {
	if v, ok := os.LookupEnv("STAGEHAND_PROVIDER"); ok && v != "" {
		cfg.Provider = v
	}
	if v, ok := os.LookupEnv("STAGEHAND_MODEL"); ok && v != "" {
		cfg.Model = v
	}
	if v, ok := os.LookupEnv("STAGEHAND_TIMEOUT"); ok && v != "" {
		d, err := parseTimeout(v)
		if err != nil {
			return fmt.Errorf("STAGEHAND_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
	}
	if v, ok := os.LookupEnv("STAGEHAND_VERBOSE"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGEHAND_VERBOSE: %w", err)
		}
		cfg.Verbose = b // DIRECT set — can be false (escape hatch)
	}
	if v, ok := os.LookupEnv("STAGEHAND_NO_COLOR"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGEHAND_NO_COLOR: %w", err)
		}
		cfg.NoColor = b // DIRECT set — NoColor (toml:"-") becomes resolvable here for the first time
	}
	return nil
}

// loadFlags overlays explicitly-set CLI flags (PRD §15.2, §16.1 layer 7). For each Config-backed flag
// (provider, model, timeout, verbose, no-color), if fs.Changed(name) the value is read and set DIRECTLY
// — so --no-color / --verbose=false-style semantics work and an explicit flag beats every lower layer.
// Unchanged flags are IGNORED (a default value is NOT an override). Behavioral flags (--all, --dry-run,
// --version, --help) are NOT Config fields and are ignored. Assumes --timeout is registered as a pflag
// STRING flag (coordination note for P1.M4.T1.S1; FINDING 7). fs may be nil (Load guards it).
func loadFlags(cfg *Config, fs *pflag.FlagSet) {
	if fs.Changed("provider") {
		if v, err := fs.GetString("provider"); err == nil {
			cfg.Provider = v
		}
	}
	if fs.Changed("model") {
		if v, err := fs.GetString("model"); err == nil {
			cfg.Model = v
		}
	}
	if fs.Changed("timeout") {
		if v, err := fs.GetString("timeout"); err == nil {
			if d, perr := parseTimeout(v); perr == nil {
				cfg.Timeout = d
			}
		}
	}
	if fs.Changed("verbose") {
		if v, err := fs.GetBool("verbose"); err == nil {
			cfg.Verbose = v // DIRECT set — can be false (escape hatch)
		}
	}
	if fs.Changed("no-color") {
		if v, err := fs.GetBool("no-color"); err == nil {
			cfg.NoColor = v // DIRECT set
		}
	}
}

// parseTimeout parses a duration that may be EITHER a Go duration string ("120s", "2m") OR a bare
// integer (seconds: "120"). Used by both STAGEHAND_TIMEOUT (env) and --timeout (CLI). Returns a wrapped
// error if neither form parses.
func parseTimeout(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid timeout %q (expected e.g. \"120s\" or 120)", s)
}
