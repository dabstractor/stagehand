# Config Precedence Gap: `no_verify` Git-Config Layer (Issue 3)

## The Bug

`NoVerify` is documented (`config.go`, `cli.md`, `configuration.md`) as a **full 5-layer
precedence** field: `--no-verify` / `STAGECOACH_NO_VERIFY` / `stagecoach.no_verify` /
`[generation].no_verify`. **Three of four layers are wired** (flag, env, file). The
**git-config layer is completely missing** — `loadGitConfig()` in `internal/config/git.go`
never queries any `stagecoach.no*` key (grep for `no_verify`/`noVerify`/`NoVerify` returns 0 matches).

Two compounding problems:
1. **Missing reader** — no `gitConfigBool(... "stagecoach.<key>")` call for NoVerify.
2. **Invalid documented key name** — docs say `stagecoach.no_verify` (snake_case) but git rejects
   underscores in the final config key segment.

## Key Files

| File | Lines | Role |
|------|-------|------|
| `internal/config/config.go` | 128-134 | `NoVerify` field + doc comment (references `stagecoach.no_verify`) |
| `internal/config/git.go` | 106-260 | `loadGitConfig()` — missing NoVerify reader. `push` reader at 174 is the template |
| `internal/config/load.go` | 309-315 | env layer (DIRECT set — correct) |
| `internal/config/load.go` | 444-447 | flag layer (DIRECT set — correct) |
| `internal/config/file.go` | 67, 290-292, 351-353 | file layer (only-true-propagates — correct) |
| `docs/cli.md` | 44 | Git-config column = `stagecoach.no_verify` (invalid) |
| `docs/configuration.md` | 117, 149, 155 | References `stagecoach.no_verify` (invalid) |
| `internal/config/git_test.go` | 54-131, 165-191 | Test templates |
| `internal/config/load_test.go` | 1389-1429 | Push precedence test template |

## The Fix (source change: 4 lines)

Add to `loadGitConfig()` in `internal/config/git.go`, right after the `push` reader at line ~177:

```go
// §9.25 FR-V5 — noVerify via git config (camelCase multi-word key — mirrors push; only-true-
// propagates via overlay, same documented limitation as the file layer).
if v, found, err := gitConfigBool(repoDir, "stagecoach.noVerify"); err != nil { // camelCase!
    return nil, err
} else if found {
    c.NoVerify = v
}
```

**Key MUST be `stagecoach.noVerify`** (camelCase). Git rejects underscores in the final segment:
- `stagecoach.no_verify` → `error: invalid key: stagecoach.no_verify` (INVALID)
- `stagecoach.noVerify` → valid (matches existing convention: `autoStageAll`, `maxDiffBytes`, etc.)

## Doc Corrections

### `docs/cli.md:44`
```
| `--no-verify` | bool | false | `STAGECOACH_NO_VERIFY` | `stagecoach.no_verify` |
```
→ Change to `stagecoach.noVerify`

### `docs/configuration.md:155`
```
> ... full 5-layer precedence (`--no-verify` / `STAGECOACH_NO_VERIFY` / `stagecoach.no_verify` / `[generation].no_verify`).
```
→ Change `stagecoach.no_verify` → `stagecoach.noVerify`. The `[generation].no_verify` TOML key stays snake_case (TOML permits underscores).

### `internal/config/config.go:130`
```
// Full 5-layer precedence: --no-verify / STAGECOACH_NO_VERIFY / stagecoach.no_verify / [generation].no_verify,
```
→ Change `stagecoach.no_verify` → `stagecoach.noVerify`

### `docs/configuration.md` git-config keys table
Add a row for `stagecoach.noVerify` (camelCase, `--bool`, FR-V5 description) if such a table exists.

## Architecture: How the Layers Connect

```
Load() (load.go:~68)
 ├─ Layer 1: Defaults()                      → NoVerify = false
 ├─ Layer 2: global TOML  → overlay(&cfg, g) → only-true-propagates (file.go:351)
 ├─ Layer 3: repo TOML    → overlay(&cfg, r) → only-true-propagates (file.go:351)
 ├─ Layer 4: git config   → overlay(&cfg, gc)← loadGitConfig() [BUG: no noVerify reader]
 ├─ Layer 5: STAGECOACH_* env (loadEnv)       → DIRECT set (load.go:315)
 └─ Layer 7: CLI flags (loadFlags)           → DIRECT set (load.go:447)
```

`overlay()` (file.go) already contains `if src.NoVerify { dst.NoVerify = true }` (file.go:352).
**The ONLY missing code is the reader inside `loadGitConfig()`** — overlay already handles NoVerify.

## Existing Patterns (mirror for new tests)

### `git_test.go` — unit test for new reader
`TestLoadGitConfig_ReadsValues` (git_test.go:54-131) sets `stagecoach.push = "true"` and asserts `cfg.Push == true`.
Add `setGitConfig(t, repo, "stagecoach.noVerify", "true")` and assert `cfg.NoVerify == true`.

### `load_test.go` — full-precedence test (ABSENT for NoVerify)
`TestLoad_PushPrecedence` (load_test.go:1389-1429) is the template.
Add `TestLoad_NoVerifyPrecedence` proving the git-config value reaches the resolved Config and env overrides it.

## Constraints
- **camelCase key is mandatory** (`stagecoach.noVerify`) — git rejects underscores. This is a hard git constraint.
- **overlay only-true-propagates**: `git config stagecoach.noVerify false` is a no-op through overlay. Force-false via env/flag.
- **TOML file key stays `no_verify`** (snake_case) — TOML allows underscores. Do not change the struct tag.
- **No validation needed** — NoVerify is a plain bool with no range/format constraint.
- **overlay(), materialize(), loadEnv, loadFlags, config.go struct tag are all correct** — do NOT touch them.
