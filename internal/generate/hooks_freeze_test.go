// hooks_freeze_test.go is the headline freeze-safety invariant test (PRD §20.2/§20.5) + the hook-
// abort → rescue test (PRD §9.25 FR-V7), folded into the CommitStaged wiring task (P1.M3.T2.S1).
//
// This is an EXTERNAL test (package generate_test) so it can import internal/hooks for
// hooks.DefaultRunner — a white-box package generate test CANNOT import internal/hooks (cycle:
// hooks imports generate for RescueError). It defines its own minimal initTempRepo (the white-box
// generate_test.go initRepo is in package generate and so unimportable here).
//
// TestCommitStaged_PreCommitFreeze_HoldsForLiveStagedSentinel — the §20.2/§20.5 invariant: a
// sentinel staged to the LIVE index AFTER write-tree (during a blocking pre-commit hook) is NOT
// swept into the commit's tree (pre-commit runs against a throwaway index primed from the frozen
// snapshot), and the live index RETAINS it staged (the core stage-while-generating guarantee).
//
// TestCommitStaged_PreCommitAbort_IsRescue — FR-V7: a non-zero pre-commit → *generate.RescueError
// (byte-identical to a generation failure) + HEAD unchanged + the live index idempotent.
package generate_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/hooks"
	"github.com/dabstractor/stagecoach/internal/stubtest"
)

// initTempRepo creates a temp git repo with repo-local identity + a seed commit, returns its dir.
// (Own copy — the white-box generate_test.go initRepo is in package generate, unimportable here.)
func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, c := range [][]string{
		{"git", "init", "-q", dir},
		{"git", "-C", dir, "config", "user.email", "t@e.com"},
		{"git", "-C", dir, "config", "user.name", "T"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	seed := filepath.Join(dir, "fileA.txt")
	if err := os.WriteFile(seed, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add seed: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-q", "-m", "seed: initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit seed: %v %s", err, out)
	}
	return dir
}

// TestCommitStaged_PreCommitFreeze_HoldsForLiveStagedSentinel is the headline §20.2/§20.5 freeze
// invariant: a sentinel staged to the LIVE index AFTER write-tree (during a blocking pre-commit
// hook) is NOT swept into the commit (pre-commit runs against a throwaway index primed from the
// frozen snapshot — the live .git/index is never touched), and the live index RETAINS it staged.
//
// Mechanics: a blocking pre-commit hook (signals READY, waits for PROCEED) opens the only seam to
// stage a sentinel "after write-tree" (CommitStaged takes the snapshot internally at step 4). The
// stub Manifest makes generation instant, so CommitStaged deterministically blocks inside
// RunCommitHooks. A concurrent `git add sentinel.txt` (a separate process, no GIT_INDEX_FILE) hits
// the LIVE .git/index — a DIFFERENT index than the hook's GIT_INDEX_FILE=<throwaway> — exactly the
// concurrent-user scenario FR-V3 protects against.
func TestCommitStaged_PreCommitFreeze_HoldsForLiveStagedSentinel(t *testing.T) {
	repo := initTempRepo(t)

	// Stage a real change for the snapshot (so there is something to commit).
	if err := os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-modified\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}

	// A blocking pre-commit hook: touches READY, then spins until PROCEED exists, then exits 0
	// (no mutation to the throwaway index).
	tmp := t.TempDir()
	ready := filepath.Join(tmp, "ready")
	proceed := filepath.Join(tmp, "proceed")
	hookBody := "#!/bin/sh\ntouch " + ready + "\nwhile [ ! -f " + proceed + " ]; do sleep 0.02; done\nexit 0\n"
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookBody), 0o755); err != nil {
		t.Fatalf("write pre-commit hook: %v", err)
	}

	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: unique freeze sentinel msg"})
	cfg := config.Defaults() // NoVerify=false → hooks run
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}}

	// Run CommitStaged in a goroutine. The stub agent is instant → generation completes →
	// CommitStaged blocks inside RunCommitHooks (the hook is spinning on PROCEED).
	done := make(chan struct{})
	var res generate.Result
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		res, err = generate.CommitStaged(ctx, deps, cfg)
		close(done)
	}()

	// Poll for READY (the hook is running, scoped to the throwaway index).
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, e := os.Stat(ready); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pre-commit hook did not start (no ready file after 5s)")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// NOW stage the sentinel to the LIVE index (a SEPARATE process, no GIT_INDEX_FILE → writes to
	// .git/index, the live one — NOT the hook's throwaway).
	if err := os.WriteFile(filepath.Join(repo, "sentinel.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if out, e := exec.Command("git", "-C", repo, "add", "sentinel.txt").CombinedOutput(); e != nil {
		t.Fatalf("stage sentinel: %v %s", e, out)
	}

	// Release the hook → it exits 0 (no mutation) → RunCommitHooks returns → CommitTree → UpdateRefCAS.
	if err := os.WriteFile(proceed, []byte{}, 0o644); err != nil {
		t.Fatalf("touch proceed: %v", err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("CommitStaged did not return after releasing the hook")
	}

	if err != nil {
		t.Fatalf("CommitStaged err=%v (hook exited 0 — expected success)", err)
	}

	// ASSERT (a): the commit's tree OMITS the sentinel (the scoped throwaway index excluded it).
	lsTree, _ := exec.Command("git", "-C", repo, "ls-tree", "-r", "--name-only", "HEAD").Output()
	if strings.Contains(string(lsTree), "sentinel.txt") {
		t.Errorf("FREEZE VIOLATED: sentinel swept into the commit:\n%s", lsTree)
	}
	if !strings.Contains(string(lsTree), "fileA.txt") {
		t.Errorf("expected fileA.txt in the commit tree, got:\n%s", lsTree)
	}
	// ASSERT (b): the LIVE index RETAINS the sentinel staged.
	diffCached, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if !strings.Contains(string(diffCached), "sentinel.txt") {
		t.Errorf("expected the sentinel to remain staged in the live index, got:\n%s", diffCached)
	}
	_ = res
}

// TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort is the Issue-4 git-parity guard: a
// commit-msg (or prepare-commit-msg) hook that empties the message file must NOT produce a commit. git
// aborts "Aborting commit due to empty commit message." (exit 1); stagecoach returns the BARE
// generate.ErrEmptyMessage (exit 1, NOT a rescue) and creates NO commit (HEAD unchanged). (PRD §9.25
// FR-V2 git parity.) Mirrors TestCommitStaged_PreCommitAbort_IsRescue's idiom but asserts a bare
// ErrEmptyMessage (not *RescueError) — a hook that empties the file is a rejection, not a hook failure.
func TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort(t *testing.T) {
	repo := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-mod\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}

	// A commit-msg hook that empties the message file (a common rejection / force-re-edit pattern).
	// exit 0 ⇒ not a hook failure (no *RescueError); the guard catches the EMPTY result.
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "commit-msg"),
		[]byte("#!/bin/sh\n> \"$1\"\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write commit-msg hook: %v", err)
	}

	headBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()

	bin := stubtest.Build(t)
	// NON-empty Out ⇒ generation succeeds; the hook then empties the message file.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: non-empty generated message"})
	cfg := config.Defaults()
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}}

	_, err := generate.CommitStaged(context.Background(), deps, cfg)
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1 — NOT a rescue), got %T: %v", err, err)
	}

	// NO commit created (HEAD unchanged — the abort returned before CommitTree).
	headAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD moved on empty-message abort: %s → %s (a commit was created)", headBefore, headAfter)
	}
}

// TestCommitStaged_PreCommitAbort_IsRescue is FR-V7: a non-zero pre-commit → CommitStaged returns
// a *generate.RescueError (byte-identical to a generation failure → exit 3), HEAD is unchanged (no
// update-ref ran), and the live index is idempotent (§20.2 property 1).
func TestCommitStaged_PreCommitAbort_IsRescue(t *testing.T) {
	repo := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-mod\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}

	// A pre-commit hook that exits non-zero (a lint failure).
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"),
		[]byte("#!/bin/sh\necho 'lint failed' 1>&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write pre-commit hook: %v", err)
	}

	headBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	stagedBefore, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()

	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: unique rescue abort msg"})
	cfg := config.Defaults()
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}}

	_, err := generate.CommitStaged(context.Background(), deps, cfg)
	if err == nil {
		t.Fatal("expected a hook-abort error, got nil")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("expected *generate.RescueError (FR-V7), got %T: %v", err, err)
	}

	// HEAD unchanged (no update-ref ran — the abort returned before CommitTree).
	headAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD moved on hook abort: %s → %s", headBefore, headAfter)
	}
	// Index idempotent (§20.2 property 1: the live index is never touched).
	stagedAfter, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if string(stagedBefore) != string(stagedAfter) {
		t.Errorf("live index changed on hook abort:\nbefore: %s\nafter:  %s", stagedBefore, stagedAfter)
	}
}
