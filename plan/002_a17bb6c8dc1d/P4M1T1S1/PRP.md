---
name: "P4.M1.T1.S1 — Add --commits/--single/--no-decompose/--max-commits + per-role (--planner-*/--stager-*/--arbiter-*) flags to internal/cmd/root.go + route the default action (internal/cmd/default_action.go) to the multi-commit decompose pipeline when nothing is staged"
description: |

  EDIT `internal/cmd/root.go` (register 10 new persistent flags in `init()`), EDIT `internal/cmd/default_action.go`
  (add `shouldDecompose` + `runDecompose` + `printDecomposeCommit` + `handleDecomposeError`; route in
  `runDefault`; add the `internal/decompose` import), EDIT `internal/cmd/root_test.go` (extend the flag
  table), EDIT `internal/cmd/default_action_test.go` (add the routing/wiring/exit-mapping tests).

  CONTRACT (P4.M1.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: root.go's init() registers persistent flags. default_action.go implements the
       nothing-staged state machine (FR16-FR20). FR-M1: decomposition activates iff nothing staged AND
       working tree has changes. When something IS staged, the single-commit primitive (v1) runs
       unchanged. New flags per §15.2: --commits (int, default 0=auto), --single/--no-decompose (bool),
       --max-commits (int, default 12), --planner-provider/--planner-model, --stager-provider/--stager-model,
       --arbiter-provider/--arbiter-model. The default action currently calls stagecoach.GenerateCommit;
       when nothing is staged and !--single, it must call the decompose path instead.
    2. INPUT: config.Commits/Single/MaxCommits fields from P1.M3.T1.S1, the Decompose pipeline from
       P3.M4.T1.S1.
    3. LOGIC: In root.go init(): add IntVar for --commits (default 0), BoolVar for --single and
       --no-decompose, IntVar for --max-commits (default 12), StringVar pairs for --planner-provider/
       --planner-model, --stager-provider/--stager-model, --arbiter-provider/--arbiter-model. In
       default_action.go: after the nothing-staged state machine, if !flagNoAutoStage && cfg.AutoStageAll
       && !cfg.Single && cfg.Commits != 1 → instead of AddAll→GenerateCommit, route to the decompose path
       (AddAll is NOT called — the planner receives the working-tree diff). If --single or --commits==1 →
       old AddAll→GenerateCommit behavior. If something IS already staged → old GenerateCommit behavior
       (decompose never re-partitions a hand-staged index). The decompose CLI path calls pkg/stagecoach.Decompose
       (P4.M2.T1.S1).
    4. OUTPUT: `stagecoach` with nothing staged → decompose (auto or forced). `stagecoach --single` → old
       behavior. `stagecoach` with something staged → old behavior. All new flags work.
    5. DOCS: [Mode A] Update the root command's flag help strings to document the decompose flags. The
       changeset-level README update is in P4.M3.T1.S1.

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  CRITICAL DEVIATION FROM THE LITERAL CONTRACT TEXT (with full justification — READ THIS FIRST):
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  The contract clause "The decompose CLI path calls pkg/stagecoach.Decompose (P4.M2.T1.S1)" CANNOT be
  honored literally: the dependency graph in tasks.json is
      P4.M1.T1.S1  deps: ['P3.M4.T1.S1', 'P1.M3.T2.S1']
      P4.M2.T1.S1  deps: ['P3.M4.T1.S1']      ← the public Decompose API; Planned, runs AFTER this task
  i.e. P4.M1.T1.S1 does NOT depend on P4.M2.T1.S1, so `pkg/stagecoach.Decompose` DOES NOT EXIST when this
  task is implemented. Calling it would not compile, and the task's validation gate (`go build ./...`)
  would fail. Therefore the decompose CLI branch calls `internal/decompose.Decompose` DIRECTLY (building
  `decompose.Deps` via `decompose.ResolveRoles` in the CLI layer). This is safe: `internal/decompose`
  imports none of `internal/cmd` / `pkg/stagecoach` (verified — only doc-comment mentions), so
  `internal/cmd → internal/decompose` is import-cycle-free. P4.M2.T1.S1 will later add `pkg/stagecoach.Decompose`
  as a thin wrapper that encapsulates exactly this Deps construction, and SWAP this one CLI call site to
  use it — the exit-code mapping, FR42 result printing, and double-print-suppression logic written here
  transfer verbatim. This is a documented coordination point with P4.M2.T1.S1, NOT a blocker.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/*.go (Decompose, DecomposeResult, CommitResult, Deps, ResolveRoles,
      DecomposeRescueError, the runLoop + FR-M12 printing) — CONSUMED READ-ONLY. Produced by P3.M4.T1.S1/S2.
    - internal/config/load.go (loadFlags ALREADY reads every new flag via fs.Changed — P1.M3.T2.S1) —
      CONSUMED READ-ONLY. This task only REGISTERS flags so loadFlags can see them.
    - internal/config/config.go (Config.Commits/Single/MaxCommits/Roles + Defaults) — CONSUMED.
    - internal/git/git.go (StatusPorcelain, HasStagedChanges, AddAll, DiffTree, New, FileChange) — CONSUMED.
    - internal/provider/{registry.go,manifest.go} (NewRegistry, DecodeUserOverrides) — CONSUMED.
    - internal/ui/verbose.go (NewVerbose) + internal/ui/output.go (New) — CONSUMED.
    - internal/exitcode/exitcode.go (For, New, constants) — CONSUMED.
    - internal/generate/{generate.go,rescue.go} (RescueError, CASError, FormatRescue, ErrRescue,
      ErrTimeout) — CONSUMED.
    - internal/signal/signal.go (Install in main.go; loop arms SetSnapshot/ClearSnapshot) — CONSUMED.
    - pkg/stagecoach/stagecoach.go (GenerateCommit, Options, Result) — CONSUMED (single-commit path only).
    - cmd/stagecoach/main.go — UNCHANGED (signal.Install already wired; ctx flows via cmd.Context()).
    - PRD.md, tasks.json, .gitignore — NEVER modify (research agent; this task edits code only).

  DELIVERABLES (4 file EDITS — no new files):
    EDIT internal/cmd/root.go          — register 10 persistent flags in init() + doc the help strings.
    EDIT internal/cmd/default_action.go — shouldDecompose + runDecompose + printDecomposeCommit +
                                         handleDecomposeError + routing in runDefault + decompose import.
    EDIT internal/cmd/root_test.go     — extend TestFlags_RegisteredAndDefaults table.
    EDIT internal/cmd/default_action_test.go — shouldDecompose + handleDecomposeError unit tests +
                                         2 Execute-level routing tests (--single opt-out; decompose entered).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all 10 flags registered with correct
  type/default/help and appear in `stagecoach --help`; bare `stagecoach` with nothing staged + dirty tree
  routes to decompose (NO AddAll); `stagecoach --single` / `--commits 1` / `--no-decompose` + dirty tree
  routes to the v1 AddAll→GenerateCommit path; `stagecoach` with something already staged routes to
  GenerateCommit (unchanged); decompose rescue/CAS are printed ONCE (by the loop, not re-printed by the
  CLI); partial-failure runs print the landed commits to stdout and exit 3/124/1; `--dry-run` forces the
  single-commit preview path (decompose is not dry-run aware).

---

## Goal

**Feature Goal**: Wire the v2 multi-commit decomposition pipeline into the CLI default action (PRD §9.14
FR-M1/M2, §15.2, §13.6.1) and expose its control surface as persistent flags. Concretely: (1) register
the 10 §15.2 decompose/per-role flags on the root command's persistent flag set; (2) add the routing
decision so that `stagecoach` with NOTHING staged and a dirty working tree invokes the decompose pipeline
(`internal/decompose.Decompose`) instead of `AddAll→GenerateCommit`, while `--single`/`--no-decompose`/
`--commits 1`, an already-staged index, `--all`, and `--dry-run` all preserve the v1 single-commit
behavior; (3) construct `decompose.Deps` and resolve the four roles via `decompose.ResolveRoles` in the
CLI layer; (4) render the FR42 success report for every commit Decompose produced (including partial
landings on FR-M12 isolation) and map the pipeline's typed errors to §15.4 exit codes WITHOUT
double-printing the rescue/CAS message the loop already emitted.

**Deliverable** (4 file EDITS — no new files):
1. `internal/cmd/root.go` (EDIT) — 10 new persistent flags in `init()` (`--commits`, `--single`,
   `--no-decompose`, `--max-commits`, `--planner-{provider,model}`, `--stager-{provider,model}`,
   `--arbiter-{provider,model}`) with Mode-A help strings.
2. `internal/cmd/default_action.go` (EDIT) — `shouldDecompose` (pure routing predicate), `runDecompose`
   (builds `Deps` + calls `decompose.Decompose` + prints results), `printDecomposeCommit` (FR42 per
   commit), `handleDecomposeError` (exit-code map + double-print suppression), and the routing hook inside
   `runDefault`; add `internal/decompose` import.
3. `internal/cmd/root_test.go` (EDIT) — extend the `TestFlags_RegisteredAndDefaults` table.
4. `internal/cmd/default_action_test.go` (EDIT) — `shouldDecompose` + `handleDecomposeError` unit tests
   + two `Execute`-level routing tests (`--single` opt-out succeeds; bare routing enters the decompose
   path).

**Success Definition**:
- **Flags**: `stagecoach --help` lists all 10 new flags with the correct type and default (`--commits` int
  default `0`, `--single`/`--no-decompose` bool default `false`, `--max-commits` int default `12`, the six
  per-role strings default `""`). `pf.Lookup(name)` returns each with the expected `DefValue`. Passing
  `--commits 3`, `--single`, `--no-decompose`, `--max-commits 20`, `--planner-model X`, etc. sets the
  corresponding `cfg` field (verified via `loadFlags`, already shipped).
- **Routing — decompose (FR-M1)**: a repo with a committed file PLUS uncommitted/un-staged changes,
  invoked with bare `stagecoach` (no `--single`), enters the decompose path — provable because, with only a
  non-tooled stub provider configured, `decompose.ResolveRoles` fails the stager role (no
  `tooled_flags`-capable fallback) and `Execute` returns a non-nil error whose message names `stager`/
  `tooled` with exit code 1. (`AddAll` is NOT called — the working-tree diff reaches the planner.)
- **Routing — opt-out (FR-M2c)**: the SAME dirty/un-staged tree with `--single` (also `--commits 1`,
  also `--no-decompose`) takes the v1 path: `AddAll` → `GenerateCommit` → exactly ONE new commit, `err==nil`.
- **Routing — staged (FR-M1)**: a repo with changes ALREADY `git add`-ed runs `GenerateCommit`
  unchanged (no decompose) — covered by existing tests; this task must not regress it.
- **Routing — dry-run**: `stagecoach --dry-run` with a dirty/un-staged tree does NOT decompose (decompose
  commits and is not dry-run aware); it falls to the single-commit auto-stage→preview path.
- **Exit mapping + no double print**: a decompose `*RescueError` (incl. `*DecomposeRescueError`) maps to
  exit 3 (rescue) / 124 (timeout) and is printed ONCE (by the loop to stderr; `handleDecomposeError`
  returns a SILENT `exitcode.New(code, nil)`); a `*CASError` maps to exit 1 (silent); a planner/safety/
  infra error maps to exit 1 and main prints `stagecoach: <msg>`.
- **Partial landing (FR-M12)**: when Decompose returns `(DecomposeResult{Commits: 0..i-1}, err)`, the CLI
  prints each landed commit's FR42 line to stdout, then maps the error's exit code (rescue already on stderr).
- `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the end user running `stagecoach` at the shell (the "plan-holder" persona, PRD §7.1) and,
transitively, the lazygit integration (PRD §15.5 customCommand `key: '<c-a>'`).

**Use Case**: the user finishes a stretch of work touching several unrelated concerns and runs `stagecoach`
on a dirty working tree with nothing staged. Instead of one sprawling commit, stagecoach decomposes the
changes into logically-coherent commits. When the user wants the old one-commit behavior (a quick
checkpoint), they pass `--single`. When they know the count, `--commits N`. When they want a different
model for planning than for messages, `--planner-model …`.

**User Journey**: (1) `stagecoach` (nothing staged, dirty tree) → decompose runs; (2) stagecoach prints
`[<sha>] <subject>` + file list for each landed concept to stdout; (3) on success exit 0; on a per-concept
failure exit 3/124/1 with the rescue recipe already on stderr and the landed commits on stdout; (4) the
user passes `--single` next time to force one commit, or `--commits 3` to skip the "how many?" step.

**Pain Points Addressed**: (a) the default action no longer forces a single mega-commit on a dirty tree;
(b) the v1 behavior is one flag (`--single`) away for users who want it; (c) per-role model routing is
discoverable via `--help` and overridable per-invocation.

## Why

- **Business value**: this is the user-facing integration of the v2 core (PRD §10.3, G11). The decompose
  pipeline (P3) is useless without CLI routing — this task makes `stagecoach` actually decompose by default.
  It is the single change that flips the default action from "one commit" to "the right number of commits."
- **Integration with existing features**: consumes `internal/decompose.Decompose` + `ResolveRoles`
  (P3.M4.T1.S1/S2), the already-shipped `loadFlags` (P1.M3.T2.S1) which reads every new flag via
  `fs.Changed`, and `config.Config.{Commits,Single,MaxCommits,Roles}` (P1.M3.T1.S1). It reuses the
  `exitcode.For` mapping and the FR42 report shape. It is the seam the PRD §15.1 synopsis promises
  ("With no command, runs the default action: commit staged changes (auto-staging all if nothing is
  staged…)"), now extended to decompose when nothing is staged.
- **Problems this solves and for whom**: the plan-holder (§7.1) wants coherent history without manual
  `git add -p` choreography. This task makes decomposition the default for the common "dirty tree, nothing
  staged" case while keeping every v1 escape hatch (`--single`, staging first, `--all`, `--dry-run`).

## What

**User-visible behavior**:
- `stagecoach` (nothing staged, dirty tree) → decompose into N commits (auto-count via the planner, or
  forced via `--commits N`); each landed commit printed as `[<short-sha>] <subject>` + file list; exit 0.
- `stagecoach --single` / `--no-decompose` / `--commits 1` (nothing staged) → v1 `AddAll`→one commit.
- `stagecoach` with something already staged → v1 `GenerateCommit` (decompose never re-partitions a
  hand-staged index — PRD FR-M1).
- `stagecoach --all` (nothing staged) → `AddAll` stages everything → v1 single commit (`--all` is a
  single-commit intent; it stages before the staged check, so `hasStaged` is true).
- `stagecoach --dry-run` (nothing staged) → single-commit auto-stage→preview (decompose is not dry-run
  aware; `--dry-run` is honored as a no-commit preview, FR49).
- On a per-concept failure mid-decompose (FR-M12), the commits that landed (0..i-1) are printed to stdout;
  the §18.3 multi-commit rescue (or §13.5 CAS) message is printed to stderr by the loop; the exit code is
  3 (rescue) / 124 (timeout) / 1 (CAS or planner/safety/infra). The rescue is printed exactly once.

**Technical requirements**: 10 new persistent flags in `root.go init()`; a pure `shouldDecompose` routing
predicate; `runDecompose` building `decompose.Deps` (`Git`, `Registry`, `Config`, `Roles` from
`ResolveRoles`, `Verbose` from `ui.NewVerbose`, `Out` = stderr) and calling `decompose.Decompose`; FR42
per-commit printing; `handleDecomposeError` mapping exit codes via `exitcode.For` and suppressing the
re-print for rescue/CAS. The decompose call uses `internal/decompose.Decompose` directly (NOT
`pkg/stagecoach.Decompose`, which does not exist yet — see the CRITICAL DEVIATION note above).

### Success Criteria

- [ ] All 10 flags registered on `rootCmd.PersistentFlags()` with correct type + default + Mode-A help:
      `--commits`(int,0), `--single`(bool,false), `--no-decompose`(bool,false), `--max-commits`(int,12),
      `--planner-provider`/`--planner-model`(str,""), `--stager-provider`/`--stager-model`(str,""),
      `--arbiter-provider`/`--arbiter-model`(str,""). They appear in `stagecoach --help`.
- [ ] `shouldDecompose(cfg, dryRun, noAutoStage)` returns the FR-M1/M2 truth table exactly (see Task 2).
- [ ] Bare `stagecoach` on a dirty/un-staged tree (with a non-tooled stub provider) enters the decompose
      path — asserted via the unique `ResolveRoles` stager-fallback error (exit 1, message names `stager`).
- [ ] `--single` (and `--commits 1`, `--no-decompose`) on the same tree takes the v1 path → 1 new commit,
      `err==nil`.
- [ ] An already-staged index runs `GenerateCommit` unchanged (existing tests stay green).
- [ ] `--dry-run` forces the single-commit preview path (no decompose).
- [ ] `handleDecomposeError`: `*RescueError`(ErrRescue)→silent exit 3; `*RescueError`(ErrTimeout)→silent
      124; `*CASError`→silent 1; planner/safety/infra→exit 1 + main prints.
- [ ] Partial DecomposeResult.Commits are printed to stdout; the rescue/CAS is printed once (by the loop).
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ — YES. Every consumed symbol is named with its exact signature + file; the exact insertion
point in `runDefault` (inside `if !hasStaged`, before the existing `switch`) is given; the exact
`shouldDecompose` truth table, the `Deps` literal fields, the `handleDecomposeError` body, the
`printDecomposeCommit` format, the flag-registration idioms (vars + `&flagVar` use to satisfy the `unused`
linter), the two non-obvious decisions (call `internal/decompose.Decompose` not the not-yet-existing
public API; suppress re-print because the loop already prints), the test harness (`rootCmd.SetArgs`+
`Execute`, `setupStubRepo`, the `TestFlags_RegisteredAndDefaults` table), and the validation gates are all
specified below. The subtle points — that `loadFlags` ALREADY reads these flags so this task only
REGISTERS them, that `Changed()` is nil-safe for the omitted `--message-*`, that the orchestrator derives
models from `deps.Config` (so the CLI passes `*cfg`), that `errors.As(err,&re)` matches
`DecomposeRescueError` without naming it, and that `--dry-run`/`--all`/staged index all bypass decompose —
are explained, not just named.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: PRD.md §9.14 (FR-M1 … FR-M12) — the decompose trigger + modes
  why: "FR-M1 (trigger: nothing staged AND working tree has changes), FR-M2 (modes: auto / --commits N /
        --single≡--no-decompose≡--commits 1), FR-M4 (max_commits safety cap, default 12). These define the
        routing predicate and the flags."
  critical: "FR-M1: 'If anything is staged, the single-commit primitive runs unchanged; stagecoach never
        re-partitions a hand-staged index.' So decompose routing lives ONLY in the nothing-staged branch.
        FR-M2c: '--commits 1 ≡ --single' (config.Load already normalizes Commits==1 ⇒ Single=true)."

- url: PRD.md §15.2 (Global flags table) — the exact flag surface to register
  why: "Authoritative flag names, env vars, git-config keys, defaults, and one-line descriptions for
        --commits/--single/--no-decompose/--max-commits and the --planner-*/--stager-*/--arbiter-* pairs.
        Mode-A docs = mirror these descriptions into the cobra help strings."
  critical: "Note --message-* is ABSENT from §15.2 (message role = global --provider/--model, FR-R2). Do
        NOT register --message-provider/--message-model. loadFlags' loop over [planner,stager,message,
        arbiter] is safe because fs.Changed is nil-safe for the unregistered message flags."

- url: PRD.md §15.4 (Exit codes) + §18.3 (rescue) + §13.6.6/§18.2 (decompose failure modes)
  why: "Exit 0 success / 1 error / 2 nothing / 3 rescue / 124 timeout. Decompose rescue/CAS come from the
        loop; the CLI maps via exitcode.For and suppresses re-print."
  critical: "The decompose LOOP (P3.M4.T1.S2) prints FormatRescueMulti + ce.Error() to deps.Out. The CLI
        must return a SILENT exitcode.New(code,nil) for those (main's `err.Error() != \"\"` guard skips
        printing). Planner/safety/infra errors are NOT pre-printed → main prints them."

- url: PRD.md §9.15 (FR-R1–R5) + §16.4 — per-role provider/model
  why: "Four roles; planner/stager/arbiter get flags, message inherits global. Precedence flag>env>
        per-role config>global config>manifest default — all handled by config.Load/loadFlags (shipped)."
  critical: "This task does NOT implement resolution — it only registers the flags. Resolution is done."

# CODEBASE FILES — pattern sources + consumed dependencies (all verified, paths exact)
- file: internal/cmd/root.go   # EDIT TARGET — register 10 flags in init()
  why: "init() registers the persistent flags (§15.2). The config-backed flags (provider/model/config/
        timeout/verbose/no-color) are bound to package vars AND read by loadFlags via fs.Changed. The
        behavioral flags (all/no-auto-stage/dry-run) are package vars read directly by runDefault."
  pattern: "Add a new var block for the 10 config-backed decompose/per-role flags (bound vars; loadFlags
        reads them via fs — the &flagVar address-counts-as-use so the `unused` linter is satisfied, exactly
        as the existing flagProvider/flagModel vars do). In init(), after the existing flag registrations,
        add pf.IntVar/BoolVar/StringVar calls with Mode-A help strings."
  gotcha: "Do NOT add package-level vars that are then UNUSED — bind each flag's &var in init() (that is
        the use). The routing reads cfg.Commits/Single (from loadFlags) and the existing flagDryRun/
        flagNoAutoStage behavioral vars — NOT the new bound vars. Keep the bound vars' comment noting
        loadFlags is the reader."

- file: internal/cmd/default_action.go   # EDIT TARGET — routing + runDecompose + helpers
  why: "runDefault is the default action. It has the §9.4 auto-stage state machine (FR16-FR20) inside
        `if !hasStaged { switch {...} }`, then the staged→GenerateCommit path. The decompose hook goes
        inside `if !hasStaged`, BEFORE the switch. handleGenError shows the §15.4 exit-mapping pattern
        (silent exitcode.New for rescue/CAS; exitcode.New(Error,err) for generic). printCommitReport +
        shortSHA show the FR42 report shape."
  pattern: "Mirror handleGenError's structure in handleDecomposeError, but SUPPRESS the print for rescue/
        CAS (the loop printed) — return exitcode.New(exitcode.For(err), nil). Mirror printCommitReport in
        printDecomposeCommit (input = decompose.CommitResult). Reuse shortSHA."
  gotcha: "default_action.go imports pkg/stagecoach, generate, git, provider, ui, exitcode. ADD
        'github.com/dustin/stagecoach/internal/decompose'. The single-path provider-validation block (reg.Get)
        is NOT reused by decompose — ResolveRoles does its own install-check + FR-R5b + FR-D4 fallback.
        cfg is *config.Config here; decompose.Deps.Config + ResolveRoles take a config.Config VALUE →
        dereference with *cfg."

- file: internal/config/load.go   # CONSUMED (read-only) — loadFlags ALREADY reads the new flags
  why: "loadFlags already has: fs.Changed('commits')→cfg.Commits; ('single'||'no-decompose')→cfg.Single=true;
        ('max-commits')→cfg.MaxCommits; and a loop over roleNames=[planner,stager,message,arbiter] doing
        Changed(role+'-provider'/'-model')→setRoleProvider/setRoleModel. Load() also normalizes
        Commits==1⇒Single=true. This is why this task ONLY registers flags."
  pattern: "No edit. Just ensure the registered flag NAMES exactly match what loadFlags looks up:
        'commits','single','no-decompose','max-commits','planner-provider','planner-model','stager-provider',
        'stager-model','arbiter-provider','arbiter-model'."
  gotcha: "fs.Changed(name) is nil-safe for unregistered names (Lookup→nil→false), so omitting
        --message-* is safe. The flag DEFAULT you register is cosmetic for --help (loadFlags only reads
        via fs.Changed, so an unset flag leaves cfg at the config-layer value). Register defaults that
        match config.Defaults(): commits=0, single=false, no-decompose=false, max-commits=12, per-role=''."

- file: internal/decompose/decompose.go   # CONSUMED — Decompose signature + DecomposeResult/CommitResult
  why: "Decompose(ctx, deps Deps) (DecomposeResult, error). DecomposeResult{Commits []CommitResult, Amended
        int}. CommitResult{SHA, Subject, Message string; Files []git.FileChange}. Decompose REQUIRES
        deps.Roles populated (ResolveRoles) and assumes the caller routed correctly (nothing staged, dirty
        tree). Mode routing INSIDE Decompose: Config.Single||Config.Commits==1 → runSingleEscape (but the
        CLI never reaches that because shouldDecompose is false for those — the CLI uses the v1
        GenerateCommit path instead, matching the contract)."
  pattern: "Call decompose.Decompose(ctx, deps); range res.Commits for FR42; on err handleDecomposeError."
  gotcha: "Decompose's PRECONDITION is the CLI's responsibility (FR-M1) — it does NOT re-check
        HasStagedChanges. The CLI must guarantee nothing-is-staged + dirty-tree before calling it
        (shouldDecompose + StatusPorcelain check). Do not call Decompose when something is staged."

- file: internal/decompose/roles.go   # CONSUMED — Deps struct + ResolveRoles
  why: "Deps{Git git.Git; Registry *provider.Registry; Config config.Config; Roles RoleManifests; Verbose
        *ui.Verbose; Out io.Writer; stager seam(nil in prod)}. ResolveRoles(cfg config.Config, reg
        *provider.Registry) (RoleManifests, RoleModels, error) — does install-checks, FR-R5b, FR-D4 stager
        fallback. The orchestrator derives each role's MODEL from deps.Config via
        config.ResolveRoleModel(role,cfg) (verified planner.go:61/message.go:102/arbiter.go:81) — so the
        CLI passes *cfg as deps.Config; RoleModels (2nd return) is UNUSED by the CLI (discard with _)."
  pattern: "overrides,_:=provider.DecodeUserOverrides(cfg.Providers); reg:=provider.NewRegistry(overrides);
        rm,_,err:=decompose.ResolveRoles(*cfg,reg); deps:=decompose.Deps{Git:g,Registry:reg,Config:*cfg,
        Roles:rm,Verbose:ui.NewVerbose(stderr,cfg.Verbose),Out:stderr}."
  gotcha: "ResolveRoles is the decompose path's validation — it returns a clear error (e.g. 'role \"stager\":
        provider … cannot stage (tooled_flags empty) …') which the CLI surfaces as exit 1. Do NOT run the
        single-path provider-validation block (reg.Get(cfg.Provider)) for decompose."

- file: internal/generate/generate.go + rescue.go   # CONSUMED — RescueError, CASError, FormatRescue
  why: "*generate.RescueError{Kind,TreeSHA,ParentSHA,Candidate,Cause} (Unwrap→Kind ∈ {ErrRescue,ErrTimeout}).
        *generate.CASError (Unwrap→git.ErrCASFailed; Error() IS the §13.5 message). exitcode.For maps
        errors.Is(ErrTimeout)→124, errors.Is(ErrRescue)→3, errors.Is(ErrCASFailed)→1."
  pattern: "handleDecomposeError: var re *generate.RescueError; var ce *generate.CASError;
        if errors.As(err,&re)||errors.As(err,&ce){ return exitcode.New(exitcode.For(err),nil) }."
  gotcha: "*decompose.DecomposeRescueError (P3.M4.T1.S2) Unwraps to *generate.RescueError, so
        errors.As(err,&re) matches it WITHOUT the CLI importing/naming the type — the CLI compiles and
        maps exit codes correctly regardless of DecomposeRescueError's existence. Do NOT import it."

- file: internal/exitcode/exitcode.go   # CONSUMED — For + New + constants
  why: "exitcode.For(err): nil→0; *ExitError→Code; ErrNothingToCommit→2; ErrTimeout→124; ErrRescue→3;
        ErrCASFailed→1; else 1. exitcode.New(code,err)*ExitError (err may be nil → Error()==''). main prints
        'stagecoach: <err>' only when err.Error()!=''."
  pattern: "Silent non-zero exit: exitcode.New(code, nil). Printed exit 1: exitcode.New(exitcode.Error, err)."

- file: internal/ui/verbose.go + output.go   # CONSUMED — NewVerbose, New, UI.Progress
  why: "ui.NewVerbose(w io.Writer, on bool) *Verbose (the deps.Verbose type). ui.New(stdout,stderr,color)
        *UI (already constructed in runDefault as u). u.Progress(label) prints the ↳ progress to stderr."
  pattern: "deps.Verbose = ui.NewVerbose(stderr, cfg.Verbose). Optional decompose progress label via u.Progress."

- file: internal/provider/registry.go   # CONSUMED — NewRegistry + DecodeUserOverrides
  why: "provider.DecodeUserOverrides(cfg.Providers map[string]map[string]any) (map[string]Manifest, error);
        provider.NewRegistry(map[string]Manifest) *Registry. Mirror pkg/stagecoach.buildDeps."

- file: internal/cmd/root_test.go   # EDIT TARGET — extend TestFlags_RegisteredAndDefaults
  why: "Table-driven {name, shorthand, defValue} asserting pf.Lookup(name)!=nil + Shorthand + DefValue.
        Add rows for the 10 new flags."
  pattern: "Append to requiredFlags: {'commits','','0'}, {'single','','false'}, {'no-decompose','','false'},
        {'max-commits','','12'}, {'planner-provider','',''}, {'planner-model','',''},
        {'stager-provider','',''}, {'stager-model','',''}, {'arbiter-provider','',''}, {'arbiter-model',''}."
  gotcha: "DefValue is the STRING form pflag stores ('0','false','12',''). resetFlags() between Executes
        clears Changed — existing harness handles this."

- file: internal/cmd/default_action_test.go   # EDIT TARGET — add shouldDecompose/handleDecomposeError + routing
  why: "setupStubRepo(t, stubOut) builds a temp repo + .stagecoach.toml stub provider + STAGECOACH_STUB_OUT;
        tests use rootCmd.SetOut/SetErr/SetArgs + Execute(context.Background()). saveRootState/
        restoreRootState + resetFlags isolate runs. shaRe/headSHA/runGit helpers exist."
  pattern: "shouldDecompose unit test: plain func, no git — pass &config.Config{} variants. handleDecomposeError
        unit test: synthesize *generate.RescueError{Kind:…}, *generate.CASError{}, and a wrapped
        decompose.ErrPlannerFailed-style error; assert *exitcode.ExitError Code + Err nil/non-nil. Routing
        Execute tests: dirty/un-staged tree + stub provider; --single → 1 commit err==nil; bare → err
        non-nil, message contains 'stager', exitcode.For(err)==1."
  gotcha: "A full happy-path decompose Execute test is NOT feasible — the real stager needs a tooled agent
        that runs git, and the stubtest harness (stubagent) is BARE. Cover the pipeline in internal/decompose
        (already done). The CLI tests cover ROUTING + WIRING + EXIT MAPPING only. For the 'decompose entered'
        assertion, the stub provider has no tooled_flags → ResolveRoles' stager fallback fails → the error
        is UNIQUE to the decompose path (GenerateCommit never calls ResolveRoles)."
```

### Current Codebase tree (relevant subset)

```bash
internal/cmd/
  root.go              # EDIT — register 10 flags in init()
  root_test.go         # EDIT — extend TestFlags_RegisteredAndDefaults
  default_action.go    # EDIT — shouldDecompose + runDecompose + printDecomposeCommit + handleDecomposeError + routing
  default_action_test.go # EDIT — unit + Execute routing tests
  config.go  config_test.go  providers.go  providers_test.go   # CONSUMED (unchanged)
internal/decompose/    # CONSUMED (read-only) — Decompose, DecomposeResult, CommitResult, Deps, ResolveRoles,
                       #   DecomposeRescueError, runLoop+FR-M12 printing (P3.M4.T1.S1/S2)
internal/config/       # CONSUMED — Config.{Commits,Single,MaxCommits,Roles}, loadFlags (already reads flags), Load
internal/git/git.go    # CONSUMED — New, HasStagedChanges, StatusPorcelain, AddAll, DiffTree, FileChange
internal/provider/     # CONSUMED — NewRegistry, DecodeUserOverrides
internal/ui/           # CONSUMED — NewVerbose, New, UI.Progress
internal/exitcode/     # CONSUMED — For, New, constants
internal/generate/     # CONSUMED — RescueError, CASError, FormatRescue, ErrRescue, ErrTimeout
internal/signal/       # CONSUMED — Install (in main.go); loop arms SetSnapshot/ClearSnapshot
pkg/stagecoach/         # CONSUMED — GenerateCommit (single path only; Decompose is P4.M2.T1.S1)
cmd/stagecoach/main.go  # CONSUMED (unchanged) — signal.Install already wired
```

### Desired Codebase tree (files this task EDITS — no new files)

```bash
internal/cmd/root.go                # +10 persistent flags in init() (+ bound vars + Mode-A help strings)
internal/cmd/default_action.go      # +shouldDecompose +runDecompose +printDecomposeCommit +handleDecomposeError
                                    #  +routing hook in runDefault + import "internal/decompose"
internal/cmd/root_test.go           # +10 rows in TestFlags_RegisteredAndDefaults
internal/cmd/default_action_test.go # +TestShouldDecompose +TestHandleDecomposeError +TestRouting_SingleOptOut
                                    #  +TestRouting_DecomposeEntered
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-DECOMPOSE-NOT-PUBLIC (CRITICAL): pkg/stagecoach.Decompose does NOT exist (it is P4.M2.T1.S1, which
//   does not precede this task — see tasks.json deps). The decompose CLI branch MUST call
//   internal/decompose.Decompose directly. internal/decompose imports no cmd/pkg-stagecoach → no cycle.
//   P4.M2.T1.S1 swaps this one call site to the public wrapper later. Do NOT add a public Decompose to
//   pkg/stagecoach here (that is P4.M2.T1.S1's deliverable — encroaching breaks the plan boundary).

// G-LOADFLAGS-ALREADY-DONE (CRITICAL): config/load.go loadFlags ALREADY reads --commits/--single/
//   --no-decompose/--max-commits and the per-role flags via fs.Changed. This task only REGISTERS them.
//   Do NOT re-implement flag→cfg resolution. Verify the registered names EXACTLY match loadFlags' lookups.

// G-NO-MESSAGE-FLAG: §15.2 has NO --message-provider/--message-model (message = global, FR-R2). Omit them.
//   loadFlags loops roleNames incl. "message" but fs.Changed("message-provider") is nil-safe (Lookup→nil
//   →false) for the unregistered flag. Registering them would be scope creep + contradicts §15.2.

// G-BOUND-VAR-LINTER: pf.StringVar(&flagVar,...) takes the address → that is a USE, so the `unused` (U1000)
//   linter does NOT fire on the bound package vars even though loadFlags (not the var) is the reader. This
//   is exactly how the existing flagProvider/flagModel/flagConfig/flagTimeout/flagVerbose/flagNoColor behave
//   (none read directly outside root.go — verified). Add a comment on the var block noting loadFlags reads
//   them via fs, so a future reader isn't confused by "unused-looking" vars.

// G-CFG-IS-PTR: In runDefault cfg is *config.Config (from Config()). decompose.Deps.Config and
//   decompose.ResolveRoles take a config.Config VALUE → dereference with *cfg. cfg may be nil only if
//   PersistentPreRunE was skipped (already guarded at the top of runDefault with an early return).

// G-DECOMPOSE-PRECONDITION: Decompose assumes the caller routed correctly (nothing staged + dirty tree).
//   It does NOT re-check. The CLI guarantees this via shouldDecompose + a StatusPorcelain("") check.
//   AddAll is NOT called on the decompose path (the planner receives the working-tree diff via
//   WorkingTreeDiff inside callPlanner). Calling AddAll would defeat decomposition.

// G-DRY-RUN-FORCES-SINGLE: Decompose is NOT dry-run aware (it commits). To honor FR49 (no-commit preview),
//   --dry-run must NOT decompose. shouldDecompose returns false when dryRun. So `stagecoach --dry-run` on a
//   dirty tree → single auto-stage→GenerateCommit(dry-run) path. Documented decision (contract is silent).

// G-DOUBLE-PRINT (CRITICAL): The decompose loop (P3.M4.T1.S2) prints FormatRescueMulti (rescue) and
//   ce.Error() (CAS) to deps.Out (= stderr). handleDecomposeError MUST return a SILENT exitcode.New(code,nil)
//   for *RescueError/*CASError so main does not re-print. For planner/safety/infra (NOT pre-printed) return
//   exitcode.New(exitcode.Error, err) so main prints 'stagecoach: <msg>'. errors.As(err,&re) matches
//   *DecomposeRescueError via its Unwrap→*RescueError — no need to import/name DecomposeRescueError.

// G-EXITCODE-FOR-TRAVERSAL: exitcode.For already does errors.Is traversal (ErrTimeout→124, ErrRescue→3,
//   ErrCASFailed→1). For *RescueError/*CASError/*DecomposeRescueError it returns the right code. Use
//   exitcode.For(err) for the silent cases; exitcode.Error (1) for the generic case.

// G-PARTIAL-RESULT: On FR-M12 partial failure Decompose returns (DecomposeResult{Commits:0..i-1}, err).
//   Print those landed commits' FR42 lines to stdout (they are real, in git log), THEN handleDecomposeError.
//   Do not swallow them. On full success print all commits then return nil.

// G-STAGED-INDEX-BYPASS: If hasStaged is true the code never enters the !hasStaged branch → decompose is
//   never considered → GenerateCommit runs (FR-M1: never re-partition a hand-staged index). --all does
//   AddAll BEFORE the check → hasStaged true (unless clean) → single path. Both are correct by construction.

// G-SIGNAL-ALREADY-INSTALLED: main.go calls signal.Install once; the handler's RescueFormat is
//   generate.FormatRescue (base form — correct for multi-commit per P3.M4.T1.S2). The loop arms
//   SetSnapshot/ClearSnapshot per-concept. This task makes NO signal changes. ctx = cmd.Context() (set by
//   Execute from main's signal-aware ctx) flows into runDecompose.

// G-CLI-TEST-CANNOT-RUN-REAL-STAGER: The stubtest harness (stubagent) is a BARE agent (canned output, no
//   tools). The real decompose stager needs a tooled agent that runs `git add`. So a full happy-path
//   decompose Execute test is impossible. Cover ROUTING (shouldDecompose), MAPPING (handleDecomposeError),
//   and ROUTING-ENTERED (ResolveRoles stager-fallback error is unique to decompose). The pipeline itself
//   is covered in internal/decompose via the stager seam.
```

## Implementation Blueprint

### Data models and structure

No new data models — this task consumes `config.Config`, `decompose.Deps`/`DecomposeResult`/`CommitResult`,
`generate.RescueError`/`CASError`, and `exitcode.ExitError` verbatim. The only new symbols are four
unexported functions in `internal/cmd/default_action.go`:

```go
// shouldDecompose is the FR-M1/M2 routing predicate (PURE — no I/O, no package-flag reads). True iff the
// default action should route to multi-commit decomposition instead of the v1 AddAll→GenerateCommit path.
// Decompose activates iff NOTHING is staged (caller guarantees via hasStaged), the working tree has changes
// (caller checks StatusPorcelain), auto-stage-all is on, the user did not opt out (--single/--no-decompose/
// --commits 1 ⇒ cfg.Single), and --dry-run is not set (decompose commits; --dry-run honors the preview).
func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool

// runDecompose builds decompose.Deps (ResolveRoles for the four roles) and runs the pipeline. Prints each
// landed commit's FR42 report to stdout (including partial landings on FR-M12), then maps the error.
// Calls internal/decompose.Decompose directly (pkg/stagecoach.Decompose is P4.M2.T1.S1 — not yet shipped).
func runDecompose(ctx context.Context, stdout, stderr io.Writer, u *ui.UI, cfg *config.Config, g git.Git) error

// printDecomposeCommit renders the FR42 success line for one decompose.CommitResult to w (stdout):
// `[<short-sha>] <subject>` then one line per file (mirrors printCommitReport; input type differs).
func printDecomposeCommit(w io.Writer, c decompose.CommitResult)

// handleDecomposeError maps a decompose error to the §15.4 outcome WITHOUT double-printing: rescue/CAS are
// already printed by the loop → silent exitcode.New(exitcode.For(err), nil); planner/safety/infra →
// exitcode.New(exitcode.Error, err) so main prints 'stagecoach: <msg>'.
func handleDecomposeError(err error) error
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/root.go — register 10 persistent flags in init()
  - ADD a var block (alongside the existing flagProvider/flagModel block) for the config-backed decompose/
    per-role flags. Comment that loadFlags (config/load.go) reads them via fs.Changed, so the bound vars are
    not read directly here (the &flagVar address is their use — satisfies the `unused` linter):
      var (
          flagCommits        int
          flagSingle         bool
          flagNoDecompose    bool
          flagMaxCommits     int
          flagPlannerProvider string
          flagPlannerModel    string
          flagStagerProvider  string
          flagStagerModel     string
          flagArbiterProvider string
          flagArbiterModel    string
      )
  - IN init(), after the existing flag registrations, ADD (Mode-A help strings mirroring PRD §15.2):
      pf.IntVar(&flagCommits, "commits", 0,
          "Force exactly N commits when nothing is staged (skips the planner's count decision; 0 = auto-decompose). 1 ≡ --single (env/git stagecoach.commits)")
      pf.BoolVar(&flagSingle, "single", false,
          "Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --no-decompose)")
      pf.BoolVar(&flagNoDecompose, "no-decompose", false,
          "Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --single)")
      pf.IntVar(&flagMaxCommits, "max-commits", 12,
          "Safety cap on auto-decompose commit count (env/git stagecoach.max_commits)")
      pf.StringVar(&flagPlannerProvider, "planner-provider", "",
          "Per-role provider override for the decomposition planner (env STAGECOACH_PLANNER_PROVIDER; git stagecoach.role.planner)")
      pf.StringVar(&flagPlannerModel, "planner-model", "",
          "Per-role model override for the decomposition planner (env STAGECOACH_PLANNER_MODEL; git stagecoach.role.planner)")
      pf.StringVar(&flagStagerProvider, "stager-provider", "",
          "Per-role provider override for the (tooled) staging agent (env STAGECOACH_STAGER_PROVIDER; git stagecoach.role.stager)")
      pf.StringVar(&flagStagerModel, "stager-model", "",
          "Per-role model override for the (tooled) staging agent (env STAGECOACH_STAGER_MODEL; git stagecoach.role.stager)")
      pf.StringVar(&flagArbiterProvider, "arbiter-provider", "",
          "Per-role provider override for the leftover arbiter (env STAGECOACH_ARBITER_PROVIDER; git stagecoach.role.arbiter)")
      pf.StringVar(&flagArbiterModel, "arbiter-model", "",
          "Per-role model override for the leftover arbiter (env STAGECOACH_ARBITER_MODEL; git stagecoach.role.arbiter)")
  - DO NOT register --message-provider/--message-model (§15.2 has none; message = global --provider/--model).
  - FOLLOW pattern: the existing pf.StringVar/BoolVar/IntVar/BoolVarP calls in init().
  - PRESERVE: every existing flag, PersistentPreRunE, Execute, Config, shouldSkipConfigLoad unchanged.
  - NAMING: snake-kebab flag names exactly matching loadFlags' fs.Changed lookups + §15.2.
  - PLACEMENT: internal/cmd/root.go init().

Task 2: EDIT internal/cmd/default_action.go — shouldDecompose + routing hook
  - ADD import "github.com/dustin/stagecoach/internal/decompose".
  - ADD func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool:
        if cfg == nil { return false }
        if cfg.Single || cfg.Commits == 1 { return false }   // --single/--no-decompose/--commits 1 → v1
        if dryRun { return false }                            // decompose commits; --dry-run → single preview
        return cfg.AutoStageAll && !noAutoStage               // FR-M1 trigger context (auto-stage on)
  - IN runDefault, INSIDE `if !hasStaged { ... }`, BEFORE the existing `switch`, ADD the decompose hook:
        // FR-M1 (P4.M1.T1.S1): nothing staged + dirty tree + decompose enabled → decompose (NO AddAll).
        if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
            status, err := g.StatusPorcelain(ctx)
            if err != nil {
                return exitcode.New(exitcode.Error, fmt.Errorf("git status --porcelain: %w", err))
            }
            if status == "" {
                return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // clean tree
            }
            return runDecompose(ctx, stdout, stderr, u, cfg, g) // planner gets the working-tree diff
        }
  - PRESERVE: the existing switch (flagNoAutoStage / cfg.AutoStageAddAll AddAll+count+notice / default) and
    the staged→GenerateCommit path UNCHANGED. When shouldDecompose is false (--single/--dry-run/etc.) the
    flow falls through to the switch exactly as before.
  - FOLLOW pattern: the existing exitcode.New(...)+errors.New(...) idiom; g.StatusPorcelain signature
    `(ctx) (output string, err error)`.

Task 3: EDIT internal/cmd/default_action.go — runDecompose + printDecomposeCommit + handleDecomposeError
  - ADD func runDecompose(ctx, stdout, stderr io.Writer, u *ui.UI, cfg *config.Config, g git.Git) error:
        overrides, err := provider.DecodeUserOverrides(cfg.Providers)
        if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("decompose: provider overrides: %w", err)) }
        reg := provider.NewRegistry(overrides)
        roleManifests, _, err := decompose.ResolveRoles(*cfg, reg)   // RoleModels (2nd) unused by CLI
        if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("decompose: %w", err)) }
        deps := decompose.Deps{
            Git: g, Registry: reg, Config: *cfg, Roles: roleManifests,
            Verbose: ui.NewVerbose(stderr, cfg.Verbose),
            Out:     stderr,   // the loop prints §18.3 rescue + §13.5 CAS here (P3.M4.T1.S2)
        }
        // Optional progress label (UX consistency with the single path); best-effort, non-essential.
        label := "Decomposing"
        if cfg.Provider != "" { label += " with " + cfg.Provider }
        label += "…"
        u.Progress(label)
        res, derr := decompose.Decompose(ctx, deps)
        for _, c := range res.Commits {     // print landed commits (success AND FR-M12 partial)
            printDecomposeCommit(stdout, c)
        }
        if derr != nil {
            return handleDecomposeError(derr) // suppress re-print; map exit code
        }
        return nil
  - ADD func printDecomposeCommit(w io.Writer, c decompose.CommitResult):
        fmt.Fprintf(w, "[%s] %s\n", shortSHA(c.SHA), c.Subject)
        for _, f := range c.Files {
            if f.SrcPath != "" {   // R/C rename/copy
                fmt.Fprintf(w, "%s  %s → %s\n", f.Status, f.SrcPath, f.Path)
                continue
            }
            fmt.Fprintf(w, "%s  %s\n", f.Status, f.Path)
        }
    (mirrors printCommitReport; reuses shortSHA. Input is decompose.CommitResult, not stagecoach.Result.)
  - ADD func handleDecomposeError(err error) error:
        var re *generate.RescueError
        var ce *generate.CASError
        if errors.As(err, &re) || errors.As(err, &ce) {   // *DecomposeRescueError unwraps to *RescueError
            return exitcode.New(exitcode.For(err), nil)    // SILENT — loop already printed to stderr
        }
        return exitcode.New(exitcode.Error, err)           // planner/safety/infra — main prints
  - FOLLOW pattern: handleGenError (exitcode.New + errors.As), printCommitReport (FR42 shape), shortSHA.
  - PRESERVE: handleGenError, runDefault's single-commit path, all existing helpers unchanged.
  - NAMING: unexported funcs; snakeCase-free Go (shouldDecompose, runDecompose, printDecomposeCommit,
    handleDecomposeError).
  - PLACEMENT: internal/cmd/default_action.go (sibling to handleGenError / printCommitReport).

Task 4: EDIT internal/cmd/root_test.go — extend the flag-registration table
  - IN TestFlags_RegisteredAndDefaults, APPEND to requiredFlags:
      {"commits","","0"}, {"single","","false"}, {"no-decompose","","false"}, {"max-commits","","12"},
      {"planner-provider","",""}, {"planner-model","",""}, {"stager-provider","",""},
      {"stager-model","",""}, {"arbiter-provider","",""}, {"arbiter-model",""}
  - DO NOT add {"message-provider",...}/{"message-model",...} (not registered; Lookup==nil would fail the test).
  - FOLLOW pattern: the existing {name, shorthand, defValue} rows + pf.Lookup assertions.
  - PLACEMENT: internal/cmd/root_test.go (TestFlags_RegisteredAndDefaults).

Task 5: EDIT internal/cmd/default_action_test.go — unit + Execute routing tests
  - ADD TestShouldDecompose (PURE — no git, no Execute). Table over (cfg, dryRun, noAutoStage, want):
      default {AutoStageAll:true} dryRun=false noAuto=false → true
      {Single:true} → false ; {Commits:1} → false ; {Commits:3} → true (forced count still decomposes)
      dryRun=true → false ; noAutoStage=true → false ; {AutoStageAll:false} → false ; cfg=nil → false
  - ADD TestHandleDecomposeError (PURE). Build errors + assert the returned *exitcode.ExitError:
      &generate.RescueError{Kind:generate.ErrRescue}  → Code==3,  Err==nil (silent)
      &generate.RescueError{Kind:generate.ErrTimeout} → Code==124, Err==nil
      &generate.CASError{...}                          → Code==1,  Err==nil
      fmt.Errorf("%w: boom", decompose.ErrPlannerFailed) → Code==1, Err!=nil (main prints)
      (Optionally a *decompose.DecomposeRescueError{Rescue:&generate.RescueError{Kind:ErrRescue},...}
       → Code==3, Err==nil — proves the Unwrap traversal. This requires importing decompose, which the
       test file may already; if not, the errors.As(&re) path is proven by the bare *RescueError row.)
  - ADD TestRouting_SingleOptOut (Execute-level): setupStubRepo; commit a file; write+leave an un-staged
      change (do NOT git add); rootCmd.SetArgs([]string{"--single"}); Execute(ctx). Assert err==nil AND
      git log count increased by exactly 1 (the v1 AddAll→GenerateCommit path ran). restoreRootState after.
  - ADD TestRouting_DecomposeEntered (Execute-level): setupStubRepo; commit a file; write+leave an un-staged
      change; rootCmd.SetArgs([]string{}) (bare — no --single); Execute(ctx). Assert err!=nil AND
      exitcode.For(err)==1 AND err.Error() contains "stager" (the stub provider has no tooled_flags →
      ResolveRoles' stager fallback fails; this error is UNIQUE to the decompose path). This proves the
      default action routed to decompose (GenerateCommit never calls ResolveRoles).
  - FOLLOW pattern: setupStubRepo + rootCmd.SetOut/SetErr/SetArgs + Execute(context.Background()) +
    saveRootState/restoreRootState + resetFlags; headSHA/runGit/gitOut for assertions.
  - GOTCHA: between Executes, resetFlags(rootCmd.Flags()) + resetFlags(rootCmd.PersistentFlags()) clear
    Changed (else a prior --single persists). The existing restoreRootState does this — use it.
  - GOTCHA: setupStubRepo COMMITS .stagecoach.toml; leave the NEW change un-staged (write the file, do NOT
    git add). The stub provider returns a canned commit message via STAGECOACH_STUB_OUT.
  - COVERAGE: routing predicate (all branches), exit mapping (3/124/1/generic + silent vs printed),
    --single opt-out (1 commit), decompose-entered (unique ResolveRoles error). No full-pipeline test
    (impossible with the bare stub — covered in internal/decompose).
  - PLACEMENT: internal/cmd/default_action_test.go.
```

### Implementation Patterns & Key Details

```go
// Routing hook inside runDefault (Task 2) — the ONLY change to runDefault's flow:
//
//   hasStaged, err := g.HasStagedChanges(ctx)
//   if err != nil { ... }
//
//   if !hasStaged {
//       // NEW (P4.M1.T1.S1): decompose routing — checked before the single-commit auto-stage machine.
//       if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
//           status, err := g.StatusPorcelain(ctx)
//           if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("git status --porcelain: %w", err)) }
//           if status == "" {
//               return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
//           }
//           return runDecompose(ctx, stdout, stderr, u, cfg, g) // NO AddAll — planner sees the working-tree diff
//       }
//       switch { /* existing FR16-FR20 single-commit machine — UNCHANGED */ }
//   }
//   // staged → RevParseHEAD + validate + GenerateCommit — UNCHANGED

// Why StatusPorcelain for the dirty-tree check (not AddAll+StagedFileCount): the decompose path must NOT
// AddAll (the planner consumes the working-tree diff). StatusPorcelain("") == clean tree ⇒ exit 2 (FR17).

// Why errors.As (not type assertion) in handleDecomposeError: *decompose.DecomposeRescueError wraps
// *generate.RescueError as a FIELD + Unwraps to it. errors.As(err, &re) traverses the chain, so the CLI
// matches it WITHOUT importing/naming DecomposeRescueError — the CLI compiles regardless of P3.M4.T1.S2.

// Why discard ResolveRoles' 2nd return (RoleModels): the orchestrator re-derives each role's MODEL from
// deps.Config via config.ResolveRoleModel(role, cfg) (planner.go:61/message.go:102/arbiter.go:81). The CLI
// passes *cfg as deps.Config, so the cfg's resolved per-role/global overrides flow through. RoleModels is
// only needed by tests that bypass ResolveRoles (dcmDeps sets Roles directly).
```

### Integration Points

```yaml
CONFIG:
  - no new fields. config.Config.{Commits,Single,MaxCommits,Roles} already exist (P1.M3.T1.S1);
    loadFlags already reads the flags (P1.M3.T2.S1); Load normalizes Commits==1⇒Single. This task only
    REGISTERS the flags so loadFlags can see them.
FLAGS (internal/cmd/root.go):
  - add to: rootCmd.PersistentFlags() in init()
  - pattern: pf.IntVar/BoolVar/StringVar bound to package vars (loadFlags reads via fs.Changed)
ROUTES (internal/cmd/default_action.go):
  - add to: runDefault, inside `if !hasStaged`, before the existing switch
  - pattern: shouldDecompose gate → StatusPorcelain dirty-check → runDecompose (else fall through to v1)
DECOMPOSE (internal/decompose): CONSUMED — Decompose, ResolveRoles, Deps, CommitResult, DecomposeResult.
EXITCODE (internal/exitcode): CONSUMED — For(err) + New(code,nil/err). No changes.
GENERATE (internal/generate): CONSUMED — RescueError, CASError, FormatRescue, ErrRescue, ErrTimeout.
SIGNAL (internal/signal): CONSUMED — Install in main.go (unchanged); loop arms Set/Clear. No changes here.
PUBLIC API (pkg/stagecoach): CONSUMED — GenerateCommit (single path). Decompose is P4.M2.T1.S1 (not yet) →
  the CLI calls internal/decompose.Decompose directly; P4.M2.T1.S1 swaps this call site later.
MAIN (cmd/stagecoach/main.go): UNCHANGED (signal-aware ctx already flows via cmd.Context()).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file edit — fix before proceeding.
go build ./...                       # compile (10 flags + shouldDecompose/runDecompose/printDecomposeCommit/
                                     #   handleDecomposeError + the internal/decompose import)
go vet ./...                         # shadowed vars, printf, unkeyed literals
gofmt -l internal/ pkg/              # MUST print nothing
golangci-lint run                    # repo linter (Makefile `make lint`) — includes `unused`

# Scope-specific quick check:
go build ./internal/cmd/... && go vet ./internal/cmd/...

# Expected: zero errors. Verify: the new internal/decompose import in default_action.go is USED (runDecompose
# references decompose.Decompose/ResolveRoles/Deps/CommitResult) — else `unused import`. Verify the bound flag
# vars are addressed (&flagCommits etc.) in init() so `unused` (U1000) does not fire. Verify NO --message-*
# flag was registered (and NO test row for it).
```

### Level 2: Unit & Routing Tests (Component Validation)

```bash
# Flag registration:
go test -race ./internal/cmd/ -run 'TestFlags_RegisteredAndDefaults' -v

# Pure routing + exit-mapping:
go test -race ./internal/cmd/ -run 'TestShouldDecompose|TestHandleDecomposeError' -v

# Execute-level routing:
go test -race ./internal/cmd/ -run 'TestRouting_SingleOptOut|TestRouting_DecomposeEntered' -v

# Full cmd package (regression — existing default-action/provider/config tests stay green):
go test -race ./internal/cmd/...

# Whole suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If TestFlags_RegisteredAndDefaults fails on a flag, the name in root.go init() does
# not match the test row (or §15.2/loadFlags) — fix the name. If TestRouting_DecomposeEntered passes with
# err==nil, decompose was NOT entered (--single/default routed to GenerateCommit) — verify the tree is
# dirty+un-staged and shouldDecompose returned true. If TestRouting_SingleOptOut makes 0 commits,
# shouldDecompose leaked true for --single (check cfg.Single normalization). If `unused` fires on a bound
# flag var, the var's address was not taken in init() (use &flagVar).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and exercise the real CLI surface (smoke; agents stubbed where needed).
go build -o /tmp/stagecoach ./cmd/stagecoach

# Help documents all new flags (Mode A):
/tmp/stagecoach --help 2>&1 | grep -E -- '--commits|--single|--no-decompose|--max-commits|--planner-|--stager-|--arbiter-'
# Expected: all 10 flag lines present with their help strings.

# --single preserves v1 (in a throwaway repo with an un-staged change + a stub/default provider, this makes
# exactly one commit; full E2E with a real provider is environment-dependent — the unit tests are authoritative).
# (Manual; CI relies on TestRouting_SingleOptOut.)

# Coverage gate (cmd is NOT gated; these are — confirm no regression from the new import surface):
make coverage-gate      # enforces >=85% on internal/{git,provider,generate,config}

# Expected: PASS. This task edits only internal/cmd (not gated) + tests; it must not lower a gated package's
# coverage. (No new exported symbols in gated packages → no coverage impact.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Routing-truth invariant (TestShouldDecompose is authoritative; this is the spec it encodes):
#   staged index          → never decompose (hasStaged true skips the branch)
#   nothing staged + clean tree → exit 2 (StatusPorcelain "")
#   nothing staged + dirty + default flags → DECOMPOSE (no AddAll)
#   nothing staged + dirty + --single/--no-decompose/--commits 1 → v1 single
#   nothing staged + dirty + --dry-run → v1 single preview (no decompose)
#   nothing staged + dirty + --all → AddAll (hasStaged true) → v1 single

# Exit-mapping invariant (TestHandleDecomposeError encodes this):
#   *RescueError(ErrRescue)            → exit 3  (silent; loop printed FormatRescueMulti)
#   *RescueError(ErrTimeout)           → exit 124 (silent)
#   *CASError                          → exit 1  (silent; loop printed ce.Error())
#   *DecomposeRescueError(…ErrRescue)  → exit 3  (silent; errors.As traversal)
#   ErrPlannerFailed-wrapped / safety  → exit 1  (main prints 'stagecoach: <msg>')

# No-double-print invariant: on a decompose rescue, the rescue block appears EXACTLY ONCE on stderr
# (printed by the loop), and main prints NOTHING (exitcode.New(code,nil) → Error()=="" → guard skips).
# Assert in TestRouting_* by capturing stderr and counting the rescue separator / "Tree ID:" lines.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (10 flags; shouldDecompose/runDecompose/printDecomposeCommit/handleDecomposeError;
      the `internal/decompose` import is used).
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` green (Makefile `make test`).
- [ ] `golangci-lint run` clean (Makefile `make lint`) — `unused` does NOT fire on the bound flag vars.
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (no gated-package regression).
- [ ] go.mod/go.sum UNCHANGED (config/decompose/git/provider/ui/exitcode/generate/signal/stagecoach all imported).

### Feature Validation

- [ ] All 10 flags registered with correct type/default/help and visible in `stagecoach --help`.
- [ ] `TestFlags_RegisteredAndDefaults` extended with the 10 rows (and NO --message-* row) — passes.
- [ ] `shouldDecompose` returns the FR-M1/M2 truth table exactly (TestShouldDecompose green).
- [ ] Bare `stagecoach` (dirty/un-staged) enters decompose (TestRouting_DecomposeEntered: err names "stager", exit 1).
- [ ] `--single` (dirty/un-staged) takes v1 → 1 commit, err==nil (TestRouting_SingleOptOut green).
- [ ] Already-staged index runs GenerateCommit unchanged (existing default-action tests green).
- [ ] `--dry-run` forces the single-commit preview (shouldDecompose false when dryRun).
- [ ] `handleDecomposeError`: rescue→silent 3/124; CAS→silent 1; planner/infra→printed 1 (TestHandleDecomposeError green).
- [ ] Partial DecomposeResult.Commits printed to stdout; rescue/CAS printed once (by the loop, not re-printed).

### Code Quality Validation

- [ ] Follows existing cmd patterns (cobra persistent flags, exitcode.New idiom, FR42 report shape, saveRootState harness).
- [ ] File placement matches the desired tree (edits to root.go, default_action.go, root_test.go, default_action_test.go only).
- [ ] Anti-patterns avoided (see below): no public-API encroachment, no re-resolution of flags, no double-print.
- [ ] The bound flag vars are addressed in init() (use = `&flagVar`); no `unused` findings.
- [ ] shouldDecompose is PURE (params only; no package-var reads) → unit-testable without Execute/git.
- [ ] handleDecomposeError does NOT import/name decompose.DecomposeRescueError (uses errors.As to *generate.RescueError).

### Documentation & Deployment

- [ ] Mode-A help strings on all 10 flags (mirror PRD §15.2 descriptions; reference env/git-config keys).
- [ ] Doc comments on shouldDecompose, runDecompose, printDecomposeCommit, handleDecomposeError citing FR-M1/M2, §15.4, §18.3.
- [ ] A code comment at the decompose call site noting it calls internal/decompose.Decompose directly because
      pkg/stagecoach.Decompose is P4.M2.T1.S1 (not yet shipped) — the coordination point for the later swap.
- [ ] No new environment variables or config keys (all already shipped via loadFlags/Defaults).
- [ ] The changeset-level README/docs update is P4.M3.T1.S1 (NOT this task — do not edit README.md).

---

## Anti-Patterns to Avoid

- ❌ Don't call `pkg/stagecoach.Decompose` — it doesn't exist yet (P4.M2.T1.S1, not a dependency). Call
  `internal/decompose.Decompose` directly. Don't add a public `Decompose` to pkg/stagecoach (P4.M2.T1.S1's job).
- ❌ Don't re-implement flag→cfg resolution — `loadFlags` (config/load.go) already reads every new flag via
  `fs.Changed`. This task only REGISTERS the flags. Re-resolving would double-apply / desync with the 7-layer precedence.
- ❌ Don't register `--message-provider`/`--message-model` — §15.2 has none (message = global, FR-R2).
  loadFlags' loop is nil-safe for the unregistered name; a test row for it would falsely fail.
- ❌ Don't AddAll on the decompose path — the planner consumes the working-tree diff (FR-M1). AddAll would
  collapse everything into the index and defeat decomposition. (The v1 AddAll stays in the single path.)
- ❌ Don't double-print the rescue/CAS — the loop (P3.M4.T1.S2) already printed to deps.Out (stderr).
  handleDecomposeError returns a SILENT exitcode.New(code,nil) for those. Re-printing duplicates the recipe.
- ❌ Don't let `--dry-run` decompose — Decompose commits and is not dry-run aware. shouldDecompose returns
  false for dryRun so `--dry-run` honors the single-commit preview (FR49).
- ❌ Don't run the single-path provider-validation block (reg.Get(cfg.Provider)) for decompose —
  `decompose.ResolveRoles` does its own install-checks + FR-R5b + FR-D4 stager fallback for all four roles.
- ❌ Don't import/name `decompose.DecomposeRescueError` in the CLI — use `errors.As(err, &re)` (re is
  `*generate.RescueError`); the Unwrap chain matches DecomposeRescueError without a hard type dependency
  (keeps the CLI compiling regardless of P3.M4.T1.S2 timing).
- ❌ Don't pass `*config.Config` (pointer) to `decompose.Deps.Config`/`ResolveRoles` — they take a
  `config.Config` VALUE. Dereference: `*cfg`.
- ❌ Don't forget to dereference ResolveRoles' 2nd return — the CLI discards RoleModels (`_, `); the
  orchestrator re-derives models from deps.Config. Passing RoleModels into Deps would be ignored (no field).
- ❌ Don't write a full happy-path decompose Execute test with the stub harness — the stub agent is BARE
  (no tools) and can't run the tooled stager. Cover routing+mapping+entered at the CLI; the pipeline is
  tested in internal/decompose (stager seam).
- ❌ Don't change the existing switch / staged→GenerateCommit path / handleGenError / main.go / signal —
  only ADD the decompose hook + helpers + flags. Regressing the v1 path breaks existing tests + FR-M1's
  "staged index runs unchanged" guarantee.
