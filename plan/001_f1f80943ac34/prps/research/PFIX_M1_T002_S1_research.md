# Research — PFIX_M1_T002_S1 (BUG-002: Silent exit 1 on non-sentinel errors)

## 1. Root cause (confirmed by reading source)

`cmd/stagehand/run.go` `runDefault()` routes errors from two call sites through
`mapErrorToExitCode(err) int`, which is a PURE int-returning mapper (it discards
the message):

```go
// site 1 — staging
if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
    return mapErrorToExitCode(err)
}
// site 2 — generation
_, err = stagehand.GenerateCommit(context.Background(), opts)
return mapErrorToExitCode(err)
```

`mapErrorToExitCode` only matches the four sentinels
(`ErrNothingToCommit`, `ErrRescue`, `ErrHeadMoved`, `ErrNothingStaged`) and
collapses everything else to `ui.ExitError` (1). The message is never printed.

By contrast, the EARLIER error paths in the SAME function DO print:
```go
out.Progressf("stagehand: %s\n", err)   // e.g. config.Load, git.New, resolveAndCheckProvider
return ui.ExitError
```

## 2. Which sentinels ALREADY print their own message (must NOT double-print)

| sentinel | producer | already prints? |
|---|---|---|
| `stagehand.ErrNothingToCommit` | maybeAutoStage → "Nothing to commit." (FR17); also generate.go L201 (direct, CLI path reaches it only via staging) | YES |
| `ErrNothingStaged` | maybeAutoStage → "Nothing staged; nothing to commit." (FR19) | YES |
| `stagehand.ErrRescue` | generate.CommitStaged → Rescue() prints the tree-SHA + recovery block BEFORE returning | YES |
| `stagehand.ErrHeadMoved` | generate.go L368-374 → prints HEAD-moved block BEFORE returning | YES |
| **any other error** (merge-conflict WriteTree, not-a-repo, generic git/generate failure, provider-not-configured) | producer is SILENT | **NO → must print here** |

## 3. The FR8 path (merge conflict) — confirmed silent today

`internal/generate/generate.go` L207-216:
```go
// WriteTree aborts on an unresolved merge conflict BEFORE generation (FR8);
// its error is returned DIRECTLY ...
treeSHA, err := deps.Git.WriteTree()
if err != nil {
    return Result{}, err   // returned DIRECTLY, NOT printed
}
```
That non-sentinel error bubbles up: GenerateCommit → runDefault →
mapErrorToExitCode → exit 1, **message discarded**. FR8 violation confirmed.

The git layer already produces the helpful message:
`internal/git/git.go` `ExitError.Error()` →
`"git exited <code> (<args>): <stderr>"` (e.g. the "unresolved merge
conflicts" stderr from `git write-tree`). It just never reaches the user.

## 4. Suggested fix design (matches bug_hunt_results.json suggestedFix)

bug_hunt_results.json BUG-002 `suggestedFix` proposes guarding with four
`errors.Is` checks before printing. Encapsulating into a clean, testable
helper pair (the codebase's "pure helper as the hermetic test target" style,
mirroring `mapErrorToExitCode` + its `TestMapErrorToExitCode`):

```go
// reportError prints err to stderr (unless its producer already printed a
// human message) and returns the PRD §15.4 exit code. runDefault's seam for
// the maybeAutoStage + GenerateCommit results.
func reportError(out *ui.Output, err error) int {
    if err != nil && !isAlreadyReported(err) {
        out.Progressf("stagehand: %s\n", err)
    }
    return mapErrorToExitCode(err)
}

func isAlreadyReported(err error) bool {
    return errors.Is(err, stagehand.ErrNothingToCommit) ||
        errors.Is(err, ErrNothingStaged) ||
        errors.Is(err, stagehand.ErrRescue) ||
        errors.Is(err, stagehand.ErrHeadMoved)
}
```

Both call sites collapse to `return reportError(out, err)`.
`mapErrorToExitCode` stays PURE (its existing test is unaffected).

## 5. Test seam

White-box `package main` in `cmd/stagehand/run_test.go` (no testify; stdlib +
internal/* only — matches `TestMapErrorToExitCode`). Add `TestReportError_*`:
- nil → no print, returns ExitSuccess.
- each self-printing sentinel → NOT printed, returns its code (2/3/1).
- a wrapped merge-conflict-style error → printed to a bytes.Buffer stderr,
  returns ExitError(1).
- a plain arbitrary error → printed, ExitError(1).

## 6. Scope / anti-regression notes

- `mapErrorToExitCode` signature + behavior UNCHANGED (existing test green).
- `ErrRescue`/`ErrHeadMoved` blocks in generate.go UNCHANGED (no double-print).
- FR51 stdout byte-clean invariant UNCHANGED (printing goes to stderr via
  Progressf, exactly like the existing earlier error paths).
- No new env vars, no public-API change, no doc edit required (PRD FR8 + §18.2
  failure-mode table already specify the correct behavior; no "silent" gap
  note exists in docs/ to correct).

## 7. Verified validation gates (all exit 0 today on current tree)

- `go build ./...`
- `go vet ./...`
- `test -z "$(gofmt -s -l internal/ cmd/ pkg/)"`  → prints nothing, exits 0
- `go test ./cmd/stagehand/`
- `go test ./...`
