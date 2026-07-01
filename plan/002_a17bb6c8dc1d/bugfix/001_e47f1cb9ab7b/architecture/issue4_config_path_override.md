# Issue 4: Config Subcommands Ignore --config / STAGEHAND_CONFIG (Major)

## Root Cause

The three config subcommands (`config init`, `config upgrade`, `config path`) compute their target
path via `config.GlobalConfigPath()` — NEVER consulting `flagConfig` (`--config` flag) or the
`STAGEHAND_CONFIG` env var.

This is because they're in `shouldSkipConfigLoad` (`internal/cmd/root.go:137`):
```go
func shouldSkipConfigLoad(cmd *cobra.Command) bool {
    name := cmd.Name()
    return name == "init" || name == "path" || name == "upgrade"
}
```

So root's `PersistentPreRunE` returns `nil` immediately (never calls `config.Load`), and the
subcommand RunE functions hardcode `config.GlobalConfigPath()`:

```go
// config.go:130 (runConfigPath)
func runConfigPath(cmd *cobra.Command, args []string) error {
    fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath()) // ← ignores --config/STAGEHAND_CONFIG
    return nil
}

// config.go:138 (runConfigUpgrade)
func runConfigUpgrade(cmd *cobra.Command, args []string) error {
    path := config.GlobalConfigPath() // ← ignores --config/STAGEHAND_CONFIG
    // ...
}

// config.go:231 (runConfigInit)
func runConfigInit(cmd *cobra.Command, args []string) error {
    path := config.GlobalConfigPath() // ← ignores --config/STAGEHAND_CONFIG
    // ...
}
```

Meanwhile, `config.Load` (`internal/config/load.go:76-82`) DOES honor the override:
```go
globalPath := opts.ConfigPathOverride                     // --config flag
explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGEHAND_CONFIG") != ""
if globalPath == "" {
    if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
        globalPath = env                                 // STAGEHAND_CONFIG env
    } else {
        globalPath = globalConfigPath()                  // discovery default
    }
}
```

## The Fix

Extract the override-aware path resolution into a shared function in the `config` package and have
the config subcommands use it.

### Step 1: Add `ResolveConfigPath` to the config package

```go
// internal/config/file.go (or load.go)

// ResolveConfigPath returns the config file path, honoring overrides in the same
// precedence as config.Load: flagConfig (--config) > STAGEHAND_CONFIG env > GlobalConfigPath().
// This is the shared resolver for both config.Load and the config init/upgrade/path subcommands.
func ResolveConfigPath(flagConfig string) string {
    if flagConfig != "" {
        return flagConfig
    }
    if env := os.Getenv("STAGEHAND_CONFIG"); env != "" {
        return env
    }
    return GlobalConfigPath()
}
```

Note: `config.Load` should be refactored to use this function too (it currently has the logic
inline at lines 76-82). This makes the resolution logic DRY.

### Step 2: Update the config subcommands

In `internal/cmd/config.go`, change each subcommand to use `config.ResolveConfigPath(flagConfig)`:

```go
// runConfigPath
func runConfigPath(cmd *cobra.Command, args []string) error {
    fmt.Fprintln(cmd.OutOrStdout(), config.ResolveConfigPath(flagConfig))
    return nil
}

// runConfigUpgrade
func runConfigUpgrade(cmd *cobra.Command, args []string) error {
    path := config.ResolveConfigPath(flagConfig)
    // ... (rest unchanged)
}

// runConfigInit
func runConfigInit(cmd *cobra.Command, args []string) error {
    path := config.ResolveConfigPath(flagConfig)
    // ... (rest unchanged)
}
```

The `flagConfig` package variable is already available in `root.go` (line 33: `flagConfig string`),
populated by `pf.StringVar(&flagConfig, "config", "", ...)` in `init()`. Since `--config` is a
persistent flag on rootCmd, it IS parsed for subcommands (cobra parses persistent flags before
RunE). The subcommands skip `config.Load` (via `shouldSkipConfigLoad`), but the `flagConfig` var
is already populated by cobra's flag parsing.

**Verification**: cobra parses persistent flags in `PersistentPreRun` / before `RunE` even for
subcommands whose `PersistentPreRunE` returns early. The `flagConfig` variable is set by the
`StringVar` binding, so it's available in the subcommand RunE.

### Edge Cases

1. **No override**: `flagConfig == ""` and `STAGEHAND_CONFIG` unset → `GlobalConfigPath()` (same as
   before — back-compatible).
2. **`--config` override**: `flagConfig = "/tmp/cfg.toml"` → uses `/tmp/cfg.toml`.
3. **`STAGEHAND_CONFIG` override**: `flagConfig == ""`, `STAGEHAND_CONFIG = "/tmp/cfg.toml"` → uses
   `/tmp/cfg.toml`.
4. **Both set**: `flagConfig` wins (flag > env, matching `config.Load` precedence).
5. **`config path` with override**: prints the resolved override path (not the global path).

## Test Strategy

1. **Unit test for `ResolveConfigPath`**: Table-driven — no override → global path; `--config` → flag
   path; `STAGEHAND_CONFIG` → env path; both → flag wins. Use `t.Setenv` for env tests.

2. **Integration test for `config path`**: Set `STAGEHAND_CONFIG=/tmp/foo.toml`, run `stagehand
   config path`, assert stdout is `/tmp/foo.toml` (not the global path). Also test `--config`.

3. **Integration test for `config upgrade`**: Write a v1 config to a custom path, run `stagehand
   --config /tmp/foo.toml config upgrade`, assert `/tmp/foo.toml` was upgraded (not the global).

4. **Integration test for `config init`**: Run `stagehand --config /tmp/foo.toml config init`,
   assert the config is written to `/tmp/foo.toml` (not the global path).

## Files to Touch

| File | Change | Doc Mode |
|------|--------|----------|
| `internal/config/file.go` | Add `ResolveConfigPath(flagConfig string) string` | JSDoc on new function |
| `internal/config/load.go` | Refactor to use `ResolveConfigPath` (DRY) | none |
| `internal/cmd/config.go` | Use `ResolveConfigPath(flagConfig)` in init/upgrade/path | JSDoc on each RunE |
| `internal/config/file_test.go` | Unit tests for `ResolveConfigPath` | — |
| `internal/cmd/config_test.go` | Integration tests for override-aware subcommands | — |
