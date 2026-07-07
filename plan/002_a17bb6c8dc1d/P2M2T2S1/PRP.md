---
name: "P2.M2.T2.S1 — Implement StatusPorcelain on the Git interface and gitRunner (PRD §13.6.5 arbiter trigger)"
description: |

  ADD ONE METHOD to the `Git` interface (`internal/git/git.go`) AND implement it on `*gitRunner`,
  consuming the existing `run()` helper (UNCHANGED):

    StatusPorcelain(ctx context.Context) (string, error)

  StatusPorcelain is the **arbiter trigger** for multi-commit decomposition (PRD §13.6.5: "After the
  loop, if `git status --porcelain` is non-empty (some changes were not claimed by any stager), the
  arbiter runs … If `git status --porcelain` is empty after the loop, the arbiter does not run — the
  perfect run."). It runs `git status --porcelain` and returns the trimmed stdout. The caller — the
  decompose orchestrator (P3.M4.T1.S1) — checks `output != ""` to decide whether to invoke the
  arbiter. Empty string ⟺ clean tree ⟺ no arbiter.

  CONTRACT (P2.M2.T2.S1, verbatim from the work item):
    1. RESEARCH NOTE: the arbiter runs iff `git status --porcelain` is non-empty after the decompose
       loop (§13.6.5). StatusPorcelain returns the raw porcelain output; the caller checks if it's empty
       (clean) or non-empty (leftovers exist). `git status --porcelain` exits 0 on success, lists each
       changed path with a 2-char status code. Read-only w.r.t. refs/index.
    2. INPUT: the existing gitRunner.run() helper.
    3. LOGIC: Add `StatusPorcelain(ctx context.Context) (string, error)` to Git interface and gitRunner.
       Implementation: run `git status --porcelain`, return stdout trimmed. Non-zero exit → error. The
       caller (decompose orchestrator) checks `strings.TrimSpace(output) != ""` to decide whether to run
       the arbiter.
    4. OUTPUT: StatusPorcelain returns the porcelain output string. Empty string means clean tree.
    5. DOCS: none — interface addition.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `run()` / `runWithInput()` in git.go — CONSUMED, not modified.
    - `TreeDiff` (P2.M2.T1.S2, parallel) — appends to the SAME interface + file. StatusPorcelain appends
      AFTER TreeDiff; appending at the END of both minimizes merge friction.
    - `WorkingTreeDiff` (P2.M2.T2.S2, sibling) — do NOT implement (separate, larger work item).
    - Decompose wiring (P3.x) — no caller references StatusPorcelain yet. This task only adds + tests.
    - go.mod / go.sum — UNCHANGED (stdlib only: context/fmt/strings already imported).
    - `StagedDiffOptions` / `EmptyTreeSHA` / `defaultExcludes` / `// Method ownership` comment — UNCHANGED.

  DELIVERABLES (1 file MODIFIED, 1 new file):
    MODIFY internal/git/git.go — (a) append `StatusPorcelain` to the `Git` INTERFACE (doc comment);
      (b) append the `(*gitRunner).StatusPorcelain` method body (a 3-statement port of StagedFileCount:
      run → err branch → code branch → return trimmed stdout).
    CREATE internal/git/statusporcelain_test.go — 8 tests (clean repo, clean unborn repo, unstaged
      changes, staged changes, raw porcelain format preserved, non-repo→error, git-binary-missing,
      context-cancelled).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (the new
  interface method is additive — no existing caller breaks); `git status --porcelain` exits 0 on a
  clean repo AND on an unborn repo (NO "128 = unborn" convention — verified); exits 128 on a non-repo
  directory → real error; a dirty repo returns non-empty porcelain with 2-char `XY` status codes
  preserved verbatim.

---

## Goal

**Feature Goal**: Implement the `git status --porcelain` plumbing primitive at the `Git` interface
boundary — the emptiness signal the decompose orchestrator uses to decide whether the arbiter runs
(PRD §13.6.5). This is the cleanest, smallest possible Git-interface method: run one read-only git
command, return its trimmed stdout, surface non-zero exits and infrastructural failures as errors.

**Deliverable** (1 file MODIFIED, 1 new):
1. `internal/git/git.go` — one new method appended to the `Git` interface (after `TreeDiff`) with a doc
   comment naming its command (`git status --porcelain`), its §13.6.5 arbiter-trigger role, its
   exit-code convention (0 on success incl. unborn/clean/dirty; 128 = non-repo/corrupt = real error;
   **NO "128 = unborn" special-case** — `git status --porcelain` works on unborn repos), and its
   read-only contract; plus the `(*gitRunner).StatusPorcelain` implementation, a 3-statement port of
   `StagedFileCount` (swap the command, drop the count loop, return the trimmed string).
2. `internal/git/statusporcelain_test.go` — 8 tests (clean repo → `("", nil)`; clean unborn repo →
   `("", nil)`; unstaged changes → non-empty containing the path; staged changes → non-empty
   containing `A  <path>`; raw porcelain `XY` format preserved; non-repo → error; git-binary-missing
   → `"git binary not found"`; context-cancelled → `errors.Is(err, context.Canceled)`), reusing the
   existing package-level test helpers (`initRepo`, `writeFile`, `stageFile`, `makeEmptyCommit`).

**Success Definition**:
- On a committed repo with no changes, `StatusPorcelain(ctx)` returns `("", nil)` (clean tree).
- On an UNBORN repo (zero commits) with no files, `StatusPorcelain(ctx)` returns `("", nil)` — NOT an
  error (proves `git status --porcelain` exits 0 on unborn repos, unlike `rev-parse HEAD`).
- On a repo with an untracked file `a.txt`, `StatusPorcelain(ctx)` returns a string containing
  `?? a.txt` (the 2-char `XY` code preserved verbatim, non-empty).
- On a repo with a staged new file `b.txt`, `StatusPorcelain(ctx)` returns a string containing
  `A  b.txt` (raw porcelain format, not parsed).
- On a plain directory that is NOT a git repo, `StatusPorcelain(ctx)` returns a non-nil error whose
  message contains `"git status --porcelain: failed"` (exit 128 = real error, NOT swallowed).
- With `PATH=""`, `StatusPorcelain(ctx)` returns a non-nil error containing `"git binary not found"`.
- With a pre-cancelled context, `StatusPorcelain(ctx)` returns a non-nil error with
  `errors.Is(err, context.Canceled)`.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` shows EXACTLY 1 modified
  (`internal/git/git.go`) + 1 new untracked test file — nothing else.

## User Persona

**Target User**: the decompose orchestrator (internal code, P3.M4.T1.S1), and by extension the end
user running `stagecoach` on an un-staged working tree to get multiple logically-coherent commits.
StatusPorcelain is NOT a user-facing CLI flag; it is the arbiter-trigger primitive the orchestrator
queries after the commit loop completes.

**Use Case**: after the decompose loop publishes all N commits, the orchestrator calls
`StatusPorcelain(ctx)`. If the result is `""` → the perfect run, the arbiter is skipped. If non-empty
→ some working-tree changes were not claimed by any stager → the arbiter runs to reconcile the
leftovers (PRD §13.6.5).

**Pain Points Addressed**: the orchestrator needs a single, atomic, read-only signal for "is the
working tree clean?" that (a) never mutates the index or refs (PRD §18.1), (b) reports both staged
residue and unstaged changes (the full `git status` view), and (c) distinguishes a genuine clean tree
from an error (non-repo/corrupt) — the latter being a real failure the orchestrator must surface, not
a false "perfect run". StatusPorcelain provides all three by returning the raw trimmed porcelain.

## Why

- **Closes PRD §13.6.5 at the plumbing layer.** §13.6.5 gates the arbiter entirely on
  `git status --porcelain` emptiness ("if `git status --porcelain` is non-empty … the arbiter runs …
  If empty … the arbiter does not run"). This task is the literal interface-level implementation of
  that gate.
- **Zero new logic — a 3-statement port of the proven `StagedFileCount`.** StatusPorcelain is
  `StagedFileCount` with the command swapped (`diff --cached --name-only` → `status --porcelain`) and
  the count loop dropped (return the trimmed string as-is, because the caller only checks emptiness —
  no path parsing needed). No new shell surface, no new exec, no new buffer handling, no parsing.
- **Lowest-risk, maximal-reuse, backward-compatible.** The interface GAINS one method (Go interfaces
  are open for extension — no existing implementation or caller breaks; `*gitRunner` is the only `Git`
  implementor, verified). No existing file's behavior changes. go.mod/go.sum untouched. Unblocks
  P3.M4.T1.S1 (the decompose orchestrator's arbiter gate).

## What

One new method on the `Git` interface (`internal/git/git.go`), implemented on `*gitRunner`, delegating
to `run()`. No new types. No new options struct (it is a pure emptiness signal — no caps/excludes/
binary filtering). No new dependencies. No caller wiring (that is P3.x). The structural edits are:
append one interface method + doc comment, append one `(*gitRunner)` method body, and add the new test
file.

### Success Criteria

- [ ] `StatusPorcelain(ctx context.Context) (string, error)` is declared on the `Git` interface with a
      doc comment naming: the command (`git status --porcelain`); the §13.6.5 arbiter-trigger role
      ("the arbiter runs iff this is non-empty"); the exit-code convention (0 on success — clean OR
      dirty, born OR unborn; 128 = non-repo/corrupt = real error; NO "128 = unborn" special-case —
      `git status --porcelain` works on unborn repos, unlike `rev-parse HEAD`); the read-only contract;
      and the empty-string ⟺ clean-tree semantics (the caller checks `output != ""`).
- [ ] `(*gitRunner).StatusPorcelain` body is exactly: `run(ctx, g.workDir, "status", "--porcelain")`
      → `if err != nil { return "", err }` → `if code != 0 { return "", fmt.Errorf("git status
      --porcelain: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }` →
      `return strings.TrimSpace(stdout), nil`. NO `if code == 128` branch.
- [ ] `statusporcelain_test.go` proves: clean committed repo → `("", nil)`; clean UNBORN repo →
      `("", nil)` (the key "unborn ≠ error" test); unstaged changes → non-empty containing the path;
      staged changes → non-empty containing `A  <path>`; raw `XY` 2-char porcelain format preserved;
      non-repo directory → error containing `"git status --porcelain: failed"`; git-binary-missing →
      `"git binary not found"`; context-cancelled → `errors.Is(err, context.Canceled)`.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified + 1 new file.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the empirical ground
truth (findings §2 — `git status --porcelain` exits 0 on clean AND unborn AND dirty; 128 ⟺ non-repo;
`XY <path>` format with trailing newline); the exact method to copy (findings §3 — the 3-statement
port of `StagedFileCount`, with the full code shown); `run()`'s invariant (findings §4 — non-zero git
exit → `(stdout, stderr, exitCode, nil)`, err ⟺ infrastructural failure only, so the two-branch
structure is exactly right); the reusable test helpers + the test idiom (findings §5); the unborn-repo
special case (findings §6 — the cleanest test, NOT an error); the scope boundaries (findings §7); and
the architecture-doc spec (findings §8 — signature + role confirmed). No prompt/decompose/registry
knowledge required — the contract is fully self-contained at the git-plumbing layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verified git status --porcelain behaviors + the port plan)
- docfile: plan/002_a17bb6c8dc1d/P2M2T2S1/research/findings.md
  why: §2 the VERIFIED git status --porcelain behaviors (exit 0 on clean/dirty/unborn; exit 128 ONLY on
       non-repo/corrupt; NO "128 = unborn" convention — this is the decisive finding, it drives the
       simple-branch implementation and forbids copying RevParseTree's 128 branch); §3 the EXACT 3-statement
       port of StagedFileCount (the whole implementation, with code shown); §4 run()'s INVARIANT (non-zero
       git exit → (stdout, stderr, code, nil); err ⟺ infrastructural only → two-branch structure); §5 the
       reusable test helpers + test idiom; §6 the unborn-repo special case (cleanest test, NOT an error);
       §7 scope boundaries (parallel TreeDiff append, do NOT touch WorkingTreeDiff); §8 architecture spec.
  critical: §2 (exit 0 on unborn → NO 128 special-case; the SIMPLE branch, NOT RevParseTree's 128-as-non-error);
            §3 (the command swap + drop-the-count-loop IS the implementation); §4 (err != nil branch must come
            FIRST and propagate UNWRAPPED so errors.Is works).

# MUST READ — the FILE TO MODIFY: the Git interface + the template method (StagedFileCount) + run()
- file: internal/git/git.go
  section: the `Git` interface (append StatusPorcelain at the END, after `TreeDiff` — currently the
           last method at ~line 133); `(*gitRunner).StagedFileCount` at ~line 756 (the EXACT method to copy
           — read-only, simple branch, `if err != nil` then `if code != 0`); `(*gitRunner).run` at ~line 140
           (the helper StatusPorcelain calls — its INVARIANT: non-zero git exit → (stdout, stderr, exitCode,
           nil), err nil).
  why: StagedFileCount is the single best template for StatusPorcelain (same simple two-branch structure,
       same stdout-returning read-only shape). Copying its body, swapping the command, and dropping the count
       loop IS the implementation. run()'s invariant justifies the two-branch structure.
  pattern: copy `(*gitRunner).StagedFileCount`'s body; change the command from `g.run(ctx, g.workDir,
           "diff", "--cached", "--name-only")` to `g.run(ctx, g.workDir, "status", "--porcelain")`; change
           the return type from `(int, error)` to `(string, error)`; DROP the `count` loop; return
           `strings.TrimSpace(stdout)` instead of `count`; change the error prefix to `"git status --porcelain:
           failed (exit %d): %s"`. Keep the `if err != nil { return "", err }` and `if code != 0 { ... }`
           branches byte-identical in SHAPE.
  gotcha: do NOT add a `if code == 128 { return "", nil }` branch — that is RevParseTree/RevParseHEAD's
          unborn convention; `git status --porcelain` exits 0 on unborn repos (findings §2), so 128 here is a
          real non-repo/corrupt error. Do NOT mutate `defaultExcludes` or any other state. Do NOT edit the
          `// Method ownership` comment block.

# MUST READ — the test exemplars (the patterns to mirror in statusporcelain_test.go)
- file: internal/git/stagedcount_test.go   (READ — the CLOSEST test template: read-only, simple-branch method)
  why: the EXACT shape StatusPorcelain tests copy: TestStagedFileCount_NothingStaged (committed clean →
       count 0; StatusPorcelain analog → ("", nil)); TestStagedFileCount_UnbornRepoWithStaged; the three
       cross-cutting error tests (_NotARepo, _GitBinaryMissing, _ContextCancelled) which StatusPorcelain
       copies ALMOST VERBATIM (same PATH="" / pre-cancelled-ctx / plain-dir fixtures, only the error-message
       substring changes to "git status --porcelain: failed").
  pattern: mirror these tests; replace `StagedFileCount()`/`count` with `StatusPorcelain()`/`out`; for the
           happy-path tests assert `strings.Contains(out, "<porcelain line>")` and `out != ""` (dirty) or
           `out == ""` (clean) instead of a count.
- file: internal/git/readtree_test.go   (READ — the sibling S1 test; the cleanest "append at END" exemplar)
  why: defines `execGit(t, dir, args...)` (independent-oracle git command, reusable) and the exact
       _NotARepo / _GitBinaryMissing / _ContextCancelled test structure StatusPorcelain mirrors. Its test
       function names are prefixed `TestReadTree_` — StatusPorcelain uses `TestStatusPorcelain_` (distinct,
       no collision).
- file: internal/git/committree_test.go   (READ — fixture definitions to REUSE)
  why: defines the package-level helpers `writeFile`, `stageFile` — ALL reusable from
       statusporcelain_test.go (same package `git`).
  gotcha: these helpers are ALREADY defined — do NOT redefine them (duplicate-symbol compile error).
- file: internal/git/revparse_test.go   (READ — makeEmptyCommit + minGitEnv helpers)
  why: defines `makeEmptyCommit(t, dir, msg)` — needed by the "committed, clean" test (and to make a born
       HEAD for the modified-tracked-file test). Reusable; do NOT redefine.
- file: internal/git/git_test.go   (READ — initRepo)
  why: defines `initRepo(t, dir)` — EVERY temp-repo test starts here (git init + repo-local identity).

# MUST READ — the design reference (signature + role)
- docfile: plan/002_a17bb6c8dc1d/architecture/binary_git_v2.md
  section: "### StatusPorcelain" — confirms the signature `StatusPorcelain(ctx context.Context) (string,
           error)`, the command (`git status --porcelain`), the arbiter-trigger role, and the "Non-empty →
           arbiter runs. Empty → perfect run." semantics. NOTE: the doc lists 5 new methods; THIS task
           implements ONLY StatusPorcelain. The other 4 (RevParseTree/ReadTree = done; TreeDiff = parallel;
           WorkingTreeDiff = sibling) are separate work items — do NOT implement them.
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "## Required New Git Methods" (line 84) + the pipeline note (line 27: "arbiter (only if status
           --porcelain non-empty)") — confirms StatusPorcelain as the arbiter trigger. Consumed by the P3
           decompose orchestrator's arbiter gate.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.5 (the arbiter — leftover reconciliation)
  why: §13.6.5 gates the arbiter ENTIRELY on `git status --porcelain` emptiness ("After the loop, if
       `git status --porcelain` is non-empty (some changes were not claimed by any stager), the arbiter
       runs … If `git status --porcelain` is empty after the loop, the arbiter does not run — the perfect
       run."). This task implements that gate at the interface boundary.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # MODIFY: append StatusPorcelain to the Git interface (doc comment) AND implement on
                      #   *gitRunner (3-statement port of StagedFileCount). run()/StagedFileCount/TreeDiff/
                      #   StagedDiffOptions/EmptyTreeSHA/defaultExcludes UNCHANGED (consumed).
  stagediff_test.go   # READ: test conventions (reuse helpers, do not redefine).
  stagedcount_test.go # READ: the CLOSEST test template (read-only simple-branch method + 3 error tests).
  readtree_test.go    # READ: execGit helper + the _NotARepo/_GitBinaryMissing/_ContextCancelled idiom.
  committree_test.go  # READ: writeFile/stageFile helpers (reuse).
  revparse_test.go    # READ: makeEmptyCommit helper (reuse).
  git_test.go         # READ: initRepo helper (reuse in every test).
  (*_test.go)         # READ: other per-method test files (the one-file-per-method convention).
go.mod / go.sum       # UNCHANGED (stdlib only: context/fmt/strings already imported in git.go).
.golangci.yml         # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added/modified

```bash
internal/git/git.go              # MODIFY — append to the `Git` interface (at the END, after `TreeDiff`,
                                 #   currently ~line 133):
                                 #     // StatusPorcelain returns the output of `git status --porcelain` —
                                 #     // the arbiter trigger for multi-commit decomposition (PRD §13.6.5:
                                 #     // "the arbiter runs iff `git status --porcelain` is non-empty after
                                 #     // the decompose loop; if empty, the arbiter does not run — the perfect
                                 #     // run"). The caller checks `output != ""` to decide. Empty string ⟺
                                 #     // clean tree. Read-only w.r.t. refs and the index. `git status
                                 #     // --porcelain` exits 0 on success (clean OR dirty, born OR unborn);
                                 #     // exit 128 means a non-repo/corrupt repo, a REAL error (NOT an unborn
                                 #     // signal — unlike rev-parse HEAD, porcelain works on unborn repos).
                                 #     StatusPorcelain(ctx context.Context) (output string, err error)
                                 #   AND append the (*gitRunner).StatusPorcelain method body at the END of the
                                 #   file (after TreeDiff's body, currently ~line 911): the 3-statement port
                                 #   of StagedFileCount (run → err branch → code branch → return trimmed stdout).
                                 #   NO other change to git.go.
internal/git/statusporcelain_test.go    # NEW — 8 tests (clean repo, clean unborn repo, unstaged changes,
                                        #   staged changes, raw porcelain format preserved, non-repo→error,
                                        #   git-binary-missing, context-cancelled).
# go.mod/go.sum UNCHANGED. run()/StagedFileCount/TreeDiff/StagedDiffOptions UNCHANGED. 0 other edits.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the command swap + drop-the-loop IS the implementation — findings §3): StatusPorcelain is
// StagedFileCount with the command swapped and the count loop dropped. Copy StagedFileCount's body:
//   StagedFileCount:  stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
//   StatusPorcelain:  stdout, stderr, code, err := g.run(ctx, g.workDir, "status", "--porcelain")
// then the SAME two branches, then `return strings.TrimSpace(stdout), nil` (NO count loop — the caller
// only checks emptiness, so there is nothing to parse).

// CRITICAL (NO "128 = unborn" special-case — findings §2): `git status --porcelain` exits 0 on a clean
// UNBORN repo (it lists untracked files as `??`; with no files, output is empty, exit 0). It exits 128
// ONLY on a non-repo / corrupt repo. Therefore 128 is a REAL error here, NOT a benign unborn signal.
// DO NOT copy RevParseTree/RevParseHEAD's `if code == 128 { return "", nil }` branch — that would swallow
// a non-repo error as a false "clean tree", and the orchestrator would silently skip the arbiter on a
// broken repo. Use the SIMPLE branch (`if code != 0 { return "", err }`) — identical to StagedFileCount
// and ReadTree. The pair of tests _CleanUnbornRepo (exit 0 → ("", nil)) + _NotARepo (exit 128 → error)
// together pin this convention.

// CRITICAL (run() returns err==nil for non-zero git exits — findings §4): run()'s INVARIANT is that a
// non-zero git exit is returned as (stdout, stderr, exitCode, nil) — err is nil, the exit code is the
// signal. Only infrastructural failures (LookPath miss / context cancel / start-I/O) return err != nil
// (exitCode -1). So the method does `if err != nil { return "", err }` FIRST (catches context.Canceled +
// git-binary-missing, propagated UNWRAPPED so errors.Is(err, context.Canceled) works at the call site),
// THEN branches on `code`. Copy StagedFileCount's two branches exactly.

// GOTCHA (return TRIMMED stdout): `git status --porcelain` emits a trailing "\n" after every line. Return
// `strings.TrimSpace(stdout)` so the caller's `output != ""` check is clean (an empty porcelain output is
// the literal empty string, not "\n"). The contract explicitly says "return stdout trimmed" and "the caller
// checks strings.TrimSpace(output) != ''" — trimming here means the caller can compare to "" directly.

// GOTCHA (do NOT parse / do NOT use -z): the caller ONLY checks emptiness (arbiter trigger). StatusPorcelain
// returns the RAW porcelain string. Do NOT split on "\n", do NOT handle rename `->` notation, do NOT add
// `-z`/`--null` (path edge cases — spaces, newlines in names — are IRRELEVANT to an emptiness check). The
// raw `XY <path>` format is preserved verbatim, which is also what the _RawPorcelainFormatPreserved test
// asserts (e.g. "?? a.txt", "A  b.txt" with the 2-char code + single space intact).

// GOTCHA (do NOT modify run(), runWithInput(), StagedFileCount, TreeDiff, or any consumed file): all are
// CONSUMED. StatusPorcelain adds ONE method + ONE test file. Editing any consumed file is a scope violation
// (and would change other methods' behavior or conflict with the parallel TreeDiff work).

// GOTCHA (parallel TreeDiff work — findings §7): TreeDiff (P2.M2.T1.S2) ALSO appends to git.go's interface
// block + gitRunner methods (it is currently the LAST interface method at ~line 133 + last body at ~line 911).
// Append StatusPorcelain at the END of the interface (after TreeDiff) and at the END of the file (after
// TreeDiff's body), and create statusporcelain_test.go as a NEW file (distinct from treediff_test.go) →
// minimal merge friction. Do NOT edit the `// Method ownership` comment block (a v1 provenance map). If a
// 3-way merge conflict appears at the closing interface brace, keep BOTH additions (independent lines).

// GOTCHA (one-file-per-method test convention): the package uses one test file per Git method. Put
// StatusPorcelain tests in a NEW `statusporcelain_test.go`. Test function names MUST be distinct (prefix
// `TestStatusPorcelain_`). Do NOT redefine any package-level helper (initRepo/writeFile/stageFile/
// makeEmptyCommit/execGit) — redefining = duplicate-symbol compile error.

// GOTCHA (Go interface extension is backward-compatible): adding StatusPorcelain to the Git interface does
// NOT break existing callers or *gitRunner (the only implementor — verified: New() returns *gitRunner; no
// other Git implementor exists). The method is additive.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. No new OPTIONS structs (StatusPorcelain takes no options — it is a pure emptiness signal).
No new constants. The implementation consumes the existing helpers unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   func (g *gitRunner) run(ctx, repo, args ...string) (stdout, stderr string, exitCode int, err error)
// run()'s invariant: non-zero git exit → (stdout, stderr, exitCode, nil); err != nil ⟺ infrastructural
// failure only (LookPath miss / context cancel / start-I/O), with exitCode == -1.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — append StatusPorcelain to the `Git` interface
  - LOCATE the END of the `Git` interface (the closing `}` after the `TreeDiff` method, currently ~line 133).
    APPEND StatusPorcelain there (after TreeDiff) — appending at the end minimizes merge friction with the
    parallel TreeDiff work.
  - INSERT, before the closing brace, the method declaration WITH a doc comment (see "Implementation
    Patterns" for the exact text).
  - NAMING: `StatusPorcelain(ctx context.Context) (string, error)` (EXACT, from the architecture doc +
    work item). Use a named return like `(output string, err error)` to match the file's doc-comment style.
  - DOC COMMENT must name: the command (`git status --porcelain`); the §13.6.5 arbiter-trigger role ("the
    arbiter runs iff this is non-empty; empty ⟺ perfect run"); the empty-string ⟺ clean-tree semantics;
    the exit-code convention (0 on success — clean OR dirty, born OR unborn; 128 = non-repo/corrupt = real
    error; NO "128 = unborn" special-case, unlike rev-parse HEAD); and the read-only contract.
  - GOTCHA: do NOT edit the `// Method ownership` comment block (a v1 provenance map; TreeDiff may touch
    nearby lines). Do NOT add any options struct (signature is `(ctx) (string, error)`).
  - PLACEMENT: internal/git/git.go, inside `type Git interface { … }`, at the END (after TreeDiff).

Task 2: MODIFY internal/git/git.go — implement (*gitRunner).StatusPorcelain (PORT of StagedFileCount)
  - APPEND a new method `func (g *gitRunner) StatusPorcelain(ctx context.Context) (string, error)` at the
    END of the file (after TreeDiff's body, currently ~line 911) — END-of-file is preferred to avoid merge
    conflict with the parallel TreeDiff work.
  - BODY (the WHOLE implementation — 4 lines after the signature):
      stdout, stderr, code, err := g.run(ctx, g.workDir, "status", "--porcelain")
      if err != nil {
          return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
      }
      if code != 0 {
          return "", fmt.Errorf("git status --porcelain: failed (exit %d): %s", code, strings.TrimSpace(stderr))
      }
      return strings.TrimSpace(stdout), nil
  - PATTERN: copy StagedFileCount's body; swap the command to `"status", "--porcelain"`; drop the `count`
    loop; return `strings.TrimSpace(stdout)` instead of `count`; change the error prefix to
    `"git status --porcelain: failed (exit %d): %s"`. Keep the `if err != nil` (FIRST, propagates UNWRAPPED)
    and `if code != 0` branches byte-identical in SHAPE to StagedFileCount.
  - GOTCHA: NO `if code == 128` branch — `git status --porcelain` exits 0 on unborn repos (findings §2); a
    128 here is a non-repo/corrupt error, surfaced via the `if code != 0` branch.
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method at the END of the file (after TreeDiff).

Task 3: CREATE internal/git/statusporcelain_test.go — 8 tests for StatusPorcelain
  - IMPORTS: context, errors, strings, testing (mirror stagedcount_test.go). The package is `git` (same
    package — helpers initRepo/writeFile/stageFile/makeEmptyCommit/execGit are visible).
  - ADD TestStatusPorcelain_CleanRepo:
      repo := t.TempDir(); initRepo(t, repo); makeEmptyCommit(t, repo, "init");   // born HEAD, nothing changed
      g := New(repo);
      out, err := g.StatusPorcelain(context.Background());
      assert err == nil && out == "";   // clean tree → ("", nil)
  - ADD TestStatusPorcelain_CleanUnbornRepo:   # KEY test — unborn ≠ error
      repo := t.TempDir(); initRepo(t, repo);   // NO commits, NO files
      g := New(repo);
      out, err := g.StatusPorcelain(context.Background());
      assert err == nil && out == "";   // unborn repo with no files → exit 0, empty porcelain (NOT an error)
  - ADD TestStatusPorcelain_UnstagedChanges:   # the primary arbiter trigger (working-tree leftovers)
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "a.txt", "x\n"); stageFile(t, repo, "a.txt"); makeEmptyCommit(t, repo, "init");
      writeFile(t, repo, "a.txt", "modified\n");   // tracked file modified, NOT staged
      writeFile(t, repo, "b.txt", "new\n");        // untracked file
      g := New(repo);
      out, err := g.StatusPorcelain(context.Background());
      assert err == nil && out != "";              // dirty → non-empty (arbiter WOULD run)
      assert strings.Contains(out, "a.txt") && strings.Contains(out, "b.txt");
  - ADD TestStatusPorcelain_StagedChanges:
      repo := t.TempDir(); initRepo(t, repo); makeEmptyCommit(t, repo, "init");
      writeFile(t, repo, "c.txt", "staged\n"); stageFile(t, repo, "c.txt");   // staged new file
      g := New(repo);
      out, err := g.StatusPorcelain(context.Background());
      assert err == nil && out != "" && strings.Contains(out, "c.txt");
  - ADD TestStatusPorcelain_RawPorcelainFormatPreserved:   # proves RAW output, not parsed
      repo := t.TempDir(); initRepo(t, repo); makeEmptyCommit(t, repo, "init");
      writeFile(t, repo, "untracked.txt", "u\n");                       // → "?? untracked.txt"
      writeFile(t, repo, "added.txt", "a\n"); stageFile(t, repo, "added.txt"); // → "A  added.txt"
      g := New(repo);
      out, err := g.StatusPorcelain(context.Background());
      assert err == nil;
      assert strings.Contains(out, "?? untracked.txt");   // 2-char XY code + single space preserved verbatim
      assert strings.Contains(out, "A  added.txt");        // staged-add code "A " preserved (two spaces: code + sep)
  - ADD TestStatusPorcelain_NotARepo:
      g := New(t.TempDir());   // plain dir, NOT a git repo (no initRepo)
      out, err := g.StatusPorcelain(context.Background());
      assert err != nil && strings.Contains(err.Error(), "git status --porcelain: failed") && out == "";
  - ADD TestStatusPorcelain_GitBinaryMissing:
      t.Setenv("PATH", "");   g := New(t.TempDir());
      out, err := g.StatusPorcelain(context.Background());
      assert err != nil && strings.Contains(err.Error(), "git binary not found") && out == "";
  - ADD TestStatusPorcelain_ContextCancelled:
      ctx, cancel := context.WithCancel(context.Background()); cancel();   // cancel BEFORE call
      g := New(t.TempDir());
      out, err := g.StatusPorcelain(ctx);
      assert errors.Is(err, context.Canceled) && out == "";
  - PATTERN: copy stagedcount_test.go's _NothingStaged / _UnbornRepoWithStaged / _NotARepo /
    _GitBinaryMissing / _ContextCancelled tests; replace StagedFileCount/count with StatusPorcelain/out;
    for happy-path tests assert `strings.Contains(out, "<porcelain line>")` + emptiness instead of a count.
  - GOTCHA: do NOT redefine initRepo/writeFile/stageFile/makeEmptyCommit/execGit — they are already
    package-level (redefining = compile error).
  - GOTCHA: TestStatusPorcelain_CleanUnbornRepo + TestStatusPorcelain_NotARepo are the pair that pins the
    "NO 128 special-case" convention: unborn is exit 0 (→ ("", nil)), non-repo is exit 128 (→ error). If a
    future implementer wrongly adds `if code == 128 { return "", nil }`, _NotARepo FAILS (catches the bug).
  - PLACEMENT: NEW file internal/git/statusporcelain_test.go.

Task 4: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/statusporcelain_test.go`
  - `go build ./...`   (whole module compiles — the interface + impl + test file)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/git/ -run "TestStatusPorcelain" -v`   (all 8 new tests)
  - `go test -race ./internal/git/`   (the WHOLE git package — existing tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; the new interface method is additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + `?? internal/git/statusporcelain_test.go`
    (2 entries); run()/StagedFileCount/TreeDiff/StagedDiffOptions UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1: the interface addition (append at the END of `type Git interface { … }`, after TreeDiff) ===

	// StatusPorcelain returns the output of `git status --porcelain` — the arbiter trigger for
	// multi-commit decomposition (PRD §13.6.5: "After the loop, if `git status --porcelain` is non-empty
	// (some changes were not claimed by any stager), the arbiter runs … If `git status --porcelain` is
	// empty after the loop, the arbiter does not run — the perfect run."). The caller — the decompose
	// orchestrator (P3) — checks `output != ""` to decide whether to invoke the arbiter; an empty string
	// means a clean tree (the perfect run). It is read-only with respect to refs and the index (PRD §18.1).
	//
	// `git status --porcelain` exits 0 on success whether the tree is clean or dirty, born or unborn (it
	// lists each changed path with a 2-char "XY" status code; untracked files appear as "??"). Exit 128
	// means a non-repo or corrupt repo — a REAL error, surfaced as a non-nil err (NOT an "unborn" signal:
	// unlike rev-parse HEAD, porcelain works on unborn repos, so there is no 128-as-non-error convention
	// here — branch on code != 0, never on code == 128). Each line is "XY <path>"; the raw string is
	// returned trimmed (caller compares to "").
	StatusPorcelain(ctx context.Context) (output string, err error)


// === Task 2: the implementation (append at the END of the file, after TreeDiff's body) ===

// StatusPorcelain returns the output of `git status --porcelain` (PRD §13.6.5 arbiter trigger). It is a
// port of StagedFileCount: the SAME simple two-branch structure (err-first infrastructural-failure
// propagation, then code != 0 → error), with the command swapped to `status --porcelain` and the count
// loop DROPPED (the caller only checks emptiness, so there is nothing to parse — return the trimmed
// stdout as-is). Read-only. NO 128-as-non-error special-case (porcelain exits 0 on unborn repos).
func (g *gitRunner) StatusPorcelain(ctx context.Context) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "status", "--porcelain")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// ALL non-zero exits are errors (128 = non-repo/corrupt). NO 128-as-non-error special-case —
		// `git status --porcelain` exits 0 on unborn repos, so a 128 here is a real caller error.
		return "", fmt.Errorf("git status --porcelain: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}
```

### Integration Points

```yaml
DATABASE:
  - none (git object store is read-only here; no migration, no index write).

CONFIG:
  - none (StatusPorcelain takes no options; no env vars, no config keys).

ROUTES:
  - none (internal plumbing method; no CLI flag, no public API surface in this task). The P3 decompose
    orchestrator (P3.M4.T1.S1) wires the call: `out, err := deps.Git.StatusPorcelain(ctx); if err != nil
    { ... }; if strings.TrimSpace(out) != "" { runArbiter(...) }`. That wiring is a SEPARATE work item.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after editing git.go + creating the test file — fix before proceeding
gofmt -w internal/git/git.go internal/git/statusporcelain_test.go
go build ./...            # whole module compiles (interface + impl + test file)
go vet ./...
golangci-lint run ./...   # errcheck/gosimple/govet/ineffassign/staticcheck/unused (.golangci.yml)

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
gofmt -l internal/ pkg/   # must print NOTHING (all formatted)
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test StatusPorcelain as created
go test -race ./internal/git/ -run "TestStatusPorcelain" -v   # all 8 new tests

# Full git-package regression (existing methods still pass — the new interface method is additive)
go test -race ./internal/git/

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# Full module regression — no other package breaks (the new interface method is additive: *gitRunner is
# the only Git implementor, so no other package needs updating).
go test ./...

# Manual plumbing check (independent oracle — proves the implementation matches real git behavior):
cd "$(mktemp -d)" && git init -q && git config user.name T && git config user.email t@t.com
echo "clean unborn, no files:"; git status --porcelain; echo "exit=$?"           # expect empty, exit 0
echo "x" > a.txt; echo "untracked:"; git status --porcelain; echo "exit=$?"      # expect "?? a.txt", exit 0
git add a.txt; echo "staged:"; git status --porcelain; echo "exit=$?"            # expect "A  a.txt", exit 0
cd "$(mktemp -d)"; echo "non-repo:"; git status --porcelain; echo "exit=$?"      # expect fatal, exit 128

# Expected: all exit codes match findings §2; go test ./... fully green.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (No creative/external validation needed — StatusPorcelain is a pure read-only plumbing primitive with
#  no network, no filesystem mutation, no concurrency. Levels 1–3 fully cover it.)

# Confirm go.mod/go.sum are untouched (stdlib-only, no new deps):
git diff --exit-code go.mod go.sum && echo "deps unchanged"
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./...`
- [ ] No linting errors: `golangci-lint run ./...`
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/ pkg/` (empty output)

### Feature Validation

- [ ] `StatusPorcelain` declared on the `Git` interface (after `TreeDiff`) with the full doc comment
- [ ] `(*gitRunner).StatusPorcelain` implemented as the 3-statement port of `StagedFileCount`
- [ ] Clean repo → `("", nil)`; clean UNBORN repo → `("", nil)` (NOT an error)
- [ ] Dirty repo → non-empty porcelain with raw `XY` 2-char codes preserved verbatim
- [ ] Non-repo → error containing `"git status --porcelain: failed"` (exit 128, NOT swallowed)
- [ ] git-binary-missing → `"git binary not found"`; context-cancelled → `errors.Is(err, context.Canceled)`
- [ ] NO `if code == 128 { return "", nil }` branch (porcelain exits 0 on unborn repos)
- [ ] Manual Level-3 plumbing check matches findings §2

### Code Quality Validation

- [ ] Follows existing codebase patterns (StagedFileCount/ReadTree simple-branch shape) and naming
- [ ] File placement matches the one-file-per-method test convention (NEW statusporcelain_test.go)
- [ ] Anti-patterns avoided (no 128 special-case, no path parsing, no options struct where none is needed)
- [ ] Consumed helpers (run/StagedFileCount/TreeDiff/helpers) UNCHANGED
- [ ] go.mod/go.sum UNCHANGED

### Documentation & Deployment

- [ ] Interface doc comment names the command, the §13.6.5 role, the exit-code convention, and read-only contract
- [ ] Method doc comment explains it is a port of StagedFileCount + why no 128 special-case
- [ ] No new env vars / config keys / CLI flags (internal plumbing addition)

---

## Anti-Patterns to Avoid

- ❌ Don't copy RevParseTree/RevParseHEAD's `if code == 128 { return "", nil }` — that is the unborn
  convention for `rev-parse`; `git status --porcelain` exits 0 on unborn repos, so 128 here is a real error.
- ❌ Don't parse the porcelain output (split on "\n", handle renames, add `-z`) — the caller only checks
  emptiness; the raw trimmed string is the contract.
- ❌ Don't add an options struct / caps / excludes / binary filtering — StatusPorcelain is a pure
  emptiness signal (unlike StagedDiff/TreeDiff/WorkingTreeDiff which carry StagedDiffOptions).
- ❌ Don't modify `run()`, `runWithInput()`, `StagedFileCount`, `TreeDiff`, or any consumed file — they
  are consumed, not owned by this task.
- ❌ Don't catch all exceptions — be specific: `err != nil` (infrastructural) THEN `code != 0` (git error),
  mirroring StagedFileCount exactly.
- ❌ Don't skip the `_CleanUnbornRepo` / `_NotARepo` test pair — together they pin the no-128-special-case
  convention; either alone is insufficient.

---

## Confidence Score

**9/10** for one-pass implementation success. This is the simplest possible Git-interface method — a
literal 3-statement port of the already-shipped `StagedFileCount` with the command swapped and the count
loop dropped. The only non-obvious decision (the exit-code convention) is fully resolved by the
empirical findings (`git status --porcelain` exits 0 on unborn repos → NO 128 special-case → simple
branch), and is pinned by the `_CleanUnbornRepo` + `_NotARepo` test pair. No new types, no new deps, no
caller wiring, no consumed-file edits — purely additive. The -1 accounts for the parallel-TreeDiff
append merge friction at the interface closing brace (low risk, both are independent additive lines).
