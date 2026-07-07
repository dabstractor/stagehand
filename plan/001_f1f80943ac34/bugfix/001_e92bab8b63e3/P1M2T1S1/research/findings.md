# P1.M2.T1.S1 — Research Findings (synthesis)

Full root-cause trace lives in `../architecture/seam_provider_preflight.md` and the
binding decision in `../architecture/decisions.md` (D2). This note captures only the
narrow facts the S1 implementer needs.

## The edit (single chokepoint)

`pkg/stagecoach/stagecoach.go` → `buildDeps` (line 154). Insert between `m.Validate()`
(line 182) and `return generate.Deps{...}` (line 186):

```go
if !reg.IsInstalled(m) {
    return generate.Deps{}, fmt.Errorf(
        "provider %q: command %q not found. Is the agent installed?",
        name, m.DetectCommand())
}
```

- `reg` (line 150) and `m` (line 180) are already in scope → no new imports. `fmt` already imported.
- `Registry.IsInstalled` (`internal/provider/registry.go:76`) does `exec.LookPath(m.DetectCommand())`.
- `Validate()` (line 182) guarantees `Command != nil && *Command != ""` → `DetectCommand()` returns a real string → genuine LookPath (not the empty-string short-circuit).

## Why this is correct (exit code)

- The error is a PLAIN `fmt.Errorf` — NOT `*generate.RescueError`, not a sentinel.
- `exitcode.For` (`internal/exitcode/exitcode.go`) falls through every `errors.Is` branch
  (NothingToCommit/Timeout/Rescue/CAS) → final `return Error` → **exit 1**.
- `internal/cmd/default_action.go` `handleGenError` generic branch → `exitcode.New(exitcode.Error, err)`
  → main prints `stagecoach: <msg>`. No rescue block.

## Why it fixes both pipelines AND leaves no dangling tree

`buildDeps` is called once by `GenerateCommit` (line 109) and its `Deps` feed BOTH:
- common path → `generate.CommitStaged` (the WriteTree-at-step-3 path), and
- advanced path → `runPipeline` (dry-run/SystemExtra; WriteTree at line 228).

The check returns BEFORE either pipeline runs, so it is strictly BEFORE any `WriteTree`.
→ no snapshot object, no armed rescue, no dangling tree. Satisfies PRD §18.2 "pre-generation".

## Regression safety (CRITICAL — verify)

Adding the check must NOT break existing tests. Reasoning (already proven by the green suite):
- The auto-detect branch (`cfg.Provider == ""`, buildDeps lines ~156-164) ALREADY calls
  `reg.IsInstalled(m)` over `reg.List()` (which includes the absolute-path stub binary), and the
  full `go test -race ./...` passes. `exec.LookPath` on an absolute executable path returns found.
- Therefore the explicit-path stub (`Provider: "stub"` with absolute `command`) also returns
  `IsInstalled == true` → no behavior change for every existing happy-path/dry-run/timeout test.
- Only manifests pointing at a genuinely-missing command flip to the new early-exit-1 path.

## Docs (Mode A — ride with this subtask, per decisions.md D6)

- `docs/cli.md` "Exit codes" table: exit-1 row already lists "agent missing"; affirm the
  **pre-generation** qualifier so it reads as a pre-snapshot fail-fast.
- `docs/how-it-works.md` "Failure modes and exit codes" table: ADD a row
  "Agent missing on $PATH → pre-generation → 1 (Error)" AHEAD of the snapshot-based rescue rows
  (it fires before WriteTree, so it precedes them logically).

## What S1 explicitly does NOT do (owned by S2)

- No new tests. `pkg/stagecoach/stagecoach_test.go` regression/missing-command tests are S2's scope
  (assert: NOT `*RescueError`, `exitcode.For(err)==1`, no dangling tree object).
