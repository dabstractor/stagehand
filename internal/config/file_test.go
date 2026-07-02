// file_test.go is a WHITE-BOX test (package config, matching the house
// convention used by config_test.go and defaults_test.go). It exercises the
// source-loader layer added in P1.M5.T2.S1: GlobalConfigPath's XDG resolution,
// parseFile's golden §16.2 parse + missing/malformed handling, readRepoFile,
// readGlobalFile, and readGitConfig's set/unset/bool/timeout paths. Imports
// are stdlib only (testing, os, os/exec, path/filepath, strings, time) plus
// the internal provider for building the golden ProviderOverrides expectation
// — NO testify.
package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// Pointer helpers keep the golden-overlay assertions compact. Each returns a
// pointer to a fresh local (mirroring how parseFile/applyGitKey produce
// pointers), so a pointer compares non-nil and its deref compares by value.
func sp(s string) *string               { return &s }
func ip(i int) *int                     { return &i }
func bp(b bool) *bool                   { return &b }
func dp(d time.Duration) *time.Duration { return &d }

// wantNil asserts got is a nil pointer.
func wantNil[T any](t *testing.T, got *T, field string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %v, want nil (unset by this source)", field, *got)
	}
}

// golden162 is the PRD §16.2 full config-file example (verbatim). It is the
// GOLDEN parse-test input: parseFile must turn it into the overlay asserted
// in TestParseFile_Golden162.
const golden162 = `# ~/.config/stagehand/config.toml

[defaults]
provider = "pi"            # default agent
model   = ""               # "" → use the manifest's default_model
timeout = "120s"
auto_stage_all = true
verbose = false

[generation]
max_diff_bytes      = 300000
max_md_lines        = 100
max_duplicate_retries = 3
output              = "raw"     # raw | json
strip_code_fence    = true
subject_target_chars = 50

# Override a built-in provider (field-merged with the built-in manifest).
[provider.pi]
default_model = "glm-5.2"
default_provider = "zai"

# Define a brand-new provider (§12.8).
[provider.myagent]
command = "/opt/myagent/bin/agent"
prompt_delivery = "stdin"
print_flag = "--once"
model_flag = "--model"
default_model = "my-model-7b"
system_prompt_flag = "--system"
bare_flags = ["--no-mcp", "--ephemeral"]
output = "raw"
`

// writeFile is a small helper that writes content to path under dir (creating
// parent dirs as needed) and fatals on error.
func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	full := rel
	if dir != "" {
		full = filepath.Join(dir, rel)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

// TestGlobalConfigPath_HonorsXDG asserts XDG_CONFIG_HOME wins and is suffixed
// with stagehand/config.toml.
func TestGlobalConfigPath_HonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	t.Setenv("HOME", "/home/someone")
	got, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath: unexpected error: %v", err)
	}
	if want := filepath.Join("/custom/xdg", "stagehand", "config.toml"); got != want {
		t.Errorf("GlobalConfigPath = %q, want %q", got, want)
	}
}

// TestGlobalConfigPath_HomeFallback asserts the ~/.config fallback when
// XDG_CONFIG_HOME is unset/empty.
func TestGlobalConfigPath_HomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/someone")
	got, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath: unexpected error: %v", err)
	}
	if want := filepath.Join("/home/someone", ".config", "stagehand", "config.toml"); got != want {
		t.Errorf("GlobalConfigPath = %q, want %q", got, want)
	}
}

// TestGlobalConfigPath_BothEmptyErrors asserts a clear error when neither
// XDG_CONFIG_HOME nor HOME is resolvable.
func TestGlobalConfigPath_BothEmptyErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := GlobalConfigPath(); err == nil {
		t.Fatal("GlobalConfigPath: want error when both XDG_CONFIG_HOME and HOME are empty, got nil")
	}
}

// TestParseFile_Golden162 is the contract-required GOLDEN parse test: the §16.2
// example must parse into an overlay whose scalar pointers hold the exact
// example values, with model → non-nil "" and verbose → non-nil false (the
// present-but-zero distinction), and whose ProviderOverrides contains pi
// (DefaultModel glm-5.2, DefaultProvider zai) and myagent (a full manifest incl
// BareFlags [--no-mcp, --ephemeral]).
func TestParseFile_Golden162(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.toml", golden162)
	got, err := parseFile(path)
	if err != nil {
		t.Fatalf("parseFile: unexpected error: %v", err)
	}

	// [defaults] scalars — model is a NON-nil pointer to "", verbose NON-nil
	// to false. no_color is absent → nil.
	if !ptrStrEq(got.Provider, "pi") {
		t.Errorf("Provider = %s, want non-nil \"pi\"", ptrStrDesc(got.Provider))
	}
	if got.Model == nil {
		t.Errorf("Model = nil, want non-nil pointer to \"\" (present-but-zero)")
	} else if *got.Model != "" {
		t.Errorf("Model = %q, want \"\"", *got.Model)
	}
	if !ptrDurEq(got.Timeout, 120*time.Second) {
		t.Errorf("Timeout = %s, want non-nil 120s", ptrDurDesc(got.Timeout))
	}
	if !ptrBoolEq(got.AutoStageAll, true) {
		t.Errorf("AutoStageAll = %s, want non-nil true", ptrBoolDesc(got.AutoStageAll))
	}
	if got.Verbose == nil {
		t.Errorf("Verbose = nil, want non-nil pointer to false (present-but-zero)")
	} else if *got.Verbose != false {
		t.Errorf("Verbose = %v, want false", *got.Verbose)
	}
	wantNil(t, got.NoColor, "NoColor")

	// [generation] scalars.
	if !ptrIntEq(got.MaxDiffBytes, 300000) {
		t.Errorf("MaxDiffBytes = %s, want non-nil 300000", ptrIntDesc(got.MaxDiffBytes))
	}
	if !ptrIntEq(got.MaxMdLines, 100) {
		t.Errorf("MaxMdLines = %s, want non-nil 100", ptrIntDesc(got.MaxMdLines))
	}
	if !ptrIntEq(got.MaxDuplicateRetries, 3) {
		t.Errorf("MaxDuplicateRetries = %s, want non-nil 3", ptrIntDesc(got.MaxDuplicateRetries))
	}
	if !ptrStrEq(got.Output, "raw") {
		t.Errorf("Output = %s, want non-nil \"raw\"", ptrStrDesc(got.Output))
	}
	if !ptrBoolEq(got.StripCodeFence, true) {
		t.Errorf("StripCodeFence = %s, want non-nil true", ptrBoolDesc(got.StripCodeFence))
	}
	if !ptrIntEq(got.SubjectTargetChars, 50) {
		t.Errorf("SubjectTargetChars = %s, want non-nil 50", ptrIntDesc(got.SubjectTargetChars))
	}

	// [provider.*] overrides.
	if got.ProviderOverrides == nil {
		t.Fatal("ProviderOverrides = nil, want map with pi and myagent")
	}
	if len(got.ProviderOverrides) != 2 {
		t.Fatalf("len(ProviderOverrides) = %d, want 2", len(got.ProviderOverrides))
	}
	pi, ok := got.ProviderOverrides["pi"]
	if !ok {
		t.Fatal("ProviderOverrides missing key \"pi\"")
	}
	if pi.DefaultModel != "glm-5.2" {
		t.Errorf("ProviderOverrides[\"pi\"].DefaultModel = %q, want \"glm-5.2\"", pi.DefaultModel)
	}
	if pi.DefaultProvider != "zai" {
		t.Errorf("ProviderOverrides[\"pi\"].DefaultProvider = %q, want \"zai\"", pi.DefaultProvider)
	}
	agent, ok := got.ProviderOverrides["myagent"]
	if !ok {
		t.Fatal("ProviderOverrides missing key \"myagent\"")
	}
	wantAgent := provider.Manifest{
		Name:             "", // §16.2 example does not set name
		Command:          "/opt/myagent/bin/agent",
		PromptDelivery:   "stdin",
		PrintFlag:        "--once",
		ModelFlag:        "--model",
		DefaultModel:     "my-model-7b",
		SystemPromptFlag: "--system",
		BareFlags:        []string{"--no-mcp", "--ephemeral"},
		Output:           "raw",
	}
	if agent.Command != wantAgent.Command {
		t.Errorf("myagent.Command = %q, want %q", agent.Command, wantAgent.Command)
	}
	if agent.PromptDelivery != wantAgent.PromptDelivery {
		t.Errorf("myagent.PromptDelivery = %q, want %q", agent.PromptDelivery, wantAgent.PromptDelivery)
	}
	if agent.PrintFlag != wantAgent.PrintFlag {
		t.Errorf("myagent.PrintFlag = %q, want %q", agent.PrintFlag, wantAgent.PrintFlag)
	}
	if agent.ModelFlag != wantAgent.ModelFlag {
		t.Errorf("myagent.ModelFlag = %q, want %q", agent.ModelFlag, wantAgent.ModelFlag)
	}
	if agent.DefaultModel != wantAgent.DefaultModel {
		t.Errorf("myagent.DefaultModel = %q, want %q", agent.DefaultModel, wantAgent.DefaultModel)
	}
	if agent.SystemPromptFlag != wantAgent.SystemPromptFlag {
		t.Errorf("myagent.SystemPromptFlag = %q, want %q", agent.SystemPromptFlag, wantAgent.SystemPromptFlag)
	}
	if !reflect.DeepEqual(agent.BareFlags, wantAgent.BareFlags) {
		t.Errorf("myagent.BareFlags = %#v, want %#v", agent.BareFlags, wantAgent.BareFlags)
	}
	if agent.Output != wantAgent.Output {
		t.Errorf("myagent.Output = %q, want %q", agent.Output, wantAgent.Output)
	}
}

// TestParseFile_MissingFileIsNotError asserts a non-existent path yields an
// empty overlay (every pointer nil, no provider overrides) with a nil error —
// the FR34 "every layer is optional" contract.
func TestParseFile_MissingFileIsNotError(t *testing.T) {
	got, err := parseFile(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("parseFile(missing): want nil error, got %v", err)
	}
	assertEmptyOverlay(t, got)
}

// TestParseFile_MalformedIsError asserts malformed TOML returns a non-nil
// error wrapping go-toml's diagnostic.
func TestParseFile_MalformedIsError(t *testing.T) {
	path := writeFile(t, t.TempDir(), "bad.toml", "[defaults\n") // unterminated table
	if _, err := parseFile(path); err == nil {
		t.Fatal("parseFile(malformed): want non-nil error, got nil")
	}
}

// TestParseFile_TimeoutBareSeconds asserts the parseDuration fallback: a bare
// integer "90" (the §16.3 git-config shape) parses to 90s, alongside "90s".
func TestParseFile_TimeoutBareSeconds(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"90s", 90 * time.Second},
		{"90", 90 * time.Second},
		{"120s", 120 * time.Second},
		{"2m", 120 * time.Second},
	} {
		got, err := parseDuration(tc.in)
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseDuration("not-a-duration"); err == nil {
		t.Error("parseDuration(\"not-a-duration\"): want error, got nil")
	}
}

// TestReadRepoFile_MissingIsNotError asserts a repo dir with no .stagehand.toml
// yields an empty overlay and nil error.
func TestReadRepoFile_MissingIsNotError(t *testing.T) {
	got, err := readRepoFile(t.TempDir())
	if err != nil {
		t.Fatalf("readRepoFile(empty dir): want nil error, got %v", err)
	}
	assertEmptyOverlay(t, got)
}

// TestReadRepoFile_Loads asserts readRepoFile actually loads the repo file when
// present.
func TestReadRepoFile_Loads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")
	got, err := readRepoFile(dir)
	if err != nil {
		t.Fatalf("readRepoFile: %v", err)
	}
	if !ptrStrEq(got.Provider, "claude") {
		t.Errorf("Provider = %s, want non-nil \"claude\"", ptrStrDesc(got.Provider))
	}
}

// TestReadRepoFile_EmptyDirInheritsCWD asserts the documented repoDir=="" case:
// filepath.Join("", ".stagehand.toml") == ".stagehand.toml" (relative to the
// process cwd), so an empty repoDir does not blow up. (Manual os.Chdir +
// restore because t.Chdir needs go1.24 and this module is go1.22.)
func TestReadRepoFile_EmptyDirInheritsCWD(t *testing.T) {
	// Run in a fresh cwd that has no .stagehand.toml: must be an empty overlay,
	// nil error (not a path error).
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, err := readRepoFile("")
	if err != nil {
		t.Fatalf("readRepoFile(\"\"): want nil error, got %v", err)
	}
	assertEmptyOverlay(t, got)
}

// TestReadGlobalFile_ViaXDG asserts readGlobalFile reads the XDG-derived path
// when XDG_CONFIG_HOME points at a temp dir holding stagehand/config.toml.
func TestReadGlobalFile_ViaXDG(t *testing.T) {
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[defaults]\nprovider = \"gemini\"\nmodel = \"g\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	got, err := readGlobalFile()
	if err != nil {
		t.Fatalf("readGlobalFile: %v", err)
	}
	if !ptrStrEq(got.Provider, "gemini") {
		t.Errorf("Provider = %s, want non-nil \"gemini\"", ptrStrDesc(got.Provider))
	}
	if !ptrStrEq(got.Model, "g") {
		t.Errorf("Model = %s, want non-nil \"g\"", ptrStrDesc(got.Model))
	}
}

// TestReadGlobalFile_MissingIsNotError asserts that with XDG pointing at an
// empty temp dir, readGlobalFile returns an empty overlay with nil error.
func TestReadGlobalFile_MissingIsNotError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := readGlobalFile()
	if err != nil {
		t.Fatalf("readGlobalFile(missing): want nil error, got %v", err)
	}
	assertEmptyOverlay(t, got)
}

// --- git-config tests -------------------------------------------------------

// initGitRepo creates a fresh temp git repo and returns its dir.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	c := exec.Command("git", "init", "-q")
	c.Dir = dir
	if err := c.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return dir
}

// gitSet runs `git config <args...>` in repoDir, fatalling on error.
func gitSet(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = repoDir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), repoDir, err, out)
	}
}

// TestReadGitConfig_EmptyRepoIsNotError asserts a repo with no stagehand.*
// keys yields an empty overlay (all pointers nil) and a nil error — the
// exit-1-per-key path is "unset", not an error.
func TestReadGitConfig_EmptyRepoIsNotError(t *testing.T) {
	repo := initGitRepo(t)
	got, err := readGitConfig(repo)
	if err != nil {
		t.Fatalf("readGitConfig(empty repo): want nil error, got %v", err)
	}
	assertEmptyOverlay(t, got)
}

// TestReadGitConfig_SetsScalars asserts set scalar keys populate the matching
// overlay pointers, an UNSET key (model) leaves its pointer nil, and no error
// is returned.
func TestReadGitConfig_SetsScalars(t *testing.T) {
	repo := initGitRepo(t)
	gitSet(t, repo, "config", "stagehand.provider", "pi")
	gitSet(t, repo, "config", "stagehand.timeout", "90s")
	gitSet(t, repo, "config", "stagehand.maxDiffBytes", "123456")
	gitSet(t, repo, "config", "stagehand.output", "json")
	// model intentionally left UNSET.

	got, err := readGitConfig(repo)
	if err != nil {
		t.Fatalf("readGitConfig: %v", err)
	}
	if !ptrStrEq(got.Provider, "pi") {
		t.Errorf("Provider = %s, want non-nil \"pi\"", ptrStrDesc(got.Provider))
	}
	if !ptrDurEq(got.Timeout, 90*time.Second) {
		t.Errorf("Timeout = %s, want non-nil 90s", ptrDurDesc(got.Timeout))
	}
	if !ptrIntEq(got.MaxDiffBytes, 123456) {
		t.Errorf("MaxDiffBytes = %s, want non-nil 123456", ptrIntDesc(got.MaxDiffBytes))
	}
	if !ptrStrEq(got.Output, "json") {
		t.Errorf("Output = %s, want non-nil \"json\"", ptrStrDesc(got.Output))
	}
	wantNil(t, got.Model, "Model (unset git key)")
	// ProviderOverrides is never populated from git-config.
	if got.ProviderOverrides != nil {
		t.Errorf("ProviderOverrides = %v, want nil (git-config cannot express provider tables)", got.ProviderOverrides)
	}
}

// TestReadGitConfig_BoolTrueAndFalse asserts the --bool keys round-trip both
// true and false as NON-nil pointers (the present-but-zero distinction holds
// for git-config too).
func TestReadGitConfig_BoolTrueAndFalse(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		repo := initGitRepo(t)
		gitSet(t, repo, "config", "--bool", "stagehand.autoStageAll", "true")
		gitSet(t, repo, "config", "--bool", "stagehand.verbose", "true")
		got, err := readGitConfig(repo)
		if err != nil {
			t.Fatalf("readGitConfig: %v", err)
		}
		if !ptrBoolEq(got.AutoStageAll, true) {
			t.Errorf("AutoStageAll = %s, want non-nil true", ptrBoolDesc(got.AutoStageAll))
		}
		if !ptrBoolEq(got.Verbose, true) {
			t.Errorf("Verbose = %s, want non-nil true", ptrBoolDesc(got.Verbose))
		}
	})
	t.Run("false", func(t *testing.T) {
		repo := initGitRepo(t)
		// Set with the bare string "false"; readGitConfig reads it via --bool.
		gitSet(t, repo, "config", "stagehand.autoStageAll", "false")
		got, err := readGitConfig(repo)
		if err != nil {
			t.Fatalf("readGitConfig: %v", err)
		}
		if got.AutoStageAll == nil {
			t.Fatal("AutoStageAll = nil, want non-nil pointer to false")
		}
		if *got.AutoStageAll != false {
			t.Errorf("AutoStageAll = %v, want false", *got.AutoStageAll)
		}
	})
}

// TestReadGitConfig_TimeoutBareSeconds asserts the §16.3 bare-integer "90"
// (no unit) parses to 90s via parseDuration inside readGitConfig.
func TestReadGitConfig_TimeoutBareSeconds(t *testing.T) {
	repo := initGitRepo(t)
	gitSet(t, repo, "config", "stagehand.timeout", "90")
	got, err := readGitConfig(repo)
	if err != nil {
		t.Fatalf("readGitConfig: %v", err)
	}
	if !ptrDurEq(got.Timeout, 90*time.Second) {
		t.Errorf("Timeout = %s, want non-nil 90s", ptrDurDesc(got.Timeout))
	}
}

// TestReadGitConfig_AllBoolKeys asserts all four --bool keys (autoStageAll,
// verbose, noColor, stripCodeFence) populate when set.
func TestReadGitConfig_AllBoolKeys(t *testing.T) {
	repo := initGitRepo(t)
	gitSet(t, repo, "config", "--bool", "stagehand.noColor", "true")
	gitSet(t, repo, "config", "--bool", "stagehand.stripCodeFence", "false")
	got, err := readGitConfig(repo)
	if err != nil {
		t.Fatalf("readGitConfig: %v", err)
	}
	if !ptrBoolEq(got.NoColor, true) {
		t.Errorf("NoColor = %s, want non-nil true", ptrBoolDesc(got.NoColor))
	}
	if got.StripCodeFence == nil {
		t.Fatal("StripCodeFence = nil, want non-nil pointer to false")
	}
	if *got.StripCodeFence != false {
		t.Errorf("StripCodeFence = %v, want false", *got.StripCodeFence)
	}
}

// --- pointer-comparison helpers --------------------------------------------

func ptrStrEq(got *string, want string) bool { return got != nil && *got == want }
func ptrBoolEq(got *bool, want bool) bool    { return got != nil && *got == want }
func ptrIntEq(got *int, want int) bool       { return got != nil && *got == want }
func ptrDurEq(got *time.Duration, want time.Duration) bool {
	return got != nil && *got == want
}

// ptrXDesc helpers render a pointer for a readable failure message.
func ptrStrDesc(got *string) string {
	if got == nil {
		return "nil"
	}
	return "non-nil " + *got
}
func ptrBoolDesc(got *bool) string {
	if got == nil {
		return "nil"
	}
	return "non-nil " + boolStr(*got)
}
func ptrIntDesc(got *int) string {
	if got == nil {
		return "nil"
	}
	return "non-nil value"
}
func ptrDurDesc(got *time.Duration) string {
	if got == nil {
		return "nil"
	}
	return "non-nil value"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// assertEmptyOverlay fatals/fails if any field of o is set — used by the
// missing-file / unset-key tests.
func assertEmptyOverlay(t *testing.T, o overlay) {
	t.Helper()
	wantNil(t, o.Provider, "Provider")
	wantNil(t, o.Model, "Model")
	wantNil(t, o.Timeout, "Timeout")
	wantNil(t, o.AutoStageAll, "AutoStageAll")
	wantNil(t, o.Verbose, "Verbose")
	wantNil(t, o.NoColor, "NoColor")
	wantNil(t, o.MaxDiffBytes, "MaxDiffBytes")
	wantNil(t, o.MaxMdLines, "MaxMdLines")
	wantNil(t, o.MaxDuplicateRetries, "MaxDuplicateRetries")
	wantNil(t, o.SubjectTargetChars, "SubjectTargetChars")
	wantNil(t, o.Output, "Output")
	wantNil(t, o.StripCodeFence, "StripCodeFence")
	if o.ProviderOverrides != nil {
		t.Errorf("ProviderOverrides = %v, want nil", o.ProviderOverrides)
	}
}
