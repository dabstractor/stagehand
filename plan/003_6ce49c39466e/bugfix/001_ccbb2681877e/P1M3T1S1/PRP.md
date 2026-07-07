---
name: "P1.M3.T1.S1 (bugfix Issue 3) — Add ReadTree(treePrime) index-sync to runSingleShortcut (FR-M11 planner-single) + regression test asserting clean git status"
description: |

  Bugfix for Issue 3 (Bug-Fix PRD §h3.2; stagecoach PRD §9.14 FR-M11, §13.6, §20.2, §18.1). The FR-M11
  planner-single shortcut (`runSingleShortcut`, internal/decompose/decompose.go:316) commits the frozen
  `tStart` (as `treePrime`) via `publishCommit` (CommitTree + UpdateRefCAS — touches HEAD only, NOT the
  index) and then goes straight to `buildCommitResult` with NO index sync. The T_start freeze (Decompose
  entry step 3) had reset the index to `baseTree`, so after a successful run `HEAD.tree == tStart` but
  `index == baseTree ≠ tStart` ⇒ `git status --porcelain` reports the just-committed files as staged
  deletions + untracked (`D  a.txt` / `?? a.txt` …). The sibling `runOneFileShortcut` (decompose.go:280)
  ALREADY fixed this with `deps.Git.ReadTree(ctx, tStart)` after `publishCommit` (its `CRITICAL (findings
  §4)` block, ~L294); `runSingleShortcut` is missing the identical sync. This task adds it + a regression
  test that reproduces the bug (status non-empty) then passes (status empty).

  THE FIX — one block inserted in `runSingleShortcut`, between the `publishCommit` error-return and
  `buildCommitResult` (mirrors `runOneFileShortcut`'s sync exactly):
    ```go
    // CRITICAL (findings §4): sync the index to the committed tree so `git status` is clean ...
    if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
        return DecomposeResult{}, fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)
    }
    ```
  `treePrime == tStart` (assigned once, never reassigned), so `ReadTree(treePrime) ≡ ReadTree(tStart)` —
  byte-identical to the reference fix. `ErrDecomposeFailed`, `fmt`, `ctx`, `deps.Git.ReadTree` are all
  already in scope (the sibling uses them). NO new imports, NO new types, NO signature change.

  ⚠️ **#1 — insert on the SUCCESS path ONLY, AFTER publishCommit, BEFORE buildCommitResult.** The sync must
  run only when the commit landed (on a publish error we already returned — the rescue recipe uses the tree
  SHA, not the index). Place it between the `if err != nil { return ... }` closing brace of `publishCommit`
  and the `cr, err := buildCommitResult(...)` line. Do NOT move/reorder `buildCommitResult`. (verification §2)

  ⚠️ **#2 — mirror the sibling's comment + error string (hyphenated).** The reference (`runOneFileShortcut`)
  uses `// CRITICAL (findings §4): …` + `fmt.Errorf("%w: one-file index sync: %w", ErrDecomposeFailed, err)`.
  This task uses `single-shortcut index sync` (hyphenated, matching the sibling's `one-file` form and the
  contract LOGIC — NOT Issue 3's prose "single shortcut" without hyphen). Adapt the comment to reference
  `treePrime` (== tStart). (verification §2)

  ⚠️ **#3 — the regression test MUST FAIL before the fix and PASS after (TDD).** Add
  `TestDecompose_SingleShortcut_CleanStatus` (decompose_test.go) mirroring
  `TestDecompose_OneFileShortcut_PlannerBypassed` (decompose_test.go:1507): BORN repo (`dcmCommitRaw
  "initial"`), 2+ un-staged files, planner stub returning `{"single":true,"message":"feat: add a and b"}`,
  default deps (Commits=0 auto), assert `dcmStatusPorcelain(t, repo) == ""`. Run it on the UNFIXED tree
  first → it FAILS (status non-empty: `D  a.txt`/`D  b.txt`/`?? a.txt`/`?? b.txt`); apply the fix → it PASSES.
  The existing `TestDecompose_SingleShortcut_CleanMessage` (L314) uses an UNBORN repo + no status assert, so
  it misses the bug — do NOT just copy it. (verification §3/§4)

  ⚠️ **#4 — use AUTO mode (Commits=0, default deps), NOT Commits≥2.** With 2 files the FR-M2b one-file
  short-circuit (exactly 1 file) does NOT fire, so the planner is invoked in auto mode and legitimately
  returns `single:true` → `runSingleShortcut`. Setting Commits≥2 would be WRONG: forced mode asks the
  planner to partition into N (FR-M2), conflicting with `single:true`. 2 files + default deps is the correct
  route. (verification §3)

  ⚠️ **#5 — NO conflict with the parallel work item.** P1.M2.T2.S1 (bugfix Issue 2, message-role provider)
  touches `internal/cmd/default_action.go`, `internal/config/roles.go`, `internal/generate/generate.go`,
  `internal/stubtest/stubtest.go`, `pkg/stagecoach/{stagecoach,stagecoach_test}.go` — NOT `internal/decompose/*`.
  This task touches ONLY `internal/decompose/decompose.go` + `internal/decompose/decompose_test.go`. Zero
  file overlap ⇒ independent. (verification §5)

  Deliverable: MODIFIED `internal/decompose/decompose.go` (the `ReadTree(treePrime)` block in
  `runSingleShortcut` + the mirroring `CRITICAL (findings §4)` comment); MODIFIED
  `internal/decompose/decompose_test.go` (NEW `TestDecompose_SingleShortcut_CleanStatus`). NO other file. NO
  go.mod change. OUTPUT: after a successful planner-single decompose run, `git status --porcelain` is empty
  (the §20.2 loop-index-cleanliness invariant holds); the new test reproduces the bug pre-fix and is green
  post-fix; the full suite stays green. DOCS: none — internal fix enforcing an existing documented invariant.

---

## Goal

**Feature Goal**: Restore the §20.2 loop-index-cleanliness invariant on the FR-M11 planner-single path: after
a successful `runSingleShortcut` decompose (planner returns `single:true` + a message, multi-file changeset,
born repo), the index is synced to the committed tree so `git status --porcelain` is empty — eliminating the
"my just-committed files show as deleted+untracked" regression. Add a regression test that would have caught
it (born repo + status assertion) and that fails before the fix / passes after.

**Deliverable** (MODIFY existing files only — no new files, no new deps):
1. `internal/decompose/decompose.go` — insert the `deps.Git.ReadTree(ctx, treePrime)` block (wrapped in
   `ErrDecomposeFailed`) into `runSingleShortcut`, between `publishCommit`'s error-return and
   `buildCommitResult`, with a `CRITICAL (findings §4)` comment mirroring `runOneFileShortcut`.
2. `internal/decompose/decompose_test.go` — add `TestDecompose_SingleShortcut_CleanStatus` (born repo,
   2 un-staged files, planner `single:true`, assert `dcmStatusPorcelain == ""`).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; the new test FAILS on the
unfixed tree (status non-empty) and PASSES after the fix (status empty); `runOneFileShortcut` and all other
decompose paths/shortcuts unchanged; the existing `TestDecompose_SingleShortcut_CleanMessage` still green;
no other file modified; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: The §20.2 loop-index-cleanliness invariant + every decompose user who hits the planner-single
path (auto mode, multi-file changeset, planner judges one commit). Transitively: the user who runs `stagecoach`
on a dirty tree, the planner says "this is one commit", stagecoach commits, and the user then runs `git status`.

**Use Case**: A user with a born repo and 2+ un-staged files runs `stagecoach` (auto-decompose). The planner
judges one commit and returns a message. Stagecoach commits via `runSingleShortcut`. BEFORE the fix,
`git status` shows the committed files as `D …/?? …` (looks broken). AFTER the fix, `git status` is clean.

**User Journey**: `Decompose` (auto, 2 files) → freeze T_start (index→baseTree) → planner → `single:true` →
`runSingleShortcut` → `publishCommit` (HEAD→tStart) → **`ReadTree(treePrime)` (index→tStart==HEAD.tree)** →
`buildCommitResult` → clean `git status`.

**Pain Points Addressed**: The post-commit "deleted + untracked" status that made a successful commit look
like stagecoach corrupted the repo (Issue 3). Restores user trust + the documented invariant.

## Why

- **Fixes a documented Major bug (Issue 3).** The bug-fix PRD §h3.2 names this exact regression with
  reproduction steps; it violates §20.2 (loop-index-cleanliness) and §18.1 (byte-for-byte-unchanged modulo
  dangling objects).
- **Parity with the sibling path.** `runOneFileShortcut` already syncs the index post-publish (findings §4);
  `runSingleShortcut` is the same kind of shortcut (commits the frozen T_start directly) and must do the same.
  The fix is the sibling's sync, verbatim (with `treePrime` for `tStart`).
- **Cheap, surgical, no-surface-change.** One 4-line block + one regression test. No config/API/flag/doc
  surface change (internal invariant enforcement). No new deps.

## What

A single `ReadTree(treePrime)` index-sync block added to `runSingleShortcut` (mirroring `runOneFileShortcut`),
plus one regression test (`TestDecompose_SingleShortcut_CleanStatus`) that reproduces the dirty-status bug on
a born repo and asserts it is clean after the fix. No other behavior, signature, file, or dependency changes.

### Success Criteria

- [ ] `runSingleShortcut` calls `deps.Git.ReadTree(ctx, treePrime)` after `publishCommit` succeeds and before
      `buildCommitResult`, wrapped in `fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)`,
      with a `CRITICAL (findings §4)` comment mirroring `runOneFileShortcut`.
- [ ] NEW `TestDecompose_SingleShortcut_CleanStatus`: born repo (`dcmCommitRaw "initial"`), 2+ un-staged files,
      planner stub `{"count":1,"single":true,"commits":[...],"message":"feat: add a and b"}`, message counter
      stub (assert NOT called), default deps (Commits=0 auto), asserts `dcmStatusPorcelain(t, repo) == ""`.
- [ ] The new test FAILS on the unfixed tree (status non-empty) and PASSES after the fix.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on the two files.
- [ ] `runOneFileShortcut` + every other decompose path/shortcut unchanged; the existing
      `TestDecompose_SingleShortcut_CleanMessage` still green; no other file modified; go.mod/go.sum unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact insertion point +
code (below), the reference fix in `runOneFileShortcut` (the comment + error to mirror), the reference test
`TestDecompose_OneFileShortcut_PlannerBypassed` (the structure to mirror), the helpers list, and the TDD
ordering (test fails before / passes after). No git-internals reasoning beyond "ReadTree syncs index→tree".

### Documentation & References

```yaml
# MUST READ — the live-tree verification (routing, no-conflict, helpers, TDD rationale)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M3T1S1/research/verification.md
  why: §1 (bug confirmed), §2 (exact insertion + prereqs in scope), §3 (routing: 2 files+auto → runSingleShortcut),
       §4 (existing test misses it; new test reproduces), §5 (no conflict with parallel P1.M2.T2.S1), §6 (helpers).
  critical: §3 (use Commits=0 auto, NOT ≥2) and §4 (test must fail-before/pass-after) are the things most
       likely to be implemented wrong.

# The bug spec (in your context as selected_prd_content)
- file: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 3" (h3.2) — the exact reproduction (`D  a.txt` / `?? a.txt` …) + suggested fix block.
  critical: the suggested fix block is the contract; this task uses `treePrime` (== tStart) and the
       hyphenated `single-shortcut index sync` error string (per the contract LOGIC + the sibling's form).

# The file being fixed — READ the two shortcut functions before editing
- file: internal/decompose/decompose.go
  section: runOneFileShortcut (L280-307) — the REFERENCE fix: the `CRITICAL (findings §4)` block (~L294) with
           `deps.Git.ReadTree(ctx, tStart)` + `fmt.Errorf("%w: one-file index sync: %w", ErrDecomposeFailed, err)`.
  section: runSingleShortcut (L316-341) — the TARGET: lacks the sync. `treePrime := tStart` (top of fn). Insert
           the block between publishCommit's error-return and buildCommitResult.
  why: the EXACT pattern to mirror (comment wording, error-wrap form, placement on the success path).
  critical: insert AFTER the `if err != nil { return ... }` of publishCommit, BEFORE `cr, err := buildCommitResult`.
       Do NOT touch runOneFileShortcut (it already has the fix) or any other function.

# The test file — mirror the reference test, not the buggy one
- file: internal/decompose/decompose_test.go
  section: TestDecompose_OneFileShortcut_PlannerBypassed (L1507-1553) — the REFERENCE test: born repo
           (`dcmCommitRaw "initial"`), un-staged file, default deps, asserts `dcmStatusPorcelain == ""` (L1549).
           MIRROR its structure (change to 2 files + planner single:true).
  section: TestDecompose_SingleShortcut_CleanMessage (L314-355) — the EXISTING single-shortcut test. It uses
           `dcmInitRepo` ONLY (UNBORN) + `deps.Config.Commits = 2` + NO status assert → MISSES the bug. Do NOT
           copy it; use it only for the planner-JSON + message-counter-stub idiom.
  section: helpers — dcmInitRepo (L28), dcmWriteFile (L36), dcmCommitRaw (L51), dcmStatusPorcelain (L111),
           dcmPlannerManifest (~L117), dcmDeps (L148), dcmLogCount, stubtest.Manifest (Script+Counter).
  critical: the new test MUST use `dcmCommitRaw` (BORN repo) — the bug only manifests when baseTree≠∅ relative
       to the committed tree on a born repo. Use `dcmPlannerManifest` for the single:true stub + a counter
       `stubtest.Manifest` for the message role (assert NOT called).

# The primitive being added
- file: internal/git/git.go
  section: ReadTree (L131) — `ReadTree(ctx context.Context, tree string) error`; `git read-tree <tree>`;
           REPLACES the index with the tree's contents; mutates index only (NEVER HEAD/refs).
  why: confirms the primitive + its index-only mutation semantic (exactly what's needed to sync index→treePrime).
  critical: it is a mutation (every non-zero exit is a real error) — wrap in ErrDecomposeFailed (as the sibling does).

# The parallel PRP (scope check — no conflict)
- file: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M2T2S1/PRP.md
  why: confirms P1.M2.T2.S1 (Issue 2) touches default_action.go/generate.go/stagecoach(.go/_test.go)/roles.go/
       stubtest.go — NOT internal/decompose/*. This task touches ONLY decompose.go + decompose_test.go ⇒ zero
       file overlap, independent.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go       # runSingleShortcut (L316) — EDIT (+ReadTree(treePrime) block). runOneFileShortcut (L280) UNCHANGED (already fixed).
  decompose_test.go  # + NEW TestDecompose_SingleShortcut_CleanStatus (mirror TestDecompose_OneFileShortcut_PlannerBypassed L1507)
internal/git/git.go  # ReadTree (L131) — UNCHANGED (the primitive being consumed)
go.mod / go.sum      # UNCHANGED (no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Two in-place edits: internal/decompose/decompose.go + internal/decompose/decompose_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — success-path-only insertion): place the ReadTree block AFTER publishCommit's `if err != nil
// { return }` and BEFORE `cr, err := buildCommitResult`. On a publish error we already returned (the rescue
// recipe uses the tree SHA, not the index) — the sync is success-only, exactly like runOneFileShortcut.

// CRITICAL (#2 — mirror the sibling; hyphenated error string): use `// CRITICAL (findings §4): …` (adapt
// tStart→treePrime) + `fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)`. Match
// runOneFileShortcut's "one-file index sync" hyphenation form (NOT Issue 3's prose "single shortcut").

// CRITICAL (#3 — the test must FAIL before / PASS after): write TestDecompose_SingleShortcut_CleanStatus
// FIRST, run it on the unfixed tree → it FAILS (status like "D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt"); apply
// the fix → it PASSES. This is the contract's TDD obligation + the proof the test catches the regression.

// CRITICAL (#4 — BORN repo + 2 files + AUTO mode): use dcmCommitRaw("initial") (BORN), TWO un-staged files
// (dcmWriteFile a.txt + b.txt), default deps (Commits=0 auto). 2 files ⇒ FR-M2b one-file short-circuit
// (exactly 1) does NOT fire ⇒ planner runs ⇒ single:true ⇒ runSingleShortcut. Do NOT set Commits≥2 (forced
// mode conflicts with single:true). Do NOT use dcmInitRepo alone (UNBORN — misses the bug, like CleanMessage).

// GOTCHA (treePrime == tStart): `treePrime := tStart` is assigned once at the top of runSingleShortcut and
// never reassigned, so ReadTree(treePrime) ≡ ReadTree(tStart) — byte-identical to runOneFileShortcut's fix.
// GOTCHA (message role must NOT be called): on single:true the planner's message is used verbatim (dup-check
// first). Use a counter stubtest.Manifest for the message role and assert the counter is absent/"0" (mirror
// CleanMessage L341-354). The planner stub returns the single:true JSON via dcmPlannerManifest.
// GOTCHA (no new imports): fmt + ErrDecomposeFailed + ctx + deps.Git.ReadTree are all already in scope in
// decompose.go (runOneFileShortcut uses them). `go mod tidy` is a no-op.
```

## Implementation Blueprint

### Data models and structure

No new types. The edit is a single control-flow block; the test is a new function. Both are shown in full.

```go
// internal/decompose/decompose.go — runSingleShortcut: INSERT this block between publishCommit's
// error-return and buildCommitResult (mirror runOneFileShortcut's ~L294 block).
//
// (current tail of runSingleShortcut, for orientation:)
//	newSHA, err := publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
//	if err != nil {
//		return DecomposeResult{}, err
//	}
//  ↓↓↓ INSERT THIS BLOCK ↓↓↓
	// CRITICAL (findings §4): sync the index to the committed tree so `git status` is clean. The freeze
	// reset index→baseTree; committing treePrime (== tStart) directly would otherwise leave
	// index==baseTree ≠ HEAD.tree==treePrime ⇒ `git status` shows the just-committed files as staged
	// deletions + untracked (Issue 3). ReadTree(treePrime) ⇒ index==treePrime==HEAD.tree ⇒ clean. Mirrors
	// runOneFileShortcut's sync. Success-only: on a publish error we returned above (the rescue recipe uses
	// the tree SHA, not the index).
	if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)
	}
//  ↑↑↑ END INSERT ↑↑↑
//	cr, err := buildCommitResult(ctx, deps, newSHA, msg, isUnborn)
//	... (rest unchanged)
```

```go
// internal/decompose/decompose_test.go — ADD TestDecompose_SingleShortcut_CleanStatus.
// Mirrors TestDecompose_OneFileShortcut_PlannerBypassed (L1507) but: 2 files (not 1) + planner single:true
// (so runSingleShortcut, not runOneFileShortcut) + the status assertion. MUST fail before the fix, pass after.
func TestDecompose_SingleShortcut_CleanStatus(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")          // BORN repo (preRunHEAD set; baseTree = HEAD^{tree})
	dcmWriteFile(t, repo, "a.txt", "a\n")      // 2 un-staged files ⇒ FR-M2b one-file short-circuit does NOT fire
	dcmWriteFile(t, repo, "b.txt", "b\n")      //   ⇒ planner is invoked in auto mode

	// Planner returns FR-M11 single:true + a message ⇒ routes to runSingleShortcut.
	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add a and b","description":"a.txt + b.txt"}],"message":"feat: add a and b"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message counter stub — must NOT be called (single:true uses the planner's message verbatim).
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	messageM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // default: Commits=0 (auto), Single=false

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(single-shortcut clean status): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a and b" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add a and b")
	}
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + 1)", dcmLogCount(t, repo))
	}

	// Message agent must NOT have been called (single:true → planner message verbatim).
	if data, ferr := os.ReadFile(counterFile); ferr == nil {
		if count := strings.TrimSpace(string(data)); count != "" && count != "0" {
			t.Errorf("message agent call count = %q, want 0", count)
		}
	}

	// §20.2 loop-index-cleanliness invariant: clean tree after a fully-successful run.
	// BEFORE the fix this is non-empty ("D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt"); AFTER it is "".
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean — proves ReadTree(treePrime) index-sync)", status)
	}
}
```

### Implementation Tasks (ordered by dependencies — TDD: test first, then fix)

```yaml
Task 1: ADD TestDecompose_SingleShortcut_CleanStatus (decompose_test.go) — write the regression test FIRST
  - ADD the function per the Blueprint (born repo via dcmCommitRaw; 2 un-staged files; planner single:true;
      message counter stub; default deps; assert 1 commit + subject + log==2 + message-not-called + status=="").
  - PLACE it adjacent to TestDecompose_SingleShortcut_CleanMessage (L314) for discoverability.
  - RUN it on the UNFIXED tree: `go test ./internal/decompose/ -run TestDecompose_SingleShortcut_CleanStatus`
      → it MUST FAIL with a non-empty status (e.g. "D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt"). This is the TDD
      proof the test reproduces Issue 3. (If it passes before the fix, the test is wrong — check the routing:
      2 files? born repo? planner JSON has single:true?)
  - GOTCHA: do NOT set deps.Config.Commits (auto mode). Do NOT use dcmInitRepo alone (must be BORN).

Task 2: ADD the ReadTree(treePrime) index-sync to runSingleShortcut (decompose.go) — the fix
  - INSERT the `CRITICAL (findings §4)` block (per the Blueprint) in runSingleShortcut between publishCommit's
      error-return and buildCommitResult. Use treePrime + the hyphenated "single-shortcut index sync" error.
  - DO NOT touch runOneFileShortcut (already fixed), publishCommit, buildCommitResult, or any other function.
  - NO new imports (fmt/ErrDecomposeFailed/ctx/deps.Git.ReadTree already in scope).

Task 3: VERIFY (TDD green + no regression)
  - RUN Task 1's test again → now PASSES (status == "").
  - RUN the full suite: `go build ./... && go vet ./... && go test ./...` → GREEN.
  - Confirm runOneFileShortcut + the existing TestDecompose_SingleShortcut_CleanMessage + all one-file/escape/
      loop/arbiter tests unchanged/green. go.mod/go.sum byte-unchanged. Only decompose.go + decompose_test.go
      modified.
```

### Implementation Patterns & Key Details

```go
// THE fix — mirror runOneFileShortcut's sync exactly, with treePrime (== tStart):
if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
	return DecomposeResult{}, fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)
}

// THE test route (must reach runSingleShortcut, not the escape-hatch or one-file bypass):
//   Single=false && Commits==0  ⇒ NOT runSingleEscape (L142)
//   2 files                     ⇒ NOT runOneFileShortcut (FR-M2b needs exactly 1; L179/L185)
//   planner single:true         ⇒ runSingleShortcut (L198/L199)
deps := dcmDeps(t, repo, roles) // default Commits=0 (auto), Single=false — DO NOT set Commits≥2

// THE assertion (§20.2 invariant):
if status := dcmStatusPorcelain(t, repo); status != "" {
	t.Errorf("status = %q, want empty (clean — proves ReadTree(treePrime) index-sync)", status)
}
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. No new imports; the fix uses already-in-scope symbols. `go mod tidy`
      is a no-op.

PACKAGE EDGES: NONE. The fix is intra-package (decompose); ReadTree is an existing git.Git method.

UPSTREAM (the inputs — consume, do NOT edit):
  - git.ReadTree (git.go:131) — the index-sync primitive.
  - publishCommit (message.go) — CommitTree + UpdateRefCAS (HEAD only); the sync follows it.
  - runOneFileShortcut (decompose.go:280) — the REFERENCE fix to mirror.

DOWNSTREAM: none new. The §20.2 invariant is now honored on the planner-single path (the arbiter/loop paths
      already honor it).

FROZEN/LEAVE (do NOT edit):
  - runOneFileShortcut, runSingleEscape, runLoop, callPlanner, publishCommit, buildCommitResult, Decompose routing.
  - internal/git/*, internal/generate/*, internal/config/*, pkg/stagecoach/*, internal/cmd/*.
  - All other decompose_test.go tests (especially TestDecompose_SingleShortcut_CleanMessage + the one-file/
    escape/loop/arbiter suites). PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/decompose/decompose.go internal/decompose/decompose_test.go
go vet ./internal/decompose/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go.mod/go.sum byte-unchanged. (No new imports — confirm:)
grep -n '"fmt"\|ErrDecomposeFailed' internal/decompose/decompose.go | head   # both already present
```

### Level 2: The regression test (TDD — fail before, pass after)

```bash
# BEFORE the fix (Task 1 only, Task 2 not yet applied) — the test MUST FAIL (reproduces Issue 3):
go test ./internal/decompose/ -run TestDecompose_SingleShortcut_CleanStatus -v
# Expected: FAIL with status = "D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt" (or equivalent non-empty).

# AFTER the fix (Task 2 applied) — the test MUST PASS:
go test ./internal/decompose/ -run TestDecompose_SingleShortcut_CleanStatus -v
# Expected: PASS (status == "").
# If it passed BEFORE the fix, the test does not reach runSingleShortcut — re-check the routing (§3/#4).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — incl. TestDecompose_SingleShortcut_CleanMessage (unchanged) and the
                   # one-file/escape/loop/arbiter suites.
# Confirm only the two intended files changed:
git diff --name-only | grep -vE '^internal/decompose/(decompose|decompose_test)\.go$' && echo "UNEXPECTED file changed" || echo "only decompose.go + decompose_test.go changed (good)"
git diff --exit-code go.mod go.sum internal/git internal/generate pkg internal/cmd PRD.md && echo "frozen files UNCHANGED (expected)"
# Confirm runOneFileShortcut untouched (it already had the fix):
git diff --exit-code internal/decompose/decompose.go && echo "decompose.go changed (expected)" || git diff internal/decompose/decompose.go | grep -E '^\+' | grep -i 'one-file\|runOneFileShortcut' && echo "ERROR: touched runOneFileShortcut"
```

### Level 4: Invariant-correctness reasoning (no runtime to start)

```bash
# The fix is a 1-line-behavior change (sync index→committed tree). Verify by reasoning + the test:
#   1. BEFORE: freeze resets index→baseTree; publishCommit moves HEAD→treePrime(==tStart); index stays
#      baseTree ⇒ index≠HEAD.tree ⇒ git status dirty (the test reproduces this).
#   2. AFTER: ReadTree(treePrime) sets index→treePrime==HEAD.tree ⇒ git status clean (the test proves it).
#   3. The commit itself is unchanged (publishCommit is untouched); only the post-publish index state differs.
#   4. §20.2 (loop-index-cleanliness) + §18.1 (byte-for-byte-unchanged modulo dangling objects) now hold on
#      the planner-single path, matching runOneFileShortcut/runSingleEscape/runLoop.
# (No Level-4 commands beyond Levels 1–3 — the test IS the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the two edited files.
- [ ] `go test ./...` GREEN; the new test FAILS pre-fix / PASSES post-fix.
- [ ] go.mod/go.sum byte-unchanged; only `internal/decompose/decompose.go` + `decompose_test.go` modified.

### Feature Validation
- [ ] `runSingleShortcut` calls `deps.Git.ReadTree(ctx, treePrime)` after `publishCommit` (success-only),
      wrapped in `ErrDecomposeFailed`, before `buildCommitResult`, with the mirroring `CRITICAL (findings §4)` comment.
- [ ] `TestDecompose_SingleShortcut_CleanStatus` asserts `dcmStatusPorcelain == ""` on a born repo with 2 files
      + planner `single:true`; it fails before the fix and passes after.
- [ ] `runOneFileShortcut` + all other decompose paths unchanged; `TestDecompose_SingleShortcut_CleanMessage` green.

### Code Quality Validation
- [ ] Mirrors the existing `runOneFileShortcut` sync (comment form, error-wrap, success-only placement).
- [ ] Test mirrors `TestDecompose_OneFileShortcut_PlannerBypassed` (born repo, status assertion) — not the
      buggy unborn `CleanMessage` test.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn.

### Documentation
- [ ] The `CRITICAL (findings §4)` comment documents WHY the sync is needed (Issue 3 / §20.2). No docs/*.md
      edits (internal invariant enforcement; changeset doc sync is a separate task).

---

## Anti-Patterns to Avoid

- ❌ **Don't insert the sync on the error path or before publishCommit.** It is success-only — place it AFTER
      publishCommit's error-return and BEFORE buildCommitResult (mirror runOneFileShortcut). On a publish error
      we already returned (the rescue uses the tree SHA, not the index).
- ❌ **Don't use a non-hyphenated error string or copy Issue 3's prose wording.** Use `single-shortcut index
      sync` (hyphenated, matching the sibling's `one-file index sync` + the contract LOGIC).
- ❌ **Don't write the test to pass before the fix.** It MUST reproduce the bug (non-empty status pre-fix).
      Use a BORN repo (`dcmCommitRaw`) + 2 files + auto mode. If it passes pre-fix, the routing is wrong (likely
      Commits≥2, or only 1 file → FR-M2b bypass, or unborn repo). (verification §3/§4)
- ❌ **Don't set `deps.Config.Commits ≥ 2`.** Forced mode asks the planner to partition into N (FR-M2),
      conflicting with `single:true`. Use default deps (Commits=0 auto). (verification §3)
- ❌ **Don't use an unborn repo for the new test.** The bug (index==baseTree ≠ HEAD.tree) must be exercised on
      a BORN repo; `dcmInitRepo` alone (unborn) misses it — that's why `CleanMessage` missed it. Use `dcmCommitRaw`.
- ❌ **Don't touch runOneFileShortcut.** It ALREADY has the fix (findings §4). Only runSingleShortcut is edited.
- ❌ **Don't copy `TestDecompose_SingleShortcut_CleanMessage` blindly.** It uses an unborn repo + no status
      assert. Mirror `TestDecompose_OneFileShortcut_PlannerBypassed` (born repo + `dcmStatusPorcelain == ""`).
- ❌ **Don't add new imports/deps/files.** fmt + ErrDecomposeFailed + ctx + ReadTree are in scope; `go mod tidy`
      is a no-op.

---

## Confidence Score

**10/10** — A surgical, fully-specified one-block bugfix that mirrors an existing, in-repo fix
(`runOneFileShortcut`'s `CRITICAL (findings §4)` ReadTree sync), with every prerequisite verified in-scope on
the live tree (`treePrime==tStart`; `ReadTree` index-only mutation; `fmt`/`ErrDecomposeFailed`/`ctx` present;
routing confirmed: 2 files + auto → planner → single:true → runSingleShortcut). The regression test mirrors
an existing passing test (`TestDecompose_OneFileShortcut_PlannerBypassed`) and is engineered to fail-before/
pass-after (TDD proof). Zero file overlap with the parallel P1.M2.T2.S1. The only residual risk — the test
accidentally not reaching `runSingleShortcut` — is pre-empted by the explicit routing guidance (2 files, born
repo, Commits=0 auto) and the "test must fail before the fix" gate.
