---
name: "P1.M1.T4.S3 — Git-config reader (stagecoach.* keys)"
description: |
  The THIRD subtask of the Configuration System (P1.M1.T4): read Stagecoach's per-repo **git config**
  layer (PRD §16.3, FR36) and yield a partial `*Config` for the precedence overlay. Implement
  `internal/config/git.go` with `loadGitConfig(repoDir string) (*Config, error)` (package `config`),
  which runs `git -C <repoDir> config --get stagecoach.<key>` for each known scalar key and a
  `gitConfigGet`/`gitConfigBool` exec helper, and `internal/config/git_test.go` (white-box, REAL git
  in a temp repo). INPUT = `Config` struct + `Defaults()` from P1.M1.T4.S1 (and the `Providers` field
  S2 adds). OUTPUT = a partial `*Config` (only set fields non-zero) consumed by S4's `Load()`
  orchestrator via S2's `overlay()`. PRD §16.1 (layer 4) + §16.3 (key example) + §9.8/FR36 + arch
  `go_ecosystem_patterns.md` §2.5 are the spec. No `go.mod`/`go.sum` change (stdlib `os/exec` only).

  ⚠️ **THE central design call — git-config keys are CAMELCASE, NOT snake_case.** Empirically (research
  §git_config_behavior.md FINDING A), `git config` REJECTS underscores in key names: `git config
  stagecoach.auto_stage_all on` → `error: invalid key: stagecoach.auto_stage_all` (exit 1, on BOTH read
  and write). Git's key grammar allows only alphanumeric + `-` in section/name. Therefore FR36's and
  the contract's literal snake_case names (`auto_stage_all`, `max_diff_bytes`, …) are **unusable**.
  The authoritative working form is the **PRD §16.3 example**, which uses **camelCase** (`autoStageAll
  = true`). S3 reads camelCase for every multi-word key: `stagecoach.autoStageAll`, `maxDiffBytes`,
  `maxMdLines`, `maxDuplicateRetries`, `subjectTargetChars`, `stripCodeFence`. Single-word keys
  (`provider`, `model`, `timeout`, `verbose`, `output`) are valid as-is. This is a **discovered
  correction** of the contract/FR36 naming (documented, not a contradiction to surface to humans — the
  §16.3 example is the spec that works). The §16.2 TOML file keeps snake_case (that is S2's decode
  layer; S3 does not touch it) — only the git-config layer is camelCase.

  ⚠️ **THE second design call — placement is `internal/config/git.go`, shelling out via `os/exec`
  directly (NOT `internal/git`).** `loadGitConfig` returns a `*config.Config`, so if it lived in
  `internal/git` there would be an import cycle (`git`→`config` for the return type AND
  `config`→`git` for S4's `Load()` calling it). `internal/git`'s `run()` is also UNEXPORTED and the
  `Git` interface (P1.M1.T2/T3, COMPLETE) has no config-reading method; adding one would modify
  completed work. The contract explicitly permits "call git config directly" — S3 is self-contained in
  `internal/config` with a small exec helper that mirrors `internal/git.run()`'s proven pattern
  (LookPath → `git -C <repo>` flag → separate stdout/stderr buffers → `errors.As(*exec.ExitError)`).
  stdlib only: `os/exec`, `context`, `fmt`, `strconv`, `strings`, `time`, `bytes`, `errors`.

  ⚠️ **THE third design call — per-key `git config --get`, exit-code-driven, missing-is-not-error.**
  Contract mandates per-key reads (NOT `--list`/`--get-regexp`); this also lets booleans use `--bool`
  for canonical normalization (FINDING C). Exit codes (FINDING B): **0 = found** (stdout has value);
  **1 = missing key → NOT an error** (return "not found", leave the field zero); any other exit →
  wrapped error. NOTE: reading config outside a repo exits **1** (not 128) because `git config --get`
  reads system+global even with no local repo — so loadGitConfig never errors for "non-repo"; it just
  finds nothing locally (global still applies — PRD §16.3 "composes with --local vs --global").

  ⚠️ **THE fourth design call — `timeout` is a bare integer SECONDS, not a duration string.** §16.3
  `timeout = 90` → `git config --get` returns `"90"`; parse `strconv.Atoi` → `time.Duration(n) *
  time.Second`. A `"90s"` value FAILS Atoi and is surfaced as a wrapped error (fail at load). This
  DIFFERS from the §16.2 TOML file (`timeout = "90s"`, parsed by S2's `time.ParseDuration`) — by
  design: git-config = integer seconds, TOML = Go duration string. Do NOT call `time.ParseDuration`
  here (it would accept "90s" and reject "90", the opposite of what §16.3 shows).

  ⚠️ **THE fifth design call — scalar keys only; NO provider manifests from git config.** §16.3 and
  FR36 list only scalar keys. `stagecoach.provider` here is the DEFAULT provider **name** (string →
  `Config.Provider`), NOT a manifest section. `Config.Providers` (the raw map S2 added) is NOT
  populated by S3 — provider manifests come ONLY from the TOML file (S2). Keeps S3 simple and avoids
  the messy `stagecoach.provider.X.Y` parsing that arch §2.5 sketches but the contract excludes.

  ⚠️ **THE sixth design call — non-zero overlay limitation is inherited (bool `false` can't be
  forced).** Because `Config` is plain-typed (S1 froze it) and S2's `overlay` copies only NON-ZERO
  fields, a git-config `autoStageAll = false` is read (`--bool` → "false", exit 0) and S3 sets
  `Config.AutoStageAll = false`, but that zero value is indistinguishable from "not set" and overlay
  does NOT apply it. Same documented v1 limitation as the TOML layers (S2). Escape hatches: env vars
  (S4, presence-checked) and CLI flags (S4, `flag.Changed`-checked). S3 documents this; it does NOT
  retype `Config` to pointers (would break S1 + all consumers). Consistent with S2.

  ⚠️ **THE seventh design call — no `context.Context` in the signature (contract-faithful).** Contract:
  `loadGitConfig(repoDir) (*Config, error)` — matches S2's `loadTOML(path)` (no context). S3 uses
  `context.Background()` internally for the exec. Config loading is fast (≈11 near-instant git calls)
  and runs early in cobra's `PersistentPreRunE`; cancellation is a non-issue. A context-aware variant
  is a trivial future refinement (add `loadGitConfigCtx(ctx, repoDir)`).

  Deliverable: `internal/config/git.go` (`loadGitConfig` + `gitConfigGet`/`gitConfigBool`/`parseInt`
  helpers, all unexported) and `internal/config/git_test.go` (white-box, REAL git in temp repos;
  replicated `initRepo`/`setGitConfig` helpers since `_test.go` helpers aren't cross-package). NO
  change to `config.go` (S1 frozen + S2's `Providers` field), `file.go` (S2), `go.mod`/`go.sum`.
  Touches ONLY `internal/config/`. INPUT = S1's `Config`+`Defaults()`. Feeds S4 (`Load()` orchestrator:
  `overlay(&cfg, loadGitConfig(repoDir))`).
---

## Goal

**Feature Goal**: Give Stagecoach the ability to read its per-repo **git config** layer (PRD §16.3,
FR36, layer 4 of §16.1) — the `stagecoach.*` keys a user sets via `git config stagecoach.provider pi`
etc. — and turn them into a partial `*Config` carrying ONLY the keys that were actually set, so the
precedence overlay (S2's `overlay`) can apply them with "higher layer wins" (FR34). Missing keys are
the normal "no override" condition and never an error.

**Deliverable**:
1. **CREATE** `internal/config/git.go` (`package config`) — the functions:
   (a) `func loadGitConfig(repoDir string) (*Config, error)` — for each known scalar key, read it via
       `git -C <repoDir> config --get stagecoach.<key>` (booleans with `--bool`); populate a fresh
       `*Config` with ONLY the keys found (exit 0); ignore missing keys (exit 1); return a wrapped
       error on any other failure (bad value, missing git binary). Returns a non-nil `*Config` on
       success (possibly all-zero if nothing was set).
   (b) unexported `func gitConfigGet(repo, key string) (value string, found bool, err error)` — runs
       `git -C <repo> config --get <fullKey>`; `found=true` on exit 0, `found=false` on exit 1,
       wrapped error otherwise.
   (c) unexported `func gitConfigBool(repo, key string) (value bool, found bool, err error)` — runs
       `git -C <repo> config --bool --get <fullKey>`; same exit-code semantics; `strconv.ParseBool`
       on the canonical `true`/`false` output.
   (d) unexported `func gitExec(repo string, args ...string) (stdout string, exitCode int, err error)`
       — the self-contained exec helper (LookPath → `-C <repo>` → separate buffers → ExitError). The
       SOLE place `internal/config` shells out to git.
2. **CREATE** `internal/config/git_test.go` (`package config`, white-box) — `TestLoadGitConfig_ReadsValues`
   (the contract's main case: set keys, assert they read back), `TestLoadGitConfig_MissingKeysIgnored`
   (the contract's "ignores missing" case: empty repo → zero Config), `TestLoadGitConfig_BoolNormalization`,
   `TestLoadGitConfig_BadTimeout`, `TestLoadGitConfig_GitBinaryMissing`, `TestGitConfigGet_FoundMissing`,
   `TestLoadGitConfig_CamelCaseKeysOnly` (locks FINDING A: underscore key is rejected), and
   `TestLoadGitConfig_OverlaysWithDefaults` (proves the S3→S2-overlay composition). Uses REAL git in
   `t.TempDir()` repos (mirrors `internal/git/*_test.go` style); `t.Setenv("HOME", t.TempDir())` for
   global-config isolation.

No `Load()` orchestrator (S4). No env/CLI layers (S4). No TOML/file code (S2). No provider manifest
type (P1.M2.T1). No `Config` struct change. No `go.mod`/`go.sum` change (stdlib `os/exec` only).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` clean;
`go test -race ./internal/config/` passes (S1's + S2's tests stay GREEN); `go test -race ./...` green;
`loadGitConfig` reads camelCase `stagecoach.*` keys (provider/model/timeout/booleans/ints/strings),
ignores missing keys, parses `timeout` as integer seconds → `time.Duration`, normalizes booleans via
`--bool`, returns a zero `*Config` (no error) when nothing is set, and a wrapped error on a bad value
or missing git binary; `git diff --exit-code go.mod go.sum` is empty.

## User Persona

**Target User**: Downstream Stagecoach subtask **S4** (`Load()` orchestrator, which calls
`overlay(&cfg, loadGitConfig(repoDir))` as layer 4 of §16.1). Transitively US8 (configuration &
precedence, FR34/FR36) and every user who configures Stagecoach via `git config stagecoach.*`.

**Use Case**: A user sets per-repo or global git config keys (`git config stagecoach.provider pi`,
`git config stagecoach.autoStageAll true`, `git config --global stagecoach.timeout 90`). S3 turns the
set keys into a partial `*Config`; S4's `overlay` applies them above the file layers (§16.1).

**User Journey**: (internal API, no end-user surface yet) S4: `cfg := Defaults()` →
`overlay(&cfg, loadTOML(globalConfigPath()))` → `overlay(&cfg, loadRepoLocalConfig())` →
**`overlay(&cfg, loadGitConfig(repoDir))`** → (S4 env) → (S4 CLI). Git config wins over both file
layers (FR34).

**Pain Points Addressed**: Removes "which git-config key names are valid", "how are booleans/timeout
parsed from git config", "is a missing key an error", "where does this reader live to avoid an import
cycle", and "how does the git layer compose with the TOML overlay" ambiguity for S4.

## Why

- **Git config is layer 4 (FR34).** Landing `loadGitConfig` now lets S4's `Load()` stay a thin,
  readable orchestrator and completes the "lower-than-env/CLI" portion of the precedence chain.
- **Locks the §16.3 → `Config` decode for the git layer.** The git layer, unlike the TOML layer (S2),
  produces a `*Config` DIRECTLY (no intermediate decode struct needed — git values are already
  scalars); S3's job is type coercion (`Atoi`, `--bool`, seconds→`Duration`) and the found/missing
  distinction via exit codes.
- **Corrects the FR36 naming before it ships broken.** FR36's snake_case `stagecoach.auto_stage_all`
  is rejected by git; S3 ships the working camelCase form (§16.3) and locks it with a regression test.
- **No user-facing surface change** (PRD "DOCS: none") — public git-config documentation ships with
  `config init` (P1.M4.T1.S4) and the README (P1.M5.T4.S1).

## What

A compiled `internal/config` package that can read the 11 known `stagecoach.*` scalar keys from a
repo's git config (local + global + system merged by git itself), coerce each to the right Go type
(string / `time.Duration` from integer seconds / `bool` via `--bool` / `int` via `Atoi`), and yield a
partial `*Config` (only-set-fields non-zero) suitable for S2's non-zero `overlay`. Missing keys are
not errors. No orchestrator, no other layers, no `Config` change, no provider manifests.

### Success Criteria

- [ ] `internal/config/git.go` exists, `package config`, imports only stdlib (`bytes`, `context`,
      `errors`, `fmt`, `os/exec`, `strconv`, `strings`, `time`). No third-party imports.
- [ ] `loadGitConfig(repoDir)` returns a non-nil `*Config` on success (all-zero if nothing set, no
      error); reads camelCase keys (`stagecoach.provider`, `.model`, `.timeout`, `.autoStageAll`,
      `.verbose`, `.maxDiffBytes`, `.maxMdLines`, `.maxDuplicateRetries`, `.subjectTargetChars`,
      `.output`, `.stripCodeFence`); booleans via `--bool`; `timeout` via `strconv.Atoi` → seconds.
- [ ] A missing key (git exit 1) leaves that field at its zero value and is NOT an error.
- [ ] A non-integer `timeout`, a missing git binary (`exec.LookPath` miss), or a git exit other than
      0/1 produces a wrapped error (with `%w` where appropriate).
- [ ] `git_test.go` has the required tests (all passing); uses REAL git in `t.TempDir()` repos and
      `t.Setenv` for PATH/HOME manipulation.
- [ ] `Config`, `Defaults()`, `file.go` (S2) are UNCHANGED by S3; `go.mod`/`go.sum` UNCHANGED
      (`git diff --exit-code go.mod go.sum` empty).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the S1 `Config`
shape (quoted below), the §16.3 key example, the empirically-verified git behaviors (research note),
and the function specs. No provider/TOML/generation-pipeline knowledge required — S3 is pure git-read
+ type coercion + a partial-`*Config` builder.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M1T4S3/research/git_config_behavior.md
  why: the EMPIRICAL basis for every design call. FINDING A (camelCase, NOT snake_case) is the single
       most important fact — it overrides the contract's literal "auto_stage_all" wording. FINDINGS
       B–G cover exit codes, --bool, integer timeout, import-cycle placement, and the overlay limit.
  critical: underscores are REJECTED by `git config` ("invalid key"). Multi-word keys MUST be
       camelCase (autoStageAll, maxDiffBytes, …) per the §16.3 example.

- file: internal/config/config.go
  why: the INPUT contract — the EXACT `Config` struct S3 builds a partial of (12 fields + the
       `Providers` field S2 adds). S3 does NOT modify it.
  pattern: field names + types (`Provider string`, `Timeout time.Duration`, `AutoStageAll bool`,
       `MaxDiffBytes int`, …). Map each git key to the field with the matching type.
  gotcha: `NoColor` is `toml:"-"` (CLI/UI only) — do NOT read it from git config. `Providers` is
       populated only by the TOML loader (S2) — do NOT read provider manifests from git config.

- file: plan/001_f1f80943ac34/P1M1T4S2/PRP.md
  section: "Implementation Blueprint" → `overlay(dst, src *Config)`
  why: S3's OUTPUT contract — the partial `*Config` must compose with S2's NON-ZERO overlay. Only
       non-zero fields are copied; this is why a found-but-zero bool (`autoStageAll=false`) is a
       documented no-op (FINDING G / S2 design call #3). S3 must leave unset fields at their zero
       value (do NOT pre-fill with Defaults() — that would make every field "set").
  pattern: `overlay` signature + the §16.1 layer-4 ordering S4 will use.

- file: internal/git/git.go
  section: `func (g *gitRunner) run(...)` (the exec pattern to MIRROR, not import)
  why: the canonical LookPath → `-C <repo>` → separate stdout/stderr buffers →
       `errors.As(*exec.ExitError)` pattern. S3's `gitExec` mirrors this EXACTLY but is self-contained
       in `internal/config` (run() is unexported; a different package cannot call it).
  pattern: `exec.LookPath("git")`; build args as `[]string{"-C", repo, ...}`; `var out, errb
       bytes.Buffer`; `cmd.Stdout=&out; cmd.Stderr=&errb`; on runErr, `errors.As(&exitErr)` to get
       `exitErr.ExitCode()`, keep `err==nil` for git exit codes.
  gotcha: do NOT set `cmd.Env` — inherit the parent env (git reads system+global config from it; the
       repo is targeted via `-C`, not cwd). `cmd.Dir` is NOT set (the `-C` flag is the repo selector).

- file: internal/git/git_test.go
  section: `func initRepo(t *testing.T, dir string)`
  why: the test-helper pattern S3's tests MUST replicate (a `_test.go` helper is not importable across
       packages). `git -C <dir> init` with a minimal git identity env.
  pattern: `t.Helper()`; `exec.Command("git","-C",dir,"init")`; `cmd.Env = append(os.Environ(),
       "GIT_AUTHOR_NAME=…", "GIT_AUTHOR_EMAIL=…", …)`; `cmd.CombinedOutput()` + `t.Fatalf` on error.
  gotcha: S3 tests additionally `t.Setenv("HOME", t.TempDir())` to isolate global git config so a
       developer's personal `~/.gitconfig` stagecoach.* keys can't leak into the test.

- url: https://git-scm.com/docs/git-config#_configuration_file
  why: confirms (a) key grammar = alphanumeric + `-` only in section/name (hence FINDING A — no `_`),
       (b) `--bool` canonicalizes to `true`/`false`, (c) `--get` exits 1 for a missing key, (d) plain
       `git config` reads the merged system+global+local scope (no `--local` needed).
  critical: the "invalid key" rejection of underscores is a hard grammar rule, not a quirk. The §16.3
       camelCase example is the spec-compliant form.

- file: PRD.md
  section: "16.3 Git-config keys" (h3.59) + "16.1 Resolution order" (h3.57) + "9.8" (h3.24, FR34/FR36)
  why: §16.3 is the authoritative key-shape example (camelCase) and the scalar key set; §16.1 fixes
       git config as layer 4 (above the two TOML files, below env/CLI); FR34 is the "higher wins"
       invariant S3's partial `*Config` serves; FR36 names the keys (but its snake_case spelling is
       corrected to camelCase per FINDING A — the §16.3 example wins).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2
go.sum
internal/
  config/
    config.go                   # S1: Config (12 fields) + Defaults()   ; S2 adds Providers `toml:"-"`
    config_test.go              # S1: TestDefaults + TestTOMLMarshalKeys...   ← UNCHANGED (stay green)
    file.go                     # S2: loadTOML/overlay/globalConfigPath/loadRepoLocalConfig + fileConfig
    file_test.go                # S2: TestLoadTOML*/TestOverlay*/TestGlobalConfigPath/...
    git.go                      # NEW (S3) ← loadGitConfig + gitConfigGet/gitConfigBool/gitExec
    git_test.go                 # NEW (S3) ← TestLoadGitConfig_*/TestGitConfigGet_*
  git/                          # T2/T3 (Git interface + gitRunner.run) — untouched by S3
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  config/
    git.go                      # NEW — loadGitConfig + gitConfigGet + gitConfigBool + gitExec (unexported)
    git_test.go                 # NEW — white-box tests, REAL git in t.TempDir() repos
# config.go / config_test.go / file.go / file_test.go UNCHANGED
# go.mod / go.sum UNCHANGED (stdlib os/exec only — no new dependency)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: git config keys CANNOT contain underscores (FINDING A). `git config stagecoach.auto_stage_all`
// => "error: invalid key". Git's key grammar: section/name = alphanumeric + '-' ONLY. Multi-word keys
// MUST be camelCase per PRD §16.3 (autoStageAll, maxDiffBytes, maxMdLines, maxDuplicateRetries,
// subjectTargetChars, stripCodeFence). Single-word keys (provider, model, timeout, verbose, output)
// are valid as-is. Do NOT use FR36's snake_case spelling — it is empirically unusable.

// CRITICAL: `git config --get` exit codes (FINDING B). exit 0 = found; exit 1 = MISSING KEY (NOT an
// error — the core contract point "missing keys are not errors"); any other exit (2 usage, etc.) =
// wrapped error. Reading OUTSIDE a repo exits 1 (not 128) — git reads system+global config even with
// no local repo, so loadGitConfig never errors for "non-repo"; it just finds nothing local.

// CRITICAL: booleans MUST be read with `--bool` (FINDING C). `git config --bool --get stagecoach.X`
// returns canonical "true"/"false" for inputs on/off/yes/no/1/0. Plain `--get` would return the raw
// "on"/"1" and need manual parsing. strconv.ParseBool on the `--bool` output never fails.

// CRITICAL: `timeout` is a bare INTEGER (seconds), parsed via strconv.Atoi -> time.Duration(n)*time.Second
// (FINDING D). §16.3 `timeout = 90` -> "90". Do NOT use time.ParseDuration (it would accept "90s" and
// reject "90" — the opposite of §16.3). A non-integer -> wrapped error (fail at load). This differs
// from the TOML layer (S2 uses time.ParseDuration on "90s") BY DESIGN.

// CRITICAL: plain `git config --get` reads local+global+system MERGED (FINDING E). Do NOT add --local
// (contract: plain `git config --get`). PRD §16.3 intends this ("composes with --local vs --global"):
// a --global stagecoach.* key applies everywhere, a --local one per-repo.

// CRITICAL: placement is internal/config/git.go with a SELF-CONTAINED os/exec helper (FINDING F). Do
// NOT import internal/git (its run() is unexported; its Git interface has no config method; importing
// it from config would also risk a cycle once S4's Load() calls the reader). Mirror run()'s pattern
// locally. Do NOT modify the internal/git Git interface (P1.M1.T2/T3 are COMPLETE).

// CRITICAL (inherited limitation, FINDING G): overlay is NON-ZERO (S2). A found-but-false bool
// (autoStageAll=false) sets Config.AutoStageAll=false (zero), which overlay does NOT apply. Documented
// v1 limitation; escape hatch = env (S4) / CLI (S4). Do NOT retype Config to pointers (breaks S1).

// GOTCHA: do NOT pre-fill the returned *Config with Defaults(). Every field must start at its zero
// value; only FOUND keys are set non-zero. (Defaults() is Layer 1; S4 calls it — S3 returns a PARTIAL
// overlay, not a fully-defaulted Config. If S3 pre-filled defaults, overlay would copy ALL of them,
// clobbering lower layers incorrectly. The zero-value start is what makes "only set fields" work.)

// GOTCHA: NO provider manifests from git config (design call #5). stagecoach.provider = default provider
// NAME (string -> Config.Provider). Config.Providers is populated ONLY by the TOML loader (S2).

// GOTCHA: NoColor is NOT read (CLI/UI only, toml:"-"). Verbose IS read (it's a [defaults] field).

// GOTCHA: errors.As(*exec.ExitError) is how you get the exit code from a non-zero run. cmd.Run()
// returns nil ONLY on exit 0; any non-zero exit is a non-nil error wrapping *exec.ExitError. Context
// cancellation surfaces as a different error (context.Canceled) — but S3 uses context.Background(), so
// this path is dormant; still, do NOT conflate a cancel error with a git exit code.

// GOTCHA (test): _test.go helpers are NOT importable across packages. internal/config/git_test.go must
// define its OWN initRepo/setGitConfig helpers (copy the bodies from internal/git/git_test.go). Use
// t.Setenv("HOME", t.TempDir()) so a developer's global ~/.gitconfig stagecoach.* keys don't leak in.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — UNCHANGED by S3. Quoted here only so the implementer knows the exact
// field set/types to map git keys onto. S2 adds `Providers map[string]map[string]any \`toml:"-"\``;
// S3 does NOT touch it. (See the real file; do not retype.)
package config

type Config struct {
	Provider            string        // <- stagecoach.provider   (string, --get)
	Model               string        // <- stagecoach.model      (string, --get)
	Timeout             time.Duration // <- stagecoach.timeout    (int seconds -> Atoi -> *time.Second)
	AutoStageAll        bool          // <- stagecoach.autoStageAll   (--bool)
	Verbose             bool          // <- stagecoach.verbose        (--bool)
	NoColor             bool          //  NOT read from git config (CLI/UI only, toml:"-")
	MaxDiffBytes        int           // <- stagecoach.maxDiffBytes   (int, --get -> Atoi)
	MaxMdLines          int           // <- stagecoach.maxMdLines     (int, --get -> Atoi)
	MaxDuplicateRetries int           // <- stagecoach.maxDuplicateRetries (int, --get -> Atoi)
	SubjectTargetChars  int           // <- stagecoach.subjectTargetChars  (int, --get -> Atoi)
	Output              string        // <- stagecoach.output         (string, --get)
	StripCodeFence      bool          // <- stagecoach.stripCodeFence (--bool)
	Providers           map[string]map[string]any // NOT read from git config (S2's TOML domain)
}
```

```go
// internal/config/git.go — NEW. package config.
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gitExec is the SOLE place internal/config shells out to git. It mirrors internal/git.run()'s proven
// pattern but is self-contained here (internal/git.run is unexported and unreachable from this
// package; importing internal/git would also risk a cycle once S4's Load() calls loadGitConfig).
//
//   resolves the git binary via exec.LookPath (PRD §19: real binary)
//   targets the repo via the -C flag (NOT cmd.Dir — goroutine-safe, matches internal/git)
//   captures stdout and stderr to SEPARATE buffers
//   returns (stdout, exitCode, err): a non-zero git exit yields (stdout, stderr-text-as-stderr-inside-
//   stdout? no — see signature) exitCode with err==nil (git uses exit codes as signals). err != nil
//   ONLY for infrastructural failure (LookPath miss, start/I/O), with exitCode == -1.
//
// INVARIANT (copied from internal/git.run): NON-ZERO git exit -> err == nil, exitCode = the code.
func gitExec(repo string, args ...string) (stdout string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir
	full = append(full, args...)

	cmd := exec.CommandContext(context.Background(), gitPath, full...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb // captured separately so callers can build precise error messages

	runErr := cmd.Run()
	if runErr == nil {
		return out.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit -> capture code, err stays nil
		return out.String(), exitErr.ExitCode(), nil
	}
	return out.String(), -1, runErr // start / I/O failure (e.g. context cancel — dormant for Background)
}

// gitConfigGet runs `git -C <repo> config --get <key>` and returns the value iff the key is present.
// Exit-code semantics (FINDING B): 0 -> found (trimmed value); 1 -> missing (found=false, NOT an
// error); anything else -> wrapped error (incl. stderr text). Used for STRING and INT keys.
func gitConfigGet(repo, key string) (value string, found bool, err error) {
	stdout, stderr, code, err := gitExec(repo, "config", "--get", key)
	if err != nil {
		return "", false, err // LookPath miss / start-I/O failure (exitCode == -1)
	}
	switch code {
	case 0:
		return strings.TrimSpace(stdout), true, nil // found
	case 1:
		return "", false, nil // missing key — NOT an error (FINDING B)
	default:
		return "", false, fmt.Errorf("git config --get %s: failed (exit %d): %s", key, code, strings.TrimSpace(stderr))
	}
}

// gitConfigBool runs `git -C <repo> config --bool --get <key>` and returns the parsed bool iff present.
// `--bool` canonicalizes any git-boolean (on/off/yes/no/1/0/true/false) to "true"/"false" (FINDING C),
// so strconv.ParseBool never fails on the output. Same exit-code semantics as gitConfigGet.
func gitConfigBool(repo, key string) (value bool, found bool, err error) {
	stdout, stderr, code, err := gitExec(repo, "config", "--bool", "--get", key)
	if err != nil {
		return false, false, err
	}
	switch code {
	case 0:
		b, perr := strconv.ParseBool(strings.TrimSpace(stdout)) // "true"/"false" — never fails in practice
		if perr != nil {
			return false, false, fmt.Errorf("git config --bool --get %s: unparseable output %q: %w", key, stdout, perr)
		}
		return b, true, nil
	case 1:
		return false, false, nil // missing key — NOT an error
	default:
		return false, false, fmt.Errorf("git config --bool --get %s: failed (exit %d): %s", key, code, strings.TrimSpace(stderr))
	}
}

// loadGitConfig reads Stagecoach's per-repo git-config layer (PRD §16.3, FR36, §16.1 layer 4) from the
// repo at repoDir and returns a PARTIAL *Config carrying ONLY the keys that were found set (all others
// remain at their zero value). Missing keys are NOT errors (git config --get exits 1 for a missing
// key, FINDING B). A non-integer timeout, a missing git binary, or any unexpected git exit yields a
// wrapped error.
//
// KEY NAMES ARE CAMELCASE (FINDING A): git config rejects underscores ("invalid key"). The multi-word
// keys follow the PRD §16.3 example (autoStageAll, maxDiffBytes, …), NOT FR36's snake_case spelling.
//
// The returned *Config is designed for S2's NON-ZERO overlay(): unset fields are zero, so overlay
// copies only the fields the user actually set. Do NOT pre-fill with Defaults() (that would make every
// field "set" and clobber lower layers — see the GOTCHA). Because overlay is non-zero, a found-but-
// false bool (autoStageAll=false) is a documented no-op (FINDING G); force false via env (S4)/CLI (S4).
func loadGitConfig(repoDir string) (*Config, error) {
	c := &Config{} // ALL fields zero; only found keys are set below.

	// --- strings (plain --get) ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.provider"); err != nil {
		return nil, err
	} else if found {
		c.Provider = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.model"); err != nil {
		return nil, err
	} else if found {
		c.Model = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.output"); err != nil {
		return nil, err
	} else if found {
		c.Output = v
	}

	// --- timeout: integer SECONDS -> time.Duration (FINDING D). NOT time.ParseDuration. ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
		return nil, err
	} else if found {
		n, perr := strconv.Atoi(v) // §16.3 "90" -> 90; "90s" FAILS (surfaced as a wrapped load error)
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.timeout: invalid integer %q: %w", v, perr)
		}
		c.Timeout = time.Duration(n) * time.Second
	}

	// --- booleans (--bool canonicalizes; FINDING C) ---
	if v, found, err := gitConfigBool(repoDir, "stagecoach.autoStageAll"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.AutoStageAll = v
	}
	if v, found, err := gitConfigBool(repoDir, "stagecoach.verbose"); err != nil {
		return nil, err
	} else if found {
		c.Verbose = v
	}
	if v, found, err := gitConfigBool(repoDir, "stagecoach.stripCodeFence"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.StripCodeFence = v
	}

	// --- ints (plain --get -> Atoi) ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.maxDiffBytes: invalid integer %q: %w", v, perr)
		}
		c.MaxDiffBytes = n
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxMdLines"); err != nil { // camelCase!
		return nil, err
	} else if found {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.maxMdLines: invalid integer %q: %w", v, perr)
		}
		c.MaxMdLines = n
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDuplicateRetries"); err != nil { // camelCase!
		return nil, err
	} else if found {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.maxDuplicateRetries: invalid integer %q: %w", v, perr)
		}
		c.MaxDuplicateRetries = n
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.subjectTargetChars"); err != nil { // camelCase!
		return nil, err
	} else if found {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.subjectTargetChars: invalid integer %q: %w", v, perr)
		}
		c.SubjectTargetChars = n
	}

	return c, nil // non-nil; all-zero if nothing was set (overlay is then a no-op)
}
```

> **DRY alternative (implementer's choice):** the four `strconv.Atoi` blocks above are repetitive. If
> preferred, factor a tiny `applyInt(repo, key string, dst *int) error` helper and a
> `applyString`/`applyBool` pair, then call them in `loadGitConfig`. The expanded form is shown for
> clarity; either is acceptable as long as the exit-code/zero-value semantics are identical. Keep the
> key list to EXACTLY the 11 camelCase keys above (no `noColor`, no provider manifests).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/git.go — the exec helper (gitExec)
  - IMPLEMENT gitExec(repo string, args ...string) (stdout string, exitCode int, err error) per the
    Data Models block: exec.LookPath("git"); args = ["-C", repo, ...]; SEPARATE stdout/stderr buffers;
    on runErr, errors.As(*exec.ExitError) -> return (stdout, exitErr.ExitCode(), nil); else
    (stdout, -1, runErr). Do NOT set cmd.Env (inherit parent) or cmd.Dir (the -C flag selects repo).
  - IMPORTS: bytes, context, errors, fmt, os/exec. (strconv/strings/time come with Task 3.)
  - WHY FIRST: gitConfigGet/gitConfigBool/loadGitConfig all call it.

Task 2: ADD gitConfigGet + gitConfigBool to internal/config/git.go
  - IMPLEMENT gitConfigGet(repo, key): gitExec(repo, "config", "--get", key); switch on code:
    0 -> (TrimSpace(stdout), true, nil); 1 -> ("", false, nil); default -> wrapped error w/ stderr.
  - IMPLEMENT gitConfigBool(repo, key): gitExec(repo, "config", "--bool", "--get", key); same switch;
    on code 0, strconv.ParseBool(TrimSpace(stdout)) (guard with a wrapped error, though it can't fail
    on canonical --bool output).
  - GOTCHA: exit 1 is "missing key" (NOT an error) for BOTH (FINDING B). Do NOT collapse 1 into the
    default error branch.

Task 3: ADD loadGitConfig to internal/config/git.go
  - IMPLEMENT per the Data Models block: c := &Config{} (ALL zero — do NOT call Defaults()); read the
    11 camelCase keys (provider/model/output strings; timeout int-seconds -> Duration; autoStageAll/
    verbose/stripCodeFence bools via --bool; maxDiffBytes/maxMdLines/maxDuplicateRetries/
    subjectTargetChars ints via Atoi). On any helper error, return (nil, err) immediately. On found,
    set the field; on not-found, leave it zero.
  - KEY LIST (EXACT — camelCase, FINDING A): stagecoach.provider, stagecoach.model, stagecoach.timeout,
    stagecoach.autoStageAll, stagecoach.verbose, stagecoach.maxDiffBytes, stagecoach.maxMdLines,
    stagecoach.maxDuplicateRetries, stagecoach.subjectTargetChars, stagecoach.output,
    stagecoach.stripCodeFence. (NO noColor; NO provider manifests.)
  - GOTCHA: timeout uses strconv.Atoi + time.Duration(n)*time.Second — NOT time.ParseDuration (FINDING D).
  - GOTCHA: a found-but-false bool sets the zero value (overlay won't apply it — FINDING G, documented).
  - RETURN: the non-nil *Config (possibly all-zero).

Task 4: CREATE internal/config/git_test.go — helpers + the core cases
  - PACKAGE: `package config` (white-box). Imports: context (if needed), os, os/exec, strings, testing,
    time. (No bytes/io needed unless you buffer stderr in a test.)
  - HELPERS (copy from internal/git/git_test.go — _test.go helpers aren't cross-package):
      initRepo(t, dir): `git -C dir init` w/ minimal GIT_AUTHOR_*/GIT_COMMITTER_* env; t.Helper();
      t.Fatalf on error.
      setGitConfig(t, dir, key, value): `git -C dir config <key> <value>` (writes repo-local .git/config
      by default); t.Helper(); t.Fatalf on error.
  - ISOLATION: at the top of each repo-based test, `t.Setenv("HOME", t.TempDir())` so a developer's
    global ~/.gitconfig stagecoach.* keys can't leak into the read (plain `git config --get` merges
    global+local, FINDING E). (initRepo's env already pins identity; HOME isolation is the extra step.)
  - TEST A TestLoadGitConfig_ReadsValues (CONTRACT main case): initRepo(repo); setGitConfig for a
    representative mix — provider=pi, model=glm-5.2, timeout=90, autoStageAll=true (use "on"),
    verbose=true (use "yes"), maxDiffBytes=12345, maxMdLines=80, maxDuplicateRetries=5,
    subjectTargetChars=60, output=json, stripCodeFence=true (use "1"). loadGitConfig(repo) -> assert
    cfg.Provider=="pi", cfg.Model=="glm-5.2", cfg.Timeout==90*time.Second, cfg.AutoStageAll==true,
    cfg.Verbose==true, cfg.MaxDiffBytes==12345, cfg.MaxMdLines==80, cfg.MaxDuplicateRetries==5,
    cfg.SubjectTargetChars==60, cfg.Output=="json", cfg.StripCodeFence==true. (Proves --bool
    normalization of on/yes/1 AND int/timeout coercion in one shot.)
  - TEST B TestLoadGitConfig_MissingKeysIgnored (CONTRACT "ignores missing"): initRepo(repo); do NOT
    set any stagecoach.* key. loadGitConfig(repo) -> err==nil, cfg!=nil, and EVERY field is its zero
    value (Provider=="", Timeout==0, AutoStageAll==false, MaxDiffBytes==0, Output=="", etc.). Proves
    exit-1-is-not-error and the zero-value start.
  - TEST C TestLoadGitConfig_BoolNormalization: set autoStageAll=off and stripCodeFence=no; assert
    cfg.AutoStageAll==false && cfg.StripCodeFence==false (proves --bool parses falsy spellings).
    (NOTE per FINDING G: this only proves the READ; overlay would not apply false — covered in TEST H.)
  - TEST D TestLoadGitConfig_BadTimeout: set timeout=notanumber; loadGitConfig -> non-nil err whose
    message contains "stagecoach.timeout" and "invalid integer". (Proves fail-at-load.)
  - TEST E TestLoadGitConfig_GitBinaryMissing: t.Setenv("PATH",""); loadGitConfig(t.TempDir()) ->
    non-nil err containing "git binary not found". (LookPath miss path.)
  - TEST F TestGitConfigGet_FoundMissing: in a repo, setGitConfig(provider,pi);
    gitConfigGet(repo,"stagecoach.provider") -> (value=="pi", found==true, nil);
    gitConfigGet(repo,"stagecoach.does.not.exist") -> ("", false, nil). (Unit-tests the helper directly.)

Task 5: ADD the regression + composition tests to internal/config/git_test.go
  - TEST G TestLoadGitConfig_CamelCaseKeysOnly (LOCKS FINDING A): setGitConfig(repo,"stagecoach.autoStageAll","true");
    assert loadGitConfig reads cfg.AutoStageAll==true (camelCase works). THEN setGitConfig(repo,
    "stagecoach.max_diff_bytes","9") — expect exec to FAIL with "invalid key" — so use exec.Command
    directly in the test (or `setGitConfigMustFail`) to assert the WRITE is rejected, and assert
    loadGitConfig(repo).MaxDiffBytes==0 (the underscore key is unreadable). This locks the camelCase
    decision as a regression test against anyone "fixing" it to snake_case.
  - TEST H TestLoadGitConfig_OverlaysWithDefaults (S2 composition / contract "for the precedence
    overlay"): set provider=pi, timeout=45, maxMdLines=7 in the repo; cfg := Defaults();
    overlay(&cfg, loadGitConfig(repo)); assert cfg.Provider=="pi" (git overrode the "" default),
    cfg.Timeout==45*time.Second (overrode 120s), cfg.MaxMdLines==7 (overrode 100), AND the unset git
    fields keep their Defaults() values (cfg.AutoStageAll==true, cfg.MaxDiffBytes==300000,
    cfg.Output=="raw"). Proves the partial-*Config -> overlay contract end-to-end. (Requires S2's
    overlay, which exists when S3 runs — they ship together.)

Task 6: VERIFY (no file change)
  - RUN the full Validation Loop (Levels 1–3). `git diff --exit-code go.mod go.sum` MUST be empty
    (S3 adds no dependency — stdlib os/exec only). S1's + S2's tests MUST still pass (S3 is purely
    additive: two new files, no edits to existing files).
```

### Implementation Patterns & Key Details

```go
// gitExec — the invariant: NON-ZERO git exit -> err == nil, exitCode = the code. (Mirror internal/git.run.)
func gitExec(repo string, args ...string) (stdout string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	full := append([]string{"-C", repo}, args...) // repo via flag, not cmd.Dir
	cmd := exec.CommandContext(context.Background(), gitPath, full...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return out.String(), exitErr.ExitCode(), nil // git exit code captured; err stays nil
		}
		return out.String(), -1, runErr // infrastructural failure
	}
	return out.String(), 0, nil
}

// The three-branch switch in gitConfigGet/gitConfigBool is the heart of "missing is not an error":
//   case 0:  found   -> (value, true,  nil)
//   case 1:  missing -> ("",    false, nil)   // <-- NOT the default branch
//   default: error   -> ("",    false, wrappedErr)
// Collapsing case 1 into default is the #1 way to get this wrong. Keep them separate.

// loadGitConfig field-set idiom (repeat per key; factor into apply* helpers if you prefer DRY):
if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil {
	return nil, err
} else if found {
	n, perr := strconv.Atoi(v)
	if perr != nil {
		return nil, fmt.Errorf("git config stagecoach.maxDiffBytes: invalid integer %q: %w", v, perr)
	}
	c.MaxDiffBytes = n // ONLY set when found; zero otherwise (overlay copies non-zero only)
}
```

```go
// git_test.go — TestLoadGitConfig_ReadsValues (the contract main case): proves --bool normalization
// (on/yes/1 -> true), int coercion, and timeout-as-seconds in one assertion block.
func TestLoadGitConfig_ReadsValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global git config (FINDING E)
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.provider", "pi")
	setGitConfig(t, repo, "stagecoach.model", "glm-5.2")
	setGitConfig(t, repo, "stagecoach.timeout", "90")
	setGitConfig(t, repo, "stagecoach.autoStageAll", "on")   // --bool normalizes "on" -> true
	setGitConfig(t, repo, "stagecoach.verbose", "yes")       // "yes" -> true
	setGitConfig(t, repo, "stagecoach.maxDiffBytes", "12345")
	setGitConfig(t, repo, "stagecoach.stripCodeFence", "1")  // "1" -> true
	setGitConfig(t, repo, "stagecoach.output", "json")

	cfg, err := loadGitConfig(repo)
	if err != nil || cfg == nil {
		t.Fatalf("loadGitConfig: cfg=%v err=%v", cfg, err)
	}
	if cfg.Provider != "pi" { t.Errorf("Provider=%q want pi", cfg.Provider) }
	if cfg.Timeout != 90*time.Second { t.Errorf("Timeout=%v want 90s", cfg.Timeout) }
	if !cfg.AutoStageAll { t.Errorf("AutoStageAll=false want true (--bool 'on')") }
	if !cfg.Verbose { t.Errorf("Verbose=false want true (--bool 'yes')") }
	if cfg.MaxDiffBytes != 12345 { t.Errorf("MaxDiffBytes=%d want 12345", cfg.MaxDiffBytes) }
	if !cfg.StripCodeFence { t.Errorf("StripCodeFence=false want true (--bool '1')") }
	if cfg.Output != "json" { t.Errorf("Output=%q want json", cfg.Output) }
}

// git_test.go — TestLoadGitConfig_MissingKeysIgnored (the contract "ignores missing"): empty repo.
func TestLoadGitConfig_MissingKeysIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo) // no stagecoach.* keys set
	cfg, err := loadGitConfig(repo)
	if err != nil { t.Fatalf("loadGitConfig err=%v, want nil (missing keys are not errors)", err) }
	if cfg == nil { t.Fatal("cfg=nil, want non-nil") }
	// EVERY field must be its zero value (nothing was set):
	if cfg.Provider != "" || cfg.Model != "" || cfg.Output != "" {
		t.Errorf("string field non-zero: %+v", cfg)
	}
	if cfg.Timeout != 0 || cfg.MaxDiffBytes != 0 || cfg.MaxMdLines != 0 ||
		cfg.MaxDuplicateRetries != 0 || cfg.SubjectTargetChars != 0 {
		t.Errorf("numeric field non-zero: %+v", cfg)
	}
	if cfg.AutoStageAll || cfg.Verbose || cfg.StripCodeFence {
		t.Errorf("bool field non-zero: %+v", cfg)
	}
}

// git_test.go — TestLoadGitConfig_OverlaysWithDefaults (the S3->S2-overlay composition contract):
func TestLoadGitConfig_OverlaysWithDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.provider", "pi")
	setGitConfig(t, repo, "stagecoach.timeout", "45")
	setGitConfig(t, repo, "stagecoach.maxMdLines", "7")

	cfg := Defaults()
	gc, err := loadGitConfig(repo)
	if err != nil { t.Fatalf("loadGitConfig err=%v", err) }
	overlay(&cfg, gc) // S2's overlay (exists when S3 ships)
	if cfg.Provider != "pi" { t.Errorf("Provider=%q want pi (git overrode default)", cfg.Provider) }
	if cfg.Timeout != 45*time.Second { t.Errorf("Timeout=%v want 45s", cfg.Timeout) }
	if cfg.MaxMdLines != 7 { t.Errorf("MaxMdLines=%d want 7", cfg.MaxMdLines) }
	// Unset git fields MUST keep Defaults() (proves partial overlay, not wholesale replace):
	if !cfg.AutoStageAll { t.Errorf("AutoStageAll=false want true (default preserved)") }
	if cfg.MaxDiffBytes != 300000 { t.Errorf("MaxDiffBytes=%d want 300000 (default preserved)", cfg.MaxDiffBytes) }
	if cfg.Output != "raw" { t.Errorf("Output=%q want raw (default preserved)", cfg.Output) }
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. S3 imports only stdlib (bytes, context, errors, fmt, os/exec, strconv, strings,
    time). `git diff --exit-code go.mod go.sum` empty.

CONFIG STRUCT (internal/config/config.go):
  - change: NONE. S3 only READS Config's fields; it does not add/remove/retype any. S2's Providers
    field is also untouched.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M1.T4.S4 (Load orchestrator): cfg := Defaults(); overlay(&cfg, loadTOML(globalConfigPath()));
        overlay(&cfg, loadRepoLocalConfig()); overlay(&cfg, loadGitConfig(repoDir)); (S4 env);
        (S4 CLI). S3 ships ONLY loadGitConfig (+ helpers); S4 does the sequencing + passes repoDir.
  - S2's overlay(dst, src *Config): NON-ZERO field-by-field copy — S3's partial *Config (zero unset
        fields) composes correctly. A found-but-false bool is a documented no-op (FINDING G).

NO DATABASE / NO ROUTES / NO FILE WRITING / NO CLI WIRING (config init is P1.M4.T1.S4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1–3 (git.go):
go build ./...                       # Whole module compiles incl. the new git.go. Expect exit 0.
gofmt -w internal/config/git.go internal/config/git_test.go
test -z "$(gofmt -l internal/config/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/            # (and `go vet ./...`) Expect zero diagnostics.
# Expected: all clean. gofmt aligns the gitExec/gitConfigGet signatures — let it.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests + all existing tests (white-box, REAL git required in PATH):
go test -race ./internal/config/ -v
# Expected: PASS — S1's TestDefaults/TestTOMLMarshalKeysAndNoColorExclusion, S2's TestLoadTOML*/
#   TestOverlay*/TestGlobalConfigPath/TestRepoProviderNotice/TestLoadRepoLocalConfig, AND S3's new
#   TestLoadGitConfig_*/TestGitConfigGet_*.
#   NOTE: these tests exec the REAL git binary in t.TempDir() repos — git must be on PATH (it is in
#   this dev/CI environment; TestLoadGitConfig_GitBinaryMissing proves the no-git path separately).

# Full suite must stay green (no regression in internal/git or elsewhere):
go test -race ./...
# Expected: all packages PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# No CLI/runtime wiring yet (no Load(), no command wiring). Validate build + deps + additive scope:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED by S3"   # MUST be empty.
# Confirm S3 is purely additive (no edits to existing files):
git diff --exit-code internal/config/config.go internal/config/config_test.go internal/config/file.go internal/config/file_test.go \
  && echo "config.go/file.go UNCHANGED by S3"   # MUST be empty.
grep -n 'func loadGitConfig' internal/config/git.go   # prints the definition line.
# Expected: binary builds; go.mod/go.sum unchanged; config.go/file.go untouched; loadGitConfig present.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Smoke the reader by hand against a real temp repo (sanity for the S4 author):
TMP=$(mktemp -d); git -C "$TMP" init -q
git -C "$TMP" config stagecoach.provider pi
git -C "$TMP" config stagecoach.timeout 90
git -C "$TMP" config stagecoach.autoStageAll true   # camelCase (underscore would be "invalid key")
# (Optional) a tiny throwaway Go snippet in /tmp that calls config.loadGitConfig("$TMP") and prints
# cfg.Provider / cfg.Timeout / cfg.AutoStageAll — OR rely on TestLoadGitConfig_ReadsValues which
# already asserts these end-to-end with REAL git.
go test ./internal/config/ -run 'TestLoadGitConfig_|TestGitConfigGet_' -v
# Inspect: provider==pi, timeout==90s, autoStageAll==true; missing keys ignored (zero fields).

# Confirm the underscore rejection empirically (documents FINDING A for future maintainers):
git -C "$TMP" config stagecoach.max_diff_bytes 9 2>&1 | grep -q "invalid key" && echo "underscore rejected (expected)"

# (Optional) lint:
golangci-lint run ./internal/config/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint is project-wide; run `make lint` in CI)."
rm -rf "$TMP"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`.
- [ ] Level 2 green: `go test -race ./internal/config/ -v` (S1's + S2's + S3's tests) AND `go test -race ./...`.
- [ ] Level 3: binary builds; `git diff --exit-code go.mod go.sum` empty; `config.go`/`file.go`/their
      tests byte-unchanged by S3; `loadGitConfig` present in `git.go`.

### Feature Validation

- [ ] `loadGitConfig` reads the 11 camelCase keys (`provider`, `model`, `timeout`, `autoStageAll`,
      `verbose`, `maxDiffBytes`, `maxMdLines`, `maxDuplicateRetries`, `subjectTargetChars`, `output`,
      `stripCodeFence`) with correct types (TestLoadGitConfig_ReadsValues).
- [ ] Missing keys leave their field at zero and are NOT errors; an empty repo yields an all-zero
      `*Config` (TestLoadGitConfig_MissingKeysIgnored).
- [ ] Booleans are normalized via `--bool` (`on`/`yes`/`1`/`off`/`no`/`0` → true/false)
      (TestLoadGitConfig_ReadsValues + TestLoadGitConfig_BoolNormalization).
- [ ] `timeout` parses as integer seconds → `time.Duration` (`90` → `90*time.Second`); a non-integer
      fails at load with a wrapped error (TestLoadGitConfig_BadTimeout).
- [ ] A missing git binary yields an error containing "git binary not found"
      (TestLoadGitConfig_GitBinaryMissing).
- [ ] The camelCase decision is locked: camelCase keys read; underscore keys are git-"invalid key"
      (TestLoadGitConfig_CamelCaseKeysOnly — regression test for FINDING A).
- [ ] The partial `*Config` composes with S2's `overlay`: git values override Defaults(), unset git
      fields preserve Defaults() (TestLoadGitConfig_OverlaysWithDefaults).

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package config`, stdlib `testing`, `t.Errorf` per-field,
      `t.Helper()` on helpers, REAL git in `t.TempDir()` (mirrors `internal/git/*_test.go`).
- [ ] Self-contained exec helper mirrors `internal/git.run()` (LookPath → `-C` flag → separate buffers
      → `errors.As(*exec.ExitError)`); does NOT import or modify `internal/git`.
- [ ] Only stdlib imports; no new third-party dependency.
- [ ] The "missing is not an error" exit-code branch (`case 1`) is kept separate from the error branch.
- [ ] No `Config` pre-fill with `Defaults()` (partial overlay integrity).

### Documentation & Deployment

- [ ] Code is self-documenting: doc comments cite FINDING A (camelCase), FINDING B (exit codes),
      FINDING D (integer timeout), FINDING G (overlay limit) so a maintainer can't accidentally
      "fix" camelCase back to snake_case or swap Atoi for ParseDuration.
- [ ] No new env vars / no CLI surface (internal; docs ship with `config init`, P1.M4.T1.S4).

---

## Anti-Patterns to Avoid

- ❌ Don't use snake_case git keys (`stagecoach.auto_stage_all`) — git rejects underscores ("invalid
  key"). Use the §16.3 camelCase form (`stagecoach.autoStageAll`). (FINDING A.)
- ❌ Don't read booleans with plain `--get` — use `--bool` for canonical `true`/`false`. (FINDING C.)
- ❌ Don't parse `timeout` with `time.ParseDuration` — git-config uses integer seconds; use
  `strconv.Atoi` → `time.Duration(n)*time.Second`. (FINDING D.)
- ❌ Don't collapse git exit 1 ("missing key") into the error branch — it is NOT an error. (FINDING B.)
- ❌ Don't import `internal/git` to reuse its `run()` — it's unexported and would create a cycle once
  S4 calls the reader. Use the self-contained `gitExec` helper. (FINDING F.)
- ❌ Don't pre-fill the returned `*Config` with `Defaults()` — it must be all-zero except found keys,
  or S2's overlay will clobber lower layers.
- ❌ Don't read `noColor` or provider manifests from git config (out of scope for this layer).
- ❌ Don't catch all errors silently — surface a bad value (non-integer) or missing git binary as a
  wrapped error so it fails at LOAD, not later.
