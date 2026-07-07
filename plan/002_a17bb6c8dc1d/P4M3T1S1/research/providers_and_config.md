# Providers & Config — Exact Source Facts (for docs)

Working dir: `/home/dustin/projects/stagecoach`. All identifiers quoted verbatim from source.

> **File-layout note for the writer:** the task prompt referred to "each builtin: builtinPi/Claude/Gemini/Opencode/Codex/Cursor/Agy" as if they were separate files. They are NOT — all seven `builtinX()` constructors live in a single file `internal/provider/builtin.go`. `BuiltinManifests()` (same file) is the map keyed by name. There is no per-provider file.

---

## 1. `preferredBuiltins` — cascading auto-detect priority

**Source: `internal/provider/registry.go` (the authoritative copy; a test `TestPreferredBuiltins_MatchesBuiltinKeys` enforces sync with `BuiltinManifests()` keys).**

```go
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
```

**Verdict: PRD §9.16's stated order `pi, opencode, cursor, agy, gemini, codex, claude` is CORRECT and matches the source exactly.** No correction needed.

A second, LOCAL mirror copy exists in `internal/config/bootstrap.go` (same values, same order) — used by the bootstrap's stager-fallback + commented-block ordering. Comment: "local copy — mirrors internal/provider/registry.go's unexported preferredBuiltins".

`DefaultProvider(installed []string)` and `FirstTooledProvider(installed []string)` both walk `preferredBuiltins` in order; both return `""` if none of the preferred built-ins are installed. User-defined §12.8 providers are NEVER auto-selected (only built-in names are candidates).

---

## 2. Per-builtin provider table (all from `internal/provider/builtin.go`)

`DetectCommand()` = `Detect` if non-empty, else `Command` (manifest.go). "stager-capable?" = `len(TooledFlags) > 0` (the only signal). `Experimental` shown as its *bool value (after Resolve → `*false` when nil).

| Name      | detect | command | default_model     | default_provider | model_flag | prompt_delivery | tooled_flags non-empty? (stager-capable) | experimental |
|-----------|--------|---------|-------------------|------------------|------------|-----------------|------------------------------------------|--------------|
| `pi`      | `pi`   | `pi`    | `""` (non-nil)    | `""` (non-nil)   | `--model`  | `stdin`         | **YES** (5 flags)                        | false        |
| `claude`  | `claude`| `claude`| `sonnet`          | NIL (omitted)    | `--model`  | `stdin`         | **YES** (3 flags)                        | false        |
| `gemini`  | `gemini`| `gemini`| `gemini-2.5-pro`  | NIL (omitted)    | `-m`       | `stdin`         | NO (nil)                                 | false        |
| `agy`     | `agy`   | `agy`   | `gemini-2.5-pro`  | NIL (omitted)    | `-m`       | `stdin`         | NO (nil)                                 | **true**     |
| `opencode`| `opencode`| `opencode`| `""` (non-nil) | NIL (omitted)    | `-m`       | `positional`    | NO (nil)                                 | false        |
| `codex`   | `codex` | `codex` | `""` (non-nil)    | NIL (omitted)    | `-m`       | `stdin`         | NO (nil)                                 | false        |
| `cursor`  | `agent` | `agent` | `""` (non-nil)    | NIL (omitted)    | `--model`  | `positional`    | NO (nil)                                 | false        |

Key correctness nuances (quote these in docs):
- **cursor is the ONLY provider where `detect ≠ name`**: `Detect="agent"`, `Command="agent"` (the binary is `agent`). `IsInstalled` probes `agent`, not `cursor`.
- **pi's `default_model` AND `default_provider` are both NON-NIL `""`** (FR-D2 decoupled from any one subscription). Distinct from nil (absent). The old `provider=zai, model=glm-5-turbo` is a documented PERSONAL OVERRIDE, not the shipped default.
- **claude's `default_provider` is NIL (key omitted)**; `provider_flag` is non-nil `""` (`n/a`).
- **stager-capable today = only `pi` + `claude`** (only these two set `TooledFlags`). All others have nil → `RenderTooled` errors → cannot serve as stager.

Exact `TooledFlags` contents:
- **pi**: `["--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]` (bare MINUS `--no-tools` — pi's native tool system ON).
- **claude**: `["--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"]` (INVERTS bare mode: tools enabled, allowlisted to git+Read+Edit).

Exact `BareFlags` for reference (used for the planner/message/arbiter bare roles):
- pi: `["--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]`
- claude: `["--tools","","--setting-sources","","--no-session-persistence"]`
- gemini: `["--approval-mode","default"]`
- agy: `["--approval-mode","default"]`
- opencode: `[]` (non-nil empty slice)
- codex: `["--sandbox","read-only","--ephemeral"]`
- cursor: `["--mode","ask","--trust"]`

Other notable fields:
- `subcommand`: opencode=`["run"]`, codex=`["exec"]`, cursor=`[]` (non-nil empty). pi/claude/gemini/agy = nil.
- `print_flag`: pi=`-p`, claude=`-p`, agy=`-p`, cursor=`-p`; gemini/opencode/codex = non-nil `""`.
- `output` = `raw` (all seven); `strip_code_fence` = `*true` (all seven).

---

## 3. `GenerateBootstrapConfig` / `config init` output

**Source: `internal/config/bootstrap.go`.**

```go
func GenerateBootstrapConfig(prov string) string
```
- `prov != ""` → used directly. `prov == ""` → cascading auto-detect via `reg.DefaultProvider(installed)` (walks `preferredBuiltins`); if nothing on `$PATH` → `"pi"` fallback (annotated in output). NO I/O in this func; `$PATH` detection via the registry.

### `CurrentConfigVersion` constant
**Source: `internal/config/config.go`:**
```go
const CurrentConfigVersion = 2
```
Comment: "v2 = per-role models + multi-commit decomposition + binary filtering."

### What populated `config init` writes (`buildBootstrapConfig`)
Order/sections written (deterministic, pure):
1. **`bootstrapHeader`** — a large commented block (precedence, env vars `STAGECOACH_*`, git config keys, CLI flags). Quoted constant, see source.
2. **`config_version = 2`** — **UNCOMMENTED** (F6). `fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)`.
3. **`[defaults]` block** — `provider = "<target>"` UNCOMMENTED (with an inline comment `# no built-in agent detected on $PATH; defaulted to "pi" …` only when target not installed); the rest commented: `# model = ""`, `# timeout = "120s"`, `# auto_stage_all = true`, `# verbose = false`.
4. **Four `[role.*]` blocks for the target** — UNCOMMENTED, canonical order **planner, stager, message, arbiter**, models from `DefaultModelsForProvider(target)`:
   - `planner` — inherits `[defaults]` provider (provider line OMITTED).
   - `stager` — may fall back to a DIFFERENT provider via `stagerFallback` (annotated with a comment when `stagerName != target`, e.g. `<target> cannot serve as the stager (no tooled_flags); routed to <stagerName> (the first stager-capable provider).`). `stagerFallback` always resolves to `pi` today.
   - `message` — inherits `[defaults]` provider.
   - `arbiter` — inherits `[defaults]` provider.
5. **Each OTHER installed provider** (iterating `preferredBuiltins`, skipping target + non-installed) — a fully **COMMENTED** `[role.*]` group (planner/stager/message/arbiter) with a `# === <name> (installed) — uncomment … ===` header.
6. **`generationCommented`** — a fully commented `[generation]` section (`max_diff_bytes`, `max_md_lines`, `max_duplicate_retries`, `subject_target_chars`, `output`, `strip_code_fence`, `max_commits`, `binary_extensions`).

**So: YES it writes `config_version = 2` (uncommented). YES it writes `[defaults]` + four `[role.*]` blocks for the target.** Bootstrap auto-detect order = `preferredBuiltins` = pi, opencode, cursor, agy, gemini, codex, claude.

`bootstrapWriteConfig(path)` is the `Load()` first-run fallback (FR-B3) caller of `GenerateBootstrapConfig("")`.

### Per-provider default-model columns (`internal/config/role_defaults.go`)

`RoleModelDefaults` = `map[string]map[string]string` (provider → role → model). `DefaultModelsForProvider(name)` returns a COPY or nil. Stager cell `""` ⇔ not stager-capable (matches builtin.go `TooledFlags`).

| provider  | planner             | stager        | message              | arbiter              |
|-----------|---------------------|---------------|----------------------|----------------------|
| pi        | `gpt-5.4`           | `gpt-5.4-mini`| `gpt-5.4-nano`       | `gpt-5.4-mini`       |
| claude    | `opus`              | `sonnet`      | `haiku`              | `sonnet`             |
| gemini    | `gemini-3.5-pro`    | `""`          | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| agy       | `gemini-3.5-pro`    | `""`          | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| opencode  | `openai/gpt-5.4`    | `""`          | `openai/gpt-5.4-nano`| `openai/gpt-5.4-mini`|
| codex     | `gpt-5.1-codex-max` | `""`          | `gpt-5.4-nano`       | `gpt-5.1-codex-mini` |
| cursor    | `gpt-5.4`           | `""`          | `gpt-5.4-nano`       | `gpt-5.4-mini`       |

(FR-D5 note in source: cursor models are "best-guess OpenAI tokens" — UNVERIFIED against `agent --help`.)

---

## 4. `config upgrade` (`upgradeConfigVersion`) — exact output

**Source: `internal/cmd/config.go`.** Command `Use: "upgrade"`. `runConfigUpgrade(cmd, args)` does I/O; `upgradeConfigVersion(content, version)` is the pure transform.

### `runConfigUpgrade` stdout/exit behavior

- **No config file** (`os.IsNotExist`): returns `exitcode.New(exitcode.Error, …)` → prints (to stderr):
  `no config file at <path> (run 'stagecoach config init' first)` — exit 1.
- **Not valid TOML**: `config <path> is not valid TOML: <err>` — exit 1, file untouched.
- **Already current** (`changed == false`): prints to **stdout** (exit 0):
  `Config at <path> is already at version 2 (no changes).` — file byte-identical.
- **Upgraded** (`changed == true`): writes file, prints to **stdout** (exit 0):
  `Upgraded config at <path> to version 2.`

`path` = `config.GlobalConfigPath()` (the GLOBAL config). Works outside a git repo.

### `upgradeConfigVersion(content, version)` outcomes (pure; comment block quoted)
- found with value == version → content unchanged, `changed=false`
- found with value != version → that ONE line rewritten to `config_version = <version>`, `changed=true`
- not found → one `config_version = <version>` line inserted after the leading comment/blank header block, `changed=true`

It scans ONLY the top-level region (stops at the first `[table]` header via `isTableHeader`). Top-level regex: `` ^config_version\s*=\s*([0-9]+) `` (anchored col 0; commented lines ignored). v2.0 has no removed/renamed keys → no other line touched.

### Load-time advisory (`configVersionNotice`, `internal/config/load.go`) — for cross-reference
Emitted to `noticeOut` (stderr) after all overlays, happy path only, `fileLoaded && version != CurrentConfigVersion`:
- `version == 0`: `stagecoach: config file has no config_version; current is 2. Run 'stagecoach config upgrade' or 'stagecoach config init --force'.`
- `version < 2`: `stagecoach: config file uses schema version <v>; current is 2. Run 'stagecoach config upgrade' or 'stagecoach config init --force'.`
- `version > 2`: `stagecoach: config file uses schema version <v>; this binary supports up to 2. Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.`
- `!fileLoaded` or current → `""` (silent).

---

## 5. The four decompose roles + flow

### Role names (single source: `internal/config/load.go`)
```go
var roleNames = []string{"planner", "stager", "message", "arbiter"}
```
(Canonical order, used by env/flag per-role overlays.)

### Flow confirmation (planner → stager → message → arbiter)
**Source: `internal/decompose/decompose.go` — `func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error)`.**

Confirmed pipeline (PRD §13.6 / §11.4 / §9.14):
1. **Mode routing (FR-M2):** `Config.Single || Config.Commits == 1` → single ESCAPE-HATCH (`runSingleEscape`: planner BYPASSED → `AddAll` → `generate.CommitStaged`, the v1 path). Else continue.
2. **callPlanner** (`forcedCount = Config.Commits`: 0=auto, ≥2=forced). callPlanner enforces the FR-M4 safety cap in auto mode. Planner failure is NON-RESCUE.
3. **FR-M11 single-SHORTCUT:** if planner returns `Single==true` + a Message → `runSingleShortcut` (use planner's message directly, dup-check first, fallback to message agent only on dup).
4. **`runLoop`** — the per-concept pipeline with **1-DEEP overlap** (stager[i+1] ∥ message[i]) + **FR-M8 empty-skip** + serialized CAS publication + FR-M12 per-concept isolation:
   - per concept i: `invokeStagerRetry` (FR-M12d: retry-once-then-empty) → `freezeSnapshot` (tree[i]) → FR-M8 skip if tree[i]==prevTree → publish previous msg[i-1] → launch msg[i].
   - final: drain+publish msg[N-1].
5. **Arbiter gate (FR-M9):** after the loop, `StatusPorcelain != ""` AND `len(commits) > 0` → `runArbiterPhase` (`runArbiter` → `resolveArbiter`). Computes `Amended` count via `findTargetIndex` BEFORE resolveArbiter. Arbiter does NOT run on a loop abort (rescue/CAS/hard error).

### Role resolution (`internal/decompose/roles.go`)
`func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)` — per role {planner,stager,message,arbiter}:
1. `config.ResolveRoleModel(role, cfg)` — field-merge (role → global → manifest-default sentinel).
2. auto-detect provider via `reg.DefaultProvider(installed)` if `""`.
3. `reg.Get → Validate → IsInstalled`.
4. **FR-R5b guard:** bare model + no provider + multi-provider manifest (`isMultiProvider` = `ProviderFlag != nil && *ProviderFlag != ""` → only `pi` today) → config error.
5. **FR-D4 stager fallback:** `role=="stager" && len(m.TooledFlags)==0` → `reg.FirstTooledProvider(installed)`; switch BOTH provider AND model (`config.DefaultModelsForProvider(fb)["stager"]`).

`RoleManifests` = `{Planner, Stager (tooled), Message, Arbiter}` (all bare except Stager which carries the post-fallback tooled manifest). `RoleModels` = four `config.RoleConfig{Provider, Model}`.

### Public `Decompose()` signature — `pkg/stagecoach/stagecoach.go`
```go
// Stable as of v2.0.
func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error)
```
- **NO-OP delegation:** `opts.Single || opts.Count == 1` → `GenerateCommit(ctx, opts.Options)`; returns `DecomposeResult{Commits: []Result{r}, Amended: 0, Provider: r.Provider}`. `opts.DryRun` honored ONLY on this single path.
- **Multi-commit path:** `resolveDecomposeConfig` → `provider.DecodeUserOverrides(cfg.Providers)` → `provider.NewRegistry` → `decompose.ResolveRoles(cfg, reg)` → build `decompose.Deps{Git, Registry, Config, Roles, Verbose, Out}` → `decompose.Decompose(ctx, deps)` → `mapDecomposeResult`.

`DecomposeOptions` struct (v2.0): embeds `Options` (apply to MESSAGE role) + `Count int` (0=auto, >0=force) + `Single bool` + `MaxCommits int` (0=inherit, default 12) + `Planner/Stager/Arbiter RoleModel` (zero ⇒ global default).

`DecomposeResult` struct (v2.0): `Commits []Result`, `Amended int`, `Provider string` (resolved MESSAGE provider).

PRECONDITION (FR-M1, owned by the CLI router): the caller must ensure NOTHING is staged — `Decompose` does NOT re-check `HasStagedChanges`.

---

## Quick reference — key identifiers

- `preferredBuiltins` — `internal/provider/registry.go` (authoritative) + mirror in `internal/config/bootstrap.go`.
- `BuiltinManifests()` / `builtinPi()/builtinClaude()/builtinGemini()/builtinAgy()/builtinOpenCode()/builtinCodex()/builtinCursor()` — all in `internal/provider/builtin.go`.
- `Manifest` struct + `Validate()`/`DetectCommand()`/`Resolve()` — `internal/provider/manifest.go`.
- `MergeManifest(base, override)` — `internal/provider/merge.go`.
- `Registry` (`NewRegistry`/`Get`/`List`/`IsInstalled`/`MarshalTOML`/`DefaultProvider`/`FirstTooledProvider`/`DecodeUserOverrides`) — `internal/provider/registry.go`.
- `CurrentConfigVersion = 2` — `internal/config/config.go`.
- `GenerateBootstrapConfig(prov)` / `buildBootstrapConfig(target, installed)` / `bootstrapHeader` / `generationCommented` / `stagerFallback` / `DefaultModelsForProvider(name)` / `RoleModelDefaults` / `roleDefaults` — `internal/config/bootstrap.go` + `internal/config/role_defaults.go`.
- `configUpgradeCmd` / `runConfigUpgrade` / `upgradeConfigVersion` / `configVersionLineRe` / `isTableHeader` / `leadingHeaderEnd` — `internal/cmd/config.go`.
- `configVersionNotice` — `internal/config/load.go`.
- `roleNames` — `internal/config/load.go`.
- `decompose.Decompose` / `DecomposeResult` / `CommitResult` / `DecomposeRescueError` / `Deps` — `internal/decompose/decompose.go`.
- `decompose.ResolveRoles` / `RoleManifests` / `RoleModels` — `internal/decompose/roles.go`.
- `stagecoach.Decompose` / `DecomposeOptions` / `DecomposeResult` / `GenerateCommit` / `Options` / `Result` / `RoleModel` — `pkg/stagecoach/stagecoach.go`.
