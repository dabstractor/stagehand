---
name: "P1.M1.T2.S1 — Rename all Go identifiers containing Stagehand/stagehand"
description: |
  Mechanical rename of 8 Go identifiers/const-values containing "stagehand" → "stagecoach" across their
  declarations AND all reference sites (including tests). 5 identifier-name renames:
  `StagehandIgnoreFile`→`StagecoachIgnoreFile`, `LoadStagehandIgnore`→`LoadStagecoachIgnore`,
  `StatusStagehand`→`StatusStagecoach`, `stagehandAliasValue`→`stagecoachAliasValue`,
  `buildStagehand`→`buildStagecoach` (+ vars). 3 const-value changes: `defaultAliasName`="stagehand"→
  "stagecoach", `lazygitMarker`="stagehand-integration"→"stagecoach-integration", `Marker`="# stagehand..."
  →"# stagecoach...". Plus the const values of `StagehandIgnoreFile` (".stagehandignore"→
  ".stagecoachignore") and `stagehandAliasValue` ("!stagehand"→"!stagecoach"). ~11 files, all mechanical
  sed replacements. `stagehandFlagUsages` is S2's territory. Env vars/config paths are M2's. Baseline GREEN.
---

## Goal

**Feature Goal**: Rename every Go identifier and const value containing "stagehand" (case-insensitive) to
"stagecoach" in the 8 specific items listed by the contract, across their declarations AND all reference
sites — so that `go build ./...` + `go vet ./...` pass with zero remaining identifier/const-value
references to "stagehand" in production .go files.

**Deliverable**: Mechanical sed-based renames across ~11 files:
- 5 identifier-name renames (declaration + all refs)
- 5 const-value changes (3 standalone + 2 paired with identifier renames)
- ~15 test-assertion string updates (lazygit marker + hook marker + alias name)
- Related variable renames (`buildStagehandOnce`, `buildStagehandPath`, `stagehandBin`)

**Success Definition**: `go build ./...` + `go vet ./...` clean; `go test ./...` green; grep for each old
identifier in production .go files returns zero; the renamed identifiers/values compile and tests pass.

## User Persona

**Target User**: The project rename effort — every downstream subtask (M2 config surface, M3 build system,
M4 docs) depends on the Go identifiers being renamed first.

## Why

- **The project is being renamed from "stagehand" to "stagecoach".** PRD §h2.30: "All references to
  'stagehand' must be replaced with 'stagecoach'." The Go structural rename (module path, directories,
  packages) is done (S1-S4); this task completes the identifier-level rename.
- **The architecture docs prescribe the exact 8 items** (rename_surface_map.md §2.5 + critical_findings.md
  F4). Each has a verified declaration site + reference inventory.
- **Mechanical, low-risk.** Each rename is a global sed across .go files followed by a compile check.
  No logic change, no behavioral change — pure identifier/value substitution.

## What

8 identifier/value renames across ~11 files, each done via sed (declaration + all references). The
`stagehandFlagUsages` cobra template func is NOT this task (S2). Env var literals (`STAGEHAND_*`) are
NOT this task (M2). Config path strings (`.stagehand.toml`, `.stagehandignore` in root.go/verbose.go
as raw literals — NOT via the const) are NOT this task (M2). User-facing error/status strings in
cmd/hook.go are NOT this task (M2.T3).

### Success Criteria

- [ ] All 8 listed identifiers/values are renamed (declaration + all references, including tests).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./...` green.
- [ ] Grep for each old identifier in production .go files (excl. plan/) returns zero.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP lists every old→new pair with its declaration file:line, every reference
site (grep-verified), the scope boundary (what is NOT this task), and the sed commands to execute.

### Documentation & References

```yaml
# MUST READ — the rename surface map + critical findings
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  why: "§2.5 lists the exclusion-file const + function names with exact file:line. §2.5 also lists root.go:164 + verbose.go:101 as string-literal references to .stagehandignore (those are M2's — NOT via the const)."
  critical: "§2.5: `const StagehandIgnoreFile = \".stagehandignore\"` → `StagecoachIgnoreFile = \".stagecoachignore\"` — BOTH name AND value change."

- docfile: plan/012_963e3918ec08/architecture/critical_findings.md
  why: "F4 lists the key identifiers that contain 'stagehand' but are NOT just the env prefix: StagehandIgnoreFile, StatusStagehand, stagehandAliasValue, defaultAliasName, lazygitMarker, Marker, stagehandFlagUsages. F6: the hook Marker rename means already-installed hooks with the old marker are treated as 'foreign' — acceptable (no public releases)."
  critical: "F4 explicitly names `stagehandFlagUsages` (cobra template func) — that is S2 (P1.M1.T2.S2), NOT this task."

- docfile: plan/012_963e3918ec08/P1M1T1S4/PRP.md
  why: "The prior sibling (binary build path verification). Explicitly defers identifiers to T2: 'IDENTIFIERS (buildStagehand, stagehandBin, stagehandOnce, runStagehand, …) → P1.M1.T2'. Confirms no overlap."
  critical: "S4 does NOT rename any identifiers. S4's scope boundary lists exactly which stagehand residues belong to T2 vs M2/M3/M4."

- docfile: plan/012_963e3918ec08/P1M1T2S1/research/identifier_rename_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-07): all 8 identifiers' declaration sites + reference inventories (grep-confirmed); the 11 files to touch; the scope boundary (what is NOT this task); decisions D1–D7. READ THIS FIRST."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/   # module already renamed (S1-S3)
├── internal/
│   ├── exclude/exclude.go + exclude_test.go          # Items 1,2
│   ├── hook/hook.go + hook_test.go + script.go       # Items 3,8
│   ├── cmd/hook.go                                    # Item 3 (ref to hook.StatusStagehand)
│   ├── cmd/integrate_gitalias.go + _test.go           # Items 4,6
│   ├── cmd/integrate_lazygit.go + _test.go            # Item 7
│   └── signal/signal_integration_test.go              # Item 5
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/exclude/exclude.go + _test.go             # StagecoachIgnoreFile + LoadStagecoachIgnore
    internal/hook/hook.go + _test.go + script.go       # StatusStagecoach + Marker value
    internal/cmd/hook.go                                # hook.StatusStagecoach ref
    internal/cmd/integrate_gitalias.go + _test.go       # stagecoachAliasValue + defaultAliasName value
    internal/cmd/integrate_lazygit.go + _test.go        # lazygitMarker value + entryTpl + assertions
    internal/signal/signal_integration_test.go           # buildStagecoach + vars
```

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — items 1 and 4 change BOTH the identifier NAME and its VALUE). For StagehandIgnoreFile:
//   const StagehandIgnoreFile = ".stagehandignore" → const StagecoachIgnoreFile = ".stagecoachignore"
// For stagehandAliasValue:
//   stagehandAliasValue = "!stagehand" → stagecoachAliasValue = "!stagecoach"
// The sed must change both the identifier and the value, not just one.

// CRITICAL (G2 — item 7 entryTpl format string). integrate_lazygit.go:28 has:
//   var entryTpl = `- key: '%s' # stagehand-integration`
// This is a SEPARATE string literal containing "stagehand-integration" — NOT via the lazygitMarker const.
// It must ALSO change to "# stagecoach-integration". The sed for the const value will NOT catch this
// unless it also replaces the bare string "stagehand-integration" → "stagecoach-integration".

// CRITICAL (G3 — item 7 test assertions). integrate_lazygit_test.go has ~15 assertions checking for
// "stagehand-integration". These MUST change to "stagecoach-integration" or the tests fail. The sed for
// the bare string "stagehand-integration" → "stagecoach-integration" across ALL .go files handles both
// the entryTpl AND the test assertions.

// CRITICAL (G4 — item 5 related vars). buildStagehand has 3 related identifiers:
//   buildStagehandOnce → buildStagecoachOnce
//   buildStagehandPath → buildStagecoachPath
//   stagehandBin → stagecoachBin
// All must be renamed (they contain "Stagehand"/"stagehand" in the identifier name).

// GOTCHA (G5 — do NOT rename stagehandFlagUsages). That cobra template func is P1.M1.T2.S2's territory.

// GOTCHA (G6 — do NOT rename STAGEHAND_ env var literals or stagehand.* git config keys). Those are
// P1.M2.T1's territory. They are string literals, NOT Go identifiers.

// GOTCHA (G7 — do NOT rename user-facing strings in cmd/hook.go like "Remove the stagehand prepare-commit-msg
// hook"). Those are P1.M2.T3's territory. The ONLY change in cmd/hook.go is `hook.StatusStagehand` →
// `hook.StatusStagecoach` (a Go identifier reference, not a string).

// GOTCHA (G8 — root.go:164 + verbose.go:101 reference ".stagehandignore" as raw string literals, NOT via
// the const. Those are P1.M2.T2's territory. Do NOT change them here.

// GOTCHA (G9 — item 3 StatusStagehand has COMMENT references too: hook.go:22 "stagehand-owned",
// hook.go:46 "StatusStagehand", hook.go:62/91 comments. Rename the identifier in comments for consistency,
// but the success check is on non-comment code.)
```

## Implementation Blueprint

### Data models and structure

No data-model change. Pure identifier/value substitution via sed.

### Implementation Tasks (ordered by dependencies — each is independent)

```yaml
Task 1: RENAME Items 1+2 — exclude package
  - FILES: internal/exclude/exclude.go, internal/exclude/exclude_test.go
  - SED commands (run in repo root):
      sed -i 's/StagehandIgnoreFile/StagecoachIgnoreFile/g' internal/exclude/exclude.go internal/exclude/exclude_test.go
      sed -i 's/LoadStagehandIgnore/LoadStagecoachIgnore/g' internal/exclude/exclude.go internal/exclude/exclude_test.go
      sed -i 's/\.stagehandignore/.stagecoachignore/g' internal/exclude/exclude.go internal/exclude/exclude_test.go
  - The third sed changes the CONST VALUE ".stagehandignore" → ".stagecoachignore" AND any inline string
    references to the filename. (In exclude.go this is the const value + error messages + verbose warnings;
    all via the const or the bare string — all should change.)
  - DO NOT: change root.go:164 or verbose.go:101 (those are M2.T2's bare-string refs, not via the const).
  - VERIFY: grep -rn "StagehandIgnore\|stagehandignore" internal/exclude/ → zero matches.
  - VERIFY: go build ./internal/exclude/

Task 2: RENAME Item 3 — StatusStagehand → StatusStagecoach
  - FILES: internal/hook/hook.go, internal/hook/hook_test.go, internal/cmd/hook.go
  - SED:
      sed -i 's/StatusStagehand/StatusStagecoach/g' internal/hook/hook.go internal/hook/hook_test.go internal/cmd/hook.go
  - VERIFY: grep -rn "StatusStagehand" internal/ → zero matches.
  - VERIFY: go build ./internal/hook/ ./internal/cmd/

Task 3: RENAME Item 8 — Marker value
  - FILE: internal/hook/script.go
  - SED: change the const VALUE (the identifier name `Marker` stays):
      sed -i 's|# stagehand prepare-commit-msg hook v1|# stagecoach prepare-commit-msg hook v1|g' internal/hook/script.go
  - ALSO check hook_test.go for marker-string assertions:
      grep -n "stagehand prepare-commit-msg" internal/hook/hook_test.go
      If found: sed -i 's|# stagehand prepare-commit-msg|# stagecoach prepare-commit-msg|g' internal/hook/hook_test.go
  - VERIFY: grep -rn "stagehand prepare-commit-msg" internal/hook/ → zero matches.

Task 4: RENAME Items 4+6 — gitalias
  - FILES: internal/cmd/integrate_gitalias.go, internal/cmd/integrate_gitalias_test.go
  - SED:
      # Item 4: identifier rename + value change
      sed -i 's/stagehandAliasValue/stagecoachAliasValue/g' internal/cmd/integrate_gitalias.go internal/cmd/integrate_gitalias_test.go
      sed -i 's/"!stagehand"/"!stagecoach"/g' internal/cmd/integrate_gitalias.go
      # Item 6: defaultAliasName value change (name stays)
      sed -i 's/defaultAliasName    = "stagehand"/defaultAliasName    = "stagecoach"/g' internal/cmd/integrate_gitalias.go
  - Check test for alias-name assertions: grep "stagehand" internal/cmd/integrate_gitalias_test.go (if the
    test checks the alias NAME string, it must change to "stagecoach").
  - VERIFY: grep -rn "stagehandAliasValue\|\"!stagehand\"\|= \"stagehand\"" internal/cmd/integrate_gitalias* → zero.
  - VERIFY: go build ./internal/cmd/

Task 5: RENAME Item 7 — lazygitMarker value + entryTpl + test assertions
  - FILES: internal/cmd/integrate_lazygit.go, internal/cmd/integrate_lazygit_test.go
  - SED (the bare string "stagehand-integration" appears in the const value, the entryTpl format string,
    AND ~15 test assertions — one sed handles all):
      sed -i 's/stagehand-integration/stagecoach-integration/g' internal/cmd/integrate_lazygit.go internal/cmd/integrate_lazygit_test.go
  - VERIFY: grep -rn "stagehand-integration" internal/cmd/integrate_lazygit* → zero matches.
  - VERIFY: go test ./internal/cmd/ -run TestLazygit

Task 6: RENAME Item 5 — buildStagehand + vars
  - FILE: internal/signal/signal_integration_test.go
  - SED:
      sed -i 's/buildStagehand/buildStagecoach/g' internal/signal/signal_integration_test.go
      sed -i 's/stagehandBin/stagecoachBin/g' internal/signal/signal_integration_test.go
  - VERIFY: grep -rn "buildStagehand\|stagehandBin" internal/signal/ → zero matches.
  - NOTE: this is a test file; the build covers compilation but the test requires the integration tag.

Task 7: VALIDATE
  - RUN: gofmt -l .            # must be empty (sed preserves formatting)
  - RUN: go build ./...        # all identifiers resolve
  - RUN: go vet ./...          # no warnings
  - RUN: go test ./...         # all tests pass (the renamed identifiers/values are consistent)
  - RUN (grep audit): grep -rni "stagehand" --include="*.go" internal/ pkg/ cmd/ | grep -v "_test.go" | grep -v "^.*//"
    # Expected: the remaining hits are STAGEHAND_ env vars, .stagehand.toml/.stagehandignore literals,
    # user-facing strings, and comments — all owned by M2/M3/M4. ZERO Go identifiers or const values.
```

### Implementation Patterns & Key Details

```bash
# === The sed discipline (each identifier is a single global replace) ===
# 1. sed -i 's/OldIdentifier/NewIdentifier/g' <all files that reference it>
# 2. Verify: grep -rn "OldIdentifier" <scope> → zero matches
# 3. Compile: go build ./... (catches any missed reference)
#
# === Items with BOTH name AND value change (G1) ===
# StagehandIgnoreFile: two seds — one for the identifier, one for the value ".stagehandignore"
# stagehandAliasValue: two seds — one for the identifier, one for the value "!stagehand"
#
# === The lazygitMarker bare-string sed (G2/G3) ===
# The string "stagehand-integration" appears in: the const value, the entryTpl format string, AND ~15
# test assertions. One sed 's/stagehand-integration/stagecoach-integration/g' across both files handles ALL.
```

### Integration Points

```yaml
IDENTIFIER RENAMES (this task):
  - internal/exclude/: StagecoachIgnoreFile + LoadStagecoachIgnore + value ".stagecoachignore"
  - internal/hook/: StatusStagecoach + Marker value "# stagecoach prepare-commit-msg hook v1"
  - internal/cmd/: stagecoachAliasValue + defaultAliasName="stagecoach" + lazygitMarker="stagecoach-integration"
  - internal/signal/: buildStagecoach + buildStagecoachOnce/Path + stagecoachBin

NO-TOUCH (explicitly):
  - stagehandFlagUsages (cobra template func) → P1.M1.T2.S2
  - STAGEHAND_* env var literals → P1.M2.T1
  - stagehand.* git config keys → P1.M2.T1
  - .stagehandignore/.stagehand.toml in root.go/verbose.go (raw strings) → P1.M2.T2
  - User-facing strings ("stagehand prepare-commit-msg hook" in cmd/hook.go) → P1.M2.T3
  - Temp dir prefixes → P1.M2.T3
  - Makefile/.goreleaser/CI → P1.M3
  - docs/README/providers → P1.M4

GATE: go build ./... → OK; go vet ./... clean; go test ./... green; grep audit clean
```

## Validation Loop

### Level 1: Compile + Vet

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .            # Expected: empty
go build ./...         # Expected: exit 0 (all renamed identifiers resolve)
go vet ./...           # Expected: exit 0
```

### Level 2: Tests

```bash
cd /home/dustin/projects/stagecoach

go test ./...          # Expected: ALL green (the renamed identifiers/values are consistent across decl + refs)
```

### Level 3: Grep Audit (zero old identifiers in production code)

```bash
cd /home/dustin/projects/stagecoach

# Each old identifier must be ABSENT from production .go files.
for id in StagehandIgnoreFile LoadStagehandIgnore StatusStagehand stagehandAliasValue buildStagehand; do
  echo "--- $id:"
  grep -rn "$id" --include="*.go" internal/ pkg/ cmd/ && echo "FAIL: $id still present" || echo "OK: absent"
done

# Each old const value must be ABSENT.
for val in ".stagehandignore" '"!stagehand"' 'stagehand-integration' '# stagehand prepare-commit-msg'; do
  echo "--- value $val:"
  grep -rn "$val" --include="*.go" internal/hook/ internal/exclude/ internal/cmd/ && echo "FAIL: still present" || echo "OK: absent"
done
```

### Level 4: Scope Boundary (what REMAINS is M2/M3/M4's territory)

```bash
cd /home/dustin/projects/stagecoach

# Remaining "stagehand" in .go files should ONLY be: env vars (STAGEHAND_), config paths (.stagehand.toml),
# user-facing strings, comments, and stagehandFlagUsages (S2).
grep -rni "stagehand" --include="*.go" internal/ pkg/ cmd/ | grep -v "^.*//.*stagehand" | head -20
# Expected: STAGEHAND_ env vars, .stagehand.toml, .stagehandignore (raw strings in root.go/verbose.go),
# user-facing error strings, and stagehandFlagUsages — ALL owned by M2/S2. ZERO Go identifiers or const values.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` green.

### Feature Validation

- [ ] All 8 listed identifiers/values renamed (declaration + all references, including tests).
- [ ] Grep for each old identifier in production .go files returns zero.
- [ ] Grep for each old const value in production .go files returns zero.
- [ ] `entryTpl` format string + ~15 lazygit test assertions updated.
- [ ] `buildStagehand` related vars (`buildStagehandOnce`, `buildStagehandPath`, `stagehandBin`) renamed.

### Scope Discipline Validation

- [ ] Did NOT rename `stagehandFlagUsages` (S2).
- [ ] Did NOT rename `STAGEHAND_*` env var literals (M2.T1).
- [ ] Did NOT rename `.stagehandignore`/`.stagehand.toml` raw strings in root.go/verbose.go (M2.T2).
- [ ] Did NOT rename user-facing strings in cmd/hook.go (M2.T3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't rename `stagehandFlagUsages` — it's S2's (P1.M1.T2.S2). (gotcha G5)
- ❌ Don't rename `STAGEHAND_*` env var literals or `stagehand.*` git config keys — they're M2.T1's. (G6)
- ❌ Don't rename user-facing strings in cmd/hook.go ("Remove the stagehand...") — they're M2.T3's. The ONLY
  change in cmd/hook.go is `hook.StatusStagehand` → `hook.StatusStagecoach`. (G7)
- ❌ Don't rename `.stagehandignore` in root.go:164 or verbose.go:101 — those are raw string literals, NOT
  via the const. They're M2.T2's. (G8)
- ❌ Don't forget the `entryTpl` format string at integrate_lazygit.go:28 — it contains "stagehand-integration"
  as a bare string, NOT via the `lazygitMarker` const. (G2)
- ❌ Don't forget the ~15 test assertions in integrate_lazygit_test.go — they check for the old marker string. (G3)
- ❌ Don't forget the related vars `buildStagehandOnce`, `buildStagehandPath`, `stagehandBin`. (G4)
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a mechanical sed-based rename of 8 identifiers/values across ~11 files, with every
declaration site and reference inventory grep-verified against the live source, and the exact sed commands
provided. The prior S4 explicitly defers all identifiers to T2 (no conflict). The main risk is the scope
boundary (items 1+4 change BOTH name AND value; the lazygit entryTpl is a bare string not via the const;
~15 test assertions must follow the value change) — all called out as CRITICAL gotchas. The residual
uncertainty (not 10/10) is whether any test assertion on the alias name or hook marker string was missed
in the grep inventory (mitigated by the `go test ./...` gate — a missed assertion fails the test, pointing
at the exact line). The S2/M2/M3/M4 boundaries are cleanly fenced.
