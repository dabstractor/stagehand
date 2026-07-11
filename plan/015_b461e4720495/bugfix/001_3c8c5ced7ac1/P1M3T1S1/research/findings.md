# Codebase Findings — P1.M3.T1.S1 (IrreducibleFloor + git.go floor check, Issue 4 / FR3j)

## 1. The bug (architecture/minor_fixes.md §Issue 4 — verified)

`internal/git/tokengate.go`:
- `applyWaterFillGate` (line 135) computes `bodyBudget := tokenLimit - skeletonTokens - promptReserve - tokenBudgetMargin`
  (line 137). When `bodyBudget <= 0` (line 138: `budgetExhausted := bodyBudget <= 0`), each file body is cut to a
  `minBodyTokens` (8) sliver + sentinel — a BEST-EFFORT fit.
- `closedLoopGate` (line 195) loops ≤ `maxClosedLoopPasses` (4) re-measuring the assembled prompt; on exit it
  `return bestDiff // best effort after maxClosedLoopPasses` (line 224) — so below the irreducible floor the FR3j
  invariant ("payload NEVER exceeds token_limit … ALWAYS") is **silently violated**.

The irreducible floor = the assembled prompt's non-body minimum: `EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin`
(skeleton + promptReserve + margin are irreducible; only the diff BODIES are truncatable, down to minBodyTokens slivers).
When `tokenLimit < floor`, even sliver-bodies push the assembled prompt over → no closed-loop pass can satisfy it.

## 2. The constants + signatures (verified verbatim)

```go
// tokengate.go:48 (UNEXPORTED const)
const tokenBudgetMargin = 1024
// tokengate.go:75
const minBodyTokens = 8
// tokengate.go:83
const maxClosedLoopPasses = 4
// tokengate.go:89
const closedLoopSlack = 64

// tokengate.go:135
func applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int) string
// tokengate.go:195 — returns bestDiff best-effort (line 224); THIS is the silent violation
func closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int, measure func(string) int) string

// tokens.go:25 (in-package, UNEXPORTED usage but the func itself is exported)
func EstimateTokens(s string) int
```

`EstimateTokens` is in the SAME package (`internal/git`) → the new `IrreducibleFloor` (in tokengate.go, package git)
references it directly with no import. The git.go call sites (also package git) call `IrreducibleFloor(...)` directly.

## 3. The fix — caller-level rejection (NOT a closedLoopGate signature change)

Per the contract + minor_fixes.md "Preferred Fix: Option (a)": add an exported helper in tokengate.go and a floor
check at each of the THREE git.go call sites, BEFORE the `closedLoopGate(...)` call. closedLoopGate's signature is
UNCHANGED (the alternative — making closedLoopGate return (string, error) — is more invasive and explicitly rejected).

```go
// ADD to tokengate.go (the helper):
func IrreducibleFloor(skeleton string, promptReserve int) int {
    return EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin
}

// ADD at each of the 3 git.go call sites, inside `if opts.TokenLimit > 0 {`, before `gatedBody := closedLoopGate(...)`:
floor := IrreducibleFloor(skeleton, opts.PromptReserveTokens)
if opts.TokenLimit < floor {
    return "", fmt.Errorf("token_limit %d is below the irreducible prompt floor %d (system prompt + numstat skeleton + framing); raise it to at least %d", opts.TokenLimit, floor, floor)
}
```

**Do NOT export `TokenBudgetMargin`.** The contract says "Export `TokenBudgetMargin` as a constant alias IF NEEDED".
It is NOT needed: IrreducibleFloor (in-package) references the unexported `tokenBudgetMargin` directly; the git.go
call sites (in-package) call IrreducibleFloor; the S2 test (internal/git/tokengate_test.go, package git) can reference
either. No EXTERNAL package needs it. Keep the change minimal — leave `tokenBudgetMargin` unexported.

## 4. The three git.go call sites (verified — ALL TEXTUALLY IDENTICAL)

```
StagedDiff       ~1057  (func at :911)
TreeDiff         ~1582  (func at :1448)
WorkingTreeDiff  ~1757  (func at :1622)
```
Each has the IDENTICAL block (anchor: `grep -n 'gatedBody := closedLoopGate' internal/git/git.go` → 3 hits):
```go
	if opts.TokenLimit > 0 {
		// FR3d/FR3i/FR3j: closedLoopGate does the first-cut water-fill (applyWaterFillGate) AND, when
		// opts.MeasureAssembled is non-nil, re-measures the ASSEMBLED prompt and re-trims if over
		// tokenLimit (FR3j hard guarantee). nil MeasureAssembled ⇒ first-cut only (behavior unchanged).
		gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)
		b.WriteString(gatedBody)
		return b.String(), nil
	}
```
**The block is byte-identical at all 3 sites** → the `gatedBody := closedLoopGate(...)` line is NOT a unique anchor.
The implementer MUST disambiguate by the ENCLOSING FUNCTION (StagedDiff / TreeDiff / WorkingTreeDiff) — apply the
insertion once per function. At each site, `skeleton` (string) and `opts.PromptReserveTokens` (int) are in scope
(verified: `skeleton` is built earlier in each function; `opts` is the function param). The floor check returns
through the existing `(string, error)` signature — `fmt.Errorf` is already imported in git.go (used by the surrounding
error messages).

## 5. Behavior-preservation proof — NO existing test breaks

The floor for typical git-layer tests = `EstimateTokens(skeleton) + 0 (PromptReserveTokens) + 1024` ≈ 1024–1100
(small numstat skeleton). Every existing git-layer TokenLimit value (internal/git/difftokenlimit_test.go, verified):
`0` (unset ⇒ legacy caps, the `> 0` branch is skipped entirely), `2000` (const tokenLimit @ :420/:446/:474),
`3000` (:211/:251/:507), `4000` (:137/:329/:357/:387), `100000` (:185). **The minimum non-zero value is 2000 > ~1100 floor.**
So every existing test's TokenLimit ≥ floor → the new `if opts.TokenLimit < floor` is FALSE → falls through to
closedLoopGate byte-identically. No existing git-layer test sets a sub-floor TokenLimit, so none break.

The tokengate_test.go tests call `applyWaterFillGate` DIRECTLY (e.g. :239 `applyWaterFillGate(nil, section, "skeleton", 100, 0)`
— tokenLimit=100 < floor, exercises the degenerate path). These are UNAFFECTED: the floor check is at the git.go CALLER,
NOT inside applyWaterFillGate/closedLoopGate. The degenerate path in applyWaterFillGate stays (it's still reachable by
direct callers and is the best-effort fallback). So tokengate_test.go stays GREEN unchanged.

**Bottom line: the fix is behavior-preserving for ALL existing tests.** It only REJECTS genuinely sub-floor TokenLimit
values, which no test currently exercises (the S2 test will add that coverage).

## 6. The `<` boundary (follow the contract)

The contract specifies `if opts.TokenLimit < floor` (strict). At exactly `tokenLimit == floor`, `bodyBudget == 0` →
`budgetExhausted=true` → bodies cut to minBodyTokens slivers; the assembled prompt = floor + slivers > floor, so the
closed loop still can't satisfy it and returns best-effort. I.e. the `== floor` (and just-above-floor) case can still
silently violate. The contract's `<` catches the clearly-impossible cases (tokenLimit < floor) and leaves the boundary
to best-effort. FOLLOW THE CONTRACT: use `<`. (A stricter `<=` would reject tokenLimit==floor too, but that is NOT what
the contract specifies — do not over-engineer.)

## 7. Docs (Mode A) — docs/configuration.md:167 (verified)

The `token_limit` bullet ends: "…re-measures it, and re-trims until it fits — a closed-loop guarantee (§9.1 FR3j) that
the payload never exceeds `token_limit`." APPEND (per the contract's DOCS clause): a sentence noting that sub-floor
limits are rejected. Anchor: the string "a closed-loop guarantee (§9.1 FR3j) that the payload never exceeds `token_limit`."
is unique in the file.

## 8. Scope — S1 vs S2

- S1 (THIS item): IrreducibleFloor helper (tokengate.go) + 3 floor checks (git.go) + docs (configuration.md). NO test.
- S2 (P1.M3.T1.S2): the test (internal/git, consumes IrreducibleFloor + the rejection). NOT this item.
- NOT in scope: changing closedLoopGate's signature; exporting tokenBudgetMargin; touching applyWaterFillGate's degenerate
  path; Issues 5/6 (separate); the generation-layer MeasureAssembled wiring (pkg/stagecoach — already exists).

## 9. Validation (verified against Makefile)

- `go build ./...` + cross-build; `go vet ./internal/git/...`; `gofmt -l internal/git/tokengate.go internal/git/git.go`.
- `go test ./internal/git/ -v` (full git package — tokengate_test.go + difftokenlimit_test.go + stagediff_test.go MUST stay green).
- `make test` (race) ; `make lint` ; `make coverage-gate` (internal/git IS gated ≥85%).
- grep guards (see PRP Validation Level 4).
