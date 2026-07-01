---
name: "P2.M2.T1.S2 — Implement TreeDiff (tree-to-tree concept diff with binary filtering) (PRD §13.6.3 invariant 2, FR3c)"
description: |

  ADD one method to the `Git` interface (`internal/git/git.go`) AND implement it on `*gitRunner`,
  consuming the existing `run()` helper and the already-variadic binary-filtering primitives
  (`detectBinaryFiles` / `fileStatuses` / `isBinaryByExtension` / `binaryPlaceholderLine` in
  `internal/git/binary.go`):

    TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error)

  TreeDiff is the **per-concept concept diff** for multi-commit decomposition (PRD §13.6.3 invariant 2:
  "the concept diff is computed tree-to-tree, never index-vs-HEAD. message[i] reasons over
  `git diff tree[i-1] tree[i]`"). It is what makes `stager[i+1] ∥ message[i]` safe — the diff is
  independent of the live index and HEAD (invariant 2's whole point). It is consumed by the per-concept
  message-generation step (P3.M2.T4.S1).

  CONTRACT (P2.M2.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: the concept diff for message[i] is `git diff tree[i-1] tree[i]` — a TREE-TO-TREE
       diff, never index-vs-HEAD (§13.6.3 invariant 2). The diff must apply the SAME caps/excludes/
       binary-filtering as StagedDiff. The existing StagedDiff uses --cached (index-vs-HEAD); TreeDiff
       uses two positional tree SHAs (no --cached). For binary files in the tree diff, emit placeholders
       per FR3c (same format as StagedDiff integration).
    2. INPUT: RevParseTree from S1 (provides tree[-1]); binary detection from P2.M1.T1.S1 (variadic).
    3. LOGIC: `git diff <treeA> <treeB>` with pathspec excludes (same pattern as StagedDiff Part 2 but
       WITHOUT --cached). Apply binary detection (detectBinaryFiles with the tree-diff args), emit
       placeholders, exclude binary paths from the diff body. Apply byte cap. For the empty tree
       (unborn base), use the well-known empty tree SHA (4b825dc642cb6eb9a060e54bf8d69288fbee4904) or
       pass as-is (git handles it). StagedDiffOptions gets a BinaryExtensions field if not already added.
    4. OUTPUT: TreeDiff returns a concept diff payload between two tree SHAs, with binary filtering
       applied. Consumed by the decompose message-generation step (P3.M2.T4.S1).
    5. DOCS: none — internal method, documented by interface doc comment.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/git/binary.go` + `binary_test.go` — P2.M1 (binary filtering). CONSUMED, not modified.
      Its helpers are ALREADY variadic and ALREADY name `TreeDiff: diffArgs = [treeA, treeB]` in the
      package doc (binary.go lines 19–20, 81). TreeDiff only CALLS them.
    - `run()` / `runWithInput()` in git.go — CONSUMED, not modified.
    - `StagedDiffOptions` struct — ALREADY has `BinaryExtensions` (added in P2.M1.T1.S1). DO NOT re-add
      the field (it would be a duplicate-declaration compile error). TreeDiff reads `opts.BinaryExtensions`.
    - RevParseTree / ReadTree (S1, parallel) — sibling work item. Do NOT implement them. Both S1 and S2
      append to `internal/git/git.go`; append TreeDiff at the END of the interface + file (see Gotchas).
    - StatusPorcelain (S-sibling), WorkingTreeDiff (S-sibling) — do NOT implement.
    - Decompose wiring (P3.x) — no caller references TreeDiff yet. This task only adds + tests the method.
    - go.mod / go.sum — UNCHANGED (stdlib only: context/fmt/sort/strings already imported).

  DELIVERABLES (1 file MODIFIED, 1 new file):
    MODIFY internal/git/git.go       — (a) append `TreeDiff` to the `Git` INTERFACE (with doc comment);
                                      (b) append the `(*gitRunner).TreeDiff` method body (a near-mechanical
                                      port of StagedDiff: every `--cached` arg swapped for the two
                                      positional tree SHAs); (c) append the exported `EmptyTreeSHA` constant.
    CREATE internal/git/treediff_test.go — 13 tests covering: basic concept diff, empty-tree base
                                      (concept[0] on unborn repo), no-changes→"", binary placeholder +
                                      excluded body, binary-extension user override, text companion kept,
                                      pathspec excludes applied, markdown not double-counted, byte cap,
                                      markdown line cap, bad-tree-SHA→error, git-binary-missing, context-cancelled.

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (the new
  interface method is additive — no existing caller breaks); `git diff <A> <B> --numstat` emits
  `-\t-\t<path>` for binaries (detectBinaryFiles works over tree args); `git diff <A> <B> --name-status`
  emits status letters for binaries (fileStatuses works over tree args); a tree diffed against itself
  returns ("", nil); a bad SHA returns a wrapped error; binary files appear as
  `"<status>\t[binary] <path>"` placeholders with NO "Binary files … differ" hunk body.

---

## Goal

**Feature Goal**: Implement the per-concept tree-to-tree concept diff at the `Git` interface boundary,
by porting the proven `StagedDiff` payload structure (markdown per-file + non-markdown aggregate, with
caps, pathspec excludes, and binary filtering) to the two-positional-tree-SHA `git diff` form. This is
the diff the decompose pipeline's message agent reasons over for each concept (PRD §13.6.3 invariant 2),
and the input whose tree-to-tree (not index-vs-HEAD) nature is what makes overlapped staging safe.

**Deliverable** (1 file MODIFIED, 1 new):
1. `internal/git/git.go` — (a) one new method appended to the `Git` interface with a doc comment naming
   its command (`git diff <treeA> <treeB>`), its §13.6.3 invariant-2 role, its exit-code convention
   (128 = bad SHA = real error; no unborn special-case), and its FR3c binary filtering; (b) the
   `(*gitRunner).TreeDiff` implementation, a near-mechanical port of `StagedDiff` in which every
   `--cached` occurrence in the `git diff` arg slices is replaced by the two positional tree SHAs
   `treeA`, `treeB`, and `detectBinaryFiles(ctx, "--cached")` / `fileStatuses(ctx, "--cached")` become
   `detectBinaryFiles(ctx, treeA, treeB)` / `fileStatuses(ctx, treeA, treeB)`; (c) the exported
   `EmptyTreeSHA` constant.
2. `internal/git/treediff_test.go` — 13 tests (basic concept diff; empty-tree base; no-changes→"";
   binary placeholder + excluded body; binary-extension override; text companion kept; excludes applied;
   markdown not double-counted; byte cap; markdown line cap; bad-SHA→error; git-binary-missing;
   context-cancelled), reusing the existing package-level test helpers (`initRepo`, `writeFile`,
   `stageFile`, `writeTreeOf`, `setIdentityConfig`, `sdManyLines`).

**Success Definition**:
- On a repo whose index held `{a.go}` then grew to `{a.go, b.go}`, with `treeA = write-tree` over the
  first index and `treeB = write-tree` over the second, `TreeDiff(ctx, treeA, treeB, StagedDiffOptions{})`
  returns a payload containing `b.go` and NOT a "Binary files" hunk (it is a text file).
- `TreeDiff(ctx, treeA, treeB2, opts)` where treeB2 adds a real binary (PNG header) returns a payload
  containing `A\t[binary] <path>` and NOT `Binary files` — the FR3c placeholder replaces the useless hunk.
- `TreeDiff(ctx, EmptyTreeSHA, treeB, opts)` (concept[0] on an unborn-repo base) returns a payload listing
  every file in treeB as added — proving the well-known empty-tree SHA is a valid `git diff` arg.
- `TreeDiff(ctx, treeA, treeA, opts)` (a tree diffed against itself) returns `("", nil)` — NOT an error
  (exit 0, empty stdout; mirrors StagedDiff's nothing-staged non-error).
- `TreeDiff(ctx, treeA, "000…000", opts)` returns a non-nil error (exit 128 = bad SHA).
- `TreeDiff(ctx, treeA, treeB, StagedDiffOptions{BinaryExtensions: []string{"dat"}})` treats a `.dat` file
  as binary (extension signal), emitting its placeholder — proving `opts.BinaryExtensions` is honored.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` shows EXACTLY 1 modified
  (`internal/git/git.go`) + 1 new untracked test file — nothing else (binary.go/run()/StagedDiffOptions
  unchanged).

## User Persona

**Target User**: the decompose pipeline's per-concept message-generation step (internal code, P3.M2.T4.S1),
and by extension the end user running `stagehand` on an un-staged working tree to get multiple
logically-coherent commits. TreeDiff is NOT a user-facing CLI flag; it is the diff primitive the message
agent consumes inside the decomposition loop.

**Use Case**: after stager[i] returns and the orchestrator snapshots `tree[i] = write-tree`, the message
agent for concept *i* is invoked with `TreeDiff(ctx, tree[i-1], tree[i], opts)` as its diff payload —
exactly what stager[i] added on top of tree[i-1], independent of where HEAD points or what stager[i+1] is
concurrently staging (§13.6.3 invariant 2). For concept[0] on an unborn repo, tree[i-1] is
`EmptyTreeSHA`. The generated message becomes `msg[i]` for commit-tree.

**Pain Points Addressed**: the decompose pipeline's message agent needs a diff that (a) is computed
tree-to-tree (never index-vs-HEAD, per invariant 2 — otherwise concurrent staging would corrupt it),
(b) applies the same binary filtering / caps / excludes as the single-commit path (FR3c: identical
placeholder format in every diff path), and (c) reuses the exact payload format the message-agent prompt
was designed around (so the same prompt works on both paths). TreeDiff provides all three by porting
StagedDiff.

## Why

- **Closes PRD §13.6.3 invariant 2 + FR3c at the plumbing layer.** Invariant 2 mandates tree-to-tree
  concept diffs (`git diff tree[i-1] tree[i]`, "never index-vs-HEAD") — the section names this exact
  command. FR3c mandates identical binary filtering in "every diff path", explicitly including "the
  per-concept tree-to-tree concept diff (§13.6.3)". This task is the literal implementation of both.
- **Reuses the proven StagedDiff structure + the already-variadic binary primitives — zero new logic.**
  TreeDiff is StagedDiff with `--cached` → `treeA, treeB`. The binary helpers in binary.go were authored
  (P2.M1.T1.S1) with `TreeDiff: diffArgs = [treeA, treeB]` named in their package doc — they are READY
  for this exact call. No new shell surface, no new exec, no new buffer handling, no new binary detection.
- **Lowest-risk, maximal-reuse port.** A near-mechanical copy of StagedDiff (one token swap per arg slice)
  guarantees the run() contract, the cap resolution, the defaultExcludes fallback, the rename
  reconciliation, the placeholder format, and the error wrapping are byte-consistent with the shipped,
  tested single-commit path. Selectively extracting "only Part 2" would be MORE divergence for no benefit.
- **Message-agent prompt consistency.** Because TreeDiff produces the SAME two-part payload format as
  StagedDiff (markdown per-file + non-markdown aggregate), the message-agent prompt (P1.M3.T1, §17.1) and
  parsing (§9.6) work identically on single-commit and multi-commit paths — no separate tree-diff prompt.
- **Low-risk, additive, backward-compatible.** The interface GAINS one method (Go interfaces are open for
  extension — no existing implementation or caller breaks). No existing file's behavior changes. go.mod/
  go.sum untouched. Unblocks P3.M2.T4.S1 (per-concept message gen).

## What

One new method on the `Git` interface (`internal/git/git.go`), implemented on `*gitRunner`, delegating to
`run()` and the variadic binary helpers. No new types (reuses `StagedDiffOptions`). One new exported
constant (`EmptyTreeSHA`). One new test file. No new dependencies. No caller wiring (that is P3.x). The
structural edits are: append one interface method + doc comment, append one `(*gitRunner)` method body,
append one `const`, and add the new test file.

### Success Criteria

- [ ] `TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error)` is
      declared on the `Git` interface with a doc comment naming: the command (`git diff <treeA> <treeB>`);
      the two positional tree SHAs as the domain (NOT `--cached` — this is the §13.6.3 invariant-2
      tree-to-tree form); the §13.6.3 role (per-concept concept diff for the message agent); the exit-128
      = bad-SHA = real error convention (NO unborn special-case — the caller resolves trees via
      RevParseTree and converts the unborn base to `EmptyTreeSHA`); and the FR3c binary filtering
      (placeholders via detectBinaryFiles/fileStatuses).
- [ ] `const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"` is exported near `defaultExcludes`
      in git.go (the well-known empty-tree object name; consumed by P3's tree[-1] resolution for unborn repos).
- [ ] `(*gitRunner).TreeDiff` is a port of `(*gitRunner).StagedDiff` in which: (a) the Part-1 markdown
      list is `["diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown"]`; (b) each Part-1 per-file
      diff is `["diff", treeA, treeB, "--", file]`; (c) binary detection is `g.detectBinaryFiles(ctx,
      treeA, treeB)`; (d) file statuses are `g.fileStatuses(ctx, treeA, treeB)`; (e) the Part-2 aggregate
      is `["diff", treeA, treeB, "--", excludes..., ":!*.md", ":!*.markdown", binExcludes...]`. ALL other
      logic (cap resolution, defaultExcludes fallback, sort, placeholder emission, line/byte caps,
      error wrapping) is byte-identical to StagedDiff.
- [ ] Every direct `git diff` invocation inside TreeDiff uses the SIMPLE branch form
      (`if code != 0 { return "", fmt.Errorf(...) }`) — NO `code == 128` special-case (a 128 here is a
      bad/unresolvable tree SHA = a real caller error).
- [ ] `treediff_test.go` proves: basic concept diff (b.go added, a.go absent); empty-tree base (every file
      as added); no-changes (`TreeDiff(A,A)` → `("", nil)`); binary placeholder present + "Binary files"
      hunk absent; binary-extension user override (`.dat`); text companion present; pathspec excludes
      applied; markdown not double-counted; byte cap sentinel; markdown line-cap sentinel; bad-SHA →
      non-nil error; git-binary-missing → `"git binary not found"`; context-cancelled →
      `errors.Is(err, context.Canceled)`.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified + 1 new file.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the empirical ground truth
(findings §1 — `git diff <A> <B>` exit codes + numstat/name-status/pathspec/empty-tree behaviors); the
fact the binary helpers are ALREADY variadic and named for TreeDiff (findings §2 + binary.go lines 19–20);
the exact StagedDiff body to port (findings §3 — the arg-slice swap table); the markdown-per-file decision
(findings §4 — full replication, with the decisive tiebreaker: the signature reuses StagedDiffOptions so
BOTH caps must be honored); the exit-code convention (findings §5 — simple branch, no 128 case); the
empty-tree base + `EmptyTreeSHA` constant (findings §6); the reusable test fixtures + the
tree-construction idiom via `writeTreeOf` (findings §7); and the parallel-S1 merge consideration
(findings §8). No prompt/decompose/registry knowledge required — the contract is fully self-contained at
the git-plumbing layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verified git diff treeA treeB behaviors + the port plan)
- docfile: plan/002_a17bb6c8dc1d/P2M2T1S2/research/findings.md
  why: §1 the verified git diff <A> <B> behaviors (exit 0 on success incl. no-changes; exit 128 on bad SHA;
       numstat emits -\t-\t for binaries over tree args; name-status emits status letters; pathspecs go
       AFTER the two positional trees; empty-tree SHA is a valid arg); §2 the binary helpers are ALREADY
       variadic + name TreeDiff (binary.go lines 19–20) — CONSUMED, not modified; §3 the EXACT StagedDiff
       arg-slice swap table (the port template); §4 the markdown-per-file decision (full replication —
       StagedDiffOptions reuse demands both caps honored); §5 exit-code convention (simple branch, NO 128);
       §6 empty-tree base + EmptyTreeSHA; §7 reusable test fixtures + the writeTreeOf tree-construction
       idiom; §8 parallel-S1 merge consideration.
  critical: §1 (exit 0 on no-changes → ("", nil), NOT error; exit 128 = bad SHA → error; the SIMPLE branch
            form, NOT RevParseHEAD's 128-as-non-error); §3 (the arg-slice swap is the WHOLE implementation);
            §4 (do NOT drop Part 1 — MaxMDLines would be silently ignored; replicate the full StagedDiff
            structure).

# MUST READ — the FILE TO MODIFY: the Git interface + StagedDiff (the port template) + run()
- file: internal/git/git.go
  section: the `Git` interface (append TreeDiff at the END, after StagedFileCount); the `StagedDiffOptions`
           struct (NOTE: `BinaryExtensions` is ALREADY present — do NOT re-add it); `(*gitRunner).StagedDiff`
           (the EXACT method to copy and port — Part 1 markdown loop + binary filtering + Part 2 aggregate);
           `(*gitRunner).run` (the helper TreeDiff calls — its INVARIANT: non-zero git exit →
           (stdout, stderr, exitCode, nil), err nil); `defaultExcludes` (place EmptyTreeSHA near it).
  why: StagedDiff is the single best template for TreeDiff (same two-part structure, same caps, same
       excludes, same binary filtering — only the diff domain differs). Copying its body and swapping the
       diff args is the INTENDED authoring pattern.
  pattern: copy `(*gitRunner).StagedDiff`'s body LITERALLY; in EVERY `git diff` arg slice, replace the
           `"--cached"` element with `treeA, treeB` (two positional elements); replace
           `g.detectBinaryFiles(ctx, "--cached")` with `g.detectBinaryFiles(ctx, treeA, treeB)` and
           `g.fileStatuses(ctx, "--cached")` with `g.fileStatuses(ctx, treeA, treeB)`. Keep EVERYTHING
           ELSE byte-identical (caps, defaultExcludes fallback, sort, placeholders, byte/line cap sentinels,
           error wrapping, the SEPARATE nmArgs slice for md+binary excludes).
  gotcha: do NOT re-add `BinaryExtensions` to `StagedDiffOptions` (duplicate-declaration compile error —
          it is already there). Do NOT add a 128 special-case (TreeDiff is not unborn-aware). Do NOT mutate
          `defaultExcludes` (append md/binary excludes to a FRESH nmArgs slice, not to `excludes`).

# MUST READ — the CONSUMED binary primitives (already variadic, already name TreeDiff)
- file: internal/git/binary.go
  section: the package doc (lines 19–20: "TreeDiff: diffArgs = [treeA, treeB]"); `detectBinaryFiles`
           (variadic `diffArgs ...string` → call with `g.detectBinaryFiles(ctx, treeA, treeB)`);
           `fileStatuses` (variadic → `g.fileStatuses(ctx, treeA, treeB)`); `isBinaryByExtension`;
           `binaryPlaceholderLine`.
  why: confirms these helpers are READY for TreeDiff with NO modification — they build
       `["diff", treeA, treeB, "--numstat"]` / `["diff", treeA, treeB, "--name-status"]` internally and
       already branch on `code != 0 → error`. TreeDiff only CALLS them.
  gotcha: do NOT edit binary.go (scope boundary). The rename reconciliation (numstat `=> ` key vs
          fileStatuses destination key) is already handled by StagedDiff's iteration-over-statuses block,
          which TreeDiff copies verbatim.

# MUST READ — the test exemplars (the patterns to mirror in treediff_test.go)
- file: internal/git/stagediff_test.go   (READ — the closest test template)
  why: the EXACT shape TreeDiff tests copy: TestStagedDiff_BinaryFilePlaceholderAndExcluded (real PNG →
       `A\t[binary] logo.png` present, `Binary files` absent, text companion present); TestStagedDiff_
       BinaryExtensionsUserOverride (text in `.dat` → not binary without override, binary WITH override);
       TestStagedDiff_MarkdownNotDoubleCounted (exactly 1 hunk); TestStagedDiff_NonMarkdownByteCap /
       _MarkdownLineCap (sentinel assertions); TestStagedDiff_GitBinaryMissing / _ContextCancelled
       (the two cross-cutting error tests); TestStagedDiff_NothingStaged → `out == ""`.
  pattern: mirror these tests but build TWO tree SHAs (treeA, treeB) via writeTreeOf over two distinct
           indices, then call TreeDiff(ctx, treeA, treeB, opts) instead of StagedDiff(ctx, opts). For the
           empty-tree base, treeA = git.EmptyTreeSHA. For no-changes, treeA == treeB.
- file: internal/git/committree_test.go   (READ — fixture definitions to REUSE)
  why: defines the package-level helpers `writeFile`, `stageFile`, `writeTreeOf`, `setIdentityConfig` —
       ALL reusable from treediff_test.go (same package git). `writeTreeOf(t, repo)` is BOTH the
       independent tree oracle AND the way tests FABRICATE the two positional tree SHAs TreeDiff needs.
  gotcha: these helpers are ALREADY defined — do NOT redefine them (duplicate-symbol compile error).
- file: internal/git/revparse_test.go   (READ — makeEmptyCommit helper, only if a test commits)
  why: defines `makeEmptyCommit(t, dir, msg)`. Most TreeDiff tests need NO commits (write-tree works on
       any index); use makeEmptyCommit only if a test needs a born HEAD.
- file: internal/git/git_test.go   (READ — initRepo)
  why: defines `initRepo(t, dir)` — EVERY temp-repo test starts here (git init + repo-local identity).

# MUST READ — the design reference (signature + role)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "## Required New Git Methods" — lists `TreeDiff(ctx, treeA, treeB string, opts StagedDiffOptions)
           (string, error)` verbatim with its role ("Returns the diff between two tree SHAs (concept diff).
           Uses: git diff <treeA> <treeB> with binary filtering applied.").
  why: confirms the exact signature, the StagedDiffOptions reuse, and the concept-diff role. NOTE: the doc
       lists 5 new methods; THIS task implements ONLY TreeDiff. The other 4 (RevParseTree/ReadTree =
       sibling S1; StatusPorcelain/WorkingTreeDiff = sibling) are separate work items — do NOT implement them.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.3 (invariant 2 + base case) and §9.1 FR3c (binary filtering in every diff path)
  why: §13.6.3 invariant 2 mandates tree-to-tree concept diffs (`git diff tree[i-1] tree[i]`) and names
       the empty-tree base case for unborn repos; FR3c mandates identical binary filtering (same
       placeholder format) in "every diff path", explicitly including "the per-concept tree-to-tree
       concept diff (§13.6.3)". This task implements both at the interface boundary.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # MODIFY: append TreeDiff to the Git interface (doc comment) AND implement on
                      #   *gitRunner (port of StagedDiff). Append EmptyTreeSHA const. run()/StagedDiff/
                      #   StagedDiffOptions UNCHANGED (consumed; BinaryExtensions already present).
  binary.go           # UNCHANGED (P2.M1; CONSUMED — detectBinaryFiles/fileStatuses are already variadic
                      #   and already name TreeDiff in the package doc, lines 19–20).
  binary_test.go      # UNCHANGED (asRunner helper + binary detection tests).
  stagediff_test.go   # READ: the test template + sdManyLines helper (reuse, do not redefine).
  committree_test.go  # READ: writeFile/stageFile/writeTreeOf/setIdentityConfig helpers (reuse).
  revparse_test.go    # READ: makeEmptyCommit helper (reuse, only if a test commits).
  git_test.go         # READ: initRepo helper (reuse in every test).
  (*_test.go)         # READ: other per-method test files (the one-file-per-method convention).
go.mod / go.sum       # UNCHANGED (stdlib only: context/fmt/sort/strings already imported in git.go).
.golangci.yml         # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added/modified

```bash
internal/git/git.go              # MODIFY — append to the `Git` interface (at the END, after
                                 #   StagedFileCount):
                                 #     // TreeDiff returns the concept diff between two tree SHAs via
                                 #     // `git diff <treeA> <treeB>` — the per-concept tree-to-tree diff
                                 #     // (PRD §13.6.3 invariant 2). Applies the SAME caps/excludes/binary
                                 #     // filtering as StagedDiff (FR3c). exit 128 = bad SHA = error.
                                 #     TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (diff string, err error)
                                 #   AND append the EmptyTreeSHA const near defaultExcludes:
                                 #     // EmptyTreeSHA is git's well-known empty-tree object name — a valid
                                 #     // `git diff` tree arg used as tree[-1] for the unborn-repo base case
                                 #     // (PRD §13.6.3). The decompose orchestrator (P3) passes it as treeA
                                 #     // when RevParseTree returns "" on an unborn repo.
                                 #     const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
                                 #   AND append the (*gitRunner).TreeDiff method body (port of StagedDiff:
                                 #   swap --cached → treeA, treeB in every git diff arg slice; swap
                                 #   detectBinaryFiles/fileStatuses to the (ctx, treeA, treeB) forms).
                                 #   NO other change to git.go.
internal/git/treediff_test.go    # NEW — 13 tests (basic concept diff, empty-tree base, no-changes,
                                 #   binary placeholder+excluded, binary-extension override, text companion,
                                 #   excludes applied, markdown not double-counted, byte cap, markdown line
                                 #   cap, bad-SHA, git-binary-missing, context-cancelled).
# go.mod/go.sum UNCHANGED. binary.go/binary_test.go UNCHANGED. run()/StagedDiff/StagedDiffOptions UNCHANGED.
# 0 other edits.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the arg-slice swap IS the implementation — findings §3): TreeDiff is StagedDiff with --cached
// replaced by the two positional tree SHAs. In EVERY `git diff` arg slice inside the method:
//   StagedDiff:  g.run(ctx, g.workDir, "diff", "--cached", "--name-only", "--", "*.md", "*.markdown")
//   TreeDiff:    g.run(ctx, g.workDir, "diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown")
// (and identically for the per-file md diff and the Part-2 aggregate). The two tree SHAs are POSITIONAL
// args that come BEFORE `--`; pathspec excludes come AFTER `--`. Verified empirically (findings §1).
// WRONG: keeping "--cached" (that is StagedDiff, not TreeDiff). WRONG: putting the tree SHAs after `--`
// (git would treat them as pathspecs).

// CRITICAL (the binary helpers are ALREADY variadic — findings §2): call them with the two tree SHAs as
// the variadic diffArgs, NOT with "--cached":
//   g.detectBinaryFiles(ctx, treeA, treeB)   // builds ["diff", treeA, treeB, "--numstat"] internally
//   g.fileStatuses(ctx, treeA, treeB)        // builds ["diff", treeA, treeB, "--name-status"] internally
// binary.go's package doc (lines 19–20) literally names "TreeDiff: diffArgs = [treeA, treeB]" — these
// helpers were authored for THIS call. Do NOT pass "--cached" (that would recompute the index-vs-HEAD
// diff, defeating the entire tree-to-tree invariant).

// CRITICAL (do NOT re-add BinaryExtensions to StagedDiffOptions — compile error): the struct ALREADY has
//   type StagedDiffOptions struct {
//       MaxDiffBytes int; MaxMDLines int; Excludes []string; BinaryExtensions []string   // ← already here
//   }
// added in P2.M1.T1.S1. TreeDiff reads opts.BinaryExtensions (same as StagedDiff). Re-declaring the field
// = duplicate-field compile error. The contract's "StagedDiffOptions gets a BinaryExtensions field IF NOT
// ALREADY ADDED" — it is ALREADY added; do nothing to the struct.

// CRITICAL (exit-code convention = SIMPLE branch, NO 128 special-case — findings §5): `git diff <A> <B>`
// exits 0 on success (even with NO changes — empty→empty is exit 0) and 128 ONLY on a bad/unresolvable
// tree SHA. A bad SHA is a real caller error → wrapped error. There is NO "unborn is not an error" concept
// here: TreeDiff is NOT unborn-aware (the caller resolves trees via RevParseTree and converts the unborn
// base to EmptyTreeSHA BEFORE calling). So use:
//   if code != 0 { return "", fmt.Errorf("git diff ...: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
// for EVERY direct git diff invocation (md list, per-file md diff, Part-2 aggregate). Do NOT copy
// RevParseHEAD/RevParseTree's `if code == 128 { return "", nil }` — that is an unrelated read-of-HEAD
// convention. A tree diffed against itself returns ("", nil) because exit==0 && stdout=="" (the empty
// payload flows through naturally), NOT because of a 128 branch.

// CRITICAL (replicate the FULL StagedDiff structure — findings §4): include BOTH Part 1 (markdown per-file,
// line-capped) AND Part 2 (non-markdown aggregate, byte-capped). Rationale: the signature reuses
// StagedDiffOptions, which carries BOTH MaxMDLines AND MaxDiffBytes — dropping Part 1 would silently
// ignore MaxMDLines (huge markdown in a concept diff would blow the prompt budget). "Apply the same
// caps/excludes/binary-filtering as StagedDiff" means BOTH caps. Do NOT implement aggregate-only.

// GOTCHA (do NOT mutate defaultExcludes — binary findings §5 / StagedDiff pattern): StagedDiff points
// `excludes` at the package-level `defaultExcludes` WITHOUT copying when opts.Excludes is empty, then
// appends md + binary excludes to a SEPARATE `nmArgs` slice (never to `excludes`). TreeDiff MUST preserve
// this — build a FRESH `nmArgs := []string{"diff", treeA, treeB, "--"}` and append excludes/md/binary to
// IT, never to `excludes`. (Mutating defaultExcludes would corrupt every subsequent StagedDiff/TreeDiff call.)

// GOTCHA (run() returns err==nil for non-zero git exits): run()'s INVARIANT is that a non-zero git exit is
// returned as (stdout, stderr, exitCode, nil) — err is nil, the exit code is the signal. Only
// infrastructural failures (LookPath miss / context cancel / start-I/O) return err != nil (exitCode -1).
// So every git diff call does `if err != nil { return "", err }` FIRST (catches context.Canceled + git-
// binary-missing, propagated UNWRAPPED so errors.Is works), THEN branches on `code`. Copy StagedDiff's
// branches exactly.

// GOTCHA (do NOT modify run(), runWithInput(), binary.go, or StagedDiff): all are CONSUMED. TreeDiff adds
// ONE method + ONE const + ONE test file. Editing any consumed file is a scope violation (and would change
// other methods' behavior or conflict with the parallel S1 work).

// GOTCHA (parallel S1 work — findings §8): S1 (RevParseTree + ReadTree) ALSO appends to git.go's
// interface block + gitRunner methods. Append TreeDiff at the END of the interface (after StagedFileCount)
// and at the END of the file (after StagedFileCount's body), and create treediff_test.go as a NEW file
// (distinct from S1's revparsetree_test.go / readtree_test.go) → minimal merge friction. Do NOT edit the
// `// Method ownership` comment block (a v1 provenance map). If a 3-way merge conflict appears at the
// closing interface brace, keep BOTH additions (independent lines).

// GOTCHA (one-file-per-method test convention): the package uses one test file per Git method. Put TreeDiff
// tests in a NEW `treediff_test.go`. Test function names MUST be distinct (prefix `TestTreeDiff_`). Do NOT
// redefine any package-level helper (initRepo/writeFile/stageFile/writeTreeOf/setIdentityConfig/sdManyLines)
// — redefining = duplicate-symbol compile error.

// GOTCHA (the writeTreeOf tree-construction idiom — findings §7): `writeTreeOf(t, repo)` captures the
// CURRENT INDEX into a tree. To build two distinct trees WITHOUT commits:
//   initRepo(t, repo); writeFile(t, repo, "a.go", ...); stageFile(t, repo, "a.go"); treeA := writeTreeOf(t, repo)
//   writeFile(t, repo, "b.go", ...); stageFile(t, repo, "b.go"); treeB := writeTreeOf(t, repo)
//   // TreeDiff(ctx, treeA, treeB, opts) → b.go added
// No commits required — `git write-tree` works on any index, born or unborn.

// GOTCHA (independent-oracle assertions — findings §7): verify TreeDiff's OUTPUT by string-matching the
// returned payload, NOT by re-calling TreeDiff or re-running git. Assert `strings.Contains(out,
// "A\t[binary] logo.png")` and `!strings.Contains(out, "Binary files")` (the stagediff_test.go idiom).
// The trees themselves ARE built via independent git (writeTreeOf), so the test is not circular.

// GOTCHA (Go interface extension is backward-compatible): adding TreeDiff to the Git interface does NOT
// break existing callers or *gitRunner (the only implementor — verified: New() returns *gitRunner; no
// other Git implementor exists). The method is additive.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. No new OPTIONS structs (TreeDiff reuses the existing `StagedDiffOptions`, which ALREADY has
`MaxDiffBytes`/`MaxMDLines`/`Excludes`/`BinaryExtensions` — do NOT re-declare any field). One new exported
CONSTANT (`EmptyTreeSHA`). The implementation consumes the existing helpers unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   func (g *gitRunner) run(ctx, repo, args ...string) (stdout, stderr string, exitCode int, err error)
// run()'s invariant: non-zero git exit → (stdout, stderr, exitCode, nil); err != nil ⟺ infrastructural
// failure only (LookPath miss / context cancel / start-I/O), with exitCode == -1.
// from internal/git/binary.go (CONSUME — do not modify; already variadic, already name TreeDiff):
//   func (g *gitRunner) detectBinaryFiles(ctx, diffArgs ...string) (map[string]bool, error)
//   func (g *gitRunner) fileStatuses(ctx, diffArgs ...string) (map[string]string, error)
//   func isBinaryByExtension(path string, extraExts []string) bool
//   func binaryPlaceholderLine(status, path string) string
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — append the EmptyTreeSHA constant
  - LOCATE the `defaultExcludes` var (search for `var defaultExcludes`).
  - INSERT immediately ABOVE or BELOW it:
      // EmptyTreeSHA is git's well-known empty-tree object name. It is a valid `git diff` tree arg and
      // is used as tree[-1] (treeA) for the unborn-repo base case of the multi-commit concept-diff loop
      // (PRD §13.6.3: "tree[-1] is the original parent tree, or the empty tree for an unborn repo"). The
      // decompose orchestrator (P3) passes it as treeA when RevParseTree returns "" on an unborn repo.
      const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
  - NAMING: `EmptyTreeSHA` (exported — consumed by the P3 decompose package in a different directory).
  - GOTCHA: this is the canonical empty-tree object name; it is content-independent and stable across all
    git versions (it is the hash of the empty tree). Do NOT compute it; hardcode the literal.
  - PLACEMENT: internal/git/git.go, near `defaultExcludes`.

Task 2: MODIFY internal/git/git.go — append TreeDiff to the `Git` interface
  - LOCATE the END of the `Git` interface (the closing `}` after the `StagedFileCount` method). APPEND
    TreeDiff there (after StagedFileCount) — appending at the end minimizes merge friction with the
    parallel S1 work (which also appends RevParseTree/ReadTree).
  - INSERT, before the closing brace, the method declaration WITH a doc comment (see "Implementation
    Patterns" for the exact text).
  - NAMING: `TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error)`
    (EXACT, from the architecture doc + work item).
  - DOC COMMENT must name: the command (`git diff <treeA> <treeB>`); the two positional tree SHAs as the
    domain (NOT --cached — the §13.6.3 invariant-2 tree-to-tree form, "never index-vs-HEAD"); the §13.6.3
    role (per-concept concept diff the message agent reasons over); the exit-128 = bad-SHA = real error
    convention (NO unborn special-case — the caller resolves trees via RevParseTree and converts the unborn
    base to EmptyTreeSHA); and the FR3c binary filtering (placeholders via detectBinaryFiles/fileStatuses,
    identical format to StagedDiff).
  - GOTCHA: do NOT edit the `// Method ownership` comment block (a v1 provenance map; S1 may touch nearby
    lines). Do NOT re-add BinaryExtensions to StagedDiffOptions (already present).
  - PLACEMENT: internal/git/git.go, inside `type Git interface { … }`, at the END.

Task 3: MODIFY internal/git/git.go — implement (*gitRunner).TreeDiff (PORT of StagedDiff)
  - APPEND a new method `func (g *gitRunner) TreeDiff(ctx context.Context, treeA, treeB string, opts
    StagedDiffOptions) (string, error)` at the END of the file (after StagedFileCount's body) — co-location
    near StagedDiff is nice-to-have but END-of-file is preferred to avoid merge conflict with S1.
  - BODY: COPY `(*gitRunner).StagedDiff`'s body LITERALLY and apply these swaps (see "Implementation
    Patterns" for the exact code):
      (a) Part-1 markdown list: `"diff", "--cached", "--name-only", "--", "*.md", "*.markdown"` →
          `"diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown"`.
      (b) Part-1 per-file diff: `"diff", "--cached", "--", file` → `"diff", treeA, treeB, "--", file`.
      (c) binary detection: `g.detectBinaryFiles(ctx, "--cached")` → `g.detectBinaryFiles(ctx, treeA, treeB)`.
      (d) file statuses: `g.fileStatuses(ctx, "--cached")` → `g.fileStatuses(ctx, treeA, treeB)`.
      (e) Part-2 aggregate base: `nmArgs := []string{"diff", "--cached", "--"}` →
          `nmArgs := []string{"diff", treeA, treeB, "--"}`.
      (f) Error message prefixes: optionally qualify (e.g. "git diff (markdown list)" stays valid; or rename
          to "git diff tree-to-tree (markdown list)"). Keeping StagedDiff's prefixes is acceptable since they
          are diagnostic-only; qualifying them with "tree-to-tree" aids debugging. CHOOSE ONE and be consistent.
  - KEEP byte-identical: cap resolution (`maxMDLines`/`maxDiffBytes` defaults), the `excludes := opts.Excludes;
    if len(excludes) == 0 { excludes = defaultExcludes }` block, the binary-path collection (`binSet[path] ||
    isBinaryByExtension(path, opts.BinaryExtensions)` iterated over `statuses`), `sort.Strings(binPaths)`,
    `binaryPlaceholderLine(statuses[path], path)` emission, the SEPARATE `binExcludes` slice, the md excludes
    (`":!*.md"`, `":!*.markdown"`) appended to `nmArgs` (NOT to `excludes`), the byte cap + sentinel, the
    per-file line cap + sentinel, the error-wrapping shape, and the `if err != nil { return "", err }`
    infrastructural-failure propagation (UNWRAPPED).
  - GOTCHA: every direct `git diff` invocation uses the SIMPLE branch (`if code != 0 { return "", err }`);
    NO `if code == 128` special-case (a 128 here is a bad SHA = real error). A no-changes diff (treeA ==
    treeB) flows through as `("", nil)` naturally (exit 0 + empty stdout).
  - PATTERN: copy StagedDiff's body and apply the 5 swaps above. That IS the implementation.
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method at the END of the file.

Task 4: CREATE internal/git/treediff_test.go — 13 tests for TreeDiff
  - IMPORTS: context, errors, strings, testing (mirror stagediff_test.go). The package is `git` (same
    package — helpers initRepo/writeFile/stageFile/writeTreeOf/setIdentityConfig/sdManyLines are visible).
  - REUSE sdManyLines(n) from stagediff_test.go for the line-cap test (do NOT redefine it).
  - ADD TestTreeDiff_BasicConceptDiff:
      initRepo(t, repo);
      writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "b.go", "package lib\n");  stageFile(t, repo, "b.go"); treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "b.go");      // b.go was added in treeB
      assert !strings.Contains(out, "a.go") || true-as-present;  // a.go unchanged → may or may not appear;
      // the KEY assertion: b.go IS in the diff (the concept added it).
  - ADD TestTreeDiff_EmptyTreeBase:
      initRepo(t, repo);
      writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go");
      writeFile(t, repo, "doc.md", "# x\n"); stageFile(t, repo, "doc.md");
      treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), EmptyTreeSHA, treeB, StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "a.go") && strings.Contains(out, "doc.md");  // both added (empty→B)
  - ADD TestTreeDiff_NoChanges:
      initRepo(t, repo);
      writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go"); treeA := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeA, StagedDiffOptions{});  // tree vs itself
      assert err == nil && out == "";   // exit 0 + empty stdout → ("", nil), NOT an error
  - ADD TestTreeDiff_BinaryPlaceholderAndExcluded:
      initRepo(t, repo);
      writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00"); stageFile(t, repo, "logo.png");
      writeFile(t, repo, "c.go", "package c\n"); stageFile(t, repo, "c.go"); treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "A\t[binary] logo.png");   // FR3c placeholder
      assert !strings.Contains(out, "Binary files");          // no useless hunk body
      assert strings.Contains(out, "c.go");                   // text companion present
  - ADD TestTreeDiff_BinaryExtensionsUserOverride:
      initRepo(t, repo);
      writeFile(t, repo, "x.go", "package main\n"); stageFile(t, repo, "x.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "data.dat", "hello\n"); stageFile(t, repo, "data.dat"); treeB := writeTreeOf(t, repo);
      g := New(repo);
      // WITHOUT override: .dat with text content is NOT binary (not in the 36-entry denylist)
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{});
      assert err == nil && !strings.Contains(out, "[binary] data.dat");
      // WITH override: caught via extension signal
      out, err = g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{BinaryExtensions: []string{"dat"}});
      assert err == nil && strings.Contains(out, "A\t[binary] data.dat");
  - ADD TestTreeDiff_KeepsTextCompanion (the binary+text mix; mirrors stagediff _BinaryKeepsTextCompanion):
      initRepo(t, repo);
      writeFile(t, repo, "seed.go", "package main\n"); stageFile(t, repo, "seed.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00"); stageFile(t, repo, "img.png");
      writeFile(t, repo, "code.go", "package main\nfunc main() {}\n"); stageFile(t, repo, "code.go");
      treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "A\t[binary] img.png") && strings.Contains(out, "code.go");
      assert !strings.Contains(out, "Binary files");
  - ADD TestTreeDiff_ExcludesApplied:
      initRepo(t, repo);
      writeFile(t, repo, "keep.go", "package main\n"); stageFile(t, repo, "keep.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "drop.go", "package drop\n"); stageFile(t, repo, "drop.go");
      treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{Excludes: []string{":!drop.go"}});
      assert err == nil;
      assert strings.Contains(out, "keep.go") && !strings.Contains(out, "drop.go");
  - ADD TestTreeDiff_MarkdownNotDoubleCounted:
      initRepo(t, repo);
      writeFile(t, repo, "seed.go", "package main\n"); stageFile(t, repo, "seed.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "only.md", "# Hi\n\ncontent\n"); stageFile(t, repo, "only.md"); treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{});
      assert err == nil && strings.Contains(out, "only.md");
      assert strings.Count(out, "diff --git a/only.md b/only.md") == 1;   // exactly 1 hunk (no double-count)
  - ADD TestTreeDiff_NonMarkdownByteCap:
      initRepo(t, repo);
      writeFile(t, repo, "seed.go", "package main\n"); stageFile(t, repo, "seed.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000)); stageFile(t, repo, "big.go");
      treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{MaxDiffBytes: 100});
      assert err == nil && strings.Contains(out, "... [diff truncated at 100 bytes]") && len(out) < 200;
  - ADD TestTreeDiff_MarkdownLineCap:
      initRepo(t, repo);
      writeFile(t, repo, "seed.go", "package main\n"); stageFile(t, repo, "seed.go"); treeA := writeTreeOf(t, repo);
      writeFile(t, repo, "big.md", sdManyLines(50)); stageFile(t, repo, "big.md"); treeB := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{MaxMDLines: 10});
      assert err == nil && strings.Contains(out, "... [diff truncated at 10 lines]");
  - ADD TestTreeDiff_BadTreeSHA:
      initRepo(t, repo);
      writeFile(t, repo, "a.go", "package main\n"); stageFile(t, repo, "a.go"); treeA := writeTreeOf(t, repo);
      g := New(repo);
      out, err := g.TreeDiff(context.Background(), treeA, "0000000000000000000000000000000000000000", StagedDiffOptions{});
      assert err != nil;   // exit 128 = bad SHA → wrapped error (NOT ("", nil))
      assert out == "";
  - ADD TestTreeDiff_GitBinaryMissing:
      t.Setenv("PATH", "");  g := New(t.TempDir());
      out, err := g.TreeDiff(context.Background(), EmptyTreeSHA, EmptyTreeSHA, StagedDiffOptions{});
      assert err != nil && strings.Contains(err.Error(), "git binary not found") && out == "";
  - ADD TestTreeDiff_ContextCancelled:
      ctx, cancel := context.WithCancel(context.Background()); cancel();
      g := New(t.TempDir());
      out, err := g.TreeDiff(ctx, EmptyTreeSHA, EmptyTreeSHA, StagedDiffOptions{});
      assert errors.Is(err, context.Canceled) && out == "";
  - PATTERN: copy TestStagedDiff_* (the placeholder/excludes/cap/git-missing/ctx-cancelled idioms) and adapt:
    build treeA + treeB via writeTreeOf over two distinct indices, then call TreeDiff(ctx, treeA, treeB, opts).
  - GOTCHA: every tree SHA used as an arg MUST come from `writeTreeOf` (a real tree) OR `EmptyTreeSHA`. Do
    NOT pass a file path or a commit SHA as a tree arg (TreeDiff runs `git diff <A> <B>`; commit SHAs would
    error or produce unexpected output). `writeTreeOf` returns a TREE SHA, which is exactly what TreeDiff wants.
  - GOTCHA: do NOT redefine initRepo/writeFile/stageFile/writeTreeOf/setIdentityConfig/sdManyLines — they
    are already package-level (redefining = compile error).
  - GOTCHA: TestTreeDiff_NoChanges is the KEY invariant-2-adjacent test — a tree diffed against itself must
    return ("", nil), proving TreeDiff does NOT error on empty diffs (the decompose orchestrator relies on
    this for the FR-M8 "tree[i] == tree[i-1]" skip, which compares SHAs but TreeDiff must also be empty-safe).
  - PLACEMENT: NEW file internal/git/treediff_test.go.

Task 5: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/treediff_test.go`
  - `go build ./...`   (whole module compiles — the interface + impl + const + test file)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/git/ -run "TestTreeDiff" -v`   (all 13 new tests)
  - `go test -race ./internal/git/`   (the WHOLE git package — existing tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; the new interface method is additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + `?? internal/git/treediff_test.go` (2 entries);
    binary.go/binary_test.go/run()/StagedDiff/StagedDiffOptions UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1: the EmptyTreeSHA constant (append near defaultExcludes) ===

// EmptyTreeSHA is git's well-known empty-tree object name. It is a valid `git diff` tree arg and is used
// as tree[-1] (treeA) for the unborn-repo base case of the multi-commit concept-diff loop (PRD §13.6.3:
// "tree[-1] is the original parent tree (git rev-parse HEAD^{tree}), or the empty tree for an unborn
// repo"). The decompose orchestrator (P3) passes it as treeA when RevParseTree returns "" on an unborn
// repo. TreeDiff itself treats both args as opaque tree SHAs and is NOT unborn-aware.
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"


// === Task 2: the interface addition (append at the END of `type Git interface { … }`) ===

	// TreeDiff returns the concept diff between two tree SHAs via `git diff <treeA> <treeB>` — the
	// per-concept tree-to-tree diff the multi-commit message agent reasons over (PRD §13.6.3 invariant 2:
	// "the concept diff is computed tree-to-tree, never index-vs-HEAD; message[i] reasons over
	// `git diff tree[i-1] tree[i]`"). It is the tree-to-tree analogue of StagedDiff (which is index-vs-HEAD):
	// it applies the SAME caps, pathspec excludes, and FR3c binary filtering (identical placeholder format
	// in every diff path), and reuses StagedDiffOptions. For the unborn-repo base case the caller passes
	// EmptyTreeSHA as treeA (TreeDiff itself is NOT unborn-aware — the caller resolves trees via RevParseTree
	// and converts the unborn base to EmptyTreeSHA). A no-change diff (treeA == treeB) returns ("", nil).
	//
	// `git diff` (without --quiet) exits 0 whether or not there are changes; exit 128 means a bad or
	// unresolvable tree SHA, which is a REAL error (NOT an unborn signal — branch on code != 0, never on
	// code == 128). Read-only with respect to refs and the index.
	TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (diff string, err error)


// === Task 3: (*gitRunner).TreeDiff — PORT of StagedDiff (swap --cached → treeA, treeB) ===
// The body is StagedDiff's body with FIVE arg-slice swaps (marked ← SWAP). Everything else is
// byte-identical. See the Gotchas for why the FULL two-part structure is replicated (not aggregate-only).

// TreeDiff returns the concept diff between two tree SHAs (PRD §13.6.3 invariant 2). It is a port of
// StagedDiff: the same two-part payload (markdown per-file, line-capped; non-markdown aggregate,
// byte-capped) with the same pathspec excludes and FR3c binary filtering — the ONLY difference is the
// diff domain (`git diff <treeA> <treeB>` instead of `git diff --cached`). Every `git diff` invocation
// uses the simple exit-code branch (code != 0 → error); exit 128 = a bad/unresolvable tree SHA = a real
// error (NOT an unborn signal — the caller resolves trees and passes EmptyTreeSHA for the unborn base).
func (g *gitRunner) TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ----  (← SWAP: --cached → treeA, treeB)
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown") // ← SWAP
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git diff tree-to-tree (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", treeA, treeB, "--", file) // ← SWAP
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff %s %s -- %s: failed (exit %d): %s", treeA, treeB, file, fcode, strings.TrimSpace(fstderr))
		}
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n')
		}
	}

	// ---- Binary filtering (PRD §9.1 FR3a/b/c, tree-to-tree path) ----  (← SWAP: --cached → treeA, treeB)
	binSet, berr := g.detectBinaryFiles(ctx, treeA, treeB) // ← SWAP (variadic helper, already ready)
	if berr != nil {
		return "", berr
	}
	statuses, serr := g.fileStatuses(ctx, treeA, treeB) // ← SWAP
	if serr != nil {
		return "", serr
	}

	binPaths := make([]string, 0, len(statuses))
	for path := range statuses {
		if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
			binPaths = append(binPaths, path)
		}
	}
	sort.Strings(binPaths)
	var binExcludes []string
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path))
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----  (← SWAP: --cached → treeA, treeB)
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", treeA, treeB, "--"} // ← SWAP
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...)
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff tree-to-tree (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}

// PATTERN NOTE: this is a faithful port of StagedDiff. The five arg-slice swaps (--cached → treeA, treeB
// in the md list / per-file md diff / Part-2 aggregate; detectBinaryFiles/fileStatuses → variadic tree
// form) are the ENTIRE implementation delta. Copying StagedDiff's body guarantees the run() contract, the
// cap resolution, the defaultExcludes-fallback, the rename reconciliation (iterate statuses by destination),
// the SEPARATE nmArgs slice (never mutate defaultExcludes), and the error-wrapping shape are all
// byte-consistent with the shipped single-commit path — which is exactly what FR3c's "identical in every
// diff path" demands.
```

### Integration Points

```yaml
INTERNAL/GIT (internal/git/git.go — MODIFY):
  - Git interface: "+ TreeDiff(ctx, treeA, treeB string, opts StagedDiffOptions) (string, error)", with a
          doc comment, APPENDED at the end of the interface block (minimal diff, avoids S1 merge conflict).
  - constants: "+ const EmptyTreeSHA = \"4b825dc642cb6eb9a060e54bf8d69288fbee4904\"" near defaultExcludes.
  - *gitRunner: "+ func (g *gitRunner) TreeDiff(...)" (port of StagedDiff; 5 arg-slice swaps; simple
          code != 0 branches; no 128 special-case). APPENDED at the end of the file.
  - run()/runWithInput()/StagedDiff()/StagedDiffOptions: UNCHANGED (consumed).

INTERNAL/GIT TESTS (1 NEW file):
  - treediff_test.go: 13 tests. Trees built via writeTreeOf over two distinct indices (no commits needed);
          empty-tree base uses EmptyTreeSHA; no-changes uses treeA==treeA. Reuses initRepo/writeFile/
          stageFile/writeTreeOf/sdManyLines (do NOT redefine).

NOT MODIFIED (scope boundaries):
  - internal/git/binary.go + binary_test.go (P2.M1 — CONSUMED; already variadic, already name TreeDiff).
  - RevParseTree/ReadTree (S1, parallel) — sibling work item; do NOT implement.
  - StatusPorcelain/WorkingTreeDiff — sibling work items; do NOT implement.
  - Decompose wiring (P3.x) — no caller references TreeDiff yet.
  - go.mod/go.sum — UNCHANGED.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating/editing the files — fix before proceeding
gofmt -w internal/git/git.go internal/git/treediff_test.go
go build ./...                 # whole module compiles (interface + impl + const + test)
go vet ./...
golangci-lint run ./...        # errcheck/gosimple/govet/ineffassign/staticcheck/unused

# Expected: Zero errors. The two cross-cutting error tests (git-binary-missing, context-cancelled) must
# NOT trip errcheck (they assert err != nil). If errors exist, READ the output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test TreeDiff in isolation first
go test -race ./internal/git/ -run "TestTreeDiff" -v

# Then the WHOLE git package (existing tests still pass — the new method is additive)
go test -race ./internal/git/

# Full regression
go test ./...

# Expected: All tests pass. The 13 new TreeDiff tests cover: basic concept diff, empty-tree base,
# no-changes→"", binary placeholder+excluded body, binary-extension override, text companion, excludes
# applied, markdown not double-counted, byte cap, markdown line cap, bad-SHA→error, git-missing,
# context-cancelled. If a test fails, debug root cause (most likely an arg-slice swap was missed or the
# 128 special-case was accidentally introduced).
```

### Level 3: Integration Testing (Git Behavior Validation)

```bash
# Manual sanity: confirm the git behaviors TreeDiff relies on (reproduce findings §1)
TMP=$(mktemp -d); cd "$TMP"; git init -q; git config user.email t@t.com; git config user.name t
printf 'package main\n' > a.go; git add a.go; TA=$(git write-tree); echo "treeA=$TA"
printf 'package lib\n' > b.go; git add b.go; TB=$(git write-tree); echo "treeB=$TB"
echo "=== numstat (binary => -/-) over tree args ==="; git diff $TA $TB --numstat
echo "=== name-status over tree args ===";           git diff $TA $TB --name-status
echo "=== empty-tree base ==="; EMPTY=4b825dc642cb6eb9a060e54bf8d69288fbee4904
git diff $EMPTY $TB --name-status; echo "exit=$?"
echo "=== no-changes (tree vs itself) ==="; git diff $TA $TA; echo "exit=$? (expect 0, empty stdout)"
echo "=== bad SHA ==="; git diff $TA 0000000000000000000000000000000000000000 >/dev/null 2>&1; echo "exit=$? (expect 128)"
cd -; rm -rf "$TMP"

# Expected: numstat emits "1\t0\tb.go"; name-status emits "A\tb.go"; empty-tree base lists b.go as A;
# no-changes is exit 0 + empty; bad SHA is exit 128. These confirm the git semantics TreeDiff depends on.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Confirm the FR3c invariant: binary placeholder format is IDENTICAL to StagedDiff
# (run a quick programmatic check that TreeDiff and StagedDiff produce the same "<status>\t[binary] <path>"
# line for the same binary file in their respective domains — optional, covered by treediff_test.go's
# TestTreeDiff_BinaryPlaceholderAndExcluded + stagediff_test.go's TestStagedDiff_BinaryFilePlaceholderAndExcluded).

# Confirm EmptyTreeSHA is the canonical empty-tree hash (content-independent, stable)
git hash-object -t tree /dev/null  # prints 4b825dc642cb6eb9a060e54bf8d69288fbee4904 on every git version

# Expected: the placeholder format matches across both diff paths (FR3c "identical in every diff path"),
# and EmptyTreeSHA equals the well-known constant.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test -race ./...`
- [ ] No linting errors: `golangci-lint run ./...`
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/ pkg/` (empty).

### Feature Validation

- [ ] `TreeDiff(ctx, treeA, treeB, opts)` declared on the `Git` interface with the specified doc comment.
- [ ] `EmptyTreeSHA` constant exported and equals `4b825dc642cb6eb9a060e54bf8d69288fbee4904`.
- [ ] TreeDiff is a faithful port of StagedDiff (5 arg-slice swaps; full two-part structure; binary
      filtering; caps; excludes) — verified by the 13 tests.
- [ ] Tree diff against itself returns `("", nil)` (empty-safe, no error).
- [ ] Bad SHA returns a wrapped error (exit 128 = real error, no 128 special-case).
- [ ] Binary files emit `"<status>\t[binary] <path>"` placeholders with NO "Binary files … differ" hunk.
- [ ] Empty-tree base (`TreeDiff(EmptyTreeSHA, treeB)`) lists every file as added.
- [ ] `opts.BinaryExtensions` honored (`.dat` user override).
- [ ] Error cases handled: git-binary-missing → `"git binary not found"`; context-cancelled →
      `errors.Is(err, context.Canceled)`.

### Code Quality Validation

- [ ] Follows the existing run() / StagedDiff patterns exactly (one-token-swap port).
- [ ] File placement matches the desired codebase tree (git.go MODIFIED; treediff_test.go NEW).
- [ ] Anti-patterns avoided (no 128 special-case; no defaultExcludes mutation; no StagedDiffOptions
      re-declaration; no helper redefinition).
- [ ] No new dependencies (go.mod/go.sum byte-identical).

### Documentation & Scope

- [ ] Interface doc comment names command, §13.6.3 role, exit convention, FR3c filtering.
- [ ] EmptyTreeSHA constant documented (unborn-repo base case, consumed by P3).
- [ ] Scope respected: binary.go/run()/StagedDiff/StagedDiffOptions unchanged; RevParseTree/ReadTree/
      StatusPorcelain/WorkingTreeDiff NOT implemented (siblings); no P3 caller wiring.

---

## Anti-Patterns to Avoid

- ❌ Don't keep `--cached` in the TreeDiff arg slices (that is StagedDiff, not TreeDiff — invariant 2
  demands tree-to-tree, never index-vs-HEAD).
- ❌ Don't pass `--cached` to `detectBinaryFiles`/`fileStatuses` (they are variadic — pass the two tree
  SHAs; `--cached` would recompute the index-vs-HEAD diff, defeating the whole point).
- ❌ Don't introduce a `code == 128` special-case (TreeDiff is not unborn-aware; a 128 is a bad SHA = real
  error). Use the simple `code != 0 → error` branch.
- ❌ Don't drop Part 1 (markdown per-file) — `StagedDiffOptions.MaxMDLines` must be honored; aggregate-only
  would silently ignore it.
- ❌ Don't re-add `BinaryExtensions` to `StagedDiffOptions` (already present — duplicate-declaration error).
- ❌ Don't mutate `defaultExcludes` (append md/binary excludes to a fresh `nmArgs` slice, not to `excludes`).
- ❌ Don't edit binary.go, run(), runWithInput(), or StagedDiff (all consumed — scope boundary).
- ❌ Don't implement RevParseTree/ReadTree/StatusPorcelain/WorkingTreeDiff (sibling work items).
- ❌ Don't redefine package-level test helpers (initRepo/writeFile/stageFile/writeTreeOf/sdManyLines) —
  duplicate-symbol compile error.
- ❌ Don't skip the no-changes test (`TreeDiff(A, A)` → `("", nil)`) — it proves empty-safety.
```
