---
name: "P1.M1.T1.S2 — Wire the CLI's loaded cfg through runDefault into Options"
description: |
  Bugfix (Issue 1 + Issue 5 — the CLI wiring half). Add `Config: cfg` to the `stagecoach.Options{...}`
  literal in `runDefault` (`internal/cmd/default_action.go`) so `GenerateCommit` consumes the
  CLI-already-loaded `*config.Config` (set by `PersistentPreRunE`, which honors `--config`) instead of
  re-loading config from scratch. This makes `resolveConfig` take its `opts.Config != nil` branch
  (added in S1, commit 13225e9), eliminating the second `config.Load` that (a) dropped `--config`
  (Issue 1: "unknown provider" for user-defined providers on the default action) and (b) double-printed
  the §19 repo-local provider notice (Issue 5). Also rewrite the now-stale "Options-as-flag-relay"
  comment and affirm in `docs/cli.md` that `--config` is honored by every command including the default
  action. No tests, no signature change, no `internal/config` edits (S3 owns the regression tests).
---

## Goal

**Feature Goal**: Close the second half of the CLI↔`pkg/stagecoach` config-handoff seam — the
**wiring** that S1's additive `Options.Config` field was built for. `runDefault` must hand the
CLI's already-resolved config (`cfg := Config()`) into `GenerateCommit` so the public API performs
**exactly one** `config.Load` (the one `PersistentPreRunE` already did, which honored `--config`),
not two.

**Deliverable** (three small edits, two files):
1. `internal/cmd/default_action.go` — add one field, `Config: cfg,`, to the `stagecoach.Options{...}`
   literal passed to `GenerateCommit` (lines ~147-154).
2. `internal/cmd/default_action.go` — rewrite the stale comment at lines ~130-134 that (falsely, after
   this change) claims "GenerateCommit re-loads config with Flags:nil".
3. `docs/cli.md` — append one affirming sentence to the `--config` prose (line 30) stating the flag is
   honored by all commands, including the default commit action. (No change to the line-20 table row or
   the line-97 flag↔env↔git-config map — both are already accurate.)

**Success Definition**:
- `stagecoach --config X.toml --provider <user-defined-in-X>` resolves the user-defined provider on the
  default commit action (Issue 1 closed — no more `unknown provider`).
- The §19 repo-local provider-redirect notice prints **exactly once** for a default-action run whose
  `.stagecoach.toml` sets `provider` (Issue 5 closed as a side effect).
- `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green (existing tests
  untouched — S2 adds none).
- `GenerateCommit` signature and `Result` unchanged; `pkg/stagecoach` and `internal/config` untouched.

## User Persona

**Target User**: A Stagecoach user who configures the tool via a custom config file
(`stagecoach --config ./my.toml`) declaring a user-defined provider under `[provider.<name>]` and then
runs the default commit action (`stagecoach --provider <name>`).

**Use Case**: "I keep my team's provider definitions in a checked-in `stagecoach.team.toml`; I want
`stagecoach --config stagecoach.team.toml --provider team-llm` to just work without the env-var trick."

**User Journey**:
1. `git add` some changes.
2. `stagecoach --config stagecoach.team.toml --provider team-llm`.
3. Expect: `↳ Generating with team-llm…` → `[<sha>] <subject>` (exit 0).
   Today (bug): `↳ Generating with team-llm…` → `stagecoach: unknown provider "team-llm"` (exit 1).

**Pain Points Addressed**: `--config` worked only for the `providers`/`config` subcommands; the default
action silently discarded it (the only workaround was the `STAGECOACH_CONFIG` env var, which takes a
different code path). Plus the confusing double §19 notice.

## Why

- **Completes the S1→S2 pair.** S1 (`pkg/stagecoach`) added the field + the skip-Load branch but could
  not by itself fix the user-visible bug — nothing sets the field. S2 is the single call-site that sets
  it. Per `architecture/decisions.md` D1 and `seam_config_handoff.md` §10 (Option B), the resolved-config
  injection is the chosen fix; S2 is its CLI consumer.
- **Fixes Issue 1 and Issue 5 at once.** One wiring change removes the second `config.Load` inside
  `resolveConfig`. That second Load was the sole root cause of both bugs (per
  `seam_config_handoff.md` §4: it omitted `ConfigPathOverride` → dropped `--config`; and it re-ran
  Layer-3 `loadRepoLocalConfig` → second §19 notice).
- **Zero risk to the standalone library path.** `Options.Config == nil` behavior in `resolveConfig` is
  byte-for-byte unchanged (S1 guaranteed this). Out-of-module library consumers who never set `Config`
  see no difference.
- **Precedence contract preserved.** `Options` overrides > Layer-7 flags > env > git-config >
  repo-local > global > defaults. The CLI's first Load already folded Layer-7 in; passing that cfg
  through keeps every layer intact in one place. The remaining `Provider/Model/Timeout/VerboseOn`
  fields on the literal re-assert Options-as-highest-precedence (redundant for the CLI, mandatory for
  the library contract documented on `Options`).

## What

A surgical, additive change to **one CLI source file** plus a one-sentence doc affirmation:

1. **`runDefault`** (`internal/cmd/default_action.go`) adds `Config: cfg` to the existing
   `stagecoach.Options{...}` literal. `cfg` is the `*config.Config` returned by `Config()` at the top of
   `runDefault` (line ~56); its type (`*config.Config`) matches `Options.Config` exactly — no address-of,
   no dereference.
2. The comment block immediately above the literal is rewritten to describe the new single-Load path
   (it currently documents the OLD double-Load workaround, which would become a lie after the wiring).
3. **`docs/cli.md`** gains one sentence affirming `--config` honors the default action.

No changes to: `GenerateCommit`/`Result`, `pkg/stagecoach/*` (S1 already done), `internal/config/*`
(the notice fix is a *side effect* of the skipped Load, not an edit), any test file (S3 owns the
`--config`-honored + notice-once regression tests), `Makefile`, `go.mod`.

### Success Criteria

- [ ] `internal/cmd/default_action.go`'s `stagecoach.Options{...}` literal includes `Config: cfg,`.
- [ ] The comment above that literal no longer claims `GenerateCommit` re-loads config; it states the
      CLI passes its already-loaded config via `Options.Config` (one Load total; `--config` honored;
      §19 notice once).
- [ ] `docs/cli.md` line-30 prose affirms `--config` is honored by all commands incl. the default action.
- [ ] `stagecoach --config <file> --provider <user-defined>` succeeds on the default action (manual repro).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No edits to `pkg/stagecoach/*`, `internal/config/*`, `GenerateCommit` signature, or any `*_test.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current `runDefault` call site and the exact comment
to rewrite, states the precise one-field addition (with the type match proven inline), references the
S1 change that already shipped the receiving field/branch (with commit + grep receipts), proves no
existing test breaks, and gives executable validation commands. The architecture docs are referenced
by section with conclusions distilled.

### Documentation & References

```yaml
# MUST READ — the binding architectural decisions (do not re-litigate)
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: "D1 chose Option (a)/(B): pass the already-resolved *config.Config into GenerateCommit via an additive Options field; skip config.Load when provided. S1 added the field + branch; S2 is the CLI call-site that sets it."
  critical: "D1 REJECTS forwarding --config as a ConfigPathOverride string (Option b): that would fix Issue 1 but NOT Issue 5 (the second Load still fires the notice twice). S2 must set Options.Config = cfg, NOT a new Options.ConfigPathOverride."

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_handoff.md
  why: "§2 quotes the exact current Options literal + the stale comment (the edit locus). §3 proves PersistentPreRunE is the Load that honors --config (flagConfig → ConfigPathOverride). §4 is the smoking-gun second Load inside resolveConfig (now guarded by S1). §9 is the data-flow diagram (notice #1 = PersistentPreRunE, notice #2 = resolveConfig → disappears once S2 sets Config). §10 Option B = S1+S2 exactly."
  section: "§2 (call site + stale comment), §3 (PersistentPreRunE honors --config), §4 (resolveConfig bug locus), §9 (data-flow), §10 (fix surfaces)"
  critical: "§2 is the verbatim text to edit. §9 shows why ONE field addition removes BOTH the dropped --config AND the duplicate notice (single root cause, single-point fix)."

# The predecessor PRP — S1 shipped the receiving surface; S2 consumes it
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M1T1S1/PRP.md
  why: "Documents the Options.Config field contract (nil-optional, ADDITIVE-ONLY) and the resolveConfig branch S1 added. S2 must conform to that contract: set Config = a *config.Config, rely on resolveConfig's opts.Config != nil branch, and leave the Options override block intact."
  critical: "S1 explicitly defers the runDefault wiring to S2 (its Integration Points: 'S2: internal/cmd/default_action.go runDefault sets Options.Config = Config()'). S2 is that deferred step."

# Source files under edit / cross-reference
- file: internal/cmd/default_action.go
  why: "THE edit target. runDefault resolves cfg := Config() at ~line 56 and builds the Options literal at ~lines 147-154; the stale 'Options-as-flag-relay' comment sits at ~lines 130-134."
  pattern: "Add `Config: cfg,` as one more field in the existing stagecoach.Options{...} composite literal. Rewrite the comment block above it. Do NOT touch runDefault's control flow, the auto-stage state machine, handleGenError, or the print helpers."
  gotcha: "cfg is *config.Config (Config() returns a pointer, root.go:108). Options.Config is *config.Config. Write `Config: cfg` — NOT `Config: &cfg` (cfg is already a pointer) and NOT `Config: *cfg` (would pass a value where a pointer is required; won't compile)."

- file: pkg/stagecoach/stagecoach.go
  why: "READ-ONLY reference (edited by S1, NOT by S2). Confirms Options.Config exists and resolveConfig takes the opts.Config != nil branch (shallow-copies cfg, skips config.Load, still applies Options overrides, still derives repoDir via os.Getwd)."
  pattern: "resolveConfig: `if opts.Config != nil { cfg = *opts.Config } else { config.Load(...) }` then the override block runs unconditionally. S2 relies on this unchanged."
  gotcha: "Do NOT edit this file in S2. If you feel tempted to change resolveConfig or Options, STOP — that is S1's (completed) surface or scope creep."

- file: internal/cmd/root.go
  why: "READ-ONLY reference. Defines flagConfig (line ~32), PersistentPreRunE (lines ~62-80) that passes flagConfig as ConfigPathOverride into the FIRST (correct) Load, loadedCfg (line ~52), and Config() (line ~108) returning *config.Config."
  pattern: "PersistentPreRunE: config.Load(ctx, LoadOpts{ConfigPathOverride: flagConfig, RepoDir, Flags: cmd.Flags()}) → loadedCfg = cfg. runDefault reads loadedCfg via Config(). S2 does not touch root.go."

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M1T1S2/research/s2_wiring_notes.md
  why: "Distilled S2 findings: the git/grep receipts proving S1-done/S2-pending, the exact edit locus, the stale-comment rewrite rationale, the no-existing-test-break proof, and the precise docs touch-points (lines 20/30/97)."

# Docs under edit
- file: docs/cli.md
  why: "THE docs edit target (Mode A). Line 20 = the --config table row (accurate, NO change). Line 30 = the --config prose (append one affirming sentence). Line 97 = the --config map row (accurate, NO change)."
  pattern: "Append to the line-30 paragraph a single sentence stating --config is honored by every command including the default commit action. Do NOT rewrite the row/map; do NOT imply it was ever subcommand-only."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/cmd/
│   ├── default_action.go     # EDIT TARGET (Options literal +1 field; comment rewrite)
│   ├── default_action_test.go# NOT edited in S2 (S3 adds the regression tests)
│   ├── root.go               # READ-ONLY (flagConfig, PersistentPreRunE, Config())
│   └── ...
├── pkg/stagecoach/
│   └── stagecoach.go          # READ-ONLY (Options.Config + resolveConfig branch — done in S1)
└── docs/
    └── cli.md                # EDIT TARGET (+1 affirming sentence, line 30)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
├── internal/cmd/
│   └── default_action.go     # MODIFIED: Options literal +`Config: cfg`; comment rewritten
└── docs/
    └── cli.md                # MODIFIED: +1 sentence in the --config prose
# (no other files touched)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/cmd/default_action.go` | MODIFY | Add `Config: cfg,` to the `Options` literal; rewrite the stale comment above it. |
| `docs/cli.md` | MODIFY | Affirm `--config` honors the default action (1 sentence). |

**Explicitly NOT touched in S2** (later subtasks / out of scope):
`pkg/stagecoach/stagecoach.go` (S1, done), `internal/config/*` (notice fix is a side effect),
`internal/cmd/default_action_test.go` (S3 regression tests), `GenerateCommit`/`Result`,
`Makefile`, `go.mod`, `PRD.md`, `tasks.json`, `prd_snapshot.md`.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — the pointer match. `cfg` in runDefault is `*config.Config` (Config() at root.go:108
// returns loadedCfg, which is *config.Config). `Options.Config` is `*config.Config` (S1 added it).
// Therefore write EXACTLY:
//     Config: cfg,
// Do NOT write `Config: &cfg` (cfg is already a pointer — &cfg would be **config.Config, won't compile),
// and do NOT write `Config: *cfg` (dereference → passes a value, field expects a pointer, won't compile).

// CRITICAL — keep the OTHER Options fields. Do NOT "simplify" by deleting Provider/Model/Timeout/
// VerboseOn in favor of only Config. They are redundant for the CLI path (cfg already carries Layer-7
// flag values) but are REQUIRED to honor the documented Options>everything precedence contract on the
// standalone-library path (Options.Config == nil). They also make the explicit-intent override explicit.
// The override block in resolveConfig runs unconditionally on both branches (S1 guarantee).

// CRITICAL — this change REMOVES a config.Load call, it does not add one. After S2:
//   PersistentPreRunE runs config.Load ONCE (honors --config). runDefault→GenerateCommit→resolveConfig
//   takes the opts.Config != nil branch and does NOT call config.Load. Total Loads for a default
//   action run = 1 (was 2). That is why Issue 5 (double §19 notice) disappears for free.

// GOTCHA — shallow copy is intentional and safe. resolveConfig does `cfg = *opts.Config` (shallow);
// the Config.Providers map header is shared with loadedCfg. This is SAFE: neither resolveConfig nor
// buildDeps mutates cfg.Providers (buildDeps reads it via provider.DecodeUserOverrides, re-encoding only).
// It is identical to the existing `cfg = *cfgPtr` pattern on the Load path. Do NOT deep-copy.

// GOTCHA — rewrite the comment, don't delete it. The block at default_action.go:130-134 currently
// documents the "Options-as-flag-relay" WORKAROUND for the double-Load. After S2 that workaround's
// premise ("GenerateCommit re-loads config with Flags:nil") is FALSE. Replace the comment with an
// accurate description of the single-Load injection path so the code is not misleading.

// GOTCHA — the docs edit is an AFFIRMATION, not a rewrite. The current docs/cli.md never says --config
// is subcommand-only; it is simply silent on which commands honor it. Add one sentence; do NOT reword
// the table row (line 20) or the map row (line 97) — both are already correct.

// GOTCHA — leave provider pre-validation as-is. runDefault already validates the provider against
// cfg.Providers (default_action.go ~137-143) BEFORE calling GenerateCommit. That validation already
// honors --config (it reads cfg, the first Load). The bug is purely the SECOND Load inside
// GenerateCommit. Do NOT move or duplicate the pre-check.
```

## Implementation Blueprint

### Data models and structure

No new data models. The change consumes the S1-shipped `Options.Config` field. Relevant types
(verbatim from source):

```go
// pkg/stagecoach/stagecoach.go — OPTIONS (already has Config; S1 shipped this — S2 does NOT edit)
type Options struct {
	Provider    string
	Model       string
	SystemExtra string
	DryRun      bool
	Timeout     time.Duration
	Verbose     io.Writer
	VerboseOn   bool
	// Config optionally supplies an already-resolved configuration; when non-nil, config.Load is
	// skipped entirely … nil ⇒ config.Load runs as before (standalone path). Additive-only (PRD §14.1).
	Config *config.Config
}

// internal/cmd/root.go — the value S2 wires in (already in scope in runDefault)
func Config() *config.Config { return loadedCfg }   // root.go:108

// internal/cmd/default_action.go — cfg is *config.Config, in scope at the call site (line ~56):
//   cfg := Config()
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: WIRE Options.Config in runDefault (internal/cmd/default_action.go)
  - LOCATE: the stagecoach.GenerateCommit(ctx, stagecoach.Options{...}) call (~lines 147-154).
  - ADD one field to the composite literal (placement: alongside the other fields; field order in a
    literal is not significant, but group it logically — e.g. first, since it is now the primary
    config carrier):
        Config:  cfg,
    (use the same gofmt-friendly column alignment as the surrounding `Provider:  cfg.Provider,` lines.)
  - TYPE CHECK: cfg is *config.Config (from `cfg := Config()` at ~line 56); Options.Config is
    *config.Config. Direct assignment — no &, no *.
  - PRESERVE every other field verbatim: Provider, Model, Timeout, DryRun, Verbose, VerboseOn.
  - DO NOT: touch runDefault's control flow, the auto-stage block, handleGenError, print helpers, the
    provider pre-validation block, or the progress label.

Task 2: REWRITE the stale comment above the literal (internal/cmd/default_action.go ~130-134)
  - LOCATE: the comment block immediately above the GenerateCommit call, currently:
        // §3: re-apply the CLI-resolved provider/model/timeout (Layer-7 flags already applied by
        // PersistentPreRunE) as Options — GenerateCommit re-loads config with Flags:nil, so opts is how the
        // CLI flags take effect (opts override is highest precedence in resolveConfig).
  - REPLACE with an accurate description, e.g.:
        // §3: hand the CLI-resolved config (cfg, loaded ONCE by PersistentPreRunE — which honors
        // --config via flagConfig→ConfigPathOverride) into GenerateCommit via Options.Config.
        // resolveConfig then SKIPS its own config.Load (S1's opts.Config != nil branch): --config is
        // honored on the default action (Issue 1) and the §19 repo-local notice prints once (Issue 5).
        // Provider/Model/Timeout/VerboseOn below re-assert the Options>everything precedence (redundant
        // for the CLI path, mandatory for the standalone-library Options.Config==nil contract).
    (Exact wording is the implementer's; it MUST (a) state config is passed via Options.Config,
     (b) state GenerateCommit does NOT re-load, (c) reference --config being honored + the single notice.)
  - GOTCHA: this is a doc-of-code change; behavior is unaffected. Keep it concise (≤ ~6 lines).

Task 3: AFFIRM --config in docs/cli.md (Mode A)
  - LOCATE: the paragraph at line 30 (immediately after the Global flags table):
        The `--config` flag is a path override for config-file discovery — it is not itself a `Config` field.
        The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config analogs.
  - APPEND one sentence affirming scope, e.g.:
        `--config` is honored by every command — including the default commit action, so a user-defined
        provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` on
        `stagecoach` directly (not just the `providers`/`config` subcommands).
  - DO NOT: edit the line-20 table row (`Path to a config file, overrides discovery` — accurate) or the
    line-97 map row (`--config | STAGECOACH_CONFIG | —` — accurate). The current wording does NOT imply
    subcommand-only, so this is a positive affirmation, not a rewrite.

Task 4: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # existing tests unchanged + green (S2 adds none)
  - RUN: the manual Issue-1 reproduction (Validation Loop Level 3) — provider defined ONLY in a
    --config file resolves on the default action; §19 notice prints exactly once.
  - FIX-FORWARD: if any gate fails, read the message, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === runDefault call site AFTER S2 (the complete GenerateCommit invocation) ===
	res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{
		Config:   cfg,        // ← NEW (S2): CLI-resolved *config.Config; resolveConfig skips its config.Load
		Provider:  cfg.Provider,
		Model:     cfg.Model,
		Timeout:   cfg.Timeout,
		DryRun:    flagDryRun,
		Verbose:   stderr,
		VerboseOn: cfg.Verbose,
	})
	if err != nil {
		return handleGenError(stderr, err) // §4: rescue/CAS/timeout/nothing/generic matrix
	}
```

```go
// === resolveConfig (pkg/stagecoach/stagecoach.go) — UNCHANGED by S2; this is what makes the wiring work ===
// (quoted so the implementer sees the receiving branch S1 already shipped)
func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("getwd: %w", err)
	}
	var cfg config.Config
	if opts.Config != nil {
		cfg = *opts.Config // ← S2's Config: cfg lands here; config.Load is NOT called
	} else {
		cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
		if err != nil {
			return config.Config{}, "", fmt.Errorf("load config: %w", err)
		}
		cfg = *cfgPtr
	}
	// overrides run unconditionally (Provider/Model/Timeout/Verbose) …
	return cfg, repoDir, nil
}
```

### Integration Points

```yaml
CLI → PUBLIC API (internal/cmd/default_action.go → pkg/stagecoach):
  - literal field added: "Config: cfg"   # cfg = Config() = loadedCfg (*config.Config)
  - effect: resolveConfig takes opts.Config != nil branch; 0 config.Load calls inside GenerateCommit

DOCUMENTATION (docs/cli.md):
  - line 30 prose: +1 sentence affirming --config honors the default action
  - line 20 table row / line 97 map row: UNCHANGED (already accurate)

NO-TOUCH (explicitly):
  - pkg/stagecoach/stagecoach.go   # Options.Config + resolveConfig branch = S1 (done)
  - internal/config/*            # Load/LoadOpts/Config/loadRepoLocalConfig/noticeOut — unchanged
  - internal/cmd/default_action_test.go  # S3 owns the regression tests
  - GenerateCommit signature / Result / buildDeps / runPipeline
  - root.go (flagConfig, PersistentPreRunE, Config())
  - Makefile, go.mod, README.md  # README + doc sweep ride with M5

DOWNSTREAM HOOKS (informational — implemented by LATER subtasks, NOT S2):
  - S3: internal/cmd/default_action_test.go asserts --config honored end-to-end + §19 notice exactly once
  - M5: README.md + docs/* overview sweep for changeset coherence
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                  # Expected: empty (if it lists internal/cmd/default_action.go, run gofmt -w on it)
go vet ./internal/cmd/...   # Expected: exit 0
go build ./...              # Expected: exit 0

# Confirm the wiring landed (one match each)
grep -n "Config:  cfg\|Config: cfg," internal/cmd/default_action.go   # Expected: one match
# Confirm pkg/stagecoach was NOT touched in S2 (git diff scoped to the two edit files)
git diff --name-only          # Expected: only internal/cmd/default_action.go and docs/cli.md
# Expected: Zero output/errors. Fix before proceeding.
```

### Level 2: Unit Tests (Component Validation — S2 adds none; existing must stay green)

```bash
cd /home/dustin/projects/stagecoach

# The cross-package seam (PersistentPreRunE → runDefault → GenerateCommit → resolveConfig)
go test -race ./internal/cmd/ -v
# Expected: ALL existing default_action_test.go tests PASS (the double-Load removal does not break them —
#           no CLI test asserts on the §19 notice count; verified by grep in research/s2_wiring_notes.md).

# The public API surface (S1's injected-config branch + the unchanged nil path)
go test -race ./pkg/stagecoach/ -v
# Expected: ALL PASS unchanged.

# Full repo
go test -race ./...          # Expected: ALL packages green.
```

### Level 3: Integration / Targeted Bug Reproduction (System Validation)

> Build the binary once (`go build -o bin/stagecoach ./cmd/stagecoach`). Drive the REAL default action.
> `bin/stubagent` is built by the test suite via `internal/stubtest`; for a manual repro, point a
> provider at any command that emits a valid commit subject to stdout (e.g. a tiny shell script), or
> reuse an installed built-in if present.

```bash
cd /home/dustin/projects/stagecoach
go build -o bin/stagecoach ./cmd/stagecoach

# --- Repro fixture: a config file declaring a USER-DEFINED provider pointing at a stub command ---
REPO=$(mktemp -d) && cd "$REPO" && git init -q && git config user.email t@t && git config user.name t

# A stub agent: prints a valid commit subject to stdout, ignores stdin.
cat > agent.sh <<'EOF'
#!/usr/bin/env sh
cat >/dev/null   # drain stdin
printf 'feat: wired config honored\n'
EOF
chmod +x agent.sh

# The --config file: defines ONLY [provider.team] (no built-in).
cat > team.toml <<EOF
[provider.team]
command = "$REPO/agent.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
EOF

echo "hello" > a.txt && git add a.txt

# === ISSUE 1 FIX: --config honored on the DEFAULT action (was: unknown provider "team") ===
stagecoach --config team.toml --provider team
# Expected: a new commit; "[<sha>] feat: wired config honored" on stdout; exit 0.
#   (Before S2: "↳ Generating with team…" then "stagecoach: unknown provider \"team\"" exit 1.)

# === ISSUE 5 FIX: §19 notice printed EXACTLY ONCE ===
# Put a provider redirect in the REPO-LOCAL config and count the notice on stderr.
printf 'provider = "team"\n' > .stagecoach.toml
echo "more" >> a.txt && git add a.txt
stagecoach --config team.toml --provider team 2>err.log
grep -c "repo-local config (.stagecoach.toml) sets provider" err.log   # Expected: 1  (was 2 before S2)

# Cleanup
cd /home/dustin/projects/stagecoach && rm -rf "$REPO"
```

### Level 4: Regression & Contract Checks

```bash
cd /home/dustin/projects/stagecoach

# STAGECOACH_CONFIG env-var path must STILL work (it was never broken; confirm no regression).
# (Reuse the team.toml fixture from Level 3; run from inside $REPO with the repo-local config removed.)
rm -f .stagecoach.toml
echo "x" >> a.txt && git add a.txt
STAGECOACH_CONFIG=team.toml stagecoach --provider team --dry-run   # Expected: prints subject; exit 0.

# The providers subcommand path must STILL work (single Load via PersistentPreRunE; no GenerateCommit).
stagecoach --config team.toml providers list   # Expected: lists "team" as detected; exit 0.

# Confirm GenerateCommit signature is byte-for-byte unchanged.
grep -n "func GenerateCommit(ctx context.Context, opts Options) (Result, error)" pkg/stagecoach/stagecoach.go
# Expected: exactly one match, unchanged.

# Confirm pkg/stagecoach was NOT modified by S2.
git diff --stat pkg/stagecoach/   # Expected: empty.

# Confirm the docs affirmation landed and the row/map are untouched.
grep -n "honored by every command\|default commit action" docs/cli.md   # Expected: ≥1 match (new sentence).
grep -n "Path to a config file, overrides discovery" docs/cli.md         # Expected: 1 match, unchanged (line 20).
grep -n '^| `--config` | `STAGECOACH_CONFIG` | — |$' docs/cli.md          # Expected: 1 match, unchanged (line 97).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (existing tests untouched; S2 adds none).

### Feature Validation

- [ ] `runDefault`'s `stagecoach.Options{...}` literal contains `Config: cfg,` (grep confirms).
- [ ] The comment above the literal accurately describes the single-Load injection path (no false
      "re-loads config with Flags:nil" claim).
- [ ] Manual Issue-1 repro passes: a user-defined provider declared ONLY in a `--config` file resolves
      on the default action (exit 0, commit created).
- [ ] Manual Issue-5 repro passes: the §19 repo-local notice prints exactly once (`grep -c` == 1).
- [ ] `STAGECOACH_CONFIG` env-var path still works (no regression).
- [ ] `providers list --config` still works (no regression).

### Scope Discipline Validation

- [ ] Did NOT edit `pkg/stagecoach/*` (S1's surface — already done).
- [ ] Did NOT edit `internal/config/*` (the notice fix is a side effect of the skipped Load, not an edit).
- [ ] Did NOT add/modify any `*_test.go` (S3 owns the regression tests).
- [ ] Did NOT change `GenerateCommit`'s signature, `Result`, `buildDeps`, `runPipeline`, or `resolveConfig`.
- [ ] Did NOT edit `root.go`, `Makefile`, `go.mod`, or `README.md`.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research/).
- [ ] Did NOT delete the Provider/Model/Timeout/VerboseOn Options fields (precedence contract).
- [ ] Did NOT forward `--config` as a new `Options.ConfigPathOverride` (rejected by D1 — leaves Issue 5 unfixed).

### Code Quality & Docs Validation

- [ ] `Config: cfg` uses the correct pointer (cfg is already `*config.Config`; no `&`/`*`).
- [ ] The rewritten comment is concise and accurate (single Load; `--config` honored; notice once).
- [ ] `docs/cli.md` change is a one-sentence affirmation; table row + map row untouched.
- [ ] `git diff --name-only` shows ONLY `internal/cmd/default_action.go` and `docs/cli.md`.

---

## Anti-Patterns to Avoid

- ❌ Don't forward `--config` as `Options.ConfigPathOverride` instead of `Options.Config` — that's Option
  (b), rejected by `decisions.md` D1: it fixes Issue 1 but NOT Issue 5 (the second `config.Load` still
  runs and re-fires the §19 notice). Set `Config: cfg`.
- ❌ Don't write `Config: &cfg` or `Config: *cfg` — `cfg` is already `*config.Config` (`Config()` returns
  a pointer). `&cfg` is `**config.Config`; `*cfg` is a value. Both fail to compile. Use `Config: cfg`.
- ❌ Don't delete the `Provider/Model/Timeout/VerboseOn` fields from the literal to "simplify" — they
  enforce the documented Options>everything precedence on the standalone-library path and are required by
  the `resolveConfig` override contract (which runs unconditionally on both branches).
- ❌ Don't edit `pkg/stagecoach/stagecoach.go` or `resolveConfig` — that surface is S1's (done). The fix is
  purely at the CLI call site.
- ❌ Don't edit `internal/config/file.go` to suppress the notice (band-aid). The double-notice disappears
  naturally once `GenerateCommit` stops calling `config.Load`.
- ❌ Don't add CLI regression tests / notice-count assertions here — that is S3's explicit scope. S2 is
  wiring + comment + docs only.
- ❌ Don't leave the stale "GenerateCommit re-loads config with Flags:nil" comment in place — after the
  wiring it is false and will mislead future readers. Rewrite it.
- ❌ Don't rewrite the `docs/cli.md` `--config` table row or map row — they are already accurate. The edit
  is a one-sentence affirmation in the prose paragraph (line 30) only.
- ❌ Don't move/duplicate the provider pre-validation block in `runDefault` — it already honors `--config`
  (it reads `cfg`). The bug is solely the second Load inside `GenerateCommit`.
- ❌ Don't deep-copy `cfg.Providers` or otherwise mutate the passed config — `resolveConfig` shallow-copies
  by value (`cfg = *opts.Config`), matching the existing Load path; downstream code never writes the map.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: This is a one-field addition to an existing composite literal, with the exact current source
quoted verbatim, the exact target line given, the pointer-type match proven (cfg is already
`*config.Config`), and the receiving branch (S1, commit `13225e9`) already shipped and grep-confirmed.
The architecture decisions (D1) pre-resolved the design (rejected the superficially-simpler
`ConfigPathOverride` route, which would leave Issue 5 unfixed). The no-existing-test-break property is
proven by grep (no CLI test asserts on the §19 notice). The only companion edits (comment rewrite, one
doc sentence) are mechanical and fully specified. S3 (the regression tests) is cleanly downstream and
cannot be broken by S2. No external research is required — this is pure in-codebase wiring against
already-quarried architecture docs.
