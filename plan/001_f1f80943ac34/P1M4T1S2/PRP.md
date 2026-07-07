---
name: "P1.M4.T1.S2 — Default commit action (auto-stage-all + CommitStaged + success report) — PRD §9.4 / §9.9 / §15.1 / §15.4 / §15.5 / Appendix B"
description: |

  Implement the root command's default-action `RunE` (PRD §15.1: "With no command, runs the default
  action: commit staged changes, auto-staging all if nothing is staged"). It owns the auto-stage-all
  state machine (PRD §9.4 / FR16–FR20), delegates the actual generation+commit to the PUBLIC API
  `pkg/stagecoach.GenerateCommit` (dogfooding US12), and renders the FR42 success report (`[<short-sha>]
  <subject>` + DiffTree name-status file list, PRD §9.9 / Appendix B.1) plus the §15.4 error outcomes
  (rescue→3, timeout→124, CAS→1, nothing-to-commit→2). Exit-code mapping is ALREADY centralized in
  `internal/exitcode.For` (S1) + `main` (S1) — S2 produces the user-facing OUTPUT and returns the right
  error; it never calls `os.Exit`.

  This is the action body that hangs on S1's scaffold. S1 shipped `internal/cmd/root.go` with a STUB
  `RunE` (`return cmd.Help()`), the §15.2 flag vars (`flagAll`/`flagNoAutoStage`/`flagDryRun`), `Config()`,
  `Version`, and `Execute(ctx)`. S2 swaps the stub for `runDefault` (a one-line edit to root.go) and
  ships the action + its integration tests in two NEW files.

  DELIVERABLES (NEW files + one targeted edit; NOTHING under internal/{config,generate,git,prompt,
  provider} or pkg/stagecoach is touched — they are frozen READ-ONLY contracts):
    1. CREATE `internal/cmd/default_action.go`      — `runDefault(cmd, args) error` (the full flow) +
       `shortSHA` + `printCommitReport` + `printDryRunMessage` helpers.
    2. EDIT  `internal/cmd/root.go`                  — stub `RunE` body → `RunE: runDefault` (one line).
    3. CREATE `internal/cmd/default_action_test.go` — integration tests: stub provider through the FULL
       CLI (`Execute`/`rootCmd`) in temp git repos; asserts commits land + exit codes + stdout/stderr.

  CONTRACT (PRD §9.4 FR16–FR20, §9.9 FR42, §15.4 exit codes, §15.5 examples, Appendix B.1/B.3/B.5):
    - `--all`/`-a` (FR20): `git add -A` BEFORE the staged check, even if something is already staged.
    - Nothing staged + `cfg.AutoStageAll && !--no-auto-stage` (FR16): `git add -A`, print
      `Nothing staged — staging all changes (N files).` (FR18) to STDERR, re-check.
    - Still nothing after auto-stage (FR17): exit 2 `Nothing to commit.`
    - Nothing staged + `--no-auto-stage` (FR19): exit 2 `Nothing staged.` (--no-auto-stage wins over
      cfg.AutoStageAll).
    - Otherwise: `stagecoach.GenerateCommit(ctx, Options{Provider:cfg.Provider, Model:cfg.Model,
      Timeout:cfg.Timeout, DryRun:flagDryRun})`.
    - Success (commit): stdout `printCommitReport` = `[<7-char-sha>] <subject>` + DiffTree name-status
      file list (FR42). Dry-run success: stdout = `res.Message` ONLY (§15.5 pipe use case).
    - Errors: `*RescueError` (ErrTimeout→124 / ErrRescue→3) → `generate.FormatRescue` to STDERR + silent
      `exitcode.New(code,nil)`; `*CASError` → `ce.Error()` to STDERR + silent `exitcode.New(1,nil)`;
      `ErrNothingToCommit` / FR17 / FR19 → friendly message via `exitcode.New(2,err)` (main prints);
      anything else → `exitcode.New(1,err)`.

  SCOPE BOUNDARY (owned by siblings — do NOT implement): `providers list/show` + `config init/path`
  subcommands (S3/S4); signal handling SIGINT/SIGTERM→child-kill→rescue (P1.M4.T2 — it swaps the ctx S1
  wired; S2 just reads `cmd.Context()`); color/TTY/verbose UI + the "↳" progress decoration (P1.M4.T3 —
  S2 isolates the report formatter so P1.M4.T3 can restyle it); dry-run OUTPUT decorations like "↳
  Generating…" / "(no commit created)" (P1.M4.T4 — S2 prints the message only).

  INPUT (upstream — READ-ONLY contracts, all on disk):
    - `pkg/stagecoach.GenerateCommit(ctx, Options) (Result, error)` + `Options{Provider,Model,SystemExtra,
      DryRun,Timeout}` + `Result{CommitSHA,Subject,Message,Provider,Model}` (NO `Changes`!) + the
      re-exported errors `stagecoach.ErrNothingToCommit/ErrTimeout/ErrRescue/ErrCASFailed` +
      `stagecoach.RescueError`/`CASError` aliases (P1.M3.T5.S1).
    - `generate.FormatRescue(treeSHA, parentSHA, candidateMsg) string` (P1.M3.T3.S1 — §18.3 block, no
      trailing newline; the CLI's `fmt.Fprintln` adds it).
    - `git.New(repoDir) Git` + `Git.HasStagedChanges/AddAll/StagedFileCount/RevParseHEAD/DiffTree`
      (P1.M1.T2/T3) — `DiffTree(ctx, sha, isRoot)` needs isRoot; `RevParseHEAD` returns isUnborn.
    - `config.Config` fields `Provider/Model/Timeout/AutoStageAll` + `config.Defaults()` (P1.M1.T4).
    - S1's `internal/cmd/root.go`: `rootCmd`, flag vars `flagAll/flagNoAutoStage/flagDryRun`, `Config()
      *config.Config`, `Execute(ctx) error`, `Version`. S1's `internal/cmd/root_test.go` helpers (same
      package — REUSABLE): `initRepo/setGitConfig/writeConfigFile/chdir/loadEnvSetup`.
    - S1's `internal/exitcode`: `For(err) int`, `New(code,err) *ExitError`, `ExitError{Code;Err}`
      (`Error()==""` when `Err==nil` → `main` skips printing), constants `Success/Error/NothingToCommit/
      Rescue/Timeout`.

  OUTPUT (downstream consumers): the default action IS the binary's primary behavior — `make build`
  then `./bin/stagecoach` (inside a git repo) produces a commit (or dry-run message, rescue block, or a
  §15.4 exit code). P1.M4.T2 swaps `cmd.Context()`; P1.M4.T3 restyles `printCommitReport` + adds
  progress lines; P1.M4.T4 adds dry-run decorations; S3/S4 `rootCmd.AddCommand(...)` alongside.

  ⚠️ S2 calls `pkg/stagecoach.GenerateCommit` (the PUBLIC API), NOT `generate.CommitStaged`. The public
     `Result` DROPS `Changes` → the CLI computes FR42's DiffTree ITSELF (capture isUnborn before
     GenerateCommit; §1/§2 in research). (design §0/§1)
  ⚠️ `GenerateCommit` re-loads config via DISCOVERY (no `--config` override). Tests MUST put the stub
     provider in the repo-local `./.stagecoach.toml` (read by BOTH the CLI and GenerateCommit), NOT a
     `--config` file. (design §2/§7)
  ⚠️ Exit codes come from `exitcode.For` (S1). To avoid main DOUBLE-printing, when the RunE has already
     printed a detailed message (rescue/CAS) it returns a SILENT `exitcode.New(code, nil)` (Err==nil →
     Error()=="" → main skips). Friendly messages (FR17/FR19) DO pass a non-nil err so main prints them.
     (design §4)
  ⚠️ stdout = RESULT (commit report / dry-run message); stderr = NOTICES + DIAGNOSTICS (FR18 notice,
     rescue block, CAS message). Preserves the §15.5 `stagecoach --dry-run | tee` pipe use case. (design §5)

  Deliverable: 2 NEW files + 1 one-line edit. `make build` → `./bin/stagecoach` inside a git repo with
  staged changes produces a commit and prints the FR42 report; `--dry-run` prints the message; nothing-
  staged paths exit 2; rescue/timeout/CAS map to 3/124/1. `go test -race ./internal/cmd/` green; no
  regression in `go test -race ./...`.

---

## Goal

**Feature Goal**: Ship Stagecoach's default commit action (PRD §15.1) — the `RunE` body that turns the S1
scaffold into a working tool: it runs the §9.4 auto-stage-all state machine (FR16–FR20), delegates
generation+commit to the public `pkg/stagecoach.GenerateCommit`, renders the FR42 success report
(`[<short-sha>] <subject>` + DiffTree file list), and maps every §15.4 outcome (success 0, nothing-to-
commit 2, rescue 3, timeout 124, CAS/general 1) via the S1 `exitcode.For` centralization — producing
exactly one clear user-facing message per run and never calling `os.Exit`.

**Deliverable** (2 NEW files + 1 one-line edit; NO edits under internal/{config,generate,git,prompt,
provider} or pkg/stagecoach):
1. `internal/cmd/default_action.go` — `package cmd`. `runDefault(cmd *cobra.Command, args []string)
   error` (full flow: --all→AddAll; HasStagedChanges FSM FR16–FR20; capture isUnborn;
   `stagecoach.GenerateCommit`; error matrix; report). Helpers: `shortSHA(sha)`, `printCommitReport(w,
   res, changes)`, `printDryRunMessage(w, msg)`.
2. EDIT `internal/cmd/root.go` — change `RunE: func(cmd, args) error { return cmd.Help() }` →
   `RunE: runDefault`. (S1's stub becomes the real action.)
3. `internal/cmd/default_action_test.go` — `package cmd`. Integration tests driving the FULL CLI
   (`rootCmd`/`Execute`) through a stub provider in temp git repos: happy-path commit, root commit,
   dry-run, nothing-staged→FR17, --no-auto-stage→FR19, --all, auto-stage FR18 notice, rescue→3, timeout
   →124, CAS→1. Reuses root_test.go helpers + copies generate-style helpers.

**Success Definition**: `make build` → `./bin/stagecoach`; inside a git repo with staged changes,
`./bin/stagecoach` creates a commit and prints `[<7-char-sha>] <subject>` + the name-status file list
(FR42) to stdout, exit 0; `./bin/stagecoach --dry-run` prints the generated message to stdout, exit 0,
no commit; nothing staged (clean tree) → `Nothing to commit.` exit 2; `--no-auto-stage` + nothing staged
→ `Nothing staged.` exit 2; `-a` stages all then commits; rescue/timeout/CAS map to 3/124/1 with the
§18.3 / §13.5 messages on stderr. `go test -race ./internal/cmd/` green; `go test -race ./...` shows NO
regression; `go vet ./...` clean; `gofmt -l` empty; only the listed files changed.

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 "the plan-holder") who runs `stagecoach` at the terminal
to generate+commit, plus lazygit/CI integrators who invoke it non-interactively (PRD §15.5 lazygit +
pipe examples). For S2 specifically: a user for whom bare `stagecoach` Just Works — it stages when
nothing is staged (FR16/US11), generates via the default agent, commits atomically, and reports what
landed (FR42) — with deterministic §15.4 exit codes scripts can branch on.

**Use Case**: `stagecoach` (commit staged, auto-staging all if the tree is otherwise clean); `stagecoach
-a` (quick checkpoint: stage everything + commit); `stagecoach --dry-run` (preview the message); `stagecoach
--no-auto-stage` (fail fast if nothing staged instead of auto-staging).

**User Journey**: user runs `stagecoach [<flags>]` → S1 `PersistentPreRunE` resolves config → S2
`runDefault`: maybe AddAll (FR20), check staged, maybe auto-stage+notice (FR16/FR18), call
`GenerateCommit` → on success print `[sha] subject` + file list (FR42); on failure print the §18.3
rescue block or §13.5 CAS message; main maps the returned error to a §15.4 exit code.

**Pain Points Addressed**: (1) v1 single-shot commit (US1/US11) — `stagecoach` with no args produces a
commit using the default agent; (2) transparent auto-staging (FR18) — the user is TOLD when Stagecoach
staged on their behalf; (3) one-shot checkpoint (US11/FR20) — `-a` stages+commits in one go; (4)
deterministic outcomes — scripts/lazygit branch on 0/1/2/3/124 (§15.4); (5) safe failures — rescue
(US3) leaves the repo untouched and prints the exact recovery command.

## Why

- **It IS the product.** Every prior milestone (M1 git plumbing, M2 providers, M3 pipeline) is exercised
  end-to-end for the first time by this `RunE`. Without it the binary (S1) only prints help.
- **Dogfoods the public API (US12).** Calling `pkg/stagecoach.GenerateCommit` (not the internal
  orchestrator) means the CLI is its own first integration customer — the API a third party would embed
  is proven by the shipped tool. This is the strongest possible correctness signal for §14.1.
- **Closes the exit-code loop (PRD §15.4).** The pipeline returns typed errors; `exitcode.For` (S1)
  maps them. S2 is the first code path that FEEDS real pipeline errors (rescue/timeout/CAS/nothing) into
  that mapping at runtime, proving the whole §15.4 contract.
- **Honors the layering (PRD §11.3).** Auto-stage lives in the CLI layer, NOT the orchestrator
  (`CommitStaged`/`GenerateCommit` never call `git add`). S2 is precisely where that boundary is drawn.

## What

A `runDefault(cmd, args) error` that: reads `cmd.Context()` + `Config()`; constructs `git.New(repoDir)`;
runs the auto-stage FSM (FR16–FR20) using `flagAll`/`flagNoAutoStage`/`cfg.AutoStageAll`; on the
nothing-staged terminal states returns `exitcode.New(exitcode.NothingToCommit, …)` (exit 2); otherwise
captures `isUnborn` and calls `stagecoach.GenerateCommit(cmd.Context(), Options{Provider:cfg.Provider,
Model:cfg.Model, Timeout:cfg.Timeout, DryRun:flagDryRun})`; on success prints the FR42 report (commit)
or the message (dry-run) to stdout; on `*RescueError` prints `generate.FormatRescue` to stderr + returns
silent `exitcode.New(124|3, nil)`; on `*CASError` prints `ce.Error()` to stderr + returns silent
`exitcode.New(1, nil)`; on `ErrNothingToCommit`/generic returns `exitcode.New(1|2, err)` (main prints).

### Success Criteria

- [ ] `internal/cmd/default_action.go` exists, `package cmd`, imports `errors`+`fmt`+`io`+`os`+
      `github.com/spf13/cobra` + `github.com/dustin/stagecoach/internal/{exitcode,generate,git}` +
      `github.com/dustin/stagecoach/pkg/stagecoach`. `runDefault` is the root `RunE`.
- [ ] `--all`/`-a` runs `git add -A` BEFORE `HasStagedChanges` (FR20).
- [ ] Nothing-staged + `cfg.AutoStageAll && !flagNoAutoStage` → `AddAll` + `StagedFileCount` + the FR18
      notice `Nothing staged — staging all changes (N files).` to STDERR + re-check; still empty → exit 2
      `Nothing to commit.` (FR17).
- [ ] Nothing-staged + `flagNoAutoStage` → exit 2 `Nothing staged.` (FR19), regardless of `cfg.AutoStageAll`.
- [ ] Staged (or staged via auto-stage) → `stagecoach.GenerateCommit(ctx, Options{Provider:cfg.Provider,
      Model:cfg.Model, Timeout:cfg.Timeout, DryRun:flagDryRun})`.
- [ ] Commit success → stdout `[<7-char-sha>] <subject>` + DiffTree name-status file list (FR42); the
      file list computed by the CLI via `git.DiffTree(res.CommitSHA, isUnborn)` (isUnborn captured
      pre-GenerateCommit). DiffTree error post-commit is non-fatal (report without the list, exit 0).
- [ ] Dry-run success → stdout = `res.Message` ONLY (no decorations); exit 0; no commit created.
- [ ] `*RescueError` → `generate.FormatRescue(treeSHA, parentSHA, candidate)` to STDERR + silent
      `exitcode.New(code, nil)` where code=124 if `errors.Is(err, generate.ErrTimeout)` else 3.
- [ ] `*CASError` → `ce.Error()` (§13.5 "HEAD moved…") to STDERR + silent `exitcode.New(exitcode.Error, nil)`.
- [ ] `ErrNothingToCommit` → `exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))`
      (exit 2; main prints). Generic error → `exitcode.New(exitcode.Error, err)` (exit 1; main prints).
- [ ] `runDefault` never calls `os.Exit`; `Config()`-nil guarded; reads `cmd.Context()` for all ops.
- [ ] `go test -race ./internal/cmd/` green; `go test -race ./...` NO regression; `go vet ./...` clean;
      `gofmt -l internal/cmd/` empty; only `default_action.go` + `default_action_test.go` NEW +
      `root.go` edited.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact upstream
signatures (all quoted below + in research/design-decisions.md), the 11 design decisions, the PRD
§9.4/§9.9/§15.4/§15.5/Appendix B contracts (in `selected_prd_content`), the test conventions to mirror
(`internal/generate/generate_test.go` integration pattern + `internal/cmd/root_test.go` helpers), and the
copy-ready skeletons in the Implementation Blueprint. No signal/UI/subcommand knowledge required (those
are explicitly out of scope — S2 reads `cmd.Context()`, `Config()`, and the flag vars S1 already
registered).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T1S2/research/design-decisions.md
  why: the 11 decisions specific to this subtask. §0 (call pkg/stagecoach.GenerateCommit, NOT internal
       CommitStaged), §1 (CLI computes FR42 DiffTree itself — public Result drops Changes; minor race
       accepted), §2 (GenerateCommit re-loads via DISCOVERY — tests use .stagecoach.toml not --config),
       §3 (pass Options{Provider,Model,Timeout,DryRun} from cmd.Config()), §4 (silent exitcode.New(code,
       nil) to avoid main double-print; exitcode.For owns the mapping), §5 (stdout=result / stderr=
       notice+diagnostics), §6 (file plan), §7 (test seam: stub via .stagecoach.toml + t.Setenv STAGECOACH_STUB_*),
       §8 (auto-stage FSM), §9 (dry-run = message only), §10 (isolated report formatter), §11 (cmd.Context()).
  critical: §0/§1 (WHY the CLI re-does DiffTree), §2 (test file location — easy to get wrong), §4 (the
       silent-ExitError pattern prevents double output), §7 (Render includes os.Environ → stub inherits env).

- docfile: plan/001_f1f80943ac34/P1M4T1S1/PRP.md   (the SCAFFOLD contract — S2 hangs on it)
  section: "Data models" (internal/cmd/root.go skeleton: rootCmd, flag vars, PersistentPreRunE, Config(),
       Execute, Version, stub RunE) + "Implementation Tasks" Task 4/5 (root.go + root_test.go helpers).
  why: S1's root.go is the file S2 edits (stub RunE → runDefault). The flag vars (flagAll/flagNoAutoStage/
       flagDryRun), Config(), Execute(ctx), Version, and the root_test.go helpers (initRepo/setGitConfig/
       writeConfigFile/chdir/loadEnvSetup) are S1's deliverables S2 consumes. Treat as the contract for
       what exists when S2 begins.
  pattern: SilenceErrors+SilenceUsage; main calls os.Exit(exitcode.For(err)) exactly once and prints
       `stagecoach: <err>` only when err.Error() != "". S2 returns errors that interplay with BOTH.
  gotcha: --version/--help short-circuit BEFORE PersistentPreRunE (cobra), so config does NOT load for
       them — irrelevant to runDefault (only the root action reaches it). root_test.go's helpers are
       package-private to `cmd` → S2's test (same package) REUSES them directly (no re-copy of those 5).

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1 — READ the contract; do NOT edit)
  section: `func GenerateCommit(ctx context.Context, opts Options) (Result, error)` + `type Options
       {Provider, Model, SystemExtra, DryRun, Timeout}` + `type Result {CommitSHA, Subject, Message,
       Provider, Model}` (NO Changes!) + the re-exported `ErrNothingToCommit/ErrTimeout/ErrRescue/
       ErrCASFailed` + `RescueError`/`CASError` type aliases.
  why: THIS is the function runDefault calls. Note GenerateCommit's resolveConfig calls `config.Load(
       ctx, LoadOpts{RepoDir: os.Getwd(), Flags: nil})` — NO ConfigPathOverride (§2: tests use
       .stagecoach.toml). On success the commit-path Result.CommitSHA is the full 40-char SHA; DryRun
       returns CommitSHA=="". runPipeline (DryRun/SystemExtra) returns the SAME typed errors as the
       delegation path, so S2's error matrix is uniform.
  pattern: Options.Provider/Model/Timeout override config (highest precedence in resolveConfig); "" →
       inherit/auto-detect. DryRun does a single generation pass, no WriteTree, no commit.
  gotcha: Result has NO Changes field — S2 computes FR42's DiffTree itself (§1). GenerateCommit loads
       config a SECOND time (CLI's PersistentPreRunE loads once) — consistent for built-ins because both
       read discovery; the --config override is NOT seen by GenerateCommit (documented limitation).

- file: internal/generate/generate.go   (P1.M3.T4.S2 — READ for the error types; do NOT edit)
  section: `var ErrNothingToCommit/ErrTimeout/ErrRescue` + `ErrCASFailed = git.ErrCASFailed` +
       `type RescueError struct{Kind; TreeSHA; ParentSHA; Candidate; Cause}` (Unwrap→Kind) +
       `type CASError struct{TreeSHA; Expected; Actual; Message}` (Unwrap→git.ErrCASFailed; Error() =
       the §13.5 "HEAD moved…" message with the manual commit-tree command).
  why: runDefault's error matrix branches on these. `errors.As(err, &re)` covers BOTH timeout and rescue
       (both are *RescueError; distinguish via `errors.Is(err, generate.ErrTimeout)`). The §13.5 message
       is already ce.Error() — S2 prints it verbatim, no re-assembly.
  pattern: RescueError.Unwrap()==Kind → errors.Is(err, ErrTimeout) works on a *RescueError{Kind:ErrTimeout}.
       A timeout and a rescue BOTH carry a non-empty TreeSHA (snapshot taken before generation).
  gotcha: check ErrTimeout BEFORE ErrRescue (a timeout IS a rescue with Kind=ErrTimeout). exitcode.For
       already does this; S2 mirrors it when choosing the silent-return code.

- file: internal/generate/rescue.go   (P1.M3.T3.S1 — READ; do NOT edit)
  section: `func FormatRescue(treeSHA, parentSHA, candidateMsg string) string` — pure assembler; returns
       the §18.3 block with NO trailing newline (S2's fmt.Fprintln adds it). Omits `-p <parentSHA>` when
       parentSHA=="" (root commit). Appends the candidate note when candidateMsg != "".
  why: runDefault prints `generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)` on the rescue/
       timeout path. The fields come straight off the *RescueError — no computation needed.

- file: internal/git/git.go   (P1.M1.T2/T3 — READ the Git interface; do NOT edit)
  section: `type Git interface` + `func New(workDir string) Git` + the methods runDefault uses:
       `HasStagedChanges(ctx) (bool, error)` (exit 1 ⇒ true; NOT an error),
       `AddAll(ctx) error`, `StagedFileCount(ctx) (int, error)`,
       `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)` (exit 128 ⇒ isUnborn),
       `DiffTree(ctx, sha string, isRoot bool) ([]FileChange, error)` (isRoot MUST be true for a root
       commit, else empty output) + `type FileChange struct{Status; SrcPath; Path}`.
  why: S2 constructs `git.New(repoDir)` for the auto-stage FSM AND the FR42 DiffTree report. The isUnborn
       from RevParseHEAD feeds DiffTree's isRoot (capture it BEFORE GenerateCommit — auto-stage doesn't
       change isUnborn). FileChange.Status is "A"/"M"/"D"/"R100"/…; SrcPath only for R/C.
  pattern: every method targets the repo via -C (goroutine-safe); a non-zero git exit is (stdout, stderr,
       code, nil) for the read methods — but the typed methods already decode that into (bool/int/sha, err).
  gotcha: DiffTree on a root commit WITHOUT isRoot=true yields empty output (silent trap) — pass isUnborn.
       HasStagedChanges exit 1 means STAGED (inverted from the usual convention) — the method already
       returns a bool, so just read it.

- file: internal/config/config.go   (P1.M1.T4.S1 — READ for Config fields; do NOT edit)
  section: `type Config struct { Provider, Model string; Timeout time.Duration; AutoStageAll, Verbose,
       NoColor bool; … }` + `func Defaults() Config` (Timeout 120s, AutoStageAll true).
  why: runDefault reads `cfg := Config()` (S1's PersistentPreRunE result) for `cfg.Provider`,
       `cfg.Model`, `cfg.Timeout` (→ Options) and `cfg.AutoStageAll` (→ the FSM). AutoStageAll defaults
       true (Layer 1); --no-auto-stage (flagNoAutoStage) overrides per-invocation.
  gotcha: there is NO All/DryRun/NoAutoStage Config field — those are the flag vars S1 registered
       (flagAll/flagDryRun/flagNoAutoStage). Provider/Model/Timeout ARE config fields (already resolved
       with Layer-7 flags by the time Config() returns).

- file: internal/cmd/root.go   (P1.M4.T1.S1 — EDIT one line: stub RunE → runDefault)
  section: `var rootCmd = &cobra.Command{ … RunE: func(cmd, args) error { return cmd.Help() } … }` (S1's
       stub) + the package vars `flagAll/flagNoAutoStage/flagDryRun bool` + `func Config() *config.Config`
       + `func Execute(ctx context.Context) error` (sets ctx on rootCmd).
  why: S2's edit is the single line changing the RunE. runDefault reads cmd.Context() (Execute set it),
       Config() (PersistentPreRunE set it), and the three flag vars. Nothing else in root.go changes.
  pattern: SilenceErrors+SilenceUsage are on (main owns output); PersistentPreRunE loaded config already.
  gotcha: do NOT touch root.go's flag registration, PersistentPreRunE, Config(), Execute, or Version —
       only the RunE field. Keep the edit surgical (merge into the existing Command literal cleanly).

- file: internal/cmd/root_test.go   (P1.M4.T1.S1 — READ; reuse its helpers, do NOT edit)
  section: the copied helpers `initRepo(t, dir)`, `setGitConfig(t, dir, key, value)`,
       `writeConfigFile(t, dir, relPath, body) string`, `chdir(t, dir)`, `loadEnvSetup(t) (home, repo,
       globalDir string)`. All `package cmd` (same package as S2's test) → directly callable.
  why: default_action_test.go drives the FULL CLI in a temp repo: it needs a git repo (initRepo), CWD
       isolation (chdir + loadEnvSetup for HOME/XDG), and config files (writeConfigFile). Reuse these —
       do NOT re-copy them (they're in the same package). ALSO copy the generate-style helpers S1 did NOT
       copy: writeFile/stageFile/commitRaw/headSHA/gitOut/runGit + shaRe (from generate_test.go).
  gotcha: rootCmd is a package-level singleton — each test restores SetArgs(nil)/SetOut/SetErr/loadedCfg
       in t.Cleanup or it poisons siblings (and trips -race). loadEnvSetup returns globalDir for writing
       the global config (S2 mostly uses the repo-local .stagecoach.toml instead — see §2/§7).

- file: internal/generate/generate_test.go   (P1.M3.T4.S2 — READ the integration-test PATTERN; copy helpers)
  section: the fixture helpers `writeFile/stageFile/commitRaw/headSHA/gitOut/runGit` + `shaRe` (lines
       ~13-70) AND the scenario structure (build stub → initRepo → commitRaw initial → writeFile+stageFile
       → stubtest.Manifest/Options → CommitStaged → assert SHA/HEAD/log). TestCommitStaged_RootCommit
       (unborn repo) and TestCommitStaged_Timeout/ParseFailRescue/CASFailure are the templates for S2's
       rescue/timeout/CAS tests (at the CLI boundary instead of the orchestrator).
  why: S2's integration test mirrors this EXACTLY but drives `rootCmd`/`Execute` (the CLI) instead of
       `CommitStaged` directly, and injects the stub via .stagecoach.toml + t.Setenv instead of a direct
       Manifest arg. The git-content helpers (writeFile etc.) are package-private to `generate` → copy
       them verbatim into default_action_test.go.
  gotcha: the stub's behavior knobs are STAGECOACH_STUB_OUT (single response), STAGECOACH_STUB_SCRIPT+
       _COUNTER (call-varying), STAGECOACH_STUB_EXIT (non-zero), STAGECOACH_STUB_SLEEP_MS (slow/timing-out).
       provider.Render includes os.Environ() → t.Setenv on these reaches the stub through the full CLI.

- file: internal/stubtest/stubtest.go   (P1.M3.T4.S1 — READ; use Build only)
  section: `func Build(t testing.TB) string` (compiles cmd/stubagent once, cached) + `type Options`
       + `func Manifest(bin, Options) provider.Manifest` (NOT used by S2 — the CLI resolves via the
       registry from .stagecoach.toml; S2 uses Build for the binary path only).
  why: S2's test calls `bin := stubtest.Build(t)` to get the stub binary path, then writes that path into
       `.stagecoach.toml`'s `[provider.stub] command`. The Options/Manifest helpers are for DIRECT
       orchestrator tests (generate_test.go); S2 does NOT use them (it goes through the registry).
  gotcha: Build skips the test if `go` isn't on PATH (t.Skipf). The stub binary is shared across tests
       (sync.Once) — fine; each test sets its OWN STAGECOACH_STUB_* via t.Setenv (restored automatically).

- url: (PRD §9.4 FR16–FR20, §9.9 FR42, §15.1 synopsis, §15.4 exit codes, §15.5 examples, §18.3 rescue,
       Appendix B.1/B.3/B.5 — already in context as selected_prd_content `h3.20`/`h2.15`/`h3.52`/`h3.56`/
       `h2.8`/`h2.25`; ALSO plan/001_f1f80943ac34/prd_snapshot.md §9, §15, §18, Appendix B)
  why: §9.4 is the AUTHORITATIVE auto-stage spec (FR16–FR20, including the exact FR18 notice wording and
       the --no-auto-stage/--all semantics). §15.4 is the AUTHORITATIVE exit-code table (0/1/2/3/124).
       Appendix B.1 is the FR42 report template (`↳ Created <sha>  <subject>` + name-status list); B.3 is
       the dry-run; B.5 is the rescue block.
  critical: §15.4 codes 2=nothing-to-commit / 3=rescue / 124=timeout (NOT the arch doc's generic table).
       Appendix B.1's "↳ Created" + "↳ Generating…" decorations are P1.M4.T3 progress style — S2 prints
       the DATA (`[<sha>] <subject>` + file list); P1.M4.T3 adds the ↳ wrapper/color. FR18's exact string:
       `Nothing staged — staging all changes (N files).` (em-dash, "files" pluralized — match verbatim).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; cobra+pflag+go-toml/v2 (S1 added cobra)
cmd/stagecoach/main.go               # P1.M4.T1.S1 — var version; ctx; cmd.Execute(ctx); os.Exit(exitcode.For(err))  (UNCHANGED by S2)
cmd/stubagent/main.go               # the fake agent read by stubtest.Build (UNCHANGED)
internal/
  cmd/root.go                       # P1.M4.T1.S1 — rootCmd + flags + PersistentPreRunE + Config() + Execute + STUB RunE  (S2 EDITS the RunE line)
  cmd/root_test.go                  # P1.M4.T1.S1 — helpers: initRepo/setGitConfig/writeConfigFile/chdir/loadEnvSetup (REUSED by S2)
  exitcode/exitcode.go              # P1.M4.T1.S1 — For/New/ExitError + §15.4 constants (read-only ref)
  config/{config,file,git,load}.go  # P1.M1.T4 — Load/Config/Defaults (read-only ref)
  generate/generate.go              # P1.M3.T4.S2 — error types (read-only ref)
  generate/rescue.go                # P1.M3.T3.S1 — FormatRescue (read-only ref)
  generate/generate_test.go         # P1.M3.T4.S2 — integration PATTERN + git-content helpers to COPY
  git/git.go                        # P1.M1.T2/T3 — Git interface + New (read-only ref)
  provider/{render,executor,registry,manifest}.go  # P1.M2 — Render(os.Environ!), Execute, registry, Validate (read-only ref)
  stubtest/stubtest.go              # P1.M3.T4.S1 — Build(t) (read-only ref; S2 uses Build only)
  {prompt,provider}/                # untouched by S2
pkg/stagecoach/stagecoach.go          # P1.M3.T5.S1 — GenerateCommit + Options + Result (read-only ref; the seam S2 calls)
Makefile                            # build/test(-race)/coverage/lint/clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/default_action.go        # NEW — package cmd. runDefault(cmd, args) error (the full default
                                      #        action: auto-stage FSM FR16–FR20 → GenerateCommit → error
                                      #        matrix → FR42 report / dry-run message) + shortSHA +
                                      #        printCommitReport + printDryRunMessage helpers.
internal/cmd/default_action_test.go   # NEW — package cmd. Integration tests via rootCmd/Execute through
                                      #        a stub provider (.stagecoach.toml + t.Setenv): commit, root
                                      #        commit, dry-run, FR17/FR19/--all/FR18, rescue, timeout, CAS.
                                      #        Reuses root_test.go helpers; copies generate-style helpers.
internal/cmd/root.go                  # EDIT — the stub RunE `return cmd.Help()` → `runDefault` (one line).
# All other files UNCHANGED. internal/{config,generate,git,prompt,provider}, pkg/stagecoach, exitcode UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (call the PUBLIC API, design §0): runDefault calls pkg/stagecoach.GenerateCommit, NOT
// generate.CommitStaged. The public Result DROPS Changes (P1.M3.T5.S1 design §1) → the CLI computes
// FR42's DiffTree ITSELF. Calling CommitStaged directly would force reimplementing buildDeps (registry
// + auto-detect + Validate) — don't. Dogfood the public surface (US12).

// CRITICAL (GenerateCommit re-loads config via DISCOVERY, design §2): pkg/stagecoach.resolveConfig calls
// config.Load(ctx, LoadOpts{RepoDir: os.Getwd(), Flags: nil}) — NO ConfigPathOverride. So a --config
// <file> is seen by the CLI's PersistentPreRunE but NOT by GenerateCommit. For BUILT-IN providers
// (always available) this is invisible. It only bites a CUSTOM manifest defined solely in a --config
// file. TESTS therefore put the stub provider in the repo-local ./.stagecoach.toml (both read it).

// CRITICAL (exit codes via exitcode.For; avoid double-print, design §4): S1's main prints `stagecoach:
// <err>` when err.Error() != "". If runDefault prints the FULL rescue/CAS message AND returns the
// original error (non-empty Error()), main prints a SECOND line. FIX: after printing the detailed
// message, return a SILENT exitcode.New(code, nil) — ExitError.Error()=="" (Err nil) → main skips. The
// exit code is still honored (exitcode.For returns the explicit ExitError.Code). Friendly messages
// (FR17/FR19/ErrNothingToCommit) DO pass a non-nil err so main prints "stagecoach: <msg>".

// CRITICAL (stdout=result / stderr=notice+diagnostics, design §5): PRD §15.5 pipes dry-run output
// (stagecoach --dry-run | tee). So stdout = the commit report (FR42) and the dry-run message ONLY;
// stderr = the FR18 auto-stage notice + the rescue block + the CAS message + error diagnostics.
// Interactive terminals merge both; pipes capture only the result.

// GOTCHA (DiffTree needs isRoot, design §1): git.DiffTree(ctx, sha, isRoot) yields EMPTY output for a
// root commit UNLESS isRoot=true (the --root flag). Capture isUnborn via RevParseHEAD BEFORE
// GenerateCommit (auto-stage/AddAll don't change isUnborn — staging isn't a commit) and pass it as
// isRoot after success. DiffTree failure post-commit is NON-FATAL: the commit already landed — print
// the report without the file list and still return nil (exit 0).

// GOTCHA (HasStagedChanges exit-1-means-staged, git.go): git diff --cached --quiet exits 1 when staged
// changes EXIST (inverted). The typed method already returns (bool, error) — just read the bool. Do NOT
// treat exit 1 as an error; the method does not expose it as one.

// GOTCHA (timeout vs rescue both are *RescueError): generate returns *RescueError{Kind:ErrTimeout} for
// timeout and *RescueError{Kind:ErrRescue} for exhausted-retries. errors.As(err, &re) catches BOTH.
// Distinguish via errors.Is(err, generate.ErrTimeout) (Unwrap==Kind) → 124; else 3. exitcode.For does
// the same; S2 mirrors it ONLY to pick the silent-return code paired with its own print.

// GOTCHA (stub provider Validate, manifest.go): Validate requires Name (non-empty) + Command (non-nil,
// non-empty). DefaultModel is OPTIONAL (Resolve defaults to ""). So a [provider.stub] with just command
// + prompt_delivery + output + strip_code_fence passes Validate. The registry adds "stub" VERBATIM as a
// §12.8 provider (no built-in base) with Name from the table key.

// GOTCHA (Render includes os.Environ, render.go): provider.Render builds spec.Env = os.Environ() +
// manifest.Env (manifest wins on collision); the executor sets cmd.Env = spec.Env (non-empty) OR
// inherits parent env (empty). EITHER WAY the stub agent inherits t.Setenv("STAGECOACH_STUB_*"). This is
// why the test controls the stub via t.Setenv instead of encoding it in the manifest env.

// GOTCHA (rootCmd is a package-level singleton): default_action_test.go drives rootCmd directly via
// SetArgs/SetOut/SetErr. RESTORE state in t.Cleanup (SetArgs([]string{}), restore Out/Err, loadedCfg=nil)
// or tests poison each other (and trip -race). Mirror root_test.go's hygiene (S1).

// GOTCHA (helpers: reuse root_test.go's, copy generate_test.go's): initRepo/setGitConfig/writeConfigFile/
// chdir/loadEnvSetup are package-private to `cmd` (S1 put them in root_test.go) → S2's test (same pkg)
// calls them directly. writeFile/stageFile/commitRaw/headSHA/gitOut/runGit/shaRe are package-private to
// `generate` (unimportable) → COPY the ~30-line set verbatim into default_action_test.go.

// GOTCHA (FR18 exact wording): "Nothing staged — staging all changes (N files)." — em-dash (—, U+2014),
// "files" always plural, to STDERR. N comes from git.StagedFileCount AFTER AddAll. Match verbatim (FR18).

// GOTCHA (short-sha is 7 chars): res.CommitSHA is the full 40-char hex. Appendix B uses 7-char SHAs.
// shortSHA(sha) = sha if len<7 else sha[:7]. Do NOT shell out to `git rev-parse --short` (no Git method
// for it; a fixed 7-char prefix matches the PRD examples and is deterministic for tests).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/cmd/default_action.go
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/pkg/stagecoach"
)

// runDefault is the root command's default action (PRD §15.1): commit staged changes, auto-staging all
// if nothing is staged and auto_stage_all is on (§9.4 FR16–FR20). It delegates generation+commit to the
// PUBLIC API pkg/stagecoach.GenerateCommit (US12 dogfooding), renders the FR42 success report, and maps
// every §15.4 outcome by RETURNING an error that exitcode.For (S1) + main translate to an exit code.
// It never calls os.Exit (only main does). Auto-stage lives HERE (CLI layer), not in the orchestrator
// (PRD §11.3 — CommitStaged/GenerateCommit never call git add).
//
// Output streams (design §5): stdout = the result (FR42 commit report, or the dry-run message — pipeable
// per §15.5); stderr = notices + diagnostics (FR18 auto-stage notice, the §18.3 rescue block, the §13.5
// CAS message). To avoid main double-printing a detailed message, the rescue/CAS paths return a SILENT
// exitcode.New(code, nil) (ExitError.Error()=="" → main's `err.Error() != ""` guard skips printing).
func runDefault(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context() // S1's Execute set this; P1.M4.T2 swaps it for a signal-aware ctx later.

	cfg := Config()
	if cfg == nil {
		// Defensive: PersistentPreRunE always loads config for the root action (cmd.Name()=="stagecoach"),
		// so this is unreachable in practice. Still fail loudly rather than nil-deref.
		return exitcode.New(exitcode.Error, errors.New("stagecoach: configuration not loaded"))
	}

	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
	}
	g := git.New(repoDir)

	// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
	// FR20: --all/-a forces `git add -A` BEFORE the staged check, even if something is already staged.
	if flagAll {
		if err := g.AddAll(ctx); err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
		}
	}

	hasStaged, err := g.HasStagedChanges(ctx)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --quiet: %w", err))
	}

	if !hasStaged {
		switch {
		case flagNoAutoStage:
			// FR19: --no-auto-stage + nothing staged → exit 2 "Nothing staged." (--no-auto-stage wins
			// over cfg.AutoStageAll). main prints "stagecoach: Nothing staged." (non-nil err).
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing staged."))
		case cfg.AutoStageAll:
			// FR16/FR18: auto-stage all, print the transparent notice, re-check.
			if err := g.AddAll(ctx); err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
			}
			n, err := g.StagedFileCount(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
			}
			fmt.Fprintf(os.Stderr, "Nothing staged — staging all changes (%d files).\n", n) // FR18 (verbatim, em-dash)
			hasStaged, err = g.HasStagedChanges(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --quiet: %w", err))
			}
			if !hasStaged {
				// FR17: clean tree even after auto-stage.
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
			}
		default:
			// cfg.AutoStageAll==false (config), no --no-auto-stage flag → don't auto-stage; exit 2.
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
		}
	}

	// ---- Staged (or staged via auto-stage): capture isUnborn, then generate+commit via the public API ----
	// isUnborn feeds DiffTree's isRoot for the FR42 report (design §1). Captured BEFORE GenerateCommit;
	// AddAll/HasStagedChanges don't change isUnborn (staging isn't a commit).
	_, isUnborn, err := g.RevParseHEAD(ctx)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("git rev-parse HEAD: %w", err))
	}

	// §3: re-apply the CLI-resolved provider/model/timeout (Layer-7 flags already applied by
	// PersistentPreRunE) as Options — GenerateCommit re-loads config with Flags:nil, so opts is how the
	// CLI flags take effect (opts override is highest precedence in resolveConfig).
	res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{
		Provider: cfg.Provider,
		Model:    cfg.Model,
		Timeout:  cfg.Timeout,
		DryRun:   flagDryRun,
	})
	if err != nil {
		return handleGenError(err) // §4: rescue/CAS/timeout/nothing/generic matrix
	}

	// ---- Success ----
	if flagDryRun || res.CommitSHA == "" {
		// Dry-run (Appendix B.3): stdout = the message ONLY (§15.5 pipe use case). The "↳ Generating…"
		// / "(no commit created)" decorations are P1.M4.T3/T4. No commit was created.
		printDryRunMessage(os.Stdout, res.Message)
		return nil // exit 0
	}

	// Commit path: FR42 report. Compute the DiffTree file list ourselves — pkg/stagecoach.Result drops
	// Changes (design §1). Best-effort: a DiffTree error post-commit is non-fatal (commit already landed).
	changes, derr := g.DiffTree(ctx, res.CommitSHA, isUnborn)
	if derr != nil {
		changes = nil // report without the file list; do NOT fail the success
	}
	printCommitReport(os.Stdout, res, changes)
	return nil // exit 0
}

// handleGenError maps a GenerateCommit error to the §15.4 outcome WITH the right user-facing output. It
// prints the detailed message for rescue/CAS (to stderr) and returns a SILENT exitcode.New(code, nil) so
// main does not double-print; for friendly/generic errors it returns exitcode.New(code, err) so main
// prints "stagecoach: <msg>". (design §4)
func handleGenError(err error) error {
	var re *generate.RescueError
	if errors.As(err, &re) { // covers BOTH ErrTimeout and ErrRescue (both are *RescueError)
		fmt.Fprintln(os.Stderr, generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
		code := exitcode.Rescue
		if errors.Is(err, generate.ErrTimeout) { // timeout → 124; rescue → 3 (timeout checked first)
			code = exitcode.Timeout
		}
		return exitcode.New(code, nil) // silent → main prints nothing; exit code honored
	}
	var ce *generate.CASError
	if errors.As(err, &ce) {
		fmt.Fprintln(os.Stderr, ce.Error())            // the §13.5 "HEAD moved…" message
		return exitcode.New(exitcode.Error, nil)       // silent; exit 1
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // exit 2; main prints
	}
	return exitcode.New(exitcode.Error, err) // generic (render/exec/write-tree/unknown-provider/…): exit 1
}

// shortSHA returns the 7-char short form of a full SHA (Appendix B uses 7-char SHAs). Returns sha
// unchanged if shorter than 7 (defensive; a real SHA is 40 hex chars).
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// printCommitReport renders the FR42 success report to w (stdout): `[<short-sha>] <subject>` followed by
// the DiffTree name-status file list, one entry per line. Isolated as a function so P1.M4.T3 can restyle
// it (the "↳ Created" decoration + color) without touching runDefault's flow (design §10). Format matches
// PRD Appendix B.1's data; the ↳ progress wrapper is P1.M4.T3.
func printCommitReport(w io.Writer, res stagecoach.Result, changes []git.FileChange) {
	fmt.Fprintf(w, "[%s] %s\n", shortSHA(res.CommitSHA), res.Subject)
	for _, c := range changes {
		if c.SrcPath != "" { // R/C rename/copy: "<status>\t<src>\t<dst>" → show "R100  old → new"
			fmt.Fprintf(w, "%s  %s → %s\n", c.Status, c.SrcPath, c.Path)
			continue
		}
		fmt.Fprintf(w, "%s  %s\n", c.Status, c.Path)
	}
}

// printDryRunMessage writes the generated message to w (stdout) for --dry-run. stdout = message ONLY
// (§15.5: `stagecoach --dry-run --no-color | tee /tmp/msg.txt`). Decorations are P1.M4.T3/T4 (design §9).
func printDryRunMessage(w io.Writer, msg string) {
	fmt.Fprintln(w, msg)
}
```

```go
// EDIT to internal/cmd/root.go (S1's file — ONE line change).
// BEFORE (S1 stub):
//   RunE: func(cmd *cobra.Command, args []string) error {
//       return cmd.Help()
//   },
// AFTER:
//   RunE: runDefault,
// (runDefault is defined in default_action.go, same package.) Everything else in root.go is UNCHANGED.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/default_action.go (the default action + helpers)
  - FILE: NEW internal/cmd/default_action.go. PACKAGE: `package cmd`. Follow "Data models" skeleton.
  - DEFINE: runDefault(cmd *cobra.Command, args []string) error (the full flow), handleGenError(err)
      error, shortSHA(sha) string, printCommitReport(w io.Writer, res stagecoach.Result, changes
      []git.FileChange), printDryRunMessage(w io.Writer, msg string).
  - IMPORTS: errors, fmt, io, os, github.com/spf13/cobra, github.com/dustin/stagecoach/internal/exitcode,
      github.com/dustin/stagecoach/internal/generate, github.com/dustin/stagecoach/internal/git,
      github.com/dustin/stagecoach/pkg/stagecoach. (Confirm the module path is github.com/dustin/stagecoach.)
  - NAMING: runDefault/handleGenError/shortSHA/printCommitReport/printDryRunMessage (unexported, package-
      level). PLACEMENT: all in internal/cmd/default_action.go.
  - GOTCHA: the FR18 string is EXACTLY `Nothing staged — staging all changes (%d files).\n` (em-dash —,
      U+2014; "files" plural) to STDERR. DiffTree failure post-commit is non-fatal (changes=nil, exit 0).
      handleGenError checks *RescueError (errors.As) → ErrTimeout-before-rescue; then *CASError; then
      ErrNothingToCommit; then generic. The rescue/CAS returns SILENT exitcode.New(code, nil).
  - GOTCHA: read cmd.Context() (NOT context.Background()) so P1.M4.T2's signal ctx flows through
      unchanged. Read Config() (S1's store); guard nil defensively.

Task 2: EDIT internal/cmd/root.go (wire runDefault as the root RunE — one line)
  - FILE: internal/cmd/root.go (S1's file). Change ONLY the RunE field of rootCmd.
  - BEFORE: `RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },` (the S1 stub;
      if S1 phrased it differently, match S1's actual text — the intent is "the stub that prints help").
  - AFTER: `RunE: runDefault,`
  - GOTCHA: do NOT touch flag registration, PersistentPreRunE, Config(), Execute, Version, or the flag
      vars. This is a ONE-line surgical edit. If root.go's RunE is an inline closure, replace the whole
      closure with the identifier `runDefault`.

Task 3: CREATE internal/cmd/default_action_test.go (integration tests through the FULL CLI)
  - FILE: NEW internal/cmd/default_action_test.go. PACKAGE: `package cmd` (same as root_test.go).
  - REUSE root_test.go helpers (same package — do NOT re-copy): initRepo, setGitConfig, writeConfigFile,
      chdir, loadEnvSetup.
  - COPY generate-style helpers verbatim from internal/generate/generate_test.go (package-private there,
      unimportable): writeFile, stageFile, commitRaw, headSHA, gitOut, runGit, shaRe (~30 lines). Keep
      their bodies identical (runGit uses `git -C dir`).
  - STUB SEAM helper: a small `setupStubRepo(t, out string) (repoDir string)` (or per-test inline) that:
      bin := stubtest.Build(t); repo := t.TempDir(); initRepo(t, repo); chdir(t, repo); write a
      .stagecoach.toml into repo via writeConfigFile whose [provider.stub] points command at bin with
      prompt_delivery=stdin/output=raw/strip_code_fence=true; t.Setenv("STAGECOACH_STUB_OUT", out). Return
      repo. (loadEnvSetup isolates the global config; .stagecoach.toml is the repo-local discovery file
      BOTH the CLI and GenerateCommit read — design §2/§7.)
  - STATE HYGIENE: each test that mutates rootCmd restores in t.Cleanup: rootCmd.SetArgs(nil) (resets
      args), restore the original Out/Err writers, set loadedCfg=nil. Capture err via Execute(ctx) and
      derive the exit code via exitcode.For(err) (import internal/exitcode in the test).
  - CASES (drive rootCmd via SetArgs + Execute(context.Background()); assert exitcode.For(err), HEAD via
      headSHA, git log via gitOut, captured stdout/stderr):
      * TestRunDefault_Commit: setupStubRepo("feat: add login"); commitRaw initial; writeFile+stageFile
        "new.txt"; SetArgs(["--provider","stub"]); Execute. Assert exit 0; HEAD != initial AND ==
        res-equivalent new SHA; gitOut("log","--format=%B","-n1")=="feat: add login"; stdout contains
        "[<7hex>] feat: add login" and the "A  new.txt" file line.
      * TestRunDefault_RootCommit: setupStubRepo("chore: initial"); NO commitRaw (unborn); writeFile+
        stageFile "first.txt"; SetArgs(["--provider","stub"]); Execute. Assert exit 0; HEAD == new SHA;
        gitOut("cat-file","-p",sha) has NO "parent " line; stdout file list non-empty (DiffTree --root).
      * TestRunDefault_DryRun: setupStubRepo("feat: dry"); commitRaw initial; writeFile+stageFile "x.txt";
        SetArgs(["--provider","stub","--dry-run"]); Execute. Assert exit 0; stdout == "feat: dry\n"
        (message ONLY — nothing else on stdout); HEAD UNCHANGED (no commit); index unchanged.
      * TestRunDefault_NothingStaged_FR17: setupStubRepo("feat: x"); commitRaw initial; NOTHING staged;
        SetArgs(["--provider","stub"]); Execute. Assert exitcode.For(err)==2 (NothingToCommit); HEAD
        unchanged. (AutoStageAll default true → tries add -A on a clean tree → FR18(0 files)+FR17. Accept
        the FR18(0) notice on stderr; assert exit 2.)
      * TestRunDefault_NoAutoStage_FR19: setupStubRepo("feat: x"); commitRaw initial; writeFile "y.txt"
        but DO NOT stage; SetArgs(["--provider","stub","--no-auto-stage"]); Execute. Assert exit 2; the
        error/err indicates "Nothing staged." (main prints "stagecoach: Nothing staged."). HEAD unchanged.
      * TestRunDefault_AllFlag: setupStubRepo("feat: all"); commitRaw initial; writeFile "a.txt" (NOT
        staged); SetArgs(["--provider","stub","-a"]); Execute. Assert exit 0; HEAD moved; gitOut log
        "feat: all"; stdout has "A  a.txt" (AddAll staged it).
      * TestRunDefault_AutoStageNotice_FR18: setupStubRepo("feat: auto"); commitRaw initial; writeFile
        "u.txt"+"v.txt" (NOT staged); SetArgs(["--provider","stub"]); Execute. Assert exit 0; STDERR
        contains "Nothing staged — staging all changes (2 files)." (FR18 verbatim, em-dash); HEAD moved.
      * TestRunDefault_Rescue: stub via STAGECOACH_STUB_SCRIPT returning "" (blank → parse fail) +
        MaxDuplicateRetries=0 to exhaust. setupStubRepo with a .stagecoach.toml [generation]
        max_duplicate_retries=0 and a script blank response; commitRaw initial; stage "z.txt"; SetArgs
        (["--provider","stub"]); Execute. Assert exitcode.For(err)==3; STDERR contains the §18.3 block
        ("❌ Commit generation failed.", "Tree ID:", the commit-tree command); HEAD + index UNCHANGED
        (idempotent — §20.2).
      * TestRunDefault_Timeout: setupStubRepo with STAGECOACH_STUB_SLEEP_MS large + a short timeout via
        STAGECOACH_TIMEOUT or .stagecoach.toml [defaults] timeout="200ms" (SleepMS=2000); commitRaw initial;
        stage "z.txt"; SetArgs(["--provider","stub"]); Execute. Assert exitcode.For(err)==124; STDERR has
        the rescue block; HEAD unchanged. (Mirror generate_test.go's TestCommitStaged_Timeout at the CLI.)
      * TestRunDefault_CAS: stub SleepMS=400; race a concurrent commitRaw during generation (mirror
        generate_test.go TestCommitStaged_CASFailure). Assert exitcode.For(err)==1; STDERR contains
        "HEAD moved"; HEAD == the concurrent commit (the orchestrator's did NOT land).
  - COVERAGE: the full §15.4 matrix (0/2/3/124/1) + FR16–FR20 + FR42 + dry-run, through the real CLI via
      a stub agent. Use stubtest.Build (compiles cmd/stubagent once). Skip-friendly if `go` missing.

Task 4: VALIDATE (run all gates; fix before declaring done)
  - `make build` → ./bin/stagecoach exists.
  - MANUAL smoke (inside a scratch git repo with a staged change): `./bin/stagecoach --provider <a real
      or stub agent>` produces a commit + the FR42 report; `./bin/stagecoach --dry-run` prints the message;
      clean tree → "Nothing to commit." exit 2. (A real agent is optional — the unit tests cover behavior
      with the stub; this is a human sanity check if an agent is handy.)
  - `go test -race ./internal/cmd/ -v` → green (root_test.go + default_action_test.go).
  - `go test -race ./...` → green (NO regression — internal/{config,generate,git,provider,prompt},
      pkg/stagecoach, exitcode untouched).
  - `go vet ./...` clean; `gofmt -l internal/cmd/` empty.
  - `git status` shows ONLY: new internal/cmd/default_action.go, new internal/cmd/default_action_test.go,
    modified internal/cmd/root.go (the one-line RunE edit).
```

### Implementation Patterns & Key Details

```go
// PATTERN: the default action delegates to the PUBLIC API and maps outcomes via returned errors.
func runDefault(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()                 // P1.M4.T2 swaps this; S2 reads it unchanged
    cfg := Config()                      // S1's PersistentPreRunE resolved it (Layer-7 flags applied)
    g := git.New(mustGetwd())            // the CLI's own git boundary (auto-stage + FR42 DiffTree)

    if flagAll { g.AddAll(ctx) }         // FR20 (force stage before the check)
    if has, _ := g.HasStagedChanges(ctx); !has {
        switch {
        case flagNoAutoStage:            return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing staged."))     // FR19
        case cfg.AutoStageAll:           g.AddAll(ctx); n, _ := g.StagedFileCount(ctx)
                                         fmt.Fprintf(os.Stderr, "Nothing staged — staging all changes (%d files).\n", n)   // FR18
                                         if has2, _ := g.HasStagedChanges(ctx); !has2 {
                                             return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // FR17
                                         }
        default:                         return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
        }
    }

    _, isUnborn, _ := g.RevParseHEAD(ctx)                                   // feeds DiffTree isRoot (design §1)
    res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{            // §3: public API dogfood
        Provider: cfg.Provider, Model: cfg.Model, Timeout: cfg.Timeout, DryRun: flagDryRun,
    })
    if err != nil { return handleGenError(err) }                            // §4 matrix

    if flagDryRun || res.CommitSHA == "" { printDryRunMessage(os.Stdout, res.Message); return nil }
    changes, _ := g.DiffTree(ctx, res.CommitSHA, isUnborn)                  // FR42 (best-effort; nil on err)
    printCommitReport(os.Stdout, res, changes)
    return nil
}

// PATTERN: silent ExitError after a detailed print → exactly one user-facing message.
//   main (S1): `if err != nil && err.Error() != "" { fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err) }`
//   exitcode.New(code, nil).Error() == "" → main's guard is FALSE → no second line. exitcode.For returns
//   the explicit code. For friendly messages (FR17/FR19) pass a NON-nil err so main prints it.

// GOTCHA: the FR42 file list is computed by the CLI (pkg/stagecoach.Result drops Changes). isUnborn must
// be captured BEFORE GenerateCommit (it commits → HEAD changes). A root commit's DiffTree needs isRoot=
// true or it returns empty.

// GOTCHA: GenerateCommit re-loads config via DISCOVERY (no --config). The CLI passes Options to re-apply
// the resolved provider/model/timeout. Tests put the stub in ./.stagecoach.toml (read by both layers).
```

### Integration Points

```yaml
ROOT.COMMAND (S1 → S2):
  - rootCmd.RunE: "S1 stub (cmd.Help) → runDefault (this subtask). ONE-line edit."
  - gotcha: "do NOT touch flag registration / PersistentPreRunE / Config() / Execute / Version."

FLAG.VARS (S1 → S2 reads):
  - flagAll, flagNoAutoStage, flagDryRun: "S1 registered these on PersistentFlags; runDefault reads them.
    --dry-run's OUTPUT decorations are P1.M4.T4; S2 executes the dry-run branch (prints message only)."

CONFIG.STORE (S1 → S2 reads):
  - Config(): "S1's PersistentPreRunE result (Layer-7 flags applied). runDefault reads cfg.Provider/
    cfg.Model/cfg.Timeout (→ Options) + cfg.AutoStageAll (→ FSM)."

EXIT.CODE (S1 → S2 returns errors it maps):
  - exitcode.For/New/ExitError + constants: "S1 centralized the §15.4 mapping. runDefault returns
    exitcode.New(code, err|nil); main calls os.Exit(exitcode.For(err)). S2 never calls os.Exit."

CONTEXT (S1 → S2 reads; P1.M4.T2 swaps):
  - cmd.Context(): "S1's Execute set ctx on rootCmd. runDefault reads cmd.Context() for all git ops +
    GenerateCommit. P1.M4.T2 replaces it with signal.NotifyContext + child-kill/rescue — no S2 change."

PUBLIC.API (P1.M3.T5.S1 → S2 calls):
  - stagecoach.GenerateCommit: "the seam. S2 passes Options{Provider,Model,Timeout,DryRun} from cmd.Config().
    pkg/stagecoach is FROZEN (parallel-built) — do NOT edit it."

GIT.BOUNDARY (P1.M1.T2/T3 → S2 uses):
  - git.New(repoDir): "the CLI's own git boundary for auto-stage (HasStagedChanges/AddAll/StagedFileCount)
    + the FR42 DiffTree report (RevParseHEAD for isUnborn; DiffTree(sha, isUnborn) post-commit)."

UI (forward — P1.M4.T3):
  - printCommitReport/printDryRunMessage: "S2 isolates the report in these functions; P1.M4.T3 restyles
    them (↳ decoration, color, TTY-aware) and adds progress lines (↳ Generating…). S2 = the DATA."
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation/edit - fix before proceeding
go build ./internal/cmd/ ./cmd/stagecoach/
gofmt -w internal/cmd/
go vet ./internal/cmd/

# Expected: zero errors. If `go build` complains runDefault is undefined, you forgot the root.go edit
# (Task 2) or named the function differently. gofmt rewrites formatting; govet reports none.
```

### Level 2: Unit/Integration Tests (the FULL CLI through a stub provider)

```bash
# The default action's tests drive the real cobra rootCmd through a stub agent in temp git repos.
go test -race ./internal/cmd/ -run TestRunDefault -v

# Full cmd package (S1's root_test.go + S2's default_action_test.go)
go test -race ./internal/cmd/ -v

# Expected: all green. If a test flakes, check rootCmd state restoration in t.Cleanup (singleton).
# Expected: TestRunDefault_Commit asserts HEAD moved + git log round-trips + stdout has [sha] subject.
# Expected: TestRunDefault_Rescue/Timeout/CAS assert exitcode.For(err) == 3/124/1 + the §18.3/§13.5 msgs.
```

### Level 3: Integration Testing (Binary Validation)

```bash
# Build the real binary
make build

# --- Manual smoke (optional; needs a real or stub agent on PATH/configured) ---
# Inside a scratch git repo with a staged change:
cd /tmp/scratchrepo && git init && git config user.name t && git config user.email t@e && \
  echo hi > a.txt && git add a.txt && /home/dustin/projects/stagecoach/bin/stagecoach
# Expected (with a configured agent): a commit is created; stdout shows [<7hex>] <subject> + "A  a.txt";
# exit 0. (With no agent: exit 1 "no provider configured…" — that's the generic-error path, expected.)

# Dry run (message only on stdout; pipeable)
/home/dustin/projects/stagecoach/bin/stagecoach --dry-run
# Expected (with an agent): the generated message on stdout, no commit, exit 0.

# Nothing staged on a clean tree → FR17 exit 2 (AutoStageAll default true → FR18(0)+FR17)
cd /tmp/cleanrepo && git init && git config user.name t && git config user.email t@e && \
  /home/dustin/projects/stagecoach/bin/stagecoach; echo "exit=$?"
# Expected: "stagecoach: Nothing to commit." (or the FR18(0 files) notice then it), exit 2.

# --no-auto-stage + nothing staged → FR19 exit 2
echo "Nothing to see" > /tmp/cleanrepo/b.txt && \
  /home/dustin/projects/stagecoach/bin/stagecoach --no-auto-stage; echo "exit=$?"
# Expected: "stagecoach: Nothing staged.", exit 2 (b.txt is unstaged; --no-auto-stage refuses to stage).

# Verify the §15.4 exit-code surface programmatically (no agent needed for the nothing-staged paths):
go test -race ./internal/cmd/ -run 'TestRunDefault_(NothingStaged_FR17|NoAutoStage_FR19)' -v
# Expected: exitcode.For(err) == 2 for both.
```

### Level 4: Domain-Specific Validation (Exit-Code + FR Contract)

```bash
# The full §15.4 matrix (0/1/2/3/124) + FR16–FR20 + FR42, through the CLI via a stub agent:
go test -race ./internal/cmd/ -run TestRunDefault -v
# Expected: Commit/RootCommit→0, NothingStaged_FR17/NoAutoStage_FR19→2, Rescue→3, Timeout→124, CAS→1.

# Confirm no regression across the WHOLE module (config/generate/git/provider/prompt/pkg/exitcode untouched)
go test -race ./...
# Expected: all green. If a generate/config test broke, S2 over-reached — recheck that ONLY
# internal/cmd/default_action.go + default_action_test.go are new and root.go's RunE line changed.
```

## Final Validation Checklist

### Technical Validation

- [ ] `make build` succeeds → `./bin/stagecoach` exists.
- [ ] `go test -race ./internal/cmd/` green (root_test.go + default_action_test.go).
- [ ] `go test -race ./...` green (NO regression elsewhere).
- [ ] `go vet ./...` clean; `gofmt -l internal/cmd/` empty.
- [ ] `git status` shows ONLY: new default_action.go, new default_action_test.go, edited root.go.

### Feature Validation

- [ ] `runDefault` is the root `RunE` (root.go one-line edit); reads `cmd.Context()` + `Config()`.
- [ ] `--all`/`-a` runs `git add -A` before `HasStagedChanges` (FR20).
- [ ] Nothing-staged FSM correct: FR19 (--no-auto-stage) / FR16+FR18+FR17 (AutoStageAll) / "Nothing to
      commit." (AutoStageAll off).
- [ ] FR18 notice `Nothing staged — staging all changes (N files).` to STDERR, verbatim (em-dash).
- [ ] Calls `stagecoach.GenerateCommit(ctx, Options{Provider,Model,Timeout,DryRun})` (PUBLIC API).
- [ ] Commit success → stdout `[<7hex>] <subject>` + DiffTree name-status list (FR42); DiffTree error
      post-commit is non-fatal (report without list, exit 0).
- [ ] Dry-run success → stdout = message ONLY; exit 0; no commit.
- [ ] `*RescueError` → FormatRescue to stderr + silent `exitcode.New(124|3, nil)` (timeout before rescue).
- [ ] `*CASError` → ce.Error() to stderr + silent `exitcode.New(1, nil)`.
- [ ] `ErrNothingToCommit`/generic → `exitcode.New(2|1, err)` (main prints "stagecoach: <msg>").
- [ ] `exitcode.For(err)` returns 0/1/2/3/124 across the test matrix; runDefault never calls os.Exit.

### Code Quality Validation

- [ ] Calls the PUBLIC `pkg/stagecoach.GenerateCommit` (US12 dogfood), NOT internal `CommitStaged`.
- [ ] stdout = result; stderr = notices + diagnostics (§15.5 pipe use case preserved).
- [ ] Silent `exitcode.New(code, nil)` after detailed prints (no main double-print).
- [ ] FR42 DiffTree computed by the CLI (isUnborn captured pre-GenerateCommit).
- [ ] Report formatting isolated (printCommitReport/printDryRunMessage) for P1.M4.T3 to restyle.
- [ ] No edits to internal/{config,generate,git,prompt,provider}, pkg/stagecoach, internal/exitcode.
- [ ] root.go edit is surgical (RunE field only); flag vars/PersistentPreRunE/Config/Execute untouched.

### Documentation & Deployment

- [ ] Every exported symbol S2 adds has a Go-doc comment (runDefault, handleGenError, shortSHA,
      printCommitReport, printDryRunMessage — unexported but documented for maintainers).
- [ ] Behavior is discoverable via `stagecoach --help` (S1) + the FR42/FR18 runtime messages (S2).
- [ ] Example sessions land in the README in P1.M5.T4 (S2 produces them at runtime; no doc file for S2).

---

## Anti-Patterns to Avoid

- ❌ Don't call `generate.CommitStaged` directly — call the PUBLIC `pkg/stagecoach.GenerateCommit` (US12
  dogfood; avoids reimplementing buildDeps/registry). The public Result drops Changes → compute DiffTree
  in the CLI.
- ❌ Don't call `os.Exit` inside runDefault/handleGenError — only `main` exits. Return an error; `main`
  maps it via `exitcode.For`.
- ❌ Don't return the original `*RescueError`/`*CASError` AFTER printing the detailed message — main would
  double-print. Return a SILENT `exitcode.New(code, nil)` (Err==nil → Error()=="" → main skips).
- ❌ Don't print the FR18 notice or rescue/CAS messages to STDOUT — stdout is the pipeable RESULT (commit
  report / dry-run message). Notices + diagnostics go to STDERR.
- ❌ Don't compute DiffTree's isRoot from a post-commit guess — capture `isUnborn` via `RevParseHEAD`
  BEFORE `GenerateCommit` (auto-stage doesn't change it). A root commit's DiffTree needs isRoot=true.
- ❌ Don't put the stub provider in a `--config` file for tests — `GenerateCommit` re-loads config via
  DISCOVERY (no ConfigPathOverride); use the repo-local `./.stagecoach.toml` so BOTH layers see it.
- ❌ Don't reimplement the auto-stage logic inside `GenerateCommit`/`CommitStaged` — auto-stage lives in
  the CLI layer (PRD §11.3). S2 owns it; the orchestrator never calls `git add`.
- ❌ Don't implement signal handling, color/TTY/verbose, the "↳" progress decoration, subcommands, or the
  dry-run output decorations — those are P1.M4.T2/T3/T4 and S3/S4. S2 reads `cmd.Context()`, prints the
  DATA, and leaves decoration to the UI layer.
- ❌ Don't edit any file under internal/{config,generate,git,prompt,provider}, pkg/stagecoach, or
  internal/exitcode — they are READ-ONLY upstream contracts (P1.M3.T5.S1 built pkg/stagecoach in parallel).
- ❌ Don't forget to restore rootCmd state in default_action_test.go's t.Cleanup — it's a package-level
  singleton reused across tests; leaking SetArgs/SetOut/SetErr/loadedCfg poisons siblings (and -race).
- ❌ Don't re-copy root_test.go's helpers (initRepo/setGitConfig/writeConfigFile/chdir/loadEnvSetup) — they
  are in the SAME package (`cmd`); call them directly. DO copy generate_test.go's writeFile/stageFile/
  commitRaw/headSHA/gitOut/runGit/shaRe (package-private to `generate`, unimportable).
