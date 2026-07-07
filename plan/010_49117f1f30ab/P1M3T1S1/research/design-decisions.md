# Design Decisions — P1.M3.T1.S1 (commit-hooks runner core, §9.25 FR-V1–V8)

> The runner that drives the repo's commit hooks around the plumbing commit path. Companion to
> `external_deps.md` (git semantics) + `open_questions.md` (the 2 verify-items). Each § cites its evidence.
> Numbered for cross-reference from the PRP.

## §0 — Scope: S1 = the runner core; S2 = recursion + message-lifecycle; M3.T2 = wiring

- **S1 (this task)** = `RunCommitHooks` (pre-commit → prepare-commit-msg → commit-msg sequence, with
  --no-verify skip, absent-hook skip, timeout, env, failure→rescue, enforceSubset re-tree) + `RunPostCommit`
  (best-effort, post-update-ref). NEW file `internal/hooks/runner.go` (+ `runner_test.go`). The package
  `internal/hooks` already exists (subset.go from P1.M2.T2.S1).
- **S2 (P1.M3.T1.S2)** = recursion prevention (skip stagecoach's OWN prepare-commit-msg via `hook.Detect`) +
  message-file lifecycle refinements. S1 leaves a clean seam (§8) so S2's edit is surgical.
- **M3.T2** = wiring `RunCommitHooks` into CommitStaged + runPipeline (the callers). NOT this task.
- Do NOT wire into any caller here; do NOT implement `hook.Detect`-based recursion (S2); do NOT edit
  subset.go/enforceSubset (P1.M2.T2.S1 owns it).

## §1 — The sequence + --no-verify scope (git-commit(1) parity)

Order (external_deps.md §1): `pre-commit` → `prepare-commit-msg` → `commit-msg` (before commit-tree), then
`post-commit` (after update-ref, separate function). `--no-verify` (cfg.NoVerify) bypasses **ONLY**
`pre-commit` and `commit-msg` — it does NOT skip `prepare-commit-msg` or `post-commit` (external_deps.md §5,
git-commit(1) confirmed). So:
- pre-commit: `if cfg.NoVerify || opts.DryRun { skip }`.
- prepare-commit-msg: **always runs** (NoVerify does NOT gate it; DryRun does NOT gate it — FR-V8a).
- commit-msg: `if cfg.NoVerify { skip }`; **runs under DryRun** (FR-V8a — so the user sees lint results).

## §2 — Dry-run composition (FR-V8a)

`opts.DryRun`: pre-commit skipped (nothing committed → no index to validate); commit-msg RUNS (lint the
would-be message); prepare-commit-msg RUNS; post-commit skipped (no commit landed). So DryRun gates
pre-commit + post-commit ONLY.

## §3 — Hook exec via os/exec DIRECTLY (NOT runWithEnv)

`runWithEnv` (git.go:492) runs the **git binary** (`git -C repo <args>` with extra env) — it is NOT for
exec'ing a user hook script, and it is a `*gitRunner` method (NOT on the `Git` interface; the runner receives
`g git.Git`). So `internal/hooks` owns hook subprocess management via `os/exec` directly (§19 permits direct
`exec.Command`, no shell — the hook path is `[]string{hookPath, args...}`). Per-hook exec:
- `cmd := exec.CommandContext(ctx, hookPath, args...)` — ctx bounded by `context.WithTimeout(ctx, cfg.HookTimeout)`.
- `cmd.Env = append(os.Environ(), "GIT_INDEX_FILE=<abs tmpIndex>", "GIT_EDITOR=:", "GIT_DIR=<g.GitDir()>")`.
- `cmd.Dir = <worktree>` (§10 — the worktree accessor).
- `cmd.Stdin = /dev/null` (non-interactive; external_deps.md §3 stdin — most hooks don't read it; /dev/null is safe).
- `cmd.Stdout`/`cmd.Stderr` → the terminal (pass-through verbatim — a noisy hook is the user's hook; FR-V6).
- capture exit code; non-zero OR ctx-deadline → hook failure (§6).

## §4 — Absent / non-executable hook → silently skipped (git parity)

git checks `access(path, X_OK)` and silently skips a non-executable/absent hook (external_deps.md §4). The
runner mirrors: `info, err := os.Stat(hookPath); if err != nil || info.Mode()&0o100 == 0 { skip }` (not
executable ⇒ skip, no error). So an absent pre-commit is a no-op (finalTree stays snapshotTree).

## §5 — Pre-commit is scoped to the snapshot (FR-V3); the LIVE index is UNTOUCHED

The core stage-while-generating invariant: pre-commit runs against the snapshotted content, NOT the live
index. The scoped sequence (subset.go doc + external_deps.md §8):
1. `tmpIndex := <throwaway path>` (e.g. `filepath.Join(os.TempDir(), "stagecoach-hook-<rand>.idx")`).
2. `g.ReadTreeInto(ctx, snapshotTree, tmpIndex)` — primes the throwaway index from the snapshot (uses
   GIT_INDEX_FILE internally; does NOT touch `.git/index`).
3. run pre-commit with `GIT_INDEX_FILE=<abs tmpIndex>` (so the hook's `git add` writes to the THROWAWAY).
4. `postTree := g.WriteTreeFrom(ctx, tmpIndex)` — captures the (possibly mutated) tree from the throwaway.
5. `if postTree != snapshotTree { enforceSubset(ctx, g, snapshotTree, postTree); finalTree = postTree }` —
   re-tree on permitted mutation (M/D/T of existing paths); sweep (a new path) → ErrHookSweptConcurrentWork.
6. `os.Remove(tmpIndex)` (cleanup; best-effort).
The LIVE `.git/index` is byte-for-byte untouched — assert in tests (the contract's explicit invariant).

## §6 — Failure → *generate.RescueError (rescue state); sweep → ErrHookSweptConcurrentWork (non-rescue)

Two distinct abort kinds (both before commit-tree; HEAD + live index untouched):
- **Hook non-zero exit OR timeout** (pre-commit/prepare-commit-msg/commit-msg) → `*generate.RescueError{
  Kind: generate.ErrRescue, TreeSHA: snapshotTree, ParentSHA: parentSHA, Candidate: msg, Cause: <exitErr|
  ctx.Err()>}`. Byte-identical to a generation-failure rescue (FR-V7); the caller prints FR44. Timeout ⇒
  Cause = `context.DeadlineExceeded` (still ErrRescue kind; the rescue recipe is the same).
- **Pre-commit sweep** (enforceSubset) → `ErrHookSweptConcurrentWork` (subset.go; a NON-rescue hard error —
  content-axis freeze violation, twin of decompose.ErrFreezeViolation). The caller surfaces it as a freeze
  abort, NOT the rescue recipe.
RunCommitHooks returns `(finalTree, finalMsg, nil)` on success or `("", "", err)` on either abort kind.

## §7 — prepare-commit-msg args + message-file lifecycle

external_deps.md §1: `prepare-commit-msg <msgfile> [<source> [<sha>]]`. For a PLAIN commit the source is
ABSENT (likely 1 arg = file only), but PRD FR-V2 says pass `""` (2 args). open_questions.md §3: VERIFY with
a 5-second test (a prepare-commit-msg that logs `$#`; `git commit --allow-empty`; argc=1 vs 2); emit the
matching argv; the PRD's `""` is the conservative default — only deviate if the test definitively shows 1 arg.
**S1 action:** pass `<msgfile> ""` (PRD default); run the 5s verify-test at implementation; record the form;
if argc=1, drop the `""`. (This is a documented verify-item, not a blocker.) After running, read the file
back and strip `#`-comment lines (git's message-file convention) → finalMsg. Non-zero exit → *RescueError.
commit-msg: `<msgfile>` (1 arg); read back → finalMsg.

## §8 — S2 seam: prepare-commit-msg recursion check

S2 owns "skip stagecoach's OWN prepare-commit-msg via `hook.Detect`" (the existing `internal/hook.Detect`
returns `StatusStagecoach` if the marker is present). S1 leaves a clean seam: an unexported helper
`shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool` that S1 stubs to `return false` (with a
`// TODO(P1.M3.T1.S2): hook.Detect(hooksDir) == hook.StatusStagecoach` comment). S2 replaces the body. S1's
tests use FOREIGN prepare-commit-msg hooks (so the stub's `false` → run them); the recursion scenario is S2's
test. This keeps S1 self-contained and S2 surgical (one function body).

## §9 — RunPostCommit: best-effort, post-update-ref, exit code DISREGARDED

`RunPostCommit(ctx, g, cfg, opts)` — a SEPARATE function the caller invokes AFTER `update-ref` succeeds.
post-commit runs with 0 args, CWD=worktree, the same env MINUS GIT_INDEX_FILE (the commit is done; no scoped
index). Its exit code is DISREGARDED (the commit already landed; git itself disregards it) — log a warning at
`--verbose` on non-zero, NEVER undo, NEVER return an error that aborts anything. Absent/non-executable ⇒ skip.
Timeout ⇒ log warning (the commit stands). This is the single FR-V7 exception to "failure aborts".

## §10 — The worktree for cmd.Dir + GIT_DIR

- `GIT_DIR` ← `g.GitDir(ctx)` (on the Git interface; `git rev-parse --absolute-git-dir`).
- `cmd.Dir` (worktree) ← the runner needs the worktree. **Confirm** the `Git` interface exposes a worktree
  accessor (e.g. `TopLevel(ctx)` / `WorkTree(ctx)` = `git rev-parse --show-toplevel`); P1.M2 ("git primitives
  for scoped hook execution") may have added it. If absent, the cleanest minimal addition is a one-method
  `TopLevel(ctx) (string, error)` on Git (handles worktrees/linked repos correctly — deriving from GitDir's
  parent is WRONG for linked worktrees). The PRP flags this as the one interface detail to confirm/add.

## §11 — Tests (temp repo + real shell-script hooks; the contract's MOCKING spec)

White-box `package hooks`. A temp git repo + real executable shell-script hooks (write via `os.WriteFile` +
`chmod 0755`). Cases (the contract's list + the invariants):
- pre-commit that `git add`s a snapshot file (permitted mutation) → finalTree = postTree (re-treed), commit
  would include the hook's fix; LIVE `.git/index` UNTOUCHED (assert via `git status` before/after).
- pre-commit that exits non-zero → `*RescueError` (Kind=ErrRescue, TreeSHA=snapshotTree).
- pre-commit that `git add`s a NEW path not in snapshot → `ErrHookSweptConcurrentWork` (enforceSubset fires).
- commit-msg that appends to the message → finalMsg carries the annotation.
- absent pre-commit → silently skipped (finalTree = snapshotTree, no error).
- timeout: a pre-commit that sleeps > a tiny HookTimeout → `*RescueError` (Cause=DeadlineExceeded).
- --no-verify: pre-commit + commit-msg skipped; prepare-commit-msg still runs.
- dry-run: pre-commit + post-commit skipped; commit-msg runs.
- post-commit (RunPostCommit): non-zero exit → logged, no error returned, no undo.
The throwaway tmpIndex + the live-index-untouched assertion are the load-bearing safety checks.
