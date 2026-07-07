package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/integrate"
)

// newIsolatedGitAliasEntry builds a gitAliasEntry isolated from the real global config.
// It sets GIT_CONFIG_GLOBAL to a temp file so the real ~/.gitconfig is NEVER touched.
// name is the alias name; "" resolves to "stagecoach".
func newIsolatedGitAliasEntry(t *testing.T, name string) *gitAliasEntry {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg) // REPLACES ~/.gitconfig → full isolation
	nm := name
	if nm == "" {
		nm = defaultAliasName
	}
	return &gitAliasEntry{git: git.New(t.TempDir()), aliasName: nm}
}

// ---------------------------------------------------------------------------
// Status tests
// ---------------------------------------------------------------------------

func TestGitAlias_Status_States(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Unset → NotInstalled
	s, err := e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (unset): %v", err)
	}
	if s != integrate.StatusNotInstalled {
		t.Errorf("Status (unset) = %v, want NotInstalled", s)
	}

	// Set ours → Installed
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!stagecoach"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}
	s, err = e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (ours): %v", err)
	}
	if s != integrate.StatusInstalled {
		t.Errorf("Status (ours) = %v, want Installed", s)
	}

	// Set foreign → Foreign
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!other"); err != nil {
		t.Fatalf("ConfigGlobalSet (foreign): %v", err)
	}
	s, err = e.Status(ctx)
	if err != nil {
		t.Fatalf("Status (foreign): %v", err)
	}
	if s != integrate.StatusForeign {
		t.Errorf("Status (foreign) = %v, want Foreign", s)
	}
}

// ---------------------------------------------------------------------------
// Install tests
// ---------------------------------------------------------------------------

func TestGitAlias_Install_Creates(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true, Out: nil, Confirm: nil})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Install Outcome = %v, want Created", res.Outcome)
	}
	if res.Target != "git-alias" {
		t.Errorf("Install Target = %q, want git-alias", res.Target)
	}
	if res.Backup != "" {
		t.Errorf("Install Backup = %q, want empty (git owns the file)", res.Backup)
	}

	// Read back to confirm.
	val, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!stagecoach" {
		t.Errorf("alias = %q found=%v, want !stagecoach found=true", val, found)
	}
}

func TestGitAlias_Install_IdempotentAlreadyOurs(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Pre-set ours.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!stagecoach"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install (already ours): %v", err)
	}
	if res.Outcome != integrate.OutcomeNoChange {
		t.Errorf("Install Outcome = %v, want NoChange", res.Outcome)
	}
}

func TestGitAlias_Install_ForeignOverwritesAfterConfirm(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Pre-set a foreign alias.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!other"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install (foreign): %v", err)
	}
	if res.Outcome != integrate.OutcomeUpdated {
		t.Errorf("Install Outcome = %v, want Updated", res.Outcome)
	}

	// Confirm overwritten.
	val, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!stagecoach" {
		t.Errorf("alias = %q found=%v, want !stagecoach found=true", val, found)
	}
}

func TestGitAlias_Install_DeclineWritesNothing(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Pre-set a foreign alias.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!other"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	res, err := e.Install(ctx, integrate.InstallOptions{
		Yes:     false,
		Confirm: func(_ io.Writer, _ string, _ string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Install (decline): %v", err)
	}
	if res.Outcome != integrate.OutcomeDeclined {
		t.Errorf("Install Outcome = %v, want Declined", res.Outcome)
	}

	// Confirm alias unchanged (still foreign).
	val, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!other" {
		t.Errorf("alias = %q found=%v, want !other found=true (unchanged)", val, found)
	}
}

func TestGitAlias_Install_ConfirmReceivesPreview(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "myalias")
	ctx := context.Background()

	// Fresh install (no pre-set).
	var gotDiff string
	res, err := e.Install(ctx, integrate.InstallOptions{
		Yes: false,
		Confirm: func(_ io.Writer, path string, diff string) bool {
			gotDiff = diff
			return true // accept
		},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome = %v, want Created", res.Outcome)
	}

	// The preview must contain the command and usage.
	if !strings.Contains(gotDiff, "git config --global alias.myalias '!stagecoach'") {
		t.Errorf("preview missing command; got %q", gotDiff)
	}
	if !strings.Contains(gotDiff, "git myalias") {
		t.Errorf("preview missing usage; got %q", gotDiff)
	}
}

func TestGitAlias_Install_ForeignConflictInPreview(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Pre-set a foreign alias.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!foreign-cmd"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	var gotDiff string
	var outBuf bytes.Buffer
	res, err := e.Install(ctx, integrate.InstallOptions{
		Yes: false,
		Out: &outBuf,
		Confirm: func(_ io.Writer, path string, diff string) bool {
			gotDiff = diff
			return true
		},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeUpdated {
		t.Errorf("Outcome = %v, want Updated", res.Outcome)
	}

	// WARNING must be printed to out (fires in both interactive and --yes modes).
	outStr := outBuf.String()
	if !strings.Contains(outStr, "WARNING") {
		t.Errorf("out missing WARNING; got %q", outStr)
	}
	if !strings.Contains(outStr, "!foreign-cmd") {
		t.Errorf("out missing foreign value; got %q", outStr)
	}
	// Preview contains a NOTE (the WARNING is now on out, not inside the preview).
	if !strings.Contains(gotDiff, "NOTE") {
		t.Errorf("preview missing NOTE; got %q", gotDiff)
	}
	if !strings.Contains(gotDiff, "!foreign-cmd") {
		t.Errorf("preview missing foreign value; got %q", gotDiff)
	}
}

// ---------------------------------------------------------------------------
// Remove tests
// ---------------------------------------------------------------------------

func TestGitAlias_Remove_Ours(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Install ours first.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!stagecoach"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	res, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if res.Outcome != integrate.OutcomeRemoved {
		t.Errorf("Remove Outcome = %v, want Removed", res.Outcome)
	}

	// Confirm gone.
	_, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if found {
		t.Error("alias still found after remove")
	}
}

func TestGitAlias_Remove_ForeignRefuses(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Pre-set a foreign alias.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!someone-elses"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	var note bytes.Buffer
	res, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true, Out: &note})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if res.Outcome != integrate.OutcomeNoChange {
		t.Errorf("Remove Outcome = %v, want NoChange", res.Outcome)
	}

	// Confirm alias unchanged.
	val, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!someone-elses" {
		t.Errorf("alias = %q found=%v, want !someone-elses found=true (unchanged)", val, found)
	}

	// Confirm a note was written.
	if !strings.Contains(note.String(), "leaving it unchanged") {
		t.Errorf("note missing 'leaving it unchanged'; got %q", note.String())
	}
}

func TestGitAlias_Remove_UnsetIsNoOp(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	res, err := e.Remove(ctx, integrate.RemoveOptions{Yes: true})
	if err != nil {
		t.Fatalf("Remove (unset): %v", err)
	}
	if res.Outcome != integrate.OutcomeNoChange {
		t.Errorf("Remove Outcome = %v, want NoChange", res.Outcome)
	}
}

func TestGitAlias_Remove_Decline(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Install ours first.
	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), "!stagecoach"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	res, err := e.Remove(ctx, integrate.RemoveOptions{
		Yes:     false,
		Confirm: func(_ io.Writer, _ string, _ string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Remove (decline): %v", err)
	}
	if res.Outcome != integrate.OutcomeDeclined {
		t.Errorf("Remove Outcome = %v, want Declined", res.Outcome)
	}

	// Alias unchanged.
	_, found, _ := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if !found {
		t.Error("alias was removed despite decline")
	}
}

// ---------------------------------------------------------------------------
// Detect / ConfigPath tests
// ---------------------------------------------------------------------------

func TestGitAlias_Detect(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "")
	ctx := context.Background()

	// Git should be on PATH in the test environment.
	if err := e.Detect(ctx); err != nil {
		t.Fatalf("Detect (git present): %v", err)
	}

	// Simulate git missing by clearing PATH.
	e2 := &gitAliasEntry{git: git.New(t.TempDir()), aliasName: "test"}
	t.Setenv("PATH", "")
	if err := e2.Detect(ctx); err == nil {
		t.Fatal("Detect (git missing): err=nil, want non-nil")
	} else if !strings.Contains(err.Error(), "git not found") {
		t.Errorf("Detect err = %v, want 'git not found'", err)
	}
}

func TestGitAlias_ConfigPath(t *testing.T) {
	// GIT_CONFIG_GLOBAL set → returns it absolute.
	e := &gitAliasEntry{git: git.New(t.TempDir()), aliasName: "test"}
	cfgFile := "/tmp/my-custom-gitconfig"
	t.Setenv("GIT_CONFIG_GLOBAL", cfgFile)
	p, err := e.ConfigPath(context.Background())
	if err != nil {
		t.Fatalf("ConfigPath (env): %v", err)
	}
	if p != cfgFile {
		t.Errorf("ConfigPath = %q, want %q", p, cfgFile)
	}

	// HOME set, GIT_CONFIG_GLOBAL unset → $HOME/.gitconfig
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	t.Setenv("HOME", "/home/testuser")
	p, err = e.ConfigPath(context.Background())
	if err != nil {
		t.Fatalf("ConfigPath (home): %v", err)
	}
	if p != "/home/testuser/.gitconfig" {
		t.Errorf("ConfigPath = %q, want /home/testuser/.gitconfig", p)
	}
}

// ---------------------------------------------------------------------------
// Custom alias name tests
// ---------------------------------------------------------------------------

func TestGitAlias_CustomAliasName(t *testing.T) {
	e := newIsolatedGitAliasEntry(t, "ci")
	ctx := context.Background()

	if e.aliasName != "ci" {
		t.Fatalf("aliasName = %q, want ci", e.aliasName)
	}
	if e.aliasKey() != "alias.ci" {
		t.Fatalf("aliasKey = %q, want alias.ci", e.aliasKey())
	}

	// Install with custom name.
	res, err := e.Install(ctx, integrate.InstallOptions{Yes: true})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Outcome != integrate.OutcomeCreated {
		t.Errorf("Outcome = %v, want Created", res.Outcome)
	}

	// Read back — should be alias.ci.
	val, found, err := e.git.ConfigGlobalGet(ctx, "alias.ci")
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!stagecoach" {
		t.Errorf("alias.ci = %q found=%v, want !stagecoach found=true", val, found)
	}

	// alias.stagecoach should NOT exist.
	_, stagecoachFound, _ := e.git.ConfigGlobalGet(ctx, "alias.stagecoach")
	if stagecoachFound {
		t.Error("alias.stagecoach should not exist (only alias.ci was set)")
	}

	// Status should be Installed.
	s, err := e.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s != integrate.StatusInstalled {
		t.Errorf("Status = %v, want Installed", s)
	}
}

// ---------------------------------------------------------------------------
// Execute-level wiring tests (--alias-name flag)
// ---------------------------------------------------------------------------

func TestIntegrateInstall_GitAlias_Execute(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	// Isolate git global config.
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(os.Stderr) // let confirm go to real stderr (but --yes skips it)
	rootCmd.SetArgs([]string{"integrate", "install", "--yes", "git-alias"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "Installed git-alias") {
		t.Errorf("output missing 'Installed git-alias'; got %q", got)
	}

	// Verify the alias was actually set in the isolated config.
	g := git.New(t.TempDir())
	val, found, err := g.ConfigGlobalGet(context.Background(), "alias.stagecoach")
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!stagecoach" {
		t.Errorf("alias.stagecoach = %q found=%v, want !stagecoach found=true", val, found)
	}
}

func TestIntegrateAliasNameFlag(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	// Isolate git global config.
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs([]string{"integrate", "install", "--yes", "git-alias", "--alias-name", "ci"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "Installed git-alias") {
		t.Errorf("output missing 'Installed git-alias'; got %q", got)
	}

	// Verify alias.ci was set (not alias.stagecoach).
	g := git.New(t.TempDir())
	val, found, err := g.ConfigGlobalGet(context.Background(), "alias.ci")
	if err != nil {
		t.Fatalf("ConfigGlobalGet (alias.ci): %v", err)
	}
	if !found || val != "!stagecoach" {
		t.Errorf("alias.ci = %q found=%v, want !stagecoach found=true", val, found)
	}

	_, stagecoachFound, _ := g.ConfigGlobalGet(context.Background(), "alias.stagecoach")
	if stagecoachFound {
		t.Error("alias.stagecoach should not exist (only alias.ci was set)")
	}
}

func TestIntegrateRemove_GitAlias_Execute(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() {
		resetIntegrateFlags(t)
		restoreRootState(t, nil, origOut, origErr, origRunE)
	}()

	// Isolate git global config.
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	// Install first.
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs([]string{"integrate", "install", "--yes", "git-alias"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute install err=%v", err)
	}

	// Now remove.
	var out2 bytes.Buffer
	rootCmd.SetOut(&out2)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs([]string{"integrate", "remove", "--yes", "git-alias"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute remove err=%v, want nil", err)
	}

	got := out2.String()
	if !strings.Contains(got, "Removed git-alias") {
		t.Errorf("output missing 'Removed git-alias'; got %q", got)
	}

	// Verify the alias is gone.
	g := git.New(t.TempDir())
	_, found, _ := g.ConfigGlobalGet(context.Background(), "alias.stagecoach")
	if found {
		t.Error("alias.stagecoach still found after remove")
	}
}

// ---------------------------------------------------------------------------
// isOurs unit test
// ---------------------------------------------------------------------------

func TestIsOurs(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"!stagecoach", true},
		{"stagecoach", true}, // edge: no leading !
		{"!other", false},
		{"!staged-hand", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isOurs(tt.val); got != tt.want {
			t.Errorf("isOurs(%q) = %v, want %v", tt.val, got, tt.want)
		}
	}
}
