---
name: "P2.M2.T2.S2 — Implement WorkingTreeDiff (unstaged working-tree diff with binary filtering) for the decompose planner input (PRD §13.6.2, FR-M3, FR3c)"
description: |

  ADD ONE METHOD to the `Git` interface (`internal/git/git.go`) AND implement it on `*gitRunner`.
  It is a mechanical port of the already-shipped `TreeDiff` (P2.M2.T1.S2) / `StagedDiff`
  (P2.M1.T1.S2) with the diff domain changed to the **unstaged working tree** — `git diff` WITHOUT
  `--cached` and WITHOUT tree positionals. The signature (verbatim from the work item + architecture
  doc):

    WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)

  WorkingTreeDiff is the **planner input** for multi-commit decomposition (PRD §13.6.2 / FR-M3:
  "Receives the full working-tree diff snapshot (with binary placeholders per FR3c) plus the style
  examples from §9.3"). The decompose orchestrator (P3.M2.T2.S1) feeds WorkingTreeDiff's payload to
  the planner agent so it can decide the commit count + partition. It applies the SAME caps, pathspec
  excludes, and FR3c binary filtering as StagedDiff/TreeDiff — the IDENTICAL placeholder format in
  every diff path (FR3c) — by reusing the already-shipped `detectBinaryFiles(ctx)` / `fileStatuses(ctx)`
  helpers with **empty diffArgs** (the `binary.go` package doc names this exact consumer:
  `WorkingTreeDiff: diffArgs = []`).

  CONTRACT (P2.M2.T2.S2, verbatim from the work item):
    1. RESEARCH NOTE: the planner (§13.6.2) receives the FULL working-tree diff — all unstaged
       changes. This is `git diff` WITHOUT --cached (working-tree-vs-index). Same caps/excludes/
       binary-filtering as StagedDiff. Binary files in the working tree get placeholder lines (FR3c).
    2. INPUT: Binary detection functions from P2.M1.T1.S1 (detectBinaryFiles/fileStatuses/
       isBinaryByExtension/binaryPlaceholderLine).
    3. LOGIC: Add WorkingTreeDiff to the Git interface and gitRunner. Mirrors StagedDiff but uses
       `git diff` (no --cached). Markdown: same per-file approach. Non-markdown: same aggregate with
       excludes + caps. Binary filtering: detectBinaryFiles with working-tree diff args (empty),
       emit placeholders, exclude from body. The planner feeds this diff + style examples (§17.5).
    4. OUTPUT: WorkingTreeDiff returns the full unstaged working-tree diff payload with binary
       filtering. Consumed by the decompose planner (P3.M2.T2.S1).
    5. DOCS: none — internal method.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `run()` / `runWithInput()` in git.go — CONSUMED, not modified.
    - `detectBinaryFiles` / `fileStatuses` / `isBinaryByExtension` / `binaryPlaceholderLine` /
      `defaultBinaryExtensions` in binary.go — CONSUMED as-is (package doc already names
      WorkingTreeDiff `diffArgs = []`). binary.go is UNCHANGED.
    - `StagedDiff`, `TreeDiff`, `StatusPorcelain` (parallel) — CONSUMED. StatusPorcelain appends to
      the SAME interface + file; WorkingTreeDiff appends AFTER it (END of interface + END of file)
      to minimize merge friction.
    - `StagedDiffOptions` / `defaultExcludes` / `EmptyTreeSHA` / `defaultMaxMDLines` /
      `defaultMaxDiffBytes` / `// Method ownership` comment — UNCHANGED.
    - Decompose wiring (P3.x) — NO caller references WorkingTreeDiff yet; this task only adds + tests.
    - go.mod / go.sum — UNCHANGED (stdlib only: context/fmt/sort/strings already imported in git.go).

  DELIVERABLES (1 file MODIFIED, 1 new file):
    MODIFY internal/git/git.go — (a) append `WorkingTreeDiff` to the `Git` INTERFACE (doc comment);
      (b) append the `(*gitRunner).WorkingTreeDiff` method body — a port of TreeDiff's body with the
      `treeA treeB` positionals dropped (no `--cached`, no tree args; empty diffArgs).
    CREATE internal/git/workingtreediff_test.go — 13 tests (basic working-tree diff, clean working
      tree → "", binary placeholder + excluded, binary user-override, keeps text companion, excludes
      applied, markdown not double-counted, non-markdown byte cap, markdown line cap, untracked-files
      omitted [documents the git-diff domain], non-repo→error, git-binary-missing, context-cancelled).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (the new
  interface method is additive — no existing caller breaks); `git diff` (no --cached) exits 0 with or
  without changes (NO --quiet, NO 128-special-case — simple branch like StagedDiff/TreeDiff); a
  tracked file modified in the working tree (not staged) appears in the payload; a tracked binary
  modified in the working tree emits `"<status>\t[binary] <path>"` and its body is excluded.

---

## Goal

**Feature Goal**: Implement the unstaged working-tree diff plumbing primitive at the `Git` interface
boundary — the diff payload the multi-commit **planner** agent reasons over (PRD §13.6.2 / FR-M3). It
is the working-tree analogue of `StagedDiff` (which is index-vs-HEAD) and the no-tree analogue of
`TreeDiff` (which is tree-to-tree): the same three-part payload (markdown per-file + line-capped;
binary placeholders via FR3c; non-markdown aggregate + byte-capped) with the SAME pathspec excludes —
the ONLY difference is the diff domain, `git diff` (working-tree-vs-index, NO `--cached`).

**Deliverable** (1 file MODIFIED, 1 new):
1. `internal/git/git.go` — one new method appended to the `Git` interface (after `StatusPorcelain`,
   the current last method) with a doc comment naming its command (`git diff` without `--cached`),
   its §13.6.2 planner-input role, its exit-code convention (0 with or without changes; 128 = real
   error; simple branch — NO `--quiet`, NO 128-special-case), the FR3c binary filtering, the
   read-only contract, AND the documented `git diff`-domain gotcha (untracked files are NOT shown);
   plus the `(*gitRunner).WorkingTreeDiff` implementation — a port of `TreeDiff`'s body with the
   `treeA treeB` positionals dropped (empty diffArgs → `detectBinaryFiles(ctx)` / `fileStatuses(ctx)`).
2. `internal/git/workingtreediff_test.go` — 13 tests mirroring `treediff_test.go`'s comprehensive
   set, adapted to the working-tree-delta setup idiom (commit a tracked baseline → modify in the
   working tree, NOT staged).

**Success Definition**:
- On a born repo with a tracked file modified in the working tree (NOT staged),
  `WorkingTreeDiff(ctx, StagedDiffOptions{})` returns a non-empty string containing that file's diff.
- On a born repo with a tracked **binary** file (e.g. `logo.png`) modified in the working tree (NOT
  staged), the payload contains `M\t[binary] logo.png` and does NOT contain `Binary files`.
- On a repo with no working-tree changes (clean), `WorkingTreeDiff(ctx, opts)` returns `("", nil)`.
- On a plain directory that is NOT a git repo, `WorkingTreeDiff(ctx, opts)` returns a non-nil error
  whose message contains `"git diff ...: failed"` (exit 128 = real error, NOT swallowed).
- With `PATH=""`, `WorkingTreeDiff(ctx, opts)` returns a non-nil error containing
  `"git binary not found"`.
- With a pre-cancelled context, `WorkingTreeDiff(ctx, opts)` returns a non-nil error with
  `errors.Is(err, context.Canceled)`.
- A file modified in the working tree but NOT in the index (untracked) does NOT appear in the payload
  (the documented `git diff` working-tree-vs-index domain — pinned by a test).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` shows EXACTLY 1 modified
  (`internal/git/git.go`) + 1 new untracked test file — nothing else.

## User Persona

**Target User**: the decompose planner agent invocation (internal code, P3.M2.T2.S1), and by extension
the end user running `stagecoach` on an un-staged working tree to get multiple logically-coherent
commits. WorkingTreeDiff is NOT a user-facing CLI flag; it is the diff primitive whose output is piped
to the planner agent's stdin (PRD §13.6.2, §17.5).

**Use Case**: when decomposition activates (nothing staged, working tree dirty — FR-M1/§13.6.1), the
orchestrator calls `WorkingTreeDiff(ctx, opts)` to capture the full unstaged working-tree diff, feeds
it (+ style examples from §9.3) to the planner, and the planner returns a JSON partition
`{count, single, commits:[{title,description}], message?}` (FR-M3).

**Pain Points Addressed**: the planner needs a single, bounded, noise-filtered diff of the working
tree (markdown per-file-capped; lock/snapshot/sourcemap/vendor excludes; binary placeholders instead
of useless `Binary files … differ` hunks; byte-capped aggregate) so it can reason about how to group
the changes into concepts without being flooded by generated noise or binary garbage.

## Why

- **Closes PRD §13.6.2 / FR-M3 at the plumbing layer.** FR-M3: the planner "Receives the full
  working-tree diff snapshot (with binary placeholders per FR3c)". This task is the literal
  interface-level implementation of that input, completing the third and final FR3c diff path (staged
  ✓ / tree-to-tree ✓ / working-tree ← THIS).
- **Zero new logic — a mechanical port of the already-shipped TreeDiff.** TreeDiff (P2.M2.T1.S2) is
  the closest sibling: it already demonstrates porting StagedDiff to a new diff domain (tree-to-tree)
  with identical caps/excludes/binary-filtering. WorkingTreeDiff is TreeDiff with the `treeA treeB`
  positionals dropped — `git diff` instead of `git diff <A> <B>`. No new types, no new options struct,
  no new shell surface, no new parsing. The `binary.go` helpers (shipped in P2.M1.T1.S1) already accept
  variadic `diffArgs`; calling them with no args produces exactly the working-tree domain.
- **Lowest-risk, maximal-reuse, backward-compatible.** The interface GAINS one method (Go interfaces
  are open for extension — no existing implementation or caller breaks; `*gitRunner` is the only `Git`
  implementor, verified). No existing file's behavior changes. go.mod/go.sum untouched. Unblocks
  P3.M2.T2.S1 (the planner agent call).

## What

One new method on the `Git` interface (`internal/git/git.go`), implemented on `*gitRunner`, delegating
to `run()` + the existing `detectBinaryFiles` / `fileStatuses` / `binaryPlaceholderLine` /
`isBinaryByExtension` helpers. No new types. No new options struct (it REUSES `StagedDiffOptions`).
No new dependencies. No caller wiring (that is P3.x). The structural edits are: append one interface
method + doc comment, append one `(*gitRunner)` method body (a port of TreeDiff), and add the new test
file.

### Success Criteria

- [ ] `WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)` is declared on
      the `Git` interface with a doc comment naming: the command (`git diff` without `--cached`); the
      §13.6.2 planner-input role; the exit-code convention (0 with or without changes; 128 = real
      error; simple branch — NO `--quiet`, NO 128-special-case); the FR3c binary filtering; the
      read-only contract; and the `git diff`-domain gotcha (untracked files are NOT shown).
- [ ] `(*gitRunner).WorkingTreeDiff` body is a port of `TreeDiff`'s body: identical three-part
      structure (markdown per-file line-capped → binary placeholders via empty-diffArgs helpers →
      non-markdown aggregate byte-capped) with the `treeA treeB` positionals DROPPED. All four
      `git diff` invocations use the simple exit-code branch (`if code != 0 → error`).
- [ ] `workingtreediff_test.go` proves: tracked-modified file appears; clean working tree → `("", nil)`;
      tracked binary modified → `M\t[binary] <path>` + body excluded; binary user-override works;
      text companion kept; excludes applied; markdown not double-counted; non-markdown byte cap;
      markdown line cap; untracked files omitted (documents the domain); non-repo → error;
      git-binary-missing → `"git binary not found"`; context-cancelled → `errors.Is(err, context.Canceled)`.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified + 1 new file.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract (findings §1
— `git diff` without `--cached`, empty diffArgs, signature confirmed by work item + architecture doc +
binary.go package doc); the EXACT method to port (findings §2 — TreeDiff's body, with a line-by-line
table mapping every `git diff` invocation's args); the exit-code convention (findings §3 — simple
branch, no --quiet, no 128-special-case); the untracked-files gotcha (findings §4 — documented + a
test that pins it); the binary-detection verification (findings §5); the test-setup idiom (findings §6
— commit a tracked baseline → modify in working tree, NOT staged); the reusable helpers (findings §7);
the scope boundaries (findings §8); and the architecture-doc spec (findings §9). No prompt/decompose/
registry knowledge required — the contract is fully self-contained at the git-plumbing layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (the port map + verified behaviors + the gotcha)
- docfile: plan/002_a17bb6c8dc1d/P2M2T2S2/research/findings.md
  why: §1 the CONTRACT (signature + `git diff` no --cached + empty diffArgs, confirmed 3 ways);
       §2 the LINE-BY-LINE PORT MAP (a table mapping every git diff invocation from StagedDiff/TreeDiff
       to WorkingTreeDiff — this IS the implementation blueprint); §3 the exit-code convention (simple
       branch); §4 the CRITICAL UNTRACKED-FILES GOTCHA (git diff omits untracked files — per the
       contract — documented + pinned by a test, NOT "fixed"); §5 binary detection verified in the
       working-tree domain; §6 the test-setup idiom (commit baseline → modify working tree, NOT staged);
       §7 reusable helpers; §8 scope boundaries; §9 architecture spec.
  critical: §2 (the port map — drop treeA/treeB from TreeDiff's body); §4 (untracked files are NOT
            shown — do NOT try to include them, do NOT add ls-files — that violates the contract);
            §3 (simple branch, NOT HasStagedChanges' --quiet inversion, NOT RevParseHEAD's 128-as-unborn).

# MUST READ — the FILE TO MODIFY: the Git interface + the two methods to port (StagedDiff + TreeDiff)
- file: internal/git/git.go
  section: the `Git` interface (append WorkingTreeDiff at the END, after `StatusPorcelain` — currently
           the last method at ~line 134); `(*gitRunner).StagedDiff` at ~line 392 (the --cached reference);
           `(*gitRunner).TreeDiff` at ~line 856 (the CLOSEST template — the body to copy verbatim and
           edit); `(*gitRunner).run` at ~line 140 (the helper WorkingTreeDiff calls — its INVARIANT:
           non-zero git exit → (stdout, stderr, exitCode, nil), err nil).
  why: TreeDiff is the single best template (it is itself a port of StagedDiff to a non---cached domain,
       so it already solved every shared concern: cap defaults, sort binPaths, the SEPARATE binExcludes
       slice, the structural markdown double-count excludes, the truncation sentinels, the error-message
       text). Copying its body and dropping the `treeA treeB` positionals IS the implementation.
  pattern: copy `(*gitRunner).TreeDiff`'s WHOLE body; in each of the 4 git-diff arg slices, DROP the
           `treeA, treeB` positionals so the command becomes plain `git diff ...` (working-tree domain);
           call `g.detectBinaryFiles(ctx)` and `g.fileStatuses(ctx)` with NO extra args (empty diffArgs ⇒
           `git diff --numstat` / `git diff --name-status`); change the error-message prefixes to drop the
           "tree-to-tree" wording (use "working-tree"); keep EVERYTHING ELSE byte-identical (cap defaults,
           markdown excludes, byte/line caps, builder concatenation). See "Implementation Patterns".
  gotcha: do NOT add `--cached` anywhere (that would capture the STAGED diff, the wrong domain). do NOT
          add `HEAD` as a positional (that would be working-tree-vs-HEAD, also wrong — the contract is
          working-tree-vs-INDEX). do NOT mutate defaultExcludes / StagedDiffOptions / any consumed file.
          do NOT edit the `// Method ownership` comment block.

# MUST READ — the CONSUMED binary helpers (read-only; the package doc already names WorkingTreeDiff)
- file: internal/git/binary.go
  section: package doc (explicitly lists `WorkingTreeDiff: diffArgs = []` as an FR3c consumer);
           `detectBinaryFiles(ctx, diffArgs...)` (call with NO diffArgs); `fileStatuses(ctx, diffArgs...)`
           (call with NO diffArgs); `binaryPlaceholderLine(status, path)` (emits "<status>\t[binary] <path>");
           `isBinaryByExtension(path, extraExts)` (flows opts.BinaryExtensions).
  why: these are the FR3a/b/c primitives WorkingTreeDiff REUSES unchanged. Their variadic diffArgs is the
       seam that makes the working-tree domain a one-word change (empty args). READ to confirm the
       contract; do NOT modify binary.go.
  gotcha: binary.go is OWNED by P2.M1.T1.S1 and already shipped — UNCHANGED. Calling `detectBinaryFiles(ctx)`
          with no variadic args builds `git diff --numstat` (working tree). Calling `fileStatuses(ctx)` with
          no args builds `git diff --name-status` (working tree).

# MUST READ — the test exemplars (the patterns to mirror in workingtreediff_test.go)
- file: internal/git/treediff_test.go   (READ — the CLOSEST test template: same three-part diff method)
  why: the EXACT test suite WorkingTreeDiff mirrors: _BasicConceptDiff, _NoChanges, _BinaryPlaceholderAndExcluded,
       _BinaryExtensionsUserOverride, _KeepsTextCompanion, _ExcludesApplied, _MarkdownNotDoubleCounted,
       _NonMarkdownByteCap, _MarkdownLineCap, _GitBinaryMissing, _ContextCancelled. Copy these, adapt the
       setup idiom (findings §6 — commit baseline → modify working tree, NOT staged), and replace
       TreeDiff's _BadTreeSHA with _NotARepo (no tree SHAs in the working-tree domain) + add
       _UntrackedFilesOmitted (the documented gotcha).
  pattern: mirror these tests; replace `g.TreeDiff(ctx, treeA, treeB, opts)` with `g.WorkingTreeDiff(ctx, opts)`;
           replace the `writeFile`+`stageFile`+`writeTreeOf` tree setup with `writeFile`+`stageFile`+
           `execGit(commit)` baseline + a second `writeFile` (working-tree delta, NOT staged).
- file: internal/git/stagediff_test.go   (READ — the binary-filtering test exemplars + sdManyLines helper)
  why: defines `sdManyLines(n)` (REUSED for the markdown line-cap test) and the canonical binary tests
       (_BinaryFilePlaceholderAndExcluded, _BinaryExtensionsUserOverride, _BinaryKeepsTextCompanion,
       _BinaryInSubdirectory, _MixedMarkdownBinaryCode). The binary-setup idiom (real PNG header bytes
       `\x89PNG\r\n\x1a\n\x00\x00\x00`) is REUSED — but staged differently (commit the binary, then modify it).
- file: internal/git/committree_test.go   (READ — fixture definitions to REUSE)
  why: defines the package-level helpers `writeFile`, `stageFile`, `writeTreeOf` — ALL reusable.
  gotcha: these helpers are ALREADY defined — do NOT redefine them (duplicate-symbol compile error).
- file: internal/git/revparsetree_test.go   (READ — execGit helper)
  why: defines `execGit(t, dir, args...)` (runs a git command in the dir, returns stdout) — USE it for the
       baseline `commit -m init` (NOT `makeEmptyCommit`, which makes an allow-EMPTY commit and stages nothing).
- file: internal/git/revparse_test.go   (READ — makeEmptyCommit helper)
  why: defines `makeEmptyCommit(t, dir, msg)` — usable for the "clean working tree" test (a commit with no
       files leaves the working tree clean), but NOT for establishing a tracked baseline.
- file: internal/git/git_test.go   (READ — initRepo helper)
  why: defines `initRepo(t, dir)` — EVERY temp-repo test starts here (git init + repo-local identity).

# MUST READ — the design reference (signature + role)
- docfile: plan/002_a17bb6c8dc1d/architecture/binary_git_v2.md
  section: "### WorkingTreeDiff" — confirms the signature `WorkingTreeDiff(ctx, opts StagedDiffOptions)
           (string, error)`, the command (`git diff` NO --cached), the FR3c binary filtering, and the
           planner-input role. NOTE: the doc lists 5 new methods; THIS task implements ONLY
           WorkingTreeDiff (the last of the 5). The others are separate work items.
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: line 11 (planner role: "analyze full working-tree diff") + line 87-89 (the WorkingTreeDiff
           signature) — confirms WorkingTreeDiff as the planner input. Consumed by P3.M2.T2.S1.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.2 (the four agent roles — planner)
  why: §13.6.2 / FR-M3: the planner "Receives the full working-tree diff snapshot (with binary
       placeholders per FR3c) plus the style examples from §9.3". This task implements that input.
- url: PRD.md §9.1 (FR1-FR4, FR3a-FR3c — diff capture + binary filtering)
  why: FR3c mandates "Binary filtering applies in EVERY diff path … the multi-commit working-tree
       snapshot (§13.6.2) … The placeholder format is identical in all three." WorkingTreeDiff is that
       second path.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go                       # MODIFY: append WorkingTreeDiff to the Git interface (doc comment) AND
                               #   implement on *gitRunner (port of TreeDiff — drop treeA/treeB). run()/
                               #   StagedDiff/TreeDiff/StatusPorcelain/StagedDiffOptions/EmptyTreeSHA/
                               #   defaultExcludes/defaultMaxMDLines/defaultMaxDiffBytes UNCHANGED (consumed).
  binary.go                    # READ: detectBinaryFiles/fileStatuses/binaryPlaceholderLine/isBinaryByExtension
                               #   (CONSUMED with empty diffArgs; package doc names WorkingTreeDiff). UNCHANGED.
  treediff_test.go             # READ: the CLOSEST test template (same three-part diff method + binary/cap/exclude tests).
  stagediff_test.go            # READ: binary-filtering test exemplars + sdManyLines helper (REUSE).
  committree_test.go           # READ: writeFile/stageFile/writeTreeOf helpers (REUSE).
  revparsetree_test.go         # READ: execGit helper (USE for baseline commit).
  revparse_test.go             # READ: makeEmptyCommit helper (REUSE for clean-tree test).
  git_test.go                  # READ: initRepo helper (REUSE in every test).
  (*_test.go)                  # READ: other per-method test files (the one-file-per-method convention).
go.mod / go.sum                # UNCHANGED (stdlib only: context/fmt/sort/strings already imported in git.go).
.golangci.yml                  # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added/modified

```bash
internal/git/git.go              # MODIFY — append to the `Git` interface (at the END, after `StatusPorcelain`,
                                 #   currently ~line 134):
                                 #     // WorkingTreeDiff returns the unstaged working-tree diff payload (PRD
                                 #     // §13.6.2 / FR-M3 — the planner input). It runs `git diff` WITHOUT
                                 #     // --cached (working-tree-vs-index): …same caps/excludes/binary filtering
                                 #     // as StagedDiff/TreeDiff; FR3c placeholder format identical. Untracked
                                 #     // files are NOT shown (git diff domain). Simple exit-code branch (0 with
                                 #     // or without changes; 128 = real error). Read-only.
                                 #     WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)
                                 #   AND append the (*gitRunner).WorkingTreeDiff method body at the END of the
                                 #   file (after StatusPorcelain's body): the port of TreeDiff (drop treeA/treeB).
                                 #   NO other change to git.go.
internal/git/workingtreediff_test.go   # NEW — 13 tests (basic working-tree diff, clean working tree, binary
                                       #   placeholder + excluded, binary user-override, keeps text companion,
                                       #   excludes applied, markdown not double-counted, non-markdown byte cap,
                                       #   markdown line cap, untracked-files omitted, non-repo→error,
                                       #   git-binary-missing, context-cancelled).
# go.mod/go.sum UNCHANGED. run()/StagedDiff/TreeDiff/binary.go UNCHANGED. 0 other edits.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the port IS the implementation — findings §2): WorkingTreeDiff is TreeDiff with the
// `treeA, treeB` positionals DROPPED from every git-diff arg slice. Copy TreeDiff's WHOLE body and:
//   TreeDiff md list:  "diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown"
//   WorkingTreeDiff:   "diff",                "--name-only", "--", "*.md", "*.markdown"
//   TreeDiff per-file: "diff", treeA, treeB, "--", file
//   WorkingTreeDiff:   "diff",                "--", file
//   TreeDiff detect:   g.detectBinaryFiles(ctx, treeA, treeB)
//   WorkingTreeDiff:   g.detectBinaryFiles(ctx)                 // empty diffArgs ⇒ git diff --numstat
//   TreeDiff statuses: g.fileStatuses(ctx, treeA, treeB)
//   WorkingTreeDiff:   g.fileStatuses(ctx)                      // empty diffArgs ⇒ git diff --name-status
//   TreeDiff aggregate:"diff", treeA, treeB, "--", <excludes..., :!*.md, :!*.markdown, binExcludes...>
//   WorkingTreeDiff:   "diff",                "--", <excludes..., :!*.md, :!*.markdown, binExcludes...>
// EVERYTHING ELSE (cap defaults, sort binPaths, SEPARATE binExcludes slice, structural markdown excludes,
// truncation sentinels, builder concatenation) is byte-identical to TreeDiff.

// CRITICAL (git diff WITHOUT --cached, WITHOUT HEAD — findings §1/§4): the diff domain is
// working-tree-vs-INDEX. Do NOT add `--cached` (that is the STAGED diff — StagedDiff's job). Do NOT add
// `HEAD` as a positional (that would be working-tree-vs-HEAD, a DIFFERENT domain). The contract is the
// literal `git diff` (working-tree-vs-index), named 3× in the work item + architecture doc + binary.go.

// CRITICAL (untracked files are NOT shown — findings §4): `git diff` (no --cached) OMITS untracked files
// (git never lists untracked files in a diff — untracked = not in the index = nothing to diff against).
// Only tracked-but-modified and tracked-but-deleted files appear. This is the EXPLICIT contract, NOT a
// bug to fix. Do NOT add `git ls-files --others` / `--exclude-standard` / `git add -A`-then-diff to
// "capture" untracked files — that would violate the work-item contract and scope-creep into the
// decompose orchestrator (P3). The stager (FR-M5) is a tooled agent with full repo access and discovers
// untracked files itself. DOCUMENT the gotcha in the interface doc comment; PIN it with
// TestWorkingTreeDiff_UntrackedFilesOmitted.

// CRITICAL (simple exit-code branch — findings §3): `git diff` (WITHOUT --quiet) exits 0 whether or not
// there are changes (empty working tree → exit 0 + empty stdout; dirty → exit 0 + non-empty stdout).
// Exit 128 = bad pathspec / corrupt repo = a REAL error. So use the SIMPLE branch (`if code != 0 → error`),
// byte-identical to StagedDiff/TreeDiff. Do NOT use `--quiet` (that is HasStagedChanges' exit-inversion
// trick — exit 1 = has-staged — IRRELEVANT here). Do NOT add a `if code == 128` special-case (that is
// RevParseHEAD/RecentMessages' unborn convention — IRRELEVANT; `git diff` exits 0 on unborn repos too).

// GOTCHA (run() returns err==nil for non-zero git exits): run()'s INVARIANT is that a non-zero git exit
// is returned as (stdout, stderr, exitCode, nil) — err is nil, the exit code is the signal. Only
// infrastructural failures (LookPath miss / context cancel / start-I/O) return err != nil (exitCode -1).
// So each git-diff invocation does `if err != nil { return "", err }` FIRST (catches context.Canceled +
// git-binary-missing, propagated UNWRAPPED so errors.Is works at the call site), THEN `if code != 0`.
// Copy TreeDiff's branches exactly (4 invocations × 2 branches each).

// GOTCHA (binExcludes is a SEPARATE slice — never append to `excludes`): in TreeDiff/StagedDiff the
// `excludes` local may alias `defaultExcludes` (when opts.Excludes is empty); appending to it would
// MUTATE the package-level defaultExcludes var across calls. WorkingTreeDiff MUST use a SEPARATE
// `var binExcludes []string` slice (copy this verbatim from TreeDiff).

// GOTCHA (structural markdown excludes are ALWAYS appended): `:!*.md` and `:!*.markdown` are appended to
// the non-markdown aggregate REGARDLESS of opts.Excludes — they are structural (prevent markdown from
// being double-counted in BOTH Part 1 and Part 2). Copy this verbatim from TreeDiff.

// GOTCHA (do NOT modify run(), binary.go, StagedDiff, TreeDiff, StatusPorcelain, or any consumed file):
// all are CONSUMED. WorkingTreeDiff adds ONE method + ONE test file. Editing any consumed file is a
// scope violation (and would conflict with the parallel StatusPorcelain work).

// GOTCHA (parallel StatusPorcelain work): StatusPorcelain (P2.M2.T2.S1) ALSO appends to git.go's
// interface block + gitRunner methods. Append WorkingTreeDiff at the END of the interface (after
// StatusPorcelain) and at the END of the file (after StatusPorcelain's body), and create
// workingtreediff_test.go as a NEW file → minimal merge friction. Do NOT edit the `// Method ownership`
// comment block. If a 3-way merge conflict appears at the closing interface brace, keep BOTH additions.

// GOTCHA (one-file-per-method test convention): the package uses one test file per Git method. Put
// WorkingTreeDiff tests in a NEW `workingtreediff_test.go`. Test function names MUST be distinct (prefix
// `TestWorkingTreeDiff_`). Do NOT redefine any package-level helper (initRepo/writeFile/stageFile/
// writeTreeOf/makeEmptyCommit/execGit/sdManyLines) — redefining = duplicate-symbol compile error.

// GOTCHA (test-setup idiom DIFFERS from StagedDiff/TreeDiff — findings §6): those tests create changes
// via writeFile+stageFile (change lands in the index/trees). WorkingTreeDiff needs the change in the
// WORKING TREE vs index, so: commit a tracked baseline (writeFile+stageFile+execGit(commit)) → then
// writeFile again to modify it (NOT staged). Use `execGit(t, repo, "commit", "-m", "init")` for the
// baseline (NOT `makeEmptyCommit`, which makes an allow-EMPTY commit and stages nothing).
```

## Implementation Blueprint

### Data models and structure

No new TYPES. No new OPTIONS structs (WorkingTreeDiff REUSES `StagedDiffOptions`). No new constants.
The implementation consumes the existing helpers unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   func (g *gitRunner) run(ctx, repo, args ...string) (stdout, stderr string, exitCode int, err error)
//   const defaultMaxMDLines = 100; const defaultMaxDiffBytes = 300000
//   var defaultExcludes = []string{":!*.lock", ":!package-lock.json", ...}
// run()'s invariant: non-zero git exit → (stdout, stderr, exitCode, nil); err != nil ⟺ infrastructural
// failure only (LookPath miss / context cancel / start-I/O), with exitCode == -1.
// from internal/git/binary.go (CONSUME — do not modify):
//   func (g *gitRunner) detectBinaryFiles(ctx, diffArgs...) (map[string]bool, error)
//   func (g *gitRunner) fileStatuses(ctx, diffArgs...) (map[string]string, error)
//   func binaryPlaceholderLine(status, path string) string
//   func isBinaryByExtension(path string, extraExts []string) bool
// detectBinaryFiles/fileStatuses take a VARIADIC diffArgs; calling them with NO args selects the
// working-tree domain (`git diff --numstat` / `git diff --name-status`).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — append WorkingTreeDiff to the `Git` interface
  - LOCATE the END of the `Git` interface (the closing `}` after the `StatusPorcelain` method, currently
    ~line 134). APPEND WorkingTreeDiff there (after StatusPorcelain) — appending at the end minimizes
    merge friction with the parallel StatusPorcelain work.
  - INSERT, before the closing brace, the method declaration WITH a doc comment (see "Implementation
    Patterns" for the exact text).
  - NAMING: `WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)` (EXACT, from
    the work item + architecture doc). Use a named return like `(diff string, err error)` to match the
    file's doc-comment style.
  - DOC COMMENT must name: the command (`git diff` WITHOUT --cached — working-tree-vs-index); the
    §13.6.2 / FR-M3 planner-input role; the FR3c binary filtering (identical placeholder format); the
    SAME caps/excludes as StagedDiff/TreeDiff; the exit-code convention (0 with or without changes;
    128 = real error; simple branch — NO --quiet, NO 128-special-case); the read-only contract; AND the
    git-diff-domain gotcha (untracked files are NOT shown).
  - GOTCHA: do NOT edit the `// Method ownership` comment block. Do NOT add any options struct (signature
    reuses StagedDiffOptions).
  - PLACEMENT: internal/git/git.go, inside `type Git interface { … }`, at the END (after StatusPorcelain).

Task 2: MODIFY internal/git/git.go — implement (*gitRunner).WorkingTreeDiff (PORT of TreeDiff)
  - APPEND a new method `func (g *gitRunner) WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions)
    (string, error)` at the END of the file (after StatusPorcelain's body) — END-of-file is preferred to
    avoid merge conflict with the parallel StatusPorcelain work.
  - BODY: copy `(*gitRunner).TreeDiff`'s WHOLE body; in each of the 4 git-diff arg slices DROP the
    `treeA, treeB` positionals (see "Implementation Patterns" for the exact body); change the two
    error-message prefixes from "tree-to-tree" to "working-tree"; keep everything else byte-identical
    (cap defaults via defaultMaxMDLines/defaultMaxDiffBytes; detectBinaryFiles(ctx)/fileStatuses(ctx)
    with NO args; sort.Strings(binPaths); SEPARATE binExcludes slice; structural :!*.md/:!*.markdown;
    truncation sentinels; builder concatenation).
  - PATTERN: the 4 git-diff invocations become:
      md list:    g.run(ctx, g.workDir, "diff", "--name-only", "--", "*.md", "*.markdown")
      per-file:   g.run(ctx, g.workDir, "diff", "--", file)
      aggregate:  g.run(ctx, g.workDir, nmArgs...) where nmArgs = ["diff", "--", excludes..., ":!*.md",
                    ":!*.markdown", binExcludes...]
      (detectBinaryFiles/fileStatuses take no diffArgs.)
    Each invocation: `if err != nil { return "", err }` FIRST (UNWRAPPED), then `if code != 0` → error.
  - GOTCHA: NO `--cached`, NO `HEAD` positional, NO `--quiet`, NO 128-special-case. The error-prefix
    strings: "git diff (markdown list): failed (exit %d): %s",
    "git diff -- %s: failed (exit %d): %s" (per-file), "git diff (non-markdown): failed (exit %d): %s"
    (mirror StagedDiff's wording, NOT TreeDiff's "tree-to-tree" wording).
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method at the END of the file (after StatusPorcelain).

Task 3: CREATE internal/git/workingtreediff_test.go — 13 tests for WorkingTreeDiff
  - IMPORTS: context, errors, strings, testing (mirror treediff_test.go). The package is `git` (same
    package — helpers initRepo/writeFile/stageFile/execGit/makeEmptyCommit/sdManyLines are visible).
  - ADD TestWorkingTreeDiff_BasicWorkingTreeDiff:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "code.go", "package main\n"); stageFile(t, repo, "code.go");
      execGit(t, repo, "commit", "-m", "init");                       # tracked baseline; index==HEAD
      writeFile(t, repo, "code.go", "package main\n// modified\n");   # WORKING-TREE delta (NOT staged)
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err == nil; assert strings.Contains(out, "code.go") && strings.Contains(out, "// modified").
  - ADD TestWorkingTreeDiff_CleanWorkingTree:
      repo := t.TempDir(); initRepo(t, repo); makeEmptyCommit(t, repo, "init");   # nothing in working tree
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err == nil && out == "";   # clean working tree → ("", nil)
  - ADD TestWorkingTreeDiff_BinaryPlaceholderAndExcluded:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00old"); stageFile(t, repo, "logo.png");
      writeFile(t, repo, "code.go", "package main\n"); stageFile(t, repo, "code.go");
      execGit(t, repo, "commit", "-m", "init");                       # tracked baseline
      writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00new");  # tracked binary MODIFIED
      writeFile(t, repo, "code.go", "package main\n// x\n");                # tracked text MODIFIED
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "M\t[binary] logo.png");   # FR3c placeholder, working-tree status M
      assert !strings.Contains(out, "Binary files");          # body excluded
      assert strings.Contains(out, "code.go");                # text companion present
  - ADD TestWorkingTreeDiff_BinaryExtensionsUserOverride:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "data.dat", "hello\n"); stageFile(t, repo, "data.dat");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "data.dat", "hello world\n");   # tracked .dat MODIFIED (text content)
      g := New(repo);
      # Without override: .dat with text content is NOT binary
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err == nil && !strings.Contains(out, "[binary] data.dat");
      # With override: caught via extension signal
      out, err = g.WorkingTreeDiff(context.Background(), StagedDiffOptions{BinaryExtensions: []string{"dat"}});
      assert err == nil && strings.Contains(out, "M\t[binary] data.dat");
  - ADD TestWorkingTreeDiff_KeepsTextCompanion:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00old"); stageFile(t, repo, "img.png");
      writeFile(t, repo, "code.go", "package main\n"); stageFile(t, repo, "code.go");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "img.png", "\x89PNG\r\n\x1a\n\x00\x00\x00new");            # binary MODIFIED
      writeFile(t, repo, "code.go", "package main\nfunc main() {}\n");              # text MODIFIED
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert strings.Contains(out, "M\t[binary] img.png") && strings.Contains(out, "code.go");
      assert !strings.Contains(out, "Binary files");
  - ADD TestWorkingTreeDiff_ExcludesApplied:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "keep.go", "package main\n"); stageFile(t, repo, "keep.go");
      writeFile(t, repo, "drop.go", "package drop\n"); stageFile(t, repo, "drop.go");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "keep.go", "package main\n// k\n");   # MODIFIED
      writeFile(t, repo, "drop.go", "package drop\n// d\n");   # MODIFIED
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{Excludes: []string{":!drop.go"}});
      assert strings.Contains(out, "keep.go") && !strings.Contains(out, "drop.go");
  - ADD TestWorkingTreeDiff_MarkdownNotDoubleCounted:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "only.md", "# a\n"); stageFile(t, repo, "only.md");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "only.md", "# a\n\nmore\n");   # tracked markdown MODIFIED
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert strings.Contains(out, "only.md");
      assert strings.Count(out, "diff --git a/only.md b/only.md") == 1;   # no double-count
  - ADD TestWorkingTreeDiff_NonMarkdownByteCap:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "big.go", "package main\n"); stageFile(t, repo, "big.go");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000));   # big MODIFIED
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 100});
      assert strings.Contains(out, "... [diff truncated at 100 bytes]") && len(out) < 200;
  - ADD TestWorkingTreeDiff_MarkdownLineCap:
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "big.md", "# t\n"); stageFile(t, repo, "big.md");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "big.md", sdManyLines(50));   # tracked markdown MODIFIED, big
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{MaxMDLines: 10});
      assert strings.Contains(out, "... [diff truncated at 10 lines]");
  - ADD TestWorkingTreeDiff_UntrackedFilesOmitted:   # DOCUMENTS the git-diff domain gotcha (findings §4)
      repo := t.TempDir(); initRepo(t, repo);
      writeFile(t, repo, "tracked.go", "package main\n"); stageFile(t, repo, "tracked.go");
      execGit(t, repo, "commit", "-m", "init");
      writeFile(t, repo, "tracked.go", "package main\n// modified\n");  # tracked MODIFIED → SHOWN
      writeFile(t, repo, "untracked.go", "new\n");                      # UNTRACKED → NOT shown
      g := New(repo);
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err == nil;
      assert strings.Contains(out, "tracked.go");          # tracked-modified IS shown
      assert !strings.Contains(out, "untracked.go");       # untracked is NOT (git diff working-tree-vs-index)
  - ADD TestWorkingTreeDiff_NotARepo:
      g := New(t.TempDir());   # plain dir, NOT a git repo (no initRepo)
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err != nil && out == "";   # exit 128 = real error (NOT swallowed). Substring-agnostic on prefix.
  - ADD TestWorkingTreeDiff_GitBinaryMissing:
      t.Setenv("PATH", "");   g := New(t.TempDir());
      out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{});
      assert err != nil && strings.Contains(err.Error(), "git binary not found") && out == "";
  - ADD TestWorkingTreeDiff_ContextCancelled:
      ctx, cancel := context.WithCancel(context.Background()); cancel();   # cancel BEFORE call
      g := New(t.TempDir());
      out, err := g.WorkingTreeDiff(ctx, StagedDiffOptions{});
      assert errors.Is(err, context.Canceled) && out == "";
  - PATTERN: copy treediff_test.go's tests; replace `g.TreeDiff(ctx, treeA, treeB, opts)` with
    `g.WorkingTreeDiff(ctx, opts)`; replace the `writeFile`+`stageFile`+`writeTreeOf` tree setup with
    `writeFile`+`stageFile`+`execGit(commit)` baseline + a second `writeFile` (working-tree delta).
  - GOTCHA: do NOT redefine initRepo/writeFile/stageFile/execGit/makeEmptyCommit/sdManyLines — they are
    already package-level (redefining = compile error). Use `execGit(t, repo, "commit", "-m", "init")`
    for the baseline (NOT makeEmptyCommit).
  - GOTCHA: TestWorkingTreeDiff_UntrackedFilesOmitted + TestWorkingTreeDiff_NotARepo together pin the
    git-diff domain contract: untracked files are omitted (by design), and a non-repo is a real error
    (NOT a benign empty result).
  - PLACEMENT: NEW file internal/git/workingtreediff_test.go.

Task 4: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/workingtreediff_test.go`
  - `go build ./...`   (whole module compiles — the interface + impl + test file)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/git/ -run "TestWorkingTreeDiff" -v`   (all 13 new tests)
  - `go test -race ./internal/git/`   (the WHOLE git package — existing tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; the new interface method is additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + `?? internal/git/workingtreediff_test.go`
    (2 entries); run()/binary.go/StagedDiff/TreeDiff/StatusPorcelain UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1: the interface addition (append at the END of `type Git interface { … }`, after StatusPorcelain) ===

	// WorkingTreeDiff returns the unstaged working-tree diff payload for multi-commit decomposition's
	// planner input (PRD §13.6.2 / FR-M3: the planner "Receives the full working-tree diff snapshot
	// (with binary placeholders per FR3c) plus the style examples from §9.3"). It is the working-tree
	// analogue of StagedDiff (which is index-vs-HEAD) and the no-tree analogue of TreeDiff (which is
	// tree-to-tree): the SAME three-part payload (markdown per-file + line-capped; FR3c binary
	// placeholders; non-markdown aggregate + byte-capped) with the SAME pathspec excludes — the ONLY
	// difference is the diff domain: it runs `git diff` WITHOUT --cached (working-tree-vs-INDEX), never
	// `git diff --cached` and never `git diff HEAD`.
	//
	// IMPORTANT — the `git diff` domain omits untracked files: `git diff` (no --cached) compares the
	// working tree to the INDEX, and git never lists untracked files in a diff (untracked = not in the
	// index = nothing to diff against). Only tracked-but-modified and tracked-but-deleted files appear.
	// This is the explicit contract (the work item names `git diff` WITHOUT --cached); the tooled stager
	// (FR-M5) discovers untracked files itself. Callers must not expect untracked files in this payload.
	//
	// `git diff` (without --quiet) exits 0 whether or not there are changes (empty working tree → exit 0,
	// empty stdout; dirty → exit 0, non-empty stdout); exit 128 means a bad pathspec or corrupt repo — a
	// REAL error (NOT an unborn signal: branch on code != 0, never on code == 128; never use --quiet).
	// Read-only with respect to refs and the index (PRD §18.1). A no-change working tree returns ("", nil).
	WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)


// === Task 2: the implementation (append at the END of the file, after StatusPorcelain's body) ===
// This is a port of TreeDiff: copy its body verbatim, drop the `treeA, treeB` positionals from every
// git-diff arg slice, and call detectBinaryFiles/fileStatuses with NO diffArgs (empty ⇒ working-tree
// domain). Same caps/excludes/binary-filtering/truncation as StagedDiff/TreeDiff.

func (g *gitRunner) WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ---- (working-tree domain: no --cached, no tree args)
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git diff (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", "--", file)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff -- %s: failed (exit %d): %s", file, fcode, strings.TrimSpace(fstderr))
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

	// ---- Binary filtering (PRD §9.1 FR3a/b/c, working-tree path) ----
	// Empty diffArgs ⇒ `git diff --numstat` / `git diff --name-status` (working-tree-vs-index).
	binSet, berr := g.detectBinaryFiles(ctx)
	if berr != nil {
		return "", berr
	}
	statuses, serr := g.fileStatuses(ctx)
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
	var binExcludes []string // SEPARATE slice — never append to `excludes` (it may alias defaultExcludes)
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path)) // "<status>\t[binary] <path>"
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", "--"}
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...) // drop binary bodies from the aggregate
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}
```

### Integration Points

```yaml
DATABASE:
  - none (git object store / index / refs are read-only here; no migration, no write).

CONFIG:
  - none directly (StagedDiffOptions carries caps/excludes/binary_extensions, populated by the caller —
    the P3 decompose orchestrator resolves these from config before calling WorkingTreeDiff).

ROUTES:
  - none (internal plumbing method; no CLI flag, no public API surface in this task). The P3 decompose
    orchestrator (P3.M2.T2.S1) wires the call: `diff, err := deps.Git.WorkingTreeDiff(ctx, opts); if err
    != nil { ... }; plannerInput := assemblePlannerPrompt(diff, styleExamples)`. That wiring is a
    SEPARATE work item.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after editing git.go + creating the test file — fix before proceeding
gofmt -w internal/git/git.go internal/git/workingtreediff_test.go
go build ./...            # whole module compiles (interface + impl + test file)
go vet ./...
golangci-lint run ./...   # errcheck/gosimple/govet/ineffassign/staticcheck/unused (.golangci.yml)

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
gofmt -l internal/ pkg/   # must print NOTHING (all formatted)
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test WorkingTreeDiff as created
go test -race ./internal/git/ -run "TestWorkingTreeDiff" -v   # all 13 new tests

# Full git-package regression (existing methods still pass — the new interface method is additive)
go test -race ./internal/git/

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# Full module regression — no other package breaks (the new interface method is additive: *gitRunner is
# the only Git implementor, so no other package needs updating).
go test ./...

# Manual plumbing check (independent oracle — proves the implementation matches real git behavior,
# especially the working-tree domain + the untracked-files omission):
cd "$(mktemp -d)" && git init -q && git config user.name T && git config user.email t@t.com
echo "x" > code.go; git add code.go; git commit -qm init        # tracked baseline
echo "x" > code.go; printf 'package main\n// modified\n' > code.go # tracked MODIFIED (not staged)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00' > logo.png              # UNTRACKED binary
echo "untracked.go:"; echo "new" > untracked.go                # UNTRACKED text
echo "----- git diff (no args) — working-tree-vs-index -----"; git diff; echo "exit=$?"
echo "----- git diff --numstat -----"; git diff --numstat
echo "----- git diff --name-status -----"; git diff --name-status
# Expected: code.go shows (modified); logo.png + untracked.go do NOT (untracked = omitted by git diff).
# go test ./... fully green.

cd "$(mktemp -d)"; echo "non-repo:"; git diff; echo "exit=$?"   # expect exit 128 (real error)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (No creative/external validation needed — WorkingTreeDiff is a pure read-only plumbing primitive with
#  no network, no filesystem mutation, no concurrency. Levels 1–3 fully cover it.)

# Confirm go.mod/go.sum are untouched (stdlib-only, no new deps):
git diff --exit-code go.mod go.sum && echo "deps unchanged"
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test -race ./...`
- [ ] No linting errors: `golangci-lint run ./...`
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/ pkg/` (empty output)

### Feature Validation

- [ ] `WorkingTreeDiff` declared on the `Git` interface (after `StatusPorcelain`) with the full doc comment
- [ ] `(*gitRunner).WorkingTreeDiff` implemented as the port of `TreeDiff` (drop treeA/treeB; empty diffArgs)
- [ ] Tracked-modified file (not staged) → appears in the payload
- [ ] Clean working tree → `("", nil)`
- [ ] Tracked binary modified → `M\t[binary] <path>` + body excluded (FR3c, working-tree status M)
- [ ] Binary user-override (`opts.BinaryExtensions`) flows through `isBinaryByExtension`
- [ ] Caps (markdown line, non-markdown byte) + excludes + markdown-not-double-counted all behave like StagedDiff
- [ ] Untracked files omitted (documents the git-diff domain — pinned by `_UntrackedFilesOmitted`)
- [ ] Non-repo → error (exit 128, NOT swallowed); git-binary-missing → `"git binary not found"`;
      context-cancelled → `errors.Is(err, context.Canceled)`
- [ ] NO `--cached`, NO `HEAD` positional, NO `--quiet`, NO 128-special-case (simple branch)
- [ ] Manual Level-3 plumbing check matches the working-tree-domain expectations

### Code Quality Validation

- [ ] Follows existing codebase patterns (TreeDiff/StagedDiff three-part shape) and naming
- [ ] File placement matches the one-file-per-method test convention (NEW workingtreediff_test.go)
- [ ] Anti-patterns avoided (no --cached/HEAD, no --quiet, no 128 special-case, no ls-files for untracked)
- [ ] Consumed helpers (run/binary.go/StagedDiff/TreeDiff/StatusPorcelain) UNCHANGED
- [ ] go.mod/go.sum UNCHANGED

### Documentation & Deployment

- [ ] Interface doc comment names the command, the §13.6.2 role, the exit-code convention, the FR3c filtering,
      the read-only contract, AND the untracked-files gotcha
- [ ] Method doc comment explains it is a port of TreeDiff + the working-tree domain
- [ ] No new env vars / config keys / CLI flags (internal plumbing addition)

---

## Anti-Patterns to Avoid

- ❌ Don't add `--cached` (that captures the STAGED diff — StagedDiff's job; the contract is working-tree-vs-index).
- ❌ Don't add `HEAD` as a positional (that is working-tree-vs-HEAD — a DIFFERENT domain; the contract is
  working-tree-vs-INDEX).
- ❌ Don't "fix" the untracked-files omission with `git ls-files --others` / `--exclude-standard` / `git
  add -A`-then-diff — the work item explicitly specifies `git diff` WITHOUT --cached, and resolving
  untracked visibility is a decompose-orchestrator (P3) decision, out of scope for this plumbing task.
- ❌ Don't use `--quiet` (that is HasStagedChanges' exit-inversion trick; irrelevant to a list-returning
  diff). Don't add a `if code == 128` special-case (that is RevParseHEAD's unborn convention; `git diff`
  exits 0 on unborn repos).
- ❌ Don't modify `run()`, `binary.go`, `StagedDiff`, `TreeDiff`, `StatusPorcelain`, or any consumed file —
  they are consumed, not owned by this task.
- ❌ Don't append binary excludes to the `excludes` local (it may alias `defaultExcludes` — mutating a
  package var). Use a SEPARATE `binExcludes` slice (copy from TreeDiff verbatim).
- ❌ Don't redefine any package-level test helper (initRepo/writeFile/stageFile/execGit/makeEmptyCommit/
  sdManyLines) — redefining = duplicate-symbol compile error.
- ❌ Don't skip the `_UntrackedFilesOmitted` / `_NotARepo` test pair — together they pin the git-diff
  working-tree-domain contract (untracked omitted by design; non-repo is a real error).

---

## Confidence Score

**9/10** for one-pass implementation success. WorkingTreeDiff is a mechanical port of the ALREADY-SHIPPED
`TreeDiff` (P2.M2.T1.S2): copy its body verbatim, drop the `treeA treeB` positionals from the four
git-diff arg slices, and call `detectBinaryFiles(ctx)` / `fileStatuses(ctx)` with empty diffArgs (the
`binary.go` package doc names this exact consumer: `WorkingTreeDiff: diffArgs = []`). Every shared concern
(caps, excludes, binary filtering, truncation, double-count prevention) is already solved identically in
TreeDiff/StagedDiff — there is no new logic. The one non-obvious characteristic (untracked files omitted by
`git diff`) is the EXPLICIT contract (named 3× across the work item + architecture doc + binary.go), fully
resolved by the empirical findings, and pinned by a dedicated test. No new types, no new deps, no caller
wiring, no consumed-file edits — purely additive. The -1 accounts for the parallel-StatusPorcelain append
merge friction at the interface closing brace (low risk, both are independent additive lines).
