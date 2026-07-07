---
name: "P1.M1.T1.S1 — Change prepare-commit-msg argv from 2 args to 1 arg; correct false verification comments"
description: |
  Git-parity bugfix (Issue 1). `runPrepareCommitMsg` (runner.go:195) passes `[]string{msgPath, ""}` (argc=2),
  but git's githooks(5) says for a plain commit "no second parameter is passed" — argc=1, $2 unset. Git 2.54.0
  empirically confirms ARGC=1. Fix: change the argv to `[]string{msgPath}`. Correct the two false "VERIFIED
  argc=2" comments (lines 52, 178) to cite githooks(5) + argc=1. Add a TDD test asserting $#==1. No caller
  affected; signature unchanged.
---

## Goal

**Feature Goal**: Restore git-parity for the `prepare-commit-msg` hook invocation: pass argc=1 (the message
file only), matching git's documented behavior for a plain commit — not argc=2 with an empty-string `$2`.

**Deliverable** (1 one-line production fix + 2 comment corrections + 1 new test):
1. `internal/hooks/runner.go:195`: change `[]string{msgPath, ""}` → `[]string{msgPath}`.
2. `internal/hooks/runner.go:52`: correct the comment from "VERIFIED argc=2" to "argc=1, $2 unset (githooks(5))".
3. `internal/hooks/runner.go:178`: same comment correction.
4. `internal/hooks/runner_test.go`: add `TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne` — installs a hook
   that records `$#` to a file, calls RunCommitHooks, asserts the file contains `ARGC=1`.

**Success Definition**: A `prepare-commit-msg` hook sees exactly 1 argument (the message file path);
`$#` is 1; `$2` is unset. The two false verification comments are corrected. The new test passes.
`go build/vet/gofmt` clean; `go test -race ./...` green.

## Why

- **Git-parity guarantee.** Git's githooks(5) documentation specifies that for a plain commit
  `prepare-commit-msg <msgfile>` is invoked with ONE argument — the `source` parameter is ABSENT. Git
  2.54.0 empirically confirms `ARGC=1`. Stagehand currently passes 2 args (the empty string `""` as $2).
- **Hooks that branch on `$#` misbehave.** A hook using `[ "$#" -eq 1 ] && …` takes the wrong branch.
  Most common hooks (husky, commitlint) use `[ -z "$2" ]` which works either way, so the practical blast
  radius is narrow — but it is a documented git-parity violation.
- **The in-source verification claim is FALSE.** The comment at runner.go:52 claims "VERIFIED argc=2 for
  a plain commit — see external_deps.md §2". `external_deps.md` does NOT exist. The verification never
  happened. Correcting the comment prevents future maintainers from trusting the false claim.

## What

A one-token production change (drop `""` from the argv slice) + two comment corrections + one TDD test.
No signature change, no caller change.

### Success Criteria

- [ ] `runPrepareCommitMsg` invokes the hook with `[]string{msgPath}` (1 arg).
- [ ] A hook that records `$#` sees `ARGC=1`.
- [ ] The comment at line 52 cites githooks(5) + argc=1 (not "VERIFIED argc=2 / external_deps.md §2").
- [ ] The comment at line 178 cites the same.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Documentation & References

```yaml
- file: internal/hooks/runner.go
  why: "EDIT (3 spots). (a) Line ~195: []string{msgPath, \"\"} → []string{msgPath}. (b) Line ~52: comment 'Invoked as `<msgfile> \"\"` (PRD FR-V2; VERIFIED argc=2 for a plain commit — see external_deps.md §2)' → 'Invoked as `<msgfile>` (git githooks(5): for a plain commit no second parameter is passed; argc=1, $2 unset)'. (c) Line ~178: same correction in the runPrepareCommitMsg doc comment."
  pattern: "The runHook call: `runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)`. Drop the empty-string second element. That's the entire production change."
  gotcha: "Do NOT change the function signature, the runHook call's other args, or the return contract. Only the argv slice changes."

- file: internal/hooks/runner_test.go
  why: "EDIT (1 new test). Add TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne. Reuse primeRunnerRepo + installHook + defaultCfg. Mirror TestRunCommitHooks_NoVerify_SkipsPreCommitAndCommitMsg_PrepareRuns (:228) for the setup/assert structure."
  pattern: "installHook(t, repo, 'prepare-commit-msg', `echo ARGC=$# > /path/to/file`) → call RunCommitHooks → read the file → assert 'ARGC=1'. The hook body is a shell script (`#!/bin/sh\necho ARGC=$# > /path\n`)."
  gotcha: "Write the argc file INSIDE the repo dir (t.TempDir-scoped) so the test is hermetic. Use filepath.Join(repo, 'argc.txt')."

- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/architecture/git_hook_semantics.md
  why: "§1 documents the empirical verification: git 2.54.0 passes argc=1 for a plain commit (no second parameter). This is the authoritative source for the fix — it supersedes the false 'VERIFIED argc=2' comment."
```

### Current Codebase Tree

```bash
stagehand/
└── internal/hooks/
    ├── runner.go        # EDIT: line ~195 (argv), lines ~52 + ~178 (comments)
    └── runner_test.go   # EDIT: + TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne
```

### Known Gotchas

```go
// The argv change is literally removing one element from a slice literal:
//   BEFORE: []string{msgPath, ""}    // argc=2 ($2 = empty string)
//   AFTER:  []string{msgPath}        // argc=1 ($2 unset — git parity)
// Do NOT add a third arg or change any other runHook parameter.

// The false comments reference "external_deps.md §2" — that file does NOT exist.
// The correction should cite githooks(5) (the authoritative git documentation) + the
// empirical observation (git 2.54.0, ARGC=1).

// The test must use $# (shell argc) — NOT $2 or ${#}. $# is the count that differs
// (1 vs 2). A test on $2 would pass either way (empty string in both cases since $2
// is "" when argc=2 and unset when argc=1 — shell prints "" for unset in most contexts).
```

## Implementation Blueprint

### Implementation Tasks

```yaml
Task 1: TDD — write the FAILING test first (internal/hooks/runner_test.go)
  - ADD after the existing prepare-commit-msg tests (near line ~317):
        func TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne(t *testing.T) {
            repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)

            argcFile := filepath.Join(repo, "argc.txt")
            installHook(t, repo, "prepare-commit-msg", `echo "ARGC=$#" > `+argcFile)

            cfg := defaultCfg()
            _, _, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
                "feat: a change", HookOpts{})
            if err != nil {
                t.Fatalf("RunCommitHooks err = %v", err)
            }

            data, readErr := os.ReadFile(argcFile)
            if readErr != nil {
                t.Fatalf("read argc file: %v", readErr)
            }
            got := strings.TrimSpace(string(data))
            if got != "ARGC=1" {
                t.Errorf("prepare-commit-msg $# = %q, want \"ARGC=1\" (git githooks(5): plain commit passes 1 arg; $2 unset)", got)
            }
        }
  - RUN: go test -run TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne ./internal/hooks/ -v
  - EXPECT: FAIL (argc=2 today). This confirms the test exercises the bug.

Task 2: FIX runner.go:195 — change argv from []string{msgPath, ""} to []string{msgPath}
  - LOCATE: the runHook call in runPrepareCommitMsg (line ~195).
  - CHANGE: `[]string{msgPath, ""}` → `[]string{msgPath}`.
  - PRESERVE: all other runHook parameters (gitDir, workTree, nil stdin, opts).

Task 3: CORRECT the comment at runner.go:52 (in the RunCommitHooks doc block)
  - LOCATE: the prepare-commit-msg bullet in the RunCommitHooks header doc (line ~52).
  - CHANGE: "Invoked as `<msgfile> \"\"` (PRD FR-V2; VERIFIED argc=2 for a plain commit — see external_deps.md §2)"
    → "Invoked as `<msgfile>` (git githooks(5): for a plain commit no second parameter is passed; argc=1, $2 unset)".

Task 4: CORRECT the comment at runner.go:178 (in runPrepareCommitMsg's doc comment)
  - LOCATE: the runPrepareCommitMsg function doc comment (line ~178).
  - CHANGE: "runs prepare-commit-msg <msgPath> \"\" (PRD FR-V2; VERIFIED argc=2 for a plain commit — external_deps.md §2)"
    → "runs prepare-commit-msg <msgPath> (git githooks(5): for a plain commit no second parameter is passed; argc=1, $2 unset)".

Task 5: VALIDATE
  - RUN: gofmt -w internal/hooks/runner.go internal/hooks/runner_test.go
  - RUN: go build ./... ; go vet ./...
  - RUN: go test -race -run TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne ./internal/hooks/ -v  # NOW PASSES
  - RUN: go test -race ./...  # full suite green
```

### Implementation Patterns & Key Details

```go
// === the ONE-line production change (runner.go:195) ===
// BEFORE:
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath, ""}, gitDir, workTree, nil, opts)
// AFTER:
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)
```

```go
// === the corrected comment (line ~52) ===
//   - prepare-commit-msg: ALWAYS runs (NoVerify + DryRun do NOT gate it — git-commit(1) parity).
//     Invoked as `<msgfile>` (git githooks(5): for a plain commit no second parameter is passed;
//     argc=1, $2 unset). The S2 seam shouldSkipStagehandPrepareCommitMsg stubs false (stagehand's
//     own hook would recurse; S2 fills via hook.Detect). Non-zero/timeout → *RescueError.
```

```go
// === the corrected comment (line ~178) ===
// runPrepareCommitMsg runs prepare-commit-msg <msgPath> (git githooks(5): for a plain commit no
// second parameter is passed; argc=1, $2 unset) on the SHARED message file. ALWAYS runs
// (NoVerify/DryRun don't gate it — the caller gates the OTHER hooks). Skipped if absent/non-exec
// OR stagehand's OWN hook (FR-V4 recursion prevention — invoking stagehand's own prepare-commit-msg
// would exec `stagehand hook exec` and recurse). Returns the CAUSE error on non-zero/timeout (the
// caller wraps the full-context *RescueError).
```

### Integration Points

```yaml
PRODUCTION (internal/hooks/runner.go):
  - line ~195: argv []string{msgPath, ""} → []string{msgPath}  (the ONLY behavior change)
  - line ~52: comment corrected (false "VERIFIED argc=2" → githooks(5) argc=1)
  - line ~178: comment corrected (same)

TESTS (internal/hooks/runner_test.go):
  - +TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne (asserts $#==1 via installHook + RunCommitHooks)

NO-TOUCH: the function signature, the return contract, any caller (CommitStaged/runPipeline/publishCommit
all call RunCommitHooks which internally calls runPrepareCommitMsg — the argv change is internal).

DOCS: none (the false "VERIFIED argc=2 / external_deps.md §2" references are in-source comments only;
no external doc references argc). The Mode A instruction is satisfied by correcting the in-source comments.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagehand
gofmt -w internal/hooks/runner.go internal/hooks/runner_test.go
gofmt -l .            # Expected: empty
go vet ./internal/hooks/...  # Expected: exit 0
go build ./...        # Expected: exit 0
```

### Level 2: Unit Tests

```bash
cd /home/dustin/projects/stagehand
go test -race -run TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne ./internal/hooks/ -v  # NOW PASSES
go test -race ./internal/hooks/ -v   # full hooks suite green
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagehand
go test -race ./...   # Expected: ALL packages green
git diff --stat       # Expected: internal/hooks/runner.go + runner_test.go only
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation
- [ ] `runPrepareCommitMsg` invokes the hook with `[]string{msgPath}` (1 arg).
- [ ] `TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne` asserts `$#` is 1.
- [ ] Comment at line ~52 corrected (no false "VERIFIED argc=2" / "external_deps.md §2").
- [ ] Comment at line ~178 corrected (same).

### Scope Discipline
- [ ] ONLY `internal/hooks/{runner,runner_test}.go` modified.
- [ ] Did NOT change the function signature or any caller.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't change `[]string{msgPath}` to `[]string{msgPath, ""}` "just in case" — git's documented behavior
  is argc=1 for a plain commit. Adding the empty string back reverts the fix.
- ❌ Don't leave the false "VERIFIED argc=2" / "external_deps.md §2" comments in place — they are factually
  wrong (the file doesn't exist; the verification never happened) and mislead future maintainers.
- ❌ Don't test on `$2` instead of `$#` — `$2` is empty/unset in both the buggy (argc=2, $2="") and fixed
  (argc=1, $2 unset) cases, so the test wouldn't distinguish them. `$#` is the count that changes (2→1).
- ❌ Don't change the runHook function signature, the runPrepareCommitMsg signature, or any other runHook
  parameter — only the argv slice literal changes.

---

## Confidence Score

**10/10** — a one-token production fix (drop `""` from a slice literal) + two comment corrections + one
TDD test that installs a `$#`-logging hook. The test uses the existing `installHook` + `primeRunnerRepo`
helpers, and the fix is the exact line the PRD issue prescribes. There is nothing that can go wrong beyond
a typo (caught by `go build`).
