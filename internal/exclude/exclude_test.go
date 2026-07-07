package exclude

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/ui"
)

// ---------------------------------------------------------------------------
// TestTranslatePattern — golden-table unit tests
// ---------------------------------------------------------------------------

func TestTranslatePattern_GoldenTable(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// From research/gitignore-to-pathspec.md worked golden-table rows.
		{"*.lock", ":(exclude,glob)**/*.lock"},
		{"/*.lock", ":(exclude,glob)*.lock"},
		{"vendor/", ":(exclude,glob)**/vendor/**"},
		{"/build/", ":(exclude,glob)build/**"},
		{"docs/*.md", ":(exclude,glob)docs/*.md"},
		{"node_modules", ":(exclude,glob)**/node_modules"},
		{"a/**/b.go", ":(exclude,glob)a/**/b.go"},
		{"**/foo.txt", ":(exclude,glob)**/foo.txt"},
		{"src/gen*.ts", ":(exclude,glob)src/gen*.ts"},
		// Additional coverage.
		{"*.min.js", ":(exclude,glob)**/*.min.js"},
		{"/dist/", ":(exclude,glob)dist/**"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := TranslatePattern(tc.input)
			if got != tc.want {
				t.Errorf("TranslatePattern(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoadStagecoachIgnore
// ---------------------------------------------------------------------------

func TestLoadStagecoachIgnore_Basic(t *testing.T) {
	dir := t.TempDir()
	content := "*.lock\n/dist/\nvendor/\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{"*.lock", "/dist/", "vendor/"}
	if len(globs) != len(want) || !sliceEqual(globs, want) {
		t.Fatalf("got %v, want %v", globs, want)
	}
}

func TestLoadStagecoachIgnore_BlankAndCommentIgnored(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\n\n  \t  \n*.lock\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{"*.lock"}
	if len(globs) != len(want) || !sliceEqual(globs, want) {
		t.Fatalf("got %v, want %v", globs, want)
	}
}

func TestLoadStagecoachIgnore_NegationSkippedAndWarns(t *testing.T) {
	dir := t.TempDir()
	content := "!keep.min.js\n*.min.js\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(&buf, true))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{"*.min.js"}
	if len(globs) != len(want) || !sliceEqual(globs, want) {
		t.Fatalf("got %v, want %v", globs, want)
	}
	if !strings.Contains(buf.String(), "DEBUG: ignoring unsupported negation pattern in .stagecoachignore: !keep.min.js") {
		t.Fatalf("expected VerboseWarn output, got %q", buf.String())
	}
}

func TestLoadStagecoachIgnore_NegationNoWarnWhenOff(t *testing.T) {
	dir := t.TempDir()
	content := "!keep.min.js\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(&buf, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(globs) != 0 {
		t.Fatalf("got %v, want empty", globs)
	}
	if buf.Len() != 0 {
		t.Fatalf("off: wrote %q, want zero bytes", buf.String())
	}
}

func TestLoadStagecoachIgnore_CRLF(t *testing.T) {
	dir := t.TempDir()
	content := "*.lock\r\n/dist/\r\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{"*.lock", "/dist/"}
	if len(globs) != len(want) || !sliceEqual(globs, want) {
		t.Fatalf("got %v, want %v", globs, want)
	}
}

func TestLoadStagecoachIgnore_MissingFile(t *testing.T) {
	dir := t.TempDir()
	globs, err := LoadStagecoachIgnore(dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if globs != nil {
		t.Fatalf("got %v, want nil", globs)
	}
}

func TestLoadStagecoachIgnore_ReadError(t *testing.T) {
	// Point repoRoot at a file (not a dir) so ReadFile on .stagecoachignore inside it fails.
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "notafile")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStagecoachIgnore(filePath, ui.NewVerbose(nil, false))
	if err == nil {
		t.Fatal("expected error when repoRoot is a file, got nil")
	}
	if !strings.Contains(err.Error(), "read .stagecoachignore") {
		t.Fatalf("error should mention .stagecoachignore, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestResolveExcludePathspecs
// ---------------------------------------------------------------------------

func TestResolveExcludePathspecs_BothSources(t *testing.T) {
	dir := t.TempDir()
	content := "*.min.js\n/dist/\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Exclude: []string{"testdata/*", "vendor/"}}
	out, err := ResolveExcludePathspecs(cfg, dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{
		":(exclude,glob)**/*.min.js",
		":(exclude,glob)dist/**",
		":(exclude,glob)testdata/*",
		":(exclude,glob)**/vendor/**",
	}
	if len(out) != len(want) || !sliceEqual(out, want) {
		t.Fatalf("got %v, want %v", out, want)
	}
}

func TestResolveExcludePathspecs_CfgExcludeOnly(t *testing.T) {
	dir := t.TempDir() // no .stagecoachignore
	cfg := config.Config{Exclude: []string{"vendor/"}}
	out, err := ResolveExcludePathspecs(cfg, dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{":(exclude,glob)**/vendor/**"}
	if len(out) != len(want) || !sliceEqual(out, want) {
		t.Fatalf("got %v, want %v", out, want)
	}
}

func TestResolveExcludePathspecs_FileOnly(t *testing.T) {
	dir := t.TempDir()
	content := "*.lock\n"
	if err := os.WriteFile(filepath.Join(dir, StagecoachIgnoreFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Exclude: nil}
	out, err := ResolveExcludePathspecs(cfg, dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	want := []string{":(exclude,glob)**/*.lock"}
	if len(out) != len(want) || !sliceEqual(out, want) {
		t.Fatalf("got %v, want %v", out, want)
	}
}

func TestResolveExcludePathspecs_BothEmpty(t *testing.T) {
	dir := t.TempDir() // no .stagecoachignore
	cfg := config.Config{Exclude: nil}
	out, err := ResolveExcludePathspecs(cfg, dir, ui.NewVerbose(nil, false))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Fatalf("got %v, want empty", out)
	}
}

// ---------------------------------------------------------------------------
// TestPathspecsBehaveLikeGitignore — real-git integration test (semantic contract)
// ---------------------------------------------------------------------------

func TestPathspecsBehaveLikeGitignore(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found, skipping real-git integration test")
	}

	repo := t.TempDir()
	initTestRepo(t, repo)

	// Create files exercising different patterns:
	//   top.lock       — matches *.lock (any-depth)
	//   sub/nested.lock — matches *.lock (any-depth, nested)
	//   dist/x.js      — matches /dist/ (root dir)
	//   sub/dist/y.js  — should NOT match /dist/ (root-anchored)
	//   docs/readme.md — matches docs/*.md (root-anchored, middle slash)
	//   sub/docs/other.md — should NOT match docs/*.md (root-anchored)
	//   keep.go        — must survive (no pattern matches)
	//   vendor/lib.go  — matches vendor/ (any-depth dir)
	writeTestFile(t, repo, "top.lock", "lock\n")
	writeTestFile(t, repo, filepath.Join("sub", "nested.lock"), "nested\n")
	writeTestFile(t, repo, filepath.Join("dist", "x.js"), "x\n")
	writeTestFile(t, repo, filepath.Join("sub", "dist", "y.js"), "y\n")
	writeTestFile(t, repo, filepath.Join("docs", "readme.md"), "# docs\n")
	writeTestFile(t, repo, filepath.Join("sub", "docs", "other.md"), "# other\n")
	writeTestFile(t, repo, "keep.go", "package main\n")
	writeTestFile(t, repo, filepath.Join("vendor", "lib.go"), "package vendor\n")

	// Stage all files.
	stageTestFile(t, repo, "top.lock")
	stageTestFile(t, repo, filepath.Join("sub", "nested.lock"))
	stageTestFile(t, repo, filepath.Join("dist", "x.js"))
	stageTestFile(t, repo, filepath.Join("sub", "dist", "y.js"))
	stageTestFile(t, repo, filepath.Join("docs", "readme.md"))
	stageTestFile(t, repo, filepath.Join("sub", "docs", "other.md"))
	stageTestFile(t, repo, "keep.go")
	stageTestFile(t, repo, filepath.Join("vendor", "lib.go"))

	// Translate the exclusion patterns.
	patterns := []string{"*.lock", "/dist/", "docs/*.md", "vendor/"}
	var pathspecs []string
	for _, p := range patterns {
		pathspecs = append(pathspecs, TranslatePattern(p))
	}

	// Run git diff --cached --name-only with the pathspecs.
	args := []string{"-C", repo, "diff", "--cached", "--name-only", "--"}
	args = append(args, pathspecs...)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff failed: %v\n%s", err, string(out))
	}

	// Parse the output — only surviving files should appear.
	surviving := parseNameOnly(string(out))

	// Files that MUST be excluded (must NOT appear in output).
	excluded := map[string]bool{
		"top.lock":        true,
		"sub/nested.lock": true,
		"dist/x.js":       true,
		"docs/readme.md":  true,
		"vendor/lib.go":   true,
	}
	for _, f := range surviving {
		if excluded[f] {
			t.Errorf("path %q should have been excluded but appeared in diff output", f)
		}
	}

	// Files that MUST survive (must appear in output).
	mustSurvive := map[string]bool{
		"keep.go":           true,
		"sub/dist/y.js":     true, // /dist/ is root-anchored; sub/dist/ should survive
		"sub/docs/other.md": true, // docs/*.md is root-anchored; sub/docs/ should survive
	}
	for f := range mustSurvive {
		if !containsStr(surviving, f) {
			t.Errorf("path %q should have survived but is missing from diff output (surviving: %v)", f, surviving)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func parseNameOnly(out string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths
}

func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, out)
		}
	}
}

func writeTestFile(t *testing.T, repo, path, body string) {
	t.Helper()
	full := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stageTestFile(t *testing.T, repo, path string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "add", "--", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s failed: %v\n%s", path, err, out)
	}
}
