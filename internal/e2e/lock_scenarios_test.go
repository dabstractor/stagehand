//go:build e2e

// lock_scenarios_test.go is the PRD §20.5 cross-process regression net for the FR52 per-repo run lock
// (PRD §18.5). It exercises the LANDED S2 contention wiring (internal/cmd/default_action.go:
// lock.Acquire + handleLockContention) against REAL stagecoach subprocesses on REAL temp git repos —
// the layer unit tests cannot reach (real flock across real processes). Reuses the harness primitives
// (buildStagecoach/newRepo/runStagecoach/waitForMarker/writeStubConfig/stubEnv) and the stub agent's
// STAGECOACH_STUB_MARKER + STAGECOACH_STUB_SLEEP_MS blocking pattern (NO new binary). Test-only.
package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestE2ELockContention exercises every PRD §18.5 contention behavior end-to-end.
func TestE2ELockContention(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "") // shared; each subtest makes its own repo

	t.Run("A_BusyRefusal_GenuineSecondBatch", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n") // first batch — covered by #1's snapshot
		stageFile(t, repo, "a.txt")

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "3000", // #1 holds the lock during generation
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // #1 holds lock + published snapshot=tree(a.txt)

		// GENUINE SECOND BATCH: stage b.txt AFTER #1 snapshotted → not in #1's snapshot → tree differs → Busy.
		writeFile(t, repo, "b.txt", "b\n")
		stageFile(t, repo, "b.txt")

		contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: b"})
		res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
		if res2.ExitCode != 5 {
			t.Fatalf("contender exit = %d, want 5 (Busy); stderr:\n%s", res2.ExitCode, res2.Stderr)
		}
		if !strings.Contains(res2.Stderr, "already in progress") {
			t.Errorf("stderr missing 'already in progress'; got:\n%s", res2.Stderr)
		}
		if !strings.Contains(res2.Stderr, repo) {
			t.Errorf("stderr missing repo path %q (holder must be named); got:\n%s", repo, res2.Stderr)
		}

		res := <-resCh // #1 finishes after its 3s sleep
		if res.ExitCode != 0 {
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if n := commitCount(t, repo); n != 2 {
			t.Errorf("commit count = %d, want 2 (seed + #1's a.txt; b.txt stays staged)", n)
		}
		if msg := runGit(t, repo, "log", "-1", "--format=%s"); msg != "feat: a" {
			t.Errorf("HEAD subject = %q, want 'feat: a'", msg)
		}
	})

	t.Run("B_NoOpFastPath_AccidentalDoubleRun", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "3000",
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // #1 snapshot = tree(a.txt)

		// #2 stages NOTHING NEW → its write-tree (tree(a.txt)) == #1's snapshot → no-op fast path → exit 0.
		contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: a"})
		res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
		if res2.ExitCode != 0 {
			t.Fatalf("contender exit = %d, want 0 (no-op fast path); stderr:\n%s", res2.ExitCode, res2.Stderr)
		}
		if !strings.Contains(res2.Stderr, "nothing to do") {
			t.Errorf("stderr missing 'nothing to do'; got:\n%s", res2.Stderr)
		}

		res := <-resCh
		if res.ExitCode != 0 {
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if n := commitCount(t, repo); n != 2 {
			t.Errorf("commit count = %d, want 2 (only #1 committed; #2 was a no-op)", n)
		}
	})

	t.Run("C_NoStaleLock_AfterExit", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: a"})

		// #1 runs to completion and its process exits → flock auto-released (no stale lock).
		res1 := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
		if res1.ExitCode != 0 {
			t.Fatalf("#1 exit = %d, want 0; stderr:\n%s", res1.ExitCode, res1.Stderr)
		}

		// After #1's exit, #2 must acquire without contention (flock released). Use a DISTINCT
		// message (feat: b) so generation does not hit the duplicate-message guard from #1's feat: a
		// — the property under test is "no stale lock", which requires #2 to actually run to exit 0.
		writeFile(t, repo, "b.txt", "b\n")
		stageFile(t, repo, "b.txt")
		env2 := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: b"})
		res2 := runStagecoach(t, bin, repo, cfg, env2, "--provider", "stub")
		if res2.ExitCode == 5 {
			t.Fatalf("#2 exited Busy (5) — stale lock! flock must auto-release on #1's exit; stderr:\n%s", res2.Stderr)
		}
		if res2.ExitCode != 0 {
			t.Fatalf("#2 exit = %d, want 0; stderr:\n%s", res2.ExitCode, res2.Stderr)
		}
		if n := commitCount(t, repo); n != 3 {
			t.Errorf("commit count = %d, want 3 (seed + #1 + #2)", n)
		}
	})

	t.Run("D_ReadOnlyBypass", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "5000", // hold the lock while we poke the read-only commands
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // #1 holds the lock

		baseEnv := stubEnv(nil)
		for _, args := range [][]string{
			{"providers", "list"},
			{"config", "path"},
			{"models", "--help"},
		} {
			res := runStagecoach(t, bin, repo, cfg, baseEnv, args...)
			if res.ExitCode == 5 {
				t.Errorf("%v exited Busy (5) — read-only subcommands must bypass the lock; stderr:\n%s", args, res.Stderr)
			}
			if strings.Contains(res.Stderr, "already in progress") {
				t.Errorf("%v hit the lock; stderr:\n%s", args, res.Stderr)
			}
		}

		if res := <-resCh; res.ExitCode != 0 { // drain #1 (lets its 5s sleep finish)
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
	})

	t.Run("E_DryRunAcquiresLock", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "3000",
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // #1 snapshot = tree(a.txt)

		// Dry-run contender stages an EXTRA file → tree differs → Busy (proves dry-run goes through runDefault
		// and acquires the lock; if it bypassed, it would print "no commit created" and exit 0).
		writeFile(t, repo, "b.txt", "b\n")
		stageFile(t, repo, "b.txt")
		contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: b"})
		res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--dry-run", "--provider", "stub")
		if res2.ExitCode != 5 {
			t.Fatalf("dry-run contender exit = %d, want 5 (Busy — dry-run acquires the lock); stderr:\n%s", res2.ExitCode, res2.Stderr)
		}
		if !strings.Contains(res2.Stderr, "already in progress") {
			t.Errorf("stderr missing 'already in progress'; got:\n%s", res2.Stderr)
		}
		if strings.Contains(res2.Stderr, "no commit created") {
			t.Errorf("dry-run proceeded past the lock (printed 'no commit created'); stderr:\n%s", res2.Stderr)
		}

		if res := <-resCh; res.ExitCode != 0 {
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
	})

	t.Run("F_DecomposeAccidentalDoubleRun_Busy", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "feature.txt", "new work\n") // ONE untracked file, NOT staged → decompose activates

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: add feature",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "4000",
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // holder: lock held, T_start published, message-gen sleep

		// Contender: same dirty tree, still nothing staged → handleLockContention:
		//   WriteTree() = baseTree ≠ snap(T_start) → Busy(5). "nothing to do" must NOT appear.
		contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add feature"})
		res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
		if res2.ExitCode != 5 {
			t.Fatalf("contender exit = %d, want 5 (Busy) — decompose no-op fast path is structurally impossible; stderr:\n%s", res2.ExitCode, res2.Stderr)
		}
		if !strings.Contains(res2.Stderr, "already in progress") {
			t.Errorf("stderr missing busy message; got:\n%s", res2.Stderr)
		}
		if strings.Contains(res2.Stderr, "nothing to do") {
			t.Errorf("decompose path must NOT hit the no-op fast path; got:\n%s", res2.Stderr)
		}

		res := <-resCh // drain holder
		if res.ExitCode != 0 {
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
	})
}
