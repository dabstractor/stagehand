package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/lock"
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
