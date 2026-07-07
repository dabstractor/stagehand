---
name: "P1.M2.T1.S1 — Config reasoning field: RoleConfig.Reasoning + global default + file/overlay/env/flag plumbing"
description: |
  Thread a per-role + global `reasoning` level (`off|low|medium|high`, PRD §9.15 FR-R6 / §16.4 / §15.2)
  end-to-end through the Stagecoach config layer so it reaches `provider.Manifest.Render(...)` as the 4th
  positional arg (the `reasoning` parameter landed by P1.M1.T1.S1, already implemented). This subtask owns
  ONLY the CONFIG plumbing + the SINGLE-COMMIT path; the multi-role decompose wiring is the NEXT subtask
  (P1.M2.T1.S2: ResolveRoles v3 + RoleModels.Reasoning + 4 decompose Render call sites + pkg/stagecoach
  RoleModel). Spec: PRD §16.4 (per-role provider/model/reasoning), §9.15 FR-R1–R6, §16.2 (`[defaults]
  reasoning`), §15.2 flag table (`--reasoning`, `--<role>-reasoning`). Research:
  `plan/003_6ce49c39466e/architecture/scout_config_model.md` §(b)/(c)/(h).

  ⚠️ **THE central design call — `reasoning` is a PLAIN `string`, resolved per-field exactly like
  provider/model.** `off|low|medium|high` are literal TOML strings; `"off"` is NON-empty so the existing
  non-zero file/overlay merge (`if x != ""`) treats `reasoning = "off"` as a real override (never skipped).
  The zero value `""` = "unset → fall through". No pointer indirection is needed (unlike the `Manifest`
  pointer fields in `internal/provider`) — this mirrors how `Config.Provider`/`Config.Model` (plain
  strings) already work everywhere. Do NOT make Reasoning a `*string`.

  ⚠️ **THE second design call — `ResolveRoleModel` becomes a 3-RETURN `(provider, model, reasoning)`.**
  Reasoning resolves with the SAME per-field precedence as provider/model — per-role `[role.<role>].reasoning`
  → global `[defaults].reasoning` (cfg.Reasoning) — and then a THIRD, lowest **shipped-default fallback
  layer** `defaultRoleReasoning[role]` (PRD §9.15 FR-R6: `planner=high`; `stager=message=arbiter=off`).
  Because "off" is the natural "" zero value, the map needs ONLY `{"planner": "high"}` — every other role's
  shipped default is the zero value "" (off). CONFIRMED: `internal/config/bootstrap.go` does NOT write a
  global reasoning, so a bootstrapped/default config has `cfg.Reasoning==""` and the shipped fallback fires
  (planner→high), matching PRD §16.2's "planner defaults to high". If a user sets `[defaults] reasoning =
  "off"`, the global wins over the shipped fallback (correct precedence — that's the user's explicit choice).
  The single-commit path reads `cfg.Reasoning` DIRECTLY (it reads `cfg.Model` directly too, not
  ResolveRoleModel) — for the active `message` role this is identical to ResolveRoleModel("message") because
  message's shipped default is off="".

  ⚠️ **THE third design call — arity change is build-breaking; S1 keeps the WHOLE module green.** Changing
  `ResolveRoleModel`'s return count breaks its 5 non-test + 7 test callers. S1 updates ALL of them: the 5
  decompose non-test call sites get a minimal `, _` discard (build-green, behavior UNCHANGED, marked
  `// TODO(P1.M2.T1.S2): wire reasoning`) — S2 replaces those discards with real reasoning consumption +
  RoleModels.Reasoning. `internal/config/roles_test.go` (7 tests) is fully migrated to `p, m, r :=` with
  reasoning assertions. The 4 decompose role `Render(...)` calls (planner/stager/message/arbiter .go) ALREADY
  pass reasoning=`""` and are LEFT at `""` (S2 wires them) — S1 does not touch those Render args, avoiding
  overlap with S2's RoleModels.Reasoning work.

  ⚠️ **THE fourth design call — close the FR-R3 `message-*` flag gap fully.** PRD §9.15 FR-R3 + §16.4 mandate
  ALL THREE flags (provider/model/reasoning) for ALL FOUR roles, including `message`. Today `root.go`
  registers planner/stager/arbiter `{provider,model}` but NO `message-*` and NO reasoning flags (load.go
  already loops all 4 roles incl. message — `fs.Changed("message-*")` is just always false). S1 registers:
  global `--reasoning`; per-role `--<role>-reasoning` ×4 (planner/stager/message/arbiter); AND
  `--message-provider` + `--message-model` to complete the FR-R3 trio for message (the load.go loop already
  honors them once registered). Registering message-reasoning but not message-provider/model would be an
  inconsistent half-fix the PRD explicitly forbids ("no role is a special case that omits a flag").

  SCOPE BOUNDARY vs P1.M2.T1.S2 (next): S1 = config struct fields + file/materialize/overlay + loadEnv/loadFlags
  + setRoleReasoning + root.go flag registration + ResolveRoleModel 3-return + single-commit Render reasoning
  (generate.go + pkg/stagecoach) + roles_test.go. S2 = ResolveRoles v3 (RoleModels.Reasoning) + the 4 decompose
  role Render reasoning args + pkg/stagecoach RoleModel + FR-R5b model-prefix guard. Do NOT implement S2's
  pieces here.

  Deliverable: edits to `internal/config/{config,file,load,roles}.go`, `internal/cmd/root.go`,
  `internal/generate/generate.go`, `pkg/stagecoach/stagecoach.go`, plus build-green arity fixes in
  `internal/decompose/{roles,planner,stager,message,arbiter}.go` and test updates in
  `internal/config/roles_test.go`. NO new files, NO new deps, NO go.mod change, NO `docs/*.md` edits (DOCS is
  Mode A — inline RoleConfig comment + flag help text ride with the code). INPUT = P1.M1.T1.S1 (Render takes
  `reasoning` 4th arg) + the current config/cmd/generate/pkg source. OUTPUT = `reasoning` flowing config →
  ResolveRoleModel → single-commit Render; every role exposes `--<role>-reasoning` incl. message.
---

## Goal

**Feature Goal**: Add a normalized `reasoning` level (`off|low|medium|high`) to Stagecoach's config model at
TWO granularities — a global `[defaults].reasoning` and a per-role `[role.<role>].reasoning` — and plumb it
through every config layer (file decode → materialize → overlay → env → flag) so that `ResolveRoleModel`
returns `(provider, model, reasoning)` and the single-commit generation path passes it to
`provider.Manifest.Render`. Apply the FR-R6 shipped role default (planner=high, others=off) as the
lowest-resolution fallback. Close the FR-R3 flag gap so all four roles (incl. `message`) expose
`--<role>-{provider,model,reasoning}` plus the global `--reasoning`.

**Deliverable** (edits to existing files only):
1. **`internal/config/config.go`** — (a) add `Reasoning string \`toml:"reasoning"\`` to `RoleConfig`
   (after `Model`); (b) add a global `Reasoning string \`toml:"reasoning"\`` to `Config` in the `[defaults]`
   group (after `Model`); (c) `Defaults()` sets `Reasoning: ""`; (d) refresh the `RoleConfig` doc comment to
   describe reasoning (Mode A docs).
2. **`internal/config/file.go`** — (a) add `Reasoning string \`toml:"reasoning"\`` to `fileDefaults` (after
   `Model`) and to `fileRoleConfig` (after `Model`); (b) `materialize`: copy global
   `if d.Reasoning != "" { c.Reasoning = d.Reasoning }` and extend the per-role `RoleConfig{...}` literal
   with `Reasoning: frc.Reasoning`; (c) `overlay`: add global
   `if src.Reasoning != "" { dst.Reasoning = src.Reasoning }` and a per-role field-merge branch
   `if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }`.
3. **`internal/config/load.go`** — (a) add `func (c *Config) setRoleReasoning(role, reasoning string)`
   (mirror `setRoleModel`'s map-value-copy write-back); (b) `loadEnv`: add global `STAGECOACH_REASONING` and a
   per-role `STAGECOACH_<ROLE>_REASONING` branch in the existing loop; (c) `loadFlags`: add global `--reasoning`
   and a per-role `<role>-reasoning` branch in the existing loop.
4. **`internal/config/roles.go`** — (a) add `var defaultRoleReasoning = map[string]string{"planner": "high"}`
   (the FR-R6 shipped fallback; off is the zero value so only planner is explicit); (b) change
   `ResolveRoleModel` to return `(provider, model, reasoning string)`, resolving reasoning per-role → global →
   `defaultRoleReasoning[role]`.
5. **`internal/cmd/root.go`** — register global `--reasoning` + per-role `--{planner,stager,message,arbiter}-reasoning`
   + `--message-provider` + `--message-model` (FR-R3 complete flag surface); add the corresponding `flag*`
   package vars; help text names env/git-config (Mode A docs).
6. **`internal/generate/generate.go`** + **`pkg/stagecoach/stagecoach.go`** — change the single-commit
   `Render(cfg.Model, sysPrompt, payload, "")` call's 4th arg from `""` to `cfg.Reasoning`.
7. **BUILD-GREEN arity fixes**: `internal/decompose/{roles,planner,stager,message,arbiter}.go` — add `, _` to
   discard the new `ResolveRoleModel` reasoning return (5 sites; behavior unchanged; `TODO(P1.M2.T1.S2)`).
8. **`internal/config/roles_test.go`** — migrate all 7 tests to `p, m, r := ResolveRoleModel(...)` + add
   reasoning assertions (incl. planner→high shipped-default and message→"" / off).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...` green
(existing tests stay green, updated/new tests pass); `ResolveRoleModel` returns 3 values with the
per-role→global→shipped(planner=high) precedence; `[defaults].reasoning` / `[role.X].reasoning` /
`STAGECOACH_REASONING` / `STAGECOACH_<ROLE>_REASONING` / `--reasoning` / `--<role>-reasoning` all override the
resolved reasoning at the correct precedence; the single-commit path passes `cfg.Reasoning` to Render; every
role (incl. message) exposes `--<role>-reasoning`; go.mod/go.sum byte-unchanged; no new files outside the
listed edits.

## User Persona

**Target User**: Downstream (a) **P1.M2.T1.S2** (consumes the 3-return `ResolveRoleModel` to build
`RoleModels.Reasoning` and wire the 4 decompose Render reasoning args), (b) the **single-commit generation
path** (`generate.CommitStaged`, `pkg/stagecoach.GenerateCommit`) which now forwards reasoning to Render, and
(c) transitively every user who wants per-role thinking/reasoning effort (FR-R6) on a supported provider.

**Use Case**: `stagecoach --reasoning high` (global) or `stagecoach --planner-reasoning high` (per-role), or
`[role.planner] reasoning = "high"` in config, sets the reasoning level that Render appends via the
provider's `reasoning_levels` manifest table (P1.M1.T1.S1). A user who sets nothing gets the shipped
planner=high / others=off defaults.

**User Journey**: flag/env/file/git/defaults → `Load()` → resolved `cfg.Reasoning` + `cfg.Roles[role].Reasoning`
→ `ResolveRoleModel(role, cfg)` → `(provider, model, reasoning)` → single-commit `Render(..., reasoning)` (S1)
/ decompose `RoleModels.X.Reasoning` → per-role Render (S2).

**Pain Points Addressed**: removes "how does a reasoning level reach Render", "why is there no
`--message-reasoning`", "what is the planner default", and "can I set reasoning globally vs per-role"
ambiguity for S2 + the generation paths.

## Why

- **Closes the reasoning config half of FR-R6 / §16.4.** P1.M1.T1.S1 added the `reasoning` Render param +
  `Manifest.ReasoningLevels`; without this subtask that param is permanently `""`. S1 is the config-side
  contract that makes reasoning reachable.
- **Unblocks P1.M2.T1.S2 cleanly.** S1 hands S2 a stable 3-return `ResolveRoleModel` + populated
  `cfg.Roles[role].Reasoning` / `cfg.Reasoning`; S2 only has to thread it into `RoleModels` + the decompose
  Render calls (its declared scope).
- **Corrects the FR-R3 flag gap.** message-* flags were missing (a v2 gap the PRD explicitly calls out);
  S1 registers the full trio for all four roles so no role is a special case.
- **Back-compatible.** On the single-commit path with nothing set, `cfg.Reasoning==""` → Render gets "" →
  `ReasoningLevels` is a graceful no-op (FR-R6) → byte-identical v2 behavior.

## What

A compiled config layer where `reasoning` is a first-class plain-string field at global + per-role
granularity, threaded through file/materialize/overlay/env/flag, resolved by `ResolveRoleModel` with the
shipped planner=high fallback, and forwarded to `Render` on the single-commit path. No new types beyond the
`Reasoning` fields and the `defaultRoleReasoning` map; no new files; no dependency change.

### Success Criteria

- [ ] `RoleConfig` has `Reasoning string \`toml:"reasoning"\`` (after `Model`); `Config` has a global
      `Reasoning string \`toml:"reasoning"\`` in the `[defaults]` group (after `Model`); `Defaults()` sets
      `Reasoning: ""`.
- [ ] `fileDefaults` and `fileRoleConfig` each gain `Reasoning string \`toml:"reasoning"\``; `materialize`
      copies global + per-role reasoning (non-zero); `overlay` merges global + per-role reasoning (non-zero
      field-merge, mirroring Provider/Model).
- [ ] `setRoleReasoning(role, reasoning string)` exists and uses the map-value-copy write-back idiom;
      `loadEnv` handles `STAGECOACH_REASONING` + per-role `STAGECOACH_<ROLE>_REASONING`; `loadFlags` handles
      `--reasoning` + per-role `<role>-reasoning` (gated on `fs.Changed`).
- [ ] `ResolveRoleModel(role, cfg) (provider, model, reasoning string)` resolves reasoning per-role → global
      → `defaultRoleReasoning[role]`; `defaultRoleReasoning == {"planner":"high"}`.
- [ ] `root.go` registers `--reasoning`, `--planner-reasoning`, `--stager-reasoning`, `--message-reasoning`,
      `--arbiter-reasoning`, `--message-provider`, `--message-model` (zero defaults; help text names env +
      git-config).
- [ ] `generate.go` and `pkg/stagecoach/stagecoach.go` single-commit `Render(...)` calls pass `cfg.Reasoning`
      (4th arg) instead of `""`.
- [ ] The 5 decompose non-test `ResolveRoleModel` call sites compile (`, _` discard); `roles_test.go` (7
      tests) is migrated to 3-return with reasoning assertions.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test -race ./...` all clean/green; go.mod/go.sum
      byte-unchanged; no files outside the listed edits.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the field/edit table above, the
`setRoleModel`/`setRoleProvider` idiom (quoted below), the `ResolveRoleModel` body (quoted below), the
exact call-site list (quoted from research), and the PRD §16.4/§9.15 FR-R6 precedence. No provider/generation
internals needed beyond "Render takes reasoning as the 4th arg".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/003_6ce49c39466e/P1M2T1S1/research/reasoning_plumbing.md
  why: the verified touchpoint list — exact line numbers for every edit, the 5 non-test + 7 test
       ResolveRoleModel callers, the single-commit Render sites (generate.go:196 + pkg/stagecoach:461), the
       bootstrap-doesn't-write-reasoning confirmation, and the setRole* write-back idiom.
  critical: the arity change breaks 12 call sites — ALL must be updated or the module won't build. The 5
       decompose non-test sites get a `, _` discard (S2 wires behavior); the 7 roles_test.go sites get full
       3-return + assertions.

- docfile: plan/003_6ce49c39466e/architecture/scout_config_model.md
  section: "(b) RoleConfig construction/population sites", "(c) ResolveRoleModel signature + callers",
           "(h) load.go env + flag wiring"
  why: the authoritative scout of every reasoning touchpoint (file:line tables). §(b) lists
       fileRoleConfig/materialize/overlay/setRole*/decompose sites; §(c) lists the 5 ResolveRoleModel
       callers + notes the single-commit path reads cfg.Model directly; §(h) lists the loadEnv/loadFlags
       loops + root.go flag-registration gap (no message-*).
  critical: §(c) "Single-commit path does NOT call ResolveRoleModel — it reads cfg.Model/cfg.Provider
       directly via buildDeps. So a global reasoning default must be read there too." ⇒ generate.go +
       pkg/stagecoach read cfg.Reasoning directly (NOT ResolveRoleModel).

- file: PRD.md
  section: "9.15 Per-role provider/model configuration (FR-R1–R6)" (h3.31), "16.4 Per-role provider/model
           configuration" (h3.69), "16.2 Full config file example" (h3.67), "15.2 Global flags" (h3.62)
  why: §9.15 FR-R6 fixes the reasoning enum (`off|low|medium|high`) + shipped defaults (`planner=high`;
       `stager=message=arbiter=off`) + the "graceful no-op" rule (never an error). §16.4 fixes the
       per-field precedence (flag > env > [role.X] > [defaults] > manifest) + "all four roles including
       message" + the global `[defaults].reasoning`. §16.2 shows `[defaults] reasoning = "off"`. §15.2 is the
       authoritative flag table (`--reasoning`, `--<role>-reasoning`, default "off (planner: high)").
  critical: FR-R6 "off is the natural zero value so only planner=high needs an explicit default" — the
       defaultRoleReasoning map is `{"planner":"high"}` only. The shipped default is the LOWEST layer
       (below the global [defaults].reasoning).

- file: internal/config/config.go
  why: the FROZEN shape you EXTEND — RoleConfig (Provider/Model), Config (Provider/Model in [defaults]),
       Defaults(). Add Reasoning alongside Provider/Model at BOTH granularities. Note Output/StripCodeFence
       are deliberately `*string`/`*bool` (defer-to-manifest); Reasoning is a PLAIN string like Provider/Model.
  pattern: `Provider string \`toml:"provider"\`` / `Model string \`toml:"model"\`` → add
       `Reasoning string \`toml:"reasoning"\`` immediately after Model in both RoleConfig and Config.

- file: internal/config/file.go
  why: the decode/materialize/overlay trio you extend. fileDefaults + fileRoleConfig are the FILE twins of
       Config/RoleConfig defaults; materialize copies non-zero fields; overlay field-merges across layers.
  pattern: mirror EXACTLY how Provider/Model are handled in fileDefaults (L52-58), materialize (global
       `if d.Provider != "" { c.Provider = d.Provider }` ~L186; per-role RoleConfig literal ~L206), and
       overlay (global `if src.Provider != ""` ~L243; per-role `if rc.Provider != "" { existing.Provider = ... }` ~L290).
       Reasoning is identical — non-zero string copy/merge.

- file: internal/config/load.go
  why: loadEnv/loadFlags + setRoleProvider/setRoleModel are the patterns. roleNames (L14) already lists all
       four roles incl. message — the per-role loops already cover message for provider/model.
  pattern: setRoleProvider/setRoleModel (L33-52) use map-value-copy write-back (`rc := c.Roles[role]; rc.X=v;
       c.Roles[role]=rc`) — setRoleReasoning MUST mirror this (the write-back is load-bearing; Go maps return
       copies). loadEnv global (STAGECOACH_PROVIDER ~L161) + per-role loop (~L190); loadFlags global
       (fs.Changed("provider") ~L213) + per-role loop (~L243).

- file: internal/config/roles.go
  why: ResolveRoleModel (L28) is the function you change to 3-return. Its body is the per-field merge
       (role → global → manifest sentinel). Reasoning follows the same shape + a shipped-default fallback.
  pattern: see the quoted body in Implementation Blueprint — add a third resolved value `reasoning` with the
       same per-role→global cascade, then `if reasoning == "" { reasoning = defaultRoleReasoning[role] }`.

- file: internal/cmd/root.go
  why: flag registration (init(), ~L95-133). Registers planner/stager/arbiter {provider,model} but NO
       message-* and NO reasoning. flag* vars are bound to addresses (satisfies the unused linter); loadFlags
       reads via fs.Changed.
  pattern: mirror `pf.StringVar(&flagPlannerModel, "planner-model", "", "…")` — register the 7 new flags
       (reasoning ×5 incl. message; message-provider; message-model) bound to new flag* vars.

- file: internal/generate/generate.go   (L196) and pkg/stagecoach/stagecoach.go (L461)
  why: the two single-commit Render call sites. Both currently pass reasoning=`""` (4th arg).
  pattern: `deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")` → `…, cfg.Reasoning)`. (Render sig from
       P1.M1.T1.S1: `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)`.)

- file: internal/config/roles_test.go
  why: the 7 ResolveRoleModel tests — all use `p, m := ResolveRoleModel(...)`. Migrate to `p, m, r :=` and
       add reasoning assertions. Note TestResolveRoleModel_BothEmptyManifestSentinel (planner, Defaults())
       now expects reasoning="high" (shipped default), NOT "".

- url: https://pkg.go.dev/github.com/spf13/pflag#FlagSet
  why: fs.Changed(name)/GetString(name) — the API loadFlags uses. Register flags at ZERO default so Changed
       reflects "user passed it".
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go            # Config + RoleConfig + Defaults() + CurrentConfigVersion    ← EDIT (Reasoning fields)
  file.go              # fileDefaults/fileRoleConfig/materialize/overlay            ← EDIT (reasoning plumbing)
  load.go              # loadEnv/loadFlags/setRoleProvider/setRoleModel/roleNames    ← EDIT (setRoleReasoning + reasoning loops)
  roles.go             # ResolveRoleModel (2-return)                                 ← EDIT (3-return + defaultRoleReasoning)
  roles_test.go        # 7 ResolveRoleModel tests                                    ← EDIT (3-return + assertions)
  role_defaults.go     # FR-D4 model table (untouched)                               ← (defaultRoleReasoning lives in roles.go)
  bootstrap.go         # writes bootstrap config (NO reasoning today — untouched)    ← (no edit; doesn't write reasoning)
  config_test.go/load_test.go/file_test.go  # existing tests                         ← may need reasoning cases (see Validation)
internal/cmd/
  root.go              # flag registration                                           ← EDIT (--reasoning + per-role + message-*)
internal/generate/
  generate.go          # CommitStaged single-commit Render (L196)                    ← EDIT (reasoning "" → cfg.Reasoning)
internal/decompose/
  roles.go planner.go stager.go message.go arbiter.go  # ResolveRoleModel callers    ← EDIT (build-green `, _` discard ×5)
pkg/stagecoach/
  stagecoach.go         # GenerateCommit single-commit Render (L461)                  ← EDIT (reasoning "" → cfg.Reasoning)
internal/provider/
  render.go            # Render(model, sysPrompt, userPayload, reasoning, mode...)   ← (INPUT from P1.M1.T1.S1; NO edit)
go.mod / go.sum        # unchanged (no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All changes are EDITS to existing files (listed in Current Codebase tree above).
# defaultRoleReasoning is added to internal/config/roles.go (co-located with its sole consumer, ResolveRoleModel).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1): Reasoning is a PLAIN string, NOT *string. off|low|medium|high are literal
// TOML strings; "off" is NON-empty so the file/overlay non-zero merge (if x != "") treats reasoning="off"
// as a real override (never skipped). The zero value "" = "unset → fall through". This mirrors how
// Config.Provider/Config.Model (plain strings) already work. Do NOT make it a pointer.

// CRITICAL (design call #2): ResolveRoleModel's reasoning resolution is per-role → global → SHIPPED DEFAULT
// (defaultRoleReasoning[role]), in that order. The shipped default is the LOWEST layer (below the global
// [defaults].reasoning). defaultRoleReasoning = {"planner":"high"} ONLY — off is the zero value, so stager/
// message/arbiter need no entry. CONFIRMED bootstrap.go writes NO reasoning ⇒ a default config has
// cfg.Reasoning=="" ⇒ the shipped fallback fires (planner→high), matching PRD §16.2 "planner defaults to high".

// CRITICAL (design call #3): the 2→3 return change breaks 12 call sites. ALL must compile or the module
// won't build (CI gate). The 5 decompose non-test sites get `, _` discard + `// TODO(P1.M2.T1.S2): wire
// reasoning` (behavior UNCHANGED — S2 does the real wiring). roles_test.go (7) migrates fully to 3-return.
// The 4 decompose role Render(...) calls ALREADY pass reasoning="" and are LEFT at "" (S2 wires them) —
// do NOT touch those Render args (avoids overlap with S2's RoleModels.Reasoning scope).

// CRITICAL (design call #4): close the FR-R3 flag gap FULLY. Register --reasoning (global) +
// --{planner,stager,message,arbiter}-reasoning + --message-provider + --message-model. load.go's roleNames
// already loops all four roles incl. message for provider/model — once the message-* flags are registered,
// fs.Changed("message-provider") etc. work. Registering reasoning but not message-provider/model would be
// the inconsistent half-fix PRD §9.15 FR-R3 forbids ("no role is a special case that omits a flag").

// CRITICAL: setRoleReasoning MUST use the map-value-copy write-back idiom (rc := c.Roles[role]; rc.Reasoning
// = v; c.Roles[role] = rc). The write-back line is load-bearing — Go maps return value copies, so
// `c.Roles[role].Reasoning = v` alone mutates a local copy and is silently lost. setRoleProvider/setRoleModel
// (load.go:33-52) are the reference.

// CRITICAL: the single-commit path reads cfg.Reasoning DIRECTLY (it reads cfg.Model directly, NOT
// ResolveRoleModel). For the active "message" role this is identical to ResolveRoleModel("message").reasoning
// because message's shipped default is off="". Do NOT route generate.go through ResolveRoleModel — just pass
// cfg.Reasoning as Render's 4th arg (matches how it passes cfg.Model as the 1st).

// GOTCHA: ResolveRoleModel is in package config and returns bare named returns (provider, model string).
// Keep the named-return style when adding `reasoning` (provider, model, reasoning string) — several tests
// and the docstring reference the names. default_action.go/buildDeps need NO change (CommitStaged receives
// cfg and reads cfg.Reasoning internally).

// GOTCHA: TOML round-trip — `[defaults] reasoning = "off"` and `[role.planner] reasoning = "high"` decode
// fine into the plain string fields (go-toml/v2 has no issue with these). The non-zero materialize/overlay
// means a file CANNOT set reasoning back to "" (the v1 zero-value limitation) — but "" means "unset" so that
// is correct, not a bug (to force off, write reasoning = "off", which is non-empty).

// GOTCHA: do NOT edit render.go / manifest.go (P1.M1.T1.S1's Render reasoning param + ReasoningLevels are
// the INPUT, already implemented). Do NOT edit role_defaults.go (FR-D4 model table is a separate concern;
// defaultRoleReasoning is role-level, not provider×role, and lives in roles.go). Do NOT bump
// CurrentConfigVersion (that is P1.M3.T1.S1). Do NOT touch docs/*.md (DOCS is Mode A — inline comments +
// flag help text ride with the code; the changeset-level doc sync is P4.M2.T1).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — RoleConfig (add Reasoning after Model)
type RoleConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"` // off|low|medium|high (FR-R6); "" ⇒ inherit global [defaults].reasoning ⇒ shipped default
}

// internal/config/config.go — Config [defaults] group (add Reasoning after Model)
type Config struct {
	// [defaults] (PRD §16.2)
	Provider  string        `toml:"provider"`
	Model     string        `toml:"model"`
	Reasoning string        `toml:"reasoning"` // off|low|medium|high (FR-R6); "" ⇒ ResolveRoleModel's shipped fallback (planner=high)
	Timeout   time.Duration `toml:"timeout"`
	// ... (rest unchanged)
}

// internal/config/config.go — Defaults() (add Reasoning: "")
func Defaults() Config {
	return Config{
		Provider:  "",
		Model:     "",
		Reasoning: "", // FR-R6: "" ⇒ fall through to the per-role shipped default (planner=high) in ResolveRoleModel
		// ... (rest unchanged)
	}
}
```

```go
// internal/config/roles.go — defaultRoleReasoning + 3-return ResolveRoleModel

// defaultRoleReasoning is the FR-R6 SHIPPED per-role reasoning fallback (the LOWEST resolution layer,
// below the global [defaults].reasoning): planner=high (decomposition benefits from reasoning);
// stager/message/arbiter=off. "off" is the natural "" zero value, so ONLY planner needs an entry —
// every other role's shipped default is "" (off). Applied by ResolveRoleModel after per-role and global
// are both empty. (Lives here, co-located with its sole consumer; the FR-D4 provider×role MODEL table
// is a separate concern in role_defaults.go.)
var defaultRoleReasoning = map[string]string{
	"planner": "high",
}

// ResolveRoleModel returns the (provider, model, reasoning) for a single agent role (PRD §16.4, §9.15
// FR-R1–R3/R6). Provider/model resolve per-field: per-role ([role.<role>]) → global ([defaults]) → ("","")
// manifest-default sentinel. Reasoning resolves the SAME per-field cascade (per-role → global) and then a
// THIRD, lowest shipped-default fallback (defaultRoleReasoning[role]: planner→high, else "" (off)).
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg. This function only
// checks the per-role entry, falls back to the global, and (reasoning only) to the shipped default. It does
// NOT consult any manifest. The reasoning "" / "off" is the graceful-no-op sentinel for Render (FR-R6).
func ResolveRoleModel(role string, cfg Config) (provider, model, reasoning string) {
	if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
		if rc.Reasoning != "" {
			reasoning = rc.Reasoning
		}
	}
	if provider == "" {
		provider = cfg.Provider
	}
	if model == "" {
		model = cfg.Model
	}
	if reasoning == "" {
		reasoning = cfg.Reasoning // global [defaults].reasoning (may itself be "")
	}
	if reasoning == "" {
		reasoning = defaultRoleReasoning[role] // FR-R6 shipped fallback: planner→high; others→"" (off)
	}
	return provider, model, reasoning
}
```

```go
// internal/config/load.go — setRoleReasoning (mirror setRoleModel's write-back idiom)
func (c *Config) setRoleReasoning(role, reasoning string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Reasoning = reasoning
	c.Roles[role] = rc // REQUIRED write-back (Go maps return value copies); preserves sibling Provider/Model (FR-R3)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: config.go — add Reasoning to RoleConfig + Config + Defaults()
  - ADD Reasoning string `toml:"reasoning"` to RoleConfig (after Model) with a one-line FR-R6 comment.
  - ADD Reasoning string `toml:"reasoning"` to Config's [defaults] group (after Model).
  - ADD Reasoning: "" to Defaults().
  - REFRESH the RoleConfig doc comment to mention reasoning (Mode A).
  - GOTCHA: plain string (NOT *string); placement immediately after Model in both structs.

Task 2: file.go — thread reasoning through fileDefaults/fileRoleConfig/materialize/overlay
  - ADD Reasoning string `toml:"reasoning"` to fileDefaults (after Model) and fileRoleConfig (after Model).
  - materialize GLOBAL: `if d.Reasoning != "" { c.Reasoning = d.Reasoning }` (right after the d.Model branch).
  - materialize PER-ROLE: extend `RoleConfig{Provider: frc.Provider, Model: frc.Model}` → add `Reasoning: frc.Reasoning`.
  - overlay GLOBAL: `if src.Reasoning != "" { dst.Reasoning = src.Reasoning }` (right after the src.Model branch).
  - overlay PER-ROLE field-merge: `if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }` (after the rc.Model branch).
  - PATTERN: copy the Provider/Model handling verbatim, s/Provider/Reasoning/ / s/Model/Reasoning/ as appropriate.

Task 3: load.go — setRoleReasoning + loadEnv + loadFlags
  - ADD func (c *Config) setRoleReasoning(role, reasoning string) per the Data Models block (write-back idiom).
  - loadEnv GLOBAL: `if v, ok := os.LookupEnv("STAGECOACH_REASONING"); ok && v != "" { cfg.Reasoning = v }`
    (near the STAGECOACH_MODEL branch).
  - loadEnv PER-ROLE loop: add `if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" { cfg.setRoleReasoning(role, v) }`
    (after the _MODEL branch in the roleNames loop).
  - loadFlags GLOBAL: `if fs.Changed("reasoning") { if v, err := fs.GetString("reasoning"); err == nil { cfg.Reasoning = v } }`
    (near the model branch).
  - loadFlags PER-ROLE loop: add `if fs.Changed(role + "-reasoning") { if v, err := fs.GetString(role + "-reasoning"); err == nil { cfg.setRoleReasoning(role, v) } }`
    (after the role-model branch).

Task 4: roles.go — defaultRoleReasoning + 3-return ResolveRoleModel
  - ADD var defaultRoleReasoning = map[string]string{"planner": "high"} (with the FR-R6 doc comment).
  - CHANGE ResolveRoleModel signature to (provider, model, reasoning string); add the reasoning cascade
    (per-role → global → defaultRoleReasoning[role]) per the Data Models block.
  - GOTCHA: keep named returns; the shipped fallback is the LOWEST layer (below global).

Task 5: root.go — register 7 flags (close the FR-R3 gap)
  - ADD flag vars: flagReasoning, flagPlannerReasoning, flagStagerReasoning, flagMessageReasoning,
    flagArbiterReasoning, flagMessageProvider, flagMessageModel (in the decompose/per-role vars block).
  - REGISTER in init(): `--reasoning` (global), `--planner-reasoning`, `--stager-reasoning`,
    `--message-reasoning`, `--arbiter-reasoning`, `--message-provider`, `--message-model` — all zero default,
    StringVar, help text naming `STAGECOACH_*` env + `stagecoach.role.<role>` git-config (mirror the planner-model line's help style; Mode A docs).
  - GOTCHA: load.go already loops all four roles incl. message — registering the flags is what makes
    fs.Changed("message-*") true when passed.

Task 6: single-commit path — forward cfg.Reasoning to Render
  - internal/generate/generate.go:196: `Render(cfg.Model, sysPrompt, payload, "")` → `…, cfg.Reasoning)`.
  - pkg/stagecoach/stagecoach.go:461: `Render(cfg.Model, sysPrompt, payload, "")` → `…, cfg.Reasoning)`.
  - GOTCHA: read cfg.Reasoning DIRECTLY (these paths read cfg.Model directly too, NOT ResolveRoleModel).

Task 7: build-green arity fixes — the 5 decompose ResolveRoleModel call sites
  - internal/decompose/roles.go:96: `prov, mdl := config.ResolveRoleModel(role, cfg)` → `prov, mdl, _ := …`
    + `// TODO(P1.M2.T1.S2): wire reasoning into RoleModels`.
  - internal/decompose/planner.go:62: `_, mdl := config.ResolveRoleModel("planner", deps.Config)` → `_, mdl, _ := …`
  - internal/decompose/stager.go:78:  `_, mdl := config.ResolveRoleModel("stager", deps.Config)`  → `_, mdl, _ := …`
  - internal/decompose/message.go:103:`_, mdl := config.ResolveRoleModel("message", deps.Config)`→ `_, mdl, _ := …`
  - internal/decompose/arbiter.go:82: `_, mdl := config.ResolveRoleModel("arbiter", deps.Config)` → `_, mdl, _ := …`
  - GOTCHA: behavior UNCHANGED (discard reasoning). Do NOT touch the 4 decompose Render(...) reasoning args
    (already "" — S2 wires them). This keeps S1/S2 non-overlapping.

Task 8: roles_test.go — migrate 7 tests to 3-return + reasoning assertions
  - Change every `p, m := ResolveRoleModel(...)` → `p, m, r := ResolveRoleModel(...)`.
  - Add reasoning assertions per test:
      * GlobalFallbackRolesNil/RoleAbsent/UnknownRole (message, no reasoning set) → r == "" (off).
      * FullOverride/ModelOnly/ProviderOnly → r == "" (no reasoning configured in those fixtures).
      * BothEmptyManifestSentinel (planner, Defaults()) → r == "high" (FR-R6 shipped planner default!).
      * AllCanonicalRoles → assert r: planner="high" (shipped, since global/role reasoning unset); message/
        arbiter="" (off).
  - ADD 2-3 NEW tests: TestResolveRoleModel_ReasoningPerRole (per-role reasoning overrides global);
    TestResolveRoleModel_ReasoningGlobalFallback (no per-role → global); TestResolveRoleModel_PlannerShippedDefault
    (Defaults(), planner → "high", message → "").
  - ALSO add reasoning cases to file_test.go (materialize/overlay reasoning copy) + load_test.go
    (STAGECOACH_REASONING / --reasoning / per-role) if those suites assert field coverage — at minimum ensure
    they still compile/pass (Reasoning is a new field; existing struct literals/DeepEquals are unaffected).

Task 9: VERIFY (no further file change)
  - RUN the full Validation Loop. go.mod/go.sum byte-unchanged. No files outside the listed edits touched.
    All existing tests stay green; the 5 decompose sites compile (S2 owns their behavioral wiring).
```

### Implementation Patterns & Key Details

```go
// The per-field resolution cascade in ResolveRoleModel — reasoning follows provider/model, + shipped fallback:
if rc, ok := cfg.Roles[role]; ok {
	if rc.Reasoning != "" {
		reasoning = rc.Reasoning // per-role [role.<role>].reasoning wins
	}
}
if reasoning == "" {
	reasoning = cfg.Reasoning // global [defaults].reasoning
}
if reasoning == "" {
	reasoning = defaultRoleReasoning[role] // FR-R6 shipped: planner→high; others→"" (off)
}

// The map-value-copy write-back in setRoleReasoning — the write-back line is load-bearing:
rc := c.Roles[role]   // VALUE copy (Go maps return copies)
rc.Reasoning = v
c.Roles[role] = rc    // <-- REQUIRED; without it the set is silently lost. Preserves sibling Provider/Model.

// Single-commit Render — read cfg.Reasoning DIRECTLY (matches how it reads cfg.Model):
spec, rerr := deps.Manifest.Render(cfg.Model, sysPrompt, payload, cfg.Reasoning)
```

```go
// roles_test.go — the keystone new test (planner shipped default fires when nothing is set):
func TestResolveRoleModel_PlannerShippedDefault(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model/Reasoning all ""
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("planner provider/model = (%q,%q), want (\"\",\"\") [manifest sentinel]", p, m)
	}
	if r != "high" {
		t.Errorf("planner reasoning = %q, want \"high\" [FR-R6 shipped default, global unset]", r)
	}
	// message has NO shipped non-off default:
	_, _, rm := ResolveRoleModel("message", cfg)
	if rm != "" {
		t.Errorf("message reasoning = %q, want \"\" (off — no shipped non-off default)", rm)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. No new dependency; reasoning is plain-string plumbing. go mod tidy MUST be a no-op.

PACKAGE EDGES:
  - internal/config → (stdlib + go-toml/v2 + spf13/pflag) only. No new imports.
  - internal/provider NOT imported by config (unchanged). Render's reasoning param is the seam.

FROZEN / NOT-EDITED (do NOT touch):
  - internal/provider/render.go + manifest.go (P1.M1.T1.S1's Render reasoning param + ReasoningLevels are the INPUT).
  - internal/config/role_defaults.go (FR-D4 model table; defaultRoleReasoning is role-level, lives in roles.go).
  - internal/config/bootstrap.go (writes NO reasoning today; P1.M4.T2 may later write [role.planner].reasoning).
  - internal/config/config.go const CurrentConfigVersion (P1.M3.T1.S1 bumps it to 3).
  - docs/*.md (DOCS is Mode A — inline comments + flag help text only; the changeset doc sync is P4.M2.T1).
  - The 4 decompose role Render(...) reasoning args (S2 wires them).

DOWNSTREAM CONTRACT (hand-off to P1.M2.T1.S2 — do NOT implement here):
  - ResolveRoleModel now returns (provider, model, reasoning). S2's ResolveRoles consumes the 3rd value into
    RoleModels.<Role>.Reasoning and passes it to the 4 decompose Render(...) calls (replacing the `, _`
    discards from Task 7). S2 ALSO adds pkg/stagecoach RoleModel.Reasoning + applyRoleOverride.
  - The single-commit path (generate.go + pkg/stagecoach) already forwards cfg.Reasoning after S1 (done here).

NO DATABASE / NO ROUTES / NO NEW FILES / NO CLI WIRING BEYOND FLAG REGISTRATION.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/config/config.go internal/config/file.go internal/config/load.go internal/config/roles.go \
  internal/config/roles_test.go internal/cmd/root.go internal/generate/generate.go \
  internal/decompose/roles.go internal/decompose/planner.go internal/decompose/stager.go \
  internal/decompose/message.go internal/decompose/arbiter.go pkg/stagecoach/stagecoach.go
test -z "$(gofmt -l internal/ pkg/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...                       # Expect zero diagnostics (catches a missed arity call site).
go build ./...                     # Whole module compiles (the 5 decompose `, _` discards are load-bearing here).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If `go build` fails on a ResolveRoleModel call site, you missed one — every caller must
#   take 3 values now. grep -rn "ResolveRoleModel" should show NO 2-value receivers outside roles.go itself.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/config/ -v          # roles_test.go (3-return + reasoning) + file_test/load_test (reasoning cases)
go test -race ./internal/generate/ -v        # generate (reasoning now reaches Render; stub manifests unaffected)
go test -race ./internal/decompose/ -v       # the 5 `, _` discard sites compile + existing ResolveRoles tests stay green
go test -race ./pkg/stagecoach/ -v            # GenerateCommit forwards cfg.Reasoning
go test -race ./internal/cmd/ -v             # flag registration (--reasoning, --message-*, per-role) + load wiring
go test -race ./...                          # Full suite — NO regressions.
# Expected: all PASS. Key new assertions: planner→high shipped default (Defaults()); per-role reasoning
#   overrides global; STAGECOACH_REASONING / --reasoning / STAGECOACH_<ROLE>_REASONING / --<role>-reasoning
#   override at correct precedence; [defaults].reasoning / [role.X].reasoning decode+merge.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm no file outside the listed edits was touched (frozen-file gate):
git diff --name-only | grep -Ev 'internal/config/(config|file|load|roles|roles_test)\.go|internal/cmd/root\.go|internal/generate/generate\.go|internal/decompose/(roles|planner|stager|message|arbiter)\.go|pkg/stagecoach/stagecoach\.go|internal/config/(file_test|load_test)\.go' \
  && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
# Flag-surface sanity (FR-R3): all four roles incl. message expose reasoning + provider + model + a global.
/tmp/stagecoach --help 2>&1 | grep -E -- '--reasoning|--planner-reasoning|--stager-reasoning|--message-reasoning|--arbiter-reasoning|--message-provider|--message-model' \
  && echo "all reasoning + message-* flags registered"
# Resolve-precedence smoke (optional throwaway): build a pflag.FlagSet + cfg, set STAGECOACH_PLANNER_REASONING=high
# + [defaults].reasoning=off, call config.ResolveRoleModel("planner", cfg) → expect "high" (per-role beats global).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Precedence-matrix sanity (mirrors FR-R6/§16.4 for reasoning across layers). Optional: a /tmp Go snippet
# that builds a config.Config with layered [role.planner].reasoning / [defaults].reasoning and prints
# ResolveRoleModel(...) for each role — eyeball planner→high (shipped, when all empty), per-role>global,
# global>shipped. The in-package roles_test.go table already asserts this; the snippet is a belt-and-suspenders.
# Property-style invariant (optional): for any cfg, ResolveRoleModel("planner", cfg).reasoning is non-"" ONLY
# via "high" shipped default or an explicit set; ResolveRoleModel("message", cfg).reasoning is "" unless
# explicitly set (message ships off). golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/ pkg/`, `go mod tidy` no-op;
      `git diff --exit-code go.mod go.sum` empty.
- [ ] Level 2 green: `go test -race ./...` (config + generate + decompose + pkg/stagecoach + cmd).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only listed files changed; all 7 flags registered.

### Feature Validation

- [ ] `RoleConfig` + `Config` both have `Reasoning string toml:"reasoning"`; `Defaults()` sets it "".
- [ ] fileDefaults/fileRoleConfig have Reasoning; materialize + overlay thread it (global + per-role, non-zero).
- [ ] `setRoleReasoning` uses the write-back idiom; loadEnv/loadFlags handle global + per-role reasoning.
- [ ] `ResolveRoleModel` returns `(provider, model, reasoning)`; reasoning = per-role → global →
      `defaultRoleReasoning[role]` (planner=high).
- [ ] root.go registers `--reasoning` + 4×`--<role>-reasoning` + `--message-provider` + `--message-model`.
- [ ] generate.go + pkg/stagecoach single-commit Render passes `cfg.Reasoning`.
- [ ] 5 decompose call sites compile (`, _`); roles_test.go migrated (planner→high asserted).

### Code Quality Validation

- [ ] Mirrors existing patterns: plain-string field like Provider/Model; setRoleReasoning mirrors setRoleModel;
      materialize/overlay mirror Provider/Model branches; flag registration mirrors planner-model.
- [ ] Reasoning is a plain string (NOT *string); defaultRoleReasoning has only `{"planner":"high"}`.
- [ ] No scope creep into S2 (RoleModels.Reasoning / decompose Render reasoning args / pkg/stagecoach RoleModel)
      or P1.M3 (config_version bump) or P1.M4 (bootstrap).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] RoleConfig doc comment describes reasoning (Mode A); flag help text names env + git-config (Mode A).
- [ ] No docs/*.md edits (changeset doc sync is P4.M2.T1, Mode B).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't make Reasoning a `*string`/`*bool`. It's a plain string like Provider/Model; "off" is a non-empty
  literal so the non-zero file/overlay merge is correct. Pointers would force dereferencing everywhere for
  zero gain and break parity with Provider/Model.
- ❌ Don't apply the shipped `planner=high` default ABOVE the global `[defaults].reasoning`. The item + FR-R6
  make it the LOWEST fallback layer (per-role → global → shipped). `defaultRoleReasoning = {"planner":"high"}`
  only; off is the zero value.
- ❌ Don't route the single-commit path (generate.go / pkg/stagecoach) through `ResolveRoleModel`. Those paths
  read `cfg.Model` directly; read `cfg.Reasoning` directly too (4th Render arg). For the message role it's
  identical anyway (message ships off="").
- ❌ Don't forget the map-value-copy write-back in `setRoleReasoning` (`c.Roles[role] = rc`). Without it the
  set is silently lost (Go maps return copies) — setRoleProvider/setRoleModel are the reference.
- ❌ Don't leave a 2-value `ResolveRoleModel` receiver anywhere — the arity change breaks compilation. Update
  all 5 non-test + 7 test call sites. (`grep -rn "ResolveRoleModel" --include=*.go` must show no 2-value use.)
- ❌ Don't wire the 4 decompose role `Render(...)` reasoning args here (they stay ""). That is S2's
  (P1.M2.T1.S2) scope — keep S1/S2 non-overlapping. S1's decompose touch is ONLY the `, _` arity discard.
- ❌ Don't register `--message-reasoning` while leaving `--message-provider`/`--message-model` unregistered —
  FR-R3 requires all three flags for all four roles. Close the gap fully.
- ❌ Don't bump `CurrentConfigVersion`, edit `bootstrap.go`, edit `role_defaults.go`, edit `render.go`/
  `manifest.go`, or edit `docs/*.md` — those are other subtasks (P1.M3.T1.S1 / P1.M4.T2 / P2 / P1.M1.T1 /
  P4.M2.T1). S1 is config reasoning plumbing only.
- ❌ Don't change go.mod/go.sum or add new files. This is pure in-place plumbing of an existing plain field.
- ❌ Don't skip `go vet`/`go build`/`gofmt`/`go test -race ./...` — they catch a missed arity call site, the
  write-back omission, and formatting drift before S2 freezes on the 3-return signature.
