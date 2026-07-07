# Research Findings — P1.M4.T1.S1 (bugfix-001, plan 002): Add `ResolveConfigPath` + refactor `config.Load`

Research-only scout. Repo: `/home/dustin/projects/stagecoach` (Stagecoach v2.0). All refs verified.

## 1. The inline logic to extract (internal/config/load.go:76-83)

Inside `Load`, the global-file PATH is resolved inline (precedence: --config > STAGECOACH_CONFIG > discovery):
```go
globalPath := opts.ConfigPathOverride
explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
if globalPath == "" {
	if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
		globalPath = env
	} else {
		globalPath = globalConfigPath()
	}
}
```
- `globalPath` is the resolved path used by the subsequent `loadTOML(globalPath)` / bootstrap / explicit-missing error logic.
- `explicit` (line 77) is computed SEPARATELY and consumed below to decide "missing EXPLICIT path → hard
  error" vs "missing discovery path → layer-absent sentinel / bootstrap". It MUST stay (ResolveConfigPath
  returns only the path; it does not report explicit). Keep the `explicit :=` line byte-identical.

## 2. The new function lives in internal/config/file.go (Path helpers section)

file.go already imports `os` and `path/filepath` (lines 3-5). `GlobalConfigPath()` (file.go ~line 91) is
the exported wrapper delegating to `globalConfigPath()` (XDG_CONFIG_HOME absolute → join; else
os.UserHomeDir() → ~/.config/stagecoach/config.toml; last-resort "config.toml").

Add `ResolveConfigPath` next to these path helpers (after `GlobalConfigPath`/`globalConfigPath`, before
`repoLocalConfigPath`), per the contract + architecture doc Step 1:
```go
// ResolveConfigPath returns the config file path, honoring overrides in the SAME precedence as
// config.Load: flagConfig (--config) > STAGECOACH_CONFIG env > GlobalConfigPath() discovery. It is the
// shared resolver for config.Load and the config init/upgrade/path subcommands (bugfix-001 Issue 4).
func ResolveConfigPath(flagConfig string) string {
	if flagConfig != "" {
		return flagConfig
	}
	if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
		return env
	}
	return GlobalConfigPath()
}
```

## 3. The refactor (internal/config/load.go:76-83 → 2 lines)

Replace the 8-line `globalPath` resolution block with:
```go
	globalPath := ResolveConfigPath(opts.ConfigPathOverride)
	explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
```
- `os` import in load.go stays needed (the `explicit` line calls os.Getenv). `ResolveConfigPath` is the
  same package (file.go) — NO new import.
- The comment block above (lines ~71-75: "Resolve the global-file path: --config > STAGECOACH_CONFIG >
  discovery...") stays accurate; optionally append "via ResolveConfigPath".
- Behavior is BYTE-IDENTICAL: the extracted function reproduces the exact override+discovery order, and
  `explicit` is unchanged.

## 4. Existing tests that MUST stay green (regression guards — do NOT modify)

internal/config/load_test.go:
- `TestLoad_ConfigPathOverride` (684) — ConfigPathOverride present → loads it.
- `TestLoad_STAGECOACH_CONFIG_EnvPath` (700) — env beats discovery; ConfigPathOverride beats env.
- `TestLoad_ConfigPathOverride_MissingFileFails` (728) — explicit MISSING path → "config file not found".
- (and the bootstrap / discovery tests using DisableBootstrap.)
These exercise Load end-to-end; the refactor must not perturb them.

## 5. New unit tests (internal/config/file_test.go) — table-driven, model on TestGlobalConfigPath

Add `TestResolveConfigPath` next to `TestGlobalConfigPath` (file_test.go:148) / `TestGlobalConfigPath_Wrapper`
(file_test.go:321). Cases (contract item 5):
- (a) flagConfig set (env unset) → returns flagConfig.
- (b) flagConfig empty, STAGECOACH_CONFIG set → returns env value.
- (c) BOTH set → flagConfig wins (flag > env).
- (d) neither set → returns GlobalConfigPath().
Use `t.Setenv` (auto-restores + non-parallel; the repo already uses t.Setenv at load_test.go:709). For (d),
make GlobalConfigPath() deterministic by `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` and compute expected =
`filepath.Join(xdg, "stagecoach", "config.toml")` (matches globalConfigPath's absolute-XDG branch).
Guard against STAGECOACH_CONFIG leaking from the environment: in each case explicitly t.Setenv/Unsetenv
STAGECOACH_CONFIG (or t.Setenv("STAGECOACH_CONFIG", "") for the flag-only / neither cases).

## 6. Scope boundary — S1 vs S2

- THIS subtask (S1): the `config` package only — add `ResolveConfigPath` (file.go), refactor `Load`
  (load.go), add unit tests (file_test.go), Go doc comment on the new function. NO user-facing doc change
  yet (the subcommand wiring lands in S2).
- S2 (P1.M4.T1.S2): wire `config.ResolveConfigPath(flagConfig)` into runConfigPath/runConfigUpgrade/
  runConfigInit (internal/cmd/config.go) + integration tests. Do NOT touch cmd/config.go here.
- P1.M6 (Mode B doc sweep) runs last.

## 7. Validation commands (verified from Makefile / prior PRPs)

- `go build ./...` / `go vet ./...`
- `go test -race ./internal/config/...` (targeted: includes the new test + all Load guards).
- `go test -race ./...` (full suite).
- `make lint` (golangci-lint; .golangci.yml present).
