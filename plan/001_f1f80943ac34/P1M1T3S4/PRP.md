---
name: "P1.M1.T3.S4 — RecentSubjects for duplicate detection"
description: |
  Replace ONE panic-stub in `internal/git/git.go` — `(*gitRunner).RecentSubjects` (landed as a stub by
  P1.M1.T2.S1) — with its real implementation. It feeds the **duplicate-rejection** loop (PRD
  §9.7/FR31, §17.3): the dedupe check (P1.M3.T2) builds a set/map from the returned `[]string` for
  O(1) exact-match lookup against a freshly-generated commit subject. It runs
  `git -C <repo> log --format=%s -<n>`, which emits EXACTLY ONE LINE PER COMMIT (git's `%s` placeholder
  is the subject = first line, and CANNOT contain a newline).
  ⚠️ **THE central design call (vs the concurrent sibling T3.S3):** RecentMessages (T3.S3) MUST use the
  NUL-delimited format `git log --format='%x00%B'` and split on `\x00`, because `%B` FULL BODIES may
  contain a markdown horizontal rule `---` that a naive `---`/`\n` split would fracture (FINDING 9).
  RecentSubjects does NOT need NUL: `%s` is single-line by git's definition, so a `"\n"` split is both
  safe and simpler. Using `%x00` here would be cargo-culting T3.S3 without understanding WHY T3.S3
  needed it. There is consequently **NO line cap** (each subject is one line; the caller bounds `n` at
  50 per FR31 — at most 50 short lines, no prompt-budget risk) and **NO new constant** and **NO new
  import** (uses only `strings` + `fmt`, both already present after T3.S3 lands). Delegates to S1's
  `run()` helper (NOT exec directly — mirrors every landed method). Branch order is byte-identical to
  RecentMessages/RevParseHEAD: `err != nil` → infrastructural failure; `code == 128` → unborn ⇒
  `(nil, nil)`; `code != 0` → wrapped error; `code == 0` → split on `"\n"`, TrimSpace each, drop
  empties (incl. the terminal trailing-empty), return newest-first. Adds ONE test file
  `internal/git/recentsubjects_test.go` (`package git`, white-box) with a 9-function matrix reusing
  `initRepo` (git_test.go) + `makeEmptyCommit` (revparse_test.go, S2) — no new helpers. Removes the
  ONE `RecentSubjects` line from `git_test.go`'s `TestStubsPanic` (required now that it is real —
  mirrors S2/S3/S4/S5/S6/T3.S1/T3.S2/T3.S3). Touches ONLY `internal/git/`; no interface, struct,
  `run()`, `runWithInput`, `FileChange`, `StagedDiffOptions`, RevParseHEAD, WriteTree, CommitTree,
  UpdateRefCAS, DiffTree, parseDiffTree, StagedDiff, HasStagedChanges, RecentMessages, CommitCount, or
  the remaining stub (`AddAll`). This is the FIFTH of the six P1.M1.T3 methods to leave stub-status
  (StagedDiff T3.S1, HasStagedChanges T3.S2 landed; RecentMessages+CommitCount T3.S3 landing
  concurrently; AddAll T3.S5 remains).
---

## Goal

**Feature Goal**: Implement the single git-log read-only method on `*gitRunner` that powers the
prompt pipeline's **duplicate-rejection** path (PRD §9.7/FR31, §17.3). `RecentSubjects` returns the
up-to-`n` most-recent commit **subjects** (first lines) that the dedupe loop (P1.M3.T2) turns into a
set/map for O(1) exact-match lookup against a freshly-generated subject — the data behind FR32's
"if the subject exactly matches one of the 50, retry generation with a rejection list."

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: replace the `RecentSubjects` panic-stub body with the real
   `git log --format=%s -<n>` + `"\n"`-split body (keep the signature
   `func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error)` byte-for-byte).
   **No import change. No new constant.**
2. **MODIFY** `internal/git/git_test.go`: remove the ONE line from `TestStubsPanic` —
   `assertPanics(t, "RecentSubjects", …)`.
3. **CREATE** `internal/git/recentsubjects_test.go` (`package git`): the 9-function test matrix.

No other files touched. **Zero new dependencies** (uses `strings` + `fmt`, both already imported).
`go.mod` unchanged (stdlib only).

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 9 new test functions passing (plus all prior tests
green); `RecentSubjects` returns `(nil, nil)` on an unborn repo, returns `n` newest-first subjects on
an N≥n-commit repo, returns `< n` subjects without error when `n` exceeds the commit count, returns
ONLY the subject (first line — never the body) for a multi-line commit, preserves a `---` inside a
subject intact (proving the `\n` split is safe for `%s`, in contrast to FINDING 9's NUL requirement
for `%B`), returns `(nil, nil)` for `n <= 0` without calling git, surfaces a missing git binary as
"git binary not found" and a cancelled context as `errors.Is(err, context.Canceled)`; `run()`,
`runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`, `StagedDiffOptions`,
`RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4), `UpdateRefCAS`/`ErrCASFailed` (S5),
`DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants/`defaultExcludes` (T3.S1),
`HasStagedChanges` (T3.S2), `RecentMessages`/`maxRecentMessageLines` (T3.S3), `CommitCount` (T3.S3),
and the remaining stub (`AddAll`) are byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T2 duplicate-rejection loop (the primary consumer) and — transitively —
the `CommitStaged` orchestrator (P1.M3.T4) and the CLI default action (P1.M4.T1.S2).

**Use Case**: After generation produces a candidate commit message, the orchestrator extracts its
subject (first line, FR30) and asks `recent, err := g.RecentSubjects(ctx, 50)` ONCE (before the
retry loop). It builds `seen := map[string]struct{}{}` from `recent` for O(1) exact-match lookup.
If the candidate subject is in `seen`, FR32 fires: retry generation with a rejection list appended
to the user prompt (§17.3), up to `max_duplicate_retries` (default 3). The set is built ONCE and
reused across all retries (the history does not change between retries — only the candidate does).

**User Journey**: `g := git.New(repoPath)` → `recent, err := g.RecentSubjects(ctx, 50)` → if
`err != nil`: surface + abort; else `seen := make(map[string]struct{}, len(recent))` →
`for _, s := range recent { seen[s] = struct{}{} }` → `if _, dup := seen[candidateSubject]; dup {
retry }`.

**Pain Points Addressed**: (1) Without a typed `[]string` of recent subjects, the dedupe loop would
re-parse `git log` output inline — duplicating the format/split logic and the unborn-repo edge case.
(2) The caller wants EXACT subjects only (not full bodies) — `RecentMessages` (T3.S3) returns full
bodies, which would require the caller to extract the first line of each (redundant work + the
markdown-`---`-in-body edge case). `RecentSubjects` returns exactly what the dedupe set needs: a
slice of one-line strings, ready for `map` membership.

## Why

- **PRD §9.7 / FR31 (Duplicate rejection, P0 → G6):** *"FR31. Fetch the last 50 commit subjects
  (`git log --format=%s -50`)."* This method IS FR31. The `50` is the caller's default (FR32's
  retry count is separate); `n` is parameterized so the method is testable and not hard-coded.
- **PRD §9.7 / FR32:** *"If the subject exactly matches one of the 50, retry generation …"* — the
  "one of the 50" is the set built from this method's return.
- **PRD §17.3 (the rejection block):** the rejected subjects appended to the retry prompt are the
  exact-match hits against this set.
- **PRD §13/§11.1 (read-only):** this method mutates no ref/object (it only reads history).
- **PRD §18.1 (the invariant):** refs/index modified only at update-ref — this method is far
  upstream of that.
- **Foundation for P1.M3.T2 (duplicate rejection loop):** the dedupe loop is BLOCKED on this method.
  It cannot build its membership set without the `[]string` this returns.

## What

### RecentSubjects

Runs `git -C <workDir> log --format=%s -<n>` via `run()` and translates:

- `n <= 0` (defensive guard, D3) → return `(nil, nil)` without calling git (avoids undefined
  `git log -0` behavior). The caller passes 50 (PRD FR31).
- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `(nil, err)`. Infrastructural-failure guard.
- `code == 128` → return `(nil, nil)`. Unborn repo (zero commits) OR non-repo dir (both exit 128 —
  inherited indistinguishability from S2's RevParseHEAD; see gotcha G4). The contract mandates
  "return empty slice on unborn repo"; callers gate on RevParseHEAD first, so 128 here is the
  defensive path. (On a new repo the duplicate check is vacuous — there is nothing to duplicate.)
- `code != 0` (any non-zero, non-128) → return
  `(nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr)))`.
- `code == 0` → `strings.Split(stdout, "\n")`; for each line: `TrimSpace`; if empty, skip; else
  append. Return the accumulated `[]string` (newest-first, git log order). **No line cap** (each
  subject is one line; `n` is bounded by the caller — D2).

No porcelain, no shell, no `cmd.Dir`/`os.Chdir` (inherited from `run()`).

### Success Criteria

- [ ] `(*gitRunner).RecentSubjects` body matches §Implementation Blueprint (delegates to `run()`;
      `--format=%s`; split on `"\n"`; TrimSpace + drop empty; NO NUL; NO line cap).
- [ ] NO `%x00` format and NO NUL split (this is the central design call — `%s` is single-line, so
      `\n` is correct; NUL would be cargo-cult from T3.S3).
- [ ] NO new import added (uses `strings` + `fmt`, both present); NO new constant added.
- [ ] `RecentSubjects` returns `(nil, nil)` on unborn, `(nil, nil)` for `n <= 0`, `n` newest-first
      subjects on a born repo, and `len(result) <= n` (never errors when `n` exceeds commit count).
- [ ] A multi-line commit yields ONLY its subject (first line) in the result — the body is excluded.
- [ ] A subject containing `---` survives intact inside ONE returned element (proves `\n` split is
      safe for `%s`; contrasts with FINDING 9's NUL requirement for `%B`).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4),
      `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants/
      `defaultExcludes` (T3.S1), `HasStagedChanges` (T3.S2), `RecentMessages`/`maxRecentMessageLines`
      (T3.S3), `CommitCount` (T3.S3) byte-identical.
- [ ] The remaining stub (`AddAll`) untouched.
- [ ] `internal/git/recentsubjects_test.go` exists in `package git` with the 9-function test matrix.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `RecentSubjects` line.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean; `go test -race ./internal/git/`
      exits 0; `make test` exits 0.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path (`github.com/dustin/stagecoach`); the
exact three files to touch (and the exact one line to remove from `git_test.go`); the exact `run()`
contract; the exact method body (empirically verified against git 2.54.0); the empirically-pinned
exit codes (unborn=128, non-repo=128, born=0); the central `\n`-vs-`%x00` design call with empirical
proof; the exact test matrix with reused helpers; and the exact validation commands. No inference
required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.7/FR31 (Duplicate rejection: 'FR31. Fetch the last 50 commit subjects
        (git log --format=%s -50).'); §9.7/FR32 (the exact-match check against these 50, and the
        rejection-list retry); §17.3 (the rejection block appended on retry — populated from the
        duplicate hits against this set); §13/§11.1 (read-only — mutates no ref/object); §18.1 (the
        invariant: refs/index modified only at update-ref)."
  critical: "This subtask owns ONLY the RecentSubjects body + its test file + the one TestStubsPanic
             line removal. Do NOT implement the dedupe set-build (P1.M3.T2), the rejection-list
             retry (P1.M3.T2), AddAll (T3.S5), the prompt builder (P1.M3.T1), the orchestrator
             (P1.M3.T4), or the CLI. Do NOT change the Git interface. Do NOT add a line cap or a
             NUL format (those belong to RecentMessages/T3.S3, NOT here — see §Why-not-NUL below)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 9 — the NUL-delimiter finding. CRITICAL TO READ IT CORRECTLY: it says commit-pi's
        git log --format='---%n%B' split on '---' broke on markdown horizontal rules INSIDE COMMIT
        BODIES (%B). The fix (use %x00 and split on \\x00) applies to %B FULL-BODY queries ONLY.
        RecentSubjects uses %s (SUBJECT = first line), which is single-line by git's definition and
        CANNOT contain a newline — so FINDING 9's NUL fix does NOT apply here. A '\\n' split is
        correct and simpler."
  critical: "Do NOT cargo-cult the %x00 format from T3.S3. FINDING 9 is about %B bodies, not %s
             subjects. Verified empirically: a subject containing '---' stays on ONE line under
             --format=%s (see research §2). Using %x00 here would be wrong-for-the-reason and would
             obscure the simpler correct model. Also read FINDING 1 (exit 128 = unborn signal,
             inherited by every read method including this one)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "The 'git log --format' / '%s' placeholder section: documents %s as 'the subject
        (first line)' — the exact placeholder FR31 names. Atomic Commit Sequence context (this is a
        read-only history query invoked before/during generation)."
  critical: "%s is the SUBJECT — one line, no body. %B is the full body (multi-line). Do NOT confuse
             them (T3.S3 uses %B and needs NUL; this uses %s and needs only \\n). The Go reference
             patterns there exec directly; we DELEGATE to run() (which exposes exitCode with
             err==nil). Do NOT copy the exec plumbing."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything this method consumes: the gitRunner struct; the run() helper
        (exact signature and verified body — NOT modified, only called); the RecentSubjects
        panic-stub being replaced; the Git interface (signature ALREADY correct and EXACTLY matching
        the work-item description: RecentSubjects(ctx, n int) ([]string, error) — no conflict,
        unlike T3.S3's maxLines); New(); the git_test.go initRepo helper; and the assertPanics
        helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero exit
             is not a Go error) is the foundation this method's code branches rely on. The interface
             signature is FIXED — do NOT change it (no need to here; it already matches)."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The closest analog PRP — RevParseHEAD is an exit-code-semantics method that delegates to
        run() and branches on code 128 for the 'unborn' signal. Its branch order (err-first, then
        code==128, then code!=0) and its test matrix (git-missing / ctx-cancelled via t.Setenv /
        cancel-before-call / non-repo-via-128) are the templates for RecentSubjects' implementation
        and tests."
  critical: "RevParseHEAD branches on code==128 ⇒ isUnborn. RecentSubjects branches on code==128 ⇒
             (nil, nil). Same 128 semantic, different return shape. S2 ADDED the 'strings' import;
             this subtask adds NONE (strings already present)."

- docfile: plan/001_f1f80943ac34/P1M1T3S3/PRP.md
  why: "The CONCURRENT SIBLING — RecentMessages is the closest structural analog (same git-log
        family, same run() delegation, same branch order, same n<=0 guard, same exit-128⇒nil, same
        TrimSpace+drop-empty parse loop). The RecentSubjects body is RecentMessages' body with three
        SIMPLIFICATIONS: %s instead of %x00%B; '\\n' split instead of '\\x00'; no line cap."
  critical: "T3.S3 is landing concurrently and edits git.go (imports+strconv, maxRecentMessageLines
             const, RecentMessages+CommitCount bodies) + git_test.go (removes RecentMessages+
             CommitCount lines from TestStubsPanic). THIS subtask edits a DIFFERENT git.go region
             (the RecentSubjects stub) + a DIFFERENT TestStubsPanic line (RecentSubjects) and adds NO
             imports. Non-overlapping. Also: do NOT replicate T3.S3's %x00/maxLines design — it exists
             for %B-body reasons that do not apply to %s-subjects (research §2)."

- docfile: plan/001_f1f80943ac34/P1M1T3S3/research/s3_research.md
  why: "T3.S3's research documents the NUL-vs-newline distinction that THIS subtask inverts: T3.S3
        PROVES %B bodies need NUL; the same proof shows %s subjects do NOT (a subject is one line)."
  critical: "Read §2 of s3_research.md for the %x00 proof — then apply the CONTRAPOSITIVE: because %s
             cannot contain a newline, '\\n' is the safe delimiter for RecentSubjects. This is the
             single most important design insight in this subtask."

- docfile: plan/001_f1f80943ac34/P1M1T3S4/research/s4_research.md
  why: "THIS subtask's own research: the \\n-vs-%x00 design call with empirical proof (§2); the
        empirically-pinned exit codes incl. non-repo==128 indistinguishability (§3); the design
        decisions D1–D5 (§4); the parallel-execution non-overlap with T3.S3 (§5); and the 9-function
        test matrix (§7)."
  critical: "§2 is THE anchor: it proves why a '\\n' split is correct for %s (and why importing %x00
             here would be wrong). §5 confirms non-overlap with the concurrent T3.S3."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-log#_pretty_formats
  why: "Documents the %s placeholder ('subject') and confirms git log appends a record separator
        (newline) after each formatted commit. Confirms --format=%s emits one line per commit."
  critical: "Establishes that --format=%s is valid and newline-delimited. Do NOT add an explicit %n
             (git already separates records with a newline); do NOT use %B (that is RecentMessages'
             placeholder, T3.S3)."
- url: https://pkg.go.dev/strings#Split
  why: "strings.Split(stdout, \"\\n\") is the splitter. The final element is \"\" (from the trailing
        newline) — handled by TrimSpace + empty-skip."
  critical: "Split on the single newline character \"\\n\", NOT on a multi-char literal. The trailing
            empty element is expected and dropped."
- url: https://pkg.go.dev/fmt#Sprintf
  why: "fmt.Sprintf(\"-%d\", n) renders the count argument (e.g. \"-50\") passed to git log. Mirrors
        RecentMessages (T3.S3) exactly for reviewer consistency."
  critical: "fmt is already imported (after T3.S3 lands). This is NOT a new import. Do NOT switch to
            strconv.Itoa unless you also confirm fmt remains used elsewhere (it does — every Errorf)."
```

### Current Codebase Tree (the assumed on-disk state — S1–S6, T3.S1, T3.S2 landed; T3.S3 landing)

> **Parallel note:** T3.S3 is landing concurrently. The tree below reflects the state AFTER T3.S3
> lands (its imports/const/RecentMessages/CommitCount real; only RecentSubjects + AddAll remain as
> stubs). This subtask's edits are confined to the `RecentSubjects` stub region and are
> non-overlapping with T3.S3's edits.

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # imports AFTER T3.S3: bytes, context, errors, fmt, io, os/exec, strconv,
│       │                 #   strings. S1: interface+gitRunner+run()+runWithInput+New()+FileChange+
│       │                 #   StagedDiffOptions+stubs; S2: RevParseHEAD real; S3: WriteTree real;
│       │                 #   S4: CommitTree real; S5: ErrCASFailed+UpdateRefCAS real; S6: DiffTree
│       │                 #   real+parseDiffTree; T3.S1: StagedDiff real (consts/defaultExcludes);
│       │                 #   T3.S2: HasStagedChanges real; T3.S3: RecentMessages real (+strconv,
│       │                 #   +maxRecentMessageLines) + CommitCount real. REMAINING STUBS (after
│       │                 #   T3.S3): RecentSubjects, AddAll.
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic + initRepo + assertPanics.
│       │                 #   TestStubsPanic AFTER T3.S3 lists 2 stubs: RecentSubjects, AddAll
│       │                 #   (RecentMessages + CommitCount lines removed by T3.S3).
│       ├── revparse_test.go   # S2: minGitEnv + makeEmptyCommit + tests  (REUSED: makeEmptyCommit)
│       ├── writetree_test.go  # S3: makeMergeConflict + tests
│       ├── committree_test.go # S4: writeFile/stageFile + tests
│       ├── updateref_test.go  # S5: cas* + tests
│       ├── difftree_test.go   # S6: dt* + tests
│       ├── stagediff_test.go  # T3.S1: sd* + tests
│       ├── hasstaged_test.go  # T3.S2: hs* + tests
│       ├── commitcount_test.go     # T3.S3 (NEW): CommitCount tests
│       └── recentmessages_test.go  # T3.S3 (NEW): RecentMessages tests
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go                  # MODIFIED — RecentSubjects stub → real (NO import change,
        │                           #   NO new constant).
        ├── git_test.go             # MODIFIED — remove the RecentSubjects line from TestStubsPanic
        ├── revparse_test.go        # UNCHANGED (S2's file; makeEmptyCommit reused, not edited)
        ├── writetree_test.go       # UNCHANGED (S3's file)
        ├── committree_test.go      # UNCHANGED (S4's file)
        ├── updateref_test.go       # UNCHANGED (S5's file)
        ├── difftree_test.go        # UNCHANGED (S6's file)
        ├── stagediff_test.go       # UNCHANGED (T3.S1's file)
        ├── hasstaged_test.go       # UNCHANGED (T3.S2's file)
        ├── commitcount_test.go     # UNCHANGED (T3.S3's file)
        ├── recentmessages_test.go  # UNCHANGED (T3.S3's file)
        └── recentsubjects_test.go  # NEW — package git; 9 tests (no new helpers)
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | Replace the `RecentSubjects` panic-stub with the real `git log --format=%s -<n>` + `\n`-split body (keep signature). No import/constant change. |
| `internal/git/git_test.go` | MODIFY | Remove the `RecentSubjects` assertPanics line from `TestStubsPanic`. Nothing else. |
| `internal/git/recentsubjects_test.go` | CREATE | `package git` tests for `RecentSubjects`. Reuse `initRepo` + `makeEmptyCommit`. No new helpers. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants
/`defaultExcludes` (T3.S1), `HasStagedChanges` (T3.S2), `RecentMessages`/`maxRecentMessageLines`
(T3.S3), `CommitCount` (T3.S3), the remaining stub (`AddAll`), every other test file, `go.mod`/
`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the central design call: \n, NOT %x00): RecentSubjects uses git log --format=%s
// and splits on "\n". This is SAFE because git's %s placeholder is the SUBJECT (first line), which
// is single-line by definition and CANNOT contain a newline. FINDING 9's NUL delimiter (%x00)
// belongs to RecentMessages (T3.S3) which queries %B FULL BODIES — bodies can contain a markdown
// horizontal rule "---" that a naive split would fracture. Subjects cannot. Verified empirically:
// a subject "fix: handle --- edge" stays on ONE line under --format=%s. Do NOT cargo-cult %x00 here.

// CRITICAL (G2 — run()'s invariant, inherited from S1): run() returns err == nil for NON-ZERO git
// exits (128, 129, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O) set
// err != nil, with exitCode == -1. So the method MUST check `err != nil` FIRST, THEN branch on
// exitCode. Verified: git log on unborn → exit 128, err nil.

// CRITICAL (G3 — exit 128 is the "unborn" signal, NOT an error): `git log` on a zero-commit repo
// exits 128 (verified: "fatal: your current branch 'main' does not have any commits yet"). The
// contract says "return empty slice on unborn repo" → map 128 ⇒ (nil, nil). This mirrors RevParseHEAD
// S2's exit-128 ⇒ isUnborn and RecentMessages T3.S3's exit-128 ⇒ (nil, nil). On a new repo the
// duplicate check is vacuous (nothing to duplicate), so empty is the correct return.

// CRITICAL (G4 — non-repo is INDISTINGUISHABLE from unborn): a plain (non-git) directory ALSO makes
// `git log` exit 128 ("fatal: not a git repository"). So a non-repo returns (nil, nil) — identical
// to unborn. This is INHERITED from S2 (RevParseHEAD treats 128 as isUnborn too) and is ACCEPTABLE:
// callers gate on RevParseHEAD first, and a non-repo never reaches this method in the happy path.
// Do NOT add a special non-repo branch — there is no exit code that distinguishes it. (The NotARepo
// test asserts (nil,nil), NOT an error — see research §3/§7.)

// GOTCHA (G5 — NO new import, NO new constant): RecentSubjects uses only strings (Split/TrimSpace)
// and fmt (Sprintf/Errorf) — BOTH already imported (fmt since S1; strings since S2; strconv was added
// by T3.S3 but is NOT needed here). There is NO line cap (each subject is one line; n is bounded by
// the caller at 50 per FR31), so NO maxRecentSubjectLines constant. Adding either would be dead code.
// Do NOT touch the import block (T3.S3 owns it; touching it risks a merge conflict).

// GOTCHA (G6 — drop the trailing empty split element): `git log --format=%s -<n>` emits a trailing
// newline after the last record, so strings.Split(stdout, "\n") yields a final "" element. TrimSpace +
// `if s == "" { continue }` handles it, plus any genuinely-empty subject (git commit --allow-empty-
// message edge case).

// GOTCHA (G7 — n <= 0 defensive guard, D3): if n <= 0, return (nil, nil) WITHOUT calling git (avoids
// undefined `git log -0` behavior). Mirrors RecentMessages (T3.S3 D7). The caller passes 50 (FR31);
// the guard is cheap defensive coding. Place it FIRST (before the run() call).

// GOTCHA (G8 — reuse helpers, do NOT redeclare): initRepo (git_test.go) and makeEmptyCommit
// (revparse_test.go, S2) are `package git` and in scope for the new test file. REUSE them — do NOT
// redeclare (redeclaration is a compile error). makeEmptyCommit(t, dir, "feat: x\n\nbody") creates a
// commit whose SUBJECT is "feat: x" and BODY is "body" — exactly the fixture needed to prove %s
// returns ONLY the subject. For N-commit fixtures, call makeEmptyCommit in a loop (inline).

// GOTCHA (G9 — test file is package git, white-box): RecentSubjects is on *gitRunner and calls the
// unexported run(). To call New() and exercise the real method against temp repos, the test MUST be
// `package git` (NOT git_test). Matches every other test file in internal/git/.

// GOTCHA (G10 — no shell, no cmd.Dir in PRODUCTION code): the method inherits S1's §19 guarantees
// because it only calls run(). Do NOT introduce exec.Command / os.Chdir / sh -c in git.go. The test
// fixtures DO use exec.Command (the reused initRepo/makeEmptyCommit helpers use []string args +
// cmd.Env, never a shell) — acceptable test-fixture usage.

// GOTCHA (G11 — the TestStubsPanic edit): git_test.go's TestStubsPanic lists RecentSubjects + AddAll
// AFTER T3.S3 lands (T3.S3 removed the RecentMessages + CommitCount lines). Once RecentSubjects is
// real, its assertPanics line FAILS ("expected panic, but did not panic"). Resolution (mirrors
// S2/S3/S4/S5/S6/T3.S1/T3.S2/T3.S3): DELETE the RecentSubjects line. After removal, TestStubsPanic
// covers ONLY AddAll (T3.S5's remaining stub). This is a DISTINCT, non-overlapping assertPanics line
// from T3.S3's two removals.

// GOTCHA (G12 — parallel merge with T3.S3): T3.S3 edits git.go's import block, adds
// maxRecentMessageLines, and replaces the RecentMessages + CommitCount stubs; it removes the
// RecentMessages + CommitCount lines from TestStubsPanic. THIS subtask edits the RecentSubjects stub
// (a DIFFERENT region of git.go, far from imports and from the other two method bodies) and removes
// the RecentSubjects line from TestStubsPanic (a DIFFERENT line). The edits are non-overlapping; if
// applying as exact-text replacements, each oldText is unique. This subtask adds NO imports, so even
// the import block is untouched.
```

## Implementation Blueprint

### Data models and structure

None added or changed. The return type `([]string, error)` is already declared in the `Git`
interface by S1. **No new constant** (unlike T3.S3's `maxRecentMessageLines` — there is no line cap
here: each subject is one line and `n` is caller-bounded).

### The `RecentSubjects` body (exact — copy verbatim)

Replaces S1's panic-stub of the same name/signature. Place it where the stub currently is.

```go
// RecentSubjects returns up to n most-recent commit SUBJECTS (the first line of each commit message)
// for duplicate detection (PRD §9.7/FR31). The dedupe loop (P1.M3.T2) builds a set/map from these for
// O(1) exact-match lookup against a freshly-generated subject (FR32's "if the subject exactly matches
// one of the 50, retry"). It runs `git log --format=%s -<n>`, which emits EXACTLY ONE LINE per commit:
// git's %s placeholder is the subject (first line) by definition and CANNOT contain a newline, so the
// records are safely newline-delimited.
//
// NOTE — why a simple "\n" split is correct here (and NOT the %x00 NUL split that RecentMessages
// uses): FINDING 9's NUL delimiter exists to disambiguate %B FULL BODIES, where a commit body may
// contain a markdown horizontal rule "---" that a naive "---"/"\n" split would fracture. Subjects
// (%s) are single-line by construction — no embedded newline is possible, and a "---" inside a
// subject stays confined to its own line (it cannot start a new record). So splitting on "\n" is both
// safe and simpler. There is NO line cap (unlike RecentMessages): each subject is exactly one line
// and the caller bounds the count (PRD FR31: n=50), so the result is at most n short lines — no
// prompt-budget risk.
//
// On an unborn repo (zero commits) git log exits 128; RecentSubjects returns (nil, nil) defensively
// (callers gate on RevParseHEAD/CommitCount; on a new repo the duplicate check is vacuous — there is
// nothing to duplicate). Requesting more subjects than exist is NOT an error — git returns only what
// is available. It is read-only with respect to refs/index (PRD §18.1).
func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil // defensive guard (D3): caller passes 50 (PRD FR31); avoids undefined `git log -0`
	}
	stdout, stderr, code, err := g.run(ctx, g.workDir, "log", "--format=%s", fmt.Sprintf("-%d", n))
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // unborn repo (zero commits) — exit-code signal, NOT an error (matches RevParseHEAD S2 / RecentMessages T3.S3)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var subjects []string
	for _, line := range strings.Split(stdout, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue // trailing newline → trailing empty element; also any genuinely-empty subject
		}
		subjects = append(subjects, s)
	}
	return subjects, nil
}
```

> **Verified:** the format (`--format=%s`), the per-commit single-line output, the exit codes
> (unborn/non-repo=128, born=0), the n>commits and n<commits behaviors, and the `---`-in-subject
> survival are all confirmed in research §2/§3 (empirically verified against git 2.54.0).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (RecentSubjects body)
  - EDIT — replace the RecentSubjects panic-stub:
      FIND:
        func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
            panic("gitRunner.RecentSubjects: not yet implemented — see P1.M1.T3.S4")
        }
      REPLACE with: the doc comment + body from §"The RecentSubjects body" above. Keep the SAME
      signature `func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error)`.
  - DO NOT touch the import block (G5/G12 — T3.S3 owns it; strings + fmt already present). DO NOT add
    any constant. DO NOT touch run(), the interface, AddAll, or any other method/stub.
  - VERIFY: go build ./internal/git/ → exit 0 (no new imports ⇒ no unused-import risk either way).

Task 2: MODIFY internal/git/git_test.go (remove 1 line from TestStubsPanic)
  - FIND inside TestStubsPanic (and DELETE):
      assertPanics(t, "RecentSubjects", func() { _, _ = g.RecentSubjects(ctx, 5) })
  - After removal (and after T3.S3's concurrent removals) TestStubsPanic covers ONLY AddAll.
  - DO NOT touch initRepo, TestNew, TestRun_*, assertPanics helper, or the AddAll assertPanics line.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (AddAll still panics).

Task 3: CREATE internal/git/recentsubjects_test.go (package git — white-box)
  - FILE: internal/git/recentsubjects_test.go
  - PACKAGE line: `package git`  (NOT git_test — G9)
  - IMPORTS: context, errors, strings, testing  (all stdlib; errors for errors.Is, strings for
    strings.Contains in err assertions). NOTE: a loop building N commits needs an int→string for the
    message; use strconv.Itoa (import strconv) OR plain literals — strconv is already imported in
    git.go but NOT in this test file by default; import it if you use it. Choose one; keep gofmt-clean.
  - REUSE (do NOT redeclare): initRepo (git_test.go), makeEmptyCommit (revparse_test.go, S2).
  - WRITE the 9 test functions (assertions in §"Test cases" below). No new helpers (G8).
  - TEST MATRIX (9 functions): see §"Test cases — RecentSubjects".
  - VERIFY: go test -race -run TestRecentSubjects ./internal/git/ → exit 0, all 9 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go  (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                 (expect: no matches)
  - RUN: git grep -n 'panic.*RecentSubjects' internal/git/git.go               (expect: no matches)
  - RUN: git grep -n '%x00' internal/git/git.go | grep -i subject  (expect: no matches — proves no
         cargo-cult NUL in RecentSubjects; the only %x00 is in RecentMessages, T3.S3)
  - RUN: git status --porcelain → expect EXACTLY internal/git/git.go (modified) + internal/git/
         git_test.go (modified) + internal/git/recentsubjects_test.go (new). (Plus whatever T3.S3's
         concurrent changes produce — non-overlapping; T3.S2/T3.S3 are distinct regions/lines.)
```

### Test cases — RecentSubjects (`recentsubjects_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestRecentSubjects_UnbornRepo` | `initRepo` (0 commits) | `err==nil && len(subjects)==0` | exit 128 ⇒ empty (G3) |
| `TestRecentSubjects_ReturnsSubjects` | `initRepo` + 3× `makeEmptyCommit` | `len==3`; order newest-first; each has NO `"\n"` (single-line) | `%s` single-line guarantee |
| `TestRecentSubjects_NExceedsCommits` | 2 commits, call `RecentSubjects(ctx, 50)` | `len==2`, `err==nil` | git returns only what exists; FR31's n=50 works |
| `TestRecentSubjects_SubjectOnlyExcludesBody` | commit `feat: add x\n\nThis is the body.` | returned `len==1`; `subjects[0]=="feat: add x"` (NO body) | `%s` vs `%B` distinction |
| `TestRecentSubjects_MarkdownHRInSubject` | commit `fix: handle --- edge` | `len==1`; `subjects[0]` contains `---` (NOT split) | `\n` split is safe for `%s` (contrast FINDING 9) |
| `TestRecentSubjects_ZeroOrNegativeN` | born repo; call `(ctx, 0)` and `(ctx, -5)` | both `(nil, nil)`, `err==nil` | n<=0 defensive guard (D3/G7) |
| `TestRecentSubjects_NotARepo` | `t.TempDir()` w/o `initRepo` | `err==nil && len==0` | exit 128 ⇒ empty (inherited, G4) |
| `TestRecentSubjects_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found" | run()'s err path propagated (G2) |
| `TestRecentSubjects_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced (not exit code) |

```go
// Subject-only-excludes-body fixture (the %s vs %B proof):
func TestRecentSubjects_SubjectOnlyExcludesBody(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "feat: add x\n\nThis is the body. It must NOT appear in subjects.")
	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("len = %d, want 1", len(subjects))
	}
	if subjects[0] != "feat: add x" {
		t.Fatalf("subject = %q, want %q (the body must be excluded — %%s, not %%B)", subjects[0], "feat: add x")
	}
}

// Markdown-HR-in-subject fixture (the FINDING 9 CONTRAPOSITIVE proof):
func TestRecentSubjects_MarkdownHRInSubject(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "fix: handle --- edge case")
	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("len = %d, want 1 (the '---' must NOT split the subject under --format=%%s)", len(subjects))
	}
	if !strings.Contains(subjects[0], "---") {
		t.Fatalf("subject lost its '---': %q", subjects[0])
	}
	if !strings.Contains(subjects[0], "edge case") {
		t.Fatalf("subject lost its tail: %q", subjects[0])
	}
}

// N-exceeds-commits fixture (FR31's n=50 against a small repo):
func TestRecentSubjects_NExceedsCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "first")
	makeEmptyCommit(t, repo, "second")
	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 50) // FR31 default, > commit count
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subjects) != 2 {
		t.Fatalf("len = %d, want 2 (git returns only what exists)", len(subjects))
	}
}
```
> NOTE on `len==0` vs `nil`: an unborn repo returns `(nil, nil)`, so `len(subjects)==0` is the correct
> assertion (a nil slice has length 0). Do NOT assert `subjects != nil` — the contract returns nil
> (empty slice), and `len(nil)==0` is the idiomatic check.

### Implementation Patterns & Key Details

```go
// === Why "\n" and NOT "\x00" (THE design call, research §2) ===
// RecentMessages (T3.S3) queries %B (full body) and splits on "\x00" because a body may contain a
// markdown "---" that a "\n"/"---" split would fracture (FINDING 9). RecentSubjects queries %s
// (subject = first line), which is single-line by git's definition — CANNOT contain a newline. So a
// "\n" split is safe, simpler, and correct. Verified: a subject "fix: handle --- edge" stays on ONE
// line under --format=%s. Do NOT import the %x00 pattern here.

// === Why NO line cap (unlike RecentMessages) ===
// RecentMessages caps at 100 lines because %B bodies are multi-line and could blow the prompt budget.
// RecentSubjects has no such risk: each subject is exactly one line, and the caller bounds n (FR31:
// 50). At most 50 short (~50-char) lines — no budget risk, no cap needed, no constant to add.

// === Why err is checked BEFORE code (the branch order) ===
// run() guarantees: err != nil  ⟹  exitCode == -1  (LookPath / context / start failure).
//                   err == nil   for every real git exit (0, 128, 129, …).
// So `if err != nil { return nil, err }` is the authoritative infrastructural-failure guard; only then
// do the code branches run. Byte-identical to RevParseHEAD/RecentMessages/HasStagedChanges.

// === Why code == 128 ⇒ (nil, nil) (not error) ===
// 128 is git's "your current branch does not have any commits yet" exit — on an unborn repo git log
// exits 128. The contract says "return empty slice on unborn repo"; on a new repo the dedupe check is
// vacuous (nothing to duplicate). Mirrors RevParseHEAD S2's 128 ⇒ isUnborn. (Non-repo also exits 128 —
// G4, inherited; acceptable because callers gate on RevParseHEAD first.)

// === Why fmt.Sprintf("-%d", n) (not strconv.Itoa) ===
// Consistency with RecentMessages (T3.S3), which renders the count the same way. fmt is already
// imported (every Errorf uses it). This is NOT a new import. Either works; match the sibling.

// === Why TrimSpace AFTER split (not before) ===
// "\n" is the structural delimiter; per-record whitespace is noise WITHIN a record. Split first, then
// TrimSpace each element. The terminal trailing-newline yields a final "" element — dropped by the
// empty-skip check (G6).

// === Reusing initRepo + makeEmptyCommit ===
// Both are `package git` helpers in sibling test files, in scope for the new test file. Call them
// directly; do NOT redefine them (redeclaration = compile error). makeEmptyCommit(t, dir, "s\n\nb")
// creates a commit whose SUBJECT is "s" and BODY is "b" (verified: -m preserves embedded newlines) —
// exactly the fixture for the %s-vs-%B test, with no new helper.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, errors.Is, strings, fmt.Sprintf, t.Setenv (1.17+) all available
  - deps: ZERO new imports; go.mod unchanged (stdlib only)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go                  # MODIFIED: RecentSubjects body (real). NO import/const change.
  - file: internal/git/git_test.go             # MODIFIED: remove RecentSubjects line from TestStubsPanic
  - file: internal/git/recentsubjects_test.go  # NEW: package git, 9 tests, no new helpers

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M3.T2.S1 (duplicate rejection loop): the PRIMARY consumer. Calls RecentSubjects(ctx, 50) ONCE
    before the retry loop; builds `seen := map[string]struct{}{}` from the []string for O(1) exact-
    match lookup against each generated candidate subject (FR32). The set is reused across all retries.
  - P1.M3.T4 (CommitStaged orchestrator): wires the dedupe loop into the generate→parse→dedupe→commit
    pipeline.
  - P1.M4.T1.S2 (CLI default action): transitively uses RecentSubjects via the orchestrator.

PARALLEL-EXECUTION NOTE (with P1.M1.T3.S3 — RecentMessages+CommitCount, landing concurrently):
  - T3.S3 edits git.go in the import block (adds strconv), the maxRecentMessageLines const region, and
    the RecentMessages + CommitCount stub regions; it removes the RecentMessages + CommitCount lines
    from git_test.go's TestStubsPanic.
  - THIS subtask edits git.go in the RecentSubjects stub region (a DIFFERENT, non-adjacent method) and
    removes the RecentSubjects line from TestStubsPanic (a DIFFERENT assertPanics line).
  - These are DISTINCT, NON-OVERLAPPING regions (different method bodies; different assertPanics lines;
    only T3.S3 touches imports — this subtask adds none). Both can land in parallel without conflict.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each edit — fix before proceeding
gofmt -w internal/git/git.go internal/git/recentsubjects_test.go   # format the touched files
go vet ./internal/git/                                             # vet the package
go build ./...                                                     # whole-module compile

# Project-wide validation
make lint   # if the Makefile wires golangci-lint; else go vet ./...
gofmt -l internal/git/   # expect: empty output (no files need formatting)

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new method in isolation
go test -race -run TestRecentSubjects ./internal/git/ -v

# Confirm the stub-panic test still passes (now covers only AddAll)
go test -race -run TestStubsPanic ./internal/git/ -v

# Full internal/git suite (all prior S1–S6 / T3.S1 / T3.S2 / T3.S3 tests must stay green)
go test -race ./internal/git/ -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# No service to start (this is a library method). Validate end-to-end against a real git repo:
cd "$(mktemp -d)" && git init -q && git config user.email t@e.com && git config user.name T
git commit --allow-empty -m "feat: first" -q
git commit --allow-empty -m "fix: second" -q
git commit --allow-empty -m "docs: third with --- rule" -q
# Run the exact command the method runs, to confirm output shape:
git log --format=%s -50 | cat -A   # expect 3 lines, each ending in $, the 3rd containing '---' intact

# Cross-check exit-code semantics the method relies on:
git log --format=%s -50 >/dev/null 2>&1; echo "born EXIT=$?"          # expect 0
( cd "$(mktemp -d)" && git log --format=%s -50 >/dev/null 2>&1; echo "unborn EXIT=$?" )  # expect 128

# Expected: command output matches the method's parse model (one subject per line, trailing newline,
# '---' confined to its own line; exit 128 on unborn).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope-discipline grep (enforce FORBIDDEN-OPERATIONS boundaries):
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   # expect: no matches (no shell)
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                  # expect: no matches (no chdir)
git grep -n 'panic.*RecentSubjects' internal/git/git.go                # expect: no matches (stub gone)
# CRITICAL anti-cargo-cult check: RecentSubjects must NOT use %x00 (that is T3.S3's %B pattern):
git grep -n '%x00' internal/git/git.go | grep -i 'subject' || echo "OK: no %x00 in RecentSubjects"
# Confirm ONLY RecentMessages uses %x00:
git grep -n '%x00' internal/git/git.go   # expect: exactly one hit, in the RecentMessages body

# Confirm the touched-files set is exactly the three expected (plus any concurrent T3.S3 changes):
git status --porcelain internal/git/

# Expected: no shell, no chdir, no remaining panic, no %x00 in RecentSubjects; status shows exactly
# git.go + git_test.go (modified) + recentsubjects_test.go (new).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./internal/git/ -v` (9 new + all prior green)
- [ ] No vet errors: `go vet ./internal/git/`
- [ ] No formatting issues: `gofmt -l internal/git/` (empty)
- [ ] `go build ./...` exits 0

### Feature Validation

- [ ] All success criteria from "What" section met
- [ ] `RecentSubjects` returns `(nil, nil)` on unborn repo (exit 128)
- [ ] `RecentSubjects` returns `n` newest-first subjects on a born repo; `< n` (no error) when
      `n` exceeds commit count
- [ ] A multi-line commit yields ONLY its subject (first line) — body excluded (`%s`, not `%B`)
- [ ] A `---` inside a subject survives intact (proves `\n` split safe for `%s`)
- [ ] `n <= 0` returns `(nil, nil)` without calling git
- [ ] Missing git binary → "git binary not found"; cancelled ctx → `errors.Is(err, context.Canceled)`
- [ ] Integration points (P1.M3.T2 dedupe consumer) will be satisfiable by the returned `[]string`

### Code Quality Validation

- [ ] Follows existing codebase patterns (byte-identical branch order to RevParseHEAD/RecentMessages)
- [ ] File placement matches desired codebase tree (`internal/git/recentsubjects_test.go`)
- [ ] Anti-patterns avoided: NO `%x00` cargo-cult (G1), NO unnecessary line cap (D2), NO new imports
      (G5)
- [ ] NO `TestStubsPanic` regression (RecentSubjects line removed; AddAll still covered)
- [ ] Reused `initRepo` + `makeEmptyCommit` (no redeclaration — G8)

### Documentation & Deployment

- [ ] Method doc comment explains the `\n`-vs-`%x00` design call (so the next reader does not "fix" it)
- [ ] No new environment variables or config (pure library method)
- [ ] No logs added (library method; callers decide logging)

---

## Anti-Patterns to Avoid

- ❌ Don't cargo-cult T3.S3's `%x00`/NUL format — it exists for `%B` bodies (FINDING 9), NOT `%s`
      subjects. A `\n` split is correct and simpler here.
- ❌ Don't add a line cap / `maxRecentSubjectLines` constant — subjects are one line each and `n` is
      caller-bounded; a cap would be dead code defending an impossible input.
- ❌ Don't add a new import — `strings` + `fmt` are already present; touching the import block risks a
      parallel-merge conflict with T3.S3.
- ❌ Don't create new patterns when existing ones work — the branch order (err → 128 → !=0 → parse) is
      byte-identical to RevParseHEAD/RecentMessages for reviewer consistency.
- ❌ Don't skip the `---`-in-subject test — it is the empirical proof that the `\n` split is safe for
      `%s` and guards against a future "helpful" refactor to `%x00`.
- ❌ Don't ignore failing tests — fix the implementation, not the test.
- ❌ Don't catch all exceptions — branch on exit code (128 ⇒ empty; !=0 ⇒ wrapped error) as specified.
