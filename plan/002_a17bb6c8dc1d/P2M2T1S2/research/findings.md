# P2.M2.T1.S2 — TreeDiff Research Findings

**Work item**: Implement `TreeDiff(ctx, treeA, treeB, opts StagedDiffOptions) (string, error)` — the
tree-to-tree concept diff (PRD §13.6.3 invariant 2) with binary filtering (FR3c).
**Verified**: 2026-07-01 against git 2.x on a temp repo (all command outputs reproduced empirically).

---

## §0. The one-paragraph design

TreeDiff is **StagedDiff with the diff args swapped**. StagedDiff runs `git diff --cached …`; TreeDiff
runs `git diff <treeA> <treeB> …`. Everything else — the markdown-per-file Part 1 (line cap), the
non-markdown aggregate Part 2 (byte cap + pathspec excludes), the binary filtering (detectBinaryFiles +
fileStatuses + placeholder emission + pathspec exclusion of binary bodies) — is byte-for-byte identical.
The binary-filtering primitives (`detectBinaryFiles`, `fileStatuses`, `isBinaryByExtension`,
`binaryPlaceholderLine` in `internal/git/binary.go`) are **already variadic** and were authored with
`TreeDiff: diffArgs = [treeA, treeB]` named in their package doc (binary.go lines 19–20, 81). So TreeDiff
is a near-mechanical port of StagedDiff: copy the StagedDiff body, replace every `--cached` occurrence
in the `git diff` arg slices with the two positional tree SHAs `treeA`, `treeB`, and call
`detectBinaryFiles(ctx, treeA, treeB)` / `fileStatuses(ctx, treeA, treeB)` instead of the `--cached`
variants. No new types, no new helpers, no new deps.

---

## §1. `git diff <treeA> <treeB>` — verified behaviors (the empirical ground truth)

All reproduced in `/tmp/tdtest` (git init + two trees built via `git write-tree`).

| Scenario | Command | Exit | stdout | Notes |
|---|---|---|---|---|
| Normal two-tree diff | `git diff <A> <B>` | **0** | full unified diff (text files) + `Binary files … differ` hunk for binaries | exits 0 whether or not there are changes |
| Binary in tree diff | `git diff <A> <B>` (B adds trailer.mp4) | 0 | `Binary files /dev/null and b/trailer.mp4 differ` | **this is the useless hunk TreeDiff must replace with a placeholder** |
| numstat on tree diff | `git diff <A> <B> --numstat` | 0 | `-\t-\ttrailer.mp4` for binaries, `1\t0\ta.go` for text | ✅ `detectBinaryFiles(ctx, A, B)` works — it builds `["diff", A, B, "--numstat"]` |
| name-status on tree diff | `git diff <A> <B> --name-status` | 0 | `A\ttrailer.mp4`, `M\ta.go`, `D\tdoc.md` | ✅ `fileStatuses(ctx, A, B)` works — it builds `["diff", A, B, "--name-status"]`; status letters present for binaries |
| pathspec exclude with two trees | `git diff <A> <B> -- ':!trailer.mp4'` | 0 | trailer.mp4 gone, a.go present | ✅ pathspecs go AFTER the two positional tree SHAs |
| multiple excludes | `git diff <A> <B> -- ':!*.md' ':!trailer.mp4'` | 0 | both excluded | ✅ the Part-2 aggregate `["diff", A, B, "--", excludes...]` works |
| numstat + pathspec | `git diff <A> <B> --numstat -- ':!trailer.mp4'` | 0 | trailer.mp4 row gone | ✅ option flag before `--` pathspec |
| empty-tree base | `git diff <EMPTY> <B> --name-status` | 0 | every file as `A` | ✅ `EMPTY = 4b825dc642cb6eb9a060e54bf8d69288fbee4904` is a valid tree arg |
| **no-changes diff** | `git diff <EMPTY> <EMPTY>` | **0** | **empty stdout** | ✅ a tree diffed against itself yields `("", nil)` — NOT an error (mirrors StagedDiff nothing-staged) |
| **bad SHA** | `git diff <EMPTY> 000…000` | **128** | stderr `fatal: bad object 000…000` | ✅ exit 128 → TreeDiff returns a wrapped error |

**Key conclusions:**
1. **Exit-code convention = simple branch form** (like DiffTree / StagedFileCount / StagedDiff-Part-2
   commands): `code != 0 → error`, `code == 0 → success`. There is **NO `code == 128` special-case** for
   TreeDiff — a 128 here means a bad/unresolvable tree SHA, which is a *real* error (the caller passed
   garbage). Do NOT copy RevParseHEAD/RevParseTree's 128-as-non-error convention (that is an *unborn*
   read-methods convention; TreeDiff is neither a read-of-HEAD nor unborn-aware — the caller resolves
   the tree SHAs explicitly via RevParseTree, converting the unborn base to the empty-tree SHA).
2. **`git diff` (no `--quiet`) exits 0 even with no changes.** A no-op tree diff (treeA == treeB, or
   empty→empty) is exit 0 + empty stdout → `("", nil)`. This is the FR-M8 "tree[i] == tree[i-1]"
   detection point's *input*, but TreeDiff itself just returns "" — the *caller* (decompose orchestrator)
   compares SHAs before calling; an empty TreeDiff result is a benign non-error.
3. **Pathspec placement**: the two tree SHAs are POSITIONAL args that come BEFORE `--`; pathspec excludes
   come AFTER `--`. The StagedDiff Part-2 arg-builder already produces `["diff", "--cached", "--", excludes...]`;
   the TreeDiff port produces `["diff", treeA, treeB, "--", excludes...]`. Verified.
4. **Binary status letters are available**: `--name-status` emits `A`/`M`/`D`/`R100` for binary files too,
   so the FR3b placeholder `<status>` is sourced correctly via `fileStatuses(ctx, treeA, treeB)`.

---

## §2. The binary-filtering helpers are ALREADY variadic + already name TreeDiff

`internal/git/binary.go` (P2.M1.T1.S1, shipped) is authored to serve all three FR3c diff paths via a
variadic `diffArgs ...string`:

```go
// binary.go package doc (lines 19–20):
//   - WorkingTreeDiff:      diffArgs = []
//   - TreeDiff:             diffArgs = [treeA, treeB]
```

- `detectBinaryFiles(ctx, diffArgs ...string) (map[string]bool, error)` → TreeDiff calls
  `g.detectBinaryFiles(ctx, treeA, treeB)` → builds `["diff", treeA, treeB, "--numstat"]`. **Verified
  empirically (§1)** that `git diff <A> <B> --numstat` emits `-\t-\t<path>` for binaries → the helper's
  `added=="-" && deleted=="-"` + extension-denylist union fires correctly.
- `fileStatuses(ctx, diffArgs ...string) (map[string]string, error)` → TreeDiff calls
  `g.fileStatuses(ctx, treeA, treeB)` → builds `["diff", treeA, treeB, "--name-status"]`. **Verified (§1)**.
- `isBinaryByExtension(path, opts.BinaryExtensions)` — pure, reused verbatim with `opts.BinaryExtensions`.
- `binaryPlaceholderLine(status, path)` → `"<status>\t[binary] <path>"` — pure, reused verbatim.

**Implication: TreeDiff does NOT touch binary.go.** It only CONSUMES these helpers. No new binary logic.

### Rename reconciliation (carry over from StagedDiff verbatim — binary findings §4)

`detectBinaryFiles` keys renames by the numstat `=> ` string (`old => new`); `fileStatuses` keys by the
clean destination (`new`). The two keys DIFFER for renames. StagedDiff resolves this by **iterating over
`fileStatuses` (destination keys)** and looking the binary set up BY destination: `binSet[dest]` matches
for non-rename A/M/D; for a rename, `binSet` holds the `=> ` key (a miss) BUT `isBinaryByExtension(dest,
opts.BinaryExtensions)` catches it via the denylist. The orphaned `=> ` key in `binSet` is harmless
(never read). **TreeDiff copies this EXACT reconciliation block from StagedDiff unchanged** — it is
identical because the helpers are the same and the union logic is path-source-agnostic.

---

## §3. StagedDiff is the port template — the FULL structure is replicated

`(*gitRunner).StagedDiff` (internal/git/git.go) builds a two-part payload:

- **Part 1 — markdown per-file, line-capped.** Lists markdown files via
  `git diff --cached --name-only -- '*.md' '*.markdown'`; diffs each individually
  (`git diff --cached -- <file>`); caps each at `maxMDLines` lines with a truncation sentinel.
- **Binary filtering.** `detectBinaryFiles` + `fileStatuses` (both over `--cached`); iterates
  `statuses` (destination keys), collects binary paths (`binSet[path] || isBinaryByExtension(path,
  opts.BinaryExtensions)`), SORTs for determinism, emits `"<status>\t[binary] <path>"` placeholders, and
  gathers a SEPARATE `binExcludes []string` of `":!"+path` to drop binary bodies from Part 2.
- **Part 2 — non-markdown aggregate, byte-capped, excluded.** `git diff --cached -- <excludes> <md
  excludes> <binExcludes>`; opts.Excludes REPLACES `defaultExcludes` when non-empty; markdown excludes
  (`:!*.md`, `:!*.markdown`) are ALWAYS appended (prevents the double-count trap); aggregate capped at
  `maxDiffBytes` bytes with a sentinel.

**TreeDiff replicates BOTH parts** (see §4 for the decision rationale). The ONLY textual differences from
the StagedDiff body are:

| StagedDiff arg slice | TreeDiff arg slice |
|---|---|
| `["diff", "--cached", "--name-only", "--", "*.md", "*.markdown"]` | `["diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown"]` |
| `["diff", "--cached", "--", file]` (per-file md) | `["diff", treeA, treeB, "--", file]` |
| `detectBinaryFiles(ctx, "--cached")` | `detectBinaryFiles(ctx, treeA, treeB)` |
| `fileStatuses(ctx, "--cached")` | `fileStatuses(ctx, treeA, treeB)` |
| `["diff", "--cached", "--", excludes..., ":!*.md", ":!*.markdown", binExcludes...]` | `["diff", treeA, treeB, "--", excludes..., ":!*.md", ":!*.markdown", binExcludes...]` |

Everything else (caps resolution, defaultExcludes fallback, the `excludes := opts.Excludes; if len==0
{ excludes = defaultExcludes }` block, the sort, the placeholder emission, the truncation sentinels,
error wrapping) is **copy-identical**.

### `defaultExcludes` mutation gotcha (carry over — binary findings §5)

StagedDiff reads `opts.Excludes`; if empty, points `excludes` at the package-level `defaultExcludes`
**WITHOUT copying**. It then builds `nmArgs` from `excludes` and appends `":!*.md"`, `":!*.markdown"`,
`binExcludes` to `nmArgs` (NOT to `excludes`) — so `defaultExcludes` is never mutated. TreeDiff MUST
preserve this: build a FRESH `nmArgs` slice and append the md + binary excludes to IT, never to
`excludes` (which may alias `defaultExcludes`).

---

## §4. The markdown-per-file decision (Part 1) — replicate FULL StagedDiff structure

The contract LOGIC text says "run `git diff <treeA> <treeB>` with pathspec excludes (same pattern as
StagedDiff **Part 2** but WITHOUT --cached)" which could be read two ways:

- **(A) Aggregate-only** — only Part 2 (single aggregate command); `MaxMDLines` ignored.
- **(B) Full replication** — Part 1 (markdown per-file, line-capped) + Part 2 (aggregate, byte-capped);
  BOTH caps honored.

**Decision: (B) Full replication.** Rationale (tiebreakers all point the same way):

1. **The signature reuses `StagedDiffOptions`** (the contract is explicit:
   `opts StagedDiffOptions`). That struct carries BOTH `MaxDiffBytes` AND `MaxMDLines`. Under (A),
   `MaxMDLines` would be silently dropped — a code smell a careful author avoids. Under (B) both are
   honored. *(Decisive.)*
2. **Consumer consistency.** P3.M2.T4.S1's message agent consumes TreeDiff output for per-concept
   messages; the single-commit path's message agent consumes StagedDiff. For the same prompt + parsing
   to work on both, the payload FORMAT should match. §13.6.3 invariant 2's whole point is "the concept
   diff is the message agent's input" — i.e., a drop-in for StagedDiff.
3. **Maximal reuse / minimal divergence.** (B) is a near-mechanical port of StagedDiff (one token swap
   per arg slice). (A) requires selectively extracting Part 2 — MORE divergence, MORE risk.
4. **"apply the same caps/excludes/binary-filtering as StagedDiff"** (contract research note 1) — "caps"
   plural most naturally includes the markdown line cap.

**Risk asymmetry confirms (B):** if the author truly intended (A), the extra Part-1 loop in (B) is
harmless (a richer payload; message agent gets more context; all tests still pass). If the author
intended (B) and we ship (A), huge markdown in a concept diff blows the prompt budget with no line cap.
(B) is the safer specification under genuine ambiguity.

**The contract's "same pattern as Part 2"** describes the CORE aggregate command (the most complex
piece); it does not prohibit Part 1. Because TreeDiff's tests are authored in THIS task (treediff_test.go),
there is no external aggregate-only test to conflict with.

---

## §5. Exit-code convention — simple branch form, NO 128 special-case

TreeDiff's `git diff <treeA> <treeB>` (and its `--name-only` / `--name-status` / per-file sub-commands)
exit 0 on success and 128 only on a **bad/unresolvable tree SHA** (verified §1). A bad SHA is a real
caller error → return a wrapped error. There is **no "unborn is not an error" concept** here:

- TreeDiff is NOT unborn-aware. The caller (decompose orchestrator, P3) resolves the base tree via
  `RevParseTree` (S1), which returns `("", nil)` on unborn; the orchestrator converts that `""` to the
  **empty-tree SHA** (`4b825dc642cb6eb9a060e54bf8d69288fbee4904`) before calling TreeDiff. So TreeDiff
  always receives two VALID tree SHAs.
- Therefore TreeDiff uses the **simple branch form** (`if code != 0 { return "", err }`) — identical to
  `DiffTree`, `StagedFileCount`, and StagedDiff's own Part-1/Part-2 sub-commands. It does NOT mirror
  RevParseHEAD/RevParseTree's `if code == 128 { return "", nil }` (that is a *read-of-HEAD* convention).

This applies to EVERY git invocation inside TreeDiff: the markdown list, each per-file markdown diff,
the numstat (inside detectBinaryFiles), the name-status (inside fileStatuses), and the Part-2 aggregate.
detectBinaryFiles/fileStatuses already branch on `code != 0 → error` internally (verified in binary.go),
so TreeDiff inherits that for free; it only needs its own `code != 0` checks on the three commands it
issues directly (md list, per-file md diff, Part-2 aggregate).

---

## §6. The empty-tree base case + the `EmptyTreeSHA` constant

PRD §13.6.3 base case: "`tree[-1]` is the original parent tree (`git rev-parse HEAD^{tree}`, or the
**empty tree** for an unborn repo)". For concept[0] on an unborn repo, the orchestrator diffs
`TreeDiff(ctx, EMPTY_TREE_SHA, tree[0], opts)`. Verified (§1) that `4b825dc642cb6eb9a060e54bf8d69288fbee4904`
is a valid `git diff` tree arg (empty→B lists every file as `A`; empty→empty is exit 0 + empty stdout).

**The orchestrator (P3) owns the `"" → EMPTY_TREE_SHA` conversion**, NOT TreeDiff — TreeDiff treats both
args as opaque tree SHAs. To let the orchestrator avoid a magic-string literal, **export a constant
`EmptyTreeSHA`** from the `git` package in this task (a one-line `const` near `defaultExcludes`). It is
low-risk, directly supports the contract's "use the well-known empty tree SHA" guidance, and is consumed
by exactly one future caller (P3's `tree[-1]` resolution). This is the ONLY addition beyond the method
itself + its tests.

---

## §7. Test fixtures (reuse — do NOT redefine) + the tree-construction idiom

All helpers are package-level in `internal/git/*_test.go` (same package `git`):

| Helper | Defined in | Signature | Use |
|---|---|---|---|
| `initRepo(t, dir)` | git_test.go | `func initRepo(t *testing.T, dir string)` | `git init` + repo-local identity. EVERY test starts here. |
| `writeFile(t, dir, name, body)` | committree_test.go | creates `dir/name` (0644) | write a file |
| `stageFile(t, dir, name)` | committree_test.go | `git -C dir add name` | stage it |
| `writeTreeOf(t, dir)` | committree_test.go | `git -C dir write-tree` → trimmed SHA | **the tree-construction oracle + the way tests BUILD tree SHAs** |
| `headSHA(t, dir)` | committree_test.go | `git rev-parse HEAD` | (not needed here) |
| `setIdentityConfig(t, dir)` | committree_test.go | repo-local user.name/email | only if a test commits |
| `makeEmptyCommit(t, dir, msg)` | revparse_test.go | `git commit --allow-empty -m msg` | establish a born HEAD (not needed for tree-only tests) |
| `sdManyLines(n)` | stagediff_test.go | n lines of "line i\n" | build a big markdown file for line-cap tests |
| `asRunner(g)` | binary_test.go | `g.(*gitRunner)` | (TreeDiff is on the interface → use `New(repo)` directly, no unwrap needed) |

**The tree-construction idiom (the key test technique):** `writeTreeOf` captures the CURRENT INDEX into a
tree. So to build two distinct trees WITHOUT commits:
```
initRepo(t, repo)
writeFile(t, repo, "a.go", "package main\n");  stageFile(t, repo, "a.go")   // index = {a.go}
treeA := writeTreeOf(t, repo)                                                 // snapshot
writeFile(t, repo, "b.go", "package lib\n");  stageFile(t, repo, "b.go")     // index = {a.go, b.go}
treeB := writeTreeOf(t, repo)                                                 // snapshot
// TreeDiff(ctx, treeA, treeB, opts) → shows b.go added, a.go unchanged (absent)
```
This mirrors `writeTreeOf`'s use in writetree_test.go and is the cleanest way to fabricate the two
positional tree SHAs TreeDiff needs. **No commits required** — `git write-tree` works on any index,
born or unborn. (Confirmed empirically: every tree SHA in §1 was built this way.)

**Independent-oracle idiom** (from addall_test.go / stagediff_test.go): assert a method's OUTPUT by
string-matching the returned payload, NOT by re-calling the method under test. For TreeDiff, assert
`strings.Contains(out, "A\t[binary] logo.png")` and `!strings.Contains(out, "Binary files")` — exactly
the stagediff_test.go binary-test idiom.

---

## §8. Parallel-work merge consideration (S1 runs concurrently)

S1 (RevParseTree + ReadTree) and S2 (TreeDiff) BOTH modify `internal/git/git.go`: each appends a method
to the `Git` interface block AND a `(*gitRunner)` method body. To minimize merge friction:

- **Append TreeDiff at the END** of the `Git` interface block (after `StagedFileCount`) and at the END of
  the file (after `StagedFileCount`'s body). S1 likewise appends at the end. Both appending new lines at
  the tail → a 3-way merge resolves cleanly in the common case; if a conflict marker appears at the
  closing interface brace, the resolver keeps BOTH additions (they are independent lines).
- **Create `treediff_test.go` as a NEW file** (distinct name from S1's `revparsetree_test.go` /
  `readtree_test.go`) → no file-level conflict.
- **Do NOT edit the `// Method ownership` comment block** (it is a v1 provenance map; S1 may also touch
  nearby lines). The new doc comment on `TreeDiff` is self-documenting.
- **Do NOT redefine any test helper** — they are shared package-level symbols; redefining = compile error.

---

## §9. go.mod / go.sum — UNCHANGED

TreeDiff uses only stdlib already imported in git.go (`context`, `fmt`, `sort`, `strings`) and the
existing helpers. The test file reuses existing imports (`context`, `errors`, `strings`, `testing` +
the package's own helpers). **No new dependencies.** `git diff --exit-code go.mod go.sum` must be empty.

---

## §10. Validation gates (verified present in the repo)

- `go build ./...` (Makefile `build`)
- `go test -race ./...` (Makefile `test`)
- `go vet ./...`
- `golangci-lint run` (config: `.golangci.yml` — errcheck/gosimple/govet/ineffassign/staticcheck/unused)
- `gofmt -l internal/ pkg/` (must be empty)

TreeDiff is on the `Git` interface → it must be implemented on `*gitRunner` (the only implementor;
verified: `New()` returns `*gitRunner`, no other `Git` implementors exist). Adding a method is
backward-compatible; existing callers/tests are unaffected (no caller references TreeDiff yet — the
decompose pipeline is P3).

---

## TL;DR implementation

1. **Interface** (git.go): append `TreeDiff(ctx, treeA, treeB string, opts StagedDiffOptions) (string, error)`
   with a doc comment (command, §13.6.3 invariant-2 role, exit-128=bad-SHA-error convention, FR3c binary
   filtering).
2. **Constant** (git.go): append `const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"` near
   `defaultExcludes`.
3. **Method** (git.go): copy `(*gitRunner).StagedDiff`'s body; swap every `--cached` in the `git diff`
   arg slices for `treeA, treeB`; swap `detectBinaryFiles(ctx, "--cached")` / `fileStatuses(ctx, "--cached")`
   for the `(ctx, treeA, treeB)` forms; keep caps, excludes, defaultExcludes-fallback, sort, placeholders,
   byte cap, line cap, error wrapping byte-identical. Simple `code != 0 → error` branches (no 128 case).
4. **Tests** (treediff_test.go): basic concept diff, empty-tree base, no-changes→"", binary placeholder
   + excluded body, binary extension override, text companion kept, excludes applied, markdown not
   double-counted, byte cap, markdown line cap, bad-SHA→error, git-missing→"git binary not found",
   context-cancelled→`context.Canceled`.
5. **Gates**: build + test -race + vet + golangci-lint + gofmt green; go.mod/go.sum unchanged.
