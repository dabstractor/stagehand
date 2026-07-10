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
	ConfigPathOverride string         // from --config (CLI); "" => fall back to STAGECOACH_CONFIG, then discovery
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

// Load resolves the full Stagecoach configuration by applying PRD §16.1 layers in precedence order
// (lowest → highest): (1) built-in Defaults(); (2) global TOML; (3) repo-local TOML; (4) repo git
// config; (5) STAGECOACH_* env vars; (7) CLI flags (only explicitly-set ones). Higher wins. Returns one
// fully-resolved *Config (never nil on success). Any layer's hard error (unreadable file, bad parse,
// git failure) is wrapped with its layer context and returned, failing load.
//
// The global-file PATH itself is resolved FIRST: opts.ConfigPathOverride (--config) > STAGECOACH_CONFIG
// (env) > globalConfigPath() discovery (FINDING 4). loadEnv/loadFlags set bool fields DIRECTLY (not via
// overlay) so a boolean can be forced false — the documented escape hatch the non-zero overlay layers
// cannot provide (FINDING 3). loadGitConfig(opts.RepoDir) is layer 4; loadRepoLocalConfig() reads CWD.
func Load(ctx context.Context, opts LoadOpts) (*Config, error) {
	// Honor ctx minimally — frozen loaders take no ctx; this is the cancellation seam.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	cfg := Defaults() // Layer 1 (by value)

	// Resolve the global-file path: --config > STAGECOACH_CONFIG > discovery (via ResolveConfigPath).
	// `explicit` records whether the path came from the user (--config / STAGECOACH_CONFIG) vs the
	// discovery default — a missing EXPLICIT path is a hard error (PRD §15.2 "overrides discovery");
	// a missing discovery file is the normal "layer absent" sentinel (tolerated below).
	globalPath := ResolveConfigPath(opts.ConfigPathOverride)
	explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""

	// Layer 2: global TOML (or --config/STAGECOACH_CONFIG override). A present file is overlaid; a
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
		fmt.Fprintf(noticeOut, "stagecoach: wrote bootstrap config to %s\n", globalPath)
		if g, err := loadTOML(globalPath); err != nil {
			return nil, fmt.Errorf("global config: %w", err)
		} else if g != nil {
			fileLoaded = true
			overlay(&cfg, g)
		}
	}

	// Layer 3: repo-local TOML (CWD .stagecoach.toml; emits the §19 notice). nil => absent.
	if r, err := loadRepoLocalConfig(); err != nil {
		return nil, fmt.Errorf("repo config: %w", err)
	} else if r != nil {
		fileLoaded = true
		overlay(&cfg, r)
	}

	// fileProvider = cfg.Provider as set by the CONFIG-FILE layers (Defaults + global + repo-local),
	// captured BEFORE the ambient layers (gitconfig / STAGECOACH_* env / --provider flag) overlay it.
	// Used by the self-hosting stub guard below to tell an intentional file-selected provider from
	// one smuggled in by a leaked environment. MUST stay positioned before Layer 4 (gitconfig).
	fileProvider := cfg.Provider

	// Layer 4: repo git config (stagecoach.* keys). Non-nil partial *Config; errors propagate.
	gc, err := loadGitConfig(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("git config: %w", err)
	}
	if gc != nil {
		overlay(&cfg, gc)
	}

	// Layer 5: STAGECOACH_* env vars (DIRECT set — booleans can be false).
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

	// PRD §9.19 FR-F1: an unknown format mode is a HARD configuration error. Validate the RESOLVED
	// value once (not per-layer — a low-layer typo overridden higher is not an error). Locale is
	// deliberately NOT validated (FR-F6: free-form, verbatim, no i18n tables).
	if err := validateFormat(cfg.Format); err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	if err := validateTemplate(cfg.Template); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	// §9.1 FR3f — diff_context is an integer in [0,3]. The config layer accepts any int and would otherwise
	// be silently clamped to 1 by buildDiffArgs (internal/git/git.go) with NO diagnostic (bugfix Issue 2).
	// Validate the fully-merged *int here so an out-of-range value (e.g. a typo'd 5) fails load with a clear,
	// field-named error. The nil (unset ⇒ default 1) and *0 (-U0, changed-lines-only) cases are VALID.
	if err := validateDiffContext(cfg.DiffContext); err != nil {
		return nil, fmt.Errorf("diff_context: %w", err)
	}
	// Finding 5: --commits / STAGECOACH_COMMITS / stagecoach.commits must be >= 0. A negative value is
	// meaningless (0 = auto-decompose, 1 = --single, N>=2 = force N). Surface it as a usage error at
	// load time rather than letting it fall through to downstream behavior.
	if err := validateCommits(cfg.Commits); err != nil {
		return nil, fmt.Errorf("commits: %w", err)
	}

	// Self-hosting guard (FR-SH1). "stub" (cmd/stubagent) is a TEST-ONLY provider double that echoes
	// $STAGECOACH_STUB_OUT verbatim as the "generated" message. It is valid ONLY when selected
	// intentionally — via the --provider flag or a config FILE (--config / repo-local .stagecoach.toml
	// / global). Refuse AMBIENT selection via $STAGECOACH_PROVIDER or the stagecoach.provider repo
	// git-config: those are the channels by which a leaked test environment (an exported
	// STAGECOACH_PROVIDER=stub + STAGECOACH_STUB_OUT left sitting in a shell) silently hijacks a real
	// `git commit-pi` / bare `stagecoach` and mints nonsense commits ("feat: add a", "x", …). Tests
	// select stub via --provider stub or a config file, so they are unaffected; fileProvider (captured
	// above, after the file layers and before the ambient layers) distinguishes a genuine file pick.
	if cfg.Provider == "stub" {
		viaFlag := opts.Flags != nil && opts.Flags.Changed("provider")
		if !viaFlag && fileProvider != "stub" {
			src := "an ambient source"
			switch {
			case os.Getenv("STAGECOACH_PROVIDER") == "stub":
				src = "$STAGECOACH_PROVIDER"
			case gc != nil && gc.Provider == "stub":
				src = "git config stagecoach.provider"
			}
			return nil, fmt.Errorf("refusing test-only provider %q: selected via %s, not --provider "+
				"or a config file (a leaked test environment would mint garbage commits through "+
				"commit-pi/stagecoach). Pass --provider stub explicitly, or unset the ambient source",
				"stub", src)
		}
	}

	return &cfg, nil
}

// loadEnv overlays STAGECOACH_* environment variables (PRD §15.2/FR35, §16.1 layer 5). Presence-semantic:
// a PRESENT, non-empty value overrides; an unset/empty var is a no-op. Booleans are set DIRECTLY (not
// via overlay) so STAGECOACH_VERBOSE=false / STAGECOACH_NO_COLOR=true work — the escape hatch the non-zero
// overlay layers cannot provide (FINDING 3). STAGECOACH_CONFIG is NOT handled here (it selects the file
// path, resolved in Load). A present-but-unparseable bool/timeout is a wrapped error (fail at load).
func loadEnv(cfg *Config) error {
	if v, ok := os.LookupEnv("STAGECOACH_PROVIDER"); ok && v != "" {
		cfg.Provider = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_MODEL"); ok && v != "" {
		cfg.Model = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_REASONING"); ok && v != "" {
		cfg.Reasoning = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_TIMEOUT"); ok && v != "" {
		d, err := parseTimeout(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
	}
	if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
		// VERBOSE=2 is advertised by PRD §19 / docs as the level that would additionally log the
		// stdin CONTENTS. It is NOT implemented today (Config.Verbose is a bool, and logging payload
		// contents would require promoting it to an int). Rather than failing with an opaque
		// strconv.ParseBool error, reject it up front with an actionable message so the failure is
		// a clear "unimplemented" instead of a confusing parse trace. Any genuinely malformed value
		// ("notabool") still gets the normal wrapped ParseBool error below.
		if v == "2" {
			return fmt.Errorf("STAGECOACH_VERBOSE: 2 is not supported yet (payload contents logging is unimplemented); " +
				"use STAGECOACH_VERBOSE=true for the size-only diagnostics")
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
		}
		cfg.Verbose = b // DIRECT set — can be false (escape hatch)
	}
	if v, ok := os.LookupEnv("STAGECOACH_NO_COLOR"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_NO_COLOR: %w", err)
		}
		cfg.NoColor = b // DIRECT set — NoColor (toml:"-") becomes resolvable here for the first time
	}

	// Per-role provider/model overrides (PRD §9.15 FR-R3, §16.4, §9.8 FR35).
	for _, role := range roleNames {
		prefix := "STAGECOACH_" + strings.ToUpper(role)
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

	// STAGECOACH_COMMITS — forced commit count (PRD §9.14 FR-M2). Errors on non-integer
	// (consistent with STAGECOACH_TIMEOUT/VERBOSE/NO_COLOR error discipline).
	if v, ok := os.LookupEnv("STAGECOACH_COMMITS"); ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_COMMITS: %w", err)
		}
		cfg.Commits = n
	}

	// No STAGECOACH_EXCLUDE: deliberately omitted (§9.18 FR-X1) — a colon/comma-joined env list is a
	// documented quoting trap for glob patterns. Do NOT "helpfully" add one; [generation].exclude and
	// --exclude/-x already cover the persistent and ad-hoc cases.

	// §9.19 FR-F1/FR-F6 — format/locale via env (presence-semantic, mirrors STAGECOACH_PROVIDER).
	if v, ok := os.LookupEnv("STAGECOACH_FORMAT"); ok && v != "" {
		cfg.Format = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_LOCALE"); ok && v != "" {
		cfg.Locale = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_TEMPLATE"); ok && v != "" {
		cfg.Template = v
	}

	// §9.22 FR-P1 — push via env (presence-semantic, mirrors STAGECOACH_VERBOSE).
	if v, ok := os.LookupEnv("STAGECOACH_PUSH"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_PUSH: %w", err)
		}
		cfg.Push = b // DIRECT set — can be false (escape hatch)
	}

	// §9.25 FR-V5 — no_verify via env (presence-semantic, DIRECT set — can be false, the escape hatch).
	if v, ok := os.LookupEnv("STAGECOACH_NO_VERIFY"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_NO_VERIFY: %w", err)
		}
		cfg.NoVerify = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_PUSH)
	}

	// §9.27 FR-K6 — no_parent_watchdog via env (presence-semantic, DIRECT set — can be false, the escape hatch).
	if v, ok := os.LookupEnv("STAGECOACH_NO_PARENT_WATCHDOG"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_NO_PARENT_WATCHDOG: %w", err)
		}
		cfg.NoParentWatchdog = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_NO_VERIFY)
	}

	// §9.4 FR16 / §9.8 FR35 / §15.2 layer 5 — auto_stage_all via env (presence-semantic, DIRECT *bool
	// set; mirrors STAGECOACH_PUSH). boolPtr(b) makes a non-nil incl. *false the explicit override a
	// default-true field needs (env DIRECT-set beats default/file/git layers 1-4).
	if v, ok := os.LookupEnv("STAGECOACH_AUTO_STAGE_ALL"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err)
		}
		cfg.AutoStageAll = boolPtr(b) // DIRECT *bool set — non-nil so false overrides the default-true lower layers
	}

	// §9.24 FR-T1c / §9.8 FR35 / §15.2 layer 5 — multi_turn_fallback via env (presence-semantic, DIRECT
	// *bool set; mirrors STAGECOACH_PUSH). boolPtr(b) makes a non-nil incl. *false the explicit override
	// a default-true field needs (env DIRECT-set beats default/file/git layers 1-4).
	if v, ok := os.LookupEnv("STAGECOACH_MULTI_TURN_FALLBACK"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_MULTI_TURN_FALLBACK: %w", err)
		}
		cfg.MultiTurnFallback = boolPtr(b) // DIRECT *bool set — non-nil so false overrides the default-true lower layers
	}

	// §9.26 FR-W1 — work-description text via env (presence-semantic, mirrors STAGECOACH_PROVIDER).
	// --work-description-file (when set) wins over this in loadFlags; env sets the base value here.
	if v, ok := os.LookupEnv("STAGECOACH_WORK_DESCRIPTION"); ok && v != "" {
		cfg.WorkDescription = v
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

	// §9.19 FR-F1/FR-F6 — format/locale via CLI flags (gated on fs.Changed, mirrors provider).
	if fs.Changed("format") {
		if v, err := fs.GetString("format"); err == nil {
			cfg.Format = v
		}
	}
	if fs.Changed("locale") {
		if v, err := fs.GetString("locale"); err == nil {
			cfg.Locale = v
		}
	}
	if fs.Changed("template") {
		if v, err := fs.GetString("template"); err == nil {
			cfg.Template = v
		}
	}

	// §9.19 FR-F7 — context via CLI flag ONLY (no env/git/file source; per-invocation). Mirrors --exclude's
	// flag-only discipline (there is no STAGECOACH_CONTEXT / stagecoach.context / [generation].context).
	if fs.Changed("context") {
		if v, err := fs.GetString("context"); err == nil {
			cfg.Context = v
		}
	}

	// §9.22 FR-E1 — --edit flag (flag-only; no env/git/file source; per-invocation). Mirrors --context's
	// flag-only discipline (there is no STAGECOACH_EDIT / stagecoach.edit / [generation].edit).
	if fs.Changed("edit") {
		if v, err := fs.GetBool("edit"); err == nil {
			cfg.Edit = v
		}
	}

	// §9.22 FR-P1 — --push flag (full 5-layer precedence; mirrors --verbose/--template).
	if fs.Changed("push") {
		if v, err := fs.GetBool("push"); err == nil {
			cfg.Push = v // DIRECT set
		}
	}

	// §9.25 FR-V5 — --no-verify flag (DIRECT set; mirrors --push). Flag var registered in P1.M1.T2.S1.
	if fs.Changed("no-verify") {
		if v, err := fs.GetBool("no-verify"); err == nil {
			cfg.NoVerify = v // DIRECT set
		}
	}

	// §9.26 FR-W1 — work-description mode. --work-description sets the text; --work-description-file
	// reads it from a file and WINS if both are set (FR-W1). ENV (STAGECOACH_WORK_DESCRIPTION) already
	// set the base value above; the flag(s) override it here when changed.
	if fs.Changed("work-description") {
		if v, err := fs.GetString("work-description"); err == nil {
			cfg.WorkDescription = v
		}
	}
	if fs.Changed("work-description-file") {
		if path, err := fs.GetString("work-description-file"); err == nil {
			if b, rerr := os.ReadFile(path); rerr == nil {
				cfg.WorkDescription = string(b) // file wins over --work-description AND env (FR-W1)
			}
		}
	}
}

// validFormats is the closed set of --format modes (PRD §9.19 FR-F1). Validation-only; S3 builds the
// prompt scaffolds from static strings, not this slice.
var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}

// validateFormat returns nil iff format is one of validFormats, else an error naming the offending value
// and the valid set (PRD §9.19 FR-F1: "An unknown mode is a hard configuration error"). PURE (no I/O) so it
// is unit-testable; called ONCE at the tail of Load() on the FULLY RESOLVED cfg.Format (not per-layer — a
// low-layer typo overridden by a higher layer is not an error). Locale is deliberately NOT validated (FR-F6).
func validateFormat(format string) error {
	for _, m := range validFormats {
		if format == m {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))
}

// validateDiffContext rejects an out-of-range diff_context (PRD §9.1 FR3f: integer 0–3). It is the
// config-layer diagnostic for bugfix Issue 2 (buildDiffArgs otherwise silently clamps to 1). Semantics:
// nil ⇒ unset ⇒ valid (default 1 applied by DiffContextValue); *0 ⇒ valid (-U0, changed-lines-only);
// *1/*2/*3 ⇒ valid. ONLY *v<0 or *v>3 is an error. PURE (no I/O) so it is unit-testable directly.
// Called from Load after every layer (file + git-config) has merged into cfg.DiffContext (the single
// chokepoint — diff_context has no env/flag source). Mirrors the validateFormat/validateTemplate shape.
func validateDiffContext(dc *int) error {
	if dc == nil {
		return nil // unset ⇒ default 1 (valid)
	}
	if v := *dc; v < 0 || v > 3 {
		return fmt.Errorf("must be in range [0,3]: got %d", v)
	}
	return nil // *0, *1, *2, *3 all valid
}

// validateTemplate returns nil iff tpl is empty or contains the literal "$msg" substring, else an error
// (PRD §9.19 FR-F8: "must contain the literal $msg", hard configuration error otherwise). PURE (no I/O) so
// it is unit-testable; called ONCE at the tail of Load() on the FULLY RESOLVED cfg.Template (not per-layer —
// a low-layer template overridden by a valid higher layer is not an error).
func validateTemplate(tpl string) error {
	if tpl == "" || strings.Contains(tpl, "$msg") {
		return nil
	}
	return fmt.Errorf("invalid template %q: must contain the literal $msg (e.g. %q)", tpl, "$msg (#205)")
}

// validateCommits rejects a nonsensical negative forced commit count (Finding 5). Semantics after
// Load's normalization: 0 ⇒ auto-decompose (valid), 1 ⇒ --single (valid, normalized), ≥2 ⇒ force N
// (valid). A negative value is meaningless and currently falls through silently (--dry-run hits the
// auto-stage path; a real run reaches the planner with forcedCount<0). Reject it up front. PURE (no I/O)
// so it is unit-testable; called ONCE at the tail of Load() on the fully-merged cfg.Commits.
func validateCommits(n int) error {
	if n < 0 {
		return fmt.Errorf("invalid commits %d: must be >= 0 (0 = auto-decompose, 1 = --single, N>=2 = force N)", n)
	}
	return nil
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
		return fmt.Sprintf("stagecoach: config file has no config_version; current is %d. "+
			"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n", CurrentConfigVersion)
	case version < CurrentConfigVersion:
		return fmt.Sprintf("stagecoach: config file uses schema version %d; current is %d. "+
			"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n", version, CurrentConfigVersion)
	default: // version > CurrentConfigVersion
		return fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
			"Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n", version, CurrentConfigVersion)
	}
}

// parseTimeout parses a duration that may be EITHER a Go duration string ("120s", "2m") OR a bare
// integer (seconds: "120"). Used by both STAGECOACH_TIMEOUT (env) and --timeout (CLI). Returns a wrapped
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
