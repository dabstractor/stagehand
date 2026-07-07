---
name: "P1.M2.T1.S1 (bugfix Issue 3) — Add stagecoach.noVerify reader in loadGitConfig + fix key name in docs and comments"
description: |
  Bugfix for Issue 3 (Major): the `no_verify` git-config precedence layer is entirely non-functional due to
  TWO defects. (A) `loadGitConfig()` (`internal/config/git.go:110`) never queries any `stagecoach.no*` key
  (grep = 0) — it reads `stagecoach.push` (line 174) but has no `noVerify` reader, so the precedence is only
  4 layers, contradicting the doc comment (config.go:130) and PRD §9.25 FR-V5. (B) The documented key
  `stagecoach.no_verify` is INVALID for git — `git config stagecoach.no_verify true` fails with
  `error: invalid key: stagecoach.no_verify` (git rejects underscores in the final config segment); every
  other multi-word stagecoach git-config key uses camelCase (`autoStageAll`, `maxDiffBytes`, `stripCodeFence`).
  Fix: add a `stagecoach.noVerify` reader mirroring the `push` reader (git.go:174-179), and correct the key
  name in the 4 places that reference the GIT-CONFIG key. TDD: failing tests first, then the reader.

  ⚠️ **THE central design call — camelCase `stagecoach.noVerify` (git-valid, matches convention).** Use
  `stagecoach.noVerify`, NOT `no_verify` (git rejects underscores in the final segment) and NOT `no-verify`
  (breaks the camelCase convention). `gitConfigBool(repoDir, "stagecoach.noVerify")` mirrors the push reader
  (git.go:174) exactly. Place it right after the push block (~line 179), in the bool-keys section.

  ⚠️ **THE second design call — fix the git-config key in 4 places, NOT the TOML key.** The git-config key
  (`stagecoach.no_verify`) and the TOML file key (`[generation].no_verify`) are DIFFERENT namespaces. Change
  ONLY the git-config-key references: docs/cli.md:44, docs/configuration.md:155 (surgical — same line has
  `[generation].no_verify` which STAYS), internal/config/config.go:130 (same surgical rule), AND
  internal/cmd/root.go:219 (the `--no-verify` flag help text — a 4th surface the contract missed; user-facing).
  Do NOT touch the TOML struct tag (`config.go:134 toml:"no_verify"` — TOML allows underscores), the
  `[generation].no_verify` references, overlay(), materialize(), loadEnv, or loadFlags — all correct.

  ⚠️ **THE third design call — TDD: failing tests FIRST, then the reader.** (1) Add the noVerify assertion
  to `git_test.go::TestLoadGitConfig_ReadsValues` (line 71) + a new `load_test.go::TestLoad_NoVerifyPrecedence`
  mirroring `TestLoad_PushPrecedence` (load_test.go:1389-1429); confirm both FAIL (reader absent). (2) Add the
  reader; confirm both PASS. (3) Fix the 4 key-name references. The precedence test proves git-reads-noVerify
  (layer 4) AND env-overrides-git (layer 5 DIRECT-set escape hatch).

  SCOPE: edit `internal/config/git.go` (the reader) + `internal/config/git_test.go` + `internal/config/load_test.go`
  (tests) + `docs/cli.md` + `docs/configuration.md` + `internal/config/config.go` (comment) + `internal/cmd/root.go`
  (help text). No hooks runner (M1), no overlay/materialize/loadEnv/loadFlags, no TOML tag. INPUT = the push
  reader (git.go:174) as the template + the NoVerify bool field (config.go:134). OUTPUT = loadGitConfig reads
  `stagecoach.noVerify`; `git config stagecoach.noVerify true` works; the 5-layer precedence is complete.
  DOCS: Mode A — the 4 key-name fixes ride with this subtask.
---

## Goal

**Feature Goal**: Make `no_verify` resolve through the full 5-layer precedence documented in config.go:130 —
specifically, add the missing git-config reader (`stagecoach.noVerify`) in `loadGitConfig`, and correct the
invalid key name (`stagecoach.no_verify` → `stagecoach.noVerify`) everywhere it appears as the GIT-CONFIG key,
so `git config stagecoach.noVerify true` works (no "invalid key" error, value is read).

**Deliverable** (edits to existing files):
1. **`internal/config/git.go`** — in `loadGitConfig()`, immediately after the `stagecoach.push` reader block
   (~line 179), add a `stagecoach.noVerify` reader mirroring it: `if v, found, err := gitConfigBool(repoDir,
   "stagecoach.noVerify"); err != nil { return nil, err } else if found { c.NoVerify = v }`.
2. **`internal/config/git_test.go`** — in `TestLoadGitConfig_ReadsValues` (line 71), add
   `setGitConfig(t, repo, "stagecoach.noVerify", "true")` after the push line (87) and a `cfg.NoVerify` assertion
   after the Push assertion (~130).
3. **`internal/config/load_test.go`** — add `TestLoad_NoVerifyPrecedence` mirroring `TestLoad_PushPrecedence`
   (1389-1429): git config noVerify=true ⇒ cfg.NoVerify; then STAGECOACH_NO_VERIFY=false ⇒ cfg.NoVerify==false.
4. **4 key-name fixes** (`stagecoach.no_verify` → `stagecoach.noVerify`, GIT-CONFIG key only):
   `docs/cli.md:44`, `docs/configuration.md:155` (surgical), `internal/config/config.go:130` (surgical),
   `internal/cmd/root.go:219` (flag help text).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` clean; the two tests
pass (failing before the reader, green after); `go test -race ./internal/config/` + `go test -race ./...`
green; `make lint` green. `git config stagecoach.noVerify true` is accepted by git AND read by stagecoach
(cfg.NoVerify==true). The 4 git-config-key references say `stagecoach.noVerify`; the TOML key (`[generation].
no_verify`, the struct tag) is unchanged. go.mod/go.sum unchanged.

## User Persona

**Target User**: A user who wants to persistently bypass hooks for a repo via git config (per the docs).
Transitively PRD §9.25 FR-V5 + §9.8 FR34 (5-layer precedence).

**Use Case**: `git config stagecoach.noVerify true` in a repo → stagecoach reads it → pre-commit/commit-msg
skipped on every run (without passing `--no-verify` each time).

**User Journey**: docs/cli.md `--no-verify` row → user runs `git config stagecoach.noVerify true` → (before
fix: "invalid key" error OR silent no-op; after fix: accepted + read) → hooks bypassed.

**Pain Points Addressed**: removes the "invalid key" error AND the silent no-op (the reader was missing) —
the documented git-config layer now works end-to-end.

## Why

- **Restores a documented, PRD-mandated precedence layer.** FR-V5 + config.go:130 promise 5 layers; today
  no_verify has only 4 (git-config missing). This completes the set, mirroring `push`.
- **Fixes a pattern break.** Every other multi-word git-config key is camelCase; `no_verify` was the lone
  outlier AND invalid for git. camelCase restores convention AND git-validity.
- **Low risk, surgical.** One reader (template = the push reader), one bool field, two tests, four doc
  surfaces. overlay/materialize/loadEnv/loadFlags are already correct — untouched.
- **User-facing correctness.** Following the docs now works (`git config stagecoach.noVerify true` succeeds
  and takes effect), closing the documented-surface gap.

## What

One git-config reader added (mirroring `push`); the invalid key name corrected in 4 git-config-key
references; two tests (unit + precedence). No hooks runner, no overlay/materialize/loadEnv/loadFlags, no
TOML tag change. The TOML file key `[generation].no_verify` stays snake_case (TOML allows underscores).

### Success Criteria

- [ ] `loadGitConfig()` reads `stagecoach.noVerify` via `gitConfigBool` right after the `stagecoach.push`
      block (~line 179); `if found { c.NoVerify = v }`.
- [ ] `TestLoadGitConfig_ReadsValues` sets `stagecoach.noVerify true` and asserts `cfg.NoVerify == true`.
- [ ] `TestLoad_NoVerifyPrecedence` exists: git `noVerify=true` ⇒ cfg.NoVerify; env `STAGECOACH_NO_VERIFY=false`
      ⇒ cfg.NoVerify==false (env DIRECT > git).
- [ ] The 4 git-config-key references say `stagecoach.noVerify`: docs/cli.md:44, docs/configuration.md:155
      (only the `stagecoach.no_verify` token; `[generation].no_verify` intact), config.go:130 (same),
      root.go:219.
- [ ] The TOML struct tag (`config.go:134 toml:"no_verify"`) and the `[generation].no_verify` / `no_verify`
      TOML references (configuration.md:117/149/155) are UNCHANGED.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go test -race ./...`, `make lint`
      clean/green; go.mod/go.sum unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the push-reader template (quoted
verbatim below), the camelCase decision, the 4 doc surfaces (with the surgical rule), and the two test
specs. No hooks/git-internals knowledge required — this is one config reader mirroring a sibling.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M2T1S1/research/noVerify_gitconfig_reader.md
  why: the two defects, the camelCase decision, the push-reader template, the 4 doc surfaces (incl. the
       root.go:219 4th surface the contract missed), the surgical rule (TOML key stays), the overlay/env
       semantics, the TDD test plan, the scope boundary.
  critical: fix the GIT-CONFIG key only. The TOML key ([generation].no_verify, the struct tag) STAYS
       snake_case — they are different namespaces. config.go:130 and configuration.md:155 each contain BOTH
       tokens on one line — change ONLY `stagecoach.no_verify`.

- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/docs/architecture/config_no_verify_layer.md
  why: the research confirming the missing reader + the invalid key name + the "mirror Push" intent.
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/docs/architecture/git_hook_semantics.md
  section: §4 (the no_verify precedence contract)

- file: internal/config/git.go
  section: loadGitConfig() (110), gitConfigBool() (66), the stagecoach.push reader (174-179)
  why: the file you edit + the EXACT template. The push reader is the sibling to mirror (bool key, camelCase
       where multi-word). Place the noVerify reader right after the push block, before the ints section.
  pattern: `if v, found, err := gitConfigBool(repoDir, "stagecoach.<key>"); err != nil { return nil, err }
           else if found { c.<Field> = v }`.
  gotcha: use camelCase "noVerify" — git rejects underscores in the final segment; camelCase is git-valid
           and matches autoStageAll/maxDiffBytes/stripCodeFence (git.go:158/168).

- file: internal/config/git_test.go
  section: TestLoadGitConfig_ReadsValues (71), setGitConfig (45)
  why: the unit test to EXTEND. setGitConfig(t, repo, key, value) writes via `git config`. Add the noVerify
       set near line 87 (after push) + the assertion near line 130 (after the Push assert).
  pattern: mirror the push lines exactly (setGitConfig + `if !cfg.NoVerify { t.Errorf(...) }`).

- file: internal/config/load_test.go
  section: TestLoad_PushPrecedence (1389-1429), loadEnvSetup (83), chdir (33), writeConfigFile (20)
  why: the precedence-test template. Mirror it as TestLoad_NoVerifyPrecedence (git true → cfg true; env false
       → cfg false). Reuses loadEnvSetup/chdir/setGitConfig/Load.
  pattern: `_, repo, _ := loadEnvSetup(t); chdir(t, repo); setGitConfig(...); Load(...); assert; t.Setenv;
           Load(...); assert`.

- file: internal/config/config.go
  section: NoVerify field doc comment (130) + field (134)
  why: the doc comment's precedence chain references `stagecoach.no_verify` (git-config) AND `[generation].
       no_verify` (TOML) on the SAME line. Change ONLY the git-config token. Leave the struct tag
       (`toml:"no_verify"`, 134) — TOML allows underscores.
  gotcha: SURGICAL edit — both tokens co-occur on line 130; change `stagecoach.no_verify` only.

- file: docs/cli.md (44) + docs/configuration.md (155) + internal/cmd/root.go (219)
  why: the other 3 git-config-key surfaces. cli.md:44 = the --no-verify row git-config column cell;
       configuration.md:155 = the precedence chain in the `**no_verify**` prose (also has `[generation].
       no_verify` — surgical); root.go:219 = the --no-verify flag help string (user-facing `--help`).
  gotcha: configuration.md:155 and config.go:130 each contain BOTH `stagecoach.no_verify` and `[generation].
       no_verify` — change ONLY the former. Do NOT touch configuration.md:117 (`# no_verify = false`
       comment) or :149 (defaults-table `no_verify` row) — those are the TOML key.

- file: internal/config/file.go (overlay 351-352, materialize 291) + internal/config/load.go (loadEnv 315, loadFlags 447)
  why: CONFIRMS these 4 layers are already correct — do NOT touch them. overlay: `if src.NoVerify {
       dst.NoVerify = true }` (only-true, same as Push). loadEnv/loadFlags: DIRECT set (the false-escape hatch).
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  git.go            # loadGitConfig (110), gitConfigBool (66), push reader (174-179) — EDIT (add noVerify reader after push block)
  git_test.go       # TestLoadGitConfig_ReadsValues (71), setGitConfig (45) — EDIT (add set + assert)
  load_test.go      # TestLoad_PushPrecedence (1389) — EDIT (add TestLoad_NoVerifyPrecedence)
  config.go         # NoVerify doc comment (130) + field (134) — EDIT (comment only; tag UNCHANGED)
  file.go           # overlay (351) / materialize (291) — NO edit (correct)
  load.go           # loadEnv (315) / loadFlags (447) — NO edit (correct)
internal/cmd/root.go # --no-verify help text (219) — EDIT (stagecoach.no_verify → stagecoach.noVerify)
docs/cli.md          # --no-verify row (44) — EDIT (git-config column cell)
docs/configuration.md # no_verify prose (155) — EDIT (surgical: only stagecoach.no_verify token)
go.mod / go.sum      # unchanged
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits: git.go (reader) + git_test.go + load_test.go (tests) + config.go (comment) +
# root.go (help) + docs/cli.md + docs/configuration.md (key-name fixes).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: use camelCase "stagecoach.noVerify". git rejects underscores in the final config segment
// (`git config stagecoach.no_verify true` ⇒ "error: invalid key"). camelCase is git-valid AND matches the
// autoStageAll/maxDiffBytes/stripCodeFence convention (git.go:158/168). Do NOT use "no_verify" or "no-verify".

// CRITICAL: the git-config key and the TOML file key are DIFFERENT namespaces. Change ONLY the git-config-key
// references (stagecoach.no_verify → stagecoach.noVerify). The TOML struct tag `toml:"no_verify"` (config.go:134)
// STAYS — TOML allows underscores. So do the `[generation].no_verify` / `no_verify` TOML references.

// CRITICAL (surgical): config.go:130 AND docs/configuration.md:155 EACH contain BOTH `stagecoach.no_verify`
// (git-config) AND `[generation].no_verify` (TOML) on the SAME line. Change ONLY the `stagecoach.no_verify`
// token; leave `[generation].no_verify` intact. A blind find/replace on "no_verify" would corrupt the TOML key.

// CRITICAL: there are FOUR git-config-key surfaces, not three. The contract listed cli.md:44,
// configuration.md:155, config.go:130; the grep found a 4th: internal/cmd/root.go:219 (--no-verify flag help
// text — user-facing --help printing the invalid key). Fix ALL FOUR.

// CRITICAL: TDD order. Write/extend the two tests FIRST and confirm they FAIL (reader absent), then add the
// reader and confirm they PASS. This proves the reader is what makes them pass (not a test bug).

// GOTCHA: overlay() is only-true-propagates for NoVerify (if src.NoVerify { dst.NoVerify = true }), same as
// Push. So git config noVerify=true ⇒ cfg.NoVerify=true; git config noVerify=false ⇒ NO-OP (can't set false
// via the git layer — the v1 limitation; the flag/env DIRECT-set layers are the false-escape hatch). The
// precedence test uses env STAGECOACH_NO_VERIFY=false to demonstrate the escape hatch.

// GOTCHA: do NOT touch overlay() (file.go:351), materialize() (file.go:291), loadEnv (load.go:315), or
// loadFlags (load.go:447) — they are already correct. This task adds ONE reader + fixes the key name.
```

## Implementation Blueprint

### Data models and structure

No new types. The reader (mirror of the push reader at git.go:174-179):

```go
// internal/config/git.go — in loadGitConfig(), immediately AFTER the stagecoach.push block (~line 179),
// before the `// --- ints (plain --get -> Atoi) ---` section:
// §9.25 FR-V5 — noVerify via git config (camelCase key: git rejects underscores in the final segment,
// matching the autoStageAll/maxDiffBytes/stripCodeFence convention).
if v, found, err := gitConfigBool(repoDir, "stagecoach.noVerify"); err != nil {
	return nil, err
} else if found {
	c.NoVerify = v
}
```

### Implementation Tasks (ordered by dependencies — TDD)

```yaml
Task 1: git_test.go — extend TestLoadGitConfig_ReadsValues (FAILING first)
  - AFTER `setGitConfig(t, repo, "stagecoach.push", "true")` (line 87), add:
    `setGitConfig(t, repo, "stagecoach.noVerify", "true") // §9.25 FR-V5`.
  - AFTER the Push assertion (line 128-130), add:
    `if !cfg.NoVerify { t.Errorf("NoVerify=false want true (stagecoach.noVerify set)") }`.
  - RUN `go test ./internal/config/ -run TestLoadGitConfig_ReadsValues` → expect FAIL (NoVerify=false; reader absent).

Task 2: load_test.go — add TestLoad_NoVerifyPrecedence (FAILING first)
  - ADD a new test mirroring TestLoad_PushPrecedence (1389-1429):
    `_, repo, _ := loadEnvSetup(t); chdir(t, repo)`.
    (a) `setGitConfig(t, repo, "stagecoach.noVerify", "true")` → `Load(ctx, LoadOpts{RepoDir: repo, DisableBootstrap: true})`
        → assert `cfg.NoVerify == true` (proves the reader).
    (b) `t.Setenv("STAGECOACH_NO_VERIFY", "false")` → `Load(...)` → assert `cfg.NoVerify == false` (env DIRECT > git).
  - RUN `go test ./internal/config/ -run TestLoad_NoVerifyPrecedence` → expect FAIL (step a: NoVerify=false).

Task 3: git.go — add the stagecoach.noVerify reader (makes Task 1 + 2 PASS)
  - IN loadGitConfig(), after the stagecoach.push block (~line 179), add the reader per the Data Models block.
  - USE camelCase "stagecoach.noVerify" (NOT no_verify).
  - RUN `go test ./internal/config/ -run 'TestLoadGitConfig_ReadsValues|TestLoad_NoVerifyPrecedence'` → expect PASS.

Task 4: Fix the 4 git-config-key references (stagecoach.no_verify → stagecoach.noVerify)
  - docs/cli.md:44 — the --no-verify row's git-config column cell.
  - docs/configuration.md:155 — SURGICAL: only the `stagecoach.no_verify` token; leave `[generation].no_verify`.
  - internal/config/config.go:130 — SURGICAL: only the `stagecoach.no_verify` token; leave `[generation].no_verify`.
  - internal/cmd/root.go:219 — the --no-verify flag help string (`git stagecoach.no_verify` → `git stagecoach.noVerify`).
  - DO NOT touch: the TOML struct tag (config.go:134), configuration.md:117 (`# no_verify = false`) or :149
    (defaults-table row) — those are the TOML file key.

Task 5: VERIFY (no further file change)
  - RUN the full Validation Loop. go.mod/go.sum byte-unchanged. overlay/materialize/loadEnv/loadFlags/runner.go
    untouched. TOML tag + TOML-key references intact.
```

### Implementation Patterns & Key Details

```go
// The reader — mirror of the push reader (git.go:174-179), camelCase key:
if v, found, err := gitConfigBool(repoDir, "stagecoach.noVerify"); err != nil {
	return nil, err
} else if found {
	c.NoVerify = v
}

// The surgical doc edit (config.go:130 and configuration.md:155 BOTH have this shape):
//   BEFORE: ... / STAGECOACH_NO_VERIFY / stagecoach.no_verify / [generation].no_verify ...
//   AFTER : ... / STAGECOACH_NO_VERIFY / stagecoach.noVerify / [generation].no_verify ...
//   (only the git-config token changed; the TOML token [generation].no_verify is intact)

// The precedence test shape (mirror TestLoad_PushPrecedence):
_, repo, _ := loadEnvSetup(t)
chdir(t, repo)
setGitConfig(t, repo, "stagecoach.noVerify", "true")
cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, DisableBootstrap: true})
// assert cfg.NoVerify == true (git-config layer reads)
t.Setenv("STAGECOACH_NO_VERIFY", "false")
cfg, err = Load(context.Background(), LoadOpts{RepoDir: repo, DisableBootstrap: true})
// assert cfg.NoVerify == false (env DIRECT > git — the escape hatch)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - internal/config/file.go overlay() (351) + materialize() (291) — already handle NoVerify correctly.
  - internal/config/load.go loadEnv() (315) + loadFlags() (447) — already DIRECT-set (correct).
  - internal/config/config.go:134 the TOML struct tag `toml:"no_verify"` — TOML allows underscores; STAYS.
  - docs/configuration.md:117 (`# no_verify = false`) + :149 (defaults-table `no_verify` row) — TOML key, STAYS.
  - internal/hooks/runner.go (P1.M1.T2.S1, parallel — trailing newline; different file, no overlap).

DOWNSTREAM (do NOT implement here):
  - The hooks runner (M3) consumes cfg.NoVerify to skip pre-commit/commit-msg (FR-V5). This task only
    ensures cfg.NoVerify is correctly RESOLVED from the git-config layer; the consumption is M3's scope.
  - The Mode-B doc sweep (P1.M5) may cross-check; the Mode-A key-name fixes here are authoritative for the
    git-config key.

NO DATABASE / NO ROUTES / NO CLI WIRING (the --no-verify flag is already registered; only its help text changes).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/config/git.go internal/config/git_test.go internal/config/load_test.go \
  internal/config/config.go internal/cmd/root.go
test -z "$(gofmt -l internal/config/ internal/cmd/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/ ./internal/cmd/   # Catches a malformed reader / bad key.
go build ./...                              # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean + build succeeds.
```

### Level 2: Unit Tests (Component Validation) — TDD gate

```bash
# After Task 1+2 (tests added, reader absent) — expect FAIL:
go test ./internal/config/ -run 'TestLoadGitConfig_ReadsValues|TestLoad_NoVerifyPrecedence' -v
# After Task 3 (reader added) — expect PASS:
go test ./internal/config/ -run 'TestLoadGitConfig_ReadsValues|TestLoad_NoVerifyPrecedence' -v
# Expected (post-fix): TestLoadGitConfig_ReadsValues asserts cfg.NoVerify==true; TestLoad_NoVerifyPrecedence
#   proves git→true then env→false (DIRECT escape hatch).
go test -race ./internal/config/   # the full config suite (no regression — overlay/materialize/loadEnv/loadFlags unchanged).
go test -race ./...                # full module — no regression.
# Expected: green throughout.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Manual git-parity smoke (the headline fix): `git config stagecoach.noVerify true` must NOT error AND be read.
TMP=$(mktemp -d); git -C "$TMP" init -q
git -C "$TMP" config stagecoach.noVerify true && echo "noVerify key ACCEPTED (no 'invalid key' error)" \
  || echo "BAD: git rejected stagecoach.noVerify"
git -C "$TMP" config --get stagecoach.noVerify   # → "true"
# Confirm the 4 git-config-key surfaces are fixed AND the TOML key is intact:
grep -rn "stagecoach.no_verify" docs/ internal/ pkg/ 2>/dev/null && echo "BAD: stale stagecoach.no_verify remains" \
  || echo "no stagecoach.no_verify git-config refs remain (good)"
grep -rn 'toml:"no_verify"\|\[generation\].no_verify\|no_verify = false\|"no_verify"' internal/config/config.go docs/configuration.md | head
# Expected: the TOML tag + [generation].no_verify + no_verify defaults references are PRESENT (unchanged) — only
#   the git-config key (stagecoach.no_verify) is gone.
# Confirm only the listed files changed:
git diff --name-only | grep -Ev 'internal/config/git\.go|internal/config/git_test\.go|internal/config/load_test\.go|internal/config/config\.go|internal/cmd/root\.go|docs/cli\.md|docs/configuration\.md' \
  && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Precedence audit (optional): the 5-layer chain now works end-to-end for noVerify. A quick table test could
# also assert: default false → file true ⇒ true (overlay) → git true ⇒ true → env false ⇒ false → flag true ⇒ true.
# The two required tests cover the git + env layers (the layers this task touches); the file layer was already
# covered by existing file-layer tests. golangci-lint: `make lint` (project-wide gate).
make lint 2>&1 | grep -iE 'noVerify|no_verify' && echo "note: lint mentioned the key" || echo "lint clean of noVerify mentions"
# Expected: no lint finding about noVerify (camelCase is a normal Go identifier; the change is a string literal).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/config/ internal/cmd/`, `go vet`, `go build ./...`, `go mod tidy` no-op.
- [ ] Level 2 green: the two tests FAIL before the reader, PASS after; `go test -race ./...` green.
- [ ] Level 3: `git config stagecoach.noVerify true` accepted by git + read by stagecoach; no stale
      `stagecoach.no_verify` git-config refs; TOML key intact; only listed files changed.

### Feature Validation

- [ ] `loadGitConfig()` reads `stagecoach.noVerify` (camelCase) via `gitConfigBool`, right after the push reader.
- [ ] `TestLoadGitConfig_ReadsValues` asserts `cfg.NoVerify == true` after `setGitConfig(..., "stagecoach.noVerify", "true")`.
- [ ] `TestLoad_NoVerifyPrecedence`: git true ⇒ cfg.NoVerify; env false ⇒ cfg.NoVerify==false.
- [ ] The 4 git-config-key references say `stagecoach.noVerify` (cli.md:44, configuration.md:155 surgical,
      config.go:130 surgical, root.go:219).

### Code Quality Validation

- [ ] Mirrors the push reader exactly (gitConfigBool, same error/return shape).
- [ ] camelCase key (git-valid + convention); TOML key + struct tag unchanged (different namespace).
- [ ] Surgical doc edits (the `[generation].no_verify` token on shared lines is intact).
- [ ] No scope creep into overlay/materialize/loadEnv/loadFlags, the TOML tag, or the hooks runner.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode-A key-name fixes ride with this subtask (the 4 surfaces reference the specific key being fixed).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't use `stagecoach.no_verify` (git rejects underscores) or `stagecoach.no-verify` (breaks convention).
  Use camelCase `stagecoach.noVerify` — git-valid AND matches autoStageAll/maxDiffBytes/stripCodeFence.
- ❌ Don't change the TOML key. The git-config key (`stagecoach.no_verify`) and the TOML file key
  (`[generation].no_verify`, the `toml:"no_verify"` struct tag) are DIFFERENT namespaces. TOML allows
  underscores; the file key stays `no_verify`. Only the git-config key is broken.
- ❌ Don't blind find/replace "no_verify" → "noVerify". config.go:130 and docs/configuration.md:155 EACH
  contain both `stagecoach.no_verify` (git-config, fix) AND `[generation].no_verify` (TOML, keep) on the same
  line. Change ONLY the git-config token. A blind replace would corrupt the TOML key.
- ❌ Don't miss the 4th surface. The contract listed 3 (cli.md, configuration.md, config.go); the grep found
  `internal/cmd/root.go:219` (the --no-verify flag help text — user-facing --help). Fix ALL FOUR.
- ❌ Don't touch overlay(), materialize(), loadEnv(), or loadFlags(). They already handle NoVerify correctly
  (overlay only-true; loadEnv/loadFlags DIRECT-set). This task adds ONE reader.
- ❌ Don't skip the TDD order. Write/extend the two tests FIRST (confirm FAIL: reader absent), then add the
  reader (confirm PASS). This proves the reader is what fixes the tests.
- ❌ Don't deviate from the push-reader template. `gitConfigBool(repoDir, "stagecoach.noVerify")` with
  `if err != nil { return nil, err } else if found { c.NoVerify = v }` — same shape as the push reader (174).
- ❌ Don't touch internal/hooks/runner.go (P1.M1.T2.S1, parallel — trailing newline) or any hooks code.
  This task is config-layer + docs only.
- ❌ Don't add the reader in the wrong section. Place it in the BOOL-keys section, right after the
  `stagecoach.push` block (~line 179), BEFORE the `// --- ints ---` section.
- ❌ Don't change go.mod/go.sum or add files. One reader + two test edits + four key-name fixes.
- ❌ Don't skip `go test -race ./internal/config/` (confirms overlay/materialize/loadEnv/loadFlags are
  unchanged and the new reader didn't regress the precedence chain) or `make lint`.
