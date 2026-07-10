package config

import (
	"fmt"
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
	err := parseInt("repo", "stagecoach.timeout", "abc", new(int))
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
	setGitConfig(t, repo, "stagecoach.provider", "pi")
	setGitConfig(t, repo, "stagecoach.model", "glm-5.2")
	setGitConfig(t, repo, "stagecoach.timeout", "90")
	setGitConfig(t, repo, "stagecoach.autoStageAll", "on") // --bool normalizes "on" -> true
	setGitConfig(t, repo, "stagecoach.verbose", "yes")     // "yes" -> true
	setGitConfig(t, repo, "stagecoach.maxDiffBytes", "12345")
	setGitConfig(t, repo, "stagecoach.stripCodeFence", "1") // "1" -> true
	setGitConfig(t, repo, "stagecoach.output", "json")
	// §9.19 FR-F1/FR-F6
	setGitConfig(t, repo, "stagecoach.format", "gitmoji")
	setGitConfig(t, repo, "stagecoach.locale", "de")
	// §9.22 FR-P1
	setGitConfig(t, repo, "stagecoach.push", "true")
	// §9.25 FR-V5 — noVerify via git config (camelCase key: git rejects underscores
	// in the final segment, matching the autoStageAll/maxDiffBytes/stripCodeFence convention).
	setGitConfig(t, repo, "stagecoach.noVerify", "true")
	// §9.27 FR-K6 — noParentWatchdog via git config (camelCase key, same convention as noVerify).
	setGitConfig(t, repo, "stagecoach.noParentWatchdog", "true")

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
	if !cfg.AutoStageAllValue() {
		t.Errorf("AutoStageAll=false want true (--bool 'on')")
	}
	if !cfg.Verbose {
		t.Errorf("Verbose=false want true (--bool 'yes')")
	}
	if cfg.MaxDiffBytes != 12345 {
		t.Errorf("MaxDiffBytes=%d want 12345", cfg.MaxDiffBytes)
	}
	if cfg.StripCodeFence == nil || !*cfg.StripCodeFence {
		t.Errorf("StripCodeFence=%v want true (--bool '1')", cfg.StripCodeFence)
	}
	if cfg.Output == nil || *cfg.Output != "json" {
		t.Errorf("Output=%v want strPtr(json)", cfg.Output)
	}
	// §9.19 FR-F1/FR-F6 — format/locale via git config
	if cfg.Format != "gitmoji" {
		t.Errorf("Format=%q want gitmoji", cfg.Format)
	}
	if cfg.Locale != "de" {
		t.Errorf("Locale=%q want de", cfg.Locale)
	}
	// §9.22 FR-P1 — push via git config
	if !cfg.Push {
		t.Errorf("Push=false want true (stagecoach.push set)")
	}
	// §9.25 FR-V5 — noVerify via git config
	if !cfg.NoVerify {
		t.Errorf("NoVerify=false want true (stagecoach.noVerify set)")
	}
	// §9.27 FR-K6 — noParentWatchdog via git config
	if !cfg.NoParentWatchdog {
		t.Errorf("NoParentWatchdog=false want true (stagecoach.noParentWatchdog set)")
	}
}

// ---------------------------------------------------------------------------
// Test B: MissingKeysIgnored — contract "ignores missing"
// ---------------------------------------------------------------------------

func TestLoadGitConfig_MissingKeysIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo) // no stagecoach.* keys set

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil (missing keys are not errors)", err)
	}
	if cfg == nil {
		t.Fatal("cfg=nil, want non-nil")
	}
	// EVERY field must be its zero value (nothing was set):
	if cfg.Provider != "" || cfg.Model != "" || cfg.Output != nil {
		t.Errorf("string field non-zero: %+v", cfg)
	}
	if cfg.Timeout != 0 || cfg.MaxDiffBytes != 0 || cfg.MaxMdLines != 0 ||
		cfg.MaxDuplicateRetries != 0 || cfg.SubjectTargetChars != 0 {
		t.Errorf("numeric field non-zero: %+v", cfg)
	}
	if cfg.AutoStageAll != nil || cfg.Verbose || (cfg.StripCodeFence != nil && *cfg.StripCodeFence) {
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
	setGitConfig(t, repo, "stagecoach.autoStageAll", "off")
	setGitConfig(t, repo, "stagecoach.stripCodeFence", "no")

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.AutoStageAllValue() {
		t.Errorf("AutoStageAll=true want false (--bool 'off')")
	}
	if cfg.StripCodeFence != nil && *cfg.StripCodeFence {
		t.Errorf("StripCodeFence=%v want false (--bool 'no')", cfg.StripCodeFence)
	}
	// §9.22 FR-P1 — push=false via git config
	setGitConfig(t, repo, "stagecoach.push", "false")
	cfg, err = loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig push err=%v, want nil", err)
	}
	if cfg.Push {
		t.Errorf("Push=true want false (stagecoach.push=false)")
	}
}

// ---------------------------------------------------------------------------
// Test D: BadTimeout — non-parseable timeout fails at load
// ---------------------------------------------------------------------------

func TestLoadGitConfig_BadTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.timeout", "notanumber")

	_, err := loadGitConfig(repo)
	if err == nil {
		t.Fatal("loadGitConfig err=nil, want non-nil for bad timeout")
	}
	if !strings.Contains(err.Error(), "stagecoach.timeout") {
		t.Errorf("err=%v, want it to contain 'stagecoach.timeout'", err)
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
	setGitConfig(t, repo, "stagecoach.timeout", "90")
	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout=%v want 90s (integer form)", cfg.Timeout)
	}

	// Duration form
	setGitConfig(t, repo, "stagecoach.timeout", "2m30s")
	cfg, err = loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if cfg.Timeout != 150*time.Second {
		t.Errorf("Timeout=%v want 2m30s (duration form)", cfg.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Test K: PerRoleTimeout — §9.15 FR-R7 per-role generation timeout via git config
// (layer 4). Proves the per-role loop is general (≥2 roles) and uses parseTimeout
// (the bare-int "300" form would fail under time.ParseDuration).
// ---------------------------------------------------------------------------

func TestLoadGitConfig_PerRoleTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global git config (FINDING E)
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s") // duration form
	setGitConfig(t, repo, "stagecoach.role.stager.timeout", "300")   // bare-int form (proves parseTimeout)

	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second {
		t.Errorf("Roles[planner].Timeout=%v want 600s", rc.Timeout)
	}
	if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
		t.Errorf("Roles[stager].Timeout=%v want 300s (bare int via parseTimeout)", rc.Timeout)
	}
	// Unset role: message has no key → absent or Timeout==0 (loop does not touch it).
	if rc, ok := cfg.Roles["message"]; ok && rc.Timeout != 0 {
		t.Errorf("Roles[message].Timeout=%v want 0 (unset role untouched)", rc.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Test L: PerRoleTimeout_BadValue — a malformed per-role git value is a HARD ERROR
// (loadGitConfig has an error return — the OPPOSITE of S2's loadFlags silent-ignore).
// The error names the per-role key and the parse failure.
// ---------------------------------------------------------------------------

func TestLoadGitConfig_PerRoleTimeout_BadValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "notanumber")

	_, err := loadGitConfig(repo)
	if err == nil {
		t.Fatal("loadGitConfig err=nil, want non-nil for bad per-role timeout")
	}
	if !strings.Contains(err.Error(), "stagecoach.role.planner.timeout") {
		t.Errorf("err=%v, want it to contain 'stagecoach.role.planner.timeout'", err)
	}
	if !strings.Contains(err.Error(), "invalid timeout") {
		t.Errorf("err=%v, want it to contain 'invalid timeout'", err)
	}
}

// ---------------------------------------------------------------------------
// Test M: PerRoleTimeout_FieldMergeViaOverlay — git sets ONLY Timeout on the role;
// overlay merges it onto a file-layer provider WITHOUT clobbering it (FR-R3). Proves
// overlay needs NO change (the != 0 guard at file.go already handles per-role Timeout).
// ---------------------------------------------------------------------------

func TestLoadGitConfig_PerRoleTimeout_FieldMergeViaOverlay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s")

	gc, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v", err)
	}
	// Git-config per-role entry has ONLY Timeout set (Provider/Model/Reasoning zero) — field hygiene.
	rc := gc.Roles["planner"]
	if rc.Timeout != 600*time.Second {
		t.Errorf("Timeout=%v want 600s", rc.Timeout)
	}
	if rc.Provider != "" || rc.Model != "" || rc.Reasoning != "" {
		t.Errorf("Roles[planner]=%+v want only Timeout set (git layer sets one field)", rc)
	}
	// Simulate a lower (file) layer with a per-role provider, then overlay the git config:
	// BOTH must survive (FR-R3 field-merge via the overlay != 0 guard at file.go).
	base := Defaults()
	base.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}}
	overlay(&base, gc)
	merged := base.Roles["planner"]
	if merged.Provider != "agy" {
		t.Errorf("Provider=%q want agy (file layer preserved — git did not clobber)", merged.Provider)
	}
	if merged.Timeout != 600*time.Second {
		t.Errorf("Timeout=%v want 600s (git layer merged in)", merged.Timeout)
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
	setGitConfig(t, repo, "stagecoach.provider", "pi")

	// Found
	v, found, err := gitConfigGet(repo, "stagecoach.provider")
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
	v, found, err = gitConfigGet(repo, "stagecoach.does.not.exist")
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
	setGitConfig(t, repo, "stagecoach.autoStageAll", "true")
	cfg, err := loadGitConfig(repo)
	if err != nil {
		t.Fatalf("loadGitConfig err=%v, want nil", err)
	}
	if !cfg.AutoStageAllValue() {
		t.Error("AutoStageAll=false want true (camelCase key works)")
	}

	// underscore key is rejected by git on WRITE (invalid key)
	cmd := exec.Command("git", "-C", repo, "config", "stagecoach.max_diff_bytes", "9")
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
	setGitConfig(t, repo, "stagecoach.provider", "pi")
	setGitConfig(t, repo, "stagecoach.timeout", "45")
	setGitConfig(t, repo, "stagecoach.maxMdLines", "7")

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
	if !cfg.AutoStageAllValue() {
		t.Errorf("AutoStageAll=false want true (default preserved)")
	}
	if cfg.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes=%d want 300000 (default preserved)", cfg.MaxDiffBytes)
	}
	if cfg.Output != nil {
		t.Errorf("Output=%v want nil (default preserved)", cfg.Output)
	}
}

// ---------------------------------------------------------------------------
// Test I: TokenLimit & DiffContext — table-driven proof of the two new int keys
// (the explicit-0 row is load-bearing: a `!= 0` value guard would drop it).
// ---------------------------------------------------------------------------

func TestLoadGitConfig_TokenLimit_DiffContext(t *testing.T) {
	// diffContext rows — unset (nil) / 0 (preserved!) / 1 / 3
	type diffRow struct {
		name   string
		setVal string // empty = don't set the key
		want   func(*int) error
	}
	diffRows := []diffRow{
		{
			name:   "unset",
			setVal: "",
			want: func(p *int) error {
				if p != nil {
					return fmt.Errorf("DiffContext=%v want nil (partial config — overlay inherits default)", p)
				}
				return nil
			},
		},
		{
			name:   "zero",
			setVal: "0",
			want: func(p *int) error {
				if p == nil {
					return fmt.Errorf("DiffContext=nil want non-nil (explicit 0 must survive)")
				}
				if *p != 0 {
					return fmt.Errorf("*DiffContext=%d want 0 (explicit 0 = changed-lines-only, FR3f)", *p)
				}
				return nil
			},
		},
		{
			name:   "one",
			setVal: "1",
			want: func(p *int) error {
				if p == nil || *p != 1 {
					return fmt.Errorf("DiffContext=%v want non-nil *1", p)
				}
				return nil
			},
		},
		{
			name:   "three",
			setVal: "3",
			want: func(p *int) error {
				if p == nil || *p != 3 {
					return fmt.Errorf("DiffContext=%v want non-nil *3", p)
				}
				return nil
			},
		},
	}
	for _, r := range diffRows {
		r := r
		t.Run("diffContext/"+r.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			repo := t.TempDir()
			initRepo(t, repo)
			if r.setVal != "" {
				setGitConfig(t, repo, "stagecoach.diffContext", r.setVal)
			}
			cfg, err := loadGitConfig(repo)
			if err != nil {
				t.Fatalf("loadGitConfig err=%v, want nil", err)
			}
			if err := r.want(cfg.DiffContext); err != nil {
				t.Error(err)
			}
		})
	}

	// tokenLimit rows — unset (0) / 120000
	type tlRow struct {
		name   string
		setVal string
		want   int
	}
	tlRows := []tlRow{
		{name: "unset", setVal: "", want: 0},
		{name: "120000", setVal: "120000", want: 120000},
	}
	for _, r := range tlRows {
		r := r
		t.Run("tokenLimit/"+r.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			repo := t.TempDir()
			initRepo(t, repo)
			if r.setVal != "" {
				setGitConfig(t, repo, "stagecoach.tokenLimit", r.setVal)
			}
			cfg, err := loadGitConfig(repo)
			if err != nil {
				t.Fatalf("loadGitConfig err=%v, want nil", err)
			}
			if cfg.TokenLimit != r.want {
				t.Errorf("TokenLimit=%d want %d", cfg.TokenLimit, r.want)
			}
			// DiffContext must remain nil when only tokenLimit is set
			if cfg.DiffContext != nil {
				t.Errorf("DiffContext=%v want nil (only tokenLimit set)", cfg.DiffContext)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test J: BadTokenLimit_DiffContext — non-integer values fail (mirror BadTimeout)
// ---------------------------------------------------------------------------

func TestLoadGitConfig_BadTokenLimit_DiffContext(t *testing.T) {
	for _, c := range []struct {
		key, val string
	}{
		{"stagecoach.diffContext", "abc"},
		{"stagecoach.tokenLimit", "NaN"},
	} {
		c := c
		t.Run(c.key+"="+c.val, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			repo := t.TempDir()
			initRepo(t, repo)
			setGitConfig(t, repo, c.key, c.val)

			_, err := loadGitConfig(repo)
			if err == nil {
				t.Fatalf("loadGitConfig err=nil, want non-nil for bad %s=%q", c.key, c.val)
			}
			if !strings.Contains(err.Error(), c.key) {
				t.Errorf("err=%v, want it to contain %q", err, c.key)
			}
			if !strings.Contains(err.Error(), "invalid integer") {
				t.Errorf("err=%v, want it to contain 'invalid integer'", err)
			}
		})
	}
}
