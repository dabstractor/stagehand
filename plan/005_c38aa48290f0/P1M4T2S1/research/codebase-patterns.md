# P1.M4.T2.S1 — git-alias integration target: condensed research

Research date: 2026-07-02. Verifies the codebase contract this subtask consumes (S1 protocol, S2
command surface) and the git alias mechanics (architecture/external_deps.md §7, VERIFIED).

## 1. The git alias mechanics (gates FR-I4) — VERIFIED

From `architecture/external_deps.md` §7 + the work item CONTRACT:
- **Install:** `git config --global alias.<name> '!stagehand'`. The `!` prefix = "run as a shell command
  from the repo toplevel, args appended." Default name `stagehand` → `git stagehand`.
- **Read-back:** `git config --global --get alias.<name>` prints the stored value **INCLUDING the `!`**
  (i.e. `!stagehand`). Strip the leading `!` when comparing "is it ours" → the command part is `stagehand`.
  Exit 1 / empty stdout when unset (NOT an error).
- **Remove:** `git config --global --unset alias.<name>`. Exit 5 when the key is not set. FR-I6: only
  unset when the current value is ours (sans-`!` == `stagehand`); a foreign value is left untouched.
- **git performs the .gitconfig edit itself** → the FR-I3 file machinery (parse/backup/atomic/validate) is
  UNNECESSARY for git-alias. BUT FR-I3c (preview + confirm) still applies: the command + resulting usage
  are shown and confirmed (`y/N`; `--yes` skips); a conflicting `alias.<name>` is surfaced before overwrite.

## 2. Where git-alias runs git config — the central design decision

**Decision: add three repo-independent global-config methods to the existing `Git` interface
(`internal/git/git.go`), implemented on `gitRunner` via the proven `run()` helper.** Not a new
self-contained exec helper.

Evidence:
- delta_prd.md R4 line 47: "`git-alias` target delegates the file edit to `git config` via the **existing
  git exec wrapper**." + work item: "existing `internal/git` exec seam for running `git config`."
- `HooksPath` (P1.M3.T1.S1) is the DIRECT precedent: a new concern (hooks dir) was added as a method on
  the `Git` interface + `gitRunner` impl in `internal/git/git.go` + a dedicated `hookspath_test.go`.
- `config/git.go` has its OWN `gitExec` ONLY to avoid an import cycle (loadGitConfig would cycle through
  internal/git); the `integrate` package has NO such cycle risk → it imports `internal/git` cleanly.

The three new methods (all repo-independent; `-C workDir` is a harmless no-op for `--global` scope):
```go
ConfigGlobalGet(ctx, key)    (value string, found bool, err error)  // git config --global --get <key>; exit 1 = not found
ConfigGlobalSet(ctx, key, value) error                              // git config --global <key> <value>
ConfigGlobalUnset(ctx, key)  (found bool, err error)                // git config --global --unset <key>; exit 5 = not set
```
Exit-code semantics mirror config/git.go's `gitConfigGet`: non-zero-but-1 (get) / non-zero-but-5 (unset)
are the "missing key" non-errors; everything else is a wrapped error. `run()` returns (stdout, stderr,
code, nil) for non-zero exits, so the impls branch on `code`.

**CRITICAL env passthrough for tests:** `gitRunner.run()` does NOT set `cmd.Env` → the child git inherits
the parent environment. So `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>)` in tests propagates to the git
subprocess, and `GIT_CONFIG_GLOBAL` (when set) REPLACES `~/.gitconfig` (full isolation — the real global
config is never read or written). This is exactly the work item's test mandate. No repo is needed for
`git config --global` — tests pass a temp dir to `git.New(<tmpdir>)` (the `-C <tmpdir>` is a no-op here).

## 3. The Entry contract this subtask implements (from S2's PRP — treat as authoritative)

`internal/integrate/registry.go` (S2) defines the EXACT interface git-alias implements:
```go
type Entry interface {
    Name() string
    Detect(ctx) error                                            // nil=tool present; non-nil=gate (exit 1 + note)
    ConfigPath(ctx) (string, error)                              // resolved config path (list CONFIG column)
    Status(ctx) (Status, error)                                  // NotInstalled/Installed/Foreign
    Install(ctx, InstallOptions) (InstallResult, error)
    Remove(ctx, RemoveOptions) (RemoveResult, error)
}
type Status int  // StatusNotInstalled / StatusInstalled / StatusForeign (+ String: not installed/installed/foreign)
type InstallOptions struct { Yes bool; Out io.Writer; Confirm ConfirmFunc }  // RemoveOptions symmetric
type InstallResult  struct { Outcome Outcome; Target, Path, Backup string }  // RemoveResult symmetric
```
- `Outcome` is S1's `integrate.Outcome` (Created/Updated/Removed/Declined/NoChange).
- `ConfirmFunc = func(out io.Writer, path, diff string) bool`; `nil ⇒ integrate.DefaultConfirm`
  (TTY-gated y/N; non-TTY stdin auto-declines; `--yes` bypasses by the caller setting opts.Yes).
- **git-alias does NOT call `protocol.Apply`** (S2's PRP flagged this explicitly: FR-I4 delegates the
  edit to git config). git-alias owns its own Install/Remove with its OWN preview+confirm (honoring
  opts.Yes / opts.Confirm). It builds a preview string (the command + resulting usage + optional conflict
  warning) and passes it as the `diff` arg to the shared ConfirmFunc → uniform TTY/--yes behavior.
- `InstallResult.Backup` is ALWAYS `""` for git-alias (git owns the .gitconfig; no backup file).

## 4. The defaultEntries registration seam (S2's, edited by T2.S1)

S2 ships `var defaultEntries = func() []integrate.Entry { return nil }` in `internal/cmd/integrate.go`.
T2.S1's SINGLE edit to S2's file: make it return `[]integrate.Entry{ &gitAliasEntry{...} }`. The gitAliasEntry
reads the resolved `--alias-name` flag and constructs `git.New(<cwd>)`. Everything else git-alias owns
lives in a NEW file `internal/cmd/integrate_gitalias.go` (+ `integrate_gitalias_test.go`) to keep the diff
to S2's code minimal (defaultEntries body + one resetIntegrateFlags line).

## 5. Command-layer conventions to mirror (from hook.go + providers.go + S2)

- `--alias-name <n>`: a LOCAL flag on BOTH `integrateInstallCmd` and `integrateRemoveCmd` (shared backing
  var `flagAliasName`; default `""` → resolved to `"stagehand"`). Registered in `init()` inside
  `integrate_gitalias.go`. Mirrors hook.go's `--strict`/`--print` (local to hookInstallCmd) — but here
  both install AND remove need the name (you remove the alias by name). Reset in `resetIntegrateFlags`
  (S2's helper; T2.S1 appends the `--alias-name` reset line).
- Exit codes via `exitcode.New(exitcode.Error, err)` / Decline+NoChange → nil (exit 0) — S2's dispatch
  already does the Outcome→verb mapping; git-alias just returns the right Outcome.
- The `integrate` group SKIPS config.Load (S2's no-op PersistentPreRunE) → works OUTSIDE a repo. git-alias
  must not assume a repo exists (global config doesn't need one).

## 6. Outcome mapping for git-alias (the dispatch verbs S2 prints)

| Action | Current alias state | Outcome | Writes? |
|--------|--------------------|---------|---------|
| Install | unset (not found) | **Created** | yes (after confirm) |
| Install | set & ours (`!stagehand`) | **NoChange** | no (idempotent) |
| Install | set & foreign | **Updated** (after surfacing conflict + confirm) | yes |
| Install | (any) user declines / non-TTY no --yes | **Declined** | no |
| Remove | unset | **NoChange** | no |
| Remove | set & ours | **Removed** (after confirm) | yes (`--unset`) |
| Remove | set & foreign | **NoChange** + stderr note (refuse — FR-I6) | no |

## 7. Test isolation pattern (the work item's mandate)

- `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile path>)` — fully isolates (replaces `~/.gitconfig`). Tests assert
  on the temp file's contents (e.g. `git config --global --get alias.stagehand` → `!stagehand`).
- The real global config is NEVER touched. For paranoia, tests MAY also `t.Setenv("GIT_CONFIG_NOSYSTEM",
  "1")` to ignore system config (rarely relevant for aliases).
- Construct the entry with `git.New(t.TempDir())` (no repo init needed — `git config --global` works
  anywhere). Inject a fixed-bool Confirm for the confirm-flow tests (no real stdin/TTY).
- Detect-absent test: `t.Setenv("PATH", "")` makes `exec.LookPath("git")` fail (mirrors
  `internal/git/git_test.go` TestRun_LookPathFailure).

## 8. File convention for new Git interface methods

All `gitRunner` methods live in `internal/git/git.go` (interface decl at top; impls below — `HooksPath`
is at L1340). Tests are one-per-method: `hookspath_test.go`, `revparse_test.go`, etc. So:
- ADD the three `ConfigGlobal*` methods to the `Git` interface (git.go) + impls on `gitRunner` in git.go.
- ADD `internal/git/configglobal_test.go` (the method's dedicated test file, hookspath_test.go style).

## 9. Scope fences

- CONSUMES: S1's `integrate.Outcome`/`ConfirmFunc`/`DefaultConfirm`; S2's `integrate.Entry`/`Registry`/
  `Status`/`InstallOptions`/`InstallResult`/`RemoveOptions`/`RemoveResult`; the `Git` interface +
  `run()` exec seam; `defaultEntries` + the cobra `integrate` group + `resetIntegrateFlags`.
- PROVIDES: the `gitAliasEntry` Entry impl + the three `Git.ConfigGlobal*` methods + the `--alias-name`
  flag + docs/cli.md git-alias target section. S2's dispatch drives it with zero cmd-layer changes.
- OUT OF SCOPE: lazygit (T2.S2, adds yaml.v3); the no-mangle protocol engine (S1, already shipped); the
  command surface / registry (S2); README coherence (P1.M7).
