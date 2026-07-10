//go:build windows

package watchdog

import "time"

// armImpl is a no-op on Windows (§9.27 FR-K7): Windows has no controlling-terminal-hangup analog
// and no init-reparenting, so there is no parent-death concept to watch. The notifier is never
// fired here; the cancel-watcher in watchdog.go still fires it harmlessly on ctx cancel / Stop.
// No poll goroutine, no prctl. The params are assigned to the blank identifier so unused-parameter
// linters and go vet stay quiet.
func armImpl(originalPpid int, interval time.Duration, n *notifier) {
	_ = originalPpid
	_ = interval
	_ = n
}
