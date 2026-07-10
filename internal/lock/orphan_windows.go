//go:build windows

package lock

// appearsOrphaned always returns false on Windows. Windows has no init-
// reparenting analog (no launchd/systemd reparenting of orphaned children), AND
// flock is a no-op here (the §13.5 CAS — update-ref HEAD compare-and-swap — is
// the guarantee per PRD §18.5/FR-K7), so orphan detection is not applicable.
// Cross-platform twin of orphan_unix.go's appearsOrphaned; called by Status only
// when the holder is alive (processAlive is always-true on Windows, so Status
// reports alive=true, orphan=false here).
func appearsOrphaned(pid int) bool { return false }
