# Research: reasoning config plumbing (P1.M2.T1.S1)

Source of truth for the touchpoints. Verified against the live codebase 2026-07-01.

## Render signature (INPUT from P1.M1.T1.S1 — already implemented)

`internal/provider/render.go:89`:
`func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)`

→ `reasoning` is the **4th positional arg**. Today every call site passes `""` (placeholder). S1 wires
the **single-commit** call sites; S2 wires the decompose role call sites.

## ResolveRoleModel callers that break on the 2→3 return change

Non-test (5) — S1 must make these compile (`_, _` → add `, _` discard; behavior unchanged; S2 wires):
- `internal/decompose/roles.go:96` — `prov, mdl := config.ResolveRoleModel(role, cfg)` (the ResolveRoles loop)
- `internal/decompose/planner.go:62` — `_, mdl := config.ResolveRoleModel("planner", deps.Config)`
- `internal/decompose/stager.go:78`  — `_, mdl := config.ResolveRoleModel("stager", deps.Config)`
- `internal/decompose/message.go:103`— `_, mdl := config.ResolveRoleModel("message", deps.Config)`
- `internal/decompose/arbiter.go:82` — `_, mdl := config.ResolveRoleModel("arbiter", deps.Config)`

Test (7, all in `internal/config/roles_test.go`) — S1 updates to `p, m, r :=` + adds reasoning asserts.

NOTE: the 4 decompose role Render calls (planner.go:96, stager.go:87, message.go:127, arbiter.go:95)
ALREADY pass reasoning=`""` and stay `""` — S2 (P1.M2.T1.S2) wires them to the resolved reasoning.
S1 does NOT touch those Render args (no behavior change, no overlap with S2's RoleModels.Reasoning work).

## Single-commit Render call sites that S1 wires to cfg.Reasoning

- `internal/generate/generate.go:196` — `deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")` → `…, cfg.Reasoning)`
- `pkg/stagecoach/stagecoach.go:461` — `deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")` → `…, cfg.Reasoning)`

Both read `cfg.Model` directly (NOT ResolveRoleModel) — so reasoning is read the same way: `cfg.Reasoning`
(the global). For the message role this equals ResolveRoleModel("message").reasoning because message's
shipped default is `off` (= the "" zero value). default_action.go / buildDeps need NO change — CommitStaged
already receives `cfg` and reads cfg.Reasoning internally.

## Shipped reasoning default + bootstrap interaction (CONFIRMED)

`grep reasoning internal/config/bootstrap.go` → **bootstrap does NOT write reasoning.** So a bootstrapped
config leaves `cfg.Reasoning == ""`. ResolveRoleModel resolution: per-role → global → `defaultRoleReasoning[role]`.
With global="" the shipped fallback fires: `planner → "high"`, others → "" (off). ⇒ planner=high holds for
the default/shipped config (matches PRD §16.2 comment "planner defaults to high"). If a user later sets
`[defaults] reasoning = "off"`, that global wins over the shipped fallback (correct precedence). The
bootstrap optionally also writing `[role.planner].reasoning="high"` for robustness is a P1.M4.T2 concern.

## setRole* idiom (load.go) — map-value-copy write-back is load-bearing

`setRoleProvider`/`setRoleModel` do `rc := c.Roles[role]; rc.X = v; c.Roles[role] = rc`. The middle write-back
is REQUIRED (Go maps return value copies). `setRoleReasoning` MUST mirror this exactly.

## root.go flag registration — the message-* gap (FR-R3)

`root.go` registers planner/stager/arbiter `{provider,model}` but NO `message-*` and NO reasoning flags.
`loadFlags`/`loadEnv` already loop all 4 roles (incl. message) for provider/model — `fs.Changed("message-*")`
is just always false today. S1 registers: global `--reasoning`; per-role `--<role>-reasoning` ×4 (incl.
message); AND `--message-provider`/`--message-model` to fully close the FR-R3 "all three flags × all four
roles" gap (load.go already handles them).

## Value semantics — reasoning is a plain string; non-zero overlay is correct

`off|low|medium|high` are literal strings; "off" is NON-empty, so the file/overlay non-zero merge
(`if x != ""`) treats `reasoning = "off"` as a real override (not skipped). The zero value `""` = "unset /
fall through". Defaults() sets Reasoning="". No pointer needed (unlike the Manifest pointer fields) —
this mirrors how Provider/Model (plain strings) already work in Config.
