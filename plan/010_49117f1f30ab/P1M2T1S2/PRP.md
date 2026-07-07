---
name: "P1.M2.T1.S2 — Scoped ReadTreeInto/WriteTreeFrom variants (GIT_INDEX_FILE-aware)"
description: |
  The scoped-index git primitives for v2.4 hook execution (FR-V3 → G22). `internal/git/git.go`'s existing
  `ReadTree` (1222) and `WriteTree` (492) operate on the repo's DEFAULT `.git/index` — they MUST NOT be used
  for scoped pre-commit (they'd mutate the live index and break the §5 stage-while-generating freeze). This
  task adds two THROWAWAY-index siblings that route through `runWithEnv` (P1.M2.T1.S1, ALREADY APPLIED) with
  `GIT_INDEX_FILE=<abs tmp>`: `ReadTreeInto(ctx, tree, indexFile)` primes a throwaway index from a frozen tree;
  `WriteTreeFrom(ctx, indexFile)` captures the post-hook tree. Both leave the live `.git/index` UNTOUCHED. PRD:
  §9.25 FR-V3 (pre-commit scoped to T_start). Research: `architecture/codebase_reality.md` §3 (the scoped-index
  gap), `architecture/external_deps.md` §8 (the GIT_INDEX_FILE absolute-path gotcha + the faithful sequence),
  and `research/s2_scoped_variants_map.md` (verified-against-live touchpoint map with full method bodies).

  ⚠️ **THE central design call — mirror ReadTree/WriteTree EXACTLY, swapping `g.run(...)` → `g.runWithEnv(ctx,
  g.workDir, []string{"GIT_INDEX_FILE=" + absIndex}, ...)`.** ReadTreeInto is ReadTree + a scoped index;
  WriteTreeFrom is WriteTree + a scoped index (incl. the scoped `ls-files -u` merge-conflict probe). Same
  LookPath/-C/[]string/separate-buffer/exit-code-semantics (inherited from runWithEnv, which mirrors run).
  ALL non-zero exits are errors (the shared mutation convention; no 128 special-case). The live `.git/index`
  is NEVER touched — that is the WHOLE POINT (FR-V3: pre-commit sees T_start, not the live staging area).

  ⚠️ **THE second design call — indexFile MUST be absolute; the variants enforce it via `filepath.Abs`.**
  external_deps.md §8 gotcha #1: "GIT_INDEX_FILE must be ABSOLUTE (a relative value resolves against the
  hook's CWD)." The hook subprocess runs with CWD=worktree root; a relative indexFile would resolve wrong.
  Each variant opens with `absIndex, err := filepath.Abs(indexFile); if err != nil { return ..., fmt.Errorf
  ("...: resolve index path: %w", err) }` then `extraEnv := []string{"GIT_INDEX_FILE=" + absIndex}`.
  (filepath.Abs is a no-op for already-absolute paths — os.CreateTemp/t.TempDir results are absolute, the
  normal case; it exists to CATCH a relative path.) git.go ALREADY imports `"path/filepath"` (S1 added `"os"`)
  → ZERO new imports.

  ⚠️ **THE third design call — S1 (runWithEnv) is ALREADY APPLIED; S2 needs ZERO new imports.** Verified live:
  git.go imports `"os"` (line 9) + `"path/filepath"` (line 11); `runWithEnv` exists on `*gitRunner`; git_test.go
  has `TestGitRunner_RunWithEnv_PassesEnv`. S2 consumes runWithEnv directly — no import changes, no go.mod change.
  Do NOT re-add `"os"` (S1 did it) or any other import.

  ⚠️ **THE fourth design call — BOTH methods go ON the `Git` interface (the public scoped surface), unlike
  runWithEnv which stays off-interface.** The Git interface (git.go:87) exposes ReadTree/WriteTree/etc.; the
  scoped variants are the public surface the hook runner (M3) calls. Place `WriteTreeFrom` immediately after
  `WriteTree` (interface ~95) and `ReadTreeInto` immediately after `ReadTree` (interface ~169) — co-located
  with their unscoped siblings (the established "co-located" discipline: run/runWithInput/runWithEnv; one
  *_test.go per primitive). The gitRunner impls go next to their siblings too (WriteTreeFrom after WriteTree
  ~492; ReadTreeInto after ReadTree ~1222).

  ⚠️ **THE fifth design call — the keystone test asserts the LIVE `.git/index` is byte-identical before/after.**
  This is what distinguishes the scoped variants from ReadTree/WriteTree. Each test snapshots the live index
  (`git ls-files` or a hash of `.git/index`) before the scoped call, runs it, and asserts the live index is
  UNCHANGED — while the throwaway index (inspected via an INDEPENDENT oracle: `GIT_INDEX_FILE=<tmp> git
  ls-files`) holds the tree / produces the SHA. Mirror the dedicated per-primitive test files
  (`readtree_test.go`, `writetree_test.go`) + their helpers (`initRepo`/`writeFile`/`stageFile`/`writeTreeOf`/
  `execGit`). NEW files: `readtreeinto_test.go` + `writetreefrom_test.go`.

  Deliverable (edits to `internal/git/git.go` + 2 NEW test files): (1) add `ReadTreeInto` + `WriteTreeFrom` to
  the `Git` interface (co-located with their siblings) + their `*gitRunner` impls (co-located with siblings);
  (2) `readtreeinto_test.go` + `writetreefrom_test.go` (happy path + the live-index-untouched keystone +
  error paths). INPUT = runWithEnv (S1, done) + ReadTree/WriteTree (the templates) + DiffTreeNames (the
  read-only reuse target for P1.M2.T2.S1's subset check — NOT touched here). OUTPUT = two scoped git primitives
  that prime/capture a throwaway index without touching the live one; the foundation for M3's scoped pre-commit.
  DOCS = none (internal git primitives). SCOPE: `internal/git/{git.go, readtreeinto_test.go, writetreefrom_test.go}`
  ONLY. Do NOT touch run()/runWithInput()/runWithEnv() (frozen), ReadTree/WriteTree (the templates — unchanged),
  DiffTreeNames (P1.M2.T2.S1's consumer), or any hook-runner/config/cli file (M3/M1).
---

## Goal

**Feature Goal**: Add two GIT_INDEX_FILE-aware git primitives — `ReadTreeInto(ctx, tree, indexFile)` and
`WriteTreeFrom(ctx, indexFile)` — to `internal/git/git.go`, scoped siblings of `ReadTree`/`WriteTree` that
operate on a THROWAWAY index file (via `runWithEnv` + `GIT_INDEX_FILE=<abs tmp>`) and leave the repo's live
`.git/index` byte-identical. These are the read-tree (prime from frozen tree) and write-tree (capture
post-hook tree) primitives the v2.4 commit-hooks runner (FR-V3) needs to scope `pre-commit` to the snapshot
tree `T_start` without violating the §5 stage-while-generating freeze.

**Deliverable** (edits to git.go + 2 new test files):
1. **`internal/git/git.go`** — (a) add `ReadTreeInto(ctx context.Context, tree, indexFile string) error` and
   `WriteTreeFrom(ctx context.Context, indexFile string) (sha string, err error)` to the `Git` interface
   (co-located with ReadTree/WriteTree); (b) add their `*gitRunner` impls (co-located with the ReadTree/WriteTree
   impls), each mirroring its unscoped sibling exactly except `g.run(...)` → `g.runWithEnv(ctx, g.workDir,
   []string{"GIT_INDEX_FILE=" + absIndex}, ...)` and a leading `filepath.Abs(indexFile)`.
2. **`internal/git/readtreeinto_test.go`** (NEW) — happy path (tree loaded into tmpIndex) + the live-index-
   untouched keystone + BadTree + GitBinaryMissing + ContextCancelled.
3. **`internal/git/writetreefrom_test.go`** (NEW) — happy path (SHA captured from a primed tmpIndex) + the
   live-index-untouched keystone + EmptyIndex (→ EmptyTreeSHA) + GitBinaryMissing + ContextCancelled.

**Success Definition**: `gofmt -l`, `go vet ./...`, `go build ./...`, `go test -race ./internal/git/`,
`go test -race ./...`, `make lint` all clean/green; ReadTreeInto primes a throwaway index from a tree WITHOUT
touching `.git/index`; WriteTreeFrom returns the tree SHA of a throwaway index WITHOUT touching `.git/index`;
both enforce an absolute `GIT_INDEX_FILE` (via `filepath.Abs`); go.mod/go.sum byte-unchanged; zero new imports
(S1 landed `"os"` + `"path/filepath"`); only `internal/git/{git.go, readtreeinto_test.go, writetreefrom_test.go}`
touched; ReadTree/WriteTree/run/runWithInput/runWithEnv byte-unchanged.

## User Persona

**Target User**: The commit-hooks runner (P1.M3.T1 — `internal/hooks`) which scopes `pre-commit` to the
snapshot tree `T_start` via a throwaway index (FR-V3). Transitively, every user who runs `stagecoach` on a
repo with `pre-commit`/`commit-msg` hooks (US19, FR-V1) — their hooks now fire on the plumbing path against
the snapshotted content, not the live index.

**Use Case**: (internal primitive, no user-visible behavior yet) the hook runner does:
`tmp := os.CreateTemp(...); defer os.Remove(tmp); g.ReadTreeInto(ctx, frozenTree, tmp); <run pre-commit with
GIT_INDEX_FILE=tmp in its env>; newTree, _ := g.WriteTreeFrom(ctx, tmp); <DiffTreeNames subset check (P1.M2.T2.S1)>`.
ReadTreeInto primes; WriteTreeFrom captures; the live index is untouched throughout.

**User Journey**: (future) `stagecoach` → snapshot tree → ReadTreeInto(T_start, tmp) → pre-commit runs against
tmp (a formatter re-stages into tmp) → WriteTreeFrom(tmp) → commit-tree against the hook-fixed tree. The
freeze (§5) holds: files the user stages DURING generation never reach the in-flight commit.

**Pain Points Addressed**: removes the structural gap (no index-mutating primitive can target a throwaway
index) that blocked scoped pre-commit. Without these, a scoped hook would either touch the live index
(breaking the freeze) or couldn't run read-tree/write-tree at all.

## Why

- **The hard primitive for FR-V3.** `pre-commit` scoped to `T_start` requires priming a throwaway index from
  the frozen tree and capturing the post-hook tree — without ever touching `.git/index`. ReadTreeInto +
  WriteTreeFrom are exactly those two operations, layered on S1's `runWithEnv`.
- **Mirrors the proven ReadTree/WriteTree.** Each scoped variant is its unscoped sibling + a `GIT_INDEX_FILE`
  env var — same exit-code semantics, same mutation convention, same merge-conflict probe (WriteTreeFrom).
  Low-risk, idiomatic for this file.
- **Unblocks M3 cleanly.** M3 (the hook runner) calls these two public interface methods; it does NOT call
  runWithEnv directly (S1 kept runWithEnv off the interface precisely so these are the public surface).
- **No API/config/deps change.** Two interface methods + two impls + tests. go.mod unchanged (S1 landed the
  only import this needs: `"os"`; `"path/filepath"` was already present).

## What

A compiled `internal/git` package exporting `ReadTreeInto` + `WriteTreeFrom` on the `Git` interface (scoped
siblings of ReadTree/WriteTree), plus their tests. No new types, no dependency change, no config change.

### Success Criteria

- [ ] `ReadTreeInto(ctx context.Context, tree, indexFile string) error` is on the `Git` interface (after
      ReadTree, ~169) + a `*gitRunner` impl (after ReadTree, ~1222). Body: `filepath.Abs(indexFile)` →
      `g.runWithEnv(ctx, g.workDir, []string{"GIT_INDEX_FILE=" + absIndex}, "read-tree", tree)` →
      `code != 0 → fmt.Errorf("git read-tree (scoped): failed (exit %d): %s", code, strings.TrimSpace(stderr))`.
- [ ] `WriteTreeFrom(ctx context.Context, indexFile string) (sha string, err error)` is on the `Git` interface
      (after WriteTree, ~95) + a `*gitRunner` impl (after WriteTree, ~492). Body: `filepath.Abs(indexFile)` →
      `g.runWithEnv(ctx, g.workDir, env, "write-tree")` → on `code != 0` the scoped `ls-files -u` probe (mirror
      WriteTree) → success `return strings.TrimSpace(stdout), nil`.
- [ ] Both variants enforce an ABSOLUTE `GIT_INDEX_FILE` via `filepath.Abs` (external_deps.md §8 #1).
- [ ] ZERO new imports in git.go (`"os"` + `"path/filepath"` already present from S1); go.mod/go.sum unchanged.
- [ ] `readtreeinto_test.go` + `writetreefrom_test.go` exist; each has a happy-path test + a LIVE-INDEX-
      UNTOUCHED keystone test (the live `.git/index` is byte-identical before/after) + error-path tests.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go test -race ./internal/git/`,
      `go test -race ./...`, `make lint` all clean/green; ReadTree/WriteTree/run/runWithInput/runWithEnv
      byte-unchanged; only git.go + the 2 new test files touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the two method bodies (quoted
verbatim below + in `research/s2_scoped_variants_map.md` §4), the co-locate-with-sibling placement, the
`filepath.Abs` gotcha, the zero-new-imports fact, and the test patterns (mirror readtree_test.go/writetree_test.go
+ the live-index-untouched keystone). No PRD/hook/provider knowledge beyond "these prime/capture a throwaway
index without touching the live one".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/010_49117f1f30ab/P1M2T1S2/research/s2_scoped_variants_map.md
  why: the AUTHORITATIVE verified-against-live touchpoint map. §1 = S1 done (imports present); §2 = the
       ReadTree/WriteTree templates; §3 = the filepath.Abs gotcha; §4 = the FULL method bodies (copy them);
       §5 = interface placement (co-locate with siblings); §6 = DiffTreeNames reuse (NOT this task); §7 =
       test patterns + the live-index-untouched keystone.
  critical: §4 (copy the bodies verbatim) + §3 (filepath.Abs is REQUIRED — a relative GIT_INDEX_FILE silently
       resolves against the hook's CWD) + §7 (the keystone test is what proves the live index is untouched).

- docfile: plan/010_49117f1f30ab/architecture/external_deps.md
  section: "8. ⚠️ The central design tension" → "Gotchas to handle" #1 (GIT_INDEX_FILE absolute) + the
           "Recommended faithful sequence" (read-tree → hook → write-tree → subset check).
  why: the absolute-path gotcha (#1) and the scoped-index sequence these primitives implement (read-tree
       <frozenTree> into tmp; write-tree from tmp after the hook). The faithful sequence is the contract.
  critical: #1 — GIT_INDEX_FILE MUST be absolute. #3 — a hook that hardcodes .git/index is caught by the
       subset check (P1.M2.T2.S1), NOT by these primitives (they only prime/capture).

- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "3. The git primitives — signatures + scoped-index gap"
  why: confirms ReadTree (1222, REPLACES .git/index) / WriteTree (492, READS .git/index) accept NO custom
       index path → the scoped variants are REQUIRED. Recommends "(a) the env seam + minimal scoped variants".
  critical: the scoped-index gap is the reason these exist. DiffTreeNames is listed as the read-only reuse
       target for the subset check (P1.M2.T2.S1, NOT this task).

- file: internal/git/git.go   (the file you EDIT)
  section: Git interface (87 — add the 2 methods, co-located with ReadTree/WriteTree); ReadTree impl (1222);
           WriteTree impl (492 — incl. the ls-files -u probe to mirror); gitRunner struct (369); New() (375).
  why: the templates. ReadTreeInto = ReadTree + GIT_INDEX_FILE; WriteTreeFrom = WriteTree + GIT_INDEX_FILE
       (incl. the scoped ls-files -u probe). The interface doc-comment style (multi-line, names PRD refs,
       states the mutation/read-only convention) is the pattern for the new doc comments.
  pattern: swap `g.run(ctx, g.workDir, <args>)` → `g.runWithEnv(ctx, g.workDir, []string{"GIT_INDEX_FILE=" +
           absIndex}, <args>)`. Keep the `code != 0 → fmt.Errorf("git <cmd> (scoped): failed (exit %d): %s",
           code, strings.TrimSpace(stderr))` shape (the "(scoped)" tag distinguishes the error source).
  gotcha: ZERO new imports — "os" + "path/filepath" are already there (S1). runWithEnv is the S1 seam (off the
           interface, called here). Place each impl next to its sibling (WriteTreeFrom after WriteTree 492;
           ReadTreeInto after ReadTree 1222).

- file: internal/git/readtree_test.go + writetree_test.go   (READ ONLY — the test patterns to mirror)
  why: the per-primitive test-file discipline + the helpers + the independent-oracle assertion style.
       readtree_test.go: TestReadTree_LoadsTreeIntoIndex uses `writeFile`/`stageFile`/`makeEmptyCommit`/
       `writeTreeOf`, removes a.txt from the index, ReadTree, then asserts via `exec.Command("git", "ls-files")`.
       writetree_test.go: TestWriteTree_EmptyIndex asserts the canonical EmptyTreeSHA (4b825dc6...).
  pattern: mirror for the scoped variants — BUT add the LIVE-INDEX-UNTOUCHED keystone (snapshot git ls-files
           before, assert unchanged after) AND inspect the throwaway index via an independent oracle
           (`exec.Command("git", "-C", repo, "ls-files")` with `cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmp)`).
  gotcha: helpers `writeFile`/`stageFile` are in committree_test.go (same package — accessible). `writeTreeOf`
           and `execGit` are test helpers already in the package. `New(repo)` returns the Git interface (use it
           for ReadTreeInto/WriteTreeFrom since they ARE on the interface — NOT `&gitRunner{}`).

- file: PRD.md (the bugfix/v2.4 spec)
  section: "9.25 Hook execution on the commit path" (h3.41) — esp. FR-V3 ("pre-commit is scoped to T_start …
           the exact materialization (a throwaway index populated from T_start) is a §12 implementation
           detail"); §13.2 (the plumbing primitives — write-tree/read-tree are index-coupled by default).
  why: FR-V3 is the requirement these primitives satisfy (a throwaway index populated from T_start). §13.2 is
       why the DEFAULT write-tree/read-tree touch .git/index (hence the scoped siblings).
  critical: FR-V3 — the committed tree comes from the (possibly hook-mutated) snapshot, NOT the live index.
       These primitives are how stagecoach primes/captures that throwaway index.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go                  # Git interface (87) + ReadTree (1222) + WriteTree (492) + DiffTreeNames (1582) + runWithEnv (S1, ~460)  ← EDIT (add 2 interface methods + 2 impls)
  readtree_test.go        # TestReadTree_* (patterns to mirror)                                                                                       ← (READ; pattern source)
  writetree_test.go       # TestWriteTree_* (patterns to mirror)                                                                                     ← (READ; pattern source)
  committree_test.go      # helpers: writeFile (31), stageFile (39)                                                                                  ← (READ; helper source)
  readtreeinto_test.go    # NEW — TestReadTreeInto_* (happy + live-index-untouched keystone + error paths)
  writetreefrom_test.go   # NEW — TestWriteTreeFrom_* (happy + live-index-untouched keystone + error paths)
go.mod / go.sum           # unchanged (no new dep; "os"+"path/filepath" already imported)
# NO docs (internal primitives), NO hook runner (M3), NO config/cli (M1), NO subset-check logic (P1.M2.T2.S1).
```

### Desired Codebase tree with files to be added

```bash
internal/git/
  readtreeinto_test.go    # NEW — TestReadTreeInto_LoadsTreeIntoScopedIndex + _LiveIndexUntouched + _BadTree + _GitBinaryMissing + _ContextCancelled
  writetreefrom_test.go   # NEW — TestWriteTreeFrom_PrimedIndex + _LiveIndexUntouched + _EmptyIndex + _GitBinaryMissing + _ContextCancelled
# git.go edited (2 interface methods + 2 impls). go.mod/go.sum unchanged. No other files.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #2): GIT_INDEX_FILE MUST be absolute. external_deps.md §8 #1: "a relative value
// resolves against the hook's CWD." Each variant opens with `absIndex, err := filepath.Abs(indexFile); if
// err != nil { return ..., fmt.Errorf("...: resolve index path: %w", err) }`. filepath.Abs is a no-op for
// absolute paths (the normal case — os.CreateTemp results are absolute); it CATCHES a relative path. git.go
// already imports "path/filepath" (S1 added "os") — ZERO new imports.

// CRITICAL (design call #1): mirror ReadTree/WriteTree EXACTLY, swapping g.run(...) → g.runWithEnv(ctx,
// g.workDir, []string{"GIT_INDEX_FILE=" + absIndex}, ...). Same exit-code semantics (inherited from
// runWithEnv, which mirrors run). ALL non-zero exits are errors (the mutation convention — no 128 special-case).
// WriteTreeFrom mirrors WriteTree's ls-files -u merge-conflict probe — but SCOPED (runWithEnv + GIT_INDEX_FILE)
// so it inspects the THROWAWAY index, not .git/index.

// CRITICAL (design call #4): BOTH methods go ON the Git interface (the public scoped surface). Unlike
// runWithEnv (off-interface, S1), ReadTreeInto/WriteTreeFrom ARE the surface M3 calls. Add them to the
// interface co-located with their siblings (WriteTreeFrom after WriteTree ~95; ReadTreeInto after ReadTree ~169).

// CRITICAL (design call #5): the keystone test asserts the LIVE .git/index is byte-identical before/after.
// This is what distinguishes the scoped variants from ReadTree/WriteTree. Snapshot `git ls-files` (or hash
// .git/index) before; assert unchanged after. Inspect the THROWAWAY index via an INDEPENDENT oracle:
// `exec.Command("git", "-C", repo, "ls-files")` with `cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmp)`.

// GOTCHA: ZERO new imports. git.go already has "os" + "path/filepath" (S1). If `go build` fails on an
// undefined symbol, you used the wrong call (g.run instead of g.runWithEnv) — do NOT add an import.

// GOTCHA: place each impl co-located with its sibling (WriteTreeFrom after WriteTree ~492; ReadTreeInto after
// ReadTree ~1222) — the established discipline (run/runWithInput/runWithEnv are co-located; *_test.go per-primitive).

// GOTCHA: use New(repo) in the tests (returns the Git interface) — ReadTreeInto/WriteTreeFrom ARE on the
// interface, so the public constructor works (unlike runWithEnv tests which need &gitRunner{} white-box).

// GOTCHA: the "(scoped)" tag in the error strings ("git read-tree (scoped): failed …") distinguishes the
// scoped variant's errors from the unscoped ReadTree/WriteTree errors in logs — keep it.

// GOTCHA: do NOT touch run()/runWithInput()/runWithEnv() (frozen — S1's seam), ReadTree/WriteTree (the
// templates — unchanged), DiffTreeNames (P1.M2.T2.S1's consumer — read-only reuse target), or any hook/config/
// cli file (M3/M1). This task is git.go + 2 test files.
```

## Implementation Blueprint

### Data models and structure

No new types. Two methods, each its unscoped sibling + `GIT_INDEX_FILE`:

```go
// internal/git/git.go — interface: add WriteTreeFrom after WriteTree (~95):
	// WriteTreeFrom captures the tree SHA of a THROWAWAY index at indexFile via GIT_INDEX_FILE-scoped
	// `git write-tree` — the scoped sibling of WriteTree. It is the capture half of the v2.4 hook scoped-
	// index mechanism (FR-V3): after ReadTreeInto primes a throwaway index from T_start and the pre-commit
	// hook (run with GIT_INDEX_FILE in its env) may re-stage fixes into it, WriteTreeFrom captures the
	// resulting tree. It READS indexFile, NOT .git/index — the live index is UNTOUCHED. indexFile is made
	// absolute (external_deps.md §8 #1: a relative GIT_INDEX_FILE resolves against the hook's CWD). Like
	// WriteTree, ALL non-zero exits are errors (incl. the scoped ls-files -u merge-conflict probe).
	WriteTreeFrom(ctx context.Context, indexFile string) (sha string, err error)

// internal/git/git.go — interface: add ReadTreeInto after ReadTree (~169):
	// ReadTreeInto primes a THROWAWAY index at indexFile from <tree> via GIT_INDEX_FILE-scoped
	// `git read-tree` — the scoped sibling of ReadTree. It is the prime half of the v2.4 hook scoped-index
	// mechanism (FR-V3): pre-commit runs against T_start (the snapshot tree), NOT the live staging area, so
	// the §5 stage-while-generating freeze holds. It WRITES indexFile, NOT .git/index — the live index is
	// UNTOUCHED. indexFile is made absolute (external_deps.md §8 #1). ALL non-zero exits are errors (the
	// shared mutation convention — no 128-as-non-error special-case, same as ReadTree).
	ReadTreeInto(ctx context.Context, tree, indexFile string) error
```

```go
// internal/git/git.go — *gitRunner impl of WriteTreeFrom, placed immediately after WriteTree (~492):
// WriteTreeFrom is the scoped sibling of WriteTree: identical except it reads the throwaway index at
// indexFile (via GIT_INDEX_FILE) instead of .git/index. The live index is NEVER touched (FR-V3 scoped
// pre-commit). indexFile is made absolute (external_deps.md §8 #1 — a relative value resolves against the
// hook's CWD). Mirrors WriteTree's ls-files -u merge-conflict probe, scoped to the throwaway index.
func (g *gitRunner) WriteTreeFrom(ctx context.Context, indexFile string) (sha string, err error) {
	absIndex, err := filepath.Abs(indexFile)
	if err != nil {
		return "", fmt.Errorf("git write-tree (scoped): resolve index path: %w", err)
	}
	env := []string{"GIT_INDEX_FILE=" + absIndex}
	stdout, stderr, code, err := g.runWithEnv(ctx, g.workDir, env, "write-tree")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (runWithEnv sets code=-1)
	}
	if code != 0 {
		// Scoped merge-conflict probe (mirror WriteTree, scoped to the throwaway index).
		if lsOut, _, _, lsErr := g.runWithEnv(ctx, g.workDir, env, "ls-files", "-u"); lsErr == nil && strings.TrimSpace(lsOut) != "" {
			return "", errors.New("unresolved merge conflicts in the scoped index — resolve them first")
		}
		return "", fmt.Errorf("git write-tree (scoped) failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// internal/git/git.go — *gitRunner impl of ReadTreeInto, placed immediately after ReadTree (~1222):
// ReadTreeInto is the scoped sibling of ReadTree: identical except it writes the throwaway index at
// indexFile (via GIT_INDEX_FILE) instead of .git/index. The live index is NEVER touched (FR-V3 scoped
// pre-commit primes T_start into a throwaway index). indexFile is made absolute (external_deps.md §8 #1).
func (g *gitRunner) ReadTreeInto(ctx context.Context, tree, indexFile string) error {
	absIndex, err := filepath.Abs(indexFile)
	if err != nil {
		return fmt.Errorf("git read-tree (scoped): resolve index path: %w", err)
	}
	_, stderr, code, err := g.runWithEnv(ctx, g.workDir,
		[]string{"GIT_INDEX_FILE=" + absIndex}, "read-tree", tree) // stdout unused (read-tree prints nothing)
	if err != nil {
		return err // git binary missing / context cancelled / start failure — UNWRAPPED
	}
	if code != 0 {
		// ALL non-zero exits are errors (mutation convention — like ReadTree). NO 128 special-case.
		return fmt.Errorf("git read-tree (scoped): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — add WriteTreeFrom to the Git interface (after WriteTree, ~95)
  - ADD the interface method + its doc comment (see Data Models). Co-located with WriteTree.
  - DOC: scoped sibling of WriteTree; reads indexFile not .git/index; FR-V3; filepath.Abs (§8 #1); mutation convention.
  - GOTCHA: it goes ON the interface (the public scoped surface), unlike runWithEnv.

Task 2: git.go — add WriteTreeFrom *gitRunner impl (after WriteTree impl, ~492)
  - ADD the impl per the Data Models block. Leading filepath.Abs(indexFile); runWithEnv(... "write-tree");
    on code!=0 the scoped ls-files -u probe; success → strings.TrimSpace(stdout).
  - GOTCHA: the probe MUST be scoped (runWithEnv + env, NOT g.run) so it inspects the THROWAWAY index.
  - GOTCHA: ZERO new imports (filepath + os already present).

Task 3: git.go — add ReadTreeInto to the Git interface + impl (after ReadTree, ~169 / ~1222)
  - ADD the interface method (after ReadTree ~169) + the impl (after ReadTree ~1222), per Data Models.
  - IMPL: filepath.Abs → runWithEnv(... "read-tree", tree); code!=0 → fmt.Errorf("git read-tree (scoped):
    failed (exit %d): %s", code, strings.TrimSpace(stderr)); success → nil.
  - DOC: scoped sibling of ReadTree; writes indexFile not .git/index; FR-V3; §8 #1; mutation convention.

Task 4: readtreeinto_test.go (NEW) — happy + live-index-untouched + error paths
  - FILE: internal/git/readtreeinto_test.go, `package git`. Mirror readtree_test.go's import block + style.
  - HELPERS (same package, in committree_test.go): writeFile, stageFile, makeEmptyCommit, writeTreeOf.
  - TEST TestReadTreeInto_LoadsTreeIntoScopedIndex:
      repo := t.TempDir(); initRepo(t, repo); writeFile(a.txt); stageFile; makeEmptyCommit; tree := writeTreeOf(repo).
      tmpIndex := filepath.Join(t.TempDir(), "scoped.index")   // a SEPARATE temp dir/file (NOT under repo/.git)
      g := New(repo)
      err := g.ReadTreeInto(ctx, tree, tmpIndex)
      ASSERT err==nil.
      ORACLE (throwaway index holds the tree): cmd := exec.Command("git", "-C", repo, "ls-files");
        cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex); out := cmd.Output(); assert TrimSpace(out)=="a.txt".
  - TEST TestReadTreeInto_LiveIndexUntouched (THE KEYSTONE):
      set up repo + live index holding {a.txt} (stage a.txt); snapshot the live index (e.g. read .git/index bytes
        OR `git ls-files` output). Call g.ReadTreeInto(ctx, tree, tmpIndex) (tmpIndex points elsewhere).
      ASSERT the live index is BYTE-IDENTICAL: `git ls-files` (NO GIT_INDEX_FILE) returns the SAME set as before
        (a.txt, unchanged) AND/OR hash .git/index before/after matches. THIS proves the scoped variant doesn't
        touch .git/index.
  - TEST TestReadTreeInto_BadTree: ReadTreeInto(ctx, "0000...0", tmp) → err contains "git read-tree (scoped): failed".
  - TEST TestReadTreeInto_GitBinaryMissing: t.Setenv("PATH",""); ReadTreeInto → err contains "git binary not found".
  - TEST TestReadTreeInto_ContextCancelled: pre-cancel ctx; ReadTreeInto → errors.Is(err, context.Canceled).
  - GOTCHA: use New(repo) (the methods are on the interface). The throwaway index file need NOT pre-exist
    (read-tree creates it). Inspect it via the INDEPENDENT oracle (GIT_INDEX_FILE env on a plain exec.Command).

Task 5: writetreefrom_test.go (NEW) — happy + live-index-untouched + error paths
  - FILE: internal/git/writetreefrom_test.go, `package git`. Mirror writetree_test.go.
  - TEST TestWriteTreeFrom_PrimedIndex:
      repo + tree (writeFile/stageFile/makeEmptyCommit/writeTreeOf). tmpIndex in a separate temp dir.
      g := New(repo); g.ReadTreeInto(ctx, tree, tmpIndex)   // prime the throwaway index
      sha, err := g.WriteTreeFrom(ctx, tmpIndex)
      ASSERT err==nil AND sha matches a 40/64-hex regex AND sha == writeTreeOf(repo) (the primed tree).
  - TEST TestWriteTreeFrom_LiveIndexUntouched (THE KEYSTONE):
      repo with a live index holding {a.txt}; snapshot .git/index. Prime a tmpIndex via ReadTreeInto(tree, tmp);
      call WriteTreeFrom(tmp). ASSERT the live .git/index (git ls-files WITHOUT GIT_INDEX_FILE) is byte-identical
      to before (the scoped write-tree read tmp, not .git/index).
  - TEST TestWriteTreeFrom_EmptyIndex: prime tmpIndex from EmptyTreeSHA (4b825dc642cb6eb9a060e54bf8d69288fbee4904)
      via ReadTreeInto; WriteTreeFrom(tmp) → sha == EmptyTreeSHA. (Confirms the scoped empty-tree path.)
  - TEST TestWriteTreeFrom_GitBinaryMissing + _ContextCancelled (mirror WriteTree's error-path tests).
  - GOTCHA: to prime the throwaway index use g.ReadTreeInto (exercising BOTH new primitives in one test). The
    SHA assertion cross-checks against writeTreeOf(repo) (the unscoped oracle for the same tree).

Task 6: VERIFY (no further file change)
  - RUN `gofmt -w`; `go vet ./...`; `go build ./...`; `go test -race ./internal/git/ -v`; `go test -race ./...`; `make lint`.
  - go.mod/go.sum byte-unchanged. ZERO new imports. ReadTree/WriteTree/run/runWithInput/runWithEnv byte-unchanged.
    Only git.go + readtreeinto_test.go + writetreefrom_test.go touched.
```

### Implementation Patterns & Key Details

```go
// The one-difference discipline (ReadTree/WriteTree are the templates):
//   ReadTree:     g.run(ctx, g.workDir, "read-tree", tree)                    // writes .git/index
//   ReadTreeInto: g.runWithEnv(ctx, g.workDir, []string{"GIT_INDEX_FILE="+abs}, "read-tree", tree)  // writes indexFile
//   WriteTree:    g.run(ctx, g.workDir, "write-tree")                        // reads .git/index
//   WriteTreeFrom:g.runWithEnv(ctx, g.workDir, env, "write-tree")            // reads indexFile
// Everything else (LookPath/-C/[]string/separate buffers/exit-code semantics) is IDENTICAL (inherited from runWithEnv).

// The absolute-path guard (external_deps.md §8 #1):
absIndex, err := filepath.Abs(indexFile)
if err != nil {
	return ..., fmt.Errorf("git <cmd> (scoped): resolve index path: %w", err)
}
env := []string{"GIT_INDEX_FILE=" + absIndex}   // absolute ⇒ CWD-independent (the hook's CWD no longer matters)

// The independent-oracle test assertion (inspect the THROWAWAY index without the runner):
cmd := exec.Command("git", "-C", repo, "ls-files")
cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
out, _ := cmd.Output()   // lists the throwaway index's paths; the live .git/index is untouched by this oracle too
```

```go
// readtreeinto_test.go — the keystone (live index untouched):
func TestReadTreeInto_LiveIndexUntouched(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo)
	tmpIndex := filepath.Join(t.TempDir(), "scoped.index") // NOT under repo/.git

	// Snapshot the LIVE index BEFORE (independent oracle — NO GIT_INDEX_FILE).
	before := execGit(t, repo, "ls-files")

	g := New(repo)
	if err := g.ReadTreeInto(context.Background(), tree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}
	// KEYSTONE: the LIVE index is byte-identical (a.txt still staged; the scoped read-tree wrote tmpIndex, not .git/index).
	if after := execGit(t, repo, "ls-files"); after != before {
		t.Errorf("live .git/index changed: before=%q after=%q (scoped variant must NOT touch .git/index)", before, after)
	}
	// And the THROWAWAY index holds the tree (independent oracle WITH GIT_INDEX_FILE).
	cmd := exec.Command("git", "-C", repo, "ls-files")
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scoped ls-files: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "a.txt" {
		t.Errorf("throwaway index = %q, want \"a.txt\"", got)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; "os"+"path/filepath" already imported (S1). go mod tidy no-op.

PACKAGE EDGES:
  - internal/git → (stdlib only). runWithEnv (S1) is the seam; NO new import. NO new module dep.

FROZEN / NOT-EDITED:
  - run() (389), runWithInput() (430), runWithEnv() (S1, ~460) — the exec seams are frozen; S2 CALLS runWithEnv.
  - ReadTree (1222) + WriteTree (492) — the unscoped templates; UNCHANGED (the scoped variants are NEW siblings).
  - DiffTreeNames (1582) — read-only/index-agnostic; the subset-check helper P1.M2.T2.S1 consumes. NOT touched here.
  - Any hook-runner/config/cli file (internal/hooks is M3; internal/cmd + config are M1). NO scope creep.

DOWNSTREAM CONSUMERS (do NOT implement here):
  - P1.M2.T2.S1 (next): the subset check — DiffTreeNames(snapshotTree, newTree) ⊆ DiffTreeNames(baseTree, tStart).
    Consumes DiffTreeNames (read-only, unchanged) + the WriteTreeFrom-captured newTree. NOT these primitives' logic.
  - M3 (internal/hooks runner): calls ReadTreeInto (prime T_start into tmp) → runs the hook with GIT_INDEX_FILE
    in its env → WriteTreeFrom (capture) → subset check → commit-tree against the captured tree. These two
    methods ARE M3's scoped-index surface.

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO HOOK RUNNER / NO DOCS (internal primitives).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/git/git.go internal/git/readtreeinto_test.go internal/git/writetreefrom_test.go
test -z "$(gofmt -l internal/git/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...          # Expect zero diagnostics (catches a wrong signature / unused var / missing interface method).
go build ./...        # Whole module compiles (incl. any caller of the Git interface — they now see 2 new methods).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm ZERO new imports (S1 landed "os"; "path/filepath" was already present):
git diff internal/git/git.go | grep -E '^\+\s*"os"|"path/filepath"' && echo "UNEXPECTED new import (re-check)" || echo "no new imports (good)"
# Confirm both methods landed on the interface + the filepath.Abs guard is present:
grep -n 'ReadTreeInto\|WriteTreeFrom' internal/git/git.go   # interface decls + impls (≥4 hits)
grep -n 'filepath.Abs(indexFile)' internal/git/git.go       # exactly 2 (one per variant)
# Expected: clean + build succeeds. If `go build` errors on an undefined symbol, you used g.run instead of
#   g.runWithEnv, or missed adding the method to the interface (a caller of Git would then fail to compile).
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/git/ -v -run 'TestReadTreeInto|TestWriteTreeFrom'
# Expected: all PASS. Key assertions:
#   TestReadTreeInto_LoadsTreeIntoScopedIndex  → throwaway index holds the tree (independent oracle).
#   TestReadTreeInto_LiveIndexUntouched        → live .git/index byte-identical before/after (KEYSTONE).
#   TestReadTreeInto_BadTree/_GitBinaryMissing/_ContextCancelled → error paths.
#   TestWriteTreeFrom_PrimedIndex              → SHA matches writeTreeOf(repo) (the primed tree).
#   TestWriteTreeFrom_LiveIndexUntouched       → live .git/index byte-identical (KEYSTONE).
#   TestWriteTreeFrom_EmptyIndex               → EmptyTreeSHA (scoped empty-tree path).
go test -race ./internal/git/   # the full git suite — no regression (ReadTree/WriteTree/run/runWithInput/runWithEnv unchanged).
go test -race ./...             # full module — no regression (the 2 new interface methods are additive).
# Expected: green throughout. If a keystone test FAILS (live index changed), the variant is calling g.run not
#   g.runWithEnv, OR GIT_INDEX_FILE isn't being set — re-check the env wiring.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only git.go + the 2 new test files changed:
git diff --name-only | grep -Ev '^internal/git/git\.go$|^internal/git/readtreeinto_test\.go$|^internal/git/writetreefrom_test\.go$' \
  && echo "UNEXPECTED file changed" || echo "only git.go + 2 new test files changed (good)"
# Confirm the exec seams + unscoped templates are byte-unchanged (frozen):
git diff --exit-code -- internal/git/git.go >/dev/null 2>&1 || { git diff internal/git/git.go | grep -E '^[-+].*\bg\.run\(|^[-+].*func \(g \*gitRunner\) (ReadTree|WriteTree|run|runWithInput|runWithEnv)\b' && echo "FROZEN method edited — re-check (should only ADD)" || echo "frozen methods unchanged (only additions)"; }
# Confirm runWithEnv (S1) is now called by both scoped variants (S1's test was its only prior caller):
grep -n 'g.runWithEnv' internal/git/git.go   # S1's test + the 2 scoped variant impls (≥2 production calls now)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — `unused`/`staticcheck` clean; the scoped variants have callers (the tests):
make lint 2>&1 | grep -iE 'ReadTreeInto|WriteTreeFrom|unused|U1000' && echo "BAD: a new method flagged" || echo "no lint finding for the scoped variants (good)"
# Absolute-path audit: confirm BOTH variants enforce filepath.Abs (the §8 #1 gotcha) — NOT a raw indexFile in GIT_INDEX_FILE:
grep -n 'GIT_INDEX_FILE=' internal/git/git.go   # exactly 2 matches, both concatenate "+absIndex" (NOT "+indexFile")
# Manual end-to-end sanity (optional, in a test): prime a throwaway index from a real tree via ReadTreeInto,
# stage a mutation into it via an independent `GIT_INDEX_FILE=<tmp> git add`, capture via WriteTreeFrom, and
# DiffTreeNames(tree, captured) — should show ONLY the mutated path. (This is the M3 sequence; the in-package
# tests cover the primitive halves. The full sequence is M3's integration test, NOT this task's.)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/git/`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op;
      `git diff --exit-code go.mod go.sum` empty; ZERO new imports in git.go.
- [ ] Level 2 green: `TestReadTreeInto_*` + `TestWriteTreeFrom_*` pass; `go test -race ./internal/git/` +
      `go test -race ./...` green.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only git.go + 2 new test files changed; frozen methods
      (run/runWithInput/runWithEnv/ReadTree/WriteTree) byte-unchanged.
- [ ] Level 4: `make lint` green — no `unused`/U1000/staticcheck finding for the scoped variants; both
      variants use `filepath.Abs` (the §8 #1 absolute-path guard).

### Feature Validation

- [ ] `ReadTreeInto(ctx, tree, indexFile)` is on the Git interface + impl; primes a throwaway index from
      `<tree>` via `GIT_INDEX_FILE=<abs>` scoped `read-tree`; live `.git/index` UNTOUCHED.
- [ ] `WriteTreeFrom(ctx, indexFile)` is on the Git interface + impl; captures the tree SHA of the throwaway
      index via `GIT_INDEX_FILE=<abs>` scoped `write-tree` (incl. the scoped `ls-files -u` probe); live index UNTOUCHED.
- [ ] Both enforce an ABSOLUTE `GIT_INDEX_FILE` (`filepath.Abs`).
- [ ] The KEYSTONE tests (live-index-untouched) pass for both variants.
- [ ] ALL non-zero exits are errors (the mutation convention); error strings carry the "(scoped)" tag.

### Code Quality Validation

- [ ] Mirrors ReadTree/WriteTree exactly (one-difference: `g.run` → `g.runWithEnv` + `GIT_INDEX_FILE`).
- [ ] Co-located with siblings (interface + impl); dedicated per-primitive test files (the established discipline).
- [ ] No scope creep into run/runWithInput/runWithEnv (frozen), DiffTreeNames (P1.M2.T2.S1), the hook runner (M3),
      config/cli (M1), or docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Multi-line doc comments on both interface methods (name FR-V3, the scoped-index mechanism, §8 #1, the
      mutation convention) — Mode A (internal primitives; no docs/*.md).
- [ ] go.mod/go.sum byte-unchanged; only 2 new test files + git.go edits.

---

## Anti-Patterns to Avoid

- ❌ Don't call `g.run(...)` in the scoped variants. They MUST use `g.runWithEnv(ctx, g.workDir,
  []string{"GIT_INDEX_FILE=" + absIndex}, ...)`. `g.run` writes/reads `.git/index` — the exact thing the
  scoped variants exist to AVOID. If a keystone test fails (live index changed), you used g.run.
- ❌ Don't pass a relative `GIT_INDEX_FILE`. external_deps.md §8 #1: a relative value resolves against the
  hook's CWD (the worktree root), not stagecoach's CWD. Open each variant with `filepath.Abs(indexFile)`.
  Do NOT concatenate the raw `indexFile` into `GIT_INDEX_FILE=`.
- ❌ Don't add imports or change go.mod. git.go ALREADY has `"os"` (S1) + `"path/filepath"`. An unused import
  fails `go vet`; a new dep is wrong. If build fails on undefined `filepath`/`os`, S1 didn't land — re-check
  (it should have).
- ❌ Don't keep the scoped variants OFF the Git interface. Unlike runWithEnv (off-interface, S1), ReadTreeInto/
  WriteTreeFrom ARE the public surface M3 calls. Add them to the interface (co-located with ReadTree/WriteTree).
- ❌ Don't drop the `ls-files -u` merge-conflict probe in WriteTreeFrom. Mirror WriteTree — BUT scoped
  (`runWithEnv` + env, NOT `g.run`) so it inspects the THROWAWAY index. Dropping it loses the clean
  "unresolved merge conflicts" diagnostic on the scoped path.
- ❌ Don't touch run()/runWithInput()/runWithEnv(), ReadTree, WriteTree, or DiffTreeNames. run*/runWithEnv are
  the frozen exec seams; ReadTree/WriteTree are the unchanged templates; DiffTreeNames is P1.M2.T2.S1's
  read-only reuse target. The scoped variants are NEW siblings.
- ❌ Don't forget the KEYSTONE test (live `.git/index` byte-identical before/after). It is the ONLY assertion
  that proves the scoped variant doesn't touch the live index — the whole point of FR-V3. Snapshot `git ls-files`
  (or hash `.git/index`) before; assert unchanged after. Inspect the THROWAWAY index via an INDEPENDENT oracle
  (`exec.Command` with `GIT_INDEX_FILE=<tmp>`), not via the runner.
- ❌ Don't inspect the throwaway index via the runner under test. Use an independent `exec.Command("git", ...)
  with `cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmp)` — otherwise you're testing the primitive with
  itself. Mirror the readtree_test.go/writetree_test.go independent-oracle style.
- ❌ Don't use `&gitRunner{}` in the tests when `New(repo)` works. ReadTreeInto/WriteTreeFrom are ON the
  interface, so `New(repo)` (the public constructor) returns a value that can call them. (`&gitRunner{}` is
  only needed for unexported methods like runWithEnv.)
- ❌ Don't place the scoped variants away from their siblings. Co-locate: WriteTreeFrom after WriteTree
  (interface ~95 + impl ~492); ReadTreeInto after ReadTree (interface ~169 + impl ~1222).
- ❌ Don't change go.mod/go.sum or add files beyond the 2 test files. Two interface methods + two impls + tests.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./internal/git/`/`make lint` — they catch a missing interface
  method (a Git-interface caller won't compile), a relative-GIT_INDEX_FILE oversight (the keystone test), an
  unused import, and the `unused`-lint gate. The keystone tests pin the FR-V3 freeze invariant.
