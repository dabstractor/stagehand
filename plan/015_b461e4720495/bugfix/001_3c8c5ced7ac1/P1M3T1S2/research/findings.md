# P1.M3.T1.S2 Research Findings — Test for sub-floor token_limit rejection (Issue 4 / FR3j)

> Research for: adding the TEST that proves P1.M3.T1.S1's fix — the `IrreducibleFloor` helper + the
> floor check at the 3 git.go diff entry points. This is a TEST-ONLY task ("DOCS: none — test-only").

---

## §0. STATE OF THE WORLD (verified against the working tree)

- **S1 (P1.M3.T1.S1) is FULLY LANDED** in the working tree. Confirmed by grep + read:
  - `internal/git/tokengate.go:100` — `func IrreducibleFloor(skeleton string, promptReserve int) int`
    body: `return EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin` (godoc :91-99 cites FR3j/Issue 4).
  - `internal/git/git.go:1057` (StagedDiff), `:1588` (TreeDiff), `:1769` (WorkingTreeDiff) — the floor check
    `floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens); if opts.TokenLimit < floor { return "", fmt.Errorf(...) }`
    BEFORE `gatedBody := closedLoopGate(...)`, inside each `if opts.TokenLimit > 0 {` block.
  - `grep -c 'IrreducibleFloor(skeleton, opts.PromptReserveTokens)' internal/git/git.go` == **3** (all sites wired).
- **The exact landed error string** (git.go:1060, verbatim at all 3 sites):
  `token_limit %d is below the irreducible prompt floor %d (system prompt + numstat skeleton + framing); raise it to at least %d`
  (with opts.TokenLimit, floor, floor interpolated). **Substring to assert**: `below the irreducible prompt floor`.
- **CONCLUSION**: S2 tests LANDED code. No "assume S1" needed — it is concrete. S2 adds ONLY test functions.

---

## §1. THE CONSTANTS + THE HELPER (verified)

- `tokenBudgetMargin = 1024` (tokengate.go:48) — the framing buffer, a floor term.
- `minBodyTokens = 8` (tokengate.go:75) — the per-file body sliver (NOT a floor term — bodies are truncatable).
- `EstimateTokens(s) = ceil(utf8.RuneCountInString(s) / 4)` (tokens.go:25) — rune-based (chars/4), the SINGLE estimator.
- `IrreducibleFloor(skeleton, promptReserve) = EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin(1024)`.
  - ⇒ the floor is ALWAYS ≥ 1024 (even empty skeleton + 0 reserve). This is WHY every existing git-layer test
    (min TokenLimit = 2000 in difftokenlimit_test.go) stays ABOVE the floor → behavior-preserving.
- The floor does NOT include minBodyTokens (bodies are truncatable; the 3 floor terms are not).

## §2. THE BOUNDARY SEMANTICS (strict `<`) — critical for the at-floor sub-test

The floor check is `if opts.TokenLimit < floor` (STRICT less-than). Therefore:
- **TokenLimit = floor - 1** → `floor-1 < floor` TRUE → returns the error. (the "below" case)
- **TokenLimit = floor** (exactly) → `floor < floor` FALSE → does NOT error → proceeds to closedLoopGate.
  At floor, bodyBudget = floor − EstimateTokens(skeleton) − reserve − tokenBudgetMargin = 0 → budgetExhausted=true →
  bodies cut to minBodyTokens slivers → closedLoopGate returns a best-effort STRING (NOT an error — it returns `string`).
  StagedDiff returns `(b.String(), nil)` — `b` already holds the skeleton (written at git.go:942 BEFORE the token-limit
  branch). ⇒ **at-floor succeeds** (err == nil, out contains the skeleton). This is the contract's "(d)" case.
- **TokenLimit = floor + 100** → `floor+100 < floor` FALSE → no error → closedLoopGate (bodyBudget=100) → string → nil err.

## §3. THE KEY TECHNIQUE — compute the EXACT floor via StagedNumstatSkeleton

The contract wants floor-1 / floor / floor+100 boundaries. That requires knowing the floor EXACTLY. The skeleton is
computed INTERNALLY by StagedDiff (the caller doesn't pass it). But StagedDiff and StagedNumstatSkeleton share the SAME
renderer (`numstatSkeleton`) with IDENTICAL args:

- **StagedDiff** (git.go:930-938): `skeletonArgs = ["--cached", "-M", "--", defaultExcludes..., opts.Excludes...]` → `g.numstatSkeleton(ctx, skeletonArgs...)`.
- **StagedNumstatSkeleton** (git.go:2190-2198): `args = ["--cached", "-M", "--", defaultExcludes..., opts.Excludes...]` → `g.numstatSkeleton(ctx, args...)`.

⇒ `g.StagedNumstatSkeleton(ctx, opts)` returns the BYTE-IDENTICAL skeleton StagedDiff uses internally (same excludes).
So the test computes `skeleton, _ := g.StagedNumstatSkeleton(ctx, opts)` then `floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens)`
and the floor is EXACT — the boundary tests are precise, not approximate. (Verified: StagedNumstatSkeleton is on the Git
interface — the workdesc test calls it via `git.Git` from internal/generate; in-package it is directly callable.)

## §4. THE E2E TEST HARNESS (clone from difftokenlimit_test.go)

`internal/git/difftokenlimit_test.go` is the established StagedDiff-TokenLimit E2E harness. Pattern (verbatim from
TestStagedDiff_TokenLimitGt0_WaterFill):
```go
repo := t.TempDir()
initRepo(t, repo)
writeFile(t, repo, "a.go", "package main\n// a\n")
stageFile(t, repo, "a.go")
g := New(repo)
out, err := g.StagedDiff(context.Background(), StagedDiffOptions{TokenLimit: ..., PromptReserveTokens: 0})
```
- Helpers `initRepo` / `writeFile` / `stageFile` / `New` are all in-package (package git) — directly reusable.
- `difftokenlimit_test.go` imports: `context`, `os/exec`, `strings`, `testing` — NO `"fmt"`. The floor-number
  assertion (`fmt.Sprintf("floor %d", floor)`) needs `"fmt"` ADDED to the import block. (Or drop the floor-number
  check and assert only the substring — but the floor-number check proves the error is ACTIONABLE, worth the import.)

## §5. TEST PLACEMENT (the cleanest split)

- **`TestIrreducibleFloor_Arithmetic`** (PURE) → `internal/git/tokengate_test.go` — alongside the helper's siblings
  (TestApplyWaterFillGate, TestClosedLoopGate_*). tokengate_test.go is `package git`, imports `fmt`/`strings`/`testing`
  (all present). PURE: hardcoded skeleton strings + promptReserve, asserts `floor == EstimateTokens(skeleton) +
  promptReserve + tokenBudgetMargin`. Table-driven (mirrors tokengate_test.go's style). No git repo, no t.TempDir.
- **`TestTokenLimitFloor`** (E2E) → `internal/git/difftokenlimit_test.go` — alongside TestStagedDiff_TokenLimitGt0_*.
  Reuses initRepo/writeFile/stageFile/New. Uses StagedNumstatSkeleton to compute the exact floor (§3). Sub-tests:
  `below_floor_rejects` (floor-1 → error), `at_floor_succeeds` (floor → nil err + skeleton present),
  `above_floor_succeeds` (floor+100 → nil err + skeleton present). Needs `"fmt"` import added.

  WHY this split: the pure test pins the helper's ARITHMETIC (formula correctness, no git); the E2E test pins the
  WIRING (StagedDiff rejects/accepts at the right boundary using the REAL skeleton). Together they fully prove Issue 4's
  fix. The contract's run command `go test ./internal/git/ -v -run TestTokenLimitFloor` exercises the E2E test (and its
  subtests); `go test ./internal/git/ -v` exercises both.

- NO name collisions: `grep -rn 'TestTokenLimitFloor|TestIrreducibleFloor' internal/git/` → none (names are free).

## §6. WHY ONLY StagedDiff (not all 3 diff fns)

The contract: "Calls one of the diff functions (StagedDiff or the most testable one)". StagedDiff is the most testable
(simplest harness — no two-tree setup like TreeDiff, no working-tree baseline commit like WorkingTreeDiff). The floor
check is BYTE-IDENTICAL at all 3 sites (git.go:1057/1588/1769 — same `IrreducibleFloor(skeleton, opts.PromptReserveTokens)`
+ same error string), so StagedDiff coverage proves the pattern; a grep guard confirms all 3 sites have the check.
(Testing all 3 would triple the harness for no additional defect-class coverage — the check is the same code 3×.)

## §7. SCOPE FENCES (what NOT to touch)

- `internal/git/tokengate.go` (IrreducibleFloor) — S1 LANDED. READ-ONLY.
- `internal/git/git.go` (the 3 floor-check sites) — S1 LANDED. READ-ONLY.
- `docs/configuration.md` — S1's docs sentence (LANDED). READ-ONLY.
- Issue 5 (doubled prefix) / Issue 6 (auto-stage grammar) — separate tasks (P1.M3.T2/T3). NOT this task.
- closedLoopGate / applyWaterFillGate signatures + the bodyBudget≤0 path — UNCHANGED by S1; S2 does not touch them.
- tokenBudgetMargin stays UNEXPORTED — the in-package test reads it directly (same package). Do NOT export it.

## §8. VALIDATION COMMANDS (project-specific, verified)
- Build: `go build ./...` (test-only change; must still compile).
- Vet: `go vet ./internal/git/...`
- Fmt: `gofmt -l internal/git/tokengate_test.go internal/git/difftokenlimit_test.go` (must be empty).
- The NEW tests (the contract's run command + the pure test):
  - `go test ./internal/git/ -v -run TestTokenLimitFloor` (E2E + subtests)
  - `go test ./internal/git/ -v -run TestIrreducibleFloor_Arithmetic` (pure)
  - `go test ./internal/git/ -v -run 'TestTokenLimitFloor|TestIrreducibleFloor'` (both)
- Full regression: `go test ./internal/git/ -v` (all existing tests stay GREEN — S1 is behavior-preserving; min TokenLimit=2000 > ~1024 floor).
- `make test` (race) + `make lint` + `make coverage-gate` (the new tests ADD coverage to the floor-rejection branch S1 left uncovered).
