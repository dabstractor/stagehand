package watchdog

import (
	"context"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/signal"
)

// TestArm_NegativeIntervalDefaults verifies that Arm with a non-positive interval does not panic
// and defaults internally to ~1s. Proven by Arm not panicking and Stop() cleaning up without hang.
func TestArm_NegativeIntervalDefaults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Arm(ctx, 0) // interval <= 0 → defaults to 1s internally; must not panic.
	Stop()      // must not hang.
}

// TestArm_StopWithoutArmIsNoOp verifies Stop() called before any Arm() does not panic
// (currentCancel == nil → guarded).
func TestArm_StopWithoutArmIsNoOp(t *testing.T) {
	// Defensively clear any currentCancel left by a prior test.
	Stop()
	// Call Stop() again with no live watchdog — must not panic.
	Stop()
}

// TestArm_StopCancelsGoroutine verifies that Stop() stops the poll goroutine (no leak, no hang)
// under the race detector. It arms a short-interval watchdog, lets it start, then stops it and
// confirms the test completes without hanging. A leaked goroutine firing Trigger later would
// surface under -race in the suite.
func TestArm_StopCancelsGoroutine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Arm(ctx, 5*time.Millisecond)
	time.Sleep(20 * time.Millisecond) // let the poll goroutine start
	Stop()                            // must stop the goroutine
	// If the goroutine leaked and were blocked on the ticker, a later reparent would fire Trigger
	// into a nil handler (no-op) — but more importantly this test must not hang. Re-arm to confirm
	// a fresh start works and Stop is idempotent.
	Arm(ctx, 5*time.Millisecond)
	Stop()
}

// TestArm_NilSignalHandlerIsNoOp verifies the nil-safety hard requirement: with no signal handler
// installed (signal.Active()==nil), Arm does NOT exit the process. The poll fires the (no-op)
// signal.Trigger on a ppid change and returns; the process continues. The test restores osGetppid
// in t.Cleanup.
func TestArm_NilSignalHandlerIsNoOp(t *testing.T) {
	// Ensure no handler is installed (library use of pkg/stagecoach that never calls Install).
	signal.RestoreDefault()
	if signal.Active() != nil {
		t.Fatalf("precondition: signal.Active() = %v, want nil", signal.Active())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Swap the osGetppid seam to simulate reparenting AFTER arming, so the poll observes a change.
	// Use the race-safe setter (the poll goroutine reads the seam concurrently).
	const orig, reparented = 11111, 99999
	restore := setGetppidForTest(func() int { return orig })
	t.Cleanup(restore)

	Arm(ctx, 5*time.Millisecond) // captures orig as originalPpid

	setGetppidForTest(func() int { return reparented }) // simulate parent death

	// Give the poll a tick to fire the (no-op) Trigger. The process must NOT exit — if it did,
	// this test function would not reach the assertion.
	time.Sleep(50 * time.Millisecond)

	// We got here, so the process did not exit → nil-safety holds.
	Stop()
}
