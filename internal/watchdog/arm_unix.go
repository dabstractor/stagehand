//go:build !windows

package watchdog

import (
	"runtime"
	"syscall"
	"time"

	"github.com/dustin/stagecoach/internal/signal"
)

// armImpl arms Unix parent-death detection (§9.27 FR-K1/K2): a getppid() poll goroutine (reliable,
// subreaper-safe) PLUS, on Linux, a best-effort prctl(PR_SET_PDEATHSIG) kernel fast path. On a
// parent-pid CHANGE (osGetppid() != originalPpid — NOT getppid()==1, which is wrong under subreapers
// per FR-K2) it calls signal.Trigger(SIGTERM) — the single rescue/exit path (forward-to-child-group
// → cancel ctx → rescue exit 3 or plain exit 143, with OnRescueExit releasing the lock) — and fires
// the notifier.
//
// The poll ALWAYS runs (even on Linux): prctl is per-thread and the runtime can retire the
// LockOSThread-pinned thread after UnlockOSThread (losing the setting); the poll covers that race,
// thread retirement, and the fork→prctl gap. n.fire() stops the poll and is safe to call alongside
// the cancel-watcher in watchdog.go (sync.Once).
func armImpl(originalPpid int, interval time.Duration, n *notifier) {
	if runtime.GOOS == "linux" {
		// Best-effort kernel fast path; failure is non-fatal (the poll is the reliable detector).
		_ = armPdeathsig(syscall.SIGTERM)
	}

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-n.done():
				// Stopped: detection fired elsewhere OR the cancel-watcher fired on ctx cancel.
				return
			case <-t.C:
				// Parent-pid CHANGE (reparented to init/subreaper) — NOT == 1 (subreaper-safe, FR-K2).
				if osGetppid() != originalPpid {
					// Routes through handle(): forward→cancel→rescue/exit + lock release (OnRescueExit).
					// Nil-safe (no handler installed → no-op) and stopped-guarded (post-RestoreDefault → no-op).
					signal.Trigger(syscall.SIGTERM)
					n.fire() // idempotent; wakes the cancel-watcher and stops this goroutine's next tick.
					return
				}
			}
		}
	}()
}
