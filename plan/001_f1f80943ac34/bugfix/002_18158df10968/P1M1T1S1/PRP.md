---
name: "P1.M1.T1.S1 — Add explicit-vs-discovery distinction in config.Load and fail fast on a missing explicit path"
description: |
  Bugfix-002 Issue 1 (Major). In `internal/config/load.go` `Load`, track whether the global config
  path came from an EXPLICIT source (`--config` flag or `STAGEHAND_CONFIG` env) vs the DISCOVERY default.
  When the source is explicit AND the file is missing, return a hard error ("config file not found: <path>")
  instead of silently treating it as "layer absent" and falling back to provider auto-detection (which
  currently invokes a real, installed AI agent). The discovery path's `(nil,nil)` absence tolerance is
  PRESERVED unchanged. Only ONE production file is touched; tests + two docs ride with the work.
---

## Goal

**Feature Goal**: Eliminate the silent-wrong-behavior bug where an explicit `--config <path>` (or
`STAGEHAND_CONFIG=<path>`) pointing at a **nonexistent** file is indistinguishable from "the discovery
default isn't there yet" — causing Stagehand to silently auto-detect and invoke a real, installed AI
agent instead of erroring. Make a missing explicit path a hard, fail-fast error (exit 1) naming the
path, consistent with how a *malformed* or *directory* explicit path already errors today.

**Deliverable** (exactly two edits in ONE production file, `internal/config/load.go`):
1. Compute `explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""` where the
   global path is resolved.
2. Add an `else if explicit { return nil, fmt.Errorf("config file not found: %s", globalPath) }` arm to
   the existing `loadTOML` if/else chain — firing only when the file is missing (`g == nil && err == nil`)
   AND the path was explicit.

Plus: focused unit tests in `internal/config/load_test.go`, and two doc updates (Mode A).

**Success Definition**: `Load` returns a non-nil error whose message contains `config file not found`
and the path, when `opts.ConfigPathOverride` or `STAGEHAND_CONFIG` points at a missing file; `Load`
returns `(nil)` silently-absent (no error) for the discovery path with no global file, byte-for-byte
unchanged. The malformed-file and directory-path cases remain errors via the existing `err != nil`
arm (unaffected). `go build/vet/gofmt` clean; `go test -race ./...` green.

## User Persona

**Target User**: The Stagehand CLI user who passes `--config` (or sets `STAGEHAND_CONFIG`) and makes a
typo / points at a not-yet-created file.

**Use Case**: `stagehand --config ~/.config/stagehand/confgi.toml` (typo) or
`STAGEHAND_CONFIG=~/new.toml stagehand` before writing `new.toml`.

**Pain Points Addressed**: Today such a typo silently falls back to provider auto-detection and
**invokes a real agent** (consuming the user's wait, API call, quota — and possibly committing with a
real generated message). The fix makes the typo loud and immediate, exactly like a malformed file.

## Why

- **PRD intent: "overrides discovery".** §15.2 defines `--config <path>` as "Path to a config file
  (overrides discovery)"; §16.1 layer resolution treats it as the override layer. The intent is that
  the named file *is* the config — so its absence is an error, not "no config".
- **Consistency with sibling failure modes.** A *malformed* explicit path errors (`loadTOML` parse
  error → exit 1); a *directory* explicit path errors (`os.ReadFile` → "is a directory" → exit 1).
  Only the *missing* case is silently swallowed — an inconsistent and dangerous special case. This fix
  closes the gap.
- **Fail-fast (§13.5) / no surprise provider redirection (§19).** The error surfaces at config load
  (CLI `PersistentPreRunE`), before any provider is resolved and before the write-tree snapshot — no
  agent is invoked, no dangling objects, no quota consumed.
- **Minimal, surgical blast radius.** The fix is two lines in one function. `loadTOML`'s
  `(nil,nil)`-on-`IsNotExist` contract is PRESERVED (the discovery path still tolerates absence). No
  change to `file.go`, `root.go`, the discovery path, or `loadTOML`.

## What

A purely additive control-flow change to `internal/config/load.go` `Load`:

1. **Compute `explicit`** at the point the global path is resolved.
2. **Fail fast** in a new `else if explicit` arm when the file is missing.

No changes to `loadTOML`, `file.go`, `root.go`, the CLI wiring, the discovery default path, or any
other package. `--config` already flows via `ConfigPathOverride` (root.go:64); `STAGEHAND_CONFIG` is
already read in `Load`.

### Success Criteria

- [ ] `Load` returns a non-nil error when `opts.ConfigPathOverride` points at a missing file; the
      error message contains `config file not found` and the path.
- [ ] `Load` returns a non-nil error when `STAGEHAND_CONFIG` points at a missing file (same message).
- [ ] `Load` returns `(nil, nil)` (no error) for the discovery path when no global file exists —
      unchanged (regression-guarded by a test).
- [ ] The malformed-file case still returns a `global config: parse config…` error via the existing
      `err != nil` arm (unaffected).
- [ ] The directory-path case still returns a `global config: read config…is a directory` error
      (unaffected).
- [ ] `ConfigPathOverride` still beats `STAGEHAND_CONFIG` (existing precedence preserved).
- [ ] Only `internal/config/load.go` is modified in production source.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `docs/cli.md` and `docs/configuration.md` note that a missing explicit `--config`/`STAGEHAND_CONFIG`
      fails fast with exit 1.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current `Load` body, the exact two edits (with the
full if/else chain before and after), a truth table tracing every input, the error wrap chain to the
user-visible string, the test helpers to reuse, and the executable validation commands. The
architecture analysis (`issue_analysis.md` ISSUE 1) pre-resolved root cause + blast radius.

### Documentation & References

```yaml
# MUST READ — the binding fix analysis (do not re-litigate)
- docfile: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: "ISSUE 1 section quotes the exact buggy Load block, the exact loadTOML (nil,nil) contract, the exact fix (explicit + else-if-error), the CLI wrap chain ('config: config file not found: <path>' → exit 1), and the test helpers/idioms to reuse."
  critical: "States loadTOML's (nil,nil)-on-IsNotExist contract MUST be preserved for the discovery path. States ONLY internal/config/load.go is touched in production (not file.go, not root.go). Confirms --config already flows via ConfigPathOverride and STAGEHAND_CONFIG is already read in Load — no wiring change."

- docfile: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/system_context.md
  why: "Confirms the 7-layer config model and that Load resolves globalPath = ConfigPathOverride > STAGEHAND_CONFIG > globalConfigPath() discovery."

# The single production file under edit
- file: internal/config/load.go
  why: "THE edit target. Load() is the function to modify. The current globalPath-resolution block + the Layer-2 loadTOML if/else chain are quoted verbatim in the Implementation Blueprint."
  pattern: "Current: resolve globalPath (override > env > discovery) → `if g,err:=loadTOML(globalPath); err!=nil {wrap 'global config'} else if g!=nil {overlay}`. New: compute `explicit`; add `else if explicit {return 'config file not found: <path>'}`."
  gotcha: "Compute `explicit` from opts.ConfigPathOverride / os.Getenv BEFORE the `if globalPath==''` block so it reflects the SOURCE (override/env), independent of whether globalPath ended up non-empty. The new arm fires ONLY when g==nil && err==nil (missing file) — malformed (parse err) and directory (read err) still hit the existing `err != nil` arm."

# Cross-references (read-only — do NOT edit)
- file: internal/config/file.go
  why: "Defines loadTOML(path) (*Config, error) returning (nil,nil) on os.IsNotExist (file.go:99-102) — the discovery path's 'layer absent' sentinel. S1 PRESERVES this contract; do NOT change loadTOML."
  gotcha: "loadTOML returns a non-nil ERROR (not nil) for malformed TOML and for a directory path — so those cases never reach the new `else if explicit` arm. Only a genuinely-missing file returns (nil,nil)."

- file: internal/cmd/root.go
  why: "Confirms --config → flagConfig → LoadOpts.ConfigPathOverride (root.go:30,64,82) and that PersistentPreRunE wraps a Load error as `config: %w` → exitcode.Error (1). S1 does NOT edit root.go."
  pattern: "PersistentPreRunE: cfg,err:=config.Load(...); if err!=nil { return exitcode.New(exitcode.Error, fmt.Errorf('config: %w', err)) }. So the user sees `config: config file not found: <path>` and exit 1."

- file: internal/config/load_test.go
  why: "EDIT TARGET (tests). Reuses package-private helpers: writeConfigFile (:18), chdir (:35), newFlagSet (:50), loadEnvSetup (:62). Existing TestLoad_ConfigPathOverride (:510) + TestLoad_STAGEHAND_CONFIG_EnvPath (:526) use EXISTING files → still pass unchanged."
  pattern: "Error idiom: `strings.Contains(err.Error(), \"<tag>\")`. Place new tests in the 'Load — path resolution tests' block (near :545)."

- docfile: plan/001_f1f80943ac34/bugfix/002_18158df10968/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1 findings: the verbatim current code, the exact two edits, a full input→outcome truth table (every case traced), the error wrap chain, and the precise new-test list with the helpers each reuses."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
├── internal/config/
│   ├── load.go        # EDIT TARGET (Load — the single production change)
│   ├── load_test.go   # EDIT TARGET (add ~3 focused path-resolution tests)
│   ├── config.go      # read-only ref (Config struct; Defaults)
│   ├── file.go        # read-only ref (loadTOML (nil,nil) contract — PRESERVED)
│   └── git.go         # NOT touched
├── internal/cmd/
│   └── root.go        # read-only ref (ConfigPathOverride wiring, CLI wrap) — NOT touched
└── docs/
    ├── cli.md             # EDIT TARGET (Mode A — --config row + note)
    └── configuration.md   # EDIT TARGET (Mode A — config-path NOTE)
```

### Desired Codebase Tree After S1

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/config/load.go         # +2 edits (explicit var; else-if-error arm)
    internal/config/load_test.go    # +3 tests (missing override / missing env / discovery-absent OK)
    docs/cli.md                     # --config row + note updated
    docs/configuration.md           # NOTE updated
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/load.go` | MODIFY | Track `explicit`; fail fast on missing explicit path. **Only production file touched.** |
| `internal/config/load_test.go` | MODIFY | Add path-resolution tests for the missing-explicit / discovery-absent cases. |
| `docs/cli.md` | MODIFY | Mode A: `--config` row + note state missing-explicit fails fast (exit 1). |
| `docs/configuration.md` | MODIFY | Mode A: the config-path NOTE states missing-explicit fails fast (exit 1). |

**Explicitly NOT touched**: `internal/config/file.go` (loadTOML), `internal/config/git.go`,
`internal/cmd/root.go` (wiring unchanged), `pkg/stagehand/*`, any provider/manifest code,
`PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`. Issue 2/3/4 belong to their own
subtasks; the README + overview-doc sweep is the final M5 task.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL: loadTOML returns (nil, nil) on os.IsNotExist (file.go:100) — this is the DISCOVERY path's
// "layer absent" sentinel and MUST be preserved. S1 does NOT touch loadTOML/file.go. The fix lives
// ENTIRELY in Load, as a new `else if explicit` arm that interprets the SAME (nil,nil) return
// differently depending on whether the path was explicit or discovered.

// CRITICAL: compute `explicit` from the SOURCE, not from whether globalPath is non-empty.
//   explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""
// This is true iff the path came from --config or STAGEHAND_CONFIG (NOT from globalConfigPath()).
// Do NOT write `explicit := globalPath != ""` — that would also be true after discovery populated it.

// CRITICAL: the new arm must be `else if explicit` AFTER `else if g != nil` — it fires ONLY when
// g == nil && err == nil (missing file). A malformed file returns a non-nil ERROR from loadTOML
// (parse error), and a directory returns a non-nil ERROR (read error), so BOTH still hit the existing
// `err != nil` arm and remain errors. Only the genuinely-missing-file case is newly an error.

// GOTCHA (error string): the Load-level error is `config file not found: <path>` (no "config:" prefix).
// The "config:" prefix is added by root.go PersistentPreRunE's wrap (CLI-only). So a package-config
// unit test asserts `strings.Contains(err.Error(), "config file not found")` + the path — NOT "config:".

// GOTCHA (test isolation): loadEnvSetup(t) sets HOME and XDG_CONFIG_HOME to a temp dir and creates a
// repo. With NO global file written to globalDir and NO --config/STAGEHAND_CONFIG, discovery returns
// (nil) — this is the regression guard for the discovery-absent contract.

// GOTCHA (no wiring change): --config already flows to ConfigPathOverride (root.go:64); STAGEHAND_CONFIG
// is already read in Load. Do NOT edit root.go. The fix is purely inside Load's globalPath handling.
```

## Implementation Blueprint

### Data models and structure

No data-model changes. `LoadOpts` is unchanged:

```go
// internal/config/load.go (EXISTING — unchanged by S1)
type LoadOpts struct {
	ConfigPathOverride string         // from --config (CLI); "" => fall back to STAGEHAND_CONFIG, then discovery
	RepoDir            string         // repo root for git config
	Flags              *pflag.FlagSet // nil => skip the CLI-flag layer
}
func Load(ctx context.Context, opts LoadOpts) (*Config, error)

// internal/config/file.go (EXISTING — unchanged by S1; contract PRESERVED)
func loadTOML(path string) (*Config, error) // returns (nil, nil) on os.IsNotExist; (nil, err) on read/parse error
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/load.go — compute `explicit` + add the missing-explicit-error arm
  - LOCATE: the "Resolve the global-file path" block + the immediately-following "Layer 2" if/else chain.
  - EDIT A (compute explicit): change the globalPath-resolution block to also record the source.
      OLD (verbatim):
        // Resolve the global-file path: --config > STAGEHAND_CONFIG > discovery.
        globalPath := opts.ConfigPathOverride
        if globalPath == "" {
            if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
                globalPath = env
            } else {
                globalPath = globalConfigPath()
            }
        }
      NEW:
        // Resolve the global-file path: --config > STAGEHAND_CONFIG > discovery.
        // `explicit` records whether the path came from the user (--config / STAGEHAND_CONFIG) vs the
        // discovery default — a missing EXPLICIT path is a hard error (PRD §15.2 "overrides discovery");
        // a missing discovery file is the normal "layer absent" sentinel (tolerated below).
        globalPath := opts.ConfigPathOverride
        explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""
        if globalPath == "" {
            if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
                globalPath = env
            } else {
                globalPath = globalConfigPath()
            }
        }
  - EDIT B (fail fast on missing explicit path): extend the Layer-2 if/else chain with an else-if arm.
      OLD (verbatim):
        // Layer 2: global TOML (or --config/STAGEHAND_CONFIG override). nil => absent (no error).
        if g, err := loadTOML(globalPath); err != nil {
            return nil, fmt.Errorf("global config: %w", err)
        } else if g != nil {
            overlay(&cfg, g)
        }
      NEW:
        // Layer 2: global TOML (or --config/STAGEHAND_CONFIG override). A present file is overlaid; a
        // read/parse error is wrapped. A MISSING file is "layer absent" (no error) for discovery, but a
        // HARD ERROR when the path was explicit (a typo'd --config must not silently fall back to
        // auto-detection and invoke an unintended agent). loadTOML's (nil,nil) contract is preserved.
        if g, err := loadTOML(globalPath); err != nil {
            return nil, fmt.Errorf("global config: %w", err)
        } else if g != nil {
            overlay(&cfg, g)
        } else if explicit {
            return nil, fmt.Errorf("config file not found: %s", globalPath)
        }
  - GOTCHA: the new arm is the LAST `else if` — it fires only when g==nil && err==nil (missing file).
  - GOTCHA: error string is exactly "config file not found: %s" (the CLI wraps with a "config: " prefix).
  - VERIFY: `go build ./internal/config/` compiles; `gofmt -w internal/config/load.go` is a no-op.
  - DO NOT: modify loadTOML, file.go, root.go, or anything outside Load's globalPath handling.

Task 2: ADD focused tests in internal/config/load_test.go
  - PLACEMENT: in the "Load — path resolution tests" block, next to TestLoad_ConfigPathOverride (:510)
    and TestLoad_STAGEHAND_CONFIG_EnvPath (:526).
  - REUSE helpers: loadEnvSetup(t), writeConfigFile(t,dir,relPath,body), chdir(t,dir).
  - ADD test 1 — TestLoad_ConfigPathOverride_MissingFileFails:
      _, repo, _ := loadEnvSetup(t); chdir(t, repo)
      missing := filepath.Join(t.TempDir(), "does-not-exist.toml")
      _, err := Load(context.Background(), LoadOpts{ConfigPathOverride: missing, RepoDir: repo})
      ASSERT err != nil
      ASSERT strings.Contains(err.Error(), "config file not found")
      ASSERT strings.Contains(err.Error(), "does-not-exist.toml")
  - ADD test 2 — TestLoad_STAGEHAND_CONFIG_EnvPath_MissingFileFails:
      _, repo, _ := loadEnvSetup(t); chdir(t, repo)
      t.Setenv("STAGEHAND_CONFIG", filepath.Join(t.TempDir(), "nope.toml"))
      _, err := Load(context.Background(), LoadOpts{RepoDir: repo})  // no ConfigPathOverride
      ASSERT err != nil && strings.Contains(err.Error(), "config file not found") && contains "nope.toml"
  - ADD test 3 (regression guard) — TestLoad_DiscoveryMissingFileOK:
      _, repo, _ := loadEnvSetup(t); chdir(t, repo)
      // no --config, no STAGEHAND_CONFIG, NO global file written to globalDir
      os.Unsetenv("STAGEHAND_CONFIG")  // belt-and-suspenders (loadEnvSetup doesn't set it)
      cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
      ASSERT err == nil            // discovery tolerates absence — contract preserved
      ASSERT cfg != nil            // Defaults() still returned
  - OPTIONAL test 4 — TestLoad_ConfigPathOverride_MissingBeatsEnv: both ConfigPathOverride (missing)
    and STAGEHAND_CONFIG set → still errors (override is the resolved path); confirms precedence + error.
  - COVERAGE: each test asserts the error TAG + the PATH substring (the CLI adds "config: "; Load does not).
  - DO NOT: change existing TestLoad_ConfigPathOverride / TestLoad_STAGEHAND_CONFIG_EnvPath (they use
    existing files → still pass). DO NOT assert on "config:" prefix at the Load level.

Task 3: UPDATE docs (Mode A — rides with the work)
  - docs/cli.md:
      · --config row (line ~20): append to the description that a path pointing at a MISSING file fails
        fast with exit 1 (like a malformed/directory path), rather than falling back to discovery.
      · prose note (line ~30): add one sentence — an explicit --config / STAGEHAND_CONFIG to a missing
        file errors with `config: config file not found: <path>` (exit 1) instead of silently
        auto-detecting a provider.
  - docs/configuration.md:
      · the `[!NOTE]` at line ~34 (the --config/STAGEHAND_CONFIG note): add the same one sentence —
        a missing explicit path fails fast with exit 1; only the discovery default tolerates absence.
  - SCOPE: only the config-path wording. Do NOT touch Issue 2/3/4 doc areas (output/strip_code_fence,
    merge-conflict message, dry-run exit codes) — those belong to their subtasks / the M5 sweep.

Task 4: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # new tests green; existing path tests + full suite green
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === Load's globalPath block AFTER S1 (the complete edited region) ===

	// Resolve the global-file path: --config > STAGEHAND_CONFIG > discovery.
	// `explicit` records whether the path came from the user (--config / STAGEHAND_CONFIG) vs the
	// discovery default — a missing EXPLICIT path is a hard error (PRD §15.2 "overrides discovery");
	// a missing discovery file is the normal "layer absent" sentinel (tolerated below).
	globalPath := opts.ConfigPathOverride
	explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""
	if globalPath == "" {
		if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
			globalPath = env
		} else {
			globalPath = globalConfigPath()
		}
	}

	// Layer 2: global TOML (or --config/STAGEHAND_CONFIG override). A present file is overlaid; a
	// read/parse error is wrapped. A MISSING file is "layer absent" (no error) for discovery, but a
	// HARD ERROR when the path was explicit (a typo'd --config must not silently fall back to
	// auto-detection and invoke an unintended agent). loadTOML's (nil,nil) contract is preserved.
	if g, err := loadTOML(globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	} else if g != nil {
		overlay(&cfg, g)
	} else if explicit {
		return nil, fmt.Errorf("config file not found: %s", globalPath)
	}
```

```go
// === Input → outcome truth table (verified by tracing every case) ===
//   --config /existing.toml            override  explicit  g!=nil   → overlay (unchanged)
//   --config /missing.toml             override  explicit  (nil,nil)→ ERROR "config file not found" (FIXED)
//   --config /malformed.toml           override  explicit  parseErr → "global config: parse config…" (unchanged)
//   --config /somedir                  override  explicit  readErr  → "global config: read config…is a directory" (unchanged)
//   STAGEHAND_CONFIG=/missing (only)   env       explicit  (nil,nil)→ ERROR (FIXED)
//   neither, no global file            discovery !explicit (nil,nil)→ no error, layer absent (unchanged)
//   neither, global file present       discovery !explicit g!=nil   → overlay (unchanged)
```

### Integration Points

```yaml
PRODUCTION (internal/config/load.go Load):
  - var added: "explicit := opts.ConfigPathOverride != \"\" || os.Getenv(\"STAGEHAND_CONFIG\") != \"\""
  - arm added: "else if explicit { return nil, fmt.Errorf(\"config file not found: %s\", globalPath) }"
  - fires ONLY when: g == nil && err == nil (missing file) AND explicit == true

ERROR PROPAGATION (unchanged wrap chain):
  - Load returns:     "config file not found: <path>"
  - root.go wrap:     "config: config file not found: <path>"  → exitcode.Error (1), pre-provider-resolution

NO-TOUCH (explicitly):
  - internal/config/file.go (loadTOML (nil,nil) contract — PRESERVED)
  - internal/config/git.go
  - internal/cmd/root.go (ConfigPathOverride wiring already in place; CLI wrap already exists)
  - pkg/stagehand/*, internal/provider/*, internal/git/*   # provider resolution is DOWNSTREAM of this fail-fast
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S1):
  - Issue 2 (output/strip_code_fence tri-state): P1.M2 — separate subtask
  - Issue 3 (merge-conflict message): P1.M3 — separate subtask
  - Issue 4 (dry-run exit codes): P1.M4 — separate subtask
  - README + overview-doc sweep: P1.M5 (final task, depends on all implementing subtasks)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l .                       # Expected: empty (run `gofmt -w internal/config/load.go` if it lists it)
go vet ./internal/config/...     # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagehand

# The three new path-resolution tests
go test -race -run 'TestLoad_ConfigPathOverride_MissingFileFails|TestLoad_STAGEHAND_CONFIG_EnvPath_MissingFileFails|TestLoad_DiscoveryMissingFileOK' ./internal/config/ -v

# The existing path tests MUST still pass (they use existing files)
go test -race -run 'TestLoad_ConfigPathOverride|TestLoad_STAGEHAND_CONFIG_EnvPath' ./internal/config/ -v

# Full config-package suite
go test -race ./internal/config/ -v

# Expected: new tests PASS (missing-explicit errors with "config file not found" + path; discovery-absent
# returns nil); existing tests PASS unchanged; full package green.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages pass
go vet ./...                     # Expected: exit 0

# Confirm ONLY internal/config/load.go changed in production source (git diff --stat)
git diff --stat -- internal/ pkg/ cmd/
# Expected: only internal/config/load.go (+ the test file + docs) appear.
```

### Level 4: End-to-End Smoke (reproduces the PRD repro — optional but recommended)

```bash
cd /home/dustin/projects/stagehand
make build 2>/dev/null || go build -o bin/stagehand ./cmd/stagehand

# Setup a throwaway repo
cd /tmp && rm -rf cfgbug && mkdir cfgbug && cd cfgbug
git init -q && git config user.email t@t.com && git config user.name t
git commit -q --allow-empty -m init && echo "content here" > file.txt && git add file.txt
SH=/home/dustin/projects/stagehand/bin/stagehand

# (a) missing --config → now exit 1 with "config file not found" (was: exit 0, real agent invoked)
$SH --config /tmp/cfgbug/DOES_NOT_EXIST.toml --dry-run ; echo "EXIT=$?"
# Expected: stderr contains "config: config file not found: /tmp/cfgbug/DOES_NOT_EXIST.toml"; EXIT=1

# (b) missing STAGEHAND_CONFIG → same
STAGEHAND_CONFIG=/tmp/cfgbug/NOPE2.toml $SH --dry-run ; echo "EXIT=$?"
# Expected: EXIT=1, "config file not found"

# (c) discovery with NO global file → still exit-able normally (NOT an error); provider auto-detect proceeds
#     (ensure HOME/XDG isolated so no real global file interferes)
HOME=/tmp/cfgbug/nohome $SH --dry-run ; echo "EXIT=$?"
# Expected: proceeds to generation (exit 0 with --dry-run, or its own provider outcome) — NOT "config file not found"

# (d) malformed explicit → still errors (unchanged behavior)
printf 'bad [toml' > bad.toml && $SH --config bad.toml --dry-run ; echo "EXIT=$?"
# Expected: EXIT=1, "expected '='" (or similar parse error) — NOT "config file not found"
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (new tests green, existing path tests unchanged).

### Feature Validation

- [ ] `Load` errors with `config file not found: <path>` for a missing `ConfigPathOverride` path.
- [ ] `Load` errors with `config file not found: <path>` for a missing `STAGEHAND_CONFIG` path.
- [ ] `Load` returns `(nil)` (no error) for discovery with no global file — regression-guarded by a test.
- [ ] Malformed-file and directory-path cases remain errors via the existing `err != nil` arm.
- [ ] `ConfigPathOverride` precedence over `STAGEHAND_CONFIG` preserved.
- [ ] End-to-end smoke (Level 4): missing explicit → exit 1; discovery-absent → not an error.

### Scope Discipline Validation

- [ ] ONLY `internal/config/load.go` modified in production source (git diff --stat confirms).
- [ ] Did NOT modify `loadTOML` / `file.go` (the `(nil,nil)` discovery contract is preserved).
- [ ] Did NOT modify `root.go` / CLI wiring (`--config` already flows via `ConfigPathOverride`).
- [ ] Did NOT touch Issue 2/3/4 code areas (separate subtasks).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Documentation

- [ ] `docs/cli.md` `--config` row + note state a missing explicit path fails fast (exit 1).
- [ ] `docs/configuration.md` NOTE states the same.
- [ ] Doc edits are scoped to the config-path wording only (Issue 2/3/4 doc areas left to their subtasks).

---

## Anti-Patterns to Avoid

- ❌ Don't modify `loadTOML` or `file.go` — the `(nil,nil)`-on-`IsNotExist` return is the discovery
  path's "layer absent" sentinel and is shared by both paths. The distinction must live in `Load`.
- ❌ Don't compute `explicit` from `globalPath != ""` — discovery populates `globalPath` too. Compute it
  from the SOURCE: `opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""`.
- ❌ Don't reorder the if/else so malformed/directory files hit the new arm — those return non-nil
  ERRORS from `loadTOML` and must continue to hit the `err != nil` arm. The new arm is the LAST
  `else if`, firing only when `g == nil && err == nil`.
- ❌ Don't edit `root.go` — `--config` already reaches `ConfigPathOverride` and the CLI already wraps
  Load errors as `config: %w` → exit 1. The fix is entirely inside `Load`.
- ❌ Don't assert on the `config:` prefix in a `Load`-level unit test — that prefix is CLI-only
  (`PersistentPreRunE`'s wrap). At the package-config level the error is `config file not found: <path>`.
- ❌ Don't change existing `TestLoad_ConfigPathOverride` / `TestLoad_STAGEHAND_CONFIG_EnvPath` — they
  use existing files and remain valid. Add new tests alongside them.
- ❌ Don't sweep Issue 2/3/4 docs or README in this subtask — those belong to their own subtasks and
  the final M5 changeset sweep.
- ❌ Don't ignore a failing `go test -race` — fix root cause; the discovery-absent contract must hold.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a two-line, single-function, single-production-file edit with the exact current
code and exact target code quoted verbatim, plus a complete input→outcome truth table tracing every
case (missing/malformed/directory × override/env/discovery). The architecture analysis
(`issue_analysis.md` ISSUE 1) pre-resolved root cause, blast radius, the CLI error-wrap chain, and the
test helpers to reuse. The only residual uncertainty (not 10/10) is the optional 4th test and the
exact doc-sentence wording — both are cosmetic and gated by the deterministic validation commands.
The discovery path is explicitly regression-guarded, and the malformed/directory cases are provably
unaffected (they return non-nil errors, never reaching the new arm).
