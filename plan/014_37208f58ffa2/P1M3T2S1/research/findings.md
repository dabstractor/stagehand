# Research Findings — P1.M3.T2.S1: `stagecoach lock status` cobra subcommand (FR-K4)

## 0. Dependency status — `lock.Status` is LANDED (do not re-implement)

`P1.M3.T1.S1` is **Complete**: `lock.Status` + `appearsOrphaned` already exist in the tree. This item
**consumes** them — it does NOT touch `internal/lock/`. Confirmed by direct read:

```go
// internal/lock/lock.go:321 (LANDED)
func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)
```
- `path==""` + `err==nil` ⇒ **no lock held** (the lock file does not exist — `os.IsNotExist`).
- `err != nil` ⇒ `lockPath` failed (no XDG + no HOME + no CWD fallback) OR a real `os.ReadFile` error.
- `alive` = `processAlive(pid, contents.Hostname)`; `orphan` = `appearsOrphaned(pid)` called **only when alive**.
- `contents` is the existing `LockContents` struct: `Pid, Hostname, Repo, Timestamp, Snapshot string`
  (lock.go:47). `parseContents` never errors (empty fields for malformed lines); malformed pid ⇒ alive/orphan false.
- READ-ONLY by construction (no flock, no os.Remove) — FR52 preserved. The Status godoc already states this.

`appearsOrphaned` exists in BOTH `internal/lock/orphan_unix.go` (`//go:build !windows`, runtime.GOOS
Linux /proc + Darwin ps) and `internal/lock/orphan_windows.go` (`//go:build windows`, always false, FR-K7).
Unexported — reached ONLY through `Status`'s `orphan` return. My item never imports it directly.

## 1. The command-group template — `internal/cmd/hook.go` (and the twin `integrate.go`)

`hook.go` is the authoritative pattern for a **diagnostic group that must NOT load config**. Three siblings
already use the identical shape (`hook.go`, `integrate.go`, `providers.go`), so a 4th (`lock.go`) is
zero-surprise. Key elements (verified by reading all three):

- **Group command** with a no-op `PersistentPreRunE`:
  ```go
  var lockCmd = &cobra.Command{
      Use: "lock", Short: "...", SilenceErrors: true, SilenceUsage: true,
      PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
  }
  ```
  **WHY the no-op**: cobra runs only the NEAREST ancestor's `PersistentPreRunE`. rootCmd's
  `PersistentPreRunE` calls `config.Load` (which triggers the first-run bootstrap write — FR-B3 — and needs
  a git repo). The `lock` group's no-op OVERRIDES it, so `lock status`:
  (a) works outside a git repo (FR-K4 diagnostic must be runnable anywhere CWD is), and
  (b) never triggers config bootstrap. **NO edit to root.go's `shouldSkipConfigLoad` is needed** — the no-op
  PreRunE is the override mechanism (confirmed: `hook`/`integrate` use it, neither touches shouldSkipConfigLoad;
  shouldSkipConfigLoad is ONLY for `config init/path/upgrade` which set it on the LEAF).

- **Leaf command** with `Args: cobra.NoArgs`, `SilenceErrors: true`, `SilenceUsage: true`, `RunE: runLockStatus`.
  (SilenceErrors/SilenceUsage on EVERY cmd — root.go:70 controls all output via `exitcode.For` in main.)

- **Registration** via `func init()`: `lockCmd.AddCommand(lockStatusCmd); rootCmd.AddCommand(lockCmd)`.
  ZERO edits to root.go (the `rootCmd` var is package-level and accessible from sibling files).

- **Bare `stagecoach lock`** (no RunE on the group) ⇒ cobra prints help (the `hook`/`integrate` default).

## 2. Exit-code contract — `internal/exitcode` (verified)

`exitcode.For(err)` is the single mapper in `main`. Constants: `Success=0`, `Error=1`, `Busy=5`, etc.
For this item:
- `runLockStatus` returns **`nil`** on success → exit **0** — including the "no lock held" case (it is a
  successful READ that found nothing, NOT an error). This is explicit in the item contract.
- `runLockStatus` returns **`exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err))`**
  on the `os.Getwd` failure or the `lock.Status` error path → exit **1**.
- **NEVER `os.Exit`** — only `main.go` does that. Return the error; `exitcode.For` + main handle the rest.
- Pattern is identical to `runHookStatus` / `runHookInstall` (hook.go:96, 113, 156) — they all return
  `exitcode.New(exitcode.Error, ...)` or `nil`.

## 3. repoDir resolution — `os.Getwd()` (verified at 3 sites)

`runLockStatus` resolves the repo via `os.Getwd()` — EXACTLY as:
- `root.go` PersistentPreRunE (`repoDir, err := os.Getwd()`),
- `default_action.go:62`,
- `hook.go:72` (`hooksDir`: `repoDir, err := os.Getwd()`).

No flag, no argument (FR-K4 says "for the current repo"). Wrap the error as
`exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))` (mirror hook.go:120 `getwd` wrap).

## 4. Output format — from `architecture/cli_test_extension.md` (authoritative)

The architecture doc specifies the EXACT format. Use it verbatim (column alignment matters for
copy-paste of the path/pid):

```
# path == "":
no run lock for <repoDir>

# path != "":
Lock: <path>
  pid:       <contents.Pid>
  hostname:  <contents.Hostname>
  repo:      <contents.Repo>
  timestamp: <contents.Timestamp>
  snapshot:  <contents.Snapshot>      # ONLY if contents.Snapshot != ""
  alive:     <true|false>
  orphaned:  <see 3-way logic below>
```

**3-way `orphaned` display logic** (the diagnostic nuance FR-K4/K7 imply):
- `orphan == true` → `orphaned:  true (holder reparented — launcher has exited)`
- `orphan == false && alive == true` → `orphaned:  false`
- `alive == false` → `orphaned:  unknown (holder is dead)`   # dead holder's orphan status is moot

(On Windows, `processAlive` is always true → always reaches the `alive==true` branch, so Windows shows
`alive: true` / `orphaned: false`. The "unknown" branch is Unix-dead-holder only. This matches FR-K7's
"reports liveness via the platform process check where available and unknown otherwise" — "unknown" =
holder is dead, can't assess reparenting.)

Print to `cmd.OutOrStdout()` (NOT stderr — it's the command's normal output; mirror hook.go:148 `st.String()`).
NEVER print to stderr for the success path (the user/script reads stdout).

## 5. Test pattern — `internal/cmd` test helpers (verified in root_test.go + hook_test.go)

Tests are `package cmd` (in-package, like hook_test.go). The established harness:
- `saveRootState(t)` / `restoreRootState(t, ...)` (root_test.go:105/111) capture/restore rootCmd's
  Out/Err/RunE + reset ALL changed flags (persistent + local) via `resetFlags`. **REQUIRED** — cobra's
  rootCmd is a package-level singleton; without restore, flag state leaks between tests.
- `rootCmd.SetOut(&out)`, `rootCmd.SetErr(...)`, `rootCmd.SetArgs([]string{"lock", "status"})`,
  then `Execute(context.Background())`. Assert on `out.String()` and `exitcode.For(err)`.
- `initRepo(t, dir)` + `chdir(t, dir)` (root_test.go:25/74) for repo-scoped tests. chdir registers a
  cleanup restoring CWD. **NOTE: `lock status` does NOT need a git repo** (the no-op PreRunE skips
  config.Load) — so a plain `t.TempDir()` + chdir suffices; `initRepo` is NOT required for these tests.
  (Use it anyway only if zero-effort; it does no harm.)

**Lock-file isolation in tests** (CRITICAL — the lock file lives OUTSIDE the repo):
- The lock dir resolves via `XDG_RUNTIME_DIR` → `XDG_CACHE_HOME` → `~/.cache/stagecoach/locks`.
  Each test MUST isolate via `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` (+ `t.Setenv("XDG_CACHE_HOME","")`)
  so it neither collides with a real dev lock nor leaks across tests.
- **`lockPath` is UNEXPORTED** in package `lock` → from `package cmd` you CANNOT plant a lock file at the
  exact path. The clean approach: acquire a REAL lock via the exported `lock.Acquire(repoPath)`, run the
  subcommand (it reads the same file via `Status`), assert, then `defer l.Release()`. The pid will be
  `os.Getpid()`, alive=true. This is deterministic and is the recommended unit-test net.
  (The dead-holder / orphan==true scenarios are the E2E harness's job — P1.M4.T1.S1.)

**macOS symlink gotcha** (from the lock package tests): `t.TempDir()` returns `/var/folders/...` on macOS,
but `os.Chdir` resolves the symlink, so `os.Getwd()` returns `/private/var/folders/...`. The "no run lock
for <repoDir>" message uses the POST-CHDIR `os.Getwd()` value. Assert against a value captured via
`wd, _ := os.Getwd()` AFTER chdir — NOT against the raw `t.TempDir()` return. (Or use
`strings.Contains(out, "no run lock for")` for the no-lock test to avoid the path-canonicalization nit.)

**The `current` singleton leak**: `lock.Acquire` stores into the package-global `current atomic.Pointer`.
ALWAYS `defer l.Release()` so a held lock doesn't outlive the test and contend with later tests. (Different
repo hashes → different files → no contention between tests, but releasing is still hygienic.)

## 6. Imports for internal/cmd/lock.go (verified — all already used by sibling cmd files)

- `fmt` (Errorf, Fprintf)
- `os` (Getwd)
- `github.com/spf13/cobra`
- `github.com/dustin/stagecoach/internal/exitcode`
- `github.com/dustin/stagecoach/internal/lock`

NO new third-party dependency (cobra is already the CLI framework; exitcode + lock are internal).

## 7. Scope boundaries (no overlap with siblings)

- **P1.M3.T1.S1** (DONE) — provides `lock.Status`. My item CONSUMES it; does NOT edit `internal/lock/`.
- **P1.M3.T3.S1** (planned) — refactors `handleLockContention` in `default_action.go:300` (the Busy-message
  orphan hint). My item does NOT touch `default_action.go` — no overlap.
- **P1.M4.T1.S1** (planned) — E2E scenarios for `lock status` (live/dead/orphan/no-lock). My item provides
  the unit-test net; the E2E harness exercises the real binary + the hard-to-unit-test branches.
- **P1.M4.T2.S1** (planned) — README CLI reference (`stagecoach lock status` bullet at §15.3). My item adds
  ONLY godoc [Mode A]; the README sync is the docs task.
- This item touches ONLY: `internal/cmd/lock.go` (NEW) + `internal/cmd/lock_test.go` (NEW). NO edit to
  root.go, default_action.go, internal/lock/*, internal/exitcode/*, or any PRD/task file.

## 8. Validation commands (verified against Makefile + existing cmd tests)

```bash
go build ./...                         # links the new subcommand into the binary
GOOS=windows go build ./...            # the lock pkg's build tags are consumed; lock.go has none — clean
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
go vet ./internal/cmd/...
gofmt -l internal/cmd/lock.go internal/cmd/lock_test.go   # empty
go test ./internal/cmd/ -run 'TestLockStatus' -race -v    # the new tests
go test ./internal/cmd/ -race                            # full cmd package regression (hook/providers/integrate/default all green)
make test                                               # full race suite
make lint                                               # golangci-lint
git status --porcelain                                  # ONLY internal/cmd/lock.go + internal/cmd/lock_test.go
```

`internal/cmd` is NOT in the coverage-gate list (Makefile:77 gates `internal/{git,provider,generate,config}`
only), so no coverage threshold pressure. The `make build` binary proves the subcommand is registered.

## 9. Manual sanity (Level 3)

```bash
make build
cd /tmp/empty-repo-or-anywhere        # NOT a git repo — proves the no-op PreRunE
./bin/stagecoach lock status          # → "no run lock for <cwd>"  exit 0
# (live-holder case: run a real stagecoach generation in another shell, then `lock status` here)
./bin/stagecoach lock                 # → cobra help for the group (no RunE)
./bin/stagecoach lock status extra    # → cobra NoArgs error (Args: cobra.NoArgs)
```
