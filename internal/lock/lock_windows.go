//go:build windows

package lock

// flock is a no-op on Windows. Windows has no POSIX flock; the §13.5 CAS
// (update-ref HEAD compare-and-swap) is the actual safety guarantee per
// PRD §18.5 (per-host limit). A no-op flock is correct for this
// defense-in-depth layer — the CAS catches everything on Windows.
func flock(fd int) error { return nil }

// isWouldBlock always returns false on Windows (no real flock contention).
func isWouldBlock(err error) bool { return false }
