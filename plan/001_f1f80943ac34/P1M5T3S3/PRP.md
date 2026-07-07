---
name: "P1.M5.T3.S3 — Makefile coverage gate (≥85% on core packages)"
description: |

  ADD a `coverage-gate` target to the existing repo-root **Makefile** (P1.M1.T1.S2) that enforces PRD
  §20.3: ≥85% statement coverage on exactly `internal/git`, `internal/provider`, `internal/generate`,
  `internal/config` (NOT `internal/ui` — lower bar, hard to test). It runs
  `go test -coverprofile=coverage.out ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...`,
  parses the resulting profile STATEMENT-WEIGHTED per package, and fails (non-zero exit) if any of the
  four packages is below 85%. Runnable locally (Linux/macOS/Git-Bash) and in CI.

  CONTRACT (P1.M5.T3.S3, verbatim):
    1. RESEARCH NOTE: "PRD §20.3 — ≥85% on internal/git, internal/provider, internal/generate,
       internal/config. Lower bar for internal/ui (hard to test, low risk). Enforced in CI."
    2. INPUT: "Makefile from P1.M1.T1.S2."
    3. LOGIC: "Add a `coverage-gate` Makefile target that runs
       `go test -coverprofile=coverage.out ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...`
       and parses the per-package coverage, failing if any target package < 85%. Use
       `go tool cover -func=coverage.out` output parsing. Mock: none — coverage analysis."
    4. OUTPUT: "A coverage gate target runnable locally and in CI."
    5. DOCS: "none — build tooling."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `.github/workflows/ci.yml` + `.golangci.yml` → S1 (P1.M5.T3.S1, Complete). ci.yml's `coverage:`
      job ALREADY enforces §20.3 via an inline awk (statement-weighted, threshold 85, on the same 4
      packages) on ubuntu-latest. S3 does NOT modify ci.yml (S1 owns it). The Makefile gate MIRRORS
      that awk's algorithm so local and CI produce the same pass/fail (proven in research/).
      OPTIONAL future DRY: replace ci.yml's inline awk with `make coverage-gate` (1-line change to the
      "Enforce >=85%" step) — flagged OPTIONAL below, NOT required, coordinate with S1's owner.
    - `.goreleaser.yaml` + `.github/workflows/release.yml` → S2 (P1.M5.T3.S2, running in parallel).
      S3 does NOT touch either. No file overlap (S2 touches neither the Makefile nor ci.yml).
    - `cmd/stagecoach/main.go`, `go.mod`, `go.sum` → UNCHANGED.
    - `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore` → READ-ONLY (`.gitignore` already ignores
      `coverage.out` — confirmed; no change needed).

  DELIVERABLE (EDIT one existing file):
    EDIT Makefile   # add COVERAGE_* vars + coverage-gate target + .PHONY + help entry. (No new files.)

  SUCCESS: `make coverage-gate` exits 0 and prints the 4 per-package statement-weighted percentages
  (all ≥85%: today 94.1 / 93.6 / 87.1 / 85.4) plus a `PASS` line; `make coverage-gate
  COVERAGE_THRESHOLD=90` exits non-zero (≥1) with per-package `FAIL` lines; `git status --short` shows
  ONLY `Makefile` changed; the existing `build/install/test/coverage/lint/clean/help` targets still work
  (`make test` + `make coverage` unchanged).

---

## Goal

**Feature Goal**: Give Stagecoach a single, local-runnable Makefile target (`make coverage-gate`) that
deterministically enforces PRD §20.3's coverage floor — ≥85% statement coverage on each of the four
core packages (`internal/git`, `internal/provider`, `internal/generate`, `internal/config`) — and that
produces the **same pass/fail decision** as the CI `coverage` job (so a green local run predicts a green
CI run, and a red local run catches the regression before push).

**Deliverable**: An EDIT to the repo-root `Makefile` (P1.M1.T1.S2) that adds:
- a `COVERAGE_THRESHOLD` variable (default `85`, overridable on the command line),
- a `COVERAGE_PKGS` variable listing the four `./internal/<pkg>/...` import patterns,
- a `.PHONY: coverage-gate` recipe that runs `go test -coverprofile=coverage.out` over those four
  packages and parses the profile statement-weighted per package (awk), printing each percentage with an
  `OK`/`FAIL` marker and exiting non-zero if any package is below the threshold,
- a help-comment line so `make help` lists `coverage-gate`.

No new files. No edits to ci.yml, .goreleaser.yaml, release.yml, go.mod, or main.go.

**Success Definition**:
- `make coverage-gate` exits `0` and prints the four packages each `OK` (≥85%) + a `PASS` summary.
- `make coverage-gate COVERAGE_THRESHOLD=90` exits non-zero (make wraps the awk `exit 1` → make exit `2`)
  and prints `FAIL` markers for the packages under 90%.
- The four percentages printed match CI's `coverage` job exactly (statement-weighted: 94.1 / 93.6 /
  87.1 / 85.4 as of this writing) — proving local↔CI parity.
- `git status --short` shows ONLY `Makefile`.
- `make help` lists `coverage-gate`; `make test`, `make coverage`, `make build`, `make clean` are
  unchanged and still work.

## User Persona

**Target User**: the **contributor/maintainer** who edits code in `internal/{git,provider,generate,
config}` and wants to know — *before* pushing — whether their change dropped a core package below 85%
and would break the CI `coverage` job (PRD §20.3 "Enforced in CI with a coverage gate").

**Use Case**: A contributor finishes a feature, runs `make coverage-gate`, sees `internal/config 85.4%  OK
… coverage gate: PASS`, and pushes with confidence. If a refactor drops `internal/config` to 83%, they
see `FAIL` + the exact gap locally and add a test — instead of discovering the breakage in CI 5 minutes
later.

**User Journey**:
1. `make coverage-gate` (or `make coverage-gate COVERAGE_THRESHOLD=88` to ratchet a package).
2. Target runs `go test -coverprofile=coverage.out ./internal/{git,provider,generate,config}/...`.
3. Awk parses the profile statement-weighted per package, prints a 4-line table + `OK`/`FAIL`.
4. Exit `0` on all-pass; non-zero on any-fail (CI step fails).

**Pain Points Addressed**: Today the coverage gate exists ONLY inside ci.yml (an inline awk the
contributor never runs locally). A contributor has no local way to predict the CI coverage job, so they
discover §20.3 regressions only after push. `make coverage-gate` brings the gate to the dev loop.

## Why

- **Realizes PRD §20.3's "coverage gate" as a local-runnable artifact.** §20.3 mandates ≥85% on the four
  core packages, enforced in CI *with a coverage gate*. S1 shipped the CI enforcement (inline awk in
  ci.yml's `coverage:` job). S3 ships the matching **Makefile** target so the gate is one `make` command
  away for every contributor — "runnable locally and in CI" (contract OUTPUT).
- **Shift-left on coverage.** A failing local `make coverage-gate` is found in seconds, not in CI
  minutes. Cheaper feedback, fewer red PRs, faster merges.
- **Local↔CI parity by construction.** The gate parses the profile with the SAME statement-weighted
  algorithm as ci.yml (research/ proves byte-identical percentages: 94.1 / 93.6 / 87.1 / 85.4). So a
  green local run = green CI run, a red local run = red CI run. No "passes locally, fails in CI" trap.
- **Scope discipline**: S3 touches ONLY the Makefile. ci.yml (S1), .goreleaser.yaml + release.yml (S2),
  README (P1.M5.T4) are all out of scope. The optional "CI calls `make coverage-gate`" DRY step is
  flagged, not required.

## What

A self-contained addition to the **Makefile** (full recipe in "Implementation Blueprint"):

1. **Variables** (near the existing `VERSION`/`BIN_DIR` block):
   - `COVERAGE_THRESHOLD := 85` — the floor (PRD §20.3). Overridable: `make coverage-gate COVERAGE_THRESHOLD=88`.
   - `COVERAGE_PKGS := ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...`
     — exactly the four core packages (ui intentionally excluded; PRD gives it a lower bar).
2. **Target** `coverage-gate` (`.PHONY`):
   - Runs `go test -coverprofile=coverage.out $(COVERAGE_PKGS)` (the contract's exact command).
   - Parses `coverage.out` statement-weighted per package via `awk` (the SAME algorithm as ci.yml's
     `coverage:` job): for each profile block `pkg/file.go:start.col,end.col  numStatements hitCount`,
     accumulate `tot[pkg]+=numStatements` and, if `hitCount>0`, `cov[pkg]+=numStatements`. The package's
     percentage is `cov/tot*100`. Compare each of the four FQ packages to `$(COVERAGE_THRESHOLD)`;
     `exit fail` (non-zero) if any is below.
   - Prints a clean 4-line table (`pkg  pct%  OK|FAIL`) + a `PASS`/`FAIL` summary.
   - Module path is derived at make-parse time via `MODULE := $(shell go list -m)` (robust to a module
     rename; matches go.mod automatically — no hardcoded path to drift).
3. **Help**: a `## …` comment on the `coverage-gate:` line so `make help` lists it (existing convention).

### Why the gate parses the raw profile, not `go tool cover -func`

The contract says "Use `go tool cover -func=coverage.out` output parsing." **`go tool cover -func` cannot
drive a correct per-package gate** — proven in `research/decisions_and_proof.md §5`:

- `go tool cover -func=coverage.out` emits per-function `file:line:\tFunc\t<pct>%` lines plus ONE global
  `total:\t(statements)\t<global>%`. On this repo that single aggregate is `90.3%` — the 4-package
  *global* average. That is useless for a **per-package** ≥85% gate (PRD §20.3 judges each package
  independently: `internal/config` is at 85.4% even though the global is 90.3%).
- Deriving per-package from `-func` output requires simple-averaging function percentages — which is NOT
  statement-weighted and DIVERGES from CI's numbers (local could read 86% while CI reads 84% → local
  pass, CI fail). That violates §20.3's "Enforced in CI" (local must predict CI).

**Therefore the gate parses the raw `coverage.out` statement-weighted** — identical to ci.yml's `coverage:`
job, guaranteeing parity. `go tool cover -func` is still available for its actual purpose via the existing
`make coverage` target (human-readable per-function breakdown over all packages). The gate (`coverage-gate`)
and the report (`coverage`) are complementary.

### Success Criteria

- [ ] `make coverage-gate` exits 0; prints 4 packages each `OK` (≥85%) + `coverage gate: PASS`.
- [ ] The 4 percentages match CI (94.1 / 93.6 / 87.1 / 85.4) — parity with ci.yml's `coverage:` job.
- [ ] `make coverage-gate COVERAGE_THRESHOLD=90` exits non-zero (≥1) and prints `FAIL` for under-threshold packages.
- [ ] `internal/ui` is NOT in `COVERAGE_PKGS` (lower bar, per PRD §20.3).
- [ ] `make help` lists `coverage-gate`.
- [ ] `make test`, `make coverage`, `make build`, `make clean` still work (unchanged).
- [ ] `git status --short` shows ONLY `Makefile`.
- [ ] ci.yml, .goreleaser.yaml, release.yml, go.mod, main.go, .gitignore UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the exact, **already-tested**
Makefile recipe (§"Implementation Blueprint" — proven to exit 0 at threshold 85 and non-zero at 90 in
research/); the verified facts about the existing Makefile (target list, `.PHONY` line, help-block
convention, `clean` already removes `coverage.out`); the parity proof (statement-weighted awk yields the
same numbers as CI); and the `go tool cover -func` deviation rationale (proven). The single genuinely
finicky part — Make's `$$`-escaping of the awk script — is given as a copy-paste block that has already
run green on this machine.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T3S3/research/decisions_and_proof.md
  why: THE decisive doc. Proves (a) the 4 packages' current coverage (94.1/93.6/87.1/85.4) so the gate
       passes today, (b) local-4-package-profile == CI-full-profile parity (identical numbers), (c)
       `go tool cover -func` only gives a global total (90.3%) so it CANNOT drive a per-package gate,
       (d) the exact Makefile recipe already runs green (exit 0 @85, exit 2 @90).
  critical: §5 (why -func is unsuitable), §6 (the proven recipe), §7 ($$ escaping + tabs + POSIX-awk).

- file: Makefile   (P1.M1.T1.S2 — the file you EDIT)
  section: the header comment block (Targets list), `VERSION ?= dev`, `.PHONY: build install test
           coverage lint clean help`, `coverage:` target (lines ~40-42), `clean:` (rm coverage.out).
  why: you ADD `coverage-gate` following the EXACT same conventions: a `## …` help comment, a `.PHONY`
       entry, `go test`/`go tool` invocations matching the `coverage:` style, and `coverage.out` as the
       profile path (already removed by `clean:` and ignored by `.gitignore`).
  pattern: existing `coverage:` recipe is `go test -coverprofile=coverage.out ./...` then
           `go tool cover -func=coverage.out`. Mirror that command style for the new target.
  gotcha: recipe lines MUST start with a TAB. The `awk` script's every `$` MUST be `$$`. Do NOT touch
          the existing targets — only ADD.

- file: .github/workflows/ci.yml   (S1 — READ only; the PARITY ANCHOR)
  section: job `coverage:` → step "Generate coverage" (`go test -coverprofile=coverage.out ./...`) and
           step "Enforce >=85% on internal/{git,provider,generate,config}" (the inline awk).
  why: the Makefile gate's awk MUST mirror THIS algorithm (statement-weighted: `tot[pkg]+=$2; if($3>0)
       cov[pkg]+=$2; pct=cov/tot*100`, threshold 85, on the 4 FQ packages) so local==CI. The repo-path
       strings (`github.com/dustin/stagecoach/internal/<pkg>`) are reconstructed in the Makefile from
       `MODULE := $(shell go list -m)` for robustness.
  gotcha: ci.yml is owned by S1 — do NOT edit it. Its inline awk already enforces §20.3 in CI. The
          Makefile gate mirrors it; the optional "CI calls make coverage-gate" change is flagged below.

- file: .gitignore   (READ only — do NOT edit)
  why: line `coverage.out` confirms the profile the gate writes is already ignored → no accidental
       commits. `Makefile` is NOT ignored → your edit is tracked.

- file: go.mod   (READ only)
  why: `module github.com/dustin/stagecoach` is what `go list -m` returns → `MODULE` var value. `go 1.22`
       floor; the awk/`go test -coverprofile` APIs are stable since Go 1.2+.

# --- Go cover toolchain (confirm the profile format + -func limitations) ---
- url: https://pkg.go.dev/testing#hdr-Code_coverage_with_the_test_binary
  why: documents `-coverprofile` and the coverprofile line format
       `name.startline.col,endline.col numStmt count` — the exact fields the awk parses (`$1`=name.range,
       `$2`=numStmt, `$3`=count). Confirms statement-weighting is the correct aggregation.
- url: https://pkg.go.dev/cmd/cover
  why: documents `go tool cover -func=cover.out` output: per-function `file:line: function pct` + a final
       `total: (statements) globalpct`. Confirms it does NOT expose per-package statement counts (the
       reason the gate parses the raw profile instead — see §"Why … not go tool cover -func" above).

# --- Make + awk (the two finicky integration points) ---
- url: https://www.gnu.org/software/make/manual/html_node/Recipe-Syntax.html
  why: recipe lines start with a TAB; a backslash-newline continues a logical line. The awk invocation
       is ONE logical recipe line (so Make runs ONE shell command), built with `\` continuations.
- url: https://www.gnu.org/software/make/manual/html_node/Variables.html
  why: `$(VAR)` expansion happens at make-parse time (before the shell runs). `$$` in a recipe is an
       escaped `$` passed to the shell/awk. Every awk `$1`/`$2`/`3` MUST be `$$1`/`$$2`/`$$3` in the recipe.

# --- PRD (authoritative spec) ---
- doc: PRD.md §20.3 (≥85% on the 4 core packages; lower bar for ui; enforced in CI with a coverage gate),
       §20.4 (CI matrix — note the coverage gate runs on ubuntu-latest only in ci.yml),
       §21.1 (the Makefile itself, from which S3 builds).
```

### Current Codebase tree (relevant slice)

```bash
Makefile                       # ← P1.M1.T1.S2. YOU EDIT THIS FILE: add coverage-gate target + COVERAGE_* vars.
.github/workflows/
  ci.yml                       # S1 (Complete). job `coverage:` already enforces §20.3 inline-awk on ubuntu. UNCHANGED.
  release.yml                  # S2 (parallel). UNCHANGED. (file may not exist yet until S2 lands — irrelevant to S3.)
.goreleaser.yaml               # S2. UNCHANGED. (may not exist yet — irrelevant.)
internal/
  git/        provider/  generate/  config/     # the 4 GATED packages (each has *_test.go).
  cmd/  exitcode/  prompt/  signal/  stubtest/  ui/   # NOT gated (ui has the lower bar; others out of §20.3).
go.mod                         # module github.com/dustin/stagecoach. UNCHANGED.
.gitignore                     # already ignores coverage.out. UNCHANGED.
```

### Desired Codebase tree with files to be added/changed

```bash
Makefile                       # EDIT — add: COVERAGE_THRESHOLD, COVERAGE_PKGS, MODULE vars;
                               #              coverage-gate target (recipe + help comment); .PHONY entry.
# (NO new files. NOTHING else changed.)
```

### Known Gotchas of our codebase & Library Quirks

```makefile
# CRITICAL (#1) — MAKE'S `$$` ESCAPING. In a Makefile recipe, `$` is the variable sigil. Every dollar
#   in the awk script MUST be doubled: `$1`→`$$1`, `$2`→`$$2`, `$3`→`$$3`, and the regex
#   `/:[0-9]+.*$/` → `/:[0-9]+.*$$/`. A single `$` silently expands to empty and the awk breaks with no
#   obvious error. The recipe in §"Implementation Blueprint" already has every `$` doubled and is PROVEN
#   green — copy it verbatim.

# CRITICAL (#2) — TABS, NOT SPACES. Every recipe line (including each awk continuation line) MUST begin
#   with a literal TAB. Spaces cause `*** missing separator. Stop.` If your editor pastes spaces, fix to
#   tabs before saving. (The existing Makefile uses tabs — match it.)

# CRITICAL (#3) — ONE LOGICAL RECIPE LINE for the awk. Each awk source line ends with `\` (backslash)
#   so Make treats the whole `awk '…' coverage.out` as a SINGLE shell command. Otherwise Make runs each
#   line as a separate shell (broken pipes / awk syntax errors). The `\` is a Make continuation, NOT
#   part of the awk (awk sees the joined source). The provided recipe is laid out this way.

# GOTCHA (#4) — WHY NOT `go tool cover -func` (the contract's wording). `go tool cover -func=cover.out`
#   emits per-FUNCTION `pct%` plus ONE global `total: (statements) globalpct%`. It does NOT expose
#   per-package statement counts, so it CANNOT compute a per-package statement-weighted percentage.
#   Proven on this repo: the only aggregate is `90.3%` (the 4-package global), while per-package ranges
#   85.4–94.1%. Simple-averaging function percentages would DIVERGE from CI's numbers → "passes locally,
#   fails in CI." The gate therefore parses the RAW profile (statement-weighted), mirroring ci.yml. `go
#   tool cover -func` stays available via the existing `make coverage` target for the human report.
#   See §"Why … not go tool cover -func" above.

# GOTCHA (#5) — LOCAL MUST PREDICT CI (parity). ci.yml's `coverage:` job runs `go test -coverprofile
#   ./...` then the same statement-weighted awk. The Makefile gate runs `go test -coverprofile` over ONLY
#   the 4 packages. PROVEN identical: each package's coverage comes from its OWN tests (no -coverpkg), so
#   the 4 packages' blocks are byte-identical either way → identical percentages. Do NOT "improve" the
#   gate with -coverpkg or simple-average — it would break parity.

# GOTCHA (#6) — `internal/config` IS AT 85.4% TODAY. The gate is TIGHT on config. That is CORRECT and
#   intended (a gate that never bites is a gate that doesn't exist). Do NOT lower COVERAGE_THRESHOLD to
#   make it "safe" — if a change drops config below 85%, the gate should fail (that's the point). If a
#   package genuinely needs a different floor later, ratchet via a per-package override (see "Ratcheting"
#   note in Implementation Patterns), NOT by lowering the global threshold.

# GOTCHA (#7) — awk PORTABILITY. Use POSIX-only awk (`sub`, `split`, `printf`, `exit`, associative
#   arrays) — works on gawk (Linux), mawk (Debian), BSD awk (macOS), and Git-Bash awk (Windows). No GNU
#   extensions. The CI gate runs on ubuntu-latest (gawk) per ci.yml; locally it works on Linux/macOS
#   natively and on Windows via Git Bash/WSL (which ship make+awk). This matches the existing Makefile's
#   Unix-ish targets (`coverage`, `lint`) — no new portability debt.

# GOTCHA (#8) — `MODULE := $(shell go list -m)` runs `go` at make-parse time. Requires `go` on PATH
#   (it always is for this project). Deriving the module path (instead of hardcoding
#   github.com/dustin/stagecoach) means a future module rename updates the gate automatically. ci.yml
#   hardcodes the same paths; if CI is later wired to call `make coverage-gate` (optional), it inherits
#   this robustness for free.

# GOTCHA (#9) — `coverage.out` LOCATION & LIFECYCLE. The gate writes `coverage.out` at repo root
#   (gitignored, removed by `make clean`). It OVERWRITES any prior `coverage.out` (same as the existing
#   `coverage:` target) — no conflict. `go test` may print `(cached)` lines to stdout; those are cosmetic
#   and do not affect the awk (which reads the file, not stdout).

# GOTCHA (#10) — SCOPE. S3 edits ONLY the Makefile. ci.yml is S1's (already enforces §20.3 inline).
#   .goreleaser.yaml/release.yml are S2's. README is P1.M5.T4's. Do NOT add CI workflow changes, release
#   docs, or install instructions here.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY prerequisites (READ + RUN, no edit)
  - RUN: `grep -n 'coverage\|\.PHONY\|clean:' Makefile` -> see the existing `coverage:` target (~line 40),
    the `.PHONY` line (~line 29), and `clean:` (~line 47). These are the anchors for your edits.
  - RUN: `grep -n coverage.out .gitignore` -> confirm `coverage.out` is ignored (no .gitignore edit needed).
  - RUN: `go list -m` -> `github.com/dustin/stagecoach` (the MODULE value the gate will use).
  - RUN (baseline): `go test -coverprofile=/tmp/baseline.out ./internal/git/... ./internal/provider/...
    ./internal/generate/... ./internal/config/...` and note the 4 self-reported percentages
    (expect 94.1 / 93.6 / 87.1 / 85.4 — if they differ, a prior change shifted coverage; the gate's job is
    to REPORT current truth, not invent numbers).
  - RUN: `grep -n "coverage-gate" Makefile` -> expect NONE (you are adding it).

Task 1: EDIT the Makefile — ADD the three variables
  - INSERT after the existing `LDFLAGS := …` / paths block (around line 27) and before `.PHONY`:
        # --- Coverage gate (PRD §20.3) -------------------------------------------------
        # Statement-weighted per-package floor on the 4 core packages. internal/ui has a lower bar
        # (PRD §20.3 — hard to test, low risk) and is intentionally excluded. Runnable locally and in CI.
        MODULE            := $(shell go list -m)
        COVERAGE_THRESHOLD ?= 85
        COVERAGE_PKGS     := ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...
  - NOTE: `?=` on COVERAGE_THRESHOLD lets `make coverage-gate COVERAGE_THRESHOLD=88` override it.
  - FOLLOW convention: the existing block uses `:=` and a `# --- Section ---` banner — match it.

Task 2: EDIT the Makefile — ADD the `coverage-gate` target (copy §"coverage-gate recipe" VERBATIM)
  - INSERT after the existing `coverage:` target (so the two coverage targets are adjacent).
  - COPY the recipe EXACTLY (it is proven green; the `$$` escaping, TABs, and `\` continuations are load-bearing).
  - NAMING: `coverage-gate` (contract literal). Help comment starts with `## ` (existing convention) so
    `make help` lists it as `coverage-gate  Enforce ≥85% …`.
  - PLACEMENT: one block, right after `coverage:`, before `lint:`.

Task 3: EDIT the Makefile — UPDATE `.PHONY` and the header help block
  - `.PHONY:` line: add `coverage-gate` (keep alphabetical-ish / grouped with coverage). 
  - Header `## Targets:` comment block: add a one-liner for `coverage-gate` matching the existing style
    (e.g. `##   coverage-gate  Fail if any of internal/{git,provider,generate,config} < 85% (PRD §20.3)`).
  - PRESERVE every existing target and comment; ONLY add.

Task 4: VALIDATE the gate (run locally — the contract's "Mock: none — coverage analysis")
  - RUN: `make coverage-gate` -> expect: 4 `OK` lines (94.1/93.6/87.1/85.4) + `coverage gate: PASS`, exit 0.
  - RUN: `make coverage-gate COVERAGE_THRESHOLD=90` -> expect: non-zero exit, `FAIL` markers on packages <90.
  - RUN: `make help` -> expect `coverage-gate` listed.
  - RUN: `make coverage && make test && make build && make clean` -> expect all unchanged/working.

Task 5: SCOPE & CLEANLINESS checks
  - RUN: `git status --short` -> expect ONLY `Makefile`.
  - RUN: `git diff --stat` -> confirm ci.yml, .goreleaser.yaml, release.yml, go.mod, main.go, .gitignore
    UNCHANGED.
  - RUN: `git diff Makefile` -> eyeball: only ADDITIONS (3 vars + 1 target + .PHONY/help line); no
    existing recipe altered; all recipe lines TAB-indented.

Task 6 (OPTIONAL — touches S1's ci.yml; coordinate with S1 owner; NOT required for the contract):
  - IF the team wants a single source of truth (local==CI by running the SAME command), replace the
    inline awk in ci.yml's `coverage:` job step "Enforce >=85% on internal/{git,provider,generate,config}"
    with: `run: make coverage-gate`. This makes CI invoke the Makefile target instead of its own awk copy,
    so they can never drift. ALSO change ci.yml's "Generate coverage" step to drop (the target runs go test
    itself) OR keep `./...` for the human artifact. MINIMAL, REVERSIBLE. If ci.yml is treated as frozen,
    SKIP this — the Makefile gate mirrors ci.yml's algorithm, so local already predicts CI. Either way the
    contract is satisfied.
```

### coverage-gate recipe (copy-pasteable — PROVEN green; preserve TABs, `$$`, and `\` continuations EXACTLY)

> The recipe below has already been executed on this machine (research/ §6): exit 0 at threshold 85,
> exit 2 at threshold 90. **Every `$` is doubled (`$$`); every recipe line begins with a TAB.** Do not
> "tidy" it — the escaping is load-bearing.

```makefile
coverage-gate: ## Enforce >=85% statement coverage on internal/{git,provider,generate,config} (PRD §20.3)
	@echo "coverage gate: threshold=$(COVERAGE_THRESHOLD)% on the 4 core packages (PRD §20.3)"
	go test -coverprofile=coverage.out $(COVERAGE_PKGS)
	@awk -v threshold=$(COVERAGE_THRESHOLD) -v mod='$(MODULE)' '\
	  /^mode:/ { next } \
	  { \
	    f=$$1; sub(/:[0-9]+.*$$/, "", f); \
	    n=split(f, parts, "/"); pkg=""; for (i=1; i<n; i++) pkg = pkg (i>1 ? "/" : "") parts[i]; \
	    tot[pkg]+=$$2; if ($$3+0 > 0) cov[pkg]+=$$2; \
	  } \
	  END { \
	    t[1]=mod "/internal/git"; t[2]=mod "/internal/provider"; \
	    t[3]=mod "/internal/generate"; t[4]=mod "/internal/config"; \
	    fail=0; \
	    for (i=1; i<=4; i++) { \
	      if (!(t[i] in tot)) { printf("  ERROR  %-58s no coverage data\n", t[i]); fail=1; continue } \
	      pct = (tot[t[i]]>0) ? cov[t[i]]/tot[t[i]]*100 : 0; \
	      mark = (pct+0 >= threshold) ? "OK" : "FAIL"; \
	      printf("  %-58s %5.1f%%  %s\n", t[i], pct, mark); \
	      if (pct+0 < threshold) { printf("           >>> %s coverage %.1f%% < %d%% threshold (PRD §20.3)\n", t[i], pct, threshold); fail=1 } \
	    } \
	    if (fail) { printf("  coverage gate: FAIL (one or more packages below %d%%)\n", threshold) } \
	    else      { printf("  coverage gate: PASS (all 4 packages >= %d%%)\n", threshold) }; \
	    exit fail \
	  }' coverage.out
```

**What the awk does** (mirrors ci.yml's `coverage:` job algorithm — statement-weighted):
- Skips the `mode:` header line.
- For each profile block `pkg/file.go:start.col,end.col  numStatements hitCount`: strips the
  `:range` off `$1` to get the file path, then trims the trailing `/file.go` (via `split` on `/` and
  re-joining all but the last element) to get the fully-qualified package `pkg`. Accumulates
  `tot[pkg]+=numStatements` and, if `hitCount>0`, `cov[pkg]+=numStatements`.
- In `END`, for each of the 4 target packages (`$(MODULE)/internal/<pkg>`), computes
  `pct = cov/tot*100`, prints `OK`/`FAIL`, sets `fail=1` if `pct<threshold`, and `exit fail`
  (0 = all pass; 1 = any fail → make exits 2).

### Implementation Patterns & Key Details

```makefile
# PATTERN — variable defaults that stay overridable (match the existing VERSION ?= dev style):
COVERAGE_THRESHOLD ?= 85          # `?=` so `make coverage-gate COVERAGE_THRESHOLD=88` wins on the CLI.

# PATTERN — derive the module path once (robust to a rename; matches go.mod automatically):
MODULE := $(shell go list -m)     # → github.com/dustin/stagecoach

# PATTERN — the four gated packages as a single space-separated list (contract's exact set; ui excluded):
COVERAGE_PKGS := ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...

# PATTERN — recipe = "go test writes the profile" THEN "awk parses it". ONE awk invocation, one exit code:
coverage-gate:
	go test -coverprofile=coverage.out $(COVERAGE_PKGS)
	@awk -v threshold=$(COVERAGE_THRESHOLD) -v mod='$(MODULE)' '…' coverage.out
# (go test failure already aborts the recipe via make's default error propagation; awk only runs on success.)

# RATCHETING (future, not required now): if one package deserves a different floor, give it a per-package
# threshold inside the awk (e.g. split t[] and thr[] arrays) rather than lowering the global floor. This
# keeps the 85% bar honest for the other three. (internal/ui is simply not in COVERAGE_PKGS.)
```

### Integration Points

```yaml
MAKEFILE (the file you EDIT — P1.M1.T1.S2):
  - ADD: 3 vars (MODULE, COVERAGE_THRESHOLD, COVERAGE_PKGS) near the existing paths block.
  - ADD: `coverage-gate` target after the existing `coverage:` target.
  - UPDATE: the `.PHONY:` line (add coverage-gate) and the header `## Targets:` help block (one-liner).
  - PRESERVE: build, install, test, coverage, lint, clean, help — UNCHANGED. `clean:` already `rm`s
    `coverage.out` (no change needed). `coverage.out` is already in `.gitignore` (no change needed).

CI (ci.yml — S1, UNCHANGED by default):
  - ci.yml's `coverage:` job ALREADY enforces §20.3 inline (awk on ubuntu-latest, threshold 85, the 4
    packages). The Makefile gate MIRRORS that awk → local==CI parity (proven). No edit required.
  - OPTIONAL (Task 6, coordinate with S1 owner): swap ci.yml's "Enforce >=85%" step to `make coverage-gate`
    for a single source of truth. Reversible, 1 line. NOT required.

PARALLEL (P1.M5.T3.S2, in-flight): creates .goreleaser.yaml + .github/workflows/release.yml. S3 touches
  neither; S2 touches neither the Makefile nor ci.yml. NO file overlap → no merge conflict between S2/S3.

SCOPE HANDOFFS (do NOT create/edit — owned elsewhere):
  - S1 (P1.M5.T3.S1, Complete): .github/workflows/ci.yml + .golangci.yml.
  - S2 (P1.M5.T3.S2): .goreleaser.yaml + release.yml.
  - P1.M5.T4: README (install/usage docs — `coverage-gate` may be mentioned there later, not here).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# No Go source changes, so gofmt/vet are no-ops. The Makefile is the only artifact — validate IT:

# (a) The Makefile parses (no "missing separator" / syntax errors) and `make help` lists the new target:
make help                       # expect a line: "coverage-gate  Enforce >=85% statement coverage …"
make -n coverage-gate           # dry-run: prints the recipe WITHOUT running — confirms Make accepts it.

# (b) TAB indentation survived the edit (spaces would break Make):
awk '/^coverage-gate:/,/^$/{if(/^[ \t]*[^#[:space:]].*:/||1){l=$0; if(match(l,/^\t/)){}else if(l ~ /^[^[:space:]#]/){}else{print "NON-TAB LINE: "NR": "l}}}' Makefile
# Simpler check: grep that recipe body lines start with a TAB:
grep -nP '^( {1,})\S' Makefile && echo "WARNING: space-indented recipe line found (must be TAB)" || echo "tab-indent OK"

# Expected: `make help` lists coverage-gate; `make -n coverage-gate` prints the go-test + awk lines; no
# space-indented recipe lines.
```

### Level 2: The gate itself (Component Validation — the contract's "Mock: none — coverage analysis")

```bash
# PASS path — default threshold 85; all 4 packages are ≥85% today (94.1/93.6/87.1/85.4):
make coverage-gate
echo "exit=$?"                   # expect exit=0 and a "coverage gate: PASS" line.

# FAIL path — raise the floor to 90 to PROVE the gate bites (config 85.4 & generate 87.1 must FAIL):
make coverage-gate COVERAGE_THRESHOLD=90
echo "exit=$?"                   # expect NON-ZERO (make wraps awk exit 1 → 2) and "FAIL" markers.

# PARITY proof — the 4 percentages MUST equal what `go test` self-reports (and what ci.yml computes):
go test -coverprofile=/tmp/parity.out ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...
# compare the "coverage: X% of statements" lines above to the `make coverage-gate` table — they must match
# to one decimal (94.1 / 93.6 / 87.1 / 85.4).

# Expected: PASS at 85; non-zero at 90; printed percentages == go test's self-report == ci.yml's awk.
```

### Level 3: Integration (System Validation — local toolchain regression)

```bash
# The new target must not disturb the existing Makefile surface:
make coverage       # unchanged: go test -coverprofile ./... && go tool cover -func (over ALL packages)
make test           # unchanged: go test -race ./...
make build          # unchanged: builds ./bin/stagecoach
make clean          # unchanged: rm -rf bin coverage.out dist/

# Confirm the gate works with a COLD profile (no stale coverage.out confusing results):
rm -f coverage.out
make coverage-gate
test ! -f coverage.out.bak          # sanity
ls -la coverage.out                 # gate regenerated it

# Expected: all existing targets green; `coverage.out` regenerated by coverage-gate; no stray files.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Prove ui is NOT gated (PRD §20.3 lower bar):
grep -n 'internal/ui' Makefile && echo "WARNING: ui is in the gate (should be excluded)" || echo "ui correctly excluded"

# Prove scope discipline (ONLY Makefile changed):
git status --short                 # expect exactly: " M Makefile"
git diff --stat -- .github/workflows/ci.yml .goreleaser.yaml .github/workflows/release.yml go.mod go.sum cmd/stagecoach/main.go .gitignore
# expect: empty (no changes to any sibling-owned file)

# Prove the awk is POSIX-portable (no GNU-only features) by running it under a second awk if available:
if command -v mawk >/dev/null; then MAWK=1; fi     # mawk is stricter about GNU-isms than gawk
# (If mawk is present and the gate still works, portability across CI ubuntu + macOS BSD awk is assured.)

# Prove a deleted/renamed package is caught (defensive): temporarily point one entry at a bogus path and
# confirm the gate reports "no coverage data" + fails — then revert. (Optional; documents the guard.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `make help` lists `coverage-gate`; `make -n coverage-gate` prints the recipe (Makefile parses).
- [ ] No space-indented recipe lines (all TAB) — `grep -nP '^( +)\S' Makefile` is empty.
- [ ] `make coverage-gate` exits 0 with `coverage gate: PASS` and the 4 `OK` lines.
- [ ] `make coverage-gate COVERAGE_THRESHOLD=90` exits non-zero with `FAIL` markers.
- [ ] The 4 percentages == `go test`'s self-report == ci.yml's awk (94.1 / 93.6 / 87.1 / 85.4).
- [ ] `make coverage`, `make test`, `make build`, `make clean` still work (unchanged).

### Feature Validation

- [ ] `COVERAGE_PKGS` contains exactly the 4 core packages; `internal/ui` is excluded.
- [ ] `COVERAGE_THRESHOLD` defaults to 85 and is overridable (`?=`, verified via the threshold=90 test).
- [ ] `MODULE := $(shell go list -m)` is used (no hardcoded module path that can drift).
- [ ] The gate uses the contract's exact `go test -coverprofile=coverage.out ./internal/{git,provider,
      generate,config}/...` command.
- [ ] The gate decision is STATEMENT-WEIGHTED from the raw profile (matches ci.yml); it does NOT rely on
      `go tool cover -func` (proven unsuitable for per-package — §"Why … not go tool cover -func").
- [ ] All Success-Criteria bullets under "What" met.

### Code Quality & Scope Validation

- [ ] `git status --short` shows ONLY `Makefile`.
- [ ] ci.yml, .goreleaser.yaml, release.yml, go.mod, go.sum, main.go, .gitignore UNCHANGED.
- [ ] The edit is purely ADDITIVE to the Makefile (no existing target altered; only new vars/target/.PHONY/help).
- [ ] The awk is POSIX (works on gawk/mawk/BSD awk/Git-Bash awk); no GNU-only extensions.
- [ ] Follows existing Makefile conventions (`## ` help comments, `# --- Section ---` banners, `:=`/`?=`).

### Documentation & Deployment

- [ ] The `coverage-gate:` line has a `## …` help comment (so `make help` documents it).
- [ ] The header `## Targets:` block lists `coverage-gate`.
- [ ] (Optional Task 6) If ci.yml was wired to call `make coverage-gate`, the commit message notes it was
      coordinated with S1's owner; otherwise the commit notes ci.yml already enforces §20.3 inline and the
      Makefile gate mirrors it.

---

## Anti-Patterns to Avoid

- ❌ Don't parse `go tool cover -func` output to make the gate decision — it has no per-package statement
      weights (only a global total); it would diverge from CI. Parse the raw `coverage.out` (proven).
- ❌ Don't forget to double every `$` (`$$1`/`$$2`/`$$3`/`/…$$/`) in the awk recipe — a single `$`
      silently breaks the gate. Copy the proven recipe verbatim.
- ❌ Don't use spaces to indent recipe lines — TABs only (`*** missing separator`).
- ❌ Don't split the awk across multiple separate recipe lines (without `\` continuation) — it must be ONE
      logical line so Make runs one shell command.
- ❌ Don't add `-coverpkg=./...` or simple-average the function percentages to "improve" the gate — it
      breaks local↔CI parity.
- ❌ Don't lower `COVERAGE_THRESHOLD` because `internal/config` (85.4%) feels tight — the tightness IS the
      gate working. Ratchet per-package if a real exception is needed; keep 85 global.
- ❌ Don't edit ci.yml, .goreleaser.yaml, release.yml, go.mod, or main.go — S3 is Makefile-only.
- ❌ Don't add `coverage.out` to `.gitignore` (it's already there) or commit `coverage.out`.
- ❌ Don't gate `internal/ui` (PRD §20.3 gives it a lower bar — it is intentionally excluded).
