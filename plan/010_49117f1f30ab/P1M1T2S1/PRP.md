---
name: "P1.M1.T2.S1 — root.go --no-verify flag definition + help text + cli.md row"
description: |
  CLI surface for FR-V5 hook bypass (§9.25). Register the `--no-verify` persistent flag in
  `internal/cmd/root.go` (var `flagNoVerify bool` + `pf.BoolVar(&flagNoVerify, "no-verify", false,
  "<help>")` after the --push registration). Help text states: bypass pre-commit + commit-msg only
  (prepare-commit-msg and post-commit still run), mirrors `git commit --no-verify`; env
  STAGECOACH_NO_VERIFY, git stagecoach.no_verify, default false; §9.25 FR-V5. Add the matching row to
  docs/cli.md global-flags table (after --push). Config.NoVerify is LANDED (S1); loadFlags READER is S2
  (parallel). This task is the flag DEFINITION + docs row only. No behavior yet (M3 consumes cfg.NoVerify).
---

## Goal

**Feature Goal**: Register the `--no-verify` persistent CLI flag (FR-V5) so it is resolvable through
`config.Load → cfg.NoVerify` (the loadFlags reader in S2 reads it via `fs.Changed`/`fs.GetBool`), and
document it in the docs/cli.md global-flags table.

**Deliverable** (2 production edits + 1 docs row):
1. `internal/cmd/root.go` — `var flagNoVerify bool` (after `flagPush`, line 100) +
   `pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>")` (after the --push block, ~line 213).
2. `docs/cli.md` — a `--no-verify` row in the global-flags table (after --push, line 43).

**Success Definition**: `--no-verify` is a registered persistent flag; `stagecoach --help` lists it with the
correct help text; `go build/vet/gofmt` clean; `go test ./...` green. The docs/cli.md table has the row
with the FR-V5 semantics. No behavior change (M3's runner consumes cfg.NoVerify).

## User Persona

**Target User**: The user who wants to bypass `pre-commit`/`commit-msg` hooks on a stagecoach commit
(mirroring `git commit --no-verify`), and the contributor wiring the loadFlags reader (S2) + the
RunCommitHooks runner (M3).

**Use Case**: `stagecoach --no-verify` on a repo with a slow `pre-commit` hook skips it for that commit;
`prepare-commit-msg` and `post-commit` still run (FR-V5).

**Pain Points Addressed**: Provides the CLI surface for the hook-bypass escape hatch — without it, there
is no way to opt out of the hooks-on-commit-path feature (§9.25) for a one-off commit.

## Why

- **PRD §9.25 FR-V5 mandates the flag.** "`--no-verify` — bypass `pre-commit` and `commit-msg` only. Mirrors `git commit --no-verify` exactly... Surfaced as CLI `--no-verify`, env `STAGECOACH_NO_VERIFY`, git config `stagecoach.no_verify`." §15.2 lists it in the global-flags table.
- **Unblocks S2 (the reader) + M3 (the runner).** S2's loadFlags reads `fs.Changed("no-verify")` — the flag must be registered for the reader to find it (cobra's `fs.Changed` returns false for an unregistered flag). M3's runner reads `cfg.NoVerify` to decide whether to skip pre-commit/commit-msg.
- **Mirrors the proven --push pattern.** `--no-verify` is a bool flag with env/git-config/default — the exact shape of `--push` (root.go:100/206-213). The registration is a 2-line mechanical edit.
- **No behavior change yet.** The flag is inert until M3 wires the runner. Defining it now lets S2 + M3 land independently against a stable flag surface.

## What

A `var flagNoVerify bool` + a `pf.BoolVar` registration in root.go (mirroring --push's shape), plus a row
in the docs/cli.md global-flags table. No direct reads of the var (same discipline as flagPush — only
loadFlags reads it). No behavior, no test (S2 has the reader test), no other file.

### Success Criteria

- [ ] `internal/cmd/root.go` has `var flagNoVerify bool` (after `flagPush`, ~line 100).
- [ ] `internal/cmd/root.go` has `pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>")` (after --push, ~line 213).
- [ ] The help text states: bypass pre-commit + commit-msg only (prepare-commit-msg and post-commit still run); env STAGECOACH_NO_VERIFY, git stagecoach.no_verify; default false; §9.25 FR-V5.
- [ ] `docs/cli.md` global-flags table has a `--no-verify` row (after --push, ~line 43).
- [ ] `flagNoVerify` is NEVER read directly (only via `fs.Changed`/`fs.GetBool` in loadFlags — same as flagPush).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current root.go lines (flagPush var :100, --push
BoolVar :206-213, the next comment at :214) + the docs/cli.md --push row (:43), gives the exact target
for each (copy-paste-ready), confirms the flag is absent + Config.NoVerify is landed, and names the sibling
task boundary (S2 = the reader; M3 = the consumer). The help text is supplied verbatim. No inference.

### Documentation & References

```yaml
# MUST READ — the FR spec + the global-flags table
- file: PRD.md
  why: "§9.25 FR-V5 (--no-verify: bypass pre-commit + commit-msg only; prepare-commit-msg and post-commit still run; mirrors git commit --no-verify; env STAGECOACH_NO_VERIFY, git stagecoach.no_verify, default false). §15.2 (the global-flags table has the --no-verify row — the exact text to mirror in docs/cli.md)."
  critical: "FR-V5's semantics ('skips pre-commit and commit-msg. prepare-commit-msg and post-commit STILL run') MUST be in the help text — it's the key distinction from a blanket 'skip all hooks'. Default false (hooks run by default)."

- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  why: "§2 confirms: root.go has `var flagPush bool` + `pf.BoolVar(&flagPush, \"push\", false, ...)` at line 206; `--no-verify` goes right after --push (~line 212). docs/cli.md --push row is at line 43 — add --no-verify after it."
  critical: "§2 line 38: 'root.go: var flagPush bool; pf.BoolVar(&flagPush, \"push\", false, …) at line 206.' §2 line 202: 'docs/cli.md global-flags table — add --no-verify row (after --push at line 43).' These are the exact insertion points."

- docfile: plan/010_49117f1f30ab/P1M1T1S2/PRP.md
  why: "The parallel sibling (load.go env/flag/git-config READER for NoVerify). Confirms it does NOT touch root.go: 'The --no-verify flag VAR is registered in P1.M1.T2.S1 (root.go); S2 writes only the loadFlags READER.' No file overlap → no conflict."
  critical: "S2's loadFlags does `fs.Changed(\"no-verify\")` + `fs.GetBool` — the flag MUST be registered (by THIS task) for the reader to work. S2's reader is inert + safe pre-T2 (fs.Changed returns false for an unregistered flag), but the flag must exist for the feature to resolve."

- docfile: plan/010_49117f1f30ab/P1M1T2S1/research/no_verify_flag_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-05): flagNoVerify ABSENT; Config.NoVerify LANDED (S1); flagPush var at :100 + BoolVar at :206-213; docs/cli.md --push row at :43; the 3 exact edits (var + BoolVar + docs row) with verbatim current→target; decisions D1–D6. READ THIS FIRST."

# The edit target
- file: internal/cmd/root.go
  why: "EDIT (2 spots). (a) Var: add `var flagNoVerify bool` after `var flagPush bool` (line 100). (b) BoolVar: add `pf.BoolVar(&flagNoVerify, \"no-verify\", false, \"<help>\")` after the --push block (lines 206-213), before the reasoning-flags comment (line 214)."
  pattern: "Mirror --push EXACTLY: a `var flagX bool` declaration + a `pf.BoolVar(&flagX, \"name\", false, \"multi-line help with (env…, git…; default false.) (§FR)\")` registration. The flag var is read ONLY via fs.Changed/fs.GetBool in loadFlags — NEVER directly."
  gotcha: "Insert the BoolVar BETWEEN the --push closing `)` (line 213) and the `// §15.2 reasoning flags` comment (line 214). Do NOT place it elsewhere. The help text must cite env STAGECOACH_NO_VERIFY + git stagecoach.no_verify + default false (mirrors --push's citation style)."

- file: docs/cli.md
  why: "EDIT (1 row). The global-flags table's --push row is at line 43. Insert the --no-verify row BETWEEN --push (43) and --planner-provider (44). Match the table's column structure: `| Flag | Type | Default | Env | Git config | Description |`."
  pattern: "Mirror the --push row's structure: `| \`--no-verify\` | bool | false | \`STAGECOACH_NO_VERIFY\` | \`stagecoach.no_verify\` | <description with §9.25 FR-V5> |`."
  gotcha: "The description must state the FR-V5 semantics precisely: 'Bypass pre-commit and commit-msg hooks... (prepare-commit-msg and post-commit still run).' NOT a blanket 'skip all hooks' — the distinction is the FR-V5 contract."

# Read-only refs
- file: internal/config/config.go
  why: "READ-ONLY. Config.NoVerify is LANDED (S1) at :128-134 (field) + :205 (Defaults: false). Confirms the field this flag resolves into."
- file: internal/config/load.go
  why: "READ-ONLY (S2's territory). loadFlags will do `fs.Changed(\"no-verify\")` + `fs.GetBool(\"no-verify\")` — the flag THIS task registers. The flag definition is the prerequisite."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/cmd/root.go     # EDIT: +var flagNoVerify (line ~100) +BoolVar (after --push, ~line 213)
└── docs/cli.md              # EDIT: +--no-verify row (after --push, line 43)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/cmd/root.go     # +var flagNoVerify bool + pf.BoolVar(&flagNoVerify, "no-verify", ...)
    docs/cli.md              # +--no-verify row in the global-flags table
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/cmd/root.go` | MODIFY | Add `var flagNoVerify bool` + the `pf.BoolVar` registration with help text. **Only production file.** |
| `docs/cli.md` | MODIFY | Add the `--no-verify` row to the global-flags table. |

**Explicitly NOT touched**: `internal/config/*` (S1/S2 — field + reader), any other `internal/cmd/*` file,
`internal/generate/*`/`internal/hooks/*` (M3 — the runner), any other docs, `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — the help text MUST state the FR-V5 distinction): --no-verify bypasses pre-commit and
// commit-msg ONLY — prepare-commit-msg and post-commit STILL run. This is the key semantic that
// distinguishes it from a blanket "skip all hooks." The help text and the docs/cli.md row MUST state this
// precisely (mirrors git commit --no-verify). Do NOT write "skip all hooks" or "bypass hooks."

// CRITICAL (G2 — the flag var is NEVER read directly): flagNoVerify is read ONLY via fs.Changed/fs.GetBool
// in loadFlags (S2). Do NOT read flagNoVerify anywhere in root.go or any other file. Same discipline as
// flagPush/flagEdit (the var exists solely for pf.BoolVar to bind to; the reader extracts the value).

// GOTCHA (G3 — place the BoolVar AFTER --push, BEFORE the reasoning flags): the --push block is at lines
// 206-213 (multi-line help ending with `)`). The next line (214) is `// §15.2 reasoning flags`. Insert
// the --no-verify BoolVar BETWEEN them. Do NOT place it among the reasoning flags or elsewhere.

// GOTCHA (G4 — the docs row goes after --push at line 43, before --planner-provider at line 44): match
// the table's column structure (Flag|Type|Default|Env|Git config|Description). The description cites §9.25
// FR-V5 and states the precise semantics.

// GOTCHA (G5 — no test for this task): the flag definition is a 2-line registration; S2's loadFlags test
// (TestLoadFlags_NoVerify) verifies the reader works once the flag is registered. The validation here is
// `go build` (the reader compiles against the registered flag) + `stagecoach --help` (the flag appears).
```

## Implementation Blueprint

### Data models and structure

No new types. `flagNoVerify bool` is a package-level var bound to the persistent flag set via `pf.BoolVar`.
It resolves into `Config.NoVerify` (S1's field, config.go:134) via the loadFlags reader (S2).

### The edits (exact — current → target)

**root.go var declaration** (after line 100):
```go
// CURRENT
var flagEdit bool
var flagPush bool

// TARGET
var flagEdit bool
var flagPush bool
var flagNoVerify bool
```

**root.go BoolVar registration** (after the --push block ~line 213, before the reasoning comment ~line 214):
```go
// CURRENT (excerpt — the --push block ends here, then the reasoning comment)
		"...push failed\" prints, and stagecoach exits 1. Skipped on --dry-run, the nothing-to-commit "+
			"exit, and any rescue/CAS abort. (env STAGECOACH_PUSH, git stagecoach.push, config "+
			"[generation].push; default false.) (§9.22 FR-P1)")
	// §15.2 reasoning flags (FR-R6) — global + per-role; zero default; loadFlags reads via fs.Changed.

// TARGET (insert the --no-verify BoolVar between --push and the reasoning comment)
		"...push failed\" prints, and stagecoach exits 1. Skipped on --dry-run, the nothing-to-commit "+
			"exit, and any rescue/CAS abort. (env STAGECOACH_PUSH, git stagecoach.push, config "+
			"[generation].push; default false.) (§9.22 FR-P1)")
	pf.BoolVar(&flagNoVerify, "no-verify", false,
		"Bypass pre-commit and commit-msg hooks for this commit (mirrors git commit --no-verify; "+
			"prepare-commit-msg and post-commit still run). (env STAGECOACH_NO_VERIFY, git "+
			"stagecoach.no_verify; default false.) (§9.25 FR-V5)")
	// §15.2 reasoning flags (FR-R6) — global + per-role; zero default; loadFlags reads via fs.Changed.
```

**docs/cli.md table row** (after line 43, before line 44):
```
// CURRENT (line 43-44)
| `--push` | bool | false | `STAGECOACH_PUSH` | `stagecoach.push` | Run plain `git push` ... (§9.22 FR-P1) |
| `--planner-provider <name>` | string | "" | ... |

// TARGET (insert the --no-verify row between --push and --planner-provider)
| `--push` | bool | false | `STAGECOACH_PUSH` | `stagecoach.push` | Run plain `git push` ... (§9.22 FR-P1) |
| `--no-verify` | bool | false | `STAGECOACH_NO_VERIFY` | `stagecoach.no_verify` | Bypass `pre-commit` and `commit-msg` hooks for this commit (mirrors `git commit --no-verify`; `prepare-commit-msg` and `post-commit` still run). §9.25, FR-V5. |
| `--planner-provider <name>` | string | "" | ... |
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/root.go — add the var + BoolVar
  - FILE: internal/cmd/root.go
  - EDIT 1 (var): add `var flagNoVerify bool` after `var flagPush bool` (line 100).
  - EDIT 2 (BoolVar): add `pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>")` after the --push
    block's closing `)` (line 213), before the `// §15.2 reasoning flags` comment (line 214). Use the
    verbatim help text from the edits above.
  - DO NOT: read flagNoVerify directly (G2); place the BoolVar elsewhere (G3); change --push or any other flag.
  - RUN: gofmt -w internal/cmd/root.go
  - VERIFY: go build ./internal/cmd/  → exit 0.

Task 2: EDIT docs/cli.md — add the table row
  - FILE: docs/cli.md
  - LOCATE the --push row (line 43) in the global-flags table. Insert the --no-verify row BETWEEN --push
    (line 43) and --planner-provider (line 44). Use the verbatim row from the edits above.
  - DO NOT: reorder existing rows; edit other tables or prose.

Task 3: VALIDATE
  - RUN: gofmt -l .            # must be empty
  - RUN: go build ./...        # the flag registration compiles; the reader (S2) compiles against it
  - RUN: go vet ./...
  - RUN: go test ./...         # whole repo green (the flag is inert; no behavior change)
  - RUN: go run ./cmd/stagecoach --help 2>&1 | grep -q "no-verify"  # the flag appears in help output
  - FIX-FORWARD: a compile failure = a typo in the var/BoolVar name; a --help miss = the flag wasn't registered.
```

### Implementation Patterns & Key Details

```go
// === The --no-verify flag mirrors --push exactly ===
// --push:   var flagPush bool (line 100) + pf.BoolVar(&flagPush, "push", false, "<help with env/git/default>") (line 206)
// --no-verify: var flagNoVerify bool (line 101) + pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>") (after line 213)
// Both are bool flags with env (STAGECOACH_*) + git-config (stagecoach.*) + default false.
// Both are read ONLY via fs.Changed/fs.GetBool in loadFlags (never directly).

// === The FR-V5 help-text distinction ===
// --no-verify bypasses pre-commit + commit-msg ONLY. prepare-commit-msg and post-commit STILL run.
// This mirrors `git commit --no-verify` exactly (FR-V5). The help text MUST state this — it is NOT
// "skip all hooks" (prepare-commit-msg runs because it composes with stagecoach's message; post-commit
// runs because it cannot undo a landed commit — FR-V5/FR-V7).
```

### Integration Points

```yaml
ROOT.GO (internal/cmd/root.go):
  - +var flagNoVerify bool (after flagPush, line 100)
  - +pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>") (after --push, ~line 213)

DOCS (docs/cli.md):
  - +--no-verify row in the global-flags table (after --push, line 43)

CONSUMED BY:
  - internal/config/load.go loadFlags (S2): fs.Changed("no-verify") + fs.GetBool → cfg.NoVerify (DIRECT set)
  - internal/hooks RunCommitHooks (M3): reads cfg.NoVerify to skip pre-commit/commit-msg

NO-TOUCH (explicitly):
  - internal/config/* (S1 field + S2 reader)
  - any other internal/cmd/* file; internal/generate/*, internal/hooks/* (M3)
  - docs other than cli.md; PRD.md, tasks.json, prd_snapshot.md, plan/*

GATE: go build ./... → OK; stagecoach --help shows --no-verify; go test ./... green
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/cmd/root.go   # Expected: empty (run gofmt -w if listed)
go vet ./internal/cmd/...        # Expected: exit 0
go build ./...                   # Expected: exit 0 (flag registration compiles)

# Expected: Zero errors.
```

### Level 2: Flag Registration (the flag appears in help)

```bash
cd /home/dustin/projects/stagecoach

# The flag is registered + appears in --help with the correct help text.
go run ./cmd/stagecoach --help 2>&1 | grep -A2 "no-verify"
# Expected: the --no-verify line with the FR-V5 help text (bypass pre-commit + commit-msg only;
# prepare-commit-msg and post-commit still run; env STAGECOACH_NO_VERIFY, git stagecoach.no_verify; default false).

go test ./...   # Expected: all green (the flag is inert; no behavior change)
```

### Level 3: Docs + Scope

```bash
cd /home/dustin/projects/stagecoach

# The docs/cli.md row exists with the correct columns.
grep -n '\-\-no-verify' docs/cli.md   # Expected: one row in the global-flags table

# Confirm ONLY the 2 intended files changed.
git diff --stat -- internal/ docs/
# Expected: internal/cmd/root.go + docs/cli.md only.
```

### Level 4: No Direct Read (the flag-var discipline)

```bash
cd /home/dustin/projects/stagecoach

# flagNoVerify is NEVER read directly (only via fs.Changed/fs.GetBool in loadFlags).
grep -n 'flagNoVerify' internal/cmd/root.go
# Expected: exactly 2 matches — the var declaration + the pf.BoolVar binding. NO other reference
# (no `if flagNoVerify`, no `flagNoVerify ==`, no direct use anywhere).

# The loadFlags reader (S2) reads it via fs.Changed:
grep -n 'no-verify' internal/config/load.go 2>/dev/null || echo "(S2 not yet landed — the reader is inert until then)"
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` green.
- [ ] `stagecoach --help` shows `--no-verify` with the FR-V5 help text.

### Feature Validation

- [ ] `var flagNoVerify bool` exists in root.go (after `flagPush`).
- [ ] `pf.BoolVar(&flagNoVerify, "no-verify", false, "<help>")` exists (after --push).
- [ ] The help text states: bypass pre-commit + commit-msg only; prepare-commit-msg and post-commit still run; env STAGECOACH_NO_VERIFY, git stagecoach.no_verify; default false; §9.25 FR-V5.
- [ ] `docs/cli.md` has the `--no-verify` row (after --push) with the FR-V5 semantics.
- [ ] `flagNoVerify` is NEVER read directly (grep: only the var + BoolVar binding).

### Scope Discipline Validation

- [ ] ONLY `internal/cmd/root.go` + `docs/cli.md` modified.
- [ ] Did NOT edit `internal/config/*` (S1/S2), any other cmd file, or `internal/hooks/*` (M3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't write "skip all hooks" in the help text — FR-V5 bypasses pre-commit + commit-msg ONLY;
  prepare-commit-msg and post-commit STILL run. State the precise semantics. (gotcha G1)
- ❌ Don't read `flagNoVerify` directly anywhere. It's bound via BoolVar and read ONLY via `fs.Changed`/
  `fs.GetBool` in loadFlags (S2). Same discipline as flagPush/flagEdit. (G2)
- ❌ Don't place the BoolVar among the reasoning flags or elsewhere — it goes after --push (line 213),
  before the reasoning comment (line 214). (G3)
- ❌ Don't write "bypass hooks" without specifying which hooks — the user must know prepare-commit-msg and
  post-commit still run (that's the git-commit-parity contract). (G1)
- ❌ Don't edit `internal/config/*` (S1 field + S2 reader), `internal/hooks/*` (M3 runner), or any other
  file. This task is root.go + docs/cli.md only.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed change — one `var bool` + one `pf.BoolVar` (mirroring the
exact shape of `--push` at root.go:100/206-213, which is quoted verbatim) + one docs/cli.md table row
(mirroring the --push row at :43, with the FR-V5 text from the PRD §15.2 table). The flag is confirmed
ABSENT (genuine add), Config.NoVerify is LANDED (S1), and the prior parallel PRP (S2) explicitly defers
the flag VAR to this task (no conflict). The help text and docs row are supplied verbatim from the contract
+ the PRD §15.2 table. Adding a `pf.BoolVar` registration is provably safe (it compiles; the reader via
`fs.Changed` is inert until S2 lands; no behavior change until M3). The residual 0.5 uncertainty is purely
gofmt alignment of the multi-line help string (cosmetic, gated by `gofmt -l .`). The S2/M3 boundaries are
cleanly fenced.
