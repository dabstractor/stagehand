---
name: "P1.M1.T1.S1 — Remove defaultRoleReasoning + ResolveRoleModel fallback + config.go doc comments + config-package tests"
description: |
  ⚠️ STATUS: ALREADY REALIZED IN THE CODEBASE. The contract describes a starting state (a
  `defaultRoleReasoning` map + `ResolveRoleModel` shipped fallback + `planner=high` test assertions)
  that DOES NOT EXIST in the live code as of 2026-07-02. The entire S1 change is already complete:
  `defaultRoleReasoning` is absent repo-wide, `ResolveRoleModel` already falls back to `cfg.Reasoning`
  with "no shipped per-role default" comments, the 3 `config.go` doc comments are already in the
  "off for every role; opt-in" wording, all 6 `roles_test.go` assertions are already flipped/renamed,
  and `go test ./internal/config/` is GREEN. This PRP is therefore a **VERIFY-AND-CONFIRM** runbook —
  confirm each deliverable is in place and the gates pass; DO NOT invent edits for a map that no
  longer exists. If all checks pass (they will), S1 is confirmed complete with zero code churn.
---

## Goal

**Feature Goal**: Ensure the FR-R6 reasoning default is `off` for EVERY role (including the planner) at
the config-resolution layer — i.e., `ResolveRoleModel` returns `reasoning=""` for all roles when nothing
is set, with NO shipped per-role fallback — and that the config package's code, doc comments, and tests
all consistently reflect that. **In the live codebase this state is already achieved; this subtask's job
is to VERIFY it is complete and consistent, not to re-perform it.**

**Deliverable**: A config package (`internal/config`) in which (a) no `defaultRoleReasoning` map exists,
(b) `ResolveRoleModel`'s reasoning fallback is solely `cfg.Reasoning` (the global), (c) the 3 `config.go`
doc comments say "off for every role; opt-in" (no "planner=high"/"shipped fallback"), (d) `roles_test.go`
asserts every role returns `""` reasoning when nothing is set, and (e) `go test ./internal/config/` is
green. **All five are already true — confirm and stop.**

**Success Definition**: The verification gates below pass (repo-wide grep absence + green config tests +
the spot-checks); no code edits are required. If any check surprisingly fails, apply the MINIMAL targeted
fix described in "If a check fails" — do not churn correct code.

## User Persona

**Target User**: The Stagecoach contributor / reviewer confirming the plan/004 "reasoning opt-in
everywhere" changeset is correctly landed at the config layer before downstream subtasks (S2: decompose/
cmd/doc surfaces) and dependent work proceed.

**Use Case**: A grep + test pass that proves no shipped `planner=high` default survives anywhere in
`internal/config`, so reasoning is genuinely opt-in.

**Pain Points Addressed**: Prevents a wasted/erroneous edit pass that would hunt for a `defaultRoleReasoning`
map that doesn't exist, or "correct" doc comments that are already correct — and gives the orchestrator
a definitive "S1 is complete" signal.

## Why

- **The change is the entire point of plan/004 Change A** (system_context.md §3): flip the shipped
  reasoning default from `planner=high` to `off` for all roles, making reasoning opt-in everywhere
  (FR-R6). Reasoning adds latency/cost and is rarely right for commit-message work.
- **It is already implemented.** A repo-wide grep for `defaultRoleReasoning` returns zero matches;
  `roles.go`, the 3 `config.go` doc comments, and all 6 `roles_test.go` assertions are in the post-change
  state; `go test ./internal/config/` is green. Re-editing would be churn with no behavioral effect.
- **Verification is cheap and authoritative.** The grep + the green test suite are exhaustive proof the
  config layer has no shipped reasoning default. critical_findings.md Finding 2 notes "the test failures
  are exhaustive — no hand-audit needed"; the absence of failures is equally exhaustive.

## What

A verification-only pass over `internal/config` (S1's scope is the config package ONLY — the contract's
point 4: "No other package is affected yet (decompose/cmd tests are S2)"). No edits unless a check fails.

### Success Criteria

- [ ] `grep -rn defaultRoleReasoning --include="*.go" .` (excl. plan/) returns ZERO matches.
- [ ] `ResolveRoleModel`'s reasoning fallback is `reasoning = cfg.Reasoning` (the global), with an
      inline comment stating "no shipped per-role default"; NO `defaultRoleReasoning[role]` line.
- [ ] `config.go` doc comments for `RoleConfig.Reasoning`, `Config.Reasoning`, and `Defaults() Reasoning`
      say "off"/"opt-in" — none cite "planner=high" or "shipped fallback".
- [ ] `roles_test.go` has `TestResolveRoleModel_NoShippedReasoningDefault` (renamed from
      `..._PlannerShippedDefault`) asserting ALL roles return `""`; the planner-specific `"high"`
      assertions are gone.
- [ ] `go test ./internal/config/` is GREEN.
- [ ] (If ALL of the above hold) — S1 is CONFIRMED COMPLETE; make no code edits.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes — and the key context is that the work is already done.** This PRP quotes the live
post-change state of `roles.go`, the 3 `config.go` comments, and the 6 test assertions, so the implementer
can recognize "this is already in the target state" rather than searching for a `defaultRoleReasoning`
map to delete. The architecture docs (`system_context.md §3 Change A`, `critical_findings.md Finding 1`)
describe the intended delta; the live code already reflects the "After" state.

### Documentation & References

```yaml
# MUST READ — the intended delta (to recognize it's already applied)
- docfile: plan/004_136878664597/docs/system_context.md
  why: "§3 'Change A' specifies the exact delta: remove defaultRoleReasoning; ResolveRoleModel reasoning fallback → cfg.Reasoning only; flip planner from high→off. The §3 table lists S1's files (roles.go, config.go, roles_test.go) vs S2's (decompose/cmd/docs)."
  critical: "§3 lines 85-88 show the intended resolution layers (per-role→global, with the shipped-default layer REMOVED for reasoning). The live roles.go already matches this."

- docfile: plan/004_136878664597/docs/critical_findings.md
  why: "Finding 1 states the behavioral change is 'a single map-entry removal' and quotes the exact old fallback block (the second `if reasoning == \"\"` → defaultRoleReasoning[role]) that must be gone. Finding 2 notes the tests are the bulk of the work and that `go test ./...` is the exhaustive oracle."

# The S1-scope files (VERIFY their current state — do not edit unless a check fails)
- file: internal/config/roles.go
  why: "VERIFY. ResolveRoleModel must have NO defaultRoleReasoning reference; its reasoning fallback must be `reasoning = cfg.Reasoning` with a 'no shipped per-role default' comment; the function doc comment must state 'Reasoning has NO shipped per-role default'. (All already true in the live file.)"
  pattern: "The live file IS the target. If it somehow still had `var defaultRoleReasoning = map[string]string{\"planner\":\"high\"}` or a second `if reasoning == \"\" { reasoning = defaultRoleReasoning[role] }`, remove them — but they are not present."

- file: internal/config/config.go
  why: "VERIFY the 3 doc comments. RoleConfig.Reasoning (~line 37), Config.Reasoning (~line 65), Defaults() Reasoning (~line 126) must say 'off by default'/'opt-in' — NOT 'planner=high'/'shipped fallback'. (All already in target wording.)"

- file: internal/config/roles_test.go
  why: "VERIFY. Must have TestResolveRoleModel_NoShippedReasoningDefault (NOT ..._PlannerShippedDefault) asserting all roles \"\"; TestResolveRoleModel_FullOverride/_BothEmptyManifestSentinel/_AllCanonicalRoles planner reasoning → \"\"; _ReasoningGlobalFallback planner inherits global. (All already flipped/renamed in the live file.)"

- docfile: plan/004_136878664597/P1M1T1S1/research/s1_already_realized.md
  why: "The evidence record: repo-wide grep absence, verbatim current state of roles.go/config.go/roles_test.go, and the green `go test ./internal/config/` result that prove S1 is already complete."

# OUT OF SCOPE (S2 = P1.M1.T1.S2 — do NOT touch in S1)
- file: internal/decompose/roles_test.go        # S2: rename TestResolveRoles_ReasoningShippedDefault; planner high→""
- file: internal/cmd/default_action_test.go     # S2: TestProgressLabel_DecomposeVerboseRoles reasoning-suffix assertion
- file: internal/cmd/root.go                     # S2: --reasoning help string
- file: pkg/stagecoach/stagecoach.go              # S2: RoleModel comment
- file: docs/cli.md                              # S2: --reasoning default column
```

### Current Codebase Tree (S1 scope = internal/config only)

```bash
stagecoach/
└── internal/config/
    ├── roles.go          # VERIFY (already post-change)
    ├── roles_test.go     # VERIFY (already flipped/renamed)
    ├── config.go         # VERIFY 3 doc comments (already target wording)
    └── (no defaults.go defaultRoleReasoning — map is absent)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (NO CHANGES — the tree is already in the desired state)
    # internal/config/{roles,config,roles_test}.go already reflect "off for every role; opt-in"
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/roles.go` | VERIFY (no edit) | Confirm no `defaultRoleReasoning`; fallback is `cfg.Reasoning`. |
| `internal/config/config.go` | VERIFY (no edit) | Confirm 3 doc comments say "off/opt-in". |
| `internal/config/roles_test.go` | VERIFY (no edit) | Confirm assertions flipped + `..._NoShippedReasoningDefault` rename. |

**Explicitly NOT touched in S1**: `internal/decompose/*`, `internal/cmd/*`, `pkg/stagecoach/*`, `docs/*`
(all S2 / P1.M1.T1.S2), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL: the contract's described STARTING STATE (a defaultRoleReasoning map + a second
// `if reasoning == "" { reasoning = defaultRoleReasoning[role] }` block + planner="high" test
// assertions) DOES NOT EXIST in the live code. grep confirms absence; the green test suite confirms
// consistency. Do NOT hunt for a map to delete — there is nothing to delete.

// CRITICAL: do NOT invent edits. If a check passes, leave the code alone. "Confirming complete" is the
// deliverable. Re-editing already-correct doc comments or re-flipping already-correct assertions is
// pure churn that risks introducing a regression.

// GOTCHA (scope): S1 is the CONFIG PACKAGE ONLY (contract point 4). decompose/cmd/pkg/docs surfaces
// are S2 (P1.M1.T1.S2). Do not touch them here even if they look stale (a grep suggests they are also
// clean, but that is S2's verification to own).

// GOTCHA: `go test ./internal/config/` is the exhaustive oracle (critical_findings.md Finding 2). If it
// is green AND the defaultRoleReasoning grep is empty, S1 is provably complete — no further audit needed.
```

## Implementation Blueprint

### Data models and structure

No data-model change — `RoleConfig.Reasoning` / `Config.Reasoning` (both `string`, zero value `""` = off)
already exist and are correct. The only "model" relevant to S1 was the removed `defaultRoleReasoning`
map, which is already gone.

### Implementation Tasks (verification pass — ordered)

```yaml
Task 1: VERIFY defaultRoleReasoning is absent repo-wide
  - RUN: grep -rn "defaultRoleReasoning" --include="*.go" . | grep -v "/plan/" || echo "ABSENT"
  - EXPECT: "ABSENT" (zero matches). If any match appears, that is the ONE case requiring an edit —
    remove the map + its usage (see "If a check fails"). In the live code it is absent.

Task 2: VERIFY ResolveRoleModel (internal/config/roles.go)
  - RUN: grep -n "defaultRoleReasoning\|reasoning = cfg.Reasoning\|no shipped per-role default" internal/config/roles.go
  - EXPECT: a line `reasoning = cfg.Reasoning // FR-R6: no shipped per-role default ...` and ZERO
    `defaultRoleReasoning` references. Confirm the function doc comment contains
    "Reasoning has NO shipped per-role default".

Task 3: VERIFY the 3 config.go doc comments
  - RUN: grep -n "Reasoning" internal/config/config.go
  - EXPECT (no "planner=high" / "shipped fallback"):
      RoleConfig.Reasoning (~37): "...off by default"
      Config.Reasoning (~65):     "...off by default; config init writes \"off\""
      Defaults() Reasoning (~126): "FR-R6: off for every role by default..."

Task 4: VERIFY roles_test.go assertions
  - RUN: grep -n "NoShippedReasoningDefault\|PlannerShippedDefault\|want \"\" (off" internal/config/roles_test.go
  - EXPECT: `TestResolveRoleModel_NoShippedReasoningDefault` EXISTS; `..._PlannerShippedDefault` does NOT;
    planner reasoning assertions want `""` (off). Confirm TestResolveRoleModel_ReasoningGlobalFallback's
    planner inherits the global (comment: "no shipped planner default anymore").

Task 5: GATE — run the config-package tests
  - RUN: go test ./internal/config/
  - EXPECT: `ok github.com/dustin/stagecoach/internal/config` (green). This is the exhaustive oracle.

Task 6: DECIDE
  - IF Tasks 1–5 all pass: S1 is CONFIRMED COMPLETE. Make NO code edits. Report done.
  - IF any task fails: apply the minimal fix in "If a check fails" and re-run Task 5 until green.
```

### Implementation Patterns & Key Details

```go
// === The TARGET state of ResolveRoleModel's reasoning fallback (already realized in roles.go) ===
	if reasoning == "" {
		reasoning = cfg.Reasoning // FR-R6: no shipped per-role default — off (== "") is the only fallback
	}
	return provider, model, reasoning
// (The second `if reasoning == "" { reasoning = defaultRoleReasoning[role] }` block is GONE.)
```

```go
// === The TARGET config-package test (already realized: TestResolveRoleModel_NoShippedReasoningDefault) ===
// FR-R6: NO role has a non-off shipped reasoning default — not even the planner. With nothing set,
// every role resolves reasoning to "" (off).
//   for role in roleNames { ... want "" ... }
```

### Integration Points

```yaml
CONFIG LAYER (internal/config) — S1 scope:
  - ResolveRoleModel: reasoning fallback = cfg.Reasoning only (NO shipped default)   [ALREADY DONE]
  - config.go doc comments (3): "off/opt-in" wording                                  [ALREADY DONE]
  - roles_test.go: NoShippedReasoningDefault + flipped planner assertions             [ALREADY DONE]

NO-TOUCH (S2 = P1.M1.T1.S2 — explicitly out of S1 scope):
  - internal/decompose/roles_test.go, internal/cmd/default_action_test.go, internal/cmd/root.go,
    pkg/stagecoach/stagecoach.go, docs/cli.md

GATE: go test ./internal/config/  →  GREEN (the exhaustive oracle)

DOWNSTREAM: S2 (P1.M1.T1.S2) verifies/updates the decompose/cmd/pkg/docs surfaces; P1.M1.T2 (config init
emits uncommented reasoning = "off") and P1.M1.T3 (changeset-level docs) are sibling subtasks.
```

## Validation Loop

### Level 1: Verification Gates (S1's "implementation")

```bash
cd /home/dustin/projects/stagecoach

# (1) defaultRoleReasoning absent repo-wide
grep -rn "defaultRoleReasoning" --include="*.go" . | grep -v "/plan/" && echo "FAIL: map still present" || echo "PASS: absent"

# (2) ResolveRoleModel fallback is cfg.Reasoning; no shipped-default line
grep -n "reasoning = cfg.Reasoning" internal/config/roles.go && \
  ! grep -q "defaultRoleReasoning\[role\]" internal/config/roles.go && echo "PASS: roles.go"

# (3) config.go doc comments have no "planner=high"/"shipped fallback"
! grep -q "planner=high\|shipped fallback" internal/config/config.go && echo "PASS: config.go comments"

# (4) roles_test.go: renamed test present; old name absent
grep -q "TestResolveRoleModel_NoShippedReasoningDefault" internal/config/roles_test.go && \
  ! grep -q "TestResolveRoleModel_PlannerShippedDefault" internal/config/roles_test.go && echo "PASS: roles_test.go"
```

### Level 2: The Exhaustive Oracle (config-package tests)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/config/            # Expected: ok (green)
go test -race ./internal/config/ -v   # verbose, confirm the reasoning tests run + pass

# Expected: all green. This is the authoritative proof the config layer has no shipped reasoning default
# and that roles_test.go is consistent with the production code.
```

### Level 3: Whole-Repository Sanity (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go build ./...           # Expected: exit 0
go vet ./...             # Expected: exit 0 (config package clean)
go test -race ./...      # Expected: all packages green (proves no stale cross-package defaultRoleReasoning ref)
```

### Level 4: Scope-Boundary Check (confirm S1 didn't touch S2 files)

```bash
cd /home/dustin/projects/stagecoach

# S1 must have made NO edits — git diff should be empty (or only the PRP/research under plan/, which is untracked/ignored)
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: empty (no production/doc changes — S1 is verify-only and the change was already landed).
```

## If a check fails (the ONLY case requiring an edit)

If (and only if) a verification gate surprisingly fails, apply the **minimal** targeted fix — do not
churn surrounding correct code:

- **`defaultRoleReasoning` present in `roles.go`**: delete the `var defaultRoleReasoning = map[string]string{...}`
  block AND the `if reasoning == "" { reasoning = defaultRoleReasoning[role] }` fallback; keep
  `reasoning = cfg.Reasoning` as the sole fallback; update the function doc comment to "no shipped
  per-role default". (Not expected — it is absent.)
- **A `config.go` doc comment still says "planner=high"/"shipped fallback"**: rewrite to
  "off for every role; opt-in per role (FR-R6)". (Not expected.)
- **`roles_test.go` still has `..._PlannerShippedDefault` or a planner `"high"` assertion**: rename to
  `..._NoShippedReasoningDefault` and flip the planner want to `""` (loop all canonical roles asserting
  `""`). (Not expected.)
- After any fix: re-run `go test ./internal/config/` until green.

## Final Validation Checklist

### Technical Validation

- [ ] `grep defaultRoleReasoning` repo-wide (excl. plan/) → ABSENT.
- [ ] `go test ./internal/config/` → GREEN.
- [ ] `go build ./...`, `go vet ./...`, `go test -race ./...` → all green.

### Feature Validation

- [ ] `ResolveRoleModel` reasoning fallback is `cfg.Reasoning` only (no shipped per-role default).
- [ ] `config.go` 3 doc comments say "off/opt-in" (no "planner=high").
- [ ] `roles_test.go` has `..._NoShippedReasoningDefault` (all roles `""`); no `..._PlannerShippedDefault`.

### Scope Discipline Validation

- [ ] S1 made **NO code edits** (verify-and-confirm; the change was already landed).
- [ ] `git diff --stat -- internal/ pkg/ cmd/ docs/` is empty.
- [ ] Did NOT touch S2 files (decompose/cmd/pkg/docs) — those are P1.M1.T1.S2.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (other than this PRP + research note).

### Documentation & Deployment

- [ ] Internal Go doc comments (roles.go + config.go) are in the target "off/opt-in" wording (already true).
- [ ] No external doc file is touched in S1 (Mode A doc rides with the work; already correct; user-facing
      docs are S2/S3's concern).

---

## Anti-Patterns to Avoid

- ❌ Don't hunt for a `defaultRoleReasoning` map to delete — it does not exist (repo-wide grep confirms).
  Searching for it wastes time and signals the contract's starting-state premise is stale.
- ❌ Don't "fix" already-correct doc comments or re-flip already-correct test assertions. If a check
  passes, leave it alone. Confirming-complete is the deliverable; churning correct code risks regressions.
- ❌ Don't expand scope into S2 (decompose/roles_test.go, default_action_test.go, root.go, pkg/stagecoach,
  docs/cli.md). S1 is the config package ONLY (contract point 4).
- ❌ Don't treat the green `go test ./internal/config/` as insufficient. Per critical_findings.md Finding 2,
  the test suite is the exhaustive oracle — green + grep-absent = provably complete.
- ❌ Don't re-add the map "to remove it again" or fabricate a before/after to justify the subtask. The
  honest outcome is "already complete; verified."
- ❌ Don't touch `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**10/10** that the verification passes with zero edits (i.e., S1 is already complete).

Rationale: Three independent pieces of evidence confirm the change is fully realized: (1) a repo-wide
grep for `defaultRoleReasoning` returns ZERO matches (the map + its usage are gone); (2) the verbatim
live state of `roles.go` (fallback `reasoning = cfg.Reasoning` + "no shipped per-role default" comments),
the 3 `config.go` doc comments ("off/opt-in"), and all 6 `roles_test.go` assertions (including the
`..._NoShippedReasoningDefault` rename) are each in the post-change target state; (3) `go test ./internal/config/`
is GREEN, which critical_findings.md identifies as the exhaustive oracle. The contract's described starting
state (map + fallback + planner=high assertions) simply does not exist in the live code. The PRP therefore
redirects the implementer from "perform edits" to "verify and confirm," with a precise fallback for the
unlikely case a gate fails. This is the correct, non-fabricating response — the confidence reflects that
the verification (not a risky edit) is the action, and the evidence for completion is already in hand.
