package generate_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// TestCommitStaged_GenerationFreeze_HoldsForLiveStagedSentinel reproduces the user's exact report:
// files staged to the LIVE index WHILE the message is being generated (no hooks involved) must NOT
// be swept into the commit. Unlike hooks_freeze_test (which stages during a blocking pre-commit
// hook), this stages during the GENERATION window with deps.Hooks == nil.
func TestCommitStaged_GenerationFreeze_HoldsForLiveStagedSentinel(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "fileA.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add seed: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-q", "-m", "seed: initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit seed: %v %s", err, out)
	}

	// Stage a real change for the snapshot.
	if err := os.WriteFile(filepath.Join(dir, "fileA.txt"), []byte("a-modified\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}

	bin := stubtest.Build(t)
	// Slow stub: 800ms generation gives us a wide window to stage the sentinel mid-generation.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: generation-window freeze repro", SleepMS: 800})
	cfg := config.Defaults()                              // NoVerify=false but NO hooks configured → deps.Hooks == nil
	deps := generate.Deps{Git: git.New(dir), Manifest: m} // Hooks == nil

	done := make(chan struct{})
	var res generate.Result
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go func() {
		res, err = generate.CommitStaged(ctx, deps, cfg)
		close(done)
	}()

	// Wait ~300ms so we're solidly inside the 800ms generation window (after the snapshot at step 4,
	// during the generate→parse→dedupe loop). Then stage a sentinel to the LIVE index.
	time.Sleep(300 * time.Millisecond)
	if e := os.WriteFile(filepath.Join(dir, "sentinel.txt"), []byte("s\n"), 0o644); e != nil {
		t.Fatalf("write sentinel: %v", e)
	}
	if out, e := exec.Command("git", "-C", dir, "add", "sentinel.txt").CombinedOutput(); e != nil {
		t.Fatalf("stage sentinel mid-generation: %v %s", e, out)
	}

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("CommitStaged did not return")
	}
	if err != nil {
		t.Fatalf("CommitStaged err=%v", err)
	}

	// ASSERT (a): the commit's tree OMITS the sentinel.
	lsTree, _ := exec.Command("git", "-C", dir, "ls-tree", "-r", "--name-only", "HEAD").Output()
	t.Logf("HEAD tree:\n%s", lsTree)
	if strings.Contains(string(lsTree), "sentinel.txt") {
		t.Errorf("FREEZE VIOLATED: sentinel staged during generation swept into the commit:\n%s", lsTree)
	}
	if !strings.Contains(string(lsTree), "fileA.txt") {
		t.Errorf("expected fileA.txt in the commit tree, got:\n%s", lsTree)
	}
	// ASSERT (b): the LIVE index RETAINS the sentinel staged.
	diffCached, _ := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only").Output()
	if !strings.Contains(string(diffCached), "sentinel.txt") {
		t.Errorf("expected the sentinel to remain staged in the live index, got:\n%s", diffCached)
	}
	_ = res
}
