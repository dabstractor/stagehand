---
name: "P1.M1.T1.S1 — Core lock package: Locker struct, flock acquire/release, dir resolver, hash, contents, SetSnapshot singleton, platform split"
description: |
  FR52 primitive (self-contained leaf). Create `internal/lock/`: `lock.go` (Locker, Acquire/Release/
  SetSnapshot, lockDir, sha256 hash, LockContents, HeldError, parseContents, atomic.Pointer singleton),
  `lock_unix.go` (`//go:build !windows` — flock via LOCK_EX|LOCK_NB), `lock_windows.go`
  (`//go:build windows` — no-op stub; CAS is the guarantee), and `lock_test.go`. The lock is an advisory
  flock auto-released on process death; located via XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagecoach
  /locks/<sha256-of-canonical-repo-path>.lock (NEVER in the repo). Acquire returns *HeldError (holder's
  parsed contents) on EWOULDBLOCK. SetSnapshot is a nil-safe package singleton (mirrors internal/signal).
  DEPENDENCY DECISION: prefer stdlib `syscall.Flock` (no go.mod change; matches the codebase's stdlib-only
  convention; the contract's x/sys-is-transitive claim is false) — see Implementation Blueprint §0. Mode A
  docs: how-it-works.md `### Per-repo run lock (FR52)` + configuration.md lock-location subsection. S2
  owns wiring/contention-message/Busy-exit-code/E2E — NOT this subtask.
---

## Goal

**Feature Goal**: Ship a complete, self-contained `internal/lock` package — the FR52 per-repo run-lock
primitive — that `internal/cmd`, `internal/generate`, and `internal/decompose` can import (the wiring
itself is S2). The lock is an advisory `flock(LOCK_EX|LOCK_NB)` auto-released on process death (no stale
locks), located in a per-system/per-user runtime dir keyed by a sha256 hash of the repo's canonical
absolute path (NEVER inside the repo), carrying diagnostic contents (pid/hostname/repo/timestamp) plus a
`snapshot=` field published via a nil-safe package singleton.

**Deliverable** (one new package + 2 docs; optionally a go.mod edit):
1. `internal/lock/lock.go` — `Locker` struct, `Acquire(repoPath) (*Locker,error)`, `(*Locker).Release()`
   (idempotent), `(*Locker).SetSnapshot(sha)` + package `SetSnapshot(sha)` singleton, `lockDir()`,
   sha256 hash, `LockContents`, `HeldError`, `parseContents`, `var current atomic.Pointer[Locker]`.
2. `internal/lock/lock_unix.go` (`//go:build !windows`) — `flock(fd int) error` via stdlib `syscall.Flock`
   (recommended) mapping `syscall.EWOULDBLOCK` → a sentinel the caller turns into `*HeldError`.
3. `internal/lock/lock_windows.go` (`//go:build windows`) — no-op `flock(fd int) error` (returns nil;
   §13.5 CAS is the guarantee on Windows).
4. `internal/lock/lock_test.go` — dir resolution, hash determinism (symlinks), contents round-trip,
   flock contention → `*HeldError`, Release idempotency, SetSnapshot updates the file.
5. `docs/how-it-works.md` — `### Per-repo run lock (FR52)` subsection under `## Safety and the rescue protocol`.
6. `docs/configuration.md` — lock-file location-resolution subsection.
7. *(Only if Option A is chosen)* `go.mod`/`go.sum` — `go get golang.org/x/sys`. **Recommended Option B
   uses stdlib `syscall.Flock` and changes neither go.mod nor go.sum.**

**Success Definition**: `go build ./...` and `go test -race ./internal/lock/` are green; the package
imports only stdlib (Option B) or stdlib + golang.org/x/sys (Option A); `Acquire` returns a working
`*Locker` on an uncontended dir, returns `*HeldError` (with the holder's parsed contents) when a second
goroutine contends on the same repo hash, `Release` is idempotent and clears the singleton, `SetSnapshot`
rewrites the file's `snapshot=` line, and the package singleton's `SetSnapshot` is a nil-safe no-op when
no lock is held. No other package is modified (S2 owns the wiring).

## User Persona

**Target User**: The contributor implementing S2 (contention behavior + wiring into `runDefault`/
`generate.go`/`decompose.go`) and the eventual end user protected from the accidental-double-run race.

**Use Case**: S2's `runDefault` will call `lock.Acquire(repoDir)`; `generate.CommitStaged` /
`decompose.Decompose` will call `lock.SetSnapshot(treeSHA)` to publish the frozen snapshot for the no-op
fast path. S1 ships exactly the API those call sites need.

**Pain Points Addressed**: Provides the `*HeldError.Contents` (pid/hostname/repo/snapshot) S2 needs to
build the contention message; the nil-safe singleton lets the library layers call `SetSnapshot`
unconditionally (no-op in tests/library use); the platform split keeps Windows compiling without a real
flock.

## Why

- **FR52 / PRD §18.5 defense-in-depth.** Two stagecoach processes on one repo race on HEAD (the loser's
  §13.5 CAS aborts → dangling snapshot + "already committed" confusion). The lock makes the common local
  double-run impossible to stumble into; the CAS catches everything else (incl. shared/network FS). S1 is
  the lock primitive; S2 wires it.
- **Self-contained leaf, no cycles.** `internal/lock` imports only stdlib (+ optionally x/sys) — no
  stagecoach deps (system_context.md §3). It mirrors the proven `internal/signal` singleton pattern.
- **Auto-release on death = no stale-lock bugs.** `flock` releases when the fd/process closes (incl.
  SIGKILL/crash) — deliberately avoiding the fragile `O_CREAT|O_EXCL`+PID-check pattern (PRD §18.5).
- **stdlib syscall convention.** The codebase deliberately avoids `golang.org/x/sys` (4 files document
  "no x/sys dependency"). `syscall.Flock` is in the stdlib on linux/darwin → Option B needs no new dep.

## What

A new `internal/lock` package (4 files) implementing advisory flock locking keyed by a sha256 hash of the
repo's canonical path, with XDG runtime/cache dir resolution (never in-repo), diagnostic + snapshot
contents, a nil-safe `SetSnapshot` singleton, and a unix/windows build-tag split. Plus two Mode-A doc
subsections. No wiring, no Busy exit code, no contention-message logic, no E2E (all S2).

### Success Criteria

- [ ] `internal/lock/lock.go` exports `Locker`, `Acquire`, `(*Locker).Release`, `(*Locker).SetSnapshot`,
      `SetSnapshot` (package), `LockContents`, `HeldError`, and `parseContents`; has `var current atomic.Pointer[Locker]`.
- [ ] `lockDir()` resolves `XDG_RUNTIME_DIR` → `XDG_CACHE_HOME` → `~/.cache/stagecoach/locks` (each only
      if the env var is absolute); returns an error (NO CWD/repo fallback) when none resolve.
- [ ] `Acquire(repoPath)` resolves the dir (`MkdirAll 0o700`), hashes the canonical path
      (`filepath.EvalSymlinks` → sha256 hex), opens `<hash>.lock` (`O_CREATE|O_RDWR`), calls `flock`;
      on success writes `pid/hostname/repo/timestamp/snapshot=""` and stores the singleton; on
      `EWOULDBLOCK` reads the holder's file and returns `*HeldError{Contents, Path}`.
- [ ] `(*Locker).Release()` closes the fd (flock auto-releases), clears the singleton if it points at `l`,
      and is idempotent (a second call is a no-op).
- [ ] `SetSnapshot(sha)` (package + method) rewrites the file's `snapshot=` line (Truncate+Seek(0,0)),
      preserving pid/hostname/repo/timestamp; the package func is a nil-safe no-op when no lock is held.
- [ ] `lock_unix.go` (`//go:build !windows`) implements `flock(fd)` via stdlib `syscall.Flock(fd, LOCK_EX|LOCK_NB)`.
- [ ] `lock_windows.go` (`//go:build windows`) implements `flock(fd)` as a no-op (returns nil) with a
      comment that the §13.5 CAS is the guarantee on Windows.
- [ ] `lock_test.go` passes under `-race`: dir resolution (incl. IsAbs guard + no-CWD-fallback error),
      hash determinism via a symlinked path, contents round-trip, contention → `*HeldError` (two
      goroutines, same repo), Release idempotency, SetSnapshot updates the file.
- [ ] `go build ./...` + `go vet ./...` + `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `docs/how-it-works.md` has `### Per-repo run lock (FR52)` under `## Safety and the rescue protocol`.
- [ ] `docs/configuration.md` has the lock-file location-resolution subsection.
- [ ] NO edits to `internal/cmd`, `internal/generate`, `internal/decompose`, `internal/exitcode`, or any
      other package (S2); NO `docs/cli.md` Busy row (S2); NO README (S3).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim `lockDir()` target, the singleton pattern (mirroring
`internal/signal`), the exact build-tag convention, the file-format/contents, the dependency decision
(with evidence), and the test matrix. The architecture docs (`system_context.md` §1–§5,
`integration_seams.md` §5–§6) pre-resolved the design, the XDG pattern to mirror, the platform split, and
the S1/S2 boundary.

### Documentation & References

```yaml
# MUST READ — the authoritative lock design + exact seams
- docfile: plan/006_c23e6f286ae7/architecture/system_context.md
  why: "§1 is the lock design (location, mechanism, contents, contention, scope); §2 is the SetSnapshot singleton pattern (mirrors internal/signal); §3 import graph (leaf, no cycles); §4 Windows portability (no-op stub rationale)."
  critical: "§1 mandates: advisory flock(LOCK_EX|LOCK_NB) auto-released on death; location NEVER in the repo; sha256 of canonical path (EvalSymlinks). §2: the singleton is the bridge so the library layers can publish snapshot= without changing the public API."

- docfile: plan/006_c23e6f286ae7/architecture/integration_seams.md
  why: "§5 gives the VERBATIM lockDir() target (mirror config/globalConfigPath, but NO CWD fallback). §6 gives the file layout + build-tag convention (mirror signal_*.go/procgroup_*.go). §7 is the contention-test pattern (S2's E2E, but informs S1's in-process HeldError test). §8 lists the doc touch points (how-it-works + configuration = S1; cli.md Busy + README = S2/S3)."
  critical: "§5 lockDir() is copy-paste-ready. §6 confirms the //go:build tag strings and that lock_test.go co-locates with the package."

# Patterns to mirror (read-only — do NOT edit)
- file: internal/signal/signal.go
  why: "The singleton pattern to mirror: `var active atomic.Pointer[Handler]` + nil-safe package wrappers (SetSnapshot is a no-op when no handler installed). lock.go's `var current atomic.Pointer[Locker]` + package SetSnapshot mirror this exactly."
  pattern: "Install/Release manage the singleton; package-level wrappers Load() and no-op on nil. lock.Acquire stores current; Release clears it if it == l; package SetSnapshot Load()s and no-ops on nil."

- file: internal/signal/signal_unix.go
  why: "The build-tag + platform-split convention: `//go:build !windows`, package decl, stdlib `syscall` import. lock_unix.go mirrors this (and uses stdlib syscall.Flock — NOT x/sys)."
  pattern: "//go:build !windows  →  package lock  →  import \"syscall\"  →  func flock(fd int) error { err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); ... map EWOULDBLOCK ... }"

- file: internal/signal/signal_windows.go
  why: "The Windows build-tag convention: `//go:build windows`. lock_windows.go mirrors it as a no-op stub."
  pattern: "//go:build windows  →  package lock  →  func flock(fd int) error { return nil // CAS is the guarantee on Windows (PRD §18.5 per-host limit) }"

- file: internal/config/file.go
  why: "globalConfigPath() (lines ~94-104) is the XDG-resolution pattern lockDir() mirrors — `if xdg := os.Getenv(...); xdg != \"\" && filepath.IsAbs(xdg)`. The ONLY difference: lockDir returns (string, error) and has NO CWD last-resort (fail loud)."

- file: internal/signal/signal_test.go
  why: "The in-package test convention: `package signal`, t.Cleanup to reset the singleton (prevent -race test poisoning), injectable seams. lock_test.go uses `package lock`, t.Cleanup to reset `current`, t.TempDir()+t.Setenv for dir resolution."

- docfile: plan/006_c23e6f286ae7/P1M1T1S1/research/s1_lock_package.md
  why: "Distilled S1 findings: the stdlib-vs-x/sys decision (with go.sum evidence), the 4-file layout, the singleton, the verbatim lockDir, hash/contents/SetSnapshot details, the test matrix, and the S1/S2 scope boundary."

# Docs to edit (Mode A)
- file: docs/how-it-works.md
  why: "EDIT. Under `## Safety and the rescue protocol` (line 142), add `### Per-repo run lock (FR52)`: two-stage defense (lock + CAS), per-host limit, never-in-repo location, no-op fast path, flock auto-release."
- file: docs/configuration.md
  why: "EDIT. Add a lock-file location-resolution subsection (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagecoach/locks/<hash>.lock) + the never-in-repo rationale."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── go.mod                       # EDIT only if Option A (x/sys); UNCHANGED for Option B (stdlib)
├── internal/
│   ├── lock/                    # NEW package (4 files)
│   │   ├── lock.go              # NEW
│   │   ├── lock_unix.go         # NEW (//go:build !windows)
│   │   ├── lock_windows.go      # NEW (//go:build windows)
│   │   └── lock_test.go         # NEW
│   ├── signal/                  # read-only ref — singleton + platform-split pattern to mirror
│   ├── config/file.go           # read-only ref — XDG globalConfigPath pattern to mirror
│   └── provider/procgroup_*.go  # read-only ref — //go:build tag convention
└── docs/
    ├── how-it-works.md          # EDIT (Mode A)
    └── configuration.md         # EDIT (Mode A)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
├── internal/lock/               # NEW (self-contained leaf; stdlib-only under Option B)
│   ├── lock.go
│   ├── lock_unix.go
│   ├── lock_windows.go
│   └── lock_test.go
├── docs/how-it-works.md         # + ### Per-repo run lock (FR52)
└── docs/configuration.md        # + lock-file location subsection
# (go.mod/go.sum unchanged under Option B; + golang.org/x/sys under Option A)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/lock/lock.go` | CREATE | Locker, Acquire/Release/SetSnapshot, lockDir, hash, contents, HeldError, singleton. |
| `internal/lock/lock_unix.go` | CREATE | `//go:build !windows` flock via stdlib syscall.Flock. |
| `internal/lock/lock_windows.go` | CREATE | `//go:build windows` no-op stub. |
| `internal/lock/lock_test.go` | CREATE | dir/hash/contents/contention/release/SetSnapshot tests. |
| `docs/how-it-works.md` | MODIFY | `### Per-repo run lock (FR52)` subsection. |
| `docs/configuration.md` | MODIFY | lock-file location subsection. |
| `go.mod` / `go.sum` | *(Option A only)* | `go get golang.org/x/sys`. **Unchanged under Option B.** |

**Explicitly NOT touched in S1**: `internal/cmd/*` (runDefault wiring = S2), `internal/generate/*` +
`internal/decompose/*` (SetSnapshot call sites = S2), `internal/exitcode/*` (Busy=5 = S2), `docs/cli.md`
(Busy row = S2), `README.md` (S3), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (dependency choice): the contract says "Add golang.org/x/sys ... it is already a transitive
// dep of cobra/pflag". THIS IS FALSE — `grep golang.org/x/sys go.sum` is empty. The codebase has a
// deliberate stdlib-only-syscall convention (procgroup_windows.go:13, signal_windows.go:11,
// isatty_windows.go:10 all comment "no golang.org/x/sys dependency"). syscall.Flock + LOCK_EX + LOCK_NB
// + EWOULDBLOCK ARE in the Go stdlib on linux/darwin. RECOMMEND Option B (stdlib syscall.Flock) — no
// go.mod change, matches convention. Option A (x/sys) is a real `go get` + new dep for no functional gain.

// CRITICAL (no CWD/repo fallback): lockDir() MUST return an error when XDG_RUNTIME_DIR + XDG_CACHE_HOME
// are unset/relative AND os.UserHomeDir() fails. Do NOT fall back to CWD or the repo — a lock in the
// repo is the §18.5 anti-pattern (pollutes git status, committable, ambiguous across worktrees, lost on
// clone). config/globalConfigPath DOES fall back to CWD; lockDir deliberately does NOT.

// CRITICAL (IsAbs guard): only honor an XDG var when it is BOTH non-empty AND filepath.IsAbs — a relative
// XDG_RUNTIME_DIR must not produce a relative lock path. Mirrors globalConfigPath.

// CRITICAL (canonical hash): use filepath.EvalSymlinks(repoPath) BEFORE hashing so two terminals in the
// same repo via different paths (one a symlink) hash identically. If EvalSymlinks errors (path vanished),
// fall back to filepath.Abs(repoPath). Hash the resulting canonical string with sha256 → hex.

// CRITICAL (singleton + -race): the package singleton `var current atomic.Pointer[Locker]` MUST be reset
// in tests (t.Cleanup → current.Store(nil)) so tests don't poison each other under -race. Mirror
// signal_test.go's installTestHandler Cleanup.

// GOTCHA (SetSnapshot rewrite): rewrite the file with Truncate(0)+Seek(0,0) (NOT append), else the
// snapshot= line duplicates. Cache pid/hostname/repo/timestamp on the Locker at Acquire so setSnapshot
// rewrites from cache + the new sha without re-reading/parsing the file.

// GOTCHA (MkdirAll): the resolved lock dir may not exist — Acquire MUST os.MkdirAll(dir, 0o700) before
// os.OpenFile. 0o700 (owner-only) since the file carries pid/hostname/repo diagnostics.

// GOTCHA (flock is per-fd, per-process): the lock is held while the fd is open. Release closes the fd →
// flock auto-releases (also on crash/SIGKILL). Do NOT keep a second fd open. Two goroutines in the SAME
// process contending on the same fd won't contend (flock is per-open-file-description) — the contention
// test MUST use two SEPARATE OpenFile calls on the same path (two goroutines each call Acquire → each
// opens its own fd → the second's flock(LOCK_EX|LOCK_NB) fails with EWOULDBLOCK).

// GOTCHA (Windows stub never compiled on Linux CI): lock_windows.go's no-op flock is excluded by the
// //go:build windows tag on Linux — no Linux test covers it, and that's correct (it's a defense-in-depth
// no-op; the CAS is the real guarantee). Do NOT try to test it on Linux.
```

## Implementation Blueprint

### §0. Dependency decision — Option B (stdlib) RECOMMENDED over Option A (x/sys)

```go
// Option B (RECOMMENDED — no go.mod change, matches the codebase's stdlib-only convention):
// internal/lock/lock_unix.go
//go:build !windows
package lock
import "syscall"
func flock(fd int) error {
    err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
    if err != nil {
        return err // caller maps syscall.EWOULDBLOCK → *HeldError
    }
    return nil
}
// Acquire maps the error: if errors.Is(err, syscall.EWOULDBLOCK) → read holder file → return *HeldError.

// Option A (contract-literal — only if the x/sys mandate is binding):
//   go get golang.org/x/sys  →  import "golang.org/x/sys/unix"
//   unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); map unix.EWOULDBLOCK → *HeldError.
// Option A adds a new module to go.mod/go.sum that the project has deliberately avoided. Prefer B.
```

### Data models and structure

```go
// internal/lock/lock.go (Option B: stdlib-only)
package lock

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Locker is an acquired per-repo run lock (PRD §18.5 / FR52). The fd holds an advisory flock
// (LOCK_EX|LOCK_NB) that auto-releases when the fd/process closes — no stale locks. Create via
// Acquire; release via Release (idempotent) or process exit.
type Locker struct {
	file      *os.File
	path      string
	pid       string
	hostname  string
	repo      string
	timestamp string
}

// LockContents is the parsed key=value contents of a lock file (diagnostic + snapshot fast-path).
type LockContents struct {
	Pid, Hostname, Repo, Timestamp, Snapshot string
}

// HeldError is returned by Acquire when another stagecoach process holds the lock (LOCK_NB failed).
// Contents is the holder's parsed lock file (for the contention message); Path is the lock file path.
type HeldError struct {
	Contents LockContents
	Path     string
}
func (e *HeldError) Error() string {
	return fmt.Sprintf("stagecoach run lock held by pid %s on %s", e.Contents.Pid, e.Contents.Hostname)
}

// current is the process-global singleton (mirrors internal/signal.active). nil when no lock is held
// (library/tests); SetSnapshot is then a nil-safe no-op.
var current atomic.Pointer[Locker]
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/lock/lock.go — the core (stdlib-only under Option B)
  - PACKAGE: package lock. IMPORTS: bufio, crypto/sha256, encoding/hex, fmt, os, path/filepath, strings,
    sync/atomic, time. (Under Option A add golang.org/x/sys/unix to lock_unix.go only.)
  - TYPES: Locker (file *os.File; path string; pid/hostname/repo/timestamp string), LockContents (5 strings),
    HeldError{Contents LockContents; Path string} + Error() method.
  - var current atomic.Pointer[Locker]  (the singleton).
  - func Acquire(repoPath string) (*Locker, error):
      1. dir, err := lockDir(); if err != nil → return nil, err.
      2. if err := os.MkdirAll(dir, 0o700); err != nil → return nil, fmt.Errorf("lock dir: %w", err).
      3. canonical, err := filepath.EvalSymlinks(repoPath); if err != nil → canonical, _ = filepath.Abs(repoPath).
      4. sum := sha256.Sum256([]byte(canonical)); hash := hex.EncodeToString(sum[:]).
      5. path := filepath.Join(dir, hash+".lock").
      6. f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600); if err != nil → return nil, err.
      7. if err := flock(int(f.Fd())); err != nil:
           - if isWouldBlock(err) (Option B: errors.Is(err, syscall.EWOULDBLOCK); Option A: errors.Is(err, unix.EWOULDBLOCK)):
               data, _ := os.ReadFile(path); f.Close(); return nil, &HeldError{Contents: parseContents(data), Path: path}
           - f.Close(); return nil, err.
      8. build contents: pid=os.Getpid(); host,_:=os.Hostname(); ts=time.Now().UTC().Format(time.RFC3339).
      9. l := &Locker{file:f, path:path, pid:pid, hostname:host, repo:repoPath, timestamp:ts}.
     10. l.writeContents("")  // snapshot="" initially.
     11. current.Store(l); return l, nil.
  - func (l *Locker) Release():
      if l == nil || l.file == nil { return }  // idempotent
      l.file.Close(); l.file = nil
      if current.Load() == l { current.Store(nil) }
  - func (l *Locker) SetSnapshot(sha string) { l.setSnapshot(sha) }   // method form
  - func (l *Locker) setSnapshot(sha string):
      if l.file == nil { return }  // already released
      l.writeContents(sha)   // Truncate+Seek(0,0)+rewrite from cached pid/host/repo/ts + new sha
  - func SetSnapshot(sha string): if l := current.Load(); l != nil { l.setSnapshot(sha) }  // nil-safe no-op
  - func lockDir() (string, error):  // VERBATIM from integration_seams.md §5 (see Implementation Patterns)
  - func (l *Locker) writeContents(snapshot string):
      l.file.Truncate(0); l.file.Seek(0, 0); write "pid=…\nhostname=…\nrepo=…\ntimestamp=…\nsnapshot=…\n"; l.file.Sync().
  - func parseContents(data []byte) LockContents:
      bufio.Scanner; for each line: kv := strings.SplitN(line, "=", 2); switch kv[0] → field. Return LockContents.

Task 2: CREATE internal/lock/lock_unix.go (//go:build !windows) — the real flock
  - BUILD TAG: `//go:build !windows` (first line, blank line, package decl — mirror signal_unix.go).
  - Option B (RECOMMENDED): import "syscall"; "errors".
        func flock(fd int) error { return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) }
        func isWouldBlock(err error) bool { return errors.Is(err, syscall.EWOULDBLOCK) }
  - Option A (contract-literal): import "golang.org/x/sys/unix"; "errors".
        func flock(fd int) error { return unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB) }
        func isWouldBlock(err error) bool { return errors.Is(err, unix.EWOULDBLOCK) }
  - NOTE: lock.go calls flock(int(f.Fd())) and isWouldBlock(err) — both defined here per-platform.

Task 3: CREATE internal/lock/lock_windows.go (//go:build windows) — the no-op stub
  - BUILD TAG: `//go:build windows` (mirror signal_windows.go).
  - func flock(fd int) error { return nil }   // CAS (§13.5) is the guarantee on Windows (PRD §18.5 per-host)
  - func isWouldBlock(err error) bool { return false }
  - COMMENT: document that Windows has no POSIX flock; LockFileEx could provide real locking but the PRD
    prescribes flock + the CAS is the actual safety guarantee; a no-op is correct and avoids a fragile
    platform-specific impl for a defense-in-depth layer.

Task 4: CREATE internal/lock/lock_test.go (package lock; in-process; -race clean)
  - import os, path/filepath, strings, sync, testing.
  - helper: reset current in t.Cleanup (current.Store(nil)) — prevent singleton poisoning under -race.
  - TestLockDir_RuntimePreferred: t.Setenv("XDG_RUNTIME_DIR", tmpAbs); unset XDG_CACHE_HOME; → resolved dir
    == tmpAbs/stagecoach/locks.
  - TestLockDir_CacheFallback: unset XDG_RUNTIME_DIR; t.Setenv("XDG_CACHE_HOME", tmpAbs) → tmpAbs/stagecoach/locks.
  - TestLockDir_HomeFallback: unset both XDG; t.Setenv("HOME", tmpHome) → tmpHome/.cache/stagecoach/locks.
  - TestLockDir_RejectedRelative: t.Setenv("XDG_RUNTIME_DIR", "rel/path") (not abs) → skipped; falls through.
  - TestLockDir_NoCwdFallbackError: unset both XDG + HOME (os.UserHomeDir fails) → lockDir returns error.
        (Use a HOME the resolver can't use, e.g. t.Setenv("HOME","") on a system where that breaks UserHomeDir,
         OR test the err path by injecting a bad state — at minimum assert the function CAN return non-nil err.)
  - TestHash_CanonicalSymlink: create tmpRepo; symlink tmpLink→tmpRepo; Acquire uses EvalSymlinks →
    both produce the same <hash>.lock filename (assert the hash function on both paths matches).
  - TestAcquireRelease_RoundTrip: Acquire(repo) → file exists; parseContents(file) == written pid/host/repo;
    snapshot="". Release() → second Release() no-op (no panic).
  - TestSetSnapshot_UpdatesFile: Acquire; SetSnapshot("abc123"); re-read file → snapshot=abc123, pid/host/repo preserved.
  - TestSetSnapshot_NilSafeNoOp: current.Store(nil); SetSnapshot("x") → no panic, no-op (no lock held).
  - TestAcquire_Contention_HeldError:
        repo := t.TempDir(); l1, err := Acquire(repo); (success)
        var l2 *Locker; var l2err error
        wg := sync.WaitGroup{}; wg.Add(1)
        go func(){ defer wg.Done(); l2, l2err = Acquire(repo) }()
        wg.Wait()
        assert l2 == nil && l2err != nil; var he *HeldError; errors.As(l2err, &he) → true
        assert he.Contents.Pid == l1.pid (parseContents round-tripped the holder's contents)
        l1.Release()
        l3, err := Acquire(repo) → success (flock released after l1.Release)   // proves auto-release-on-close
    Run the whole suite with -race.

Task 5: VALIDATE (the package must build + test green in isolation, then the whole repo)
  - RUN: go build ./internal/lock/
  - RUN: go test -race ./internal/lock/
  - RUN: go build ./...   (proves the new leaf imports cleanly; no other package changed)
  - RUN: go vet ./... ; gofmt -l internal/lock/
  - RUN: go test -race ./...   (full suite green — S1 adds a leaf, changes nothing else)
  - FIX-FORWARD: read failures, fix, re-run.

Task 6: DOCS (Mode A)
  - docs/how-it-works.md: under `## Safety and the rescue protocol` (line 142), add `### Per-repo run lock (FR52)`:
    two-stage defense (run lock = first line, prevents the common local double-run; §13.5 CAS = second,
    never-clobber-HEAD, holds even on shared/network FS); per-host limit (the lock is local; cross-host
    shared FS is the CAS's job); never-in-repo location (XDG runtime/cache; in-repo pollutes git status /
    committable / ambiguous across worktrees / lost on clone); no-op fast path (a contender with nothing
    new staged since the holder's published snapshot= exits 0); flock auto-release (no stale locks on crash/SIGKILL).
  - docs/configuration.md: add a subsection: lock-file location = $XDG_RUNTIME_DIR/stagecoach/locks/<hash>.lock
    → else $XDG_CACHE_HOME/... → else ~/.cache/stagecoach/locks/<hash>.lock; <hash> = sha256 of the repo's
    canonical absolute path; never inside the repo (§18.5).
  - DO NOT edit docs/cli.md (Busy row = S2) or README (S3).
```

### Implementation Patterns & Key Details

```go
// === lockDir() — VERBATIM target (integration_seams.md §5); NO CWD fallback ===
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

// === Acquire — the contention branch (EWOULDBLOCK → *HeldError with parsed holder contents) ===
	if err := flock(int(f.Fd())); err != nil {
		if isWouldBlock(err) {
			data, _ := os.ReadFile(path)
			f.Close()
			return nil, &HeldError{Contents: parseContents(data), Path: path}
		}
		f.Close()
		return nil, err
	}

// === setSnapshot — rewrite in place (Truncate+Seek), preserve cached diagnostics ===
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil { return }
	l.file.Truncate(0); l.file.Seek(0, 0)
	l.writeContents(sha)   // uses l.pid/l.hostname/l.repo/l.timestamp + sha
}
func (l *Locker) writeContents(snapshot string) {
	fmt.Fprintf(l.file, "pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Sync()
}

// === parseContents — bufio.Scanner + SplitN(line,"=",2) ===
func parseContents(data []byte) LockContents {
	var c LockContents
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		kv := strings.SplitN(sc.Text(), "=", 2)
		if len(kv) != 2 { continue }
		switch kv[0] {
		case "pid":       c.Pid = kv[1]
		case "hostname":  c.Hostname = kv[1]
		case "repo":      c.Repo = kv[1]
		case "timestamp": c.Timestamp = kv[1]
		case "snapshot":  c.Snapshot = kv[1]
		}
	}
	return c
}
```

### Integration Points

```yaml
NEW PACKAGE (internal/lock — self-contained leaf):
  - imports: stdlib only (Option B); stdlib + golang.org/x/sys (Option A)
  - NO stagecoach imports (leaf — no cycle risk, system_context §3)
  - exports: Locker, Acquire, (*Locker).Release, (*Locker).SetSnapshot, SetSnapshot (package),
             LockContents, HeldError, parseContents

PLATFORM SPLIT (mirror signal_*.go / procgroup_*.go):
  - lock_unix.go    //go:build !windows  → flock via syscall.Flock (Option B) / unix.Flock (Option A)
  - lock_windows.go //go:build windows    → no-op stub (CAS is the guarantee)

SINGLETON (mirror internal/signal.active):
  - var current atomic.Pointer[Locker]
  - Acquire stores; Release clears-if-matches; package SetSnapshot nil-safe no-op

NO-TOUCH (S2 = P1.M1.T2 / S3):
  - internal/cmd/default_action.go       # S2: acquire/release funnel + contention message
  - internal/exitcode/exitcode.go        # S2: Busy = 5
  - internal/generate/generate.go        # S2: lock.SetSnapshot(treeSHA) at ~line 186
  - internal/decompose/decompose.go      # S2: lock.SetSnapshot(tStart) at ~line 170
  - docs/cli.md                          # S2: Busy exit-code row
  - README.md                            # S3: race-free safety pitch
  - internal/e2e/*                       # S2: contention E2E scenarios

DOWNSTREAM HOOKS (informational — S2 owns):
  - S2 wires Acquire/Release in runDefault (default_action.go:55), HandleAcquireError (no-op-vs-Busy),
    SetSnapshot calls in generate.go + decompose.go, the Busy exit code, and the E2E contention tests.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/lock/            # Expected: empty (run gofmt -w on any listed file)
go vet ./internal/lock/            # Expected: exit 0
go build ./internal/lock/          # Expected: exit 0
go build ./...                     # Expected: exit 0 (leaf adds cleanly; nothing else changed)
```

### Level 2: Unit Tests (the package's own validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/lock/ -v   # Expected: all green (dir/hash/contents/contention/release/SetSnapshot)

# Expected: TestAcquire_Contention_HeldError proves two Acquire calls on the same repo → second returns
# *HeldError with the first's parsed contents; after Release, a third Acquire succeeds (auto-release).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...                 # Expected: ALL packages green (S1 adds a leaf, changes nothing else)
go vet ./...                        # Expected: exit 0

# Confirm ONLY the new package + the 2 docs changed (go.mod/go.sum too ONLY under Option A)
git status --porcelain -- internal/ docs/ go.mod go.sum
# Expected: internal/lock/{lock,lock_unix,lock_windows,lock_test}.go + docs/{how-it-works,configuration}.md
#           (+ go.mod/go.sum under Option A only). No other files.
```

### Level 4: Behavioral Smoke (manual cross-check of the primitive)

```bash
cd /home/dustin/projects/stagecoach

# (Optional) a throwaway /tmp main that imports internal/lock and prints the resolved lock path + hash
# for the CWD, then Acquire/Release — confirms XDG resolution + the canonical hash end-to-end. Not required
# (lock_test.go covers it in-process), but useful if you want to eyeball the real <hash>.lock path under
# your XDG_RUNTIME_DIR. The permanent coverage is lock_test.go's TestAcquireRelease_RoundTrip +
# TestAcquire_Contention_HeldError.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the new `./internal/lock/`).

### Feature Validation

- [ ] `lock.go` exports Locker/Acquire/Release/SetSnapshot/package-SetSnapshot/LockContents/HeldError/parseContents + `var current atomic.Pointer[Locker]`.
- [ ] `lockDir()` resolves XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache (IsAbs-guarded); errors with NO CWD fallback.
- [ ] `Acquire` MkdirAll's the dir, hashes the canonical path, opens `<hash>.lock`, flocks; writes contents; sets singleton.
- [ ] `Acquire` on contention returns `*HeldError` with the holder's parsed contents.
- [ ] `Release` is idempotent; clears the singleton if it matches.
- [ ] `SetSnapshot` (both forms) rewrites `snapshot=` in place; package form is nil-safe no-op.
- [ ] `lock_unix.go` (`//go:build !windows`) uses stdlib `syscall.Flock` (Option B) or `unix.Flock` (Option A).
- [ ] `lock_windows.go` (`//go:build windows`) is a documented no-op stub.

### Scope Discipline Validation

- [ ] ONLY `internal/lock/` (4 files) + 2 docs created/modified (go.mod/go.sum too ONLY under Option A).
- [ ] Did NOT edit `internal/cmd`/`internal/generate`/`internal/decompose`/`internal/exitcode` (S2).
- [ ] Did NOT edit `docs/cli.md` (S2) or `README.md` (S3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Follows the singleton pattern (`internal/signal`) and platform-split convention (`signal_*.go`/`procgroup_*.go`).
- [ ] `lockDir()` mirrors `config/globalConfigPath` (IsAbs guard) but deliberately drops the CWD fallback.
- [ ] Dependency choice documented (Option B stdlib recommended; Option A x/sys if mandated).
- [ ] Tests reset the singleton in t.Cleanup (-race clean); contention test uses two separate OpenFile fds.

---

## Anti-Patterns to Avoid

- ❌ Don't blindly `go get golang.org/x/sys` — verify first. It is NOT a transitive dep (empty go.sum), and
  the codebase deliberately avoids x/sys (4 files document it). Prefer stdlib `syscall.Flock` (Option B).
- ❌ Don't add a CWD/repo fallback to `lockDir()`. A lock inside the repo is the §18.5 anti-pattern
  (pollutes git status, committable, ambiguous across worktrees, lost on clone). Missing home + no XDG is
  a hard error.
- ❌ Don't honor a relative XDG var. The `filepath.IsAbs` guard is mandatory (mirrors globalConfigPath).
- ❌ Don't hash the raw `repoPath` — `filepath.EvalSymlinks` FIRST so two terminals in the same repo via
  different paths hash identically. Fall back to `filepath.Abs` only if EvalSymlinks errors.
- ❌ Don't append the `snapshot=` line on SetSnapshot — Truncate(0)+Seek(0,0) and rewrite, or you'll
  duplicate it. Cache pid/hostname/repo/timestamp on the Locker so you don't re-read the file.
- ❌ Don't forget `os.MkdirAll(dir, 0o700)` — the resolved lock dir usually doesn't exist yet.
- ❌ Don't forget to reset the singleton in tests (t.Cleanup → current.Store(nil)) — it leaks across tests
  under -race (mirror signal_test.go's installTestHandler).
- ❌ Don't write the contention test with one shared fd — flock is per-open-file-description, so two
  goroutines must each call Acquire (each opens its own fd) for the second to get EWOULDBLOCK.
- ❌ Don't touch `internal/cmd`/`generate`/`decompose`/`exitcode`/`docs/cli.md`/README — that's S2/S3. S1
  is the self-contained leaf package + 2 Mode-A docs only.
- ❌ Don't try to test the Windows no-op stub on Linux — the `//go:build windows` tag excludes it from the
  Linux build; the CAS is the real guarantee there. Coverage of the stub is implicit in the build split.
- ❌ Don't implement stale-lock reaping / PID-liveness — flock auto-releases on death; that's the whole
  point (PRD §18.5 deliberately rejects the O_CREAT|O_EXCL+PID-check pattern).

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a self-contained leaf package with the design fully pre-resolved by the architecture
docs (`system_context.md` §1–§5 + `integration_seams.md` §5–§6), which give the verbatim `lockDir()`, the
singleton pattern (mirroring `internal/signal`), the exact build-tag convention, the file-format/contents,
and the S1/S2 boundary. Three proven codebase patterns are reused directly (signal singleton, signal/
procgroup platform split, config XDG resolution). The one judgment call — the dependency choice — is
surfaced with evidence (go.sum is empty; 4 files document the stdlib-only convention; `syscall.Flock` is
in the stdlib) and a clear recommendation (Option B), with the contract-literal alternative (Option A)
documented. The test matrix is fully specified for in-process (-race) validation including the subtle
"two separate fds" contention requirement. The residual uncertainty (not 10/10) is purely the dependency
decision's reviewer-facing perception (a reviewer expecting x/sys per the literal contract may question
Option B) — mitigated by the evidence-based recommendation and the fact that either option is functionally
correct and build-green. S2 (wiring/contention/Busy/E2E) is cleanly fenced and cannot be broken by S1
since the package is a pure leaf that nothing else imports yet.
