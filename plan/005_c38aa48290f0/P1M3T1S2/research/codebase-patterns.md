# P1.M3.T1.S2 — Codebase research notes

## Contract inherited from P1.M3.T1.S1 (assume implemented exactly)

`internal/hook/script.go` (same package this subtask extends):

```go
const Marker = "# stagecoach prepare-commit-msg hook v1"     // EXPORTED — status/uninstall detection
const ScriptMode os.FileMode = 0o755                        // EXPORTED — file mode to write
func hookScript(strict bool) string                         // UNEXPORTED — same-package access only
```

`hookScript(false)` →
```
#!/bin/sh
# stagecoach prepare-commit-msg hook v1
exec stagecoach hook exec "$@"
```
`hookScript(true)` last line → `exec stagecoach hook exec --strict "$@"`.

`internal/git/git.go` gains `HooksPath(ctx context.Context) (string, error)` on the `Git` interface +
`*gitRunner` — returns the **absolute** hooks dir via `git rev-parse --git-path hooks` (honors
`core.hooksPath` + worktrees). Non-repo → error (exit 128). `git.New(workDir) Git` is the constructor
(`git.go:269`).

## Cobra registration pattern (copy providers.go / config.go verbatim)

- Command group var with **no RunE** → bare `stagecoach hook` prints help (cobra default).
- Leaves are `var xCmd = &cobra.Command{ Use, Short, Long, Args, SilenceErrors:true, SilenceUsage:true, RunE }`.
- `func init()` does `parent.AddCommand(leaf...)` then `rootCmd.AddCommand(parent)` — **ZERO edits to root.go**
  (design "parallel-safe"; providers.go:68-72, config.go). A sibling subtask (P1.M3.T2.S1 `hook exec`) will
  add its own leaf to `hookCmd` from a separate file in the same package — so `hookCmd` must be a
  package-level var.
- RunE returns `nil` on success or `exitcode.New(exitcode.Error, err)` on failure; **never `os.Exit`**.
  `main()` maps the returned error via `exitcode.For`.
- Write user output to `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (testable), not `os.Stdout`.

## exit codes (internal/exitcode/exitcode.go)

`Success=0`, `Error=1`, `NothingToCommit=2`, `Rescue=3`, `Timeout=124`. Foreign-hook refusal = **exit 1**
(FR-H2). `exitcode.New(exitcode.Error, nil)` = a **silent** exit 1 (ExitError.Error()=="" → main skips
printing) — use it when the command already printed its own message to stderr (foreign refusal). Same trick
as default_action.go:33.

## CRITICAL: config-load side effect — hook group must skip config

`root.PersistentPreRunE` calls `config.Load`, which on first run **auto-writes a bootstrap config file**
(FR-B3, load.go:103-109: `bootstrapWriteConfig(globalPath)`). `hook install|uninstall|status` need only the
repo's hooks dir — never the resolved provider config — and a read-only `hook status` must not silently
create `~/.config/stagecoach/config.toml`.

**Solution (self-contained, no root.go edit, no shouldSkipConfigLoad edit, no name collision):** give
`hookCmd` its own `PersistentPreRunE: func(*cobra.Command, []string) error { return nil }`. Cobra runs only
the **nearest** PersistentPreRunE in the command chain, so this overrides root's for the whole hook group.
(config.go's leaves instead rely on root's `shouldSkipConfigLoad` name check — but adding "install"/
"uninstall"/"status" names there risks colliding with future `integrate install|remove` (§9.21), so the
group-level override is cleaner.)

Consequence for T2: `hook exec` inherits this no-op PreRun too and must load its own config in RunE — which
suits FR-H5's never-block semantics anyway. That is T2's concern, not this subtask's.

## Getting the hooks dir in a command

Commands read cwd via `os.Getwd()` then `git.New(repoDir)` (default_action.go:50-52). So:
```go
repoDir, _ := os.Getwd(); dir, err := git.New(repoDir).HooksPath(ctx)
```
Cmd-level tests must therefore `t.Chdir(tempRepo)` (Go 1.24 testing.T.Chdir) — see default_action_test.go
for the temp-repo-in-cwd pattern. `install --print` does NOT touch disk or cwd, so it works anywhere.

## Docs surface (Mode A → docs/cli.md)

`docs/cli.md` has a `## Subcommands` section (line 59) with `### providers list`, `### config init`, etc.
Add a `### hook install` / `### hook uninstall` / `### hook status` block documenting `--print`, `--strict`,
and the foreign-hook never-clobber policy. Exit-code table (line 131) already covers 1.

## FR mapping

- FR-H1: install writes executable `prepare-commit-msg` with Marker; per-repo (HooksPath), never global.
- FR-H2: foreign → refuse exit 1 + print manual `exec stagecoach hook exec "$@"` line; no `--force`;
  `install --print` → script to stdout.
- FR-H3: uninstall removes only when Marker present; status reports `none` / `stagecoach (v1)` / `foreign`.
- FR-H5: `install --strict` bakes `--strict` into the script body (via `hookScript(true)`).
