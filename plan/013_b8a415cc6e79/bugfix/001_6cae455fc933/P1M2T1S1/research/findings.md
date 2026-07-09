# Research: P1.M2.T4.S1 — Gracefully reject STAGECOACH_VERBOSE=2 (PRD Issue 4)

Task IDs: plan tree = **P1.M2.T4.S1**; output path dir = **P1M2T1S1** (orchestrator dir-name). Same item.
Scope: replace the opaque `strconv.ParseBool("2")` error with a clear, actionable "not yet
supported" message. **Minimal bugfix** — `Config.Verbose` STAYS a `bool`; do NOT promote to `int`
(that is the cross-cutting D9 feature change, explicitly out of scope).

All claims verified against current source. Line numbers are advisory (siblings edit the same
files in parallel); locate edits by content via grep.

---

## 1. The fix site — `internal/config/load.go`, `loadEnv`, STAGECOACH_VERBOSE handler

Current code (the handler, ~lines 246-251):
```go
if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
    }
    cfg.Verbose = b // DIRECT set — can be false (escape hatch)
}
```

**Fix** — add a pre-check for `"2"` BEFORE `strconv.ParseBool`, returning a clear error:
```go
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

### Why `errors.New` and NOT format-less `fmt.Errorf`
`.golangci.yml` enables **staticcheck**. staticcheck **S1021** ("call to fmt.Errorf has no
formatting directives" → use `errors.New`) fires on any `fmt.Errorf` with no `%` verb. Every error
in load.go today uses `fmt.Errorf("…: %w", err)`, so there is no existing S1021 baseline. To stay
lint-clean, the new constant-message error MUST be `errors.New(...)`.

### Import consequence
load.go's import block (lines 3-12):
```go
import (
    "context"
    "fmt"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/spf13/pflag"
)
```
**Add `"errors"`** in the stdlib group, alphabetically between `"context"` and `"fmt"`:
```go
    "context"
    "errors"
    "fmt"
```
(gofumpt/gosimple-clean; alphabetical stdlib group preserved.)

---

## 2. `strconv.ParseBool` behavior (confirmed by `go run`)

Accepted strings: `1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False`.
- `ParseBool("2")` → error `strconv.ParseBool: parsing "2": invalid syntax`
- `ParseBool("notabool")` → error `strconv.ParseBool: parsing "notabool": invalid syntax`
- `ParseBool("1")` / `("true")` → true, nil  •  `("0")` / `("false")` → false, nil

The new pre-check special-cases ONLY `"2"` (the value PRD §19 documents for stdin-contents
logging). All other invalid values (e.g. `"3"`, `"notabool"`) keep the existing
`STAGECOACH_VERBOSE: %w` ParseBool path unchanged and still tested by `TestLoadEnv_BadBoolErrors`.

---

## 3. Error-wrapping chain → final user-facing string (exit 1)

| hop | file:line | code |
|----|-----------|------|
| leaf | `load.go:246-251` | (after fix) `errors.New("STAGECOACH_VERBOSE=2 is not yet supported …")` |
| Load wrap | `load.go:142-143` | `return nil, fmt.Errorf("env config: %w", err)` |
| CLI wrap | `internal/cmd/root.go:151` | `return exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err))` |
| render | `cmd/stagecoach/main.go:67` | `fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)` |

`exitcode.(*ExitError).Error()` returns `e.Err.Error()` — preserves the full wrapped chain, so `%v`
renders every layer. **Final string** (exit 1):
```
stagecoach: config: env config: STAGECOACH_VERBOSE=2 is not yet supported (stdin-contents logging is deferred); use 0/1/true/false
```
(Before the fix, the same path produced the opaque
`stagecoach: config: env config: STAGECOACH_VERBOSE: strconv.ParseBool: parsing "2": invalid syntax`.
Only the leaf changes; the three outer prefixes are untouched.)

---

## 4. `Config.Verbose` field — DO NOT CHANGE

`internal/config/config.go` ~line 70:
```go
Verbose      bool          `toml:"verbose"`        // print resolved cmd, raw output, retries
```
The minimal fix keeps `Verbose bool`. Promoting to `int` would widen scope across ~20 call sites
(`ui.NewVerbose(w, on bool)`, `Deps.Verbose`, the `pf.BoolVarP` flag binding at
`internal/cmd/root.go:166`, file.go/git.go loaders, etc.) — that is the deferred D9 feature, NOT
this bugfix. Out of scope.

---

## 5. `internal/ui/verbose.go` — two comment blocks to update

Both currently say VERBOSE=2 is "un-parseable / out of scope". After the fix VERBOSE=2 is
*deliberately rejected with a clear message*, so the comments must say so.

- **Block A — type doc comment, SECURITY paragraph (lines ~17-18):**
  > `Stdin contents are NOT logged at VERBOSE=1 (deferred to a future VERBOSE=2 — see D9;
  > Config.Verbose is a bool, so VERBOSE=2 is currently un-parseable and out of scope).`
  → rewrite to: stdin-contents logging at a future VERBOSE=2 is **not yet implemented**; setting
  `STAGECOACH_VERBOSE=2` now fails config load with a clear "not yet supported" error (see D9 /
  `config.loadEnv`).

- **Block B — `VerboseRawOutput` doc comment (lines ~52-53):**
  > `NOTE: stdin contents are NOT logged at VERBOSE=1 (deferred to a future VERBOSE=2 — would
  > require Config.Verbose to become an int; currently it is a bool and ParseBool("2") errors).`
  → rewrite to: VERBOSE=2 (stdin-contents logging) is not yet implemented; setting it fails config
  load with a clear "not yet supported" error rather than an opaque parse error (see D9).
  Implementing it would require `Config.Verbose` to become an int.

Locate by grep (`grep -n 'VERBOSE=2' internal/ui/verbose.go`) — do NOT hardcode line numbers.

---

## 6. Documentation updates (PRD "[Mode A]" — rides with the work)

### `docs/configuration.md` — STAGECOACH_VERBOSE env-var table row (~line 179)
Current:
```
| `STAGECOACH_VERBOSE` | `--verbose` | Print resolved command and output | `STAGECOACH_VERBOSE=true stagecoach` |
```
Append to the Description column (keep the example cell unchanged):
```
| `STAGECOACH_VERBOSE` | `--verbose` | Print resolved command and output (VERBOSE=2 stdin-contents logging is not yet implemented and produces a clear error if set) | `STAGECOACH_VERBOSE=true stagecoach` |
```
**Locate by content** (`grep -n 'STAGECOACH_VERBOSE.*--verbose' docs/configuration.md`): this row
is in the "## Environment variables" table, a DIFFERENT section from the git-config edits that
sibling P1.M1.T3.S1 makes, so there is no content conflict — only possible line drift.

### `docs/cli.md` — `--verbose` flag row (line 28)
Current:
```
| `--verbose`, `-v` | bool | false | `STAGECOACH_VERBOSE` | — | Print resolved command, raw output, retries |
```
Append to the Description column:
```
| `--verbose`, `-v` | bool | false | `STAGECOACH_VERBOSE` | — | Print resolved command, raw output, retries (VERBOSE=2 is not yet supported and produces a clear error) |
```

No other doc file needs a change (confirmed by site sweep below). README.md / how-it-works.md /
providers.md only reference `--verbose` as a flag, never a *level*.

---

## 7. Test patterns — `internal/config/load_test.go` (house idioms)

Existing tests (do NOT modify; ADD alongside):
- `TestLoadEnv_BadBoolErrors` (≈line 228) — loadEnv-level, env `"notabool"` → err contains
  `"STAGECOACH_VERBOSE"`. Idiom: `cfg := Defaults()` → `t.Setenv(...)` → `err := loadEnv(&cfg)`.
- `TestLoad_BadEnvBoolErrors` (≈line 1147) — Load-level, asserts both `"env config"` and
  `"STAGECOACH_VERBOSE"`. Idiom: `_, repo, _ := loadEnvSetup(t)` → `chdir(t, repo)` → `t.Setenv`
  → `_, err := Load(context.Background(), LoadOpts{RepoDir: repo})`.
- `TestLoadEnv_VerboseTrue/False` (≈149/190) cover `"true"`/`"false"`, NOT `"1"`/`"0"`.

New tests to add (copy-paste-ready, see PRP §Implementation Tasks):
- `TestLoadEnv_Verbose2Rejected` — loadEnv-level, env `"2"` → err contains `"not yet supported"`.
- `TestLoad_Verbose2Rejected` — Load-level, asserts `"env config"` AND `"not yet supported"`.
- `TestLoadEnv_VerboseLevelValues` (table-driven, t.Run subtests) — locks the contract that the
  pre-check does NOT regress valid values: `"2"`→err "not yet supported"; `"1"`→Verbose=true,nil;
  `"0"`→Verbose=false,nil. (t.Run + t.Setenv-on-subtest keeps env isolation clean.)

The literal substring `"not yet supported"` MUST stay in lockstep with the `errors.New(...)` leaf
string — that string drives every assertion.

---

## 8. Comprehensive site sweep (scout-confirmed) — files needing a change

| file | change | type |
|------|--------|------|
| `internal/config/load.go` | pre-check `v == "2"` + `errors.New` + add `"errors"` import | CODE (the fix) |
| `internal/config/load_test.go` | +3 new test funcs (loadEnv-level, Load-level, level-values) | TEST (additive) |
| `internal/ui/verbose.go` | rewrite 2 comment blocks (lines ~17-18, ~52-53) | COMMENT |
| `docs/configuration.md` | append note to STAGECOACH_VERBOSE env row (~179) | DOC |
| `docs/cli.md` | append note to `--verbose` flag row (line 28) | DOC |

**No other file needs a change.** Confirmed NO matches for verbose-*level* / VERBOSE=2 /
stdin-contents logging in: `README.md` (only `--verbose` flag usage @362),
`docs/providers.md` (no verbose), `docs/how-it-works.md` (only flag usage @292), any `*_test.go`
(all `Verbose` hits are the `ui.Verbose` diagnostics *sink object*, not env-level parsing), and the
`TestRunDefault_VerboseEnv` (@default_action_test.go:894) which only proves `=1` works.

---

## 9. Parallel-execution coordination

This item runs in parallel with **P1.M1.T3.S1** (docs/configuration.md git-config key spelling)
and possibly lingering effects of **P1.M1.T2.S1** (env-var table rows, ALREADY landed). Risk:
shared file `docs/configuration.md` line-number drift. Mitigation: **locate the env-row edit by
content via grep**, never by hardcoded line number. The two edits land in DIFFERENT sections
(env-var table vs git-config section) → no merge conflict, only drift, which the content anchor
absorbs. `load.go`, `verbose.go`, `cli.md`, `load_test.go` are touched by NO other in-flight task
(plan_status: P1.M2.T5/T6/T7 are different files, all Planned/not-started).

---

## 10. Validation commands (verified against Makefile)

- Full suite (race): `make test`  (= `go test -race ./...`)
- Focused: `go test -race ./internal/config/ -run 'Verbose' -v`
- Lint (staticcheck+gosimple+errcheck+govet+ineffassign+unused): `make lint`
  (= `golangci-lint run`; CI pins **v1.61** — v2 rejects the v1 schema in .golangci.yml)
- Coverage gate (≥85% on internal/{git,provider,generate,config}): `make coverage-gate`
- Build: `make build`

---

## 11. External references

- `https://pkg.go.dev/strconv#ParseBool` — accepted-value set + NumError on anything else (why
  `"2"` fails today and why a pre-check is the clean fix).
- `https://go.dev/blog/go1.13-errors` — `%w` wrapping conventions; here the new leaf has no
  underlying error so `errors.New` is correct (not `%w`).
