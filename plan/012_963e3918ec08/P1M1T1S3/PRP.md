---
name: "P1.M1.T1.S3 — Rename files and package declarations within renamed directories"
description: |
  Finish the Go package-identity rename inside the S2-renamed directories: `git mv` the two files
  `pkg/stagecoach/stagehand.go → stagecoach.go` and `stagehand_test.go → stagecoach_test.go`, change
  `package stagehand → package stagecoach` in both, and fix the godoc `// Package stagehand` header. ⚠️
  CONTRACT CORRECTION (same pattern as S2): the build is currently GREEN only because the sole importer
  `internal/cmd/default_action.go` references the package via the `stagehand.` qualifier
  (`stagehand.GenerateCommit`/`Options`/`Result`). Renaming the package declaration WILL BREAK those
  references, so the contract's OUTPUT (`go build ./cmd/stagecoach/` must succeed) REQUIRES S3 to also
  update the `stagehand.` → `stagecoach.` qualifier refs in default_action.go (the package-qualifier usages
  are an inseparable consequence of the package rename — they can't be deferred to T2 without a broken
  intermediate build). S2 LANDED (dirs are stagecoach/); S3 operates on the files inside + the sole
  importer's package-identity refs. `cmd/stagecoach/main.go` is `package main` (verified) and needs NO
  edit. NO docs (M4); NO identifiers (T2); NO user-facing strings (P1.M2.T3).
---

## Goal

**Feature Goal**: Complete the `stagecoach` package identity inside the S2-renamed `pkg/stagecoach/`
directory — rename the two `.go` files to `stagecoach.go`/`stagecoach_test.go`, change both `package`
declarations to `stagecoach`, fix the godoc header, and update the sole importer's package-qualifier
references so the build stays green end-to-end.

**Deliverable** (2 git-mv file renames + 3 in-file edits + 1 importer sed):
1. `git mv pkg/stagecoach/stagehand.go pkg/stagecoach/stagecoach.go`
2. `git mv pkg/stagecoach/stagehand_test.go pkg/stagecoach/stagecoach_test.go`
3. In `pkg/stagecoach/stagecoach.go`: `package stagehand` → `package stagecoach` (line 7) + the godoc header `// Package stagehand is Stagehand's …` → `// Package stagecoach is Stagecoach's …` (line 1).
4. In `pkg/stagecoach/stagecoach_test.go`: `package stagehand` → `package stagecoach` (line 1).
5. **CONTRACT CORRECTION (build-required):** `sed -i 's|stagehand\.\([A-Z]\)|stagecoach.\1|g' internal/cmd/default_action.go` — updates the 6 package-qualifier / `pkg/stagehand.X` references across lines 26/194/217/340/371 (the importer's references to the renamed package).
6. Verify `cmd/stagecoach/main.go` is `package main` (it is — no edit).

**Success Definition**: `go build ./cmd/stagecoach/` succeeds (the contract gate); `go build ./...`,
`go vet ./...`, `go test ./...` all green; `git status` shows the 2 file moves as renames (R) + the edited
files; zero `package stagehand` declarations remain; zero `stagehand.<Upper>` qualifier refs remain in
production code. No user-facing strings, identifiers, docs, env vars, or Makefile/.goreleaser/CI touched.

## User Persona

**Target User**: The contributor implementing S4 (binary build-path verification), P1.M1.T2 (Go identifier
rename), and the reviewer confirming the structural Go rename is complete and the tree compiles.

**Use Case**: After S2 renamed the directories, the files inside still carry the old name (`stagehand.go`)
and package (`package stagehand`). S3 finishes the package identity so `import "...pkg/stagecoach"` resolves
to a package literally named `stagecoach`, referenced as `stagecoach.GenerateCommit(...)`.

**Pain Points Addressed**: Closes the package-name/dir-name inconsistency S2 left (files named
`stagehand.go` declaring `package stagehand` inside a `pkg/stagecoach/` directory), and — critically — keeps
the build green through the rename by carrying the importer's qualifier references along with the
declaration (the trap a literal "rename files + package decls only" would fall into).

## Why

- **Completes the Go structural rename.** S1 did the module path + import prefixes; S2 did the directories +
  their 3 import-path shadows. The files and package declarations inside `pkg/stagecoach/` are the last
  structural holdouts of the name `stagehand`. S3 finishes them.
- **Contract correction (necessary, not optional).** The contract LOGIC (c) says "replace `package stagehand`
  with `package stagecoach` in both files," but its OUTPUT #4 says "`go build ./cmd/stagecoach/` must
  succeed." Empirically the build is GREEN today ONLY because `internal/cmd/default_action.go` references the
  package via the `stagehand.` qualifier (`stagehand.GenerateCommit`/`Options`/`Result` at lines 194/340).
  Renaming the declaration without updating those qualifiers produces `undefined: stagehand` and a FAILED
  build — directly contradicting OUTPUT #4. The qualifier usages are an inseparable consequence of a Go
  package rename (you cannot rename a package without renaming its usages), so they belong to S3, not T2
  (identifiers) or P1.M2.T3 (strings). Nobody else fixes them between S3 and a green build.
- **PRD mandate + golint hygiene.** §h2.30: "all references to `stagehand` must be replaced with
  `stagecoach`." The godoc `// Package stagehand` header must match the `package stagecoach` declaration or
  golint/revive/staticcheck flag it — so the header is part of the package-identity rename.
- **Lowest-risk completion.** Two history-preserving `git mv`s + three precise in-file edits + one
  `[A-Z]`-anchored sed on the sole importer. No logic change; the references merely follow the package name.

## What

Two `git mv` file renames, the two `package` declaration edits, the godoc header edit, and one precise sed
on `internal/cmd/default_action.go` (the sole importer) updating the 6 package-qualifier / `pkg/stagehand.X`
references the build depends on. `cmd/stagecoach/main.go` is verified `package main` (no edit).

### Success Criteria

- [ ] `pkg/stagecoach/stagecoach.go` exists; `pkg/stagecoach/stagehand.go` does not.
- [ ] `pkg/stagecoach/stagecoach_test.go` exists; `pkg/stagecoach/stagehand_test.go` does not.
- [ ] `git status` shows the 2 file moves as renames (R), not add+delete.
- [ ] `pkg/stagecoach/stagecoach.go` line 7 declares `package stagecoach`; its godoc header (line 1) reads `// Package stagecoach is Stagecoach's …`.
- [ ] `pkg/stagecoach/stagecoach_test.go` line 1 declares `package stagecoach`.
- [ ] `internal/cmd/default_action.go` reads `stagecoach.GenerateCommit` / `stagecoach.Options` / `stagecoach.Result` (lines 194, 340) and `pkg/stagecoach.X` in the comments (lines 26, 217, 371) — zero `stagehand.<Upper>` refs remain.
- [ ] `cmd/stagecoach/main.go` is `package main` (verified, unchanged).
- [ ] `go build ./cmd/stagecoach/` succeeds (the contract gate).
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all green.
- [ ] ZERO `package stagehand` declarations remain anywhere (grep confirms).
- [ ] ONLY `pkg/stagecoach/{stagecoach,stagecoach_test}.go` (renamed+edited) + `internal/cmd/default_action.go` (M) change. No user-facing strings, identifiers, docs, env/config, Makefile/.goreleaser/CI, go.mod touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current working-tree state (S2 landed; build green; the
sole importer is default_action.go:21), the EXACT 2 `git mv` commands, the EXACT package-decl + godoc-header
edits (with current/target text), the EXACT sed for the importer (with the `[A-Z]` anchor that avoids
`stagehand:`/`"stagehand"`/`stagehand.go`), the complete verified table of all 6 qualifier/path references
(lines 26/194/217/340/371), and the proof of why a "files + decls only" approach fails the build gate. The
contract correction is documented with the build-break reasoning (same pattern S2 established).

### Documentation & References

```yaml
# MUST READ — the contract correction (why decls-only fails the build gate)
- docfile: plan/012_963e3918ec08/P1M1T1S3/research/s3_rename_files_packages.md
  why: "Proves empirically that go build ./... is GREEN today only because default_action.go references the package via the 'stagehand.' qualifier; renaming the declaration breaks lines 194/340 unless the importer is updated. Gives the complete verified table of all 6 refs (lines 26/194/217/340/371) and the precise [A-Z]-anchored sed."
  critical: "The #1 trap is following the contract LOGIC literally ('replace package stagehand in both files, no other code changes'): that leaves the importer's stagehand.GenerateCommit/Options/Result dangling → 'undefined: stagehand' → go build ./cmd/stagecoach/ FAILS (contradicting OUTPUT #4). S3 MUST sed default_action.go's package-qualifier refs."

- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  why: "Layer 1.4 (file renames): pkg/stagehand/stagehand.go → pkg/stagecoach/stagecoach.go. Layer 1.5 (package declarations): stagehand.go:7 + stagehand_test.go:1. Confirms the 2 files + 2 decl sites are the rename targets. (The map does NOT list the importer qualifier refs — that gap is this subtask's contract correction, same as S2 found the map missed the 3 import-path subdir refs.)"
  critical: "Layer 1.4/1.5 are the authority for the file + decl renames. The importer qualifier refs are the necessary completion the map doesn't enumerate."

- docfile: plan/012_963e3918ec08/P1M1T1S2/PRP.md
  why: "S2 LANDED the directory renames (cmd/stagehand→cmd/stagecoach, pkg/stagehand→pkg/stagecoach) + fixed the 3 import-path subdir refs (incl. default_action.go:21 now imports 'pkg/stagecoach'). S3 consumes S2's output: the dirs are stagecoach/, the import path is correct, only the files/decls/qualifiers inside remain. Establishes the CONTRACT CORRECTION pattern S3 follows."
  critical: "S2 is LANDED (verified: pkg/stagecoach/ + cmd/stagecoach/ exist). S3 does NOT re-edit import paths or directories. default_action.go:21 already imports pkg/stagecoach (S2 fixed it); S3 fixes only the QUALIFIER usages (stagehand.→stagecoach.) that reference the package by name."

- docfile: plan/012_963e3918ec08/P1M1T1S1/PRP.md
  why: "S1 LANDED the module path (go.mod = github.com/dustin/stagecoach) + all import-path prefixes. Context for why the import path is already correct; S3 is downstream of S1+S2."

# The files under edit
- file: pkg/stagecoach/stagehand.go → stagecoach.go   # git mv + 2 edits (package decl L7 + godoc header L1)
  why: "The public API package file (34 KB). git mv preserves history; then 'package stagehand'→'package stagecoach' (L7) and the godoc header '// Package stagehand is Stagehand's …'→'// Package stagecoach is Stagecoach's …' (L1). The file BODY has other stagehand/Stagehand mentions (comments/strings/identifiers) — those are T2/P1.M2/P1.M4, NOT S3."
  pattern: "git mv for history; edit the package clause + its attached godoc header only. The exported symbols (GenerateCommit, Options, Result, Decompose, ...) keep their NAMES — only the package name changes."
  gotcha: "Do NOT sweep the file body for 'stagehand'/'Stagehand' — that's T2 (identifiers) / P1.M2 (strings) / P1.M4 (docs). S3 touches ONLY the package clause (L7) + the godoc header line (L1)."

- file: pkg/stagecoach/stagehand_test.go → stagecoach_test.go   # git mv + 1 edit (package decl L1)
  why: "The internal test file (68 KB, 'package stagehand'). It uses BARE names (no 'stagehand.' qualifier — verified), so changing the package decl to 'stagecoach' needs NO body change (bare-name resolution is package-name-independent). git mv + change L1."
  pattern: "git mv + 'package stagehand'→'package stagecoach' on line 1. Done."
  gotcha: "The test file is an INTERNAL test ('package stagehand', not 'package stagehand_test') — it references exported symbols bare (GenerateCommit, not stagehand.GenerateCommit). So no qualifier fix is needed inside it. Verified: zero 'stagehand.<Upper>' refs in the test file."

- file: internal/cmd/default_action.go   # sed (CONTRACT CORRECTION — the importer's qualifier refs)
  why: "The SOLE importer of pkg/stagecoach (line 21). References the package via 'stagehand.' qualifier: stagehand.GenerateCommit + stagehand.Options (L194, CODE), stagehand.Result (L340, CODE), and pkg/stagehand.{GenerateCommit,Result,Decompose} in comments (L26/L217/L371). The CODE refs break the build when the package is renamed; the comment refs describe the renamed package. All 5 lines are the package-identity surface S3 owns."
  pattern: "sed -i 's|stagehand\\.\\([A-Z]\\)|stagecoach.\\1|g' internal/cmd/default_action.go  — the [A-Z] anchor targets exactly the exported-symbol qualifier / path.symbol form, NOT 'stagehand:' (error prefixes), '\"stagehand\"' (cmd-name string L42), or 'stagehand '/'stagehand.' lowercase."
  gotcha: "Do NOT touch the user-facing STRINGS / cmd-name in default_action.go: 'stagehand: configuration not loaded' (L44), 'stagehand: getwd' (L51), 'stagehand: --edit ignored' (L72), 'stagehand: another stagehand run' (L279), cmd.Name()==\"stagehand\" (L42). Those are P1.M2.T3 (strings) / T2 (cmd-name wiring). S3's sed anchor ([A-Z] after the dot) provably skips them."

- file: cmd/stagecoach/main.go   # VERIFY ONLY (no edit)
  why: "main.go is 'package main' (line 1 — verified). It imports internal/cmd (NOT pkg/stagecoach directly) and has ZERO 'stagehand.<Upper>' qualifier refs (verified). So it needs NO edit from S3. The contract point (d) asks to VERIFY its package is 'package main' — confirmed."

# Read-only refs (do NOT edit in S3)
- file: go.mod
  why: "READ-ONLY (S1 landed): module github.com/dustin/stagecoach. S3 does not touch it."
- file: internal/signal/signal_integration_test.go, internal/e2e/harness_test.go
  why: "READ-ONLY (S2 fixed their cmd/stagehand → cmd/stagecoach build-path refs). S3 does not touch test build-paths (S4 verifies them)."

# PRD authority (already in the selected content)
- prd: PRD.md §14 (package layout: cmd/stagecoach/ + pkg/stagecoach/ + pkg/stagecoach/stagecoach.go) + §14.1 ('package stagecoach' public surface) + §h2.30 ("all references to 'stagehand' must be replaced with 'stagecoach'").
  why: "The target layout (stagecoach.go declaring 'package stagecoach') and the global rename mandate."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/                              # repo dir name unchanged (rename is in-repo)
├── cmd/stagecoach/                     # S2 landed (was cmd/stagehand/)
│   └── main.go                         # 'package main' (VERIFY only — no edit)
├── pkg/stagecoach/                     # S2 landed (was pkg/stagehand/)
│   ├── stagehand.go        # RENAME → stagecoach.go    + edit L7 (package) + L1 (godoc)
│   └── stagehand_test.go   # RENAME → stagecoach_test.go + edit L1 (package)
└── internal/
    └── cmd/default_action.go           # EDIT (sed): stagehand.<Upper> → stagecoach.<Upper> (lines 26/194/217/340/371)
```

### Desired Codebase Tree After S3

```bash
stagehand/
├── cmd/stagecoach/
│   └── main.go                         # 'package main' (unchanged)
├── pkg/stagecoach/
│   ├── stagecoach.go       # 'package stagecoach' + godoc '// Package stagecoach is Stagecoach's …'
│   └── stagecoach_test.go  # 'package stagecoach'
└── internal/
    └── cmd/default_action.go           # stagecoach.GenerateCommit/Options/Result (code) + pkg/stagecoach.X (comments)
```

| Path | Action | Responsibility |
|---|---|---|
| `pkg/stagecoach/stagehand.go → stagecoach.go` | git mv + edit L7 + L1 | File rename + `package stagecoach` + godoc header. |
| `pkg/stagecoach/stagehand_test.go → stagecoach_test.go` | git mv + edit L1 | File rename + `package stagecoach`. |
| `internal/cmd/default_action.go` | sed (5 lines) | `stagehand.<Upper>` → `stagecoach.<Upper>` (the importer's package-qualifier refs). **Contract correction (build-required).** |
| `cmd/stagecoach/main.go` | VERIFY only | Confirm `package main` (no edit). |

**Explicitly NOT touched**: `go.mod` (S1 — landed), the directories (S2 — landed), the import-path strings
(S1/S2 — landed), test build-paths (S4), Go identifiers / `cmd.Name()=="stagehand"` (T2), user-facing
strings `stagehand: …` (P1.M2.T3), Makefile/.goreleaser/CI (P1.M3), docs/providers/FUTURE_SPEC (P1.M4),
`PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`, and the BODY of stagecoach.go (its other
`stagehand`/`Stagehand` mentions are T2/P1.M2/P1.M4).

### Known Gotchas of our Codebase & Library Quirks

```bash
# CRITICAL (contract correction — do NOT do "files + decls only"): the build is GREEN today only because
# default_action.go references the package via the 'stagehand.' qualifier. Renaming 'package stagehand' →
# 'package stagecoach' WITHOUT updating those qualifiers yields 'undefined: stagehand' at lines 194/340 and
# go build ./cmd/stagecoach/ FAILS — contradicting OUTPUT #4. S3 MUST sed default_action.go's package-
# qualifier refs. (Same correction pattern S2 used for the import-path shadow.)

# CRITICAL (use git mv, not plain mv): git mv records the move as a rename (R), preserving blame history.
# A plain mv + git add shows as delete+add. Both files are git-tracked (confirmed).

# CRITICAL (the [A-Z] sed anchor): use 's|stagehand\.\([A-Z]\)|stagecoach.\1|g' (note the captured uppercase
# letter). This targets EXACTLY the exported-symbol qualifier / path.symbol form (stagehand.GenerateCommit,
# pkg/stagehand.Result) and provably SKIPS: 'stagehand:' (error prefixes L44/51/72/279 — colon, not dot),
# '"stagehand"' (cmd.Name() string L42 — quote, not dot), 'stagehand ' (spaces L55/103), and 'stagehand.go'/
# 'stagehand_test' (lowercase). A bare 's|stagehand|stagecoach|g' would wrongly hit those — DO NOT use it.

# GOTCHA (test file is internal — no body change needed): stagecoach_test.go is 'package stagehand' (not
# 'package stagehand_test'), so it references exported symbols BARE (GenerateCommit, not
# stagehand.GenerateCommit). Changing its package decl to 'stagecoach' needs NO body edit — bare-name
# resolution is package-name-independent. Verified: zero 'stagehand.<Upper>' refs in the test file.

# GOTCHA (main.go needs no edit): cmd/stagecoach/main.go is 'package main' and imports internal/cmd (NOT
# pkg/stagecoach directly) — it has zero 'stagehand.<Upper>' qualifier refs. The contract point (d) asks to
# VERIFY 'package main'; it is. No edit.

# GOTCHA (don't sweep the body): stagecoach.go's body (34 KB) has other stagehand/Stagehand mentions
# (comments, error strings, identifiers). S3 touches ONLY the package clause (L7) + godoc header (L1).
# Body rename is T2 (identifiers) / P1.M2 (strings) / P1.M4 (docs).

# GOTCHA (don't touch user strings in default_action.go): the 'stagehand: …' error prefixes, the
# 'stagehand: another stagehand run …' message, and cmd.Name()=="stagehand" are P1.M2.T3 (strings) / T2
# (cmd-name). The [A-Z]-anchored sed provably skips them (colon/quote/space after 'stagehand', not '.<Upper>').

# GOTCHA (build is the gate, not a side effect): today go build ./... is GREEN. After S3 it MUST STILL be
# green (the qualifier sed is precisely what keeps it green). If go build ./cmd/stagecoach/ fails after S3,
# the most likely cause is a missed stagehand.<Upper> qualifier ref — grep -rnE 'stagehand\.[A-Z]' to find it.
```

## Implementation Blueprint

### Data models and structure

No data-model change — two file renames + package-clause edits + one importer sed. The exported symbol NAMES
(`GenerateCommit`, `Options`, `Result`, `Decompose`, `RoleModel`, …) are UNCHANGED — only the package NAME
changes (`stagehand` → `stagecoach`). The relevant existing state (the rename targets):

```go
// pkg/stagecoach/stagehand.go (current — to become stagecoach.go)
// Package stagehand is Stagehand's public library surface (PRD §14.1).     ← L1 godoc (edit)
// ...
package stagehand                                                             ← L7 (edit)

// pkg/stagecoach/stagehand_test.go (current — to become stagecoach_test.go)
package stagehand                                                             ← L1 (edit)

// internal/cmd/default_action.go (current importer — the contract correction)
import "github.com/dustin/stagecoach/pkg/stagecoach"                          ← L21 (S2 fixed; unchanged)
// L194: stagehand.GenerateCommit(ctx, stagehand.Options{                     ← sed
// L340: func printCommitReport(w io.Writer, res stagehand.Result, ...)        ← sed
// L26/217/371 (comments): pkg/stagehand.{GenerateCommit,Result,Decompose}    ← sed
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git mv the two files (preserve history)
  - RUN: git mv pkg/stagecoach/stagehand.go pkg/stagecoach/stagecoach.go
  - RUN: git mv pkg/stagecoach/stagehand_test.go pkg/stagecoach/stagecoach_test.go
  - VERIFY: ls pkg/stagecoach/ → stagecoach.go + stagecoach_test.go (no stagehand*.go).
  - VERIFY: git status shows R (rename) for both, not D+A.
  - DO NOT: edit file contents yet (Task 2/3); touch main.go.

Task 2: stagecoach.go — package declaration + godoc header
  - EDIT line 7: 'package stagehand' → 'package stagecoach'.
  - EDIT line 1 (godoc header): '// Package stagehand is Stagehand's public library surface (PRD §14.1).' →
    '// Package stagecoach is Stagecoach's public library surface (PRD §14.1).'.
  - DO NOT: edit any other line in the file (body comments/strings/identifiers are T2/P1.M2/P1.M4).

Task 3: stagecoach_test.go — package declaration
  - EDIT line 1: 'package stagehand' → 'package stagecoach'.
  - DO NOT: edit the body (internal test — bare names, no qualifier refs; verified).

Task 4: CONTRACT CORRECTION — sed default_action.go's package-qualifier refs (build-required)
  - RUN: sed -i 's|stagehand\.\([A-Z]\)|stagecoach.\1|g' internal/cmd/default_action.go
  - VERIFY the 6 substitutions landed on lines 26/194/217/340/371:
        grep -nE 'stagehand\.[A-Z]' internal/cmd/default_action.go   # → ZERO matches (all converted)
        grep -nE 'stagecoach\.[A-Z]' internal/cmd/default_action.go  # → the converted refs (incl. L194 x2)
  - VERIFY user-facing strings are UNTOUCHED (the [A-Z] anchor skipped them):
        grep -nE 'stagehand:|"stagehand"' internal/cmd/default_action.go  # → the error prefixes + cmd.Name() still say 'stagehand' (P1.M2.T3/T2 own these)
  - DO NOT: use a bare 's|stagehand|stagecoach|g' (would hit error strings/cmd-name); touch other files.

Task 5: VERIFY cmd/stagecoach/main.go (no edit)
  - RUN: head -1 cmd/stagecoach/main.go   # → 'package main'
  - RUN: grep -nE 'stagehand\.[A-Z]' cmd/stagecoach/main.go || echo "OK: no stagehand. qualifier in main.go"
  - EXPECT: 'package main' + zero qualifier refs. main.go needs NO edit.

Task 6: VALIDATE (the contract gate)
  - RUN: go build ./cmd/stagecoach/         # the CONTRACT GATE — must succeed (exit 0)
  - RUN: go build ./...                      # whole tree green
  - RUN: go vet ./...                        # clean (incl. no godoc Package-header mismatch)
  - RUN: go test ./...                       # all packages green (test file's bare names unaffected)
  - RUN: grep -rn '^package stagehand' --include='*.go' . | grep -v './.git/'   # → ZERO (no package stagehand remains)
  - RUN: git status --short                  # 2 renames (R) + default_action.go (M); nothing unexpected
  - FIX-FORWARD: if go build ./cmd/stagecoach/ fails with 'undefined: stagehand', a qualifier ref was
    missed — grep -rnE 'stagehand\.[A-Z]' --include='*.go' . to find it and fix it (re-run the sed on that file).
```

### Implementation Patterns & Key Details

```bash
# === the complete S3 (2 git moves + 3 edits + 1 sed) ===
git mv pkg/stagecoach/stagehand.go pkg/stagecoach/stagecoach.go
git mv pkg/stagecoach/stagehand_test.go pkg/stagecoach/stagecoach_test.go
# stagecoach.go: L7 'package stagehand' → 'package stagecoach'; L1 godoc 'Package stagehand is Stagehand's' → 'Package stagecoach is Stagecoach's'
# stagecoach_test.go: L1 'package stagehand' → 'package stagecoach'
sed -i 's|stagehand\.\([A-Z]\)|stagecoach.\1|g' internal/cmd/default_action.go
```

```bash
# === verify the contract gate + zero residue ===
go build ./cmd/stagecoach/                                                    # → exit 0 (the gate)
grep -rn '^package stagehand' --include='*.go' . | grep -v './.git/'          # → ZERO
grep -rnE 'stagehand\.[A-Z]' --include='*.go' . | grep -v './.git/' | grep -v '_test.go:' || echo "OK: no stagehand.<Upper> qualifier refs in production code"
# (the test file stagecoach_test.go has none either — verified internal/bare-names)
git status --short | grep -E 'pkg/stage|default_action'                       # → 2 R + 1 M
```

```go
// === default_action.go — the contract correction, before → after ===
// BEFORE (broken once package is renamed):
//   L194:  res, err := stagehand.GenerateCommit(ctx, stagehand.Options{
//   L340:  func printCommitReport(w io.Writer, res stagehand.Result, changes []git.FileChange) {
//   L26:   // PUBLIC API pkg/stagehand.GenerateCommit (US12 dogfooding), ...
//   L217:  // ... — pkg/stagehand.Result drops ...
//   L371:  // ... (pkg/stagehand.Decompose is P4.M2.T1.S1 — ...
// AFTER (sed 's|stagehand\.\([A-Z]\)|stagecoach.\1|g'):
//   L194:  res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{
//   L340:  func printCommitReport(w io.Writer, res stagecoach.Result, changes []git.FileChange) {
//   L26:   // PUBLIC API pkg/stagecoach.GenerateCommit (US12 dogfooding), ...
//   L217:  // ... — pkg/stagecoach.Result drops ...
//   L371:  // ... (pkg/stagecoach.Decompose is P4.M2.T1.S1 — ...
// UNTOUCHED (the [A-Z] anchor skips these — owned by P1.M2.T3/T2):
//   L42:  cmd.Name()=="stagehand"      L44: "stagehand: configuration not loaded"
//   L51:  "stagehand: getwd: ..."      L72: "stagehand: --edit ignored ..."
//   L279: "stagehand: another stagehand run is already in progress ..."
```

### Integration Points

```yaml
PACKAGE IDENTITY (pkg/stagecoach/):
  - file: stagehand.go → stagecoach.go            # 'package stagecoach' + godoc '// Package stagecoach'
  - file: stagehand_test.go → stagecoach_test.go  # 'package stagecoach'

IMPORTER QUALIFIER REFS (internal/cmd/default_action.go — the contract correction):
  - stagehand.GenerateCommit/Options (L194) → stagecoach.*       # CODE (build-breaking if missed)
  - stagehand.Result (L340) → stagecoach.Result                  # CODE (build-breaking if missed)
  - pkg/stagehand.{GenerateCommit,Result,Decompose} (L26/217/371) → pkg/stagecoach.*   # comments

VERIFIED NO-OP (cmd/stagecoach/main.go):
  - 'package main' (L1); zero stagehand.<Upper> refs; imports internal/cmd (not pkg/stagecoach). No edit.

CONSUMED (read-only — S1+S2 landed):
  - go.mod module = github.com/dustin/stagecoach (S1)
  - directories cmd/stagecoach/ + pkg/stagecoach/ (S2)
  - default_action.go:21 import = pkg/stagecoach (S2)

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - go.mod / directories / import-path strings / test build-paths   # S1/S2 landed; S4 verifies test paths
  - Go identifiers / cmd.Name()=="stagehand"                        # P1.M1.T2
  - user-facing strings 'stagehand: …'                              # P1.M2.T3
  - .stagehandignore / env vars / git-config keys                   # P1.M2
  - Makefile / .goreleaser / CI                                     # P1.M3
  - README / docs/*.md / providers/*.toml / FUTURE_SPEC.md          # P1.M4
  - the BODY of stagecoach.go (other stagehand/Stagehand mentions)  # T2 / P1.M2 / P1.M4
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S3):
  - S4: verify binary build-paths in test code (signal_integration_test.go / harness_test.go — S2 already fixed; S4 re-verifies + greps for any others).
  - T2: rename Go IDENTIFIERS containing stagehand/Stagehand (the cmd.Name()=="stagehand" wiring, type/func/var names). Non-build-breaking; follows S3.
  - P1.M2.T3: rename the 'stagehand: …' user-facing strings.
  - P1.M5.T2.S1: final grep audit — zero stagehand references in tracked files (the safety net).
```

## Validation Loop

### Level 1: The Renames + Edits (the deliverable)

```bash
cd /home/dustin/projects/stagehand

# files renamed (old gone, new present)
ls pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go && ! ls pkg/stagecoach/stagehand.go pkg/stagecoach/stagehand_test.go 2>/dev/null
# Expected: the two stagecoach files listed; the stagehand files error out.

# git tracked as renames (R), not delete+add
git status --short | grep -E 'pkg/stagecoach'
# Expected: R pkg/stagecoach/stagehand.go → stagecoach.go ; R .../stagehand_test.go → stagecoach_test.go.

# package declarations + godoc header
sed -n '1p;7p' pkg/stagecoach/stagecoach.go    # → '// Package stagecoach is Stagecoach's …' + 'package stagecoach'
sed -n '1p' pkg/stagecoach/stagecoach_test.go  # → 'package stagecoach'

# zero 'package stagehand' anywhere
grep -rn '^package stagehand' --include='*.go' . | grep -v './.git/' || echo "OK: zero package stagehand"
```

### Level 2: The Contract Correction (the importer qualifier refs)

```bash
cd /home/dustin/projects/stagehand

# the sed landed — zero stagehand.<Upper> refs, the stagecoach.* refs present
grep -nE 'stagehand\.[A-Z]' internal/cmd/default_action.go || echo "OK: zero stagehand.<Upper> in default_action.go"
grep -nE 'stagecoach\.(GenerateCommit|Options|Result|Decompose)' internal/cmd/default_action.go
# Expected: OK zero; then the converted refs (incl. L194 stagecoach.GenerateCommit + stagecoach.Options).

# user-facing strings UNTOUCHED (the [A-Z] anchor skipped them — P1.M2.T3/T2 own these)
grep -nE 'stagehand:|"stagehand"' internal/cmd/default_action.go
# Expected: the error prefixes (L44/51/72/279) + cmd.Name()=="stagehand" (L42) STILL say 'stagehand'.
```

### Level 3: The Contract Gate — Build Green (the real validation)

```bash
cd /home/dustin/projects/stagehand

go build ./cmd/stagecoach/     # THE CONTRACT GATE — must exit 0
go build ./...                 # whole tree green
go vet ./...                   # clean (no godoc Package-header mismatch; no unused import)
go test ./...                  # all packages green (test file bare-names unaffected)
# Expected: all exit 0. If 'go build ./cmd/stagecoach/' fails with 'undefined: stagehand', a qualifier ref
# was missed — grep -rnE 'stagehand\.[A-Z]' --include='*.go' . and fix it.
```

### Level 4: Scope Discipline (only the intended files changed)

```bash
cd /home/dustin/projects/stagehand

# S3 touches ONLY: the 2 renamed pkg files + default_action.go. Confirm:
git status --short | grep -vE '^\?\?'
# Expected: R pkg/stagecoach/stagehand.go→stagecoach.go ; R .../stagehand_test.go→stagecoach_test.go ;
#           M internal/cmd/default_action.go ; (M pkg/stagecoach/stagecoach.go + stagecoach_test.go for the decl/header edits).
#           NOTHING else (no go.mod, no other internal/, no cmd/, no docs, no Makefile).

# confirm main.go is unchanged (verify-only)
git diff -- cmd/stagecoach/main.go   # Expected: empty

# confirm the body of stagecoach.go was NOT swept (only L1 + L7 changed)
git diff -- pkg/stagecoach/stagecoach.go | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)'
# Expected: only the L1 godoc header line + the L7 package line (2 changed lines).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./cmd/stagecoach/` exits 0 (the contract gate).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0 (no godoc Package-header mismatch).
- [ ] `go test ./...` — all packages green.
- [ ] ZERO `package stagehand` declarations remain (`grep -rn '^package stagehand'`).
- [ ] ZERO `stagehand.<Upper>` qualifier refs in production code.

### Feature Validation

- [ ] `pkg/stagecoach/stagecoach.go` + `stagecoach_test.go` exist (renamed); old names gone.
- [ ] Both files declare `package stagecoach`; `stagecoach.go` godoc header reads `// Package stagecoach`.
- [ ] `default_action.go` reads `stagecoach.GenerateCommit`/`Options`/`Result` (code) + `pkg/stagecoach.X` (comments).
- [ ] `cmd/stagecoach/main.go` is `package main` (verified, unchanged).

### Scope Discipline Validation

- [ ] ONLY the 2 renamed pkg files + `internal/cmd/default_action.go` modified (git status confirms).
- [ ] `main.go` unchanged (verify-only).
- [ ] Did NOT touch user-facing strings (`stagehand: …`), `cmd.Name()=="stagehand"`, or Go identifiers (T2/P1.M2.T3).
- [ ] Did NOT sweep the body of `stagecoach.go` for other `stagehand`/`Stagehand` mentions (only L1+L7).
- [ ] Did NOT edit go.mod (S1), directories (S2), import-path strings (S1/S2), test build-paths (S4), docs (P1.M4), Makefile/.goreleaser/CI (P1.M3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] `git mv` used (history preserved — renames R, not delete+add).
- [ ] The `[A-Z]`-anchored sed targets exactly the package-qualifier/path.symbol form (provably skips `stagehand:`/`"stagehand"`/`stagehand.go`).
- [ ] The godoc header matches the package declaration (golint-clean).
- [ ] The contract correction (importer qualifier fix) is documented + achieves OUTPUT #4.

---

## Anti-Patterns to Avoid

- ❌ Don't do "files + package decls only" — the contract LOGIC (c) names just the 2 pkg files, but OUTPUT #4
  requires `go build ./cmd/stagecoach/` to succeed. The build is green TODAY only because `default_action.go`
  references the package via the `stagehand.` qualifier; renaming the declaration breaks lines 194/340
  (`undefined: stagehand`). S3 MUST sed the importer's qualifier refs. (Same correction pattern S2 used.)
- ❌ Don't use a bare `s|stagehand|stagecoach|g` sed on `default_action.go` — it would wrongly hit the
  user-facing strings (`stagehand: configuration not loaded`, `stagehand: another stagehand run …`) and the
  `cmd.Name()=="stagehand"` wiring (P1.M2.T3/T2 territory). Use the `[A-Z]`-anchored
  `s|stagehand\.\([A-Z]\)|stagecoach.\1|g` — it targets exactly the exported-symbol qualifier / path.symbol
  form and provably skips colon/quote/space/lowercase.
- ❌ Don't use plain `mv` — use `git mv` to preserve history (rename R, not delete+add). Both files are
  git-tracked.
- ❌ Don't sweep the body of `stagecoach.go` for other `stagehand`/`Stagehand` mentions. S3 touches ONLY the
  package clause (L7) + the godoc header (L1). Body comments/strings/identifiers are T2 / P1.M2 / P1.M4.
- ❌ Don't edit `cmd/stagecoach/main.go` — it's `package main` (verified) and has zero `stagehand.<Upper>`
  qualifier refs (it imports `internal/cmd`, not `pkg/stagecoach`). The contract point (d) is a VERIFY, not
  an edit.
- ❌ Don't touch the user-facing strings in `default_action.go` (`stagehand: …` prefixes, the
  `cmd.Name()=="stagehand"` string). Those are P1.M2.T3 (strings) / T2 (cmd-name wiring). The `[A-Z]` sed
  anchor provably skips them.
- ❌ Don't change the exported symbol NAMES (`GenerateCommit`, `Options`, `Result`, `Decompose`, …). Only the
  PACKAGE name changes. Symbol names are T2's concern (and most don't contain "stagehand" anyway).
- ❌ Don't edit go.mod (S1), directories (S2), import-path strings (S1/S2), test build-paths (S4), docs
  (P1.M4), Makefile/.goreleaser/CI (P1.M3), or `.stagehandignore`/env/git-config (P1.M2). S3 is the package
  identity (files + decls + godoc header) + the importer qualifier refs ONLY.
- ❌ Don't chase the build to green by renaming things outside scope — if it fails, the cause is a MISSED
  `stagehand.<Upper>` qualifier ref (grep finds it; re-run the sed on that file), NOT a license to rename
  strings/identifiers.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: Two history-preserving `git mv`s + two package-clause edits + one godoc-header edit + one precise
`[A-Z]`-anchored sed. The key de-risking is the EMPIRICAL verification of the complete change surface: `go
build ./...` is GREEN today (proving the sole importer is `default_action.go` and it uses the `stagehand.`
qualifier), and grep enumerated the EXACT 6 qualifier/path references (lines 26/194/217/340/371 — 2 code,
3 comment) plus confirmed there are NO other importers and NO `stagehand.<Upper>` refs in main.go or the test
file. The `[A-Z]` sed anchor is provably precise (it skips the `stagehand:`/`"stagehand"`/`stagehand `
strings that belong to P1.M2.T3/T2). The contract correction (importer qualifier fix) is documented with the
build-break reasoning — same pattern S2 established, so the implementer won't follow the literal "decls
only" LOGIC into a failed build gate. S2 is LANDED (verified: dirs are stagecoach/), so S3 operates on the
real post-S2 tree. The only residual uncertainty (not 10/10) is whether the implementing agent resists the
temptation to over-sweep the file body (the PRP fences this in 5 places + the Level-4 git-diff gate asserts
only L1+L7 changed in stagecoach.go). The blast radius is 2 renamed files + 1 importer; everything else is
cleanly fenced.
