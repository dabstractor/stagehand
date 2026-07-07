package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Detect
// ---------------------------------------------------------------------------

func TestDetect_None(t *testing.T) {
	dir := t.TempDir()
	st, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect err=%v", err)
	}
	if st != StatusNone {
		t.Errorf("Detect = %v, want StatusNone", st)
	}
}

func TestDetect_Stagecoach(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, HookFilename)
	if err := os.WriteFile(p, []byte(hookScript(false, "")), ScriptMode); err != nil {
		t.Fatal(err)
	}
	st, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect err=%v", err)
	}
	if st != StatusStagecoach {
		t.Errorf("Detect = %v, want StatusStagecoach", st)
	}
}

func TestDetect_Foreign(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, HookFilename)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect err=%v", err)
	}
	if st != StatusForeign {
		t.Errorf("Detect = %v, want StatusForeign", st)
	}
}

func TestDetect_EmptyFileIsForeign(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, HookFilename)
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect err=%v", err)
	}
	if st != StatusForeign {
		t.Errorf("Detect = %v, want StatusForeign (empty file has no marker)", st)
	}
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_Fresh(t *testing.T) {
	dir := t.TempDir()
	prev, err := Install(dir, false, "")
	if err != nil {
		t.Fatalf("Install err=%v", err)
	}
	if prev != StatusNone {
		t.Errorf("Install prev=%v, want StatusNone", prev)
	}
	// File exists and is executable
	info, err := os.Stat(filepath.Join(dir, HookFilename))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != ScriptMode {
		t.Errorf("file perm = %o, want %o", info.Mode().Perm(), ScriptMode)
	}
	data, _ := os.ReadFile(filepath.Join(dir, HookFilename))
	if string(data) != hookScript(false, "") {
		t.Errorf("file content mismatch")
	}
}

func TestInstall_IdempotentReinstall(t *testing.T) {
	dir := t.TempDir()
	_, _ = Install(dir, false, "")

	prev2, err := Install(dir, false, "")
	if err != nil {
		t.Fatalf("re-Install err=%v", err)
	}
	if prev2 != StatusStagecoach {
		t.Errorf("re-Install prev=%v, want StatusStagecoach", prev2)
	}
	data, _ := os.ReadFile(filepath.Join(dir, HookFilename))
	if string(data) != hookScript(false, "") {
		t.Errorf("content changed on reinstall")
	}
	info, _ := os.Stat(filepath.Join(dir, HookFilename))
	if info.Mode().Perm() != ScriptMode {
		t.Errorf("file perm = %o, want %o after reinstall", info.Mode().Perm(), ScriptMode)
	}
}

func TestInstall_Strict(t *testing.T) {
	dir := t.TempDir()
	_, err := Install(dir, true, "")
	if err != nil {
		t.Fatalf("Install strict err=%v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, HookFilename))
	if string(data) != hookScript(true, "") {
		t.Errorf("strict content mismatch")
	}
	if !strings.Contains(string(data), "--strict") {
		t.Error("strict script missing --strict")
	}
}

func TestInstall_ForeignRefusal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, HookFilename)
	foreignContent := []byte("#!/bin/sh\necho mine\n")
	if err := os.WriteFile(p, foreignContent, 0o644); err != nil {
		t.Fatal(err)
	}
	foreignBefore, _ := os.ReadFile(p)

	prev, err := Install(dir, false, "")
	if !errors.Is(err, ErrForeignHook) {
		t.Fatalf("Install err=%v, want ErrForeignHook", err)
	}
	if prev != StatusForeign {
		t.Errorf("Install prev=%v, want StatusForeign", prev)
	}

	// Never-clobber invariant: file bytes unchanged
	foreignAfter, _ := os.ReadFile(p)
	if string(foreignBefore) != string(foreignAfter) {
		t.Error("foreign file was modified (never-clobber violated)")
	}
}

func TestInstall_CreatesHooksDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested", "hooks")
	_, err := Install(nested, false, "")
	if err != nil {
		t.Fatalf("Install nested err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, HookFilename)); err != nil {
		t.Error("hook file not created in nested dir")
	}
}

// ---------------------------------------------------------------------------
// Uninstall
// ---------------------------------------------------------------------------

func TestUninstall_RemovesOurs(t *testing.T) {
	dir := t.TempDir()
	_, _ = Install(dir, false, "")

	st, err := Uninstall(dir)
	if err != nil {
		t.Fatalf("Uninstall err=%v", err)
	}
	if st != StatusStagecoach {
		t.Errorf("Uninstall st=%v, want StatusStagecoach", st)
	}
	if _, err := os.Stat(filepath.Join(dir, HookFilename)); !os.IsNotExist(err) {
		t.Error("hook file not removed")
	}
}

func TestUninstall_ForeignRefusal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, HookFilename)
	foreignContent := []byte("#!/bin/sh\necho mine\n")
	if err := os.WriteFile(p, foreignContent, 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := Uninstall(dir)
	if !errors.Is(err, ErrForeignHook) {
		t.Fatalf("Uninstall err=%v, want ErrForeignHook", err)
	}
	if st != StatusForeign {
		t.Errorf("Uninstall st=%v, want StatusForeign", st)
	}
	foreignAfter, _ := os.ReadFile(p)
	if string(foreignContent) != string(foreignAfter) {
		t.Error("foreign file was modified by uninstall")
	}
}

func TestUninstall_NoneIsErrNoHook(t *testing.T) {
	dir := t.TempDir()
	st, err := Uninstall(dir)
	if !errors.Is(err, ErrNoHook) {
		t.Fatalf("Uninstall err=%v, want ErrNoHook", err)
	}
	if st != StatusNone {
		t.Errorf("Uninstall st=%v, want StatusNone", st)
	}
}

// ---------------------------------------------------------------------------
// Status.String()
// ---------------------------------------------------------------------------

func TestStatus_String(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusNone, "none"},
		{StatusStagecoach, "stagecoach (v1)"},
		{StatusForeign, "foreign"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Script / InvocationLine drift guards
// ---------------------------------------------------------------------------

func TestScript_MatchesHookScript(t *testing.T) {
	if Script(false, "") != hookScript(false, "") {
		t.Error(`Script(false, "") != hookScript(false, "")`)
	}
	if Script(true, "") != hookScript(true, "") {
		t.Error(`Script(true, "") != hookScript(true, "")`)
	}
}

func TestInvocationLine_InScript(t *testing.T) {
	for _, strict := range []bool{false, true} {
		script := hookScript(strict, "")
		il := InvocationLine(strict)
		if !strings.Contains(script, il) {
			t.Errorf("hookScript(%v) does not contain InvocationLine(%v)\nscript=%q\nline=%q", strict, strict, script, il)
		}
	}
}
