package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/hook"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resetHookFlags resets the hook-local flags that are not covered by restoreRootState.
// restoreRootState resets rootCmd's persistent flags, but --print and --strict are
// local to hookInstallCmd and survive between tests.
func resetHookFlags(t *testing.T) {
	t.Helper()
	flagHookPrint = false
	flagHookStrict = false
	if f := hookInstallCmd.Flags().Lookup("print"); f != nil && f.Changed {
		f.Changed = false
	}
	if f := hookInstallCmd.Flags().Lookup("strict"); f != nil && f.Changed {
		f.Changed = false
	}
}

// setupHookRepo creates a temp git repo and chdir's into it. Returns the repo dir.
func setupHookRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)
	return repo
}

// ---------------------------------------------------------------------------
// hook install --print (no repo needed)
// ---------------------------------------------------------------------------

func TestHookInstall_Print(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "install", "--print"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	want := hook.Script(false, "")
	if got != want {
		t.Errorf("print output mismatch (got %d bytes, want %d bytes)", len(got), len(want))
	}
}

func TestHookInstall_PrintStrict(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "install", "--print", "--strict"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	want := hook.Script(true, "")
	if got != want {
		t.Errorf("strict print output mismatch")
	}
}

// ---------------------------------------------------------------------------
// hook install / status / uninstall round-trip
// ---------------------------------------------------------------------------

func TestHookInstallStatusUninstall_RoundTrip(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t) // prevent bootstrap side effects

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)

	// status → none
	rootCmd.SetArgs([]string{"hook", "status"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("status err=%v", err)
	}
	if !strings.Contains(out.String(), "none") {
		t.Errorf("initial status = %q, want 'none'", out.String())
	}

	// install
	out.Reset()
	rootCmd.SetArgs([]string{"hook", "install"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("install err=%v", err)
	}
	if !strings.Contains(out.String(), "Installed") {
		t.Errorf("install output = %q, want 'Installed'", out.String())
	}

	// verify file exists and is executable
	hooksDir := repo + "/.git/hooks"
	info, err := os.Stat(hooksDir + "/prepare-commit-msg")
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("hook perm = %o, want 0o755", info.Mode().Perm())
	}

	// status → stagehand (v1)
	out.Reset()
	rootCmd.SetArgs([]string{"hook", "status"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("status err=%v", err)
	}
	if strings.TrimSpace(out.String()) != "stagehand (v1)" {
		t.Errorf("post-install status = %q, want 'stagehand (v1)'", out.String())
	}

	// reinstall (idempotent → "Updated")
	out.Reset()
	rootCmd.SetArgs([]string{"hook", "install"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("reinstall err=%v", err)
	}
	if !strings.Contains(out.String(), "Updated") {
		t.Errorf("reinstall output = %q, want 'Updated'", out.String())
	}

	// uninstall
	out.Reset()
	rootCmd.SetArgs([]string{"hook", "uninstall"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("uninstall err=%v", err)
	}
	if !strings.Contains(out.String(), "Removed") {
		t.Errorf("uninstall output = %q, want 'Removed'", out.String())
	}

	// status → none again
	out.Reset()
	rootCmd.SetArgs([]string{"hook", "status"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("final status err=%v", err)
	}
	if strings.TrimSpace(out.String()) != "none" {
		t.Errorf("final status = %q, want 'none'", out.String())
	}
}

// ---------------------------------------------------------------------------
// hook install — foreign refusal
// ---------------------------------------------------------------------------

func TestHookInstall_ForeignRefused(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	// Create a foreign hook
	hooksDir := repo + "/.git/hooks"
	foreignPath := hooksDir + "/prepare-commit-msg"
	foreignContent := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(foreignPath, []byte(foreignContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"hook", "install"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("install err=nil, want exit 1 (foreign refusal)")
	}
	// Exit code 1
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err is not *ExitError: %v", err)
	}
	if ee.Code != exitcode.Error {
		t.Errorf("exit code = %d, want %d", ee.Code, exitcode.Error)
	}

	// stderr contains the manual invocation line
	stderr := errBuf.String()
	if !strings.Contains(stderr, "exec stagehand hook exec") {
		t.Errorf("stderr = %q, want to contain the invocation line", stderr)
	}
	if !strings.Contains(stderr, "foreign") {
		t.Errorf("stderr = %q, want to contain 'foreign'", stderr)
	}

	// Foreign file UNCHANGED (never-clobber invariant)
	after, _ := os.ReadFile(foreignPath)
	if string(after) != foreignContent {
		t.Error("foreign file was modified (never-clobber violated)")
	}
}

// ---------------------------------------------------------------------------
// hook uninstall — foreign refusal + idempotent none
// ---------------------------------------------------------------------------

func TestHookUninstall_ForeignRefused(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	hooksDir := repo + "/.git/hooks"
	foreignPath := hooksDir + "/prepare-commit-msg"
	foreignContent := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(foreignPath, []byte(foreignContent), 0o644); err != nil {
		t.Fatal(err)
	}

	rootCmd.SetOut(io.Discard)
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"hook", "uninstall"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("uninstall err=nil, want exit 1 (foreign refusal)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("exit code = %v, want exit 1", err)
	}

	after, _ := os.ReadFile(foreignPath)
	if string(after) != foreignContent {
		t.Error("foreign file was modified by uninstall")
	}
}

func TestHookUninstall_NoneIsIdempotent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	setupHookRepo(t)
	isolateHome(t)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "uninstall"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("uninstall none err=%v, want nil (idempotent exit 0)", err)
	}
	if !strings.Contains(out.String(), "No stagehand") {
		t.Errorf("uninstall none output = %q, want informational note", out.String())
	}
}

// ---------------------------------------------------------------------------
// hook status — foreign state
// ---------------------------------------------------------------------------

func TestHookStatus_Foreign(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	hooksDir := repo + "/.git/hooks"
	if err := os.WriteFile(hooksDir+"/prepare-commit-msg", []byte("#!/bin/sh\necho mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "status"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("status err=%v", err)
	}
	if strings.TrimSpace(out.String()) != "foreign" {
		t.Errorf("status = %q, want 'foreign'", out.String())
	}
}

// ---------------------------------------------------------------------------
// hook install --strict bakes --strict into the script
// ---------------------------------------------------------------------------

func TestHookInstall_StrictBaked(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "install", "--strict"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("install --strict err=%v", err)
	}

	data, err := os.ReadFile(repo + "/.git/hooks/prepare-commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "--strict") {
		t.Error("strict hook missing --strict in script body")
	}
}

// ---------------------------------------------------------------------------
// hook install --config <path> bakes STAGEHAND_CONFIG into the script (report Finding 4)
// ---------------------------------------------------------------------------

func TestHookInstall_ConfigBaked(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", "/special/config.toml", "hook", "install"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("install --config err=%v", err)
	}

	data, err := os.ReadFile(repo + "/.git/hooks/prepare-commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	// The explicit --config path is baked in as a STAGEHAND_CONFIG export so `hook exec` at commit time
	// resolves the SAME config the user selected at install time (report Finding 4).
	if !strings.Contains(script, "export STAGEHAND_CONFIG='/special/config.toml'") {
		t.Errorf("config path NOT baked into hook script:\n%s", script)
	}
	// The exec line is still present.
	if !strings.Contains(script, `exec stagehand hook exec "$@"`) {
		t.Errorf("exec line missing after config bake:\n%s", script)
	}
}

// TestHookInstall_NoConfig_NotBaked verifies the default (no --config) install does NOT emit a
// STAGEHAND_CONFIG line — `hook exec` falls back to env/discovery as before (no behavior change for
// the common case).
func TestHookInstall_NoConfig_NotBaked(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := setupHookRepo(t)
	isolateHome(t)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "install"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("install err=%v", err)
	}

	data, err := os.ReadFile(repo + "/.git/hooks/prepare-commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "STAGEHAND_CONFIG") {
		t.Errorf("no --config install must NOT bake STAGEHAND_CONFIG:\n%s", string(data))
	}
}

// ---------------------------------------------------------------------------
// hook group (no subcommand → help)
// ---------------------------------------------------------------------------

func TestHookGroup_NoSubcommandPrintsHelp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (prints help)", err)
	}
	got := buf.String()
	for _, sub := range []string{"install", "uninstall", "status"} {
		if !strings.Contains(got, sub) {
			t.Errorf("help output missing %q subcommand", sub)
		}
	}
}

// ---------------------------------------------------------------------------
// hook status does NOT create a global config (no bootstrap side effect)
// ---------------------------------------------------------------------------

func TestHookStatus_NoConfigLoad(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetHookFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)
	_ = repo // used by chdir

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", "status"})

	err := Execute(context.Background())
	// We don't care if it succeeds or fails — just that no config was created
	_ = err

	configPath := home + "/stagehand/config.toml"
	if _, err := os.Stat(configPath); err == nil {
		t.Error("hook status created a global config file (bootstrap side effect)")
	}
}

// runGitCmd is a local helper for running git in a directory.
func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
