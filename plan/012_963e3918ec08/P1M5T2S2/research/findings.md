# P1.M5.T2.S2 — Research Findings: Full build + test suite with stagecoach identity

Verification of the stagehand→stagecoach rename at the **build + test + runtime-identity** layer.
All commands below were executed in the live repo (`/home/dustin/projects/stagehand`) on 2026-07-07.
**Headline: everything is already GREEN.** This is a certification task; the rename's M1–M4 passes plus
the structural work left the build/test/identity surface fully stagecoach. The PRP's job is to define the
exact certification commands + expected outputs and a scoped remediation playbook in case a future straggler
appears.

---

## §1. The 5 contract steps — VERIFIED state (all pass)

| # | Contract step | Command run | Result |
|---|---------------|-------------|--------|
| a | `make build` → `./bin/stagecoach` | `make build` | **EXIT 0**; produces `bin/stagecoach` (9.1 MB). Makefile target: `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`. |
| b | `go test ./... -count=1` | `go test ./... -count=1` | **17 ok / 0 FAIL / 2 no-test**. exit 0. |
| c | `go vet ./...` | `go vet ./...` | **EXIT 0** (clean). |
| d | `./bin/stagecoach --version` | `./bin/stagecoach --version` | `stagecoach version dev (44c299b-dirty)` exit 0. |
| e | `./bin/stagecoach --help` identity | `./bin/stagecoach --help` | `Usage:\n  stagecoach [flags]\n  stagecoach [command]`; subcommands incl. `config Manage the Stagecoach config file`; flags show `STAGECOACH_*` env vars + `stagecoach.*` git keys. exit 0. |

Additional project QA gates (Makefile-defined; not in the contract's 5 steps but part of "full test suite"):
- `make test` (race detector): `go test -race ./...` → **17 ok / 0 FAIL / 2 no-test**. exit 0.
- `make coverage-gate` (PRD §20.3, ≥85% on the 4 core packages): **PASS** — git 88.7%, provider 91.1%, generate 90.4%, config 86.2%.

---

## §2. CRITICAL GOTCHA — build-tag-gated tests are NOT in default `go test ./...`

The item description says the test suite "includes unit tests, integration tests with stub agents, and the
**e2e harness**." But the e2e harness and the real-agent suite are `//go:build`-gated and are EXCLUDED from
a bare `go test ./...`. Running ONLY step (b) satisfies the literal command but **misses the e2e harness the
item explicitly names**. The implementer MUST run them explicitly.

Build tags present (`grep -rn '//go:build' --include='*_test.go'`):

| Tag | Files | Included in `go test ./...`? | Required action |
|-----|-------|------------------------------|-----------------|
| `e2e` | `internal/e2e/{harness,hook_scenarios,lock_scenarios,scenarios}_test.go` (4) | **NO** | **Run explicitly**: `go test -tags e2e ./internal/e2e/... -count=1` |
| `integration_real` | `internal/generate/realagent_test.go` (1) | **NO** | **Compile only** (no real agents in CI); see §3 |
| `!windows` | `internal/lock/lock_unix_test.go`, `internal/signal/signal_integration_test.go` | yes (on Linux) | part of default suite |
| `windows` | `internal/provider/procgroup_windows_test.go` | skipped on Linux | n/a (cross-platform) |

**e2e harness VERIFIED:** `go test -tags e2e ./internal/e2e/... -count=1` → **ok 22.4s** exit 0.
Zero `stagehand` refs in `internal/e2e`. The harness self-builds `./cmd/stagecoach` into a temp dir
(`buildStagecoach`, `harness_test.go:44-70`), names it `stagecoach`, writes config to `stagecoach.toml`
(line 173), and drives it as a subprocess. All stagecoach.

The `internal/stubtest/` package (stub-agent integration) has **NO build tag** → it IS in the default
`go test ./...` (one of the 17 ok packages).

---

## §3. The `integration_real` suite — compile, don't run

`internal/generate/realagent_test.go` is `//go:build integration_real`. It runs ONLY under
`-tags integration_real` AND when `STAGECOACH_RUN_REAL=1`, and it invokes the real pi/claude/gemini/etc.
binaries (PRD §20.1 layer-4: "opt-in, NOT in CI"). Real agents are not assumed installed → running it will
either skip (env off) or fail (agent missing). The correct certification is **compile-only**:
`go vet -tags integration_real ./internal/generate/...` → **EXIT 0** (verified). This proves the rename left
no straggler that breaks the real-agent suite's compilation, which is all CI can guarantee.

---

## §4. Identity surface — where "stagecoach" must appear and how it's derived

- **Command name**: `rootCmd.Use = "stagecoach"` (`internal/cmd/root.go:121`). Cobra derives `--help`'s
  `Usage:\n  stagecoach [flags]` and every subcommand's parent path from this. VERIFIED in `--help`.
- **`--version` prefix**: there is NO `SetVersionTemplate` / custom template anywhere (grep-confirmed).
  Cobra's default version template is `{{.Name}} version {{.Version}}\n` → `stagecoach version <V>`.
  `.Name()` = first token of `Use` = "stagecoach". VERIFIED: `stagecoach version dev (44c299b-dirty)`.
- **Version value**: `main.go` `var version = "dev"`, enriched by `resolveVersion()` via `debug.ReadBuildInfo`
  (vcs.revision + vcs.modified) → `dev (<short-sha>-dirty)` for a dirty tree, `dev (<short-sha>)` clean, a
  real `vX.Y.Z` kept verbatim (goreleaser / `make VERSION=`). `make build` injects `-X main.version=$(VERSION)`
  (default `dev`). So `--version` correctly prints `dev` or `dev (<sha>...)` — exactly the contract's "version
  (or `dev`)".
- **Env vars**: every persistent flag's usage string names `STAGECOACH_*` (e.g. `env STAGECOACH_PROVIDER`,
  `STAGECOACH_MODEL`, `STAGECOACH_CONFIG`, `STAGECOACH_TIMEOUT`, `STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR`,
  per-role `STAGECOACH_{PLANNER,STAGER,ARBITER,MESSAGE}_{PROVIDER,MODEL,REASONING}`, `STAGECOACH_FORMAT`,
  `STAGECOACH_LOCALE`, `STAGECOACH_TEMPLATE`, `STAGECOACH_PUSH`, `STAGECOACH_NO_VERIFY`, `STAGECOACH_REASONING`).
  VERIFIED in `--help` flag block.
- **git config keys**: flag usage strings name `stagecoach.*` (e.g. `git stagecoach.provider`,
  `stagecoach.model`, `stagecoach.timeout`, `stagecoach.commits`, `stagecoach.max_commits`,
  `stagecoach.role.{planner,stager,arbiter,message}`, `stagecoach.format`, `stagecoach.locale`,
  `stagecoach.template`, `stagecoach.push`, `stagecoach.noVerify`, `stagecoach.reasoning`). VERIFIED in `--help`.
- **Prose name**: subcommand `config` Short = "Manage the Stagecoach config file" (capital S, one word).
- **Error prefix**: `main.go` prints `stagecoach: %v` on error; `root.go:134` `stagecoach: getwd:`. All stagecoach.
- **Binary path**: Makefile `BIN := bin/stagecoach`, `MAIN_PKG := ./cmd/stagecoach`. e2e harness builds
  `github.com/dustin/stagecoach/cmd/stagecoach`.

**The single authoritative assertion**: `./bin/stagecoach --help 2>&1 | grep -i stagehand` MUST return nothing.

---

## §5. Rename residue — ZERO in the compiled/test surface

- `git grep -il 'stagehand' -- '*.go'` → **0 files** (entire repo, plan/ included — plan/ has no .go).
- `git grep -li 'stagehand' -- 'internal/*/testdata/*'` and golden/embedded files → **0 files**.
- e2e + stubtest + realagent + cmd/stubagent: `grep -rni stagehand` → **0 lines**.

The ONLY remaining `stagehand` text repo-wide is the three documented exception categories (handled by the
sibling **P1.M5.T2.S1** grep-audit task): `plan/012_963e3918ec08/` (rename docs), `**/tasks.json`
(orchestrator-owned), and `PRD.md:2366` (the rename directive, read-only). None of these are compiled or
exercised by any test, so they cannot affect THIS task's build/test/identity outcomes.

---

## §6. Relationship to the sibling P1.M5.T2.S1 (grep audit) — NON-OVERLAPPING

S1 (parallel, treated as a CONTRACT) fixes exactly 3 grep-audit stragglers in the SHIPPED surface:
- `internal/git/git.go:390` — a **comment** (`.git/STAGEHAND_EDITMSG` → `STAGECOACH_EDITMSG`). Comments are
  not compiled → does not affect S2's build/vet/test.
- `.goreleaser.yaml:1` — a **comment**. Does not affect S2.
- `.golangci.yml:40` — a lint-exclusion **path** (`pkg/stagehand/...` → `pkg/stagecoach/...`). Affects S2 ONLY
  if S2 runs `make lint` (golangci-lint); it is NOT among the contract's 5 steps. (If S2 runs `make lint`, the
  S1 path-fix restores the intended errcheck/unused suppression for `stagecoach_test.go`.)

**Boundary**: S1 owns the grep audit + those 3 fixes. S2 owns build + test + runtime-identity certification.
If S2 discovers a stagehand straggler that BREAKS build/test/identity (e.g. a test asserting on a
"stagehand" string, a hardcoded `bin/stagehand` path, a fixture embedding the old name), S2 fixes THAT test
breakage (it is in-scope as "make the test suite pass"). S2 does NOT re-run S1's whole-repo grep audit or
touch S1's 3 files / plan/ / PRD.md / tasks.json.

---

## §7. Remediation playbook (IF a straggler breaks build/test — currently none needed)

The current state needs NO fixes. But a future regression (e.g. a merge reintroduces a stagehand token) could
break a test that asserts on output/paths. Locate-and-fix, scoped to the test/build surface:

1. **Compile failure / vet failure**: `go build ./... 2>&1` or `go vet ./... 2>&1` names the file:line.
   The fix is the rename of the offending identifier/path to stagecoach (M1–M4 conventions).
2. **Test failure asserting on identity**: `go test ./... -count=1 2>&1 | grep -A5 FAIL` → the failing test +
   its assertion. Read it; if it expects a "stagehand" string/path/env, update the expectation to stagecoach
   (these are test-only changes; they do not alter production behavior). Check testdata/golden files under the
   failing package too.
3. **e2e failure**: run `go test -tags e2e ./internal/e2e/... -count=1 -v 2>&1 | grep -A10 FAIL`. The harness
   self-builds `cmd/stagecoach`; a failure there is either a binary-name/path mismatch or an assertion on
   history/output that embeds the old name.
4. **`--version`/`--help` identity leak**: `./bin/stagecoach --version` / `--help` shows "stagehand" → the
   leak is in `root.go` (`Use:`, flag usage strings), `main.go` (version/error prefix), or a subcommand's
   `Use`/`Short`. Grep: `git grep -ni stagehand -- internal/cmd cmd`.

NEVER touch to "fix": `plan/012_963e3918ec08/`, `**/tasks.json`, `PRD.md` (S1's documented exceptions; they
are not compiled and not in any test path).

---

## §8. Out of scope (explicit)

- The whole-repo `git grep -i stagehand` audit → **P1.M5.T2.S1** (the sibling task; its corrected audit is
  the close-out gate for the rename's TEXT surface).
- S1's 3 fixes (git.go comment, .goreleaser.yaml comment, .golangci.yml path) → S1.
- Documentation consistency (badges, GitHub links, distribution paths) → **P1.M5.T3.S1/S2**.
- Historical plan/ artifact rename → **P1.M5.T1.S1**.
- Real-agent execution (integration_real RUN, not just compile) → manual pre-release (PRD §20.1 layer-4).
- New tests, new features, behavior changes → none. This is verification only.
