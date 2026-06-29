---
name: "P1.M1.T1.S1 — Initialize Go module, directory tree, and .gitignore"
description: |
  Greenfield Go scaffolding for Stagehand. Create `go.mod` (module `github.com/dustin/stagehand`, `go 1.22`),
  the full package tree from PRD §14, the `.gitignore` build/test artifacts, and a `main.go` stub so
  `go build ./...` compiles. No deps yet (cobra / go-toml arrive in later subtasks).
---

## Goal

**Feature Goal**: Establish a compilable Go module skeleton for the Stagehand repository —
module path, version directive, the complete package directory tree from PRD §14, a working
`.gitignore`, and a minimal `main.go` entrypoint — such that `go build ./...` succeeds with
zero dependencies and zero external network access.

**Deliverable**:
1. `go.mod` — module `github.com/dustin/stagehand`, directive `go 1.22`, no `toolchain` line, no `require` block.
2. The full directory tree from PRD §14 (`cmd/`, `internal/{config,provider,prompt,git,generate,ui}/`, `pkg/stagehand/`, `providers/`, `docs/`).
3. `.gitignore` at repo root, augmented with the four contract-required entries (`/bin/`, `*.test`, `coverage.out`, `/dist/`).
4. `cmd/stagehand/main.go` — minimal stub (`package main; func main(){}`).
5. Empirically-verified: `go build ./...` exits 0.

**Success Definition**: A fresh clone of this repo runs `go build ./...` and `go vet ./...`
cleanly, `go.mod` declares `module github.com/dustin/stagehand` + `go 1.22`, the `.gitignore`
excludes build/test artifacts, and the §14 directory tree exists on disk.

## User Persona

**Target User**: The Stagehand contributor (developer implementing subsequent subtasks T2–T5, M2–M5).

**Use Case**: Every later subtask (git plumbing, config, provider manifests, CLI) needs a
compilable module to land code into. This subtask is the foundation every later `*.go` file
imports from and builds within.

**Pain Points Addressed**: Removes the "where do I put this file / what module path / what go
version" ambiguity for every downstream task by fixing the package layout and module identity now.

## Why

- **Foundation for the entire v1 build.** PRD §14 defines the canonical Go package layout; this
  subtask instantiates it so subsequent subtasks have a home for each `*.go` file.
- **Locks the module path** `github.com/dustin/stagehand` — every import path downstream
  (`github.com/dustin/stagehand/internal/git`, `github.com/dustin/stagehand/pkg/stagehand`) derives from it.
- **Pins the language floor at `go 1.22`** per PRD §22.3 (stdlib `os/exec`, `os/signal`,
  `context`, `encoding/json` — all available; 1.22 also gives the enhanced `for` loop ranges used later).
- **Intentionally deprioritizes dependencies.** Cobra and `pelletier/go-toml/v2` are deliberately
  NOT added here — they land in their own subtasks (M1.T4 config, M4.T1 CLI) to keep this slice
  trivially compilable and reviewable.
- **No user-facing surface change** (PRD §"DOCS: none") — purely internal scaffolding.

## What

A compilable Go module with the PRD §14 directory structure in place. No application logic, no
dependencies, no config parsing, no git operations. The only executable artifact is a no-op binary.

### Success Criteria

- [ ] `go.mod` exists with `module github.com/dustin/stagehand` and `go 1.22` (no `toolchain`, no `require`).
- [ ] Directories exist on disk: `cmd/stagehand/`, `internal/config/`, `internal/provider/`, `internal/prompt/`, `internal/git/`, `internal/generate/`, `internal/ui/`, `pkg/stagehand/`, `providers/`, `docs/`.
- [ ] `cmd/stagehand/main.go` exists with `package main` and an empty `func main()`.
- [ ] `.gitignore` at repo root contains all four entries: `/bin/`, `*.test`, `coverage.out`, `/dist/`.
- [ ] `.gitignore` does NOT ignore `plan/`, `PRD.md`, or any task/PRD-snapshot files.
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing (all Go files already formatted).
- [ ] No dependencies added (`go.mod` has no `require` block; no `go.sum` generated).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** This PRP states the exact module path, go version, directory list,
.gitignore entries, the empirically-verified command sequence (including the `-go` flag trap),
and the validation commands. No inference required.

### Documentation & References

```yaml
# MUST READ - Primary spec sources
- file: PRD.md
  why: "§14 (Package layout) is the authoritative directory tree; §22.3 (Dependencies) pins go 1.22+ and lists cobra/go-toml as LATER deps (do NOT add now)."
  critical: "§14 also lists files like internal/git/git.go, providers/*.toml, Makefile, README.md — these belong to LATER subtasks, NOT this one. This subtask only creates DIRECTORIES + go.mod + .gitignore + main.go stub."

- docfile: plan/001_f1f80943ac34/architecture/system_context.md
  why: "§1 confirms greenfield state, Go 1.26.4 + git 2.54.0 installed; §4 restates the §14 layout; §6 lists deps (cobra, go-toml) as not-yet-added."
  section: "§1 Project State, §4 Package Layout, §6 Dependencies"

- docfile: plan/001_f1f80943ac34/P1M1T1S1/research/go_mod_scaffold_probe.md
  why: "EMPIRICALLY VERIFIED go.mod creation sequence and empty-dir build behavior on the installed go1.26.4 toolchain. Read this before running any go command."
  critical: "go mod init has NO -go flag in go1.26.4. Must run `go mod edit -go=1.22` after init. Empty package dirs are silently skipped by go build ./... — only main.go is required."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "Project-wide gotchas every later subtask needs. Skim now; the go-toml omitempty (FINDING 5) and exit-code traps are NOT this subtask's concern, but you must know the .gitignore must accommodate coverage.out (FINDING 7 region) and *.test binaries."
```

### Current Codebase Tree

Current repo state (greenfield — only planning artifacts + a partial `.gitignore`):

```bash
stagehand/
├── .gitignore            # PARTIAL — missing /bin/, *.test, coverage.out
├── PRD.md                # the product spec
└── plan/
    └── 001_f1f80943ac34/
        ├── architecture/      # research docs (system_context, critical_findings, etc.)
        ├── P1M1T1S1/          # THIS subtask (PRP.md, research/)
        ├── prd_index.txt
        ├── prd_snapshot.md
        └── tasks.json
```

No Go code, no `go.mod`, no `go.sum` exists yet.

### Desired Codebase Tree After This Subtask

```bash
stagehand/
├── .gitignore            # MODIFIED — +/bin/, +*.test, +coverage.out (+ keep existing entries)
├── PRD.md                # unchanged
├── go.mod                # NEW — module github.com/dustin/stagehand, go 1.22
├── plan/                 # unchanged
├── cmd/
│   └── stagehand/
│       └── main.go       # NEW — stub: package main; func main(){}
├── internal/             # NEW — empty package dirs (populated by later subtasks)
│   ├── config/           #   ← P1.M1.T4
│   ├── provider/         #   ← P1.M2
│   ├── prompt/           #   ← P1.M3.T1
│   ├── git/              #   ← P1.M1.T2 / T3
│   ├── generate/         #   ← P1.M3.T4
│   └── ui/               #   ← P1.M4.T3
├── pkg/
│   └── stagehand/        # NEW empty dir ← P1.M3.T5 (public API)
├── providers/            # NEW empty dir ← P1.M5.T2 (TOML manifests)
└── docs/                 # NEW empty dir ← PRD.md copy / overview (M5.T5)
```

**File responsibilities for THIS subtask (only 2 NEW files + 1 MODIFIED):**
| Path | Action | Responsibility |
|---|---|---|
| `go.mod` | NEW | Module identity + language floor. No deps. |
| `cmd/stagehand/main.go` | NEW | Entry point stub. Compiles to a no-op binary so `go build ./...` succeeds. |
| `.gitignore` | MODIFY | Add `/bin/`, `*.test`, `coverage.out`. Keep all existing entries. |

**Explicitly NOT created now** (later subtasks): `Makefile` (S2), `internal/**/*.go`,
`pkg/stagehand/stagehand.go`, `providers/*.toml`, `docs/PRD.md`, `.goreleaser.yaml`, `README.md`,
`go.sum`. Creating these now is scope creep and will be overwritten/removed by their owners.

### Known Gotchas of our Codebase & Toolchain

```go
// CRITICAL: `go mod init -go=1.22` is INVALID in go1.26.4 (flag does not exist → exit 2).
// VERIFIED FIX: run `go mod init github.com/dustin/stagehand` THEN `go mod edit -go=1.22`.
//   - init writes `go 1.26.4`; edit rewrites to `go 1.22` with NO toolchain directive.
//   - Do NOT hand-edit go.mod with a `toolchain` line — keep it minimal per contract.

// CRITICAL: Empty package directories (internal/git, pkg/stagehand, …) are SILENTLY SKIPPED
// by `go build ./...`, `go vet ./...`, and `go list ./...`. They produce ZERO errors.
// VERIFIED: only `cmd/stagehand/main.go` is needed for the build to pass. Do NOT add stub
// .go files to other packages — the contract says "only where needed", and each package gets
// real source files in its own later subtask.

// GOTCHA: Git does not track empty directories. The new internal/* and pkg/stagehand dirs will
// not appear in `git status` until a later subtask adds a .go file. This is EXPECTED and fine —
// `go build ./...` does not depend on them. Do NOT add .gitkeep files (scope creep).

// GOTCHA: `go.mod`'s `go 1.22` under the 1.26.4 toolchain does NOT trigger a toolchain download
// (current >= required), so `go build ./...` runs fully offline. No go.sum is generated (no deps).

// CRITICAL (forbidden): Do NOT add `plan/`, `PRD.md`, `prd_snapshot.md`, or `tasks.json` to
// .gitignore. These MUST remain tracked. The existing .gitignore already complies — keep it.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: INITIALIZE the Go module (module path + go version)
  - RUN: cd <repo-root>  # /home/dustin/projects/stagehand
  - RUN: go mod init github.com/dustin/stagehand
      # creates go.mod with `go 1.26.4` (current toolchain default)
      # REQUIRES: no pre-existing go.mod in repo root (confirmed greenfield)
  - RUN: go mod edit -go=1.22
      # rewrites the `go` directive to `go 1.22`; no toolchain directive added
  - VERIFY go.mod content is EXACTLY:
        module github.com/dustin/stagehand

        go 1.22
    (blank line between module and go; trailing newline; NO require/toolchain block)
  - WHY TWO STEPS: `go mod init -go=1.22` is NOT a valid flag in go1.26.4 (verified, exit 2).
  - NAMING: module path MUST be exactly `github.com/dustin/stagehand` (lowercase, no scheme/https).
  - DO NOT: run `go get`, `go mod tidy`, or add any imports — no deps in this subtask.

Task 2: CREATE the full §14 directory tree (empty package directories)
  - RUN (single command):
      mkdir -p cmd/stagehand \
               internal/config internal/provider internal/prompt \
               internal/git internal/generate internal/ui \
               pkg/stagehand providers docs
  - VERIFY: `find cmd internal pkg providers docs -type d` lists all 10 leaf dirs.
  - DO NOT: create any .go files in these dirs yet. Do NOT add .gitkeep files.
  - NOTE: These dirs will be untracked by git until later subtasks add source. Expected.

Task 3: CREATE cmd/stagehand/main.go (the only Go source file)
  - CREATE file: cmd/stagehand/main.go
  - CONTENT (exact — gofmt-verified canonical form):
        package main

        func main() {}
  - NAMING: package `main` (required for an executable in cmd/stagehand).
  - PLACEMENT: cmd/stagehand/main.go per PRD §14 (entrypoint dir).
  - DO NOT: add cobra, flags, or logic — those are P1.M4 (CLI layer). Stub only.
  - WHY: a package dir with ≥1 .go file is what `go build ./...` actually compiles.

Task 4: MODIFY .gitignore (augment, do not replace)
  - READ existing .gitignore at repo root (already present with build/env/OS/editor entries).
  - PRESERVE every existing line (do not delete /dist/, /build/, .env*, .DS_Store, *.swp, etc.).
  - APPEND (or merge in) these THREE missing contract-required entries (verify each is absent first):
        # Go build / test artifacts (Stagehand)
        /bin/
        *.test
        coverage.out
  - NOTE: `/dist/` is already present and satisfies the contract's `dist/` requirement — leave as-is.
  - VERIFY each of the 4 contract entries resolves present:
      grep -E '^(/bin/|\*\.test|coverage\.out|/dist/)$' .gitignore   # → 4 lines
  - FORBIDDEN: do NOT add plan/, PRD.md, prd_snapshot.md, or tasks.json to .gitignore.

Task 5: VALIDATE the build (run all gates below; all must pass before declaring done)
  - RUN: go build ./...     # expected exit 0
  - RUN: go vet ./...       # expected exit 0
  - RUN: go list ./...      # expected output: github.com/dustin/stagehand/cmd/stagehand
  - RUN: gofmt -l .         # expected: empty (no unformatted files)
  - RUN: go env GOMOD       # expected: /home/dustin/projects/stagehand/go.mod
  - FIX-FORWARD: if any gate fails, read the message, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```bash
# === The canonical, verified command sequence (copy-pasteable) ===
cd /home/dustin/projects/stagehand          # repo root

# 1. Module (two steps — see gotcha: `go mod init -go=1.22` is invalid)
go mod init github.com/dustin/stagehand
go mod edit -go=1.22

# 2. Directory tree (PRD §14) — all empty for now
mkdir -p cmd/stagehand \
         internal/config internal/provider internal/prompt \
         internal/git internal/generate internal/ui \
         pkg/stagehand providers docs

# 3. The single Go source file (stub)
cat > cmd/stagehand/main.go <<'EOF'
package main

func main() {}
EOF

# 4. .gitignore — append the three missing entries (keep existing content)
#    (use an editor to merge these in; shown here as an append for reference only)
cat >> .gitignore <<'EOF'

# Go build / test artifacts (Stagehand)
/bin/
*.test
coverage.out
EOF

# 5. Validate
go build ./... && go vet ./... && gofmt -l . && go list ./...
```

```go
// cmd/stagehand/main.go — the ENTIRE Go source for this subtask.
// Deliberately a no-op. Real wiring (cobra root cmd, flags, GenerateCommit call)
// arrives in P1.M4.T1. Keeping it empty here means no deps and a trivial review.
package main

func main() {}
```

### Integration Points

```yaml
MODULE:
  - file: go.mod
  - module path: "github.com/dustin/stagehand"   # all future import paths derive from this
  - go directive: "1.22"                          # PRD §22.3 floor; no toolchain line
  - dependencies: NONE (cobra + go-toml added in M1.T4 / M4.T1)

GITIGNORE:
  - file: .gitignore (repo root, MODIFIED not replaced)
  - add: "/bin/"        # §21.1: make build → ./bin/stagehand
  - add: "*.test"       # go test -c binaries
  - add: "coverage.out" # §20.3 coverage target output
  - keep: "/dist/" (already satisfies contract's dist/), plus existing entries
  - NEVER ignore: plan/, PRD.md, prd_snapshot.md, tasks.json

LATER-SUBTASK HOOKS (informational — do NOT implement now):
  - P1.M1.T1.S2: adds Makefile (build/test/lint/coverage targets using ./bin/)
  - P1.M1.T2:    fills internal/git/*.go
  - P1.M1.T4:    fills internal/config/*.go + adds go-toml dep
  - P1.M2:       fills internal/provider/*.go + providers/*.toml
  - P1.M3.T5:    fills pkg/stagehand/stagehand.go (public API)
  - P1.M4.T1:    rewrites cmd/stagehand/main.go with cobra + adds cobra dep
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating main.go — must be clean before proceeding
cd /home/dustin/projects/stagehand

gofmt -l .                    # Expected: no output (all files formatted)
go vet ./...                  # Expected: exit 0, no warnings

# Expected: Zero output/errors. If gofmt lists files, run `gofmt -w cmd/stagehand/main.go`.
```

### Level 2: Build (Module Validation)

```bash
cd /home/dustin/projects/stagehand

go build ./...                # Expected: exit 0, no output, produces no error
go list ./...                 # Expected: prints `github.com/dustin/stagehand/cmd/stagehand`
                             # (empty package dirs are correctly skipped — verified empirically)
go env GOMOD                  # Expected: /home/dustin/projects/stagehand/go.mod

# Confirm go.mod is minimal & correct
cat go.mod
# Expected:
#   module github.com/dustin/stagehand
#
#   go 1.22
# (NO toolchain line, NO require block)

# Confirm no go.sum was generated (no deps)
test ! -f go.sum && echo "OK: no go.sum"   # Expected: OK: no go.sum
```

### Level 3: .gitignore Integrity

```bash
cd /home/dustin/projects/stagehand

# All four contract-required entries present (expect 4 matches)
grep -nE '^(/bin/|\*\.test|coverage\.out|/dist/)$' .gitignore | wc -l   # Expected: 4

# Negative test — forbidden ignores must NOT be present
! grep -qE '(^|/)plan/?$'    .gitignore && echo "OK: plan/ not ignored"
! grep -qE 'PRD\.md'         .gitignore && echo "OK: PRD.md not ignored"
! grep -qE 'tasks\.json'     .gitignore && echo "OK: tasks.json not ignored"
! grep -qE 'prd_snapshot'    .gitignore && echo "OK: prd_snapshot not ignored"

# Directory tree present (10 leaf dirs)
find cmd internal pkg providers docs -type d | sort
# Expected to include: cmd/stagehand, internal/{config,provider,prompt,git,generate,ui},
#                      pkg/stagehand, providers, docs
```

### Level 4: Clean-Room Build (Optional but Recommended)

```bash
# Prove a fresh clone builds identically (simulates reviewer environment)
cd /tmp && rm -rf sh_clean && mkdir sh_clean && cd sh_clean
cp -r /home/dustin/projects/stagehand/go.mod .
cp -r /home/dustin/projects/stagehand/cmd .
go build ./...            # Expected: exit 0 (offline, no deps, no go.sum needed)
echo "Clean-room build: PASS"
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0 (run it, confirm).
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go list ./...` prints only `github.com/dustin/stagehand/cmd/stagehand`.
- [ ] `go env GOMOD` points at the repo-root go.mod.

### Feature Validation

- [ ] `go.mod` contains exactly `module github.com/dustin/stagehand` and `go 1.22` (no toolchain/require).
- [ ] All 10 §14 leaf directories exist on disk.
- [ ] `cmd/stagehand/main.go` is the stub (`package main` + empty `func main()`).
- [ ] `.gitignore` has `/bin/`, `*.test`, `coverage.out`, `/dist/` (4 grep matches).
- [ ] `.gitignore` does NOT ignore `plan/`, `PRD.md`, `prd_snapshot.md`, `tasks.json`.
- [ ] No dependencies added; no `go.sum` file generated.

### Scope Discipline Validation

- [ ] Did NOT add cobra or go-toml (those are M4.T1 / M1.T4).
- [ ] Did NOT create a Makefile (that is S2 / next subtask).
- [ ] Did NOT add stub `.go` files to `internal/*` or `pkg/stagehand` (later subtasks own those).
- [ ] Did NOT create `providers/*.toml`, `docs/PRD.md`, `.goreleaser.yaml`, or `README.md`.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't run `go mod init -go=1.22` — the flag doesn't exist in go1.26.4 (verified). Use the two-step init+edit.
- ❌ Don't add a `toolchain` directive to go.mod — keep it minimal per the contract.
- ❌ Don't create stub `.go` files in `internal/*` or `pkg/stagehand` — empty dirs are skipped cleanly by the build (verified); real files arrive in their own subtasks.
- ❌ Don't add `.gitkeep` files — git untracking of empty dirs is expected and harmless here.
- ❌ Don't `go get`/`go mod tidy` or add any imports — this subtask is dependency-free.
- ❌ Don't replace the existing `.gitignore` wholesale — AUGMENT it (preserve /dist/, .env*, OS/editor entries).
- ❌ Don't ignore `plan/`, `PRD.md`, `prd_snapshot.md`, or `tasks.json` (forbidden).
- ❌ Don't create the Makefile, README.md, goreleaser, or providers/*.toml — out of scope for S1.
- ❌ Don't hand-write go.mod when `go mod init` + `go mod edit` produce it canonically.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a deterministic scaffolding task. The two non-obvious traps — (1) the invalid
`-go` flag on `go mod init`, and (2) empty package directories being silently skipped rather than
erroring — are both empirically verified and explicitly documented with the exact fix and copy-pasteable
command sequence. The directory list, module path, go version, and .gitignore entries are all pinned
to specific values. The only residual uncertainty (not 10/10) is human error in merging the .gitignore
rather than overwriting it — mitigated by explicit "MODIFY not replace" + grep verification gates.
