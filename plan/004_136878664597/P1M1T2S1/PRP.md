---
name: "P1.M1.T2.S1 — Add uncommented reasoning = "off" to bootstrap config [defaults] + test + docs/configuration.md"
description: |
  ⚠️ STATUS: 100% ALREADY REALIZED IN THE CODEBASE. The contract describes inserting an uncommented
  `reasoning = "off"` line into `internal/config/bootstrap.go`'s `[defaults]` writer, adding a test
  assertion to `bootstrap_test.go`, and uncommenting `reasoning` in `docs/configuration.md` — but ALL
  THREE sites are ALREADY in the exact target state at HEAD (committed in `9d33b9e` "make reasoning off
  by default for all roles", the same commit that landed the sibling S1/S2 reasoning-default flip). The
  working tree is CLEAN; `go test ./internal/config/` is GREEN; there is EXACTLY ONE reasoning line in
  bootstrap.go. This PRP is a VERIFY-AND-CONFIRM runbook: run the five gates, confirm each site, and pass.
  Do NOT blindly "insert" per the contract — a duplicate `reasoning = "off"` line in `[defaults]` would
  break `TestBuildBootstrapConfig_ValidTOML` (go-toml/v2 rejects duplicate keys). The only permissible
  edit is a single surgical one IF (and only if) a gate reveals a site is somehow not yet target-state.
---

## Goal

**Feature Goal**: Ensure `stagecoach config init` produces a config file with an **uncommented**
`reasoning = "off"` under `[defaults]` (FR-B1 discoverability), that the bootstrap test asserts this, and
that `docs/configuration.md`'s config example matches — with NO duplicate lines and the full suite green.
**In the live codebase this state is already fully achieved; this subtask's job is to VERIFY it is
complete (and, in the unlikely event a site drifted, apply one surgical edit).**

**Deliverable** (verify-and-confirm; expected zero edits at HEAD):
1. `internal/config/bootstrap.go`: the `buildBootstrapConfig` `[defaults]` writer emits EXACTLY ONE
   uncommented `reasoning = "off"` line (with the FR-R6 comment) after the `provider` line.
2. `internal/config/bootstrap_test.go`: `TestBuildBootstrapConfig_Pi` asserts `content` contains
   `reasoning = "off"` (uncommented) under `[defaults]`.
3. `docs/configuration.md`: the "Populated config" example has `reasoning = "off"` UNCOMMENTED (line 80)
   with the `# off|low|medium|high; off by default for every role (FR-R6)` comment.
4. The full test suite is green, including `TestBuildBootstrapConfig_ValidTOML` (the duplicate-detector).

**Success Definition**: All five verification gates pass (the three sites are each target-state; the
reasoning line count is exactly 1 in bootstrap.go and uncommented in the docs; `go test -race ./...` is
green). No duplicate is introduced. If every gate already passes at HEAD (it does), the outcome is
"verified complete — no source edits."

## User Persona

**Target User**: The Stagecoach contributor / reviewer confirming the plan/004 "reasoning opt-in
everywhere" changeset is consistently reflected in the `config init` bootstrap output and its docs, so a
fresh `stagecoach config init` visibly exposes the `reasoning` field (discoverable + obviously opt-in)
rather than hiding it.

**Use Case**: A new user runs `stagecoach config init`, opens the generated config, and sees
`reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)` — they immediately
know the field exists, that `off` is the default, and that they can opt in per role. The test guards this
against regression; the docs match what the command emits.

**Pain Points Addressed**: Prevents the "property absent from the written config is, for the user, a
property that does not exist" failure mode (FR-B1 rationale), and prevents a naive re-implementation from
introducing a duplicate `reasoning =` key that would corrupt the generated TOML.

## Why

- **FR-B1 mandates an explicit, discoverable `reasoning = "off"` in the bootstrap `[defaults]`.** The PRD
  rationale (§9.17 FR-B1): the shipped default is `off` for every role (FR-R6), emitted *explicitly* "so
  the field is discoverable and obviously opt-in in the generated file rather than hidden." This subtask
  owns that emission + its test + the matching docs example.
- **Already landed.** Commit `9d33b9e` ("make reasoning off by default for all roles") implemented this
  alongside the Change-A behavioral flip (S1/S2). The plan/004 `system_context.md §3` lists these sites as
  "to change" because it was a planning sketch against an older snapshot; the implementing commit already
  did them. Re-applying the contract verbatim would create DUPLICATES that break the TOML validity test.
- **The duplicate-key risk is real and silent-ish.** go-toml/v2 rejects a duplicate `reasoning` key in
  `[defaults]`; `TestBuildBootstrapConfig_ValidTOML` would fail — but `TestBuildBootstrapConfig_Pi`'s
  `strings.Contains` check would still pass (it can't distinguish one occurrence from two). So a naive
  "insert" looks superficially fine yet breaks the ValidTOML gate. This PRP front-loads that trap.
- **No new design, no schema change, no API change.** This is a one-line config emission + a one-line test
  assertion + a one-line docs uncomment, all already present. The risk is entirely in NOT undoing/redoing
  committed work incorrectly.

## What

A verification pass over the three sites (`bootstrap.go`, `bootstrap_test.go`, `docs/configuration.md`)
plus the green-suite gate. No behavioral change is expected (the production behavior — emitting
`reasoning = "off"` — already exists). The authoritative checks are (a) exactly-one reasoning line in
bootstrap.go, (b) the assertion present in the test, (c) reasoning uncommented in the docs example, and
(d) `TestBuildBootstrapConfig_ValidTOML` green (the duplicate-detector).

### Success Criteria

- [ ] `internal/config/bootstrap.go` `buildBootstrapConfig` emits EXACTLY ONE uncommented
      `reasoning = "off"` line (with `# off|low|medium|high; off by default for every role (FR-R6)`)
      after the `provider` line, before the commented `# model` line.
- [ ] `internal/config/bootstrap_test.go` `TestBuildBootstrapConfig_Pi` asserts the content contains
      `reasoning = "off"` (uncommented) under `[defaults]`.
- [ ] `docs/configuration.md` "Populated config" example has `reasoning = "off"` UNCOMMENTED (~line 80)
      with the FR-R6 comment (NOT a `# reasoning = …` commented line).
- [ ] `TestBuildBootstrapConfig_ValidTOML` passes (proves no duplicate `reasoning` key in `[defaults]`).
- [ ] `grep -c 'reasoning = \\"off\\"' internal/config/bootstrap.go` → **1** (not 0, not 2).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` → all green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes — and the key context is that the work is already 100% done at HEAD.** This PRP
quotes the LIVE post-change state of all three sites verbatim (so the implementer recognizes "this is
already in the target state" rather than blindly inserting a duplicate), identifies the duplicate-key
failure mode, gives the exact five verification gates with expected values, and prescribes the single
surgical edit to apply ONLY if a gate reveals drift. The architecture docs (`docs/system_context.md §3`)
and the research note pin the provenance (commit `9d33b9e`).

### Documentation & References

```yaml
# MUST READ — the intended delta + provenance
- docfile: plan/004_136878664597/docs/system_context.md
  why: "§3 'Change B' lists this task's three sites (bootstrap.go:~124, bootstrap_test.go:~26-31, docs/configuration.md:77-83) and the intended edits. §1 confirms the baseline is GREEN and the repo is mature."
  critical: "§3 was a PLANNING sketch against an older snapshot. The actual implementing commit 9d33b9e already landed all three Change-B edits alongside Change A. The contract's 'starting state' (reasoning absent/commented) DOES NOT EXIST at HEAD — verify, do not blindly insert."

- docfile: plan/004_136878664597/P1M1T2S1/research/bootstrap_reasoning_verification.md
  why: "EMPIRICALLY VERIFIED findings: all three sites target-state at HEAD; the implementing commit is 9d33b9e; the working tree is CLEAN; baseline config tests GREEN; the #1 failure mode is a DUPLICATE reasoning line that breaks TestBuildBootstrapConfig_ValidTOML; the five deterministic gates. READ THIS FIRST."
  critical: "§3 (the duplicate-line failure mode) and §5 (the five gates) are the core of the task. §2 quotes the live verbatim state of each site so the implementer confirms rather than re-inserts."

# The three edit sites (VERIFY; surgical edit only if drifted)
- file: internal/config/bootstrap.go
  why: "Site (a). buildBootstrapConfig's [defaults] writer (lines ~120-128) ALREADY emits `reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below` (line 127) after the provider line, before the commented # model line. Byte-for-byte the contract string."
  pattern: "The [defaults] block writes provider (uncommented, with a conditional not-detected annotation), THEN the reasoning line (uncommented), THEN the # model/# timeout/# auto_stage_all/# verbose commented block."
  gotcha: "There must be EXACTLY ONE reasoning line. A second one (from a blind contract 'insert') makes the generated TOML have a duplicate `reasoning` key under [defaults] → TestBuildBootstrapConfig_ValidTOML fails. Do NOT add a duplicate."

- file: internal/config/bootstrap_test.go
  why: "Site (b). TestBuildBootstrapConfig_Pi ALREADY asserts `strings.Contains(content, `reasoning = \"off\"`)` with the error `missing uncommented reasoning = \"off\" in [defaults]`, immediately after the provider = \"pi\" check. A doc comment rides above it."
  pattern: "Same-package test (package config); table/helper style. assertContains helper + direct strings.Contains. TestBuildBootstrapConfig_ValidTOML (later in the file) unmarshals 6 (target,installed) cases — it is the duplicate-detector."
  gotcha: "Do NOT add a second identical assertion. Do NOT weaken TestBuildBootstrapConfig_ValidTOML to tolerate duplicates (it is the guard against exactly this regression)."

- file: docs/configuration.md
  why: "Site (c). The 'Populated config' TOML example (~lines 77-83) ALREADY has `reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6)` UNCOMMENTED at line 80, between provider and the commented # model block."
  pattern: "The example mirrors what `config init` emits (provider claude, reasoning off uncommented, model/timeout/auto_stage_all/verbose commented, then [role.*] blocks)."
  gotcha: "Line 80 must NOT start with `# ` (that would mean reasoning is still commented — it is NOT). Confirm with `grep -n '^reasoning = \"off\"' docs/configuration.md` → a single match at :80."

- docfile: plan/004_136878664597/P1M1T1S2/PRP.md
  why: "The sibling verify-and-confirm PRP (S2) for Change A's downstream surfaces. Establishes the SAME pattern (work ~95-100% already done; verify, don't churn) and confirms docs/configuration.md is THIS task's territory (S2 explicitly leaves it to P1.M1.T2)."
  critical: "S2 does NOT touch bootstrap.go, bootstrap_test.go, or docs/configuration.md — no conflict. The two subtasks are complementary: S2 = downstream test/surface verification; S1(this) = bootstrap emission + its test + the config-example doc."

# Cross-references (read-only — do NOT edit)
- file: internal/config/config.go
  why: "READ-ONLY. Config.Reasoning is the [defaults].reasoning field; Defaults() sets it \"\". The bootstrap writes \"off\" explicitly (discoverability) even though the in-memory default is \"\". S1 (Change A) already updated the doc comments here to 'off for every role'."
- file: internal/config/roles.go
  why: "READ-ONLY. ResolveRoleModel returns reasoning=\"\" for all roles when nothing is set (Change A, landed by S1). The bootstrap's `reasoning = \"off\"` is the user-facing discoverable default that matches this behavior."
- url: https://github.com/pelletier/go-toml/v2#readme
  why: "Confirms go-toml/v2 rejects duplicate keys in the same table (a duplicate `reasoning` key under [defaults] is a decode error). This is WHY a blind contract 'insert' breaks TestBuildBootstrapConfig_ValidTOML."
  critical: "The duplicate-key rejection is the deterministic safety net that catches a naive duplicate insertion — but the implementer should not rely on it; verify exactly-one FIRST."
```

### Current Codebase Tree (this task's scope)

```bash
stagecoach/
├── internal/config/
│   ├── bootstrap.go        # Site (a): [defaults] writer — reasoning = "off" ALREADY present (line 127)
│   └── bootstrap_test.go   # Site (b): TestBuildBootstrapConfig_Pi assertion ALREADY present
└── docs/
    └── configuration.md    # Site (c): [defaults] example — reasoning ALREADY uncommented (line 80)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (expected: ZERO edits at HEAD — all three sites are target-state)
    internal/config/bootstrap.go        # unchanged (already emits exactly one reasoning = "off")
    internal/config/bootstrap_test.go   # unchanged (assertion already present)
    docs/configuration.md               # unchanged (reasoning already uncommented)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/bootstrap.go` | VERIFY (edit ONLY if drifted) | Confirm exactly-one uncommented `reasoning = "off"` line in `[defaults]`. |
| `internal/config/bootstrap_test.go` | VERIFY (edit ONLY if drifted) | Confirm the `reasoning = "off"` assertion in `TestBuildBootstrapConfig_Pi`. |
| `docs/configuration.md` | VERIFY (edit ONLY if drifted) | Confirm `reasoning = "off"` uncommented in the Populated config example. |

**Explicitly NOT touched**: `internal/config/config.go`, `roles.go`, `roles_test.go` (S1 — verified
complete), `internal/decompose/*`, `internal/cmd/*`, `pkg/stagecoach/*`, `docs/cli.md` (S2 — the
downstream-surfaces verify), `internal/ui/verbose_test.go` (the formatter test), `README.md` + other docs
(P1.M1.T3.S1), `providers/*.toml`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (#1 failure mode — the DUPLICATE line): the contract is phrased "INSERT ...". An implementer
// who does not read the live file will add a SECOND `reasoning = "off"` line to bootstrap.go's
// [defaults] writer. Consequences:
//   - TestBuildBootstrapConfig_Pi STILL PASSES (strings.Contains can't tell 1 from 2).
//   - TestBuildBootstrapConfig_ValidTOML FAILS — go-toml/v2 rejects a duplicate `reasoning` key in [defaults]
//     ("toml: key 'reasoning' already exists"). This test unmarshals 6 (target,installed) cases.
// MITIGATION: the line is ALREADY present (bootstrap.go:127). Verify EXACTLY ONE; do NOT insert.
//   grep -c 'reasoning = \\"off\\"' internal/config/bootstrap.go   # MUST be 1

// CRITICAL (the contract's starting state does not exist): the contract and system_context §3 describe a
// starting state where reasoning is ABSENT (bootstrap.go) / COMMENTED (docs). At HEAD that state DOES NOT
// EXIST — commit 9d33b9e landed all three edits. Do NOT assume the starting state; VERIFY the current
// state first. The working tree is CLEAN (committed) — do not "re-do" committed work.

// GOTCHA (the in-memory default "" vs the bootstrap "off"): Config.Reasoning defaults to "" in memory
// (config.go Defaults()), but the bootstrap WRITES "off" explicitly for discoverability (FR-B1). This is
// intentional, not a contradiction: the written config is the user-facing surface; the empty in-memory
// default means "not set" which resolves to off everywhere (FR-R6). Do NOT "fix" this discrepancy — it is
// the design.

// GOTCHA (exactly-one is the gate, not at-least-one): strings.Contains passes for 1 OR 2 occurrences.
// Use `grep -c` (count) on bootstrap.go and `grep -n '^reasoning'` (anchored, uncommented) on the docs to
// PROVE exactly one uncommented line. TestBuildBootstrapConfig_ValidTOML is the runtime backstop.

// GOTCHA (the docs example uses "claude", not "pi"): docs/configuration.md:78 shows `provider = "claude"`
// (a readable single-backend example). The bootstrap's actual output for a pi user writes `provider = "pi"`
// with blanked multi-backend models. Both correctly carry the uncommented `reasoning = "off"` line. The
// docs example is illustrative; do NOT try to make it emit "pi".

// GOTCHA (do not touch sibling files): S1 owns config.go/roles.go/roles_test.go (Change A); S2 owns
// decompose/roles_test.go, default_action_test.go, root.go, pkg/stagecoach, docs/cli.md. This task owns
// ONLY bootstrap.go + bootstrap_test.go + docs/configuration.md. Editing a sibling site is scope creep
// and risks conflicting with a parallel/landed subtask.
```

## Implementation Blueprint

### Data models and structure

No data-model change. The `Config.Reasoning` field (`*string`/`string` in config.go) already exists; the
bootstrap simply writes the literal `"off"` into the generated TOML's `[defaults]` for discoverability.
No struct/type/Resolve changes (those are S1's, already complete).

### Implementation Tasks (ordered — verify-then-(maybe)-fix)

```yaml
Task 1: VERIFY bootstrap.go site (a) — exactly-one uncommented reasoning line
  - RUN: grep -n 'reasoning = \\"off\\"' internal/config/bootstrap.go
  - EXPECT: exactly ONE match at ~line 127, inside buildBootstrapConfig's [defaults] writer, reading:
        b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below\n")
  - RUN: grep -c 'reasoning = \\"off\\"   # off|low|medium|high; off by default for every role (FR-R6)' internal/config/bootstrap.go
  - EXPECT: 1   (a 0 means it drifted/missing → apply the ONE insert; a 2 means a DUPLICATE was added → DELETE the duplicate)
  - IF ALREADY 1 (it is at HEAD): NO EDIT. Move on.

Task 2: VERIFY bootstrap_test.go site (b) — the assertion present
  - RUN: grep -n 'missing uncommented reasoning' internal/config/bootstrap_test.go
  - EXPECT: exactly ONE match inside TestBuildBootstrapConfig_Pi, immediately after the `provider = "pi"` check:
        if !strings.Contains(content, `reasoning = "off"`) {
            t.Error("missing uncommented reasoning = \"off\" in [defaults]")
        }
  - IF PRESENT (it is at HEAD): NO EDIT. Move on.
  - IF ABSENT (drifted): add exactly that assertion block after the provider check (no duplicate).

Task 3: VERIFY docs/configuration.md site (c) — reasoning uncommented
  - RUN: grep -n '^reasoning = "off"' docs/configuration.md
  - EXPECT: exactly ONE match at ~line 80 (anchored to column 1 → UNcommented), reading:
        reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)
  - RUN (negative — should be ZERO commented reasoning in [defaults]): grep -n '^# reasoning' docs/configuration.md
  - EXPECT: zero matches (if a `# reasoning = "off"` line exists in the [defaults] example, it is stale — uncomment it).
  - IF ALREADY UNCOMMENTED (it is at HEAD): NO EDIT. Move on.

Task 4: GATE — the duplicate-detector + full suite
  - RUN: go test -race ./internal/config/ -run TestBuildBootstrapConfig   # incl. ValidTOML (duplicate-detector)
  - EXPECT: PASS. If TestBuildBootstrapConfig_ValidTOML fails with a duplicate-key error, a duplicate
           reasoning line was introduced — DELETE it (Task 1's `grep -c` should have caught it first).
  - RUN: go build ./... ; go vet ./... ; gofmt -l .
  - EXPECT: clean / empty.
  - RUN: go test -race ./...      # whole repo green
  - EXPECT: all packages PASS.

Task 5: SCOPE-CHECK — only the three sites touched (ideally zero)
  - RUN: git diff --stat -- internal/config/bootstrap.go internal/config/bootstrap_test.go docs/configuration.md
  - EXPECT at HEAD: EMPTY (no edits were needed). If a surgical edit was applied (a site had drifted),
           only the one drifted file appears, with a minimal diff (no duplicate lines).
  - RUN: git diff --stat -- internal/config/config.go internal/config/roles.go internal/decompose/ internal/cmd/ pkg/ docs/cli.md internal/ui/verbose_test.go
  - EXPECT: EMPTY (sibling sites are S1/S2's territory — untouched by this task).
```

### Implementation Patterns & Key Details

```go
// === The live target state of all three sites (VERIFY — do not re-insert) ===

// bootstrap.go:127 (inside buildBootstrapConfig, [defaults] writer) — ALREADY PRESENT:
b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below\n")

// bootstrap_test.go (TestBuildBootstrapConfig_Pi, after the provider check) — ALREADY PRESENT:
if !strings.Contains(content, `reasoning = "off"`) {
    t.Error("missing uncommented reasoning = \"off\" in [defaults]")
}

// docs/configuration.md:80 (Populated config example, [defaults]) — ALREADY PRESENT & UNCOMMENTED:
reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)

// === Why exactly-one matters (the duplicate-key trap) ===
// A SECOND `reasoning = "off"` in [defaults] makes the generated TOML invalid:
//   go-toml/v2 → "toml: key 'reasoning' already exists"
// strings.Contains (TestBuildBootstrapConfig_Pi) cannot detect this (1 vs 2 both "contain").
// TestBuildBootstrapConfig_ValidTOML (unmarshals 6 cases) is the runtime backstop; `grep -c` is the
// pre-flight check. ALWAYS verify count == 1 before declaring done.
```

### Integration Points

```yaml
BOOTSTRAP (internal/config/bootstrap.go):
  - [defaults] writer: emits exactly-one uncommented `reasoning = "off"` (FR-B1 discoverability)
  - NO change to provider/model/role/generation emission

TESTS (internal/config/bootstrap_test.go):
  - TestBuildBootstrapConfig_Pi: asserts reasoning = "off" present (uncommented)
  - TestBuildBootstrapConfig_ValidTOML: the duplicate-detector (unmarshals 6 cases) — MUST stay green

DOCS (docs/configuration.md):
  - Populated config example: reasoning uncommented with FR-R6 comment (matches what config init emits)

CONSUMED (S1's output — already landed):
  - Config.Reasoning / ResolveRoleModel: off ("") for every role when unset (Change A)

GATE: go test -race ./... → GREEN ; grep -c reasoning in bootstrap.go → 1 ; docs reasoning uncommented

NO-TOUCH (explicitly — owned by sibling subtasks):
  - internal/config/config.go, roles.go, roles_test.go   # S1 (Change A — verified complete)
  - internal/decompose/*, internal/cmd/*, pkg/stagecoach/*, docs/cli.md   # S2 (downstream surfaces)
  - internal/ui/verbose_test.go   # the reasoningSuffix FORMATTER test (S2 Finding 3 — never touch)
  - README.md + other docs        # P1.M1.T3.S1 (stale-reference sweep)
  - providers/*.toml              # reasoning_levels tables unchanged
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks, NOT this one):
  - P1.M1.T3.S1: verify README.md + remaining docs/ have no stale reasoning-default references
```

## Validation Loop

### Level 1: The Three-Site Verification (target-state check)

```bash
cd /home/dustin/projects/stagecoach

# (1) bootstrap.go: EXACTLY ONE uncommented reasoning line with the contract comment
grep -c 'reasoning = \\"off\\"   # off|low|medium|high; off by default for every role (FR-R6)' internal/config/bootstrap.go
# Expected: 1   (0 = missing → insert once; 2 = duplicate → delete one)

# (2) bootstrap_test.go: the assertion present (exactly once)
grep -c 'missing uncommented reasoning' internal/config/bootstrap_test.go
# Expected: 1

# (3) docs/configuration.md: reasoning UNCOMMENTED (column-1 anchored) in the [defaults] example
grep -n '^reasoning = "off"' docs/configuration.md
# Expected: 80:reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)
grep -c '^# reasoning' docs/configuration.md
# Expected: 0   (no stale commented reasoning line in the example)
```

### Level 2: The Duplicate-Detector + Component Tests

```bash
cd /home/dustin/projects/stagecoach

# The ValidTOML test is the runtime duplicate-key detector
go test -race ./internal/config/ -v -run TestBuildBootstrapConfig
# Expected: ALL PASS — incl. TestBuildBootstrapConfig_Pi (asserts reasoning present) AND
#           TestBuildBootstrapConfig_ValidTOML (rejects a duplicate reasoning key if one existed).

# Full config suite
go test -race ./internal/config/
# Expected: ok
```

### Level 3: The Exhaustive Oracle (whole repo)

```bash
cd /home/dustin/projects/stagecoach

go build ./...           # Expected: exit 0
go vet ./...             # Expected: exit 0
gofmt -l .               # Expected: empty
go test -race ./...      # Expected: ALL packages green (14 packages ok per system_context §1)
```

### Level 4: Scope-Boundary + End-to-End Emission Check

```bash
cd /home/dustin/projects/stagecoach

# (A) End-to-end: a real `config init` emission contains exactly-one uncommented reasoning under [defaults]
#     (mirrors what TestBuildBootstrapConfig_Pi asserts, but via the public generator).
cat > /tmp/sh_emit_check.go <<'EOF'
package main
import ("fmt";"strings";"github.com/dustin/stagecoach/internal/config")
func main() {
  content := config.GenerateBootstrapConfig("pi")
  // exactly-one reasoning = "off" line, uncommented
  lines := strings.Split(content, "\n")
  n := 0
  for _, l := range lines { if strings.HasPrefix(l, "reasoning = \"off\"") { n++ } }
  fmt.Printf("uncommented reasoning lines = %d\n", n)
  if n != 1 { fmt.Println("FAIL: want exactly 1"); return }
  fmt.Println("PASS: exactly one uncommented reasoning = \"off\" in [defaults]")
}
EOF
go run /tmp/sh_emit_check.go && rm -f /tmp/sh_emit_check.go
# Expected: uncommented reasoning lines = 1 ; PASS

# (B) Scope: ideally zero edits at HEAD (all three sites target-state); sibling files untouched
git diff --stat -- internal/config/bootstrap.go internal/config/bootstrap_test.go docs/configuration.md
# Expected at HEAD: EMPTY (no edits needed). (If a site drifted, only that one file appears, minimal diff.)

git diff --stat -- internal/config/config.go internal/config/roles.go internal/decompose/ internal/cmd/ pkg/ docs/cli.md internal/ui/verbose_test.go
# Expected: EMPTY (sibling territories untouched)
```

## Final Validation Checklist

### Technical Validation

- [ ] `grep -c 'reasoning = \\"off\\"   # off|low|medium|high; off by default for every role (FR-R6)' internal/config/bootstrap.go` → **1**.
- [ ] `grep -c 'missing uncommented reasoning' internal/config/bootstrap_test.go` → **1**.
- [ ] `grep -n '^reasoning = "off"' docs/configuration.md` → one match (~:80); `grep -c '^# reasoning'` → **0**.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` → all green.
- [ ] `TestBuildBootstrapConfig_ValidTOML` passes (no duplicate `reasoning` key in `[defaults]`).

### Feature Validation

- [ ] `buildBootstrapConfig` emits EXACTLY ONE uncommented `reasoning = "off"` after the `provider` line.
- [ ] `TestBuildBootstrapConfig_Pi` asserts the content contains `reasoning = "off"` (uncommented).
- [ ] `docs/configuration.md` Populated config example has `reasoning = "off"` UNCOMMENTED with the FR-R6 comment.
- [ ] End-to-end (`GenerateBootstrapConfig("pi")`) yields exactly one uncommented `reasoning = "off"` line.

### Scope Discipline Validation

- [ ] At HEAD, ZERO source edits were required (all three sites already target-state). If a drift was
      found and fixed, only the ONE drifted file was touched, with a minimal diff (no duplicates).
- [ ] `git diff --stat` for the three files is empty (HEAD) OR a single minimal surgical edit.
- [ ] Sibling files UNCHANGED: `config.go`, `roles.go`, `roles_test.go` (S1); `decompose/*`, `cmd/*`,
      `pkg/stagecoach/*`, `docs/cli.md` (S2); `ui/verbose_test.go` (formatter test); README/other docs (T3.S1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] The implementer recognized the work is already complete (no churn / no duplicate insertion).
- [ ] The `grep -c` (count) check is used — not just `strings.Contains`/`grep` (which can't detect a duplicate).
- [ ] The in-memory `""` default vs the bootstrap `"off"` written value is understood as intentional (FR-B1).

---

## Anti-Patterns to Avoid

- ❌ Don't blindly "INSERT" the reasoning line per the contract — it is ALREADY present (bootstrap.go:127).
  A duplicate `reasoning = "off"` in `[defaults]` breaks `TestBuildBootstrapConfig_ValidTOML` (go-toml/v2
  rejects duplicate keys). Verify EXACTLY ONE first (`grep -c` → 1).
- ❌ Don't assume the contract's starting state exists. The contract and system_context §3 describe a
  starting state (reasoning absent/commented) that DOES NOT EXIST at HEAD — commit `9d33b9e` landed all
  three edits. VERIFY the live state; do not re-apply committed work.
- ❌ Don't add a second assertion to `bootstrap_test.go` — `TestBuildBootstrapConfig_Pi` already asserts
  `reasoning = "off"` (uncommented). A duplicate assertion is harmless but is churn.
- ❌ Don't "fix" the `""` (in-memory default) vs `"off"` (bootstrap written value) discrepancy — it is the
  design: `""` means "not set" (resolves to off everywhere, FR-R6); the bootstrap writes `"off"` explicitly
  for discoverability (FR-B1). Both are correct.
- ❌ Don't weaken or remove `TestBuildBootstrapConfig_ValidTOML` to tolerate a duplicate key — it is the
  runtime guard against exactly the duplicate-line regression this task must avoid.
- ❌ Don't edit sibling files: `config.go`/`roles.go`/`roles_test.go` (S1), `decompose/*`/`cmd/*`/
  `pkg/stagecoach/*`/`docs/cli.md` (S2), `ui/verbose_test.go` (the formatter test), README/other docs (T3.S1).
- ❌ Don't try to make the `docs/configuration.md` example emit "pi" — it intentionally shows "claude" as a
  readable single-backend example; both correctly carry the uncommented `reasoning = "off"` line.
- ❌ Don't fabricate a before/after diff to justify the subtask. The honest outcome is "100% already
  complete at HEAD (commit 9d33b9e); verified green; zero edits."
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** that the verification passes with zero edits at HEAD.

Rationale: Four independent pieces of evidence confirm this task is already 100% realized: (1) the verbatim
live state of all three sites — `bootstrap.go:127` emits the exact contract string
`reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below`;
`bootstrap_test.go`'s `TestBuildBootstrapConfig_Pi` asserts `reasoning = "off"` (uncommented) right after
the `provider = "pi"` check; `docs/configuration.md:80` has `reasoning = "off"` uncommented with the FR-R6
comment — each byte-for-byte the contract's target; (2) the implementing commit `9d33b9e` ("make reasoning
off by default for all roles") is the most recent commit touching all three files, and the working tree is
CLEAN (committed, not WIP); (3) `grep -c` confirms EXACTLY ONE reasoning line in bootstrap.go (no
duplicate); (4) `go test -race ./internal/config/` is GREEN, including `TestBuildBootstrapConfig_ValidTOML`
(the duplicate-key detector). The contract's described starting state (reasoning absent/commented) simply
does not exist. The PRP therefore redirects the implementer from "perform the three inserts" to "verify the
three sites are target-state, confirm exactly-one (not duplicate), pass the gates," with the duplicate-line
trap and the `""`-vs-`"off"` design callout explained so the implementer does not introduce a regression or
"fix" a non-bug. The residual 0.5 uncertainty is purely the mechanical possibility that a gate reveals
drift (e.g., a partial revert), in which case the PRP prescribes the single surgical edit per drifted site —
but at HEAD no such drift exists.
