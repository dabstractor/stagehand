---
name: "P1.M1.T3.S5 — AddAll and StagedFileCount (auto-stage-all primitives)"
description: |
  Close out the `internal/git` git-wrapper by implementing the FINAL two methods in `internal/git/git.go`:
  `AddAll(ctx) error` (the last remaining panic-stub, landed by P1.M1.T2.S1) and `StagedFileCount(ctx) (int,
  error)` (a NEW interface method — see the central design call below). Together they power the PRD §9.4
  auto-stage-all path (FINDING 11): when nothing is staged and `auto_stage_all` is enabled, the CLI layer
  (P1.M4.T1.S2) calls `AddAll` then `StagedFileCount` to produce the FR18 transparent notice
  "Nothing staged — staging all changes (N files)." `--all`/`-a` (FR20) also routes through `AddAll`.
  ⚠️ **THE central design call — StagedFileCount is NOT in the `Git` interface today.** The interface
  (S1's contract) declares only `AddAll(ctx) error` as the S5 method. The work-item CONTRACT mandates a
  SECOND method, `StagedFileCount(ctx) (int, error)`, for the FR18 count. This subtask therefore ADDS
  `StagedFileCount` to the `Git` interface (one new doc-commented method, appended after `AddAll`; no
  existing signature touched) AND implements its body. This is the single most important structural
  addition — without it the FR18 "N files" notice is impossible.
  ⚠️ **THE second design call — StagedFileCount does NOT invert exit codes (distinct from its sibling
  HasStagedChanges).** `HasStagedChanges` (T3.S2) runs `git diff --cached --quiet`, where `--quiet` makes
  exit 1 mean "staged" (FINDING 6 inversion). `StagedFileCount` runs `git diff --cached --name-only`
  (NO `--quiet`): WITHOUT `--quiet`, `git diff` exits 0 whether or not changes exist, and `--name-only`
  emits the file LIST (one path per line) that we count. Adding `--quiet` here would SUPPRESS the output
  and BREAK counting — so `--quiet` is deliberately OMITTED. StagedFileCount's branch order is the SIMPLE
  mutation/read form (`code != 0 → error`), byte-identical to `StagedDiff`/`DiffTree`, NOT
  HasStagedChanges' `switch code {0/1/default}` form. This mirrors S4's `%s`-vs-`%x00` lesson: the
  close sibling looks deceptively similar, and the difference (here: omitting `--quiet`) is load-bearing.
  ⚠️ **THE third design call — AddAll is a MUTATION with NO 128 special-case.** `git add -A` exits 0 on
  every happy path (born/unborn, with/without changes); only a non-repo/corrupt repo makes it non-zero
  (exit 128). So `AddAll`'s branch order is the SIMPLE mutation form (`code != 0 → error`), identical to
  `WriteTree`/`CommitTree` — NOT the read-method form (`RevParseHEAD`/`RecentMessages`) that treats
  exit 128 as "unborn is not an error". add -A never needs that, because add -A never exits 128 in a repo.
  ⚠️ **Stub-guard CLOSURE (D6).** `AddAll` is the FINAL panic-stub. Removing its `assertPanics` line from
  `git_test.go`'s `TestStubsPanic` leaves `ctx` and `g` as UNUSED locals → Go compile error. Grep confirms
  `assertPanics` is used ONLY in `TestStubsPanic` (git_test.go:104 def, :123 sole call). Therefore this
  subtask REMOVES the entire `TestStubsPanic` function AND the `assertPanics` helper — the clean closure
  of S1's "panic to fail fast" scaffolding, now that zero stubs remain. This is a slightly larger
  `git_test.go` edit than prior subtasks but is the logically correct end state.
  Delegates to S1's `run()` helper (NOT exec directly — mirrors every landed method). Both methods use
  ONLY `strings` + `fmt` (already imported); NO new import, NO new constant. Adds TWO test files
  (`addall_test.go`, `stagedcount_test.go`, both `package git`, white-box) reusing `initRepo`
  (git_test.go) + `writeFile`/`stageFile` (committree_test.go, S4) + `makeEmptyCommit` (revparse_test.go,
  S2). Touches ONLY `internal/git/`; no `run()`/`runWithInput`/`New`/`gitRunner`/`FileChange`/
  `StagedDiffOptions`/any prior method/any other test file/`go.mod`/`Makefile` change. Mock: real git
  binary in temp repos. This is the SIXTH and FINAL P1.M1.T3 method pair — completes the foundation git
  wrapper (consumed by P1.M3.T4 orchestrator, P1.M3.T2 dedupe is already fed by S4, and P1.M4.T1.S2 CLI
  default action).
---

## Goal

**Feature Goal**: Implement the two git-wrapper methods that close out `internal/git` and power the PRD
§9.4 nothing-staged / auto-stage-all path (FR16/FR18/FR20, FINDING 11). `AddAll` runs `git add -A`
(stages modified + untracked + deleted across the whole worktree). `StagedFileCount` runs
`git diff --cached --name-only` and returns the count of staged files — the `N` in the FR18 notice
"Nothing staged — staging all changes (N files)." The CLI layer (P1.M4.T1.S2) and `--all`/`-a` (FR20)
consume them; the auto-stage logic (P1.M3.T2.S2 / FINDING 11) orchestrates AddAll → re-check → notice.

**Deliverable**:
1. **MODIFY** `internal/git/git.go` —
   (a) **ADD** `StagedFileCount(ctx context.Context) (int, error)` to the `Git` interface (doc comment +
       one-line signature, appended AFTER the existing `AddAll` interface line; the FIRST new interface
       member since S1).
   (b) **UPDATE** the interface ownership comment block so the S5 line lists BOTH methods.
   (c) **REPLACE** the `(*gitRunner).AddAll` panic-stub body with the real `git add -A` body (keep the
       signature `func (g *gitRunner) AddAll(ctx context.Context) error` byte-for-byte).
   (d) **ADD** the `(*gitRunner).StagedFileCount` method body (new method, placed after `CommitCount`,
       before the existing `AddAll` stub location — or adjacent to it; see §Placement).
   No import change. No new constant.
2. **MODIFY** `internal/git/git_test.go` — REMOVE the entire `TestStubsPanic` function AND the
   `assertPanics` helper (D6: AddAll is the final stub; leaving the assertPanics line → unused-local
   compile error). `initRepo` / `TestNew` / `TestRun_*` untouched.
3. **CREATE** `internal/git/addall_test.go` (`package git`): the 7-function AddAll test matrix.
4. **CREATE** `internal/git/stagedcount_test.go` (`package git`): the 9-function StagedFileCount matrix.

No other files touched. **Zero new dependencies** (uses `strings` + `fmt`, both already imported).
`go.mod` unchanged (stdlib only).

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 16 new test functions passing (plus all prior tests
green); `AddAll` stages modified+untracked+deleted files and exits clean on a clean tree / unborn repo;
`StagedFileCount` returns the correct count of staged files (0 on clean tree, N after staging, includes
deletions, keeps a space-in-name as one entry); both surface a missing git binary as "git binary not
found" and a cancelled context as `errors.Is(err, context.Canceled)`; `run()`, `runWithInput`, `New`,
`gitRunner`, `FileChange`, `StagedDiffOptions`, and every prior method (RevParseHEAD…CommitCount) are
byte-identical to their landed forms; the `Git` interface gains exactly one new method
(`StagedFileCount`); `TestStubsPanic` and `assertPanics` are gone (zero stubs remain).

## User Persona

**Target User**: The CLI default-action (P1.M4.T1.S2) and the auto-stage-all path (FINDING 11 /
P1.M3.T2.S2) — and, transitively, US11 ("As a plan-holder with nothing staged, I want `stagecoach` to
stage all changes and commit them in one message").

**Use Case**: The orchestrator/CLI checks `HasStagedChanges` (T3.S2). If nothing is staged and
`auto_stage_all` (default true) is on (FR16/FR19), it calls `g.AddAll(ctx)` (this subtask), then
`g.StagedFileCount(ctx)` (this subtask) to read the `N` for the FR18 notice `Nothing staged — staging all
changes (N files).`, then re-checks `HasStagedChanges` (FR17: if still nothing → "nothing to commit",
exit 2). `--all`/`-a` (FR20) force-routes through `AddAll` even when something is already staged.

**User Journey**: `g := git.New(repoPath)` → `if staged, _ := g.HasStagedChanges(ctx); !staged && autoStage
{ n, err := g.StagedFileCount(ctx); /* err guard */; fmt.Fprintf(os.Stderr, "Nothing staged — staging all
changes (%d files).\n", n); if err := g.AddAll(ctx); err != nil { /* surface */ }; /* re-check */ }`.

**Pain Points Addressed**: (1) Without a typed `AddAll`, the CLI would inline `exec.Command("git",
"add", "-A")` — duplicating the `-C`/LookPath/buffer/error plumbing that `run()` already centralizes
(and breaking the "only one place shells out" boundary, PRD §19). (2) Without `StagedFileCount`, the
notice would either hard-code a wrong count or re-parse `git diff` output inline in the UI layer —
leaking git internals into presentation code. A typed `(int, error)` keeps the boundary clean.

## Why

- **PRD §9.4 / FR16 (Nothing-staged / auto-stage-all, P0 → G5):** *"If `git diff --cached --quiet`
  reports no staged changes: if `auto_stage_all` is enabled (default: true), run `git add -A`, then
  re-check."* — `AddAll` IS the `git add -A` step.
- **PRD §9.4 / FR18:** *"Print a transparent notice when auto-staging occurs, e.g. `Nothing staged —
  staging all changes (3 files).`"* — `StagedFileCount` returns the `3` (the file count after add).
- **PRD §9.4 / FR20:** *"Provide `--all` / `-a` to force `git add -A` even when something is already
  staged."* — `--all` calls `AddAll` unconditionally.
- **PRD §9.4 / FR17:** *"If after auto-stage there are still no changes (clean working tree), exit with a
  friendly 'nothing to commit' message and exit code 2."* — the re-check is `HasStagedChanges` (T3.S2),
  but `AddAll` must run first (and be correct) for FR17 to be reachable.
- **PRD §18.1 (the invariant):** the index is mutated by `AddAll` (`git add -A` writes the index). This
  is an EXPECTED, pre-commit, non-ref mutation — it stages, it does NOT move HEAD (only `UpdateRefCAS`
  moves a ref). `AddAll` is upstream of the immutable snapshot (`WriteTree`/`CommitTree`), so it does not
  threaten the "snapshot then CAS" atomicity.
- **Foundation closure:** `AddAll` is the FINAL panic-stub in the git wrapper. Implementing it + adding
  `StagedFileCount` closes P1.M1.T3 and unblocks P1.M3.T4 (orchestrator) and P1.M4.T1.S2 (CLI default
  action) — both of which call the auto-stage path.

## What

### AddAll

Runs `git -C <workDir> add -A` via `run()`. Branches:
- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `err`. Infrastructural-failure guard.
- `code != 0` → return `fmt.Errorf("git add -A: failed (exit %d): %s", code, strings.TrimSpace(stderr))`.
  (Non-repo → exit 128; corrupt repo / permission → non-zero. ALL are real errors.)
- `code == 0` → return `nil`. (Staged everything — or a clean-tree no-op, both exit 0.)

**No 128 special-case** (D2/central design call #3): `add -A` is a mutation; it treats ALL non-zero
exits as errors, like `WriteTree`/`CommitTree`. It is NOT a read method that special-cases 128 as
"unborn is fine".

### StagedFileCount

Runs `git -C <workDir> diff --cached --name-only` via `run()` (NO `--quiet`). Branches:
- `run()` returns `err != nil` → return `(0, err)`. Infrastructural-failure guard.
- `code != 0` → return `(0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code,
  strings.TrimSpace(stderr)))`. (Non-repo → exit **129**, see D5; corrupt repo → non-zero. ALL real
  errors.)
- `code == 0` → `strings.Split(stdout, "\n")`; for each line, `if strings.TrimSpace(line) != "" { count++
  }`; return `(count, nil)`. (Trailing newline → trailing `""` element → dropped; empty output → count 0.)

**NO `--quiet`** (central design call #2): `--name-only` produces the file LIST we count; `--quiet`
would suppress it. Without `--quiet`, exit is 0 whether or not changes exist — so the branch order is the
simple `code != 0 → error` form, NOT `HasStagedChanges`' inversion form.

### Success Criteria

- [ ] `(*gitRunner).AddAll` body matches §Implementation Blueprint (delegates to `run()`; args
      `["add","-A"]`; SIMPLE branch order err → !=0 → nil; NO 128 special-case).
- [ ] `(*gitRunner).StagedFileCount` body matches §Implementation Blueprint (delegates to `run()`; args
      `["diff","--cached","--name-only"]`; NO `--quiet`; split on `"\n"`; count non-empty-TrimSpace lines).
- [ ] The `Git` interface gains exactly ONE new method: `StagedFileCount(ctx context.Context) (int, error)`
      with a doc comment. No existing signature changed.
- [ ] The interface ownership comment block lists BOTH `AddAll` and `StagedFileCount` under P1.M1.T3.S5.
- [ ] NO new import added; NO new constant added.
- [ ] `AddAll` stages modified + untracked + deleted files; exits clean (nil) on a clean tree and on an
      unborn repo; returns a wrapped error on a non-repo.
- [ ] `StagedFileCount` returns 0 on a clean tree, N after staging N files, includes deletions in the
      count, counts a space-in-name file as ONE entry.
- [ ] `TestStubsPanic` and `assertPanics` are REMOVED from `git_test.go` (zero stubs remain; no
      unused-local compile error).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, `FileChange`, `StagedDiffOptions`, and every prior
      method (RevParseHEAD S2 … CommitCount T3.S3) byte-identical.
- [ ] `internal/git/addall_test.go` (7 tests) + `internal/git/stagedcount_test.go` (9 tests) exist in
      `package git` and pass.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean; `go test -race ./internal/git/`
      exits 0; `make test` exits 0.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path (`github.com/dustin/stagecoach`); the exact
four files to touch; the exact `run()` contract; BOTH exact method bodies (empirically verified against
real git); the empirically-pinned exit codes (add -A: 0 on happy / 128 on non-repo; diff --cached
--name-only: 0 whether-or-not-staged / 129 on non-repo); the three central design calls (interface
addition; omit `--quiet`; no 128 special-case for the mutation); the exact stub-guard-closure edit; the
16-test matrix with reused helpers; and the exact validation commands. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.4/FR16 (auto-stage-all: 'run git add -A, then re-check'); §9.4/FR17 (still-nothing → exit 2);
        §9.4/FR18 (the notice 'Nothing staged — staging all changes (N files).' — N = StagedFileCount's
        return); §9.4/FR19 (--no-auto-stage disables); §9.4/FR20 (--all/-a force-add); §18.1 (the
        invariant: AddAll mutates the INDEX, not a ref — only UpdateRefCAS moves HEAD); §19 (real git
        binary via run(), no shell); US11 (stage-all-and-commit v1 behavior)."
  critical: "This subtask owns ONLY AddAll + StagedFileCount (+ the interface addition) + the stub-guard
             closure + the two test files. Do NOT implement the auto-stage orchestration (P1.M3.T2.S2 /
             P1.M4.T1.S2), the notice string formatting (that's the UI layer P1.M4.T3), the re-check
             (HasStagedChanges is T3.S2, already landed), or the exit-2 path. Do NOT change any existing
             Git interface signature (only ADD StagedFileCount)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 11 (Auto-stage-all default and the nothing-to-commit path: add -A → re-check → FR18
        notice → FR17 exit 2). FINDING 6 (the --quiet exit-code inversion — CRITICAL to read correctly:
        it is a property of --quiet ALONE; StagedFileCount OMITS --quiet, so it does NOT invert — the
        contrast is the load-bearing insight). FINDING 1 (exit 128 = unborn/non-repo signal for READ
        methods — N/A to AddAll, a mutation)."
  critical: "FINDING 6 is THE trap: HasStagedChanges (T3.S2) uses --quiet and switches on exit 1.
             StagedFileCount uses --name-only (no --quiet) and must NOT replicate the switch — it uses the
             simple code!=0→error form. Cargo-culting HasStagedChanges' --quiet/switch here would SUPPRESS
             the file list and break counting. Read FINDING 6, then apply the CONTRAPOSITIVE: omitting
             --quiet means exit 0 always (on success), so there is nothing to switch on."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§5 ('git diff --cached'): the --quiet exit-code table (0=nothing staged, 1=staged, >1=error) and
        the pathspec magic-prefix syntax. Confirms --name-only emits 'only names of changed files' (one
        path per line) and that --quiet 'Disable all output of the program' (the reason we OMIT it). The
        commit-tree/write-tree sections (§3/§4) for the mutation branch-order pattern AddAll mirrors."
  critical: "The Go reference patterns there exec directly; we DELEGATE to run() (which exposes exitCode
             with err==nil for non-zero exits). Do NOT copy the exec plumbing. §5's hasStagedChanges
             example uses --quiet + switch — that is HasStagedChanges' pattern, NOT StagedFileCount's."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything both methods consume: the gitRunner struct; the run() helper (exact
        signature and verified body — NOT modified, only called); the AddAll panic-stub being replaced;
        the Git interface (into which we ADD StagedFileCount); New(); the git_test.go initRepo helper; and
        the assertPanics helper (which we REMOVE here)."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero exit is
             not a Go error) is the foundation both methods' code branches rely on. The interface is
             FROZEN except for the ONE method we append (StagedFileCount)."

- docfile: plan/001_f1f80943ac34/P1M1T3S2/PRP.md
  why: "The CLOSEST sibling — HasStagedChanges runs `git diff --cached --quiet` and switches on exit 1.
        Its test matrix (git-missing / ctx-cancelled / non-repo-via-non-zero / nothing-staged / staged) is
        the template. The CENTRAL CONTRAST: HasStagedChanges NEEDS --quiet (answer in exit code);
        StagedFileCount MUST NOT use --quiet (answer in stdout line count). Read this PRP to understand
        what NOT to copy."
  critical: "Do NOT replicate T3.S2's `switch code { case 0: false; case 1: true; default: error }` in
             StagedFileCount. StagedFileCount uses `if code != 0 { error }` then counts stdout lines.
             Both use `git diff --cached` but the OPTION SET determines the exit semantics — that is the
             entire point."

- docfile: plan/001_f1f80943ac34/P1M1T3S4/PRP.md
  why: "The previous subtask (RecentSubjects). Its house STYLE (gotchas G1–Gn, decisions D1–Dn, exact
        code body, scope-discipline greps, parallel-execution notes, reused-helpers test matrix) is the
        template this PRP follows. Its central `%s`-vs-`%x00` design call is the conceptual sibling of
        this subtask's `--name-only`-vs-`--quiet` call: a close sibling's option choice is load-bearing."
  critical: "S4 also demonstrates the EXACT pattern for removing an assertPanics line from TestStubsPanic.
             THIS subtask goes ONE step further: because AddAll is the FINAL stub, it removes TestStubsPanic
             AND assertPanics entirely (D6). Read S4's removal to see the prior pattern, then apply the
             closure here."

- docfile: plan/001_f1f80943ac34/P1M1T3S5/research/s5_research.md
  why: "THIS subtask's own research: the interface-absence finding + the addition decision (§0); the
        empirical add -A exit table (§1); the empirical diff --cached --name-only table incl. the NO
        inversion proof + the non-repo==129 finding + the space-in-name-one-line proof (§2); the
        stub-guard closure decision (§3); the test matrix (§7)."
  critical: "§0 (interface absence), §2a (no --quiet inversion), and §3 (stub closure) are the three
             anchors. §2a is THE proof that the simple branch form is correct for StagedFileCount."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-add
  why: "git add reference. `-A, --all`: 'Update the index not only where the working tree has a file
        matching <pathspec> but also where the index already has an entry. This adds, modifies, and
        removes index entries to match the working tree.' Confirms add -A stages modified + untracked +
        deleted across the whole worktree (FR16/FR20)."
  critical: "Confirms add -A is the whole-worktree stage-all (not per-path). It exits 0 on success (the
             doc lists no 'exit 1 = something staged' semantic — that is --quiet-only). Do NOT pass a
             pathspec to AddAll (the contract is `git add -A`, bare)."
- url: https://git-scm.com/docs/git-diff#Documentation/git-diff.txt---name-only
  why: "--name-only: 'Show only names of changed files.' One path per line. This is the output form
        StagedFileCount counts."
  critical: "Establishes --name-only emits the file list (one per line). Do NOT pair it with --quiet
             (--quiet 'Disable all output of the program' — would suppress the list and break counting)."
- url: https://git-scm.com/docs/git-diff#Documentation/git-diff.txt--quiet
  why: "--quiet: 'Disable all output of the program. Implies --exit-code.' THIS is the source of FINDING
        6's exit-1 inversion. StagedFileCount DELIBERATELY OMITS --quiet."
  critical: "Read this to understand WHY omitting --quiet is load-bearing: --quiet turns the answer into
             an exit code AND suppresses output; without it, the answer is in stdout and exit is 0. The
             two options solve the SAME question ('what is staged?') in mutually-exclusive ways."
- url: https://git-scm.com/docs/git-diff#Documentation/git-diff.txt-z
  why: "-z: NUL-delimited pathnames for the pathological case of filenames with embedded newlines."
  critical: "Considered-and-rejected alternative (D4). The contract mandates the `wc -l` line-split form;
             -z would deviate. A filename with an embedded newline would inflate the count — accepted
             limitation (vanishingly rare; FR18's N is informational)."
```

### Current Codebase Tree (the assumed on-disk state — S1–S6, T3.S1–T3.S4 landed)

> The `git.go` read for this PRP shows `RecentSubjects` (S4), `RecentMessages`/`CommitCount` (S3), and
> all S2/S3/S4/S5/S6/T3.S1/T3.S2 methods already REAL — i.e. this working copy is at the end of
> P1.M1.T3 with ONLY `AddAll` left as a stub, and `TestStubsPanic` covering only AddAll. This subtask's
> edits are entirely NON-OVERLAPPING with any prior region.

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # imports (landed): bytes, context, errors, fmt, io, os/exec, strconv, strings.
│       │                 #   S1: Git interface (tail lists AddAll as S5's ONLY method) + gitRunner +
│       │                 #   run()/runWithInput/New/FileChange/StagedDiffOptions + stubs; S2: RevParseHEAD;
│       │                 #   S3: WriteTree; S4: CommitTree; S5: UpdateRefCAS/ErrCASFailed; S6: DiffTree+
│       │                 #   parseDiffTree; T3.S1: StagedDiff (consts/defaultExcludes); T3.S2:
│       │                 #   HasStagedChanges; T3.S3: RecentMessages(+strconv,+maxRecentMessageLines)+
│       │                 #   CommitCount; T3.S4: RecentSubjects. REMAINING STUB: AddAll (the last one).
│       ├── git_test.go   # S1: TestNew/TestRun_*/initRepo + assertPanics + TestStubsPanic. TestStubsPanic
│       │                 #   currently lists ONLY AddAll (S3/S4 removed their lines). assertPanics is used
│       │                 #   ONLY in TestStubsPanic (grep-confirmed).
│       ├── revparse_test.go   # S2 (REUSED: makeEmptyCommit)
│       ├── writetree_test.go  # S3
│       ├── committree_test.go # S4 (REUSED: writeFile, stageFile)
│       ├── updateref_test.go  # S5
│       ├── difftree_test.go   # S6
│       ├── stagediff_test.go  # T3.S1
│       ├── hasstaged_test.go  # T3.S2
│       ├── commitcount_test.go     # T3.S3
│       ├── recentmessages_test.go  # T3.S3
│       └── recentsubjects_test.go  # T3.S4
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go               # MODIFIED — (a) interface: ADD StagedFileCount; (b) comment block:
        │                        #   S5 lists AddAll + StagedFileCount; (c) AddAll stub → real;
        │                        #   (d) ADD StagedFileCount body. NO import/const change.
        ├── git_test.go          # MODIFIED — REMOVE TestStubsPanic + assertPanics (D6). Keep initRepo/
        │                        #   TestNew/TestRun_*.
        ├── revparse_test.go     # UNCHANGED (makeEmptyCommit reused, not edited)
        ├── writetree_test.go    # UNCHANGED
        ├── committree_test.go   # UNCHANGED (writeFile/stageFile reused, not edited)
        ├── updateref_test.go    # UNCHANGED
        ├── difftree_test.go     # UNCHANGED
        ├── stagediff_test.go    # UNCHANGED
        ├── hasstaged_test.go    # UNCHANGED
        ├── commitcount_test.go  # UNCHANGED
        ├── recentmessages_test.go # UNCHANGED
        ├── recentsubjects_test.go # UNCHANGED
        ├── addall_test.go       # NEW — package git; 7 tests (reuse initRepo/writeFile/stageFile/makeEmptyCommit)
        └── stagedcount_test.go  # NEW — package git; 9 tests (reuse initRepo/writeFile/stageFile/makeEmptyCommit)
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (a) Add `StagedFileCount` to the `Git` interface (doc comment + signature). (b) Update the interface comment block's S5 line. (c) Replace the `AddAll` panic-stub with the real `git add -A` body. (d) Add the `StagedFileCount` method body. No import/constant change. |
| `internal/git/git_test.go` | MODIFY | Remove the `TestStubsPanic` function and the `assertPanics` helper (D6). Nothing else. |
| `internal/git/addall_test.go` | CREATE | `package git` tests for `AddAll`. Reuse `initRepo`/`writeFile`/`stageFile`/`makeEmptyCommit`. No new helpers. |
| `internal/git/stagedcount_test.go` | CREATE | `package git` tests for `StagedFileCount`. Reuse `initRepo`/`writeFile`/`stageFile`/`makeEmptyCommit`. No new helpers. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, `FileChange`,
`StagedDiffOptions`, any existing `Git` interface signature (only ONE method ADDED), RevParseHEAD (S2),
WriteTree (S3), CommitTree (S4), UpdateRefCAS/ErrCASFailed (S5), DiffTree/parseDiffTree (S6),
StagedDiff/its constants/`defaultExcludes` (T3.S1), HasStagedChanges (T3.S2),
RecentMessages/`maxRecentMessageLines` (T3.S3), CommitCount (T3.S3), RecentSubjects (T3.S4), every other
test file, `go.mod`/`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — THE central design call: ADD StagedFileCount to the interface). The work-item contract
// mandates StagedFileCount(ctx) (int, error), but the Git interface (S1) declares ONLY AddAll. This
// subtask MUST append StagedFileCount to the interface (doc comment + signature) — the FIRST new
// interface member since S1. Add it AFTER the existing AddAll interface line; do NOT reorder or alter
// any existing signature. Without this, the FR18 "N files" notice cannot be produced.

// CRITICAL (G2 — StagedFileCount OMITS --quiet; NO exit-1 inversion). HasStagedChanges (T3.S2) runs
// `git diff --cached --quiet` and switches on exit 1 (FINDING 6). StagedFileCount runs `git diff --cached
// --name-only` (NO --quiet): --name-only EMITS the file list we count; --quiet would SUPPRESS it. Without
// --quiet, exit is 0 whether or not changes exist (verified empirically). So StagedFileCount uses the
// SIMPLE branch form (code != 0 → error), NOT HasStagedChanges' switch. A "helpful" future refactor that
// adds --quiet would silently make StagedFileCount ALWAYS return 0 (empty output). This is the analog of
// S4's %s-vs-%x00 call: the sibling's option is load-bearing.

// CRITICAL (G3 — AddAll is a MUTATION with NO 128 special-case). `git add -A` exits 0 on every happy
// path (born/unborn, with/without changes); only non-repo/corrupt makes it non-zero (exit 128). So
// AddAll's branch order is the SIMPLE mutation form (code != 0 → error), byte-identical to
// WriteTree/CommitTree. Do NOT cargo-cult RevParseHEAD/RecentMessages' exit-128⇒not-an-error — that is a
// READ-method semantic; add -A is a mutation that treats ALL non-zero exits as errors.

// CRITICAL (G4 — run()'s invariant, inherited from S1). run() returns err == nil for NON-ZERO git exits
// (128, 129, …). Only infrastructural failures (LookPath miss, context cancel, start/I/O) set err != nil,
// with exitCode == -1. So BOTH methods MUST check `err != nil` FIRST, THEN branch on exitCode. Verified:
// add -A on non-repo → exit 128, err nil; diff --cached on non-repo → exit 129, err nil.

// GOTCHA (G5 — non-repo exits DIFFERENTLY for the two commands). For `git add -A`, non-repo → exit 128
// (fatal: not a git repository). For `git diff --cached --name-only`, non-repo → exit 129 (git falls into
// --no-index two-file mode; --cached invalid there). This 128-vs-129 difference is INCONSEQUENTIAL: both
// branch on `code != 0 → error` and surface the exit code + stderr in the message. Do NOT special-case
// either code (mirrors DiffTree: "branch on code != 0, not on a specific code").

// GOTCHA (G6 — NO new import, NO new constant). Both methods use strings (Split/TrimSpace) and fmt
// (Errorf) — both ALREADY imported (the block lists bytes/context/errors/fmt/io/os.exec/strconv/strings).
// AddAll uses fmt+strings; StagedFileCount uses fmt+strings. No strconv, no new const. Do NOT touch the
// import block or the const region (defaultExcludes/defaultMaxMDLines/maxRecentMessageLines) — touching
// either risks dead code or a merge surprise. There is no line cap for StagedFileCount (it returns an
// int, not a list — a cap is meaningless).

// GOTCHA (G7 — count non-empty lines via TrimSpace + empty-skip). `git diff --cached --name-only` emits
// a trailing newline after the last path → strings.Split(stdout, "\n") yields a final "" element. For
// EMPTY output (nothing staged), stdout is "" → Split → [""] → one empty element. Both are dropped by
// `if strings.TrimSpace(line) == "" { continue }` (or `!= "" { count++ }`). Verified: 3 files → count 3;
// nothing staged → count 0.

// GOTCHA (G8 — filenames with SPACES stay on ONE line). Verified: `sub/has space.txt` is emitted as the
// single line `sub/has space.txt` (git does NOT quote spaces in --name-only without -z). So line-counting
// is correct for the common case. A filename with an EMBEDDED NEWLINE would inflate the count (accepted
// limitation, D4; would require -z). The TestStagedFileCount_FilenameWithSpace test guards the common case.

// GOTCHA (G9 — stub-guard CLOSURE: remove TestStubsPanic + assertPanics). AddAll is the FINAL panic-stub.
// Removing only its assertPanics line leaves `ctx`/`g` as unused locals → Go compile error. Grep confirms
// assertPanics is used ONLY in TestStubsPanic (git_test.go:104 def, :123 sole call). Therefore REMOVE the
// entire TestStubsPanic function AND the assertPanics helper. Do NOT leave an empty TestStubsPanic shell
// (unused vars). This is the clean closure of S1's "panic to fail fast" scaffolding — zero stubs remain.

// GOTCHA (G10 — reuse helpers, do NOT redeclare). initRepo (git_test.go), writeFile/stageFile
// (committree_test.go, S4's CommitTree tests), and makeEmptyCommit (revparse_test.go, S2) are `package
// git` and in scope for the new test files. REUSE them — do NOT redeclare (redeclaration = compile error).
// To verify AddAll's effect INDEPENDENTLY of StagedFileCount, run `git diff --cached --name-only` inline
// via exec.Command in the test (mirrors the fixture-helper style, which uses exec directly — no shell).

// GOTCHA (G11 — test files are package git, white-box). AddAll/StagedFileCount are on *gitRunner and
// call the unexported run(). To call New() and exercise the real methods against temp repos, the tests
// MUST be `package git` (NOT git_test). Matches every other test file in internal/git/.

// GOTCHA (G12 — no shell, no cmd.Dir in PRODUCTION code). Both methods inherit S1's §19 guarantees because
// they only call run(). Do NOT introduce exec.Command / os.Chdir / sh -c in git.go. The test fixtures DO
// use exec.Command (the reused initRepo/writeFile/stageFile/makeEmptyCommit helpers use []string args +
// cmd.Env, never a shell) — acceptable test-fixture usage.

// GOTCHA (G13 — AddAll mutates the INDEX, not a ref (PRD §18.1)). add -A writes .git/index. This is an
// EXPECTED pre-commit mutation, upstream of the immutable snapshot (WriteTree/CommitTree) and far
// upstream of the ref CAS (UpdateRefCAS). It does NOT threaten the "snapshot then CAS" atomicity because
// the snapshot (WriteTree) is taken AFTER AddAll, from the freshly-staged index. Document this in
// AddAll's doc comment so a future reader does not mistake it for a ref mutation.
```

## Implementation Blueprint

### Data models and structure

None added or changed. `AddAll`'s signature `func (g *gitRunner) AddAll(ctx context.Context) error` is
already declared in the `Git` interface by S1. `StagedFileCount`'s signature
`func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error)` is ADDED to the interface here
(the one interface change). **No new constant** (StagedFileCount returns an int — a cap is meaningless;
AddAll returns an error). **No new struct field.**

### The `AddAll` body (exact — copy verbatim)

Replaces S1's panic-stub of the same name/signature. Place it where the stub currently is.

```go
// AddAll stages every change in the working tree — new, modified, AND deleted files — via
// `git add -A` (PRD §9.4/FR16, FR20; FINDING 11). It is the auto-stage-all primitive the CLI default
// action (P1.M4.T1.S2) calls when nothing is staged (and `auto_stage_all` is on, default true) and that
// `--all`/`-a` (FR20) force-invokes even when something is already staged. `-A` operates on the WHOLE
// worktree (no pathspec) — it adds untracked files, updates modified ones, and removes deleted ones,
// making the index match the working tree.
//
// It MUTATES THE INDEX (writes .git/index) — this is an EXPECTED pre-commit mutation, NOT a ref change
// (PRD §18.1: refs move ONLY at UpdateRefCAS). The immutable snapshot (WriteTree) is taken AFTER AddAll,
// from the freshly-staged index, so AddAll does not threaten the snapshot-then-CAS atomicity. On a clean
// working tree `git add -A` is a safe no-op (exit 0, index unchanged).
//
// `git add -A` exits 0 on every happy path (born or unborn repo, with or without changes); a non-zero
// exit (128 on a non-repo / corrupt repo) is a real error. So — unlike the read methods that special-case
// exit 128 as "unborn is not an error" — AddAll treats ALL non-zero exits as errors (it is a mutation,
// structurally identical to WriteTree/CommitTree). It delegates to run() (not exec) and targets the repo
// via the -C flag (not cmd.Dir).
func (g *gitRunner) AddAll(ctx context.Context) error {
	_, stderr, code, err := g.run(ctx, g.workDir, "add", "-A")
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return fmt.Errorf("git add -A: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}
```

> **Verified:** `git add -A` exit 0 on born (modified+untracked) / clean tree / unborn+staged / unborn
> empty; exit 128 on non-repo (research §1). The args `["add", "-A"]` are passed as []string (no shell).
> `stdout` is referenced as `_` because add -A prints nothing on success (silence unused-var linters —
> mirrors UpdateRefCAS's `_ = stdout`).

### The `StagedFileCount` body (exact — copy verbatim)

A NEW method (and a NEW interface member — G1). Place it adjacent to AddAll (e.g. immediately after the
AddAll body, or after CommitCount; choose one and keep gofmt spacing). The interface declaration goes in
the `Git` interface (see §"The interface addition" below).

```go
// StagedFileCount returns the number of files currently staged in the index (PRD §9.4/FR18). It is the
// `N` in the auto-stage notice "Nothing staged — staging all changes (N files)." — the CLI layer
// (P1.M4.T1.S2) calls it AFTER AddAll to report how many files auto-staging touched. It runs
// `git diff --cached --name-only`, which lists each staged path on its own line (one per file: added,
// modified, OR deleted — all count), and returns the count of non-empty lines.
//
// NOTE — why this OMITS `--quiet` (and does NOT invert exit codes like its sibling HasStagedChanges):
// HasStagedChanges (T3.S2) runs `git diff --cached --quiet`, where `--quiet` SUPPRESSES output and
// encodes the answer in the exit code (exit 1 = staged, FINDING 6). StagedFileCount needs the file LIST
// to count it, so it uses `--name-only` and OMITS `--quiet`: without `--quiet`, `git diff` exits 0
// whether or not changes exist, and `--name-only` emits the paths. Adding `--quiet` here would SUPPRESS
// the list and silently make StagedFileCount ALWAYS return 0. So StagedFileCount uses the SIMPLE branch
// form (code != 0 → error), byte-identical to StagedDiff/DiffTree — NOT HasStagedChanges' switch form.
//
// Counting splits stdout on "\n" and counts non-empty (post-TrimSpace) lines. The trailing newline after
// the last path yields a final "" element, which is dropped; empty output (nothing staged) yields count 0.
// A filename containing SPACES stays on ONE line (git does not quote spaces without -z), so the count is
// correct for the common case. A filename with an EMBEDDED NEWLINE would inflate the count — an accepted
// limitation (vanishingly rare; FR18's N is informational); the contract mandates the `wc -l` line-split
// form, NOT the NUL-delimited `-z` alternative. It is read-only with respect to refs (PRD §18.1) — it
// mutates neither the index nor HEAD.
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		// NOTE: a NON-REPO directory exits 129 here (git falls into --no-index mode; --cached invalid),
		// NOT 128 — but we branch on code != 0, not on a specific code (G5). 128 (corrupt) and 129 (non-repo)
		// are both real errors. Do NOT add --quiet (G2): it would suppress stdout and break the count.
		return 0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++ // trailing newline → final "" element is skipped; empty output → count 0
		}
	}
	return count, nil
}
```

> **Verified:** `git diff --cached --name-only` exit 0 with 3 staged files (3 lines) / nothing staged
> (empty, count 0) / unborn+staged (1 line); exit 129 on non-repo; space-in-name stays one line (research
> §2). The args `["diff", "--cached", "--name-only"]` are passed as []string (no shell).

### The interface addition (exact — copy verbatim)

In the `Git` interface, the tail currently ends with the `AddAll` doc comment + signature. ADD
`StagedFileCount` immediately AFTER the `AddAll(ctx context.Context) error` line:

```go
	// AddAll stages all changes (git add -A). Used by the auto-stage-all path (PRD §9.4 / FINDING 11).
	AddAll(ctx context.Context) error

	// StagedFileCount returns the number of files currently staged (git diff --cached --name-only,
	// count of non-empty lines). Used for the FR18 "Nothing staged — staging all changes (N files)."
	// notice. Read-only with respect to refs and the index.
	StagedFileCount(ctx context.Context) (int, error)
}
```

### The interface ownership comment update (exact)

In the comment block above the `Git interface` declaration, the S5 line currently reads:

```go
//	RecentSubjects    — P1.M1.T3.S4   AddAll           — P1.M1.T3.S5
```

UPDATE it so S5 lists BOTH methods (keep the 2-column alignment roughly; gofmt aligns the `—`):

```go
//	RecentSubjects    — P1.M1.T3.S4   AddAll / StagedFileCount — P1.M1.T3.S5
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — interface (ADD StagedFileCount + update comment block)
  - EDIT 1 — in the Git interface tail, after the AddAll interface signature line:
      FIND:
        // AddAll stages all changes (git add -A). Used by the auto-stage-all path (PRD §9.4 / FINDING 11).
        AddAll(ctx context.Context) error
      REPLACE with: the SAME two lines PLUS the StagedFileCount doc comment + signature from
        §"The interface addition". (Adds one interface method.)
  - EDIT 2 — in the interface ownership comment block:
      FIND:  //	RecentSubjects    — P1.M1.T3.S4   AddAll           — P1.M1.T3.S5
      REPLACE: //	RecentSubjects    — P1.M1.T3.S4   AddAll / StagedFileCount — P1.M1.T3.S5
  - DO NOT touch any other interface signature. DO NOT touch the import block (G6).
  - VERIFY: go build ./internal/git/ → exit 0 (gitRunner now lacks a StagedFileCount method → build FAILS
    until Task 3 adds the body; that is EXPECTED mid-Task-1. Run the build again after Task 3.)

Task 2: MODIFY internal/git/git.go — AddAll body (replace panic-stub)
  - EDIT — replace the AddAll panic-stub:
      FIND:
        func (g *gitRunner) AddAll(ctx context.Context) error {
            panic("gitRunner.AddAll: not yet implemented — see P1.M1.T3.S5")
        }
      REPLACE with: the doc comment + body from §"The AddAll body". Keep the SAME signature
      `func (g *gitRunner) AddAll(ctx context.Context) error`.
  - DO NOT touch run(), the interface, or any other method/stub.

Task 3: MODIFY internal/git/git.go — ADD StagedFileCount body
  - EDIT — INSERT the doc comment + body from §"The StagedFileCount body" as a NEW method on
    *gitRunner. Placement: immediately AFTER the AddAll body (keeps the two auto-stage methods
    adjacent) OR after CommitCount — choose one; ensure a single blank line separates methods (gofmt).
  - DO NOT add a panic-stub for it (it is real from creation). DO NOT touch imports/constants.
  - VERIFY: go build ./internal/git/ → exit 0 (now gitRunner satisfies the interface incl. StagedFileCount).

Task 4: MODIFY internal/git/git_test.go — REMOVE TestStubsPanic + assertPanics (D6)
  - EDIT — DELETE the entire assertPanics function AND the entire TestStubsPanic function (they are
    contiguous: assertPanics first, then TestStubsPanic). The exact text to remove is in §"The
    git_test.go edit" below. Leave initRepo, TestNew, TestRun_* UNTOUCHED.
  - VERIFY: go build ./internal/git/ → exit 0 (no unused locals; assertPanics no longer referenced).

Task 5: CREATE internal/git/addall_test.go (package git — white-box)
  - FILE: internal/git/addall_test.go
  - PACKAGE line: `package git`  (NOT git_test — G11)
  - IMPORTS: context, errors, os, os/exec, strings, testing  (errors for errors.Is; os for os.Remove in
    the deletion test; os/exec + strings for the inline staged-list verification; reuse writeFile/
    stageFile/makeEmptyCommit/initRepo so do NOT redeclare them).
  - WRITE the 7 test functions (assertions in §"Test cases — AddAll"). No new helpers (G10).
  - VERIFY: go test -race -run TestAddAll ./internal/git/ → exit 0, all 7 pass.

Task 6: CREATE internal/git/stagedcount_test.go (package git — white-box)
  - FILE: internal/git/stagedcount_test.go
  - PACKAGE line: `package git`
  - IMPORTS: context, errors, testing  (errors for errors.Is; reuse helpers). NOTE: the space-in-name
    test needs no extra import beyond what writeFile provides (filepath.Join is inside writeFile).
  - WRITE the 9 test functions (assertions in §"Test cases — StagedFileCount"). No new helpers.
  - VERIFY: go test -race -run TestStagedFileCount ./internal/git/ → exit 0, all 9 pass.

Task 7: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go  (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                 (expect: no matches)
  - RUN: git grep -n 'panic.*AddAll' internal/git/git.go                       (expect: no matches — stub gone)
  - RUN: git grep -n 'assertPanics\|TestStubsPanic' internal/git/git_test.go   (expect: no matches — removed)
  - RUN: git grep -n -- '--quiet' internal/git/git.go | grep -i 'stagedcount\|StagedFileCount' \
         (expect: no matches — StagedFileCount must NOT use --quiet, G2; the only --quiet is in
         HasStagedChanges, T3.S2)
  - RUN: git grep -nc 'StagedFileCount' internal/git/git.go  (expect: ≥2 — one in the interface, one as
         the method receiver)
  - RUN: git status --porcelain → expect EXACTLY internal/git/git.go (modified) + internal/git/
         git_test.go (modified) + internal/git/addall_test.go (new) + internal/git/stagedcount_test.go
         (new). Nothing else under internal/git/ or elsewhere.
```

### The git_test.go edit (exact — DELETE these two contiguous functions)

Remove BOTH of these (they are contiguous in the current file):

```go
func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic, but did not panic", name)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "not yet implemented") {
			t.Fatalf("%s: panic message = %v, want it to contain 'not yet implemented'", name, r)
		}
	}()
	fn()
}

func TestStubsPanic(t *testing.T) {
	ctx := context.Background()
	g := New(".")

	assertPanics(t, "AddAll", func() { _ = g.AddAll(ctx) })
}
```

After removal, the file retains: the `import` block (note: `strings` and `context` remain used by
TestRun_* and initRepo's `os.Environ`; verify no import becomes unused — `strings` is used in
TestRun_HappyPath/CapturesExitCode/LookPathFailure; `context` in TestRun_*; `bytes` in
CapturesExitCode; `os`/`os/exec` in initRepo. ALL remain used. No import removal needed.)

> **IMPORTANT — verify imports stay used after the deletion.** The four imports in git_test.go are
> `bytes` (TestRun_CapturesExitCode), `context` (TestRun_*), `os` + `os/exec` (initRepo), `strings`
> (TestRun_*). assertPanics/TestStubsPanic also used `strings` and `context`, but those imports have
> OTHER users in the file — so removing the two functions does NOT orphan any import. If a future
> refactor removes more tests, re-check; for THIS edit, no import change is required.

### Test cases — AddAll (`addall_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestAddAll_StagesModifiedAndUntracked` | born: commit a.go; modify a.go; create b.go (untracked); AddAll | err==nil; inline `git diff --cached --name-only` lists BOTH a.go AND b.go | add -A stages modified + untracked (FR16/FR20) |
| `TestAddAll_StagesDeletion` | born: commit a.go; `os.Remove(a.go)`; AddAll | err==nil; staged set contains a.go (deleted) | add -A stages deletions |
| `TestAddAll_CleanTreeNoOp` | born: commit; clean tree; AddAll | err==nil; `g.StagedFileCount()==0` | add -A safe no-op on clean tree |
| `TestAddAll_UnbornRepoStagesFiles` | unborn: create f.go; AddAll | err==nil; `g.StagedFileCount()==1` | add -A works on unborn (vs empty tree) |
| `TestAddAll_NotARepo` | plain `t.TempDir()` (no initRepo) | err!=nil contains "git add -A: failed" | non-repo (exit 128) → wrapped error (G3/G5) |
| `TestAddAll_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found" | run() err path (G4) |
| `TestAddAll_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced |

```go
// Helper-free inline verification (mirrors fixture-helper style — exec directly, no shell):
func TestAddAll_StagesModifiedAndUntracked(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")     // tracked
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "package main\nvar x = 1\n") // modified
	writeFile(t, repo, "b.go", "package main\n")           // untracked

	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	// Independent verification (NOT via StagedFileCount — decouples the assertion):
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("verify diff: %v", err)
	}
	got := strings.Fields(string(out)) // splits on whitespace; each path is one token here
	want := map[string]bool{"a.go": true, "b.go": true}
	for _, p := range got {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatalf("AddAll did not stage expected files; missing %v, got %v", want, got)
	}
}

func TestAddAll_StagesDeletion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	if err := os.Remove(filepath.Join(repo, "a.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background()) // integration: deletion counts as 1 staged
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (deletion should be staged)", count)
	}
}

func TestAddAll_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail (G10)
	g := New(t.TempDir())
	err := g.AddAll(context.Background())
	if err == nil {
		t.Fatal("AddAll err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
}

func TestAddAll_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism
	g := New(t.TempDir())
	err := g.AddAll(ctx)
	if err == nil {
		t.Fatal("AddAll err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
```
> NOTE: `addall_test.go` imports must include `path/filepath` (for the deletion test's `filepath.Join`)
> and `os` (for `os.Remove`) and `os/exec` + `strings` (for the inline verification). Reuse
> `writeFile`/`stageFile`/`makeEmptyCommit`/`initRepo` (do NOT redeclare — G10).

### Test cases — StagedFileCount (`stagedcount_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestStagedFileCount_NothingStaged` | born: commit; clean | count==0, err==nil | empty output → 0 (G7) |
| `TestStagedFileCount_ThreeFiles` | stage 3 distinct files | count==3 | line-count correct |
| `TestStagedFileCount_AfterAddAll` | modify+untracked; AddAll | count==2 | integration w/ AddAll (FR18 N) |
| `TestStagedFileCount_IncludesDeletion` | commit a.go; rm; AddAll | count==1 | deletions counted |
| `TestStagedFileCount_FilenameWithSpace` | file `sub/has space.txt`; AddAll | count==1 | space-in-name stays one line (G8/D4) |
| `TestStagedFileCount_UnbornRepoWithStaged` | unborn: create+AddAll f.go | count==1, err==nil | unborn + staged diffs vs empty tree |
| `TestStagedFileCount_NotARepo` | plain `t.TempDir()` (no initRepo) | err!=nil contains "failed" | non-repo (exit 129) → error (G5) |
| `TestStagedFileCount_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found" | run() err path (G4) |
| `TestStagedFileCount_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced |

```go
func TestStagedFileCount_NothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // HEAD exists; nothing NEW staged
	g := New(repo)
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (nothing staged → empty output)", count)
	}
}

func TestStagedFileCount_ThreeFiles(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "1\n"); stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "2\n"); stageFile(t, repo, "b.go")
	writeFile(t, repo, "c.go", "3\n"); stageFile(t, repo, "c.go")
	g := New(repo)
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
}

func TestStagedFileCount_FilenameWithSpace(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "sub/has space.txt", "x\n") // create dir + file with a SPACE in the name
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (a space in the name must NOT split into 2 lines under --name-only)",
			count)
	}
}

func TestStagedFileCount_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo (no initRepo) → exit 129
	count, err := g.StagedFileCount(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (non-repo → exit 129)")
	}
	if !strings.Contains(err.Error(), "git diff --cached --name-only: failed") {
		t.Fatalf("err = %v, want it to contain 'git diff --cached --name-only: failed'", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}

func TestStagedFileCount_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	g := New(t.TempDir())
	count, err := g.StagedFileCount(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}

func TestStagedFileCount_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := New(t.TempDir())
	count, err := g.StagedFileCount(ctx)
	if err == nil {
		t.Fatal("err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}
```
> NOTE: `stagedcount_test.go` imports: `context`, `errors`, `strings`, `testing` (and reuses
> writeFile/stageFile/makeEmptyCommit/initRepo — do NOT redeclare). `strings` is used in the NotARepo /
> GitBinaryMissing Contains checks.

### Implementation Patterns & Key Details

```go
// === Why AddAll has NO 128 special-case (the mutation-vs-read distinction, G3) ===
// RevParseHEAD/RecentMessages/CommitCount/RecentSubjects special-case exit 128 as "unborn is not an
// error" because their READ command (rev-parse/log/rev-list) exits 128 on a zero-commit repo. `git add
// -A` is a MUTATION that exits 0 on unborn (it stages vs the empty tree); it exits 128 only on a
// non-repo/corrupt repo — a real error. So AddAll mirrors WriteTree/CommitTree (code != 0 → error), NOT
// the read methods. Verified: add -A on unborn+staged → exit 0; add -A on non-repo → exit 128.

// === Why StagedFileCount OMITS --quiet (G2, the central design call) ===
// HasStagedChanges (T3.S2) runs `git diff --cached --quiet`: --quiet SUPPRESSES output and the answer
// lives in the exit code (1 = staged, FINDING 6). StagedFileCount needs the file LIST to count it, so it
// uses `--name-only` and OMITS --quiet: without --quiet, exit is 0 whether-or-not-staged (verified), and
// --name-only emits the paths one per line. Adding --quiet would make StagedFileCount ALWAYS return 0.

// === Why count non-empty lines (D3) and NOT -z (D4) ===
// The contract mandates "git diff --cached --name-only | wc -l equivalent (split lines)". Splitting on
// "\n" and counting non-empty lines is exactly that. -z (NUL-delimited) would handle embedded-newline
// filenames but deviates from the contract; FR18's N is informational. Filenames with SPACES stay one
// line (verified) — the common case is correct.

// === Why err is checked BEFORE code (the branch order, G4) ===
// run() guarantees err != nil ⟹ exitCode == -1 (LookPath / context / start failure) and err == nil for
// every real git exit (0, 128, 129). So `if err != nil { return err }` is the authoritative
// infrastructural-failure guard; only then does the code branch run. Byte-identical to every landed method.

// === Why code != 0 (not code == 128/129) for the error branch (G5) ===
// add -A non-repo → 128; diff --cached non-repo → 129. Branching on `code != 0` handles BOTH (and any
// other non-zero) uniformly. Mirrors DiffTree ("branch on code != 0, not on a specific code").

// === Why the interface gains StagedFileCount HERE (G1) ===
// S1 declared only AddAll as the S5 method (the contract evolved to add the count). Appending
// StagedFileCount now (vs. retro-fitting later) means the interface and its single implementation ship
// together, and the FR18 consumer (P1.M4.T1.S2) can be written against a complete interface.

// === Reusing initRepo + writeFile + stageFile + makeEmptyCommit ===
// All are `package git` helpers in sibling test files, in scope for the new test files. Call them
// directly; do NOT redefine (redeclaration = compile error). For INDEPENDENT AddAll verification (not via
// StagedFileCount), run `git diff --cached --name-only` inline via exec.Command (mirrors the fixture-
// helper style: []string args, no shell).
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, errors.Is, strings, fmt, os/exec, os.Remove, filepath.Join all available
  - deps: ZERO new imports; go.mod unchanged (stdlib only)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go            # MODIFIED: interface +AddAll comment+signature +StagedFileCount;
  #                                       AddAll stub→real; +StagedFileCount body. NO import/const change.
  - file: internal/git/git_test.go       # MODIFIED: remove TestStubsPanic + assertPanics (D6)
  - file: internal/git/addall_test.go    # NEW: package git, 7 tests, reuse helpers
  - file: internal/git/stagedcount_test.go # NEW: package git, 9 tests, reuse helpers

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M4.T1.S2 (CLI default action): the PRIMARY consumer. Calls HasStagedChanges; if nothing staged and
    auto_stage_all: print FR18 notice with StagedFileCount's N, call AddAll, re-check (FR17). Also the
    --all/-a (FR20) path calls AddAll unconditionally.
  - P1.M3.T2.S2 (auto-stage logic) / FINDING 11: orchestrates AddAll → re-check → notice.
  - The notice STRING formatting lives in the UI layer (P1.M4.T3), NOT here — StagedFileCount returns
    the int; the UI formats "Nothing staged — staging all changes (3 files).".

PARALLEL-EXECUTION NOTE (with P1.M1.T3.S4 — RecentSubjects, concurrent or already-landed):
  - The git.go read shows RecentSubjects (S4) already REAL in this working copy and TestStubsPanic already
    reduced to AddAll-only. Regardless of S4's exact status, THIS subtask edits the AddAll stub region +
    adds StagedFileCount (interface tail + new method, regions no prior subtask touched) + removes
    TestStubsPanic/assertPanics + adds two new test files. These are DISTINCT, NON-OVERLAPPING regions.
    This subtask adds NO imports and NO constants, so the import block and const region are untouched.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each edit — fix before proceeding
gofmt -w internal/git/git.go internal/git/addall_test.go internal/git/stagedcount_test.go
go vet ./internal/git/
go build ./...

# Project-wide validation
make lint   # if the Makefile wires golangci-lint; else go vet ./...
gofmt -l internal/git/   # expect: empty output (no files need formatting)

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test each new method in isolation
go test -race -run TestAddAll ./internal/git/ -v
go test -race -run TestStagedFileCount ./internal/git/ -v

# Confirm the stub-guard test is GONE (no TestStubsPanic / assertPanics)
go test -race -run 'TestStubsPanic|assertPanics' ./internal/git/ -v   # expect: no tests ran (0 tests)

# Full internal/git suite (all prior S1–S6 / T3.S1–T3.S4 tests must stay green)
go test -race ./internal/git/ -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# No service to start (library methods). Validate end-to-end against a real git repo:
cd "$(mktemp -d)" && git init -q && git config user.email t@e.com && git config user.name T
git commit --allow-empty -q -m "init"
echo "a" > a.go; echo "b" > b.go; echo "c" > c.go
git add a.go            # stage one
git add -A; echo "add -A EXIT=$?"                 # expect 0
git diff --cached --name-only | cat -A            # expect a.go$ b.go$ c.go$ (3 lines)
git diff --cached --name-only | grep -c .          # expect 3 (the count StagedFileCount returns)
# Inversion check (proves StagedFileCount must NOT use --quiet):
git diff --cached --name-only >/dev/null 2>&1; echo "name-only EXIT=$?"   # expect 0 (NOT 1)
git diff --cached --quiet        >/dev/null 2>&1; echo "quiet EXIT=$?"     # expect 1 (the inversion)

# Cross-check exit-code semantics the methods rely on:
( cd "$(mktemp -d)" && git add -A >/dev/null 2>&1; echo "add -A nonrepo EXIT=$?" )  # expect 128
( cd "$(mktemp -d)" && git diff --cached --name-only >/dev/null 2>&1; echo "diff nonrepo EXIT=$?" )  # expect 129

# Expected: command output matches the methods' parse model (exit 0 on success for both; --name-only is
# NOT inverted; non-repo 128 for add / 129 for diff; one path per line).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope-discipline grep (enforce FORBIDDEN-OPERATIONS boundaries):
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   # expect: no matches (no shell)
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                  # expect: no matches (no chdir)
git grep -n 'panic.*AddAll' internal/git/git.go                        # expect: no matches (stub gone)
git grep -n 'assertPanics\|TestStubsPanic' internal/git/git_test.go    # expect: no matches (removed)

# CRITICAL anti-cargo-cult check: StagedFileCount must NOT use --quiet (G2):
git grep -n -- '--quiet' internal/git/git.go   # expect: exactly ONE hit, in HasStagedChanges (T3.S2)
git grep -A15 'func (g \*gitRunner) StagedFileCount' internal/git/git.go | grep -- '--quiet' \
  && echo "FAIL: --quiet leaked into StagedFileCount" || echo "OK: no --quiet in StagedFileCount"

# Confirm StagedFileCount exists in BOTH the interface and as a method:
git grep -nc 'StagedFileCount' internal/git/git.go   # expect: ≥2 (interface + receiver)

# Confirm the touched-files set is exactly the four expected:
git status --porcelain internal/git/

# Expected: no shell, no chdir, no remaining panic, no TestStubsPanic/assertPanics, no --quiet in
# StagedFileCount; status shows exactly git.go + git_test.go (modified) + addall_test.go +
# stagedcount_test.go (new).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./internal/git/ -v` (16 new + all prior green)
- [ ] No vet errors: `go vet ./internal/git/`
- [ ] No formatting issues: `gofmt -l internal/git/` (empty)
- [ ] `go build ./...` exits 0

### Feature Validation

- [ ] All success criteria from "What" section met
- [ ] `Git` interface gained exactly ONE new method (`StagedFileCount`); no existing signature changed
- [ ] `AddAll` stages modified + untracked + deleted; safe no-op on clean tree; works on unborn;
      wrapped error on non-repo
- [ ] `StagedFileCount` returns 0 (clean), N (N staged), includes deletions, counts space-in-name as 1
- [ ] `AddAll` has NO 128 special-case (mutation form, G3)
- [ ] `StagedFileCount` has NO `--quiet` and NO exit-code switch (simple form, G2)
- [ ] Missing git binary → "git binary not found"; cancelled ctx → `errors.Is(err, context.Canceled)`
- [ ] `TestStubsPanic` + `assertPanics` REMOVED (zero stubs remain; no unused-local compile error)
- [ ] Integration points (P1.M4.T1.S2 auto-stage + FR18 notice) satisfiable by `(int, error)` + `AddAll`

### Code Quality Validation

- [ ] Follows existing codebase patterns (AddAll mirrors WriteTree/CommitTree; StagedFileCount mirrors
      StagedDiff/DiffTree simple-branch form)
- [ ] File placement matches desired codebase tree (`addall_test.go` + `stagedcount_test.go`)
- [ ] Anti-patterns avoided: NO `--quiet` cargo-cult (G2), NO 128 special-case on AddAll (G3),
      NO new imports/constants (G6)
- [ ] Reused `initRepo` + `writeFile` + `stageFile` + `makeEmptyCommit` (no redeclaration — G10)

### Documentation & Deployment

- [ ] Method doc comments explain the three design calls (interface addition; omit `--quiet`; no 128
      special-case) so the next reader does not "fix" them
- [ ] No new environment variables or config (pure library methods)
- [ ] No logs added (library methods; callers decide logging)

---

## Anti-Patterns to Avoid

- ❌ Don't add `--quiet` to StagedFileCount — it suppresses the file list and silently returns 0 forever
      (G2). The whole point is to COUNT the `--name-only` output.
- ❌ Don't replicate HasStagedChanges' `switch code {0/1/default}` in StagedFileCount — that inversion is
      `--quiet`-only; without `--quiet`, exit is always 0 on success (G2).
- ❌ Don't special-case exit 128 in AddAll — add -A is a mutation; ALL non-zero exits are errors (G3).
      The 128⇒"unborn is fine" semantic belongs to READ methods only.
- ❌ Don't forget to ADD `StagedFileCount` to the `Git` interface (G1) — it is contractually required and
      ABSENT today. The implementation alone will not compile without the interface member (well, the
      struct method exists but `New()` returns the interface, so callers can't see it).
- ❌ Don't just delete the AddAll line from TestStubsPanic — that leaves unused locals (`ctx`, `g`) →
      compile error. Remove the whole `TestStubsPanic` + `assertPanics` (D6/G9).
- ❌ Don't add a new import or constant — `strings` + `fmt` are already present (G6).
- ❌ Don't switch to `-z` for StagedFileCount — the contract mandates the `wc -l` line-split form (D4).
- ❌ Don't create new patterns when existing ones work — the branch order is byte-identical to
      WriteTree/CommitTree (AddAll) and StagedDiff/DiffTree (StagedFileCount).
- ❌ Don't ignore failing tests — fix the implementation, not the test.
- ❌ Don't catch all exceptions — branch on exit code as specified (both use `code != 0 → wrapped error`).
