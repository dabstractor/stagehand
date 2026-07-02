# Research notes — P1.M5.T2.S1 (internal/config/file.go)

## Verified behaviors (live checks against this codebase)

### go-toml/v2 pointer-field presence semantics (CRITICAL for overlay design)
- `*Sub` (pointer to a table struct): **nil when the table is absent**, non-nil when present.
- `*string`/`*int`/`*bool` (pointer scalars inside a present table): **nil when the KEY is
  absent**, non-nil even when the value is the zero value (`""`, `0`, `false`).
- `map[string]X`: nil when no entries present, populated otherwise.

⇒ This is exactly what we need: pointer DTO fields distinguish "not set by source" (nil)
from "explicitly set to zero" (non-nil). The §16.2 golden file sets `model = ""` and
`verbose = false` — these MUST surface as non-nil pointers so a higher-precedence source
can override a lower one with a zero value.

### `git config --get` exit codes (real git binary)
- Key present → exit 0, value on stdout.
- Key absent   → **exit 1** (this is the "not set" signal, NOT an error → nil pointer).
- Real failure (not a repo, corrupt config) → exit 128 → MUST surface as error.
⇒ readGitConfig must inspect `*exec.ExitError.ExitCode()` and treat 1 as "missing".

### `git config --bool --get`
- Returns `true`/`false` on stdout, exit 0 when set; exit 1 when absent.

## Dependency edges (must preserve)
- config → provider (one-way): file.go imports `provider.Manifest` for the `[provider.<name>]`
  tables. provider/registry does NOT import config (plan_overview key decision 1).
- config does NOT import internal/git: readGitConfig uses os/exec directly for
  `git config --get`. Keeps config decoupled from the git plumbing package.

## Package-doc ownership convention
- config.go OWNS the `// Package config` doc. file.go uses a plain `package config` line
  (mirrors internal/git/log.go deferring to git.go).

## House test conventions
- White-box: `package config` (NOT config_test).
- stdlib `testing` only — NO testify (see config_test.go, defaults_test.go).
- Real binary integration allowed where needed (internal/git tests use real git; PRD §20.1
  layer 2). file.go tests will write temp TOML files (os/t.TempDir) and, for readGitConfig,
  create a temp git repo + real `git config` invocations.

## Overlay design (decided)
Pointer-based Overlay (nil = "this source did not set the field"):
- Provider/Model/Output *string
- Timeout *time.Duration
- AutoStageAll/Verbose/NoColor/StripCodeFence *bool
- MaxDiffBytes/MaxMdLines/MaxDuplicateRetries/SubjectTargetChars *int
- ProviderOverrides map[string]provider.Manifest  (nil = no provider tables in this source)

Each reader returns a FRESH Overlay containing only the fields its source set. Merge into a
base Config is T3.S1's job (Load). Combining provider-override tables across global+repo
sources (field-merge of provider overrides) is also T3.S1's job — this task only emits each
source's own provider tables.

## Functions (decided API)
- GlobalConfigPath() (string, error) — EXPORTED (reused by CLI config path/init M7.T3.S2).
  $XDG_CONFIG_HOME/stagehand/config.toml else $HOME/.config/stagehand/config.toml.
- parseFile(path string) (Overlay, error) — unexported shared core: read+unmarshal a TOML
  file into fileDTO → Overlay. Missing file (fs.ErrNotExist) → empty Overlay, nil error.
  Malformed TOML → wrapped non-nil error. Used by readGlobalFile/readRepoFile AND by T3.S1
  for the --config override.
- readGlobalFile() (Overlay, error) → parseFile(GlobalConfigPath()).
- readRepoFile(repoDir string) (Overlay, error) → parseFile(filepath.Join(repoDir, ".stagehand.toml")).
- readGitConfig(repoDir string) (Overlay, error) — per-key `git config --get stagehand.<key>`
  in repoDir; exit 1 = skip; --bool for bool keys.
