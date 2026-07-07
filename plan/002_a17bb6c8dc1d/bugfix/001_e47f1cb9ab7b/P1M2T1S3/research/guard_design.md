# Research: HEAD-movement guard design (P1.M2.T1.S3)

## The critical design insight: guard must BYPASS retry-then-empty

`invokeStagerRetry` (closure in `runLoop`, decompose.go ~L330) treats ANY error from
`invokeStager` as a *retryable stager failure*: it retries once, and on a second failure
returns `nil` (treats the concept as empty → FR-M8 empty-skip).

If the guard's `ErrStagerMovedHEAD` were returned from `invokeStager` naively,
`invokeStagerRetry` would (a) RETRY the already-corrupted stager, and (b) on continued
failure treat it as empty. Both are WRONG — a HEAD-movement is a HARD abort, never
retry-then-empty, because the stager corrupted repo state (no snapshot to restore).

**Therefore the guard belongs INSIDE `invokeStagerRetry`**, and `errors.Is(err, ErrStagerMovedHEAD)`
must short-circuit the retry/empty branches at every error check.

## Chosen structure: a `runOnce` closure inside invokeStagerRetry

- Capture `preStagerHEAD` ONCE at the top of `invokeStagerRetry` (before the first `invokeStager`).
  Matches contract 3(a): "before the first invokeStager call".
- `runOnce`: call `invokeStager`, capture `postStagerHEAD`, compare. If `pre != post` return
  `fmt.Errorf("%w: ...", ErrStagerMovedHEAD, ...)`. Else return invokeStager's own error.
- First attempt: `runOnce()`. If nil → done. If `errors.Is(..., ErrStagerMovedHEAD)` → return it (HARD).
- Retry: `runOnce()` again. If nil → done. If HEAD-moved → return it (HARD).
- Otherwise → empty-skip (return nil), preserving FR-M8.

Capturing pre ONCE is correct: the guard guarantees HEAD never silently moves between attempts
(a failed attempt that also moved HEAD is itself caught → HARD abort, no retry).

## Error propagation → exit code 1 (verified, no new wiring needed)

```
invokeStagerRetry → ErrStagerMovedHEAD
  → runLoop returns (commits, nil, err)            [loop body: err != nil → drainMsg + return]
  → Decompose returns (DecomposeResult{Commits}, err)
  → runDecompose → handleDecomposeError(err)
  → NOT *RescueError / *CASError → exitcode.New(exitcode.Error, err)
  → main: exitcode.For → 1, prints "stagecoach: stager moved HEAD from ... to ..."
```
No change needed in exitcode.go or handleDecomposeError. The `%w` wrap makes
`errors.Is(err, ErrStagerMovedHEAD)` true (required for the unit test assertion).

## Unborn-repo handling

RevParseHEAD returns `("", true, nil)` on unborn. pre="" / post="" compare equal (guard passes)
UNLESS the stager created a commit (post="<sha>" → caught). Contract consideration 3(c) satisfied.

## Test seam mechanics

- The seam `deps.stager` has signature `func(ctx, Deps, prompt.PlannerCommit) error` and is
  dispatched by `invokeStager`. Test closures capture `repo` + `t` and run raw git via
  `dcmRunGit(t, repo, ...)` (see `dcmStagerSeam`).
- To MOVE HEAD in the rogue seam: `dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "rogue")`.
  On a BORN repo (initial commit present) this advances HEAD → pre != post.
- Happy-path seam: `dcmRunGit(t, repo, "add", file)` — mutates index only, HEAD unchanged.

## Where to put the sentinel

`ErrStagerMovedHEAD` → `internal/decompose/stager.go`, next to `ErrStagerFailed` (same domain,
same doc-comment style — Go's equivalent of the requested "[Mode A] JSDoc").

## Verified facts from the codebase

- `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)` — git.go interface + impl.
- `invokeStager` dispatcher: `internal/decompose/decompose.go` L485-491.
- `invokeStagerRetry` closure: `internal/decompose/decompose.go` ~L330-347.
- `deps.stager` seam field: `internal/decompose/roles.go` (Deps struct, unexported).
- Existing tests using the seam: TestDecompose_Overlap (L432), TestDecompose_EmptyConceptSkip (L494),
  TestInvokeStager_NilSeam (L1313), TestInvokeStager_NilDepsStager (L1342).
- Helpers: dcmInitRepo, dcmCommitRaw, dcmWriteFile, dcmRunGit, dcmPlannerManifest,
  dcmMessageScriptManifest, dcmAllRoles, dcmStagerSeam — all in decompose_test.go top section.
- Build/test: `go build ./...`, `go test -race ./...`, `golangci-lint run` (Makefile).
- Coverage gate (Makefile coverage-gate) is on internal/{git,provider,generate,config} ONLY —
  NOT decompose, so no coverage threshold pressure here.
