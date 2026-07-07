---
name: "P1.M1.T1.S1 — Rename go.mod module path and all Go import paths"
description: |
  Mechanical rename: go.mod `module github.com/dustin/stagehand` → `module github.com/dustin/stagecoach`;
  sed all ~244 Go import-path occurrences across ~73 .go files. `go build ./...` must still compile.
  No docs, no identifiers, no env vars — just the module path + import strings. The FIRST step of the
  project rename (directories/identifiers are S2-S4).
---

## Goal

**Feature Goal**: Change the Go module path from `github.com/dustin/stagehand` to `github.com/dustin/stagecoach`
in `go.mod` and in ALL Go import paths across the codebase, so `go build ./...` compiles under the new
identity. This is Layer 1 of the rename (module + imports only — directories/identifiers/env-vars are
later subtasks).

**Deliverable** (2 mechanical edits):
1. `go.mod` line 1: `module github.com/dustin/stagehand` → `module github.com/dustin/stagecoach`.
2. All `.go` files: `sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g'` across ~73 files / ~244 occurrences.

**Success Definition**: `go build ./...` and `go vet ./...` compile clean (all import paths match the
new module name). Zero remaining `github.com/dustin/stagehand` occurrences in any `.go` file. The
codebase functions identically — this is a pure rename, no logic change.

## Why

- **First step of the project rename.** The PRD (§h2.30) states: "this project was originally named
  'stagehand' and has been renamed. All references to 'stagehand' must be replaced with 'stagecoach'."
  Layer 1 (module + imports) must land before directory/identifier renames (S2-S4) because Go compilation
  depends on the import paths matching the module name.
- **Pure mechanical rename = zero risk.** Every `github.com/dustin/stagehand/` import path becomes
  `github.com/dustin/stagecoach/` — a straight string substitution with no semantic change. The code
  compiles identically (the packages are the same; only the module-qualified path changes).
- **go.sum is NOT affected.** go.sum has no self-reference (verified) — there are no internal module
  dependencies, so no checksum entry for `github.com/dustin/stagehand`.

## What

A two-command mechanical rename. No logic change, no new files, no doc edits.

### Success Criteria

- [ ] `go.mod` line 1 reads `module github.com/dustin/stagecoach`.
- [ ] Zero `github.com/dustin/stagehand` occurrences remain in any `.go` file (grep confirms).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./...` passes (the rename is import-path-only; tests compile against the new path).

## All Needed Context

### Documentation & References

```yaml
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  why: "Layer 1 (§1.1-1.2) specifies the exact commands: `go mod edit -module github.com/dustin/stagecoach` for go.mod; `find . -name '*.go' -exec sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' {} +` for imports. Confirms ~257 occurrences across ~100+ files (actual: ~244 across ~73 files — close). Notes go.sum has no self-reference."

- file: go.mod
  why: "EDIT line 1: `module github.com/dustin/stagehand` → `module github.com/dustin/stagecoach`. Use `go mod edit -module github.com/dustin/stagecoach` (the canonical Go tool — validates the result)."

- file: all .go files under cmd/, pkg/, internal/
  why: "EDIT: sed-replace every `github.com/dustin/stagehand` import path → `github.com/dustin/stagecoach`. This includes test files (_test.go) which import the module's internal packages."
```

### Known Gotchas

```bash
# Use `go mod edit -module` (NOT a manual text edit) for go.mod — the Go tool updates the module
# path atomically and is the canonical way. Then sed the .go files.

# The sed MUST cover test files (_test.go) — they import internal packages via the module path.
# The find pattern `-name '*.go'` covers both production and test files.

# Do NOT touch go.sum — it has no self-reference (verified: `grep stagehand go.sum` is empty).

# Do NOT rename directories, package declarations, identifiers, env vars, or strings in this subtask.
# S2 renames cmd/stagehand→cmd/stagecoach and pkg/stagehand→pkg/stagecoach directories.
# S3 renames package declarations within those directories.
# P1.M1.T2 renames Go identifiers (Stagehand→Stagecoach).
# P1.M2 renames env vars, git config keys, file paths, strings.
# This subtask is ONLY the module path + import strings.
```

## Implementation Blueprint

### Implementation Tasks

```yaml
Task 1: Rename the go.mod module path
  - RUN: go mod edit -module github.com/dustin/stagecoach
  - VERIFY: head -1 go.mod → "module github.com/dustin/stagecoach"

Task 2: Rename all Go import paths
  - RUN: find . -name '*.go' -not -path './.git/*' -exec sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' {} +
  - VERIFY: grep -r "github.com/dustin/stagehand" --include="*.go" . | grep -v './.git/' | wc -l → 0

Task 3: Validate compilation
  - RUN: go build ./...    # Expected: exit 0 (all import paths match the new module name)
  - RUN: go vet ./...      # Expected: exit 0
  - RUN: go test ./...     # Expected: all packages pass (import-path-only rename; no logic change)
  - FIX-FORWARD: if any file was missed (unlikely with find+sed), grep + manually fix.
```

### Implementation Patterns & Key Details

```bash
# The complete rename (2 commands):
go mod edit -module github.com/dustin/stagecoach
find . -name '*.go' -not -path './.git/*' -exec sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' {} +

# Verify:
head -1 go.mod                                                    # → module github.com/dustin/stagecoach
grep -r "github.com/dustin/stagehand" --include="*.go" . | wc -l # → 0
go build ./...                                                    # → exit 0
```

### Integration Points

```yaml
MODULE PATH (go.mod):
  - "module github.com/dustin/stagecoach"  (was: github.com/dustin/stagehand)

IMPORT PATHS (all .go files):
  - every "github.com/dustin/stagehand/..." → "github.com/dustin/stagecoach/..."

NO-TOUCH:
  - go.sum (no self-reference)
  - directories (cmd/stagehand, pkg/stagehand — S2 renames them)
  - package declarations within those dirs (S3)
  - Go identifiers (Stagehand→Stagecoach — P1.M1.T2)
  - env vars / git config keys / strings (P1.M2)
  - docs / Makefile / .goreleaser / CI (P1.M3-P1.M4)
  - plan/ directory (P1.M5)
  - PRD.md, tasks.json, prd_snapshot.md
```

## Validation Loop

### Level 1: Build

```bash
cd /home/dustin/projects/stagehand
go build ./...   # Expected: exit 0
go vet ./...     # Expected: exit 0
```

### Level 2: Tests

```bash
go test ./...    # Expected: all packages pass
```

### Level 3: Grep Audit

```bash
# Zero remaining old-module references in Go files
grep -r "github.com/dustin/stagehand" --include="*.go" . | grep -v './.git/' | wc -l
# Expected: 0

# New module path is correct
head -1 go.mod
# Expected: module github.com/dustin/stagecoach
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./...` — all packages pass.
- [ ] Zero `github.com/dustin/stagehand` in `.go` files.
- [ ] `go.mod` reads `module github.com/dustin/stagecoach`.

### Scope Discipline
- [ ] ONLY `go.mod` + `.go` files modified (module path + import strings).
- [ ] Did NOT rename directories, packages, identifiers, env vars, strings, docs.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**10/10** — a two-command mechanical `sed` rename. `go build ./...` is the exhaustive oracle: if any
import path was missed, the compiler reports it. The go.sum has no self-reference (verified). No logic
change, no risk beyond a typo (caught by the build).
