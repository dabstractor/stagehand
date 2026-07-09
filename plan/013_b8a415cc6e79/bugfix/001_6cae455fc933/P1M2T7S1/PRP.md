# PRP — P1.M2.T7.S1: Sweep README.md and overview docs for stale config-precedence references

name: "P1.M2.T7.S1 — Sync changeset-level documentation (config-precedence sweep)"
description: "Sweep all user-facing docs + the embedded `config init` template comments for stale config-precedence references introduced by the P1 config-precedence bugfix (Issues 1–3). The shipped markdown is already clean; the surviving stale references live in the Go template strings emitted by `config init`."

---

## Goal

**Feature Goal**: After the P1 config-precedence bugfix (Issues 1–3: `*bool` overlay fix, new
`STAGECOACH_AUTO_STAGE_ALL`/`STAGECOACH_MULTI_TURN_FALLBACK` env vars, and the camelCase git-config
key reconciliation in `docs/configuration.md`), NO user-facing documentation surface advertises the
un-settable snake_case git-config key `stagecoach.auto_stage_all` or otherwise contradicts the
post-fix precedence model. Specifically: (a) verify README.md + overview docs are clean, and
(b) fix the two surviving stale git-config-key references embedded in the `config init` template
comments that ship verbatim into every user's `~/.config/stagecoach/config.toml`.

**Deliverable**:
1. A documented sweep result proving `README.md`, `docs/README.md`, `FUTURE_SPEC.md`, and
   `docs/*.md` contain zero stale config-precedence references (the grep returns nothing).
2. Two one-line doc fixes in Go source template strings:
   - `internal/config/bootstrap.go:268`: `stagecoach.auto_stage_all` → `stagecoach.autoStageAll`
   - `internal/cmd/config.go:534`: `stagecoach.auto_stage_all` → `stagecoach.autoStageAll`
3. A regression assertion that pins the camelCase git key in the written config so it cannot
   silently revert.

**Success Definition**:
- `grep -rn 'stagecoach\.auto_stage_all' docs/ README.md FUTURE_SPEC.md` returns **nothing**.
- `stagecoach config init` (and `config init --template`) writes a config whose git-config
  hint uses `stagecoach.autoStageAll` (camelCase), verifiable by grepping the emitted file.
- The recommended `git config stagecoach.autoStageAll false` exits 0 (settable); the stale
  `git config stagecoach.auto_stage_all false` exits 1 with `error: invalid key`.
- `go build ./...` and `go test ./internal/config/... ./internal/cmd/...` pass.

## Why

- **Completes Issue 2's doc reconciliation across all surfaces.** P1.M1.T3.S1 fixed the snake_case
  git key in `docs/configuration.md` (lines 210 + 218, commits 9df1c66 + 79f4dc2) but missed the
  two Go template strings that write the *same* `# Git config keys` hint into every user's config
  file. The bug report (Issue 2) explicitly names the bootstrap output as a place the key is
  advertised: *"The config-file key is advertised as configurable in the bootstrap output and
  `docs/configuration.md`."* This PRP closes that gap.
- **Prevents a silent footgun.** `git config` rejects underscores in the final key segment, so a
  user who copy-pastes the shipped `git config stagecoach.auto_stage_all true` hint gets
  `error: invalid key` and the setting silently never applies — the exact failure class the P1
  bugfix exists to eliminate.
- **Keeps the docs sweep verifiable.** The grep assertion + regression test make the
  "no stale snake_case git key anywhere user-facing" invariant enforceable going forward.

## What

User-visible behavior does not change (this is a documentation-only changeset). What changes is the
text inside the config-file template written by `config init` / `config init --template`:

- The `# Git config keys` hint block in the written config now shows the *correct*, settable
  camelCase key `stagecoach.autoStageAll` instead of the un-settable snake_case form.
- No TOML field names change (`auto_stage_all` remains the snake_case TOML field — see the
  two-axis naming table in Context; do NOT touch TOML fields).

### Success Criteria

- [ ] `grep -rn 'stagecoach\.auto_stage_all' --include='*.md' docs/ README.md FUTURE_SPEC.md`
      → no output.
- [ ] `grep -rn 'git config stagecoach.auto_stage_all' --include='*.go' internal/` → no output
      (both template lines fixed to `autoStageAll`).
- [ ] `stagecoach config init` (into a temp dir) emits a file containing `git config
      stagecoach.autoStageAll` and NOT containing `git config stagecoach.auto_stage_all`.
- [ ] `go build ./...` succeeds; `go test ./internal/config/... ./internal/cmd/...` passes,
      including the new regression assertion.
- [ ] (Sanity, empirical) `git config stagecoach.autoStageAll false` exits 0;
      `git config stagecoach.auto_stage_all false` exits 1 with `error: invalid key`.

## All Needed Context

### Context Completeness Check

"If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?" — Yes. This PRP distinguishes the two naming axes (TOML field vs git-config key),
names the exact two files/lines, explains why the existing tests will not break, and gives copy-paste
verification commands. No prior knowledge of the wider config layer is required.

### Documentation & References

```yaml
# MUST READ — the authoritative, already-fixed docs (source of truth to align the templates to)
- file: docs/configuration.md
  why: The "Git-config keys" section (table around lines 207–228) is the canonical post-fix spelling.
       Line 211 (ini block) shows `autoStageAll = true`; line 220 (table) shows
       `stagecoach.autoStageAll | bool | git config --get --bool stagecoach.autoStageAll`.
       This is the exact spelling the two Go templates must match.
  pattern: "camelCase final segment for multi-word git keys (autoStageAll, stripCodeFence,
           tokenLimit, diffContext); lowercase single-word segments otherwise (provider, model,
           timeout, push)."
  gotcha: "git FORBIDS underscores in the final config-key segment — `git config
          stagecoach.auto_stage_all` fails with `error: invalid key` (exit 1). The code reads
          camelCase (internal/config/git.go:159). Never 'fix' the code to snake_case; fix docs."

- file: docs/cli.md
  why: The consolidated "## Flag ↔ env ↔ git-config map" table (around line 388) shows the
       `--no-auto-stage` row mapping to `STAGECOACH_AUTO_STAGE_ALL` (inverse) /
       `stagecoach.autoStageAll` — the post-fix canonical mapping.
  pattern: "the map is the single cross-surface reference; it was updated in commit 79f4dc2."

# THE TWO TARGETS (stale references to fix)
- file: internal/config/bootstrap.go
  why: The `bootstrapHeader` const (def at line 236) is written verbatim by `buildBootstrapConfig`
       (line 143) → `GenerateBootstrapConfig`/`GenerateBootstrapConfigWithOverrides` →
       `bootstrapWriteConfig` (the real `config init` write). Line 268 inside it contains the
       stale `#   git config stagecoach.auto_stage_all true`.
  pattern: "string literal inside a Go `const` (raw string literal in backticks). Edit the text in
           place; no escaping needed."
  gotcha: "Line 161 (`b.WriteString(\"...# auto_stage_all = true...\")`) is the TOML FIELD —
          snake_case is CORRECT there; DO NOT touch it. Only line 268 (the git-config hint) is wrong."

- file: internal/cmd/config.go
  why: The `exampleConfigTemplate` const (def at line 491) is written by `config init --template`
       (used at line 437: `content = exampleConfigTemplate`). Line 534 inside it contains the
       stale `#   git config stagecoach.auto_stage_all true`.
  pattern: "string literal inside a Go `const` (raw string literal in backticks). Edit the text in
           place."
  gotcha: "Line 553 (`# auto_stage_all = true`) is the TOML FIELD — snake_case is CORRECT; DO NOT
          touch it. Only line 534 (the git-config hint) is wrong."

# TEST PATTERNS — confirms edits are safe + where to add the regression assertion
- file: internal/config/bootstrap_test.go
  why: Uses `strings.Contains` checks (config_version, provider names, role models) — does NOT pin
       the git-key line, so editing bootstrap.go:268 will not break it.
  pattern: "add a regression assertion here (or in config_test.go) that the generated config
           Contains `stagecoach.autoStageAll` and does NOT Contain `git config stagecoach.auto_stage_all`."
  gotcha: "do NOT introduce an exact-equality golden of the whole header — it is high-churn; pin
          only the specific camelCase substring."

- file: internal/cmd/config_test.go
  why: Lines 438–439 and 622–624 compare the WRITTEN config to the `exampleConfigTemplate` constant
       with `!=`. Editing the constant changes BOTH sides equally → tests still pass. Confirms
       `exampleConfigTemplate` is LIVE.
  pattern: "exact-equality against the same const symbol is self-consistent; safe to edit the const."

# RESEARCH NOTES (already written by this PRP session — full detail)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T7S1/research/findings.md
  why: Consolidated sweep result + two-axis naming table + test-safety analysis.
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T7S1/research/code_ground_truth.md
  why: Code-level truth: git.go:159 reads `stagecoach.autoStageAll`; load.go env vars; *bool semantics.
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T7S1/research/canonical_model.md
  why: Canonical post-fix docs text (configuration.md / cli.md) the templates must match.
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T7S1/research/readme_audit.md
  why: README/docs-README/FUTURE_SPEC audit — all clean (verification reference).
```

### Current Codebase tree (the relevant slice)

```bash
internal/
  config/
    bootstrap.go        # `bootstrapHeader` const → written by `config init` (TARGET line 268)
    bootstrap_test.go   # Contains-based tests; add regression assertion here
    git.go              # reads `stagecoach.autoStageAll` (camelCase) at line 159 — the truth
    config.go           # Config struct + Defaults()
  cmd/
    config.go           # `exampleConfigTemplate` const → written by `config init --template` (TARGET line 534)
    config_test.go      # exact-equality vs the const symbol (self-consistent; safe)
docs/
  configuration.md      # ALREADY FIXED (9df1c66, 79f4dc2) — canonical spelling
  cli.md                # ALREADY FIXED — flag↔env↔git map
  README.md             # overview index — clean
  how-it-works.md       # clean
  providers.md          # clean
README.md               # clean (precedence ladder line 264 correct)
FUTURE_SPEC.md          # clean
```

### Desired Codebase tree with files to be MODIFIED (no new files)

```bash
internal/config/bootstrap.go      # MODIFY line 268: auto_stage_all → autoStageAll (git-config hint)
internal/config/bootstrap_test.go # ADD: regression assertion pinning camelCase git key
internal/cmd/config.go            # MODIFY line 534: auto_stage_all → autoStageAll (git-config hint)
# (optionally add a parallel assertion in internal/cmd/config_test.go)
# README.md / docs/* / FUTURE_SPEC.md: NO changes required (verified clean; document the grep result)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (two-axis naming): `auto_stage_all` is the TOML *field* (snake_case, CORRECT —
//   see config.go:69 `toml:"auto_stage_all"`, bootstrap.go:161, config.go:553). `stagecoach.autoStageAll`
//   is the git-config *key* (camelCase, CORRECT — git.go:159). git FORBIDS underscores in the final
//   config-key segment, so `git config stagecoach.auto_stage_all true` → `error: invalid key` (exit 1).
//   RULE: only edit the git-config *hint* lines (bootstrap.go:268, config.go:534); NEVER touch the
//   TOML field occurrences. grep each edit to confirm you changed the right one.

// CRITICAL (multi_turn has NO git key): there is no `stagecoach.multiTurnFallback` git-config key
//   anywhere in git.go. If the templates imply a git key for multi_turn, that is wrong — but the
//   two target lines only mention auto_stage_all, so do not invent a multi_turn git key.

// GOTCHA (config_test.go exact-equality): config_test.go:438,622 compare the written config to the
//   `exampleConfigTemplate` constant with `!=`. Editing the const changes both sides equally →
//   tests keep passing. Do not be alarmed that an exact-equality test references the symbol you edit.

// GOTCHA (PRD is read-only): PRD.md:338 (FR36) STILL lists the snake_case `stagecoach.auto_stage_all`.
//   That is human-owned and FORBIDDEN to edit. The PRD/PRD-snapshot snake_case hits in plan/** are
//   orchestrator-owned snapshots — also read-only. Scope is ONLY the two live Go template lines.

// GOTCHA (do not fix the code): git.go is CORRECT (reads camelCase). Do NOT change git.go to read
//   snake_case. The bug is purely in the doc/template text.
```

## Implementation Blueprint

### Data models and structure

None — documentation-only changeset. No structs, schemas, migrations, or config keys change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the sweep (no edits — produce evidence)
  - RUN: grep -rn 'stagecoach\.auto_stage_all' --include='*.md' docs/ README.md FUTURE_SPEC.md
  - EXPECT: no output (confirms shipped markdown is clean after 9df1c66 + 79f4dc2).
  - RUN: grep -rn 'git config stagecoach.auto_stage_all' --include='*.go' internal/
  - EXPECT: exactly two hits — internal/config/bootstrap.go:268 and internal/cmd/config.go:534.
  - RUN: grep -rn 'stagecoach\.auto_stage_all' --include='*.md' docs/ README.md FUTURE_SPEC.md
        ; grep -n 'auto_stage_all = true' internal/config/bootstrap.go internal/cmd/config.go
  - DOCUMENT: confirm bootstrap.go:161 + config.go:553 are the TOML FIELD (snake_case, CORRECT, must stay).
  - OUTPUT: record the grep results in the PRP execution notes as the "sweep baseline".

Task 2: FIX internal/config/bootstrap.go:268 (the `config init` populated header)
  - EDIT the line inside the `bootstrapHeader` const (def bootstrap.go:236):
      OLD: #   git config stagecoach.auto_stage_all true
      NEW: #   git config stagecoach.autoStageAll true
  - SCOPE GUARD: change ONLY the git-config hint line (268). Do NOT touch line 161
    (`# auto_stage_all = true`) — that is the TOML field.
  - VERIFY after edit: grep -n 'auto_stage_all\|autoStageAll' internal/config/bootstrap.go
    → line 161 stays `auto_stage_all` (TOML), line 268 is now `autoStageAll` (git key).

Task 3: FIX internal/cmd/config.go:534 (the `config init --template` inert header)
  - EDIT the line inside the `exampleConfigTemplate` const (def config.go:491):
      OLD: #   git config stagecoach.auto_stage_all true
      NEW: #   git config stagecoach.autoStageAll true
  - SCOPE GUARD: change ONLY the git-config hint line (534). Do NOT touch line 553
    (`# auto_stage_all = true`) — that is the TOML field.
  - VERIFY after edit: grep -n 'auto_stage_all\|autoStageAll' internal/cmd/config.go
    → line 553 stays `auto_stage_all` (TOML), line 534 is now `autoStageAll` (git key).

Task 4: ADD a regression assertion (so the camelCase key cannot silently revert)
  - PICK: internal/config/bootstrap_test.go (preferred — `GenerateBootstrapConfig` is the primary
    `config init` path). Mirror the existing `strings.Contains(content, ...)` style.
  - IMPLEMENT (inside the existing populated-config test, after the current Contains checks):
      // Regression for Issue 2 (P1.M2.T7.S1): the git-config hint must use the settable camelCase key.
      if strings.Contains(content, "git config stagecoach.auto_stage_all") {
          t.Errorf("bootstrap config advertises un-settable snake_case git key stagecoach.auto_stage_all; use camelCase autoStageAll")
      }
      if !strings.Contains(content, "stagecoach.autoStageAll") {
          t.Errorf("bootstrap config missing camelCase git key stagecoach.autoStageAll")
      }
  - OPTIONAL: add the same pair of assertions in internal/cmd/config_test.go against the
    `exampleConfigTemplate`-written content (the test around lines 622–624 that writes --template).
  - GOTCHA: assert on the git-config *hint* substrings, not the TOML field — the TOML `auto_stage_all`
    is expected to remain snake_case and must NOT be flagged.

Task 5: BUILD + RUN the affected tests
  - RUN: go build ./...
  - RUN: go test ./internal/config/... ./internal/cmd/...
  - EXPECT: compile clean; all tests pass (config_test.go exact-equality still holds because the
    const changed on both sides; bootstrap_test.go Contains checks unaffected).

Task 6 (OPTIONAL, lower priority — only if time permits; do NOT expand scope without sign-off):
  - COMPLETE the env-var list in the two template comments to include the P1 additions
    `STAGECOACH_AUTO_STAGE_ALL` (inverse of `--no-auto-stage`) and `STAGECOACH_MULTI_TURN_FALLBACK`,
    aligning with docs/configuration.md:200–201. Keep the comment style identical. This is a
    broader "sync changeset-level documentation" enhancement; the git-key fix (Tasks 2–4) is the
    required deliverable and can ship without this.
```

### Implementation Patterns & Key Details

```go
// The ONLY text change (applied identically at two sites). In a raw string literal (backticks),
// so no escaping is required — just replace the substring.

// internal/config/bootstrap.go  — inside const bootstrapHeader (line ~268)
//   and
// internal/cmd/config.go        — inside const exampleConfigTemplate (line ~534)
//
//   OLD fragment:
//     # Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
//     #   git config stagecoach.provider pi
//     #   git config stagecoach.model ""
//     #   git config stagecoach.timeout 120s
//     #   git config stagecoach.auto_stage_all true      <-- CHANGE THIS LINE
//     #   (read via `git config --get stagecoach.<key>`)
//
//   NEW fragment:
//     #   git config stagecoach.autoStageAll true        <-- camelCase (matches git.go:159)

// Regression assertion pattern (bootstrap_test.go), mirroring existing Contains style:
//   if strings.Contains(content, "git config stagecoach.auto_stage_all") {
//       t.Errorf("bootstrap config advertises un-settable snake_case git key stagecoach.auto_stage_all")
//   }
//   if !strings.Contains(content, "stagecoach.autoStageAll") {
//       t.Errorf("bootstrap config missing camelCase git key stagecoach.autoStageAll")
//   }
```

### Integration Points

```yaml
DATABASE: none
CONFIG:
  - NO config keys, defaults, or precedence layers change. TOML field `auto_stage_all` (snake_case)
    is unchanged and remains the correct TOML spelling.
ROUTES: none
BUILD:
  - go build ./... must remain green (string-literal edits; no API change).
DOCS:
  - README.md, docs/README.md, FUTURE_SPEC.md, docs/*.md: NO changes required (verified clean in
    Task 1). Do NOT edit PRD.md (forbidden) or plan/** snapshots (orchestrator-owned, read-only).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# String-literal edits in Go source — must compile.
go build ./...
go vet ./internal/config/... ./internal/cmd/...

# Markdown (if any .md were touched — they should NOT be): optional lint.
# markdownlint docs/ README.md   # only if a markdown file was actually changed

# Expected: zero errors. gofmt the two edited files for hygiene:
gofmt -w internal/config/bootstrap.go internal/cmd/config.go
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two packages whose template strings + tests are in scope.
go test ./internal/config/... -run 'Bootstrap|Config' -v
go test ./internal/cmd/...    -run 'Config|Template' -v

# Full affected package suites.
go test ./internal/config/...
go test ./internal/cmd/...

# Expected: all pass. The new regression assertion (Task 4) must pass; config_test.go exact-equality
# vs the exampleConfigTemplate const still holds because the const changed on both sides.
```

### Level 3: Integration Testing (System Validation)

```bash
# 3a. Build the binary.
go build -o /tmp/stagecoach-t71 ./cmd/stagecoach

# 3b. Verify the POPULATED config (config init) emits the camelCase git key.
tmpdir=$(mktemp -d)
cd "$tmpdir" && git init -q
HOME="$tmpdir" /tmp/stagecoach-t71 config init >/dev/null 2>&1 || true
written="$tmpdir/.config/stagecoach/config.toml"
echo "--- written populated config git-key hint ---"
grep -n 'git config stagecoach' "$written" || echo "(no git-config hint found — investigate)"
# Expected: shows  git config stagecoach.autoStageAll true   (camelCase)
grep -q 'git config stagecoach.autoStageAll true' "$written" && echo "OK: camelCase git key present in config init output"
grep -q 'git config stagecoach.auto_stage_all' "$written" \
  && { echo "FAIL: snake_case git key still present in config init output"; exit 1; } || echo "OK: no snake_case git key"

# 3c. Verify the INERT template (config init --template) emits the camelCase git key.
HOME="$tmpdir" /tmp/stagecoach-t71 config init --template --force >/dev/null 2>&1 || true
grep -q 'git config stagecoach.autoStageAll true' "$written" && echo "OK: camelCase git key present in --template output"
grep -q 'git config stagecoach.auto_stage_all' "$written" \
  && { echo "FAIL: snake_case git key still present in --template output"; exit 1; } || echo "OK: no snake_case git key in --template"

# 3d. Empirical git-key settablility (proves the camelCase key is the settable one).
cd "$tmpdir"
git config stagecoach.autoStageAll false; echo "camelCase exit=$? (expect 0)"
git config stagecoach.auto_stage_all false 2>&1 | grep -q 'invalid key' \
  && echo "OK: snake_case rejected with 'invalid key'" || echo "FAIL: snake_case unexpectedly accepted"

cd /home/dustin/projects/stagecoach && rm -rf "$tmpdir"
```

### Level 4: Creative & Domain-Specific Validation (the actual "sweep" proof)

```bash
# 4a. The sweep: NO snake_case git-config key anywhere in shipped user-facing docs.
grep -rn 'stagecoach\.auto_stage_all' --include='*.md' docs/ README.md FUTURE_SPEC.md
# Expected: no output.

# 4b. The sweep: NO stale snake_case git-config key line in live Go source.
grep -rn 'git config stagecoach.auto_stage_all' --include='*.go' internal/
# Expected: no output.

# 4c. Confirm the TOML field spelling is PRESERVED (regression guard — must NOT have been touched).
grep -rn 'auto_stage_all = true' internal/config/bootstrap.go internal/cmd/config.go
# Expected: TWO hits (bootstrap.go:161, config.go:553) — these are the TOML fields and must remain snake_case.

# 4d. Confirm code reads the camelCase key (unchanged — sanity that docs now match code).
grep -n 'autoStageAll' internal/config/git.go
# Expected: a hit at git.go:159 (the gitConfigGet/gitConfigBool reader).

# 4e. Full repo build + test (nothing else regressed).
go build ./...
go test ./...
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go build ./...` and `go vet ./internal/config/... ./internal/cmd/...` clean; edited
      files gofmt-clean.
- [ ] Level 2: `go test ./internal/config/...` and `go test ./internal/cmd/...` pass, including the
      new regression assertion (Task 4).
- [ ] Level 3: `stagecoach config init` and `config init --template` both emit a file containing
      `git config stagecoach.autoStageAll true` and NOT containing `git config stagecoach.auto_stage_all`.
- [ ] Level 3: `git config stagecoach.autoStageAll false` exits 0; `git config stagecoach.auto_stage_all
      false` exits 1 with `error: invalid key`.
- [ ] Level 4: `grep -rn 'stagecoach\.auto_stage_all' --include='*.md' docs/ README.md FUTURE_SPEC.md`
      returns nothing; `grep -rn 'git config stagecoach.auto_stage_all' --include='*.go' internal/`
      returns nothing.
- [ ] `go test ./...` green repo-wide.

### Feature Validation

- [ ] All success criteria in "What" met.
- [ ] The TOML field `auto_stage_all` (snake_case) is preserved at bootstrap.go:161 and config.go:553
      (NOT accidentally changed).
- [ ] The git-config hint at bootstrap.go:268 and config.go:534 both use camelCase `autoStageAll`.
- [ ] No new env var, CLI flag, git key, or config layer was introduced (documentation-only).
- [ ] PRD.md and plan/** snapshots were NOT modified.

### Code Quality Validation

- [ ] Follows existing template-comment style verbatim (only the key spelling changes).
- [ ] Regression assertion uses the existing `strings.Contains` test idiom.
- [ ] No exact-equality golden of the whole header introduced (high-churn anti-pattern).
- [ ] Scope respected: only the two git-config hint lines + the assertion; no scope creep into the
      optional env-var-list completion (Task 6) without explicit sign-off.

### Documentation & Deployment

- [ ] The two template comments now agree with `docs/configuration.md` and `docs/cli.md`
      (the already-fixed canonical spelling).
- [ ] Sweep result (Task 1) recorded in execution notes as evidence the markdown is clean.

---

## Anti-Patterns to Avoid

- ❌ Don't change the TOML field `auto_stage_all` to camelCase — snake_case is the correct TOML
  spelling (config.go:69 `toml:"auto_stage_all"`). Only the **git-config key** segment is camelCase.
- ❌ Don't "fix" `internal/config/git.go` to read snake_case — the code is correct; git FORCES
  camelCase (underscores are invalid in the final config-key segment). Fix the doc/template text only.
- ❌ Don't edit `PRD.md` (FR36 still says snake_case) or any `plan/**` snapshot — those are
  human/orchestrator-owned and read-only.
- ❌ Don't add a `stagecoach.multiTurnFallback` git key — none exists and creating one is out of scope.
- ❌ Don't introduce a full-header exact-equality golden test — it is high-churn and will fight future
  legit edits. Pin only the specific camelCase substring (positive + negative Contains).
- ❌ Don't expand into the optional env-var-list completion (Task 6) as if it were required — it is
  lower-priority "nice to have"; the git-key fix (Tasks 2–4) is the required deliverable.

---

## Scope Boundaries & Residual Risk (for the orchestrator)

- **In scope**: the two git-config hint lines (bootstrap.go:268, config.go:534), the regression
  assertion, and the documentation of the verified-clean markdown sweep.
- **Out of scope (flagged, do not action here)**: P1.M2.T4.S1 marks
  `STAGECOACH_VERBOSE=2` graceful-rejection as Complete, but `grep -rn 'not yet supported' internal/`
  is empty and `load.go:246-251` still `ParseBool`s (opaque error on `2`). No shipped doc claims
  VERBOSE=2 works or is unsupported, so this docs-sweep has nothing to fix on that axis — but the
  orchestrator should reconcile the T4 status vs the code. This is a separate task's concern.
- **Optional (Task 6)**: completing the env-var list in the two template comments is a broader
  "sync changeset-level documentation" enhancement; defer unless explicitly approved.
