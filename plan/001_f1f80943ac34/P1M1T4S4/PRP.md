---
name: "P1.M1.T4.S4 — Full precedence resolution (env vars + CLI flags) and Load() orchestrator"
description: |
  The FOURTH and FINAL subtask of the Configuration System (P1.M1.T4): assemble the 7-layer precedence
  resolver (PRD §16.1, FR34) into a single `Load()` orchestrator and add the two HIGHEST layers — env
  vars (layer 5, FR35/§15.2) and CLI flags (layer 7, via `flags.Changed()`) — that S1/S2/S3 deliberately
  deferred. Implement `internal/config/load.go` (`package config`) with:
    (a) `func Load(ctx context.Context, opts LoadOpts) (*Config, error)` — applies layers in precedence
        order: `Defaults()` → global TOML → repo TOML → git config → env vars → CLI flags;
    (b) `type LoadOpts struct { ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet }`;
    (c) `func loadEnv(cfg *Config)` — presence-checked `STAGECOACH_*` overlay, DIRECT field set for
        bools (the boolean-false escape hatch S2/S3 documented);
    (d) `func loadFlags(cfg *Config, fs *pflag.FlagSet)` — `flags.Changed()`-gated overlay, DIRECT field
        set; and
    (e) `func parseTimeout(s string) (time.Duration, error)` — dual-form (`"120s"` OR integer seconds).
  Plus `internal/config/load_test.go`: table-driven precedence tests with layered env/file/flag/git
  combinations. INPUT = S1's `Config`+`Defaults()`, S2's `loadTOML`/`overlay`/`loadRepoLocalConfig`/
  `globalConfigPath`, S3's `loadGitConfig`. OUTPUT = one fully-resolved `*Config` consumed by the CLI
  layer (P1.M4.T1) and the generate orchestrator (P1.M3.T4). PRD §16.1 (order), §15.2/FR35 (env + flag
  names + defaults), §9.8/FR34 (precedence), arch `go_ecosystem_patterns.md` §2.6–2.8 (CORE ideas only —
  see GOTCHA: that sketch targets an abandoned nested-struct model) are the spec.

  ⚠️ **THE central design call — env + CLI bool overlays set DIRECTLY (bypass the non-zero overlay).**
  S2's `overlay(dst, src)` copies ONLY non-zero scalars, so layers 2–4 (TOML/git) cannot force a bool
  `false` or an int/string zero — a documented v1 limitation. **S4 resolves exactly this** for the two
  highest layers: `loadEnv` and `loadFlags` do NOT build a partial `*Config`+`overlay()`; they mutate
  the resolved `*Config` IN PLACE — env when the var is PRESENT, flags when `flags.Changed(name)`. So
  `STAGECOACH_VERBOSE=false` and an explicit `--no-color` correctly yield `Verbose=false`/`NoColor=false`
  after `Load`. This is the "force false via env (S4)/CLI (S4)" escape hatch S2/S3 pointed at. (Layers
  2–4 stay partial-`*Config` + `overlay()` — unchanged.)

  ⚠️ **THE second design call — pflag is a NEW dependency.** `go.mod` currently has ONLY
  `github.com/pelletier/go-toml/v2 v2.4.2` (verified). S4 imports `github.com/spf13/pflag` (for
  `*pflag.FlagSet` + `flags.Changed`/`GetString`), so it MUST `go get github.com/spf13/pflag`, adding
  entries to BOTH `go.mod` (require) and `go.sum`. This is the FIRST subtask to introduce a CLI dep
  (cobra itself arrives in P1.M4.T1.S1 and will pass `cmd.Flags()` into `Load`). **The S3-style gate
  "`git diff --exit-code go.mod go.sum` empty" is INVERTED here** — the diff MUST show pflag added; an
  empty diff means the dep was not added and `load.go` will not compile.

  ⚠️ **THE third design call — arch §2.6–2.8 is a NON-AUTHORITATIVE sketch.** That code targets an OLD
  NESTED-struct `Config` (`cfg.Defaults.Provider`, a typed `cfg.Provider[name].APIKey` map). The real
  `Config` (S1) is FLAT + plain-typed (`Provider string`, `Timeout time.Duration`, …). Therefore: use
  the PRD §15.2/FR35 env NAMES (`STAGECOACH_PROVIDER`, `STAGECOACH_MODEL`, `STAGECOACH_TIMEOUT`,
  `STAGECOACH_CONFIG`, `STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR`) — NOT arch's invented
  `STAGECOACH_DEFAULT_*`; and set the flat `cfg.Model`/`cfg.Provider` strings — NOT a typed manifest map.
  The arch sketch's CORE IDEAS ARE followed: presence-check env vars, and `flags.Changed()` for
  explicitly-set flags. (See research/findings.md FINDING 2.)

  ⚠️ **THE fourth design call — `STAGECOACH_CONFIG` selects a FILE PATH, not a value overlay.** PRD §15.2:
  `--config`/`STAGECOACH_CONFIG` = "Path to a config file (overrides discovery)." They choose WHICH file
  becomes layer 2; they are NOT a layer-5 value. Path precedence: `--config` (carried in
  `opts.ConfigPathOverride`) > `STAGECOACH_CONFIG` (env) > `globalConfigPath()` discovery. `Load` resolves
  this at the TOP (before layer 2 loads). `loadEnv` does NOT treat `STAGECOACH_CONFIG` as a value;
  `loadFlags` does NOT touch `config` (the caller — cobra PersistentPreRunE, P1.M4.T1.S1 — fills
  `opts.ConfigPathOverride` from the `--config` flag).

  ⚠️ **THE fifth design call — `loadRepoLocalConfig()` (S2) is frozen; `opts.RepoDir` feeds ONLY git.**
  S2's `loadRepoLocalConfig()` takes NO args, reads `.stagecoach.toml` RELATIVE TO CWD, and emits the §19
  provider-redirect notice. S4 calls it AS-IS (no S2 edit — S2 is COMPLETE). `opts.RepoDir` is passed to
  `loadGitConfig(opts.RepoDir)` ONLY. Normally CWD == repoDir (PRD §11.2 process model); tests of the
  repo-local layer use `os.Chdir` (Go 1.22 has no `t.Chdir`).

  ⚠️ **THE sixth design call — `ctx` is honored minimally.** `Load(ctx, opts)` carries ctx per the
  contract, but the frozen loaders (`loadTOML`/`loadRepoLocalConfig`/`loadGitConfig`) take no ctx
  (`loadGitConfig` uses `context.Background()` internally). S4 does ONE `ctx.Err()` check at entry
  (cancellation requested → return early). It is the seam for a future ctx-aware variant, not a
  cancellation thread.

  ⚠️ **THE seventh design call — `--timeout` should be a pflag STRING flag.** Contract: "parse `120s` /
  integer seconds." `STAGECOACH_TIMEOUT` (raw env string) may be either form; `parseTimeout` handles
  both (`time.ParseDuration` first, then `strconv.Atoi`→seconds). Recommend P1.M4.T1.S1 register
  `--timeout` as a STRING flag so `loadFlags` reads `flags.GetString("timeout")` + `parseTimeout`
  (identical to env). S4's own tests build a pflag.FlagSet with `timeout` as a string flag.
  (Coordination note, not a blocker: if P1.M4 later chooses a `Duration` flag, `loadFlags` should use
  `flags.GetDuration` instead. S4 cannot dictate P1.M4's flag type — only the parsed result.)

  Deliverable: `internal/config/load.go` (`Load` + `LoadOpts` + `loadEnv` + `loadFlags` + `parseTimeout`)
  and `internal/config/load_test.go` (table-driven precedence). TOUCHES ONLY `internal/config/` source
  + `go.mod`/`go.sum` (pflag). Does NOT modify `config.go` (S1 frozen), `file.go` (S2 frozen),
  `git.go` (S3 frozen). INPUT = S1/S2/S3 outputs. OUTPUT = the fully-resolved `*Config`.
---

## Goal

**Feature Goal**: Give Stagecoach its 7-layer configuration resolver (PRD §16.1, FR34): a single
`Load()` that produces one fully-resolved `*Config` by applying, lowest→highest, the built-in defaults,
global TOML, repo-local TOML, repo git config, `STAGECOACH_*` environment variables, and CLI flags —
where env/flags are the two layers S1/S2/S3 deferred, and where "higher wins" is enforced per field.
Critically, the env and CLI layers must be able to force a boolean to `false` (the documented escape
hatch the lower, non-zero-overlay layers cannot), and CLI flags must overlay ONLY the flags the user
explicitly set (`flags.Changed()`).

**Deliverable**:
1. **CREATE** `internal/config/load.go` (`package config`) —
   (a) `func Load(ctx context.Context, opts LoadOpts) (*Config, error)` — resolve the global-file path
       (`opts.ConfigPathOverride` > `STAGECOACH_CONFIG` > `globalConfigPath()`); then apply layers:
       `cfg := Defaults()`; `overlay(&cfg, loadTOML(globalPath))`; `overlay(&cfg, loadRepoLocalConfig())`;
       `overlay(&cfg, loadGitConfig(opts.RepoDir))`; `loadEnv(&cfg)`; if `opts.Flags != nil`:
       `loadFlags(&cfg, opts.Flags)`; return `&cfg`.
   (b) `type LoadOpts struct { ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet }`.
   (c) `func loadEnv(cfg *Config)` — for `STAGECOACH_PROVIDER/MODEL` (non-empty string → set),
       `STAGECOACH_TIMEOUT` (`parseTimeout` → set), `STAGECOACH_VERBOSE` (`ParseBool` → DIRECT set),
       `STAGECOACH_NO_COLOR` (`ParseBool` → DIRECT set). `STAGECOACH_CONFIG` is consumed at path
       resolution, NOT here. DIRECT set = the boolean-false escape hatch.
   (d) `func loadFlags(cfg *Config, fs *pflag.FlagSet)` — for each of `provider`/`model`/`timeout`/
       `verbose`/`no-color`: if `fs.Changed(name)`, read the value and DIRECT-set the field. (Only the
       Config-backed flags; behavioral flags `--all`/`--dry-run`/… are CLI control-flow, P1.M4.)
   (e) `func parseTimeout(s string) (time.Duration, error)` — `time.ParseDuration(s)`; on error,
       `strconv.Atoi(s)` → `time.Duration(n)*time.Second`; else wrapped error.
2. **CREATE** `internal/config/load_test.go` (`package config`, white-box) — table-driven precedence
   tests (see Task 5): defaults-only, each layer overriding the one below, env-over-git, CLI-over-env,
   **unset-CLI-flag does NOT override** (the `flags.Changed` correctness test), boolean-false escape
   hatch (env + CLI), `STAGECOACH_CONFIG`/`--config` path resolution, `parseTimeout` both forms, and
   error propagation (bad global file, git-binary missing). Reuses `initRepo`/`setGitConfig` from
   `git_test.go` (S3, same package) + its own TOML-file/chdir/pflag helpers.

**`go get github.com/spf13/pflag`** is part of this deliverable (FINDING 1: pflag is absent from
`go.mod` today). No change to `config.go`/`file.go`/`git.go`.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` clean;
`go test -race ./internal/config/ -v` passes (S1's + S2's + S3's tests stay GREEN, plus S4's new
tests); `go test -race ./...` green; `go.mod`/`go.sum` now contain `github.com/spf13/pflag` (the diff
is NON-empty — the opposite of S3's gate); `Load` resolves all 7 layers in the correct order, env/CLI
can force booleans false, and unset CLI flags leave lower-layer values intact.

## User Persona

**Target User**: Downstream Stagecoach subtasks **P1.M4.T1** (CLI: calls `Load(ctx, LoadOpts{
ConfigPathOverride: cfgFlag, RepoDir: repoRoot, Flags: cmd.Flags()})` in cobra's `PersistentPreRunE`)
and **P1.M3.T4** (generate orchestrator: consumes the resolved `*Config`). Transitively US8
(configuration & precedence, FR34/FR35) and every user who configures Stagecoach via env vars or flags.

**Use Case**: A user runs `STAGECOACH_PROVIDER=gemini stagecoach --model flash --verbose` or
`stagecoach --provider claude --no-color`. `Load` merges defaults + files + git + env + flags and the
explicit flag wins; an unset `--model` leaves the env/file/git/default value intact.

**User Journey**: (internal API until P1.M4) cobra `PersistentPreRunE` → `cfg, err := config.Load(ctx,
config.LoadOpts{ConfigPathOverride: <from --config>, RepoDir: repoRoot, Flags: cmd.Flags()})` → on
error, surface + exit 1; on success, store the resolved `*Config` for the command + the generation
pipeline.

**Pain Points Addressed**: Removes "what order are layers applied", "how do env vars map to fields",
"can env/CLI force a bool false", "how do I detect an explicitly-set flag vs a default", "does
`STAGECOACH_CONFIG` pick a file or set a value", and "where does pflag enter the module" ambiguity for
P1.M4 and P1.M3.T4.

## Why

- **Closes the configuration system (P1.M1.T4).** Layers 1–4 (defaults, TOML×2, git) shipped in
  S1/S2/S3; S4 adds layers 5 + 7 (env + CLI) and the `Load()` glue, producing the single resolved
  `*Config` every later stage consumes.
- **Unlocks the boolean-false escape hatch.** Until S4, no layer could turn a default `true` OFF
  (`auto_stage_all`, `verbose`, `strip_code_fence`) — `overlay()` skips zero values. S4's DIRECT env/CLI
  set is the documented fix, and it lands the `NoColor` field as config-resolvable for the first time.
- **No user-facing surface yet** (PRD "DOCS: none — internal"). The env/precedence reference docs ship
  with `config init` (P1.M4.T1.S4) and the README (P1.M5.T4.S1) — S4 is the engine they document.

## What

A compiled `internal/config` package exposing a `Load()` that returns one fully-resolved `*Config`,
plus the two overlay functions (`loadEnv`, `loadFlags`) and `parseTimeout`. `Load` honors the exact
§16.1 ordering and the §15.2/FR35 env+flag names; env/flags DIRECT-set their fields (booleans can be
false); unset CLI flags do not clobber lower layers. No `Config` struct change, no provider-manifest
type (P1.M2.T1), no edit to S1/S2/S3 files.

### Success Criteria

- [ ] `internal/config/load.go` exists, `package config`, imports stdlib (`context`, `fmt`, `os`,
      `strconv`, `time`) + `github.com/spf13/pflag` (the ONLY new third-party import).
- [ ] `Load(ctx, opts)` resolves the global path as `opts.ConfigPathOverride` (non-empty) → else
      `os.Getenv("STAGECOACH_CONFIG")` (non-empty) → else `globalConfigPath()`; applies layers 1–7 in
      §16.1 order; returns `&cfg` (never nil on success) or a wrapped error.
- [ ] `loadEnv` overlays `STAGECOACH_PROVIDER`/`STAGECOACH_MODEL` (non-empty→set),
      `STAGECOACH_TIMEOUT` (`parseTimeout`), `STAGECOACH_VERBOSE`/`STAGECOACH_NO_COLOR` (`ParseBool`→
      DIRECT set). A present-but-unparseable bool or timeout yields a wrapped error.
- [ ] `loadFlags` overlays ONLY `flags.Changed(name)` flags among `provider`/`model`/`timeout`/
      `verbose`/`no-color`; DIRECT-set (so `--no-color` can mean "false" semantics appropriately).
- [ ] `parseTimeout("120s")==120s` AND `parseTimeout("120")==120s`; `"abc"` → non-nil error.
- [ ] `go.mod`/`go.sum` contain `github.com/spf13/pflag`; `git diff go.mod go.sum` is NON-empty.
- [ ] `config.go`/`file.go`/`git.go` byte-unchanged by S4.
- [ ] `load_test.go` has the required tests (all passing); S1/S2/S3 tests stay green.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the S1 `Config`
shape (quoted below), the S2/S3 function signatures (quoted below), the §16.1 order, the §15.2/FR35
env+flag names, and the design calls above. No provider/generation-pipeline knowledge required — S4 is
pure layer-glue + two direct-set overlays + path resolution.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M1T4S4/research/findings.md
  why: the EMPIRICAL basis for every design call. FINDING 1 (pflag NEW dep), FINDING 2 (arch §2.6–2.8
       is a NON-authoritative sketch of an abandoned nested model — use PRD §15.2 env names + flat
       fields), FINDING 3 (env/CLI bool DIRECT set = the boolean-false escape hatch), FINDING 4
       (STAGECOACH_CONFIG = file PATH not a value), FINDING 5 (loadRepoLocalConfig frozen, reads CWD),
       FINDING 7 (parseTimeout dual-form + --timeout as STRING flag).
  critical: do NOT copy arch §2.6–2.8 verbatim — its `STAGECOACH_DEFAULT_*` names and typed
       `cfg.Provider[name]` manifest map target a model that does not exist in S1.

- file: internal/config/config.go
  why: the INPUT/OUTPUT contract — the EXACT flat `Config` struct S4 resolves (12 fields + `Providers`).
       S4 does NOT modify it.
  pattern: field names + types (`Provider string`, `Model string`, `Timeout time.Duration`,
       `AutoStageAll bool`, `Verbose bool`, `NoColor bool \`toml:"-"\``, `MaxDiffBytes int`, …,
       `Output string`, `StripCodeFence bool`, `Providers map[string]map[string]any \`toml:"-"\``).
  gotcha: `NoColor` is `toml:"-"` — EXCLUDED from layers 2–4 (file/git) by S1/S2/S3 — but S4 makes it
       settable via `STAGECOACH_NO_COLOR` (env) and `--no-color` (CLI). `Providers` raw map is populated
       ONLY by S2's TOML loader — S4 NEVER touches it (no env/CLI provider-manifest mutation).

- file: internal/config/file.go
  why: the S2 OUTPUT contract S4 composes. S4 CALLS (does not edit): `globalConfigPath()`,
       `loadTOML(path) (*Config, error)` (nil,nil if file absent), `loadRepoLocalConfig() (*Config,
       error)` (CWD-relative `.stagecoach.toml`, emits §19 notice), and `overlay(dst, src *Config)`
       (NON-ZERO field-by-field copy).
  pattern: `loadTOML`/`loadRepoLocalConfig`/`overlay` signatures + `globalConfigPath` (XDG→HOME→CWD
       fallback). `Load` wraps each call: `if g, err := loadTOML(p); err != nil { return nil, err }
       else if g != nil { overlay(&cfg, g) }`.
  gotcha: `overlay` is NON-ZERO — it CANNOT force a bool false / int 0 / string "". That is precisely
       why S4's env/CLI overlays set DIRECTLY (not via overlay) for the bool fields (FINDING 3). String
       and int env/flags can use either overlay or direct set (non-zero when present) — direct set is
       simpler and uniform; use it everywhere in loadEnv/loadFlags.

- file: internal/config/git.go
  why: the S3 OUTPUT contract S4 composes. S4 CALLS (does not edit): `loadGitConfig(repoDir string)
       (*Config, error)` (partial *Config, only found keys non-zero; nil-safe: always non-nil on
       success). `opts.RepoDir` is passed here.
  pattern: `loadGitConfig` reads camelCase `stagecoach.*` keys; returns non-nil *Config (possibly
       all-zero); errors on bad value / missing git binary. `Load` wraps it the same way as loadTOML.
  gotcha: `loadGitConfig` takes `repoDir` (a STRING path), NOT CWD. Pass `opts.RepoDir` (may be "" —
       the CLI layer will resolve the repo root in P1.M4; "" is fine for tests that pre-create a repo
       at a temp path). Do NOT modify git.go.

- file: internal/config/git_test.go
  section: `func initRepo(t *testing.T, dir string)`, `func setGitConfig(t *testing.T, dir, key, value string)`
  why: REUSABLE same-package test helpers (S3). `load_test.go` is `package config`, so it can call
       `initRepo`/`setGitConfig` DIRECTLY (no copy). Both use `t.Setenv("HOME", t.TempDir())` for
       global-config isolation.
  pattern: `initRepo(t, repo)`; `setGitConfig(t, repo, "stagecoach.provider", "pi")`. For the git
       PRECEDENCE layer in load_test, reuse these.
  gotcha: `_test.go` helpers ARE visible across files IN THE SAME package — so load_test.go reuses
       S3's helpers. load_test.go adds ONLY its own NEW helpers (TOML-file writer, chdir save/restore,
       pflag.FlagSet builder).

- url: https://pkg.go.dev/github.com/spf13/pflag#FlagSet
  why: `FlagSet.Changed(name) bool`, `FlagSet.GetString(name)`, `FlagSet.GetBool(name)`,
       `FlagSet.Lookup(name)`. The API S4's `loadFlags` uses.
  critical: `Changed(name)` is the ONLY correct way to know a flag was EXPLICITLY set (vs left at its
       default). Do NOT compare the flag value to its default — a user setting `--provider
       <same-as-default>` must still count as "set". For a string flag, `GetString`; for bool,
       `GetBool`. (For `--timeout` use `GetString`+`parseTimeout` per FINDING 7 — string flag.)

- url: https://git-scm.com/docs/git-config / https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html
  why: (informational) env-var semantics. `os.LookupEnv` distinguishes unset vs empty; PRD §15.2 env
       vars are PRESENCE-semantic. For STRINGS use `os.Getenv("X") != ""`; for BOOLS parse the value
       with `strconv.ParseBool` (accepts 1/0/t/f/T/F/true/false/TRUE/FALSE/…). A present but empty
       `STAGECOACH_VERBOSE=` is treated as "not set" (skip) to match the string convention.

- file: PRD.md
  section: "16.1 Resolution order" (h3.57), "15.2 Global flags" (h3.53, the env+flag NAME table +
       defaults), "9.8" (h3.24, FR34/FR35)
  why: §16.1 fixes the layer ORDER; §15.2 is the AUTHORITATIVE env-var + flag-name + default table
       (overrides arch §2.6's invented `STAGECOACH_DEFAULT_*`); FR34 = "higher wins", FR35 = the env
       var set. Defaults: provider "" (auto-detect), model "" (manifest default), timeout "120s",
       verbose false, no-color TTY-aware.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2  (pflag ABSENT)
go.sum                          # no spf13 entries
internal/
  config/
    config.go                   # S1: Config (12 fields + Providers) + Defaults()        ← UNCHANGED by S4
    config_test.go              # S1: TestDefaults + TestTOMLMarshalKeys...               ← UNCHANGED (stay green)
    file.go                     # S2: loadTOML/overlay/loadRepoLocalConfig/globalConfigPath ← UNCHANGED (S4 CALLS these)
    file_test.go                # S2: TestLoadTOML*/TestOverlay*/TestGlobalConfigPath/...  ← UNCHANGED
    git.go                      # S3: loadGitConfig + gitConfigGet/gitConfigBool/gitExec  ← UNCHANGED (S4 CALLS loadGitConfig)
    git_test.go                 # S3: initRepo/setGitConfig + TestLoadGitConfig_*          ← UNCHANGED (S4 REUSES helpers)
    load.go                     # NEW (S4) ← Load + LoadOpts + loadEnv + loadFlags + parseTimeout
    load_test.go                # NEW (S4) ← table-driven precedence tests
  git/                          # T2/T3 (Git interface + gitRunner.run) — untouched by S4
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  config/
    load.go                     # NEW — Load + LoadOpts + loadEnv + loadFlags + parseTimeout
    load_test.go                # NEW — table-driven precedence (env/file/flag/git layering)
go.mod                          # + require github.com/spf13/pflag <v>   (NEW dep)
go.sum                          # + spf13/pflag entries                   (NEW dep)
# config.go / config_test.go / file.go / file_test.go / git.go / git_test.go UNCHANGED
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (FINDING 1): pflag is NOT in go.mod today. S4 MUST `go get github.com/spf13/pflag`, which
// adds it to BOTH go.mod (require) AND go.sum. The S3-style gate "git diff --exit-code go.mod go.sum
// empty" is INVERTED for S4 — the diff MUST show pflag added; an empty diff means the import fails to
// resolve and `go build`/`go test` error with "no required module provides package .../pflag".

// CRITICAL (FINDING 2): arch go_ecosystem_patterns.md §2.6–2.8 is a NON-AUTHORITATIVE sketch. It
// targets an ABANDONED nested-struct Config (cfg.Defaults.Provider, cfg.Provider[name].APIKey typed
// map). DO NOT copy it. Use PRD §15.2/FR35 env names (STAGECOACH_PROVIDER/MODEL/TIMEOUT/CONFIG/VERBOSE/
// NO_COLOR — NOT STAGECOACH_DEFAULT_*), and set FLAT fields (cfg.Provider/cfg.Model strings — NOT a
// typed manifest map). The sketch's CORE IDEAS (presence-check env; flags.Changed()) ARE correct.

// CRITICAL (FINDING 3): env + CLI bool overlays set DIRECTLY on the *Config, NOT via overlay().
// overlay() is non-zero and cannot force a bool false. loadEnv/loadFlags do: if present/Changed, set
// cfg.Field = parsedValue DIRECTLY (so Verbose=false, NoColor=false work). This is THE fix for the
// S2/S3-documented boolean-false limitation. (String/int fields can also direct-set; uniform it.)

// CRITICAL (FINDING 4): STAGECOACH_CONFIG selects the GLOBAL file PATH (layer 2), it is NOT a layer-5
// value. Resolve at Load top: opts.ConfigPathOverride (non-empty) > os.Getenv("STAGECOACH_CONFIG")
// (non-empty) > globalConfigPath(). loadEnv must NOT touch STAGECOACH_CONFIG. loadFlags must NOT touch
// the "config" flag (the cobra caller fills opts.ConfigPathOverride).

// CRITICAL (FINDING 5): loadRepoLocalConfig() (S2) is FROZEN — takes no args, reads CWD .stagecoach.toml,
// emits the §19 notice. Call it AS-IS. opts.RepoDir feeds ONLY loadGitConfig(opts.RepoDir). Do NOT edit
// file.go. Tests of the repo-local layer use os.Chdir into a temp dir + t.Cleanup to restore CWD.

// CRITICAL (FINDING 6): ctx is in the signature (Load(ctx, opts)) but frozen loaders take no ctx. Do ONE
// ctx.Err() check at Load entry; return early on cancellation. Not a cancellation thread.

// CRITICAL (FINDING 7): --timeout should be a pflag STRING flag (recommend in P1.M4.T1.S1) so loadFlags
// uses flags.GetString("timeout") + parseTimeout (both "120s" and "120" work, identical to env).
// parseTimeout: time.ParseDuration(s) first; on error strconv.Atoi(s) -> time.Duration(n)*time.Second;
// else wrapped error. STAGECOACH_TIMEOUT env uses the same parseTimeout.

// CRITICAL (FINDING 8): use os.LookupEnv to tell "unset" from "set-to-empty". For STRINGS, non-empty
// check (os.Getenv("X") != "") means "set". For BOOLS (STAGECOACH_VERBOSE/NO_COLOR), parse the value with
// strconv.ParseBool; a present-but-unparseable value is a wrapped load error (fail at load, like S2/S3).
// Treat empty string as "not set" (skip) for both, matching the string convention.

// CRITICAL (FINDING 9): loadFlags overlays ONLY the Config-backed flags: provider, model, timeout,
// verbose, no-color. Behavioral flags (--all/-a, --no-auto-stage, --dry-run, --version, --help/-h) are
// CLI control-flow handled in P1.M4.T1/T4 — S4 ignores them (they are not Config fields). If
// opts.Flags is nil, skip loadFlags entirely (programmatic callers).

// GOTCHA: NoColor (toml:"-") becomes config-resolvable for the FIRST time in S4 (via STAGECOACH_NO_COLOR
// env + --no-color CLI). The UI layer (P1.M4.T3.S1) later makes it TTY-aware at runtime; S4 just
// resolves the configured value (true/false; "" defaults stay false until UI decides).

// GOTCHA: Providers map is NEVER touched by loadEnv/loadFlags (no env/CLI provider-manifest mutation in
// v1 — manifests are P1.M2.T1; the raw map is S2's TOML domain). STAGECOACH_PROVIDER sets the Provider
// NAME string (cfg.Provider), NOT a manifest.

// GOTCHA: wrap loader errors with the layer context, e.g. fmt.Errorf("global config: %w", err), so the
// CLI can report WHICH layer failed. Each layer's call is guarded: err != nil -> return nil, err
// immediately (fail at load — consistent with S2/S3).

// GOTCHA (test): load_test.go reuses initRepo/setGitConfig from git_test.go (same package — visible
// across _test.go files). Add load_test.go's OWN helpers only for: writing a TOML file into a temp dir,
// os.Chdir save/restore (Go 1.22 has no t.Chdir), and building a standalone *pflag.FlagSet. Isolate
// discovery with t.Setenv("XDG_CONFIG_HOME", absTempDir) and t.Setenv("HOME", t.TempDir()).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — UNCHANGED by S4. Quoted only so the implementer maps env/flags to fields.
// (See the real file; do not retype.) Flat + plain-typed + resolved.
type Config struct {
	Provider            string        // <- STAGECOACH_PROVIDER  / --provider
	Model               string        // <- STAGECOACH_MODEL     / --model
	Timeout             time.Duration // <- STAGECOACH_TIMEOUT   / --timeout  (parseTimeout: "120s" or 120)
	AutoStageAll        bool          // NOT env/CLI settable in v1 (no STAGECOACH_/flag for it per §15.2)
	Verbose             bool          // <- STAGECOACH_VERBOSE   / --verbose   (DIRECT set; can be false)
	NoColor             bool          // <- STAGECOACH_NO_COLOR  / --no-color  (DIRECT set; can be true)
	MaxDiffBytes        int           // NOT env/CLI settable in v1
	MaxMdLines          int           // NOT env/CLI settable in v1
	MaxDuplicateRetries int           // NOT env/CLI settable in v1
	SubjectTargetChars  int           // NOT env/CLI settable in v1
	Output              string        // NOT env/CLI settable in v1
	StripCodeFence      bool          // NOT env/CLI settable in v1
	Providers           map[string]map[string]any // NEVER touched by loadEnv/loadFlags (S2 TOML domain)
}
// Defaults() (S1) returns Layer 1 by value. loadEnv/loadFlags overlay DIRECTLY; layers 2–4 use overlay().
```

```go
// internal/config/load.go — NEW. package config.
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/pflag"
)

// LoadOpts configures the Load() resolver. Populated by the caller (cobra PersistentPreRunE in
// P1.M4.T1.S1): ConfigPathOverride from the --config flag ("" if not passed); RepoDir = resolved repo
// root (for git config); Flags = cmd.Flags() (nil for programmatic callers -> no flag overlay).
type LoadOpts struct {
	ConfigPathOverride string        // from --config (CLI); "" => fall back to STAGECOACH_CONFIG, then discovery
	RepoDir            string        // repo root for git config (passed to loadGitConfig); "" is valid for tests
	Flags              *pflag.FlagSet // cobra/pflag set; nil => skip the CLI-flag layer
}

// Load resolves the full Stagecoach configuration by applying PRD §16.1 layers in precedence order
// (lowest → highest): (1) built-in Defaults(); (2) global TOML; (3) repo-local TOML; (4) repo git
// config; (5) STAGECOACH_* env vars; (7) CLI flags (only explicitly-set ones). Higher wins. Returns one
// fully-resolved *Config (never nil on success). Any layer's hard error (unreadable file, bad parse,
// git failure) is wrapped with its layer context and returned, failing load.
//
// The global-file PATH itself is resolved FIRST: opts.ConfigPathOverride (--config) > STAGECOACH_CONFIG
// (env) > globalConfigPath() discovery (FINDING 4). loadEnv/loadFlags set bool fields DIRECTLY (not via
// overlay) so a boolean can be forced false — the documented escape hatch the non-zero overlay layers
// cannot provide (FINDING 3). loadGitConfig(opts.RepoDir) is layer 4; loadRepoLocalConfig() reads CWD.
func Load(ctx context.Context, opts LoadOpts) (*Config, error) {
	// Honor ctx minimally — frozen loaders take no ctx; this is the cancellation seam.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}

	cfg := Defaults() // Layer 1 (by value)

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

	// Layer 3: repo-local TOML (CWD .stagecoach.toml; emits the §19 notice). nil => absent.
	if r, err := loadRepoLocalConfig(); err != nil {
		return nil, fmt.Errorf("repo config: %w", err)
	} else if r != nil {
		overlay(&cfg, r)
	}

	// Layer 4: repo git config (stagecoach.* keys). Non-nil partial *Config; errors propagate.
	gc, err := loadGitConfig(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("git config: %w", err)
	}
	if gc != nil {
		overlay(&cfg, gc)
	}

	// Layer 5: STAGECOACH_* env vars (DIRECT set — booleans can be false).
	if err := loadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("env config: %w", err)
	}

	// Layer 7 (6): CLI flags — ONLY explicitly-set ones (flags.Changed). Skipped if opts.Flags == nil.
	if opts.Flags != nil {
		loadFlags(&cfg, opts.Flags)
	}

	return &cfg, nil
}

// loadEnv overlays STAGECOACH_* environment variables (PRD §15.2/FR35, §16.1 layer 5). Presence-semantic:
// a PRESENT, non-empty value overrides; an unset/empty var is a no-op. Booleans are set DIRECTLY (not
// via overlay) so STAGECOACH_VERBOSE=false / STAGECOACH_NO_COLOR=true work — the escape hatch the non-zero
// overlay layers cannot provide (FINDING 3). STAGECOACH_CONFIG is NOT handled here (it selects the file
// path, resolved in Load). A present-but-unparseable bool/timeout is a wrapped error (fail at load).
func loadEnv(cfg *Config) error {
	if v, ok := os.LookupEnv("STAGECOACH_PROVIDER"); ok && v != "" {
		cfg.Provider = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_MODEL"); ok && v != "" {
		cfg.Model = v
	}
	if v, ok := os.LookupEnv("STAGECOACH_TIMEOUT"); ok && v != "" {
		d, err := parseTimeout(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_TIMEOUT: %w", err)
		}
		cfg.Timeout = d // DIRECT set (non-zero by construction when parsed)
	}
	if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
		}
		cfg.Verbose = b // DIRECT set — can be false (escape hatch)
	}
	if v, ok := os.LookupEnv("STAGECOACH_NO_COLOR"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_NO_COLOR: %w", err)
		}
		cfg.NoColor = b // DIRECT set — NoColor (toml:"-") becomes resolvable here for the first time
	}
	return nil
}

// loadFlags overlays explicitly-set CLI flags (PRD §15.2, §16.1 layer 7). For each Config-backed flag
// (provider, model, timeout, verbose, no-color), if fs.Changed(name) the value is read and set DIRECTLY
// — so --no-color / --verbose=false-style semantics work and an explicit flag beats every lower layer.
// Unchanged flags are IGNORED (a default value is NOT an override). Behavioral flags (--all, --dry-run,
// --version, --help) are NOT Config fields and are ignored. Assumes --timeout is registered as a pflag
// STRING flag (coordination note for P1.M4.T1.S1; FINDING 7). fs may be nil (Load guards it).
func loadFlags(cfg *Config, fs *pflag.FlagSet) {
	if fs.Changed("provider") {
		if v, err := fs.GetString("provider"); err == nil {
			cfg.Provider = v
		}
	}
	if fs.Changed("model") {
		if v, err := fs.GetString("model"); err == nil {
			cfg.Model = v
		}
	}
	if fs.Changed("timeout") {
		if v, err := fs.GetString("timeout"); err == nil {
			if d, perr := parseTimeout(v); perr == nil {
				cfg.Timeout = d
			}
		}
	}
	if fs.Changed("verbose") {
		if v, err := fs.GetBool("verbose"); err == nil {
			cfg.Verbose = v // DIRECT set — can be false (escape hatch)
		}
	}
	if fs.Changed("no-color") {
		if v, err := fs.GetBool("no-color"); err == nil {
			cfg.NoColor = v // DIRECT set
		}
	}
}

// parseTimeout parses a duration that may be EITHER a Go duration string ("120s", "2m") OR a bare
// integer (seconds: "120"). Used by both STAGECOACH_TIMEOUT (env) and --timeout (CLI). Returns a wrapped
// error if neither form parses. (S2's TOML layer uses time.ParseDuration only; S3's git layer uses
// Atoi-only; S4 accepts BOTH because env/CLI values are free-form strings.)
func parseTimeout(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid timeout %q (expected e.g. \"120s\" or 120)", s)
}
```

> **`--timeout` flag-type coordination (FINDING 7):** the code above assumes P1.M4.T1.S1 registers
> `--timeout` as a STRING flag (so `parseTimeout` handles both forms). If P1.M4 instead registers it as
> a `pflag.Duration` flag, swap the `timeout` branch to `if v, err := fs.GetDuration("timeout"); err ==
> nil { cfg.Timeout = v }` and drop `parseTimeout` for the CLI path (env still needs `parseTimeout`).
> This is a coordination NOTE, not a blocker — S4's tests build the flag set as a STRING flag to stay
> self-consistent, and `parseTimeout` is correct regardless.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the pflag dependency
  - RUN `go get github.com/spf13/pflag@latest` (or a recent stable pin). This updates go.mod (adds a
    `require github.com/spf13/pflag <v>` line) and go.sum (adds the module + go.sum hash entries).
  - VERIFY: `go mod tidy` leaves a clean module; `grep spf13/pflag go.mod` prints the require line;
    `grep spf13/pflag go.sum` prints >=2 lines.
  - WHY FIRST: load.go imports pflag; nothing compiles until it's present.

Task 2: CREATE internal/config/load.go — parseTimeout + LoadOpts (no pflag use yet)
  - IMPLEMENT parseTimeout(s string) (time.Duration, error) per the Data Models block: try
    time.ParseDuration; on error try strconv.Atoi -> time.Duration(n)*time.Second; else wrapped error.
  - IMPLEMENT type LoadOpts struct { ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet }.
  - IMPORTS: context, fmt, os, strconv, time, github.com/spf13/pflag (pflag used only by LoadOpts +
    loadFlags).
  - WHY: parseTimeout is dependency-free and trivially unit-testable; LoadOpts is the public input type.

Task 3: ADD loadEnv to internal/config/load.go
  - IMPLEMENT loadEnv(cfg *Config) error per the Data Models block: os.LookupEnv for
    STAGECOACH_PROVIDER/MODEL (set on non-empty), STAGECOACH_TIMEOUT (parseTimeout), STAGECOACH_VERBOSE/
    STAGECOACH_NO_COLOR (strconv.ParseBool -> DIRECT set). Empty/unset -> skip. Bad bool/timeout ->
    wrapped error. DO NOT touch STAGECOACH_CONFIG (path resolution is in Load).
  - GOTCHA: DIRECT set for booleans (FINDING 3) — do NOT build a partial *Config + overlay() here.

Task 4: ADD loadFlags + Load to internal/config/load.go
  - IMPLEMENT loadFlags(cfg, fs *pflag.FlagSet) per the Data Models block: for provider/model/timeout/
    verbose/no-color, gate on fs.Changed(name) and set DIRECTLY (GetString for strings, GetBool for
    bools, GetString+parseTimeout for timeout). Ignore --config/--all/--dry-run/--version/--help.
  - IMPLEMENT Load(ctx, opts LoadOpts) (*Config, error) per the Data Models block: ctx.Err() entry
    check; resolve globalPath (opts.ConfigPathOverride > STAGECOACH_CONFIG > globalConfigPath());
    cfg := Defaults(); layer 2 loadTOML(globalPath)+overlay; layer 3 loadRepoLocalConfig()+overlay;
    layer 4 loadGitConfig(opts.RepoDir)+overlay; layer 5 loadEnv(&cfg); layer 7 loadFlags(&cfg,fs) if
    opts.Flags != nil; return &cfg. Wrap each loader error with its layer context.
  - GOTCHA: call loadRepoLocalConfig() AS-IS (no args — it reads CWD + emits the §19 notice). Pass
    opts.RepoDir ONLY to loadGitConfig. Do NOT edit file.go/git.go.

Task 5: CREATE internal/config/load_test.go — helpers + parseTimeout + precedence (table-driven)
  - PACKAGE: `package config` (white-box). Imports: context, os, os/exec, path/filepath, testing, time,
    github.com/spf13/pflag. REUSE initRepo/setGitConfig from git_test.go (same package — visible).
  - HELPERS (load_test.go's OWN): writeConfigFile(t, dir, relPath, body) (os.WriteFile w/ MkdirAll);
    chdir(t, dir) (save os.Getwd; os.Chdir(dir); t.Cleanup restore — Go 1.22 has no t.Chdir);
    newFlagSet(t) (*pflag.FlagSet) (a fresh pflag.NewFlagSet("test", pflag.ContinueOnError) with the 5
    Config-backed flags pre-registered: provider/model as String "", timeout as String "", verbose as
    Bool false, no-color as Bool false). Use t.Setenv("XDG_CONFIG_HOME", absTempDir) +
    t.Setenv("HOME", t.TempDir()) to isolate discovery/global git config.
  - TEST parseTimeout: TestParseTimeout_DurationAndSeconds + TestParseTimeout_Invalid ("120s"==120s,
    "120"==120s, "2m"==2*time.Minute, "abc"/""/"-5s" -> error).
  - TEST loadEnv: TestLoadEnv_StringsTimeoutBools (set all 5, assert direct set incl.
    STAGECOACH_VERBOSE=false -> Verbose==false) + TestLoadEnv_NoColorResolvable (STAGECOACH_NO_COLOR=true
    -> NoColor==true; absent -> unchanged) + TestLoadEnv_BadBoolErrors + TestLoadEnv_BadTimeoutErrors +
    TestLoadEnv_EmptyStringsSkipped (STAGECOACH_PROVIDER="" leaves Provider unchanged).
  - TEST loadFlags: TestLoadFlags_ChangedOnly (set --provider=gemini via fs.Set; assert Provider==gemini
    and Model unchanged (not Changed)) — the §2.7 correctness test + TestLoadFlags_BoolDirect
    (--no-color=true -> NoColor==true) + TestLoadFlags_NoneChanged (nothing set -> cfg == Defaults()).
  - TEST Load PRECEDENCE (the contract's main case — table-driven): for each row, set up the layers and
    assert the winner per field. Cases: (a) TestLoad_DefaultsOnly (no files/env/flags -> pure Defaults);
    (b) TestLoad_GlobalFileOverridesDefaults (global TOML sets provider=pi -> Provider==pi, rest default);
    (c) TestLoad_RepoFileOverridesGlobal (global provider=pi, repo .stagecoach.toml provider=claude,
    chdir into repo -> Provider==claude); (d) TestLoad_GitOverridesRepoFile (repo file provider=claude,
    git stagecoach.provider=gemini -> Provider==gemini); (e) TestLoad_EnvOverridesGit (git provider=gemini,
    STAGECOACH_PROVIDER=pi -> Provider==pi); (f) TestLoad_CLIOverridesEnv (env provider=pi, --provider
    flag=claude -> Provider==claude); (g) TestLoad_UnsetCLIFlagDoesNotOverride (env provider=pi, flagset
    with NO Set on provider -> Provider==pi; proves flags.Changed gating); (h)
    TestLoad_EnvBoolFalseEscape (Defaults Verbose==false; set Verbose true via global file... NOTE file
    overlay cannot set false; instead: global file verbose=true -> Verbose==true, then
    STAGECOACH_VERBOSE=false -> Verbose==false; proves the escape hatch); (i) TestLoad_NoColorFromCLI
    (--no-color -> NoColor==true even though toml:"-"). Use a helper that builds a temp HOME with
    stagecoach/config.toml, a temp repo dir with .stagecoach.toml + git config, chdir into the repo, set
    env, set flags, call Load(context.Background(), LoadOpts{...}), assert fields.

Task 6: TEST Load path resolution + error propagation
  - TestLoad_ConfigPathOverride_CLI: write a TOML at /tmp/x.toml provider=custom; Load with
    LoadOpts{ConfigPathOverride: that path} -> Provider==custom (proves --config beats discovery).
  - TestLoad_STAGECOACH_CONFIG_EnvPath: with XDG/HOME pointing at a default-dir TOML provider=A, set
    STAGECOACH_CONFIG=<other file> provider=B -> Provider==B (env path beats discovery). THEN set
    opts.ConfigPathOverride=<yet another file> provider=C -> Provider==C (--config beats env path).
  - TestLoad_BadGlobalFileErrors: write a malformed TOML at the global path -> Load returns non-nil err
    wrapping "global config".
  - TestLoad_BadEnvBoolErrors: STAGECOACH_VERBOSE=notabool -> Load returns non-nil err wrapping "env
    config"/"STAGECOACH_VERBOSE".
  - TestLoad_GitConfigErrorPropagates: a repo whose stagecoach.timeout is non-integer (set via
    setGitConfig) -> Load returns non-nil err wrapping "git config". (Proves layer-4 errors surface.)
  - TestLoad_NilFlagsSkipped: LoadOpts{Flags: nil} -> no panic, env/file/git still applied.

Task 7: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). `git diff go.mod go.sum` MUST be NON-empty (pflag added).
    config.go/file.go/git.go MUST be byte-unchanged (`git diff --exit-code internal/config/config.go
    internal/config/file.go internal/config/git.go` empty). S1/S2/S3 tests MUST stay green.
```

### Implementation Patterns & Key Details

```go
// The layer-application idiom in Load — guard err, guard nil, then overlay (layers 2–4) or direct-set
// (layers 5–7). This is the EXACT §16.1 order; do not reorder.
if g, err := loadTOML(globalPath); err != nil {
	return nil, fmt.Errorf("global config: %w", err)   // hard error -> fail load
} else if g != nil {
	overlay(&cfg, g)                                    // nil => file absent => no-op (not an error)
}

// DIRECT set for env/CLI booleans — THE fix for the non-zero overlay limitation:
if v, ok := os.LookupEnv("STAGECOACH_VERBOSE"); ok && v != "" {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("STAGECOACH_VERBOSE: %w", err)
	}
	cfg.Verbose = b   // <-- direct, NOT overlay(&cfg, &Config{Verbose:b}); can be false
}

// flags.Changed is the ONLY correct "explicitly set" test (arch §2.7). Comparing to the default value
// is WRONG (a user setting --provider <same-as-default> must still override).
if fs.Changed("provider") {
	if v, err := fs.GetString("provider"); err == nil { cfg.Provider = v }
}

// Global-path resolution — STAGECOACH_CONFIG is a PATH selector, NOT a value (FINDING 4):
globalPath := opts.ConfigPathOverride
if globalPath == "" {
	if env := os.Getenv("STAGECOACH_CONFIG"); env != "" {
		globalPath = env
	} else {
		globalPath = globalConfigPath()   // XDG_CONFIG_HOME -> HOME -> CWD fallback (S2)
	}
}
```

```go
// load_test.go — the precedence table (sketch). One helper builds all layers in temp dirs + chdir.
func TestLoad_CLIOverridesEnv(t *testing.T) {
	env := newLoadEnv(t)               // helper: temp HOME+repo, isolation, helpers wired
	env.setGlobalTOML(t, "[defaults]\nprovider = \"pi\"\n")
	env.chdirToRepo(t)                 // repo dir; .stagecoach.toml absent here
	t.Setenv("STAGECOACH_PROVIDER", "gemini")
	fs := newFlagSet(t)
	if err := fs.Set("provider", "claude"); err != nil { t.Fatal(err) }   // Changed("provider")==true

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: env.repo, Flags: fs})
	if err != nil { t.Fatalf("Load: %v", err) }
	if cfg.Provider != "claude" {
		t.Errorf("Provider=%q want claude (CLI flag beats env+file+default)", cfg.Provider)
	}
	// Unchanged flags must NOT override — model is unset at every layer:
	if cfg.Model != "" {
		t.Errorf("Model=%q want \"\" (nobody set it)", cfg.Model)
	}
}

// The escape-hatch test: env bool can force false even after a file set it true.
func TestLoad_EnvBoolFalseEscape(t *testing.T) {
	env := newLoadEnv(t)
	env.setGlobalTOML(t, "[defaults]\nverbose = true\n")  // file overlay: Verbose=true (non-zero OK)
	env.chdirToRepo(t)
	t.Setenv("STAGECOACH_VERBOSE", "false")                 // env DIRECT set -> Verbose=false (escape hatch)

	cfg, err := Load(context.Background(), LoadOpts{RepoDir: env.repo})
	if err != nil { t.Fatalf("Load: %v", err) }
	if cfg.Verbose {
		t.Errorf("Verbose=true want false (env DIRECT set must override the file's true)")
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: ADD github.com/spf13/pflag (the ONLY new dep). `go get github.com/spf13/pflag@latest`;
    `go mod tidy`. The diff MUST be NON-empty (opposite of S3's gate).

CONFIG STRUCT (internal/config/config.go):
  - change: NONE. S4 only READS Config; it adds no fields/retypes. NoColor finally gets env/CLI sources,
    but its FIELD is unchanged (toml:"-").

FROZEN FILES (do NOT edit):
  - file.go (S2): Load CALLS globalConfigPath/loadTOML/loadRepoLocalConfig/overlay AS-IS.
  - git.go (S3): Load CALLS loadGitConfig(opts.RepoDir) AS-IS.
  - config.go/config_test.go/file_test.go/git_test.go: unchanged (S4 tests ADD load_test.go).

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M4.T1.S1 (CLI): in cobra PersistentPreRunE, `cfg, err := config.Load(cmd.Context(),
        config.LoadOpts{ConfigPathOverride: <from --config flag>, RepoDir: repoRoot,
        Flags: cmd.Flags()})`; on err, print + exit 1. Register --timeout as a STRING flag (FINDING 7).
  - P1.M3.T4 (generate): consumes the resolved *Config (cfg.Provider, cfg.Timeout, cfg.Verbose, ...).

NO DATABASE / NO ROUTES / NO FILE WRITING / NO CLI WIRING (config init is P1.M4.T1.S4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (go get pflag):
grep spf13/pflag go.mod && grep spf13/pflag go.sum   # MUST both print (pflag added).
go mod tidy && go build ./...                          # Whole module compiles. Expect exit 0.

# After Tasks 2–4 (load.go):
gofmt -w internal/config/load.go internal/config/load_test.go
test -z "$(gofmt -l internal/config/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/        # (and `go vet ./...`) Expect zero diagnostics.
# Expected: all clean. pflag import resolves. load.go/builds.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests + all existing tests (white-box; REAL git required in PATH for the git-layer cases):
go test -race ./internal/config/ -v
# Expected: PASS — S1's TestDefaults/TestTOMLMarshalKeys..., S2's TestLoadTOML*/TestOverlay*/...,
#   S3's TestLoadGitConfig_*/TestGitConfigGet_* AND S4's TestParseTimeout_*, TestLoadEnv_*,
#   TestLoadFlags_*, TestLoad_* (precedence table + path resolution + error propagation).
#   NOTE: the git-layer precedence tests exec the REAL git binary in t.TempDir() repos — git must be
#   on PATH (it is in this dev/CI env; S3's TestLoadGitConfig_GitBinaryMissing covers the no-git path).

# Full suite must stay green (no regression in internal/git or elsewhere):
go test -race ./...
# Expected: all packages PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + dependency + additive-scope checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff go.mod go.sum | grep -q spf13/pflag && echo "pflag ADDED (expected)"   # MUST print.
# Confirm S4 did NOT touch S1/S2/S3 files:
git diff --exit-code internal/config/config.go internal/config/config_test.go \
  internal/config/file.go internal/config/file_test.go internal/config/git.go internal/config/git_test.go \
  && echo "S1/S2/S3 files UNCHANGED by S4"   # MUST be empty.
grep -n 'func Load' internal/config/load.go          # prints the Load definition line.
grep -n 'func loadEnv\|func loadFlags\|func parseTimeout' internal/config/load.go
# Expected: binary builds; pflag in go.mod/go.sum; S1/S2/S3 byte-unchanged; Load/loadEnv/loadFlags/
#   parseTimeout present.

# Smoke Load end-to-end against a real layered setup (sanity for P1.M3.T4/P1.M4 authors):
TMP=$(mktemp -d); mkdir -p "$TMP/xdg/stagecoach"
printf '[defaults]\nprovider = "pi"\ntimeout = "60s"\n' > "$TMP/xdg/stagecoach/config.toml"
REPO=$(mktemp -d); git -C "$REPO" init -q
printf '[defaults]\nprovider = "claude"\n' > "$REPO/.stagecoach.toml"
git -C "$REPO" config stagecoach.provider gemini      # camelCase (S3); beats both files
( cd "$REPO" && XDG_CONFIG_HOME="$TMP/xdg" STAGECOACH_PROVIDER=pi \
    go test ./internal/config/ -run 'TestLoad_CLIOverridesEnv' -v )   # already asserts env>git>file
# (Or add a one-off /tmp snippet calling config.Load directly — the table tests cover this already.)
rm -rf "$TMP" "$REPO"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Precedence matrix smoke (mirrors the §16.1 contract for ONE field across all 7 layers). Optional but
# recommended: a throwaway /tmp/main_go that builds a pflag.FlagSet + env + files + git and prints the
# resolved cfg.Provider / cfg.Verbose / cfg.NoColor / cfg.Timeout, to eyeball "higher wins".
cat > /tmp/smoke_load_test.go <<'EOF'
package main
import ("context";"fmt";"os";"github.com/dustin/stagecoach/internal/config";"github.com/spf13/pflag")
func main(){
  fs := pflag.NewFlagSet("smoke", pflag.ContinueOnError)
  fs.String("provider","",""); fs.String("timeout","",""); fs.Bool("verbose",false,""); fs.Bool("no-color",false,"")
  _=fs.Set("no-color","true")
  cfg,err := config.Load(context.Background(), config.LoadOpts{Flags: fs})
  if err!=nil{fmt.Println("err:",err);os.Exit(1)}
  fmt.Printf("provider=%q verbose=%v noColor=%v timeout=%v\n", cfg.Provider,cfg.Verbose,cfg.NoColor,cfg.Timeout)
}
EOF
# (Run from a repo scratch dir with layers set up; or rely on the in-package table tests which already
#  assert the full matrix. golangci-lint is project-wide: `make lint`.)
golangci-lint run ./internal/config/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint is project-wide)."
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`.
- [ ] Level 2 green: `go test -race ./internal/config/ -v` (S1+S2+S3+S4 tests) AND `go test -race ./...`.
- [ ] Level 3: binary builds; `go.mod`/`go.sum` contain `github.com/spf13/pflag` (diff NON-empty);
      `config.go`/`file.go`/`git.go`/their tests byte-unchanged by S4; `Load`/`loadEnv`/`loadFlags`/
      `parseTimeout` present in `load.go`.

### Feature Validation

- [ ] `Load` applies the 7 layers in §16.1 order; a higher layer overrides a lower one per field
      (TestLoad_GlobalFileOverridesDefaults → … → TestLoad_CLIOverridesEnv).
- [ ] Env/CLI booleans set DIRECTLY and CAN force `false` (TestLoad_EnvBoolFalseEscape,
      TestLoad_NoColorFromCLI, TestLoadEnv_StringsTimeoutBools) — the S2/S3 escape hatch now works.
- [ ] An UNSET CLI flag does NOT override lower layers (TestLoad_UnsetCLIFlagDoesNotOverride,
      TestLoadFlags_ChangedOnly) — `flags.Changed()` correctness.
- [ ] `STAGECOACH_CONFIG`/`--config` select the global FILE path, not a value, with CLI > env >
      discovery (TestLoad_ConfigPathOverride_CLI, TestLoad_STAGECOACH_CONFIG_EnvPath).
- [ ] `parseTimeout` accepts `"120s"` AND `120` (TestParseTimeout_DurationAndSeconds); rejects garbage.
- [ ] Hard errors (bad global TOML, bad env bool, git timeout parse) propagate wrapped with the layer
      name (TestLoad_BadGlobalFileErrors, TestLoad_BadEnvBoolErrors, TestLoad_GitConfigErrorPropagates).
- [ ] `opts.Flags == nil` is safe (TestLoad_NilFlagsSkipped).

### Code Quality Validation

- [ ] Follows the flat `Config` + frozen S1/S2/S3 function signatures (no retype, no signature change).
- [ ] env/flag names match PRD §15.2/FR35 (NOT arch §2.6's `STAGECOACH_DEFAULT_*`).
- [ ] pflag is the ONLY new import; no other dependency creep (`go mod tidy` clean).
- [ ] Errors are wrapped with layer context and use `%w`; no panics on missing files/nil flags.

### Documentation & Deployment

- [ ] Code is self-documenting (each function carries the §16.1 layer + FINDING it implements).
- [ ] No new env vars invented beyond the §15.2 set; no new user-facing surface (docs ship in P1.M4).
- [ ] The `--timeout` STRING-flag recommendation is recorded for P1.M4.T1.S1 (FINDING 7 / inline note).

---

## Anti-Patterns to Avoid

- ❌ Don't copy arch §2.6–2.8 verbatim — it targets an abandoned nested-struct `Config`. Use PRD §15.2
  env names + the flat `Config` fields.
- ❌ Don't route env/CLI booleans through `overlay()` — it's non-zero and CANNOT force `false`. DIRECT-set.
- ❌ Don't compare a flag's value to its default to detect "set" — use `flags.Changed(name)` ONLY.
- ❌ Don't treat `STAGECOACH_CONFIG` as a layer-5 value — it selects the global file PATH (resolve in Load).
- ❌ Don't edit S1/S2/S3 files (`config.go`/`file.go`/`git.go` + tests) — S4 is additive + go.mod only.
- ❌ Don't forget `go get github.com/spf13/pflag` — without it `load.go` won't compile (pflag is absent today).
- ❌ Don't mutate `Config.Providers` from env/CLI — provider manifests are P1.M2.T1; the raw map is S2's.
- ❌ Don't conflate `loadRepoLocalConfig()` (CWD, frozen, no args) with `loadGitConfig(opts.RepoDir)` —
  only the latter takes the repo dir.
