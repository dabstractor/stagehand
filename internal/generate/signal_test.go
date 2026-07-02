package generate

// White-box test for the SIGINT/SIGTERM → cancel-and-rescue signal handler
// (P1.M6.T2.S1), matching the internal/generate/rescue_test.go and
// internal/provider/executor_test.go house conventions: the _test.go file is
// `package generate` (NOT `package generate_test`) so it sits in the same
// package as the (exported) functions under test and can read package-private
// state (activeCh, commitSignals). It has TWO layers:
//
//   - PURE unit tests for handleSignal (the os.Exit-free core): assert the
//     cancel→rescue→return-ExitRescue ordering and the nil-arg safety, with NO
//     subprocess and NO real signal. These need stdlib `testing` ONLY.
//
//   - SUBPROCESS re-exec integration tests that prove the REAL signal path: a
//     SIGINT to a handler-armed child prints the §18.3 rescue block + frozen
//     TREE_SHA and exits 3; and after restoreSignalHandler(), a SIGINT during a
//     mocked update-ref does NOT rescue and exits with the default disposition
//     (130, i.e. != 3). The child is the test binary itself re-invoked via
//     `os.Args[0] -test.run=^TestName$` with STAGEHAND_SIGNAL_TEST gating the
//     child branch (the canonical Go helper-process pattern, used by
//     os/exec_test and the executor tests' REAL-process/timing style). The
//     parent/child handshake over stdout ("READY") makes the tests flake-free
//     without a fixed sleep.
//
// It uses stdlib bytes/bufio/errors/os/os/exec/strings/syscall/testing/time +
// internal/ui ONLY (NO testify, NO internal/git — the rescue render is driven
// directly via Rescue + a ui.Output wired to the child's os.Stderr). The
// containsAll helper is reused from rescue_test.go (same package).

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/ui"
)

// Fixed tree/parent SHAs the child rescueFn renders; the parent asserts these
// substrings appear (rescue test) or do NOT appear (restore test) in the
// child's stderr, proving the frozen TREE_SHA is carried on the signal path.
const (
	rescueTreeSHA   = "9f3a1c0deadbeef"
	rescueParentSHA = "abc1234"
)

// TestHandleSignal_CallsCancelAndRescueThenReturnsExitRescue pins the pure-core
// contract (PRD §18.4 step 2 cancel → rescue → exit-3): handleSignal invokes
// ctxCancel, then rescueFn, then returns the ui.ExitRescue CONSTANT (not a
// hardcoded 3) — and it does all this with NO os.Exit, so a plain `go test`
// observes every step. The ordering is captured into a sequence slice (the call
// is synchronous on the test goroutine, so no race).
func TestHandleSignal_CallsCancelAndRescueThenReturnsExitRescue(t *testing.T) {
	var seq []string
	cancelCalled := false
	rescueCalled := false

	cancelFn := func() {
		cancelCalled = true
		seq = append(seq, "cancel")
	}
	rescueFn := func() {
		rescueCalled = true
		seq = append(seq, "rescue")
	}

	code := handleSignal(cancelFn, rescueFn)

	if !cancelCalled {
		t.Errorf("handleSignal did not call ctxCancel (must cancel the executor context first)")
	}
	if !rescueCalled {
		t.Errorf("handleSignal did not call rescueFn (must render the §18.3 block)")
	}
	if code != ui.ExitRescue {
		t.Errorf("handleSignal returned %d, want %d (ui.ExitRescue, NOT a hardcoded 3)", code, ui.ExitRescue)
	}
	// Ordering: cancel MUST happen before rescue (the agent's group kill is
	// initiated before the user-facing block is printed).
	if len(seq) != 2 || seq[0] != "cancel" || seq[1] != "rescue" {
		t.Errorf("handleSignal call order = %v, want [cancel rescue]", seq)
	}
}

// TestHandleSignal_NilArgsDoNotPanic pins the nil-guard contract from the Final
// Validation Checklist: handleSignal(nil, nil) is well-defined and returns
// ui.ExitRescue without panicking, so the handler never crashes if a caller
// forgets to wire one of the closures.
func TestHandleSignal_NilArgsDoNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleSignal(nil, nil) panicked: %v", r)
		}
	}()

	code := handleSignal(nil, nil)
	if code != ui.ExitRescue {
		t.Errorf("handleSignal(nil, nil) returned %d, want %d (ui.ExitRescue)", code, ui.ExitRescue)
	}
}

// TestHandleSignal_PartialNilArgs exercises the mixed nil-guards: a nil rescueFn
// with a real cancel still cancels and returns ExitRescue; a nil cancel with a
// real rescueFn still rescues. This locks in that each closure is independently
// nil-guarded.
func TestHandleSignal_PartialNilArgs(t *testing.T) {
	t.Run("nil rescueFn still cancels", func(t *testing.T) {
		called := false
		code := handleSignal(func() { called = true }, nil)
		if !called {
			t.Errorf("ctxCancel not called when rescueFn is nil")
		}
		if code != ui.ExitRescue {
			t.Errorf("returned %d, want %d", code, ui.ExitRescue)
		}
	})
	t.Run("nil ctxCancel still rescues", func(t *testing.T) {
		called := false
		code := handleSignal(nil, func() { called = true })
		if !called {
			t.Errorf("rescueFn not called when ctxCancel is nil")
		}
		if code != ui.ExitRescue {
			t.Errorf("returned %d, want %d", code, ui.ExitRescue)
		}
	})
}

// TestRestoreSignalHandler_RestoresDefaultDisposition is a PURE check (no
// subprocess) that restoreSignalHandler clears the package-private activeCh and
// is safe to call when nothing is armed (no-op). The full behavioral proof that
// a post-restore SIGINT uses the default disposition is the subprocess test
// TestSignal_RestoreBeforeUpdateRef below.
func TestRestoreSignalHandler_RestoresDefaultDisposition(t *testing.T) {
	// Start clean: nothing armed.
	if activeCh != nil {
		t.Fatalf("precondition: activeCh = %v, want nil", activeCh)
	}
	// Restoring with nothing armed must be a safe no-op.
	restoreSignalHandler()
	if activeCh != nil {
		t.Errorf("activeCh = %v, want nil after no-op restore", activeCh)
	}

	// Arm, then restore, then activeCh must be nil again.
	installSignalHandler(func() {}, func() {})
	if activeCh == nil {
		t.Fatalf("activeCh = nil after install, want a channel")
	}
	restoreSignalHandler()
	if activeCh != nil {
		t.Errorf("activeCh = %v, want nil after restore", activeCh)
	}
}

// exitInfo extracts the child's exit outcome from a cmd.Wait error: the exit
// code (128+signal for a signal kill, matching shell convention), whether the
// process was terminated by a signal, and which signal. It lets the subprocess
// tests assert both "exit 3" (rescue) and "killed by SIGINT with the default
// disposition" (restore) precisely.
func exitInfo(err error) (code int, signaled bool, sig syscall.Signal) {
	if err == nil {
		return 0, false, 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				s := ws.Signal()
				return 128 + int(s), true, s
			}
			return ws.ExitStatus(), false, 0
		}
		return ee.ExitCode(), false, 0
	}
	return -1, false, 0
}

// waitForReady reads the child's stdout line by line until it emits "READY",
// then returns nil. It is the parent/child handshake that makes the signal
// tests flake-free: the parent only sends SIGINT AFTER the child has armed the
// handler (rescue test) or restored it (restore test), so there is no race on
// handler installation. A timeout guards against a child that never becomes
// ready.
func waitForReady(t *testing.T, cmd *exec.Cmd, stdout *bufio.Reader) {
	t.Helper()
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == "READY" {
				ready <- struct{}{}
				return
			}
		}
		// Scanner stopped (pipe closed / error): signal so the select does not
		// hang the full timeout.
		ready <- struct{}{}
	}()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("child never signaled READY within 3s")
	}
}

// rescueChildRescueFn is the rescueFn the rescue-test child installs: it renders
// the PRD §18.3 block to the child's os.Stderr via Rescue + a ui.Output, exactly
// mirroring the documented CommitStaged wiring (Rescue(out, treeSHA, parentSHA,
// "")). candidate is "" on the signal path. noColor=true keeps the captured
// stderr plain so the parent's substring assertions hold.
func rescueChildRescueFn() {
	o := ui.NewOutput(os.Stdout, os.Stderr, false, true)
	Rescue(o, rescueTreeSHA, rescueParentSHA, "")
}

// TestSignal_RescueOnSIGINT proves the REAL signal path (PRD §18.2:
// SIGINT/SIGTERM post-snapshot → rescue, exit 3): a handler-armed child that
// receives a real SIGINT prints the §18.3 rescue block (carrying the frozen
// TREE_SHA) to stderr and exits with code 3 (ui.ExitRescue). The child is the
// test binary re-invoked with STAGEHAND_SIGNAL_TEST=rescue; it arms the
// handler, signals READY, then blocks until the SIGINT lands.
func TestSignal_RescueOnSIGINT(t *testing.T) {
	if os.Getenv("STAGEHAND_SIGNAL_TEST") == "rescue" {
		// CHILD: arm the handler, then block. SIGINT triggers cancel→rescue→exit3.
		installSignalHandler(func() {}, rescueChildRescueFn)
		if _, err := os.Stdout.WriteString("READY\n"); err != nil {
			t.Fatalf("child: write READY: %v", err)
		}
		// select {} blocks forever; the armed handler goroutine exits(3) on SIGINT
		// before this could ever return, so there is deliberately no return here.
		select {}
	}

	// PARENT: exec the test binary in child mode, handshake, send SIGINT, assert.
	cmd := exec.Command(os.Args[0], "-test.run=^TestSignal_RescueOnSIGINT$")
	cmd.Env = append(os.Environ(), "STAGEHAND_SIGNAL_TEST=rescue")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	waitForReady(t, cmd, bufio.NewReader(stdoutPipe))

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	err = cmd.Wait()

	code, _, _ := exitInfo(err)
	if code != ui.ExitRescue {
		t.Errorf("exit code = %d, want %d (ui.ExitRescue) after SIGINT to a handler-armed child (wait err=%v)", code, ui.ExitRescue, err)
	}

	got := stderr.String()
	// The §18.3 block must be present with the frozen TREE_SHA, the manual
	// recovery command, and the failure notice — proving the rescue render ran
	// on the signal path and carried the snapshot's tree SHA.
	if missing := containsAll(got,
		"Commit generation failed",             // the §18.3 failure notice
		"safely snapshotted before generation", // the snapshot-safety line
		"Tree ID: "+rescueTreeSHA,              // the frozen TREE_SHA echoed
		rescueTreeSHA,                          // tree also appears in the manual command
		"git commit-tree",                      // manual command token
		"xargs git update-ref HEAD",            // manual command token
		"commit-tree -p "+rescueParentSHA,      // -p <parent> adjacency (non-root)
	); missing != "" {
		t.Errorf("rescue stderr missing %q\n--got--\n%s", missing, got)
	}
}

// TestSignal_RestoreBeforeUpdateRef proves restoreSignalHandler's contract
// (PRD §18.4 step 3): after the handler is restored (called immediately BEFORE
// git.UpdateRefCAS), a SIGINT during the final ref update is NOT mistaken for a
// generation failure — no rescue block is printed and the process exits with
// Go's DEFAULT SIGINT disposition (130, i.e. exit code != 3). The child arms
// the handler, restores it, signals READY, then blocks in the mock update-ref
// window; the parent sends SIGINT and asserts no rescue + non-rescue exit.
func TestSignal_RestoreBeforeUpdateRef(t *testing.T) {
	if os.Getenv("STAGEHAND_SIGNAL_TEST") == "restore" {
		// CHILD: arm then IMMEDIATELY restore (mirroring CommitStaged restoring
		// right before UpdateRefCAS), then block in the mock update-ref window.
		// SIGINT now hits the DEFAULT disposition, not the rescue handler.
		installSignalHandler(func() {}, rescueChildRescueFn)
		restoreSignalHandler()
		if _, err := os.Stdout.WriteString("READY\n"); err != nil {
			t.Fatalf("child: write READY: %v", err)
		}
		// select {} blocks in the mock update-ref window; SIGINT's default
		// disposition terminates the process (exit 130) before this returns, so
		// there is deliberately no return here.
		select {}
	}

	// PARENT: exec the test binary in child mode, handshake, send SIGINT, assert.
	cmd := exec.Command(os.Args[0], "-test.run=^TestSignal_RestoreBeforeUpdateRef$")
	cmd.Env = append(os.Environ(), "STAGEHAND_SIGNAL_TEST=restore")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}

	waitForReady(t, cmd, bufio.NewReader(stdoutPipe))

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	err = cmd.Wait()

	code, signaled, sig := exitInfo(err)

	// The defining contract: restore worked, so this is NOT a rescue exit.
	if code == ui.ExitRescue {
		t.Errorf("exit code = %d (ui.ExitRescue), want != 3 — restore must prevent the rescue path during update-ref (stderr=%q)", code, stderr.String())
	}
	// On linux (the validation gate) a default-disposition SIGINT deterministically
	// terminates the process with SIGINT (exit 130); assert it precisely so a
	// regression to "handler still armed and rescued" can't sneak through as some
	// other non-3 code.
	if !signaled || sig != syscall.SIGINT {
		t.Errorf("child exit = (code=%d, signaled=%v, sig=%v), want terminated by SIGINT (default disposition → 130); this proves restoreSignalHandler reset the disposition (stderr=%q)", code, signaled, sig, stderr.String())
	}
	// And crucially: NO rescue block. restoreSignalHandler must keep the
	// rescue-armed window CLOSED across the ref update.
	got := stderr.String()
	for _, needle := range []string{
		"Commit generation failed",
		"Tree ID: " + rescueTreeSHA,
		"xargs git update-ref HEAD",
	} {
		if strings.Contains(got, needle) {
			t.Errorf("restore-path stderr must NOT contain %q (no rescue block after restore)\n--got--\n%s", needle, got)
		}
	}
}
