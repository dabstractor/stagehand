---
name: "P1.M2.T3.S2 — Rename session ID prefix, temp dir prefix, and bootstrap config template (verify S1's broad sed caught them; scoped-fix fallback)"
description: |
  VERIFY-with-targeted-fallback task. Closes 3 specific categories of the `stagehand` → `stagecoach` rename in
  Go source: (A) the multi-turn session-ID prefix (`internal/generate/multiturn.go` `newSessionID()` — PRD
  §9.24 FR-T6), (B) the stubagent temp-dir prefix (`internal/stubtest/stubtest.go`), and (C) the `config init`
  bootstrap config template (`internal/config/bootstrap.go` `bootstrapHeader` constant — PRD §9.17 FR-B1).

  ⚠️ **THE central finding — S1's broad sed has ALREADY subsumed these categories.** The sibling P1.M2.T3.S1 PRP
  (the contract for the parallel task) explicitly states its broad `sed s/stagehand/stagecoach/g;
  s/Stagehand/Stagecoach/g` "ALSO converts the session-id/temp/bootstrap categories (future P1.M2.T3.S2) …
  the broad sed SUBSUMES them if it lands first." Verified at research time: the codebase shows **0** stagehand
  residue in `.go` (`grep -rni 'stagehand' --include='*.go'` → 0), all 3 targets are already `stagecoach`, and
  `go build ./...` is clean. **This task's primary work is therefore VERIFICATION**, with a scoped manual-fix
  fallback IF (and only if) the PRE-CHECK finds S1 did NOT land or missed one of these specific contexts. See
  research §1.

  ⚠️ **THE 3 targets, confirmed converted today** (research §2): (A) `internal/generate/multiturn.go:206`
  `"stagecoach-%d"` + `:208` `"stagecoach-" + hex.EncodeToString(b[:])` (the `newSessionID()` one-run-scope id);
  (B) `internal/stubtest/stubtest.go:49` `os.MkdirTemp("", "stagecoach-stubagent-*")`; (C) `internal/config/
  bootstrap.go:236-269` `bootstrapHeader` — fully `stagecoach` (`# Stagecoach configuration file`, `stagecoach
  config init`, `STAGECOACH_*`, `.stagecoach.toml`, `stagecoach.*`). The contract's verification checklist
  (item 3c: `"# Stagecoach configuration file"`, `"stagecoach config init"`, `"STAGECOACH_"`, `".stagecoach.toml"`,
  `"stagecoach.*"`) ALL present.

  ⚠️ **THE 2 test-coordination points — a manual fix MUST keep them in lockstep** (research §3). (3a) The
  session-id FORMAT is asserted by `sessionIDRe = regexp.MustCompile(\`stagecoach-[0-9a-f]{32}\`)` at
  `internal/generate/generate_multiturn_test.go:218` (used at L154 to assert the id is stable across all N+1
  turns). If `newSessionID()` is manually fixed but this regex stays `stagehand-`, `TestMultiTurn` finds zero
  ids → fails. (3b) The bootstrap template is exercised by `internal/cmd/config_test.go:816` ("config init —
  populated bootstrap") — its expected substrings must match `bootstrapHeader`. S1's broad sed renamed BOTH
  sides identically → coordinated today. A scoped fallback fix MUST touch the test file too (research §6).

  ⚠️ **THE fallback mechanism is a SCOPED sed on the 3 contract files (+ their test files), NOT a repo-wide
  re-sed.** S1 (P1.M2.T3.S1) owns the broad catch-all. This task owns ONLY these 3 named categories. If the
  PRE-CHECK finds residue on these files, apply the three-variant sed (`stagehand`/`Stagehand`/`STAGEHAND` —
  the last because `bootstrapHeader` contains `STAGECOACH_*` literals) to ONLY: multiturn.go, stubtest.go,
  bootstrap.go, generate_multiturn_test.go, config_test.go, config_init_interactive_test.go. Today (S1 landed)
  this is a NO-OP. See research §6.

  ⚠️ **THE codebase is at `/home/dustin/projects/stagehand`** (module already `github.com/dustin/stagecoach`;
  on-disk dir name unchanged). `/home/dustin/projects/stagecoach` exists but holds only a plan/ snapshot — it is
  NOT the codebase. All commands run from `/home/dustin/projects/stagehand`. See research §0.

  Deliverable: (1) PRE-CHECK the residue state; (2) if 0 → verify the 3 targets + 2 coordinated tests via the
  spot-check gates and STOP (verify-only no-op); (3) if residue found → scoped three-variant sed on the 3
  contract files + their test files + gofmt; (4) the 5 verification gates. NO new files, NO non-`.go` edits, NO
  identifier/import/module changes, NO go.mod/go.sum change, NO docs. OUTPUT: zero `stagehand` (case-insensitive)
  in any `.go` file; the 3 targets confirmed `stagecoach`; `go build ./...` + `go test ./... -count=1` pass.
  DOCS: none (string literals internal to Go source; the bootstrap template's user-visible content is tested by
  config init tests).
---

## Goal

**Feature Goal**: Confirm (and, only if needed, complete) the `stagehand` → `stagecoach` rename for the 3
specific Go-source string categories this task owns: the multi-turn session-ID prefix, the stubagent temp-dir
prefix, and the `config init` bootstrap config template — so every string literal in these areas uses
`stagecoach` (case-appropriate), matching the renamed module/binary/env-vars/git-config-keys from M1/M2.T1.

**Deliverable** (verify-first; scoped-fix fallback only if a straggler is found):
1. A PRE-CHECK measuring the current `.go` residue count (expected 0 — S1's broad sed already landed).
2. Spot-check verification of the 3 targets (multiturn.go session id, stubtest.go temp-dir prefix, bootstrap.go
   `bootstrapHeader`) and the 2 coordinated test assertions (`sessionIDRe` regex, `config init` test).
3. **IF** a straggler is found: a scoped three-variant sed (`s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g;
   s/STAGEHAND/STAGECOACH/g`) on ONLY the 3 contract files + their coordinated test files, then `gofmt -w`.
4. The 5 verification gates (zero residue; 3 targets confirmed; compiles; tests green; scope = .go only).

**Success Definition**: `grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'`
returns ZERO; the 3 contract targets confirmed `stagecoach` via the spot-check greps; `go build ./...` clean;
`go test ./... -count=1` green (proves the session-id regex + config init test stayed coordinated with
production); go.mod/go.sum byte-unchanged; NO non-`.go` file touched (docs/Makefile/goreleaser/CI/plan = M3/M4/M5).

## User Persona

**Target User**: Every user + developer who sees these strings. The session-id prefix (`stagecoach-<32hex>`) is
logged in each multi-turn turn's argv (diagnostic — visible in `--verbose`); the temp-dir prefix names the
stubagent binary's `/tmp` dir during tests; the bootstrap template is the file `stagecoach config init` writes
(the first thing a new user sees). After this task all three say `stagecoach`, consistent with the binary/module/
env/config identity. Transitively PRD h2.30 (the rename mandate).

**Use Case**: A user runs `stagecoach config init` and gets a `# Stagecoach configuration file` populated with
`STAGECOACH_*` env-var guidance + `.stagecoach.toml` path notes (not stale `stagehand`/`STAGEHAND`). A developer
inspecting a multi-turn run's verbose log sees `stagecoach-<32hex>` session ids. Tests build the stubagent into a
`stagecoach-stubagent-*` temp dir.

**User Journey**: `stagecoach config init` → writes the bootstrap template (all `stagecoach`/`STAGECOACH_*`) →
the generated config is immediately consistent with the tool's identity. (Today — after S1 — this is already true;
this task is the verify-and-confirm gate for these 3 categories.)

**Pain Points Addressed**: closes the rename for these 3 specific string categories; satisfies the h2.30 mandate
for them; the verify gate gives the maintainer confidence S1's broad sed didn't miss a context here.

## Why

- **h2.30 mandates it.** "All references to stagehand must be replaced with stagecoach." M1 (module/imports/
  identifiers) + M2.T1 (env/git-config) landed; S1 (the broad `.go` catch-all) is in flight and explicitly
  subsumes these 3 categories. THIS task is the verify-and-confirm for the 3 categories the contract names
  explicitly — it exists to catch any straggler S1's broad sed might have missed in these specific contexts.
- **The contract is explicit that this is a verify-first task.** Item 3 LOGIC: "(a) Verify the broad sed from S1
  caught the bootstrap template, session IDs, and temp dir prefix. (b) If any were missed (e.g., the sed pattern
  missed a specific context), fix manually." The verify gate IS the deliverable; the fix is the fallback.
- **The session-id format has a coordinated test.** `sessionIDRe` at `generate_multiturn_test.go:218` asserts
  `stagecoach-[0-9a-f]{32}`. A manual fix to `newSessionID()` without updating the regex breaks `TestMultiTurn`.
  This PRP documents the lockstep so the fallback fix doesn't desync it.
- **The bootstrap template is user-facing (config init output).** It is the first artifact a new user sees; a
  stale `stagehand`/`STAGEHAND` here is the most visible rename gap. Its content is covered by config init tests.
- **No new surface/deps/docs.** String-literal content rename (or verify-no-op); go.mod unchanged; docs are M4.

## What

A verify-first check of the 3 contract categories, with a scoped three-variant sed fallback on ONLY the 3
contract files (+ their coordinated test files) IF the PRE-CHECK finds residue. No identifier/import/module
changes (already done by M1). No non-`.go` files. No new files. The verification is zero residue + the 3 target
spot-checks + build + test.

### Success Criteria

- [ ] `grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l` → **0**
      (the contract's OUTPUT gate; case-insensitive catches any `STAGEHAND` straggler too).
- [ ] `internal/generate/multiturn.go`: `newSessionID()` returns `"stagecoach-" + hex…` (L208) and the time
      fallback `"stagecoach-%d"` (L206); comment at L199 says `stagecoach-<32 hex>`.
- [ ] `internal/stubtest/stubtest.go:49`: `os.MkdirTemp("", "stagecoach-stubagent-*")`.
- [ ] `internal/config/bootstrap.go` `bootstrapHeader` (L236-269): contains `"# Stagecoach configuration file"`,
      `"stagecoach config init"`, `STAGECOACH_*` (env-var guidance), `.stagecoach.toml`, `stagecoach.*`.
- [ ] `internal/generate/generate_multiturn_test.go:218`: `sessionIDRe = regexp.MustCompile(\`stagecoach-[0-9a-f]{32}\`)`
      (the coordinated test assertion — matches `newSessionID()`).
- [ ] `go build ./...` clean; `go test ./... -count=1` green (proves session-id regex + config init test stayed
      coordinated with production); `gofmt -l` clean; go.mod/go.sum byte-unchanged; NO non-`.go` file touched.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the current-state finding (S1's broad
sed already converted all 3 targets → likely a verify-only no-op), the exact line numbers + confirmed content of
each target, the 2 test-coordination points (sessionIDRe regex + config init test), the scoped-fallback sed
command + the file list it applies to, the codebase location, and the 5 verification gates. No feature knowledge
beyond "rename stagehand→stagecoach in these 3 string categories."

### Documentation & References

```yaml
# MUST READ — the authoritative research (current state + the 3 targets + the 2 coordination points + the fallback)
- docfile: plan/012_963e3918ec08/P1M2T3S2/research/session-id-temp-bootstrap-rename.md
  why: §0 (codebase at /home/dustin/projects/stagehand, NOT /stagecoach), §1 (CURRENT STATE: S1's broad sed
       already ran → 0 residue → this is verify-first), §2 (the 3 targets with exact line numbers + confirmed
       content), §3 (the 2 test-coordination points — sessionIDRe regex + config init test), §4 (no test asserts
       the temp-dir prefix — diagnostic only), §5 (parallel S2 .stagecoachignore already landed — confirms the
       sed-subsumes-siblings dynamic), §6 (the scoped fallback sed + file list), §7 (the 5 gates), §8 (scope fences).
  critical: §1 (verify-first; the fix is a fallback, not the default) + §3 (the lockstep rule — a manual fix on
       multiturn.go MUST also update sessionIDRe at test:218, or TestMultiTurn fails) + §6 (scoped to ONLY these
       files, NOT a repo-wide re-sed).

# THE contract for the parallel task that subsumes these categories (S1)
- docfile: plan/012_963e3918ec08/P1M2T3S1/PRP.md
  why: S1's broad sed "ALSO converts the session-id/temp/bootstrap categories (future P1.M2.T3.S2) … the broad
       sed subsumes them if it lands first." This task is the verify-and-confirm for exactly those categories.
       If S1 landed (the verified current state), this task is a verify-only no-op. If S1 did NOT land, apply
       this task's scoped fallback (research §6) — do NOT re-run S1's repo-wide sed (that's S1's scope).
  critical: the relationship is subsumption, not duplication. This task does NOT re-sed all ~60 files — only the
       3 contract files (+ their tests) IF a straggler is found.

# THE rename surface map (the category taxonomy — Layer 3.4/3.5/3.9 are this task's)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: "Layer 3" — 3.4 (session id), 3.5 (temp dir), 3.9 (bootstrap template).
  why: maps the 3 categories this task owns. Confirms M1.T2 renamed the IDENTIFIERS + most VALUES; what this
       task verifies is the straggler STRING literals in these 3 contexts.

# THE production files (the 3 targets — verify; edit ONLY if a straggler is found)
- file: internal/generate/multiturn.go
  section: newSessionID() (L201-209) — L206 `"stagecoach-%d"` (time fallback), L208 `"stagecoach-" + hex.EncodeToString(b[:])`
           (primary crypto/rand), L199 comment `"stagecoach-<32 hex>"`.
  why: target (A) — the multi-turn session-ID prefix (PRD §9.24 FR-T6). One-run-scope; logged in each turn's argv.
  pattern: the prefix is a string literal concatenated with hex-encoded random bytes. A fix changes the literal.
  gotcha: the FORMAT is asserted by sessionIDRe at generate_multiturn_test.go:218 — a fix here MUST update that
           regex in lockstep (research §3a) or TestMultiTurn finds zero ids.
- file: internal/stubtest/stubtest.go
  section: L49 `os.MkdirTemp("", "stagecoach-stubagent-*")`.
  why: target (B) — the stubagent temp-dir prefix. Diagnostic only (names the /tmp dir for the fake-agent binary).
  gotcha: NO test asserts the prefix STRING (research §4) — a fix here has no coordinated test; pure cosmetic.
- file: internal/config/bootstrap.go
  section: bootstrapHeader constant (L236-269) — the entire generated config template.
  why: target (C) — the `config init` populated bootstrap (PRD §9.17 FR-B1). Contains `# Stagecoach configuration
       file`, `stagecoach config init`, `STAGECOACH_*` env-var guidance, `.stagecoach.toml`, `stagecoach.*`.
  pattern: a raw-string + concatenation constant; the rename touches Stagehand/stagehand/STAGEHAND variants.
  gotcha: the rendered content is exercised by internal/cmd/config_test.go:816 ("config init — populated
           bootstrap") + config_init_interactive_test.go — a fix here MUST keep those tests' expected substrings
           coordinated (research §3b). S1's broad sed already did (both sides renamed identically).

# THE coordinated test files (verify; edit ONLY if the production file is manually fixed)
- file: internal/generate/generate_multiturn_test.go
  section: L216-218 — `sessionIDRe = regexp.MustCompile(\`stagecoach-[0-9a-f]{32}\`)` (used at L154 to assert the
           id is STABLE across all N+1 turns).
  why: the session-id FORMAT assertion. MUST match newSessionID()'s prefix (research §3a).
- file: internal/cmd/config_test.go
  section: L816 — "(1) config init — populated bootstrap" test.
  why: exercises bootstrapHeader content; its expected substrings MUST match the constant (research §3b).

# THE PRD basis
- file: PRD.md h2.30 — "this project was originally named stagehand and has been renamed. All references to
       stagehand must be replaced with stagecoach."
  why: the rename mandate. This task is the verify-and-confirm for 3 specific string categories of that mandate.
- file: PRD.md §9.17 (h3.33 FR-B1) — config init writes a populated, working config (the bootstrapHeader purpose).
- file: PRD.md §9.24 (h3.40 FR-T6) — multi-turn session id `stagecoach-<run-uuid>` (the newSessionID() purpose).
```

### Current Codebase tree (relevant slice)

```bash
# Codebase root: /home/dustin/projects/stagehand   (module github.com/dustin/stagecoach; on-disk name unchanged)
internal/generate/multiturn.go            # newSessionID() L201-209 — target (A) session-id prefix   ← verify / scoped-fix
internal/generate/generate_multiturn_test.go  # sessionIDRe L218 — coordinated assertion (A)            ← verify / scoped-fix
internal/stubtest/stubtest.go             # L49 os.MkdirTemp("stagecoach-stubagent-*") — target (B)   ← verify / scoped-fix
internal/config/bootstrap.go              # bootstrapHeader L236-269 — target (C) bootstrap template  ← verify / scoped-fix
internal/cmd/config_test.go               # L816 config init populated-bootstrap — coordinated (C)    ← verify / scoped-fix
internal/cmd/config_init_interactive_test.go  # config init interactive — coordinated (C)             ← verify / scoped-fix
go.mod / go.sum                           # unchanged (content-only; module already stagecoach)
# NOT touched: README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows/*,
#              FUTURE_SPEC.md, plan/* — owned by M3/M4/M5. Identifiers/imports/module already renamed (M1).
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Verify-first; IF a straggler is found, scoped content rename on the 3 contract files + their
# coordinated test files (string literals only; no identifier/import/structure change).
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (verify-FIRST — S1's broad sed already converted these): the codebase at research time shows 0
# stagehand residue in .go, all 3 targets are stagecoach, go build is clean. The PRE-CHECK (Task 0) measures
# this; if 0, the task is a verify-only no-op (run the spot-check gates + build/test + STOP). Do NOT run the
# fallback sed on an already-clean tree (harmless but wasteful, and it overlaps S1's scope).

# CRITICAL (the fallback is SCOPED to these 3 categories, NOT a repo-wide re-sed): S1 (P1.M2.T3.S1) owns the
# broad catch-all across ~60 files. This task owns ONLY session-id / temp-dir / bootstrap-template. If the
# PRE-CHECK finds residue on these files, sed ONLY multiturn.go, stubtest.go, bootstrap.go (+ their test files).
# A repo-wide sed here would duplicate S1's work + risk touching files outside this task's scope.

# CRITICAL (the three-variant sed — STAGEHAND too): bootstrapHeader contains STAGECOACH_* env-var literals.
# A missed sed would leave STAGEHAND_* there. The fallback uses s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g;
# s/STAGEHAND/STAGECOACH/g (the STAGEHAND variant is the one most likely missed by a stagehand/Stagehand-only sed).
# (M2.T1.S1 renamed the env-var CONSUMERS; bootstrapHeader is a TEMPLATE that lists them as guidance text.)

# CRITICAL (the sessionIDRe lockstep — research §3a): newSessionID()'s prefix (multiturn.go:208) ↔ sessionIDRe
# regex (generate_multiturn_test.go:218: `stagecoach-[0-9a-f]{32}`) ↔ the comment at multiturn.go:199. If you
# manually fix one, fix all three, or TestMultiTurn's sessionIDRe.FindAllString (L154) finds zero ids → fails.
# S1's broad sed renamed all three identically → coordinated today.

# CRITICAL (the config init test lockstep — research §3b): bootstrapHeader's rendered content is asserted by
# config_test.go:816 + config_init_interactive_test.go. If you manually fix bootstrap.go, the test's expected
# substrings must match. S1's broad sed renamed both → coordinated today.

# GOTCHA (no test asserts the temp-dir PREFIX string — research §4): stubtest.go:49's "stagecoach-stubagent-*"
# is diagnostic only. No test greps for the prefix. A fix here is pure cosmetic (no coordinated test to update).
# (e2e/harness_test.go:157's writeStubConfig is UNRELATED — it writes a TOML config to t.TempDir, not the stubagent binary dir.)

# GOTCHA (.go-ONLY): the verify + fallback gates are --include='*.go'. README.md/docs/*.md still reference
# stagehand INTENTIONALLY (P1.M4 owns docs). Makefile/.goreleaser/CI = P1.M3. plan/ = P1.M5.T1. A repo-wide grep
# STILL shows stagehand in those — EXPECTED, not a failure of this task. This task's gate is .go-ONLY.

# GOTCHA (codebase location): work in /home/dustin/projects/stagehand (the on-disk codebase; module already
# github.com/dustin/stagecoach). /home/dustin/projects/stagecoach is the plan-staging dir (only plan/). (Matches
# the S1 PRP's gotcha + research §0.)

# GOTCHA (the parallel .stagecoachignore sibling already landed): root.go:164 + verbose.go:101 already say
# .stagecoachignore (S1's broad sed subsumed the token, or P1.M2.T2.S2 ran). NOT this task's concern, but confirms
# the sed-subsumes-siblings dynamic — do NOT re-touch those files here.

# GOTCHA (goftest after any fallback sed): run `gofmt -w` on any file the sed touched. A sed on a raw-string
# constant (bootstrapHeader) shouldn't disturb gofmt alignment, but gofmt is free + defensive.
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A verify-first check of 3 string-literal categories, with a scoped content-rename
fallback.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PRE-CHECK — measure the current residue state (decides verify-only vs scoped-fix)
  - RUN (from /home/dustin/projects/stagehand):
      grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l
      grep -rn 'STAGEHAND' --include='*.go' internal/generate/multiturn.go internal/stubtest/stubtest.go internal/config/bootstrap.go
  - IF the count is 0 AND the STAGEHAND grep is empty: S1 already converted these → SKIP Task 1's sed → go to
    Task 2 (the verify gates). This is the EXPECTED path (S1's broad sed subsumed these categories).
  - IF the count is >0 OR STAGEHAND has hits on the 3 contract files: go to Task 1 (the scoped fallback fix).

Task 1: THE scoped fallback fix (ONLY if Task 0 found a straggler on the 3 contract files)
  - RUN (from /home/dustin/projects/stagehand) — three-variant sed on ONLY the 3 contract files + their
    coordinated test files (research §6):
      sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' \
        internal/generate/multiturn.go \
        internal/stubtest/stubtest.go \
        internal/config/bootstrap.go \
        internal/generate/generate_multiturn_test.go \
        internal/cmd/config_test.go \
        internal/cmd/config_init_interactive_test.go
  - THEN gofmt the touched files: gofmt -w internal/generate/multiturn.go internal/stubtest/stubtest.go \
      internal/config/bootstrap.go internal/generate/generate_multiturn_test.go internal/cmd/config_test.go \
      internal/cmd/config_init_interactive_test.go
  - GOTCHA: the three-variant sed (incl. STAGEHAND) is REQUIRED because bootstrapHeader lists STAGECOACH_* env
    vars as guidance text — a stagehand/Stagehand-only sed would miss STAGEHAND_* there. Touching the test files
    TOO keeps production + assertions in lockstep (research §3). Do NOT sed non-.go files; do NOT sed other .go
    files (S1 owns those).

Task 2: VERIFY — the 5 gates (deterministic; the task's real deliverable)
  - GATE 1 (zero residue): grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l  → MUST be 0
  - GATE 2 (the 3 targets spot-checked):
      grep -n 'stagecoach-%d\|stagecoach-" + hex' internal/generate/multiturn.go                      # 2 hits (L206, L208)
      grep -n 'stagecoach-stubagent-\*' internal/stubtest/stubtest.go                                  # 1 hit (L49)
      grep -nE '# Stagecoach configuration file|stagecoach config init|STAGECOACH_|\.stagecoach\.toml|stagecoach\.\*' internal/config/bootstrap.go  # many
  - GATE 3 (the 2 coordinated test assertions):
      grep -n 'stagecoach-\[0-9a-f\]{32}' internal/generate/generate_multiturn_test.go                 # 1 hit (L218 sessionIDRe)
      grep -n 'config init — populated bootstrap' internal/cmd/config_test.go                          # 1 hit (L816)
  - GATE 4 (compiles + tests coordinated): go build ./...  → clean ;  go test ./... -count=1  → green
  - GATE 5 (scope): git diff --name-only | grep -vE '\.go$' → EMPTY ;  git diff --exit-code go.mod go.sum → clean
```

### Implementation Patterns & Key Details

```bash
# THE verify-first flow. From /home/dustin/projects/stagehand:
#   1. PRE-CHECK: count residue. 0 → verify-only (expected, S1 landed). >0 → scoped fallback.
#   2. (fallback only) scoped three-variant sed on the 3 contract files + their test files + gofmt.
#   3. The 5 gates.

# THE scoped fallback sed (research §6) — three variants, ONLY these files:
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' \
  internal/generate/multiturn.go internal/stubtest/stubtest.go internal/config/bootstrap.go \
  internal/generate/generate_multiturn_test.go internal/cmd/config_test.go internal/cmd/config_init_interactive_test.go

# WHY three variants (incl. STAGEHAND): bootstrapHeader (bootstrap.go:236-269) is a TEMPLATE that lists the
# STAGECOACH_* env vars as user guidance. M2.T1.S1 renamed the env-var CONSUMERS (the os.Getenv("STAGECOACH_…")
# call sites); the TEMPLATE listing them as guidance text is a separate string surface. A stagehand/Stagehand-only
# sed would leave STAGEHAND_* literals in bootstrapHeader → GATE 1's -rni (case-insensitive) catches them.
# Touching the test files TOO keeps production + assertions coordinated (the sessionIDRe regex + config init test).

# THE 5 gates (the contract's OUTPUT, made deterministic):
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # GATE 1: 0
grep -n 'stagecoach-%d\|stagecoach-" + hex' internal/generate/multiturn.go                      # GATE 2a: L206 + L208
grep -n 'stagecoach-stubagent-\*' internal/stubtest/stubtest.go                                 # GATE 2b: L49
grep -nE '# Stagecoach configuration file|stagecoach config init|STAGECOACH_|\.stagecoach\.toml|stagecoach\.\*' internal/config/bootstrap.go  # GATE 2c
grep -n 'stagecoach-\[0-9a-f\]{32}' internal/generate/generate_multiturn_test.go                 # GATE 3a: L218
grep -n 'config init — populated bootstrap' internal/cmd/config_test.go                          # GATE 3b: L816
go build ./...        # GATE 4a: clean
go test ./... -count=1   # GATE 4b: green (-count=1 disables cache; the coordination proof)

# SCOPE proof (only the 3 contract files + their tests changed, IF the fallback ran; today: NONE changed):
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go touched" || echo "only .go (or none) — good"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — content-only string-literal rename (or verify-no-op); module already
      stagecoach (M1). go mod tidy is a no-op. git diff --exit-code go.mod go.sum MUST be empty.

PACKAGE EDGES: NONE — no import changes (M1 owned imports; verified none remain). The rename is string-literal
      content only (session-id prefix, temp-dir prefix, bootstrap template text).

FROZEN / NOT-EDITED:
  - Identifiers / import paths / module path: ALREADY renamed (M1.T1/M1.T2). The sed must NOT (and cannot) touch
    them — verified all residue (if any) is string literals.
  - Non-.go files: README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows/*,
    FUTURE_SPEC.md → P1.M3 (build/CI) + P1.M4 (docs). These still reference stagehand INTENTIONALLY.
  - plan/ artifacts (this PRP, research, rename_surface_map, prd_snapshot, tasks.json) → P1.M5.T1.
  - The .stagecoachignore token (root.go:164 + verbose.go:101) → parallel P1.M2.T2.S2 (already landed).
  - Other .go string/comment residue (error prefixes, status strings, hook script, etc.) → P1.M2.T3.S1 (the
    broad catch-all). This task owns ONLY the 3 named categories.

DOWNSTREAM / SIBLINGS:
  - P1.M2.T3.S1 (parallel, broad catch-all): subsumes these 3 categories if it lands first (the verified current
    state). If S1 lands, this task is a verify-only no-op. If S1 has NOT landed when this task runs, the scoped
    fallback (Task 1) handles these 3 categories without duplicating S1's repo-wide scope.
  - P1.M3 (Makefile/.goreleaser/CI), P1.M4 (README/docs), P1.M5 (plan/ + final grep audit) — non-.go; not touched
    here. P1.M5.T2.S1 is the final "zero stagehand in tracked files" audit that catches any straggler repo-wide.

NO DATABASE / NO ROUTES / NO CONFIG LOGIC CHANGE (env-var consumers + git-config keys already renamed M2.T1;
this task verifies the session-id/temp-prefix/bootstrap-TEMPLATE string literals).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
# (Only if Task 1's fallback sed ran — defensive; a sed on a raw-string constant shouldn't disturb gofmt.)
gofmt -w internal/generate/multiturn.go internal/stubtest/stubtest.go internal/config/bootstrap.go \
  internal/generate/generate_multiturn_test.go internal/cmd/config_test.go internal/cmd/config_init_interactive_test.go 2>/dev/null
test -z "$(gofmt -l internal/ cmd/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/generate/ ./internal/stubtest/ ./internal/config/ ./internal/cmd/   # structural check.
go build ./...   # GATE 4a: proves no identifier/import corrupted.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
```

### Level 2: Unit Tests (the coordination gate — production strings == test assertions)

```bash
cd /home/dustin/projects/stagehand
go test ./internal/generate/ ./internal/stubtest/ ./internal/config/ ./internal/cmd/ -count=1   # the affected packages
go test ./... -count=1   # GATE 4b: full module green. -count=1 disables the cache (forces a real run).
# Expected: ALL PASS. This is the proof that the sessionIDRe regex (test:218) matches newSessionID()'s prefix
# (multiturn.go:208) AND the config init test (config_test.go:816) matches bootstrapHeader. A divergence
# (impossible under a uniform sed, but defensive) fails here.
```

### Level 3: Integration Testing (the zero-residue + target spot-checks)

```bash
cd /home/dustin/projects/stagehand
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# GATE 1 (zero residue — the contract's OUTPUT):
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # MUST be 0
# GATE 2 (the 3 targets spot-checked):
grep -n 'stagecoach-%d\|stagecoach-" + hex' internal/generate/multiturn.go                       # L206 + L208
grep -n 'stagecoach-stubagent-\*' internal/stubtest/stubtest.go                                  # L49
grep -nE '# Stagecoach configuration file|stagecoach config init|STAGECOACH_|\.stagecoach\.toml|stagecoach\.\*' internal/config/bootstrap.go
# GATE 3 (the 2 coordinated test assertions):
grep -n 'stagecoach-\[0-9a-f\]{32}' internal/generate/generate_multiturn_test.go                 # L218 sessionIDRe
grep -n 'config init — populated bootstrap' internal/cmd/config_test.go                          # L816
# GATE 5 (scope): only the 3 contract files + their tests changed IF the fallback ran; today NONE changed:
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go file touched" || echo "only .go (or none) — good"
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagehand
# THE lint gate (content-only rename or verify-no-op; no lint drift expected):
make lint 2>&1 | tail -5
# Cross-platform build (the rename is content-only; Windows compiles too):
GOOS=windows go build ./... && echo "windows build OK"
# Smoke: config init writes a stagecoach template (end-to-end of target C):
/tmp/stagecoach config init --provider pi 2>&1 | head -3   # mentions the written path
# NOTE: a repo-wide grep will STILL show stagehand in README.md + docs/*.md + Makefile + .goreleaser.yaml +
# providers/*.toml + plan/* — that is EXPECTED (M3/M4/M5 scope), NOT a failure of this task. This task's gate
# is .go-ONLY (GATE 1). P1.M5.T2.S1 is the final repo-wide zero-residue audit.
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # EXPECT: 0
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l`, `go vet`, `go build ./...`, `go mod tidy` no-op (gofmt/vet only if fallback ran).
- [ ] Level 2 green: `go test ./... -count=1` (the coordination proof — sessionIDRe + config init test match production).
- [ ] Level 3: GATE 1 zero `.go` residue; GATE 2 the 3 targets confirmed; GATE 3 the 2 coordinated tests confirmed;
      GATE 5 only the 3 contract files + tests changed IF the fallback ran (else none); go.mod/go.sum unchanged.
- [ ] Level 4: `make lint` green; `GOOS=windows go build ./...` OK; repo-wide grep still shows docs/Makefile/etc.
      residue (EXPECTED — M3/M4/M5).

### Feature Validation
- [ ] `grep -rni 'stagehand' --include='*.go'` → **0** (the contract's OUTPUT gate).
- [ ] `internal/generate/multiturn.go`: `newSessionID()` returns `stagecoach-<32hex>` (L208) + `stagecoach-%d` (L206).
- [ ] `internal/stubtest/stubtest.go:49`: `os.MkdirTemp("", "stagecoach-stubagent-*")`.
- [ ] `internal/config/bootstrap.go` `bootstrapHeader`: `# Stagecoach configuration file`, `stagecoach config init`,
      `STAGECOACH_*`, `.stagecoach.toml`, `stagecoach.*`.
- [ ] `sessionIDRe` (test:218) == `stagecoach-[0-9a-f]{32}` (matches newSessionID).

### Code Quality Validation
- [ ] Verify-first: the PRE-CHECK decided verify-only vs scoped-fix (didn't blindly re-sed an already-clean tree).
- [ ] Scope-disciplined: `.go`-only; ONLY the 3 contract files + their coordinated tests touched (if at all);
      identifiers/imports (M1) + non-.go (M3/M4) + plan (M5) + the .stagecoachignore sibling UNTOUCHED.
- [ ] The fallback (if it ran) kept production + test in lockstep (sessionIDRe + config init test coordinated).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] No docs edited here (README/docs/*.md stagehand refs are P1.M4.T1's scope, per the item's DOCS line).
- [ ] go.mod/go.sum byte-unchanged; no new files; the bootstrap template's user-visible content is covered by
      config init tests (not by docs).

---

## Anti-Patterns to Avoid

- ❌ **Don't blindly re-run a repo-wide sed.** S1 (P1.M2.T3.S1) owns the broad catch-all across ~60 files. This
  task owns ONLY session-id / temp-dir / bootstrap-template. Run the PRE-CHECK first; if residue is 0 (the
  expected, verified state), the task is verify-only — STOP after the gates. If residue is found on these files,
  sed ONLY the 3 contract files + their coordinated test files (research §6).
- ❌ **Don't skip the PRE-CHECK (Task 0).** It decides verify-only vs scoped-fix. Running the fallback sed on an
  already-clean tree (S1 landed) is wasteful + overlaps S1's scope. The PRE-CHECK count is the decision point.
- ❌ **Don't use a two-variant sed (stagehand/Stagehand only).** `bootstrapHeader` lists `STAGECOACH_*` env vars
  as guidance text — a stagehand/Stagehand-only sed would leave `STAGEHAND_*` literals there → GATE 1's
  case-insensitive `-rni` catches them. Use the THREE-variant sed (incl. `s/STAGEHAND/STAGECOACH/g`).
- ❌ **Don't fix multiturn.go without updating `sessionIDRe`.** The session-id format is asserted by
  `sessionIDRe = regexp.MustCompile(\`stagecoach-[0-9a-f]{32}\`)` at generate_multiturn_test.go:218. A manual
  fix to `newSessionID()` that doesn't update the regex makes `TestMultiTurn` find zero ids → fails. Sed the
  test file in the same pass (research §3a).
- ❌ **Don't fix bootstrap.go without checking the config init test.** `bootstrapHeader`'s rendered content is
  asserted by config_test.go:816 + config_init_interactive_test.go. Sed those test files in the same pass so
  expected substrings stay coordinated (research §3b).
- ❌ **Don't sed non-`.go` files.** The verify + fallback are `--include='*.go'` / scoped to the named `.go`
  files. README.md, docs/*.md, Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows, FUTURE_SPEC.md
  still reference stagehand INTENTIONALLY — P1.M3 (build/CI) + P1.M4 (docs) own them. plan/ is P1.M5.T1.
- ❌ **Don't worry about corrupting identifiers/imports.** M1.T1/M1.T2 already renamed them. The 0-residue grep
  (or the scoped sed on string literals) cannot touch them. `go build ./...` is the gate that proves it.
- ❌ **Don't work in `/home/dustin/projects/stagecoach`.** That's the plan-staging dir (only `plan/`). The
  codebase is at `/home/dustin/projects/stagehand` (module already `github.com/dustin/stagecoach`). (Research §0.)
- ❌ **Don't conflate "zero stagehand refs repo-wide" with this task's gate.** A repo-wide grep STILL shows
  stagehand in README.md + docs/*.md + Makefile + .goreleaser.yaml + providers/*.toml + plan/* — that is EXPECTED
  (M3/M4/M5 scope), NOT a failure. This task's gate is `.go`-ONLY (GATE 1). P1.M5.T2.S1 is the final audit.
- ❌ **Don't touch the .stagecoachignore token.** That's parallel P1.M2.T2.S2 (root.go:164 + verbose.go:101),
  already landed (S1's sed subsumed it, or S2 ran). Re-touching it overlaps a sibling.
- ❌ **Don't change go.mod/go.sum or add files.** Verify-first no-op, or a scoped content rename on the 3 contract
  files + their tests. No new files; no identifier/import/module change.
- ❌ **Don't skip `go test ./... -count=1`.** It is the coordination gate — the proof that the sessionIDRe regex
  + config init test match the production strings. `-count=1` disables the cache (forces a real run).

---

## Confidence Score

**9/10** — the task is a verify-first check of 3 specific, well-located string-literal categories (exact line
numbers + confirmed current content), and the verified current state shows S1's broad sed already converted all
3 (0 residue, build clean) — so the realistic outcome is a verify-only no-op with deterministic spot-check gates.
The scoped fallback (three-variant sed on ONLY the 3 contract files + their 2 coordinated test files) is fully
specified for the case S1 didn't land, with the lockstep rules (sessionIDRe regex + config init test) called out
so a manual fix can't desync the tests. The -1 reserves for the (low) chance S1's broad sed missed a specific
context in one of these files (e.g. a STAGEHAND_* literal in bootstrapHeader that a two-variant sed would skip) —
the three-variant fallback sed + GATE 1's case-insensitive `-rni` catch exactly that.
