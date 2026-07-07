# P4.M1.T1.S1 — Research Findings

## Work item
Add `--commits`/`--single`/`--no-decompose`/`--max-commits` + per-role (`--planner-*`/`--stager-*`/`--arbiter-*`)
flags to `internal/cmd/root.go` and update `internal/cmd/default_action.go` so the default action routes to
the decompose pipeline (instead of `AddAll→GenerateCommit`) when nothing is staged.

## CRITICAL: sequencing / dependency graph (from tasks.json)

```
P4.M1.T1.S1  deps: ['P3.M4.T1.S1', 'P1.M3.T2.S1']   (this task)
P4.M2.T1.S1  deps: ['P3.M4.T1.S1']                   (public Decompose API — Planned, AFTER)
```

**P4.M1.T1.S1 does NOT depend on P4.M2.T1.S1.** Therefore `pkg/stagecoach.Decompose` does NOT exist when
this task is implemented. The contract text says "The decompose CLI path calls pkg/stagecoach.Decompose
(P4.M2.T1.S1)" — but that is a FORWARD REFERENCE that cannot compile yet.

### Resolution (verified)
The decompose CLI branch MUST call `internal/decompose.Decompose` DIRECTLY (building `decompose.Deps` via
`decompose.ResolveRoles` in the CLI layer). Verified import graph:
- `internal/decompose` imports NONE of `internal/cmd` / `pkg/stagecoach` (grep confirmed — only mentions are
  in doc comments). So `internal/cmd → internal/decompose` is cycle-free.
- `internal/cmd/default_action.go` already imports `pkg/stagecoach` (for `GenerateCommit`); adding the
  `internal/decompose` import is safe.

P4.M2.T1.S1 will later (a) add `pkg/stagecoach.Decompose` as a thin wrapper encapsulating the Deps
construction, and (b) SWAP this CLI call site to use it. The exit-code mapping, result printing, and
double-print-suppression logic this task writes transfer directly. **This is a documented coordination
point**, not a blocker.

## CRITICAL: the flag RESOLUTION layer is already done

`internal/config/load.go` `loadFlags()` (P1.M3.T2.S1) ALREADY reads every new flag via `fs.Changed`:

```go
// (load.go, already shipped)
if fs.Changed("commits")         { cfg.Commits    = fs.GetInt("commits") }
if fs.Changed("single")||fs.Changed("no-decompose") { cfg.Single = true }
if fs.Changed("max-commits")     { cfg.MaxCommits = fs.GetInt("max-commits") }
for role in [planner,stager,message,arbiter]:
    if fs.Changed(role+"-provider") { cfg.setRoleProvider(role, ...) }
    if fs.Changed(role+"-model")    { cfg.setRoleModel(role, ...) }
```

So this task ONLY needs to REGISTER the flags on the persistent flag set in `root.go init()` so that:
(1) they appear in `--help`, (2) cobra parses them, (3) `loadFlags` can read them via `fs.Changed`.

- `pflag.FlagSet.Changed(name)` is nil-safe for unregistered flags (`Lookup` returns nil → false). So
  registering only `--planner-*`/`--stager-*`/`--arbiter-*` (omitting `--message-*`) is safe — `loadFlags`
  skips `message` because `Changed("message-provider")==false`. The contract only names
  planner/stager/arbiter (message = global `--provider`/`--model`, FR-R2).
- `pf.StringVar(&flagVar, ...)` takes the address of the package var → that counts as a USE, so the
  `unused` (U1000) linter does NOT fire on config-backed flag package vars even though `loadFlags` reads
  via `fs.Get*` (this is exactly how the existing `flagProvider`/`flagModel`/... vars behave — verified:
  none of them are read directly outside `root.go`).

## Routing logic (FR-M1 / FR-M2)

Current `default_action.go` flow:
1. `flagAll` → `AddAll`.
2. `hasStaged = HasStagedChanges`.
3. `if !hasStaged { switch { case flagNoAutoStage: exit2; case cfg.AutoStageAll: AddAll+count+notice;
   default: exit2 } }`
4. staged → `RevParseHEAD` + validate provider + `GenerateCommit`.

NEW routing: decompose activates **iff** nothing staged AND working tree has changes AND decompose enabled.
Extract a pure helper `shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool`:
- false if `cfg==nil`, `cfg.Single`, `cfg.Commits==1` (--single/--no-decompose/--commits 1 ⇒ v1 path),
- false if `dryRun` (decompose is NOT dry-run aware; `--dry-run` honors the single-commit preview — FR49),
- else `cfg.AutoStageAll && !noAutoStage`.

In `runDefault`, inside `if !hasStaged`, BEFORE the existing switch:
```go
if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
    status, err := g.StatusPorcelain(ctx)        // FR-M1: working tree has changes?
    if err != nil { return ... }
    if status == "" { return exitcode.New(NothingToCommit, "Nothing to commit.") }
    return runDecompose(...)                       // NO AddAll — planner gets working-tree diff
}
```
`--single`/`--commits 1` ⇒ `shouldDecompose=false` ⇒ falls through to the existing `AutoStageAll→AddAll→
GenerateCommit` (old behavior). Something already staged ⇒ `hasStaged=true` ⇒ never enters this branch ⇒
old `GenerateCommit` (decompose never re-partitions a hand-staged index). `--all` ⇒ `AddAll` makes
`hasStaged=true` ⇒ single path. All consistent with the contract.

## Decompose entry: Deps construction (CLI layer)

`internal/decompose.Decompose(ctx, deps decompose.Deps) (DecomposeResult, error)`.
The orchestrator derives each role's MODEL from `deps.Config` via `config.ResolveRoleModel(role, cfg)`
(verified: planner.go:61, message.go:102, arbiter.go:81) — NOT from a Models field. So the CLI must pass
the resolved cfg as `deps.Config` and the post-ResolveRoles `RoleManifests` as `deps.Roles`.

```go
overrides, err := provider.DecodeUserOverrides(cfg.Providers)   // mirror pkg/stagecoach.buildDeps
reg := provider.NewRegistry(overrides)
roleManifests, _, err := decompose.ResolveRoles(*cfg, reg)       // 2nd return (RoleModels) unused by CLI
if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("decompose: %w", err)) }
deps := decompose.Deps{
    Git: g, Registry: reg, Config: *cfg, Roles: roleManifests,
    Verbose: ui.NewVerbose(stderr, cfg.Verbose),   // ui.NewVerbose(w io.Writer, on bool) *Verbose
    Out:     stderr,                                // rescue/CAS destination (P3.M4.T1.S2)
}
res, err := decompose.Decompose(ctx, deps)
```

`decompose.Deps` fields (roles.go): `Git git.Git`, `Registry *provider.Registry`, `Config config.Config`,
`Roles RoleManifests`, `Verbose *ui.Verbose`, `Out io.Writer`, `stager` seam (nil in prod). `ResolveRoles`
does install-checks + FR-R5b + FR-D4 stager fallback (its own validation — NOT the single-provider block).

## Exit-code mapping + double-print suppression

`exitcode.For(err)` ALREADY maps via `errors.Is` traversal: `ErrNothingToCommit→2`, `ErrTimeout→124`,
`ErrRescue→3`, `ErrCASFailed→1`, else 1. `*decompose.DecomposeRescueError` (P3.M4.T1.S2) Unwraps to
`*generate.RescueError` → so `errors.Is(err, ErrRescue/ErrTimeout)` works. `*generate.CASError` Unwraps to
`git.ErrCASFailed` → 1.

**P3.M4.T1.S2 DESIGN (must follow): the decompose LOOP prints the §18.3 rescue (`FormatRescueMulti`) and
the §13.5 CAS message (`ce.Error()`) to `deps.Out` (= stderr).** So the CLI must NOT re-print. The CLI
handler:
```go
func handleDecomposeError(err error) error {
    var re *generate.RescueError; var ce *generate.CASError
    if errors.As(err, &re) || errors.As(err, &ce) {   // DecomposeRescueError unwraps to *RescueError
        return exitcode.New(exitcode.For(err), nil)   // SILENT (loop printed) — main's ""-guard skips print
    }
    return exitcode.New(exitcode.Error, err)           // planner/safety/infra — main prints "stagecoach: …"
}
```
`errors.As(err, &re)` matches `*DecomposeRescueError` via its Unwrap chain WITHOUT the CLI naming/importing
the `DecomposeRescueError` type ⇒ the CLI compiles regardless of P3.M4.T1.S2 timing, and exit codes are
correct either way. **Dependency on S2's loop-printing:** by P4 implementation time S2 ("Implementing") is
complete (declared sequencing). If S2 had NOT landed, suppression would swallow an unprinted message —
but the orchestrator sequences S2 before P4 integration.

## Result printing

`decompose.CommitResult{SHA, Subject, Message, Files []git.FileChange}`. Print FR42 per commit:
`[<short-sha>] <subject>` + file list (mirror `printCommitReport`, but takes `decompose.CommitResult`).
On PARTIAL failure (FR-M12) Decompose returns `(DecomposeResult{Commits: 0..i-1}, err)` — print the landed
commits to stdout, then `handleDecomposeError` (rescue already on stderr). So:
```go
res, err := decompose.Decompose(ctx, deps)
for _, c := range res.Commits { printDecomposeCommit(stdout, c) }
if err != nil { return handleDecomposeError(err) }
return nil
```

## Signal

`main.go` calls `signal.Install(...)` ONCE before `Execute` (handler's `RescueFormat = generate.FormatRescue`,
the BASE form — correct for multi-commit per P3.M4.T1.S2). The decompose loop arms `SetSnapshot`/`ClearSnapshot`
per-concept (P3.M4.T1.S2). **No signal change in this task** — the signal-aware ctx flows via `cmd.Context()`.

## Testing strategy (full pipeline is covered in internal/decompose; CLI tests ROUTING+WIRING+MAPPING)

cmd tests use `rootCmd.SetOut/SetErr/SetArgs` + `Execute(context.Background())`; fixtures: `setupStubRepo`
(temp repo + stub provider in `.stagecoach.toml`), `stubtest.Build`. `TestFlags_RegisteredAndDefaults`
(root_test.go) is a table of {name, shorthand, defValue} asserting `pf.Lookup`.

1. **Flag registration** — extend the `TestFlags_RegisteredAndDefaults` table: commits(0), single(false),
   no-decompose(false), max-commits(12), planner/stager/arbiter-provider(""), *-model("").
2. **`shouldDecompose` pure unit test** — all combos (default→true; Single→false; Commits==1→false;
   Commits==3→true; dryRun→false; noAutoStage→false; AutoStageAll=false→false; nil→false).
3. **`handleDecomposeError` pure unit test** — synthesized *RescueError(ErrRescue)→code3,silent;
   *RescueError(ErrTimeout)→124,silent; *CASError→1,silent; ErrPlannerFailed-wrapped→1,printed.
4. **Execute-level --single opt-out** — dirty unstaged tree + stub provider + `--single` ⇒ single path ⇒
   AddAll ⇒ GenerateCommit ⇒ 1 new commit, err==nil. (Proves opt-out routing end-to-end.)
5. **Execute-level decompose routing entered** — dirty unstaged tree + stub provider (NO tooled_flags),
   bare `stagecoach` ⇒ shouldDecompose⇒runDecompose⇒ResolveRoles ⇒ stager fallback fails ⇒ err contains
   "stager"/"tooled", exitcode 1. (Proves decompose routing entered; GenerateCommit never calls
   ResolveRoles, so this error is unique to the decompose path. Full happy-path decompose needs a real
   tooled stager agent — NOT feasible with the bare stub harness — and is covered in internal/decompose.)

## Files touched

EDIT `internal/cmd/root.go` — register 10 flags in `init()` (vars + help strings, Mode A docs).
EDIT `internal/cmd/default_action.go` — `shouldDecompose`, `runDecompose`, `printDecomposeCommit`,
  `handleDecomposeError`; route in `runDefault`; add `internal/decompose` import.
EDIT `internal/cmd/root_test.go` — extend flag table.
EDIT `internal/cmd/default_action_test.go` — shouldDecompose/handleDecomposeError unit tests + 2 Execute
  routing tests.
go.mod/go.sum UNCHANGED. No new files. Coverage gate: cmd is NOT gated (internal/{git,provider,generate,
config} are). Validation: `go build ./... && go vet ./... && go test -race ./... && golangci-lint run &&
gofmt -l internal/ pkg/`.
