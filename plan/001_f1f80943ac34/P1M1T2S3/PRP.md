---
name: "P1.M1.T2.S3 — WriteTree with merge-conflict failure detection"
description: |
  Replace the `(*gitRunner).WriteTree` panic-stub (landed by P1.M1.T2.S1) with a real
  implementation that delegates to the verified `run()` helper. Signature is fixed by the `Git`
  interface: `WriteTree(ctx) (sha string, err error)`. It runs `git -C <repo> write-tree`, which
  serializes the current index into a tree object and prints its SHA — a read-only-with-respect-to-
  refs operation (it does NOT modify the index or HEAD, per PRD §13.2). On success return the trimmed
  stdout (the TREE_SHA). On exit != 0 (128 on git 2.x, caused by unresolved merge conflicts — unmerged
  stage 1/2/3 entries — in the index) return a descriptive error naming "unresolved merge conflicts"
  with the exit code and trimmed stderr. Adds ONE new test file `internal/git/writetree_test.go`
  (package git) covering staged-files (happy path), empty-index (edge), merge-conflict (the core
  failure mode), git-missing, and context-cancelled, plus a `makeMergeConflict` fixture helper. Also
  removes the single `WriteTree` line from `git_test.go`'s `TestStubsPanic` (required consequence of
  making the method real — mirrors S2's RevParseHEAD removal). NO import changes (`fmt`/`strings`
  already imported by S1/S2). Touches ONLY `internal/git/`; no interface, struct, `run()`, or other-
  method changes.
---

## Goal

**Feature Goal**: Implement the second real git plumbing method on `*gitRunner` — `WriteTree` — the
immutable-snapshot primitive at the heart of Stagecoach's atomic-commit flow (PRD §13.2 step 1). It
freezes the *current index* into a permanent tree object (printing its SHA) **without touching the
index or HEAD**, so that downstream `CommitTree` (P1.M1.T2.S4) and the rescue protocol (P1.M3.T3) can
refer to "what was staged at time T" regardless of what the user does to the index afterward. The
implementation MUST detect the one failure mode that matters — unresolved merge conflicts in the
index (unmerged stage 1/2/3 entries, which make `write-tree` refuse to write) — and surface it as a
clear, descriptive error rather than an opaque subprocess failure.

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: replace the `WriteTree` panic-stub body with the ~6-line
   delegation to `run()` (exact body in §Blueprint). **No import changes** — `fmt` and `strings` are
   already imported (S1 added `fmt`; S2 added `strings`).
2. **CREATE** `internal/git/writetree_test.go` (`package git`): five test functions
   (`TestWriteTree_StagedFiles`, `TestWriteTree_EmptyIndex`, `TestWriteTree_MergeConflict`,
   `TestWriteTree_GitBinaryMissing`, `TestWriteTree_ContextCancelled`) plus one fixture helper
   (`makeMergeConflict`).
3. **MODIFY** `internal/git/git_test.go`: remove the single `assertPanics(t, "WriteTree", …)` line
   from `TestStubsPanic` (required now that `WriteTree` is real — the test would otherwise fail
   expecting a panic that no longer occurs).

No other files touched. No new dependencies (stdlib only). `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 5 new `WriteTree` cases passing (plus S1's `run()`
tests, S2's `RevParseHEAD` tests, and the now-trimmed `TestStubsPanic` all still green); `WriteTree`
on a repo with staged files returns a 40-hex SHA matching `^[0-9a-f]{40,64}$`; `WriteTree` on a repo
whose index has unresolved merge conflicts returns a non-nil error whose message contains
`"unresolved merge conflicts"`; `WriteTree` on an empty/unborn index returns the canonical empty-tree
SHA `4b825dc642cb6eb9a060e54bf8d69288fbee4904`; a missing git binary surfaces as a non-nil error
(NOT misread as a conflict or success).

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary caller), and the sibling
plumbing subtask S4 (`CommitTree`) and rescue subtask P1.M3.T3 — the immediate consumers of the
`TREE_SHA` that `WriteTree` returns.

**Use Case**: Immediately after confirming something is staged (and before kicking off message
generation), the orchestrator calls `treeSHA, err := g.WriteTree(ctx)` to freeze the index into an
immutable snapshot. If `err != nil` (conflict), it aborts with a diagnostic before generation. If it
succeeds, `treeSHA` is handed to `CommitTree` later and, on generation failure, into the rescue
message (§18.3) so the user can recover the originally-staged files manually.

**User Journey**: `g := git.New(repoPath)` → `tree, err := g.WriteTree(ctx)` → if `err != nil`: abort
("Resolve merge conflicts first." — §18.2, exit 1); else: pass `tree` to generation, then
`CommitTree(ctx, tree, parents, msg)`, and stash `tree` for the rescue path.

**Pain Points Addressed**: Decouples *what gets committed* (the index at snapshot time) from *when the
commit happens* (whenever the model finishes, tens of seconds later) — the central design argument of
PRD §13.1. Without a read-only `write-tree`, the tool would have to call `git commit` (which reads the
index at commit time, racing with the user's continued staging). With it, `TREE_SHA` is a permanent,
immutable record that survives any subsequent index mutation.

## Why

- **PRD §13.2 (The plumbing alternative):** *"`git write-tree` serializes the current index into a
  tree object and prints its SHA. Crucially, this does not modify the index or HEAD. It is a pure,
  read-only-with-respect-to-refs operation that freezes a copy of the staging area into the object
  store. After this call, `TREE_SHA` refers to a permanent, immutable record of 'what was staged at
  time T'."* This subtask IS that primitive.
- **PRD §18.2 (Failure modes):** the row *"Merge conflicts in index / When: `write-tree` / Response:
  'Resolve merge conflicts first.' / Exit 1"* — `WriteTree`'s error is the signal that triggers this
  abort-before-generation response.
- **PRD §18.3 (The rescue message):** the rescue path fires *"When `TREE_SHA` is set and `NEW_SHA` is
  not"* — `WriteTree` is what produces `TREE_SHA`. A correct, conflict-detecting `WriteTree` is the
  precondition for the entire rescue protocol.
- **`git_plumbing_summary.md` Exit-Code Cheat Sheet:** the row for `git write-tree`: `exit 0 =
  TREE_SHA`; `exit 128 = conflict in index`; note *"abort before generation"*. This is the one-row spec.
- **`git_plumbing_reference.md` §1:** the canonical Go pattern for write-tree + the conflict-failure
  analysis. §1 states: *"The stable signal is exit code ≠ 0 plus the word 'merge' or 'needs merge'
  on stderr. Do not rely on a single exact phrase."* — this drives the `code != 0` branch and the
  decision NOT to substring-match a specific phrase.
- **Foundation for S4 / P1.M3.T3:** `CommitTree` (S4) takes the returned `treeSHA` as its `<tree>`
  argument; the rescue protocol (P1.M3.T3) embeds it in the manual-recovery command. Both are blocked
  on a correct `WriteTree`. This is the second of the 11 interface methods to leave stub-status,
  reusing and validating the `run()`-delegation pattern S2 established.

## What

`WriteTree` runs `git -C <workDir> write-tree` via the existing `run()` helper and translates the
four-tuple into `(sha, err)`:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `("", err)`. This is the ONLY path that returns a non-nil error for infrastructural reasons.
- `exitCode != 0` (128 on git 2.x — unresolved merge conflicts in the index) → return
  `("", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr)))`.
  The message names "unresolved merge conflicts" (per the work-item contract) and includes the exit
  code and trimmed stderr (whose real text is `"<path>: unmerged (…)"` / `"fatal: git-write-tree: error
  building trees"` on git 2.54.0) for debuggability.
- `exitCode == 0` → return `(strings.TrimSpace(stdout), nil)`. The trimmed 40-hex (sha-1) or 64-hex
  (sha-256) TREE_SHA.

No porcelain, no `--missing`, no tree-SHA format validation in production code (the contract mandates
plain `write-tree` and a return of the trimmed stdout on success).

### Success Criteria

- [ ] `(*gitRunner).WriteTree` body matches §Implementation Blueprint verbatim (no `panic`).
- [ ] NO import changes to `git.go` (`fmt` and `strings` are already present from S1/S2).
- [ ] `internal/git/writetree_test.go` exists in `package git` with the 5 named test functions.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `WriteTree` line (removed — see Level-2 note).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all 5 `TestWriteTree_*` pass and S1/S2's tests still pass.
- [ ] `TestWriteTree_MergeConflict` asserts `err != nil` and the message contains
      `"unresolved merge conflicts"` on an index with unmerged entries.
- [ ] `TestWriteTree_StagedFiles` asserts the returned SHA matches `^[0-9a-f]{40,64}$`.
- [ ] NO change to `run()`, `New`, `gitRunner`, the `Git` interface, `FileChange`, `StagedDiffOptions`,
      or any of the other 9 method stubs.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` (inherited from S1; `WriteTree` adds none).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact three files to touch (and the
exact single line to remove from `git_test.go`); the exact `run()` contract being delegated to
(signatures + the `err==nil`-for-non-zero-exits invariant); the exact `WriteTree` body (verified-
equivalent to a throwaway invocation run against git 2.54.0); the empirically-pinned conflict behavior
(exit 128, stderr `"<path>: unmerged (…)"` / `"fatal: git-write-tree: error building trees"`); the
exact 5 test cases with verified assertions and the `makeMergeConflict` fixture (branch-name-agnostic
via `git checkout -`); and the exact validation commands with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§13.2 (the plumbing alternative: write-tree is read-only w.r.t. refs, the immutable-snapshot
        primitive) is the behavior this method implements; §13.1 (why git commit is the wrong
        primitive) is the motivation; §18.2 (failure modes: 'Merge conflicts in index' at write-tree
        → 'Resolve merge conflicts first', exit 1) is the abort semantics; §18.3 (rescue message fires
        when TREE_SHA is set) is why a correct WriteTree is the precondition for rescue; §19 (security:
        args as []string, never sh -c) is inherited from run() and must not be violated."
  critical: "This subtask owns ONLY WriteTree's body + its tests + the one-line TestStubsPanic edit.
             Do NOT implement CommitTree (S4), UpdateRefCAS (S5), the rescue protocol (P1.M3.T3), or any
             other method. Do NOT change the Git interface (it is already correct from S1)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Atomic Commit Sequence (step 3 = write-tree → TREE_SHA, 'fails exit 128 if conflicts in
        index') and the Exit-Code Cheat Sheet row for `git write-tree` (exit 0 = TREE_SHA; exit 128 =
        conflict in index; 'abort before generation'). This is the one-row spec."
  critical: "Confirms exit 128 is the conflict signal and that run()'s err=nil-for-non-zero-exits
             invariant (S1 gotcha G2) is what makes the exitCode branch reachable. Also the
             Cross-Platform Notes (always `git -C <repo>`, separate stdout/stderr buffers, args as
             []string) — all inherited from run(), none re-added."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§1 (`git write-tree`) documents exact semantics, the conflict failure mode, and the canonical
        Go pattern. §1's KEY guidance: 'The stable signal is exit code ≠ 0 plus the word merge or needs
        merge on stderr. Do not rely on a single exact phrase.' This drives the `code != 0` branch."
  critical: "§1's *representative* conflict stderr ('error: cannot write tree object' / 'fatal: <path>:
             needs merge') does NOT match git 2.54.0's actual bytes ('<path>: unmerged (…)' / 'fatal:
             git-write-tree: error building trees'). §1 explicitly warns stderr phrasing drifts across
             versions. Branch on exitCode, NOT on a stderr substring (see gotcha G4)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 1 (unborn HEAD) and FINDING 6 (diff --quiet inverted exits) are the sibling traps S2/T3
        encode; they confirm run()'s 'non-zero exit is not a Go error' invariant that WriteTree relies
        on. There is no dedicated write-tree finding, but the write-tree conflict behavior is pinned in
        THIS subtask's own research (see below)."
  critical: "Confirms run()'s err==nil-for-128 invariant is the foundation WriteTree builds on."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything WriteTree consumes: the gitRunner struct, the run() helper (exact
        signature and verified body), the WriteTree panic-stub being replaced, the Git interface
        (signature already correct), New(), and the git_test.go initRepo(t,dir) helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1's gotchas G2/G3 are the
             foundation."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The CONTRACT for the pattern to MIRROR: RevParseHEAD is the first real run()-delegating method,
        and WriteTree is structurally identical (err-first guard → code branch → trimmed-stdout return).
        S2 also added the `strings` import that WriteTree reuses — so WriteTree needs NO import change.
        S2's revparse_test.go defines `minGitEnv()` and `makeEmptyCommit()`, which WriteTree's tests
        reuse without redeclaring."
  critical: "S2's Level-2 note documents the EXACT TestStubsPanic-edit pattern (remove the now-real
             method's line) — apply the same one-line removal for WriteTree. S2's gotcha G7 (test file
             is `package git`, white-box) and G8 (reuse initRepo, don't redeclare) apply identically."

- docfile: plan/001_f1f80943ac34/P1M1T2S3/research/write_tree_validation.md
  why: "THIS subtask's own research: the exact WriteTree body, the empirically-pinned write-tree
        behavior (empty index → 4b825dc…; staged → 40-hex; conflict → exit 128 with 'unmerged'/'error
        building trees'), the conflict-fixture design (branch-agnostic via `git checkout -`), and the
        test matrix."
  critical: "§2.1 is the single most important finding: git 2.54.0's conflict stderr differs from the
             architecture doc's representative text, which is WHY we branch on exitCode ≠ 0 and name
             'unresolved merge conflicts' ourselves rather than matching a phrase. §5 documents the
             TestStubsPanic edit. §7 is the decisions log (D1: code != 0 not == 128; D2: no phrase match)."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-write-tree#_description
  why: "Documents that write-tree 'creates a tree object using the current index' and prints its SHA;
        on an empty index it writes the well-known empty tree (4b825dc642cb6eb9a060e54bf8d69288fbee4904
        for sha-1). Confirms it does not touch refs."
  critical: "Establishes `git write-tree` (no flags) is the contractually-mandated form; do not add
             `--missing` or other flags."
- url: https://git-scm.com/docs/git-write-tree#_description
  why: "The same page documents the conflict failure: write-tree 'will refuse to write a new tree
        object' when the index has unmerged entries, exiting non-zero."
  critical: "Confirms exit ≠ 0 is the conflict signal (not a specific code or phrase)."
- url: https://pkg.go.dev/strings#TrimSpace
  why: "TrimSpace removes the trailing '\\n' from git's single-line tree-SHA output (and trims stderr
        in the error path)."
  critical: "`strings` is already imported (S2); no new import. The trimmed SHA is what CommitTree (S4)
             passes as <tree> and what the rescue message embeds."
- url: https://pkg.go.dev/context#Canceled
  why: "The error value run() returns (via ctx.Err()) when the context is cancelled before/during the
        git call; WriteTree propagates it unchanged."
  critical: "The ctx-cancel test asserts errors.Is(err, context.Canceled) — confirm it is the unwrapped ctx.Err()."
- url: https://git-scm.com/docs/git-checkout#Documentation/git-checkout.txt-emgitcheckoutem
  why: "Documents `git checkout -` / `git checkout @{-1}` = switch to the previous branch. Used in the
        makeMergeConflict fixture to return to the original branch without knowing its name."
  critical: "Makes the conflict fixture robust to init.defaultBranch = main vs master (verified working)."
```

### Current Codebase Tree (after S1 + S2 have landed — verified on disk)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface + gitRunner + run() + New() + stubs; S2: RevParseHEAD real
│       │                 # imports: bytes, context, errors, fmt, os/exec, strings  ← strings ALREADY HERE
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic(10 methods, incl WriteTree) + initRepo
│       └── revparse_test.go  # S2: 4 TestRevParseHEAD_* + minGitEnv + makeEmptyCommit
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go            # MODIFIED — WriteTree stub → real body (NO import change)
        ├── git_test.go       # MODIFIED — remove the ONE `WriteTree` line from TestStubsPanic
        ├── revparse_test.go  # UNCHANGED (S2's file; minGitEnv/makeEmptyCommit reused, not edited)
        └── writetree_test.go # NEW — package git; 5 TestWriteTree_* + makeMergeConflict
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | Replace the `WriteTree` panic-stub with the `run()`-delegating body. Nothing else (no import change). |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "WriteTree", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/writetree_test.go` | CREATE | `package git` tests for `WriteTree`: staged / empty-index / conflict / git-missing / ctx-cancelled, plus the `makeMergeConflict` fixture. |

**Explicitly NOT created/modified:** `run()`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
`StagedDiffOptions`, the other 9 method stubs, `revparse_test.go` (S2's tests), `go.mod`/`go.sum`, the
`Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — NO import changes needed): S1 imported bytes, context, errors, fmt, os/exec; S2
// added strings (RevParseHEAD uses strings.TrimSpace). WriteTree uses ONLY fmt.Errorf and
// strings.TrimSpace — both already imported. Do NOT touch the import block. (Contrast with S2, whose
// one non-obvious edit was adding strings; this subtask has no such edit.)

// CRITICAL (G2 — run()'s invariant, inherited from S1/S2): run() returns err == nil for NON-ZERO git
// exits (1, 128, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O) set
// err != nil, with exitCode == -1. Therefore WriteTree MUST check `err != nil` FIRST (authoritative,
// covers the -1 cases), THEN branch on exitCode. A missing binary (code=-1, err!=nil) is surfaced as a
// real error BEFORE the `code != 0` conflict branch runs — so a LookPath miss is never misreported as a
// conflict. TestWriteTree_GitBinaryMissing guards against regressing this.

// CRITICAL (G3 — TestStubsPanic must drop WriteTree): git_test.go's TestStubsPanic STILL includes
// `assertPanics(t, "WriteTree", func() { _, _ = g.WriteTree(ctx) })`. Once WriteTree is real (no
// panic), assertPanics fails with "expected panic, but did not panic". Resolution (mirrors S2's
// RevParseHEAD removal): DELETE that one line. This is the ONLY edit to git_test.go. After removal,
// TestStubsPanic covers the remaining 9 stubs. Document it in the commit message.

// CRITICAL (G4 — conflict stderr phrasing differs from the architecture doc): git_plumbing_reference.md
// §1 shows the REPRESENTATIVE conflict stderr as 'error: cannot write tree object' / 'fatal: <path>:
// needs merge'. On git 2.54.0 the ACTUAL bytes are '<path>: unmerged (<sha>)' (3 lines, one per stage)
// and 'fatal: git-write-tree: error building trees', exit 128. The word "merge" IS present (inside
// "unmerged"), so the contract's heuristic still matches — BUT do NOT substring-match a specific
// phrase in production code. Branch on `code != 0` (the stable signal per §1) and name "unresolved
// merge conflicts" in OUR message. The test asserts on OUR message, not git's stderr bytes.

// GOTCHA (G5 — reuse S1/S2 helpers, do not redeclare): initRepo(t,dir) lives in git_test.go (package
// git); minGitEnv() and makeEmptyCommit(t,dir,msg) live in revparse_test.go (package git). All three
// are in scope for writetree_test.go (same package) — REUSE them directly. Do NOT redeclare any of
// them (a redeclaration is a compile error). The ONE new helper is makeMergeConflict(t,dir) — a
// distinct name, no collision.

// GOTCHA (G6 — conflict fixture must be branch-name-agnostic): init.defaultBranch is 'main' on this
// box but may be 'master' elsewhere. The makeMergeConflict fixture returns to the original branch via
// `git checkout -` (previous-branch), NOT via a hardcoded branch name. Verified: after `git checkout
// -b side` from branch X, `git checkout -` returns to X. (Do NOT use `git checkout - <name>` — that is
// invalid syntax; use `git checkout -` alone, or `git checkout <name>` with the captured name.)

// GOTCHA (G7 — empty-tree SHA constant): on an empty index (unborn repo, nothing staged) write-tree
// exits 0 and prints the canonical sha-1 empty-tree object id 4b825dc642cb6eb9a060e54bf8d69288fbee4904.
// TestWriteTree_EmptyIndex asserts equality with this constant (test repos are sha-1 by default here).
// For a sha-256 repo the constant differs; if that test ever fails on a sha-256 box, re-pin to the
// observed value and note the object format. (The happy-path test uses the hex regex, robust to either.)

// GOTCHA (G8 — no tree-SHA validation in production code): the contract says on exit 0 return the
// trimmed stdout. Do NOT add a hex-length check or parsing to WriteTree — that would deviate from the
// contract and is unnecessary (downstream commit-tree will reject a bad tree). The `^[0-9a-f]{40,64}$`
// regex is TEST-ONLY (sanity-check of git's own contract; 40=sha-1, 64=sha-256).

// GOTCHA (G9 — test file is package git, white-box): WriteTree is on *gitRunner and calls the
// unexported run(); New() returns the Git interface but the meaningful fixture work needs `package git`
// to call New() and exercise the real *gitRunner path, and to reuse initRepo/minGitEnv/makeEmptyCommit.
// Match S1's git_test.go and S2's revparse_test.go package (gotcha carried from S2 G7).

// GOTCHA (G10 — merge fixture's `git merge` exiting non-zero is EXPECTED): makeMergeConflict ends with
// `git merge side`, which exits non-zero BECAUSE of the conflict — that is the intended outcome, not a
// fixture failure. Assert on `err == nil` (merge succeeded cleanly) and t.Fatalf in THAT case; ignore
// the non-zero exit. After it, `git ls-files -u` shows 3 unmerged entries and write-tree exits 128.

// GOTCHA (G11 — no shell, no cmd.Dir): WriteTree inherits S1's §19 guarantees because it only calls
// run(). Do NOT introduce exec.Command / os.Chdir / sh -c anywhere in the new code (the makeMergeConflict
// helper DOES use exec.Command directly — that is the TEST fixture, parallel to S1's initRepo and S2's
// makeEmptyCommit, and is fine because it sets `cmd.Env` and uses []string args, never a shell).
// The Level-3 grep for sh -c / cmd.Dir covers production code (git.go); the test fixture's exec.Command
// is acceptable (it is how every existing test helper drives git).
```

## Implementation Blueprint

### Data models and structure

None added or changed. `WriteTree`'s return types (`string, error`) are already declared in the `Git`
interface by S1. No new structs, no new options type.

### The `WriteTree` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature.

```go
// WriteTree materializes the current index into a tree object and returns its SHA. It is a
// read-only-with-respect-to-refs operation: it writes a tree object to the object store but does
// NOT modify the index or HEAD (PRD §13.2). It is the immutable-snapshot primitive consumed by
// CommitTree (P1.M1.T2.S4) and the rescue protocol (P1.M3.T3).
//
// write-tree fails (non-zero exit, 128 on git 2.x) when the index has unresolved merge conflicts
// (unmerged stage 1/2/3 entries). That is surfaced here as run()'s exitCode != 0 (err stays nil per
// run()'s invariant); the error names "unresolved merge conflicts" and includes the trimmed stderr,
// whose text contains "unmerged"/"error building trees" on a real conflict (git_plumbing_reference
// §1: the stable signal is exit ≠ 0; do NOT match a single exact stderr phrase).
func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "write-tree")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}
```

> **Verified:** the branch order and the `code != 0 ⟹ conflict error` mapping are confirmed by this
> subtask's research §2 (conflict: `exitCode=128, err=nil`, stderr `"<path>: unmerged (…)"` /
> `"fatal: git-write-tree: error building trees"`) and by the `run()` invariant pinned in S1's
> `run_helper_validation.md` §4, re-verified empirically on this box (git 2.54.0).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (one surgical edit)
  - FIND the stub:
      func (g *gitRunner) WriteTree(ctx context.Context) (string, error) {
          panic("gitRunner.WriteTree: not yet implemented — see P1.M1.T2.S3")
      }
  - REPLACE with the body in §"The WriteTree body" above (keep the same signature, add the doc comment).
  - DO NOT touch the import block (fmt + strings already present — gotcha G1).
  - DO NOT touch: run(), New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD
    (already real from S2), or any of the other 9 method stubs.
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic:
      assertPanics(t, "WriteTree", func() { _, _ = g.WriteTree(ctx) })
  - DELETE that single line (and any now-trailing separator). After removal TestStubsPanic covers the
    remaining 9 stubs: CommitTree, UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges,
    RecentMessages, RecentSubjects, CommitCount, AddAll.
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper).
  - WHY: once WriteTree is real it no longer panics; assertPanics would fail (gotcha G3). Mirrors S2's
    RevParseHEAD removal.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (9 stubs still panic).

Task 3: CREATE internal/git/writetree_test.go (package git — white-box)
  - FILE: internal/git/writetree_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G9; matches git_test.go / revparse_test.go)
  - IMPORTS: context, errors, os, os/exec, path/filepath, regexp, strings, testing
  - WRITE the fixture helper makeMergeConflict(t, dir) (exact body in research §4.1):
      - Uses a local runGit(args...) closure that execs `git -C dir <args...>` with env =
        append(minGitEnv(), GIT_AUTHOR_*/GIT_COMMITTER_* identity). (minGitEnv reused from
        revparse_test.go — gotcha G5.)
      - Uses a local writeFile(name, body) closure (os.WriteFile + filepath.Join).
      - Sequence: write conflict.txt="base"; add; commit "base"; checkout -b side; write "side";
        commit -am "side-change"; checkout "-" (previous branch — gotcha G6); write "main";
        commit -am "main-change"; git merge side (expect non-zero — gotcha G10; t.Fatalf if it
        SUCCEEDS cleanly).
      - t.Helper().
  - WRITE the 5 test functions (assertions in §"Test cases" below):
      TestWriteTree_StagedFiles:
        repo := t.TempDir(); initRepo(t, repo)
        write a file via os.WriteFile(filepath.Join(repo,"a.txt"), []byte("hello\n"), 0o644)
        stage it: exec.Command("git","-C",repo,"add","a.txt") with env append(minGitEnv(), identity...)
        g := New(repo)
        sha, err := g.WriteTree(context.Background())
        assert err == nil
        assert regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha)   // 40=sha-1 / 64=sha-256
      TestWriteTree_EmptyIndex:
        repo := t.TempDir(); initRepo(t, repo)   // no commits, nothing staged
        g := New(repo)
        sha, err := g.WriteTree(context.Background())
        assert err == nil
        assert sha == "4b825dc642cb6eb9a060e54bf8d69288fbee4904"   // canonical sha-1 empty tree (gotcha G7)
      TestWriteTree_MergeConflict:
        repo := t.TempDir(); initRepo(t, repo); makeMergeConflict(t, repo)
        g := New(repo)
        sha, err := g.WriteTree(context.Background())
        assert err != nil
        assert strings.Contains(err.Error(), "unresolved merge conflicts")   // ← THE contract assertion
        assert sha == ""
      TestWriteTree_GitBinaryMissing:
        t.Setenv("PATH", "")                              // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                             // dir need not be a repo (LookPath fails first)
        sha, err := g.WriteTree(context.Background())
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert sha == ""                                  // ← guard: LookPath miss NOT misread as conflict
      TestWriteTree_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call (deterministic)
        g := New(t.TempDir())
        sha, err := g.WriteTree(ctx)
        assert err != nil && errors.Is(err, context.Canceled)
        assert sha == ""
  - NAMING: TestWriteTree_<Scenario>; helper makeMergeConflict (distinct from S1/S2 helpers — gotcha G5).
  - DO NOT redeclare initRepo / minGitEnv / makeEmptyCommit (they live in git_test.go / revparse_test.go).
  - VERIFY: go test -race -run 'TestWriteTree' ./internal/git/ → exit 0, all 5 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git grep -n 'panic.*WriteTree' internal/git/git.go                       (expect: no matches — stub gone)
  - RUN: git status --porcelain → expect EXACTLY:
        M internal/git/git.go
        M internal/git/git_test.go
        ?? internal/git/writetree_test.go
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestWriteTree_StagedFiles` | `initRepo` + write+`git add` a.txt | `err==nil && sha` matches `^[0-9a-f]{40,64}$` | Happy path: index → 40-hex TREE_SHA, trimmed |
| `TestWriteTree_EmptyIndex` | `initRepo` only (unborn, nothing staged) | `err==nil && sha=="4b825dc642cb6eb9a060e54bf8d69288fbee4904"` | Edge: works on fresh repo; empty index → canonical empty-tree SHA |
| `TestWriteTree_MergeConflict` | `initRepo` + `makeMergeConflict` | `err!=nil` contains `"unresolved merge conflicts"`; `sha==""` | **The core failure mode**: conflict → descriptive error (exit ≠ 0) |
| `TestWriteTree_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found"; `sha==""` | `run()`'s err path is propagated, not misread as conflict |
| `TestWriteTree_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `sha==""` | ctx.Err() surfaced (not exit 0/128) |

### Implementation Patterns & Key Details

```go
// === Why branch on `code != 0` (not `code == 128`) ===
// The work-item contract says verbatim: "On exit != 0, return a descriptive error mentioning
// 'unresolved merge conflicts'." git_plumbing_reference.md §1: "The stable signal is exit code ≠ 0
// … Do not rely on a single exact phrase." write-tree's only documented non-zero exit is the conflict
// case (128 on git 2.x), so treating any non-zero exit as a conflict precondition failure is correct
// and slightly more future-proof than pinning exactly 128. RevParseHEAD (S2) pinned == 128 because
// unborn has a distinct 128-only meaning; write-tree has no such ambiguity.

// === Why err is checked BEFORE code (the branch order) ===
// run() guarantees: err != nil  ⟹  exitCode == -1  (LookPath / context / start failure).
//                   err == nil   for every real git exit (0, 128, …).
// So `if err != nil { return "", err }` is the authoritative infrastructural-failure guard. Only when
// err == nil does `code != 0` decide conflict-vs-success. A missing git binary (code=-1, err!=nil) is
// thus NEVER misreported as a conflict — guarded by TestWriteTree_GitBinaryMissing.

// === Why we include the trimmed stderr in the conflict message ===
// On git 2.54.0 the stderr is "<path>: unmerged (<sha>)" / "fatal: git-write-tree: error building
// trees". Including it (trimmed) gives the user/debugger the exact failing path and git's phrasing
// without coupling our detection to it. The contract requires the message MENTION "unresolved merge
// conflicts" (which our literal text satisfies); the stderr is supplementary.

// === Why strings.TrimSpace on stdout (success case) ===
// git prints "<tree-sha>\n" (trailing newline). TrimSpace yields the bare 40/64-hex SHA that
// CommitTree (S4) passes as <tree> and the rescue message embeds. Untrimmed, those args would carry
// a "\n" and fail. (Verified: git emits LF even on Windows; no \r\n handling needed.)

// === Why the ctx-cancel test cancels BEFORE the call ===
// Pre-cancelling makes cmd.Run() fail deterministically; run()'s ctx.Err()-before-errors.As path
// returns (-1, context.Canceled) reliably. Cancelling mid-run would race with a fast write-tree.

// === Reusing initRepo / minGitEnv / makeEmptyCommit from the existing test files ===
// Because writetree_test.go is `package git`, all three helpers are in scope. Call them directly; do
// NOT redefine them (a redeclaration is a compile error — gotcha G5). makeMergeConflict is the ONE
// new helper, with a distinct name. It uses exec.Command directly (parallel to initRepo/makeEmptyCommit)
// with a minimal env (minGitEnv + identity) and []string args (no shell).
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, strings, regexp, os.WriteFile (1.16+), path/filepath, t.Setenv (1.17+),
    errors.Is all available
  - deps: NONE added (stdlib only)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go             # MODIFIED: WriteTree body (NO import change)
  - file: internal/git/git_test.go        # MODIFIED: remove the WriteTree line from TestStubsPanic
  - file: internal/git/writetree_test.go  # NEW: package git, 5 tests + makeMergeConflict

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M1.T2.S4 (CommitTree): receives WriteTree's returned treeSHA as its <tree> argument
  - P1.M3.T3 (Rescue protocol): embeds the treeSHA in the manual-recovery command
    (git commit-tree -p <PARENT_SHA> -m "msg" <TREE_SHA> | xargs git update-ref HEAD)
  - P1.M3.T4 (CommitStaged orchestrator): the primary caller —
    treeSHA, err := g.WriteTree(ctx); if err != nil { abort "Resolve merge conflicts first" (exit 1) }

PARALLEL-EXECUTION NOTE:
  - S1 (git.go + git_test.go) and S2 (revparse_test.go + RevParseHEAD body) have BOTH landed on disk.
    This subtask EDITS git.go in exactly one spot (the WriteTree method body), EDITS git_test.go in
    exactly one spot (the TestStubsPanic WriteTree line), and ADDS a separate writetree_test.go — no
    overlap with S1/S2's other content. The edit anchors (the WriteTree stub text, the TestStubsPanic
    WriteTree line) are quoted verbatim above. If git.go does not yet contain the WriteTree stub at
    edit time, S1 has not landed; in that case the edits apply once it does.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...       # Expected: exit 0, no warnings (e.g. no unused import, no shadowing)
go build ./internal/git/        # Expected: exit 0 (package compiles; NO new imports needed)
go build ./...                  # Expected: exit 0 (whole module compiles)

# Expected: zero output/errors. Unlike S2, there is NO `undefined: strings` risk — WriteTree uses only
# fmt + strings, both already imported (gotcha G1). If `go build` complains about strings/fmt, S1 or S2
# did not land correctly; re-read internal/git/git.go's import block.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v -run 'TestWriteTree' ./internal/git/   # Expected: 5 tests PASS, exit 0
# Must see: TestWriteTree_StagedFiles, TestWriteTree_EmptyIndex, TestWriteTree_MergeConflict,
#           TestWriteTree_GitBinaryMissing, TestWriteTree_ContextCancelled — all ok.

go test -race -run 'TestStubsPanic' ./internal/git/     # Expected: PASS, exit 0
# After removing the WriteTree line, TestStubsPanic covers the remaining 9 stubs and must still pass.

go test -race ./internal/git/    # Expected: exit 0 — S1's run() tests, S2's RevParseHEAD tests,
                                 # TestStubsPanic (9 stubs), AND the 5 new WriteTree tests all pass.

make test                        # Expected: exit 0 (target = go test -race ./...)
```

> **Note on the `TestStubsPanic` edit (gotcha G3):** `git_test.go`'s `TestStubsPanic` currently
> asserts a panic for `WriteTree`. Once `WriteTree` is real (no panic), `assertPanics` fails with
> "expected panic, but did not panic". The REQUIRED fix (Task 2) is to remove the single
> `assertPanics(t, "WriteTree", …)` line — this is an allowed exception to "don't touch git_test.go"
> because it is the direct, necessary consequence of implementing WriteTree, and a permanently-failing
> suite is worse. This mirrors exactly what S2 did for RevParseHEAD. Document the removal in the commit
> message. If `TestStubsPanic` uses a static per-method list (it does — one `assertPanics` line each),
> the removal is a one-line deletion. Do NOT touch the other 9 lines.

### Level 3: Security & Structural Invariants (the §19 enforcement + scope discipline)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution anywhere in the git wrapper PRODUCTION code (inherited; new code adds none).
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output. (writetree_test.go's makeMergeConflict uses exec.Command directly with
# []string args + cmd.Env, never a shell — parallel to S1's initRepo and S2's makeEmptyCommit; that
# is acceptable test-fixture usage. If you want belt-and-suspenders, the same grep over writetree_test.go
# also returns nothing, because the fixture uses []string args, not sh -c.)

# No os.Chdir / cmd.Dir in PRODUCTION code (inherited; new code adds none).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output.

# The WriteTree stub is gone (replaced by a real body).
git grep -n 'WriteTree' internal/git/git.go
# Expected: the interface method comment + the func declaration + body; NO line matching
# `panic("gitRunner.WriteTree`.

# git.go import block UNCHANGED (no new imports).
git diff internal/git/git.go | grep -E '^\+\s*"'
# Expected: NO output (no added import lines). The only added lines are the WriteTree body.

# Only the intended files changed.
git status --porcelain
# Expected EXACTLY:
#   M internal/git/git.go
#   M internal/git/git_test.go
#   ?? internal/git/writetree_test.go

# go.mod / go.sum untouched.
git diff --name-only go.mod go.sum
# Expected: NO output.
```

### Level 4: Runtime Smoke Test (prove WriteTree works against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the staged/empty/conflict behavior the tests assert, against the real binary:
tmp=$(mktemp -d); git -C "$tmp" init -q

# empty index → canonical empty tree
git -C "$tmp" write-tree; echo "empty EXIT=$?"     # expect 4b825dc642cb6eb9a060e54bf8d69288fbee4904, EXIT=0

# staged file → 40-hex
echo hello > "$tmp/a.txt"; git -C "$tmp" add a.txt
git -C "$tmp" write-tree; echo "staged EXIT=$?"    # expect 40-hex SHA, EXIT=0

# conflict → exit 128
git -C "$tmp" -c user.name=t -c user.email=t@t commit -q -m base
git -C "$tmp" checkout -q -b side; echo side > "$tmp/a.txt"; git -C "$tmp" commit -q -am side
git -C "$tmp" checkout -q -; echo main > "$tmp/a.txt"; git -C "$tmp" commit -q -am main
git -C "$tmp" merge side >/dev/null 2>&1; echo "merge exit (conflict expected)=$?"
git -C "$tmp" write-tree; echo "conflict EXIT=$?"  # expect 'unmerged'/'error building trees', EXIT=128
rm -rf "$tmp"
# If the conflict exit is not 128 (e.g. a different git version), the implementation is still correct
# (it branches on `code != 0`, not on 128 specifically); only TestWriteTree_EmptyIndex's constant and
# the research's pinned stderr would need re-checking.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (no new imports needed — fmt + strings already present).
- [ ] `go test -race ./internal/git/` exits 0 (5 new `TestWriteTree_*` + S1/S2 tests + trimmed TestStubsPanic pass).
- [ ] `make test` exits 0.

### Feature Validation

- [ ] `(*gitRunner).WriteTree` body matches §Blueprint (delegates to `run()`, branches on `code != 0`).
- [ ] On a repo with staged files: returns `(40-hex-sha, nil)` (trimmed, valid hex).
- [ ] On an empty/unborn index: returns `(4b825dc642cb6eb9a060e54bf8d69288fbee4904, nil)`.
- [ ] On an index with unresolved merge conflicts: returns a non-nil error whose message contains
      `"unresolved merge conflicts"`.
- [ ] On a missing git binary: returns a non-nil error mentioning "git binary not found" (NOT a conflict).
- [ ] On a cancelled context: returns `errors.Is(err, context.Canceled)`.
- [ ] No production-code tree-SHA validation added (regex is test-only).

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] NO `cmd.Dir` / `os.Chdir` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] `run()` is NOT re-implemented or modified (delegated to, unchanged).
- [ ] NO change to the git.go import block (fmt + strings already present from S1/S2).
- [ ] `Git` interface, `gitRunner`, `New`, `FileChange`, `StagedDiffOptions` unchanged.
- [ ] The other 9 method stubs untouched (still panic with their owning-subtask messages).
- [ ] `go.mod` / `go.sum` unchanged (no new deps).
- [ ] Only `internal/git/git.go` (WriteTree body), `internal/git/git_test.go` (one-line TestStubsPanic
      removal), and `internal/git/writetree_test.go` (new) are changed.
- [ ] No signal handling / `SysProcAttr` / process-group code added (that is P1.M4.T2).

### Documentation & Deployment

- [ ] Doc comment on `WriteTree` explains it is read-only w.r.t. refs (PRD §13.2) and that non-zero
      exit means unresolved merge conflicts (surfaces the trimmed stderr; do not phrase-match).
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't substring-match a specific git stderr phrase to detect conflicts (`"needs merge"`,
  `"cannot write tree object"`) — git 2.54.0 actually emits `"<path>: unmerged (…)"` /
  `"fatal: git-write-tree: error building trees"` (gotcha G4). Branch on `run()`'s `exitCode != 0`
  (the stable signal per git_plumbing_reference.md §1) and name "unresolved merge conflicts" in OUR
  message. The test asserts on OUR message, not git's bytes.
- ❌ Don't pin on `code == 128` when the contract says "On exit != 0" — use `code != 0`. write-tree's
  only documented non-zero exit is the conflict case; `code != 0` matches the contract verbatim and is
  future-proof (decision D1).
- ❌ Don't re-implement the `exec.CommandContext` plumbing inside `WriteTree` — delegate to `run()`,
  which already handles LookPath, `-C repo` args, separate buffers, and `errors.As(*exec.ExitError)`.
  Duplicating it breaks the single-shell-out-point invariant and §19.
- ❌ Don't forget to remove the `WriteTree` line from `git_test.go`'s `TestStubsPanic` — once WriteTree
  is real it no longer panics, and `assertPanics` fails (gotcha G3). This is the one required touch to
  git_test.go; mirror S2's RevParseHEAD removal.
- ❌ Don't add any import — `fmt` (S1) and `strings` (S2) are already in `internal/git/git.go`. WriteTree
  uses only those two (gotcha G1). Editing the import block is both unnecessary and out of scope.
- ❌ Don't redeclare `initRepo` / `minGitEnv` / `makeEmptyCommit` in `writetree_test.go` — they already
  live in `git_test.go` / `revparse_test.go` (same package). Reuse them; use the distinct name
  `makeMergeConflict` for the one new helper (gotcha G5).
- ❌ Don't hardcode a branch name (`"main"`/`"master"`) in the conflict fixture — `init.defaultBranch`
  varies. Return to the original branch via `git checkout -` (previous-branch), verified working
  (gotcha G6). Do NOT use `git checkout - <name>` (invalid syntax).
- ❌ Don't treat the conflict-merge's non-zero exit as a fixture failure — `git merge side` exits
  non-zero BECAUSE of the intended conflict; `t.Fatalf` only if it SUCCEEDS cleanly (gotcha G10).
- ❌ Don't add tree-SHA format validation (hex/length checks) to the production method — the contract
  says return the trimmed stdout on exit 0. Keep the regex in the TEST only (gotcha G8).
- ❌ Don't switch to `git write-tree --missing` or any flagged variant — the work-item contract mandates
  plain `write-tree`.
- ❌ Don't write the test as `package git_test` (black-box) — `New()` returns the `Git` interface but
  the fixture work needs `package git` to reuse `initRepo`/`minGitEnv`/`makeEmptyCommit` and exercise
  the real `*gitRunner` path. Match S1/S2's package (gotcha G9).
- ❌ Don't cancel the context mid-run in the cancel test — cancel BEFORE the call for determinism, and
  assert via `errors.Is(err, context.Canceled)`.
- ❌ Don't touch `revparse_test.go` (S2's file) — `minGitEnv`/`makeEmptyCommit` are reused read-only.
- ❌ Don't implement `CommitTree` / `UpdateRefCAS` / the rescue protocol / any other method, and don't
  edit `run()` or the interface — those are other subtasks' deliverables.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: This is a small, fully-specified change that reuses the exact `run()`-delegation pattern
S2 already proved end-to-end with `RevParseHEAD`. The `WriteTree` body is ~6 lines and is verified-
equivalent to the canonical pattern in `git_plumbing_reference.md` §1, re-pinned empirically on this
exact box (git 2.54.0: empty index → `4b825dc642cb6eb9a060e54bf8d69288fbee4904`; staged → 40-hex;
conflict → exit 128, stderr `"<path>: unmerged (…)"` / `"fatal: git-write-tree: error building
trees"`). The single most important design call — branch on `exitCode != 0` and name "unresolved merge
conflicts" ourselves rather than phrase-matching git's version-sensitive stderr — is empirically
justified (the architecture doc's representative stderr does NOT match 2.54.0's actual bytes) and
makes the implementation version-robust. Unlike S2, there is NO import edit (fmt + strings already
present), removing the one mechanical failure mode S2 had. The five test cases each map to a distinct,
verified branch (staged/ empty/ conflict/ LookPath-miss/ ctx-cancel) with deterministic fixtures; the
`makeMergeConflict` fixture is verified working and branch-name-agnostic via `git checkout -`. The one
cross-cutting edit (remove the `WriteTree` line from `TestStubsPanic`) is anticipated with an explicit,
scoped resolution (Task 2 / Level-2 note) mirroring S2, so the agent is not blocked. No external
dependencies, no interface changes, no ambiguity in the contract.
