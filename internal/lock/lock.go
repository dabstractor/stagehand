// Package lock implements the FR52 per-repo run lock (PRD §18.5): an advisory
// flock(LOCK_EX|LOCK_NB) auto-released on process death (no stale locks).
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
// closes — no stale locks. Create via Acquire; release via Release (idempotent)
// or process exit.
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
// flock auto-released on process death (no stale locks).
func Acquire(repoPath string) (*Locker, error) {
	dir, err := lockDir()
	if err != nil {
		return nil, fmt.Errorf("lock dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("lock dir: %w", err)
	}

	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}

	sum := sha256.Sum256([]byte(canonical))
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(dir, hash+".lock")

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
		repo:      repoPath,
		timestamp: ts,
	}

	l.writeContents("")
	current.Store(l)
	return l, nil
}

// Release releases the lock. Idempotent: a second call is a no-op. Closing the
// fd auto-releases the flock; this also clears the singleton if it points at l.
func (l *Locker) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()
	l.file = nil
	if current.Load() == l {
		current.Store(nil)
	}
}

// SetSnapshot (method) rewrites the lock file's snapshot= line with the given
// sha, preserving pid/hostname/repo/timestamp. No-op if the lock has already
// been released.
func (l *Locker) SetSnapshot(sha string) {
	l.setSnapshot(sha)
}

// setSnapshot rewrites the lock file's snapshot= line in place (Truncate+Seek),
// preserving cached pid/hostname/repo/timestamp plus the new sha.
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil {
		return
	}
	l.file.Truncate(0)
	l.file.Seek(0, 0)
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
// at the current file position. Caller is responsible for Truncate+Seek if
// rewriting.
func (l *Locker) writeContents(snapshot string) {
	fmt.Fprintf(l.file, "pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
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

// lockHash returns the sha256 hex hash of the repo's canonical path (EvalSymlinks,
// falling back to Abs). Exported for testing.
func lockHash(repoPath string) string {
	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// lockPath returns the full lock file path for a repo. Exported for testing.
func lockPath(repoPath string) (string, error) {
	dir, err := lockDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, lockHash(repoPath)+".lock"), nil
}

// IsHeldError reports whether err is a *HeldError.
func IsHeldError(err error) bool {
	var he *HeldError
	return errors.As(err, &he)
}
