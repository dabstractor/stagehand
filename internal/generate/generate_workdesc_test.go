// generate_workdesc_test.go — work-description mode tests (PRD §9.26 FR-W1–W8).
//
// Covers three layers:
//  1. prompt.BuildWorkDescSystemPrompt / BuildWorkDescPayload — pure unit tests (the description-first
//     payload + the round-budget system prompt).
//  2. parseReadLines / skeletonPaths / nextChunk — pure unit tests for the loose READ <path> protocol
//     parser (FR-W3) and the chunk cursor (FR-W5).
//  3. CommitStaged end-to-end with the stub agent (the full description-first read/answer loop):
//     - happy path (model READs a file, then emits the message) → committed;
//     - round-budget forced conclusion (FR-W6);
//     - the mode does NOT cascade into multi-turn fallback (FR-W7);
//     - non-append provider → rescue (FR-W4, session_mode gate).
package generate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// ---- 1. Prompt builder unit tests ----

func TestBuildWorkDescSystemPrompt_IncludesBudget(t *testing.T) {
	s := prompt.BuildWorkDescSystemPrompt("BASE", 5)
	// The protocol intro and the budget line are both present.
	if !strings.Contains(s, "READ <path>") {
		t.Errorf("system prompt missing READ protocol: %q", s)
	}
	if !strings.Contains(s, "at most 5 responses") {
		t.Errorf("system prompt missing round budget N=5: %q", s)
	}
	if !strings.HasPrefix(s, "BASE\n\n") {
		t.Errorf("system prompt must START with the base prompt: %q", s)
	}
}

func TestBuildWorkDescSystemPrompt_ClampsNonPositive(t *testing.T) {
	// A non-positive budget collapses to 1 (defensive; guarantees termination).
	s := prompt.BuildWorkDescSystemPrompt("BASE", 0)
	if !strings.Contains(s, "at most 1 responses") {
		t.Errorf("non-positive budget should clamp to 1: %q", s)
	}
}

func TestBuildWorkDescPayload_DescriptionFirst(t *testing.T) {
	got := prompt.BuildWorkDescPayload("add login flow", "phrase it casually", "10\t2\tsrc/login.go\n")
	// Description leads (FR-W2: description is content-authoritative, at the top).
	if !strings.HasPrefix(got, "Work description (what this commit does") {
		t.Errorf("payload must lead with the work description: %q", got)
	}
	if !strings.Contains(got, "add login flow") {
		t.Errorf("payload missing the work description text: %q", got)
	}
	// Context block present when set (FR-W1: --context is the _how_).
	if !strings.Contains(got, "phrase it casually") {
		t.Errorf("payload missing the context block: %q", got)
	}
	// Skeleton (the file menu) present (FR-W2).
	if !strings.Contains(got, "src/login.go") {
		t.Errorf("payload missing the skeleton/file menu: %q", got)
	}
	// NO diff bodies (FR-W2: "no diff bodies" — the model READs them on demand).
	if strings.Contains(got, "diff --git") {
		t.Errorf("payload must NOT contain diff bodies: %q", got)
	}
}

func TestBuildWorkDescPayload_NoContext(t *testing.T) {
	got := prompt.BuildWorkDescPayload("fix bug", "", "1\t0\tmain.go\n")
	if strings.Contains(got, "Directing guidance") {
		t.Errorf("empty context must omit the context block: %q", got)
	}
}

// ---- 2. READ-protocol parser unit tests (FR-W3) ----

func TestParseReadLines_Basic(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n5\t0\thelper.go\n"
	got := parseReadLines("Let me check.\nREAD main.go\nThanks", skeleton)
	want := []string{"main.go"}
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("parseReadLines = %v, want %v", got, want)
	}
}

func TestParseReadLines_CaseInsensitiveAndPunctuationForgiving(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// Backticks, commas, mixed case verb, trailing punctuation — all forgiving (FR-W3).
	got := parseReadLines("read `main.go`, please", skeleton)
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("forgiving parse = %v, want [main.go]", got)
	}
}

func TestParseReadLines_MultipleCommaSeparated(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\ta.go\n5\t0\tb.go\n1\t0\tc.go\n"
	got := parseReadLines("READ a.go, b.go\nREAD c.go", skeleton)
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != 3 {
		t.Fatalf("parseReadLines = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("parseReadLines[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestParseReadLines_NonStagedIgnored(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// A path NOT in the skeleton is ignored (FR-W3: non-staged/unrecognized ignored).
	got := parseReadLines("READ other.go\nREAD main.go", skeleton)
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("non-staged READ should be ignored: %v, want [main.go]", got)
	}
}

func TestParseReadLines_NoReadLineIsMessage(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// A response with no valid READ line yields no paths → the caller treats it as the message (FR-W7).
	got := parseReadLines("feat: add login\n\nThis is the commit message.", skeleton)
	if len(got) != 0 {
		t.Errorf("a no-READ response must yield no paths: %v", got)
	}
}

func TestParseReadLines_Deduplicates(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	got := parseReadLines("READ main.go\nREAD main.go", skeleton)
	if len(got) != 1 {
		t.Errorf("duplicate READs must dedupe: %v", got)
	}
}

func TestStripReadLines(t *testing.T) {
	// READ lines are removed; the message remains.
	got := stripReadLines("READ a.go\nfeat: my change\n\nbody")
	if got != "feat: my change\n\nbody" {
		t.Errorf("stripReadLines = %q, want the message sans READ lines", got)
	}
	// An all-READ response yields "" (no message → ParseOutput ok=false → rescue).
	if got := stripReadLines("READ a.go\nREAD b.go"); got != "" {
		t.Errorf("all-READ strip = %q, want empty", got)
	}
}

func TestSkeletonPaths_Parses(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n5\t0\tsub/util.go\n-\t-\tbin.dat\n"
	set := skeletonPaths(skeleton)
	if !set["main.go"] || !set["sub/util.go"] || !set["bin.dat"] {
		t.Errorf("skeletonPaths = %v, want main.go+sub/util.go+bin.dat", set)
	}
}

func TestSkeletonPaths_Empty(t *testing.T) {
	if skeletonPaths("") != nil {
		t.Error("empty skeleton must yield nil")
	}
	if skeletonPaths("Change summary (numstat: added\tdeleted\tpath):\n") != nil {
		t.Error("header-only skeleton must yield nil")
	}
}

func TestNextChunk_SmallDiffIsOneChunk(t *testing.T) {
	diff := "diff --git a/x b/x\n+hello\n"
	chunk, total, advance := nextChunk(diff, 0)
	if total != 1 {
		t.Errorf("small diff total = %d, want 1", total)
	}
	if chunk != diff {
		t.Errorf("small diff chunk = %q, want the whole diff", chunk)
	}
	if advance != len(diff) {
		t.Errorf("advance = %d, want %d", advance, len(diff))
	}
	// Cursor exhausted → empty chunk (FR-W5 end-of-diff).
	c2, _, a2 := nextChunk(diff, len(diff))
	if c2 != "" || a2 != 0 {
		t.Errorf("exhausted cursor chunk = %q advance = %d, want empty/0", c2, a2)
	}
}

// ---- 3. CommitStaged end-to-end (work-description mode) ----

// TestCommitStaged_WorkDescription_HappyPath: the model READs a staged file, then emits a unique
// commit message. The commit lands with that message; HEAD advances; the staged file is committed.
func TestCommitStaged_WorkDescription_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc NewFeature() string { return \"x\" }\n")
	stageFile(t, repo, "feature.go")

	beforeHEAD := headSHA(t, repo)

	// Script: turn 1 = "READ feature.go" (a READ request); turn 2 = the commit message (no READ).
	// SessionMode="append" so RenderMultiTurn's gate passes (FR-W4).
	m := appendScriptManifest(t, bin, []string{"READ feature.go", "feat: add NewFeature function"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add the NewFeature function"

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.CommitSHA == "" {
		t.Fatal("CommitSHA empty — nothing committed")
	}
	if res.Subject != "feat: add NewFeature function" {
		t.Errorf("Subject = %q, want the work-description message", res.Subject)
	}
	if headSHA(t, repo) == beforeHEAD {
		t.Error("HEAD did not advance")
	}
}

// TestCommitStaged_WorkDescription_RoundBudgetForcesConclusion (FR-W6): with a tiny round budget,
// the model keeps requesting READs past the cap; the forced-conclusion turn demands the message and
// the run commits the message from that turn (the cap guarantees termination).
func TestCommitStaged_WorkDescription_RoundBudgetForcesConclusion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
	stageFile(t, repo, "feature.go")

	// Script: turn 1 = READ; turn 2 = READ (now over the cap of 1); turn 3 = the message.
	m := appendScriptManifest(t, bin, []string{"READ feature.go", "READ feature.go", "feat: forced conclusion"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add F"
	cfg.WorkDescReadRounds = 1 // cap = 1 round; the 2nd READ triggers the forced-conclusion turn

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: forced conclusion" {
		t.Errorf("Subject = %q, want the forced-conclusion message", res.Subject)
	}
}

// TestCommitStaged_WorkDescription_NoCascadeToMultiTurn (FR-W7): work-description mode does NOT
// cascade into §9.24 multi-turn fallback even when the final message is empty/duplicate. A no-valid-
// message run rescues (the existing §9.10 protocol), never multi-turn.
func TestCommitStaged_WorkDescription_NoCascadeToMultiTurn(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "big.txt", strings.Repeat("line\n", 2000)) // huge payload ⇒ multi-turn WOULD trigger if it cascaded
	stageFile(t, repo, "big.txt")

	// Script: every response is a READ that never resolves to a message → after the cap, forced-
	// conclusion turn yields "" (empty) → no valid message → rescue (FR-W7). Multi-turn would have
	// consumed many script entries delivering chunks; here the rescue fires without that.
	m := appendScriptManifest(t, bin, []string{"READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", ""})
	cfg := config.Defaults()
	cfg.WorkDescription = "huge change"
	cfg.WorkDescReadRounds = 3
	cfg.MultiTurnFallback = true // enabled — but FR-W7 says it must NOT trigger
	cfg.MultiTurnChunkTokens = 4 // tiny ⇒ multi-turn WOULD fire on the default path
	cfg.MaxDuplicateRetries = 0

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected a rescue error (no valid message), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-W7 rescue, not a multi-turn cascade)", err)
	}
	if re.Kind != ErrRescue {
		t.Errorf("Kind = %v, want ErrRescue", re.Kind)
	}
}

// TestCommitStaged_WorkDescription_NonAppendProviderRescues (FR-W4): a provider without
// session_mode="append" yields a turn-1 RenderMultiTurn error → rescue (provider support identical
// to §9.24). The run must NOT commit.
func TestCommitStaged_WorkDescription_NonAppendProviderRescues(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")

	beforeHEAD := headSHA(t, repo)

	// RAW NewScript ⇒ SessionMode unset (⇒ "" after Resolve) ⇒ RenderMultiTurn's gate fails.
	m := stubtest.NewScript(t, bin, []string{"feat: x"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add x"

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected a rescue (non-append provider), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-W4 non-append rescue)", err)
	}
	// HEAD must be unchanged (never committed).
	if headSHA(t, repo) != beforeHEAD {
		t.Errorf("HEAD moved on a non-append-provider rescue (repo must be unchanged)")
	}
}

// TestCommitStaged_WorkDescription_OffByDefault (FR-W1): an empty WorkDescription runs the default
// diff-first path unchanged. Sanity: the feature is opt-in and never the default.
func TestCommitStaged_WorkDescription_OffByDefault(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")

	// Default path: one-shot, single response, no READ protocol. SessionMode unset is fine (Render, not
	// RenderMultiTurn). WorkDescription == "" ⇒ the default loop runs.
	m := stubtest.NewScript(t, bin, []string{"feat: default path"})
	cfg := config.Defaults() // WorkDescription == ""

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: default path" {
		t.Errorf("Subject = %q, want the default-path message", res.Subject)
	}
}

// TestStagedNumstatSkeleton_MirrorsStagedSet: the skeleton returned by git.StagedNumstatSkeleton is
// the same file menu StagedDiff prepends — it is the READ-able path set (FR-W2).
func TestStagedNumstatSkeleton_MirrorsStagedSet(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.go", "package main\n")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	g := git.New(repo)
	skeleton, err := g.StagedNumstatSkeleton(context.Background(), git.StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedNumstatSkeleton: %v", err)
	}
	if !strings.Contains(skeleton, "a.go") || !strings.Contains(skeleton, "b.go") {
		t.Errorf("skeleton = %q, must list a.go and b.go", skeleton)
	}
	if strings.Contains(skeleton, "diff --git") {
		t.Errorf("skeleton must NOT contain diff bodies: %q", skeleton)
	}
}

// TestStagedFileDiff_SinglePath: StagedFileDiff returns ONE staged file's diff body, no skeleton.
func TestStagedFileDiff_SinglePath(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.go", "package main\n")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	g := git.New(repo)
	diff, err := g.StagedFileDiff(context.Background(), "a.go", git.StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedFileDiff: %v", err)
	}
	if !strings.Contains(diff, "a.go") {
		t.Errorf("StagedFileDiff(a.go) = %q, must mention a.go", diff)
	}
	if strings.Contains(diff, "b.go") {
		t.Errorf("StagedFileDiff(a.go) must NOT mention b.go: %q", diff)
	}
	// A non-staged path yields "" (the caller notes "not in the staged changes").
	missing, _ := g.StagedFileDiff(context.Background(), "nonexistent.go", git.StagedDiffOptions{DiffContext: 1})
	if missing != "" {
		t.Errorf("StagedFileDiff(nonexistent) = %q, want empty", missing)
	}
}
