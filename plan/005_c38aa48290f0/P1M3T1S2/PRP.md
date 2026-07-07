name: "P1.M3.T1.S2 — internal/hook package + `hook install|uninstall|status` commands"
description: |
  Build the hook-lifecycle logic in `internal/hook/hook.go` (Detect / Install / Uninstall over a hooks
  directory, marker-gated and never-clobber) on top of the S1 primitives (`Marker`, `ScriptMode`,
  `hookScript`, `git.HooksPath`), and wire a cobra `hook` command group (`install [--print] [--strict]`,
  `uninstall`, `status`) in `internal/cmd/hook.go`. Per-repo only. Foreign hooks are refused (exit 1, no
  --force). Ships the docs (Mode A → docs/cli.md). The installed script invokes `stagecoach hook exec "$@"`,
  whose runtime is P1.M3.T2.S1 — NOT this subtask.

---

## Goal

**Feature Goal**: Ship working `stagecoach hook install|uninstall|status` — the per-repo `prepare-commit-msg`
hook lifecycle from PRD §9.20 (FR-H1/FR-H2/FR-H3/FR-H5): marker-gated idempotent install, never-clobber
refusal of foreign hooks, `--print` to stdout, `--strict` baked into the script, and a three-state status
report.

**Deliverable**:
1. `internal/hook/hook.go` — `Status` type + `Detect`, `Install`, `Uninstall`, exported `Script` /
   `InvocationLine` accessors, and `ErrForeignHook` / `ErrNoHook` sentinels — all operating on a hooks
   **directory path** (no git dependency, so it is unit-testable with a bare temp dir). Plus
   `internal/hook/hook_test.go`.
2. `internal/cmd/hook.go` — the `hook` cobra group (`hookCmd`) with leaves `install`/`uninstall`/`status`,
   registered on `rootCmd` via `init()` (zero root.go edits), with a group-level no-op `PersistentPreRunE`.
   Plus `internal/cmd/hook_test.go`.
3. `docs/cli.md` — a `### hook install|uninstall|status` subcommand block (Mode A doc duty).

**Success Definition**:
- `stagecoach hook install` writes an executable (0755) `prepare-commit-msg` at `HooksPath()` containing the
  S1 script; re-running rewrites in place (idempotent); `--strict` bakes `--strict` into the script body;
  `--print` writes the script to stdout and touches no disk.
- A pre-existing **foreign** hook is never modified — `install`/`uninstall` refuse with exit 1 and print the
  one-line manual invocation.
- `stagecoach hook uninstall` removes the hook only when the marker is present; `status` prints exactly
  `none` / `stagecoach (v1)` / `foreign`.
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all green.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who commits from an IDE / lazygit via plain `git commit` and
wants stagecoach to auto-fill the empty message. `hook install` is the one-time setup they run per repo.

**Use Case**: `cd repo && stagecoach hook install` → thereafter a plain `git commit` (empty message) fires the
hook, which calls `stagecoach hook exec` (P1.M3.T2.S1) to fill the message. `hook status` tells them what's
installed; `hook uninstall` reverts.

**User Journey**: `hook install` → `HooksPath()` resolves the dir → `internal/hook.Install` detects state →
missing/ours → write `hookScript(strict)` mode 0755 with the Marker; foreign → refuse + print manual line.

**Pain Points Addressed**: incumbents overwrite whatever `prepare-commit-msg` exists — mangling a user's
husky/lint-staged hook. Stagecoach refuses (FR-H2, the never-clobber invariant) and offers the one-line manual
path instead.

## Why

- **PRD §9.20 FR-H1**: install resolves the hook dir via `git rev-parse --git-path hooks` (via S1's
  `HooksPath`) and writes an executable `prepare-commit-msg` with the Marker; per-repo, never global.
- **PRD §9.20 FR-H2**: foreign hook → refuse (exit 1) + print the one-line invocation; **no `--force`**;
  `install --print` emits the script to stdout instead of disk.
- **PRD §9.20 FR-H3**: uninstall removes only when the marker is present; status reports
  `none` / `stagecoach (v1)` / `foreign`.
- **PRD §9.20 FR-H5**: `install --strict` bakes `--strict` into the script body (via `hookScript(true)`).
- **delta_prd R3**: new package `internal/hook/`; foreign-hook refusal has NO `--force` by design.
- **Scope fence**: this subtask does NOT implement `hook exec` (the runtime — P1.M3.T2.S1) nor re-implement
  `HooksPath`/`hookScript`/`Marker`/`ScriptMode` (P1.M3.T1.S1). It consumes them.

## What

Hook-lifecycle logic + a cobra command group + docs. All state decisions key off the S1 `Marker`.

### Success Criteria

- [ ] `internal/hook/hook.go`: `Status` (`StatusNone`/`StatusStagecoach`/`StatusForeign`) with
      `String()` → `none`/`stagecoach (v1)`/`foreign`; `Detect`, `Install`, `Uninstall`, `Script`,
      `InvocationLine`; `ErrForeignHook`, `ErrNoHook`.
- [ ] `Install(dir, strict)` writes 0755 script for missing/ours (idempotent rewrite); returns
      `ErrForeignHook` for foreign **without touching the file**; creates the hooks dir if absent.
- [ ] `Uninstall(dir)` removes only when ours; `ErrForeignHook` for foreign (untouched); `ErrNoHook` for none.
- [ ] `internal/cmd/hook.go`: `hook` group + `install [--print] [--strict]` / `uninstall` / `status`,
      registered via `init()` on `rootCmd`; group-level no-op `PersistentPreRunE` (no config.Load / no
      first-run bootstrap side effect).
- [ ] `install --print` writes the script to stdout, no disk write, works outside a repo.
- [ ] Foreign refusal prints the manual `exec stagecoach hook exec "$@"` line to stderr, exit 1.
- [ ] `docs/cli.md` documents the three commands + `--print`/`--strict` + foreign-hook policy.
- [ ] Full build/test/vet/lint green.

## All Needed Context

### Context Completeness Check

_This PRP names the exact S1 symbols consumed, the exact cobra pattern to copy (providers.go/config.go), the
one non-obvious design call (group-level no-op PersistentPreRunE to avoid config.Load's first-run bootstrap
write), the full `internal/hook` API with bodies, the cmd wiring with bodies, and the test harness to mirror.
An implementer with no prior codebase knowledge can complete it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M3T1S2/research/codebase-patterns.md
  why: Condensed research — the S1 contract, the cobra registration pattern, the config-load first-run
       side-effect and the PersistentPreRunE fix, exit-code conventions, and the FR mapping.
  section: all (short)
  critical: |
    config.Load auto-writes a bootstrap config on first run (load.go:103-109). hook install/uninstall/status
    must NOT trigger it → give hookCmd a no-op PersistentPreRunE (cobra runs only the nearest one).

- docfile: plan/005_c38aa48290f0/P1M3T1S1/PRP.md
  why: The CONTRACT for what S1 delivers — internal/hook/script.go (Marker, ScriptMode, hookScript(strict))
       and git.HooksPath. Assume implemented exactly as specified. This subtask ADDS hook.go to the same
       internal/hook package (so hookScript, unexported, is accessible) and consumes HooksPath.
  section: "Goal + Data models"
  critical: 'Marker = "# stagecoach prepare-commit-msg hook v1"; ScriptMode = 0o755; hookScript unexported;
             HooksPath returns absolute hooks dir. Do NOT re-create any of these.'

- file: internal/cmd/providers.go
  why: THE cobra pattern to copy — a command group var with no RunE (prints help), leaves with
       SilenceErrors/SilenceUsage + RunE, init() doing parent.AddCommand(leaves) then
       rootCmd.AddCommand(parent) with ZERO root.go edits, RunE returning exitcode.New(Error, ...).
  pattern: "Group var (L24-33) + leaf vars (L36-66) + init() registration (L68-72) + runX returning
            exitcode.New on failure, writing to cmd.OutOrStdout()."
  gotcha: "providers INHERITS root's PersistentPreRunE (loads config). hook must NOT — override it."

- file: internal/cmd/config.go
  why: Second cobra-group precedent (config init/path/upgrade). Shows a group whose leaves avoid config.Load
       — but via root's shouldSkipConfigLoad name check. We use the cleaner group-level override instead
       (name collisions: integrate will also add install/remove — §9.21).
  section: "L37-55 (configCmd group + leaf)"

- file: internal/cmd/root.go
  why: rootCmd (the AddCommand target), PersistentPreRunE (which we override for hook), shouldSkipConfigLoad
       (do NOT edit it), Config() accessor (hook does NOT use it), exitcode usage.
  pattern: "rootCmd.AddCommand from an init() in a sibling file; SilenceErrors/SilenceUsage on rootCmd."
  gotcha: "root.PersistentPreRunE calls config.Load → first-run bootstrap WRITE (FR-B3). Override with a
           no-op PersistentPreRunE on hookCmd so install/uninstall/status never load config."

- file: internal/cmd/default_action.go
  why: How a command gets a repo-bound git.Git: repoDir, _ := os.Getwd(); g := git.New(repoDir) (L50-52).
       And the silent-exit trick: exitcode.New(code, nil) exits non-zero WITHOUT main printing (used for the
       foreign refusal, which prints its own stderr message).
  pattern: "git.New(os.Getwd()) → g.HooksPath(ctx). exitcode.New(exitcode.Error, nil) = silent exit 1."

- file: internal/exitcode/exitcode.go
  why: Success=0/Error=1 constants; exitcode.New(code, err); ExitError.Error()=="" when err==nil (silent).
  critical: "Foreign refusal = exit 1. Use exitcode.New(exitcode.Error, nil) after printing the message."

- file: internal/git/git.go
  why: git.New(workDir) Git (L269) constructor; the S1-added HooksPath(ctx) method on the interface.
  pattern: "g := git.New(repoDir); dir, err := g.HooksPath(ctx)"

- file: internal/cmd/providers_test.go
  why: Cobra-command test harness — build args, capture cmd.OutOrStdout()/ErrOrStderr(), assert on output +
       returned error's exit code (errors.As *exitcode.ExitError). Mirror for hook_test.go.
  pattern: "execute the leaf command with SetArgs/SetOut/SetErr; assert stdout + exit code."

- file: internal/cmd/default_action_test.go
  why: The temp-git-repo-in-cwd test pattern (t.Chdir into an initialized temp repo) needed because hook
       commands read os.Getwd(). Reuse its repo-init helper approach for hook_test.go integration cases.
  pattern: "repo := t.TempDir(); git init + user config; t.Chdir(repo); run command."

- url: https://pkg.go.dev/github.com/spf13/cobra#Command
  why: PersistentPreRunE semantics — by default only the NEAREST PersistentPreRunE in the command chain runs,
       so hookCmd's overrides root's for the whole group (the mechanism behind the config-skip design).
  critical: "Nearest-wins (no EnableTraverseRunHooks). Defining PersistentPreRunE on hookCmd replaces root's
             for install/uninstall/status (and future hook exec)."

- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: §9.20 FR-H1/FR-H2/FR-H3/FR-H5 — the authoritative requirements. §15.3 the subcommand descriptions.
  section: "§9.20 (FR-H1/H2/H3/H5), §15.3"
```

### Current Codebase tree (relevant slice)

```bash
internal/hook/
  script.go        # S1 (assume done): Marker, ScriptMode, hookScript(strict) — CONSUMED, not edited
  script_test.go   # S1 — unchanged
  hook.go          # NEW — Status, Detect, Install, Uninstall, Script, InvocationLine, sentinels
  hook_test.go     # NEW — temp-dir unit tests (idempotent, foreign refusal, strict, status strings)
internal/cmd/
  root.go          # UNCHANGED — rootCmd is the AddCommand target (do NOT edit)
  providers.go     # pattern reference (command group)
  config.go        # pattern reference (command group)
  default_action.go# pattern reference (git.New(Getwd), silent exit)
  hook.go          # NEW — hook cobra group + install/uninstall/status leaves, init() registration
  hook_test.go     # NEW — cobra-command tests (temp repo via t.Chdir; --print bypass)
internal/git/git.go# S1 (assume done): HooksPath(ctx) — CONSUMED
docs/cli.md        # EDIT — add ### hook install|uninstall|status subcommand block
```

### Desired Codebase tree

```bash
# Two NEW source files (internal/hook/hook.go, internal/cmd/hook.go) + their tests + one docs edit.
# No root.go edits (init()-based registration, providers.go/config.go pattern). No new dependencies.
# hookScript/Marker/ScriptMode/HooksPath are consumed from S1, never re-declared.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (config side-effect): root.PersistentPreRunE → config.Load → first-run bootstrapWriteConfig
// (FR-B3, load.go:103-109). A read-only `hook status` must not silently create ~/.config/stagecoach/config.toml.
// FIX: hookCmd defines its OWN PersistentPreRunE returning nil. Cobra runs only the nearest one, so config
// never loads for hook install/uninstall/status. This is self-contained in hook.go — do NOT edit root.go's
// shouldSkipConfigLoad (adding "install"/"uninstall"/"status" names there would collide with future
// `integrate install|remove`, §9.21).

// CRITICAL (never-clobber, FR-H2): a file exists WITHOUT the Marker → Foreign → Install/Uninstall MUST NOT
// write or delete it. Detect first, branch, return ErrForeignHook BEFORE any WriteFile/Remove. An EMPTY
// (0-byte) existing file counts as Foreign (safe default — it has no marker). The tests assert the foreign
// file's bytes are byte-for-byte UNCHANGED after a refused install (the never-clobber invariant).

// CRITICAL (same-package access): hook.go lives in package hook alongside S1's script.go, so it calls the
// UNEXPORTED hookScript(strict) directly. Export a thin Script(strict) wrapper ONLY for the cmd layer's
// `install --print` (unexported symbols don't cross the internal/cmd boundary).

// GOTCHA (executable bit): os.WriteFile honors umask, so the mode arg may be masked below 0o755. After
// WriteFile, call os.Chmod(path, ScriptMode) to GUARANTEE 0o755 (also fixes a rewrite over a file with
// different perms). Tests assert (info.Mode().Perm() == 0o755).

// GOTCHA (hooks dir may not exist): with core.hooksPath pointing at a fresh custom dir, HooksPath returns a
// path whose dir is absent. Install must os.MkdirAll(hooksDir, 0o755) before WriteFile.

// GOTCHA (drift guard): InvocationLine(strict) must stay identical to the exec line inside hookScript(strict).
// Add a test asserting strings.Contains(hookScript(strict), InvocationLine(strict)) for both strict values.

// GOTCHA (silent exit): the foreign refusal prints its own multi-line stderr message, then returns
// exitcode.New(exitcode.Error, nil) — nil err so main does NOT double-print (ExitError.Error()==""). Same
// idiom as default_action.go. A generic failure (WriteFile error) returns exitcode.New(exitcode.Error, err).

// GOTCHA (test cwd): hook install/uninstall/status read os.Getwd() → cmd-level tests must t.Chdir into an
// initialized temp git repo (Go 1.24 testing.T.Chdir; mirror default_action_test.go). `install --print`
// bypasses Getwd/disk entirely, so its test needs no repo.

// GOTCHA (uninstall idempotency): None (nothing installed) → ErrNoHook → cmd exits 0 with an informational
// line (idempotent, like `rm -f`). Foreign → ErrForeignHook → exit 1 (refuse). Only Foreign is a hard
// refusal; the never-clobber invariant is about not touching someone else's file, not about erroring when
// there's nothing to remove.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/hook/hook.go  (NEW — same package as S1's script.go)
package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// HookFilename is the git hook stagecoach manages (PRD §9.20 — prepare-commit-msg only).
const HookFilename = "prepare-commit-msg"

// Status is the state of a repo's prepare-commit-msg hook (PRD §9.20 FR-H3).
type Status int

const (
	StatusNone      Status = iota // no prepare-commit-msg file
	StatusStagecoach               // stagecoach-owned (Marker present)
	StatusForeign                 // a hook file exists WITHOUT our Marker (never touch it)
)

// String renders the FR-H3 report tokens EXACTLY: "none" / "stagecoach (v1)" / "foreign".
func (s Status) String() string {
	switch s {
	case StatusStagecoach:
		return "stagecoach (v1)"
	case StatusForeign:
		return "foreign"
	default:
		return "none"
	}
}

// Sentinels for the refusal paths (FR-H2 / FR-H3). Callers use errors.Is.
var (
	ErrForeignHook = errors.New("a foreign prepare-commit-msg hook exists")
	ErrNoHook      = errors.New("no stagecoach prepare-commit-msg hook is installed")
)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/hook/hook.go — Detect
  - IMPLEMENT: Detect(hooksDir string) (Status, error). os.ReadFile(filepath.Join(hooksDir, HookFilename));
    os.ErrNotExist → StatusNone,nil; other read error → StatusNone,err; strings.Contains(data, Marker) →
    StatusStagecoach; else StatusForeign. (Marker is S1's exported const, same package.)
  - PLACEMENT: internal/hook/hook.go, after the type/const/sentinel block above.

Task 2: CREATE internal/hook/hook.go — Install (marker-gated idempotent + never-clobber)
  - IMPLEMENT: Install(hooksDir string, strict bool) (Status, error). prev,err := Detect(...); on err return.
    prev==StatusForeign → return prev, ErrForeignHook (NO write). os.MkdirAll(hooksDir, 0o755).
    os.WriteFile(path, []byte(hookScript(strict)), ScriptMode) then os.Chmod(path, ScriptMode). return prev,nil.
  - RETURNS prev so the cmd layer prints "Installed" vs "Updated".

Task 3: CREATE internal/hook/hook.go — Uninstall
  - IMPLEMENT: Uninstall(hooksDir string) (Status, error). st,err := Detect(...); on err return. switch st:
    StatusStagecoach → os.Remove(path) (return st, removeErr); StatusForeign → st, ErrForeignHook;
    default(None) → st, ErrNoHook.

Task 4: CREATE internal/hook/hook.go — Script + InvocationLine accessors
  - IMPLEMENT: Script(strict bool) string { return hookScript(strict) }  // for `install --print`.
    InvocationLine(strict bool) string → `exec stagecoach hook exec "$@"` (+ ` --strict ` before "$@" if strict).
  - Keep InvocationLine consistent with hookScript (drift-guard test in Task 8).

Task 5: CREATE internal/cmd/hook.go — the hook command group + leaf vars
  - IMPLEMENT: hookCmd (Use "hook", Short, Long, SilenceErrors/SilenceUsage, PersistentPreRunE: return nil).
    hookInstallCmd (Use "install", Args NoArgs, RunE runHookInstall), hookUninstallCmd, hookStatusCmd.
  - FLAGS on install ONLY (local, not persistent): --print (bool), --strict (bool).

Task 6: CREATE internal/cmd/hook.go — init() registration + RunE bodies + hooksDir helper
  - init(): hookInstallCmd.Flags().BoolVar for print/strict; hookCmd.AddCommand(install,uninstall,status);
    rootCmd.AddCommand(hookCmd). (No root.go edit — providers.go pattern.)
  - hooksDir(ctx): repoDir,_ := os.Getwd(); return git.New(repoDir).HooksPath(ctx).
  - runHookInstall/runHookUninstall/runHookStatus per the snippets below.

Task 7: EDIT docs/cli.md — add the hook subcommand block
  - Under "## Subcommands", add ### hook install|uninstall|status documenting install/--print/--strict,
    uninstall, status, and the foreign-hook (never-clobber, no --force) policy. Note the script calls
    `stagecoach hook exec "$@"` (runtime lands with P1.M3.T2.S1).

Task 8: CREATE internal/hook/hook_test.go — temp-dir unit tests
  - TestDetect_None/Stagecoach/Foreign (write files into t.TempDir()).
  - TestInstall_Fresh: prev==StatusNone; file exists; Perm()==0o755; content==hookScript(false).
  - TestInstall_IdempotentReinstall: install twice; 2nd prev==StatusStagecoach; content stable; still 0755.
  - TestInstall_Strict: content==hookScript(true); contains "--strict".
  - TestInstall_ForeignRefusal: pre-write foreign bytes; Install→errors.Is(ErrForeignHook); file bytes
    UNCHANGED (never-clobber invariant).
  - TestInstall_CreatesHooksDir: hooksDir = filepath.Join(t.TempDir(),"nested"); Install creates it.
  - TestUninstall_RemovesOurs / _ForeignRefusal(untouched) / _NoneIsErrNoHook.
  - TestStatus_String: the three exact tokens.
  - TestScript_MatchesHookScript + TestInvocationLine_InScript (drift guards).

Task 9: CREATE internal/cmd/hook_test.go — cobra-command tests
  - TestHookInstall_Print: run `hook install --print`; stdout==hookScript(false); no repo needed; no file.
  - TestHookInstall_PrintStrict: stdout==hookScript(true).
  - TestHookInstallStatusUninstall_RoundTrip: t.Chdir(temp git repo); install → status "stagecoach (v1)" →
    uninstall → status "none". (Mirror default_action_test.go repo init.)
  - TestHookInstall_ForeignRefused: pre-create a foreign prepare-commit-msg in the repo's hooks dir; run
    install; assert exit code 1 (errors.As *exitcode.ExitError, Code==1) and stderr contains the invocation
    line; file unchanged.
  - TestHookCmd_NoConfigLoad (optional): running `hook status` in a repo does not create a global config
    file (assert the bootstrap path was not written — or that Config()/PreRun was skipped).
```

### Implementation Patterns & Key Details

```go
// internal/hook/hook.go — the three operations
func Detect(hooksDir string) (Status, error) {
	data, err := os.ReadFile(filepath.Join(hooksDir, HookFilename))
	if errors.Is(err, os.ErrNotExist) {
		return StatusNone, nil
	}
	if err != nil {
		return StatusNone, err
	}
	if strings.Contains(string(data), Marker) { // Marker: S1 exported const, same package
		return StatusStagecoach, nil
	}
	return StatusForeign, nil
}

func Install(hooksDir string, strict bool) (Status, error) {
	prev, err := Detect(hooksDir)
	if err != nil {
		return prev, err
	}
	if prev == StatusForeign {
		return prev, ErrForeignHook // never clobber — no write
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return prev, err
	}
	p := filepath.Join(hooksDir, HookFilename)
	if err := os.WriteFile(p, []byte(hookScript(strict)), ScriptMode); err != nil {
		return prev, err
	}
	if err := os.Chmod(p, ScriptMode); err != nil { // guarantee 0o755 despite umask / prior perms
		return prev, err
	}
	return prev, nil
}

func Uninstall(hooksDir string) (Status, error) {
	st, err := Detect(hooksDir)
	if err != nil {
		return st, err
	}
	switch st {
	case StatusStagecoach:
		return st, os.Remove(filepath.Join(hooksDir, HookFilename))
	case StatusForeign:
		return st, ErrForeignHook
	default:
		return st, ErrNoHook
	}
}

func Script(strict bool) string { return hookScript(strict) } // for `install --print`

func InvocationLine(strict bool) string {
	if strict {
		return `exec stagecoach hook exec --strict "$@"`
	}
	return `exec stagecoach hook exec "$@"`
}
```

```go
// internal/cmd/hook.go — the group + leaves + wiring
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/hook"
)

var (
	flagHookPrint  bool
	flagHookStrict bool
)

// hookCmd is the PRD §9.20 hook command group. No RunE → bare `stagecoach hook` prints help.
// Its no-op PersistentPreRunE OVERRIDES root's (cobra runs only the nearest): install/uninstall/status
// need only the repo's hooks dir, never the resolved config — and must not trigger config.Load's
// first-run bootstrap write (FR-B3). P1.M3.T2.S1 adds `hook exec` as a sibling leaf to this group.
var hookCmd = &cobra.Command{
	Use:               "hook",
	Short:             "Manage the per-repo prepare-commit-msg hook",
	Long:              `Install, remove, or inspect stagecoach's per-repo prepare-commit-msg hook (PRD §9.20).`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
}

var hookInstallCmd = &cobra.Command{
	Use:           "install",
	Short:         "Install the prepare-commit-msg hook in this repo",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookInstall,
}

var hookUninstallCmd = &cobra.Command{
	Use:           "uninstall",
	Short:         "Remove the stagecoach prepare-commit-msg hook",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookUninstall,
}

var hookStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Report the prepare-commit-msg hook state (none|stagecoach (v1)|foreign)",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookStatus,
}

func init() {
	hookInstallCmd.Flags().BoolVar(&flagHookPrint, "print", false,
		"Write the hook script to stdout instead of installing it")
	hookInstallCmd.Flags().BoolVar(&flagHookStrict, "strict", false,
		"Bake --strict into the hook so generation failures abort the commit (default: never block)")
	hookCmd.AddCommand(hookInstallCmd, hookUninstallCmd, hookStatusCmd)
	rootCmd.AddCommand(hookCmd) // register on root — NO edit to root.go (providers.go pattern)
}

// hooksDir resolves this repo's absolute hooks directory via S1's git.HooksPath.
func hooksDir(ctx context.Context) (string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return git.New(repoDir).HooksPath(ctx)
}

func runHookInstall(cmd *cobra.Command, _ []string) error {
	if flagHookPrint { // FR-H2: --print bypasses disk entirely (works outside a repo)
		fmt.Fprint(cmd.OutOrStdout(), hook.Script(flagHookStrict))
		return nil
	}
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	prev, err := hook.Install(dir, flagHookStrict)
	if errors.Is(err, hook.ErrForeignHook) { // FR-H2 never-clobber refusal
		fmt.Fprintf(cmd.ErrOrStderr(),
			"stagecoach: a foreign prepare-commit-msg hook already exists; refusing to overwrite it.\n"+
				"To use stagecoach, add this line to your existing hook:\n\n    %s\n",
			hook.InvocationLine(flagHookStrict))
		return exitcode.New(exitcode.Error, nil) // silent exit 1 — message already printed
	}
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: install hook: %w", err))
	}
	verb := "Installed"
	if prev == hook.StatusStagecoach {
		verb = "Updated"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s stagecoach prepare-commit-msg hook.\n", verb)
	return nil
}

func runHookUninstall(cmd *cobra.Command, _ []string) error {
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	switch _, err := hook.Uninstall(dir); {
	case errors.Is(err, hook.ErrForeignHook):
		fmt.Fprintln(cmd.ErrOrStderr(),
			"stagecoach: prepare-commit-msg hook is foreign; refusing to remove it.")
		return exitcode.New(exitcode.Error, nil) // exit 1
	case errors.Is(err, hook.ErrNoHook):
		fmt.Fprintln(cmd.OutOrStdout(), "No stagecoach prepare-commit-msg hook to remove.")
		return nil // idempotent — exit 0
	case err != nil:
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: uninstall hook: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Removed stagecoach prepare-commit-msg hook.")
	return nil
}

func runHookStatus(cmd *cobra.Command, _ []string) error {
	dir, err := hooksDir(cmd.Context())
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	st, err := hook.Detect(dir)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: hook status: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), st.String()) // "none" / "stagecoach (v1)" / "foreign"
	return nil
}
```

### Integration Points

```yaml
CONSUMES (from P1.M3.T1.S1 — do NOT re-implement):
  - internal/hook/script.go: Marker (const), ScriptMode (os.FileMode), hookScript(strict) (unexported).
  - internal/git/git.go: git.HooksPath(ctx) on the Git interface.

REGISTERS:
  - rootCmd.AddCommand(hookCmd) from internal/cmd/hook.go init() — NO root.go edit (providers.go pattern).

PROVIDES (to P1.M3.T2.S1 — hook exec):
  - hookCmd is a package-level var; hook exec adds itself via hookCmd.AddCommand(hookExecCmd) from a new
    hookexec.go in package cmd. Note: hookCmd's no-op PersistentPreRunE means hook exec inherits NO config
    load and must load its own config in RunE (fits FR-H5 never-block). That is T2's concern.

DOCS:
  - docs/cli.md: ### hook install|uninstall|status under ## Subcommands (Mode A).

OUT OF SCOPE (do NOT touch):
  - hook exec runtime (P1.M3.T2.S1); HooksPath / hookScript / Marker / ScriptMode (P1.M3.T1.S1);
    root.go / shouldSkipConfigLoad; any provider/config resolution.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/hook/hook.go internal/hook/hook_test.go internal/cmd/hook.go internal/cmd/hook_test.go
go build ./...        # new internal/hook logic + internal/cmd/hook.go must compile
go vet ./...
golangci-lint run
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/hook/... -v   # Detect/Install/Uninstall: idempotent, foreign refusal, strict, statuses
go test ./internal/cmd/...  -run Hook -v   # cobra: --print bypass, round-trip, foreign refusal exit 1
go test ./internal/cmd/...  -v   # ensure existing providers/config/default_action tests still pass
# Expected: all pass. Foreign file bytes unchanged after refused install; scripts match S1 exactly.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build and drive the real binary in a throwaway repo.
go build -o /tmp/stagecoach ./cmd/stagecoach 2>/dev/null || go build -o /tmp/stagecoach .
tmp=$(mktemp -d); ( cd "$tmp" && git init -q
  /tmp/stagecoach hook status                 # -> none
  /tmp/stagecoach hook install                # -> Installed ...   (exit 0)
  test -x .git/hooks/prepare-commit-msg && echo "executable OK"
  grep -q 'stagecoach prepare-commit-msg hook v1' .git/hooks/prepare-commit-msg && echo "marker OK"
  /tmp/stagecoach hook install                # -> Updated ...      (idempotent, exit 0)
  /tmp/stagecoach hook status                 # -> stagecoach (v1)
  /tmp/stagecoach hook install --print        # -> script to stdout, no change on disk
  /tmp/stagecoach hook install --strict; grep -q -- '--strict' .git/hooks/prepare-commit-msg && echo "strict OK"
  /tmp/stagecoach hook uninstall              # -> Removed ...      (exit 0)
  /tmp/stagecoach hook status                 # -> none
  # foreign refusal:
  printf '#!/bin/sh\necho mine\n' > .git/hooks/prepare-commit-msg
  /tmp/stagecoach hook install; echo "exit=$?"  # -> prints manual line, exit 1
  grep -q 'echo mine' .git/hooks/prepare-commit-msg && echo "foreign UNCHANGED OK"
  /tmp/stagecoach hook uninstall; echo "exit=$?" # -> refuses foreign, exit 1
)
# Expected: statuses/verbs as annotated; foreign file never modified; strict baked in; print to stdout only.
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...
golangci-lint run ./...
# Confirm `hook status` does NOT create a global config (no config.Load side effect):
tmp=$(mktemp -d); ( cd "$tmp" && git init -q
  XDG_CONFIG_HOME="$tmp/xdg" /tmp/stagecoach hook status
  test ! -e "$tmp/xdg/stagecoach/config.toml" && echo "no bootstrap side effect OK" )
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...` green; existing cmd tests unchanged.
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] install writes 0755 marker'd script; reinstall rewrites (idempotent, "Updated"); `--strict` bakes in
      `--strict`; `--print` → stdout only (no disk, works outside a repo).
- [ ] Foreign hook: install AND uninstall refuse (exit 1), print manual line, leave the file byte-identical.
- [ ] uninstall removes only when marker present; None → exit 0 idempotent note.
- [ ] status prints exactly `none` / `stagecoach (v1)` / `foreign`.
- [ ] `hook status` creates NO global config file (no bootstrap side effect).

### Code Quality Validation
- [ ] internal/hook logic takes a hooks-DIR path (no git dependency) — unit-testable with a bare temp dir.
- [ ] Detect-first / never-clobber ordering (return ErrForeignHook before any write/remove).
- [ ] os.Chmod after WriteFile guarantees 0o755; os.MkdirAll creates a missing custom hooks dir.
- [ ] hook group registered via init() on rootCmd (no root.go edit); no-op PersistentPreRunE on hookCmd.
- [ ] Script/InvocationLine kept consistent with S1's hookScript (drift-guard tests).
- [ ] No re-implementation of HooksPath / hookScript / Marker / ScriptMode.

### Documentation & Deployment
- [ ] docs/cli.md documents install/--print/--strict, uninstall, status, and the foreign-hook (no --force)
      policy; notes the script calls `stagecoach hook exec "$@"` (runtime = P1.M3.T2.S1).

---

## Anti-Patterns to Avoid

- ❌ Don't edit root.go or shouldSkipConfigLoad — register via init() and skip config with hookCmd's own
  no-op PersistentPreRunE (adding "install"/"uninstall"/"status" to shouldSkipConfigLoad collides with
  future `integrate install|remove`).
- ❌ Don't add a `--force` to override a foreign hook — the never-clobber refusal is the whole point (FR-H2).
- ❌ Don't write/delete a foreign file — Detect first, return ErrForeignHook before any disk mutation.
- ❌ Don't re-declare Marker / ScriptMode / hookScript / HooksPath — consume S1's.
- ❌ Don't rely on WriteFile's mode arg for the exec bit — umask can mask it; os.Chmod after.
- ❌ Don't let `hook status` trigger config.Load's first-run bootstrap write.
- ❌ Don't implement `hook exec` here — that's P1.M3.T2.S1 (this subtask only defines the hookCmd group it
  attaches to).
- ❌ Don't write to os.Stdout directly — use cmd.OutOrStdout()/ErrOrStderr() so tests can capture output.

---

## Confidence Score

**9/10** for one-pass implementation success. The surface is small and fully pinned: the hook-lifecycle
logic is pure file I/O over a directory path (no git needed to unit-test), the cobra wiring copies two
established in-repo precedents (providers.go/config.go) verbatim, and every FR maps to a named function with
its body written out. The one genuinely non-obvious call — a group-level no-op PersistentPreRunE to dodge
config.Load's first-run bootstrap write — is documented with its mechanism (cobra nearest-wins) and its
rationale. The −1 is residual environment risk in the cmd-level tests (t.Chdir into a temp git repo; exact
foreign-refusal stderr assertions), neutralized by asserting on exit codes + substring rather than exact
formatting, and by keeping the disk-free `--print` path testable without a repo.
