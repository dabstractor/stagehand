---
name: "P1.M3.T1.S3 — Add empty-message guard in decompose.publishCommit after RunCommitHooks (Issue 4: a hook that empties the message file must abort, not create an empty-message commit — the decompose path)"
description: |

  Bugfix for Issue 4 (Bug-Fix PRD §h2.2/h3.3; stagecoach PRD §9.25 FR-V2 git parity + §9.8/§13.2 atomic-commit
  core). After `hooks.RunCommitHooks` returns, `decompose.publishCommit` assigns `finalTree`/`finalMsg` and
  passes `finalMsg` straight to `CommitTree` with NO empty-message check (message.go:230-235). A
  `prepare-commit-msg` or `commit-msg` hook that empties the message file (a common rejection / force-re-edit
  pattern) therefore produces a commit with an EMPTY message (invalid git state that `git commit` refuses
  unless `--allow-empty-message`). This task adds the missing guard — the SAME pattern S1 (DONE,
  generate.go:436-439) shipped for `CommitStaged` and S2 (IMPLEMENTING in parallel) ships for `runPipeline`:
  after the `if herr != nil` block, if the hook-adjusted message is empty (after trimming), abort with the
  BARE `generate.ErrEmptyMessage` (exit 1, NOT exit 0 warn-and-print, NOT exit 3 rescue, NOT CAS partial).

  This is the THIRD and FINAL of the three call sites named in Issue 4: CommitStaged (S1, done) → runPipeline
  (S2, parallel) → **publishCommit (S3, this task)**. Each is an independent one-guard fix. After S3, Issue 4
  is closed on every commit path.

  THE FIX — one guard inserted in `publishCommit` (`internal/decompose/message.go`), AFTER the closing `}`
  of `if herr != nil { return "", herr }` (file line ~234), BEFORE `newSHA, err := deps.Git.CommitTree(ctx,
  finalTree, parents, finalMsg)` (file line ~235):
    ```go
    // §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message
    // file (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit
    // message."; mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as
    // the --edit path (generate.EditMessage), generate.CommitStaged's guard (S1), and runPipeline's
    // guard (S2). HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
    if strings.TrimSpace(finalMsg) == "" {
        return "", generate.ErrEmptyMessage
    }
    ```
  `generate.ErrEmptyMessage` (finalize.go:45) is EXPORTED; `decompose/message.go` ALREADY imports `strings`
  (line 34) + `generate` (line 37 — for RescueError/CASError/ExtractSubject/...) ⇒ NO new import. The abort
  is BARE (not `*RescueError`, not `*CASError`) → exit 1 (verified: exitcode.go:65 maps
  `errors.Is(err, generate.ErrEmptyMessage)` → exit 1).

  ⚠️ **#1 — The variable is `finalMsg` (NOT `msg`), the return is `("", generate.ErrEmptyMessage)` (NOT
       `Result{}, ErrEmptyMessage`).** `publishCommit` keeps the hook result in a SEPARATE `finalTree`/
       `finalMsg` pair (it never reassigns `tree`/`msg`, unlike S1/S2's hooks blocks). Its return type is
       `(string, error)` where the string is the new SHA → return the empty-string zero value (mirrors
       `return "", herr` one line above). `generate.ErrEmptyMessage` is referenced cross-package
       (generate.ErrEmptyMessage, NOT bare ErrEmptyMessage — decompose ≠ generate). (research §1)

  ⚠️ **#2 — runLoop propagation is VERIFIED: the bare error rides the EXISTING hard-error path.** runLoop's
       `publishCommit` error handling (decompose.go:484-491) does `errors.As(err, &ce)` (→ false for a bare
       ErrEmptyMessage, which is not `*CASError`) then `return err` (HARD). So S3 does NOT touch runLoop,
       Decompose(), the CLI, or exitcode — the guard's bare error propagates through the same path any
       non-CAS `publishCommit` failure already takes. Prior committed concepts stand (already in HEAD); the
       empty-message concept is NOT committed (the abort is before CommitTree). (research §3)

  ⚠️ **#3 — TDD: write the FAILING test first.** Mirror `TestPublishCommit_PreCommitAbort_RescueError`
       (message_test.go:436) but install a `commit-msg` that empties the file (`> "$1"; exit 0`) and assert
       `errors.Is(err, generate.ErrEmptyMessage)` + HEAD unchanged. Before the guard: `publishCommit`
       creates an empty-message commit (git `commit-tree` does NOT refuse empty messages — Issue 4 confirmed
       `git log -1 --format=%B | xxd → 0a`) → HEAD moves + err==nil → the test FAILS. After the guard: the
       guard returns ErrEmptyMessage before CommitTree → the test PASSES. (research §4)

  ⚠️ **#4 — publishCommit ONLY (scope).** Do NOT touch `generate.CommitStaged` (S1, done), `runPipeline`
       (S2, parallel), `runLoop`, `Decompose()`, the CLI, exitcode, or any hooks/generate/config code. S3
       is `internal/decompose/message.go` (+ its test) ONLY. (research §0)

  ⚠️ **#5 — No new imports (either file); go.mod UNCHANGED.** message.go already imports `strings` + `generate`;
       message_test.go already imports `errors` + `strings` + `generate`. No new external dep.

  Deliverable: MODIFIED `internal/decompose/message.go` (the one guard after the `if herr != nil` block) +
  MODIFIED `internal/decompose/message_test.go` (NEW `TestPublishCommit_HookEmptiesMessage_Aborts` TDD
  regression test). NO other file. NO go.mod change. OUTPUT: `publishCommit` returns `("", generate.ErrEmptyMessage)`
  when a hook empties the message; no commit is created (HEAD unchanged); the bare error propagates through
  runLoop's existing hard-error path to exit 1. DOCS: [Mode A] none — the abort matches git's "Aborting
  commit due to empty commit message." + the existing `--edit` path + S1's CommitStaged guard.

---

## Goal

**Feature Goal**: Close the Issue-4 git-parity gap on the decompose path: a `prepare-commit-msg` or
`commit-msg` hook that empties the message file must abort (exit 1, no commit) — not create a commit with
an empty message (invalid git state that `git commit` refuses). Add the missing empty-message guard in
`decompose.publishCommit` after `hooks.RunCommitHooks` returns, mirroring S1's `CommitStaged` guard,
S2's `runPipeline` guard, the existing `--edit` path (`generate.EditMessage`), and `git commit`'s
"Aborting commit due to empty commit message." This is the THIRD and FINAL Issue-4 call site; after it,
every commit path guards an emptied message.

**Deliverable** (MODIFY existing files only):
1. `internal/decompose/message.go` — insert `if strings.TrimSpace(finalMsg) == "" { return "",
   generate.ErrEmptyMessage }` (with the §9.25 git-parity comment) after the closing `}` of the
   `if herr != nil { return "", herr }` block (~line 234), before `CommitTree` (~line 235).
2. `internal/decompose/message_test.go` — add `TestPublishCommit_HookEmptiesMessage_Aborts` (the TDD
   regression test).

**Success Definition**: the new test FAILS on the unfixed tree (`publishCommit` creates an empty-message
commit → err==nil, HEAD moved) and PASSES after the guard (`errors.Is(err, generate.ErrEmptyMessage)` +
HEAD unchanged); the existing publishCommit/hook tests stay green; `go build/vet/test ./...` green; only
the two files changed; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: A user running `stagecoach` in decompose mode (multi-commit decomposition, PRD §13.6) whose
`commit-msg` or `prepare-commit-msg` hook empties the message file to reject a commit (a commitlint that
rejects, a "force re-edit" hook, or a buggy hook that truncates). Transitively: git parity (the decompose
path must behave like `git commit`).

**Use Case**: A `commit-msg` hook runs `> "$1"; exit 0` to reject the message for concept i. Under the bug,
`publishCommit` creates a commit for concept i with an EMPTY message (invalid git state) and moves on —
silently producing a corrupt multi-commit series. After the fix, the abort is explicit: `publishCommit`
returns `generate.ErrEmptyMessage` → exit 1, concept i is not committed, the user sees "empty commit
message — aborted" exactly as `git commit` produces.

**User Journey**: decompose → ... → `publishCommit(ctx, deps, tree[i], parentSHA, msg[i])` → hooks run →
commit-msg empties the file → `finalMsg=""` → **the guard fires** → `generate.ErrEmptyMessage` → runLoop's
hard-error path → `Decompose()` → CLI → exit 1; HEAD + live index untouched for concept i (the abort is
before `CommitTree`).

**Pain Points Addressed**: A hook that empties the message silently creates a commit with no message — an
invalid git state (`git commit` refuses it unless `--allow-empty-message`), a divergence from git parity,
and one that also breaks downstream concerns (the anti-duplicate subject check, `git log` readability).
The guard makes the abort explicit and git-parity.

## Why

- **Fixes a documented Major git-parity bug (Issue 4) on the third and final call site.** The atomic-commit
  core must not land a bad commit (§9.8/§13.2); `git commit` aborts on an empty message; stagecoach diverged
  on all three commit paths. S1 fixed `CommitStaged`; S2 fixes `runPipeline`; **S3 fixes `publishCommit`** —
  closing Issue 4 entirely.
- **The decompose path is especially dangerous.** A silent empty-message commit mid-series corrupts a
  multi-commit decomposition (an invalid commit lands in HEAD, breaking the CAS chain and `git log`), and
  the empty subject also defeats the anti-duplicate check for subsequent concepts.
- **Mirrors S1 (CommitStaged) + S2 (runPipeline) + the existing `--edit` path.** Same sentinel
  (`generate.ErrEmptyMessage`), same bare propagation (exit 1, NOT rescue). One guard + one test. No
  config/API/flag/doc surface change.

## What

A single empty-message guard added to `publishCommit` (after the `if herr != nil` block, before
`CommitTree`), plus one TDD regression test. No other behavior, signature, file, or dependency changes.

### Success Criteria
- [ ] `publishCommit` checks `strings.TrimSpace(finalMsg) == ""` after the `if herr != nil` block (before
      `CommitTree`) and returns `"", generate.ErrEmptyMessage` (bare, same sentinel as S1's CommitStaged +
      EditMessage + S2's runPipeline).
- [ ] NEW `TestPublishCommit_HookEmptiesMessage_Aborts`: a `commit-msg` hook that empties the file
      (`> "$1"; exit 0`) → `errors.Is(err, generate.ErrEmptyMessage)` + HEAD unchanged (no commit). FAILS
      before the guard (err==nil, HEAD moved), PASSES after.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean.
- [ ] Only `internal/decompose/message.go` + `internal/decompose/message_test.go` changed; go.mod/go.sum
      byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can do this from: the exact guard (below), the
`generate.ErrEmptyMessage` sentinel (exported, exit 1), the `publishCommit` insertion point (after the
`if herr != nil` block, before `CommitTree`), the runLoop propagation verification (the bare error rides
the existing hard-error path), and the closest test template (`TestPublishCommit_PreCommitAbort_RescueError`).
No hook-runner/generate-internals/runLoop-scheduling knowledge required — this is a one-line empty-check +
one test.

### Documentation & References

```yaml
# MUST READ — the design calls (the guard, the variable/return specifics, the runLoop propagation, the test)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S3/research/design-decisions.md
  why: §0 (scope: publishCommit ONLY; S1=done, S2=parallel), §1 (THE guard — finalMsg var, ("",generate.ErrEmptyMessage)
       return), §2 (BARE error → exit 1, NOT rescue/CAS), §3 (runLoop propagation VERIFIED — bare error hits the
       existing `return err` hard path; S3 does NOT touch runLoop), §4 (TDD — mirrors TestPublishCommit_PreCommitAbort_RescueError),
       §5 (no new imports), §6 (reuse msgInstallHook/messageDeps/...), §7 (complementary to EditMessage guard), §8 (test placement).
  critical: §1 (finalMsg NOT msg; return ("", generate.ErrEmptyMessage) NOT (Result{}, ErrEmptyMessage)), §3 (runLoop does NOT
       need changing — the bare error propagates via the existing hard-error path) are the things most likely to be implemented wrong.

# MUST READ — the S1 + S2 CONTRACTS (the SAME-pattern precedents)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S1/PRP.md
  why: S1 added the identical guard to `generate.CommitStaged` (generate.go:436-439: `if strings.TrimSpace(msg)
       == "" { return Result{}, ErrEmptyMessage }`). S3 mirrors it in publishCommit — same sentinel, same bare
       propagation (exit 1), same after-hooks-block placement. The only deltas are the variable name (finalMsg)
       and the return type ((string, error) → ("", generate.ErrEmptyMessage)).
  critical: S1 is DONE. S3 does NOT touch generate.go. The guards are independent (one per call site).
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S2/PRP.md
  why: S2 (parallel) adds the same guard to `pkg/stagecoach.runPipeline`. Confirms the shared Issue-4 contract
       (bare generate.ErrEmptyMessage → exit 1; after the hooks block; before the commit). S3 does NOT touch pkg/stagecoach.
  critical: zero file overlap with S2 (pkg/stagecoach) and S1 (internal/generate). S3 is internal/decompose ONLY.

# MUST READ — the caller analysis (the publishCommit hooks block + the gap)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/docs/architecture/hooks_runner_and_callers.md
  section: "### Caller (c): decompose.publishCommit — message.go:230-235" — the hooks block + "Empty-message
           check: NONE" + the ErrEmptyMessage sentinel reference.
  why: confirms the gap (no empty check between RunCommitHooks and CommitTree) + the exact structure S3 edits.

# The bug spec (in your context as selected_prd_content)
- file: plan/010_…/bugfix/001_d93268e01058/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 4" (h3.3) — the exact reproduction (commit-msg `> "$1"`; git aborts exit 1; stagecoach creates
           an empty-message commit) + the suggested fix (guard after RunCommitHooks, return ErrEmptyMessage) +
           the explicit naming of the decompose publishCommit call site.
  critical: the fix returns the BARE generate.ErrEmptyMessage (exit 1), mirroring EditMessage + S1 — NOT a rescue.

# THE FILE BEING FIXED — READ publishCommit + the hooks block + CommitTree before editing
- file: internal/decompose/message.go
  section: `publishCommit` (line ~224) — the hooks block (`finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx,
           deps.Git, deps.Config, tree, parentSHA, msg, hooks.HookOpts{DryRun:false, Verbose:deps.Verbose})`;
           `if herr != nil { return "", herr }`); then `newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents,
           finalMsg)`. INSERT the guard between the `if herr != nil` block's closing `}` and the `CommitTree` line.
  why: the EXACT insertion point + confirms `finalMsg` is the hook-adjusted message + `publishCommit` returns
       `(string, error)`.
  critical: do NOT touch the hooks block internals, CommitTree, the UpdateRefCAS/CASError handling, RunPostCommit,
       generateMessage, or the package doc-comments. Only the guard is added.

# The sentinel (exported — consumed via the generate import)
- file: internal/generate/finalize.go
  section: `var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")` (L45, EXPORTED);
           `EditMessage`'s `if edited == "" { return "", ErrEmptyMessage }` (L117-118).
  why: the sentinel to return (decompose imports generate ⇒ reference as generate.ErrEmptyMessage).
  critical: it is a BARE error (not *RescueError, not *CASError) → exitcode.For() → exit 1 (exitcode.go:65).

# The runLoop propagation proof — READ to confirm S3 does NOT need to touch runLoop
- file: internal/decompose/decompose.go
  section: runLoop's publishCommit error handling (~L484-491): `newSHA, err := publishCommit(...); if err != nil {
           var ce *generate.CASError; if errors.As(err, &ce) { ...FR-M12b CAS...; return ce }; return err }`. Also
           the single-concept path (~L336) and arbiter path (~L390): `if err != nil { return DecomposeResult{}, err }`.
  why: PROVES a bare generate.ErrEmptyMessage from publishCommit hits the `return err` HARD path (errors.As(&ce)
       is false) and propagates verbatim to Decompose() → CLI → exit 1. S3 does NOT change runLoop.
  critical: do NOT add FR-M12 isolation (rescue) for the empty-message case — the work item specifies a BARE
       non-rescue error that propagates as a hard error.

# The test file + the template to mirror
- file: internal/decompose/message_test.go
  section: `TestPublishCommit_PreCommitAbort_RescueError` (L436) — the EXACT template: `bin := stubtest.Build(t)`
           → `repo := t.TempDir()` → `msgInitRepo` → `msgCommitRaw(t, repo, "initial")` → `parentSHA := msgHeadSHA`
           → `msgWriteFile`/`msgStageFile` → `tree := msgGitOut(t, repo, "write-tree")` → `msgInstallHook` →
           `messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))` → `publishCommit(...)` → assert error +
           HEAD unchanged. Helpers `msgInstallHook` (L392), `messageDeps` (L74), `msgHeadSHA` (L66).
  why: the test idiom — `package decompose` (white-box; publishCommit is unexported); the hook is a REAL shell
       script in `<repo>/.git/hooks/commit-msg` (chmod 0755); HEAD-unchanged is asserted via `msgHeadSHA`.
  critical: the test already imports `errors` (L6) + `strings` (L11) + `generate` (L16) ⇒ NO new import. The `msg`
       arg passed to publishCommit must be NON-empty (so the HOOK empties it, not the caller).

# The exit-code mapping (verify exit 1, no CLI change needed)
- file: internal/exitcode/exitcode.go
  section: L65 `if errors.Is(err, generate.ErrEmptyMessage) { ... return Error }` (exit 1).
  why: confirms the bare generate.ErrEmptyMessage → exit 1 via the EXISTING CLI mapping. No exitcode/CLI change.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  message.go          # publishCommit (~L224; hooks block ~L230-234; CommitTree ~L235) — EDIT (+guard)
  message_test.go     # TestPublishCommit_* (incl. _PreCommitAbort_RescueError at L436) — EDIT (+_HookEmptiesMessage_Aborts)
  decompose.go        # runLoop publishCommit error handling (~L484-491) + single-concept/arbiter paths — UNCHANGED (propagation is existing)
  chain.go            # resolveArbiter/resolveNewCommit — UNCHANGED
  ...                 # planner/stager/arbiter — UNCHANGED
internal/generate/
  generate.go         # S1's CommitStaged guard (L436-439) — UNCHANGED (the precedent; S3 mirrors it)
  finalize.go         # ErrEmptyMessage (L45) — UNCHANGED (the sentinel, consumed as generate.ErrEmptyMessage)
internal/exitcode/
  exitcode.go         # ErrEmptyMessage → exit 1 (L65) — UNCHANGED (existing mapping)
go.mod / go.sum       # UNCHANGED (strings + generate already imported; no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits: internal/decompose/message.go (the guard) + message_test.go (the test).
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (#1 — the variable is finalMsg, the return is ("", generate.ErrEmptyMessage)): publishCommit keeps the
//   hook result in a SEPARATE finalTree/finalMsg pair (it does NOT reassign tree/msg like S1/S2). So the guard reads
//   `finalMsg` (NOT `msg`). publishCommit returns (string, error) where the string is the new SHA → return the empty-
//   string zero value: `return "", generate.ErrEmptyMessage` (mirrors `return "", herr` one line above). Reference the
//   sentinel cross-package: generate.ErrEmptyMessage (NOT bare ErrEmptyMessage — decompose ≠ generate). (research §1)

// CRITICAL (#2 — BARE error → exit 1, NOT rescue, NOT CAS): return "", generate.ErrEmptyMessage. ErrEmptyMessage is a
//   bare error (not *RescueError/exit 3, not *CASError/§13.5 partial). exitcode.go:65 maps it → exit 1. (research §2)

// CRITICAL (#3 — runLoop does NOT need changing): runLoop's publishCommit error handling does `errors.As(err, &ce)`
//   (→ false for a bare ErrEmptyMessage) then `return err` (HARD). The bare error propagates via that existing path.
//   S3 does NOT touch runLoop, Decompose(), the CLI, or exitcode. (research §3)

// CRITICAL (#4 — TDD: the test FAILS before the guard): before the fix, a commit-msg that empties the file →
//   RunCommitHooks returns finalMsg="" → CommitTree creates an empty-message commit (git commit-tree does NOT refuse
//   empty messages — Issue 4) → publishCommit returns (newSHA, nil), HEAD moved. So the test's errors.Is fails
//   (err==nil) AND HEAD moved → FAILS on the unfixed tree. Run it FIRST (TDD proof), then add the guard → PASSES.

// GOTCHA (publishCommit ONLY — scope): the same gap is fixed in CommitStaged (S1, DONE) and runPipeline (S2, parallel).
//   Fix ONLY publishCommit here; do NOT touch internal/generate, pkg/stagecoach, runLoop, or Decompose().
// GOTCHA (commit-msg hook empties the file): use msgInstallHook(t, repo, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n").
//   commit-msg is the LAST hook to touch the message file ⇒ unambiguous finalMsg="". (prepare-commit-msg would also
//   trigger it, but commit-msg is the work-item-specified, cleaner reproduction.)
// GOTCHA (the msg arg passed to publishCommit must be NON-empty): pass e.g. "feat: add new" (the HOOK empties it,
//   not the caller) — otherwise the test would exercise a different path.
// GOTCHA (no new imports): message.go imports strings (L34) + generate (L37); message_test.go imports errors (L6) +
//   strings (L11) + generate (L16). go mod tidy is a no-op.
// GOTCHA (HEAD-unchanged assertion): assert via msgHeadSHA(t, repo) before/after — the abort returns before
//   CommitTree+UpdateRefCAS, so HEAD is identical (FR-V7 idempotent, like the pre-commit-abort test).
```

## Implementation Blueprint

### Data models and structure

No new types. The guard + the one test:

```go
// internal/decompose/message.go — publishCommit: INSERT the guard after the `if herr != nil` block, before CommitTree.
//
// (current, for orientation — the hooks block + what follows:)
//	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
//		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
//	if herr != nil {
//		return "", herr // *generate.RescueError (FR-V7) — propagate DIRECTLY (not wrapped)
//	}
//  ↓↓↓ INSERT THIS GUARD ↓↓↓
	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file
	// (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit message.";
	// mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit path
	// (generate.EditMessage), generate.CommitStaged's guard (S1), and runPipeline's guard (S2). The bare
	// error propagates via runLoop's existing hard-error path (it is not *CASError, so it skips FR-M12b).
	// HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(finalMsg) == "" {
		return "", generate.ErrEmptyMessage
	}
//  ↑↑↑ END INSERT ↑↑↑
//	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
//	if err != nil {
//		return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err)
//	}
//	... (UpdateRefCAS / CASError / RunPostCommit unchanged)
```

```go
// internal/decompose/message_test.go (package decompose) — ADD the TDD regression test.

// TestPublishCommit_HookEmptiesMessage_Aborts is the Issue-4 guard on the decompose path: a commit-msg hook
// that empties the message file must NOT create an empty-message commit. git aborts "Aborting commit due to
// empty commit message."; stagecoach returns the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue — it is
// neither *RescueError nor *CASError). Mirrors TestPublishCommit_PreCommitAbort_RescueError's structure but
// swaps the hook (commit-msg `> "$1"; exit 0`) + the assertion (errors.Is(err, generate.ErrEmptyMessage)).
// FAILS before the guard (publishCommit creates an empty-message commit → err==nil, HEAD moved); PASSES after.
func TestPublishCommit_HookEmptiesMessage_Aborts(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	// A commit-msg hook that empties the message file (exit 0 ⇒ not a hook failure; the guard catches the
	// EMPTY result). commit-msg is the last hook to touch the file ⇒ finalMsg unambiguously "".
	msgInstallHook(t, repo, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	_, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new") // NON-empty msg (the hook empties it)
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("error type = %T, want generate.ErrEmptyMessage (bare, exit 1 — NOT *RescueError/exit 3, NOT *CASError): %v", err, err)
	}
	// The error must NOT be a rescue (exit 3) or a CAS partial — and must NOT be wrapped in ErrPublicationFailed.
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Errorf("error is *generate.RescueError (exit 3) — the empty-message abort must be the BARE generate.ErrEmptyMessage (exit 1)")
	}
	if errors.Is(err, ErrPublicationFailed) {
		t.Errorf("error is wrapped in ErrPublicationFailed — generate.ErrEmptyMessage must propagate DIRECTLY")
	}

	// NO commit created (HEAD unchanged — the abort returned before CommitTree+UpdateRefCAS; FR-V7 idempotent).
	if got := msgHeadSHA(t, repo); got != parentSHA {
		t.Errorf("HEAD = %q, want %q (unchanged — the empty-message abort returned before CommitTree)", got, parentSHA)
	}
}
```

### Implementation Tasks (ordered by dependencies — TDD: test first, then fix)

```yaml
Task 1: ADD the FAILING test (message_test.go) — write the regression test FIRST
  - ADD TestPublishCommit_HookEmptiesMessage_Aborts per the Blueprint (mirrors TestPublishCommit_PreCommitAbort_RescueError:
      bin/repo/msgInitRepo/msgCommitRaw/parentSHA/msgWriteFile/msgStageFile/tree + a commit-msg `> "$1"; exit 0` hook
      via msgInstallHook + messageDeps + publishCommit(tree, parentSHA, "feat: add new") + assert errors.Is(err,
      generate.ErrEmptyMessage) + NOT *RescueError + NOT ErrPublicationFailed-wrapped + HEAD unchanged).
  - RUN on the UNFIXED tree: `go test ./internal/decompose/ -run TestPublishCommit_HookEmptiesMessage_Aborts -v`
      → it MUST FAIL (err==nil → t.Fatal; or HEAD moved). This is the TDD proof the test reproduces Issue 4.
      (If it passes before the fix, the hook didn't run or the msg arg was empty — check messageDeps wired Config
      + the hook body + that "feat: add new" is passed.)
  - GOTCHA: the msg arg to publishCommit must be NON-empty ("feat: add new"); the HOOK empties it.
  - GOTCHA: message_test.go already imports errors+strings+generate — NO new import.

Task 2: ADD the guard (message.go) — the fix
  - INSERT `if strings.TrimSpace(finalMsg) == "" { return "", generate.ErrEmptyMessage }` (with the §9.25 git-parity
      comment) AFTER the closing `}` of the `if herr != nil { return "", herr }` block (~L234), BEFORE
      `newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)` (~L235).
  - USE `finalMsg` (NOT msg) + `generate.ErrEmptyMessage` (cross-package) + return ("", generate.ErrEmptyMessage).
  - strings (L34) + generate (L37) already imported — NO new import.
  - DO NOT touch the hooks block internals, CommitTree, the UpdateRefCAS/CASError handling, RunPostCommit,
      generateMessage, or the package doc-comments.

Task 3: VERIFY (TDD green + no regression)
  - RUN Task 1's test again → now PASS (err == generate.ErrEmptyMessage; HEAD unchanged).
  - RUN the full suite: `go build ./... && go vet ./... && go test ./...` → GREEN.
  - CONFIRM the existing publishCommit/hook tests (TestPublishCommit_Success, _RootCommit, _CASFailure,
      _PrepareCommitMsgAnnotates, _PreCommitAbort_RescueError) stay green; go.mod/go.sum byte-unchanged;
      only message.go + message_test.go modified; generate.go (S1) + pkg/stagecoach (S2) untouched.
```

### Implementation Patterns & Key Details

```go
// THE guard (bare generate.ErrEmptyMessage → exit 1, mirroring S1's CommitStaged + S2's runPipeline + EditMessage + git):
if strings.TrimSpace(finalMsg) == "" {
	return "", generate.ErrEmptyMessage
}
// THE test assertion (bare error, NOT *RescueError, NOT *CASError, NOT ErrPublicationFailed-wrapped):
if !errors.Is(err, generate.ErrEmptyMessage) { t.Errorf(...) }   // NOT errors.As(&re), NOT errors.As(&ce)
// THE HEAD-unchanged assertion (no commit created — the abort returned before CommitTree+UpdateRefCAS):
if got := msgHeadSHA(t, repo); got != parentSHA { t.Errorf("HEAD moved ...") }
// THE hook install (commit-msg empties the file — last hook to touch it ⇒ unambiguous finalMsg=""):
msgInstallHook(t, repo, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. The guard uses already-imported `strings` + `generate`. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE. The guard is intra-package (decompose); no new import. message_test.go already imports errors/strings/generate.

UPSTREAM (the inputs — consume, do NOT edit):
  - hooks.RunCommitHooks (decompose imports hooks directly — NOT via an interface) — returns (finalTree, finalMsg, herr).
  - generate.ErrEmptyMessage (finalize.go:45) — the exported sentinel.

DOWNSTREAM: the runLoop / single-concept / arbiter call sites propagate the bare generate.ErrEmptyMessage via
      their EXISTING error handling (runLoop: errors.As(&ce)→false→`return err` HARD; single-concept/arbiter:
      `return DecomposeResult{}, err`). The CLI maps it → exit 1 (exitcode.go:65). S1's CommitStaged guard +
      the existing --edit path already do this; the publishCommit guard now matches them.

FROZEN/LEAVE (do NOT edit):
  - The hooks block internals (RunCommitHooks call), CommitTree, the UpdateRefCAS/CASError handling, RunPostCommit,
    generateMessage, the package doc-comments in message.go.
  - internal/generate/* (S1's CommitStaged guard — DONE), pkg/stagecoach/* (S2's runPipeline guard — parallel),
    internal/decompose/decompose.go (runLoop), internal/decompose/chain.go, internal/hooks/*, internal/cmd/*,
    internal/config/*, internal/exitcode/*.
  - PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/decompose/message.go internal/decompose/message_test.go
go vet ./internal/decompose/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go.mod/go.sum byte-unchanged (strings + generate already in scope).
```

### Level 2: The regression test (TDD — fail before, pass after)

```bash
# BEFORE the guard (Task 1 only, Task 2 not yet applied) — the test MUST FAIL (reproduces Issue 4):
go test ./internal/decompose/ -run TestPublishCommit_HookEmptiesMessage_Aborts -v
# Expected: FAIL — "expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created)".

# AFTER the guard (Task 2 applied) — the test MUST PASS:
go test ./internal/decompose/ -run TestPublishCommit_HookEmptiesMessage_Aborts -v
# Expected: PASS — errors.Is(err, generate.ErrEmptyMessage); HEAD unchanged (parentSHA == parentSHA).
# If the test passed BEFORE the fix, it does not reproduce the bug (check the hook ran + the msg arg is non-empty).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...    # Expect clean.
go test ./...     # Expect all PASS — incl. the existing publishCommit tests (TestPublishCommit_Success, _RootCommit,
                  # _CASFailure, _PrepareCommitMsgAnnotates, _PreCommitAbort_RescueError) which stay green.
# Confirm only the two intended files changed:
git diff --name-only | grep -vE '^internal/decompose/(message|message_test)\.go$' && echo "UNEXPECTED file changed" || echo "only message.go + message_test.go (good)"
# Confirm frozen files UNCHANGED (S1's generate.go guard intact, S2's pkg/stagecoach untouched, runLoop untouched):
git diff --exit-code go.mod go.sum internal/generate pkg/stagecoach internal/decompose/decompose.go internal/decompose/chain.go internal/hooks internal/cmd internal/config internal/exitcode PRD.md && echo "frozen files UNCHANGED (expected)"
grep -n 'TrimSpace(finalMsg) == ""' internal/decompose/message.go && echo "(S3 guard present)"
grep -n 'TrimSpace(msg) == ""' internal/generate/generate.go && echo "(S1 guard intact — untouched)"
```

### Level 4: Git-parity reasoning (no runtime to start)

```bash
# The fix is a one-line empty-check. Verify by reasoning + the test:
#   1. A commit-msg (or prepare-commit-msg) hook empties the file (exit 0) → RunCommitHooks returns finalMsg=""
#      → the `if herr != nil` block is skipped (herr==nil) → the guard fires → generate.ErrEmptyMessage (bare) → exit 1.
#   2. runLoop propagation: the bare error is not *CASError ⇒ errors.As(&ce) is false ⇒ it skips FR-M12b and hits
#      `return err` (HARD) ⇒ Decompose() ⇒ CLI ⇒ exit 1. Prior committed concepts stand (already in HEAD); concept i
#      is NOT committed (CommitTree never ran). (the test asserts HEAD unchanged for the single-concept direct call)
#   3. The sentinel + bare propagation match S1's CommitStaged guard + S2's runPipeline guard + the existing --edit
#      path + git's "Aborting commit due to empty commit message." — exit 1, NOT exit 0, NOT exit 3.
# (No Level-4 commands beyond Levels 1–3 — the test IS the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the two edited files.
- [ ] `go test ./...` GREEN; the new test FAILS pre-fix / PASSES post-fix.
- [ ] go.mod/go.sum byte-unchanged; only `internal/decompose/message.go` + `message_test.go` modified.

### Feature Validation
- [ ] `publishCommit` returns `"", generate.ErrEmptyMessage` when a hook empties the message (after trimming).
- [ ] No commit created (HEAD unchanged); the abort is BEFORE CommitTree (FR-V7 idempotent).
- [ ] The error is the BARE `generate.ErrEmptyMessage` (exit 1, NOT `*RescueError`/exit 3, NOT `*CASError`, NOT
      ErrPublicationFailed-wrapped) — matches S1 + S2 + EditMessage + git.
- [ ] The bare error propagates via runLoop's EXISTING hard-error path (`return err`); S3 does NOT touch runLoop.
- [ ] The existing publishCommit tests stay green.

### Code Quality Validation
- [ ] Mirrors S1's CommitStaged guard (same sentinel, same bare propagation, same after-hooks-block placement).
- [ ] Test mirrors `TestPublishCommit_PreCommitAbort_RescueError` (same setupTestRepo/msgInstallHook idiom).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (CommitStaged=S1, runPipeline=S2, runLoop/Decompose=frozen).

### Documentation
- [ ] Inline comment cites §9.25 git parity (Issue 4) + the S1/S2/EditMessage/git precedent + the runLoop propagation
      note. No docs/*.md edits (matches git's existing behavior + S1 + the existing --edit path; the changeset doc
      sync is P1.M5).

---

## Anti-Patterns to Avoid

- ❌ **Don't use `msg` (use `finalMsg`) or return `Result{}` (publishCommit returns `(string, error)`).** The
      guard is `if strings.TrimSpace(finalMsg) == "" { return "", generate.ErrEmptyMessage }`. publishCommit keeps
      the hook result in `finalTree`/`finalMsg` (it does not reassign `tree`/`msg` like S1/S2). (research §1)
- ❌ **Don't reference bare `ErrEmptyMessage`.** decompose ≠ generate; use `generate.ErrEmptyMessage` (cross-package).
- ❌ **Don't wrap the error as `*generate.RescueError` (exit 3) or `*generate.CASError` (§13.5) or in
      `ErrPublicationFailed`.** The empty-message abort is the BARE `generate.ErrEmptyMessage` → exit 1, mirroring
      S1 + S2 + EditMessage + git. (research §2)
- ❌ **Don't touch `runLoop`, `Decompose()`, the CLI, or exitcode.** The bare error rides the EXISTING hard-error
      propagation (`return err` after the `errors.As(&ce)` check). Adding FR-M12 isolation/rescue for the empty
      case is out of scope and contradicts the work item ("bare non-rescue error"). (research §3)
- ❌ **Don't write a test that passes before the fix.** It MUST fail on the unfixed tree (publishCommit creates an
      empty-message commit → err==nil, HEAD moved). Use a NON-empty msg arg + a commit-msg hook that empties it. (research §4)
- ❌ **Don't fix `generate.CommitStaged` (S1, done), `runPipeline` (S2, parallel), or any other call site here.**
      This task is `decompose.publishCommit` ONLY. (research §0)
- ❌ **Don't touch the hooks block internals / CommitTree / UpdateRefCAS-CASError / RunPostCommit / generateMessage /
      the package doc-comments.** Only the guard (between the `if herr != nil` block and `CommitTree`) is added.
- ❌ **Don't add imports/deps.** `strings` + `generate` are already imported in message.go; `errors` + `strings` +
      `generate` in message_test.go. (research §5)

---

## Confidence Score

**10/10** — a contract-specified, line-accurate one-guard bugfix that mirrors an ALREADY-SHIPPED in-repo pattern
(S1's `generate.CommitStaged` guard at generate.go:436-439, and S2's parallel `runPipeline` guard) and an existing
in-repo test (`TestPublishCommit_PreCommitAbort_RescueError` at message_test.go:436 — the closest template, swapped
hook+assertion). The `publishCommit` hooks block (message.go:230-234) + the `CommitTree` call (line 235) are read
precisely (the insertion gap is unambiguous); the two `publishCommit`-specific deltas — the `finalMsg` variable (not
`msg`) and the `("", generate.ErrEmptyMessage)` return (not `Result{}`) — are explicitly specified. The runLoop
propagation (decompose.go:484-491) is VERIFIED: the bare `generate.ErrEmptyMessage` is not `*CASError`, so
`errors.As(&ce)` is false and it rides the existing `return err` hard path — no runLoop/CLI/exitcode change needed
(exitcode.go:65 already maps `generate.ErrEmptyMessage` → exit 1). `strings` + `generate` are already imported in
both message.go and message_test.go. Zero file overlap with S1 (generate.go, DONE) or S2 (pkg/stagecoach, parallel).
The TDD test is engineered to fail-before/pass-after, reproducing the exact bug (empty-message commit, HEAD moved)
then proving the abort. No residual risk.
