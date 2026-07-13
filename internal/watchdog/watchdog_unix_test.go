//go:build !windows

package watchdog

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/signal"
)

// TestArm_FiresTriggerOnParentDeath is the detection-path proof (§9.27 FR-K1/K2): when the poll
// observes a parent-pid CHANGE (osGetppid() != originalPpid — NOT == 1), it calls
// signal.Trigger(syscall.SIGTERM), which routes through the real signal.Install path
// (forward→cancel→rescue/exit). A fake Exit records the code without exiting, so the test asserts
// exitCode == 143 (SIGTERM, pre-snapshot). Mirrors signal_test.go's installTestHandler +
// TestHandler_Exit143SIGTERM idiom. White-box (package watchdog) so the unexported osGetppid seam
// can be swapped. The exit code is read via a channel to stay race-free under -race.
func TestArm_FiresTriggerOnParentDeath(t *testing.T) {
	exitCh := make(chan int, 1)
	buf := new(bytes.Buffer)

	ctx, _ := signal.Install(context.Background(), signal.Options{
		Exit: func(c int) { exitCh <- c }, // fake Exit records WITHOUT exiting; sent on a channel
		Out:  buf,
	})
	t.Cleanup(func() {
		signal.RestoreDefault() // stop the installed handler's run goroutine (closes its channel)
	})

	const orig, reparented = 11111, 99999
	restore := setGetppidForTest(func() int { return orig })
	t.Cleanup(restore) // restore the real seam

	Arm(ctx, 5*time.Millisecond) // captures orig as originalPpid

	setGetppidForTest(func() int { return reparented }) // simulate parent death (poll will see the change)

	// Wait for the fake Exit to record 143 (the watchdog poll drives Trigger → handle → Exit).
	// The race-safe channel handoff replaces the shared exitCode variable the naive idiom uses.
	select {
	case exitCode := <-exitCh:
		if exitCode != 143 {
			t.Errorf("exitCode = %d, want 143 (SIGTERM pre-snapshot via signal.Trigger)", exitCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit code 143 (watchdog did not fire Trigger on parent death)")
	}

	Stop() // belt-and-suspenders cleanup
}
