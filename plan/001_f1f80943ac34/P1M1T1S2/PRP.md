---
name: "P1.M1.T1.S2 — Create Makefile with build/test/lint/coverage targets"
description: |
  Add the repository `Makefile` (PRD §21.1) exposing six developer/CI targets — `build`, `test`,
  `lint`, `coverage`, `install`, `clean` — plus a `VERSION` variable (`dev` default) wired into the
  `-ldflags "-X main.version=…"` injection pattern. `make build` produces `./bin/stagecoach`; this is
  the entry point for every later subtask's validation loop and the CI matrix (§20.4). Pure build
  tooling — no user-facing surface, no dependencies, no Go source changes.
---

## Goal

**Feature Goal**: Provide a single, conventional `Makefile` at the repo root that gives every
Stagecoach contributor and the CI pipeline one-command access to build, test (with `-race`),
report coverage, lint, install, and clean — with version stamping ready for release via ldflags.

**Deliverable**: One new file — `Makefile` (repo root) — containing exactly six targets
(`build`, `test`, `lint`, `coverage`, `install`, `clean`) plus a conventional `help` target, a
`VERSION ?= dev` variable feeding `-X main.version=$(VERSION)` through `LDFLAGS`, and proper
`.PHONY` declarations. No other files touched.

**Success Definition**: `make build` produces an executable `./bin/stagecoach`; `make test` runs
`go test -race ./...` and exits 0; `make coverage` writes `coverage.out` and prints the
per-function coverage table; `make install` places the binary in `$GOPATH/bin`; `make clean`
removes `bin/ coverage.out dist/`; `make lint` is wired to `golangci-lint run` (verified via
dry-run since the binary is absent from this dev box). `make` with no argument defaults to `build`.

## User Persona

**Target User**: The Stagecoach contributor (developers of subtasks T2–T5, M2–M5) and the CI runner.

**Use Case**: Every later subtask's "Validation Loop" section references `make build` / `make test`
etc. Contributors type `make <target>` instead of remembering long `go build -ldflags …` invocations;
CI invokes the same targets so local and CI behavior are identical.

**User Journey**: `git clone` → `make build` → run `./bin/stagecoach` → `make test` before pushing.
At release: `make build VERSION=v1.2.3` (or goreleaser sets `VERSION`) → the stamped binary reports
its version via `main.version`.

**Pain Points Addressed**: Eliminates per-developer command drift (different flags, wrong output
paths); guarantees the `-ldflags` version-injection convention is encoded in one canonical place.

## Why

- **PRD §21.1 is explicit:** `make build` → `./bin/stagecoach`, plus `make test`, `make lint`,
  `make coverage`, and version injection via `-ldflags "-X main.version=…"`. This subtask delivers
  exactly that surface.
- **Foundation for the entire validation strategy.** Every subsequent subtask's PRP "Validation
  Loop" (Levels 1–4) is written against `go build ./...` / `go test ./...`; the Makefile is the
  friendly wrapper around those commands plus the canonical output path (`./bin/stagecoach`).
- **Locks the version-injection contract now** so P1.M4.T1 (which adds `var version string` to
  `main.go`) and P1.M5.T3.S2 (goreleaser) both have a single, agreed mechanism to plug into.
- **Prerequisite for CI (P1.M5.T3.S1):** the GitHub Actions matrix invokes `make build`, `make
  test`, `make lint` — those targets must exist before the workflow file references them.
- **No user-facing surface (PRD "DOCS: none")** — build tooling only.

## What

A `Makefile` with the six targets below. Every target is a thin, conventional wrapper over the
documented Go toolchain commands — no logic, no conditionals, no external dependencies beyond `go`
itself (plus `golangci-lint` for `lint`, which is assumed present per standard Go convention and
installed by CI / locally on demand).

### The six targets (contract-specified)

| Target | Command | Contract source |
|--------|---------|-----------------|
| `build` | `go build -ldflags "$(LDFLAGS)" -o bin/stagecoach ./cmd/stagecoach` | §21.1 base + "VERSION for ldflags pattern" |
| `test` | `go test -race ./...` | "go test ./…" + "Use `go test -race` in the test target" |
| `lint` | `golangci-lint run` | "golangci-lint run" + §20.4 |
| `coverage` | `go test -coverprofile=coverage.out ./… && go tool cover -func=coverage.out` | contract verbatim |
| `install` | `go install -ldflags "$(LDFLAGS)" ./cmd/stagecoach` | "go install ./cmd/stagecoach" |
| `clean` | `rm -rf bin/ coverage.out dist/` | contract verbatim |

> **`build` uses ldflags — see "Why VERSION is not dead code" under Gotchas.** The `-o bin/stagecoach
> ./cmd/stagecoach` tail is preserved exactly as the contract states; `-ldflags` is added because the
> contract separately instructs "Add a `VERSION` variable defaulting to `dev` for the ldflags pattern"
> and §21.1 says version is injected via ldflags. Without ldflags in `build`, `VERSION` would be
> unreferenced dead code.

### Success Criteria

- [ ] `Makefile` exists at repo root.
- [ ] Targets present: `build`, `test`, `lint`, `coverage`, `install`, `clean` (plus `help`).
- [ ] Every recipe line begins with a **TAB** (not spaces) — `make` fails with `missing separator` otherwise.
- [ ] `VERSION ?= dev` defined; `LDFLAGS := -X main.version=$(VERSION)`.
- [ ] `build` & `install` pass `-ldflags "$(LDFLAGS)"`.
- [ ] `test` uses `-race`.
- [ ] `.PHONY` lists all targets (none are file-based).
- [ ] `make build` produces an executable `./bin/stagecoach` (exit 0).
- [ ] `make test` exits 0 (even with zero test files — verified).
- [ ] `make coverage` writes `coverage.out` and prints the `-func` table (exit 0).
- [ ] `make clean` removes `bin/`, `coverage.out`, `dist/`.
- [ ] `make -n lint` prints `golangci-lint run` (dry-run; binary not installed locally — see Gotchas).
- [ ] No Go source files, no `go.mod`, no `.gitignore`, no other files modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP states the exact module/binary paths, the complete Makefile content
(copy-pasteable with a heredoc that preserves TABs), every empirically-verified gotcha (ldflags
no-op on a missing symbol, no-test-files exit 0, golangci-lint absence, the TAB-vs-spaces trap), and
the exact validation commands with expected exit codes. No inference required.

### Documentation & References

```yaml
# MUST READ — the spec sources for the targets and the version pattern
- file: PRD.md
  why: "§21.1 Build: make build → ./bin/stagecoach; make test/lint/coverage; version via -ldflags -X main.version. §20.4 CI matrix lists golangci-lint/govulncheck (goreleaser=§21.2). §21.4 Versioning: semver; v1.0.0 = feature-complete."
  critical: "§21.1 is the literal contract for these targets. Do NOT add govulncheck/release/coverage-gate targets — they belong to P1.M5.T3 (CI/release). This subtask owns ONLY the 6 targets + VERSION var."

- docfile: plan/001_f1f80943ac34/P1M1T1S2/research/makefile_verification.md
  why: "EMPIRICALLY VERIFIED toolchain behaviors: ldflags -X is a silent no-op on a missing symbol (exit 0); go test -race/coverprofile exit 0 with zero test files; go install lands in $GOPATH/bin; golangci-lint is MISSING from this box (use make -n lint). Read before running any make command."
  critical: "The #1 failure mode is TAB-vs-spaces in recipe lines. Use the provided heredoc; verify with grep -P '^\t' Makefile."

- docfile: plan/001_f1f80943ac34/P1M1T1S1/PRP.md
  why: "The CONTRACT for the inputs this Makefile consumes: module path github.com/dustin/stagecoach, go 1.22, entrypoint at ./cmd/stagecoach (stub main.go), .gitignore already lists /bin/ *.test coverage.out /dist/."
  critical: "S1 is being implemented IN PARALLEL. The Makefile FILE can be created now, but VALIDATION (make build/test/coverage) requires S1's go.mod + cmd/stagecoach/main.go to already exist. If validating before S1 lands, run the targets in a throwaway module per the research note, or sequence after S1."

- docfile: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  why: "Appendix B shows the target go.mod shape (module + go 1.22) and confirms the -ldflags -X main.version convention used by goreleaser-compatible builds."
  section: "Appendix B (go.mod), and the ldflags/version context"

# External references (exact, anchor-level)
- url: https://pkg.go.dev/cmd/link
  why: "Documents the -X linker flag semantics: -X importpath.name=value sets a package-level string var; it is a SILENT NO-OP if the symbol does not exist (verified empirically)."
  critical: "This is WHY build can ship ldflags before main.go declares var version — no conditional gating needed."
- url: https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
  why: ".PHONY declaration semantics — all targets here are non-file, must be declared phony so they always run."
- url: https://golangci-lint.run/usage/install/
  why: "Install golangci-lint locally if you want to actually execute `make lint` (the binary is absent from this dev box). CI installs it via the golangci-lint-action."
```

### Current Codebase Tree (after P1.M1.T1.S1 lands)

```bash
stagecoach/
├── .gitignore            # from S1 — already has /bin/ *.test coverage.out /dist/
├── PRD.md
├── go.mod                # from S1 — module github.com/dustin/stagecoach, go 1.22
├── cmd/stagecoach/main.go # from S1 — stub: package main; func main(){}
├── internal/{config,provider,prompt,git,generate,ui}/   # from S1 — empty dirs
├── pkg/stagecoach/        # from S1 — empty
├── providers/            # from S1 — empty
├── docs/                 # from S1 — empty
└── plan/                 # unchanged
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
├── ... (everything above, unchanged)
└── Makefile              # NEW — this subtask's ONLY deliverable
```

**File responsibilities:**
| Path | Action | Responsibility |
|---|---|---|
| `Makefile` | NEW (repo root) | The 6 targets + help + VERSION/LDFLAGS vars. Nothing else. |

**Explicitly NOT created/modified now** (other subtasks own these): `.golangci.yml`, CI workflow
files (`.github/workflows/`), `.goreleaser.yaml`, `README.md`, coverage-gate logic (≥85%),
govulncheck wiring, any `*.go` source.

### Known Gotchas of our codebase & toolchain

```makefile
# CRITICAL (#1 failure mode): Makefile recipe lines MUST start with a TAB (U+0009), not spaces.
#   Symptom of spaces: `Makefile:5: *** missing separator.  Stop.`
#   Fix: use the heredoc creation command below (heredocs preserve tabs literally).
#   Verify: `grep -Pc '^\t' Makefile` must equal the number of recipe lines (7).

# CRITICAL: ldflags -X main.version=dev is a SILENT NO-OP until main.go declares `var version string`
#   (that arrives in P1.M4.T1). VERIFIED: `go build -ldflags "-X main.version=dev" -o bin/x .` exits 0
#   even with an empty main.go. The Go linker only errors if the symbol EXISTS but is not a string.
#   → Ship ldflags NOW; it becomes effective the moment a later subtask adds `var version = "dev"`.

# CRITICAL: golangci-lint is NOT installed on this dev box (go1.26.4, GNU Make 4.4.1 only).
#   The `lint` target MUST still call `golangci-lint run` (contract + PRD §20.4); do NOT stub it
#   or gate it behind `command -v`. CI (P1.M5.T3.S1) and local devs install it separately.
#   For validation here, use `make -n lint` (dry-run prints the command without executing).

# GOTCHA: VERSION uses `?=` (recursive assignment with default) NOT `:=` — so release tooling and
#   `make build VERSION=v1.2.3` can override it. `:=` would freeze the default and break release injection.

# GOTCHA: `go test -race ./...` and `go test -coverprofile=coverage.out ./...` both EXIT 0 even when
#   zero *_test.go files exist (prints "[no test files]"). VERIFIED. So `make test` / `make coverage`
#   are safe to run immediately after scaffolding; coverage ramps up in M1.T2+.

# GOTCHA: `go install ./cmd/stagecoach` writes to $GOPATH/bin (= ~/go/bin when GOBIN unset), NOT ./bin.
#   That is correct/expected for the `install` target — it differs from `build` (which writes ./bin).

# WHY VERSION is not dead code: the contract states both "build (go build -o bin/stagecoach ./cmd/stagecoach)"
# AND "Add a VERSION variable defaulting to dev for the ldflags pattern" + §21.1 "version injected via
# -ldflags at release". A VERSION var that build never reads would be dead code. Coherent reading:
# build = the base command PLUS -ldflags "$(LDFLAGS)". The -o and ./cmd/stagecoach args are preserved verbatim.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE the Makefile at repo root (EXACT content below — recipe lines use TABs)
  - FILE: /home/dustin/projects/stagecoach/Makefile
  - CREATE via heredoc (preserves TABs literally — DO NOT substitute spaces for the leading \t):
      cat > Makefile <<'EOF'
      <exact content from "The Canonical Makefile" below>
      EOF
  - NAMING: filename exactly `Makefile` (capital M, no extension). NOT `makefile` or `GNUmakefile`.
  - PLACEMENT: repo root (alongside go.mod). NOT in cmd/ or a subdir.
  - VERIFY TABS: `grep -Pc '^\t' Makefile` → must print 8 (one per recipe line).
  - DEPENDENCY (for validation only): requires P1.M1.T1.S1's go.mod + cmd/stagecoach/main.go to exist.
    If S1 hasn't landed, the FILE can still be created; defer the make-build/test/coverage runs.

Task 2: VALIDATE — run each target and confirm the documented exit code
  - RUN (in repo root, after S1 has landed go.mod + main.go):
      make -n lint                 # dry-run → must print: golangci-lint run   (no execution)
      make build                   # exit 0; produces ./bin/stagecoach (executable)
      test -x ./bin/stagecoach && echo OK   # binary is executable
      make test                    # exit 0 (prints "[no test files]" — fine)
      make coverage                # exit 0; writes coverage.out; prints the -func table
      make install                 # exit 0; binary at $GOPATH/bin/stagecoach
      make clean                   # exit 0; bin/ coverage.out dist/ removed
      make                         # no-arg → runs build (default goal); produces ./bin/stagecoach
      make build VERSION=v9.9.9    # exit 0; confirms VERSION override works
  - FOR lint (real run, optional): install golangci-lint first:
      # see https://golangci-lint.run/usage/install/  (binary installer recommended over `go install`)
  - FIX-FORWARD: if `make <target>` errors, the cause is almost always (a) TAB→spaces, or
    (b) S1 not yet landed. Read the error, fix, re-run. Do NOT skip.

Task 3: SCOPE-CHECK — confirm no collateral files were created/modified
  - RUN: `git status --porcelain` → expect ONLY `?? Makefile` (and S1's own outputs if it landed).
  - RUN: `git diff -- go.mod .gitignore cmd/` → expect NOTHING from THIS subtask.
  - DO NOT create .golangci.yml, .github/workflows/*, .goreleaser.yaml, README.md, or any *.go file.
```

### The Canonical Makefile (EXACT content — recipe lines begin with TAB)

> **Use the `write` tool with this content verbatim, OR the heredoc in Task 1.** Every line under a
> target that starts a recipe (the lines beginning with `go …`, `golangci-lint …`, `rm …`, `@grep …`)
> **MUST start with a TAB character**, shown here as `→` for visibility — replace each `→` with a real
> TAB (U+0009). Comments (lines starting with `#`) and variable assignments may use spaces.

```makefile
## Stagecoach — build, test, lint, coverage, and install targets. (PRD §21.1)
##
## Usage:  make <target>
##
## Targets:
##   build      Compile the stagecoach binary to ./bin/stagecoach
##   install    Install stagecoach into $GOPATH/bin
##   test       Run all tests with the race detector enabled
##   coverage   Run tests and print per-function coverage
##   lint       Run golangci-lint
##   clean      Remove bin/, coverage.out, and dist/
##   help       Print this help

.DEFAULT_GOAL := build

# --- Version (PRD §21.1, §21.4) -------------------------------------------------
# Injected into main.version via -ldflags at build time. Defaults to "dev";
# override for releases:  make build VERSION=v1.2.3   (goreleaser sets it via env).
# NOTE: -X main.version=... is a silent no-op until main.go declares `var version string`
# (a later subtask adds it). VERIFIED: build exits 0 either way.
VERSION ?= dev

# --- Paths & flags --------------------------------------------------------------
BIN_DIR  := bin
BIN      := $(BIN_DIR)/stagecoach
MAIN_PKG := ./cmd/stagecoach
LDFLAGS  := -X main.version=$(VERSION)

.PHONY: build install test coverage lint clean help

build: ## Compile the stagecoach binary to ./bin/stagecoach
→	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(MAIN_PKG)

install: ## Install stagecoach into $GOPATH/bin
→	go install -ldflags "$(LDFLAGS)" $(MAIN_PKG)

test: ## Run all tests with the race detector enabled
→	go test -race ./...

coverage: ## Run tests and print per-function coverage
→	go test -coverprofile=coverage.out ./...
→	go tool cover -func=coverage.out

lint: ## Run golangci-lint
→	golangci-lint run

clean: ## Remove bin/, coverage.out, and dist/
→	rm -rf $(BIN_DIR) coverage.out dist/

help: ## Print this help
→	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
```

**IMPORTANT — convert the `→` markers to real TABs.** A copy-paste that turns the tabs into spaces
will break with `missing separator`. After writing the file, the tab verification in Task 2
(`grep -Pc '^\t' Makefile`) must return **8** (build=1 + install=1 + test=1 + coverage=2 + lint=1 +
clean=1 + help=1 = 8).

### Implementation Patterns & Key Details

```makefile
# === Why `?=` and not `:=` for VERSION ===
# `VERSION ?= dev` assigns ONLY if VERSION is unset, so `make build VERSION=v1.0.0` (and goreleaser's
# exported VERSION) can override it. `:= dev` would ignore any override and break release stamping.

# === Why `-ldflags "$(LDFLAGS)"` with double-quotes ===
# LDFLAGS expands to `-X main.version=dev`. It contains an `=` and could contain spaces if more
# flags are added later. Quoting it keeps it a single argv element to the linker.

# === Why .PHONY ===
# `make build` produces a file named `bin/stagecoach`, and there's no source file literally named
# `build`/`test`/etc., but declaring them PHONY guarantees make re-runs them every invocation and
# never mistakes a same-named file for an up-to-date target. Standard for Go Makefiles.

# === Why -race only in `test`, not `coverage` ===
# The contract specifies `-race` for the test target only and defines coverage as
# `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` (no -race).
# Follow the contract literally; do not "improve" coverage by adding -race.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" (from S1's go.mod) → MAIN_PKG ./cmd/stagecoach resolves
  - entrypoint:  ./cmd/stagecoach (from S1) → build/install targets reference it
  - go directive: 1.22 (from S1) → all go subcommands work under the 1.26.4 toolchain

GITIGNORE (consumed, not modified):
  - S1 already added /bin/, *.test, coverage.out, /dist/ → build/coverage/clean outputs are ignored
  - `make clean` removes bin/, coverage.out, dist/ which are ALL gitignored — no tracked files harmed

LATER-SUBTASK HOOKS (informational — do NOT implement now):
  - P1.M4.T1: adds `var version = "dev"` to main.go → ldflags -X becomes effective immediately
  - P1.M5.T3.S1: GitHub Actions invokes `make build && make test && make lint` (targets must exist)
  - P1.M5.T3.S2: goreleaser sets VERSION env → `make build` stamps the release version
  - P1.M5.T3.S3: adds a coverage GATE (≥85% on core pkgs) — may add a new `coverage-check` target THEN;
                 this subtask's `coverage` only RUNS + REPORTS (no gate)
```

## Validation Loop

### Level 1: Makefile Syntax (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# TAB integrity — every recipe line must start with a real TAB (expect 8)
grep -Pc '^\t' Makefile                       # Expected: 8

# All seven targets parse (dry-run each — prints the command, no execution)
make -n build && make -n test && make -n coverage && make -n lint && make -n install && make -n clean
# Expected: each prints its command(s); exit 0.

# Confirm no "missing separator" errors (a real parse, not dry-run)
make help                                     # Expected: prints the target table; exit 0
# Expected: Zero errors. If "missing separator.  Stop." → a recipe line uses spaces, not a TAB.
```

### Level 2: Build & Test Targets (requires P1.M1.T1.S1 to have landed)

```bash
cd /home/dustin/projects/stagecoach
# Precondition: go.mod + cmd/stagecoach/main.go exist (P1.M1.T1.S1 delivered them).

make build                                    # Expected: exit 0
test -x ./bin/stagecoach && echo "binary OK"   # Expected: binary OK (executable)
./bin/stagecoach; echo "run exit=$?"           # Expected: exit 0 (no-op stub)

make test                                     # Expected: exit 0 (prints "[no test files]")

# VERSION override round-trips through ldflags
make build VERSION=v9.9.9                     # Expected: exit 0 (rebuilds with that version)

make                                          # Expected: runs `build` (default goal); exit 0
```

### Level 3: Coverage, Install, Clean, Lint

```bash
cd /home/dustin/projects/stagecoach

make coverage                                 # Expected: exit 0; creates coverage.out
test -f coverage.out && echo "coverage OK"    # Expected: coverage OK
go tool cover -func=coverage.out              # Expected: prints table (0.0% until tests land in M1.T2+)

make install                                  # Expected: exit 0
test -x "$(go env GOPATH)/bin/stagecoach" && echo "install OK"   # Expected: install OK

make clean                                    # Expected: exit 0
test ! -e bin && test ! -e coverage.out && echo "clean OK"      # Expected: clean OK

# lint: golangci-lint is NOT installed on this box → dry-run to confirm wiring
make -n lint                                  # Expected: prints exactly "golangci-lint run"
# Optional real run (install first):
#   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
#   make lint        # Expected: exit 0 (no Go files to lint yet, or only the stub)
```

### Level 4: Scope & Conventions

```bash
cd /home/dustin/projects/stagecoach

# ONLY the Makefile was added by this subtask
git status --porcelain                        # Expected (this subtask's contribution): "?? Makefile"
                                              # (S1's own outputs may also appear if it landed in parallel)

# Forbidden collateral files must NOT exist from this subtask
for f in .golangci.yml .goreleaser.yaml README.md; do
  test ! -e "$f" && echo "OK: $f not created"
done
test ! -d .github/workflows && echo "OK: no CI workflow files (that's P1.M5.T3.S1)"
```

## Final Validation Checklist

### Technical Validation

- [ ] `grep -Pc '^\t' Makefile` returns `8` (all recipe lines are TAB-indented).
- [ ] `make help` exits 0 and prints the target table (no `missing separator`).
- [ ] `make build` exits 0 and produces executable `./bin/stagecoach`.
- [ ] `make test` exits 0.
- [ ] `make coverage` exits 0, writes `coverage.out`, prints the `-func` table.
- [ ] `make install` exits 0; binary present at `$(go env GOPATH)/bin/stagecoach`.
- [ ] `make clean` exits 0; `bin/`, `coverage.out`, `dist/` are gone.
- [ ] `make -n lint` prints exactly `golangci-lint run`.
- [ ] `make` (no arg) runs `build` (default goal).

### Feature Validation

- [ ] All six contracted targets exist: `build`, `test`, `lint`, `coverage`, `install`, `clean`.
- [ ] `VERSION ?= dev` defined; `LDFLAGS := -X main.version=$(VERSION)`.
- [ ] `build` and `install` pass `-ldflags "$(LDFLAGS)"`.
- [ ] `test` passes `-race`.
- [ ] `.PHONY` declares every target.
- [ ] `make build VERSION=v1.2.3` succeeds (override works).

### Scope Discipline Validation

- [ ] Created ONLY `Makefile` — no `.golangci.yml`, no CI workflow, no goreleaser config, no README.
- [ ] Did NOT add a govulncheck target (CI matrix owns it → P1.M5.T3.S1).
- [ ] Did NOT add a coverage ≥85% gate (→ P1.M5.T3.S3).
- [ ] Did NOT modify `go.mod`, `.gitignore`, `cmd/stagecoach/main.go`, or any file under `plan/`.
- [ ] Did NOT add Go source files or dependencies.

---

## Anti-Patterns to Avoid

- ❌ Don't indent recipe lines with spaces — `make` requires TABs (`missing separator` error). Use the heredoc.
- ❌ Don't use `:=` for `VERSION` — it blocks release overrides; use `?=` so `VERSION=v1.2.3` and goreleaser work.
- ❌ Don't omit `-ldflags` from `build` — the `VERSION` variable exists solely to feed it; omitting it makes VERSION dead code.
- ❌ Don't add `-race` to the `coverage` target — the contract specifies `-race` for `test` only.
- ❌ Don't gate `lint` behind `command -v golangci-lint` or stub it to a no-op — CI/devs install the tool; the target must call `golangci-lint run` directly.
- ❌ Don't create `.golangci.yml`, `.github/workflows/*`, `.goreleaser.yaml`, govulncheck, or coverage-gate targets — other subtasks own them.
- ❌ Don't create the file as `makefile` (lowercase) or `GNUmakefile` — it must be exactly `Makefile`.
- ❌ Don't modify `go.mod`, `.gitignore`, or any `*.go` file — this subtask is build tooling only.
- ❌ Don't reference `main.version` in Go code yet — that's P1.M4.T1; ldflags is a silent no-op until then (verified).

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a single static file with a fully-specified, copy-pasteable body (heredoc form
preserves TABs). Every non-obvious behavior was empirically verified on the exact installed
toolchain (GNU Make 4.4.1 / go1.26.4): the ldflags `-X` no-op on a missing symbol (exit 0), the
no-test-files exit 0 for `test`/`coverage`, the `$GOPATH/bin` install destination, and the
documented absence of `golangci-lint` (mitigated with `make -n lint`). The only residual uncertainty
(not 10/10) is the parallel-execution sequencing: the Makefile can be authored before P1.M1.T1.S1
lands, but running `make build`/`test`/`coverage` requires S1's `go.mod` + `cmd/stagecoach/main.go`.
This is explicitly called out in the validation preconditions so the implementing agent sequences
correctly. The TAB-vs-spaces trap — the single most common Makefile authoring failure — is
front-loaded as the #1 gotcha with a heredoc fix and a `grep -Pc '^\t'` gate.
