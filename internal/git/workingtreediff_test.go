package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWorkingTreeDiff_BasicWorkingTreeDiff(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "code.go", "package main\n")
	stageFile(t, repo, "code.go")
	execGit(t, repo, "commit", "-m", "init") // tracked baseline; index==HEAD

	writeFile(t, repo, "code.go", "package main\n// modified\n") // working-tree delta (NOT staged)

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected payload to contain code.go, got:\n%s", out)
	}
	if !strings.Contains(out, "// modified") {
		t.Fatalf("expected payload to contain '// modified', got:\n%s", out)
	}
}

func TestWorkingTreeDiff_CleanWorkingTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // nothing in working tree

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if out != "" {
		t.Fatalf("expected empty payload for clean working tree, got %q", out)
	}
}

func TestWorkingTreeDiff_BinaryPlaceholderAndExcluded(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00old")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "code.go", "package main\n")
	stageFile(t, repo, "code.go")
	execGit(t, repo, "commit", "-m", "init") // tracked baseline

	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00new") // tracked binary MODIFIED
	writeFile(t, repo, "code.go", "package main\n// x\n")              // tracked text MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "M\t[binary] logo.png") {
		t.Fatalf("expected FR3c placeholder for logo.png (working-tree status M), got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no 'Binary files' hunk body for logo.png, got:\n%s", out)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected text companion code.go present, got:\n%s", out)
	}
}

func TestWorkingTreeDiff_BinaryExtensionsUserOverride(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "data.dat", "hello\n")
	stageFile(t, repo, "data.dat")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "data.dat", "hello world\n") // tracked .dat MODIFIED (text content)

	g := New(repo)

	// Without override: .dat with text content is NOT binary
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if strings.Contains(out, "[binary] data.dat") {
		t.Fatalf("expected NO binary placeholder without user override, got:\n%s", out)
	}

	// With override: caught via extension signal
	out, err = g.WorkingTreeDiff(context.Background(), StagedDiffOptions{BinaryExtensions: []string{"dat"}})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "M\t[binary] data.dat") {
		t.Fatalf("expected binary placeholder with user override, got:\n%s", out)
	}
}

func TestWorkingTreeDiff_KeepsTextCompanion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00old")
	stageFile(t, repo, "img.png")
	writeFile(t, repo, "code.go", "package main\n")
	stageFile(t, repo, "code.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00new") // binary MODIFIED
	writeFile(t, repo, "code.go", "package main\nfunc main() {}\n")   // text MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "M\t[binary] img.png") {
		t.Fatalf("expected binary placeholder, got:\n%s", out)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected code.go hunk present, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no binary hunk body, got:\n%s", out)
	}
}

func TestWorkingTreeDiff_ExcludesApplied(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "keep.go", "package main\n")
	stageFile(t, repo, "keep.go")
	writeFile(t, repo, "drop.go", "package drop\n")
	stageFile(t, repo, "drop.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "keep.go", "package main\n// k\n") // MODIFIED
	writeFile(t, repo, "drop.go", "package drop\n// d\n") // MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{Excludes: []string{":!drop.go"}})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "keep.go") {
		t.Fatalf("expected keep.go in payload, got:\n%s", out)
	}
	// The excluded file's hunk is absent but the [excluded] placeholder IS present.
	if strings.Contains(out, "diff --git a/drop.go") {
		t.Fatalf("expected drop.go hunk to be absent (user-excluded), got:\n%s", out)
	}
	if !strings.Contains(out, "[excluded] drop.go") {
		t.Fatalf("expected [excluded] placeholder for drop.go, got:\n%s", out)
	}
}

func TestWorkingTreeDiff_MarkdownNotDoubleCounted(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "only.md", "# a\n")
	stageFile(t, repo, "only.md")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "only.md", "# a\n\nmore\n") // tracked markdown MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "only.md") {
		t.Fatalf("expected only.md in payload, got:\n%s", out)
	}
	count := strings.Count(out, "diff --git a/only.md b/only.md")
	if count != 1 {
		t.Fatalf("expected exactly 1 diff hunk for only.md (no double-count), got %d\n%s", count, out)
	}
}

func TestWorkingTreeDiff_NonMarkdownByteCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.go", "package main\n")
	stageFile(t, repo, "big.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000)) // big MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 100})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 100 bytes]") {
		t.Fatalf("expected byte-cap sentinel, got:\n%s", out)
	}
	if len(out) >= 200 {
		t.Fatalf("expected len(out) < 200 (capped at 100 + sentinel), got %d", len(out))
	}
}

func TestWorkingTreeDiff_MarkdownLineCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.md", "# t\n")
	stageFile(t, repo, "big.md")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "big.md", sdManyLines(50)) // tracked markdown MODIFIED, big

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{MaxMDLines: 10})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 10 lines]") {
		t.Fatalf("expected markdown line-cap sentinel, got:\n%s", out)
	}
}

// TestWorkingTreeDiff_UntrackedFilesOmitted documents the git-diff domain gotcha:
// `git diff` (no --cached) compares working-tree-vs-index, and git NEVER lists untracked files
// in a diff (untracked = not in the index = nothing to diff against). This is the explicit contract,
// NOT a bug. The tooled stager (FR-M5) discovers untracked files itself.
func TestWorkingTreeDiff_UntrackedFilesOmitted(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "tracked.go", "package main\n")
	stageFile(t, repo, "tracked.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "tracked.go", "package main\n// modified\n") // tracked MODIFIED → SHOWN
	writeFile(t, repo, "untracked.go", "new\n")                     // UNTRACKED → NOT shown

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "tracked.go") {
		t.Fatalf("expected tracked-modified to be shown, got:\n%s", out)
	}
	if strings.Contains(out, "untracked.go") {
		t.Fatalf("expected untracked file to be omitted (git diff domain), got:\n%s", out)
	}
}

func TestWorkingTreeDiff_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo

	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err == nil {
		t.Fatal("expected error for non-repo, got nil")
	}
	if out != "" {
		t.Fatalf("expected empty output on error, got %q", out)
	}
}

func TestWorkingTreeDiff_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")

	g := New(t.TempDir())
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err == nil {
		t.Fatal("expected error when git binary is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("expected error to contain 'git binary not found', got: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestWorkingTreeDiff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call

	g := New(t.TempDir())
	out, err := g.WorkingTreeDiff(ctx, StagedDiffOptions{})
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestWorkingTreeDiff_ExcludedPlaceholderAndUnion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "keep.go", "package main\n")
	stageFile(t, repo, "keep.go")
	writeFile(t, repo, "secret.conf", "pass=abc\n")
	stageFile(t, repo, "secret.conf")
	writeFile(t, repo, "pkg.lock", "{}\n")
	stageFile(t, repo, "pkg.lock")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "keep.go", "package main\n// k\n")      // MODIFIED
	writeFile(t, repo, "secret.conf", "pass=xyz\n")             // MODIFIED
	writeFile(t, repo, "pkg.lock", "{\"v\": 2}\n")            // MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{Excludes: []string{":(exclude,glob)**/secret.conf"}})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "keep.go") {
		t.Fatalf("expected keep.go present, got:\n%s", out)
	}
	if strings.Contains(out, "diff --git a/secret.conf") {
		t.Fatalf("expected secret.conf hunk absent, got:\n%s", out)
	}
	if !strings.Contains(out, "[excluded] secret.conf") {
		t.Fatalf("expected [excluded] placeholder for secret.conf, got:\n%s", out)
	}
	// UNION: pkg.lock also excluded
	if strings.Contains(out, "pkg.lock") {
		t.Fatalf("expected pkg.lock excluded (default denylist UNION), got:\n%s", out)
	}
}

func TestWorkingTreeDiff_ExcludedEmptyIsNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "code.go", "package main\n")
	stageFile(t, repo, "code.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "code.go", "package main\n// m\n") // MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected code.go present, got:\n%s", out)
	}
	if strings.Contains(out, "[excluded]") {
		t.Fatalf("expected no [excluded] when Excludes is nil, got:\n%s", out)
	}
}

func TestWorkingTreeDiff_ExcludedBinaryPrecedence(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00old")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "code.go", "package main\n")
	stageFile(t, repo, "code.go")
	execGit(t, repo, "commit", "-m", "init")

	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00new") // binary MODIFIED
	writeFile(t, repo, "code.go", "package main\n// x\n")             // text MODIFIED

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{Excludes: []string{":(exclude,glob)**/logo.png"}})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "[binary] logo.png") {
		t.Fatalf("expected [binary] for binary+excluded, got:\n%s", out)
	}
	if strings.Contains(out, "[excluded] logo.png") {
		t.Fatalf("expected NO [excluded] for binary+excluded (binary wins), got:\n%s", out)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected code.go present, got:\n%s", out)
	}
}
