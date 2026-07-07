---
name: "P1.M1.T2.S4 — CommitTree with stdin message delivery and root-commit handling"
description: |
  Replace the `(*gitRunner).CommitTree` panic-stub (landed by P1.M1.T2.S1) with a real
  implementation. Signature is fixed by the `Git` interface (already landed): `CommitTree(ctx, tree,
  parents []string, msg) (sha string, err error)`. It builds args `commit-tree <tree> [-p <parent>]...
  -F -` and delivers the message via stdin (`cmd.Stdin = strings.NewReader(msg)`), so the message is
  bulletproof against special characters, leading dashes, quotes, and newlines (FINDING 4). An empty
  `parents` slice produces a ROOT commit (no `-p`); a non-empty slice adds one `-p` per parent
  (repeatable, forward-compatible with v2 merges). The message-via-stdin requirement CANNOT be met
  by S1's `run()` helper (it never sets `cmd.Stdin`), so this subtask adds ONE co-located helper
  `runWithInput(ctx, repo, stdin io.Reader, args...)` that is structurally identical to `run()` plus
  `cmd.Stdin = stdin`. `run()` itself is NOT modified (S2/S3 forbid it; S3 is landing concurrently).
  On success return the trimmed stdout (NEW_SHA — a dangling commit object; NO ref is moved). On
  `code != 0` (bad tree/parent) return a descriptive error. Adds ONE new import (`io`) to git.go
  and ONE new test file `internal/git/committree_test.go` (package git) covering root commit, child
  commit, stdin-special-chars message roundtrip, bad-tree, git-missing, and context-cancelled, plus
  five small fixture helpers. Also removes the single `CommitTree` line from `git_test.go`'s
  `TestStubsPanic` (required consequence of making the method real — mirrors S2/S3). Touches ONLY
  `internal/git/`; no interface, struct, `run()`, RevParseHEAD, or WriteTree changes.
---

## Goal

**Feature Goal**: Implement the third real git plumbing method on `*gitRunner` — `CommitTree` — the
dangling-commit-object primitive at the heart of Stagecoach's atomic-commit flow (PRD §13.2 step 2).
It takes an immutable `TREE_SHA` (from WriteTree, S3), zero-or-more `parents` (from RevParseHEAD's
`isUnborn`: empty ⇒ root commit; `[]string{sha}` ⇒ single parent), and a `msg`, and produces a NEW
commit object **without moving any ref** (the commit is "dangling" until UpdateRefCAS, S5, publishes
it). The message MUST be delivered via stdin (`-F -`) — never `-m` — so that messages containing
leading dashes, quotes, or newlines are stored byte-for-byte and never misparsed as git flags
(FINDING 4). Because S1's `run()` helper cannot deliver stdin, this subtask introduces a co-located
`runWithInput` helper (run() + `cmd.Stdin`); `run()` is left byte-identical.

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: (a) add `"io"` to the import block; (b) add the unexported
   `runWithInput` helper method (structurally identical to `run()` + `cmd.Stdin = stdin`); (c)
   replace the `CommitTree` panic-stub body with the ~12-line body that builds args and delegates
   to `runWithInput` (exact body in §Blueprint).
2. **CREATE** `internal/git/committree_test.go` (`package git`): six test functions
   (`TestCommitTree_RootCommit`, `TestCommitTree_ChildCommit`, `TestCommitTree_MessageViaStdin`,
   `TestCommitTree_BadTree`, `TestCommitTree_GitBinaryMissing`, `TestCommitTree_ContextCancelled`)
   plus five fixture helpers (`setIdentityConfig`, `writeFile`, `stageFile`, `writeTreeOf`,
   `headSHA`, `commitMessage`).
3. **MODIFY** `internal/git/git_test.go`: remove the single `assertPanics(t, "CommitTree", …)` line
   from `TestStubsPanic` (required now that CommitTree is real — the test would otherwise fail
   expecting a panic that no longer occurs; mirrors S2/S3).

No other files touched. No new dependencies (`io` is stdlib). `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 6 new `CommitTree` cases passing (plus S1's `run()`
tests, S2's `RevParseHEAD` tests, S3's `WriteTree` tests, and the now-trimmed `TestStubsPanic` all
still green); `CommitTree` with `parents==nil` (root) returns a 40-hex SHA whose `git cat-file -p`
shows NO `parent` line; `CommitTree` with `parents=[p]` returns a 40-hex SHA whose commit's parent
is `p`; a message containing leading dashes, quotes, and newlines is stored byte-for-byte (verified
via `git log --format=%B` roundtrip); a bad tree SHA returns a non-nil error whose message contains
`"git commit-tree: failed"`; a missing git binary surfaces as a non-nil error mentioning "git binary
not found" (NOT misread as a commit success); `run()` is byte-identical to its landed form (S2/S3).

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary caller), and the sibling
plumbing subtask S5 (`UpdateRefCAS`) — the immediate consumer of the `NEW_SHA` that `CommitTree`
returns.

**Use Case**: After snapshotting the index (`WriteTree` → `TREE_SHA`) and generating a commit
message, the orchestrator calls `newSHA, err := g.CommitTree(ctx, treeSHA, parents, msg)` to create
the commit object. `parents` is `nil` when `RevParseHEAD` reported `isUnborn` (root commit) or
`[]string{parentSHA}` otherwise. The returned `NEW_SHA` is dangling (no ref moved yet); it is handed
to `UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)` for the atomic publish (§13.2 step 3).

**User Journey**: `g := git.New(repoPath)` → `parent, isUnborn, err := g.RevParseHEAD(ctx)` →
`tree, err := g.WriteTree(ctx)` → (generate `msg`) → `parents := []string(nil); if !isUnborn { parents = []string{parent} }`
→ `newSHA, err := g.CommitTree(ctx, tree, parents, msg)` → `g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)`.

**Pain Points Addressed**: Decouples *commit-object creation* from *ref publication* — the central
design argument of PRD §13.1. A `git commit` would move HEAD non-atomically (racing with concurrent
terminal commits); `commit-tree` creates a dangling object that only becomes reachable via the CAS
in `update-ref`, preserving §18.1's invariant ("refs modified only at the final `update-ref` step,
and only if HEAD is unchanged since the snapshot"). The `-F -` stdin delivery eliminates the entire
class of message-quoting/injection bugs (a message starting with `-` would be misparsed by `-m`).

## Why

- **PRD §13.2 (The plumbing alternative):** *"`git commit-tree (-p <parent>) -m <msg> <tree>` —
  creates a commit object with the given tree, parent, and message, and prints its SHA. This also
  does not touch any ref. The commit object exists in the object store but is 'dangling'
  (unreferenced) until step 3."* (Stagecoach uses `-F -` instead of `-m` per FINDING 4; same
  semantics, safer message delivery.) This subtask IS that primitive.
- **PRD §9.9 / FR39 (Commit creation):** *"Create the commit object: if `PARENT_SHA` is non-empty,
  `git commit-tree -p <PARENT_SHA> -m <MSG> <TREE_SHA>`; else `git commit-tree -m <MSG> <TREE_SHA>`
  (root commit)."* `parents==nil` is the empty-PARENT_SHA/root case; `parents=[sha]` is the child
  case. (FR39 writes `-m`; FINDING 4 upgrades to `-F -` for safety — the work-item contract
  mandates `-F -`.)
- **PRD §13.5 (Edge cases):** *"Rootless repo (no commits yet): `PARENT_SHA` is empty.
  `commit-tree` is called without `-p` (creates a root commit)."* An empty `parents` slice is the
  signal; no special "is root?" flag exists in `CommitTree` — the orchestrator drives it via
  `isUnborn`.
- **PRD §18.1 (The invariant):** *"Every code path that does not reach a successful `update-ref`
  leaves the repository byte-for-byte unchanged (modulo harmless dangling objects)."* `CommitTree`
  is the primary producer of those "harmless dangling objects" — a failed generation still leaves a
  recoverable `NEW_SHA` for the rescue protocol (P1.M3.T3), and the repo is untouched because no ref
  moved.
- **`critical_findings.md` FINDING 4:** *"`-m "<msg>"` works but … use `-F -` (read message from
  stdin) to avoid ALL quoting issues with special characters, leading dashes, quotes, and newlines.
  … `-F -` via `cmd.Stdin = strings.NewReader(msg)` is bulletproof."* This is why `CommitTree` uses
  stdin and why a stdin-capable helper (`runWithInput`) is required.
- **`git_plumbing_reference.md` §2:** the canonical Go pattern for `commit-tree` with stdin
  (`cmd.Stdin = strings.NewReader(msg)`, `-F -`, repeatable `-p`, root = omit `-p`).
- **`git_plumbing_summary.md` Atomic Commit Sequence:** step 5 = `commit-tree [-p PARENT] -F - TREE`
  → `NEW_SHA (dangling, no ref moved)`; step 3–6 are the atomic core.
- **Foundation for S5 / P1.M3.T3:** `UpdateRefCAS` (S5) publishes the `NEW_SHA` this method returns;
  the rescue protocol (P1.M3.T3) embeds `NEW_SHA` (or `TREE_SHA` on failure) in its recovery
  command. Both are blocked on a correct `CommitTree`. This is the third of the 11 interface methods
  to leave stub-status.

## What

`CommitTree` builds `commit-tree <tree> [-p <p>]... -F -`, feeds `msg` to the child's stdin via
`runWithInput`, and translates the four-tuple into `(sha, err)`:

- `runWithInput` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `("", err)`. This is the ONLY path that returns a non-nil error for infrastructural reasons.
- `exitCode != 0` (128 on git 2.x — bad/nonexistent tree SHA or invalid parent SHA) → return
  `("", fmt.Errorf("git commit-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr)))`.
- `exitCode == 0` → return `(strings.TrimSpace(stdout), nil)`. The trimmed 40-hex (sha-1) or 64-hex
  (sha-256) NEW_SHA — a **dangling** commit object (no ref moved).

The `parents` slice drives the `-p` flags: each element appends `-p <parent>`; an empty/nil slice
appends nothing (root commit). `msg` is ALWAYS delivered via stdin (`-F -`), never `-m`.

No porcelain, no `git commit`, no ref movement, no SHA-format validation in production code.

### Success Criteria

- [ ] `internal/git/git.go` imports `"io"` (in addition to S1/S2's `bytes, context, errors, fmt,
      os/exec, strings`).
- [ ] `(*gitRunner).runWithInput` exists with the exact signature in §Blueprint (run() + `cmd.Stdin`).
- [ ] `(*gitRunner).CommitTree` body matches §Implementation Blueprint verbatim (no `panic`).
- [ ] `run()` is byte-identical to its landed form (S1/S2/S3) — NOT modified.
- [ ] `internal/git/committree_test.go` exists in `package git` with the 6 named test functions.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `CommitTree` line (removed).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all 6 `TestCommitTree_*` pass and S1/S2/S3's tests
      still pass.
- [ ] `TestCommitTree_RootCommit` asserts `parents==nil` ⇒ returned SHA has NO `parent` line.
- [ ] `TestCommitTree_MessageViaStdin` asserts a message with leading dashes/quotes/newlines is
      stored byte-for-byte (roundtrip via `git log --format=%B`).
- [ ] NO change to `run()`, `New`, `gitRunner`, the `Git` interface, `FileChange`, `StagedDiffOptions`,
      `RevParseHEAD` (real from S2), `WriteTree` (real from S3), or the other 8 method stubs.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` (inherited from S1; new code adds none).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact three files to touch (and
the exact single line to remove from `git_test.go`); the exact `run()`/`runWithInput` contract
(signatures + the `err==nil`-for-non-zero-exits invariant); the exact `CommitTree` body and the
exact `runWithInput` body (verified-equivalent to a throwaway invocation run against git 2.54.0);
the empirically-pinned commit-tree behavior (root ⇒ no `parent` line; child ⇒ `parent <p>`; bad
tree ⇒ exit 128 `fatal: ... is not a valid object`; `-F -` preserves leading-dash messages that `-m`
would misparse); the exact 6 test cases with verified assertions and the 6 fixture helpers; and the
exact validation commands with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§13.2 (the plumbing alternative: commit-tree creates a dangling commit, touches NO ref) is
        the behavior this method implements; §13.1 (why git commit is the wrong primitive) is the
        motivation; §9.9/FR39 (commit creation: -p <parent> for child, omit for root) is the
        parent-handling spec; §13.5 (rootless repo edge case) is why parents may be empty;
        §18.1 (the invariant: refs modified only at update-ref) is why CommitTree must not move
        a ref; §19 (security: args as []string, never sh -c) is inherited and must not be violated."
  critical: "This subtask owns ONLY CommitTree's body + runWithInput + its tests + the one-line
             TestStubsPanic edit + the 'io' import. Do NOT implement UpdateRefCAS (S5), the rescue
             protocol (P1.M3.T3), or any other method. Do NOT change the Git interface (already
             correct from S1). Do NOT modify run() (S2/S3 forbid it)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 4 — the -m vs -F - finding: use `-F -` (stdin) to avoid ALL quoting issues with
        special characters, leading dashes, quotes, newlines; Go passes args as []string (no shell)
        so this is bulletproof. FINDING 1 (unborn HEAD) and FINDING 6 (diff --quiet) are the sibling
        traps S2/T3 encode; they confirm run()'s 'non-zero exit is not a Go error' invariant that
        runWithInput and CommitTree rely on."
  critical: "FINDING 4 is THE reason CommitTree uses stdin and why a stdin-capable helper
             (runWithInput) is required instead of delegating to run() (which cannot set cmd.Stdin)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§2 (`git commit-tree`) documents exact semantics, the -p/-F/-m flags (-p repeatable; root =
        omit -p; -F - reads message from stdin; -m would misparse leading-dash messages), the
        identity resolution (user.name/user.email from config OR GIT_AUTHOR_*/GIT_COMMITTER_* env),
        and the canonical Go pattern with cmd.Stdin = strings.NewReader(msg). Cross-cutting
        conventions: TrimSpace single-line output; capture separate stdout/stderr; args as []string;
        never sh -c."
  critical: "§2's standalone example sets cmd.Env with hardcoded identity — that is ILLUSTRATIVE,
             not Stagecoach's behavior. Stagecoach inherits the parent env (commits AS the configured
             user); tests set repo-local config (see gotcha G6). §2 confirms -F - trims a single
             trailing newline from input (relevant to the message-roundtrip assertion, gotcha G9)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Atomic Commit Sequence (step 5 = commit-tree [-p PARENT] -F - TREE → NEW_SHA dangling)
        and the Exit-Code Cheat Sheet row for `git commit-tree`: exit 0 = NEW_SHA; exit 128 =
        bad tree/parent; note 'object write only; no ref moved'. This is the one-row spec."
  critical: "Confirms exit 0 = success and the 'no ref moved' guarantee. run()'s err=nil-for-non-zero
             invariant (S1 gotcha G2) is what makes the exitCode branch reachable in runWithInput."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything CommitTree consumes: the gitRunner struct, the run() helper
        (exact signature and verified body — which this subtask does NOT modify), the CommitTree
        panic-stub being replaced, the Git interface (signature already correct: parents []string),
        New(), the git_test.go initRepo(t,dir) helper, and the assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil
             for non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero
             exit is not a Go error) is the foundation runWithInput replicates. S1's invariant 'run()
             is the ONLY place Stagecoach shells out' is preserved by keeping runWithInput co-located
             in the same file with the identical structure."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The CONTRACT for the pattern to MIRROR and the helpers to reuse: RevParseHEAD is a real
        run()-delegating method (err-first guard → code branch → trimmed-stdout return) — CommitTree
        mirrors it (delegating to runWithInput instead). S2 added the `strings` import CommitTree
        reuses. S2's revparse_test.go defines `minGitEnv()` and `makeEmptyCommit()`, which
        CommitTree's tests reuse without redeclaring."
  critical: "S2's Level-2 note documents the EXACT TestStubsPanic-edit pattern (remove the now-real
             method's line) — apply the same one-line removal for CommitTree. S2's gotcha G7 (test
             file is `package git`, white-box) and G8 (reuse initRepo, don't redeclare) apply
             identically."

- docfile: plan/001_f1f80943ac34/P1M1T2S3/PRP.md
  why: "The CONTRACT for the SIBLING whose TestStubsPanic edit S4 must coexist with: S3 removed the
        WriteTree line; S4 removes the CommitTree line (distinct lines, no overlap). S3 also
        confirmed the WriteTree body is real on disk (S3 is landing/landed concurrently) — so
        CommitTree's tests can rely on staged files + write-tree producing a valid TREE_SHA."
  critical: "S3 and S4 both EDIT git_test.go's TestStubsPanic and git.go. S3's git.go edit is the
        WriteTree body; S4's git.go edits are (a) the io import, (b) runWithInput, (c) the CommitTree
        body — all DISTINCT regions from WriteTree. S3's git_test.go edit is the WriteTree line; S4's
        is the CommitTree line — distinct lines. No textual overlap. (See Parallel-Execution Note.)"

- docfile: plan/001_f1f80943ac34/P1M1T2S4/research/commit_tree_validation.md
  why: "THIS subtask's own research: the run() stdin gap and the runWithInput decision (§1), the
        parents []string ↔ parentSHA reconciliation (§2), the empirically-pinned commit-tree
        behavior on git 2.54.0 (§3: root/child/bad-tree/-F - special-chars), the identity-handling
        strategy (§4: production inherits env, tests set repo config), the test design matrix (§5),
        and the decisions log (§7: D1–D8)."
  critical: "§1 (the runWithInput decision) and §2 (parents []string reconciliation) are the two
             non-obvious design calls an implementing agent would otherwise guess at. §3's empirical
             transcript pins every assertion (root ⇒ no parent line; child ⇒ parent <p>; bad tree ⇒
             exit 128 'is not a valid object'; leading-dash message preserved verbatim). §5 names the
             distinct helper names (no collision with S3's closure-locals writeFile/runGit)."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-commit-tree#_options
  why: "Documents -p (repeatable parent; omit for root commit), -F <file> with - meaning stdin
        (read message verbatim, trims a single trailing newline), and -m (each is a separate
        paragraph; misparses leading-dash messages). Confirms the CommitTree(-F -, parents [])
        contract and why -m is avoided."
  critical: "Establishes -F - (stdin) is the contractually-mandated, bulletproof message-delivery
             form; -p is repeatable and omittable (root)."
- url: https://git-scm.com/docs/git-commit-tree#_description
  why: "Documents that commit-tree 'creates a new commit object' and prints its SHA, deriving
        author/committer identity from the configuration files and the GIT_AUTHOR_*/GIT_COMMITTER_*
        environment variables. Confirms it does NOT move any ref (the dangling-object guarantee)."
  critical: "Confirms identity resolution from CONFIG (so tests can use `git config user.name`) and
             the 'no ref moved' guarantee central to §13.2/§18.1."
- url: https://pkg.go.dev/os/exec#Cmd
  why: "Cmd.Stdin is an io.Reader; when set, it is connected to the child's standard input. With
        cmd.Env == nil (the default), the child inherits the parent's environment. This is HOW -F -
        receives the message and HOW identity is inherited."
  critical: "This is the entire mechanism: cmd.Stdin = strings.NewReader(msg) feeds -F -; cmd.Env
             stays nil so production inherits the user's git config / env. runWithInput sets both."
- url: https://pkg.go.dev/strings#NewReader
  why: "strings.NewReader returns an io.Reader over a string — the stdin source for the message."
  critical: "strings is already imported (S2); no new import for NewReader. The 'io' import is only
             needed for runWithInput's parameter type io.Reader."
- url: https://pkg.go.dev/io#Reader
  why: "The io.Reader interface — runWithInput's stdin parameter type."
  critical: "This is the ONE new symbol forcing the 'io' import. CommitTree passes strings.NewReader(msg)
             (which satisfies io.Reader)."
```

### Current Codebase Tree (after S1 + S2 + S3 have landed — verified on disk)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface + gitRunner + run() + New() + stubs; S2: RevParseHEAD real;
│       │                 #          S3: WriteTree real
│       │                 # imports: bytes, context, errors, fmt, os/exec, strings  ← strings ALREADY HERE
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic(9 methods, incl CommitTree) + initRepo + assertPanics
│       ├── revparse_test.go  # S2: 4 TestRevParseHEAD_* + minGitEnv + makeEmptyCommit
│       └── writetree_test.go # S3: 5 TestWriteTree_* + makeMergeConflict (closure-locals writeFile/runGit)
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go              # MODIFIED — +io import; +runWithInput helper; CommitTree stub → real body
        ├── git_test.go         # MODIFIED — remove the ONE `CommitTree` line from TestStubsPanic
        ├── revparse_test.go    # UNCHANGED (S2's file; minGitEnv/makeEmptyCommit reused, not edited)
        ├── writetree_test.go   # UNCHANGED (S3's file; not edited)
        └── committree_test.go  # NEW — package git; 6 TestCommitTree_* + 6 fixture helpers
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (1) Add `"io"` to imports. (2) Add the unexported `runWithInput` helper. (3) Replace the `CommitTree` panic-stub with the `runWithInput`-delegating body. Nothing else. |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "CommitTree", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/committree_test.go` | CREATE | `package git` tests for `CommitTree`: root / child / stdin-special-chars / bad-tree / git-missing / ctx-cancelled, plus the `setIdentityConfig`/`writeFile`/`stageFile`/`writeTreeOf`/`headSHA`/`commitMessage` fixtures. |

**Explicitly NOT created/modified:** `run()` (byte-identical), `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), the other 8
method stubs, `revparse_test.go` (S2's tests), `writetree_test.go` (S3's tests), `go.mod`/`go.sum`,
the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — run() CANNOT deliver stdin): S1's run() creates the cmd but never sets cmd.Stdin,
// so the child's stdin is /dev/null. commit-tree with -F - reads the message from stdin until EOF;
// with no stdin, it reads an empty message → empty-commit failure or misleading success. This is
// WHY this subtask adds runWithInput (run() + cmd.Stdin). Do NOT attempt to deliver the message via
// run(); do NOT add -m to run() args (FINDING 4 forbids -m for leading-dash safety). The ONLY
// correct path is runWithInput(ctx, repo, strings.NewReader(msg), args...). (See research §1.)

// CRITICAL (G2 — do NOT modify run()): S2 and S3 (both landed) forbid modifying run(), and S3 is
// landing concurrently. runWithInput is a NEW, SEPARATE method structurally identical to run() plus
// one line (cmd.Stdin = stdin). The ~20 lines of near-duplication is a deliberate trade-off for not
// touching run() during parallel landing. A future refactor could extract a shared runCore, but that
// would rewrite run()'s body (S2/S3 territory) and is OUT OF SCOPE. (See research §1, decision D1.)

// CRITICAL (G3 — the interface is `parents []string`, NOT a singular parentSHA): the work-item
// CONTRACT prose says "CommitTree(ctx, treeSHA, parentSHA, message)" and "if parentSHA != '' append
// -p". But the Git interface (S1, ALREADY LANDED) is CommitTree(ctx, tree, parents []string, msg).
// The interface is authoritative. Map: each element of parents → a `-p <parent>` arg; empty/nil
// parents → no -p → root commit. Do NOT change the interface, add an isUnborn param, or pass a
// single string. The orchestrator (P1.M3.T4) decides parents from RevParseHEAD's isUnborn.
// (See research §2, decision D3.)

// CRITICAL (G4 — err checked BEFORE code, inherited invariant): runWithInput returns err==nil for
// NON-ZERO git exits (128, …) and err!=nil with exitCode==-1 ONLY for infrastructural failures
// (LookPath miss, context cancel, start/I/O). So CommitTree MUST check `err != nil` FIRST
// (authoritative), THEN branch on exitCode. A missing git binary (code=-1, err!=nil) is surfaced as
// a real error BEFORE the `code != 0` failure branch runs — so a LookPath miss is never misreported
// as a commit failure. TestCommitTree_GitBinaryMissing guards against regressing this.

// CRITICAL (G5 — the TestStubsPanic edit): git_test.go's TestStubsPanic (after S2+S3) STILL
// includes `assertPanics(t, "CommitTree", func() { _, _ = g.CommitTree(ctx, "tree", nil, "msg") })`
// as its FIRST line. Once CommitTree is real (no panic), assertPanics fails with "expected panic,
// but did not panic". Resolution (mirrors S2's RevParseHEAD removal and S3's WriteTree removal):
// DELETE that one line. After removal, TestStubsPanic covers the remaining 8 stubs. This is the
// ONLY edit to git_test.go. Document it in the commit message.

// GOTCHA (G6 — identity: production inherits env, tests set repo config): commit-tree resolves
// author/committer from user.name/user.email config OR GIT_AUTHOR_*/GIT_COMMITTER_* env. Production
// CommitTree sets cmd.Env=nil (inherits parent env) — it commits AS the configured user; do NOT
// inject identity. But temp test repos (from initRepo) have NO config and the test process env has
// no global identity, so commit-tree would fail with "empty ident name/address not allowed".
// Resolution: the test fixture setIdentityConfig(t,dir) runs `git -C dir config user.name Test` and
// `git -C dir config user.email test@example.com` (writes .git/config). commit-tree then resolves
// identity from repo config. This avoids t.Setenv (process-global within a package run) and is robust
// in CI where global config is absent. (S2's makeEmptyCommit sets identity via ITS OWN command's
// cmd.Env — that env is NOT visible to the later commit-tree call CommitTree makes, hence the
// repo-config approach here. See research §4, decision D6.)

// GOTCHA (G7 — reuse S1/S2 helpers, do not redeclare): initRepo(t,dir) lives in git_test.go;
// minGitEnv() and makeEmptyCommit(t,dir,msg) live in revparse_test.go — all package git. REUSE them
// directly in committree_test.go. Do NOT redeclare any (a redeclaration is a compile error). The NEW
// helper names are setIdentityConfig, writeFile, stageFile, writeTreeOf, headSHA, commitMessage —
// all DISTINCT from S3's makeMergeConflict closure-locals (which are NOT package-level, so no
// collision even by name). (See research §5.)

// GOTCHA (G8 — getting a TREE_SHA for the tests): CommitTree needs a valid <tree> arg. Create it
// directly in the fixture: write a file, `git add`, then `git write-tree` → trimmed 40-hex SHA.
// Use the writeTreeOf(t,dir) helper. Do NOT call g.WriteTree(ctx) inside CommitTree's tests (that
// couples this method's unit tests to S3's WriteTree working — keep them decoupled; the orchestrator
// chains them, but each method is unit-tested in isolation).

// GOTCHA (G9 — message-roundtrip trailing newline): `git log --format=%B` appends one trailing \n
// as a record separator. `-F -` trims a single trailing newline from the INPUT (per git docs §2).
// So: pass test messages WITHOUT a trailing newline (so -F -'s trim is a no-op and the stored
// message == input), then compare strings.TrimSpace(retrieved) == input. This is robust regardless
// of the exact separator bytes. (See research §3, §5; verified on git 2.54.0.)

// GOTCHA (G10 — -F - is ALWAYS used, even for the empty/root case): there is no `-m` path in
// CommitTree. A message starting with `-` (e.g. "-n -p --foo") would be misparsed by -m as git
// flags; -F - reads it verbatim from stdin. Verified empirically: such a message is stored
// byte-for-byte. This is FINDING 4's core point. Do NOT add a `-m msg` fallback.

// GOTCHA (G11 — no shell, no cmd.Dir): CommitTree and runWithInput inherit S1's §19 guarantees.
// runWithInput uses exec.CommandContext + []string args + -C repo flag (NOT cmd.Dir / os.Chdir).
// Do NOT introduce exec.Command / os.Chdir / sh -c in the production code. The test fixtures DO use
// exec.Command directly (parallel to S1's initRepo, S2's makeEmptyCommit, S3's makeMergeConflict) —
// that is acceptable test-fixture usage ([]string args + cmd.Env, never a shell). The Level-3 grep
// for sh -c / cmd.Dir covers PRODUCTION code (git.go).

// GOTCHA (G12 — no SHA validation in production): on exit 0 return the trimmed stdout. Do NOT add
// a hex-length check to CommitTree — that would deviate from the contract and is unnecessary
// (downstream update-ref will reject a bad SHA). The `^[0-9a-f]{40,64}$` regex is TEST-ONLY
// (sanity-check of git's own contract; 40=sha-1, 64=sha-256).

// GOTCHA (G13 — test file is package git, white-box): CommitTree is on *gitRunner and delegates to
// the unexported runWithInput; New() returns the Git interface but the fixture work needs
// `package git` to call New() and reuse initRepo/minGitEnv/makeEmptyCommit. Match S1/S2/S3's package
// (carried from S2 G7 / S3 G9).

// GOTCHA (G14 — the single import delta is `io`): git.go currently imports bytes, context, errors,
// fmt, os/exec, strings (S1+S2). runWithInput's signature uses io.Reader → add "io". CommitTree
// uses only strings.NewReader, strings.TrimSpace, fmt.Errorf (all already imported). The compiler
// errors `undefined: io` if you forget; stating it here saves a wasted build cycle.
```

## Implementation Blueprint

### Data models and structure

None added or changed. `CommitTree`'s return type (`string, error`) is already declared in the `Git`
interface by S1. No new structs, no new options type. The `io.Reader` parameter type on the new
`runWithInput` helper is a stdlib interface (no new type defined).

### The `runWithInput` helper (exact — copy verbatim)

Add this as a new unexported method on `*gitRunner`, immediately after the existing `run()` method.
It is `run()` + `cmd.Stdin = stdin`.

```go
// runWithInput is run() plus a stdin pipe. It exists because run() cannot set cmd.Stdin (its body
// leaves stdin as /dev/null), and commit-tree with -F - must read the commit message from stdin
// (FINDING 4: -F - avoids ALL quoting/special-character/leading-dash issues that -m would suffer).
// It is the ONLY other place Stagecoach shells out to git; it is co-located with run() and shares
// its structure exactly (LookPath → -C repo → separate buffers → errors.As(ExitError) with
// err==nil for non-zero exits). run() itself is intentionally left unmodified (see research §1).
//
// Identity: cmd.Env is NOT set here, so the child inherits the parent environment. Production
// callers commit AS the configured user (git resolves user.name/user.email from config/env);
// tests set repo-local user.name/user.email via `git config` (see committree_test.go).
func (g *gitRunner) runWithInput(ctx context.Context, repo string, stdin io.Reader, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}

	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1 of S1)
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	cmd.Stdin = stdin                                  // ← the one difference from run()
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()

	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit → capture code, err stays nil
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr // start / I/O failure
}
```

> **Verified:** structurally identical to the empirically-validated `run()` body (S1's
> `run_helper_validation.md` §4.1), with the single addition of `cmd.Stdin = stdin`. The
> `err==nil`-for-non-zero-exits invariant is preserved (research §1, §3).

### The `CommitTree` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature.

```go
// CommitTree creates a commit object for tree with the given parents and message and returns its
// SHA. The message is delivered via stdin with `-F -` (NOT -m) so it is bulletproof against special
// characters, leading dashes, quotes, and newlines (FINDING 4; verified empirically that a message
// beginning with "-n -p --foo" is stored verbatim). parents == nil/empty ⇒ root commit (no -p);
// each element of a non-empty parents slice appends a `-p <parent>` (repeatable, forward-compatible
// with v2 merge commits). Like write-tree, this does NOT move any ref: the returned commit is a
// dangling object until UpdateRefCAS (P1.M1.T2.S5) publishes it (PRD §13.2, §18.1).
//
// commit-tree fails (non-zero exit, 128 on git 2.x) when tree or a parent is not a valid object;
// that is surfaced here as runWithInput's exitCode != 0 (err stays nil per its invariant).
func (g *gitRunner) CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error) {
	args := make([]string, 0, 4+len(parents)*2)
	args = append(args, "commit-tree", tree)
	for _, p := range parents {
		args = append(args, "-p", p) // repeatable; root commit = empty parents (no -p appended)
	}
	args = append(args, "-F", "-") // message via stdin — avoids all quoting pitfalls (FINDING 4)

	stdout, stderr, code, err := g.runWithInput(ctx, g.workDir, strings.NewReader(msg), args...)
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git commit-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}
```

> **Verified:** the args shape (`commit-tree <tree> [-p <p>]... -F -`) and the branch order are
> confirmed by this subtask's research §3 (root ⇒ no parent line in `cat-file -p`; child ⇒
> `parent <p>`; bad tree ⇒ exit 128 `fatal: ... is not a valid object`; `-F -` preserves
> leading-dash messages), re-verified empirically on this box (git 2.54.0).

### The import change (exact)

git.go's current import block (after S1+S2):
```go
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)
```
Add `"io"` (keep gofmt's alphabetical grouping — it sorts between `fmt` and `os/exec`):
```go
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (three surgical edits)
  - EDIT 1 — add the "io" import:
      FIND the import block (bytes, context, errors, fmt, os/exec, strings) and ADD "io" in sorted
      order (between fmt and os/exec). (This is the ONLY import change; gotcha G14.)
  - EDIT 2 — add the runWithInput helper:
      INSERT the runWithInput method (§"The runWithInput helper") immediately AFTER the existing
      run() method's closing brace and BEFORE the stubs section. Keep it unexported, on *gitRunner.
  - EDIT 3 — replace the CommitTree panic-stub:
      FIND the stub:
        func (g *gitRunner) CommitTree(ctx context.Context, tree string, parents []string, msg string) (string, error) {
            panic("gitRunner.CommitTree: not yet implemented — see P1.M1.T2.S4")
        }
      REPLACE with the body in §"The CommitTree body" above (keep the same signature, add the doc comment).
  - DO NOT touch: run(), New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD
    (already real from S2), WriteTree (already real from S3), or any of the other 8 method stubs
    (UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects,
    CommitCount, AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic (the FIRST assertPanics line):
      assertPanics(t, "CommitTree", func() { _, _ = g.CommitTree(ctx, "tree", nil, "msg") })
  - DELETE that single line. After removal TestStubsPanic covers the remaining 8 stubs: UpdateRefCAS,
    DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll.
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper,
    the other assertPanics lines).
  - WHY: once CommitTree is real it no longer panics; assertPanics would fail (gotcha G5). Mirrors
    S2's RevParseHEAD removal and S3's WriteTree removal.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (8 stubs still panic).

Task 3: CREATE internal/git/committree_test.go (package git — white-box)
  - FILE: internal/git/committree_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G13; matches git_test.go / revparse_test.go)
  - IMPORTS: context, errors, os, os/exec, path/filepath, regexp, strings, testing
  - WRITE the fixture helpers (exact bodies in research §5; all DISTINCT names — gotcha G7):
      setIdentityConfig(t, dir):
        - for kv in ["user.name=Test", "user.email=test@example.com"]:
            exec.Command("git", "-C", dir, "config", kv); on error t.Fatalf. t.Helper().
        - (writes .git/config so commit-tree resolves identity — gotcha G6.)
      writeFile(t, dir, name, body):
        - os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); on error t.Fatalf. t.Helper().
      stageFile(t, dir, name):
        - exec.Command("git", "-C", dir, "add", name) with default env; on CombinedOutput error
          t.Fatalf. t.Helper().
      writeTreeOf(t, dir) string:
        - exec.Command("git", "-C", dir, "write-tree"); return strings.TrimSpace(stdout); on error
          t.Fatalf. t.Helper(). (Gets a valid TREE_SHA — gotcha G8; do NOT call g.WriteTree.)
      headSHA(t, dir) string:
        - exec.Command("git", "-C", dir, "rev-parse", "HEAD"); return trimmed stdout; on error
          t.Fatalf. t.Helper().
      commitMessage(t, dir, sha) string:
        - exec.Command("git", "-C", dir, "log", "--format=%B", "-n", "1", sha); return trimmed
          stdout; on error t.Fatalf. t.Helper(). (Message roundtrip — gotcha G9.)
  - WRITE the 6 test functions (assertions in §"Test cases" below):
      TestCommitTree_RootCommit:
        repo := t.TempDir(); initRepo(t, repo); setIdentityConfig(t, repo)
        writeFile(t, repo, "a.txt", "hello\n"); stageFile(t, repo, "a.txt")
        tree := writeTreeOf(t, repo)
        g := New(repo)
        sha, err := g.CommitTree(context.Background(), tree, nil, "feat: root commit")  // parents==nil
        assert err == nil
        assert regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha)
        // verify NO parent line (root commit):
        cat := exec.Command("git", "-C", repo, "cat-file", "-p", sha); out, _ := cat.CombinedOutput()
        assert !bytes.Contains(out, []byte("\nparent "))   // root commit has no parent line
      TestCommitTree_ChildCommit:
        repo := t.TempDir(); initRepo(t, repo); setIdentityConfig(t, repo)
        makeEmptyCommit(t, repo, "initial")                // reuse S2's helper
        writeFile(t, repo, "a.txt", "hello\n"); stageFile(t, repo, "a.txt")
        tree := writeTreeOf(t, repo)
        parent := headSHA(t, repo)
        g := New(repo)
        sha, err := g.CommitTree(context.Background(), tree, []string{parent}, "feat: child")  // parents=[p]
        assert err == nil
        assert regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha)
        cat := exec.Command("git", "-C", repo, "cat-file", "-p", sha); out, _ := cat.CombinedOutput()
        assert bytes.Contains(out, []byte("parent "+parent))   // child commit links the parent
      TestCommitTree_MessageViaStdin:
        repo := t.TempDir(); initRepo(t, repo); setIdentityConfig(t, repo)
        writeFile(t, repo, "a.txt", "hello\n"); stageFile(t, repo, "a.txt")
        tree := writeTreeOf(t, repo)
        msg := "feat: x\n\nbody line\n--weird--leading dashes and \"quotes\" and 'apos'"  // no trailing \n (gotcha G9)
        g := New(repo)
        sha, err := g.CommitTree(context.Background(), tree, nil, msg)
        assert err == nil
        assert commitMessage(t, repo, sha) == strings.TrimSpace(msg)   // byte-for-byte roundtrip (gotcha G9/G10)
      TestCommitTree_BadTree:
        repo := t.TempDir(); initRepo(t, repo); setIdentityConfig(t, repo)   // identity set even though unused (git may check early)
        g := New(repo)
        sha, err := g.CommitTree(context.Background(), "0000000000000000000000000000000000000000", nil, "msg")
        assert err != nil
        assert strings.Contains(err.Error(), "git commit-tree: failed")   // ← THE contract assertion (code != 0)
        assert sha == ""
      TestCommitTree_GitBinaryMissing:
        t.Setenv("PATH", "")                              // makes runWithInput's LookPath("git") fail
        g := New(t.TempDir())                             // dir need not be a repo (LookPath fails first)
        sha, err := g.CommitTree(context.Background(), "tree", nil, "msg")
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert sha == ""                                  // ← guard: LookPath miss NOT misread as commit
      TestCommitTree_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call (deterministic)
        g := New(t.TempDir())
        sha, err := g.CommitTree(ctx, "tree", nil, "msg")
        assert err != nil && errors.Is(err, context.Canceled)
        assert sha == ""
  - NAMING: TestCommitTree_<Scenario>; helpers setIdentityConfig/writeFile/stageFile/writeTreeOf/
    headSHA/commitMessage (distinct from S1/S2/S3 helpers — gotcha G7).
  - DO NOT redeclare initRepo / minGitEnv / makeEmptyCommit / makeMergeConflict (they live in
    git_test.go / revparse_test.go / writetree_test.go).
  - VERIFY: go test -race -run 'TestCommitTree' ./internal/git/ → exit 0, all 6 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git grep -n 'panic.*CommitTree' internal/git/git.go                      (expect: no matches — stub gone)
  - RUN: git grep -n 'func (g \*gitRunner) runWithInput' internal/git/git.go      (expect: exactly 1 match)
  - RUN: git status --porcelain → expect EXACTLY:
        M internal/git/git.go
        M internal/git/git_test.go
        ?? internal/git/committree_test.go
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestCommitTree_RootCommit` | initRepo + setIdentityConfig + stage+writeTree | `err==nil`; SHA matches `^[0-9a-f]{40,64}$`; `cat-file -p` has **no `parent` line** | Root commit (`parents==nil` ⇒ no `-p`) — §13.5 rootless-repo edge case |
| `TestCommitTree_ChildCommit` | initRepo + setIdentityConfig + makeEmptyCommit + stage+writeTree + headSHA | `err==nil`; SHA matches hex; `cat-file -p` contains `parent <headSHA>` | Child commit (`parents=[p]` ⇒ `-p p`) — FR39 non-empty parent path |
| `TestCommitTree_MessageViaStdin` | initRepo + setIdentityConfig + stage+writeTree; msg has `\n`, `--`, `"`, `'` | `err==nil`; `commitMessage(sha) == TrimSpace(msg)` | **The core `-F -` guarantee**: special chars/leading-dash preserved byte-for-byte (FINDING 4) |
| `TestCommitTree_BadTree` | initRepo + setIdentityConfig; tree=`000…0` | `err!=nil` contains `"git commit-tree: failed"`; SHA == `""` | Error branch: invalid tree → exit 128 (code != 0) |
| `TestCommitTree_GitBinaryMissing` | `t.Setenv("PATH","")` | `err!=nil` contains "git binary not found"; SHA == `""` | `runWithInput`'s err path propagated, not misread as commit success |
| `TestCommitTree_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; SHA == `""` | ctx.Err() surfaced (not exit 0) |

### Implementation Patterns & Key Details

```go
// === Why runWithInput exists (and run() is not modified) ===
// run() (S1) never sets cmd.Stdin, so it cannot feed -F -. Modifying run()'s signature/body is
// forbidden by S2/S3 (and S3 lands concurrently). runWithInput is run() + one line, co-located,
// preserving the single-file / single-pattern shell-out discipline. The ~20 lines of duplication
// is the price of not touching run() during parallel landing. (research §1, decision D1.)

// === Why parents []string, not parentSHA (G3) ===
// The Git interface (landed by S1) takes parents []string. The contract's prose ("parentSHA")
// describes the v1 use: the orchestrator passes nil (root) or []string{sha} (child). []string is
// forward-compatible with v2 merges (-p A -p B). Each element → one -p; empty → root. CommitTree
// does NOT call RevParseHEAD or know about isUnborn — its caller decides parents.

// === Why -F - ALWAYS, never -m (G10, FINDING 4) ===
// -m misparses a message beginning with "-" as git flags. Verified: msg="-n -p --foo" with -m fails
// or garbage; with -F - it is stored verbatim. -F - reads stdin until EOF; -F also trims a single
// trailing newline (relevant to the roundtrip assertion, G9). There is no -m path in CommitTree.

// === Why branch on `code != 0` (not `code == 128`) ===
// The contract says "on success return trimmed stdout". commit-tree's only documented non-zero exit
// is a bad tree/parent (128 on git 2.x). Treating any non-zero exit as a commit-object failure is
// correct and slightly more future-proof than pinning exactly 128 (decision D4; mirrors S3's
// WriteTree). The message names "git commit-tree: failed" and includes the trimmed stderr (whose
// real text on git 2.54.0 is "fatal: <sha> is not a valid object").

// === Why err is checked BEFORE code (the branch order, G4) ===
// runWithInput guarantees: err != nil ⟹ exitCode == -1 (LookPath / context / start failure).
//                       err == nil  for every real git exit (0, 128, …).
// So `if err != nil { return "", err }` is the authoritative infrastructural-failure guard. Only
// when err == nil does `code != 0` decide failure-vs-success. A missing git binary (code=-1, err!=nil)
// is thus NEVER misreported as a commit failure — guarded by TestCommitTree_GitBinaryMissing.

// === Why we include the trimmed stderr in the failure message ===
// On git 2.54.0 the stderr is "fatal: <sha> is not a valid object". Including it (trimmed) gives the
// user/debugger git's exact phrasing. The contract requires the message indicate failure (our literal
// "git commit-tree: failed" satisfies that); the stderr is supplementary. Do NOT substring-match the
// stderr for detection — branch on exitCode (the stable signal).

// === Why strings.TrimSpace on stdout (success case) ===
// git prints "<commit-sha>\n" (trailing newline). TrimSpace yields the bare 40/64-hex SHA that
// UpdateRefCAS (S5) publishes and the rescue message embeds. Untrimmed, the SHA would carry a "\n".

// === Why identity is via repo config in tests (G6) ===
// commit-tree resolves user.name/user.email from config OR env. Production inherits parent env
// (cmd.Env=nil → commits AS the user). Tests write .git/config via setIdentityConfig so commit-tree
// resolves identity without env pollution or global-config assumptions (robust in CI).

// === Why the message-roundtrip test compares TrimSpace(retrieved) == msg (G9) ===
// git log --format=%B appends one \n separator; -F - trims one trailing newline from input. Passing
// a message with NO trailing newline makes -F -'s trim a no-op, so stored == input; then
// TrimSpace(retrieved) == input. Robust to the exact separator bytes.

// === Reusing initRepo / minGitEnv / makeEmptyCommit from the existing test files ===
// Because committree_test.go is `package git`, those helpers are in scope. Call them directly; do
// NOT redefine them (a redeclaration is a compile error — gotcha G7). makeEmptyCommit is used in the
// child-commit test to establish HEAD before staging the new tree.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, io, strings, regexp, os.WriteFile (1.16+), path/filepath, t.Setenv
    (1.17+), errors.Is, bytes.Contains all available
  - deps: NONE added (io is stdlib)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go               # MODIFIED: +io import, +runWithInput helper, CommitTree body
  - file: internal/git/git_test.go          # MODIFIED: remove the CommitTree line from TestStubsPanic
  - file: internal/git/committree_test.go   # NEW: package git, 6 tests + 6 fixtures

DOWNSTREAM CONSUMERS (informational — do NOT implement now):
  - P1.M1.T2.S5 (UpdateRefCAS): publishes CommitTree's returned NEW_SHA via 3-arg CAS
    (git update-ref HEAD <newSHA> <expectedOld>); for a root commit expectedOld = all-zeros hash.
  - P1.M3.T3 (Rescue protocol): on generation failure with a candidate message, the rescue message
    offers `git commit-tree -p <PARENT_SHA> -m "msg" <TREE_SHA> | xargs git update-ref HEAD`
    (the manual equivalent of CommitTree + UpdateRefCAS).
  - P1.M3.T4 (CommitStaged orchestrator): the primary caller —
    newSHA, err := g.CommitTree(ctx, treeSHA, parents, msg); then g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)

PARALLEL-EXECUTION NOTE:
  - S1/S2/S3 have landed on disk (git.go has real RevParseHEAD + WriteTree; git_test.go's
    TestStubsPanic has 9 lines with CommitTree first; revparse_test.go and writetree_test.go exist).
    This subtask EDITS git.go in three spots (io import / runWithInput / CommitTree body) — all
    DISTINCT regions from S3's WriteTree-body edit. It EDITS git_test.go in one spot (the CommitTree
    assertPanics line) — a DIFFERENT line from S3's WriteTree-line removal. It ADDS a separate
    committree_test.go — no overlap with S3's writetree_test.go. If git.go does not yet contain the
    CommitTree stub at edit time (S1 not landed), the edits apply once it does; otherwise the edit
    anchors (the CommitTree stub text, the io import block, the run() closing brace) are quoted
    verbatim above.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...       # Expected: exit 0, no warnings (e.g. no unused import, no shadowing)
go build ./internal/git/        # Expected: exit 0 (package compiles; the `io` import resolves)
go build ./...                  # Expected: exit 0 (whole module compiles)

# Expected: zero output/errors. If `go build` says `undefined: io`, you forgot the import (gotcha G14).
# If `go vet` flags the runWithInput/CommitTree duplication or an unused var, READ and fix before
# proceeding. (Near-duplication of run()/runWithInput is intentional — see G2; vet does not flag it.)
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v -run 'TestCommitTree' ./internal/git/   # Expected: 6 tests PASS, exit 0
# Must see: TestCommitTree_RootCommit, TestCommitTree_ChildCommit, TestCommitTree_MessageViaStdin,
#           TestCommitTree_BadTree, TestCommitTree_GitBinaryMissing, TestCommitTree_ContextCancelled
#           — all ok.

go test -race -run 'TestStubsPanic' ./internal/git/     # Expected: PASS, exit 0
# After removing the CommitTree line, TestStubsPanic covers the remaining 8 stubs and must still pass.

go test -race ./internal/git/    # Expected: exit 0 — S1's run() tests, S2's RevParseHEAD tests,
                                 # S3's WriteTree tests, TestStubsPanic (8 stubs), AND the 6 new
                                 # CommitTree tests all pass.

make test                        # Expected: exit 0 (target = go test -race ./...)
```

> **Note on the `TestStubsPanic` edit (gotcha G5):** `git_test.go`'s `TestStubsPanic` currently
> asserts a panic for `CommitTree` (its first `assertPanics` line). Once `CommitTree` is real (no
> panic), `assertPanics` fails with "expected panic, but did not panic". The REQUIRED fix (Task 2)
> is to remove the single `assertPanics(t, "CommitTree", …)` line — this is an allowed exception to
> "don't touch git_test.go" because it is the direct, necessary consequence of implementing
> CommitTree, and a permanently-failing suite is worse. This mirrors exactly what S2 did for
> RevParseHEAD and S3 did for WriteTree. Document the removal in the commit message. Do NOT touch the
> other 8 `assertPanics` lines.

### Level 3: Security & Structural Invariants (the §19 enforcement + scope discipline)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution anywhere in the git wrapper PRODUCTION code (inherited; new code adds none).
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output. (committree_test.go's fixtures use exec.Command directly with []string args +
# default/custom cmd.Env, never a shell — parallel to S1's initRepo, S2's makeEmptyCommit, S3's
# makeMergeConflict; acceptable test-fixture usage. The grep over committree_test.go also returns
# nothing because the fixtures use []string args, not sh -c.)

# No os.Chdir / cmd.Dir in PRODUCTION code (inherited; new code adds none).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output.

# The CommitTree stub is gone (replaced by a real body).
git grep -n 'CommitTree' internal/git/git.go
# Expected: the interface method comment + the func declaration + body; NO line matching
# `panic("gitRunner.CommitTree`.

# runWithInput exists exactly once.
git grep -n 'func (g \*gitRunner) runWithInput' internal/git/git.go
# Expected: exactly one match.

# run() is byte-identical to its landed form (NOT modified).
git grep -n 'func (g \*gitRunner) run\b' internal/git/git.go
# Expected: exactly one match (the original run()); its body unchanged.

# git.go import block has exactly ONE added line ("io").
git diff internal/git/git.go | grep -E '^\+\s*"'
# Expected: exactly one line: `+	"io"`. (runWithInput and CommitTree add no other imports.)

# Only the intended files changed.
git status --porcelain
# Expected EXACTLY:
#   M internal/git/git.go
#   M internal/git/git_test.go
#   ?? internal/git/committree_test.go

# go.mod / go.sum untouched.
git diff --name-only go.mod go.sum
# Expected: NO output.
```

### Level 4: Runtime Smoke Test (prove CommitTree works against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the root/child/bad-tree/stdin behavior the tests assert, against the real binary:
tmp=$(mktemp -d); git -C "$tmp" init -q
git -C "$tmp" config user.name Test; git -C "$tmp" config user.email test@example.com
echo hello > "$tmp/a.txt"; git -C "$tmp" add a.txt
TREE=$(git -C "$tmp" write-tree); echo "TREE=$TREE"

# ROOT commit (no -p), message via stdin (-F -), multi-line with leading-dash body:
MSG=$'feat: root commit\n\nbody line 1\n--weird--leading dashes'
ROOT=$(printf '%s' "$MSG" | git -C "$tmp" commit-tree "$TREE" -F -); echo "ROOT=$ROOT"
git -C "$tmp" cat-file -p "$ROOT" | grep -q '^parent ' && echo "BUG: root has parent" || echo "root: NO parent (correct)"

# CHILD commit (-p ROOT):
CHILD=$(printf '%s' "$MSG" | git -C "$tmp" commit-tree "$TREE" -p "$ROOT" -F -); echo "CHILD=$CHILD"
git -C "$tmp" cat-file -p "$CHILD" | grep -q "^parent $ROOT" && echo "child: parent linked (correct)"

# Message that STARTS with a dash — safe with -F -, catastrophic with -m:
DASHMSG=$'-n -p --foo'
DSHA=$(printf '%s' "$DASHMSG" | git -C "$tmp" commit-tree "$TREE" -F -)
echo "dash-message stored verbatim: [$(git -C "$tmp" log --format=%B --no-walk "$DSHA" | tr -d '\n')]"

# Bad tree → exit 128:
printf 'x' | git -C "$tmp" commit-tree 0000000000000000000000000000000000000000 -F - 2>&1; echo "BADTREE_EXIT=$?"
rm -rf "$tmp"
# Expected: ROOT/CHILD are 40-hex; root has NO parent; child links parent; dash-message stored
# verbatim (-n -p --foo); bad tree exits 128 with "is not a valid object".
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (the `io` import resolves).
- [ ] `go test -race ./internal/git/` exits 0 (6 new `TestCommitTree_*` + S1/S2/S3 tests + trimmed TestStubsPanic pass).
- [ ] `make test` exits 0.

### Feature Validation

- [ ] `(*gitRunner).CommitTree` body matches §Blueprint (builds args, delegates to `runWithInput`, branches on `code != 0`).
- [ ] `(*gitRunner).runWithInput` exists and is run() + `cmd.Stdin = stdin`.
- [ ] With `parents==nil` (root): returns `(40-hex-sha, nil)`; `cat-file -p` shows NO `parent` line.
- [ ] With `parents=[p]` (child): returns `(40-hex-sha, nil)`; `cat-file -p` shows `parent <p>`.
- [ ] A message with leading dashes, quotes, and newlines is stored byte-for-byte (roundtrip via `git log --format=%B`).
- [ ] On a bad tree SHA: returns a non-nil error whose message contains `"git commit-tree: failed"`.
- [ ] On a missing git binary: returns a non-nil error mentioning "git binary not found" (NOT a commit success).
- [ ] On a cancelled context: returns `errors.Is(err, context.Canceled)`.
- [ ] No production-code SHA validation added (regex is test-only).
- [ ] No ref is moved (the returned SHA is dangling — verify via the smoke test that `HEAD` is unchanged after CommitTree).

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] NO `cmd.Dir` / `os.Chdir` in `internal/git/git.go` (Level 3 grep → no matches).
- [ ] `run()` is NOT re-implemented or modified (delegated to by nothing here; byte-identical).
- [ ] NO change to the git.go import block beyond adding `"io"` (gotcha G14).
- [ ] `Git` interface, `gitRunner`, `New`, `FileChange`, `StagedDiffOptions`, `RevParseHEAD`, `WriteTree` unchanged.
- [ ] The other 8 method stubs untouched (still panic with their owning-subtask messages).
- [ ] `go.mod` / `go.sum` unchanged (no new deps).
- [ ] Only `internal/git/git.go` (io import + runWithInput + CommitTree body), `internal/git/git_test.go` (one-line TestStubsPanic removal), and `internal/git/committree_test.go` (new) are changed.
- [ ] No signal handling / `SysProcAttr` / process-group code added (that is P1.M4.T2).

### Documentation & Deployment

- [ ] Doc comment on `CommitTree` explains the `-F -` stdin choice (FINDING 4), the root-vs-child
      `parents` semantics, and the "no ref moved" guarantee (§13.2, §18.1).
- [ ] Doc comment on `runWithInput` explains why it exists (run() can't do stdin) and that it does
      NOT set `cmd.Env` (inherits parent env).
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't try to deliver the message via `run()` — it never sets `cmd.Stdin`, so `-F -` would read an
  empty message. Use `runWithInput` (gotcha G1). And don't add `-m <msg>` to the args instead of
  `-F -` — `-m` misparses a message beginning with `-` as git flags (FINDING 4, gotcha G10; verified
  empirically that `"-n -p --foo"` is stored verbatim only via `-F -`).
- ❌ Don't modify `run()` to add a stdin parameter — S2/S3 forbid it, S3 lands concurrently, and it
  would cascade edits to RevParseHEAD/WriteTree callers. Add the separate `runWithInput` helper
  (gotcha G2, decision D1). The near-duplication is intentional and documented.
- ❌ Don't change the `CommitTree` signature to take a singular `parentSHA string` — the `Git`
  interface (landed by S1) is `parents []string` and is authoritative. Map empty slice ⇒ root,
  each element ⇒ one `-p` (gotcha G3, decision D3). Do NOT add an `isUnborn` parameter.
- ❌ Don't check `exitCode` before `err` and forget the `err != nil` guard — a missing git binary
  yields `exitCode == -1`, which must surface as a real error, not reach the `code != 0` branch as a
  false "commit failure" (gotcha G4; guarded by `TestCommitTree_GitBinaryMissing`).
- ❌ Don't forget to add `"io"` to the imports — `runWithInput`'s parameter is `io.Reader`. The
  compiler catches it, but state it up front (gotcha G14). CommitTree itself needs no new import
  (`strings.NewReader`, `strings.TrimSpace`, `fmt.Errorf` are already present).
- ❌ Don't inject identity into `CommitTree`/`runWithInput` via `cmd.Env` in production — Stagecoach
  commits AS the configured user (inherits parent env). Identity injection is a TEST-fixture concern
  via `git config user.name/user.email` (gotcha G6, decision D6).
- ❌ Don't forget to remove the `CommitTree` line from `git_test.go`'s `TestStubsPanic` — once
  CommitTree is real it no longer panics, and `assertPanics` fails (gotcha G5). This is the one
  required touch to git_test.go; mirror S2/S3's removals.
- ❌ Don't add SHA-format validation (hex/length checks) to the production method — on exit 0 return
  the trimmed stdout (gotcha G12). Keep the regex TEST-only.
- ❌ Don't call `g.WriteTree(ctx)` inside `CommitTree`'s tests to get a tree SHA — that couples this
  method's unit tests to S3's WriteTree. Get the tree via the `writeTreeOf` fixture (`git write-tree`
  directly) so CommitTree is tested in isolation (gotcha G8).
- ❌ Don't redeclare `initRepo` / `minGitEnv` / `makeEmptyCommit` / `makeMergeConflict` in
  `committree_test.go` — they live in git_test.go / revparse_test.go / writetree_test.go (same
  package). Reuse them; use distinct names for the new helpers (gotcha G7).
- ❌ Don't write the test as `package git_test` (black-box) — CommitTree delegates to the unexported
  `runWithInput`, and the fixtures reuse unexported-helpers' neighbors; the test MUST be `package git`
  (gotcha G13). Match S1/S2/S3's package.
- ❌ Don't cancel the context mid-run in the cancel test — cancel BEFORE the call for determinism, and
  assert via `errors.Is(err, context.Canceled)`.
- ❌ Don't compare the roundtripped message without `TrimSpace` — `git log --format=%B` appends one
  trailing `\n`; pass messages without a trailing newline and compare `TrimSpace(retrieved) == msg`
  (gotcha G9).
- ❌ Don't touch `run()`, `RevParseHEAD`, `WriteTree`, `revparse_test.go`, `writetree_test.go`, or any
  other method/file — those are other subtasks' deliverables (gotcha G2 + scope discipline).
- ❌ Don't implement `UpdateRefCAS` (S5), the rescue protocol (P1.M3.T3), or the orchestrator
  (P1.M3.T4) — those consume `CommitTree`'s output but are separate subtasks.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: This is a small, fully-specified change that reuses the exact `run()`-delegation pattern
S2/S3 already proved end-to-end, with the one well-understood addition of a stdin-capable twin
(`runWithInput`). The `CommitTree` body is ~12 lines and the `runWithInput` body is `run()` plus one
line (`cmd.Stdin = stdin`); both are verified-equivalent to throwaway invocations run against git
2.54.0 on this exact box (root ⇒ no `parent` line in `cat-file -p`; child ⇒ `parent <p>`; bad tree ⇒
exit 128 `fatal: ... is not a valid object`; `-F -` preserves a message beginning with `-n -p --foo`
byte-for-byte). The two non-obvious design calls — (1) add `runWithInput` rather than modify `run()`
(decision D1), and (2) the interface's `parents []string` is authoritative over the contract prose's
singular `parentSHA` (decision D3) — are both explicitly resolved with rationale, so the implementing
agent is not left to guess. The single mechanical step — adding `"io"` to imports — is called out
(gotcha G14); unlike S2 (which added `strings` as its one import edit), there is no risk of `undefined`
beyond `io`. The six test cases each map to a distinct, verified branch (root/ child/ stdin-special-
chars/ bad-tree/ LookPath-miss/ ctx-cancel) with deterministic fixtures; identity is handled via
repo-local config (robust in CI), and the message-roundtrip assertion accounts for `%B`'s trailing
newline (gotcha G9). The one cross-cutting edit (remove the `CommitTree` line from `TestStubsPanic`)
is anticipated with an explicit, scoped resolution (Task 2 / Level-2 note) mirroring S2/S3, and the
parallel-execution note confirms the git.go/git_test.go edit regions are distinct from S3's. No
external dependencies, no interface changes, no ambiguity in the contract.
