# PRP — P1.M2.T1.S2: Tests — missing provider command fails fast (exit 1, not `*RescueError`, no dangling tree)

> **Scope discipline.** This subtask is **TEST-ONLY**. The production fix (the `reg.IsInstalled(m)`
> pre-flight check in `buildDeps`) was implemented in **P1.M2.T1.S1** (already merged — see the
> `Pre-flight (PRD §18.2)` block in `pkg/stagecoach/stagecoach.go` `buildDeps`, after `m.Validate()`).
> S2 writes the **regression tests that lock in** Issue 3's fix and would fail loudly if anyone
> reverts or weakens that check. **Do NOT modify any source file, the pre-flight check itself,
> `PRD.md`, or any `tasks.json`.** Output = two new test functions (+ one tiny helper each).

---

## Goal

**Feature Goal**: Prove, via green regression tests, that a provider whose `command` is not on
`$PATH` fails **fast** (exit **1**) in `buildDeps` **before** the `write-tree` snapshot — so the
error is **not** a `*RescueError`, `exitcode.For` maps it to **1**, the message names the missing
command, and **no new tree object** is written (no dangling tree, no §18.3 rescue block).

**Deliverable**:
1. `pkg/stagecoach/stagecoach_test.go` — `TestGenerateCommit_MissingProviderCommand_Issue3` driving the
   **public library seam** (`GenerateCommit` directly) with the full assertion set (a)–(d), plus the
   `objectCountLine` helper.
2. `internal/cmd/default_action_test.go` — `TestRunDefault_MissingProviderCommand_Issue3` driving the
   **full CLI seam** (`rootCmd.SetArgs` + `Execute`) asserting exit 1 + no rescue block on stderr +
   no dangling tree, plus the `objectCountLine` helper.

**Success Definition**:
- Both new tests PASS under `go test -race ./...`, and the entire existing suite stays green.
- A thought-experiment (or a temporary revert of the S1 pre-flight block) makes the count-objects
  guard and the `*RescueError`-negation guard **fail** — i.e. the tests genuinely guard the fix.

---

## Why

- **PRD Issue 3 / §18.2 failure table / §13.5**: a missing agent command must be a **pre-generation,
  exit-1** failure, never a post-snapshot exit-3 rescue. The only existing missing-command test
  (`internal/provider/executor_test.go:128-137` `TestExecute_CommandNotFound`) covers `Execute` in
  **isolation** — it does not prove the *pipeline* fails fast, does not assert the error type /
  exit code, and does not assert no dangling tree is left.
- **Regression value**: Issue 3's root cause is ordering-sensitive (snapshot taken before the command
  is probed). A future refactor that moves/rescopes `buildDeps`, or drops the `IsInstalled` call,
  would silently re-introduce exit-3 + a dangling tree. These tests make that impossible to miss.
- **Both seams**: Issue 1's fix (P1.M1) wired the CLI's loaded config through `Options.Config` into
  `GenerateCommit`; Issue 3's fix lives in `buildDeps` (the single chokepoint feeding **both**
  `CommitStaged` and the dry-run `runPipeline`). Testing the library seam proves the chokepoint;
  testing the CLI seam proves the end-to-end user journey (`stagecoach --provider missing`).

---

## What

Two new Go test functions (one per package) that register a `[provider.missing]` whose `command` is
an absolute path that does not exist (`/nonexistent/path/agent`), stage a **new** file, run the
seam, and assert Issue 3's contract:

| Clause | Assertion | How |
|---|---|---|
| (a) not rescue | `errors.As(err, &re)` (`re *RescueError`) is **false** | library test |
| (b) exit 1 | `exitcode.For(err) == exitcode.Error` (1) | both tests |
| (c) message | `err.Error()` contains `not found` AND `Is the agent installed?` AND the command path | both tests |
| (d) no dangling tree | `git count-objects -v` `count:` line **unchanged** before/after the run | both tests |

The CLI test additionally asserts the **§18.3 rescue block is absent** from stderr (no
`❌ Commit generation failed.`, no `Tree ID:`).

### Success Criteria

- [ ] `TestGenerateCommit_MissingProviderCommand_Issue3` (pkg/stagecoach) passes; asserts (a)–(d).
- [ ] `TestRunDefault_MissingProviderCommand_Issue3` (internal/cmd) passes; asserts (b)–(d) + no
      rescue block on stderr.
- [ ] Both tests stage a **new** file (so the count-objects guard is a real regression catch — see
      Gotchas).
- [ ] `go test -race ./pkg/stagecoach/... ./internal/cmd/...` green; `go test -race ./...` green.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact two files, the exact test-function skeletons (copy-paste-ready),
the helpers to reuse, the helper to add, the proven detection method, the error-flow rationale, and
the runnable validation commands are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root-cause trace; section 6 is the test gap this PRP fills)
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_provider_preflight.md
  why: Section 6 proves ONLY an Execute-level test exists (no GenerateCommit/CLI test for the
       missing-command path); section 5 proves exitcode.For maps *RescueError->3 / plain->1;
       section 7 is the buildDeps insertion point (already implemented in S1).
  critical: The pre-flight error MUST stay a plain fmt.Errorf (NOT *RescueError) — that is what the
            (a)/(b) assertions depend on. Do NOT change the error shape.

# The S1 fix under test (READ-ONLY — do not modify; only assert against its behavior)
- file: pkg/stagecoach/stagecoach.go
  why: buildDeps (search "Pre-flight (PRD §18.2)") — the reg.IsInstalled(m) check + the exact error
       string `provider %q: command %q not found. Is the agent installed?` that clause (c) matches.
  pattern: GenerateCommit calls resolveConfig -> buildDeps -> (CommitStaged|runPipeline). buildDeps
           returns BEFORE WriteTree, so no object is written — that is what clause (d) verifies.
  gotcha: The check fires for BOTH the commit path AND dry-run (single chokepoint). A dry-run
          subtest is optional but valuable to lock in both-pipeline protection.

# PRIMARY PATTERN — the library test to extend (copy its fixture helpers + assertion style)
- file: pkg/stagecoach/stagecoach_test.go
  why: Reuse setupTestRepo's chdir/.stagecoach.toml pattern and the errors.Is/exitcode assertion
       style already used by TestGenerateCommit_Timeout / TestResolveConfig_InjectedConfig.
  pattern: setupTestRepo(t, stubtest.Options{...}) builds the stub + writes [provider.stub]. For the
           missing-command test you do NOT need the stub — write a focused [provider.missing] TOML
           inline (the command does not exist, so no binary is built/executed).
  gotcha: This file does NOT currently import internal/exitcode — ADD the import for exitcode.For.
          RescueError is already in scope (type alias of generate.RescueError) — use `var re *RescueError`.

# PRIMARY PATTERN — the CLI test to extend (copy saveRootState/restoreRootState + Execute seam)
- file: internal/cmd/default_action_test.go
  why: Mirror TestRunDefault_Rescue (exit 3) / TestRunDefault_CAS (exit 1) — same saveRootState
       bracket, rootCmd.SetArgs, Execute(ctx), exitcode.For(err), stderr-buffer rescue-block checks.
  pattern: setupStubRepoRaw(t, tomlBody) writes a raw .stagecoach.toml, initRepo, chdir — NO stub
           build needed; perfect for a [provider.missing] config.
  gotcha: setupStubRepoRaw does NOT create an initial commit — add `runGit add .stagecoach.toml` +
          `runGit commit -m initial` so HEAD exists (and so the new-file count guard is meaningful).

# Exit-code mapping (why clause b holds for a plain error)
- file: internal/exitcode/exitcode.go
  why: For() order: nil->0; *ExitError->Code; NothingToCommit->2; Timeout->124; Rescue->3; CAS->1;
       else `return Error`. A plain missing-command error and the CLI's *ExitError{Code:1} both -> 1.
  gotcha: Do NOT %w-wrap any generate.* sentinel in the (read-only) pre-flight error — it already
          doesn't, which is why exitcode.For returns 1, not 3/124. The tests assert this end-state.

# Proven detection method (clause d) — empirically verified in research/
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M2T1S2/research/dangling_tree_detection.md
  why: Proves `git count-objects -v` count-line increments (3->5) when write-tree writes a NEW tree,
       and is unchanged when write-tree is skipped. Documents why a NEW staged file is required and
       why git fsck / cat-file -t are NOT usable here.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go            # buildDeps: the S1 pre-flight (READ-ONLY, under test)
pkg/stagecoach/stagecoach_test.go       # ADD: TestGenerateCommit_MissingProviderCommand_Issue3 + objectCountLine
internal/cmd/default_action_test.go   # ADD: TestRunDefault_MissingProviderCommand_Issue3 + objectCountLine
internal/cmd/root_test.go             # REUSE helpers: initRepo, writeConfigFile, chdir, saveRootState, restoreRootState, runGit, gitOut
internal/exitcode/exitcode.go         # REUSE: exitcode.For, exitcode.Error
internal/stubtest/stubtest.go         # NOT needed for these tests (missing command = no stub build)
internal/provider/executor_test.go    # EXISTING Execute-level test (TestExecute_CommandNotFound) — leave as-is
```

### Desired Codebase tree (files ADDED-TO; no new files)

```bash
pkg/stagecoach/stagecoach_test.go       # +1 test func +1 helper (+ import internal/exitcode)
internal/cmd/default_action_test.go   # +1 test func +1 helper (no new import; strings/gitOut already present)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (regression-guard correctness): the test MUST stage a NEW file before the run. With
//   nothing staged, CommitStaged/runPipeline short-circuit to ErrNothingToCommit BEFORE WriteTree,
//   so `git count-objects` would be unchanged EVEN IN THE BUGGY BUILD — silently masking a
//   regression. Staging new.txt guarantees that, if the pre-flight check were removed, WriteTree
//   would write a NEW tree object and the count guard would fire. (Verified in research/ note.)

// CRITICAL (clause a): assert errors.As(err, &re) is FALSE using the IN-PACKAGE type alias.
//   In pkg/stagecoach the alias is `RescueError` (stagecoach.go: `type RescueError = generate.RescueError`),
//   so `var re *RescueError; errors.As(err, &re)` works without importing generate.
//   In internal/cmd there is no alias — use `var re *generate.RescueError` (generate is imported
//   transitively via exitcode; add the import if the linter requires it) OR rely solely on
//   exitcode.For(err)!=Rescue + stderr-has-no-rescue-block (sufficient for the CLI seam).

// GOTCHA (test helper portability): _test.go helpers are NOT importable across packages, so
//   objectCountLine/runGit/gitOut/initRepo are DUPLICATED per package (the repo already does this —
//   see the "copied from internal/generate/generate_test.go" comments). Add objectCountLine to BOTH
//   test files; do not try to share it.

// GOTCHA (chdir): GenerateCommit resolves the repo via os.Getwd(). Both seams already chdir into a
//   t.TempDir() and register a t.Cleanup to restore CWD — reuse that exact pattern
//   (setupTestRepo / setupStubRepoRaw + chdir already do it).

// GOTCHA (command path): use an ABSOLUTE non-existent path ("/nonexistent/path/agent"). exec.LookPath
//   on a missing absolute path returns an error on every platform -> IsInstalled=false. A relative
//   name also works (see TestExecute_CommandNotFound) but the absolute path matches the bug repro
//   and the S1 error message verbatim.

// GOTCHA (no stub needed): unlike the happy-path tests, these tests do NOT call stubtest.Build —
//   the whole point is the command does not exist. This makes the tests fast and self-contained.
```

---

## Implementation Blueprint

### The shared helper (add to BOTH test files)

```go
// objectCountLine returns the "count:" line of `git count-objects -v` for the repo at dir.
// Used to assert no NEW loose objects (no dangling tree) were written by a failed run.
// count-objects is the clean signal here: git fsck false-positives on staged-but-uncommitted blobs,
// and cat-file -t needs a SHA the failed run never produced. (see research/dangling_tree_detection.md)
func objectCountLine(t *testing.T, dir string) string {
	t.Helper()
	for _, line := range strings.Split(gitOut(t, dir, "count-objects", "-v"), "\n") {
		if strings.HasPrefix(line, "count:") {
			return line
		}
	}
	t.Fatalf("git count-objects -v: no 'count:' line in output:\n%s", gitOut(t, dir, "count-objects", "-v"))
	return ""
}
```
> `gitOut` (pkg/stagecoach) / `gitOut` (internal/cmd) already exist in both files. `strings` is already
> imported in both. No new imports for the helper.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY pkg/stagecoach/stagecoach_test.go  (add import + helper + test)
  - ADD IMPORT: "github.com/dustin/stagecoach/internal/exitcode"  (NOT currently imported; needed for
    exitcode.For / exitcode.Error). RescueError is already an in-package type alias — no import needed.
  - ADD HELPER: objectCountLine(t, dir) string  (code above). Reuses the file's existing gitOut helper.
  - ADD TEST: TestGenerateCommit_MissingProviderCommand_Issue3  (skeleton below).
  - REGISTRATION: repo-local .stagecoach.toml with [provider.missing] command="/nonexistent/path/agent"
    (faithful to the bug repro + the contract wording "via repo-local .stagecoach.toml"). Write it into
    a t.TempDir() repo, initRepo + commitRaw("initial"), chdir (t.Cleanup restore).
  - FIXTURE: writeFile new.txt + stageFile new.txt  (MUST stage a NEW file — see Gotchas).
  - ASSERTIONS (a)-(d): errors.As(*RescueError)==false; exitcode.For==exitcode.Error; err.Error()
    contains "not found" + "Is the agent installed?" + "/nonexistent/path/agent"; objectCountLine equal
    before/after.
  - OPTIONAL SUBTEST: t.Run("dryrun", ...) with Options{Provider:"missing", DryRun:true} asserting the
    SAME (a)-(d) — locks in that buildDeps protects the runPipeline path too (decisions.md D2). Low
    cost, high value; recommended.
  - PLACEMENT: after TestGenerateCommit_SystemExtra / alongside TestGenerateCommit_Timeout.
  - NAMING: TestGenerateCommit_MissingProviderCommand_Issue3 (matches TestRunDefault_*_IssueN style
    used by P1.M1.T1.S3).

Task 2: MODIFY internal/cmd/default_action_test.go  (add helper + test)
  - ADD HELPER: objectCountLine(t, dir) string  (identical body; package-private duplication is the
    repo convention). Uses existing gitOut; strings already imported. NO new import.
  - ADD TEST: TestRunDefault_MissingProviderCommand_Issue3  (skeleton below).
  - FIXTURE: setupStubRepoRaw(t, toml) where toml = the [provider.missing] block; then runGit add
    .stagecoach.toml + runGit commit -m initial (setupStubRepoRaw does NOT commit); writeFile + stageFile
    a NEW new.txt.
  - SEAM: saveRootState/restoreRootState bracket; rootCmd.SetOut/SetErr buffers; rootCmd.SetArgs(
    []string{"--provider", "missing"}); err := Execute(context.Background()).
  - ASSERTIONS: exitcode.For(err)==exitcode.Error; err.Error() contains "not found" + "Is the agent
    installed?"; stderr buffer contains NEITHER "❌ Commit generation failed." NOR "Tree ID:" (no §18.3
    rescue block); objectCountLine equal before/after.
  - PLACEMENT: alongside TestRunDefault_Rescue / TestRunDefault_CAS (the other exit-code-path tests).
  - NAMING: TestRunDefault_MissingProviderCommand_Issue3.
  - GUARDRAIL: do NOT modify default_action.go, root.go, stagecoach.go, exitcode.go, or any non-test file.
```

### Test skeletons (copy-paste-ready)

**Task 1 — `pkg/stagecoach/stagecoach_test.go`:**

```go
// TestGenerateCommit_MissingProviderCommand_Issue3 proves PRD Issue 3 is fixed: a provider whose
// command is not on $PATH fails FAST (exit 1) in buildDeps BEFORE the write-tree snapshot — so the
// error is NOT a *RescueError, exitcode.For maps it to 1, the message names the missing command, and
// NO new tree object is written. Before P1.M2.T1.S1 this surfaced as exit 3 (rescue) + a dangling tree.
func TestGenerateCommit_MissingProviderCommand_Issue3(t *testing.T) {
	// Fresh repo with a repo-local .stagecoach.toml registering a provider whose command does not exist.
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	const toml = "[provider.missing]\n" +
		"command = \"/nonexistent/path/agent\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = true\n"
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write .stagecoach.toml: %v", err)
	}

	// Chdir (GenerateCommit resolves the repo via os.Getwd()).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	// MUST stage a NEW file: if the pre-flight check were removed (regression), WriteTree would write
	// a new tree object and the count-objects guard below would catch it. With nothing staged, the
	// pipeline short-circuits to ErrNothingToCommit before WriteTree, masking the regression.
	writeFile(t, repo, "new.txt", "content")
	stageFile(t, repo, "new.txt")

	beforeCount := objectCountLine(t, repo)

	_, err = GenerateCommit(context.Background(), Options{Provider: "missing"})

	afterCount := objectCountLine(t, repo)

	// Must error.
	if err == nil {
		t.Fatal("GenerateCommit: err = nil, want non-nil (missing provider command)")
	}
	// (a) NOT a *RescueError (the bug returned *RescueError -> exit 3 + rescue block + dangling tree).
	var re *RescueError
	if errors.As(err, &re) {
		t.Errorf("error is *RescueError (%v); want a plain pre-generation error (exit 1)", re)
	}
	// (b) exitcode.For(err) == 1. A plain error falls through to `return Error`.
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error); err=%v", code, exitcode.Error, err)
	}
	// (c) message names the missing command.
	if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") || !strings.Contains(msg, "/nonexistent/path/agent") {
		t.Errorf("err.Error() = %q; want to contain 'not found', 'Is the agent installed?', and '/nonexistent/path/agent'", msg)
	}
	// (d) NO new tree object written (pre-flight ran before WriteTree).
	if beforeCount != afterCount {
		t.Errorf("dangling tree: git count-objects changed\n  before: %s\n  after:  %s\n(pre-flight must run before WriteTree)", beforeCount, afterCount)
	}
}
```

**Task 2 — `internal/cmd/default_action_test.go`:**

```go
// TestRunDefault_MissingProviderCommand_Issue3 proves PRD Issue 3 is fixed end-to-end through the CLI
// seam: `stagecoach --provider <missing-command>` exits 1 with the not-found message and NO §18.3
// rescue block / no dangling tree. Before P1.M2.T1.S1 this was exit 3 + rescue block + dangling tree.
func TestRunDefault_MissingProviderCommand_Issue3(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	toml := "[provider.missing]\n" +
		"command = \"/nonexistent/path/agent\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = true\n"
	repo := setupStubRepoRaw(t, toml)
	// setupStubRepoRaw does not commit; add an initial commit so HEAD exists and the new-file
	// count-objects guard is meaningful.
	runGit(t, repo, "add", ".stagecoach.toml")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, "new.txt", "content") // NEW file — see pkg/stagecoach test comment for why
	stageFile(t, repo, "new.txt")

	beforeCount := objectCountLine(t, repo)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "missing"})

	err := Execute(context.Background())

	afterCount := objectCountLine(t, repo)

	if err == nil {
		t.Fatal("Execute: err = nil, want non-nil (missing provider command)")
	}
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") {
		t.Errorf("err.Error() = %q; want to contain 'not found' and 'Is the agent installed?'", msg)
	}
	// NO §18.3 rescue block on stderr (the bug printed "❌ Commit generation failed." + "Tree ID:").
	stderr := errBuf.String()
	if strings.Contains(stderr, "❌ Commit generation failed.") {
		t.Errorf("stderr contains the rescue block (want NONE for a missing command):\n%s", stderr)
	}
	if strings.Contains(stderr, "Tree ID:") {
		t.Errorf("stderr contains 'Tree ID:' rescue recipe (want NONE):\n%s", stderr)
	}
	// NO dangling tree object.
	if beforeCount != afterCount {
		t.Errorf("dangling tree: git count-objects changed\n  before: %s\n  after:  %s", beforeCount, afterCount)
	}
}
```

### Implementation Patterns & Key Details

```go
// PATTERN (mirror the repo's existing Issue-N regression tests): name *_Issue3; add a header comment
// stating the before/after behavior; assert BOTH the positive (exit 1, message) AND the negative
// (NOT *RescueError, NO rescue block, NO dangling tree). Compare TestRunDefault_Rescue (exit 3) and
// TestRunDefault_CAS (exit 1) in the same file.

// PATTERN (CLI seam): always bracket with saveRootState/restoreRootState and call Execute(ctx), NOT
// rootCmd.Execute() directly — Execute is the package entry that wires PersistentPreRunE. See every
// TestRunDefault_* test.

// WHY count-objects (not fsck/cat-file): see research/dangling_tree_detection.md. fsck false-positives
// on staged blobs; cat-file -t needs a SHA the failed run never produced. count-objects count-line is
// the deterministic, proven signal (3->5 when write-tree writes a new tree; unchanged when skipped).

// WHY stage a NEW file: makes the count guard a TRUE regression catch. With nothing staged the guard
// would pass even in the buggy build (ErrNothingToCommit short-circuits before WriteTree).
```

### Integration Points

```yaml
CODE: none — test-only. No source file, import (beyond adding internal/exitcode to the pkg/stagecoach
      test), export, config, route, or signal change. The S1 pre-flight check is UNCHANGED.
DATABASE: none.
CONFIG: tests write a throwaway repo-local .stagecoach.toml inside t.TempDir() (auto-cleaned); no
        committed config change.
ROUTES: none.
```

---

## Validation Loop

### Level 1: Syntax & Style (run after each edit)

```bash
# Build + vet the two affected packages. Expected: clean.
go build ./...
go vet ./pkg/stagecoach/... ./internal/cmd/...

# Format check (gofmt is the repo formatter — .golangci.yml / Makefile use it).
gofmt -l pkg/stagecoach/stagecoach_test.go internal/cmd/default_action_test.go
# Expected: lists nothing. If it does: gofmt -w <those files>.
```

### Level 2: The new tests (component validation)

```bash
# Run the two new tests verbosely, with -race.
go test -race -run 'TestGenerateCommit_MissingProviderCommand_Issue3' ./pkg/stagecoach/ -v
go test -race -run 'TestRunDefault_MissingProviderCommand_Issue3'    ./internal/cmd/   -v
# Expected: both PASS. If a clause fails, READ the assertion message — it names exactly which
# invariant (type/exit-code/message/no-dangling-tree) broke.
```

### Level 3: Regression guard (prove the tests actually guard the fix)

```bash
# Sanity check the guard fires if the fix is removed (do this on a THROWAWAY branch / git stash after):
#   temporarily comment out the `if !reg.IsInstalled(m) { ... }` block in pkg/stagecoach/stagecoach.go
#   buildDeps, re-run the two tests — they MUST now FAIL (clause a: *RescueError; clause d: count
#   changed; CLI: rescue block on stderr). Then `git checkout pkg/stagecoach/stagecoach.go` to restore.
# This step is OPTIONAL verification of test quality, not a CI gate.
```

### Level 4: Full suite (no regressions elsewhere)

```bash
# The change is test-only; every previously-green test must stay green.
go test -race ./...
# Expected: all PASS (incl. the existing TestExecute_CommandNotFound and all happy-path/rescue/
# timeout/CAS/dry-run/stage-while-generating tests — none of their providers are missing, so the
# S1 pre-flight stays a no-op for them).
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./pkg/stagecoach/... ./internal/cmd/...` clean.
- [ ] `gofmt -l` reports nothing for the two edited test files.
- [ ] `go test -race -run '...MissingProviderCommand_Issue3' ./pkg/stagecoach/ ./internal/cmd/` green.
- [ ] `go test -race ./...` — entire suite green (no regressions).

### Feature Validation (Issue 3 contract)
- [ ] (a) library test: `errors.As(err, &re)` (*RescueError) is **false**.
- [ ] (b) both tests: `exitcode.For(err) == exitcode.Error` (1).
- [ ] (c) both tests: `err.Error()` contains `not found` + `Is the agent installed?` (+ command path).
- [ ] (d) both tests: `git count-objects -v` count-line **unchanged** before/after (no dangling tree).
- [ ] CLI test: stderr contains **no** `❌ Commit generation failed.` and **no** `Tree ID:`.
- [ ] Both tests stage a NEW file (regression-guard correctness).
- [ ] (Optional) dry-run subtest passes the same (a)–(d) — locks in both-pipeline protection.

### Code Quality Validation
- [ ] Test naming matches `*_Issue3` convention (consistent with P1.M1.T1.S3's `*_Issue1`/`*_Issue5`).
- [ ] `objectCountLine` added to both test files (helper duplication is the repo convention).
- [ ] Reuses existing helpers (runGit/gitOut/initRepo/commitRaw/writeFile/stageFile/setupStubRepoRaw/
      saveRootState/restoreRootState) — no reimplemented git plumbing.
- [ ] No source file modified (test-only); `internal/exitcode` import added only to the pkg/stagecoach test.

### Documentation
- [ ] Each test has a header comment stating the before/after behavior it locks in.
- [ ] `research/dangling_tree_detection.md` records the proven detection method (already written).

---

## Anti-Patterns to Avoid

- ❌ **Don't modify the S1 pre-flight check or any source file** — S2 is test-only. The fix already
  exists; you are only locking it in.
- ❌ **Don't skip staging a NEW file** — without it the count-objects guard passes even in the buggy
  build (ErrNothingToCommit short-circuits before WriteTree), making the test worthless as a guard.
- ❌ **Don't use `git fsck` / `git cat-file -t <sha>` for the dangling-tree assertion** — fsck
  false-positives on staged blobs; cat-file needs a SHA the failed run never produced. Use
  `git count-objects -v` count-line.
- ❌ **Don't assert the error IS a sentinel** (`errors.Is(err, ErrRescue)`) — the whole point is that
  it is a PLAIN error. Assert `errors.As(*RescueError)` is false and `exitcode.For == Error`.
- ❌ **Don't build/register the stub for these tests** — the missing command never runs; `stubtest.Build`
  is unnecessary and would muddy the "command not found" intent.
- ❌ **Don't share `objectCountLine` across packages** — `_test.go` helpers aren't importable; duplicate
  it (the repo already duplicates runGit/initRepo/etc this way).
- ❌ **Don't hardcode the count integer** — compare the `count:` LINE string before vs after (robust to
  the repo's starting object count and to git housekeeping).
- ❌ **Don't forget `t.Cleanup(os.Chdir)` / `restoreRootState`** — both seams mutate global state (CWD,
  rootCmd flags); every other test in these files brackets with cleanup; follow suit.

---

## Confidence Score

**9/10** — This is a tightly-bounded, test-only task: the fix under test is already merged and its
exact error string is known; the two target files, the helpers to reuse, copy-paste-ready test
skeletons, the proven (empirically-verified) dangling-tree detection method, and the runnable
`go test` validation commands are all specified. The -1 reserves for the optional dry-run subtest
judgment call and the cross-platform absolute-path consideration (low-risk; an absolute
non-existent path fails `exec.LookPath` on every Go-supported OS).
