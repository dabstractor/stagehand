# S1 Implementation Notes — internal/lock package (FR52 primitive)

> Scope: P1.M1.T1.S1. A self-contained leaf package: advisory flock run lock, XDG dir resolution,
> sha256 repo-path hash, lock-contents parse, SetSnapshot singleton, unix/windows platform split.
> Verified against the live codebase + arch docs (system_context.md, integration_seams.md) 2026-07-03.

## 1. CRITICAL: the `golang.org/x/sys` mandate is questionable — recommend stdlib `syscall.Flock`

The contract (1a) says: "Add golang.org/x/sys to go.mod ... it is already a transitive dep of cobra/pflag
on Linux but not in go.mod directly." **This is factually wrong:**
- `grep golang.org/x/sys go.sum` → ZERO matches. It is NOT a transitive dep; `go get` would do a real
  network fetch + add a new module to go.mod/go.sum.
- The codebase has a **deliberate, documented stdlib-only convention** for syscalls. Four files explicitly
  avoid x/sys: `internal/provider/procgroup_windows.go:13`, `internal/signal/signal_windows.go:11`,
  `internal/ui/isatty_windows.go:10` — each comments *"stdlib-only — no golang.org/x/sys dependency"*.
  Unix-side syscalls use raw `syscall` (`procgroup_unix.go`: `syscall.SysProcAttr{Setpgid:true}`,
  `syscall.Kill`; `signal_unix.go`: `syscall.Kill`).

**`syscall.Flock` is in the Go stdlib on all Unix targets** (linux/amd64 + darwin/arm64 = the CI matrix,
system_context §4): `syscall.Flock(fd int, how int) error`, `syscall.LOCK_EX`, `syscall.LOCK_NB`,
`syscall.EWOULDBLOCK` are all exported. So flock needs NO new dependency.

**Recommendation: use stdlib `syscall.Flock` (Option B).** It matches the codebase convention, needs no
`go get`/network, and keeps go.mod/go.sum unchanged. Map `syscall.EWOULDBLOCK` → `*HeldError`.
The contract's x/sys path (Option A) is a one-line `go get golang.org/x/sys` + `unix.Flock` if the
mandate is binding — but it diverges from convention for no functional gain. The PRP presents both,
recommends B.

## 2. The 4 package files + exact build-tag convention (mirror signal_*.go / procgroup_*.go)

```
internal/lock/
├── lock.go          // Locker, Acquire/Release/SetSnapshot, lockDir, hash, LockContents, HeldError, parseContents, singleton
├── lock_unix.go     //go:build !windows  — flock(fd) via syscall.Flock(fd, LOCK_EX|LOCK_NB)
├── lock_windows.go  //go:build windows    — no-op stub: flock(fd) returns nil (CAS is the guarantee)
└── lock_test.go     // dir resolution, hash, contents, contention (two goroutines), Release idempotency, SetSnapshot
```
Build tags are EXACTLY `//go:build !windows` / `//go:build windows` (the convention signal_unix.go /
signal_windows.go / procgroup_*.go use). The platform-specific function is `flock(fd int) error`
(called from Acquire in lock.go; defined per-platform in lock_{unix,windows}.go).

## 3. The singleton (mirror internal/signal/signal.go)

`internal/signal` uses `var active atomic.Pointer[Handler]` with nil-safe package wrappers
(RegisterChild/SetSnapshot/…). The lock package mirrors this exactly:
```go
var current atomic.Pointer[Locker]
func Acquire(repoPath string) (*Locker, error) { ... l := &Locker{...}; current.Store(l); return l, nil }
func (l *Locker) Release() { if current.Load() == l { current.Store(nil) }; ... }  // idempotent
func SetSnapshot(sha string) { if l := current.Load(); l != nil { l.setSnapshot(sha) } }  // nil-safe no-op
```
This makes the package usable WITHOUT a held lock in tests/library use (SetSnapshot is a no-op then),
exactly like signal.SetSnapshot. The wiring CALL SITES (generate.go/decompose.go) are S2 — S1 ships
only the package + the singleton.

## 4. lockDir() (mirror config/globalConfigPath, but NO CWD fallback — fail loud)

From integration_seams.md §5 (verbatim target):
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
Acquire MUST `os.MkdirAll(dir, 0o700)` before OpenFile (the resolved dir may not exist). CRITICAL: the
`filepath.IsAbs(xdg)` guard (matches globalConfigPath) prevents a relative XDG var from producing a
relative lock path. NO CWD/repo fallback — a missing home + no XDG is a hard error (PRD §18.5 rejects
in-repo locks).

## 5. hash = sha256 hex of canonical absolute repo path

```go
canonical, err := filepath.EvalSymlinks(repoPath)
if err != nil { canonical, _ = filepath.Abs(repoPath) }  // fallback if EvalSymlinks fails (path vanishing)
sum := sha256.Sum256([]byte(canonical))
hash := hex.EncodeToString(sum[:])
```
EvalSymlinks is what makes two terminals in the same repo via different paths (e.g. one via a symlinked
dir) hash identically. Acquire opens `<dir>/<hash>.lock` with `O_CREATE|O_RDWR`.

## 6. LockContents + parseContents + the file format

```go
type LockContents struct { Pid, Hostname, Repo, Timestamp, Snapshot string }
type HeldError struct { Contents LockContents; Path string }
func (h *HeldError) Error() string { return "lock held by pid " + h.Contents.Pid + " on " + h.Contents.Hostname }
func parseContents(data []byte) LockContents {
    // bufio.Scanner line-by-line; strings.SplitN(line, "=", 2); key switch → field
}
```
File format (one key=value per line): `pid=…`, `hostname=…`, `repo=…`, `timestamp=…` (RFC3339 UTC),
`snapshot=…` ("" until SetSnapshot). Acquire WRITES these on success (pid=os.Getpid(),
hostname=os.Hostname(), repo=repoPath, timestamp=time.Now().UTC().Format(time.RFC3339), snapshot="").
On contention, the contender READS the holder's file and parseContents populates HeldError.Contents
for S2's message. (S2 decides no-op-vs-Busy; S1 just surfaces the contents.)

## 7. SetSnapshot — rewrite the file in place while holding the fd

```go
func (l *Locker) setSnapshot(sha string) {
    // re-derive contents: keep existing pid/hostname/repo/timestamp, set Snapshot=sha
    // l.file.Truncate(0); l.file.Seek(0,0); write key=value lines; sync
}
```
Must Truncate+Seek(0,0) (NOT append) so the snapshot= line replaces the empty one. Reads the prior
contents (parse the existing file) to preserve pid/hostname/repo/timestamp — OR the Locker caches them
(simpler: cache pid/hostname/repo/timestamp on the struct at Acquire; setSnapshot just rewrites from
cache + new sha). Caching avoids a re-read; recommend caching.

## 8. Tests (lock_test.go, package lock, in-process — temp dirs + real flock)

- `TestLockDir_*`: t.Setenv XDG_RUNTIME_DIR/XDG_CACHE_HOME/HOME → assert resolution order + IsAbs guard
  + NO CWD fallback (unset HOME + both XDG → expect error). Use a temp dir for HOME.
- `TestHash_Canonical`: same repo via a symlinked path → identical hash (EvalSymlinks).
- `TestAcquireRelease`: Acquire → file exists with contents → Release → idempotent (second Release no-op).
- `TestSetSnapshot_UpdatesFile`: Acquire (snapshot="") → SetSnapshot("abc123") → re-read file → snapshot=abc123.
- `TestAcquire_Contention_HeldError`: goroutine 1 Acquire(repo) holds; goroutine 2 Acquire(same repo)
  → returns *HeldError whose Contents.Pid == goroutine 1's pid (parseContents round-trip). After g1
  Release, g2 Acquire succeeds (flock released). Run with -race.
- Windows: lock_windows.go's no-op flock is never compiled on Linux CI (build tag) — its test surface
  is implicitly covered by the build-tag split (no Linux test needed for the stub).

## 9. Scope discipline (S1 vs S2)

S1 = the package (4 files) + go.mod (ONLY if Option A x/sys chosen; Option B = no go.mod change) +
2 docs (how-it-works.md `### Per-repo run lock (FR52)`, configuration.md lock-location subsection).
NOT S1 (S2 = P1.M1.T2): the Busy exit code (exitcode.go), the acquire/release wiring in runDefault,
the SetSnapshot CALL SITES in generate.go/decompose.go, the contention MESSAGE logic (no-op fast path
+ Busy refusal — needs git write-tree access), the cli.md Busy row, the E2E contention tests, README.
S1's HeldError surfaces the contents S2 needs; S1 does NOT decide no-op-vs-Busy.

## 10. Docs (Mode A — rides with S1)

- docs/how-it-works.md: under `## Safety and the rescue protocol` (line 142), add `### Per-repo run lock
  (FR52)` — two-stage defense (lock + CAS), per-host limit (shared/network FS = CAS's job), never-in-repo
  location rationale, no-op fast path (contender with nothing new staged → exit 0), flock auto-release.
- docs/configuration.md: add a subsection on lock-file location resolution (XDG_RUNTIME_DIR →
  XDG_CACHE_HOME → ~/.cache/stagecoach/locks/<hash>.lock) + the never-in-repo rationale.
