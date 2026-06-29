---
name: "P1.M1.T3.S2 — HasStagedChanges (diff --cached --quiet exit-code semantics)"
description: |
  Replace the `(*gitRunner).HasStagedChanges` panic-stub (landed by P1.M1.T2.S1) with the real
  implementation — the cheap "is anything staged?" gate that the orchestrator calls before snapshot
  + generate (PRD §9.4/FR16–FR17, Appendix C, FINDING 6). Signature is fixed by the already-landed
  `Git` interface: `HasStagedChanges(ctx) (bool, error)`. It runs `git -C <repo> diff --cached
  --quiet` and translates the exit code: `err != nil` → infrastructural failure (propagate,
  `(false, err)`); `code == 0` → nothing staged, `(false, nil)`; `code == 1` → staged changes
  exist, `(true, nil)`; any other code (`>1`, e.g. 128/129 corrupt-or-not-a-repo) → `(false,
  fmt.Errorf(...))`. The exit codes are **INVERTED from usual convention** (FINDING 6): exit 1 is
  the "has staged" SIGNAL, not an error — a naive `if err != nil` would misread it. `--quiet`
  produces NO stdout (the exit code is the sole signal), so stdout is discarded. Delegates to S1's
  `run()` helper (NOT exec directly — mirrors RevParseHEAD S2 / WriteTree S3 / DiffTree S6);
  `run()`'s invariant (err==nil for every real git exit; exitCode carries the signal; err!=nil ⟺
  LookPath miss / ctx cancel / start-I/O with code=-1) is the foundation for the branch order.
  Empirically pinned on this box (git 2.54.0): clean index → exit 0; staged file → exit 1;
  committed-but-nothing-new-staged → exit 0; non-repo dir → exit 129). Uses a `switch` (case
  0 / case 1 / default) — clearest encoding of the two-value map + catch-all. Adds ZERO new imports
  (`fmt`, `strings` are both already imported in git.go — distinct from S2, which had to add
  `strings`). Adds ONE new test file `internal/git/hasstaged_test.go` (`package git`, white-box)
  covering: nothing-staged (false), staged-file (true), committed-but-nothing-staged (false — proves
  index-vs-HEAD), not-a-repo (error), git-binary-missing (error), ctx-cancelled (error). Reuses
  `initRepo` (git_test.go), `writeFile`/`stageFile` (committree_test.go, S4), `makeEmptyCommit`
  (revparse_test.go, S2) — no new helpers, no helper-name collisions. Also removes the single
  `HasStagedChanges` line from `git_test.go`'s `TestStubsPanic` (required consequence of making the
  method real — mirrors S2/S3/S4/S5/S6 and the concurrently-landing T3.S1 which removes its own
  `StagedDiff` line; the two edits are distinct, non-overlapping lines). Touches ONLY
  `internal/git/`; no interface, struct, `run()`, `runWithInput`, `FileChange`, `StagedDiffOptions`,
  RevParseHEAD, WriteTree, CommitTree, UpdateRefCAS, DiffTree, parseDiffTree, StagedDiff, or the
  remaining 4 method stubs (RecentMessages, RecentSubjects, CommitCount, AddAll). This is the SECOND
  of the six P1.M1.T3 methods (StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects,
  CommitCount, AddAll) to leave stub-status (StagedDiff is T3.S1, landing concurrently — its
  `HasStagedChanges` edit is distinct from this subtask's).
---

## Goal

**Feature Goal**: Implement the `HasStagedChanges` git method on `*gitRunner` — the read-only,
sub-millisecond "is anything staged?" gate that the `CommitStaged` orchestrator (P1.M3.T4) calls
BEFORE taking the immutable snapshot and BEFORE generating a commit message. It answers PRD §9.4 /
FR16 ("If `git diff --cached --quiet` reports no staged changes …") and FR17 ("If after auto-stage
there are still no changes … exit … code 2"). The method MUST translate git's exit code correctly:
**exit 1 means staged changes EXIST (return `true`), exit 0 means nothing staged (return `false`)** —
the inversion from usual convention that a naive `err != nil` check would get wrong (FINDING 6).

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: replace the `HasStagedChanges` panic-stub body with the
   ~12-line `switch`-on-`run()`-exit-code body (exact body in §Blueprint). NO import changes —
   `fmt` and `strings` are both already imported.
2. **MODIFY** `internal/git/git_test.go`: remove the single
   `assertPanics(t, "HasStagedChanges", func() { _, _ = g.HasStagedChanges(ctx) })` line from
   `TestStubsPanic` (required now that HasStagedChanges is real — mirrors S2/S3/S4/S5/S6).
3. **CREATE** `internal/git/hasstaged_test.go` (`package git`): the six-function test matrix below
   (nothing-staged, staged-file, committed-but-nothing-staged, not-a-repo, git-binary-missing,
   ctx-cancelled). No new helpers.

No other files touched. No new dependencies. `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the six new `TestHasStagedChanges_*` cases passing
(plus S1's `run()` tests, S2–S5's plumbing tests, S6's DiffTree tests, and S1-T3.S1's StagedDiff
tests all still green); a fresh repo with nothing staged returns `(false, nil)`; a repo with one
staged `.go` file returns `(true, nil)`; a repo with a commit but nothing NEW staged returns
`(false, nil)` (proving the comparison is index-vs-HEAD, not "anything exists"); a non-repo
directory returns a non-nil error (exit 129, NOT misread as `true`/`false`); a missing git binary
surfaces as a non-nil error mentioning "git binary not found"; a cancelled context surfaces as
`errors.Is(err, context.Canceled)`; `run()`, `runWithInput`, the `Git` interface, `FileChange`,
`StagedDiffOptions`, `RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS`, `DiffTree`,
`parseDiffTree`, `StagedDiff`, and the remaining 4 stubs are byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary caller), and — transitively —
the auto-stage-all path (P1.M3.T2.S2) and the CLI default action (P1.M4.T1.S2).

**Use Case**: Before spending model tokens on generation, the orchestrator asks
`staged, err := g.HasStagedChanges(ctx)`. When `staged == false` and `auto_stage_all` is enabled
(default true), it runs `g.AddAll()` (P1.M1.T3.S5) and re-checks `HasStagedChanges`; if STILL
false (clean working tree), it exits 2 with a friendly "nothing to commit" message (FR17, FINDING 11).
When `staged == true`, it proceeds to `WriteTree` → `CommitTree` → `UpdateRefCAS`. The bool is also
the nothing-to-commit signal for the exit-2 path (PRD §15.4 exit codes).

**User Journey**: `g := git.New(repoPath)` → `staged, err := g.HasStagedChanges(ctx)` → if `err !=
nil`: surface + abort; if `!staged && auto_stage_all`: `g.AddAll(ctx)` → `staged, _ =
g.HasStagedChanges(ctx)` (re-check) → if `!staged`: exit 2; if `staged`: proceed to snapshot+generate.

**Pain Points Addressed**: (1) Without the exit-code inversion handled, a staged change would be
silently treated as an error (exit 1 → `err != nil`), breaking the entire happy path. (2) Without a
cheap pre-check, the orchestrator would take a snapshot + run generation against an empty index,
wasting tokens and producing a meaningless message before failing at `commit-tree`. (3) Index-vs-HEAD
semantics (not "does anything exist") are what make the auto-stage-all re-check meaningful.

## Why

- **PRD §9.4 / FR16–FR17 (Nothing-staged / auto-stage-all, P0 → G5):** *"FR16. If `git diff --cached
  --quiet` reports no staged changes … if `auto_stage_all` is enabled … run `git add -A`, then
  re-check for changes. FR17. If after auto-stage there are still no changes … exit … code 2."*
  `HasStagedChanges` IS the `git diff --cached --quiet` report — invoked twice in the auto-stage-all
  flow (before, and after the `git add -A`).
- **PRD §15.4 (Exit codes):** exit 2 = "nothing to commit" — driven by `HasStagedChanges` returning
  false after the (optional) auto-stage-all.
- **PRD Appendix C (Line-by-line porting map):** the auto-stage-all / staged-diff rows — the
  "nothing staged" detection is the gate the rest of the flow hangs off.
- **`critical_findings.md` FINDING 6:** the inverted-exit-code trap. `git diff --cached --quiet`
  exits 1 when staged changes EXIST. This subtask is the **structural encoding** of that inversion
  into a typed `bool` — so no downstream caller can ever get it wrong.
- **`critical_findings.md` FINDING 11 (Auto-stage-all default):** the nothing-to-commit path
  consumes `HasStagedChanges`'s bool twice (pre- and post-`add -A`).
- **Foundation for P1.M3.T4 / P1.M4.T1.S2:** the orchestrator and CLI default action are blocked on a
  correct `HasStagedChanges`. It is the cheapest possible gate (no diff output, one exit code) and
  is read-only w.r.t. refs/index (PRD §18.1 invariant — it mutates nothing).

## What

`HasStagedChanges` runs `git -C <workDir> diff --cached --quiet` via the existing `run()` helper and
translates the four-tuple into `(bool, error)`:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `(false, err)`. This is the ONLY path that returns a non-nil error; it is the
  infrastructural-failure guard.
- `code == 0` → return `(false, nil)`. Nothing staged (index == HEAD). True whether the repo is
  unborn (zero commits) or has commits with nothing new staged.
- `code == 1` → return `(true, nil)`. **Staged changes exist.** This is the INVERTED semantic — exit
  1 is the "has staged" signal, NOT an error (FINDING 6).
- any other code (the `default`, e.g. 128 corrupt repo / 129 not-a-repo) → return
  `(false, fmt.Errorf("git diff --cached --quiet: failed (exit %d): %s", code, strings.TrimSpace(stderr)))`.

`--quiet` suppresses diff output (verified: empty stdout), so stdout is discarded (`_`); stderr is
captured only for the default/error arm (carries git's diagnostic, e.g. "not a git repository"). No
pathspec/exclude argument in v1 — the contract is the bare `git diff --cached --quiet` (the
auto-stage-all / commit gate answers "is ANYTHING staged?", not "is anything staged outside
lock/vendor?"; the optional `-- ':!*.lock'` pathspec from `git_plumbing_reference.md` is a future
enhancement, out of scope).

No porcelain, no shell, no `cmd.Dir`/`os.Chdir` (inherited from `run()`), no new types, no new
constants, no new error sentinels, no new imports.

### Success Criteria

- [ ] `(*gitRunner).HasStagedChanges` body matches §Implementation Blueprint verbatim (no `panic`);
      delegates to `run()` (NOT exec directly); branches `err`-first, then `switch code { case 0;
      case 1; default }`.
- [ ] A staged change returns `(true, nil)` (exit 1 → true — the FINDING 6 inversion, NOT an error).
- [ ] A clean index returns `(false, nil)` (exit 0).
- [ ] A non-{0,1} exit returns `(false, err)` (exit >1, e.g. 128/129 — NOT misread as true/false).
- [ ] NO new imports in git.go (`fmt`, `strings` already present — distinct from S2 which added
      `strings`).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4),
      `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff` (T3.S1), and
      the remaining 4 method stubs (RecentMessages, RecentSubjects, CommitCount, AddAll) are
      byte-identical to their landed forms.
- [ ] `internal/git/hasstaged_test.go` exists in `package git` with the six test functions below.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `HasStagedChanges` line (removed; 4
      stubs remain after S1-T3.S1's `StagedDiff` removal: RecentMessages, RecentSubjects,
      CommitCount, AddAll).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all six new cases pass and S1–S6 / T3.S1's tests still
      pass.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path (`github.com/dustin/stagehand`); the
exact three files to touch (and the exact single line to remove from `git_test.go`); the exact
`run()` contract (signature + the `err==nil`-for-non-zero-exits invariant that the `code` branches
rely on); the exact `HasStagedChanges` body (verified-equivalent to throwaway git invocations run
against git 2.54.0); the empirically-pinned exit codes (clean=0, staged=1, committed-nothing-new=0,
non-repo=129); the exact six-test matrix with verified assertions and the reused helpers; and the
exact validation commands with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.4/FR16–FR17 (Nothing-staged / auto-stage-all: 'If git diff --cached --quiet reports no
        staged changes … if auto_stage_all is enabled … run git add -A, then re-check'; 'If after
        auto-stage there are still no changes … exit code 2'); §15.4 (exit code 2 = nothing-to-commit,
        driven by this bool); Appendix C (the staged-diff / auto-stage rows); §13/§11.1 (this is the
        read-only gate before the immutable snapshot — mutates no ref/object); §18.1 (the invariant:
        refs/index modified only at the final update-ref step — this method touches neither)."
  critical: "This subtask owns ONLY the HasStagedChanges body + its tests + the one-line
             TestStubsPanic edit. Do NOT implement StagedDiff (T3.S1), AddAll (P1.M1.T3.S5), the
             orchestrator (P1.M3.T4), the auto-stage-all flow (P1.M3.T2.S2), or the CLI (P1.M4.T1).
             Do NOT change the Git interface or add pathspec/exclude args (out of scope for v1)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 6 — THE spec for this method: 'git diff --cached --quiet exit 0 = nothing staged,
        exit 1 = staged changes exist, exit >1 = error. This is INVERTED from usual convention — must
        check exit code explicitly.' FINDING 11 — the auto-stage-all flow that consumes this bool
        twice (pre- and post-git add -A)."
  critical: "FINDING 6 is the single reason this method exists as a typed bool rather than a raw
             exit code: encoding exit 1 ⇒ true here prevents every downstream caller from
             re-deriving (and mis-deriving) the inversion."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "The '--quiet → nothing staged? signal' section (around line 185): documents exit 0/1/>1 and
        gives a canonical hasStagedChanges Go pattern (exec-direct). The Atomic Commit Sequence
        step 1 is this exact command ('git diff --cached --quiet → exit 1 = staged; exit 0 = nothing')."
  critical: "The reference pattern execs directly and branches on *exec.ExitError; our version
             DELEGATES to run() (which already performs errors.As(*exec.ExitError) and exposes
             exitCode with err==nil). Do NOT copy the exec plumbing verbatim — call run(). Also: the
             reference's optional -- ':!*.lock' exclude pathspec is a FUTURE enhancement, out of
             scope for v1 (the contract is the bare command)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Exit-Code Cheat Sheet row for 'git diff --cached --quiet': exit 0 = nothing staged,
        exit 1 = staged changes exist, exit 128 = error, 'exit 1 = has staged'. And Atomic Commit
        Sequence step 1. This is the one-row spec."
  critical: "Confirms exit 1 is the 'has staged' signal and that run()'s err=nil-for-nonzero-exits
             invariant (S1 gotcha G2) is what makes the code branch reachable."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything HasStagedChanges consumes: the gitRunner struct; the run()
        helper (exact signature and verified body — which this subtask does NOT modify, only calls);
        the HasStagedChanges panic-stub being replaced; the Git interface (signature already correct:
        HasStagedChanges(ctx) (bool, error)); New(); the git_test.go initRepo helper; and the
        assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil
             for non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2
             (non-zero exit is not a Go error) is the foundation HasStagedChanges's code branches
             rely on."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The closest analog PRP — RevParseHEAD is also an exit-code-semantics method that delegates
        to run() and returns (value, error). Its branch order (err-first, then code) and its test
        matrix (git-missing / ctx-cancelled via t.Setenv / cancel-before-call) are the templates for
        this subtask's implementation and tests."
  critical: "RevParseHEAD branches on a single special code (128); HasStagedChanges branches on two
             (0 and 1), so a switch (case 0 / case 1 / default) is used instead of the if-chain.
             S2 ADDED the 'strings' import; HasStagedChanges does NOT (fmt and strings are both
             already imported in git.go)."

- docfile: plan/001_f1f80943ac34/P1M1T3S2/research/hasstaged_validation.md
  why: "THIS subtask's own research: the contract recap (§1); the empirically-pinned exit codes on
        git 2.54.0 incl. the non-repo exit-129 finding (§2); the exact implementation with the
        switch decision (§3); the comparison with the arch-doc canonical pattern (§4 — why we
        delegate to run() and omit pathspec); the test matrix (§5); and the non-overlap with T3.S1
        (§6)."
  critical: "§2 (the non-repo case exits 129, not 128 — confirms the default arm must catch ANY
             non-{0,1} code, not just 128) and §3 decision D1 (switch over if-chain) are the two
             non-obvious calls. §6 documents the parallel-execution non-conflict with T3.S1."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-diff#_description
  why: "Documents `git diff --cached` (diff of staged changes vs HEAD) and `--quiet` ('Disable all
        output of the program. Implies --exit-code'). Confirms --quiet emits no stdout and uses the
        exit code as the signal."
  critical: "Establishes that --quiet produces NO output (stdout discarded) and that exit 1 = the
             'differences exist' signal — the FINDING 6 inversion. Distinct from the NON-quiet diff
             used by StagedDiff (T3.S1), which exits 0 on success regardless of changes."
- url: https://git-scm.com/docs/git-diff#Documentation/git-diff.txt-em--exit-code
  why: "Documents --exit-code ('Make the program exit with codes similar to diff(1). That is, it
        exits with 1 if there were differences and 0 means no differences'). --quiet implies this."
  critical: "This is the authoritative basis for exit 1 = 'has differences' (has staged). It is the
             signal the code==1 arm reads."
- url: https://pkg.go.dev/context#Canceled
  why: "The error value run() returns (via ctx.Err()) when the context is cancelled before/during the
        git call; HasStagedChanges propagates it unchanged."
  critical: "The ctx-cancel test asserts errors.Is(err, context.Canceled) — confirm it is the
             unwrapped ctx.Err() (run() returns ctx.Err() directly when ctx.Err() != nil)."
```

### Current Codebase Tree (after S1 + S2 + S3 + S4 + S5 + S6 + T3.S1 have landed — the assumed state)

```bash
stagehand/
├── PRD.md
├── go.mod                # module github.com/dustin/stagehand, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagehand/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface+gitRunner+run()+New()+FileChange+StagedDiffOptions+stubs;
│       │                 #   S2: RevParseHEAD real (+strings import); S3: WriteTree real;
│       │                 #   S4: runWithInput+CommitTree real (+io import); S5: ErrCASFailed+
│       │                 #   UpdateRefCAS real; S6: DiffTree real+parseDiffTree; T3.S1: StagedDiff
│       │                 #   real (constants+defaultExcludes). imports: bytes, context, errors,
│       │                 #   fmt, io, os/exec, strings  ← ALL HasStagedChanges needs present (fmt, strings)
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic + initRepo + assertPanics.
│       │                 #   After T3.S1 lands, TestStubsPanic lists 5 stubs: HasStagedChanges,
│       │                 #   RecentMessages, RecentSubjects, CommitCount, AddAll (StagedDiff line
│       │                 #   removed by T3.S1). NOTE: the current on-disk state may still show the
│       │                 #   StagedDiff line if T3.S1 hasn't merged yet — either way, THIS subtask
│       │                 #   removes ONLY the HasStagedChanges line.
│       ├── revparse_test.go   # S2: minGitEnv + makeEmptyCommit + 4 tests  (REUSED: makeEmptyCommit)
│       ├── writetree_test.go  # S3: makeMergeConflict + 5 tests
│       ├── committree_test.go # S4: setIdentityConfig/writeFile/stageFile/writeTreeOf/headSHA/
│       │                       #   commitMessage + 6 tests  (REUSED: writeFile, stageFile)
│       ├── updateref_test.go  # S5: cas*/gitIdentityEnv + 6 tests
│       ├── difftree_test.go   # S6: dtCommit/dtRemove + 9 tests
│       └── stagediff_test.go  # T3.S1: sd* helpers + ~11 tests
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── internal/
    └── git/
        ├── git.go              # MODIFIED — HasStagedChanges stub → real switch body. NO import change.
        ├── git_test.go         # MODIFIED — remove the ONE `HasStagedChanges` line from TestStubsPanic
        ├── revparse_test.go    # UNCHANGED (S2's file; makeEmptyCommit reused, not edited)
        ├── writetree_test.go   # UNCHANGED (S3's file)
        ├── committree_test.go  # UNCHANGED (S4's file; writeFile/stageFile reused, not redeclared)
        ├── updateref_test.go   # UNCHANGED (S5's file)
        ├── difftree_test.go    # UNCHANGED (S6's file)
        ├── stagediff_test.go   # UNCHANGED (T3.S1's file)
        └── hasstaged_test.go   # NEW — package git; the test matrix below (NO new helpers)
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | Replace the `HasStagedChanges` panic-stub with the `switch`-on-`run()`-exit-code body (keep the same signature, add the doc comment). No import changes. |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "HasStagedChanges", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/hasstaged_test.go` | CREATE | `package git` tests for `HasStagedChanges`: nothing-staged / staged-file / committed-nothing-staged / not-a-repo / git-binary-missing / ctx-cancelled. Reuses `initRepo`, `writeFile`, `stageFile`, `makeEmptyCommit`. No new helpers. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants
/`defaultExcludes` (T3.S1), the remaining 4 method stubs (RecentMessages, RecentSubjects,
CommitCount, AddAll), `revparse_test.go` (S2), `writetree_test.go` (S3), `committree_test.go` (S4),
`updateref_test.go` (S5), `difftree_test.go` (S6), `stagediff_test.go` (T3.S1), `go.mod`/`go.sum`,
the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the core trap this subtask exists to encode): `git diff --cached --quiet` exits
// 1 when staged changes EXIST and 0 when nothing is staged — INVERTED from the usual convention
// (FINDING 6). A naive `if err != nil { /* error */ }` would treat exit 1 (something staged) as an
// error. HasStagedChanges reads run()'s exitCode and maps code==1 → (true, nil), code==0 →
// (false, nil). Verified on this box (git 2.54.0): staged file → exit 1; clean index → exit 0.
// `--quiet` ALSO implies `--exit-code` (git-diff docs), which is what makes exit 1 the "has
// differences" signal.

// CRITICAL (G2 — run()'s invariant, inherited from S1): run() returns err == nil for NON-ZERO git
// exits (1, 128, 129, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O)
// set err != nil, with exitCode == -1. Therefore HasStagedChanges MUST check `err != nil` FIRST
// (authoritative, covers the -1 cases), THEN branch on exitCode via switch. A reversed check (code
// before err) would still be correct (code==-1 falls to the default error arm, which is fine), but
// err-first is the clear/readable order and matches every landed method (RevParseHEAD/WriteTree/…).

// CRITICAL (G3 — LookPath miss must NOT be misread): when git is absent, run() returns
// exitCode == -1, err != nil. The HasStagedChanges `if err != nil { return false, err }` guard runs
// BEFORE the switch, so a missing binary is surfaced as a real error (NOT silently false or true).
// TestHasStagedChanges_GitBinaryMissing guards against regressing this.

// CRITICAL (G4 — exit >1 is the catch-all error, not just 128): a non-repo directory makes
// `git -C <tmp> diff --cached --quiet` exit 129 (git re-interprets `diff --cached` outside a repo as
// `git diff --no-index`, where `--cached` is an unknown option → exit 129), NOT 128. A corrupt repo
// may exit 128. EITHER WAY the switch's `default` arm catches it. Do NOT branch on `code == 128`
// specifically — branch on "not 0 and not 1". Verified empirically (git 2.54.0: non-repo → 129).
// (Contrast RevParseHEAD S2, which DOES branch on the exact code 128 — that method's unborn signal
// is 128-specific; HasStagedChanges has no single special code beyond 0 and 1.)

// GOTCHA (G5 — ZERO new imports): HasStagedChanges uses g.run (S1), fmt.Errorf/Sprintf (fmt —
// present), strings.TrimSpace (strings — present). git.go's import block (bytes, context, errors,
// fmt, io, os/exec, strings) already has everything. Do NOT add an import. (This is the key
// difference from S2/RevParseHEAD, which HAD to add "strings". The compiler errors on an unused
// import if you add one erroneously.)

// GOTCHA (G6 -- --quiet produces NO stdout): `git diff --cached --quiet` with --quiet disables all
// output (verified: empty stdout). Discard it with `_, stderr, code, err := g.run(...)`. stderr IS
// captured for the default/error arm (carries the diagnostic, e.g. "not a git repository" / "unknown
// option `cached'"). Do NOT assert on stdout in tests.

// GOTCHA (G7 — the TestStubsPanic edit): git_test.go's TestStubsPanic (after S2–S6 and T3.S1 removed
// their lines) includes
//   assertPanics(t, "HasStagedChanges", func() { _, _ = g.HasStagedChanges(ctx) }).
// Once HasStagedChanges is real (no panic), assertPanics fails with "expected panic, but did not
// panic". Resolution (mirrors S2/S3/S4/S5/S6/T3.S1): DELETE that one line. After removal,
// TestStubsPanic covers the remaining 4 stubs (RecentMessages, RecentSubjects, CommitCount, AddAll).
// This is the ONLY edit to git_test.go. NOTE: T3.S1 removes a DIFFERENT line (StagedDiff); the two
// edits do not overlap (distinct assertPanics lines).

// GOTCHA (G8 — reuse helpers, do NOT redeclare): initRepo (git_test.go), writeFile + stageFile
// (committree_test.go, S4), and makeEmptyCommit (revparse_test.go, S2) are all `package git` and in
// scope for hasstaged_test.go. REUSE them — do NOT redeclare (a redeclaration is a compile error).
// No new helpers are needed for this subtask. (If one were needed, use an `hs` prefix to avoid
// collision with T3.S1's `sd`, S5's `cas`, S6's `dt`.)

// GOTCHA (G9 — test file is package git, white-box): HasStagedChanges is on *gitRunner and calls the
// unexported run(); the unexported gitRunner.workDir is read by New. To call New() and exercise the
// real method against temp repos, the test MUST be `package git` (NOT `git_test`). This matches every
// other test file in internal/git/.

// GOTCHA (G10 — no shell, no cmd.Dir in PRODUCTION code): HasStagedChanges inherits S1's §19
// guarantees because it only calls run(). Do NOT introduce exec.Command / os.Chdir / sh -c anywhere
// in the new code. The test fixtures DO use exec.Command directly (the reused writeFile/stageFile/
// makeEmptyCommit/initRepo helpers use []string args + cmd.Env, never a shell) — that is acceptable
// test-fixture usage. The Level-1 grep for sh -c / cmd.Dir covers PRODUCTION code (git.go) only.

// GOTCHA (G11 — the "committed but nothing staged" fixture proves index-vs-HEAD): a repo with a
// commit but nothing NEW staged ALSO returns exit 0 (verified). TestHasStagedChanges_CommittedNothing
// Staged uses makeEmptyCommit to establish HEAD, then asserts (false, nil) — proving the comparison
// is index-vs-HEAD, NOT "does any file/object exist". Without this test, an implementation that
// (wrongly) checked `git status` or `git log` emptiness could pass the other cases but fail real use.

// GOTCHA (G12 — context cancellation order in run()): run() checks ctx.Err() BEFORE errors.As(
// ExitError). So when ctx is cancelled, HasStagedChanges gets (stdout, stderr, -1, ctx.Err()) and
// the `err != nil` guard returns (false, ctx.Err()). The cancel-test cancels ctx BEFORE the call for
// determinism and asserts errors.Is(err, context.Canceled).
```

## Implementation Blueprint

### Data models and structure

None added or changed. `HasStagedChanges`'s return type `(bool, error)` is already declared in the
`Git` interface by S1. No new constants, no new structs, no new options type, no new error sentinel.

### The `HasStagedChanges` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature. Place it where the stub currently is
(further down in git.go, among the other method stubs/bodies). Keep the interface's doc-comment
intent but write a method-level doc comment explaining FINDING 6.

```go
// HasStagedChanges reports whether the index differs from HEAD (PRD §9.4/FR16–FR17, FINDING 6). It
// runs `git diff --cached --quiet`, which produces NO output (--quiet disables it) and encodes the
// answer in the exit code. The semantics are INVERTED from the usual convention and must be read
// explicitly: exit 0 → nothing staged (index == HEAD); exit 1 → staged changes EXIST (this is the
// "has staged" signal, NOT an error); any other exit (e.g. 128 corrupt repo, 129 not-a-repo) → a
// real error. A naive `err != nil` check would misread exit 1 as an error — this method is the
// structural encoding of the inversion into a typed bool so no downstream caller can get it wrong.
//
// It is read-only with respect to refs and the index (PRD §18.1): it mutates nothing. The orchestrator
// (P1.M3.T4) calls it as the pre-generation gate and again after auto-stage-all (FINDING 11); the
// CLI uses it to drive the exit-2 "nothing to commit" path (PRD §15.4).
func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
	_, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--quiet")
	if err != nil {
		return false, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	switch code {
	case 0:
		return false, nil // nothing staged (index == HEAD)
	case 1:
		return true, nil // staged changes exist — exit 1 is the signal, NOT an error (FINDING 6)
	default:
		return false, fmt.Errorf("git diff --cached --quiet: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
}
```

> **Verified:** the args (`["diff", "--cached", "--quiet"]`), the exit-code mapping (0→false,
> 1→true, else→error), and the empty-stdout behavior are confirmed by this subtask's research §2
> (re-verified empirically against git 2.54.0: clean=0, staged=1, committed-nothing-new=0,
> non-repo=129). `--quiet` implies `--exit-code` (git-diff docs), which is what makes exit 1 the
> "has differences" signal.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (method body only — NO import change)
  - EDIT — replace the HasStagedChanges panic-stub:
      FIND the stub (it is currently — on the post-S1 disk — one of these two forms depending on
      whether the interface doc comment was copied onto the stub; match whichever exists):
        Form A (bare stub, most likely):
          func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
              panic("gitRunner.HasStagedChanges: not yet implemented — see P1.M1.T3.S2")
          }
      REPLACE with: the doc comment + `HasStagedChanges` method verbatim from §"The HasStagedChanges
      body" above. Keep the SAME signature `func (g *gitRunner) HasStagedChanges(ctx context.Context)
      (bool, error)`.
  - DO NOT touch: the import block (all needed symbols already present — gotcha G5), run(),
    runWithInput, New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD (real
    from S2), WriteTree (real from S3), CommitTree (real from S4), UpdateRefCAS/ErrCASFailed (real
    from S5), DiffTree/parseDiffTree (real from S6), StagedDiff/its constants/defaultExcludes (real
    from T3.S1), or any of the remaining 4 method stubs (RecentMessages, RecentSubjects, CommitCount,
    AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic:
      assertPanics(t, "HasStagedChanges", func() { _, _ = g.HasStagedChanges(ctx) })
  - DELETE that single line. After removal TestStubsPanic covers the remaining 4 stubs:
    RecentMessages, RecentSubjects, CommitCount, AddAll. (If T3.S1 has not yet merged, the StagedDiff
    line is also still present — leave it; T3.S1 owns removing its own line. This subtask removes
    ONLY the HasStagedChanges line.)
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper,
    the other assertPanics lines).
  - WHY: once HasStagedChanges is real it no longer panics; assertPanics would fail (gotcha G7).
    Mirrors S2/S3/S4/S5/S6/T3.S1's removals.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (4 stubs still panic).

Task 3: CREATE internal/git/hasstaged_test.go (package git — white-box)
  - FILE: internal/git/hasstaged_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G9; matches the other test files)
  - IMPORTS: context, errors, strings, testing  (all stdlib; errors for errors.Is, strings for
    strings.Contains in the err-message assertions; NO os/os/exec/regexp needed — gotcha G8)
  - REUSE (do NOT redeclare): initRepo (git_test.go), writeFile + stageFile (committree_test.go, S4),
    makeEmptyCommit (revparse_test.go, S2).
  - WRITE the six test functions (assertions in §"Test cases" below). No new helpers (gotcha G8).
  - TEST MATRIX (6 functions):
      TestHasStagedChanges_NothingStaged:
        repo := t.TempDir(); initRepo(t, repo)          // fresh repo, nothing staged
        g := New(repo)
        staged, err := g.HasStagedChanges(context.Background())
        assert err == nil
        assert staged == false                            // exit 0 → nothing staged
      TestHasStagedChanges_StagedFile:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go")
        g := New(repo)
        staged, err := g.HasStagedChanges(context.Background())
        assert err == nil
        assert staged == true                             // exit 1 → staged (the INVERTED semantics)
      TestHasStagedChanges_CommittedNothingStaged:
        repo := t.TempDir(); initRepo(t, repo)
        makeEmptyCommit(t, repo, "initial")              // HEAD exists, nothing NEW staged
        g := New(repo)
        staged, err := g.HasStagedChanges(context.Background())
        assert err == nil
        assert staged == false                            // index == HEAD → exit 0 (G11)
      TestHasStagedChanges_NotARepo:
        g := New(t.TempDir())                             // a plain dir, NOT a git repo (no initRepo)
        staged, err := g.HasStagedChanges(context.Background())
        assert err != nil
        assert strings.Contains(err.Error(), "git diff --cached --quiet: failed")  // default arm, exit 129
        assert staged == false                            // exit >1 → error, NOT misread (G4)
      TestHasStagedChanges_GitBinaryMissing:
        t.Setenv("PATH", "")                              // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                             // dir need not be a repo (LookPath fails first)
        staged, err := g.HasStagedChanges(context.Background())
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert staged == false                            // guard: LookPath miss NOT misread (G3)
      TestHasStagedChanges_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call (G12)
        g := New(t.TempDir())
        staged, err := g.HasStagedChanges(ctx)
        assert err != nil && errors.Is(err, context.Canceled)
        assert staged == false
  - NAMING: TestHasStagedChanges_<Scenario>.
  - DO NOT redeclare initRepo / writeFile / stageFile / makeEmptyCommit (they live in sibling test
    files in the same package).
  - VERIFY: go test -race -run TestHasStagedChanges ./internal/git/ → exit 0, all 6 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go  (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                 (expect: no matches)
  - RUN: git grep -n 'panic.*HasStagedChanges' internal/git/git.go             (expect: no matches — stub gone)
  - RUN: git status --porcelain → expect EXACTLY internal/git/git.go (modified) + internal/git/
         git_test.go (modified) + internal/git/hasstaged_test.go (new). (Plus whatever T3.S1's
         concurrent changes produce — non-overlapping.)
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestHasStagedChanges_NothingStaged` | `initRepo` (nothing staged) | `err==nil && staged==false` | exit 0 → nothing staged |
| `TestHasStagedChanges_StagedFile` | `initRepo` + `writeFile` + `stageFile` | `err==nil && staged==true` | exit 1 → staged (the FINDING 6 inversion) |
| `TestHasStagedChanges_CommittedNothingStaged` | `initRepo` + `makeEmptyCommit` (HEAD exists, nothing new) | `err==nil && staged==false` | index-vs-HEAD comparison, not "anything exists" (G11) |
| `TestHasStagedChanges_NotARepo` | `t.TempDir()` WITHOUT `initRepo` | `err!=nil` contains "git diff --cached --quiet: failed"; `staged==false` | exit>1 (129) → error, not misread (G4) |
| `TestHasStagedChanges_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found"; `staged==false` | `run()`'s err path propagated, not misread (G3) |
| `TestHasStagedChanges_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `staged==false` | ctx.Err() surfaced (not exit code) |

### Implementation Patterns & Key Details

```go
// === Why a switch (case 0 / case 1 / default) instead of an if-chain ===
// There are exactly two "good" codes (0 = nothing staged, 1 = staged) and EVERYTHING else is an
// error. A switch with case 0 / case 1 / default makes the two-value map + catch-all explicit and
// hard to misread. (Contrast RevParseHEAD S2, which used `if code == 128 / if code != 0` because it
// had ONE special code; HasStagedChanges has two.) The default arm catches 128 (corrupt repo), 129
// (not-a-repo), and any future code uniformly — do NOT special-case 128 (gotcha G4).

// === Why err is checked BEFORE code (the branch order) ===
// run() guarantees: err != nil  ⟹  exitCode == -1  (LookPath / context / start failure).
//                   err == nil   for every real git exit (0, 1, 128, 129, …).
// So `if err != nil { return false, err }` is the authoritative infrastructural-failure guard.
// Only when err == nil does the switch run. Reversing the order would still be correct (code==-1
// falls to the default error arm, which wraps stderr that is empty-but-harmless), but err-first is
// clearer and matches every landed method.

// === Why stdout is discarded (`_`) ===
// `git diff --cached --quiet` emits NO stdout (--quiet disables all output; verified empty). There
// is nothing to inspect, so `_, stderr, code, err := g.run(...)`. stderr IS captured for the
// default/error arm only — it carries git's diagnostic (e.g. "not a git repository" / "unknown
// option `cached'"). (Contrast UpdateRefCAS, which keeps `stdout` + `_ = stdout` because update-ref
// semantics differ; here `_` is the clean, honest choice.)

// === Why no pathspec/exclude argument in v1 ===
// The contract is the bare `git diff --cached --quiet` — the gate answers "is ANYTHING staged?",
// which is exactly what the orchestrator's snapshot-yes/no and the auto-stage-all re-check need.
// The arch-doc's optional `-- ':!*.lock'` exclude ("is anything staged outside lock/vendor?") is a
// FUTURE enhancement; adding it now would deviate from the work-item contract and complicate the
// method. (StagedDiff, T3.S1, owns pathspec excludes — a different concern.)

// === Why the ctx-cancel test cancels BEFORE the call ===
// Pre-cancelling makes cmd.Run() fail deterministically (the process is killed/not started), so
// run()'s ctx.Err()-before-errors.As path returns (-1, context.Canceled) reliably. Cancelling
// mid-run would race with a fast `diff --cached --quiet` completion (it is sub-millisecond).

// === Reusing initRepo / writeFile / stageFile / makeEmptyCommit ===
// Because hasstaged_test.go is `package git`, these helpers (declared in sibling test files in the
// same package) are in scope. Call them directly; do NOT redefine them (a redeclaration is a compile
// error). initRepo (git_test.go) makes an unborn repo; writeFile/stageFile (committree_test.go, S4)
// stage a file; makeEmptyCommit (revparse_test.go, S2) creates a commit to establish HEAD.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagehand" → package import path "github.com/dustin/stagehand/internal/git"
  - go directive: 1.22 → context, errors.Is, strings, t.Setenv (1.17+) all available
  - deps: NONE added (fmt, strings are stdlib and already imported)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go              # MODIFIED: HasStagedChanges body (NO import change)
  - file: internal/git/git_test.go         # MODIFIED: remove the HasStagedChanges line from TestStubsPanic
  - file: internal/git/hasstaged_test.go   # NEW: package git, 6 tests, no new helpers

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M3.T4 (CommitStaged orchestrator): the primary caller — `staged, err := g.HasStagedChanges(ctx)`
    is the gate before snapshot+generate.
  - P1.M3.T2.S2 (auto-stage-all / nothing-to-commit): consumes the bool twice (before and after
    `g.AddAll()`); still-false after add → exit 2 (FR17, FINDING 11).
  - P1.M4.T1.S2 (CLI default action): wires auto-stage-all + CommitStaged + success report; uses the
    bool for the exit-2 path (PRD §15.4).

PARALLEL-EXECUTION NOTE (with P1.M1.T3.S1 — StagedDiff, landing concurrently):
  - T3.S1 edits git.go in the StagedDiff region (immediately after parseDiffTree) and removes the
    StagedDiff line from git_test.go's TestStubsPanic.
  - THIS subtask edits git.go in the HasStagedChanges region (a different, later method stub) and
    removes the HasStagedChanges line from TestStubsPanic (a different assertPanics line).
  - These are DISTINCT, NON-OVERLAPPING regions (different method bodies; different assertPanics
    lines). Both can land in parallel without conflict. If git.go does not yet have T3.S1's
    StagedDiff real-body at edit time, that is fine — this subtask's edit anchor (the HasStagedChanges
    stub text) is independent of StagedDiff's state.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...        # Expected: exit 0, no warnings (e.g. no unused import, no shadowing)
go build ./internal/git/         # Expected: exit 0 (package compiles; fmt+strings already imported)
go build ./...                   # Expected: exit 0 (whole module compiles)

# Expected: zero output/errors. If `go build` says `undefined: fmt` or `undefined: strings`, the
# imports were somehow removed — they MUST stay (run() and the other real methods use them). This
# subtask adds NO import and removes NONE.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagehand

go test -race -v -run 'TestHasStagedChanges' ./internal/git/   # Expected: 6 tests PASS, exit 0
# Must see: TestHasStagedChanges_NothingStaged, _StagedFile, _CommittedNothingStaged, _NotARepo,
#           _GitBinaryMissing, _ContextCancelled — all ok.

go test -race -run 'TestStubsPanic' ./internal/git/   # Expected: exit 0 — 4 remaining stubs panic
go test -race ./internal/git/    # Expected: exit 0 — S1's run() tests, S2–S5's plumbing tests,
                                 # S6's DiffTree tests, T3.S1's StagedDiff tests, AND the 6 new
                                 # HasStagedChanges tests all pass.

make test                        # Expected: exit 0 (Makefile target = go test -race ./...)
```

### Level 3: Security & Structural Invariants (the §19 enforcement + scope discipline)

```bash
cd /home/dustin/projects/stagehand

# PRD §19: NO shell execution in the PRODUCTION git wrapper (inherited; new code adds none).
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output.

# No os.Chdir / cmd.Dir in PRODUCTION code (inherited; new code adds none).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output.

# The HasStagedChanges stub is gone (replaced by a real body).
git grep -n 'HasStagedChanges' internal/git/git.go
# Expected: the interface declaration + the real func declaration + body; NO line matching
# `panic("gitRunner.HasStagedChanges`.

# Only the intended files changed (plus T3.S1's concurrent non-overlapping changes, if any).
git status --porcelain
# Expected (this subtask's contribution):
#   M internal/git/git.go
#   M internal/git/git_test.go
#   ?? internal/git/hasstaged_test.go

# go.mod / go.sum untouched.
git diff --name-only go.mod go.sum
# Expected: NO output.
```

### Level 4: Runtime Smoke Test (prove HasStagedChanges works against a real repo)

```bash
cd /home/dustin/projects/stagehand

# Reproduce the exit codes the tests assert, against the real binary (mirrors the research):
tmp=$(mktemp -d); git -C "$tmp" init -q
git -C "$tmp" diff --cached --quiet; echo "nothing-staged EXIT=$?"   # expect 0
printf 'package main\n' > "$tmp/a.go"; git -C "$tmp" add a.go
git -C "$tmp" diff --cached --quiet; echo "staged       EXIT=$?"     # expect 1
git -C "$tmp" -c user.name=t -c user.email=t@t commit -q -m init
git -C "$tmp" diff --cached --quiet; echo "committed    EXIT=$?"     # expect 0 (nothing new staged)
tmp2=$(mktemp -d)
git -C "$tmp2" diff --cached --quiet; echo "non-repo     EXIT=$?"    # expect 129 (>1 → error)
rm -rf "$tmp" "$tmp2"
# If the staged-file exit code is NOT 1, the box's git differs from 2.54.0 or the index is dirty —
# re-pin the TestHasStagedChanges_StagedFile assertion to the observed behavior and note the git version.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (NO import change — fmt + strings already present).
- [ ] `go test -race ./internal/git/` exits 0 (6 new `TestHasStagedChanges_*` + all prior tests pass).
- [ ] `make test` exits 0.

### Feature Validation

- [ ] `(*gitRunner).HasStagedChanges` body matches §Blueprint (delegates to `run()`, switches on
      `code` with case 0 / case 1 / default).
- [ ] On a repo with a staged file: returns `(true, nil)` (exit 1 → staged — the FINDING 6 inversion).
- [ ] On a repo with nothing staged (unborn OR committed-nothing-new): returns `(false, nil)`.
- [ ] On a non-repo directory: returns `(false, err)` containing "git diff --cached --quiet: failed".
- [ ] On a missing git binary: returns `(false, err)` mentioning "git binary not found" (NOT
      `staged==true` or `staged==false`-without-error).
- [ ] On a cancelled context: returns `(false, errors.Is(err, context.Canceled))`.

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] NO `cmd.Dir` / `os.Chdir` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] `run()` is NOT re-implemented or modified (delegated to, unchanged).
- [ ] `Git` interface, `gitRunner`, `New`, `FileChange`, `StagedDiffOptions` unchanged.
- [ ] `RevParseHEAD`/`WriteTree`/`CommitTree`/`UpdateRefCAS`/`DiffTree`/`parseDiffTree`/`StagedDiff`
      and their constants/vars unchanged.
- [ ] The remaining 4 method stubs untouched (RecentMessages, RecentSubjects, CommitCount, AddAll —
      still panic with their owning-subtask messages).
- [ ] `go.mod` / `go.sum` unchanged (no new deps).
- [ ] Only `internal/git/git.go` (modified), `internal/git/git_test.go` (modified — one line removed),
      and `internal/git/hasstaged_test.go` (new) are changed by this subtask.
- [ ] No signal handling / `SysProcAttr` / process-group code added (that is P1.M4.T2).

### Documentation & Deployment

- [ ] Doc comment on `HasStagedChanges` explains the FINDING 6 exit-code inversion (0→nothing,
      1→staged, >1→error) and the read-only invariant.
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't treat exit 1 as an error — `git diff --cached --quiet` exits 1 when staged changes EXIST
  (FINDING 6). Branch on `run()`'s `exitCode == 1` → `(true, nil)`. A naive `if err != nil` reads
  exit 1 as a failure and breaks the entire happy path (gotcha G1).
- ❌ Don't special-case exit 128 for the error path — a non-repo directory exits **129** (verified),
  and a corrupt repo may exit 128. Use the switch `default` (any non-{0,1} code → error); do NOT
  write `if code == 128` (gotcha G4).
- ❌ Don't re-implement the `exec.CommandContext` plumbing inside `HasStagedChanges` — delegate to
  `run()`, which already handles LookPath, `-C repo` args, separate buffers, and
  `errors.As(*exec.ExitError)`. Duplicating it breaks the single-shell-out-point invariant and §19.
- ❌ Don't check `exitCode` before `err` and omit the `err != nil` guard — a missing git binary yields
  `exitCode == -1`, which must surface as a real error via the `err != nil` arm (it falls to the
  switch default otherwise, wrapping an empty stderr — functionally OK but the explicit guard is
  clearer and is what TestHasStagedChanges_GitBinaryMissing asserts) (gotcha G3).
- ❌ Don't add an import — `fmt` and `strings` are BOTH already imported in git.go (unlike S2/
  RevParseHEAD which had to add `strings`). Adding one triggers an "unused import" compile error
  (gotcha G5).
- ❌ Don't capture/inspect stdout — `--quiet` produces NO output (verified empty). Use
  `_, stderr, code, err := g.run(...)` (gotcha G6).
- ❌ Don't add a pathspec/exclude argument (`-- ':!*.lock'`) — the contract is the bare
  `git diff --cached --quiet`. The optional exclude is a future enhancement (StagedDiff owns excludes).
- ❌ Don't write the test as `package git_test` (black-box) — `New()` returns the `Git` interface but
  the fixtures need `package git` to share `initRepo`/`writeFile`/`stageFile`/`makeEmptyCommit`.
  Match every other test file's package (gotcha G9).
- ❌ Don't redeclare `initRepo` / `writeFile` / `stageFile` / `makeEmptyCommit` in `hasstaged_test.go`
  — they already live in sibling test files in the same package. Reuse them (gotcha G8).
- ❌ Don't cancel the context mid-run in the cancel test — cancel BEFORE the call for determinism, and
  assert via `errors.Is(err, context.Canceled)` (gotcha G12).
- ❌ Don't touch `git_test.go` except for removing the ONE `HasStagedChanges` line from
  `TestStubsPanic` (gotcha G7). Don't remove T3.S1's `StagedDiff` line — that's T3.S1's edit.
- ❌ Don't implement `StagedDiff`/`AddAll`/the orchestrator/the CLI, and don't edit `run()` or the
  interface — those are other subtasks' deliverables.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: This is a small, fully-specified change — the body is ~12 lines and is verified-equivalent
to the pattern in `critical_findings.md` FINDING 6 and `git_plumbing_reference.md`'s `--quiet`
section, re-pinned empirically on this exact box (git 2.54.0: clean=0, staged=1,
committed-nothing-new=0, non-repo=129). The delegation target (`run()`) and its
`err==nil`-for-non-zero-exits invariant are themselves empirically verified in S1's research and
quoted verbatim here. The exit-code mapping (0→false, 1→true, else→error) is encoded as a `switch`
that is unreadable-by-accident. ZERO import changes are needed (the one mechanical step that tripped
S2 — adding `strings` — does not apply here because both `fmt` and `strings` are already imported).
The six test cases each map to a distinct, verified branch (nothing-staged / staged /
committed-nothing-staged / not-a-repo / LookPath-miss / ctx-cancel) with deterministic fixtures
reused from sibling test files. The single cross-subtask wrinkle (the `TestStubsPanic` line removal)
is anticipated with an explicit, scoped resolution that does not conflict with T3.S1's concurrent
removal of its own `StagedDiff` line. No external dependencies, no interface changes, no ambiguity in
the contract.
