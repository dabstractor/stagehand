# Research Notes — P1.M5.T3.S1 (config Load())

## Dependency verification (DONE artifacts in tree)
- **P1.M5.T1.S1** `internal/config/config.go` (Config struct, NO toml tags, `ProviderOverrides map[string]provider.Manifest`) + `defaults.go` (`Default()` floor, `Default*` consts). Default() leaves ProviderOverrides nil.
- **P1.M5.T2.S1** `internal/config/file.go` — unexported `overlay` (pointer-per-scalar presence), `fileDTO/defaultsDTO/generationDTO`, `GlobalConfigPath()`, `parseFile(path)`, `readGlobalFile()`, `readRepoFile(repoDir)`, `readGitConfig(repoDir)`. parseFile doc says it is "reused by Load() (T3.S1) for the --config / STAGEHAND_CONFIG override path".
- **P1.M2.T3.S2** `internal/provider/registry.go` — `NewRegistry(builtins, overrides) *Registry` field-merges overrides onto builtins (unexported `mergeManifest`). `Get/List/Detect` are pointer receivers → return `*Registry`.
- White-box test infra already in `file_test.go`: `initGitRepo`, `gitSet`, `writeFile`, pointer helpers (`ptrStrEq` etc.), `golden162`, `assertEmptyOverlay`. `load_test.go` reuses all of it.

## Key design decisions resolved
1. **Flags struct lives in package config** (CLI layer M7.T2 populates it; avoids import cycle CLI↔config). Carries env + CLI as TWO distinguishable layers so Load can apply env-then-flag (MOCKING "flag>env"). `Flags{Env FlagsLayer; Flag FlagsLayer}`, each field a pointer (nil=unset).
2. **FlagsLayer fields = the 6 STAGEHAND_* vars only** (FR35/§15.2): ConfigPath, Provider, Model, Timeout(*time.Duration), Verbose, NoColor. Generation scalars + AutoStageAll have NO env/flag (AutoStageAll is controlled by --all/--no-auto-stage ACTION flags in M7.T2, not a config setter).
3. **ProviderOverrides layered per-key shallow merge** across global→repo file (repo key replaces global same-name key; different keys survive). Field-merge onto BUILT-IN happens later in NewRegistry (config cannot call provider's unexported mergeManifest; MOCKING only requires field-merge over built-ins, already proven in registry_test.go).
4. **trustNotice** set iff repo-file OR repo git-config has non-nil Provider pointer. <name> = final resolved cfg.Provider. Format: `stagehand: repo-local config changed provider to <name>` (§19). NOT for global file / env / flag / explicit --config.
5. **--config/STAGEHAND_CONFIG** ("overrides discovery", §15.2): if Flags has non-nil ConfigPath (flag wins over env), parseFile(that path) REPLACES global+repo file layers; git-config/env/flag still apply; no trust notice (user-explicit).
6. **Return**: `(cfg Config, reg *provider.Registry, trustNotice string, err error)`. `reg := provider.NewRegistry(provider.Builtins(), cfg.ProviderOverrides)`.

## Verified commands (go1.26, module builds clean)
- `go build ./internal/config/`, `go vet ./internal/config/`, `gofmt -l ./internal/config/`, `go test ./internal/config/ -v`, `go test ./...` all valid single commands.
