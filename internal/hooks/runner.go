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
// hook.Detect) is the S2 sibling (P1.M3.T1.S2) via the shouldSkipStagecoachPrepareCommitMsg seam; caller
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

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/hook"
	"github.com/dustin/stagecoach/internal/ui"
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
//     Invoked as `<msgfile>` (git githooks(5): for a plain commit no second parameter is passed;
//     argc=1, $2 unset). The S2 seam shouldSkipStagecoachPrepareCommitMsg stubs false (stagehand's
//     own hook would recurse; S2 fills via hook.Detect). Non-zero/timeout → *RescueError.
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

	// (c)+(d) MESSAGE-FILE LIFECYCLE — ONE shared temp file for prepare-commit-msg + commit-msg (FR-V2
	// git parity: commit-msg sees prepare's output). Strip ONLY at the final read-back (git cleanup=strip,
	// honoring core.commentChar) — NEVER between the two hooks.
	msgFile, err := os.CreateTemp("", "stagehand-hookmsg-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("hooks: create message file: %w", err)
	}
	msgPath := msgFile.Name()
	defer os.Remove(msgPath)
	// git parity (strbuf_complete_line): git's COMMIT_EDITMSG always ends with \n so an append-style
	// prepare-commit-msg/commit-msg hook (e.g. `echo "Signed-off-by: ..." >> "$1"`) starts on a new
	// line, not concatenated onto the subject. Add a trailing \n if the message lacks one. The
	// mutation is consumed on read-back: stripCommentLines does TrimRight("\n") (see read-back below).
	if !strings.HasSuffix(finalMsg, "\n") {
		finalMsg += "\n"
	}
	if _, werr := msgFile.WriteString(finalMsg); werr != nil {
		msgFile.Close()
		return "", "", fmt.Errorf("hooks: write message file: %w", werr)
	}
	msgFile.Close()

	// (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun do NOT gate it; FR-V1/FR-V8a). Skipped if
	// absent/non-exec OR stagehand's OWN hook (FR-V4 recursion prevention).
	if cerr := runPrepareCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
		return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
			ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("prepare-commit-msg: %w", cerr)}
	}

	// (d) COMMIT-MSG — skip if --no-verify (FR-V5); RUNS under --dry-run (FR-V8a: lint the would-be msg).
	if !cfg.NoVerify {
		if cerr := runCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
			return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
				ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("commit-msg: %w", cerr)}
		}
	}

	// Final read-back (after commit-msg) + strip comment lines (git cleanup=strip; honor core.commentChar).
	commentChar, ccErr := g.CommentChar(ctx)
	if ccErr != nil || commentChar == "" {
		commentChar = "#" // best-effort default — NEVER block the commit on a commentChar read failure
	}
	data, rErr := os.ReadFile(msgPath)
	if rErr != nil {
		return "", "", fmt.Errorf("hooks: read back message file: %w", rErr)
	}
	finalMsg = stripCommentLines(string(data), commentChar)

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

// runPrepareCommitMsg runs prepare-commit-msg <msgPath> (git githooks(5): for a plain commit no
// second parameter is passed; argc=1, $2 unset) on the SHARED message file. ALWAYS runs
// (NoVerify/DryRun don't gate it — the caller gates the OTHER hooks). Skipped if absent/non-exec
// OR stagehand's OWN hook (FR-V4 recursion prevention — invoking stagehand's own prepare-commit-msg
// would exec `stagehand hook exec` and recurse). Returns the CAUSE error on non-zero/timeout (the
// caller wraps the full-context *RescueError).
func runPrepareCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if !hookExecutable(hookPath) {
		return nil // absent/non-exec → silent skip
	}
	if shouldSkipStagecoachPrepareCommitMsg(hooksDir) { // FR-V4: stagehand's OWN hook → skip (recursion)
		if opts.Verbose != nil { // nil-safe
			opts.Verbose.VerboseWarn("skipping stagehand's own prepare-commit-msg hook on the plumbing path (FR-V4 recursion prevention)")
		}
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)
}

// runCommitMsg runs commit-msg <msgPath> on the SHARED message file (sees prepare's output). Returns the
// CAUSE error on non-zero/timeout (the caller wraps the full-context *RescueError).
func runCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if !hookExecutable(hookPath) {
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)
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

// ReconcileIndex syncs the LIVE index entries for the snapshot paths to match the COMMITTED tree
// (finalTree) after a permitted pre-commit hook mutation re-treed. git-commit parity (PRD §9.25 FR-V3,
// report Finding F1): when `git commit` runs a pre-commit hook that modifies and re-stages a file
// (the formatter / lint-staged / prettier workflow), it commits the hook's version AND syncs the
// index to that version, so `git status` is clean and a subsequent plain `git commit` cannot
// re-commit the pre-hook blob. stagehand's atomic-commit core builds the commit from a FROZEN tree
// (PRD §13.2 G4) and runs pre-commit against a THROWAWAY index (FR-V3), so by default the live index
// retains the pre-hook blob and diverges from HEAD — this reconciles exactly the mutated snapshot
// paths (DiffTreeNameStatus(snapshotTree, finalTree)) to finalTree, preserving every OTHER staged
// entry (e.g. files the user staged during generation — PRD §13.2 G4 "stage while generating").
//
// DESIGN (why it is surgical, not a blanket `git read-tree HEAD`): a blanket read-tree would CLOBBER
// any path the user staged during generation that is NOT in the committed tree. SyncIndexPaths updates
// ONLY the snapshot paths to finalTree's blobs, leaving all other index entries untouched.
//
// NO-OP when finalTree == snapshotTree (no hook mutation — the common case: the snapshot already
// equals the index content at freeze time, so the index is already consistent with HEAD). Also a
// no-op when DryRun (no commit landed — the caller must NOT call this under --dry-run).
//
// FAILURE is best-effort (the commit already landed): a wrapped error is RETURNED for the caller to
// log at --verbose; it NEVER undoes the commit and is NOT an abort. DiffTreeNameStatus failure or a
// SyncIndexPaths git error leaves the index as-is (the divergence is cosmetic — `git status` shows
// MM and a subsequent commit would re-stage the pre-hook blob — but the just-made commit is correct).
func ReconcileIndex(ctx context.Context, g git.Git, snapshotTree, finalTree string, opts HookOpts) error {
	if finalTree == snapshotTree {
		return nil // no hook mutation → index already consistent with HEAD
	}
	// The mutated snapshot paths = every path that differs between the frozen snapshot and the
	// committed (post-hook) tree. DiffTreeNameStatus runs WITHOUT -M/-C, so 'M'/'D'/'T' lines are
	// exactly the snapshot paths the hook modified/deleted/typechanged (enforceSubset already
	// guaranteed no 'A' — a permitted mutation adds NO new path). SyncIndexPaths brings each of these
	// live-index entries to finalTree's blob.
	nameStatus, err := g.DiffTreeNameStatus(ctx, snapshotTree, finalTree)
	if err != nil {
		return fmt.Errorf("hooks: reconcile index: diff-tree-name-status: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(nameStatus, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		paths = append(paths, fields[len(fields)-1]) // the path (last tab-field)
	}
	if err := g.SyncIndexPaths(ctx, finalTree, paths); err != nil {
		return fmt.Errorf("hooks: reconcile index: %w", err)
	}
	return nil
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

// shouldSkipStagecoachPrepareCommitMsg implements FR-V4 recursion prevention: if the installed
// prepare-commit-msg is stagehand's OWN (detected by its Marker line via hook.Detect), skip it on the
// plumbing path — the message is already generated and invoking it would exec `stagehand hook exec`
// and recurse. A foreign hook (StatusForeign) RUNS and may annotate; absent (StatusNone) is a no-op.
// Pure (returns bool; the verbose log is in the caller, runPrepareCommitMsg). A Detect read error ⇒
// StatusNone ⇒ don't skip (conservative: run rather than recurse-stall on a rare read failure).
func shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool {
	status, _ := hook.Detect(hooksDir)
	return status == hook.StatusStagecoach
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

// stripCommentLines drops git message-file comment lines (lines beginning with commentChar, default
// '#') — git's default cleanup=strip, honoring core.commentChar. Used on the final read-back of the
// shared message file (after prepare-commit-msg + commit-msg have run on it).
func stripCommentLines(s, commentChar string) string {
	if commentChar == "" {
		commentChar = "#"
	}
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, commentChar) {
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
