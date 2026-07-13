package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/lock"
)

// contentionFakeGit embeds git.Git and overrides only WriteTree — the sole method
// handleLockContention calls. Uncalled methods are nil (panics if invoked), which is
// fine since the helper calls only WriteTree (G8).
type contentionFakeGit struct {
	git.Git
	writeTreeSHA string
	writeTreeErr error
}

func (f *contentionFakeGit) WriteTree(_ context.Context) (string, error) {
	return f.writeTreeSHA, f.writeTreeErr
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_NoOpFastPath — snapshot matches contender's index → exit 0
// ---------------------------------------------------------------------------

func TestHandleLockContention_NoOpFastPath(t *testing.T) {
	held := &lock.HeldError{
		Contents: lock.LockContents{
			Pid:      "4242",
			Hostname: "testhost",
			Repo:     "/r",
			Snapshot: "abc123",
		},
		Path: "/x.lock",
	}
	g := &contentionFakeGit{writeTreeSHA: "abc123"}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	code := exitcode.For(err)
	if code != exitcode.Success {
		t.Errorf("exitcode.For(err) = %d, want %d (Success)", code, exitcode.Success)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
	if !strings.Contains(buf.String(), "nothing to do") {
		t.Errorf("stderr = %q, want 'nothing to do'", buf.String())
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_TreeDiffers — snapshot != contender's tree → exit 5
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_TreeDiffers(t *testing.T) {
	// Pin the seam so the hint's absence does not depend on the runner's process tree (a deterministic
	// not-orphaned holder). The holder here is NOT orphaned → the FR-K5 hint must be ABSENT.
	orig := orphanChecker
	t.Cleanup(func() { orphanChecker = orig })
	orphanChecker = func(lock.LockContents) bool { return false }

	held := &lock.HeldError{
		Contents: lock.LockContents{
			Pid:      "4242",
			Hostname: "testhost",
			Repo:     "/r",
			Snapshot: "abc123",
		},
		Path: "/x.lock",
	}
	g := &contentionFakeGit{writeTreeSHA: "zzz999"}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	code := exitcode.For(err)
	if code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
	msg := buf.String()
	if !strings.Contains(msg, "4242") || !strings.Contains(msg, "testhost") {
		t.Errorf("stderr = %q, want to contain pid '4242' and host 'testhost'", msg)
	}
	// FR-K5: the lock path is on its OWN line (copy-pasteable), not buried mid-sentence.
	if !strings.Contains(msg, "\nLock: /x.lock\n") {
		t.Errorf("stderr = %q, want '\\nLock: /x.lock\\n' on its own line", msg)
	}
	// Regression guard: the OLD buried-in-sentence form is GONE.
	if strings.Contains(msg, "finishes. Lock:") {
		t.Errorf("stderr = %q, must NOT contain the old buried 'finishes. Lock:' form", msg)
	}
	// The holder is not orphaned → the hint must be ABSENT.
	if strings.Contains(msg, "holder's launcher") {
		t.Errorf("stderr = %q, hint must be ABSENT when not orphaned", msg)
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_EmptySnapshot — holder didn't publish snapshot → exit 5
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_EmptySnapshot(t *testing.T) {
	held := &lock.HeldError{
		Contents: lock.LockContents{
			Pid:      "4242",
			Hostname: "testhost",
			Repo:     "/r",
			Snapshot: "", // empty → fast path skipped
		},
		Path: "/x.lock",
	}
	g := &contentionFakeGit{writeTreeSHA: "abc123"}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	code := exitcode.For(err)
	if code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_EmptyDiagnostics — Issue 4b: a contender that read a
// partial/empty lock file (the residual race window) has empty Repo/Pid/Hostname. The guard
// must substitute sensible fallbacks so the Busy message never renders as "on  (pid  on )".
// Covers BOTH ways to reach the Busy branch with empty diagnostics: (a) empty snapshot →
// fast path skipped; (b) non-matching snapshot → fast path fails → fall through. Exit code
// stays Busy(5), SILENT.
// ---------------------------------------------------------------------------
func TestHandleLockContention_Busy_EmptyDiagnostics(t *testing.T) {
	cases := []struct {
		name      string
		snapshot  string
		writeTree string // contender's WriteTree result (only consulted when snapshot != "")
	}{
		{"empty_snapshot", "", ""},                   // fast path SKIPPED (snapshot empty) → Busy; WriteTree uncalled
		{"nonmatching_snapshot", "abc123", "zzz999"}, // fast path FAILS (trees differ) → fall through → Busy
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := &lock.HeldError{
				Contents: lock.LockContents{
					Pid:      "", // empty — the partial-read reproduction
					Hostname: "",
					Repo:     "",
					Snapshot: tc.snapshot,
				},
				Path: "/x.lock", // always non-empty (lock file path) — passed through unchanged
			}
			g := &contentionFakeGit{writeTreeSHA: tc.writeTree}

			var buf bytes.Buffer
			err := handleLockContention(&buf, held, g, context.Background())

			if code := exitcode.For(err); code != exitcode.Busy {
				t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
			}
			if err.Error() != "" {
				t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
			}
			msg := buf.String()
			// The fallbacks are present:
			if !strings.Contains(msg, "an unknown repo") {
				t.Errorf("stderr = %q, want to contain repo fallback 'an unknown repo'", msg)
			}
			if !strings.Contains(msg, "<unknown>") {
				t.Errorf("stderr = %q, want to contain pid/hostname fallback '<unknown>'", msg)
			}
			// The lock Path is still reported (always non-empty):
			if !strings.Contains(msg, "/x.lock") {
				t.Errorf("stderr = %q, want to contain the lock path '/x.lock'", msg)
			}
			// The broken pattern is ABSENT (the contract's exact check + a robust no-double-space guard):
			if strings.Contains(msg, "on  (") {
				t.Errorf("stderr = %q, contains broken 'on  (' (double-space) pattern", msg)
			}
			if strings.Contains(msg, "  ") {
				t.Errorf("stderr = %q, contains a double-space (no legit double space exists in the message)", msg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_WriteTreeErr — WriteTree fails → fall through to Busy (G5)
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_WriteTreeErr(t *testing.T) {
	held := &lock.HeldError{
		Contents: lock.LockContents{
			Pid:      "4242",
			Hostname: "testhost",
			Repo:     "/r",
			Snapshot: "abc123",
		},
		Path: "/x.lock",
	}
	g := &contentionFakeGit{writeTreeErr: errors.New("merge conflicts")}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	code := exitcode.For(err)
	if code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy, falls through on WriteTree err)", code, exitcode.Busy)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_SilentExits — both Success and Busy returns are silent
// ---------------------------------------------------------------------------

func TestHandleLockContention_SilentExits(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		held := &lock.HeldError{
			Contents: lock.LockContents{Pid: "1", Hostname: "h", Repo: "/r", Snapshot: "abc123"},
			Path:     "/x.lock",
		}
		g := &contentionFakeGit{writeTreeSHA: "abc123"}
		err := handleLockContention(&bytes.Buffer{}, held, g, context.Background())
		if err.Error() != "" {
			t.Errorf("Success exit: err.Error() = %q, want empty (silent)", err.Error())
		}
	})
	t.Run("busy", func(t *testing.T) {
		held := &lock.HeldError{
			Contents: lock.LockContents{Pid: "1", Hostname: "h", Repo: "/r", Snapshot: ""},
			Path:     "/x.lock",
		}
		g := &contentionFakeGit{}
		err := handleLockContention(&bytes.Buffer{}, held, g, context.Background())
		if err.Error() != "" {
			t.Errorf("Busy exit: err.Error() = %q, want empty (silent)", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_OrphanHint — FR-K5: when the holder APPEARS
// orphaned (seam forced true) AND pid is non-empty, the hint follows Lock:
// naming the REAL pid, the lock path, and `stagecoach lock status`.
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_OrphanHint(t *testing.T) {
	orig := orphanChecker
	t.Cleanup(func() { orphanChecker = orig })
	orphanChecker = func(lock.LockContents) bool { return true } // force orphan==true

	held := &lock.HeldError{
		Contents: lock.LockContents{Pid: "4242", Hostname: "testhost", Repo: "/r", Snapshot: ""},
		Path:     "/x.lock",
	}
	g := &contentionFakeGit{}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	if code := exitcode.For(err); code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
	msg := buf.String()
	// Lock on its own line.
	if !strings.Contains(msg, "\nLock: /x.lock\n") {
		t.Errorf("want '\\nLock: /x.lock\\n' own line; got %q", msg)
	}
	// Hint names the REAL pid + the lock path + the status pointer.
	if !strings.Contains(msg, "kill 4242") {
		t.Errorf("hint must name the real pid ('kill 4242'); got %q", msg)
	}
	if !strings.Contains(msg, "rm /x.lock") {
		t.Errorf("hint must name the lock path ('rm /x.lock'); got %q", msg)
	}
	if !strings.Contains(msg, "stagecoach lock status") {
		t.Errorf("hint must point at 'stagecoach lock status'; got %q", msg)
	}
	if !strings.Contains(msg, "holder's launcher appears to have exited") {
		t.Errorf("hint wording missing ('holder's launcher appears to have exited'); got %q", msg)
	}
	// The hint must NOT substitute the <unknown> fallback into the kill instruction.
	if strings.Contains(msg, "kill <unknown>") {
		t.Errorf("hint must use the REAL pid, not '<unknown>'; got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_NoOrphanHint — the hint is ABSENT when the
// holder is not orphaned (seam false). The new own-line Lock format is present.
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_NoOrphanHint(t *testing.T) {
	orig := orphanChecker
	t.Cleanup(func() { orphanChecker = orig })
	orphanChecker = func(lock.LockContents) bool { return false } // holder not orphaned

	held := &lock.HeldError{
		Contents: lock.LockContents{Pid: "4242", Hostname: "testhost", Repo: "/r", Snapshot: ""},
		Path:     "/x.lock",
	}
	g := &contentionFakeGit{}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	if code := exitcode.For(err); code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
	msg := buf.String()
	if !strings.Contains(msg, "\nLock: /x.lock\n") {
		t.Errorf("Lock own line still present; got %q", msg)
	}
	if strings.Contains(msg, "holder's launcher") {
		t.Errorf("hint must be ABSENT when not orphaned; got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_NoOrphanHintWhenPidEmpty — the Pid != "" guard
// is INDEPENDENT of the predicate: even with the seam CLAIMING orphan==true,
// an empty pid NEVER emits the hint (the kill instruction needs a real pid).
// The Issue-4b <unknown> fallback still renders in the main message.
// ---------------------------------------------------------------------------

func TestHandleLockContention_Busy_NoOrphanHintWhenPidEmpty(t *testing.T) {
	orig := orphanChecker
	t.Cleanup(func() { orphanChecker = orig })
	orphanChecker = func(lock.LockContents) bool { return true } // predicate CLAIMS orphan

	held := &lock.HeldError{
		Contents: lock.LockContents{Pid: "", Hostname: "testhost", Repo: "/r", Snapshot: ""},
		Path:     "/x.lock",
	}
	g := &contentionFakeGit{}

	var buf bytes.Buffer
	err := handleLockContention(&buf, held, g, context.Background())

	if code := exitcode.For(err); code != exitcode.Busy {
		t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
	}
	if err.Error() != "" {
		t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
	}
	msg := buf.String()
	if strings.Contains(msg, "holder's launcher") {
		t.Errorf("hint must be ABSENT when pid is empty (guard independent of predicate); got %q", msg)
	}
	if !strings.Contains(msg, "\nLock: /x.lock\n") {
		t.Errorf("Lock own line still present; got %q", msg)
	}
	// The Issue-4b fallback '<unknown>' still renders in the main message.
	if !strings.Contains(msg, "<unknown>") {
		t.Errorf("pid fallback '<unknown>' must still render; got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_LockReleasedAfterRun — proves defer locker.Release() fires
// ---------------------------------------------------------------------------

func TestRunDefault_LockReleasedAfterRun(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: x")
	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")

	// Isolate lock dir to the test's temp tree (G7): isolateHome does NOT set these.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", "")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	code := exitcode.For(err)
	if code != exitcode.Success {
		t.Errorf("exitcode.For(err) = %d, want %d (Success)", code, exitcode.Success)
	}

	// The lock must have been released — a subsequent Acquire must succeed.
	_, lockErr := lock.Acquire(repo)
	if lockErr != nil {
		t.Errorf("lock.Acquire after run failed: %v (lock was not released)", lockErr)
	}
}
