// runner.go is the commit-hooks runner core (PRD §9.25 FR-V1–V8): it drives the repo's commit hooks
// around the plumbing commit path while keeping the snapshot-based atomic-commit core intact.
//
// RunCommitHooks runs pre-commit → prepare-commit-msg → commit-msg BETWEEN message generation and
// commit-tree, scoped to the snapshot tree (FR-V3: pre-commit runs against a THROWAWAY index, the live
// .git/index is never touched). RunPostCommit runs post-commit AFTER update-ref succeeds (best-effort;
// its exit code is DISREGARDED). The runner mirrors `git commit`'s hook semantics (order, env, args,
// --no-verify scope, X_OK skip, timeout) so a user's husky/lint-staged/conventional-commit-lint/notify
// hooks fire on a stagehand-produced commit.
//
// This file owns ONLY the runner. Recursion prevention (skip stagehand's OWN prepare-commit-msg via
// hook.Detect) is the S2 sibling (P1.M3.T1.S2) via the shouldSkipStagehandPrepareCommitMsg seam; caller
// wiring is P1.M3.T2/M3.T3; the FR-V3 subset backstop lives in subset.go (P1.M2.T2.S1).
package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/ui"
)

// HookOpts carries the runner's per-call options.
//
// DryRun (FR-V8a) gates pre-commit + post-commit (skip); commit-msg + prepare-commit-msg STILL run
// (the user sees lint results on the would-be message). Verbose is nil-safe (nil ⇒ silent progress).
type HookOpts struct {
	DryRun  bool
	Verbose *ui.Verbose
}

// RunCommitHooks runs the repo's pre-commit → prepare-commit-msg → commit-msg hooks (in git's
// documented order, external_deps.md §1) BETWEEN message generation and commit-tree, scoped to the
// snapshot tree (FR-V3). It returns the (possibly re-treed) finalTree and the (possibly hook-
// annotated) finalMsg for commit-tree.
//
// Sequence (PRD §9.25 FR-V1–V8):
//   - pre-commit: SKIP if cfg.NoVerify (FR-V5) OR opts.DryRun (FR-V8a). Runs against a THROWAWAY
//     index primed from snapshotTree (the live .git/index is byte-for-byte untouched). A permitted
//     mutation (M/D/T of an existing snapshot path) → re-tree (finalTree = postTree); a NEW path
//     → ErrHookSweptConcurrentWork (the FR-V3 freeze backstop). Non-zero/timeout → *RescueError.
//   - prepare-commit-msg: ALWAYS runs (NoVerify + DryRun do NOT gate it — git-commit(1) parity).
//     Invoked as `<msgfile> ""` (PRD FR-V2; VERIFIED argc=2 for a plain commit — see external_deps.md
//     §2). The S2 seam shouldSkipStagehandPrepareCommitMsg stubs false (stagehand's own hook would
//     recurse; S2 fills via hook.Detect). Non-zero/timeout → *RescueError.
//   - commit-msg: SKIP if cfg.NoVerify (FR-V5); RUNS under DryRun (FR-V8a — lint the would-be
//     message). Invoked as `<msgfile>`. Non-zero/timeout → *RescueError.
//
// Error mapping (two distinct abort kinds, both before commit-tree; HEAD + live index untouched):
//   - hook non-zero exit OR timeout → *generate.RescueError{Kind: ErrRescue, TreeSHA: snapshotTree,
//     ParentSHA, Candidate: msg, Cause} — byte-identical to a generation failure (FR-V7); the caller
//     prints FR44 and exits 3.
//   - pre-commit sweep (enforceSubset) → ErrHookSweptConcurrentWork — a NON-rescue hard error
//     (content-axis freeze violation); the caller surfaces it as a freeze abort.
//
// On any error RunCommitHooks returns ("", "", err). post-commit is SEPARATE (RunPostCommit, called
// by the caller AFTER update-ref succeeds).
func RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
	opts HookOpts) (finalTree, finalMsg string, err error) {

	finalTree, finalMsg = snapshotTree, msg // defaults: no hook mutation / no annotation

	hooksDir, err := g.HooksPath(ctx)
	if err != nil {
		return "", "", fmt.Errorf("hooks: resolve hooks dir: %w", err)
	}
	gitDir, err := g.GitDir(ctx)
	if err != nil {
		return "", "", fmt.Errorf("hooks: resolve git dir: %w", err)
	}
	workTree, err := g.TopLevel(ctx) // §10 — the worktree (CWD) for the hook subprocess.
	if err != nil {
		return "", "", fmt.Errorf("hooks: resolve worktree: %w", err)
	}

	// (b) PRE-COMMIT — skip if --no-verify (FR-V5) or --dry-run (FR-V8a).
	if !(cfg.NoVerify || opts.DryRun) {
		finalTree, err = runPreCommitScoped(ctx, g, cfg, opts, hooksDir, gitDir, workTree,
			snapshotTree, parentSHA, msg)
		if err != nil {
			return "", "", err // *RescueError (non-zero/timeout) or ErrHookSweptConcurrentWork (sweep)
		}
	}

	// (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun do NOT gate it; FR-V1/FR-V8a).
	finalMsg, err = runPrepareCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree,
		snapshotTree, parentSHA, finalMsg)
	if err != nil {
		return "", "", err // *RescueError
	}

	// (d) COMMIT-MSG — skip if --no-verify (FR-V5); RUNS under --dry-run (FR-V8a: lint the would-be msg).
	if !cfg.NoVerify {
		finalMsg, err = runCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree,
			snapshotTree, parentSHA, finalMsg)
		if err != nil {
			return "", "", err // *RescueError
		}
	}

	return finalTree, finalMsg, nil
}

// runPreCommitScoped runs pre-commit against a THROWAWAY index primed from snapshotTree (FR-V3: the
// live .git/index is never touched). It returns the post-hook tree (re-treed if the hook mutated
// snapshot paths), *RescueError (non-zero/timeout), or ErrHookSweptConcurrentWork (the hook staged a
// new path not in the snapshot).
func runPreCommitScoped(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, snapshotTree, parentSHA, msg string) (string, error) {
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if !hookExecutable(hookPath) {
		return snapshotTree, nil // absent/non-executable → silent skip (git access(X_OK) parity)
	}
	tmpIndex, err := tmpIndexPath() // throwaway path (os.TempDir + rand)
	if err != nil {
		return "", fmt.Errorf("hooks: create scoped index: %w", err)
	}
	defer os.Remove(tmpIndex) // best-effort cleanup

	// Prime the throwaway index from snapshotTree (does NOT touch .git/index).
	if err := g.ReadTreeInto(ctx, snapshotTree, tmpIndex); err != nil {
		return "", fmt.Errorf("hooks: prime scoped index: %w", err)
	}
	// run pre-commit (0 args) with GIT_INDEX_FILE=<abs tmpIndex>, GIT_EDITOR=:, GIT_DIR=<gitDir>.
	exitErr := runHook(ctx, cfg.HookTimeout, hookPath, nil, gitDir, workTree,
		map[string]string{"GIT_INDEX_FILE": tmpIndex}, opts)
	if exitErr != nil {
		return "", rescueErr(snapshotTree, parentSHA, msg, "pre-commit", exitErr) // *RescueError
	}
	postTree, err := g.WriteTreeFrom(ctx, tmpIndex)
	if err != nil {
		return "", fmt.Errorf("hooks: capture post-pre-commit tree: %w", err)
	}
	if postTree == snapshotTree {
		return snapshotTree, nil // no mutation
	}
	// Re-tree on permitted mutation; enforceSubset returns ErrHookSweptConcurrentWork on a new path.
	if err := enforceSubset(ctx, g, snapshotTree, postTree); err != nil {
		return "", err // ErrHookSweptConcurrentWork (non-rescue) or wrapped git error
	}
	return postTree, nil // permitted mutation → re-tree (git-commit parity)
}

// runPrepareCommitMsg writes msg to a temp file, runs prepare-commit-msg <msgfile> "" (PRD FR-V2;
// VERIFIED argc=2 for a plain commit — external_deps.md §2), reads back stripped of #-comments.
// ALWAYS runs (NoVerify/DryRun don't gate it). [SEAM] shouldSkipStagehandPrepareCommitMsg stubs false
// here; S2 (P1.M3.T1.S2) fills it via hook.Detect(hooksDir) == hook.StatusStagehand.
func runPrepareCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, snapshotTree, parentSHA, msg string) (string, error) {
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if !hookExecutable(hookPath) || shouldSkipStagehandPrepareCommitMsg(hooksDir) { // §8 seam — S2 fills
		return msg, nil // absent/non-exec OR stagehand's own hook (recursion) → skip
	}
	readBack, err := runMsgHook(ctx, cfg, opts, hookPath, gitDir, workTree,
		snapshotTree, parentSHA, msg, []string{""}) // PRD FR-V2: <msgfile> "" (VERIFIED argc=2 for a plain commit)
	if err != nil {
		return "", err // *RescueError
	}
	return stripCommentLines(readBack), nil
}

// runCommitMsg runs commit-msg <msgfile>, reads back. Skipped only by --no-verify (NOT dry-run).
func runCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, snapshotTree, parentSHA, msg string) (string, error) {
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if !hookExecutable(hookPath) {
		return msg, nil
	}
	return runMsgHook(ctx, cfg, opts, hookPath, gitDir, workTree,
		snapshotTree, parentSHA, msg, nil) // commit-msg: 1 arg <msgfile>
}

// runMsgHook writes msg to a temp file, runs <hook> <msgfile> [extra...], reads back. Non-zero/timeout
// → *RescueError carrying the full rescue context (snapshotTree/parentSHA/msg — byte-identical to a
// generation failure, FR-V7).
func runMsgHook(ctx context.Context, cfg config.Config, opts HookOpts, hookPath, gitDir, workTree,
	snapshotTree, parentSHA, msg string, extraArgs []string) (string, error) {
	tmpMsg, err := os.CreateTemp("", "stagehand-msg-*.txt")
	if err != nil {
		return "", fmt.Errorf("hooks: create message file: %w", err)
	}
	tmpMsgPath := tmpMsg.Name()
	defer os.Remove(tmpMsgPath)
	if _, werr := tmpMsg.WriteString(msg); werr != nil {
		tmpMsg.Close()
		return "", fmt.Errorf("hooks: write message file: %w", werr)
	}
	tmpMsg.Close()
	args := append([]string{tmpMsgPath}, extraArgs...)
	if exitErr := runHook(ctx, cfg.HookTimeout, hookPath, args, gitDir, workTree, nil, opts); exitErr != nil {
		return "", rescueErr(snapshotTree, parentSHA, msg, filepath.Base(hookPath), exitErr) // *RescueError
	}
	data, rerr := os.ReadFile(tmpMsgPath)
	if rerr != nil {
		return "", fmt.Errorf("hooks: read back message file: %w", rerr)
	}
	return string(data), nil
}

// RunPostCommit runs post-commit AFTER update-ref succeeds (best-effort). 0 args; the same env MINUS
// GIT_INDEX_FILE (the commit is done; no scoped index). Its exit code is DISREGARDED (the commit
// already landed; git itself disregards it — githooks(5): "it cannot affect the outcome of git
// commit"). Non-zero/timeout ⇒ log a warning at --verbose; NEVER undo; NEVER return an aborting error
// (the single FR-V7 exception to "failure aborts"). Skipped under opts.DryRun (FR-V8a: no commit
// landed). Absent/non-executable ⇒ skip.
func RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts) error {
	if opts.DryRun {
		return nil // FR-V8a: post-commit skipped under --dry-run (no commit landed).
	}
	hooksDir, err := g.HooksPath(ctx)
	if err != nil {
		return nil // best-effort: don't fail the run on a discovery error
	}
	gitDir, err := g.GitDir(ctx)
	if err != nil {
		return nil
	}
	workTree, err := g.TopLevel(ctx)
	if err != nil {
		return nil
	}
	hookPath := filepath.Join(hooksDir, "post-commit")
	if !hookExecutable(hookPath) {
		return nil
	}
	if exitErr := runHook(ctx, cfg.HookTimeout, hookPath, nil, gitDir, workTree, nil, opts); exitErr != nil {
		if opts.Verbose != nil { // nil-safe
			opts.Verbose.VerboseWarn(fmt.Sprintf("post-commit hook exited non-zero (commit stands): %v", exitErr))
		}
	}
	return nil // ALWAYS nil — exit code disregarded
}

// ---- unexported helpers ----

// runHook execs a hook script directly via os/exec (NOT via git runWithEnv — hooks are USER SCRIPTS,
// not git commands; runWithEnv runs the git binary and is a *gitRunner method NOT on the Git
// interface). §19 permits direct exec with no shell. Env = os.Environ() + GIT_EDITOR=: + GIT_DIR +
// extraEnv (e.g. GIT_INDEX_FILE for pre-commit). CWD = workTree. stdin = /dev/null (non-interactive;
// external_deps.md §3 — most hooks don't read stdin, /dev/null avoids hangs). stdout/stderr pass
// through verbatim (FR-V6 — a noisy hook is the user's hook). ctx is bounded by timeout (cfg.HookTimeout).
// Returns nil on exit 0, or the causing error (*exec.ExitError on non-zero, context.DeadlineExceeded
// on timeout) — the caller maps it to *RescueError.
func runHook(ctx context.Context, timeout time.Duration, hookPath string, args []string,
	gitDir, workTree string, extraEnv map[string]string, opts HookOpts) error {
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(hctx, hookPath, args...) // []string; NO shell (§19)
	cmd.Dir = workTree
	env := append(os.Environ(), "GIT_EDITOR=:", "GIT_DIR="+gitDir)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	cmd.Stdin = strings.NewReader("") // /dev/null equivalent (non-interactive)
	cmd.Stdout = os.Stdout            // pass-through (FR-V6)
	cmd.Stderr = os.Stderr
	if opts.Verbose != nil { // nil-safe
		opts.Verbose.VerboseCommand(hookPath + " " + strings.Join(args, " "))
	}
	if err := cmd.Run(); err != nil {
		if cerr := hctx.Err(); cerr == context.DeadlineExceeded {
			return cerr // timeout — caller maps to *RescueError (Cause = DeadlineExceeded)
		}
		return err // *exec.ExitError (non-zero) or other start/IO failure
	}
	return nil
}

// hookExecutable reports whether path exists and is executable (git's access(X_OK) parity —
// external_deps.md §4). Absent/non-executable ⇒ the caller silently skips the hook (never an error).
func hookExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o100 != 0 // owner-executable bit
}

// shouldSkipStagehandPrepareCommitMsg is the S2 SEAM: skip stagehand's OWN prepare-commit-msg hook
// (it would `exec stagehand hook exec`, regenerating/recursing). S1 STUBS false (foreign hooks run —
// S1's tests use foreign prepare-commit-msg hooks; the recursion scenario is S2's).
//
// TODO(P1.M3.T1.S2): return hook.Detect(hooksDir) == hook.StatusStagehand (the existing
// internal/hook.Detect returns StatusStagehand if the marker is present).
func shouldSkipStagehandPrepareCommitMsg(hooksDir string) bool {
	_ = hooksDir
	return false
}

// rescueErr maps a hook failure (non-zero/timeout) to *generate.RescueError — byte-identical to a
// generation failure (FR-V7). The rescue context (snapshotTree/parentSHA/msg) is threaded in full so
// the caller (M3.T2) prints FR44 + exits 3 exactly as it would for a generation failure.
func rescueErr(snapshotTree, parentSHA, msg, hookName string, cause error) error {
	return &generate.RescueError{
		Kind:      generate.ErrRescue,
		TreeSHA:   snapshotTree,
		ParentSHA: parentSHA,
		Candidate: msg,
		Cause:     fmt.Errorf("hook %s failed: %w", hookName, cause),
	}
}

// stripCommentLines drops git message-file comment lines (lines beginning with "#", the default
// core.commentChar) introduced by prepare-commit-msg's commented metadata (external_deps.md §6).
func stripCommentLines(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// tmpIndexPath returns a throwaway path for a scoped index file (os.TempDir + a unique name). The
// caller owns the lifecycle (defer os.Remove). NOT under .git/ — keeps the repo clean (open_questions §6).
func tmpIndexPath() (string, error) {
	f, err := os.CreateTemp("", "stagehand-hook-*.idx")
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}
