package git

// White-box tests for the four plumbing primitives in plumbing.go. They are
// package git (NOT git_test) so they can call the unexported (g *Git).run
// seam, read the unexported g.dir, and compose the S2 harness helpers
// (newTempRepo/seedCommits/writeFileStage in gittestutil_test.go) which live
// as package-git _test.go files in this SAME directory. They drive the REAL
// host git binary (git 2.54.0, PRD §20.1 layer 2) — no mocks of git, no
// go-git — with one behavior per Test* function. Each test asserts the §18.1
// safety invariants where they apply (atomic HEAD after a CAS failure,
// idempotent index across WriteTree, commit-tree touching no ref).

import (
	"errors"
	"strings"
	"testing"
)

// runOut runs a git command expected to succeed and returns its trimmed
// stdout, failing the test on any error. It is the output-returning
// counterpart to the S2 harness mustRun (which discards stdout), kept local
// to these plumbing tests to avoid touching the shipped harness file.
func runOut(t *testing.T, g *Git, args ...string) string {
	t.Helper()
	out, err := g.run(args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

// TestRevParseHEAD_UnbornRepo proves an unborn (rootless) repo is reported as
// hasParent=false with a NIL error — the v1 root-commit path, NOT a failure
// (contract: "treat non-zero/empty as ok=false, not error"). git reports this
// as exit 128 with "unknown revision"; RevParseHEAD must swallow that into
// hasParent=false (FR39).
func TestRevParseHEAD_UnbornRepo(t *testing.T) {
	g := newTempRepo(t) // unborn: no commits
	sha, has, err := g.RevParseHEAD()
	if err != nil {
		t.Fatalf("RevParseHEAD on unborn repo returned error %v; want nil", err)
	}
	if has {
		t.Error("hasParent = true; want false on an unborn repo")
	}
	if sha != "" {
		t.Errorf("sha = %q; want empty string on an unborn repo", sha)
	}
}

// TestRevParseHEAD_RepoWithCommits proves a repo with history returns
// hasParent=true, no error, and the full 40-hex HEAD SHA, cross-checked
// against a raw `rev-parse HEAD`.
func TestRevParseHEAD_RepoWithCommits(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"a", "b"})
	sha, has, err := g.RevParseHEAD()
	if err != nil {
		t.Fatalf("RevParseHEAD returned error %v; want nil", err)
	}
	if !has {
		t.Error("hasParent = false; want true in a repo with commits")
	}
	if len(sha) != 40 {
		t.Errorf("sha length = %d; want 40 (full SHA), sha=%q", len(sha), sha)
	}
	if want := runOut(t, g, "rev-parse", "HEAD"); sha != want {
		t.Errorf("RevParseHEAD = %q; raw rev-parse HEAD = %q", sha, want)
	}
}

// TestWriteTree_EmptyIndex proves write-tree on an empty index returns the
// well-known empty-tree SHA, read-only w.r.t. HEAD/index (PRD §13.2 step 1).
func TestWriteTree_EmptyIndex(t *testing.T) {
	g := newTempRepo(t) // empty index, no staged content
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree on empty index returned error %v; want nil", err)
	}
	if want := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"; tree != want {
		t.Errorf("WriteTree empty index = %q; want the empty-tree SHA %q", tree, want)
	}
}

// TestWriteTree_StagedContent proves write-tree returns a 40-hex tree SHA for
// staged content and, critically, does NOT mutate the index (§18.1 idempotent
// index): `git diff --cached --name-only` is byte-for-byte identical before
// and after.
func TestWriteTree_StagedContent(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"a"})
	writeFileStage(t, g, "new.txt", "x\n")

	before := runOut(t, g, "diff", "--cached", "--name-only")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	if len(tree) != 40 {
		t.Errorf("tree length = %d; want 40, tree=%q", len(tree), tree)
	}
	after := runOut(t, g, "diff", "--cached", "--name-only")
	if before != after {
		t.Errorf("index changed across WriteTree (§18.1 idempotent index):\nbefore=%q\nafter =%q", before, after)
	}
}

// TestWriteTree_StagedConflict proves an unresolved merge conflict in the
// index makes WriteTree return a CLEAR error whose message contains
// "conflict" (FR8 clarity), aborting BEFORE any generation. git 2.54.0 never
// emits FR8's literal phrase — it reports "<path>: unmerged (...)" + "error
// building trees" — so the conflict is constructed with a REAL divergent
// `git merge` that leaves `UU <path>` unmerged entries in the index.
func TestWriteTree_StagedConflict(t *testing.T) {
	g := newTempRepo(t)

	// A tracked file both branches will edit on the SAME line.
	writeFileStage(t, g, "shared.txt", "base\n")
	mustRun(t, g, "commit", "-q", "-m", "base")
	branch := runOut(t, g, "symbolic-ref", "--short", "HEAD") // default branch name
	mustRun(t, g, "branch", "side")                           // side at base

	// Ours: on the default branch, change the shared line.
	writeFileStage(t, g, "shared.txt", "ours\n")
	mustRun(t, g, "commit", "-q", "-m", "ours")

	// Theirs: on side, change the SAME line divergently.
	mustRun(t, g, "checkout", "-q", "side")
	writeFileStage(t, g, "shared.txt", "theirs\n")
	mustRun(t, g, "commit", "-q", "-m", "theirs")

	// Merge side into the default branch → content CONFLICT (exit 1 is fine).
	mustRun(t, g, "checkout", "-q", branch)
	if _, mergeErr := g.run("merge", "side"); mergeErr == nil {
		t.Fatal("git merge unexpectedly succeeded; want a content conflict (non-zero exit)")
	}

	// The index now holds UU shared.txt → WriteTree must abort clearly (FR8).
	_, err := g.WriteTree()
	if err == nil {
		t.Fatal("WriteTree returned nil error on a conflicted index; want a conflict error")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Errorf("WriteTree conflict error = %q; want it to contain %q", err.Error(), "conflict")
	}
}

// TestCommitTree_RootNoParent proves CommitTree("", msg, tree) creates a root
// commit (no -p, FR39) returning a 40-hex SHA and, crucially, touches NO ref:
// HEAD is STILL unborn afterward (PRD §13.2 step 2 — only update-ref advances
// HEAD).
func TestCommitTree_RootNoParent(t *testing.T) {
	g := newTempRepo(t)
	writeFileStage(t, g, "a.txt", "a\n")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	root, err := g.CommitTree("", "root msg", tree)
	if err != nil {
		t.Fatalf("CommitTree root returned error %v; want nil", err)
	}
	if len(root) != 40 {
		t.Errorf("root commit length = %d; want 40, root=%q", len(root), root)
	}
	// commit-tree writes a (dangling) commit object but advances NO ref: HEAD
	// must STILL be unborn until update-ref runs.
	if _, has, _ := g.RevParseHEAD(); has {
		t.Error("HEAD advanced after CommitTree; commit-tree must not touch any ref (PRD §13.2)")
	}
}

// TestCommitTree_WithParent proves CommitTree(parent, msg, tree) creates a
// child commit (-p parent) returning a 40-hex SHA distinct from the parent.
func TestCommitTree_WithParent(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"first"})
	parent, has, err := g.RevParseHEAD()
	if err != nil || !has {
		t.Fatalf("setup RevParseHEAD = (%q,%v,%v); want a parent SHA", parent, has, err)
	}
	writeFileStage(t, g, "b.txt", "b\n")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	child, err := g.CommitTree(parent, "second", tree)
	if err != nil {
		t.Fatalf("CommitTree child returned error %v; want nil", err)
	}
	if len(child) != 40 {
		t.Errorf("child commit length = %d; want 40, child=%q", len(child), child)
	}
	if child == parent {
		t.Error("child SHA == parent SHA; want a distinct child commit")
	}
}

// TestUpdateRefCAS_RootCommit proves the 1-arg no-expected form (expected=="")
// advances HEAD on an unborn repo to the root commit (decisions.md §7: this
// form is ONLY legal for the root commit). After it, RevParseHEAD reports
// hasParent=true and the root SHA.
func TestUpdateRefCAS_RootCommit(t *testing.T) {
	g := newTempRepo(t)
	writeFileStage(t, g, "a.txt", "a\n")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	root, err := g.CommitTree("", "root", tree)
	if err != nil {
		t.Fatalf("CommitTree root returned error %v; want nil", err)
	}
	if err := g.UpdateRefCAS("HEAD", root, ""); err != nil {
		t.Fatalf("UpdateRefCAS root returned error %v; want nil", err)
	}
	sha, has, err := g.RevParseHEAD()
	if err != nil {
		t.Fatalf("RevParseHEAD after root CAS returned error %v; want nil", err)
	}
	if !has {
		t.Fatal("hasParent = false after root CAS; want true (HEAD should have advanced)")
	}
	if sha != root {
		t.Errorf("HEAD = %q; want the root SHA %q", sha, root)
	}
}

// TestUpdateRefCAS_SucceedsWhenHEADUnchanged proves the 3-arg CAS form
// (expected=parent) succeeds when HEAD is unchanged since the snapshot,
// advancing HEAD to the new commit (FR40).
func TestUpdateRefCAS_SucceedsWhenHEADUnchanged(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"first"})
	parent, has, err := g.RevParseHEAD()
	if err != nil || !has {
		t.Fatalf("setup RevParseHEAD = (%q,%v,%v); want a parent SHA", parent, has, err)
	}
	writeFileStage(t, g, "b.txt", "b\n")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	newSHA, err := g.CommitTree(parent, "second", tree)
	if err != nil {
		t.Fatalf("CommitTree returned error %v; want nil", err)
	}
	if err := g.UpdateRefCAS("HEAD", newSHA, parent); err != nil {
		t.Fatalf("UpdateRefCAS returned error %v; want nil when HEAD is unchanged", err)
	}
	sha, _, err := g.RevParseHEAD()
	if err != nil {
		t.Fatalf("RevParseHEAD after CAS returned error %v; want nil", err)
	}
	if sha != newSHA {
		t.Errorf("HEAD = %q; want the new SHA %q", sha, newSHA)
	}
}

// TestUpdateRefCAS_FailsWhenHEADMoved proves the §18.1 atomic invariant: when
// HEAD moved concurrently, the CAS with a STALE expected must FAIL (FR41,
// never --force), and HEAD is byte-for-byte UNCHANGED afterward (still the
// post-move value, NOT newSHA). The failure is a routable typed *ExitError
// with Code 128.
func TestUpdateRefCAS_FailsWhenHEADMoved(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"first"})
	parent, has, err := g.RevParseHEAD()
	if err != nil || !has {
		t.Fatalf("setup RevParseHEAD = (%q,%v,%v); want a parent SHA", parent, has, err)
	}

	// Snapshot: stage new content, build a candidate commit (HEAD not touched).
	writeFileStage(t, g, "b.txt", "b\n")
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree returned error %v; want nil", err)
	}
	newSHA, err := g.CommitTree(parent, "candidate", tree)
	if err != nil {
		t.Fatalf("CommitTree returned error %v; want nil", err)
	}

	// CONCURRENTLY, HEAD advances past parent (another commit landed). The
	// staged b.txt rides along in that commit; only HEAD's movement matters.
	seedCommits(t, g, []string{"concurrent"})
	postMove, has, err := g.RevParseHEAD()
	if err != nil || !has {
		t.Fatalf("post-move RevParseHEAD = (%q,%v,%v); want a SHA", postMove, has, err)
	}
	if postMove == parent {
		t.Fatal("setup invariant failed: HEAD did not move off parent")
	}

	// CAS with the STALE parent must FAIL (HEAD is at postMove, not parent).
	casErr := g.UpdateRefCAS("HEAD", newSHA, parent)
	if casErr == nil {
		t.Fatal("UpdateRefCAS succeeded with stale expected; want a CAS failure (FR41)")
	}

	// §18.1 atomic invariant: HEAD is byte-for-byte UNCHANGED — still
	// postMove, NOT newSHA.
	sha, has, err := g.RevParseHEAD()
	if err != nil {
		t.Fatalf("RevParseHEAD after failed CAS returned error %v; want nil", err)
	}
	if !has {
		t.Fatal("hasParent = false after a CAS failure; HEAD should be unchanged, not unborn")
	}
	if sha != postMove {
		t.Errorf("HEAD after CAS failure = %q; want %q (atomic HEAD invariant, §18.1)", sha, postMove)
	}
	if sha == newSHA {
		t.Error("HEAD advanced to newSHA despite CAS failure; §18.1 invariant violated")
	}

	// The CAS failure is a routable typed *ExitError with Code 128 (git's
	// "cannot lock ref ... is at X but expected Y"), so the generate layer can
	// errors.As into it and produce the §18.2 message.
	var ee *ExitError
	if !errors.As(casErr, &ee) {
		t.Fatalf("CAS error is %T; want *ExitError (errors.As)", casErr)
	}
	if ee.Code != 128 {
		t.Errorf("ExitError.Code = %d; want 128", ee.Code)
	}
}
