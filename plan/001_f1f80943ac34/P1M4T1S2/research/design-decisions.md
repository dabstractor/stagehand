# P1.M4.T1.S2 — Design Decisions

The default commit action: the root command's `RunE` body (auto-stage-all →
`stagecoach.GenerateCommit` → FR42 report), wired onto the S1 scaffold. S1 ships `internal/cmd/root.go`
(stub RunE = `cmd.Help()`); S2 swaps in the real action and ships its integration tests.

All signatures below are quoted VERBATIM from the on-disk contracts (P1.M3.T5.S1 `pkg/stagecoach`,
P1.M3.T4.S2 `internal/generate`, P1.M1.T3 `internal/git`, P1.M4.T1.S1 `internal/cmd` + `internal/exitcode`).

---

## §0 — S2 calls `pkg/stagecoach.GenerateCommit` (the public API), NOT `generate.CommitStaged`

The task says "call GenerateCommit (with DryRun if --dry-run)". `GenerateCommit` is the PUBLIC API
(`pkg/stagecoach`, PRD §14.1, US12: "a stable Go API so I can embed commit-message generation"). The CLI
dogfoods its own public surface — the correct layering. Calling `generate.CommitStaged` directly would
(a) force the CLI to reimplement provider resolution (`buildDeps`: registry + auto-detect + Validate)
and (b) duplicate `pkg/stagecoach` logic. So: `stagecoach.GenerateCommit(ctx, stagecoach.Options{...})`.

CONSEQUENCE (§1 + §2): the public `Result` drops `Changes`, and `GenerateCommit` re-loads config. Both
are accepted and handled in the CLI layer (not by modifying the frozen `pkg/stagecoach`).

## §1 — The CLI computes FR42's DiffTree itself (`stagecoach.Result` has no `Changes`)

`pkg/stagecoach.Result = {CommitSHA, Subject, Message, Provider, Model}` — NO `Changes` (P1.M3.T5.S1
design §1: "the file listing is a CLI/report concern, not a library concern"; `generate.Result` HAS
`Changes []git.FileChange` but the public mapping drops it). FR42 requires the file list:

```
[abc1234] feat(auth): accept SAML tokens for enterprise login   ← [<short-sha>] <subject>
M  src/login.go                                                   ← DiffTree name-status
A  src/login_test.go
```

So the CLI reproduces `generate.CommitStaged`'s step 9 itself: capture `parentSHA, isUnborn` via
`git.New(repoDir).RevParseHEAD(ctx)` BEFORE `GenerateCommit` (auto-stage does not change isUnborn), then
after success `g.DiffTree(res.CommitSHA, isUnborn)`. The CLI already constructs a `git.Git` for the
auto-stage dance (HasStagedChanges/AddAll/StagedFileCount), so this adds one RevParseHEAD + one DiffTree.

MINOR RACE (accepted, cosmetic): the CLI's `isUnborn` is read once, then `GenerateCommit` reads it again
internally. If HEAD transitions unborn→born between the two reads (a concurrent commit), the report's
`--root` flag could be one attempt stale. This is the SAME class of best-effort window
`generate.CommitStaged` itself carries (step-1 RevParseHEAD → step-9 DiffTree, with the generate loop
between). It affects ONLY the post-commit file listing (never correctness of the commit, which the CAS
guards). DiffTree failure after a successful commit is non-fatal: print the report without the file list,
still return nil (exit 0) — the commit already landed.

## §2 — `GenerateCommit` re-loads config via DISCOVERY (no `--config` override) — tests use `.stagecoach.toml`

`pkg/stagecoach.resolveConfig` calls `config.Load(ctx, LoadOpts{RepoDir: os.Getwd(), Flags: nil})` — it
passes NO `ConfigPathOverride`. So `--config <file>` (which the CLI's `PersistentPreRunE` honors via
`flagConfig`) is NOT seen by `GenerateCommit`'s internal load. For the COMMON case (built-in providers
pi/claude/…, always available) this is invisible. It ONLY bites if a user defines a CUSTOM provider
manifest solely in a `--config` file (not in discovery): then the CLI sees it but `GenerateCommit`
doesn't → "unknown provider". That is a latent limitation of `pkg/stagecoach.GenerateCommit`'s API, NOT
S2's to fix (frozen contract). Documented; non-blocking for v1.

IMPLICATION FOR TESTS: the stub provider manifest must live where BOTH the CLI and `GenerateCommit` read
it = DISCOVERY = the repo-local `./.stagecoach.toml` (both use `os.Getwd()`; `repoLocalConfigPath()` is
`./.stagecoach.toml`). So the integration test writes `.stagecoach.toml` into the temp repo (NOT a
`--config` override). Global config is isolated via `loadEnvSetup` (HOME/XDG → temp), copied by S1 into
`root_test.go` (same package — S2 reuses it).

## §3 — The CLI passes `Options{Provider,Model,Timeout,DryRun}` from `cmd.Config()`

The CLI's `PersistentPreRunE` (S1) already resolved config WITH flags (Layer 7: `--provider`/`--model`/
`--timeout` via `fs.Changed`). `cmd.Config()` holds that. `GenerateCommit` re-loads with `Flags: nil`,
so to make the CLI flags take effect, S2 re-applies the resolved values as `Options` (opts override is
HIGHEST precedence in `resolveConfig`):

```go
cfg := Config() // resolved by PersistentPreRunE (incl. Layer-7 flags)
res, err := stagecoach.GenerateCommit(cmd.Context(), stagecoach.Options{
    Provider: cfg.Provider, // "" → auto-detect (preserved)
    Model:    cfg.Model,    // "" → manifest default_model (preserved)
    Timeout:  cfg.Timeout,  // 120s default from config.Defaults()
    DryRun:   flagDryRun,   // --dry-run (P1.M4.T4 refines output; S2 executes it)
})
```

`AutoStageAll` is a CONFIG field read directly by the CLI's auto-stage logic (`cfg.AutoStageAll`,
default true) — it is NOT an Options field (the public API doesn't stage; US12).

## §4 — Exit codes come from `exitcode.For` (S1); S2's RunE owns OUTPUT + returns the right error

S1 already centralized the §15.4 mapping in `exitcode.For(err)` (called once in `main`). S2 does NOT
re-map. S2's RunE:
- Prints the detailed, user-facing message for each outcome (success report, FR18 notice, rescue via
  `generate.FormatRescue`, CAS via `*generate.CASError.Error()`).
- Returns an error that `exitcode.For` maps to the right code.

THE DOUBLE-PRINT TRAP: `main` prints `stagecoach: <err>\n` when `err.Error() != ""` (S1). If the RunE
prints the FULL rescue block AND returns the original `*RescueError` (non-empty `.Error()`), main prints
a second summary line. RESOLUTION: when the RunE has already printed the detailed message, it returns a
SILENT `exitcode.New(<code>, nil)` — `ExitError.Error()` returns "" (Err is nil) → main's guard
(`err.Error() != ""`) skips printing. The exit code is still honored (`exitcode.For` returns the explicit
ExitError.Code). This gives exactly one user-facing message per failure.

```go
var re *generate.RescueError
if errors.As(err, &re) {                       // covers BOTH ErrTimeout and ErrRescue
    fmt.Fprintln(os.Stderr, generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
    code := exitcode.Rescue
    if errors.Is(err, generate.ErrTimeout) { code = exitcode.Timeout } // 124 vs 3
    return exitcode.New(code, nil)             // silent → main prints nothing; exit code honored
}
var ce *generate.CASError
if errors.As(err, &ce) {
    fmt.Fprintln(os.Stderr, ce.Error())        // the §13.5 "HEAD moved…" message
    return exitcode.New(exitcode.Error, nil)   // silent; exit 1
}
// friendly messages (FR17/FR19/ErrNothingToCommit): DO want main's "stagecoach: <msg>"
if errors.Is(err, generate.ErrNothingToCommit) {
    return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
}
return exitcode.New(exitcode.Error, err)       // generic: exit 1, main prints it
```

NOTE: timeout (124) vs rescue (3) is decided by `errors.Is(err, generate.ErrTimeout)` — a
`*RescueError{Kind:ErrTimeout}` unwraps to `ErrTimeout`, so this correctly yields 124 (mirrors
`exitcode.For`'s timeout-before-rescue ordering). The RunE computes the code explicitly here only to
pair it with the silent return; `exitcode.For` would derive the same code from the original error.

## §5 — Output streams: stdout = RESULT, stderr = NOTICES/DIAGNOSTICS

PRD §15.5 pipes dry-run output (`stagecoach --dry-run --no-color | tee /tmp/msg.txt`) → stdout must be the
MESSAGE ONLY for dry-run. Generalizing:
- **stdout**: the commit success report (FR42) and the dry-run message. (The payload a user pipes.)
- **stderr**: the FR18 auto-stage notice, the rescue block, the CAS message, and all error diagnostics.

P1.M4.T3 (UI/TTY/color/verbose) later refines HOW these are rendered; S2 establishes WHAT goes where.
Interactive terminals merge both, so the user sees everything; pipes capture only the result.

## §6 — File plan: ONE new action file + ONE test file + a one-line edit to `root.go`

S1 ships `internal/cmd/root.go` with `RunE: func(...) error { return cmd.Help() }` (stub) + the flag
vars (`flagAll`, `flagNoAutoStage`, `flagDryRun`) + `Config()` + `Version` + `Execute(ctx)`. S2:
1. **CREATE** `internal/cmd/default_action.go` (`package cmd`) — `runDefault(cmd, args) error` (the full
   flow) + small report helpers (`shortSHA`, `printCommitReport`, `printDryRun`). Imports `cobra`,
   `errors`, `fmt`, `os`, `github.com/dustin/stagecoach/{internal/exitcode, internal/generate, internal/git,
   pkg/stagecoach}`.
2. **EDIT** `internal/cmd/root.go` — replace the stub `RunE` body with `RunE: runDefault` (one line).
3. **CREATE** `internal/cmd/default_action_test.go` (`package cmd`) — integration tests driving the FULL
   CLI (`Execute(ctx)` / `rootCmd`) through a stub provider in a temp repo, asserting commits land +
   exit codes + output.

Why a separate file (not inline in root.go)? The action body is substantial (auto-stage FSM + report +
error matrix); isolating it keeps root.go as the SCAFFOLD (S1's concern) and the action as S2's. It also
gives P1.M4.T3 a single function to restyle.

## §7 — Test seam: stub provider via repo-local `.stagecoach.toml` + `t.Setenv(STAGECOACH_STUB_*)`

Confirmed: `provider.Render` builds `spec.Env = os.Environ() + manifest.Env` (render.go), and the
executor sets `cmd.Env = spec.Env` (non-empty) OR inherits parent env (empty). EITHER WAY the stub agent
inherits the test process's `STAGECOACH_STUB_*` vars. So:

1. `bin := stubtest.Build(t)` — compiles `cmd/stubagent` once.
2. `loadEnvSetup(t)` + `chdir(repo)` — isolate global config + CWD (S1 copied these into root_test.go).
3. Write `.stagecoach.toml` into the repo:
   ```toml
   [provider.stub]
   command = "<bin>"
   prompt_delivery = "stdin"
   output = "raw"
   strip_code_fence = true
   ```
   (`Validate` requires only Name+Command; the registry adds "stub" verbatim as a §12.8 provider since
   it's not a built-in.)
4. `t.Setenv("STAGECOACH_STUB_OUT", "feat: add login")` (or `STAGECOACH_STUB_SCRIPT`/`_EXIT`/`_SLEEP_MS`
   for rescue/timeout scenarios) — controls the stub per-test.
5. `rootCmd.SetArgs(["--provider", "stub"])`; capture stdout/stderr via `rootCmd.SetOut`/`SetErr`;
   `Execute(ctx)`.
6. Assert `exitcode.For(err)`, HEAD moved to a new SHA, `git log --format=%B -n1` == stub output, stdout
   contains the short-sha + subject.

HELPERS: S2's test (package `cmd`) REUSES root_test.go's `initRepo`/`setGitConfig`/`writeConfigFile`/
`chdir`/`loadEnvSetup` (same package — no copy). It COPIES the additional generate-style helpers from
`internal/generate/generate_test.go` (package-private there, unimportable): `writeFile`/`stageFile`/
`commitRaw`/`headSHA`/`runGit`/`gitOut` + `shaRe`. (~30 lines.) This mirrors exactly how S1 and P1.M3.T4
copied their own helper sets.

STATE HYGIENE: `rootCmd` is a package-level singleton (S1). Each test restores `SetArgs(nil)`,
`SetOut`/`SetErr` originals, and `loadedCfg = nil` in `t.Cleanup` (else tests poison each other + -race).

## §8 — Auto-stage FSM (FR16–FR20), precise

```
g := git.New(repoDir)
if flagAll { g.AddAll(ctx) }                         // FR20: force stage even if something staged
has, _ := g.HasStagedChanges(ctx)
if !has {
    switch {
    case flagNoAutoStage:                            // FR19
        return exitcode.New(NothingToCommit, errors.New("Nothing staged."))   // exit 2; main prints
    case cfg.AutoStageAll:                           // FR16/FR18
        g.AddAll(ctx); n, _ := g.StagedFileCount(ctx)
        fmt.Fprintf(os.Stderr, "Nothing staged — staging all changes (%d files).\n", n)  // FR18
        has, _ = g.HasStagedChanges(ctx)
        if !has { return exitcode.New(NothingToCommit, errors.New("Nothing to commit.")) } // FR17
    default:                                         // AutoStageAll off via config (no flag)
        return exitcode.New(NothingToCommit, errors.New("Nothing to commit."))              // exit 2
    }
}
// has == true → generate
```
`--no-auto-stage` (FR19) takes ABSOLUTE precedence over `cfg.AutoStageAll` when nothing is staged. The
config-`AutoStageAll=false`-without-flag case falls through to "Nothing to commit." (no separate FR; the
sensible non-auto-staging outcome). `flagAll`'s AddAll runs BEFORE the HasStagedChanges check (FR20), so
on a clean tree it yields the FR18(0 files)+FR17 sequence — an accepted, harmless edge.

## §9 — Dry-run output is the MESSAGE ONLY (stdout); decorations deferred to P1.M4.T3/T4

For `--dry-run` success, S2 prints `res.Message` to stdout and returns nil (exit 0). The Appendix B.3
decorations ("↳ Generating with…", "(no commit created)") are progress-message/UI concerns owned by
P1.M4.T3 (progress/TTY/color) and P1.M4.T4 (dry-run mode) — S2's contract is "call GenerateCommit with
DryRun" + emit the message. Keeping stdout = message-only preserves the §15.5 pipe use case verbatim.

## §10 — Report format is isolated for P1.M4.T3 to restyle

`printCommitReport(w, res, changes)` renders `[<short-sha>] <subject>` + the name-status file list.
`shortSHA(sha)` = first 7 hex chars (matches Appendix B's 7-char SHAs; `res.CommitSHA` is full 40).
The "↳ Created" progress decoration and color are P1.M4.T3; isolating the formatter in one function lets
P1.M4.T3 swap it without touching the flow. If `DiffTree` errors post-commit (§1), print the report
without the file list (the commit already succeeded — never fail on a report error).

## §11 — `runDefault` reads `cmd.Context()` (set by S1's `Execute`); signals are P1.M4.T2

S1's `Execute(ctx)` calls `rootCmd.SetContext(ctx)`, so `runDefault`'s `cmd.Context()` is the S1 baseline
(`context.Background()`). Every git op + `GenerateCommit` takes `cmd.Context()`. P1.M4.T2 later swaps the
ctx for `signal.NotifyContext` + child-kill/rescue — no change to S2's code (it reads `cmd.Context()`).
