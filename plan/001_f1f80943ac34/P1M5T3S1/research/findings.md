# P1.M5.T3.S1 — Research Findings (GitHub Actions CI matrix)

Decisive findings only. The PRP (../PRP.md) consumes these. All numbers measured live on the
codebase at research time (HEAD = `aaf125a`).

## 1. Codebase facts (verified)

- **Module**: `github.com/dustin/stagecoach` (go.mod); **`go 1.22`**.
- **Main binary**: `./cmd/stagecoach` (Makefile `MAIN_PKG`). Also `./cmd/stubagent`.
- **Core packages** (PRD §20.3 coverage targets): `internal/git`, `internal/provider`,
  `internal/generate`, `internal/config`. Plus `internal/{cmd,prompt,signal,stubtest,ui,exitcode}`,
  `pkg/stagecoach`.
- **No existing CI**: `.github/` does not exist. No `.golangci.yml`, no `.goreleaser.yaml`, no
  `release.yml`. Greenfield.
- **Default branch**: `main` (`* main`, `remotes/origin/main`). Trigger filter = `branches: [main]`.
- **Makefile targets** (the ones CI mirrors): `build` (`go build -ldflags … -o bin/stagecoach
  ./cmd/stagecoach`), `test` (`go test -race ./...`), `coverage` (`go test -coverprofile=coverage.out
  ./...` + `go tool cover -func`), `lint` (`golangci-lint run`), `clean`, `install`.
  - NOTE: `make coverage` does NOT gate (it only prints). A gate is NEW work (this task + S3).
- **`.gitignore`** already ignores `/bin/`, `coverage.out`, `/dist/` — safe; CI artifacts won't pollute.

## 2. COVERAGE BASELINE (THE #1 RISK) — measured via merged `go test -coverprofile ./...`

Statement-weighted per-package coverage (exactly how the gate computes it):

| package              | coverage | stmts covered/total | vs 85% gate |
|----------------------|---------:|---------------------|-------------|
| internal/git         |   94.1 % | 192 / 204           | ✅ pass     |
| internal/provider    |   93.6 % | 262 / 280           | ✅ pass     |
| internal/generate    |   87.1 % | 121 / 139           | ⚠️ close (1 regression drops it under) |
| **internal/config**  |   **83.3 %** | **205 / 246**   | ❌ **FAILS** (~5 statements short: 0.85×246 = 209.1 → need ≥210, have 205) |

Overall repo total: 84.2 %.

**IMPLICATION**: A coverage gate implemented verbatim to PRD §20.3 (≥85 % on all 4) will be **RED on
day one** because `internal/config` is 83.3 %. A RED gate on commit is a broken deliverable. The
implementer MUST resolve this before shipping (see PRP "Coverage-gap decision tree"):
  - (A) **preferred**: add tests covering the ~5 uncovered statements in `internal/config` to cross
    85 %, then ship the gate at the spec'd 85 %. Use `go tool cover -func=coverage.out | grep
    internal/config | grep -v 100.0%` or `-html=coverage.out` to find the gaps.
  - (B) **sanctioned fallback**: ratchet `internal/config`'s floor to its current 83 % with an inline
    `# TODO(§20.3): raise 83→85` note, so the gate is GREEN now and prevents regression.
  - (C) shipping a gate that fails CI = **forbidden**.

`go test -race ./...` is currently green (all packages `ok`). So the build+test gate is safe; only
the coverage floor is a problem.

## 3. Arch-matrix reality (GitHub-hosted runners vs PRD §20.4's 6 OS×arch combos)

PRD §20.4 wants build+test on `{linux, macos, windows} × {amd64, arm64}`. GitHub-hosted runner reality:

| runner label      | OS      | arch  | available? |
|-------------------|---------|-------|------------|
| `ubuntu-latest`   | linux   | amd64 | ✅ free (public repo) |
| `macos-latest`    | macos   | **arm64** (Apple Silicon, macos-14) | ✅ |
| `macos-13`        | macos   | amd64 (Intel) | ✅ |
| `windows-latest`  | windows | amd64 | ✅ |
| linux/arm64       | linux   | arm64 | ⚠️ free arm64 runners are new/restricted; NOT reliably available as a plain label |
| windows/arm64     | windows | arm64 | ⚠️ `windows-11-arm` exists in preview; not stable for v1 |

**Mapping decision**: matrix `os: [ubuntu-latest, macos-latest, macos-13, windows-latest]` gives
**native** (no-emulation, no-flake) test execution on **4 of 6** combos:
linux/amd64, macos/arm64, macos/amd64, windows/amd64. The 2 remaining combos (linux/arm64,
windows/arm64) are covered by a separate lightweight **cross-compile** job (`GOARCH=arm64 go build
./...` for both OSes) — proves arm64 compiles without paying for/suffering QEMU emulation. macos-latest
already gives real arm64 **test** coverage, so the PRD's arm64-test intent is genuinely satisfied for
macos. Full linux/arm64 + windows/arm64 test execution needs QEMU or arm64 runners = documented
optional enhancement, **out of v1 scope** (flaky + slow).

**Avoid**: pretending `ubuntu-latest` covers arm64, or `include:`-ing an arm64 label that 404s.

## 4. Recommended workflow structure (idiomatic split, not one mega-matrix-job)

The contract literally says "a matrix job … Steps: checkout, setup-go, go test -race, golangci-lint
run, govulncheck … coverage gate". Running lint+vulncheck+coverage on all 8 matrix cells (4 os × 2
go) is wasteful and flaky (golangci-lint on Windows is slow; coverage is arch-independent). The
idiomatic, one-pass-safe structure = separate jobs, all gating the PR:

- **`build-test`** (matrix os×go): checkout, setup-go (cache), `go build ./...`, `go test -race ./...`.
- **`lint`** (ubuntu, one Go): `golangci/golangci-lint-action`.
- **`vulncheck`** (ubuntu, one Go): `golang/govulncheck-action`.
- **`coverage`** (ubuntu, one Go): `go test -coverprofile=coverage.out ./...` + the self-contained
  ≥85 % gate (awk on the cover profile).

Every contract deliverable is present (build+test on the matrix; golangci-lint; govulncheck;
coverage gate ≥85 %). Splitting = faster, cheaper, clearer failure attribution, no Windows-lint flake.

## 5. Actions/versions to pin (verified current majors)

- `actions/checkout@v4`
- `actions/setup-go@v5` (go-version input per matrix; `cache: true` auto-uses `go.sum` — it exists).
- `golangci/golangci-lint-action@v6` (v6 requires the action to install its own binary; pin
  `version: v1.61` or `latest`). **Note**: v6 of the action dropped some inputs; `version` + `args`
  are still valid.
- `golang/govulncheck-action@v1` (input `go-version` + `go-package: ./...`).
- Alternative to the action: `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (no third-party
  action). The action is cleaner; either is acceptable.

## 6. LINT gate is UNVERIFIED locally — flagged risk

`golangci-lint` is **NOT installed** on this machine (`command -v golangci-lint` → not found), so the
lint gate cannot be pre-verified here. `golangci-lint-action` installs its own binary in CI, so the
gate WILL run — but the codebase's current lint state is unknown. If existing code triggers findings,
the first CI run goes RED. **Mitigation in PRP**: ship a conservative `.golangci.yml`
(errcheck/gosimple/govet/ineffassign/staticcheck/unused; `disable-all: true`), and REQUIRE the
implementer to run `make lint` locally (install golangci-lint if needed) and either fix findings or
relax the config until green BEFORE relying on the gate.

## 7. govulncheck is UNVERIFIED locally — lower risk

`govulncheck` not installed locally either. But the deps are tiny (cobra, pflag, go-toml/v2,
mousetrap) and current; vulncheck is very likely green. If a vuln is found, it surfaces a real
advisory the implementer addresses (update dep) — acceptable, and the action makes it deterministic.

## 8. SCOPE BOUNDARIES (parallel / sibling subtasks — do NOT cross)

- **S1 OWNS** (this task): `.github/workflows/ci.yml` (new) + `.golangci.yml` (new). The
  coverage-gate STEP lives **inline in ci.yml** (self-contained awk).
- **S2 OWNS** (Planned): `.goreleaser.yaml` + the **release-on-tag** job (`release.yml` or a release
  job). **S1 MUST NOT add goreleaser / release-on-tag** — §20.4 mentions it in the same bullet, but
  the orchestrator split it into S2. S1's ci.yml triggers on push(main)+PR only; tag/release is S2.
- **S3 OWNS** (Planned): the `make coverage-gate` **Makefile target**. S1's ci.yml does **NOT** call a
  Makefile coverage target (it doesn't exist yet). S1 keeps the gate inline. After S3 lands, a
  recommended micro-refactor lets ci.yml delegate to `make coverage-gate` (single source of truth) —
  that refactor is **out of S1's scope** (would steal S3's work / couple to a not-yet-existing target).
- **P1.M5.T2.S1** (in-flight, parallel): adds `providers/*.toml` + `internal/provider/referencefiles_test.go`.
  No file conflict with S1. S1's `go test ./...` will pick up that new test; it's green by T2.S1's own
  contract, so S1's gates stay green. No coordination needed beyond "don't edit the same files".
- **Makefile**: S1 does **NOT** modify it (S3 owns Makefile coverage; S1 owns ci.yml only).
- **PRD.md / tasks.json / .gitignore**: READ-ONLY (research agent; never modify).

## 9. `act` for local validation (per contract "validated by running act or pushing to a branch")

`act` is NOT installed locally. Options for the implementer to validate ci.yml without burning GitHub
minutes: (a) install `act` (`brew install act` / the Nektos install script) + Docker, then
`act -j build-test --matrix os:ubuntu-latest` (use the `--matrix` override to run one cell); (b) push
ci.yml to a throwaway branch and watch the Actions tab; (c) at minimum, YAML-lint the file
(`python -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" ci.yml` or `actionlint`) so a syntax
typo never reaches the Actions tab. `actionlint` (rhysd/actionlint) is the best static CI validator.

## 10. Decisive one-liners the implementer needs

- Re-measure the 4 packages (authoritative): `go test -coverprofile=c.out ./... && go tool cover -func=c.out | tail -1` then per-package via the awk in the PRP.
- Find uncovered config lines: `go tool cover -func=coverage.out | grep internal/config | grep -v 100.0%` (or `-html`).
- Local CI dry-run: `act push -j lint` (needs Docker + act).
- Static YAML/CI validation: `actionlint .github/workflows/*.yml`.
