# PRP — P1.M4.T1.S2: Wire `ResolveConfigPath` into the `config init`/`upgrade`/`path` subcommands (Issue 4)

**Issue**: bugfix-001 Issue 4 (Major) — `config init` / `config upgrade` / `config path` ignore the
`--config` flag and the `STAGECOACH_CONFIG` env var and always operate on `config.GlobalConfigPath()`. The
override-aware resolver landed in the sibling subtask **P1.M4.T1.S1** (`config.ResolveConfigPath`); this
subtask is the thin **wiring + tests + help-text** layer that makes the three subcommands actually use it.
**PRD refs**: §15.2 (`--config` / `STAGECOACH_CONFIG` "overrides discovery"), §9.8 FR38 (`config init` /
`config path` / `config upgrade`).
**Binding analysis**: `plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md`
(Step 2 + Test Strategy 2–4 + "Files to Touch").
**Dependency**: **P1.M4.T1.S1** (Complete) — `config.ResolveConfigPath(flagConfig string) string` already
ships in `internal/config/file.go:99` and `config.Load` already calls it (`internal/config/load.go:76`).

---

## Goal

**Feature Goal**: Make `stagecoach --config X config {path,upgrade,init}` (and `STAGECOACH_CONFIG=X`) operate
on file `X` instead of silently using the global config path. Concretely: replace the three hardcoded
`config.GlobalConfigPath()` calls in the three config-subcommand `RunE` functions with
`config.ResolveConfigPath(flagConfig)` (the same resolver `config.Load` uses), update the two now-misleading
`Long` help texts to state the override IS honored, and add integration tests proving all three subcommands
honor `--config`/`STAGECOACH_CONFIG` while staying back-compatible when neither is set.

**Deliverable**:
1. **Code (3 one-line edits)** in `internal/cmd/config.go`: `runConfigPath` (L130), `runConfigUpgrade` (L138),
   `runConfigInit` (L231) — each swaps `config.GlobalConfigPath()` → `config.ResolveConfigPath(flagConfig)`.
   No import changes (`config` is already imported; `flagConfig` is the same-package package var in `root.go`).
2. **Docs (Mode A — rides with this work)**: rewrite the `Long` text of `configPathCmd` and `configUpgradeCmd`
   to remove the "not a --config/STAGECOACH_CONFIG override" / "targets the GLOBAL config" language and state
   the override IS honored.
3. **Tests**: 5 integration tests in `internal/cmd/config_test.go` covering (a) `config path --config`,
   (b) `config path` with `STAGECOACH_CONFIG`, (c) `config upgrade --config`, (d) `config init --config`,
   (e) back-compat (no override → global path; the existing tests are the regression guard).

**Success Definition**:
- `stagecoach --config X config path` prints `X`; `… config upgrade` upgrades `X` (not the global file);
  `… config init` writes to `X`. Same for `STAGECOACH_CONFIG=X`.
- No override set → `ResolveConfigPath("")` returns `GlobalConfigPath()` (back-compatible; every existing
  config test stays green unchanged).
- `go build ./...`, `go vet ./...`, `go test -race ./...`, `make lint`, `make coverage-gate` all green.

## Why

- **User impact**: A user who drives stagecoach with a custom/repo config (`--config` / `STAGECOACH_CONFIG`)
  and then runs `config upgrade` today **silently mutates (or creates) their global config** instead of the
  intended file; `config path` misleads debugging by reporting the global path. This is the exact
  "Major" repro in the PRD.
- **Single source of truth**: S1 already extracted the precedence (`--config > STAGECOACH_CONFIG >
  GlobalConfigPath()`) into one resolver. Wiring the subcommands to it makes the resolution rule authoritative
  in one place (no second drift-prone copy), matching how `config.Load` already resolves.
- **Scope respect**: This is purely the wiring + tests + help-text for Issue 4. It does NOT touch
  `config.Load`, `ResolveConfigPath`, the config loaders, or the default commit action. It does NOT change
  `shouldSkipConfigLoad` (the subcommands intentionally skip `config.Load` — they read the already-parsed
  `flagConfig` var instead).

## What

### User-visible behavior (after fix)
- `stagecoach --config /tmp/foo.toml config path` → prints `/tmp/foo.toml`.
- `STAGECOACH_CONFIG=/tmp/foo.toml stagecoach config path` → prints `/tmp/foo.toml`.
- `stagecoach --config /tmp/foo.toml config upgrade` → upgrades `/tmp/foo.toml` in place (the global path is
  NOT read or written).
- `stagecoach --config /tmp/foo.toml config init` → writes the bootstrap config to `/tmp/foo.toml`.
- No `--config` and `STAGECOACH_CONFIG` unset → all three operate on the global path (unchanged behavior).

### Technical behavior
- cobra parses the root persistent `--config` flag during `Execute` (BEFORE `PersistentPreRunE` and `RunE`),
  populating the `flagConfig` package var (`root.go:99`, `pf.StringVar(&flagConfig, "config", "", …)`). The
  config subcommands are in `shouldSkipConfigLoad`, so `PersistentPreRunE` returns early (no `config.Load`),
  but **`flagConfig` is already populated** by then. Each `RunE` now reads it via
  `config.ResolveConfigPath(flagConfig)`.
- Precedence is identical to `config.Load`: `flagConfig` (`--config`) > `STAGECOACH_CONFIG` env >
  `GlobalConfigPath()` discovery.

### Success Criteria
- [ ] `runConfigPath`/`runConfigUpgrade`/`runConfigInit` use `config.ResolveConfigPath(flagConfig)`.
- [ ] `configPathCmd.Long` and `configUpgradeCmd.Long` state `--config`/`STAGECOACH_CONFIG` are honored.
- [ ] 5 new tests pass (a–e); the ~30 existing config tests pass unchanged (regression guard for back-compat).
- [ ] No new imports, no change to `config.Load`/`ResolveConfigPath`/loaders/`shouldSkipConfigLoad`.

## All Needed Context

### Context Completeness Check

✅ Passes the "No Prior Knowledge" test: the exact 3 lines to edit (with current line numbers), the exact
function to call (already shipped by S1), the same-package var to read, the cobra-flag-parsing rationale,
the exact help-text rewrites, the exact test patterns/helpers to reuse, and the validation commands are all
specified below.

### Documentation & References

```yaml
# MUST READ — binding analysis for this fix
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md
  section: "The Fix" → Step 2 (Update the config subcommands) + Test Strategy 2–4 + "Files to Touch".
  why: THE binding root-cause + fix analysis. Proves (with the shouldSkipConfigLoad + PersistentPreRunE
       trace) that flagConfig IS populated before RunE even though config.Load is skipped, gives the exact
       one-line edits, and lists the integration-test scenarios.
  critical: flagConfig is set by cobra's StringVar binding during Execute — it is available in RunE even
            though PersistentPreRunE returned early. Do NOT add a config.Load call to these subcommands.

# MUST READ — the resolver shipped by S1 (do NOT modify, only call it)
- file: internal/config/file.go
  lines: 99-109 (ResolveConfigPath: flagConfig != "" → flagConfig; else STAGECOACH_CONFIG → env; else
         GlobalConfigPath()).
  why: This is the function the three RunE's now call. Its precedence is identical to config.Load
       (load.go:76), so the subcommands and the default action resolve the same path for the same inputs.
  gotcha: ResolveConfigPath reads STAGECOACH_CONFIG via os.Getenv internally — the subcommands do NOT need to
          read the env var themselves; passing flagConfig ("") is enough and the env fallback is automatic.

# MUST READ — the file & functions to edit (internal/cmd/config.go)
- file: internal/cmd/config.go
  lines: 129-131 (runConfigPath: `fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath())` at L130);
         137-139 (runConfigUpgrade: `path := config.GlobalConfigPath()` at L138);
         230-232 (runConfigInit: `path := config.GlobalConfigPath()` at L231);
         56-69 (configPathCmd.Long — the "not a --config/STAGECOACH_CONFIG override" sentence to rewrite);
         85-100 (configUpgradeCmd.Long — the "It targets the GLOBAL config" sentence to rewrite).
  why: THE edit sites. All three RunE's are package-`cmd` funcs; `flagConfig` is in scope.
  pattern: Keep each function body otherwise byte-identical; only the path-source changes.

# MUST READ — the flagConfig var (same package, no import needed)
- file: internal/cmd/root.go
  lines: 30 (`flagConfig string`); 99 (`pf.StringVar(&flagConfig, "config", "", "…")` — a PERSISTENT flag);
         70-96 (PersistentPreRunE — returns nil early for init/path/upgrade via shouldSkipConfigLoad, but
         cobra has ALREADY parsed --config into flagConfig before this runs).
  why: Confirms flagConfig is the right var to pass and that it is populated for the config subcommands.
  gotcha: config.go and root.go are both package `cmd`, so flagConfig is directly referenced (no qualification).

# MUST READ — the test patterns/helpers to reuse (internal/cmd/config_test.go + root_test.go)
- file: internal/cmd/config_test.go
  lines: 35-78 (setupNoRepo: isolates HOME+XDG to temp dirs, chdir's into a plain dir, returns
         home/plainDir/globalDir); 41-54 (TestConfigPath_PrintsGlobalPath — the CLOSEST analogue: drives
         `Execute` with `rootCmd.SetArgs([]string{"config","path"})`, asserts stdout == GlobalConfigPath());
         67-69 (writeConfigFile helper); 285-330 (TestConfigUpgrade_AddsVersion/AlreadyCurrent — write a
         config, run upgrade, read it back).
  why: The new tests REUSE setupNoRepo / writeConfigFile / saveRootState+restoreRootState / Execute /
       exitcode.For verbatim. Mirror TestConfigPath_PrintsGlobalPath for the (a)/(b) tests and
       TestConfigUpgrade_AddsVersion for the (c) test.
- file: internal/cmd/root_test.go
  lines: 152-179 (saveRootState / restoreRootState); 182-189 (resetFlags).
  why: CRITICAL for test isolation — restoreRootState calls resetFlags(rootCmd.PersistentFlags()), which does
       `f.Value.Set(f.DefValue)`. For the --config flag (DefValue=""), this RESETS the bound flagConfig var
       back to "". So the standard `defer restoreRootState(...)` pattern already prevents flagConfig from
       leaking between tests. Every new test MUST use saveRootState+defer restoreRootState (the existing
       tests all do).
  critical: pflag does NOT reset bound vars between Execute calls within ONE test (see root_test.go:243
            `flagProvider = ""` manual reset). But ACROSS tests, restoreRootState's resetFlags handles it.
            If a test sets --config in multiple sub-Runs, manually reset flagConfig between them.

# CONFIRM S1 shipped (read-only sanity — no edit)
- file: internal/config/load.go
  lines: 76-77 (`globalPath := ResolveConfigPath(opts.ConfigPathOverride)` + the unchanged `explicit :=`).
  why: Confirms S1 is actually complete (ResolveConfigPath exists AND config.Load uses it). If this line is
       still the old inline block, STOP — S1 is not landed and this wiring cannot compile.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go        # runConfigPath (129), runConfigUpgrade (137), runConfigInit (230) + the 2 Long texts — EDIT
  config_test.go   # ADD the 5 Issue-4 integration tests (reuse setupNoRepo/writeConfigFile/saveRootState)
  root.go          # flagConfig (30), StringVar (99), PersistentPreRunE (70) — NO edit (same-package var read)
  root_test.go     # saveRootState/restoreRootState/resetFlags/writeConfigFile/chdir helpers — reuse, NO edit
internal/config/
  file.go          # ResolveConfigPath (99) — SHIPPED by S1, NO edit
  load.go          # Load uses ResolveConfigPath (76) — NO edit
```

### Desired Codebase tree (files MODIFIED — no new files)

```bash
internal/cmd/config.go        # 3 one-line RunE edits + 2 Long help-text rewrites (Mode A)
internal/cmd/config_test.go   # +5 integration tests (a–e)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — flagConfig IS populated for the config subcommands even though they skip config.Load. cobra
// parses ALL flags (persistent + local) during Execute, BEFORE PersistentPreRunE/RunE. shouldSkipConfigLoad
// only short-circuits the config.Load call inside PersistentPreRunE — it does NOT suppress flag parsing.
// So reading flagConfig in runConfigPath/Upgrade/Init is correct. Do NOT add a config.Load call.

// CRITICAL — flagConfig is package var in internal/cmd/root.go (root.go:30). config.go is the SAME package
// (internal/cmd), so reference it unqualified: `config.ResolveConfigPath(flagConfig)`. No import change.

// CRITICAL — test isolation of flagConfig. pflag keeps the bound var across Execute() calls. restoreRootState
// → resetFlags(rootCmd.PersistentFlags()) → f.Value.Set(f.DefValue) resets flagConfig back to "" (DefValue).
// So EVERY new test must use `origArgs, origOut, origErr, origRunE := saveRootState(t); defer
// restoreRootState(t, origArgs, origOut, origErr, origRunE)` (exactly like TestConfigPath_PrintsGlobalPath).
// If a single test sets --config in multiple t.Run sub-cases, also manually `flagConfig = ""` + clear the
// flag's Changed bit between sub-cases (mirror root_test.go:240-244).

// CRITICAL — STAGECOACH_CONFIG env isolation. ResolveConfigPath falls back to os.Getenv("STAGECOACH_CONFIG").
// The ambient env may carry STAGECOACH_CONFIG. Each test MUST explicitly manage it via t.Setenv: set it for
// the env-override tests; t.Setenv("STAGECOACH_CONFIG", "") for the --config-only and back-compat tests so a
// leaked env var can't change the resolved path. (t.Setenv auto-restores.)

// GOTCHA — HOME/XDG isolation for the back-compat / global-path assertions. GlobalConfigPath() reads
// $XDG_CONFIG_HOME then $HOME. setupNoRepo sets both to a temp `home`, so the global path is deterministic:
// filepath.Join(home, "stagecoach", "config.toml"). Use this for the "global NOT touched" assertion in test (c)
// and the back-compat assertion in test (e).

// GOTCHA — file-existence requirements differ per subcommand. config PATH only prints (never reads) → the
// --config target need NOT exist. config UPGRADE reads the file (os.ReadFile) → the --config target MUST
// exist (write a v1 config first; else runConfigUpgrade returns "no config file at X"). config INIT writes
// the file and refuses overwrite unless --force → point --config at a NON-existent path in a t.TempDir()
// (parent exists; MkdirAll(filepath.Dir(path)) is a no-op).

// GOTCHA — config init's --config target parent dir. runConfigInit does os.MkdirAll(filepath.Dir(path)).
// For a path inside t.TempDir() the parent already exists (no-op). Do NOT point --config at a path whose
// parent cannot be created (e.g. a read-only dir). Keep targets under t.TempDir().

// GOTCHA — config init auto-detects providers (GenerateBootstrapConfig). In the test env it may detect real
// agents on the host; assert STRUCTURAL properties (file written to the --config path; contains
// "config_version = 2"), NOT a specific provider/model (mirror TestConfigInit_Populated_WritesWorkingConfig).

// GOTCHA — cobra persistent-flag arg position. `--config X` may appear before the subcommand
// (rootCmd.SetArgs([]string{"--config", X, "config", "path"})) — this is the form the contract specifies and
// the form to use. cobra also accepts trailing placement, but the contract's form is canonical.
```

## Implementation Blueprint

### The 3 code edits (internal/cmd/config.go)

```go
// EDIT 1 — runConfigPath (L130):
//   FROM: fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath())
//   TO:   fmt.Fprintln(cmd.OutOrStdout(), config.ResolveConfigPath(flagConfig))

// EDIT 2 — runConfigUpgrade (L138):
//   FROM: path := config.GlobalConfigPath()
//   TO:   path := config.ResolveConfigPath(flagConfig)

// EDIT 3 — runConfigInit (L231):
//   FROM: path := config.GlobalConfigPath()
//   TO:   path := config.ResolveConfigPath(flagConfig)
// (Each function body is otherwise byte-identical. flagConfig is the same-package root.go var.)
```

### The 2 help-text rewrites (Mode A — internal/cmd/config.go)

```go
// configPathCmd.Long — REMOVE the misleading "not a --config/STAGECOACH_CONFIG override" sentence; STATE
// the override IS honored. Suggested replacement Long (keep the Go backtick-concat style the file uses):
Long: `Print the config file path that ` + "`config init`" + `/` + "`config upgrade`" + ` operate on and that
Stagecoach reads as its global config layer.

By default this is the DISCOVERED global location ($XDG_CONFIG_HOME/stagecoach/config.toml, or
~/.config/stagecoach/config.toml). The --config flag and STAGECOACH_CONFIG env var ARE honored here: when
either is set, this prints that override path — the same file ` + "`config init`" + `/` + "`config upgrade`" + `
then target — so you can confirm exactly which file a command will touch.`,

// configUpgradeCmd.Long — REPLACE "It targets the GLOBAL config (the path printed by `stagecoach config path`)."
// with override-aware wording. Suggested replacement for that sentence (keep the rest of the Long verbatim):
//   ...
//   This is the remediation the load-time advisory points at when a config has no config_version or an older
//   one. It targets the file reported by ` + "`stagecoach config path`" + ` — by default the GLOBAL config, but
//   the --config flag and STAGECOACH_CONFIG env var ARE honored, so ` + "`--config X config upgrade`" + ` (or
//   STAGECOACH_CONFIG=X) upgrades file X instead of the global file.
//   ...
// (configInitCmd.Long is NOT in scope per the contract; its wording does not claim --config is ignored.
//  Optional consistency touch only — do not expand scope.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: READ-ONLY orientation (no edits)
  - CONFIRM S1 shipped: internal/config/file.go:99 has `func ResolveConfigPath(flagConfig string) string`
    AND internal/config/load.go:76 reads `globalPath := ResolveConfigPath(opts.ConfigPathOverride)`. If the
    resolver is missing, STOP — this subtask depends on S1 and cannot compile without it.
  - READ internal/cmd/config.go L129-131, L137-139, L230-232 (the 3 edit sites) and L56-69, L85-100 (the 2
    Long texts). READ internal/cmd/root.go L30 + L99 + L70-96 (flagConfig + PersistentPreRunE trace).

Task 1: EDIT internal/cmd/config.go — the 3 RunE swaps
  - runConfigPath (L130): GlobalConfigPath() → ResolveConfigPath(flagConfig).
  - runConfigUpgrade (L138): GlobalConfigPath() → ResolveConfigPath(flagConfig).
  - runConfigInit (L231): GlobalConfigPath() → ResolveConfigPath(flagConfig).
  - KEEP each function body otherwise byte-identical. NO import change (config already imported; flagConfig
    same-package). VERIFY: go build ./... && go vet ./....

Task 2: EDIT internal/cmd/config.go — the 2 Long help-text rewrites (Mode A)
  - configPathCmd.Long: remove "not a --config/STAGECOACH_CONFIG override, which selects a separate read
    path"; state --config/STAGECOACH_CONFIG ARE honored (use the suggested text above).
  - configUpgradeCmd.Long: replace "It targets the GLOBAL config (the path printed by …)" with override-aware
    wording (use the suggested text above).
  - DO NOT touch configInitCmd.Long (out of contract scope). Preserve Go backtick-concat (` + "`" + `) style.

Task 3: ADD the 5 integration tests to internal/cmd/config_test.go
  - (a) TestConfigPath_ConfigFlag_PrintsOverride: setupNoRepo; t.Setenv("STAGECOACH_CONFIG",""); SetArgs
        ["--config", <tmp>/foo.toml, "config", "path"]; assert stdout == <tmp>/foo.toml.
  - (b) TestConfigPath_StagecoachConfigEnv_PrintsOverride: setupNoRepo; t.Setenv("STAGECOACH_CONFIG",
        <tmp>/foo.toml); SetArgs ["config","path"]; assert stdout == <tmp>/foo.toml.
  - (c) TestConfigUpgrade_ConfigFlag_UpgradesOverride_NotGlobal: setupNoRepo; writeConfigFile a v1 config
        ("config_version = 1\n[defaults]\nprovider = \"pi\"\n") to <tmp>/foo.toml; SetArgs ["--config",
        <tmp>/foo.toml, "config","upgrade"]; Execute; assert <tmp>/foo.toml now has config_version = 2 AND
        provider="pi" preserved; assert the GLOBAL path (filepath.Join(home,"stagecoach","config.toml")) was
        NOT created (os.Stat → NotExist).
  - (d) TestConfigInit_ConfigFlag_WritesOverride: setupNoRepo; point --config at a NON-existent
        <tmp>/foo.toml; SetArgs ["--config", <tmp>/foo.toml, "config","init"]; Execute; assert file exists at
        <tmp>/foo.toml AND contains "config_version = 2".
  - (e) TestConfigPath_NoOverride_BackCompatGlobal: setupNoRepo; t.Setenv("STAGECOACH_CONFIG",""); SetArgs
        ["config","path"]; assert stdout == config.GlobalConfigPath() (== the existing
        TestConfigPath_PrintsGlobalPath behavior — proves back-compat; the existing tests are also regression
        guards).
  - PATTERN for every test: `origArgs,origOut,origErr,origRunE := saveRootState(t); defer restoreRootState(t,
    origArgs,origOut,origErr,origRunE)`. Use a t.TempDir()-based --config target (parent exists). Use
    rootCmd.SetOut(&buf)/SetErr(io.Discard); assert on strings.TrimSpace(buf.String()).
  - DO NOT modify existing tests.

Task 4: VALIDATE — run the gates (see Validation Loop). All green, no regressions.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the override test skeleton (mirror TestConfigPath_PrintsGlobalPath at config_test.go:41):
func TestConfigPath_ConfigFlag_PrintsOverride(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	setupNoRepo(t)
	t.Setenv("STAGECOACH_CONFIG", "") // isolate: this test exercises the FLAG, not the env
	override := filepath.Join(t.TempDir(), "foo.toml") // parent (TempDir) exists

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", override, "config", "path"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if got := strings.TrimSpace(out.String()); got != override {
		t.Errorf("config path = %q, want override %q (--config must be honored)", got, override)
	}
}

// PATTERN — the upgrade "global NOT touched" assertion (test c):
globalPath := filepath.Join(home, "stagecoach", "config.toml") // home from setupNoRepo
if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
	t.Errorf("global config %s must NOT exist (upgrade must target the --config file only)", globalPath)
}
// then assert the override file was upgraded:
data, _ := os.ReadFile(override)
if !strings.Contains(string(data), "config_version = 2") {
	t.Errorf("override file not upgraded; got:\n%s", data)
}
```

### Integration Points

```yaml
CODE (this subtask):
  - file: internal/cmd/config.go
    change: "3 RunE swaps (GlobalConfigPath→ResolveConfigPath(flagConfig)) + 2 Long help-text rewrites"
    risk: LOW. ResolveConfigPath("") == GlobalConfigPath(), so with no override the behavior is identical
          (byte-for-byte). The only behavioral change is when flagConfig/STAGECOACH_CONFIG is non-empty.

TESTS:
  - file: internal/cmd/config_test.go
    change: "+5 integration tests (a–e)"
    guards: the ~30 existing config tests (esp. TestConfigPath_PrintsGlobalPath, TestConfigUpgrade_AddsVersion,
            TestConfigInit_Populated_WritesWorkingConfig, TestConfigLifecycle_InitThenUpgrade) stay GREEN
            UNCHANGED — they exercise the no-override path, which is back-compatible.

NO NEW IMPORTS. NO config.Load / ResolveConfigPath / loader / shouldSkipConfigLoad CHANGES. NO DATABASE /
ROUTES / DEPENDENCIES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 + Task 2 — fix before proceeding to tests.
go build ./...          # compiles (flagConfig same-package; config already imported)
go vet ./...            # vet clean
make lint               # golangci-lint — zero findings
# Expected: zero errors. The edits are 3 one-liners + 2 string-literal changes; types already line up.
```

### Level 2: The new tests (Component Validation)

```bash
# Run ONLY the new Issue-4 wiring tests, verbosely.
go test -race ./internal/cmd/ -run 'ConfigPath_ConfigFlag|ConfigPath_StagecoachConfigEnv|ConfigUpgrade_ConfigFlag|ConfigInit_ConfigFlag|ConfigPath_NoOverride' -v
# Expected: 5 PASS. If a path test reports the GLOBAL path instead of the override, flagConfig is not flowing
# — re-check the runConfigPath swap (L130) and that SetArgs puts --config before the subcommand. If upgrade
# reports "no config file", the override file wasn't written first (test c prerequisite). If init errors
# "already exists", the --config target wasn't fresh.
```

### Level 3: Full suite + regression (System Validation)

```bash
# The whole cmd package (incl. all pre-existing config tests = back-compat regression guard).
go test -race ./internal/cmd/ -v
# Expected: ALL green. A failure in an EXISTING test means a back-compat regression — most likely flagConfig
# leaked from a new test (re-check every new test uses saveRootState+defer restoreRootState) or a Long-text
# edit broke a help-output assertion.

# Full repo suite (Makefile `test`):
go test -race ./...
# Expected: all packages pass.
```

### Level 4: Manual end-to-end smoke (proves the user-visible fix)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
tmp=$(mktemp -d)
printf 'config_version = 1\n[defaults]\nprovider = "pi"\n' > "$tmp/cfg.toml"
# config path honors --config:
/tmp/stagecoach --config "$tmp/cfg.toml" config path          # → prints "$tmp/cfg.toml"
# config upgrade honors --config (upgrades cfg.toml; global untouched):
/tmp/stagecoach --config "$tmp/cfg.toml" config upgrade        # → "Upgraded config at $tmp/cfg.toml to version 2."
grep -q 'config_version = 2' "$tmp/cfg.toml" && echo "OK: cfg.toml upgraded"
# config path honors STAGECOACH_CONFIG:
STAGECOACH_CONFIG="$tmp/cfg.toml" /tmp/stagecoach config path   # → prints "$tmp/cfg.toml"
# back-compat (no override → global):
unset STAGECOACH_CONFIG; /tmp/stagecoach config path            # → prints the global path
rm -rf "$tmp"
```

### Level 5: Coverage gate (PRD §20.3)

```bash
# internal/cmd is not in the coverage-gate set (internal/{git,provider,generate,config} are), but confirm
# no regression:
make coverage-gate     # all 4 core packages still >= 85%
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` — all packages pass.
- [ ] `make lint` — zero findings.
- [ ] `make coverage-gate` — no regression.

### Feature Validation
- [ ] `runConfigPath`/`runConfigUpgrade`/`runConfigInit` call `config.ResolveConfigPath(flagConfig)`.
- [ ] `configPathCmd.Long` + `configUpgradeCmd.Long` state `--config`/`STAGECOACH_CONFIG` are honored (the
      misleading "not a … override" / "targets the GLOBAL config" language removed).
- [ ] Test (a): `config path --config X` → prints X.
- [ ] Test (b): `config path` with `STAGECOACH_CONFIG=X` → prints X.
- [ ] Test (c): `config upgrade --config X` → X upgraded to config_version=2; global path NOT touched.
- [ ] Test (d): `config init --config X` → X written (contains config_version=2).
- [ ] Test (e): no override → global path (back-compat).
- [ ] All ~30 pre-existing config tests green unchanged.

### Code Quality Validation
- [ ] 3 one-line swaps + 2 Long rewrites only; no collateral edits.
- [ ] Reuses setupNoRepo/writeConfigFile/saveRootState/restoreRootState/Execute/exitcode.For (no duplication).
- [ ] Every new test isolates STAGECOACH_CONFIG (t.Setenv) and HOME/XDG (setupNoRepo).
- [ ] Every new test uses saveRootState + defer restoreRootState (flagConfig reset via resetFlags).

### Documentation & Boundaries
- [ ] Mode A help-text rides here (configPathCmd + configUpgradeCmd Long). README/overview sweep is P1.M6.
- [ ] No change to config.Load / ResolveConfigPath / loaders / shouldSkipConfigLoad / the default action.

---

## Anti-Patterns to Avoid

- ❌ Don't add a `config.Load` call to the config subcommands — they intentionally skip it
  (`shouldSkipConfigLoad`); read the already-parsed `flagConfig` var instead.
- ❌ Don't read `STAGECOACH_CONFIG` yourself in the RunE — `ResolveConfigPath` does the env fallback; just pass
  `flagConfig`.
- ❌ Don't qualify `flagConfig` as `root.flagConfig` or similar — it is an unexported package var in the SAME
  package (`internal/cmd`); reference it bare.
- ❌ Don't forget `t.Setenv("STAGECOACH_CONFIG", "")` in the --config-only and back-compat tests — a leaked
  ambient env var would change the resolved path and cause flaky failures.
- ❌ Don't forget `saveRootState` + `defer restoreRootState` in every new test — without it `flagConfig` leaks
  (pflag doesn't reset bound vars between Execute calls) and later tests see a stale `--config` value.
- ❌ Don't point `config upgrade --config` at a non-existent file in a test — `runConfigUpgrade` reads it
  first; write a v1 config before running (test c prerequisite).
- ❌ Don't point `config init --config` at an existing file without `--force` — init refuses to overwrite
  (exit 1). Use a fresh t.TempDir() path.
- ❌ Don't assert a specific provider/model in the `config init --config` test — bootstrap auto-detection
  varies by host; assert structural properties (file written, config_version=2) instead.
- ❌ Don't modify `configInitCmd.Long` — the contract scopes the help rewrite to `configPathCmd` +
  `configUpgradeCmd` (configInitCmd does not claim `--config` is ignored).
- ❌ Don't touch `config.Load`, `ResolveConfigPath`, the loaders, or `shouldSkipConfigLoad` — S1 owns the
  resolver; the wiring is one-directional (subcommands call it).

---

## Confidence Score

**9 / 10** — This is a small, precisely-scoped wiring task: three one-line swaps (`GlobalConfigPath()` →
`ResolveConfigPath(flagConfig)`) at lines already cited (L130/L138/L231), two help-string rewrites, and five
integration tests that reuse the existing `setupNoRepo`/`saveRootState`/`Execute` patterns verbatim. The
dependency (`ResolveConfigPath`) is confirmed shipped (file.go:99) and already proven by `config.Load`. The
sole subtlety — that `flagConfig` is populated for config-subcommand RunEs despite `shouldSkipConfigLoad` —
is documented in the binding architecture doc and re-verified here, and `resetFlags`'s `f.Value.Set(DefValue)`
is confirmed to reset the bound var for cross-test isolation. Back-compat is mechanically guaranteed
(`ResolveConfigPath("") == GlobalConfigPath()`), and the ~30 existing config tests are the regression guard.
The only residual hazards are mechanical (env/HOME isolation, file-existence prerequisites, flagConfig
leakage between sub-Runs), all called out above.
