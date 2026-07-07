// Package lock implements the FR52 per-repo run lock (PRD §18.5): an advisory
// flock(LOCK_EX|LOCK_NB) auto-released on process death — the LOCK never goes
// stale; orphaned FILES (SIGKILL/crash/signal-rescue os.Exit bypassing the
// deferred Remove) are reaped by pid-liveness on Acquire.
// The lock file lives outside the repo (XDG runtime/cache dir) keyed by a
// sha256 hash of the repo's canonical absolute path.
//
// This package imports ONLY stdlib (no golang.org/x/sys — matches the codebase's
// stdlib-only convention; syscall.Flock is in the Go stdlib on linux/darwin).
// It is a self-contained leaf (no stagehand imports — no cycle risk).
//
// The SetSnapshot singleton (mirrors internal/signal.active) lets library layers
// publish the frozen tree SHA for the no-op fast path without knowing whether a
// lock is held. nil-safe: SetSnapshot is a no-op when no lock is acquired.
package lock

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Locker is an acquired per-repo run lock (PRD §18.5 / FR52). The fd holds an
// advisory flock (LOCK_EX|LOCK_NB) that auto-releases when the fd/process
// closes — the LOCK never goes stale (flock auto-releases); orphaned FILES are
// reaped by pid-liveness on the next Acquire. Create via Acquire; release via
// Release (idempotent) or process exit.
type Locker struct {
	file      *os.File
	path      string
	pid       string
	hostname  string
	repo      string
	timestamp string
}

// LockContents is the parsed key=value contents of a lock file (diagnostic +
// snapshot fast-path).
type LockContents struct {
	Pid, Hostname, Repo, Timestamp, Snapshot string
}

// HeldError is returned by Acquire when another stagehand process holds the lock
// (LOCK_NB failed). Contents is the holder's parsed lock file (for the
// contention message); Path is the lock file path.
type HeldError struct {
	Contents LockContents
	Path     string
}

func (e *HeldError) Error() string {
	return fmt.Sprintf("stagehand run lock held by pid %s on %s", e.Contents.Pid, e.Contents.Hostname)
}

// current is the process-global singleton (mirrors internal/signal.active). nil
// when no lock is held (library/tests); SetSnapshot is then a nil-safe no-op.
var current atomic.Pointer[Locker]

// Acquire acquires the per-repo run lock for the given repoPath. On success it
// returns a *Locker; on contention (EWOULDBLOCK) it returns a *HeldError
// containing the holder's parsed lock file contents. The lock is an advisory
// flock auto-released on process death — the LOCK never goes stale; after
// taking its own flock, Acquire reaps orphaned *.lock FILES whose recorded pid
// is dead (the holder's own live-pid file survives).
func Acquire(repoPath string) (*Locker, error) {
	path, err := lockPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("lock path: %w", err)
	}
	canonical, _ := lockHash(repoPath) // Issue 3: canonical path for the repo= diagnostic

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("lock dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := flock(int(f.Fd())); err != nil {
		if isWouldBlock(err) {
			data, _ := os.ReadFile(path)
			f.Close()
			return nil, &HeldError{Contents: parseContents(data), Path: path}
		}
		f.Close()
		return nil, fmt.Errorf("flock: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	host, _ := os.Hostname()
	ts := time.Now().UTC().Format(time.RFC3339)

	l := &Locker{
		file:      f,
		path:      path,
		pid:       pid,
		hostname:  host,
		repo:      canonical, // Issue 3: canonical path (was repoPath — raw CWD/symlink)
		timestamp: ts,
	}

	l.writeContents("")
	current.Store(l)
	reapStaleLocks(filepath.Dir(path)) // §18.5: reap orphaned *.lock files whose pid is dead (holder's live pid survives)
	return l, nil
}

// reapStaleLocks removes every *.lock file in dir whose recorded pid is not a
// live process on its recorded hostname (PRD §18.5 stale-FILE reaping). Called
// from Acquire AFTER the holder's own flock succeeds — the holder's pid is
// os.Getpid() (live), so its own file is never reaped. SAFETY INVARIANT: a LIVE
// pid is NEVER reaped (processAlive is conservative on every ambiguity) —
// unlinking a live holder's inode-bound-flock file would let a contender
// O_CREATE a fresh inode and flock it, defeating FR52. Only a DEAD pid (no open
// fd → no flock) is safe to unlink. Malformed/empty pid → skip (best-effort).
// All errors are ignored throughout (reaping is best-effort disk hygiene; a
// failed Glob/ReadFile/Remove is a no-op, never fatal).
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil {
			continue // malformed/empty pid → skip
		}
		if !processAlive(pid, c.Hostname) {
			os.Remove(f) // dead pid → safe to unlink (ignore error)
		}
	}
}

// Release releases the lock and removes the lock file (Issue 2 — disk hygiene).
// Idempotent: a second call is a no-op (the l.file==nil guard returns before the
// remove). Closing the fd auto-releases the flock; this also clears the singleton
// if it points at l, and best-effort removes the lock file.
//
// CRITICAL ORDERING: the fd is closed FIRST (releasing the flock on the inode)
// and the file is removed AFTER, so the remove never races a still-held flock —
// removing while held would let a contender OpenFile(O_CREATE) a freshly-created
// inode and flock it (free), defeating FR52. os.Remove errors are ignored (the
// file may already be gone, or a concurrent Acquire may have recreated it — both
// harmless; the next Acquire recreates it via O_CREATE).
func (l *Locker) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close() // release the flock FIRST
	path := l.path
	l.file = nil
	if current.Load() == l {
		current.Store(nil)
	}
	os.Remove(path) // best-effort cleanup; ignore errors
}

// SetSnapshot (method) rewrites the lock file's snapshot= line with the given
// sha, preserving pid/hostname/repo/timestamp. No-op if the lock has already
// been released.
func (l *Locker) SetSnapshot(sha string) {
	l.setSnapshot(sha)
}

// setSnapshot rewrites the lock file's snapshot= line IN PLACE on the held fd,
// preserving the cached pid/hostname/repo/timestamp plus the new sha. No-op if
// the lock has already been released.
//
// Issue 4 fix: the rewrite is Seek→Write→Truncate→Sync (see writeContents) —
// Write BEFORE Truncate so the file is NEVER empty during the rewrite (a
// contender's os.ReadFile in Acquire's EWOULDBLOCK branch never observes an
// empty/partial-diagnostic file). NEVER temp-file+os.Rename: flock is
// inode-bound and rename would orphan the holder's flock on the old inode,
// bypassing FR52 contention detection (architecture/flock_inode_constraint.md).
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil {
		return
	}
	l.writeContents(sha)
}

// SetSnapshot (package-level) publishes the snapshot SHA to the current lock
// holder. Nil-safe no-op when no lock is held (current == nil). This is the
// bridge so library layers can call it unconditionally.
func SetSnapshot(sha string) {
	if l := current.Load(); l != nil {
		l.setSnapshot(sha)
	}
}

// ReleaseCurrent releases the current lock holder, if any (nil-safe no-op when
// no lock is held). It is the exit-path seam for FR52 §18.5: signal.handle()
// cannot import internal/lock (the signal package is stdlib-only), so the
// signal handler calls an OnRescueExit callback (defaulted to a no-op; wired in
// main.go to ReleaseCurrent) immediately before os.Exit — removing the lock
// file that os.Exit's defer-skipping would otherwise orphan. Idempotent
// (Release's l.file==nil guard) and nil-safe (current==nil → no-op), exactly
// mirroring SetSnapshot.
func ReleaseCurrent() {
	if l := current.Load(); l != nil {
		l.Release()
	}
}

// lockDir returns the directory for lock files (mirrors config/globalConfigPath
// but with NO CWD fallback — a lock in the repo is the §18.5 anti-pattern).
// Resolution: XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagehand/locks.
func lockDir() (string, error) {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "stagehand", "locks"), nil
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "stagehand", "locks"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err // NO CWD fallback — a lock in the repo is the §18.5 anti-pattern
	}
	return filepath.Join(home, ".cache", "stagehand", "locks"), nil
}

// writeContents writes the lock file contents (pid/hostname/repo/timestamp/snapshot)
// as a single buffered, in-place rewrite on the held fd:
// Seek(0,0) → Write(full) → Truncate(len) → Sync. Used by BOTH Acquire (the
// initial write) and setSnapshot (the snapshot update).
//
// Write-before-Truncate is the Issue 4 invariant: the file is never empty
// mid-rewrite — it is always the old content, a prefix of the new content, or
// the complete new content. The single Write overwrites the old content from
// offset 0; Truncate(int64(len(content))) then cuts any stale trailing bytes
// left from a previous, possibly-longer content (the shrink case). The fd/inode
// is unchanged, so flock semantics hold. I/O errors are intentionally ignored
// (codebase style; the public SetSnapshot signatures are void and a write
// failure is non-fatal — flock still holds; the snapshot is a fast-path/
// diagnostic nicety).
func (l *Locker) writeContents(snapshot string) {
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Seek(0, 0)
	l.file.Write([]byte(content))
	l.file.Truncate(int64(len(content)))
	l.file.Sync()
}

// parseContents parses the key=value lock file contents from raw bytes.
// Unrecognized or malformed lines are silently skipped.
func parseContents(data []byte) LockContents {
	var c LockContents
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		kv := strings.SplitN(sc.Text(), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "pid":
			c.Pid = kv[1]
		case "hostname":
			c.Hostname = kv[1]
		case "repo":
			c.Repo = kv[1]
		case "timestamp":
			c.Timestamp = kv[1]
		case "snapshot":
			c.Snapshot = kv[1]
		}
	}
	return c
}

// lockHash returns the repo's canonical path and its sha256 hex hash. The
// canonical path (EvalSymlinks, falling back to Abs) is the single source of
// truth reused by BOTH the lock filename (hash) and the diagnostic repo= field
// (Issue 3) — the two always agree for a symlinked checkout. lockHash is the
// sole canonicalization site in the package (DRY). It is exercised directly by
// lockHash tests.
func lockHash(repoPath string) (canonical, hash string) {
	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}
	sum := sha256.Sum256([]byte(canonical))
	return canonical, hex.EncodeToString(sum[:])
}

// lockPath returns the full lock file path for a repo (lockDir + lockHash). It
// is the single source of truth shared by Acquire and the path-consistency test.
// The canonical returned by lockHash is intentionally discarded here (only the
// hash keys the filename); Acquire reuses the canonical for the repo= field.
func lockPath(repoPath string) (string, error) {
	dir, err := lockDir()
	if err != nil {
		return "", err
	}
	_, hash := lockHash(repoPath)
	return filepath.Join(dir, hash+".lock"), nil
}

// IsHeldError reports whether err is a *HeldError.
func IsHeldError(err error) bool {
	var he *HeldError
	return errors.As(err, &he)
}
