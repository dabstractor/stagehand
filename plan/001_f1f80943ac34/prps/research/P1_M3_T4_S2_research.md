# Research — P1.M3.T4.S2: internal/git/stage.go — HasStagedChanges / AddAll

## ⚠️ CRITICAL: the work-item contract INVERTS the `git diff --cached --quiet` exit codes

The work-item contract states:

> HasStagedChanges() (bool, error) — `git diff --cached --quiet`;
> exit 0 → true (staged), exit 1 → false (clean), other → error.

**This mapping is empirically FALSE and must be inverted in the implementation.**

### Empirical verification (git 2.54.0, host)

```
# unborn repo, nothing staged
$ git diff --cached --quiet; echo exit=$?     -> exit=0

# unborn repo, file staged (git add a.txt)
$ git diff --cached --quiet; echo exit=$?     -> exit=1

# committed repo, clean index
$ git diff --cached --quiet; echo exit=$?     -> exit=0

# committed repo, modified+staged tracked file
$ git diff --cached --quiet; echo exit=$?     -> exit=1
```

So reality is: **exit 0 = clean (no staged changes); exit 1 = staged changes present.**

### Corroborating evidence the contract is the inverted one

1. **git `--exit-code` doc** (printed by `git diff --no-index` usage above):
   "exit with 1 if there were differences, 0 otherwise". `--quiet` implies `--exit-code`.
2. **PRD FR16**: "If `git diff --cached --quiet` reports no staged changes (FR5 path): if
   `auto_stage_all` is enabled ... run `git add -A`, then re-check". ⇒ exit 0 = "no staged
   changes". The PRD is self-consistent with git reality; only the work-item contract line is
   inverted.
3. **Method name semantics**: `HasStagedChanges` MUST return `true` when staged changes exist
   (= exit 1). The literal contract would make it return `true` on a clean index — the opposite
   of its name, which would break the CLI empty-check (P1.M7.T2.S1) and the auto-stage path.
4. **Reference impl (commit-pi)** uses `git diff --cached --quiet` for the "nothing staged"
   detection in §1 (`if diff empty → ...`). Consistent with exit 0 = clean.

### RESOLUTION used in the PRP

HasStagedChanges maps:
- **exit 0** → `return (false, nil)`   // clean index, no staged changes
- **exit 1** → `return (true, nil)`    // staged changes present
- **any other non-zero** (e.g. non-repo) → `return (false, ee)` where `ee` is the typed
  `*ExitError` from `(g *Git).run` (genuine failure, surfaced as-is)

Because `run` returns a `nil` error on exit 0, the implementation is:
```go
func (g *Git) HasStagedChanges() (bool, error) {
    _, err := g.run("diff", "--cached", "--quiet")
    if err == nil {
        return false, nil // exit 0 — clean index
    }
    var ee *ExitError
    if errors.As(err, &ee) && ee.Code == 1 {
        return true, nil  // exit 1 — staged changes present (git diff --exit-code convention)
    }
    return false, err // any other non-zero — genuine failure
}
```

## Other verified facts

### HasStagedChanges works correctly on an UNBORN repo (NO special handling needed)
Unlike `rev-list --count HEAD` / `git log` (which exit 128 on an unborn repo and need the
"unknown revision" / "does not have any commits yet" detection), `git diff --cached --quiet`
on an unborn repo exits:
- 0 when nothing is staged (clean), 1 when a file is staged.
So stage.go does NOT need any unborn-repo special-casing — it is SIMPLER than log.go. Verified.

### `git add -A` (AddAll) behavior
- On a clean unborn repo (nothing to add): exit 0. Verified.
- On a repo with NEW + MODIFIED + DELETED files: stages all three (status shows `D f.txt` deleted
  staged + `A new.txt` new staged), exit 0. Verified.
- So AddAll just runs `git add -A` and returns `nil` on success or the typed `*ExitError` on
  failure. `--quiet`/`-A` are literal args (no shell — PRD §19).

### Non-repo error case for HasStagedChanges
Outside any repo, `git diff --cached` falls into `git diff --no-index` mode and errors
"unknown option `cached'" (exit 129). So the "other → error" path surfaces a non-1 `*ExitError`.
The test should assert `err != nil`, `errors.As(err, &ee)` is true, and the bool is `false` —
do NOT pin the exact exit code (it is 129 here but the contract's "other → error" intent is
"anything that isn't the clean/staged signal"). The non-repo test uses a `*Git` bound to a
`t.TempDir()` WITHOUT `git init` (same posture as git_test.go's TestRun_NonRepoIsExitError).

## Pattern references (all shipped, DO NOT modify)
- `internal/git/git.go`: `(g *Git) run(args ...string) (string, error)` (UNEXPORTED; returns nil
  error on exit 0, typed `*ExitError{Args,Code,Stderr}` on non-zero). `ExitError` is the type to
  `errors.As` into. Confirms stage.go must be `package git` (white-box).
- `internal/git/plumbing.go` + `internal/git/diff.go` + `internal/git/log.go`: in-package
  method-file precedents — plain `package git` line + leading file-level comment (git.go OWNS the
  `// Package git` doc), thin methods over `g.run`, `errors.As` into `*ExitError`.
- `internal/git/gittestutil_test.go` (S2 harness): `newTempRepo(t)` (unborn repo, deterministic
  identity), `seedCommits(t, g, msgs)`, `writeFileStage(t, g, path, content)`, `mustRun(t, g, args...)`.
  stage_test.go composes these; it is white-box `package git`.
- `internal/git/git_test.go`'s `TestRun_NonRepoIsExitError`: the non-repo → typed `*ExitError`
  assertion posture to mirror for the HasStagedChanges error case.

## Test design (stage_test.go, white-box package git, stdlib testing only, REAL git)
1. `TestHasStagedChanges_FalseOnCleanIndex` — seed a commit, make NO changes; assert (false, nil).
2. `TestHasStagedChanges_TrueAfterStagingAFile` — write+stage a file (unborn repo is fine); assert (true, nil).
3. `TestHasStagedChanges_TrueAfterModifyingTrackedAndStaging` — seed commit, modify+stage tracked file; assert (true, nil).
4. `TestHasStagedChanges_NonRepoIsError` — *Git on a non-init temp dir; assert (false, err) with err a typed *ExitError (errors.As), do not pin the code.
5. `TestAddAll_StagesNewModifiedDeleted` — seed commit (f.txt); create new.txt, delete f.txt, modify via write; AddAll(); then assert `git diff --cached --name-status` contains the new file (A) and the deleted file (D) — i.e. all three classes staged. Assert AddAll returned nil.
6. `TestAddAll_CleanRepoReturnsNoError` — unborn or committed repo with nothing to add; AddAll() returns nil; HasStagedChanges() still (false, nil).

Naming: `TestHasStagedChanges_<Scenario>` / `TestAddAll_<Scenario>`. One behavior per Test*.
