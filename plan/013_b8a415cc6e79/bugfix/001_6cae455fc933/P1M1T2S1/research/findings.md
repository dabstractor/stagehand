# Research: P1.M1.T2.S1 — STAGECOACH_AUTO_STAGE_ALL & STAGECOACH_MULTI_TURN_FALLBACK env cases

**Scope**: Add two DIRECT-set env cases to `loadEnv` (internal/config/load.go) + unit tests + two
docs rows. All line numbers verified against the working tree (2026-07-09). S1 (*bool conversion)
is ALREADY APPLIED in the working tree — the fields are `*bool` with accessors and `boolPtr` exists.

## 1. CONFIRMED: load.go is greenfield for these two env vars

`grep -rn 'STAGECOACH_AUTO_STAGE_ALL|STAGECOACH_MULTI_TURN_FALLBACK' internal/ cmd/ docs/` → EMPTY.
S1's PRP explicitly fenced this off ("load.go env/flag layers are UNAFFECTED... adding env vars is
sibling task T2"). So this task adds them fresh.

## 2. The pattern to mirror (load.go bool DIRECT-set block)

`loadEnv` (load.go:229-324) bool DIRECT-set blocks use this exact shape (STAGECOACH_PUSH @301,
STAGECOACH_NO_VERIFY @310):

```go
if v, ok := os.LookupEnv("STAGECOACH_PUSH"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_PUSH: %w", err)
    }
    cfg.Push = b // DIRECT set — can be false (escape hatch)
}
```

### CRITICAL DISTINCTION (the one thing that differs from PUSH/NO_VERIFY)
`Push` and `NoVerify` are PLAIN `bool`, so they write `cfg.Push = b`.
`AutoStageAll` and `MultiTurnFallback` are `*bool` (post-S1), so they MUST write `cfg.AutoStageAll = boolPtr(b)`.
A non-nil pointer (including `*false`) is the explicit-override signal the *bool overlay relies on;
writing a plain `cfg.AutoStageAll = b` would NOT COMPILE (pointer vs bool). The error wrapper MUST be
`return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err)` (and the multi-turn variant) — same shape
as the other bool blocks.

## 3. Exact insertion point in load.go

- STAGECOACH_NO_VERIFY block: lines 310-315 (`cfg.NoVerify = b` @315).
- STAGECOACH_WORK_DESCRIPTION block (a string var): starts @320.
- `return nil` @324.
- **Insert the two new bool blocks between line 315 and line 320** — grouping all the bool
  DIRECT-set env vars (PUSH @301, NO_VERIFY @310, then AUTO_STAGE_ALL + MULTI_TURN_FALLBACK) together,
  before the string WORK_DESCRIPTION var. The item says "near the STAGECOACH_PUSH block" — this is
  the natural placement (bool DIRECT-set group).

## 4. Post-S1 field/accessor state (config.go — already landed)

- `boolPtr` helper: config.go:7 (`func boolPtr(b bool) *bool { return &b }`) — UNEXPORTED (package config only; fine — load.go is package config).
- `AutoStageAll *bool` (config.go:69); `MultiTurnFallback *bool` (config.go:84).
- `Defaults()` seeds `AutoStageAll: boolPtr(true)` (config.go:189), `MultiTurnFallback: boolPtr(true)` (config.go:199).
- Accessors: `AutoStageAllValue()` (config.go:241-246), `MultiTurnFallbackValue()` (config.go:253-258) — nil⇒true default, non-nil⇒dereference. **Tests read these accessors**, never the raw `*bool`.
- The git layer already produces `*bool` (git.go:162 `c.AutoStageAll = boolPtr(v)`); there is NO
  `stagecoach.multiTurnFallback` git key (only autoStageAll). Env (layer 5) DIRECT-set *bool beats
  git (layer 4) and file (layers 2-3) and default (layer 1).

## 5. Test patterns to mirror (load_test.go — package config)

| Existing test | Line | What it proves | Mirror for |
|---|---|---|---|
| `TestLoadEnv_Push` | 1322 | true⇒true; false⇒DIRECT-escape false | `TestLoadEnv_AutoStageAll` / `TestLoadEnv_MultiTurnFallback` |
| `TestLoadEnv_BadBoolErrors` | 229 | invalid bool ⇒ error containing the var name | bad-bool error cases |
| `TestLoad_EnvBoolFalseEscape` | 716 | full `Load`: env DIRECT false overrides file true | precedence (env > file) |
| `TestLoad_EnvOverridesGit` | 601 | full `Load`: env overrides git-config | precedence (env > git) |

- Helpers available in load_test.go: `loadEnvSetup(t)` (returns globalDir, repo, cleanup),
  `chdir(t, dir)`, `setGitConfig(t, repo, key, val)`, `writeConfigFile(t, dir, name, body)`,
  `newFlagSet(t)`. `t.Setenv` (Go 1.17+) auto-cleans env — use it (no manual os.Unsetenv needed).
- Defaults() leaves both fields `boolPtr(true)` (non-nil), so the "unset ⇒ no-op" case asserts
  `AutoStageAllValue()==true` after `loadEnv(&cfg)` with no env set. The "false DIRECT escape" case
  starts from Defaults() and asserts the accessor flips to `false`.
- For the full-Load precedence test, mirror `TestLoad_EnvBoolFalseEscape` (716): write a config file
  with `[defaults]\nauto_stage_all = true\n` (or rely on the default-true), set
  `t.Setenv("STAGECOACH_AUTO_STAGE_ALL","false")`, call `Load(ctx, LoadOpts{RepoDir: repo})`, assert
  `cfg.AutoStageAllValue()==false`. This proves layer 5 (env *false) DIRECT-set beats layer 1-4.

## 6. Docs: env-var table (docs/configuration.md)

- The env-var table ENDS at line 199 with the `STAGECOACH_PUSH` row; line 201 begins `## Git-config keys`.
- **Add two rows between line 199 and line 201.** Exact content (per item description):
  - `STAGECOACH_AUTO_STAGE_ALL` | `--no-auto-stage` (inverse) | Auto-stage all when nothing staged (true=enable, false=disable) | `STAGECOACH_AUTO_STAGE_ALL=false stagecoach`
  - `STAGECOACH_MULTI_TURN_FALLBACK` | (no flag) | Enable lossless multi-turn fallback on large diffs (true=enable, false=disable) | `STAGECOACH_MULTI_TURN_FALLBACK=false stagecoach`
- `--no-auto-stage` flag CONFIRMED to exist (internal/cmd/root.go:170 `pf.BoolVar(&flagNoAutoStage, "no-auto-stage", false, ...)`). It is the per-invocation INVERSE; the env var is the persistent form.
- There is NO `--no-multi-turn` flag (grep confirms), so "(no flag)" is accurate.
- SCOPE FENCE: line 166 currently mis-states that multi_turn_fallback can be set "via
  `git config stagecoach.autoStageAll`-style *bool behavior" (there is no multiTurnFallback git key).
  Do NOT fix that here — it is Issue 2 / sibling P1.M1.T3.S1 (docs reconciliation). T2.S1 touches
  ONLY the two env-var table rows.

## 7. Validation commands (verified project conventions)

- `go build ./...` — compile (test files included).
- `go vet ./internal/config/...` — vet.
- `gofmt -l internal/config/load.go internal/config/load_test.go docs/configuration.md` (md not gofmt'd; check only .go).
- `go test ./internal/config/... -run 'LoadEnv_AutoStageAll|LoadEnv_MultiTurnFallback|Load_AutoStageAll_Env|Load_MultiTurnFallback_Env' -v`
- `make test` — full race suite (project standard).
- Grep guard: `grep -rn 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' internal/config/load.go` must show exactly the two new blocks.
