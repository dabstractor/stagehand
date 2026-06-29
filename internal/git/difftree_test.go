package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// ---- Fixture helpers (dt-prefixed to avoid collision with S4's writeFile/stageFile/headSHA and S5's cas* helpers) ----

// dtCommit creates a commit in dir with msg using staged changes (NOT --allow-empty).
// REUSES S2's minGitEnv(); mirrors makeEmptyCommit's env pattern minus --allow-empty.
func dtCommit(t *testing.T, dir, msg string) {
	t.Helper()
	env := append(minGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd := exec.Command("git", "-C", dir, "commit", "-m", msg)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dtCommit failed: %v\n%s", err, out)
	}
}

// dtRemove stages a deletion of name in dir via git rm.
func dtRemove(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rm", "-q", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dtRemove %s failed: %v\n%s", name, err, out)
	}
}

// ---- DiffTree integration tests (real repo) ----

func TestDiffTree_ChildCommit(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// root commit with two files
	writeFile(t, repo, "keep.txt", "v1\n")
	stageFile(t, repo, "keep.txt")
	writeFile(t, repo, "gone.txt", "v1\n")
	stageFile(t, repo, "gone.txt")
	dtCommit(t, repo, "root")

	// child commit: modify keep.txt, delete gone.txt, add new.txt
	writeFile(t, repo, "keep.txt", "v2\n")
	stageFile(t, repo, "keep.txt") // M
	dtRemove(t, repo, "gone.txt")  // D
	writeFile(t, repo, "new.txt", "v1\n")
	stageFile(t, repo, "new.txt") // A
	dtCommit(t, repo, "child")

	child := headSHA(t, repo)
	g := New(repo)
	changes, err := g.DiffTree(context.Background(), child, false) // isRoot=false (has parent)
	if err != nil {
		t.Fatalf("DiffTree child: err = %v, want nil", err)
	}

	got := map[string]bool{}
	for _, c := range changes {
		got[c.Status+"\t"+c.Path] = true
		if c.SrcPath != "" {
			t.Fatalf("DiffTree child: unexpected SrcPath %q for %s\t%s (2-field expected)", c.SrcPath, c.Status, c.Path)
		}
	}
	if len(changes) != 3 {
		t.Fatalf("DiffTree child: len(changes) = %d, want 3", len(changes))
	}
	for _, key := range []string{"M\tkeep.txt", "D\tgone.txt", "A\tnew.txt"} {
		if !got[key] {
			t.Fatalf("DiffTree child: missing expected change %q; got %v", key, got)
		}
	}
}

func TestDiffTree_RootCommit_WithRootFlag(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.txt", "x\n")
	stageFile(t, repo, "a.txt")
	writeFile(t, repo, "b.txt", "y\n")
	stageFile(t, repo, "b.txt")
	dtCommit(t, repo, "root") // creates the ROOT commit (unborn → root)

	root := headSHA(t, repo)
	g := New(repo)
	changes, err := g.DiffTree(context.Background(), root, true) // isRoot=true → --root appended
	if err != nil {
		t.Fatalf("DiffTree root+--root: err = %v, want nil", err)
	}
	if len(changes) != 2 {
		t.Fatalf("DiffTree root+--root: len(changes) = %d, want 2", len(changes))
	}

	got := map[string]bool{}
	for _, c := range changes {
		if c.Status != "A" {
			t.Fatalf("DiffTree root+--root: Status = %q, want \"A\"", c.Status)
		}
		got[c.Path] = true
	}
	for _, p := range []string{"a.txt", "b.txt"} {
		if !got[p] {
			t.Fatalf("DiffTree root+--root: missing path %q", p)
		}
	}
}

func TestDiffTree_RootCommit_WithoutRootFlag(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.txt", "x\n")
	stageFile(t, repo, "a.txt")
	dtCommit(t, repo, "root")

	root := headSHA(t, repo)
	g := New(repo)
	changes, err := g.DiffTree(context.Background(), root, false) // isRoot=false → NO --root
	if err != nil {
		t.Fatalf("DiffTree root without --root: err = %v, want nil (empty output is not an error, G5)", err)
	}
	if len(changes) != 0 {
		t.Fatalf("DiffTree root without --root: len(changes) = %d, want 0 (core isRoot trap G1)", len(changes))
	}
}

func TestDiffTree_NoChanges(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.txt", "x\n")
	stageFile(t, repo, "a.txt")
	dtCommit(t, repo, "first")
	first := headSHA(t, repo)

	// a second commit that changes nothing (re-stage identical content via --allow-empty)
	makeEmptyCommit(t, repo, "noop") // REUSE S2 helper
	noop := headSHA(t, repo)

	g := New(repo)

	// also verify the first commit has changes (sanity)
	ch1, err := g.DiffTree(context.Background(), first, true)
	if err != nil {
		t.Fatalf("DiffTree noChanges: first commit err = %v, want nil", err)
	}
	if len(ch1) != 1 || ch1[0].Status != "A" || ch1[0].Path != "a.txt" {
		t.Fatalf("DiffTree noChanges: first commit sanity check failed: %+v", ch1)
	}

	changes, err := g.DiffTree(context.Background(), noop, false)
	if err != nil {
		t.Fatalf("DiffTree noChanges: err = %v, want nil", err)
	}
	if len(changes) != 0 {
		t.Fatalf("DiffTree noChanges: len(changes) = %d, want 0 (empty output, exit 0, G5)", len(changes))
	}
}

func TestDiffTree_BadSHA(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	changes, err := g.DiffTree(context.Background(), "0000000000000000000000000000000000000000", false)
	if err == nil {
		t.Fatal("DiffTree bad SHA: err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "git diff-tree: failed") {
		t.Fatalf("DiffTree bad SHA: err = %v, want it to contain 'git diff-tree: failed'", err)
	}
	if !strings.Contains(err.Error(), "(exit 128)") {
		t.Fatalf("DiffTree bad SHA: err = %v, want it to contain '(exit 128)'", err)
	}
	if changes != nil {
		t.Fatalf("DiffTree bad SHA: changes = %+v, want nil", changes)
	}
}

func TestDiffTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir()) // dir need not be a repo
	changes, err := g.DiffTree(context.Background(), "sha", false)
	if err == nil {
		t.Fatal("DiffTree git-missing: err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("DiffTree git-missing: err = %v, want it to contain 'git binary not found'", err)
	}
	if changes != nil {
		t.Fatalf("DiffTree git-missing: changes = %+v, want nil", changes)
	}
}

func TestDiffTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	changes, err := g.DiffTree(ctx, "sha", false)
	if err == nil {
		t.Fatal("DiffTree ctx-cancelled: err = nil, want non-nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DiffTree ctx-cancelled: err = %v, want context.Canceled", err)
	}
	if changes != nil {
		t.Fatalf("DiffTree ctx-cancelled: changes = %+v, want nil", changes)
	}
}

// ---- parseDiffTree unit tests (direct, no git needed) ----

func TestParseDiffTree_Formats(t *testing.T) {
	in := "A\tadded.txt\nM\tmod.txt\nD\tdel.txt\nR100\told.txt\tnew.txt\nC90\tsrc.txt\tdst.txt\n"
	got := parseDiffTree(in)
	if len(got) != 5 {
		t.Fatalf("parseDiffTree formats: len = %d, want 5", len(got))
	}

	expected := []FileChange{
		{Status: "A", Path: "added.txt"},
		{Status: "M", Path: "mod.txt"},
		{Status: "D", Path: "del.txt"},
		{Status: "R100", SrcPath: "old.txt", Path: "new.txt"}, // 3-field rename
		{Status: "C90", SrcPath: "src.txt", Path: "dst.txt"},  // 3-field copy
	}
	for i, want := range expected {
		if got[i] != want {
			t.Fatalf("parseDiffTree formats: [%d] = %+v, want %+v", i, got[i], want)
		}
	}
}

func TestParseDiffTree_Empty(t *testing.T) {
	if len(parseDiffTree("")) != 0 {
		t.Fatal("parseDiffTree(\"\"): len != 0, want nil/empty slice (G15)")
	}
	if len(parseDiffTree("\n  \n")) != 0 {
		t.Fatal("parseDiffTree(\"\\n  \\n\"): len != 0, want nil/empty slice (whitespace-only)")
	}
}
