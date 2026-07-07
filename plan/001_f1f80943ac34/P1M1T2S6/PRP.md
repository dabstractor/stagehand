---
name: "P1.M1.T2.S6 — DiffTree for 'what landed' reporting"
description: |
  Replace the `(*gitRunner).DiffTree` panic-stub (landed by P1.M1.T2.S1) with the real
  implementation — the read-only "what landed" report printed after a successful commit (PRD
  §9.9/FR42, Appendix C). Signature is fixed by the already-landed `Git` interface:
  `DiffTree(ctx, sha string, isRoot bool) ([]FileChange, error)`. It builds args
  `["diff-tree", "--no-commit-id", "--name-status", "-r"]`, conditionally appends `--root` when
  isRoot (so a root commit diffs against the empty tree — verified: a root commit WITHOUT `--root`
  produces NO output, the trap the `isRoot` parameter exists to avoid), appends `sha`, delegates to
  S1's `run()` helper (NOT `runWithInput` — diff-tree reads no stdin), and parses git's tab-separated
  output into `[]FileChange` via a new private `parseDiffTree` helper (2 fields → status+path;
  3 fields → status+src+dst for rename/copy). The command deliberately does NOT pass `-M` (rename
  detection): it reproduces commit-pi's exact UX verbatim (PRD Appendix C "Identical UX"), so renames
  surface as a D+A pair; `parseDiffTree` still handles 3-field R/C lines defensively and is exercised
  by a direct unit test. Branches err-first (infrastructural, unwrapped) then `code != 0` (bad SHA ⇒
  exit 128 ⇒ wrapped `"git diff-tree: failed"` error) then success (parse + return). Introduces ONE
  new private function (`parseDiffTree`) and ZERO new imports (`fmt`, `strings` already present in
  git.go). Adds ONE new test file `internal/git/difftree_test.go` (package git) covering: child
  commit A/M/D (real repo), root commit WITH `--root` (all A), root commit WITHOUT `--root` (empty —
  the core `isRoot` trap), no-change commit (empty), bad SHA (exit 128), git-missing, ctx-cancelled,
  plus two direct `parseDiffTree` unit tests (formats incl. R100/C90 3-field; empty input). Helpers
  use a `dt` prefix (`dtCommit`, `dtRemove`) to avoid name collisions with S4's (`setIdentityConfig`,
  `writeFile`, `stageFile`, `headSHA`, `commitMessage`) and S5's planned (`cas*`) helpers when all
  land together. Also removes the single `DiffTree` line from `git_test.go`'s `TestStubsPanic`
  (required consequence of making the method real — mirrors S2/S3/S4/S5). Touches ONLY
  `internal/git/`; no interface, struct, `run()`, `runWithInput`, `FileChange`, RevParseHEAD,
  WriteTree, CommitTree, or UpdateRefCAS changes. This is the LAST of the four real git plumbing
  methods in milestone P1.M1.T2 (S2–S5); the remaining 6 interface methods are T3 (diff capture,
  log queries, staging).
---

## Goal

**Feature Goal**: Implement the fifth real git plumbing method on `*gitRunner` — `DiffTree` — the
read-only report of "what landed" in a commit. After the snapshot-based flow publishes a commit via
`UpdateRefCAS` (P1.M1.T2.S5), the orchestrator (P1.M3.T4) calls
`changes, _ := g.DiffTree(ctx, newSHA, isRoot)` to obtain the file-level change set, which the CLI
layer (P1.M4.T1) prints as `[<sha>] <subject>` followed by the file list (PRD §9.9/FR42: *"On
success, print `[<short-sha>] <subject>` and `git diff-tree --no-commit-id --name-status -r
<NEW_SHA>` so the user sees what landed."*). It runs
`git -C <repo> diff-tree --no-commit-id --name-status -r [--root] <sha>` and parses the tab-separated
output into `[]FileChange{Status, SrcPath, Path}`. For a root commit (no parent), `isRoot=true`
appends `--root` so git diffs against the empty tree (every file shows as `A`); without `--root` a
root commit produces **no output** (verified empirically on git 2.54.0) — the trap the `isRoot`
parameter exists to avoid.

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: (a) replace the `DiffTree` panic-stub body with the ~12-line body
   that delegates to `run()` and branches `err`-first then `code != 0` then success (exact body in
   §Blueprint); (b) add the private `parseDiffTree(out string) []FileChange` helper immediately after
   it. NO import changes — `fmt`, `strings` are both already imported; `FileChange` is already
   declared (S1).
2. **CREATE** `internal/git/difftree_test.go` (`package git`): nine test functions
   (`TestDiffTree_ChildCommit`, `TestDiffTree_RootCommit_WithRootFlag`,
   `TestDiffTree_RootCommit_WithoutRootFlag`, `TestDiffTree_NoChanges`, `TestDiffTree_BadSHA`,
   `TestDiffTree_GitBinaryMissing`, `TestDiffTree_ContextCancelled`, `TestParseDiffTree_Formats`,
   `TestParseDiffTree_Empty`) plus two `dt`-prefixed fixture helpers (`dtCommit`, `dtRemove`).
3. **MODIFY** `internal/git/git_test.go`: remove the single `assertPanics(t, "DiffTree", …)` line from
   `TestStubsPanic` (required now that DiffTree is real — the test would otherwise fail expecting a
   panic that no longer occurs; mirrors S2/S3/S4/S5).

No other files touched. No new dependencies. `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 9 new cases passing (plus S1's `run()` tests, S2's
`RevParseHEAD` tests, S3's `WriteTree` tests, S4's `CommitTree` tests, S5's `UpdateRefCAS` tests, and
the now-trimmed `TestStubsPanic` all still green); on a child commit, `DiffTree` returns the parsed
A/M/D changes with correct statuses and paths; on a root commit with `isRoot=true`, every file is `A`;
on a root commit with `isRoot=false`, the result is **empty** (no error — proving `--root` matters);
on a no-change commit (identical to parent), the result is empty; on a bad SHA (all-zeros), it returns
an error whose message contains `"git diff-tree: failed"` and `"(exit 128)"`; `parseDiffTree` parses
synthetic 2-field and 3-field (R100/C90) lines correctly and returns an empty slice for empty input;
a missing git binary surfaces as a non-nil error mentioning "git binary not found"; `run()`,
`runWithInput`, the `Git` interface, `FileChange`, `RevParseHEAD`, `WriteTree`, `CommitTree`, and
`UpdateRefCAS` are byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary caller), and — transitively —
the CLI success-report UX (P1.M4.T1.S2) that prints `[<short-sha>] <subject>` + the file list.

**Use Case**: After `UpdateRefCAS` (P1.M1.T2.S5) atomically publishes `newSHA` to HEAD, the
orchestrator reports what landed: `changes, _ := g.DiffTree(ctx, newSHA, isRoot)`. `isRoot` is the
same `isUnborn` flag captured before generation by `RevParseHEAD` (S2) — equivalently, true iff the
`parents` slice passed to `CommitTree` (S4) was empty. The orchestrator does NOT handle this error
strictly (the commit already landed; a diff-tree failure is best-effort cosmetic), but for v1 it
propagates non-nil errors like any other git call. The returned `[]FileChange` feeds the CLI's
"what landed" printout.

**User Journey**: `g := git.New(repoPath)` → `parent, isUnborn, _ := g.RevParseHEAD(ctx)` → … →
`newSHA, _ := g.CommitTree(ctx, tree, parents, msg)` →
`_ = g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)` →
`changes, _ := g.DiffTree(ctx, newSHA, isUnborn)` → (CLI prints `[<short-sha>] <subject>` then each
`change.Status` + `change.Path`).

**Pain Points Addressed**: Closure of the snapshot flow — without "what landed", the user sees only a
SHA and a subject, not which files changed. `git diff-tree --name-status` is the canonical, parseable,
porcelain-free way to show that (PRD Appendix C: "Identical UX" to commit-pi). Doing it via a typed
`[]FileChange` (not a raw string) lets the CLI format/color it consistently later without re-parsing.

## Why

- **PRD §9.9 / FR42 (Commit creation):** *"On success, print `[<short-sha>] <subject>` and
  `git diff-tree --no-commit-id --name-status -r <NEW_SHA>` so the user sees what landed."* This
  method IS that diff-tree call, surfaced as a typed `[]FileChange`.
- **PRD Appendix C (Line-by-line porting map):** the row *"`git diff-tree --name-status` success print
  | main.go | Identical UX"* — the command must be byte-identical to commit-pi's. That is why the
  implementation does NOT add `-M` (rename detection): the PRD pins the exact command (decision D1).
- **PRD §11.1 / §13 (High-level data flow / core IP):** step 7 of the atomic sequence
  (`git_plumbing_summary.md`) is `git diff-tree --no-commit-id --name-status -r [--root] NEW → "what
  landed"`. This is the read-only, ref-safe tail of the flow (it mutates nothing; runs AFTER HEAD has
  already advanced).
- **PRD §13.5 (Edge cases — rootless repo):** a root commit has no parent to diff against. With
  `--root`, git diffs against the empty tree and shows every file as `A`; without `--root`, it prints
  nothing (verified). The `isRoot` parameter is the structural encoding of this edge — the caller
  already knows root-ness from `RevParseHEAD`'s `isUnborn` (decision D2).
- **Foundation for P1.M3.T4 / P1.M4.T1:** the `CommitStaged` orchestrator (P1.M3.T4) calls this method
  after `UpdateRefCAS`; the CLI default action (P1.M4.T1.S2) renders the result as the success report.
  Both are blocked on a correct `DiffTree`. This is the fifth of the 11 interface methods to leave
  stub-status (and the LAST in P1.M1.T2; the remaining 6 — StagedDiff, HasStagedChanges,
  RecentMessages, RecentSubjects, CommitCount, AddAll — are milestone P1.M1.T3).

## What

`DiffTree` builds `diff-tree --no-commit-id --name-status -r [--root] <sha>`, delegates to `run()`,
and translates the four-tuple into `([]FileChange, error)`:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `nil, err` unchanged. (DiffTree does not define a sentinel error — there is no
  caller-actionable category here; a bad SHA is a programmer error, not a race to recover from.)
- `exitCode != 0` (128 on git 2.x — bad SHA / not-a-valid-object) → return
  `nil, fmt.Errorf("git diff-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))`.
- `exitCode == 0` → return `parseDiffTree(stdout), nil`. This covers THREE distinct "empty output"
  cases that are all success: a child commit with changes (parsed), a root commit with `--root`
  (parsed, all `A`), a root commit WITHOUT `--root` (empty → nil slice), and a no-change commit
  (empty → nil slice). Empty output is NOT an error.

`parseDiffTree` splits the trimmed output on `\n`, skips empty lines, and splits each line on `\t`:
2 fields → `FileChange{Status, Path}`; 3 fields → `FileChange{Status, SrcPath, Path}` (rename/copy);
any other field count is skipped defensively (git output is well-formed, so this never fires in
practice).

No porcelain, no `git show`, no `-M`/`-C` (rename/copy detection), no SHA-format validation in
production code, no re-reading of HEAD.

### Success Criteria

- [ ] `(*gitRunner).DiffTree` body matches §Implementation Blueprint verbatim (no `panic`); delegates
      to `run()` (NOT `runWithInput`); branches `err`-first, then `code != 0`, then success.
- [ ] The args slice is EXACTLY `["diff-tree", "--no-commit-id", "--name-status", "-r"]`, with
      `"--root"` appended IFF `isRoot`, then `sha` appended last. NO `-M`/`-C`/`--find-renames`.
- [ ] The private `parseDiffTree(out string) []FileChange` function exists, immediately after
      `DiffTree`, with the field-routing logic above (2-field, 3-field, skip-otherwise).
- [ ] NO new imports in git.go (`fmt`, `strings` already present; `FileChange` already declared).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS` (real from S5),
      and the other 6 method stubs (StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects,
      CommitCount, AddAll) are byte-identical to their landed forms.
- [ ] `internal/git/difftree_test.go` exists in `package git` with the 9 named test functions and the
      2 `dt`-prefixed helpers.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `DiffTree` line (removed; 6 stubs
      remain: StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all 9 new cases pass and S1/S2/S3/S4/S5's tests still
      pass.
- [ ] `TestDiffTree_RootCommit_WithoutRootFlag` asserts an EMPTY result (proving `--root` is
      necessary for root commits — the core trap).
- [ ] `TestDiffTree_BadSHA` asserts the error message contains `"git diff-tree: failed"` and
      `"(exit 128)"`.
- [ ] `TestParseDiffTree_Formats` exercises the 3-field R100/C90 path (NOT reachable via the
      production command — tested directly on `parseDiffTree`).
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact three files to touch (and
the exact single line to remove from `git_test.go`); the exact `run()` contract (signature + the
`err==nil`-for-non-zero-exits invariant that the `code != 0` branch relies on); the exact `DiffTree`
body and the exact `parseDiffTree` body (verified-equivalent to throwaway invocations run against
git 2.54.0); the empirically-pinned `diff-tree` behavior (child ⇒ A/M/D 2-field lines; root+`--root`
⇒ all A; root-without-`--root` ⇒ empty; no-change ⇒ empty; bad SHA ⇒ exit 128; default rename = D+A
pair, `-M` = R100 3-field — deliberately NOT used); the octal-quoted-path gotcha (`core.quotepath`,
documented as a v1 limitation, NOT addressed); the exact 9 test cases with verified assertions and
the 2 helpers; and the exact validation commands with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.9/FR42 (on success, print [<short-sha>] <subject> + diff-tree --no-commit-id --name-status
        -r <NEW_SHA>); §13/§11.1 (the atomic snapshot sequence; diff-tree is the read-only step-7
        report AFTER update-ref has already advanced HEAD); §13.5 (rootless repo edge — a root commit
        has no parent to diff against); Appendix C (the diff-tree success print is 'Identical UX' to
        commit-pi — the command must be byte-for-byte the same, hence NO -M)."
  critical: "This subtask owns ONLY the DiffTree body + parseDiffTree helper + its tests + the one-line
             TestStubsPanic edit. Do NOT implement StagedDiff/HasStagedChanges/RecentMessages/
             RecentSubjects/CommitCount/AddAll (those are P1.M1.T3), the orchestrator (P1.M3.T4), the
             CLI success-report rendering (P1.M4.T1), or any other method. Do NOT change the Git
             interface or FileChange (already correct from S1). Do NOT modify run()/runWithInput."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Atomic Commit Sequence (step 7 = diff-tree --no-commit-id --name-status -r [--root] NEW →
        'what landed') and the Exit-Code Cheat Sheet row for diff-tree ('(always) / — / bad sha / use
        --root for root commits'). Also the 'diff-tree (what landed)' Go pattern block."
  critical: "Confirms: (1) -r and --no-commit-id and --name-status are ALWAYS used; (2) --root is the
             root-commit flag; (3) parse = split on \\t, 2 fields = status+path, 3 fields = status+src+dst.
             The summary's diff-tree Go pattern is the direct ancestor of this implementation."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§7 (git diff-tree --no-commit-id --name-status -r <sha> — 'what landed') documents: the flags
        (-r recurse, --no-commit-id suppress SHA line, --name-status tab-separated), the status codes
        (A/M/D/R/C/T/U; R/C carry a similarity score like R90), the stable parse format, the --root
        rule for root commits, and a reference Go pattern (FileChange{Status,Path,SrcPath} + switch on
        len(fields) case 2/3)."
  critical: "§7 is THE spec for the parser. Its Go pattern (switch len(f) case 2/case 3/default continue)
             is what parseDiffTree implements. Confirms a root commit WITHOUT --root produces no output."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything DiffTree consumes: the gitRunner struct; the run() helper (exact
        signature and verified body — which this subtask does NOT modify, only calls); the DiffTree
        panic-stub being replaced; the Git interface (signature already correct: ctx, sha string,
        isRoot bool) ([]FileChange, error); the FileChange type (Status/SrcPath/Path — already
        declared); New(); the git_test.go initRepo(t,dir) helper; and the assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero exit
             is not a Go error) is the foundation DiffTree's `code != 0` branch relies on."

- docfile: plan/001_f1f80943ac34/P1M1T2S4/PRP.md
  why: "The CONTRACT for the IMMEDIATE upstream producer: CommitTree returns the NEW_SHA that DiffTree
        reports on, and the orchestrator derives isRoot from whether the parents slice it passed to
        CommitTree was empty (len(parents)==0 ⟺ root). S4 also introduced runWithInput + the io import
        (both landed) — DiffTree does NOT use runWithInput (diff-tree reads no stdin), confirming the
        delegation target is run()."
  critical: "S4 is landed. S4's git.go edit is the CommitTree body + runWithInput + the io import;
             S6's git.go edits are (a) the DiffTree body, (b) parseDiffTree — DISTINCT regions. S4's
             git_test.go edit removed the CommitTree line; S6's removes the DiffTree line — distinct
             lines. S4's test helpers (setIdentityConfig, writeFile, stageFile, writeTreeOf, headSHA,
             commitMessage) are REUSED by S6 (not redeclared) — S6's new helpers use a `dt` prefix
             (dtCommit, dtRemove) to stay collision-free."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The CONTRACT for the upstream that produces isRoot: RevParseHEAD returns parentSHA + isUnborn.
        The orchestrator passes isUnborn as DiffTree's isRoot. S2's revparse_test.go defines minGitEnv()
        and makeEmptyCommit(t,dir,msg), which DiffTree's tests reuse without redeclaring. S2 also
        established the err-first / code-branch method pattern that DiffTree mirrors."
  critical: "S2's Level-2 note documents the EXACT TestStubsPanic-edit pattern (remove the now-real
             method's line) — apply it for DiffTree."

- docfile: plan/001_f1f80943ac34/P1M1T2S5/PRP.md
  why: "The CONTRACT for the immediately-preceding step: UpdateRefCAS publishes newSHA to HEAD, and
        ONLY AFTER it succeeds does the orchestrator call DiffTree (the report is post-publish,
        read-only). Confirms S5's git.go edit (ErrCASFailed var + UpdateRefCAS body) and git_test.go
        edit (remove UpdateRefCAS line) are DISTINCT from S6's edits. S5's test helpers (casCommit,
        casHEAD, casMoveHEAD, casOut, gitIdentityEnv) are name-collision risks — S6's `dt` prefix
        avoids them."
  critical: "S5 lands concurrently. S5 and S6 both edit git.go (distinct regions: S5 = UpdateRefCAS +
             ErrCASFailed near the other commit methods; S6 = DiffTree + parseDiffTree further down)
             and both edit git_test.go's TestStubsPanic (distinct lines: UpdateRefCAS vs DiffTree).
             On the disk snapshot, git_test.go's TestStubsPanic already lists 7 stubs (DiffTree,
             StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll) — S6
             removes ONLY the DiffTree line, leaving 6. No text overlap with S5's edit."

- docfile: plan/001_f1f80943ac34/P1M1T2S6/research/difftree_validation.md
  why: "THIS subtask's own research: the signature reconciliation (§1, none needed), the
        empirically-pinned diff-tree behavior on git 2.54.0 incl. the root-with/without---root trap
        and the empty-output cases (§2), the -M rename-detection decision D1 (§3), the parse contract
        (§3), the octal-quoted-path gotcha (§4), the parseDiffTree factoring decision D4 (§5), the
        isRoot/caller-driven decision D2 (§6), the test design matrix (§7), the helper-name collision
        avoidance with S4/S5 (§8, decision D3), and the decisions log D1–D7 (§9)."
  critical: "§2 (root-without---root = empty; bad SHA = exit 128) and §3 (the -M decision + parse
             rule) are the two non-obvious calls an implementing agent would otherwise guess at. §8
             (dt-prefix naming) is what keeps the package compiling when S5 and S6 land together."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-diff-tree#_description
  why: "Documents that git diff-tree walks the commit/tree pairs and, with --name-status, prints
        '<status>\\t<path>' (and '<status>\\t<src>\\t<dst>' for renames/copies). Confirms -r recurses
        into subtrees and --no-commit-id suppresses the leading commit line."
  critical: "Establishes the output contract parseDiffTree depends on: tab-separated, one line per
             changed path, 2 fields for A/M/D/T, 3 fields for R/C."
- url: https://git-scm.com/docs/git-diff-tree#Documentation/git-diff-tree.txt---root
  why: "Documents --root: 'Show the commit ... as a diff against an empty tree ... when the commit is
        a root commit.' This is EXACTLY why DiffTree needs the isRoot parameter — without --root a root
        commit emits nothing."
  critical: "Confirms the --root semantics empirically re-verified in research §2.2: root+--root ⇒ all
             files as A; root-without---root ⇒ no output."
- url: https://git-scm.com/docs/git-diff-tree#Documentation/git-diff-tree.txt--no-commit-id
  why: "Documents --no-commit-id: suppresses the commit SHA line that diff-tree prints first by
        default. Always required so the output is purely the file list (else the first line would be
        the SHA and parseDiffTree would mis-route it as a 1-field 'line' and skip it — harmless but
        wrong)."
  critical: "--no-commit-id is mandatory; without it the first output line is the bare commit SHA
             (no tab), which parseDiffTree would skip — silently dropping nothing important but
             masking a misconfiguration."
- url: https://git-scm.com/docs/git-config#Documentation/git-config.txt-corequotepath
  why: "Documents core.quotepath (default true): non-ASCII path components are quoted with octal C-
        style escapes AND the whole path is wrapped in double quotes. This is the octal-quoted-path
        gotcha (research §4 / gotcha G6)."
  critical: "Explains why a path like spécial.txt appears as \"sp\\303\\251cial.txt\" in diff-tree
             output under the default config. v1 does NOT pass -c core.quotepath=false (would diverge
             from the PRD command); the tab-split still works because the quotes are part of the path
             field, not a separator."
- url: https://pkg.go.dev/strings#Split
  why: "strings.Split(s, sep) splits on every occurrence of sep; strings.Split(\"\", \"\\n\") returns
        [\"\"] (a one-element slice with an empty string), which is why parseDiffTree skips empty
        lines. strings.TrimSpace removes the trailing newline so the final empty segment is the only
        one and is skipped."
  critical: "The TrimSpace-before-Split + skip-empty-line guard is what makes empty output (root-
             without---root, no-change commit) yield a clean nil slice instead of a spurious empty
             FileChange."
```

### Current Codebase Tree (after S1 + S2 + S3 + S4 have landed; S5 in flight — verified on disk)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface+gitRunner+run()+New()+FileChange+stubs; S2: RevParseHEAD real;
│       │                 #   S3: WriteTree real; S4: runWithInput+CommitTree real+io import.
│       │                 #   S5 (in flight): ErrCASFailed + UpdateRefCAS real.
│       │                 # imports: bytes, context, errors, fmt, io, os/exec, strings  ← ALL S6 needs present
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic + initRepo + assertPanics
│       │                 #   TestStubsPanic currently lists 7 stubs: DiffTree, StagedDiff,
│       │                 #   HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll
│       ├── revparse_test.go  # S2: 4 TestRevParseHEAD_* + minGitEnv + makeEmptyCommit
│       ├── writetree_test.go # S3: 5 TestWriteTree_* + makeMergeConflict
│       ├── committree_test.go # S4: setIdentityConfig/writeFile/stageFile/writeTreeOf/headSHA/commitMessage + 6 tests
│       └── updateref_test.go  # S5 (landing concurrently): cas*/gitIdentityEnv + 6 TestUpdateRefCAS_*
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go              # MODIFIED — DiffTree stub → real body; +private parseDiffTree. NO import change.
        ├── git_test.go         # MODIFIED — remove the ONE `DiffTree` line from TestStubsPanic
        ├── revparse_test.go    # UNCHANGED (S2's file; minGitEnv/makeEmptyCommit reused, not edited)
        ├── writetree_test.go   # UNCHANGED (S3's file)
        ├── committree_test.go  # UNCHANGED (S4's file; helpers reused, not redeclared)
        ├── updateref_test.go   # UNCHANGED (S5's file; landing concurrently — distinct `cas`/`dt` prefixes)
        └── difftree_test.go    # NEW — package git; 7 TestDiffTree_* + 2 TestParseDiffTree_* + dtCommit/dtRemove
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (1) Replace the `DiffTree` panic-stub with the `run()`-delegating body. (2) Add the private `parseDiffTree(out string) []FileChange` helper immediately after. No import changes. |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "DiffTree", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/difftree_test.go` | CREATE | `package git` tests for `DiffTree` + `parseDiffTree`: child A/M/D, root with/without `--root`, no-change, bad SHA, git-missing, ctx-cancelled, parse formats (incl. R100/C90), parse empty, plus the `dt`-prefixed fixtures. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), `UpdateRefCAS`/`ErrCASFailed` (S5), the other 6 method stubs (StagedDiff, HasStagedChanges,
RecentMessages, RecentSubjects, CommitCount, AddAll), `revparse_test.go` (S2), `writetree_test.go`
(S3), `committree_test.go` (S4), `updateref_test.go` (S5), `go.mod`/`go.sum`, the `Makefile`,
anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — a root commit WITHOUT --root produces NO output, NOT an error): empirically (git
// 2.54.0, research §2.2), `git diff-tree --no-commit-id --name-status -r <root-sha>` exits 0 with
// EMPTY stdout. Only WITH `--root` does it diff against the empty tree and emit every file as `A`.
// This is THE trap the `isRoot` parameter exists to avoid. The caller (orchestrator) MUST pass
// isRoot=true for a root commit (it knows root-ness from RevParseHEAD's isUnborn / CommitTree's
// empty parents). DiffTree appends `--root` iff isRoot. TestDiffTree_RootCommit_WithoutRootFlag
// guards this by asserting an empty (not errored) result when isRoot is wrongly false.
// (Decision D2; git-diff-tree docs #--root.)

// CRITICAL (G2 — branch on `code != 0`, and the ONLY non-zero case is a bad SHA = exit 128): the
// cheat-sheet row for diff-tree says "(always) / — / bad sha". Verified: a valid SHA always exits 0
// (even for empty-output cases); an all-zeros / non-existent SHA exits 128 ("fatal: bad object").
// So `code != 0` ⟺ git could not resolve the SHA. Branch on `code != 0` (stable), NOT `code == 128`
// (matches the established S2/S3/S4/S5 method pattern; decision D5). The message embeds the actual
// code for diagnostics.

// CRITICAL (G3 — do NOT add -M / --find-renames to the command): PRD §9.9/FR42 pins the EXACT command
// `git diff-tree --no-commit-id --name-status -r <NEW_SHA>`, and Appendix C requires "Identical UX"
// to commit-pi. Verified (research §2.3): with the default (no -M), a rename surfaces as a D+A pair
// (both 2-field lines); with -M it collapses to a single `R100\told\tnew` 3-field line. The PRD
// command omits -M, so renames show as D+A. parseDiffTree STILL handles 3-field R/C lines defensively
// (so a future caller adding -M, or a git default change, is already supported), and the 3-field path
// is exercised directly via TestParseDiffTree_Formats. Do NOT add -M/-C/--find-renames. (Decision D1.)

// CRITICAL (G4 — delegate to run(), NOT runWithInput): diff-tree reads nothing from stdin (unlike
// commit-tree's -F - message). So DiffTree calls g.run(ctx, g.workDir, args...). runWithInput (S4)
// exists solely to feed commit-tree's stdin. Using it here would be semantically wrong (nil reader)
// and wasteful. (Mirrors S5's run()-not-runWithInput decision.)

// CRITICAL (G5 — empty output is a SUCCESS, return nil slice): THREE success cases produce empty
// stdout: (a) a root commit without --root [shouldn't happen if caller sets isRoot correctly, but is
// not an error], (b) a commit identical to its parent (no changes), (c) a child commit that happened
// to change nothing. parseDiffTree("") returns a nil slice (len 0). DiffTree returns (nil-slice, nil).
// Do NOT treat empty output as an error. TestDiffTree_NoChanges and TestDiffTree_RootCommit_Without
// RootFlag guard this.

// GOTCHA (G6 — octal-quoted non-ASCII paths, core.quotepath): by default git wraps non-ASCII path
// components in double quotes with octal C-style escapes (e.g. spécial.txt → "sp\303\251cial.txt").
// Verified (research §4). The tab-split STILL works (the quotes are part of the path field, not a
// separator), so parseDiffTree is unaffected, but the Path field will contain the quoted form. This
// is a KNOWN v1 limitation: the production command must NOT pass `-c core.quotepath=false` (would
// diverge from the PRD command / "Identical UX"). Do NOT add config flags to "fix" this. (Decision
// D6; git-config docs #corequotepath.)

// GOTCHA (G7 — the TestStubsPanic edit): git_test.go's TestStubsPanic (after S2+S3+S4, and with S5's
// UpdateRefCAS line removed) STILL includes
//   assertPanics(t, "DiffTree", func() { _, _ = g.DiffTree(ctx, "sha", false) }).
// Once DiffTree is real (no panic), assertPanics fails with "expected panic, but did not panic".
// Resolution (mirrors S2/S3/S4/S5): DELETE that one line. After removal, TestStubsPanic covers the
// remaining 6 stubs (StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount,
// AddAll). This is the ONLY edit to git_test.go. Document it in the commit message.

// GOTCHA (G8 — ZERO new imports): DiffTree uses g.run (S1), fmt.Errorf (fmt — present),
// strings.Split/TrimSpace (strings — present). FileChange is already declared in git.go (S1).
// git.go's import block (bytes, context, errors, fmt, io, os/exec, strings) already has everything.
// Do NOT add an import. The compiler will complain about an unused import if you add one erroneously.
// parseDiffTree uses strings only (already imported).

// GOTCHA (G9 — test helper names must not collide with S4's or S5's): S4 (committree_test.go) defines
// setIdentityConfig, writeFile, stageFile, writeTreeOf, headSHA, commitMessage. S5 (updateref_test.go,
// landing concurrently) plans casCommit, casHEAD, casMoveHEAD, casOut, gitIdentityEnv. S6's NEW
// helpers use a `dt` prefix: dtCommit, dtRemove. All DISTINCT — the package compiles when S5 and S6
// land together. REUSE (do not redeclare) initRepo (git_test.go), minGitEnv and makeEmptyCommit
// (revparse_test.go), writeFile/stageFile/headSHA (committree_test.go). (Research §8, decision D3.)

// GOTCHA (G10 — creating commits with file changes in a fixture): the existing makeEmptyCommit (S2)
// does `git commit --allow-empty` (no file changes). To produce A/M/D entries in a real commit-tree
// output, fixtures must commit ACTUAL file changes: writeFile + stageFile for A/M, `git rm` for D.
// The dtCommit helper runs `git commit -m <msg>` with identity env (mirrors makeEmptyCommit's env
// pattern, minus --allow-empty) so staged changes are committed and HEAD advances. dtRemove runs
// `git rm -q <name>` to stage a deletion. These are test-fixture usages of porcelain (acceptable —
// the Level-1 grep for sh -c / cmd.Dir covers PRODUCTION code (git.go) only).

// GOTCHA (G11 — parseDiffTree is a SEPARATE private function, not inline): the 3-field rename/copy
// parse path is NOT reachable via the production command (no -M). To test it, parseDiffTree must be
// callable directly → it is a package-private function exercised by TestParseDiffTree_Formats with a
// synthetic 5-line input. This separates the "exec git" concern (tested via a real repo) from the
// "parse bytes" concern (tested via strings). (Decision D4.)

// GOTCHA (G12 — test file is package git, white-box): DiffTree is on *gitRunner and parseDiffTree is
// package-private; the fixtures reuse unexported helpers. The test MUST be `package git`. Match
// S1/S2/S3/S4/S5's package (carried from S1 G9).

// GOTCHA (G13 — no shell, no cmd.Dir in PRODUCTION code): DiffTree inherits S1's §19 guarantees
// (run() uses exec.CommandContext + []string args + -C repo flag, NOT cmd.Dir / os.Chdir). Do NOT
// introduce exec.Command / os.Chdir / sh -c in git.go. The test fixtures DO use exec.Command directly
// (parallel to S1's initRepo, S2's makeEmptyCommit, S3's makeMergeConflict, S4's fixtures) — that is
// acceptable test-fixture usage ([]string args + cmd.Env, never a shell). The Level-1 grep for sh -c
// / cmd.Dir covers PRODUCTION code (git.go) only.

// GOTCHA (G14 — args ordering: --root BEFORE sha): build the slice as
//   ["diff-tree", "--no-commit-id", "--name-status", "-r", (maybe "--root"), sha]
// i.e. append "--root" (if isRoot) BEFORE appending sha. Verified: git accepts --root in either
// position, but keeping flags-then-positional matches the PRD command layout and is unambiguous.

// GOTCHA (G15 — parseDiffTree returns a nil slice for empty input, by design): `var changes
// []FileChange` starts nil; if no lines parse, it stays nil. Callers (the orchestrator / CLI) only
// `range` over the result, which is a no-op on nil. Do NOT pre-initialize to a non-nil empty slice
// (decision D7) — nil is the idiomatic Go "empty collection" and matches the git_plumbing_reference
// §7 reference pattern.
```

## Implementation Blueprint

### Data models and structure

No new types. `FileChange` is already declared by S1 (Status, SrcPath, Path string) and is the value
type this method returns a slice of. The ONE new symbol is a **private** function:

```go
// parseDiffTree parses the tab-separated output of `git diff-tree --no-commit-id --name-status -r`
// into FileChange values. Each non-empty line is one of:
//   "<status>\t<path>"               (A/M/D/T — 2 fields)
//   "<status><score>\t<src>\t<dst>"  (R/C — 3 fields, e.g. "R100\told.txt\tnew.txt")
// Empty lines (including the trailing newline after TrimSpace) are skipped. Lines with any other
// field count are skipped defensively (git output is well-formed, so this never fires in practice).
// Returns a nil slice for empty/whitespace-only input (range-safe).
func parseDiffTree(out string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 2:
			changes = append(changes, FileChange{Status: fields[0], Path: fields[1]})
		case 3:
			changes = append(changes, FileChange{Status: fields[0], SrcPath: fields[1], Path: fields[2]})
		}
	}
	return changes
}
```

No new options type, no new struct, no new error sentinel. `run()`'s return shape
`(stdout, stderr, exitCode, err)` is already defined by S1.

### The `DiffTree` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature. Place `parseDiffTree` immediately
after it.

```go
// DiffTree returns the file-level change set of commit sha versus its first parent — the "what
// landed" report printed after a successful commit (PRD §9.9/FR42, Appendix C). It runs
// `git diff-tree --no-commit-id --name-status -r [--root] <sha>` and parses the tab-separated output
// into []FileChange. For a root commit (no parent), isRoot MUST be true so git diffs against the
// empty tree via --root; otherwise a root commit yields NO output (verified on git 2.54.0: empty
// stdout, exit 0 — the trap the isRoot parameter exists to avoid). The command intentionally does NOT
// pass -M (rename detection): it reproduces commit-pi's exact `diff-tree --name-status` UX (PRD
// Appendix C "Identical UX"), so renames surface as a D+A pair; parseDiffTree still handles 3-field
// R/C lines defensively.
//
// diff-tree exits 128 only on a bad/unresolvable SHA (verified); that is surfaced via run()'s
// exitCode != 0 (err stays nil per run()'s invariant). Empty output (root-without---root, or a
// no-change commit) is exit 0 and yields a nil slice — NOT an error.
func (g *gitRunner) DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error) {
	args := []string{"diff-tree", "--no-commit-id", "--name-status", "-r"}
	if isRoot {
		args = append(args, "--root") // root commit: diff against the empty tree (G1)
	}
	args = append(args, sha) // flags first, then the positional SHA (G14)

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// Only a bad SHA reaches here (exit 128). Branch on code != 0, not code == 128 (G2).
		return nil, fmt.Errorf("git diff-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return parseDiffTree(stdout), nil // may be a nil slice for empty output (G5)
}
```

> **Verified:** the args shape, the `--root` conditional, and the branch order are confirmed by this
> subtask's research §2 (child ⇒ A/M/D; root+`--root` ⇒ all A; root-without-`--root` ⇒ empty;
> no-change ⇒ empty; bad SHA ⇒ exit 128), re-verified empirically on this box (git 2.54.0).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (two surgical edits — NO import change)
  - EDIT 1 — replace the DiffTree panic-stub:
      FIND the stub:
        func (g *gitRunner) DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error) {
            panic("gitRunner.DiffTree: not yet implemented — see P1.M1.T2.S6")
        }
      REPLACE with the body in §"The DiffTree body" above (keep the same signature, add the doc
      comment, delegate to run(), branch err-first then code != 0 then success, call parseDiffTree).
  - EDIT 2 — add the parseDiffTree helper:
      INSERT the `func parseDiffTree(out string) []FileChange { … }` declaration (with its doc
      comment, verbatim from §"Data models and structure") immediately AFTER the DiffTree method's
      closing brace and BEFORE the next stub (StagedDiff).
  - DO NOT touch: the import block (all needed symbols already present — gotcha G8), run(),
    runWithInput, New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD (real
    from S2), WriteTree (real from S3), CommitTree (real from S4), UpdateRefCAS/ErrCASFailed (real
    from S5), or any of the other 6 method stubs (StagedDiff, HasStagedChanges, RecentMessages,
    RecentSubjects, CommitCount, AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic:
      assertPanics(t, "DiffTree", func() { _, _ = g.DiffTree(ctx, "sha", false) })
  - DELETE that single line. After removal TestStubsPanic covers the remaining 6 stubs: StagedDiff,
    HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll.
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper,
    the other assertPanics lines).
  - WHY: once DiffTree is real it no longer panics; assertPanics would fail (gotcha G7). Mirrors
    S2/S3/S4/S5's removals.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (6 stubs still panic).

Task 3: CREATE internal/git/difftree_test.go (package git — white-box)
  - FILE: internal/git/difftree_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G12; matches the other test files)
  - IMPORTS: context, errors, os, os/exec, strings, testing  (all stdlib)
  - WRITE the fixture helpers (exact bodies below; all DISTINCT `dt`-prefixed names so they do not
    collide with S4's or S5's helpers when all land together — gotcha G9):
      dtCommit(t, dir, msg string):
        - env := append(minGitEnv(),
            "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
            "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com").
          REUSES S2's minGitEnv() (do NOT redeclare). Mirrors makeEmptyCommit's env pattern, but
          WITHOUT --allow-empty (so staged file changes are committed and HEAD advances; on an unborn
          repo this creates the ROOT commit).
        - cmd := exec.Command("git", "-C", dir, "commit", "-m", msg); cmd.Env = env.
        - on non-zero exit t.Fatalf("dtCommit failed: %v\n%s", err, out) with CombinedOutput.
          t.Helper().
      dtRemove(t, dir, name string):
        - cmd := exec.Command("git", "-C", dir, "rm", "-q", name).
        - on error t.Fatalf("dtRemove %s failed: %v\n%s", name, err, out) with CombinedOutput.
          t.Helper().  (Stages a deletion in the index; used to produce a D entry.)
  - WRITE the 7 DiffTree test functions (assertions in §"Test cases" below):
      TestDiffTree_ChildCommit:
        repo := t.TempDir(); initRepo(t, repo)
        // root commit with two files
        writeFile(t, repo, "keep.txt", "v1\n"); stageFile(t, repo, "keep.txt")   // REUSE S4 helpers
        writeFile(t, repo, "gone.txt", "v1\n"); stageFile(t, repo, "gone.txt")
        dtCommit(t, repo, "root")
        // child commit: modify keep.txt, delete gone.txt, add new.txt
        writeFile(t, repo, "keep.txt", "v2\n"); stageFile(t, repo, "keep.txt")   // M
        dtRemove(t, repo, "gone.txt")                                             // D
        writeFile(t, repo, "new.txt", "v1\n"); stageFile(t, repo, "new.txt")     // A
        dtCommit(t, repo, "child")
        child := headSHA(t, repo)                                                 // REUSE S4 helper
        g := New(repo)
        changes, err := g.DiffTree(context.Background(), child, false)            // isRoot=false (has parent)
        assert err == nil
        // build a set of "status\tpath" for easy assertion
        got := map[string]bool{}
        for _, c := range changes { got[c.Status+"\t"+c.Path] = true; c.SrcPath=="" /* 2-field */ }
        assert got["M\tkeep.txt"] && got["D\tgone.txt"] && got["A\tnew.txt"] && len(changes)==3
      TestDiffTree_RootCommit_WithRootFlag:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.txt", "x\n"); stageFile(t, repo, "a.txt")
        writeFile(t, repo, "b.txt", "y\n"); stageFile(t, repo, "b.txt")
        dtCommit(t, repo, "root")                       // creates the ROOT commit (unborn → root)
        root := headSHA(t, repo)
        g := New(repo)
        changes, err := g.DiffTree(context.Background(), root, true)   // isRoot=true → --root appended
        assert err == nil
        assert len(changes)==2 && all changes have Status=="A"
        assert paths {a.txt, b.txt} both present
      TestDiffTree_RootCommit_WithoutRootFlag:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.txt", "x\n"); stageFile(t, repo, "a.txt")
        dtCommit(t, repo, "root")
        root := headSHA(t, repo)
        g := New(repo)
        changes, err := g.DiffTree(context.Background(), root, false)  // isRoot=false → NO --root
        assert err == nil                              // empty output is NOT an error (G5)
        assert len(changes)==0                         // THE core trap: root-without---root = nothing (G1)
      TestDiffTree_NoChanges:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.txt", "x\n"); stageFile(t, repo, "a.txt")
        dtCommit(t, repo, "first")
        first := headSHA(t, repo)
        // a second commit that changes nothing (re-stage identical content via --allow-empty)
        makeEmptyCommit(t, repo, "noop")               // REUSE S2 helper (allow-empty child)
        noop := headSHA(t, repo)
        g := New(repo)
        changes, err := g.DiffTree(context.Background(), noop, false)
        assert err == nil && len(changes)==0           // empty output, exit 0 (G5)
      TestDiffTree_BadSHA:
        repo := t.TempDir(); initRepo(t, repo)
        g := New(repo)
        changes, err := g.DiffTree(context.Background(), "0000000000000000000000000000000000000000", false)
        assert err != nil
        assert strings.Contains(err.Error(), "git diff-tree: failed")
        assert strings.Contains(err.Error(), "(exit 128)")   // real exit (bad object) (G2)
        assert changes == nil
      TestDiffTree_GitBinaryMissing:
        t.Setenv("PATH", "")                           // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                          // dir need not be a repo
        changes, err := g.DiffTree(context.Background(), "sha", false)
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert changes == nil
      TestDiffTree_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call
        g := New(t.TempDir())
        changes, err := g.DiffTree(ctx, "sha", false)
        assert err != nil && errors.Is(err, context.Canceled)
        assert changes == nil
  - WRITE the 2 parseDiffTree unit test functions (direct, no git needed — exercises the 3-field path
    the production command cannot reach because it omits -M, gotcha G11):
      TestParseDiffTree_Formats:
        in := "A\tadded.txt\nM\tmod.txt\nD\tdel.txt\nR100\told.txt\tnew.txt\nC90\tsrc.txt\tdst.txt\n"
        got := parseDiffTree(in)
        assert len(got)==5
        assert got[0]==FileChange{Status:"A", Path:"added.txt"}
        assert got[1]==FileChange{Status:"M", Path:"mod.txt"}
        assert got[2]==FileChange{Status:"D", Path:"del.txt"}
        assert got[3]==FileChange{Status:"R100", SrcPath:"old.txt", Path:"new.txt"}   // 3-field rename
        assert got[4]==FileChange{Status:"C90", SrcPath:"src.txt", Path:"dst.txt"}    // 3-field copy
      TestParseDiffTree_Empty:
        assert len(parseDiffTree(""))==0               // nil/empty slice for empty input (G15)
        assert len(parseDiffTree("\n  \n"))==0          // whitespace-only also empty
  - NAMING: TestDiffTree_<Scenario>; TestParseDiffTree_<Scenario>; helpers dtCommit/dtRemove
    (distinct from S1/S2/S3/S4/S5 helpers — gotcha G9).
  - DO NOT redeclare initRepo / minGitEnv / makeEmptyCommit / writeFile / stageFile / headSHA (they
    live in git_test.go / revparse_test.go / committree_test.go).
  - VERIFY: go test -race -run 'TestDiffTree|TestParseDiffTree' ./internal/git/ → exit 0, all 9 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git grep -n 'panic.*DiffTree' internal/git/git.go                       (expect: no matches — stub gone)
  - RUN: git grep -n 'func parseDiffTree' internal/git/git.go                    (expect: exactly 1 match)
  - RUN: git grep -n 'func (g \*gitRunner) DiffTree' internal/git/git.go         (expect: exactly 1 match)
  - RUN: git grep -n 'runWithInput' internal/git/git.go | grep -i difftree       (expect: no matches — uses run())
  - RUN: git grep -nE '\-M\b|\-C\b|--find-renames' internal/git/git.go | grep -i diff-tree
        (expect: no matches — NO rename detection, G3)
  - RUN: git status --porcelain → expect EXACTLY:
        M internal/git/git.go
        M internal/git/git_test.go
        ?? internal/git/difftree_test.go
        (If updateref_test.go / other files appear as M/??, S5 landed concurrently and you are seeing
        its changes — coordinate; S6 itself touches only the three files above.)
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestDiffTree_ChildCommit` | root (keep.txt, gone.txt); child (M keep, D gone, A new) | `err==nil`; changes = {M→keep.txt, D→gone.txt, A→new.txt}; len 3 | the A/M/D 2-field parse path against a real repo (the primary UX, FR42) |
| `TestDiffTree_RootCommit_WithRootFlag` | root commit (a.txt, b.txt); `isRoot=true` | `err==nil`; len 2; both Status `A`; both paths present | `--root` appended → diffs against empty tree (§13.5 root edge) |
| `TestDiffTree_RootCommit_WithoutRootFlag` | root commit; `isRoot=false` | `err==nil`; **len 0** | **THE core trap**: root-without-`--root` = nothing; `isRoot` matters (G1) |
| `TestDiffTree_NoChanges` | no-change child commit (allow-empty) | `err==nil`; len 0 | empty output is success, not error (G5) |
| `TestDiffTree_BadSHA` | all-zeros SHA | err contains `"git diff-tree: failed"` + `"(exit 128)"`; nil slice | bad-object → exit 128 → wrapped error (G2) |
| `TestDiffTree_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found"; nil slice | infrastructural failure surfaces cleanly |
| `TestDiffTree_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; nil slice | ctx.Err() propagated |
| `TestParseDiffTree_Formats` | synthetic 5-line string (A/M/D/R100/C90) | 5 FileChange; R100 & C90 carry SrcPath | the 3-field rename/copy parse path (NOT reachable via production command — needs -M; tested directly, G11) |
| `TestParseDiffTree_Empty` | `""` and `"\n  \n"` | `len==0` | empty/trailing-newline handling (G15) |

### Implementation Patterns & Key Details

```go
// === Why args are flags-first, --root-conditional, sha-last (G14) ===
// args := []string{"diff-tree", "--no-commit-id", "--name-status", "-r"}
// if isRoot { args = append(args, "--root") }
// args = append(args, sha)
// git accepts --root before or after the positional, but flags-then-positional matches the PRD
// command layout and is unambiguous. The three always-on flags (-r, --no-commit-id, --name-status)
// are the EXACT set PRD §9.9/FR42 specifies; adding or removing any is a divergence.

// === Why branch on `code != 0` (not `code == 128`) (G2) ===
// Verified (research §2.2): a valid SHA ALWAYS exits 0 (even when output is empty); the ONLY non-zero
// case is an unresolvable SHA (exit 128). So `code != 0` ⟺ "git could not resolve the SHA". Branching
// on `code != 0` is stable across versions and matches the established S2/S3/S4/S5 method pattern.
// The message embeds the actual code ("(exit 128)") so a future change is visible in diagnostics.

// === Why NO -M (G3) ===
// PRD §9.9/FR42 + Appendix C ("Identical UX") pin the EXACT command. -M would collapse a D+A rename
// into an R100 line — a DIFFERENT UX than commit-pi. So renames show as D+A. parseDiffTree still
// handles 3-field R/C lines (defensive + unit-tested) so the parser is future-proof, but the
// production command never emits them.

// === Why run() and not runWithInput (G4) ===
// diff-tree reads nothing from stdin (unlike commit-tree's -F - message). runWithInput (S4) exists
// solely to feed stdin. Using it here would be semantically wrong (nil reader) and wasteful. run() is
// the correct, already-landed helper. (Mirrors S5.)

// === Why empty output is success (G5) ===
// A root commit without --root, a no-change commit, and (pathologically) a child that changed nothing
// all produce empty stdout + exit 0. parseDiffTree("") returns nil. DiffTree returns (nil, nil). Do
// NOT invent an "ErrNoChanges" — there is no caller-actionable category here (the commit already
// landed; "what landed" is best-effort cosmetic).

// === Why parseDiffTree is split out (G11) ===
// The 3-field rename/copy path cannot be exercised through DiffTree (production command omits -M).
// A separate private parseDiffTree is directly unit-testable with a synthetic string, cleanly
// separating the exec concern (real repo) from the parse concern (strings). (Decision D4.)

// === Why nil slice for empty (G15) ===
// `var changes []FileChange` stays nil if nothing appends. range over nil is a no-op. Idiomatic Go;
// matches the git_plumbing_reference §7 reference pattern. Do not pre-make a non-nil empty slice.

// === Why dt-prefixed helper names (G9) ===
// S4 declared writeFile/stageFile/headSHA/commitMessage/setIdentityConfig/writeTreeOf; S5 plans
// cas*/gitIdentityEnv. If S6 reused those NAMES it would be a redeclaration compile error. The `dt`
// prefix (dtCommit, dtRemove) is collision-free. REUSE (don't redeclare) initRepo/minGitEnv/
// makeEmptyCommit/writeFile/stageFile/headSHA.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, errors, fmt, strings, os/exec, testing.T.TempDir/Setenv all available
  - deps: NONE added (fmt/strings already imported; test helpers use only stdlib)

INTERNAL/GIT PACKAGE (modified here):
  - NEW private function: `parseDiffTree(out string) []FileChange` (package-private; tested white-box).
  - NEW real method: `(*gitRunner).DiffTree` (satisfies the already-declared Git interface method).
  - NO new exported symbols (FileChange is already exported from S1; DiffTree was already in the
    interface). This is the first method in the milestone with NO new exported symbol.

CALLERS (future, NOT built here — documented so the contract is clear):
  - P1.M3.T4 (CommitStaged orchestrator): after `g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)`
    succeeds, `changes, _ := g.DiffTree(ctx, newSHA, isUnborn)`; passes `changes` to the success report.
    (isUnborn comes from the RevParseHEAD call at the top of the flow; equivalently, true iff the
    parents slice passed to CommitTree was empty.)
  - P1.M4.T1.S2 (CLI default action): renders `[<short-sha>] <subject>` then each FileChange
    (Status + Path; optionally SrcPath for R/C).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation - fix before proceeding
gofmt -w internal/git/git.go internal/git/difftree_test.go   # format the new/edited files
go vet ./internal/git/                                        # vet the package

# Project-wide validation
go build ./...
go vet ./...
gofmt -l internal/git/

# Expected: Zero errors and empty gofmt -l output. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new components in isolation
go test -race -run 'TestDiffTree|TestParseDiffTree' ./internal/git/ -v

# The stubs test (must still pass with the DiffTree line removed — 6 stubs remain)
go test -race -run TestStubsPanic ./internal/git/ -v

# Full package suite (S1 run() + S2 RevParseHEAD + S3 WriteTree + S4 CommitTree + S5 UpdateRefCAS + S6 DiffTree)
go test -race ./internal/git/ -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
# Specifically:
#  - TestDiffTree_ChildCommit must show {M→keep.txt, D→gone.txt, A→new.txt}.
#  - TestDiffTree_RootCommit_WithoutRootFlag must show len(changes)==0 (the isRoot trap).
#  - TestDiffTree_BadSHA must show err containing "(exit 128)".
#  - TestParseDiffTree_Formats must show the R100/C90 3-field entries with SrcPath populated.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (no main changes expected — just confirms the package compiles in-tree)
go build ./...

# Manual end-to-end diff-tree sanity check against a throwaway repo (mirrors research §2):
TMP=$(mktemp -d); cd "$TMP"; git init -q; git config user.name T; git config user.email t@e.com
echo a > a.txt; git add a.txt
ROOT=$(printf root | git commit-tree -F - $(git write-tree)); git update-ref HEAD "$ROOT"
echo "root WITHOUT --root:";  git diff-tree --no-commit-id --name-status -r "$ROOT";         echo "(expect: empty)"
echo "root WITH --root:";     git diff-tree --no-commit-id --name-status -r --root "$ROOT";  echo "(expect: A	a.txt)"
echo "bad SHA:";              git diff-tree --no-commit-id --name-status -r 0000000000000000000000000000000000000000; echo "EXIT=$? (expect 128)"
cd /; rm -rf "$TMP"

# Expected: the manual commands print empty / "A\ta.txt" / EXIT=128 respectively, confirming git's
# diff-tree behavior matches what DiffTree + parseDiffTree branch on. No network, no external services.

# Scope-discipline greps (production code only — git.go)
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go          # expect: no matches
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                           # expect: no matches
git grep -n 'panic.*DiffTree' internal/git/git.go                              # expect: no matches (stub gone)
git grep -n 'func parseDiffTree' internal/git/git.go                           # expect: exactly 1 match
git grep -n 'runWithInput' internal/git/git.go | grep -i difftree             # expect: no matches (uses run())
git grep -nE '\-M\b|--find-renames' internal/git/git.go | grep -i diff-tree   # expect: no matches (no rename detection, G3)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Prove the parse is robust against the octal-quoted-path gotcha (G6) — the tab-split still works:
TMP=$(mktemp -d); cd "$TMP"; git init -q; git config user.name T; git config user.email t@e.com
printf 'x\n' > "$(printf 'sp\xc3\xa9cial.txt')"; git add . 2>/dev/null
ROOT=$(printf q | git commit-tree -F - $(git write-tree)); git update-ref HEAD "$ROOT"
echo "default quotepath (quoted, still tab-split OK):"
git diff-tree --no-commit-id --name-status -r --root "$ROOT" | awk -F'\t' '{ print "status=["$1"] path=["$2"]" }'
cd /; rm -rf "$TMP"
# Expected: status=[A] path=["sp\303\251cial.txt"] — the path field carries git's default quoting,
# but the tab-split succeeded (2 fields). This confirms parseDiffTree is unaffected; the quoted form
# is a documented v1 limitation (G6), NOT a parse bug.

# Race-detector confidence: every test runs under -race (Makefile `test` target = go test -race ./...).
# DiffTree is a single read-only git subprocess (no ref mutation, no locking concerns); -race validates
# the surrounding Go (run()'s buffer handling, parseDiffTree's slice appends — no shared state).

# Expected: the octal-path probe shows a clean 2-field split; all -race tests pass.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/git/` exits 0 (all 9 new cases + S1/S2/S3/S4/S5 cases + trimmed TestStubsPanic).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.

### Feature Validation

- [ ] On a child commit (A/M/D), `DiffTree` returns the parsed changes with correct statuses and paths.
- [ ] On a root commit with `isRoot=true`, every change is `A` (both files present).
- [ ] On a root commit with `isRoot=false`, the result is **empty** (no error) — proving `--root` matters.
- [ ] On a no-change commit, the result is empty (no error).
- [ ] On a bad SHA (all-zeros), returns an error containing `"git diff-tree: failed"` and `"(exit 128)"`.
- [ ] `parseDiffTree` parses synthetic 2-field (A/M/D) and 3-field (R100/C90) lines correctly; the
      3-field entries carry `SrcPath`.
- [ ] `parseDiffTree("")` returns an empty (nil) slice.
- [ ] A missing git binary returns an error mentioning "git binary not found".
- [ ] A cancelled context returns `errors.Is(err, context.Canceled)==true`.
- [ ] The command does NOT add `-M`/`-C`/`--find-renames` (grep confirms — "Identical UX", G3).

### Code Quality Validation

- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`, `StagedDiffOptions`,
      `RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS`/`ErrCASFailed` are byte-identical to
      landed forms.
- [ ] ZERO new imports in git.go (`fmt`/`strings` already present; `FileChange` already declared).
- [ ] `parseDiffTree` is package-private (lowercase) and placed immediately after `DiffTree`.
- [ ] Helper names use the `dt` prefix (no collision with S4's or S5's helpers).
- [ ] File placement matches the desired codebase tree (only `internal/git/` touched).
- [ ] Anti-patterns avoided (check against Anti-Patterns section).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

### Documentation & Deployment

- [ ] `DiffTree` has a doc comment citing PRD §9.9/FR42, Appendix C, and the `--root`/`isRoot` contract.
- [ ] `parseDiffTree` has a doc comment describing the 2-field/3-field routing and the empty-input rule.
- [ ] No new environment variables or config (internal method; PRD §9.9/§13 are the spec).
- [ ] The TestStubsPanic one-line removal is noted in the commit message (mirrors S2/S3/S4/S5).

---

## Anti-Patterns to Avoid

- ❌ Don't omit `--root` for root commits — a root commit WITHOUT `--root` produces NO output (verified).
  The `isRoot` parameter exists precisely to append `--root`; the caller drives it from RevParseHEAD's
  `isUnborn` (gotcha G1).
- ❌ Don't add `-M`/`-C`/`--find-renames` to the command — PRD §9.9/FR42 + Appendix C pin the exact
  command ("Identical UX"). Renames surface as D+A by design (gotcha G3).
- ❌ Don't treat empty output as an error — root-without-`--root`, no-change commits, and (rarely) a
  child that changed nothing all yield empty stdout + exit 0 (gotcha G5).
- ❌ Don't branch on `code == 128` — branch on `code != 0` (stable; matches S2/S3/S4/S5) (gotcha G2).
- ❌ Don't use `runWithInput` — diff-tree reads no stdin; use `run()` (gotcha G4).
- ❌ Don't inline the parse logic — split `parseDiffTree` out so the 3-field rename/copy path is
  unit-testable without `-M` (gotcha G11).
- ❌ Don't add `-c core.quotepath=false` to "fix" octal-quoted non-ASCII paths — that diverges from
  the PRD command; the quoting is a documented v1 limitation and the tab-split still works (gotcha G6).
- ❌ Don't pre-initialize `parseDiffTree`'s result to a non-nil empty slice — nil is idiomatic and
  range-safe (gotcha G15).
- ❌ Don't modify `run()`, `runWithInput`, the interface, `FileChange`, or any sibling method — S6 owns
  only `DiffTree` + `parseDiffTree` + its tests + the one-line `TestStubsPanic` edit.
- ❌ Don't add an import — `fmt`/`strings` are both already present.
- ❌ Don't name test helpers `headSHA`/`writeFile`/`casCommit`/etc. — collide with S4/S5; use the `dt`
  prefix (gotcha G9).

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: The `DiffTree` body and the `parseDiffTree` body are byte-equivalent to throwaway Go/git
invocations executed against the exact installed git (2.54.0) — the root-with/without-`--root`
behavior, the empty-output success cases, the bad-SHA exit 128, and the `-M` rename-vs-D+A distinction
are all empirically confirmed (research §2). The signature, the `FileChange` type, and the `run()`
helper are all already landed and unchanged. The `-M` omission (decision D1) and the split-out
`parseDiffTree` (decision D4) are the two non-obvious calls, both documented with their rationale and
guarded by tests. The only residual uncertainty (not 10/10) is the parallel-landing interaction with
S5: both edit `git.go` and `git_test.go`, but in disjoint regions/lines (UpdateRefCAS vs DiffTree),
and S6's `dt`-prefixed helpers are collision-free with S5's `cas`-prefixed helpers — so the package
compiles when both land. The "no-shell / no-cmd.Dir / no-`-M`" invariants are enforced by greppable
Level-3 gates.
