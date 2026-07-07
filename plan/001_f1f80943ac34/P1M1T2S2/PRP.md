---
name: "P1.M1.T2.S2 — RevParseHEAD with unborn-repo detection (exit-code check, NOT string emptiness)"
description: |
  Replace the `(*gitRunner).RevParseHEAD` panic-stub (landed by P1.M1.T2.S1) with a real
  implementation that delegates to the verified `run()` helper. Signature is fixed by the `Git`
  interface: `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)`. It runs
  `git -C <repo> rev-parse HEAD` and interprets the result by **exit code**: `err != nil` →
  infrastructural failure (propagate); `code == 128` → unborn repo (`return "", true, nil`); `code != 0`
  → unexpected error; `code == 0` → return `strings.TrimSpace(stdout), false, nil`. The unborn branch
  keys off exit 128, NOT stdout emptiness (the `commit-pi` latent bug — `git` prints literal `"HEAD\n"`
  to stdout on a zero-commit repo). Adds one stdlib import (`strings`) to `git.go` and one new test
  file `internal/git/revparse_test.go` (package git) covering born / unborn / git-missing /
  context-cancelled. Touches ONLY `internal/git/`; no interface, struct, `run()`, or other-method
  changes.
---

## Goal

**Feature Goal**: Implement the first real git plumbing method on `*gitRunner` — `RevParseHEAD` —
so that downstream steps of the atomic-snapshot flow (P1.M1.T2.S4 `CommitTree` root-commit handling
and P1.M1.T2.S5 `UpdateRefCAS` expected-old value) can obtain the current HEAD SHA *and* reliably
distinguish a rootless (zero-commit) repo. The unborn detection MUST be exit-code-based to avoid
the latent `commit-pi` bug where a non-empty-but-meaningless `"HEAD\n"` stdout is mistaken for a
commit SHA.

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: (a) add `"strings"` to the import block; (b) replace the
   `RevParseHEAD` panic-stub body with the ~8-line delegation to `run()` (exact body in §Blueprint).
2. **CREATE** `internal/git/revparse_test.go` (`package git`): four test functions
   (`TestRevParseHEAD_UnbornRepo`, `TestRevParseHEAD_BornRepo`, `TestRevParseHEAD_GitBinaryMissing`,
   `TestRevParseHEAD_ContextCancelled`) plus two small fixture helpers (`makeEmptyCommit`, `minGitEnv`).

No other files touched. No new dependencies (`strings` is stdlib). `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 4 new `RevParseHEAD` cases passing (plus S1's
existing `run()` tests still green); `RevParseHEAD` on a zero-commit temp repo returns
`("", true, nil)` (proving exit-128 detection); `RevParseHEAD` on a repo with one empty commit
returns `(40-hex-sha, false, nil)`; a missing git binary surfaces as a non-nil error (NOT misread
as `isUnborn=true`).

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator, and the sibling plumbing subtasks
S4 (`CommitTree`) and S5 (`UpdateRefCAS`) — the immediate consumers of `(sha, isUnborn, err)`.

**Use Case**: Before generating a commit message, the orchestrator calls
`sha, isUnborn, err := g.RevParseHEAD(ctx)` to learn the snapshot's parent. When `isUnborn`, the
parent list passed to `CommitTree` is empty (root commit, no `-p`) and the CAS expected-old value
for `UpdateRefCAS` is the all-zeros hash (PRD §13.5, §18.1).

**User Journey**: `g := git.New(repoPath)` → `parent, isUnborn, err := g.RevParseHEAD(ctx)` →
if `isUnborn`: `parents = nil; expectedOld = zeroHash`; else: `parents = []string{parent}; expectedOld = parent`.

**Pain Points Addressed**: Eliminates the `commit-pi` class of bug where `PARENT_SHA` silently
becomes the literal string `"HEAD"` on a fresh repo, which then causes a downstream
`git commit-tree -p HEAD` to fail with a confusing "not a valid SHA" error instead of creating the
intended root commit.

## Why

- **PRD §13.5 (Edge cases):** *"Rootless repo (no commits yet): `PARENT_SHA` is empty.
  `commit-tree` is called without `-p` (creates a root commit). `update-ref HEAD <new>` is called
  without the expected-old argument. Handled."* `RevParseHEAD`'s `isUnborn` flag is the single
  signal that drives this branching; getting it wrong breaks the first-commit path entirely.
- **PRD §18.1 (The invariant):** *"The repository's refs and index are modified only at the final
  `update-ref` step, and only if HEAD is unchanged since the snapshot."* `RevParseHEAD` supplies the
  `expectedOld` SHA that makes the CAS in `update-ref` (S5) a true compare-and-swap. For a root
  commit it supplies the unborn signal that selects the all-zeros expected-old.
- **`critical_findings.md` FINDING 1:** the unborn-`HEAD` trap. `git rev-parse HEAD` on a zero-commit
  repo prints `"HEAD\n"` (non-empty!) to stdout and exits 128. The zsh `commit-pi` checks
  `[[ -n "$PARENT_SHA" ]]` and would proceed with `PARENT_SHA="HEAD"`. **This subtask is the
  structural fix: encode the exit-code check into a typed `isUnborn bool` return.**
- **Foundation for S4/S5:** `CommitTree` (S4) uses `isUnborn` to decide `parents` (empty ⇒ root);
  `UpdateRefCAS` (S5) uses `isUnborn` to set `expectedOld` to the all-zeros hash. Both are blocked
  on a correct `RevParseHEAD`. This is the first of the 11 interface methods to leave stub-status,
  proving the `run()`-delegation pattern works end-to-end for a real method.

## What

`RevParseHEAD` runs `git -C <workDir> rev-parse HEAD` via the existing `run()` helper and translates
the four-tuple into `(sha, isUnborn, err)`:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `("", false, err)`. This is the ONLY path that returns a non-nil error.
- `exitCode == 128` → return `("", true, nil)`. Unborn repo. **Detected via exit code, not stdout.**
- `exitCode != 0` (any value other than 0 or 128 — unexpected for `rev-parse HEAD`) → return
  `("", false, fmt.Errorf("git rev-parse HEAD: unexpected exit %d: %s", code, strings.TrimSpace(stderr)))`.
- `exitCode == 0` → return `(strings.TrimSpace(stdout), false, nil)`. The trimmed 40-hex (sha-1) or
  64-hex (sha-256) HEAD SHA.

No porcelain, no `--verify -q`, no `symbolic-ref`, no SHA-format validation in production code
(the contract mandates plain `rev-parse HEAD` and a return of the trimmed stdout on success).

### Success Criteria

- [ ] `internal/git/git.go` imports `"strings"` (in addition to S1's `bytes, context, errors, fmt, os/exec`).
- [ ] `(*gitRunner).RevParseHEAD` body matches §Implementation Blueprint verbatim (no `panic`).
- [ ] `internal/git/revparse_test.go` exists in `package git` with the 4 named test functions.
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all 4 `TestRevParseHEAD_*` pass and S1's tests still pass.
- [ ] `TestRevParseHEAD_UnbornRepo` asserts `isUnborn == true` on a zero-commit repo.
- [ ] NO change to `run()`, `New`, `gitRunner`, the `Git` interface, `FileChange`, `StagedDiffOptions`,
      or any of the other 10 method stubs.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` (inherited from S1; `RevParseHEAD` adds none).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact two files to touch; the
exact `run()` contract being delegated to (signatures + the `err==nil`-for-non-zero-exits invariant);
the exact `RevParseHEAD` body (verified-equivalent to a throwaway program run against git 2.54.0);
the exact import delta (`strings`); the exact 4 test cases with verified assertions and fixture
helpers; and the exact validation commands with expected results. The unborn-repo behavior is
re-pinned empirically on this box (git 2.54.0: exit 128, stdout `"HEAD\n"`). No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§13.5 (rootless-repo edge case: empty PARENT_SHA, no -p, no expected-old) is the behavior
        RevParseHEAD's isUnborn flag drives; §18.1 (the invariant) is why expectedOld must be exact;
        §19 (security: args as []string, never sh -c) is inherited from run() and must not be violated."
  critical: "This subtask owns ONLY RevParseHEAD's body + its tests. Do NOT implement CommitTree (S4),
             UpdateRefCAS (S5), or any other method. Do NOT change the Git interface (it is already
             correct from S1 and consumed by the orchestrator in P1.M3.T4)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 1 — the unborn-HEAD trap: `git rev-parse HEAD` prints literal 'HEAD\\n' to stdout
        (NON-empty) and exits 128 on a zero-commit repo. The zsh commit-pi checks string emptiness
        and is therefore latently buggy. The fix is exit-code-based detection."
  critical: "FINDING 1 contains a near-identical Go snippet using exec directly; our version delegates
             to run() (which already implements the exec+errors.As logic) and then branches on
             run()'s exitCode. Do NOT duplicate the exec plumbing — call run()."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§4 (`git rev-parse HEAD`) documents the exact stdout/stderr/exit-code shapes and shows the
        canonical exit-code-check pattern. §'Cross-cutting conventions' mandates: TrimSpace the
        single-line output; never use stdout-emptiness as a semantic signal; include a stderr
        snippet in error messages."
  critical: "§4 also mentions a 'more robust alternative' (`--verify -q HEAD` / `symbolic-ref`).
             The work-item CONTRACT mandates plain `rev-parse HEAD` — do NOT switch to --verify -q."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Exit-Code Cheat Sheet row for `git rev-parse HEAD`: exit 0 = has SHA, exit 128 = unborn
        repo, and 'stdout = literal HEAD on unborn; check exit code!'. This is the one-row spec."
  critical: "Confirms exit 128 is the unborn signal and that run()'s err=nil-for-128 invariant (S1
             gotcha G2) is what makes the exitCode branch reachable."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything RevParseHEAD consumes: the gitRunner struct, the run() helper
        (exact signature and verified body), the RevParseHEAD panic-stub being replaced, the Git
        interface (signature already correct), New(), and the git_test.go initRepo(t,dir) helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil
             for non-zero exits and exitCode==-1 for infrastructural failures. S1's gotchas G2/G3
             (non-zero exit is not a Go error; unborn stdout is 'HEAD\\n') are the foundation."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/research/run_helper_validation.md
  why: "§3.1 empirically pins the rev-parse HEAD unborn behavior (stdout='HEAD\\n', exit 128, err=nil
             from run()). §4 confirms errors.As(ExitError)+ExitCode() is correct. §8 D3 is the
             decision that non-zero exits keep err=nil — the reason RevParseHEAD reads exitCode."
  critical: "This is the verified source of truth for run()'s return semantics. RevParseHEAD's branch
             order (err first, then code==128, then code!=0, then code==0) depends on D3."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/research/revparse_head_validation.md
  why: "THIS subtask's own research: the exact RevParseHEAD body, the import delta (strings), the
        born-vs-unborn test fixtures (initRepo + makeEmptyCommit), the 4 test cases with assertions,
        and the deterministic ctx-cancel handling."
  critical: "§3.1 calls out the single non-obvious edit (add 'strings' to imports). §4.2 gives the
             test matrix. §5 is the do-NOT-do scope list."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt-emHEAD
  why: "Documents that `git rev-parse HEAD` resolves the current HEAD commit; on an unborn branch it
        errors. Confirms the 128-exit + diagnostic behavior we branch on."
  critical: "Establishes `rev-parse HEAD` (not `--verify -q`) is the contractually-mandated form."
- url: https://pkg.go.dev/strings#TrimSpace
  why: "TrimSpace removes the trailing '\\n' from git's single-line SHA output (and trims stderr in
        the error path)."
  critical: "This is the only new stdlib symbol RevParseHEAD introduces; it is why 'strings' is added to imports."
- url: https://pkg.go.dev/context#Canceled
  why: "The error value run() returns (via ctx.Err()) when the context is cancelled before/during the
        git call; RevParseHEAD propagates it unchanged."
  critical: "The ctx-cancel test asserts errors.Is(err, context.Canceled) — confirm it is the unwrapped ctx.Err()."
```

### Current Codebase Tree (after P1.M1.T2.S1 has landed — assume as-specified)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # ← S1: Git interface + gitRunner + run() + New() + 11 panic-stubs
│       └── git_test.go   # ← S1: package git; TestNew/TestRun_*/TestStubsPanic + initRepo(t,dir)
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go            # MODIFIED — added "strings" import; RevParseHEAD stub → real body
        ├── git_test.go       # UNCHANGED (S1's file; initRepo helper reused, not edited)
        └── revparse_test.go  # NEW — package git; 4 TestRevParseHEAD_* + makeEmptyCommit + minGitEnv
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (1) Add `"strings"` to imports. (2) Replace `RevParseHEAD` panic-stub with the `run()`-delegating body. Nothing else. |
| `internal/git/revparse_test.go` | CREATE | `package git` tests for `RevParseHEAD`: unborn / born / git-missing / ctx-cancelled, plus `makeEmptyCommit` and `minGitEnv` fixture helpers. |

**Explicitly NOT created/modified:** `run()`, `New`, `gitRunner`, the `Git` interface,
`FileChange`, `StagedDiffOptions`, the other 10 method stubs, `git_test.go` (S1's tests),
`go.mod`/`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the core trap this subtask exists to fix): on a zero-commit repo,
// `git rev-parse HEAD` prints the LITERAL STRING "HEAD\n" to stdout (NON-empty!) and exits 128.
// Detecting unborn-ness via `if stdout == ""` is WRONG — it is the commit-pi latent bug.
// RevParseHEAD branches on run()'s exitCode == 128, NOT on stdout emptiness.
// Verified on this box (git 2.54.0): stdout="HEAD\n", stderr=fatal, exit=128, err=nil from run().

// CRITICAL (G2 — run()'s invariant, inherited from S1): run() returns err == nil for NON-ZERO git
// exits (1, 128, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O) set
// err != nil, with exitCode == -1. Therefore RevParseHEAD MUST check `err != nil` FIRST (authoritative,
// covers the -1 cases), THEN branch on exitCode. A reversed check (code before err) would still be
// correct here because err!=nil ⟹ code==-1 (which is neither 0 nor 128), but checking err first is
// the clear/readable order and matches every architecture example.

// CRITICAL (G3 — LookPath miss must NOT be misread as unborn): when git is absent, run() returns
// exitCode == -1, err != nil. The RevParseHEAD `if err != nil { return "", false, err }` guard runs
// BEFORE the `code == 128` branch, so a missing binary is surfaced as a real error (isUnborn=false),
// never as a silent unborn. TestRevParseHEAD_GitBinaryMissing guards against regressing this.

// GOTCHA (G4 — the import delta): S1's git.go imports bytes, context, errors, fmt, os/exec — but NOT
// strings. RevParseHEAD uses strings.TrimSpace (born-case SHA + error-path stderr). You MUST add
// "strings" to the import block. The compiler will error `undefined: strings` if you forget; stating
// it here saves a wasted build cycle.

// GOTCHA (G5 — context cancellation order in run()): run() checks ctx.Err() BEFORE errors.As(
// ExitError). So when ctx is cancelled, RevParseHEAD gets (stdout, stderr, -1, ctx.Err()) and the
// `err != nil` guard returns ("", false, ctx.Err()). The cancel-test cancels ctx BEFORE the call for
// determinism and asserts errors.Is(err, context.Canceled).

// GOTCHA (G6 — do NOT add SHA validation to production code): the contract says on exit 0 return
// the trimmed stdout. Do NOT add a hex-length check or parsing to RevParseHEAD — that would deviate
// from the contract and is unnecessary (downstream commit-tree/update-ref will reject a bad SHA).
// The `^[0-9a-f]{40,64}$` regex is TEST-ONLY (sanity-check of git's own contract; 40=sha-1, 64=sha-256).

// GOTCHA (G7 — test file is package git, white-box): RevParseHEAD is on *gitRunner and calls the
// unexported run(); the unexported gitRunner.workDir is read by New. To call New() and exercise the
// real method against temp repos, the test MUST be `package git` (NOT `git_test`). This matches S1's
// git_test.go, so both files coexist in package git and share helpers (initRepo).

// GOTCHA (G8 — reuse S1's initRepo, do not redeclare): S1 defines initRepo(t, dir) in git_test.go
// (package git) producing an UNBORN repo. revparse_test.go (same package) reuses it directly — do NOT
// redeclare initRepo or you get a compile error. For the BORN fixture, add a NEW helper makeEmptyCommit
// (distinct name, no collision). minGitEnv is also new and distinct.

// GOTCHA (G9 — empty commit identity): `git commit --allow-empty` needs author/committer identity.
// makeEmptyCommit passes GIT_AUTHOR_NAME/EMAIL and GIT_COMMITTER_NAME/EMAIL via cmd.Env (plus a
// minimal PATH+HOME from minGitEnv) so the commit succeeds even with no global git config set.

// GOTCHA (G10 — no shell, no cmd.Dir): RevParseHEAD inherits S1's §19 guarantees because it only
// calls run(). Do NOT introduce exec.Command / os.Chdir / sh -c anywhere in the new code. The
// validation greps (Level 3) must still find zero matches.
```

## Implementation Blueprint

### Data models and structure

None added or changed. `RevParseHEAD`'s return types (`string, bool, error`) are already declared
in the `Git` interface by S1. No new structs, no new options type.

### The `RevParseHEAD` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature.

```go
// RevParseHEAD returns the SHA HEAD currently points at. On a repository with zero commits it
// returns sha="" and isUnborn=true, detected via git's exit code 128 (NOT stdout emptiness —
// `git rev-parse HEAD` prints the literal string "HEAD\n" to stdout on an unborn repo, which is
// the latent bug in commit-pi; see critical_findings.md FINDING 1).
func (g *gitRunner) RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error) {
    stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "HEAD")
    if err != nil {
        return "", false, err // git binary missing / context cancelled / start failure (run sets code=-1)
    }
    if code == 128 {
        return "", true, nil // unborn repo — exit-code signal, NOT string emptiness
    }
    if code != 0 {
        return "", false, fmt.Errorf("git rev-parse HEAD: unexpected exit %d: %s", code, strings.TrimSpace(stderr))
    }
    return strings.TrimSpace(stdout), false, nil
}
```

> **Verified:** the branch order and the `code == 128 ⟹ ("", true, nil)` mapping are confirmed by
> S1's `run_helper_validation.md` §3.1 (unborn: `stdout="HEAD\n", exitCode=128, err=nil`) and §4
> (exit-code path), re-pinned empirically on this box (git 2.54.0).

### The import change (exact)

S1's import block:
```go
import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os/exec"
)
```
Add `"strings"` (keep gofmt's alphabetical grouping):
```go
import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os/exec"
    "strings"
)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (two surgical edits)
  - EDIT 1 — add the "strings" import:
      FIND the import block (bytes, context, errors, fmt, os/exec) and ADD "strings" in sorted order.
      (This is the ONLY import change; bytes/errors/os/exec stay — run() still uses them.)
  - EDIT 2 — replace the RevParseHEAD panic-stub:
      FIND the stub:
        func (g *gitRunner) RevParseHEAD(ctx context.Context) (string, bool, error) {
            panic("gitRunner.RevParseHEAD: not yet implemented — see P1.M1.T2.S2")
        }
      REPLACE with the body in §"The RevParseHEAD body" above (keep the same signature, add the doc comment).
  - DO NOT touch: run(), New, gitRunner, Git interface, FileChange, StagedDiffOptions, or any of the
    other 10 method stubs (WriteTree, CommitTree, UpdateRefCAS, DiffTree, StagedDiff,
    HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: CREATE internal/git/revparse_test.go (package git — white-box)
  - FILE: internal/git/revparse_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G7; matches git_test.go)
  - IMPORTS: context, errors, os, os/exec, regexp, strings, testing
  - WRITE the fixture helpers:
      minGitEnv() []string   → returns []string{"PATH="+os.Getenv("PATH"), "HOME="+os.Getenv("HOME")}
      makeEmptyCommit(t, dir, msg)  → exec "git -C dir commit --allow-empty -m msg" with env =
        append(minGitEnv(), GIT_AUTHOR_NAME/EMAIL, GIT_COMMITTER_NAME/EMAIL = Test/test@example.com);
        on error t.Fatalf with CombinedOutput. t.Helper().
  - WRITE the 4 test functions (assertions in §"Test cases" below):
      TestRevParseHEAD_UnbornRepo:
        repo := t.TempDir(); initRepo(t, repo)          // REUSE S1's helper (gotcha G8) — zero commits
        g := New(repo)
        sha, isUnborn, err := g.RevParseHEAD(context.Background())
        assert err == nil
        assert isUnborn == true                           // ← THE exit-128 detection assertion
        assert sha == ""
      TestRevParseHEAD_BornRepo:
        repo := t.TempDir(); initRepo(t, repo); makeEmptyCommit(t, repo, "initial")
        g := New(repo)
        sha, isUnborn, err := g.RevParseHEAD(context.Background())
        assert err == nil
        assert isUnborn == false
        assert regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha)  // sha-1=40 / sha-256=64
      TestRevParseHEAD_GitBinaryMissing:
        t.Setenv("PATH", "")                              // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                             // dir need not be a repo (LookPath fails first)
        sha, isUnborn, err := g.RevParseHEAD(context.Background())
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert isUnborn == false                          // ← guard: LookPath miss NOT misread as unborn
        assert sha == ""
      TestRevParseHEAD_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call (deterministic)
        g := New(t.TempDir())
        sha, isUnborn, err := g.RevParseHEAD(ctx)
        assert err != nil && errors.Is(err, context.Canceled)
        assert isUnborn == false
        assert sha == ""
  - NAMING: TestRevParseHEAD_<Scenario>; helpers minGitEnv / makeEmptyCommit (distinct from S1's initRepo).
  - DO NOT redeclare initRepo (it lives in git_test.go). If initRepo is NOT present for any reason
    (S1 not yet landed at edit time), define a LOCAL helper named initRepoForRevParse instead — never
    two symbols with the same name in one package.
  - VERIFY: go test -race ./internal/git/ → exit 0, all 4 new tests pass.

Task 3: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/                    (expect: no matches)
  - RUN: git grep -n 'panic.*RevParseHEAD' internal/git/git.go             (expect: no matches — stub gone)
  - RUN: git status --porcelain → expect ONLY internal/git/git.go (modified) + internal/git/revparse_test.go (new).
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestRevParseHEAD_UnbornRepo` | `initRepo` (0 commits) | `err==nil && isUnborn==true && sha==""` | Exit-128 detection (NOT string emptiness) |
| `TestRevParseHEAD_BornRepo` | `initRepo` + `makeEmptyCommit` | `err==nil && isUnborn==false && sha` matches `^[0-9a-f]{40,64}$` | Happy path returns a real SHA, trimmed |
| `TestRevParseHEAD_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found"; `isUnborn==false` | `run()`'s err path is propagated, not misread as unborn |
| `TestRevParseHEAD_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `isUnborn==false` | ctx.Err() surfaced (not exit 128) |

### Implementation Patterns & Key Details

```go
// === Why err is checked BEFORE code (the branch order) ===
// run() guarantees: err != nil  ⟹  exitCode == -1  (LookPath / context / start failure).
//                   err == nil   for every real git exit (0, 128, …).
// So `if err != nil { return "", false, err }` is the authoritative infrastructural-failure guard.
// Only when err == nil do the exitCode branches (128 / nonzero / 0) run. Reversing the order would
// still be correct (code==-1 hits neither 0 nor 128, falling to the catch-all error), but err-first
// is clearer and matches the architecture reference's "check *exec.ExitError.ExitCode() for signals"
// guidance (the err here subsumes the non-ExitError cases run() already separated out).

// === Why we include stderr only in the unexpected-exit branch ===
// The unborn branch returns a clean ("", true, nil) — callers treat isUnborn as a normal condition,
// not an error, so it must NOT carry a scary fatal-message. Only the genuinely-unexpected case
// (exit code not 0/128, e.g. a corrupted repo) wraps the trimmed stderr for debuggability.

// === Why strings.TrimSpace on stdout (born case) ===
// git prints "<sha>\n" (trailing newline). TrimSpace yields the bare 40/64-hex SHA that CommitTree
// (S4) passes as -p and UpdateRefCAS (S5) uses as expectedOld. Untrimmed, those args would carry a
// "\n" and fail. (Verified: git emits LF even on Windows; no \r\n handling needed — see git_plumbing_summary.)

// === Why the ctx-cancel test cancels BEFORE the call ===
// Pre-cancelling makes cmd.Run() fail deterministically (the process is killed/not started), so
// run()'s ctx.Err()-before-errors.As path returns (-1, context.Canceled) reliably. Cancelling
// mid-run would race with a fast `rev-parse` completion.

// === Reusing initRepo from git_test.go ===
// Because revparse_test.go is `package git`, S1's initRepo(t, dir) is in scope. Call it directly;
// do NOT redefine it (a redeclaration is a compile error). It produces an unborn repo (git init, no
// commits) — exactly the fixture TestRevParseHEAD_UnbornRepo needs.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, strings, regexp, t.Setenv (1.17+), errors.Is all available
  - deps: NONE added (strings is stdlib)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go            # MODIFIED: +strings import, RevParseHEAD body
  - file: internal/git/revparse_test.go  # NEW: package git, 4 tests + 2 helpers

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M1.T2.S4 (CommitTree): reads isUnborn → parents == nil (root commit, omit -p) when true
  - P1.M1.T2.S5 (UpdateRefCAS): reads isUnborn → expectedOld == all-zeros hash when true; else expectedOld == sha
  - P1.M1.T3.S3/S4 (RecentMessages/Subjects/CommitCount): callers short-circuit on isUnborn (git log/rev-list
    fail with 128 on an unborn repo)
  - P1.M3.T4 (CommitStaged orchestrator): the primary caller — `parent, isUnborn, err := g.RevParseHEAD(ctx)`

PARALLEL-EXECUTION NOTE:
  - S1 (git.go + git_test.go) is landing concurrently. This subtask EDITS git.go in exactly two spots
    (one import line, one method body) and ADDS a separate revparse_test.go — no overlap with S1's
    test file. If git.go does not yet exist at edit time, S1 has not landed; in that case the agent
    should still be able to apply the edits once S1 lands (the edit anchors are the S1 stub text,
    which is quoted verbatim above). Do NOT create git.go from scratch (that is S1's deliverable).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...        # Expected: exit 0, no warnings (e.g. no unused import, no shadowing)
go build ./internal/git/         # Expected: exit 0 (package compiles; strings import resolves)
go build ./...                   # Expected: exit 0 (whole module compiles)

# Expected: zero output/errors. If `go build` says `undefined: strings`, you forgot the import (gotcha G4).
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v -run 'TestRevParseHEAD' ./internal/git/   # Expected: 4 tests PASS, exit 0
# Must see: TestRevParseHEAD_UnbornRepo, TestRevParseHEAD_BornRepo,
#           TestRevParseHEAD_GitBinaryMissing, TestRevParseHEAD_ContextCancelled — all ok.

go test -race ./internal/git/    # Expected: exit 0 — S1's tests (TestNew/TestRun_*/TestStubsPanic)
                                 # STILL PASS (the RevParseHEAD stub is now real, so TestStubsPanic
                                 # must skip/exclude RevParseHEAD — see Level-3 check).

make test                        # Expected: exit 0 (S2's target = go test -race ./...)
```

> **Note on S1's `TestStubsPanic`:** S1's stub-panic test iterates the 11 methods expecting each to
> panic. Once RevParseHEAD is real, that test will FAIL if it includes RevParseHEAD in its iteration.
> The S1 PRP's `TestStubsPanic` asserts a panic for "each of the 11 methods" — so making RevParseHEAD
> real will break it. **Resolution:** S1's `TestStubsPanic` is S1's responsibility; this subtask does
> NOT edit `git_test.go`. If `TestStubsPanic` breaks, the correct fix is for the implementing agent
> to confirm whether S1's test enumerates methods dynamically (and thus self-excludes real ones) or
> statically lists them. If it statically lists RevParseHEAD and fails, **stop and flag**: the cleanest
> fix is to remove RevParseHEAD from S1's static list (a one-line edit to git_test.go), but editing
> git_test.go touches S1's deliverable. Prefer instead: if S1's TestStubsPanic uses a per-method
> recover helper that only FAILS on a non-panic, a real (non-panicking) RevParseHEAD will fail it. In
> that case, the minimal non-conflicting fix is to update the static list in git_test.go to drop
> RevParseHEAD (this is an allowed exception to "don't touch git_test.go" because it is the direct,
> required consequence of implementing RevParseHEAD, and the alternative — a permanently-failing suite
> — is worse). Document the edit in the commit message.

### Level 3: Security & Structural Invariants (the §19 enforcement + scope discipline)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution anywhere in the git wrapper (inherited; new code adds none).
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/
# Expected: NO output.

# No os.Chdir / cmd.Dir (inherited; new code adds none).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/
# Expected: NO output.

# The RevParseHEAD stub is gone (replaced by a real body).
git grep -n 'RevParseHEAD' internal/git/git.go
# Expected: the func declaration + body; NO line matching `panic("gitRunner.RevParseHEAD`.

# Only the intended files changed.
git status --porcelain
# Expected EXACTLY:
#   M internal/git/git.go
#   ?? internal/git/revparse_test.go
# (and possibly M internal/git/git_test.go IF the TestStubsPanic fix in Level-2 note was needed.)

# go.mod / go.sum untouched.
git diff --name-only go.mod go.sum
# Expected: NO output.
```

### Level 4: Runtime Smoke Test (prove RevParseHEAD works against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the unborn/born behavior the test asserts, against the real binary (mirrors the research):
tmp=$(mktemp -d); git -C "$tmp" init -q
git -C "$tmp" rev-parse HEAD; echo "unborn EXIT=$?"   # expect stdout "HEAD", EXIT=128
git -C "$tmp" -c user.name=t -c user.email=t@t commit --allow-empty -q -m init
git -C "$tmp" rev-parse HEAD; echo "born   EXIT=$?"   # expect 40-hex SHA, EXIT=0
rm -rf "$tmp"
# If either exit code differs (e.g. unborn != 128), the box's git differs from 2.54.0; re-pin the
# assertion in TestRevParseHEAD_UnbornRepo to the observed code and note the git version.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (the `strings` import resolves).
- [ ] `go test -race ./internal/git/` exits 0 (4 new `TestRevParseHEAD_*` + S1's tests pass).
- [ ] `make test` exits 0.

### Feature Validation

- [ ] `(*gitRunner).RevParseHEAD` body matches §Blueprint (delegates to `run()`, branches on `code == 128`).
- [ ] On a zero-commit repo: returns `("", true, nil)` (exit-128 detection, not string emptiness).
- [ ] On a one-commit repo: returns `(40-hex-sha, false, nil)` (trimmed, valid hex).
- [ ] On a missing git binary: returns a non-nil error mentioning "git binary not found" (NOT `isUnborn=true`).
- [ ] On a cancelled context: returns `errors.Is(err, context.Canceled)`.
- [ ] No production-code SHA-format validation added (regex is test-only).

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` anywhere (Level 3 grep → no matches).
- [ ] NO `cmd.Dir` / `os.Chdir` (Level 3 grep → no matches).
- [ ] `run()` is NOT re-implemented or modified (delegated to, unchanged).
- [ ] `Git` interface, `gitRunner`, `New`, `FileChange`, `StagedDiffOptions` unchanged.
- [ ] The other 10 method stubs untouched (still panic with their owning-subtask messages).
- [ ] `go.mod` / `go.sum` unchanged (no new deps).
- [ ] Only `internal/git/git.go` (modified) and `internal/git/revparse_test.go` (new) are changed
      (plus, if strictly necessary, the one-line `TestStubsPanic` list edit noted in Level 2).
- [ ] No signal handling / `SysProcAttr` / process-group code added (that is P1.M4.T2).

### Documentation & Deployment

- [ ] Doc comment on `RevParseHEAD` explains the exit-128-vs-stdout-emptiness distinction (FINDING 1).
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't detect an unborn repo with `if sha == ""` / `if stdout == ""` — `git rev-parse HEAD` prints the literal `"HEAD\n"` (non-empty!) and exits 128. Branch on `run()`'s `exitCode == 128` (gotcha G1).
- ❌ Don't re-implement the `exec.CommandContext` plumbing inside `RevParseHEAD` — delegate to `run()`, which already handles LookPath, `-C repo` args, separate buffers, and `errors.As(*exec.ExitError)`. Duplicating it breaks the single-shell-out-point invariant and §19 (gotcha: "NO exec.Command in the new code").
- ❌ Don't check `exitCode` before `err` and then forget the `err != nil` guard — a missing git binary yields `exitCode == -1`, which must surface as a real error, not fall through to `code == 128` (it won't, since -1 ≠ 128, but the explicit guard makes the intent unambiguous and is tested by `TestRevParseHEAD_GitBinaryMissing`) (gotcha G3).
- ❌ Don't forget to add `"strings"` to the imports — `strings.TrimSpace` is used in both the success and the unexpected-error branches. The compiler will catch it, but state it up front (gotcha G4).
- ❌ Don't add SHA-format validation (hex/length checks) to the production method — the contract says return the trimmed stdout on exit 0. Keep the regex in the TEST only (gotcha G6).
- ❌ Don't switch to `git rev-parse --verify -q HEAD` or `git symbolic-ref` — the work-item contract mandates plain `rev-parse HEAD`. The architecture reference mentions those as alternatives; do not adopt them.
- ❌ Don't write the test as `package git_test` (black-box) — `New()` returns the `Git` interface but the meaningful fixture work needs `package git` to share `initRepo` and exercise the real `*gitRunner` path. Match S1's `git_test.go` package (gotcha G7).
- ❌ Don't redeclare `initRepo` in `revparse_test.go` — it already lives in `git_test.go` (same package). Reuse it; use distinct names (`makeEmptyCommit`, `minGitEnv`) for new helpers (gotcha G8).
- ❌ Don't cancel the context mid-run in the cancel test — cancel BEFORE the call for determinism, and assert via `errors.Is(err, context.Canceled)` (gotcha G5).
- ❌ Don't touch `git_test.go` except for the one-line `TestStubsPanic` exclusion if (and only if) it statically enumerates RevParseHEAD and therefore fails once the method is real (Level-2 note). Everything else in `git_test.go` is S1's.
- ❌ Don't implement `CommitTree` / `UpdateRefCAS` / any other method, and don't edit `run()` or the interface — those are other subtasks' deliverables.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: This is a small, fully-specified change. The `RevParseHEAD` body is ~8 lines and is
verified-equivalent to the pattern in `critical_findings.md` FINDING 1 and `git_plumbing_reference.md`
§4, re-pinned empirically on this exact box (git 2.54.0: unborn → exit 128 / stdout `"HEAD\n"`; born →
exit 0 / 40-hex SHA). The delegation target (`run()`) and its `err==nil`-for-non-zero-exits invariant
are themselves empirically verified in S1's research and quoted verbatim here. The only non-obvious
mechanical step — adding `"strings"` to the imports — is called out explicitly (gotcha G4). The four
test cases each map to a distinct, verified branch (unborn/ born/ LookPath-miss/ ctx-cancel) with
deterministic fixtures. The one residual cross-subtask wrinkle (S1's `TestStubsPanic` may statically
list RevParseHEAD) is anticipated with an explicit, scoped resolution (Level-2 note) so the agent is
not blocked. No external dependencies, no interface changes, no ambiguity in the contract.
