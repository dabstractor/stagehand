package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTreeDiff_BasicConceptDiff(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "b.go", "package lib\n")
	stageFile(t, repo, "b.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "b.go") {
		t.Fatalf("expected payload to contain b.go (the concept addition), got:\n%s", out)
	}
}

func TestTreeDiff_EmptyTreeBase(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "doc.md", "# x\n")
	stageFile(t, repo, "doc.md")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), EmptyTreeSHA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.go") {
		t.Fatalf("expected payload to contain a.go, got:\n%s", out)
	}
	if !strings.Contains(out, "doc.md") {
		t.Fatalf("expected payload to contain doc.md, got:\n%s", out)
	}
}

func TestTreeDiff_NoChanges(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	treeA := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeA, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if out != "" {
		t.Fatalf("expected empty payload for tree vs itself, got %q", out)
	}
}

func TestTreeDiff_BinaryPlaceholderAndExcluded(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "c.go", "package c\n")
	stageFile(t, repo, "c.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] logo.png") {
		t.Fatalf("expected FR3c placeholder for logo.png, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no 'Binary files' hunk body for logo.png, got:\n%s", out)
	}
	if !strings.Contains(out, "c.go") {
		t.Fatalf("expected text companion c.go present, got:\n%s", out)
	}
}

func TestTreeDiff_BinaryExtensionsUserOverride(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "x.go", "package main\n")
	stageFile(t, repo, "x.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "data.dat", "hello\n")
	stageFile(t, repo, "data.dat")
	treeB := writeTreeOf(t, repo)

	g := New(repo)

	// Without override: .dat with text content is NOT binary
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if strings.Contains(out, "[binary] data.dat") {
		t.Fatalf("expected NO binary placeholder without user override, got:\n%s", out)
	}

	// With override: caught via extension signal
	out, err = g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{BinaryExtensions: []string{"dat"}})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] data.dat") {
		t.Fatalf("expected binary placeholder with user override, got:\n%s", out)
	}
}

func TestTreeDiff_KeepsTextCompanion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "seed.go", "package main\n")
	stageFile(t, repo, "seed.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "img.png")
	writeFile(t, repo, "code.go", "package main\nfunc main() {}\n")
	stageFile(t, repo, "code.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] img.png") {
		t.Fatalf("expected binary placeholder, got:\n%s", out)
	}
	if !strings.Contains(out, "code.go") {
		t.Fatalf("expected code.go hunk present, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no binary hunk body, got:\n%s", out)
	}
}

func TestTreeDiff_ExcludesApplied(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "seed.go", "package main\n")
	stageFile(t, repo, "seed.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "keep.go", "package main\n")
	stageFile(t, repo, "keep.go")
	writeFile(t, repo, "drop.go", "package drop\n")
	stageFile(t, repo, "drop.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{Excludes: []string{":!drop.go"}})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "keep.go") {
		t.Fatalf("expected keep.go in payload, got:\n%s", out)
	}
	if strings.Contains(out, "drop.go") {
		t.Fatalf("expected drop.go to be excluded, got:\n%s", out)
	}
}

func TestTreeDiff_MarkdownNotDoubleCounted(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "seed.go", "package main\n")
	stageFile(t, repo, "seed.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "only.md", "# Hi\n\ncontent\n")
	stageFile(t, repo, "only.md")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "only.md") {
		t.Fatalf("expected only.md in payload, got:\n%s", out)
	}
	count := strings.Count(out, "diff --git a/only.md b/only.md")
	if count != 1 {
		t.Fatalf("expected exactly 1 diff hunk for only.md (no double-count), got %d\n%s", count, out)
	}
}

func TestTreeDiff_NonMarkdownByteCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "seed.go", "package main\n")
	stageFile(t, repo, "seed.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000))
	stageFile(t, repo, "big.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{MaxDiffBytes: 100})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 100 bytes]") {
		t.Fatalf("expected byte-cap sentinel, got:\n%s", out)
	}
	if len(out) >= 200 {
		t.Fatalf("expected len(out) < 200 (capped at 100 + sentinel), got %d", len(out))
	}
}

func TestTreeDiff_MarkdownLineCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "seed.go", "package main\n")
	stageFile(t, repo, "seed.go")
	treeA := writeTreeOf(t, repo)

	writeFile(t, repo, "big.md", sdManyLines(50))
	stageFile(t, repo, "big.md")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{MaxMDLines: 10})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 10 lines]") {
		t.Fatalf("expected markdown line-cap sentinel, got:\n%s", out)
	}
}

func TestTreeDiff_BadTreeSHA(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	treeA := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, "0000000000000000000000000000000000000000", StagedDiffOptions{})
	if err == nil {
		t.Fatal("expected error for bad tree SHA, got nil")
	}
	if out != "" {
		t.Fatalf("expected empty output on error, got %q", out)
	}
}

func TestTreeDiff_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")

	g := New(t.TempDir())
	out, err := g.TreeDiff(context.Background(), EmptyTreeSHA, EmptyTreeSHA, StagedDiffOptions{})
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

func TestTreeDiff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := New(t.TempDir())
	out, err := g.TreeDiff(ctx, EmptyTreeSHA, EmptyTreeSHA, StagedDiffOptions{})
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
