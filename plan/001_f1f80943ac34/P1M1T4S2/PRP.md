---
name: "P1.M1.T4.S2 — TOML file loading (global + repo-local) with go-toml/v2"
description: |
  The SECOND subtask of the Configuration System (P1.M1.T4): load Stagecoach's TOML config files and
  merge them. Implement `internal/config/file.go` with three contract functions plus a repo-local
  helper: `loadTOML(path) (*Config, error)` (decode a §16.2 file; `(nil, nil)` if missing),
  `globalConfigPath()` (`$XDG_CONFIG_HOME/stagecoach/config.toml` else `~/.config/stagecoach/config.toml`),
  and `overlay(dst, src *Config)` (field-by-field non-zero merge; provider map merged key-by-key).
  Plus `loadRepoLocalConfig()` which loads `./.stagecoach.toml` and prints a one-line stderr notice
  when it sets the provider (PRD §19). PRD §16.1 (resolution order, layers 2–3) + §16.2 (file shape)
  + §9.8/FR34 are the spec; arch `go_ecosystem_patterns.md` §2.4 (overlay/merge) + §5.5
  (map-of-struct decode) are the patterns. INPUT = `Config` struct + `Defaults()` from P1.M1.T4.S1
  (complete). OUTPUT = a merged `Config` from the two file layers for the `Load()` orchestrator
  (P1.M1.T4.S4). go-toml/v2 v2.4.2 is already a dependency (S1 added it) — **no go.mod/go.sum change**.
  DOCS: none yet (internal; public config-file docs ship with `config init`, P1.M4.T1.S4).

  ⚠️ **THE central design call — `Config` is NOT a TOML decode target; S2 introduces its own nested
  `fileConfig` decode struct, then materializes into the plain resolved `*Config`.** S1 deliberately
  shipped `Config` FLAT + plain-typed (`Timeout time.Duration`, no `[defaults]`/`[generation]`
  nesting) and proved it cannot be unmarshaled directly from the §16.2 file: that file is nested
  (`[defaults]`, `[generation]`, `[provider.X]`) and spells the duration as the STRING `"120s"`, which
  go-toml/v2 cannot decode into `time.Duration`. (Empirically re-confirmed: decoding into a plain
  nested struct leaves `Timeout` as the string `"120s"` — see research note.) The contract wording
  "toml.Unmarshal into Config" therefore means *decode into a Config-shaped intermediate, then yield a
  `*Config`*. Concretely, S2 defines an **unexported `fileConfig`** mirroring §16.2
  (`Defaults fileDefaults \`toml:"defaults"\`` · `Generation fileGeneration \`toml:"generation"\`` ·
  `Provider map[string]map[string]any \`toml:"provider"\``; `Timeout string` inside `fileDefaults`),
  decodes the file into it, and materializes the **non-zero** set fields into a fresh `*Config`
  (`time.ParseDuration` for the timeout). This honors S1's frozen `Config` shape AND the §16.2 file
  layout. **Do NOT change `Config`'s existing 12 fields/tags/values** (S1 froze them) — S2 only ADDS
  one field (see call #2).

  ⚠️ **THE second design call — `Config` gains ONE field: `Providers map[string]map[string]any`.**
  The contract says overlay merges "the Provider map … key-by-key", so a provider map must be
  reachable from `*Config`. But S1 deliberately deferred the provider manifest type (it is P1.M2.T1 —
  forward dependency; importing it here would risk an import cycle `config`→`provider`). S2 therefore
  carries user-defined / override provider definitions as a **raw `map[string]map[string]any`** (the
  generic form go-toml/v2 decodes `[provider.X]` into natively — empirically verified). The provider
  REGISTRY (P1.M2.T3) later consumes this map: for each name it re-encodes the entry to TOML and
  `toml.Unmarshal`s into a (pointer-typed) `Manifest`, then field-merges with the built-in manifest
  (PRD §16.1). Tag it **`toml:"-"`**: it is INPUT-only (populated by the loader, consumed by the
  registry) and must NOT appear in a flat `Config` marshal (avoids a tag clash with the existing
  `Provider string \`toml:"provider"\`` and keeps `config init` / S1's marshal test unaffected). This
  is an ADDITIVE extension of `Config` (no existing field renamed/retyped) — explicitly permitted.

  ⚠️ **THE third design call — overlay is NON-ZERO (per the explicit contract) → documented v1
  limitation for zero-value file overrides.** The contract specifies `overlay(dst, src *Config)` that
  "merges non-zero fields" and cites arch §2.4, whose overlay is precisely "if `src.X != zero` then
  `dst.X = src.X`". Because the resolved `Config` carries PLAIN types (S1 froze this), a file setting
  a field to its ZERO value (`auto_stage_all = false`, `strip_code_fence = false`,
  `max_duplicate_retries = 0`, etc.) is INDISTINGUISHABLE from "field absent" and is therefore NOT
  applied by overlay (arch §2.4 "Bool caveat" / FINDING 5). This is a deliberate, contract-specified
  tradeoff for the FILE layers (2 & 3): to force a zero value, use a higher layer — env vars
  (`STAGECOACH_AUTO_STAGE_ALL=0` etc., S4, presence-checked) or CLI flags (`--no-auto-stage`,
  `--no-strip-code-fence`, S4, `flag.Changed`-checked) — which CAN set `false`/`0`. S2 documents this
  limitation in code comments and ships a clean, contract-faithful non-zero overlay; the empirically-
  validated pointer-typed `fileConfig` upgrade (arch §5.4 Option A — `*bool`/`*int` correctly capture
  `false`/`0`) is noted as a one-function future refinement. **Do NOT silently "fix" this by making
  `Config` pointer-typed** (breaks S1's frozen plain struct + all consumers).

  ⚠️ **THE fourth design call — repo-local path is `.stagecoach.toml` (a FILE), NOT arch §2.8's
  `.stagecoach/config.toml` (a DIR).** The contract (point 1) and PRD §16.1 (h3.57 item 4) both say
  `./.stagecoach.toml`; arch `go_ecosystem_patterns.md` §2.8's `filepath.Join(".stagecoach",
  "config.toml")` is inconsistent and is NOT the authority here. `repoLocalConfigPath()` returns
  `".stagecoach.toml"`.

  ⚠️ **THE fifth design call — `globalConfigPath()` honors the XDG Base Directory Specification.**
  `$XDG_CONFIG_HOME` is used ONLY if set AND absolute (the XDG spec mandates ignoring a relative or
  empty value); otherwise fall back to `~/.config/stagecoach/config.toml` via `os.UserHomeDir()`
  (Go 1.12+, respects `$HOME` on Unix / `%USERPROFILE%` on Windows). This refines arch §2.8's
  simpler `xdg != ""` check by one `filepath.IsAbs` line — prevents a relative `XDG_CONFIG_HOME` from
  silently resolving against the CWD (a real footgun in shared dev boxes / CI).

  Deliverable: `internal/config/file.go` (`globalConfigPath`, `loadTOML`, `overlay`,
  `loadRepoLocalConfig`, + unexported `fileConfig`/`fileDefaults`/`fileGeneration`/
  `repoLocalConfigPath`/`repoProviderNotice`) and `internal/config/file_test.go` (white-box, temp
  TOML files via `t.TempDir()`, `t.Setenv` for the XDG path). Also MODIFY `internal/config/config.go`
  to ADD the `Providers` field (additive; `Defaults()` + all other fields unchanged). No `go.mod`/
  `go.sum` change. Touches ONLY `internal/config/`. INPUT = S1's `Config`+`Defaults()`. This feeds
  S3 (git-config reader) and S4 (`Load()` orchestrator); the provider map feeds P1.M2.T3.
---

## Goal

**Feature Goal**: Give Stagecoach the ability to read its TOML configuration from the two file layers
defined by PRD §16.1 — the GLOBAL file (`$XDG_CONFIG_HOME/stagecoach/config.toml`, default
`~/.config/stagecoach/config.toml`) and the REPO-LOCAL file (`./.stagecoach.toml`) — and to merge either
into the resolved `Config` from P1.M1.T4.S1 with correct precedence ("higher layer wins", FR34) and
correct section handling (`[defaults]` / `[generation]` / `[provider.X]`, string→Duration parse for
`timeout`). A missing file is a normal "no override" condition, never an error.

**Deliverable**:
1. **MODIFY** `internal/config/config.go` — ADD exactly one field to `Config`:
   `Providers map[string]map[string]any \`toml:"-"\`` (doc-commented per design call #2). Do NOT touch
   the existing 12 fields, their tags, `Defaults()` values, or imports. (`Defaults()` returns
   `Providers: nil` implicitly — leave the literal as-is or omit the key; either yields a nil map.)
2. **CREATE** `internal/config/file.go` (`package config`) — unexported decode structs + the functions:
   (a) `func globalConfigPath() string` — XDG-aware (set+absolute → `$XDG_CONFIG_HOME/stagecoach/config.toml`;
       else `~/.config/stagecoach/config.toml` via `os.UserHomeDir()`).
   (b) `func loadTOML(path string) (*Config, error)` — `os.ReadFile`; if `os.IsNotExist` → `(nil, nil)`;
       `toml.Unmarshal` into an unexported `fileConfig`; materialize non-zero fields into a fresh
       `*Config` (`time.ParseDuration` for `Timeout`; copy the provider map).
   (c) `func overlay(dst, src *Config)` — for each scalar field, if `src.X != zero` then `dst.X = src.X`;
       merge `src.Providers` into `dst.Providers` key-by-key (whole-entry replace per arch §2.4);
       `nil`-safe (`src == nil` or `src.Providers == nil` → no-op for that part).
   (d) `func loadRepoLocalConfig() (*Config, error)` — wraps `loadTOML(repoLocalConfigPath())`; if the
       loaded config sets `Provider != ""`, print a one-line notice to a swappable `io.Writer`
       (default `os.Stderr`) per PRD §19.
   (e) unexported helpers: `fileConfig`/`fileDefaults`/`fileGeneration` (the §16.2 decode shape),
       `repoLocalConfigPath()` (`".stagecoach.toml"`), `repoProviderNotice(cfg *Config) string`
       (pure: returns the notice text or `""`).
3. **CREATE** `internal/config/file_test.go` (`package config`, white-box) — `TestLoadTOMLValid`,
   `TestLoadTOMLMissing`, `TestOverlayPartial` (the contract's three required cases), plus
   `TestGlobalConfigPath` (XDG set / unset via `t.Setenv`), `TestOverlayProvidersKeyReplace`,
   `TestRepoProviderNotice`, and `TestLoadRepoLocalConfig`. Use `t.TempDir()` + `os.WriteFile` for
   temp TOML files; `t.Setenv` for env-dependent path logic; mirror `internal/git`/`config` test style.

No `Load()` orchestrator (that is S4). No env/CLI/git-config layers (S3, S4). No provider `Manifest`
type (P1.M2.T1). No file writing / `config init` (P1.M4.T1.S4).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` are clean;
`go test -race ./internal/config/` passes (S1's two tests stay GREEN — the `Providers` addition with
`toml:"-"` does not disturb them) and `go test -race ./...` stays green; `globalConfigPath()`
resolves correctly under set/unset/relative `XDG_CONFIG_HOME`; `loadTOML` decodes a §16.2 file
(`timeout "120s"` → `120*time.Duration`, `[provider.pi]` → `Config.Providers["pi"]`,
`[generation]`/`[defaults]` scalars) and returns `(nil, nil)` for a missing path; `overlay` copies
ONLY non-zero scalar fields (a partial src leaves dst's other fields intact) and replaces provider
entries per-key; `loadRepoLocalConfig` prints the §19 notice iff the repo file sets `provider`;
`go.mod`/`go.sum` are UNCHANGED (go-toml/v2 already pinned by S1).

## User Persona

**Target User**: Downstream Stagecoach subtasks — S3 (git-config reader, overlays onto the same
`*Config`) and S4 (`Load()` orchestrator, which calls `loadTOML(globalConfigPath())`,
`loadRepoLocalConfig()`, and `overlay` in §16.1 order); plus P1.M2.T3 (provider registry, consumes
`Config.Providers`). Transitively US8 (configuration & precedence, FR34/FR37) and every user who
tunes Stagecoach via a file.

**Use Case**: A user authors `~/.config/stagecoach/config.toml` (global) and/or `./.stagecoach.toml`
(per-repo). S2 turns each into a partial `*Config` and provides the merge primitive so S4 can build
the file-merged config before applying git/env/CLI layers.

**User Journey**: (internal API, no end-user surface yet) S4: `cfg := Defaults()` →
`overlay(&cfg, loadTOML(globalConfigPath()))` → `overlay(&cfg, loadRepoLocalConfig())` → (S3 git) →
(S4 env) → (S4 CLI). The §19 notice prints to stderr during the repo step when the repo file sets a
provider.

**Pain Points Addressed**: Removes "how do I decode the §16.2 nested file into the flat `Config`",
"what is the global path under XDG", "where does the provider map live", and "how do two file layers
merge" ambiguity for S3/S4/M2.T3 by landing the primitives now.

## Why

- **The file layers are the heart of FR34 (layers 2 & 3).** Landing `loadTOML` + `overlay` +
  `globalConfigPath` now lets S4's `Load()` be a thin, readable orchestrator and lets S3 focus purely
  on `git config` parsing.
- **Locks the §16.2 → `Config` decode bridge.** S1 froze `Config` as flat/plain; S2 defines the ONE
  intermediate (`fileConfig`) that bridges the nested, string-duration file to that struct. Future
  loaders (S3 git, S4 env/CLI) produce `*Config` directly and need no such bridge.
- **Establishes the provider-map carrier.** `[provider.X]` user definitions must reach the registry
  (P1.M2.T3) through the merged `Config`; S2 fixes the type (`map[string]map[string]any`, raw) and
  the merge rule (key-replace between file layers; field-merge with built-ins is the registry's job).
- **Honors PRD §19 (security/clarity).** A repo-local file silently redirecting the provider could be
  surprising (e.g. a cloned repo pointing `provider` at an unexpected agent). The one-line stderr
  notice flags this the moment it happens.
- **No user-facing surface change** (PRD "DOCS: none") — public config-file documentation ships with
  `config init` (P1.M4.T1.S4).

## What

A compiled `internal/config` package that can (1) resolve the global config path under XDG, (2) decode
a §16.2 TOML file into a partial `*Config` (or `(nil, nil)` if absent), (3) field-by-field overlay one
`*Config` onto another with provider-map key-replace, and (4) load the repo-local file with a §19
provider-redirect notice. `Config` carries a new raw `Providers` map. No orchestrator, no other layers,
no `Manifest` type, no file writing.

### Success Criteria

- [ ] `internal/config/file.go` exists, `package config`, imports only stdlib (`os`, `fmt`,
      `path/filepath`, `time`) + `github.com/pelletier/go-toml/v2` (already in `go.mod`).
- [ ] `globalConfigPath()` returns `$XDG_CONFIG_HOME/stagecoach/config.toml` when `XDG_CONFIG_HOME` is
      set AND absolute; otherwise `filepath.Join(home, ".config", "stagecoach", "config.toml")` where
      `home` comes from `os.UserHomeDir()`.
- [ ] `loadTOML(path)` returns `(nil, nil)` for a missing file (via `os.IsNotExist`); returns a
      wrapped error for other read errors or a TOML parse error; on success returns a `*Config`
      populated from the file's non-zero fields with `Timeout` parsed via `time.ParseDuration` from
      the §16.2 string (e.g. `"120s"` → `120*time.Second`).
- [ ] `overlay(dst, src *Config)` is `nil`-safe (`src == nil` → return); copies each scalar field only
      when `src`'s value is non-zero; merges `src.Providers` into `dst.Providers` key-by-key
      (initializing `dst.Providers` if nil), each key replacing dst's whole entry.
- [ ] `loadRepoLocalConfig()` loads `".stagecoach.toml"` and, iff the result is non-nil and its
      `Provider != ""`, writes the `repoProviderNotice` text to the package's notice writer
      (default `os.Stderr`).
- [ ] `Config` has the NEW field `Providers map[string]map[string]any \`toml:"-"\``; all other fields,
      tags, and `Defaults()` values are byte-identical to S1.
- [ ] `file_test.go` has the required + supplemental tests (all passing); uses `t.TempDir()` for files
      and `t.Setenv` for env-dependent path tests.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` clean; `go test -race ./...` green;
      `go.mod`/`go.sum` unchanged by S2 (`git diff --exit-code go.mod go.sum` is empty).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the S1 `Config`
shape (quoted in full below), the §16.2 file example, the empirically-verified decode behaviors (research
note), and the function specs. No git/provider/generation knowledge required — S2 is pure file I/O +
decode + merge.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- url: https://github.com/pelletier/go-toml/v2
  why: confirms (a) struct-tag decode into nested structs, (b) `[provider.X]` decodes into a Go map
       natively (no custom unmarshaler), (c) `toml:"-"` excludes a field from BOTH marshal & unmarshal,
       (d) there is NO `omitempty` (zero values are always emitted — relevant only for future `config init`).
  critical: a STRING TOML value (`timeout = "120s"`) decodes into a Go `string`, NOT into
       `time.Duration` — this is WHY S1's flat `Config` cannot be the decode target and S2 needs the
       intermediate `fileConfig`. (Empirically re-verified for v2.4.2 — see research note Finding C.)

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "2.4 Layers 2–3: TOML File Loading and Merging" (the `loadTOML` + `overlay` reference impl)
  why: the canonical overlay: `os.ReadFile` → `os.IsNotExist` ⇒ `(nil,nil)`; `toml.Unmarshal`; non-zero
       field-by-field scalar overlay; provider-map "each named entry replaces entirely (no sub-field
       merge within a provider)". S2 follows this PATTERN for scalars + provider key-replace.
  pattern: `if src.X != "" { dst.X = src.X }` for strings; `if src.N != 0 { dst.N = src.N }` for ints;
       `if src.B { dst.B = true }` for bools (NOTE the documented caveat — see call #3).
  gotcha: arch §2.4's overlay is written against a NESTED `Config{Defaults,Generation,Provider map}`;
       S1's `Config` is FLAT. Translate the §2.4 scalar-overlay idea onto S1's flat fields. Also: arch
       §2.4's repo path (`".stagecoach/config.toml"`) is WRONG for this project — use `.stagecoach.toml`
       (design call #4).

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "2.8 Full Load Function (All Layers)" — the `globalConfigPath()` reference impl.
  why: the XDG resolution pattern (`os.Getenv("XDG_CONFIG_HOME")` else `os.UserHomeDir()`). S2 adds the
       `filepath.IsAbs` guard (design call #5) to be XDG-spec-compliant.
  pattern: `filepath.Join(xdg, "stagecoach", "config.toml")` / `filepath.Join(home, ".config", "stagecoach", "config.toml")`.
  gotcha: do NOT copy arch §2.8's `Load()` — that is S4's job. S2 ships only `globalConfigPath()`,
       `loadTOML`, `overlay`, `loadRepoLocalConfig`.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "5.5 Decoding `[provider.X]` into `map[string]Manifest`" + "5.8 Decoding into map[string]any"
  why: confirms go-toml/v2 decodes a TOML table-map into a Go map natively. S2 uses the generic
       `map[string]map[string]any` (outer key = provider name; inner map = that provider's fields),
       empirically verified (research note Finding A). Inner scalar types: string→`string`,
       int→`int64`, array→`[]any` (P1.M2.T3 handles the int64/[]any when building `Manifest`).
  pattern: `Provider map[string]map[string]any \`toml:"provider"\``; nil if the file has no `[provider]`.

- file: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: "FINDING 5" (no `omitempty`; overlay/merge "absent vs zero" ambiguity)
  why: the root cause of S2's documented non-zero limitation and the pointer/map workaround. S2 ships
       the non-zero overlay (per contract) and DOCUMENTS the limitation; the pointer-based `fileConfig`
       (arch §5.4 Option A) is the noted future refinement.

- file: internal/config/config.go
  why: the INPUT contract — the EXACT `Config` struct (12 fields + tags) and `Defaults()` S2 builds on.
       S2 ADDS one field (`Providers`, `toml:"-"`); everything else is frozen.
  pattern: read it verbatim before editing; preserve field order, tags, comments, and `Defaults()` values.

- file: internal/config/config_test.go
  why: S1's existing tests MUST stay green after S2 adds `Providers` (`toml:"-"` ⇒ excluded from marshal
       ⇒ `TestTOMLMarshalKeysAndNoColorExclusion` still sees exactly the 11 leaf keys, no `no_color`).
       Also the test-style convention to mirror: white-box `package config`, stdlib `testing`, `t.Errorf`.

- file: PRD.md
  section: "16.1 Resolution order (FR34)" (h3.57) + "16.2 Full config file example" (h3.58) + "9.8" (h3.24)
  why: §16.1 fixes layers 2 (global) & 3 (repo-local `.stagecoach.toml`) and the field-by-field provider-
       merge-with-built-in principle (the registry's job, not S2's); §16.2 is the exact file shape to
       decode; FR34 is the "higher wins" invariant S2's overlay serves.

- file: plan/001_f1f80943ac34/P1M1T4S1/PRP.md
  why: confirms S1 deliberately DEFERRED the provider map ("provider manifests are P1.M2.T1 — out of
       scope"), anticipated that "loaders (S2-S4) decode into their own intermediate structs", and froze
       `Config` as flat/plain-typed. S2 honors all three.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2
go.sum                          # has the go-toml/v2 v2.4.2 hashes (S1 added them)
internal/
  config/
    config.go                   # S1: Config (12 fields) + Defaults()        ← MODIFY (add Providers)
    config_test.go              # S1: TestDefaults + TestTOMLMarshalKeys...   ← UNCHANGED (must stay green)
  git/                          # T2/T3 (RevParseHEAD…AddAll) — untouched by S2
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added/modified

```bash
internal/
  config/
    config.go                   # MODIFIED — +Providers map[string]map[string]any `toml:"-"` (additive)
    config_test.go              # UNCHANGED (S1)
    file.go                     # NEW — fileConfig + globalConfigPath + loadTOML + overlay
                                #        + loadRepoLocalConfig + repo helpers
    file_test.go                # NEW — TestLoadTOMLValid/Missing, TestOverlayPartial (+ supplemental)
# go.mod / go.sum UNCHANGED (go-toml/v2 already a dependency from S1)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the resolved `Config` is FLAT + plain-typed (S1 froze it); it is NOT a §16.2 decode target.
// The §16.2 file is NESTED ([defaults]/[generation]/[provider.X]) and spells `timeout` as the STRING
// "120s", which go-toml/v2 decodes into a Go `string`, NOT time.Duration. => S2 decodes into its OWN
// unexported `fileConfig` (nested, `Timeout string`) and materializes into the flat `*Config`, parsing
// the duration with time.ParseDuration. Do NOT attempt toml.Unmarshal(data, &Config{}).

// CRITICAL (v1 limitation, contract-specified): overlay is NON-ZERO. Because Config has plain types,
// a file cannot override a field to its ZERO value: `auto_stage_all = false`, `strip_code_fence = false`,
// `max_duplicate_retries = 0`, `verbose = false`(no-op vs default), `output = ""`, etc. are NOT applied
// by overlay (arch §2.4 "Bool caveat" / FINDING 5). Documented + escape hatches = env (S4) / CLI (S4),
// which ARE presence-checked and CAN set false/0. Do NOT "fix" by retyping Config to pointers (breaks
// S1 + all consumers). The validated future refinement = pointer-typed fileConfig + presence-aware overlay.

// CRITICAL: go-toml/v2 has NO omitempty; it marshals EVERY tagged field (incl. zero values). The new
// Providers field is tagged `toml:"-"` specifically so it (a) is excluded from any flat Config marshal
// (no clash with `Provider string \`toml:"provider"\``) and (b) is excluded from flat unmarshal (Config
// is never unmarshaled from §16.2 anyway — fileConfig is). Providers is populated ONLY by the loader.

// CRITICAL: the provider map's MERGE semantics differ by layer boundary. BETWEEN two file layers (S2's
// overlay) it is KEY-REPLACE (arch §2.4: repo's [provider.pi] replaces global's [provider.pi] entirely).
// BETWEEN a file layer and a BUILT-IN manifest it is FIELD-MERGE (PRD §16.1) — but that is P1.M2.T3's
// job, NOT S2's. S2 only accumulates raw entries; the registry decodes+merges them later.

// GOTCHA: repo-local path is the FILE `.stagecoach.toml` (contract + PRD §16.1), NOT arch §2.8's
// `.stagecoach/config.toml` directory. Do not copy arch §2.8's repo path.

// GOTCHA: `os.IsNotExist` (not `errors.Is(err, fs.ErrNotExist)`) is the idiomatic missing-file check
// here and matches arch §2.4. (`errors.Is(err, fs.ErrNotExist)` also works on Go 1.13+; either is fine —
// pick one and be consistent. os.IsNotExist is shortest and matches the cited pattern.)

// GOTCHA: time.ParseDuration("120s") → 120*time.Second (correct). But ParseDuration("") errors; guard
// with `if s != ""` before parsing. ParseDuration rejects "" and unknown units — surface a wrapped error
// so a malformed `timeout = "120"` (no unit) fails loudly at load, not silently at generation time.

// GOTCHA: a §16.2 file may omit [defaults], [generation], or [provider] entirely. A nested decode struct
// leaves the missing substruct zero-valued and a missing map nil — materialize must handle all three
// (copy only non-zero scalars; copy the provider map only if non-nil).

// MINOR: os.UserHomeDir() can error (e.g. $HOME unset). Per arch §2.8, fall back to a sensible default
// ("config.toml" in CWD) rather than crashing globalConfigPath() — but log nothing (S4/CLI handles UX).

// MINOR: the §19 notice is INFORMATIONAL (stderr), not data. Word it factually about the FILE's setting,
// not the final provider (higher layers may still override). Use a swappable package-level io.Writer so
// tests don't have to capture os.Stderr.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go  — ONLY CHANGE: add the Providers field (additive). Everything else is
// byte-identical to S1. Place the new field in a clearly-commented group AFTER the [generation] block.
package config

import "time"

type Config struct {
	// [defaults] (PRD §16.2)
	Provider     string        `toml:"provider"`
	Model        string        `toml:"model"`
	Timeout      time.Duration `toml:"timeout"`
	AutoStageAll bool          `toml:"auto_stage_all"`
	Verbose      bool          `toml:"verbose"`

	// CLI / UI only — NOT in the §16.2 config file
	NoColor bool `toml:"-"`

	// [generation] (PRD §16.2)
	MaxDiffBytes        int    `toml:"max_diff_bytes"`
	MaxMdLines          int    `toml:"max_md_lines"`
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"`
	SubjectTargetChars  int    `toml:"subject_target_chars"`
	Output              string `toml:"output"`
	StripCodeFence      bool   `toml:"strip_code_fence"`

	// [provider.<name>] user-defined / override provider definitions (PRD §16.2, §12.8).
	// Carried as a RAW map: the provider MANIFEST type is defined later (P1.M2.T1), so config must not
	// import it (import-cycle risk). The registry (P1.M2.T3) consumes this map — for each name it
	// re-encodes the entry to TOML and unmarshals into a Manifest, then field-merges with the built-in
	// manifest per PRD §16.1. toml:"-" => excluded from flat marshal (no clash with `Provider` string)
	// and from flat unmarshal (Config is never decoded from §16.2; fileConfig is). Populated by the
	// file loaders (P1.M1.T4.S2); nil means "no user-defined providers".
	Providers map[string]map[string]any `toml:"-"`
}

// Defaults() is UNCHANGED from S1 (Providers is implicitly nil — do not add it to the literal).
func Defaults() Config { /* …exactly as S1 shipped… */ }
```

```go
// internal/config/file.go  — NEW.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// fileConfig is the §16.2 file decode target: NESTED (matches [defaults]/[generation]/[provider.X]),
// with Timeout as a STRING ("120s") because go-toml/v2 cannot decode "120s" into time.Duration and the
// resolved Config is flat/plain (S1). loadTOML materializes this into a *Config. UNEXPORTED.
type fileConfig struct {
	Defaults   fileDefaults             `toml:"defaults"`
	Generation fileGeneration           `toml:"generation"`
	Provider   map[string]map[string]any `toml:"provider"` // nil if the file has no [provider] table
}

type fileDefaults struct {
	Provider     string `toml:"provider"`
	Model        string `toml:"model"`
	Timeout      string `toml:"timeout"` // §16.2 duration string, e.g. "120s"; parsed in loadTOML
	AutoStageAll bool   `toml:"auto_stage_all"`
	Verbose      bool   `toml:"verbose"`
}

type fileGeneration struct {
	MaxDiffBytes        int    `toml:"max_diff_bytes"`
	MaxMdLines          int    `toml:"max_md_lines"`
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"`
	SubjectTargetChars  int    `toml:"subject_target_chars"`
	Output              string `toml:"output"`
	StripCodeFence      bool   `toml:"strip_code_fence"`
}

// noticeOut is the destination for the §19 repo-local provider-redirect notice. Swappable for tests;
// defaults to os.Stderr. (PRD §19: a repo-local config redirecting the provider is surfaced to the user.)
var noticeOut io.Writer = os.Stderr

// globalConfigPath returns the GLOBAL config path (PRD §16.1 layer 2): $XDG_CONFIG_HOME/stagecoach/config.toml
// when XDG_CONFIG_HOME is set AND absolute (XDG Base Dir Spec: a relative/empty value is ignored);
// otherwise ~/.config/stagecoach/config.toml via os.UserHomeDir().
func globalConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "stagecoach", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml" // last-resort fallback (CWD); matches arch §2.8
	}
	return filepath.Join(home, ".config", "stagecoach", "config.toml")
}

// repoLocalConfigPath returns the REPO-LOCAL config path (PRD §16.1 layer 3): the file ./.stagecoach.toml.
// (Contract + PRD §16.1; NOT arch §2.8's .stagecoach/config.toml directory.)
func repoLocalConfigPath() string { return ".stagecoach.toml" }

// loadTOML reads and decodes a TOML file into a partial *Config (PRD §16.2). A MISSING file is the
// normal "no override" condition: it returns (nil, nil). Other read errors and parse errors are
// returned wrapped (with the path). Only NON-ZERO fields from the file are materialized (arch §2.4
// non-zero overlay semantics — see the v1 limitation note in Config.Providers / the PRP).
func loadTOML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // not an error: layer simply absent
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return materialize(&fc), nil
}

// materialize copies the NON-ZERO fields of a fileConfig into a fresh *Config. (Unexported helper.)
func materialize(fc *fileConfig) *Config {
	c := &Config{}
	// [defaults]
	if fc.Defaults.Provider != "" {
		c.Provider = fc.Defaults.Provider
	}
	if fc.Defaults.Model != "" {
		c.Model = fc.Defaults.Model
	}
	if fc.Defaults.Timeout != "" {
		d, err := time.ParseDuration(fc.Defaults.Timeout) // "120s" -> 120*time.Second
		if err != nil {
			// Defer error reporting to the caller path: materialize is best-effort; in practice
			// loadTOML should validate before calling materialize — see Implementation Patterns.
		}
		c.Timeout = d
	}
	if fc.Defaults.AutoStageAll {
		c.AutoStageAll = true
	} // NOTE: cannot set false here — see v1 limitation (non-zero overlay).
	if fc.Defaults.Verbose {
		c.Verbose = true
	}
	// [generation]
	if fc.Generation.MaxDiffBytes != 0 {
		c.MaxDiffBytes = fc.Generation.MaxDiffBytes
	}
	if fc.Generation.MaxMdLines != 0 {
		c.MaxMdLines = fc.Generation.MaxMdLines
	}
	if fc.Generation.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = fc.Generation.MaxDuplicateRetries
	}
	if fc.Generation.SubjectTargetChars != 0 {
		c.SubjectTargetChars = fc.Generation.SubjectTargetChars
	}
	if fc.Generation.Output != "" {
		c.Output = fc.Generation.Output
	}
	if fc.Generation.StripCodeFence {
		c.StripCodeFence = true
	} // NOTE: cannot set false here — v1 limitation.
	// [provider.X] — raw map, copied whole (nil-safe).
	c.Providers = fc.Provider
	return c
}

// overlay merges src into dst field-by-field (arch §2.4): each NON-ZERO scalar in src overrides dst;
// the Providers map is merged KEY-BY-KEY (a key in src replaces dst's whole entry for that key — no
// sub-field merge within a provider at the file↔file boundary; field-merge with BUILT-IN manifests is
// the registry's job, P1.M2.T3). Nil-safe: a nil src (or nil src.Providers) is a no-op for that part.
func overlay(dst, src *Config) {
	if src == nil {
		return
	}
	// [defaults]
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.AutoStageAll {
		dst.AutoStageAll = true
	}
	if src.Verbose {
		dst.Verbose = true
	}
	// [generation]
	if src.MaxDiffBytes != 0 {
		dst.MaxDiffBytes = src.MaxDiffBytes
	}
	if src.MaxMdLines != 0 {
		dst.MaxMdLines = src.MaxMdLines
	}
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
	if src.Output != "" {
		dst.Output = src.Output
	}
	if src.StripCodeFence {
		dst.StripCodeFence = true
	}
	// [provider.X]
	if len(src.Providers) > 0 {
		if dst.Providers == nil {
			dst.Providers = make(map[string]map[string]any, len(src.Providers))
		}
		for name, entry := range src.Providers {
			dst.Providers[name] = entry // key-level replace (arch §2.4)
		}
	}
}

// loadRepoLocalConfig loads the repo-local ./.stagecoach.toml. If it sets the default provider, a
// one-line notice is written to noticeOut (default os.Stderr) per PRD §19 (a repo file redirecting
// the provider is surfaced to the user). Returns (nil, nil) if the file is absent.
func loadRepoLocalConfig() (*Config, error) {
	cfg, err := loadTOML(repoLocalConfigPath())
	if err != nil {
		return nil, err
	}
	if msg := repoProviderNotice(cfg); msg != "" {
		fmt.Fprint(noticeOut, msg)
	}
	return cfg, nil
}

// repoProviderNotice returns the §19 notice text iff cfg is non-nil and sets Provider != ""; else "".
// Pure (no I/O) so it is trivially unit-testable. (The notice flags the FILE's setting, not the final
// provider — higher layers may still override.)
func repoProviderNotice(cfg *Config) string {
	if cfg == nil || cfg.Provider == "" {
		return ""
	}
	return fmt.Sprintf("stagecoach: repo-local config (.stagecoach.toml) sets provider to %q\n", cfg.Provider)
}
```

> **Implementer note on the `time.ParseDuration` error path:** the `materialize` sketch above silently
> swallows a bad duration. Do NOT ship that. The clean version makes `loadTOML` parse the duration
> itself and return a wrapped error on failure (see Implementation Patterns below) — `materialize`
> then receives an already-validated `time.Duration`. Pick ONE place to parse+validate (in `loadTOML`,
> before building the `*Config`) and keep `materialize`/`overlay` pure. The sketch shows the field-copy
> shape; the canonical version is in Implementation Patterns.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/config.go  — add the Providers field
  - ADD: `Providers map[string]map[string]any \`toml:"-"\`` as a new field group AFTER the [generation]
    block, with the doc comment in the Data Models block (explains raw-map rationale + toml:"-" + P1.M2.T3).
  - PRESERVE: the existing 12 fields, their tags, order, comments, the `Defaults()` values, and `import "time".
  - DO NOT add Providers to the Defaults() literal (leave it implicitly nil).
  - VERIFY after edit: `go build ./internal/config/` compiles; S1's `config_test.go` STILL PASSES
    (`toml:"-"` ⇒ Providers excluded from marshal ⇒ TestTOMLMarshalKeysAndNoColorExclusion unchanged).
  - WHY FIRST: file.go (Task 3) references Config.Providers; the field must exist before file.go compiles.

Task 2: CREATE internal/config/file.go  — decode structs + path helpers
  - IMPLEMENT: fileConfig/fileDefaults/fileGeneration (the Data Models block); globalConfigPath()
    (XDG set+absolute else ~/.config/...); repoLocalConfigPath() (".stagecoach.toml"); the package-level
    `var noticeOut io.Writer = os.Stderr` (+ `import "io"`).
  - IMPORTS: os, fmt, path/filepath, time, io, github.com/pelletier/go-toml/v2. (All already available.)
  - NAMING: unexported decode structs + path helpers; exported names only if S4 must call them directly
    (S4 calls loadTOML/globalConfigPath/overlay/loadRepoLocalConfig — keep those, they're already lowercase
    in-package which is fine since S4 is the SAME package `config`). Confirm: S3/S4 are `package config`
    too, so lowercase identifiers are reachable. (If a future split moves S4 out of package, revisit — not now.)
  - GOTCHA: globalConfigPath must check filepath.IsAbs(xdg), not just xdg != "" (XDG spec — design call #5).

Task 3: ADD loadTOML + materialize + overlay to internal/config/file.go
  - IMPLEMENT loadTOML(path): os.ReadFile → os.IsNotExist ⇒ (nil,nil); toml.Unmarshal into fileConfig;
    parse+validate Timeout via time.ParseDuration (return wrapped error on failure); build & return *Config.
  - IMPLEMENT materialize(fc, timeout) OR inline: copy non-zero [defaults]/[generation] scalars; set the
    parsed Timeout; copy the provider map (nil-safe). KEEP non-zero semantics (arch §2.4 / contract).
  - IMPLEMENT overlay(dst, src): nil-safe; non-zero scalar overlay per the Data Models block; provider
    key-replace (init dst.Providers if nil).
  - GOTCHA: do the time.ParseDuration in loadTOML (NOT materialize) so a bad duration is a load error.
  - GOTCHA: overlay's bool branches are `if src.X { dst.X = true }` — this is the documented v1 limitation;
    add a one-line comment pointing to the PRP/Config.Providers note. Do NOT add a `*bool` workaround.

Task 4: ADD loadRepoLocalConfig + repoProviderNotice to internal/config/file.go
  - IMPLEMENT repoProviderNotice(cfg) string (pure: "" when cfg nil or Provider == "").
  - IMPLEMENT loadRepoLocalConfig(): loadTOML(repoLocalConfigPath()); if msg := repoProviderNotice(cfg); msg != "" { fmt.Fprint(noticeOut, msg) }; return cfg, err.
  - GOTCHA: write to `noticeOut` (swappable), NOT os.Stderr directly — tests swap it to a bytes.Buffer.

Task 5: CREATE internal/config/file_test.go
  - PACKAGE: `package config` (white-box, like config_test.go / internal/git tests). Imports: os, os/exec
    (NOT needed), path/filepath, strings, testing, time, and (optionally) bytes/io for the notice test.
  - HELPER: `writeTempTOML(t, body) string` — t.TempDir()+os.WriteFile+filepath.Join; returns the path.
    (t.TempDir() auto-cleans; safe under -race.)
  - TEST A TestLoadTOMLValid: write a §16.2 file (defaults provider="pi"/timeout="90s"/auto_stage_all=true,
    generation max_diff_bytes=12345/output="json", provider.pi{default_model="glm-5.2"} + provider.myagent
    with a bare_flags array); loadTOML → assert Provider=="pi", Timeout==90*time.Second,
    AutoStageAll==true, MaxDiffBytes==12345, Output=="json", Providers["pi"]["default_model"]=="glm-5.2",
    Providers has keys {"pi","myagent"}.
  - TEST B TestLoadTOMLMissing: loadTOML(filepath.Join(t.TempDir(),"nope.toml")) → (nil, nil), err==nil.
  - TEST C TestOverlayPartial (CONTRACT CASE): dst=Defaults(); src=&Config{Timeout:90*time.Second,
    Output:"json"} (everything else zero); overlay(&dst, src) → dst.Timeout==90s, dst.Output=="json",
    and ALL OTHER fields unchanged from Defaults() (AutoStageAll still true, MaxDiffBytes still 300000, …).
    This proves field-by-field (NOT wholesale replace).
  - TEST D TestGlobalConfigPath: t.Setenv("XDG_CONFIG_HOME", absTmp) → endsWith "stagecoach/config.toml"
    under it; t.Setenv("XDG_CONFIG_HOME","") → under home/.config/stagecoach/config.toml; t.Setenv with a
    RELATIVE path → ignored (falls back to home). (Use t.Setenv — auto-restores, -race-safe.)
  - TEST E TestOverlayProvidersKeyReplace: dst.Providers={"pi":{"default_model":"A"},"claude":{…}},
    src.Providers={"pi":{"default_model":"B"}}; overlay → dst.Providers["pi"]=={"default_model":"B"}
    (replaced), "claude" still present.
  - TEST F TestRepoProviderNotice: repoProviderNotice(&Config{Provider:"pi"}) contains `.stagecoach.toml`
    and `pi`; repoProviderNotice(nil)==""; repoProviderNotice(&Config{})=="".
  - TEST G TestLoadRepoLocalConfig: write .stagecoach.toml (in a chdir'd t.TempDir OR pass a path —
    SIMPLEST: test repoProviderNotice + a small loadRepoLocalConfig test that swaps noticeOut to a
    buffer and asserts the notice is emitted when the CWD's .stagecoach.toml sets provider; use
    t.Chdir (Go 1.24+) or os.Chdir+restore if t.Chdir unavailable). If chdir is awkward, at minimum
    unit-test repoProviderNotice (pure) and assert loadRepoLocalConfig defers to loadTOML for I/O.
  - PATTERN: mirror internal/git/*_test.go style (t.Helper for helpers, t.Errorf for per-field asserts,
    no external test deps).

Task 6: VERIFY (no file change)
  - RUN the full Validation Loop (Levels 1–3). `git diff --exit-code go.mod go.sum` MUST be empty
    (S2 adds no dependency). S1's config_test.go MUST still pass (the Providers field is toml:"-").
```

### Implementation Patterns & Key Details

```go
// CANONICAL loadTOML — parse+validate the duration HERE (not in materialize) so a bad value is a load error.
func loadTOML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	// Validate the duration string up front (a malformed "timeout" must fail at LOAD, not at generation).
	var timeout time.Duration
	if fc.Defaults.Timeout != "" {
		timeout, err = time.ParseDuration(fc.Defaults.Timeout) // "120s" -> 120*time.Second
		if err != nil {
			return nil, fmt.Errorf("parse config %s: invalid timeout %q: %w", path, fc.Defaults.Timeout, err)
		}
	}
	return materialize(&fc, timeout), nil
}

// CANONICAL materialize — pure; receives an already-parsed duration. Non-zero overlay semantics.
func materialize(fc *fileConfig, timeout time.Duration) *Config {
	c := &Config{Timeout: timeout} // zero if the file didn't set one (overlay skips zero — correct)
	d, g := &fc.Defaults, &fc.Generation
	if d.Provider != "" { c.Provider = d.Provider }
	if d.Model != "" { c.Model = d.Model }
	if d.AutoStageAll { c.AutoStageAll = true }      // v1 limitation: cannot set false via file
	if d.Verbose { c.Verbose = true }
	if g.MaxDiffBytes != 0 { c.MaxDiffBytes = g.MaxDiffBytes }
	if g.MaxMdLines != 0 { c.MaxMdLines = g.MaxMdLines }
	if g.MaxDuplicateRetries != 0 { c.MaxDuplicateRetries = g.MaxDuplicateRetries }
	if g.SubjectTargetChars != 0 { c.SubjectTargetChars = g.SubjectTargetChars }
	if g.Output != "" { c.Output = g.Output }
	if g.StripCodeFence { c.StripCodeFence = true } // v1 limitation: cannot set false via file
	c.Providers = fc.Provider                         // nil-safe: nil if no [provider] table
	return c
}

// CANONICAL overlay — see Data Models block; reprinted here for the implementer's single-source view.
// (Copy the overlay body from the Data Models section verbatim; do not deviate from non-zero semantics.)
```

```go
// file_test.go — TestOverlayPartial (the CONTRACT "merge partial override" case): proves field-by-field.
func TestOverlayPartial(t *testing.T) {
	dst := Defaults() // Layer-1 baseline (AutoStageAll=true, MaxDiffBytes=300000, Timeout=120s, …)
	src := &Config{Timeout: 90 * time.Second, Output: "json"} // a PARTIAL override: only 2 fields set
	overlay(&dst, src)
	if dst.Timeout != 90*time.Second { t.Errorf("Timeout = %v, want 90s", dst.Timeout) }
	if dst.Output != "json" { t.Errorf("Output = %q, want json", dst.Output) }
	// Everything else MUST be untouched (NOT a wholesale replace):
	if !dst.AutoStageAll { t.Errorf("AutoStageAll clobbered: false, want true (partial merge broken)") }
	if dst.MaxDiffBytes != 300000 { t.Errorf("MaxDiffBytes clobbered: %d, want 300000", dst.MaxDiffBytes) }
	if dst.Provider != "" { t.Errorf("Provider clobbered: %q, want empty", dst.Provider) }
}

// file_test.go — TestLoadTOMLValid (decode + duration parse + provider map).
func TestLoadTOMLValid(t *testing.T) {
	body := `
[defaults]
provider = "pi"
timeout = "90s"
auto_stage_all = true

[generation]
max_diff_bytes = 12345
output = "json"

[provider.pi]
default_model = "glm-5.2"

[provider.myagent]
command = "/opt/myagent/bin/agent"
bare_flags = ["--no-mcp", "--ephemeral"]
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil { t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err) }
	if cfg.Provider != "pi" { t.Errorf("Provider=%q want pi", cfg.Provider) }
	if cfg.Timeout != 90*time.Second { t.Errorf("Timeout=%v want 90s", cfg.Timeout) }
	if !cfg.AutoStageAll { t.Errorf("AutoStageAll=false want true") }
	if cfg.MaxDiffBytes != 12345 { t.Errorf("MaxDiffBytes=%d want 12345", cfg.MaxDiffBytes) }
	if cfg.Output != "json" { t.Errorf("Output=%q want json", cfg.Output) }
	if len(cfg.Providers) != 2 { t.Errorf("Providers len=%d want 2", len(cfg.Providers)) }
	if m := cfg.Providers["pi"]; m["default_model"] != "glm-5.2" { t.Errorf("pi.default_model=%v", m["default_model"]) }
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. go-toml/v2 v2.4.2 is already required (S1). file.go imports it (genuinely used by
    loadTOML's toml.Unmarshal), so `go mod tidy` is a no-op. `git diff --exit-code go.mod go.sum` empty.

CONFIG STRUCT (internal/config/config.go):
  - add field: `Providers map[string]map[string]any \`toml:"-"\`` (additive; frozen S1 fields untouched).

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M1.T4.S3 (git-config): will produce a *Config from `git config stagecoach.*` and call overlay().
  - P1.M1.T4.S4 (Load orchestrator): cfg := Defaults(); overlay(&cfg, loadTOML(globalConfigPath()));
        overlay(&cfg, loadRepoLocalConfig()); (then S3 git, S4 env, S4 CLI). S2 ships the primitives;
        S4 does the sequencing. overlay's NON-ZERO semantics are part of this contract — do not change them.
  - P1.M2.T3 (provider registry): reads cfg.Providers (raw map); for each name re-encodes the entry to
        TOML and unmarshals into a Manifest, then field-merges with the built-in per PRD §16.1.

NO DATABASE / NO ROUTES / NO FILE WRITING (config init is P1.M4.T1.S4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (config.go) + Tasks 2–4 (file.go):
go build ./...                       # Whole module compiles incl. the extended Config + new file.go. Expect exit 0.
gofmt -w internal/config/            # Auto-align tags/comments; then verify:
test -z "$(gofmt -l internal/config/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/            # (and `go vet ./...`) Expect zero diagnostics (io.Writer var, etc.).
# Expected: all clean. gofmt will align the new Providers tag with the [generation] block — let it.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new + existing tests (white-box, stdlib testing):
go test -race ./internal/config/ -v
# Expected: PASS — TestDefaults, TestTOMLMarshalKeysAndNoColorExclusion (S1, STILL GREEN because
#   Providers is toml:"-"), and the new TestLoadTOMLValid/TestLoadTOMLMissing/TestOverlayPartial/
#   TestGlobalConfigPath/TestOverlayProvidersKeyReplace/TestRepoProviderNotice/TestLoadRepoLocalConfig.

# Full suite must stay green (no regression in internal/git or elsewhere):
go test -race ./...
# Expected: all packages PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# No CLI/runtime integration yet (no Load(), no command wiring). Validate deps + build end-to-end:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED by S2"   # MUST be empty.
grep -c 'Providers map\[string\]map\[string\]any' internal/config/config.go   # prints 1.
# Expected: binary builds; go.mod/go.sum unchanged; exactly one Providers field added.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Smoke the decode/merge by hand against a realistic §16.2 file (sanity for the S4 author + M2.T3):
cat > /tmp/sh.toml <<'EOF'
[defaults]
provider = "pi"
timeout = "120s"

[provider.pi]
default_model = "glm-5.2"

[provider.myagent]
command = "/opt/myagent/bin/agent"
bare_flags = ["--no-mcp"]
EOF
# (Optional) a tiny Go snippet in a throwaway module that calls config.loadTOML + overlay and prints
# cfg.Provider/cfg.Timeout/cfg.Providers — OR rely on TestLoadTOMLValid which already asserts these.
go test ./internal/config/ -run 'TestLoadTOMLValid|TestOverlayPartial|TestGlobalConfigPath' -v
# Inspect: timeout decoded to 120s; provider map has pi+myagent; overlay is partial (no clobber).
# (Optional) lint:
golangci-lint run ./internal/config/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint is project-wide; run `make lint` in CI)."
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`.
- [ ] Level 2 green: `go test -race ./internal/config/ -v` (S1's two + the new ones) AND `go test -race ./...`.
- [ ] Level 3: binary builds; `git diff --exit-code go.mod go.sum` empty; exactly one `Providers` field added.

### Feature Validation

- [ ] `globalConfigPath()` honors set+absolute `XDG_CONFIG_HOME`, falls back to `~/.config/stagecoach/config.toml`,
      and ignores a relative `XDG_CONFIG_HOME` (3 sub-assertions).
- [ ] `loadTOML` returns `(nil, nil)` for a missing file (no error); decodes a §16.2 file with
      `timeout "90s"`→`90*time.Second`, `[provider.pi]`→`cfg.Providers["pi"]`, and the `[defaults]`/
      `[generation]` scalars; returns a wrapped error on a malformed `timeout` or invalid TOML.
- [ ] `overlay` copies ONLY non-zero scalars (TestOverlayPartial: a 2-field src leaves all other dst
      fields at their Defaults() values) and replaces provider entries key-by-key (TestOverlayProvidersKeyReplace).
- [ ] `loadRepoLocalConfig` loads `.stagecoach.toml` and emits the §19 notice iff it sets `Provider != ""`
      (TestRepoProviderNotice + TestLoadRepoLocalConfig).
- [ ] `Config` has the new `Providers map[string]map[string]any \`toml:"-"\``; S1's 12 fields/tags/
      `Defaults()` values unchanged (TestDefaults + TestTOMLMarshalKeysAndNoColorExclusion still green).

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package config`, stdlib `testing`, `t.Errorf` field-by-field
      asserts, `t.TempDir()`/`t.Setenv` for isolated I/O & env (mirrors `internal/git`/`config` tests).
- [ ] File placement matches the desired tree (`file.go` + `file_test.go`; one additive field in `config.go`).
- [ ] Non-zero overlay semantics preserved (arch §2.4); the bool/zero limitation is documented in code.
- [ ] No premature scope: no `Load()`, no env/CLI/git-config layers, no `Manifest` type, no file writing.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comments on `Config.Providers`, `fileConfig`, `globalConfigPath`, `loadTOML`, `overlay`,
      `loadRepoLocalConfig`, `repoProviderNotice` explain the design calls (XDG, non-zero limitation,
      raw-map carriers, §19 notice).
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none" — public config-file docs ship with
      `config init`, P1.M4.T1.S4).
- [ ] `go.mod`/`go.sum` are UNCHANGED (the only non-`internal/config/`-only touch is the additive
      `config.go` field, which is inside `internal/config/`).

---

## Anti-Patterns to Avoid

- ❌ Don't `toml.Unmarshal(data, &Config{})` directly — `Config` is flat/plain with `Timeout time.Duration`;
  the §16.2 file is nested with `timeout = "120s"` (a string). Decode into the unexported `fileConfig` and
  materialize. (S1 froze `Config` as the resolved type, not a decode target.)
- ❌ Don't retype `Config`'s fields to pointers to "fix" the non-zero limitation — that breaks S1's frozen
  plain struct and every consumer (`cfg.Timeout`, `cfg.Verbose`, …). The limitation is contract-specified
  (arch §2.4); the validated fix is a pointer-typed `fileConfig` + presence-aware overlay (future refinement).
- ❌ Don't change `overlay`'s non-zero semantics to wholesale-replace or to "always take src" — the contract
  mandates field-by-field non-zero merge. Wholesale replace would clobber the Defaults() baseline.
- ❌ Don't merge provider entries FIELD-by-field in `overlay` — at the file↔file boundary it is KEY-REPLACE
  (arch §2.4). Field-merge with BUILT-IN manifests is the registry's job (P1.M2.T3, PRD §16.1).
- ❌ Don't import `provider`/`Manifest` into `config` — the type doesn't exist yet (P1.M2.T1) and would risk
  an import cycle. Carry providers as the raw `map[string]map[string]any`.
- ❌ Don't tag `Providers` with a real key (e.g. `toml:"provider"`) — it clashes with `Provider string`'s tag
  and would leak into flat marshaling. Use `toml:"-"`.
- ❌ Don't use `.stagecoach/config.toml` (arch §2.8's directory) for the repo-local path — the contract and
  PRD §16.1 say `.stagecoach.toml` (a file). Use `repoLocalConfigPath() == ".stagecoach.toml"`.
- ❌ Don't treat `XDG_CONFIG_HOME` as authoritative when it's relative or empty — the XDG spec says ignore
  it then; guard with `filepath.IsAbs`.
- ❌ Don't swallow a bad `timeout` silently — parse it in `loadTOML` and return a wrapped error so a
  malformed value fails at LOAD, not silently at generation time.
- ❌ Don't write the §19 notice straight to `os.Stderr` — write to the swappable `noticeOut io.Writer` so
  tests don't have to capture stderr.
- ❌ Don't implement `Load()` / env / CLI / git-config here — those are S4 / S3. S2 ships file primitives.
- ❌ Don't add a dependency — go-toml/v2 is already pinned (S1); `git diff go.mod go.sum` must be empty.
- ❌ Don't break S1's tests — adding `Providers` with `toml:"-"` keeps `TestTOMLMarshalKeysAndNoColorExclusion`
  green (Providers is marshal-excluded). If it breaks, you tagged `Providers` with a real key.
