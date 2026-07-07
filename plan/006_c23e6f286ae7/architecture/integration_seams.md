# Integration Seams — Exact Touch Points for FR52

> Every line number and signature below was verified against the live codebase on 2026-07-03.
> PRP agents should treat these as authoritative — do not re-derive; use these exact seams.

---

## 1. `internal/cmd/default_action.go` — the lock acquire/release funnel

### Acquire point (after repoDir resolution, before auto-stage state machine)

```go
// internal/cmd/default_action.go:48-55 (EXISTING)
repoDir, err := os.Getwd()
if err != nil {
    return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
}
g := git.New(repoDir)
// ← ACQUIRE LOCK HERE. repoDir is the lock key. Everything below is "the run".
//   Pattern:
//   locker, lockErr := lock.Acquire(repoDir)
//   if lockErr != nil {
//       return lock.HandleAcquireError(stderr, lockErr, g, ctx)
//           // → exit 0 (no-op fast path) or exitcode.New(exitcode.Busy, nil)
//   }
//   defer locker.Release()
```

This single insertion point covers BOTH the single-commit path (lines 73–204) and the
decompose path (lines 95–110 → `runDecompose`). `defer locker.Release()` fires on every
early-exit: nothing-to-commit (exit 2), dry-run success (exit 0), CAS failure (exit 1),
rescue (exit 3), push failure (exit 1). Read-only subcommands have their own `RunE` and
never reach `runDefault` — they bypass the lock naturally.

### `snapshot=` update is NOT in runDefault — it's in the library

The snapshot is captured inside `stagecoach.GenerateCommit` (→ `generate.CommitStaged`) and
inside `decompose.Decompose`. The holder publishes it via the lock package's singleton
`lock.SetSnapshot(sha)` (see `system_context.md` §2). `runDefault` does NOT need to know the
tree SHA.

---

## 2. `internal/exitcode/exitcode.go` — add `Busy = 5`

### Current constants (lines 22–30)
```go
const (
    Success         = 0
    Error           = 1
    NothingToCommit = 2
    Rescue          = 3
    Timeout         = 124
)
```

### Add `Busy`
```go
const (
    Success         = 0
    Error           = 1
    NothingToCommit = 2
    Rescue          = 3
    Busy            = 5   // NEW — another stagecoach run holds the per-repo lock; retry later
    Timeout         = 124
)
```

**Value rationale:** `5` is in the 1–7 range, distinct from all existing codes (0/1/2/3/124),
and does not collide with sysexits.h conventions that might confuse scripts. `4` is reserved
for potential future use.

### `For()` mapping — no change needed

`Busy` is only ever returned via explicit `exitcode.New(exitcode.Busy, nil)` from
`runDefault`. `For()`'s existing `*ExitError` short-circuit (line ~57: `var ee *ExitError;
if errors.As(err, &ee) { return ee.Code }`) handles it. No new `errors.Is` arm is needed.

---

## 3. `internal/generate/generate.go` — `lock.SetSnapshot` wiring (single path)

```go
// internal/generate/generate.go ~line 177 (EXISTING)
treeSHA, err := deps.Git.WriteTree(ctx)
// ... error handling ...

// internal/generate/generate.go ~line 186 (EXISTING)
signal.SetSnapshot(treeSHA, parentSHA, "")
// ← ADD: lock.SetSnapshot(treeSHA)   (one line, mirrors the signal.SetSnapshot call)
```

`lock.SetSnapshot` is a no-op when no lock is held (tests, library-only use). Import:
`import "github.com/dustin/stagecoach/internal/lock"`.

---

## 4. `internal/decompose/decompose.go` — `lock.SetSnapshot` wiring (decompose path)

```go
// internal/decompose/decompose.go ~line 164 (EXISTING)
tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
// ... error handling ...
// ← ADD: lock.SetSnapshot(tStart)   (one line, after the freeze)
```

Import: `import "github.com/dustin/stagecoach/internal/lock"`.

---

## 5. `internal/config/file.go` — the XDG pattern to mirror (for `lockDir()`)

```go
// internal/config/file.go:94-104 (EXISTING — the pattern to copy)
func globalConfigPath() string {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" && filepath.IsAbs(xdg) {
        return filepath.Join(xdg, "stagecoach", "config.toml")
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "config.toml" // last-resort fallback (CWD)
    }
    return filepath.Join(home, ".config", "stagecoach", "config.toml")
}
```

### `lockDir()` (NEW, in `internal/lock`) — mirrors but with different XDG dirs + loud fail
```go
func lockDir() (string, error) {
    if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" && filepath.IsAbs(xdg) {
        return filepath.Join(xdg, "stagecoach", "locks"), nil
    }
    if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
        return filepath.Join(xdg, "stagecoach", "locks"), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err // NO CWD fallback — a lock in the repo is the §18.5 anti-pattern
    }
    return filepath.Join(home, ".cache", "stagecoach", "locks"), nil
}
```

---

## 6. Platform split — mirror `signal_*.go` / `procgroup_*.go`

```
internal/lock/
├── lock.go           // Locker struct, Acquire/Release/SetSnapshot, lockDir, contents, hash
├── lock_unix.go      //go:build !windows  — unix.Flock(fd, LOCK_EX|LOCK_NB)
├── lock_windows.go   //go:build windows    — no-op stub (Acquire succeeds, Release/SetSnapshot no-op)
└── lock_test.go      // unit tests (dir resolution, hash, contents, contention via temp files)
```

The split follows the exact `//go:build` tag convention already used by
`internal/signal/signal_{unix,windows}.go` and
`internal/provider/procgroup_{unix,windows}.go`.

---

## 7. E2E harness — contention test pattern

The e2e harness (`internal/e2e/harness_test.go`) already has everything needed:

```go
// harness_test.go:48-57 — compiles the binary ONCE
build := exec.Command(goPath, "build", "-o", stagecoachBin,
    "github.com/dustin/stagecoach/cmd/stagecoach")

// harness_test.go:79-85 — creates a temp repo
func newRepo(t *testing.T) string { ... git init ... }

// harness_test.go:163-184 — invokes the subprocess
func runStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) e2eResult

// harness_test.go:216-228 — deterministic concurrent-race primitive
func waitForMarker(t *testing.T, path string, timeout time.Duration)
```

**Contention test pattern:**
1. `newRepo(t)` → seed a commit → stage a change.
2. Write a **stub agent config** whose stub binary blocks on a marker file (sleeps until
   the marker appears) — so subprocess #1 holds the lock during generation.
3. Launch stagecoach subprocess #1 (`runStagecoach` with the blocking stub) in a goroutine.
4. `waitForMarker` to confirm #1 has entered generation (holds the lock).
5. Launch stagecoach subprocess #2 against the **same** repo.
6. Assert: subprocess #2 exits `Busy` (code 5) and stderr contains the contention message.
7. Write the marker to release #1; assert #1 commits successfully.
8. Assert: no lock file contents remain stale (the file may persist but flock is released).

**No-op fast-path test:**
1. Same setup, but #2 is launched AFTER #1 has snapshotted (holder published `snapshot=`).
2. #2's index is identical to #1's snapshot (nothing new staged).
3. Assert: #2 exits 0 with "nothing to do" message.

---

## 8. Docs touch points

| Doc | Section | What to add | Mode |
|---|---|---|---|
| `docs/cli.md` | `## Exit codes` (line ~366) | `5` row: "Busy — another stagecoach run holds the per-repo lock" | A (rides with contention/wiring subtask) |
| `docs/how-it-works.md` | `## Safety and the rescue protocol` | New `### Per-repo run lock (FR52)` subsection: two-stage defense, per-host limit, never-in-repo location, no-op fast path | A (rides with lock-primitive subtask) |
| `docs/configuration.md` | (lock-file location) | Lock-file location resolution via `XDG_RUNTIME_DIR` / `XDG_CACHE_HOME` / `~/.cache` | A (rides with lock-primitive subtask) |
| `README.md` | Safety section | Race-free / safe-to-double-invoke property alongside snapshot/atomic-commit pitch | B (final task) |
