---
name: "P1.M2.T1.S1 — buildDiffArgs helper + refactor 3 diff functions"
description: |
  Pure refactor (FR3e/FR3f/FR3i ENABLER — no behavior change). Extract a shared leading-token builder
  `buildDiffArgs(domain ...string) []string` in `internal/git/git.go` and route the THREE sibling diff
  functions — `StagedDiff`, `TreeDiff`, `WorkingTreeDiff` — through it at all NINE argv-construction
  sites (3 per function: the markdown name-list run, the markdown per-file run, and the non-markdown
  aggregate `nmArgs`). The three functions are near-verbatim copies whose ONLY difference is the diff
  domain positional args (`--cached` / `treeA treeB` / nothing); today each duplicates the `"diff",
  <domain>…` prefix inline. The helper centralizes that prefix so T2 (P1.M2.T2.S1: inject `-M` +
  `-U<diff_context>`) and M3/M4 (numstat skeleton, water-fill truncation) become single-site edits instead
  of a 9-way scatter. **OUTPUT MUST BE BYTE-IDENTICAL** — the existing golden suites
  (`stagediff_test.go` 23 tests, `treediff_test.go` 12 tests, `workingtreediff_test.go` N tests) MUST pass
  unchanged. No new flags are added or removed; the helper returns ONLY `["diff", domain…]`.

  ⚠️ **THE central design call — the helper is MINIMAL: it standardizes the LEADING TOKEN only.**
  `func buildDiffArgs(domain ...string) []string { return append([]string{"diff"}, domain…) }`. It does
  NOT inject `-M`/`-U` (that is T2), does NOT alter excludes, does NOT consume the new
  `StagedDiffOptions` fields (`TokenLimit`/`DiffContext`/`PromptReserveTokens` — UNREAD here, behavior-free).
  Adding anything beyond the leading token would break byte-identity. The variadic `domain` shape is kept
  precisely so T2 can insert `-M`/`-U` after `"diff"` in ONE place.

  ⚠️ **THE second design call — name it `buildDiffArgs`, not `diffArgs`.** The task TITLE and plan_status
  task name both say `buildDiffArgs`; the item_description LOGIC snippet wrote `diffArgs`. `buildDiffArgs`
  wins because (1) it is the canonical task identity, (2) `diffArgs` is ALREADY a parameter name in
  `internal/git/binary.go` (`detectBinaryFiles(ctx, diffArgs …string)`, `fileStatuses(ctx, diffArgs …)`) —
  a package-level `diffArgs` function would be shadowed inside those (confusing for readers), and (3)
  `buildX` is the idiomatic Go builder convention. The signature/body are exactly as the contract specifies.

  ⚠️ **THE third design call — byte-identical transformation per site; `binary.go` is NOT a refactor
  target.** Each of the 9 sites swaps its inline `"diff", <domain>…` prefix for `buildDiffArgs(<domain>…)…`
  and keeps its EXACT current trailing tokens (`--name-only -- *.md *.markdown` / `-- <file>` / `-- excludes
  :!*.md :!*.markdown binExcludes`). Token-for-token == today's argv ⇒ identical stdout. The proven variadic
  pattern in `binary.go` (`detectBinaryFiles`/`fileStatuses`) is the PATTERN TO MIRROR, NOT a refactor
  target — those keep building their own `["diff", …, "--numstat"/"--name-status"]` and are called from the
  3 functions with the domain (`"--cached"`/`treeA treeB`/nothing) UNCHANGED. (T2 may later extend -M/-U to
  them; out of scope here.)

  SCOPE: edit `internal/git/git.go` ONLY (add `buildDiffArgs` immediately before `StagedDiff`; rewrite the
  9 argv sites — 3 each in StagedDiff/TreeDiff/WorkingTreeDiff). NO new tests (the golden suites already
  pin byte-identical output; a pure refactor adds no behavior to test), NO docs, NO `binary.go`, NO
  call-site edits. INPUT = the 3 diff functions as they stand. OUTPUT = a single shared helper; the 3
  functions route through it; stdout byte-identical; T2/M3/M4 enabled as single-site edits.
---

## Goal

**Feature Goal**: Eliminate the triplicated `git diff` leading-token construction across the three sibling
diff functions by extracting one `buildDiffArgs(domain ...string) []string` helper, routing all 9
argv-construction sites (3 per function) through it, and producing BYTE-IDENTICAL diff output — so the
upcoming FR3e/FR3f (`-M`/`-U`) and FR3g/FR3i (numstat skeleton, water-fill) changes land in ONE place
instead of nine.

**Deliverable** (edits to ONE file):
1. **`internal/git/git.go`** — (a) add `func buildDiffArgs(domain ...string) []string { return
   append([]string{"diff"}, domain…) }` immediately before `StagedDiff` (the first of the three), with a
   doc comment naming it the shared leading-token builder + that T2 extends it with `-M`/`-U`;
   (b) rewrite the 9 argv sites — StagedDiff (`buildDiffArgs("--cached")`), TreeDiff
   (`buildDiffArgs(treeA, treeB)`), WorkingTreeDiff (`buildDiffArgs()`) — each site's md-list / per-file /
   nmArgs invocation builds its base via the helper then appends its existing trailing tokens verbatim.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean;
`go test ./internal/git/` green with the golden suites (`TestStagedDiff_*`, `TestTreeDiff_*`,
`TestWorkingTreeDiff_*`) passing UNCHANGED — byte-identical stdout (the refactor adds/removes no flag, no
token). `go test ./...` green (no regression). go.mod/go.sum unchanged; only `internal/git/git.go` touched;
no new files, no new tests, no docs.

## User Persona

**Target User**: The NEXT subtasks (T2 P1.M2.T2.S1 inject `-M`/`-U`; M3 numstat skeleton; M4 water-fill) and
maintainers — transitively FR3e/FR3f/FR3g/FR3i (every `git diff` invocation gets rename detection, reduced
context, the skeleton, truncation). This refactor is the foundation that makes those single-site edits.

**Use Case**: A maintainer adding `-M` (FR3e) edits `buildDiffArgs` once; today they would have to edit 9
near-identical sites across 3 functions and risk divergence (exactly the central refactor risk called out in
`architecture/diff_capture_touchmap.md §1`).

**User Journey**: (internal refactor, no user-visible change) `StagedDiff`/`TreeDiff`/`WorkingTreeDiff` →
each `g.run` builds its argv via `buildDiffArgs(domain…)` + trailing tokens → identical `git diff` stdout.

**Pain Points Addressed**: removes the "any new flag/filter must be applied to all three" duplication hazard
so future FR3e–FR3i work can't silently miss a path.

## Why

- **Enables T2/M3/M4 as single-site edits.** Without this, `-M`/`-U`/skeleton/truncation each land in 9
  places across 3 near-verbatim functions — divergence-prone. The helper is the chokepoint.
- **Zero risk.** A pure, byte-identical refactor guarded by the existing golden suites (35+ diff tests).
  No behavior, no flags, no options consumed, no output change.
- **Matches the proven pattern.** `binary.go`'s `detectBinaryFiles`/`fileStatuses` already centralize their
  own `["diff", diffArgs…]` construction via a variadic; this extends the same discipline to the 3 inline
  diff sites.
- **Tiny.** One ~3-line helper + 9 mechanical argv rewrites in one file; no new tests/docs/deps.

## What

One new helper in `internal/git/git.go`; the three diff functions construct every `git diff` argv through
it; diff stdout is byte-identical. No options consumed, no flags added, no `binary.go`/call-site/doc changes.

### Success Criteria

- [ ] `buildDiffArgs(domain ...string) []string` exists in `internal/git/git.go` (immediately before
      `StagedDiff`), returning `append([]string{"diff"}, domain…)`, with a doc comment.
- [ ] All 9 argv sites route through it: StagedDiff md-list (~686), per-file (~697), nmArgs (771-775);
      TreeDiff md-list (~1136), per-file (~1147), nmArgs (1210-1214); WorkingTreeDiff md-list (~1270),
      per-file (~1281), nmArgs (1345-1349). Each appends its EXACT current trailing tokens.
- [ ] Domain per function: StagedDiff `buildDiffArgs("--cached")`; TreeDiff `buildDiffArgs(treeA, treeB)`;
      WorkingTreeDiff `buildDiffArgs()`.
- [ ] `binary.go`'s `detectBinaryFiles`/`fileStatuses` are UNCHANGED (pattern to mirror, not a target);
      their callers in the 3 functions still pass the domain verbatim.
- [ ] `go test ./internal/git/` green — `TestStagedDiff_*` (23), `TestTreeDiff_*` (12), `TestWorkingTreeDiff_*`
      pass UNCHANGED (byte-identical stdout); `go test ./...` green; `go vet`/`gofmt` clean; go.mod/go.sum
      unchanged; only `internal/git/git.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the 9-site table below (with exact
line numbers + the per-site transformation), the helper body, and the domain cheat-sheet. No PRD/git-internals
knowledge required — this is a mechanical, byte-preserving argv refactor.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/007_b33d310438c6/P1M2T1S1/research/buildDiffArgs_helper.md
  why: the 9 exact argv sites (line numbers), the byte-identical transformation per site, the naming
       decision (buildDiffArgs vs diffArgs), the "minimal helper" rule, and the scope boundary.
  critical: the helper returns ONLY ["diff", domain…]. Add NOTHING else (no -M/-U/excludes) or you break
       byte-identity. binary.go is the pattern to mirror, NOT a refactor target.

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  section: "1. The THREE sibling diff functions (FR3c parity)"
  why: the scouting map — confirms the 3 functions are near-verbatim copies differing only in the domain
       args, lists the exact nmArgs/md-list/per-file shapes per function, and names the central refactor
       risk (any new flag must hit all three OR a helper is extracted first — this task is the extraction).
  critical: the domain positional args per function: StagedDiff=`--cached`, TreeDiff=`treeA treeB`,
       WorkingTreeDiff=(none).

- file: internal/git/git.go
  section: StagedDiff (~670-790), TreeDiff (~1122-1215), WorkingTreeDiff (~1256-1350)
  why: the 3 functions you edit. Each has exactly 3 `"diff"` literals (md-list, per-file, nmArgs) —
       confirmed by awk count (3/3/3). Place the helper immediately before StagedDiff.
  pattern: nmArgs uses a named local (`nmArgs := …; nmArgs = append(…)`); md-list/per-file pass inline
           literals to `g.run(ctx, g.workDir, "diff", …)`. After refactor: build base via buildDiffArgs,
           append trailing tokens, pass `slice…` to g.run (variadic — identical args inside run).
  gotcha: the per-file site is INSIDE a loop — build the base fresh each iteration
          (`append(buildDiffArgs(domain…), "--", file)…`), NOT hoisted+mutated (avoids any backing-array
          aliasing doubt). g.run consumes the slice before the next iteration, but fresh-per-iteration is
          obviously safe.

- file: internal/git/binary.go
  section: detectBinaryFiles (~98), fileStatuses (~130)
  why: the PROVEN variadic pattern to mirror: `args := make([]string, 0, 1+len(diffArgs)+1); args = append
       (args, "diff"); args = append(args, diffArgs…); args = append(args, "--numstat"/"--name-status")`.
       These are NOT refactor targets — they keep building their own argv and are called from the 3
       functions with the domain verbatim.
  gotcha: do NOT rename or alter these; do NOT route them through buildDiffArgs (out of scope; T2 may
          extend -M/-U coverage there later).

- file: internal/git/stagediff_test.go + treediff_test.go + workingtreediff_test.go
  why: the GOLDEN SUITES — the byte-identical safety net. 23 + 12 + N tests pinning exact stdout
       (TestStagedDiff_MarkdownAndCode, _ExcludesLockSnapMapVendor, _MarkdownNotDoubleCounted,
       _BinaryFilePlaceholderAndExcluded, _NonMarkdownByteCap; TestTreeDiff_BasicConceptDiff,
       _ExcludesApplied, _BinaryPlaceholderAndExcluded; TestWorkingTreeDiff_BasicWorkingTreeDiff, …).
  pattern: these tests MUST pass UNCHANGED — you add NO new tests (a pure refactor has no new behavior to
           pin). If any fails, the refactor changed a token; fix the transformation, do NOT touch the test.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # StagedDiff (~670), TreeDiff (~1122), WorkingTreeDiff (~1256) — EDIT (add helper + 9 sites)
  binary.go           # detectBinaryFiles/fileStatuses (variadic pattern to mirror) — NO edit
  stagediff_test.go   # 23 golden tests (byte-identical safety net) — NO edit (must pass unchanged)
  treediff_test.go    # 12 golden tests — NO edit
  workingtreediff_test.go # N golden tests — NO edit
go.mod / go.sum       # unchanged
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE edit: internal/git/git.go (helper + 9 argv rewrites).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the helper returns ONLY append([]string{"diff"}, domain…). Add NO -M, NO -U, NO excludes,
// NO skeleton — this subtask standardizes the LEADING TOKEN ONLY. Anything more breaks byte-identity
// (T2/M3/M4 add the rest via the helper later). The new StagedDiffOptions fields are UNREAD here.

// CRITICAL: byte-identity is token-for-token. StagedDiff md-list today is
//   ["diff","--cached","--name-only","--","*.md","*.markdown"]
// after refactor: buildDiffArgs("--cached") => ["diff","--cached"]; append trailing 4 => SAME 6 tokens.
// Verify this shape mentally for every site. g.run is variadic ⇒ slice... == inline literals inside run.

// CRITICAL: name it buildDiffArgs, NOT diffArgs. diffArgs is already a parameter in binary.go
// (detectBinaryFiles/fileStatuses) — a package-level diffArgs function would shadow it. buildDiffArgs
// matches the task title + plan_status and the buildX convention.

// GOTCHA: the per-file site is in a loop. Build the base FRESH each iteration
// (`append(buildDiffArgs(domain...), "--", file)...`), do NOT hoist a shared base and append into it
// (buildDiffArgs may return a slice with spare capacity; appending in a loop could mutate its backing
// array). Fresh-per-iteration is obviously correct and byte-identical.

// GOTCHA: do NOT touch binary.go's detectBinaryFiles/fileStatuses or their call sites in the 3 functions
// (`g.detectBinaryFiles(ctx, "--cached")`, `g.fileStatuses(ctx, treeA, treeB)`, `g.detectBinaryFiles(ctx)`,
// `g.detectExcludedStatuses(...)`). They already use the variadic domain pattern; routing them through
// buildDiffArgs is out of scope (and would change nothing about THEIR output). T2 decides -M/-U coverage
// for numstat/name-status.

// GOTCHA: WorkingTreeDiff's domain is EMPTY — `buildDiffArgs()` returns `["diff"]` (append([]string{"diff"})
// with no variadic). Its md-list becomes `append(buildDiffArgs(), "--name-only", "--", "*.md", "*.markdown")…`
// == today's `["diff","--name-only","--","*.md","*.markdown"]`. Verify the empty-domain case explicitly.

// GOTCHA: place the helper immediately BEFORE StagedDiff (the first diff function) with a doc comment —
// not in a separate file, not at package top (keeps it next to its 3 consumers; the doc names T2 as the
// next extender).
```

## Implementation Blueprint

### Data models and structure

No new types. The single helper:

```go
// buildDiffArgs returns the leading argv slice for a `git diff` invocation: ["diff", domain…].
// It is the shared leading-token builder for the three sibling diff functions (StagedDiff/TreeDiff/
// WorkingTreeDiff), which differ only in the diff-domain positional args (--cached / treeA treeB / none).
// Centralizing the prefix lets FR3e/FR3f (-M / -U<diff_context>) and later FR3g/FR3i changes land in ONE
// place instead of nine. domain is the verbatim diff domain: ("--cached"), (treeA, treeB), or ().
// Pure function; no I/O. (This subtask standardizes the leading token ONLY — T2 injects -M/-U here.)
func buildDiffArgs(domain ...string) []string {
	return append([]string{"diff"}, domain...)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — add buildDiffArgs immediately before StagedDiff
  - ADD the helper per the Data Models block, right before `func (g *gitRunner) StagedDiff(...)`.
  - NAME: buildDiffArgs (NOT diffArgs — see gotcha). Signature `(domain ...string) []string`.
  - BODY: `return append([]string{"diff"}, domain...)` — NOTHING else.
  - DOC: name it the shared leading-token builder; note T2 (P1.M2.T2.S1) injects -M/-U here.

Task 2: git.go — refactor StagedDiff's 3 sites (domain = "--cached")
  - md-list (~686): replace `g.run(ctx, g.workDir, "diff", "--cached", "--name-only", "--", "*.md", "*.markdown")`
    with a base built via `buildDiffArgs("--cached")` + append the 4 trailing tokens, passed as `slice...`.
  - per-file (~697, INSIDE the loop): replace `"diff", "--cached", "--", file` with
    `append(buildDiffArgs("--cached"), "--", file)…` (fresh per iteration).
  - nmArgs (771-775): replace `nmArgs := []string{"diff", "--cached", "--"}` with
    `nmArgs := buildDiffArgs("--cached"); nmArgs = append(nmArgs, "--")`; keep the subsequent appends
    (excludes… / ":!*.md", ":!*.markdown" / binExcludes…) UNCHANGED.
  - VERIFY token-equivalence vs the current argv (6 / 4 / 2+excludes+2+binExcludes tokens).

Task 3: git.go — refactor TreeDiff's 3 sites (domain = treeA, treeB)
  - md-list (~1136): `buildDiffArgs(treeA, treeB)` + "--name-only","--","*.md","*.markdown".
  - per-file (~1147, in loop): `append(buildDiffArgs(treeA, treeB), "--", file)…`.
  - nmArgs (1210-1214): `nmArgs := buildDiffArgs(treeA, treeB); nmArgs = append(nmArgs, "--")`; rest unchanged.

Task 4: git.go — refactor WorkingTreeDiff's 3 sites (domain = EMPTY)
  - md-list (~1270): `buildDiffArgs()` + "--name-only","--","*.md","*.markdown".
  - per-file (~1281, in loop): `append(buildDiffArgs(), "--", file)…`.
  - nmArgs (1345-1349): `nmArgs := buildDiffArgs(); nmArgs = append(nmArgs, "--")`; rest unchanged.
  - GOTCHA: confirm `buildDiffArgs()` == `["diff"]` (empty variadic) so the empty-domain argv matches today.

Task 5: VERIFY (no further file change)
  - RUN the Validation Loop. The golden suites (TestStagedDiff_*/TestTreeDiff_*/TestWorkingTreeDiff_*) MUST
    pass unchanged. go.mod/go.sum byte-unchanged. ONLY internal/git/git.go touched. No binary.go, no tests,
    no call sites, no docs.
```

### Implementation Patterns & Key Details

```go
// The helper — minimal, variadic, the single future injection point:
func buildDiffArgs(domain ...string) []string {
	return append([]string{"diff"}, domain...)
}

// md-list site pattern (StagedDiff shown; TreeDiff uses treeA,treeB; WorkingTreeDiff uses empty):
mdList, stderr, code, err := g.run(ctx, g.workDir,
	append(buildDiffArgs("--cached"), "--name-only", "--", "*.md", "*.markdown")...)

// per-file site pattern (inside the loop; fresh base each iteration):
fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir,
	append(buildDiffArgs("--cached"), "--", file)...)

// nmArgs site pattern (named local, mirroring today's style):
nmArgs := buildDiffArgs("--cached")
nmArgs = append(nmArgs, "--")
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure refactor, no new dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - internal/git/binary.go (detectBinaryFiles/fileStatuses — the variadic pattern to mirror, NOT a target).
  - The detectBinaryFiles/fileStatuses/detectExcludedStatuses CALL SITES inside the 3 functions
    (g.detectBinaryFiles(ctx, "--cached") etc.) — pass the domain verbatim, unchanged.
  - internal/git/{stagediff,treediff,workingtreediff,difftree,difftreenames}_test.go — the golden suites
    (must pass UNCHANGED; add NO new tests).
  - The 6 production call sites (generate/hook/stagehand/decompose) — P1.M1.T2.S2 owns the
    StagedDiffOptions struct-literal field mapping; this task is function-internal only.
  - docs/* (no user-facing change; P1.M5 owns the diff-capture doc sync).

DOWNSTREAM ENABLED (do NOT implement here):
  - P1.M2.T2.S1 (T2): inject `-M` + `-U<diff_context>` into buildDiffArgs (ONE place) + update golden
    fixtures (output WILL change there — that's T2's behavior, gated by its own tests).
  - P1.M3 (numstat skeleton), P1.M4 (water-fill): build on this centralized argv site.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/git/git.go
test -z "$(gofmt -l internal/git/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/git/        # Catches a stray unused var / a broken append.
go build ./...                # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If go vet flags buildDiffArgs as unused, you missed a site — all 9 must route through it
# (and the helper IS used once you touch the first site).
```

### Level 2: Unit Tests (Component Validation) — THE byte-identical gate

```bash
go test ./internal/git/ -v -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff'
# Expected: ALL PASS UNCHANGED (23 + 12 + N golden tests). These pin exact stdout — if ANY fails, the
#   refactor changed a token; re-check the transformation (do NOT edit the test).
go test -race ./internal/git/      # the full git suite (incl. difftree/difftreenames/binary tests)
go test -race ./...                # full module — no regression (the 3 diff functions feed generate/decompose)
# Expected: green throughout. This refactor adds NO new behavior, so NO new test is warranted — the golden
# suites ARE the regression net.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only internal/git/git.go changed:
git diff --name-only | grep -Ev '^internal/git/git\.go$' && echo "UNEXPECTED file changed" || echo "only internal/git/git.go changed (good)"
# Byte-identity smoke (optional but recommended): capture StagedDiff/TreeDiff/WorkingTreeDiff stdout on a
# fixture repo BEFORE (git stash the refactor) and AFTER, then diff them — expect empty (identical). The
# golden suites already prove this; the smoke is belt-and-suspenders.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep audit: confirm NO inline `"diff", "--cached"` / `"diff", treeA` / `"diff", "--"` argv literal
# remains in the 3 functions (every site now starts from buildDiffArgs), AND that buildDiffArgs is the
# ONLY new `"diff"`-prefixing site:
grep -n '"diff"' internal/git/git.go   # the 3 functions' sites should now go through buildDiffArgs; binary.go
                                       # is a separate file. Expect the helper + g.run calls using buildDiffArgs.
# golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test -race ./internal/git/` (golden suites unchanged) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `internal/git/git.go` touched.

### Feature Validation

- [ ] `buildDiffArgs(domain ...string) []string` exists, returns `append([]string{"diff"}, domain…)`, placed
      before StagedDiff, doc-commented (names T2 as the -M/-U extender).
- [ ] All 9 argv sites (3 per function) route through it with the correct domain (StagedDiff `--cached`;
      TreeDiff `treeA treeB`; WorkingTreeDiff empty) and EXACT current trailing tokens.
- [ ] Byte-identical: `TestStagedDiff_*` / `TestTreeDiff_*` / `TestWorkingTreeDiff_*` pass UNCHANGED.

### Code Quality Validation

- [ ] Mirrors `binary.go`'s variadic `["diff", domain…]` discipline; per-file loop builds the base fresh
      each iteration (no shared-backing-array mutation).
- [ ] `binary.go` UNCHANGED; `detectBinaryFiles`/`fileStatuses` call sites in the 3 functions unchanged.
- [ ] No scope creep into -M/-U (T2), excludes, numstat skeleton (M3), water-fill (M4), options fields,
      call sites (P1.M1.T2.S2), or docs (P1.M5).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (internal refactor; P1.M5 owns the diff-capture doc sync).
- [ ] go.mod/go.sum byte-unchanged; no new files; no new tests.

---

## Anti-Patterns to Avoid

- ❌ Don't add ANYTHING to `buildDiffArgs` beyond `append([]string{"diff"}, domain…)`. No `-M`, no `-U`, no
  excludes, no skeleton — this task standardizes the LEADING TOKEN ONLY. Anything else breaks byte-identity
  and steals T2/M3/M4's scope.
- ❌ Don't name it `diffArgs`. That collides with the `diffArgs` parameter in `binary.go`'s
  `detectBinaryFiles`/`fileStatuses` (shadowing). Use `buildDiffArgs` (task title + plan_status + `buildX`
  convention).
- ❌ Don't touch `binary.go` or the `detectBinaryFiles`/`fileStatuses`/`detectExcludedStatuses` call sites.
  They are the variadic pattern to MIRROR, not refactor targets; their output is unchanged. T2 decides
  whether -M/-U extend to numstat/name-status.
- ❌ Don't hoist a shared `buildDiffArgs` base outside the per-file loop and `append` into it each iteration
  — the returned slice may have spare capacity and a loop append could mutate its backing array. Build fresh
  per iteration (`append(buildDiffArgs(domain…), "--", file)…`).
- ❌ Don't change the trailing tokens at any site. md-list keeps `--name-only -- *.md *.markdown`; per-file
  keeps `-- <file>`; nmArgs keeps `-- excludes :!*.md :!*.markdown binExcludes`. Byte-identity is token-for-token.
- ❌ Don't forget the EMPTY-domain case (WorkingTreeDiff): `buildDiffArgs()` must yield exactly `["diff"]`,
  so its md-list becomes `["diff","--name-only","--","*.md","*.markdown"]` as today. Verify explicitly.
- ❌ Don't add new tests. A pure refactor has no new behavior to pin; the existing golden suites are the
  regression net. (If you feel a test urge, direct it at T2's -M/-U behavior, not here.)
- ❌ Don't edit the 6 production call sites (generate/hook/stagehand/decompose) — that's P1.M1.T2.S2's
  StagedDiffOptions field-mapping scope. This task is strictly function-internal.
- ❌ Don't change go.mod/go.sum or add files. One file, one helper, nine rewrites.
- ❌ Don't skip `go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff'` — it is THE
  byte-identical gate; a single changed token fails it and tells you exactly which site to re-check.
