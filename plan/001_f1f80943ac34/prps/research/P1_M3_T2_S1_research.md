# Research — P1.M3.T2.S1: internal/git/plumbing.go — RevParseHEAD, WriteTree, CommitTree, UpdateRefCAS

All behaviors below verified empirically on the host (git 2.54.0) by driving
the REAL git binary in temp repos, exactly as `internal/git/git.go`'s
`(g *Git) run(args ...string)` does. Baseline `go test ./internal/git/` is
GREEN (S1 git.go + S1 git_test.go + S2 gittestutil_test.go all shipped).

## 0. Contract (verbatim from work item)
- INPUT: `git.Git` (P1.M3.T1.S1 — DONE/shipped): `New(dir)`, unexported
  `run(args...) (string,error)`, typed `*ExitError{Args,Code,Stderr}`.
- LOGIC:
  - `RevParseHEAD() (sha string, hasParent bool, err error)` — `rev-parse HEAD`;
    hasParent=false on rootless repo (treat non-zero/empty as ok=false, NOT error).
  - `WriteTree() (sha, err)` — `write-tree`; detect 'unresolved merge conflicts'
    in stderr → return clear error (FR8) BEFORE any generation.
  - `CommitTree(parent, msg, tree string) (sha, err)` — `commit-tree -m <msg> <tree>`;
    OMIT `-p` when parent=="" (root commit, FR39).
  - `UpdateRefCAS(ref, newSHA, expected string) error` — `update-ref <ref> <newSHA> <expected>`
    (2-arg CAS form); when expected=="" (root commit) use the no-expected form.
    NEVER `--force`.
- MOCKING: temp-repo tests via the S2 harness (newTempRepo/seedCommits/writeFileStage).
- OUTPUT: atomicity primitives consumed by generate commit step (P1.M6.T1.S1)
  and asserted by invariant tests (P1.M6.T3.S3).
- RESEARCH: reference_impl.md §1; decisions.md §7 (safety invariants); PRD §13.2.

## 1. The S1 `run` contract these methods build on (verified against shipped git.go)
- `run` returns RAW stdout (trailing "\n" INCLUDED) on success, or a typed
  `*ExitError` on a non-zero git exit, or a wrapped non-typed error on a start
  failure. So each plumbing method:
  - on success → `strings.TrimSpace(out)` to get the clean SHA.
  - on a non-zero exit → `errors.As(err, &ee)` into `*git.ExitError` and read
    `ee.Stderr` (our buffer — NOT `(*exec.ExitError).Stderr`, which is empty).
- `run` is UNEXPORTED → plumbing.go is in `package git` (same package), so it
  calls `g.run(...)` directly. This file uses a PLAIN `package git` line (git.go
  OWNS the `// Package git` doc — do NOT add a second one).

## 2. Verified git behaviors (the spec ground truth)

### 2a. `git rev-parse HEAD` (RevParseHEAD)
| repo state | stdout | exit | stderr |
|---|---|---|---|
| UNBORN (fresh init, 0 commits) | `HEAD` (the literal word) | 128 | `fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.` |
| ≥1 commit | `<40-hex-sha>\n` | 0 | (empty) |

- ⇒ unborn detection: `run` returns `*ExitError{Code:128, Stderr:"...ambiguous argument 'HEAD': unknown revision..."}`.
  Detect via `ee.Code==128 && strings.Contains(ee.Stderr, "unknown revision")` (stable,
  semantically precise: HEAD points at an unknown revision = no commits yet). In that case
  return `("", false, nil)` — NOT an error (contract: "treat non-zero/empty as ok=false").
  A DIFFERENT non-zero error (e.g. not-a-repo → stderr contains "not a git repository") is a
  REAL error → return it wrapped/typed as-is.
- On success `sha = strings.TrimSpace(out)`. Defensive: if `sha == ""` also return
  `("", false, nil)` (contract: "empty as ok=false").
- Alternative considered (NOT used — contract says plain `rev-parse HEAD`):
  `git rev-parse --verify -q HEAD` → exit 1 + empty stderr + empty stdout on unborn (cleaner
  but a different exit code & no message). Stick to the contract's `rev-parse HEAD`.

### 2b. `git write-tree` (WriteTree)
| index state | stdout | exit | stderr |
|---|---|---|---|
| normal (any staged content, incl. empty index) | `<40-hex-tree-sha>\n` | 0 | (empty, 0 bytes) |
| EMPTY index (e.g. unborn repo, nothing staged) | `4b825dc642cb6eb9a060e54bf8d69288fbee4904\n` | 0 | (empty) |
| unresolved MERGE CONFLICT in index | (empty) | 128 | per-file `<path>: unmerged (<sha>)` lines + `fatal: git-write-tree: error building trees` |

- ⚠️ FR8 wording says "unresolved merge conflicts" but git 2.54.0 NEVER prints that
  literal phrase. The actual conflict signal is the substring **`unmerged`** (one line per
  conflicting path) plus `fatal: git-write-tree: error building trees`. So WriteTree's conflict
  detection MUST match `unmerged` (or `error building trees`), NOT the literal FR8 string.
- ⇒ WriteTree: `out, err := g.run("write-tree")`. On err → `errors.As(&ee)`; if
  `strings.Contains(ee.Stderr, "unmerged")` return a CLEAR wrapped error, e.g.
  `fmt.Errorf("git: unresolved merge conflicts in index (resolve them before generating): %s", ee.Stderr)`
  (surface the git detail). Any OTHER non-zero exit → return the typed error as-is. On success
  return `(strings.TrimSpace(out), nil)`.

### 2c. `git commit-tree` (CommitTree)
| invocation | stdout | exit | touches refs? |
|---|---|---|---|
| `commit-tree -m <msg> <tree>` (ROOT, no -p) | `<40-hex-commit-sha>\n` | 0 | NO |
| `commit-tree -p <parent> -m <msg> <tree>` | `<40-hex-commit-sha>\n` | 0 | NO |

- ⇒ CommitTree: build args conditionally on `parent != ""`:
  - `parent == ""` (root, FR39): `args := []string{"commit-tree", "-m", msg, tree}` (OMIT `-p`).
  - `parent != ""`: `args := []string{"commit-tree", "-p", parent, "-m", msg, tree}`.
  - Call `g.run(args...)`; on err return as-is; on success `return (strings.TrimSpace(out), nil)`.
- `msg` is passed as ONE `-m` arg (verified in S2 research: a single -m preserves newlines/
  multi-paragraph bodies verbatim via `git log --format=%B`; do NOT split into multiple -m).
- commit-tree does NOT touch HEAD/refs (verified: after a root commit-tree, `rev-parse HEAD`
  still exits 128 on an unborn repo — only update-ref advances HEAD). ✓ matches PRD §13.2.

### 2d. `git update-ref` CAS (UpdateRefCAS)
| invocation | condition | exit | HEAD after |
|---|---|---|---|
| `update-ref HEAD <new>` (no expected — ROOT form) | — | 0 | = `<new>` |
| `update-ref HEAD <new> <expected>` (2-arg CAS) | HEAD == expected | 0 | = `<new>` (ADVANCED) |
| `update-ref HEAD <new> <expected>` (2-arg CAS) | HEAD != expected | 128 | UNCHANGED (== actual, not new) |

- CAS-FAILURE stderr (HEAD moved): `fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': is at <actual> but expected <expected>` (exit 128).
- ⚠️ ATOMIC INVARIANT CONFIRMED: on CAS failure HEAD is byte-for-byte unchanged (still the
  pre-call actual value, NOT newSHA). This is the §18.1 safety invariant the tests MUST assert.
- ⇒ UpdateRefCAS: build args conditionally on `expected != ""`:
  - `expected == ""` (root commit): `args := []string{"update-ref", ref, newSHA}` (NO expected-old — 1-arg form is ONLY legal here, decisions.md §7).
  - `expected != ""`: `args := []string{"update-ref", ref, newSHA, expected}` (3-arg CAS form).
  - Call `g.run(args...)`; return `nil` on success, the typed error on failure. NEVER append `--force`.
- The CAS failure is a NORMAL, routable `*ExitError` (exit 128); the generate layer turns it
  into the §18.2 "HEAD moved" message + exit 1. UpdateRefCAS itself just surfaces git's error.

## 3. Safety invariants under test (decisions.md §7 — these tests MUST assert them)
- Never call `git update-ref` without expected-old EXCEPT the root commit (expected=="").
- Never `--force`. Never `git commit`. (plumbing.go uses neither.)
- ATOMIC HEAD: after a CAS failure, `git rev-parse HEAD` is UNCHANGED.
- IDEMPOTENT INDEX: write-tree/commit-tree/update-ref do NOT modify the index; assert
  `git diff --cached --name-only` is unchanged across WriteTree/CommitTree (snapshot immutability:
  `git cat-file -p <TREE_SHA>` is stable after later staging).

## 4. Required MOCKING/test scenarios (from contract) — all via S2 harness + real git
1. ROOT commit (parent==""): CommitTree omits -p; UpdateRefCAS(expected=="") advances HEAD;
   RevParseHEAD returns hasParent=false on the unborn repo BEFORE the root commit.
2. Normal CAS: seed ≥1 commit → RevParseHEAD hasParent=true; write-tree → tree; commit-tree -p
   → newSHA; UpdateRefCAS(expected=parent) SUCCEEDS; HEAD == newSHA.
3. CAS SUCCEEDS when HEAD unchanged (== expected).
4. CAS FAILS (returns err) when HEAD moved concurrently: capture parent; move HEAD (seedCommit/
   `git commit` via harness or a direct `g.run("commit",...)`); then UpdateRefCAS with the STALE
   expected → returns non-nil error; AND assert HEAD is UNCHANGED afterward (atomic invariant).
5. WriteTree errors CLEARLY on a STAGED CONFLICT: construct a real merge conflict in the temp
   repo (divergent branches + `git merge` → CONFLICT, leaving `UU <path>` in the index), then
   assert WriteTree returns an error whose message contains "conflict" (FR8 clarity). NOTE:
   constructing a conflict requires the index to hold unmerged stages — a porcelain `git merge`
   that hits a content conflict leaves exactly that; OR use `git update-index --cacheinfo` per
   stage. Easiest reproducible path: seed a base commit, two branches changing the same line,
   `git merge` → CONFLICT (exit 1, fine) → index now has `UU` entries → WriteTree fails.

## 5. Dependency / scope discipline (anti-regression)
- Depends on S1 (DONE) + S2 harness (DONE) ONLY. Uses `g.run` + `errors.As(&ee)` into `*ExitError`.
- ONE NEW file `internal/git/plumbing.go` + ONE NEW file `internal/git/plumbing_test.go`
  (white-box `package git`, stdlib `testing` ONLY, drives the REAL binary via the S2 harness).
- DO NOT touch git.go/git_test.go/gittestutil_test.go, main.go, Makefile, go.mod, go.sum,
  internal/ui, internal/provider. DO NOT run `go mod tidy`. No go-git, no testify.
- DOCS = Mode A: godoc on each exported method citing PRD §13.2/§13.5/§18.1, FR7/FR8/FR39/FR40/FR41,
  and decisions.md §7 (safety invariants). No README/docs created.

## 6. Validation gates (verified working on host)
- `go build ./internal/git/` → 0 (builds non-test files; plumbing.go added).
- `go vet ./internal/git/` → clean (COMPILES `_test.go` incl. plumbing_test.go).
- `test -z "$(gofmt -l internal/git/)"` → empty.
- `go test ./internal/git/` → PASS (plumbing_test.go + existing S1/S2 tests).
- `go test ./...` → whole-module green.
