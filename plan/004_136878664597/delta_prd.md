# Delta PRD — Reasoning is opt-in everywhere (off for all roles)

## 1. Diff analysis (what actually changed)

`diff plan/003_6ce49c39466e/prd_snapshot.md PRD.md` yields **6 changed lines across one theme** — reasoning defaults. Word/line count: trivial.

| # | PRD location | Change |
|---|---|---|
| 1 | Revision history (line 12) | "per-role **agent**/model configuration" → "per-role **provider**/model configuration". **Doc-only terminology cleanup** (the code already says `provider`; P1.M2.T1 implemented this correctly). No code impact. |
| 2 | FR-R6 (line 365) | Shipped reasoning defaults flipped: **OLD** `planner = high; stager = message = arbiter = off` → **NEW** `planner = stager = message = arbiter = off`. Reasoning is now **opt-in everywhere**; set any role to `low`/`medium`/`high` to enable it (most commonly the planner). |
| 3 | FR-R6 duplicate paragraph (line 367) | **Removed** a stale duplicate FR-R6 paragraph that still used the old "agent/provider/model" wording. **Doc-only cleanup** — the code had no duplicate (`ResolveRoleModel` is a single function). No code impact. |
| 4 | FR-B1 (line 392) | `config init` now also writes `reasoning = "off"` explicitly into `[defaults]`, so the field is *discoverable and obviously opt-in* in the generated file rather than hidden (rationale: "a property absent from the written config is, for the user, a property that does not exist"). |
| 5 | CLI flag table §15.2 (line 1328) | `--reasoning` default column: `off (planner: high)` → `off`; note added "`off` for every role out of the box." |
| 6 | Config examples §16.2/§16.4 (lines 1444, 1502, 1508) | Comments updated: "planner defaults to high" → "shipped default is off for every role"; planner `reasoning = "high"` annotated as **OPT-IN**. |

**Net new functionality:** none. **Modified requirement:** one (FR-R6 shipped default) + one enhancement (FR-B1 emits `reasoning="off"`). **Removed:** one stale duplicate PRD paragraph (no code).

## 2. Size check

This is a **small tweak** (a single default-value flip + a config-init line + doc cleanups). Output complexity is matched to that: **1 phase, 1 milestone, 1–2 tasks.** Not a feature; not a new subsystem.

## 3. Scope delta

### Requirement MODIFIED — FR-R6: shipped reasoning default flipped to `off` for ALL roles

The previous session (P1.M2.T1.S1, step g) implemented the shipped fallback as `planner = high; others = off` in `internal/config/roles.go`'s `defaultRoleReasoning` map. This delta **removes the `planner → high` entry**, so every role resolves to `off` (the empty zero value) when nothing else is set. Resolution precedence is otherwise unchanged (CLI > env > `[role.*]` > `[defaults]` > shipped default). The opt-in path is unchanged: a user sets `[role.planner].reasoning = "high"` (or `--planner-reasoning high`) to get the prior planner behavior.

**Affected implementation sites (all in the completed P1 codebase — update, do not re-implement):**

- `internal/config/roles.go` — `defaultRoleReasoning` map: delete the `"planner": "high"` entry (leave the map empty or the var removed, whatever is cleanest; off is the `""` zero value so no other role needs an entry). Update the doc comment block (lines 1–19) and the inline fallback comment (line 63) that say "planner→high" to "all roles → off (opt-in)".
- `internal/config/config.go` — three doc comments still state "planner=high" / "fall through to the per-role shipped default (planner=high)": lines 27–28, 65, 126. Rewrite to "off for every role; opt-in per role".
- `internal/cmd/root.go:137` — the `--reasoning` flag help string says `default off, planner: high`; change to `default off` (matches §15.2 row 5 above).

**Tests that assert the OLD shipped default (must be flipped to assert `off`):**
- `internal/config/roles_test.go` — lines 43, 85 (planner default == "high"), 112/132/136 (per-role override cases — these set reasoning explicitly so they stay, but verify wording), 154/168 (global-unset planner→high cases become planner→off), 183/189/192 (precedence cases — re-derive expected values; "global off beats shipped high" no longer applies since shipped is now off).
- `internal/decompose/roles_test.go` — lines 541/551/552 and 566/579/580 assert `Planner.Reasoning == "high"` as the shipped default; flip to `"off"` and rewrite the comments.
- `internal/cmd/default_action_test.go:1439-1440` — asserts stderr contains `(reasoning: high)` on the planner line; with the default now `off` the rendered reasoning token is absent/`off`. Update the assertion to the new rendered form (the planner line should show `off` or no reasoning suffix per `internal/ui`'s verbose formatter).
- `internal/ui/verbose_test.go:20` — asserts `DEBUG: planner ... (reasoning: high)`; this is a *format* test using a hand-built role model, so it may be fine as-is if the input fixture explicitly sets `high`. Verify the fixture and keep it testing the formatter, not the default. (If the fixture relied on the default, set `Reasoning: "high"` explicitly on the test input.)

> **Guidance:** the safest, smallest mechanical change is: remove the one map entry, fix the doc comments, then let `go test ./...` enumerate every assertion that needs flipping — the compiler/tests will be exhaustive and there is no need to hand-audit every call site.

### Requirement MODIFIED — FR-B1: `config init` writes `reasoning = "off"` explicitly

`internal/config/bootstrap.go` writes `[defaults]` with `provider` uncommented and the rest commented (lines 119–128). This delta adds an **uncommented** `reasoning = "off"` line immediately after `provider`, so the generated file visibly exposes the opt-in field. Add a short inline comment explaining it is the shipped default emitted for discoverability.

## 4. Documentation impact

**Mode A (rides with the work):**
- `internal/config/roles.go`, `internal/config/config.go` — doc comments on `defaultRoleReasoning` / `Config.Reasoning` / `RoleConfig.Reasoning` (covered above).
- `internal/cmd/root.go` — `--reasoning` flag help text (covered above).
- `internal/config/bootstrap.go` — the inline comment on the newly-emitted `reasoning = "off"` line.
- `internal/config/providers/*.toml` — **none.** The `reasoning_levels` manifest tables (pi/claude) are unchanged; only the *which level is the default* changes, not the level→tokens mapping. Verify no provider `.toml` hard-codes a planner default (none should).

**Mode B (changeset-level docs):** **No.** There is no `docs/` tree in this repo (confirmed: no `docs/` directory), and the README feature blurb / marketing surface does not mention reasoning-level defaults. The PRD itself (§9.15 FR-R6, §15.2, §16.2, §16.4) is the only cross-cutting doc and it is **already updated** in `PRD.md` — no separate sync task is needed. The repo's generated `--help` text and the bootstrap config comment are the user-facing surfaces, both covered by Mode A above.

## 5. Reference to completed work

The previous session (plan/003) implemented the full FR-R6 reasoning machinery: the `ReasoningLevels` manifest table, the `Render(model, sys, user, reasoning)` signature, the per-role config plumbing (`RoleConfig.Reasoning`, `--<role>-reasoning` flags for all four roles including message), and `ResolveRoleModel`'s 3-return. **All of that stands unchanged.** This delta touches only the *value of the lowest precedence layer* (the shipped fallback) and one line in config-init output — it is a ~1-map-entry behavioral edit plus comment/test hygiene. Reuse all of P1.M2.T1; do not restructure reasoning resolution.

## 6. Leveraged prior research

- `plan/003_6ce49c39466e/architecture/scout_config_model.md` — locates `ResolveRoleModel` (roles.go:28→63), the role-resolution layers, and confirms off is the `""` zero value (so only planner=high needed an entry — removing it is the whole change).
- `plan/003_6ce49c39466e/architecture/system_context.md` — the reasoning-precedence narrative.
- No new research needed. No web search needed.

---

## Phase P1 — Flip reasoning default to opt-in (off for all roles)

### Milestone P1.M1 — Remove planner=high shipped fallback; off everywhere

**Task P1.M1.T1 — defaultRoleReasoning flip + doc comments + flag help + tests**
- Remove the `"planner": "high"` entry from `defaultRoleReasoning` (`internal/config/roles.go`); all roles now resolve to `off` when unset. Update the var's doc comment and the `ResolveRoleModel` fallback comment.
- Update the three `internal/config/config.go` doc comments (Config/Defaults/RoleConfig.Reasoning) that still cite "planner=high".
- Update `internal/cmd/root.go:137` `--reasoning` help text: drop `, planner: high`.
- Run `go test ./...`; flip every assertion of the planner shipped default from `"high"` to `"off"` (roles_test.go, decompose/roles_test.go, default_action_test.go). For `ui/verbose_test.go`, ensure fixtures set `Reasoning` explicitly (it is a formatter test, not a default test).
- **Mode A docs:** the comments above + the `--reasoning` help string ride with this task.

**Task P1.M1.T2 — config init emits `reasoning = "off"` (FR-B1)**
- In `internal/config/bootstrap.go` (the `[defaults]` writer, lines ~119–128), add an **uncommented** `reasoning = "off"` line after `provider`, with an inline comment noting it is the shipped default emitted for discoverability/opt-in clarity.
- Add/adjust a bootstrap test asserting the generated config contains an uncommented `reasoning = "off"` under `[defaults]`.
- **Mode A docs:** the inline comment on the emitted line.

*Done state: `go test ./...` green; `stagecoach config init` writes an uncommented `reasoning = "off"`; with no config a planner run resolves reasoning to `off`; setting `[role.planner].reasoning = "high"` still turns it on.*
