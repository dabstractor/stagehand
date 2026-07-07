---
name: "P1.M1.T1.S1 — Config struct fields + Defaults + fileConfig/materialize/overlay (NoVerify + HookTimeout)"
description: |
  Config scaffolding (FR-V5/V6 hook execution, step 1 of 2). Add `NoVerify bool` (FR-V5, mirrors Push:
  only-true-propagates) and `HookTimeout time.Duration` (FR-V6, mirrors Timeout: duration-zero guard +
  string parse in loadTOML) to Config + Defaults + fileGeneration + materialize + overlay. NoVerify:
  config.go field + Defaults(false), fileGeneration.NoVerify bool, materialize `if g.NoVerify {c.NoVerify=true}`,
  overlay `if src.NoVerify {dst.NoVerify=true}`. HookTimeout: config.go field + Defaults(10*time.Minute),
  fileGeneration.HookTimeout string, loadTOML parse alongside Timeout, materialize param + `c.HookTimeout=ht`,
  overlay `if src.HookTimeout != 0 {dst.HookTimeout=src.HookTimeout}`. Env/flag layers land in S2. No behavior
  yet (unread until M3). Mode A: docs/configuration.md hook_timeout row.
---

## Goal

**Feature Goal**: Land the config-layer plumbing for the §9.25 hook-execution feature's two knobs:
`NoVerify` (FR-V5, the `--no-verify` bypass) and `HookTimeout` (FR-V6, per-hook timeout). Both resolve
through the standard 5-layer precedence (defaults → file → git-config → env → flag); S1 covers layers 1-4
(config struct, Defaults, fileGeneration, materialize, overlay); S2 adds env + flag + git-config.

**Deliverable** (6 production edits across 2 files + 1 doc file):
1. `internal/config/config.go`: + `NoVerify bool \`toml:"no_verify"\`` + `HookTimeout time.Duration \`toml:"hook_timeout"\`` + Defaults (`NoVerify: false`, `HookTimeout: 10 * time.Minute`).
2. `internal/config/file.go`: + `fileGeneration.NoVerify bool` + `fileGeneration.HookTimeout string`; loadTOML HookTimeout parse; materialize hookTimeout param + NoVerify/HookTimeout copy; overlay NoVerify/HookTimeout copy.
3. `docs/configuration.md`: + `hook_timeout` row in [generation] table + template block; note `no_verify`.

**Success Definition**: `Config.NoVerify` and `Config.HookTimeout` resolve correctly from a `[generation]` config file through materialize + overlay; `Defaults()` seeds them; the fields are dead (unconsumed) until M3; `go build/vet/gofmt` clean; `go test -race ./...` green.

## Why

- **First step of the hook-execution config chain.** §9.25 FR-V5/V6 need two config knobs. S1 is the
  struct/defaults/file-layer foundation; S2 adds env/flag/git-config. M3 wires the consumers.
- **Mirrors two proven patterns exactly.** NoVerify mirrors Push (bool, only-true-propagates); HookTimeout
  mirrors Timeout (Duration, string-in-file, parse-in-loadTOML, duration-zero guard). The arch
  `codebase_reality.md §2` specifies these templates byte-for-byte.
- **Pure scaffolding = lowest risk.** Nothing reads the fields yet (unread until M3's RunCommitHooks).
  Existing tests are field-specific (not exhaustive DeepEqual), so they stay green.

## What

Two new config fields with their file-layer plumbing, mirroring established patterns exactly. No caller
change, no behavior change, no env/flag wiring (S2).

### Success Criteria

- [ ] `Config.NoVerify bool \`toml:"no_verify"\`` + `Config.HookTimeout time.Duration \`toml:"hook_timeout"\``.
- [ ] `Defaults()`: `NoVerify: false` + `HookTimeout: 10 * time.Minute`.
- [ ] `fileGeneration.NoVerify bool \`toml:"no_verify"\`` + `fileGeneration.HookTimeout string \`toml:"hook_timeout"\``.
- [ ] `loadTOML`: parses `fc.Generation.HookTimeout` (duration string) alongside the existing timeout parse.
- [ ] `materialize`: receives `hookTimeout time.Duration` param; sets `c.HookTimeout = hookTimeout`; copies NoVerify via `if g.NoVerify { c.NoVerify = true }`.
- [ ] `overlay`: `if src.NoVerify { dst.NoVerify = true }` + `if src.HookTimeout != 0 { dst.HookTimeout = src.HookTimeout }`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `docs/configuration.md` has the `hook_timeout` row + template entry.

## All Needed Context

### Documentation & References

```yaml
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  why: "§2 is the authoritative template source. 'NoVerify → mirror Push (5-layer bool)' specifies every line byte-for-byte: config.go field + Defaults(false) + fileGeneration.NoVerify bool + materialize `if g.NoVerify {c.NoVerify=true}` + overlay `if src.NoVerify {dst.NoVerify=true}`. 'HookTimeout → mirror Timeout (5-layer duration)' specifies: config.go time.Duration + Defaults(10*time.Minute) + fileGeneration.HookTimeout string + loadTOML parse + materialize + overlay `!= 0`. Also notes HookTimeout is file+default only (no env/flag/git-config)."
  critical: "§2 confirms: NoVerify's only-true-propagates limitation is ACCEPTED (document it like Push). HookTimeout has NO env/flag/git-config (file+default only per FR-V6 — simplest). Both mirror byte-for-byte."

- file: internal/config/config.go
  why: "EDIT. Config struct: add NoVerify + HookTimeout near Push (line ~126) / Timeout (line ~68). Defaults(): add NoVerify:false + HookTimeout:10*time.Minute near Push:false (line ~192)."
  pattern: "Mirror Push's 4-line doc comment style for NoVerify; mirror Timeout's inline comment for HookTimeout. Fields are flat (not nested). gofmt realigns."

- file: internal/config/file.go
  why: "EDIT (4 spots). (a) fileGeneration struct: + NoVerify bool + HookTimeout string (near Push, line ~66). (b) loadTOML: + HookTimeout parse alongside Timeout (near line ~177). (c) materialize: + hookTimeout param + copy lines. (d) overlay: + NoVerify (only-true) + HookTimeout (!=0) near Push (line ~332)."
  pattern: "Mirror Push byte-for-byte for NoVerify; mirror Timeout byte-for-byte for HookTimeout. The materialize signature gains a hookTimeout time.Duration parameter (like it already has timeout)."

- file: docs/configuration.md
  why: "EDIT (Mode A). Add `hook_timeout` to the [generation] table (near the push row) + to the commented [generation] template block. Note `no_verify` is a flag/env/git-config bool (not a [generation] key — it's a CLI/behavioral flag, not a generation knob; but it IS in the config struct for 5-layer resolution)."

# Read-only refs
- file: internal/config/config.go # Push field (:126), Timeout field (:68)
  why: "The EXACT templates. Push: 4-line doc comment + `Push bool toml:\"push\"`. Timeout: `Timeout time.Duration toml:\"timeout\"` with inline comment."
- file: internal/config/file.go # materialize (:260-300), overlay (:310-345), loadTOML (:170-196)
  why: "The exact plumbing. materialize signature: `func materialize(fc *fileConfig, timeout time.Duration) *Config`. overlay: `if src.Push { dst.Push = true }` (:332), `if src.Timeout != 0 { dst.Timeout = src.Timeout }` (:319-320). loadTOML: `timeout, err = time.ParseDuration(fc.Defaults.Timeout)` (:177-178), `return materialize(&fc, timeout), nil` (:196)."
```

### Current Codebase Tree

```bash
stagecoach/
├── internal/config/
│   ├── config.go     # EDIT: + NoVerify + HookTimeout fields + Defaults
│   └── file.go       # EDIT: + fileGeneration fields + loadTOML parse + materialize + overlay
└── docs/
    └── configuration.md  # EDIT: + hook_timeout row
```

### Known Gotchas

```go
// CRITICAL (only-true-propagates for NoVerify): the file layer CANNOT set NoVerify=false via config.
// materialize: `if g.NoVerify { c.NoVerify = true }` — a file that sets `no_verify = false` is a no-op
// (false is the zero value; only-true-propagates). This is the SAME accepted limitation as Push.
// Document it in the field comment + docs/configuration.md. (The flag/env layers CAN set it false — S2.)

// CRITICAL (HookTimeout is file+default ONLY): HookTimeout has NO env var, NO flag, NO git-config key
// (per arch §2: "Decision: HookTimeout is file + default only (no env/flag/git-config) — simplest").
// S1 adds only the struct + Defaults + file layer. S2 adds NoVerify's env/flag/git-config but NOT HookTimeout's.

// GOTTA (loadTOML parse): add the HookTimeout duration-string parse in loadTOML alongside the existing
// Timeout parse (line ~177). Parse `fc.Generation.HookTimeout` (a fileGeneration string field) into a
// time.Duration, validate it (malformed string → wrapped error at load), and pass it to materialize as
// a new parameter. Mirror the existing timeout parse's error message format.

// GOTCHA (materialize signature change): materialize gains a `hookTimeout time.Duration` parameter →
// `func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) *Config`. The ONLY caller is
// loadTOML (line 196) — update that one call site: `materialize(&fc, timeout, hookTimeout)`.

// GOTCHA (gofmt): the struct fields are column-aligned. Adding NoVerify (8 chars) and HookTimeout
// (11 chars + time.Duration type) may shift the alignment column. RUN gofmt -w after the edit.
```

## Implementation Blueprint

### Implementation Tasks

```yaml
Task 1: config.go — add NoVerify + HookTimeout fields + Defaults
  - ADD after the Push field block (line ~126), before the Providers field:
        // NoVerify is the §9.25 FR-V5 --no-verify hook bypass (mirrors `git commit --no-verify`).
        // When true, skips pre-commit and commit-msg hooks (prepare-commit-msg and post-commit still run).
        // Full 5-layer precedence: --no-verify / STAGECOACH_NO_VERIFY / stagecoach.no_verify / [generation].no_verify,
        // default false — hooks run by default; --no-verify is the deliberate exception. FILE LAYER LIMITATION
        // (same as Push): only-true-propagates — a file setting `no_verify = false` is a no-op; the flag/env
        // layers can set it false. See cmd root.go + hooks.RunCommitHooks (M3).
        NoVerify bool `toml:"no_verify"`

        // HookTimeout is the §9.25 FR-V6 per-hook execution timeout. Bounds each hook invocation so a wedged
        // hook cannot hang a commit. Defaults: 10m. File-only (no env/flag/git-config) per arch §2 decision.
        HookTimeout time.Duration `toml:"hook_timeout"`
  - ADD in Defaults() after Push:false (line ~192):
        NoVerify:             false,    // §9.25 FR-V5 default (hooks run by default)
        HookTimeout:          10 * time.Minute, // §9.25 FR-V6 default per-hook timeout
  - DO NOT: add env/flag/git-config handling (S2); change Push or Timeout.

Task 2: file.go — add fileGeneration fields
  - ADD to fileGeneration struct after Push (line ~66):
        NoVerify             bool     `toml:"no_verify"`         // §9.25 FR-V5 — only-true-propagates (mirrors Push)
        HookTimeout          string   `toml:"hook_timeout"`      // §9.25 FR-V6 — duration string "10m", parsed in loadTOML
  - DO NOT: add to fileDefaults (these are [generation] keys, not [defaults]).

Task 3: file.go — loadTOML: parse HookTimeout
  - LOCATE the existing Timeout parse (lines ~177-182). ADD alongside it, BEFORE `return materialize(...)`:
        var hookTimeout time.Duration
        if fc.Generation.HookTimeout != "" {
            hookTimeout, err = time.ParseDuration(fc.Generation.HookTimeout)
            if err != nil {
                return nil, fmt.Errorf("parse config %s: invalid hook_timeout %q: %w", path, fc.Generation.HookTimeout, err)
            }
        }
  - UPDATE the materialize call: `return materialize(&fc, timeout, hookTimeout), nil`

Task 4: file.go — materialize: accept hookTimeout + copy fields
  - CHANGE the signature: `func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) *Config`
  - ADD after `c := &Config{Timeout: timeout}`: `c.HookTimeout = hookTimeout` (zero if unset — overlay skips zero)
  - ADD after the Push copy (line ~275-276): `if g.NoVerify { c.NoVerify = true }`

Task 5: file.go — overlay: copy NoVerify + HookTimeout
  - ADD after the Push overlay line (line ~332-333):
        // §9.25 FR-V5 — no_verify (only-true-propagates, same as Push)
        if src.NoVerify {
            dst.NoVerify = true
        }
  - ADD after the Push overlay (near the generation section):
        // §9.25 FR-V6 — hook_timeout (duration-zero guard, same as Timeout)
        if src.HookTimeout != 0 {
            dst.HookTimeout = src.HookTimeout
        }

Task 6: docs/configuration.md — Mode A
  - ADD `hook_timeout` row to the [generation] table (near the push row ~line 145):
        "| `hook_timeout` | duration | `10m` | Per-hook execution timeout (§9.25 FR-V6). |"
  - ADD to the commented [generation] template block (~line 104):
        "hook_timeout = \"10m\"     # per-hook execution timeout (§9.25 FR-V6)"
  - NOTE `no_verify` is a flag/env/git-config bool (not a [generation] key — if the doc distinguishes sections,
    note it belongs to the CLI/behavior surface, not the generation tuning table). OR: if NoVerify IS in
    fileGeneration (as implemented — mirroring Push), add a `no_verify` row too: 
        "| `no_verify` | bool | `false` | Skip pre-commit and commit-msg hooks (§9.25 FR-V5; mirrors `git commit --no-verify`). |"

Task 7: VALIDATE
  - RUN: gofmt -w internal/config/config.go internal/config/file.go
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === config.go — NoVerify field (mirrors Push) ===
	// NoVerify is the §9.25 FR-V5 --no-verify hook bypass (mirrors `git commit --no-verify`).
	// When true, skips pre-commit and commit-msg hooks (prepare-commit-msg and post-commit still run).
	// Full 5-layer precedence: --no-verify / STAGECOACH_NO_VERIFY / stagecoach.no_verify / [generation].no_verify,
	// default false — hooks run by default; --no-verify is the deliberate exception. FILE LAYER LIMITATION
	// (same as Push): only-true-propagates — a file setting `no_verify = false` is a no-op; the flag/env
	// layers can set it false. See cmd root.go + hooks.RunCommitHooks (M3).
	NoVerify bool `toml:"no_verify"`

	// HookTimeout is the §9.25 FR-V6 per-hook execution timeout. Bounds each hook invocation so a wedged
	// hook cannot hang a commit. Defaults: 10m. File-only (no env/flag/git-config) per arch §2 decision.
	HookTimeout time.Duration `toml:"hook_timeout"`
```

```go
// === file.go — loadTOML parse (alongside existing Timeout) ===
	var hookTimeout time.Duration
	if fc.Generation.HookTimeout != "" {
		hookTimeout, err = time.ParseDuration(fc.Generation.HookTimeout)
		if err != nil {
			return nil, fmt.Errorf("parse config %s: invalid hook_timeout %q: %w", path, fc.Generation.HookTimeout, err)
		}
	}
	return materialize(&fc, timeout, hookTimeout), nil
```

```go
// === file.go — materialize (signature + HookTimeout + NoVerify) ===
func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) *Config {
	c := &Config{Timeout: timeout, HookTimeout: hookTimeout} // zero if unset → overlay skips zero
	// ...
	if g.Push {
		c.Push = true
	}
	if g.NoVerify {
		c.NoVerify = true // only-true-propagates (same as Push)
	}
	// ...
```

```go
// === file.go — overlay (NoVerify + HookTimeout, after Push) ===
	// §9.22 FR-P1 — push
	if src.Push {
		dst.Push = true
	}
	// §9.25 FR-V5 — no_verify (only-true-propagates)
	if src.NoVerify {
		dst.NoVerify = true
	}
	// §9.25 FR-V6 — hook_timeout (duration-zero guard)
	if src.HookTimeout != 0 {
		dst.HookTimeout = src.HookTimeout
	}
```

### Integration Points

```yaml
CONFIG STRUCT (internal/config/config.go):
  - field: "NoVerify bool `toml:\"no_verify\"`"        # mirrors Push; only-true-propagates
  - field: "HookTimeout time.Duration `toml:\"hook_timeout\"`"  # mirrors Timeout; duration-zero guard

DEFAULTS: NoVerify: false; HookTimeout: 10 * time.Minute

FILE LAYER (internal/config/file.go):
  - fileGeneration: +NoVerify bool +HookTimeout string
  - loadTOML: +HookTimeout parse (alongside Timeout)
  - materialize: +hookTimeout param; c.HookTimeout=hookTimeout; if g.NoVerify {c.NoVerify=true}
  - overlay: if src.NoVerify {dst.NoVerify=true}; if src.HookTimeout != 0 {dst.HookTimeout=src.HookTimeout}

NO-TOUCH: env/flag/git-config resolution (S2 = P1.M1.T1.S2); any consumer (M3); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOCS (Mode A): docs/configuration.md — hook_timeout row + template entry; no_verify note.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/config/config.go internal/config/file.go
gofmt -l .            # Expected: empty
go vet ./internal/config/...  # Expected: exit 0
go build ./...        # Expected: exit 0
```

### Level 2: Unit Tests

```bash
cd /home/dustin/projects/stagecoach
go test -race ./internal/config/ -v   # Expected: all green (existing tests field-specific, not DeepEqual)
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach
go test -race ./...   # Expected: ALL packages green (S1 adds dead fields; no behavior change)
git diff --stat       # Expected: internal/config/{config,file}.go + docs/configuration.md only
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation
- [ ] `Config.NoVerify bool` + `Config.HookTimeout time.Duration` exist with TOML tags + doc comments.
- [ ] `Defaults()`: NoVerify=false + HookTimeout=10*time.Minute.
- [ ] `fileGeneration.NoVerify bool` + `fileGeneration.HookTimeout string`.
- [ ] `loadTOML` parses HookTimeout (duration string) alongside Timeout.
- [ ] `materialize` receives hookTimeout; sets `c.HookTimeout`; copies NoVerify (only-true).
- [ ] `overlay` copies NoVerify (only-true) + HookTimeout (!=0).
- [ ] `docs/configuration.md` has `hook_timeout` row + template entry.

### Scope Discipline
- [ ] ONLY `internal/config/{config,file}.go` + `docs/configuration.md` modified.
- [ ] Did NOT add env/flag/git-config (S2); consumer wiring (M3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't add env/flag/git-config for HookTimeout — arch §2 explicitly decides "file + default only".
  (NoVerify gets those layers in S2; HookTimeout does not.)
- ❌ Don't make NoVerify `*bool` — it mirrors Push (plain bool, only-true-propagates). The accepted
  limitation (file can't set false) is documented in the field comment.
- ❌ Don't forget the materialize signature change — it gains a `hookTimeout time.Duration` parameter.
  The ONLY caller is `loadTOML` (line ~196); update that one call site.
- ❌ Don't parse HookTimeout in materialize — parse it in loadTOML (alongside Timeout) so a malformed
  string fails at LOAD (with the path in the error), not at merge time. Mirror Timeout exactly.
- ❌ Don't place NoVerify/HookTimeout in `fileDefaults` — they are `[generation]` keys (fileGeneration),
  mirroring Push (which is also in fileGeneration).

---

## Confidence Score

**9.5/10** — a purely additive config-fields edit mirroring two proven templates (Push for NoVerify,
Timeout for HookTimeout) byte-for-byte. The arch `codebase_reality.md §2` specifies every line. The only
residual uncertainty is the materialize signature change (adding the `hookTimeout` parameter) — a trivial
mechanical change with exactly one caller. The fields are dead (unconsumed) after S1, so there is no
behavior change to regress.
