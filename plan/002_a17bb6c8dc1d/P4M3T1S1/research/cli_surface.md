# CLI Surface — Exact Facts from Source

Source files inspected (HEAD):
- `internal/cmd/root.go` (flag declarations, `init()`, `shouldSkipConfigLoad`)
- `internal/cmd/default_action.go` (routing: `runDefault`, `shouldDecompose`)
- `internal/cmd/config.go` (`config` subcommand tree)
- `internal/config/load.go` (`Load`, `loadEnv`, `loadFlags`)
- `internal/config/config.go` (`Config` struct, `Defaults()`)
- `internal/config/git.go` (git-config layer — confirmed NO role/commits keys)

All strings below are quoted verbatim from source.

---

## 1. Root flags — EXACT declarations

All registered on `rootCmd.PersistentFlags()` in `internal/cmd/root.go:init()` (lines ~103-130).
Every flag is persistent (inherited by subcommands) EXCEPT the `config init` local flags (see §4).

### 1a. Config-backed global flags (Layer-7, read by `loadFlags` via `fs.Changed`)

| Long | Short | Type | Default | Description (verbatim) |
|---|---|---|---|---|
| `--provider` | — | string | `""` | `Provider/agent to use (env STAGECOACH_PROVIDER, git stagecoach.provider; default auto-detected)` |
| `--model` | — | string | `""` | `Model override (env STAGECOACH_MODEL, git stagecoach.model; default per-manifest default_model)` |
| `--config` | — | string | `""` | `Path to a config file, overrides discovery (env STAGECOACH_CONFIG)` |
| `--timeout` | — | string | `""` | `Generation timeout, e.g. "120s" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 120s)` |
| `--verbose` | `-v` | bool | `false` | `Print resolved command, raw output, retries (env STAGECOACH_VERBOSE)` |
| `--no-color` | — | bool | `false` | `Disable color (env STAGECOACH_NO_COLOR, NO_COLOR; default TTY-aware)` |

Note: `--timeout` is registered as a pflag **STRING** (not Duration) — `loadFlags` reads it via `fs.GetString("timeout")` then `parseTimeout`. `--config` is NOT a Config field (it feeds `LoadOpts.ConfigPathOverride`).

### 1b. Behavioral flags (NOT Config fields; read directly by `runDefault`)

| Long | Short | Type | Default | Description (verbatim) |
|---|---|---|---|---|
| `--all` | `-a` | bool | `false` | `Run git add -A before snapshotting, even if something is staged` |
| `--no-auto-stage` | — | bool | `false` | `If nothing is staged, exit instead of auto-staging` |
| `--dry-run` | — | bool | `false` | `Generate and print the message; do not commit` |

### 1c. Decompose / per-role flags (P4.M1.T1.S1)

| Long | Short | Type | Default | Description (verbatim) |
|---|---|---|---|---|
| `--commits` | — | int | `0` | `Force exactly N commits when nothing is staged (skips the planner's count decision; 0 = auto-decompose). 1 ≡ --single (env/git stagecoach.commits)` |
| `--single` | — | bool | `false` | `Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --no-decompose)` |
| `--no-decompose` | — | bool | `false` | `Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --single)` |
| `--max-commits` | — | int | `12` | `Safety cap on auto-decompose commit count (env/git stagecoach.max_commits)` |
| `--planner-provider` | — | string | `""` | `Per-role provider override for the decomposition planner (env STAGECOACH_PLANNER_PROVIDER; git stagecoach.role.planner)` |
| `--planner-model` | — | string | `""` | `Per-role model override for the decomposition planner (env STAGECOACH_PLANNER_MODEL; git stagecoach.role.planner)` |
| `--stager-provider` | — | string | `""` | `Per-role provider override for the (tooled) staging agent (env STAGECOACH_STAGER_PROVIDER; git stagecoach.role.stager)` |
| `--stager-model` | — | string | `""` | `Per-role model override for the (tooled) staging agent (env STAGECOACH_STAGER_MODEL; git stagecoach.role.stager)` |
| `--arbiter-provider` | — | string | `""` | `Per-role provider override for the leftover arbiter (env STAGECOACH_ARBITER_PROVIDER; git stagecoach.role.arbiter)` |
| `--arbiter-model` | — | string | `""` | `Per-role model override for the leftover arbiter (env STAGECOACH_ARBITER_MODEL; git stagecoach.role.arbiter)` |

### ⚠️ CORRECTIONS to the task brief's flag assumptions

1. **There is NO `--message-provider` / `--message-model` flag.** Verified by `grep -r "message-provider\|message-model\|message_provider\|message_model" *.go` → **no matches**. Only planner/stager/arbiter per-role flags are registered in `root.go:init()`. (`loadFlags` *would* honor a `--message-*` flag if registered, because it loops `roleNames = ["planner","stager","message","arbiter"]` and checks `fs.Changed(role+"-provider")`, but the registration is absent and `Changed==false` → skipped. The root.go comment confirms: *"loop all four so a --message-* flag/env is honored if set (registration in P4.M1.T1 may omit it; Changed==false → skipped)"*.)*
2. The `--commits` and `--max-commits` description strings reference `(env/git stagecoach.commits)` and `(env/git stagecoach.max_commits)`, but see §3/§5 below: **there is no `STAGECOACH_MAX_COMMITS` env var and no git `stagecoach.commits`/`stagecoach.max_commits` key.** The description strings are aspirational/inaccurate relative to the implementation — flag this if docs must match the binary exactly.

Also auto-registered (not in `init()`):
- `--version` — auto-added by cobra from the `Version` field; NO `-v` shorthand (`-v` is taken by `--verbose`). Prints + exits BEFORE `PersistentPreRunE`, so config does NOT load.
- `--help` / `-h` — cobra built-in.

---

## 2. DEFAULT-ACTION routing condition (exact, quoted)

From `internal/cmd/default_action.go:runDefault` (lines ~61-113). The branch on `hasStaged`:

```go
hasStaged, err := g.HasStagedChanges(ctx)
if err != nil {
    return exitcode.New(exitcode.Error, fmt.Errorf("staged changes check: %w", err))
}

if !hasStaged {
    // FR-M1 (P4.M1.T1.S1): nothing staged + dirty tree + decompose enabled → decompose (NO AddAll).
    if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
        status, err := g.StatusPorcelain(ctx)
        if err != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("git status --porcelain: %w", err))
        }
        if status == "" {
            return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // clean tree
        }
        return runDecompose(ctx, stdout, stderr, u, cfg, g) // planner gets the working-tree diff
    }
    switch {
    case flagNoAutoStage:
        // FR19: --no-auto-stage + nothing staged → exit 2 "Nothing staged."
        return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing staged."))
    case cfg.AutoStageAll:
        // FR16/FR18: auto-stage all, print the transparent notice, re-check.
        if err := g.AddAll(ctx); err != nil { ... }
        ... // AddAll, StagedFileCount, FR18 notice, re-HasStagedChanges; falls through to GenerateCommit
    default:
        // cfg.AutoStageAll==false (config), no --no-auto-stage flag → don't auto-stage; exit 2.
        return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
    }
}
// ---- Staged (or staged via auto-stage): ... GenerateCommit (single-commit CommitStaged path) ----
```

The `shouldDecompose` predicate (PURE — `default_action.go:253-264`):

```go
// shouldDecompose is the FR-M1/M2 routing predicate (PURE — no I/O, no package-flag reads). True iff
// the default action should route to multi-commit decomposition instead of the v1 AddAll→GenerateCommit
// path. Decompose activates iff NOTHING is staged (caller guarantees via hasStaged), auto-stage-all is
// on, the user did not opt out (--single/--no-decompose/--commits 1 ⇒ cfg.Single), and --dry-run is not
// set (decompose commits; --dry-run honors the preview).
func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool {
    if cfg == nil {
        return false
    }
    if cfg.Single || cfg.Commits == 1 { // --single/--no-decompose/--commits 1 → v1
        return false
    }
    if dryRun { // decompose commits; --dry-run → single preview
        return false
    }
    return cfg.AutoStageAll && !noAutoStage // FR-M1 trigger context (auto-stage on)
}
```

### Routing summary (when nothing is staged, `!hasStaged`)

Decision order, first match wins:
1. `shouldDecompose(...)` TRUE → **Decompose** (multi-commit). Requires: NOT `cfg.Single`, NOT `cfg.Commits==1`, NOT `--dry-run`, AND `cfg.AutoStageAll && !flagNoAutoStage`. (Clean-tree guard: if `StatusPorcelain==""` → exit 2 "Nothing to commit.")
2. `flagNoAutoStage` → exit 2 **"Nothing staged."**
3. `cfg.AutoStageAll` → `git add -A` + FR18 notice + re-check → **single-commit GenerateCommit** (CommitStaged). (If still nothing → exit 2 "Nothing to commit.")
4. default (`AutoStageAll==false`, no `--no-auto-stage`) → exit 2 **"Nothing to commit."**

So: **Decompose runs only when nothing is staged AND `AutoStageAll` is on AND user did not opt out (`--single`/`--no-decompose`/`--commits 1`/`--dry-run`) AND `--no-auto-stage` is not set.** Otherwise the single-commit CommitStaged path (or an exit-2) runs. When something IS staged, it always goes to the single-commit GenerateCommit path (decompose is never entered from the staged branch).

`cfg.Single` is forced true by the `Commits==1 ≡ Single` normalization in `Load()` (load.go ~line 145: `if cfg.Commits == 1 { cfg.Single = true }`), applied AFTER both env and flag layers.

---

## 3. STAGECOACH_* env vars — exact list (read by `loadEnv`, `internal/config/load.go:159-207`)

Presence-semantic: a PRESENT, **non-empty** value overrides; unset/empty is a no-op. Booleans set DIRECTLY (can force false).

| Env var | Field set | Type / parse | Notes |
|---|---|---|---|
| `STAGECOACH_PROVIDER` | `cfg.Provider` | string | |
| `STAGECOACH_MODEL` | `cfg.Model` | string | |
| `STAGECOACH_TIMEOUT` | `cfg.Timeout` | `parseTimeout` (Go duration OR bare int seconds) | error wrapped `STAGECOACH_TIMEOUT: ...` |
| `STAGECOACH_VERBOSE` | `cfg.Verbose` | `strconv.ParseBool` (DIRECT) | error wrapped `STAGECOACH_VERBOSE: ...` |
| `STAGECOACH_NO_COLOR` | `cfg.NoColor` | `strconv.ParseBool` (DIRECT) | error wrapped `STAGECOACH_NO_COLOR: ...` |
| `STAGECOACH_PLANNER_PROVIDER` | `cfg.Roles["planner"].Provider` | string | via `setRoleProvider` (map-value-copy write-back) |
| `STAGECOACH_PLANNER_MODEL` | `cfg.Roles["planner"].Model` | string | via `setRoleModel` |
| `STAGECOACH_STAGER_PROVIDER` | `cfg.Roles["stager"].Provider` | string | |
| `STAGECOACH_STAGER_MODEL` | `cfg.Roles["stager"].Model` | string | |
| `STAGECOACH_MESSAGE_PROVIDER` | `cfg.Roles["message"].Provider` | string | **read by loadEnv** (roleNames loop includes "message") |
| `STAGECOACH_MESSAGE_MODEL` | `cfg.Roles["message"].Model` | string | **read by loadEnv** |
| `STAGECOACH_ARBITER_PROVIDER` | `cfg.Roles["arbiter"].Provider` | string | |
| `STAGECOACH_ARBITER_MODEL` | `cfg.Roles["arbiter"].Model` | string | |
| `STAGECOACH_COMMITS` | `cfg.Commits` | `strconv.Atoi` (int) | error wrapped `STAGECOACH_COMMITS: ...`; `1` normalizes to `Single=true` in `Load()` |

The per-role vars are generated by a loop: `prefix := "STAGECOACH_" + strings.ToUpper(role)` over `roleNames = []string{"planner","stager","message","arbiter"}`, reading `prefix+"_PROVIDER"` and `prefix+"_MODEL"`.

### ⚠️ CORRECTIONS to the task brief's env-var assumptions

- **`STAGECOACH_CONFIG` is NOT read by `loadEnv`.** It is read directly in `Load()` (load.go ~lines 72-82) to resolve the global-file PATH (`--config > STAGECOACH_CONFIG > discovery`). The `loadEnv` docstring explicitly states: *"STAGECOACH_CONFIG is NOT handled here (it selects the file path, resolved in Load)."* It does NOT map to a Config field.
- **`STAGECOACH_SINGLE` does NOT exist.** `grep -r "STAGECOACH_SINGLE"` → no matches. Single is reached only via `--single`/`--no-decompose` flags or `STAGECOACH_COMMITS=1` normalization.
- **`STAGECOACH_NO_DECOMPOSE` does NOT exist.** No matches.
- **`STAGECOACH_MAX_COMMITS` does NOT exist.** No matches. `MaxCommits` comes only from the config file (`[generation].max_commits`, default 12) or the `--max-commits` flag. (The `--max-commits` flag description string mentions `env/git stagecoach.max_commits`, but neither is implemented.)

---

## 4. `config` subcommand tree (exact, `internal/cmd/config.go`)

Registered via `init()` in config.go: `rootCmd.AddCommand(configCmd)`, and `configCmd.AddCommand(configInitCmd/configPathCmd/configUpgradeCmd)`. All three leaves are in `shouldSkipConfigLoad` (root's `PersistentPreRunE` returns nil for `cmd.Name()` ∈ {"init","path","upgrade"}), so they work outside a git repo.

### `config` (group, no RunE — bare `stagecoach config` prints help)
- `Use:` `"config"`
- `Short:` `"Manage the Stagecoach config file"`
- `Long:` `` `Inspect, bootstrap, or upgrade the Stagecoach global config file.` ``

### `config init` (`configInitCmd`)
- `Use:` `"init"`
- `Short:` `"Bootstrap a working config (auto-detects your agent)"`
- `Args:` `cobra.NoArgs`
- Local flags (registered on `configInitCmd.Flags()`, NOT persistent):

| Long | Short | Type | Default | Description (verbatim) |
|---|---|---|---|---|
| `--provider` | — | string | `""` | `Target a specific provider instead of auto-detecting` |
| `--force` | — | bool | `false` | `Overwrite an existing config file` |
| `--template` | — | bool | `false` | `Write the inert all-commented reference config (v1 behavior)` |

(Long help text is a multi-line literal; key behavior: auto-detects highest-priority installed built-in in order `pi, opencode, cursor, agy, gemini, codex, claude`; defaults to `"pi"` if none detected; `--template` writes the inert all-commented `exampleConfigTemplate`; refuses overwrite unless `--force`, exit code 1.)

### `config path` (`configPathCmd`)
- `Use:` `"path"`
- `Short:` `"Print the resolved global config path"`
- `Args:` `cobra.NoArgs`
- No flags. Prints `config.GlobalConfigPath()` + newline to stdout.

### `config upgrade` (`configUpgradeCmd`)
- `Use:` `"upgrade"`
- `Short:` `"Upgrade an existing config to the current schema version"`
- `Args:` `cobra.NoArgs`
- **No flags.**
- `Long:` (verbatim, note the `fmt.Sprintf`-injected current version):
  > `Rewrite an existing Stagecoach config file in place so its config_version matches this binary's current schema version (` + `` `config_version = %d` `` + `).` (where `%d` = `config.CurrentConfigVersion`)
  >
  > `Only the top-level config_version line is added or updated — every other line (your values, comments, ordering) is preserved byte-for-byte. Running it twice is safe: a file already at the current version is left unchanged ("already up to date").`
  >
  > `This is the remediation the load-time advisory points at when a config has no config_version or an older one. It targets the GLOBAL config (the path printed by ` + `` `stagecoach config path` `` + `).`
  >
  > `If no config file exists, run ` + `` `stagecoach config init` `` + ` first. If the file is not valid TOML, it is left untouched and an error is printed.`

(Behavior: reads global path, validity-gates with `toml.Unmarshal` into `map[string]any`, calls pure `upgradeConfigVersion(content, CurrentConfigVersion)`, writes back only if changed. Idempotent.)

---

## 5. Cross-layer facts worth flagging for doc accuracy

- **Git-config layer (`internal/config/git.go`) reads NONE of the decompose/role keys.** It reads only: `stagecoach.provider`, `stagecoach.model`, `stagecoach.output`, `stagecoach.timeout`, `stagecoach.autoStageAll` (camelCase!), `stagecoach.verbose`, `stagecoach.stripCodeFence` (camelCase), `stagecoach.maxDiffBytes`, `stagecoach.maxMdLines`, `stagecoach.maxDuplicateRetries`, `stagecoach.subjectTargetChars` (all camelCase). There is **no** `stagecoach.commits`, `stagecoach.single`, `stagecoach.max_commits`, or `stagecoach.role.*` git key. So the `--commits`/`--max-commits` description strings citing `git stagecoach.commits` / `git stagecoach.max_commits` are NOT backed by implementation.
- **`Defaults()` (`config.go`):** `Commits: 0`, `Single: false`, `MaxCommits: 12`, `AutoStageAll: true`, `Timeout: 120s`, `Provider/Model: ""`.
- **`roleNames` (load.go:16):** `[]string{"planner", "stager", "message", "arbiter"}` — the canonical four roles; drives both `loadEnv` and `loadFlags` loops.
- **`flagMessageProvider`/`flagMessageModel` package vars do NOT exist** in root.go — only planner/stager/arbiter per-role flag vars are declared.

---

## Start Here
Open `internal/cmd/root.go` (the `init()` block, lines ~103-130) for the authoritative flag table, then `internal/cmd/default_action.go:253` (`shouldDecompose`) for routing, then `internal/config/load.go:159` (`loadEnv`) for env vars. `internal/cmd/config.go` `init()` + the four `var *Cmd = &cobra.Command{...}` blocks are the config-subcommand source of truth.
