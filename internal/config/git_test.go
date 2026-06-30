package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers (replicated from internal/git/git_test.go — _test.go helpers
// are not importable across packages).
// ---------------------------------------------------------------------------

// initRepo creates a minimal git repo in dir for testing.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Set repo-local user identity so every subsequent git operation in this repo
	// works even without a global ~/.gitconfig.
	cfgCmd := exec.Command("git", "-C", dir, "config", "user.name", "Test")
	cfgCmd.Env = os.Environ()
	if out, err := cfgCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v\n%s", err, out)
	}
	emailCmd := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com")
	emailCmd.Env = os.Environ()
	if out, err := emailCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v\n%s", err, out)
	}
}

// setGitConfig writes a git config key=value in the repo at dir (repo-local).
func setGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "config", key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config %s=%s failed: %v\n%s", key, value, err, out)
	}
}

// ---------------------------------------------------------------------------
// Test parseInt error path
// ---------------------------------------------------------------------------

func TestParseInt_Error(t *testing.T) {
	err := parseInt("repo", "stagehand.timeout", "abc", new(int))
	if err == nil {
		t.Fatal("parseInt err=nil, want error for non-integer")
	}
	if !strings.Contains(err.Error(), "invalid integer") {
		t.Errorf("err=%v, want 'invalid integer'", err)
	}
}

// ---------------------------------------------------------------------------
// Test A: ReadsValues — contract main case
// ---------------------------------------------------------------------------

func TestLoadGitConfig_ReadsValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global git config (FINDING E)
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagehand.provider", "pi")
	setGitConfig(t, repo, "stagehand.model", "glm-5.2")
	setGitConfig(t, repo, "stagehand.timeout", "90")
	setGitConfig(t, repo, "stagehand.autoStageAll", "on") // --bool normalizes "on" -> true
	setGitConfig(t, repo, "stagehand.verbose", "yes")     // "yes" -> true
	setGitConfig(t, repo, "stagehand.maxDiffBytes", "12345")
	setGitConfig(t, repo, "stagehand.stripCodeFence", "1") // "1" -> true
	setGitConfig(t, repo, "stagehand.output", "json")

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("cfg=nil, want non-nil")
	}
	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi", cfg.Provider)
	}
	if cfg.Model != "glm-5.2" {
		t.Errorf("Model=%q want glm-5.2", cfg.Model)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout=%v want 90s", cfg.Timeout)
	}
	if !cfg.AutoStageAll {
		t.Errorf("AutoStageAll=false want true (--bool 'on')")
	}
	if !cfg.Verbose {
		t.Errorf("Verbose=false want true (--bool 'yes')")
	}
	if cfg.MaxDiffBytes != 12345 {
		t.Errorf("MaxDiffBytes=%d want 12345", cfg.MaxDiffBytes)
	}
	if !cfg.StripCodeFence {
		t.Errorf("StripCodeFence=false want true (--bool '1')")
	}
	if cfg.Output != "json" {
		t.Errorf("Output=%q want json", cfg.Output)
	}
}

// ---------------------------------------------------------------------------
// Test B: MissingKeysIgnored — contract "ignores missing"
// ---------------------------------------------------------------------------

func TestLoadGitConfig_MissingKeysIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo) // no stagehand.* keys set

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil (missing keys are not errors)", err)
	}
	if cfg == nil {
		t.Fatal("cfg=nil, want non-nil")
	}
	// EVERY field must be its zero value (nothing was set):
	if cfg.Provider != "" || cfg.Model != "" || cfg.Output != "" {
		t.Errorf("string field non-zero: %+v", cfg)
	}
	if cfg.Timeout != 0 || cfg.MaxDiffBytes != 0 || cfg.MaxMdLines != 0 ||
		cfg.MaxDuplicateRetries != 0 || cfg.SubjectTargetChars != 0 {
		t.Errorf("numeric field non-zero: %+v", cfg)
	}
	if cfg.AutoStageAll || cfg.Verbose || cfg.StripCodeFence {
		t.Errorf("bool field non-zero: %+v", cfg)
	}
}

// ---------------------------------------------------------------------------
// Test C: BoolNormalization — --bool parses falsy spellings
// ---------------------------------------------------------------------------

func TestLoadGitConfig_BoolNormalization(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagehand.autoStageAll", "off")
	setGitConfig(t, repo, "stagehand.stripCodeFence", "no")

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.AutoStageAll {
		t.Errorf("AutoStageAll=true want false (--bool 'off')")
	}
	if cfg.StripCodeFence {
		t.Errorf("StripCodeFence=true want false (--bool 'no')")
	}
}

// ---------------------------------------------------------------------------
// Test D: BadTimeout — non-parseable timeout fails at load
// ---------------------------------------------------------------------------

func TestLoadGitConfig_BadTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagehand.timeout", "notanumber")

	_, err := loadGitConfig(repo)
	if err == nil {
		t.Fatal("loadGitConfig err=nil, want non-nil for bad timeout")
	}
	if !strings.Contains(err.Error(), "stagehand.timeout") {
		t.Errorf("err=%v, want it to contain 'stagehand.timeout'", err)
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("err=%v, want it to contain 'invalid timeout'", err)
	}
}

// Test D2: Timeout accepts both "90" and "90s" forms from git config
func TestLoadGitConfig_TimeoutDurationForm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)

	// Integer form
	setGitConfig(t, repo, "stagehand.timeout", "90")
	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout=%v want 90s (integer form)", cfg.Timeout)
	}

	// Duration form
	setGitConfig(t, repo, "stagehand.timeout", "2m30s")
	cfg, err = loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.Timeout != 150*time.Second {
		t.Errorf("Timeout=%v want 2m30s (duration form)", cfg.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Test E: GitBinaryMissing — LookPath miss path
// ---------------------------------------------------------------------------

func TestLoadGitConfig_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes LookPath("git") fail for this test only
	_, err := loadGitConfig(t.TempDir())
	if err == nil {
		t.Fatal("loadGitConfig err=nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Errorf("err=%v, want it to contain 'git binary not found'", err)
	}
}

// ---------------------------------------------------------------------------
// Test F: GitConfigGet_FoundMissing — unit-tests the helper directly
// ---------------------------------------------------------------------------

func TestGitConfigGet_FoundMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagehand.provider", "pi")

	// Found
	v, found, err := gitConfigGet(repo, "stagehand.provider")
	if err != nil {
		t.Fatalf("gitConfigGet err=%v, want nil", err)
	}
	if !found {
		t.Fatal("found=false, want true")
	}
	if v != "pi" {
		t.Errorf("value=%q want pi", v)
	}

	// Missing
	v, found, err = gitConfigGet(repo, "stagehand.does.not.exist")
	if err != nil {
		t.Fatalf("gitConfigGet missing err=%v, want nil", err)
	}
	if found {
		t.Fatal("found=true, want false for missing key")
	}
	if v != "" {
		t.Errorf("value=%q want empty", v)
	}
}

// ---------------------------------------------------------------------------
// Test G: CamelCaseKeysOnly — locks FINDING A regression test
// ---------------------------------------------------------------------------

func TestLoadGitConfig_CamelCaseKeysOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)

	// camelCase works
	setGitConfig(t, repo, "stagehand.autoStageAll", "true")
	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if !cfg.AutoStageAll {
		t.Error("AutoStageAll=false want true (camelCase key works)")
	}

	// underscore key is rejected by git on WRITE (invalid key)
	cmd := exec.Command("git", "-C", repo, "config", "stagehand.max_diff_bytes", "9")
	if out, err := cmd.CombinedOutput(); err == nil {
		// If git somehow accepted it (future git version?), that's unexpected but not a test failure
		// — the PRP documents this. Log but don't fail.
		t.Logf("WARNING: git accepted underscore key (unexpected): %s", out)
	} else {
		// Expect "invalid key" in stderr
		if !strings.Contains(string(out), "invalid key") {
			t.Errorf("expected 'invalid key' in output, got: %s", out)
		}
	}

	// The underscore key is unreadable — loadGitConfig should still work and
	// MaxDiffBytes should be 0 (the camelCase key was never set).
	cfg, err = loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig after underscore attempt err=%v, want nil", err)
	}
	if cfg.MaxDiffBytes != 0 {
		t.Errorf("MaxDiffBytes=%d want 0 (underscore key is unreadable)", cfg.MaxDiffBytes)
	}
}

// ---------------------------------------------------------------------------
// Test H: OverlaysWithDefaults — S3→S2-overlay composition contract
// ---------------------------------------------------------------------------

func TestLoadGitConfig_OverlaysWithDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagehand.provider", "pi")
	setGitConfig(t, repo, "stagehand.timeout", "45")
	setGitConfig(t, repo, "stagehand.maxMdLines", "7")

	cfg := Defaults()
	gc, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v", err)
	}
	overlay(&cfg, gc) // S2's overlay (exists when S3 ships)

	if cfg.Provider != "pi" {
		t.Errorf("Provider=%q want pi (git overrode default)", cfg.Provider)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout=%v want 45s", cfg.Timeout)
	}
	if cfg.MaxMdLines != 7 {
		t.Errorf("MaxMdLines=%d want 7", cfg.MaxMdLines)
	}
	// Unset git fields MUST keep Defaults() (proves partial overlay, not wholesale replace):
	if !cfg.AutoStageAll {
		t.Errorf("AutoStageAll=false want true (default preserved)")
	}
	if cfg.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes=%d want 300000 (default preserved)", cfg.MaxDiffBytes)
	}
	if cfg.Output != "raw" {
		t.Errorf("Output=%q want raw (default preserved)", cfg.Output)
	}
}
