# S2 Verified Touchpoint Map — Scoped ReadTreeInto/WriteTreeFrom (P1.M2.T1.S2)

> Verified against the LIVE repo (module github.com/dustin/stagecoach, 2026-07-06). Research only.
> S1 (runWithEnv) is ALREADY APPLIED — git.go imports "os" + "path/filepath"; git_test.go has
> TestGitRunner_RunWithEnv_PassesEnv. S2 builds directly on it.

## 1. S1 is DONE — runWithEnv + imports already present

`runWithEnv(ctx, repo, extraEnv []string, args ...string) (stdout, stderr string, exitCode int, err error)`
exists on `*gitRunner` (S1). git.go's import block ALREADY has `"os"` (line 9) AND `"path/filepath"`
(line 11). So S2 needs ZERO new imports — `filepath.Abs` is available. S2's only callers of runWithEnv
in production (the scoped variants) also satisfy any `unused`-lint concern for runWithEnv beyond S1's test.

## 2. The two unscoped templates (the API + impl source)

**ReadTree** (git.go:1222, interface ~169) — `g.run(ctx, g.workDir, "read-tree", tree)`; stdout unused
(read-tree prints nothing); `code != 0 → fmt.Errorf("git read-tree: failed (exit %d): %s", code,
TrimSpace(stderr))`. ALL non-zero exits are errors (mutation convention; no 128 special-case).

**WriteTree** (git.go:492, interface ~95) — `g.run(ctx, g.workDir, "write-tree")`; on `code != 0` it probes
`g.run(ctx, g.workDir, "ls-files", "-u")` for a clean "unresolved merge conflicts" message, else
`fmt.Errorf("git write-tree failed (exit %d): %s", ...)`; success → `return strings.TrimSpace(stdout), nil`.

The scoped variants mirror these EXACTLY, swapping `g.run(...)` → `g.runWithEnv(ctx, g.workDir,
[]string{"GIT_INDEX_FILE=" + absIndex}, ...)`.

## 3. The GIT_INDEX_FILE absolute-path gotcha (external_deps.md §8 #1)

"A relative GIT_INDEX_FILE resolves against the hook's CWD." The hook subprocess runs with CWD=worktree
root, so a relative value would resolve to the WRONG path. The scoped variants MUST guarantee an absolute
value: `absIndex, err := filepath.Abs(indexFile); if err != nil { return ..., fmt.Errorf("...: resolve
index path: %w", err) }`. (filepath.Abs is a no-op for already-absolute paths — os.CreateTemp/t.TempDir
results are absolute, the normal case. It exists to CATCH a relative path.) Then
`extraEnv := []string{"GIT_INDEX_FILE=" + absIndex}`.

## 4. The two new methods — signatures + bodies

```go
// ReadTreeInto primes a THROWAWAY index (indexFile) from <tree> via GIT_INDEX_FILE-scoped read-tree.
// It is the scoped sibling of ReadTree: identical EXCEPT it writes indexFile, NOT .git/index — the live
// index is UNTOUCHED (FR-V3: pre-commit scoped to T_start via a throwaway index). indexFile is made
// absolute (external_deps.md §8 #1). ALL non-zero exits are errors (same mutation convention as ReadTree).
func (g *gitRunner) ReadTreeInto(ctx context.Context, tree, indexFile string) error {
	absIndex, err := filepath.Abs(indexFile)
	if err != nil {
		return fmt.Errorf("git read-tree (scoped): resolve index path: %w", err)
	}
	_, stderr, code, err := g.runWithEnv(ctx, g.workDir,
		[]string{"GIT_INDEX_FILE=" + absIndex}, "read-tree", tree)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("git read-tree (scoped): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// WriteTreeFrom captures the tree SHA of a THROWAWAY index (indexFile) via GIT_INDEX_FILE-scoped write-tree.
// It is the scoped sibling of WriteTree: identical EXCEPT it reads indexFile, NOT .git/index. indexFile is
// made absolute (external_deps.md §8 #1). Mirrors WriteTree's ls-files -u merge-conflict probe (scoped).
func (g *gitRunner) WriteTreeFrom(ctx context.Context, indexFile string) (sha string, err error) {
	absIndex, err := filepath.Abs(indexFile)
	if err != nil {
		return "", fmt.Errorf("git write-tree (scoped): resolve index path: %w", err)
	}
	env := []string{"GIT_INDEX_FILE=" + absIndex}
	stdout, stderr, code, err := g.runWithEnv(ctx, g.workDir, env, "write-tree")
	if err != nil {
		return "", err
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
```

## 5. Interface placement — co-locate each scoped variant with its unscoped sibling

Add BOTH to the `Git` interface (git.go:87 — they ARE the public scoped surface, unlike runWithEnv which
stays off-interface). Place `WriteTreeFrom` immediately after `WriteTree` (interface ~95) and `ReadTreeInto`
immediately after `ReadTree` (interface ~169) — the "co-located with siblings" discipline used throughout
git.go (run/runWithInput/runWithEnv are co-located; each *_test.go is per-primitive). The gitRunner method
impls go next to their siblings too (WriteTreeFrom after WriteTree ~492; ReadTreeInto after ReadTree ~1222).

## 6. DiffTreeNames reuse — read-only, index-agnostic (NOT this task's work)

`DiffTreeNames(ctx, treeA, treeB) ([]string, error)` (git.go:1582) runs `git diff-tree -r --name-only
--no-commit-id <treeA> <treeB>` — it compares two TREE SHAs, never touches the index. It is ALREADY the
subset-check helper (P1.M2.T2.S1 consumes it: `DiffTreeNames(snapshotTree, newTree)` must be a subset of
`DiffTreeNames(baseTree, tStart)`). S2 does NOT touch it — just notes it's the read-only reuse target.

## 7. Test patterns (mirror readtree_test.go / writetree_test.go)

Dedicated per-primitive test files exist: `readtree_test.go`, `writetree_test.go`, `difftreenames_test.go`,
etc. Helpers (in committree_test.go): `initRepo(t, dir)`, `writeFile(t, dir, name, body)`,
`stageFile(t, dir, name)`, `makeEmptyCommit(t, repo, msg)`, `writeTreeOf(t, repo)` (tree SHA),
`execGit(t, repo, args...)` (trimmed stdout). Tests use INDEPENDENT ORACLES (`exec.Command("git", ...)`
with `GIT_INDEX_FILE=<tmp>` to inspect the throwaway index; plain `git ls-files` to inspect the live one).

THE KEYSTONE INVARIANT for the scoped-variant tests: **the live .git/index is byte-identical before/after**.
Assert via `git ls-files` (or hashing `.git/index`) before and after the scoped call — it MUST be unchanged
(this is what distinguishes ReadTreeInto/WriteTreeFrom from ReadTree/WriteTree). The throwaway index is
inspected via `GIT_INDEX_FILE=<tmp> git ls-files` (an independent oracle, NOT via the runner).

NEW test files: `readtreeinto_test.go` + `writetreefrom_test.go` (per-primitive discipline). Each covers:
happy-path (tree loaded into tmpIndex / SHA returned), **live-index-untouched** (the keystone), BadTree
(ReadTreeInto) / empty index (WriteTreeFrom → EmptyTreeSHA), GitBinaryMissing, ContextCancelled.
