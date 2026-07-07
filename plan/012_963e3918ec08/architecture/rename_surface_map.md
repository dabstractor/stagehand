# Rename Surface Map: stagehand → stagecoach

## Overview

The rename touches every layer of the project. Below is the complete surface,
organized by dependency order (earlier items must complete before later ones
can be verified).

## Layer 1: Go Module & Package Structure (MUST BE FIRST)

### 1.1 Module path (`go.mod`)
- **Line 1:** `module github.com/dustin/stagehand` → `module github.com/dustin/stagecoach`
- **Tool:** `go mod edit -module github.com/dustin/stagecoach`

### 1.2 Import paths (all `.go` files)
- **Pattern:** `github.com/dustin/stagehand/` → `github.com/dustin/stagecoach/`
- **Count:** ~257 occurrences across ~100+ Go files
- **Tool:** `find . -name '*.go' -exec sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' {} +`

### 1.3 Directory renames
- `cmd/stagehand/` → `cmd/stagecoach/`
- `pkg/stagehand/` → `pkg/stagecoach/`

### 1.4 File renames (inside renamed directories)
- `pkg/stagehand/stagehand.go` → `pkg/stagecoach/stagecoach.go`
- `pkg/stagehand/stagehand_test.go` → `pkg/stagecoach/stagecoach_test.go`
- `cmd/stagehand/main.go` → `cmd/stagecoach/main.go`

### 1.5 Package declarations
- `pkg/stagehand/stagehand.go:7`: `package stagehand` → `package stagecoach`
- `pkg/stagehand/stagehand_test.go:1`: `package stagehand` → `package stagecoach`

### 1.6 Build commands in test code
- `internal/stubtest/stubtest.go:59`: `"github.com/dustin/stagehand/cmd/stubagent"` → `stagecoach`
- `internal/signal/signal_integration_test.go:143`: same import path for stubagent
- `internal/signal/signal_integration_test.go:167`: `"github.com/dustin/stagehand/cmd/stagehand"` → `stagecoach`
- `internal/cmd/default_action_test.go`: binary build paths
- `internal/e2e/harness_test.go`: binary build paths

**Verification gate:** `go build ./...` and `go vet ./...` must pass after this layer.

---

## Layer 2: Configuration Surface (Runtime Contract)

### 2.1 Environment variable prefix
The env var prefix `STAGEHAND_` is hardcoded as literal strings (NOT a single constant). Files:
- `internal/config/load.go` — ~30+ occurrences of `"STAGEHAND_PROVIDER"`, `"STAGEHAND_MODEL"`, `"STAGEHAND_REASONING"`, `"STAGEHAND_TIMEOUT"`, `"STAGEHAND_VERBOSE"`, `"STAGEHAND_NO_COLOR"`, `"STAGEHAND_CONFIG"`, `"STAGEHAND_COMMITS"`, `"STAGEHAND_FORMAT"`, `"STAGEHAND_LOCALE"`, `"STAGEHAND_TEMPLATE"`, `"STAGEHAND_PUSH"`, `"STAGEHAND_STUB_OUT"`, plus the per-role prefix `"STAGEHAND_" + strings.ToUpper(role)`
- `internal/cmd/root.go` — flag help text: `STAGEHAND_PROVIDER`, `STAGEHAND_MODEL`, etc.
- `internal/config/bootstrap.go` — template comments listing all env vars
- `internal/e2e/harness_test.go` — `STAGEHAND_RUN_REAL`
- All `*_test.go` files that set/unset env vars

**Pattern:** `STAGEHAND_` → `STAGECOACH_` (case-sensitive — uppercase only)

### 2.2 Git config keys
The git config section `stagehand.*` is hardcoded as literal strings in:
- `internal/config/git.go` — 26+ occurrences: `"stagehand.provider"`, `"stagehand.model"`, `"stagehand.timeout"`, `"stagehand.autoStageAll"`, `"stagehand.verbose"`, `"stagehand.format"`, `"stagehand.locale"`, `"stagehand.template"`, `"stagehand.push"`, `"stagehand.noVerify"`, `"stagehand.maxDiffBytes"`, `"stagehand.maxMdLines"`, `"stagehand.tokenLimit"`, `"stagehand.diffContext"`, `"stagehand.maxDuplicateRetries"`, `"stagehand.subjectTargetChars"`, `"stagehand.output"`, `"stagehand.stripCodeFence"`, plus role keys `"stagehand.role.planner.provider"` etc.
- `internal/config/load_test.go` — test assertions using `"stagehand.provider"`, `"stagehand.timeout"`, etc.
- `internal/cmd/root.go` — flag help text referencing `git stagehand.provider`
- `internal/config/bootstrap.go` — template comments

**Pattern:** `"stagehand.` → `"stagecoach.` (lowercase, in string literals)

### 2.3 Config file paths
- `internal/config/file.go:102`: `filepath.Join(xdg, "stagehand", "config.toml")` → `"stagecoach"`
- `internal/config/file.go:108`: `filepath.Join(home, ".config", "stagehand", "config.toml")` → `"stagecoach"`
- `internal/config/file.go:127`: `func repoLocalConfigPath() string { return ".stagehand.toml" }` → `".stagecoach.toml"`
- `internal/config/file.go:491`: error message referencing `.stagehand.toml`
- `internal/config/bootstrap.go:244,246`: template comments referencing `.stagehand.toml`
- `internal/config/load.go:109,118`: comments referencing paths
- Tests in `file_test.go`, `load_test.go`: path assertions

### 2.4 Lock directory path
- `internal/lock/lock.go:222,225,231`: `filepath.Join(xdg, "stagehand", "locks")` and `filepath.Join(home, ".cache", "stagehand", "locks")` → `"stagecoach"`
- `internal/lock/lock_test.go`: path assertions

### 2.5 Exclusion file constant
- `internal/exclude/exclude.go:27-28`: `const StagehandIgnoreFile = ".stagehandignore"` → `StagecoachIgnoreFile = ".stagecoachignore"`
- `internal/exclude/exclude.go:65,70,75,85,93`: function/variable names referencing `StagehandIgnore` / `LoadStagehandIgnore`
- `internal/cmd/root.go:164`: help text referencing `.stagehandignore`
- `internal/ui/verbose.go:101`: warning text referencing `.stagehandignore`
- All `exclude_test.go` references

---

## Layer 3: User-Facing CLI Surface

### 3.1 Root command name
- `internal/cmd/root.go:121`: `Use: "stagehand"` → `Use: "stagecoach"`

### 3.2 Error message prefixes
Throughout `internal/`:
- `"stagehand: ..."` prefix in error/errorf strings (~20+ occurrences)
- Files: `config/migrate.go`, `config/load.go`, `config/file.go`, `config/git.go`, `generate/generate.go`, `generate/finalize.go`, `cmd/providers.go`, `cmd/root.go`, etc.

**Pattern:** `"stagehand: ` → `"stagecoach: ` and `stagehand: ` in format strings

### 3.3 Progress / status messages
- `internal/ui/output.go`, `verbose.go` — progress messages
- `internal/generate/generate.go` — progress output

### 3.4 Session ID prefix
- `internal/generate/multiturn.go:206-208`: `"stagehand-%d"` and `"stagehand-" + hex...` → `"stagecoach-..."`

### 3.5 Temp directory prefix
- `internal/stubtest/stubtest.go:49`: `"stagehand-stubagent-*"` → `"stagecoach-stubagent-*"`

### 3.6 Hook script (internal/hook/)
- `internal/hook/script.go:14`: `const Marker = "# stagehand prepare-commit-msg hook v1"` → `"# stagecoach prepare-commit-msg hook v1"`
- `internal/hook/script.go:35`: `run := `exec stagehand hook exec "$@"`` → `exec stagecoach hook exec "$@"`
- `internal/hook/script.go:37`: strict variant
- `internal/hook/hook.go:22`: `StatusStagehand` → `StatusStagecoach` (Go identifier)
- `internal/hook/hook.go:30`: `return "stagehand (v1)"` → `"stagecoach (v1)"`
- `internal/hook/hook.go:41`: `ErrNoHook = errors.New("no stagehand prepare-commit-msg hook...")` → `stagecoach`
- `internal/hook/hook.go:120,122`: script content with `exec stagehand hook exec`

### 3.7 Integrate: git alias defaults
- `internal/cmd/integrate_gitalias.go:17`: `defaultAliasName = "stagehand"` → `"stagecoach"`
- `internal/cmd/integrate_gitalias.go:18`: `stagehandAliasValue = "!stagehand"` → `stagecoachAliasValue = "!stagecoach"` (identifier + value)
- `internal/cmd/integrate_gitalias.go:25,28,30,36,39,60,123,128,129,131,146,175`: all references

### 3.8 Integrate: lazygit marker
- `internal/cmd/integrate_lazygit.go:20`: `lazygitMarker = "stagehand-integration"` → `"stagecoach-integration"`

### 3.9 Bootstrap config template
- `internal/config/bootstrap.go:236-269`: The entire `bootstrapHeader` constant — every `Stagehand`/`stagehand`/`STAGEHAND`/`.stagehand.toml` reference in the generated config template comments

### 3.10 CLI help text
- `internal/cmd/root.go:153-180+`: All flag descriptions referencing `STAGEHAND_*`, `git stagehand.*`, etc.
- `internal/cmd/config.go`, `providers.go`, `hook.go`, `hookexec.go`, `models.go`, `integrate.go` — Long descriptions

### 3.11 Exit code / lock messages
- `internal/exitcode/exitcode.go`: comment referencing stagehand
- `internal/lock/lock.go:51,60`: error messages

---

## Layer 4: Build, Release & CI

### 4.1 Makefile
- Binary name: `stagehand` → `stagecoach` (lines 6-9, 28-30, 52-65)
- MAIN_PKG: `./cmd/stagehand` → `./cmd/stagecoach` (line 30)
- Build target output: `./bin/stagehand` → `./bin/stagecoach`
- Install-test: `stagehand-test` → `stagecoach-test`
- All `@echo` messages

### 4.2 .goreleaser.yaml
- `project_name: stagehand` → `stagecoach` (line 12)
- Build ID: `stagehand` → `stagecoach` (line 20)
- `main: ./cmd/stagehand` → `./cmd/stagecoach` (line 21)
- `binary: stagehand` → `stagecoach` (line 22)
- Archive refs: `stagehand` (line 39)
- Homebrew formula name + tap refs (lines 69-80)
- Scoop name + bucket refs (lines 85-92)
- AUR name (lines 98-103)
- URLs: `github.com/dustin/stagehand` → `github.com/dustin/stagecoach` (lines 80, 92, 95)
- Comments referencing stagehand (lines 5-8)

### 4.3 .github/workflows/ci.yml
- Lines 102-105: `github.com/dustin/stagehand/internal/git` etc. → `stagecoach`

### 4.4 .gitignore
- Line 4: `/stagehand` → `/stagecoach`
- Line 21-22: `.stagehand.toml` → `.stagecoach.toml`

### 4.5 bin/ artifacts
- `bin/stagehand`, `bin/stagehand-test` — regenerated on build, but old artifacts should be cleaned

---

## Layer 5: Documentation

### 5.1 README.md (~12,700+ occurrences of "Stagehand"/"stagehand")
- Title, hero text, all sections, install instructions, examples, badge URLs
- Badge URL: `github.com/dustin/stagehand/actions/...` → `stagecoach`

### 5.2 docs/ (5 files)
- `docs/README.md`: title and description
- `docs/cli.md`: comprehensive CLI reference
- `docs/configuration.md`: config reference
- `docs/how-it-works.md`: architecture explanation
- `docs/providers.md`: provider documentation

### 5.3 providers/*.toml (8 files)
- Comments referencing stagehand in each manifest file

### 5.4 FUTURE_SPEC.md
- Any references to stagehand

---

## Execution Order (Critical Path)

```
Layer 1 (Go module + imports + dirs + files + packages)
    ↓ [go build ./... passes]
Layer 2 (env vars + git config + paths + lock dir + exclude file)
    ↓ [go test ./... passes]
Layer 3 (CLI strings + hook script + integrate + bootstrap + help)
    ↓ [go test ./... passes, e2e smoke test]
Layer 4 (Makefile + goreleaser + CI + gitignore + bin cleanup)
    ↓ [make build works]
Layer 5 (README + docs + provider tomls)
    ↓ [grep -ri stagehand returns ZERO hits]
```

## Verification Gates

1. After Layer 1: `go build ./... && go vet ./...`
2. After Layer 2: `go test ./internal/config/... ./internal/exclude/... ./internal/lock/...`
3. After Layer 3: `go test ./...` (full suite)
4. After Layer 4: `make build` produces `./bin/stagecoach`
5. After Layer 5: `grep -ri 'stagehand' --include='*.go' --include='*.md' --include='*.toml' --include='*.yaml' --include='*.yml' --include='Makefile' . | grep -v '.git/' | wc -l` == 0
