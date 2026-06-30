package stagehand

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/stubtest"
)

// --- Fixture helpers (copied from internal/generate/generate_test.go — package-private, unimportable) ---

// initRepo creates a git repo in dir with repo-local identity config (no env pollution).
func initRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
}

// writeFile creates a file at dir/name with the given body.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", full, err)
	}
}

// stageFile runs git add for name in dir.
func stageFile(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "add", name)
}

// headSHA returns the current HEAD SHA of the repo at dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

// commitRaw creates an empty commit with the given message.
func commitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// gitOut runs a raw git command in dir and returns trimmed stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGit(t, dir, args...)
}

// runGit executes git -C dir args... and returns trimmed stdout.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

var shaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// setupTestRepo initializes a temp git repo with an initial commit, changes CWD into it,
// and registers the stub provider via a repo-local .stagehand.toml.
func setupTestRepo(t *testing.T, stubOpts stubtest.Options) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()

	// Write repo-local .stagehand.toml to register the stub provider.
	// config.Load Layer 3 reads CWD/.stagehand.toml; DecodeUserOverrides decodes [provider.stub].
	var sb strings.Builder
	sb.WriteString("[provider.stub]\n")
	sb.WriteString("command = \"" + bin + "\"\n")
	sb.WriteString("prompt_delivery = \"stdin\"\n")
	sb.WriteString("output = \"raw\"\n")
	sb.WriteString("strip_code_fence = true\n")

	if stubOpts.Out != "" || stubOpts.SleepMS > 0 {
		sb.WriteString("\n[provider.stub.env]\n")
		if stubOpts.Out != "" {
			sb.WriteString("STAGEHAND_STUB_OUT = \"" + stubOpts.Out + "\"\n")
		}
		if stubOpts.SleepMS > 0 {
			sb.WriteString("STAGEHAND_STUB_SLEEP_MS = \"" + strconv.Itoa(stubOpts.SleepMS) + "\"\n")
		}
	}

	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// Chdir into the repo (GenerateCommit uses os.Getwd()).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	return bin
}

// objectCountLine returns the "count:" line of `git count-objects -v` for the repo at dir.
// Used to assert no NEW loose objects (no dangling tree) were written by a failed run.
func objectCountLine(t *testing.T, dir string) string {
	t.Helper()
	for _, line := range strings.Split(gitOut(t, dir, "count-objects", "-v"), "\n") {
		if strings.HasPrefix(line, "count:") {
			return line
		}
	}
	t.Fatalf("git count-objects -v: no 'count:' line in output:\n%s", gitOut(t, dir, "count-objects", "-v"))
	return ""
}

// --- Tests ---

// TestGenerateCommit_Success verifies the happy path: a repo with a staged change,
// stub returns "feat: add x", GenerateCommit creates a commit with the expected result.
func TestGenerateCommit_Success(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: add x"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "hello world")
	stageFile(t, repoDir, "new.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}

	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Subject != "feat: add x" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: add x")
	}
	if res.Message != "feat: add x" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: add x")
	}
	if res.Provider != "stub" {
		t.Errorf("Provider = %q, want %q", res.Provider, "stub")
	}

	// HEAD should match CommitSHA.
	if got := headSHA(t, repoDir); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
}

// TestGenerateCommit_DryRun verifies that DryRun returns a message without creating a commit.
func TestGenerateCommit_DryRun(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: preview"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	beforeSHA := headSHA(t, repoDir)

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v", err)
	}

	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	if res.Message != "feat: preview" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: preview")
	}
	if res.Subject != "feat: preview" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: preview")
	}

	// HEAD should be unchanged.
	afterSHA := headSHA(t, repoDir)
	if afterSHA != beforeSHA {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeSHA, afterSHA)
	}
}

// TestGenerateCommit_NothingStaged verifies that nothing staged returns ErrNothingToCommit.
func TestGenerateCommit_NothingStaged(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: x"})

	ctx := context.Background()
	_, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("errors.Is(err, ErrNothingToCommit) = false, error = %v", err)
	}
}

// TestGenerateCommit_ProviderOverride verifies that opts.Provider selects the stub provider.
func TestGenerateCommit_ProviderOverride(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: override"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "a.txt", "data")
	stageFile(t, repoDir, "a.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.Provider != "stub" {
		t.Errorf("Provider = %q, want %q", res.Provider, "stub")
	}
}

// TestGenerateCommit_Timeout verifies that a stub sleeping longer than opts.Timeout
// returns ErrTimeout (DryRun path) or *RescueError{Kind:ErrTimeout} (commit path).
func TestGenerateCommit_Timeout(t *testing.T) {
	// DryRun path: ErrTimeout (bare sentinel, no TreeSHA).
	t.Run("dryrun", func(t *testing.T) {
		setupTestRepo(t, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
		repoDir, _ := os.Getwd()

		writeFile(t, repoDir, "z.txt", "data")
		stageFile(t, repoDir, "z.txt")

		ctx := context.Background()
		_, err := GenerateCommit(ctx, Options{
			Provider: "stub",
			DryRun:   true,
			Timeout:  150 * time.Millisecond,
		})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("errors.Is(err, ErrTimeout) = false, error = %v", err)
		}
		// DryRun path returns *RescueError{Kind:ErrTimeout} with real TreeSHA (S1 snapshot).
		var re *RescueError
		if !errors.As(err, &re) {
			t.Fatalf("dryrun: error type = %T, want *RescueError", err)
		}
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("dryrun: errors.Is(err, ErrTimeout) = false, error = %v", err)
		}
		if re.TreeSHA == "" {
			t.Error("dryrun: RescueError.TreeSHA empty, want non-empty (snapshot was taken — S1)")
		}
	})

	// Commit path (SystemExtra set): *RescueError{Kind:ErrTimeout} with TreeSHA.
	t.Run("commit_path", func(t *testing.T) {
		setupTestRepo(t, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
		repoDir, _ := os.Getwd()

		writeFile(t, repoDir, "z2.txt", "data")
		stageFile(t, repoDir, "z2.txt")

		ctx := context.Background()
		_, err := GenerateCommit(ctx, Options{
			Provider:    "stub",
			SystemExtra: "extra instructions", // forces runPipeline commit path
			Timeout:     150 * time.Millisecond,
		})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
		var re *RescueError
		if !errors.As(err, &re) {
			t.Fatalf("error type = %T, want *RescueError", err)
		}
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("errors.Is(err, ErrTimeout) = false, got ErrRescue instead?")
		}
		if re.TreeSHA == "" {
			t.Error("RescueError.TreeSHA is empty, want non-empty (snapshot was taken)")
		}
	})
}

// TestGenerateCommit_SystemExtra forces the runPipeline path and commits with extra instructions.
func TestGenerateCommit_SystemExtra(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: with extra"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "s.txt", "data")
	stageFile(t, repoDir, "s.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub", SystemExtra: "refs ticket #42"})
	if err != nil {
		t.Fatalf("GenerateCommit with SystemExtra: %v", err)
	}

	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Message != "feat: with extra" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: with extra")
	}
	// HEAD should have advanced.
	if got := headSHA(t, repoDir); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
}

// TestGenerateCommit_MissingProviderCommand_Issue3 proves PRD Issue 3 is fixed: a provider whose
// command is not on $PATH fails FAST (exit 1) in buildDeps BEFORE the write-tree snapshot — so the
// error is NOT a *RescueError, exitcode.For maps it to 1, the message names the missing command, and
// NO new tree object is written. Before P1.M2.T1.S1 this surfaced as exit 3 (rescue) + a dangling tree.
func TestGenerateCommit_MissingProviderCommand_Issue3(t *testing.T) {
	// Fresh repo with a repo-local .stagehand.toml registering a provider whose command does not exist.
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	const toml = "[provider.missing]\n" +
		"command = \"/nonexistent/path/agent\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = true\n"
	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	// Chdir (GenerateCommit resolves the repo via os.Getwd()).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	// MUST stage a NEW file: if the pre-flight check were removed (regression), WriteTree would write
	// a new tree object and the count-objects guard below would catch it. With nothing staged, the
	// pipeline short-circuits to ErrNothingToCommit before WriteTree, masking the regression.
	writeFile(t, repo, "new.txt", "content")
	stageFile(t, repo, "new.txt")

	beforeCount := objectCountLine(t, repo)

	_, err = GenerateCommit(context.Background(), Options{Provider: "missing"})

	afterCount := objectCountLine(t, repo)

	// Must error.
	if err == nil {
		t.Fatal("GenerateCommit: err = nil, want non-nil (missing provider command)")
	}
	// (a) NOT a *RescueError (the bug returned *RescueError -> exit 3 + rescue block + dangling tree).
	var re *RescueError
	if errors.As(err, &re) {
		t.Errorf("error is *RescueError (%v); want a plain pre-generation error (exit 1)", re)
	}
	// (b) exitcode.For(err) == 1. A plain error falls through to `return Error`.
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error); err=%v", code, exitcode.Error, err)
	}
	// (c) message names the missing command.
	if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") || !strings.Contains(msg, "/nonexistent/path/agent") {
		t.Errorf("err.Error() = %q; want to contain 'not found', 'Is the agent installed?', and '/nonexistent/path/agent'", msg)
	}
	// (d) NO new tree object written (pre-flight ran before WriteTree).
	if beforeCount != afterCount {
		t.Errorf("dangling tree: git count-objects changed\n  before: %s\n  after:  %s\n(pre-flight must run before WriteTree)", beforeCount, afterCount)
	}

	// Optional: dry-run subtest — proves buildDeps protects the runPipeline path too.
	t.Run("dryrun", func(t *testing.T) {
		// Create a fresh repo for the dry-run subtest (can't reuse — CWD already restored).
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
			t.Fatalf("write .stagehand.toml: %v", err)
		}
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd: %v", err)
		}
		if err := os.Chdir(repo); err != nil {
			t.Fatalf("chdir %s: %v", repo, err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
		writeFile(t, repo, "new2.txt", "content")
		stageFile(t, repo, "new2.txt")

		beforeCount := objectCountLine(t, repo)
		_, err = GenerateCommit(context.Background(), Options{Provider: "missing", DryRun: true})
		afterCount := objectCountLine(t, repo)

		if err == nil {
			t.Fatal("GenerateCommit dryrun: err = nil, want non-nil")
		}
		var re *RescueError
		if errors.As(err, &re) {
			t.Errorf("dryrun: error is *RescueError (%v); want plain error", re)
		}
		if code := exitcode.For(err); code != exitcode.Error {
			t.Errorf("dryrun: exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
		}
		if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") {
			t.Errorf("dryrun: err.Error() = %q; want 'not found' + 'Is the agent installed?'", msg)
		}
		if beforeCount != afterCount {
			t.Errorf("dryrun: dangling tree: count changed\n  before: %s\n  after:  %s", beforeCount, afterCount)
		}
	})
}

// TestResolveConfig_InjectedConfig proves that when opts.Config is non-nil, resolveConfig uses
// the injected config directly and does NOT call config.Load. The proof: the injected config
// carries a Providers map entry for a stub provider, and the test runs in a temp dir with NO
// .stagehand.toml and NO STAGEHAND_CONFIG env — if Load ran, it would find no "stub" provider
// (built-ins only) and the Providers map would be empty. The injected provider surviving proves
// Load was skipped.
func TestResolveConfig_InjectedConfig(t *testing.T) {
	bin := stubtest.Build(t)

	// Build a config.Config with the stub provider registered in the Providers map.
	// This is the same shape that config.Load would produce from a .stagehand.toml [provider.stub] table.
	injected := config.Config{
		Provider: "stub",
		Providers: map[string]map[string]any{
			"stub": {
				"command":          bin,
				"prompt_delivery":  "stdin",
				"output":           "raw",
				"strip_code_fence": true,
			},
		},
	}

	// Create a temp git repo with NO .stagehand.toml (so config.Load would find no stub provider).
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// Save and restore CWD (resolveConfig calls os.Getwd).
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	ctx := context.Background()

	t.Run("injected_config_used", func(t *testing.T) {
		cfg, repoDir, err := resolveConfig(ctx, Options{Config: &injected})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Provider != "stub" {
			t.Errorf("cfg.Provider = %q, want %q", cfg.Provider, "stub")
		}
		if cfg.Providers == nil || cfg.Providers["stub"] == nil {
			t.Error("cfg.Providers[\"stub\"] is nil — injected providers map was lost")
		}
		if repoDir != repo {
			t.Errorf("repoDir = %q, want %q", repoDir, repo)
		}
	})

	t.Run("options_overrides_apply_on_injected", func(t *testing.T) {
		// Inject a config with Provider="" and override via Options.Provider.
		emptyProviderCfg := injected
		emptyProviderCfg.Provider = ""
		cfg, _, err := resolveConfig(ctx, Options{Config: &emptyProviderCfg, Provider: "stub"})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Provider != "stub" {
			t.Errorf("cfg.Provider = %q, want %q (Options override should win)", cfg.Provider, "stub")
		}
	})

	t.Run("timeout_override_applies", func(t *testing.T) {
		cfg, _, err := resolveConfig(ctx, Options{Config: &injected, Timeout: 5 * time.Minute})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Timeout != 5*time.Minute {
			t.Errorf("cfg.Timeout = %v, want %v", cfg.Timeout, 5*time.Minute)
		}
	})
}
