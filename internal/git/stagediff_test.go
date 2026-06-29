package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sdManyLines returns a string with n lines of the form "line 0\nline 1\n...line n-1\n".
func sdManyLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestStagedDiff_MarkdownAndCode(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.md", "# Title\n\nbody\n")
	stageFile(t, repo, "a.md")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "b.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected payload to contain a.md, got:\n%s", out)
	}
	if !strings.Contains(out, "b.go") {
		t.Fatalf("expected payload to contain b.go, got:\n%s", out)
	}
}

func TestStagedDiff_ExcludesLockSnapMapVendor(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "keep.go", "package main\n")
	stageFile(t, repo, "keep.go")
	writeFile(t, repo, "pkg.lock", "{}\n")
	stageFile(t, repo, "pkg.lock")
	writeFile(t, repo, "package-lock.json", "{}\n")
	stageFile(t, repo, "package-lock.json")
	writeFile(t, repo, "x.snap", "snap\n")
	stageFile(t, repo, "x.snap")
	writeFile(t, repo, "y.map", "{}\n")
	stageFile(t, repo, "y.map")
	os.MkdirAll(filepath.Join(repo, "vendor"), 0o755)
	writeFile(t, repo, "vendor/lib.go", "package vendor\n")
	stageFile(t, repo, "vendor/lib.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "keep.go") {
		t.Fatalf("expected keep.go in payload, got:\n%s", out)
	}
	for _, noise := range []string{"pkg.lock", "package-lock.json", "x.snap", "y.map", "vendor/lib.go"} {
		if strings.Contains(out, noise) {
			t.Fatalf("expected %q to be excluded from payload, got:\n%s", noise, out)
		}
	}
}

func TestStagedDiff_MarkdownNotDoubleCounted(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "only.md", "# Hi\n\ncontent\n")
	stageFile(t, repo, "only.md")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "only.md") {
		t.Fatalf("expected only.md in payload, got:\n%s", out)
	}
	count := strings.Count(out, "diff --git a/only.md b/only.md")
	if count != 1 {
		t.Fatalf("expected exactly 1 diff hunk for only.md, got %d (double-count trap G1)\n%s", count, out)
	}
}

func TestStagedDiff_MarkdownLineCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.md", sdManyLines(50))
	stageFile(t, repo, "big.md")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{MaxMDLines: 10})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 10 lines]") {
		t.Fatalf("expected markdown line-cap sentinel, got:\n%s", out)
	}
	// With only markdown staged, the entire output is the md hunk. The cap keeps 10 lines
	// plus the sentinel, so the total newline count should be bounded.
	lineCount := strings.Count(out, "\n")
	if lineCount > 11 {
		t.Fatalf("expected ≤ 11 lines (10 cap + sentinel), got %d lines\n%s", lineCount, out)
	}
}

func TestStagedDiff_NonMarkdownByteCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000))
	stageFile(t, repo, "big.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 100})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "... [diff truncated at 100 bytes]") {
		t.Fatalf("expected byte-cap sentinel, got:\n%s", out)
	}
	if len(out) >= 200 {
		t.Fatalf("expected len(out) < 200 (capped at 100 + sentinel), got %d", len(out))
	}
}

func TestStagedDiff_NothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if out != "" {
		t.Fatalf("expected empty payload, got %q", out)
	}
}

func TestStagedDiff_OnlyMarkdown(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.md", "# x\n")
	stageFile(t, repo, "a.md")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected a.md in payload, got:\n%s", out)
	}
}

func TestStagedDiff_OnlyCode(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package x\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.go") {
		t.Fatalf("expected a.go in payload, got:\n%s", out)
	}
}

func TestStagedDiff_CustomExcludesOverride(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "keep.go", "package main\n")
	stageFile(t, repo, "keep.go")
	writeFile(t, repo, "drop.go", "package drop\n")
	stageFile(t, repo, "drop.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{Excludes: []string{":!drop.go"}})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "keep.go") {
		t.Fatalf("expected keep.go in payload, got:\n%s", out)
	}
	if strings.Contains(out, "drop.go") {
		t.Fatalf("expected drop.go to be excluded (custom excludes override), got:\n%s", out)
	}
}

func TestStagedDiff_DefaultsOnZero(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "small.md", "# tiny\n")
	stageFile(t, repo, "small.md")
	writeFile(t, repo, "small.go", "package main\n")
	stageFile(t, repo, "small.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{}) // zero-value
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if strings.Contains(out, "truncated") {
		t.Fatalf("expected no truncation sentinel for small diffs under default caps, got:\n%s", out)
	}
	if !strings.Contains(out, "small.md") {
		t.Fatalf("expected small.md in payload, got:\n%s", out)
	}
	if !strings.Contains(out, "small.go") {
		t.Fatalf("expected small.go in payload, got:\n%s", out)
	}
}

func TestStagedDiff_MarkdownExtensions(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.md", "# md\n")
	stageFile(t, repo, "a.md")
	writeFile(t, repo, "b.markdown", "# markdown\n")
	stageFile(t, repo, "b.markdown")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected a.md in payload, got:\n%s", out)
	}
	if !strings.Contains(out, "b.markdown") {
		t.Fatalf("expected b.markdown in payload, got:\n%s", out)
	}
}

func TestStagedDiff_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
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

func TestStagedDiff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	out, err := g.StagedDiff(ctx, StagedDiffOptions{})
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
