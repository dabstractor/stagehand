name: "P1.M2.T2.S2 — Add backup file assertions to TestConfigUpgrade_OlderUpdated and TestConfigUpgrade_V2ToV3Rewrite (Issue 3 / FR-B8)"
description: >
  TEST-ONLY task. Consumes the P1.M2.T2.S1 production fix (`runConfigUpgrade` now calls
  `config.WriteTimestampedBackup(path)` before `os.WriteFile`, creating a `config.toml.bak.<RFC3339-UTC>`
  sibling holding the PRIOR content, ONLY when `changed==true`). This task adds the POSITIVE regression
  assertions that PROVE the backup exists after a real upgrade — cloning the proven `filepath.Glob(
  config.toml.bak.*)` pattern already used by `config init --force` (config_test.go:631-634, 675-678).
  Three insertion sites in ONE file (internal/cmd/config_test.go): (A) TestConfigUpgrade_OlderUpdated —
  glob ≥1 + the backup holds the prior v1 content; (B) TestConfigUpgrade_V2ToV3Rewrite first run — glob ≥1
  + the backup content equals the local `v2` var exactly; (C) TestConfigUpgrade_V2ToV3Rewrite end — exactly
  ONE backup survives the idempotent re-run (the `!changed` gate creates no 2nd backup). NO new import
  (`path/filepath` already at config_test.go:8; `os`/`strings` in scope). NO new file. NO docs. The
  assertions PASS with the S1 fix and FAIL without it (no backup → len(matches)==0). Verify
  TestConfigUpgrade_AlreadyCurrent + TestConfigUpgrade_InertFile_NoOp stay green (they return early before
  the backup block → no spurious backup). Run `go test ./internal/cmd/ -v -run TestConfigUpgrade`.

---

## Goal

**Feature Goal**: Lock in the FR-B8 reversible-write guarantee for `config upgrade` with positive test
assertions: after a real (changed==true) upgrade, a timestamped `.bak.*` backup of the prior config MUST
exist alongside the upgraded file. These assertions are the regression net for Issue 3 — they fail without
the S1 production fix and pass with it.

**Deliverable** (3 insertion sites in ONE file — `internal/cmd/config_test.go`; no new files/imports/docs):
1. **`TestConfigUpgrade_OlderUpdated`** (config_test.go:1137) — after the content assertions, add a glob
   `filepath.Join(globalDir, "config.toml.bak.*")` → `len(matches) == 0 ⇒ t.Errorf(...)` check, plus a
   content check that the backup holds the prior v1 content (contains `config_version = 1`, not `= 3`).
2. **`TestConfigUpgrade_V2ToV3Rewrite`** (config_test.go:1423) — after the FIRST run's content assertions,
   add the same glob → ≥1 check, plus an exact-equality check that the backup content equals the local `v2`
   var.
3. **`TestConfigUpgrade_V2ToV3Rewrite`** end — after the idempotent 2nd run, assert exactly ONE backup
   survives (`len(matches2) != 1 ⇒ t.Errorf(...)`) — proving the `!changed` gate creates no spurious 2nd
   backup.

**Success Definition**:
- `go test ./internal/cmd/ -v -run TestConfigUpgrade` is GREEN with the S1 fix applied (OlderUpdated +
  V2ToV3Rewrite assert the backup; AlreadyCurrent + InertFile_NoOp + Idempotent + MalformedTOML unchanged).
- The new assertions FAIL if the S1 fix is reverted (no `.bak.*` file → `len(matches) == 0` → `t.Errorf`).
  (This is verified by reasoning / a temporary revert; it is not a separate committed test.)
- The 2nd-run assertion in V2ToV3Rewrite confirms exactly 1 backup after the idempotent re-run (no
  spurious 2nd backup from the `!changed` gate).
- `gofmt -l internal/cmd/config_test.go` empty; `make test` (race) green; `make lint` clean.
- NO new import; NO new file; NO production-code change; NO docs change.

## User Persona (if applicable)

**Target User**: The maintainer / contributor who needs confidence that `config upgrade` honors FR-B8 and
won't silently regress (Issue 3).

**Use Case**: A future change to `runConfigUpgrade` that accidentally removes or re-orders the backup call
trips these assertions in CI before shipping.

**User Journey**: contributor edits `runConfigUpgrade` → `go test ./internal/cmd/ -run TestConfigUpgrade` →
the OlderUpdated/V2ToV3Rewrite backup assertion fails loudly ("no timestamped backup created ... (FR-B8)")
→ the regression is caught at test time, not in a user's lost config.

**Pain Points Addressed**: Issue 3 had NO backup assertion on the upgrade path — the S1 fix could have been
silently regressed. This task closes that gap with a deterministic, failing-without-the-fix test.

## Why

- **FR-B8 / Issue 3 (Major)**: "Every config-writing command leaves a timestamped backup of the prior file."
  `config init --force` was already tested for this (config_test.go:631-634, 675-678); `config upgrade` was
  the lone violator AND had no test. S1 fixed the production code; this task fixes the test coverage so the
  fix is protected.
- **Fails-without-the-fix proof**: the assertions exist precisely to make a regression of Issue 3 a CI
  failure. Without S1's `WriteTimestampedBackup` call, no `.bak.*` file is created and `len(matches)==0`.
- **Bounded, no-conflict scope**: 3 insertions in one test file. The parallel sibling (S1) edits
  `internal/cmd/config.go` (production); this task edits `internal/cmd/config_test.go` (tests) — no overlap.
  `path/filepath` is already imported; no new helper needed (clone the existing glob pattern).

## What

**User-visible behavior**: None (test-only).

**Technical change**: 3 `filepath.Glob`-based assertion blocks inserted into 2 existing test functions in
`internal/cmd/config_test.go`. See the Implementation Blueprint for verbatim before/after + exact anchors.

### Success Criteria
- [ ] `TestConfigUpgrade_OlderUpdated` asserts `filepath.Glob(globalDir/config.toml.bak.*)` has ≥1 match
      after a real v1→v3 upgrade, and the backup holds the prior v1 content.
- [ ] `TestConfigUpgrade_V2ToV3Rewrite` asserts ≥1 backup after the first (v2→v3) run, with content equal
      to the local `v2` var exactly.
- [ ] `TestConfigUpgrade_V2ToV3Rewrite` asserts exactly 1 backup after the idempotent 2nd run (no spurious
      2nd backup — the `!changed` gate).
- [ ] `TestConfigUpgrade_AlreadyCurrent` and `TestConfigUpgrade_InertFile_NoOp` still PASS unchanged (they
      return early before the backup block → no backup; verified by `go test -run TestConfigUpgrade`).
- [ ] The assertions FAIL without the S1 fix (no `.bak.*` → `len(matches)==0`).
- [ ] `go test ./internal/cmd/ -v -run TestConfigUpgrade` GREEN (with S1 fix).
- [ ] `gofmt -l internal/cmd/config_test.go` empty; `make test` (race) green; `make lint` clean.
- [ ] NO new import; NO new file; NO production-code change; NO docs change.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact proven pattern to clone (config_test.go:631-634, with the verified `filepath` import at
:8), the 3 exact insertion sites with verbatim anchor text (OlderUpdated end; V2ToV3Rewrite first-run gap;
V2ToV3Rewrite end), the path-resolution proof (`setupNoRepo` sets XDG_CONFIG_HOME → `globalDir` =
`$XDG/stagecoach` = the glob root), the proof that `writeConfigFile` writes body verbatim (so exact-content
assertions are valid), the reason the two early-return tests need no edit (they gate before the backup block),
and the "fails-without-the-fix" rationale. No new imports, no new helpers.

### Documentation & References

```yaml
# MUST READ — the consolidated findings (verbatim anchors + path-resolution proof + the fails-without-fix rationale)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T2S2/research/findings.md
  why: "§1 the pattern to clone; §2 why globalDir is the glob root (XDG resolution); §3 the 3 exact insertion
        sites with anchor text; §4 why AlreadyCurrent + InertFile_NoOp need no edit; §5 the fails-without-fix
        proof; §6 scope fence."
  critical: "§3 + §5: the assertions are placed AFTER Execute+content checks so the upgrade has run; the glob
             root is globalDir (NOT config.GlobalConfigPath()'s dir string — use the same globalDir the test
             already has). filepath is already imported — do NOT add it."

# MUST READ — the production fix this task consumes (TREAT AS A CONTRACT; lands in parallel as P1.M2.T2.S1)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T2S1/PRP.md
  why: "S1 inserts `config.WriteTimestampedBackup(path)` into runConfigUpgrade AFTER the `if !changed` gate
        and BEFORE os.WriteFile. So a real upgrade (changed==true) creates exactly one .bak.<ts>; a no-op run
        (changed==false) creates NONE. This task's assertions encode that contract."
  critical: "Do NOT edit internal/cmd/config.go here — that is S1's scope. This task is TEST-ONLY in
             internal/cmd/config_test.go. The backup holds the PRIOR (pre-overwrite) content, by design."

# MUST READ — the bug + the backup primitive's contract
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/config_upgrade_backup.md
  why: "'Existing Tests' section confirms: the backup assertion pattern exists ONLY for config init --force
        (config_test.go:631-634, 675-678); the upgrade tests (OlderUpdated :1137, V2ToV3Rewrite :1423) have
        NO backup assertions. 'WriteTimestampedBackup Signature' confirms the backup filename is
        config.toml.bak.<RFC3339-compact-UTC> and the backup is the PRIOR content."
  critical: "The glob pattern is config.toml.bak.* (the backup is a SIBLING of config.toml in the same dir)."

# MUST EDIT — the file (the ONLY file this task touches); 3 insertion sites
- file: internal/cmd/config_test.go
  why: "TestConfigUpgrade_OlderUpdated (:1137) + TestConfigUpgrade_V2ToV3Rewrite (:1423) are the two tests
        to extend. The config init --force backup pattern at :631-634 / :675-678 is the verbatim template.
        `filepath` is imported at :8; `os`/`strings` in scope."
  pattern: "Clone `matches, _ := filepath.Glob(filepath.Join(globalDir, \"config.toml.bak.*\")); if len(matches) == 0 {
            t.Errorf(...) }`. Add a content check on matches[0] for rigor."
  gotcha: "V2ToV3Rewrite runs Execute TWICE (first = real upgrade; second = idempotent no-op). Place the
           first-run backup assertion AFTER the first run's content checks and BEFORE the `// Re-run` comment;
           place the no-spurious-2nd-backup assertion at the very end. Do NOT glob between the two runs in a
           way that breaks the 2nd run's setup."

# CONTEXT — the assertion pattern source (clone it; do NOT edit these tests)
- file: internal/cmd/config_test.go   # TestConfigInit_Force* backup blocks :631-634, :675-678
  why: "The exact `filepath.Glob(filepath.Join(globalDir, \"config.toml.bak.*\"))` idiom to reuse. Read to
        clone the style (FR-B8 citation in the error message)."

# CONTEXT — the helpers the tests use (so the content assertions are correct)
- file: internal/cmd/root_test.go     # writeConfigFile :61
  why: "writeConfigFile writes the body via os.WriteFile VERBATIM (no transformation), so the backup holds
        exactly the bytes the test passed. This makes exact-content assertions (== v2) valid."
- file: internal/cmd/config_test.go   # setupNoRepo :26
  why: "setupNoRepo sets XDG_CONFIG_HOME=home and returns globalDir=home/stagecoach; config.GlobalConfigPath()
        resolves to globalDir/config.toml. So globbing globalDir/config.toml.bak.* is correct."

# CONTEXT — the backup primitive (READ-ONLY; do NOT modify)
- file: internal/config/backup.go     # WriteTimestampedBackup :18
  why: "Returns (backupPath, nil) on success; the backupPath is path + \".bak.<RFC3339-compact-UTC>\". The
        backup is the PRIOR content (copied before overwrite). Confirms the glob pattern config.toml.bak.*."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config_test.go    # EDIT (this task) — +backup assertions in TestConfigUpgrade_OlderUpdated + _V2ToV3Rewrite
  config.go         # READ-ONLY — S1 (parallel) inserts the production backup call in runConfigUpgrade
internal/config/
  backup.go         # READ-ONLY — WriteTimestampedBackup (the reused primitive; backup filename shape)
# go.mod, Makefile, docs/ — READ-ONLY (no docs change; test-only)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (the ONLY file this task touches):
internal/cmd/config_test.go   # +3 assertion blocks (2 tests): glob ≥1 backup + content + no-spurious-2nd-backup
# (NO new files. NO new imports. NO other modifications.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (NO new import): config_test.go already imports "path/filepath" (line 8), "os", and "strings".
// The glob + ReadFile + Contains assertions reuse only those. Do NOT add an import (it would be unused →
// compile error, or redundant).

// CRITICAL (glob root = globalDir, the test's local var): setupNoRepo returns globalDir = $XDG_CONFIG_HOME/stagecoach,
// and config.GlobalConfigPath() resolves to globalDir/config.toml. The backup is a SIBLING (config.toml.bak.<ts>)
// in the SAME dir. So filepath.Join(globalDir, "config.toml.bak.*") is the correct glob (matches the config init
// --force tests at :631-634). Do NOT glob a different dir.

// CRITICAL (V2ToV3Rewrite runs Execute TWICE): the FIRST run is a real v2→v3 upgrade (changed==true → 1 backup
// created); the SECOND run is idempotent (changed==false → !changed early return → NO 2nd backup). Place the
// positive backup assertion after the FIRST run's content checks (before the "// Re-run" comment), and the
// "exactly 1 backup" assertion at the END (after the 2nd run). Asserting len==1 at the end catches BOTH the
// S1 bug (0 backups) AND a future regression that moves the backup above the !changed gate (>1 backups).

// CRITICAL (the backup holds PRIOR content, by design): WriteTimestampedBackup copies the file BEFORE the
// overwrite, so matches[0] holds the pre-upgrade bytes. For V2ToV3Rewrite the pre-upgrade content is the local
// var `v2` → exact-equality `string(bak) == v2` is valid (writeConfigFile writes body verbatim). For
// OlderUpdated the pre-upgrade content is the inline "config_version = 1\n..." literal → assert it contains
// "config_version = 1" and NOT "config_version = 3" (proves it is the prior version, no restructuring needed).

// GOTCHA (do NOT edit AlreadyCurrent / InertFile_NoOp): they return early BEFORE the backup block
// (AlreadyCurrent via !changed; InertFile_NoOp via IsInert), so they create NO backup and are behaviorally
// unchanged by S1. Just run them green. An OPTIONAL negative assertion (len(matches)==0) is a nice-to-have
// (Task 5) but NOT required by the contract — the contract says "verify they still pass".

// GOTCHA (timestamp granularity is harmless here): WriteTimestampedBackup is second-granular, but the V2ToV3Rewrite
// 2nd run is a NO-OP (!changed → returns before the backup block), so NO 2nd backup is attempted → no same-second
// filename collision in this test. The end assertion len==1 is stable.
```

## Implementation Blueprint

### Data models and structure

None. No new types, helpers, or fixtures. Three localized assertion insertions in two existing test functions,
cloning an existing pattern.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/config_test.go — TestConfigUpgrade_OlderUpdated: add the backup assertion (Edit A)
  - LOCATE: the end of TestConfigUpgrade_OlderUpdated (~:1166-1172). The last content assertion is
    `if !strings.Contains(content, "max_md_lines = 7") { t.Error("max_md_lines = 7 not preserved") }` followed
    by the function's closing `}`.
  - OLD (the anchor — the max_md_lines assertion + closing brace):
        \tif !strings.Contains(content, "max_md_lines = 7") {
        \t\tt.Error("max_md_lines = 7 not preserved")
        \t}
        }
  - NEW (insert the backup block before the closing brace):
        \tif !strings.Contains(content, "max_md_lines = 7") {
        \t\tt.Error("max_md_lines = 7 not preserved")
        \t}

        \t// FR-B8: a timestamped backup of the prior (v1) config must exist alongside the upgraded file.
        \tmatches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
        \tif len(matches) == 0 {
        \t\tt.Errorf("no timestamped backup created after config upgrade (FR-B8)")
        \t} else {
        \t\tbak, _ := os.ReadFile(matches[0])
        \t\tif !strings.Contains(string(bak), "config_version = 1") {
        \t\t\tt.Errorf("backup does not hold prior (v1) content; got:\\n%s", bak)
        \t\t}
        \t}
        }
  - ANCHOR UNIQUENESS: the `max_md_lines = 7 not preserved` string is unique in the file.
  - NOTE: the `else` content-check is the contract's "optionally also assert the backup file content equals
    the pre-upgrade content" — kept cheap (contains v1 marker). Drop the `else` block if a minimal diff is
    preferred; the `len(matches)==0` glob check is the required core.
  - PRESERVE: every existing assertion in the test; the writeConfigFile setup; the Execute call.

Task 2: EDIT internal/cmd/config_test.go — TestConfigUpgrade_V2ToV3Rewrite: first-run backup assertion (Edit B)
  - LOCATE: the first run's LAST content assertion + the `// Re-run` comment (~:1455-1459):
        \tif !strings.Contains(upgraded, "config_version = 3") {
        \t\tt.Errorf("on-disk config_version not 3:\\n%s", upgraded)
        \t}

        \t// Re-run → no change (idempotent).
  - NEW (insert the backup block between the config_version check and the `// Re-run` comment):
        \tif !strings.Contains(upgraded, "config_version = 3") {
        \t\tt.Errorf("on-disk config_version not 3:\\n%s", upgraded)
        \t}

        \t// FR-B8: a timestamped backup of the prior (v2) config must exist alongside the upgraded file.
        \tmatches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
        \tif len(matches) == 0 {
        \t\tt.Errorf("no timestamped backup created after config upgrade (FR-B8)")
        \t} else {
        \t\tbak, _ := os.ReadFile(matches[0])
        \t\tif string(bak) != v2 {
        \t\t\tt.Errorf("backup does not hold prior (v2) content; got:\\n%s", bak)
        \t\t}
        \t}

        \t// Re-run → no change (idempotent).
  - ANCHOR UNIQUENESS: the `on-disk config_version not 3` error string is unique.
  - EXACT-EQUALITY NOTE: `v2` is the local var built at the top of the test; writeConfigFile writes it
    verbatim, so the backup (created before the overwrite) holds exactly `v2`. This is strictly stronger
    than the OlderUpdated contains-check.

Task 3: EDIT internal/cmd/config_test.go — TestConfigUpgrade_V2ToV3Rewrite: no-spurious-2nd-backup (Edit C)
  - LOCATE: the END of TestConfigUpgrade_V2ToV3Rewrite (~:1500-1503):
        \tdata2, _ := os.ReadFile(globalPath)
        \tif string(data2) != upgraded {
        \t\tt.Errorf("second run changed the file (not idempotent)")
        \t}
        }
  - NEW (insert the no-spurious-2nd-backup assertion before the closing brace):
        \tdata2, _ := os.ReadFile(globalPath)
        \tif string(data2) != upgraded {
        \t\tt.Errorf("second run changed the file (not idempotent)")
        \t}

        \t// FR-B8: the idempotent re-run must NOT create a second backup (changed==false returns before the backup block).
        \tmatches2, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
        \tif len(matches2) != 1 {
        \t\tt.Errorf("expected exactly 1 backup after idempotent re-run, got %d (FR-B8 no-op gate)", len(matches2))
        \t}
        }
  - ANCHOR UNIQUENESS: the `second run changed the file (not idempotent)` string is unique.
  - WHY len==1 (not ==0): the FIRST run created 1 backup; the 2nd run creates 0. Total == 1. This catches
    both "0" (S1 bug: no backup ever created) and ">1" (regression: backup moved above the !changed gate).
  - VARIABLE NAME: use `matches2` (not `matches`) to avoid shadowing/conflicting with the `matches` from Edit B
    in the same function scope.

Task 4: VERIFY — focused + full tests, format, lint
  - go test ./internal/cmd/ -run 'TestConfigUpgrade' -v    # ALL upgrade tests green (with S1 fix)
  - go test ./internal/cmd/ -v                              # broader cmd regression
  - make test                                               # race; full suite
  - gofmt -l internal/cmd/config_test.go                    # empty
  - make lint
  - grep guards (see Validation Loop Level 4)

Task 5 (OPTIONAL strengthening): negative backup assertions on the two early-return tests
  - IF you want to lock in the no-op-gate semantics, add to TestConfigUpgrade_AlreadyCurrent (after its
    byte-identical assertion) and TestConfigUpgrade_InertFile_NoOp (after its no-op assertion):
        \tmatches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
        \tif len(matches) != 0 {
        \t\tt.Errorf("spurious backup created on no-op upgrade (got %d); the !changed/inert gate should return before the backup block", len(matches))
        \t}
  - NOTE: these PASS both with AND without the S1 fix (without the fix, no backup is ever created → len==0).
    So they are NOT part of the "fails-without-the-fix" proof — they guard a DIFFERENT regression class
    (spurious backups from a mis-ordered gate). They are optional; the contract only requires "verify they
    still pass", which Task 4's `go test -run TestConfigUpgrade` already does.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the backup-exists assertion (clone of config_test.go:631-634, config init --force).
matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
if len(matches) == 0 {
    t.Errorf("no timestamped backup created after config upgrade (FR-B8)")
}

// PATTERN: strengthen with a content check (the backup holds the PRIOR, pre-overwrite content).
// V2ToV3Rewrite can use exact equality (v2 is a local var); OlderUpdated uses a contains-marker:
if len(matches) > 0 {
    bak, _ := os.ReadFile(matches[0])
    if string(bak) != v2 {                       // V2ToV3Rewrite (exact)
        t.Errorf("backup does not hold prior (v2) content; got:\n%s", bak)
    }
    // OR for OlderUpdated (contains — no restructuring of the inline literal):
    // if !strings.Contains(string(bak), "config_version = 1") { t.Errorf("backup not prior v1 content") }
}

// PATTERN: the no-spurious-2nd-backup assertion (V2ToV3Rewrite end). The 1st run made 1 backup; the
// idempotent 2nd run (!changed) makes 0. Asserting ==1 (not ==0) catches BOTH the S1 bug and a gate reorder.
matches2, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
if len(matches2) != 1 {
    t.Errorf("expected exactly 1 backup after idempotent re-run, got %d (FR-B8 no-op gate)", len(matches2))
}
```

### Integration Points

```yaml
TEST FILE (internal/cmd/config_test.go):
  - TestConfigUpgrade_OlderUpdated: +glob ≥1 backup + contains-v1 content check (Edit A)
  - TestConfigUpgrade_V2ToV3Rewrite: +glob ≥1 backup + exact-==v2 content check after 1st run (Edit B);
    +exactly-1-backup after idempotent 2nd run (Edit C)
  - [OPTIONAL] TestConfigUpgrade_AlreadyCurrent + TestConfigUpgrade_InertFile_NoOp: +len==0 negative (Edit D, Task 5)

NO database / migration / routes / new types / new imports / new helpers / production-code change / docs change.

DEPENDENCY (treat as a contract; lands in parallel as P1.M2.T2.S1):
  - runConfigUpgrade must call config.WriteTimestampedBackup(path) after the `if !changed` gate and before
    os.WriteFile. WITHOUT it, these new assertions FAIL (len(matches)==0). WITH it, they PASS.

SCOPE FENCES: NO production-code change (S1 owns internal/cmd/config.go); NO change to internal/config/backup.go;
  NO new file; NO new import (filepath/os/strings already in scope); NO docs (FR-B8 already documented);
  NO overlap with parallel sibling S1 (production) or P1.M2.T1.S2 (internal/config/bootstrap_test.go).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (the test file must compile — no new import; filepath/os/strings already in scope).
go build ./...
go vet ./internal/cmd/...
# Expected: clean. A vet/build error means a typo or a missing symbol — there shouldn't be one.

# Format — the edited file must be gofmt-clean.
gofmt -l internal/cmd/config_test.go
# Expected: empty. If listed: gofmt -w internal/cmd/config_test.go.

# Lint.
make lint      # golangci-lint
# Expected: zero errors.

# Scope guard: ONLY internal/cmd/config_test.go changed (test-only; no production change).
git diff --name-only
# Expected: internal/cmd/config_test.go (exactly ONE file). NO internal/cmd/config.go (that is S1).
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The focused upgrade suite — ALL must be green WITH the S1 fix.
go test ./internal/cmd/ -run 'TestConfigUpgrade' -v
# Expected: PASS — OlderUpdated (incl. new backup assertion), V2ToV3Rewrite (incl. both new assertions),
#           AlreadyCurrent (unchanged), Idempotent (unchanged), MalformedTOML (unchanged), InertFile_NoOp
#           (unchanged), ExtraArgs (unchanged), and the pure upgradeConfigVersion sub-tests (unchanged).

# Broader cmd regression (the config init --force backup tests — the pattern source — must stay green).
go test ./internal/cmd/ -v

# Full race suite.
make test
# Expected: green (race detector). This is the master gate.

# FAILS-WITHOUT-THE-FIX check (manual reasoning / optional temporary revert): if you `git stash` the S1
# production change (internal/cmd/config.go) and re-run, the new OlderUpdated + V2ToV3Rewrite assertions
# MUST fail with "no timestamped backup created after config upgrade (FR-B8)". (Do NOT commit this revert —
# it is a verification step only. Restore S1 afterward.) This proves the test is a real regression net.
```

### Level 3: Integration Testing (System Validation)

```bash
# Not applicable — this is a unit-test-only task. The "integration" is `go test ./internal/cmd/ -run
# TestConfigUpgrade -v` (Level 2), which exercises the real Execute → runConfigUpgrade →
# WriteTimestampedBackup path end-to-end against a real temp-dir config file.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: exactly TWO upgrade tests now assert a backup (OlderUpdated + V2ToV3Rewrite).
grep -n 'config.toml.bak.\*' internal/cmd/config_test.go
# Expect: ≥4 hits — the 2 pre-existing config init --force assertions (:631, :676) + the new ones in
#         OlderUpdated + V2ToV3Rewrite (Edit A + Edit B + Edit C). Confirm OlderUpdated and V2ToV3Rewrite
#         each have at least one.

# Guard 2: the new assertions cite FR-B8 (the requirement being tested).
grep -n 'FR-B8' internal/cmd/config_test.go
# Expect: new hits in TestConfigUpgrade_OlderUpdated and TestConfigUpgrade_V2ToV3Rewrite (in addition to the
#         pre-existing config init --force FR-B8 citations).

# Guard 3: NO new import added (filepath was already at :8).
git diff internal/cmd/config_test.go | grep -E '^\+.*"path/filepath"'
# Expect: EMPTY (no new import line). filepath/os/strings were already imported.

# Guard 4: NO production-code change (S1 owns config.go).
git diff --name-only | grep -v '_test.go'
# Expect: EMPTY (this task is test-only). internal/cmd/config.go must NOT appear in THIS task's diff.

# Guard 5: the V2ToV3Rewrite no-spurious-2nd-backup uses `matches2` (no shadow of Edit B's `matches`).
grep -n 'matches2' internal/cmd/config_test.go
# Expect: ≥1 hit in TestConfigUpgrade_V2ToV3Rewrite.

# Guard 6: scope — only one file changed.
git status --porcelain
# Expect: 1 file (internal/cmd/config_test.go). No new files.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l internal/cmd/config_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) green

### Feature Validation
- [ ] `TestConfigUpgrade_OlderUpdated` asserts ≥1 `config.toml.bak.*` after a real v1→v3 upgrade (+ prior v1 content)
- [ ] `TestConfigUpgrade_V2ToV3Rewrite` asserts ≥1 backup after the first v2→v3 run (+ content == `v2` exactly)
- [ ] `TestConfigUpgrade_V2ToV3Rewrite` asserts exactly 1 backup after the idempotent 2nd run (no spurious 2nd backup)
- [ ] `TestConfigUpgrade_AlreadyCurrent` + `TestConfigUpgrade_InertFile_NoOp` still PASS unchanged
- [ ] `go test ./internal/cmd/ -v -run TestConfigUpgrade` GREEN (with the S1 fix)
- [ ] (Verification) the new assertions FAIL without the S1 fix (no `.bak.*` → `len(matches)==0`)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/config_test.go` modified (1 file)
- [ ] NO production-code change (`internal/cmd/config.go` is S1's scope; not in this diff)
- [ ] NO new file; NO new import (`path/filepath` already at :8; `os`/`strings` in scope)
- [ ] NO change to `internal/config/backup.go`; NO docs change
- [ ] NO overlap with parallel sibling S1 (production) or P1.M2.T1.S2 (`internal/config/bootstrap_test.go`)

### Code Quality & Docs
- [ ] The assertion style clones the existing config init --force backup pattern (config_test.go:631-634)
- [ ] Error messages cite FR-B8 (consistency with the existing backup assertions)
- [ ] The V2ToV3Rewrite 2nd-backup check uses a distinct var name (`matches2`) to avoid shadowing

---

## Anti-Patterns to Avoid

- ❌ Don't add a new import. `path/filepath` is already imported at config_test.go:8; `os` and `strings` are
  in scope throughout the file. Adding `"path/filepath"` again is a compile error (duplicate/unused).
- ❌ Don't glob the wrong directory. The backup is a SIBLING of `config.toml` in `globalDir` (=
  `$XDG_CONFIG_HOME/stagecoach`, returned by `setupNoRepo`). Use `filepath.Join(globalDir, "config.toml.bak.*")`
  — the SAME root the config init --force tests use. Globbing `config.GlobalConfigPath()`'s parent or a
  hardcoded path is wrong.
- ❌ Don't assert `len(matches) == 0` for the V2ToV3Rewrite END check. The FIRST run created 1 backup; the
  end-of-test count is 1, not 0. Assert `len(matches2) == 1` (catches both the S1 bug AND a gate-reorder
  regression). The `== 0` negative assertion belongs ONLY on the early-return tests (AlreadyCurrent /
  InertFile_NoOp) — and only as an optional Task 5 strengthening.
- ❌ Don't shadow `matches` in V2ToV3Rewrite. Edit B declares `matches`; Edit C (same function, later scope)
  must use a different name (`matches2`). Re-declaring `matches` is a compile error (`matches redeclared`).
- ❌ Don't edit `internal/cmd/config.go` or any production file. This is a TEST-ONLY task; the production fix
  is S1 (parallel sibling). Editing config.go here = scope creep and a merge conflict with S1.
- ❌ Don't restructure the existing tests. Insert the assertions at the verified anchor points (after the
  content checks / before the closing brace). Changing the setup (e.g., extracting the pre-upgrade content
  into a named var for OlderUpdated) is fine IF minimal, but the inline-literal contains-check avoids it
  entirely — prefer the smallest diff.
- ❌ Don't add the backup assertions to `TestConfigUpgrade_AlreadyCurrent` or `TestConfigUpgrade_InertFile_NoOp`
  as POSITIVE assertions. They return early before the backup block and create NO backup — a positive
  `len > 0` assertion would FAIL. (An optional NEGATIVE `len == 0` assertion is fine — Task 5.)
- ❌ Don't insert the V2ToV3Rewrite first-run assertion in a place that breaks the 2nd-run setup. Place it
  AFTER the first run's content checks and BEFORE the `// Re-run → no change` comment (the `out.Reset()` /
  `resetFlags` block must stay intact between the two runs).
- ❌ Don't forget the `v2` exact-equality in V2ToV3Rewrite is valid ONLY because `writeConfigFile` writes the
  body verbatim and the backup is taken before the overwrite. If you doubt this, use the contains-check form
  instead (contains `config_version = 2`, not `= 3`).
- ❌ Don't commit a revert of S1 to "prove" the fails-without-fix behavior. That verification is a temporary
  `git stash`/revert you restore immediately — it is NOT part of the deliverable. The committed state is
  S1-applied + these assertions, all green.

---

## Confidence Score: 10/10

This is a test-only task with a proven pattern to clone (config_test.go:631-634, used twice for config init
--force), three exact insertion sites with verbatim anchor text (OlderUpdated end; V2ToV3Rewrite first-run
gap; V2ToV3Rewrite end), the verified facts that `filepath` is already imported (:8), `globalDir` is the
correct glob root (setupNoRepo + globalConfigPath resolution), `writeConfigFile` writes body verbatim (so
exact-content assertions are valid), and the two early-return tests need no edit (they gate before the
backup block). The "fails-without-the-fix" contract is structurally guaranteed: without S1's
`WriteTimestampedBackup` call, no `.bak.*` file exists and `len(matches)==0` fires `t.Errorf`. No new imports,
no new helpers, no production change, no docs, no overlap with the parallel sibling. One-pass success is
essentially guaranteed; the only judgment call (exact-`v2`-equality vs contains-check) is resolved with the
exact form for V2ToV3Rewrite (where `v2` is a local var) and the contains form for OlderUpdated (inline
literal, minimal diff) — both spelled out verbatim.
