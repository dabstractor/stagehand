// Package watchdog implements the §9.27 FR-K1/K2 parent-death watchdog.
//
// When the launching process dies without sending a signal (the lazygit/IDE/detaching-terminal
// case — §18.5's orphaned-run gap), the child stagecoach is reparented to init/subreaper. The
// watchdog detects the parent-pid change and calls signal.Trigger(SIGTERM), reusing the signal
// handler's rescue/lock-release exit path (forward-to-child-group → cancel ctx → rescue exit 3 or
// plain exit 143, with OnRescueExit releasing the lock file before exit on BOTH branches — FR52
// §18.5). Detection is a parent-pid CHANGE from the value captured at Arm time (subreaper-safe —
// NOT getppid()==1, which is wrong under systemd-run/docker/supervisord per PRD §9.27 FR-K2).
//
// prctl(PR_SET_PDEATHSIG) is a Linux-only best-effort kernel fast path: when it works the kernel
// delivers a real SIGTERM with no poll latency. It is PER-THREAD, so the getppid poll ALWAYS runs
// (even on Linux) as the reliable detector — the runtime may retire the LockOSThread-pinned
// thread after UnlockOSThread (losing the setting), and there is a fork→prctl race window.
//
// Windows is a no-op (FR-K7): Windows has no controlling-terminal-hangup analog and no
// init-reparenting, so there is no parent-death concept to watch.
//
// This package imports ONLY the Go stdlib and internal/signal (one-directional — internal/signal
// never imports back, so there is no cycle). It does NOT import internal/lock (the lock release
// rides signal's OnRescueExit seam wired in main.go) or internal/config (the cfg.NoParentWatchdog
// gate is read by the consumer, P1.M2.T2.S2).
package watchdog

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// osGetppid is the test seam over os.Getppid. Production code reads the real parent PID through
// it; tests swap it via setGetppidForTest to simulate parent death (reparenting) without forking.
//
// It is backed by an atomic pointer so the poll goroutine's reads never race with a test's swap
// (the race detector is a hard requirement). Both the getter (osGetppid) and the test setter go
// through the atomic load/store. The seam is an unexported package symbol, so the tests live in
// `package watchdog` (white-box).
var getppidPtr atomic.Pointer[func() int]

func init() {
	f := os.Getppid
	getppidPtr.Store(&f)
}

// osGetppid returns the current parent PID via the (swappable) seam.
func osGetppid() int {
	f := getppidPtr.Load()
	if f == nil {
		return os.Getppid()
	}
	return (*f)()
}

// setGetppidForTest atomically swaps the osGetppid seam. Returns a restore function the caller
// defers (or runs in t.Cleanup) so the seam does not leak into other tests. Test-only helper.
func setGetppidForTest(f func() int) (restore func()) {
	prev := getppidPtr.Load()
	getppidPtr.Store(&f)
	return func() {
		if prev == nil {
			g := os.Getppid
			getppidPtr.Store(&g)
			return
		}
		getppidPtr.Store(prev)
	}
}

// currentMu guards currentCancel so Stop() can cancel the live watchdog safely from any goroutine.
var (
	currentMu     sync.Mutex
	currentCancel context.CancelFunc
)

// notifier is the leak-free lifecycle channel. It is closed once (via sync.Once) for ANY reason —
// either the poll detected a parent-pid change (and fired the rescue path) or the cancel-watcher
// noticed ctx cancel / Stop (process exit). Both the poll goroutine and the cancel-watcher goroutine
// select on done() so neither can leak blocked on a ticker.
type notifier struct {
	once sync.Once
	ch   chan struct{}
}

func newNotifier() *notifier { return &notifier{ch: make(chan struct{})} }

// fire closes the underlying channel exactly once (idempotent). Safe for concurrent callers
// (the poll on detection and the cancel-watcher on cancel can both call it).
func (n *notifier) fire() { n.once.Do(func() { close(n.ch) }) }

// done returns the channel that is closed when the watchdog has been triggered or stopped.
func (n *notifier) done() <-chan struct{} { return n.ch }

// Arm starts the parent-death watchdog (§9.27 FR-K1/K2/K7).
//
// The watchdog records the current parent PID (originalPpid := osGetppid()) and, on the Unix
// build, launches a getppid() poll goroutine that calls signal.Trigger(syscall.SIGTERM) as soon as
// it observes osGetppid() != originalPpid (the parent died and the process was reparented). On
// Linux it ALSO best-effort arms prctl(PR_SET_PDEATHSIG) as a latency fast path (the poll is the
// reliable detector because prctl is per-thread and its effect can be lost). On Windows Arm is a
// no-op (FR-K7).
//
// The poll ALWAYS runs on Unix and always selects on the notifier's done() channel, so calling
// Stop() — or cancelling ctx — stops the goroutine without a leak. The goroutine exits on its own
// when the process exits (ctx.Done() fires because ctx is the signal-aware context from main.go).
//
// Nil-safety is a hard requirement: if signal.Install was never called (library use of
// pkg/stagecoach, signal.Active()==nil), signal.Trigger is a no-op, the process does NOT exit, and
// the poll goroutine exits cleanly on ctx cancel. Arm never calls os.Exit.
//
// interval is the poll period; if interval <= 0 a ~1s default is used (FR-K2).
func Arm(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second // FR-K2 default ~1s
	}
	originalPpid := osGetppid()

	armCtx, cancel := context.WithCancel(ctx)
	currentMu.Lock()
	currentCancel = cancel
	currentMu.Unlock()

	n := newNotifier()
	armImpl(originalPpid, interval, n) // build-tagged: Unix polls + (Linux) prctl; Windows no-op

	// cancel-watcher: when ctx is cancelled (process exit or Stop), fire the notifier so the
	// poll goroutine's select on done() unblocks and the goroutine exits (leak-free).
	go func() {
		<-armCtx.Done()
		n.fire()
	}()
}

// Stop cancels the live watchdog (if any), stopping its poll goroutine. It is a no-op if Arm was
// never called (currentCancel == nil) and idempotent (safe to call multiple times). Intended for
// library/test use; the normal exit path is the process dying, which cancels the ctx passed to Arm.
func Stop() {
	currentMu.Lock()
	c := currentCancel
	currentCancel = nil
	currentMu.Unlock()
	if c != nil {
		c()
	}
}
