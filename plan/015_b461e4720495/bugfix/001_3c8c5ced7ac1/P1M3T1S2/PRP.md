name: "P1.M3.T1.S2 — Add test for sub-floor token_limit rejection (Issue 4 / FR3j) — test-only"
description: >
  The TEST-ONLY companion to P1.M3.T1.S1 (which is FULLY LANDED): prove that the three git.go diff entry points
  (StagedDiff / TreeDiff / WorkingTreeDiff) REJECT token_limit values below the irreducible prompt floor with a clear,
  actionable error, and ACCEPT limits at the floor (the minimum valid value) and above. S1 landed the surface this task
  exercises: `func IrreducibleFloor(skeleton string, promptReserve int) int` (internal/git/tokengate.go:100, =
  EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin) and the floor check at all 3 git.go call sites
  (1057 StagedDiff / 1588 TreeDiff / 1769 WorkingTreeDiff): `floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens);
  if opts.TokenLimit < floor { return "", fmt.Errorf("token_limit %d is below the irreducible prompt floor %d (system
  prompt + numstat skeleton + framing); raise it to at least %d", opts.TokenLimit, floor, floor) }`. This task adds TWO
  test functions (NO production change, NO docs): (1) TestIrreducibleFloor_Arithmetic — a PURE table-driven unit test in
  internal/git/tokengate_test.go (alongside TestApplyWaterFillGate/TestClosedLoopGate_*) that pins the helper's formula
  (EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin) over empty/typical/multibyte skeletons + a non-zero
  reserve, and asserts the floor is always ≥ tokenBudgetMargin; (2) TestTokenLimitFloor — an E2E test in
  internal/git/difftokenlimit_test.go (alongside TestStagedDiff_TokenLimitGt0_*) that clones the established harness
  (initRepo/writeFile/stageFile/New(repo)/StagedDiff) and uses the KEY TECHNIQUE: compute the EXACT skeleton StagedDiff
  uses internally via g.StagedNumstatSkeleton (the SAME numstatSkeleton renderer with IDENTICAL args — git.go:930-938 vs
  2190-2198), so floor := IrreducibleFloor(skeleton, 0) is exact, then assert 3 sub-cases: below_floor_rejects
  (TokenLimit=floor-1 → error containing "below the irreducible prompt floor" AND naming the floor value), at_floor_succeeds
  (TokenLimit=floor → nil error, output contains the skeleton — the floor is the minimum VALID value; the strict-< check is
  FALSE so closedLoopGate runs and returns a best-effort string), above_floor_succeeds (TokenLimit=floor+100 → nil error +
  skeleton present). The boundary semantics are verified: the check is strict `<`, so exactly floor passes through (bodyBudget=0
  → best-effort, NOT an error). Tests ONLY StagedDiff (the contract's "most testable one") — the floor check is byte-identical
  at all 3 sites so StagedDiff coverage proves the pattern; a grep guard confirms all 3 sites. NO production change (S1 owns
  tokengate.go/git.go/docs — all LANDED); NO new files; NO new deps. difftokenlimit_test.go needs `"fmt"` ADDED to its import
  block (for the floor-number assertion); tokengate_test.go already imports fmt/strings/testing. Run:
  `go test ./internal/git/ -v -run TestTokenLimitFloor` (the contract's command) + `go test ./internal/git/ -v` (full regression,
  all existing tests stay GREEN — S1 is behavior-preserving: min git-layer TokenLimit=2000 > ~1024 floor).

---

## Goal

**Feature Goal**: Prove (with deterministic, hermetic tests) that Issue 4's fix works: a `token_limit` below the
irreducible prompt floor is REJECTED with a clear, actionable error (naming the floor), while a `token_limit` at the
floor (the minimum valid value) or above is ACCEPTED. This closes the test-coverage gap S1 explicitly left: S1 landed
the `IrreducibleFloor` helper + the 3 caller checks but assigned the test to S2 ("NO test in this item — P1.M3.T1.S2
owns the test"), so the floor-rejection BRANCH is currently UNCOVERED. This task covers it.

**Deliverable** (test-only — 2 new test functions, 1 import added; NO production change, NO docs, NO new files):
1. **internal/git/tokengate_test.go** — `TestIrreducibleFloor_Arithmetic` (PURE table-driven unit test of the helper's formula).
2. **internal/git/difftokenlimit_test.go** — `TestTokenLimitFloor` (E2E test of StagedDiff's sub-floor rejection + at/above-floor acceptance, with 3 subtests) + `"fmt"` import added.

**Success Definition**:
- `TestIrreducibleFloor_Arithmetic` passes: for empty/typical/multibyte skeletons + a non-zero reserve,
  `IrreducibleFloor(skeleton, promptReserve) == EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin`, and the
  floor is always ≥ `tokenBudgetMargin` (1024).
- `TestTokenLimitFloor` passes with 3 green subtests: `below_floor_rejects` (TokenLimit=floor-1 → non-nil error
  containing "below the irreducible prompt floor" and naming the floor value), `at_floor_succeeds` (TokenLimit=floor →
  nil error, output contains "Change summary (numstat"), `above_floor_succeeds` (TokenLimit=floor+100 → nil error +
  skeleton present).
- The contract's run command passes: `go test ./internal/git/ -v -run TestTokenLimitFloor`.
- The full `go test ./internal/git/ -v` stays GREEN (S1 is behavior-preserving; the new tests only ADD coverage).
- `go build ./...` clean; `gofmt -l` empty on the 2 edited files; `make lint` + `make coverage-gate` green (the new
  tests cover the previously-uncovered floor-rejection branch — internal/git coverage goes UP).
- Scope: `git diff --name-only` == {internal/git/tokengate_test.go, internal/git/difftokenlimit_test.go}. NO production
  file touched (tokengate.go/git.go/docs are S1's, LANDED + READ-ONLY).

## User Persona (if applicable)

**Target User**: The maintainer who needs confidence that Issue 4's "fail loud on sub-floor token_limit" fix actually
fires (and does NOT fire at/above the floor) — i.e. regression protection so a future refactor cannot silently revert
the floor check or break its boundary semantics.

**Use Case**: CI runs `go test ./internal/git/ -v -run TestTokenLimitFloor` on every change; if someone removes the
floor check, changes `<` to `<=`, or miscalculates the floor, `below_floor_rejects` or `at_floor_succeeds` fails.

**Pain Points Addressed**: S1's PRP explicitly flagged that the floor-rejection branch is uncovered until S2 lands ("the
new floor-rejection BRANCH is NOT exercised by any existing test … S2 adds exactly that coverage"). This task is that
coverage — it removes the coverage-gate risk S1 called out.

## Why

- **Issue 4 / FR3j**: the PRD states the FR3j closed-loop guarantee holds "always", but before S1 a sub-floor
  `token_limit` silently produced an over-budget payload. S1 made it fail loud; S2 PROVES it fails loud (and only
  below the floor).
- **The at-floor boundary is subtle and worth pinning**: the check is strict `<`, so `TokenLimit == floor` passes
  through to closedLoopGate (bodyBudget=0 → best-effort slivers, NOT an error). A future "tightening" to `<=` would
  break the documented "the floor is the minimum valid value" contract. `at_floor_succeeds` pins this.
- **Bounded, test-only scope**: 2 test functions + 1 import. No production change, no docs, no new files. The new tests
  only ADD coverage (the floor-rejection branch S1 left uncovered).

## What

**User-visible behavior**: None (test-only). The tests verify the behavior S1 shipped: sub-floor `token_limit` errors
with an actionable message; at/above-floor succeeds.

**Technical change**: 2 test functions (1 pure, 1 E2E with 3 subtests) + 1 import line. See the Implementation Blueprint
for the verbatim test code.

### Success Criteria
- [ ] `internal/git/tokengate_test.go` adds `TestIrreducibleFloor_Arithmetic` (PURE, table-driven) asserting
      `IrreducibleFloor(skeleton, promptReserve) == EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin`
      for ≥4 cases (empty/typical/multibyte skeleton + a non-zero reserve) + the `≥ tokenBudgetMargin` invariant.
- [ ] `internal/git/difftokenlimit_test.go` adds `TestTokenLimitFloor` (E2E) with 3 subtests using the EXACT floor
      computed via `g.StagedNumstatSkeleton`: `below_floor_rejects` (floor-1 → error), `at_floor_succeeds` (floor → nil),
      `above_floor_succeeds` (floor+100 → nil).
- [ ] `difftokenlimit_test.go` imports `"fmt"` (for the floor-number assertion in `below_floor_rejects`).
- [ ] `go test ./internal/git/ -v -run TestTokenLimitFloor` GREEN (the contract's run command).
- [ ] `go test ./internal/git/ -v -run TestIrreducibleFloor_Arithmetic` GREEN.
- [ ] `go test ./internal/git/ -v` GREEN (full regression — existing tests unchanged).
- [ ] `go build ./...` clean; `gofmt -l` empty on the 2 files; `make lint` + `make coverage-gate` green.
- [ ] Scope: `git diff --name-only` == {internal/git/tokengate_test.go, internal/git/difftokenlimit_test.go}.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim test code for both functions (ready to paste, anchored to the verified S1 signatures), the proof
that S1 is LANDED (IrreducibleFloor at tokengate.go:100; floor check at git.go 1057/1588/1769; exact error string), the
KEY TECHNIQUE (StagedNumstatSkeleton produces the byte-identical skeleton StagedDiff uses internally — same numstatSkeleton
args at git.go:930-938 vs 2190-2198 — so the floor is exact and the boundaries are precise), the boundary semantics (strict
`<`: floor-1 errors, floor passes through to closedLoopGate best-effort with nil error, floor+100 nil error), the constants
(tokenBudgetMargin=1024, EstimateTokens=ceil(runes/4)), the E2E harness to clone (initRepo/writeFile/stageFile/New — all
in-package, from difftokenlimit_test.go), the import situation (difftokenlimit_test.go needs `"fmt"` added; tokengate_test.go
already has fmt/strings/testing), the test-placement rationale (pure helper test → tokengate_test.go; E2E wiring test →
difftokenlimit_test.go), the run commands, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative codebase findings (S1 state, the key technique, the boundary semantics, the harness)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M3T1S2/research/findings.md
  why: "§0 S1 is FULLY LANDED (IrreducibleFloor @ tokengate.go:100; floor check @ git.go 1057/1588/1769; exact error string);
        §1 the constants + the helper formula; §2 the strict-< boundary semantics (at-floor passes through → nil err);
        §3 the KEY TECHNIQUE — StagedNumstatSkeleton shares numstatSkeleton with StagedDiff (identical args) ⇒ exact floor;
        §4 the E2E harness to clone + the 'fmt' import need; §5 test placement; §6 why only StagedDiff; §7 scope fences."
  critical: "§0/§2: S1 is LANDED — test it, don't re-implement. The check is strict `<` so at_floor_succeeds MUST assert nil
             err (floor passes through to closedLoopGate best-effort). §3: use g.StagedNumstatSkeleton to get the EXACT skeleton
             — do NOT hardcode/predict the skeleton string (fragile). §4: difftokenlimit_test.go needs 'fmt' added."

# MUST READ — the S1 PRP (the surface this task tests + the explicit "S2 owns the test" handoff)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M3T1S1/PRP.md
  why: "Defines the exact IrreducibleFloor signature + formula, the verbatim error string, the 3 call sites, the strict-<
        boundary, and the explicit S1/S2 split ('NO test in this item — P1.M3.T1.S2 owns the test'). S1's coverage note
        flags the floor-rejection branch as uncovered until S2 — this task is that coverage."
  critical: "The error string is verbatim: 'token_limit %d is below the irreducible prompt floor %d (system prompt + numstat
             skeleton + framing); raise it to at least %d'. Assert the substring 'below the irreducible prompt floor'. S1
             chose strict `<` (NOT `<=`) — at_floor_succeeds relies on that."

# MUST READ — the bug + the chosen fix (Option a: caller-level rejection; the helper code; the docs-impact note)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/minor_fixes.md
  section: "Issue 4: Token Gate Sub-270 Invariant Violation (FR3j)"
  why: "States the problem, the constants, the 3 call sites, the verbatim IrreducibleFloor helper + floor check, and
        Option (a) over Option (b) (closedLoopGate signature UNCHANGED). Confirms the floor formula and the test file
        (tokengate_test.go)."
  critical: "The floor = EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin (NOT including minBodyTokens)."

# MUST EDIT — tokengate_test.go (add the PURE arithmetic test)
- file: internal/git/tokengate_test.go
  why: "package git (in-package — calls IrreducibleFloor + EstimateTokens + tokenBudgetMargin directly). Imports fmt/strings/
        testing (all present — NO new import). Home of TestApplyWaterFillGate/TestClosedLoopGate_* — the pure helper-test
        siblings. TestIrreducibleFloor_Arithmetic fits here (it tests the tokengate.go helper)."
  pattern: "tokengate_test.go's tests are PURE table-driven (t.Run subtests, HARDCODED string literals, no t.TempDir, no I/O).
            Mirror that style: a []struct{name, skeleton, promptReserve} table, t.Run per case, assert got == want."
  gotcha: "IrreducibleFloor is the file's FIRST exported test target that is also a pure helper — but EstimateTokens and
           tokenBudgetMargin are in-package, so the assertion `EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin`
           reads them directly (no import). Do NOT export tokenBudgetMargin (S1 kept it unexported; the in-package test reads it)."

# MUST EDIT — difftokenlimit_test.go (add the E2E boundary test + 'fmt' import)
- file: internal/git/difftokenlimit_test.go
  why: "package git (in-package). Home of TestStagedDiff_TokenLimitGt0_WaterFill etc. — the StagedDiff-TokenLimit E2E siblings
        + the helpers (initRepo/writeFile/stageFile/New/commitAllowEmpty). TestTokenLimitFloor fits here (it tests StagedDiff's
        floor wiring). Imports today: context, os/exec, strings, testing — NO 'fmt' (ADD it for the floor-number assertion)."
  pattern: "Clone TestStagedDiff_TokenLimitGt0_WaterFill's harness verbatim: repo := t.TempDir(); initRepo(t, repo);
            writeFile(t, repo, 'a.go', ...); stageFile(t, repo, 'a.go'); g := New(repo); g.StagedDiff(ctx, StagedDiffOptions{...})."
  gotcha: "Use g.StagedNumstatSkeleton(ctx, opts) to compute the EXACT skeleton (it shares numstatSkeleton with StagedDiff —
           identical args). Do NOT hardcode/predict the skeleton string. Add 'fmt' to the import block. The at-floor subtest
           MUST assert err == nil (the strict-< check passes at exactly floor)."

# CONTEXT — the S1 deliverables this task tests (LANDED; READ-ONLY)
- file: internal/git/tokengate.go   # IrreducibleFloor (:100) — the helper under arithmetic test
- file: internal/git/git.go         # StagedDiff floor check (:1057) — the wiring under E2E test; StagedNumstatSkeleton (:2190)
  why: "Confirms the exact signatures + the error string the tests assert against. StagedDiff's skeleton (git.go:930-938) and
        StagedNumstatSkeleton's (git.go:2190-2198) build IDENTICAL numstatSkeleton args ⇒ the test's computed floor is exact."
  critical: "READ-ONLY. Do NOT edit tokengate.go/git.go (S1 LANDED). The floor check is strict `<` (at-floor passes through)."

# CONTEXT — the estimator (the helper composes it; the arithmetic test asserts it)
- file: internal/git/tokens.go   # EstimateTokens (:25) = ceil(utf8.RuneCountInString(s)/4) — rune-based
  why: "The helper's first term. Rune-based (chars/4) — so a multibyte skeleton (CJK) estimates by runes, not bytes. The
        arithmetic test's multibyte case verifies this (the floor uses EstimateTokens, not len())."

# CONTEXT — the architect's chosen fix (Option a) + the docs-impact note
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/test_patterns.md
  why: "Confirms the test conventions (table-driven, t.Run subtests) and that tokengate_test.go is the token-gate test home.
        Notes docs/configuration.md:167 (S1's docs change — NOT this task)."
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  tokengate.go            # READ-ONLY (S1 LANDED) — IrreducibleFloor (:100), tokenBudgetMargin (:48), applyWaterFillGate/closedLoopGate
  git.go                  # READ-ONLY (S1 LANDED) — StagedDiff floor check (:1057), StagedNumstatSkeleton (:2190), numstatSkeleton
  tokens.go               # READ-ONLY — EstimateTokens (:25, ceil(runes/4))
  tokengate_test.go       # EDIT — +TestIrreducibleFloor_Arithmetic (PURE, no new import)
  difftokenlimit_test.go  # EDIT — +TestTokenLimitFloor (E2E, 3 subtests) + "fmt" import
  (other *_test.go)       # READ-ONLY — regression net (full go test ./internal/git/ stays green)
# go.mod / Makefile — READ-ONLY (no new dep; make test / lint / coverage-gate)
```

### Desired Codebase tree with files to be modified

```bash
# MODIFIED (no new files). EXACTLY 2 files:
internal/git/tokengate_test.go       # +TestIrreducibleFloor_Arithmetic (PURE table-driven, ~30 lines)
internal/git/difftokenlimit_test.go  # +TestTokenLimitFloor (E2E, 3 subtests, ~55 lines) + "fmt" import
# (NOT touched: tokengate.go/git.go [S1 LANDED], docs/configuration.md [S1], any other source — test-only)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 is LANDED — test it, don't re-implement): IrreducibleFloor @ tokengate.go:100 and the floor check @
// git.go:1057/1588/1769 are ALREADY in the tree (grep -c confirms 3 sites). This task adds ONLY test functions.
// Do NOT edit tokengate.go/git.go/docs — they are S1's deliverables (READ-ONLY here).

// CRITICAL (use StagedNumstatSkeleton for the EXACT floor — do NOT hardcode the skeleton): the skeleton StagedDiff uses
// internally is computed by numstatSkeleton(ctx, ["--cached","-M","--",excludes...]). StagedNumstatSkeleton (git.go:2190)
// builds the IDENTICAL args and calls the same renderer. So g.StagedNumstatSkeleton(ctx, opts) returns the byte-identical
// skeleton ⇒ floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens) is EXACT ⇒ the floor-1/floor/floor+100 boundaries
// are precise. Hardcoding/predicting the skeleton string is FRAGILE (the numstat format or excludes could change).

// CRITICAL (the at-floor subtest MUST assert err == nil — the check is strict `<`): at TokenLimit == floor, the check
// `if opts.TokenLimit < floor` is FALSE → proceeds to closedLoopGate. bodyBudget = floor − skel − reserve − margin = 0 →
// budgetExhausted=true → bodies cut to minBodyTokens slivers → closedLoopGate returns a best-effort STRING (it returns
// `string`, NOT (string,error)). StagedDiff returns (b.String(), nil) — b holds the skeleton (written at git.go:942 BEFORE
// the token-limit branch). So at-floor: err == nil AND out contains "Change summary (numstat". Do NOT assert an error at floor.

// GOTCHA (difftokenlimit_test.go needs "fmt" ADDED): it imports context/os/exec/strings/testing today (NO fmt). The
// below_floor_rejects subtest's floor-number assertion (fmt.Sprintf("floor %d", floor)) needs it. Add "fmt" to the import
// block. (tokengate_test.go ALREADY imports fmt — no change there.) If you prefer to avoid the import, drop the floor-number
// check and assert only the substring "below the irreducible prompt floor" — but the floor-number check proves the error is
// ACTIONABLE (names the floor), which is a core part of Issue 4's value, so prefer keeping it + adding the import.

// GOTCHA (the floor is ALWAYS ≥ tokenBudgetMargin = 1024): even an empty skeleton + 0 reserve ⇒ floor = 0 + 0 + 1024 = 1024.
// This is WHY every existing git-layer test (min TokenLimit = 2000 in difftokenlimit_test.go) stays ABOVE the floor and is
// UNAFFECTED by S1's check (behavior-preserving). The arithmetic test asserts this invariant (got >= tokenBudgetMargin).

// GOTCHA (EstimateTokens is RUNE-based, ceil(runes/4) — NOT len()/4): a multibyte skeleton (e.g. "中文.go") estimates by
// utf8.RuneCountInString, not bytes. The arithmetic test's multibyte case verifies the floor uses EstimateTokens (so a CJK
// path is not over-counted). Do NOT replace EstimateTokens(skeleton) with len(skeleton)/4 in any assertion.

// GOTCHA (tokenBudgetMargin stays UNEXPORTED — the in-package test reads it directly): S1 kept tokenBudgetMargin unexported
// (tokengate.go:48); only IrreducibleFloor is exported. The arithmetic test is in package git (internal) so it reads the
// unexported tokenBudgetMargin const directly in its `want` expression. Do NOT export tokenBudgetMargin.
```

## Implementation Blueprint

### Data models and structure

None. No production change, no new types. Two test functions (one pure table-driven, one E2E with 3 subtests).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/tokengate_test.go — ADD TestIrreducibleFloor_Arithmetic (the PURE arithmetic test)
  - PLACE: after the last existing test (TestClosedLoopGate_EffectiveLimitFloor, ~:445) or near the other pure helper
    tests. APPEND at end of file (gofmt-clean).
  - NO new import (tokengate_test.go already imports fmt/strings/testing; IrreducibleFloor + EstimateTokens +
    tokenBudgetMargin are in-package).
  - ADD (verbatim — PURE table-driven, mirrors tokengate_test.go's style):
        // TestIrreducibleFloor_Arithmetic pins the IrreducibleFloor helper's formula (Issue 4 / FR3j): the floor is the
        // non-body minimum — EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin. PURE (no git repo, no I/O).
        // The E2E test (difftokenlimit_test.go TestTokenLimitFloor) consumes this floor at the StagedDiff call site.
        func TestIrreducibleFloor_Arithmetic(t *testing.T) {
            cases := []struct {
                name          string
                skeleton      string
                promptReserve int
            }{
                {"empty_skeleton_no_reserve", "", 0},
                {"typical_skeleton_no_reserve", "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n5\t0\thelper.go\n", 0},
                {"skeleton_with_reserve", "1\t0\tf.go\n", 512},
                {"multibyte_skeleton", "5\t2\t中文.go\n", 0}, // rune-based EstimateTokens (chars/4, NOT bytes)
            }
            for _, tc := range cases {
                t.Run(tc.name, func(t *testing.T) {
                    got := IrreducibleFloor(tc.skeleton, tc.promptReserve)
                    want := EstimateTokens(tc.skeleton) + tc.promptReserve + tokenBudgetMargin
                    if got != want {
                        t.Errorf("IrreducibleFloor(%q, %d) = %d, want %d (EstimateTokens=%d + reserve=%d + margin=%d)",
                            tc.skeleton, tc.promptReserve, got, want,
                            EstimateTokens(tc.skeleton), tc.promptReserve, tokenBudgetMargin)
                    }
                    // The floor is ALWAYS ≥ tokenBudgetMargin (1024) — the margin alone floors it even with an empty
                    // skeleton + zero reserve. This is WHY every existing git-layer test (TokenLimit ≥ 2000) stays above
                    // the floor (S1 behavior-preserving).
                    if got < tokenBudgetMargin {
                        t.Errorf("floor %d < tokenBudgetMargin %d (the margin alone floors it)", got, tokenBudgetMargin)
                    }
                })
            }
        }
  - NAMING: TestIrreducibleFloor_Arithmetic (descriptive; sits with the TestClosedLoopGate_* siblings).
  - COVERAGE: the formula over empty/typical/multibyte skeletons + a non-zero reserve + the ≥-margin invariant.

Task 2: EDIT internal/git/difftokenlimit_test.go — ADD "fmt" to the import block
  - OLD (the import block, ~:3-9):
        import (
            "context"
            "os/exec"
            "strings"
            "testing"
        )
  - NEW:
        import (
            "context"
            "fmt"
            "os/exec"
            "strings"
            "testing"
        )
  - WHY: Task 3's below_floor_rejects subtest uses fmt.Sprintf("floor %d", floor) to assert the error NAMES the floor
    value (actionable). gofmt keeps the block sorted.
  - GOTCHA: ONLY add "fmt" — do not touch the other imports. context/strings/testing are already used by the existing
    tests (and by Task 3).

Task 3: EDIT internal/git/difftokenlimit_test.go — ADD TestTokenLimitFloor (the E2E boundary test)
  - PLACE: APPEND at end of file (after TestWorkingTreeDiff_TokenLimitGt0, the last test). gofmt-clean.
  - ADD (verbatim — clones the TestStagedDiff_TokenLimitGt0_WaterFill harness; uses StagedNumstatSkeleton for the exact floor):
        // TestTokenLimitFloor pins Issue 4's fix (FR3j): StagedDiff REJECTS token_limit below the irreducible prompt floor
        // with a clear error, and ACCEPTS it at the floor (the minimum valid value — the strict-< check passes through to
        // closedLoopGate's best-effort) and above. The floor is computed from the EXACT skeleton StagedDiff uses internally
        // via StagedNumstatSkeleton (the same numstatSkeleton renderer — git.go:930-938 vs 2190-2198 — identical args), so
        // the floor-1/floor/floor+100 boundaries are precise. Run: go test ./internal/git/ -v -run TestTokenLimitFloor.
        func TestTokenLimitFloor(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            // Stage a couple of files so the numstat skeleton is non-empty (⇒ floor > tokenBudgetMargin alone).
            writeFile(t, repo, "a.go", "package main\n// a\n")
            stageFile(t, repo, "a.go")
            writeFile(t, repo, "b.go", "package lib\n// b\n")
            stageFile(t, repo, "b.go")

            g := New(repo)
            // Compute the EXACT skeleton StagedDiff will use (StagedNumstatSkeleton shares numstatSkeleton with StagedDiff —
            // identical args, identical output). PromptReserveTokens=0 for a tight, predictable floor.
            opts := StagedDiffOptions{DiffContext: 1, PromptReserveTokens: 0}
            skeleton, serr := g.StagedNumstatSkeleton(context.Background(), opts)
            if serr != nil {
                t.Fatalf("StagedNumstatSkeleton: %v", serr)
            }
            if skeleton == "" {
                t.Fatal("skeleton empty — nothing staged (fixture setup failed)")
            }
            floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens)

            // (a) BELOW the floor (floor-1): the floor check fires → error naming the floor.
            t.Run("below_floor_rejects", func(t *testing.T) {
                o := opts
                o.TokenLimit = floor - 1
                _, err := g.StagedDiff(context.Background(), o)
                if err == nil {
                    t.Fatal("StagedDiff err = nil, want non-nil (token_limit below the floor must be rejected)")
                }
                if !strings.Contains(err.Error(), "below the irreducible prompt floor") {
                    t.Errorf("err = %q, want it to contain 'below the irreducible prompt floor'", err.Error())
                }
                // The error names the floor value (actionable — the user knows exactly what to set).
                if !strings.Contains(err.Error(), fmt.Sprintf("floor %d", floor)) {
                    t.Errorf("err = %q, want it to name the floor value %d", err.Error(), floor)
                }
            })

            // (b) AT the floor (exactly): the strict-< check is FALSE → proceeds to closedLoopGate → returns a
            // best-effort string, NO error. The floor is the minimum VALID value.
            t.Run("at_floor_succeeds", func(t *testing.T) {
                o := opts
                o.TokenLimit = floor
                out, err := g.StagedDiff(context.Background(), o)
                if err != nil {
                    t.Fatalf("StagedDiff at floor err = %v, want nil (the floor is the minimum valid value)", err)
                }
                // The skeleton is always prepended (git.go:942, BEFORE the token-limit branch) — proves the gate ran.
                if !strings.Contains(out, "Change summary (numstat") {
                    t.Errorf("at-floor output missing the skeleton; out=\n%s", out)
                }
            })

            // (c) ABOVE the floor (floor+100): normal path, no error.
            t.Run("above_floor_succeeds", func(t *testing.T) {
                o := opts
                o.TokenLimit = floor + 100
                out, err := g.StagedDiff(context.Background(), o)
                if err != nil {
                    t.Fatalf("StagedDiff above floor err = %v, want nil", err)
                }
                if !strings.Contains(out, "Change summary (numstat") {
                    t.Errorf("above-floor output missing the skeleton; out=\n%s", out)
                }
            })
        }
  - WHY the technique works: StagedNumstatSkeleton (git.go:2190) builds args = ["--cached","-M","--",defaultExcludes...,
    opts.Excludes...] — IDENTICAL to StagedDiff's internal skeleton (git.go:930-938). Same renderer (numstatSkeleton) ⇒
    byte-identical skeleton ⇒ floor is exact ⇒ boundaries are precise.
  - WHY at_floor_succeeds asserts nil err: the check is strict `<`; at TokenLimit==floor the check is FALSE → proceeds to
    closedLoopGate (bodyBudget=0 → best-effort slivers) → returns (string, nil). The skeleton is always prepended.
  - FOLLOW pattern: TestStagedDiff_TokenLimitGt0_WaterFill (initRepo + writeFile + stageFile + New + StagedDiff).
  - NAMING: TestTokenLimitFloor (matches the contract's run command `-run TestTokenLimitFloor`).

Task 4: VERIFY — build, vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./...
  - go vet ./internal/git/...
  - gofmt -l internal/git/tokengate_test.go internal/git/difftokenlimit_test.go   # must be empty
  - go test ./internal/git/ -v -run TestTokenLimitFloor                  # the contract's command (E2E + 3 subtests)
  - go test ./internal/git/ -v -run TestIrreducibleFloor_Arithmetic       # the pure arithmetic test
  - go test ./internal/git/ -v                                            # full regression (all existing tests GREEN)
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the KEY TECHNIQUE — exact floor via the shared renderer): StagedNumstatSkeleton produces the same skeleton
// StagedDiff uses internally, so the test's floor is exact (not approximate/hardcoded):
opts := StagedDiffOptions{DiffContext: 1, PromptReserveTokens: 0}
skeleton, _ := g.StagedNumstatSkeleton(context.Background(), opts)   // == StagedDiff's internal skeleton (git.go:930-938 == 2190-2198)
floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens)        // exact — the same floor StagedDiff computes

// PATTERN (the 3 boundary subtests — strict-< semantics):
o.TokenLimit = floor - 1  // → err != nil, contains "below the irreducible prompt floor" + names the floor
o.TokenLimit = floor      // → err == nil (strict-< is FALSE at floor; closedLoopGate best-effort; skeleton prepended)
o.TokenLimit = floor + 100 // → err == nil (normal path)

// PATTERN (the pure arithmetic test — pins the formula, no git):
want := EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin   // the 3 floor terms (NO minBodyTokens)
if IrreducibleFloor(skeleton, promptReserve) != want { ... }
// + invariant: floor >= tokenBudgetMargin (1024) always — WHY existing tests (TokenLimit >= 2000) are unaffected.
```

### Integration Points

```yaml
CODE (internal/git — TEST ONLY):
  - tokengate_test.go: +TestIrreducibleFloor_Arithmetic (PURE; no new import).
  - difftokenlimit_test.go: +TestTokenLimitFloor (E2E; 3 subtests) + "fmt" import.

NO-CHANGE (scope fences):
  - tokengate.go / git.go / docs/configuration.md: S1 LANDED — READ-ONLY (do NOT edit; this task only TESTS the landed code).
  - closedLoopGate / applyWaterFillGate signatures + the bodyBudget≤0 path: UNCHANGED (S1 chose Option a).
  - tokenBudgetMargin: stays UNEXPORTED (the in-package test reads it directly).
  - TreeDiff / WorkingTreeDiff floor checks: NOT directly tested (StagedDiff is the contract's "most testable one"; the
    check is byte-identical at all 3 sites — grep-guarded).
  - Issue 5 (doubled prefix) / Issue 6 (auto-stage grammar): separate tasks (P1.M3.T2/T3).

CONSUMERS OF THIS CHANGE:
  - The floor-rejection BRANCH (git.go:1059 `if opts.TokenLimit < floor`) — UNCOVERED by any existing test (S1 left it for
    S2) — is now covered by TestTokenLimitFloor/below_floor_rejects. internal/git coverage goes UP (resolves S1's
    coverage-gate coordination risk).

NO database / migration / routes / new types / new files / new deps / production change / docs change.
  - The only non-test line added is the "fmt" import in difftokenlimit_test.go.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (test-only change; the test files must compile — esp. the new "fmt" import + the StagedNumstatSkeleton call).
go build ./...
# Expected: clean. A failure usually means a typo in the test, a missing "fmt" import, or calling StagedNumstatSkeleton
#           with the wrong signature.

# Vet.
go vet ./internal/git/...
# Expected: clean.

# Format.
gofmt -l internal/git/tokengate_test.go internal/git/difftokenlimit_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. The new tests use every imported symbol (fmt.Sprintf, strings.Contains, context.Background, etc.)
#           — no `unused`. IrreducibleFloor is used by both tests; EstimateTokens/tokenBudgetMargin by the arithmetic test.

# Scope guard: ONLY the 2 test files changed.
git diff --name-only
# Expected: internal/git/tokengate_test.go  internal/git/difftokenlimit_test.go  (exactly these 2).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The contract's run command — the E2E boundary test + its 3 subtests.
go test ./internal/git/ -v -run TestTokenLimitFloor
# Expected: PASS — below_floor_rejects (floor-1 → error), at_floor_succeeds (floor → nil), above_floor_succeeds (floor+100 → nil).

# The pure arithmetic test.
go test ./internal/git/ -v -run TestIrreducibleFloor_Arithmetic
# Expected: PASS — the formula holds for all 4 cases + the ≥-margin invariant.

# Both new tests together.
go test ./internal/git/ -v -run 'TestTokenLimitFloor|TestIrreducibleFloor_Arithmetic'
# Expected: PASS.

# Full regression — ALL existing git tests MUST stay GREEN (S1 is behavior-preserving; this task adds tests only).
go test ./internal/git/ -v
# Expected: GREEN. The existing TestStagedDiff_TokenLimitGt0_* / TestClosedLoopGate_* / TestApplyWaterFillGate etc. are
#           UNCHANGED (min TokenLimit=2000 > ~1024 floor ⇒ S1's check never fires for them).

# Full race suite + coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make test
make coverage-gate
# Expected: green / passes. internal/git coverage goes UP — the new tests cover the floor-rejection branch S1 left
#           uncovered (resolving S1's flagged coverage-gate coordination risk).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (no production change, but proves the package still links into the binary).
make build

# (The deterministic unit/E2E tests in Level 2 ARE the within-scope proof. A manual --token-limit e2e is S1's Level 3,
#  already covered there — not repeated here. This task is test-only.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: BOTH new tests exist.
grep -n 'func TestIrreducibleFloor_Arithmetic' internal/git/tokengate_test.go
grep -n 'func TestTokenLimitFloor' internal/git/difftokenlimit_test.go
# Expected: 1 hit each.

# Guard 2: TestTokenLimitFloor has the 3 boundary subtests.
grep -n 'below_floor_rejects\|at_floor_succeeds\|above_floor_succeeds' internal/git/difftokenlimit_test.go
# Expected: 3 hits (one per t.Run).

# Guard 3: the E2E test uses StagedNumstatSkeleton (the exact-floor technique — NOT a hardcoded skeleton).
grep -n 'StagedNumstatSkeleton' internal/git/difftokenlimit_test.go
# Expected: ≥1 hit (the skeleton computation). And NO hardcoded skeleton assertion like `floor == <constant>`.

# Guard 4: the at-floor subtest asserts err == nil (strict-< semantics — at-floor passes through).
grep -A3 'at_floor_succeeds' internal/git/difftokenlimit_test.go | grep 'err != nil'
# Expected: the assertion `if err != nil { t.Fatalf(... want nil ...) }` is present (proving at-floor expects NO error).

# Guard 5: the error-substring assertion matches the S1 error string.
grep -n 'below the irreducible prompt floor' internal/git/difftokenlimit_test.go
# Expected: ≥1 hit (the strings.Contains assertion in below_floor_rejects).

# Guard 6: difftokenlimit_test.go imported "fmt".
grep -n '"fmt"' internal/git/difftokenlimit_test.go
# Expected: 1 hit.

# Guard 7: NO production file changed (test-only scope).
git diff --name-only | grep -vE '_test\.go$'
# Expected: EMPTY (only the 2 _test.go files changed; no tokengate.go/git.go/docs).

# Guard 8: the S1 deliverables are intact (READ-ONLY — this task did not touch them).
grep -c 'func IrreducibleFloor(skeleton string, promptReserve int) int' internal/git/tokengate.go
# Expected: 1 (unchanged). And:
grep -c 'IrreducibleFloor(skeleton, opts.PromptReserveTokens)' internal/git/git.go
# Expected: 3 (all 3 floor-check sites intact — this task only tests them, does not edit them).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/git/...` clean
- [ ] `gofmt -l internal/git/tokengate_test.go internal/git/difftokenlimit_test.go` empty
- [ ] `make lint` zero errors (all imported symbols used)
- [ ] `go test ./internal/git/ -v -run TestTokenLimitFloor` GREEN (3 subtests)
- [ ] `go test ./internal/git/ -v -run TestIrreducibleFloor_Arithmetic` GREEN
- [ ] `go test ./internal/git/ -v` GREEN (full regression)
- [ ] `make test` (race) + `make coverage-gate` green (coverage UP — floor-rejection branch now covered)

### Feature Validation
- [ ] TestIrreducibleFloor_Arithmetic pins the formula (EstimateTokens + promptReserve + tokenBudgetMargin) + ≥-margin invariant
- [ ] TestTokenLimitFloor/below_floor_rejects: floor-1 → error containing "below the irreducible prompt floor" + naming the floor
- [ ] TestTokenLimitFloor/at_floor_succeeds: floor → nil error + skeleton present (strict-< passes through)
- [ ] TestTokenLimitFloor/above_floor_succeeds: floor+100 → nil error + skeleton present
- [ ] The E2E floor is computed via StagedNumstatSkeleton (exact, not hardcoded)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == {internal/git/tokengate_test.go, internal/git/difftokenlimit_test.go}
- [ ] NO production file touched (tokengate.go/git.go/docs are S1's — LANDED + READ-ONLY)
- [ ] NO test for TreeDiff/WorkingTreeDiff (StagedDiff is the contract's "most testable one"; check is byte-identical at all 3)
- [ ] NO change to Issue 5/6 (separate tasks P1.M3.T2/T3)
- [ ] tokenBudgetMargin stays unexported (in-package test reads it directly)
- [ ] Grep guards 1–8 (Level 4) all pass

### Code Quality & Docs
- [ ] Both tests are hermetic (t.TempDir; no shared state; no network)
- [ ] Test names match the contract's run command (`-run TestTokenLimitFloor`)
- [ ] Subtest names are descriptive (below_floor_rejects / at_floor_succeeds / above_floor_succeeds)
- [ ] Comments cite Issue 4 / FR3j + the StagedNumstatSkeleton technique + the strict-< boundary rationale

---

## Anti-Patterns to Avoid

- ❌ Don't edit `internal/git/tokengate.go`, `internal/git/git.go`, or `docs/configuration.md`. Those are S1's deliverables
  and they are LANDED. This task is TEST-ONLY — it adds test functions that exercise S1's landed code. Editing production
  crosses the S1/S2 boundary (and the grep guard 7 catches it).
- ❌ Don't hardcode/predict the skeleton string in the E2E test. The skeleton StagedDiff uses is computed internally by
  numstatSkeleton; its exact bytes depend on the numstat format + the exclude union. Instead, compute it via
  `g.StagedNumstatSkeleton(ctx, opts)` (git.go:2190) which builds the IDENTICAL args (git.go:930-938 == 2190-2198) and
  returns the byte-identical skeleton. Then `floor := IrreducibleFloor(skeleton, 0)` is EXACT and the floor/floor+100
  boundaries are precise. Hardcoding a skeleton (e.g. assuming "1\t0\ta.go\n") risks a mismatch → flaky boundary tests.
- ❌ Don't assert an ERROR in the at_floor_succeeds subtest. The floor check is strict `<` (S1's explicit choice — the
  contract says "the floor is the minimum valid value"). At TokenLimit==floor, `floor < floor` is FALSE → the check does
  NOT fire → StagedDiff proceeds to closedLoopGate (bodyBudget=0 → best-effort slivers) and returns `(string, nil)`. The
  at-floor subtest MUST assert `err == nil` (+ skeleton present). If you assert an error there, you have misunderstood the
  `<` semantics and the test will fail — do not "fix" it by changing the assertion to expect an error; the production
  behavior (nil err at floor) is correct.
- ❌ Don't change the floor check from `<` to `<=` to make an at-floor error "work". The strict `<` is S1's deliberate
  design (the floor is the minimum VALID value). This task TESTS that design, it does not change it. If at_floor_succeeds
  fails with an error, the production check is wrong (not the test) — but S1 is LANDED and verified, so it won't.
- ❌ Don't replace `EstimateTokens(skeleton)` with `len(skeleton)/4` in the arithmetic test's `want`. EstimateTokens is
  RUNE-based (utf8.RuneCountInString, ceil(/4)) — NOT byte-based. The multibyte test case ("中文.go") verifies this: by
  runes it estimates fewer tokens than by bytes. Using len()/4 would make the multibyte case assert the wrong `want`.
- ❌ Don't export `tokenBudgetMargin`. S1 kept it unexported (tokengate.go:48); only IrreducibleFloor is exported. The
  arithmetic test is in `package git` (internal) so it reads the unexported const directly in its `want` expression.
  Exporting it widens the public surface for zero consumers (the test is in-package).
- ❌ Don't add tests for TreeDiff and WorkingTreeDiff. The contract says "Calls one of the diff functions (StagedDiff or
  the most testable one)". StagedDiff is the most testable (simplest harness). The floor check is BYTE-IDENTICAL at all 3
  git.go sites (1057/1588/1769 — same IrreducibleFloor call + same error string), so StagedDiff coverage proves the
  pattern; grep guard 8 confirms all 3 sites are intact. Triplicating the harness adds no defect-class coverage.
- ❌ Don't forget the `"fmt"` import in difftokenlimit_test.go. It imports context/os/exec/strings/testing today (NO fmt).
  The below_floor_rejects subtest's `fmt.Sprintf("floor %d", floor)` assertion needs it. If you skip the floor-number
  assertion (substring-only), you can skip the import — but the floor-number check proves the error is ACTIONABLE (Issue 4's
  core value: tell the user exactly what to set), so prefer keeping it + adding the import.
- ❌ Don't assert the FULL error string with hardcoded floor numbers in below_floor_rejects (e.g. `err.Error() == "token_limit
  1023 is below the irreducible prompt floor 1024 ..."`). The floor depends on the actual skeleton (computed at runtime via
  StagedNumstatSkeleton), so hardcoding the number is fragile. Assert the SUBSTRING "below the irreducible prompt floor" +
  that the runtime `floor` value appears (`fmt.Sprintf("floor %d", floor)`) — both are robust to the skeleton's actual size.
- ❌ Don't conflate this with Issue 5 (doubled "stagecoach:" prefix) or Issue 6 ("(1 files)" grammar). Those are separate
  tasks (P1.M3.T2/T3). This task is Issue 4's TEST ONLY.

---

## Confidence Score: 9/10

This is a test-only task exercising ALREADY-LANDED, verified code (S1 is fully implemented: IrreducibleFloor @
tokengate.go:100; floor check @ git.go 1057/1588/1769, all grep-confirmed). Every test-design question is resolved against
the real code: the exact helper signature + formula, the exact error string (assert the substring + the runtime floor
value), the KEY TECHNIQUE (StagedNumstatSkeleton shares numstatSkeleton with StagedDiff — identical args at git.go:930-938
vs 2190-2198 — so the computed floor is exact and the boundaries are precise), the strict-< boundary semantics (at-floor
passes through to closedLoopGate best-effort → nil err; the at_floor_succeeds subtest asserts nil, not an error), the
constants (tokenBudgetMargin=1024, EstimateTokens=ceil(runes/4) rune-based), the E2E harness to clone (initRepo/writeFile/
stageFile/New — all in-package), the import need (difftokenlimit_test.go + "fmt"), the test placement (pure → tokengate_test.go;
E2E → difftokenlimit_test.go), and the run commands (incl. the contract's `-run TestTokenLimitFloor`). The verbatim test code
is provided. The -1 from 10/10 reflects the one thing the implementer must honor precisely: the at_floor_succeeds subtest
asserts `err == nil` (strict-< semantics) — an implementer who "intuitively" expects an error at the floor will mis-write
the assertion; the Anti-Patterns + the gotcha block make this explicit. The grep guards (esp. 4 + 7) catch both the
boundary mis-write and any scope creep into production files.
