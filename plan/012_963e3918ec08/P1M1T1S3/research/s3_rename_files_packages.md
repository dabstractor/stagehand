# Research Note — P1.M1.T1.S3 (rename files + package declarations in renamed dirs)

## Working-tree state (verified)

S2 has **LANDED**: directories are `cmd/stagecoach/` and `pkg/stagecoach/`. The repo dir is still
`/home/dustin/projects/stagehand` (rename is in-repo). `go.mod` module = `github.com/dustin/stagecoach` (S1).

Inside `pkg/stagecoach/` the files are STILL `stagehand.go` (34 KB) + `stagehand_test.go` (68 KB), both
declaring `package stagehand`. This is the state S3 transforms.

## The pivotal finding — the build is currently GREEN (and why)

`go build ./...` exits 0 today; `go vet ./pkg/stagecoach/` exits 0. Go does NOT require the package name to
match the directory — `pkg/stagecoach/` containing `package stagehand` compiles fine. The sole importer is
`internal/cmd/default_action.go:21` (`"github.com/dustin/stagecoach/pkg/stagecoach"`), and it references the
package via the `stagehand.` qualifier — which resolves because the declared package name is still
`stagehand`.

⇒ **Changing `package stagehand` → `package stagecoach` WILL BREAK the `stagehand.` qualifier references in
default_action.go.** For the contract's OUTPUT (`go build ./cmd/stagecoach/` must succeed) to hold, S3 must
ALSO update those qualifier references. This is a **CONTRACT CORRECTION** (same pattern as S2's import-path
fix): the contract LOGIC (c) names only the 2 pkg files, but OUTPUT #4 (build succeeds) requires the
importer's qualifier refs to follow the package rename. Nobody else fixes them between S3 and a green build
(S4 = test build-paths; T2 = identifiers; P1.M2.T3 = strings).

## The complete, verified change surface

### A. File renames (git mv — preserve history)
- `git mv pkg/stagecoach/stagehand.go pkg/stagecoach/stagecoach.go`
- `git mv pkg/stagecoach/stagehand_test.go pkg/stagecoach/stagecoach_test.go`

### B. Package declarations (the contract core)
- `pkg/stagecoach/stagecoach.go:7`: `package stagehand` → `package stagecoach`
- `pkg/stagecoach/stagecoach_test.go:1`: `package stagehand` → `package stagecoach`

### C. godoc package header (attached to the declaration — golint-canonical)
- `pkg/stagecoach/stagecoach.go:1`: `// Package stagehand is Stagehand's public library surface` →
  `// Package stagecoach is Stagecoach's public library surface`
  (golint/revive/staticcheck flag a `// Package <X>` mismatch with `package <Y>`; this is the package's own
  identity header, not body prose — body "Stagehand" mentions are T2/P1.M2/P1.M4.)

### D. CONTRACT CORRECTION — package-qualifier refs in the sole importer (build-required)
`internal/cmd/default_action.go` is the ONLY file importing `pkg/stagecoach` (verified: grep #1). Complete
set of `stagehand.<Uppercase>` references (verified: grep #2/#3 — these are ALL of them):

| Line | Content (excerpt) | Kind | Action |
|---|---|---|---|
| 26 | `// PUBLIC API pkg/stagehand.GenerateCommit (US12 dogfooding)` | comment | `pkg/stagehand` → `pkg/stagecoach` |
| 194 | `res, err := stagehand.GenerateCommit(ctx, stagehand.Options{` | **CODE** | `stagehand.` → `stagecoach.` (2 refs) |
| 217 | `// ... — pkg/stagehand.Result drops` | comment | `pkg/stagehand` → `pkg/stagecoach` |
| 340 | `func printCommitReport(w io.Writer, res stagehand.Result, ...)` | **CODE** | `stagehand.` → `stagecoach.` |
| 371 | `// ... (pkg/stagehand.Decompose is P4.M2.T1.S1 —` | comment | `pkg/stagehand` → `pkg/stagecoach` |

The CODE refs (194, 340) are the build-breakers; the comment refs (26, 217, 371) describe the renamed
package and are now factually wrong — all 5 lines reference the package identity S3 owns.

**Precise sed** that targets exactly these (a `stagehand.` followed by an UPPERCASE letter = exported-symbol
qualifier / path.symbol — does NOT touch `stagehand:` error prefixes, `"stagehand"` cmd-name, or
`stagehand.go`/`stagehand_test` lowercase):

```bash
sed -i 's|stagehand\.\([A-Z]\)|stagecoach.\1|g' internal/cmd/default_action.go
```

Verified this matches exactly lines 26, 194, 217, 340, 371 (6 substitutions; line 194 has 2) and nothing
else in the file (the `stagehand:` / `"stagehand"` / `stagehand ` strings have colon/space/space after, not
`.<Upper>`).

### E. cmd/stagecoach/main.go — verify only (NO edit)
- Confirmed `package main` (line 1). NO `stagehand.` qualifier (grep #6: none — main.go imports
  `internal/cmd`, not `pkg/stagecoach` directly). main.go needs NO change from S3.

## What is explicitly OUT OF SCOPE (verified — owned by other tasks)

`default_action.go` has OTHER `stagehand` references that are NOT package-identity — S3 leaves them:
- line 42: `cmd.Name()=="stagehand"` (cobra command-name string) — T2 / cmd wiring
- line 44: `errors.New("stagehand: configuration not loaded")` — P1.M2.T3 (user-facing strings)
- line 51: `fmt.Errorf("stagehand: getwd: ...")` — P1.M2.T3
- line 72: `"stagehand: --edit ignored ..."` — P1.M2.T3
- line 279: `"stagehand: another stagehand run ..."` — P1.M2.T3
- line 55, 103: comments about "stagehand processes" / prints — P1.M4 / grep-audit (P1.M5.T2.S1)

The body of `pkg/stagecoach/stagecoach.go` (34 KB) has other `stagehand`/`Stagehand` mentions (comments,
error strings, identifiers) — those are T2 (identifiers) / P1.M2 (strings) / P1.M4 (docs). S3 touches ONLY
the package declaration + its godoc header.

The test file `stagecoach_test.go` is `package stagehand` (INTERNAL — uses bare names, no `stagehand.`
qualifier: verified grep #4). So changing its package decl to `stagecoach` needs NO body change — bare-name
resolution is package-name-independent.

## Why the qualifier fix is S3's, not T2's (scope boundary)

T2 = "Rename Go IDENTIFIERS containing stagehand" (declared names: types/funcs/vars). The `stagehand.`
package-qualifier USAGES are a different category — they are CONSEQUENCES of the package-declaration rename.
You cannot rename a Go package without simultaneously updating every usage of its name as a qualifier, or
the build breaks. This coupling makes the qualifier fix inseparable from the declaration rename (S3), and
S3's OUTPUT (build succeeds) demands it. T2 handles declared identifiers (a separate, non-build-breaking
surface that can follow).

## rename_surface_map.md authority

Layer 1.4 (file renames): `pkg/stagehand/stagehand.go → pkg/stagecoach/stagecoach.go` (the test file isn't
listed in 1.4 but follows identically — the map's §1.5 lists both package-decl sites). Layer 1.5 (package
declarations): `stagehand.go:7` + `stagehand_test.go:1`. The map does NOT list the importer qualifier refs
— that's the gap this contract correction fills (same as S2 found the map didn't list the 3 import-path
subdir refs S1's sed missed).

## Validation summary

- `go build ./cmd/stagecoach/` succeeds (the contract gate).
- `go build ./...` + `go vet ./...` + `go test ./...` green (the package rename + qualifier fix keeps the
  whole tree compiling; the test file's bare names are unaffected).
- `git status` shows the 2 file renames as R (history preserved), the 2 package-decl/godoc edits in the
  renamed files, and default_action.go (M).
- Grep: zero `package stagehand` remains; zero `stagehand.<Upper>` qualifier refs remain in production code
  (default_action.go now reads `stagecoach.`).
