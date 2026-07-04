---
name: "P1.M2.T1.S1 (bugfix Issue 2) — Config-layer diff_context range validation + docs"
description: |
  Bugfix for Issue 2 (Minor): `diff_context` out-of-range values are silently clamped to 1 by
  `buildDiffArgs` (`internal/git/git.go:689`) with NO diagnostic. The config layer accepts any integer; a
  user setting `diff_context = 5` gets `-U1` with zero feedback, violating PRD §9.1 FR3f ("integer 0–3").
  Fix (system_context.md §3.3 RECOMMENDED = the contract's PREFERRED option): add a config-layer range
  validation that returns a CLEAR ERROR at load time — a single `validateDiffContext(dc *int) error` helper
  in `internal/config/load.go` (next to the existing `validateFormat`/`validateTemplate`), invoked inline
  from `Load()` on the fully-merged `cfg.DiffContext`. An out-of-range value fails config load with a clear
  message; valid values (0,1,2,3) and unset behave byte-identically to before.

  ⚠️ **THE central design call — validate at the CONFIG LAYER (load path), NOT in the pure `buildDiffArgs`.**
  `buildDiffArgs` is a PURE function ("Pure function; no I/O.") that fires per diff-invocation across 3
  functions — adding a warning there would break purity AND spam. The load path (`Load()`) already returns
  errors and already hosts the validation cluster (`validateFormat`/`validateTemplate` at load.go:431/444 →
  called from Load at ~176/179). A standalone `validateDiffContext(dc *int) error` next to them, called from
  Load right after `validateTemplate`, is the consistent, testable, single-site fix. (The contract's
  `Config.Validate()` alternative is sanctioned too, but no `Config.Validate()` exists today and the
  standalone-helper matches the established `validateFormat`/`validateTemplate` convention — lower surprise.)

  ⚠️ **THE second design call — PRESERVE the load-bearing `*int` 0-vs-unset semantics.** `Config.DiffContext`
  is `*int`. `nil` ⇒ unset ⇒ default 1 (-U1) — VALID. `*0` ⇒ explicit -U0 (changed-lines-only) — VALID (THE
  key row, pinned by `file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit`). `*1`/`*2`/`*3` — VALID.
  ONLY `*v` where `v < 0` or `v > 3` is INVALID. The check MUST be `if dc != nil && (*dc < 0 || *dc > 3)` —
  guard `dc != nil` FIRST (nil deref would panic; nil is the valid unset case), and NEVER write `*dc < 1` or
  `!= 0` (that would reject the valid explicit 0).

  ⚠️ **THE third design call — call sites + `buildDiffArgs` clamp are UNCHANGED.** The 6 production call sites
  already consume `cfg.DiffContextValue()` / `deps.Config.DiffContextValue()` (generate.go:184,
  decompose.go:624, message.go:96, planner.go:85, hook/exec.go:128, stagehand.go:442) — leave them. After
  validation, `DiffContextValue()` returns a value in [0,3], so `buildDiffArgs`'s clamp (git.go:689) is NEVER
  hit for config-loaded runs — leave it as belt-and-suspenders for programmatic `Config` construction (which
  bypasses `Load`/`Validate`). Do NOT touch `DiffContextValue`, `buildDiffArgs`, `materialize`, or `overlay`.

  SCOPE: edit `internal/config/load.go` (add `validateDiffContext` + the inline `Load()` call), the pure
  unit test (mirror `TestMaterializeOverlay_DiffContext_TokenLimit`'s `intp` table style), and Mode-A docs
  (`docs/configuration.md` 3 spots + `internal/config/bootstrap.go:291` comment). NO git.go, NO call sites,
  NO materialize/overlay/buildDiffArgs, NO new deps. INPUT = the merged `cfg.DiffContext` (*int) at the end
  of `Load()`. OUTPUT = out-of-range diff_context fails config load with a clear message; valid/unset
  byte-identical. DOCS ride WITH the work (Mode A — no separate doc subtask).
---

## Goal

**Feature Goal**: Make an out-of-range `diff_context` (PRD §9.1 FR3f: integer 0–3) fail config load with a
clear, field-named error instead of being silently clamped to 1 at diff time — while preserving the
load-bearing `*int` semantics (`nil`⇒unset/default 1, `*0`⇒valid -U0) so valid configurations behave
byte-identically to before.

**Deliverable** (edits to existing files):
1. **`internal/config/load.go`** — (a) add `func validateDiffContext(dc *int) error` next to
   `validateFormat`/`validateTemplate` (~431/444); (b) invoke it inline from `Load()` right after the
   `validateTemplate` call (~179), returning `fmt.Errorf("diff_context: %w", err)`.
2. **Pure unit test** (in `load_test.go` next to any `validateFormat` test, mirroring
   `file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit`'s `intp` table style): `nil`/0/1/2/3 pass;
   4/5/-1 error.
3. **Mode-A docs** — `docs/configuration.md` (comment block ~107, defaults table ~131, prose ~147) +
   `internal/config/bootstrap.go:291` template comment: state the valid range is 0–3 and out-of-range is
   rejected at config load.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/` clean;
`go test -race ./internal/config/` green (new validation test passes; `TestMaterializeOverlay_DiffContext_TokenLimit`
unchanged — the explicit-0 row STILL passes); `go test -race ./...` green (no regression — valid/unset config
behaves identically). An out-of-range `diff_context` (5, -1, 4) makes `Load()` return a non-nil error whose
text contains "diff_context" and the offending value; 0/1/2/3 and unset load exactly as before. go.mod/go.sum
unchanged; no file outside `internal/config/load.go` + the test + the 2 doc surfaces is touched.

## User Persona

**Target User**: A user who sets `diff_context` in config or git-config and mistypes an out-of-range value
(e.g. `diff_context = 5`, or `git config stagehand.diffContext 9`). Transitively PRD §9.1 FR3f ("integer 0–3").

**Use Case**: `diff_context = 5` in `~/.config/stagehand/config.toml` → stagehand fails at load with
`diff_context: must be in range [0,3]: got 5` instead of silently running with `-U1`.

**User Journey**: config file/git-config → `Load()` → `validateDiffContext(cfg.DiffContext)` → error or
proceed → (on proceed) call sites' `DiffContextValue()` returns [0,3] → `buildDiffArgs` emits `-U<context>`
(clamp never fires).

**Pain Points Addressed**: removes the silent-clamp footgun (a typo produces -U1 with no feedback) with an
early, clear, testable error — without weakening the valid `*0`/unset cases or touching the diff hot path.

## Why

- **Surfaces bad config early and clearly.** A typo (`diff_context = 5`) now fails at load with a field-named
  message instead of silently degrading to -U1. Matches the existing `format`/`template` hard-error discipline.
- **Single-site, low-risk.** One pure helper + one inline Load call; no diff-path change, no call-site change,
  no new types. The `buildDiffArgs` clamp stays as a defensive guard (now never hit for valid config).
- **Preserves the `*int` contract.** `nil`⇒1 and `*0`⇒-U0 are load-bearing (FR3f explicitly allows 0); the
  guard `dc != nil && (*dc < 0 || *dc > 3)` rejects ONLY the genuinely invalid values.
- **Consistent.** Mirrors `validateFormat`/`validateTemplate` exactly (same file, same shape, same Load wiring).
- **Transparent docs.** Mode-A: the range + rejection is documented where `diff_context` is described.

## What

One pure validation helper in `internal/config/load.go`, invoked once from `Load()` on the merged config; a
pure table unit test; and 4 doc surface updates. No diff-function, call-site, materialize/overlay, or
buildDiffArgs changes. Valid configurations are byte-identical.

### Success Criteria

- [ ] `func validateDiffContext(dc *int) error` exists in `internal/config/load.go` next to
      `validateFormat`/`validateTemplate`; returns nil for `nil`/`*0`/`*1`/`*2`/`*3`, non-nil for `*v<0 || *v>3`.
- [ ] `Load()` calls `validateDiffContext(cfg.DiffContext)` right after the `validateTemplate` call and returns
      `fmt.Errorf("diff_context: %w", err)` on failure (before `return &cfg`).
- [ ] The pure unit test passes: `nil`, `intp(0)`, `intp(1)`, `intp(2)`, `intp(3)` → no error; `intp(4)`,
      `intp(5)`, `intp(-1)` → non-nil error whose text names the range/got-value.
- [ ] `TestMaterializeOverlay_DiffContext_TokenLimit` passes UNCHANGED (the explicit-0 row is still valid).
- [ ] `docs/configuration.md` (~107 comment, ~131 table row, ~147 prose) + `internal/config/bootstrap.go:291`
      template comment state the valid range is 0–3 and out-of-range is rejected at config load.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go test -race ./...` clean/green;
      go.mod/go.sum unchanged; no file outside `internal/config/load.go` + the test + the 2 doc surfaces touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the helper body (quoted below), the
exact Load insertion point (right after `validateTemplate`), the `*int` guard rule, and the test table. No
git/diff-internals knowledge required — this is a pure config-value range check.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M2T1S1/research/diff_context_validation.md
  why: the chosen fix (config-layer error via validateDiffContext), the helper body, the exact Load insertion
       point, the *int guard rule (nil⇒valid, *0⇒valid, only <0/>3 invalid), the test style to mirror, and
       the scope boundary (no call-site/buildDiffArgs/materialize/overlay changes).
  critical: the guard MUST be `dc != nil && (*dc < 0 || *dc > 3)`. Never `*dc < 1` or `!= 0` — *0 is a VALID
       explicit value (-U0). Guard nil FIRST (else nil-deref panic).

- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/docs/system_context.md
  section: "3. Issue 2 — diff_context out-of-range" (3.1 current behavior, 3.2 two options, 3.3 recommendation)
  why: §3.3 mandates option (a) — a config-layer error as a single helper called from load — and the *int
       semantics to preserve. §4.1 gives the file/line refs (buildDiffArgs git.go:689; DiffContextValue
       config.go:201; overlay file.go:226/340). §5 lists the test conventions (pure table style, no I/O).
  critical: §3.3 "nil ⇒ unset (default 1), *0 ⇒ valid (-U0), only values <0 or >3 are rejected."

- file: internal/config/load.go
  section: Load() (76, the validation cluster ~176-182), validateFormat (431), validateTemplate (444)
  why: the validation-helper pattern to MIRROR. validateFormat/validateTemplate are standalone funcs taking
       the resolved value, returning a descriptive error; Load wraps with the field name
       (`fmt.Errorf("format: %w", err)`). validateDiffContext follows the SAME shape (takes *int, returns
       "must be in range [0,3]: got N"; Load wraps `fmt.Errorf("diff_context: %w", err)`).
  pattern: place validateDiffContext next to validateTemplate; call it from Load right after the
           validateTemplate call, before the self-hosting provider guard (~201).

- file: internal/config/config.go
  section: Config.DiffContext *int (82), Defaults() DiffContext: intPtr(1) (175), DiffContextValue() (201)
  why: the field you validate + the *int semantics. DiffContextValue returns *c.DiffContext verbatim or 1 —
       it does NOT clamp, so without validation an out-of-range *int reaches buildDiffArgs' clamp via the
       call sites. Validate the RAW *int in Load BEFORE DiffContextValue is called at the call sites.
  gotcha: do NOT modify DiffContextValue, Defaults(), or the field — only ADD validation in load.go.

- file: internal/config/file_test.go
  section: TestMaterializeOverlay_DiffContext_TokenLimit (814)
  why: the test style to MIRROR — `intp := func(v int) *int { return &v }`, table-driven `t.Run` subtests,
       pure (no t.TempDir, no I/O). It is ALSO the regression anchor: the explicit_0 row MUST still pass
       (your validation must accept *0). Do NOT edit it.
  pattern: your TestValidateDiffContext uses the same intp helper + table shape (nil/intp(0..3) pass;
           intp(4/5/-1) error).

- file: internal/git/git.go
  section: buildDiffArgs (689, the clamp)
  why: confirms the clamp you are SUPERSEDING for valid config (it stays as belt-and-suspenders). The clamp
       is `if ctx < 0 || ctx > 3 { ctx = 1 }` — the SAME range you validate, so they agree by construction.
  gotcha: do NOT touch buildDiffArgs. After validation, ctx ∈ [0,3] always, so the clamp is dead code for
           config-loaded runs but remains a guard for programmatic Config construction.

- file: internal/config/git.go
  section: diff_context git-config resolver (203-214)
  why: confirms diff_context can come from git-config (`stagehand.diffContext`) as well as file. The resolver
       parses the integer but does NOT range-check — so a `git config stagehand.diffContext 5` flows through
       overlay into cfg.DiffContext and is caught by your Load-time validation (the single chokepoint).
  gotcha: do NOT add a range check here — keep validation single-site at Load.

- file: docs/configuration.md (~107 comment, ~131 table, ~147 prose) + internal/config/bootstrap.go:291
  why: the Mode-A doc surfaces. Append "valid range 0–3; out-of-range rejected at config load" to each.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go             # Load() (76) + validateFormat (431) + validateTemplate (444) — EDIT (add validateDiffContext + Load call)
  config.go           # DiffContext *int (82), Defaults (161), DiffContextValue (201) — NO edit (READ for semantics)
  file.go             # materialize (226) + overlay (340) — NO edit (copy *int verbatim, by design)
  git.go              # diff_context git-config resolver (203-214) — NO edit (validation is single-site at Load)
  file_test.go        # TestMaterializeOverlay_DiffContext_TokenLimit (814) — NO edit (regression anchor)
  load_test.go        # validateFormat/validateTemplate tests — EDIT (add TestValidateDiffContext here)
internal/git/
  git.go              # buildDiffArgs clamp (689) — NO edit (belt-and-suspenders, now dead for valid config)
docs/configuration.md # diff_context comment ~107, table ~131, prose ~147 — EDIT (Mode A)
internal/config/bootstrap.go # template comment ~291 — EDIT (Mode A)
go.mod / go.sum       # unchanged
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits: internal/config/load.go (helper + Load call), internal/config/load_test.go (test),
# docs/configuration.md (3 spots), internal/config/bootstrap.go (1 comment).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the guard is `if dc != nil && (*dc < 0 || *dc > 3)`. Guard dc != nil FIRST (a nil deref panics;
// nil is the valid "unset" case ⇒ default 1). NEVER write `*dc < 1` or `*dc != 0` — *0 is a VALID explicit
// value (-U0, changed-lines-only), pinned by TestMaterializeOverlay_DiffContext_TokenLimit's explicit_0 row.

// CRITICAL: validate at the CONFIG LAYER (Load path), NOT in buildDiffArgs. buildDiffArgs is a PURE function
// that fires per diff-invocation × 3 functions — a warning there breaks purity and spams. Load already returns
// errors and hosts validateFormat/validateTemplate. Mirror them.

// CRITICAL: leave the 6 call sites, DiffContextValue, buildDiffArgs, materialize, and overlay UNCHANGED. The
// call sites consume DiffContextValue() (returns [0,3] after validation); buildDiffArgs' clamp stays as a
// defensive guard for programmatic Config construction (bypasses Load/Validate). This keeps the change single-site.

// GOTCHA: validateDiffContext takes *int (the RAW merged field), NOT the DiffContextValue() int. DiffContextValue
// returns the value verbatim (no clamp) — validating its output would be too late (the *int is the source of
// truth; validate it directly so the error names the configured value before any resolution).

// GOTCHA: diff_context has NO env/flag path (grep loadEnv/loadFlags finds none) — only file + git-config
// sources. Validating cfg.DiffContext at the end of Load (after all overlays) covers every source in one place.

// GOTCHA: the helper is PURE (no I/O) so it is unit-testable directly with intp(5)/intp(-1)/intp(0)/nil — no
// t.TempDir, no git, no file I/O. This matches the contract's "no git, no I/O" test guidance and the
// TestMaterializeOverlay_DiffContext_TokenLimit style.

// GOTCHA (docs): update BOTH the user-facing docs/configuration.md (3 spots) AND the bootstrap template
// comment (bootstrap.go:291). The defaults table row (~131) may only have room for a brief "(range 0–3)" —
// put the full "out-of-range rejected at config load" in the prose (~147) and the comment block (~107).
```

## Implementation Blueprint

### Data models and structure

No new types. The single helper + the Load wiring:

```go
// internal/config/load.go — next to validateFormat/validateTemplate (~431/444):

// validateDiffContext rejects an out-of-range diff_context (PRD §9.1 FR3f: integer 0–3). It is the
// config-layer diagnostic for bugfix Issue 2 (buildDiffArgs otherwise silently clamps to 1). Semantics:
// nil ⇒ unset ⇒ valid (default 1 applied by DiffContextValue); *0 ⇒ valid (-U0, changed-lines-only);
// *1/*2/*3 ⇒ valid. ONLY *v<0 or *v>3 is an error. Pure (no I/O) → unit-testable directly. Called from
// Load after every layer (file + git-config) has merged into cfg.DiffContext.
func validateDiffContext(dc *int) error {
	if dc == nil {
		return nil
	}
	if v := *dc; v < 0 || v > 3 {
		return fmt.Errorf("must be in range [0,3]: got %d", v)
	}
	return nil
}

// internal/config/load.go — inside Load(), right after the validateTemplate call (~179), before the
// self-hosting provider guard (~201):
if err := validateDiffContext(cfg.DiffContext); err != nil {
	return nil, fmt.Errorf("diff_context: %w", err)
}
```

```go
// internal/config/load_test.go — the pure table test (mirror TestMaterializeOverlay_DiffContext_TokenLimit):
func TestValidateDiffContext(t *testing.T) {
	intp := func(v int) *int { return &v }
	cases := []struct {
		name    string
		dc      *int
		wantErr bool
	}{
		{"unset_nil", nil, false},              // nil ⇒ unset ⇒ default 1 (VALID)
		{"explicit_0", intp(0), false},         // THE key row: -U0 changed-lines-only (VALID)
		{"explicit_1", intp(1), false},
		{"explicit_2", intp(2), false},
		{"explicit_3", intp(3), false},
		{"over_3_four", intp(4), true},         // boundary just over 3
		{"over_3_five", intp(5), true},         // the contract's example
		{"negative_one", intp(-1), true},        // the contract's example
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDiffContext(tc.dc)
			if tc.wantErr && err == nil {
				t.Errorf("validateDiffContext(%v) = nil, want error", tc.dc)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateDiffContext(%v) = %v, want nil", tc.dc, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "range") {
				t.Errorf("error %q should name the range", err.Error())
			}
		})
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: load.go — add validateDiffContext helper
  - ADD func validateDiffContext(dc *int) error next to validateFormat/validateTemplate (~431/444).
  - BODY: `if dc == nil { return nil }; if v := *dc; v < 0 || v > 3 { return fmt.Errorf("must be in range [0,3]: got %d", v) }; return nil`.
  - DOC: name it the Issue-2 config-layer diagnostic; restate the *int semantics (nil⇒1, *0⇒valid, <0/>3 invalid).
  - GOTCHA: guard `dc != nil` FIRST; NEVER `*dc < 1` or `!= 0`.

Task 2: load.go — invoke validateDiffContext from Load()
  - INSERT right after the `validateTemplate(cfg.Template)` call (~179), before the self-hosting provider guard:
    `if err := validateDiffContext(cfg.DiffContext); err != nil { return nil, fmt.Errorf("diff_context: %w", err) }`.
  - WHY HERE: cfg.DiffContext is the fully-merged *int (file + git-config layers applied); the format/template
    validations are the established cluster to sit beside.
  - GOTCHA: do NOT re-wrap with a redundant field name (the helper returns a value message; Load adds "diff_context:").

Task 3: load_test.go — add TestValidateDiffContext (pure table)
  - ADD the test per the Data Models block: intp helper + 8 subtests (nil/0/1/2/3 pass; 4/5/-1 error).
  - PATTERN: mirror file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit's table style (pure, no I/O).
  - ASSERT: error cases return non-nil whose text contains "range"; valid cases return nil.

Task 4: docs/configuration.md — 3 Mode-A spots
  - ~107 comment block: append "; valid range 0–3 — out-of-range rejected at config load" to the diff_context line.
  - ~131 defaults table: append "(range 0–3)" to the diff_context row's source/notes cell.
  - ~147 prose (the **diff_context** bullet): add "Valid range is 0–3; an out-of-range value is rejected at
    config load with a clear error (§9.1 FR3f)."

Task 5: internal/config/bootstrap.go — template comment (~291)
  - Append "; valid 0–3 (out-of-range rejected at config load)" to the `# diff_context = 1 # ...` comment.

Task 6: VERIFY (no further file change)
  - RUN the Validation Loop. go.mod/go.sum byte-unchanged. No file outside load.go + load_test.go +
    docs/configuration.md + bootstrap.go touched. TestMaterializeOverlay_DiffContext_TokenLimit passes unchanged.
```

### Implementation Patterns & Key Details

```go
// The *int guard — the whole correctness keystone (nil and *0 are BOTH valid):
func validateDiffContext(dc *int) error {
	if dc == nil {
		return nil // unset ⇒ default 1 (valid)
	}
	if v := *dc; v < 0 || v > 3 {
		return fmt.Errorf("must be in range [0,3]: got %d", v)
	}
	return nil // *0, *1, *2, *3 all valid
}

// The Load wiring — one line, right after validateTemplate, field-named wrap (mirrors validateFormat):
if err := validateDiffContext(cfg.DiffContext); err != nil {
	return nil, fmt.Errorf("diff_context: %w", err)
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure validation + docs; no new dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - internal/config/config.go (DiffContext field, Defaults, DiffContextValue — READ for semantics, no edit).
  - internal/config/file.go (materialize/overlay — copy *int verbatim by design; no range check there).
  - internal/config/git.go (diff_context git-config resolver — parses int, no range check; validation is
    single-site at Load).
  - internal/git/git.go (buildDiffArgs clamp — stays as belt-and-suspenders; now dead for config-loaded runs).
  - The 6 StagedDiffOptions call sites (generate/decompose/hook/stagehand — already consume DiffContextValue()).
  - internal/config/file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit (regression anchor; the
    explicit_0 row MUST still pass — do NOT edit).

DOWNSTREAM / PARALLEL (no conflict):
  - P1.M1.T2.S1 (parallel) is TEST-ONLY in internal/git/difftokenlimit_test.go (Issue-1 truncation E2E) —
    different package/file, zero overlap.
  - P1.M3.T1.S1 (Mode-B doc sweep) is a CATCH-ALL that may also touch docs/configuration.md; the Mode-A
    updates here are scoped to the diff_context range wording — coordinate so the two don't overwrite each
    other (this task's wording is the authoritative diff_context-range statement).

NO DATABASE / NO ROUTES / NO CLI / NO NEW FILES / NO DIFF-PATH CHANGES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/config/load.go internal/config/load_test.go
test -z "$(gofmt -l internal/config/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/        # Catches a nil-deref or a wrong guard.
go build ./...                   # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If go vet flags validateDiffContext, re-check the nil guard ordering.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/config/ -v -run 'TestValidateDiffContext|TestMaterializeOverlay_DiffContext_TokenLimit'
# Expected: TestValidateDiffContext PASS (nil/0/1/2/3 → nil; 4/5/-1 → error naming the range) AND
#   TestMaterializeOverlay_DiffContext_TokenLimit PASS UNCHANGED (the explicit_0 row is still valid — the
#   *int semantics are preserved). If the latter fails, your guard wrongly rejects *0 (re-check: `dc != nil &&`).
go test -race ./...              # Full suite — no regression (valid/unset config behaves identically).
# Expected: green throughout. This task adds a load-time check that ONLY fires for out-of-range values; every
#   existing test uses valid/unset diff_context, so none should change behavior.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the listed files changed:
git diff --name-only | grep -Ev 'internal/config/load\.go|internal/config/load_test\.go|docs/configuration\.md|internal/config/bootstrap\.go' \
  && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
# Behavioral smoke (optional): write a t.TempDir config with diff_context = 5 → config.Load(...) returns a
# non-nil error whose Error() contains "diff_context" and "5". (The pure TestValidateDiffContext is the
# deterministic anchor; this is belt-and-suspenders proving the Load wiring.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Boundary audit (optional): confirm the guard is symmetric and complete — 0 and 3 pass, 4 and -1 fail. The
# table test already covers this (over_3_four, negative_one). golangci-lint: `make lint` (project-wide gate).
# Doc audit: grep -n "diff_context" docs/configuration.md internal/config/bootstrap.go — each occurrence now
# mentions the 0–3 range + the rejection.
grep -n "diff_context" docs/configuration.md internal/config/bootstrap.go
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go mod tidy` no-op.
- [ ] Level 2 green: `TestValidateDiffContext` passes; `TestMaterializeOverlay_DiffContext_TokenLimit` passes
      UNCHANGED; `go test -race ./...` green.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only listed files changed.

### Feature Validation

- [ ] `validateDiffContext(dc *int) error` rejects `*v<0 || *v>3`; accepts nil, *0, *1, *2, *3.
- [ ] `Load()` calls it post-overlay and returns `fmt.Errorf("diff_context: %w", err)` on failure.
- [ ] Valid/unset config loads byte-identically (no behavior change for 0/1/2/3/unset).
- [ ] docs/configuration.md (3 spots) + bootstrap.go:291 comment state range 0–3 + rejection.

### Code Quality Validation

- [ ] Mirrors `validateFormat`/`validateTemplate` (standalone helper in load.go, field-named wrap in Load).
- [ ] `*int` semantics preserved (nil⇒1, *0⇒valid); nil-guard first.
- [ ] No scope creep into buildDiffArgs, call sites, materialize/overlay, DiffContextValue, or git.go.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode-A docs ride with the work (docs/configuration.md + bootstrap.go comment); no separate doc subtask.
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't add the diagnostic inside `buildDiffArgs`. It is a PURE function that fires per diff-invocation × 3
  functions — a warning there breaks purity and spams. Validate at the config layer (`Load`) once.
- ❌ Don't write `*dc < 1` or `*dc != 0` as the guard. `*0` is a VALID explicit value (-U0, changed-lines-only),
  pinned by `TestMaterializeOverlay_DiffContext_TokenLimit`. The guard is `dc != nil && (*dc < 0 || *dc > 3)`.
- ❌ Don't forget the `dc != nil` guard — dereferencing a nil `*int` panics, and nil is the valid "unset" case.
- ❌ Don't touch `DiffContextValue`, `buildDiffArgs`, the 6 call sites, `materialize`, or `overlay`. The change
  is single-site (load.go helper + Load call); the existing clamp stays as belt-and-suspenders.
- ❌ Don't validate `DiffContextValue()`'s int OUTPUT — validate the RAW `*int` field, so the error names the
  configured value before resolution and fires once at load (not at each of the 6 call sites).
- ❌ Don't add a range check to the git-config resolver (`git.go:214`) or to `materialize`/`overlay`. Validation
  is single-site at `Load` — that's the chokepoint covering both file and git-config sources.
- ❌ Don't edit `TestMaterializeOverlay_DiffContext_TokenLimit` — it is the regression anchor proving `*0` stays
  valid. If it fails, YOUR guard is wrong (re-check it accepts `*0`).
- ❌ Don't use testify or file I/O in the unit test — mirror the pure `intp` table style of
  `TestMaterializeOverlay_DiffContext_TokenLimit` (the contract specifies no git, no I/O).
- ❌ Don't skip the Mode-A docs — `docs/configuration.md` (3 spots) and `bootstrap.go:291` must state the 0–3
  range and the rejection. This rides WITH the work (no separate doc subtask).
- ❌ Don't change go.mod/go.sum or add new files. One helper, one Load call, one test, four doc-spot edits.
- ❌ Don't skip `go test -race ./...` — it confirms valid/unset configurations are byte-identical (no existing
  test uses an out-of-range value, so all stay green; if any fails, the guard wrongly rejected a valid value).
