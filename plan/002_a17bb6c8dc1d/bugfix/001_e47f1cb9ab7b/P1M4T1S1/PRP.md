# PRP — P1.M4.T1.S1 (bugfix-001, plan 002): Add `ResolveConfigPath` and refactor `config.Load` to use it

**Issue**: bugfix-001 Issue 4 (Major) — `config init`/`upgrade`/`path` ignore `--config`/`STAGECOACH_CONFIG` and always operate on the global config path. This subtask is the **enabling refactor**: extract the override-aware path resolution that `config.Load` already performs inline into a shared, exported `config.ResolveConfigPath` so S2 can wire it into the config subcommands.
**PRD refs**: §15.2 (`--config`/`STAGECOACH_CONFIG` "overrides discovery"), §9.8 FR38 (`config init`/`path`/`upgrade`), §16.1 layer resolution.
**Binding analysis**: `plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md` (Step 1 + Test Strategy 1).

---

## Goal

**Feature Goal**: Extract the global-config-path resolution currently inlined in `config.Load` (internal/config/load.go:76-83) into a single exported, override-aware resolver `config.ResolveConfigPath(flagConfig string) string` in internal/config/file.go, with precedence `--config > STAGECOACH_CONFIG > GlobalConfigPath()` — and refactor `config.Load` to call it. `config.Load`'s observable behavior must be byte-identical (refactor for DRY, not a behavior change).

**Deliverable**:
1. **Code (new)**: `func ResolveConfigPath(flagConfig string) string` in internal/config/file.go (Path-helpers section), with a Go doc comment explaining the precedence and that it is the shared resolver for `config.Load` and the config subcommands.
2. **Code (refactor)**: internal/config/load.go:76-83 reduced to two lines (`globalPath := ResolveConfigPath(opts.ConfigPathOverride)` + the unchanged `explicit :=` line).
3. **Tests**: a table-driven `TestResolveConfigPath` in internal/config/file_test.go covering the four precedence cases.
4. **Docs**: Go doc comment on `ResolveConfigPath` (Mode A — the only doc change for S1; no user-facing docs yet; subcommand wiring + its docs are S2).

**Success Definition**:
- `go build ./...`, `go vet ./...`, `go test -race ./internal/config/...`, `go test -race ./...`, `make lint` all green.
- `ResolveConfigPath` reproduces the exact precedence of the old inline block; `config.Load`'s output for every existing Load test (override-present, env-beats-discovery, flag-beats-env, explicit-missing→error, bootstrap, discovery) is unchanged.
- The new `TestResolveConfigPath` passes for all four cases.

## Why

- **Unblocks the fix**: Issue 4's fix needs the config subcommands to reuse the *same* override-aware path resolution as the default action. Today that logic lives inline in `config.Load`, unreachable from the subcommands. Extracting it into a shared function is the prerequisite that S2 consumes (`config.ResolveConfigPath(flagConfig)`). Without this, S2 would have to duplicate the resolution (a second drift-prone copy).
- **DRY / single source of truth**: one resolver, two call sites (Load + subcommands). The precedence rule (`--config > STAGECOACH_CONFIG > GlobalConfigPath()`) becomes authoritative in exactly one place.
- **Zero behavior change to Load**: this is a pure extraction. The `explicit` flag (which distinguishes a missing EXPLICIT path → hard error from a missing discovery path → layer-absent/bootstrap) is preserved verbatim.

## What

### User-visible behavior
None in S1 — this is an internal refactor. `config.Load` behaves exactly as before. The user-visible fix (subcommands honoring the override) is S2.

### Technical behavior
- `config.ResolveConfigPath(flagConfig)`:
  - `flagConfig != ""` → returns `flagConfig` (the `--config` value).
  - else `STAGECOACH_CONFIG` set → returns the env value.
  - else → returns `GlobalConfigPath()` (`$XDG_CONFIG_HOME/stagecoach/config.toml` when set & absolute, else `~/.config/stagecoach/config.toml`, last-resort `config.toml`).
- `config.Load`: `globalPath := ResolveConfigPath(opts.ConfigPathOverride)`; `explicit` still computed as `opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""`.

### Success Criteria
- [ ] `ResolveConfigPath` added to internal/config/file.go in the Path-helpers section, precedence `flagConfig > STAGECOACH_CONFIG > GlobalConfigPath()`.
- [ ] Go doc comment on `ResolveConfigPath` states the precedence and that it's the shared resolver for `config.Load` + the config init/upgrade/path subcommands.
- [ ] internal/config/load.go:76-83 refactored to call `ResolveConfigPath`; the `explicit :=` line preserved byte-identical; no other Load logic touched.
- [ ] `TestResolveConfigPath` (table-driven, 4 cases) added to internal/config/file_test.go using `t.Setenv`.
- [ ] All existing Load tests pass unchanged (`TestLoad_ConfigPathOverride`, `TestLoad_STAGECOACH_CONFIG_EnvPath`, `TestLoad_ConfigPathOverride_MissingFileFails`, etc.).
- [ ] No change to `internal/cmd/config.go` (S2's scope); no user-facing doc change (S1).

## All Needed Context

### Context Completeness Check
✅ Passes the "No Prior Knowledge" test: the exact lines to extract, the exact insertion site, the function body, the doc-comment requirement, the test cases, and the regression-guard tests are all specified below.

### Documentation & References

```yaml
# MUST READ — binding analysis for this fix
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md
  section: "The Fix" → Step 1 (Add ResolveConfigPath) + Test Strategy 1 (unit test)
  why: THE binding root-cause + fix analysis. Contains the verbatim function body, the precedence
       rationale, the Load refactor, and the exact 4 table-driven test cases.
  critical: ResolveConfigPath returns ONLY the path (not `explicit`); keep the explicit := line in Load.
            Behavior must be byte-identical (this is an extraction, not a behavior change).

# The inline logic to EXTRACT (internal/config/load.go)
- file: internal/config/load.go
  lines: 76-83 (the globalPath resolution block) — inside Load(), right after `cfg := Defaults()`.
  why: This 8-line block (globalPath := opts.ConfigPathOverride; explicit := ...; if globalPath == "" {...})
       is what becomes ResolveConfigPath. The line above it (explicit :=) MUST be preserved verbatim.
  pattern: Replace the 8 lines with `globalPath := ResolveConfigPath(opts.ConfigPathOverride)` + keep the
           existing `explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""`.

# The file to add the function to (Path-helpers section)
- file: internal/config/file.go
  lines: ~85-97 (GlobalConfigPath wrapper + globalConfigPath impl); insert ResolveConfigPath right AFTER
         globalConfigPath() and before repoLocalConfigPath() (Path helpers section).
  why: file.go already imports os + path/filepath (lines 3-5). GlobalConfigPath() is the discovery fallback
       ResolveConfigPath calls. Colocating keeps all path resolution in one place.
  gotcha: ResolveConfigPath must call the EXPORTED GlobalConfigPath() (not the unexported globalConfigPath())
          — it is a package API; matching the architecture doc verbatim. (Both return the same value; the
          exported form is the documented contract for future cross-package callers like S2.)

# The existing path tests to MODEL the new test on (internal/config/file_test.go)
- file: internal/config/file_test.go
  lines: 148-181 (TestGlobalConfigPath — os.Setenv/defer-restore XDG pattern + filepath.Join expected);
         321-335 (TestGlobalConfigPath_Wrapper — exercises the exported wrapper)
  why: Copy the file/env isolation pattern. Prefer t.Setenv (auto-restores, non-parallel) over the older
       os.Setenv/defer-restore — the repo uses t.Setenv at load_test.go:709 and the contract requests it.
  pattern: Table-driven; for the GlobalConfigPath() fallback case make it deterministic via
           t.Setenv("XDG_CONFIG_HOME", t.TempDir()) → expected filepath.Join(xdg, "stagecoach", "config.toml").

# The Load regression guards — DO NOT MODIFY (must stay green)
- file: internal/config/load_test.go
  lines: 684 (TestLoad_ConfigPathOverride); 700 (TestLoad_STAGECOACH_CONFIG_EnvPath — env beats discovery,
         flag beats env); 728 (TestLoad_ConfigPathOverride_MissingFileFails — explicit missing → "config
         file not found")
  why: These exercise Load end-to-end through the refactored block. They are the proof the extraction is
       behavior-preserving. If any of these FAILS after the refactor, the extraction is wrong.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  file.go        # GlobalConfigPath/globalConfigPath (~85-97) — ADD ResolveConfigPath here (Path helpers)
  file_test.go   # TestGlobalConfigPath (148), _Wrapper (321) — ADD TestResolveConfigPath
  load.go        # Load() globalPath block (76-83) — REFACTOR to call ResolveConfigPath
  load_test.go   # TestLoad_ConfigPathOverride (684), _EnvPath (700), _MissingFileFails (728) — regression guards
```

### Desired Codebase tree (files MODIFIED — no new files)

```bash
internal/config/file.go        # +ResolveConfigPath (with Go doc comment)
internal/config/load.go        # -8 lines / +1 line in Load() (refactor to use ResolveConfigPath)
internal/config/file_test.go   # +TestResolveConfigPath (table-driven, 4 cases)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — ResolveConfigPath returns ONLY the path. Load's `explicit` flag (which decides "missing
// EXPLICIT path → hard error" vs "missing discovery → layer-absent/bootstrap") MUST stay computed in
// Load, byte-identical:
//   explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
// Do NOT try to derive `explicit` from the returned path (a valid flagConfig could equal the global path).

// CRITICAL — this is a PURE EXTRACTION (behavior-preserving). The new function must reproduce the EXACT
// precedence order of the old inline block: flagConfig, then STAGECOACH_CONFIG, then globalConfigPath().
// Reordering would break TestLoad_STAGECOACH_CONFIG_EnvPath (flag-beats-env) or the discovery fallback.

// GOTCHA — call the EXPORTED GlobalConfigPath() inside ResolveConfigPath (not the unexported
// globalConfigPath()). Both return the same value; the exported form is the documented API for cross-
// package callers (S2's cmd/config.go is in package cmd, not config).

// GOTCHA — load.go still needs the `os` import (the explicit line calls os.Getenv). ResolveConfigPath is
// in the same package (file.go) → NO new import in either file.

// GOTCHA — env-test isolation: STAGECOACH_CONFIG may be set in the ambient test environment. Each test case
// must explicitly set/clear STAGECOACH_CONFIG via t.Setenv (use t.Setenv("STAGECOACH_CONFIG", "") for the
// "absent" cases) so cases don't cross-contaminate.
```

## Implementation Blueprint

### Data models and structure

No new data models. This is a single pure function + a refactor. The relevant existing shapes:
- `LoadOpts.ConfigPathOverride string` (load.go) — the `--config` flag value threaded from the CLI.
- `GlobalConfigPath() string` (file.go) — the discovery fallback.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD config.ResolveConfigPath to internal/config/file.go
  - INSERT: the function (below) in the Path-helpers section, immediately AFTER globalConfigPath()'s
            closing brace and BEFORE repoLocalConfigPath().
  - EXACT body:
      // ResolveConfigPath returns the config file path, honoring overrides in the SAME precedence as
      // config.Load: flagConfig (--config) > STAGECOACH_CONFIG env > GlobalConfigPath() discovery. It is the
      // shared resolver for config.Load and the config init/upgrade/path subcommands (bugfix-001 Issue 4).
      func ResolveConfigPath(flagConfig string) string {
          if flagConfig != "" {
              return flagConfig
          }
          if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
              return env
          }
          return GlobalConfigPath()
      }
  - NAMING: exported ResolveConfigPath (CamelCase, exported because S2 calls it from package cmd).
  - DO NOT: change GlobalConfigPath/globalConfigPath/repoLocalConfigPath; add imports (os already present).

Task 2: REFACTOR internal/config/load.go::Load to use ResolveConfigPath
  - REPLACE the 8-line block at lines 76-83:
        globalPath := opts.ConfigPathOverride
        explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
        if globalPath == "" {
            if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
                globalPath = env
            } else {
                globalPath = globalConfigPath()
            }
        }
    WITH:
        globalPath := ResolveConfigPath(opts.ConfigPathOverride)
        explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
  - PRESERVE: the `explicit :=` line VERBATIM (it drives the explicit-missing hard error + bootstrap
              logic below). PRESERVE everything else in Load (loadTOML/overlay/bootstrap/explicit branches).
  - OPTIONAL: append " (via ResolveConfigPath)" to the comment at lines ~71-75 for accuracy.
  - VERIFY after: `go build ./...` (ResolveConfigPath is same-package; no import change) &&
                  `go vet ./...`.

Task 3: ADD TestResolveConfigPath to internal/config/file_test.go
  - PLACE: next to TestGlobalConfigPath (line 148) / TestGlobalConfigPath_Wrapper (line 321).
  - IMPLEMENT: table-driven test covering the 4 precedence cases:
      (a) flagConfig set (STAGECOACH_CONFIG unset) → returns flagConfig.
      (b) flagConfig empty, STAGECOACH_CONFIG set → returns the env value.
      (c) BOTH set → flagConfig wins.
      (d) neither set → returns GlobalConfigPath().
  - FOLLOW pattern: t.Setenv for STAGECOACH_CONFIG (and XDG_CONFIG_HOME for case d); for the "unset" env
            cases use t.Setenv("STAGECOACH_CONFIG", "") so ambient env can't leak. For case (d), make the
            expected path deterministic: t.Setenv("XDG_CONFIG_HOME", t.TempDir()); expected =
            filepath.Join(xdg, "stagecoach", "config.toml") (matches globalConfigPath's absolute-XDG branch).
  - NAMING: func TestResolveConfigPath(t *testing.T); table row names like "flag_only"/"env_only"/
            "flag_beats_env"/"neither_global".
  - COVERAGE: all 4 cases; assert exact equality (got == want).
  - DO NOT: modify the existing TestGlobalConfigPath* tests; touch load_test.go.

Task 4: NO user-facing docs this subtask
  - S1 ships ONLY the Go doc comment on ResolveConfigPath (Task 1). The config-subcommand behavior change
    + its docs land in S2; the README/overview sweep is P1.M6.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the new resolver (file.go, Path-helpers section):
//   Pure function of (flagConfig string, env STAGECOACH_CONFIG, GlobalConfigPath()). No side effects.
//   Precedence is a strict cascade: first non-empty source wins.
func ResolveConfigPath(flagConfig string) string {
	if flagConfig != "" {
		return flagConfig
	}
	if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
		return env
	}
	return GlobalConfigPath()
}

// PATTERN — the Load refactor (load.go:76-83 → 2 lines). The explicit flag is the ONLY thing
// ResolveConfigPath does NOT surface, so it stays inline:
//   globalPath := ResolveConfigPath(opts.ConfigPathOverride)   // <-- replaces the 7-line cascade
//   explicit   := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""  // UNCHANGED
// globalPath then feeds loadTOML / the explicit-missing hard error / bootstrap exactly as before.
```

### Integration Points

```yaml
CODE (this subtask):
  - file: internal/config/file.go
    change: "+ResolveConfigPath(flagConfig string) string (with Go doc comment)"
  - file: internal/config/load.go
    change: "Load() globalPath block (76-83) → ResolveConfigPath(opts.ConfigPathOverride); explicit kept"
    risk: NONE (pure behavior-preserving extraction; regression guards in load_test.go prove it).

TESTS:
  - file: internal/config/file_test.go
    change: "+TestResolveConfigPath (4 table cases)"
    guards: TestLoad_ConfigPathOverride (684), TestLoad_STAGECOACH_CONFIG_EnvPath (700),
            TestLoad_ConfigPathOverride_MissingFileFails (728) stay GREEN UNCHANGED.

NO DATABASE / NO NEW CONFIG KEYS / NO ROUTES / NO NEW DEPENDENCIES / NO CLI CHANGES (S2 owns cmd/config.go).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 + Task 2 — fix before proceeding to tests.
go build ./...            # compiles (ResolveConfigPath same-package; os already imported in file.go)
go vet ./...              # vet clean
make lint                 # golangci-lint — zero findings
# Expected: zero errors. The refactor removes code + adds one function; types already line up.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new test (after Task 3):
go test -race ./internal/config/ -run TestResolveConfigPath -v
# Expected: PASS all 4 cases (flag_only, env_only, flag_beats_env, neither_global).

# Targeted regression gate — proves the extraction is behavior-preserving:
go test -race ./internal/config/ -run 'TestLoad_(ConfigPathOverride|ConfigPathOverride_MissingFileFails|STAGECOACH_CONFIG_EnvPath)' -v
# Expected: all PASS unchanged (override-present, env-beats-discovery, flag-beats-env, explicit-missing→error).

# Full config package:
go test -race ./internal/config/...
# Expected: ALL green (incl. bootstrap + discovery tests that flow through the refactored block).

# Full suite (Makefile `test`):
go test -race ./...
# Expected: all packages pass. If a Load test FAILS, the extraction reordered the precedence — re-check.
```

### Level 3: Integration / End-to-End Smoke (manual proof, optional)

```bash
# ResolveConfigPath has no CLI wiring yet (that's S2), but prove Load still honors the override end-to-end:
go build -o /tmp/stagecoach ./cmd/stagecoach
tmp=$(mktemp -d) && cd "$tmp"
git init -q && git config user.email t@t && git config user.name t
printf '[defaults]\nprovider = "overridden"\n' > /tmp/override.toml
/tmp/stagecoach --config /tmp/override.toml config path   # S2 will make this print the override; S1: still global (expected — S1 is config pkg only)
# The real S1 proof is the unit + Load test gates above. (The subcommand honoring the override is S2.)
cd - && rm -rf "$tmp"
```

### Level 4: Doc Validation

```bash
# The Go doc comment renders correctly in `go doc`:
go doc github.com/dustin/stagecoach/internal/config.ResolveConfigPath
# Expected: prints the doc comment (precedence + "shared resolver for config.Load and the config ... subcommands").
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./internal/config/...` green (incl. new TestResolveConfigPath + all Load guards).
- [ ] `go test -race ./...` green.
- [ ] `make lint` — zero findings.

### Feature Validation
- [ ] `ResolveConfigPath` in file.go Path-helpers section; precedence `flagConfig > STAGECOACH_CONFIG > GlobalConfigPath()`.
- [ ] Go doc comment states the precedence + that it's the shared resolver for Load + config subcommands.
- [ ] load.go:76-83 refactored to `ResolveConfigPath(opts.ConfigPathOverride)`; `explicit :=` preserved verbatim.
- [ ] `TestResolveConfigPath` (4 table cases) passes using `t.Setenv`.
- [ ] Load regression guards (`TestLoad_ConfigPathOverride`, `_EnvPath`, `_MissingFileFails`) green unchanged.

### Code Quality Validation
- [ ] Follows existing file.go path-helper placement + doc-comment style.
- [ ] Pure behavior-preserving extraction; no Load observable change.
- [ ] No new imports, no new files, no CLI/doc-of-record changes (S2/P1.M6 scope).

### Documentation & Boundaries
- [ ] S1 ships ONLY the Go doc comment (Mode A). NO user-facing doc change yet.
- [ ] `internal/cmd/config.go` (subcommand wiring) explicitly deferred to S2; README/overview sweep to P1.M6.

---

## Anti-Patterns to Avoid

- ❌ Don't reorder the precedence cascade in `ResolveConfigPath` — it must match the old inline block exactly (flag → env → global) or `TestLoad_STAGECOACH_CONFIG_EnvPath` (flag-beats-env) breaks.
- ❌ Don't try to derive `explicit` from the returned path — keep the existing `explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""` line in Load verbatim.
- ❌ Don't call the unexported `globalConfigPath()` inside `ResolveConfigPath` — use the exported `GlobalConfigPath()` (the documented cross-package API S2 relies on).
- ❌ Don't modify `internal/cmd/config.go` or any user-facing doc — that's S2 / P1.M6.
- ❌ Don't leave `STAGECOACH_CONFIG` unmanaged in the new test cases — explicitly `t.Setenv(...)` (or `t.Setenv("STAGECOACH_CONFIG","")`) each case to avoid ambient-env cross-contamination.
- ❌ Don't treat this as a behavior change — it is a pure DRY extraction; if any Load test changes behavior, the extraction is wrong.

---

## Confidence Score

**9 / 10** — This is a small, well-bounded pure extraction: one ~10-line function added to file.go
(which already imports `os` and exposes `GlobalConfigPath`), an 8-line block in Load collapsed to 2
lines with the `explicit` flag preserved verbatim, and a 4-case table-driven test modeled on the
existing `TestGlobalConfigPath`. The binding architecture doc provides the verbatim function body and
exact test cases. The regression guards (`TestLoad_ConfigPathOverride`, `_EnvPath`,
`_MissingFileFails`) make the "behavior-preserving" claim mechanically checkable. The only residual
uncertainty is ambient `STAGECOACH_CONFIG` in the test environment, which is handled by explicit
`t.Setenv` per case.
