---
name: "P1.M3.T1.S1 — Add empty-message guard in generate.CommitStaged after RunCommitHooks (Issue 4: a hook that empties the message file must abort, not create an empty-message commit)"
description: |

  Bugfix for Issue 4 (Bug-Fix PRD §h2.2/h3.3; stagehand PRD §9.25 FR-V2 git parity + §9.8/§13.2 atomic-commit
  core). After `RunCommitHooks` returns, `generate.CommitStaged` reassigns `msg = fm` (the hook-adjusted
  message) and passes it straight to `CommitTree` with NO empty-message check (generate.go:426-439). A
  `prepare-commit-msg` or `commit-msg` hook that empties the message file (a common rejection / force-re-edit
  pattern) therefore creates a commit with an EMPTY message — an invalid git state that `git commit` refuses
  ("Aborting commit due to empty commit message.", exit 1). The `--edit` path (EditMessage) DOES guard empty
  (returns ErrEmptyMessage), but hooks run AFTER the editor, so a hook can empty a message the editor already
  validated. This task adds the missing guard: after the hooks block, if the finalized message is empty
  (after trimming), abort with the SAME bare `ErrEmptyMessage` (exit 1, NOT exit 3 rescue).

  THE FIX — one guard inserted in `CommitStaged`, after the `if deps.Hooks != nil { ... }` block (after
  `treeSHA, msg = ft, fm`), before the `parents`/`CommitTree` block:
    ```go
    if strings.TrimSpace(msg) == "" {
        return Result{}, ErrEmptyMessage // §9.25 git parity (Issue 4): a hook emptied the message → abort (exit 1, NOT rescue)
    }
    ```
  `ErrEmptyMessage` is the package-local sentinel (`internal/generate/finalize.go:45`, same package — no
  import); `strings` is already imported in generate.go. The abort is BARE (not `*RescueError`) → exit 1,
  mirroring EditMessage's path. HEAD + live index are untouched (the abort returns before CommitTree → no
  update-ref ran) — a clean abort.

  ⚠️ **#1 — TDD: write the FAILING test first.** Add `TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort`
       in `internal/generate/hooks_freeze_test.go`: install a `commit-msg` hook that empties the file
       (`> "$1"`; exit 0), call CommitStaged with a stub provider (outputs a NON-empty message), assert
       `errors.Is(err, generate.ErrEmptyMessage)` + NO commit created (HEAD unchanged). BEFORE the guard it
       FAILS (CommitStaged succeeds → err==nil → t.Fatal); AFTER it PASSES. (verification §4)

  ⚠️ **#2 — ErrEmptyMessage is the right sentinel (BARE → exit 1, NOT rescue).** It is the same `var` the
       `--edit` path already returns (finalize.go:45/117-118; CommitStaged propagates it bare at generate.go
       ~411). Do NOT wrap it as `*RescueError` (that would be exit 3); do NOT invent a new sentinel. Same
       package (`generate`) — reference it unqualified. (verification §2/§3)

  ⚠️ **#3 — place the guard AFTER the hooks block (before CommitTree).** After `if deps.Hooks != nil { ... }`'s
       closing `}` (after `treeSHA, msg = ft, fm`), before the `parents`/`CommitTree` block. This guards the
       FINAL msg unconditionally before CommitTree (the contract's literal "after line 431" = inside-the-block
       reading is equally valid and guards the hooks-ran case specifically; the after-block placement is
       strictly more defensive at zero cost — pick either, recommend after-block). (verification §3)

  ⚠️ **#4 — CommitStaged ONLY (scope boundary).** Issue 4 names three call sites with the same gap:
       CommitStaged (this task), runPipeline (S2/P1.M3.T1.S2), publishCommit (S3/P1.M3.T1.S3). Fix ONLY
       CommitStaged here; do NOT touch pkg/stagehand or internal/decompose. (verification §6)

  ⚠️ **#5 — No conflict with the parallel work item.** P1.M2.T1.S1 (Issue 3, no_verify git-config) touches
       docs/*, internal/cmd/root.go, internal/config/*, internal/hooks/runner.go — NOT generate.go or
       hooks_freeze_test.go. Zero file overlap. (verification §5)

  Deliverable: MODIFIED `internal/generate/generate.go` (the one-line guard after the hooks block) + MODIFIED
  `internal/generate/hooks_freeze_test.go` (NEW `TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort`).
  NO other file. NO go.mod change. OUTPUT: CommitStaged returns `Result{}, ErrEmptyMessage` when a hook empties
  the message; no commit created; the error propagates bare → exit 1 (NOT exit 3). DOCS: none — matches git's
  existing 'Aborting commit due to empty commit message.' + the existing --edit path.

---

## Goal

**Feature Goal**: Close the Issue-4 git-parity gap: a `prepare-commit-msg` or `commit-msg` hook that empties
the message file must abort the commit (exit 1, no commit created) — not create a commit with an empty
message. Add the missing empty-message guard in `generate.CommitStaged` after `RunCommitHooks` returns,
mirroring the existing `--edit` path's `ErrEmptyMessage` abort and `git commit`'s "Aborting commit due to
empty commit message."

**Deliverable** (MODIFY existing files only):
1. `internal/generate/generate.go` — insert `if strings.TrimSpace(msg) == "" { return Result{}, ErrEmptyMessage }`
   after the hooks block, before CommitTree.
2. `internal/generate/hooks_freeze_test.go` — add `TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort`
   (commit-msg empties file → `ErrEmptyMessage` + no commit).

**Success Definition**: the new test FAILS on the unfixed tree (a commit with an empty message is created →
err==nil) and PASSES after the guard (`errors.Is(err, generate.ErrEmptyMessage)` + HEAD unchanged); the
existing hook tests stay green; `go build/vet/test ./...` green; only the two files changed; go.mod/go.sum
byte-unchanged.

## User Persona

**Target User**: A user whose `commit-msg` or `prepare-commit-msg` hook empties the message file to reject a
commit (a conventional-commit lint that rejects, a "force re-edit" hook, or a buggy hook that truncates).
Transitively: git-parity (the commit path must behave like `git commit`).

**Use Case**: A `commit-msg` hook runs `> "$1"; exit 0` to reject the message. Under the bug, stagehand
creates a commit with an empty message (invalid git state). After the fix, stagehand aborts with "empty
commit message — aborted" (exit 1, no commit) — exactly what `git commit` does.

**User Journey**: `stagehand` → generation produces a message → hooks run → commit-msg empties the file →
**the guard fires** → `ErrEmptyMessage` → exit 1, no commit, HEAD + index untouched.

**Pain Points Addressed**: A hook that empties the message silently creates an invalid empty-message commit
(breaks `git log`, the anti-duplicate subject check, and git's own contract). The guard makes the abort
explicit and git-parity.

## Why

- **Fixes a documented Major git-parity bug (Issue 4).** The atomic-commit core must not land a bad commit
  (§9.8/§13.2); `git commit` aborts on an empty message; stagehand diverged.
- **Mirrors the existing `--edit` path.** `EditMessage` already returns `ErrEmptyMessage` for an empty edited
  message; the hooks path (which runs AFTER the editor) lacked the same guard. This makes the two paths
  consistent.
- **Cheap, surgical, no-surface-change.** One guard line + one test. No config/API/flag/doc surface change
  (the abort matches git's existing behavior + the existing sentinel).

## What

A single empty-message guard added to `CommitStaged` (after the hooks block, before CommitTree), plus one
TDD regression test. No other behavior, signature, file, or dependency changes.

### Success Criteria
- [ ] `CommitStaged` checks `strings.TrimSpace(msg) == ""` after the hooks block and returns
      `Result{}, ErrEmptyMessage` (bare, same sentinel as EditMessage).
- [ ] NEW `TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort`: a commit-msg hook that empties the
      file → `errors.Is(err, generate.ErrEmptyMessage)` + HEAD unchanged (no commit). FAILS before the guard,
      PASSES after.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean.
- [ ] Only `internal/generate/generate.go` + `hooks_freeze_test.go` changed; go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can do this from: the exact guard (below), the ErrEmptyMessage
sentinel (same package), the exact test template (`TestCommitStaged_PreCommitAbort_IsRescue`), and the TDD
ordering. No hook-runner/generate-internals knowledge required — this is a one-line empty-check + a test.

### Documentation & References

```yaml
# MUST READ — the live-tree verification (gap, sentinel, fix, test, no-conflict)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S1/research/verification.md
  why: §1 (the gap — no empty check after hooks), §2 (ErrEmptyMessage, bare→exit 1), §3 (the fix + placement),
       §4 (the TDD test mirroring PreCommitAbort_IsRescue), §5 (no conflict with P1.M2.T1.S1), §6 (scope: CommitStaged only).
  critical: §2 (ErrEmptyMessage, NOT RescueError — exit 1 not 3) and §4 (the test FAILS before / PASSES after).

# The bug spec (in your context as selected_prd_content)
- file: plan/010_…/bugfix/001_d93268e01058/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 4" (h3.3) — the exact reproduction (commit-msg `> "$1"`; git aborts exit 1; stagehand
           creates an empty-message commit) + the suggested fix (guard after RunCommitHooks, return ErrEmptyMessage).
  critical: the fix returns the BARE ErrEmptyMessage (exit 1), mirroring EditMessage — NOT a rescue.

# The file being fixed — READ the hooks block + CommitTree before editing
- file: internal/generate/generate.go
  section: CommitStaged — the `--edit` path (~L410-413, propagates EditMessage's ErrEmptyMessage bare); the
           hooks block (L426-432: `if deps.Hooks != nil { ...; treeSHA, msg = ft, fm }`); the parents/CommitTree
           block (L434-439). INSERT the guard after the hooks block's `}`, before the parents block.
  why: the EXACT insertion point + the EditMessage precedent (the guard mirrors it).
  critical: do NOT touch the hooks block internals, CommitTree, UpdateRefCAS, or the signal handling.

# The sentinel (same package)
- file: internal/generate/finalize.go
  section: `var ErrEmptyMessage = errors.New(...)` (L45); `EditMessage`'s `if edited == "" { return "", ErrEmptyMessage }`
           (L117-118).
  why: the sentinel to return (same package `generate` — reference unqualified; no import).
  critical: it is a BARE error (not *RescueError) → exitcode.For() → exit 1 (NOT exit 3).

# The test file + the template to mirror
- file: internal/generate/hooks_freeze_test.go
  section: `TestCommitStaged_PreCommitAbort_IsRescue` (L163-210) — the EXACT template: `initTempRepo`, modify+`git add`,
           install a shell-script hook (`os.WriteFile(..., 0o755)`), capture `headBefore`, `bin := stubtest.Build(t)`,
           `stubtest.Manifest(bin, stubtest.Options{Out: ...})`, `deps := generate.Deps{Git: git.New(repo), Manifest: m,
           Hooks: hooks.DefaultRunner{}}`, `generate.CommitStaged(...)`, assert error + HEAD unchanged.
  why: the test idiom — EXTERNAL `package generate_test` (required: white-box can't import internal/hooks — cycle);
       `generate.ErrEmptyMessage` is exported so the external test references it.
  critical: the new test asserts `errors.Is(err, generate.ErrEmptyMessage)` (NOT `errors.As(&re)` — this is a bare
       abort, not a RescueError) AND HEAD unchanged (no commit created).

# The parallel PRP — the no-conflict confirmation
- file: plan/010_…/bugfix/001_d93268e01058/P1M2T1S1/PRP.md
  why: confirms P1.M2.T1.S1 (Issue 3, no_verify git-config) touches docs/* + cmd/root.go + config/* + hooks/runner.go
       — NOT generate.go or hooks_freeze_test.go. Zero file overlap ⇒ independent.
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go             # CommitStaged (L426-439 hooks block + CommitTree) — EDIT (+the guard after the hooks block)
  finalize.go             # ErrEmptyMessage (L45) — UNCHANGED (the sentinel, consumed)
  hooks_freeze_test.go    # external package generate_test — EDIT (+TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort)
go.mod / go.sum           # UNCHANGED (no new dep — strings + errors already imported)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits: internal/generate/generate.go (the guard) + hooks_freeze_test.go (the test).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ErrEmptyMessage is BARE → exit 1, NOT a rescue): return Result{}, ErrEmptyMessage (same sentinel
// EditMessage uses). Do NOT wrap as *generate.RescueError (that's exit 3); do NOT invent a new error. Same
// package (generate) — reference it unqualified. (verification §2)

// CRITICAL (TDD: test FAILS before the guard): before the fix, CommitStaged succeeds (err==nil) and creates a
// commit with an empty message → the test's `err == nil` → t.Fatal. Run the test on the UNFIXED tree first to
// confirm it reproduces the bug; then add the guard → it PASSES. (verification §4)

// CRITICAL (placement: after the hooks block, before CommitTree): the guard sits between `if deps.Hooks != nil
// { ... }`'s closing `}` and the `parents`/`CommitTree` block. It guards the FINAL msg unconditionally. (verification §3)

// GOTCHA (CommitStaged ONLY — scope): the same gap exists in runPipeline (pkg/stagehand) and publishCommit
// (decompose) — those are S2 (P1.M3.T1.S2) and S3 (P1.M3.T1.S3). Do NOT touch them here.
// GOTCHA (external test package): hooks_freeze_test.go is `package generate_test` (NOT `package generate`) so it
// can import internal/hooks. Reference the sentinel as `generate.ErrEmptyMessage`.
// GOTCHA (stub must output NON-empty): the stub Manifest's Out must be a real message (e.g. "feat: change") so
// generation succeeds and the hook (not the generator) is what empties it.
// GOTCHA (strings already imported): generate.go imports "strings" — no new import.
```

## Implementation Blueprint

### Data models and structure

No new types. The guard + the test:

```go
// internal/generate/generate.go — CommitStaged: INSERT the guard after the hooks block, before the parents block.
//
// (current, for orientation — the hooks block + what follows:)
//	if deps.Hooks != nil {
//		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)
//		if herr != nil {
//			return Result{}, herr // *RescueError (FR-V7) or ErrHookSweptConcurrentWork (FR-V3)
//		}
//		treeSHA, msg = ft, fm // hook may have re-treed (permitted mutation) + annotated the msg
//	}
//  ↓↓↓ INSERT THIS GUARD ↓↓↓
	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file
	// (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit message.";
	// mirror it — return the BARE ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit path above.
	// HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(msg) == "" {
		return Result{}, ErrEmptyMessage
	}
//  ↑↑↑ END INSERT ↑↑↑
//	// Step 7: commit-tree — build the DANGLING commit object from the FROZEN tree.
//	var parents []string
//	if !isUnborn { parents = []string{parentSHA} }
//	newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)
```

```go
// internal/generate/hooks_freeze_test.go (package generate_test) — ADD the TDD regression test.
// Mirrors TestCommitStaged_PreCommitAbort_IsRescue but: commit-msg empties the file → ErrEmptyMessage (bare).

// TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort is the Issue-4 git-parity guard: a commit-msg
// (or prepare-commit-msg) hook that empties the message file must NOT produce a commit. git aborts "Aborting
// commit due to empty commit message." (exit 1); stagehand returns the BARE generate.ErrEmptyMessage (exit 1,
// NOT a rescue) and creates NO commit (HEAD unchanged). (PRD §9.25 FR-V2 git parity.)
func TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort(t *testing.T) {
	repo := initTempRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-mod\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}

	// A commit-msg hook that empties the message file (a common rejection pattern). exit 0 ⇒ not a hook
	// failure (no *RescueError); the guard catches the EMPTY result.
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "commit-msg"),
		[]byte("#!/bin/sh\n> \"$1\"\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write commit-msg hook: %v", err)
	}

	headBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()

	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: non-empty generated message"}) // non-empty ⇒ generation succeeds; the hook empties it
	cfg := config.Defaults()
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}}

	_, err := generate.CommitStaged(context.Background(), deps, cfg)
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1 — NOT a rescue), got %T: %v", err, err)
	}

	// NO commit created (HEAD unchanged — the abort returned before CommitTree).
	headAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD moved on empty-message abort: %s → %s (a commit was created)", headBefore, headAfter)
	}
}
```

### Implementation Tasks (ordered by dependencies — TDD: test first, then fix)

```yaml
Task 1: ADD the FAILING test (hooks_freeze_test.go) — write the regression test FIRST
  - ADD TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort per the Blueprint (commit-msg `> "$1"; exit 0`;
      stub Out a non-empty message; assert errors.Is(err, generate.ErrEmptyMessage) + HEAD unchanged).
  - RUN on the UNFIXED tree: `go test ./internal/generate/ -run TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort -v`
      → it MUST FAIL (CommitStaged succeeds → err==nil → t.Fatal "expected ErrEmptyMessage, got nil"). This is
      the TDD proof the test reproduces Issue 4. (If it passes before the fix, the test is wrong — the stub
      output or the hook is not triggering the empty-message path.)
  - GOTCHA: the test is `package generate_test` (external) — reference the sentinel as generate.ErrEmptyMessage.

Task 2: ADD the guard (generate.go) — the fix
  - INSERT `if strings.TrimSpace(msg) == "" { return Result{}, ErrEmptyMessage }` (with the §9.25 git-parity
      comment) AFTER the `if deps.Hooks != nil { ... }` block's closing `}`, BEFORE the `parents`/`CommitTree`
      block.
  - USE the package-local ErrEmptyMessage (same package — no import); strings is already imported.
  - DO NOT touch the hooks block internals, CommitTree, UpdateRefCAS, or signal handling.

Task 3: VERIFY (TDD green + no regression)
  - RUN Task 1's test again → now PASSES (err == ErrEmptyMessage; HEAD unchanged).
  - RUN the full suite: `go build ./... && go vet ./... && go test ./...` → GREEN.
  - CONFIRM the existing hook tests (TestCommitStaged_PreCommitFreeze_*, TestCommitStaged_PreCommitAbort_IsRescue)
      stay green; go.mod/go.sum byte-unchanged; only generate.go + hooks_freeze_test.go modified.
```

### Implementation Patterns & Key Details

```go
// THE guard (bare ErrEmptyMessage → exit 1, mirroring EditMessage + git's empty-message abort):
if strings.TrimSpace(msg) == "" {
    return Result{}, ErrEmptyMessage
}
// THE test assertion (bare error, NOT *RescueError):
if !errors.Is(err, generate.ErrEmptyMessage) { t.Errorf(...) }   // NOT errors.As(&re)
// HEAD-unchanged assertion (no commit created — the abort returned before CommitTree):
if string(headBefore) != string(headAfter) { t.Errorf("HEAD moved ...") }
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. The guard uses already-imported `strings` + the same-package
      `ErrEmptyMessage`. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE. The guard is intra-package (generate); no new import.

UPSTREAM (the inputs — consume, do NOT edit):
  - RunCommitHooks (via deps.Hooks) — returns the hook-adjusted (fm) message.
  - ErrEmptyMessage (finalize.go:45) — the sentinel.

DOWNSTREAM: the caller (runGenerate / CLI) propagates the bare ErrEmptyMessage → exitcode.For() → exit 1
      (NOT exit 3). The existing --edit path already does this; the hooks-path guard now matches it.

FROZEN/LEAVE (do NOT edit):
  - The hooks block internals (RunCommitHooks call), CommitTree, UpdateRefCAS, signal handling.
  - internal/hooks/*, internal/decompose/*, pkg/stagehand/*, internal/cmd/*, internal/config/*.
  - runPipeline (pkg/stagehand) + publishCommit (decompose) — those are S2 (P1.M3.T1.S2) + S3 (P1.M3.T1.S3).
  - PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/generate/generate.go internal/generate/hooks_freeze_test.go
go vet ./internal/generate/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go.mod/go.sum byte-unchanged (strings + ErrEmptyMessage already in scope).
```

### Level 2: The regression test (TDD — fail before, pass after)

```bash
# BEFORE the guard (Task 1 only, Task 2 not yet applied) — the test MUST FAIL (reproduces Issue 4):
go test ./internal/generate/ -run TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort -v
# Expected: FAIL — "expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created)".

# AFTER the guard (Task 2 applied) — the test MUST PASS:
go test ./internal/generate/ -run TestCommitStaged_CommitMsgEmptiesMessage_IsEmptyMessageAbort -v
# Expected: PASS — errors.Is(err, generate.ErrEmptyMessage) + HEAD unchanged.
# If it passed BEFORE the fix, the test does not reproduce the bug (check the stub Out + the hook).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...    # Expect clean.
go test ./...     # Expect all PASS — incl. the existing hook tests (PreCommitFreeze, PreCommitAbort_IsRescue)
                  # which stay green (the guard doesn't affect the rescue/freeze paths).
# Confirm only the two intended files changed:
git diff --name-only | grep -vE '^internal/generate/(generate|hooks_freeze_test)\.go$' && echo "UNEXPECTED file changed" || echo "only generate.go + hooks_freeze_test.go (good)"
git diff --exit-code go.mod go.sum internal/hooks internal/decompose pkg internal/cmd internal/config PRD.md && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Git-parity reasoning (no runtime to start)

```bash
# The fix is a one-line empty-check. Verify by reasoning + the test:
#   1. A commit-msg (or prepare-commit-msg) hook empties the file → RunCommitHooks returns fm="" → the guard
#      fires → ErrEmptyMessage (bare) → exit 1, no commit (the test proves it).
#   2. The abort is BEFORE CommitTree → no dangling commit, no update-ref → HEAD + live index untouched.
#   3. The sentinel + bare propagation match the existing --edit path (EditMessage) + git's "Aborting commit
#      due to empty commit message." — exit 1, NOT exit 3 rescue.
# (No Level-4 commands beyond Levels 1–3 — the test IS the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the two edited files.
- [ ] `go test ./...` GREEN; the new test FAILS pre-fix / PASSES post-fix.
- [ ] go.mod/go.sum byte-unchanged; only `internal/generate/generate.go` + `hooks_freeze_test.go` modified.

### Feature Validation
- [ ] `CommitStaged` returns `Result{}, ErrEmptyMessage` when a hook empties the message (after trimming).
- [ ] No commit created (HEAD unchanged); the abort is BEFORE CommitTree.
- [ ] The error is the BARE `ErrEmptyMessage` (exit 1, NOT `*RescueError`/exit 3) — matches EditMessage + git.
- [ ] The existing hook tests (PreCommitFreeze, PreCommitAbort_IsRescue) stay green.

### Code Quality Validation
- [ ] Mirrors the existing `--edit` empty-message guard (same sentinel, same bare propagation).
- [ ] Test mirrors `TestCommitStaged_PreCommitAbort_IsRescue` (same deps/hook-install idiom).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (runPipeline/publishCommit are S2/S3).

### Documentation
- [ ] Inline comment cites §9.25 git parity (Issue 4). No docs/*.md edits (matches git's existing behavior +
      the existing --edit path; the changeset doc sync is P1.M5).

---

## Anti-Patterns to Avoid

- ❌ **Don't wrap the error as `*RescueError`.** That's exit 3 (a rescue). The empty-message abort is a BARE
      `ErrEmptyMessage` → exit 1, mirroring EditMessage + git. Same sentinel, same package. (verification §2)
- ❌ **Don't invent a new sentinel.** `ErrEmptyMessage` (finalize.go:45) already exists and is the right one.
- ❌ **Don't write a test that passes before the fix.** It MUST fail on the unfixed tree (CommitStaged succeeds
      → err==nil). Use a stub that outputs a NON-empty message + a hook that empties it. (verification §4)
- ❌ **Don't fix runPipeline or publishCommit here.** Those are S2 (P1.M3.T1.S2) + S3 (P1.M3.T1.S3). This task
      is CommitStaged ONLY. (verification §6)
- ❌ **Don't touch the hooks block internals / CommitTree / UpdateRefCAS / signal handling.** Only the guard
      (after the hooks block) is added.
- ❌ **Don't add imports/deps.** `strings` is already imported; `ErrEmptyMessage` is same-package.

---

## Confidence Score

**10/10** — a contract-specified, line-accurate one-guard bugfix that mirrors an existing in-repo pattern
(EditMessage's `ErrEmptyMessage` abort) and an existing in-repo test (`TestCommitStaged_PreCommitAbort_IsRescue`).
The gap is verified (no empty check between the hooks block and CommitTree), the sentinel is verified
(`ErrEmptyMessage`, bare → exit 1), the placement is unambiguous (after the hooks block, before CommitTree),
and the TDD test is engineered to fail-before/pass-after. Zero file overlap with the parallel P1.M2.T1.S1
(config/docs/hooks/runner.go). No residual risk: the fix is a one-line `strings.TrimSpace` check returning a
same-package sentinel, and the test reproduces the exact bug (commit-msg empties file → empty commit created)
then proves the abort.
