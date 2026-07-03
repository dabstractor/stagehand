# PRP for PFIX_M1_T002_S1

## Objective

Eliminate the SILENT exit-1 failure path (BUG-002, severity major) by making `cmd/stagehand/run.go` `runDefault()` actually print non-sentinel error messages to stderr before mapping them to an exit code. Today both the `maybeAutoStage` result and the non-sentinel `stagehand.GenerateCommit` result are passed straight to `mapErrorToExitCode(err) int`, which discards the message, so FR8 merge-conflicts, not-a-repo, and any other generic git/generate failure exit 1 with ZERO output. The fix adds a minimal, testable `reportError(out, err) int` seam that prints `out.Progressf("stagehand: %s\n", err)` for every error EXCEPT the four sentinels (`ErrNothingToCommit`, `ErrNothingStaged`, `ErrRescue`, `ErrHeadMoved`) whose producers ALREADY print their own human message (avoiding double-printing), then delegates to the unchanged pure `mapErrorToExitCode`. Both call sites collapse to `return reportError(out, err)`. Minimal, pattern-faithful, no regressions, FR51 stdout byte-clean invariant preserved.

## Context

## Goal

**Feature Goal**: BUG-002 is fixed: every failure path out of `stagehand`'s default action prints a human-readable diagnostic to STDERR. Specifically FR8 — when `write-tree` fails (e.g. unresolved merge conflicts in the index), stagehand aborts before any generation WITH A CLEAR ERROR. Equivalently the PRD §18.2 failure-mode table is satisfied for the `Merge conflicts in index` row ("Resolve merge conflicts first." / the git layer's own clear message) and the not-a-repo case. Self-printing sentinels (ErrNothingToCommit/ErrNothingStaged/ErrRescue/ErrHeadMoved) are NOT double-printed.

**Deliverable**: A focused patch to `cmd/stagehand/run.go` — one new pure helper `isAlreadyReported(err error) bool`, one new print+map seam `reportError(out *ui.Output, err error) int`, and the two `runDefault` call sites (`maybeAutoStage` result + `GenerateCommit` result) collapsed from `return mapErrorToExitCode(err)` to `return reportError(out, err)`. Plus white-box unit tests in `cmd/stagehand/run_test.go` following the existing `TestMapErrorToExitCode` style. No other files change; `mapErrorToExitCode` stays byte-for-byte identical.

**Success Definition**: Reproduction from BUG-002 no longer reproduces — (a) MERGE CONFLICT: after the documented `git merge b` conflict with a file staged, `stagehand --provider <any>` exits 1 AND prints a non-empty message to stderr (the git layer's merge-conflict / write-tree error). (b) NOT-A-REPO: `cd /tmp/empty && stagehand --provider <any>` exits 1 AND prints a non-empty message. The four sentinels still exit silently with their OWN messages (no duplication). `go build ./...`, `go vet ./...`, `gofmt -s -l` (empty), and `go test ./...` all pass.

## Why

- FR8 is a hard PRD requirement ("abort with a clear error before any generation"). Today it is violated: the merge-conflict WriteTree error is returned directly by `internal/generate/generate.go` (L214-216) and then SILENTLY swallowed by `mapErrorToExitCode`.
- A CLI that exits 1 with zero output is hostile UX — the user has no clue what failed. The git layer (`internal/git/git.go` `ExitError.Error()`) and GenerateCommit already produce excellent messages (e.g. `"git exited <code> (<args>): <stderr>"`); runDefault just throws them away.
- Bug reference: BUG-002 (severity major). Recorded in `plan/001_f1f80943ac34/bug_hunt_results.json` with a `suggestedFix` that this PRP implements in a cleaner, test-encapsulated form.

## Root Cause

`cmd/stagehand/run.go` `runDefault()` ends with two error-routing sites:
```go
if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
    return mapErrorToExitCode(err)            
}
...
_, err = stagehand.GenerateCommit(context.Background(), opts)
return mapErrorToExitCode(err)               

```
`mapErrorToExitCode(err) int` is a PURE int-returning mapper (it never prints). It matches the four sentinels and collapses EVERYTHING ELSE to `ui.ExitError` (1). Contrast with the EARLIER error paths in the SAME function which DO print:
```go
out.Progressf("stagehand: %s\n", err)
return ui.ExitError

```
So merge-conflicts (WriteTree failure, returned directly by generate.go L216), not-a-repo (HasStagedChanges/AddAll `*git.ExitError` wrapped by maybeAutoStage as `"stage: %w"`), and any other generic git/generate failure all exit 1 with NO output.

## All Needed Context

### Documentation & References

```yaml
- url: https:
  why: this codebase uses dense, single-purpose godoc; every new helper needs a comment in the same style (see existing mapErrorToExitCode / maybeAutoStage comments).
- docfile: PRD.md
  section: FR8 (L243) + §15.4 Exit codes (L960-966) + §13.5 edge cases (L822-823) + §18.2 failure-mode table (L1160-1170)
  why: FR8 mandates a CLEAR error before generation on write-tree failure; §18.2 row 'Merge conflicts in index | write-tree | Resolve merge conflicts first. | 1'; §15.4 table: exit 1 = General error. This fix makes runDefault honor them.
- file: cmd/stagehand/run.go
  why: the TARGET FILE. Owns runDefault (the bug), mapErrorToExitCode (the pure mapper that must stay pure + unchanged), buildOptions, resolveAndCheckProvider. The earlier error paths (config.Load / git.New / resolveAndCheckProvider) already use `out.Progressf("stagehand: %s\n", err)` — the EXACT print pattern to reuse.
  pattern: pure helpers are the hermetic test targets; runDefault RETURNS int (no os.Exit — main.go does the single os.Exit). `out` is already in scope in runDefault.
  gotcha: mapErrorToExitCode has an existing table-driven test TestMapErrorToExitCode — do NOT change its signature or behavior; add the printing in a NEW sibling helper.
- file: cmd/stagehand/stage.go
  why: owns maybeAutoStage + the ErrNothingStaged sentinel + the stager interface. maybeAutoStage ALREADY prints its own human messages before returning the sentinels: 'Nothing staged; nothing to commit.\n' (ErrNothingStaged, FR19) and 'Nothing to commit.\n' (ErrNothingToCommit, FR17). Wrapped git failures are returned as fmt.Errorf("stage: %w", err) with NO message printed.
  pattern: errors.Is on the sentinels is how downstream branches (errors.Is is used throughout; e.g. mapErrorToExitCode).
- file: pkg/stagehand/stagehand.go
  why: owns the public sentinel aliases ErrNothingToCommit / ErrRescue / ErrHeadMoved (re-exported from internal/generate). GenerateCommit returns WriteTree errors DIRECTLY (no message) and returns the sentinels AFTER their producer prints.
- file: internal/generate/generate.go
  why: L207-216 — WriteTree failure (FR8 merge conflict) is `return Result{}, err` DIRECTLY (no message printed); L243-354 ErrRescue returns AFTER Rescue() prints the block; L368-376 ErrHeadMoved returns AFTER out.Progressf prints the HEAD-moved block. Confirms WHICH sentinels self-print (ErrRescue, ErrHeadMoved) and which path is silent (WriteTree / generic fmt.Errorf).
- file: internal/git/git.go
  why: ExitError.Error() = fmt.Sprintf("git exited %d (%s): %s", Code, args, stderr) — the clear message that is currently swallowed. New() (binary missing) is a separate path already handled in runDefault.
- file: internal/ui/output.go
  why: Progressf writes to STDERR ONLY (FR51 stdout byte-clean invariant), format string owns its trailing \n. This is the sink the fix uses.
- file: cmd/stagehand/run_test.go
  why: white-box `package main`, stdlib + cobra + internal/* + pkg/stagehand ONLY, NO testify. TestMapErrorToExitCode is the table-driven style to mirror for TestReportError_*.
- file: plan/001_f1f80943ac34/bug_hunt_results.json
  why: BUG-002 entry — suggestedFix proposes guarding the print with four errors.Is checks; this PRP encapsulates that into isAlreadyReported + reportError for testability.

```

### Current vs Desired code at the two call sites

CURRENT (`cmd/stagehand/run.go` runDefault tail):
```go
if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
    return mapErrorToExitCode(err)            
}
...
_, err = stagehand.GenerateCommit(context.Background(), opts)
return mapErrorToExitCode(err)               

```

DESIRED:
```go
if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
    return reportError(out, err)              
}
...
_, err = stagehand.GenerateCommit(context.Background(), opts)
return reportError(out, err)                  

```

### Known Gotchas

```go

```

## Implementation Blueprint

### New helpers (cmd/stagehand/run.go) — pure, testable, house-style

```go
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

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY cmd/stagehand/run.go — add helpers
  - ADD the two godoc'd helpers reportError(out *ui.Output, err error) int and
    isAlreadyReported(err error) bool (exact bodies in the blueprint above).
    PLACE them adjacent to mapErrorToExitCode (they are its print-wrapping
    sibling). Imports `errors` + `github.com/dustin/stagehand/internal/ui` +
    `github.com/dustin/stagehand/pkg/stagehand` are ALREADY imported in run.go.
  - DO NOT modify mapErrorToExitCode, buildFlags, resolveAndCheckProvider,
    buildOptions, or any other existing function.
Task 2: MODIFY cmd/stagehand/run.go — rewire the two call sites in runDefault
  - SITE 1 (staging): change
        if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil {
            return mapErrorToExitCode(err)
        }
    to `return reportError(out, err)`.
  - SITE 2 (generation): change
        _, err = stagehand.GenerateCommit(context.Background(), opts)
        return mapErrorToExitCode(err)
    to `return reportError(out, err)`.
  - UPDATE runDefault's doc comment to note non-sentinel errors are now printed
    to stderr via reportError (FR8/§18.2). Keep the rest of the comment.
Task 3: ADD unit tests in cmd/stagehand/run_test.go (white-box package main, no testify)
  - ADD TestReportError table-driven, mirroring TestMapErrorToExitCode. Drive a
    fresh *ui.Output over a bytes.Buffer stderr (verbose=false, noColor=true)
    and assert (a) printed-or-not, (b) returned exit code. Cases:
      * nil                       -> not printed, ExitSuccess
      * ErrNothingToCommit        -> NOT printed, ExitNothingToCommit
      * ErrNothingStaged          -> NOT printed, ExitNothingToCommit
      * ErrRescue                 -> NOT printed, ExitRescue
      * ErrHeadMoved              -> NOT printed, ExitError
      * fmt.Errorf("git: unresolved merge conflicts in index")
                                  -> PRINTED (stderr contains 'merge conflicts'
                                     and the full text), ExitError
      * fmt.Errorf("stage: %w", &git.ExitError{...}) style wrapped error
                                  -> PRINTED, ExitError
      * arbitrary errors.New("boom") -> PRINTED, ExitError
  - ADD TestIsAlreadyReported asserting each sentinel (and a wrapped sentinel
    via fmt.Errorf("outer: %w", ...)) returns true and an arbitrary error
    returns false.
  - DO NOT modify existing tests.

```

### Integration Points

```yaml
NO database / config / routes changes.
NO public API change (cmd package; nothing exported).
NO new env vars / flags.
FR51: printing goes to stderr via out.Progressf — stdout stays byte-clean.
DOCS: no per-item doc edit required — PRD FR8 (L243), §13.5 (L822-823) and the
  §18.2 failure-mode table (L1160-1170) ALREADY specify the correct behavior
  ("Resolve merge conflicts first." / exit 1). No "silent" gap note exists in
  docs/CONFIGURATION.md or README.md to correct. (Mode B: the changeset-level
  docs already match the post-fix behavior.)

```

## Validation Loop

### Level 1 — Syntax/Type/Vet/Fmt (one command per gate)
- `go build ./...`
- `go vet ./...`
- `test -z "$(gofmt -s -l internal/ cmd/ pkg/)"`   # prints nothing & exits 0 when formatted

### Level 2 — Unit tests
- `go test ./cmd/stagehand/`   # the affected package (run.go + run_test.go)
- `go test ./...`              # full suite (must stay green; no regressions)

### Level 3 — End-to-end reproduction (proves BUG-002 is fixed)
Build the binary, then in a scratch dir:
- MERGE CONFLICT: `git init; echo a>f; git add f; git commit -m init; git checkout -b b; echo b>f; git commit -am b2; git checkout master 2>/dev/null || git checkout -B master 2>/dev/null; echo m>f; git commit -am m2; git merge b; echo x>>f; git add f` then `./stagehand --provider <any-installed> 2>/tmp/err.txt` — assert /tmp/err.txt is NON-empty (contains the merge/write-tree error) and exit code is 1.
- NOT-A-REPO: `cd "$(mktemp -d)" && /path/to/stagehand --provider <any> 2>/tmp/err2.txt` — assert /tmp/err2.txt is NON-empty (not-a-repo message) and exit code is 1.
- NO-REGRESSION (sentinels unchanged): in a clean repo with nothing staged, `./stagehand --no-auto-stage 2>/tmp/s.txt` — assert stderr contains exactly the friendly "Nothing staged; nothing to commit." (NOT duplicated) and exit code is 2.

## Final Validation Checklist

### Technical
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./...` is clean.
- [ ] `gofmt -s -l internal/ cmd/ pkg/` lists nothing.
- [ ] `go test ./...` is green (no regressions; new reportError tests pass).

### Feature (BUG-002 / FR8)
- [ ] Merge-conflict reproduction now prints a NON-empty stderr message and exits 1.
- [ ] Not-a-repo reproduction now prints a NON-empty stderr message and exits 1.
- [ ] The four sentinels are NOT double-printed (each still prints only its own producer message).
- [ ] stdout stays byte-clean (FR51) — all new printing is on stderr.

### Code quality
- [ ] mapErrorToExitCode is byte-for-byte unchanged (its existing test stays green).
- [ ] New helpers follow the pure-helper-as-test-target house style + dense godoc.
- [ ] Both call sites collapse to a single `return reportError(out, err)`.
- [ ] errors.Is used (not ==) so wrapped sentinels/git-errors resolve correctly.

## Anti-Patterns to Avoid
- Do NOT print inside mapErrorToExitCode (breaks its purity + existing test).
- Do NOT unconditionally print err (double-prints the four self-printing sentinels).
- Do NOT use == instead of errors.Is (wrapped errors would mis-resolve).
- Do NOT print to stdout (violates FR51 byte-clean invariant).
- Do NOT touch mapErrorToExitCode / generate.go / stage.go / the rescue or HEAD-moved paths (out of scope; risk of regression).
- Do NOT broaden the fix into a refactor — keep it minimal and focused on the silent-exit-1 bug.

## DOCS Impact
No per-item doc edit is required. PRD FR8 (L243), §13.5 edge-case "Unresolved
merge conflicts in the index ... aborts before any generation with 'resolve
merge conflicts first.'" (L822-823), and the §18.2 failure-mode table row
"Merge conflicts in index | write-tree | 'Resolve merge conflicts first.' | 1"
(L1164) ALREADY specify the target behavior this fix implements; no "silent"
gap note exists in docs/CONFIGURATION.md or README.md to correct. Mode B:
changeset-level docs already match the post-fix behavior.


## Implementation Steps

1. MODIFY cmd/stagehand/run.go: ADD two godoc'd pure helpers adjacent to mapErrorToExitCode — `func reportError(out *ui.Output, err error) int { if err != nil && !isAlreadyReported(err) { out.Progressf("stagehand: %s\n", err) }; return mapErrorToExitCode(err) }` and `func isAlreadyReported(err error) bool { return errors.Is(err, stagehand.ErrNothingToCommit) || errors.Is(err, ErrNothingStaged) || errors.Is(err, stagehand.ErrRescue) || errors.Is(err, stagehand.ErrHeadMoved) }`. The imports `errors`, `internal/ui`, and `pkg/stagehand` are already present. Do NOT modify mapErrorToExitCode or any other existing function.
2. MODIFY cmd/stagehand/run.go runDefault: SITE 1 — change `if err := maybeAutoStage(g, out, cfg, allFlag, noAutoStage); err != nil { return mapErrorToExitCode(err) }` to `return reportError(out, err)`. SITE 2 — change `_, err = stagehand.GenerateCommit(context.Background(), opts); return mapErrorToExitCode(err)` to `_, err = stagehand.GenerateCommit(context.Background(), opts); return reportError(out, err)`. Update runDefault's doc comment to note non-sentinel errors are now surfaced to stderr via reportError (FR8/§18.2); keep the rest of the comment.
3. ADD unit tests in cmd/stagehand/run_test.go (white-box package main, stdlib + internal/* + pkg/stagehand only, NO testify, mirror the existing TestMapErrorToExitCode table style): TestReportError driving a fresh ui.NewOutput over a bytes.Buffer stderr (verbose=false, noColor=true) asserting (printed?, exit code) for: nil->(no,ExitSuccess); ErrNothingToCommit->(no,ExitNothingToCommit); ErrNothingStaged->(no,ExitNothingToCommit); ErrRescue->(no,ExitRescue); ErrHeadMoved->(no,ExitError); a merge-conflict-style fmt.Errorf("git: unresolved merge conflicts in index")->(yes,ExitError); a wrapped fmt.Errorf("stage: %w", arbitraryGitErr)->(yes,ExitError); errors.New("boom")->(yes,ExitError). Also add TestIsAlreadyReported asserting each sentinel (and a wrapped sentinel via fmt.Errorf("outer: %w", sentinel)) returns true and an arbitrary error returns false. Do not modify existing tests.

## Validation Gates

### Level 1: 1

go build ./...

### Level 2: 1

go vet ./...

### Level 3: 1

test -z "$(gofmt -s -l internal/ cmd/ pkg/)"

### Level 4: 2

go test ./cmd/stagehand/

### Level 5: 2

go test ./...

## Success Criteria

- [ ] FR8 merge-conflict reproduction no longer fails silently: after creating a merge conflict and staging a file, `stagehand --provider <any>` exits 1 AND prints a NON-empty message to stderr (the git write-tree merge-conflict error), satisfying PRD FR8 / §18.2 row 'Merge conflicts in index'.
- [ ] Not-a-repo reproduction no longer fails silently: running stagehand in a non-git directory exits 1 AND prints a NON-empty stderr message (the git 'not a repository' error).
- [ ] No double-printing / no regression for the four self-printing sentinels: ErrNothingToCommit, ErrNothingStaged, ErrRescue, and ErrHeadMoved each still print ONLY their producer's own message and map to their existing exit codes (2/2/3/1). E.g. `stagehand --no-auto-stage` in a clean repo prints exactly 'Nothing staged; nothing to commit.' once and exits 2.
- [ ] mapErrorToExitCode is byte-for-byte unchanged (its existing TestMapErrorToExitCode stays green); the printing lives in the new reportError/isAlreadyReported seam.
- [ ] FR51 stdout byte-clean invariant preserved: all new output is on stderr via out.Progressf; stdout is touched only by GenerateCommit's success/dry-run blocks.
- [ ] All gates green: `go build ./...`, `go vet ./...`, `gofmt -s -l` (empty), `go test ./cmd/stagehand/`, and `go test ./...` pass with no regressions; new TestReportError + TestIsAlreadyReported pass.
- [ ] Fix is minimal and focused: only cmd/stagehand/run.go (helpers + 2 call sites + comment) and cmd/stagehand/run_test.go (new tests) change; generate.go, stage.go, mapErrorToExitCode body, rescue/HEAD-moved paths, and public API are untouched.

## References

- cmd/stagehand/run.go (TARGET: runDefault two silent call sites; mapErrorToExitCode pure mapper to keep unchanged; earlier error paths already use out.Progressf("stagehand: %s\n", err) — the pattern to reuse)
- cmd/stagehand/stage.go (maybeAutoStage already prints 'Nothing staged; nothing to commit.' / 'Nothing to commit.' before returning ErrNothingStaged/ErrNothingToCommit; wraps git failures as fmt.Errorf("stage: %w", err) with no message)
- pkg/stagehand/stagehand.go (public sentinel aliases ErrNothingToCommit/ErrRescue/ErrHeadMoved; GenerateCommit returns WriteTree errors directly with no message)
- internal/generate/generate.go (L207-216 WriteTree failure returned DIRECTLY = the FR8 silent path; L243-354 ErrRescue returns AFTER Rescue() prints; L368-376 ErrHeadMoved returns AFTER out.Progressf prints — confirms which sentinels self-print)
- internal/git/git.go (ExitError.Error() = 'git exited <code> (<args>): <stderr>' — the clear message currently swallowed; New() binary-missing is a separate already-handled path)
- internal/ui/output.go (Progressf writes to STDERR ONLY; format owns trailing \n; FR51 byte-clean invariant)
- cmd/stagehand/run_test.go (white-box package main test conventions; TestMapErrorToExitCode table-driven style to mirror for TestReportError/TestIsAlreadyReported)
- PRD.md FR8 (L243), §13.5 (L822-823), §15.4 exit codes (L960-966), §18.2 failure-mode table (L1164) — already specify the target behavior
- plan/001_f1f80943ac34/bug_hunt_results.json (BUG-002 suggestedFix: guard the print with four errors.Is checks before out.Progressf)
- plan/001_f1f80943ac34/prps/research/PFIX_M1_T002_S1_research.md (full root-cause + helper-design analysis)
