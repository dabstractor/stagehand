# Research — Git.HooksPath() resolution + POSIX hook script (P1.M3.T1.S1)

## Source of truth
- `plan/005_c38aa48290f0/architecture/external_deps.md` §3 — VERIFIED (git-scm.com + git 2.54.0).
- PRD §9.20 FR-H1 (install), FR-H5 (`--strict`), Appendix E #15 (git-for-windows POSIX portability).
- `internal/git/git.go` — the `run()` / `gitRunner` seam every method already uses.

## §3 verified facts (verbatim distillation)
- `git rev-parse --git-path hooks` **honors `core.hooksPath`** and, from a **linked worktree**, returns the
  **common dir's** hooks path (correct — hooks are shared; installing from any worktree targets the shared dir).
- **From a subdirectory it returns a RELATIVE path** → must be resolved to absolute before use.
- The hook is **NOT suppressed by `--no-verify`**, and a non-zero exit **aborts the commit** (why FR-H5's
  never-block contract matters — but that is S2/T2's concern, not this subtask).
- Residual in-task check (Appendix E #15): the script must run under **git-for-windows `sh`** → keep it
  **strict POSIX** (no bashisms).

## Resolution decision (the one real design call)
`gitRunner.run()` execs git with `-C g.workDir` (never `os.Chdir`, never `cmd.Dir`). So git's effective cwd
is `g.workDir`, and any relative `--git-path` output is relative **to `g.workDir`**. Therefore:

```
raw := TrimSpace(stdout)
if filepath.IsAbs(raw) { return filepath.Clean(raw) }        // core.hooksPath=abs, some worktree returns
return filepath.Abs(filepath.Join(g.workDir, raw))            // relative → join with the -C dir, then Abs
```

`filepath.Abs` also `Clean`s, so `../.git/hooks` from a subdirectory collapses correctly:
`Abs(Join("/repo/sub", "../.git/hooks")) == "/repo/.git/hooks"`. This single branch covers **all four**
required test layouts (default, core.hooksPath, subdirectory, linked worktree) with no per-case logic.

## Exit-code convention (differs from the unborn methods)
`git rev-parse --git-path hooks` succeeds (exit 0) even on an **unborn** repo — it needs no commits. So there
is **NO 128-as-non-error** convention here (unlike `RevParseHEAD`/`CommitCount`/`RecentSubjects`). Exit 128 =
not-a-repo/corrupt = a **real error**. This mirrors `StatusPorcelain`'s convention (works on unborn; 128 is a
real error). Branch on `code != 0`, never on `code == 128`.

## The hook script (FR-H1 / FR-H5)
Marker (identity line, stable, used by S2's status/uninstall detection and idempotent rewrite):

```
# stagecoach prepare-commit-msg hook v1
```

Full script (`hookScript(false)`), mode 0755, shebang `#!/bin/sh`:

```sh
#!/bin/sh
# stagecoach prepare-commit-msg hook v1
exec stagecoach hook exec "$@"
```

Strict variant (`hookScript(true)`, FR-H5 opt-in — failures abort the commit): the body becomes
`exec stagecoach hook exec --strict "$@"`.

POSIX audit: shebang + a `#` comment + a single `exec … "$@"`. `"$@"` is POSIX; `exec` is POSIX; no arrays, no
`[[`, no `function`, no `local`, no process substitution. Verifiable with `sh -n` (and `dash -n` /
`checkbashisms` where available). No bashisms.

## Placement of `hookScript`
`hookScript(strict bool) string` is **unexported** per the contract → it must live in the package that
consumes it. S2 ("internal/hook package + hook install|uninstall|status") owns that package, so this subtask
**creates `internal/hook/script.go`** (marker const + `hookScript`), and S2 builds the install/status/uninstall
commands on top of it. `Marker` is exported so S2 detection and cross-package tests can reference the identity
string; the file mode is exposed as a documented const for S2's writer. `HooksPath()` goes on the
`internal/git` `Git` interface (it wraps a git plumbing call — it belongs with the other runner methods).
