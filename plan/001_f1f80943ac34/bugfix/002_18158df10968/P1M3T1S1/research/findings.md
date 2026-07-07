# Research Notes — P1.M3.T1.S1 (Issue 3: clean WriteTree unmerged-conflict error)

Scope: make `git.WriteTree` (`internal/git/git.go`) return a single clean line on the
unresolved-merge-conflict failure instead of appending git's raw multi-line stderr. Behavior (exit 1,
pre-generation, HEAD/index untouched) is already correct — only the **message** changes.

## 1. The edit site (exact, verified)

`internal/git/git.go`, `func (g *gitRunner) WriteTree` (definition line 219). Current block (lines 224-226):

```go
	if code != 0 {
		return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
	}
```

The `%s` appends the FULL trimmed stderr (multi-line: `f.txt: unmerged (...)` ×N +
`fatal: git-write-tree: error building trees`).

## 2. The run() seam (git.go:95) — the only git-exec helper

`func (g *gitRunner) run(ctx, repo, args...) (stdout, stderr, exitCode, err)`. On a non-zero git exit it
returns `(stdout, stderr, exitCode, nil)` — `err` is nil, code is the real exit. So a second probe
`g.run(ctx, g.workDir, "ls-files", "-u")` is trivially available: non-empty trimmed stdout ⇒ unmerged
entries present (stages 1/2/3). On its own failure it returns a non-nil err (context cancelled / git
missing) → we fall through to the detailed message. Failure-path only — not hot.

## 3. Preferred fix (ls-files -u check) — accurate + preserves detail for non-conflict failures

```go
if code != 0 {
	// PRD §13.5: detect unresolved merge conflicts and return a single clean line (don't dump git's
	// raw multi-line stderr). `git ls-files -u` lists unmerged stage entries; non-empty ⇒ conflict.
	// Failure path only (not hot). On any ls-files error, fall through to the detailed diagnostic.
	if lsOut, _, _, lsErr := g.run(ctx, g.workDir, "ls-files", "-u"); lsErr == nil && strings.TrimSpace(lsOut) != "" {
		return "", errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagecoach")
	}
	return "", fmt.Errorf("git write-tree failed (exit %d): %s", code, strings.TrimSpace(stderr))
}
```
- `errors` IS already imported (git.go:6) — no new import.
- Clean message keeps the substring "unresolved merge conflicts" → existing test assertion holds.
- Clean message has NO raw noise (`fatal: git-write-tree`, `error building trees`) → new assertion passes.
- Non-conflict write-tree failure (rare: corrupted index) keeps the detailed diagnostic (incl. stderr).

(Minimal acceptable variant, per contract: just drop the `%s` —
`return "", errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagecoach")`.
exit-128-on-populated-index is unambiguously unmerged in practice. Either is acceptable; PRP specifies
the preferred variant as primary.)

## 4. Blast radius — message-only; exit code + call sites unchanged

WriteTree callers (both return the error as-is, untouched by S1):
- `internal/generate/generate.go:156` — `CommitStaged` step 3.
- `pkg/stagecoach/stagecoach.go:260` — `runPipeline` step 3 (dry-run + SystemExtra).

Flow: WriteTree error → caller returns it → CLI `handleGenError` (`internal/cmd/default_action.go:188`)
**default branch**: `return exitcode.New(exitcode.Error, err)` → exit 1; main prints `stagecoach: <msg>`.
A clean single-line `msg` ⇒ `stagecoach: unresolved merge conflicts in the index — resolve them first,
then re-run stagecoach` (exit 1). STILL pre-generation (write-tree is step 3, before step-5 generation),
STILL exit 1, STILL HEAD/index untouched. No RescueError/CASError/NothingToCommit reclassification.

## 5. Test — surgical additions to TestWriteTree_MergeConflict (writetree_test.go ~119-133)

Reuse `makeMergeConflict` fixture (writetree_test.go:22-68) as-is. In `TestWriteTree_MergeConflict`:
- KEEP: `strings.Contains(err.Error(), "unresolved merge conflicts")` (the clean message retains it).
- ADD: `!strings.Contains(err.Error(), "fatal: git-write-tree")` and `!strings.Contains(err.Error(),
  "error building trees")` (the clean message has no raw noise).
- KEEP: `sha == ""` assertion.

No new test fixtures, no new test functions required for S1 (the contract scope is exactly these two
additions to the existing test). The non-conflict write-tree failure path is an out-of-scope edge case.

## 6. DOCS — verify-and-skip (no edit required)

Contract DOCS clause is conditional. Verified via grep:
`grep -rn -i "merge.conflict|unresolved|resolve.*merge|unmerged" docs/ README.md` → **EMPTY**.
`docs/how-it-works.md` §"Failure modes and exit codes" (line 53) table has NO merge-conflict row (rows:
Agent missing / Generation failed / Generation timed out / CAS failure / Nothing to commit / General
error) — nothing to align. → "none — message-only change with no new config/API surface". Any proactive
merge-conflict doc narrative belongs to **P1.M5.T1.S2** (final docs sweep), not S1.

## 7. References

- `plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md` ISSUE 3 (root cause +
  fix variants + blast radius).
- PRD §13.5 ("resolve merge conflicts first") + §18.2 failure table (merge conflicts → exit 1).
