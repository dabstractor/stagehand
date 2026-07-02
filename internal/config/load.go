package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// roleNames is the canonical list of agent roles (PRD §13.6.2 / §9.15 FR-R1).
// One source for loadEnv, loadFlags, and tests — loop all four so a --message-* flag/env
// is honored if set (registration in P4.M1.T1 may omit it; Changed==false → skipped).
var roleNames = []string{"planner", "stager", "message", "arbiter"}

// LoadOpts configures the Load() resolver. Populated by the caller (cobra PersistentPreRunE in
// P1.M4.T1.S1): ConfigPathOverride from the --config flag ("" if not passed); RepoDir = resolved repo
// root (for git config); Flags = cmd.Flags() (nil for programmatic callers -> no flag overlay).
type LoadOpts struct {
	ConfigPathOverride string         // from --config (CLI); "" => fall back to STAGEHAND_CONFIG, then discovery
	RepoDir            string         // repo root for git config (passed to loadGitConfig); "" is valid for tests
	Flags              *pflag.FlagSet // cobra/pflag set; nil => skip the CLI-flag layer
	DisableBootstrap   bool           // TEST-ONLY seam (FR-B3): true => skip the first-run auto-write. Production never sets it.
}

// setRoleProvider sets the Provider field for a role in cfg.Roles, lazily allocating the map.
// Map-value-copy write-back is REQUIRED: Go maps return value copies, so `rc.Provider = v`
// alone mutates a local copy. The write-back (`c.Roles[role] = rc`) is the load-bearing line.
// Setting one field (Provider) does NOT clobber the sibling (Model) — FR-R3 field-merge.
func (c *Config) setRoleProvider(role, provider string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Provider = provider
	c.Roles[role] = rc
}

// setRoleModel sets the Model field for a role in cfg.Roles, lazily allocating the map.
// Map-value-copy write-back is REQUIRED (same idiom as setRoleProvider).
// Setting Model does NOT clobber an existing Provider — FR-R3 field-merge.
func (c *Config) setRoleModel(role, model string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Model = model
	c.Roles[role] = rc
}

// setRoleReasoning sets the Reasoning field for a role in cfg.Roles, lazily allocating the map.
// Map-value-copy write-back is REQUIRED (same idiom as setRoleModel).
// Setting Reasoning does NOT clobber existing Provider/Model — FR-R3 field-merge.
func (c *Config) setRoleReasoning(role, reasoning string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Reasoning = reasoning
	c.Roles[role] = rc
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

	// Resolve the global-file path: --config > STAGEHAND_CONFIG > discovery (via ResolveConfigPath).
	// `explicit` records whether the path came from the user (--config / STAGEHAND_CONFIG) vs the
	// discovery default — a missing EXPLICIT path is a hard error (PRD §15.2 "overrides discovery");
	// a missing discovery file is the normal "layer absent" sentinel (tolerated below).
	globalPath := ResolveConfigPath(opts.ConfigPathOverride)
	explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""

	// Layer 2: global TOML (or --config/STAGEHAND_CONFIG override). A present file is overlaid; a
	// read/parse error is wrapped. A MISSING file is "layer absent" (no error) for discovery, but a
	// HARD ERROR when the path was explicit (a typo'd --config must not silently fall back to
	// auto-detection and invoke an unintended agent). loadTOML's (nil,nil) contract is preserved.
	var fileLoaded bool // true when ANY config file (global or repo-local) was loaded — used by the advisory
	if g, err := loadTOML(globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	} else if g != nil {
		fileLoaded = true
		overlay(&cfg, g)
	} else if explicit {
		return nil, fmt.Errorf("config file not found: %s", globalPath)
	} else if !opts.DisableBootstrap {
		// FR-B3 first-run fallback (P1.M4.T4.S1): no global config AND no explicit override → auto-write
		// the populated bootstrap config, notice the path, then load it as Layer 2.
		if err := bootstrapWriteConfig(globalPath); err != nil {
			return nil, fmt.Errorf("bootstrap config: %w", err)
		}
		fmt.Fprintf(noticeOut, "stagehand: wrote bootstrap config to %s\n", globalPath)
		if g, err := loadTOML(globalPath); err != nil {
			return nil, fmt.Errorf("global config: %w", err)
		} else if g != nil {
			fileLoaded = true
			overlay(&cfg, g)
		}
	}

	// Layer 3: repo-local TOML (CWD .stagehand.toml; emits the §19 notice). nil => absent.
	if r, err := loadRepoLocalConfig(); err != nil {
		return nil, fmt.Errorf("repo config: %w", err)
	} else if r != nil {
		fileLoaded = true
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

	// Normalize: Commits==1 ≡ Single (PRD §9.14 FR-M2c / §15.2). Covers BOTH env and flag
	// sources (applied after both layers). Only sets TRUE (never clears Single).
	if cfg.Commits == 1 {
		cfg.Single = true
	}

	// v3 in-memory migration (PRD §9.17 FR-B7): when a loaded file predates v3, fold any v2
	// default_provider into the model slash-prefix BEFORE the caller's provider.DecodeUserOverrides
	// (which would silently drop the now-removed field). Idempotent; invents nothing. The in-memory
	// Config is then v3-shaped (ConfigVersion set to current). One-time deprecation notice below.
	if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {
		orig := cfg.ConfigVersion
		migrateV2ToV3(&cfg)
		cfg.ConfigVersion = CurrentConfigVersion
		fmt.Fprint(noticeOut, migrationNotice(orig))
	} else if msg := configVersionNotice(fileLoaded, cfg.ConfigVersion); msg != "" {
		// version > current (ahead) — the only remaining live configVersionNotice case in Load
		// (version==current ⇒ ""; the older/missing cases are handled by the migration branch above).
		fmt.Fprint(noticeOut, msg)
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
	if v, ok := os.LookupEnv("STAGEHAND_REASONING"); ok && v != "" {
		cfg.Reasoning = v
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

	// Per-role provider/model overrides (PRD §9.15 FR-R3, §16.4, §9.8 FR35).
	for _, role := range roleNames {
		prefix := "STAGEHAND_" + strings.ToUpper(role)
		if v, ok := os.LookupEnv(prefix + "_PROVIDER"); ok && v != "" {
			cfg.setRoleProvider(role, v)
		}
		if v, ok := os.LookupEnv(prefix + "_MODEL"); ok && v != "" {
			cfg.setRoleModel(role, v)
		}
		if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" {
			cfg.setRoleReasoning(role, v)
		}
	}

	// STAGEHAND_COMMITS — forced commit count (PRD §9.14 FR-M2). Errors on non-integer
	// (consistent with STAGEHAND_TIMEOUT/VERBOSE/NO_COLOR error discipline).
	if v, ok := os.LookupEnv("STAGEHAND_COMMITS"); ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("STAGEHAND_COMMITS: %w", err)
		}
		cfg.Commits = n
	}

	// No STAGEHAND_EXCLUDE: deliberately omitted (§9.18 FR-X1) — a colon/comma-joined env list is a
	// documented quoting trap for glob patterns. Do NOT "helpfully" add one; [generation].exclude and
	// --exclude/-x already cover the persistent and ad-hoc cases.

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
	if fs.Changed("reasoning") {
		if v, err := fs.GetString("reasoning"); err == nil {
			cfg.Reasoning = v
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

	// Per-role provider/model overrides (PRD §9.15 FR-R3, §15.2).
	for _, role := range roleNames {
		if fs.Changed(role + "-provider") {
			if v, err := fs.GetString(role + "-provider"); err == nil {
				cfg.setRoleProvider(role, v)
			}
		}
		if fs.Changed(role + "-model") {
			if v, err := fs.GetString(role + "-model"); err == nil {
				cfg.setRoleModel(role, v)
			}
		}
		if fs.Changed(role + "-reasoning") {
			if v, err := fs.GetString(role + "-reasoning"); err == nil {
				cfg.setRoleReasoning(role, v)
			}
		}
	}

	// Decompose flags (PRD §9.14 FR-M2, §15.2).
	if fs.Changed("commits") {
		if v, err := fs.GetInt("commits"); err == nil {
			cfg.Commits = v
		}
	}
	if fs.Changed("single") || fs.Changed("no-decompose") {
		cfg.Single = true // --single and --no-decompose are aliases (FR-M2c)
	}
	if fs.Changed("max-commits") {
		if v, err := fs.GetInt("max-commits"); err == nil {
			cfg.MaxCommits = v
		}
	}

	// §9.18 FR-X1 — exclude UNIONS onto whatever the config files contributed; it does NOT replace
	// (differs from every scalar flag above, which sets DIRECTLY). Each -x/--exclude occurrence is
	// appended in CLI order after [global..., repo...] globs.
	if fs.Changed("exclude") {
		if vals, err := fs.GetStringArray("exclude"); err == nil {
			cfg.Exclude = append(cfg.Exclude, vals...)
		}
	}
}

// configVersionNotice returns the PRD §9.17 FR-B4 advisory text when a loaded config file's schema version
// is missing (0), older, or newer than CurrentConfigVersion; "" when no file was loaded (fileLoaded=false)
// or the version is current. PURE (no I/O) so it is unit-testable; the caller (Load) writes the result to
// noticeOut. config_version is metadata, not a precedence layer (PRD §16.1) — this is its only consumer.
func configVersionNotice(fileLoaded bool, version int) string {
	if !fileLoaded {
		return "" // no file → nothing to be stale
	}
	switch {
	case version == CurrentConfigVersion:
		return ""
	case version == 0:
		return fmt.Sprintf("stagehand: config file has no config_version; current is %d. "+
			"Run 'stagehand config upgrade' or 'stagehand config init --force'.\n", CurrentConfigVersion)
	case version < CurrentConfigVersion:
		return fmt.Sprintf("stagehand: config file uses schema version %d; current is %d. "+
			"Run 'stagehand config upgrade' or 'stagehand config init --force'.\n", version, CurrentConfigVersion)
	default: // version > CurrentConfigVersion
		return fmt.Sprintf("stagehand: config file uses schema version %d; this binary supports up to %d. "+
			"Upgrade stagehand, or run 'stagehand config init --force' to regenerate.\n", version, CurrentConfigVersion)
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
