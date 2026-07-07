// Package signal implements Stagecoach's SIGINT/SIGTERM safety net (PRD §18.4 / §9.10 FR45,
// FINDING 8). It intercepts Ctrl-C / SIGTERM, forwards the signal to the running child agent's
// whole process group (so no orphaned grandchildren survive), runs the §18.3 rescue protocol if
// the snapshot was taken (print TREE_SHA + manual recovery command, exit 3), or exits cleanly
// (130/143) pre-snapshot, and restores the default signal disposition immediately before the
// final atomic update-ref so a last-instant Ctrl-C isn't misreported as a failure.
//
// Library-safe (D4): signal is opt-in (Install). A pkg/stagecoach consumer who never installs it
// gets baseline behavior (their own ctx/signals; cmd.Cancel still kills the group). No behavior
// change for library use. All package wrappers (RegisterChild, SetSnapshot, etc.) are nil-safe
// no-ops when no handler is installed.
//
// This package imports NO stagecoach packages (stdlib-only). The rescue message reaches the handler
// via the Options.RescueFormat callback (wired in main.go), avoiding a signal↔generate import cycle.
package signal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// Options configures Install. All fields are optional — zero values are defaulted in Install.
// The injectable seams (Kill, Exit, RescueFormat, Out) exist so unit tests can exercise the
// handler logic in-process without really killing a process or exiting.
type Options struct {
	// RescueFormat assembles the §18.3 rescue message. Wired in main.go to generate.FormatRescue.
	// If nil, Install substitutes a minimal base formatter.
	RescueFormat func(treeSHA, parentSHA, candidate string) string

	// Out is where the rescue message is written (default os.Stderr).
	Out io.Writer

	// Kill forwards a signal to a child process group (default KillProcessGroup). Tests inject a recorder.
	Kill func(pid int, sig os.Signal) error

	// Exit terminates the process (default os.Exit). Tests inject a recorder so os.Exit doesn't kill them.
	Exit func(int)

	// OnRescueExit is called immediately before the handler exits on BOTH signal
	// paths (the post-snapshot rescue exit 3 AND the pre-snapshot 130/143 exit).
	// It is the exit-path lock-release seam: wired in main.go to
	// lock.ReleaseCurrent so the lock file is removed before os.Exit skips the
	// deferred Release (FR52 §18.5). Defaulted to a no-op here so the signal
	// package stays stdlib-only (it cannot import internal/lock) and so library
	// use of pkg/stagecoach (no Install wiring) is unaffected.
	OnRescueExit func()
}

// Handler is the installed signal handler (singleton — see design-decisions F4). Created by Install;
// accessed elsewhere via the nil-safe package wrappers (RegisterChild/SetSnapshot/RestoreDefault/…),
// NOT by holding the pointer directly.
type Handler struct {
	opts   Options
	ch     chan os.Signal     // buffered; signal.Notify delivers SIGINT/SIGTERM here
	cancel context.CancelFunc // cancels the signal-aware ctx (→ Execute unwinds + cmd.Cancel kills group)

	childPID atomic.Int64 // registered child PID (0 = none); Setpgid ⇒ PGID==PID, so Kill(pid) ⇒ whole group

	mu            sync.Mutex
	snapTree      string // "" = no snapshot armed (pre-snapshot signal → exit 130, no rescue)
	snapParent    string // parentSHA ("" on root commit — FormatRescue omits -p)
	snapCandidate string // last parsed message (for the §18.3 candidate note); "" if none

	stopped atomic.Bool // RestoreDefault sets this; goroutine exits after draining
}

// active is the process-global singleton (signals are process-global — see F4). nil when no handler
// is installed (library use of pkg/stagecoach); all package wrappers are nil-safe then.
var active atomic.Pointer[Handler]

// Install sets up SIGINT/SIGTERM interception and returns a context cancelled on signal. Stores
// the handler in active so the package wrappers (RegisterChild/SetSnapshot/…) reach it. Call ONCE
// in main, BEFORE cmd.Execute. opts fields are defaulted if zero.
func Install(parent context.Context, opts Options) (context.Context, *Handler) {
	// Default the injectable seams.
	if opts.RescueFormat == nil {
		opts.RescueFormat = func(tree, parent, cand string) string {
			return "❌ Commit generation failed.\nTree ID: " + tree
		}
	}
	if opts.Out == nil {
		opts.Out = os.Stderr
	}
	if opts.Kill == nil {
		opts.Kill = KillProcessGroup // build-tag func (signal_unix.go / signal_windows.go)
	}
	if opts.Exit == nil {
		opts.Exit = os.Exit
	}
	if opts.OnRescueExit == nil {
		opts.OnRescueExit = func() {} // no-op default; main.go wires lock.ReleaseCurrent (FR52 exit-path release)
	}

	ctx, cancel := context.WithCancel(parent)
	h := &Handler{opts: opts, cancel: cancel, ch: make(chan os.Signal, 1)}
	// SIGTERM is a no-op path on Windows (harmless — compiles cross-platform, branch never fires there).
	signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)
	active.Store(h)
	go h.run()
	return ctx, h
}

// Active returns the installed handler, or nil if Install was never called (library use).
func Active() *Handler { return active.Load() }

// run is the handler goroutine. One signal ⇒ forward-to-group → cancel → rescue-or-exit.
// (v1 exits on the first signal; double-Ctrl-C force-exit polish is future work.)
func (h *Handler) run() {
	for sig := range h.ch {
		h.handle(sig)
		// v1: exit on first signal (the handler calls Exit, which normally os.Exits).
		// If Exit is a test fake that doesn't actually exit, return to avoid an infinite loop.
		return
	}
}

// handle processes a single signal: forward to child group, cancel ctx, then rescue or exit.
// Extracted from run() so unit tests can call it directly without goroutine timing.
func (h *Handler) handle(sig os.Signal) {
	// If RestoreDefault was already called, do nothing (the goroutine has exited; default
	// disposition applies). This prevents a stale handle call from killing a recycled PID.
	if h.stopped.Load() {
		return
	}

	// 1. Forward the signal to the child's process group (if one is registered).
	//    Belt-and-suspenders with the executor's cmd.Cancel (which also SIGTERMs the group
	//    on the ctx cancel below). Both kill paths are idempotent SIGTERM to the group.
	if pid := h.childPID.Load(); pid > 0 {
		_ = h.opts.Kill(int(pid), sig) // -pid handled inside KillProcessGroup (Unix) / pid as-is (Windows)
	}

	// 2. Cancel the signal-aware ctx → Execute returns context.Canceled → CommitStaged unwinds.
	h.cancel()

	// 3. Rescue (snapshot armed) or plain exit (pre-snapshot). Snapshot read under the lock.
	h.mu.Lock()
	tree, parent, cand := h.snapTree, h.snapParent, h.snapCandidate
	h.mu.Unlock()

	if tree != "" {
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand)) // Fprintln adds the trailing \n
		h.opts.OnRescueExit()                                             // release the lock file before os.Exit orphans it (FR52 §18.5)
		h.opts.Exit(3)                                                    // §18.2: post-snapshot → exit 3
		return
	}
	h.opts.OnRescueExit()               // pre-snapshot exit too (lock is held from default_action.go:59)
	h.opts.Exit(exitCodeForSignal(sig)) // 130 SIGINT / 143 SIGTERM — "just exit" (§18.4 step 2)
}

// ---- Nil-safe package wrappers (called by provider.Execute / generate.CommitStaged / runPipeline) ----
// Each no-ops when no handler is installed (Active()==nil), so callers need no nil-checks.

// RegisterChild records the running child's PID so a signal can be forwarded to its process group.
// Called by provider.Execute after cmd.Start. Setpgid ⇒ PGID==PID, so Kill(pid) addresses the whole tree.
func RegisterChild(pid int) {
	if h := active.Load(); h != nil {
		h.childPID.Store(int64(pid))
	}
}

// ClearChild clears the registered child PID. Called by provider.Execute (deferred) so a later
// signal can't kill a recycled PID. Idempotent.
func ClearChild() {
	if h := active.Load(); h != nil {
		h.childPID.Store(0)
	}
}

// SetSnapshot arms the rescue path: after this, a signal prints FormatRescue + exits 3.
// Called by generate.CommitStaged / runPipeline immediately after WriteTree succeeds.
func SetSnapshot(treeSHA, parentSHA, candidate string) {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapTree, h.snapParent, h.snapCandidate = treeSHA, parentSHA, candidate
		h.mu.Unlock()
	}
}

// SetCandidate updates the candidate message (for the §18.3 note) without touching tree/parent.
// Called in the generate loop after each parsed message.
func SetCandidate(candidate string) {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapCandidate = candidate
		h.mu.Unlock()
	}
}

// ClearSnapshot disarms the rescue path. Belt-and-suspenders on success (RestoreDefault already
// neuters the handler before update-ref). Called before the success return.
func ClearSnapshot() {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapTree, h.snapParent, h.snapCandidate = "", "", ""
		h.mu.Unlock()
	}
}

// RestoreDefault stops signal delivery to our channel, restoring Go's default disposition.
// Called right before update-ref (§18.4 step 3) so a last-instant Ctrl-C isn't mistaken for a
// failure. Idempotent. After this, a signal uses Go's default disposition (exit), NOT rescue.
func RestoreDefault() {
	if h := active.Load(); h != nil {
		if h.stopped.CompareAndSwap(false, true) {
			signal.Stop(h.ch) // stop delivering SIGINT/SIGTERM to h.ch → default disposition restored
			close(h.ch)       // let the goroutine's range exit
		}
	}
}
