# P1.M5.T3.S3 — Coverage Gate: Research, Decisions, and Proof

> Work item: **Makefile coverage gate (≥85% on core packages)** — PRD §20.3.
> Scope: add a `make coverage-gate` target to the existing Makefile (P1.M1.T1.S2).

## 1. Environment facts (verified on this machine)

- Go toolchain: `go version go1.26.4 linux/amd64` (repo `go.mod` floor is `go 1.22`).
- `make` → `/usr/bin/make` (GNU make). `awk` → `/usr/bin/awk` (gawk).
- Repo module path: `module github.com/dustin/stagecoach` (go.mod line 1) → `go list -m` = `github.com/dustin/stagecoach`.
- `.gitignore` already ignores `coverage.out` and `Makefile` is NOT gitignored (good — it's source).

## 2. Current per-package coverage of the 4 core packages (the gate's subjects)

Command: `go test -coverprofile=coverage.out ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...`

`go test` self-reported (statement-weighted, authoritative per-package):

| Package             | Coverage | vs 85% |
|---------------------|---------:|:------:|
| internal/git        |    94.1% | OK     |
| internal/provider   |    93.6% | OK     |
| internal/generate   |    87.1% | OK     |
| internal/config     |    85.4% | OK (TIGHT) |

**Conclusion: the gate PASSES on first run today.** Note `internal/config` is at 85.4% — barely
clearing. The gate is real: any regression dropping config ≥0.5pp fails it. This is the intended
behavior of a coverage gate (it catches regressions, it does not lower the bar).

## 3. CI already enforces §20.3 (S1, Complete) — the parity anchor

`.github/workflows/ci.yml` job `coverage:` (S1) already runs, on `ubuntu-latest`:

```
go test -coverprofile=coverage.out ./...
# then an inline awk that parses coverage.out, statement-weighted, threshold=85, on the 4 FQ packages
```

The awk in ci.yml computes **statement-weighted** per-package coverage directly from the raw
coverprofile fields: `$1`=block id (`pkg/file.go:start.col,end.col`), `$2`=numStatements,
`$3`=hitCount. It sums `tot[pkg]+=$2` and (if hit) `cov[pkg]+=$2`, then `pct = cov/tot*100`.

## 4. PROOF: local 4-package profile == CI's full `./...` profile (parity)

Ran ci.yml's **exact** awk against a profile generated from ONLY the 4 packages:

```
github.com/dustin/stagecoach/internal/git        94.1%
github.com/dustin/stagecoach/internal/provider   93.6%
github.com/dustin/stagecoach/internal/generate   87.1%
github.com/dustin/stagecoach/internal/config     85.4%
exit=0
```

Identical to `go test`'s self-report. **Why identical:** Go computes each package's coverage from its
OWN tests hitting its OWN code (no `-coverpkg`). Whether you run `./...` or just the 4 packages, the
per-package blocks for those 4 are byte-identical → identical statement-weighted percentages. So the
Makefile gate (4 packages) and CI (`./...` filtered to 4) **cannot diverge in their decision**.

## 5. DECISION: gate parses raw `coverage.out` (statement-weighted), NOT `go tool cover -func`

The contract says "Use `go tool cover -func=coverage.out` output parsing." After analysis this is
**not suitable as the gate's source of truth**:

`go tool cover -func=coverage.out` emits, per function: `file:line:\tFuncName\t<pct>%`, plus one final
global line `total:\t(statements)\t<globalPct>%`. It does **NOT** expose per-package statement counts.
Verified on this repo: the only aggregate line is:

```
total:                          (statements)            90.3%
```

That 90.3% is the 4-package GLOBAL average — useless for a **per-package** ≥85% gate (PRD §20.3 wants
each package judged independently). The only way to derive per-package from `-func` output is to
simple-average the function percentages, which is NOT statement-weighted and DIVERGES from CI's numbers
(e.g. local could read 86% while CI's statement-weighted reads 84% → local pass, CI fail). That is a
footgun that violates PRD §20.3's "Enforced in CI" (local must predict CI).

**Therefore the gate mirrors ci.yml: parse the raw `coverage.out` statement-weighted.** `go tool cover`
is still USED by the repo for its actual purpose — the existing `make coverage` target prints
`go tool cover -func=coverage.out` for the human-readable per-function breakdown. The gate
(`coverage-gate`) and the report (`coverage`) are complementary targets. This honors the contract's
intent (use the Go cover toolchain) while producing a correct, CI-matching gate.

## 6. PROOF: the exact Makefile recipe works (escaping, parity, pass/fail)

Built and ran a throwaway Makefile with this recipe (the `$$` escaping is the critical part):

```make
MODULE := $(shell go list -m)
COVERAGE_THRESHOLD := 85
COVERAGE_PKGS := ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...
coverage-gate:
	go test -coverprofile=coverage.out $(COVERAGE_PKGS)
	@awk -v threshold=$(COVERAGE_THRESHOLD) -v mod='$(MODULE)' '\
	  /^mode:/ { next } \
	  { f=$$1; sub(/:[0-9]+.*$$/, "", f); \
	    n=split(f, parts, "/"); pkg=""; for (i=1; i<n; i++) pkg = pkg (i>1 ? "/" : "") parts[i]; \
	    tot[pkg]+=$$2; if ($$3+0 > 0) cov[pkg]+=$$2 } \
	  END { t[1]=mod "/internal/git"; ...; fail=0; for (i=1;i<=4;i++){ ...compare pct<threshold... } exit fail }' coverage.out
```

Results:
- `make coverage-gate` (threshold 85) → printed the 4 percentages, `OK` each, `coverage gate: PASS`, **exit 0**.
- `make coverage-gate COVERAGE_THRESHOLD=90` → **exit 2** (make's recipe-failure code; awk `exit 1`
  wrapped by make → 2). Non-zero ⇒ CI step fails as required.

So the recipe is copy-paste-ready for the PRP.

## 7. Makefile/awk gotchas captured for the PRP

- **`$$` escaping**: every `$` in the awk script is `$$` in a Makefile recipe (`$1`→`$$1`, `$2`→`$$2`,
  `$3`→`$$3`, and the regex `/:[0-9]+.*$$/`). `$` alone is Make's variable sigil and would break.
- **Line-continuation**: each awk line ends with `\ ` (backslash-space) so Make treats the whole awk
  invocation as ONE logical recipe line (one shell command). The awk itself uses `\` for Make only.
- **TABS, not spaces**: recipe lines MUST start with a TAB (hard requirement of Make). The edit must
  preserve tab indentation. (When writing the PRP recipe, mark this explicitly.)
- **POSIX awk only**: `sub/split/printf/exit/arrays` are POSIX → works on gawk, mawk, BSD awk (macOS),
  and Git-Bash awk (Windows). No GNU-only features (no `gawk`-specific extensions).
- **`-v var=$(MAKE_VAR)`**: Make expands `$(COVERAGE_THRESHOLD)` and `$(MODULE)` BEFORE awk runs, so
  `awk -v threshold=85 -v mod=github.com/dustin/stagecoach`. Clean injection, no quoting issues for
  these identifier-safe values.

## 8. Portability / OS matrix (PRD §20.4)

- CI runs the coverage gate on **ubuntu-latest only** (ci.yml `coverage:` job). No Windows/macOS
  coverage gate in CI. So awk-on-Linux is the CI contract.
- Locally: Linux/macOS have make+awk natively. Windows: works under Git Bash / WSL (both ship make+awk);
  native cmd.exe does not. This matches the existing `coverage`/`lint` Makefile targets (also Unix-ish).
  No new portability debt vs the existing Makefile.

## 9. Scope boundaries (respect siblings)

- **S3 MODIFIES**: the Makefile ONLY (adds `coverage-gate` target + `COVERAGE_*` vars + `.PHONY` + help).
- **ci.yml (S1)**: already enforces §20.3 via inline awk. NOT modified by S3 (owned by S1; Complete).
  Optional future DRY: have CI's `coverage` job call `make coverage-gate` (1-line change to the
  "Enforce >=85%" step) — flagged OPTIONAL in the PRP, not required, because the inline awk already
  works and S1 owns the file. The Makefile gate's awk mirrors ci.yml's algorithm, so local already
  predicts CI regardless.
- **.goreleaser.yaml / release.yml (S2)**: untouched (S2 owns; running in parallel; no file overlap).
- **PRD.md / tasks.json / .gitignore**: READ-ONLY.

## 10. Contract-vs-design reconciliation summary

| Contract bullet | How satisfied |
|---|---|
| ≥85% on git/provider/generate/config; lower bar for ui | threshold=85 on exactly those 4; ui excluded (not in `COVERAGE_PKGS`). |
| INPUT: Makefile from P1.M1.T1.S2 | Target ADDED to that Makefile. |
| runs `go test -coverprofile=coverage.out ./internal/{git,provider,generate,config}/...` | Verbatim command. |
| parses per-package coverage, fails if any <85% | statement-weighted awk, `exit fail` (non-zero). PROVEN. |
| Use `go tool cover -func=coverage.out` output parsing | Honored in spirit (Go cover toolchain); but `-func` cannot yield per-package statement-weighted numbers (proven §5), so the GATE parses the raw profile (matching CI). `go tool cover -func` remains available via the existing `coverage` target for the function breakdown. |
| OUTPUT: a coverage gate target runnable locally and in CI | `make coverage-gate` runs anywhere make+awk+go exist (Linux/macOS/Git-Bash); CI could call it. |
