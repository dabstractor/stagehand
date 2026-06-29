# P1.M1.T3.S5 — Research: AddAll + StagedFileCount

> Subtask implements the LAST two git-wrapper methods in `internal/git/git.go`:
> `AddAll(ctx) error` (`git add -A`) and `StagedFileCount(ctx) (int, error)`
> (`git diff --cached --name-only` + count non-empty lines). StagedFileCount feeds the FR18
> "Nothing staged — staging all changes (N files)." notice.

## 0. The central structural finding — StagedFileCount is ABSENT from the `Git` interface

The work-item contract mandates TWO methods, but the current `Git` interface (read from
`internal/git/git.go`) declares ONLY `AddAll`:

```go
// ... (interface tail, as it exists today) ...
	RecentSubjects(ctx context.Context, n int) (subjects []string, err error)
	CommitCount(ctx context.Context) (count int, err error)
	AddAll(ctx context.Context) error     // ← the ONLY S5 method present today
}
```

There is **no `StagedFileCount`** in the interface, and no stub for it. The interface ownership
comment block lists only `AddAll — P1.M1.T3.S5`.

**Decision D1:** This subtask MUST add `StagedFileCount(ctx context.Context) (int, error)` to the
`Git` interface (with a doc comment matching the existing style) AND implement its body, in addition
to replacing the `AddAll` panic-stub. This is the single most important structural addition in this
subtask — missing it means the FR18 notice cannot be produced. The interface is otherwise frozen
(S1's contract); we ADD one method, we change no existing signature.

## 1. Empirical verification — `git add -A` (AddAll)

Run against real git (git 2.x) in temp repos:

| Scenario | `git add -A` exit | Notes |
|---|---|---|
| Born repo, modified + untracked files | **0** | stages everything (verified: a.txt, b.txt, c.txt all staged) |
| Born repo, clean tree (nothing to stage) | **0** | safe no-op |
| Unborn repo (0 commits) + untracked file | **0** | stages vs empty tree |
| Unborn repo, empty | **0** | no-op |
| Non-repo directory | **128** | `fatal: not a git repository` — a REAL error |

**Implication for the branch order (D2):** `git add -A` exits 0 on every happy path (born/unborn,
with/without changes). It is a MUTATION (writes the index), structurally identical to `WriteTree` /
`CommitTree` — which treat ALL non-zero exits as errors. It does NOT have the exit-128⇒"unborn is not
an error" semantic that the READ methods (`RevParseHEAD`, `RecentMessages`, `CommitCount`) have,
because add -A never exits 128 in a repo. So `AddAll`'s branch order is the SIMPLE mutation form:

```
err != nil → return err               // git missing / ctx cancelled / start failure (code==-1)
code != 0  → return wrapped error     // non-repo (128), corrupt repo, etc.
code == 0  → return nil               // staged (or clean-tree no-op)
```

Byte-identical to `WriteTree` / `CommitTree` (no 128 special-case). This is the correct analogy;
`RevParseHEAD` (which DOES special-case 128) is the WRONG analogy because add -A never needs to.

## 2. Empirical verification — `git diff --cached --name-only` (StagedFileCount)

| Scenario | exit | stdout | Notes |
|---|---|---|---|
| Born repo, 3 staged files | **0** | `a.txt\nb.txt\nc.txt\n` | one path per line, trailing newline |
| Born repo, nothing staged (clean) | **0** | `""` | EMPTY output, still exit 0 |
| Born repo, staged file | **0** | the path | (without --quiet) |
| Unborn repo + 1 staged file | **0** | `f.txt\n` | diffs vs empty tree |
| Unborn repo, nothing staged | **0** | `""` | exit 0 |
| **Non-repo directory** | **129** | (usage dump) | `git diff` falls into `--no-index` mode; `--cached` invalid there → 129 |

### 2a. THE central gotcha for StagedFileCount — NO exit-1 inversion

`git diff --cached --name-only` (the form this method uses) **omits `--quiet`**. Without `--quiet`:

- `git diff` exits **0 whether or not changes exist** (verified: staged file → exit 0; nothing staged → exit 0).
- The exit-1 inversion (FINDING 6) is a property of `--quiet` ALONE (`git diff --cached --quiet` →
  exit 1 means staged). 

Therefore `StagedFileCount` MUST use the SIMPLE branch form (`code != 0 → error`), NOT
`HasStagedChanges`'s `switch code { case 0/1/default }` form. A future reader who "optimizes" by
adding `--quiet` would BREAK this method: `--quiet` suppresses stdout entirely (no file names to
count). **The `--name-only` (with output) form is load-bearing — it is what produces the list to
count.** Document this prominently (gotcha).

This is the analog of S4's `\n`-vs-`%x00` design call: a close sibling (`HasStagedChanges`) uses a
deceptively similar-looking command, and the DIFFERENCE (here: omitting `--quiet`; there: `%s` vs
`%B`) is the load-bearing detail. Do NOT cargo-cult `HasStagedChanges`'s `--quiet` here.

### 2b. Counting approach (D3) — split on "\n", count non-empty lines

The contract mandates: "Count via `git diff --cached --name-only | wc -l` equivalent (split lines)."

```go
count := 0
for _, line := range strings.Split(stdout, "\n") {
    if strings.TrimSpace(line) != "" {
        count++
    }
}
```

- The trailing newline after the last path yields a final `""` element → dropped by the empty-skip.
- Empty output (nothing staged) → `""` → `Split` → `[""]` → 1 empty element → count 0. Verified.
- 3 files → count 3. Verified.

### 2c. Filenames with SPACES stay on ONE line (D4)

Verified: a file `sub/has space.txt` is emitted as the single line `sub/has space.txt` (NOT quoted,
NOT split). So line-counting is correct for the overwhelmingly common case (spaces in names).

**Considered-and-rejected alternative: `--name-only -z` (NUL-delimited).** `git diff --cached
--name-only -z` would emit paths NUL-delimited, robust against filenames containing EMBEDDED
NEWLINES (pathological). The contract explicitly mandates the `wc -l` line-split approach, and FR18's
"N files" notice is informational (display text, not a transactional invariant), so the line-count is
sufficient and matches the contract. A filename with a literal newline in it would inflate the count —
this is an accepted, documented limitation (such filenames are vanishingly rare and hostile to every
shell tool). Do NOT switch to `-z`; the contract chose split-lines.

### 2d. Non-repo exits 129, NOT 128 (D5)

For `git diff --cached --name-only` in a NON-repo directory, git falls back to its `--no-index`
two-file diff mode; `--cached` is invalid there, so it exits **129** (with a usage dump to stderr),
not 128. This is DISTINCT from `git log` / `git rev-list` (which exit 128 on non-repo). For
`StagedFileCount` this difference is INCONSEQUENTIAL: the branch logic is `code != 0 → error`
regardless of whether the code is 128 or 129. The error message includes the exit code + trimmed
stderr, so the 129 surfaces in diagnostics. No special-casing needed (mirrors DiffTree's "branch on
code != 0, not on a specific code").

## 3. Stub-guard closure (D6) — remove TestStubsPanic + assertPanics

`AddAll` is the FINAL panic-stub. Current `internal/git/git_test.go`:

```go
func TestStubsPanic(t *testing.T) {
	ctx := context.Background()
	g := New(".")
	assertPanics(t, "AddAll", func() { _ = g.AddAll(ctx) })
}
```

Once `AddAll` is real, this test FAILS ("expected panic, but did not panic"). Prior subtasks removed
only the one assertPanics line. But AddAll is the LAST stub — removing its line leaves `ctx` and `g`
as UNUSED locals → **Go compile error** (`ctx declared and not used`).

**Decision D6:** REMOVE the entire `TestStubsPanic` function AND the `assertPanics` helper. Grep
confirms `assertPanics` is referenced ONLY at `git_test.go:104` (definition) and `git_test.go:123`
(sole call, inside TestStubsPanic). No other test uses it. The stub-guard scaffolding has served its
purpose (S1's "panic to fail fast" for each not-yet-implemented method); with zero stubs remaining it
is dead code. Full removal is the clean closure and avoids the unused-local compile error. This is a
slightly larger `git_test.go` edit than prior subtasks, but it is the logically correct end state.

## 4. No new imports, no new constants (D7)

Both method bodies use only `strings` (Split/TrimSpace) and `fmt` (Errorf/Sprintf) — both ALREADY
imported in `git.go` (the import block currently lists: bytes, context, errors, fmt, io, os/exec,
strconv, strings). StagedFileCount introduces no constant. AddAll introduces no constant. The import
block and the `defaultExcludes`/`defaultMaxMDLines`/`maxRecentMessageLines` const region are
UNTOUCHED.

## 5. Parallel-execution / file-state note

The `git.go` read for this research shows `RecentSubjects` (S4), `RecentMessages`/`CommitCount`
(S3), and all S2/S3/S4/S5/S6/T3.S1/T3.S2 methods already REAL — i.e. this working copy is at the
end of P1.M1.T3 with ONLY `AddAll` left as a stub, and `TestStubsPanic` covering only AddAll.
Regardless of whether S4 is "concurrent" or already landed, this subtask's edits are entirely
NON-OVERLAPPING with any prior region:
- NEW interface method `StagedFileCount` (appended after `AddAll` in the interface tail) — a region
  no prior subtask touched (AddAll's interface line was added by S1 and never re-edited).
- REPLACES the `AddAll` panic-stub body (the only stub).
- ADDS the `StagedFileCount` method body (new, after `CommitCount`).
- REMOVES `TestStubsPanic` + `assertPanics` from `git_test.go`.
- CREATES `addall_test.go` + `stagedcount_test.go`.

No import block change, no const change, no existing-method change.

## 6. External references (exact)

- https://git-scm.com/docs/git-add — `git add -A` "update the index ... where the current working
  tree matches ... `-A, --all` ... Update the index not only where the working tree has a file
  matching `<pathspec>` but also where the index already has an entry. This adds, modifies, and
  removes index entries to match the working tree." Confirms `-A` stages modified + removed +
  untracked.
- https://git-scm.com/docs/git-add#_stages_modified_and_untracked_files — the canonical `-A`
  semantics: stages new, modified, AND deleted files across the whole worktree.
- https://git-scm.com/docs/git-diff#Documentation/git-diff.txt---name-only — `--name-only`: "Show
  only names of changed files." One path per line (the form without `-z`).
- https://git-scm.com/docs/git-diff#Documentation/git-diff.txt---cached — `--cached`/`--staged`:
  "view the changes you staged for the next commit."
- https://git-scm.com/docs/git-diff#Documentation/git-diff.txt--quiet — `--quiet`: "Disable all
  output of the program. Implies `--exit-code`." — THIS is the source of FINDING 6's inversion; we
  deliberately OMIT it (D2a).
- https://git-scm.com/docs/git-diff#Documentation/git-diff.txt-z — `-z`: NUL-delimited output for
  pathnames; the considered-and-rejected alternative (D4).

## 7. Test matrix (final)

**`addall_test.go` (package git, white-box — reuse initRepo, writeFile, stageFile, makeEmptyCommit):**

| Test | Fixture | Asserts | Proves |
|---|---|---|---|
| `TestAddAll_StagesModifiedAndUntracked` | born; commit a.go; modify a.go + create b.go (untracked); AddAll | inline `git diff --cached --name-only` lists BOTH a.go and b.go; err==nil | add -A stages modified + untracked (FR16/FR20) |
| `TestAddAll_StagesDeletion` | born; commit a.go; `os.Remove(a.go)`; AddAll | staged set contains a.go (as deleted) | add -A stages deletions too |
| `TestAddAll_CleanTreeNoOp` | born; commit; clean tree; AddAll | err==nil; StagedFileCount==0 | add -A is a safe no-op on clean tree |
| `TestAddAll_UnbornRepoStagesFiles` | unborn; create f.go; AddAll | err==nil; StagedFileCount==1 | add -A works on unborn (vs empty tree) |
| `TestAddAll_NotARepo` | plain TempDir (no init) | err!=nil contains "git add" | non-repo (exit 128) → wrapped error |
| `TestAddAll_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found" | run() err path (G2) |
| `TestAddAll_ContextCancelled` | cancel() before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced |

**`stagedcount_test.go` (package git, white-box):**

| Test | Fixture | Asserts | Proves |
|---|---|---|---|
| `TestStagedFileCount_NothingStaged` | born clean | count==0, err==nil | empty output → 0 (D3) |
| `TestStagedFileCount_ThreeFiles` | stage 3 files | count==3 | line-count correct |
| `TestStagedFileCount_AfterAddAll` | modify+untracked; AddAll | count==2 | integration w/ AddAll (FR18 N) |
| `TestStagedFileCount_IncludesDeletion` | commit a.go; rm; AddAll | count==1 | deletions counted |
| `TestStagedFileCount_FilenameWithSpace` | file "sub/has space.txt"; AddAll | count==1 | space-in-name stays one line (D4) |
| `TestStagedFileCount_NotARepo` | plain TempDir | err!=nil contains "failed" (exit 129) | non-repo → error (D5) |
| `TestStagedFileCount_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found" | run() err path |
| `TestStagedFileCount_ContextCancelled` | cancel() before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced |
| `TestStagedFileCount_UnbornRepoWithStaged` | unborn; create+AddAll f.go | count==1, err==nil | unborn + staged diffs vs empty tree |
