# P1.M3.T3.S1 — Research Findings (Auto-stage notice grammar "(1 files)" → "(1 file)")

Source: direct codebase reads + architecture/minor_fixes.md §Issue 6. No external research (pure Go
`fmt.Sprintf` + a branch; no new library). 4 tool calls.

## §0 — The single production site (internal/cmd/default_action.go)

The auto-stage notice, **line 150** (confirmed by grep — the ONLY site in the repo):
```go
n, err := g.StagedFileCount(ctx)   // line 141
if err != nil { ... }
if n == 0 {
    // line 147-149: Clean tree → early return, NOTICE SKIPPED (Issue 7 fix). Only n>=1 reaches line 150.
    return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
}
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 (text verbatim, em-dash; colorized)  ← LINE 150
```
- `n` is the int from `g.StagedFileCount(ctx)`. `n>=1` is guaranteed at line 150 (the `n==0` early
  return above skips the notice — so the grammar fix only ever sees n>=1; the singular case is exactly `n==1`).
- `u` is the `*ui.UI` in scope in `runDefault` (the `u.Yellow` call already uses it). `stderr` is the
  `io.Writer` param. `fmt` is already imported. NO new symbol/import for the fix.
- The em-dash `—` (U+2014) in "staged — staging" MUST be preserved (recurring codebase gotcha; the
  parallel P1.M3.T2.S1 flags the same em-dash invariant for ErrEmptyMessage).

## §1 — The verbatim fix (architecture/minor_fixes.md §Issue 6, lines 148-154)

```go
noun := "files"
if n == 1 {
    noun = "file"
}
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d %s).", n, noun)))
```
- A 3-line insert BEFORE the Fprintln (the `noun` conditional) + a 1-token edit to the format string
  (`%d files)` → `%d %s).` + the `noun` arg). The comment `// FR18 (text verbatim, em-dash; colorized)`
  is updated to note the singular/plural conditional (keep the FR18/em-dash/colorized notes).

## §2 — The existing test (internal/cmd/default_action_test.go:439-462) — STAYS GREEN

`TestRunDefault_AutoStageNotice_FR18`:
- `repo := setupStubRepo(t, "feat: auto")`
- `writeFile(t, repo, "u.txt", "content")` + `writeFile(t, repo, "v.txt", "content")` → 2 untracked files
- `rootCmd.SetArgs([]string{"--provider", "stub", "--single"})`
- asserts `strings.Contains(stderr, "Nothing staged — staging all changes (2 files).")` (line 460)
- ALSO asserts HEAD moved (`git log` subject == "feat: auto") — proves the run completed.

n=2 → "files" (plural) → the existing assertion STILL HOLDS after the fix (the n==1 branch is not
taken). NO edit to this test.

## §3 — The NEW n=1 test (to ADD) — mirrors §2 with ONE file + singular assertion

Model: a sibling test `TestRunDefault_AutoStageNoticeSingular_Issue6` (naming follows the codebase's
`TestRunDefault_*_Issue<N>` convention — cf. `_CleanTreeNoAutoStageNotice_Issue7` at line 480,
`_MissingProviderCommand_Issue3` at line 810, `_RepoLocalNoticeOnce_Issue5` at line 1227).
- `repo := setupStubRepo(t, "feat: one")`
- `writeFile(t, repo, "only.txt", "content")` → ONE untracked file (n=1 after AddAll)
- `rootCmd.SetArgs([]string{"--provider", "stub", "--single"})`
- assert `strings.Contains(stderr, "Nothing staged — staging all changes (1 file).")` (SINGULAR — the fix)
- NEGATIVE guard: assert `!strings.Contains(stderr, "(1 files)")` (the bug — locks regression)
- light HEAD-moved check (subject == "feat: one") to prove the run completed (mirrors §2's thoroughness).

## §4 — Why `strings.Contains` works against `u.Yellow` output

`u.Yellow` colorizes ONLY when the writer is a TTY. The test passes a `bytes.Buffer` (not a TTY) →
`u.Yellow` is a no-op → the stderr captured is the PLAIN string. PROVEN: the existing `_FR18` test
passes today with an exact `strings.Contains` match including the trailing period. The new n=1 test
uses the identical assertion shape → safe. (No need to strip ANSI codes.)

## §5 — Scope fences (zero overlap with siblings — confirmed)

TOUCHES (2 files):
- `internal/cmd/default_action.go` — REWRITE line 150 (the format string) + insert the `noun`
  conditional (3 lines) immediately before it; update the trailing comment.
- `internal/cmd/default_action_test.go` — ADD `TestRunDefault_AutoStageNoticeSingular_Issue6`. The
  existing `_AutoStageNotice_FR18` is UNCHANGED (n=2 stays green).

DOES NOT TOUCH (zero overlap):
- `internal/generate/finalize.go` + `generate_test.go` → P1.M3.T2.S1 (Issue 5). That PRP explicitly
  states "P1.M3.T3.S1 (Issue 6, default_action.go — zero overlap)" and "scope: git status ==
  finalize.go + generate_test.go". Different package entirely (`internal/generate` vs `internal/cmd`).
- `internal/git/tokengate.go` → P1.M3.T1.S1/T1.S2 (Issue 4). Different package (`internal/git`).
- README/docs → P1.M4.T1 (the contract says "DOCS: none — minor UI text polish, no config/API surface").
- main.go, exitcode.go, generate.go, decompose/*, pkg/stagecoach/*, go.mod, any PRD/task file.
NO new type/flag/import/dependency. NO behavior change beyond the one noun.

## §6 — Validation commands (verified against Makefile)

```bash
gofmt -l internal/cmd/default_action.go internal/cmd/default_action_test.go   # empty
go vet ./internal/cmd/...
go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice   # runs BOTH _FR18 (n=2) + _Issue6 (n=1)
go test ./internal/cmd/ -race                                     # full cmd-package regression
make test && make lint && make build
git status --porcelain   # == default_action.go + default_action_test.go ONLY
```
The `-run TestRunDefault_AutoStageNotice` filter matches both `..._AutoStageNotice_FR18` and
`..._AutoStageNoticeSingular_Issue6` (substring match) → both run in one invocation.
