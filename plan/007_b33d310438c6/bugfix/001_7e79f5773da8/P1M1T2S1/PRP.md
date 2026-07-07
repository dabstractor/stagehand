---
name: "P1.M1.T2.S1 — E2E multi-section truncation regression tests in difftokenlimit_test.go"
description: |
  TEST-ONLY regression (PRD Issue 1 — Major). Add 3 E2E integration tests (one per diff function:
  StagedDiff, TreeDiff, WorkingTreeDiff) + 1 shared assertion helper to `internal/git/difftokenlimit_test.go`.
  Each test stages/creates TWO large non-markdown files (a.go, b.go) so under a small TokenLimit BOTH are
  capped by the water-fill → the FIRST section's `... [truncated]` sentinel is followed by the SECOND
  section's `diff --git` header. Asserts: (a) NO glue (`![truncated]diff --git`); (b) the fixed form
  (`[truncated]\ndiff --git`); (c) both files' `diff --git` headers survive; (d) the FR3g numstat skeleton
  is present; (e) the legacy `at N bytes/lines` sentinels are ABSENT; + sentinel count == 2. The design is
  EMPIRICALLY PROVEN against the real git binary for all 3 functions. The S1 fix is already landed; these
  tests PIN the fixed behavior. Reuses existing helpers (initRepo/writeFile/stageFile/writeTreeOf/
  commitAllowEmpty). Plain if/t.Errorf style, no testify. Test-only — no production/docs change.
---

## Goal

**Feature Goal**: Pin the Issue-1 fix (FR3i truncated-section newline separator) with E2E integration
regression tests across all three diff entry points, so the multi-section glue defect
(`... [truncated]diff --git a/b.go b/b.go`) can never silently return. Each test exercises the path the
existing tests miss: TWO files BOTH truncated under a small `token_limit`, where the FIRST section's
sentinel is followed by the SECOND section's `diff --git` — the exact join the bug corrupted.

**Deliverable** (ONE test file modified; 3 new test funcs + 1 shared helper):
1. `internal/git/difftokenlimit_test.go` — add `assertMultiSectionTruncationFormat(t, out)` helper (6
   assertions, `t.Helper()`), plus three top-level tests:
   `TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated`,
   `TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated`,
   `TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated`.
   Each: `t.TempDir` + `initRepo` + two large files + the function-specific setup → call → assert via the
   helper. No production change, no docs, no new binary.

**Success Definition**: `go test ./internal/git/...` green with the 3 new tests passing (each: no-glue,
fixed form, both headers, skeleton, no-legacy, 2 sentinels — all empirically proven). Existing tests stay
green. `go vet ./...` + `gofmt -l .` clean. No file outside `internal/git/difftokenlimit_test.go` touched.

## User Persona

**Target User**: The Stagecoach maintainer guarding the FR3i truncation-format invariant against regressions, and any future contributor who might refactor `truncateByWaterFill` or the gate wiring.

**Use Case**: CI runs `go test ./internal/git/...`. If anyone reverts the S1 sentinel-newline fix (or breaks the multi-section recomposition), these E2E tests fail loudly — catching a malformed-payload bug that the existing single-section tests cannot detect.

**User Journey**: a contributor edits `truncateByWaterFill` → runs `go test ./internal/git/...` → the multi-section tests assert the intermediate sentinel is followed by `\n` then the next `diff --git` (not glued) → a regression is caught before merge.

**Pain Points Addressed**: Closes the exact coverage gap that let Issue 1 ship: every existing `>0` test capped only ONE file, so the intermediate-section join was never exercised E2E. These tests are that coverage.

## Why

- **The bug report mandated this regression test.** Issue 1 (TEST_RESULTS.md): "add a regression test that truncates a *non-last* section and asserts the sentinel is followed by `\n`." S1 (P1.M1.T1.S1, landed) applied the fix + pure unit tests in `truncatediff_test.go`; this task is the E2E integration counterpart across the three real diff functions.
- **E2E catches what pure unit tests cannot.** The pure `truncateByWaterFill` tests feed string literals; the E2E tests exercise the full pipeline (real `git diff` → numstat skeleton → section assembly → gate → water-fill → recomposition) against a real temp repo. This is the layer where section-ordering / argv / glue bugs actually hide.
- **The gap was structural.** Single-section truncation (the existing tests) cannot expose a between-sections glue defect — there is no next section. Only a BOTH-truncated multi-section test can. The bug report's "Areas needing more attention" names this exact gap.
- **Lowest-risk change.** Pure test addition; reuses every helper (initRepo/writeFile/stageFile/writeTreeOf/commitAllowEmpty) and the existing assertion style; the design is empirically proven against the live code. No production code can be broken.

## What

Three new top-level E2E test functions in `internal/git/difftokenlimit_test.go` (one per diff function)
plus one shared assertion helper. Each test creates TWO large non-markdown files under
`StagedDiffOptions{TokenLimit: 4000, DiffContext: 1, PromptReserveTokens: 0}` so BOTH are capped, then
asserts (via the helper) the no-glue property + both headers + skeleton + no-legacy + 2 sentinels. The
function-specific setups mirror the existing `TestTreeDiff_TokenLimitGt0` (writeTreeOf for two tree SHAs)
and `TestWorkingTreeDiff_TokenLimitGt0` (commitAllowEmpty baseline then unstaged edits).

### Success Criteria

- [ ] `assertMultiSectionTruncationFormat(t *testing.T, out string)` helper defined (with `t.Helper()`), containing the 6 assertions.
- [ ] `TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated` — stages a.go + b.go (both large), calls `StagedDiff`, asserts via the helper.
- [ ] `TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated` — treeA (base.go) vs treeB (base+a+b), calls `TreeDiff`, asserts via the helper.
- [ ] `TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated` — baseline commit then unstaged bloat of a.go + b.go, calls `WorkingTreeDiff`, asserts via the helper.
- [ ] Each test uses `TokenLimit: 4000, DiffContext: 1, PromptReserveTokens: 0` and is self-contained (`t.TempDir` + `initRepo`).
- [ ] The helper asserts: (a) `!Contains(out, "[truncated]diff --git")`; (b) `Contains(out, "... [truncated]\ndiff --git")`; (c) both `diff --git a/a.go b/a.go` and `diff --git a/b.go b/b.go`; (d) `Contains(out, "Change summary (numstat")`; (e) `!Contains(out, "diff truncated at")`; sentinel `Count == 2`.
- [ ] `go test ./internal/git/...` green (3 new tests pass; existing tests unchanged).
- [ ] `go vet ./...`, `gofmt -l .` clean.
- [ ] No file outside `internal/git/difftokenlimit_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP provides the copy-paste-ready helper + 3 test bodies (verified against
the live code), names every reused helper with its file/line, states the exact assertion substrings (all
grep-confirmed in the existing tests), gives the function-specific setups (mirroring the existing
TreeDiff/WorkingTreeDiff tests), and — critically — backs the design with an empirical proof table showing
all 6 assertions hold for all 3 diff functions against the real git binary. The S1 fix is confirmed landed;
the tests pass today and pin the fixed behavior.

### Documentation & References

```yaml
# MUST READ — the bug + the mandated regression
- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/TEST_RESULTS.md
  why: "Issue 1: the glued `... [truncated]diff --git a/b.go b/b.go` defect, the empirical repro, and the mandate: 'add a regression test that truncates a non-last section and asserts the sentinel is followed by \\n (not by diff --git)'. Notes the existing tests missed it because they truncate only the LAST/only section."
  critical: "States the bug is in the shared truncateByWaterFill (so all 3 diff functions are affected) and that the regression MUST truncate a NON-LAST section. This task IS that E2E regression across all 3 functions."

- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/docs/system_context.md
  why: "§2.3 (the test scaffold), §5 (helper signatures/locations), §6 (the invariants: the water-fill sentinel shape, the legacy-sentinel absence on >0). Confirms the helper inventory and the assertion substrings."
  critical: "§6 invariant 2 is the `... [truncated]` sentinel shape; invariant 1 is the ==0 legacy-sentinel anchor. The new tests assert invariant 2's line-shape discipline (sentinel on its own line, next header at a line start) for the MULTI-section case."

- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M1T1S1/PRP.md
  why: "The parallel sibling (S1 = the production fix + pure unit tests). Confirms the fix landed in truncatediff.go:204 (`+ truncatedSentinel + \"\\n\"`) and that S1 explicitly leaves difftokenlimit_test.go to THIS task (S2). No file overlap → no conflict."
  critical: "S1's fix is the trailing `\\n` after the sentinel. These E2E tests assert that fix end-to-end. S1 owns truncatediff.go + truncatediff_test.go; S2 (this) owns difftokenlimit_test.go."

- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M1T2S1/research/multisection_e2e_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-04): the S1 fix is LANDED; baseline GREEN; the test design (2 large files, TokenLimit 4000) produces all 6 assertions PASSING for all 3 diff functions (proof table); the captured join region `... [truncated]\\ndiff --git a/b.go b/b.go`; the helper inventory with file:line; the assertion-style decision; decisions D1–D7. READ THIS FIRST."
  critical: "§3 (the proof table) is the decisive evidence the test design works before writing it. §4 (helper signatures) and §5 (assertion substrings) give the exact strings to use. §6 explains why the tests pass now (fix landed) and would have failed pre-fix."

# The edit target + its established patterns
- file: internal/git/difftokenlimit_test.go
  why: "THE edit target. The E2E integration file for the token-limit gate. Existing scaffold: `repo := t.TempDir(); initRepo(t, repo)` then writeFile/stageFile pairs, `g := New(repo)`, then the diff call. Existing tests: TestStagedDiff_TokenLimitGt0_WaterFill (ONE huge + ONE small — the gap), TestTreeDiff_TokenLimitGt0 (writeTreeOf for two trees), TestWorkingTreeDiff_TokenLimitGt0 (commitAllowEmpty baseline + unstaged edits). commitAllowEmpty helper at :25."
  pattern: "Plain if/t.Errorf/t.Fatalf + strings.Contains/Count, NO testify. Each test self-contained (own t.TempDir). The file defines commitAllowEmpty as a helper — a shared assertion helper is consistent. Assertion substrings in use: 'Change summary (numstat' (skeleton), 'diff truncated at' (legacy detector), '... [truncated]' (water-fill sentinel)."
  gotcha: "Append the 3 new tests + 1 helper; do NOT modify the existing tests. The new tests use the SAME setup patterns as TestTreeDiff_TokenLimitGt0 (writeTreeOf) and TestWorkingTreeDiff_TokenLimitGt0 (commitAllowEmpty + unstaged edits) — mirror them."

# Read-only helper definitions (do NOT edit)
- file: internal/git/git_test.go
  why: "READ-ONLY. initRepo(t, dir) at :13 — git init + identity config. Reused by every test."
- file: internal/git/committree_test.go
  why: "READ-ONLY. writeFile(t,dir,name,body) :31, stageFile(t,dir,name) :39, writeTreeOf(t,dir) string :48, headSHA(t,dir) string :59. The helpers the new tests reuse."

# Read-only production refs (the code under test — do NOT edit)
- file: internal/git/truncatediff.go
  why: "READ-ONLY. truncateByWaterFill:204 — the S1 fix landed here (`... + truncatedSentinel + \"\\n\"`). const truncatedSentinel = \"... [truncated]\" (:57). The tests assert this fix's output shape end-to-end."
- file: internal/git/git.go
  why: "READ-ONLY. StagedDiffOptions fields TokenLimit/DiffContext/PromptReserveTokens (:55/64/72). The three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff) consume opts via the shared gate."

# External references
- url: https://pkg.go.dev/testing#T.Helper
  why: "Confirm t.Helper() marks a test helper so assertion failures report the CALLER's line (not the helper's line). This is why the shared assertMultiSectionTruncationFormat helper should call t.Helper() — failures point at the specific test/section."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── difftokenlimit_test.go   # EDIT TARGET — append 3 tests + 1 helper
    ├── git_test.go              # READ-ONLY — initRepo helper
    ├── committree_test.go       # READ-ONLY — writeFile/stageFile/writeTreeOf/headSHA helpers
    ├── truncatediff.go          # READ-ONLY — the S1 fix landed here (:204); code under test
    └── git.go                   # READ-ONLY — StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    internal/git/difftokenlimit_test.go   # +assertMultiSectionTruncationFormat helper + 3 MultiSection tests
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/difftokenlimit_test.go` | MODIFY (append) | Add the shared `assertMultiSectionTruncationFormat` helper + 3 E2E tests (one per diff function) pinning the multi-section no-glue property. |

**Explicitly NOT touched**: `internal/git/truncatediff.go` (the S1 fix — landed), `truncatediff_test.go`
(S1's pure unit tests), `git.go` / `tokengate.go` / `waterfill.go` / `tokens.go` / `numstat.go` /
`skeleton.go` (production code under test), other `internal/git/*_test.go` files, any other package, any
docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — TWO files, BOTH truncated): the existing >0 tests cap only ONE file, which is why they
// missed the bug. The multi-section glue only appears when a truncated section is FOLLOWED by another
// section. Each new test MUST stage/create TWO large files so BOTH are capped (sentinel count == 2). Do
// NOT make one of them small (that collapses back to the already-tested single-section case).

// CRITICAL (G2 — the bug signature vs the fixed form): the two key assertions are complementary:
//   (a) !strings.Contains(out, "[truncated]diff --git")   // the GLUED form (bug) — must be ABSENT
//   (b)  strings.Contains(out, "... [truncated]\ndiff --git")  // the FIXED form — must be PRESENT
// Note (a) uses "[truncated]diff --git" (no \n) and (b) uses "... [truncated]\ndiff --git" (with \n).
// Both hold IFF the S1 fix is in place. If you swap the \n you invert the test. (Empirically proven.)

// CRITICAL (G3 — WorkingTreeDiff needs TRACKED files): untracked files do NOT appear in `git diff`
// (working-tree-vs-index). TestWorkingTreeDiff_TokenLimitGt0_MultiSection MUST commit a baseline
// (commitAllowEmpty) that tracks base.go + a.go + b.go, THEN bloat a.go + b.go UNSTAGED. Mirrors the
// existing TestWorkingTreeDiff_TokenLimitGt0 exactly.

// GOTCHA (G4 — TreeDiff needs two real trees): use writeTreeOf(t, repo) to capture treeA (base.go only)
// and treeB (base.go + a.go + b.go). The diff treeA→treeB shows both a.go and b.go as added. Mirrors the
// existing TestTreeDiff_TokenLimitGt0.

// GOTCHA (G5 — assertion substrings are grep-confirmed): skeleton header is "Change summary (numstat"
// (NOT "numstat skeleton" or "change summary"); the legacy detector is "diff truncated at" (matches both
// "... [diff truncated at N bytes]" and "... [diff truncated at N lines]"); the water-fill sentinel is
// "... [truncated]". Reuse these EXACT substrings (the existing tests at :109/216/257 use them).

// GOTCHA (G6 — DiffContext: 1 explicitly): pass DiffContext: 1 (the production default -U1) in the
// StagedDiffOptions literal. The struct's zero-value DiffContext is 0 (= -U0, valid but not the default);
// the contract requires the resolved value (1) explicitly. (Not load-bearing for the truncation
// assertions, but matches the contract + production default.)

// GOTCHA (G7 — no testify): use plain `if !strings.Contains(out, ...) { t.Errorf(...) }`. The file has
// ZERO testify imports; adding one would break the convention. The shared helper uses t.Helper() +
// t.Errorf, which IS the file's style.

// GOTCHA (G8 — TokenLimit 4000 is proven): the existing TestStagedDiff_TokenLimitGt0_WaterFill uses 4000
// with ONE 1000-line huge file (capped). With TWO 1000-line files under 4000, BOTH are capped (empirically
// verified: sentinel count == 2). Do NOT lower TokenLimit below ~2000 (risk of the minBodyTokens floor
// path changing behavior) or raise it above ~8000 (risk of one file fitting whole → only 1 sentinel).

// GOTCHA (G9 — the fix is landed; tests pin it): truncatediff.go:204 ALREADY has `+ "\n"` after the
// sentinel. These tests PASS against the current code. They are a REGRESSION NET: if anyone reverts the
// S1 newline, assertions (a) and (b) fail loudly. Do NOT try to "fix" anything in production — this task
// is tests-only.
```

## Implementation Blueprint

### Data models and structure

No new types. The tests reuse `StagedDiffOptions{TokenLimit, DiffContext, PromptReserveTokens}` and the
existing helpers. The shared assertion helper takes the returned `out string`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the shared assertion helper to internal/git/difftokenlimit_test.go
  - FILE: internal/git/difftokenlimit_test.go
  - PLACE: near the existing commitAllowEmpty helper (after it, before the first Test func) OR at the file
    end. Mark it with a comment header.
  - WRITE (verbatim — the 6 assertions, empirically proven):
        // assertMultiSectionTruncationFormat asserts the Issue-1 fix holds for a multi-section water-fill
        // output (TWO files both truncated): no glued sentinel, the fixed sentinel+newline+diff--git join,
        // both file headers survive, the FR3g skeleton is present, legacy sentinels are absent, and exactly
        // 2 truncation sentinels appear (both files capped). Shared by the three MultiSection tests.
        func assertMultiSectionTruncationFormat(t *testing.T, out string) {
            t.Helper()
            // (a) Bug signature GONE: the sentinel is NEVER immediately followed by `diff --git`.
            if strings.Contains(out, "[truncated]diff --git") {
                t.Errorf("multi-section: sentinel glued to next diff --git (Issue 1 regressed); out=\n%s", out)
            }
            // (b) The fixed form: a truncated section's sentinel is followed by a newline then the next diff --git.
            if !strings.Contains(out, "... [truncated]\ndiff --git") {
                t.Errorf("multi-section: expected '... [truncated]\\ndiff --git' (sentinel on own line, next header at line start); out=\n%s", out)
            }
            // (c) Both files' diff --git section headers survive.
            if !strings.Contains(out, "diff --git a/a.go b/a.go") {
                t.Errorf("multi-section: a.go diff --git header missing; out=\n%s", out)
            }
            if !strings.Contains(out, "diff --git a/b.go b/b.go") {
                t.Errorf("multi-section: b.go diff --git header missing; out=\n%s", out)
            }
            // (d) The FR3g numstat skeleton is present (completeness floor).
            if !strings.Contains(out, "Change summary (numstat") {
                t.Errorf("multi-section: FR3g skeleton missing; out=\n%s", out)
            }
            // (e) The legacy 'at N bytes/lines' sentinels are ABSENT on the token_limit>0 path.
            if strings.Contains(out, "diff truncated at") {
                t.Errorf("multi-section: legacy 'diff truncated at N' sentinel must be ABSENT; out=\n%s", out)
            }
            // Both files truncated ⇒ exactly 2 water-fill sentinels.
            if c := strings.Count(out, "... [truncated]"); c != 2 {
                t.Errorf("multi-section: expected 2 sentinels (both files capped), got %d; out=\n%s", c, out)
            }
        }
  - DO NOT: import testify; change the signature (takes `out string`); add a `label` param (the test name
    in the output identifies the function — keep it simple).

Task 2: ADD TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated
  - PLACE: after TestStagedDiff_TokenLimitGt0_HugeMarkdown (group with the StagedDiff tests).
  - WRITE (verbatim — mirrors TestStagedDiff_TokenLimitGt0_WaterFill but with TWO huge files):
        func TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // TWO large non-markdown files, BOTH staged. Under a small token_limit BOTH are capped by the
            // water-fill → the first section's sentinel is followed by the second's diff --git (the
            // multi-section join the single-section tests never exercise — the gap that hid Issue 1).
            writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 1000))
            stageFile(t, repo, "a.go")
            writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other payload line yyyyyyyy\n", 1000))
            stageFile(t, repo, "b.go")

            g := New(repo)
            out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
                TokenLimit:          4000,
                DiffContext:         1,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("StagedDiff err = %v, want nil", err)
            }
            assertMultiSectionTruncationFormat(t, out)
        }
  - VERIFY (after Task 4): `go test -run TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated ./internal/git/ -v` → PASS.

Task 3: ADD TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated
  - PLACE: after TestTreeDiff_TokenLimitGt0.
  - WRITE (verbatim — mirrors TestTreeDiff_TokenLimitGt0 but with TWO huge files in treeB):
        func TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // treeA: a baseline (base.go only).
            writeFile(t, repo, "base.go", "package main\n")
            stageFile(t, repo, "base.go")
            treeA := writeTreeOf(t, repo)
            // treeB: add TWO large files.
            writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 1000))
            stageFile(t, repo, "a.go")
            writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other payload line yyyyyyyy\n", 1000))
            stageFile(t, repo, "b.go")
            treeB := writeTreeOf(t, repo)

            g := New(repo)
            out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{
                TokenLimit:          4000,
                DiffContext:         1,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("TreeDiff err = %v, want nil", err)
            }
            assertMultiSectionTruncationFormat(t, out)
        }
  - VERIFY: `go test -run TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated ./internal/git/ -v` → PASS.

Task 4: ADD TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated
  - PLACE: after TestWorkingTreeDiff_TokenLimitGt0.
  - WRITE (verbatim — mirrors TestWorkingTreeDiff_TokenLimitGt0 but bloats TWO files unstaged):
        func TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // Baseline: commit base.go + a.go + b.go (small, TRACKED) so working-tree changes appear in
            // `git diff`. Untracked files do NOT appear in `git diff`.
            writeFile(t, repo, "base.go", "package main\n")
            stageFile(t, repo, "base.go")
            writeFile(t, repo, "a.go", "package main\n")
            stageFile(t, repo, "a.go")
            writeFile(t, repo, "b.go", "package lib\n")
            stageFile(t, repo, "b.go")
            commitAllowEmpty(t, repo, "baseline")
            // UNSTAGED working-tree bloat of BOTH tracked files → both large → both capped under the budget.
            writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// unstaged payload line xxxxxxxx\n", 1000))
            writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other unstaged line yyyyyyyy\n", 1000))

            g := New(repo)
            out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{
                TokenLimit:          4000,
                DiffContext:         1,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
            }
            assertMultiSectionTruncationFormat(t, out)
        }
  - VERIFY: `go test -run TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated ./internal/git/ -v` → PASS.

Task 5: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: gofmt -w internal/git/difftokenlimit_test.go
  - RUN: go vet ./internal/git/...
  - RUN: go test ./internal/git/...    # ALL green — 3 new MultiSection tests pass; existing tests unchanged
  - RUN: go test ./...                  # whole repo green (test-only change; no production touched)
  - FIX-FORWARD: if a MultiSection test fails on the sentinel COUNT (!= 2), the bodies may be too small
    or the budget too large — read `out` (printed by the helper) and increase the body size or decrease
    TokenLimit. (Empirically, 1000 lines × TokenLimit 4000 → 2 sentinels for all three.) If it fails on
    the no-glue assertion, the S1 fix has been reverted — do NOT fix production here; flag it.
```

### Implementation Patterns & Key Details

```go
// === The multi-section join (the heart of the regression) ===
// Before S1 (bug):   ...first body... [truncated]diff --git a/b.go b/b.go   (GLUED — one line)
// After S1 (fixed):  ...first body... [truncated]\ndiff --git a/b.go b/b.go (sentinel on own line)
// The two complementary assertions pin the fixed shape:
//   (a) !Contains(out, "[truncated]diff --git")          // glued form absent
//   (b)  Contains(out, "... [truncated]\ndiff --git")    // fixed form present

// === Why TWO large files (not one huge + one small) ===
// The existing WaterFill tests use 1 huge + 1 small: only ONE section is truncated (the huge one), and it
// is the LAST section → no next section to glue to → the bug is invisible. The multi-section case needs
// BOTH files truncated so the FIRST section's sentinel is followed by the SECOND's diff --git. With both
// ~9000 tokens and bodyBudget ~3900, the water-fill level L ≈ 1950 → both capped → 2 sentinels → the
// intermediate join is exercised. (Empirically verified for all 3 diff functions.)

// === The shared helper is consistent with the file's conventions ===
// difftokenlimit_test.go already defines commitAllowEmpty as a helper. assertMultiSectionTruncationFormat
// follows the same pattern (plain func, t.Helper). It avoids 3×6 assertion duplication and makes the
// "same property across 3 entry points" structure explicit. t.Helper() ensures failures point at the
// caller's test, not the helper.

// === The skeleton-header + legacy-detector substrings (grep-confirmed) ===
// "Change summary (numstat" — the FR3g skeleton header (difftokenlimit_test.go:109/216/257).
// "diff truncated at"     — matches both legacy sentinels ("... [diff truncated at N bytes/lines]").
// "... [truncated]"       — the water-fill sentinel (const truncatedSentinel, truncatediff.go:57).
```

### Integration Points

```yaml
TEST (internal/git/difftokenlimit_test.go):
  - +assertMultiSectionTruncationFormat(t, out) helper (6 assertions, t.Helper)
  - +TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated
  - +TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated
  - +TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated
  - each: t.TempDir + initRepo + 2 large files + function-specific setup → call → helper

CONSUMED HELPERS (READ-ONLY):
  - initRepo (git_test.go:13), writeFile/stageFile/writeTreeOf (committree_test.go:31-59),
    commitAllowEmpty (difftokenlimit_test.go:25)

CODE UNDER TEST (READ-ONLY — do NOT edit):
  - internal/git/truncatediff.go:204 (the S1 fix — sentinel + "\n")
  - internal/git/git.go (StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions)
  - internal/git/{tokengate,waterfill,tokens,numstat,skeleton}.go (the gate pipeline)

GATE: go test ./internal/git/... → GREEN (3 new tests pass; existing unchanged); go test ./... green

NO-TOUCH (explicitly):
  - internal/git/truncatediff.go, truncatediff_test.go   # S1 (landed) — the fix + its pure unit tests
  - internal/git/{git,tokengate,waterfill,tokens,numstat,skeleton}.go + other *_test.go  # production + other tests
  - any package outside internal/git; any docs
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational):
  - The release gate `go test ./internal/git/...` consumes these tests alongside S1's pure unit tests.
  - P1.M2 (diff_context validation) and P1.M3 (docs sweep) are orthogonal; this task is unaffected.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/difftokenlimit_test.go   # Expected: empty (run gofmt -w if listed)
go vet ./internal/git/...                        # Expected: exit 0
go build ./...                                   # Expected: exit 0 (test-only; nothing else changed)

# Expected: Zero errors. (Note: `go vet` on a *_test.go file compiles the test variants too.)
```

### Level 2: The Three New Tests (each must PASS)

```bash
cd /home/dustin/projects/stagecoach

# Each MultiSection test in isolation (the contract's verification commands):
go test -run TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated   ./internal/git/ -v
go test -run TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated     ./internal/git/ -v
go test -run TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated ./internal/git/ -v

# Expected: each PASS. The helper's 6 assertions all hold (empirically proven): no-glue, fixed form,
# both headers, skeleton, no-legacy, 2 sentinels.
```

### Level 3: Whole-Package + Whole-Repo Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/git/...    # Expected: ALL green — 3 new tests + every existing test (incl. S1's pure
                              # truncatediff_test.go subtests, which these complement).
go test ./...                 # Expected: ALL packages green (test-only change; no production touched).

# Confirm ONLY the one test file changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/git/difftokenlimit_test.go only. No production file, no other test file.
```

### Level 4: The Bug-Is-Gone Property (direct cross-check)

```bash
cd /home/dustin/projects/stagecoach

# The 3 new tests directly assert the glued form is absent. Cross-check by running the whole git suite
# and grepping for the (now impossible) glued shape in any failure output:
go test ./internal/git/ -v 2>&1 | grep -E "MultiSection|PASS|FAIL" | head

# Expected: 3 MultiSection tests PASS, 0 FAIL. The glued shape "[truncated]diff --git" is asserted ABSENT
# by the helper's assertion (a) in all three.

# (Optional belt-and-suspenders) Confirm the S1 fix is in place (the tests' raison d'être):
grep -n 'truncatedSentinel + "\\\\n"' internal/git/truncatediff.go   # → :204 (the fix landed)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` (incl. `./internal/git/...`) — all packages green.

### Feature Validation

- [ ] `assertMultiSectionTruncationFormat(t, out)` helper defined with `t.Helper()` and the 6 assertions.
- [ ] `TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated` exists, stages two large files, passes.
- [ ] `TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated` exists (treeA/treeB via writeTreeOf), passes.
- [ ] `TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated` exists (commitAllowEmpty + unstaged), passes.
- [ ] Each test uses `TokenLimit: 4000, DiffContext: 1, PromptReserveTokens: 0` and its own `t.TempDir`.
- [ ] The helper asserts: no-glue, fixed form, both `diff --git` headers, FR3g skeleton, no legacy sentinel, sentinel count == 2.

### Scope Discipline Validation

- [ ] ONLY `internal/git/difftokenlimit_test.go` modified (`git diff --stat` confirms; 3 tests + 1 helper appended).
- [ ] Did NOT edit production code (`truncatediff.go`, `git.go`, siblings) — the fix is already landed; this task pins it.
- [ ] Did NOT edit `truncatediff_test.go` (S1's pure unit tests) or any other `*_test.go`.
- [ ] Did NOT touch any docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] Tests follow the existing `difftokenlimit_test.go` style (plain if/t.Errorf/t.Fatalf, strings.Contains/Count, NO testify).
- [ ] Each test is self-contained (own `t.TempDir` + `initRepo`); reuses the established helpers.
- [ ] The shared helper uses `t.Helper()` so failures point at the calling test.
- [ ] Test names follow the existing `<Func>_TokenLimitGt0_<Variant>` convention.

---

## Anti-Patterns to Avoid

- ❌ Don't cap only ONE file (one huge + one small) — that collapses to the already-tested single-section case and hides the glue. BOTH files must be large so BOTH are truncated (sentinel count == 2) (gotcha G1).
- ❌ Don't swap the `\n` in the two key assertions. (a) is `!Contains("[truncated]diff --git")` (the GLUED form, no `\n` — must be absent); (b) is `Contains("... [truncated]\ndiff --git")` (the FIXED form, with `\n` — must be present). Swapping inverts the test (G2).
- ❌ Don't use untracked files for the WorkingTreeDiff test — they don't appear in `git diff`. Commit a baseline (commitAllowEmpty) tracking base+a+b, THEN bloat a.go + b.go unstaged (G3).
- ❌ Don't forget `writeTreeOf` for BOTH trees in the TreeDiff test — `treeA` (base.go) and `treeB` (base+a+b) (G4).
- ❌ Don't invent new assertion substrings — use the grep-confirmed `"Change summary (numstat"` (skeleton), `"diff truncated at"` (legacy detector), `"... [truncated]"` (water-fill sentinel) (G5).
- ❌ Don't omit `DiffContext: 1` — the struct's zero-value is 0 (-U0); the contract requires the resolved default (1) explicitly (G6).
- ❌ Don't import testify — the file uses plain if/t.Errorf; adding testify breaks the convention (G7).
- ❌ Don't set TokenLimit outside the ~2000–8000 range — 4000 is empirically proven to cap both 1000-line files (count == 2). Too low risks the minBodyTokens floor; too high risks only one file capped (G8).
- ❌ Don't edit production code — the S1 fix is landed; these tests PIN it. A failing no-glue assertion means the fix was reverted — flag it, don't "fix" it in production here (G9).
- ❌ Don't edit `truncatediff_test.go` (S1's pure unit tests) or any other test/production file — this task is `difftokenlimit_test.go` only.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a pure test addition whose design is **empirically proven** against the live code before
writing. A throwaway probe exercised the exact setup (two 1000-line files, `TokenLimit: 4000, DiffContext: 1,
PromptReserveTokens: 0`) through the real `StagedDiff`, `TreeDiff`, and `WorkingTreeDiff` against real temp
git repos, and confirmed ALL SIX contract assertions hold for ALL THREE functions (no-glue, fixed form, both
headers, skeleton, no-legacy, sentinel count == 2), with the captured join region showing exactly
`... [truncated]\ndiff --git a/b.go b/b.go`. The S1 fix is confirmed landed (`truncatediff.go:204`), the
baseline is GREEN, and the helper inventory + assertion substrings are grep-confirmed in the existing tests.
The test bodies mirror the existing `TestTreeDiff_TokenLimitGt0` / `TestWorkingTreeDiff_TokenLimitGt0`
setups verbatim (only the file count differs), and the shared helper follows the file's own `commitAllowEmpty`
precedent. The prior parallel PRP (S1) is cleanly fenced (different file). The residual 0.5 uncertainty is
purely gofmt placement + whether a CI box's git produces a marginally different body size that shifts the
sentinel count — mitigated by the proven 1000-line × 4000-token setup (which gives large margins: each body
is ~9000 tokens vs a ~1950-token water-fill level) and the explicit "read `out` and adjust body size"
fix-forward guidance. No production code, no docs, no other tests are in scope, so the blast radius is one
test file.
