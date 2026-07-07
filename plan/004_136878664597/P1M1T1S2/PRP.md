---
name: "P1.M1.T1.S2 — Update downstream test assertions + user-facing surfaces (decompose roles_test, default_action_test, root.go flag help, pkg/stagecoach comment, docs/cli.md)"
description: |
  ⚠️ STATUS: ~95% ALREADY REALIZED IN THE CODEBASE. The contract describes a starting state (a
  `TestResolveRoles_ReasoningShippedDefault` test name, planner `"high"` assertions, a
  `(reasoning: high)` suffix assertion, a 'planner: high' flag-help string, 'shipped default' RoleModel
  comments, and an '(off; planner: high)' docs/cli.md cell) that DOES NOT EXIST in the live code as of
  2026-07-02. Four of the five sites are already in the target state and the full repo is GREEN; the
  shipped-default phrasing `planner: high`/`planner=high` is ZERO across the tree. The ONE genuine
  remaining edit is a stale SECTION-HEADER COMMENT at `internal/decompose/roles_test.go:537` that still
  reads `// TestResolveRoles_ReasoningShippedDefault` (the func below it was already renamed to
  `..._NoShippedReasoningDefault`). This PRP is a VERIFY-AND-CONFIRM runbook: confirm each of the five
  sites is in the target state, apply the single comment fix, and pass the gates. Do NOT invent edits for
  sites that are already correct. CRITICAL: `ui/verbose_test.go` MUST NOT be changed (Finding 3 — it tests
  the reasoningSuffix FORMATTER, not the default).
---

## Goal

**Feature Goal**: Ensure every downstream test assertion and user-facing surface consistently reflects
the S1 behavioral change (shipped reasoning default is now `off` for EVERY role, including the planner) —
so there is no stale `planner=high` shipped-default reference anywhere in the tree, the `--reasoning`
flag help / public `RoleModel` doc comment / `docs/cli.md` all say "off for every role", and
`go test ./...` is green across all packages. **In the live codebase this state is almost entirely
achieved; this subtask's job is to VERIFY it is complete and apply ONE missed comment fix.**

**Deliverable** (verify-and-confirm + one micro-edit):
1. `internal/decompose/roles_test.go`: the test is ALREADY renamed to `TestResolveRoles_NoShippedReasoningDefault`
   with planner assertions already `""`; FIX the stale section-header comment at :537 (still says
   `// TestResolveRoles_ReasoningShippedDefault`) to match the renamed func.
2. `internal/cmd/default_action_test.go`: VERIFY `TestProgressLabel_DecomposeVerboseRoles` (:1415) already
   asserts NO `(reasoning: …)` suffix (the inverted `if strings.Contains(stderr, "(reasoning:")` at :1440).
3. `internal/cmd/root.go`: VERIFY the `--reasoning` flag help (:137) already says "default off for every role".
4. `pkg/stagecoach/stagecoach.go`: VERIFY the `RoleModel` doc comments (:62, :66) already say "off by default for every role".
5. `docs/cli.md`: VERIFY the `--reasoning` row (:43) default column is already `"" (off)`.

**Success Definition**: The verification gates pass (the four "already-correct" sites confirmed; the
stale comment fixed; `grep "ReasoningShippedDefault"` → ZERO; `grep 'planner: high\|planner=high'` → ZERO;
`go test ./...` green). No other code is touched. `ui/verbose_test.go` is unchanged (Finding 3).

## User Persona

**Target User**: The Stagecoach contributor / reviewer confirming the plan/004 "reasoning opt-in
everywhere" changeset is consistently reflected in ALL downstream surfaces (decompose tests, the verbose
progress-label test, the CLI flag help, the public API doc comment, and the CLI docs) before the
changeset ships.

**Use Case**: A grep + green-test pass proving no shipped `planner=high` default survives in any test
assertion or any user-facing string, so `--reasoning`/`--planner-reasoning` are honestly opt-in
everywhere and the docs match the behavior.

**Pain Points Addressed**: Prevents a wasted edit pass hunting for stale `planner=high` assertions that
were already flipped, and removes the one genuinely-stale artifact (the orphaned section-header comment)
that a faithful "rename" should have caught.

## Why

- **Completes the plan/004 Change A ripple.** S1 flipped the config layer (default off for every role);
  S2 is the downstream mirror — the tests that asserted the old default, and the user-facing strings that
  advertised it, must all say "off". A stale `planner: high` in the flag help or docs would mislead users.
- **Almost entirely already done.** A repo-wide grep for the shipped-default phrasing
  (`planner: high` / `planner=high`) returns ZERO; the four assertion/surface sites are each in the
  post-change target state; `go test ./...` is green. Re-editing them would be churn.
- **The one real fix is the missed rename of a doc banner.** The func `TestResolveRoles_ReasoningShippedDefault`
  was renamed to `..._NoShippedReasoningDefault` but its section-header comment block was not — leaving a
  doc banner that names a test that no longer exists. Completing the rename (1 line) removes the last
  `ReasoningShippedDefault` reference in the tree.

## What

A verification pass over the five S2 sites (decompose/roles_test.go, default_action_test.go, root.go,
pkg/stagecoach/stagecoach.go, docs/cli.md) plus one targeted comment fix. No behavioral change (the
production behavior — `off` for every role — was already landed by S1/earlier work). The authoritative
"no stale shipped-default" check is `grep 'planner: high\|planner=high'` (ZERO) — the contract's
`grep 'planner.*high'` is a rough sanity check that still matches legitimate per-role/formatter/example
references (see Gotchas).

### Success Criteria

- [ ] `internal/decompose/roles_test.go:537` section-header comment reads `// TestResolveRoles_NoShippedReasoningDefault` (matches the func at :542).
- [ ] `TestResolveRoles_NoShippedReasoningDefault` (:542) asserts planner/stager/message/arbiter reasoning == `""`.
- [ ] `TestResolveRoles_ReasoningPerRoleSet` (:565) asserts planner `""` + message `"low"`.
- [ ] `default_action_test.go:1440` asserts NO `(reasoning:` substring in the verbose decompose stderr.
- [ ] `root.go:137` `--reasoning` help contains "default off for every role".
- [ ] `pkg/stagecoach/stagecoach.go` RoleModel comments (:62, :66) say "off by default".
- [ ] `docs/cli.md:43` `--reasoning` default column is `"" (off)`.
- [ ] `grep -rn "ReasoningShippedDefault" --include="*.go" .` (excl. plan/) → ZERO.
- [ ] `grep -rn 'planner: high\|planner=high' internal/ pkg/ docs/` → ZERO.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` → all green.
- [ ] `ui/verbose_test.go` is UNCHANGED (Finding 3).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes — and the key context is that the work is ~95% already done.** This PRP quotes the
LIVE post-change state of all five sites (so the implementer recognizes "this is already in the target
state" rather than hunting for stale `planner=high` assertions), identifies the ONE genuine edit (the
orphaned section-header comment), and gives the precise grep gates that distinguish stale
shipped-default refs from legitimate per-role/formatter/example references. The architecture docs
(`system_context.md §3`, `critical_findings.md` Finding 3) describe the intended delta and the
verbose_test.go guard.

### Documentation & References

```yaml
# MUST READ — the intended delta + the verbose_test.go guard
- docfile: plan/004_136878664597/docs/system_context.md
  why: "§3 'Change A' lists S2's exact file set (decompose/roles_test.go, default_action_test.go, root.go, pkg/stagecoach/stagecoach.go, docs/cli.md) and the per-site intended edits."
  critical: "§3 confirms S1 = config package; S2 = these downstream surfaces. The verbose_test.go guard is NOT in S2's list."

- docfile: plan/004_136878664597/docs/critical_findings.md
  why: "Finding 3: ui/verbose_test.go sets Reasoning:\"high\" on the planner RoleLine to test the reasoningSuffix FORMATTER — it MUST NOT be changed (it is not a default test). Finding 2: the test suite is the exhaustive oracle."
  critical: "Finding 3 is the #1 trap — an implementer who greps 'planner.*high' and 'fixes' verbose_test.go would DELETE the formatter's only coverage. Do not touch it."

- docfile: plan/004_136878664597/P1M1T1S1/PRP.md
  why: "S1 — the sibling that landed (verified) the config-layer flip (defaultRoleReasoning removed; ResolveRoleModel → cfg.Reasoning; config.go doc comments; roles_test.go). S1 is already complete; S2 consumes its behavioral output (planner now resolves to \"\")."

- docfile: plan/004_136878664597/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: the per-site verification table (4 of 5 already done), the ONE genuine edit (roles_test.go:537 comment), the imprecise-grep framing, and the verbose_test.go guard."

# The S2 sites (VERIFY; one needs a 1-line edit)
- file: internal/decompose/roles_test.go
  why: "VERIFY + 1 EDIT. The func is ALREADY TestResolveRoles_NoShippedReasoningDefault (:542) with planner assertions already \"\" (:551-552, :579-580) and ReasoningPerRoleSet message=\"low\" (:582). The ONLY edit: the section-header COMMENT at :537 still says '// TestResolveRoles_ReasoningShippedDefault' — change it to match the renamed func."
  gotcha: "The rename of the FUNC is done; only its doc-banner comment was missed. Do NOT re-flip the (already-correct) assertions."
- file: internal/cmd/default_action_test.go
  why: "VERIFY (no edit). TestProgressLabel_DecomposeVerboseRoles (:1415) already asserts NO '(reasoning: …)' suffix via `if strings.Contains(stderr, \"(reasoning:\")` (:1440) with the explanatory comment (:1439). Already the inverted assertion the contract specifies."
- file: internal/cmd/root.go
  why: "VERIFY (no edit). The --reasoning flag help (:137) already reads '…default off for every role)'. Already target wording."
- file: pkg/stagecoach/stagecoach.go
  why: "VERIFY (no edit). The RoleModel doc comment (:62) already says 'off by default for every role'; the Reasoning field comment (:66) already says 'off by default'. Already target wording."
- file: docs/cli.md
  why: "VERIFY (no edit). The --reasoning row (:43) default column is already '\"\" (off)' (NOT '(off; planner: high)'). Already target wording."

# MUST-NOT-TOUCH (Finding 3)
- file: internal/ui/verbose_test.go
  why: "DO NOT TOUCH. Lines 13/43/53 set Reasoning:\"high\" on the planner RoleLine; line 20 asserts 'DEBUG: planner  p in pi (reasoning: high)'. This tests the reasoningSuffix FORMATTER (does the suffix render when reasoning is non-empty?), NOT the shipped default. Changing it removes the formatter's only coverage."

# Read-only cross-refs (legitimate planner...high matches — NOT stale)
- file: internal/config/roles_test.go
  why: "READ-ONLY. Lines 132/136/188 match 'planner.*high' but are PER-ROLE OVERRIDE tests (explicit cfg values) — S1's territory, already verified. Legitimate; not stale."
- file: docs/configuration.md
  why: "READ-ONLY. Line 153 'STAGECOACH_PLANNER_REASONING=high' is a per-role env-var EXAMPLE (P1.M1.T2/T3 territory). Legitimate; not stale."
```

### Current Codebase Tree (S2 scope)

```bash
stagecoach/
├── internal/decompose/roles_test.go   # VERIFY + 1-line comment fix (:537)
├── internal/cmd/default_action_test.go # VERIFY (already inverted at :1440)
├── internal/cmd/root.go                # VERIFY (--reasoning help :137 already correct)
├── pkg/stagecoach/stagecoach.go          # VERIFY (RoleModel comments :62/:66 already correct)
├── docs/cli.md                         # VERIFY (--reasoning row :43 already '"" (off)')
└── internal/ui/verbose_test.go         # MUST NOT TOUCH (Finding 3 — formatter test)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (ONE 1-line edit; everything else unchanged)
    internal/decompose/roles_test.go   # :537 comment: ReasoningShippedDefault → NoShippedReasoningDefault
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/decompose/roles_test.go` | VERIFY + 1 EDIT | Confirm func/assertions already target; fix the orphaned section-header comment at :537. |
| `internal/cmd/default_action_test.go` | VERIFY (no edit) | Confirm :1440 already asserts no `(reasoning:` suffix. |
| `internal/cmd/root.go` | VERIFY (no edit) | Confirm :137 already "default off for every role". |
| `pkg/stagecoach/stagecoach.go` | VERIFY (no edit) | Confirm :62/:66 already "off by default". |
| `docs/cli.md` | VERIFY (no edit) | Confirm :43 already `"" (off)`. |

**Explicitly NOT touched**: `internal/config/*` (S1 — verified complete), `internal/ui/verbose_test.go`
(Finding 3 — formatter test), `docs/configuration.md` (P1.M1.T2/T3; its :153 env-var example is
legitimate), `README.md` + other docs (P1.M1.T3.S1), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL: the contract's described STARTING STATE (TestResolveRoles_ReasoningShippedDefault func name,
// planner "high" assertions, a "(reasoning: high)" suffix assertion, 'planner: high' flag help,
// 'shipped default' RoleModel comments, '(off; planner: high)' docs cell) DOES NOT EXIST in the live
// code. Four of five sites are already target-state; the repo is GREEN. Do NOT hunt for stale assertions
// to flip — they were already flipped. The ONE real edit is the orphaned comment at roles_test.go:537.

// CRITICAL (Finding 3): ui/verbose_test.go sets Reasoning:"high" on the planner RoleLine and asserts
// "(reasoning: high)". This tests the reasoningSuffix FORMATTER, NOT the default. It MUST NOT be changed.
// An implementer who greps 'planner.*high' and "fixes" verbose_test.go would DELETE the formatter coverage.

// CRITICAL (imprecise grep): the contract's verification `grep -rn 'planner.*high' internal/ pkg/ docs/`
// is a ROUGH sanity check, NOT a "zero matches" gate. It STILL legitimately matches:
//   - internal/config/roles_test.go:132,136,188 (per-role OVERRIDE tests — S1's, valid)
//   - internal/ui/verbose_test.go:13,20,43,53 (FORMATTER tests — Finding 3, MUST NOT change)
//   - docs/configuration.md:153 (per-role env-var EXAMPLE — valid)
//   - false positives on "higher" (e.g. config/file_test.go "higher layer" model-merge comments)
// The AUTHORITATIVE stale-check is `grep -rn 'planner: high\|planner=high'` → ZERO (confirmed), plus
// `grep "ReasoningShippedDefault"` → ZERO (after the :537 comment fix).

// GOTCHA: the rename of the FUNC TestResolveRoles_NoShippedReasoningDefault is DONE (:542). Only its
// SECTION-HEADER COMMENT block (:533-539) still names the old test. Fix the comment, not the func.

// GOTCHA: go test ./... is the exhaustive oracle (critical_findings.md Finding 2). Green + the two
// authoriative greps empty = provably complete. No hand-audit of every planner reference is needed.
```

## Implementation Blueprint

### Data models and structure

No data-model change. The S1 behavioral output (planner resolves reasoning to `""` when nothing is set)
is what S2's test assertions verify; the user-facing strings describe it. No struct/type changes.

### Implementation Tasks (ordered — verify-then-fix)

```yaml
Task 1: VERIFY default_action_test.go (no edit)
  - RUN: grep -n "TestProgressLabel_DecomposeVerboseRoles\|(reasoning:" internal/cmd/default_action_test.go
  - EXPECT: TestProgressLabel_DecomposeVerboseRoles (:1415) + `if strings.Contains(stderr, "(reasoning:")` (:1440).
  - CONFIRM the assertion is INVERTED (errors when the suffix IS present), with the comment at :1439.
  - If already correct (it is): no edit.

Task 2: VERIFY root.go --reasoning help (no edit)
  - RUN: grep -n "default off for every role\|planner: high" internal/cmd/root.go
  - EXPECT: a match for "default off for every role" (:137); ZERO matches for "planner: high".
  - If already correct (it is): no edit.

Task 3: VERIFY pkg/stagecoach/stagecoach.go RoleModel comments (no edit)
  - RUN: grep -n "off by default\|shipped default" pkg/stagecoach/stagecoach.go
  - EXPECT: "off by default for every role" (:62) + "off by default" (:66); ZERO "shipped default".
  - If already correct (it is): no edit.

Task 4: VERIFY docs/cli.md --reasoning row (no edit)
  - RUN: grep -n "reasoning <level>" docs/cli.md
  - EXPECT: the row's Default column is `"" (off)` (NOT `"" (off; planner: high)`).
  - If already correct (it is): no edit.

Task 5: FIX the stale section-header comment in decompose/roles_test.go (THE ONE EDIT)
  - LOCATE line 537: the banner comment `// TestResolveRoles_ReasoningShippedDefault` sitting between two
    `// ----...----` lines, directly above `func TestResolveRoles_NoShippedReasoningDefault` (:542).
  - EDIT: change `// TestResolveRoles_ReasoningShippedDefault` → `// TestResolveRoles_NoShippedReasoningDefault`
    (so the doc banner matches the func it documents).
  - DO NOT touch the func (already renamed) or the assertions (already "").
  - WHY: completes the contract's "RENAME" — the func rename was done but its header comment was missed;
    this removes the last `ReasoningShippedDefault` reference in the tree.

Task 6: VERIFY the decompose test assertions are target-state (no edit beyond Task 5)
  - RUN: grep -n "NoShippedReasoningDefault\|want \"\\\\\"\".*no shipped default\|want low (per-role" internal/decompose/roles_test.go
  - EXPECT: TestResolveRoles_NoShippedReasoningDefault (:542) with planner/stager/message/arbiter all
    want ""; TestResolveRoles_ReasoningPerRoleSet (:565) planner "" + message "low". (Already correct.)

Task 7: GATE — the authoritative greps + full suite
  - RUN: grep -rn "ReasoningShippedDefault" --include="*.go" . | grep -v "/plan/"   # → ZERO (after Task 5)
  - RUN: grep -rn 'planner: high\|planner=high' internal/ pkg/ docs/                 # → ZERO
  - RUN: go build ./... ; go vet ./... ; gofmt -l .                                  # all clean
  - RUN: go test -race ./...                                                          # all green
  - CONFIRM (guard): git diff --stat -- internal/ui/verbose_test.go                  # EMPTY (untouched)
```

### Implementation Patterns & Key Details

```go
// === The ONE edit: internal/decompose/roles_test.go:537 (comment banner → matches the renamed func) ===
// ---------------------------------------------------------------------------
// TestResolveRoles_NoShippedReasoningDefault      // was: TestResolveRoles_ReasoningShippedDefault
// ---------------------------------------------------------------------------

func TestResolveRoles_NoShippedReasoningDefault(t *testing.T) {   // ← already renamed (unchanged)
	...
	if rmodels.Planner.Reasoning != "" {                            // ← already "" (unchanged)
		t.Errorf("planner reasoning = %q, want \"\" (off — no shipped default)", rmodels.Planner.Reasoning)
	}
	...
}
```

```go
// === The 4 already-correct sites (VERIFY ONLY — do not edit) ===

// default_action_test.go:1440 (already the INVERTED assertion)
if strings.Contains(stderr, "(reasoning:") {    // all four roles default to off ⇒ reasoningSuffix=="" ⇒ NO suffix
    ...error...                                  // (contract: "assert NO reasoning suffix appears on ANY role line")
}

// root.go:137 (already "default off for every role")
"...default off for every role)"

// pkg/stagecoach/stagecoach.go:62 (already "off by default for every role")
"...off by default for every role)."

// docs/cli.md:43 (already '"" (off)')
| `--reasoning <level>` | string | "" (off) | ... |
```

### Integration Points

```yaml
DOWNSTREAM SURFACES (S2 scope — verify + 1 comment fix):
  - internal/decompose/roles_test.go   # :537 comment fix (func/assertions already target)
  - internal/cmd/default_action_test.go # :1440 already inverted (verify only)
  - internal/cmd/root.go                # :137 already "default off for every role" (verify only)
  - pkg/stagecoach/stagecoach.go          # :62/:66 already "off by default" (verify only)
  - docs/cli.md                         # :43 already '"" (off)' (verify only)

CONSUMED (S1's output — already landed):
  - ResolveRoleModel returns reasoning="" for all roles when nothing set (planner no longer "high")

GATE: go test -race ./... → GREEN ; grep 'planner: high|planner=high' → ZERO ; grep ReasoningShippedDefault → ZERO

MUST-NOT-TOUCH:
  - internal/ui/verbose_test.go   # Finding 3 — reasoningSuffix FORMAT test (fixture sets Reasoning:"high")
  - internal/config/*             # S1's territory (verified complete)
  - docs/configuration.md         # P1.M1.T2/T3 (the :153 env-var example is legitimate)
  - README.md + other docs        # P1.M1.T3.S1
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by SIBLING subtasks, NOT S2):
  - P1.M1.T2.S1: config init emits uncommented reasoning = "off" in [defaults] (FR-B1) + docs/configuration.md
  - P1.M1.T3.S1: verify README.md + remaining docs/ have no stale reasoning-default references
```

## Validation Loop

### Level 1: The Authoritative Stale-Reference Gates

```bash
cd /home/dustin/projects/stagecoach

# (1) No orphaned old test name anywhere (after the :537 comment fix)
grep -rn "ReasoningShippedDefault" --include="*.go" . | grep -v "/plan/" && echo "FAIL: stale name present" || echo "PASS: absent"

# (2) No shipped-default phrasing anywhere
grep -rn 'planner: high\|planner=high' internal/ pkg/ docs/ && echo "FAIL: stale shipped default" || echo "PASS: absent"

# (3) The func is renamed; the comment matches
grep -n "TestResolveRoles_NoShippedReasoningDefault" internal/decompose/roles_test.go   # → 2 matches (comment + func)
```

### Level 2: The Per-Site Verification (the 4 already-correct surfaces)

```bash
cd /home/dustin/projects/stagecoach

# default_action_test.go: inverted assertion present
grep -n 'strings.Contains(stderr, "(reasoning:")' internal/cmd/default_action_test.go   # → :1440

# root.go: "default off for every role"
grep -n "default off for every role" internal/cmd/root.go                                # → :137

# pkg/stagecoach: "off by default"
grep -n "off by default" pkg/stagecoach/stagecoach.go                                      # → :62, :66

# docs/cli.md: '"" (off)' default column
grep -n '^| `--reasoning <level>`' docs/cli.md                                           # → row with '"" (off)'
```

### Level 3: The Exhaustive Oracle (full suite)

```bash
cd /home/dustin/projects/stagecoach

go build ./...           # Expected: exit 0
go vet ./...             # Expected: exit 0
gofmt -l .               # Expected: empty
go test -race ./...      # Expected: ALL packages green (decompose + cmd + config + provider + ui + ...)
```

### Level 4: Scope-Boundary Check (verbose_test.go untouched; only 1 file edited)

```bash
cd /home/dustin/projects/stagecoach

# verbose_test.go MUST be unchanged (Finding 3)
git diff --stat -- internal/ui/verbose_test.go                                           # Expected: EMPTY

# S2's only production/test edit is the 1-line comment in decompose/roles_test.go
git diff --stat -- internal/ pkg/ docs/
# Expected: only internal/decompose/roles_test.go (the :537 comment). (S1's config files are not touched by S2.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `grep "ReasoningShippedDefault"` repo-wide (excl. plan/) → ABSENT (after the :537 fix).
- [ ] `grep 'planner: high\|planner=high' internal/ pkg/ docs/` → ABSENT.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` → all green.

### Feature Validation

- [ ] `roles_test.go:537` comment banner reads `TestResolveRoles_NoShippedReasoningDefault` (matches func :542).
- [ ] `TestResolveRoles_NoShippedReasoningDefault` asserts all four roles reasoning == `""`.
- [ ] `TestResolveRoles_ReasoningPerRoleSet` asserts planner `""` + message `"low"`.
- [ ] `default_action_test.go:1440` asserts NO `(reasoning:` suffix in verbose decompose stderr.
- [ ] `root.go:137` `--reasoning` help says "default off for every role".
- [ ] `pkg/stagecoach/stagecoach.go` RoleModel comments say "off by default".
- [ ] `docs/cli.md:43` `--reasoning` default column is `"" (off)`.

### Scope Discipline Validation

- [ ] ONLY `internal/decompose/roles_test.go` edited by S2 (the 1-line :537 comment); the other 4 sites verified-only.
- [ ] `ui/verbose_test.go` UNCHANGED (Finding 3 — `git diff --stat` empty).
- [ ] Did NOT touch `internal/config/*` (S1), `docs/configuration.md` (P1.M1.T2/T3), README/other docs (P1.M1.T3.S1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (other than this PRP + research note).

### Code Quality Validation

- [ ] The 4 verified sites are recognized as already-correct (no churn).
- [ ] The 1 edit completes the rename (comment matches func) — no behavioral change.
- [ ] The `grep 'planner.*high'` rough check is interpreted correctly (legitimate per-role/formatter/example matches distinguished from stale shipped-default refs).

---

## Anti-Patterns to Avoid

- ❌ Don't hunt for stale `planner=high` assertions to flip — they were already flipped. Four of five sites
  are target-state; the repo is GREEN. Re-editing correct code is churn that risks regressions.
- ❌ Don't touch `ui/verbose_test.go`. Finding 3 is explicit: its `Reasoning:"high"` planner fixture tests
  the `reasoningSuffix` FORMATTER, not the default. "Fixing" it deletes the formatter's only coverage.
- ❌ Don't treat `grep 'planner.*high'` returning matches as a failure. It legitimately matches per-role
  OVERRIDE tests (config/roles_test.go), FORMAT tests (ui/verbose_test.go), an env-var EXAMPLE
  (docs/configuration.md:153), and "higher" false positives. The authoritative check is
  `grep 'planner: high\|planner=high'` → ZERO.
- ❌ Don't rename the func `TestResolveRoles_NoShippedReasoningDefault` — it is ALREADY renamed. The only
  stale artifact is its SECTION-HEADER COMMENT (:537); fix the comment, not the func.
- ❌ Don't expand into `internal/config/*` (S1 — verified complete), `docs/configuration.md` (P1.M1.T2/T3),
  or README/other docs (P1.M1.T3.S1). S2 is the five listed surfaces only.
- ❌ Don't fabricate a before/after to justify the subtask. The honest outcome is "~95% already complete;
  one orphaned comment fixed; verified green."
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** that the verification passes and the single comment fix lands cleanly.

Rationale: Four independent pieces of evidence confirm S2 is ~95% already realized: (1) the verbatim live
state of all five sites — `TestResolveRoles_NoShippedReasoningDefault` (:542) with planner assertions
already `""` (:551-552, :579-580); the inverted `default_action_test.go:1440` assertion; root.go:137
"default off for every role"; pkg/stagecoach:62/:66 "off by default"; docs/cli.md:43 `"" (off)` — each in
the post-change target state; (2) the shipped-default phrasing `grep 'planner: high\|planner=high'`
returns ZERO across the tree; (3) `grep "ReasoningShippedDefault"` returns exactly ONE line (the orphaned
:537 comment) — the single genuine edit; (4) `go test ./...` is GREEN. The contract's described starting
state (stale func name, planner "high" assertions, "(reasoning: high)" suffix assertion, 'planner: high'
help, 'shipped default' comments, '(off; planner: high)' docs cell) simply does not exist. The PRP
therefore redirects the implementer from "perform the five edits" to "verify four, fix one comment," with
the verbose_test.go guard (Finding 3) and the imprecise-grep framing called out explicitly so the
implementer does not delete formatter coverage or chase legitimate per-role/example matches. The residual
0.5 uncertainty is purely the mechanical application of the 1-line comment edit + gofmt, which the
deterministic gates catch immediately.
