# System Context — FR52 Per-Repo Run Lock (Plan 006 Delta)

> **Status:** Researched & validated against the live codebase (2026-07-03).
> The entire v2.0/v3/v2.1 core is implemented and green. This delta adds exactly **one**
> new feature: FR52 — a per-repo advisory `flock`-based run lock that prevents two
> concurrent `stagecoach` commit-producing processes from racing on the same repository.

---

## 0. What exists (do not re-implement)

The full single-commit pipeline (§13.1–§13.5), multi-commit decomposition (§13.6),
per-role config (§9.15), provider manifests (§12), prompt engineering (§17), hook mode
(§9.20), tool integrations (§9.21), `--edit`/`--push` (§9.22), and discovery (§9.23) are
**all implemented and tested**. This delta composes over them — it touches the CLI entry
point and adds one new package; it does not modify the commit/rescue/CAS logic.

### Key existing seams (exact locations)

| Seam | File | Line | Signature / Detail |
|---|---|---|---|
| **Lock acquire point (single + decompose funnel)** | `internal/cmd/default_action.go` | 48–55 | `runDefault`: `repoDir, err := os.Getwd()` → `g := git.New(repoDir)`. Lock acquires immediately after line 55, before the auto-stage state machine (line 73+). `defer release()` covers every early-exit path. |
| **shouldDecompose** | `internal/cmd/default_action.go` | 297 | `func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool` — gates decompose routing. |
| **runDecompose** | `internal/cmd/default_action.go` | 308 | Called from `runDefault` at ~line 104. The lock in `runDefault` covers it (single funnel). |
| **WriteTree (single-path snapshot)** | `internal/generate/generate.go` | ~177 | `treeSHA, err := deps.Git.WriteTree(ctx)` → immediately followed by `signal.SetSnapshot(treeSHA, parentSHA, "")` at ~line 186. `lock.SetSnapshot(treeSHA)` goes here. |
| **FreezeWorkingTree (decompose T_start)** | `internal/decompose/decompose.go` | ~164 | `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)` → `lock.SetSnapshot(tStart)` goes here. |
| **UpdateRefCAS (the second defense)** | `internal/git/git.go` | 519 | `func (g *gitRunner) UpdateRefCAS(ctx, ref, newSHA, expectedOld string) error`. `ErrCASFailed` at line 511. **Unchanged** — the lock is defense-in-depth over this. |
| **Exit-code registry** | `internal/exitcode/exitcode.go` | 22–30 | Constants: `Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124`. `Busy` (=5) is added here. `For()` at line 52 maps errors → codes. |
| **XDG dir-resolution pattern** | `internal/config/file.go` | 94–104 | `globalConfigPath()`: `XDG_CONFIG_HOME` (if abs) → `~/.config/stagecoach/config.toml`. Mirror for `XDG_RUNTIME_DIR` / `XDG_CACHE_HOME` / `~/.cache`. **Difference:** lock dir has NO CWD last-resort (must fail loud — a lock in the repo is the anti-pattern §18.5 rejects). |
| **Signal handler (singleton)** | `internal/signal/signal.go` | 64–186 | `var active atomic.Pointer[Handler]`; `SetSnapshot()`/`ClearSnapshot()` are package-level singleton calls from generate/decompose. **The lock mirrors this pattern** for `lock.SetSnapshot()`. |
| **Platform split pattern** | `internal/signal/signal_unix.go` / `signal_windows.go` | 1–5 | `//go:build !windows` / `//go:build windows`. The lock package uses the same split: `lock_unix.go` (flock) / `lock_windows.go` (no-op stub). |
| **E2E harness** | `internal/e2e/harness_test.go` | 48–228 | `buildStagecoach` compiles the binary; `newRepo` creates a temp repo; `runStagecoach` invokes the subprocess; `waitForMarker` is a deterministic concurrent-race primitive. |

---

## 1. The lock design (PRD §18.5, authoritative)

### Location — per-system, per-user runtime dir, NEVER inside the repo

PRD §18.5 explicitly rejects `.git/stagecoach.lock` (pollutes `git status`, committable,
ambiguous across worktrees, lost on clone). The lock lives in:

1. `$XDG_RUNTIME_DIR/stagecoach/locks/<hash>.lock` (preferred — tmpfs, per-login, auto-cleaned)
2. Else `$XDG_CACHE_HOME/stagecoach/locks/<hash>.lock`
3. Else `~/.cache/stagecoach/locks/<hash>.lock`

`<hash>` = `sha256` hex of the repository's **canonical absolute path** (symlinks resolved
via `filepath.EvalSymlinks` so two terminals in the same repo via different paths hash
identically). Two repos → independent locks; two terminals in the same repo → contention.

### Mechanism — advisory `flock(2)`, auto-released on process death

`flock(fd, LOCK_EX | LOCK_NB)` on the `.lock` file descriptor. **Non-blocking** (`LOCK_NB`):
the user is interactive; blocking would hang their terminal. Released automatically when the
fd/process closes — including under `SIGKILL`, `os.Exit` (the signal handler's path), or a
crash. **No stale locks, no PID-liveness heuristics.** This is why the signal-handler
integration is trivial: `defer release()` is skipped on `os.Exit`, but `flock` releases
anyway because the fd closes.

This is a deliberate rejection of the `O_CREAT|O_EXCL` + PID-check pattern whose stale-lock
bugs are the classic failure mode for hand-rolled locks.

### Contents — diagnostic + snapshot fast-path

One `key=value` per line: `pid`, `hostname`, `repo`, start `timestamp`, and `snapshot=<sha>`.
`pid`/`hostname`/`repo` are diagnostic (naming the holder in the contention message).
`snapshot=` enables the no-op fast path — written after the holder freezes its snapshot.
None of it is used for stale-lock reaping (flock handles that).

### Contention behavior — no-op fast path or Busy refusal

When `LOCK_NB` fails (another stagecoach holds the lock):

1. **No-op fast path (exit 0):** If the holder published `snapshot=` AND the contender's own
   `git write-tree` (index-read-only, safe to take without the lock) produces a byte-identical
   tree SHA → nothing new has been staged since the holder began → the contender is a redundant
   accidental-double-run → exit 0: *"nothing to do — an in-progress run already covers your
   staged changes."*

2. **Busy refusal (exit `Busy=5`):** Otherwise read the holder's `pid`/`hostname`/`repo` and
   exit non-zero with: *"stagecoach: another stagecoach run is already in progress on `<repo>`
   (pid `<N>` on `<host>`). Your newly-staged changes will remain staged — re-run `stagecoach`
   after it finishes. Lock: `<path>`."* Never block; never force-break the lock.

### Scope — commit-producing actions only

`runDefault` acquires the lock (covering both single-commit and decompose modes). Read-only
subcommands (`config`, `providers`, `models`, `integrate list`/`status`, `hook status`,
`--version`, `--help`) have their own `RunE` and never reach `runDefault` — they bypass the
lock naturally. `--dry-run` goes through `runDefault` and calls `WriteTree` (a snapshot
operation), so it acquires the lock too.

---

## 2. The `snapshot=` wiring (how the holder publishes its frozen tree)

The lock is acquired in `runDefault` (CLI layer), but the snapshot is captured **inside** the
library (`generate.CommitStaged` / `decompose.Decompose`). To bridge this without changing
the public API, the lock package uses a **package-level singleton** — exactly the pattern
`internal/signal` already uses for `signal.SetSnapshot()`:

```go
// internal/lock — singleton (mirrors internal/signal/signal.go)
var current atomic.Pointer[Locker]

func Acquire(repoPath string) (*Locker, error) {
    // ... flock, write contents, set current ...
}
func (l *Locker) Release() {
    // ... close fd (auto-unlock), clear current ...
}
func SetSnapshot(sha string) {  // called from generate.go / decompose.go
    if l := current.Load(); l != nil {
        l.setSnapshot(sha)  // update the snapshot= field in the lock file
    }
    // no-op if no lock held (library usable without the lock, e.g. in tests)
}
```

Wiring points (one-line additions):
- **Single path:** `internal/generate/generate.go` ~line 186, next to
  `signal.SetSnapshot(treeSHA, parentSHA, "")`: add `lock.SetSnapshot(treeSHA)`.
- **Decompose path:** `internal/decompose/decompose.go` ~line 170, after
  `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)`: add `lock.SetSnapshot(tStart)`.

---

## 3. Import graph (no cycles)

```
internal/lock          → stdlib + golang.org/x/sys (LEAF — no stagecoach imports)
internal/generate      → internal/lock (NEW — alongside existing internal/signal)
internal/decompose     → internal/lock (NEW — alongside existing internal/signal)
internal/cmd           → internal/lock (NEW — acquire/release in runDefault)
```

`internal/lock` is a pure leaf. No cycle risk.

---

## 4. Windows portability

`golang.org/x/sys/unix.Flock` is POSIX-only. Windows has no POSIX `flock`. The lock package
uses the same build-tag split as `signal_*.go` and `procgroup_*.go`:

- `lock_unix.go` (`//go:build !windows`): real `unix.Flock(fd, LOCK_EX|LOCK_NB)`.
- `lock_windows.go` (`//go:build windows`): **no-op stub** — `Acquire` always succeeds
  (returns a dummy `*Locker`), `Release`/`SetSnapshot` are no-ops. The §13.5 CAS remains
  the guarantee on Windows. This matches the PRD §18.5 "per-host" limit: the lock is a
  local optimization; the CAS is the atomic guarantee that holds everywhere.

Rationale: Windows `LockFileEx` could provide real locking, but the PRD prescribes `flock`
as the mechanism and the CAS is the actual safety guarantee. A no-op stub on Windows is
correct (the CAS catches any race) and avoids a fragile platform-specific implementation
for a defense-in-depth layer.

---

## 5. Defense in depth (the two layers)

| Layer | What it prevents | Scope | Mechanism |
|---|---|---|---|
| **Run lock (FR52, NEW)** | Two `stagecoach` processes on the same repo/host racing on HEAD (the common local accidental double-run) | Per-host | `flock(LOCK_EX\|LOCK_NB)` — non-blocking, auto-released |
| **CAS update-ref (§13.5, EXISTING)** | Any concurrent HEAD movement (cross-host shared FS, another tool, a non-stagecoach process) | Universal | `git update-ref HEAD <new> <old>` — atomic, refuses to clobber |

Both stay. The lock makes the common local race impossible to stumble into; the CAS catches
everything else (including the shared-filesystem case the lock cannot cover).
