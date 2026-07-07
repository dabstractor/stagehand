# Verification — P1.M3.T1.S1 (empty-message guard in CommitStaged after RunCommitHooks, Issue 4)

> Live-tree confirmation that the contract's fix + test are exact and conflict-free. Numbered for cross-
> reference from the PRP. (2026-07-06)

## §1 — The gap is exactly as described (CommitStaged has NO empty check after hooks)

`internal/generate/generate.go`, `CommitStaged`:
- L~410-413: the `--edit` path runs `EditMessage` and propagates its error BARE (`return Result{}, err //
  ErrEmptyMessage propagates BARE → exitcode.For() → exit 1 (NOT rescue)`). So the editor path DOES guard
  empty (via `EditMessage`'s `if edited == "" { return "", ErrEmptyMessage }` at finalize.go:117-118).
- L426-432: the hooks block — `if deps.Hooks != nil { ft, fm, herr := deps.Hooks.RunCommitHooks(...); if
  herr != nil { return Result{}, herr }; treeSHA, msg = ft, fm }`. After this, `msg` is the hook-adjusted
  message (possibly emptied by a prepare-commit-msg or commit-msg hook). **There is NO empty check here.**
- L434-439: `parents` block, then `newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)` — the
  (possibly empty) msg flows straight into the commit. **This is the bug (Issue 4).**

The hooks run AFTER EditMessage, so a hook can empty a message the editor already validated — the editor's
guard cannot catch it. Confirmed by reading the function top-to-bottom.

## §2 — ErrEmptyMessage exists and is the right sentinel (same package, bare → exit 1)

`internal/generate/finalize.go:45`: `var ErrEmptyMessage = errors.New("stagecoach: empty commit message —
aborted")`. Used by `EditMessage` (finalize.go:117-118). It is a BARE error (not `*RescueError`) →
`exitcode.For()` maps it to exit 1 (NOT exit 3 rescue). The CommitStaged `--edit` path already propagates
it bare (L~411 comment). The new guard returns the SAME sentinel (`ErrEmptyMessage`, same package — no
import needed; `generate.go` is `package generate`). `strings` is ALREADY imported in generate.go.

## §3 — The fix (one guard, after the hooks block, before CommitTree)

After the `if deps.Hooks != nil { ... }` block (i.e., after `treeSHA, msg = ft, fm`), before the `parents`
block / `CommitTree`:
```go
if strings.TrimSpace(msg) == "" {
    return Result{}, ErrEmptyMessage // §9.25 git parity (Issue 4): a hook emptied the message → abort (exit 1, NOT rescue)
}
```
Placement: AFTER the hooks block's closing `}` (unconditional — guards the final `msg` before CommitTree
regardless of whether hooks ran; harmless when hooks didn't run because EditMessage already guaranteed
non-empty). The contract's literal "after line 431 (`treeSHA, msg = ft, fm`)" reading (inside the block) is
equally valid (it guards the hooks-ran case specifically); the after-block placement is strictly more
defensive at zero cost. Either satisfies the contract. HEAD + live index are untouched (the abort returns
before CommitTree → no update-ref ran) — a clean abort, byte-identical state to EditMessage's empty abort.

## §4 — The test (TDD; mirrors TestCommitStaged_PreCommitAbort_IsRescue)

`internal/generate/hooks_freeze_test.go` is the hook-test home AND is an EXTERNAL test (`package
generate_test`) — required because a white-box `package generate` test cannot import `internal/hooks`
(cycle: hooks imports generate for RescueError). `ErrEmptyMessage` is EXPORTED, so the external test
references it as `generate.ErrEmptyMessage`. The existing `TestCommitStaged_PreCommitAbort_IsRescue` is the
exact template (install a shell-script hook + `stubtest.Manifest` + `deps := generate.Deps{Git: git.New(repo),
Manifest: m, Hooks: hooks.DefaultRunner{}}` + `generate.CommitStaged` + assert error + HEAD unchanged).

New test `TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort`:
- install a `commit-msg` hook that empties the file: `#!/bin/sh\n> "$1"\nexit 0\n` (chmod 0755).
- stub outputs a NON-empty message (`"feat: non-empty generated message"`) so generation succeeds; the hook
  then empties it.
- assert `errors.Is(err, generate.ErrEmptyMessage)` (NOT `*RescueError` — this is a bare exit-1 abort).
- assert NO commit created (`git rev-parse HEAD` unchanged) — the abort returned before CommitTree.
- (Optional) assert the live index idempotent (the existing template's check).

**TDD:** before the guard, CommitStaged succeeds (err==nil) and creates a commit with an empty message →
the test's `err == nil` → `t.Fatal` FAILS. After the guard, `err == ErrEmptyMessage` → passes. This proves
the test catches the regression.

## §5 — No conflict with the parallel work item (P1.M2.T1.S1)

P1.M2.T1.S1 (the `no_verify` git-config layer fix, Issue 3) touches: `docs/*`, `internal/cmd/root.go`,
`internal/config/*`, `internal/hooks/runner.go`. It does **NOT** touch `internal/generate/generate.go` or
`internal/generate/hooks_freeze_test.go` (this task's two files). Zero file overlap ⇒ no merge conflict;
the two are independent. (P1.M1.T1.S1/T2.S1 — Issues 1 & 2 — are Complete and also don't touch generate.go's
CommitStaged guard; they fixed the hooks runner's argv + newline.)

## §6 — Scope boundary (S1 = CommitStaged ONLY; S2/S3 = the other two paths)

The PRD Issue 4 names THREE call sites with the same gap: `CommitStaged` (generate.go), `runPipeline`
(pkg/stagecoach), and `publishCommit` (decompose). This task (S1) fixes **CommitStaged ONLY**. The sibling
S2 (P1.M3.T1.S2) fixes runPipeline; S3 (P1.M3.T1.S3) fixes publishCommit. Do NOT touch those files here.
