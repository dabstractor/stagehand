# P1.M1.T2.S6 — DiffTree Validation Research

> Empirically verified against **git 2.54.0** + **go1.26.4** on this box. Every claim below was
> produced by running the literal command in a throwaway `git init` temp repo (transcripts inline).

## 1. Signature reconciliation (interface is authoritative — mirrors S4/S5)

The work-item CONTRACT prose writes `DiffTree(ctx, sha string, isRoot bool) ([]FileChange, error)`.
The `Git` interface (S1, **ALREADY LANDED** on disk in `internal/git/git.go`) is:

```go
DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error)
```

**They match exactly** — no reconciliation needed (unlike S5's `ref` parameter). The only thing to
verify is that `FileChange` is already declared with the right fields:

```go
type FileChange struct {
    Status  string // "A","M","D","R","C","T","U"; R/C carry a similarity score e.g. "R100"
    SrcPath string // non-empty only for R/C (the rename/copy source); "" otherwise
    Path    string // the destination path — always set
}
```

Confirmed on disk (landed by S1). So S6 changes **zero types** — it only replaces the `DiffTree`
panic-stub body with the real implementation + adds a private `parseDiffTree` helper.

## 2. Empirically-pinned `git diff-tree` behavior (git 2.54.0)

### 2.1 The command family

```
git diff-tree --no-commit-id --name-status -r [--root] <sha>
```

| Flag | Effect |
|---|---|
| `-r` | recurse into subtrees (without it you get top-level tree entries only). ALWAYS use. |
| `--no-commit-id` | suppress the leading commit-SHA line `diff-tree` prints by default. ALWAYS use. |
| `--name-status` | print `<status>\t<path>` (and `<status>\t<src>\t<dst>` for R/C). |
| `--root` | for a root commit (no parent): diff against the empty tree → every file shows as `A`. |

### 2.2 The exit-code / output matrix (verified)

| Scenario | Command | stdout | exit |
|---|---|---|---|
| child commit vs parent | `diff-tree --no-commit-id --name-status -r <child>` | `M\ta\nA\tb\n...` | **0** |
| root commit, WITH `--root` | `diff-tree --no-commit-id --name-status -r --root <root>` | `A\ta\nA\tb\n...` | **0** |
| root commit, WITHOUT `--root` | `diff-tree --no-commit-id --name-status -r <root>` | **(empty)** | **0** |
| no-change commit (== parent) | `diff-tree --no-commit-id --name-status -r <noop>` | **(empty)** | **0** |
| bad SHA (all-zeros) | `diff-tree --no-commit-id --name-status -r 0000…0` | (empty) | **128** |

**Transcript (verbatim from the throwaway repo):**
```
=== ROOT COMMIT (no --root) ===
diff-tree WITHOUT --root:           EXIT=0      (no output)
diff-tree WITH --root:              A	a.txt     EXIT=0

=== CHILD COMMIT with A/M/D ===
diff-tree child (vs parent):        M	a.txt
                                    A	b.txt
                                    A	c.txt    EXIT=0

=== no-change commit ===
diff-tree noop:                     EXIT=0 OUTPUT_LEN=0   (empty → empty slice)

=== BAD SHA ===
fatal: bad object 0000000000000000000000000000000000000000
EXIT=128
```

**Key consequence:** a non-zero exit is ONLY produced by a bad SHA (128). Empty output is exit 0 in
TWO cases (root-without-`--root`, and a commit identical to its parent). So detection of "git failed"
= `code != 0` (stable), exactly as run()'s invariant promises (`err` stays `nil`, code carries 128).

### 2.3 Rename detection: the `-M` question (CRITICAL design call)

The work-item CONTRACT and PRD §9.9/FR42 specify the EXACT command:
`git diff-tree --no-commit-id --name-status -r <NEW_SHA>` — **no `-M`**.

Verified what `-M` (rename detection) changes:

```
=== rename: default (no -M) ===
D	b.txt
A	renamed.txt               EXIT=0          ← 2-field D + 2-field A

=== rename: WITH -M ===
R100	b.txt	renamed.txt     EXIT=0          ← 3-field R100 line
```

**Decision D1 — do NOT add `-M`:** the command must reproduce commit-pi's exact UX verbatim
(PRD Appendix C: "`git diff-tree --name-status` success print | main.go | Identical UX"). With the
default (no `-M`), renames surface as a `D`+`A` pair (both 2-field lines). The parser STILL handles
3-field `R`/`C` lines defensively — so if a future caller adds `-M`, or git changes a default, the
parser is already correct. The `FileChange.SrcPath` field is exercised by a direct unit test of
`parseDiffTree` (synthetic 3-field input) rather than via DiffTree itself, because the production
command never emits a 3-field line.

## 3. The parse contract (tab-separated, stable)

`--name-status` output is one line per changed path, tab-separated (verified with `cat -A`):

```
A\tpath/to/new/file                 (2 fields: status, path)
M\tpath/to/modified
D\tpath/to/deleted
R100\told/name\tnew/name            (3 fields: status+score, src, dst)
C90\tsrc.txt\tdst.txt
```

Parse rule (from `git_plumbing_reference.md` §7, confirmed empirically):
- `strings.Split(line, "\t")`
- 2 fields → `FileChange{Status: f[0], Path: f[1]}`
- 3 fields → `FileChange{Status: f[0], SrcPath: f[1], Path: f[2]}` (rename/copy)
- anything else → skip (defensive; git output is well-formed so this never fires in practice)

Trailing newline: git terminates output with `\n`. `strings.TrimSpace(out)` before `Split` removes
it; the resulting empty final segment is skipped by the `if line == ""` guard. For fully-empty output
(root-without-`--root`, no-change commit) `TrimSpace("") == ""`, `Split("", "\n") == [""]`, the loop
skips it, and `parseDiffTree` returns a nil slice (len 0). Verified.

## 4. GOTCHA — octal-quoted paths (`core.quotepath`)

**Verified** that with the DEFAULT `core.quotepath=true`, git wraps non-ASCII paths in double quotes
and octal-escapes the bytes:

```
default quotepath (quoted):    A^I"sp\303\251cial.txt"$       (cat -A; ^I = TAB)
with -c core.quotepath=false:  A^IspM-CM-)cial.txt$            (raw UTF-8)
```

So the path field for `spécial.txt` is literally `"sp\303\251cial.txt"` (quotes + octal) under the
default. This is a **known v1 limitation**: the production command does NOT pass
`-c core.quotepath=false` (it must match PRD §9.9/Appendix C verbatim), so non-ASCII paths are
presented with git's default quoting. **This is acceptable** — it is byte-identical to what commit-pi
showed (the "Identical UX" requirement). The tab-split still works (the quote is part of the path
field, not a separator). Documented as gotcha G6 in the PRP; NOT addressed in code (would diverge
from the PRD command).

## 5. Why a separate `parseDiffTree` function (not inline)

S2/S3/S4/S5 each implement a single git call with a trivial post-step (TrimSpace + branch). DiffTree
is the first method whose post-step (line splitting + field routing) is non-trivial. Splitting the
pure `parseDiffTree(out string) []FileChange` out from the exec method:
1. makes the rename/copy 3-field path directly unit-testable WITHOUT depending on `-M` (which the
   production command deliberately omits);
2. separates the "exec git" concern (testable via a real repo) from the "parse bytes" concern
   (testable via synthetic strings, no git needed);
3. is idiomatic Go and trivially readable.

There is no precedent FOR or AGAINST in the landed code (the other methods had no parsing step), so
this is a clean, uncontroversial extension of the established pattern.

## 6. The `--root` / `isRoot` contract

`DiffTree` takes `isRoot bool` (NOT a parent SHA). The orchestrator (P1.M3.T4) already knows whether
the commit it just made is a root commit — it has `isUnborn` from `RevParseHEAD` (S2) and the
`parents` slice it passed to `CommitTree` (S4: `len(parents)==0` ⟺ root). So `isRoot` is a natural,
already-available input; DiffTree does NOT need to inspect the commit object to discover root-ness
(which would be an extra `cat-file` call). Decision D2: the caller drives `isRoot`; DiffTree just
appends `--root` when true. This mirrors how `CommitTree` (S4) is driven by `parents` and
`UpdateRefCAS` (S5) is driven by the caller's `expectedOld` — the plumbing layer stays dumb.

## 7. Test design matrix

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestDiffTree_ChildCommit` | root commit (keep.txt, gone.txt); child commit (M keep, D gone, A new) | `err==nil`; changes map has `M`→keep.txt, `D`→gone.txt, `A`→new.txt | the A/M/D 2-field parse path against a real repo (the primary UX) |
| `TestDiffTree_RootCommit_WithRootFlag` | root commit (two files); `isRoot=true` | every change is `A`; both paths present | `--root` is appended and diffs against the empty tree (FR42 root edge) |
| `TestDiffTree_RootCommit_WithoutRootFlag` | same root commit; `isRoot=false` | `err==nil`; **empty** changes | proves `--root` matters: without it a root commit yields nothing (the trap the `isRoot` param exists to avoid) |
| `TestDiffTree_NoChanges` | child commit identical to parent | `err==nil`; empty changes | empty-output edge (exit 0, len 0) — distinguishes "nothing" from "error" |
| `TestDiffTree_BadSHA` | all-zeros SHA | `err` non-nil, contains `"git diff-tree: failed"` + `"(exit 128)"`; nil slice | bad-object path → exit 128 → wrapped error |
| `TestDiffTree_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains `"git binary not found"` | infrastructural failure surfaces cleanly |
| `TestDiffTree_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() propagated |
| `TestParseDiffTree_Formats` | synthetic 5-line string (A/M/D/R100/C90) | 5 FileChange; R100 & C90 carry SrcPath | the 3-field rename/copy parse path (NOT reachable via the production command — needs `-M`; tested directly) |
| `TestParseDiffTree_Empty` | `""` | `len==0` | empty/trailing-newline handling |

## 8. Helper-name collision avoidance (S5 lands concurrently)

Existing helper names (declared in `package git` test files — all visible across files):
- `git_test.go` (S1): `initRepo`, `assertPanics`
- `revparse_test.go` (S2): `minGitEnv`, `makeEmptyCommit`
- `writetree_test.go` (S3): `makeMergeConflict`
- `committree_test.go` (S4): `setIdentityConfig`, `writeFile`, `stageFile`, `writeTreeOf`, `headSHA`, `commitMessage`
- `updateref_test.go` (S5, planned): `casCommit`, `casHEAD`, `casMoveHEAD`, `casOut`, `gitIdentityEnv`

S6's NEW helpers use a **`dt` prefix** (diff-tree): `dtCommit`, `dtRemove`. Distinct from every name
above. S6 REUSES (does not redeclare) `initRepo`, `writeFile`, `stageFile`, `headSHA`, `minGitEnv`.
Decision D3: the `dt` prefix keeps the package compiling when S5 and S6 land together.

## 9. Decisions log

- **D1** — do NOT add `-M` to the diff-tree command. Match the PRD §9.9/FR42 exact command
  (`diff-tree --no-commit-id --name-status -r <sha>`) and Appendix C's "Identical UX". Renames
  surface as D+A; the parser still handles 3-field R/C defensively + via a direct unit test.
- **D2** — `isRoot` is caller-supplied (the orchestrator already knows root-ness from RevParseHEAD's
  isUnborn / CommitTree's parents). DiffTree does NOT cat-file the commit to discover it. Plumbing
  layer stays dumb (mirrors S4/S5).
- **D3** — new test helpers use a `dt` prefix; reuse existing helpers verbatim.
- **D4** — split `parseDiffTree` out as a private function (testable; separates exec from parse).
- **D5** — branch on `code != 0` for the bad-SHA failure (NOT `code == 128`): only 128 is observed,
  but `!= 0` is stable and matches the established S2/S3/S4/S5 method pattern.
- **D6** — octal-quoted paths (`core.quotepath`) are a documented v1 limitation, NOT addressed (would
  diverge from the PRD command); the tab-split still works.
- **D7** — `parseDiffTree` returns a nil slice (len 0) for empty input (idiomatic; range-safe). Not
  initialized to a non-nil empty slice — callers only range over it.
