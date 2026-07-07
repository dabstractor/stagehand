---
name: "P1.M1.T3.S3 — CommitCount & RecentMessages for style learning"
description: |
  Replace TWO panic-stubs in `internal/git/git.go` — `(*gitRunner).RecentMessages` and
  `(*gitRunner).CommitCount` (both landed as stubs by P1.M1.T2.S1) — with their real implementations.
  Together they feed the prompt builder's "style learning" path (PRD §9.3/FR10–FR12, §17.1): CommitCount
  decides mature-repo (>1 commit) vs new-repo (≤1 commit) prompt selection; RecentMessages supplies the
  up-to-20 full commit messages that become the style examples in the mature-repo system prompt
  (P1.M3.T1.S1). **CRITICAL (FINDING 9):** commit-pi used `git log --format='---%n%B' -20` and split on
  `---`, but `---` is a valid markdown horizontal rule that can appear inside a commit body — producing
  spurious splits and corrupting the style examples. This subtask uses the NUL-delimited format
  `git log --format='%x00%B' -<n>` and splits on `\x00`, which CANNOT occur in commit message text
  (git forbids NUL in object content). ⚠️ **INTERFACE-SIGNATURE NOTE (the central reconciliation):** the
  work-item description writes `RecentMessages(ctx, n int, maxLines int)`, but the `Git` interface is
  ALREADY landed by S1 as `RecentMessages(ctx context.Context, n int) ([]string, error)` and is treated
  as FIXED/immutable by every sibling subtask (S2, T3.S1, T3.S2 all protect it byte-for-byte). The PRD
  (FR11) fixes the line cap at 100 (NOT caller-configurable in v1), so the `maxLines` intent is
  implemented as an INTERNAL package constant `maxRecentMessageLines = 100` and the signature stays
  `(ctx, n int)`. CommitCount runs `git rev-list --count HEAD` (exit 128 on unborn ⇒ return `(0, nil)`
  per contract). RecentMessages runs `git log --format='%x00%B' -<n>`, splits on NUL, TrimSpaces each,
  drops empties, and caps TOTAL lines across returned messages at 100 (keeping COMPLETE messages only —
  partial style examples would mislead the model). Both methods delegate to S1's `run()` helper (NOT
  exec directly — mirrors every landed method). Adds ONE new import `strconv` (CommitCount parses the
  count via `strconv.Atoi`) — the ONLY import change in this subtask (distinct from T3.S2 which adds
  none). Adds TWO test files `internal/git/commitcount_test.go` and `internal/git/recentmessages_test.go`
  (`package git`, white-box) covering: unborn/zero, multi-commit counts, single-line & multi-line &
  markdown-`---`-bearing bodies, n-exceeds-commits, the 100-line cap, git-binary-missing, ctx-cancelled.
  Reuses `initRepo` (git_test.go) + `makeEmptyCommit` (revparse_test.go, S2) — no new helpers. Removes
  the TWO `RecentMessages` + `CommitCount` lines from `git_test.go`'s `TestStubsPanic` (required now
  that they are real — mirrors S2/S3/S4/S5/S6/T3.S1/T3.S2). Touches ONLY `internal/git/`; no interface,
  struct, `run()`, `runWithInput`, `FileChange`, `StagedDiffOptions`, RevParseHEAD, WriteTree,
  CommitTree, UpdateRefCAS, DiffTree, parseDiffTree, StagedDiff, HasStagedChanges, or the remaining 2
  method stubs (RecentSubjects, AddAll). This is the THIRD+FOURTH of the six P1.M1.T3 methods to leave
  stub-status (StagedDiff T3.S1, HasStagedChanges T3.S2 already landed; RecentSubjects T3.S4 and AddAll
  T3.S5 remain).
---

## Goal

**Feature Goal**: Implement the two git-log/recount read-only methods on `*gitRunner` that power the
prompt builder's **style learning** path (PRD §9.3/FR10–FR12, §17.1). `CommitCount` answers "is this a
mature repo (>1 commit) or a new repo (≤1 commit)?" — the branch point between the style-example
system prompt (§17.1) and the conventional-commit fallback prompt (§17.2). `RecentMessages` returns the
up-to-20 most-recent FULL commit messages that the prompt builder (P1.M3.T1.S1) interpolates as style
examples — delivered via a **NUL-delimited** git log format that eliminates the `---` markdown-collision
bug latent in commit-pi (FINDING 9), then capped at 100 total lines (PRD FR11).

**Deliverable**:
1. **MODIFY** `internal/git/git.go`:
   - Add `strconv` to the import block (gofmt-sorted between `os/exec` and `strings`).
   - Add the package constant `maxRecentMessageLines = 100` (co-located with the other commit-pi
     defaults near `defaultMaxMDLines`/`defaultMaxDiffBytes`).
   - Replace the `RecentMessages` panic-stub body with the real NUL-split + cap body.
   - Replace the `CommitCount` panic-stub body with the real `rev-list --count` + Atoi body.
2. **MODIFY** `internal/git/git_test.go`: remove the TWO lines from `TestStubsPanic` —
   `assertPanics(t, "RecentMessages", …)` and `assertPanics(t, "CommitCount", …)`.
3. **CREATE** `internal/git/commitcount_test.go` (`package git`): the 5-function test matrix.
4. **CREATE** `internal/git/recentmessages_test.go` (`package git`): the 9-function test matrix.

No other files touched. ONE new stdlib dependency (`strconv`) — `go.mod` unchanged (stdlib).

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 14 new test functions passing (plus all prior S1–S6 /
T3.S1 / T3.S2 tests green); `CommitCount` returns `(0, nil)` on an unborn repo, `(3, nil)` on a 3-commit
repo; `RecentMessages` returns `nil` on unborn, returns trimmed full messages (subject + body) on a
multi-line-commit repo, returns ≤ the requested `n` (never errors when `n` exceeds the commit count),
preserves a `---` markdown rule intact inside a single returned message (proving NUL safety), and caps
the total returned line count at ≤ 100; both methods surface a missing git binary as "git binary not
found" and a cancelled context as `errors.Is(err, context.Canceled)`; `run()`, `runWithInput`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4),
`UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants/
`defaultExcludes` (T3.S1), `HasStagedChanges` (T3.S2), and the remaining 2 stubs (`RecentSubjects`,
`AddAll`) are byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T1 prompt builder (the primary consumer) and — transitively — the
`CommitStaged` orchestrator (P1.M3.T4) and the CLI default action (P1.M4.T1.S2).

**Use Case**: After confirming the repo is born (via RevParseHEAD), the orchestrator asks
`count, err := g.CommitCount(ctx)`. If `count > 1` it calls `msgs, err := g.RecentMessages(ctx, 20)`
and hands the slice to the mature-repo prompt builder (§17.1) as the style-example block; if `count <= 1`
it uses the new-repo conventional-commit fallback prompt (§17.2) and does NOT call RecentMessages. The
multi-line-vs-single-line detection (PRD FR12 — does the history contain commits WITH bodies?) is done
by the prompt builder scanning the returned messages for `"\n"` past the subject; RecentMessages
supplies the raw full messages that make that scan possible.

**User Journey**: `g := git.New(repoPath)` → `count, err := g.CommitCount(ctx)` → if `err != nil`:
surface + abort; if `count <= 1`: fallback prompt (no RecentMessages call); if `count > 1`:
`msgs, err := g.RecentMessages(ctx, 20)` → `prompt := buildMaturePrompt(msgs)` → feed diff via stdin.

**Pain Points Addressed**: (1) Without NUL-delimiting, a commit body containing `---` would be split
into multiple fragments, corrupting the style examples and potentially leaking a `---` into the prompt
where it could be misread as a separator (the commit-pi latent bug, FINDING 9). (2) Without the 100-line
cap, a repo with huge multi-line commit messages could blow the prompt budget. (3) Without a typed
`CommitCount`, the prompt builder would re-derive "mature vs new" by checking message-slice emptiness —
ambiguous (a 1-commit repo has a non-empty message but is NOT "mature").

## Why

- **PRD §9.3 / FR10–FR12 (Prompt construction, P0 → G6):** *"FR10. Count commits (`git rev-list --count
  HEAD`). FR11. For repos with >1 commit: fetch the last 20 full commit messages … trimmed, capped at
  100 lines. FR12. Detect whether the history contains multi-line commits (subject + body) by scanning
  the examples."* CommitCount IS FR10; RecentMessages IS FR11 (the "last 20 full commit messages,
  trimmed, capped at 100 lines" delivery); the messages RecentMessages returns are what FR12 scans.
- **PRD §17.1 (The system prompt, mature repo):** the `---`-delimited style-example block in the
  prompt template is populated from RecentMessages' return. The NUL split (vs commit-pi's `---` split)
  is what makes this robust.
- **PRD §17.2 (new-repo fallback):** selected when CommitCount ≤ 1.
- **`critical_findings.md` FINDING 9:** the `git log --format='---%n%B'` delimiter collision — the
  single reason these methods exist as a NUL-delimited pair rather than a naive `---` split.
- **Foundation for P1.M3.T1.S1 (prompt construction):** the prompt builder is BLOCKED on both methods.
  CommitCount selects the prompt; RecentMessages supplies its examples.

## What

### CommitCount

Runs `git -C <workDir> rev-list --count HEAD` via `run()` and translates:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `(0, err)`. Infrastructural-failure guard.
- `code == 128` → return `(0, nil)`. Unborn repo (zero commits) OR non-repo dir (both exit 128 —
  inherited indistinguishability from S2's RevParseHEAD; see gotcha G4). The contract mandates
  "returns 0 on unborn"; callers gate on RevParseHEAD first, so 128 here is the defensive path.
- `code != 0` (any non-zero, non-128) → return
  `(0, fmt.Errorf("git rev-list --count HEAD: failed (exit %d): %s", code, strings.TrimSpace(stderr)))`.
- `code == 0` → parse `strconv.Atoi(strings.TrimSpace(stdout))`; on parse error return a wrapped error;
  on success return `(n, nil)`.

No porcelain, no shell, no `cmd.Dir`/`os.Chdir` (inherited from `run()`).

### RecentMessages

Runs `git -C <workDir> log --format=%x00%B -<n>` via `run()` and translates:

- `run()` returns `err != nil` → return `(nil, err)`. Infrastructural-failure guard.
- `n <= 0` (defensive guard, D7) → return `(nil, nil)` without calling git.
- `code == 128` → return `(nil, nil)`. Unborn repo (zero commits) — no messages to show. Defensive
  (callers gate on RevParseHEAD/CommitCount); matches the "empty ⇒ fallback" contract.
- `code != 0` (non-zero, non-128) → return `(nil, fmt.Errorf("git log: failed (exit %d): %s", …))`.
- `code == 0` → `strings.Split(stdout, "\x00")`; for each part: `TrimSpace`; if empty, skip; else
  accumulate line count and append, **stopping before total would exceed `maxRecentMessageLines` (100)**
  (D4 — keep complete messages only). Return the accumulated `[]string` (newest-first, git log order).

### Success Criteria

- [ ] `(*gitRunner).CommitCount` body matches §Implementation Blueprint (delegates to `run()`; exit
      128 ⇒ `(0, nil)`; else parses via `strconv.Atoi`).
- [ ] `(*gitRunner).RecentMessages` body matches §Blueprint (delegates to `run()`; `%x00%B` format;
      split on `"\x00"`; TrimSpace + drop empty; cap total lines at `maxRecentMessageLines`=100 keeping
      complete messages).
- [ ] `maxRecentMessageLines = 100` constant added (co-located with the other commit-pi defaults).
- [ ] `strconv` added to imports (gofmt-sorted: `os/exec`, `strconv`, `strings`); NO other import change.
- [ ] `CommitCount` returns `(0, nil)` on unborn, `(N, nil)` on an N-commit repo.
- [ ] `RecentMessages` returns `nil` on unborn, trimmed full messages on a multi-commit repo, and
      `len(result) <= n` (never errors when `n` exceeds commit count).
- [ ] A commit body containing `---` survives inside ONE returned message (NUL safety, FINDING 9).
- [ ] Total returned lines ≤ 100 even with many large multi-line commits.
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4),
      `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants/
      `defaultExcludes` (T3.S1), `HasStagedChanges` (T3.S2) byte-identical.
- [ ] The remaining 2 stubs (`RecentSubjects`, `AddAll`) untouched.
- [ ] `internal/git/commitcount_test.go` + `internal/git/recentmessages_test.go` exist in
      `package git` with the test matrices below.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `RecentMessages` or `CommitCount` lines.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean; `go test -race ./internal/git/`
      exits 0; `make test` exits 0.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path (`github.com/dustin/stagecoach`); the
exact four files to touch (and the exact two lines to remove from `git_test.go`); the exact `run()`
contract; the exact method bodies (empirically verified against git 2.54.0); the empirically-pinned
exit codes (unborn=128, non-repo=128, born=0); the interface-signature reconciliation (D1); the NUL-
format proof; the cap algorithm with a verified result (30×4-line → 25 kept, 100 lines); the exact
test matrices with reused helpers; and the exact validation commands. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.3/FR10–FR12 (Prompt construction: 'FR10. Count commits (git rev-list --count HEAD). FR11. For
        repos with >1 commit: fetch the last 20 full commit messages … trimmed, capped at 100 lines.
        FR12. Detect whether the history contains multi-line commits by scanning the examples.'); §17.1
        (the mature-repo prompt's style-example block — populated from RecentMessages); §17.2 (new-repo
        fallback, selected when CommitCount ≤ 1); §13/§11.1 (read-only methods — mutate no ref/object);
        §18.1 (the invariant: refs/index modified only at update-ref)."
  critical: "This subtask owns ONLY CommitCount + RecentMessages bodies + tests + the 2 TestStubsPanic
             line removals + the strconv import + the maxRecentMessageLines constant. Do NOT implement
             RecentSubjects (T3.S4), AddAll (P1.M1.T3.S5), the prompt builder (P1.M3.T1.S1), the
             orchestrator (P1.M3.T4), or the CLI. Do NOT change the Git interface or add a maxLines
             parameter (D1 — the cap is an internal constant)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 9 — THE spec for RecentMessages: 'commit-pi uses git log --format=\"---%n%B\" -20 and
        splits on ---, but --- is also a valid markdown horizontal rule. More robust: use a NUL byte
        delimiter: git log --format='%x00%B' -20 and split on \\x00. This cannot occur in commit message
        text.' This finding is the ENTIRE reason RecentMessages uses %x00 rather than ---."
  critical: "FINDING 9 is the single reason this method exists with a NUL split. Do NOT use the commit-pi
             --- format. Split on the Go string \"\\x00\" (a single NUL byte). Verified: a commit body
             containing '---' survives intact inside one split element (see research §2)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "The rev-list --count and 'git log --format=%B' sections: documents rev-list --count ('number of
        commits') and the %B placeholder ('raw body'). Atomic Commit Sequence context (these are
        read-only history queries invoked before generation)."
  critical: "rev-list --count HEAD exits 128 on unborn (verified) — same exit as rev-parse HEAD on
             unborn. The Go reference patterns there exec directly; we DELEGATE to run() (which exposes
             exitCode with err==nil). Do NOT copy the exec plumbing."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything these methods consume: the gitRunner struct; the run() helper (exact
        signature and verified body — NOT modified, only called); the RecentMessages + CommitCount
        panic-stubs being replaced; the Git interface (signatures already correct:
        RecentMessages(ctx, n int) ([]string, error) and CommitCount(ctx) (int, error)); New(); the
        git_test.go initRepo helper; and the assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero exit
             is not a Go error) is the foundation these methods' code branches rely on. The interface
             signatures are FIXED — do NOT add a maxLines parameter to RecentMessages (D1)."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The closest analog PRP — RevParseHEAD is also an exit-code-semantics method that delegates to
        run() and branches on code 128 for the 'unborn' signal. Its branch order (err-first, then
        code==128, then code!=0) and its test matrix (git-missing / ctx-cancelled via t.Setenv /
        cancel-before-call) are the templates for CommitCount's implementation and tests."
  critical: "RevParseHEAD branches on code==128 → isUnborn. CommitCount branches on code==128 → (0,nil).
             Same 128 semantic, different return shape. S2 ADDED the 'strings' import; this subtask
             ADDS 'strconv' (analogous precedent)."

- docfile: plan/001_f1f80943ac34/P1M1T3S1/PRP.md
  why: "The analog for a method that does POST-CAPTURE processing of git output (StagedDiff caps bytes/
        lines AFTER capturing). Its cap-sentinel approach and its test fixtures (loops creating many
        staged files) inform RecentMessages' line-cap test design. Also documents the commit-pi default
        constants block where maxRecentMessageLines is co-located."
  critical: "StagedDiff appends a '... [truncated]' sentinel because it is a DIFF (the model must know
             it is partial). RecentMessages does NOT append a sentinel — its outputs are STYLE EXAMPLES
             (a sentinel would pollute the prompt); it simply stops before exceeding 100 lines (D4)."

- docfile: plan/001_f1f80943ac34/P1M1T3S3/research/s3_research.md
  why: "THIS subtask's own research: the interface-signature conflict resolution (§1/D1); the
        empirically-pinned exit codes incl. the non-repo==128 indistinguishability (§2); the NUL-format
        xxd proof and the markdown '---' collision proof (§2); the cap algorithm with a verified result
        (§2/D4); the design decisions D1–D8 (§3); the test matrices (§4); and the non-overlap with
        T3.S2 (§5)."
  critical: "§1/D1 (interface FIXED, maxLines is a constant) and §2 (the %x00%B format proof + the cap
             result) are the two non-obvious anchors. §2 also documents that makeEmptyCommit preserves
             multi-line messages (no new helper needed)."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-log#_pretty_formats
  why: "Documents the %B placeholder ('raw body (unwrapped subject and body)') and the %x00 placeholder
        ('NUL byte') used in --format. Confirms %x00 emits a literal NUL and %B emits the full message."
  critical: "Establishes that --format='%x00%B' is valid and that %x00 is the delimiter. Do NOT use
             '%x00%n%B' or '%B%x00' — '%x00%B' places the NUL BEFORE each record (matches the split
             where the leading element is the empty pre-first-NUL string, dropped by TrimSpace)."
- url: https://git-scm.com/docs/git-rev-list#Documentation/git-rev-list.txt---count
  why: "Documents `rev-list --count` ('Limit the number of commits … --count prints … the number of
        commits'). Confirms it outputs a single integer."
  critical: "Establishes the exact command and its single-integer output. On unborn, it exits 128
             (verified) — parse only when exit 0."
- url: https://pkg.go.dev/strconv#Atoi
  why: "The canonical Go string→int parser used by CommitCount. Atoi returns (int, error) — the error
        path is wrapped into a 'git rev-list --count HEAD: unparseable output' error."
  critical: "Prefer Atoi over fmt.Sscanf for a simple integer parse (idiomatic; fmt.Sscanf is overkill
            and the %d scan has subtle whitespace rules). This is why `strconv` is the new import (D6)."
- url: https://pkg.go.dev/strings#Split
  why: "strings.Split(stdout, \"\\x00\") is the NUL splitter. The leading element (before the first NUL)
        is always \"\" — handled by the TrimSpace + empty-skip loop."
  critical: "The Go string literal \"\\x00\" is a SINGLE byte (NUL), NOT two characters. Do not write
            \"\\\\x00\" or split on the 4-char literal \"\\x00\"."
```

### Current Codebase Tree (the assumed on-disk state — S1–S6, T3.S1, T3.S2 landed)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # imports: bytes, context, errors, fmt, io, os/exec, strings (NO strconv yet).
│       │                 #   S1: interface+gitRunner+run()+runWithInput+New()+FileChange+StagedDiffOptions+stubs;
│       │                 #   S2: RevParseHEAD real (+strings); S3: WriteTree real; S4: CommitTree real (+io);
│       │                 #   S5: ErrCASFailed+UpdateRefCAS real; S6: DiffTree real+parseDiffTree;
│       │                 #   T3.S1: StagedDiff real (defaultMaxMDLines/defaultMaxDiffBytes/defaultExcludes);
│       │                 #   T3.S2: HasStagedChanges real. REMAINING STUBS: RecentMessages, RecentSubjects,
│       │                 #   CommitCount, AddAll.
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic + initRepo + assertPanics.
│       │                 #   TestStubsPanic currently lists 4 stubs: RecentMessages, RecentSubjects,
│       │                 #   CommitCount, AddAll (HasStagedChanges + StagedDiff lines already removed).
│       ├── revparse_test.go   # S2: minGitEnv + makeEmptyCommit + 4 tests  (REUSED: makeEmptyCommit)
│       ├── writetree_test.go  # S3: makeMergeConflict + 5 tests
│       ├── committree_test.go # S4: writeFile/stageFile/... + 6 tests
│       ├── updateref_test.go  # S5: cas* + 6 tests
│       ├── difftree_test.go   # S6: dt* + 9 tests
│       ├── stagediff_test.go  # T3.S1: sd* + tests
│       └── hasstaged_test.go  # T3.S2: hs* + tests
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go                 # MODIFIED — add strconv import; add maxRecentMessageLines const;
        │                          #   RecentMessages stub → real; CommitCount stub → real.
        ├── git_test.go            # MODIFIED — remove RecentMessages + CommitCount lines from TestStubsPanic
        ├── revparse_test.go       # UNCHANGED (S2's file; makeEmptyCommit reused, not edited)
        ├── writetree_test.go      # UNCHANGED (S3's file)
        ├── committree_test.go     # UNCHANGED (S4's file)
        ├── updateref_test.go      # UNCHANGED (S5's file)
        ├── difftree_test.go       # UNCHANGED (S6's file)
        ├── stagediff_test.go      # UNCHANGED (T3.S1's file)
        ├── hasstaged_test.go      # UNCHANGED (T3.S2's file)
        ├── commitcount_test.go    # NEW — package git; 5 tests (no new helpers)
        └── recentmessages_test.go # NEW — package git; 9 tests (no new helpers)
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | Add `strconv` import (gofmt-sorted); add `maxRecentMessageLines` const; replace the `RecentMessages` + `CommitCount` panic-stubs with real bodies (keep signatures). |
| `internal/git/git_test.go` | MODIFY | Remove the `RecentMessages` + `CommitCount` assertPanics lines from `TestStubsPanic`. Nothing else. |
| `internal/git/commitcount_test.go` | CREATE | `package git` tests for `CommitCount`. Reuse `initRepo` + `makeEmptyCommit`. No new helpers. |
| `internal/git/recentmessages_test.go` | CREATE | `package git` tests for `RecentMessages`. Reuse `initRepo` + `makeEmptyCommit`. No new helpers. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), `StagedDiff`/its constants
/`defaultExcludes` (T3.S1), `HasStagedChanges` (T3.S2), the remaining 2 stubs (`RecentSubjects`,
`AddAll`), every other test file, `go.mod`/`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/
other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the FINDING 9 delimiter): commit-pi split `git log --format='---%n%B'` on "---",
// but "---" is a valid markdown horizontal rule that can appear inside a commit body. Use the NUL-
// delimited format `git log --format='%x00%B' -<n>` and split on the Go string "\x00" (ONE NUL byte).
// NUL cannot occur in commit message text (git forbids it in object content). Verified: a body with
// "---" survives intact inside ONE split element. Do NOT split on the 4-char literal "\\x00".

// CRITICAL (G2 — run()'s invariant, inherited from S1): run() returns err == nil for NON-ZERO git
// exits (128, 129, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O) set
// err != nil, with exitCode == -1. So both methods MUST check `err != nil` FIRST, THEN branch on
// exitCode. Verified: rev-list --count on unborn → exit 128, err nil; git log on unborn → exit 128,
// err nil.

// CRITICAL (G3 — exit 128 is the "unborn" signal for BOTH methods, NOT an error): `git rev-list
// --count HEAD` and `git log` on a zero-commit repo BOTH exit 128 (verified). The contract says
// CommitCount "returns 0 on unborn" → map 128 ⇒ (0, nil). RecentMessages on unborn returns (nil, nil)
// (defensive — callers gate via RevParseHEAD/CommitCount, but the method is safe to call on unborn).
// This mirrors RevParseHEAD S2's exit-128 ⇒ isUnborn handling.

// CRITICAL (G4 — non-repo is INDISTINGUISHABLE from unborn): a plain (non-git) directory ALSO makes
// both `git rev-list --count HEAD` and `git log` exit 128 ("fatal: not a git repository"). So a non-
// repo returns (0, nil) from CommitCount and (nil, nil) from RecentMessages — identical to unborn.
// This is INHERITED from S2 (RevParseHEAD treats 128 as isUnborn too) and is ACCEPTABLE: callers
// gate on RevParseHEAD first, and a non-repo never reaches these methods in the happy path (it would
// fail at RevParseHEAD and be treated as a "new repo" → harmless fallback prompt). Do NOT add a
// special non-repo branch — there is no exit code that distinguishes it. (No NotARepo-returns-error
// test exists for this reason; see research §4.)

// CRITICAL (G5 — the interface-signature reconciliation, D1): the landed Git interface declares
// `RecentMessages(ctx context.Context, n int) ([]string, error)` — TWO params (ctx, n). The work-item
// description's `maxLines int` parameter is NOT in the interface and MUST NOT be added (it would break
// the interface, the stub, New, and TestStubsPanic). The PRD FR11 fixes the line cap at 100 (not
// caller-configurable in v1), so implement it as the internal constant `maxRecentMessageLines = 100`.
// The `n` param IS the count passed to `git log -<n>`.

// GOTCHA (G6 — ONE new import, strconv): CommitCount parses the count via strconv.Atoi (idiomatic;
// preferred over fmt.Sscanf). strconv is NOT currently in git.go's import block. Add it gofmt-sorted:
//   bytes, context, errors, fmt, io, os/exec, strconv, strings   ← strconv between os/exec and strings
// ("strconv" < "strings" alphabetically: compare char 7 'c' < 'i'). gofmt will sort it; place it
// manually to be safe. This is the ONLY import change in this subtask (T3.S2 adds none; S2 added
// strings — analogous precedent). Do NOT remove any existing import.

// GOTCHA (G7 — the cap keeps COMPLETE messages, D4): iterate the trimmed messages newest-first (git
// log order); accumulate `lines = strings.Count(msg, "\n") + 1`; STOP when the next message would push
// total > maxRecentMessageLines (100). Do NOT append a truncation sentinel (these are STYLE EXAMPLES,
// not a partial diff — a sentinel like "... [truncated]" would pollute the prompt). Verified: 30
// commits × 4 lines ⇒ 25 kept, exactly 100 lines.

// GOTCHA (G8 — drop the leading empty split element): `git log --format='%x00%B'` emits a NUL BEFORE
// the first record, so strings.Split(stdout, "\x00")[0] is always "" (the text before the first NUL).
// TrimSpace + `if msg == "" { continue }` handles it, plus any genuinely-empty commit messages.

// GOTCHA (G9 — reuse helpers, do NOT redeclare): initRepo (git_test.go) and makeEmptyCommit
// (revparse_test.go, S2) are `package git` and in scope for both new test files. REUSE them — do NOT
// redeclare (redeclaration is a compile error). makeEmptyCommit(t, dir, "subj\n\nbody") creates a
// MULTI-LINE commit (verified: -m preserves embedded newlines) — so multi-line fixtures need NO new
// helper. For N-commit fixtures, call makeEmptyCommit in a loop (inline).

// GOTCHA (G10 — test files are package git, white-box): CommitCount/RecentMessages are on *gitRunner
// and call the unexported run(). To call New() and exercise the real methods against temp repos, the
// tests MUST be `package git` (NOT git_test). Matches every other test file in internal/git/.

// GOTCHA (G11 — no shell, no cmd.Dir in PRODUCTION code): both methods inherit S1's §19 guarantees
// because they only call run(). Do NOT introduce exec.Command / os.Chdir / sh -c in git.go. The test
// fixtures DO use exec.Command (the reused initRepo/makeEmptyCommit helpers use []string args +
// cmd.Env, never a shell) — acceptable test-fixture usage.

// GOTCHA (G12 — the TestStubsPanic edit): git_test.go's TestStubsPanic currently lists 4 stubs
// (RecentMessages, RecentSubjects, CommitCount, AddAll). Once RecentMessages + CommitCount are real,
// their assertPanics lines FAIL ("expected panic, but did not panic"). Resolution (mirrors
// S2/S3/S4/S5/S6/T3.S1/T3.S2): DELETE both lines. After removal, TestStubsPanic covers the remaining
// 2 stubs (RecentSubjects, AddAll). These are 2 DISTINCT, non-overlapping assertPanics lines — also
// distinct from T3.S2's HasStagedChanges line (already removed).

// GOTCHA (G13 — n <= 0 defensive guard, D7): if n <= 0, return (nil, nil) WITHOUT calling git (avoids
// undefined `git log -0` behavior). The caller passes 20 (PRD FR11); the guard is cheap defensive
// coding. Place it AFTER the err check but the exact placement relative to the code==128 branch does
// not matter (n<=0 returns before any git call). Keep it minimal: `if n <= 0 { return nil, nil }`.
```

## Implementation Blueprint

### Data models and structure

None added or changed. Both return types (`int, error` and `[]string, error`) are already declared in
the `Git` interface by S1. ONE new package constant:

```go
const maxRecentMessageLines = 100 // PRD §9.3/FR11: ≤100 total lines across the style-example block
```

Co-locate it with the other commit-pi defaults (`defaultMaxMDLines`/`defaultMaxDiffBytes`), near the
`StagedDiff` region — it belongs to the "commit-pi defaults" family.

### The `CommitCount` body (exact — copy verbatim)

Replaces S1's panic-stub of the same name/signature. Place it where the stub currently is.

```go
// CommitCount returns the number of commits reachable from HEAD (PRD §9.3/FR10). It decides the
// mature-repo (>1 commit) vs new-repo (≤1 commit) prompt branch (PRD §17.1 vs §17.2). It runs
// `git rev-list --count HEAD`, which prints a single integer on success (exit 0) and exits 128 on an
// unborn repo (zero commits — the SAME exit-code signal RevParseHEAD S2 uses for isUnborn). On unborn
// it returns (0, nil) per contract; callers SHOULD but need not short-circuit via RevParseHEAD first
// (the method is safe to call on unborn). It is read-only with respect to refs/index (PRD §18.1).
//
// Note (FINDING-adjacent): a non-repo directory ALSO exits 128 ("fatal: not a git repository") and is
// therefore indistinguishable from unborn at this layer — inherited from RevParseHEAD's exit-128
// semantic and acceptable (callers gate on RevParseHEAD; a non-repo never reaches here in the happy
// path). Any other non-zero exit (not 0, not 128) is a real error.
func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return 0, nil // unborn repo (zero commits) — exit-code signal, NOT an error (matches RevParseHEAD S2)
	}
	if code != 0 {
		return 0, fmt.Errorf("git rev-list --count HEAD: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	n, perr := strconv.Atoi(strings.TrimSpace(stdout))
	if perr != nil {
		return 0, fmt.Errorf("git rev-list --count HEAD: unparseable output %q: %w", stdout, perr)
	}
	return n, nil
}
```

> **Verified:** the command (`rev-list --count HEAD`), the exit codes (unborn/non-repo=128,
> born=0), and the single-integer stdout are confirmed in research §2 (re-verified empirically against
> git 2.54.0).

### The `RecentMessages` body (exact — copy verbatim)

Replaces S1's panic-stub of the same name/signature.

```go
// RecentMessages returns up to n most-recent FULL commit messages (PRD §9.3/FR11, §17.1) for the
// mature-repo prompt builder's style-example block (P1.M3.T1.S1). It runs
// `git log --format=%x00%B -<n>`, which emits a NUL byte BEFORE each commit body — a delimiter that
// CANNOT collide with commit message text (FINDING 9: commit-pi's `---%n%B` split on `---` broke on
// markdown horizontal rules inside bodies; %x00 cannot occur in object content, verified). The output
// is split on "\x00", each part is trimmed, empties (including the leading pre-first-NUL element) are
// dropped, and the TOTAL line count is capped at maxRecentMessageLines (100, PRD FR11) keeping COMPLETE
// messages only (partial style examples would mislead the model — no truncation sentinel is appended).
// Git log returns newest-first, so the slice is newest-first. It is read-only (PRD §18.1).
//
// On an unborn repo (zero commits) git log exits 128; RecentMessages returns (nil, nil) defensively
// (callers gate on RevParseHEAD/CommitCount and take the new-repo fallback path when empty). Requesting
// more messages than exist is NOT an error — git returns only what is available.
func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil // defensive guard (D7): caller passes 20 (PRD FR11); avoids undefined `git log -0`
	}
	stdout, stderr, code, err := g.run(ctx, g.workDir, "log", "--format=%x00%B", fmt.Sprintf("-%d", n))
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // unborn repo — no messages; defensive (callers gate on RevParseHEAD/CommitCount)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var messages []string
	totalLines := 0
	for _, part := range strings.Split(stdout, "\x00") {
		msg := strings.TrimSpace(part)
		if msg == "" {
			continue // leading pre-first-NUL element, or a genuinely-empty message
		}
		lines := strings.Count(msg, "\n") + 1
		if totalLines+lines > maxRecentMessageLines {
			break // keep COMPLETE messages only; stop before exceeding the cap (D4)
		}
		messages = append(messages, msg)
		totalLines += lines
	}
	return messages, nil
}
```

> **Verified:** the format (`--format=%x00%B`), the split (leading empty element dropped), the cap
> (30×4-line ⇒ 25 kept, 100 lines), and the markdown-`---` preservation are confirmed in research §2.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (imports — add strconv)
  - EDIT the import block: add "strconv" gofmt-sorted between "os/exec" and "strings". Result:
      import (
          "bytes"
          "context"
          "errors"
          "fmt"
          "io"
          "os/exec"
          "strconv"
          "strings"
      )
  - DO NOT remove or reorder any other import. (gofmt will verify the sort.)
  - VERIFY: go build ./internal/git/ → exit 0 (strconv now usable; not yet used until Task 3).

Task 2: MODIFY internal/git/git.go (add the maxRecentMessageLines constant)
  - ADD near the existing commit-pi defaults block (defaultMaxMDLines / defaultMaxDiffBytes):
      const maxRecentMessageLines = 100 // PRD §9.3/FR11: ≤100 total lines across the style-example block
  - Co-locate with the other `const (...)` commit-pi block OR as a standalone const line — either is
    gofmt-clean; prefer the standalone line (it belongs to a different method family than the
    StagedDiff defaults).
  - DO NOT touch defaultExcludes / defaultMaxMDLines / defaultMaxDiffBytes.
  - VERIFY: go build ./internal/git/ → exit 0.

Task 3: MODIFY internal/git/git.go (CommitCount body)
  - EDIT — replace the CommitCount panic-stub:
      FIND:
        func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
            panic("gitRunner.CommitCount: not yet implemented — see P1.M1.T3.S3")
        }
      REPLACE with: the doc comment + body from §"The CommitCount body" above. Keep the SAME signature
      `func (g *gitRunner) CommitCount(ctx context.Context) (int, error)`.
  - DO NOT touch run(), the interface, or any other method/stub.
  - VERIFY: go build ./internal/git/ → exit 0 (strconv.Atoi now used → no unused-import error).

Task 4: MODIFY internal/git/git.go (RecentMessages body)
  - EDIT — replace the RecentMessages panic-stub:
      FIND:
        func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
            panic("gitRunner.RecentMessages: not yet implemented — see P1.M1.T3.S3")
        }
      REPLACE with: the doc comment + body from §"The RecentMessages body" above. Keep the SAME
      signature `func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error)`.
  - DO NOT touch the RecentSubjects or AddAll stubs.
  - VERIFY: go build ./... && go vet ./internal/git/ → exit 0.

Task 5: MODIFY internal/git/git_test.go (remove 2 lines from TestStubsPanic)
  - FIND inside TestStubsPanic (and DELETE both):
      assertPanics(t, "RecentMessages", func() { _, _ = g.RecentMessages(ctx, 5) })
      assertPanics(t, "CommitCount", func() { _, _ = g.CommitCount(ctx) })
  - After removal TestStubsPanic covers the remaining 2 stubs: RecentSubjects, AddAll.
  - DO NOT touch initRepo, TestNew, TestRun_*, assertPanics helper, or the other assertPanics lines.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (2 stubs still panic).

Task 6: CREATE internal/git/commitcount_test.go (package git — white-box)
  - FILE: internal/git/commitcount_test.go
  - PACKAGE line: `package git`  (NOT git_test — G10)
  - IMPORTS: context, errors, strings, testing  (all stdlib; errors for errors.Is, strings for
    strings.Contains in err assertions; NO os/os-exec/strconv needed in tests)
  - REUSE (do NOT redeclare): initRepo (git_test.go), makeEmptyCommit (revparse_test.go, S2).
  - WRITE the 5 test functions (assertions in §"Test cases" below). No new helpers (G9).
  - TEST MATRIX (5 functions): see §"Test cases — CommitCount".
  - VERIFY: go test -race -run TestCommitCount ./internal/git/ → exit 0, all 5 pass.

Task 7: CREATE internal/git/recentmessages_test.go (package git — white-box)
  - FILE: internal/git/recentmessages_test.go
  - PACKAGE line: `package git`  (NOT git_test — G10)
  - IMPORTS: context, errors, strings, testing  (all stdlib)
  - REUSE (do NOT redeclare): initRepo (git_test.go), makeEmptyCommit (revparse_test.go, S2).
  - WRITE the 9 test functions (assertions in §"Test cases" below). No new helpers (G9).
  - TEST MATRIX (9 functions): see §"Test cases — RecentMessages".
  - VERIFY: go test -race -run TestRecentMessages ./internal/git/ → exit 0, all 9 pass.

Task 8: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go  (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                 (expect: no matches)
  - RUN: git grep -n 'panic.*\(RecentMessages\|CommitCount\)' internal/git/git.go (expect: no matches)
  - RUN: git status --porcelain → expect EXACTLY internal/git/git.go (modified) + internal/git/
         git_test.go (modified) + internal/git/commitcount_test.go (new) + internal/git/
         recentmessages_test.go (new). (Plus whatever T3.S2's concurrent changes produce — non-
         overlapping; T3.S2 touches only HasStagedChanges body + its TestStubsPanic line +
         hasstaged_test.go, and adds NO import.)
```

### Test cases — CommitCount (`commitcount_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestCommitCount_UnbornRepo` | `initRepo` (0 commits) | `err==nil && count==0` | exit 128 ⇒ 0 (G3) |
| `TestCommitCount_ThreeCommits` | `initRepo` + 3× `makeEmptyCommit` | `err==nil && count==3` | parse of multi-digit count |
| `TestCommitCount_TenCommits` | `initRepo` + loop 10× `makeEmptyCommit` | `err==nil && count==10` | loop fixture + 2-digit count |
| `TestCommitCount_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found"; `count==0` | run()'s err path propagated (G2/G3) |
| `TestCommitCount_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `count==0` | ctx.Err() surfaced (not exit code) |

```go
// Fixture pattern (reuse makeEmptyCommit in a loop):
func TestCommitCount_TenCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	for i := 0; i < 10; i++ {
		makeEmptyCommit(t, repo, fmt.Sprintf("commit %d", i)) // fmt imported? use strconv.Itoa OR a plain literal
	}
	g := New(repo)
	count, err := g.CommitCount(context.Background())
	if err != nil { t.Fatalf("err = %v", err) }
	if count != 10 { t.Fatalf("count = %d, want 10", count) }
}
```
> NOTE: if `fmt` is not imported in this test file, build the loop message with `strconv.Itoa(i)` (then
> import strconv) OR use plain literals (`makeEmptyCommit(t, repo, "c")` x10 unrolled). SIMPLEST: import
> `strconv` in the test and use `strconv.Itoa(i)` — matches the production import. Choose one; keep it
> gofmt-clean.

### Test cases — RecentMessages (`recentmessages_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestRecentMessages_UnbornRepo` | `initRepo` (0 commits) | `err==nil && len(msgs)==0` | exit 128 ⇒ empty (G3) |
| `TestRecentMessages_SingleLine` | `initRepo` + 2 single-line commits | `len==2`; each `==` the stored subject; each has NO `"\n"` | single-line detection (FR12) |
| `TestRecentMessages_MultiLineBody` | commit `feat: x\n\nBody A.\nBody B.` | `msgs[0]` contains `"\n\n"` (body present); line count ≥ 3 | multi-line detection (FR12) |
| `TestRecentMessages_MarkdownHRCollision` | commit `docs: u\n\n---\n\nafter hr` | the returned message contains `"---"` AND has NOT been split (only ONE message for that commit; `len==1` if single commit) | NUL safety vs commit-pi `---` bug (FINDING 9, G1) |
| `TestRecentMessages_NExceedsCommits` | 2 commits, call `RecentMessages(ctx, 20)` | `len==2`, `err==nil` | git returns only what exists; no error |
| `TestRecentMessages_LineCap100` | loop 30× a 4-line commit; call `RecentMessages(ctx, 30)` | `totalLines <= 100`; `len(msgs) < 30` (some dropped); `len(msgs) >= 1` | the cap keeps complete messages (D4/G7) |
| `TestRecentMessages_NotARepo` | `t.TempDir()` w/o `initRepo` | `err==nil && len(msgs)==0` | exit 128 ⇒ empty (inherited, G4) |
| `TestRecentMessages_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found" | run()'s err path propagated |
| `TestRecentMessages_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced |

```go
// Line-cap helper (compute total lines across returned messages, for the assertion):
func rmTotalLines(msgs []string) int {
	total := 0
	for _, m := range msgs {
		total += strings.Count(m, "\n") + 1
	}
	return total
}

// Markdown-HR-collision fixture (the FINDING 9 proof):
func TestRecentMessages_MarkdownHRCollision(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "docs: update\n\n---\n\nThis text is after a horizontal rule")
	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 5)
	if err != nil { t.Fatalf("err = %v", err) }
	if len(msgs) != 1 { t.Fatalf("len = %d, want 1 (the --- must NOT split the message)", len(msgs)) }
	if !strings.Contains(msgs[0], "---") { t.Fatalf("message lost its '---': %q", msgs[0]) }
	if !strings.Contains(msgs[0], "after a horizontal rule") { t.Fatalf("body lost: %q", msgs[0]) }
}
```

### Implementation Patterns & Key Details

```go
// === Why %x00 (NUL) and NOT --- (FINDING 9) ===
// commit-pi split `git log --format='---%n%B'` on "---", but "---" is a valid markdown horizontal
// rule that can appear inside a commit body → spurious splits, corrupted style examples. The NUL
// byte (%x00) CANNOT occur in commit message text (git forbids NUL in object content), so splitting
// on "\x00" is collision-free. Verified: a body with "---" survives intact inside ONE element.

// === Why split and then TrimSpace (not TrimSpace-then-split) ===
// The NUL is the structural delimiter; the per-record whitespace (leading/trailing newlines git adds
// after %B) is noise WITHIN a record. So: split on "\x00" FIRST, THEN TrimSpace each element. The
// leading element (before the first NUL) is always "" — dropped by the empty-skip check.

// === Why err is checked BEFORE code (the branch order) ===
// run() guarantees: err != nil  ⟹  exitCode == -1  (LookPath / context / start failure).
//                   err == nil   for every real git exit (0, 128, 129, …).
// So `if err != nil { return …, err }` is the authoritative infrastructural-failure guard; only then
// do the code branches run. Matches every landed method (RevParseHEAD/WriteTree/…/HasStagedChanges).

// === Why code == 128 ⇒ empty/zero (not error) ===
// 128 is git's "HEAD does not resolve" exit — on an unborn repo (zero commits) BOTH rev-list --count
// and git log exit 128. The contract says CommitCount "returns 0 on unborn"; RecentMessages returns
// empty. This mirrors RevParseHEAD S2's 128 ⇒ isUnborn. (Non-repo also exits 128 — G4, inherited.)

// === Why the cap keeps complete messages (no sentinel) ===
// StagedDiff (T3.S1) appends "... [truncated]" because it delivers a DIFF the model must know is
// partial. RecentMessages delivers STYLE EXAMPLES — a truncation sentinel would itself become a
// "style example" and pollute the prompt. So it simply stops before exceeding 100 lines, silently
// dropping older messages. The 100 is a prompt-budget guard, not a fidelity signal.

// === Why strconv.Atoi (not fmt.Sscanf) ===
// Atoi is the idiomatic single-int parser; Sscanf's "%d" has subtle leading-whitespace semantics and
// is overkill for one integer. Atoi's error path is wrapped into a clear "unparseable output" error
// (defensive — git's rev-list --count output is always a clean integer, so this never fires in
// practice, but it makes the parse total rather than partial).

// === Reusing initRepo + makeEmptyCommit ===
// Both are `package git` helpers in sibling test files, in scope for the new test files. Call them
// directly; do NOT redefine them (redeclaration = compile error). makeEmptyCommit(t, dir, "s\n\nb")
// creates a MULTI-LINE commit (verified: -m preserves embedded newlines) — no new multi-line helper.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, errors.Is, strconv, strings, t.Setenv (1.17+) all available
  - deps: ONE stdlib import added (strconv); go.mod unchanged (stdlib only)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go                 # MODIFIED: +strconv import, +maxRecentMessageLines const,
    #                                          #   RecentMessages + CommitCount bodies (real)
  - file: internal/git/git_test.go            # MODIFIED: remove RecentMessages + CommitCount lines from TestStubsPanic
  - file: internal/git/commitcount_test.go    # NEW: package git, 5 tests, no new helpers
  - file: internal/git/recentmessages_test.go # NEW: package git, 9 tests, no new helpers

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M3.T1.S1 (mature-repo prompt builder): consumes BOTH — CommitCount selects the prompt branch;
    RecentMessages' []string becomes the style-example block (§17.1). FR12's multi-line detection scans
    these messages for "\n" past the subject.
  - P1.M3.T4 (CommitStaged orchestrator): calls CommitCount then (if >1) RecentMessages(ctx, 20).
  - P1.M4.T1.S2 (CLI default action): wires the orchestrator; transitively uses both.

PARALLEL-EXECUTION NOTE (with P1.M1.T3.S2 — HasStagedChanges, landing concurrently):
  - T3.S2 edits git.go in the HasStagedChanges region (a different, already-real method) and removes
    the HasStagedChanges line from git_test.go's TestStubsPanic.
  - THIS subtask edits git.go in the RecentMessages + CommitCount regions (different method stubs),
    adds the strconv import + maxRecentMessageLines const, and removes the RecentMessages + CommitCount
    lines from TestStubsPanic (different assertPanics lines).
  - These are DISTINCT, NON-OVERLAPPING regions (different method bodies; different assertPanics lines;
    only THIS subtask touches imports — T3.S2 adds none). Both can land in parallel without conflict.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...        # Expected: exit 0, no warnings (e.g. no unused import, no shadowing)
go build ./internal/git/         # Expected: exit 0 (strconv import added + used by CommitCount)
go build ./...                   # Expected: exit 0 (whole module compiles)

# Expected: zero output/errors. If `go build` says `undefined: strconv`, the import was not added —
# re-add it gofmt-sorted. If `go vet` says "imported and not used: strconv", CommitCount's Atoi was
# not written — complete Task 3. This subtask adds strconv and removes NO import.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v -run 'TestCommitCount' ./internal/git/      # Expected: 5 tests PASS, exit 0
go test -race -v -run 'TestRecentMessages' ./internal/git/   # Expected: 9 tests PASS, exit 0

go test -race -run 'TestStubsPanic' ./internal/git/   # Expected: exit 0 — 2 remaining stubs panic
go test -race ./internal/git/    # Expected: exit 0 — S1's run() tests, S2–S5's plumbing tests,
                                 # S6's DiffTree tests, T3.S1's StagedDiff tests, T3.S2's HasStagedChanges
                                 # tests, AND the 14 new CommitCount+RecentMessages tests all pass.

make test                        # Expected: exit 0 (Makefile target = go test -race ./...)
```

### Level 3: Security & Structural Invariants (the §19 enforcement + scope discipline)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution in the PRODUCTION git wrapper (inherited; new code adds none).
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output.

# No os.Chdir / cmd.Dir in PRODUCTION code (inherited; new code adds none).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output.

# The two stubs are gone (replaced by real bodies).
git grep -nE 'panic.*gitRunner\.(RecentMessages|CommitCount)' internal/git/git.go
# Expected: NO output.

# Only the intended files changed (plus T3.S2's concurrent non-overlapping changes, if any).
git status --porcelain
# Expected (this subtask's contribution):
#   M internal/git/git.go
#   M internal/git/git_test.go
#   ?? internal/git/commitcount_test.go
#   ?? internal/git/recentmessages_test.go

# go.mod / go.sum untouched (strconv is stdlib).
git diff --name-only go.mod go.sum
# Expected: NO output.
```

### Level 4: Runtime Smoke Test (prove both methods work against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the behaviors the tests assert, against the real binary (mirrors the research):
tmp=$(mktemp -d); git -C "$tmp" init -q

echo "--- CommitCount ---"
git -C "$tmp" rev-list --count HEAD; echo "unborn EXIT=$?"        # expect 128 (⇒ 0,nil)
git -C "$tmp" -c user.name=t -c user.email=t@t commit -q --allow-empty -m one
git -C "$tmp" -c user.name=t -c user.email=t@t commit -q --allow-empty -m two
git -C "$tmp" -c user.name=t -c user.email=t@t commit -q --allow-empty -m three
git -C "$tmp" rev-list --count HEAD; echo "three-commits EXIT=$?" # expect "3\n", exit 0

echo "--- RecentMessages (NUL format) ---"
git -C "$tmp" commit -q --allow-empty -m $'docs: x\n\n---\n\nbody after HR'
git -C "$tmp" log --format='%x00%B' -20 | cat -v | head           # NULs visible as ^@; --- intact

echo "--- Non-repo (both exit 128) ---"
tmp2=$(mktemp -d)
git -C "$tmp2" rev-list --count HEAD; echo "nonrepo count EXIT=$?"  # expect 128
git -C "$tmp2" log --format='%x00%B' -5;    echo "nonrepo log  EXIT=$?"  # expect 128
rm -rf "$tmp" "$tmp2"
# If the unborn/three-commit/non-repo exit codes differ, the box's git differs from 2.54.0 — re-pin
# the test assertions to the observed behavior and note the git version.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (strconv import added AND used by CommitCount.Atoi).
- [ ] `go test -race ./internal/git/` exits 0 (14 new tests + all prior tests pass).
- [ ] `make test` exits 0.

### Feature Validation

- [ ] `(*gitRunner).CommitCount` body matches §Blueprint (delegates to `run()`; exit 128 ⇒ `(0, nil)`;
      else `strconv.Atoi`).
- [ ] `(*gitRunner).RecentMessages` body matches §Blueprint (delegates to `run()`; `%x00%B`; split on
      `"\x00"`; TrimSpace + drop empty; cap total lines at 100 keeping complete messages; `n<=0` guard).
- [ ] `maxRecentMessageLines = 100` constant present.
- [ ] `CommitCount` returns `(0, nil)` on unborn, `(N, nil)` on an N-commit repo.
- [ ] `RecentMessages` returns `nil` on unborn, trimmed full messages on a multi-commit repo, and
      `len(result) <= n` (no error when `n` exceeds commit count).
- [ ] A commit body containing `---` survives inside ONE returned message (FINDING 9 NUL safety).
- [ ] Total returned lines ≤ 100 even with many large multi-line commits.
- [ ] On a missing git binary: both methods return an error mentioning "git binary not found".
- [ ] On a cancelled context: both methods return `errors.Is(err, context.Canceled)`.

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] NO `cmd.Dir` / `os.Chdir` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] `run()` is NOT re-implemented or modified (delegated to, unchanged).
- [ ] `Git` interface unchanged (NO maxLines parameter added — D1; cap is the internal constant).
- [ ] `gitRunner`, `New`, `FileChange`, `StagedDiffOptions` unchanged.
- [ ] `RevParseHEAD`/`WriteTree`/`CommitTree`/`UpdateRefCAS`/`DiffTree`/`parseDiffTree`/`StagedDiff`/
      `HasStagedChanges` and their constants/vars unchanged.
- [ ] The remaining 2 method stubs untouched (`RecentSubjects`, `AddAll` — still panic with their
      owning-subtask messages).
- [ ] `go.mod` / `go.sum` unchanged (strconv is stdlib; no new external deps).
- [ ] Only `internal/git/git.go` (modified), `internal/git/git_test.go` (modified — two lines removed),
      `internal/git/commitcount_test.go` (new), and `internal/git/recentmessages_test.go` (new) are
      changed by this subtask.
- [ ] No signal handling / `SysProcAttr` / process-group code added (that is P1.M4.T2).

### Documentation & Deployment

- [ ] Doc comment on `CommitCount` explains exit-128 ⇒ 0 (unborn) and the inherited non-repo
      indistinguishability, plus the read-only invariant.
- [ ] Doc comment on `RecentMessages` explains the FINDING 9 NUL rationale, the cap (complete messages
      only, no sentinel), and the exit-128 ⇒ empty defensive path.
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't use the commit-pi `---%n%B` format — `---` collides with markdown horizontal rules in commit
  bodies (FINDING 9). Use `%x00%B` and split on `"\x00"`.
- ❌ Don't split on the 4-char literal `"\\x00"` — split on the single-byte Go string `"\x00"` (NUL).
- ❌ Don't add a `maxLines int` parameter to `RecentMessages` — the interface is FIXED (D1); the cap is
  the internal `maxRecentMessageLines` constant.
- ❌ Don't treat exit 128 as an error — it is the "unborn repo" signal for both methods (⇒ 0 / empty).
- ❌ Don't append a truncation sentinel to the capped messages — they are style examples, not a diff;
  a sentinel would pollute the prompt. Stop before exceeding 100 lines.
- ❌ Don't use `fmt.Sscanf` to parse the count — `strconv.Atoi` is idiomatic.
- ❌ Don't redeclare `initRepo` / `makeEmptyCommit` in the new test files — reuse them (redeclaration is
  a compile error).
- ❌ Don't remove any existing import when adding `strconv` — gofmt-sorted insertion only.
- ❌ Don't skip the markdown-`---`-collision test — it is the empirical proof of FINDING 9.
