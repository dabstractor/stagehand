package integrate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/ui"
)

// ---------------------------------------------------------------------------
// blockTarget — in-package test vehicle (no yaml/toml dep)
//
// A marker-delimited text block within an arbitrary file. Parse scans for a balanced
// START/END marker pair; Upsert ensures the block is present (replace, never duplicate);
// Remove deletes the block. Validate re-scans for balance on a throwaway instance.
// A badValidate flag injects a Validate failure for the backup/restore test.
// ---------------------------------------------------------------------------

type blockTarget struct {
	marker      string // "# stagecoach-test-marker"
	endMarker   string // "# end-stagecoach-test-marker"
	managedLine string // the line Upsert installs
	badValidate bool   // inject a Validate failure
	lines       []string
	hasEntry    bool
}

func newBlockTarget(managedLine string) *blockTarget {
	return &blockTarget{
		marker:      "# stagecoach-test-marker",
		endMarker:   "# end-stagecoach-test-marker",
		managedLine: managedLine,
	}
}

func (t *blockTarget) Marker() string { return t.marker }

func (t *blockTarget) Parse(data []byte) error {
	t.lines = strings.Split(string(data), "\n")
	start, end := -1, -1
	for i, l := range t.lines {
		if strings.TrimSpace(l) == t.marker {
			if start != -1 {
				return fmt.Errorf("duplicate start marker")
			}
			start = i
		}
		if strings.TrimSpace(l) == t.endMarker {
			end = i
		}
	}
	if start != -1 && end == -1 {
		return fmt.Errorf("unbalanced marker: %s without %s", t.marker, t.endMarker)
	}
	t.hasEntry = start != -1
	return nil
}

func (t *blockTarget) HasEntry() bool { return t.hasEntry }

func (t *blockTarget) Upsert() ([]byte, error) {
	var newLines []string
	if t.hasEntry {
		start, end := -1, -1
		for i, l := range t.lines {
			if strings.TrimSpace(l) == t.marker {
				start = i
			}
			if strings.TrimSpace(l) == t.endMarker {
				end = i
			}
		}
		newLines = append(newLines, t.lines[:start]...)
		newLines = append(newLines, t.marker, t.managedLine, t.endMarker)
		newLines = append(newLines, t.lines[end+1:]...)
	} else {
		newLines = append(newLines, t.lines...)
		newLines = append(newLines, t.marker, t.managedLine, t.endMarker)
	}
	return []byte(strings.Join(newLines, "\n")), nil
}

func (t *blockTarget) Remove() ([]byte, error) {
	if !t.hasEntry {
		return []byte(strings.Join(t.lines, "\n")), nil
	}
	start, end := -1, -1
	for i, l := range t.lines {
		if strings.TrimSpace(l) == t.marker {
			start = i
		}
		if strings.TrimSpace(l) == t.endMarker {
			end = i
		}
	}
	var newLines []string
	newLines = append(newLines, t.lines[:start]...)
	newLines = append(newLines, t.lines[end+1:]...)
	return []byte(strings.Join(newLines, "\n")), nil
}

func (t *blockTarget) Validate(data []byte) error {
	if t.badValidate {
		return fmt.Errorf("injected validation failure")
	}
	probe := &blockTarget{marker: t.marker, endMarker: t.endMarker}
	return probe.Parse(data)
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// applyHelper seeds a file (if seed != nil) and runs Apply, returning the result and
// the file bytes before and after (nil if the file doesn't exist at that point).
func applyHelper(t *testing.T, seed []byte, path string, target Target, action Action, yes bool, confirm ConfirmFunc) (ApplyResult, []byte, []byte) {
	t.Helper()
	if seed != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, seed, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var before []byte
	if seed != nil {
		var err error
		before, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	res, err := Apply(context.Background(), ApplyOptions{
		Path:    path,
		Target:  target,
		Action:  action,
		Yes:     yes,
		Out:     io.Discard,
		Confirm: confirm,
	})
	if err != nil {
		t.Fatalf("Apply unexpected error: %v", err)
	}
	var after []byte
	if _, statErr := os.Stat(path); statErr == nil {
		after, _ = os.ReadFile(path)
	}
	return res, before, after
}

// ---------------------------------------------------------------------------
// Test matrix
// ---------------------------------------------------------------------------

func TestApply_UpsertCreatesMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.txt") // parent dir doesn't exist

	target := newBlockTarget("managed-line")
	res, _, after := applyHelper(t, nil, path, target, ActionUpsert, true, nil)

	if res.Outcome != OutcomeCreated {
		t.Errorf("Outcome = %v, want OutcomeCreated", res.Outcome)
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty (missing file → no backup)", res.Backup)
	}
	if after == nil {
		t.Fatal("file was not created")
	}
	if !strings.Contains(string(after), "# stagecoach-test-marker") {
		t.Error("created file missing the marker")
	}
	if !strings.Contains(string(after), "managed-line") {
		t.Error("created file missing the managed line")
	}
	// Parent dir was created
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Error("parent dir was not created")
	}
}

func TestApply_UpsertIdempotentDoubleInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	// First install
	target1 := newBlockTarget("managed-line")
	res1, _, _ := applyHelper(t, nil, path, target1, ActionUpsert, true, nil)
	if res1.Outcome != OutcomeUpdated {
		t.Errorf("first install Outcome = %v, want OutcomeUpdated", res1.Outcome)
	}
	if res1.Backup == "" {
		t.Error("first install should have written a backup")
	}
	afterFirst, _ := os.ReadFile(path)

	// Second install (fresh target, same path)
	target2 := newBlockTarget("managed-line")
	res2, _, afterSecond := applyHelper(t, nil, path, target2, ActionUpsert, true, nil)
	if res2.Outcome != OutcomeNoChange {
		t.Errorf("second install Outcome = %v, want OutcomeNoChange", res2.Outcome)
	}
	if res2.Backup != "" {
		t.Errorf("second install Backup = %q, want empty (no change)", res2.Backup)
	}
	if !bytes.Equal(afterFirst, afterSecond) {
		t.Error("file changed on second install (should be idempotent)")
	}
	// Verify exactly one block
	if strings.Count(string(afterSecond), "# stagecoach-test-marker") != 1 {
		t.Error("expected exactly one start marker after double install")
	}
}

func TestApply_UpsertReplacesNotDuplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\n# stagecoach-test-marker\nold-managed-line\n# end-stagecoach-test-marker\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("new-managed-line")
	res, _, after := applyHelper(t, nil, path, target, ActionUpsert, true, nil)

	if res.Outcome != OutcomeUpdated {
		t.Errorf("Outcome = %v, want OutcomeUpdated", res.Outcome)
	}
	content := string(after)
	// Exactly one block, not two
	if strings.Count(content, "# stagecoach-test-marker") != 1 {
		t.Error("duplicate start marker (not idempotent)")
	}
	if strings.Count(content, "# end-stagecoach-test-marker") != 1 {
		t.Error("duplicate end marker (not idempotent)")
	}
	// Managed line replaced
	if !strings.Contains(content, "new-managed-line") {
		t.Error("new managed line not found")
	}
	if strings.Contains(content, "old-managed-line") {
		t.Error("old managed line still present")
	}
	// Surrounding lines preserved verbatim
	if !strings.Contains(content, "foreign-line-1") {
		t.Error("foreign-line-1 lost")
	}
	if !strings.Contains(content, "foreign-line-2") {
		t.Error("foreign-line-2 lost")
	}
	// File should be different from before (block content changed)
	if bytes.Equal(before, after) {
		t.Error("file unchanged (Upsert should have replaced the managed line)")
	}
}

func TestApply_RemoveDeletesOnlyEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\n# stagecoach-test-marker\nmanaged-line\n# end-stagecoach-test-marker\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	target := newBlockTarget("managed-line")
	res, _, after := applyHelper(t, nil, path, target, ActionRemove, true, nil)

	if res.Outcome != OutcomeRemoved {
		t.Errorf("Outcome = %v, want OutcomeRemoved", res.Outcome)
	}
	if res.Backup == "" {
		t.Error("Remove should have written a backup")
	}
	content := string(after)
	if strings.Contains(content, "# stagecoach-test-marker") {
		t.Error("marker still present after Remove")
	}
	if strings.Contains(content, "managed-line") {
		t.Error("managed line still present after Remove")
	}
	if !strings.Contains(content, "foreign-line-1") {
		t.Error("foreign-line-1 lost (surgical scope violated)")
	}
	if !strings.Contains(content, "foreign-line-2") {
		t.Error("foreign-line-2 lost (surgical scope violated)")
	}
}

func TestApply_RemoveMissingIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("managed-line")
	res, _, after := applyHelper(t, nil, path, target, ActionRemove, true, nil)

	if res.Outcome != OutcomeNoChange {
		t.Errorf("Outcome = %v, want OutcomeNoChange", res.Outcome)
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty (no change)", res.Backup)
	}
	if !bytes.Equal(before, after) {
		t.Error("file modified on Remove-no-op")
	}
}

func TestApply_CorruptInputRefusal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	// Unbalanced: START marker without END marker
	seed := []byte("# stagecoach-test-marker\nsome-content\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("managed-line")
	_, err := Apply(context.Background(), ApplyOptions{
		Path:   path,
		Target: target,
		Action: ActionUpsert,
		Yes:    true,
		Out:    io.Discard,
	})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("error = %v, want it to mention parse error", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("file was modified despite parse refusal")
	}
	// No backup written for parse refusal
	entries, _ := filepath.Glob(path + ".stagecoach-backup.*")
	if len(entries) != 0 {
		t.Errorf("backup written despite parse refusal: %v", entries)
	}
}

func TestApply_DeclineWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newdir", "config.txt") // parent doesn't exist, file doesn't exist

	target := newBlockTarget("managed-line")
	decline := func(_ io.Writer, _ string, _ string) bool { return false }
	res, _, after := applyHelper(t, nil, path, target, ActionUpsert, false, decline)

	if res.Outcome != OutcomeDeclined {
		t.Errorf("Outcome = %v, want OutcomeDeclined", res.Outcome)
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty (declined)", res.Backup)
	}
	if after != nil {
		t.Error("file was created despite decline")
	}
	// Parent dir should NOT have been created (MkdirAll runs AFTER confirm)
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Error("parent dir was created despite decline")
	}
}

func TestApply_DeclineExistingFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("managed-line")
	decline := func(_ io.Writer, _ string, _ string) bool { return false }
	res, _, after := applyHelper(t, nil, path, target, ActionUpsert, false, decline)

	if res.Outcome != OutcomeDeclined {
		t.Errorf("Outcome = %v, want OutcomeDeclined", res.Outcome)
	}
	if !bytes.Equal(before, after) {
		t.Error("existing file was modified despite decline")
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty (declined)", res.Backup)
	}
}

func TestApply_BackupWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("managed-line")
	res, _, _ := applyHelper(t, nil, path, target, ActionUpsert, true, nil)

	if res.Outcome != OutcomeUpdated {
		t.Errorf("Outcome = %v, want OutcomeUpdated", res.Outcome)
	}
	if res.Backup == "" {
		t.Fatal("expected a backup to be written")
	}
	// Backup file exists and equals orig
	backupData, err := os.ReadFile(res.Backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backupData, before) {
		t.Error("backup content doesn't match original file content")
	}
	// Target file has the block
	targetData, _ := os.ReadFile(path)
	if !strings.Contains(string(targetData), "# stagecoach-test-marker") {
		t.Error("target file missing the block after write")
	}
}

func TestApply_ValidateFailureRestoresBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	seed := []byte("foreign-line-1\nforeign-line-2\n")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	before, _ := os.ReadFile(path)

	target := newBlockTarget("managed-line")
	target.badValidate = true

	res, err := Apply(context.Background(), ApplyOptions{
		Path:   path,
		Target: target,
		Action: ActionUpsert,
		Yes:    true,
		Out:    io.Discard,
	})
	if err == nil {
		t.Fatal("expected validation failure error, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %v, want it to mention validation failed", err)
	}
	// Target file restored to orig
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("file not restored after validation failure")
	}
	// Backup retained
	if res.Backup == "" {
		t.Error("backup should have been written before the failed validation")
	}
	_, backupErr := os.Stat(res.Backup)
	if backupErr != nil {
		t.Errorf("backup file not found at %q: %v", res.Backup, backupErr)
	}
}

func TestDefaultConfirm_NonTTYDeclines(t *testing.T) {
	// This test verifies non-TTY auto-decline. It only runs when stdin is not a TTY
	// (the normal case for `go test` in CI/pipelines). When stdin IS a TTY, the
	// DefaultConfirm enters the interactive prompt path — skip to avoid blocking.
	if ui.IsTerminal(os.Stdin) {
		t.Skip("stdin is a TTY; non-TTY auto-decline only applies in non-interactive contexts")
	}
	var buf bytes.Buffer
	result := DefaultConfirm(&buf, "/tmp/test.txt", "some diff content")
	if result != false {
		t.Error("DefaultConfirm should auto-decline when stdin is not a TTY")
	}
	if !strings.Contains(buf.String(), "non-interactive") {
		t.Errorf("expected non-interactive decline message, got: %q", buf.String())
	}
}

func TestDefaultConfirm_ShowsDiff(t *testing.T) {
	// Non-TTY path — verify diff is printed before the decline message.
	var buf bytes.Buffer
	diff := "--- a/file\n+++ b/file\n@@ -1 +1 @@\n-old\n+new\n"
	DefaultConfirm(&buf, "/tmp/test.txt", diff)
	output := buf.String()
	if !strings.HasPrefix(output, diff) {
		t.Errorf("expected diff to be printed first, got: %q", output)
	}
}

func TestPreviewDiff_NonEmptyIffChanged(t *testing.T) {
	ctx := context.Background()
	path := "/tmp/config.txt"

	// Identical content → empty diff
	orig := []byte("line1\nline2\n")
	diff, err := previewDiff(ctx, path, orig, orig, false)
	if err != nil {
		t.Fatalf("identical diff: %v", err)
	}
	if diff != "" {
		t.Errorf("identical files should yield empty diff, got: %q", diff)
	}

	// Changed content → non-empty diff with +/- lines
	changed := []byte("line1\nLINE2\n")
	diff, err = previewDiff(ctx, path, orig, changed, false)
	if err != nil {
		t.Fatalf("changed diff: %v", err)
	}
	if diff == "" {
		t.Error("changed files should yield non-empty diff")
	}
	if !strings.Contains(diff, "-line2") || !strings.Contains(diff, "+LINE2") {
		t.Errorf("diff should contain +/- lines, got: %q", diff)
	}

	// Create-if-missing (oldMissing=true) → diff shows all lines added
	newContent := []byte("new-line-1\nnew-line-2\n")
	diff, err = previewDiff(ctx, path, nil, newContent, true)
	if err != nil {
		t.Fatalf("create-if-missing diff: %v", err)
	}
	if diff == "" {
		t.Error("create-if-missing should yield non-empty diff")
	}
	if !strings.Contains(diff, "+new-line-1") {
		t.Errorf("diff should show added lines, got: %q", diff)
	}
}

func TestBackupPath_Format(t *testing.T) {
	got := BackupPath("/x/y", 1700000000)
	want := "/x/y.stagecoach-backup.1700000000"
	if got != want {
		t.Errorf("BackupPath = %q, want %q", got, want)
	}
}

func TestOutcome_String(t *testing.T) {
	tests := []struct {
		o    Outcome
		want string
	}{
		{OutcomeCreated, "Created"},
		{OutcomeUpdated, "Updated"},
		{OutcomeRemoved, "Removed"},
		{OutcomeDeclined, "Declined"},
		{OutcomeNoChange, "NoChange"},
	}
	for _, tt := range tests {
		if got := tt.o.String(); got != tt.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", tt.o, got, tt.want)
		}
	}
}

func TestApply_RemoveMissingFileIsNoOp(t *testing.T) {
	// Remove on a completely missing file → NoChange, nothing written
	// Also verifies Remove of an existing file without the block (different from missing file)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt") // doesn't exist

	target := newBlockTarget("managed-line")
	res, err := Apply(context.Background(), ApplyOptions{
		Path:   path,
		Target: target,
		Action: ActionRemove,
		Yes:    true,
		Out:    io.Discard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != OutcomeNoChange {
		t.Errorf("Outcome = %v, want OutcomeNoChange", res.Outcome)
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty", res.Backup)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Remove on missing file")
	}
}
