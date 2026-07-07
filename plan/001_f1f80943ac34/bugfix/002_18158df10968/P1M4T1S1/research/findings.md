# Research Findings — P1.M4.T1.S1 (bugfix-002): dry-run RescueError branch in handleGenError

Research-only scout. Repo: `/home/dustin/projects/stagecoach`. All refs verified 2026-06-30.

## 1. The exact function to edit

`handleGenError` (internal/cmd/default_action.go:169-188). Current body (verified):
```go
func handleGenError(stderr io.Writer, err error) error {
	var re *generate.RescueError
	if errors.As(err, &re) {            // covers BOTH ErrTimeout and ErrRescue
		fmt.Fprintln(stderr, generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
		code := exitcode.Rescue
		if errors.Is(err, generate.ErrTimeout) {  // timeout → 124; rescue → 3
			code = exitcode.Timeout
		}
		return exitcode.New(code, nil)   // silent → main prints nothing extra
	}
	var ce *generate.CASError
	if errors.As(err, &ce) { ... }       // exit 1
	if errors.Is(err, generate.ErrNothingToCommit) { ... }  // exit 2
	return exitcode.New(exitcode.Error, err)  // generic → exit 1
}
```

The new dry-run branch goes at the **TOP**, before the existing `errors.As(err, &re)` block.

## 2. `flagDryRun` is a package var — directly readable

`internal/cmd/root.go:40`: `flagDryRun bool`. It is a package-level var in `internal/cmd`, so `handleGenError` (same package) reads it directly — NO signature change, NO new parameter, NO new import.

## 3. The error sentinels / RescueError (confirmed in internal/generate/generate.go)

- Line 54: `var ErrTimeout = errors.New("stagecoach: generation timed out")`
- Line 59: `var ErrRescue = errors.New("stagecoach: commit generation failed after retries")`
- Lines 76-93: `type RescueError struct { Kind error; TreeSHA, ParentSHA, Candidate string }`;
  `func (e *RescueError) Unwrap() error { return e.Kind }` → `errors.Is(err, generate.ErrTimeout)`
  works because Unwrap returns the Kind.

So `pkg/stagecoach.runPipeline` returns `*generate.RescueError{Kind: ErrTimeout|ErrRescue}` for dry-run
generation failures (library UNCHANGED by this task). The CLI's `errors.As(err, &re)` + `errors.Is(err, generate.ErrTimeout)`
discrimination pattern is reused verbatim for the dry-run branch.

## 4. exitcode values (internal/exitcode/exitcode.go)

- `Error = 1`, `NothingToCommit = 2`, `Rescue = 3`, `Timeout = 124` (lines 23-27).
- `exitcode.New(code, nil)` → SILENT (main prints nothing extra, because the code already wrote to
  stderr via fmt.Fprintln). `exitcode.New(code, err)` → main prints `stagecoach: <err>`.
- The contract wants: dry-run gen failure → exit 1 + a SHORT stderr line + NO recovery recipe.
  So: `fmt.Fprintln(stderr, msg)` then `return exitcode.New(exitcode.Error, nil)`.

## 5. The library contract is UNCHANGED (critical regression guard)

`pkg/stagecoach/stagecoach_test.go` `TestGenerateCommit_Timeout` "dryrun" subtest (~296-362) asserts
the LIBRARY returns `*RescueError{Kind:ErrTimeout}` → `exitcode.For(err) == Timeout (124)`. This task
ONLY changes the CLI rendering in `handleGenError`. The library (`pkg/stagecoach`) is untouched, so that
test continues to pass unchanged. DO NOT modify it.

## 6. The COMMIT-path CLI tests stay as-is (second regression guard)

- `TestRunDefault_Rescue` (default_action_test.go:504) → exit 3 (blank stub, max_duplicate_retries=0).
- `TestRunDefault_Timeout` (default_action_test.go:563) → exit 124 (slow stub + short timeout).
Neither uses `--dry-run`, so the new `if flagDryRun` branch is skipped → behavior unchanged.
DO NOT modify them.

## 7. Test patterns to model the new dry-run variants on

- `TestRunDefault_DryRun` (268-308): the `rootCmd.SetArgs([]string{"--provider","stub","--dry-run"})`
  pattern + `saveRootState`/`restoreRootState` bracket + stdout/stderr buffers + `Execute(ctx)`.
- `TestRunDefault_Timeout` (563-595): uses `setupStubRepoWithTimeout(t, out, sleepMs, timeout)`
  (default_action_test.go:109-133) which builds the stub, writes a repo-local `.stagecoach.toml` with
  a short `[defaults] timeout`, commits the config, and sets `STAGECOACH_STUB_SLEEP_MS`.
- `TestRunDefault_Rescue` (504-541): uses `setupStubRepoRaw` + `[generation] max_duplicate_retries = 0`
  + `STAGECOACH_STUB_OUT=""` (blank → unparseable → parse-retry exhaustion → rescue).

So the two new tests:
- (a) dry-run TIMEOUT: `setupStubRepoWithTimeout(t, "feat: slow", 2000, 150*time.Millisecond)`,
  SetArgs `{"--provider","stub","--dry-run"}` → assert `exitcode.For(err) == exitcode.Error` (1) +
  stderr contains the short timeout message + stderr does NOT contain "git commit-tree" (no recipe).
- (b) dry-run RESCUE: `setupStubRepoRaw` + `max_duplicate_retries = 0` + `STAGECOACH_STUB_OUT=""`,
  SetArgs `{"--provider","stub","--dry-run"}` → exit 1 + short message + no recipe.

## 8. flagDryRun isolation between tests (verified safe)

`saveRootState`/`restoreRootState` (root_test.go:104-127) + `resetFlags` (root_test.go:131-137) reset
every changed flag (including `--dry-run`) back to its DefValue ("false") in t.Cleanup. So
`flagDryRun` does NOT leak across tests. The new tests follow the same save/restore bracket — no
special handling needed.

## 9. The short messages (exact wording from the contract)

```
timeout (ErrTimeout):  "generation timed out; run without --dry-run to see the recovery recipe"
rescue   (ErrRescue) : "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
```
Default msg = the rescue one; the `errors.Is(err, generate.ErrTimeout)` check overrides it with the
timeout wording. Both end in a hint to re-run without --dry-run (the recovery recipe lives there).

## 10. Docs touched (Mode A — rides with this implementing subtask)

- `docs/cli.md` §"Global flags" `--dry-run` row (line ~26, the table row): add that a dry-run
  GENERATION failure (timeout or parse/dedupe exhaustion) exits 1 with a short message rather than
  3/124 + a recovery recipe.
- `docs/cli.md` §"Exit codes" (line ~76, the table): add a note (under the table or appended prose)
  that `--dry-run` generation failures report exit 1 with a short message (not 3/124 + recipe), since
  no commit was ever intended. Keep exit codes 3/124 documented for the non-dry-run paths.

## 11. Boundaries / siblings

- This subtask (S1) = the CLI `handleGenError` branch + the two CLI tests + Mode A docs/cli.md edits.
- The library `pkg/stagecoach` is UNCHANGED (the rescue/timeout semantics there are intentional per
  bugfix-001 FR49). Only the CLI wraps a dry-run gen failure to exit 1.
- P1.M5 (Mode B final doc sweep: README.md + docs/how-it-works.md) runs LAST and depends on every
  implementing subtask; S1's cli.md edits are the authoritative source P1.M5 reconciles against.
- CAS / nothing-to-commit / generic errors on dry-run are NOT special-cased — they already exit 1/2
  appropriately (the new branch only intercepts `*RescueError`).

## 12. Validation commands (verified from Makefile)

- `go build ./...` / `go vet ./...`
- `go test -race ./internal/cmd/... ./pkg/stagecoach/...` (the targeted gate; the library test guard
  lives here too).
- `go test -race ./...` (full suite).
- `make lint` (golangci-lint).
