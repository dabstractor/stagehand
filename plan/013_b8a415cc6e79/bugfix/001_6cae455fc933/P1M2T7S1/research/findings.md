# P1.M2.T7.S1 — Research findings (consolidated)

Synthesis of the three scout reports (`readme_audit.md`, `canonical_model.md`,
`code_ground_truth.md`) plus direct repo-wide verification. Read-only; no code changed.

## Headline conclusion

The literal sweep target — `README.md` and overview docs (`docs/README.md`,
`FUTURE_SPEC.md`) — is **CLEAN**. Every earlier bugfix commit (9df1c66, 79f4dc2)
purged the snake_case git-config key from shipped markdown. The precedence ladder
at `README.md:264` is correct and matches `docs/configuration.md:5-20` rung-for-rung.

The **actual stale config-precedence references** that survive in the repo are
NOT in markdown — they are in two Go source files that emit the user-facing
`config init` template comment (explicitly part of Issue 2's scope that
P1.M1.T3.S1 addressed only in `docs/configuration.md`):

| File | Line | Stale text | Fix |
|------|------|-----------|-----|
| `internal/config/bootstrap.go` | 268 | `#   git config stagecoach.auto_stage_all true` | `#   git config stagecoach.autoStageAll true` |
| `internal/cmd/config.go` | 534 | `#   git config stagecoach.auto_stage_all true` | `#   git config stagecoach.autoStageAll true` |

These ship verbatim into every user's `~/.config/stagecoach/config.toml` via
`config init` (bootstrap.go `bootstrapHeader` → `buildBootstrapConfig` → write)
and `config init --template` (config.go `exampleConfigTemplate`, config.go:437).

## CRITICAL two-axis naming distinction (the core gotcha)

| Surface | Key | Case | Status |
|---------|-----|------|--------|
| TOML config-file field | `auto_stage_all` | snake_case | ✅ CORRECT — do NOT touch |
| TOML config-file field | `multi_turn_fallback` | snake_case | ✅ CORRECT — do NOT touch |
| git-config key | `stagecoach.autoStageAll` | camelCase | ✅ what code reads (git.go:159) |
| git-config key | `stagecoach.auto_stage_all` | snake_case | ❌ un-settable (git rejects `_` in name segment) — THE BUG |
| git-config key | `stagecoach.multiTurnFallback` | — | N/A — does NOT exist (no git key for multi_turn) |
| env var | `STAGECOACH_AUTO_STAGE_ALL` | UPPER_SNAKE | ✅ DIRECT `*bool` (load.go:321) |
| env var | `STAGECOACH_MULTI_TURN_FALLBACK` | UPPER_SNAKE | ✅ DIRECT `*bool` (load.go:332) |

`bootstrap.go:161` (`...# auto_stage_all = true...`) and `config.go:553`
(`# auto_stage_all = true`) are the **TOML field** — snake_case is correct; leave
them. Only the **git-config key** lines (268, 534) are wrong.

## Test-safety analysis (verified directly)

- `internal/config/bootstrap_test.go`: uses `strings.Contains` on specific values
  (config_version, provider names, role models). Does NOT pin the git-key line.
  Editing bootstrap.go:268 will not break it.
- `internal/cmd/config_test.go:438-439, 622-624`: compares the WRITTEN config to
  the `exampleConfigTemplate` **constant itself** with `!=`. Editing the constant
  changes BOTH sides of the comparison equally → tests still pass. Confirms
  `exampleConfigTemplate` is LIVE (config.go:437 `content = exampleConfigTemplate`).
- NO test currently asserts the git key is camelCase → recommend adding a
  regression assertion so the camelCase form cannot silently revert.

## Sweep result (repo-wide `grep -rn 'stagecoach\.auto_stage_all'`)

- Shipped markdown (README.md, docs/*): **ZERO** — clean.
- `plan/**` (PRDs, snapshots, research): present, but READ-ONLY / orchestrator-owned.
- `PRD.md:338` (FR36): present — **forbidden to modify** (PRD is human-owned).
- `.pi-subagents/artifacts/`: internal agent artifacts, not docs.
- Live Go source: **exactly two** (bootstrap.go:268, config.go:534) — the targets.

## Optional / residual (flagged, mostly out of strict scope)

1. README.md:67 — could mention multi_turn_fallback disable path; low value (README
   deliberately defers config detail to docs/). Defensible to leave.
2. Template comments (bootstrap.go + config.go) omit several env vars
   (`STAGECOACH_AUTO_STAGE_ALL`, `STAGECOACH_MULTI_TURN_FALLBACK`, `STAGECOACH_PUSH`,
   `STAGECOACH_NO_VERIFY`, `STAGECOACH_FORMAT`, `STAGECOACH_LOCALE`,
   `STAGECOACH_TEMPLATE`, `STAGECOACH_WORK_DESCRIPTION`). Optional completion
   within "sync changeset-level documentation" — broader; mark lower priority.
3. **CROSS-TASK residual (T4, NOT T7)**: `STAGECOACH_VERBOSE=2` "not yet supported"
   message appears NOT to have landed in code (`grep -rn 'not yet supported'` empty;
   load.go:246-251 still `ParseBool`s, erroring opaquely on "2"). No shipped doc
   claims VERBOSE=2 works or is unsupported, so T7 has nothing to fix on this axis.
   Flag for orchestrator to reconcile with T4 (plan marks T4.S1 Complete).
