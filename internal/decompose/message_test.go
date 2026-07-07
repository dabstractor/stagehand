package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
)

// --- Fixture helpers (msg*-prefixed to avoid colliding with planner_test.go's un-prefixed
//     copies AND stager_test.go's stg* copies — all in package decompose) ---

func msgInitRepo(t *testing.T, dir string) {
	t.Helper()
	msgRunGit(t, dir, "init")
	msgRunGit(t, dir, "config", "user.name", "Test")
	msgRunGit(t, dir, "config", "user.email", "test@example.com")
}

func msgWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("msgWriteFile %s: %v", full, err)
	}
}

func msgStageFile(t *testing.T, dir, name string) {
	t.Helper()
	msgRunGit(t, dir, "add", name)
}

func msgCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	msgRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

func msgRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func msgGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return msgRunGit(t, dir, args...)
}

func msgHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return msgGitOut(t, dir, "rev-parse", "HEAD")
}

var msgShaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// messageDeps builds a minimal Deps for message tests (no ResolveRoles).
func messageDeps(t *testing.T, repo string, m provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Message: m},
		Verbose: nil,
	}
}

// --- generateMessage tests ---

func TestGenerateMessage_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	// Build two trees via git add + write-tree.
	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add b"})
	deps := messageDeps(t, repo, m)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	if msg != "feat: add b" {
		t.Errorf("msg = %q, want %q", msg, "feat: add b")
	}
}

func TestGenerateMessage_DedupeRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "feat: existing") // HEAD subject = "feat: existing"

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.NewScript(t, bin, []string{"feat: existing", "feat: fresh"})
	deps := messageDeps(t, repo, m)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	if msg != "feat: fresh" {
		t.Errorf("msg = %q, want %q (duplicate should have been rejected)", msg, "feat: fresh")
	}
}

func TestGenerateMessage_ParseFailRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.NewScript(t, bin, []string{""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // single attempt → blank → loop exhausted → rescue
	deps := Deps{
		Git:    git.New(repo),
		Config: cfg,
		Roles:  RoleManifests{Message: m},
	}

	_, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err == nil {
		t.Fatal("expected error on parse-fail rescue, got nil")
	}

	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if re.Kind != generate.ErrRescue {
		t.Errorf("re.Kind = %v, want ErrRescue", re.Kind)
	}
	if re.TreeSHA != treeB {
		t.Errorf("re.TreeSHA = %q, want %q", re.TreeSHA, treeB)
	}
}

func TestGenerateMessage_Timeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond
	m := stubtest.Manifest(bin, stubtest.Options{SleepMS: 2000})
	deps := messageDeps(t, repo, m)
	deps.Config = cfg

	_, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}

	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if re.Kind != generate.ErrTimeout {
		t.Errorf("re.Kind = %v, want ErrTimeout", re.Kind)
	}
	if !errors.Is(err, generate.ErrTimeout) {
		t.Errorf("errors.Is(err, generate.ErrTimeout) = false, want true")
	}
}

func TestGenerateMessage_EmptyDiff(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	tree := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x"})
	deps := messageDeps(t, repo, m)

	_, err := generateMessage(context.Background(), deps, tree, tree)
	if err == nil {
		t.Fatal("expected error on empty diff, got nil")
	}
	if !errors.Is(err, ErrMessageFailed) {
		t.Errorf("errors.Is(err, ErrMessageFailed) = false, error = %v", err)
	}
	if !strings.Contains(err.Error(), "empty concept diff") {
		t.Errorf("error message does not contain 'empty concept diff': %v", err)
	}
}

// --- publishCommit tests ---

func TestPublishCommit_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	newSHA, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new")
	if err != nil {
		t.Fatalf("publishCommit: %v", err)
	}
	if !msgShaRe.MatchString(newSHA) {
		t.Errorf("newSHA = %q, want hex SHA", newSHA)
	}
	if got := msgHeadSHA(t, repo); got != newSHA {
		t.Errorf("HEAD = %q, want %q", got, newSHA)
	}
	logMsg := msgGitOut(t, repo, "log", "--format=%B", "-n1", newSHA)
	if logMsg != "feat: add new" {
		t.Errorf("git log message = %q, want %q", logMsg, "feat: add new")
	}
	headTree := msgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	if headTree != tree {
		t.Errorf("HEAD tree = %q, want %q", headTree, tree)
	}
}

func TestPublishCommit_RootCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo) // UNBORN — no commits yet

	msgWriteFile(t, repo, "root.txt", "x\n")
	msgStageFile(t, repo, "root.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	newSHA, err := publishCommit(context.Background(), deps, tree, "", "feat: root")
	if err != nil {
		t.Fatalf("publishCommit: %v", err)
	}
	if got := msgHeadSHA(t, repo); got != newSHA {
		t.Errorf("HEAD = %q, want %q", got, newSHA)
	}
	// Verify no parent line.
	parents := msgGitOut(t, repo, "log", "--format=%P", "-n1")
	if parents != "" {
		t.Errorf("root commit has parent = %q, want empty", parents)
	}
}

func TestPublishCommit_CASFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo) // == X

	// Pre-move HEAD via a concurrent commit (HEAD X→Z).
	msgCommitRaw(t, repo, "concurrent")
	actualZ := msgHeadSHA(t, repo) // == Z

	msgWriteFile(t, repo, "new.txt", "data\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	_, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: msg")
	if err == nil {
		t.Fatal("expected error on CAS failure, got nil")
	}

	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CASError", err)
	}
	if ce.Expected != parentSHA {
		t.Errorf("ce.Expected = %q, want %q (parentSHA)", ce.Expected, parentSHA)
	}
	if ce.Actual != actualZ {
		t.Errorf("ce.Actual = %q, want %q (actualZ)", ce.Actual, actualZ)
	}
	// HEAD UNMOVED — the CAS refused to clobber.
	if got := msgHeadSHA(t, repo); got != actualZ {
		t.Errorf("HEAD = %q, want %q (unchanged)", got, actualZ)
	}
	if !strings.Contains(ce.Error(), "HEAD moved") {
		t.Errorf("CASError.Error() does not contain 'HEAD moved': %s", ce.Error())
	}
}

func TestGenerateMessage_ResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	// Build two trees via git add + write-tree (non-empty diff so generateMessage doesn't short-circuit).
	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add b"})
	pflag := "--provider"
	m.ProviderFlag = &pflag // pi-shaped: ProviderFlag triggers slash-prefix splitting
	mf := "--model"
	m.ModelFlag = &mf
	dm := "gpt-5.4"
	m.DefaultModel = &dm

	deps := messageDeps(t, repo, m)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter

	var buf bytes.Buffer
	deps.Verbose = ui.NewVerbose(&buf, true)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	_ = msg

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("message command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("message command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}

// --- publishCommit hook wiring tests (P1.M3.T3.S1 — PRD §9.25 FR-V1/V7/V8c) ---

// msgInstallHook writes an executable hook script to <repo>/.git/hooks/<name>, mode 0755 (the
// owner-exec bit is what hookExecutable checks — without it the hook is skipped and the test is
// vacuous). Mirrors internal/generate/hooks_freeze_test.go's hook-install idiom.
func msgInstallHook(t *testing.T, repo, name, body string) {
	t.Helper()
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write %s hook: %v", name, err)
	}
}

// TestPublishCommit_PrepareCommitMsgAnnotates proves publishCommit runs the repo's commit hooks
// around CommitTree: a prepare-commit-msg that appends a marker to the message file lands a commit
// whose committed message carries the append (hooks ran + the annotated finalMsg was committed).
// PRD §9.25 FR-V1/V8c.
func TestPublishCommit_PrepareCommitMsgAnnotates(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	// Install a prepare-commit-msg that appends a marker line to the message file ($1).
	msgInstallHook(t, repo, "prepare-commit-msg", "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	if _, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new"); err != nil {
		t.Fatalf("publishCommit: %v", err)
	}

	logMsg := msgGitOut(t, repo, "log", "--format=%B", "-n1")
	if !strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]\n") {
		t.Errorf("committed message = %q, want [HOOK-RAN] on its own line (the hook append must not glue onto the subject — Issue 2 parity)", logMsg)
	}
}

// TestPublishCommit_PreCommitAbort_RescueError proves a non-zero pre-commit aborts the commit via
// the existing rescue recipe (FR-V7): publishCommit returns *generate.RescueError (propagated
// DIRECTLY, not wrapped in ErrPublicationFailed) and HEAD is unchanged (no CommitTree/update-ref ran).
func TestPublishCommit_PreCommitAbort_RescueError(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	// Install a pre-commit that exits 1.
	msgInstallHook(t, repo, "pre-commit", "#!/bin/sh\nexit 1\n")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	_, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new")
	if err == nil {
		t.Fatal("expected error on pre-commit abort, got nil")
	}

	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *generate.RescueError (FR-V7)", err)
	}
	if errors.Is(err, ErrPublicationFailed) {
		t.Error("RescueError was wrapped in ErrPublicationFailed — should be propagated DIRECTLY")
	}

	// HEAD UNCHANGED — no CommitTree/update-ref ran (FR-V7 idempotent).
	if got := msgHeadSHA(t, repo); got != parentSHA {
		t.Errorf("HEAD = %q, want %q (unchanged — FR-V7 rescue leaves HEAD untouched)", got, parentSHA)
	}
}

// TestPublishCommit_HookEmptiesMessage_Aborts is the Issue-4 guard on the decompose path: a commit-msg hook
// that empties the message file must NOT create an empty-message commit. git aborts "Aborting commit due to
// empty commit message."; stagehand returns the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue — it is
// neither *RescueError nor *CASError). Mirrors TestPublishCommit_PreCommitAbort_RescueError's structure but
// swaps the hook (commit-msg `> "$1"; exit 0`) + the assertion (errors.Is(err, generate.ErrEmptyMessage)).
// FAILS before the guard (publishCommit creates an empty-message commit → err==nil, HEAD moved); PASSES after.
func TestPublishCommit_HookEmptiesMessage_Aborts(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	// A commit-msg hook that empties the message file (exit 0 ⇒ not a hook failure; the guard catches the
	// EMPTY result). commit-msg is the last hook to touch the file ⇒ finalMsg unambiguously "".
	msgInstallHook(t, repo, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	_, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new") // NON-empty msg (the hook empties it)
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("error type = %T, want generate.ErrEmptyMessage (bare, exit 1 — NOT *RescueError/exit 3, NOT *CASError): %v", err, err)
	}
	// The error must NOT be a rescue (exit 3) or a CAS partial — and must NOT be wrapped in ErrPublicationFailed.
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Errorf("error is *generate.RescueError (exit 3) — the empty-message abort must be the BARE generate.ErrEmptyMessage (exit 1)")
	}
	if errors.Is(err, ErrPublicationFailed) {
		t.Errorf("error is wrapped in ErrPublicationFailed — generate.ErrEmptyMessage must propagate DIRECTLY")
	}

	// NO commit created (HEAD unchanged — the abort returned before CommitTree+UpdateRefCAS; FR-V7 idempotent).
	if got := msgHeadSHA(t, repo); got != parentSHA {
		t.Errorf("HEAD = %q, want %q (unchanged — the empty-message abort returned before CommitTree)", got, parentSHA)
	}
}
