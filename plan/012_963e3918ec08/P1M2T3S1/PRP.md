---
name: "P1.M2.T3.S1 — Catch-all stagehand→stagecoach rename in .go (error prefixes + status/progress + ALL remaining residue)"
description: |
  Finish the `stagehand` → `stagecoach` rename in the **Go source**. The contract TITLE says "error message
  prefixes and status/progress strings" (~20 sites, Layer 3.2-3.3), but the contract MECHANISM is a broad
  `sed s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g` and the OUTPUT gate is **"Zero occurrences of
  'stagehand' (case-insensitive) in any .go file."** The REAL residue is **377 occurrences across ~60 .go
  files** — the title vastly underestimates the catch-all scope. **This PRP follows the MECHANISM + OUTPUT
  (the catch-all), not the title's narrow count.**

  ⚠️ **THE critical finding — the real scope is 377, NOT ~20.** `grep -rni 'stagehand' --include='*.go' |
  wc -l` → 377, spanning nearly every package (17 files in internal/cmd alone, then generate/config/hook/
  provider/prompt/decompose/integrate/hooks/e2e/ui/signal/lock/exitcode/pkg/cmd). The contract's "~20+
  error-prefix locations" lists only Layer 3.2. The residue is scattered COMMENTS + straggler STRINGS that
  M1.T2 (identifiers) missed. **Follow the broad-sed mechanism + the zero-residue gate; do NOT limit to the
  title's ~20 sites (that would leave ~357 residue and FAIL the OUTPUT gate).** See research §1.

  ⚠️ **THE broad sed is SAFE (the contract's CAUTION, resolved).** Verified: all 377 hits are **comments +
  string literals** — ZERO identifiers, ZERO import paths remain (M1.T1 imports + M1.T2 identifiers + M2.T1
  env-var/git-config keys already landed). No all-caps `STAGEHAND` residue either (M2.T1.S1 did the env
  vars). So `s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g` CANNOT corrupt an identifier or import
  (none left); `go build ./...` proves it. See research §2.

  ⚠️ **THE semantic strings — production + test rename IN LOCKSTEP.** A few straggler strings have test
  assertions (hook status `"stagehand (v1)"`, hook script `exec stagehand hook exec`, backup suffix
  `.stagehand-backup.`, root `Use: "stagehand"`). A uniform sed renames the production string AND its test
  assertion IDENTICALLY → they stay matched → `go test ./... -count=1` passes. The hook-script rename is
  CORRECT, not cosmetic: the binary is `cmd/stagecoach`, so the installed hook MUST invoke `stagecoach`.
  See research §3.

  ⚠️ **THE hook-script rename is a NECESSARY fix.** `internal/hook/{hook.go,script.go}` emit
  `exec stagehand hook exec "$@"` — the installed prepare-commit-msg hook invokes a binary name that NO
  LONGER EXISTS (the binary is `stagecoach`). Sed → `exec stagecoach hook exec` so installed hooks work.
  Tests (script_test.go, cmd/hook_test.go, hooks/runner_test.go) assert the script content → both rename
  in lockstep. See research §3.

  ⚠️ **THE overlap with siblings (same direction → no conflict).** The broad sed SUBSUMES: (a) the PARALLEL
  P1.M2.T2.S2 (`.stagehandignore` → `.stagecoachignore` at root.go:164 + verbose.go:101 — the token contains
  "stagehand", so sed converts it); (b) the FUTURE P1.M2.T3.S2 (session-ID prefix `multiturn.go`,
  temp-dir prefixes, bootstrap template — Layer 3.4/3.5/3.9). All rename in the SAME direction → identical
  end state regardless of sequencing. If this task lands first, those siblings become verify-only no-ops.
  The PRP flags this so the orchestrator can re-sequence if it wants the siblings to remain meaningful.
  See research §4-5.

  ⚠️ **THE scope fences — `.go` ONLY.** The sed is `--include='*.go'`. README.md, docs/*.md, Makefile,
  .goreleaser.yaml, providers/*.toml, .github/workflows, FUTURE_SPEC.md → P1.M3 (build/CI) + P1.M4 (docs).
  `bin/*` + root binaries are build artifacts (rebuild clean). Identifiers/imports/module are ALREADY
  renamed (M1) — the sed must not (and cannot) touch them. The `plan/` dir → P1.M5.T1. See research §6.

  Deliverable: a broad `sed s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g` across all `.go` files under
  `internal/`, `cmd/`, `pkg/` (~60 files, 377 occurrences → 0). NO new files, NO non-`.go` edits, NO
  identifier/import changes, NO go.mod/go.sum change, NO docs. OUTPUT: zero `stagehand` (case-insensitive)
  in any `.go` file; `go build ./...` + `go test ./... -count=1` pass. DOCS: none (docs are M4).
---

## Goal

**Feature Goal**: Eliminate EVERY remaining `stagehand`/`Stagehand` occurrence in the Go source (comments,
error-prefix string literals, status/progress strings, and the handful of straggler semantic strings) so
the `.go` codebase is consistently `stagecoach`-named end to end — matching the renamed module
(`github.com/dustin/stagecoach`), binary (`cmd/stagecoach`), env vars (`STAGECOACH_*`), and git-config keys
(`stagecoach.*`) that M1/M2.T1 already landed.

**Deliverable** (in-place content rename across ~60 existing `.go` files — NO new files):
A single broad sed — `grep -rl 'stagehand\|Stagehand' --include='*.go' internal/ cmd/ pkg/ | xargs sed -i
's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'` — followed by `gofmt -w` on any file the sed touched,
then the verification gates.

**Success Definition**: `grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'`
returns ZERO; `go build ./...` clean (proves no identifier/import corrupted); `go test ./... -count=1` green
(proves semantic strings stayed coordinated with their test assertions); `gofmt -l internal/ cmd/ pkg/` clean;
go.mod/go.sum byte-unchanged; NO non-`.go` file touched (docs/Makefile/goreleaser/CI = M3/M4).

## User Persona

**Target User**: Every user + developer who sees the tool's strings. Today the binary is `stagecoach`, env
vars are `STAGECOACH_*`, git-config keys are `stagecoach.*` — but the error messages still say
`"stagehand: …"`, the hook status still prints `"stagehand (v1)"`, the installed hook script still invokes
`exec stagehand hook exec` (a binary that no longer exists), and code comments still reference "Stagehand".
After this task, the Go source speaks with one `stagecoach` voice. Transitively PRD h2.30 (the rename
mandate: "All references to stagehand must be replaced with stagecoach").

**Use Case**: A user runs `stagecoach`, hits an error, and sees `stagecoach: nothing staged to commit`
(today: `stagehand: …` — a stale name from a binary that isn't on their PATH under that name). Or they run
`stagecoach hook status` and see `stagecoach (v1)` (today: `stagehand (v1)`). Or their installed hook
actually works (today: `exec stagehand hook exec` fails because the binary is `stagecoach`).

**User Journey**: `stagecoach <anything>` → every error prefix, status string, installed-hook script, and
developer-facing comment says `stagecoach` — internally consistent with the module/binary/env/config identity.

**Pain Points Addressed**: removes the stale-name confusion (errors/hook-status naming a binary that isn't
`stagehand`), fixes the broken installed-hook script (`exec stagehand` → `exec stagecoach`), and satisfies
the h2.30 rename mandate for the entire `.go` surface.

## Why

- **h2.30 mandates it.** "All references to stagehand must be replaced with stagecoach." M1 (module/imports/
  identifiers) + M2.T1 (env/git-config) landed; THIS task closes the `.go` string/comment surface — the last
  big batch of `stagehand` residue (377 sites).
- **The installed hook is currently BROKEN.** `internal/hook/{hook.go,script.go}` emit
  `exec stagehand hook exec` — the installed prepare-commit-msg hook invokes `stagehand`, a binary that no
  longer exists (it's `stagecoach`). This rename is a functional fix, not cosmetics.
- **Internal consistency.** Error messages, hook status, backup filenames, comments all naming `stagehand`
  while everything else is `stagecoach` is confusing for users and developers alike.
- **The broad sed is the RIGHT mechanism for a 377-site scattered residue.** Hand-editing 377 sites across
  ~60 files is error-prone; a uniform sed (verified safe — comments/strings only) + `go test` gate is
  deterministic and self-verifying.
- **No new surface/deps/docs.** Content-only rename; go.mod unchanged; docs are M4.

## What

A uniform `stagehand`→`stagecoach` / `Stagehand`→`Stagecoach` string+comment rename across all `.go` files
under `internal/`, `cmd/`, `pkg/`, via a broad sed. No identifier/import/module changes (already done). No
non-`.go` files. No new files. The verification is zero residue + build + test.

### Success Criteria

- [ ] `grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l` → **0**
      (the contract's OUTPUT gate; case-insensitive catches any `STAGEHAND` straggler too).
- [ ] `go build ./...` clean (proves no identifier/import corrupted — all residue was comments/strings).
- [ ] `go test ./... -count=1` green (proves semantic strings + their test assertions renamed in lockstep).
- [ ] `gofmt -l internal/ cmd/ pkg/` clean (sed doesn't change Go structure, but run gofmt to be safe).
- [ ] go.mod/go.sum byte-unchanged (content-only; module already stagecoach).
- [ ] NO non-`.go` file touched: README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml,
      .github/workflows/*, FUTURE_SPEC.md, plan/* all UNCHANGED (M3/M4/M5 scope).
- [ ] The installed-hook script content is `exec stagecoach hook exec` (was `exec stagehand hook exec`) —
      matching the `cmd/stagecoach` binary.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the broad-sed command (quoted),
the real-scope finding (377, not ~20), the safety proof (all comments/strings; no identifiers/imports), the
semantic-string lockstep rationale, the scope fences (.go-only; docs/Makefile/etc. = M3/M4), the codebase-
location note, and the 4 verification gates. No feature knowledge beyond "rename stagehand→stagecoach."

### Documentation & References

```yaml
# MUST READ — the authoritative research (real scope + safety + mechanism + gates)
- docfile: plan/012_963e3918ec08/P1M2T3S1/research/stagehand-go-catchall-rename.md
  why: §1 (the real 377-site scope vs the title's ~20), §2 (WHY the broad sed is safe — all comments/strings,
       no identifiers/imports; no STAGEHAND), §3 (the semantic strings + production/test lockstep table;
       the hook-script-is-a-fix note), §4 (subsumes sibling S2's categories), §5 (overlap with parallel
       .stagehandignore — same direction, no conflict), §6 (scope fences), §7 (the mechanism + 4 gates).
  critical: §1 (follow the mechanism + zero-residue gate, NOT the title's ~20 count) + §2 (sed can't break
       identifiers — none remain) + §3 (hook script `exec stagehand` → `exec stagecoach` is a NECESSARY fix).

# The rename surface map (Layer 3 = the user-facing CLI surface; the categories)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: "Layer 3" (3.1 root cmd name, 3.2 error prefixes, 3.3 progress/status, 3.4 session id, 3.5 temp
       dir, 3.6 hook script, 3.7 git alias, 3.8 lazygit marker, 3.9 bootstrap template, 3.10 CLI help,
       3.11 exit/lock messages).
  why: maps every category the broad sed covers. Confirms M1.T2 already renamed the IDENTIFIERS (StatusStagecoach,
       stagecoachAliasValue, lazygitMarker="stagecoach-integration") + most VALUES — what remains is comments
       + straggler strings. The broad sed is the catch-all across all of Layer 3.
  critical: 3.6 (the hook script + status string are stragglers M1.T2 missed — sed fixes them; the script
       rename is a functional fix). 3.4/3.5/3.9 are nominally sibling P1.M2.T3.S2's — the broad sed subsumes them.

# The PRD basis
- file: PRD.md h2.30 — "this project was originally named stagehand and has been renamed. All references to
       stagehand must be replaced with stagecoach."
  why: the rename mandate. This task closes the .go string/comment surface of that mandate.
- file: PRD.md §15.2 (h3.72 Global flags) + §9.13 (h3.29 Verbosity) + §18.1 (h3.88 The invariant).
  why: the selected PRD context. The flags table already uses stagecoach_* env/git-config (M2.T1); the error/
       status strings this task renames are the user-facing counterparts.

# The parallel task (overlap proof — same direction, no conflict)
- docfile: plan/012_963e3918ec08/P1M2T2S2/PRP.md
  why: the parallel .stagehandignore task edits cmd/root.go:164 + ui/verbose.go:101. The broad sed ALSO
       converts `.stagehandignore`→`.stagecoachignore` (token contains "stagehand"). Same direction → identical
       end state → no conflict (git auto-resolves an identical edit). The codebase is at
       /home/dustin/projects/stagehand (NOT /stagecoach — plan-only); module is already stagecoach.
  critical: work in /home/dustin/projects/stagehand. If both tasks edit root.go:164, the result is identical.

# READ-ONLY proof (the semantic stragglers + their test assertions — sed coordinates them)
- file: internal/hook/hook.go   (L30 `return "stagehand (v1)"`; L120/122 `exec stagehand hook exec`)
- file: internal/hook/script.go (L35/37 `exec stagehand hook exec`)
- file: internal/hook/hook_test.go (L232 `{StatusStagecoach, "stagehand (v1)"}`) + internal/hook/script_test.go
       + internal/cmd/hook_test.go (L150/227/403) + internal/hooks/runner_test.go (L436)
  why: PROOF the straggler status/script strings have coordinated test assertions. A uniform sed renames both
       sides identically → tests pass. The hook-script rename is a functional fix (binary is stagecoach).
- file: internal/integrate/protocol.go (L131 `"%s.stagehand-backup.%d"`) + protocol_test.go (L357/557) +
       internal/cmd/integrate_lazygit_test.go (L522/523)
  why: PROOF the backup-suffix string + its test assertions rename in lockstep.
- file: cmd/stagecoach/main.go (L67 `fmt.Fprintf(os.Stderr, "stagehand: %v\n", err)`)
  why: the top-level error printer — the `stagehand:` prefix on every main-path error. Sed → `stagecoach:`.
```

### Current Codebase tree (relevant slice)

```bash
# Codebase root: /home/dustin/projects/stagehand   (module github.com/dustin/stagecoach; on-disk name unchanged)
# ~60 .go files across these dirs contain 'stagehand'/'Stagehand' (377 total occurrences):
internal/   cmd/   pkg/    # ALL .go files with residue — broad sed (content-only: comments + string literals)
go.mod / go.sum    # unchanged (module already stagecoach; content-only rename)
# NOT touched: README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows/*,
#              FUTURE_SPEC.md, plan/*, bin/* — owned by M3/M4/M5.
```

### Desired Codebase tree with files to be added

```bash
# NO new files. In-place content rename across ~60 existing .go files (comments + string literals only).
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (follow the MECHANISM + zero-residue gate, NOT the title's ~20 count): the real residue is 377
# across ~60 files. Limiting to the ~20 error-prefix sites (Layer 3.2) leaves ~357 residue and FAILS the
# OUTPUT gate. The broad sed is the contract's chosen mechanism for exactly this reason.

# CRITICAL (the sed is SAFE — no identifiers/imports remain): M1.T1 (imports/module) + M1.T2 (identifiers)
# + M2.T1 (env vars STAGECOACH_* + git-config stagecoach.*) already landed. Verified: all 377 hits are
# comments + string literals. The sed CANNOT corrupt an identifier or import (none left). go build is the gate.

# CRITICAL (no STAGEHAND all-caps residue): M2.T1.S1 renamed env vars. grep 'STAGEHAND' → empty. The two-
# variant sed (stagehand/Stagehand) covers everything. (If a STAGEHAND straggler appears, the -rni gate
# catches it → add s/STAGEHAND/STAGECOACH/g.)

# CRITICAL (semantic strings rename in lockstep): a uniform sed across ALL .go (production + test) renames
# a production string AND its test assertion IDENTICALLY → they stay matched → go test passes. Do NOT sed
# only production files (that would desync assertions). sed internal/ + cmd/ + pkg/ together.

# CRITICAL (the hook-script rename is a NECESSARY fix): internal/hook emits `exec stagehand hook exec` but
# the binary is cmd/stagecoach → installed hooks currently invoke a non-existent binary. Sed → `exec stagecoach
# hook exec` fixes it. Do NOT preserve `exec stagehand` "to avoid changing behavior" — the current behavior
# is BROKEN.

# GOTCHA (.go-ONLY): the sed is --include='*.go'. README.md/docs/*.md still reference stagehand INTENTIONALLY
# (P1.M4 owns docs). Makefile/.goreleaser/CI = P1.M3. plan/ = P1.M5.T1. A repo-wide sed without --include
# would clobber all of those.

# GOTCHA (codebase location): work in /home/dustin/projects/stagehand (the on-disk codebase; module already
# github.com/dustin/stagecoach). /home/dustin/projects/stagecoach is the PLAN-staging dir (only plan/).
# (Matches the parallel P1.M2.T2.S2 PRP's note.)

# GOTCHA (the sed subsumes siblings): it converts .stagehandignore→.stagecoachignore (parallel P1.M2.T2.S2)
# AND the session-id/temp/bootstrap categories (future P1.M2.T3.S2). Same direction → identical end state →
# no conflict. Flag for the orchestrator: if those siblings should remain meaningful, re-sequence them first.

# GOTCHA (goftest after sed): sed doesn't change Go structure, but run `gofmt -w` on touched files to be
# safe (a sed could in theory disturb alignment in a multi-line string concat — unlikely, but gofmt is free).
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A content-only string/comment rename via sed.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PRE-CHECK — confirm the real scope + safety (don't trust the title's ~20)
  - RUN (from /home/dustin/projects/stagehand):
      grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # EXPECT ~377
      grep -rn 'STAGEHAND' --include='*.go' . | grep -v '.git/'                                      # EXPECT empty
  - IF the count is ~0: the rename already landed (a sibling finished it) — verify go build + go test
    pass and STOP (this task is a no-op). IF STAGEHAND has hits: add s/STAGEHAND/STAGECOACH/g to Task 1.

Task 1: THE broad sed (the contract's mechanism)
  - RUN (from /home/dustin/projects/stagehand):
      grep -rl 'stagehand\|Stagehand' --include='*.go' internal/ cmd/ pkg/ \
        | xargs sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'
  - This converts ALL 377 occurrences (comments + error-prefix strings + status strings + hook script +
    backup suffix + session id + temp prefixes + bootstrap template + CLI help text) in ONE pass.
  - GOTCHA: sed internal/ cmd/ pkg/ TOGETHER (production + test in lockstep). Do NOT sed non-.go files.
    Do NOT sed the plan/ dir. Do NOT sed .git/ or .pi-subagents/ (the grep -rl + --include='*.go' excludes
    them, but the xargs form is safe regardless since grep -rl only lists .go matches).

Task 2: gofmt the touched files (defensive — sed doesn't change structure, but free)
  - RUN: gofmt -w $(grep -rl 'stagecoach' --include='*.go' internal/ cmd/ pkg/)  # or just gofmt -w internal/ cmd/ pkg/
  - (Optional; sed on string literals/comments shouldn't disturb gofmt alignment. Included for safety.)

Task 3: VERIFY — the 4 gates (deterministic)
  - GATE 1 (zero residue): grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l  → MUST be 0
  - GATE 2 (compiles): go build ./...   → clean (proves no identifier/import corrupted)
  - GATE 3 (tests coordinated): go test ./... -count=1   → green (proves semantic strings == test assertions)
  - GATE 4 (format): gofmt -l internal/ cmd/ pkg/   → clean
  - GATE 5 (scope): git diff --name-only | grep -vE '\.go$' → EMPTY (no non-.go file touched)
  - GATE 6 (deps): git diff --exit-code go.mod go.sum → clean
```

### Implementation Patterns & Key Details

```bash
# THE entire change (one broad sed). From /home/dustin/projects/stagehand:
grep -rl 'stagehand\|Stagehand' --include='*.go' internal/ cmd/ pkg/ \
  | xargs sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'

# WHY this is safe + correct:
#  - M1.T1/M1.T2/M2.T1 already renamed identifiers + imports + module + env vars + git-config keys.
#  - The remaining 377 hits are ALL comments + string literals (grep-verified; no identifiers/imports).
#  - A uniform sed renames production strings AND their test assertions IDENTICALLY → tests stay coordinated.
#  - The hook-script `exec stagehand hook exec` → `exec stagecoach hook exec` is a NECESSARY fix (binary =
#    cmd/stagecoach; the current installed-hook script invokes a non-existent binary name).

# THE 4 gates (the contract's OUTPUT, made deterministic):
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # GATE 1: 0
go build ./...        # GATE 2: clean
go test ./... -count=1   # GATE 3: green (-count=1 disables cache)
gofmt -l internal/ cmd/ pkg/   # GATE 4: clean

# SCOPE proof (only .go changed; docs/Makefile/goreleaser/CI/plan untouched):
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go file touched" || echo "only .go files (good)"
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — content-only string/comment rename; module already stagecoach (M1).
      go mod tidy is a no-op. git diff --exit-code go.mod go.sum MUST be empty.

PACKAGE EDGES: NONE — no import changes (M1 owned imports; verified none remain). The rename is string/
      comment content only.

FROZEN / NOT-EDITED:
  - Identifiers / import paths / module path: ALREADY renamed (M1.T1/M1.T2). The sed must NOT (and cannot)
    touch them — verified all 377 residue hits are comments/strings.
  - Non-.go files: README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows/*,
    FUTURE_SPEC.md → P1.M3 (build/CI) + P1.M4 (docs). These still reference stagehand INTENTIONALLY.
  - plan/ artifacts (this PRP, research, rename_surface_map, prd_snapshot, tasks.json) → P1.M5.T1.
  - bin/* + root binaries → build artifacts (rebuild clean).

DOWNSTREAM / SIBLINGS (same direction → no conflict; this task subsumes them if it lands first):
  - P1.M2.T2.S2 (parallel, .stagehandignore): cmd/root.go:164 + ui/verbose.go:101. Sed converts the token.
  - P1.M2.T3.S2 (future, session-id/temp/bootstrap): multiturn.go, temp prefixes, bootstrap.go. Sed converts.
  - P1.M3 (Makefile/.goreleaser/CI), P1.M4 (README/docs), P1.M5 (plan/ + final grep audit) — non-.go; not
    touched here. P1.M5.T2.S1 is the final "zero stagehand in tracked files" audit that catches any straggler.

NO DATABASE / NO ROUTES / NO CONFIG LOGIC CHANGE (env vars + git-config keys already renamed M2.T1; this
task is error-prefix/status-string/comment content).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
gofmt -w $(grep -rl 'stagecoach' --include='*.go' internal/ cmd/ pkg/) 2>/dev/null || gofmt -w internal/ cmd/ pkg/
test -z "$(gofmt -l internal/ cmd/ pkg/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...     # catches any structural disturbance (none expected — comments/strings only).
go build ./...   # GATE 2: proves no identifier/import corrupted (the CAUTION resolution).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
```

### Level 2: Unit Tests (the coordination gate — production strings == test assertions)

```bash
cd /home/dustin/projects/stagehand
go test ./... -count=1
# Expected: ALL PASS. -count=1 disables the test cache (forces a real run). This is the proof that every
# semantic string (hook status, hook script, backup suffix, error prefixes, etc.) and its test assertion
# renamed in lockstep. A divergence (impossible under a uniform sed, but defensive) fails here.
```

### Level 3: Integration Testing (the zero-residue + scope gates)

```bash
cd /home/dustin/projects/stagehand
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# GATE 1 (zero residue — the contract's OUTPUT):
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # MUST be 0
# GATE 5 (scope — only .go changed; docs/Makefile/goreleaser/CI/plan untouched):
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go file touched" || echo "only .go files (good)"
# Smoke: the installed-hook script + error prefix now say stagecoach:
go build -o /tmp/stagecoach ./cmd/stagecoach && /tmp/stagecoach --help 2>&1 | head -1   # should show "stagecoach"
# Confirm the hook status string + script content (the straggler semantic strings):
grep -n 'stagecoach (v1)' internal/hook/hook.go && echo "status string renamed (good)"
grep -n 'exec stagecoach hook exec' internal/hook/hook.go internal/hook/script.go && echo "hook script renamed (good — binary is stagecoach)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagehand
# THE lint gate (content-only rename; no lint drift expected):
make lint 2>&1 | tail -5
# Cross-platform build (Windows compiles too — the rename is content-only):
GOOS=windows go build ./... && echo "windows build OK"
# NOTE: a repo-wide grep will STILL show stagehand in README.md + docs/*.md + Makefile + .goreleaser.yaml +
# providers/*.toml + plan/* — that is EXPECTED (M3/M4/M5 scope), NOT a failure of this task. This task's gate
# is .go-ONLY (GATE 1). P1.M5.T2.S1 is the final repo-wide zero-residue audit.
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # EXPECT: 0
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l internal/ cmd/ pkg/`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test ./... -count=1` (the coordination proof — semantic strings == assertions).
- [ ] Level 3: GATE 1 zero `.go` residue; GATE 5 only `.go` files changed; go.mod/go.sum unchanged;
      `GOOS=windows go build ./...` OK.
- [ ] Level 4: `make lint` green; repo-wide grep still shows docs/Makefile/etc. residue (EXPECTED — M3/M4/M5).

### Feature Validation
- [ ] `grep -rni 'stagehand' --include='*.go'` → **0** (the contract's OUTPUT gate).
- [ ] Error prefixes say `stagecoach:` (e.g. generate.go `ErrNothingToCommit`, main.go:67).
- [ ] Hook status prints `stagecoach (v1)`; installed-hook script is `exec stagecoach hook exec` (functional fix).
- [ ] Backup suffix is `.stagecoach-backup.`; root `Use: "stagecoach"`; comments say Stagecoach throughout.

### Code Quality Validation
- [ ] The broad sed was applied uniformly (production + test in lockstep) — no desynced assertions.
- [ ] Scope-disciplined: `.go`-only; identifiers/imports (M1) + non-.go (M3/M4) + plan (M5) UNTOUCHED.
- [ ] The title's ~20 underestimate was NOT followed — the real 377-site scope was addressed via the broad sed.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] No docs edited here (README/docs/*.md stagehand refs are P1.M4.T1's scope, per the item's DOCS line).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ **Don't limit to the title's ~20 error-prefix sites.** The real residue is 377 across ~60 files. Limiting
  to Layer 3.2 leaves ~357 residue and FAILS the contract's OUTPUT gate (zero `stagehand` in `.go`). Follow
  the MECHANISM (broad sed) + OUTPUT (zero residue), not the title's count.
- ❌ **Don't sed only production files (skip tests).** A few semantic strings have test assertions (hook
  status `"stagehand (v1)"`, script `exec stagehand hook exec`, backup suffix `.stagehand-backup.`). Seding
  production without tests DESYNCS them → tests fail. Sed `internal/ cmd/ pkg/` together (all .go).
- ❌ **Don't sed non-`.go` files.** The sed is `--include='*.go'`. README.md, docs/*.md, Makefile,
  .goreleaser.yaml, providers/*.toml, .github/workflows, FUTURE_SPEC.md still reference stagehand
  INTENTIONALLY — P1.M3 (build/CI) + P1.M4 (docs) own them. plan/ is P1.M5.T1. A repo-wide `sed` clobbers
  all of those.
- ❌ **Don't preserve `exec stagehand hook exec` "to avoid changing behavior."** The current behavior is
  BROKEN — the binary is `cmd/stagecoach`, so the installed hook invokes a non-existent `stagehand` binary.
  The sed → `exec stagecoach hook exec` is a NECESSARY functional fix.
- ❌ **Don't worry about corrupting identifiers/imports.** Verified: all 377 hits are comments + string
  literals. M1.T1/M1.T2 already renamed identifiers + imports + module. The sed CANNOT touch them (none
  remain). `go build ./...` is the gate that proves it.
- ❌ **Don't forget the all-caps check.** The two-variant sed (`stagehand`/`Stagehand`) covers everything
  TODAY (no `STAGEHAND` residue — M2.T1.S1 did env vars). But the `-rni` (case-insensitive) GATE 1 catches
  any `STAGEHAND` straggler → if found, add `s/STAGEHAND/STAGECOACH/g`.
- ❌ **Don't work in `/home/dustin/projects/stagecoach`.** That's the plan-staging dir (only `plan/`). The
  codebase is at `/home/dustin/projects/stagehand` (module already `github.com/dustin/stagecoach`).
- ❌ **Don't conflate "zero stagehand refs repo-wide" with this task's gate.** A repo-wide grep STILL shows
  stagehand in README.md + docs/*.md + Makefile + .goreleaser.yaml + providers/*.toml + plan/* — that is
  EXPECTED (M3/M4/M5 scope), NOT a failure. This task's gate is `.go`-ONLY (GATE 1).
- ❌ **Don't change go.mod/go.sum or add files.** Content-only string/comment rename across ~60 existing .go files.
- ❌ **Don't skip `go test ./... -count=1`.** It is the coordination gate — the proof that every semantic
  string and its test assertion renamed in lockstep. `-count=1` disables the cache (forces a real run).
- ❌ **Don't skip the PRE-CHECK (Task 0).** If a sibling already finished the rename, the residue count is
  ~0 and this task is a no-op (just verify build+test). Running the sed on an already-clean tree is harmless
  but wasteful; the PRE-CHECK catches the already-done case.

---

## Confidence Score

**9/10** — the contract's own MECHANISM (broad sed) + OUTPUT (zero residue) correctly handle the real 377-site
scope that the title underestimates; the safety is verified (all comments/strings, no identifiers/imports, no
STAGEHAND); the semantic strings rename in lockstep (uniform sed across production+test, gated by
`go test -count=1`); the hook-script rename is a necessary functional fix; and the 4 gates are deterministic.
The -1 reserves for the slim chance a semantic string has a test assertion in a file the `grep -rl` misses
(e.g. a generated/ignored file) — GATE 3 (`go test`) catches any such desync, and the PRE-CHECK + GATE 1
absorb any scope shift since the last grep.
