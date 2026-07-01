# Issue 3: Post-Arbiter Output Fidelity (Major)

## Root Cause

`runDecompose` in `internal/cmd/default_action.go` prints `res.Commits` from `DecomposeResult`:
```go
// default_action.go:297
res, derr := decompose.Decompose(ctx, deps)
for _, c := range res.Commits {
    printDecomposeCommit(stdout, c)
}
```

`DecomposeResult.Commits` carries the **loop's pre-arbiter** entries (acknowledged internally as
the "post-arbiter gap" — see `decompose.go`'s `§G-RESULT` doc comment on `DecomposeResult`).

After the arbiter runs (`resolveArbiter` in `chain.go`), SHAs change:
- **Tip amend** (`resolveTipAmend`): the amended commit gets a **new SHA** (new tree + CommitTree
  creates a different object), but stdout prints the **old (pre-amend) SHA**, which is now dangling.
- **Mid-chain rebuild** (`resolveMidChain`): commits `[i..N-1]` are rebuilt with new SHAs; stdout
  prints the old SHAs for the rebuilt entries.
- **Null / new-commit** (`resolveNewCommit`): the arbiter creates an (N+1)-th commit that lands in
  `git log` but is **never printed** to the user at all.

## Why SHAs Change

The arbiter's resolution paths all use `CommitTree` + `UpdateRefCAS`:
- `resolveTipAmend`: `CommitTree(treePrime, [tipParent], tipMsg)` → new SHA (different tree = different commit).
- `resolveMidChain`: each rebuilt `CommitTree(treePrime, parent, msg)` → new SHA.
- `resolveNewCommit`: `CommitTree(treePrime, [tipSHA], msg)` → new (N+1)-th commit.

None of these return the new SHAs to the caller — they just move HEAD.

## The Fix

After a successful arbiter phase, re-read git for the final commits in the range
`preRunHEAD..HEAD` and replace `DecomposeResult.Commits` with accurate post-arbiter entries.

### Step 1: Add a git commit-range listing function

The Git interface (`internal/git/git.go`) currently has NO range-based commit listing. Add:

```go
// LogEntry is one commit in a log range (oldest-first when via LogRange).
type LogEntry struct {
    SHA     string // full commit SHA
    Subject string // first line of the commit message
}

// LogRange returns the commits in the range baseSHA..HEAD, oldest-first.
// Runs `git log --reverse --format=%H%x1f%s baseSHA..HEAD`.
// For a root-commit base (preRunHEAD was unborn), pass the all-zeros SHA or
// use a special form (see implementation notes below).
LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error)
```

**Implementation notes**:
- Uses `git log --reverse --format=%H%x1f%s <baseSHA>..HEAD`.
- `%x1f` (ASCII Unit Separator) delimits SHA from subject (safe delimiter — subjects never contain it).
- For the **unborn-repo** edge case (preRunHEAD was `""`/isUnborn): `git log --reverse <all-zeros>..HEAD`
  doesn't work (all-zeros is not a valid range base). Instead use `git log --reverse --format=... HEAD`
  and limit to the commits created this run. Alternatively, since this run created ALL commits on an
  unborn repo, `LogRange` with the unborn base should return ALL commits. The cleanest approach: when
  preRunHEAD indicates unborn, call `RecentMessages`/a new function to get all commits, or use
  `git log --reverse --max-count=<N+1>` where N is the number of commits.

  **Simpler approach**: Since `Decompose` already knows the commit count (len of the loop's commits +
  1 for null path), it can use `git log --reverse -<count>` on HEAD. But `LogRange` with an explicit
  base is more robust. For the unborn case, pass `git.EmptyTreeSHA` (the 40-zero SHA works as a base
  in `git log <sha>..HEAD` when the repo has commits — git treats it as "no commits reachable").

  **Actually**: `git log --reverse --format=... <base>..HEAD` works correctly when base is any SHA
  that is NOT reachable from HEAD (which is the case for a pre-run HEAD that was later superseded).
  For the unborn case, we can special-case: if preRunHEAD was `""`, use `git log --reverse` without
  a range base (all commits on HEAD).

### Step 2: Re-read commits after arbiter in Decompose

In `decompose.go`, after `runArbiterPhase` succeeds:

```go
// After successful arbiter (amended > 0, or arbiter ran):
if status != "" && len(commits) > 0 {
    amended, err = runArbiterPhase(...)
    if err != nil { return ..., err }
    // NEW: re-read the final commits for accurate SHAs
    commits = rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn)
}
```

`rereadFinalCommits`:
1. Call `deps.Git.LogRange(ctx, preRunHEAD)` (or the unborn variant) → `[]LogEntry`.
2. For each entry, call `deps.Git.DiffTree(sha, isRoot)` → `[]git.FileChange`.
3. Build `[]CommitResult{SHA, Subject, Message, Files}` and return.
   - `Message` can be set to `""` (it's carried for completeness but NOT used in the success report —
     only SHA + Subject + Files are printed).

**For the unborn edge case**: `preRunHEAD` is `""`. We can pass a special sentinel or handle it
inside `LogRange`. The simplest: pass the all-zeros SHA `strings.Repeat("0", 40)` as the base —
`git log 0000..HEAD` lists all commits on HEAD (the all-zeros ref is the "empty tree"/unborn
marker, and git treats `<zeros>..HEAD` as "everything reachable from HEAD").

Actually, the correct git semantics: `git log A..B` shows commits reachable from B but not from A.
For an unborn repo, there's no pre-existing commit A. But once the run creates commits, HEAD has
commits. We need "all commits created this run." The all-zeros SHA works as a non-existent ref:
`git log 0000000000000000000000000000000000000000..HEAD` lists ALL commits reachable from HEAD.

**Verified pattern**: This is exactly what `git rev-list <zeros>..HEAD` does — it lists all commits.

### Step 3: Update §G-RESULT doc and DecomposeResult.Amended

After the fix:
- `DecomposeResult.Commits` is now **accurate** post-arbiter (no stale SHAs, no missing commits).
- `DecomposeResult.Amended` is still computed before the arbiter for informational purposes, but
  the `Commits` slice is the source of truth for display.
- The `§G-RESULT` doc comment on `DecomposeResult` should be updated to reflect that the gap is
  closed (Commits are now re-read post-arbiter).

## What Does NOT Change
- `default_action.go` — `runDecompose` still iterates `res.Commits` and prints them; no change needed.
- The loop's pre-arbiter `commits` slice is still used for `buildArbiterCommits` (the arbiter input).
- The arbiter resolution logic in `chain.go` is unchanged.

## Test Strategy

1. **Unit test for `LogRange`**: Create a repo with known commits, call `LogRange(baseSHA)`, assert
   correct SHAs + subjects, oldest-first.

2. **Integration test for tip amend**: Run a decompose where the stub planner partitions into 2
   concepts, the arbiter folds leftover into the tip (tip amend path), and assert:
   - `DecomposeResult.Commits[last].SHA` matches `git log` (the post-amend SHA, not the pre-amend one).
   - `printDecomposeCommit` would print the correct (resolvable) SHA.

3. **Integration test for null path**: Run a decompose where the arbiter creates a new (N+1)-th
   commit, and assert `DecomposeResult.Commits` has N+1 entries (the new commit IS included).

4. **Integration test for mid-chain**: Run a decompose where the arbiter rebuilds from a mid-chain
   target, and assert all SHAs in `DecomposeResult.Commits` are resolvable and match `git log`.

## Files to Touch

| File | Change | Doc Mode |
|------|--------|----------|
| `internal/git/git.go` | Add `LogEntry` type + `LogRange` method to interface + `gitRunner` | JSDoc on new method |
| `internal/git/git_test.go` (or new `logrange_test.go`) | Unit tests for LogRange | — |
| `internal/decompose/decompose.go` | Add `rereadFinalCommits`; call it after arbiter; update §G-RESULT doc | JSDoc |
| `internal/decompose/decompose_test.go` | Integration tests for post-arbiter SHA accuracy | — |
