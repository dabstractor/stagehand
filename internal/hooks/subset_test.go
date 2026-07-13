package hooks

// subset_test.go exercises enforceSubset against REAL git output. It builds a repo, primes a
// throwaway index from a snapshot tree via git.ReadTreeInto (P1.M2.T1.S2), applies a scoped
// mutation via an INDEPENDENT oracle (exec.Command "git update-index" with GIT_INDEX_FILE env —
// mirroring P1.M2.T1.S2's test discipline), captures postTree via git.WriteTreeFrom, then asserts
// enforceSubset's verdict. This tests the SUBSET CHECK end-to-end; the scoped primitives have their
// own coverage in internal/git.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/git"
)

// --- minimal repo helpers (internal/git's are package-private; recreated here) ---

func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// repo-local identity so every later git op works without a global gitconfig.
	for _, args := range [][]string{
		{"config", "user.name", "Test"},
		{"config", "user.email", "test@example.com"},
	} {
		_ = execGitSilent(t, dir, args...)
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile(%s) failed: %v", name, err)
	}
}

func stageFile(t *testing.T, dir, name string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "add", name).CombinedOutput(); err != nil {
		t.Fatalf("git add %s failed: %v\n%s", name, err, out)
	}
}

func makeEmptyCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("makeEmptyCommit failed: %v\n%s", err, out)
	}
}

func writeTreeOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "write-tree").Output()
	if err != nil {
		t.Fatalf("git write-tree failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func execGitSilent(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// execGit mirrors internal/git's helper: runs git -C dir with a minimal identity env.
func execGit(t *testing.T, dir string, args ...string) string {
	return execGitSilent(t, dir, args...)
}

// --- scoped-mutation oracles (independent of the runner under test) ---

// scopedUpdateIndex runs `git -C repo update-index --add <path>` against the THROWAWAY index at
// tmpIndex (via GIT_INDEX_FILE env), NOT the live .git/index. It stages a (possibly new) file.
func scopedUpdateIndex(t *testing.T, repo, tmpIndex, path string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "update-index", "--add", path)
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scoped update-index --add %s: %v\n%s", path, err, out)
	}
}

// scopedRemoveCached runs `git -C repo rm --cached --quiet -- <path>` against the THROWAWAY index at
// tmpIndex (via GIT_INDEX_FILE env). It removes <path> from the index REGARDLESS of the working tree
// (unlike update-index --remove, which reconciles against the working tree and would re-keep a file
// still present on disk). The working-tree file is left untouched.
func scopedRemoveCached(t *testing.T, repo, tmpIndex, path string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rm", "--cached", "--quiet", "--", path)
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scoped rm --cached %s: %v\n%s", path, err, out)
	}
}

// --- keystone matrix ---

// primeSnapshot builds a repo with a.go + b.go staged, an initial commit, and returns the tree
// holding both files plus a fresh throwaway index file primed from that tree (via ReadTreeInto).
func primeSnapshot(t *testing.T) (repo, snapshotTree, tmpIndex string, g git.Git) {
	t.Helper()
	repo = t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package x\n\nfunc a() {}\n")
	writeFile(t, repo, "b.go", "package x\n\nfunc b() {}\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	makeEmptyCommit(t, repo, "init")
	snapshotTree = writeTreeOf(t, repo)

	tmpIndex = filepath.Join(t.TempDir(), "scoped.index")
	g = git.New(repo)
	if err := g.ReadTreeInto(context.Background(), snapshotTree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}
	return repo, snapshotTree, tmpIndex, g
}

// TestEnforceSubset_NoMutation: no scoped mutation ⇒ postTree == snapshotTree ⇒ nil.
func TestEnforceSubset_NoMutation(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	if postTree != snapshotTree {
		t.Errorf("postTree = %q, want snapshotTree %q (no mutation)", postTree, snapshotTree)
	}
	if err := enforceSubset(context.Background(), g, snapshotTree, postTree); err != nil {
		t.Errorf("enforceSubset (no mutation) err = %v, want nil", err)
	}
	_ = repo
}

// TestEnforceSubset_PermittedModify: a formatter reformats a.go (content change) ⇒ 'M' ⇒ nil.
func TestEnforceSubset_PermittedModify(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	writeFile(t, repo, "a.go", "package x\n\nfunc a() { /* reformatted */ }\n")
	scopedUpdateIndex(t, repo, tmpIndex, "a.go")

	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	if postTree == snapshotTree {
		t.Fatalf("postTree == snapshotTree, want different (a.go was modified)")
	}
	if err := enforceSubset(context.Background(), g, snapshotTree, postTree); err != nil {
		t.Errorf("enforceSubset (permitted M) err = %v, want nil", err)
	}
}

// TestEnforceSubset_PermittedDelete: the hook removes a.go (a snapshot path) ⇒ 'D' ⇒ nil.
func TestEnforceSubset_PermittedDelete(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	scopedRemoveCached(t, repo, tmpIndex, "a.go")

	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	if postTree == snapshotTree {
		t.Fatalf("postTree == snapshotTree, want different (a.go was removed)")
	}
	if err := enforceSubset(context.Background(), g, snapshotTree, postTree); err != nil {
		t.Errorf("enforceSubset (permitted D) err = %v, want nil", err)
	}
}

// TestEnforceSubset_ForbiddenAddedFile (THE KEYSTONE): the hook stages a NEW c.go ⇒ 'A' ⇒ hard error.
func TestEnforceSubset_ForbiddenAddedFile(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	writeFile(t, repo, "c.go", "package x\n\nfunc c() {}\n")
	scopedUpdateIndex(t, repo, tmpIndex, "c.go")

	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	err = enforceSubset(context.Background(), g, snapshotTree, postTree)
	if err == nil {
		t.Fatal("enforceSubset (forbidden A) err = nil, want non-nil")
	}
	if !errors.Is(err, ErrHookSweptConcurrentWork) {
		t.Fatalf("enforceSubset err = %v, want errors.Is(err, ErrHookSweptConcurrentWork)", err)
	}
	if !strings.Contains(err.Error(), "c.go") {
		t.Errorf("enforceSubset err = %q, want it to name c.go", err.Error())
	}
}

// TestEnforceSubset_ForbiddenRename: a rename (a.go→a2.go) shows as D a.go + A a2.go under no -M.
func TestEnforceSubset_ForbiddenRename(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	// write a2.go with a.go's content, then stage a2.go and remove a.go (scoped).
	aBody, rerr := os.ReadFile(filepath.Join(repo, "a.go"))
	if rerr != nil {
		t.Fatalf("read a.go: %v", rerr)
	}
	writeFile(t, repo, "a2.go", string(aBody))
	scopedUpdateIndex(t, repo, tmpIndex, "a2.go")
	scopedRemoveCached(t, repo, tmpIndex, "a.go")

	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	err = enforceSubset(context.Background(), g, snapshotTree, postTree)
	if err == nil {
		t.Fatal("enforceSubset (forbidden rename) err = nil, want non-nil")
	}
	if !errors.Is(err, ErrHookSweptConcurrentWork) {
		t.Fatalf("enforceSubset err = %v, want errors.Is(err, ErrHookSweptConcurrentWork)", err)
	}
	if !strings.Contains(err.Error(), "a2.go") {
		t.Errorf("enforceSubset err = %q, want it to name the rename destination a2.go", err.Error())
	}
}

// TestEnforceSubset_MultipleViolations: two new files added ⇒ error names BOTH (comma-joined).
func TestEnforceSubset_MultipleViolations(t *testing.T) {
	repo, snapshotTree, tmpIndex, g := primeSnapshot(t)
	writeFile(t, repo, "c.go", "package x\n")
	writeFile(t, repo, "d.go", "package x\n")
	scopedUpdateIndex(t, repo, tmpIndex, "c.go")
	scopedUpdateIndex(t, repo, tmpIndex, "d.go")

	postTree, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	err = enforceSubset(context.Background(), g, snapshotTree, postTree)
	if err == nil {
		t.Fatal("enforceSubset (multiple) err = nil, want non-nil")
	}
	if !errors.Is(err, ErrHookSweptConcurrentWork) {
		t.Fatalf("enforceSubset err = %v, want errors.Is(err, ErrHookSweptConcurrentWork)", err)
	}
	for _, want := range []string{"c.go", "d.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("enforceSubset err = %q, want it to name %s", err.Error(), want)
		}
	}
}

// TestEnforceSubset_DiffTreeFailure: a bad tree SHA ⇒ a wrapped NON-sentinel error (not a sweep).
func TestEnforceSubset_DiffTreeFailure(t *testing.T) {
	_, snapshotTree, _, g := primeSnapshot(t)
	// An all-zero "tree" is not a valid tree object → git diff-tree fails with exit 128.
	err := enforceSubset(context.Background(), g, "0000000000000000000000000000000000000000", snapshotTree)
	if err == nil {
		t.Fatal("enforceSubset (bad tree) err = nil, want non-nil")
	}
	if errors.Is(err, ErrHookSweptConcurrentWork) {
		t.Errorf("enforceSubset (bad tree) err = %v, want it NOT to be ErrHookSweptConcurrentWork (a git failure, not a sweep)", err)
	}
}
