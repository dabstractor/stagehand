# PRP — P1.M3.T1.S1: Clean the `git.WriteTree` unmerged-conflict error message (Issue 3)

> **Scope discipline.** This subtask is a **message-only** fix for bugfix-002 Issue 3. When
> `git write-tree` fails because the index has unresolved merge conflicts, `git.WriteTree`
> (`internal/git/git.go`) currently appends git's **raw multi-line stderr** to the error — so the user
> sees `f.txt: unmerged (...)` ×N + `fatal: git-write-tree: error building trees` instead of PRD §13.5's
> clean "resolve merge conflicts first" guidance. The behavior is already **correct** (exit 1,
> pre-generation, HEAD/index untouched) — only the **message** is noisy.
>
> S1 replaces the noisy `%s`-stderr message with a **single clean line** on the conflict path, while
> preserving the detailed diagnostic for a genuine non-conflict write-tree failure. Exit-code semantics,
> the call sites, and all other failure modes are unchanged. S1 also makes two surgical test additions
> and runs a verify-and-skip docs gate. **It does NOT** add a pre-WriteTree conflict pre-check elsewhere
> (generate.go / stagehand.go / default_action.go), refactor `run()`, or change any exit code.

---

## Goal

**Feature Goal**: `git.WriteTree` returns a single clean error line — `"unresolved merge conflicts in
the index — resolve them first, then re-run stagehand"` — when `git write-tree` fails due to unmerged
index entries (exit 128), instead of dumping git's raw multi-line stderr. PRD §13.5's "resolve merge
conflicts first" guidance is honored, and the message still contains the substring
`unresolved merge conflicts` so the existing test assertion holds.

**Deliverable**:
1. One source edit in `internal/git/git.go::WriteTree` (the `if code != 0` block, lines 224-226): on
   failure, probe `git ls-files -u`; if unmerged entries are present, return the clean single-line error;
   otherwise return a detailed (stderr-inclusive) error for the rare non-conflict write-tree failure.
2. Two surgical test additions to `internal/git/writetree_test.go::TestWriteTree_MergeConflict`: KEEP the
   existing `unresolved merge conflicts` contains-assertion; ADD assertions that the message does NOT
   contain the raw noise (`fatal: git-write-tree`, `error building trees`). Reuse `makeMergeConflict`
   as-is.
3. A docs **verification** (Mode A): confirm via `grep` that no doc under `docs/` or `README.md` quotes
   or describes the merge-conflict wording; correct one **only if** such wording is found. (Verified at
   planning time: none exists → no doc edit is required for S1.)

**Success Definition**:
- An unresolved-merge-conflict run prints exactly one clean line (`stagehand: unresolved merge conflicts
  in the index — resolve them first, then re-run stagehand`) and exits **1**, with the model never
  invoked and HEAD/index untouched — identical behavior, cleaner message.
- A non-conflict write-tree failure (rare) still surfaces the detailed diagnostic (exit code + stderr).
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and the **entire existing** `go test -race ./...`
  suite stays green, with the two new test assertions passing.

---

## Why

- **PRD §13.5**: *"Unresolved merge conflicts in the index: `write-tree` fails. Stagehand aborts before
  any generation with 'resolve merge conflicts first.'"* and the **§18.2 failure table** (merge
  conflicts → exit **1**). The behavior is correct; the message is not.
- **Root cause** (verified: `architecture/issue_analysis.md` ISSUE 3): `WriteTree` on `code != 0` returns
  `fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code,
  strings.TrimSpace(stderr))`. The `%s` appends git's **full** trimmed stderr, which for unmerged paths
  is multi-line noise (`<file>: unmerged (...)` ×N + `fatal: git-write-tree: error building trees`).
- **User impact**: a user hitting a routine merge conflict sees a wall of git plumbing text instead of
  the friendly guidance, and the cosmetic `↳ Generating with <provider>…` label prints *before* the
  failure (write-tree is step 3, pre-generation). After the fix, the failure is still pre-generation /
  exit 1, but the message is the single PRD-prescribed line.
- **Fix value**: localized to one method (`WriteTree`); both call sites
  (`generate.go:156` / `stagehand.go:260`) propagate the error as-is, and `handleGenError`'s default
  branch already maps a plain error to exit 1 — so the cleaner message requires **no** caller changes.

---

## What

In `internal/git/git.go`, function `WriteTree` (definition at line 219), **replace the `if code != 0`
block (lines 224-226)**. Today:

```go
	if code != 0 {
		return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
	}
```

becomes (preferred variant — detects the conflict via `ls-files -u` and preserves detail otherwise):

```go
	if code != 0 {
		// PRD §13.5: when write-tree fails on an unmerged index, return a single clean line instead of
		// dumping git's raw multi-line stderr. Probe `git ls-files -u` (lists unmerged stage entries);
		// non-empty stdout ⇒ unresolved conflicts. Failure path only (not hot); on any ls-files error
		// fall through to the detailed diagnostic so a genuine non-conflict failure isn't hidden.
		if lsOut, _, _, lsErr := g.run(ctx, g.workDir, "ls-files", "-u"); lsErr == nil && strings.TrimSpace(lsOut) != "" {
			return "", errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagehand")
		}
		return "", fmt.Errorf("git write-tree failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
```

That is the **entire** source change. Notes for the implementer:
- `errors` is **already imported** in `git.go` (line 6) — `errors.New` needs no new import. `fmt`, `strings`,
  and `g.run` (the only git-exec seam, `git.go:95`) are all already in scope.
- `g.run(ctx, g.workDir, "ls-files", "-u")` mirrors how every other bound method invokes git: repo via the
  `-C` flag inside `run()`, varargs after. It returns `(stdout, stderr, exitCode, err)`; on a non-zero
  git exit `err` is nil and `stdout` carries the unmerged-entry listing.
- The clean message **must** contain the substring `unresolved merge conflicts` (existing test assertion)
  and **must not** contain `fatal: git-write-tree` / `error building trees` (new test assertions).
- The fallback detailed error (non-conflict write-tree failure) keeps the stderr for diagnosability; it
  changes the prefix from `git write-tree: unresolved merge conflicts in index` to `git write-tree failed`
  so a non-conflict failure is no longer mislabeled as a conflict.

### Minimal acceptable variant (if the preferred probe is undesirable)

The contract permits a minimal variant — drop the `%s` stderr entirely and return the clean single line
on any `code != 0` (exit-128 on a populated index is unambiguously unmerged in practice):

```go
	if code != 0 {
		return "", errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagehand")
	}
```

Both variants pass the test additions. **The preferred variant is specified as primary** (it is more
accurate and preserves the diagnostic for genuine non-conflict failures); use the minimal variant only if
a reviewer objects to the extra failure-path git call.

### Call sites are NOT changed

Both callers return the `WriteTree` error as-is (do NOT touch them):
- `internal/generate/generate.go:156` — `CommitStaged` step 3: `treeSHA, err := deps.Git.WriteTree(ctx)`.
- `pkg/stagehand/stagehand.go:260` — `runPipeline` step 3: `treeSHA, err := deps.Git.WriteTree(ctx)`.

The error flows: caller returns it → CLI `handleGenError` (`internal/cmd/default_action.go:188`)
**default branch** `return exitcode.New(exitcode.Error, err)` → exit **1**; main prints
`stagehand: <clean msg>`. A clean single-line `msg` produces exactly one line of output. No
`*RescueError`/`*CASError`/`ErrNothingToCommit` reclassification (those are distinct branches checked
earlier in `handleGenError`).

### Docs (Mode A) — verify-and-skip

The contract's doc clause is conditional: align `docs/how-it-works.md` §"Failure modes and exit codes"
**only if** it quotes/describes the merge-conflict wording. Verified at planning time that the table (line
53) has **no** merge-conflict row (rows: Agent missing / Generation failed / Generation timed out / CAS
failure / Nothing to commit / General error) and that `grep -rn -i "merge.conflict|unresolved|resolve.*
merge|unmerged" docs/ README.md` is **empty**. → **No doc edit is required for S1.**

The implementer must re-run that grep as a **gate** (Validation Level 4) and, *if and only if* a doc now
quotes the merge-conflict wording, align it to the clean single-line message. Do **not** proactively add a
merge-conflict row to the failure-modes table — that is **P1.M5.T1.S2** (final docs sweep) scope; keeping
S1 surgical avoids colliding with that future work item.

### Success Criteria

- [ ] `WriteTree` returns the clean single-line message on the unmerged-index failure (exit 128), and the
      detailed diagnostic on a non-conflict failure (preferred variant).
- [ ] The clean message contains `unresolved merge conflicts` and contains **none** of
      `fatal: git-write-tree` / `error building trees`.
- [ ] Exit code is unchanged (still **1**, via `handleGenError` default branch); the model is never
      invoked (write-tree is step 3, pre-generation); HEAD/index untouched.
- [ ] Both call sites (`generate.go:156`, `stagehand.go:260`) are byte-for-byte unchanged.
- [ ] `TestWriteTree_MergeConflict` passes with the two new noise-absence assertions added; every other
      test stays green.
- [ ] Docs grep gate passes (no doc quotes the merge-conflict wording).

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact method, exact line range, before/after block, the `run()` seam
signature, the nil-err-on-non-zero-exit invariant, the propagation path (call sites → `handleGenError`
default branch → exit 1), the test's exact assertions + fixture reuse, and the verify-and-skip docs gate
are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + fix variants + blast radius)
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: ISSUE 3 proves the %s stderr is the noise source, names WriteTree as the sole edit site, lists the
       two acceptable fix variants (preferred ls-files -u / minimal drop-%s), and confirms the message
       propagates unchanged through both call sites to exit 1.
  section: "ISSUE 3 (Minor) — Raw multi-line git write-tree error instead of clean 'resolve merge conflicts' message"

# The edit site + the git-exec seam
- file: internal/git/git.go
  why: WriteTree (def L219) — the `if code != 0` block at L224-226 is THE EDIT SITE. run() (L95) is the
       only git-exec helper; it returns (stdout, stderr, exitCode, err) with err==nil on a non-zero git
       exit, so a probe `g.run(ctx, g.workDir, "ls-files", "-u")` is trivially available.
  pattern: Every bound method calls `g.run(ctx, g.workDir, <args>...)` and checks err then code — follow
           that exact shape. errors (L6) + fmt + strings are already imported.
  gotcha: On a non-zero git exit, run() sets code and leaves err nil (gotcha G2). Do NOT treat code!=0 as
          an error from run(); it is the normal conflict signal. ls-files -u non-empty trimmed stdout ⇒
          unmerged stage entries present.

# Why a clean message still exits 1 (propagation path)
- file: internal/cmd/default_action.go
  why: handleGenError (L188) default branch: `return exitcode.New(exitcode.Error, err)` → exit 1; main
       prints `stagehand: <msg>`. A plain error (not *RescueError/*CASError/ErrNothingToCommit) hits this
       branch — so a cleaner WriteTree message needs NO caller change.
  section: "func handleGenError" — the final `return exitcode.New(exitcode.Error, err)` line.

# Call sites (do NOT change — confirm the error propagates as-is)
- file: internal/generate/generate.go
  why: CommitStaged step 3 (L156): `treeSHA, err := deps.Git.WriteTree(ctx)` → returns err unchanged.
- file: pkg/stagehand/stagehand.go
  why: runPipeline step 3 (L260): `treeSHA, err := deps.Git.WriteTree(ctx)` → returns err unchanged.

# The test to extend (fixture reused as-is)
- file: internal/git/writetree_test.go
  why: TestWriteTree_MergeConflict (L119-133) asserts strings.Contains(err.Error(), "unresolved merge
       conflicts") and sha=="". KEEP the contains-assertion (clean message retains the substring); ADD
       noise-absence assertions. makeMergeConflict (L22-68) creates the conflict — reuse as-is.
  pattern: existing assertions use strings.Contains; mirror that for the new !strings.Contains checks.
  gotcha: The clean message must NOT contain 'fatal: git-write-tree' or 'error building trees' (the raw
          git stderr phrases). Assert both are absent.

# Docs gate (Mode A) — verify, don't blindly edit
- file: docs/how-it-works.md
  why: §"Failure modes and exit codes" (L53) table has NO merge-conflict row → nothing to align.
  section: "### Failure modes and exit codes"
```

### Current Codebase tree (relevant slice)

```bash
internal/git/git.go              # WriteTree (L219) — THE EDIT SITE (if code!=0 block L224-226); run() (L95) seam
internal/git/writetree_test.go   # TestWriteTree_MergeConflict (L119-133) + makeMergeConflict (L22-68)
internal/generate/generate.go    # CommitStaged step 3 (L156) — caller, UNCHANGED
pkg/stagehand/stagehand.go       # runPipeline step 3 (L260) — caller, UNCHANGED
internal/cmd/default_action.go   # handleGenError default branch (L188) — exit 1, UNCHANGED
docs/how-it-works.md             # failure-modes table (L53) — verify-only (no merge-conflict row)
```

### Desired Codebase tree with files changed

```bash
internal/git/git.go              # MODIFIED — WriteTree: clean conflict message + detailed non-conflict fallback
internal/git/writetree_test.go   # MODIFIED — TestWriteTree_MergeConflict: +2 noise-absence assertions
# (no new files; no other source files touched; no new fixtures/tests)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: run() returns err==nil on a non-zero git exit (gotcha G2) — code carries the exit status.
//   So in WriteTree, `code != 0` (NOT `err != nil`) is the conflict signal. The `ls-files -u` probe uses
//   the same run(): check `lsErr == nil` (not cancelled/missing-git) AND `strings.TrimSpace(lsOut) != ""`.

// CRITICAL: The clean message MUST contain the substring "unresolved merge conflicts" — the existing
//   TestWriteTree_MergeConflict assertion depends on it. The contract's example message does contain it;
//   do not rephrase it away (e.g. don't shorten to just "resolve merge conflicts first").

// CRITICAL: Do NOT append the raw multi-line stderr to the CONFLICT message. The whole point of S1 is to
//   stop dumping "<file>: unmerged (...)" ×N + "fatal: git-write-tree: error building trees". The detailed
//   stderr is kept ONLY for the rare non-conflict write-tree failure (preferred variant fallback).

// GOTCHA: errors is already imported in git.go (L6) — use errors.New for the clean message; no new import.
//   g.run, fmt, strings are all in scope. Do NOT add os/exec or shell out differently.

// GOTCHA: write-tree exit 128 on a populated index is unambiguously unmerged, but a non-conflict failure
//   (corrupted index, etc.) is possible — the preferred ls-files -u probe distinguishes the two so a real
//   non-conflict failure keeps its diagnostic. The minimal variant (drop %s unconditionally) is acceptable
//   but less accurate; prefer the probe.
```

---

## Implementation Blueprint

### The exact edit (Task 1)

In `internal/git/git.go`, `WriteTree`, **replace lines 224-226** (the `if code != 0 { … }` block) with
the preferred-variant block shown in the "What" section. No other line in the function or file changes.

### The exact test edit (Task 2)

In `internal/git/writetree_test.go`, `TestWriteTree_MergeConflict` (lines ~119-133). The current body is:

```go
	g := New(repo)
	sha, err := g.WriteTree(context.Background())
	if err == nil {
		t.Fatal("WriteTree err = nil, want non-nil (unresolved merge conflicts)")
	}
	if !strings.Contains(err.Error(), "unresolved merge conflicts") {
		t.Fatalf("WriteTree err = %v, want it to contain 'unresolved merge conflicts'", err)
	}
	if sha != "" {
		t.Fatalf("WriteTree sha = %q, want empty string on conflict", sha)
	}
```

Insert, immediately AFTER the existing `unresolved merge conflicts` contains-assertion and BEFORE the
`sha != ""` check, the two noise-absence assertions:

```go
	// The conflict message must be a single clean line — no raw git stderr noise (bugfix-002 Issue 3).
	if strings.Contains(err.Error(), "fatal: git-write-tree") {
		t.Errorf("WriteTree err = %q; want it to NOT contain raw 'fatal: git-write-tree' stderr", err.Error())
	}
	if strings.Contains(err.Error(), "error building trees") {
		t.Errorf("WriteTree err = %q; want it to NOT contain raw 'error building trees' stderr", err.Error())
	}
```

Use `t.Errorf` (not `t.Fatalf`) for the new checks so both fire if both noise phrases are present. Reuse
`makeMergeConflict(t, repo)` as-is — do not modify the fixture.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go :: WriteTree, the `if code != 0` block (L224-226)
  - REPLACE: the single fmt.Errorf("%s"-stderr) line with the preferred variant: an `ls-files -u` probe
             that returns errors.New(<clean single line>) when unmerged entries exist, else a detailed
             fmt.Errorf for the non-conflict case.
  - REUSE: g.run(ctx, g.workDir, "ls-files", "-u") — the existing git-exec seam (git.go:95). Do NOT add a
           new exec/LookPath call or import.
  - MESSAGE INVARIANT: clean message contains "unresolved merge conflicts"; contains NONE of
             "fatal: git-write-tree" / "error building trees".
  - DO NOT TOUCH: the happy-path return (L227), run(), any other method, the call sites, handleGenError.
  - DEPENDENCIES: none new (errors/fmt/strings/g.run all in scope).

Task 2: MODIFY internal/git/writetree_test.go :: TestWriteTree_MergeConflict (L119-133)
  - KEEP: the existing strings.Contains(err.Error(), "unresolved merge conflicts") assertion.
  - ADD: two !strings.Contains assertions for "fatal: git-write-tree" and "error building trees".
  - REUSE: makeMergeConflict (L22-68) as-is — do not modify or duplicate the fixture.
  - NAMING/PLACEMENT: insert the two new checks between the existing contains-assertion and the sha check.
  - DO NOT: add new test functions or fixtures (the non-conflict write-tree path is out of S1 scope).

Task 3: VERIFY docs (Mode A — gate, not an edit task)
  - RUN: grep -rn -i "merge.conflict\|unresolved\|resolve.*merge\|unmerged" docs/ README.md
  - EXPECT: empty output (no doc quotes/describes the merge-conflict wording). If empty → NO doc edit.
  - IF NON-EMPTY: align only the offending statement to the clean single-line message.
  - DO NOT: proactively add a merge-conflict row to docs/how-it-works.md — that is P1.M5.T1.S2 scope.
```

### Implementation Patterns & Key Details

```go
// PATTERN: every bound method in git.go calls g.run(ctx, g.workDir, <args>...) and branches on err then
// code. The ls-files -u probe follows that exact shape — it is NOT a new pattern.
//
// PATTERN (error style): WriteTree already returns fmt.Errorf for its failure case; the clean conflict
// message uses errors.New (no formatting needed). Both are plain errors → handleGenError default branch
// → exitcode.New(exitcode.Error, err) → exit 1. Identical discipline to the "git binary not found" error
// returned by run().
//
// GOTCHA: the conflict message is a SINGLE line. errors.New("…") has no trailing newline; main's
// `stagehand: <msg>` print adds one. Do not embed "\n" in the message.
//
// WHY a probe (not just dropping %s): write-tree can fail for non-conflict reasons (rare: corrupted
// index). The ls-files -u probe returns the clean message ONLY when conflicts are confirmed, preserving
// the detailed stderr diagnostic for genuine other-cause failures. The minimal "drop %s" variant is
// acceptable per the contract but conflates the two cases.
```

### Integration Points

```yaml
CODE: one block in internal/git/git.go::WriteTree (no new imports, exports, or API change).
DATABASE/OBJECT STORE: none — write-tree still fails before writing a tree on conflict (unchanged).
CONFIG: none.
ROUTES: none.
EXIT CODES: none — still exit 1 via handleGenError default branch (plain error → exitcode.Error).
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Task 1)

```bash
go build ./...
go vet ./internal/git/...
gofmt -l internal/git/git.go     # expect: no output
# If gofmt lists the file, run: gofmt -w internal/git/git.go
```

### Level 2: Unit tests (run after Task 2)

```bash
# Targeted: the extended test must pass (conflict → clean message, no raw noise).
go test -race ./internal/git/... -run TestWriteTree -v

# Whole suite with -race (Makefile `make test` is exactly this) — confirm no regression.
go test -race ./...
# Expected: all PASS. Reasoning (why nothing else breaks):
#   * TestWriteTree_MergeConflict — clean message still contains "unresolved merge conflicts" (KEPT).
#   * TestWriteTree_StagedFiles / _EmptyIndex — happy path; code==0; the if-block is never entered.
#   * TestWriteTree_GitBinaryMissing — run() returns err (LookPath fails) before the if-code block.
#   * TestWriteTree_ContextCancelled — run() returns context.Canceled before the if-code block.
#   * generate/stagehand tests — callers return the WriteTree error as-is; exit-1 semantics unchanged.
```

> **If a previously-green test now fails**: it is almost certainly `TestWriteTree_MergeConflict` failing
> a noise-absence assertion — meaning the clean message still contains raw stderr (the edit didn't fully
> drop the `%s`, or the ls-files probe fell through to the detailed branch). Re-check that the probe's
> condition (`lsErr == nil && strings.TrimSpace(lsOut) != ""`) is correct and that the clean
> `errors.New(...)` line is the one returned on conflict. Do NOT weaken the noise-absence assertions.

### Level 3: Manual / end-to-end (proves the clean message; mirrors the bug repro)

```bash
go build -o /tmp/stagehand ./cmd/stagehand

TMP=$(mktemp -d) && cd "$TMP"
git init -q && git config user.email t@t.com && git config user.name t
printf 'l\n' > f.txt && git add f.txt && git commit -q -m init
git checkout -q -b o && printf 'lo\n' > f.txt && git commit -q -am o
git checkout -q master 2>/dev/null || git checkout -q -b master   # branch-name tolerance
printf 'lm\n' > f.txt && git commit -q -am m
git merge o           # -> CONFLICT (expected non-zero)

# point stagehand at any stub/installed provider (merge conflict is detected pre-generation, so the
# provider is never invoked — a stub is fine; even a missing one won't change the conflict message).
printf '[provider.stub]\ncommand = "/bin/true"\noutput = "raw"\nstrip_code_fence = true\n' > config.toml
echo staged > s.txt && git add s.txt   # stage something so write-tree is reached

/tmp/stagehand --config config.toml 2>&1; echo "EXIT=$?"
# EXPECT (after fix): a SINGLE clean line, e.g.
#   stagehand: unresolved merge conflicts in the index — resolve them first, then re-run stagehand
#   EXIT=1
# AND: NO "fatal: git-write-tree", NO "error building trees", NO "<file>: unmerged (...)" lines.
# AND: HEAD/index untouched (the conflict is still there): git ls-files -u  # still lists unmerged entries.
```

### Level 4: Docs gate (verify-and-skip)

```bash
# MUST be empty — confirms no doc quotes/describes the merge-conflict wording.
grep -rn -i "merge.conflict\|unresolved\|resolve.*merge\|unmerged" docs/ README.md || echo "OK: no merge-conflict doc wording"
# If non-empty, align ONLY the offending statement (see Task 3). Do not add a new doc row here.
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./internal/git/...` clean (or `go vet ./...`).
- [ ] `gofmt -l internal/git/git.go internal/git/writetree_test.go` reports nothing.
- [ ] `go test -race ./internal/git/... -run TestWriteTree -v` passes (incl. the two new assertions).
- [ ] `go test -race ./...` — all previously-green tests still PASS.
- [ ] Manual repro (Level 3): merge conflict → exit 1, single clean line, NO raw stderr noise, HEAD/index
      untouched.

### Feature Validation
- [ ] `WriteTree` returns the clean single-line message on the unmerged-index failure (preferred: via the
      `ls-files -u` probe); returns the detailed diagnostic on a non-conflict failure.
- [ ] The clean message contains `unresolved merge conflicts` and contains **none** of
      `fatal: git-write-tree` / `error building trees`.
- [ ] Exit code unchanged (still **1**); the model is never invoked; HEAD/index untouched.
- [ ] Both call sites (`generate.go:156`, `stagehand.go:260`) are unchanged.

### Code Quality Validation
- [ ] Single, surgical block edit in `WriteTree`; no edits to `run()`, other git methods, the call sites,
      `handleGenError`, or `exitcode`.
- [ ] Reuses the existing `g.run` seam and `makeMergeConflict` fixture — no new exec calls, no new imports.
- [ ] Two surgical test additions (`t.Errorf`, not `t.Fatalf`); no new test functions or fixtures.
- [ ] No speculative doc additions (a merge-conflict failure-modes row stays in P1.M5.T1.S2 scope).

### Documentation
- [ ] Docs gate run and recorded (empty = no edit required; non-empty = the single offending line aligned).

---

## Anti-Patterns to Avoid

- ❌ **Don't append the raw stderr to the conflict message** — that is the exact bug being fixed. The `%s`
  must go from the conflict path.
- ❌ **Don't drop the "unresolved merge conflicts" substring** — the existing test assertion (and the PRD
  §13.5 wording) depend on it.
- ❌ **Don't change the call sites or `handleGenError`** — the error propagates as-is to exit 1; no caller
  change is needed or wanted.
- ❌ **Don't add a pre-WriteTree conflict pre-check in `generate.go`/`stagehand.go`/`default_action.go`** —
  the fix belongs inside `WriteTree` (single chokepoint, both call sites covered).
- ❌ **Don't refactor `run()` or add a new git-exec helper** — reuse `g.run(ctx, g.workDir, "ls-files", "-u")`.
- ❌ **Don't change any exit code** — a plain error already maps to exit 1 via the default branch.
- ❌ **Don't proactively add a merge-conflict row to `docs/how-it-works.md`** — that is P1.M5.T1.S2 scope;
  keep S1's docs work to the verify-and-skip gate.
- ❌ **Don't add tests/fixtures for the non-conflict write-tree failure path** — out of S1 scope; the
  contract is exactly the two noise-absence assertions on the existing conflict test.

---

## Confidence Score

**9/10** — The fix is a single, precisely-located block edit (lines 224-226) with an exact before/after,
the `run()` seam and its nil-err-on-non-zero-exit invariant are documented, the propagation path to exit 1
is proven (call sites + `handleGenError` default branch), the test additions are exact (reuse
`makeMergeConflict`, keep the contains-assertion, add two `!strings.Contains` checks), and the docs work
is a verified no-op gate. The -1 reserves for the implementation-variant judgment call (preferred
`ls-files -u` probe vs. minimal drop-`%s`) — both are acceptable and test-equivalent, but a reviewer might
prefer the simpler minimal variant; the PRP specifies the preferred variant as primary with the minimal as
a documented fallback.
