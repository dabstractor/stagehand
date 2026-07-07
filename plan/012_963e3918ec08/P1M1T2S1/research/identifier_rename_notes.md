# Research: Go Identifier Rename (P1.M1.T2.S1)

> **Purpose:** Pin the exact edits for renaming 8 Go identifiers/values containing "stagehand" → "stagecoach",
> checked against the live codebase on 2026-07-07. Module path already `github.com/dustin/stagecoach` (S1-S3 done).
> Baseline `go build ./...` passes. Prior PRP (S4) explicitly defers identifiers to THIS task → no conflict.

---

## 1. The 8 Renames + Their Reference Sites (verified via grep)

### Identifier renames (name changes):

| # | Old → New | Type | Decl File | Ref Sites |
|---|---|---|---|---|
| 1 | `StagehandIgnoreFile` → `StagecoachIgnoreFile` | const | exclude.go:28 | exclude.go:70,75; exclude_test.go (refs) |
| 2 | `LoadStagehandIgnore` → `LoadStagecoachIgnore` | func | exclude.go:69 | exclude.go:99; exclude_test.go:52 |
| 3 | `StatusStagehand` → `StatusStagecoach` | enum | hook.go:22 | hook.go:29,56,99; hook_test.go:36-37; cmd/hook.go:117 |
| 4 | `stagehandAliasValue` → `stagecoachAliasValue` | const | gitalias.go:18 | gitalias.go:129,146 |
| 5 | `buildStagehand` → `buildStagecoach` + vars | func+vars | signal_integration_test.go:152-173 | :30,54,160,171,173 |

### Const-value changes (name stays, value changes):

| # | Const | Old Value → New Value | File | Affected |
|---|---|---|---|---|
| 6 | `defaultAliasName` | `"stagehand"` → `"stagecoach"` | gitalias.go:17 | gitalias_test.go (if it checks the alias name string) |
| 7 | `lazygitMarker` | `"stagehand-integration"` → `"stagecoach-integration"` | integrate_lazygit.go:20 | entryTpl at :28; integrate_lazygit_test.go (~15 assertions) |
| 8 | `Marker` | `"# stagehand prepare-commit-msg hook v1"` → `"# stagecoach prepare-commit-msg hook v1"` | hook/script.go:14 | hook_test.go (if it checks marker string content) |

### Item 4 also has a VALUE change:
`stagehandAliasValue = "!stagehand"` → `stagecoachAliasValue = "!stagecoach"` (both name AND value change — the command part follows `defaultAliasName`).

### Item 5 related identifiers:
- `buildStagehandOnce` → `buildStagecoachOnce` (:154)
- `buildStagehandPath` → `buildStagecoachPath` (:155, :171, :173)
- `stagehandBin` → `stagecoachBin` (:30, :54)

### Item 1 also has a VALUE change (per §2.5):
`StagehandIgnoreFile = ".stagehandignore"` → `StagecoachIgnoreFile = ".stagecoachignore"` (both name AND value change).

---

## 2. Scope Boundary (CRITICAL)

**THIS task owns:** the 8 listed identifiers + their const values + ALL reference sites (including test files that reference the renamed identifiers/values).

**NOT this task (explicitly):**
- `stagehandFlagUsages` (cobra template func) → P1.M1.T2.S2
- `STAGEHAND_*` env var literals → P1.M2.T1
- `stagehand.*` git config keys → P1.M2.T1
- `.stagehandignore`/`.stagehand.toml` string literals in root.go/verbose.go (NOT via the const) → P1.M2.T2
- User-facing strings in cmd/hook.go ("Remove the stagehand prepare-commit-msg hook") → P1.M2.T3
- Temp dir prefixes (`stagehand-stubagent-*`) → P1.M2.T3
- Comments (non-functional) → optional cleanup, M4/M5

---

## 3. The Files Touched (per identifier)

| File | Changes |
|---|---|
| `internal/exclude/exclude.go` | Items 1,2: const name+value, func name, all refs |
| `internal/exclude/exclude_test.go` | Items 1,2: test func name + refs |
| `internal/hook/hook.go` | Item 3: enum name + refs |
| `internal/hook/hook_test.go` | Item 3: assertion refs |
| `internal/cmd/hook.go` | Item 3: `hook.StatusStagehand` → `hook.StatusStagecoach` |
| `internal/hook/script.go` | Item 8: Marker value |
| `internal/cmd/integrate_gitalias.go` | Items 4,6: const name+value, defaultAliasName value |
| `internal/cmd/integrate_gitalias_test.go` | Item 6: if it checks the alias name string |
| `internal/cmd/integrate_lazygit.go` | Item 7: lazygitMarker value + entryTpl format string |
| `internal/cmd/integrate_lazygit_test.go` | Item 7: ~15 `"stagehand-integration"` assertions → `"stagecoach-integration"` |
| `internal/signal/signal_integration_test.go` | Item 5: func name + vars + refs |

---

## 4. Decisions Log

| # | Decision | Rationale |
|---|---|---|
| D1 | Items 1+1value: rename BOTH name AND value | §2.5 explicitly: `StagecoachIgnoreFile = ".stagecoachignore"` |
| D2 | Item 4+4value: rename BOTH name AND value | `"!stagehand"` → `"!stagecoach"` — the command part follows the binary name |
| D3 | Items 5: rename `buildStagehand` + `buildStagehandOnce` + `buildStagehandPath` + `stagehandBin` | All contain "Stagehand"/"stagehand" in the identifier name |
| D4 | Item 7: `entryTpl` format string at integrate_lazygit.go:28 also changes | It contains `"# stagehand-integration"` as a literal — must change to `"# stagecoach-integration"` |
| D5 | Item 7: ALL ~15 test assertions in integrate_lazygit_test.go change | They check for the marker string; if the value changes, the assertions must follow |
| D6 | Item 8: check hook_test.go for marker-string assertions | If any test checks `strings.Contains(..., "# stagehand ...")`, it must change |
| D7 | Comments: optional but encouraged | The contract says "non-comment" — comments are exempt from the success check, but renaming them is good hygiene |
