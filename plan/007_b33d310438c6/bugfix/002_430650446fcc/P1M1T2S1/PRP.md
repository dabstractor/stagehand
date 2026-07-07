---
name: "P1.M1.T2.S1 — E2E content-embedded diff --git literal regression tests in difftokenlimit_test.go (StagedDiff + TreeDiff + WorkingTreeDiff)"
description: |
  TEST-ONLY regression (PRD Issue 1, bugfix 002 — Major). Add 3 E2E integration tests (one per diff
  function) + 1 shared assertion helper + 1 body-builder helper to `internal/git/difftokenlimit_test.go`.
  Each test stages ONE non-markdown file (`fixtures.diff`) whose CONTENT embeds many `diff --git ` literals
  (a realistic test-fixture / golden-snapshot / .patch file), under a small TokenLimit. Asserts: (a) FITS
  BUDGET (`EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin`); (b) TRUNCATION OCCURRED (exactly 1
  sentinel for the single large file); (c) NOT FRAGMENTED (exactly ONE real `diff --git a/<file>` header —
  the bug fragmented one file into ~N bogus sections); (d) FR3g numstat skeleton present; (e) legacy
  `at N bytes/lines` sentinels ABSENT. The design is EMPIRICALLY PROVEN against the real git binary for all
  3 functions. The S1 fix is already landed; these tests PIN it. Reuses existing helpers
  (initRepo/writeFile/stageFile/writeTreeOf/commitAllowEmpty). Plain if/t.Errorf style, no testify.
  Test-only — no production/docs change.
---

## Goal

**Feature Goal**: Pin the Issue-1 fix (bugfix 002 — `splitDiffSections` line-anchoring) with E2E
integration regression tests across all three diff entry points, so the fragmentation defect (a non-md
file whose content embeds `diff --git ` literals being torn into bogus sections, silently overflowing
`token_limit`) can never silently return. Each test exercises the path the existing tests miss: a single
real file whose content contains `diff --git ` literals, which the un-anchored split fragmented.

**Deliverable** (ONE test file modified; 3 new test funcs + 1 shared assertion helper + 1 body-builder helper):
1. `internal/git/difftokenlimit_test.go` — add `embeddedDiffLiteralBody(blocks int) string` body-builder,
   `assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)` helper (5 assertions, `t.Helper()`),
   plus three top-level tests:
   `TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral`,
   `TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral`,
   `TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral`.
   Each: `t.TempDir` + `initRepo` + the fixture (content-embedded literals) + the function-specific setup →
   call → assert via the helper. No production change, no docs, no new binary.

**Success Definition**: `go test ./internal/git/...` green with the 3 new tests passing (each: fits budget
~1001-1005 tokens, exactly 1 sentinel, exactly 1 real header, skeleton present, no-legacy — all empirically
proven). Existing tests stay green. `go vet ./...` + `gofmt -l .` clean. No file outside
`internal/git/difftokenlimit_test.go` touched.

## User Persona

**Target User**: The Stagecoach maintainer guarding the FR3d "payload always fits" contract + the FR3i section-splitting invariant against regressions, and any future contributor who might refactor `splitDiffSections` or the gate wiring.

**Use Case**: CI runs `go test ./internal/git/...`. If anyone reverts the S1 line-anchoring fix (or re-introduces an un-anchored split), these E2E tests fail loudly — catching a silent context-window overflow that the existing single-file/single-section tests cannot detect.

**User Journey**: a contributor edits `splitDiffSections` → runs `go test ./internal/git/...` → the content-embedded-literal tests assert the single real file is ONE section (not fragmented), the payload fits the budget, and truncation occurred → a regression is caught before merge.

**Pain Points Addressed**: Closes the exact coverage gap that let Issue 1 ship: every existing test stages content free of `diff --git ` literals, so the fragmentation path was never exercised E2E. These tests are that coverage.

## Why

- **The bug report mandated this regression test.** Issue 1 (TEST_RESULTS.md): "add a regression test that stages a non-markdown file whose content contains `diff --git ` literals under a truncating `token_limit` and asserts (a) the payload fits the budget and (b) exactly one `... [truncated]` sentinel is emitted." S1 (P1.M1.T1.S1, landed) applied the fix + pure unit tests in `truncatediff_test.go`; this task is the E2E integration counterpart across the three real diff functions.
- **E2E catches what pure unit tests cannot.** The pure `TestSplitDiffSections` tests feed string literals; the E2E tests exercise the full pipeline (real `git diff` → numstat skeleton → section assembly → gate → water-fill → recomposition) against a real temp repo with realistic content. This is the layer where the silent overflow actually manifested.
- **The bug was a silent contract violation.** FR3d's headline promise ("the payload always fits your model's context window") was silently broken (~7× overflow measured) precisely for the population that opts into `token_limit` because their model has a small context window. The regression net must cover the realistic triggers (test fixtures, golden snapshots, .patch/.diff files, docs quoting diffs).
- **Lowest-risk change.** Pure test addition; reuses every helper (initRepo/writeFile/stageFile/writeTreeOf/commitAllowEmpty/EstimateTokens/tokenBudgetMargin/tail) and the existing assertion style; the design is empirically proven against the live code. No production code can be broken.

## What

Three new top-level E2E test functions in `internal/git/difftokenlimit_test.go` (one per diff function)
plus one shared assertion helper and one body-builder helper. Each test creates ONE non-markdown file
(`fixtures.diff`) whose content embeds 300 `diff --git ` literals, under
`StagedDiffOptions{TokenLimit: 2000, PromptReserveTokens: 0}`, then asserts (via the helper) the
fits-budget + truncated + not-fragmented + skeleton + no-legacy properties. The function-specific setups
mirror the existing `TestTreeDiff_TokenLimitGt0` (writeTreeOf for two tree SHAs) and
`TestWorkingTreeDiff_TokenLimitGt0` (commitAllowEmpty baseline then unstaged edits).

### Success Criteria

- [ ] `embeddedDiffLiteralBody(blocks int) string` body-builder defined (300-block default content with distinct sample names fa..fz).
- [ ] `assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)` helper defined (with `t.Helper()`), containing the 5 assertions.
- [ ] `TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` — stages `fixtures.diff`, calls `StagedDiff`, asserts via the helper.
- [ ] `TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` — treeA (base.go) vs treeB (base+fixtures.diff), calls `TreeDiff`, asserts via the helper.
- [ ] `TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` — baseline commit then unstaged bloat of `fixtures.diff`, calls `WorkingTreeDiff`, asserts via the helper.
- [ ] Each test uses `TokenLimit: 2000, PromptReserveTokens: 0` and is self-contained (`t.TempDir` + `initRepo`).
- [ ] The helper asserts: (a) `EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin`; (b) `Count(out, "... [truncated]") == 1`; (c) `Count(out, "diff --git a/"+file+" b/"+file) == 1`; (d) `Contains(out, "Change summary (numstat")`; (e) `!Contains(out, "diff truncated at")`.
- [ ] `go test ./internal/git/...` green (3 new tests pass; existing tests unchanged).
- [ ] `go vet ./...`, `gofmt -l .` clean.
- [ ] No file outside `internal/git/difftokenlimit_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP provides the copy-paste-ready body-builder + helper + 3 test bodies
(verified against the live code), names every reused helper with its file/line, states the exact assertion
substrings (all grep-confirmed), gives the function-specific setups (mirroring the existing TreeDiff/
WorkingTreeDiff tests), and — critically — backs the design with an empirical proof table showing all 5
assertions hold for all 3 diff functions against the real git binary. The S1 fix is confirmed landed; the
tests pass today and pin the fixed behavior.

### Documentation & References

```yaml
# MUST READ — the bug + the mandated regression
- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/TEST_RESULTS.md
  why: "Issue 1: `splitDiffSections` fragments files whose content contains the literal `diff --git ` (the un-anchored `strings.Split(diff, \"diff --git \")`), defeating token_limit truncation (~7× overflow at token_limit=2000, 0 sentinels). The measured-overflow table + the realistic triggers (fixtures/snapshots/.patch/docs). Mandates: 'add a regression test that stages a non-markdown file whose content contains `diff --git ` literals under a truncating token_limit and asserts (a) the payload fits the budget and (b) exactly one sentinel.'"
  critical: "States the bug is in the shared splitDiffSections (so all 3 diff functions are affected via applyWaterFillGate) and that the regression MUST use a file whose CONTENT embeds `diff --git ` literals. This task IS that E2E regression across all 3 functions."

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/architecture/system_context.md
  why: "§1 root cause (un-anchored split); §5.2 + §8 the budget-slack ceiling (`EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin`, tokenBudgetMargin=1024) + helper signatures/locations; §6 invariant 2 (the >0 truncation contract)."
  critical: "§5.2/§8 give the exact fits-budget ceiling the existing TestStagedDiff_TokenLimitGt0_WaterFill uses (mirror it). §6 invariant 2 is the FR3d 'always fits' contract these tests enforce for the content-embedded-literal case."

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/P1M1T1S1/PRP.md
  why: "The parallel sibling (S1 = the production fix + pure unit tests). Confirms the fix landed in truncatediff.go (`diffSectionBoundaryRe = (?m)^diff --git ` + FindAllStringIndex Shape-B slice) and that S1 explicitly leaves difftokenlimit_test.go to THIS task (S2). No file overlap → no conflict."
  critical: "S1's fix is the line-anchored regex slice. These E2E tests assert that fix end-to-end (the single real file is ONE section, sized, truncated to fit). S1 owns truncatediff.go + truncatediff_test.go; S2 (this) owns difftokenlimit_test.go."

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/P1M1T2S1/research/content_embedded_e2e_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-04): the S1 fix is LANDED; baseline GREEN; the test design (fixtures.diff with 300 embedded `diff --git ` blocks, TokenLimit 2000) produces all 5 assertions PASSING for all 3 diff functions (proof table: ~1001-1005 tokens, 1 sentinel, 1 real header, skeleton, no-legacy); the helper inventory with file:line; the assertion-style decision; decisions D1–D9. READ THIS FIRST."
  critical: "§3 (the proof table) is the decisive evidence the test design works before writing it. §4 (helper signatures) and §5 (assertion substrings) give the exact strings to use. §6 explains why the tests pass now (fix landed) and would have failed pre-fix (~14543 tokens, 0 sentinels)."

# The edit target + its established patterns
- file: internal/git/difftokenlimit_test.go
  why: "THE edit target. The E2E integration file for the token-limit gate. Existing scaffold: `repo := t.TempDir(); initRepo(t, repo)` then writeFile/stageFile pairs, `g := New(repo)`, then the diff call. Existing tests: TestStagedDiff_TokenLimitGt0_WaterFill (huge+small — single section, the gap), TestTreeDiff_TokenLimitGt0 (writeTreeOf for two trees), TestWorkingTreeDiff_TokenLimitGt0 (commitAllowEmpty baseline + unstaged edits). commitAllowEmpty helper at :29. The fits-budget assertion `EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin` is at the WaterFill test (~line 113) — MIRROR it."
  pattern: "Plain if/t.Errorf/t.Fatalf + strings.Contains/Count, NO testify. Each test self-contained (own t.TempDir). The file defines commitAllowEmpty as a helper — shared helpers are consistent. Assertion substrings in use: 'Change summary (numstat' (skeleton), 'diff truncated at' (legacy detector), '... [truncated]' (water-fill sentinel)."
  gotcha: "Append the 3 new tests + 2 helpers; do NOT modify the existing tests. The new tests use the SAME setup patterns as TestTreeDiff_TokenLimitGt0 (writeTreeOf) and TestWorkingTreeDiff_TokenLimitGt0 (commitAllowEmpty + unstaged edits) — mirror them. The content-embedded fixture body is built via a loop (strings.Builder), NOT strings.Repeat (the names vary)."

# Read-only helper definitions (do NOT edit)
- file: internal/git/git_test.go
  why: "READ-ONLY. initRepo(t, dir) at :13 — git init + identity (env + git config). Reused by every test. (There is NO initRepoWithUser helper — do not invent one.)"
- file: internal/git/committree_test.go
  why: "READ-ONLY. writeFile(t,dir,name,body) :31, stageFile(t,dir,name) :39, writeTreeOf(t,dir) string :48, headSHA(t,dir) string :59. The helpers the new tests reuse."
- file: internal/git/tokens.go
  why: "READ-ONLY. EstimateTokens(s string) int — the chars/4 token estimator used in the fits-budget assertion."
- file: internal/git/tokengate.go
  why: "READ-ONLY. const tokenBudgetMargin = 1024 (:48) — the safety margin; the fits-budget ceiling is tokenLimit + 2*tokenBudgetMargin."

# Read-only production refs (the code under test — do NOT edit)
- file: internal/git/truncatediff.go
  why: "READ-ONLY. diffSectionBoundaryRe = (?m)^diff --git  (:55 — the S1 fix); splitDiffSections uses FindAllStringIndex slicing (lines ~85-120). const truncatedSentinel = \"... [truncated]\". The tests assert this fix's output shape end-to-end."
- file: internal/git/git.go
  why: "READ-ONLY. StagedDiffOptions fields TokenLimit/PromptReserveTokens. The three diff functions consume opts via the shared gate (applyWaterFillGate → splitDiffSections)."

# External references
- url: https://pkg.go.dev/regexp#Regexp.FindAllStringIndex
  why: "Confirms FindAllStringIndex returns [start,end] pairs of matches; the S1 fix slices at match starts so each section begins at its header (no re-prefix). Understanding this explains WHY the content-embedded literal is inert: (?m)^diff --git matches only at column 0, and every diff content line is marker-prefixed (+/-/space/\\)."
- url: https://pkg.go.dev/testing#T.Helper
  why: "Confirm t.Helper() marks a test helper so assertion failures report the CALLER's line. The shared assertContentEmbeddedLiteralNotFragmented helper should call t.Helper()."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── difftokenlimit_test.go   # EDIT TARGET — append 3 tests + 2 helpers
    ├── git_test.go              # READ-ONLY — initRepo helper
    ├── committree_test.go       # READ-ONLY — writeFile/stageFile/writeTreeOf/headSHA helpers
    ├── truncatediff.go          # READ-ONLY — the S1 fix landed here (:55, ~85-120); code under test
    ├── tokengate.go             # READ-ONLY — tokenBudgetMargin const (1024); applyWaterFillGate
    ├── tokens.go                # READ-ONLY — EstimateTokens
    └── git.go                   # READ-ONLY — StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    internal/git/difftokenlimit_test.go   # +embeddedDiffLiteralBody + assertContentEmbeddedLiteralNotFragmented + 3 ContentEmbeddedDiffGitLiteral tests
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/difftokenlimit_test.go` | MODIFY (append) | Add the `embeddedDiffLiteralBody` body-builder, the `assertContentEmbeddedLiteralNotFragmented` helper, and 3 E2E tests (one per diff function) pinning the content-embedded-literal non-fragmentation + fits-budget property. |

**Explicitly NOT touched**: `internal/git/truncatediff.go` (the S1 fix — landed), `truncatediff_test.go`
(S1's pure unit tests), `tokengate.go` / `git.go` / `waterfill.go` / `tokens.go` / `numstat.go` /
`skeleton.go` (production code under test), other `internal/git/*_test.go` files, any other package, any
docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — the CONTENT embeds `diff --git ` literals, NOT the staging): the bug is triggered by a
// non-md file whose BODY contains lines like "+diff --git a/foo b/foo" (a documented/fixture diff). The
// file is staged normally; the literals are inside its content. Do NOT stage hundreds of real files —
// stage ONE file whose content embeds the literals. (The bug fragmented that one file into bogus sections.)

// CRITICAL (G2 — the body needs a loop, not strings.Repeat): the sample file names vary (fa, fb, …) so
// each block is distinct. Use a strings.Builder loop (the embeddedDiffLiteralBody helper). strings.Repeat
// would produce identical blocks (still triggers the bug, but the contract's repro varies the name —
// match it for realism).

// CRITICAL (G3 — the not-fragmented assertion uses the FULL real header): assert
//   Count(out, "diff --git a/"+file+" b/"+file) == 1
// where file is "fixtures.diff". The embedded literals use sample names fX (fa..fz), so they do NOT match
// "diff --git a/fixtures.diff" — the count is exactly 1 (the one real section header). Do NOT count a
// loose "diff --git a/" (that would also match the truncated embedded literals in the body). The full
// header form is unambiguous.

// CRITICAL (G4 — the fits-budget ceiling mirrors the existing test): assert
//   EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin   (tokenBudgetMargin=1024)
// This is the EXACT ceiling the existing TestStagedDiff_TokenLimitGt0_WaterFill uses (the 2× margin
// absorbs the skeleton/header overhead the gate does NOT count against bodyBudget). Before the fix, the
// same setup shipped ~14543 tokens (>> 4048); after, ~1005. Do NOT use a tighter ceiling (the overhead is
// real and absorbed by the margin by design).

// GOTCHA (G5 — exactly 1 sentinel, not >= 1): each test stages exactly ONE large file → exactly 1
// truncation. Assert Count(out, "... [truncated]") == 1 (empirically confirmed for all three). Stronger
// than the contract's >= 1 floor; catches an under-truncation regression too.

// GOTCHA (G6 — WorkingTreeDiff needs TRACKED files): untracked files do NOT appear in `git diff`. The
// WorkingTreeDiff test MUST commit a baseline (commitAllowEmpty) that tracks base.go + fixtures.diff
// (small), THEN bloat fixtures.diff UNSTAGED with the embedded-literal body. Mirrors the existing
// TestWorkingTreeDiff_TokenLimitGt0.

// GOTCHA (G7 — TreeDiff needs two real trees): use writeTreeOf for treeA (base.go only) and treeB
// (base.go + fixtures.diff). The diff treeA→treeB shows fixtures.diff as added. Mirrors the existing
// TestTreeDiff_TokenLimitGt0.

// GOTCHA (G8 — no testify, no DiffContext): use plain if/t.Errorf. The contract's scaffold is
// StagedDiffOptions{TokenLimit: N, PromptReserveTokens: 0} — do NOT add DiffContext (the existing WaterFill
// test + the contract's repro both omit it; not load-bearing for fragmentation).

// GOTCHA (G9 — the fix is landed; tests pin it): truncatediff.go:55 ALREADY has the line-anchored regex.
// These tests PASS against the current code. They are a REGRESSION NET: if anyone reverts the S1
// line-anchoring, assertions (a)/(b)/(c) fail loudly (overflow, 0 sentinels, fragmentation). Do NOT try
// to "fix" anything in production — this task is tests-only.
```

## Implementation Blueprint

### Data models and structure

No new types. The tests reuse `StagedDiffOptions{TokenLimit, PromptReserveTokens}`, `EstimateTokens`,
`tokenBudgetMargin`, and the existing helpers. Two new helpers: a body-builder (returns a string) and an
assertion helper (takes `out, tokenLimit, file`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the body-builder helper to internal/git/difftokenlimit_test.go
  - FILE: internal/git/difftokenlimit_test.go
  - PLACE: near the existing commitAllowEmpty helper (after it, before the first Test func) OR at the file
    end. Mark with a comment header.
  - WRITE (verbatim — 300-block default; distinct sample names fa..fz cycling):
        // embeddedDiffLiteralBody builds a non-markdown file body whose CONTENT contains `blocks`
        // documented `diff --git ` literals — a realistic test-fixture / golden-snapshot / .patch file.
        // Each block is a sample diff with a distinct sample filename (fa..fz cycling). Used by the
        // content-embedded-literal regression tests to prove splitDiffSections does NOT fragment one real
        // file into bogus sections (bugfix 002 Issue 1).
        func embeddedDiffLiteralBody(blocks int) string {
            var sb strings.Builder
            for i := 0; i < blocks; i++ {
                c := string(rune('a' + (i % 26)))
                sb.WriteString("diff --git a/f" + c + " b/f" + c + "\n@@ -1 +1 @@\n-old\n+new\n\n")
            }
            return sb.String()
        }
  - DO NOT: use strings.Repeat (names vary); invent a fixture on disk (the body is an in-memory string).

Task 2: ADD the shared assertion helper to internal/git/difftokenlimit_test.go
  - PLACE: right after embeddedDiffLiteralBody.
  - WRITE (verbatim — the 5 assertions, empirically proven):
        // assertContentEmbeddedLiteralNotFragmented asserts the bugfix-002 Issue-1 fix holds for a
        // content-embedded-`diff --git `-literal water-fill output: (a) the payload FITS the token budget
        // (FR3d), (b) the single large file is TRUNCATED (exactly 1 sentinel), (c) it is NOT fragmented
        // (exactly ONE real `diff --git a/<file>` header — the bug produced hundreds of bogus sections),
        // (d) the FR3g numstat skeleton is present, (e) the legacy `at N bytes/lines` sentinels are
        // absent. Shared by the three ContentEmbeddedDiffGitLiteral tests.
        func assertContentEmbeddedLiteralNotFragmented(t *testing.T, out string, tokenLimit int, file string) {
            t.Helper()
            // (a) FR3d contract: the payload fits the budget (skeleton/header overhead absorbed by 2× margin).
            if got := EstimateTokens(out); got > tokenLimit+2*tokenBudgetMargin {
                t.Errorf("content-embedded: payload %d tokens overflows token_limit %d (FR3d contract broken); out tail=\n%s", got, tokenLimit, tail(out, 200))
            }
            // (b) Truncation occurred: exactly 1 sentinel for the single large file.
            if c := strings.Count(out, "... [truncated]"); c != 1 {
                t.Errorf("content-embedded: expected exactly 1 truncation sentinel (the single large file capped), got %d; out tail=\n%s", c, tail(out, 200))
            }
            // (c) NOT fragmented: the single real file appears as exactly ONE section header. The bug
            //     fragmented one file into ~N bogus sections (one per embedded `diff --git ` literal).
            realHeader := "diff --git a/" + file + " b/" + file
            if c := strings.Count(out, realHeader); c != 1 {
                t.Errorf("content-embedded: expected exactly 1 real %q header (not fragmented), got %d; out tail=\n%s", realHeader, c, tail(out, 200))
            }
            // (d) The FR3g numstat skeleton is present (completeness floor).
            if !strings.Contains(out, "Change summary (numstat") {
                t.Errorf("content-embedded: FR3g skeleton missing; out=\n%s", out)
            }
            // (e) The legacy 'at N bytes/lines' sentinels are ABSENT on the token_limit>0 path.
            if strings.Contains(out, "diff truncated at") {
                t.Errorf("content-embedded: legacy 'diff truncated at N' sentinel must be ABSENT; out=\n%s", out)
            }
        }
  - DO NOT: import testify; tighten the budget ceiling below tokenLimit+2*margin; count a loose "diff --git a/".

Task 3: ADD TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - PLACE: after the existing TestStagedDiff_TokenLimitGt0_* tests (group with the StagedDiff tests).
  - WRITE (verbatim — mirrors the WaterFill scaffold but with ONE embedded-literal file):
        func TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // ONE non-md file whose CONTENT embeds many `diff --git ` literals (a fixture/snapshot/.patch).
            // Under a small token_limit the file MUST be capped as ONE section (the un-anchored split
            // fragmented it into bogus sections, silently overflowing the budget — bugfix 002 Issue 1).
            writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))
            stageFile(t, repo, "fixtures.diff")

            g := New(repo)
            const tokenLimit = 2000
            out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
                TokenLimit:          tokenLimit,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("StagedDiff err = %v, want nil", err)
            }
            assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
        }
  - VERIFY (after Task 6): `go test -run TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral ./internal/git/ -v` → PASS.

Task 4: ADD TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - PLACE: after the existing TestTreeDiff_TokenLimitGt0.
  - WRITE (verbatim — mirrors TestTreeDiff_TokenLimitGt0 but with the embedded-literal fixture in treeB):
        func TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // treeA: a baseline (base.go only).
            writeFile(t, repo, "base.go", "package main\n")
            stageFile(t, repo, "base.go")
            treeA := writeTreeOf(t, repo)
            // treeB: add the embedded-literal fixture.
            writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))
            stageFile(t, repo, "fixtures.diff")
            treeB := writeTreeOf(t, repo)

            g := New(repo)
            const tokenLimit = 2000
            out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{
                TokenLimit:          tokenLimit,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("TreeDiff err = %v, want nil", err)
            }
            assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
        }
  - VERIFY: `go test -run TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral ./internal/git/ -v` → PASS.

Task 5: ADD TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - PLACE: after the existing TestWorkingTreeDiff_TokenLimitGt0.
  - WRITE (verbatim — mirrors TestWorkingTreeDiff_TokenLimitGt0 but bloats the embedded-literal fixture unstaged):
        func TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // Baseline: commit base.go + fixtures.diff (small, TRACKED) so working-tree changes appear in
            // `git diff`. Untracked files do NOT appear in `git diff`.
            writeFile(t, repo, "base.go", "package main\n")
            stageFile(t, repo, "base.go")
            writeFile(t, repo, "fixtures.diff", "seed\n")
            stageFile(t, repo, "fixtures.diff")
            commitAllowEmpty(t, repo, "baseline")
            // UNSTAGED working-tree bloat of the tracked fixture with the embedded-literal body.
            writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))

            g := New(repo)
            const tokenLimit = 2000
            out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{
                TokenLimit:          tokenLimit,
                PromptReserveTokens: 0,
            })
            if err != nil {
                t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
            }
            assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
        }
  - VERIFY: `go test -run TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral ./internal/git/ -v` → PASS.

Task 6: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: gofmt -w internal/git/difftokenlimit_test.go
  - RUN: go vet ./internal/git/...
  - RUN: go test ./internal/git/...    # ALL green — 3 new tests pass; existing tests unchanged
  - RUN: go test ./...                  # whole repo green (test-only; no production touched)
  - FIX-FORWARD: if a test fails on the fits-budget assertion (overflow), the S1 fix has been reverted —
    do NOT fix production here; flag it. If it fails on the sentinel count (!= 1) or fragmentation
    (real header != 1), read `out` (printed by the helper) — but per the empirical proof all three pass at
    HEAD; a failure indicates a regression or a setup bug to diagnose from the stderr.
```

### Implementation Patterns & Key Details

```go
// === Why ONE file with embedded literals (not many real files) ===
// The bug is about a single real file whose BODY content carries `diff --git ` literals being fragmented
// by the un-anchored split. Staging many real files (each with its own genuine `diff --git` header) does
// NOT trigger the bug — those are REAL boundaries. The trigger is content-embedded literals, so the test
// stages ONE file whose content embeds them.

// === Why the not-fragmented assertion uses the FULL real header ===
// The embedded literals use sample names fX (fa..fz); the real file is fixtures.diff. The full header
// "diff --git a/fixtures.diff b/fixtures.diff" appears EXACTLY once (the one real section). The truncated
// body may contain some "+diff --git a/fX b/fX" lines, but those do NOT match the full fixtures.diff
// header. So Count(full header) == 1 robustly proves non-fragmentation. (The bug left this count at 1 too,
// but shipped 300+ bogus fX sections AND overflowed the budget — the fits-budget + sentinel assertions
// catch the overflow.)

// === The fits-budget ceiling (mirror the existing test) ===
// EstimateTokens(out) <= tokenLimit + 2*tokenBudgetMargin   (tokenBudgetMargin = 1024)
// The 2× margin absorbs the skeleton/header overhead the gate does NOT subtract from bodyBudget (by
// design — tokengate.go comment item (c)). Before the fix: ~14543 tokens >> 4048. After: ~1005. Do NOT
// tighten this ceiling.

// === The shared helper + body-builder are consistent with the file's conventions ===
// difftokenlimit_test.go already defines commitAllowEmpty as a helper; stagediff_test.go defines
// sdManyLines. embeddedDiffLiteralBody + assertContentEmbeddedLiteralNotFragmented follow the same
// pattern (plain func, t.Helper). They avoid 3×5 assertion duplication and a 3× body-loop duplication.
```

### Integration Points

```yaml
TEST (internal/git/difftokenlimit_test.go):
  - +embeddedDiffLiteralBody(blocks int) string               (body-builder)
  - +assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)  (5 assertions, t.Helper)
  - +TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - +TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - +TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral
  - each: t.TempDir + initRepo + embedded-literal fixture + function-specific setup → call → helper

CONSUMED HELPERS / CONSTS (READ-ONLY):
  - initRepo (git_test.go:13), writeFile/stageFile/writeTreeOf (committree_test.go:31-59),
    commitAllowEmpty (difftokenlimit_test.go:29), EstimateTokens (tokens.go), tokenBudgetMargin (tokengate.go:48),
    tail (truncatediff_test.go)

CODE UNDER TEST (READ-ONLY — do NOT edit):
  - internal/git/truncatediff.go:55 + ~85-120 (the S1 line-anchored fix — diffSectionBoundaryRe + FindAllStringIndex)
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
  - P1.M2.T1.S1 (docs sweep) is orthogonal; this task is unaffected.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/difftokenlimit_test.go   # Expected: empty (run gofmt -w if listed)
go vet ./internal/git/...                        # Expected: exit 0
go build ./...                                   # Expected: exit 0 (test-only; nothing else changed)

# Expected: Zero errors.
```

### Level 2: The Three New Tests (each must PASS)

```bash
cd /home/dustin/projects/stagecoach

# Each ContentEmbeddedDiffGitLiteral test in isolation:
go test -run TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral      ./internal/git/ -v
go test -run TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral        ./internal/git/ -v
go test -run TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral ./internal/git/ -v

# Expected: each PASS. The helper's 5 assertions all hold (empirically proven): fits-budget (~1001-1005
# tokens ≤ 4048), exactly 1 sentinel, exactly 1 real header, skeleton present, no legacy sentinel.
```

### Level 3: Whole-Package + Whole-Repo Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/git/...    # Expected: ALL green — 3 new tests + every existing test (incl. S1's pure
                              # truncatediff_test.go TestSplitDiffSections cases, which these complement).
go test ./...                 # Expected: ALL packages green (test-only change; no production touched).

# Confirm ONLY the one test file changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/git/difftokenlimit_test.go only. No production file, no other test file.
```

### Level 4: The Bug-Is-Gone Property (direct cross-check)

```bash
cd /home/dustin/projects/stagecoach

# The 3 new tests directly assert the fragmentation is gone. Cross-check by running the whole git suite
# and grepping for the (now impossible) overflow in any failure output:
go test ./internal/git/ -v 2>&1 | grep -E "ContentEmbedded|PASS|FAIL" | head

# Expected: 3 ContentEmbeddedDiffGitLiteral tests PASS, 0 FAIL. The overflow shape (payload >> token_limit
# + 0 sentinels) is asserted absent by the helper's assertions (a) and (b) in all three.

# (Optional belt-and-suspenders) Confirm the S1 fix is in place (the tests' raison d'être):
grep -n 'diffSectionBoundaryRe = regexp' internal/git/truncatediff.go   # → :55 (the line-anchored fix landed)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` (incl. `./internal/git/...`) — all packages green.

### Feature Validation

- [ ] `embeddedDiffLiteralBody(blocks int) string` body-builder defined (distinct sample names).
- [ ] `assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, file)` helper defined with `t.Helper()` and the 5 assertions.
- [ ] `TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` exists, stages the embedded-literal fixture, passes.
- [ ] `TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` exists (treeA/treeB via writeTreeOf), passes.
- [ ] `TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral` exists (commitAllowEmpty + unstaged), passes.
- [ ] Each test uses `TokenLimit: 2000, PromptReserveTokens: 0` and its own `t.TempDir`.
- [ ] The helper asserts: fits-budget, exactly 1 sentinel, exactly 1 real `diff --git a/<file> b/<file>` header, FR3g skeleton, no legacy sentinel.

### Scope Discipline Validation

- [ ] ONLY `internal/git/difftokenlimit_test.go` modified (`git diff --stat` confirms; 3 tests + 2 helpers appended).
- [ ] Did NOT edit production code (`truncatediff.go`, `git.go`, siblings) — the fix is already landed; this task pins it.
- [ ] Did NOT edit `truncatediff_test.go` (S1's pure unit tests) or any other `*_test.go`.
- [ ] Did NOT touch any docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] Tests follow the existing `difftokenlimit_test.go` style (plain if/t.Errorf/t.Fatalf, strings.Contains/Count, NO testify).
- [ ] Each test is self-contained (own `t.TempDir` + `initRepo`); reuses the established helpers.
- [ ] The shared helper uses `t.Helper()` so failures point at the calling test.
- [ ] Test names follow the existing `<Func>_TokenLimitGt0_<Variant>` convention.
- [ ] The body-builder uses a loop with distinct sample names (not strings.Repeat).

---

## Anti-Patterns to Avoid

- ❌ Don't stage many real files — the bug is about ONE file whose content embeds `diff --git ` literals.
  Many real files have genuine `diff --git` headers (real boundaries) and do NOT trigger the fragmentation (G1).
- ❌ Don't use `strings.Repeat` for the body — the sample names vary (fa..fz); use the `embeddedDiffLiteralBody` loop (G2).
- ❌ Don't count a loose `"diff --git a/"` for the not-fragmented assertion — it matches the truncated
  embedded literals in the body. Use the FULL header `"diff --git a/"+file+" b/"+file` (== 1) (G3).
- ❌ Don't tighten the fits-budget ceiling below `tokenLimit + 2*tokenBudgetMargin` — the 2× margin absorbs
  the skeleton/header overhead by design; the existing WaterFill test uses exactly this ceiling (G4).
- ❌ Don't assert `>= 1` sentinels when `== 1` is correct — each test stages ONE large file → exactly 1
  truncation (empirically confirmed). `== 1` is stronger (G5).
- ❌ Don't use untracked files for the WorkingTreeDiff test — commit a baseline (commitAllowEmpty) tracking
  base + fixtures.diff, THEN bloat fixtures.diff unstaged (G6).
- ❌ Don't forget `writeTreeOf` for BOTH trees in the TreeDiff test — `treeA` (base.go) and `treeB`
  (base + fixtures.diff) (G7).
- ❌ Don't import testify or add `DiffContext` — the contract's scaffold is `{TokenLimit, PromptReserveTokens}`;
  the existing WaterFill test + the contract's repro both omit DiffContext (G8).
- ❌ Don't edit production code — the S1 fix is landed; these tests PIN it. A failing fits-budget assertion
  means the fix was reverted — flag it, don't "fix" it in production here (G9).
- ❌ Don't edit `truncatediff_test.go` (S1's pure unit tests) or any other test/production file — this task
  is `difftokenlimit_test.go` only.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a pure test addition whose design is **empirically proven** against the live code before
writing. A throwaway probe exercised the exact setup (one `fixtures.diff` with 300 embedded `diff --git `
blocks, `TokenLimit: 2000, PromptReserveTokens: 0`) through the real `StagedDiff`, `TreeDiff`, and
`WorkingTreeDiff` against real temp git repos, and confirmed ALL FIVE contract assertions hold for ALL
THREE functions (fits-budget ~1001-1005 tokens ≤ 4048, exactly 1 sentinel, exactly 1 real header, skeleton
present, no-legacy). The S1 fix is confirmed landed (`truncatediff.go:55` line-anchored regex +
`FindAllStringIndex` Shape-B slice), the baseline is GREEN, and the helper inventory + assertion substrings
are grep-confirmed in the existing tests. The test bodies mirror the existing `TestTreeDiff_TokenLimitGt0` /
`TestWorkingTreeDiff_TokenLimitGt0` setups verbatim (only the file content differs), and the shared helpers
follow the file's own `commitAllowEmpty` + `sdManyLines` precedent. The prior parallel PRP (S1) is cleanly
fenced (different file). The residual 0.5 uncertainty is purely gofmt placement + whether a CI box's git
produces a marginally different body size that shifts the token count — mitigated by the proven setup
(which fits the budget with ~3000 tokens of slack: ~1005 actual vs 4048 ceiling) and the explicit "read
`out` and diagnose" fix-forward guidance. No production code, no docs, no other tests are in scope, so the
blast radius is one test file.
