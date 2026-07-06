---
name: "P1.M3.T1.S1 — Commit-hooks runner core (internal/hooks.RunCommitHooks + RunPostCommit): the scoped pre-commit → prepare-commit-msg → commit-msg sequence + post-commit, with --no-verify parity, absent-hook skip, timeout, env, and failure→rescue — PRD §9.25 FR-V1–V8 (→ G22)"
description: |

  Land the runner core of "Hook execution on the commit path" (§9.25): a NEW `internal/hooks/runner.go`
  exporting `RunCommitHooks` (the pre-commit → prepare-commit-msg → commit-msg sequence run BETWEEN
  generation and commit-tree) and `RunPostCommit` (best-effort, after update-ref). It consumes the
  just-landed primitives — `git.HooksPath`/`GitDir`/`ReadTreeInto`/`WriteTreeFrom` (P1.M2.T1.S2),
  `enforceSubset` (P1.M2.T2.S1, same package), `cfg.NoVerify`/`cfg.HookTimeout` (P1.M1.T1.S1) — and is
  consumed by the wiring subtasks (P1.M3.T2) and refined by the sibling S2 (recursion prevention). The
  snapshot-based atomic-commit core is INTACT: pre-commit runs against a THROWAWAY scoped index (the live
  `.git/index` is never touched), and HEAD still moves via CAS update-ref.

  THE RUNNER (RunCommitHooks):
    RunCommitHooks(ctx, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string, opts HookOpts)
        (finalTree, finalMsg string, err error)
    HookOpts{ DryRun bool; Verbose *ui.Verbose }
    Sequence:
      hooksDir = g.HooksPath(ctx)
      (b) PRE-COMMIT   — skip if cfg.NoVerify || opts.DryRun. If <hooksDir>/pre-commit is executable:
            tmpIndex := throwaway path; g.ReadTreeInto(snapshotTree, tmpIndex)
            run pre-commit (0 args) with env {GIT_INDEX_FILE=<abs tmp>, GIT_EDITOR=:, GIT_DIR=<gitdir>}, CWD=worktree, stdin=/dev/null, bounded by cfg.HookTimeout
            postTree := g.WriteTreeFrom(tmpIndex); os.Remove(tmpIndex)
            if postTree != snapshotTree { enforceSubset(ctx,g,snapshotTree,postTree); finalTree = postTree }  // re-tree on permitted mutation
            non-zero/timeout → *generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA, Candidate:msg, Cause}
      (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun do NOT gate it). [SEAM §8: S2's hook.Detect skip]
            write msg → msgfile; run prepare-commit-msg <msgfile> "" (PRD FR-V2; verify argc via open_questions §3)
            read back, strip #-comment lines → finalMsg. non-zero → *RescueError.
      (d) COMMIT-MSG — skip if cfg.NoVerify; RUNS under DryRun (FR-V8a). run commit-msg <msgfile>; read back → finalMsg. non-zero/timeout → *RescueError.
      return (finalTree, finalMsg, nil)
    RunPostCommit(ctx, g, cfg, opts) — SEPARATE; caller runs it AFTER update-ref succeeds; 0 args; exit code
      DISREGARDED (log @verbose, never undo, never error).

  ⚠️ **#1 — hook exec via os/exec DIRECTLY, NOT runWithEnv.** `runWithEnv` (git.go:492) runs the GIT BINARY
       (`git -C repo …`) and is a `*gitRunner` method NOT on the `Git` interface; the runner receives
       `g git.Git`. Hooks are USER SCRIPTS, not git commands — `internal/hooks` owns their subprocess
       management via `exec.CommandContext(hookPath, args...)` (§19 permits direct exec, no shell). Env =
       `os.Environ() + {GIT_INDEX_FILE, GIT_EDITOR=:, GIT_DIR}`; Stdin=/dev/null; Stdout/Stderr pass-through
       (FR-V6); ctx bounded by cfg.HookTimeout. (research §3)

  ⚠️ **#2 — --no-verify bypasses ONLY pre-commit + commit-msg (git-commit(1) parity).** prepare-commit-msg
       and post-commit ALWAYS run (external_deps.md §5). DryRun (FR-V8a) skips pre-commit + post-commit but
       RUNS commit-msg (lint the would-be message) and prepare-commit-msg. (research §1/§2)

  ⚠️ **#3 — absent/non-executable hook → silently skip (git access(X_OK) parity).** `os.Stat` + mode check;
       not executable ⇒ no-op (finalTree stays snapshotTree), never an error. (research §4)

  ⚠️ **#4 — pre-commit is SCOPED to the snapshot; the LIVE .git/index is UNTOUCHED.** ReadTreeInto primes a
       THROWAWAY tmpIndex from snapshotTree; the hook's `git add` writes to that tmpIndex (GIT_INDEX_FILE);
       WriteTreeFrom captures postTree; os.Remove cleans up. `.git/index` is byte-for-byte unchanged — the
       stage-while-generating invariant (§5/FR-V3). ASSERT in tests. (research §5)

  ⚠️ **#5 — two abort kinds, both before commit-tree.** (a) hook non-zero/timeout → `*generate.RescueError`
       (byte-identical rescue state; caller prints FR44). (b) pre-commit sweep (enforceSubset) →
       `ErrHookSweptConcurrentWork` (NON-rescue freeze hard error). RunCommitHooks returns ("","",err) for
       either; the caller (M3.T2) branches. (research §6)

  ⚠️ **#6 — S2 seam for recursion prevention.** S1 stubs `shouldSkipStagehandPrepareCommitMsg(hooksDir)=false`;
       S2 (P1.M3.T1.S2) fills it with `hook.Detect(hooksDir)==StatusStagehand`. S1 tests use FOREIGN
       prepare-commit-msg hooks; the recursion scenario is S2's. (research §8)

  ⚠️ **#7 — prepare-commit-msg argv: pass `<msgfile> ""` (PRD FR-V2 default); VERIFY argc via open_questions
       §3's 5s test (a hook logging `$#`; plain `git commit --allow-empty`). If argc=1, drop the `""`.**
       (research §7)

  ⚠️ **#8 — RunPostCommit exit code is DISREGARDED.** It runs after update-ref; the commit already landed.
       Non-zero/timeout ⇒ log a warning at --verbose; NEVER undo; NEVER return an aborting error. (research §9)

  ⚠️ **#9 — worktree (cmd.Dir) + GIT_DIR.** GIT_DIR ← `g.GitDir(ctx)`. cmd.Dir (worktree) ← confirm the Git
       interface exposes a worktree accessor (TopLevel/WorkTree = `git rev-parse --show-toplevel`, likely from
       P1.M2); if absent, add the one-method `TopLevel(ctx)` (deriving from GitDir's parent is WRONG for linked
       worktrees). (research §10)

  Deliverable: NEW `internal/hooks/runner.go` (`HookOpts`, `RunCommitHooks`, `RunPostCommit`, the unexported
  hook-exec + skip + seam helpers) + NEW `internal/hooks/runner_test.go` (temp-repo + real shell-script hooks).
  NO wiring into callers (M3.T2). NO recursion prevention (S2). NO edit to subset.go. DOCS: none here
  (M3.T2.S1 / M4.T1.S1 own the feature docs).

---

## Goal

**Feature Goal**: Implement the commit-hooks runner core (§9.25 FR-V1–V8) that runs the repository's
`pre-commit` → `prepare-commit-msg` → `commit-msg` hooks between message generation and `commit-tree`, and
`post-commit` after publication — faithfully emulating `git commit`'s hook sequence while keeping the
snapshot-based atomic-commit core intact (scoped throwaway index for pre-commit; HEAD still via CAS
update-ref). Supports `--no-verify` (pre-commit+commit-msg bypass), `--dry-run` (FR-V8a), per-hook timeout,
absent-hook skip, and failure→rescue.

**Deliverable** (NEW files in the existing `internal/hooks` package):
1. **NEW `internal/hooks/runner.go`** — `type HookOpts struct { DryRun bool; Verbose *ui.Verbose }`;
   `func RunCommitHooks(ctx, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string, opts HookOpts) (finalTree, finalMsg string, err error)`;
   `func RunPostCommit(ctx, g git.Git, cfg config.Config, opts HookOpts) error`; + unexported helpers
   (`runHook` (the os/exec seam), `hookExecutable` (X_OK skip), `shouldSkipStagehandPrepareCommitMsg` (S2 stub),
   `writeMsgFile`/`readMsgFileStrippingComments`, `tmpIndexPath`).
2. **NEW `internal/hooks/runner_test.go`** — temp-repo + real shell-script hooks covering every branch
   (permitted mutation + live-index-untouched; non-zero→RescueError; sweep→ErrHookSweptConcurrentWork;
   commit-msg append; absent-hook skip; timeout; --no-verify; dry-run; post-commit disregarded).

**Success Definition**: `go build ./... && go vet ./... && go test ./internal/hooks/...` GREEN; `gofmt -l`
clean; the runner drives the 3-hook sequence with correct env/timeout/skip; pre-commit's throwaway index
leaves `.git/index` untouched (asserted); hook non-zero/timeout → `*generate.RescueError`; sweep →
`ErrHookSweptConcurrentWork`; absent/non-executable hook → silent skip; `--no-verify` skips pre-commit +
commit-msg only; dry-run skips pre-commit + post-commit; RunPostCommit disregards exit code; the S2 seam is
in place; no wiring into callers; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: The wiring subtasks P1.M3.T2 (CommitStaged + runPipeline) and P1.M3.T3 (decompose
publishCommit), which call `RunCommitHooks` after generation and before `commit-tree`, and `RunPostCommit`
after `update-ref`. Transitively: every user whose `pre-commit`/`commit-msg`/`post-commit` (husky, lint-staged,
conventional-commit lint, notifications) should fire on a `stagehand` commit (US19 / §5 caveat closure).

**Use Case**: A user with a `pre-commit` formatter and a `commit-msg` conventional-commit lint runs `stagehand`.
The runner executes pre-commit against the snapshot (the formatter's fixes are re-treed into the commit),
prepare-commit-msg + commit-msg over the generated message (the lint may reject → rescue), and post-commit
after the commit lands. `--no-verify` bypasses pre-commit + commit-msg for the one-off escape.

**User Journey**: (internal) caller → `RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, opts)` →
(finalTree, finalMsg, nil) → caller does `commit-tree -p parent finalTree finalMsg` + `update-ref` CAS →
caller calls `RunPostCommit(...)`. On any pre-commit/prepare/commit-msg failure → `*RescueError` (caller
prints FR44, exit 3); on sweep → `ErrHookSweptConcurrentWork` (freeze abort).

**Pain Points Addressed**: The v1–v2.3 plumbing path ran NO hooks — a user's pre-commit/commit-msg/post-commit
never fired on a stagehand commit. This runner closes that gap while preserving the snapshot/atomicity/stage-
while-generating core.

## Why

- **Closes the §5 caveat (hooks bypassed on the plumbing path).** The headline v2.4 feature (§9.25, G22):
  hooks now fire on every `stagehand`-produced commit, without forcing the user to hook mode (§9.20).
- **The runner is the reusable core; wiring is trivial.** All three commit chokepoints (single, dry-run,
  decompose) call the same `RunCommitHooks`/`RunPostCommit` — the runner centralizes the git-hook semantics
  (order, env, --no-verify, timeout, rescue) so M3.T2/M3.T3 wiring is a few lines each.
- **Faithful to git + the snapshot core.** Pre-commit is scoped to the snapshot (FR-V3 freeze holds); HEAD
  still moves via CAS (the rescue + stage-while-generating guarantees survive). `--no-verify` mirrors
  `git commit --no-verify` exactly.

## What

A new `runner.go` (the sequence + exec + skip + rescue + post-commit) and a new `runner_test.go` (temp-repo
shell-script-hook integration tests). No caller wiring, no recursion prevention (S2), no subset.go edit, no
config/CLI/docs surface (those landed in P1.M1; the feature docs land in M3.T2.S1/M4.T1.S1).

### Success Criteria

- [ ] `HookOpts{DryRun bool; Verbose *ui.Verbose}`; `RunCommitHooks(...)` + `RunPostCommit(...)` exported.
- [ ] Sequence: pre-commit (skip if NoVerify||DryRun) → prepare-commit-msg (always) → commit-msg (skip if
      NoVerify; runs under DryRun). post-commit via the separate RunPostCommit.
- [ ] Hook exec: `exec.CommandContext` (ctx←cfg.HookTimeout); Env=GIT_INDEX_FILE+GIT_EDITOR=:GIT_DIR;
      Dir=worktree; Stdin=/dev/null; Stdout/Stderr pass-through. NOT runWithEnv.
- [ ] Absent/non-executable hook → silent skip (os.Stat + X_OK).
- [ ] Pre-commit scoped: ReadTreeInto(snapshot,tmp) → run → WriteTreeFrom(tmp)=postTree → enforceSubset →
      re-tree; LIVE `.git/index` untouched (asserted); tmpIndex cleaned up.
- [ ] Hook non-zero/timeout → `*generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA,
      Candidate:msg, Cause}`. Sweep → `ErrHookSweptConcurrentWork`.
- [ ] prepare-commit-msg: write msgfile; run `<msgfile> ""` (verify argc); read back, strip #-comments.
      [S2 seam `shouldSkipStagehandPrepareCommitMsg`=false in place.]
- [ ] RunPostCommit: 0 args; exit code disregarded (log @verbose; no undo; no aborting error).
- [ ] `go build ./... && go vet ./... && go test ./internal/hooks/...` GREEN; `gofmt -l` clean; go.mod/go.sum
      byte-unchanged; no wiring into callers; no edit to subset.go.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact dependency signatures
(below), the sequence + env + error mapping (research §1/§3/§5/§6), the git hook semantics (external_deps.md),
the code skeleton (Blueprint), and the test matrix (research §11). No decompose/generate-internals knowledge
required beyond the RescueError type.

### Documentation & References

```yaml
# MUST READ — the design calls (sequence, exec, env, error mapping, S2 seam, tests)
- docfile: plan/010_49117f1f30ab/P1M3T1S1/research/design-decisions.md
  why: §0 (scope: S1 vs S2 vs M3.T2), §1 (--no-verify scope), §2 (dry-run), §3 (os/exec not runWithEnv),
       §4 (absent-hook skip), §5 (scoped pre-commit + untouched live index), §6 (RescueError vs ErrHookSwept),
       §7 (prepare-commit-msg args + verify), §8 (S2 seam), §9 (RunPostCommit disregarded), §10 (worktree/GIT_DIR),
       §11 (test matrix).
  critical: §3 (os/exec, NOT runWithEnv), §5 (throwaway index; live .git/index untouched), §6 (two abort kinds),
       §1/§2 (--no-verify + dry-run skip tables) are the things most likely to be implemented wrong.

# MUST READ — the git hook semantics (order, args, env, --no-verify, X_OK) — the authoritative external reference
- docfile: plan/010_49117f1f30ab/architecture/external_deps.md
  section: §1 (ordering + invocation args), §2 (prepare-commit-msg source ABSENT vs "" — the verify item),
       §3 (env: GIT_INDEX_FILE/GIT_EDITOR/GIT_DIR/stdin), §4 (discovery + X_OK skip), §5 (--no-verify scope).
  critical: §5 (--no-verify skips ONLY pre-commit + commit-msg) and §3 (the exact env) are the git-parity load-
       bearers. §2/§7 → run the 5s argc verify-test.

# The verify-items (prepare-commit-msg argv; dry-run composition)
- docfile: plan/010_49117f1f30ab/architecture/open_questions.md
  section: §3 (prepare-commit-msg "" vs absent — verify argc) + §4 (dry-run hook composition FR-V8a).
  critical: §3 — pass `""` per PRD default; verify argc; deviate only if the test shows 1 arg.

# The contract inputs — the dependency signatures (READ each before coding)
- file: internal/hooks/subset.go   (P1.M2.T2.S1 — SAME PACKAGE)
  section: `func enforceSubset(ctx, g git.Git, snapshotTree, postTree string) error` (UNEXPORTED; same package ⇒
           call directly); `ErrHookSweptConcurrentWork`; the CALLER's re-tree one-liner (in its doc comment).
  why: the FR-V3 freeze backstop. nil ⇒ permitted (re-tree); ErrHookSweptConcurrentWork ⇒ sweep (non-rescue abort).
  critical: do NOT edit subset.go; do NOT re-implement the subset check — CALL enforceSubset.

- file: internal/git/git.go   (the Git interface methods the runner consumes)
  section: `HooksPath(ctx) (string, error)` (L1859); `GitDir(ctx) (string, error)` (the GIT_DIR source);
           `ReadTreeInto(ctx, tree, indexFile) error` (L1324); `WriteTreeFrom(ctx, indexFile) (sha, error)` (L576);
           `runWithEnv` (L492 — *gitRunner-only, NOT for hook exec; do NOT call).
  why: the scoped-index + hooksDir + gitdir primitives. All on the `Git` interface except runWithEnv.
  critical: hook exec is os/exec DIRECT (runWithEnv runs the git binary; not on the interface). Confirm the
           worktree accessor (§10); if absent, add TopLevel(ctx).

- file: internal/generate/generate.go   (the RescueError type)
  section: `type RescueError{Kind error; TreeSHA, ParentSHA, Candidate string; Cause error}` (L84); `ErrRescue`
           (L67); `Unwrap()` returns Kind.
  why: the failure→rescue return. Map hook non-zero/timeout → *RescueError{Kind:ErrRescue, TreeSHA:snapshotTree,
       ParentSHA, Candidate:msg, Cause:<exitErr|ctx.Err()>}.
  critical: RescueError is how the caller (M3.T2) prints FR44 + exit 3 — byte-identical to a generation failure.

- file: internal/config/config.go   (P1.M1.T1.S1 — the config fields)
  section: `NoVerify bool` (L134, default false); `HookTimeout time.Duration` (L138, default 10m).
  why: the --no-verify bypass + the per-hook timeout (context.WithTimeout(ctx, cfg.HookTimeout)).

- file: internal/ui/verbose.go   (the Verbose type)
  section: `type Verbose struct`; `VerboseWarn(msg)`, `VerboseCommand`, etc.
  why: HookOpts carries *ui.Verbose; log hook progress/warnings (nil-safe — Verbose may be nil).

- file: internal/hook/hook.go   (the EXISTING hook-mode package — for S2's seam, referenced here)
  section: `Detect(hooksDir) (Status, error)`; `StatusStagehand`/`StatusForeign`/`StatusNone`; `Marker`.
  why: S2 (P1.M3.T1.S2) fills `shouldSkipStagehandPrepareCommitMsg` with `hook.Detect(hooksDir)==StatusStagehand`.
       S1 STUBS it (false); S2 implements. (NOTE: `internal/hook` singular ≠ `internal/hooks` plural — this package.)

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md (or plan/010_…/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" FR-V1–V8 (the authoritative spec: scope/order, emulation,
           snapshot-scoped pre-commit, prepare-compose, --no-verify, env+timeout, failure→rescue, composition).
  critical: FR-V3 (scoped pre-commit, subset enforcement), FR-V5 (--no-verify scope), FR-V6 (env+timeout),
       FR-V7 (failure→rescue; post-commit exception), FR-V8a (dry-run: commit-msg runs).

# The parallel PRP (scope check — P1.M2.T2.S1 owns subset.go; this task owns runner.go)
- file: plan/010_…/P1M2T2S1/PRP.md
  why: confirms enforceSubset's signature + that subset.go is P1.M2.T2.S1's deliverable (do NOT duplicate/edit).
       The runner CONSUMES enforceSubset; it does not re-implement the subset check.
```

### Current Codebase tree (relevant slice)

```bash
internal/hooks/
  subset.go          # P1.M2.T2.S1 — enforceSubset + ErrHookSweptConcurrentWork (UNCHANGE — consume)
  subset_test.go     # P1.M2.T2.S1 — UNCHANGED
  runner.go          # NEW (this subtask) ← HookOpts + RunCommitHooks + RunPostCommit + helpers
  runner_test.go     # NEW (this subtask) ← temp-repo shell-script-hook integration tests
internal/hook/       # the EXISTING hook-mode package (Detect/Marker/Status — S2 consumes; S1 references only)
internal/git/git.go  # HooksPath/GitDir/ReadTreeInto/WriteTreeFrom on Git interface — UNCHANGED (consumed)
internal/generate/   # RescueError/ErrRescue — UNCHANGED (consumed)
internal/config/     # NoVerify/HookTimeout — UNCHANGED (consumed)
go.mod / go.sum      # UNCHANGED (stdlib os/exec/os/context/path/fmt/strings + internal deps; no new external dep)
```

### Desired Codebase tree with files to be added

```bash
internal/hooks/runner.go          # NEW — the runner core (RunCommitHooks + RunPostCommit + HookOpts + helpers)
internal/hooks/runner_test.go     # NEW — temp-repo + real shell-script hooks (every branch)
# NO other file added. NO wiring (M3.T2). NO subset.go edit. NO recursion (S2).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — os/exec DIRECTLY, not runWithEnv): runWithEnv (git.go:492) runs the git binary and is a
// *gitRunner method NOT on the Git interface. Hooks are user scripts — exec them via exec.CommandContext
// (§19: direct exec, no shell). Env = os.Environ() + GIT_INDEX_FILE + GIT_EDITOR=: + GIT_DIR. (research §3)

// CRITICAL (#2 — --no-verify skips ONLY pre-commit + commit-msg): prepare-commit-msg + post-commit ALWAYS run.
// DryRun (FR-V8a) skips pre-commit + post-commit; commit-msg + prepare-commit-msg RUN under dry-run. (research §1/§2)

// CRITICAL (#3 — scoped pre-commit; LIVE .git/index UNTOUCHED): ReadTreeInto(snapshot, tmpIndex) primes a
// THROWAWAY index; the hook's git add writes to tmpIndex (GIT_INDEX_FILE); WriteTreeFrom captures postTree;
// os.Remove(tmpIndex). .git/index is byte-for-byte unchanged — ASSERT in tests. (research §5)

// CRITICAL (#4 — two abort kinds): hook non-zero/timeout → *generate.RescueError (rescue state, FR44); sweep →
// ErrHookSweptConcurrentWork (non-rescue freeze abort). Both before commit-tree; both leave HEAD+live index
// untouched. RunCommitHooks returns ("","",err) for either. (research §6)

// CRITICAL (#5 — enforceSubset is SAME PACKAGE, unexported): call enforceSubset(ctx,g,snapshot,postTree)
// directly (NOT g.EnforceSubset). nil ⇒ re-tree; ErrHookSweptConcurrentWork ⇒ propagate. Do NOT re-implement.

// GOTCHA (absent/non-executable hook → silent skip): os.Stat + mode&0o100==0 ⇒ skip (git access(X_OK) parity).
// GOTCHA (timeout via context): ctx, cancel := context.WithTimeout(ctx, cfg.HookTimeout); defer cancel(); pass
// ctx to exec.CommandContext. A timeout ⇒ the cmd fails with ctx.Err()==DeadlineLineExceeded ⇒ *RescueError.
// GOTCHA (stdin /dev/null): cmd.Stdin = os.Stdin would block on a TTY; use strings.NewReader("") or os.DevNull.
// GOTCHA (stdout/stderr pass-through): cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr (a noisy hook is the
// user's hook; FR-V6). (For tests, capture to a buffer to assert.)
// GOTCHA (prepare-commit-msg "" vs absent): pass `<msgfile> ""` (PRD default); VERIFY argc (open_questions §3).
// GOTCHA (strip #-comments on read-back): git message files carry commented metadata; drop lines starting with "#".
// GOTCHA (S2 seam): shouldSkipStagehandPrepareCommitMsg(hooksDir) stubs false; S2 fills with hook.Detect.
// GOTCHA (RunPostCommit exit disregarded): NEVER return an aborting error; log @verbose; never undo.
// GOTCHA (Verbose may be nil): guard `if opts.Verbose != nil { opts.Verbose.VerboseWarn(...) }`.
// GOTCHA (worktree cmd.Dir): GIT_DIR ← g.GitDir(ctx); worktree ← confirm/add a Git TopLevel(ctx) accessor
// (deriving from GitDir's parent is WRONG for linked worktrees). (research §10)
// GOTCHA (no new external deps): os/exec + os + context + path/filepath + fmt + strings + the internal deps.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/hooks/runner.go
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

// HookOpts carries the runner's per-call options. DryRun (FR-V8a) gates pre-commit + post-commit (skip);
// commit-msg + prepare-commit-msg still run. Verbose is nil-safe (nil ⇒ silent).
type HookOpts struct {
	DryRun  bool
	Verbose *ui.Verbose
}

// RunCommitHooks runs the repo's pre-commit → prepare-commit-msg → commit-msg hooks (in git's documented
// order) BETWEEN generation and commit-tree, scoped to the snapshot tree (FR-V3). It returns the
// (possibly re-treed) finalTree and the (possibly hook-annotated) finalMsg for commit-tree. A hook non-zero
// exit or timeout returns *generate.RescueError (the identical rescue state as a generation failure, FR-V7);
// a pre-commit that sweeps a path not in the snapshot returns ErrHookSweptConcurrentWork (a non-rescue freeze
// abort). HEAD and the live .git/index are byte-for-byte unchanged on any error (pre-commit runs against a
// THROWAWAY index). post-commit is SEPARATE (RunPostCommit, after update-ref). (PRD §9.25 FR-V1–V8.)
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
	workTree, err := g.TopLevel(ctx) // §10 — confirm/add TopLevel on Git (`git rev-parse --show-toplevel`)
	if err != nil {
		return "", "", fmt.Errorf("hooks: resolve worktree: %w", err)
	}

	// (b) PRE-COMMIT — skip if --no-verify (FR-V5) or --dry-run (FR-V8a).
	if !(cfg.NoVerify || opts.DryRun) {
		finalTree, err = runPreCommitScoped(ctx, g, cfg, opts, hooksDir, gitDir, workTree, snapshotTree)
		if err != nil {
			return "", "", err // *RescueError (non-zero/timeout) or ErrHookSweptConcurrentWork (sweep)
		}
	}

	// (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun do NOT gate it; FR-V1/FR-V8a).
	finalMsg, err = runPrepareCommitMsg(ctx, g, cfg, opts, hooksDir, gitDir, workTree, finalMsg)
	if err != nil {
		return "", "", err // *RescueError
	}

	// (d) COMMIT-MSG — skip if --no-verify (FR-V5); RUNS under --dry-run (FR-V8a: lint the would-be message).
	if !cfg.NoVerify {
		finalMsg, err = runCommitMsg(ctx, g, cfg, opts, hooksDir, gitDir, workTree, finalMsg)
		if err != nil {
			return "", "", err // *RescueError
		}
	}

	return finalTree, finalMsg, nil
}

// runPreCommitScoped runs pre-commit against a THROWAWAY index primed from snapshotTree (FR-V3: the live
// .git/index is never touched). It returns the post-hook tree (re-treed if the hook mutated snapshot paths),
// or *RescueError (non-zero/timeout), or ErrHookSweptConcurrentWork (the hook staged a new path).
func runPreCommitScoped(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, snapshotTree string) (string, error) {
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if !hookExecutable(hookPath) {
		return snapshotTree, nil // absent/non-executable → silent skip (git access(X_OK) parity)
	}
	tmpIndex, err := tmpIndexPath() // throwaway path (os.TempDir + rand)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpIndex) // best-effort cleanup

	if err := g.ReadTreeInto(ctx, snapshotTree, tmpIndex); err != nil {
		return "", fmt.Errorf("hooks: prime scoped index: %w", err)
	}
	// run pre-commit (0 args) with GIT_INDEX_FILE=<abs tmpIndex>, GIT_EDITOR=:, GIT_DIR=<gitDir>.
	exitErr := runHook(ctx, cfg.HookTimeout, hookPath, nil, gitDir, workTree,
		map[string]string{"GIT_INDEX_FILE": tmpIndex}, opts)
	if exitErr != nil {
		return "", rescueErr(cfg, snapshotTree, "", "pre-commit", exitErr) // Candidate filled by caller? msg passed in
	}
	postTree, err := g.WriteTreeFrom(ctx, tmpIndex)
	if err != nil {
		return "", fmt.Errorf("hooks: capture post-pre-commit tree: %w", err)
	}
	if postTree == snapshotTree {
		return snapshotTree, nil // no mutation
	}
	// re-tree on permitted mutation; enforceSubset returns ErrHookSweptConcurrentWork on a new path.
	if err := enforceSubset(ctx, g, snapshotTree, postTree); err != nil {
		return "", err // ErrHookSweptConcurrentWork (non-rescue) or wrapped git error
	}
	return postTree, nil // permitted mutation → re-tree (git-commit parity)
}

// runPrepareCommitMsg writes msg to a temp file, runs prepare-commit-msg <msgfile> "" (PRD FR-V2; verify
// argc via open_questions §3), reads back stripped of #-comments. ALWAYS runs (NoVerify/DryRun don't gate it).
// [SEAM] shouldSkipStagehandPrepareCommitMsg stubs false here; S2 (P1.M3.T1.S2) fills it via hook.Detect.
func runPrepareCommitMsg(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, msg string) (string, error) {
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if !hookExecutable(hookPath) || shouldSkipStagehandPrepareCommitMsg(hooksDir) { // §8 seam — S2 fills
		return msg, nil // absent/non-exec OR stagehand's own hook (recursion) → skip
	}
	finalMsg, err := runMsgHook(ctx, cfg, opts, hookPath, gitDir, workTree, msg,
		[]string{""}) // PRD FR-V2: <msgfile> "" (verify argc)
	if err != nil {
		return "", err
	}
	return stripCommentLines(finalMsg), nil
}

// runCommitMsg runs commit-msg <msgfile>, reads back. Skipped only by --no-verify (NOT dry-run).
func runCommitMsg(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts,
	hooksDir, gitDir, workTree, msg string) (string, error) {
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if !hookExecutable(hookPath) {
		return msg, nil
	}
	return runMsgHook(ctx, cfg, opts, hookPath, gitDir, workTree, msg, nil) // commit-msg: 1 arg <msgfile>
}

// runMsgHook writes msg to a temp file, runs <hook> <msgfile> [extra...], reads back. Non-zero/timeout → *RescueError.
func runMsgHook(ctx context.Context, cfg config.Config, opts HookOpts, hookPath, gitDir, workTree, msg string,
	extraArgs []string) (string, error) {
	tmpMsg, err := os.CreateTemp("", "stagehand-msg-*.txt")
	if err != nil {
		return "", err
	}
	tmpMsgPath := tmpMsg.Name()
	defer os.Remove(tmpMsgPath)
	if _, werr := tmpMsg.WriteString(msg); werr != nil {
		tmpMsg.Close()
		return "", werr
	}
	tmpMsg.Close()
	args := append([]string{tmpMsgPath}, extraArgs...)
	if exitErr := runHook(ctx, cfg.HookTimeout, hookPath, args, gitDir, workTree, nil, opts); exitErr != nil {
		return "", rescueErr(cfg, "", "", filepath.Base(hookPath), exitErr) // TreeSHA/ParentSHA filled by caller
	}
	data, rerr := os.ReadFile(tmpMsgPath)
	if rerr != nil {
		return "", rerr
	}
	return string(data), nil
}

// RunPostCommit runs post-commit AFTER update-ref succeeds (best-effort). 0 args; exit code DISREGARDED
// (the commit already landed; git disregards it). Non-zero/timeout ⇒ log @verbose; NEVER undo; NEVER abort.
// (PRD §9.25 FR-V7 exception.)
func RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, opts HookOpts) error {
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
		if opts.Verbose != nil {
			opts.Verbose.VerboseWarn(fmt.Sprintf("post-commit hook exited non-zero (commit stands): %v", exitErr))
		}
	}
	return nil // ALWAYS nil — exit code disregarded
}

// ---- unexported helpers ----

// runHook execs a hook script directly (NOT via git runWithEnv — hooks are user scripts; §19: direct exec,
// no shell). Env = os.Environ() + GIT_EDITOR=: + GIT_DIR + extraEnv (e.g. GIT_INDEX_FILE). CWD=workTree.
// stdin=/dev/null; stdout/stderr pass through (FR-V6). Returns nil on exit 0, or the causing error
// (*exec.ExitError / ctx.Err()) on non-zero/timeout — the caller maps it to *RescueError.
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
	if opts.Verbose != nil {
		opts.Verbose.VerboseCommand(hookPath + " " + strings.Join(args, " "))
	}
	if err := cmd.Run(); err != nil {
		if cerr := hctx.Err(); cerr == context.DeadlineExceeded {
			return cerr // timeout
		}
		return err // *exec.ExitError (non-zero) or other
	}
	return nil
}

// hookExecutable reports whether path exists and is executable (git's access(X_OK) parity). Absent/non-exec ⇒ skip.
func hookExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o100 != 0 // owner-executable bit
}

// shouldSkipStagehandPrepareCommitMsg is the S2 SEAM: skip stagehand's OWN prepare-commit-msg hook (recursion).
// S1 stubs false (foreign hooks run). S2 (P1.M3.T1.S2) fills: hook.Detect(hooksDir) == hook.StatusStagehand.
func shouldSkipStagehandPrepareCommitMsg(hooksDir string) bool {
	// TODO(P1.M3.T1.S2): return hook.Detect(hooksDir) == hook.StatusStagehand
	return false
}

// rescueErr maps a hook failure (non-zero/timeout) to *generate.RescueError — byte-identical to a generation
// failure (FR-V7). The caller fills TreeSHA/ParentSHA/Candidate for the pre-commit case (runPreCommitScoped
// passes snapshotTree); for *-commit-msg hooks, RunCommitHooks wraps with the snapshot context.
func rescueErr(cfg config.Config, treeSHA, parentSHA, hookName string, cause error) error {
	return &generate.RescueError{
		Kind:      generate.ErrRescue,
		TreeSHA:   treeSHA,
		ParentSHA: parentSHA,
		Candidate: "",
		Cause:     fmt.Errorf("hook %s failed: %w", hookName, cause),
	}
}

// stripCommentLines drops git message-file comment lines (lines beginning with "#").
func stripCommentLines(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func tmpIndexPath() (string, error) {
	f, err := os.CreateTemp("", "stagehand-hook-*.idx")
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}
```

> NOTE: the skeleton above shows the structure and the load-bearing logic. Two refinements the implementer
> resolves while coding: (1) `rescueErr` should carry the FULL `*RescueError{TreeSHA:snapshotTree,
> ParentSHA, Candidate:msg, Cause}` — thread `snapshotTree`/`parentSHA`/`msg` into the msg-hook helpers
> (the skeleton's `runMsgHook`/`rescueErr` leave TreeSHA/Candidate empty for brevity; fill them so the rescue
> state is byte-identical to a generation failure — FR-V7). (2) confirm/add `g.TopLevel(ctx)` (§10). Both are
> local, mechanical completions, not design changes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CONFIRM prerequisites (dependency signatures + the worktree accessor)
  - READ internal/hooks/subset.go (enforceSubset signature + ErrHookSweptConcurrentWork — same package, call directly).
  - CONFIRM g.GitDir/g.HooksPath/g.ReadTreeInto/g.WriteTreeFrom on the Git interface (they are).
  - CONFIRM the Git interface has a worktree accessor (TopLevel/WorkTree). IF NOT, add `TopLevel(ctx) (string,
      error)` = `git rev-parse --show-toplevel` to the Git interface + gitRunner (one-method extension; P1.M2's
      theme). Do NOT derive worktree from GitDir's parent (wrong for linked worktrees).
  - RUN the prepare-commit-msg argc verify-test (open_questions §3): a hook logging `$#`; `git commit
      --allow-empty`; observe argc=1 vs 2. Pass `<msgfile> ""` (PRD default) unless the test shows argc=1.

Task 2: CREATE internal/hooks/runner.go (HookOpts + RunCommitHooks + RunPostCommit + helpers)
  - IMPLEMENT per the Blueprint. Hook exec via os/exec DIRECTLY (runHook); env {GIT_INDEX_FILE?, GIT_EDITOR=:,
      GIT_DIR}; CWD=worktree; stdin=/dev/null; stdout/stderr pass-through; ctx←cfg.HookTimeout.
  - SEQUENCE: pre-commit (skip if NoVerify||DryRun; scoped tmpIndex + enforceSubset + re-tree) → prepare-commit-msg
      (always; [S2 seam]) → commit-msg (skip if NoVerify; runs under DryRun).
  - ERROR MAPPING: hook non-zero/timeout → *generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA,
      Candidate:msg, Cause}; sweep → propagate ErrHookSweptConcurrentWork.
  - RunPostCommit: exit code DISREGARDED (log @verbose; never undo; never abort).
  - S2 SEAM: shouldSkipStagehandPrepareCommitMsg=false stub + TODO comment.
  - GOTCHA: thread snapshotTree/parentSHA/msg into rescueErr so the rescue state is byte-identical (FR-V7).

Task 3: CREATE internal/hooks/runner_test.go (temp repo + real shell-script hooks)
  - WHITE-BOX package hooks. Temp git repo; write executable shell-script hooks (os.WriteFile + chmod 0755).
  - CASES (research §11): (1) pre-commit git-adds a snapshot file → re-treed; LIVE .git/index UNTOCHANGED
      (assert git status before/after); (2) pre-commit exits non-zero → *RescueError; (3) pre-commit adds a NEW
      path → ErrHookSweptConcurrentWork; (4) commit-msg appends → finalMsg annotated; (5) absent pre-commit →
      skip (finalTree=snapshotTree); (6) timeout (sleep > tiny HookTimeout) → *RescueError (DeadlineExceeded);
      (7) --no-verify → pre-commit+commit-msg skip, prepare-commit-msg runs; (8) dry-run → pre-commit+post-commit
      skip, commit-msg runs; (9) RunPostCommit non-zero → logged, nil returned, no undo.
  - The LIVE-index-untouched assertion (case 1) + the throwaway-tmpIndex cleanup are the load-bearing safety checks.

Task 4: VERIFY
  - RUN: gofmt -w; go vet ./internal/hooks/; go build ./...; go test ./internal/hooks/...; go test ./...
  - CONFIRM: no wiring into callers; no subset.go edit; no recursion (S2 stub); go.mod/go.sum byte-unchanged.
```

### Implementation Patterns & Key Details

```go
// THE hook exec (os/exec DIRECT — NOT runWithEnv; hooks are user scripts, §19):
cmd := exec.CommandContext(hctx, hookPath, args...)     // []string, no shell
cmd.Dir = workTree                                       // g.TopLevel(ctx)
cmd.Env = append(os.Environ(), "GIT_EDITOR=:", "GIT_DIR="+gitDir, "GIT_INDEX_FILE="+tmpIndex) // pre-commit only
cmd.Stdin = strings.NewReader("")                        // /dev/null
cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr            // pass-through (FR-V6)

// THE scoped pre-commit (LIVE .git/index UNTOUCHED — FR-V3):
g.ReadTreeInto(ctx, snapshotTree, tmpIndex)              // prime throwaway
runHook(... GIT_INDEX_FILE=tmpIndex ...)                 // hook writes to throwaway
postTree := g.WriteTreeFrom(ctx, tmpIndex)               // capture
if postTree != snapshotTree { enforceSubset(...); finalTree = postTree }  // re-tree on permitted mutation

// THE --no-verify + dry-run skip table:
//   pre-commit:        skip if NoVerify || DryRun
//   prepare-commit-msg: NEVER skip (NoVerify/DryRun don't gate)
//   commit-msg:        skip if NoVerify  (RUNS under DryRun — FR-V8a)
//   post-commit:       skip if DryRun    (NoVerify doesn't gate; separate RunPostCommit)

// THE error mapping (two abort kinds, both before commit-tree):
//   hook non-zero/timeout → *generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA, Candidate:msg, Cause}
//   pre-commit sweep      → ErrHookSweptConcurrentWork (propagate from enforceSubset; non-rescue freeze abort)
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. runner.go uses stdlib (os/exec, os, context, path/filepath, fmt,
      strings, time) + internal deps (config, generate, git, ui). No new external dep.

PACKAGE EDGES: internal/hooks → {internal/git (Git interface), internal/config, internal/generate (RescueError),
      internal/ui (Verbose)}. It does NOT import internal/hook (S2 does, for hook.Detect). It CALLS enforceSubset
      (same package). If Task 1 adds TopLevel to the Git interface, that's an internal/git addition (one method).

UPSTREAM (the inputs — consume, do NOT edit):
  - git.Git: HooksPath/GitDir/ReadTreeInto/WriteTreeFrom (+ TopLevel per §10).
  - internal/hooks/subset.go: enforceSubset + ErrHookSweptConcurrentWork (P1.M2.T2.S1).
  - config.Config: NoVerify/HookTimeout (P1.M1.T1.S1).
  - generate.RescueError/ErrRescue (the failure type).

DOWNSTREAM (the consumers — NOT this task):
  - P1.M3.T2.S1 (CommitStaged wiring): calls RunCommitHooks after generation, before commit-tree; RunPostCommit
       after update-ref.
  - P1.M3.T2.S2 (runPipeline dry-run wiring): FR-V8a (commit-msg only).
  - P1.M3.T3 (decompose publishCommit wiring): per-concept scoped.
  - P1.M3.T1.S2 (sibling): fills shouldSkipStagehandPrepareCommitMsg (hook.Detect) + message-lifecycle refinements.

FROZEN/LEAVE (do NOT edit):
  - internal/hooks/subset.go (+ its test) — P1.M2.T2.S1.
  - internal/git/* (the Git interface + primitives) — except possibly adding TopLevel (§10, Task 1).
  - internal/generate/*, internal/config/*, internal/hook/*, internal/cmd/*, pkg/stagehand/*.
  - PRD.md, go.mod, Makefile. NO wiring into any caller.

NO NEW DATABASE / ROUTES / CLI / CONFIG / DOCS (here).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/hooks/runner.go internal/hooks/runner_test.go
go vet ./internal/hooks/
go build ./...     # incl. the optional TopLevel addition if Task 1 added it
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go build clean; go.mod/go.sum byte-unchanged.
```

### Level 2: Runner unit/integration tests (temp repo + shell-script hooks)

```bash
go test ./internal/hooks/... -v
# Expected PASS — verify every branch:
#   pre-commit permitted-mutation → re-treed; LIVE .git/index UNTOUCHED (the FR-V3 assertion)
#   pre-commit non-zero → *RescueError (Kind=ErrRescue, TreeSHA=snapshotTree)
#   pre-commit sweep (new path) → ErrHookSweptConcurrentWork (errors.Is)
#   commit-msg appends → finalMsg annotated
#   absent pre-commit → skip (finalTree=snapshotTree, no error)
#   timeout → *RescueError (Cause≈DeadlineExceeded)
#   --no-verify → pre-commit+commit-msg skip; prepare-commit-msg runs
#   dry-run → pre-commit+post-commit skip; commit-msg runs
#   RunPostCommit non-zero → nil returned, warning logged, no undo
# Plus the existing subset_test.go stays green (runner.go didn't touch subset.go).
```

### Level 3: Whole-repo + frozen-file check

```bash
go test ./...     # Expect all PASS (no caller wired yet — the runner is standalone; no regression).
git diff --name-only   # Expect ONLY internal/hooks/runner.go + runner_test.go (+ optional git.go TopLevel).
git diff --exit-code internal/hooks/subset.go internal/hooks/subset_test.go && echo "subset.go UNCHANGED (expected)"
git diff --exit-code internal/generate internal/config internal/hook internal/cmd pkg internal/ui && echo "frozen pkgs UNCHANGED"
# Confirm NO wiring into callers (M3.T2 owns it):
! grep -rq "hooks.RunCommitHooks\|hooks.RunPostCommit" internal/generate internal/cmd pkg && echo "no caller wiring (good)"
```

### Level 4: Git-parity + invariant reasoning

```bash
# The runner's correctness rests on git-parity (external_deps.md) + the FR-V3 freeze. Verify by reasoning + tests:
#   1. ORDER: pre-commit → prepare-commit-msg → commit-msg (before commit-tree); post-commit (after, separate). (FR-V1)
#   2. --no-verify scope: skips ONLY pre-commit + commit-msg (prepare-commit-msg + post-commit run). (FR-V5, external_deps §5)
#   3. ENV: GIT_INDEX_FILE (pre-commit scoped), GIT_EDITOR=:, GIT_DIR; CWD=worktree; stdin=/dev/null. (FR-V6)
#   4. FREEZE: pre-commit runs against the throwaway index; .git/index byte-unchanged (asserted). (FR-V3)
#   5. RESCUE: hook non-zero/timeout → *RescueError (byte-identical to generation failure); sweep → ErrHookSwept. (FR-V7)
#   6. POST-COMMIT: exit disregarded (commit stands). (FR-V7 exception)
# (No Level-4 commands beyond Levels 1–3 — the tests + the external_deps parity ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the new files.
- [ ] `go test ./internal/hooks/...` GREEN (every branch); `go test ./...` GREEN (no regression).
- [ ] go.mod/go.sum byte-unchanged; only `internal/hooks/runner.go` + `runner_test.go` (+ optional git.go TopLevel).

### Feature Validation
- [ ] `RunCommitHooks` runs pre-commit → prepare-commit-msg → commit-msg; returns (finalTree, finalMsg, nil) or an abort.
- [ ] `--no-verify` skips pre-commit + commit-msg only; dry-run skips pre-commit + post-commit (commit-msg runs).
- [ ] Pre-commit scoped (throwaway index); LIVE `.git/index` untouched (asserted); enforceSubset re-tree/sweep.
- [ ] Hook non-zero/timeout → `*generate.RescueError`; sweep → `ErrHookSweptConcurrentWork`.
- [ ] Absent/non-executable hook → silent skip; `RunPostCommit` disregards exit code.
- [ ] S2 seam (`shouldSkipStagehandPrepareCommitMsg=false`) in place.

### Code Quality Validation
- [ ] Hook exec via os/exec directly (NOT runWithEnv); env/timeout/stdin/stdout per FR-V6.
- [ ] Calls `enforceSubset` (same package) — does NOT re-implement the subset check.
- [ ] Anti-patterns avoided (see below); no caller wiring (M3.T2); no subset.go edit; no recursion (S2).

### Documentation
- [ ] Doc comments cite PRD §9.25 FR-V1–V8 + external_deps.md. No docs/*.md here (M3.T2.S1/M4.T1.S1 own feature docs).

---

## Anti-Patterns to Avoid

- ❌ **Don't use runWithEnv for hook exec.** It runs the git binary and is `*gitRunner`-only (not on the Git
      interface). Hooks are user scripts — `os/exec.CommandContext` directly (§19). (research §3)
- ❌ **Don't skip prepare-commit-msg under --no-verify.** `--no-verify` bypasses ONLY pre-commit + commit-msg
      (git-commit(1) parity). prepare-commit-msg + post-commit ALWAYS run. (research §1)
- ❌ **Don't touch the live .git/index.** Pre-commit runs against a THROWAWAY tmpIndex (ReadTreeInto/WriteTreeFrom
      with GIT_INDEX_FILE). The live index is byte-for-byte unchanged — the stage-while-generating invariant. (research §5)
- ❌ **Don't conflate the two abort kinds.** Hook non-zero/timeout → `*RescueError` (rescue state, FR44); sweep →
      `ErrHookSweptConcurrentWork` (non-rescue freeze abort). Both propagate; the caller branches. (research §6)
- ❌ **Don't re-implement the subset check.** `enforceSubset` (same package) is the FR-V3 backstop. Call it; do
      NOT edit subset.go. (research §5)
- ❌ **Don't wire into callers or implement recursion prevention.** Wiring is M3.T2/M3.T3; recursion (hook.Detect)
      is S2 (P1.M3.T1.S2). S1 stubs `shouldSkipStagehandPrepareCommitMsg=false`. Stop at the runner core.
- ❌ **Don't let RunPostCommit abort.** Its exit code is DISREGARDED (commit already landed). Log @verbose; never
      undo; return nil. (research §9)
- ❌ **Don't block on stdin.** Use `/dev/null` (strings.NewReader("")); a TTY stdin would hang the hook.
- ❌ **Don't derive the worktree from GitDir's parent.** WRONG for linked worktrees. Use `git rev-parse
      --show-toplevel` (confirm/add `g.TopLevel`). (research §10)
- ❌ **Don't add new external deps / edit PRD.md / wire callers / edit subset.go.** Scope is runner.go + runner_test.go.

---

## Confidence Score

**8/10** — a substantial but well-bounded runner: the git-hook semantics are pinned by `external_deps.md`
(order/args/env/--no-verify/X_OK — authoritative), the dependency signatures are verified in-tree
(`enforceSubset` same-package; `HooksPath`/`GitDir`/`ReadTreeInto`/`WriteTreeFrom` on the Git interface;
`RescueError{Kind,TreeSHA,ParentSHA,Candidate,Cause}`; `ui.Verbose`; `cfg.NoVerify`/`cfg.HookTimeout`), and
the two abort kinds + the scoped-index/live-index-untouched invariant are precisely specified with asserting
tests. The code skeleton is structurally complete. The -2 reserves for: (a) the one interface detail to
confirm/add (`g.TopLevel` for the worktree — a one-method extension if absent); (b) the prepare-commit-msg
`""`-vs-absent argv (a 5s verify-test resolves it; the PRD `""` default is safe either way); (c) the
shell-script-hook integration tests being environment-sensitive (chmod/exec on the test host) — mitigated by
the temp-repo pattern used throughout `internal/git` tests. The S2 seam keeps recursion-prevention cleanly
deferred, and the no-wiring/no-subset-edit/no-recursion scope prevents collision with the sibling/parallel work.
