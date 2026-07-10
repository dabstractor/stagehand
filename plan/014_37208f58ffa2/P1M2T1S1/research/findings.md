# Research: P1.M2.T1.S1 — Add `NoParentWatchdog` config field (7-layer precedence, FR-K6)

Add a new `NoParentWatchdog bool toml:"no_parent_watchdog"` config field by **exactly mirroring the
existing `NoVerify` field** at all 7 precedence touch points. This is the FR-K6 escape hatch for the
parent-death watchdog (the watchdog itself + its consumer land in P1.M2.T2). Default **false**
(watchdog runs by default). NO CLI flag (FR-K6 lists only env + git-config + file).

All claims verified against current source. Line numbers are advisory (the file may drift slightly
as parallel tasks land — locate edits by content/grep or by anchoring next to the `NoVerify` sibling).

---

## 0. THE TEMPLATE — `NoVerify` at all 7 touch points (exact, verified)

`NoVerify` is the verbatim copy source. The new field is identical except for the name.

| # | Touch point | File:line | NoVerify (copy this) | NoParentWatchdog (new) |
|---|-------------|-----------|----------------------|------------------------|
| 1 | Config struct field + doc | config.go:136-142 | `NoVerify bool \`toml:"no_verify"\`` (+ 6-line doc comment) | `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` (+ doc comment citing §9.27 FR-K6) |
| 2 | Defaults() | config.go:214 | `NoVerify: false,` | `NoParentWatchdog: false,` |
| 3 | fileGeneration struct | file.go:68 | `NoVerify bool \`toml:"no_verify"\`  // §9.25 FR-V5 …` | `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`  // §9.27 FR-K6 …` |
| 4 | materialize() | file.go:298-300 | `if g.NoVerify { c.NoVerify = true }` | `if g.NoParentWatchdog { c.NoParentWatchdog = true }` |
| 5 | overlay() | file.go:362-364 | `if src.NoVerify { dst.NoVerify = true }` | `if src.NoParentWatchdog { dst.NoParentWatchdog = true }` |
| 6 | loadEnv() | load.go:317-326 | `STAGECOACH_NO_VERIFY` DIRECT set | `STAGECOACH_NO_PARENT_WATCHDOG` DIRECT set |
| 7 | loadGitConfig() | git.go:180-186 | `stagecoach.noVerify` (camelCase) | `stagecoach.noParentWatchdog` (camelCase) |

**Placement strategy**: anchor every edit IMMEDIATELY ADJACENT to its `NoVerify` sibling (same struct,
same function, same block). That keeps the diff reviewable and survives line-number drift. Do NOT
reorder existing fields.

---

## 1. The 7 touch points — exact current code

### (1) Config struct field — `internal/config/config.go:136-142`
```go
// NoVerify is the §9.25 FR-V5 --no-verify hook bypass (mirrors `git commit --no-verify`).
// When true, skips pre-commit and commit-msg hooks (prepare-commit-msg and post-commit still run).
// Full 5-layer precedence: --no-verify / STAGECOACH_NO_VERIFY / stagecoach.noVerify / [generation].no_verify,
// default false — hooks run by default; --no-verify is the deliberate exception. FILE LAYER LIMITATION
// (same as Push): only-true-propagates — a file setting `no_verify = false` is a no-op; the flag/env
// layers can set it false. See cmd root.go + hooks.RunCommitHooks (M3).
NoVerify bool `toml:"no_verify"`
```
**Add immediately after** (same struct, next field). Doc comment cites §9.27 FR-K6, notes default
false (watchdog runs by default), notes NO CLI flag (env + git + file only), notes only-true-propagates
file layer (mirrors NoVerify/Push).

### (2) Defaults() — `internal/config/config.go:214`
```go
NoVerify:             false,            // §9.25 FR-V5 default (hooks run by default)
```
**Add** `NoParentWatchdog: false,` (align with surrounding column style; comment `// §9.27 FR-K6 default (watchdog runs by default)`).

### (3) fileGeneration struct — `internal/config/file.go:68`
```go
NoVerify             bool     `toml:"no_verify"`         // §9.25 FR-V5 — only-true-propagates (mirrors Push)
HookTimeout          string   `toml:"hook_timeout"`      // §9.25 FR-V6 — duration string "10m", parsed in loadTOML
```
**Add** `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` — place it after `NoVerify`/`HookTimeout`
in the field list (keep gofmt column alignment). This is the `[generation]` TOML table struct.

> **TOML table note**: `fileGeneration` is the `[generation]` table (file.go:46). So `no_verify` and
> the new `no_parent_watchdog` are **`[generation]` keys in a config file** (NOT `[defaults]`). The
> bootstrap commented doc line therefore goes in the `generationCommented` block (see §3).

### (4) materialize() — `internal/config/file.go:298-300`
```go
// §9.25 FR-V5 — no_verify from file (only-true-propagates, mirrors Push — a file cannot set it false).
if g.NoVerify {
    c.NoVerify = true
}
```
**Add immediately after**:
```go
// §9.27 FR-K6 — no_parent_watchdog from file (only-true-propagates, mirrors NoVerify/Push).
if g.NoParentWatchdog {
    c.NoParentWatchdog = true
}
```

### (5) overlay() — `internal/config/file.go:362-364`
```go
// §9.25 FR-V5 — no_verify (only-true-propagates, same as Push)
if src.NoVerify {
    dst.NoVerify = true
}
```
**Add immediately after**:
```go
// §9.27 FR-K6 — no_parent_watchdog (only-true-propagates, same as NoVerify/Push)
if src.NoParentWatchdog {
    dst.NoParentWatchdog = true
}
```

### (6) loadEnv() — `internal/config/load.go:317-326`
```go
// §9.25 FR-V5 — no_verify via env (presence-semantic, DIRECT set — can be false, the escape hatch).
if v, ok := os.LookupEnv("STAGECOACH_NO_VERIFY"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_NO_VERIFY: %w", err)
    }
    cfg.NoVerify = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_PUSH)
}
```
**Add immediately after** (verbatim mirror; only the names change):
```go
// §9.27 FR-K6 — no_parent_watchdog via env (presence-semantic, DIRECT set — can be false, the escape hatch).
if v, ok := os.LookupEnv("STAGECOACH_NO_PARENT_WATCHDOG"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_NO_PARENT_WATCHDOG: %w", err)
    }
    cfg.NoParentWatchdog = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_NO_VERIFY)
}
```
No new import (os, strconv, fmt already in load.go).

### (7) loadGitConfig() — `internal/config/git.go:180-186`
```go
// §9.25 FR-V5 — noVerify via git config (camelCase key: git rejects underscores in the final segment,
// matching the autoStageAll/maxDiffBytes/stripCodeFence convention).
if v, found, err := gitConfigBool(repoDir, "stagecoach.noVerify"); err != nil {
    return nil, err
} else if found {
    c.NoVerify = v
}
```
**Add immediately after**:
```go
// §9.27 FR-K6 — noParentWatchdog via git config (camelCase key, same convention as noVerify).
if v, found, err := gitConfigBool(repoDir, "stagecoach.noParentWatchdog"); err != nil {
    return nil, err
} else if found {
    c.NoParentWatchdog = v
}
```

---

## 2. NAMING DECISIONS (confirmed vs the PRD's loose notation)

| Layer | Key | Why | Source |
|-------|-----|-----|--------|
| TOML struct tag / file key | `no_parent_watchdog` (snake_case) | Codebase convention: EVERY toml tag is snake_case (`no_verify`, `auto_stage_all`, …) | file.go tags; architecture note decision |
| Env var | `STAGECOACH_NO_PARENT_WATCHDOG` (ALL-CAPS) | Codebase convention: EVERY env var is `STAGECOACH_*` all-caps (`STAGECOACH_NO_VERIFY`) | load.go:320; architecture note decision #1 |
| Git config key | `stagecoach.noParentWatchdog` (camelCase) | git REJECTS underscores in the final key segment (`invalid key`); matches `noVerify`/`autoStageAll` | git.go:180 comment; architecture note decision #2 |

> The PRD writes `stagecoach_NO_PARENT_WATCHDOG` (FR-K6, line 569) and `noParentWatchdog` (§16.3
> example, line 1709) — both are **conceptual notation, not literal**. The PRD also writes
> `stagecoach_NO_VERIFY` for the EXISTING var, whose actual code is `STAGECOACH_NO_VERIFY`. **Follow
> the codebase, not the PRD prose.** (architecture note decisions #1/#2.)

---

## 3. CRITICAL GOTCHA — go-toml/v2 key matching (empirically verified)

`go.mod` uses `github.com/pelletier/go-toml/v2 v2.4.2`. Verified with a throwaway program: go-toml/v2
matches keys **case-INSENSITIVELY but WORD-SEPARATION-SENSITIVELY** (it lowercases both the incoming
key and the toml tag, then compares — underscores are significant).

```
TOML key                 lowercased key      lowercased tag (no_parent_watchdog)   decoded?
no_parent_watchdog=true  no_parent_watchdog  ==                                    → TRUE   ✓
noParentWatchdog  =true  noparentwatchdog    !=                                    → FALSE (silently dropped)
NoParentWatchdog  =true  noparentwatchdog    !=                                    → FALSE (silently dropped)
```

**Therefore any commented doc key in a generated config template MUST be snake_case
`no_parent_watchdog`** (matching the toml tag). The item description's literal `# noParentWatchdog =
false` (camelCase) is **WRONG** — if a user uncommented it, go-toml would silently ignore it and the
watchdog would NOT be disabled. ⚠️ Use snake_case in bootstrap.go. (This also contradicts PRD.md:1709.)

---

## 4. Bootstrap template doc additions (`internal/config/bootstrap.go`)

Two additions (PRD Mode A — docs ride with the work). The field is a `[generation]` key (fileGeneration),
so the commented-key doc goes in the `generationCommented` block.

### (a) env-var comment block — lines 251-262 (header at ~250)
After the `STAGECOACH_NO_COLOR` line (255) OR at the end of the list (after line 262 STAGECOACH_COMMITS),
add:
```
#   STAGECOACH_NO_PARENT_WATCHDOG=1   # opt out of the parent-death lock watchdog (§9.27 FR-K6)
```
(All-caps var, matching the surrounding `STAGECOACH_*` lines.)

### (b) generationCommented block — lines 293-305 (the `[generation]` commented keys)
Add (snake_case!):
```
# no_parent_watchdog    = false  # opt out of the parent-death lock watchdog — set true if you launch via nohup/setsid/systemd-run (§9.27 FR-K6)
```
Place it among the other commented `[generation]` keys (e.g. after `multi_turn_chunk_tokens` or
where the column alignment fits). Note this block does NOT currently list `no_verify`/`push`/
`hook_timeout` — that's fine; the item explicitly asks to document `no_parent_watchdog` here.

> **Bootstrap test brittleness (low)**: `internal/config/bootstrap_test.go` checks substrings +
> valid-TOML structure, NOT byte-exact on the generation block → adding a commented line is safe.
> (Contrast: `internal/cmd/config_test.go:438` byte-asserts a SEPARATE template
> `exampleConfigTemplate` in `internal/cmd/config.go` — do NOT touch that; out of scope for this
> item, defer to P1.M4.T2 docs sync.)

---

## 5. Tests to add (mirror the existing NoVerify tests)

| New test | Mirrors (file:line) | What it proves |
|----------|---------------------|----------------|
| `TestLoadEnv_NoParentWatchdog` (extend the big env test @151, or new func) | load_test.go:151 + assertion @171 | `STAGECOACH_NO_PARENT_WATCHDOG=true` → `cfg.NoParentWatchdog==true` |
| `TestLoadEnv_NoParentWatchdog_FalseEscape` (or extend BoolFalseEscape @189) | load_test.go:189-204 | `STAGECOACH_NO_PARENT_WATCHDOG=false` → `cfg.NoParentWatchdog==false` (DIRECT-set escape hatch) |
| git_test.go: add `setGitConfig(... "stagecoach.noParentWatchdog", "true")` + assertion | git_test.go:88-90, 134-136 | `stagecoach.noParentWatchdog=true` → `cfg.NoParentWatchdog==true` (camelCase git key) |
| `TestLoad_NoParentWatchdogPrecedence` | load_test.go:1686-1717 | git config=true, then `STAGECOACH_NO_PARENT_WATCHDOG=false` → `false` (env beats git, DIRECT-set escape) |

Idioms:
- loadEnv-level: `cfg := Defaults()` → `t.Setenv(...)` → `err := loadEnv(&cfg)`; assert `cfg.NoParentWatchdog`.
- git-level: `setGitConfig(t, repo, "stagecoach.noParentWatchdog", "true")` → `cfg, err := loadGitConfig(repo)`; assert.
- Load-level: `loadEnvSetup(t)` + `chdir(t, repo)` + `Load(ctx, LoadOpts{RepoDir: repo, DisableBootstrap: true})`.

**NO flag test** — there is no `--no-parent-watchdog` flag (do NOT mirror `TestLoadFlags_NoVerify`).

---

## 6. Scope boundaries (what NOT to do)

- **NO CLI flag** in `internal/cmd/root.go` and **NO loadFlags entry** in load.go:475-486. FR-K6 has no
  flag (env + git + file only). Do NOT add `BoolVarP` or `fs.Changed("no-parent-watchdog")`.
- **NO consumer wiring**. The watchdog arming (`if !cfg.NoParentWatchdog { watchdog.Arm(...) }` in
  default_action.go) is P1.M2.T2.S2 — a later task. After this subtask the field has ZERO production
  readers (mirrors NoVerify's pre-consumer state).
- **NO migration / NO config-version bump**. Adding a default-false optional field is backward-
  compatible (scout-confirmed: migrate.go is field-specific, no enumeration; go-toml silently drops
  unknown keys; `CurrentConfigVersion` unchanged).
- **NO `internal/cmd/config.go` exampleConfigTemplate edit** — it has a byte-exact golden test
  (config_test.go:438) and is out of this item's scope (defer to P1.M4.T2).
- **NO README/docs** change — P1.M4.T2 owns the changeset-level docs.

---

## 7. Backward compatibility & enumeration (scout-confirmed)

- Field absent from all source today (zero `NoParentWatchdog`/`no_parent_watchdog` hits in *.go).
- `migrate.go`'s only migration (`migrateV2ToV3`) is field-specific (folds removed `default_provider`);
  no generic field list, no reflection, no switch-on-fields → no update needed.
- No `knownKeys`/allowlist/`reflect.`/field-count/strict-decode anywhere in internal/config/
  (`DisallowUnknownFields` has zero source hits; go-toml silently drops unknown keys).
- No field-count/complete-set test exists → no enumeration test to update.
- Old config files (without the key) decode fine (field stays false, the zero value). Old binaries
  reading a new file that sets the key just ignore it. Fully backward-compatible.

---

## 8. Parallel-execution coordination

Parallel sibling P1.M1.T2.S1 edits `internal/signal/*` only. P1.M1.T1.S1 also touches signal files.
**Neither touches `internal/config/*`** → no file overlap with this item. The config package is mine
alone at this time (plan_status: P1.M2.T2 watchdog, P1.M3 lock, P1.M4 docs are all different packages,
Planned/not-started). So no merge conflicts; still, anchor edits next to the `NoVerify` sibling (not
by line number) to absorb any incidental drift.

---

## 9. Validation commands (Makefile)

- Build (native): `go build ./...`
- Cross-build (field is plain bool, no platform tag, but confirm): `GOOS=windows go build ./...` / `GOOS=linux`
- Vet: `go vet ./internal/config/...`
- Format: `gofmt -l internal/config/*.go` (must be empty)
- Focused tests: `go test ./internal/config/ -run 'NoParentWatchdog' -v` and `-run 'Verbose|NoVerify|Precedence'`
- Full suite (race): `make test`
- Lint: `make lint` (golangci-lint v1.61: staticcheck/gosimple/govet/errcheck/ineffassign/unused)
- Coverage gate: `make coverage-gate` (≥85% on internal/{git,provider,generate,config})
- Bootstrap template validity: `go test ./internal/config/ -run 'Bootstrap' -v` (valid-TOML + substring)
