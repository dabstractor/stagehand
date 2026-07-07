# Critical Findings

## F1: This is a rename task, NOT a greenfield build

The entire stagecoach feature set described in the PRD is **already implemented** under the name "stagehand". The task is purely a mechanical rename from "stagehand" → "stagecoach" across all tracked files. No new features need to be built.

**Evidence:**
- 63,008 lines of Go across ~200 files
- `go build ./cmd/stagehand/` succeeds (exit 0)
- Git remote is already `dabstractor/stagecoach` (renamed on GitHub)
- PRD content itself already uses "stagecoach" throughout
- The PRD's final note: "this project was originally named 'stagehand' and has been renamed"

## F2: No backward compatibility needed

The project has no public releases. The goreleaser config comment states "before the first REAL tag." The rename can be clean and atomic — no migration paths, no dual-name support, no deprecation warnings.

## F3: 132 functional files + 622 historical plan files need renaming

| Category | Files | Priority |
|----------|-------|----------|
| Go source (internal/, cmd/, pkg/) | ~105 | P0 — tool breaks without this |
| Docs (README.md, docs/*.md) | ~6 | P0 — user-facing |
| Build (Makefile, .goreleaser.yaml, .github/) | ~3 | P0 — CI/release breaks |
| Config (.gitignore) | 1 | P0 |
| Provider manifests (providers/*.toml) | 8 | P1 — comments only |
| FUTURE_SPEC.md | 1 | P1 |
| Historical plan/ artifacts | ~622 | P2 — cosmetic only |

## F4: The env var prefix is NOT a single constant

`STAGEHAND_` appears as ~30+ hardcoded literal strings in `internal/config/load.go` and throughout `internal/cmd/root.go` flag help text. There is no centralized `EnvPrefix` constant. A global sed is the safest approach, but each replacement must be verified to avoid false positives (e.g., a variable named `stagehandFlagUsages` in cobra template functions).

**Key identifiers that contain "stagehand" but are NOT just the env prefix:**
- `StagehandIgnoreFile` (const, `internal/exclude/exclude.go`)
- `StatusStagehand` (enum, `internal/hook/hook.go`)
- `stagehandAliasValue` (const, `internal/cmd/integrate_gitalias.go`)
- `defaultAliasName = "stagehand"` (const, same file)
- `lazygitMarker = "stagehand-integration"` (const, `internal/cmd/integrate_lazygit.go`)
- `Marker = "# stagehand prepare-commit-msg hook v1"` (const, `internal/hook/script.go`)
- `stagehandFlagUsages` (cobra template func, `internal/cmd/root.go`)

## F5: Go module rename requires coordinated sequencing

The safest order is:
1. `go mod edit -module github.com/dustin/stagecoach`
2. `sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g'` across all `.go` files
3. `git mv` directories: `cmd/stagehand/` → `cmd/stagecoach/`, `pkg/stagehand/` → `pkg/stagecoach/`
4. `git mv` files within those directories
5. Rename `package stagehand` → `package stagecoach` declarations
6. `go build ./...` to verify

Steps 1-2 can be combined (sed the go.mod too, or use `go mod edit`). Steps 3-5 must happen in order (dirs before files, dirs before package checks).

## F6: The hook script marker is a stability contract

`internal/hook/script.go:14`: `const Marker = "# stagehand prepare-commit-msg hook v1"`

This marker is used by `hook status` and `hook uninstall` to identify stagecoach-owned hooks. Renaming it to `"# stagecoach prepare-commit-msg hook v1"` means:
- Any **already-installed** hooks with the old marker will be treated as "foreign" (not ours)
- This is acceptable since there are no public releases (F2)

## F7: Session ID and temp-dir prefixes are cosmetic

- `internal/generate/multiturn.go:206-208`: Session IDs are prefixed with `"stagehand-"` — purely diagnostic, never parsed by the tool. Safe to rename.
- `internal/stubtest/stubtest.go:49`: Temp dir prefix `"stagehand-stubagent-*"` — cosmetic, but should be renamed for consistency.

## F8: Historical plan/ files are tracked but non-functional

The `plan/001_*` through `plan/011_*` directories contain previous task breakdowns, PRP files, and architecture research from building the tool. They reference "stagehand" extensively but are never compiled, never executed, and never shipped to users. They should be renamed for completeness but are the lowest priority.

## F9: README.md badge URL references the old GitHub path

`README.md` contains badge URLs like `https://github.com/dustin/stagehand/actions/workflows/ci.yml/badge.svg`. The git remote is `dabstractor/stagecoach` (not `dustin/stagecoach`), so these URLs need updating to match the actual repo. The goreleaser config also references `dustin/stagehand` — these should become `dabstractor/stagecoach` (matching the actual remote) or the eventual canonical GitHub path.

## F10: .gitignore has stagehand-specific entries

```
/stagehand          → /stagecoach
.stagehand.toml     → .stagecoach.toml
```

The `/stubagent` entry stays unchanged (it's the test stub binary, not named after the project).
