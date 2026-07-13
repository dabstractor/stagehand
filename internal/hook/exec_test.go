package hook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// boolPtr returns a pointer to b — a local helper for setting *bool Config fields (config.boolPtr is
// unexported; this test package needs its own copy to assign cfg.MultiTurnFallback, now a *bool).
func boolPtr(b bool) *bool { return &b }

// runGit runs a git command in repo dir. Test helper.
func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// mustWriteFile is a test helper that writes a file and fatals on error.
func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// initTempRepo creates a temp git repo with a seed commit and returns its path + the git runner.
func initTempRepo(t *testing.T) (string, git.Git) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	mustWriteFile(t, filepath.Join(dir, "init.txt"), []byte("init\n"))
	runGit(t, dir, "add", "init.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir, git.New(dir)
}

func TestNoOpSource(t *testing.T) {
	for _, src := range []string{"message", "template", "merge", "squash", "commit"} {
		if !NoOpSource(src) {
			t.Errorf("NoOpSource(%q) = false, want true", src)
		}
	}
	for _, src := range []string{"", "chat", "foo", "amend"} {
		if NoOpSource(src) {
			t.Errorf("NoOpSource(%q) = true, want false", src)
		}
	}
}

func TestWriteMessageFile_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg")

	orig := "# Please enter the commit message...\n# more comments\n"
	mustWriteFile(t, path, []byte(orig))

	msg := "feat: add x"
	if err := WriteMessageFile(path, msg); err != nil {
		t.Fatalf("WriteMessageFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := "feat: add x\n\n# Please enter the commit message...\n# more comments\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestWriteMessageFile_EmptyOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg")

	msg := "feat: add y"
	if err := WriteMessageFile(path, msg); err != nil {
		t.Fatalf("WriteMessageFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := "feat: add y\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestRun_SourceGateNoOp(t *testing.T) {
	stubBin := stubtest.Build(t)
	_, g := initTempRepo(t)

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# comments\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: should not appear"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	for _, src := range []string{"message", "template", "merge", "squash", "commit"} {
		err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, src)
		if err != ErrNoOp {
			t.Errorf("source=%q: err=%v, want ErrNoOp", src, err)
		}
		data, _ := os.ReadFile(msgFile)
		if string(data) != "# comments\n" {
			t.Errorf("source=%q: msg-file was modified", src)
		}
	}
}

func TestRun_EmptyDiffNoOp(t *testing.T) {
	stubBin := stubtest.Build(t)
	_, g := initTempRepo(t)

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# comments\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: should not appear"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != ErrNoOp {
		t.Errorf("err=%v, want ErrNoOp (no staged changes)", err)
	}
}

func TestRun_HappyPath(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# Please enter the commit message...\n"
	mustWriteFile(t, msgFile, []byte(orig))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: add x\n\nbody text"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)

	if !strings.HasPrefix(s, "feat: add x\n") {
		t.Errorf("msg-file does not start with generated message; got:\n%s", s)
	}
	if !strings.Contains(s, "# Please enter the commit message...") {
		t.Errorf("comment block missing from msg-file; got:\n%s", s)
	}
}

func TestRun_ParseFailRetryThenOK(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("#\n"))

	m := stubtest.NewScript(t, stubBin, []string{"", "feat: valid message"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 3}

	var buf strings.Builder
	verbose := ui.NewVerbose(&buf, true)

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m, Verbose: verbose}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if !strings.HasPrefix(string(data), "feat: valid message") {
		t.Errorf("msg-file should start with the valid retry message; got:\n%s", string(data))
	}
}

func TestRun_DuplicateRejected(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(repoDir, name)
		mustWriteFile(t, p, []byte(name+"\n"))
		runGit(t, repoDir, "add", name)
		runGit(t, repoDir, "commit", "-m", "feat: add x")
	}

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("#\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: add x"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected error (dup exhaustion), got nil")
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "#\n" {
		t.Errorf("msg-file was modified on dup exhaustion; got:\n%s", string(data))
	}
}

func TestRun_StubExit1_NeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	// Empty output + exit 1 → ParseOutput returns ok=false → retries exhaust → error.
	m := stubtest.Manifest(stubBin, stubtest.Options{Exit: 1, Out: ""})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 1}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected error (stub exit 1 + empty output → parse fail exhaustion), got nil")
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("msg-file was modified on stub exit 1; got:\n%s", string(data))
	}
}

func TestRun_TimeoutNeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	m := stubtest.Manifest(stubBin, stubtest.Options{SleepMS: 5000})
	cfg := config.Config{Timeout: 50 * time.Millisecond, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("msg-file was modified on timeout; got:\n%s", string(data))
	}
}

// TestRun_MessageRoleTimeoutNeverBlock verifies FR-R7 on the hook one-shot path: the message role's
// resolved timeout (ResolveRoleTimeout("message", cfg)) — NOT the flat cfg.Timeout — bounds the hook
// one-shot Execute. Sets the GLOBAL large (30s, which would NOT time out against a 5000ms stub sleep)
// and the MESSAGE-ROLE small (50ms → times out). Asserting "hook generation timed out" + an untouched
// msg-file here proves msgTimeout (50ms), not cfg.Timeout (30s), reached the one-shot Execute. This is
// the positive proof of the P1.M3.T1.S2 hook wiring; TestRun_TimeoutNeverBlock is the behavior-
// preserving-by-default regression canary (there msgTimeout == cfg.Timeout == 50ms). The :252 budget
// display is not exercised here (the one-shot times out before multi-turn); it is grep-guarded instead.
func TestRun_MessageRoleTimeoutNeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	m := stubtest.Manifest(stubBin, stubtest.Options{SleepMS: 5000})
	cfg := config.Config{
		Timeout:             30 * time.Second, // LARGE global (would NOT time out under the old cfg.Timeout read)
		MaxDuplicateRetries: 2,
		Roles:               map[string]config.RoleConfig{"message": {Timeout: 50 * time.Millisecond}}, // SMALL role → times out
	}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("msg-file was modified on message-role timeout; got:\n%s", string(data))
	}
}

func TestRun_NoPlumbing(t *testing.T) {
	src, err := os.ReadFile("exec.go")
	if err != nil {
		t.Fatalf("read exec.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	forbidden := map[string]bool{
		"WriteTree": true, "CommitTree": true, "UpdateRefCAS": true,
		"DiffTree": true, "signal.": true,
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		for f := range forbidden {
			if strings.Contains(trimmed, f) {
				t.Errorf("exec.go:%d: forbidden reference %q in: %s", i+1, f, trimmed)
			}
		}
	}
}

// appendScriptManifest builds an append-mode (SessionMode="append") scripted stub manifest: the stub
// emits `responses` sequentially across calls (one-shot + multi-turn turns). Replicates generate's
// appendScriptManifest (internal/generate/generate_test.go:857) — it is NOT exported, so the hook test
// needs its own copy. Use for the multi-turn success/small-payload tests.
func appendScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	appendMode := "append"
	m.SessionMode = &appendMode
	return m
}

// TestRun_MultiTurnSuccess_WritesMessageFile: one-shot exhausts (call 1 = ""), the FR-T1 gate fires
// (conditions a–d hold), generate.Run consumes the scripted turns, and the final returns the message →
// WriteMessageFile writes it. chunkTokens=4 keeps N bounded but > 1 while the multi-line diff exceeds it.
func TestRun_MultiTurnSuccess_WritesMessageFile(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	// A diff large enough to exceed MultiTurnChunkTokens=4 (the diff body — diff --git/+++/@@/+lines — is
	// well over 4 tokens even for one file, as in generate's TestCommitStaged_MultiTurnFallbackSuccess).
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// script[0]="" ⇒ one-shot parse-fail ⇒ exhaust (MaxDuplicateRetries=0 ⇒ 1 attempt). Then multi-turn
	// consumes ["ok","ok","feat: multi-turn win"] across its turns; the final returns the message.
	m := appendScriptManifest(t, stubBin, []string{"", "ok", "ok", "feat: multi-turn win"})
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0, // one-shot: 1 attempt (the "")
		MultiTurnFallback:    boolPtr(true),
		MultiTurnChunkTokens: 4, // low ⇒ the diff exceeds one chunk ⇒ condition (b) true
		TokenLimit:           0,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("expected multi-turn success (nil err), got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if !strings.HasPrefix(string(data), "feat: multi-turn win") {
		t.Errorf("msg-file should start with the generated message; got:\n%s", string(data))
	}
}

// TestRun_MultiTurnFailure_NeverBlock: an append-mode EXIT-1 stub — one-shot exhausts (exit 1), the
// gate fires, generate.Run fails (turn exit 1 → cause!=nil) → fall through → exhaustion error; the
// msg-file is BYTE-IDENTICAL to its pre-Run content (FR-H5).
func TestRun_MultiTurnFailure_NeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// stubtest.Manifest (single-response, exits 1) + manually set SessionMode="append" so the gate fires.
	m := stubtest.Manifest(stubBin, stubtest.Options{Exit: 1, Out: ""})
	appendMode := "append"
	m.SessionMode = &appendMode
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    boolPtr(true),
		MultiTurnChunkTokens: 4,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Fatal("expected exhaustion error (multi-turn failed), got nil")
	}
	if !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error, got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("FR-H5 violated: msg-file modified on multi-turn failure; got:\n%s", string(data))
	}
}

// TestRun_MultiTurnSkipped_NonAppend: stubtest.NewScript ⇒ SessionMode nil (NOT append) ⇒ the gate's
// outer if (condition d) is false ⇒ skip → existing exhaustion error.
func TestRun_MultiTurnSkipped_NonAppend(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// RAW NewScript ⇒ SessionMode nil (NOT append) ⇒ condition (d) false ⇒ gate skips.
	m := stubtest.NewScript(t, stubBin, []string{""})
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    boolPtr(true),
		MultiTurnChunkTokens: 4,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil || !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error (non-append skip), got: %v", err)
	}
}

// TestRun_MultiTurnSmallPayloadSkip: an append provider but a TINY diff + huge chunkTokens ⇒ condition
// (b) (EstimateTokens(payload) > chunkTokens) is false ⇒ skip → exhaustion error.
func TestRun_MultiTurnSmallPayloadSkip(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "tiny.txt"), []byte("x\n")) // a 1-char change ⇒ tiny payload
	runGit(t, repoDir, "add", "tiny.txt")

	m := appendScriptManifest(t, stubBin, []string{""}) // SessionMode="append" (cond d true)
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    boolPtr(true),
		MultiTurnChunkTokens: 100000, // huge ⇒ EstimateTokens(payload) ≤ chunkTokens ⇒ cond (b) false
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil || !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error (small-payload skip), got: %v", err)
	}
}
