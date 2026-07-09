name: "P1.M2.T4.S1 — Gracefully reject STAGECOACH_VERBOSE=2 with a clear 'not yet supported' message (PRD Issue 4)"
description: >
  Minimal bugfix for PRD Issue 4 / §19. Today `STAGECOACH_VERBOSE=2` aborts config load with an
  OPAQUE error — `strconv.ParseBool: parsing "2": invalid syntax` — because `Config.Verbose` is a
  `bool` parsed by `strconv.ParseBool` in `internal/config/load.go` (loadEnv). PRD §19 documents
  `stagecoach_VERBOSE=2` for stdin-contents logging, so a user following the docs hits a cryptic
  exit-1. This task replaces the opaque parse error with a CLEAR, actionable message:
  `STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false`.
  It special-cases exactly `"2"` (BEFORE `ParseBool`) in loadEnv and returns `errors.New(...)`.
  `Config.Verbose` STAYS a `bool` — promoting it to `int` is the cross-cutting D9 feature and is
  explicitly OUT OF SCOPE. Valid values (`0/1/true/false`) are unchanged. Three new unit tests lock
  the behavior; two `verbose.go` comment blocks and two doc rows (configuration.md env table +
  cli.md flag table) are synced (PRD "[Mode A]").

---

## Goal

**Feature Goal**: Replace the opaque `strconv.ParseBool("2")` failure for `STAGECOACH_VERBOSE=2`
with a clear, actionable "not yet supported" error message, so a user who sets the PRD-§19-documented
`STAGECOACH_VERBOSE=2` learns it is deferred and is told exactly which values to use instead —
without changing the behavior of `0/1/true/false` or the type of `Config.Verbose`.

**Deliverable**:
1. **CODE**: a 3-line pre-check in `internal/config/load.go`'s `loadEnv` `STAGECOACH_VERBOSE`
   handler (return `errors.New(...)` when `v == "2"`, before `strconv.ParseBool`) + the `"errors"`
   import added to the stdlib import group.
2. **TEST**: 3 new test functions in `internal/config/load_test.go` (loadEnv-level rejection,
   Load-level rejection asserting the `env config` wrap, and a table-driven level-values test
   proving `1`→true / `0`→false / `2`→error).
3. **COMMENTS**: rewrite the two `VERBOSE=2` deferral comments in `internal/ui/verbose.go`
   (~lines 17-18 and ~52-53) to state VERBOSE=2 is now deliberately rejected with a clear message.
4. **DOCS** (PRD Mode A): append a "not yet implemented → clear error" note to the
   `STAGECOACH_VERBOSE` row in `docs/configuration.md` and to the `--verbose` row in `docs/cli.md`.

**Success Definition**:
- `STAGECOACH_VERBOSE=2 ./stagecoach …` (full Load path) exits 1 and prints exactly:
  `stagecoach: config: env config: STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false`
  (no `strconv.ParseBool` / `invalid syntax` anywhere in the message).
- `STAGECOACH_VERBOSE=1` / `=true` → `cfg.Verbose == true`, no error. `=0` / `=false` → `false`, no
  error. (`ParseBool` path unchanged for these and for other invalid values like `"notabool"`.)
- `Config.Verbose` is STILL `bool` (no type promotion; grep `Verbose\s+bool` in config.go is unchanged).
- `make test` green; new tests pass; `make lint` clean (no staticcheck S1021 from a format-less
  `fmt.Errorf`); `make coverage-gate` still ≥85%.
- The two `verbose.go` comments and the two doc rows reflect the clear-rejection reality.

## User Persona (if applicable)

**Target User**: A developer who reads PRD §19 ("never the stdin contents unless
`stagecoach_VERBOSE=2`") and sets `STAGECOACH_VERBOSE=2` to debug the payload contents.

**Use Case**: Diagnosing a token-limit / payload-size issue by wanting to see the actual stdin diff
the model receives — the exact use case PRD §19 says VERBOSE=2 is for.

**User Journey**: Before: `STAGECOACH_VERBOSE=2 stagecoach` → cryptic
`strconv.ParseBool: parsing "2": invalid syntax`, exit 1, no clue what to do. After: same command →
`… is not yet supported (stdin-contents logging is deferred); use 0/1/true/false` — the user
immediately knows VERBOSE=2 is deferred and drops back to `=1`.

**Pain Points Addressed**: PRD Issue 4 — "the code comments acknowledge VERBOSE=2 is deferred, but
the PRD documents it as a feature, so a user following the PRD hits an error" that is "an opaque
parse error."

## Why

- **Issue 4 (Minor)**: `cfg.Verbose` is `bool` and `loadEnv` parses `STAGECOACH_VERBOSE` with
  `strconv.ParseBool`, which rejects `"2"`. PRD §19 documents `stagecoach_VERBOSE=2` for
  stdin-contents logging, so the documented value is an error today. The cheapest, in-scope fix
  (per the issue's own "Suggested Fix") is to reject `"2"` with a CLEAR message rather than
  implement the feature. Implementing it (promote `Verbose` to `int`) is cross-cutting and out of
  scope (see D9 in `plan/001_…/P1M4T3S2/research/design-decisions.md`).
- **Consistency**: this matches the project's existing error discipline — a present-but-invalid env
  bool/timeout/commits already fails load with a wrapped, named error (see the `STAGECOACH_TIMEOUT`
  / `STAGECOACH_COMMITS` handlers adjacent in loadEnv). The fix just upgrades the *message* for the
  one value the PRD names.
- **Bounded scope**: ~3 lines of code, 3 tests, 2 comment rewrites, 2 one-row doc notes. No type
  change, no migration, no new field, no flag change.

## What

**User-visible behavior**: `STAGECOACH_VERBOSE=2` still fails config load with exit 1 (the value is
genuinely not implemented), but the message is now actionable: it names the variable, says
"not yet supported", explains what is deferred (stdin-contents logging), and lists the valid
alternatives (`0/1/true/false`). `0/1/true/false` behave exactly as before.

**Technical change** (one site in `internal/config/load.go`):
```go
// BEFORE (loadEnv, STAGECOACH_VERBOSE handler):
if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
    }
    cfg.Verbose = b // DIRECT set — can be false (escape hatch)
}

// AFTER:
if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
    if v == "2" {
        return errors.New("STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false")
    }
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
    }
    cfg.Verbose = b // DIRECT set — can be false (escape hatch)
}
```
Plus `"errors"` added to the stdlib import group (alphabetical, between `"context"` and `"fmt"`).

### Success Criteria
- [ ] `STAGECOACH_VERBOSE=2` → load error whose `.Error()` contains `"not yet supported"` and does
      NOT contain `"strconv.ParseBool"` or `"invalid syntax"`.
- [ ] Full user-facing line (exit 1) is exactly:
      `stagecoach: config: env config: STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false`
- [ ] `STAGECOACH_VERBOSE=1` → `cfg.Verbose == true`, `err == nil`. `=0` → `false`, `nil`. `=true`
      / `=false` unchanged (still pass through `ParseBool`).
- [ ] Other invalid values (e.g. `"notabool"`, `"3"`) still fail via the unchanged `ParseBool`
      path: error contains `"STAGECOACH_VERBOSE"` (existing `TestLoadEnv_BadBoolErrors` still green).
- [ ] `Config.Verbose` is unchanged: `grep -n 'Verbose\s\+bool' internal/config/config.go` returns
      the same `Verbose      bool          \`toml:"verbose"\`` line.
- [ ] No new import beyond `"errors"`; no flag/type/struct change.
- [ ] `make test` green; `make lint` clean (in particular, NO staticcheck **S1021**).
- [ ] `verbose.go`'s two VERBOSE=2 comments and both doc rows updated.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact before/after code for the single fix site, the exact import edit, the exact
user-facing output string (with the full 3-layer wrap traced), the house test idioms with
copy-paste-ready test functions, the lint constraint that forces `errors.New` over format-less
`fmt.Errorf`, the locate-by-grep rule (parallel siblings shift line numbers), and the exact two
doc rows to edit with their before/after text.

### Documentation & References

```yaml
# MUST READ — the authoritative research (every claim here is sourced in this file)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T1S1/research/findings.md
  why: "The complete research: fix-site before/after, the errors.New-vs-fmt.Errorf lint constraint
        (staticcheck S1021), the 3-layer error-wrap trace, the verbose.go comment rewrites, the doc
        rows, the test idioms, and the site-sweep proving no other file changes."
  critical: "§1 mandates errors.New + the 'errors' import (NOT fmt.Errorf — S1021 is enabled).
             §2/§3 give the exact ParseBool error and the exact final user string. §8 is the
             exhaustive file-change list."

# MUST READ — the Issue-4 root-cause note (cross-check)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_provider_verbose.md
  why: "Issue 4 section confirms Config.Verbose is bool (config.go:70), ParseBool rejects '2', and
        the verbose.go comments (now lines ~17-18, ~52-53) acknowledge VERBOSE=2 as deferred. It
        also enumerates the OUT-OF-SCOPE int-promotion scope map — do NOT do any of that."
  critical: "'Minimal fix: special-case 2 in loadEnv with a clear message' is the prescribed path.
             The int-promotion table is reference-only; touching it widens scope and must be avoided."

# MUST READ — the file being edited (the fix)
- file: internal/config/load.go
  why: "loadEnv is the env-overlay layer. The STAGECOACH_VERBOSE handler is the single fix site
        (locate with: grep -n 'STAGECOACH_VERBOSE' internal/config/load.go). Its caller at
        ~line 142-143 wraps any returned error with 'env config: %w'."
  pattern: "Every handler is `if v, ok := os.LookupEnv(\"STAGECOACH_<X>\"); ok && v != \"\" { … }`.
            Errors use `return fmt.Errorf(\"STAGECOACH_<X>: %w\", err)` — the new '2' branch is the
            ONE exception: a constant message with no underlying error → errors.New."
  gotcha: "load.go does NOT currently import 'errors' — you MUST add it to the stdlib group
           (alphabetical, between 'context' and 'fmt'). staticcheck S1021 fires on format-less
           fmt.Errorf, so do not try to 'save' the import by using fmt.Errorf without a verb."

# MUST READ — the type that MUST NOT change
- file: internal/config/config.go
  why: "Holds `Verbose bool` (~line 70). Confirm it stays bool: grep -n 'Verbose' internal/config/config.go."
  critical: "Do NOT promote Verbose to int. That is the deferred D9 feature and a cross-cutting
             change (~20 sites: ui.NewVerbose, Deps.Verbose, root.go:166 flag binding, file.go/git.go
             loaders). Out of scope for this bugfix."

# MUST READ — comments to rewrite
- file: internal/ui/verbose.go
  why: "Two comment blocks say VERBOSE=2 is 'un-parseable / out of scope'. After the fix VERBOSE=2 is
        DELIBERATELY rejected with a clear message — the comments must reflect that."
  pattern: "Locate with: grep -n 'VERBOSE=2' internal/ui/verbose.go → exactly 2 hits (type-doc
            SECURITY paragraph ~lines 17-18; VerboseRawOutput NOTE ~lines 52-53). Rewrite each."
  gotcha: "Do NOT change the Verbose struct, its methods, or the on bool field. COMMENTS ONLY here."

# MUST READ — tests to add (house idioms live here)
- file: internal/config/load_test.go
  why: "The loadEnv-level idiom is TestLoadEnv_BadBoolErrors (≈228): cfg := Defaults() → t.Setenv →
        err := loadEnv(&cfg); assert strings.Contains(err.Error(), …). The Load-level idiom is
        TestLoad_BadEnvBoolErrors (≈1147): loadEnvSetup(t) + chdir(t, repo) + t.Setenv → Load(...);
        assert BOTH 'env config' AND the leaf substring."
  critical: "The leaf substring 'not yet supported' in your new tests MUST match the errors.New(...)
             string exactly. Keep them in lockstep. Do NOT modify the existing
             TestLoadEnv_BadBoolErrors / TestLoad_BadEnvBoolErrors (they pin the 'notabool' path)."

# MUST READ — docs to update (PRD Mode A)
- file: docs/configuration.md
  why: "The '## Environment variables' table has the STAGECOACH_VERBOSE row. Append the note to its
        Description column (keep the Example cell unchanged)."
  gotcha: "Locate by content: grep -n 'STAGECOACH_VERBOSE.*--verbose' docs/configuration.md. Sibling
           P1.M1.T3.S1 edits the DIFFERENT '## Git-config keys' section in parallel → line drift only,
           no content conflict. Never hardcode the line number."

- file: docs/cli.md
  why: "Line 28: the `--verbose`, `-v` flag row. Append the same note to its Description column."

# CONFIRMING — the CLI flag (unchanged, but cited for the out-of-scope note)
- file: internal/cmd/root.go
  why: "Line 166 binds `pf.BoolVarP(&flagVerbose, \"verbose\", \"v\", false, …)` and line 151 wraps
        Load's error with `fmt.Errorf(\"config: %w\", err)`. This is WHY the final string has the
        'config: ' prefix and WHY --verbose stays a bool (int promotion would touch this — out of scope)."
  critical: "Do NOT touch root.go. It is cited only to document the wrap chain and the bool flag."

# EXTERNAL — the parsing primitive
- url: https://pkg.go.dev/strconv#ParseBool
  why: "Documents the exact accepted-value set (1,t,T,TRUE,true,True,0,f,F,FALSE,false,False) and the
        NumError any other string returns — confirms why '2' fails today and why a pre-check is the
        clean fix."
- url: https://go.dev/blog/go1.13-errors
  why: "%w wrapping conventions. Here the new leaf has NO underlying error (we are not wrapping a
        strconv error for the '2' case — we are short-circuiting before ParseBool), so errors.New is
        the correct choice, not fmt.Errorf with %w."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go          # EDIT — loadEnv STAGECOACH_VERBOSE handler + add "errors" import (THE FIX)
  load_test.go     # EDIT — +3 test funcs (loadEnv-level, Load-level, level-values table)
  config.go        # READ-ONLY — Verbose bool stays bool (DO NOT promote to int)
internal/ui/
  verbose.go       # EDIT (comments only) — rewrite 2 VERBOSE=2 deferral blocks (~17-18, ~52-53)
internal/cmd/
  root.go          # READ-ONLY — wrap chain (config: %w @151) + BoolVarP flag @166 (cited for scope)
cmd/stagecoach/
  main.go          # READ-ONLY — render site (stagecoach: %v @67)
docs/
  configuration.md # EDIT — append note to STAGECOACH_VERBOSE env row (~179)
  cli.md           # EDIT — append note to --verbose flag row (line 28)
.golangci.yml      # READ-ONLY — enables staticcheck (=> errors.New, not format-less fmt.Errorf)
Makefile           # READ-ONLY — `make test` / `make lint` / `make coverage-gate` / `make build`
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/config/load.go          # +3 lines (pre-check) + 1 import line ("errors")
internal/config/load_test.go     # +3 test funcs
internal/ui/verbose.go           # 2 comment-block rewrites (no code change)
docs/configuration.md            # 1 row's Description cell extended
docs/cli.md                      # 1 row's Description cell extended
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (lint forces errors.New): .golangci.yml enables `staticcheck`. staticcheck S1021
// ("call to fmt.Errorf has no formatting directives") fires on ANY fmt.Errorf with no `%` verb.
// The new "not yet supported" message is a CONSTANT with no underlying error, so it MUST be
// errors.New(...). Using fmt.Errorf("...") to avoid the import will FAIL `make lint`.

// CRITICAL (new import): load.go currently imports context/fmt/os/strconv/strings/time + pflag.
// It does NOT import "errors". You MUST add "errors" to the stdlib group, alphabetically between
// "context" and "fmt". gofumpt/gosimple-clean.

// CRITICAL (do NOT promote Verbose to int): Config.Verbose is bool and STAYS bool. Every other
// loader (Defaults/file/git/flag) and consumer (ui.NewVerbose, Deps.Verbose) treats it as bool.
// Promoting to int is the deferred D9 feature (~20 sites) and is OUT OF SCOPE.

// CRITICAL (special-case ONLY "2"): the pre-check is `v == "2"` — the single value PRD §19 names.
// Do NOT try to generalize to "any number > 1" or "any non-bool"; other invalid values (e.g. "3",
// "notabool") keep the existing STAGECOACH_VERBOSE: %w ParseBool path (and its tests) unchanged.

// GOTCHA (message ↔ test lockstep): the literal substring "not yet supported" appears in the
// errors.New(...) string AND in the test assertions. If you reword the message, reword the asserts
// in lockstep, or the new tests fail.

// GOTCHA (locate by content, not line number): siblings edit docs/configuration.md in parallel
// (P1.M1.T3.S1 = git-config section; P1.M1.T2.S1 env rows already landed). Use
// `grep -n 'STAGECOACH_VERBOSE.*--verbose' docs/configuration.md` to find the row; do not hardcode 179.

// GOTCHA (t.Setenv in a loop): if you write a table-driven test that calls t.Setenv for several
// values, wrap each case in t.Run(name, func(t *testing.T){ t.Setenv(...) }) and call t.Setenv on
// the SUBTEST's t — this keeps env isolation clean and avoids cross-case bleed.
```

## Implementation Blueprint

### Data models and structure
None. No types, no struct fields, no new symbols. `Config.Verbose` stays `bool`. The only new
identifier is the literal error string.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/load.go — add the "errors" import
  - FIND the import block (top of file, lines 3-12). The stdlib group is:
        "context"
        "fmt"
        "os"
        "strconv"
        "strings"
        "time"
    followed by a blank line and the "github.com/spf13/pflag" group.
  - ADD "errors" between "context" and "fmt" (alphabetical):
        "context"
        "errors"
        "fmt"
        ...
  - VERIFY: `go build ./internal/config/` compiles (unused import would error; here it IS used in Task 2).

Task 2: EDIT internal/config/load.go — the STAGECOACH_VERBOSE pre-check (THE FIX)
  - LOCATE: `grep -n 'STAGECOACH_VERBOSE' internal/config/load.go` → the handler block:
        if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
            b, err := strconv.ParseBool(v)
            if err != nil {
                return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
            }
            cfg.Verbose = b // DIRECT set — can be false (escape hatch)
        }
  - INSERT, immediately inside the `if v, ok := …` block and BEFORE the `b, err := strconv.ParseBool(v)`
    line:
        if v == "2" {
            return errors.New("STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false")
        }
  - PRESERVE: the ParseBool block and its `STAGECOACH_VERBOSE: %w` error (unchanged — still covers
    "notabool", "3", etc.), the `cfg.Verbose = b` DIRECT-set comment, and every OTHER env handler.
  - RESULT: handler reads
        if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
            if v == "2" {
                return errors.New("STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false")
            }
            b, err := strconv.ParseBool(v)
            if err != nil {
                return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
            }
            cfg.Verbose = b // DIRECT set — can be false (escape hatch)
        }

Task 3: EDIT internal/ui/verbose.go — rewrite the two VERBOSE=2 comments (COMMENTS ONLY)
  - LOCATE: `grep -n 'VERBOSE=2' internal/ui/verbose.go` → exactly 2 hits.
  - BLOCK A (type-doc SECURITY paragraph, ~lines 17-18) — current:
        // carries *_API_KEY credentials). Stdin contents are NOT logged at VERBOSE=1 (deferred to a future
        // VERBOSE=2 — see D9; Config.Verbose is a bool, so VERBOSE=2 is currently un-parseable and out of scope).
    REPLACE the two tail sentences with:
        // carries *_API_KEY credentials). Stdin contents are NOT logged at VERBOSE=1. Logging them at a
        // future VERBOSE=2 is not yet implemented: setting STAGECOACH_VERBOSE=2 now fails config load with
        // a clear "not yet supported" error (see D9 and config.loadEnv) rather than being silently ignored.
  - BLOCK B (VerboseRawOutput NOTE, ~lines 52-53) — current:
        // NOTE: stdin contents are NOT logged at VERBOSE=1 (deferred to a future VERBOSE=2 — would require
        // Config.Verbose to become an int; currently it is a bool and ParseBool("2") errors).
    REPLACE with:
        // NOTE: stdin contents are NOT logged at VERBOSE=1. VERBOSE=2 (stdin-contents logging) is not yet
        // implemented: STAGECOACH_VERBOSE=2 fails config load with a clear "not yet supported" error rather
        // than an opaque parse error (see D9). Implementing it would require Config.Verbose to become an int.
  - PRESERVE: the Verbose struct, the `on bool` field, NewVerbose, and every method body. COMMENTS ONLY.

Task 4: EDIT docs/configuration.md — append the note to the STAGECOACH_VERBOSE env row (PRD Mode A)
  - LOCATE BY CONTENT: `grep -n 'STAGECOACH_VERBOSE.*--verbose' docs/configuration.md` → the row:
        | `STAGECOACH_VERBOSE` | `--verbose` | Print resolved command and output | `STAGECOACH_VERBOSE=true stagecoach` |
  - REPLACE WITH (only the Description cell gains a parenthetical; Example cell unchanged):
        | `STAGECOACH_VERBOSE` | `--verbose` | Print resolved command and output (VERBOSE=2 stdin-contents logging is not yet implemented and produces a clear error if set) | `STAGECOACH_VERBOSE=true stagecoach` |
  - PRESERVE: the 4-column pipe layout, backtick wrapping, and every OTHER env row. Do not touch the
    git-config section (sibling P1.M1.T3.S1 owns it).

Task 5: EDIT docs/cli.md — append the note to the --verbose flag row (PRD Mode A)
  - LOCATE: line 28:
        | `--verbose`, `-v` | bool | false | `STAGECOACH_VERBOSE` | — | Print resolved command, raw output, retries |
  - REPLACE WITH (only the Description cell gains a parenthetical):
        | `--verbose`, `-v` | bool | false | `STAGECOACH_VERBOSE` | — | Print resolved command, raw output, retries (VERBOSE=2 is not yet supported and produces a clear error) |
  - PRESERVE: the 6-column layout and every other flag row.

Task 6: ADD internal/config/load_test.go — 3 new test functions (follow the house idiom)
  - PLACE: near the existing verbose tests (after TestLoadEnv_BadBoolErrors, ~line 241, and
    alongside TestLoad_BadEnvBoolErrors, ~line 1147). Keep the file's section-header comment style.
  - TEST A — loadEnv-level (mirrors TestLoadEnv_BadBoolErrors):
        func TestLoadEnv_Verbose2Rejected(t *testing.T) {
            cfg := Defaults()
            t.Setenv("STAGECOACH_VERBOSE", "2")
            err := loadEnv(&cfg)
            if err == nil {
                t.Fatal("loadEnv err=nil, want error for STAGECOACH_VERBOSE=2")
            }
            if !strings.Contains(err.Error(), "not yet supported") {
                t.Errorf("err=%v, want it to contain 'not yet supported'", err)
            }
            // The clear message REPLACES the opaque parse error:
            if strings.Contains(err.Error(), "strconv.ParseBool") || strings.Contains(err.Error(), "invalid syntax") {
                t.Errorf("err=%v, must NOT contain the raw strconv parse error", err)
            }
        }
  - TEST B — Load-level (mirrors TestLoad_BadEnvBoolErrors; asserts the env-config wrap):
        func TestLoad_Verbose2Rejected(t *testing.T) {
            _, repo, _ := loadEnvSetup(t)
            chdir(t, repo)
            t.Setenv("STAGECOACH_VERBOSE", "2")
            _, err := Load(context.Background(), LoadOpts{RepoDir: repo})
            if err == nil {
                t.Fatal("Load err=nil, want error for STAGECOACH_VERBOSE=2")
            }
            if !strings.Contains(err.Error(), "env config") {
                t.Errorf("err=%v, want it to contain 'env config'", err)
            }
            if !strings.Contains(err.Error(), "not yet supported") {
                t.Errorf("err=%v, want it to contain 'not yet supported'", err)
            }
        }
  - TEST C — table-driven level-values (locks 1→true / 0→false / 2→error; t.Run for env isolation):
        func TestLoadEnv_VerboseLevelValues(t *testing.T) {
            cases := []struct {
                name        string
                val         string
                wantErr     bool
                wantVerbose bool
            }{
                {"two", "2", true, false},
                {"one", "1", false, true},
                {"zero", "0", false, false},
            }
            for _, c := range cases {
                t.Run(c.name, func(t *testing.T) {
                    cfg := Defaults()
                    t.Setenv("STAGECOACH_VERBOSE", c.val)
                    err := loadEnv(&cfg)
                    if c.wantErr {
                        if err == nil {
                            t.Fatalf("val=%q: want error, got nil", c.val)
                        }
                        if !strings.Contains(err.Error(), "not yet supported") {
                            t.Errorf("val=%q: err=%v, want it to contain 'not yet supported'", c.val, err)
                        }
                        return
                    }
                    if err != nil {
                        t.Fatalf("val=%q: want nil, got %v", c.val, err)
                    }
                    if cfg.Verbose != c.wantVerbose {
                        t.Errorf("val=%q: Verbose=%v want %v", c.val, cfg.Verbose, c.wantVerbose)
                    }
                })
            }
        }
  - NAMING: test_{behavior} / TestLoad{Env,}_… matching the file's existing style.
  - COVERAGE: positive (=1/=0), the documented-but-deferred value (=2), and (via Test A/B) the
    error-replacement assertion. Do NOT modify existing tests.

Task 7: VERIFY — build, focused tests, full suite, lint, coverage gate, scope guards
  - go build ./...                                  # compiles
  - go test -race ./internal/config/ -run 'Verbose' -v   # the 3 new tests + existing verbose tests pass
  - make test                                       # full race suite green
  - make lint                                       # clean (no staticcheck S1021)
  - make coverage-gate                              # still ≥85% on internal/config
  - (scope guards: see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the loadEnv error-discipline hierarchy (every handler follows this). The "2" branch is
// the ONE deliberate exception — a constant message with no wrapped underlying error:
if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
    if v == "2" {                                       // <-- NEW: short-circuit the documented value
        return errors.New(                              //     constant message (no %w: nothing to wrap)
            "STAGECOACH_VERBOSE=2 is not yet supported " +
            "(stdin-contents logging is deferred); use 0/1/true/false")
    }
    b, err := strconv.ParseBool(v)                      // unchanged: handles 0/1/true/false AND
    if err != nil {                                     //   other invalid values ("notabool", "3", …)
        return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
    }
    cfg.Verbose = b
}

// PATTERN: the 3-layer user-facing wrap (unchanged; only the leaf changes):
//   loadEnv    → errors.New("STAGECOACH_VERBOSE=2 is not yet supported …")
//   Load       → fmt.Errorf("env config: %w", err)            // load.go:~143
//   root.go    → fmt.Errorf("config: %w", err)               // root.go:151
//   main.go    → fmt.Fprintf(stderr, "stagecoach: %v\n", err) // main.go:67
// Final (exit 1):
//   stagecoach: config: env config: STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false
```

### Integration Points

```yaml
NO database / migration / routes / new endpoint / new flag. ONE 3-line code guard + import + 3 tests
+ 2 comment rewrites + 2 one-cell doc edits.

CONFIG (internal/config/load.go):
  - import: add "errors" to the stdlib group (alphabetical: context, errors, fmt, …).
  - loadEnv: new `if v == "2" { return errors.New(...) }` branch in the STAGECOACH_VERBOSE handler.

TYPE (internal/config/config.go): NO CHANGE — `Verbose bool` stays bool (out-of-scope to promote).

CLI (internal/cmd/root.go): NO CHANGE — `pf.BoolVarP(... "verbose" ...)` stays a bool flag; the
  `config: %w` wrap at line 151 is unchanged.

DOCS:
  - docs/configuration.md: STAGECOACH_VERBOSE env row Description cell += "(VERBOSE=2 … not yet
    implemented … produces a clear error if set)".
  - docs/cli.md: --verbose flag row Description cell += "(VERBOSE=2 is not yet supported … clear error)".

COMMENTS (internal/ui/verbose.go): 2 deferral-comment rewrites (~17-18, ~52-53). No code change.

COORDINATION (parallel sibling P1.M1.T3.S1): edits in DIFFERENT sections of docs/configuration.md
  (env-var table vs git-config section) → no content conflict, only line drift; the content-anchored
  grep in Task 4 absorbs it. load.go / verbose.go / cli.md / load_test.go are touched by no other
  in-flight task (plan_status: P1.M2.T5/T6/T7 are different files, Planned/not-started).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Compile the changed package (catches the new import / typos immediately).
go build ./internal/config/

# Format check (gofmt/gofumpt). The added import must sit alphabetically in the stdlib group.
gofmt -l internal/config/load.go internal/config/load_test.go internal/ui/verbose.go
# Expected: empty (all already-formatted). If a file is listed, run `gofmt -w` on it.

# Lint — CRITICAL: this task MUST use errors.New, not format-less fmt.Errorf.
make lint      # = golangci-lint run  (CI pins v1.61; v2 rejects the .golangci.yml v1 schema)
# Expected: zero errors. If staticcheck S1021 ("call to fmt.Errorf has no formatting directives")
# fires on load.go, you used fmt.Errorf for the constant message — switch to errors.New (Task 1/2).

# Scope guard: only the 5 intended files changed.
git diff --name-only
# Expected: internal/config/load.go  internal/config/load_test.go  internal/ui/verbose.go
#            docs/configuration.md  docs/cli.md   (exactly these 5, in some order)
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 3 new tests + all existing verbose/ParseBool tests.
go test -race ./internal/config/ -run 'Verbose' -v
# Expected: PASS — TestLoadEnv_Verbose2Rejected, TestLoad_Verbose2Rejected,
#           TestLoadEnv_VerboseLevelValues (subtests two/one/zero), PLUS the pre-existing
#           TestLoadEnv_BadBoolErrors / TestLoad_BadEnvBoolErrors / TestLoadEnv_VerboseTrue/False
#           still green (the "notabool"/"3" path is untouched).

# Full race suite (proves no regression elsewhere).
make test
# Expected: green (race detector).

# Coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make coverage-gate
# Expected: passes (the new branch + tests add coverage, not remove it).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary and reproduce the EXACT user-facing message end-to-end.
make build          # → ./bin/stagecoach

# Setup a scratch repo with one staged change so Load reaches the verbose handler.
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t.com && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' >> f.txt && git add f.txt

# CASE 1 — the fix: STAGECOACH_VERBOSE=2 → clear message, exit 1, NO strconv noise.
SC=/home/dustin/projects/stagecoach/bin/stagecoach   # path to the binary from `make build`
out=$(STAGECOACH_VERBOSE=2 "$SC" 2>&1); code=$?
echo "$out" | grep -q 'STAGECOACH_VERBOSE=2 is not yet supported' && echo "OK: clear message"
echo "$out" | grep -q 'use 0/1/true/false' && echo "OK: actionable hint"
echo "$out" | grep -q 'stdin-contents logging is deferred' && echo "OK: explains the deferral"
! echo "$out" | grep -q 'strconv.ParseBool' && echo "OK: no opaque parse error"
[ "$code" -ne 0 ] && echo "OK: non-zero exit ($code)"
# Expected: all five "OK:" lines print; exit code is 1.

# CASE 2 — regression: STAGECOACH_VERBOSE=1 must still work (no error; just runs verbose).
STAGECOACH_VERBOSE=1 "$SC" 2>&1 | grep -q 'DEBUG:' && echo "OK: =1 still verbose-enabled"
# Expected: prints DEBUG: lines (verbose mode on) and succeeds.

cd - && rm -rf "$d"
```

> Note on the reproduction: the scratch repo must have a *staged* change so config Load (and thus
> loadEnv) is actually exercised. `--help` short-circuits BEFORE Load and will NOT reproduce the
> error — use a real (or `--dry-run`) invocation.

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: Config.Verbose is STILL bool (the out-of-scope int promotion must NOT have happened).
grep -n 'Verbose\s\+bool' internal/config/config.go
# Expected: the unchanged `Verbose      bool          \`toml:"verbose"\`` line. (No `int`.)

# Scope guard 2: NO format-less fmt.Errorf slipped in (would trip staticcheck S1021).
grep -n 'fmt.Errorf(' internal/config/load.go | grep -vi '%'
# Expected: empty (every fmt.Errorf in load.go has a % verb). The new branch uses errors.New.

# Scope guard 3: the "errors" import is present and used.
grep -n '"errors"' internal/config/load.go && go build ./internal/config/
# Expected: the import line prints AND the build succeeds.

# Scope guard 4: verbose.go changed COMMENTS ONLY (no struct/method/field change).
git diff internal/ui/verbose.go | grep -E '^[+-]' | grep -vE '^[+-]\s*//|^[+-]{3}'
# Expected: empty (every changed line is a // comment). If anything else appears, you edited code.

# Scope guard 5: the opaque parse path is preserved for other invalid values.
go test -race ./internal/config/ -run 'TestLoadEnv_BadBoolErrors' -v
# Expected: PASS (env "notabool" still errors with "STAGECOACH_VERBOSE" via the unchanged ParseBool path).

# Scope guard 6: docs edits are one-cell appends (table structure intact).
grep -n 'STAGECOACH_VERBOSE.*not yet' docs/configuration.md   # the appended note
grep -n 'verbose.*not yet supported' docs/cli.md              # the appended note
# Expected: exactly one hit each, inside the existing rows (no new/removed columns).

# Scope guard 7: message ↔ test lockstep — the literal substring is present in both.
grep -rn 'not yet supported' internal/config/load.go internal/config/load_test.go
# Expected: the errors.New string in load.go AND the assertions in load_test.go.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (the `"errors"` import is used; no unused-import error)
- [ ] `gofmt -l` lists none of the edited .go files
- [ ] `make test` green (race detector); the 3 new tests pass
- [ ] `make lint` clean — in particular **NO staticcheck S1021** (confirms `errors.New` was used)
- [ ] `make coverage-gate` still ≥85% on `internal/config`

### Feature Validation
- [ ] `STAGECOACH_VERBOSE=2` (full Load) → exit 1, line is exactly
      `stagecoach: config: env config: STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false`
- [ ] That line contains NONE of `strconv.ParseBool` / `invalid syntax`
- [ ] `STAGECOACH_VERBOSE=1` → `cfg.Verbose == true`, no error; `=0` → `false`, no error (Level 2/3)
- [ ] `STAGECOACH_VERBOSE=notabool` still errors with `"STAGECOACH_VERBOSE"` (unchanged ParseBool path)
- [ ] All success criteria in the "What" section met

### Scope-Boundary Validation
- [ ] `git diff --name-only` == exactly {load.go, load_test.go, verbose.go, configuration.md, cli.md}
- [ ] `Config.Verbose` is still `bool` (grep guard; no int promotion)
- [ ] `internal/cmd/root.go` untouched (the `config: %w` wrap and the `BoolVarP` flag are unchanged)
- [ ] `internal/ui/verbose.go` diff is COMMENTS ONLY (Level 4 scope guard 4)
- [ ] No new flag, struct field, type, or migration introduced
- [ ] No generalization beyond `v == "2"` (other invalid values keep the ParseBool path)

### Documentation & Code Quality
- [ ] Both doc rows gain the "not yet supported / clear error" note (Description cell only)
- [ ] Both `verbose.go` comment blocks rewritten to reflect deliberate clear rejection (not "un-parseable")
- [ ] New tests follow the house idiom (Defaults()/loadEnvSetup+chdir/t.Setenv/strings.Contains)
- [ ] The literal `"not yet supported"` is identical in the `errors.New` string and the test asserts
- [ ] Comments/docs are accurate and do not promise VERBOSE=2 works

---

## Anti-Patterns to Avoid

- ❌ Don't use format-less `fmt.Errorf("…")` for the new message to "save" the import — staticcheck
  **S1021** is enabled (`.golangci.yml`) and `make lint` will fail. Use `errors.New(...)` and add
  the `"errors"` import.
- ❌ Don't promote `Config.Verbose` from `bool` to `int`, add a `VerboseLevel`, or touch the
  `pf.BoolVarP` flag binding. That is the deferred D9 feature (~20 sites: `ui.NewVerbose`,
  `Deps.Verbose`, `root.go:166`, `file.go`/`git.go` loaders, all `*Verbose` consumers). It is a
  cross-cutting feature change, NOT this bugfix.
- ❌ Don't generalize the pre-check past `v == "2"` (no "any number > 1", no regex). PRD §19 names
  exactly `2`; other invalid values (`"3"`, `"notabool"`) must keep the existing `STAGECOACH_VERBOSE:
  %w` ParseBool path and its tests.
- ❌ Don't reword the error message without also rewording the test assertions. The substring
  `"not yet supported"` is asserted in `loadEnv`-level, `Load`-level, and table tests — keep them
  in lockstep.
- ❌ Don't locate the docs edit by hardcoded line number (179). Sibling P1.M1.T3.S1 edits the same
  file's git-config section in parallel and line numbers drift. Use
  `grep -n 'STAGECOACH_VERBOSE.*--verbose' docs/configuration.md`.
- ❌ Don't change `internal/cmd/root.go`, `cmd/stagecoach/main.go`, or the `exitcode` package. They
  are cited only to document the `config:` / `env config:` / `stagecoach:` wrap chain — the fix is
  entirely within `load.go`.
- ❌ Don't edit the `Verbose` struct, its methods, or the `on bool` field in `verbose.go`. That file
  gets COMMENT rewrites ONLY — verify with the Level 4 scope guard (`git diff` shows only `//` lines).
- ❌ Don't modify the existing `TestLoadEnv_BadBoolErrors` / `TestLoad_BadEnvBoolErrors`. They pin the
  `"notabool"` opaque-parse path, which must stay intact. ADD new tests alongside them.
- ❌ Don't try to reproduce the error with `--help` — it short-circuits before config Load. Use a real
  run (scratch repo with a staged change) or `--dry-run`.

---

## Confidence Score: 9/10

This is a 3-line code guard at a single, exactly-located site, with the before/after code spelled
out, the forced `errors.New`/import constraint (staticcheck S1021) flagged, the full 3-layer
error-wrap trace giving the exact final user-facing string, copy-paste-ready tests in the house
idiom, and an exhaustive site sweep proving no other file changes. The one residual (not a full 10)
is parallel-execution line drift in `docs/configuration.md`, which the content-anchored grep and the
different-section split fully absorb — so the deduction is conservative. No type change, no
migration, no flag change; `Config.Verbose` stays `bool` (the out-of-scope int promotion is fenced
off explicitly). One-pass implementation is highly likely.
