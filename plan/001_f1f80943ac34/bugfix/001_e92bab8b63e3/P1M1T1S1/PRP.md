---
name: "P1.M1.T1.S1 — Add resolved-config injection to Options; skip config.Load in resolveConfig when provided"
description: |
  Bugfix (Issue 1 + Issue 5 root cause). Add an additive, nil-optional `Config *config.Config` field to
  `pkg/stagecoach.Options`. In `resolveConfig`, when `opts.Config != nil`, use a shallow copy of the
  caller-supplied config and SKIP `config.Load` entirely (preserving the `os.Getwd()` repoDir derivation
  and the existing Options Provider/Model/Timeout/Verbose overrides). When `opts.Config == nil`,
  behavior is UNCHANGED (standalone single-Load path). No signature change, no docs, no CLI wiring.
---

## Goal

**Feature Goal**: Close the architectural seam that is the **single root cause** of PRD Issue 1
(`--config` silently ignored by the default commit action) and Issue 5 (the §19 repo-local provider
notice printed twice): `pkg/stagecoach.resolveConfig` unconditionally re-runs `config.Load` with a
fresh `LoadOpts` that drops `--config` and re-fires Layer-3 side effects. Give callers a way to inject
an already-resolved config so `GenerateCommit` does not load a second time.

**Deliverable** (exactly two edits in ONE file, `pkg/stagecoach/stagecoach.go`):
1. A new additive field `Config *config.Config` on the `Options` struct (documented nil-optional /
   in-module CLI use, ADDITIVE-ONLY per the v1.0 stability contract).
2. A branch at the top of `resolveConfig`: if `opts.Config != nil`, `cfg := *opts.Config` (shallow
   value copy) and **skip** the `config.Load` call; keep the `os.Getwd()` repoDir derivation and the
   existing `opts.Provider/Model/Timeout/Verbose` override block.

**Success Definition**: `GenerateCommit`'s signature and `Result` are unchanged; when a caller passes
a non-nil `Options.Config`, `resolveConfig` returns that config (with Options overrides applied) and
performs **zero** `config.Load` calls; when `Options.Config` is nil, `resolveConfig` behaves
byte-for-byte as today. All existing tests pass under `-race`; build/vet/gofmt clean.

## User Persona

**Target User**: The Stagecoach contributor implementing the immediately-following subtasks S2 (CLI
wiring) and S3 (regression tests), plus future library consumers.

**Use Case**: S2 (`internal/cmd/default_action.go`) will pass the CLI's already-loaded `*config.Config`
(produced once by `PersistentPreRunE`, which honors `--config`) into `GenerateCommit` via this new
field. Without S1, S2 has no field to set.

**Pain Points Addressed**: Removes the "Options-as-flag-relay" workaround (the comment at
`default_action.go:130-134`) that compensated for `Flags: nil` but had **no field for `--config`** and
no way to suppress the duplicate Layer-3 notice.

## Why

- **Single root cause, two bugs.** Per `architecture/decisions.md` D1 and `seam_config_handoff.md` §4/§9,
  the one `config.Load` call inside `resolveConfig` (`stagecoach.go:114`) is responsible for BOTH the
  dropped `--config` (ConfigPathOverride omitted) and the doubled §19 notice (Layer-3
  `loadRepoLocalConfig` runs a second time). Letting the caller supply the resolved config eliminates
  the second Load entirely, fixing both at once.
- **PRD "Option a" (chosen over Option b).** Adding a `ConfigPathOverride` field alone (Option b) would
  fix Issue 1 but NOT Issue 5 (the second Load still runs). Passing the resolved config fixes both and
  removes the double-load's side effects wholesale.
- **API-stable.** `Options` is documented "Stable as of v1.0 / ADDITIVE-ONLY" — adding a field is
  explicitly permitted. `pkg/stagecoach` **already imports `internal/config`** (used today in
  `resolveConfig`'s signature), so the new field type introduces **no new import edge** and no
  import-cycle risk.
- **Precedence contract preserved.** `Options` overrides > Layer-7 flags > env > git-config >
  repo-local > global > defaults. The CLI's first Load already folded Layer-7 in; re-applying
  `opts.Provider/Model/Timeout/Verbose` on the passed config is redundant-but-correct and keeps the
  standalone-library path (`Options.Config == nil`) identical to today.
- **Scope discipline.** This subtask is the **field + branch only**. Wiring the CLI through it is S2;
  the cross-package regression tests are S3. S1 ships a focused unit test proving the injected path.

## What

A purely additive internal change to `pkg/stagecoach/stagecoach.go`:

1. **`Options` struct** gains one field: `Config *config.Config`.
2. **`resolveConfig`** gains a nil-check: if `opts.Config != nil`, copy the supplied config by value
   and skip `config.Load`; otherwise run the existing load path unchanged.

No changes to `GenerateCommit`'s signature, `Result`, `buildDeps`, `runPipeline`, the CLI
(`internal/cmd/*`), the config package, docs, or any existing test.

### Success Criteria

- [ ] `Options` has a new field `Config *config.Config`, documented as nil-optional / ADDITIVE-ONLY.
- [ ] `resolveConfig`, when `opts.Config != nil`, does **not** call `config.Load` (the
      `config.Load(ctx, config.LoadOpts{...})` line is inside an `else` / guarded by `opts.Config == nil`).
- [ ] `resolveConfig` still derives `repoDir` via `os.Getwd()` in both branches.
- [ ] The existing `opts.Provider/Model/Timeout/Verbose` override block runs in both branches
      (redundant for the CLI path, mandatory for the standalone-library precedence contract).
- [ ] `GenerateCommit` signature is unchanged: `func GenerateCommit(ctx context.Context, opts Options) (Result, error)`.
- [ ] `Options.Config == nil` path is byte-for-byte identical to the current behavior.
- [ ] A new focused unit test in `pkg/stagecoach/stagecoach_test.go` proves the injected-config path
      resolves the injected provider (no `--config` file, no repo-local `.stagecoach.toml` provider →
      only the in-memory `Config.Providers` map can supply the provider, proving Load was skipped).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` all clean; `go test -race ./...` green (existing
      tests untouched and passing).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current `Options` struct, the exact current
`resolveConfig` body, the precise edit (two locations in one file), the copy-semantics gotcha, and the
executable validation commands. The architecture docs (`decisions.md` D1, `seam_config_handoff.md`)
are referenced by section with their conclusions distilled inline.

### Documentation & References

```yaml
# MUST READ — the binding architectural decision (do not re-litigate)
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: "D1 is the binding choice: Option (a) — pass the resolved *config.Config into GenerateCommit via an additive Options field; skip config.Load when provided."
  critical: "D1 rejects Option (b) (ConfigPathOverride-only) because it fixes Issue 1 but NOT Issue 5. D1 also documents the shallow-copy precedent, the preserved precedence contract, and the in-module-only field rationale. S1 implements D1's field + resolveConfig branch ONLY (not the runDefault wiring — that is S2)."

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_handoff.md
  why: "§4 quotes the exact buggy resolveConfig body (the smoking-gun config.Load line) and §9 shows the data-flow diagram (notice #1 from PersistentPreRunE, notice #2 from resolveConfig). §10 Option B is exactly S1's design."
  section: "§4 (resolveConfig body), §9 (data-flow), §10 Option B"
  critical: "§4 proves the fix is single-point: guarding the ONE config.Load call closes both bugs. §7 lists the verbatim signatures (Options, resolveConfig, Load, LoadOpts, Config) an implementer must match."

# The two source files under edit / cross-reference
- file: pkg/stagecoach/stagecoach.go
  why: "THE edit target. Contains Options (struct), resolveConfig (the function to branch), GenerateCommit (signature — must NOT change). pkg/stagecoach already imports internal/config — no new import."
  pattern: "resolveConfig currently: repoDir:=os.Getwd() → cfgPtr,err:=config.Load(...) → cfg:=*cfgPtr → apply overrides → return. New branch inserts 'if opts.Config != nil { cfg = *opts.Config } else { <existing Load> }' BEFORE the override block."
  gotcha: "cfg := *opts.Config is a SHALLOW copy — the Providers map header is shared. This is SAFE (resolveConfig/buildDeps never write cfg.Providers[k]) and matches the existing cfg := *cfgPtr pattern. Do NOT deep-copy."

- file: internal/config/load.go
  why: "Defines Load(ctx, LoadOpts) (*Config, error) and LoadOpts{ConfigPathOverride, RepoDir, Flags}. The S1 change makes config.Load OPTIONAL in resolveConfig; it does NOT touch load.go."
  pattern: "Load is unchanged. The new field's type is *config.Config (the value Load returns)."

- file: internal/config/config.go
  why: "Defines the Config struct (incl. Providers map[string]map[string]any at line ~55) and Defaults(). Read to confirm the shallow-copy safety: only scalar fields are written by resolveConfig's override block."
  gotcha: "Providers is a map → shallow copy aliases it. Safe per above. NoColor has toml:\"-\"; pkg/stagecoach does not read NoColor (grep confirms zero refs) — do not add handling for it in S1."

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1-specific findings: the single edit locus, the shallow-copy safety proof, the no-new-import confirmation, the unaffected-existing-tests inventory, and the S1-vs-S2-vs-S3 scope boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── pkg/stagecoach/
│   ├── stagecoach.go        # EDIT TARGET (Options + resolveConfig)
│   └── stagecoach_test.go   # EDIT TARGET (add ONE focused unit test for the injected path)
└── internal/
    ├── config/
    │   ├── config.go       # Config struct (read-only ref — Providers map, Defaults)
    │   ├── load.go         # Load / LoadOpts (read-only ref — NOT edited in S1)
    │   └── file.go         # loadRepoLocalConfig / §19 notice (NOT edited in S1; the double-print
    │                       #   disappears as a side effect of resolveConfig not calling Load again)
    └── cmd/
        └── default_action.go  # NOT edited in S1 (S2 wires Options.Config = Config() here)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── pkg/stagecoach/
    ├── stagecoach.go        # MODIFIED: Options +1 field; resolveConfig +1 nil-check branch
    └── stagecoach_test.go   # MODIFIED: +1 focused test (TestGenerateCommit_InjectedConfig)
# (no other files touched)
```

| Path | Action | Responsibility |
|---|---|---|
| `pkg/stagecoach/stagecoach.go` | MODIFY | Add `Config *config.Config` to `Options`; branch `resolveConfig` to skip `config.Load` when non-nil. |
| `pkg/stagecoach/stagecoach_test.go` | MODIFY | Add one unit test proving the injected-config path resolves the injected provider without a Load. |

**Explicitly NOT touched in S1** (later subtasks / out of scope):
`internal/cmd/default_action.go` (S2 wiring), `internal/cmd/default_action_test.go` (S3 regression),
`internal/config/*` (no change — the notice fix is a side effect, not an edit), `GenerateCommit`
signature / `Result`, docs (contract: "DOCS: none").

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL: cfg := *opts.Config is a SHALLOW copy. Config.Providers (map[string]map[string]any,
// config.go:55) is a map → the copy shares the map header with the caller's *Config.
// This is SAFE here: resolveConfig's override block writes ONLY scalars (Provider/Model/Timeout/
// Verbose); buildDeps reads cfg.Providers via provider.DecodeUserOverrides (read-only re-encode).
// It is ALSO identical to the existing pattern — the Load path already does `cfg := *cfgPtr`.
// DO NOT deep-copy / clone Providers. The contract says "set cfg := *opts.Config (copy)" — a plain
// value copy. Deep-copying diverges from the Load path and is scope creep.

// CRITICAL: keep the os.Getwd() repoDir derivation in BOTH branches. resolveConfig returns
// (config.Config, string, error) where the string is repoDir — buildDeps(cfg, repoDir) needs it to
// construct git.New(repoDir). The injected-config branch must still call os.Getwd().

// CRITICAL: keep the existing opts.Provider/Model/Timeout/Verbose override block running for BOTH
// branches. For the CLI path (S2) these overrides are redundant (PersistentPreRunE already folded
// Layer-7 flags into the passed cfg), but the standalone-library contract requires Options to be the
// highest-precedence layer. Removing the override block would break the Options>everything guarantee.

// GOTCHA: no new import. pkg/stagecoach already imports internal/config (resolveConfig's return type
// + config.Load). The new field *config.Config reuses it. Do NOT add an import.

// GOTCHA (API stability): Options is documented "Stable as of v1.0 / ADDITIVE-ONLY". Adding a field
// is permitted. External (out-of-module) callers cannot name the unexported internal/config.Config
// type, so they cannot set Config non-nil — that's fine, the field is for the in-module CLI (S2).
// Document the field as nil-optional in the same style as the existing Verbose/VerboseOn fields.

// GOTCHA (test boundary): the "§19 notice printed exactly once" property is ONLY observable end-to-
// end through the CLI — loadRepoLocalConfig writes to internal/config.noticeOut (unexported package
// var, inaccessible from pkg/stagecoach tests). That assertion belongs to S3 (default_action_test.go),
// NOT S1. S1's own test proves Load was skipped by resolving an injected provider that exists ONLY in
// the in-memory Config.Providers map (no on-disk config file supplies it).
```

## Implementation Blueprint

### Data models and structure

No new data models. The change extends the existing `Options` struct with one field and adds a branch
to the existing `resolveConfig`. The relevant types (verbatim from source):

```go
// pkg/stagecoach/stagecoach.go (EXISTING — to be extended)
type Options struct {
	Provider    string
	Model       string
	SystemExtra string
	DryRun      bool
	Timeout     time.Duration
	Verbose     io.Writer
	VerboseOn   bool
	// Config  *config.Config   ← NEW FIELD (S1): injected resolved config; nil ⇒ config.Load (unchanged)
}

// internal/config/load.go (EXISTING — unchanged by S1)
func Load(ctx context.Context, opts LoadOpts) (*Config, error)
type LoadOpts struct {
	ConfigPathOverride string
	RepoDir            string
	Flags              *pflag.FlagSet
}

// internal/config/config.go (EXISTING — unchanged by S1)
type Config struct {
	Provider            string
	Model               string
	Timeout             time.Duration
	AutoStageAll        bool
	Verbose             bool
	NoColor             bool            `toml:"-"`
	MaxDiffBytes        int
	MaxMdLines          int
	MaxDuplicateRetries int
	SubjectTargetChars  int
	Output              string
	StripCodeFence      bool
	Providers           map[string]map[string]any  `toml:"-"`  // ← map: shallow-copied on *opts.Config
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the Config field to Options (pkg/stagecoach/stagecoach.go)
  - LOCATE: the Options struct (the one with the "ADDITIVE-ONLY for future versions" doc comment).
  - ADD a new field as the LAST field of the struct (preserves field order; additive-only):
        // Config optionally supplies an already-resolved configuration, skipping config.Load entirely
        // (still subject to the Options overrides below). nil ⇒ config.Load runs as before (standalone
        // path). Intended for in-module callers (the CLI) that have already loaded config once and want
        // to avoid a second load. Additive-only (PRD §14.1); external callers leave this nil.
        Config *config.Config
  - NAMING: `Config` (exported, matches the type name; consistent with Options' other exported fields).
  - TYPE: `*config.Config` (pointer, nil-optional; matches the value returned by config.Load).
  - VERIFY no new import needed: `config` is already imported (resolveConfig uses config.Load/Config).
  - DO NOT: change any other Options field, the Result struct, or GenerateCommit's signature.

Task 2: BRANCH resolveConfig to skip config.Load when opts.Config != nil
  - LOCATE: resolveConfig(ctx, opts Options) (config.Config, string, error) — ~line 108-141.
  - PRESERVE the leading repoDir derivation verbatim:
        repoDir, err := os.Getwd()
        if err != nil { return config.Config{}, "", fmt.Errorf("getwd: %w", err) }
  - REPLACE the unconditional load:
        cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
        if err != nil { return config.Config{}, "", fmt.Errorf("load config: %w", err) }
        cfg := *cfgPtr // copy to value
    WITH a nil-guarded branch:
        var cfg config.Config
        if opts.Config != nil {
            cfg = *opts.Config // shallow copy; skip config.Load (caller already resolved it)
        } else {
            cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
            if err != nil { return config.Config{}, "", fmt.Errorf("load config: %w", err) }
            cfg = *cfgPtr // copy to value
        }
  - PRESERVE the existing override block VERBATIM and UNCONDITIONALLY (runs in both branches):
        if opts.Provider != "" { cfg.Provider = opts.Provider }
        if opts.Model != ""    { cfg.Model = opts.Model }
        if opts.Timeout != 0   { cfg.Timeout = opts.Timeout }
        if opts.VerboseOn      { cfg.Verbose = true }
        return cfg, repoDir, nil
  - GOTCHA: use `var cfg config.Config` + assignment in both branches (NOT a redeclaration); keep the
    `// copy to value` comment on BOTH `cfg = *...` lines to flag the shallow-copy semantics.
  - VERIFY: `go build ./pkg/stagecoach/` compiles; `opts.Config == nil` path is textually the old code.

Task 3: ADD a focused unit test (pkg/stagecoach/stagecoach_test.go)
  - NAME: TestGenerateCommit_InjectedConfig (or TestResolveConfig_InjectedConfig if resolveConfig
    is testable directly — it is unexported but the test is in package stagecoach, so direct call works).
  - PREFER testing resolveConfig DIRECTLY (in-package test can call unexported funcs):
      - Build a config.Config{Provider: "stub", Providers: map[string]map[string]any{"stub": {...}}}
        matching the stub manifest shape used by setupTestRepo's .stagecoach.toml (command=prompt_delivery
        stdin, output raw, strip_code_fence true). Use stubtest.Build(t) for the command path.
      - Call resolveConfig(ctx, Options{Config: &injected}).
      - ASSERT: returned cfg.Provider == "stub" AND returned cfg.Providers["stub"] != nil AND err == nil.
      - ASSERT (proves Load skipped): run in a temp dir with NO .stagecoach.toml and NO STAGECOACH_CONFIG
        env (os.Unsetenv) — if Load ran, it would find no "stub" provider (built-ins only) and the
        cfg.Providers map would be empty / provider would be "". The injected map surviving proves Load
        was skipped.
  - ALTERNATIVELY (if exercising GenerateCommit end-to-end is preferred): use the existing
    setupTestRepo machinery but pass Options{Config: <a config whose Providers registers the stub>}
    instead of relying on the repo-local .stagecoach.toml, and run in a repo with NO .stagecoach.toml.
  - FOLLOW pattern: existing stagecoach_test.go helpers (initRepo, writeFile, stageFile, stubtest.Build).
  - COVERAGE: (a) injected Config is used as-is (provider resolved), (b) Options overrides still apply
    on top of the injected config (e.g. Options{Config: &cfg, Provider: "stub"} where cfg.Provider="").
  - DO NOT: assert on the §19 notice count (that is S3, requires CLI stderr capture of noticeOut).
  - DO NOT: add CLI / runDefault tests (S2/S3 own the cross-package seam).
  - PLACEMENT: alongside the existing TestGenerateCommit_* functions in stagecoach_test.go.

Task 4: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # existing tests unchanged + green; new test green
  - FIX-FORWARD: if any gate fails, read the message, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === resolveConfig AFTER S1 (the complete function) ===
// resolveConfig resolves config for a GenerateCommit call. When opts.Config is non-nil (in-module CLI
// path), it is shallow-copied and config.Load is skipped entirely — eliminating the double-load that
// dropped --config (Issue 1) and double-printed the §19 repo-local notice (Issue 5). When nil
// (standalone path), behavior is unchanged. Options overrides apply in BOTH branches (highest
// precedence). See architecture/decisions.md D1.
func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("getwd: %w", err)
	}

	var cfg config.Config
	if opts.Config != nil {
		cfg = *opts.Config // shallow copy; caller already resolved config — skip config.Load (D1)
	} else {
		cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
		if err != nil {
			return config.Config{}, "", fmt.Errorf("load config: %w", err)
		}
		cfg = *cfgPtr // copy to value
	}

	// Apply caller overrides (highest precedence — explicit intent wins over file/env/git-config).
	// Redundant for the CLI path (PersistentPreRunE already folded Layer-7 in) but mandatory for the
	// standalone-library precedence contract.
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		cfg.Model = opts.Model
	}
	if opts.Timeout != 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.VerboseOn {
		cfg.Verbose = true
	}

	return cfg, repoDir, nil
}
```

```go
// === Options AFTER S1 (new field appended last) ===
type Options struct {
	Provider    string
	Model       string
	SystemExtra string
	DryRun      bool
	Timeout     time.Duration
	Verbose     io.Writer
	VerboseOn   bool
	// Config optionally supplies an already-resolved configuration; when non-nil, config.Load is
	// skipped entirely (the caller — typically the in-module CLI — has already loaded config once).
	// Options overrides below still apply on top. nil ⇒ config.Load runs as before (standalone path).
	// Additive-only (PRD §14.1); external out-of-module callers leave this nil.
	Config *config.Config
}
```

### Integration Points

```yaml
PUBLIC API (pkg/stagecoach.Options):
  - field added: "Config *config.Config"   # nil-optional, ADDITIVE-ONLY
  - signature unchanged: "func GenerateCommit(ctx context.Context, opts Options) (Result, error)"
  - Result unchanged

INTERNAL (pkg/stagecoach.resolveConfig):
  - branch added: "if opts.Config != nil { cfg = *opts.Config } else { config.Load(...) }"
  - os.Getwd() repoDir derivation: preserved in both branches
  - Options override block: preserved, runs unconditionally in both branches

NO-TOUCH (explicitly):
  - internal/config/*        # Load, LoadOpts, Config, loadRepoLocalConfig, noticeOut — unchanged
  - internal/cmd/*           # runDefault wiring is S2; CLI regression tests are S3
  - GenerateCommit body      # delegates to resolveConfig as before; no change
  - docs/*, README.md        # contract: "DOCS: none"; per-issue doc sync rides with S2/S3 + M5 sweep

DOWNSTREAM HOOKS (informational — implemented by LATER subtasks, NOT S1):
  - S2: internal/cmd/default_action.go runDefault sets Options.Config = Config() (the loadedCfg)
  - S3: internal/cmd/default_action_test.go asserts --config honored end-to-end + §19 notice once
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                       # Expected: empty (reformat if it lists pkg/stagecoach/*.go)
go vet ./pkg/stagecoach/...       # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: Zero output/errors. If gofmt lists files, run `gofmt -w pkg/stagecoach/stagecoach.go`.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The new focused test for the injected-config path
go test -race -run 'TestGenerateCommit_InjectedConfig|TestResolveConfig_InjectedConfig' ./pkg/stagecoach/ -v

# Full pkg/stagecoach suite (existing tests must remain green on the nil path)
go test -race ./pkg/stagecoach/ -v

# Expected: new test PASSES (proves Load skipped — injected provider resolved with no on-disk config);
#           all existing tests PASS unchanged (they use Options{} with Config == nil).
```

### Level 3: Whole-Repository Regression (No Behavior Change on the nil Path)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages pass (config, git, generate, provider, cmd, …)
go vet ./...                     # Expected: exit 0

# Confirm GenerateCommit signature is byte-for-byte unchanged
grep -n "func GenerateCommit(ctx context.Context, opts Options) (Result, error)" pkg/stagecoach/stagecoach.go
# Expected: exactly one match, unchanged signature.

# Confirm the Options field is additive (Result struct untouched; no field removed/renamed)
grep -n "type Options struct" -A 12 pkg/stagecoach/stagecoach.go   # Config *config.Config present, others intact
grep -n "type Result struct" -A 8  pkg/stagecoach/stagecoach.go    # unchanged
```

### Level 4: Targeted Bug-Reproduction Check (manual smoke, optional for S1)

> S1 alone does NOT fix the user-visible bug end-to-end (that needs S2 wiring). This check only
> confirms the new field exists and the nil path is unchanged. The real `--config`-honored /
> notice-once assertions are S3's validation gates.

```bash
cd /home/dustin/projects/stagecoach

# Confirm the field is present and documented
grep -n "Config \*config.Config" pkg/stagecoach/stagecoach.go      # Expected: one match (the Options field)

# Confirm resolveConfig now guards config.Load behind opts.Config == nil
grep -n "if opts.Config != nil" pkg/stagecoach/stagecoach.go       # Expected: one match
grep -n "config.Load(ctx" pkg/stagecoach/stagecoach.go             # Expected: still one match, now in the else-branch
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (existing untouched + new test green).

### Feature Validation

- [ ] `Options` has `Config *config.Config` (additive; documented nil-optional / ADDITIVE-ONLY).
- [ ] `resolveConfig` skips `config.Load` when `opts.Config != nil` (Load is in the `else` branch).
- [ ] `os.Getwd()` repoDir derivation runs in both branches.
- [ ] Options override block runs unconditionally in both branches.
- [ ] New unit test proves the injected-config path resolves an injected provider with no on-disk
      config file (i.e., `config.Load` was provably skipped).
- [ ] `GenerateCommit` signature and `Result` struct unchanged.

### Scope Discipline Validation

- [ ] Did NOT edit `internal/cmd/*` (runDefault wiring is S2).
- [ ] Did NOT add CLI regression tests / §19-notice-count assertions (S3).
- [ ] Did NOT edit `internal/config/*` (the notice fix is a side effect of skipping Load, not an edit).
- [ ] Did NOT change `GenerateCommit`'s signature, body control flow, `buildDeps`, or `runPipeline`.
- [ ] Did NOT add docs (contract: "DOCS: none").
- [ ] Did NOT deep-copy the `Providers` map (plain `cfg = *opts.Config`, matching the Load path).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Field doc comment matches the style of existing Options fields (Verbose/VerboseOn).
- [ ] The `// shallow copy` comment flags the aliasing semantics on both `cfg = *...` lines.
- [ ] No new imports added (`config` already imported).
- [ ] Existing tests untouched (the nil path is byte-for-byte the prior behavior).

---

## Anti-Patterns to Avoid

- ❌ Don't add `ConfigPathOverride` to Options instead of `*config.Config` — that's Option (b),
  rejected by D1: it fixes Issue 1 but NOT Issue 5 (the second Load still fires the notice twice).
- ❌ Don't deep-copy / clone the `Providers` map — `cfg = *opts.Config` is a shallow value copy, the
  same pattern as the existing `cfg = *cfgPtr`. Deep-copying diverges and is scope creep; the map is
  never mutated downstream.
- ❌ Don't remove or gate the Options override block behind the `opts.Config != nil` branch — it must
  run in BOTH branches to preserve the standalone-library precedence contract (Options > everything).
- ❌ Don't move/drop the `os.Getwd()` repoDir derivation — `buildDeps(cfg, repoDir)` needs it.
- ❌ Don't change `GenerateCommit`'s signature or touch `Result` (API-stable; unnecessary).
- ❌ Don't wire `runDefault` here (S2) or add CLI regression tests (S3) — respect the subtask boundary.
- ❌ Don't edit `internal/config/file.go` to suppress the notice (band-aid; the notice double-print
  disappears naturally once Load is skipped in the injected path).
- ❌ Don't add docs — the contract says "DOCS: none"; doc sync rides with S2/S3 and the M5 sweep.
- ❌ Don't ignore a failing `go test -race` — fix root cause; the nil path must stay green.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a two-location edit in a single file with the exact current source quoted verbatim
and the exact target code given. The architecture decisions (D1) and the seam research
(`seam_config_handoff.md` §4/§9/§10 Option B) pre-resolved every design question — including the
rejection of the superficially-simpler `ConfigPathOverride` alternative (which would leave Issue 5
unfixed) and the shallow-copy safety (proven by the existing `cfg := *cfgPtr` precedent and the
absence of any downstream `cfg.Providers` mutation). The only residual uncertainty (not 10/10) is the
test-author's choice between testing `resolveConfig` directly (in-package, cleanest) vs. driving
`GenerateCommit` end-to-end — both are specified as acceptable, and either proves Load was skipped.
Downstream S2/S3 are cleanly separated and cannot be broken by S1 because the nil path is unchanged.
