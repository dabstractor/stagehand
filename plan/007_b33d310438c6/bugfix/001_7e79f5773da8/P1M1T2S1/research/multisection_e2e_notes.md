# Research: E2E Multi-Section Truncation Regression (P1.M1.T2.S1)

> **Purpose:** Pin the exact, empirically-verified E2E regression tests for the Issue-1 fix (FR3i
> truncated-section newline separator), checked against the live codebase on 2026-07-04. **The S1 fix is
> ALREADY LANDED** (`internal/git/truncatediff.go:204` writes `... + truncatedSentinel + "\n"`); baseline
> `go test ./internal/git/` is GREEN. These tests PIN the fixed behavior so it cannot regress. The test
> design (two large files, both capped) is PROVEN end-to-end against the real git binary for ALL THREE
> diff functions (StagedDiff, TreeDiff, WorkingTreeDiff) — every contract assertion holds.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagehand`, `go 1.22` |
| Edit target | `internal/git/difftokenlimit_test.go` (add 3 new top-level test funcs + 1 helper) |
| S1 fix status | **LANDED** — `truncatediff.go:204`: `body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"` (trailing `\n` present) |
| Baseline | `go test ./internal/git/` → **ok (5.856s)** |
| StagedDiffOptions fields | `TokenLimit int`, `DiffContext int`, `PromptReserveTokens int` (git.go:55/64/72) — all present |
| Prior PRP (S1) | Production fix in `truncatediff.go` + pure unit tests in `truncatediff_test.go`. Explicitly does NOT touch `difftokenlimit_test.go` (→ THIS task). **No conflict.** |

---

## 2. The Gap These Tests Close (Issue 1)

Every existing `>0` test in `difftokenlimit_test.go` caps **only ONE** large file (`TestStagedDiff_TokenLimitGt0_WaterFill` = 1 huge + 1 small; `TestTreeDiff_TokenLimitGt0` = 1 huge + 1 small; `TestWorkingTreeDiff_TokenLimitGt0` = 1 huge + 1 small). When only the LAST/only section is truncated, the glued-sentinel bug (`[truncated]diff --git`) is invisible — there is no NEXT section to be glued to. The multi-section path where TWO files are BOTH truncated (so the FIRST section's sentinel is followed by the SECOND section's `diff --git`) was **never exercised E2E** — exactly the gap that hid Issue 1. These tests fill it: TWO large files, both capped, asserting the intermediate sentinel is followed by `\n` then the next `diff --git`.

---

## 3. The Test Design — PROVEN against the real git binary

**Setup:** TWO non-markdown files (a.go, b.go), each ~1000 lines of generated content, under `TokenLimit: 4000, DiffContext: 1, PromptReserveTokens: 0`. The bodies (~9000 tokens each) far exceed the body budget (~3900 after skeleton+margin), so the water-fill caps BOTH → 2 sentinels, and the intermediate section's sentinel is followed by the next section's `diff --git`.

**Empirical proof** (throwaway probe against the live `internal/git` package, all three diff functions):

| Assertion | StagedDiff | TreeDiff | WorkingTreeDiff |
|---|---|---|---|
| (a) `!Contains(out, "[truncated]diff --git")` (no glue) | ✅ true | ✅ true | ✅ true |
| (b) `Contains(out, "... [truncated]\ndiff --git")` (fixed form) | ✅ true | ✅ true | ✅ true |
| (c1) `Contains(out, "diff --git a/a.go b/a.go")` | ✅ true | ✅ true | ✅ true |
| (c2) `Contains(out, "diff --git a/b.go b/b.go")` | ✅ true | ✅ true | ✅ true |
| (d) `Contains(out, "Change summary (numstat")` (FR3g skeleton) | ✅ true | ✅ true | ✅ true |
| (e) `!Contains(out, "diff truncated at")` (no legacy sentinel) | ✅ true | ✅ true | ✅ true |
| sentinel count `Count(out, "... [truncated]")` == 2 | ✅ 2 | ✅ 2 | ✅ 2 |

The captured join region (StagedDiff): `"enerated p\n... [truncated]\ndiff --git a/b.go b/b.go\nnew file mode 1006"` — exactly the fixed shape (sentinel on its own line, next header at a line start).

---

## 4. Helpers (all same-package `git`, confirmed via grep)

| Helper | Location | Signature |
|---|---|---|
| `initRepo` | `git_test.go:13` | `func initRepo(t *testing.T, dir string)` — `git init` + identity config |
| `writeFile` | `committree_test.go:31` | `func writeFile(t *testing.T, dir, name, body string)` — working-tree write (NOT staged) |
| `stageFile` | `committree_test.go:39` | `func stageFile(t *testing.T, dir, name string)` — `git add <name>` |
| `writeTreeOf` | `committree_test.go:48` | `func writeTreeOf(t *testing.T, dir string) string` — `git write-tree` → SHA |
| `headSHA` | `committree_test.go:59` | `func headSHA(t *testing.T, dir string) string` |
| `commitAllowEmpty` | `difftokenlimit_test.go:25` | `func commitAllowEmpty(t *testing.T, dir, msg string)` — baseline HEAD for working-tree diffs |

The three per-function setups (mirroring the existing `TestTreeDiff_TokenLimitGt0` / `TestWorkingTreeDiff_TokenLimitGt0`):
- **StagedDiff**: `newRepo` + stage a.go + stage b.go → `g.StagedDiff(ctx, opts)`.
- **TreeDiff**: write+stage base.go → `treeA = writeTreeOf`; write+stage a.go + b.go → `treeB = writeTreeOf` → `g.TreeDiff(ctx, treeA, treeB, opts)`.
- **WorkingTreeDiff**: stage base.go + a.go(small) + b.go(small) → `commitAllowEmpty("baseline")`; then UNSTAGED bloat of a.go + b.go → `g.WorkingTreeDiff(ctx, opts)`. (Untracked files do NOT appear in `git diff`; the baseline must commit the tracked files first.)

---

## 5. The Assertion Style (follow the existing file — NO testify)

`difftokenlimit_test.go` uses **plain `if/t.Errorf/t.Fatalf` + `strings.Contains`/`strings.Count`**, no testify. Each test is self-contained with `repo := t.TempDir(); initRepo(t, repo)`. The existing file DOES define a helper (`commitAllowEmpty` at :25), so a shared assertion helper is consistent with the file's conventions. To avoid 3×6 assertion duplication, define ONE helper `assertMultiSectionTruncationFormat(t *testing.T, out string)` (with `t.Helper()`) containing the 6 assertions, called from each of the 3 tests.

**Key strings (all grep-confirmed in the existing tests):**
- FR3g skeleton header: `"Change summary (numstat"` (used at difftokenlimit_test.go:109/216/257).
- Water-fill sentinel: `"... [truncated]"` (the const `truncatedSentinel` in truncatediff.go:57).
- Legacy sentinel detector: `"diff truncated at"` (matches both `"... [diff truncated at N bytes]"` and `"... [diff truncated at N lines]"`).

---

## 6. Why These Tests PASS Now (and would have FAILED before S1)

The S1 fix landed `+ "\n"` after the sentinel. Before the fix, the join was `... [truncated]diff --git` (assertion (a) would FAIL — the glue substring is present; (b) would FAIL — no `\n` between). After the fix, the join is `... [truncated]\ndiff --git` ((a) passes — no glue; (b) passes — `\n` present). These tests therefore PIN the fixed behavior: they pass today and will fail loudly if anyone reverts the S1 newline. This is the regression net the bug report demanded ("add a test that truncates a non-last section").

---

## 7. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Test structure? | 3 new top-level `func TestX(t)` (one per diff function) + 1 shared helper. | Matches the existing `<Func>_TokenLimitGt0_<Variant>` naming (top-level funcs, not t.Run subtests). The helper avoids 3×6 duplication; the file already has a helper (`commitAllowEmpty`). |
| D2 | Two files, both truncated? | YES — a.go + b.go, each ~1000 lines, TokenLimit 4000. | Empirically proven: both capped (2 sentinels) for all 3 diff functions. The existing ONE-huge-file tests miss the multi-section glue. |
| D3 | TokenLimit value? | `4000` (Staged/Tree/Working). | Proven: the existing `WaterFill` test uses 4000 with one huge file; with two huge files both exceed the body budget → both capped. |
| D4 | DiffContext? | `1` (explicit). | The production default (-U1); the contract requires passing the resolved value explicitly. |
| D5 | Shared assertion helper? | YES — `assertMultiSectionTruncationFormat(t, out)`. | DRY; plain `if/t.Errorf` (not testify); the file already has a helper precedent. The 6 assertions are identical across all 3 tests. |
| D6 | Sentinel-count assertion? | `Count(out, "... [truncated]") == 2`. | Proven (2 for all three). Guards against accidental under-truncation (only 1 capped) which would weaken the multi-section coverage. |
| D7 | Scope? | ONLY `difftokenlimit_test.go`. | S1 owns truncatediff.go + truncatediff_test.go; S2/M3 own other concerns. This task is the E2E regression tests only. |
