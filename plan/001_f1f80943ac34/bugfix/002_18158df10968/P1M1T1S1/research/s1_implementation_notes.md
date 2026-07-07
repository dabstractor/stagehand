# S1 Implementation Notes — explicit-vs-discovery config path distinction

> Scope: P1.M1.T1.S1 — make `config.Load` fail fast when an EXPLICIT `--config`/`STAGECOACH_CONFIG`
> path points at a missing file. Verified against the live source on 2026-06-30.

## 1. The exact bug locus (internal/config/load.go, verbatim current code)

```go
	// Resolve the global-file path: --config > STAGECOACH_CONFIG > discovery.
	globalPath := opts.ConfigPathOverride
	if globalPath == "" {
		if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
			globalPath = env
		} else {
			globalPath = globalConfigPath()
		}
	}

	// Layer 2: global TOML (or --config/STAGECOACH_CONFIG override). nil => absent (no error).
	if g, err := loadTOML(globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	} else if g != nil {
		overlay(&cfg, g)
	}
```

`loadTOML` (internal/config/file.go:99-102) returns `(nil, nil)` on `os.IsNotExist` — the DISCOVERY
path's "layer absent" sentinel. There is no record of whether `globalPath` came from an explicit
source. → missing explicit file == "discovery default absent" → no provider resolved →
`pkg/stagecoach.buildDeps` auto-detects the first **installed** built-in (pi/claude/…) and invokes the
REAL agent. The `loadTOML` `(nil,nil)` contract MUST be preserved for the discovery path.

## 2. The precise two-edit fix (matches contract + issue_analysis ISSUE 1 exactly)

**Edit A** — compute `explicit` alongside globalPath resolution:
```go
	globalPath := opts.ConfigPathOverride
	explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
	if globalPath == "" { ... }
```
`explicit` is `true` iff globalPath's source is `--config` or `STAGECOACH_CONFIG` (NOT discovery).
- ConfigPathOverride != "" → globalPath = override, explicit=true (the `if globalPath==""` block is skipped).
- ConfigPathOverride == "" && env != "" → globalPath = env, explicit=true.
- both empty → globalPath = globalConfigPath() (discovery), explicit=false.

**Edit B** — add an `else if explicit` arm to the loadTOML if/else chain (fires ONLY when
`g == nil && err == nil`, i.e. a missing file):
```go
	if g, err := loadTOML(globalPath); err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	} else if g != nil {
		overlay(&cfg, g)
	} else if explicit {
		return nil, fmt.Errorf("config file not found: %s", globalPath)
	}
```

## 3. Why this is airtight (verified by tracing every input)

| Input | globalPath | explicit | loadTOML result | outcome |
|---|---|---|---|---|
| `--config /existing.toml` | override | true | g!=nil | overlay (unchanged) ✓ |
| `--config /missing.toml` | override | true | (nil,nil) | **ERROR "config file not found"** ✓ (FIXED) |
| `--config /malformed.toml` | override | true | parse err | "global config: parse config…" (unchanged ✓) |
| `--config /somedir` | override | true | read err | "global config: read config…is a directory" (unchanged ✓) |
| `STAGECOACH_CONFIG=/missing` (no --config) | env | true | (nil,nil) | **ERROR** ✓ (FIXED) |
| neither, no global file | globalConfigPath() | false | (nil,nil) | no error, layer absent ✓ (unchanged) |
| neither, global file present | globalConfigPath() | false | g!=nil | overlay ✓ (unchanged) |

The malformed/directory cases are UNAFFECTED because loadTOML returns a non-nil ERROR (not nil) for
those — the new `else if explicit` arm only triggers on `g == nil && err == nil` (missing file).

## 4. Error wrap chain (user-visible string) — verified

`Load` returns `fmt.Errorf("config file not found: %s", globalPath)`.
- `pkg/stagecoach.resolveConfig` wraps it as `load config: config file not found: <path>` (stagecoach.go
  `fmt.Errorf("load config: %w", err)`) — but this path is bypassed in the CLI now (bugfix-001 S1
  passes the resolved cfg; Load is called once in PersistentPreRunE).
- `internal/cmd/root.go` PersistentPreRunE (line ~65) wraps as `config: config file not found: <path>`
  and returns `exitcode.New(exitcode.Error, …)` → **exit 1**, before any provider resolution.

So: Load-level (package config) tests assert `strings.Contains(err.Error(), "config file not found")`
+ the path substring. The `config:` prefix is CLI-only, not visible in the Load unit test.

## 5. Production scope — EXACTLY one file

Per contract: `internal/config/load.go` is the ONLY production file touched. Do NOT modify:
- `loadTOML` / `file.go` — its `(nil,nil)` contract is the discovery sentinel, preserved as-is.
- `root.go` — `--config` already flows via `ConfigPathOverride` (root.go:64, :82); STAGECOACH_CONFIG is
  already read in Load. No wiring change.

## 6. Test reuse (internal/config, package-private helpers — load_test.go)

- `loadEnvSetup(t) (home, repo, globalDir)` (:62) — isolates HOME/XDG, makes a repo.
- `writeConfigFile(t, dir, relPath, body) string` (:18).
- `chdir(t, repo)` (:35).
- Error idiom: `strings.Contains(err.Error(), "<tag>")`.

Existing path tests STILL PASS unchanged (they use EXISTING files):
- `TestLoad_ConfigPathOverride` (:510) — existing custom.toml.
- `TestLoad_STAGECOACH_CONFIG_EnvPath` (:526) — existing env-config.toml + clipath.toml.

NEW tests to add (place in the "Load — path resolution tests" block near :545):
1. `TestLoad_ConfigPathOverride_MissingFileFails` — `ConfigPathOverride: repo+"/missing.toml"` →
   err non-nil, contains `"config file not found"` AND `missing.toml`.
2. `TestLoad_STAGECOACH_CONFIG_EnvPath_MissingFileFails` — `t.Setenv("STAGECOACH_CONFIG", …+"/nope.toml")`
   → err contains `"config file not found"`.
3. `TestLoad_DiscoveryMissingFileOK` (regression guard) — no --config, no STAGECOACH_CONFIG, no global
   file in globalDir → err == nil (discovery tolerates absence); this is the guard that the new code
   did NOT break the discovery "layer absent" contract.
4. (optional) `TestLoad_ConfigPathOverride_BeatsEnvMissingFile` — both set but only the override
   matters; missing override path → error regardless of env.

## 7. Docs (Mode A — rides with the work)

- `docs/cli.md` line 20 (`--config` row description) + the prose note at line 30: add that an
  explicit `--config`/`STAGECOACH_CONFIG` pointing at a MISSING file fails fast with exit 1 (consistent
  with a malformed/directory path), rather than falling back to discovery.
- `docs/configuration.md` line 34 (the `[!NOTE]`): same wording.

These are docs-only changes; they do not affect the binary. Issue 2/3/4 doc sweeps belong to their
own subtasks / the final M5 task.
