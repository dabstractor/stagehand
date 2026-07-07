---
name: "P1.M5.T2.S2 — Full build + test suite passes with stagecoach identity (rename verified at the build/test/runtime layer)"
description: |

  THIS IS A VERIFICATION / CERTIFICATION TASK. It proves the stagehand→stagecoach rename (PRD h2.30) leaves
  the project in a state where (a) `make build` produces `./bin/stagecoach`, (b) the FULL test suite passes —
  unit + stub-agent integration + the build-tag-gated e2e harness, (c) `go vet ./...` is clean, and (d) the
  compiled binary presents a fully-stagecoach runtime identity (`--version` prefix, `--help` command name,
  `STAGECOACH_*` env vars, `stagecoach.*` git config keys) with ZERO stagehand leakage in any compiled or
  test surface. It is the build/test counterpart to the sibling grep-audit task P1.M5.T2.S1.

  ⚠️ HEADLINE RESEARCH FINDING (executed in the live repo 2026-07-07): EVERYTHING IS ALREADY GREEN.
  The rename's M1–M4 passes left the build/test/identity surface fully stagecoach. Concretely:
    - `make build`                     → EXIT 0, produces `bin/stagecoach` (9.1 MB).
    - `go test ./... -count=1`         → 17 ok / 0 FAIL / 2 no-test. exit 0.
    - `go test -tags e2e ./internal/e2e/... -count=1`  → ok 22.4s, exit 0.  ← NOT in default `go test ./...`
    - `go vet ./...`                   → EXIT 0 (clean).
    - `go vet -tags integration_real ./internal/generate/...` → EXIT 0 (real-agent suite compiles; not run in CI).
    - `./bin/stagecoach --version`     → `stagecoach version dev (44c299b-dirty)` exit 0.
    - `./bin/stagecoach --help`        → `Usage: stagecoach [flags]`; STAGECOACH_* env; stagecoach.* git keys; exit 0.
    - `git grep -il 'stagehand' -- '*.go'` → 0 files (entire repo). testdata/golden → 0 files.
    - Bonus project gates: `make test` (race) → 17 ok / 0 FAIL; `make coverage-gate` → PASS (4 core pkgs ≥85%).

  ⚠️ CRITICAL GOTCHA (the one real subtlety): the contract's step (b) — `go test ./... -count=1` — does NOT
  include the e2e harness or the real-agent suite, because they are `//go:build`-gated. The item description
  explicitly says the suite "includes ... the e2e harness", so a correct certification runs BOTH the default
  suite AND `go test -tags e2e ./internal/e2e/...`, and compiles (not runs) the `-tags integration_real`
  suite. Running only step (b) satisfies the literal command but silently skips the e2e harness.

  CONTRACT (item_description §1–§5; PRD h2.30; PRD §20.1–20.5 testing strategy):
    1. RESEARCH NOTE: "comprehensive tests (internal/e2e/, internal/stubtest/). All must pass with the new
       identity. Build must produce ./bin/stagecoach."
    2. INPUT: "fully renamed Go source (M1–M2 complete, plus M3 for Makefile)."
    3. LOGIC: (a) make build → ./bin/stagecoach no errors; (b) go test ./... -count=1 passes (unit + stub +
       e2e); (c) go vet ./... clean; (d) ./bin/stagecoach --version prints version (or dev); (e)
       ./bin/stagecoach --help shows 'stagecoach', STAGECOACH_ env vars, stagecoach.* git config keys.
    4. OUTPUT: make build succeeds; go test passes; go vet passes; --help shows correct identity.
    5. DOCS: none — verification step.

  SCOPE BOUNDARY (do NOT touch unless a stagehand straggler BREAKS build/test/identity — currently none):
    - The whole-repo `git grep -i stagehand` audit → P1.M5.T2.S1 (the sibling grep-audit task). S1 owns its
      3 fixes (git.go comment, .goreleaser.yaml comment, .golangci.yml path); none affect build/test.
    - plan/012_963e3918ec08/, **/tasks.json, PRD.md → S1's documented exceptions; not compiled, not in any
      test path; NEVER touch to "fix" a stagehand count.
    - Production behavior, new features, new tests → none. Verification only.
    - Real-agent EXECUTION (integration_real RUN) → manual pre-release (PRD §20.1 layer-4, opt-in, not CI).

  DELIVERABLE: a verification report (recorded in the implementation summary — DOCS: none, no new file)
  demonstrating the 5 contract steps + the e2e harness + the integration_real compile, each with the precise
  expected output/identity asserted. PLUS a scoped locate-and-fix IF (and only if) a stagehand straggler
  breaks build/test/identity (currently: zero fixes needed). NO new files.

  SUCCESS: all 5 contract steps pass with the identity assertions met; the e2e harness passes under
  `-tags e2e`; the integration_real suite compiles under `-tags integration_real`; `--help`/`--version`
  contain zero "stagehand"; the Go surface (all .go + all build tags + testdata) has zero stagehand refs.

---

## Goal

**Feature Goal**: Certify — by executing, not by asserting — that the renamed `stagecoach` project builds
cleanly (`make build` → `./bin/stagecoach`), its complete test suite passes (unit + stub-agent integration +
the e2e harness), `go vet ./...` is clean, and the compiled binary presents a fully-`stagecoach` runtime
identity (`--version` prefixes with `stagecoach version`, `--help` uses `stagecoach` as the command name and
surfaces `STAGECOACH_*` env vars and `stagecoach.*` git config keys) with zero `stagehand` leakage anywhere
in the compiled or test surface.

**Deliverable**: A verification record (implementation summary — DOCS: none, no new file) showing each of the
5 contract steps (a–e) executed with its expected output, the build-tag-gated e2e harness run explicitly
(`go test -tags e2e ./internal/e2e/...`), and the `integration_real` suite compiled (`go vet -tags
integration_real ./internal/generate/...`). A scoped locate-and-fix is applied ONLY if a stagehand straggler
breaks build/test/identity (research found: none needed today).

**Success Definition**:
- `make build` exits 0 and `bin/stagecoach` exists and is executable.
- `go test ./... -count=1` → 0 FAIL (17 ok, 2 no-test).
- `go test -tags e2e ./internal/e2e/... -count=1` → 0 FAIL (the e2e harness the item names; NOT in default suite).
- `go vet ./...` → clean (exit 0); `go vet -tags integration_real ./internal/generate/...` → clean.
- `./bin/stagecoach --version` output starts with `stagecoach version ` and contains `dev` (or `dev (<sha>…)`
  or a real `vX.Y.Z`); contains NO `stagehand`.
- `./bin/stagecoach --help` shows `Usage:\n  stagecoach [flags]`, lists subcommands, and its flag block shows
  `STAGECOACH_*` env vars and `stagecoach.*` git config keys; `grep -i stagehand` over its output → nothing.
- `git grep -il 'stagehand' -- '*.go'` → 0 files; testdata → 0 files (no rename residue in any compiled/test path).

## User Persona

**Target User**: the maintainer certifying the rename before the first `stagecoach` tag, and the release
engineer who needs `make build` + `make test` + `make coverage-gate` green. Secondary: any contributor
running `go test ./...` whose local run must be green post-rename.

**Use Case**: "Does the renamed project actually build and pass its whole test suite, and does the binary
call itself stagecoach at runtime?" The maintainer runs the 5 steps + the e2e tag, eyeballs the `--version`
and `--help` output, and ships. A test that asserts on a stale "stagehand" string, or a `bin/stagehand` path,
would silently break; this task catches that.

**Pain Points Addressed**: a rename that compiles but whose tests still assert on the old name (or whose
binary still prints the old name in `--help`/`--version`) is a half-finished rename. This task is the proof
that the rename reached the build/test/runtime layer, not just the source text.

## Why

- **It is the rename's build/test close-out gate (PRD h2.30), complementary to S1's text gate.** S1 certifies
  the repo's TEXT surface has no stagehand; S2 certifies the project BUILDS, its TESTS pass, and the BINARY
  presents the right identity. Both are needed; neither subsumes the other.
- **Catches the build-tag blind spot the literal command (b) has.** `go test ./...` excludes the `//go:build
  e2e` harness and the `//go:build integration_real` suite. The item explicitly wants the e2e harness in
  scope; running only step (b) would silently skip it. This task names the extra command and runs it.
- **Catches identity leaks that a green build cannot.** `go build` succeeding says nothing about whether
  `--help` still prints "stagehand" somewhere in a flag usage string or subcommand `Short`. The identity
  assertions (`--version` prefix, `--help` grep) are the net that catches those.
- **Establishes the rename's regression baseline.** The verification commands + expected outputs become the
  re-runnable check the next maintainer uses after any future change to confirm the identity held.

## What

Run, observe, and assert on the 5 contract steps plus the two build-tag-gated suites, recording expected
identity output. The complete certification sequence (each verified green in research):

1. **Build**: `make build` → `./bin/stagecoach` (exit 0; Makefile target `go build -ldflags "-X
   main.version=dev" -o bin/stagecoach ./cmd/stagecoach`).
2. **Default test suite**: `go test ./... -count=1` → 0 FAIL (unit + stub-agent integration in
   `internal/stubtest`, which has no build tag).
3. **e2e harness** (build-tag-gated — MUST be run explicitly): `go test -tags e2e ./internal/e2e/... -count=1`
   → 0 FAIL.
4. **Vet**: `go vet ./...` → clean; `go vet -tags integration_real ./internal/generate/...` → clean
   (compile-check the real-agent suite; do not run it — it needs real CLIs + `STAGECOACH_RUN_REAL=1`).
5. **Version**: `./bin/stagecoach --version` → starts `stagecoach version `, then `dev` or `dev (<sha>…)`.
6. **Help identity**: `./bin/stagecoach --help` → `Usage: stagecoach [flags]`; flag block shows `STAGECOACH_*`
   env + `stagecoach.*` git keys; `grep -i stagehand` over output → nothing.
7. **Residue check (compiled surface only)**: `git grep -il 'stagehand' -- '*.go'` → 0 files; testdata → 0.

If any step reveals a stagehand straggler that BREAKS build/test/identity, apply the scoped locate-and-fix
(§"Implementation Tasks" Task 6; currently zero fixes are needed).

### Success Criteria

- [ ] `make build` exits 0 and `bin/stagecoach` is executable.
- [ ] `go test ./... -count=1` → 0 FAIL.
- [ ] `go test -tags e2e ./internal/e2e/... -count=1` → 0 FAIL.
- [ ] `go vet ./...` clean; `go vet -tags integration_real ./internal/generate/...` clean.
- [ ] `./bin/stagecoach --version` output starts with `stagecoach version ` and contains no `stagehand`.
- [ ] `./bin/stagecoach --help` shows `stagecoach` as command name + `STAGECOACH_*` env + `stagecoach.*` git
      keys; `./bin/stagecoach --help 2>&1 | grep -i stagehand` → no output.
- [ ] `git grep -il 'stagehand' -- '*.go'` → 0 files (compiled/test surface clean).
- [ ] (Bonus, project QA) `make test` (race) → 0 FAIL; `make coverage-gate` → PASS (4 core pkgs ≥85%).
- [ ] If any fix was needed: it is a test/build-surface change only (no plan/, PRD.md, tasks.json, production
      behavior); the re-run of the affected step is green; the fix is recorded in the summary.

## All Needed Context

### Context Completeness Check

_Pass._ An implementer with no prior repo knowledge can certify this from: the exact commands (verbatim), the
expected output for each (identity assertions spelled out), the build-tag map (which suites the default
`go test ./...` hides), the identity-derivation facts (where "stagecoach" comes from in `--version`/`--help`),
and a scoped remediation playbook for the case a straggler breaks something. No deep Go/cobra/goreleaser
internals are required to run the certification; the playbook covers the "if a test fails" path.

### Documentation & References

```yaml
# MUST READ — THE verified state + the build-tag nuance + the remediation playbook (this task's own research)
- docfile: plan/012_963e3918ec08/P1M5T2S2/research/findings.md
  why: §1 the 5 contract steps VERIFIED green (table with exact outputs); §2 the build-tag map (e2e +
       integration_real are NOT in default `go test ./...` — the one real subtlety); §3 why integration_real
       is compile-only; §4 the identity surface (where "stagecoach" is derived: rootCmd.Use, cobra default
       version template, resolveVersion, flag usage strings); §5 zero residue in .go/testdata; §6 the
       non-overlap with sibling S1; §7 the remediation playbook; §8 out-of-scope.
  critical: §2 (run `-tags e2e` explicitly), §4 (the identity assertions), §6 (do NOT duplicate S1's audit).

# MUST READ — the sibling grep-audit task (the contract for the rename's TEXT surface; NON-OVERLAPPING)
- docfile: plan/012_963e3918ec08/P1M5T2S1/PRP.md
  why: S1 (parallel, treated as a CONTRACT) owns the whole-repo `git grep -i stagehand` audit + its 3 fixes
       (internal/git/git.go:390 comment, .goreleaser.yaml:1 comment, .golangci.yml:40 path). Those are
       comment/path-only and do NOT affect build/test (a comment is not compiled; the .golangci path only
       matters if S2 runs `make lint`, which is NOT among the contract's 5 steps). S2 consumes S1's
       documented exception categories (plan/012, tasks.json, PRD directive) as "not compiled, not in any
       test path — never touch to fix a stagehand count."
  critical: do NOT re-run S1's grep audit as this task's deliverable; do NOT touch S1's 3 files, plan/,
            PRD.md, or tasks.json. S2 is build + test + runtime-identity only.

# MUST READ — the build system (the `make build` / `make test` / `make coverage-gate` definitions)
- file: Makefile
  section: `build` target (lines ~36-37): `go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/stagecoach` where
           `BIN := bin/stagecoach`, `LDFLAGS := -X main.version=$(VERSION)`, `VERSION ?= dev`. `test` target:
           `go test -race ./...`. `coverage-gate`: ≥85% on internal/{git,provider,generate,config} (PRD §20.3).
  why: confirms step (a)'s exact invocation and that the binary is named `stagecoach`; the `VERSION=dev`
       default is why `--version` shows `dev` (enriched by resolveVersion).
  gotcha: `make test` adds `-race`; the contract's step (b) is the bare `go test ./... -count=1` (no race).
          Both are run (race is a bonus project gate). `make lint` (golangci-lint) is NOT a contract step.

# MUST READ — the binary's identity source (Use field + version resolution + error prefix)
- file: internal/cmd/root.go
  section: line 120-126 `rootCmd = &cobra.Command{ Use: "stagecoach", Short: "AI-assisted commit message
           generator", Version: Version, ... }`; lines 153-234 the persistent flags whose usage strings name
           every `STAGECOACH_*` env var and `stagecoach.*` git config key; line 246 the `stagecoachFlagUsages`
           template func (cobra help wrapping). There is NO `SetVersionTemplate`/`SetHelpTemplate` anywhere.
  why: this is WHY `--help` says `stagecoach` and WHY `--version` prefixes `stagecoach version ` — cobra
       derives both from `rootCmd.Use`. Asserting on these outputs validates the rename reached the CLI layer.
  gotcha: do NOT edit root.go in this task (it is already correct). It is referenced to explain the expected
          `--help`/`--version` output, not to be changed.

# MUST READ — the version value (`--version` prints `dev` or `dev (<sha>…)`)
- file: cmd/stagecoach/main.go
  section: `var version = "dev"`; `resolveVersion()` enriches bare `dev` via `debug.ReadBuildInfo`
           (vcs.revision + vcs.modified) → `dev (<short-sha>-dirty)` for a dirty tree; a real `vX.Y.Z` is
           returned verbatim. `main()` sets `cmd.Version = resolveVersion(version)` before `cmd.Execute`.
  why: explains the EXACT `--version` output to assert: `stagecoach version dev (<7-hex>-dirty)` on a dirty
       working tree (the rename changeset makes the tree dirty), `stagecoach version dev (<7-hex>)` clean,
       `stagecoach version vX.Y.Z` for a tagged release.
  gotcha: the `-dirty` suffix is EXPECTED while the rename changeset is uncommitted — it is not a failure.

# READ — the e2e harness (self-builds cmd/stagecoach; the suite the item names but default go test hides)
- file: internal/e2e/harness_test.go   (//go:build e2e)
  section: `buildStagecoach` (lines ~44-70) compiles `github.com/dustin/stagecoach/cmd/stagecoach` ONCE per
           test process into a temp dir named `stagecoach`; `runStagecoach` drives it as a subprocess.
  why: confirms the e2e suite uses the renamed binary/path/config (`stagecoach.toml`) and that a green e2e
       run is meaningful identity proof (the harness exercises the real CLI end-to-end against temp repos).
  gotcha: the suite is `//go:build e2e` — `go test ./...` does NOT run it. Run `go test -tags e2e
          ./internal/e2e/...` explicitly (research: ok 22.4s).

# READ — the PRD testing strategy (authority for "full test suite" scope)
- file: PRD.md   (READ ONLY — headings h3.93, h3.94, h3.97)
  section: §20.1 Layers (unit pure-fns; unit git-wrapper w/ real git; integration stub provider; integration
           real agents `//go:build integration_real` opt-in NOT in CI); §20.2 invariants; §20.5 e2e scenario
           harness (throwaway-repo, `//go:build e2e`, drives the real agent or a stub).
  why: authoritative basis for treating the e2e harness as IN scope ("the suite includes the e2e harness")
       and the integration_real suite as COMPILE-ONLY (opt-in, not CI). Read-only; do not edit.
```

### Current Codebase tree (relevant slice)

```bash
Makefile                      # build/test/coverage-gate/lint targets; BIN := bin/stagecoach
go.mod                        # module github.com/dustin/stagecoach  (go 1.22)
cmd/stagecoach/main.go        # var version="dev"; resolveVersion(); cmd.Version set before Execute
cmd/stubagent/                # the stub agent the stubtest/e2e suites invoke (no test files)
internal/cmd/root.go          # rootCmd.Use="stagecoach"; STAGECOACH_* + stagecoach.* in flag usage
internal/cmd/*_test.go        # unit tests for the CLI (in default suite)
internal/stubtest/*_test.go   # stub-agent integration (NO build tag → in default suite)
internal/e2e/*_test.go        # //go:build e2e  → NOT in default suite; run with -tags e2e
internal/generate/realagent_test.go  # //go:build integration_real → compile-only in CI
internal/{git,provider,generate,config}/  # the 4 coverage-gate packages (≥85%)
pkg/stagecoach/stagecoach_test.go  # public-package test (in default suite)
bin/stagecoach                # produced by `make build` (gitignored; the contract's output artifact)
```

### Desired Codebase tree with files to be changed

```bash
# ZERO file changes expected (research: everything is already green + zero stagehand in .go/testdata).
# `make build` regenerates bin/stagecoach (gitignored build output — not a tracked change).
# IF (and only if) a stagehand straggler breaks build/test/identity: fix THAT test/build-surface file only
#   (locate via the remediation playbook). No new files. No plan/, PRD.md, tasks.json, production-code edits.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the one real subtlety): default `go test ./...` does NOT run the e2e harness or the real-agent
# suite — both are //go:build-gated. The item says the suite "includes ... the e2e harness", so a correct
# certification runs BOTH:
#     go test ./... -count=1                          # unit + stubtest (no build tag)
#     go test -tags e2e ./internal/e2e/... -count=1   # the e2e harness (//go:build e2e)
# and COMPILES (not runs) the real-agent suite:
#     go vet -tags integration_real ./internal/generate/...   # //go:build integration_real
# Running only step (b) satisfies the literal command but silently skips the e2e harness.

# CRITICAL (integration_real is COMPILE-ONLY in CI): it runs ONLY under -tags integration_real AND
# STAGECOACH_RUN_REAL=1 AND with real pi/claude/gemini/etc. CLIs installed (PRD §20.1 layer-4, opt-in).
# CI cannot guarantee those. Compile it (`go vet -tags integration_real ...`); do not run it.

# CRITICAL (`--version` shows `-dirty` while the rename changeset is uncommitted): resolveVersion() appends
# "-dirty" via debug.ReadBuildInfo (vcs.modified) when the tree is modified. Expected output during this
# task: `stagecoach version dev (<7-hex>-dirty)`. The -dirty is NOT a failure — it reflects the uncommitted
# rename work. A clean tree shows `dev (<7-hex>)`; a tagged release shows `vX.Y.Z`.

# GOTCHA (where the identity comes from — no custom template): there is NO SetVersionTemplate/SetHelpTemplate
# anywhere. cobra's default version template is `{{.Name}} version {{.Version}}` → "stagecoach version <V>"
# (.Name() = first token of rootCmd.Use = "stagecoach"). --help's `Usage: stagecoach [flags]` is likewise
# derived from rootCmd.Use. So a correct rootCmd.Use ⇒ correct identity automatically.

# GOTCHA (the stubtest package IS in the default suite): internal/stubtest has NO build tag, so it is one of
# the 17 packages `go test ./...` runs. The e2e package DOES have a tag. Don't conflate them — stubtest
# (stub-agent integration) is covered by step (b); e2e (subprocess of the compiled binary) needs -tags e2e.

# GOTCHA (bin/ is gitignored): `make build` writes bin/stagecoach (and make clean removes it). It is a build
# artifact, not a tracked file; `git status` will NOT show it. The contract's "must produce ./bin/stagecoach"
# is satisfied by the file existing after `make build`, not by it being tracked.

# GOTCHA (the local checkout dir is /home/dustin/projects/stagehand): an untracked working-tree PATH, not a
# tracked file. `go build`/`go test`/`git grep` do not see it; it ships in no artifact. OUT OF SCOPE (the
# sibling S1 PRP documents the same). Do not try to "fix" the directory name.

# GOTCHA (make test adds -race; the contract step (b) does not): `make test` = `go test -race ./...`. The
# contract's step (b) is the bare `go test ./... -count=1`. Both are run: (b) is the contract gate; `make
# test` is the bonus project gate that also exercises the race detector (research: both green).

# GOTCHA (do NOT run S1's grep audit as this task): the whole-repo `git grep -i stagehand` count is the
# sibling P1.M5.T2.S1's deliverable (it is >0 by design: plan/012 docs, tasks.json, PRD directive). This
# task's residue check is SCOPED to the compiled/test surface: `git grep -il 'stagehand' -- '*.go'` and
# testdata — both are 0 and that is what matters for build/test/identity.
```

## Implementation Blueprint

### Data models and structure

_None._ This is a verification task: run commands, assert on output, record results. No data models, no new
files, no production-code edits (unless a stagehand straggler breaks a test — then a scoped test-surface fix).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: BUILD — `make build` produces ./bin/stagecoach
  - RUN: `make build`   (Makefile target: `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`)
  - ASSERT: exit code 0; `test -x bin/stagecoach` succeeds (executable binary exists).
  - EXPECTED: stdout ends `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`; no errors.
  - WHY: step (a). Confirms the renamed module path (github.com/dustin/stagecoach) and entry point
      (./cmd/stagecoach) compile and the binary is named stagecoach.

Task 2: VET — `go vet ./...` clean
  - RUN: `go vet ./...`
  - ASSERT: exit code 0; no output (clean).
  - EXPECTED: silent success.
  - WHY: step (c). Catches rename residue that compiles but is vet-suspicious (unused imports of a renamed
      path, printf mismatches, etc.). Run BEFORE tests so a vet error isn't masked by test output.

Task 3: DEFAULT TEST SUITE — `go test ./... -count=1` (unit + stub-agent integration)
  - RUN: `go test ./... -count=1`
  - ASSERT: exit code 0; 0 packages FAIL. Expect ~17 ok, 2 no-test (cmd/stagecoach has no _test.go; cmd/stubagent
      has none). internal/stubtest (no build tag) is INCLUDED here.
  - IF FAIL: capture `go test ./... -count=1 2>&1 | grep -E 'FAIL|---'`, read the failing test + assertion.
      If it asserts on a "stagehand" string/path/env/fixture, apply Task 6 (scoped test-surface fix). Re-run.
  - WHY: step (b) — the literal contract command. Covers unit (internal/*) + the stub-agent integration
      (internal/stubtest) + public package (pkg/stagecoach). Does NOT cover e2e (Task 4) or real-agent (Task 5).

Task 4: E2E HARNESS — `go test -tags e2e ./internal/e2e/...` (build-tag-gated; the item names it)
  - RUN: `go test -tags e2e ./internal/e2e/... -count=1`
  - ASSERT: exit code 0; `ok github.com/dustin/stagecoach/internal/e2e`. (Research: ~22s; harness self-builds
      cmd/stagecoach into a temp dir, seeds throwaway git repos, drives the binary as a subprocess.)
  - IF FAIL: `go test -tags e2e ./internal/e2e/... -count=1 -v 2>&1 | grep -A15 '--- FAIL'`. The harness
      exercises real CLI routing + config-load + real-repo plumbing; a failure is either a binary-name/path
      mismatch or a history/output assertion embedding the old name. Apply Task 6 if rename-related.
  - WHY: the item explicitly says the suite "includes ... the e2e harness" but `//go:build e2e` excludes it
      from Task 3. Running it explicitly is the difference between "the literal command passed" and "the
      suite the item describes actually passed".

Task 5: REAL-AGENT SUITE — compile-only (`go vet -tags integration_real`)
  - RUN: `go vet -tags integration_real ./internal/generate/...`
  - ASSERT: exit code 0; clean (the realagent_test.go compiles under the tag).
  - DO NOT RUN the suite: it needs `-tags integration_real` AND `STAGECOACH_RUN_REAL=1` AND real
      pi/claude/gemini/etc. CLIs installed (PRD §20.1 layer-4, opt-in, NOT in CI). Running it will skip (env
      off) or fail (agent missing) — neither proves anything about the rename.
  - WHY: proves the rename left no straggler that breaks the real-agent suite's COMPILATION, which is all CI
      can guarantee. (If a real agent IS installed and you want a manual smoke, `STAGECOACH_RUN_REAL=1 go
      test -tags integration_real ./internal/generate/...` — optional, not required.)

Task 6: RUNTIME IDENTITY — `--version` and `--help` (THE identity assertions)
  - RUN: `./bin/stagecoach --version`
      ASSERT: output matches `^stagecoach version dev( \([0-9a-f]{7}(-dirty)?\))?$` OR `^stagecoach version v…`.
      EXPECTED (dirty tree, this task): `stagecoach version dev (<7-hex>-dirty)`.
      ASSERT: `./bin/stagecoach --version 2>&1 | grep -i stagehand` → no output.
  - RUN: `./bin/stagecoach --help`
      ASSERT: the `Usage:` block shows `stagecoach [flags]` and `stagecoach [command]`.
      ASSERT: the `Available Commands:` list includes `config`, `hook`, `integrate`, `models`, `providers`.
      ASSERT: the `Flags:` block contains `STAGECOACH_PROVIDER` (or any `STAGECOACH_` env) AND `stagecoach.` git key.
      ASSERT: `./bin/stagecoach --help 2>&1 | grep -i stagehand` → no output (THE identity-leak gate).
  - WHY: steps (d)+(e). A green build says nothing about whether --help still prints "stagehand" in a flag
      usage string or subcommand Short. These assertions are the net that catches an identity leak.

Task 7: RESIDUE CHECK (compiled/test surface only — do NOT run S1's whole-repo audit)
  - RUN: `git grep -il 'stagehand' -- '*.go'`            # ASSERT: no output (0 files).
  - RUN: `git grep -li 'stagehand' -- 'internal/*/testdata/*'`   # ASSERT: no output (0 testdata files).
  - WHY: confirms no rename residue in any file that is compiled or read by a test. (The whole-repo count is
      S1's deliverable and is >0 by design — plan/012 docs, tasks.json, PRD directive — all non-compiled.)
  - GOTCHA: do NOT widen this to a repo-wide `git grep -i stagehand | wc -l`; that is S1's audit and its >0
      is the documented exceptions, NOT a failure of this task.

Task 8: BONUS PROJECT GATES (the Makefile's own QA — record, not strictly required by the contract)
  - RUN: `make test`               # go test -race ./...  → ASSERT 0 FAIL (research: 17 ok).
  - RUN: `make coverage-gate`      # ASSERT PASS (4 core pkgs ≥85%; research: git 88.7, provider 91.1,
      # generate 90.4, config 86.2).
  - OPTIONAL: `make lint` (golangci-lint) — only if v1.61 installed (CI pin); the sibling S1's .golangci.yml
      path-fix (pkg/stagehand→pkg/stagecoach) restores the errcheck/unused suppression for stagecoach_test.go.
      NOT a contract step; skip if the tool/version is unavailable (go vet from Task 2 is the smoke check).
  - WHY: these are the project's defined QA gates (PRD §20.3, §20.1); recording them green strengthens the
      "full test suite passes" claim beyond the contract's literal 5 steps.

Task 9: RECORD + SCOPE AUDIT
  - RECORD (in the implementation summary, NOT a new file — DOCS: none): each step's command + observed
      output (esp. the exact --version string and a snippet of --help showing stagecoach identity); the e2e
      run; the integration_real compile; the residue check (0 .go, 0 testdata); any fix applied (none expected).
  - SCOPE: `git status --short` shows ONLY build output (bin/, gitignored → not shown) and any Task 6 fix
      (none expected). NOTHING in plan/, PRD.md, tasks.json, or production .go beyond a test-surface fix.
      `git status --porcelain | grep -cE 'tasks\.json|^.. PRD\.md|plan/012_963e3918ec08'` → 0 (untouched).
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the full certification sequence (run in order; each verified green in research).
make build                                                    # (a) → bin/stagecoach
go vet ./...                                                  # (c) clean (run before tests)
go test ./... -count=1                                        # (b) unit + stubtest; ~17 ok, 0 FAIL
go test -tags e2e ./internal/e2e/... -count=1                 # e2e harness (NOT in default suite) → ok ~22s
go vet -tags integration_real ./internal/generate/...         # real-agent compile-only (do NOT run)
./bin/stagecoach --version                                    # (d) "stagecoach version dev (<sha>-dirty)"
./bin/stagecoach --help | head -20                            # (e) Usage: stagecoach [flags]; STAGECOACH_*; stagecoach.*

# PATTERN: the identity-leak gates (the assertions a green build cannot make for you).
./bin/stagecoach --version 2>&1 | grep -i stagehand          # MUST be empty
./bin/stagecoach --help    2>&1 | grep -i stagehand          # MUST be empty
./bin/stagecoach --help    2>&1 | grep -c 'STAGECOACH_'      # MUST be >0 (env vars present)
./bin/stagecoach --help    2>&1 | grep -c 'stagecoach\.'     # MUST be >0 (git config keys present)

# PATTERN: the compiled-surface residue check (scoped — NOT S1's whole-repo audit).
git grep -il 'stagehand' -- '*.go'                           # MUST be empty (0 files)
git grep -li 'stagehand' -- 'internal/*/testdata/*'          # MUST be empty (0 testdata files)

# PATTERN (IF a test fails on a stale "stagehand" assertion — currently none): locate, read, fix the
# EXPECTATION (test-only), re-run. Never "fix" by reintroducing stagehand into production code.
go test ./... -count=1 2>&1 | grep -E 'FAIL|---' | head      # the failing package/test
# then read the failing _test.go; if it expects a "stagehand" string/path/env, update it to stagecoach.
# Check the package's testdata/ for golden/fixture files embedding the old name too.

# CRITICAL: run `-tags e2e` explicitly. `go test ./...` excludes //go:build e2e AND //go:build integration_real.
# CRITICAL: compile (vet) integration_real; do not run it (needs real CLIs + STAGECOACH_RUN_REAL=1).
# CRITICAL: --version's "-dirty" suffix is EXPECTED while the rename changeset is uncommitted (not a failure).
# CRITICAL: do NOT run S1's whole-repo `git grep -i stagehand | wc -l` as this task's gate — it is >0 by
#   design (plan/012 docs, tasks.json, PRD directive) and is S1's deliverable, not a build/test failure.
```

### Integration Points

```yaml
RENAME CLOSE-OUT (PRD h2.30):
  - S1 (text surface) + S2 (build/test/runtime surface) together close the rename. S1 certifies no stagehand
    in the repo's TEXT (with documented exceptions); S2 certifies the project BUILDS, its TESTS pass, and the
    BINARY presents the right identity. Both required; neither subsumes the other.
  - S1's 3 fixes (git.go comment, .goreleaser.yaml comment, .golangci.yml path) are comment/path-only and do
    NOT affect S2's build/vet/test outcomes. The .golangci.yml path-fix only matters if S2 runs `make lint`
    (a bonus gate, not a contract step) — it restores the errcheck/unused suppression for stagecoach_test.go.

BUILD SYSTEM (Makefile):
  - `make build` → bin/stagecoach (the contract's output artifact; gitignored, regenerated each build).
  - `make test` (race) and `make coverage-gate` (PRD §20.3 ≥85%) are the project's own QA gates; running
    them green strengthens the "full test suite passes" claim.

DOWNSTREAM (P1.M5.T3.S1/S2 — documentation sync): run AFTER the rename is verified build/test-clean. Their
  "documentation internal consistency" and "badge/GitHub-link/distribution-path" checks assume a building,
    passing, correctly-identified binary — which this task certifies.
```

## Validation Loop

### Level 1: Build + vet (immediate)

```bash
make build && test -x bin/stagecoach && echo "BUILD OK"      # Expect: BUILD OK (bin/stagecoach executable)
go vet ./...                                                  # Expect: clean (exit 0, no output)
# Expected: BUILD OK; vet silent. If vet fails, read the file:line — a rename straggler; fix and re-run.
```

### Level 2: Test suite (default + e2e + real-agent compile)

```bash
go test ./... -count=1                                        # (b) Expect: 0 FAIL (~17 ok, 2 no-test)
go test -tags e2e ./internal/e2e/... -count=1                 # e2e harness (NOT in default) Expect: ok ~22s
go vet -tags integration_real ./internal/generate/...         # real-agent compile Expect: clean
# Expected: all green. If a test FAILs, capture it, read the assertion; if it expects "stagehand", fix the
# test expectation (test-only) and re-run. Do NOT run integration_real as a test (needs real CLIs).
```

### Level 3: Runtime identity (`--version` + `--help`)

```bash
./bin/stagecoach --version                                    # (d) Expect: "stagecoach version dev (<7-hex>-dirty)"
./bin/stagecoach --version 2>&1 | grep -i stagehand          # Expect: NO output (identity-leak gate)
./bin/stagecoach --help 2>&1 | grep -E '^Usage:|^  stagecoach'  # Expect: "Usage:" + "  stagecoach [flags]"
./bin/stagecoach --help 2>&1 | grep -c 'STAGECOACH_'          # Expect: >0 (env vars present)
./bin/stagecoach --help 2>&1 | grep -c 'stagecoach\.'         # Expect: >0 (git config keys present)
./bin/stagecoach --help 2>&1 | grep -i stagehand              # Expect: NO output (identity-leak gate)
# Expected: version prefixed "stagecoach version "; help shows stagecoach command + STAGECOACH_* + stagecoach.*;
# no "stagehand" anywhere in either output.
```

### Level 4: Residue + bonus gates + scope audit

```bash
# Compiled/test-surface residue (scoped — NOT S1's whole-repo audit):
git grep -il 'stagehand' -- '*.go'                            # Expect: no output (0 files)
git grep -li 'stagehand' -- 'internal/*/testdata/*'           # Expect: no output (0 testdata files)

# Bonus project QA gates (Makefile-defined):
make test                                                     # race; Expect: 0 FAIL
make coverage-gate                                            # Expect: PASS (4 core pkgs ≥85%)
make lint 2>/dev/null || echo "(golangci-lint v1.61 not installed; go vet is the smoke check — OK)"  # optional

# Scope audit (only build output / a test-surface fix; nothing forbidden touched):
git status --short                                            # Expect: at most a test-surface fix (none expected); bin/ is gitignored
git status --porcelain | grep -cE 'tasks\.json|^.. PRD\.md|plan/012_963e3918ec08'   # Expect: 0 (untouched)
# Expected: compiled/test surface clean; bonus gates green; scope respected.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: `make build` → `bin/stagecoach` executable; `go vet ./...` clean.
- [ ] Level 2: `go test ./... -count=1` → 0 FAIL; `go test -tags e2e ./internal/e2e/...` → 0 FAIL;
      `go vet -tags integration_real ./internal/generate/...` clean.
- [ ] Level 3: `--version` starts `stagecoach version ` (no stagehand); `--help` shows stagecoach command +
      STAGECOACH_* env + stagecoach.* git keys (no stagehand); both grep-for-stagehand gates empty.
- [ ] Level 4: 0 stagehand in *.go; 0 in testdata; bonus gates green (make test, make coverage-gate);
      scope respected (no plan/, PRD.md, tasks.json, production-code changes).

### Feature Validation
- [ ] Step (a) `make build` → `./bin/stagecoach` exists, executable.
- [ ] Step (b) `go test ./... -count=1` → passes (unit + stub-agent integration in internal/stubtest).
- [ ] Step (c) `go vet ./...` → clean.
- [ ] Step (d) `./bin/stagecoach --version` → prints `dev` (or `dev (<sha>…)` or `vX.Y.Z`); no stagehand.
- [ ] Step (e) `./bin/stagecoach --help` → `stagecoach` command name, `STAGECOACH_*` env vars, `stagecoach.*`
      git config keys; no stagehand.
- [ ] The e2e harness (the suite the item names but default `go test ./...` hides) passes under `-tags e2e`.
- [ ] The integration_real suite compiles under `-tags integration_real` (compile-only; not run in CI).

### Code Quality Validation
- [ ] No production-code BEHAVIOR changed (verification only; any fix is a test/build-surface expectation).
- [ ] No new files created (DOCS: none — verification step; results recorded in the implementation summary).
- [ ] The build-tag blind spot was handled: BOTH the default suite AND `-tags e2e` were run; integration_real
      was compiled (not run), with the rationale recorded.
- [ ] Scope respected: PRD.md, tasks.json, plan/012, plan/001–011, and S1's 3 files UNTOUCHED (unless a fix
      was required in a test file — none expected).

### Documentation & Deployment
- [ ] DOCS: none (verification step — no new doc file). The certification results (commands + observed
      outputs, esp. the exact --version string and the --help identity snippet) are recorded in the
      implementation summary.
- [ ] The certification command sequence is recorded so the next maintainer can re-run it without re-deriving
      the build-tag set or the identity assertions.

---

## Anti-Patterns to Avoid

- ❌ **Don't run only `go test ./...` and call the suite done.** The e2e harness (`//go:build e2e`) and the
  real-agent suite (`//go:build integration_real`) are EXCLUDED from the default run. The item explicitly
  wants the e2e harness in scope. Run `go test -tags e2e ./internal/e2e/...` explicitly, and compile (vet)
  integration_real. (findings §2/§3)
- ❌ **Don't run the integration_real suite as a test.** It needs `-tags integration_real` AND
  `STAGECOACH_RUN_REAL=1` AND real pi/claude/etc. CLIs installed (PRD §20.1 layer-4, opt-in, NOT in CI).
  Running it skips (env off) or fails (agent missing) — proving nothing. COMPILE it (`go vet -tags
  integration_real ./internal/generate/...`); that is all CI can guarantee. (findings §3)
- ❌ **Don't treat `--version`'s `-dirty` suffix as a failure.** resolveVersion() appends `-dirty` via
  debug.ReadBuildInfo (vcs.modified) when the tree is modified — and the rename changeset makes it dirty.
  `stagecoach version dev (<7-hex>-dirty)` is the EXPECTED output during this task. (findings §4)
- ❌ **Don't run S1's whole-repo `git grep -i stagehand | wc -l` as this task's gate.** It is >0 BY DESIGN
  (plan/012 rename docs, orchestrator-owned tasks.json, the read-only PRD directive) and is the sibling
  P1.M5.T2.S1's deliverable. This task's residue check is SCOPED to the compiled/test surface (`*.go` +
  testdata), which is 0. (findings §5/§6)
- ❌ **Don't touch plan/, PRD.md, tasks.json, or S1's 3 files.** They are not compiled and not in any test
  path; they cannot affect build/test/identity. S1 owns them. The only edit this task may make is a scoped
  fix to a TEST/build-surface file IF a stagehand straggler breaks it (currently none). (findings §6/§8)
- ❌ **Don't "fix" a failing test by reintroducing stagehand into production code.** If a test asserts on a
  stale "stagehand" string/path/env, fix the TEST's expectation to stagecoach (test-only). Production code is
  already fully renamed; reverting any of it would be a regression. (findings §7)
- ❌ **Don't conflate `make test` (race) with the contract's step (b).** `make test` = `go test -race ./...`
  (a bonus project gate). Step (b) is the bare `go test ./... -count=1`. Both are run; step (b) is the
  contract gate. (gotcha)
- ❌ **Don't hide a no-op result.** If everything is already green (research says it is), SAY SO — but STILL
  execute every step (build, vet, default test, e2e, integration_real compile, --version, --help, residue)
  and record the observed outputs. Verification by execution is the deliverable, not file churn. If a fix WAS
  needed, record it precisely (file:line, before/after, which step it unblocked).
