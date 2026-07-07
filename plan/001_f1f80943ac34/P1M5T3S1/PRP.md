---
name: "P1.M5.T3.S1 — GitHub Actions CI matrix (build+test, golangci-lint, govulncheck, ≥85% coverage gate)"
description: |

  Ship `.github/workflows/ci.yml` (NEW) + `.golangci.yml` (NEW) — a GitHub Actions CI pipeline that
  runs on every push to `main` and every pull request, and gates merges. It has FOUR jobs:
  (1) `build-test` — a matrix over `os × Go` (`ubuntu-latest`, `macos-latest`, `macos-13`,
  `windows-latest` × Go `1.22`, `1.23`) that does `go build ./...` + `go test -race ./...`;
  (2) `lint` — `golangci-lint` via the official action, config in `.golangci.yml`;
  (3) `vulncheck` — `govulncheck ./...` via the official action;
  (4) `coverage` — `go test -coverprofile=coverage.out ./...` + a SELF-CONTAINED per-package coverage
  gate that FAILS the job if any of `internal/git`, `internal/provider`, `internal/generate`,
  `internal/config` is below 85 % (PRD §20.3).

  CONTRACT (P1.M5.T3.S1, verbatim):
    1. RESEARCH NOTE: "PRD §20.4 — GitHub Actions: build+test on {linux, macos, windows} × {amd64,
       arm64}, Go 1.22 and 1.23. golangci-lint. govulncheck. Release on tag via goreleaser. Coverage
       gate ≥85% on internal/git, provider, generate, config (§20.3)."
    2. INPUT: "Makefile from P1.M1.T1.S2."
    3. LOGIC: "Create .github/workflows/ci.yml with a matrix job (os: ubuntu-latest, macos-latest,
       windows-latest; go: 1.22, 1.23). Steps: checkout, setup-go, go test -race ./..., golangci-lint
       run, govulncheck ./.... Add a coverage gate step that fails if coverage < 85% on the target
       packages. Mock: none — CI config validated by running act or pushing to a branch."
    4. OUTPUT: "A CI pipeline that runs on every push/PR across the full OS×arch×Go matrix."
    5. DOCS: "none — CI config."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `.goreleaser.yaml` + release-on-tag job → S2 (P1.M5.T3.S2). §20.4 names goreleaser in the same
      bullet, but the orchestrator split it into S2. S1's ci.yml triggers on push(main)+PR ONLY and
      contains NO release/tag/goreleaser logic. Do not create release.yml, do not add a `on: push:
      tags:` trigger.
    - The `make coverage-gate` Makefile TARGET → S3 (P1.M5.T3.S3). S1's coverage gate lives INLINE in
      ci.yml (self-contained awk). Do NOT add a Makefile target; do NOT have ci.yml call a Makefile
      coverage target that does not exist yet. (A future micro-refactor to delegate ci.yml →
      `make coverage-gate` is out of S1's scope.)
    - `Makefile` → UNCHANGED. CI MIRRORS the existing targets (`build`/`test`/`coverage`/`lint`); it
      does not edit them.
    - `PRD.md`, `tasks.json`, `.gitignore` → READ-ONLY (research agent never modifies these; and even
      at implementation time, `.gitignore` already covers `/bin/`, `coverage.out`, `/dist/`).

  DELIVERABLE (TWO new files, NO edits to existing files):
    CREATE .github/workflows/ci.yml     # 4 jobs: build-test(matrix), lint, vulncheck, coverage
    CREATE .golangci.yml                # conservative, deterministic linter set (so CI lint is stable)

  SUCCESS: ci.yml is valid YAML (passes `actionlint`); pushed to a branch, the Actions tab shows all
  four jobs green on a repo with `go test -race ./...` green and coverage ≥85 % on the 4 core packages;
  `act` local dry-run of at least the `coverage` and `lint` jobs succeeds; no existing file modified.

---

## Goal

**Feature Goal**: Give Stagecoach a GitHub Actions CI pipeline (PRD §20.4) that, on every push to
`main` and every pull request, (a) builds and runs the race-enabled test suite across the full
`os × Go` matrix, (b) runs `golangci-lint`, (c) runs `govulncheck`, and (d) enforces the PRD §20.3
coverage gate (≥85 % on `internal/git`, `internal/provider`, `internal/generate`, `internal/config`).
The matrix maps PRD's `{linux, macos, windows} × {amd64, arm64}` intent onto GitHub-hosted runners in
the most honest, flake-free way possible (4 native combos + an arm64 cross-compile check).

**Deliverable**: Two new files at repo root — `.github/workflows/ci.yml` (the pipeline; 4 jobs) and
`.golangci.yml` (the deterministic lint config the `lint` job consumes). No edits to any existing file.
The coverage gate is a self-contained awk step INSIDE ci.yml (no third-party action, no Makefile
dependency) that parses `go test -coverprofile` output and fails the job on any sub-85 % core package.

**Success Definition**:
- `.github/workflows/ci.yml` + `.golangci.yml` exist; `actionlint .github/workflows/ci.yml` is clean.
- On push/PR the Actions tab runs four jobs: `build-test` (matrix), `lint`, `vulncheck`, `coverage` —
  all green on a tree where `go test -race ./...` is green and the 4 core packages are ≥85 %.
- The matrix exercises linux/amd64, macos/amd64, macos/arm64, windows/amd64 (native), plus a
  cross-compile check for linux/arm64 and windows/arm64.
- `govulncheck ./...` is clean; `golangci-lint run` is clean under `.golangci.yml`.
- The coverage gate FAILS when a core package drops below 85 % (verifiable by temporarily lowering the
  threshold) and annotates the offending package via `::error::`.
- `git status --short` shows ONLY the 2 new files; the Makefile, go.mod, PRD.md are untouched.

## User Persona

**Target User**: the Stagecoach **contributor / maintainer** (the person whose PR is gated) and the
**release engineer** (who trusts CI green before tagging). Indirectly, every end user who installs a
build that CI has validated across the matrix.

**Use Case**: A contributor opens a PR. Within minutes, GitHub Actions reports whether the change
builds+tests on Linux/macOS/Windows, passes lint, has no known vulns, and did not regress coverage on
the four safety-critical packages. A red check blocks merge; the failing job pinpoints the cause
(matrix cell, lint finding, vuln, or the specific package below 85 %).

**Pain Points Addressed**: Today there is NO CI (`.github/` does not exist), so regressions in
cross-platform behavior, lint drift, new CVEs in deps, or coverage erosion on `internal/git` (the
CAS/atomicity core, PRD §20.2) land silently. This pipeline makes all four visible and blocking.

## Why

- **Realizes PRD §20.4 + §20.3.** §20.4 mandates the GitHub Actions matrix + golangci-lint +
  govulncheck; §20.3 mandates the ≥85 % coverage gate "enforced in CI." This task is that enforcement.
- **Protects the integrity-critical core.** `internal/git` owns `UpdateRefCAS` (PRD §18.1 invariant,
  risk "High — data integrity", §22.1). The coverage gate keeps its coverage (and that of
  provider/generate/config) from silently eroding.
- **Catches cross-platform breakage early.** Stagecoach shells out to `git` and agent CLIs on Linux,
  macOS, and Windows (PRD §22.2 "POSIX-ish" + Scoop install path §21.3). The OS matrix catches a
  Windows path-separator or a Unix-only syscall before a user hits it.
- **Shifts quality left (cheap).** lint + vuln + coverage on every PR is far cheaper than a post-
  release hotfix (the PRP "validation gates" philosophy).
- **Scope discipline**: this task ships the **CI** half of §20.4; the **release-on-tag (goreleaser)**
  half is S2, and the **Makefile** coverage target is S3. Keeping them separate lets each ship and
  validate independently.

## What

A two-file CI addition (see "Implementation Blueprint" for full content):

1. **`.github/workflows/ci.yml`** — triggers: `push: branches: [main]` + `pull_request:`. Four jobs:
   - `build-test` (the matrix): `os: [ubuntu-latest, macos-latest, macos-13, windows-latest]` ×
     `go: ['1.22', '1.23']`. Steps: `actions/checkout@v4`, `actions/setup-go@v5` (pinned go-version,
     `cache: true`), `go build ./...`, `go test -race ./...`. `fail-fast: false`.
   - `lint`: `golangci/golangci-lint-action@v6` reading `.golangci.yml` (ubuntu, one Go version).
   - `vulncheck`: `golang/govulncheck-action@v1` over `./...` (ubuntu, one Go version).
   - `coverage`: `go test -coverprofile=coverage.out ./...` then a self-contained awk gate that fails
     if any of the 4 core packages is < 85 %.
   - A concurrency group cancels superseded runs; `permissions: contents: read` (least privilege).
2. **`.golangci.yml`** — `disable-all: true` + a conservative enable list (errcheck, gosimple, govet,
   ineffassign, staticcheck, unused), `run.timeout: 5m`, `run.go: '1.22'`. Deterministic across runs.
3. **A cross-arch coverage note**: the `build-test` matrix gives native test execution on
   linux/amd64 (ubuntu-latest), macos/amd64 (macos-13), macos/arm64 (macos-latest), windows/amd64
   (windows-latest). linux/arm64 + windows/arm64 are verified by a build step that sets
   `GOARCH=arm64` (compile-only), because GitHub-hosted arm64 runners for those OSes are not reliably
   available and QEMU emulation is slow/flaky — out of v1 scope (documented).

### Success Criteria

- [ ] `.github/workflows/ci.yml` + `.golangci.yml` exist at repo root; no other file changed.
- [ ] `actionlint .github/workflows/ci.yml` exits 0 (or, if `actionlint` unavailable, the YAML parses
      with `python -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`).
- [ ] The `build-test` matrix enumerates exactly `ubuntu-latest, macos-latest, macos-13,
      windows-latest` × `1.22, 1.23` (8 cells) with `fail-fast: false`.
- [ ] `lint` job uses `golangci-lint-action@v6` and `.golangci.yml`; `make lint` is green locally
      (the implementer verifies — see "Lint gate is UNVERIFIED locally").
- [ ] `vulncheck` job runs `govulncheck` over `./...` and is green.
- [ ] `coverage` job fails if ANY of `internal/git`, `internal/provider`, `internal/generate`,
      `internal/config` is < 85 %, and annotates the offender via `::error::`. Demonstrate by
      temporarily setting the threshold to 99 % and confirming the job fails (then revert).
- [ ] ci.yml triggers ONLY on `push: branches: [main]` + `pull_request:`. NO `tags:` trigger, NO
      goreleaser, NO release job (those are S2).
- [ ] The Makefile is unchanged; ci.yml does not call any not-yet-existing Make target.
- [ ] Pushed to a branch, the Actions tab runs all four jobs green (or `act` dry-run of `lint` +
      `coverage` succeeds locally).

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the exact ci.yml + .golangci.yml
content (§"Implementation Blueprint", copy-pasteable); the Makefile targets CI mirrors (§"Documentation
& References"); the verified coverage baseline + the config-83 % decision tree (§"Known Gotchas"); the
runner→arch mapping table (§"Known Gotchas"); the exact coverage-gate awk (§"Implementation Patterns");
and the validation commands. The only genuinely uncertain inputs (current lint state; whether config
can be topped to 85 %) are called out as explicit decision points with sanctioned fallbacks.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T3S1/research/findings.md
  why: THE decisive doc. §2 the verified coverage baseline (config 83.3 % = the #1 risk); §3 the
       runner→arch mapping table; §4 the idiomatic 4-job split rationale; §5 the pinned action
       versions; §6/§7 the lint/vuln "UNVERIFIED locally" flags; §8 the S2/S3 scope boundaries.
  critical: §2 (config 83.3 % decision tree), §3 (arch mapping), §8 (do NOT add goreleaser/Makefile).

- file: Makefile   (P1.M1.T1.S2 — READ only; CI MIRRORS these targets, does not edit them)
  section: build (`go build -ldflags … -o bin/stagecoach ./cmd/stagecoach`), test (`go test -race ./...`),
           coverage (`go test -coverprofile=coverage.out ./...` + `go tool cover -func`), lint
           (`golangci-lint run`).
  why: ci.yml must produce the SAME checks the Makefile defines, so local `make test`/`make lint` and
       CI agree. The coverage step in ci.yml reuses the `go test -coverprofile=coverage.out ./...`
       invocation from `make coverage` and ADDS the gate (which `make coverage` lacks).
  pattern: CI jobs are the Makefile targets split into separate jobs + a gate. VERSION injection
           (-ldflags) is NOT needed in CI's `go build ./...` (that's a release concern = S2/goreleaser).
  gotcha: `make coverage` does NOT gate (it only prints `-func`). The gate is NEW (this task). Do not
          assume `make coverage` enforces anything.

- file: go.mod   (READ only)
  why: module path `github.com/dustin/stagecoach` (used verbatim in the coverage-gate package list) and
       `go 1.22` (the floor; matrix tests 1.22 AND 1.23). Deps are tiny (cobra, pflag, go-toml/v2,
       mousetrap) → govulncheck is very likely green.

- file: .gitignore   (READ only — do NOT edit)
  why: already ignores `/bin/`, `coverage.out`, `/dist/`. CI artifacts (coverage.out) won't be
       committed by accident. Confirms S1 must NOT touch .gitignore.

- url: https://github.com/actions/setup-go   (actions/setup-go@v5)
  why: the `go-version` input + `cache: true` (auto-keyed on go.sum, which exists). 
  section: "Supported version syntax" (use `'1.22'`/`'1.23'` → resolves to latest 1.22.x/1.23.x).

- url: https://github.com/golangci/golangci-lint-action   (v6)
  why: `version` (the golangci-lint binary) + `args` inputs. v6 installs its own binary.
  critical: v6 dropped some legacy inputs; `version` + `args: --timeout=5m` remain valid. Set
            `version: v1.61` (or `latest`) so the lint result is deterministic.

- url: https://github.com/golang/govulncheck-action   (v1)
  why: `go-version` + `go-package: ./...` inputs. Cleaner than `go run golang.org/x/vuln/...`.

- url: https://github.com/rhysd/actionlint   (static CI validator)
  why: BEST offline validator for ci.yml — catches matrix/expr/action-name typos before pushing.
       Install: `go install github.com/rhysd/actionlint/cmd/actionlint@latest`, then
       `actionlint .github/workflows/ci.yml`.

- url: https://github.com/nektos/act   (local runner; contract mentions "act")
  why: run ci.yml jobs locally in Docker. `act push -j coverage` (one job, one matrix cell via
       `--matrix os:ubuntu-latest`). Needs Docker.

- url: (PRD internal) PRD.md §20.3 (h3.70 coverage target), §20.4 (h3.71 CI matrix), §20.1 (h3.68 test
       layers — explains why the 4 core packages are the coverage gate targets), §21.1 (Makefile build
       targets CI mirrors), §22.3 (Go 1.22+ dependency).
  why: authoritative spec for the matrix, the 85 % target, and the 4 named packages.
```

### Current Codebase tree (relevant slice)

```bash
.github/                     # ← does NOT exist yet (greenfield)
  workflows/
    ci.yml                   # ← NEW (this task)
.golangci.yml                # ← NEW (this task)
Makefile                     # P1.M1.T1.S2 — build/test/coverage/lint targets. UNCHANGED (CI mirrors them).
go.mod / go.sum              # module github.com/dustin/stagecoach; go 1.22. UNCHANGED.
cmd/stagecoach/               # main binary (Makefile MAIN_PKG). UNCHANGED.
internal/{git,provider,generate,config}/   # ← the 4 PRD §20.3 coverage-gate packages. UNCHANGED.
PRD.md                       # READ-ONLY.
.gitignore                   # already ignores /bin/ coverage.out /dist/. UNCHANGED.
# (S2 will later add .goreleaser.yaml + a release job; S3 will later add a make coverage-gate target.
#  Neither exists yet; S1 does not create or reference either.)
```

### Desired Codebase tree with files to be added

```bash
.github/workflows/ci.yml     # NEW — 4 jobs (build-test matrix, lint, vulncheck, coverage).
.golangci.yml                # NEW — conservative deterministic linter set.
# ALL other files UNCHANGED. No Makefile edit, no go.mod edit, no release.yml, no goreleaser.
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (internal/config IS BELOW THE 85% GATE — measured 83.3%, 205/246 stmts). A coverage gate
# implemented verbatim to PRD §20.3 FAILS on day one. The implementer MUST resolve this before shipping
# (see "Coverage-gap decision tree" below). git=94.1%, provider=93.6%, generate=87.1% all pass;
# generate is close to the line (one regression drops it under). Re-measure at implementation time:
#   go test -coverprofile=coverage.out ./...
# then run the awk gate from §"Implementation Patterns" and read the 4 numbers.

# CRITICAL (THE ARCH MATRIX CANNOT BE 6 NATIVE CELLS). GitHub-hosted runners give native test
# execution on only 4 of the 6 PRD OS×arch combos:
#   ubuntu-latest  = linux/amd64
#   macos-13       = macos/amd64   (Intel)
#   macos-latest   = macos/arm64   (Apple Silicon; macos-14)
#   windows-latest = windows/amd64
# linux/arm64 + windows/arm64 runners are NOT reliably available (preview/restricted). Do NOT fake them
# with a label that 404s. Cover those 2 combos with a COMPILE-ONLY cross-build (GOARCH=arm64) instead.
# macos-latest already gives REAL arm64 TEST coverage, so the PRD's "test on arm64" intent is genuinely
# satisfied for macos. Full QEMU test emulation for linux/arm64 = optional, out of v1 scope (slow/flaky).

# CRITICAL (SCOPE: NO goreleaser / NO release-on-tag). §20.4's "Release on tag via goreleaser" is
# S2 (P1.M5.T3.S2). S1's ci.yml triggers ONLY on push(main)+PR. Do NOT add `on: push: tags:`, do NOT
# create release.yml, do NOT add a goreleaser job. Crossing this boundary collides with S2's work.

# CRITICAL (SCOPE: NO Makefile change). The `make coverage-gate` TARGET is S3 (P1.M5.T3.S3). S1's
# coverage gate is INLINE awk in ci.yml (self-contained, no third-party action, no Make dependency).
# Do NOT add a Makefile target; do NOT have ci.yml shell out to a coverage Make target. After S3 ships,
# a future refactor may let ci.yml delegate to `make coverage-gate` — that is OUT of S1's scope.

# GOTCHA (COVERAGE GATE MUST BE STATEMENT-WEIGHTED, NOT A MEAN OF FUNCTION %s). `go tool cover -func`
# prints per-FUNCTION percentages; averaging those is mathematically WRONG (a 100-line uncovered func
# weighted the same as a 1-line covered one). Parse the cover PROFILE (coverage.out) directly: each
# line is `pkg/file.go:start.col,end.col numStatements hitCount`. Sum numStatements → total; sum where
# hitCount>0 → covered; coverage% = covered/total*100. The awk in §"Implementation Patterns" does this.

# GOTCHA (PER-PACKAGE OWN-TESTS COVERAGE, NOT -coverpkg). `go test -coverprofile=coverage.out ./...`
# measures each package's coverage by ITS OWN tests (the standard, honest interpretation of §20.3).
# Do NOT add `-coverpkg=./internal/...` — that would inflate numbers via cross-package tests and is not
# what §20.3 means. The numbers in the baseline table above were measured this way.

# GOTCHA (LINT GATE IS UNVERIFIED LOCALLY). golangci-lint is NOT installed on the authoring machine.
# golangci-lint-action installs its own binary in CI, so the gate WILL run — but the codebase's current
# lint state is UNKNOWN. If existing code triggers findings, the first CI run is RED. MITIGATION:
# ship a conservative .golangci.yml (disable-all + a short enable list) AND the implementer MUST run
# `make lint` locally (install golangci-lint if needed: `go install github.com/golangci/golangci-lint/
# cmd/golangci-lint@v1.61.0`) and fix findings or relax .golangci.yml until green BEFORE relying on it.

# GOTCHA (GOVULNCHECK ALSO UNVERIFIED LOCALLY, but low risk). Deps are tiny + current; very likely
# green. If a vuln surfaces, update the offending dep (go get + go mod tidy) — that IS in scope (the
# gate must be green), and it's the gate working as intended.

# GOTCHA (golangci-lint-action DOES NOT NEED setup-go's cache to be disabled, but the lint job should
# be its OWN job, not a matrix cell). Running golangci-lint on windows/macos across 8 cells is slow and
# Windows-lint is historically flaky. Keep lint as a single ubuntu job. Same for vulncheck + coverage.

# GOTCHA (concurrency: cancel-in-progress only for PRs). Set `cancel-in-progress:
# ${{ github.event_name == 'pull_request' }}` so a force-push to main isn't cancelled mid-run.

# GOTCHA (permissions: least privilege). Add `permissions: contents: read` at workflow top. CI that
# only reads code + runs tests needs no write scope.

# GOTCHA (P1.M5.T2.S1 runs in parallel and adds providers/*.toml + internal/provider/referencefiles_
# test.go). No file conflict with S1. S1's `go test ./...` will include that new test; it is green by
# T2.S1's own contract, so S1's gates stay green. No coordination beyond "don't edit the same files".
```

#### Coverage-gap decision tree (resolves the config=83.3 % blocker)

```
MEASURE FIRST:  go test -coverprofile=coverage.out ./...   (then run the gate awk, read the 4 numbers)

IF all 4 packages (git, provider, generate, config) are >= 85 %:
    -> ship the gate with threshold = 85. DONE.

IF internal/config (or any) is < 85 % (CONFIG WAS 83.3% = 205/246 stmts; needs >=210 = +5 statements):
    OPTION A (PREFERRED — keeps the 85% gate honest):
        Add tests in internal/config covering the ~5 uncovered statements, until coverage >= 85 %.
        Find the gaps:  go tool cover -func=coverage.out | grep internal/config | grep -v 100.0%
        (or: GOFLAGS= go tool cover -html=coverage.out  in a browser).
        This is a small, quality-positive change. Re-run the gate; ship at 85 %.
    OPTION B (SANCTIONED FALLBACK — if the uncovered statements are genuinely hard/error-path code):
        Ratchet ONLY that package's floor to its current coverage in the gate awk
        (give config a separate `floor=83` with an inline `# TODO(§20.3): raise internal/config 83→85`
        comment). Gate is GREEN now, prevents regression, flags the gap. Ship.
    OPTION C (FORBIDDEN): ship a gate that fails CI on day one. Do NOT do this.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY the baseline + measure coverage (READ + RUN, no edit)
  - RUN: `go test -race ./...` -> expect all `ok` (currently green; confirms build-test gate is safe).
  - RUN: `go test -coverprofile=coverage.out ./...` then the gate awk in §"Implementation Patterns".
    Read the 4 numbers. EXPECT (measured): git 94.1%, provider 93.6%, generate 87.1%, config 83.3%.
  - RUN (find config gaps, only if Option A): `go tool cover -func=coverage.out | grep internal/config
    | grep -v 100.0%`.
  - RUN (lint baseline, if golangci-lint available): `make lint` -> note findings (the lint gate is
    UNVERIFIED locally; see gotcha). Install if needed: `go install github.com/golangci/golangci-lint/
    cmd/golangci-lint@v1.61.0`.
  - CONFIRM: `.github/` does not exist (`ls .github` -> no such dir). Created by writing ci.yml.

Task 1: RESOLVE the coverage gap (DECISION — do this before/while writing the gate)
  - Follow the "Coverage-gap decision tree" above.
  - PREFERRED: Option A — add the minimal tests to internal/config to reach >= 85% (small, safe).
    (Only if you judge Option B necessary: ratchet config's floor in the gate awk with a TODO.)
  - RECORD: the final measured 4 numbers in the PRP's validation notes.

Task 2: CREATE .golangci.yml (deterministic lint config)
  - CONTENT: copy §".golangci.yml" below verbatim. `disable-all: true` + enable [errcheck, gosimple,
    govet, ineffassign, staticcheck, unused]; `run.timeout: 5m`; `run.go: '1.22'`.
  - VERIFY locally: `golangci-lint run` (install if needed) -> green. If findings exist, EITHER fix
    them OR relax .golangci.yml (e.g. add an `issues.exclude-rules` entry) until green. Do NOT ship a
    lint gate you have not seen pass locally.

Task 3: CREATE .github/workflows/ci.yml (the pipeline)
  - CONTENT: copy §"ci.yml" below verbatim (then adjust the coverage threshold per Task 1's decision).
  - TRIGGERS: `on: push: branches: [main]` + `pull_request:`. (NO tags, NO release — S2 owns those.)
  - JOBS (4): build-test (matrix), lint, vulncheck, coverage — all gate the PR.
  - MATRIX: os [ubuntu-latest, macos-latest, macos-13, windows-latest] x go ['1.22','1.23'];
    fail-fast: false.
  - ACTIONS PINNED: checkout@v4, setup-go@v5 (cache: true), golangci-lint-action@v6 (version: v1.61),
    govulncheck-action@v1.
  - PERMISSIONS: `contents: read`. CONCURRENCY: cancel superseded PR runs.

Task 4: THE COVERAGE GATE (already inside ci.yml's `coverage` job — verify it behaves)
  - Confirm the awk is statement-weighted (parses coverage.out profile lines, NOT `cover -func` means).
  - Confirm it targets exactly the 4 module paths: github.com/dustin/stagecoach/internal/{git,provider,
    generate,config}.
  - DEMONSTRATE the gate bites: temporarily set `threshold=99`, run the awk locally on coverage.out,
    confirm it prints `::error::… < 99%` and exits non-zero; then REVERT to 85.

Task 5: VALIDATE (without burning GitHub minutes)
  - STATIC (best): `go install github.com/rhysd/actionlint/cmd/actionlint@latest && actionlint
    .github/workflows/ci.yml` -> exit 0. (Falls back to YAML parse: `python -c "import
    yaml;yaml.safe_load(open('.github/workflows/ci.yml'))"`.)
  - LOCAL RUN (if Docker available): install `act`, then `act push -j coverage` and `act push -j lint`
    (one ubuntu cell). Expect green.
  - OR (definitive): push ci.yml + .golangci.yml to a throwaway branch, open a PR, watch the Actions
    tab: all 4 jobs green.
  - FINAL: `git status --short` -> ONLY .github/workflows/ci.yml + .golangci.yml (+ any Task 1 test
    files). Makefile/go.mod/PRD.md untouched.
```

### Implementation Patterns & Key Details

#### `ci.yml` (copy-pasteable; adjust `threshold` per Task 1)

```yaml
# .github/workflows/ci.yml — Stagecoach CI (PRD §20.4 build/test matrix + §20.3 coverage gate).
# Scope NOTE: release-on-tag via goreleaser lives in a SEPARATE workflow owned by P1.M5.T3.S2.
# This file triggers ONLY on push(main) + pull_request.
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

# Cancel superseded runs on the same ref (only auto-cancel for PRs; let main runs finish).
concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}

jobs:
  # --- (1) Build + race-enabled tests across the os x Go matrix -----------------
  build-test:
    name: test (${{ matrix.os }} / go${{ matrix.go }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        # Maps PRD §20.4's {linux,macos,windows} x {amd64,arm64} onto native GitHub runners:
        #   ubuntu-latest  = linux/amd64
        #   macos-13       = macos/amd64  (Intel)
        #   macos-latest   = macos/arm64  (Apple Silicon)  <-- real arm64 TEST coverage
        #   windows-latest = windows/amd64
        # linux/arm64 + windows/arm64 are compile-checked in the `cross-build` job below.
        os: [ubuntu-latest, macos-latest, macos-13, windows-latest]
        go: ['1.22', '1.23']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
          cache: true            # auto-keyed on go.sum
      - name: Build
        run: go build ./...
      - name: Cross-build arm64 (covers linux/arm64 + windows/arm64 compile)
        # Proves the arm64 targets COMPILE without paying for/suffering QEMU test emulation.
        run: |
          go env -w GOARCH=arm64 GOOS=linux   && go build ./...
          go env -w GOARCH=arm64 GOOS=windows && go build ./...
          go env -u GOARCH GOOS
      - name: Test (race)
        run: go test -race ./...

  # --- (2) golangci-lint (single ubuntu job — faster, no windows-lint flake) ----
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.61
          args: --timeout=5m

  # --- (3) govulncheck (single ubuntu job) -------------------------------------
  vulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: golang/govulncheck-action@v1
        with:
          go-version: '1.22'
          go-package: ./...

  # --- (4) Coverage gate (PRD §20.3: >=85% on the 4 core packages) -------------
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true
      - name: Generate coverage
        # Per-package OWN-TESTS coverage (the standard interpretation of §20.3). NOT -coverpkg.
        run: go test -coverprofile=coverage.out ./...
      - name: Enforce >=85% on internal/{git,provider,generate,config}
        # Self-contained, statement-weighted gate. No third-party action, no Make dependency.
        # threshold (PRD §20.3 = 85). If you ratchet a package (see "Coverage-gap decision tree"),
        # give it a per-package floor below instead of lowering this global threshold.
        run: |
          awk '
            /^mode:/ {next}
            {
              f=$1; sub(/:[0-9]+.*$/,"",f);            # f = pkgpath/file.go
              n=split(f,p,"/"); pkg=""; for(i=1;i<n;i++){pkg=pkg (i>1?"/":"") p[i]}
              tot[pkg]+=$2; if($3+0>0) cov[pkg]+=$2    # $2=numStatements, $3=hitCount
            }
            END{
              threshold=85
              t[1]="github.com/dustin/stagecoach/internal/git"
              t[2]="github.com/dustin/stagecoach/internal/provider"
              t[3]="github.com/dustin/stagecoach/internal/generate"
              t[4]="github.com/dustin/stagecoach/internal/config"
              fail=0
              for(i=1;i<=4;i++){
                if(!(t[i] in tot)){ printf("::error::%s — no coverage data\n",t[i]); fail=1; continue }
                pct=(tot[t[i]]>0)? cov[t[i]]/tot[t[i]]*100 : 0
                printf("%-55s %5.1f%%\n", t[i], pct)
                if(pct < threshold){ printf("::error::%s coverage %.1f%% < %d%%\n", t[i], pct, threshold); fail=1 }
              }
              exit fail
            }
          ' coverage.out
```

#### `.golangci.yml` (copy-pasteable; relax only if a finding forces it)

```yaml
# .golangci.yml — deterministic lint config consumed by ci.yml's `lint` job.
# Conservative: disable-all + a short, well-understood enable list so CI lint is stable and the
# codebase is very likely already green. Add linters deliberately; never enable-all.
run:
  timeout: 5m
  go: '1.22'

linters:
  disable-all: true
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused

linters-settings:
  errcheck:
    # Don't flag unchecked Close() on io.Closer (common, noisy, low-value).
    exclude-functions:
      - (io.Closer).Close

issues:
  # Don't auto-disable; surface real findings. Show up to 50 per file.
  max-issues-per-linter: 0
  max-same-issues: 0
```

### Integration Points

```yaml
GITHUB ACTIONS (new):
  - the pipeline lives at .github/workflows/ci.yml (GitHub auto-discovers *.yml in .github/workflows/).
    No registration step. The first push/PR after the file lands triggers it.
  - .golangci.yml at repo ROOT is auto-discovered by golangci-lint (and by golangci-lint-action).
    No path config needed.

MAKEFILE (UNCHANGED — CI mirrors, does not edit):
  - ci.yml's build/test/coverage invocations intentionally MATCH the Makefile targets (`go build ./...`,
    `go test -race ./...`, `go test -coverprofile=coverage.out ./...`) so `make test`/`make lint` and
    CI agree. The only ADD is the coverage GATE, which the Makefile lacks (S3 will add the Makefile
    target separately).

SCOPE HANDOFFS (do NOT create these — owned elsewhere):
  - S2 (P1.M5.T3.S2): .goreleaser.yaml + release-on-tag workflow (e.g. release.yml). ci.yml has NO
    tags trigger and NO goreleaser step, leaving S2 a clean field.
  - S3 (P1.M5.T3.S3): a `make coverage-gate` Makefile target. ci.yml's gate is inline awk; after S3
    lands, ci.yml MAY delegate to `make coverage-gate` (single source of truth) — but that refactor is
    out of S1's scope (would couple to a not-yet-existing target / duplicate S3's deliverable).

PARALLEL (P1.M5.T2.S1, in-flight):
  - adds providers/*.toml + internal/provider/referencefiles_test.go. No file overlap with S1. S1's
    `go test ./...` includes that new test (green by T2.S1's contract) — S1's gates stay green.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Static CI validation (best offline check — catches matrix/expr/action typos before pushing):
go install github.com/rhysd/actionlint/cmd/actionlint@latest
actionlint .github/workflows/ci.yml            # expect: exit 0, no output

# YAML parse fallback (if actionlint unavailable):
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"
python3 -c "import yaml; yaml.safe_load(open('.golangci.yml')); print('ok')"

# Go-side: no Go files are added by this task (except any Task-1 config tests, which use normal gates):
gofmt -l .                                     # expect empty for any files you touched
go vet ./...                                    # expect clean
# Expected: zero errors. actionlint clean = the YAML, matrix, and ${{ }} expressions are well-formed.
```

### Level 2: Local job execution (Component Validation)

```bash
# Reproduce each CI job locally so you KNOW it is green before pushing.

# build-test matrix cell (pick go 1.22, linux/amd64 — what you already have):
go build ./...
GOOS=linux   GOARCH=arm64 go build ./...        # the cross-build step
GOOS=windows GOARCH=arm64 go build ./...
go test -race ./...                             # expect: all ok

# lint (install if needed):  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
make lint   # OR:  golangci-lint run --timeout=5m      # expect: clean

# vulncheck (install if needed):  go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...                               # expect: no vulnerabilities

# coverage gate (run the EXACT awk from ci.yml against a fresh profile):
go test -coverprofile=coverage.out ./...
awk '...<paste the gate awk from ci.yml>...' coverage.out    # expect: 4 packages all >=85%, exit 0

# Prove the gate bites: temporarily set threshold=99 in the awk, re-run -> expect exit 1 +
# "::error::… < 99%". REVERT to 85 before committing.
```

### Level 3: Integration (System Validation — the contract's "act or push to a branch")

```bash
# Option A — local CI runner (needs Docker):
#   install act:  curl -s https://webi.sh/act | sh   (or: brew install act)
act push -j coverage                           # runs the coverage job in Docker
act push -j lint                               # runs the lint job
# (use `--matrix os:ubuntu-latest` to run one matrix cell of build-test)

# Option B (definitive) — push to a throwaway branch and watch the Actions tab:
git checkout -b ci/dry-run
git add .github/workflows/ci.yml .golangci.yml
git commit -m "ci: add build/test matrix, golangci-lint, govulncheck, >=85% coverage gate"
git push -u origin ci/dry-run
# Open the PR. All four jobs (build-test x8, lint, vulncheck, coverage) should be green.

# Expected: all jobs green on a tree where `go test -race ./...` is green and the 4 core packages
# are >=85%. The coverage job's log prints the 4 per-package percentages.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Demonstrate the coverage gate enforces the floor (negative test):
#   temporarily lower one package's effective coverage by setting threshold=99 in the awk,
#   run it on coverage.out, confirm exit 1 + "::error::" annotation naming the package. REVERT.

# Confirm scope discipline (no boundary violations):
git diff --name-only $(git merge-base HEAD main)   # expect ONLY: .github/workflows/ci.yml, .golangci.yml
                                                      # (+ any Task-1 config test files)
grep -nE 'goreleaser|tags:|release' .github/workflows/ci.yml   # expect: no matches (S2 owns release)
grep -nE 'coverage-gate|make coverage' .github/workflows/ci.yml # expect: no matches (S3 owns Make target)
# Expected: the file contains exactly the 4 CI jobs; no release/Makefile coupling.
```

## Final Validation Checklist

### Technical Validation

- [ ] `actionlint .github/workflows/ci.yml` exits 0 (or YAML parses).
- [ ] `.golangci.yml` parses; `golangci-lint run` is green locally.
- [ ] `go build ./...` + `GOARCH=arm64 go build ./...` (linux+windows) succeed.
- [ ] `go test -race ./...` green.
- [ ] `govulncheck ./...` reports no vulnerabilities.
- [ ] Coverage gate: all 4 core packages ≥85 % (config topped up via Option A, or ratcheted via B).

### Feature Validation

- [ ] All 4 success-criteria bullets under "What" met.
- [ ] Negative test: lowering the gate threshold to 99 % makes the `coverage` job fail with an
      `::error::` annotation naming the offending package (then reverted).
- [ ] Pushed to a branch, the Actions tab runs build-test(×8) + lint + vulncheck + coverage, all green
      (or `act` dry-run of `lint` + `coverage` succeeds).
- [ ] Matrix enumerates exactly ubuntu-latest, macos-latest, macos-13, windows-latest × 1.22, 1.23.

### Code Quality & Scope Validation

- [ ] `git status --short` shows ONLY `.github/workflows/ci.yml` + `.golangci.yml` (+ any Task-1
      config tests). Makefile, go.mod, PRD.md, .gitignore UNCHANGED.
- [ ] NO goreleaser / release / `tags:` trigger (S2's scope).
- [ ] ci.yml does NOT call any Makefile coverage target (S3's scope); the gate is self-contained awk.
- [ ] `permissions: contents: read` (least privilege); concurrency cancels superseded PR runs.
- [ ] Action versions pinned (checkout@v4, setup-go@v5, golangci-lint-action@v6, govulncheck-action@v1).

### Documentation & Deployment

- [ ] ci.yml's header comment states its scope (push(main)+PR) and that release-on-tag is S2.
- [ ] The runner→arch mapping is documented inline in the matrix comment block.
- [ ] The coverage-gate awk is commented (statement-weighted; why not -coverpkg; the 4 target packages).

---

## Anti-Patterns to Avoid

- ❌ Don't add a `tags:` trigger, a `release.yml`, or any goreleaser step — release-on-tag is S2.
- ❌ Don't edit the Makefile or add a `coverage-gate` Make target — that's S3. Keep the gate inline.
- ❌ Don't average `go tool cover -func` per-function percentages — that's mathematically wrong
      coverage. Parse the cover PROFILE (statement-weighted) — the awk does this.
- ❌ Don't use `-coverpkg=./internal/...` to inflate the numbers — §20.3 means each package's
      own-tests coverage. Use plain `go test -coverprofile=coverage.out ./...`.
- ❌ Don't fake the arch matrix with a label that 404s (linux/arm64, windows/arm64 aren't reliably
      available as GitHub-hosted runners). Use the 4 native runners + a cross-compile check.
- ❌ Don't run lint/vulncheck/coverage inside the 8-cell build-test matrix (slow, Windows-lint flake,
      redundant). Split into single-runner jobs.
- ❌ Don't ship a coverage gate that fails on day one (config was 83.3 %). Resolve via Option A/B first.
- ❌ Don't ship a lint gate you haven't seen pass locally (golangci-lint isn't installed by default;
      install it, run `make lint`, fix/relax until green).
- ❌ Don't use `enable-all` in .golangci.yml — it's noisy and brittle. Start conservative, add deliberately.
- ❌ Don't set `cancel-in-progress: true` unconditionally — that cancels mid-run main pushes.
