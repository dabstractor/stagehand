package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
	"github.com/dustin/stagecoach/internal/ui"
)

// arbStrPtr / arbBoolPtr are local pointer helpers (provider.strPtr/boolPtr are unexported).
func arbStrPtr(s string) *string { return &s }
func arbBoolPtr(b bool) *bool    { return &b }

// --- Fixture helpers (arb*-prefixed to avoid collisions with planner_test/stager_test/message_test) ---

// arbInitRepo creates a git repo in dir with repo-local identity config (no env pollution).
func arbInitRepo(t *testing.T, dir string) {
	t.Helper()
	arbRunGit(t, dir, "init")
	arbRunGit(t, dir, "config", "user.name", "Test")
	arbRunGit(t, dir, "config", "user.email", "test@example.com")
}

// arbWriteFile creates a file at dir/name with the given body.
func arbWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("arbWriteFile %s: %v", full, err)
	}
}

// arbStageFile runs git add for name in dir.
func arbStageFile(t *testing.T, dir, name string) {
	t.Helper()
	arbRunGit(t, dir, "add", name)
}

// arbCommitRaw creates an empty commit with the given message.
func arbCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	arbRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// arbRunGit executes git -C dir args... and returns trimmed stdout.
func arbRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// arbDeps builds a minimal Deps for arbiter tests (no ResolveRoles).
func arbDeps(t *testing.T, repo string, m provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Arbiter: m},
		Verbose: nil,
	}
}

// arbCommits builds a small []CommitInfo from a real repo: two commits with known files,
// then populates Files via DiffTree.
func arbCommits(t *testing.T, repo string, ctx context.Context) []CommitInfo {
	t.Helper()
	arbCommitRaw(t, repo, "feat: first commit")
	arbWriteFile(t, repo, "a.go", "package main")
	arbStageFile(t, repo, "a.go")
	arbCommitRaw(t, repo, "feat: add a.go")
	arbWriteFile(t, repo, "b.go", "package b")
	arbStageFile(t, repo, "b.go")
	arbCommitRaw(t, repo, "feat: add b.go")

	// Get the SHAs in chronological order (oldest first).
	shas := strings.Split(arbRunGit(t, repo, "log", "--format=%H", "--reverse"), "\n")
	if len(shas) < 3 {
		t.Fatalf("expected 3 commits, got %d", len(shas))
	}
	// We want commits 1 and 2 (the ones with files).
	shaA := shas[1] // "feat: add a.go"
	shaB := shas[2] // "feat: add b.go"

	g := git.New(repo)
	filesA, err := g.DiffTree(ctx, shaA, false)
	if err != nil {
		t.Fatalf("DiffTree(%s): %v", shaA, err)
	}
	filesB, err := g.DiffTree(ctx, shaB, false)
	if err != nil {
		t.Fatalf("DiffTree(%s): %v", shaB, err)
	}

	return []CommitInfo{
		{SHA: shaA, Subject: "feat: add a.go", Files: filesA},
		{SHA: shaB, Subject: "feat: add b.go", Files: filesB},
	}
}

// --- Tests ---

func TestRunArbiter_ConfidentTarget(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())
	shaA := commits[0].SHA

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": "` + shaA + `"}`})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "diff --git leftover")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target == nil {
		t.Fatal("Target is nil, want non-nil")
	}
	if *out.Target != shaA {
		t.Errorf("Target = %q, want %q", *out.Target, shaA)
	}
}

func TestRunArbiter_NullTarget(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "some leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil", *out.Target)
	}
}

func TestRunArbiter_TargetNotInList(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	bogus := "0123456789abcdef0123456789abcdef01234567"
	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": "` + bogus + `"}`})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil (not-in-list should degrade to null)", *out.Target)
	}
}

func TestRunArbiter_EmptyTarget(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": ""}`})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil (empty target should degrade to null)", *out.Target)
	}
}

func TestRunArbiter_ParseFailureNull(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: "not json at all"})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v (expected nil error, graceful null on parse failure)", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil", *out.Target)
	}
}

func TestRunArbiter_TimeoutNull(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`, SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond

	deps := arbDeps(t, repo, m)
	deps.Config = cfg

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v (expected nil error, graceful null on timeout)", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil (timeout should degrade to null)", *out.Target)
	}
}

// TestRunArbiter_PerRoleTimeoutNull proves the per-role [role.arbiter].timeout override bounds
// runArbiter's Execute, not the global cfg.Timeout (FR-R7, §9.15/§16.1). With the GLOBAL set large
// (30s — would NOT time out vs the 2000ms stub sleep) and the PER-ROLE small (100ms — times out),
// the timeout firing proves ResolveRoleTimeout("arbiter", …) reached Execute. The arbiter has NO
// built-in timeout, so with no override the global is used (behavior-preserving — proven by
// TestRunArbiter_TimeoutNull above). The failure semantic is graceful null (§13.6.5 "when in doubt,
// null"), UNCHANGED. Clone of S1's TestCommitStaged_MessageRoleTimeout.
func TestRunArbiter_PerRoleTimeoutNull(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`, SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 30 * time.Second // LARGE — 2000ms stub sleep would NOT time out here
	cfg.Roles = map[string]config.RoleConfig{
		"arbiter": {Timeout: 100 * time.Millisecond}, // SMALL → times out (proves ResolveRoleTimeout bounds Execute)
	}

	deps := arbDeps(t, repo, m)
	deps.Config = cfg

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v (expected nil error, graceful null on per-role timeout)", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil (per-role timeout should degrade to null)", *out.Target)
	}
}

func TestRunArbiter_NonZeroExitValidStdout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())
	shaA := commits[0].SHA

	m := stubtest.Manifest(bin, stubtest.Options{Exit: 1, Out: `{"target": "` + shaA + `"}`})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target == nil {
		t.Fatal("Target is nil, want non-nil (non-zero exit should fall through to parse)")
	}
	if *out.Target != shaA {
		t.Errorf("Target = %q, want %q", *out.Target, shaA)
	}
}

func TestRunArbiter_RenderError(t *testing.T) {
	// Build a manifest with Command="" so Validate fails inside Render.
	m := provider.Manifest{
		Name:           "bad-stub",
		Command:        nil,
		PromptDelivery: arbStrPtr("stdin"),
		Output:         arbStrPtr("raw"),
		StripCodeFence: arbBoolPtr(true),
	}
	deps := arbDeps(t, t.TempDir(), m)

	_, err := runArbiter(context.Background(), deps, nil, "leftover")
	if err == nil {
		t.Fatal("expected error on render failure, got nil")
	}
	if !errors.Is(err, ErrArbiterFailed) {
		t.Errorf("errors.Is(err, ErrArbiterFailed) = false, error = %v", err)
	}
}

func TestRunArbiter_NoRetry(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	// Use a script with one response (invalid JSON); the counter file tells us how many times the
	// stub was invoked. NO retry means the counter should be exactly 1.
	dir := t.TempDir()
	counter := dir + "/counter"
	script := dir + "/script.txt"
	if err := os.WriteFile(script, []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	m := stubtest.Manifest(bin, stubtest.Options{Script: script, Counter: counter})
	deps := arbDeps(t, repo, m)

	out, err := runArbiter(context.Background(), deps, commits, "leftover diff")
	if err != nil {
		t.Fatalf("runArbiter: %v (expected nil error on parse-failure null)", err)
	}
	if out.Target != nil {
		t.Errorf("Target = %q, want nil", *out.Target)
	}

	// Read the counter — should be 1 (exactly one Execute call, no retry).
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	count := strings.TrimSpace(string(data))
	if count != "1" {
		t.Errorf("stub call count = %q, want 1 (no retry)", count)
	}
}

func TestRunArbiter_PayloadConversion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())
	shaA := commits[0].SHA

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": "` + shaA + `"}`})
	deps := arbDeps(t, repo, m)

	// Run arbiter — the stub should receive the payload on stdin containing SHA + Subject + paths.
	out, err := runArbiter(context.Background(), deps, commits, "diff --git leftover.txt\n+new line")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	if out.Target == nil || *out.Target != shaA {
		t.Fatalf("unexpected output: %+v", out)
	}
	// If we get here without panic/invalid, the payload was well-formed enough for Render.
}

func TestConvertArbiterCommits(t *testing.T) {
	tests := []struct {
		name     string
		commits  []CommitInfo
		wantLen  int
		wantSHA  []string
		wantPath [][]string // expected Files per commit
	}{
		{
			name:     "empty input",
			commits:  nil,
			wantLen:  0,
			wantSHA:  nil,
			wantPath: nil,
		},
		{
			name: "added files",
			commits: []CommitInfo{
				{SHA: "aaa", Subject: "feat: a", Files: []git.FileChange{{Status: "A", Path: "a.go"}}},
			},
			wantLen:  1,
			wantSHA:  []string{"aaa"},
			wantPath: [][]string{{"a.go"}},
		},
		{
			name: "rename — SrcPath dropped",
			commits: []CommitInfo{
				{SHA: "bbb", Subject: "refactor: rename", Files: []git.FileChange{{Status: "R100", SrcPath: "old.go", Path: "new.go"}}},
			},
			wantLen:  1,
			wantSHA:  []string{"bbb"},
			wantPath: [][]string{{"new.go"}},
		},
		{
			name: "multiple files",
			commits: []CommitInfo{
				{SHA: "ccc", Subject: "feat: multi", Files: []git.FileChange{
					{Status: "A", Path: "x.go"},
					{Status: "M", Path: "y.go"},
					{Status: "D", Path: "z.go"},
				}},
			},
			wantLen:  1,
			wantSHA:  []string{"ccc"},
			wantPath: [][]string{{"x.go", "y.go", "z.go"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, validSHAs := convertArbiterCommits(tc.commits)
			if len(result) != tc.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tc.wantLen)
			}
			if len(validSHAs) != tc.wantLen {
				t.Errorf("len(validSHAs) = %d, want %d", len(validSHAs), tc.wantLen)
			}
			for i, sha := range tc.wantSHA {
				if _, ok := validSHAs[sha]; !ok {
					t.Errorf("validSHAs missing %q", sha)
				}
				if result[i].SHA != sha {
					t.Errorf("result[%d].SHA = %q, want %q", i, result[i].SHA, sha)
				}
			}
			for i, paths := range tc.wantPath {
				if len(result[i].Files) != len(paths) {
					t.Errorf("result[%d].Files len = %d, want %d", i, len(result[i].Files), len(paths))
					continue
				}
				for j, p := range paths {
					if result[i].Files[j] != p {
						t.Errorf("result[%d].Files[%d] = %q, want %q", i, j, result[i].Files[j], p)
					}
				}
			}
		})
	}
}

func TestTargetInRun(t *testing.T) {
	valid := map[string]struct{}{
		"abc123": {},
		"def456": {},
	}

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"empty", "", false},
		{"known", "abc123", true},
		{"unknown", "000000", false},
		{"partial match", "abc12", false}, // exact match required
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := targetInRun(tc.target, valid)
			if got != tc.want {
				t.Errorf("targetInRun(%q) = %v, want %v", tc.target, got, tc.want)
			}
		})
	}
}

// TestBuildArbiterUserPayload verifies the prompt package's payload assembly contains the
// expected SHA, Subject, and path content (unit-level verification of the conversion seam).
func TestBuildArbiterUserPayload_ContainsFields(t *testing.T) {
	commits := []prompt.ArbiterCommit{
		{SHA: "sha1aaa", Subject: "feat: one", Files: []string{"a.go", "b.go"}},
		{SHA: "sha2bbb", Subject: "fix: two", Files: []string{"c.go"}},
	}
	diff := "diff --git leftover.txt\n+new line"

	payload := prompt.BuildArbiterUserPayload(commits, diff)

	if !strings.Contains(payload, "sha1aaa") {
		t.Error("payload missing SHA sha1aaa")
	}
	if !strings.Contains(payload, "feat: one") {
		t.Error("payload missing Subject 'feat: one'")
	}
	if !strings.Contains(payload, "a.go") {
		t.Error("payload missing file 'a.go'")
	}
	if !strings.Contains(payload, "b.go") {
		t.Error("payload missing file 'b.go'")
	}
	if !strings.Contains(payload, "sha2bbb") {
		t.Error("payload missing SHA sha2bbb")
	}
	if !strings.Contains(payload, "fix: two") {
		t.Error("payload missing Subject 'fix: two'")
	}
	if !strings.Contains(payload, "c.go") {
		t.Error("payload missing file 'c.go'")
	}
	// Verify leftover diff is present verbatim.
	if !strings.Contains(payload, diff) {
		t.Error("payload missing leftover diff")
	}
	// Verify Status/SrcPath are NOT in the payload (we only put Path strings).
	if strings.Contains(payload, "R100") {
		t.Error("payload should not contain FileChange.Status 'R100'")
	}
	if strings.Contains(payload, "old.go") && !strings.Contains(diff, "old.go") {
		t.Error("payload should not contain FileChange.SrcPath 'old.go' in commit block")
	}
}

func TestRunArbiter_ResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	arbInitRepo(t, repo)
	commits := arbCommits(t, repo, context.Background())

	m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`})
	pflag := "--provider"
	m.ProviderFlag = &pflag // pi-shaped: ProviderFlag triggers slash-prefix splitting
	m.ModelFlag = arbStrPtr("--model")
	m.DefaultModel = arbStrPtr("gpt-5.4") // fallback model when none pinned

	deps := arbDeps(t, repo, m)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render splits into --provider openrouter

	var buf bytes.Buffer
	deps.Verbose = ui.NewVerbose(&buf, true)

	out, err := runArbiter(context.Background(), deps, commits, "diff --git leftover")
	if err != nil {
		t.Fatalf("runArbiter: %v", err)
	}
	_ = out

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("arbiter command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("arbiter command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}
