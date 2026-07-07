---
name: "P1.M3.T1.S2 — Add empty-message guard in pkg/stagecoach.runPipeline after RunCommitHooks (Issue 4: a hook that empties the message file must abort, not carry an empty message — the dry-run/SystemExtra path)"
description: |

  Bugfix for Issue 4 (Bug-Fix PRD §h2.2/h3.3; stagecoach PRD §9.25 FR-V2 git parity + §9.8/§13.2 atomic-commit
  core). After `RunCommitHooks` returns, `pkg/stagecoach.runPipeline` reassigns `msg = fm` (the hook-adjusted
  message) and — under dryRun — returns it in `Result{Message: msg}`, or under the commit path passes it
  straight to `CommitTree`, with NO empty-message check (stagecoach.go:673 → :678 dryRun return / :694
  CommitTree). A `prepare-commit-msg` or `commit-msg` hook that empties the message file (a common rejection /
  force-re-edit pattern) therefore produces a dry-run preview with an EMPTY message (exit 0) OR a commit with
  an empty message (invalid git state that `git commit` refuses). This task adds the missing guard — the SAME
  pattern S1 (DONE, generate.go:436-439) used for `CommitStaged`: after the hooks block, if the finalized
  message is empty (after trimming), abort with the BARE `generate.ErrEmptyMessage` (exit 1, NOT exit 0
  warn-and-print, NOT exit 3 rescue).

  S1 (P1.M3.T1.S1) shipped the identical guard in `generate.CommitStaged`. S2 is the SECOND of three call
  sites named in Issue 4: CommitStaged (S1, done) → **runPipeline (S2, this task)** → publishCommit (S3,
  P1.M3.T1.S3, planned). Each is an independent one-guard fix.

  THE FIX — one guard inserted in `runPipeline`, AFTER the `if deps.Hooks != nil { ... }` block (after
  `treeSHA, msg = ft, fm` at L673, after the block's closing `}` at L674), BEFORE the `if dryRun` early return
  (L678):
    ```go
    // §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file.
    // git aborts "Aborting commit due to empty commit message."; mirror it — return the BARE
    // generate.ErrEmptyMessage (exit 1, NOT a rescue). This is NOT the dryRun warn-and-print path
    // (ErrEmptyMessage is not a *RescueError, and this guard sits AFTER the hooks block) — an empty message
    // aborts even under --dry-run. HEAD + live index are untouched (the abort returns before CommitTree).
    if strings.TrimSpace(msg) == "" {
        return Result{}, generate.ErrEmptyMessage
    }
    ```
  `generate.ErrEmptyMessage` (finalize.go:45) is EXPORTED; pkg/stagecoach already imports `generate` + `strings`
  ⇒ NO new import. The abort is BARE (not `*RescueError`) → exit 1.

  ⚠️ **#1 — ErrEmptyMessage is NOT a *RescueError ⇒ it does NOT enter the dryRun warn-and-print (THE key
       point).** runPipeline's dryRun warn-and-print (FR-V8a, L655-668) handles ONLY `*generate.RescueError`
       (`errors.As(herr, &re)`) and sits INSIDE the hooks block's `if herr != nil` branch. The empty-message
       guard is AFTER the hooks block and returns a BARE error — it never reaches the warn-and-print, and
       wouldn't match `errors.As(&re)` even if it did. ⇒ Under BOTH dryRun and commit paths, an empty message →
       `ErrEmptyMessage` → exit 1 (NOT exit 0, NOT exit 3). (research §2)

  ⚠️ **#2 — TDD: write the FAILING test first; the dryRun test is load-bearing.** Before the guard, a dryRun
       whose hook empties the file returns `Result{Message: ""}, nil` (exit 0 — the bug). The dryRun test
       proves the abort is NOT swallowed by FR-V8a. Write it FIRST (it FAILS on the unfixed tree), then add the
       guard (it PASSES). (research §4)

  ⚠️ **#3 — Placement: AFTER the hooks block, BEFORE `if dryRun`.** Between L674 (the `if deps.Hooks != nil`
       block's closing `}`) and L677 (the dry-run comment / `if dryRun`). This guards the FINAL msg
       unconditionally — covering both the hook-exited-0 case (`else` set `msg = fm = ""`) and the
       hook-exited-non-zero under dryRun case (warn-and-print set `msg = re.Candidate`, possibly ""). Same
       placement S1 used in CommitStaged. (research §3)

  ⚠️ **#4 — runPipeline ONLY (scope).** Do NOT touch `generate.CommitStaged` (S1, done) or
       `decompose.publishCommit` (S3). S2 is `pkg/stagecoach/stagecoach.go` (+ its test) ONLY. (research §0/§6)

  ⚠️ **#5 — No new imports in stagecoach.go; go.mod UNCHANGED.** `strings` (L15) + `generate` (L21) already
       imported. stagecoach_test.go may need `filepath` + `errors` (add if missing). No new external dep.

  Deliverable: MODIFIED `pkg/stagecoach/stagecoach.go` (the one-line guard after the hooks block) + MODIFIED
  `pkg/stagecoach/stagecoach_test.go` (NEW dryRun + commit-path regression tests). NO other file. NO go.mod
  change. OUTPUT: runPipeline returns `Result{}, generate.ErrEmptyMessage` when a hook empties the message,
  under both dry-run and commit paths; no commit created; the dry-run preview correctly shows the abort
  instead of carrying an empty message. DOCS: none — matches git's existing 'Aborting commit due to empty
  commit message.' + the existing --edit path + S1's CommitStaged guard.

---

## Goal

**Feature Goal**: Close the Issue-4 git-parity gap on the dry-run/SystemExtra path: a `prepare-commit-msg` or
`commit-msg` hook that empties the message file must abort (exit 1, no commit / no empty dry-run preview) —
not carry an empty message. Add the missing empty-message guard in `pkg/stagecoach.runPipeline` after
`RunCommitHooks` returns, mirroring S1's `CommitStaged` guard and `git commit`'s "Aborting commit due to empty
commit message."

**Deliverable** (MODIFY existing files only):
1. `pkg/stagecoach/stagecoach.go` — insert `if strings.TrimSpace(msg) == "" { return Result{},
   generate.ErrEmptyMessage }` after the hooks block (L674), before the `if dryRun` early return (L678).
2. `pkg/stagecoach/stagecoach_test.go` — add `TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1` (the
   load-bearing dryRun test) + `TestGenerateCommit_HookEmptiesMessage_NoCommit` (the commit-path test).

**Success Definition**: the dryRun test FAILS on the unfixed tree (runPipeline returns `Result{Message:""}, nil`
— exit 0) and PASSES after the guard (`errors.Is(err, generate.ErrEmptyMessage)`); the commit-path test asserts
`ErrEmptyMessage` + HEAD unchanged (no commit); the existing runPipeline/DryRun tests stay green;
`go build/vet/test ./...` green; only the two files changed; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: A user running `stagecoach --dry-run` (or with `--context`/SystemExtra) whose `commit-msg` or
`prepare-commit-msg` hook empties the message file to reject a commit (a lint that rejects, a "force re-edit"
hook, or a buggy hook that truncates). Transitively: git parity (the dry-run path must behave like `git commit`).

**Use Case**: A `commit-msg` hook runs `> "$1"; exit 0` to reject the message. Under the bug, `stagecoach
--dry-run` prints a dry-run preview with an EMPTY message (exit 0) — misleading the user into thinking the
commit would land. After the fix, the dry-run aborts with "empty commit message — aborted" (exit 1) — exactly
what `git commit` does, and what the user needs to see to know the hook rejected the message.

**User Journey**: `stagecoach --dry-run` → generation produces a message → hooks run → commit-msg empties the
file → **the guard fires** → `generate.ErrEmptyMessage` → exit 1, no commit, HEAD + index untouched, the dry-run
preview shows the abort (not an empty message).

**Pain Points Addressed**: A hook that empties the message silently produces an empty dry-run preview (exit 0)
or an empty-message commit (invalid git state). The guard makes the abort explicit and git-parity on BOTH the
dry-run and commit paths.

## Why

- **Fixes a documented Major git-parity bug (Issue 4) on the second of three call sites.** The atomic-commit
  core must not land a bad commit (§9.8/§13.2); `git commit` aborts on an empty message; stagecoach diverged.
  S1 fixed CommitStaged; S2 fixes runPipeline; S3 will fix publishCommit.
- **The dry-run path is especially misleading.** Under the bug, `stagecoach --dry-run` with an emptying hook
  returns exit 0 with an empty message — the user thinks the commit is fine. The guard makes the abort visible.
- **Mirrors S1 (CommitStaged) + the existing `--edit` path.** Same sentinel (`ErrEmptyMessage`), same bare
  propagation (exit 1, NOT rescue). One guard + two tests. No config/API/flag/doc surface change.

## What

A single empty-message guard added to `runPipeline` (after the hooks block, before the dryRun return), plus two
TDD regression tests. No other behavior, signature, file, or dependency changes.

### Success Criteria
- [ ] `runPipeline` checks `strings.TrimSpace(msg) == ""` after the hooks block (before `if dryRun`) and returns
      `Result{}, generate.ErrEmptyMessage` (bare, same sentinel as S1's CommitStaged + EditMessage).
- [ ] NEW `TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1`: a commit-msg hook that empties the file +
      `DryRun:true` → `errors.Is(err, generate.ErrEmptyMessage)`. FAILS before the guard (err==nil, exit 0),
      PASSES after.
- [ ] NEW `TestGenerateCommit_HookEmptiesMessage_NoCommit`: the same hook + `SystemExtra` (forces runPipeline
      !dryRun) → `errors.Is(err, generate.ErrEmptyMessage)` + HEAD unchanged (no commit).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean.
- [ ] Only `pkg/stagecoach/stagecoach.go` + `stagecoach_test.go` changed; go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can do this from: the exact guard (below), the
`generate.ErrEmptyMessage` sentinel (exported), the runPipeline hooks-block structure (the insertion point),
the dryRun-warn-and-print subtlety (why ErrEmptyMessage isn't swallowed), and the test template
(`TestGenerateCommit_DryRun`). No hook-runner/generate-internals knowledge required — this is a one-line
empty-check + two tests.

### Documentation & References

```yaml
# MUST READ — the design calls (the guard, the dryRun-warn-and-print subtlety, the tests)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S2/research/design-decisions.md
  why: §0 (scope: runPipeline only; S1=done, S3=later), §1 (the guard), §2 (ErrEmptyMessage is NOT *RescueError
       ⇒ exit 1 under both paths — NOT warn-and-print exit 0), §3 (placement after the hooks block), §4 (TDD:
       the dryRun test is load-bearing), §5 (no new imports), §6 (no conflict with S1/S3).
  critical: §2 (the dryRun warn-and-print handles ONLY *RescueError; the bare ErrEmptyMessage never enters it ⇒
       exit 1 even under dryRun) is the thing most likely to be misunderstood.

# MUST READ — the S1 CONTRACT (the SAME-pattern precedent, already shipped)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S1/PRP.md
  why: S1 added the identical guard to `generate.CommitStaged` (generate.go:436-439: `if strings.TrimSpace(msg)
       == "" { return Result{}, ErrEmptyMessage }`). S2 mirrors it in runPipeline — same sentinel, same bare
       propagation (exit 1), same after-hooks-block placement.
  critical: S1 is DONE (git status confirms generate.go modified). S2 does NOT touch generate.go. The two guards
       are independent (one per call site).

# MUST READ — the caller analysis (the runPipeline hooks block + the gap)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/docs/architecture/hooks_runner_and_callers.md
  section: "### Caller (b): pkg/stagecoach.runPipeline — stagecoach.go:652-694" — the hooks block + "Empty-message
           check: NONE. Under dryRun, empty msg flows to Result.Message." + the ErrEmptyMessage sentinel reference.
  why: confirms the gap (no empty check) + the exact structure S2 edits.
  critical: the doc confirms `hooks` already imports `generate` (so pkg/stagecoach — which imports generate —
       can reference generate.ErrEmptyMessage).

# The bug spec (in your context as selected_prd_content)
- file: plan/010_…/bugfix/001_d93268e01058/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 4" (h3.3) — the exact reproduction (commit-msg `> "$1"`; git aborts exit 1; stagecoach creates
           an empty-message commit / empty dry-run preview) + the suggested fix (guard after RunCommitHooks,
           return ErrEmptyMessage).
  critical: the fix returns the BARE ErrEmptyMessage (exit 1), mirroring EditMessage + S1 — NOT a rescue.

# THE FILE BEING FIXED — READ the hooks block + dryRun return + CommitTree before editing
- file: pkg/stagecoach/stagecoach.go
  section: runPipeline (L416) — the hooks block (L651-674: `if deps.Hooks != nil { ft, fm, herr := RunCommitHooks
           (..., dryRun, ...); if herr != nil { if dryRun { ...warn-and-print... } else { return herr } } else
           { treeSHA, msg = ft, fm } }`); the dryRun early return (L678: `if dryRun { ...; return Result{Message:
           msg}, nil }`); CommitTree (L694). INSERT the guard between L674 (the block's `}`) and L677 (the
           dry-run comment).
  why: the EXACT insertion point + the dryRun-warn-and-print structure (the guard must sit AFTER it).
  critical: do NOT touch the hooks block internals, the warn-and-print logic, CommitTree, UpdateRefCAS, or the
       dryRun return. Only the guard (between the hooks block and `if dryRun`) is added.

# The sentinel (exported — consumed via the generate import)
- file: internal/generate/finalize.go
  section: `var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")` (L45, EXPORTED);
           `EditMessage`'s `if edited == "" { return "", ErrEmptyMessage }` (L117-118); CommitStaged propagates
           it bare (generate.go:413, +S1's guard at :436-439).
  why: the sentinel to return (pkg/stagecoach imports generate ⇒ reference as generate.ErrEmptyMessage).
  critical: it is a BARE error (not *RescueError) → exitcode.For() → exit 1 (NOT exit 3); NOT swallowed by the
       dryRun warn-and-print (which matches only *RescueError).

# The test file + the template to mirror
- file: pkg/stagecoach/stagecoach_test.go
  section: `TestGenerateCommit_DryRun` (L236) — the EXACT template: `setupTestRepo(t, stubtest.Options{Out:...})`
           → `repoDir, _ := os.Getwd()` → `writeFile`/`stageFile` → `headSHA` → `GenerateCommit(ctx, Options{
           Provider:"stub", DryRun:true})`. Helpers `setupTestRepo`/`writeFile`/`stageFile`/`headSHA` (in-file).
  why: the test idiom — `package stagecoach` (white-box; runPipeline is unexported but GenerateCommit exercises
       it); `buildDeps` wires `deps.Hooks=hooks.DefaultRunner{}` (L387) ⇒ hooks run.
  critical: the hook is a REAL shell script in `<repoDir>/.git/hooks/commit-msg` (chmod 0755); the stub Out
       must be NON-empty (so generation succeeds and the HOOK empties it). Ensure `filepath` + `errors` imported.

# The parallel PRP — the no-conflict confirmation
- file: plan/010_…/bugfix/001_d93268e01058/P1M3T1S1/PRP.md
  why: confirms S1 is generate.go + hooks_freeze_test.go (DONE) — NOT pkg/stagecoach. Zero file overlap ⇒ S2 is
       independent.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/
  stagecoach.go        # runPipeline (L416; hooks block L651-674; dryRun return L678; CommitTree L694) — EDIT (+guard)
  stagecoach_test.go   # EDIT (+TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1 + _HookEmptiesMessage_NoCommit)
internal/generate/
  generate.go         # S1's CommitStaged guard (L436-439) — UNCHANGED (the precedent; S2 mirrors it)
  finalize.go         # ErrEmptyMessage (L45) — UNCHANGED (the sentinel, consumed as generate.ErrEmptyMessage)
go.mod / go.sum       # UNCHANGED (strings + generate already imported; no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits: pkg/stagecoach/stagecoach.go (the guard) + stagecoach_test.go (the tests).
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (#1 — ErrEmptyMessage is BARE → exit 1, NOT rescue, NOT dryRun warn-and-print): return Result{},
//   generate.ErrEmptyMessage. The dryRun warn-and-print (FR-V8a) handles ONLY *generate.RescueError; it sits
//   INSIDE the hooks block. The guard is AFTER the hooks block and returns a bare error ⇒ exit 1 under BOTH
//   dryRun and commit paths. (research §2)

// CRITICAL (#2 — TDD: the dryRun test FAILS before the guard): before the fix, a dryRun whose hook empties the
//   file returns Result{Message:""}, nil (exit 0 — the bug). Run the dryRun test on the UNFIXED tree first → it
//   MUST FAIL (err==nil). Then add the guard → it PASSES (errors.Is(err, generate.ErrEmptyMessage)). (research §4)

// CRITICAL (#3 — placement: AFTER the hooks block, BEFORE `if dryRun`): between L674 (the `if deps.Hooks != nil`
//   block's `}`) and L677 (the dry-run comment). NOT inside the hooks block; NOT inside the warn-and-print. (research §3)

// GOTCHA (runPipeline ONLY — scope): the same gap exists in CommitStaged (S1, DONE) and publishCommit (S3,
//   P1.M3.T1.S3). Fix ONLY runPipeline here; do NOT touch internal/generate or internal/decompose.
// GOTCHA (stub must output NON-empty): setupTestRepo's stub Out must be a real message (e.g. "feat: change") so
//   generation succeeds and the HOOK (not the generator) empties it.
// GOTCHA (SystemExtra forces runPipeline !dryRun): the commit-path test uses Options{SystemExtra:"..."} (no
//   DryRun) — `!DryRun && SystemExtra==""` delegates to CommitStaged, so SystemExtra is needed to exercise
//   runPipeline's commit tail.
// GOTCHA (no new imports in stagecoach.go): strings (L15) + generate (L21) already imported. stagecoach_test.go
//   may need `filepath` (for .git/hooks path) + `errors` (for errors.Is) — add if missing.
// GOTCHA (buildDeps wires Hooks): GenerateCommit → buildDeps → deps.Hooks=hooks.DefaultRunner{} (L387) ⇒ the
//   `if deps.Hooks != nil` branch is live in runPipeline; a real hook in .git/hooks/ IS exec'd.
```

## Implementation Blueprint

### Data models and structure

No new types. The guard + the two tests:

```go
// pkg/stagecoach/stagecoach.go — runPipeline: INSERT the guard after the hooks block, before `if dryRun`.
//
// (current, for orientation — the hooks block + what follows:)
//	if deps.Hooks != nil {
//		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)
//		if herr != nil {
//			if dryRun {
//				var re *generate.RescueError
//				if errors.As(herr, &re) { ...warn-and-print; msg = wouldBe... } else { return Result{}, herr }
//			} else {
//				return Result{}, herr // !dryRun → rescue
//			}
//		} else {
//			treeSHA, msg = ft, fm // hook accepted (possibly re-treed + prepare-annotated)
//		}
//	}
//  ↓↓↓ INSERT THIS GUARD ↓↓↓
	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file
	// (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit message.";
	// mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit path
	// and S1's CommitStaged guard. This is NOT the dryRun warn-and-print path (ErrEmptyMessage is not a
	// *RescueError, and this guard sits AFTER the hooks block) — an empty message aborts even under --dry-run.
	// HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(msg) == "" {
		return Result{}, generate.ErrEmptyMessage
	}
//  ↑↑↑ END INSERT ↑↑↑
//	// ---- Dry-run success: skip commit-tree/update-ref. ----
//	if dryRun { ...; return Result{Message: msg, ...}, nil }
```

```go
// pkg/stagecoach/stagecoach_test.go (package stagecoach) — ADD the two TDD regression tests.

// TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1 is the Issue-4 dry-run guard: a commit-msg hook that
// empties the message file must NOT produce an empty dry-run preview (exit 0). git aborts; stagecoach returns
// the BARE generate.ErrEmptyMessage (exit 1, NOT the FR-V8a warn-and-print — ErrEmptyMessage is not a
// *RescueError, and the emptying hook exits 0 ⇒ no RescueError). FAILS before the guard (err==nil); PASSES after.
func TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: non-empty generated message"}) // non-empty ⇒ generation succeeds
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	// A commit-msg hook that empties the message file (exit 0 ⇒ not a hook failure; the guard catches the EMPTY result).
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "commit-msg"), []byte("#!/bin/sh\n> \"$1\"\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write commit-msg hook: %v", err)
	}

	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a dry-run with an empty message was produced — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1 — NOT the dryRun warn-and-print), got %T: %v", err, err)
	}
}

// TestGenerateCommit_HookEmptiesMessage_NoCommit is the Issue-4 commit-path guard: the same emptying hook on the
// runPipeline commit tail (forced via SystemExtra, since !DryRun && SystemExtra=="" delegates to CommitStaged) →
// generate.ErrEmptyMessage + NO commit created (HEAD unchanged — the abort returned before CommitTree).
func TestGenerateCommit_HookEmptiesMessage_NoCommit(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: non-empty generated message"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooksDir, "commit-msg"), []byte("#!/bin/sh\n> \"$1\"\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write commit-msg hook: %v", err)
	}

	beforeSHA := headSHA(t, repoDir)
	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", SystemExtra: "extra context"}) // SystemExtra ⇒ runPipeline !dryRun
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1), got %T: %v", err, err)
	}
	// NO commit created (HEAD unchanged — the abort returned before CommitTree).
	afterSHA := headSHA(t, repoDir)
	if afterSHA != beforeSHA {
		t.Errorf("HEAD moved on empty-message abort: %s → %s (a commit was created)", beforeSHA, afterSHA)
	}
}
```

### Implementation Tasks (ordered by dependencies — TDD: test first, then fix)

```yaml
Task 1: ADD the FAILING tests (stagecoach_test.go) — write the regression tests FIRST
  - ADD TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1 + TestGenerateCommit_HookEmptiesMessage_NoCommit
    per the Blueprint (setupTestRepo + a commit-msg `> "$1"; exit 0` hook + DryRun:true / SystemExtra).
  - ENSURE stagecoach_test.go imports `filepath` + `errors` (add if missing).
  - RUN on the UNFIXED tree: `go test ./pkg/stagecoach/ -run TestGenerateCommit_DryRun_HookEmptiesMessage -v`
      → it MUST FAIL (err==nil → t.Fatal). This is the TDD proof the test reproduces Issue 4. (If it passes
      before the fix, the stub Out is empty or the hook didn't run — check buildDeps wired deps.Hooks.)
  - GOTCHA: the stub Out must be NON-empty (the HOOK empties it, not the generator).

Task 2: ADD the guard (stagecoach.go) — the fix
  - INSERT `if strings.TrimSpace(msg) == "" { return Result{}, generate.ErrEmptyMessage }` (with the §9.25
      git-parity comment) AFTER the `if deps.Hooks != nil { ... }` block's closing `}` (L674), BEFORE the
      `if dryRun` early return (L678).
  - USE generate.ErrEmptyMessage (pkg/stagecoach imports generate); strings is already imported.
  - DO NOT touch the hooks block internals, the warn-and-print logic, CommitTree, UpdateRefCAS, or the dryRun return.

Task 3: VERIFY (TDD green + no regression)
  - RUN Task 1's tests again → now PASS (err == generate.ErrEmptyMessage; HEAD unchanged on the commit path).
  - RUN the full suite: `go build ./... && go vet ./... && go test ./...` → GREEN.
  - CONFIRM the existing runPipeline/DryRun tests (TestGenerateCommit_DryRun, _DedupeRetry, _ParseRetry,
      _Snapshot) stay green; go.mod/go.sum byte-unchanged; only stagecoach.go + stagecoach_test.go modified.
```

### Implementation Patterns & Key Details

```go
// THE guard (bare generate.ErrEmptyMessage → exit 1, mirroring S1's CommitStaged + EditMessage + git):
if strings.TrimSpace(msg) == "" {
    return Result{}, generate.ErrEmptyMessage
}
// THE test assertion (bare error, NOT *RescueError):
if !errors.Is(err, generate.ErrEmptyMessage) { t.Errorf(...) }   // NOT errors.As(&re)
// THE commit-path HEAD-unchanged assertion (no commit created — the abort returned before CommitTree):
if afterSHA := headSHA(t, repoDir); afterSHA != beforeSHA { t.Errorf("HEAD moved ...") }
// THE dryRun test is load-bearing: it proves the abort is NOT swallowed by FR-V8a's warn-and-print (exit 0).
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. The guard uses already-imported `strings` + `generate` (via the existing
      import). `go mod tidy` is a no-op.

PACKAGE EDGES: NONE. The guard is intra-package (stagecoach); no new import. stagecoach_test.go may add stdlib
      `filepath`/`errors` (both already standard test imports).

UPSTREAM (the inputs — consume, do NOT edit):
  - RunCommitHooks (via deps.Hooks, wired by buildDeps L387) — returns the hook-adjusted (fm) message.
  - generate.ErrEmptyMessage (finalize.go:45) — the exported sentinel.

DOWNSTREAM: the caller (CLI / integrator) propagates the bare generate.ErrEmptyMessage → exitcode.For() → exit 1
      (NOT exit 0, NOT exit 3). S1's CommitStaged guard + the existing --edit path already do this; the
      runPipeline guard now matches them.

FROZEN/LEAVE (do NOT edit):
  - The hooks block internals (RunCommitHooks call, the dryRun warn-and-print), CommitTree, UpdateRefCAS, the
    dryRun return, signal handling.
  - internal/generate/* (S1's CommitStaged guard — DONE), internal/decompose/* (S3's publishCommit — later),
    internal/hooks/*, internal/cmd/*, internal/config/*.
  - PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go
go vet ./pkg/stagecoach/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go.mod/go.sum byte-unchanged (strings + generate already in scope).
```

### Level 2: The regression tests (TDD — fail before, pass after)

```bash
# BEFORE the guard (Task 1 only, Task 2 not yet applied) — the dryRun test MUST FAIL (reproduces Issue 4):
go test ./pkg/stagecoach/ -run TestGenerateCommit_DryRun_HookEmptiesMessage -v
# Expected: FAIL — "expected generate.ErrEmptyMessage, got nil (a dry-run with an empty message was produced)".

# AFTER the guard (Task 2 applied) — BOTH tests MUST PASS:
go test ./pkg/stagecoach/ -run 'TestGenerateCommit_DryRun_HookEmptiesMessage|TestGenerateCommit_HookEmptiesMessage_NoCommit' -v
# Expected: PASS — errors.Is(err, generate.ErrEmptyMessage); the commit-path test also asserts HEAD unchanged.
# If the dryRun test passed BEFORE the fix, it does not reproduce the bug (check the stub Out is non-empty + the hook ran).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...    # Expect clean.
go test ./...     # Expect all PASS — incl. the existing runPipeline/DryRun tests (TestGenerateCommit_DryRun,
                  # _DedupeRetry, _ParseRetry, _Snapshot) which stay green (the guard doesn't affect them).
# Confirm only the two intended files changed:
git diff --name-only | grep -vE '^pkg/stagecoach/(stagecoach|stagecoach_test)\.go$' && echo "UNEXPECTED file changed" || echo "only stagecoach.go + stagecoach_test.go (good)"
git diff --exit-code go.mod go.sum internal/generate internal/decompose internal/hooks internal/cmd internal/config PRD.md && echo "frozen files UNCHANGED (expected)"
# Confirm S1's generate.go guard is still in place (S2 didn't touch it):
grep -n 'TrimSpace(msg) == ""' internal/generate/generate.go && echo "(S1 guard intact)"
```

### Level 4: Git-parity reasoning (no runtime to start)

```bash
# The fix is a one-line empty-check. Verify by reasoning + the tests:
#   1. A commit-msg (or prepare-commit-msg) hook empties the file (exit 0) → RunCommitHooks returns fm="" →
#      the hooks block's `else` sets msg="" → the guard fires → generate.ErrEmptyMessage (bare) → exit 1. (the dryRun test)
#   2. Under dryRun, the abort is NOT swallowed by FR-V8a's warn-and-print: ErrEmptyMessage is not a *RescueError,
#      and the guard sits AFTER the hooks block (outside the `if herr != nil` warn-and-print branch). (the dryRun test)
#   3. On the commit path (SystemExtra → runPipeline !dryRun), the abort returns before CommitTree → no commit,
#      HEAD + live index untouched. (the commit-path test)
#   4. The sentinel + bare propagation match S1's CommitStaged guard + the existing --edit path + git's
#      "Aborting commit due to empty commit message." — exit 1, NOT exit 0, NOT exit 3.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the two edited files.
- [ ] `go test ./...` GREEN; the dryRun test FAILS pre-fix / PASSES post-fix; the commit-path test PASSES.
- [ ] go.mod/go.sum byte-unchanged; only `pkg/stagecoach/stagecoach.go` + `stagecoach_test.go` modified.

### Feature Validation
- [ ] `runPipeline` returns `Result{}, generate.ErrEmptyMessage` when a hook empties the message (after trimming).
- [ ] Under dryRun: the abort is NOT swallowed by FR-V8a warn-and-print (exit 1, NOT exit 0).
- [ ] On the commit path: no commit created (HEAD unchanged); the abort is BEFORE CommitTree.
- [ ] The error is the BARE `generate.ErrEmptyMessage` (exit 1, NOT `*RescueError`/exit 3) — matches S1 + EditMessage + git.
- [ ] The existing runPipeline/DryRun tests stay green.

### Code Quality Validation
- [ ] Mirrors S1's CommitStaged guard (same sentinel, same bare propagation, same after-hooks-block placement).
- [ ] Tests mirror `TestGenerateCommit_DryRun` (same setupTestRepo/hook-install idiom).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (CommitStaged=S1, publishCommit=S3).

### Documentation
- [ ] Inline comment cites §9.25 git parity (Issue 4) + the dryRun-warn-and-print distinction. No docs/*.md edits
      (matches git's existing behavior + S1 + the existing --edit path; the changeset doc sync is P1.M5).

---

## Anti-Patterns to Avoid

- ❌ **Don't wrap the error as `*generate.RescueError`.** That's exit 3 (a rescue). The empty-message abort is a
      BARE `generate.ErrEmptyMessage` → exit 1, mirroring S1 + EditMessage + git. Same sentinel. (research §2)
- ❌ **Don't place the guard INSIDE the hooks block or the warn-and-print branch.** It goes AFTER the hooks block
      (L674), BEFORE `if dryRun` (L678). Inside the warn-and-print would conflate it with FR-V8a. (research §3)
- ❌ **Don't write a dryRun test that passes before the fix.** It MUST fail on the unfixed tree (runPipeline
      returns `Result{Message:""}, nil` → err==nil). Use a stub with NON-empty Out + a hook that empties it. (research §4)
- ❌ **Don't fix CommitStaged (S1, done) or publishCommit (S3) here.** This task is runPipeline ONLY. (research §0)
- ❌ **Don't touch the hooks block internals / warn-and-print / CommitTree / UpdateRefCAS / dryRun return.** Only
      the guard (between the hooks block and `if dryRun`) is added.
- ❌ **Don't add imports/deps to stagecoach.go.** `strings` + `generate` are already imported. (stagecoach_test.go
      may add stdlib `filepath`/`errors`.)
- ❌ **Don't forget the SystemExtra trick for the commit-path test.** `!DryRun && SystemExtra==""` delegates to
      CommitStaged (S1's path); use `SystemExtra:"..."` to force runPipeline's commit tail. (research §4)

---

## Confidence Score

**10/10** — a contract-specified, line-accurate one-guard bugfix that mirrors an ALREADY-SHIPPED in-repo pattern
(S1's `generate.CommitStaged` guard at generate.go:436-439 — same sentinel, same bare propagation, same
after-hooks-block placement) and an existing in-repo test (`TestGenerateCommit_DryRun`). The runPipeline hooks
block (L651-674) + the dryRun early return (L678) are read precisely (the insertion gap is unambiguous); the
dryRun-warn-and-print subtlety (it handles ONLY `*RescueError` inside the hooks block; the bare
`generate.ErrEmptyMessage` returned AFTER the block never enters it ⇒ exit 1 under both paths) is the one
non-obvious point and is explicitly specified. `buildDeps` wires `deps.Hooks` (L387) ⇒ the public-API test
exercises hooks; `strings` + `generate` are already imported. Zero file overlap with S1 (generate.go, DONE) or
S3 (decompose, later). The TDD dryRun test is engineered to fail-before/pass-after, reproducing the exact bug
(empty dry-run preview, exit 0) then proving the abort. No residual risk.
