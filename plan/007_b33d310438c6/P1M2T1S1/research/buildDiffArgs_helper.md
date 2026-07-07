# Research: buildDiffArgs helper + refactor 3 diff functions (P1.M2.T1.S1)

Verified against the live codebase. Source of truth for the pure refactor (byte-identical output).

## The 9 argv-construction sites (3 per function) — ALL must route through the helper

Each of the three sibling diff functions in `internal/git/git.go` builds the SAME three `git diff`
invocations inline, differing ONLY in the leading domain tokens after `"diff"`:

| Function | Domain (positional args after "diff") | md-list site | per-file site | nmArgs site |
|---|---|---|---|---|
| `StagedDiff` (~670) | `"--cached"` | ~686 | ~697 | 771-775 |
| `TreeDiff` (~1122) | `treeA, treeB` | ~1136 | ~1147 | 1210-1214 |
| `WorkingTreeDiff` (~1256) | *(none)* | ~1270 | ~1281 | 1345-1349 |

Confirmed: exactly **3 `"diff"` literals per function** (awk count = 3/3/3) — no other inline diff
invocations to miss.

### Current shape (verbatim)

md-list (StagedDiff): `g.run(ctx, g.workDir, "diff", "--cached", "--name-only", "--", "*.md", "*.markdown")`
per-file (StagedDiff): `g.run(ctx, g.workDir, "diff", "--cached", "--", file)`
nmArgs (StagedDiff):
```go
nmArgs := []string{"diff", "--cached", "--"}
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
nmDiff, … := g.run(ctx, g.workDir, nmArgs...)
```
TreeDiff is identical with `"--cached"` → `treeA, treeB`; WorkingTreeDiff with the domain omitted.

## The byte-identical transformation

`buildDiffArgs(domain ...string) []string` returns `append([]string{"diff"}, domain...)`. Then each site
prepends the helper's result and appends its EXACT current trailing tokens:

- **md-list**: `append(buildDiffArgs(<domain>...), "--name-only", "--", "*.md", "*.markdown")…`
- **per-file** (inside the loop): `append(buildDiffArgs(<domain>...), "--", file)…`
- **nmArgs**: `nmArgs := buildDiffArgs(<domain>…); nmArgs = append(nmArgs, "--"); nmArgs = append(nmArgs, excludes…); nmArgs = append(nmArgs, ":!*.md", ":!*.markdown"); nmArgs = append(nmArgs, binExcludes…)`

Token-by-token == the current argv (e.g. StagedDiff md-list → `["diff","--cached","--name-only","--","*.md","*.markdown"]`
before AND after). `g.run` is variadic, so passing `slice...` vs inline literals yields the identical args
slice inside `run`. ⇒ **byte-identical stdout**, pinned by the golden suites.

## ⚠️ Naming: `buildDiffArgs` (title + plan_status) vs `diffArgs` (description LOGIC)

The task TITLE and plan_status task name both say **`buildDiffArgs`**; the item_description LOGIC snippet
writes `func diffArgs(...)`. RECOMMEND `buildDiffArgs` because: (1) it matches the canonical task identity
(title + plan_status); (2) `diffArgs` is ALREADY a parameter name in `internal/git/binary.go`
(`detectBinaryFiles(ctx, diffArgs …string)`, `fileStatuses(ctx, diffArgs …string)`) — a package-level
function named `diffArgs` would be shadowed inside those (not a bug, but confusing for readers); (3)
`buildX` is the idiomatic Go builder-helper convention. The signature is otherwise exactly as specified:
`func buildDiffArgs(domain ...string) []string { return append([]string{"diff"}, domain…) }`.

## The variadic pattern to mirror (NOT a refactor target)

`internal/git/binary.go` `detectBinaryFiles`/`fileStatuses` (~98/130) already use the proven
`args = append(args, "diff"); args = append(args, diffArgs…); args = append(args, "--numstat"/"--name-status")`
shape with a `domain ...string` (called `diffArgs` there) variadic. These are the PATTERN reference; they
are NOT refactored by this task (the contract scopes the helper to the 3 inline sites per function:
md-list / per-file / nmArgs). `detectBinaryFiles`/`fileStatuses` keep building their own `["diff", …]`
internally and are called from the 3 functions with the domain (`"--cached"` / `treeA, treeB` / nothing) —
UNCHANGED. (T2 may later extend -M/-U coverage to them, but that is out of scope here.)

## Minimal helper — NO -M/-U, NO excludes, NO DiffContext

This subtask STANDARDIZES THE LEADING TOKEN ONLY. The helper returns `["diff", domain…]` — nothing more.
T2 (P1.M2.T2.S1) injects `-M` + `-U<diff_context>` in this ONE place; M3/M4 add the numstat skeleton /
water-fill. Keeping the helper minimal now guarantees byte-identical output (no flag is added or removed).
The new `StagedDiffOptions` fields (TokenLimit/DiffContext/PromptReserveTokens from P1.M1.T2.S1, populated
at call sites by P1.M1.T2.S2) are UNREAD by this refactor — behavior-free.

## Safety net — the golden suites (byte-identical)

- `internal/git/stagediff_test.go` — 23 tests (TestStagedDiff_MarkdownAndCode, _ExcludesLockSnapMapVendor,
  _MarkdownNotDoubleCounted, _MarkdownLineCap, _NonMarkdownByteCap, _BinaryFilePlaceholderAndExcluded, …).
- `internal/git/treediff_test.go` — 12 tests (TestTreeDiff_BasicConceptDiff, _ExcludesApplied,
  _BinaryPlaceholderAndExcluded, _NonMarkdownByteCap, …).
- `internal/git/workingtreediff_test.go` — N tests (TestWorkingTreeDiff_BasicWorkingTreeDiff,
  _CleanWorkingTree, …).

These pin exact stdout. `go test ./internal/git/` is the regression gate: if the refactor changes even one
byte, they fail. They MUST pass unchanged (the PRP's success criterion).

## Scope boundary (no conflict)

- **P1.M1.T2.S2 (parallel)** maps cfg fields at the 6 production CALL-SITE struct literals
  (generate/hook/stagecoach/decompose) + adds `Config.DiffContextValue()`. It does NOT touch the 3 diff
  function internals → ZERO overlap with this refactor.
- **T2 (next)** injects -M/-U via this helper (single-site edit, enabled by this refactor).
- This task: ONLY `internal/git/git.go` (add the helper + edit 9 argv sites). No tests added (the golden
  suites already pin output; a pure refactor needs no new test), no docs, no binary.go, no call sites.

## Per-file domain cheat-sheet (for the edit)

- StagedDiff: `buildDiffArgs("--cached")`
- TreeDiff: `buildDiffArgs(treeA, treeB)`
- WorkingTreeDiff: `buildDiffArgs()`
