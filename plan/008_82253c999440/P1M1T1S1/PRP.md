---
name: "P1.M1.T1.S1 — Add OverlayTreePaths to the Git interface + gitRunner impl + comprehensive unit tests"
description: |
  New leaf git primitive (PRD §13.6.5 / FR-M10 mid-chain rebuild). `OverlayTreePaths(ctx, baseTree,
  sourceTree, paths)` returns a new tree equal to baseTree with each path in `paths` overwritten by its
  state in sourceTree (present → `update-index --cacheinfo <mode>,<blob>,<path>`; absent → deletion-overlay
  via `update-index --force-remove <path>`). Impl: read-tree baseTree → ONE `ls-tree -r --full-tree
  sourceTree -- paths` (parsed by a new parseLsTree helper) → per-path update-index → write-tree. Empty
  paths ⇒ baseTree verbatim. Index/object-store-only mutation; NEVER touches the working tree or a ref
  (mirrors FreezeWorkingTree). Add the interface doc-block after DiffTreeNames; the gitRunner impl +
  parseLsTree near FreezeWorkingTree; a NEW internal/git/overlaytree_test.go with the named cases
  (overlay-only, deletion-overlay, empty-paths-noop, mid-chain-simulation, the standard 4-case negative
  set, DoesNotTouchWorkingTree). Consumed by P1.M1.T2.S1. No docs in S1.
---

## Goal

**Feature Goal**: Land the `OverlayTreePaths` git primitive — the freeze-safe tree-folding operation the
FR-M1d arbiter mid-chain resolution needs to rebuild a linear commit chain from frozen trees without ever
reading the live working tree. It produces `tree′[j] = OverlayTreePaths(tree[j], T_start, leftoverPaths)` —
a new tree equal to `tree[j]` with the leftover paths overlaid from `T_start` — using only `.git/index` and
the object store.

**Deliverable** (one interface decl + one impl + one helper + one new test file):
1. `internal/git/git.go` Git interface: add `OverlayTreePaths(ctx, baseTree, sourceTree, paths) (treeSHA, err)`
   with its doc-block, immediately after `DiffTreeNames` (~line 289).
2. `internal/git/git.go` gitRunner: add `(g *gitRunner) OverlayTreePaths(...)` + the unexported
   `parseLsTree(out string) map[string]lsTreeEntry` helper, near `FreezeWorkingTree`/`DiffTreeNames`
   (~line 1572+).
3. `internal/git/overlaytree_test.go` (NEW): comprehensive unit tests — overlay-only-listed-paths,
   deletion-overlay, empty-paths-noop, mid-chain-leftover-simulation, the standard 4-case negative set
   (BadTree/NotARepo/GitBinaryMissing/ContextCancelled), and DoesNotTouchWorkingTree.

**Success Definition**: `OverlayTreePaths` returns a new tree SHA whose content is `baseTree` with each
`paths` entry overwritten from `sourceTree` (present) or removed (absent); empty `paths` returns `baseTree`
verbatim with no index mutation; the working tree on disk is never touched; no ref moves; bad tree / non-repo
/ missing-git / cancelled-context all surface non-nil errors. `go build/vet/gofmt` clean; `go test -race ./...`
green. Consumed unchanged by the arbiter rewrite (P1.M1.T2.S1).

## User Persona

**Target User**: The contributor implementing the FR-M1d arbiter mid-chain resolution (P1.M1.T2.S1) —
`resolveMidChain` calls `OverlayTreePaths(tree[j], tStart, leftoverPaths)` per rebuilt commit. This is a
foundation primitive; no end-user-visible surface.

**Use Case**: After a decompose run, the arbiter decides leftovers belong to an *earlier* commit `i`.
Stagecoach rebuilds the chain `i..N-1`: for each `j`, `tree′[j] = OverlayTreePaths(tree[j], T_start,
leftoverPaths)`, then `commit-tree tree′[j] -p rebuiltParent` reusing `msg[j]`. The rebuilt tip == `T_start`.

**Pain Points Addressed**: Provides the one tree-folding operation that makes mid-chain arbiter resolution
freeze-safe (no live `git add`, no `git status` read) — closing the v2.0–v2.1 loophole where a concurrent
working-tree change during the planner call could be swept into an arbiter commit (FR-M1d).

## Why

- **Closes the arbiter freeze loophole (FR-M1d).** v2.0–v2.1's mid-chain resolution ran `git add` against
  the live tree; a concurrent change could land in an arbiter commit. `OverlayTreePaths` builds the rebuilt
  trees entirely from frozen SHAs (`tree[j]`, `T_start`) — the live working tree is never consulted.
- **Single-purpose leaf primitive, proven discipline.** It orchestrates `read-tree` + `update-index` +
  `write-tree` — index/object-store-only mutation, no ref, no working-tree touch — exactly the discipline
  `FreezeWorkingTree`/`ReadTree`/`WriteTree` already follow (arch §1.4: "`OverlayTreePaths` mirrors exactly
  this discipline").
- **One round-trip for the source blobs.** A single `ls-tree -r --full-tree sourceTree -- paths...` reads
  every (mode, blob) pair, then per-path `update-index` applies them. No per-path git calls to read source.
- **Foundation for the critical path.** P1.M1.T2.S1 (the arbiter rewrite) is blocked until this primitive
  exists with a stable signature. S1 unblocks it.

## What

A new git primitive (`OverlayTreePaths`) + its parse helper (`parseLsTree`) + a comprehensive test file.
No caller changes (the arbiter rewrite is T2), no other package, no docs. The primitive is pure git
plumbing orchestration (read-tree → ls-tree → update-index × N → write-tree).

### Success Criteria

- [ ] `Git` interface declares `OverlayTreePaths(ctx, baseTree, sourceTree string, paths []string) (treeSHA string, err error)` (after `DiffTreeNames`).
- [ ] `gitRunner.OverlayTreePaths` implements: empty `paths` → return `baseTree` verbatim; else `read-tree baseTree` → one `ls-tree -r --full-tree sourceTree -- paths` → per-path `update-index --cacheinfo` (present) / `--force-remove` (absent) → `write-tree`.
- [ ] `parseLsTree` parses `<mode> <type> <blob>\t<path>` into `map[path]lsTreeEntry{mode, blob}`.
- [ ] Mutation exit-code convention: `code != 0 ⇒ wrapped error`; `err != nil` (binary-missing/cancel) propagated UNWRAPPED — mirrors `ReadTree`/`DiffTreeNames`. NO 128-as-non-error special case.
- [ ] The primitive mutates ONLY `.git/index` + the object store; NEVER touches the working tree; NEVER moves a ref.
- [ ] `overlaytree_test.go` passes: overlay-only, deletion-overlay, empty-paths-noop, mid-chain-simulation, the 4-case negative set, DoesNotTouchWorkingTree.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim `OverlayTreePaths` + `parseLsTree` implementation from
arch §3.2-§3.3, the verbatim interface doc-block from §3.1, the exact placement (after `DiffTreeNames` /
near `FreezeWorkingTree`), the exit-code convention, the 6 named test cases (with the helpers to reuse and
the existing tests to mirror), and the gotchas. The #1 trap — the mutation exit-code convention (code != 0 ⇒
error, NO 128 special-case; err != nil UNWRAPPED) — is stated with the sibling references.

### Documentation & References

```yaml
# MUST READ — the authoritative OverlayTreePaths design
- docfile: plan/008_82253c999440/docs/architecture/arbiter_freeze_parity.md
  why: "§1.4 confirms OverlayTreePaths does NOT exist + EmptyTreeSHA present + the placement table; §3.1 the interface doc-block (verbatim); §3.2-§3.3 the verbatim gitRunner impl + parseLsTree pseudocode; §3.4 placement (after DiffTreeNames / near FreezeWorkingTree); the exit-code convention (mutation form, no 128 special-case); §3.4 the named test cases + the 4-case negative set + DoesNotTouchWorkingTree."
  critical: "§3.2-§3.3 is copy-paste-ready pseudocode (read-tree → ls-tree → per-path update-index → write-tree). §3.4: empty paths ⇒ baseTree verbatim (NO write-tree). §2.6-§2.7 explain WHY OverlayTreePaths (not ReadTree(tStart)) for mid-chain — a leftover path must take T_start's blob while non-leftover paths keep tree[j]'s blob."

- file: internal/git/git.go
  why: "EDIT (2 spots). (a) Interface: add the OverlayTreePaths doc-block + decl after DiffTreeNames (~line 289). (b) gitRunner: add (g *gitRunner) OverlayTreePaths + parseLsTree near FreezeWorkingTree/DiffTreeNames (~line 1572+). Reuse g.run(ctx, g.workDir, args...) and g.WriteTree(ctx). EmptyTreeSHA at :644."
  pattern: "Mirror DiffTreeNames (git.go:1572): `stdout, stderr, code, err := g.run(...)`; `if err != nil { return nil, err }` (UNWRAPPED); `if code != 0 { return nil, fmt.Errorf(\"git …: failed (exit %d): %s\", code, strings.TrimSpace(stderr)) }`. NO 128-as-non-error special case."
  gotcha: "The mutation exit-code convention: err != nil ⇒ propagate UNWRAPPED (git binary missing / context cancelled / start failure); code != 0 ⇒ wrap with stderr. This is the SAME convention as ReadTree/WriteTree/Add/DiffTreeNames — mirror it exactly. Do NOT introduce a 128 special-case (that's RevParseHEAD's unborn-signal, unrelated)."

- file: internal/git/readtree_test.go
  why: "READ-ONLY ref — the template for the standard 4-case negative set. TestReadTree_BadTree (:96), _NotARepo (:111), _GitBinaryMissing (:124, asserts err contains 'git binary not found'), _ContextCancelled (:139). Mirror these four EXACTLY for OverlayTreePaths."
- file: internal/git/freezeworkingtree_test.go
  why: "READ-ONLY ref — the DoesNotTouchWorkingTree pattern. TestFreezeWorkingTree_LeavesWorkingTreeUnchanged (:122): write a working-tree file, call the primitive, assert the file is UNCHANGED on disk. Mirror it."
- file: internal/git/git_test.go # initRepo:13, runGit:285
  why: "READ-ONLY — initRepo(t, dir) (git init + repo-local user.name/email) is the temp-repo helper every internal/git test reuses."
- file: internal/git/committree_test.go # writeFile:31, stageFile:39
  why: "READ-ONLY — writeFile(t,dir,name,body) + stageFile(t,dir,name) build the file content + stage it (for constructing base/source trees)."
- file: internal/git/revparsetree_test.go # execGit:115
  why: "READ-ONLY — execGit(t,dir,args...) is the ORACLE: runs git directly and returns trimmed stdout. Use it to build trees (`execGit(t,dir,\"write-tree\")`), verify results (`execGit(t,dir,\"ls-tree\",\"-r\",result)`), and assert content."

- docfile: plan/008_82253c999440/P1M1T1S1/research/s1_overlaytreepaths.md
  why: "Distilled S1 findings: the verbatim impl + parseLsTree, the interface doc-block, the placement, the exit-code convention, the 6 named test cases (with helper reuse), and the S1/T2/T3 scope boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── git.go                       # EDIT: + interface decl (after DiffTreeNames); + gitRunner.OverlayTreePaths + parseLsTree (near FreezeWorkingTree)
    └── overlaytree_test.go          # NEW: comprehensive unit tests
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── internal/git/
    ├── git.go                       # +OverlayTreePaths (interface + gitRunner) + parseLsTree
    └── overlaytree_test.go          # NEW (6 named cases + 4-case negative set + DoesNotTouchWorkingTree)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | + `OverlayTreePaths` interface decl (after `DiffTreeNames`); + `gitRunner.OverlayTreePaths` + `parseLsTree` (near `FreezeWorkingTree`). **Only production file.** |
| `internal/git/overlaytree_test.go` | CREATE | Comprehensive unit tests (overlay-only, deletion-overlay, empty-noop, mid-chain-sim, 4-case negative, DoesNotTouchWorkingTree). |

**Explicitly NOT touched**: `internal/decompose/*` (the arbiter rewrite = P1.M1.T2.S1), any other `internal/git/*.go`
(siblings), any other package, docs (P1.M1.T2.S2 / P3), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (exit-code convention): g.run returns (stdout, stderr, code, err). err != nil ⇒ git binary
// missing / context cancelled / start failure → propagate UNWRAPPED (return "", err). code != 0 ⇒ git
// non-zero exit → wrap with stderr (return "", fmt.Errorf("git …: failed (exit %d): %s", code, …)).
// This is the MUTATION form used by ReadTree/WriteTree/Add/DiffTreeNames. NO 128-as-non-error special-case
// (that's RevParseHEAD's unborn-repo signal — UNRELATED; do not reuse it here).

// CRITICAL (empty paths = verbatim no-op): if len(paths)==0 return baseTree, nil IMMEDIATELY — do NOT
// run read-tree or write-tree. The empty-paths test asserts the index is UNCHANGED (capture write-tree
// before, assert equal after). This is the early-return the arch §3.2 step 1 specifies.

// CRITICAL (never touch working tree / never move a ref): OverlayTreePaths runs read-tree + update-index
// + write-tree — ALL index/object-store-only. It MUST NOT run checkout / reset --hard / add (working-tree
// read) / update-ref / commit-tree. The DoesNotTouchWorkingTree test pins this. Mirrors FreezeWorkingTree.

// GOTCHA (--cacheinfo comma form): `update-index --cacheinfo <mode>,<blob>,<path>` is the single-arg
// comma form (git 2.0+). Build it via fmt.Sprintf("%s,%s,%s", ent.mode, ent.blob, p). A path containing
// a comma would break the form — pre-existing limitation (arch §6 risk 5); tests use simple paths.

// GOTCHA (parseLsTree format): ls-tree -r --full-tree emits "<mode> <type> <blob>\t<path>" per line.
// Split on the FIRST \t (path is everything after); split the left on spaces → [mode, type, blob]; ignore
// type (always "blob" for -r over file paths). Use strings.IndexByte(line, '\t') + strings.Fields(left).

// GOTCHA (ls-tree path quoting): ls-tree quotes paths with special chars (core.quotePath); spaces are NOT
// special (safe). Non-ASCII/quote paths are a pre-existing limitation — tests use simple ASCII paths.

// GOTCHA (ls-tree pathspec): the args are ["ls-tree","-r","--full-tree",sourceTree,"--",paths...]. The
// "--" separates the tree from the pathspecs (defensive even though sourceTree is a SHA). append a fresh
// slice (don't mutate a shared backing array).

// GOTCHA (test style): in-package (package git), t.TempDir(), REAL git binary, no testify. Reuse initRepo/
// writeFile/stageFile/execGit. Build distinct trees by staging different file sets + `execGit write-tree`.
// Mirror readtree_test.go for the 4-case negative set; mirror freezeworkingtree_test.go:122 for DoesNotTouch.
```

## Implementation Blueprint

### Data models and structure

One new interface method + one new struct + one helper. The relevant existing types/helpers (unchanged):

```go
// internal/git/git.go (EXISTING — reused)
func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout, stderr string, exitCode int, err error)
func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error)
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // git.go:644

// NEW (this subtask)
type lsTreeEntry struct{ mode, blob string }
func parseLsTree(out string) map[string]lsTreeEntry
func (g *gitRunner) OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (string, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the interface doc-block + decl (internal/git/git.go, after DiffTreeNames ~line 289)
  - LOCATE the `DiffTreeNames(ctx, ...) (paths []string, err error)` decl in the Git interface (the block
    ending ~line 289, before ConfigGlobalGet).
  - INSERT immediately after it the doc-block + decl from "Implementation Patterns" (verbatim from arch §3.1).
  - NAMING: OverlayTreePaths; params (ctx, baseTree, sourceTree string, paths []string); return (treeSHA string, err error).
  - DO NOT: add to gitRunner here (that's Task 2); change DiffTreeNames or any other interface method.

Task 2: ADD gitRunner.OverlayTreePaths + parseLsTree (internal/git/git.go, near FreezeWorkingTree ~line 1572)
  - LOCATE the gitRunner.DiffTreeNames impl (git.go ~1572) — place the new code right after it.
  - ADD the lsTreeEntry struct + parseLsTree helper + (g *gitRunner) OverlayTreePaths (verbatim from arch
    §3.2-§3.3 / "Implementation Patterns"). Reuse g.run(ctx, g.workDir, args...) and g.WriteTree(ctx).
  - STEPS inside OverlayTreePaths: (1) len(paths)==0 → return baseTree, nil; (2) read-tree baseTree;
    (3) ONE ls-tree -r --full-tree sourceTree -- paths... → parseLsTree; (4) per path: present in map →
    update-index --cacheinfo <mode>,<blob>,path; absent → update-index --force-remove path; (5) return g.WriteTree(ctx).
  - EXIT-CODE: every g.run call → if err != nil return "", err (UNWRAPPED); if code != 0 return "", wrap.
  - DO NOT: touch the working tree (no checkout/reset --hard); move a ref (no update-ref/commit-tree);
    introduce a 128 special-case; mutate a shared args backing array.

Task 3: CREATE internal/git/overlaytree_test.go (package git; real git; temp repos)
  - PACKAGE: `package git`. IMPORTS: context, os, os/exec, path/filepath, strings, testing. (Mirror
    readtree_test.go / freezeworkingtree_test.go.)
  - REUSE: initRepo(t, dir), writeFile(t,dir,name,body), stageFile(t,dir,name), execGit(t,dir,args...).
  - BUILD-TREE HELPER (local): a small `writeTree(t, dir) string` that returns execGit(t, dir, "write-tree")
    (capture a tree from the current index). Use it to build baseTree/sourceTree/tree1/tStart.
  - TestOverlayTreePaths_OverlayOnlyListedPaths:
      initRepo + commit a base (a.go="A", b.go="B") → baseTree = writeTree.
      stage a.go="A'" + c.go="C" (b.go unchanged) → sourceTree = writeTree.
      reset index to baseTree (execGit read-tree baseTree) so the runner starts clean.
      result, err := g.OverlayTreePaths(ctx, baseTree, sourceTree, []string{"a.go","c.go"})
      ASSERT err==nil; oracle: execGit ls-tree -r result → a.go="A'", b.go="B", c.go="C".
  - TestOverlayTreePaths_DeletionOverlay:
      base={a.go,b.go}; source={a.go} (b.go absent); paths=[b.go].
      result, err := OverlayTreePaths(baseTree, sourceTree, [b.go]).
      ASSERT ls-tree result → a.go present (base value), b.go ABSENT.
  - TestOverlayTreePaths_EmptyPathsNoop:
      build baseTree; capture indexTree = execGit write-tree (before).
      result, err := OverlayTreePaths(baseTree, sourceTree, nil) AND paths=[] (two sub-asserts or two tests).
      ASSERT result == baseTree; AND execGit write-tree (after) == indexTree (index UNCHANGED — no read-tree ran).
  - TestOverlayTreePaths_MidChainLeftoverSimulation:
      build tree1 (a subset, e.g. only a.go staged) and tStart (full: a.go+b.go+c.go).
      paths := execGit diff-tree -r --name-only --no-commit-id tree1 tStart  (the leftover set).
      result, err := OverlayTreePaths(tree1, tStart, paths).
      ASSERT DiffTreeNames(result, tStart) is empty (leftovers folded in ⇒ result == tStart content).
  - TestOverlayTreePaths_BadTree / _NotARepo / _GitBinaryMissing / _ContextCancelled:
      mirror readtree_test.go:96/111/124/139 EXACTLY (swap ReadTree for OverlayTreePaths with a non-empty
      paths arg so it reaches read-tree; BadTree passes a bogus SHA as baseTree).
  - TestOverlayTreePaths_DoesNotTouchWorkingTree:
      mirror freezeworkingtree_test.go:122: write a working-tree file (e.g. untouched.txt="keep"), build
      base/source trees from OTHER files, call OverlayTreePaths, assert untouched.txt is UNCHANGED on disk
      (read it back; also assert execGit status --porcelain still lists it as untracked/modified as before).
  - DO NOT: use testify; touch the working tree in the overlay-only/deletion tests; add integration tests
    for the arbiter (that's P1.M1.T2/T3).

Task 4: VALIDATE
  - RUN: gofmt -w internal/git/git.go internal/git/overlaytree_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/git/ -v -run TestOverlayTreePaths   # all new tests green
  - RUN: go test -race ./...                                          # full suite green
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === interface doc-block + decl (after DiffTreeNames) ===
	// OverlayTreePaths returns a NEW tree equal to baseTree with each path in paths overwritten by its
	// state in sourceTree (PRD §13.6.5 / FR-M10 mid-chain rebuild). For each path in paths:
	//   - present in sourceTree → overwritten with sourceTree's (mode, blob)
	//     (git update-index --cacheinfo <mode>,<blob>,<path>).
	//   - absent in sourceTree  → removed from the result (deletion-overlay)
	//     (git update-index --force-remove <path>).
	// The (mode, blob) pairs come from ONE `git ls-tree -r --full-tree <sourceTree> -- <paths...>`.
	// Implementation: read-tree baseTree (index = baseTree) → per-path update-index → write-tree.
	// EMPTY paths ⇒ return baseTree verbatim (no-op early return, NO index mutation).
	// It mutates ONLY .git/index and the object store (same discipline as FreezeWorkingTree/ReadTree/
	// WriteTree); it NEVER touches the working tree and NEVER moves a ref. Bad/unresolvable tree SHA
	// ⇒ a wrapped error (code != 0; NO 128-as-non-error special case — mirror ReadTree/DiffTreeNames).
	OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (treeSHA string, err error)
```

```go
// === gitRunner impl + parseLsTree (near FreezeWorkingTree/DiffTreeNames) ===
type lsTreeEntry struct{ mode, blob string }

// parseLsTree parses `git ls-tree -r --full-tree` output ("<mode> <type> <blob>\t<path>" per line) into
// map[path]→{mode, blob}. The <type> column (always "blob" for -r over file paths) is ignored. Pure.
func parseLsTree(out string) map[string]lsTreeEntry {
	m := make(map[string]lsTreeEntry)
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		path := line[tab+1:]
		left := strings.Fields(line[:tab]) // [mode, type, blob]
		if len(left) < 3 {
			continue
		}
		m[path] = lsTreeEntry{mode: left[0], blob: left[2]} // left[1] = type ("blob"), ignored
	}
	return m
}

func (g *gitRunner) OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (string, error) {
	if len(paths) == 0 {
		return baseTree, nil // early no-op — avoids a pointless write-tree + read-tree
	}
	// 1. read-tree baseTree → index = baseTree.
	if _, stderr, code, err := g.run(ctx, g.workDir, "read-tree", baseTree); err != nil {
		return "", err
	} else if code != 0 {
		return "", fmt.Errorf("git read-tree (overlay base): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	// 2. ONE ls-tree sourceTree for the requested paths.
	lsArgs := append([]string{"ls-tree", "-r", "--full-tree", sourceTree, "--"}, paths...)
	lsOut, stderr, code, err := g.run(ctx, g.workDir, lsArgs...)
	if err != nil {
		return "", err
	} else if code != 0 {
		return "", fmt.Errorf("git ls-tree (overlay source): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	blobs := parseLsTree(lsOut)
	// 3. per-path update-index (cacheinfo if present in source; force-remove if absent = deletion-overlay).
	for _, p := range paths {
		if ent, ok := blobs[p]; ok {
			if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--cacheinfo",
				fmt.Sprintf("%s,%s,%s", ent.mode, ent.blob, p)); err != nil {
				return "", err
			} else if code != 0 {
				return "", fmt.Errorf("git update-index --cacheinfo %s: failed (exit %d): %s", p, code, strings.TrimSpace(stderr))
			}
		} else {
			if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--force-remove", p); err != nil {
				return "", err
			} else if code != 0 {
				return "", fmt.Errorf("git update-index --force-remove %s: failed (exit %d): %s", p, code, strings.TrimSpace(stderr))
			}
		}
	}
	// 4. write-tree → new tree SHA.
	return g.WriteTree(ctx)
}
```

### Integration Points

```yaml
GIT INTERFACE (internal/git/git.go):
  - added: OverlayTreePaths(ctx, baseTree, sourceTree string, paths []string) (treeSHA string, err error)
    (after DiffTreeNames)

GITRUNNER (internal/git/git.go):
  - added: (g *gitRunner) OverlayTreePaths — read-tree → ls-tree → per-path update-index → write-tree
  - added: type lsTreeEntry + func parseLsTree (unexported helper)
  - reuse: g.run(ctx, g.workDir, args...) + g.WriteTree(ctx); const EmptyTreeSHA

TESTS (internal/git/overlaytree_test.go NEW):
  - 6 named cases + 4-case negative set + DoesNotTouchWorkingTree
  - reuse: initRepo, writeFile, stageFile, execGit (oracle); real git binary; no testify

NO-TOUCH (explicitly):
  - internal/decompose/*          # the arbiter rewrite (resolveArbiter → 3 paths using OverlayTreePaths) = P1.M1.T2.S1
  - any other internal/git/*.go    # siblings (ReadTree/WriteTree/FreezeWorkingTree/DiffTreeNames unchanged)
  - any other package; docs (P1.M1.T2.S2 / P3); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M1.T2.S1: resolveArbiter — Path C (mid-chain) calls OverlayTreePaths(tree[j], tStart, leftoverPaths)
    per rebuilt commit; Path A/B commit T_start directly. Plus the ReadTree(T_start) index-sync + chain_test updates.
  - P1.M1.T2.S2: Mode A doc edit (docs/how-it-works.md arbiter freeze narrative).
  - P1.M1.T3: arbiter freeze-parity invariant integration tests (consume OverlayTreePaths via the arbiter).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/git/git.go internal/git/overlaytree_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/git/...        # Expected: exit 0
go build ./...                   # Expected: exit 0 (new interface method satisfied by gitRunner)
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The new primitive's comprehensive tests (real git, temp repos)
go test -race ./internal/git/ -v -run TestOverlayTreePaths

# Expected: ALL cases pass — overlay-only (a.go'+b.go+c.go), deletion-overlay (b.go removed),
# empty-paths-noop (baseTree verbatim, index unchanged), mid-chain-simulation (leftovers folded ⇒ == tStart),
# BadTree/NotARepo/GitBinaryMissing/ContextCancelled (non-nil errors), DoesNotTouchWorkingTree (file unchanged).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green (S1 adds a primitive; no caller change)
go vet ./...                     # Expected: exit 0

# Confirm the interface is satisfied (gitRunner implements OverlayTreePaths) — `go build ./...` above covers it.
# Confirm ONLY the 2 intended files changed / 1 new file added
git status --porcelain -- internal/
# Expected: modified internal/git/git.go + new internal/git/overlaytree_test.go. Nothing else.
```

### Level 4: Behavioral Smoke (the freeze-safety property, direct)

```bash
cd /home/dustin/projects/stagecoach

# TestOverlayTreePaths_DoesNotTouchWorkingTree is the direct assertion. Cross-check the core fold property:
# OverlayTreePaths(tree1, tStart, DiffTreeNames(tree1,tStart)) must yield a tree whose content == tStart.
go test -race ./internal/git/ -v -run 'TestOverlayTreePaths_MidChainLeftoverSimulation'

# Expected: PASS. The mid-chain case is the exact fold the arbiter (P1.M1.T2.S1) will rely on:
# leftoverPaths = diff-names(tipTree, T_start); OverlayTreePaths(tree[j], T_start, leftoverPaths) folds them in.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0 (gitRunner satisfies the new interface method).
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] `Git.OverlayTreePaths` declared (after `DiffTreeNames`) with the doc-block.
- [ ] `gitRunner.OverlayTreePaths`: empty paths → baseTree verbatim; else read-tree → ls-tree → per-path update-index → write-tree.
- [ ] `parseLsTree` parses `<mode> <type> <blob>\t<path>` → `map[path]lsTreeEntry{mode,blob}`.
- [ ] Mutation exit-code convention (code != 0 ⇒ wrap; err != nil ⇒ UNWRAPPED; NO 128 special-case).
- [ ] Index/object-store-only mutation; never touches working tree; never moves a ref.
- [ ] All 6 named cases + 4-case negative set + DoesNotTouchWorkingTree pass.

### Scope Discipline Validation

- [ ] ONLY `internal/git/git.go` modified + `internal/git/overlaytree_test.go` created (git status confirms).
- [ ] Did NOT edit `internal/decompose/*` (arbiter rewrite = P1.M1.T2.S1) or any other package.
- [ ] Did NOT add docs (P1.M1.T2.S2 / P3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Mirrors `DiffTreeNames`/`ReadTree` exit-code convention + error-wrapping style.
- [ ] Mirrors `FreezeWorkingTree` discipline (index/object-store-only; no ref; no working-tree touch).
- [ ] Tests reuse `initRepo`/`writeFile`/`stageFile`/`execGit`; real git binary; no testify.
- [ ] The 4-case negative set mirrors `readtree_test.go` exactly; DoesNotTouch mirrors `freezeworkingtree_test.go:122`.

---

## Anti-Patterns to Avoid

- ❌ Don't introduce a 128-as-non-error special-case. `OverlayTreePaths` uses the MUTATION exit-code
  convention (code != 0 ⇒ wrapped error; err != nil ⇒ UNWRAPPED) — identical to ReadTree/WriteTree/Add/
  DiffTreeNames. The 128 special-case is RevParseHEAD's unborn-repo signal and is UNRELATED.
- ❌ Don't forget the empty-paths early return. `if len(paths) == 0 { return baseTree, nil }` MUST be the
  first statement — it skips read-tree AND write-tree. The empty-paths-noop test pins that the index is
  unchanged (assert write-tree before == write-tree after).
- ❌ Don't touch the working tree or move a ref. OverlayTreePaths runs read-tree + update-index + write-tree
  ONLY — no checkout, no reset --hard, no `add` (working-tree read), no update-ref, no commit-tree. The
  DoesNotTouchWorkingTree test pins this; the discipline matches FreezeWorkingTree.
- ❌ Don't use the 3-arg `--cacheinfo` form. Use the single-arg comma form `<mode>,<blob>,<path>` (git 2.0+)
  via `fmt.Sprintf("%s,%s,%s", ent.mode, ent.blob, p)`. The 3-arg form was deprecated in git 2.0.
- ❌ Don't parse ls-tree with `strings.Split(line, " ")` for the whole line — the path follows a `\t`, not a
  space. Split on the first `\t` (IndexByte), then `strings.Fields` the LEFT side for mode/type/blob.
- ❌ Don't mutate a shared args backing array. `append([]string{...}, paths...)` allocates a fresh slice for
  the ls-tree call (the arch pseudocode does this).
- ❌ Don't build the (mode, blob) map with per-path `ls-tree` calls — ONE `ls-tree -r --full-tree sourceTree
  -- paths...` reads every entry. Per-path calls would be O(N) git invocations.
- ❌ Don't edit `internal/decompose/*` or add arbiter tests here — the arbiter rewrite + its integration
  tests are P1.M1.T2 / P1.M1.T3. S1 is the leaf primitive + its own unit tests only.
- ❌ Don't use testify or t.TempDir-less tests. The internal/git convention is in-package (`package git`),
  t.TempDir(), real git binary, reusing initRepo/writeFile/stageFile/execGit. Mirror readtree_test.go.
- ❌ Don't skip the 4-case negative set or DoesNotTouchWorkingTree — they're the standard coverage every
  internal/git primitive carries (BadTree/NotARepo/GitBinaryMissing/ContextCancelled + the working-tree
  invariant). Their absence would be a coverage gap.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: The arch `arbiter_freeze_parity.md §3` supplies verbatim copy-paste-ready code for both
`OverlayTreePaths` and `parseLsTree`, plus the verbatim interface doc-block, the exact placement (after
DiffTreeNames / near FreezeWorkingTree), and the exit-code convention. The primitive mirrors two
already-proven disciplines: the mutation exit-code convention (DiffTreeNames/ReadTree) and the
index/object-store-only no-ref/no-working-tree-touch discipline (FreezeWorkingTree). The test design reuses
four existing helpers (initRepo/writeFile/stageFile/execGit) and mirrors two existing test patterns
(readtree_test.go's 4-case negative set, freezeworkingtree_test.go's LeavesWorkingTreeUnchanged), so the
test file is low-risk. The one residual uncertainty (not 10/10) is the mid-chain-leftover-simulation test's
exact tree-construction bookkeeping (building tree1 vs tStart so that DiffTreeNames(tree1,tStart) is the
leftover set), which the implementer must get right for the fold assertion to hold — mitigated by the
`execGit diff-tree` oracle and the clear "result content == tStart" property. The arbiter rewrite (T2) and
docs (T2.S2/P3) are cleanly fenced and untouched.
