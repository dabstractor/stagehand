// This file adds the SIGINT/SIGTERM → cancel-and-rescue signal handler for the
// snapshot-based atomic-commit pipeline (P1.M6.T2.S1). installSignalHandler
// arms a POST-SNAPSHOT handler that cancels the executor context (so
// Executor.Run SIGTERM-then-SIGKILLs the agent's whole process group via
// SysProcAttr.Setpgid) and runs the rescue render if a snapshot was taken, then
// exits ui.ExitRescue (3). restoreSignalHandler resets SIGINT/SIGTERM to the
// default disposition and is called by CommitStaged immediately BEFORE
// git.UpdateRefCAS so a last-instant Ctrl-C is NOT mistaken for a generation
// failure (PRD §18.4 step 3, mirroring the reference script's `trap - INT TERM`
// before the commit). It is a stdlib-only file (os, os/signal, syscall) plus
// internal/ui for the ExitRescue constant, and it uses a plain "package
// generate" line because generate.go (P1.M6.T1.S1) OWNS the // Package generate
// doc comment, mirroring rescue.go and dedupe.go.
package generate

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/dustin/stagehand/internal/ui"
)

// exit is the os.Exit seam the handler goroutine calls after handleSignal
// returns the rescue exit code. It is a package-level var (NOT a const) so the
// exit step is injectable for testing; the subprocess integration tests re-exec
// the test binary and exercise the REAL os.Exit via the parent's exit-code
// assertion. handleSignal itself NEVER calls os.Exit — the goroutine does, via
// this seam — which is what keeps handleSignal unit-testable.
var exit = os.Exit

// commitSignals is the frozen signal set the handler arms (PRD §18.4 step 1;
// reference_impl.md §5 `trap - INT TERM`): SIGINT (Ctrl-C) and SIGTERM (a
// generic "stop" from the shell / job control). It is a var so signal.Notify
// and signal.Reset can spread it; it is package-private so the set stays
// frozen to these two and no other file can widen it.
var commitSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// activeCh is the channel signal.Notify is currently delivering to, or nil
// when no handler is armed. It is package-private and tracks ONE active
// handler at a time — CommitStaged arms exactly once after WriteTree — so
// restoreSignalHandler knows which channel to signal.Stop before nil-ing it.
var activeCh chan os.Signal

// installSignalHandler arms a SIGINT/SIGTERM handler for the post-snapshot
// window of the atomic-commit pipeline. It is called by CommitStaged
// (P1.M6.T1.S1) AFTER git.WriteTree has safely snapshotted the staged index
// into the object store and BEFORE the agent generation runs, so a
// SIGINT/SIGTERM arriving in this window (PRD §18.2: "SIGINT/SIGTERM
// post-snapshot → rescue, exit 3") does the safe thing instead of leaving a
// half-killed agent or a lost snapshot. A SIGINT/SIGTERM BEFORE this point
// uses Go's default disposition (exit 130) — no handler is armed yet, which
// realizes PRD §18.4 step 2's "else just exit".
//
// On receipt of either signal the armed goroutine: (1) calls ctxCancel, which
// cancels the executor context — Executor.Run observes ctx.Done() and sends
// SIGTERM to the agent's whole process group (SysProcAttr.Setpgid made the
// agent a group leader, so kill(-pgid) fells the agent AND any children),
// then escalates to SIGKILL after a ~2s grace; (2) calls rescueFn, which — in
// the CommitStaged wiring — gates on treeSHA != "" and renders the PRD §18.3
// recovery block via Rescue so the user's spent agent quota is recoverable via
// the printed manual commit-tree command; (3) exits ui.ExitRescue (3). The
// TREE gating lives INSIDE the rescueFn closure the caller passes (the handler
// signature is fixed to ctxCancel + rescueFn), NOT in extra handler params.
//
// The ctxCancel parameter IS the cancel func of the context Executor.Run
// observes (internal/provider/executor.go): the generate caller owns a
// cancellable context, threads ctx into Run and cancel into the signal handler,
// so the signal path and the timeout/cancel path collapse onto the SAME
// ctx.Done() → group-kill logic. No stored context or public Cancel accessor
// is needed; the cancel func parameter is the seam.
func installSignalHandler(ctxCancel, rescueFn func()) {
	// A prior handler should never still be armed (CommitStaged arms exactly
	// once), but stop it defensively so two goroutines never race to exit and
	// the old channel/goroutine never leaks if install is somehow called twice.
	if activeCh != nil {
		signal.Stop(activeCh)
	}

	// Buffered(1): a signal arriving in the tiny window between signal.Notify
	// returning and the goroutine starting to read is buffered rather than
	// dropped, so the first Ctrl-C is never lost.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, commitSignals...)
	activeCh = ch

	go func() {
		<-ch // block until a committed SIGINT/SIGTERM arrives
		exit(handleSignal(ctxCancel, rescueFn))
	}()
}

// handleSignal is the PURE, os.Exit-free core of the signal handler: it calls
// ctxCancel (if non-nil), then rescueFn (if non-nil), then returns
// ui.ExitRescue. It is the unit-testable seam — the handler goroutine wraps it
// with the os.Exit call (via the exit seam), so this function has NO side
// effect beyond what the caller-supplied closures do and never terminates the
// process directly. Both arguments are nil-guarded so handleSignal(nil, nil)
// is well-defined (returns ExitRescue without panicking).
//
// Splitting the exit out of the pure core is deliberate and mirrors Rescue
// (P1.M6.T1.S3), which is a pure-render fn whose exit code is the caller's
// job: it keeps the ordering (cancel → rescue → return-code) observable in a
// plain `go test` without a subprocess, and it lets the unit test assert the
// return value is the ui.ExitRescue CONSTANT (not a hardcoded 3).
func handleSignal(ctxCancel, rescueFn func()) int {
	if ctxCancel != nil {
		ctxCancel()
	}
	if rescueFn != nil {
		rescueFn()
	}
	return ui.ExitRescue
}

// restoreSignalHandler disarms the SIGINT/SIGTERM handler installed by
// installSignalHandler and restores the DEFAULT disposition for both signals
// (signal.Stop on the active channel + signal.Reset on the signal set — the Go
// equivalent of the reference script's `trap - INT TERM`). It is called by
// CommitStaged immediately BEFORE git.UpdateRefCAS (PRD §18.4 step 3; work-item
// contract), so a Ctrl-C that arrives during the final atomic ref update is
// NOT mistaken for a generation failure: no rescue block is printed and the
// CAS proceeds — or, if the Ctrl-C lands, Go's default disposition exits 130
// rather than 3. It is a safe no-op when no handler is armed.
//
// NOTE on ordering (Gotcha #1): restore MUST happen BEFORE UpdateRefCAS, never
// after. decisions.md §3 places the restore comment on the line AFTER
// UpdateRefCAS, but that line-POSITION is superseded by the comment TEXT and
// the PRD §18.4 step-3 contract, which both say BEFORE: restoring after would
// leave the rescue-armed window open across the ref mutation and let a
// last-instant Ctrl-C spuriously print the rescue block.
func restoreSignalHandler() {
	if activeCh != nil {
		signal.Stop(activeCh) // stop delivering committed signals to this channel
		activeCh = nil
	}
	// Restore the default disposition for both signals regardless of whether a
	// handler was armed (belt-and-suspenders, mirroring `trap - INT TERM` so a
	// SIGINT/SIGTERM returns to its pre-install behavior even if activeCh was
	// somehow already nil).
	signal.Reset(commitSignals...)
}

// CommitStaged wiring (P1.M6.T1.S1) — NOT applied in this task because
// generate.go / CommitStaged does not yet exist at implementation time (scope
// boundary: this task MUST NOT stub CommitStaged). The exact two call-sites,
// for P1.M6.T1.S1 to apply verbatim, are:
//
//	// (1) AFTER git.WriteTree returns treeSHA (the post-snapshot point):
//	installSignalHandler(cancel, func() {
//		if treeSHA != "" {
//			Rescue(out, treeSHA, parentSHA, "") // candidate is "" on the signal path
//		}
//	})
//
//	// (2) IMMEDIATELY BEFORE git.UpdateRefCAS(ref, newSHA, expectedSHA):
//	restoreSignalHandler()
//
// `cancel` is the cancel func of the context passed to Executor.Run; `out` is
// the *ui.Output; treeSHA comes from git.WriteTree and parentSHA from
// git.RevParseHEAD. Restoring BEFORE UpdateRefCAS (not after) is mandatory —
// see the Gotcha #1 note on restoreSignalHandler.
