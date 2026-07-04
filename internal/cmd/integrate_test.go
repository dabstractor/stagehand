package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/integrate"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resetIntegrateFlags resets the integrate-local flags that are not covered by
// restoreRootState. restoreRootState resets rootCmd's persistent flags, but
// --yes is a PERSISTENT flag on integrateCmd and may survive between tests.
func resetIntegrateFlags(t *testing.T) {
	t.Helper()
	flagIntegrateYes = false
	if f := integrateCmd.PersistentFlags().Lookup("yes"); f != nil && f.Changed {
		f.Changed = false
	}
	// T2.S1: reset --alias-name flag on install and remove
	flagAliasName = ""
	for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
		if f := c.Flags().Lookup("alias-name"); f != nil && f.Changed {
			f.Changed = false
		}
	}
	// T2.S2: reset --key flag on install and remove
	flagLazygitKey = ""
	for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
		if f := c.Flags().Lookup("key"); f != nil && f.Changed {
			f.Changed = false
		}
	}
}

// fakeEntry is a test Entry implementation with configurable behavior and a call log.
type fakeEntry struct {
	name          string
	detectErr     error
	status        integrate.Status
	statusErr     error
	configPath    string
	configErr     error
	installRes    integrate.InstallResult
	installErr    error
	removeRes     integrate.RemoveResult
	removeErr     error
	installCalled bool
	installOpts   integrate.InstallOptions
	removeCalled  bool
	removeOpts    integrate.RemoveOptions
}

func (f *fakeEntry) Name() string                                       { return f.name }
func (f *fakeEntry) Detect(_ context.Context) error                     { return f.detectErr }
func (f *fakeEntry) ConfigPath(_ context.Context) (string, error)       { return f.configPath, f.configErr }
func (f *fakeEntry) Status(_ context.Context) (integrate.Status, error) { return f.status, f.statusErr }
func (f *fakeEntry) Install(_ context.Context, o integrate.InstallOptions) (integrate.InstallResult, error) {
	f.installCalled = true
	f.installOpts = o
	return f.installRes, f.installErr
}
func (f *fakeEntry) Remove(_ context.Context, o integrate.RemoveOptions) (integrate.RemoveResult, error) {
	f.removeCalled = true
	f.removeOpts = o
	return f.removeRes, f.removeErr
}

// ---------------------------------------------------------------------------
// Registry unit tests (internal/integrate/registry.go)
// ---------------------------------------------------------------------------

func TestRegistry_GetList(t *testing.T) {
	a := &fakeEntry{name: "alpha"}
	b := &fakeEntry{name: "beta"}
	c := &fakeEntry{name: "charlie"}

	reg := integrate.NewRegistry([]integrate.Entry{c, a, b, nil}) // nil skipped; order mixed

	// Get — hit
	if e, ok := reg.Get("beta"); !ok || e.Name() != "beta" {
		t.Errorf("Get(beta) = %v, %v; want beta, true", e, ok)
	}

	// Get — miss
	if _, ok := reg.Get("ghost"); ok {
		t.Error("Get(ghost) = true, want false")
	}

	// List — sorted ascending by Name
	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	want := []string{"alpha", "beta", "charlie"}
	for i, w := range want {
		if list[i].Name() != w {
			t.Errorf("List()[%d].Name() = %q, want %q", i, list[i].Name(), w)
		}
	}

	// Nil entries → empty registry
	emptyReg := integrate.NewRegistry(nil)
	if _, ok := emptyReg.Get("anything"); ok {
		t.Error("NewRegistry(nil) should have no entries but Get returned true")
	}
	if list := emptyReg.List(); len(list) != 0 {
		t.Errorf("NewRegistry(nil).List() len = %d, want 0", len(list))
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		s    integrate.Status
		want string
	}{
		{integrate.StatusNotInstalled, "not installed"},
		{integrate.StatusInstalled, "installed"},
		{integrate.StatusForeign, "foreign"},
		{integrate.Status(99), "not installed"}, // unknown → default
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// printIntegrateList tests
// ---------------------------------------------------------------------------

func TestPrintIntegrateList_Table(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:       "charlie",
			detectErr:  nil, // detected
			status:     integrate.StatusInstalled,
			configPath: "/home/user/.config/charlie.conf",
		},
		&fakeEntry{
			name:      "alpha",
			detectErr: errors.New("not found"), // not detected
			status:    integrate.StatusNotInstalled,
		},
		&fakeEntry{
			name:       "bravo",
			detectErr:  nil,
			status:     integrate.StatusForeign,
			configPath: "/home/user/.config/bravo.yml",
		},
	})

	var stdout, stderr bytes.Buffer
	printIntegrateList(ctx, &stdout, &stderr, reg)

	got := stdout.String()

	// Header
	if !strings.Contains(got, "TARGET") || !strings.Contains(got, "DETECTED") || !strings.Contains(got, "STATUS") || !strings.Contains(got, "CONFIG") {
		t.Error("missing header row")
	}

	// 3 data rows
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 4 { // header + 3 rows
		t.Fatalf("expected 4 lines (header + 3 rows), got %d: %q", len(lines), lines)
	}

	// Sorted order: alpha, bravo, charlie
	idxAlpha := strings.Index(got, "alpha")
	idxBravo := strings.Index(got, "bravo")
	idxCharlie := strings.Index(got, "charlie")
	if idxAlpha >= idxBravo || idxBravo >= idxCharlie {
		t.Errorf("rows not sorted ascending: alpha@%d bravo@%d charlie@%d", idxAlpha, idxBravo, idxCharlie)
	}

	// Detection glyphs (tabwriter replaces tabs with spaces)
	// Check that each target has the right detection glyph on its line
	for _, line := range lines[1:] {
		switch {
		case strings.Contains(line, "alpha"):
			if !strings.Contains(line, "✗") {
				t.Error(`alpha should show ✗ (not detected)`)
			}
		case strings.Contains(line, "bravo"):
			if !strings.Contains(line, "✓") {
				t.Error(`bravo should show ✓ (detected)`)
			}
		case strings.Contains(line, "charlie"):
			if !strings.Contains(line, "✓") {
				t.Error(`charlie should show ✓ (detected)`)
			}
		}
	}

	// Status tokens
	if !strings.Contains(got, "not installed") {
		t.Error(`missing "not installed" status`)
	}
	if !strings.Contains(got, "installed") {
		t.Error(`missing "installed" status`)
	}
	if !strings.Contains(got, "foreign") {
		t.Error(`missing "foreign" status`)
	}

	// Config paths
	if !strings.Contains(got, "/home/user/.config/charlie.conf") {
		t.Error(`missing charlie config path`)
	}
	if !strings.Contains(got, "/home/user/.config/bravo.yml") {
		t.Error(`missing bravo config path`)
	}

	// Empty config path → "—"
	// alpha has no configPath, should show "—"
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "alpha") && !strings.HasSuffix(strings.TrimSpace(line), "—") {
			t.Errorf(`alpha row should end with "—" for empty config, got: %q`, line)
		}
	}
}

func TestPrintIntegrateList_Empty(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry(nil)

	var stdout bytes.Buffer
	printIntegrateList(ctx, &stdout, io.Discard, reg)

	got := strings.TrimSpace(stdout.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Errorf("empty registry should have header-only; got %d lines: %q", len(lines), got)
	}
	if !strings.Contains(got, "TARGET") || !strings.Contains(got, "DETECTED") || !strings.Contains(got, "STATUS") || !strings.Contains(got, "CONFIG") {
		t.Error("missing header in empty list")
	}
}

// ---------------------------------------------------------------------------
// dispatchInstall tests
// ---------------------------------------------------------------------------

func TestDispatchInstall_AllInstalled(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:       "alpha",
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeCreated, Target: "alpha", Path: "/a"},
		},
		&fakeEntry{
			name:       "beta",
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeUpdated, Target: "beta", Path: "/b", Backup: "/b.bak"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"alpha", "beta"}, opts, &stdout, &stderr)

	if err != nil {
		t.Errorf("dispatchInstall err=%v, want nil (all installed)", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Installed alpha integration") {
		t.Error(`missing "Installed alpha" output`)
	}
	if !strings.Contains(got, "Updated beta integration") {
		t.Error(`missing "Updated beta" output`)
	}
	if !strings.Contains(got, "Backup: /b.bak") {
		t.Error(`missing backup info for updated beta`)
	}
	if stderr.Len() > 0 {
		t.Errorf("stderr should be empty, got %q", stderr.String())
	}
}

func TestDispatchInstall_DetectionGateExit1(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:       "alpha",
			detectErr:  errors.New("lazygit not found"),
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeCreated, Target: "alpha", Path: "/a"},
		},
		&fakeEntry{
			name:       "beta",
			detectErr:  nil, // detected OK
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeCreated, Target: "beta", Path: "/b"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"alpha", "beta"}, opts, &stdout, &stderr)

	if err == nil {
		t.Fatal("dispatchInstall err=nil, want error (detection gate)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("expected exitcode.Error, got %v", err)
	}

	// alpha was detection-gated (note on stderr)
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "alpha") || !strings.Contains(stderrStr, "lazygit not found") {
		t.Errorf(`stderr should mention alpha + detect error, got %q`, stderrStr)
	}
	// beta was still installed (batch continue)
	if !strings.Contains(stdout.String(), "Installed beta integration") {
		t.Error(`beta should still be installed (batch continue)`)
	}
}

func TestDispatchInstall_UnknownTarget(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{name: "alpha"},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"ghost"}, opts, &stdout, &stderr)

	if err == nil {
		t.Fatal("dispatchInstall err=nil, want error (unknown target)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("expected exitcode.Error, got %v", err)
	}
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "unknown target") || !strings.Contains(stderrStr, "ghost") {
		t.Errorf(`stderr should mention "unknown target" + "ghost", got %q`, stderrStr)
	}
	if !strings.Contains(stderrStr, "see `stagehand integrate list`") {
		t.Error(`stderr should mention "see integrate list"`)
	}
}

func TestDispatchInstall_DeclineAndNoChangeNotErrors(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:       "alpha",
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeDeclined, Target: "alpha"},
		},
		&fakeEntry{
			name:       "beta",
			installRes: integrate.InstallResult{Outcome: integrate.OutcomeNoChange, Target: "beta"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: false, Out: io.Discard, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"alpha", "beta"}, opts, &stdout, &stderr)

	if err != nil {
		t.Errorf("dispatchInstall err=%v, want nil (decline/nochange not errors)", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Declined alpha") {
		t.Error(`missing "Declined alpha" output`)
	}
	if !strings.Contains(got, "No changes for beta") {
		t.Error(`missing "No changes for beta" output`)
	}
}

func TestDispatchInstall_InstallError(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:       "alpha",
			installErr: errors.New("disk full"),
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.InstallOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchInstall(ctx, reg, []string{"alpha"}, opts, &stdout, &stderr)

	if err == nil {
		t.Fatal("dispatchInstall err=nil, want error (install failure)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("expected exitcode.Error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "install alpha failed") {
		t.Error(`stderr should mention "install alpha failed"`)
	}
	if !strings.Contains(stderr.String(), "disk full") {
		t.Error(`stderr should mention the install error`)
	}
}

// ---------------------------------------------------------------------------
// dispatchRemove tests
// ---------------------------------------------------------------------------

func TestDispatchRemove_AllRemoved(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:      "alpha",
			removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeRemoved, Target: "alpha", Path: "/a", Backup: "/a.bak"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.RemoveOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchRemove(ctx, reg, []string{"alpha"}, opts, &stdout, &stderr)

	if err != nil {
		t.Errorf("dispatchRemove err=%v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "Removed alpha integration") {
		t.Error(`missing "Removed alpha" output`)
	}
	if !strings.Contains(stdout.String(), "Backup: /a.bak") {
		t.Error(`missing backup info`)
	}
}

func TestDispatchRemove_DetectionGate(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:      "alpha",
			detectErr: errors.New("tool missing"),
			removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeRemoved, Target: "alpha", Path: "/a"},
		},
		&fakeEntry{
			name:      "beta",
			detectErr: nil,
			removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeRemoved, Target: "beta", Path: "/b"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.RemoveOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchRemove(ctx, reg, []string{"alpha", "beta"}, opts, &stdout, &stderr)

	if err == nil {
		t.Fatal("dispatchRemove err=nil, want error (detection gate)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("expected exitcode.Error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "alpha") || !strings.Contains(stderr.String(), "tool missing") {
		t.Errorf(`stderr should mention alpha + detect error, got %q`, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Removed beta integration") {
		t.Error(`beta should still be removed (batch continue)`)
	}
}

func TestDispatchRemove_UnknownTarget(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry(nil)

	var stdout, stderr bytes.Buffer
	opts := integrate.RemoveOptions{Yes: true, Out: io.Discard, Confirm: nil}
	err := dispatchRemove(ctx, reg, []string{"ghost"}, opts, &stdout, &stderr)

	if err == nil {
		t.Fatal("dispatchRemove err=nil, want error (unknown target)")
	}
	if !strings.Contains(stderr.String(), "unknown target") {
		t.Error(`stderr should mention "unknown target"`)
	}
}

func TestDispatchRemove_DeclineAndNoChangeNotErrors(t *testing.T) {
	ctx := context.Background()
	reg := integrate.NewRegistry([]integrate.Entry{
		&fakeEntry{
			name:      "alpha",
			removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeDeclined, Target: "alpha"},
		},
		&fakeEntry{
			name:      "beta",
			removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeNoChange, Target: "beta"},
		},
	})

	var stdout, stderr bytes.Buffer
	opts := integrate.RemoveOptions{Yes: false, Out: io.Discard, Confirm: nil}
	err := dispatchRemove(ctx, reg, []string{"alpha", "beta"}, opts, &stdout, &stderr)

	if err != nil {
		t.Errorf("dispatchRemove err=%v, want nil (decline/nochange not errors)", err)
	}
	if !strings.Contains(stdout.String(), "Declined alpha remove") {
		t.Error(`missing "Declined alpha remove" output`)
	}
	if !strings.Contains(stdout.String(), "No changes for beta") {
		t.Error(`missing "No changes for beta" output`)
	}
}

// ---------------------------------------------------------------------------
// Execute-level wiring tests (cobra integration)
// ---------------------------------------------------------------------------

func TestIntegrateList_EmptyWiring(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	// Swap defaultEntries to nil to test empty-registry wiring (git-alias is
	// now a default entry; this test verifies the wiring works with zero entries).
	saved := defaultEntries
	defaultEntries = func() []integrate.Entry { return nil }
	defer func() { defaultEntries = saved }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "TARGET") || !strings.Contains(got, "DETECTED") {
		t.Error(`list output missing header columns`)
	}

	// Only header (no data rows) since we swapped defaultEntries to nil
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Errorf("empty registry list should be header-only; got %d lines: %q", len(lines), got)
	}
}

func TestIntegrateInstall_BogusExit1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	var errBuf bytes.Buffer
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"integrate", "install", "bogus"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown target)")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) || ee.Code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", exitcode.For(err), exitcode.Error)
	}
	if !strings.Contains(errBuf.String(), "unknown target") || !strings.Contains(errBuf.String(), "bogus") {
		t.Errorf(`stderr should mention "unknown target" + "bogus", got %q`, errBuf.String())
	}
}

func TestIntegrateInstall_NoArgsUsage(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate", "install"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (no args)")
	}
	// cobra.MinimumNArgs(1) triggers a usage error
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exit code = %d, want %d (Error)", code, exitcode.Error)
	}
}

// TestIntegrateRemove_UninstallAlias verifies that `integrate uninstall` is accepted as an alias for
// `integrate remove` (report Bug 3). The two integration surfaces used different verbs (`hook uninstall`
// vs `integrate remove`), making `uninstall` a discoverability trap. cobra aliases bridge the gap.
func TestIntegrateRemove_UninstallAlias(t *testing.T) {
	// The alias is registered on integrateRemoveCmd.
	found := false
	for _, a := range integrateRemoveCmd.Aliases {
		if a == "uninstall" {
			found = true
		}
	}
	if !found {
		t.Fatalf("integrateRemoveCmd.Aliases = %v, want 'uninstall' present", integrateRemoveCmd.Aliases)
	}

	// End-to-end: `stagehand integrate uninstall <target>` resolves to runIntegrateRemove and dispatches
	// a remove against the registry (the fake records it).
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	fake := &fakeEntry{
		name:      "test-target",
		removeRes: integrate.RemoveResult{Outcome: integrate.OutcomeRemoved, Target: "test-target", Path: "/t"},
	}
	saved := defaultEntries
	defaultEntries = func() []integrate.Entry { return []integrate.Entry{fake} }
	defer func() { defaultEntries = saved }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate", "uninstall", "test-target"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil (uninstall alias should dispatch remove)", err)
	}
	if !fake.removeCalled {
		t.Errorf("fake.removeCalled = false, want true (uninstall alias must dispatch to Remove)")
	}
	if !strings.Contains(out.String(), "Removed test-target integration") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestIntegrateYesFlag(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	// Swap defaultEntries to inject a fake that records opts.Yes
	fake := &fakeEntry{
		name:       "test-target",
		installRes: integrate.InstallResult{Outcome: integrate.OutcomeCreated, Target: "test-target", Path: "/t"},
	}
	saved := defaultEntries
	defaultEntries = func() []integrate.Entry { return []integrate.Entry{fake} }
	defer func() { defaultEntries = saved }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate", "install", "--yes", "test-target"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	if !fake.installCalled {
		t.Fatal("fake.Install was not called")
	}
	if !fake.installOpts.Yes {
		t.Error("fake.Install saw Yes=false, want true (--yes flag)")
	}
}

func TestIntegrateList_OutsideRepo(t *testing.T) {
	// Verify integrate list works outside a git repo (no-op PersistentPreRunE skips config.Load).
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate", "list"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (works outside repo)", err)
	}

	got := out.String()
	if !strings.Contains(got, "TARGET") {
		t.Error(`list output missing header (should work outside repo)`)
	}
	// Confirm no config was bootstrapped
	if _, statErr := os.Stat(home + "/stagehand/config.toml"); statErr == nil {
		t.Error("integrate list created a config file (bootstrap side effect)")
	}
}

// ---------------------------------------------------------------------------
// Integrate group (no subcommand → help)
// ---------------------------------------------------------------------------

func TestIntegrateGroup_NoSubcommandPrintsHelp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"integrate"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (prints help)", err)
	}
	got := buf.String()
	for _, sub := range []string{"list", "install", "remove"} {
		if !strings.Contains(got, sub) {
			t.Errorf("help output missing %q subcommand", sub)
		}
	}
}
