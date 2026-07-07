# Config Model & Loaders — Scout Map (v2→v3 migration)

Scope: removing `default_provider`, adding per-role `reasoning`, bumping
`CurrentConfigVersion` 2→3. All paths relative to repo root.

---

## (a) `Config` struct — `internal/config/config.go`

**File:** `internal/config/config.go:60-129` (`type Config struct`).

Fields (grouped):
- `[defaults]` — `Provider string` (L67), `Model string` (L69), `Timeout time.Duration` (L70), `AutoStageAll bool` (L71), `Verbose bool` (L72).
- CLI/UI only (`toml:"-"`) — `NoColor bool` (L75), `Commits int` (L78), `Single bool` (L79).
- `[generation]` — `MaxDiffBytes int` (L82), `MaxMdLines int` (L83), `MaxDuplicateRetries int` (L84), `SubjectTargetChars int` (L85), `Output *string` (L86), `StripCodeFence *bool` (L87), `MaxCommits int` (L91), `BinaryExtensions []string` (L92).
- `Providers map[string]map[string]any \`toml:"-"\`` (L100) — RAW provider map (no manifest import). Field-level merged in `overlay`.
- `Roles map[string]RoleConfig \`toml:"-"\`` (L109) — per-role overrides.
- `ConfigVersion int \`toml:"config_version"\`` (L115) — top-level metadata.

**Migration touchpoints for per-role `reasoning`:** add a field to `RoleConfig`, to `Config` (global default, e.g. a new `[defaults] reasoning` or generation-style global), thread through `materialize`/`overlay`/`setRole*`, `ResolveRoleModel` return shape, and `Deps`/`RoleModels` in decompose. See (b).

`Defaults()` at `config.go:121-152` — all default values; add `reasoning` default here.

`RoleConfig` struct — **`config.go:43-46`:**
```go
type RoleConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}
```
**Adding `reasoning`** = add a 3rd field here. This struct is the per-role carrier.

---

## (b) `RoleConfig` construction / population sites

| Site | file:line | How |
|---|---|---|
| File decode twin | `internal/config/file.go:21-24` (`fileRoleConfig`) | TOML decode of `[role.<r>]` → `{Provider, Model}`. **Add `Reasoning` here + a `toml:"reasoning"` tag.** |
| fileConfig table map | `internal/config/file.go:33` (`Role map[string]fileRoleConfig`) | top-level `[role]` table |
| `materialize` file→typed | `internal/config/file.go:202-208` | `c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model}` — **add `Reasoning: frc.Reasoning`** |
| `overlay` role field-merge | `internal/config/file.go:284-296` | per-role field-merge; **add a `Reasoning != ""` branch** |
| `setRoleProvider` (env/flag) | `internal/config/load.go:33-40` | sets `.Provider` only |
| `setRoleModel` (env/flag) | `internal/config/load.go:45-52` | sets `.Model` only — **add `setRoleReasoning` (or extend) for `STAGECOACH_<ROLE>_REASONING` + `--<role>-reasoning`** |
| decompose `setRole` | `internal/decompose/roles.go:193-204` | `rc := config.RoleConfig{Provider: prov, Model: mdl}` — **add reasoning from `ResolveRoleModel`** |
| decompose `RoleModels` | `internal/decompose/roles.go:38-47` | `RoleModels{Planner,Stager,Message,Arbiter config.RoleConfig}` — carries the resolved per-role config |
| pkg stagecoach overrides | `pkg/stagecoach/stagecoach.go:263-279` (`applyRoleOverride`) | programmatic RoleModel→cfg.Roles; takes a `RoleModel{Provider,Model}` — **extend `RoleModel`** |

---

## (c) `ResolveRoleModel` signature + callers

**Definition:** `internal/config/roles.go:28`
```go
func ResolveRoleModel(role string, cfg Config) (provider, model string)
```
Returns ONLY `(provider, model)`. **For `reasoning`** this signature must change to a 3-return (or return a `RoleConfig`). Currently it returns two bare strings.

**Callers (non-test):**
- `internal/decompose/arbiter.go:82` — `_, mdl := config.ResolveRoleModel("arbiter", deps.Config)`
- `internal/decompose/message.go:103` — `_, mdl := config.ResolveRoleModel("message", deps.Config)`
- `internal/decompose/planner.go:62` — `_, mdl := config.ResolveRoleModel("planner", deps.Config)`
- `internal/decompose/stager.go:78` — `_, mdl := config.ResolveRoleModel("stager", deps.Config)`
- `internal/decompose/roles.go:96` — `prov, mdl := config.ResolveRoleModel(role, cfg)` (the `ResolveRoles` loop, the authoritative 4-role resolver)

**Single-commit path** (`pkg/stagecoach`, `internal/cmd/default_action.go`) does NOT call `ResolveRoleModel` — it reads `cfg.Model`/`cfg.Provider` directly via `buildDeps`. So a global `reasoning` default must be read there too.

Each decompose caller then calls `deps.Roles.<Role>.Render(mdl, "", sysPrompt, payload, mode)` — note Render is called with `provider=""` (FR-R5b: the manifest's `default_provider` supplies the inference backend). **This is exactly what changes in v3** (the slash-prefix on model replaces `default_provider`).

---

## (d) `CurrentConfigVersion` + `config_version` read/compare/write

**Value:** `internal/config/config.go:13` → `const CurrentConfigVersion = 2`. **Bump to 3.**

| Site | file:line | Role |
|---|---|---|
| const definition | `config.go:13` | `= 2` |
| `fileConfig` decode field | `file.go:28` (`ConfigVersion int \`toml:"config_version"\``) | file decode |
| `materialize` copy | `file.go:199-201` | `if fc.ConfigVersion != 0 { c.ConfigVersion = fc.ConfigVersion }` |
| `overlay` copy | `file.go:299-301` | non-zero wins |
| load-time advisory | `load.go:259` (`configVersionNotice(fileLoaded, cfg.ConfigVersion)`) + impl `load.go:265-285` | compares to `CurrentConfigVersion`, prints older/newer/missing notice to `noticeOut` |
| `Load` call site | `load.go:255` | after all overlays, happy path |
| bootstrap writer | `bootstrap.go:118` | `fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)` |
| `config upgrade` cmd | `internal/cmd/config.go:96` (Long text), `:117` (`configVersionLineRe`), `:155` (`upgradeConfigVersion(string(data), config.CurrentConfigVersion)`), `:157`,`:163` (print) | rewrites top-level line in place |
| `upgradeConfigVersion` | `internal/cmd/config.go:178-204` | pure textual edit — **v3 migration (FR-B7) needs more than a version bump here: it must also rewrite `default_provider`→model-prefix and drop the key. This is the future extension point.** |
| `config init` writer | `internal/cmd/config.go:runConfigInit` (~L288-330) → `config.GenerateBootstrapConfig` | writes via bootstrap |
| first-run auto-bootstrap | `load.go:79-91` (`bootstrapWriteConfig`) | `Load` writes then loads |
| test fixtures | `internal/cmd/config_test.go` (many: L162,180,218,223,287,340,764,808-916,986,1013,1030), `internal/cmd/default_action_test.go:1203` | hardcode `config_version = 2` — **must update to 3** |

The load-time advisory (`configVersionNotice`, `load.go:265-285`) is the v3 *auto-migrate in-memory* trigger point per FR-B7 (delta_prd.md R1.2): on `version < 3`, mutate the resolved `Config` in memory (prepend `default_provider` to model strings) + emit a deprecation notice.

---

## (e) `fileConfig`/`fileRoleConfig` + `materialize`/`overlay`

**File:** `internal/config/file.go`.

- `fileRoleConfig` — `file.go:21-24` (`{Provider, Model}`).
- `fileConfig` — `file.go:26-35`:
  - `ConfigVersion int` (L28), `Defaults fileDefaults` (L29), `Generation fileGeneration` (L30), `Role map[string]fileRoleConfig` (L33), `Provider map[string]map[string]any` (L35).
- `fileDefaults` — `file.go:37-43` (`Provider, Model, Timeout string, AutoStageAll, Verbose`). **Global `reasoning` default would go here + a `fileConfig` global field.**
- `fileGeneration` — `file.go:45-54`.

**`materialize(fc, timeout) *Config`** — `file.go:143-211`:
- copies non-zero scalars; `Roles` conversion at `L202-208`; `c.Providers = fc.Provider` at `L210`.
- **v3 migration note:** materialize is where `default_provider`-to-model-prefix could be folded for the raw `Providers` map and the `Roles` models, but the cleaner spot is the load-time advisory in `Load` (operates on the fully-resolved `Config`).

**`overlay(dst, src *Config)`** — `file.go:219-303`:
- `[defaults]` L223-235; `[generation]` L237-252; `[provider.X]` field-merge L266-278; `[role.<r>]` field-merge L284-296; scalars L299-303.

**Load layering** — `internal/config/load.go:64-127` (`Load`):
1. `Defaults()` (L66)
2. global TOML via `loadTOML` (L73-93) — with first-run `bootstrapWriteConfig` fallback (L79-91)
3. repo-local `loadRepoLocalConfig` (L97-103)
4. git config `loadGitConfig` (L107-112)
5. env `loadEnv` (L116-119)
6. flags `loadFlags` (L122-124)
7. `Commits==1 ⇒ Single` normalize (L130-133)
8. `configVersionNotice` (L137-139)

---

## (f) `[provider.<name>] default_provider` decode/use path (RAW map)

Config does NOT have a typed `default_provider` field — it flows as **raw `any`** in `Config.Providers` (`map[string]map[string]any`):

- Decode: `file.go:35` `Provider map[string]map[string]any` → `file.go:210` `c.Providers = fc.Provider`.
- Field-merge across layers: `file.go:266-278`.
- Bridge to typed Manifest: **`provider.DecodeUserOverrides`** (`internal/provider/registry.go:147-167`) — re-encodes each `map[string]any` to TOML, unmarshals into `Manifest` (which has `DefaultProvider *string \`toml:"default_provider"\``, `internal/provider/manifest.go:59`).
- Callers of `DecodeUserOverrides(cfg.Providers)`:
  - `pkg/stagecoach/stagecoach.go:183`, `:311`
  - `internal/cmd/default_action.go:131`, `:271`
  - `internal/cmd/providers.go:114` (`raw = cfg.Providers`)
- Registry merge: `provider.NewRegistry` (`registry.go:46-65`) + `MergeManifest` (`merge.go:56-60` — `DefaultProvider` field-merge).
- Manifest field: **`internal/provider/manifest.go:58-59`**:
  ```go
  ProviderFlag    *string `toml:"provider_flag"`
  DefaultProvider *string `toml:"default_provider"`
  ```
- Consumed at Render: **`internal/provider/render.go:93-96`** (`providerToUse = *r.DefaultProvider`), token emit at `render.go:117-119` (`args = append(args, *r.ProviderFlag, providerToUse)`), and the FR-R5b backstop `render.go:107-111`.
- Consumed by decompose `inferenceProvider` guard: `internal/decompose/roles.go:182-187` (`*m.Resolve().DefaultProvider`).
- Built-in defaults: `internal/provider/builtin.go` — pi `DefaultProvider: strPtr("")` (L50, non-nil empty); all others **nil** (omitted).

**v3 removal:** the manifest field `DefaultProvider` is to be **removed**; the inference backend moves to a slash-prefix on `model`. `Config.Providers` raw map keeps carrying whatever the user wrote (the field-level map merge is generic), so config itself needs NO change to drop `default_provider` — but the load-time in-memory migration (FR-B7) must rewrite `Providers[<name>]["default_provider"]` into the model prefix and delete the key, AND rewrite per-role/global models. Touch: `materialize` or a new migrate step in `Load`.

Reference TOML files carrying `default_provider`: `providers/pi.toml:52` (`default_provider = ""`). Others explicitly note absence (agy/cursor/codex/opencode/gemini/claude `.toml`).

---

## (g) `config init` bootstrap writer

**Pure generator:** `GenerateBootstrapConfig(prov)` → `buildBootstrapConfig(target, installed)` — `internal/config/bootstrap.go`.

`buildBootstrapConfig` (`bootstrap.go:108-180`):
- Header (`bootstrapHeader` const, `bootstrap.go:184-225`).
- `config_version = CurrentConfigVersion` (`bootstrap.go:118`).
- `[defaults] provider = <target>` uncommented (`bootstrap.go:121-129`); `model`/`timeout`/`auto_stage_all`/`verbose` COMMENTED.
- pi blanking: `bootstrap.go:131-143` — when `target=="pi"`, all `models[role]=""` (pi needs `default_provider` to route). **This pi-specific blanking block + its NOTE (`bootstrap.go:139-141`) is v3-affected:** in v3 there's no `default_provider`, so pi models can carry a slash-prefix; the blanking rationale changes.
- Four `[role.*]` blocks via `writeRoleBlock` (`bootstrap.go:78-87`): planner/stager/message/arbiter. **`writeRoleBlock` writes only `provider` + `model`** — **add a `reasoning` line if a per-role reasoning default is desired.**
- Other installed providers as commented `[role.*]` groups via `writeCommentedRoleBlock` (`bootstrap.go:90-95`).
- Commented `[generation]` (`generationCommented`, `bootstrap.go:227-238`).

`stagerFallback` (`bootstrap.go:69-77`) + `DefaultModelsForProvider` (`role_defaults.go:90-104`, table at `role_defaults.go:32-87`).

**Writer (I/O):** `bootstrapWriteConfig(path)` (`bootstrap.go:39-50`) — MkdirAll + WriteFile. Called from `Load` first-run (`load.go:79`) and NOT from the `config init` command (that one calls `GenerateBootstrapConfig` then writes itself — `internal/cmd/config.go:runConfigInit`, ~L312-322).

**It writes `default_provider`: NO** — it never writes a `default_provider` key (pi's is intentionally left unset). So removing `default_provider` needs no bootstrap write change there; only the **pi-blanking NOTE text** (`bootstrap.go:139-141`) and possibly the role block content change for v3.

---

## (h) load.go env + flag wiring

**File:** `internal/config/load.go`.

`roleNames` — `load.go:14`: `[]string{"planner", "stager", "message", "arbiter"}` (loop target for env+flag per-role).

**Env (`loadEnv`, `load.go:157-202`):**
- `STAGECOACH_PROVIDER` (L161), `STAGECOACH_MODEL` (L165), `STAGECOACH_TIMEOUT` (L169), `STAGECOACH_VERBOSE` (L177), `STAGECOACH_NO_COLOR` (L185).
- Per-role loop `loadEnv:190-194`: `STAGECOACH_<ROLE>_PROVIDER` → `setRoleProvider`; `STAGECOACH_<ROLE>_MODEL` → `setRoleModel`. **Add `STAGECOACH_<ROLE>_REASONING` → `setRoleReasoning` here.**
- `STAGECOACH_COMMITS` (L199-203).

**Flags (`loadFlags`, `load.go:210-262`):**
- `provider` (L213), `model` (L219), `timeout` (L225), `verbose` (L233), `no-color` (L239).
- Per-role loop `loadFlags:243-252`: `<role>-provider` → `setRoleProvider`; `<role>-model` → `setRoleModel`. **Add `<role>-reasoning` → `setRoleReasoning` here.**
- `commits` (L256), `single`/`no-decompose` (L260), `max-commits` (L263... actually L263 in this file is the closing of loadFlags; max-commits is at the end ~L263-265).

**Flag REGISTRATION** (so `fs.Changed` works) — `internal/cmd/root.go:108-133`:
- `commits` (L108), `single` (L110), `no-decompose` (L112), `max-commits` (L114), `planner-provider/model` (L123-126), `stager-provider/model` (L127-130), `arbiter-provider/model` (L131-133). **No `message-*` flags registered** (loadFlags still loops but `fs.Changed("message-provider")` is always false). **Add `<role>-reasoning` flag registration here for the new field.**

**Helpers:** `setRoleProvider` (`load.go:33-40`), `setRoleModel` (`load.go:45-52`) — value-copy write-back idiom (maps return copies). **Mirror for `setRoleReasoning`.**

---

## Architecture summary

```
Defaults() ─┐
            ├─ Load() [load.go] ─► fully-resolved *Config
[global TOML]─loadTOML─materialize─overlay ─┐
[repo .stagecoach.toml]──────────────────────┤
[git config stagecoach.*]────────────────────┤   (all via overlay: non-zero field-merge)
[STAGECOACH_* env]───────────────────────────┤   (DIRECT set: bools can be false)
[--flags (fs.Changed only)]─────────────────┘

Config.Providers (raw map) ─► provider.DecodeUserOverrides ─► NewRegistry (MergeManifest)
Config.Roles (typed) ─────► ResolveRoleModel(role) ─► (provider,model) ─► Render(model, "", ...)
                                                                       ▲
                                  manifest.DefaultProvider supplies inference backend (REMOVED in v3)
```

**v3 migration load-bearing touchpoints (concentrated):**
1. `config.go:13` bump `CurrentConfigVersion = 3`.
2. `config.go:43-46` add `Reasoning` to `RoleConfig` (+ global default field on `Config` if needed).
3. `roles.go:28` change `ResolveRoleModel` to return reasoning too (3-return or `RoleConfig`).
4. `roles.go:96` + the 4 decompose callers (arbiter/message/planner/stager `.go`) adapt to new return.
5. `file.go:21-24` + `file.go:202-208` + `file.go:284-296` thread `reasoning` through decode/materialize/overlay.
6. `load.go:190-194` + `load.go:243-252` + new `setRoleReasoning` + `root.go` flag registration.
7. `bootstrap.go:78-87` (`writeRoleBlock`) + pi-blanking NOTE + `config_version` line.
8. FR-B7 in-memory migrate: a new step in `Load` (or `materialize`) triggered by `configVersionNotice` path: prepend `default_provider` → model prefix, drop the key, in `Config.Providers`/`Config.Roles`/`Config.Model`.
9. `cmd/config.go:178-204` `upgradeConfigVersion` — extend for v3 (rewrite file, not just bump).
10. `manifest.go:58-59` remove `DefaultProvider`; `render.go:93-119` slash-prefix split; `decompose/roles.go:182-187` `inferenceProvider` guard rework.

## Start Here
`internal/config/config.go` — `CurrentConfigVersion` (L13), `RoleConfig` (L43-46), `Config` (L60-129). Then `internal/config/roles.go` (`ResolveRoleModel` L28) and `internal/config/file.go` (`fileRoleConfig` L21 + `materialize`/`overlay`).
