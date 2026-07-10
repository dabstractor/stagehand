name: "P1.M3.T2.S1 — Implement `stagecoach lock status` cobra subcommand (FR-K4)"
description: >
  The USER-FACING surface for FR-K4 (PRD §9.27 / §15.3): a NEW read-only `stagecoach lock status`
  cobra subcommand that prints the current repo's run-lock state — path, holder pid/hostname/repo/
  timestamp/snapshot, liveness, and (Unix) orphan status — WITHOUT acquiring or breaking the lock
  (FR52 preserved). This is ONE new file `internal/cmd/lock.go` (a cobra command GROUP `lock` with a
  no-op PersistentPreRunE + ONE leaf `status`) registered on rootCmd via `init()` with ZERO edits to
  root.go, plus ONE new test file `internal/cmd/lock_test.go`. It CONSUMES the already-landed
  `lock.Status(repoPath)` from P1.M3.T1.S1 (the `(path, contents, alive, orphan, err)` read-only API)
  and does NOT touch internal/lock/. The group command is a clone of the proven `hook`/`integrate`/
  `providers` pattern: no-op `PersistentPreRunE` OVERRIDES root's `config.Load` (cobra runs only the
  nearest ancestor's) so `lock status` works OUTSIDE a git repo and never triggers config bootstrap
  (FR-B3). Exit codes: nil → 0 (success, INCLUDING the "no lock held" case — a read that found nothing
  is not an error), `exitcode.New(exitcode.Error, ...)` → 1 (Getwd or Status failure). NEVER os.Exit.
  Mode-A godoc on `runLockStatus` states the read-only contract; the README CLI-reference sync is the
  later docs task (P1.M4.T2.S1). NO overlap with P1.M3.T3.S1 (Busy-message reformat in
  default_action.go) or P1.M4.T1.S1 (E2E scenarios). Unit tests pin the deterministic branches
  (no-lock path, live-self-holder via real lock.Acquire); the dead-holder + orphan==true scenarios are
  the E2E harness's job (P1.M4.T1.S1).

---

## Goal

**Feature Goal**: Give Stagecoach a `stagecoach lock status` subcommand (PRD §9.27 FR-K4, §15.3) that
answers "what is the state of THIS repo's run lock?" by printing the lock path + the holder's parsed
pid/hostname/repo/timestamp/snapshot + liveness + (Unix) orphan status — purely read-only. This is the
"show me where the lock is so I can decide" surface: the user learns the path and the holder's state,
then `kill`s the pid or `rm`s the file THEMSELVES. The subcommand never auto-breaks a lock (FR52
preserved) and works even outside a git repo (it is a diagnostic that must run anywhere CWD is).

**Deliverable**:
1. `internal/cmd/lock.go` (NEW) — a cobra command GROUP `lockCmd` (no-op `PersistentPreRunE`, no RunE →
   bare `stagecoach lock` prints help) + ONE leaf `lockStatusCmd` (`status`, `Args: cobra.NoArgs`,
   `RunE: runLockStatus`), registered on `rootCmd` via `init()`. `runLockStatus` resolves repoDir via
   `os.Getwd()`, calls `lock.Status(repoDir)`, and prints per the format spec (§Implementation Blueprint).
2. `internal/cmd/lock_test.go` (NEW) — unit tests for the no-lock path, the live-self-holder path (via a
   real `lock.Acquire`), and the error path; reuse `saveRootState`/`restoreRootState`/`chdir`.

**Success Definition**:
- `stagecoach lock status` in a repo/dir with no lock prints exactly `no run lock for <repoDir>\n` and
  exits 0 (nil error).
- `stagecoach lock status` while a lock IS held (a real `lock.Acquire` in a test, or a concurrent
  stagecoach run) prints `Lock: <path>` + the 7 indented fields (pid/hostname/repo/timestamp/snapshot
  when set/alive/orphaned) and exits 0.
- `stagecoach lock status` outside a git repo still works (the no-op `PersistentPreRunE` skips
  `config.Load` — no bootstrap write, no "not a git repo" error). Verified by a test that chdir's into a
  non-git `t.TempDir()`.
- The `lock` GROUP is registered on rootCmd with ZERO edits to root.go (registration via `init()` on the
  package-level `rootCmd` var — the hook/integrate/providers pattern). `git status --porcelain` shows
  ONLY `internal/cmd/lock.go` + `internal/cmd/lock_test.go` (scope guard).
- Exit codes: success (incl. no-lock) → 0; Getwd/Status error → 1. Never `os.Exit`.
- `go build ./...` (+ GOOS=windows/linux/darwin) clean; `go vet ./internal/cmd/...` clean;
  `gofmt -l` empty; `go test ./internal/cmd/` (race) green; `make test` + `make lint` clean.
- [Mode A] Godoc on `runLockStatus` explains the read-only contract (never acquires/breaks/removes the
  lock — FR52; the user decides whether to kill/rm).

## User Persona (if applicable)

**Target User**: A developer whose `stagecoach` invocation got the §18.5 Busy message ("another run is
in progress"), or who is debugging a stuck lock (the §9.27 "the lock stays forever" orphan case).

**Use Case**: The user wants to SEE the lock's full state — path, holder pid/host, is it alive, is it
orphaned — before deciding to `kill <N>` or `rm <path>`. `stagecoach lock status` prints exactly this.
The user then acts deliberately; the subcommand changed nothing.

**User Journey**: user runs `stagecoach` → Busy message names a lock → user runs `stagecoach lock status`
→ sees "pid 1234, alive, appears orphaned (launcher exited)" → user `kill 1234` → next run unblocked.

**Pain Points Addressed**: FR-K4 — the missing "show me the lock so I can decide" surface. Before this,
the only lock info was buried mid-sentence in the Busy message, with no standalone liveness/orphan read
and no easy copy-paste of the lock path/pid.

## Why

- **FR-K4 / §15.3**: the PRD pins `stagecoach lock status` as a read-only diagnostic: "prints the lock
  path, the holder's pid/hostname/repo/timestamp/snapshot, whether the holder is alive, and (Unix)
  whether it appears orphaned (reparented). With no lock held, prints 'no run lock for <repo>'. Changes
  nothing; the user decides whether to kill/rm. Never auto-breaks (FR52)." This item is exactly that
  subcommand. The read-only API it calls (`lock.Status`) was landed by P1.M3.T1.S1.
- **FR52 preserved by construction**: the subcommand only CALLS `lock.Status` (read-only) and prints. It
  never calls `lock.Acquire`, never `os.Remove`s, never touches the flock. The user makes the kill/rm
  decision; the diagnostic just informs it.
- **Works outside a git repo**: a diagnostic must run anywhere. The no-op `PersistentPreRunE` (mirroring
  `hook`/`integrate`) OVERRIDES root's `config.Load` — which needs a git repo and triggers first-run
  bootstrap. NO edit to root.go's `shouldSkipConfigLoad` (that hook is for `config init/path/upgrade`
  leaves; the group-level no-op PreRunE is the override mechanism cobra intends).
- **Zero-surprise pattern**: 3 sibling group commands (`hook`, `integrate`, `providers`) already use the
  identical shape. A 4th (`lock`) is the established way to add a diagnostic group — no new pattern, no
  root.go edit, no new dependency.

## What

**User-visible behavior**:
```
$ stagecoach lock status            # no lock held
no run lock for /home/me/proj

$ stagecoach lock status            # a live holder (self or another stagecoach run)
Lock: /run/user/1000/stagecoach/locks/ab12….lock
  pid:       4242
  hostname:  devbox
  repo:      /home/me/proj
  timestamp: 2026-07-10T12:00:00Z
  snapshot:  3f8a…                     # only when set
  alive:     true
  orphaned:  false                     # or "true (holder reparented — launcher has exited)"
                                       # or "unknown (holder is dead)" when alive=false

$ stagecoach lock                   # bare group → cobra help (no RunE on the group)
$ stagecoach lock status extra      # → cobra NoArgs error (Args: cobra.NoArgs)
```

**Technical change**: one new cobra group + leaf, registered via `init()`, delegating to `lock.Status`.

### Success Criteria
- [ ] `internal/cmd/lock.go` defines `lockCmd` (group, no-op `PersistentPreRunE`, no RunE) and
      `lockStatusCmd` (`Use: "status"`, `Args: cobra.NoArgs`, `RunE: runLockStatus`), and an `init()`
      that does `lockCmd.AddCommand(lockStatusCmd); rootCmd.AddCommand(lockCmd)` — NO edit to root.go.
- [ ] `lockCmd`'s no-op `PersistentPreRunE` is `func(*cobra.Command, []string) error { return nil }`
      (overrides root's config.Load — grep guard).
- [ ] `runLockStatus(cmd, _ []string) error` resolves repoDir via `os.Getwd()` (error →
      `exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))`), calls `lock.Status(repoDir)`
      (error → `exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err))`), and prints
      to `cmd.OutOrStdout()` per the format spec. Returns nil on success (incl. no-lock), the ExitError
      on failure. NEVER `os.Exit`.
- [ ] When `path == ""`: prints exactly `no run lock for <repoDir>\n` and returns nil (exit 0).
- [ ] When `path != ""`: prints `Lock: <path>\n` + indented pid/hostname/repo/timestamp + `snapshot:`
      (only if `contents.Snapshot != ""`) + `alive:` + the 3-way `orphaned:` line.
- [ ] [Mode A] Godoc on `runLockStatus` states: read-only (never acquires/breaks/removes the lock —
      FR52); the user decides whether to kill/rm; path=="" means no lock held.
- [ ] `internal/cmd/lock_test.go` covers: no-lock path (exit 0, "no run lock for"), live-self-holder via
      real `lock.Acquire` (exit 0, "Lock:", pid==os.Getpid, alive:true), and the Status/Getwd error path
      (exit 1). Tests isolate XDG + restore rootCmd via saveRootState/restoreRootState.
- [ ] `go build ./...` + GOOS=windows/linux/darwin all clean; `go vet ./internal/cmd/...` clean;
      `gofmt -l internal/cmd/lock.go internal/cmd/lock_test.go` empty.
- [ ] `go test ./internal/cmd/ -race` green (new tests + full cmd-package regression);
      `make test` + `make lint` clean.
- [ ] `git status --porcelain` shows ONLY `internal/cmd/lock.go` + `internal/cmd/lock_test.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact `lock.Status` signature + return semantics (now LANDED, quoted in full), the exact
`hook`/`integrate`/`providers` group-command pattern to clone (with line refs + the cobra "nearest
PersistentPreRunE wins" mechanism), the exact output format (with the 3-way orphaned display logic), the
exact exit-code contract (`exitcode.New(exitcode.Error, ...)` / nil; never os.Exit), the exact repoDir
resolution (`os.Getwd()` at 3 sites), the exact test harness (`saveRootState`/`restoreRootState`/
`chdir`/`Execute`), the lock-file isolation approach (`t.Setenv("XDG_RUNTIME_DIR", …)` + a real
`lock.Acquire` since `lockPath` is unexported), the macOS `/private/var` symlink gotcha for the repoDir
assertion, the scope fences (no edit to root.go/default_action.go/internal/lock), and 8 grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative subcommand + output-format spec (near-complete runLockStatus shape)
- docfile: plan/014_37208f58ffa2/architecture/cli_test_extension.md
  why: "'Subcommand registration pattern' gives the hook.go template + the EXACT lock.go skeleton (group
        cmd + leaf + init() registration). 'runLockStatus implementation shape' gives the verbatim output
        format (Lock: <path> + indented fields + the 3-way orphaned line + 'no run lock for <repo>')."
  critical: "The no-op PersistentPreRunE OVERRIDES root's config.Load (cobra runs only the nearest) — NO
             edit to root.go's shouldSkipConfigLoad. Exit codes: nil→0 (incl. no-lock), Error→1. Never os.Exit."

# MUST READ — codebase-specific findings for THIS item (Status contract confirmed LANDED, test harness,
#              lock-file isolation, the macOS symlink gotcha, scope fences, validation commands)
- docfile: plan/014_37208f58ffa2/P1M3T2S1/research/findings.md
  why: "§0 lock.Status is LANDED (quoted signature + path==''/error semantics) — CONSUME, don't re-impl;
        §1 the hook/integrate/providers group pattern (no-op PreRunE override, init() registration);
        §2 exit-code contract (nil→0 incl no-lock, exitcode.New(Error,...)→1, never os.Exit);
        §3 repoDir=os.Getwd(); §4 verbatim output format + 3-way orphaned logic;
        §5 test harness (saveRootState/restoreRootState/chdir/Execute) + XDG isolation + real lock.Acquire;
        §6 imports; §7 scope fences (no overlap with T3.S1/T1.S1/M4); §8 validation commands."
  critical: "lockPath is UNEXPORTED → tests can't plant a lock file at the exact path; use a REAL
             lock.Acquire for the live-holder test. macOS: assert 'no run lock for' via os.Getwd() AFTER
             chdir (t.TempDir()=/var/..., Getwd=/private/var/...). The 'no-lock' case returns nil (exit 0),
             NOT an error."

# MUST READ — the template file (clone this group-command structure exactly)
- file: internal/cmd/hook.go
  why: "hookCmd (line 39: Use/Short/Long/SilenceErrors/SilenceUsage + no-op PersistentPreRunE) is the
        group to clone; hookStatusCmd (line 65: Use/Short/Args:cobra.NoArgs/SilenceErrors/SilenceUsage/
        RunE) is the leaf to clone; init() (line 90: hookCmd.AddCommand(...); rootCmd.AddCommand(hookCmd))
        is the registration to clone; runHookStatus (line 154: resolve dir, call lib, print to
        cmd.OutOrStdout, return nil / exitcode.New(exitcode.Error,...)) is the RunE body to mirror."
  pattern: "Group with no-op PersistentPreRunE + leaves with Args:cobra.NoArgs; register via init() on
            rootCmd; RunE returns nil on success, exitcode.New(exitcode.Error, fmt.Errorf('stagecoach: %w', err)) on error."
  gotcha: "The package doc comment (hook.go:1-8) explains WHY the no-op PreRunE (overrides config.Load so
           it works outside a git repo + no bootstrap write). Write an analogous package/file doc for lock.go."

# MUST READ — the second twin (confirms the pattern is not hook-specific)
- file: internal/cmd/integrate.go
  why: "integrateCmd (line ~53) uses the IDENTICAL no-op PersistentPreRunE ('SKIP config.Load (like hook)');
        init() (line 88-89) does integrateCmd.AddCommand(...); rootCmd.AddCommand(integrateCmd). Confirms
        the pattern is the canonical way to add a diagnostic group — a 4th (lock) is zero-surprise."
  pattern: "Same as hook.go — cross-check the group/leaf/init() shape against a second example."

# CONTEXT — root.go (the rootCmd var + the config.Load PersistentPreRunE the no-op OVERRIDES; do NOT edit)
- file: internal/cmd/root.go
  why: "rootCmd (line 92) is the package-level var lock.go's init() will AddCommand onto (NO edit here).
        rootCmd.PersistentPreRunE (line 100) calls config.Load — this is what the lock group's no-op
        OVERRIDES (cobra runs only the nearest ancestor's PersistentPreRunE). shouldSkipConfigLoad (line
        ~245) is ONLY for config init/path/upgrade LEAVES — the lock group does NOT use it (the group-level
        no-op PreRunE is the override). Execute() (line ~258) is what tests call."
  critical: "Do NOT edit root.go. Do NOT add 'lock' to shouldSkipConfigLoad. The no-op PersistentPreRunE
             on lockCmd IS the override mechanism (proven by hook/integrate)."

# CONTEXT — the consumed API (lock.Status) — read its exact contract (LANDED, do not modify)
- file: internal/lock/lock.go
  why: "Status (line 321): func Status(repoPath string) (path string, contents LockContents, alive bool,
        orphan bool, err error). path=='' + nil err ⇒ no lock. err != nil ⇒ lockPath/ReadFile failure.
        LockContents (line 47): Pid, Hostname, Repo, Timestamp, Snapshot string (all string)."
  pattern: "Status godoc (line 309) states read-only (FR52) — mirror its tone in runLockStatus's godoc."
  gotcha: "Do NOT modify internal/lock/. This item CONSUMES Status. parseContents never errors (empty
           fields for malformed lines) so contents fields may be '' — print them as-is (they're diagnostic)."

# CONTEXT — exit-code mapper (verify the nil→0 / ExitError→1 contract)
- file: internal/exitcode/exitcode.go
  why: "For(err): nil→Success(0); *ExitError→its Code; else→Error(1). New(code, err) wraps a forced code.
        runLockStatus returns nil (exit 0 incl. no-lock) or exitcode.New(exitcode.Error, ...) (exit 1)."
  critical: "NEVER os.Exit — only main.go does that. Return the error; exitcode.For + main map it."

# CONTEXT — the test harness helpers (reuse, do not reinvent)
- file: internal/cmd/root_test.go
  why: "saveRootState (line 105) / restoreRootState (line 111) capture+restore rootCmd Out/Err/RunE + reset
        ALL changed flags. initRepo (line 25) / chdir (line 74) for repo-scoped tests (chdir auto-restores
        CWD via t.Cleanup). resetFlags resets pflag state between Executes."
  pattern: "Every Execute()-based test: saveRootState/defer restoreRootState; rootCmd.SetOut(&buf);
            rootCmd.SetArgs([]string{...}); Execute(ctx); assert buf + exitcode.For(err)."
  gotcha: "lock status needs NO git repo (no-op PreRunE skips config.Load) — initRepo is OPTIONAL. A plain
           t.TempDir() + chdir suffices for the no-lock/outside-git-repo test."

# CONTEXT — an existing in-package cmd test (the Execute() assertion idiom to mirror)
- file: internal/cmd/hook_test.go
  why: "TestHookInstall_Print (line 47) shows the exact idiom: saveRootState; defer restoreRootState +
        resetHookFlags; rootCmd.SetOut(&buf); rootCmd.SetErr(io.Discard); rootCmd.SetArgs(['hook','install',
        '--print']); Execute(ctx); assert buf + err. Mirror this for TestLockStatus_*."
  pattern: "Capture stdout via SetOut(&buf); discard stderr via SetErr(io.Discard); map err via exitcode.For."

# CONTEXT — PRD §9.27 FR-K4 (the requirement) + §15.3 (the CLI reference bullet)
- docfile: plan/014_37208f58ffa2/prd_snapshot.md
  section: "§9.27 FR-K4 + §15.3 'stagecoach lock status' bullet + §15.4 exit codes"
  why: "FR-K4: 'prints the path; the holder's parsed pid/hostname/repo/timestamp/snapshot; whether the
        holder process is alive; and — on Unix — whether it appears orphaned. With no lock held it prints
        \"no run lock for <repo>\". It changes nothing.' §15.3: identical bullet. §15.4: 0=success, 1=error."
  critical: "'Changes nothing' (read-only, FR52). The no-lock case is a SUCCESS (exit 0), not an error."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go                 # READ-ONLY — rootCmd var (line 92) + config.Load PersistentPreRunE (line 100) the no-op OVERRIDES
  hook.go                 # READ-ONLY — the group-command TEMPLATE to clone (hookCmd/hookStatusCmd/init())
  integrate.go            # READ-ONLY — 2nd twin of the same pattern (confirms it's canonical)
  providers.go            # READ-ONLY — 3rd twin (group + leaves via init())
  default_action.go       # READ-ONLY — handleLockContention (line 300) is P1.M3.T3.S1's; NOT this item
  hook_test.go            # READ-ONLY — the Execute() test idiom to mirror (saveRootState/restoreRootState)
  root_test.go            # READ-ONLY — test helpers (saveRootState/restoreRootState/initRepo/chdir/resetFlags)
  lock_contention_test.go # READ-ONLY — existing lock tests (handleLockContention); NOT this item
internal/lock/
  lock.go                 # READ-ONLY — Status (line 321, LANDED) is CONSUMED; LockContents (line 47)
  orphan_unix.go          # READ-ONLY — appearsOrphaned (reached via Status.orphan; never imported directly)
  orphan_windows.go       # READ-ONLY — appearsOrphaned always-false (FR-K7)
internal/exitcode/
  exitcode.go             # READ-ONLY — New/Error/For (the exit-code mapper)
Makefile                  # test=line 70 (-race); lint=line 103; coverage-gate=line 77 (NOT internal/cmd); build=line 52
```

### Desired Codebase tree with files to be added

```bash
internal/cmd/
  lock.go                 # NEW — lockCmd group (no-op PersistentPreRunE) + lockStatusCmd leaf + runLockStatus + init()
  lock_test.go            # NEW — TestLockStatus_NoLockHeld / _LockHeldAlive / _StatusError (saveRootState/restoreRootState)
# NOTHING ELSE. No edit to root.go, default_action.go, internal/lock/*, internal/exitcode/*, go.mod, or any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (cobra runs only the NEAREST ancestor's PersistentPreRunE): the lock group's no-op
// PersistentPreRunE OVERRIDES rootCmd's config.Load. This is WHY `lock status` works outside a git repo
// and never triggers config bootstrap (FR-B3) — WITHOUT editing root.go. Do NOT add 'lock' to
// shouldSkipConfigLoad (that hook is for config init/path/upgrade LEAVES; the group no-op is the override).
// Proven by hook.go (line 45) + integrate.go (line 54) — neither edits root.go.

// CRITICAL (lock.Status is the contract — LANDED by P1.M3.T1.S1; CONSUME, don't modify internal/lock/):
//   func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)
//   path=="" + err==nil  ⇒ NO LOCK HELD (os.IsNotExist on the lock file) — print "no run lock for <repo>".
//   err != nil           ⇒ lockPath failed (no XDG + no HOME) or a real ReadFile error — exit 1.
//   alive                ⇒ processAlive(pid, hostname); orphan ⇒ appearsOrphaned(pid) (only when alive).
//   LockContents         ⇒ {Pid, Hostname, Repo, Timestamp, Snapshot string} — print fields as-is (may be "").

// CRITICAL (the "no lock" case is a SUCCESS, exit 0): runLockStatus returns nil (→ exit 0) when path=="".
// It is a READ that found nothing, NOT an error. Only Getwd/Status failures return exitcode.New(exitcode.Error, ...).

// CRITICAL (NEVER os.Exit): only main.go calls os.Exit (via exitcode.For). RunE returns nil or an
// *exitcode.ExitError; exitcode.For + main map it. Mirror runHookStatus (hook.go:154).

// GOTCHA (lockPath is UNEXPORTED in internal/lock): from package cmd you CANNOT plant a lock file at the
// exact path. For the live-holder test, acquire a REAL lock via lock.Acquire(repoPath), run the subcommand,
// assert, defer l.Release(). The pid will be os.Getpid(), alive=true. (Dead-holder / orphan==true are E2E.)

// GOTCHA (lock-file isolation): the lock file lives OUTSIDE the repo (XDG_RUNTIME_DIR/XDG_CACHE_HOME/~/.cache).
// Each test MUST t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) (+ t.Setenv("XDG_CACHE_HOME","")) to avoid
// colliding with a real dev lock or leaking across tests. ALWAYS defer l.Release() (the `current` singleton).

// GOTCHA (macOS t.TempDir() symlink): t.TempDir() returns "/var/folders/..." but os.Chdir resolves it, so
// os.Getwd() returns "/private/var/folders/...". The "no run lock for <repoDir>" message uses the POST-CHDIR
// Getwd value. Assert via strings.Contains(out, "no run lock for") OR capture wd via os.Getwd() AFTER chdir.

// GOTCHA (print to cmd.OutOrStdout(), NOT stderr): the success output is the command's NORMAL result —
// users/scripts read stdout. Mirror hook.go:156 (fmt.Fprintln(cmd.OutOrStdout(), ...)). Only the (never-used
// here) foreign-hook refusal prints to ErrOrStderr.

// GOTCHA (cobra NoArgs): lockStatusCmd has Args: cobra.NoArgs — `stagecoach lock status extra` is a usage
// error (cobra handles it; SilenceUsage+SilenceErrors on the cmd means main maps the returned error to 1).
// No test strictly required (cobra's contract), but the bare-group help + NoArgs behavior are manual checks.
```

## Implementation Blueprint

### Data models and structure

None NEW. The subcommand consumes the existing `lock.LockContents` struct (`Pid, Hostname, Repo,
Timestamp, Snapshot string`) and the 4 scalar returns of `lock.Status`. No new types, no fields, no
packages, no flags.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/lock.go — the cobra group + leaf + RunE + init() (clone hook.go)
  - FILE DOC (package cmd): a 4-6 line comment explaining the `lock` group is a read-only diagnostic
    (FR-K4/§9.27), that its no-op PersistentPreRunE OVERRIDES root's config.Load (cobra runs only the
    nearest) so `lock status` works outside a git repo + never triggers config bootstrap (FR-B3), and that
    it is registered via init() with ZERO edits to root.go (the hook/integrate/providers pattern).
  - IMPORTS (all already used by sibling cmd files): fmt, os, github.com/spf13/cobra,
    github.com/dustin/stagecoach/internal/exitcode, github.com/dustin/stagecoach/internal/lock.
  - GROUP COMMAND (clone hook.go:39-47):
      var lockCmd = &cobra.Command{
          Use:               "lock",
          Short:             "Inspect the per-repo run lock (FR52/§9.27)",
          Long:              `Read-only diagnostics for stagecoach's per-repo run lock (PRD §9.27 FR-K4).`,
          SilenceErrors:     true,
          SilenceUsage:      true,
          PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load
      }
    No RunE → bare `stagecoach lock` prints help (the hook/integrate default).
  - LEAF COMMAND (clone hook.go:65-72):
      var lockStatusCmd = &cobra.Command{
          Use:           "status",
          Short:         "Print the run lock holder's pid/host/repo/liveness/orphan-status",
          Args:          cobra.NoArgs,
          SilenceErrors: true,
          SilenceUsage:  true,
          RunE:          runLockStatus,
      }
  - REGISTRATION (clone hook.go:90-93):
      func init() {
          lockCmd.AddCommand(lockStatusCmd)
          rootCmd.AddCommand(lockCmd) // register on root — NO edit to root.go (hook/integrate/providers pattern)
      }
  - NAMING: lockCmd, lockStatusCmd, runLockStatus (mirror hookCmd/hookStatusCmd/runHookStatus).
  - FOLLOW pattern: internal/cmd/hook.go (group + leaf + init + RunE) — IDENTICAL structure.
  - GOTCHA: the no-op PersistentPreRunE signature is `func(*cobra.Command, []string) error { return nil }`
    (cobra assigns param names only when used; the hook/integrate form omits them — match it).

Task 2: IMPLEMENT func runLockStatus(cmd *cobra.Command, _ []string) error — the read-only diagnostic
  - GODOC [Mode A]: "runLockStatus implements `stagecoach lock status` (PRD §9.27 FR-K4): a READ-ONLY
    diagnostic that prints the current repo's run-lock state — the lock path, the holder's parsed
    pid/hostname/repo/timestamp/snapshot, whether the holder is alive, and (Unix) whether it appears
    orphaned. It never acquires the flock and never breaks/removes a lock (FR52 preserved); the user
    decides whether to kill/rm. With no lock held it prints 'no run lock for <repoDir>' and exits 0 (a
    read that found nothing is a success, not an error). It works outside a git repo (the lock group's
    no-op PersistentPreRunE skips config.Load). Consumes lock.Status (P1.M3.T1.S1)."
  - BODY (mirror runHookStatus hook.go:154 + the format from architecture/cli_test_extension.md):
      repoDir, err := os.Getwd()
      if err != nil {
          return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
      }
      path, contents, alive, orphan, err := lock.Status(repoDir)
      if err != nil {
          return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err))
      }
      out := cmd.OutOrStdout()
      if path == "" {
          fmt.Fprintf(out, "no run lock for %s\n", repoDir)
          return nil // exit 0 — a read that found nothing
      }
      fmt.Fprintf(out, "Lock: %s\n", path)
      fmt.Fprintf(out, "  pid:       %s\n", contents.Pid)
      fmt.Fprintf(out, "  hostname:  %s\n", contents.Hostname)
      fmt.Fprintf(out, "  repo:      %s\n", contents.Repo)
      fmt.Fprintf(out, "  timestamp: %s\n", contents.Timestamp)
      if contents.Snapshot != "" {
          fmt.Fprintf(out, "  snapshot:  %s\n", contents.Snapshot)
      }
      fmt.Fprintf(out, "  alive:     %v\n", alive)
      switch {
      case orphan:
          fmt.Fprintln(out, "  orphaned:  true (holder reparented — launcher has exited)")
      case alive:
          fmt.Fprintln(out, "  orphaned:  false")
      default:
          fmt.Fprintln(out, "  orphaned:  unknown (holder is dead)")
      }
      return nil
  - FOLLOW pattern: runHookStatus (hook.go:154) for the Getwd→lib→print→return-nil/ExitError shape.
  - GOTCHA: print to cmd.OutOrStdout() (NOT stderr). The "no lock" case returns nil (exit 0).
  - GOTCHA: the orphaned line is a 3-way switch (orphan / alive-but-not-orphan / dead). On Windows
    processAlive is always true → reaches the `case alive:` branch (orphaned: false). "unknown" is dead-holder only.
  - GOTCHA: NEVER os.Exit, NEVER lock.Acquire, NEVER os.Remove — read-only (grep guard).

Task 3: CREATE internal/cmd/lock_test.go (package cmd) — reuse saveRootState/restoreRootState + a real lock.Acquire
  - IMPORTS: bytes, context, io, os, strings, testing; github.com/dustin/stagecoach/internal/exitcode,
    github.com/dustin/stagecoach/internal/lock. (Match hook_test.go's import set.)
  - TestLockStatus_NoLockHeld (the path=="" contract + outside-git-repo works):
      - isolate XDG: t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME", "")
      - repo := t.TempDir(); chdir(t, repo)   // NOT a git repo — proves the no-op PreRunE works outside git
      - saveRootState/defer restoreRootState; rootCmd.SetOut(&buf); rootCmd.SetErr(io.Discard);
        rootCmd.SetArgs([]string{"lock", "status"}); err := Execute(context.Background())
      - assert exitcode.For(err)==exitcode.Success (nil → 0); assert strings.Contains(buf.String(),
        "no run lock for")  (use Contains to dodge the macOS /private/var symlink nit; OR capture
        repoDir via os.Getwd() after chdir and assert exact match).
  - TestLockStatus_LockHeldAlive (the live-holder path via a REAL lock.Acquire):
      - isolate XDG (same t.Setenv as above); repo := t.TempDir(); chdir(t, repo)
      - l, err := lock.Acquire(repo); if err != nil { t.Fatal(err) }; defer l.Release()
      - saveRootState/defer restoreRootState; rootCmd.SetOut(&buf); rootCmd.SetErr(io.Discard);
        rootCmd.SetArgs([]string{"lock", "status"}); err = Execute(context.Background())
      - assert exitcode.For(err)==exitcode.Success
      - out := buf.String(); assert strings.Contains(out, "Lock:"); assert strings.Contains(out,
        "pid:       "+strconv.Itoa(os.Getpid())) (the holder is THIS test process); assert
        strings.Contains(out, "alive:     true"); assert strings.Contains(out, "orphaned:")
        (either "false" or the reparented line — assert the field is PRESENT, not its exact value, since
        CI-under-init could differ; see GOTCHA).
  - TestLockStatus_StatusErrorPropagation (the exit-1 path — force lockDir failure):
      - t.Setenv("XDG_RUNTIME_DIR", ""); t.Setenv("XDG_CACHE_HOME", ""); t.Setenv("HOME", "")
        // forces lockDir's os.UserHomeDir error → Status returns err
      - repo := t.TempDir(); chdir(t, repo)
      - saveRootState/defer restoreRootState; rootCmd.SetOut(&buf); rootCmd.SetErr(io.Discard);
        rootCmd.SetArgs([]string{"lock", "status"}); err := Execute(context.Background())
      - assert err != nil; assert exitcode.For(err)==exitcode.Error (1)
      - GOTCHA: os.UserHomeDir with empty HOME fails on Linux/Darwin (returns $HOME error). On some CI
        runners HOME may be set by the runner env outside t.Setenv's reach — if flaky, mark t.Skip with a
        comment, OR prefer asserting exitcode.For(err)==exitcode.Error when err!=nil (the core contract).
  - FOLLOW pattern: hook_test.go TestHookInstall_Print (line 47) for the saveRootState/SetOut/SetArgs/
    Execute/assert idiom; root_test.go saveRootState/restoreRootState/chdir signatures.
  - NAMING: TestLockStatus_<Scenario>. snake-camel as the existing cmd tests.
  - GOTCHA: the dead-holder + orphan==true scenarios are E2E (P1.M4.T1.S1) — do NOT unit-test them (a
    real reparented-to-init pid is flaky/OS-dependent). Add a comment stating this.
  - GOTCHA: defer l.Release() is MANDATORY in the alive test (the `current` singleton must not leak).

Task 4: VERIFY — build (native+cross-compile), vet, format, full regression, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...
  - go vet ./internal/cmd/... ; gofmt -l internal/cmd/lock.go internal/cmd/lock_test.go   # empty
  - go test ./internal/cmd/ -run 'TestLockStatus' -race -v ; go test ./internal/cmd/ -race
  - make test ; make lint ; make build
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the diagnostic group (clone of hook.go/integrate.go). The no-op PersistentPreRunE is the
// config.Load override — cobra runs only the NEAREST ancestor's, so this wins WITHOUT editing root.go.
var lockCmd = &cobra.Command{
	Use:               "lock",
	Short:             "Inspect the per-repo run lock (FR52/§9.27)",
	Long:              `Read-only diagnostics for stagecoach's per-repo run lock (PRD §9.27 FR-K4).`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load
}

var lockStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Print the run lock holder's pid/host/repo/liveness/orphan-status",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runLockStatus,
}

func init() {
	lockCmd.AddCommand(lockStatusCmd)
	rootCmd.AddCommand(lockCmd) // register on root — NO edit to root.go (hook/integrate/providers pattern)
}

// PATTERN: the read-only RunE (resolve repoDir → call lock.Status → print → return nil/ExitError).
func runLockStatus(cmd *cobra.Command, _ []string) error {
	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	path, contents, alive, orphan, err := lock.Status(repoDir)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err))
	}
	out := cmd.OutOrStdout()
	if path == "" {
		fmt.Fprintf(out, "no run lock for %s\n", repoDir) // a read that found nothing → exit 0
		return nil
	}
	fmt.Fprintf(out, "Lock: %s\n", path)
	fmt.Fprintf(out, "  pid:       %s\n", contents.Pid)
	fmt.Fprintf(out, "  hostname:  %s\n", contents.Hostname)
	fmt.Fprintf(out, "  repo:      %s\n", contents.Repo)
	fmt.Fprintf(out, "  timestamp: %s\n", contents.Timestamp)
	if contents.Snapshot != "" {
		fmt.Fprintf(out, "  snapshot:  %s\n", contents.Snapshot)
	}
	fmt.Fprintf(out, "  alive:     %v\n", alive)
	switch {
	case orphan:
		fmt.Fprintln(out, "  orphaned:  true (holder reparented — launcher has exited)")
	case alive:
		fmt.Fprintln(out, "  orphaned:  false") // Windows always lands here (processAlive is always true)
	default:
		fmt.Fprintln(out, "  orphaned:  unknown (holder is dead)")
	}
	return nil // exit 0 — even when the holder is dead/orphaned; the USER decides whether to kill/rm
}
```

### Integration Points

```yaml
CLI SURFACE:
  - ADD `stagecoach lock` (group, bare → help) + `stagecoach lock status` (leaf, the diagnostic).
  - REGISTER via init() on the package-level rootCmd var — NO edit to root.go.
  - The `lock` group joins the existing groups: providers, config, hook, integrate (cobra help lists it).

IMPORTS (internal/cmd/lock.go, NEW file):
  - fmt, os (stdlib)
  - github.com/spf13/cobra (already the CLI framework)
  - github.com/dustin/stagecoach/internal/exitcode (exit-code mapper)
  - github.com/dustin/stagecoach/internal/lock (lock.Status — the read-only API, LANDED by P1.M3.T1.S1)
  - NO new third-party dependency (go.mod unchanged).

NO database / migration / routes / new types / new flag / config change / root.go edit.
  - The Busy-message REFORMAT (orphan hint in handleLockContention) is P1.M3.T3.S1 (separate item) — NOT here.
  - The README CLI-reference sync is P1.M4.T2.S1 — NOT here (this item adds ONLY godoc [Mode A]).
  - The E2E scenarios (dead-holder, orphan==true, live-holder via real binary) are P1.M4.T1.S1 — NOT here.

SCOPE FENCES:
  - Touches ONLY internal/cmd/lock.go (NEW) + internal/cmd/lock_test.go (NEW).
  - Does NOT edit root.go, default_action.go, hook.go, integrate.go, providers.go, internal/lock/*,
    internal/exitcode/*, cmd/stagecoach/main.go, go.mod, or any PRD/task file.
  - Adds NO flag, NO exported type, NO third-party dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-compile (lock.go has no build tags itself, but it imports internal/lock which has them —
# all four OS targets must build).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean. If GOOS=windows fails, you accidentally referenced a Unix-only symbol.

# Vet.
go vet ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/lock.go internal/cmd/lock_test.go
# Expected: empty. If listed: gofmt -w internal/cmd/lock.go internal/cmd/lock_test.go

# Lint.
make lint   # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. (lockCmd/lockStatusCmd are used by init(); runLockStatus by the leaf's RunE.)

# Scope guard: ONLY the two new files changed.
git status --porcelain
# Expected: internal/cmd/lock.go, internal/cmd/lock_test.go. ZERO changes elsewhere (esp. NOT root.go,
#           default_action.go, or internal/lock/).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new subcommand tests.
go test ./internal/cmd/ -run 'TestLockStatus' -race -v
# Expected: TestLockStatus_NoLockHeld / _LockHeldAlive / _StatusErrorPropagation all PASS.
#   - NoLockHeld: exit 0, stdout contains "no run lock for".
#   - LockHeldAlive: exit 0, stdout contains "Lock:", "pid:       <getpid>", "alive:     true", "orphaned:".
#   - StatusErrorPropagation: exit 1 (exitcode.Error).

# Full cmd-package regression (the new subcommand registers on rootCmd — ensure no flag/help collision).
go test ./internal/cmd/ -race
# Expected: green (existing hook/providers/integrate/default/lock-contention tests all green + new tests).

# Full race suite.
make test
# Expected: green. internal/cmd has no shared mutable state in the new code (runLockStatus reads + prints).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (proves the subcommand is registered + links into the binary).
make build

# Manual: outside a git repo (proves the no-op PreRunE skips config.Load — the core FR-K4 property).
cd "$(mktemp -d)"   # a bare temp dir, NOT a git repo
"$(pwd)/../../bin/stagecoach" lock status   # → "no run lock for <cwd>", exit 0
echo "exit=$?"

# Manual: the bare group prints help (no RunE on lockCmd).
bin/stagecoach lock            # → cobra help listing `status`
echo "exit=$?"                 # 0 (help is success)

# Manual: NoArgs rejection.
bin/stagecoach lock status extra   # → usage error, exit 1

# Manual: live-holder (run a real generation in another shell, then `lock status` here).
# (Covered unit-test-wise by TestLockStatus_LockHeldAlive via lock.Acquire; the real-binary live path is
#  the E2E harness's job — P1.M4.T1.S1.)

# Expected: "no run lock" exits 0; help exits 0; the NoArgs error exits 1; the live-holder print exits 0.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: lock.go defines the group with a no-op PersistentPreRunE (the config.Load override).
grep -n 'PersistentPreRunE: func(\*cobra.Command, \[\]string) error { return nil }' internal/cmd/lock.go
# Expect: 1 hit (on lockCmd).

# Guard 2: lockStatusCmd has Args: cobra.NoArgs + RunE: runLockStatus.
grep -n 'Args: *cobra.NoArgs' internal/cmd/lock.go
grep -n 'RunE: *runLockStatus' internal/cmd/lock.go
# Expect: 1 hit each.

# Guard 3: registration via init() on rootCmd — NO edit to root.go.
grep -n 'rootCmd.AddCommand(lockCmd)' internal/cmd/lock.go
grep -n 'lockCmd.AddCommand(lockStatusCmd)' internal/cmd/lock.go
git diff --name-only | grep -q '^internal/cmd/root\.go$' && echo "FAIL: root.go edited" || echo "OK: root.go untouched"

# Guard 4: runLockStatus is READ-ONLY — it must NOT call lock.Acquire, os.Remove, flock, or os.Exit.
grep -nE 'lock\.Acquire|os\.Remove|flock|os\.Exit' internal/cmd/lock.go
# Expect: ZERO hits. (It calls ONLY os.Getwd + lock.Status + fmt.Fprint* + exitcode.New.)

# Guard 5: runLockStatus calls lock.Status with the 5-value signature.
grep -n 'path, contents, alive, orphan, err := lock.Status(repoDir)' internal/cmd/lock.go
# Expect: 1 hit.

# Guard 6: the "no lock" case returns nil (exit 0), NOT an error.
grep -A2 'no run lock for %s' internal/cmd/lock.go
# Expect: the fmt.Fprintf line immediately followed by `return nil`.

# Guard 7: the orphaned line uses the 3-way switch (orphan / alive-but-not-orphan / dead).
grep -n 'orphaned:' internal/cmd/lock.go
# Expect: 3 hits (true/false/unknown) OR a switch with 3 cases printing "orphaned:".

# Guard 8: never os.Exit in the new file.
grep -n 'os\.Exit' internal/cmd/lock.go
# Expect: ZERO hits.

# Guard 9: scope — only the two new files.
git status --porcelain
# Expect: internal/cmd/lock.go + internal/cmd/lock_test.go ONLY.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean
- [ ] `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l internal/cmd/lock.go internal/cmd/lock_test.go` empty
- [ ] `make lint` zero errors (lockCmd/lockStatusCmd/runLockStatus all referenced)
- [ ] `go test ./internal/cmd/ -race` green (new TestLockStatus_* + full cmd-package regression)
- [ ] `make test` (full race suite) green

### Feature Validation
- [ ] `stagecoach lock status` with no lock prints `no run lock for <repoDir>` and exits 0 (nil error)
- [ ] `stagecoach lock status` with a held lock prints `Lock: <path>` + 7 indented fields and exits 0
- [ ] `stagecoach lock status` works OUTSIDE a git repo (no-op PreRunE skips config.Load — no bootstrap, no error)
- [ ] The "no lock" case returns nil (exit 0), NOT exitcode.New(Error,...) (grep guard 6)
- [ ] The orphaned display is the 3-way switch (orphan / alive-not-orphan / dead-unknown) (grep guard 7)
- [ ] `stagecoach lock` (bare group) prints help; `lock status extra` is a NoArgs error
- [ ] Error path (Getwd/Status failure) returns `exitcode.New(exitcode.Error, ...)` → exit 1

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/lock.go` + `internal/cmd/lock_test.go`
- [ ] NO edit to root.go, default_action.go, hook.go, integrate.go, providers.go, internal/lock/*,
      internal/exitcode/*, cmd/stagecoach/main.go, go.mod, or any test outside the two new files
- [ ] NO new flag, NO new exported TYPE, NO new third-party dependency (go.mod unchanged)
- [ ] NO Busy-message change (that's P1.M3.T3.S1), NO README sync (P1.M4.T2.S1), NO E2E scenarios (P1.M4.T1.S1)

### Code Quality & Docs
- [ ] [Mode A] Godoc on `runLockStatus`: read-only (never acquires/breaks/removes the lock — FR52), the user
      decides whether to kill/rm, path=="" means no lock held, works outside a git repo
- [ ] File doc comment on lock.go explains the no-op PersistentPreRunE override (config.Load skip) + the
      init()-on-rootCmd pattern (why no root.go edit)
- [ ] Follows the hook/integrate/providers group-command pattern exactly (clone, not a new pattern)
- [ ] Tests reuse saveRootState/restoreRootState/chdir + a real lock.Acquire; document the dead-holder/
      orphan==true scenarios are E2E (P1.M4.T1.S1)

---

## Anti-Patterns to Avoid

- ❌ Don't edit root.go. The `lock` group registers via `init()` on the package-level `rootCmd` var (the
  hook/integrate/providers pattern). Do NOT add `lock` to `shouldSkipConfigLoad` — the group's no-op
  `PersistentPreRunE` IS the override (cobra runs only the nearest ancestor's; proven by hook/integrate).
- ❌ Don't omit the no-op `PersistentPreRunE` on `lockCmd`. Without it, root's `config.Load` runs for
  `lock status` — which needs a git repo and triggers the first-run bootstrap write (FR-B3), breaking the
  "works outside a git repo" diagnostic property. The no-op is load-bearing, not decorative.
- ❌ Don't make the "no lock" case an error. `path==""` with `err==nil` is a SUCCESSFUL read that found
  nothing — print `no run lock for <repoDir>` and `return nil` (exit 0). Only Getwd/Status failures exit 1.
- ❌ Don't call `os.Exit`. Only main.go does that (via exitcode.For). Return nil or
  `exitcode.New(exitcode.Error, ...)`; exitcode.For + main map it. (grep guard 8.)
- ❌ Don't call `lock.Acquire`, `os.Remove`, `flock`, or any mutation in `runLockStatus`. It is READ-ONLY
  (FR-K4: "It changes nothing"; FR52 preserved). It calls ONLY `os.Getwd` + `lock.Status` + `fmt.Fprint*` +
  `exitcode.New`. (grep guard 4.)
- ❌ Don't print the success output to stderr. It is the command's NORMAL result — users/scripts read stdout.
  Use `cmd.OutOrStdout()` (mirror hook.go:156). (Only never-used-here refusal paths use ErrOrStderr.)
- ❌ Don't plant a lock file by computing the path in the test. `lockPath` is UNEXPORTED in `internal/lock`,
  unreachable from `package cmd`. Acquire a REAL lock via the exported `lock.Acquire(repo)` for the
  live-holder test (the pid is then os.Getpid(), alive=true). The dead-holder + orphan==true cases are E2E.
- ❌ Don't forget XDG isolation in tests. The lock file lives OUTSIDE the repo (XDG_RUNTIME_DIR/…). Without
  `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` a test could collide with a real dev lock or leak across tests.
  Always `defer l.Release()` (the `current` singleton).
- ❌ Don't assert the exact "no run lock for <repoDir>" path on macOS without resolving the symlink.
  `t.TempDir()` is `/var/...` but `os.Getwd()` after chdir is `/private/var/...`. Use `strings.Contains(out,
  "no run lock for")` OR assert against a Getwd captured AFTER chdir.
- ❌ Don't reinvent the test harness. `saveRootState`/`restoreRootState`/`chdir`/`Execute` already exist
  (root_test.go, hook_test.go). Reuse them — cobra's rootCmd is a package-level singleton; without
  restoreRootState + resetFlags, flag/state leaks between tests.
- ❌ Don't add a flag, a RunE to the group, or a third-party dep. FR-K4 is a zero-flag read-only diagnostic;
  bare `stagecoach lock` prints help (no RunE); cobra/exitcode/lock are all already present. go.mod unchanged.
- ❌ Don't overlap with siblings. The Busy-message orphan hint is P1.M3.T3.S1 (handleLockContention in
  default_action.go); the README CLI-reference bullet is P1.M4.T2.S1; the dead-holder/orphan E2E scenarios
  are P1.M4.T1.S1. This item is ONLY `internal/cmd/lock.go` + `internal/cmd/lock_test.go`.
