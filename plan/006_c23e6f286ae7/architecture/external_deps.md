# External Dependencies — FR52 Per-Repo Run Lock

> **New dependency:** `golang.org/x/sys` (for `unix.Flock`). This is the **only** new
> third-party dependency the feature introduces. It is already a transitive dependency of
> cobra/pflag on most platforms but is NOT in `go.mod` directly — must be `go get`-ed.

---

## 1. `golang.org/x/sys/unix.Flock` — the lock primitive

### API
```go
import "golang.org/x/sys/unix"

fd := int(f.Fd())  // f = *os.File opened on <hash>.lock
err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
// err == nil → lock acquired; err == EWOULDBLOCK → contention; else → I/O error
// Released automatically when fd closes (process exit, f.Close(), SIGKILL)
```

### Semantics
- `LOCK_EX` — exclusive lock (only one holder at a time).
- `LOCK_NB` — non-blocking. Returns `EWOULDBLOCK` immediately if the lock is held;
  never blocks. **Critical:** the user is interactive; blocking would hang their terminal.
- **Auto-release:** the lock is released when the file descriptor closes — at process exit
  (including `os.Exit` from the signal handler, or `SIGKILL`/crash), or on explicit
  `f.Close()`. This is the key property: **no stale locks, no PID-liveness heuristics**.
- The `.lock` file persists on disk after release (it's just an empty/tiny file); `flock`
  locks the *fd*, not the file's existence. This is fine — the file is in a runtime/cache dir.

### Contention detection
```go
err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
if err != nil {
    if errors.Is(err, unix.EWOULDBLOCK) {
        // another stagecoach holds the lock → contention path
    } else {
        // genuine I/O error → treat as acquisition failure (exit Error)
    }
}
```

### Platform availability
- **Linux, macOS, *BSD:** `unix.Flock` works directly. (`flock(2)` is a real syscall on
  Linux; on macOS/BSD it may be emulated via `fcntl` but `unix.Flock` handles the mapping.)
- **Windows:** `unix.Flock` does not exist (no POSIX flock). The `lock_windows.go` stub
  provides a no-op. See `system_context.md` §4.

---

## 2. Canonical repo-path hashing — `crypto/sha256` (stdlib)

```go
import (
    "crypto/sha256"
    "encoding/hex"
    "path/filepath"
)

canonical, err := filepath.EvalSymlinks(repoPath)
if err != nil {
    canonical = repoPath  // best-effort: use raw path if symlink resolution fails
}
h := sha256.Sum256([]byte(canonical))
hash := hex.EncodeToString(h[:])  // 64 hex chars → <hash>.lock
```

**Symlink canonicalization** (`filepath.EvalSymlinks`): two terminals in the same repo via
different paths (e.g., `/home/user/proj` vs `/tmp/symlink-to-proj`) must hash identically.
Resolving symlinks gives a canonical absolute path. If resolution fails, fall back to the
raw `repoPath` (the lock still works; it just might not deduplicate across symlinks).

---

## 3. Lock-file contents — `encoding` (stdlib, key=value lines)

Simple `key=value` per line (not TOML — a lock file is tiny and needs no schema):

```go
pid=12345
hostname=workstation
repo=/home/dustin/projects/stagecoach
timestamp=2026-07-03T22:06:00Z
snapshot=9f3a1c...
```

Written by the holder at acquire time (snapshot empty), updated via `SetSnapshot(sha)` after
the freeze. Read by the contender for the fast-path comparison and the contention message.
Parsed with `bufio.Scanner` + `strings.SplitN(line, "=", 2)`.

---

## 4. No other new dependencies

The feature uses only:
- `golang.org/x/sys/unix` (flock) — NEW, direct dep
- `os`, `os/exec` (file I/O, hostname) — stdlib
- `crypto/sha256`, `encoding/hex` (path hashing) — stdlib
- `path/filepath` (symlink resolution, path joining) — stdlib
- `bufio`, `strings` (contents parsing) — stdlib
- `sync/atomic` (singleton pointer) — stdlib
- `time` (timestamp) — stdlib

The existing `internal/git.Git` interface (already in scope from `runDefault`) provides
`WriteTree(ctx) (sha, error)` for the contender's no-op-fast-path comparison — no new git
method is needed.
