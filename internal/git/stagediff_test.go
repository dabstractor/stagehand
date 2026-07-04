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
	// The excluded file's hunk is absent but the [excluded] placeholder IS present.
	if strings.Contains(out, "diff --git a/drop.go") {
		t.Fatalf("expected drop.go hunk to be absent (user-excluded), got:\n%s", out)
	}
	if !strings.Contains(out, "[excluded] drop.go") {
		t.Fatalf("expected [excluded] placeholder for drop.go, got:\n%s", out)
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

func TestStagedDiff_BinaryFilePlaceholderAndExcluded(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Real binary (PNG header) — git content-sniffs ⇒ numstat -/-
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "logo.png")
	// Text companion
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] logo.png") {
		t.Fatalf("expected placeholder for logo.png, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no 'Binary files' body for logo.png, got:\n%s", out)
	}
	if !strings.Contains(out, "a.go") {
		t.Fatalf("expected text companion a.go present, got:\n%s", out)
	}
}

func TestStagedDiff_BinaryKeepsTextCompanion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "img.png")
	writeFile(t, repo, "code.go", "package main\nfunc main() {}\n")
	stageFile(t, repo, "code.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
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

func TestStagedDiff_BinaryExtensionsUserOverride(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// TEXT content in a .dat file (.dat is NOT in the 36-entry built-in denylist)
	writeFile(t, repo, "data.dat", "hello\n")
	stageFile(t, repo, "data.dat")

	g := New(repo)

	// Without the user override — .dat with text content is NOT binary
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if strings.Contains(out, "[binary] data.dat") {
		t.Fatalf("expected NO binary placeholder without user override, got:\n%s", out)
	}

	// With the user override — caught via extension signal
	out, err = g.StagedDiff(context.Background(), StagedDiffOptions{BinaryExtensions: []string{"dat"}})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] data.dat") {
		t.Fatalf("expected binary placeholder with user override, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no binary hunk body (text content), got:\n%s", out)
	}
}

func TestStagedDiff_BinaryInSubdirectory(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	os.MkdirAll(filepath.Join(repo, "assets"), 0o755)
	writeFile(t, repo, "assets/logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "assets/logo.png")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "A\t[binary] assets/logo.png") {
		t.Fatalf("expected subdirectory binary placeholder, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no binary body, got:\n%s", out)
	}
}

func TestStagedDiff_MixedMarkdownBinaryCode(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.md", "# Title\n\nbody\n")
	stageFile(t, repo, "a.md")
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "b.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected markdown a.md, got:\n%s", out)
	}
	if !strings.Contains(out, "A\t[binary] logo.png") {
		t.Fatalf("expected binary placeholder, got:\n%s", out)
	}
	if !strings.Contains(out, "b.go") {
		t.Fatalf("expected code b.go, got:\n%s", out)
	}
	if strings.Contains(out, "Binary files") {
		t.Fatalf("expected no binary hunk body, got:\n%s", out)
	}
}

func TestStagedDiff_NoBinaryWhenOnlyText(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "package lib\n")
	stageFile(t, repo, "b.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if strings.Contains(out, "[binary]") {
		t.Fatalf("expected no [binary] lines for text-only staging, got:\n%s", out)
	}
}

func TestStagedDiff_ExcludedPlaceholder(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// feature.go (kept, non-excluded), secret.conf (user-excluded), pkg.lock (default-denylist)
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")
	writeFile(t, repo, "secret.conf", "password=abc\n")
	stageFile(t, repo, "secret.conf")
	writeFile(t, repo, "pkg.lock", "{}\n")
	stageFile(t, repo, "pkg.lock")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{Excludes: []string{":(exclude,glob)**/secret.conf"}})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	// feature.go present
	if !strings.Contains(out, "feature.go") {
		t.Fatalf("expected feature.go hunk present, got:\n%s", out)
	}
	// secret.conf: hunk absent, placeholder present
	if strings.Contains(out, "diff --git a/secret.conf") {
		t.Fatalf("expected secret.conf hunk to be absent, got:\n%s", out)
	}
	if !strings.Contains(out, "[excluded] secret.conf") {
		t.Fatalf("expected [excluded] placeholder for secret.conf, got:\n%s", out)
	}
	// UNION proof: pkg.lock is ALSO excluded (defaultExcludes still applies when opts.Excludes is set)
	if strings.Contains(out, "pkg.lock") {
		t.Fatalf("expected pkg.lock to be excluded (default denylist UNION), got:\n%s", out)
	}
}

func TestStagedDiff_ExcludedEmptyIsNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{}) // Excludes is nil (zero value)
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "a.go") {
		t.Fatalf("expected a.go present, got:\n%s", out)
	}
	if strings.Contains(out, "[excluded]") {
		t.Fatalf("expected no [excluded] placeholder when Excludes is nil, got:\n%s", out)
	}
}

func TestStagedDiff_ExcludedBinaryPrecedence(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Real binary (PNG) that is also user-excluded → [binary] only, no [excluded]
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{Excludes: []string{":(exclude,glob)**/logo.png"}})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "[binary] logo.png") {
		t.Fatalf("expected [binary] for binary+excluded file, got:\n%s", out)
	}
	if strings.Contains(out, "[excluded] logo.png") {
		t.Fatalf("expected NO [excluded] for binary+excluded file (binary wins), got:\n%s", out)
	}
	if strings.Contains(out, "diff --git a/logo.png") {
		t.Fatalf("expected no hunk for binary+excluded file, got:\n%s", out)
	}
	if !strings.Contains(out, "a.go") {
		t.Fatalf("expected a.go present, got:\n%s", out)
	}
}

// TestStagedDiff_RenameDetectedCompact pins FR3e (-M, always on): a staged pure rename
// (delete old + add new of identical content, >=50% similar) must render as the compact extended
// header (rename from / rename to) rather than a full-content delete+add body.
func TestStagedDiff_RenameDetectedCompact(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "old.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "old.go")
	execGit(t, repo, "commit", "-qm", "base") // baseline

	// pure rename: identical content, new path
	writeFile(t, repo, "new.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "new.go")
	os.Remove(filepath.Join(repo, "old.go"))
	execGit(t, repo, "add", "-A") // stage delete-old + add-new (>=50% similar -> -M sees a rename)

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	if !strings.Contains(out, "rename from") || !strings.Contains(out, "rename to") {
		t.Fatalf("FR3e (-M): expected compact rename (rename from/to), got:\n%s", out)
	}
}

// TestStagedDiff_DiffContextZero_ChangedLinesOnly pins FR3f (-U<n>): DiffContext:0 (-U0) emits
// changed lines only (the unchanged anchor `func a()` line is ABSENT), while DiffContext:1 (-U1,
// the production default) retains one anchor context line each side of the change (the anchor IS present).
func TestStagedDiff_DiffContextZero_ChangedLinesOnly(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc b(){}\nfunc c(){}\n")
	stageFile(t, repo, "a.go")
	execGit(t, repo, "commit", "-qm", "base")

	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc B(){}\nfunc c(){}\n") // edit middle line only
	stageFile(t, repo, "a.go")

	g := New(repo)
	out0, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 0})
	if err != nil {
		t.Fatalf("StagedDiff -U0: %v", err)
	}
	out1, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff -U1: %v", err)
	}
	// The unchanged `func a(){}` line appears as a context line (single leading space) at -U1 but is
	// dropped at -U0 (changed-lines-only). We check for the leading-space context form to avoid
	// matching the `@@ ... @@ func a(){}` hunk heading that git emits at every -U level.
	if !strings.Contains(out1, "\n func a(){}") {
		t.Fatalf("DiffContext:1 (-U1) should retain an anchor context line, got:\n%s", out1)
	}
	if strings.Contains(out0, "\n func a(){}") {
		t.Fatalf("DiffContext:0 (-U0) should drop unchanged context (changed-lines-only), got:\n%s", out0)
	}
}
