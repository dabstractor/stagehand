# PRP — P1.M3.T1.S1: Add `LogEntry` type and `LogRange` method to `Git` interface + `gitRunner`

> **Scope discipline.** This subtask adds a **new, backward-compatible capability** to the `git.Git`
> surface: a `LogEntry{SHA, Subject}` type and a `LogRange(ctx, baseSHA) ([]LogEntry, error)` method
> that lists commits in the range `baseSHA..HEAD`, oldest-first. It is the **foundational primitive**
> that S2 (`P1.M3.T2.S1` — `rereadFinalCommits`) consumes to fix Issue 3 (post-arbiter stale/dangling
> SHAs in the success report). **S1 does NOT** wire `LogRange` into `decompose.go`, change the arbiter,
> or touch the success-report printer — that is all S2. S1 adds only the type + method + impl + unit
> tests + interface JSDoc.

> ### ⚠️ CRITICAL — the contract's all-zeros range approach is empirically WRONG (corrected below)
> The work-item contract (item d) and `architecture/issue3_post_arbiter_output.md` claim
> `git log <all-zeros>..HEAD` "lists ALL commits reachable from HEAD." **This is false.** Verified
> against the real git binary: the all-zeros SHA is **rejected in every form** —
> `git log <zeros>..HEAD` → `fatal: Invalid revision range` (exit 128); `^<zeros> HEAD` →
> `fatal: bad object`; plain `<zeros>` → `fatal: bad object`. The **only** working form for "all
> commits reachable from HEAD" is the **no-range** `git log --reverse … HEAD`. The implementation
> below detects the all-zeros sentinel and branches to that no-range form. **Do NOT** use
> `<zeros>..HEAD` — it will fail at runtime with exit 128 even on a healthy born repo.

---

## Goal

**Feature Goal**: A new `git.Git.LogRange(ctx, baseSHA)` method that returns `[]LogEntry{SHA, Subject}`
for the commits in `baseSHA..HEAD`, oldest-first, with correct handling of the empty range, the
originally-unborn base (all-zeros sentinel), and the truly-unborn repo. It is the read-only primitive
S2 uses to re-read the final commits after the arbiter amends/rebuilds/creates.

**Deliverable**:
1. A `LogEntry` type (`internal/git/git.go`) and a `LogRange` method added to the `Git` interface + a
   `gitRunner` implementation, running `git log --reverse --format=%H%x1f%s <baseSHA>..HEAD` (or the
   no-range `HEAD` form when `baseSHA` is the all-zeros unborn sentinel).
2. A new test file `internal/git/logrange_test.go` covering the 3-commit range, empty range,
   all-zeros base, and truly-unborn repo.
3. Mode-A JSDoc on the `LogEntry` type and the `LogRange` interface method (this interface doc **rides
   with the work** — it is the documentation for a brand-new API).

**Success Definition**:
- On a 3-commit repo A→B→C: `LogRange(A)` returns `[B, C]` oldest-first with correct SHAs + subjects;
  `LogRange(HEAD)` returns `nil` (empty range); `LogRange(all-zeros)` returns `[A, B, C]` oldest-first;
  on a truly-unborn repo `LogRange(all-zeros)` returns `(nil, nil)`.
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and the **entire existing** `go test ./...` suite
  stays green (the interface extension is backward-compatible — `git.Git` is implemented ONLY by
  `gitRunner`, which S1 updates; there are no mocks to fix).

---

## Why

- **PRD §13.6.5 / FR-M9–M10** (arbiter reconciliation) + **§15.4 / FR42** (success report): after the
  arbiter amends/rebuilds/creates commits, the SHAs in `DecomposeResult.Commits` are stale (dangling)
  and an arbiter-created (N+1)-th commit is omitted entirely (full root cause:
  `architecture/issue3_post_arbiter_output.md`). The fix is to **re-read the final commits** in the
  range `preRunHEAD..HEAD` and replace the loop's pre-arbiter entries.
- **Gap**: the `Git` interface has **no** range-based commit listing today (`RecentMessages`,
  `RecentSubjects`, `CommitCount` are all `HEAD`-anchored with `-<n>` counts). S1 closes that gap with
  `LogRange`, so S2 can do `deps.Git.LogRange(ctx, preRunHEAD)` and pair each entry with `DiffTree`.
- **Why a dedicated method (not reusing RecentSubjects)**: `RecentSubjects` returns only subjects
  (no SHAs) and is `HEAD`-anchored; the fix needs **full SHAs** (for `[<short-sha>]` in the report and
  for `DiffTree`) over an explicit **range** (`preRunHEAD..HEAD`), excluding the pre-run tip.

---

## What

### The `LogEntry` type (place near `FileChange`, git.go ~L11-19)

```go
// LogEntry is one commit in a log range (oldest-first when produced by LogRange).
// It is the post-arbiter re-read primitive (PRD §13.6.5 / FR-M9): after the arbiter amends/rebuilds/
// creates commits, the orchestrator re-reads the final commits in preRunHEAD..HEAD via LogRange and
// pairs each entry's SHA with DiffTree to rebuild the accurate success report (FR42).
type LogEntry struct {
	SHA     string // full 40/64-hex commit SHA (git %H)
	Subject string // first line of the commit message (git %s — single-line by construction)
}
```

### The `LogRange` interface method (append as the LAST method in `Git`, after `WorkingTreeDiff`)

The `Git` interface closes at the `}` after `WorkingTreeDiff` (~L178/180). Insert this method + JSDoc
immediately before that closing brace:

```go
	// LogRange returns the commits in the range baseSHA..HEAD, oldest-first, as []LogEntry. It runs
	// `git log --reverse --format=%H%x1f%s baseSHA..HEAD`: --reverse yields oldest-first, %H is the
	// full SHA, %x1f (ASCII Unit Separator) delimits SHA from subject (a safe delimiter — subjects
	// never contain \x1f, and %s is single-line by construction), and %s is the subject (first line).
	// It is read-only with respect to refs and the index (PRD §18.1).
	//
	// baseSHA is the pre-run HEAD captured before the decompose loop/arbiter mutated it. The range
	// `baseSHA..HEAD` is "commits reachable from HEAD but not from baseSHA" — i.e. exactly the commits
	// created/rewritten this run. An empty range (baseSHA == HEAD) returns (nil, nil).
	//
	// Originally-unborn repo: pass the all-zeros SHA strings.Repeat("0", 40) as baseSHA. The
	// `<zeros>..HEAD` range is INVALID (git rejects it as "Invalid revision range"), so LogRange
	// detects the all-zeros sentinel and runs `git log --reverse … HEAD` (NO range) instead — listing
	// ALL commits reachable from HEAD, which on an originally-unborn repo are exactly the commits
	// created this run. (A real all-zeros ref is never a valid git range base.)
	//
	// Truly-unborn repo (HEAD has no commits): git exits 128 ("ambiguous argument 'HEAD'"); LogRange
	// returns (nil, nil) — the 128-as-non-error convention shared with RevParseHEAD / RecentSubjects /
	// CommitCount. Any other non-zero exit is a real error.
	LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error)
```

### The `gitRunner` implementation (place after `CommitCount`, ~L751-756)

```go
// LogRange returns the commits in the range baseSHA..HEAD, oldest-first (see the interface doc).
// Implementation: `git log --reverse --format=%H%x1f%s <baseSHA>..HEAD`, parsed one LogEntry per line.
// For the all-zeros unborn sentinel the `<zeros>..HEAD` range is invalid, so it runs the no-range
// `... HEAD` form instead (all commits reachable from HEAD).
func (g *gitRunner) LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error) {
	args := []string{"log", "--reverse", "--format=%H%x1f%s"}
	if baseSHA == strings.Repeat("0", 40) {
		// Originally-unborn base: the all-zeros ref is NOT a valid `git log` range base (git rejects
		// `<zeros>..HEAD` as "Invalid revision range"). List ALL commits reachable from HEAD instead —
		// on an originally-unborn repo these are exactly the commits created this run.
		args = append(args, "HEAD")
	} else {
		args = append(args, baseSHA+"..HEAD")
	}

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // truly-unborn repo (no commits on HEAD) — 128-as-non-error (matches RevParseHEAD/RecentSubjects/CommitCount)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var entries []LogEntry
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue // trailing newline → trailing empty element
		}
		sha, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue // defensive: skip a line lacking the %x1f delimiter
		}
		entries = append(entries, LogEntry{SHA: sha, Subject: subject})
	}
	return entries, nil
}
```

Notes for the implementer:
- `err`, `fmt`, `strings` are **already imported** in `git.go` (L3-12) — no new imports. Go 1.22 →
  `strings.Cut` (Go 1.18+) is available and is the cleanest SHA/subject split.
- `strings.Cut(line, "\x1f")` returns `(sha, subject, ok)`: `ok=false` only if there's no `\x1f`
  (shouldn't happen with the format, but the guard is defensive). When the subject is empty the line
  is `<sha>\x1f` → `sha=<sha>, subject="", ok=true` (handled correctly).
- The all-zeros sentinel `strings.Repeat("0", 40)` matches the codebase's existing "unborn/root"
  convention (e.g. `UpdateRefCAS` callers pass all-zeros as `expectedOld` for a root commit). **Do
  not** also special-case `""` — the S2 caller will pass all-zeros for the unborn case (clear contract).

### Backward compatibility (why nothing else breaks)

`git.Git` is implemented **only** by `*gitRunner` (the sole `New() Git` returns `*gitRunner`,
git.go:190; there are no mocks — all tests use real git). Adding a method to the interface forces
`gitRunner` to implement it, which S1 does in the same change. Every existing `git.Git` consumer keeps
compiling unchanged. Verified: `grep` shows no other `Git` implementors.

### Success Criteria

- [ ] `LogEntry{SHA, Subject}` type exists in `internal/git/git.go`.
- [ ] `LogRange(ctx, baseSHA) ([]LogEntry, error)` is a `Git` interface method with a `gitRunner` impl.
- [ ] `LogRange(A)` on a 3-commit A→B→C repo returns `[B, C]` oldest-first (correct SHAs + subjects).
- [ ] `LogRange(<HEAD>)` returns `nil` (empty range); `LogRange(all-zeros)` returns `[A, B, C]`.
- [ ] `LogRange(all-zeros)` on a truly-unborn repo returns `(nil, nil)`.
- [ ] The interface method + type carry JSDoc (range semantics, oldest-first, %x1f, all-zeros, 128).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, and `go test ./...` all pass.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact type/method/impl (copy-ready), exact placement lines, the `run()`
seam + 128-as-non-error convention, the empirically-correct all-zeros handling (correcting the
contract), the backward-compat proof, and the exact test cases + helpers are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause of Issue 3 + the LogRange design)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue3_post_arbiter_output.md
  why: Names LogRange as Step 1 of the Issue-3 fix; documents the %x1f delimiter, oldest-first via
       --reverse, and the all-zeros unborn base. NOTE its all-zeros-range claim is WRONG (see below);
       the corrected no-range approach is in this PRP + the research note.
  section: "### Step 1: Add a git commit-range listing function"
  critical: Do NOT use `<zeros>..HEAD` — empirically exit 128 "Invalid revision range". Use the
            no-range `HEAD` form when baseSHA is all-zeros (see research note + this PRP).

# The edit site + the git-exec seam
- file: internal/git/git.go
  why: (a) LogEntry goes near FileChange (L11-19). (b) LogRange interface method appends after
       WorkingTreeDiff (interface closes ~L178/180). (c) gitRunner.LogRange impl goes after
       CommitCount (~L751-756). run() (L204) is the only git-exec helper; returns
       (stdout, stderr, code, err) with err==nil on a non-zero git exit.
  pattern: Mirror RecentSubjects (L698) / CommitCount (L735): run() → check err → `if code == 128
           { return <zero>, nil }` → `if code != 0 { fmt.Errorf(...) }` → parse stdout. The
           `--format=%x00%B` + split-on-\x00 in RecentMessages (L646) is the NUL-delimited precedent;
           LogRange's `%H%x1f%s` + split-on-\x1f is the SHA/subject analogue.
  gotcha: The all-zeros SHA is NOT a valid `git log` range/exclude/revision arg in ANY form (all exit
          128). Detect the sentinel and use the no-range `HEAD` form. On a truly-unborn repo even the
          no-range form exits 128 ("ambiguous argument 'HEAD'") → return (nil, nil).

# Patterns to mirror in the implementation
- file: internal/git/git.go :: RecentSubjects (L698)
  why: Same shape (git log --format=…, 128→nil, split + trim). LogRange is the range + SHA variant.
- file: internal/git/git.go :: RecentMessages (L646)
  why: NUL-delimited multi-field precedent (`%x00%B`); LogRange's `%x1f` delimiter is the analogue.

# Backward-compat proof (the only Git implementor)
- file: internal/git/git.go :: New (L190)
  why: `func New(workDir string) Git { return &gitRunner{workDir: workDir} }` — the ONLY Git factory;
       *gitRunner is the ONLY implementor. No mocks exist (all tests use real git). Adding an interface
       method breaks no other implementor.

# Test conventions + helpers
- file: internal/git/git_test.go
  why: initRepo(t, dir) (L12) inits a repo with test identity. TestNew (L30) shows the *gitRunner
       cast pattern. These are the shared helpers reused by every git test file.
- file: internal/git/recentsubjects_test.go
  why: Per-method test-file convention (one file per Git method) + uses makeEmptyCommit(t, repo, msg)
       to create commits + the assertion style to mirror. LogRange's test goes in logrange_test.go.
  pattern: TestRecentSubjects_ReturnsSubjects (L24) creates 3 commits and asserts order/content.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/git.go            # Git interface (closes ~L178) + gitRunner + run() (L204); log methods L646-756
internal/git/git_test.go       # initRepo (L12) + TestNew / TestRun helpers
internal/git/recentsubjects_test.go  # makeEmptyCommit + per-method test-file convention to mirror
```

### Desired Codebase tree with files changed

```bash
internal/git/git.go            # MODIFIED — +LogEntry type, +LogRange interface method, +gitRunner.LogRange impl
internal/git/logrange_test.go  # NEW — unit tests for LogRange (3-commit range, empty, all-zeros, unborn)
# (no other source files touched; S2 wires LogRange into decompose.go)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (empirically verified): git REJECTS the all-zeros SHA in every log form —
//   `git log 0000…0000..HEAD` → exit 128 "Invalid revision range"; `^0000…0000 HEAD` → "bad object";
//   plain `0000…0000` → "bad object". ONLY the no-range form `git log --reverse … HEAD` lists all
//   commits reachable from HEAD. LogRange MUST branch on the all-zeros sentinel to the no-range form.
//   (On a truly-unborn repo even the no-range form exits 128 "ambiguous argument 'HEAD'" → (nil,nil).)

// CRITICAL: run() returns err==nil on a non-zero git exit (gotcha G2) — code carries the exit status.
//   So branch on `code` (not `err`) for 128-as-non-error and for the generic failure. This matches
//   RevParseHEAD/RecentSubjects/CommitCount exactly.

// GOTCHA: %s subjects are single-line by git construction (no embedded newline) and never contain
//   \x1f (ASCII Unit Separator, a non-printable control char) — so splitting each line on the FIRST
//   \x1f via strings.Cut is unambiguous. Do NOT split the whole stdout on \x1f (that would merge the
//   SHA of one commit with the subject of another across newlines); split stdout on "\n" first, then
//   Cut each line on "\x1f".

// GOTCHA: `git log A..HEAD` EXCLUDES A (reachable from HEAD but NOT from A). So LogRange(A) on
//   A→B→C returns [B, C], not [A, B, C]. The base commit is intentionally excluded — it is the
//   pre-run tip, not a commit created this run. (The all-zeros/unborn path returns ALL commits because
//   there was no pre-run tip to exclude.)

// GOTCHA: The all-zeros sentinel is strings.Repeat("0", 40) — the codebase's "unborn/root" convention
//   (UpdateRefCAS callers pass it as expectedOld for a root commit). There is no exported constant for
//   it; reuse strings.Repeat("0", 40) inline. The S2 caller passes all-zeros (not "") for the unborn
//   case — do NOT special-case "" (keep the contract to the documented sentinel).
```

---

## Implementation Blueprint

### Data model

```go
// LogEntry is one commit in a log range (oldest-first when produced by LogRange).
type LogEntry struct {
	SHA     string // full 40/64-hex commit SHA (git %H)
	Subject string // first line of the commit message (git %s — single-line by construction)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go :: add LogEntry type
  - ADD: the LogEntry struct (code above) near FileChange (L11-19) — the analogous "one entry in a
         listing" struct. Keep the field doc comments.
  - NAMING: LogEntry (CamelCase), SHA/Subject (exported, matching FileChange's exported fields).
  - DEPENDENCIES: none.

Task 2: MODIFY internal/git/git.go :: add LogRange to the Git interface
  - ADD: the LogRange interface method + its JSDoc (code above) as the LAST method, immediately before
         the `}` that closes the Git interface (after WorkingTreeDiff, ~L178/180).
  - DOC (Mode A): the JSDoc rides with the work — cover range semantics (baseSHA..HEAD), oldest-first,
         %x1f delimiter, the all-zeros special case (no-range form), and 128-as-non-error.
  - DEPENDENCIES: LogEntry (Task 1) is referenced in the signature.

Task 3: MODIFY internal/git/git.go :: add gitRunner.LogRange implementation
  - ADD: the gitRunner.LogRange method (code above), placed after CommitCount (~L751-756) with the
         other log-family methods.
  - REUSE: g.run(ctx, g.workDir, args...) — the existing git-exec seam (L204). Do NOT add a new exec
           call or import (err/fmt/strings already imported; Go 1.22 ⇒ strings.Cut available).
  - BRANCHING: err→return; code==128→(nil,nil); code!=0→fmt.Errorf; else parse. Mirror RecentSubjects.
  - ALL-ZEROS: detect strings.Repeat("0",40) → no-range `HEAD` form (NOT `<zeros>..HEAD`).
  - PARSING: split stdout on "\n", skip "", strings.Cut each line on "\x1f", skip !ok, append LogEntry.
  - DEPENDENCIES: the interface method (Task 2) + LogEntry (Task 1).

Task 4: CREATE internal/git/logrange_test.go
  - IMPLEMENT: unit tests mirroring recentsubjects_test.go's style + helpers.
  - CASES: (1) 3-commit A→B→C: LogRange(A)→[B,C] oldest-first, correct SHAs+subjects; (2) empty range
           LogRange(HEAD)→nil; (3) all-zeros base LogRange(strings.Repeat("0",40))→[A,B,C]; (4) truly-
           unborn repo (initRepo only) LogRange(all-zeros)→(nil,nil). Optional: special-chars subject.
  - REUSE: initRepo(t, repo) (git_test.go:12) + makeEmptyCommit(t, repo, msg) (used across git tests).
  - SHAS: capture commit SHAs in-test via `git -C <repo> rev-parse HEAD~2` (or a run/gitOut helper) to
          assert exact SHA equality; mirror recentsubjects_test.go assertion style.
  - NAMING: TestLogRange_<Scenario> (matches TestRecentSubjects_<Scenario> convention).
  - COVERAGE: all 4 cases above (positive + edge); no mocks (real git, like every other git test).
  - DEPENDENCIES: the impl (Task 3).
```

### Implementation Patterns & Key Details

```go
// PATTERN (gitRunner log method): see RecentSubjects (L698) / CommitCount (L735):
//   stdout, stderr, code, err := g.run(ctx, g.workDir, <args>...)
//   if err != nil { return <zero>, err }              // infrastructural (missing git / cancelled)
//   if code == 128 { return <zero>, nil }             // unborn signal (128-as-non-error)
//   if code != 0 { return <zero>, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
//   // parse stdout ...
// LogRange follows this exactly, adding the all-zeros→no-range branch before the run() call.
//
// PATTERN (delimiter split): RecentMessages uses --format=%x00%B + strings.Split(stdout, "\x00").
// LogRange uses --format=%H%x1f%s + (per LINE) strings.Cut(line, "\x1f"). Split stdout on "\n" FIRST,
// then Cut each line — never Cut the whole stdout (that crosses commit boundaries).
//
// GOTCHA: LogRange is a pure addition to the Git interface. Since *gitRunner is the only implementor
// and S1 adds the method to it, no other file changes to satisfy the interface. S2 is the first caller.
```

### Integration Points

```yaml
CODE: internal/git/git.go only (+ new logrange_test.go). No new imports; Go 1.22 strings.Cut used.
DATABASE/OBJECT STORE: none — `git log` is read-only (no object writes, no ref/index mutation).
CONFIG: none.
ROUTES: none.
INTERFACE CONTRACT: LogRange is additive (Git gains one method). Backward-compatible — no mock updates.
S2 HANDOFF: rereadFinalCommits will call deps.Git.LogRange(ctx, preRunHEAD-or-all-zeros); for an
            originally-unborn repo it passes strings.Repeat("0",40) (NOT "").
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Tasks 1-3)

```bash
go build ./...
go vet ./internal/git/...
gofmt -l internal/git/git.go     # expect: no output
# If gofmt lists the file, run: gofmt -w internal/git/git.go
```

### Level 2: Unit tests (run after Task 4)

```bash
# The new LogRange tests (all 4 cases must pass).
go test ./internal/git/... -run TestLogRange -v

# Confirm the interface still has a single satisfied implementor (no other package breaks) + full suite.
go test ./...
# Expected: all PASS. Reasoning (why nothing else breaks):
#   * Git is implemented ONLY by *gitRunner; S1 adds LogRange to it in the same change → interface satisfied.
#   * No mocks exist (all git tests use real git) → no other implementor to update.
#   * LogRange is brand-new; no existing caller references it yet (S2 will be the first).
```

> **If a previously-green test now fails**: it would have to be a compile failure elsewhere claiming
> `Git` is unimplemented — impossible since `*gitRunner` now implements LogRange. A failure inside
> `TestLogRange_*` means the parsing/sentinel logic is off — re-check the all-zeros branch uses the
> no-range `HEAD` form (NOT `<zeros>..HEAD`) and that the split is per-line (`\n`) then per-field (`\x1f`).

### Level 3: Manual / empirical (proves the git semantics; mirrors the research verification)

```bash
TMP=$(mktemp -d) && cd "$TMP"
git init -q && git config user.email t@e.com && git config user.name t
printf 'a\n' > f && git add f && git commit -q -m "commit A"
printf 'b\n' > f && git commit -q -am "commit B"
printf 'c\n' > f && git commit -q -am "commit C"
A=$(git rev-list --max-parents=0 HEAD)

echo "=== range A..HEAD (oldest-first) — expect B then C ==="
git log --reverse --format='%H%x1f%s' $A..HEAD | awk -F'\x1f' '{print substr($1,1,8)" "$2}'

echo "=== empty range HEAD..HEAD — expect nothing (exit 0) ==="
git log --reverse --format='%H%x1f%s' HEAD..HEAD; echo "[exit=$?]"

echo "=== all-zeros MUST use no-range HEAD form (NOT <zeros>..HEAD) ==="
git log --reverse --format='%s' 0000000000000000000000000000000000000000..HEAD 2>&1 | head -1   # FAILS: Invalid revision range
git log --reverse --format='%s' HEAD                                                          # WORKS: A B C

echo "=== unborn repo, no-range HEAD exits 128 (LogRange returns nil,nil) ==="
cd "$(mktemp -d)" && git init -q
git log --reverse --format='%H%x1f%s' HEAD >/dev/null 2>&1; echo "[exit=$? (expect 128)]"
```

### Level 4: Interface-doc consistency

```bash
# Confirm the JSDoc covers the four required points (Mode A — doc rides with the work).
grep -n "LogRange\|LogEntry" internal/git/git.go
# Expect: the LogEntry type, the LogRange interface method, AND the gitRunner impl, each present; the
# interface method's doc should mention: baseSHA..HEAD, oldest-first (--reverse), %x1f, all-zeros special case.
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./internal/git/...` clean (or `go vet ./...`).
- [ ] `gofmt -l internal/git/git.go internal/git/logrange_test.go` reports nothing.
- [ ] `go test ./internal/git/... -run TestLogRange -v` passes all 4 cases.
- [ ] `go test ./...` — all previously-green tests still PASS (interface extension is backward-compatible).

### Feature Validation
- [ ] `LogEntry{SHA, Subject}` type exists with doc comments.
- [ ] `LogRange(ctx, baseSHA) ([]LogEntry, error)` is on the `Git` interface with a `gitRunner` impl.
- [ ] `LogRange(A)` on A→B→C → `[B, C]` oldest-first, correct SHAs + subjects (base A excluded).
- [ ] `LogRange(<HEAD>)` → `nil` (empty range); `LogRange(all-zeros)` → `[A, B, C]` (no-range form).
- [ ] `LogRange(all-zeros)` on a truly-unborn repo → `(nil, nil)` (128-as-non-error).
- [ ] The all-zeros branch uses the **no-range `HEAD` form**, NOT `<zeros>..HEAD` (empirically verified).

### Code Quality Validation
- [ ] Follows the existing `run()` + 128-as-non-error + delimiter-split patterns (RecentSubjects analogue).
- [ ] Reuses `g.run`, `strings.Cut`, `strings.Repeat("0",40)` — no new imports, no new exec helper.
- [ ] `LogEntry` placed near `FileChange`; impl placed with the log-family methods; interface method last.
- [ ] Test file follows the per-method convention (`logrange_test.go`) and reuses `initRepo`/`makeEmptyCommit`.
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] Interface method JSDoc covers range semantics, oldest-first, %x1f delimiter, all-zeros special case,
      128-as-non-error (Mode A — rides with the work; no other doc file references LogRange yet).

---

## Anti-Patterns to Avoid

- ❌ **Don't use `<all-zeros>..HEAD`** — empirically `fatal: Invalid revision range` (exit 128). Use the
  no-range `HEAD` form when `baseSHA` is the all-zeros sentinel. (This corrects the contract + arch doc.)
- ❌ **Don't split the whole stdout on `\x1f`** — that merges one commit's SHA with the next's subject.
  Split on `\n` first, then `strings.Cut` each line on `\x1f`.
- ❌ **Don't branch on `err` for the 128 case** — `run()` returns `err==nil` on a non-zero git exit
  (gotcha G2); branch on `code == 128`.
- ❌ **Don't wire LogRange into decompose.go / the arbiter / the printer** — that is S2. S1 is the
  primitive + its unit tests only.
- ❌ **Don't add a mock or a second Git implementor** — all git tests use real git; the interface is
  satisfied by adding the method to `*gitRunner` alone.
- ❌ **Don't special-case `""` as the unborn base** — the contract's sentinel is `strings.Repeat("0",40)`
  (the codebase's "unborn/root" convention). Keep the contract to that sentinel for S2 clarity.
- ❌ **Don't forget `--reverse`** — without it the range is newest-first; the success report needs
  oldest-first (chronological commit order).
- ❌ **Don't drop the JSDoc** — Mode A requires the interface doc to ride with the new API.

---

## Confidence Score

**9/10** — The type, interface method, and implementation are copy-ready and verified against the real
git binary (including the critical correction that the all-zeros range is invalid → no-range `HEAD`
form). The `run()`/128-as-non-error/delimiter-split patterns are pinned to exact reference methods
(RecentSubjects/RecentMessages), backward compatibility is proven (`*gitRunner` is the sole
implementor, no mocks), and the four test cases + helpers are specified. The -1 reserves for the
test-helper detail (`makeEmptyCommit`'s exact definition and how the test captures the A SHA) — both
are routine to resolve by grepping the existing git test files; non-blocking.
