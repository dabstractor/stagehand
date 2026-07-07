package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/integrate"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newIsolatedLazygitEntry builds a lazygitEntry with an explicit tmp configPath.
// Tests NEVER touch the real ~/.config/lazygit/config.yml.
func newIsolatedLazygitEntry(t *testing.T, key string) *lazygitEntry {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.yml")
	k := key
	if k == "" {
		k = defaultLazygitKey
	}
	return &lazygitEntry{configPath: cfg, key: k}
}

// readFixture reads a testdata file relative to the package directory.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "lazygit", name))
	if err != nil {
		t.Fatalf("read testdata/lazygit/%s: %v", name, err)
	}
	return data
}

// ---------------------------------------------------------------------------
// lazygitTarget tests (direct method calls, no Apply, no cobra)
// ---------------------------------------------------------------------------

func TestLazygitTarget_ParseGolden(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}

	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse(golden): %v", err)
	}
	if tgt.HasEntry() {
		t.Error("HasEntry() = true, want false (golden has no stagehand marker)")
	}
	// customCommands should exist with one pre-existing entry
	top := tgt.topMap()
	seq := tgt.locateSeq(top)
	if seq == nil {
		t.Fatal("locateSeq returned nil — customCommands missing from golden")
	}
	if len(seq.Content) != 1 {
		t.Errorf("customCommands has %d entries, want 1 (pre-existing 'b' entry)", len(seq.Content))
	}
}

func TestLazygitTarget_Upsert_AddsEntrySemantically(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Re-parse the output to assert semantic content.
	var outRoot yaml.Node
	if err := yaml.Unmarshal(out, &outRoot); err != nil {
		t.Fatalf("re-parse output: %v", err)
	}
	outTgt := &lazygitTarget{key: defaultLazygitKey, root: &outRoot}

	// (a) marked entry present with correct defaults
	if !outTgt.HasEntry() {
		t.Fatal("output HasEntry() = false, want true (stagehand entry should be present)")
	}
	marked := outTgt.findMarkedItem()
	if marked == nil {
		t.Fatal("findMarkedItem returned nil")
	}
	// Extract the marked entry's fields via pair-walking.
	fields := map[string]string{}
	for i := 0; i+1 < len(marked.Content); i += 2 {
		fields[marked.Content[i].Value] = marked.Content[i+1].Value
	}
	if fields["key"] != "<c-a>" {
		t.Errorf("marked key = %q, want '<c-a>'", fields["key"])
	}
	if fields["context"] != "files" {
		t.Errorf("marked context = %q, want 'files'", fields["context"])
	}
	if fields["command"] != "stagehand" {
		t.Errorf("marked command = %q, want 'stagehand'", fields["command"])
	}
	if fields["output"] != "none" {
		t.Errorf("marked output = %q, want 'none'", fields["output"])
	}
	if fields["loadingText"] != "Generating commit message…" {
		t.Errorf("marked loadingText = %q, want 'Generating commit message…'", fields["loadingText"])
	}
	if fields["description"] != "stagehand: AI commit" {
		t.Errorf("marked description = %q, want 'stagehand: AI commit'", fields["description"])
	}

	// (b) pre-existing 'b' entry preserved
	outSeq := outTgt.locateSeq(outTgt.topMap())
	foundExisting := false
	for _, it := range outSeq.Content {
		if it.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(it.Content); j += 2 {
			if it.Content[j].Value == "key" && it.Content[j+1].Value == "b" {
				foundExisting = true
			}
		}
	}
	if !foundExisting {
		t.Error("pre-existing 'b' entry not preserved after Upsert")
	}

	// (c) other top-level blocks present
	top := outTgt.topMap()
	topKeys := map[string]bool{}
	for i := 0; i+1 < len(top.Content); i += 2 {
		topKeys[top.Content[i].Value] = true
	}
	for _, want := range []string{"gui", "git", "customCommands"} {
		if !topKeys[want] {
			t.Errorf("top-level key %q missing after Upsert", want)
		}
	}

	// (d) marker substring present in output
	if !strings.Contains(string(out), "stagecoach-integration") {
		t.Error("output missing 'stagecoach-integration' marker")
	}
}

func TestLazygitTarget_Upsert_IdempotentStableRoundTrip(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	first, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert (1st): %v", err)
	}

	// Re-parse and Upsert again
	tgt2 := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt2.Parse(first); err != nil {
		t.Fatalf("Parse (2nd): %v", err)
	}
	second, err := tgt2.Upsert()
	if err != nil {
		t.Fatalf("Upsert (2nd): %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("idempotent round-trip: first != second (len %d vs %d)", len(first), len(second))
	}
}

func TestLazygitTarget_Upsert_ReplaceNotDuplicate(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// First upsert adds the entry.
	firstOut, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert (1st): %v", err)
	}

	// Parse the first output, then Upsert again (replace path).
	tgt2 := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt2.Parse(firstOut); err != nil {
		t.Fatalf("Parse (2nd): %v", err)
	}
	// Count items before second Upsert
	seqBefore := tgt2.locateSeq(tgt2.topMap())
	countBefore := len(seqBefore.Content)

	secondOut, err := tgt2.Upsert()
	if err != nil {
		t.Fatalf("Upsert (2nd): %v", err)
	}

	// Re-parse and assert still exactly one marked entry, no duplicates.
	var outRoot yaml.Node
	if err := yaml.Unmarshal(secondOut, &outRoot); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	outTgt := &lazygitTarget{key: defaultLazygitKey, root: &outRoot}
	seqAfter := outTgt.locateSeq(outTgt.topMap())
	if len(seqAfter.Content) != countBefore {
		t.Errorf("sequence length changed: %d → %d (should be same — replace, not duplicate)", countBefore, len(seqAfter.Content))
	}
	markedCount := 0
	for _, it := range seqAfter.Content {
		if isStagecoachItem(it) {
			markedCount++
		}
	}
	if markedCount != 1 {
		t.Errorf("marked items after 2nd Upsert: %d, want 1 (replace, never duplicate)", markedCount)
	}
}

func TestLazygitTarget_Remove_DeletesOnlyMarked(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Upsert first to add the stagehand entry.
	upserted, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Now parse the upserted content and Remove.
	tgt2 := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt2.Parse(upserted); err != nil {
		t.Fatalf("Parse (upserted): %v", err)
	}
	removed, err := tgt2.Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Re-parse the removed content.
	var outRoot yaml.Node
	if err := yaml.Unmarshal(removed, &outRoot); err != nil {
		t.Fatalf("re-parse removed: %v", err)
	}
	outTgt := &lazygitTarget{key: defaultLazygitKey, root: &outRoot}

	if outTgt.HasEntry() {
		t.Error("HasEntry() = true after Remove, want false (marker should be gone)")
	}

	// Sibling 'b' entry should be preserved.
	seq := outTgt.locateSeq(outTgt.topMap())
	if seq == nil {
		t.Fatal("customCommands missing after Remove")
	}
	foundB := false
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(it.Content); j += 2 {
			if it.Content[j].Value == "key" && it.Content[j+1].Value == "b" {
				foundB = true
			}
		}
	}
	if !foundB {
		t.Error("pre-existing 'b' entry lost after Remove")
	}
}

func TestLazygitTarget_Remove_EmptySeq(t *testing.T) {
	// Build a config with ONLY the stagehand entry (no sibling).
	yamlStr := `customCommands:
  - key: '<c-a>' # stagecoach-integration
    context: 'files'
    command: 'stagehand'
    loadingText: 'Generating commit message…'
    output: 'none'
    description: 'stagehand: AI commit'
`
	tgt := &lazygitTarget{key: "<c-a>"}
	if err := tgt.Parse([]byte(yamlStr)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !tgt.HasEntry() {
		t.Fatal("precondition: HasEntry should be true")
	}

	out, err := tgt.Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Re-parse and assert HasEntry is false.
	outTgt := &lazygitTarget{key: "<c-a>"}
	if err := outTgt.Parse(out); err != nil {
		t.Fatalf("Parse output: %v", err)
	}
	if outTgt.HasEntry() {
		t.Error("HasEntry() = true after removing only entry, want false")
	}
}

func TestLazygitTarget_Remove_NoMarkerUnchanged(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := tgt.Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Re-parse and assert no stagehand entry.
	var outRoot yaml.Node
	if err := yaml.Unmarshal(out, &outRoot); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	outTgt := &lazygitTarget{key: defaultLazygitKey, root: &outRoot}
	if outTgt.HasEntry() {
		t.Error("Remove on a clean file should not add a stagehand entry")
	}
}

func TestLazygitTarget_ParseCorruptRefuses(t *testing.T) {
	corrupt := readFixture(t, "golden_corrupt.yml")
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse(corrupt); err == nil {
		t.Fatal("Parse(corrupt): err=nil, want non-nil (corrupt YAML should be refused)")
	}
}

func TestLazygitTarget_Validate(t *testing.T) {
	tgt := &lazygitTarget{key: defaultLazygitKey}

	// Well-formed bytes → nil
	wellFormed := []byte("key: value\n")
	if err := tgt.Validate(wellFormed); err != nil {
		t.Errorf("Validate(well-formed): %v, want nil", err)
	}

	// Corrupt bytes → non-nil
	corrupt := readFixture(t, "golden_corrupt.yml")
	if err := tgt.Validate(corrupt); err == nil {
		t.Error("Validate(corrupt): err=nil, want non-nil")
	}
}

func TestLazygitTarget_CreateIfMissing_EmptyFile(t *testing.T) {
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse([]byte("")); err != nil {
		t.Fatalf("Parse(empty): %v", err)
	}

	out, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Re-parse and assert valid with the stagehand entry.
	outTgt := &lazygitTarget{key: defaultLazygitKey}
	if err := outTgt.Parse(out); err != nil {
		t.Fatalf("Parse output: %v", err)
	}
	if !outTgt.HasEntry() {
		t.Error("HasEntry() = false after Upsert on empty file, want true")
	}
	seq := outTgt.locateSeq(outTgt.topMap())
	if seq == nil {
		t.Fatal("customCommands missing after Upsert on empty file")
	}
	if len(seq.Content) != 1 {
		t.Errorf("customCommands has %d entries, want 1", len(seq.Content))
	}
}

func TestLazygitTarget_CreateIfMissing_AbsentKey(t *testing.T) {
	// Config with gui and git but NO customCommands.
	noCC := `gui:
  showRandomTip: false
git:
  autoFetch: false
`
	tgt := &lazygitTarget{key: defaultLazygitKey}
	if err := tgt.Parse([]byte(noCC)); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Re-parse and assert customCommands exists AND original keys preserved.
	var outRoot yaml.Node
	if err := yaml.Unmarshal(out, &outRoot); err != nil {
		t.Fatalf("re-parse absent-key: %v", err)
	}
	outTgt := &lazygitTarget{key: defaultLazygitKey, root: &outRoot}

	if !outTgt.HasEntry() {
		t.Error("HasEntry() = false, want true")
	}
	seq := outTgt.locateSeq(outTgt.topMap())
	if seq == nil {
		t.Fatal("customCommands missing after Upsert")
	}

	top := outTgt.topMap()
	topKeys := map[string]bool{}
	for i := 0; i+1 < len(top.Content); i += 2 {
		topKeys[top.Content[i].Value] = true
	}
	if !topKeys["gui"] {
		t.Error("gui block lost after appending customCommands")
	}
	if !topKeys["git"] {
		t.Error("git block lost after appending customCommands")
	}
	if !topKeys["customCommands"] {
		t.Error("customCommands key not present after Upsert")
	}
}

func TestLazygitTarget_CustomKey(t *testing.T) {
	golden := readFixture(t, "golden_input.yml")
	tgt := &lazygitTarget{key: "<c-s>"}
	if err := tgt.Parse(golden); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	out, err := tgt.Upsert()
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	var outRoot yaml.Node
	if err := yaml.Unmarshal(out, &outRoot); err != nil {
		t.Fatalf("re-parse custom key: %v", err)
	}
	outTgt := &lazygitTarget{key: "<c-s>", root: &outRoot}
	if !outTgt.HasEntry() {
		t.Fatal("HasEntry() = false")
	}
	marked := outTgt.findMarkedItem()
	// Extract key value
	for i := 0; i+1 < len(marked.Content); i += 2 {
		if marked.Content[i].Value == "key" {
			if marked.Content[i+1].Value != "<c-s>" {
				t.Errorf("key = %q, want '<c-s>'", marked.Content[i+1].Value)
			}
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Apply / Entry tests (real Apply over tmp files)
// ---------------------------------------------------------------------------

func TestLazygitEntry_Install_Creates(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome = %v, want Created", res.Outcome)
	}
	if res.Target != lazygitTargetName {
		t.Errorf("Target = %q, want %q", res.Target, lazygitTargetName)
	}
	if res.Backup != "" {
		t.Errorf("Backup = %q, want empty (created, not modified)", res.Backup)
	}

	// File should exist with the marker.
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "stagecoach-integration") {
		t.Error("config missing stagecoach-integration marker")
	}
}

func TestLazygitEntry_Install_Updated(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	// Pre-write the golden fixture.
	if err := os.WriteFile(e.configPath, readFixture(t, "golden_input.yml"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ctx := context.Background()
	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeUpdated {
		t.Errorf("Outcome = %v, want Updated", res.Outcome)
	}
	if res.Backup == "" {
		t.Error("Backup is empty, want non-empty (existing-file modification)")
	}
	if !strings.Contains(res.Backup, ".stagehand-backup.") {
		t.Errorf("Backup path = %q, want '.stagehand-backup.' pattern", res.Backup)
	}

	// Entry should be present.
	data, _ := os.ReadFile(e.configPath)
	if !strings.Contains(string(data), "stagecoach-integration") {
		t.Error("config missing stagecoach-integration marker after install")
	}
}

func TestLazygitEntry_Install_IdempotentNoChange(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	// First install.
	res1, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install (1st): %v", err)
	}
	if res1.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome (1st) = %v, want Created", res1.Outcome)
	}

	// Second install → NoChange.
	res2, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install (2nd): %v", err)
	}
	if res2.Outcome != integrate.OutcomeNoChange {
		t.Errorf("Outcome (2nd) = %v, want NoChange", res2.Outcome)
	}
}

func TestLazygitEntry_Install_DeclineWritesNothing(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	res, err := e.Install(ctx, integrate.InstallOptions{
		Yes:     false,
		Confirm: func(_ io.Writer, _ string, _ string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Install (decline): %v", err)
	}
	if res.Outcome != integrate.OutcomeDeclined {
		t.Errorf("Outcome = %v, want Declined", res.Outcome)
	}

	// File should not exist (or be unchanged).
	if _, err := os.Stat(e.configPath); err == nil {
		t.Error("file exists after decline, want absent/unchanged")
	}
}

func TestLazygitEntry_Install_ConfirmReceivesDiff(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	var gotDiff string
	res, err := e.Install(ctx, integrate.InstallOptions{
		Yes: false,
		Confirm: func(_ io.Writer, _ string, diff string) bool {
			gotDiff = diff
			return true
		},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome = %v, want Created", res.Outcome)
	}

	// The diff should contain stagehand-related content.
	if !strings.Contains(gotDiff, "stagecoach-integration") {
		t.Errorf("diff missing 'stagecoach-integration'; got %q", gotDiff)
	}
	if !strings.Contains(gotDiff, "customCommands") {
		t.Errorf("diff missing 'customCommands'; got %q", gotDiff)
	}
}

func TestLazygitEntry_Install_ForeignKeyWarning(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "") // key defaults to <c-a>
	foreignYAML := `customCommands:
  - key: '<c-a>'
    command: 'other-tool'
    context: 'files'
`
	if err := os.WriteFile(e.configPath, []byte(foreignYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	res, err := e.Install(context.Background(), integrate.InstallOptions{Yes: true, Out: &buf})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("output missing WARNING; got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "duplicate") {
		t.Errorf("output missing 'duplicate'; got %q", buf.String())
	}
	// CONTRACT CORRECTION: Apply reports an existing-file modification as Updated, NOT Created
	// (OutcomeCreated is reserved for the missing-file path — protocol.go:228-235).
	if res.Outcome != integrate.OutcomeUpdated {
		t.Errorf("Outcome = %v, want Updated", res.Outcome)
	}

	// The config must contain exactly TWO <c-a> entries: the foreign (unmarked) + stagehand's marked one.
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if n := strings.Count(string(data), "key: '<c-a>'"); n != 2 {
		t.Errorf("want exactly 2 customCommands entries with key '<c-a>', got %d.\nconfig:\n%s", n, data)
	}
	if !strings.Contains(string(data), "stagecoach-integration") {
		t.Error("config missing stagecoach-integration marker")
	}
}

func TestLazygitEntry_Install_NoForeignNoWarning(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	// golden_input.yml's only customCommands key is 'b' — no <c-a> conflict.
	if err := os.WriteFile(e.configPath, readFixture(t, "golden_input.yml"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	_, err := e.Install(context.Background(), integrate.InstallOptions{Yes: true, Out: &buf})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if strings.Contains(buf.String(), "WARNING") {
		t.Errorf("output should NOT contain WARNING for a clean config; got %q", buf.String())
	}
}

func TestLazygitEntry_Install_ForeignKeyWarningInteractiveConfirm(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	foreignYAML := `customCommands:
  - key: '<c-a>'
    command: 'other-tool'
    context: 'files'
`
	if err := os.WriteFile(e.configPath, []byte(foreignYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	var gotDiff string
	_, err := e.Install(context.Background(), integrate.InstallOptions{
		Yes: false,
		Out: &buf,
		Confirm: func(_ io.Writer, _ string, diff string) bool {
			gotDiff = diff
			return true
		},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	// WARNING surfaces on opts.Out (our buf); Apply's diff surfaces in the Confirm func's diff arg.
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("opts.Out missing WARNING; got %q", buf.String())
	}
	if !strings.Contains(gotDiff, "stagecoach-integration") {
		t.Errorf("confirm diff missing stagehand entry; got %q", gotDiff)
	}
}

func TestLazygitEntry_Install_CorruptRefuses(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	// Pre-write corrupt content.
	corrupt := readFixture(t, "golden_corrupt.yml")
	if err := os.WriteFile(e.configPath, corrupt, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ctx := context.Background()
	_, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err == nil {
		t.Fatal("Install(corrupt): err=nil, want error (parse refusal)")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("error = %q, want 'parse error'", err.Error())
	}

	// File should be UNCHANGED.
	data, _ := os.ReadFile(e.configPath)
	if !bytes.Equal(data, corrupt) {
		t.Error("corrupt config was modified by install, want unchanged")
	}
}

func TestLazygitEntry_Remove_Removed(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	// Install first.
	res1, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil || res1.Outcome != integrate.OutcomeCreated {
		t.Fatalf("Install: outcome=%v err=%v", res1.Outcome, err)
	}

	// Now remove.
	res2, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if res2.Outcome != integrate.OutcomeRemoved {
		t.Errorf("Remove Outcome = %v, want Removed", res2.Outcome)
	}
	if res2.Backup == "" {
		t.Error("Backup is empty, want non-empty (existing-file modification)")
	}

	// Re-read and assert no marker.
	data, _ := os.ReadFile(e.configPath)
	if strings.Contains(string(data), "stagecoach-integration") {
		t.Error("marker still present after Remove")
	}
}

func TestLazygitEntry_Remove_NoChange(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	res, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if res.Outcome != integrate.OutcomeNoChange {
		t.Errorf("Outcome = %v, want NoChange (nothing to remove)", res.Outcome)
	}
}

func TestLazygitEntry_Status_States(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	// Missing file → NotInstalled
	s, err := e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (missing): %v", err)
	}
	if s != integrate.StatusNotInstalled {
		t.Errorf("Status (missing) = %v, want NotInstalled", s)
	}

	// Install → Installed
	if _, err := e.Install(ctx, integrate.InstallOptions{Yes: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	s, err = e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (installed): %v", err)
	}
	if s != integrate.StatusInstalled {
		t.Errorf("Status (installed) = %v, want Installed", s)
	}

	// Hand-write an unmarked entry with key "<c-a>" → Foreign
	// Remove the stagehand entry first, then write a foreign one.
	if _, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	foreignYAML := `customCommands:
  - key: '<c-a>'
    command: 'other-tool'
    context: 'files'
`
	if err := os.WriteFile(e.configPath, []byte(foreignYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err = e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (foreign): %v", err)
	}
	if s != integrate.StatusForeign {
		t.Errorf("Status (foreign) = %v, want Foreign", s)
	}

	// Clean golden (no customCommands with our key) → NotInstalled
	os.Remove(e.configPath)
	if err := os.WriteFile(e.configPath, readFixture(t, "golden_input.yml"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err = e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (clean golden): %v", err)
	}
	if s != integrate.StatusNotInstalled {
		t.Errorf("Status (clean golden) = %v, want NotInstalled", s)
	}
}

func TestLazygitEntry_Detect(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	// lazygit may or may not be on PATH in the test env.
	err := e.Detect(ctx)
	if err != nil {
		// If lazygit is not installed, that's OK in the test env
		if !strings.Contains(err.Error(), "lazygit not found") {
			t.Errorf("Detect err = %v, want 'lazygit not found' or nil", err)
		}
	}

	// Clear PATH → should fail
	e2 := &lazygitEntry{configPath: e.configPath, key: e.key}
	t.Setenv("PATH", "")
	err = e2.Detect(ctx)
	if err == nil {
		t.Fatal("Detect (empty PATH): err=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "lazygit not found") {
		t.Errorf("Detect err = %v, want 'lazygit not found'", err)
	}
}

func TestLazygitEntry_ConfigPath(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	p, err := e.ConfigPath(context.Background())
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if p != e.configPath {
		t.Errorf("ConfigPath = %q, want %q", p, e.configPath)
	}
}

// ---------------------------------------------------------------------------
// Dispatch + --key wiring tests
// ---------------------------------------------------------------------------

func TestIntegrateInstall_Lazygit_Dispatch(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{e})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: true, Out: &stderr, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"lazygit"}, opts, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dispatchInstall: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Installed lazygit") {
		t.Errorf("output missing 'Installed lazygit'; got %q", got)
	}

	// Verify the file has the marker.
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "stagecoach-integration") {
		t.Error("config missing stagecoach-integration marker")
	}
}

func TestIntegrateRemove_Lazygit_Dispatch(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	ctx := context.Background()

	// Install first.
	if _, err := e.Install(ctx, integrate.InstallOptions{Yes: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove via dispatch.
	reg := integrate.NewRegistry([]integrate.Entry{e})
	var stdout, stderr bytes.Buffer
	err := dispatchRemove(ctx, reg, []string{"lazygit"}, integrate.RemoveOptions{Yes: true, Out: &stderr}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dispatchRemove: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Removed lazygit") {
		t.Errorf("output missing 'Removed lazygit'; got %q", got)
	}

	// Verify marker gone.
	data, _ := os.ReadFile(e.configPath)
	if strings.Contains(string(data), "stagecoach-integration") {
		t.Error("marker still present after Remove")
	}
}

func TestIntegrateLazygitKeyFlag(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "<c-s>")
	ctx := context.Background()

	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome = %v, want Created", res.Outcome)
	}

	// Read config and assert custom key.
	data, _ := os.ReadFile(e.configPath)
	if !strings.Contains(string(data), "key: '<c-s>'") {
		t.Errorf("config missing '<c-s>' key; got %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// isStagecoachItem unit test
// ---------------------------------------------------------------------------

func TestIsStagecoachItem(t *testing.T) {
	// Build a marked entry via the template.
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(entryTpl, "<c-a>")), &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	item := doc.Content[0].Content[0] // SequenceNode → MappingNode
	if !isStagecoachItem(item) {
		t.Error("isStagecoachItem(template entry) = false, want true")
	}

	// Unmarked entry should not match.
	unmarked := `customCommands:
  - key: '<c-a>'
    command: 'other'
`
	var doc2 yaml.Node
	if err := yaml.Unmarshal([]byte(unmarked), &doc2); err != nil {
		t.Fatalf("parse unmarked: %v", err)
	}
	// doc2.Content[0] is the top map; Content[1] is the sequence; Content[0] is the item.
	seq := doc2.Content[0].Content[1]
	if len(seq.Content) == 0 || isStagecoachItem(seq.Content[0]) {
		t.Error("isStagecoachItem(unmarked) = true, want false")
	}
}
