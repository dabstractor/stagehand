# P1.M1.T4.S4 — Research Findings (env + CLI flags + Load() orchestrator)

Source: direct inspection of `internal/config/{config,file,git}.go`, `git_test.go`, `go.mod`,
`go.sum`, `plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md` §2.6–2.8, and the S1/S2/S3
PRPs. All facts below are verified against the current tree (config tests pass: `ok ... cached`).

---

## FINDING 1 — pflag is NOT a dependency yet. S4 ADDS it (go.mod + go.sum change, unlike S3).

`go.mod` currently has exactly ONE require:
```
require github.com/pelletier/go-toml/v2 v2.4.2
```
`go.sum` has NO `spf13/pflag` or `cobra` entries. `LoadOpts` carries a `*pflag.FlagSet` and the
contract mandates `flags.Changed()` → S4 must `go get github.com/spf13/pflag`, which adds it to BOTH
`go.mod` (require) and `go.sum`. This is the FIRST subtask to introduce a CLI-stdlib dependency; cobra
itself arrives later in P1.M4.T1.S1 (cobra embeds a pflag.FlagSet, so P1.M4.T1.S1 will pass
`cmd.Flags()` into `Load`). **The S3 validation gate "`git diff --exit-code go.mod go.sum` empty" is
INVERTED for S4** — the diff MUST show pflag added; an empty diff means the dependency was not added
and `load.go` will not compile.

## FINDING 2 — arch §2.6–2.8 is a NON-AUTHORITATIVE sketch based on an ABANDONED model.

`go_ecosystem_patterns.md` §2.6–2.8 was written against an OLD nested-struct `Config`
(`cfg.Defaults.Provider`, `cfg.Defaults.Model`, a typed `cfg.Provider[name].APIKey` map). The ACTUAL
`Config` (S1, `internal/config/config.go`) is **flat + plain-typed** (`Provider string`,
`Model string`, `Timeout time.Duration`, `MaxDiffBytes int`, …) with no nested `Defaults` and no
typed provider-manifest map (`Providers` is `map[string]map[string]any`, `toml:"-"`, S2-owned).
Therefore arch §2.6–2.8 code MUST NOT be copied verbatim. Two concrete divergences:

- **Env var NAMES.** arch §2.6 invents `STAGECOACH_DEFAULT_PROVIDER`, `STAGECOACH_DEFAULT_MODEL`. The
  AUTHORITATIVE names are PRD §15.2 / FR35 (also the contract): `STAGECOACH_PROVIDER`,
  `STAGECOACH_MODEL`, `STAGECOACH_TIMEOUT`, `STAGECOACH_CONFIG`, `STAGECOACH_VERBOSE`,
  `STAGECOACH_NO_COLOR`. Use the PRD names.
- **Flag overlay targets.** arch §2.7 sets `cfg.Provider[cfg.Defaults.Provider].Model` (typed
  manifest). S4 sets the flat `cfg.Model` string only. No provider-manifest mutation in S4
  (manifests are P1.M2.T1; `Config.Providers` raw map is populated ONLY by S2's TOML loader).

The arch sketches' CORE IDEAS are still correct and ARE followed: presence-check env vars
(`os.Getenv != ""`), and `flags.Changed(name)` to detect explicitly-set CLI flags. Only the field
targets and names are corrected to the real flat `Config`.

## FINDING 3 — S2's `overlay()` is NON-ZERO. S4's env/CLI bool overlays set DIRECTLY (the escape hatch).

`overlay(dst, src *Config)` copies only NON-ZERO scalars (`file.go`). S2/S3 document a v1 limitation:
a layer cannot force a bool to `false` or an int/string to its zero value via `overlay`. **S4 resolves
exactly this limitation** for the two highest layers:

- **Env (layer 5)** and **CLI flags (layer 7)** do NOT build a partial `*Config` + `overlay()`. They
  set `Config` fields DIRECTLY on the target: env when the var is PRESENT (`os.LookupEnv` / non-empty),
  flags when `flags.Changed(name)`. So `STAGECOACH_VERBOSE=false` and `--no-color` (explicitly false
  meaning) correctly produce `Verbose=false` / `NoColor=false` after Load. This is the documented
  "force false via env/CLI" escape hatch from S2/S3 — S4 is where it lands.

Implication: `loadEnv`/`loadFlags` mutate `*Config` in place; they are NOT pure overlay-partial
builders. `loadGitConfig`/`loadTOML` (layers 2–4) remain partial-`*Config` + `overlay()`.

## FINDING 4 — `STAGECOACH_CONFIG` controls the global FILE PATH, not a layer-5 value.

PRD §15.2: both `--config` and `STAGECOACH_CONFIG` mean "Path to a config file (overrides discovery)."
They select WHICH file becomes layer 2 — they are NOT a value overlay. Precedence for the path
itself: `--config` (CLI, carried in `opts.ConfigPathOverride`) > `STAGECOACH_CONFIG` (env) > discovery
(`globalConfigPath()`). S4 resolves this at the TOP of `Load`, then layers 2–4 load. `loadEnv` does
NOT touch `STAGECOACH_CONFIG` as a value (it's consumed for the path). `loadFlags` does NOT touch
`config` (the caller — cobra PersistentPreRunE, P1.M4.T1.S1 — populates `opts.ConfigPathOverride`
from the `--config` flag; S4 just honors it).

## FINDING 5 — `loadRepoLocalConfig()` (S2) reads CWD; `opts.RepoDir` feeds ONLY `loadGitConfig`.

S2's `loadRepoLocalConfig()` is frozen: signature `func loadRepoLocalConfig() (*Config, error)`,
reads `.stagecoach.toml` RELATIVE TO CWD, and emits the §19 provider-redirect notice to stderr. It
takes NO `repoDir`. S4 calls it AS-IS (no S2 modification — S2 is COMPLETE). `opts.RepoDir` is passed
to `loadGitConfig(opts.RepoDir)` only. In normal operation CWD == repoDir (PRD §11.2: process CWD =
repo root), so the two coincide; tests that exercise the repo-local layer use `os.Chdir` into a temp
dir (Go 1.22 has no `t.Chdir`).

## FINDING 6 — `ctx context.Context` is in the signature; frozen loaders don't accept it.

Contract: `Load(ctx, opts LoadOpts) (*Config, error)`. `loadTOML`, `loadRepoLocalConfig`, and
`loadGitConfig` (S2/S3) have NO ctx parameter and `loadGitConfig` uses `context.Background()`
internally. S4 honors `ctx` minimally: a single `ctx.Err()` check at entry (cancellation requested →
bail early). It is the seam for a future ctx-aware loader variant; not a full cancellation thread.

## FINDING 7 — timeout dual-form parse for `STAGECOACH_TIMEOUT`.

Contract: "For timeout, parse `120s` / integer seconds." S2's TOML layer uses `time.ParseDuration`
(`"120s"`); S3's git layer uses `strconv.Atoi`→seconds (`90`). `STAGECOACH_TIMEOUT` (env, a raw string)
may legally be EITHER form. S4 adds a shared `parseTimeout(s) (time.Duration, error)`:
1. `time.ParseDuration(s)` first — handles `"120s"`, `"2m"`, `"1h30m"`.
2. on error, `strconv.Atoi(s)` → `time.Duration(n) * time.Second` — handles bare `"120"`.
3. else wrapped error.

For the `--timeout` CLI flag: **recommend P1.M4.T1.S1 register `--timeout` as a pflag STRING flag**
(not `Duration`), so `loadFlags` reads it via `flags.GetString("timeout")` + `parseTimeout` and both
forms work identically across env and CLI. S4's own tests construct a pflag.FlagSet with `timeout` as
a string flag. (If P1.M4 later chooses a Duration flag, `loadFlags` should use `GetDuration` instead —
flag this as a coordination note, not a blocker.)

## FINDING 8 — reusable test helpers already exist in `git_test.go` (S3, same package).

`internal/config/git_test.go` (package `config`, written by S3 in parallel) defines:
- `func initRepo(t *testing.T, dir string)` — `git -C <dir> init` with minimal identity env.
- `func setGitConfig(t *testing.T, dir, key, value string)` — `git -C <dir> config <key> <value>`.

Both use `t.Setenv("HOME", t.TempDir())` for global-config isolation. Because `load_test.go` is the
SAME package, it can call these DIRECTLY (no copy). S4's `load_test.go` adds its own helpers only for
the NEW concerns: writing TOML files into temp dirs, `os.Chdir` save/restore for repo-local tests,
and building standalone `pflag.FlagSet`s.

## FINDING 9 — the flag set S4 overlays is a FIXED, small set (the Config-backed flags).

Per PRD §15.2 the global flags are `--provider --model --config --timeout --all/-a --no-auto-stage
--dry-run --verbose/-v --no-color --version --help/-h`. Of these, only the ones that map to a
`Config` field are overlaid by `loadFlags`: **`provider`, `model`, `timeout`, `verbose`, `no-color`**.
`config` is consumed for the path (FINDING 4). The behavioral flags `--all/-a`, `--no-auto-stage`,
`--dry-run`, `--version`, `--help/-h` are CLI control-flow handled in the command layer (P1.M4.T1/T4),
NOT Config fields — S4 ignores them. `NoColor` is the one field that is `toml:"-"` (excluded from the
file/git layers by S1/S2/S3) but IS settable via env (`STAGECOACH_NO_COLOR`, layer 5) and CLI
(`--no-color`, layer 7) — S4 is where `NoColor` first becomes config-resolvable (the UI layer
P1.M4.T3.S1 makes it TTY-aware at runtime; S4 just resolves the configured value).

## FINDING 10 — env-var values and the bool parse.

`os.LookupEnv` distinguishes "unset" from "set-to-empty". PRD §15.2 env vars are PRESENCE-semantic
(`STAGECOACH_PROVIDER` present = override). For strings use non-empty check (`os.Getenv("X") != ""`).
For bools (`STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR`) use `strconv.ParseBool` on the value
(accepts `1/0/t/f/T/F/true/false/TRUE/FALSE/...`); a present-but-unparseable bool is a wrapped load
error (fail at load, consistent with S2/S3's timeout/parse stance). Empty-string env (`STAGECOACH_VERBOSE=`
with nothing) is treated as "not set" (skip) to match the string convention.
