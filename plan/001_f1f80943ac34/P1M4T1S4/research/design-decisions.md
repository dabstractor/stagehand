# P1.M4.T1.S4 — config init/path — Research & Design Decisions

Research artifacts for `config init` / `config path` subcommands (PRD §9.8 FR38, §15.3, §16.2).
This file is the reasoning behind the PRP. All file/line references are against the current repo HEAD.

## FINDINGS (verified by reading the source)

### F1 — `globalConfigPath()` is UNEXPORTED (the critical constraint)
- `internal/config/file.go:60` defines `func globalConfigPath() string` — lowercase, package-private.
- The `cmd` package (`internal/cmd`) is a DIFFERENT package → it CANNOT call `globalConfigPath()`.
- The work item lists `globalConfigPath()` as INPUT. The only clean way to consume it cross-package
  is to ADD an exported wrapper in `file.go`:
  ```go
  // GlobalConfigPath returns the resolved global Stagehand config path (PRD §16.1 layer 2),
  // i.e. the file `config init` writes and `config path` prints.
  func GlobalConfigPath() string { return globalConfigPath() }
  ```
- This is ADDITIVE (one new exported func delegating to the existing unexported one — ZERO behavior
  change). `file_test.go` and `load.go` continue to call the unexported `globalConfigPath()` unchanged.
- **Why not duplicate the XDG/home logic in cmd?** Because then `config path` and `config.Load` would
  have two sources of truth for "where is the global file?" — they could drift, and `config path` would
  print a path different from the one Load reads. The single-line wrapper eliminates that hazard.
- Safety: `internal/config/` is owned by P1.M1.T4, which is COMPLETE. No parallel work touches it.
  The S3 PRP froze config because S3 didn't need it; S4 explicitly depends on globalConfigPath, so
  this minimal export is in-scope and safe.

### F2 — `shouldSkipConfigLoad` is ALREADY pre-wired for S4
- `internal/cmd/root.go`: `func shouldSkipConfigLoad(cmd *cobra.Command) bool { name := cmd.Name();
  return name == "init" || name == "path" }`.
- S1 shipped this KNOWING S4 would add `config init` / `config path`. For `stagehand config init`,
  cobra resolves to the leaf `configInitCmd` whose `.Name()` == `"init"` → skip returns true. Same for
  `config path` → Name `"path"` → true.
- CONSEQUENCE: PersistentPreRunE returns nil immediately (no `config.Load`, no Layer-4 `git config`
  shell-out). So `config init` / `config path` work ANYWHERE — including OUTSIDE a git repo and on a
  machine with no config at all. This is exactly right for FR38.
- S4 therefore does NOT edit root.go. Registration is via `init()` in the NEW `internal/cmd/config.go`
  (mirrors S3's `providers.go`). Parallel-safe with everything.

### F3 — Provider Manifest TOML tags (for the `[provider.X]` template example)
- `internal/provider/manifest.go` — the struct's `toml:` tags are the authoritative field names:
  `name`, `detect`, `command`, `subcommand`, `prompt_delivery` (stdin|positional|flag), `prompt_flag`,
  `print_flag`, `model_flag`, `default_model`, `system_prompt_flag`, `provider_flag`,
  `default_provider`, `bare_flags`, `output` (raw|json), `json_field`, `strip_code_fence`,
  `retry_instruction`, `env`.
- The §16.2 `[provider.pi]` and `[provider.myagent]` examples use a subset of these — all valid.
  The `config init` template's commented provider example MUST use these exact tag names.

### F4 — Config-file decode shape (for the `[defaults]` / `[generation]` template)
- `internal/config/file.go` `fileConfig` decodes: `[defaults]` {provider, model, timeout(STRING),
  auto_stage_all, verbose}; `[generation]` {max_diff_bytes, max_md_lines, max_duplicate_retries,
  subject_target_chars, output, strip_code_fence}; `[provider.X]` raw map.
- Defaults (config.go `Defaults()`): timeout 120s, auto_stage_all true, max_diff_bytes 300000,
  max_md_lines 100, max_duplicate_retries 3, output "raw", strip_code_fence true,
  subject_target_chars 50. The template comments should show these as the documented default values.

### F5 — Test harness (reused from root_test.go, same package)
- `internal/cmd/root_test.go` helpers (package `cmd`, directly callable): `saveRootState`,
  `restoreRootState`, `resetFlags`, `loadEnvSetup` (sets `HOME`=`XDG_CONFIG_HOME`=temp dir → so
  `GlobalConfigPath()` == `<tmp>/stagehand/config.toml`), `initRepo`, `setGitConfig`,
  `writeConfigFile`, `chdir`.
- IMPORTANT for S4: to prove init/path work WITHOUT a git repo, tests chdir into a plain `t.TempDir()`
  (NO `initRepo`). Because shouldSkipConfigLoad=true, no git shell-out occurs → exit 0.

### F6 — Exit-code + output conventions (mirror S1/S3)
- `internal/exitcode/exitcode.go`: `New(code, err)`, `For(err)`, `const Error = 1`. RunE funcs RETURN
  `exitcode.New(exitcode.Error, err)` on failure; NEVER `os.Exit`. main does `os.Exit(exitcode.For(err))`
  and prints `stagehand: <err>` to STDERR when err.Error() != "".
- `stdout` = data (the path; the init confirmation). `stderr` = diagnostics (errors via main).

## DESIGN DECISIONS

### D1 — Export `globalConfigPath` as `GlobalConfigPath()` (F1)
Additive wrapper in `internal/config/file.go`. The single edit to an existing file. Rationale in F1.

### D2 — Register via `init()` in NEW `internal/cmd/config.go`; do NOT edit root.go (F2)
Mirror S3. `configCmd` (group, no RunE → bare `config` prints help) + `configInitCmd` (Use "init",
Args NoArgs) + `configPathCmd` (Use "path", Args NoArgs). `init()` does
`configCmd.AddCommand(configInitCmd, configPathCmd); rootCmd.AddCommand(configCmd)`.

### D3 — Template as a Go `const` string (not `//go:embed`)
Single static, richly-commented TOML document. A const is simplest, needs no embed path, and keeps
the doc co-located with the command. This const IS the Mode-A user-facing config documentation.

### D4 — `config init` REFUSES to overwrite an existing file → exit 1
Non-destructive: protects user edits. Returns `exitcode.New(exitcode.Error, fmt.Errorf("config file
already exists at %s (not overwritten)", path))`. main prints `stagehand: <err>` to stderr, exit 1.
No `--force` flag in scope (1-point task; keep minimal).

### D5 — `config path` prints ONLY the path to stdout (one line, scriptable)
`fmt.Fprintln(os.Stdout, config.GlobalConfigPath())`. No label → `$(stagehand config path)` works.
Uses GlobalConfigPath() (the DISCOVERED global XDG/home path), NOT --config/STAGEHAND_CONFIG (those
select a READ path for Load, which is a different workflow; FR38 targets the global location).

### D6 — `config init` confirmation → stdout
`fmt.Fprintf(os.Stdout, "Wrote example config to %s\n", path)`. Friendly primary feedback; no data-
piping need that would be polluted.

### D7 — `config init` creates parent dirs (MkdirAll) then writes 0o644
`os.MkdirAll(filepath.Dir(path), 0o755)`; `os.WriteFile(path, []byte(exampleConfigTemplate), 0o644)`.
0o644 matches the `writeConfigFile` test-helper convention and standard config-file perms.

### D8 — Works outside a git repo (F2)
Because shouldSkipConfigLoad=true. Verify in tests: chdir into a plain temp dir, run config path/init,
assert exit 0. No initRepo needed.

### D9 — The template documents precedence + env vars + git keys (Mode-A DOCS)
The template's header comments cover: §16.1 precedence (FR34), STAGEHAND_* env vars (FR35), git
`stagehand.*` keys (FR36/§16.3), and each section's purpose. Every option line is `#`-commented so
the written file is INERT (no behavior change) until the user uncomments.
