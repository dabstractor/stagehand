---
name: "P1.M4.T1.S1 (bugfix Issue 4) — Add reasoning env vars to bootstrapHeader + update test"
description: |

  Header-only consistency fix (Issue 4, minor). The generated config's `bootstrapHeader` constant
  (internal/config/bootstrap.go) documents every `STAGECOACH_*` env knob EXCEPT the reasoning ones that
  shipped with FR-R6. Add the 2 missing reasoning env-var lines so the header matches docs/cli.md and
  docs/configuration.md. No behavior change — the header IS the documentation; updating it IS the work.

  CONTRACT (item_description §3 — verbatim strings + placement): inside the `bootstrapHeader` raw string,
  insert TWO lines AFTER the per-role `STAGECOACH_ARBITER_PROVIDER / _MODEL` line and BEFORE
  `STAGECOACH_COMMITS`:
    #   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)
    #   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
  Then add a regression test asserting `buildBootstrapConfig` output contains `STAGECOACH_REASONING` and
  `STAGECOACH_<ROLE>_REASONING`.

  CRITICAL CORRECTION (item_description §1): do NOT add `STAGECOACH_MAX_COMMITS` — it is NOT an env var.
  `load.go:301` reads `max-commits` only as a CLI FLAG (`fs.Changed` + `fs.GetInt`, no `os.LookupEnv`);
  docs/cli.md shows `—` in its env column. The `--max-commits` flag is already documented in the header's
  CLI-flags section; adding an env line would be FALSE documentation. Add ONLY the 2 reasoning lines.

  DELIVERABLES (2 files; go.mod unchanged):
    1. MODIFY internal/config/bootstrap.go        — insert the 2 reasoning env-var lines in `bootstrapHeader`.
    2. MODIFY internal/config/bootstrap_test.go   — add TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars.

  SCOPE NOTE: the `<ROLE>` literal shorthand is intentional and consistent — the header's CLI-flags section
  already uses the `--<role>-provider` shorthand with "(role = planner|stager|message|arbiter)". The env-var
  block documents per-role provider/model as 4 explicit lines, but the `<ROLE>` reasoning line matches the
  CLI section's established compact form (research design-decisions §2). Use the task's 2 verbatim lines.

  SCOPE BOUNDARY (what this does NOT do): NO new env vars (the 5 are already real — load.go:181 global +
  :215 per-role loop); NO `STAGECOACH_MAX_COMMITS`; NO edits to load.go/loadEnv (reasoning already wired);
  NO behavior change (header-only); NO docs-file edits (P1.M6.T1.S1 owns README/cli.md/providers.md sync;
  the bootstrapHeader is a separate doc surface). This is the smallest possible fix for Issue 4.

  INPUT (upstream — read-only): `bootstrapHeader` const + `buildBootstrapConfig` (bootstrap.go); the
  `assertContains`/`strings.Contains` test pattern (bootstrap_test.go). Verified the 5 reasoning env vars
  are real (load.go) and that `STAGECOACH_MAX_COMMITS` is not.

  OUTPUT (downstream): the generated config header (written by `config init` / the first-run fallback)
  now documents the reasoning env vars, consistent with docs/cli.md:43-49 + docs/configuration.md:152-156.

  ⚠️ Insert the EXACT 2 verbatim lines between the ARBITER and COMMITS lines (item_description §3).
  ⚠️ Do NOT add STAGECOACH_MAX_COMMITS (item_description §1 correction — it's a flag, not an env var).
  ⚠️ Keep the lines as comments (`#`-prefixed) inside the raw string — the header must stay inert/comments.

  Deliverable: 2 modified files; `go build ./... && go test ./...` green; go.mod/go.sum unchanged.

---

## Goal

**Feature Goal**: Close the header-only docs-drift of Issue 4 — the `bootstrapHeader` constant that
prefaces every generated config (populated bootstrap via `config init` + first-run fallback) omits the
FR-R6 reasoning env vars. Add the 2 missing lines (`STAGECOACH_REASONING` global +
`STAGECOACH_<ROLE>_REASONING` per-role) so the generated header documents all `STAGECOACH_*` knobs,
matching docs/cli.md and docs/configuration.md. No behavior change.

**Deliverable** (2 files; go.mod unchanged):
1. `internal/config/bootstrap.go` — insert 2 reasoning env-var lines into the `bootstrapHeader` raw string
   (after `STAGECOACH_ARBITER_PROVIDER / _MODEL`, before `STAGECOACH_COMMITS`).
2. `internal/config/bootstrap_test.go` — `TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars`
   asserting the output contains `STAGECOACH_REASONING` and `STAGECOACH_<ROLE>_REASONING`.

**Success Definition**: `buildBootstrapConfig("pi", nil)` output contains both `STAGECOACH_REASONING` and
`STAGECOACH_<ROLE>_REASONING`; the new test FAILS without the fix and PASSES with it; the existing
bootstrap tests stay green (TOML still valid — header is all comments); `go build ./... &&
go test ./...` green; go.mod/go.sum byte-unchanged; only the 2 listed files differ.

## User Persona

**Target User**: The Stagecoach user reading a generated config to discover available env knobs (PRD §7.1
"the plan-holder"). They open `~/.config/stagecoach/config.toml` (written by `config init`) and scan the
header to see what `STAGECOACH_*` vars they can set. Today the reasoning vars are invisible there even
though they're real and documented elsewhere — a confusing inconsistency.

**Use Case**: User wants to pin reasoning effort for one run without editing the file: they look in the
generated header for the env-var name, find `STAGECOACH_REASONING` / `STAGECOACH_<ROLE>_REASONING`, and run
`STAGECOACH_PLANNER_REASONING=high stagecoach`.

**User Journey**: user runs `stagecoach config init` → the populated config (prefaced by `bootstrapHeader`)
is written → user reads the header → sees the reasoning env vars documented alongside provider/model/commits.

**Pain Points Addressed**: docs drift — a real, loadEnv-read env var (`STAGECOACH_REASONING`, FR-R6) was
invisible in the generated config's header, so users had to find docs/cli.md to discover it. Now the
header is the single self-consistent reference.

## Why

- **It IS Issue 4.** The bug list (§h3.3) names this exact gap: the header omits the reasoning env vars;
  docs/cli.md documents them; this is a header-only inconsistency. This subtask fixes it.
- **Self-consistency of the generated config.** The header is the Mode-A doc surface users actually see
  (it's written into their config file). Every other env var is listed there; reasoning should be too.
- **Trivial, safe, isolated.** 2 comment lines + 1 test. No behavior, no schema, no precedence change.
  The header is inert (all `#` comments), so TOML validity and every existing test are unaffected.

## What

Insert 2 `#`-commented env-var lines into the `bootstrapHeader` raw-string constant at one precise spot,
and add one `strings.Contains`-style regression test (mirroring `bootstrap_test.go`'s `assertContains`
helper). No new code paths, no new types, no new deps, no behavior change, no other doc files.

### Success Criteria

- [ ] `internal/config/bootstrap.go`: `bootstrapHeader` contains the 2 verbatim lines, placed AFTER
      `#   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter` and BEFORE
      `#   STAGECOACH_COMMITS   …`.
- [ ] The 2 lines are EXACTLY:
      `#   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)`
      `#   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)`
- [ ] NO `STAGECOACH_MAX_COMMITS` line is added (it's a flag, not an env var — item_description §1).
- [ ] `internal/config/bootstrap_test.go`: `TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars`
      asserts `buildBootstrapConfig("pi", nil)` output contains both `STAGECOACH_REASONING` and
      `STAGECOACH_<ROLE>_REASONING` (uses the existing `assertContains` helper).
- [ ] The new test FAILS without the header edit and PASSES with it (real regression guard).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean;
      go.mod/go.sum byte-unchanged; only `bootstrap.go` + `bootstrap_test.go` differ.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact 2 verbatim lines
+ their precise insertion point (quoted), the §1 correction (don't add max-commits), the exact test to
add (given), and the file paths. No loadEnv/registry/git knowledge required — this is a 2-line
string-literal edit + a `strings.Contains` test.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M4T1S1/research/design-decisions.md
  why: the 6 decisions. §0 (the exact 2 lines + insertion spot), §1 (DO NOT add STAGECOACH_MAX_COMMITS —
       it's a flag), §2 (the <ROLE> shorthand is consistent with the CLI-flags section), §3 (the 5 env
       vars are real — load.go:181/215), §4 (no test pins exact header content → safe), §5 (the new test),
       §6 (no conflict with parallel P1.M3.T1.S1).
  critical: §0 (verbatim lines + spot), §1 (the max-commits correction — adding it would be FALSE docs).

# MUST READ — the file to edit (the bootstrapHeader const + buildBootstrapConfig)
- file: internal/config/bootstrap.go   (EDIT bootstrapHeader; READ buildBootstrapConfig)
  section: `const bootstrapHeader = \`# Stagecoach configuration file (populated bootstrap).\n…\`` — the
       raw-string header. The env-var block lists PROVIDER/MODEL/TIMEOUT/CONFIG/VERBOSE/NO_COLOR, the 4
       per-role `_PROVIDER / _MODEL` lines, then `STAGECOACH_COMMITS`. `buildBootstrapConfig` prepends it
       via `b.WriteString(bootstrapHeader)` (so its output — and the test target — includes the header).
  why: this is THE file to edit. The insertion point is between the `STAGECOACH_ARBITER_PROVIDER / _MODEL`
       line and the `STAGECOACH_COMMITS` line.
  pattern: every env-var line is `#   STAGECOACH_<NAME>   <description>` (3-space indent after `#`).
  gotcha: the raw string uses `+` concatenation for backticks (`+ "`" + `) — stay INSIDE the raw-string
       segment for the insertion (no backtick handling needed at the insertion point). Keep lines `#`-
       prefixed (the header is inert comments).

# MUST READ — the test file to extend (the assertContains pattern)
- file: internal/config/bootstrap_test.go   (EDIT: add the test; READ the pattern)
  section: the `assertContains(t, content, substrs...)` helper (bottom of file) + e.g.
       `TestBuildBootstrapConfig_Pi` which does `strings.Contains(content, …)` over `buildBootstrapConfig`
       output. There is NO existing test that validates the header env-var block — this subtask adds the
       first one.
  why: mirror this exact style for the new test. The new test targets `buildBootstrapConfig("pi", nil)`
       output (which includes the header).
  gotcha: no existing test exact-matches the header, so inserting lines breaks nothing (§4). The
       TOML-validity test (`TestBuildBootstrapConfig_ValidTOML`) unmarshals the output — the header is all
       comments, so 2 new comment lines don't affect it.

# MUST READ — proves the 5 env vars are real + the max-commits correction
- file: internal/config/load.go   (READ ONLY — do NOT edit)
  section: `loadEnv` — `os.LookupEnv("STAGECOACH_REASONING")` (line 181 → cfg.Reasoning) + the per-role
       loop `os.LookupEnv(prefix + "_REASONING")` over roleNames={planner,stager,message,arbiter} (line
       215 → cfg.setRoleReasoning). AND `loadFlags` line 301: `fs.Changed("max-commits")` + `fs.GetInt`
       (a CLI FLAG, NOT os.LookupEnv → STAGECOACH_MAX_COMMITS does not exist).
  why: confirms the 2 reasoning lines document REAL env vars (so the header is truthful) and that
       STAGECOACH_MAX_COMMITS must NOT be added (§1).
  critical: do NOT add a max-commits env line. load.go reads it only as a flag.

# The documented wording the header must match (READ ONLY)
- file: docs/cli.md   (lines 43-49, 164-170)
  why: the canonical env-var table. `STAGECOACH_REASONING` = "Global reasoning effort: off|low|medium|high";
       the 4 per-role `STAGECOACH_<ROLE>_REASONING`. The header's 2 new lines mirror this wording.
- file: docs/configuration.md   (lines 152-156)
  why: the second canonical source documenting the same 5 reasoning env vars. Consistency target.

- url: (the bug list Issue 4 — already in context as selected_prd_content `h3.3`; also
       plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/prd_snapshot.md §h3.3)
  why: Issue 4 is the AUTHORITATIVE statement of the gap. Note its prose mentions "max-commits" but the
       item_description §1 OVERRIDES that (max-commits is a flag, not an env var) — follow item_description.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap.go         # EDIT: bootstrapHeader const (+2 reasoning env-var lines). READ buildBootstrapConfig.
  bootstrap_test.go    # EDIT: + TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars. READ assertContains pattern.
  load.go              # READ ONLY: proves the 5 reasoning env vars are real (loadEnv:181/215); max-commits is a flag (:301). DO NOT EDIT.
docs/cli.md            # READ ONLY: canonical env-var wording (lines 43-49). DO NOT EDIT (P1.M6 owns docs sync).
docs/configuration.md  # READ ONLY: second canonical source (lines 152-156). DO NOT EDIT.
go.mod / go.sum        # UNCHANGED (no new imports; test uses testing+strings already imported).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 2 MODIFIED files only:
internal/config/bootstrap.go        # bootstrapHeader: +2 reasoning env-var lines (between ARBITER_PROVIDER and COMMITS).
internal/config/bootstrap_test.go   # + TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars.
# go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (do NOT add STAGECOACH_MAX_COMMITS, design §1): Issue 4's prose says "reasoning (and max-commits)"
//   but item_description §1 corrects it — max-commits is a CLI FLAG (load.go:301 fs.Changed+fs.GetInt), NOT
//   an env var (no os.LookupEnv; docs/cli.md env column is "—"). Adding it would be FALSE documentation.
//   Add ONLY the 2 reasoning lines.

// CRITICAL (verbatim lines + exact spot, design §0): insert the EXACT 2 lines (item_description §3) AFTER
//   the `STAGECOACH_ARBITER_PROVIDER / _MODEL` line and BEFORE `STAGECOACH_COMMITS`. Don't reword them.

// GOTCHA (<ROLE> literal is intentional, design §2): the per-role reasoning line uses the literal
//   `<ROLE>` shorthand, matching the header's CLI-flags section (`--<role>-provider`, "(role =
//   planner|stager|message|arbiter)"). It is NOT a mistake even though the env-var block's provider/model
//   lines are 4 explicit per-role lines — the <ROLE> form is the established compact shorthand.

// GOTCHA (raw-string edit): bootstrapHeader is a backtick raw string with `+ "`" +` concatenations for
//   backticks. The insertion point (between ARBITER and COMMITS) is inside a plain raw-string segment —
//   no backtick/concatenation handling needed. Just add the 2 `#`-prefixed lines.

// GOTCHA (header is inert comments): every header line starts with `#`. Keep the 2 new lines `#`-prefixed
//   so the generated config stays valid TOML (the TestBuildBootstrapConfig_ValidTOML test unmarshals it).

// GOTCHA (the 2 assertions are independent): `STAGECOACH_REASONING` is NOT a substring of
//   `STAGECOACH_<ROLE>_REASONING` (the latter has `<ROLE>_` between STAGECOACH_ and REASONING). So the test
//   must assert BOTH strings — asserting only the global one would not catch a missing per-role line.

// GOTCHA (no parallel conflict, design §6): the running P1.M3.T1.S1 edits decompose.go/roles.go/generate.go/
//   default_action.go — NOT bootstrap.go/bootstrap_test.go. No overlap. This subtask touches only these 2 files.
```

## Implementation Blueprint

### Data models and structure

```go
// NO new data models. This is a 2-line string-literal edit + a strings.Contains test.
// The only "structure" is the bootstrapHeader raw string's env-var block, which gains 2 lines.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/bootstrap.go — insert the 2 reasoning env-var lines into bootstrapHeader
  - FILE: internal/config/bootstrap.go, `const bootstrapHeader`.
  - LOCATE the env-var block. Find the line:
        #   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
    and the line immediately after it:
        #   STAGECOACH_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
  - INSERT these EXACT 2 lines BETWEEN them (preserve the 3-space-after-# indent + the description column):
        #   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)
        #   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
  - DO NOT add any STAGECOACH_MAX_COMMITS line (design §1 — it is a flag, not an env var).
  - DO NOT touch any other part of bootstrap.go (buildBootstrapConfig, GenerateBootstrapConfig, the CLI-
      flags section, generationCommented — all unchanged). The `--max-commits` FLAG is already documented
      in the header's CLI-flags section; that's correct and stays.
  - GOTCHA: the lines are inside a backtick raw string — no escaping. Keep them `#`-prefixed.

Task 2: EDIT internal/config/bootstrap_test.go — add the regression test
  - FILE: internal/config/bootstrap_test.go, `package config`. ADD a new test function:
        // TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars guards Issue 4: the generated config
        // header must document the FR-R6 reasoning env vars (global + per-role), matching docs/cli.md.
        func TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars(t *testing.T) {
            content := buildBootstrapConfig("pi", nil)
            assertContains(t, content,
                "STAGECOACH_REASONING",
                "STAGECOACH_<ROLE>_REASONING",
            )
        }
  - PLACE it among the other `TestBuildBootstrapConfig_*` functions (before the Helpers section). It uses
      the existing `assertContains` helper (bottom of file) — NO new helper needed. `strings` + `testing`
      are already imported.
  - GOTCHA: assert BOTH strings (they're independent — neither is a substring of the other; design gotcha).
      The test FAILS without Task 1 (neither string is in the header today) and PASSES with it → real guard.

Task 3: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/config/bootstrap.go internal/config/bootstrap_test.go`
  - `go vet ./internal/config/`
  - `go test ./internal/config/ -run 'TestBuildBootstrapConfig' -v` → all PASS, incl. the new test.
  - `go build ./... && go test ./...` → GREEN (no regression).
  - `git diff --exit-code go.mod go.sum` → empty (no new deps).
  - `git status` shows ONLY internal/config/bootstrap.go + internal/config/bootstrap_test.go modified.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the bootstrapHeader env-var line (3-space indent, name + description):
//   #   STAGECOACH_<NAME>   <description>
// Mirror it exactly. The 2 new lines match this shape (the description column isn't perfectly aligned
// across the whole block — the existing provider/model lines are longer — so match the task's verbatim
// spacing, not a computed column).

// PATTERN: the bootstrap_test.go assertion style (strings.Contains via assertContains):
//   content := buildBootstrapConfig("pi", nil)
//   assertContains(t, content, "SUBSTR1", "SUBSTR2")
// The new test uses exactly this — no new helper, no exact-match, no frequency count needed.

// CRITICAL: the header is prepended to buildBootstrapConfig output via b.WriteString(bootstrapHeader),
//   so testing buildBootstrapConfig("pi", nil) output IS testing the header (no need to reference the
//   unexported bootstrapHeader const directly, though that would also work).

// GOTCHA: buildBootstrapConfig("pi", nil) is the cheapest target (no $PATH, deterministic). Any target
//   would do — the header is target-independent — but "pi"/nil mirrors TestBuildBootstrapConfig_NoInstallFallback.
```

### Integration Points

```yaml
HEADER.ENV_BLOCK:
  - insert: "2 lines (STAGECOACH_REASONING + STAGECOACH_<ROLE>_REASONING) between ARBITER_PROVIDER and COMMITS"
  - do-not-add: "STAGECOACH_MAX_COMMITS (flag, not env — design §1)"

TEST.SURFACE:
  - add: "TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars (strings.Contains via assertContains)"
  - target: "buildBootstrapConfig(\"pi\", nil) output (includes the header)"

GO.MODULE: change NONE. No new imports (testing + strings already in bootstrap_test.go). `go mod tidy` no-op.

FROZEN/LEAVE (do NOT edit):
  - internal/config/load.go (reasoning already wired — loadEnv:181/215; max-commits is a flag at :301).
  - docs/cli.md, docs/configuration.md, README.md, docs/providers.md (P1.M6.T1.S1 owns docs sync).
  - everything else (internal/cmd/*, internal/decompose/*, internal/generate/*, internal/provider/*, etc.).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/bootstrap.go internal/config/bootstrap_test.go
go vet ./internal/config/
# Confirm the 2 lines landed in the right spot (between ARBITER_PROVIDER and COMMITS):
grep -n -A1 'STAGECOACH_REASONING ' internal/config/bootstrap.go    # expect the REASONING line + the <ROLE>_REASONING line
grep -n 'STAGECOACH_MAX_COMMITS' internal/config/bootstrap.go       # expect NO match (must not be added)
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; the 2 reasoning lines present; NO STAGECOACH_MAX_COMMITS line.
```

### Level 2: Unit tests (the new test + no regression)

```bash
# The new test in isolation:
go test ./internal/config/ -run TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars -v
# Expected PASS. (Pre-fix it FAILS — verify by temporarily reverting Task 1 if desired.)

# All bootstrap tests:
go test ./internal/config/ -run TestBuildBootstrapConfig -v
# Expected: all PASS — incl. TestBuildBootstrapConfig_ValidTOML (the 2 new comment lines don't break TOML).

# Full config suite (no regression):
go test ./internal/config/ -v
# Expected: all PASS.
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — nothing depends on the header text except the new test.
# Confirm ONLY the 2 target files differ:
git status --porcelain
# Expected: exactly 2 modified files (internal/config/bootstrap.go, internal/config/bootstrap_test.go).
git diff --exit-code internal/config/load.go docs/cli.md docs/configuration.md go.mod go.sum \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (header-only, inert, truthful)

```bash
# No server/DB/subprocess. Verify by reasoning + Level 2:
#   1. The 2 new lines document REAL env vars — re-confirm load.go reads them:
grep -n 'STAGECOACH_REASONING"\|"_REASONING"' internal/config/load.go   # expect the global (:181) + per-role loop (:215)
#   2. The header is INERT (all '#') — the generated config stays valid TOML:
go test ./internal/config/ -run TestBuildBootstrapConfig_ValidTOML -v   # PASS
#   3. NO false max-commits env line was added (truthful docs):
! grep -q 'STAGECOACH_MAX_COMMITS' internal/config/bootstrap.go && echo "OK: no false max-commits env line"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the new test; no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] `git status` shows EXACTLY 2 modified files (bootstrap.go, bootstrap_test.go); every LEAVE file unchanged.

### Feature Validation
- [ ] `bootstrapHeader` contains `STAGECOACH_REASONING` (global) and `STAGECOACH_<ROLE>_REASONING` (per-role).
- [ ] The 2 lines are placed between `STAGECOACH_ARBITER_PROVIDER / _MODEL` and `STAGECOACH_COMMITS`.
- [ ] NO `STAGECOACH_MAX_COMMITS` line was added (it's a flag, not an env var).
- [ ] The lines are `#`-commented (header stays inert; generated config stays valid TOML).
- [ ] `TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars` PASSES (and fails without the edit).

### Code Quality Validation
- [ ] The 2 lines match the existing env-var line shape (`#   STAGECOACH_<NAME>   <desc>`).
- [ ] The `<ROLE>` shorthand matches the header's CLI-flags-section `--<role>-…` precedent (consistent).
- [ ] The new test mirrors `bootstrap_test.go`'s `assertContains`/`strings.Contains` style (no new pattern).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] The generated config header now documents all reasoning env vars, consistent with docs/cli.md:43-49
      and docs/configuration.md:152-156. (The header IS the documentation; updating it IS the work.)

---

## Anti-Patterns to Avoid

- ❌ **Don't add `STAGECOACH_MAX_COMMITS`.** Issue 4's prose mentions "max-commits" but item_description §1
  corrects it: `max-commits` is a CLI FLAG (load.go:301), NOT an env var (no `os.LookupEnv`; docs/cli.md
  env column is "—"). Adding it would be FALSE documentation. (§1)
- ❌ **Don't reword the 2 lines.** item_description §3 gives them verbatim. Use them exactly (the `<ROLE>`
  literal, the "off|low|medium|high" wording, the PRD § refs). (§0)
- ❌ **Don't expand the per-role reasoning into 4 explicit lines** (PLANNER/STAGER/MESSAGE/ARBITER). The
  task specifies the single `<ROLE>` shorthand line, which matches the header's CLI-flags-section
  `--<role>-…` precedent. (§2)
- ❌ **Don't exact-match the header in the test.** Use `strings.Contains` (the `assertContains` helper) —
  the existing `bootstrap_test.go` style. Exact-match would be brittle and isn't needed. (§5)
- ❌ **Don't assert only `STAGECOACH_REASONING`.** It's NOT a substring of `STAGECOACH_<ROLE>_REASONING`;
  assert BOTH so a missing per-role line is caught. (gotcha)
- ❌ **Don't touch load.go, docs/cli.md, docs/configuration.md, or any other file.** Reasoning is already
  wired in loadEnv; docs sync is P1.M6.T1.S1's job; this subtask is the header-only fix. (scope boundary)
- ❌ **Don't add a non-`#`-prefixed line.** Every header line is a comment; the generated config must stay
  valid TOML (`TestBuildBootstrapConfig_ValidTOML` unmarshals it). (gotcha)
