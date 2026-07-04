package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// indexLeakRe is the FR3h LEAK DETECTOR used by the regression tests: it matches ANY git index-header
// line at all (`index <hex>..<hex>` optionally followed by a space + octal mode), in BOTH the
// modified-file form (with trailing space+mode) and the add/delete form (mode on the separate
// `new file mode` / `deleted file mode` line, so NO trailing space). It is BROADER than the
// production indexLineRe on purpose: testing the leak with indexLineRe itself would be circular — a
// buggy indexLineRe that fails to match the no-mode form would also fail to *detect* the leak.
var indexLeakRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+( [0-7]+)?$`)

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
	// With only markdown staged, the markdown body is the md hunk. The cap keeps 10 lines plus the
	// sentinel. The payload is led by the FR3g skeleton (header + one numstat row + blank line = 3
	// lines), so the total newline count is bounded by 11 (md) + 3 (skeleton) = 14.
	lineCount := strings.Count(out, "\n")
	if lineCount > 14 {
		t.Fatalf("expected ≤ 14 lines (10 md cap + sentinel + 3 skeleton), got %d lines\n%s", lineCount, out)
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

// TestStripIndexLines verifies FR3h's post-capture filter: the `index <oid>..<oid> <mode>` header line is
// removed; every other line is preserved verbatim — including a content line that starts with the word
// "index" but lacks the OID `..` form (the regex disambiguator), and the rename/similarity extended headers.
func TestStripIndexLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "index line removed, structural + content kept",
			input: "diff --git a/a.go b/a.go\nindex 600d48a..62b056e 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			// FR3h regression for NEWLY-ADDED files: git emits the index line WITHOUT a trailing
			// space+mode (the mode goes on the separate `new file mode` line). The original regex
			// required a trailing space, leaking the blob OID for every added file.
			name:  "NEW file: index line WITHOUT trailing mode is removed (FR3h add/delete regression)",
			input: "diff --git a/x.go b/x.go\nnew file mode 100644\nindex 0000000..32b4245\n--- /dev/null\n+++ b/x.go\n@@ -0,0 +1 @@\n+new\n",
			want:  "diff --git a/x.go b/x.go\nnew file mode 100644\n--- /dev/null\n+++ b/x.go\n@@ -0,0 +1 @@\n+new\n",
		},
		{
			// FR3h regression for DELETED files: same no-trailing-mode form as adds.
			name:  "DELETED file: index line WITHOUT trailing mode is removed",
			input: "diff --git a/x.go b/x.go\ndeleted file mode 100644\nindex 3367afd..0000000\n--- a/x.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n",
			want:  "diff --git a/x.go b/x.go\ndeleted file mode 100644\n--- a/x.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n",
		},
		{
			// THE contract negative case: a content line that starts with "index" but lacks the OID form is KEPT.
			// "index of items" → "of" is not hex → no match. The diff-marked variants (space/-/+) also kept.
			name:  "content line starting with index but no OID form is kept",
			input: "index of items in the list\n",
			want:  "index of items in the list\n",
		},
		{
			name:  "diff-marked content lines mentioning index are kept (markers protect them)",
			input: "diff --git a/a.go b/a.go\nindex 600d48a..62b056e 100644\n@@ -1,3 +1,3 @@\n // index of items\n-index of items\n+index of other\n",
			want:  "diff --git a/a.go b/a.go\n@@ -1,3 +1,3 @@\n // index of items\n-index of items\n+index of other\n",
		},
		{
			name:  "no index line → byte-identical (fast path)",
			input: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			name:  "multiple files → all index lines removed, headers/content kept",
			input: "diff --git a/a.go b/a.go\nindex 1111111..2222222 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/b.md b/b.md\nindex 3333333..4444444 100644\n--- a/b.md\n+++ b/b.md\n@@ -1 +1 @@\n-x\n+y\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/b.md b/b.md\n--- a/b.md\n+++ b/b.md\n@@ -1 +1 @@\n-x\n+y\n",
		},
		{
			// Composes with FR3e (-M): a pure rename has NO index line; similarity index / rename from / to
			// start with a different token → all KEPT. stripIndexLines is a no-op here (correct).
			name:  "rename path: similarity index / rename from / rename to KEPT (no index line present)",
			input: "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n",
			want:  "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n",
		},
		{
			name:  "empty string → empty string",
			input: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := stripIndexLines(tc.input); got != tc.want {
				t.Errorf("stripIndexLines mismatch:\n got=%q\nwant=%q", got, tc.want)
			}
		})
	}
}

// TestStagedDiff_IndexLineStripped is the FR3h integration test: a real captured StagedDiff payload
// contains NO `index <oid>..<oid> <mode>` line, while the structural identity lines (diff --git, +++) are
// retained. Proves the helper is wired into the capture→strip→cap path.
func TestStagedDiff_IndexLineStripped(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	execGit(t, repo, "commit", "-qm", "base")
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	// FR3h: no line in the captured payload matches the index-header leak detector.
	for _, line := range strings.Split(out, "\n") {
		if indexLeakRe.MatchString(line) {
			t.Errorf("FR3h: index line present in StagedDiff output: %q\nfull output:\n%s", line, out)
		}
	}
	// Sanity: the kept structural lines are present.
	if !strings.Contains(out, "diff --git a/a.go b/a.go") {
		t.Errorf("diff --git header missing (should be KEPT):\n%s", out)
	}
	if !strings.Contains(out, "+++ b/a.go") {
		t.Errorf("+++ header missing (should be KEPT):\n%s", out)
	}
}

// TestStagedDiff_IndexLineStripped_NewFile is the FR3h regression for newly-ADDED files. Git emits the
// index line WITHOUT a trailing space+mode for add/delete (the mode goes on the separate `new file mode`
// line). The original indexLineRe required a trailing space, so the no-mode form LEAKED into the agent
// payload for every added file. This test stages a brand-new file and asserts the index line is stripped.
//
// Leak detection uses a BROADER regex than indexLineRe: any `index <hex>..<hex>` line at all (with or
// without a trailing mode). Testing the leak with indexLineRe itself would be circular — a buggy
// indexLineRe that fails to match the no-mode form would also fail to *detect* the leak.
func TestStagedDiff_IndexLineStripped_NewFile(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	execGit(t, repo, "commit", "-qm", "base")

	writeFile(t, repo, "brand_new.go", "package main\nfunc new(){}\n")
	stageFile(t, repo, "brand_new.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	// FR3h: detect ANY index line (with or without trailing mode) leaking into the payload.
	for _, line := range strings.Split(out, "\n") {
		if indexLeakRe.MatchString(line) {
			t.Errorf("FR3h: index line present in StagedDiff output for a NEW file: %q\nfull output:\n%s", line, out)
		}
	}
	// Sanity: the structural markers for a new-file diff are present.
	if !strings.Contains(out, "new file mode 100644") {
		t.Errorf("new file mode line missing (should be KEPT):\n%s", out)
	}
	if !strings.Contains(out, "diff --git a/brand_new.go b/brand_new.go") {
		t.Errorf("diff --git header missing (should be KEPT):\n%s", out)
	}
}

// TestStagedDiff_IndexLineStripped_DeletedFile is the FR3h regression for DELETED files. As with new
// files, git omits the trailing space+mode on the index line for deletions (the mode goes on the
// `deleted file mode` line). The original regex required the trailing space, leaking the blob OID into
// the agent payload for every deleted file.
func TestStagedDiff_IndexLineStripped_DeletedFile(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "doomed.go", "package main\nfunc doomed(){}\n")
	stageFile(t, repo, "doomed.go")
	execGit(t, repo, "commit", "-qm", "base")

	execGit(t, repo, "rm", "-q", "doomed.go") // git rm stages the deletion

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if indexLeakRe.MatchString(line) {
			t.Errorf("FR3h: index line present in StagedDiff output for a DELETED file: %q\nfull output:\n%s", line, out)
		}
	}
	if !strings.Contains(out, "deleted file mode 100644") {
		t.Errorf("deleted file mode line missing (should be KEPT):\n%s", out)
	}
	if !strings.Contains(out, "diff --git a/doomed.go b/doomed.go") {
		t.Errorf("diff --git header missing (should be KEPT):\n%s", out)
	}
}

// TestStagedDiff_OrderingInvariant_SkeletonPlaceholdersMdCode pins the FR3g ordering invariant
// (PRD §9.1 FR3g / system_context §7): the payload sections appear in the canonical order
// skeleton → [binary]/[excluded] placeholders → markdown bodies → non-markdown bodies. With a
// staged change spanning a code file, a markdown file, and a binary file, every section is present
// and the indices are strictly increasing. (The contract's "Ordering invariant — test it".)
func TestStagedDiff_OrderingInvariant_SkeletonPlaceholdersMdCode(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "code.go", "package main\nfunc main() {}\n") // non-markdown body
	writeFile(t, repo, "README.md", "# Title\n\nbody\n")            // markdown body
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00") // binary placeholder
	stageFile(t, repo, "code.go")
	stageFile(t, repo, "README.md")
	stageFile(t, repo, "logo.png")

	out, err := New(repo).StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	iSkeleton := strings.Index(out, "Change summary (numstat:")
	iBinary := strings.Index(out, "[binary] logo.png")
	iMd := strings.Index(out, "diff --git a/README.md")
	iCode := strings.Index(out, "diff --git a/code.go")
	for _, idx := range []struct {
		name string
		i    int
	}{
		{"skeleton", iSkeleton},
		{"binary placeholder", iBinary},
		{"markdown body", iMd},
		{"code body", iCode},
	} {
		if idx.i < 0 {
			t.Fatalf("%s section not found in output:\n%s", idx.name, out)
		}
	}
	if !(iSkeleton < iBinary && iBinary < iMd && iMd < iCode) {
		t.Fatalf("ordering invariant violated: skeleton@%d binary@%d md@%d code@%d "+
			"(want skeleton < binary < md < code)\n%s", iSkeleton, iBinary, iMd, iCode, out)
	}
}

// TestStagedDiff_SkeletonCompleteUnderBodyCap pins the FR3g completeness floor: the skeleton lists
// EVERY changed file (including binary) even when a diff body is byte-capped. (The under-truncation
// resilience itself is verified in M4; S2 asserts the floor pre-truncation.)
func TestStagedDiff_SkeletonCompleteUnderBodyCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.go", strings.Repeat("// line\n", 300))  // will be byte-capped in Part 2
	writeFile(t, repo, "small.go", "package main\n")                // whole body
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00") // binary (no body)
	stageFile(t, repo, "big.go")
	stageFile(t, repo, "small.go")
	stageFile(t, repo, "logo.png")

	out, err := New(repo).StagedDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 50}) // tight cap
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	// The skeleton block is everything from the header up to the first blank line that follows it.
	// It must list every changed file regardless of body truncation.
	skeleton := out
	if strings.HasPrefix(out, "Change summary (numstat:") {
		// header line + rows end at the first blank line (the second "\n\n" boundary after the header).
		rest := out
		// Find the blank-line terminator: the first occurrence of "\n\n" after the header row.
		if i := strings.Index(rest, "\n\n"); i >= 0 {
			skeleton = rest[:i]
		}
	} else {
		t.Fatalf("payload does not begin with the skeleton header:\n%s", out)
	}
	for _, path := range []string{"big.go", "small.go", "logo.png"} {
		if !strings.Contains(skeleton, path) {
			t.Errorf("skeleton missing changed file %s (FR3g completeness floor):\nskeleton:\n%s\nfull:\n%s",
				path, skeleton, out)
		}
	}
}
