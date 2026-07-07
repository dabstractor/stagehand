name: "P1.M4.T2.S2 — Regression test: no '(0 files)' notice on a clean tree (Issue 7)"
description: |

  Test-only regression test for PRD Issue 7. CRITICAL STATE FINDING: the preceding subtask
  P1.M4.T2.S1 (commit `8fdcd29` "suppress misleading zero-file auto-stage notice on clean tree")
  already shipped BOTH (a) the production fix — the `if n == 0 { return ...NothingToCommit... }`
  early-return in `internal/cmd/default_action.go` — AND (b) an in-line Issue 7 regression
  assertion INSIDE the pre-existing `TestRunDefault_NothingStaged_FR17` test:

      // Issue 7: clean tree must NOT print the misleading "staging all changes" notice.
      stderr := errBuf.String()
      if strings.Contains(stderr, "staging all changes") {
          t.Errorf("stderr = %q, want NO auto-stage notice on a clean tree (Issue 7)", stderr)
      }

  That in-line assertion is LITERALLY the exact form the S2 contract specified
  (`if strings.Contains(stderr, "staging all changes") { t.Error(...) }`). So the contract's primary
  action ("Extend TestRunDefault_NothingStaged_FR17 …") is ALREADY DONE and GREEN.

  The genuine, NON-REDUNDANT deliverable for S2 is therefore the contract's EXPLICIT ALTERNATIVE —
  "(or add a dedicated test)" — which both the architecture doc (Part B "Tests") and the S1 PRP
  (Task 2 "ALTERNATIVE") sanction. This PRP adds a dedicated, self-documenting
  `TestRunDefault_CleanTreeNoAutoStageNotice_Issue7` that follows the established `_Issue<N>`
  regression-test convention already used by Issues 1, 3, and 5 (Issue 7 is currently the ONLY fixed
  issue without a dedicated `_Issue7` test). No production code is touched (test-only per the contract's
  DOCS: none).

---

## Goal

**Feature Goal**: Add a dedicated, named regression test `TestRunDefault_CleanTreeNoAutoStageNotice_Issue7`
to `internal/cmd/default_action_test.go` that proves PRD Issue 7 is fixed on a clean tree: with nothing
staged and the default `auto_stage_all = true`, running the default action (a) still exits with code 2
(`NothingToCommit`), (b) does NOT print the misleading "staging all changes" notice to stderr, and
(c) leaves HEAD unchanged. The test follows the exact harness pattern and naming convention of the
existing `TestRunDefault_*_Issue{1,3,5}` tests.

**Deliverable**: One new test function appended to `internal/cmd/default_action_test.go` (placed
immediately after `TestRunDefault_AutoStageNotice_FR18`). No other files change. No production edits.

**Success Definition**:
- `TestRunDefault_CleanTreeNoAutoStageNotice_Issue7` exists, is named with the `_Issue7` suffix, has a
  "proves Issue 7 is fixed" doc comment, and PASSES with `-race`.
- It asserts exit code == `exitcode.NothingToCommit` (2), `err.Error()` contains "Nothing to commit.",
  stderr does NOT contain "staging all changes", and HEAD is unchanged.
- The pre-existing in-line Issue 7 assertion inside `TestRunDefault_NothingStaged_FR17` (added by S1)
  is LEFT IN PLACE (an extra guard; do not remove it).
- `TestRunDefault_AutoStageNotice_FR18` (the n=2 positive notice) stays GREEN and UNCHANGED.
- `go build ./...`, `go vet ./...`, `gofmt -l`, `make lint`, and `go test -race ./...` all pass.

## User Persona (if applicable)

N/A — this is a developer-facing regression test. The "user" is a future maintainer who must be able to
discover, by name, that Issue 7 (the "(0 files)" clean-tree cosmetic bug) is guarded against regression.

## Why

- **Business value**: A dedicated `_Issue7` test is the discoverable, convention-consistent regression
  artifact. Issues 1, 3, and 5 each have a dedicated `TestRunDefault_*_IssueN` test; Issue 7 was the
  lone exception (its only guard was an anonymous in-line assertion buried in a test named `…_FR17`).
  This closes that consistency gap.
- **Integration**: Pure test addition in the CLI-layer test file. Zero production-code surface change.
  Sits alongside `TestRunDefault_NothingStaged_FR17` (clean-tree exit-2) and
  `TestRunDefault_AutoStageNotice_FR18` (dirty-tree notice) as the explicit Issue-7 regression point.
- **Problems solved**: Without a dedicated test, a future refactor that re-introduces the "(0 files)"
  notice on a clean tree would only be caught if someone happened to read the `…_FR17` body; a named
  `…_Issue7` test is greppable and self-documenting.

## What

A new test function in `internal/cmd/default_action_test.go` that, on a fully-clean working tree with
the default `auto_stage_all = true`, drives the default action end-to-end through the CLI seam and asserts:
the misleading "staging all changes" notice is ABSENT from stderr while the exit code is still 2 and HEAD
is still unchanged. It is a negative-of-the-positive `TestRunDefault_AutoStageNotice_FR18` (which asserts
the notice IS present when n=2).

### Success Criteria

- [ ] New test `TestRunDefault_CleanTreeNoAutoStageNotice_Issue7` added, named with `_Issue7` suffix.
- [ ] Test asserts: exit code == `exitcode.NothingToCommit` (2).
- [ ] Test asserts: `err.Error()` contains "Nothing to commit." (the main-printed message survives).
- [ ] Test asserts: stderr does NOT contain the substring "staging all changes" (the Issue 7 fix).
- [ ] Test asserts: HEAD unchanged (last commit subject still "init: add stagecoach config").
- [ ] Test uses the standard `saveRootState`/`restoreRootState` + `setupStubRepo` harness.
- [ ] `TestRunDefault_AutoStageNotice_FR18` and `TestRunDefault_NothingStaged_FR17` stay GREEN + unchanged.
- [ ] No production files modified (test-only).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact test to mirror (`TestRunDefault_AutoStageNotice_FR18`), the exact
clean-tree setup (`setupStubRepo` with everything committed), the exact assertions, the exact placement
(after FR18), the exact `_Issue7` naming convention, and the exact validation commands are all specified.

### Documentation & References

```yaml
# MUST READ - Include these in your context window

- file: internal/cmd/default_action_test.go
  why: The ONLY file to edit. Mirror the harness of TestRunDefault_AutoStageNotice_FR18 (lines ~419-452)
        and the clean-tree setup of TestRunDefault_NothingStaged_FR17 (lines ~311-348). Append the new
        test immediately after the FR18 test's closing brace.
  pattern: |
    Each test in this file follows this skeleton:
        func TestRunDefault_<Name>(t *testing.T) {
            origArgs, origOut, origErr, origRunE := saveRootState(t)
            defer restoreRootState(t, origArgs, origOut, origErr, origRunE)
            repo := setupStubRepo(t, "<stub output>")
            // ... arrange (write/stage files, or leave clean) ...
            var outBuf, errBuf bytes.Buffer
            rootCmd.SetOut(&outBuf)
            rootCmd.SetErr(&errBuf)
            rootCmd.SetArgs([]string{"--provider", "stub"})
            err := Execute(context.Background())
            // ... assertions on err, exitcode.For(err), outBuf, errBuf, git log ...
        }
  gotcha: |
    setupStubRepo(t, stubOut) creates a temp git repo, writes+COMMITS a .stagecoach.toml pointing at the
    stub binary, and chdirs into it. Because the config is committed, a repo straight from setupStubRepo
    with NO further writeFile/stageFile is a FULLY CLEAN tree — exactly the Issue 7 scenario. Do NOT add
    any writeFile/stageFile calls (that would make the tree dirty and defeat the test).

- file: internal/cmd/default_action_test.go (TestRunDefault_NothingStaged_FR17, ~line 311)
  why: S1 ALREADY added an in-line Issue 7 assertion at the end of this test (lines ~344-348):
            // Issue 7: clean tree must NOT print the misleading "staging all changes" notice.
            stderr := errBuf.String()
            if strings.Contains(stderr, "staging all changes") { t.Errorf(...) }
        This is the contract's "extend" path, already shipped + GREEN. LEAVE IT IN PLACE — it is a valid
        extra guard. Do NOT duplicate it and do NOT remove it.
  critical: |
    The S2 contract's PRIMARY action ("Extend TestRunDefault_NothingStaged_FR17 …") is ALREADY DONE by S1.
    Re-adding the same assertion there is a no-op. S2's real value is the DEDICATED test below (the
    contract's explicit "or add a dedicated test" alternative).

- file: internal/cmd/default_action_test.go (TestRunDefault_AutoStageNotice_FR18, ~line 419)
  why: The POSITIVE counterpart — writes 2 unstaged files, asserts stderr CONTAINS the verbatim
        "Nothing staged — staging all changes (2 files)." notice (em-dash U+2014). The new Issue 7 test
        is the NEGATIVE of this: clean tree (0 files) → notice ABSENT. Mirror FR18's harness exactly;
        only the arrange step (omit writeFile/stageFile) and the notice assertion (Contains vs !Contains)
        differ. This test MUST stay GREEN and unchanged — it guards the n>0 happy path.
  gotcha: The em-dash (—) in the FR18 literal is U+2014, not a hyphen; the new negative test must use the
        plain ASCII substring "staging all changes" (not the full em-dash string) so it is robust to the
        ANSI Yellow color wrapping that may surround the notice on a terminal.

- file: internal/exitcode/exitcode.go
  why: Defines exitcode.NothingToCommit (=2), exitcode.New(code, err) (*ExitError), and exitcode.For(err).
        The clean-tree path returns exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")).
        For() resolves that to 2; (*ExitError).Error() returns the wrapped err's string "Nothing to commit."
        (non-empty → main.go prints it).
  critical: Assert BOTH exitcode.For(err) == exitcode.NothingToCommit AND
        strings.Contains(err.Error(), "Nothing to commit.") — proving §15.4 exit-2 semantics AND the
        user-facing message both survive the fix.

- file: internal/cmd/default_action.go (runDefault, cfg.AutoStageAll branch, ~lines 68-90)
  why: READ-ONLY reference (do NOT edit). This is the code under test. The S1 fix is the
        `if n == 0 { return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) }`
        block (between the StagedFileCount error guard and the FR18 Fprintln). On a clean tree,
        AddAll is a no-op and StagedFileCount returns 0, so this early-return fires BEFORE the
        "staging all changes" Fprintln — which is exactly why stderr is clean. The test exists to
        lock this in.
  pattern: |
        case cfg.AutoStageAll:
            if err := g.AddAll(ctx); err != nil { ... }
            n, err := g.StagedFileCount(ctx)
            if err != nil { ... }
            if n == 0 {                                     // ← S1 fix (Issue 7); fires on clean tree
                return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
            }
            fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n)))
            ...

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_and_autostage.md
  why: PART B "Tests covering these paths" is the authoritative note for THIS test. It states the
        regression assertion should be added "to this test (or a new dedicated test)". S1 took the
        "to this test" path; S2 takes the "new dedicated test" path it explicitly permits.
  section: "## Tests covering these paths" → "Part B (auto-stage notice)" bullet on TestRunDefault_NothingStaged_FR17.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M4T2S1/PRP.md
  why: The S1 PRP's "Task 2" listed the dedicated test as an "ALTERNATIVE / ALSO ACCEPTABLE"
        (TestRunDefault_CleanTreeNoAutoStageNotice_Issue7) but the S1 implementer chose the in-line
        "extend" form instead. S2 implements that named alternative.
  section: "Implementation Tasks" → "Task 2" → "ALTERNATIVE / ALSO ACCEPTABLE".
```

### Current Codebase tree (relevant slice)

```bash
internal/
  cmd/
    default_action.go          # READ-ONLY (S1 fix already shipped here; the code under test)
    default_action_test.go     # ← EDIT HERE: append TestRunDefault_CleanTreeNoAutoStageNotice_Issue7
  exitcode/
    exitcode.go                # exitcode.NothingToCommit (=2), exitcode.For, *ExitError.Error
  git/ (git.go, addall_test.go, stagedcount_test.go)   # StagedFileCount→0 on clean tree (S1 proven here)
Makefile                       # build / test / lint / coverage targets
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/cmd/default_action_test.go   # MODIFIED: +1 test func TestRunDefault_CleanTreeNoAutoStageNotice_Issue7
```
No new files. No new packages. No production-code edits. No config/schema/migration changes.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (STATE): The contract's literal "extend TestRunDefault_NothingStaged_FR17" action is ALREADY
//   DONE by S1 (commit 8fdcd29). Re-adding that assertion is a no-op and will confuse reviewers. S2's
//   deliverable is the DEDICATED test (the contract's "or add a dedicated test" alternative). Do not
//   touch the existing in-line assertion in …_FR17; leave it as an extra guard.

// GOTCHA: setupStubRepo(t, stubOut) COMMITS the .stagecoach.toml, so a repo straight from it with NO
//   further writeFile/stageFile is a CLEAN tree. Adding any writeFile/stageFile would make the tree
//   dirty and break the test (the notice WOULD then correctly print for n>0). The Issue 7 scenario
//   requires the tree to be clean — so arrange NOTHING after setupStubRepo.

// GOTCHA: Assert the ASCII substring "staging all changes" (NOT the full em-dash FR18 string). The
//   production notice is wrapped in u.Yellow(...) ANSI escapes on a terminal; the substring is robust
//   to that. (TestRunDefault_AutoStageNotice_FR18 asserts the full verbatim string and passes because
//   the test captures a non-TTY stream where Yellow is a no-op, but the substring is the safer negative
//   assertion — and it is exactly what S1's in-line assertion uses, so stay consistent with it.)

// GOTCHA: On the clean-tree path, Execute returns a NON-NIL error (exitcode.NothingToCommit=2 is still
//   an error). Use `if err == nil { t.Fatal(...) }` then assert exitcode.For(err)==NothingToCommit.
//   Do NOT t.Fatalf on err != nil before checking the code — exit 2 is the EXPECTED non-zero outcome.

// GOTCHA: rootCmd is package-global and mutated by SetArgs/SetOut/SetErr across tests. ALWAYS wrap with
//   saveRootState(t)/defer restoreRootState(t,...) (every test in this file does). Omitting it causes
//   flaky cross-test bleed.
```

## Implementation Blueprint

### Data models and structure

None. Test-only; no types, models, or schemas change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/cmd/default_action_test.go — append the dedicated Issue 7 regression test
  - LOCATE: the end of TestRunDefault_AutoStageNotice_FR18 (its closing `}` is immediately before the
    `// ---...--- // TestRunDefault_Rescue — …` separator block). Insert the new test BETWEEN them,
    so the two auto-stage-notice tests (positive FR18 + negative Issue7) are adjacent.
  - INSERT (verbatim structure; mirror TestRunDefault_AutoStageNotice_FR18's harness exactly):

        // ---------------------------------------------------------------------------
        // TestRunDefault_CleanTreeNoAutoStageNotice_Issue7 — clean tree: NO "(0 files)" notice
        // ---------------------------------------------------------------------------

        // TestRunDefault_CleanTreeNoAutoStageNotice_Issue7 proves PRD Issue 7 is fixed end-to-end through
        // the CLI seam: on a fully-clean tree with the default auto_stage_all=true, the misleading
        // "Nothing staged — staging all changes (0 files)." notice is NOT printed, while the process
        // still exits 2 (NothingToCommit) with the "Nothing to commit." message and HEAD unchanged.
        // Before P1.M4.T2.S1 the "(0 files)" notice printed right before "Nothing to commit." (Issue 7).
        func TestRunDefault_CleanTreeNoAutoStageNotice_Issue7(t *testing.T) {
            origArgs, origOut, origErr, origRunE := saveRootState(t)
            defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

            // setupStubRepo COMMITS .stagecoach.toml, so with NO further writeFile/stageFile the tree is
            // fully clean — exactly the Issue 7 scenario (nothing to auto-stage).
            repo := setupStubRepo(t, "feat: x")

            var outBuf, errBuf bytes.Buffer
            rootCmd.SetOut(&outBuf)
            rootCmd.SetErr(&errBuf)
            rootCmd.SetArgs([]string{"--provider", "stub"})

            err := Execute(context.Background())
            if err == nil {
                t.Fatal("Execute err=nil, want error (nothing to commit on a clean tree)")
            }

            // Still exit 2 (NothingToCommit) — §15.4 semantics unchanged by the Issue 7 cosmetic fix.
            if code := exitcode.For(err); code != exitcode.NothingToCommit {
                t.Errorf("exitcode.For(err) = %d, want %d (NothingToCommit)", code, exitcode.NothingToCommit)
            }

            // The user-facing message still prints (main's err.Error() != "" guard fires).
            if msg := err.Error(); !strings.Contains(msg, "Nothing to commit.") {
                t.Errorf("err.Error() = %q, want to contain 'Nothing to commit.'", msg)
            }

            // Issue 7: the misleading "staging all changes (0 files)." notice must be ABSENT on a clean tree.
            stderr := errBuf.String()
            if strings.Contains(stderr, "staging all changes") {
                t.Errorf("stderr = %q, want NO auto-stage notice on a clean tree (Issue 7)", stderr)
            }

            // HEAD unchanged — last commit is still the config commit from setupStubRepo.
            if logMsg := gitOut(t, repo, "log", "--format=%s", "-n1"); logMsg != "init: add stagecoach config" {
                t.Errorf("HEAD moved to %q, want 'init: add stagecoach config'", logMsg)
            }
        }

  - FOLLOW pattern: TestRunDefault_AutoStageNotice_FR18 (harness) + TestRunDefault_NothingStaged_FR17
    (clean-tree setup: setupStubRepo with no arrange) + the doc-comment style of the other
    _Issue{1,3,5} tests ("proves PRD Issue N is fixed …").
  - NAMING: TestRunDefault_CleanTreeNoAutoStageNotice_Issue7 (matches the _Issue<N> convention; the
    name the S1 PRP pre-allocated under "ALTERNATIVE").
  - IMPORTS: none new — bytes, context, strings, testing, exitcode are all already imported in this file.
  - PLACEMENT: immediately after TestRunDefault_AutoStageNotice_FR18, before the TestRunDefault_Rescue
    separator. (Acceptable alternative: grouped with the other _Issue<N> tests at the file's tail, after
    TestRunDefault_RepoLocalNoticeOnce_Issue5 — either keeps it discoverable by name.)
  - DO NOT:
      * modify default_action.go or any production file (the fix is already shipped by S1).
      * remove or alter the in-line Issue 7 assertion S1 added to TestRunDefault_NothingStaged_FR17.
      * weaken TestRunDefault_AutoStageNotice_FR18 (n=2 notice must still be asserted present).
      * add writeFile/stageFile after setupStubRepo (that would dirty the tree and break the test).
```

### Implementation Patterns & Key Details

```go
// The new test is the NEGATIVE twin of TestRunDefault_AutoStageNotice_FR18:
//
//   FR18 (positive, dirty tree, n=2): stderr CONTAINS "Nothing staged — staging all changes (2 files)."
//   Issue7 (negative, clean tree, n=0): stderr does NOT CONTAIN "staging all changes"
//
// Why the early-return (S1, already shipped) makes this pass: in the cfg.AutoStageAll branch of runDefault,
// StagedFileCount returns 0 on a clean tree, and the `if n == 0 { return ...NothingToCommit... }` guard
// fires BEFORE the FR18 Fprintln — so stderr never receives the notice. The dedicated test LOCKS THAT IN.
//
// Why a DEDICATED test (vs. only S1's in-line assertion in …_FR17): discoverability + convention.
// Issues 1, 3, 5 each have a named TestRunDefault_*_IssueN; Issue 7 was the lone gap. A future maintainer
// grepping "_Issue7" or reading the _Issue<N> cluster now finds the explicit regression guard.

// Assertion ordering that avoids false failures:
//   err := Execute(ctx)                 // returns NON-NIL on the clean-tree exit-2 path
//   if err == nil { t.Fatal(...) }      // exit 2 is an error; nil would be a real bug
//   if exitcode.For(err) != exitcode.NothingToCommit { t.Errorf(...) }   // == 2
//   if !strings.Contains(err.Error(), "Nothing to commit.") { ... }     // message survives
//   if strings.Contains(stderr, "staging all changes") { t.Errorf(...) } // Issue 7 regression
//   if gitOut(...,"log","--format=%s","-n1") != "init: add stagecoach config" { ... } // HEAD pinned
```

### Integration Points

```yaml
DATABASE: none
CONFIG:   none (no config field added/removed/changed; auto_stage_all default true is exercised, not altered)
ROUTES:   none
NO new imports, no new files, no production-code edits, no schema/migration changes. Pure test addition.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet after adding the test (Go has no separate format/lint beyond these + golangci-lint)
go build ./...            # Expected: zero errors
go vet ./...              # Expected: zero errors
gofmt -l internal/cmd/default_action_test.go   # Expected: empty output (already formatted)
make lint                 # golangci-lint run — Expected: zero issues
```

### Level 2: Unit / Component Tests

```bash
# Run the NEW test in isolation (race detector on, matching `make test`)
go test -race ./internal/cmd/ -run 'RunDefault_CleanTreeNoAutoStageNotice_Issue7' -v
# Expected: PASS.

# Run the auto-stage cluster together to confirm no regression in the siblings (FR17 has S1's in-line
# Issue 7 assertion; FR18 is the positive n=2 notice; both must stay GREEN):
go test -race ./internal/cmd/ -run 'RunDefault_(NothingStaged_FR17|AutoStageNotice_FR18|CleanTreeNoAutoStageNotice_Issue7)' -v
# Expected: all three PASS.

# Run the full _Issue<N> regression cluster (consistency check across Issues 1, 3, 5, 7):
go test -race ./internal/cmd/ -run '_Issue[0-9]' -v
# Expected: all PASS.

# Full suite (matches `make test`)
go test -race ./...
# Expected: all packages pass.
```

### Level 3: Integration / Manual Sanity (confirm the scenario end-to-end via the binary)

```bash
# Build the binary
make build    # → ./bin/stagecoach

# Reproduce the Issue 7 scenario on a clean tree via the real binary (positive proof the fix + test align):
cd "$(mktemp -d)" && git init -q && git config user.email t@t && git config user.name t \
  && git commit -q --allow-empty -m "init"
# Clean tree. Run with any provider (auto_stage_all default = true):
./bin/stagecoach --provider stub 2>err.txt ; echo "exit=$?"
# AFTER the fix: err.txt contains ONLY "Nothing to commit." (NO "staging all changes (0 files).").
grep -c "staging all changes" err.txt   # Expected: 0  (the property the new test locks in)
grep -c "Nothing to commit"    err.txt   # Expected: >=1
# exit=2. HEAD unchanged: git log --format=%s -n1 == "init".

# Positive control (the n>0 notice still prints verbatim on a dirty tree — guards against over-fixing):
echo x > a.txt && ./bin/stagecoach --provider stub 2>err2.txt
grep -c "staging all changes (1 files)" err2.txt   # Expected: 1  (FR18 notice survives for n>0)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Confirm the test FAILS against the pre-fix code (true regression value — it must catch Issue 7
# coming back). Temporarily revert ONLY the S1 production guard and re-run the new test:
git stash         # optional, if working tree is dirty
git checkout 8fdcd29~1 -- internal/cmd/default_action.go   # remove the `if n == 0` early-return
go test -race ./internal/cmd/ -run 'RunDefault_CleanTreeNoAutoStageNotice_Issue7' -v
# Expected: FAIL (stderr now contains "staging all changes (0 files)." → the t.Errorf fires).
# Then RESTORE the fix:
git checkout 8fdcd29 -- internal/cmd/default_action.go
git stash pop    # if you stashed
go test -race ./internal/cmd/ -run 'RunDefault_CleanTreeNoAutoStageNotice_Issue7' -v
# Expected: PASS again. (This proves the test genuinely guards Issue 7, not a tautology.)

# Coverage gate (PRD §20.3) is unaffected (internal/cmd is not in the 85%-gate package set), but run:
make coverage    # informational; Expected: no regression in internal/{git,provider,generate,config}
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l internal/cmd/default_action_test.go` is empty
- [ ] `make lint` (golangci-lint) passes
- [ ] `go test -race ./...` passes (full suite)

### Feature Validation

- [ ] `TestRunDefault_CleanTreeNoAutoStageNotice_Issue7` exists with `_Issue7` suffix + doc comment
- [ ] It asserts exit code == `exitcode.NothingToCommit` (2)
- [ ] It asserts `err.Error()` contains "Nothing to commit."
- [ ] It asserts stderr does NOT contain "staging all changes"
- [ ] It asserts HEAD unchanged (last commit == "init: add stagecoach config")
- [ ] `TestRunDefault_AutoStageNotice_FR18` (n=2 notice) stays GREEN + unchanged
- [ ] `TestRunDefault_NothingStaged_FR17` stays GREEN (S1's in-line assertion left in place)
- [ ] Level 4 negative-control: reverting S1's `if n == 0` guard makes the new test FAIL (true guard)

### Code Quality Validation

- [ ] Follows the existing `TestRunDefault_*_IssueN` naming + doc-comment convention (Issues 1, 3, 5)
- [ ] Mirrors the standard `saveRootState`/`restoreRootState` + `setupStubRepo` + `Execute(ctx)` harness
- [ ] No production files modified (test-only, per contract DOCS: none)
- [ ] Anti-patterns avoided: no duplicated in-line assertion in …_FR17, no dirty-tree arrange, no
      t.Fatalf before the exit-code check, no assertion of the full em-dash string in the negative case

### Documentation & Deployment

- [ ] No doc changes required (the FR18 template is unchanged for n>0; pure regression test — see
      work-item DOCS: none). The test's doc comment is the only documentation surface.

---

## Anti-Patterns to Avoid

- ❌ Don't re-add the in-line `strings.Contains(stderr, "staging all changes")` assertion to
  `TestRunDefault_NothingStaged_FR17` — S1 (commit 8fdcd29) already put it there. That is a no-op;
  S2's deliverable is the DEDICATED test (the contract's "or add a dedicated test" alternative).
- ❌ Don't modify `internal/cmd/default_action.go` or any production file — the `if n == 0` fix is
  already shipped by S1. S2 is test-only.
- ❌ Don't add `writeFile`/`stageFile` after `setupStubRepo` — that dirties the tree, makes n>0, and
  causes the notice to (correctly) print, failing the negative assertion.
- ❌ Don't `t.Fatalf` on `err != nil` before checking `exitcode.For(err)` — exit 2 (NothingToCommit)
  is an expected NON-NIL error.
- ❌ Don't assert the full em-dash string "Nothing staged — staging all changes …" in the negative
  case — use the ASCII substring "staging all changes" (robust to ANSI Yellow wrapping; matches S1's
  in-line assertion).
- ❌ Don't weaken or alter `TestRunDefault_AutoStageNotice_FR18` — the n=2 positive notice must remain
  asserted present (it guards against the fix over-reaching into the n>0 path).
- ❌ Don't introduce a new test file or new imports — extend `default_action_test.go`; `bytes`,
  `context`, `strings`, `testing`, and `exitcode` are already imported.

---

## Confidence Score: 9/10

One-pass success is highly likely: this is a single new test function that mirrors an existing,
passing test (`TestRunDefault_AutoStageNotice_FR18`) using helpers (`setupStubRepo`,
`saveRootState`, `Execute`, `exitcode.For`, `gitOut`) already proven throughout the file. The
production fix it guards (S1's `if n == 0` early-return) is already shipped and the FR17/FR18
siblings are already green, so the only remaining work is the named, convention-consistent
`_Issue7` test. The single residual risk is a reviewer mistaking this for a duplicate of S1's
in-line assertion — which this PRP addresses head-on in the Goal/Context/Gotchas sections.
