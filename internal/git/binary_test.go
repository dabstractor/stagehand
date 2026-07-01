package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// asRunner unwraps a Git interface value to the concrete *gitRunner for internal method tests.
// Both files are in package git, so this is legal.
func asRunner(g Git) *gitRunner { return g.(*gitRunner) }

// ---- Pure-function table tests (no git repo needed) ----

func TestIsBinaryByExtension(t *testing.T) {
	tests := []struct {
		path  string
		extra []string
		want  bool
		desc  string
	}{
		{"logo.PNG", nil, true, "case-insensitive hit"},
		{"a.jpg", nil, true, "default hit"},
		{"archive.tar.gz", nil, true, "terminal ext .gz in list"},
		{"a.md", nil, false, "default miss"},
		{"noext", nil, false, "no extension"},
		{"a.", nil, false, "trailing dot → empty ext"},
		{"a.dat", nil, false, "not in default list"},
		{"a.dat", []string{"dat"}, true, "extra ext without dot"},
		{"a.DAT", []string{".dat"}, true, "extra ext with dot, case-insensitive"},
		{"a.bin", []string{" bin "}, true, "extra ext with whitespace"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := isBinaryByExtension(tc.path, tc.extra)
			if got != tc.want {
				t.Errorf("isBinaryByExtension(%q, %v) = %v, want %v", tc.path, tc.extra, got, tc.want)
			}
		})
	}
}

func TestBinaryPlaceholderLine(t *testing.T) {
	tests := []struct {
		status, path string
		want         string
		desc         string
	}{
		{"M", "assets/logo.png", "M\t[binary] assets/logo.png", "modified"},
		{"A", "public/trailer.mp4", "A\t[binary] public/trailer.mp4", "added"},
		{"D", "old.bin", "D\t[binary] old.bin", "deleted"},
		{"R100", "new.png", "R100\t[binary] new.png", "rename status"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := binaryPlaceholderLine(tc.status, tc.path)
			if got != tc.want {
				t.Errorf("binaryPlaceholderLine(%q, %q) = %q, want %q", tc.status, tc.path, got, tc.want)
			}
		})
	}
}

// ---- detectBinaryFiles: temp-repo tests ----

func TestDetectBinaryFiles_RealBinaryNumstat(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Write a REAL binary (PNG header — git content-sniffs as binary ⇒ numstat emits -/-).
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	stageFile(t, repo, "logo.png")

	// Write a text file — should NOT appear in the binary set.
	writeFile(t, repo, "notes.txt", "hello world\n")
	stageFile(t, repo, "notes.txt")

	g := asRunner(New(repo))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("detectBinaryFiles err = %v, want nil", err)
	}
	if !binary["logo.png"] {
		t.Errorf("expected logo.png in binary set, got: %v", binary)
	}
	if binary["notes.txt"] {
		t.Errorf("expected notes.txt NOT in binary set, got: %v", binary)
	}
}

func TestDetectBinaryFiles_ExtensionOnlyTextPng(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// TEXT content with a denylisted extension: git treats as text (numstat 1/0),
	// so ONLY the extension denylist catches it. This is the key two-signal union test.
	writeFile(t, repo, "fake.png", "hello\n")
	stageFile(t, repo, "fake.png")

	g := asRunner(New(repo))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("detectBinaryFiles err = %v, want nil", err)
	}
	if !binary["fake.png"] {
		t.Errorf("expected fake.png in binary set (extension denylist), got: %v", binary)
	}
}

func TestDetectBinaryFiles_TextFileNotBinary(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "main.go", "package main\nfunc main() {}\n")
	stageFile(t, repo, "main.go")

	g := asRunner(New(repo))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("detectBinaryFiles err = %v, want nil", err)
	}
	if len(binary) != 0 {
		t.Errorf("expected empty binary set, got: %v", binary)
	}
}

func TestDetectBinaryFiles_EmptyRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := asRunner(New(repo))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("detectBinaryFiles err = %v, want nil", err)
	}
	if len(binary) != 0 {
		t.Errorf("expected empty binary set, got: %v", binary)
	}
}

func TestDetectBinaryFiles_Rename(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	// Commit a binary, then rename it — detect via working tree (no --cached).
	writeFile(t, repo, "old.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	stageFile(t, repo, "old.png")
	makeEmptyCommit(t, repo, "initial")

	// Rename via git mv.
	mvCmd := exec.Command("git", "-C", repo, "mv", "old.png", "new.png")
	if out, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v\n%s", err, out)
	}

	g := asRunner(New(repo))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("detectBinaryFiles err = %v, want nil", err)
	}
	if len(binary) == 0 {
		t.Errorf("expected non-empty binary set for rename, got: %v", binary)
	}
	// The numstat key for a rename contains " => " (findings §4).
	foundRenameKey := false
	for k := range binary {
		if strings.Contains(k, "=>") {
			foundRenameKey = true
			break
		}
	}
	if !foundRenameKey {
		t.Errorf("expected a key containing '=>' for the rename, got keys: %v", binary)
	}
}

func TestDetectBinaryFiles_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := asRunner(New(t.TempDir()))
	binary, err := g.detectBinaryFiles(context.Background(), "--cached")
	if err == nil {
		t.Fatal("expected error when git binary is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("expected error to contain 'git binary not found', got: %v", err)
	}
	if binary != nil {
		t.Fatalf("expected nil map, got: %v", binary)
	}
}

func TestDetectBinaryFiles_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := asRunner(New(t.TempDir()))
	binary, err := g.detectBinaryFiles(ctx, "--cached")
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if binary != nil {
		t.Fatalf("expected nil map, got: %v", binary)
	}
}

// ---- fileStatuses: temp-repo tests ----

func TestFileStatuses_AddedModified(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	// Stage a real binary and a text file.
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	stageFile(t, repo, "logo.png")
	writeFile(t, repo, "notes.txt", "hello\n")
	stageFile(t, repo, "notes.txt")

	g := asRunner(New(repo))
	statuses, err := g.fileStatuses(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("fileStatuses err = %v, want nil", err)
	}
	if statuses["logo.png"] != "A" {
		t.Errorf("expected logo.png status 'A', got %q", statuses["logo.png"])
	}
	if statuses["notes.txt"] != "A" {
		t.Errorf("expected notes.txt status 'A', got %q", statuses["notes.txt"])
	}
}

func TestFileStatuses_RenameDestination(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	// Commit a text file, then rename it.
	writeFile(t, repo, "old.txt", "hello\n")
	stageFile(t, repo, "old.txt")
	makeEmptyCommit(t, repo, "initial")

	mvCmd := exec.Command("git", "-C", repo, "mv", "old.txt", "new.txt")
	if out, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v\n%s", err, out)
	}

	g := asRunner(New(repo))
	statuses, err := g.fileStatuses(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("fileStatuses err = %v, want nil", err)
	}
	status, ok := statuses["new.txt"]
	if !ok {
		t.Errorf("expected new.txt in statuses, got: %v", statuses)
	}
	if !strings.HasPrefix(status, "R") {
		t.Errorf("expected rename status starting with 'R', got %q", status)
	}
}

func TestFileStatuses_EmptyRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := asRunner(New(repo))
	statuses, err := g.fileStatuses(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("fileStatuses err = %v, want nil", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty statuses, got: %v", statuses)
	}
}

func TestFileStatuses_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")

	g := asRunner(New(t.TempDir()))
	statuses, err := g.fileStatuses(context.Background(), "--cached")
	if err == nil {
		t.Fatal("expected error when git binary is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("expected error to contain 'git binary not found', got: %v", err)
	}
	if statuses != nil {
		t.Fatalf("expected nil map, got: %v", statuses)
	}
}

func TestFileStatuses_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := asRunner(New(t.TempDir()))
	statuses, err := g.fileStatuses(ctx, "--cached")
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if statuses != nil {
		t.Fatalf("expected nil map, got: %v", statuses)
	}
}
