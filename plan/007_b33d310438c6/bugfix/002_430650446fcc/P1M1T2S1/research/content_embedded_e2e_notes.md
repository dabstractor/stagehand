# Research: E2E Content-Embedded `diff --git` Literal Regression (P1.M1.T2.S1, bugfix 002)

> **Purpose:** Pin the exact, empirically-verified E2E regression tests for the Issue-1 fix
> (`splitDiffSections` line-anchoring — bugfix 002), checked against the live codebase on 2026-07-04.
> **The S1 fix is ALREADY LANDED** (`internal/git/truncatediff.go:55` `diffSectionBoundaryRe = (?m)^diff --git `,
> `splitDiffSections` rewritten to `FindAllStringIndex` slicing — Shape B); baseline `go test ./internal/git/`
> is GREEN. These tests PIN the fixed behavior. The test design (one non-md file whose content embeds many
> `diff --git ` literals, under a small TokenLimit) is PROVEN end-to-end against the real git binary for ALL
> THREE diff functions — every contract assertion holds.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagehand`, `go 1.22` |
| Edit target | `internal/git/difftokenlimit_test.go` (add 3 top-level test funcs + 1 assertion helper + 1 body-builder helper) |
| S1 fix status | **LANDED** — `truncatediff.go:55` `var diffSectionBoundaryRe = regexp.MustCompile("(?m)^diff --git ")`; `splitDiffSections` (lines ~85-120) uses `FindAllStringIndex` slicing (Shape B, NO re-prefix). The un-anchored `strings.Split(diff, "diff --git ")` is GONE. |
| Baseline | `go test ./internal/git/` → **ok (8.374s)** |
| Budget constant | `tokenBudgetMargin = 1024` (tokengate.go:48). The existing fits-budget ceiling: `EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin`. |
| Prior PRP (S1) | Production fix in `truncatediff.go` + pure unit cases in `truncatediff_test.go` (`TestSplitDiffSections`). Explicitly leaves `difftokenlimit_test.go` to THIS task (S2). **No conflict.** |

---

## 2. The Gap These Tests Close (Issue 1, bugfix 002)

`splitDiffSections` split the non-markdown aggregate on the **un-anchored** substring `"diff --git "`, so a
content line `+diff --git a/foo b/foo` (a fixture/doc/snapshot/.patch embedding a sample diff) was torn into
a bogus section boundary → one real file fragmented into many tiny bogus sections → water-fill sized them
tiny → truncated nothing → payload **silently overflowed** `token_limit` (~7× measured at token_limit=2000),
breaking FR3d's "always fits" contract. The existing `difftokenlimit_test.go` tests stage content free of
`diff --git ` literals, so the fragmentation path was never exercised E2E. These tests stage a non-md file
whose CONTENT embeds many `diff --git ` literals and assert the file is ONE section (not fragmented),
truncated to fit.

---

## 3. The Test Design — PROVEN against the real git binary

**Setup:** ONE non-markdown file (`fixtures.diff`) whose content is 300 documented `diff --git ` blocks
(`strings.Repeat`-style loop, sample names fa..fz cycling), under `StagedDiffOptions{TokenLimit: 2000,
PromptReserveTokens: 0}`. The file body (~9000+ tokens) far exceeds the body budget, so the water-fill
caps it; the line-anchored split keeps it ONE section.

**Empirical proof** (throwaway probe against the live `internal/git` package, all three diff functions):

| Assertion | StagedDiff | TreeDiff | WorkingTreeDiff |
|---|---|---|---|
| (a) `EstimateTokens(out) <= tokenLimit + 2*1024` (fits budget) | ✅ 1005 ≤ 4048 | ✅ 1005 ≤ 4048 | ✅ 1001 ≤ 4048 |
| (b) `Count(out, "... [truncated]") == 1` (truncated) | ✅ 1 | ✅ 1 | ✅ 1 |
| (c) `Count(out, "diff --git a/fixtures.diff b/fixtures.diff") == 1` (NOT fragmented) | ✅ 1 | ✅ 1 | ✅ 1 |
| (d) `Contains(out, "Change summary (numstat")` (FR3g skeleton) | ✅ true | ✅ true | ✅ true |
| (e) `!Contains(out, "diff truncated at")` (no legacy sentinel) | ✅ true | ✅ true | ✅ true |

Before the S1 fix, the same setup shipped ~14543 tokens (7× over) with 0 sentinels and 300+ bogus headers.
The fix collapses it to ~1005 tokens, 1 sentinel, 1 real header.

---

## 4. Helpers (all same-package `git`, confirmed via grep)

| Helper | Location | Signature |
|---|---|---|
| `initRepo` | `git_test.go:13` | `func initRepo(t *testing.T, dir string)` — `git init` + identity (NO `initRepoWithUser` exists; do not invent one) |
| `writeFile` | `committree_test.go:31` | `func writeFile(t *testing.T, dir, name, body string)` — working-tree write (NOT staged) |
| `stageFile` | `committree_test.go:39` | `func stageFile(t *testing.T, dir, name string)` — `git add <name>` |
| `writeTreeOf` | `committree_test.go:48` | `func writeTreeOf(t *testing.T, dir string) string` — `git write-tree` → SHA |
| `commitAllowEmpty` | `difftokenlimit_test.go:25` | `func commitAllowEmpty(t *testing.T, dir, msg string)` — baseline HEAD for working-tree diffs |
| `EstimateTokens` | `tokens.go` | `func EstimateTokens(s string) int` — the chars/4 estimator |
| `tokenBudgetMargin` | `tokengate.go:48` | `const tokenBudgetMargin = 1024` |
| `tail` | `truncatediff_test.go` | `func tail(s string, n int) string` — last n bytes (for error messages) |

The three per-function setups (mirroring the existing `TestStagedDiff_TokenLimitGt0_WaterFill`,
`TestTreeDiff_TokenLimitGt0`, `TestWorkingTreeDiff_TokenLimitGt0`):
- **StagedDiff**: `newRepo` + writeFile `fixtures.diff` + stageFile → `g.StagedDiff(ctx, opts)`.
- **TreeDiff**: write+stage `base.go` → `treeA = writeTreeOf`; write+stage `fixtures.diff` → `treeB = writeTreeOf` → `g.TreeDiff(ctx, treeA, treeB, opts)`.
- **WorkingTreeDiff**: stage `base.go` + `fixtures.diff`(small) → `commitAllowEmpty("baseline")`; then UNSTAGED bloat of `fixtures.diff` (the embedded-literal body) → `g.WorkingTreeDiff(ctx, opts)`.

---

## 5. The Assertion Style + Helper Decision (follow the existing file — NO testify)

`difftokenlimit_test.go` uses **plain `if/t.Errorf/t.Fatalf` + `strings.Contains`/`strings.Count`**, no
testify. Each test is self-contained with `repo := t.TempDir(); initRepo(t, repo)`. The file already defines
a helper (`commitAllowEmpty` :25). To avoid 3×5 assertion duplication, define ONE shared helper
`assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)` (with `t.Helper()`) containing the 5
assertions, plus a small body-builder `embeddedDiffLiteralBody(blocks int) string`. Called from each of the
3 tests.

**Key strings (all grep-confirmed):**
- FR3g skeleton header: `"Change summary (numstat"` (difftokenlimit_test.go:109/216/257).
- Water-fill sentinel: `"... [truncated]"`.
- Legacy sentinel detector: `"diff truncated at"`.
- Real file header (full form): `"diff --git a/" + file + " b/" + file` — distinct from the embedded `fX` sample names so `Count == 1` proves non-fragmentation.

---

## 6. Why These Tests PASS Now (and would have FAILED before S1)

The S1 fix replaced the un-anchored `strings.Split(diff, "diff --git ")` with the line-anchored
`(?m)^diff --git ` regex slice. Before the fix, a content line `+diff --git a/fX b/fX` matched the split →
300 bogus sections → water-fill truncated nothing → ~14543 tokens shipped (assertion (a) FAILS: 14543 >
4048; (b) FAILS: 0 sentinels; (c) FAILS: 1 real header but 300+ total `diff --git a/` matches if counting
loosely). After the fix, the file is ONE section → sized → truncated to fit → ~1005 tokens, 1 sentinel, 1
real header. These tests PIN the fixed behavior: they pass today and fail loudly if anyone reverts the S1
line-anchoring.

---

## 7. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Test structure? | 3 new top-level `func TestX(t)` (one per diff function) + 1 shared assertion helper + 1 body-builder helper. | Matches the existing `<Func>_TokenLimitGt0_<Variant>` naming (top-level funcs). Helpers avoid 3×5 duplication; the file already has a helper (`commitAllowEmpty`). |
| D2 | One file with embedded literals (not two large files)? | YES — `fixtures.diff` with 300 `diff --git ` blocks. | The bug is specifically about content-embedded literals fragmenting ONE file. Two-large-files is the bugfix-001 (glue) test, a different defect. |
| D3 | TokenLimit / blocks? | `TokenLimit: 2000`, 300 blocks. | Empirically proven: ~1001-1005 tokens out (fits 4048 ceiling), 1 sentinel, 1 real header, for all 3 functions. The contract's repro used 500 blocks / 2000; 300 is sufficient and faster. |
| D4 | PromptReserveTokens? | `0`. | Matches the contract's scaffold + the existing WaterFill test. |
| D5 | DiffContext? | NOT set (omit; matches the contract's scaffold + repro). | The contract's drop-in repro omits it; the existing WaterFill test omits it. Not load-bearing for the fragmentation assertions. |
| D6 | Fragmentation assertion form? | `Count(out, "diff --git a/"+file+" b/"+file) == 1` (full header). | Distinct from the embedded `fX` sample names → robustly proves the real file is ONE section. The bug would leave the count at 1 but ship 300+ bogus sections of `fX` — the fits-budget + sentinel assertions catch the overflow. |
| D7 | Shared assertion helper? | YES — `assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)`. | DRY; plain `if/t.Errorf` (not testify); file has a helper precedent. The 5 assertions are identical across all 3 tests (only the diff-function call differs). |
| D8 | Sentinel count == 1 (not >= 1)? | `== 1`. | Each test stages exactly ONE large file → exactly 1 truncation. Empirically confirmed (1 for all three). Stronger than the contract's floor (>= 1). |
| D9 | Scope? | ONLY `difftokenlimit_test.go`. | S1 owns truncatediff.go + truncatediff_test.go; P1.M2 owns docs. This task is the E2E regression tests only. |
