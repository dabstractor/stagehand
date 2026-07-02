# S2 Implementation Notes — downstream test assertions + user-facing surfaces (verify-and-confirm)

> Scope: P1.M1.T1.S2 — the ripple-effect updates after S1 removed the shipped `planner=high` reasoning
> default. Five sites: decompose/roles_test.go, default_action_test.go, root.go, pkg/stagehand,
> docs/cli.md. **STATUS: ~95% ALREADY REALIZED in the live codebase (2026-07-02).** Like S1, this is a
> VERIFY-AND-CONFIRM runbook with ONE genuine micro-edit (a stale section-header comment). Verified.

## 0. Baseline (confirmed)

- `go test ./...` is **GREEN** (every package `ok` / `no test files`; no failures).
- `grep -rn 'planner: high\|planner=high' internal/ pkg/ docs/` → **ZERO matches** (the shipped-default
  phrasing is entirely gone from the tree).
- `grep -rn "defaultRoleReasoning"` (S1's deliverable) → absent.

## 1. Per-site verification (4 of 5 ALREADY in target state)

| # | site | contract action | LIVE state | verdict |
|---|------|-----------------|------------|---------|
| a | `internal/decompose/roles_test.go` (rename + 2 assertions) | rename `TestResolveRoles_ReasoningShippedDefault` → `..._NoShippedReasoningDefault`; planner `"high"`→`""` (both tests) | FUNC already `TestResolveRoles_NoShippedReasoningDefault` (:542); planner asserts `""` (:551-552, :579-580); `ReasoningPerRoleSet` message="low" (:582). **DONE.** But the SECTION-HEADER COMMENT at :537 still reads `// TestResolveRoles_ReasoningShippedDefault` (stale — doesn't match the renamed func). | ⚠️ ONE micro-edit (the comment) |
| b | `internal/cmd/default_action_test.go:1440` (assertion inversion) | assert NO `(reasoning: …)` suffix on any role line | Line 1440 already `if strings.Contains(stderr, "(reasoning:") {` → error; comment :1439 already says "no (reasoning: …) suffix". **DONE.** | ✅ no edit |
| c | `internal/cmd/root.go:137` (--reasoning help) | 'default off, planner: high' → 'default off for every role' | Already `"…default off for every role)"` (:137). **DONE.** | ✅ no edit |
| d | `pkg/stagehand/stagehand.go:62,66` (RoleModel comments) | 'shipped default' → 'off by default for every role' | Already `"off by default for every role"` (:62) + `"off by default"` (:66). **DONE.** | ✅ no edit |
| e | `docs/cli.md:43` (--reasoning default column) | `'"" (off; planner: high)'` → `'"" (off)'` | Already `"" (off)` (:43). **DONE.** | ✅ no edit |

## 2. The ONE genuine edit — the stale section-header comment

`internal/decompose/roles_test.go:537` — the only remaining `ReasoningShippedDefault` reference in the
tree (`grep -rn "ReasoningShippedDefault" --include="*.go" . | grep -v plan/` → exactly this one line):

```
// ---------------------------------------------------------------------------
// TestResolveRoles_ReasoningShippedDefault        ← STALE; the func below is ..._NoShippedReasoningDefault
// ---------------------------------------------------------------------------

func TestResolveRoles_NoShippedReasoningDefault(t *testing.T) {     // ← already renamed (the rename is done)
```

Fix: change the comment line to `// TestResolveRoles_NoShippedReasoningDefault` so the doc banner
matches the func it documents. This completes the contract's "RENAME" action (the func is already
renamed; only its header comment was missed). 1-line edit; no behavioral effect; improves consistency.

## 3. The verification grep — IMPRECISE; frame it correctly

The contract's verification `grep -rn 'planner.*high' internal/ pkg/ docs/` is a ROUGH sanity check, NOT
a "zero matches" gate. It will STILL legitimately match (do NOT "fix" these — they are correct):

- `internal/config/roles_test.go:132,136,188` — per-role **override** tests (S1's territory; `planner:
  {Reasoning: "high"}` as an explicit per-role cfg value, and "per-role off beats global high"). Valid.
- `internal/ui/verbose_test.go:13,20,43,53` — `reasoningSuffix` **FORMAT** tests (fixture sets
  `Reasoning:"high"` explicitly to test the formatter). critical_findings.md **Finding 3: must NOT change.**
- `docs/configuration.md:153` — a per-role env-var **EXAMPLE** (`STAGEHAND_PLANNER_REASONING=high`). Valid.
- False positives on the word "higher" (e.g. `internal/config/file_test.go:602,608,612` — "higher layer"
  in model-merge comments; `planner.*high` matches "planner"… "high"er).

The AUTHORITATIVE check is the shipped-default phrasing: `grep -rn 'planner: high\|planner=high'`
→ **ZERO** (confirmed). Use that, plus `grep "ReasoningShippedDefault"` → ZERO (after the comment fix),
plus the green `go test ./...`, as the real gates.

## 4. The verbose_test.go guard (Finding 3 — DO NOT TOUCH)

`internal/ui/verbose_test.go` sets `Reasoning:"high"` on the planner `RoleLine` (lines 13/43/53) and
asserts `DEBUG: planner  p in pi (reasoning: high)` (line 20). This tests the `reasoningSuffix`
FORMATTER (does it render the suffix when reasoning is non-empty?), NOT the shipped default. It must be
left exactly as-is — changing it would remove the only coverage that the suffix renders at all.

## 5. Scope discipline — what S2 does NOT do

- NOT `internal/config/*` (S1's territory — already verified complete).
- NOT `ui/verbose_test.go` (Finding 3 — formatter test).
- NOT `docs/configuration.md` (P1.M1.T2/T3 territory; the :153 env-var example is legitimate).
- NOT README.md / other docs (P1.M1.T3.S1 verifies those).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 6. Sources

- `plan/004_136878664597/docs/system_context.md` §3 (Change A downstream sites — S2's file list).
- `plan/004_136878664597/docs/critical_findings.md` Finding 3 (verbose_test.go guard) + Finding 2 (tests are the exhaustive oracle).
- `plan/004_136878664597/P1M1T1S1/PRP.md` (S1 — the verify-and-confirm template + the "already realized" honesty).
- The 5 live files (verified current state above).
