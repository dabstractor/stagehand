# Architectural Research: CLI ↔ pkg/stagehand Config-Handoff Seam

**Scope:** root cause of PRD Issue 1 (--config not honored by `pkg/stagehand.GenerateCommit`) and PRD Issue 5 (§19 repo-local provider notice printed twice).
**Verdict:** both bugs share a single root cause — `runDefault` calls `stagehand.GenerateCommit`, whose internal `resolveConfig` re-runs `config.Load` with a **fresh `LoadOpts`** that drops `--config` (`ConfigPathOverride: ""`) and drops the CLI flags (`Flags: nil`). The notice double-print is a direct side-effect of that second `Load`.

---

## 1. `GenerateCommit` signature and `Options` struct

**File:** `pkg/stagehand/stagehand.go:34-67` (verbatim)

```go
// GenerateCommit generates and (unless Options.DryRun) creates a commit from the
// currently-staged index. …
// Stable as of v1.0.
func GenerateCommit(ctx context.Context, opts Options) (Result, error) {
	cfg, repoDir, err := resolveConfig(ctx, opts)
	…
}
```

```go
type Options struct {
	Provider    string        // manifest name; "" → resolved default (auto-detect installed built-ins)
	Model       string        // "" → manifest default_model
	SystemExtra string        // appended to the built system prompt (extra integrator instructions)
	DryRun      bool          // if true, return the message WITHOUT committing (CommitSHA == "")
	Timeout     time.Duration // per-attempt generation timeout; 0 → config default (120s)
	Verbose     io.Writer     // optional; when set AND cfg.Verbose, diagnostics …
	VerboseOn   bool          // when true, forces cfg.Verbose=true (highest precedence)
}
```

**Critical gap for Issue 1:** `Options` has **no** field for `ConfigPathOverride` / `--config`. There is no way for a caller to forward the `--config` flag through `GenerateCommit` to its internal `config.Load`. The struct is documented as "ADDITIVE-ONLY for future versions" — adding a field is allowed but is the only fix surface.

`Result` (for completeness): `{CommitSHA, Subject, Message, Provider, Model string}` — `pkg/stagehand/stagehand.go:69-77`.

---

## 2. The exact call site in `runDefault`

**File:** `internal/cmd/default_action.go:147-156` (verbatim)

```go
	res, err := stagehand.GenerateCommit(ctx, stagehand.Options{
		Provider:  cfg.Provider,
		Model:     cfg.Model,
		Timeout:   cfg.Timeout,
		DryRun:    flagDryRun,
		Verbose:   stderr,
		VerboseOn: cfg.Verbose,
	})
	if err != nil {
		return handleGenError(stderr, err) // §4: rescue/CAS/timeout/nothing/generic matrix
	}
```

**Surrounding context** (the comment immediately above, `default_action.go:130-134`):

```go
	// §3: re-apply the CLI-resolved provider/model/timeout (Layer-7 flags already applied by
	// PersistentPreRunE) as Options — GenerateCommit re-loads config with Flags:nil, so opts is how the
	// CLI flags take effect (opts override is highest precedence in resolveConfig).
```

This comment **documents the workaround** and the bug simultaneously: the author KNEW `GenerateCommit` re-loads config with `Flags:nil` and compensated by passing `Provider/Model/Timeout/Verbose/VerboseOn` back through `Options`. **Fields NOT compensated:**
- `ConfigPathOverride` (no `Options` field exists) → **`--config` is dropped** (Issue 1).
- `NoColor` (no `Options` field; `Config.NoColor` has `toml:"-"` so it is never serialized, and `pkg/stagehand` does not read `cfg.NoColor` — grep confirms zero references — so this gap is currently inert).
- `AutoStageAll`, `MaxDiffBytes`, `MaxMdLines`, `MaxDuplicateRetries`, `SubjectTargetChars`, `Output`, `StripCodeFence`, `Providers` → these are NOT re-applied via `Options`. They are re-derived from a fresh `Load`, so they are correct **only when** that second `Load` produces the same file/env/git-config layers as the first — i.e. when `--config` is absent. With `--config /custom.toml`, the second `Load` re-discovers the global file instead of using `/custom.toml`, so any values that came from the `--config` file (Providers overrides, generation knobs, etc.) are silently **lost** inside `GenerateCommit`.

The CLI-loaded `cfg` (returned by `Config()`) is used **only** for: provider pre-validation (`default_action.go:137-143`), the progress label (`:145-152`), and the `Options` fields above. Everything else (`buildDeps`, prompt sizing, retry count, CAS, render) happens inside `GenerateCommit` against the **second** config.

---

## 3. `root.go`: parsing `--config` into `flagConfig` and whether it reaches `runDefault`

**File:** `internal/cmd/root.go:29-46`

```go
var (
	flagProvider string
	flagModel    string
	flagConfig   string // --config → LoadOpts.ConfigPathOverride (NOT a Config field)
	flagTimeout  string // STRING — config.Load reads via fs.GetString("timeout") (FINDING 7)
	flagVerbose  bool
	flagNoColor  bool
)

var (
	flagAll         bool
	flagNoAutoStage bool
	flagDryRun      bool
)
```

**Registration** (`root.go:88`): `pf.StringVar(&flagConfig, "config", "", "Path to a config file, overrides discovery (env STAGEHAND_CONFIG)")` — zero default, so `fs.Changed("config")` correctly reflects "user passed it".

**`PersistentPreRunE`** (`root.go:62-80`) — verbatim:

```go
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if shouldSkipConfigLoad(cmd) {
			return nil
		}
		repoDir, err := os.Getwd()
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: getwd: %w", err))
		}
		cfg, err := config.Load(cmd.Context(), config.LoadOpts{
			ConfigPathOverride: flagConfig,
			RepoDir:            repoDir,
			Flags:              cmd.Flags(),
		})
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err))
		}
		loadedCfg = cfg
		return nil
	},
```

**This is the path that WORKS.** `LoadOpts.ConfigPathOverride: flagConfig` is passed in here (the FIRST Load), so `--config` is honored for the CLI's own resolved `cfg`. `Flags: cmd.Flags()` enables Layer-7 flag overlay.

**Propagation to `runDefault`:** `flagConfig` itself is NOT propagated. `runDefault` does not read `flagConfig` or `flagProvider`/`flagModel`/`flagTimeout` — it reads `cfg := Config()` (`default_action.go:56`), which is the package-level `loadedCfg` set by `PersistentPreRunE`. The `Options` struct handed to `GenerateCommit` carries the resolved **values** (`cfg.Provider`, etc.), not the raw flags. As noted in §2, `flagConfig` has nowhere to go — `Options` has no field for it.

`Config()` accessor (`root.go:108`): `func Config() *config.Config { return loadedCfg }`.

**`shouldSkipConfigLoad`** (`root.go:95-99`) returns true only for subcommands named `init` / `path` — so for the default root action, `PersistentPreRunE` always runs and `loadedCfg` is always set.

---

## 4. The exact body of `resolveConfig` — the bug locus

**File:** `pkg/stagehand/stagehand.go:108-141` (verbatim)

```go
// resolveConfig loads the full 7-layer config and applies Options overrides (highest precedence).
func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("getwd: %w", err)
	}

	cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
	if err != nil {
		return config.Config{}, "", fmt.Errorf("load config: %w", err)
	}

	cfg := *cfgPtr // copy to value

	// Apply caller overrides (highest precedence — explicit intent wins over file/env/git-config).
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		cfg.Model = opts.Model
	}
	if opts.Timeout != 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.VerboseOn {
		cfg.Verbose = true
	}

	return cfg, repoDir, nil
}
```

**The smoking gun is on the `config.Load` line:**

```go
cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
```

- **`ConfigPathOverride:` is omitted** → defaults to `""` → `config.Load` falls back to `STAGEHAND_CONFIG` env, then `globalConfigPath()` discovery. The `--config` value the user passed on the CLI is **never seen** by this Load. **(Issue 1.)**
- **`Flags: nil`** → `config.Load`'s Layer-7 (`loadFlags`) is **skipped** (`load.go:75-77`). Provider/model/timeout/verbose/no-color flags are NOT re-applied here; they survive only because `runDefault` re-injects Provider/Model/Timeout/Verbose via `Options` (§2).
- **A second `config.Load` is performed at all** → Layer-3 `loadRepoLocalConfig()` runs a second time → the §19 notice prints a second time. **(Issue 5.)**

This is the **only** `config.Load` call site inside `pkg/stagehand`. The seam is single-point: fixing this one call fixes both bugs.

`resolveConfig`'s override block re-applies `Provider/Model/Timeout/Verbose` from `opts` (highest precedence) — this is the "Options-as-flag-relay" pattern the comment in §2 refers to. It is a workaround for `Flags: nil`; it does not address `ConfigPathOverride` or the duplicate notice.

---

## 5. `LoadOpts` struct definition (all fields)

**File:** `internal/config/load.go:14-20` (verbatim)

```go
type LoadOpts struct {
	ConfigPathOverride string         // from --config (CLI); "" => fall back to STAGEHAND_CONFIG, then discovery
	RepoDir            string         // repo root for git config (passed to loadGitConfig); "" is valid for tests
	Flags              *pflag.FlagSet // cobra/pflag set; nil => skip the CLI-flag layer
}
```

Three fields. `ConfigPathOverride` is the one dropped by `resolveConfig`. The comment on `ConfigPathOverride` even says "from --config (CLI)" — it was designed for exactly this seam; `resolveConfig` simply doesn't populate it.

---

## 6. Where `loadRepoLocalConfig` is called and why the §19 notice prints twice

**`loadRepoLocalConfig` definition** — `internal/config/file.go:223-235` (verbatim):

```go
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
```

Notice text — `file.go:237-245`:

```go
func repoProviderNotice(cfg *Config) string {
	if cfg == nil || cfg.Provider == "" {
		return ""
	}
	return fmt.Sprintf("stagehand: repo-local config (.stagehand.toml) sets provider to %q\n", cfg.Provider)
}
```

`noticeOut` — `file.go:48-50`: `var noticeOut io.Writer = os.Stderr` (package-level, swappable for tests).

**Sole call site from `Load`:** `internal/config/load.go:57-61`:

```go
	// Layer 3: repo-local TOML (CWD .stagehand.toml; emits the §19 notice). nil => absent.
	if r, err := loadRepoLocalConfig(); err != nil {
		return nil, fmt.Errorf("repo config: %w", err)
	} else if r != nil {
		overlay(&cfg, r)
	}
```

**The double-print:** `config.Load` is invoked **twice per CLI commit**:

1. **First** — `root.go` `PersistentPreRunE` (§3) → `loadRepoLocalConfig()` → notice printed (1st).
2. **Second** — `default_action.go` `runDefault` → `stagehand.GenerateCommit` → `resolveConfig` (`stagehand.go:114`) → `config.Load` → `loadRepoLocalConfig()` → notice printed (2nd).

The notice is fired by `loadRepoLocalConfig` itself (side-effect I/O at load time), and `resolveConfig` cannot suppress it because it has no way to tell `Load` "skip the notice, the CLI already printed it." The two `LoadOpts` instances are independent — there is no cache, no "already-loaded" guard, no flag in `LoadOpts` to mute the notice.

**Note:** the providers command path (`providers.go` `runProvidersList`/`runProvidersShow`) does NOT double-print because it never calls `GenerateCommit` — it reads `Config()` directly (the value already loaded by `PersistentPreRunE`). This is the "path that WORKS" referenced in the task: a single `Load` via `PersistentPreRunE`, no `pkg/stagehand` involvement.

---

## 7. Verbatim signatures and types for an implementer

```go
// pkg/stagehand/stagehand.go
func GenerateCommit(ctx context.Context, opts Options) (Result, error)

type Options struct {
	Provider    string
	Model       string
	SystemExtra string
	DryRun      bool
	Timeout     time.Duration
	Verbose     io.Writer
	VerboseOn   bool
}

type Result struct {
	CommitSHA string
	Subject   string
	Message   string
	Provider  string
	Model     string
}

func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error)
// internal: calls config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
```

```go
// internal/config/load.go
func Load(ctx context.Context, opts LoadOpts) (*Config, error)

type LoadOpts struct {
	ConfigPathOverride string
	RepoDir            string
	Flags              *pflag.FlagSet
}
```

```go
// internal/config/config.go
type Config struct {
	Provider            string                    `toml:"provider"`
	Model               string                    `toml:"model"`
	Timeout             time.Duration             `toml:"timeout"`
	AutoStageAll        bool                      `toml:"auto_stage_all"`
	Verbose             bool                      `toml:"verbose"`
	NoColor             bool                      `toml:"-"`            // CLI/UI only; not in file
	MaxDiffBytes        int                       `toml:"max_diff_bytes"`
	MaxMdLines          int                       `toml:"max_md_lines"`
	MaxDuplicateRetries int                       `toml:"max_duplicate_retries"`
	SubjectTargetChars  int                       `toml:"subject_target_chars"`
	Output              string                    `toml:"output"`
	StripCodeFence      bool                      `toml:"strip_code_fence"`
	Providers           map[string]map[string]any `toml:"-"`
}

func Defaults() Config
```

```go
// internal/cmd/root.go
var (
	flagConfig  string  // --config value
	loadedCfg   *config.Config
)
func Config() *config.Config
// PersistentPreRunE calls: config.Load(ctx, config.LoadOpts{
//     ConfigPathOverride: flagConfig, RepoDir: repoDir, Flags: cmd.Flags(),
// })
```

```go
// internal/config/file.go
var noticeOut io.Writer = os.Stderr
func loadRepoLocalConfig() (*Config, error)   // prints §19 notice as a side effect
func repoProviderNotice(cfg *Config) string    // pure; returns "" if cfg==nil or Provider==""
```

---

## 8. Existing tests covering this seam

### `pkg/stagehand/stagehand_test.go`
- **No test exercises the `--config` / `ConfigPathOverride` path.** All tests build config via a **repo-local `.stagehand.toml`** (the Layer-3 file) and pass `Options{Provider: "stub", …}` — they never set `LoadOpts.ConfigPathOverride` because `Options` has no such field.
- `setupTestRepo` (`stagehand_test.go:74-117`) writes `.stagehand.toml` to a temp repo and `chdir`s into it; `GenerateCommit` then discovers config via `os.Getwd()`. This sidesteps `--config` entirely.
- Covered cases: `TestGenerateCommit_Success`, `_DryRun`, `_NothingStaged`, `_ProviderOverride`, `_Timeout` (dryrun + commit_path subtests), `_SystemExtra`.
- **Implication:** the `--config`-dropping bug is **invisible** to the current test suite — there is no assertion that `GenerateCommit` honors a custom config path.

### `internal/cmd/default_action_test.go`
- Drives the **full seam** via `Execute(context.Background())` with `rootCmd.SetArgs(...)`, so `PersistentPreRunE` → `runDefault` → `GenerateCommit` → `resolveConfig` all run.
- `setupStubRepo` (`:106-128`) and `setupStubRepoRaw` (`:160-170`) write `.stagehand.toml`; **none pass `--config`**. Every test sets `--provider stub` (re-applying the provider through `Options`, which masks the bug).
- Covered cases: `TestRunDefault_Commit`, `_RootCommit`, `_DryRun`, `_NothingStaged_FR17`, `_NoAutoStage_FR19`, `_AllFlag`, `_AutoStageNotice_FR18`, `_Rescue`, `_Timeout`, `_CAS`, `_VerboseFlag`, `_VerboseEnv`.
- **No test asserts on the §19 notice text** (`"repo-local config (.stagehand.toml) sets provider to"`) in stderr. The double-print would not be caught.
- **No test passes `--config /custom/path`** through the CLI.

### Other relevant tests
- `internal/config/file_test.go:261, 280, 302` — unit-tests `loadRepoLocalConfig` directly (including the notice via a swappable `noticeOut` buffer). These prove the notice fires once per `Load` call; they do not cover the cross-package double-call.
- `internal/config/load_test.go:568-573` — `Load` propagates a "repo config" wrapped error for bad TOML; relevant if an implementer changes the Layer-3 call site.

---

## 9. Data-flow diagram

```
                       CLI invocation: stagehand [--config X.toml] [--provider P] ...
                                              │
 ┌────────────────────────────────────────────┴──────────────────────────────────────────┐
 │  root.go PersistentPreRunE                                                              │
 │    config.Load(ctx, LoadOpts{ConfigPathOverride: flagConfig, RepoDir, Flags: cmd.Flags()})│
 │      └─ Layer 3: loadRepoLocalConfig() → §19 notice #1 (to os.Stderr)                  │
 │    loadedCfg = cfg                                                                      │
 └────────────────────────────────────────────┬──────────────────────────────────────────┘
                                              │  Config() → *Config (correct, --config honored)
 ┌────────────────────────────────────────────┴──────────────────────────────────────────┐
 │  default_action.go runDefault                                                           │
 │    cfg := Config()  // has Provider/Model/Timeout from flags (Layer 7 applied above)    │
 │    stagehand.GenerateCommit(ctx, Options{Provider: cfg.Provider, Model: cfg.Model,      │
 │                                          Timeout: cfg.Timeout, DryRun, Verbose, VerboseOn})│
 └────────────────────────────────────────────┬──────────────────────────────────────────┘
                                              │
 ┌────────────────────────────────────────────┴──────────────────────────────────────────┐
 │  pkg/stagehand.GenerateCommit → resolveConfig                                          │
 │    config.Load(ctx, LoadOpts{RepoDir: repoDir, Flags: nil})   ← BUG                     │
 │      · ConfigPathOverride omitted → "" → --config X.toml IGNORED (re-discovers global)  │
 │      · Flags: nil → Layer 7 skipped (provider/model survive via Options; others re-Ld)  │
 │      └─ Layer 3: loadRepoLocalConfig() → §19 notice #2 (to os.Stderr)  ← BUG           │
 │    apply opts.Provider/Model/Timeout/Verbose overrides                                   │
 └─────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Fix surfaces (for the parent orchestrator — not implemented here)

Two independent, composable options. Either fixes Issue 5; both are needed to fully fix Issue 1.

**Option A — forward config through `Options` (additive, API-stable):**
- Add `ConfigPathOverride string` to `Options` (`pkg/stagehand/stagehand.go`).
- In `resolveConfig`, populate `config.LoadOpts.ConfigPathOverride` from `opts.ConfigPathOverride`.
- In `runDefault`, set `Options.ConfigPathOverride: flagConfig` (the CLI package-level var).
- This does NOT fix the duplicate `Load` / duplicate notice on its own.

**Option B — accept a pre-resolved config (eliminates the second Load):**
- Add `Config *config.Config` (or `ResolvedConfig`) to `Options`; when non-nil, `resolveConfig`/`GenerateCommit` skips `config.Load` entirely and uses the caller's value (still applying `Options` overrides).
- `runDefault` passes `Config: cfg` (the `Config()` value) — one Load total, no double notice, `--config` honored because the first Load already used it.
- **Caveat:** `pkg/stagehand` importing `internal/config` is already the case (it does so today), so no new import cycle. But this couples the public API to an `internal/` type — review whether that's acceptable for a "Stable as of v1.0" surface, or whether a stable interface/struct should be introduced.

**Notice-suppression fallback (if a second Load must remain):**
- Add a `MuteRepoNotice bool` (or similar) to `LoadOpts`; thread it through `loadRepoLocalConfig` → skip `fmt.Fprint(noticeOut, …)` when set. `resolveConfig` would set it. This is a band-aid; Option B is cleaner.

**Risk:** the comment at `default_action.go:130-134` explicitly documents the current "Options-as-flag-relay" workaround. Any fix must preserve the precedence contract: `Options` overrides > Layer-7 flags > env > git-config > repo-local > global > defaults. Both options preserve this.

---

## Start Here

**`pkg/stagehand/stagehand.go:108-141` (`resolveConfig`)** — the single-line `config.Load` call is the root cause of both Issue 1 (dropped `ConfigPathOverride`) and Issue 5 (duplicate Layer-3 notice). Every other file is upstream (CLI flag parsing, `PersistentPreRunE`) or downstream (the notice printer, the `Load` resolver). Fixing this one call — by letting the caller supply either a `ConfigPathOverride` (Option A) or a fully-resolved `*Config` (Option B) — closes the seam.
