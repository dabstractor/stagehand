# System Context — Plan 004 (Reasoning opt-in everywhere)

## 1. Project state

**Stagecoach** is a Go CLI tool that generates git commit messages by shelling out to AI coding agents
(pi, Claude Code, Gemini, opencode, etc.) using the user's existing coding-plan subscription quota.

The codebase is **mature** — v1.0 (single-commit core) was built in plan/001, v2.0 features
(multi-commit decomposition, per-role models, binary filtering, agy/qwen-code providers, config
bootstrap, cascading provider priority) were built in plan/002, and config v3 (inference provider
folded into model string, reasoning levels, freeze hardening) was built in plan/003.

**All tests pass** (`go test ./...` — 14 packages ok). **`go vet ./...`** clean. **`go build ./...`** PASS.

Go version: `go1.26.4`. Git: `2.54.0`. Dependencies: `cobra v1.10.2`, `pelletier/go-toml/v2 v2.4.2`.

### Available agents on this machine
`pi`, `claude`, `gemini`, `opencode`, `codex`, `agy`, `cursor` (`agent`) — all on `$PATH`.

## 2. What this plan (004) changes

**ONE behavioral change + ONE enhancement. Nothing new is built.**

### Change A — FR-R6: shipped reasoning default flipped to `off` for ALL roles

**Before (plan/003):** `internal/config/roles.go` had a `defaultRoleReasoning` map with `planner: "high"`
(all other roles defaulted to `""` = off). This meant the decomposition planner ran with thinking/reasoning
on by default.

**After (this plan):** The `defaultRoleReasoning` map is removed entirely. Every role — including the
planner — resolves to `off` (the `""` zero value) when nothing is set. Reasoning is **opt-in everywhere**:
a user sets `[role.planner].reasoning = "high"` (or `--planner-reasoning high`) to enable it.

The resolution precedence chain is unchanged:
```
CLI flag > env > [role.<role>] config > [defaults] global > shipped default (now off for all)
```

The only delta is the **value of the lowest layer**: `planner`'s shipped fallback goes from `"high"` to `""`.

### Change B — FR-B1: `config init` writes `reasoning = "off"` explicitly

`config init` now emits an **uncommented** `reasoning = "off"` line in the `[defaults]` section of the
generated config, so the field is discoverable and obviously opt-in (rationale: "a property absent from the
written config is, for the user, a property that does not exist").

## 3. Implementation sites (verified against current HEAD + working tree)

### Change A — roles.go + tests + comments

| File | Lines | Change |
|---|---|---|
| `internal/config/roles.go` | 1–63 | **Remove** `defaultRoleReasoning` map (was `{"planner": "high"}`). Rewrite `ResolveRoleModel`'s fallback logic (line ~63: `reasoning = cfg.Reasoning` — drop the final `defaultRoleReasoning[role]` fallback). Rewrite doc comment block (lines 1–19) and inline comments. |
| `internal/config/config.go` | 27–28, 65, 126 | Update 3 doc comments: `RoleConfig.Reasoning`, `Config.Reasoning`, `Defaults() Reasoning` — change "planner=high" / "fall through to shipped default (planner=high)" to "off for every role; opt-in per role". |
| `internal/cmd/root.go` | 137 | `--reasoning` help string: `default off, planner: high` → `default off`. |
| `pkg/stagecoach/stagecoach.go` | 62, 66 | `RoleModel` comment: "shipped default" → "off by default for every role". |
| `internal/config/roles_test.go` | ~43, ~85, ~109-116, ~154-168, ~172-189 | Flip assertions: `TestResolveRoleModel_FullOverride` planner reasoning `high`→`""`; `TestResolveRoleModel_BothEmptyManifestSentinel` same; `TestResolveRoleModel_AllCanonicalRoles` planner entry `high`→`""`; rename `TestResolveRoleModel_PlannerShippedDefault` → `TestResolveRoleModel_NoShippedReasoningDefault`, assert all roles `""`; `TestResolveRoleModel_ReasoningOffIsNonZero` comment updates. |
| `internal/decompose/roles_test.go` | ~540-580 | Rename `TestResolveRoles_ReasoningShippedDefault` → `TestResolveRoles_NoShippedReasoningDefault`; assert planner `""` (not `"high"`); `TestResolveRoles_ReasoningPerRoleSet` planner assertion `high`→`""`. |
| `internal/cmd/default_action_test.go` | ~1438-1440 | `TestProgressLabel_DecomposeVerboseRoles`: change assertion from "stderr contains `(reasoning: high)`" to "stderr contains NO reasoning suffix". |
| `docs/cli.md` | 43 | `--reasoning` default column: `"" (off; planner: high)` → `"" (off)`. |

### Change B — bootstrap.go

| File | Lines | Change |
|---|---|---|
| `internal/config/bootstrap.go` | ~124 | Add `b.WriteString("reasoning = \"off\"   # off\|low\|medium\|high; off by default for every role (FR-R6)\n")` after the `provider` line in `buildBootstrapConfig`'s `[defaults]` section. |
| `internal/config/bootstrap_test.go` | ~26-31 | Add assertion: `content` contains `reasoning = "off"` uncommented under `[defaults]`. |
| `docs/configuration.md` | 77-83 | Move `reasoning = "off"` from commented to uncommented in the config example; update comment text. |

## 4. What does NOT change

- **`internal/ui/verbose.go` / `verbose_test.go`** — the `reasoningSuffix` formatter and its format test use
  explicit fixtures (`RoleLine{Reasoning: "high"}`). These test the FORMATTING function, not the default, and
  are correct as-is.
- **`providers/*.toml`** — the `reasoning_levels` manifest tables (pi `--thinking`, claude `--effort`) are
  UNCHANGED. Only which level is DEFAULT changes, not the level→tokens mapping.
- **`docs/providers.md`** — documents the manifest `reasoning_levels` field shape, not defaults. No change.
- **`docs/how-it-works.md`, `README.md`** — no reasoning-default claims. README shows `--reasoning high` as
  an example invocation, not as a default.
- **`internal/provider/render.go`** — the Render chokepoint's reasoning-token emission is unchanged (it already
  does a graceful no-op for `""`/`"off"`).

## 5. Key architectural facts for downstream agents

1. **`ResolveRoleModel` (roles.go:42-67)** returns `(provider, model, reasoning string)`. Resolution is
   per-field independent (FR-R3): provider from per-role→global, model from per-role→global, reasoning from
   per-role→global→**shipped default**. This plan removes the shipped-default layer for reasoning, so
   reasoning falls through to just per-role→global (both of which are `""`/off when unset).

2. **`config.Reasoning` (config.go:65)** — the global `[defaults].reasoning` field. `Defaults()` sets it `""`.
   `config init` now writes `"off"` explicitly (Change B).

3. **`RoleConfig.Reasoning` (config.go:36)** — the per-role `[role.<role>].reasoning` field. Zero value `""`.

4. **`reasoningSuffix` (ui/verbose.go:84-89)** — returns `" (reasoning: <level>)"` for low/medium/high; empty
   for `""`/`"off"`/unknown. This is why the `default_action_test` assertion changes from "contains suffix"
   to "contains NO suffix" — with planner default now `off`, no role emits a suffix in the verbose roster.

5. **The delta_prd.md's Mode B claim is incorrect** — it says "no `docs/` directory exists" but there IS one
   (`docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md`, `docs/providers.md`, `docs/README.md`).
   However, its conclusion (no separate Mode B sweep needed) is still CORRECT: each docs file is a per-file
   Mode A change that rides with the implementing task. There is no cross-cutting feature-overview doc that
   describes the reasoning default.

## 6. Risk assessment

**Risk: NONE.** This is a ~1-line behavioral edit (remove one map entry) + one config-init line + mechanical
test/comment updates. All changes are already validated in the working tree (tests pass). The resolution
precedence chain is unchanged; only the lowest layer's value shifts. No new code paths, no schema changes,
no API changes, no new dependencies.

The one thing to watch: the `default_action_test.go` assertion change (from "contains suffix" to "contains NO
suffix") must be precise — it should assert `!strings.Contains(stderr, "(reasoning:")` rather than checking
for a specific empty string, since all four roles now default to off.
