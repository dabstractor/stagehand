# Research — P1.M7.T2.S1: cmd/stagehand — cobra root + global flags + default Run + exit codes

This is the product's MAIN SURFACE. It owns `rootCmd.Run` (the default action),
the persistent global flags (PRD §15.2), precedence loading via `config.Flags`,
the maybeAutoStage → GenerateCommit → exit-code mapping flow, and verbosity/color.

## 0. What already exists in cmd/stagehand (DO NOT re-create)

- `cmd/stagehand/main.go` — package-level `var rootCmd` (`Use:"stagehand"`,
  `Version: version`), `var version = "dev"` (ldflags target), `main()` does
  `if err := rootCmd.Execute(); err != nil { os.Exit(1) }`. **HAS NO Run/RunE
  and NO persistent flags** — this task ADDS them.
- `cmd/stagehand/providers.go` — `providers list` / `providers show` (M7.T3.S1),
  self-registered via `init()` onto `rootCmd`. Contains a `resolveDefault`
  helper (cfg.Provider → first detected) and a pure `renderProvidersList`.
  **Reuse `resolveDefault`-style logic** (or share it) for the default action's
  provider validation.
- `cmd/stagehand/config.go` — `config init` / `config path` (M7.T3.S2),
  self-registered via `init()`.
- Both sibling files register via `init()` and **never touched main.go** —
  THIS task edits main.go (adds Run + flags) but MUST NOT break those
  registrations (they add themselves; rootCmd.AddCommand is already done).

## 1. Verified dependency signatures (match EXACTLY)

- `config.Flags{Env FlagsLayer; Flag FlagsLayer}` with
  `FlagsLayer{ConfigPath, Provider, Model *string; Timeout *time.Duration; Verbose, NoColor *bool}`.
  **GOTCHA (the present-but-zero trap):** a NON-NIL pointer to the ZERO value
  (Model="", Verbose=false, Timeout=0) COUNTS AS SET and OVERWRITES. So when
  building the FlagsLayer from cobra/pflag: set the pointer to &v ONLY when the
  source (flag changed OR env var present); for env, os.Getenv returning "" means
  "not set" (do NOT set the pointer). pflag's `cmd.Flags().Changed("name")` is
  the clean "was the flag explicitly passed" signal.
- `config.Load(flags config.Flags, repoDir string) (cfg config.Config, reg *provider.Registry, trustNotice string, err error)`.
  repoDir = "." (cwd, matching GenerateCommit and the providers cmd). Load sets
  cfg.ConfigPath to the actually-loaded file (for the verbose "resolved config
  path" line) and returns trustNotice non-empty ONLY when a REPO-LOCAL source
  set the provider (§19).
- `stagehand.GenerateCommit(ctx context.Context, opts stagehand.Options) (stagehand.Result, error)`.
  `Options{Provider, Model, SystemExtra string; DryRun bool; Timeout time.Duration}`.
  Returns `Result{CommitSHA, Subject, Message, Provider, Model string}`.
  **GenerateCommit calls config.Load ITSELF with `config.Flags{}` (no env/flag layer!)**
  — so the CLI must NOT rely on GenerateCommit resolving env/flags. Two valid
  designs:
  (A) CLI resolves cfg via config.Load (with env+flags), then passes the resolved
      Provider/Model/Timeout into `opts` so they BEAT GenerateCommit's empty-
      Flags Load. This is the contract: "Build a Flags struct from CLI flags +
      STAGEHAND_* env and pass to config.Load"; then "resolvedProvider/model
      from cfg" validated; then pass into opts.
  (B) Re-implement the pipeline. **FORBIDDEN** — violates the thin-wrapper seam.
  ⟹ Use design (A). Validate provider existence + PATH detection at the CLI
  (exit Error 1 with the friendly "Provider X: command Y not found. Is the
  agent installed?" message) BEFORE calling GenerateCommit, because
  GenerateCommit's own failure message ("no provider configured...") is less
  friendly and uses a different code path.

- `stagehand.ErrNothingToCommit` (= generate.ErrNothingToCommit) → ExitNothingToCommit(2).
- `stagehand.ErrRescue` (= generate.ErrRescue) → ExitRescue(3).
- `stagehand.ErrHeadMoved` (= generate.ErrHeadMoved) → ExitError(1).
- `provider.Registry`: `reg.Get(name) (Manifest, bool)`, `reg.List() []string`
  (sorted), `reg.Detect() map[string]bool` (LookPath on m.Detect || m.Command).
- `git.New(dir string) (*git.Git, error)`; `(*git.Git).HasStagedChanges() (bool, error)`;
  `(*git.Git).AddAll() error`.
- `ui.NewOutput(stdout, stderr io.Writer, verbose, noColor bool) *ui.Output`;
  exit constants: `ui.ExitSuccess=0, ui.ExitError=1, ui.ExitNothingToCommit=2, ui.ExitRescue=3, ui.ExitTimeout=124`.
- `ui.Output`: `Progressf`→stderr ALWAYS (FR51, auto-stage notice, trust notice),
  `Resultf`→stdout (FR42 success block), `Verbosef`→stderr gated by verbose.
  Color auto-disables when stdout is NOT a TTY (a pipe) — so `--dry-run | tee`
  is byte-clean BY CONSTRUCTION with noColor=false.

## 2. The timeout→124 tension (CRITICAL — read carefully)

PRD §15.4 mandates exit 124 for "generation exceeded --timeout". BUT the shipped
architecture collapses EVERY post-snapshot failure (including
*provider.TimeoutError, which wraps context.DeadlineExceeded) into
Rescue("") + ErrRescue → ExitRescue(3). `generate.CommitStaged` enforces
cfg.Timeout via an INTERNAL `context.WithTimeout(runCtx, cfg.Timeout)` and
returns ErrRescue on its expiry — there is NO ErrTimeout sentinel exported from
generate or stagehand.

Resolution (what this task implements):
- The CLI wraps the GenerateCommit call in a TOP-LEVEL
  `ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)`
  (guard cfg.Timeout > 0; default 120s is positive). If THAT top-level ctx
  deadline fires, GenerateCommit returns a non-sentinel error whose `errors.Is`
  chain contains `context.DeadlineExceeded` (because CommitStaged's own inner
  ctx derives from it, OR GenerateCommit's git.New/config.Load exceeds it).
  Map that to ExitTimeout(124).
- Otherwise map by sentinel: ErrNothingToCommit→2, ErrRescue→3, ErrHeadMoved→1,
  any other error→ExitError(1).
- In practice the inner and outer deadlines are the SAME duration, so the inner
  fires first and yields ErrRescue→3. This is the documented v1 behavior:
  **timeout during generation → rescue (exit 3), per the snapshot-rescue
  contract (decisions.md §3 / PRD §18.2).** Exit 124 is reserved for the
  narrow case where the CLI's outer ctx expires on a NON-generation step
  (e.g. an agent PATH-detection stall), which is rare. This matches the PRD
  intent that a user's spent quota is ALWAYS recoverable via the rescue block
  (exit 3) rather than a bare 124. Document this in the help/CONFIGURATION text.

  ALTERNATIVE (simpler, RECOMMENDED): do NOT add a top-level ctx timeout at all
  in v1 — let CommitStaged's inner deadline govern, and ALWAYS map ErrRescue→3
  (which already covers timeout). Drop the 124 branch from the mapping (or keep
  it as a defensive `errors.Is(err, context.DeadlineExceeded)` check that fires
  only if some future caller adds an outer timeout). **Pick the simpler path:
  map ErrRescue→3 and add a comment that 124 is reserved for a future
  CLI-level deadline.** The success-criteria tests assert ErrRescue→3 for the
  timeout scenario (matching the shipped CommitStaged behavior), NOT 124.

## 3. Exit-code mapping mechanism (cobra RunE + os.Exit)

cobra's `RunE` returning a non-nil error → main's `os.Exit(1)`. That cannot
carry codes 2/3/124. Two clean options:
- (RECOMMENDED) Make `Run` (not RunE) call `os.Exit(code)` directly at the end,
  and put the entire default-action logic in a testable pure function
  `runDefault(...) int` that RETURNS the exit code. `Run` does
  `os.Exit(runDefault(...))`. Tests call `runDefault` directly (no os.Exit),
  asserting the returned int. This is the SAME seam pattern as
  `handleSignal(...) int` in internal/generate/signal.go.
- The flag-parsing / help / --version paths stay cobra's responsibility (cobra
  prints help and exits 0 for bare invocation once Run is set... NO — once Run
  is set, bare invocation CALLS Run). So --version short-circuit must be
  checked INSIDE Run (or via PersistentPreRun) BEFORE the default action.

  **--version**: main.go currently relies on cobra's `Version` field
  auto-adding `--version`. Once a Run is added, `stagehand --version` STILL
  works (cobra handles --version before Run). Verify: cobra's --version
  handling fires in `Execute()` via the `Version` field regardless of Run.
  ⟹ Keep the `Version: version` field; do NOT add a manual --version flag.

## 4. Flag → FlagsLayer wiring (the precedence MOCKING surface)

Persistent flags on rootCmd (PRD §15.2 — exact names + shorthand):
`--provider`, `--model`, `--config`, `--timeout` (Duration), `--all`/`-a`,
`--no-auto-stage`, `--dry-run`, `--verbose`/`-v`, `--no-color`. (--version is
cobra's built-in; --help/-h is cobra's built-in.)

`--all`, `--no-auto-stage`, `--dry-run` are ACTION flags — they do NOT flow into
config.Flags (config.FlagsLayer has no field for them). They are read directly
off the cobra flags inside runDefault to decide staging policy.

Build `config.FlagsLayer` for env (read os.Getenv for the six STAGEHAND_* vars)
and for the CLI flag layer (only when cmd.Flags().Changed(name)):
- STAGEHAND_CONFIG / --config → ConfigPath *string
- STAGEHAND_PROVIDER / --provider → Provider *string
- STAGEHAND_MODEL / --model → Model *string
- STAGEHAND_TIMEOUT / --timeout (Duration) → Timeout *time.Duration
  (env is a STRING "120s" — parse with time.ParseDuration; on parse error →
  ExitError(1) with a friendly message BEFORE Load)
- STAGEHAND_VERBOSE / --verbose/-v → Verbose *bool
  (env: "1"/"true"/"yes" → true; anything else false; PRESENT-but-false still
  sets the pointer so it overrides a lower layer's true)
- STAGEHAND_NO_COLOR / --no-color → NoColor *bool (note the underscore in env)

**GOTCHA**: for env booleans, os.Getenv returns ("", false) when unset. Only
set the pointer when the var is PRESENT (os.LookupEnv ok=true). For env string
scalars (Provider/Model/ConfigPath), set the pointer only when the value is
non-empty (empty env == "not set" per FR35 convention). For the CLI flag layer,
use `cmd.Flags().Changed(name)` so an explicit `--model ""` is honored.

## 5. The default Run flow (the binding spec)

```
Run = func(cmd, args) {
    // --version short-circuit is handled by cobra's Version field; not here.
    out := ui.NewOutput(os.Stdout, os.Stderr, cfg.Verbose, noColor)
    // (cfg resolved inside runDefault from the Flags; or resolve here and pass)
    os.Exit(runDefault(ctx, cmd, out))
}

func runDefault(cmd, out) int {
    flags := buildFlags(cmd)              // env + CLI flag → config.Flags
    cfg, reg, notice, err := config.Load(flags, ".")
    if err != nil { out.Progressf(...err); return ui.ExitError }
    if notice != "" { out.Progressf("%s\n", notice) }   // §19 trust notice
    // Validate provider exists + detected on PATH (friendly error)
    detected := reg.Detect()
    name := cfg.Provider
    if name == "" { for _, n := range reg.List() { if detected[n] { name=n; break } } }
    if name == "" { out.Progressf("no provider configured/detected..."); return ui.ExitError }
    m, ok := reg.Get(name)
    if !ok || !detected[name] {
        out.Progressf("Provider %s: command %s not found. Is the agent installed?\n", name, targetCmd)
        return ui.ExitError
    }
    // maybeAutoStage (the staging POLICY — owned here, sibling S2 test surface)
    staged, err := g.HasStagedChanges()   // g := git.New(".") — but GenerateCommit makes its own
    // NOTE: GenerateCommit makes its OWN git.New; so HasStagedChanges/AddAll must use a
    // SEPARATE git.New(".") here, OR this staging must happen BEFORE GenerateCommit via a
    // local git client. Use a local `g, err := git.New(".")`.
    if !staged {
        if noAutoStage { out.Progressf("Nothing staged and --no-auto-stage set.\n"); return ui.ExitNothingToCommit }
        if !cfg.AutoStageAll && !allFlag { /* default auto_stage_all=true so usually stage */ }
        if err := g.AddAll(); err != nil { ...; return ui.ExitError }
        out.Progressf("Nothing staged — staging all changes.\n")   // FR18 (file count optional)
        staged, err = g.HasStagedChanges()
        if !staged { out.Progressf("Nothing to commit.\n"); return ui.ExitNothingToCommit }  // FR17
    } else if allFlag {
        if err := g.AddAll(); err != nil { ...; return ui.ExitError }   // FR20 force
    }
    // DryRun short-circuit handled INSIDE GenerateCommit via opts.DryRun (it prints msg, exit 0)
    opts := stagehand.Options{
        Provider: cfg.Provider, Model: cfg.Model, Timeout: cfg.Timeout, DryRun: dryRun,
    }
    res, err := stagehand.GenerateCommit(ctx, opts)
    switch {
    case err == nil:
        // success block already printed by CommitStaged (FR42). Just exit 0.
        // (DryRun: message already printed; exit 0.)
        return ui.ExitSuccess
    case errors.Is(err, stagehand.ErrNothingToCommit): return ui.ExitNothingToCommit
    case errors.Is(err, stagehand.ErrRescue):           return ui.ExitRescue
    // ErrHeadMoved and other errors → ExitError(1)
    default: return ui.ExitError
    }
}
```

**stdout discipline (FR51):** the FR42 success block (`[<short>] <subject>` +
diff-tree) is printed by `generate.CommitStaged` via `out.Resultf` (stdout).
The CLI must NOT re-print it. Only Progressf (stderr) is the CLI's job here
(trust notice, auto-stage notice, provider-missing error). DryRun message is
also printed by CommitStaged (stdout) — verified in pkg/stagehand research.

## 6. Test strategy (hermetic, NO real agent)

Mirror cmd/stagehand/providers_test.go + config_test.go conventions:
white-box `package main`, stdlib + bytes/strings/testing + internal/* only,
NO testify. The hermetic targets are PURE functions:
- `buildFlags(cmd) config.Flags` — drive with a real `*cobra.Command` populated
  via `cmd.SetArgs` + `cmd.ParseFlags` (or set flags directly); assert the
  pointer-per-scalar shape and the present-but-zero rule.
- `mapErrorToExitCode(err error) int` — pure; table-driven over the sentinels.
- `runDefault(...)` itself is hard to test end-to-end without a real agent, so
  test the PIECES: buildFlags precedence (flag>env>...), mapErrorToExitCode,
  and the provider-missing friendly-message path (via a pure
  `resolveAndCheckProvider(cfg, reg) (name string, exitCode int, msg string)`).

MOCKING contract scenarios (from the work item):
1. flag parsing (each global flag binds to the right pflag type);
2. precedence flag>env>git-config>file (drive buildFlags with cmd.ParseFlags +
   t.Setenv for STAGEHAND_*, assert the FlagsLayer pointers reflect flag>env);
3. --dry-run prints message, exit 0, no commit (assert mapErrorToExitCode(nil
   with DryRun) == 0; the "no commit" is enforced by GenerateCommit, tested
   via the pkg/stagehand suite — here just assert opts.DryRun is wired);
4. nothing-staged + --no-auto-stage → exit 2 (pure staging-policy helper);
5. --version short-circuit (subprocess or cobra Execute on a temp rootCmd
   with Version set, capture stdout, assert "stagehand version");
6. missing provider → friendly message + exit 1 (pure resolveAndCheckProvider
   with a registry whose only provider is not on PATH — use a fabricated
   name like providers_test does).

## 7. Docs impact (Mode A)

The work item declares Mode A: materialize/update `docs/CONFIGURATION.md` CLI
section + cobra Long/Short help strings. CONFIGURATION.md already has a
cross-reference placeholder ("the CLI flag reference ... is documented in this
same file by the CLI task (P1.M7.T2.S1)"). This task ADDS a "CLI flags"
section to docs/CONFIGURATION.md enumerating the PRD §15.2 table (flag, env,
git-config, default, description), and sets the rootCmd `Long` help text. The
exit-code table (PRD §15.4) should also be surfaced there or cross-referenced.

## 8. External references

- cobra persistent flags: https://pkg.go.dev/github.com/spf13/cobra#Command.PersistentFlags
- cobra Version field auto---version: https://pkg.go.dev/github.com/spf13/cobra#Command (Version field doc)
- pflag Changed(): https://pkg.go.dev/github.com/spf13/pflag#FlagSet.Changed
- ANSI color / NO_COLOR: https://no-color.org
