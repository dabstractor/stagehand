name: "P1.M3.T3.S1 — Auto-stage notice singular/plural grammar: '(1 files)' → '(1 file)' (Issue 6)"
description: >
  A TINY, surgical fix for Issue 6 (PRD §h3.5 "Issue 6: Auto-stage notice uses '(1 files)' — minor grammar").
  The FR18 auto-stage notice at internal/cmd/default_action.go:150 hardcodes the plural "files", so when
  exactly one file is auto-staged the user sees the ungrammatical "Nothing staged — staging all changes
  (1 files)." The fix: insert a 3-line `noun` conditional immediately before the Fprintln (noun="files";
  if n==1 { noun="file" }) and change the format string `(%d files).` → `(%d %s).` with `noun` as the new
  arg (the verbatim fix from architecture/minor_fixes.md §Issue 6). The n>=1 guarantee is already in place
  (the n==0 clean-tree early return at lines 147-149 — the Issue 7 fix — skips the notice, so line 150 only
  ever sees n>=1; the singular case is exactly n==1). ONE new test, `TestRunDefault_AutoStageNoticeSingular_Issue6`,
  is ADDED to internal/cmd/default_action_test.go (mirrors the existing `TestRunDefault_AutoStageNotice_FR18`
  at line 439 but stages ONE file and asserts the singular "(1 file)." + a negative regression guard that
  "(1 files)" is ABSENT). The existing n=2 test (line 460, asserts "(2 files).") is UNCHANGED and STAYS
  GREEN (n=2 takes the plural branch). u.Yellow is a no-op for the test's bytes.Buffer (non-TTY) — proven
  by the existing exact-substring test passing today — so strings.Contains works unchanged. The em-dash `—`
  (U+2014) in "staged — staging" is preserved (recurring codebase gotcha). NO new import (fmt already
  imported; u/stderr/n already in scope). ZERO overlap with the parallel P1.M3.T2.S1 (Issue 5 — touches
  internal/generate/finalize.go + generate_test.go, a DIFFERENT package; that PRP explicitly states
  "P1.M3.T3.S1 (Issue 6, default_action.go — zero overlap)") and P1.M3.T1.S1 (Issue 4 — internal/git/tokengate.go).
  DOCS: none (minor UI text polish, no config/API surface — README sync is P1.M4.T1). Scope: git status ==
  internal/cmd/default_action.go + internal/cmd/default_action_test.go.

---

## Goal

**Feature Goal**: Make the FR18 auto-stage notice grammatically correct for the singular case —
"Nothing staged — staging all changes (1 file)." instead of the current ungrammatical "(1 files)." — while
leaving the plural case (n≥2) byte-identical and the n==0 clean-tree path (which never prints the notice)
untouched. (PRD §h3.5 Issue 6.)

**Deliverable**:
1. `internal/cmd/default_action.go` — at line 150, insert a 3-line `noun` conditional (`noun := "files";
   if n == 1 { noun = "file" }`) immediately before the `fmt.Fprintln`, and change the format string from
   `"Nothing staged — staging all changes (%d files)."` to `"Nothing staged — staging all changes (%d %s)."`
   with `noun` appended to the `fmt.Sprintf` args. Update the trailing comment to note the singular/plural
   conditional (keep the FR18/em-dash/colorized notes).
2. `internal/cmd/default_action_test.go` — ADD `TestRunDefault_AutoStageNoticeSingular_Issue6` (one file
   staged → asserts stderr contains "(1 file)." AND does NOT contain "(1 files)"). The existing
   `TestRunDefault_AutoStageNotice_FR18` (n=2, line 460) is UNCHANGED.

**Success Definition**:
- With exactly one untracked file + `auto_stage_all=true` (default), the notice prints exactly
  `Nothing staged — staging all changes (1 file).` (singular "file").
- With two untracked files (the existing `_FR18` test), the notice still prints exactly
  `Nothing staged — staging all changes (2 files).` (plural "files") — UNCHANGED.
- The n==0 clean-tree path (Issue 7) is UNCHANGED — no notice prints, exit 2.
- The em-dash `—` (U+2014) in "staged — staging" is preserved (no hyphen substitution).
- `go build ./...` clean; `gofmt -l` empty on the 2 files; `go vet ./internal/cmd/...` clean;
  `go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice` green (BOTH the n=2 and n=1 tests);
  `make test` + `make lint` clean.
- `git status --porcelain` == `internal/cmd/default_action.go` + `internal/cmd/default_action_test.go`.

## User Persona (if applicable)

**Target User**: Any Stagecoach user who runs `stagecoach` with nothing staged and the default
`auto_stage_all=true` (the §9.4 FR16/FR18 path) when exactly ONE file changed — e.g. a one-line typo fix
in a single file, or a fresh repo with one new file.

**Use Case**: User edits one file, runs `stagecoach` without `git add`; Stagecoach auto-stages all and
prints the transparent FR18 notice naming the count. Today that notice reads "(1 files)" — ungrammatical.

**User Journey**: user runs `stagecoach` → sees "Nothing staged — staging all changes (1 file)." → proceeds
to generation. (Trivial polish; the notice is the user's confirmation that auto-stage fired.)

**Pain Points Addressed**: Issue 6 — a small but visible grammar defect in a user-facing notice that fires
on the extremely common single-file-edit workflow.

## Why

- **Polish on a high-frequency path**: the auto-stage notice fires whenever a user runs `stagecoach` with
  nothing staged (the default `auto_stage_all=true`). The single-file case is one of the MOST common
  real-world shapes (a one-file fix), so "(1 files)" is seen often. It's a cheap, correct fix.
- **Correctness of user-facing text**: FR18 specifies the notice text; grammar defects undermine the
  "transparent notice" intent (the user should read it as a normal English sentence).
- **Zero risk**: the change is a pure noun swap gated on `n == 1`. The plural path (n≥2) is
  byte-identical; the n==0 path never reaches the line; no control flow, exit code, or config changes.
- **No API/config/doc surface**: the notice is stderr UI text only. The contract says "DOCS: none"; the
  README/docs sync (if any) is the later P1.M4.T1 changeset task.

## What

**User-visible behavior** (stderr notice only; exit codes and all other behavior UNCHANGED):
```
# BEFORE (n=1):  Nothing staged — staging all changes (1 files).     ← ungrammatical
# AFTER  (n=1):  Nothing staged — staging all changes (1 file).      ← correct (Issue 6 fix)
# n≥2 (UNCHANGED): Nothing staged — staging all changes (2 files).
# n==0 (UNCHANGED): no notice printed (clean-tree early return, Issue 7)
```

**Technical change**: insert a `noun` conditional + add one `%s`/arg to a single `fmt.Sprintf`. One new test.

### Success Criteria
- [ ] `internal/cmd/default_action.go:150` region: a `noun` variable is set to `"files"`, switched to
      `"file"` iff `n == 1`, and the format string is `"Nothing staged — staging all changes (%d %s)."`
      with `n, noun` as the `fmt.Sprintf` args (em-dash `—` preserved). The `u.Yellow(...)` wrapper and
      `fmt.Fprintln(stderr, ...)` are UNCHANGED.
- [ ] The `n == 0` clean-tree early return (lines 147-149, the Issue 7 fix) is UNCHANGED — the notice
      never prints for a clean tree.
- [ ] `internal/cmd/default_action_test.go` ADDS `TestRunDefault_AutoStageNoticeSingular_Issue6` that:
      stages exactly ONE untracked file, runs Execute with `--provider stub --single`, and asserts stderr
      CONTAINS `"Nothing staged — staging all changes (1 file)."` AND does NOT contain `"(1 files)"`.
- [ ] The existing `TestRunDefault_AutoStageNotice_FR18` (line 439-461, n=2, asserts `"(2 files)."`) is
      UNCHANGED and still PASSES (n=2 → plural branch).
- [ ] `go build ./...` clean; `gofmt -l` empty on the 2 files; `go vet ./internal/cmd/...` clean.
- [ ] `go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice` green (both tests); `make test` +
      `make lint` clean.
- [ ] `git status --porcelain` == the 2 files ONLY (scope guard).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current line 150 (with its surrounding context: the `StagedFileCount` call, the
`n==0` early return that guarantees n≥1, the `u`/`stderr`/`n` symbols already in scope), the verbatim fix
code (from architecture/minor_fixes.md §Issue 6), the verbatim existing test to mirror (line 439-462), the
exact new test to write (one file, singular assertion + negative guard), the proof that `strings.Contains`
works against `u.Yellow` output (non-TTY no-op, proven by the existing test), the em-dash gotcha, the scope
fences (zero overlap with parallel T2.S1 / T1.S1), and 6 grep guards.

### Documentation & References

```yaml
# MUST READ — the verbatim fix code + the test requirement (Issue 6 spec).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/minor_fixes.md
  section: "## Issue 6: Auto-Stage Notice Grammar \"(1 files)\""
  why: "Gives the EXACT fix (the `noun` conditional + `(%d %s).` format string) and states the existing
        n=2 test (default_action_test.go:460) stays green + a new n=1 test is needed."
  critical: "Confirms the n==0 clean-tree path returns early (lines 147-150) BEFORE the notice, so the
             format string only ever sees n>=1 — the singular case is exactly n==1. Em-dash preserved."

# MUST READ — codebase-specific findings for THIS item (verbatim line 150 + context, the test to mirror,
#              the u.Yellow non-TTY proof, the em-dash gotcha, scope fences, validation commands).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M3T3S1/research/findings.md
  why: "§0 the single production site (line 150, the ONLY site) + the n>=1 guarantee; §1 the verbatim fix;
        §2 the existing _FR18 test (verbatim, to mirror); §3 the exact new n=1 test; §4 why strings.Contains
        works (u.Yellow non-TTY no-op); §5 scope fences (zero overlap); §6 validation commands."

# MUST READ — the parallel sibling PRP (P1.M3.T2.S1, Issue 5 — doubled prefix). It is being implemented
#              IN PARALLEL; it touches internal/generate/finalize.go + generate_test.go — a DIFFERENT
#              PACKAGE from this item (internal/cmd). Read it to confirm ZERO overlap.
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M3T2S1/PRP.md
  why: "Confirms the sibling's scope: 'Touches ONLY internal/generate/finalize.go + generate_test.go' and
        explicitly 'P1.M3.T3.S1 (Issue 6, default_action.go — zero overlap)'. This item's 2 files
        (internal/cmd/default_action.go + default_action_test.go) do NOT overlap. NO merge conflict."
  critical: "Do NOT edit internal/generate/* (the sibling owns them). Do NOT touch the ErrEmptyMessage
             sentinel or main.go's prefix (Issue 5's territory). This item is internal/cmd ONLY."

# MUST READ — the production site to edit (the format string at line 150 + its surrounding context).
- file: internal/cmd/default_action.go
  why: "Line 141: `n, err := g.StagedFileCount(ctx)`. Lines 147-149: the n==0 clean-tree early return
        (Issue 7 fix) that GUARANTEES n>=1 at line 150. Line 150: the FR18 notice `fmt.Fprintln(stderr,
        u.Yellow(fmt.Sprintf(\"Nothing staged — staging all changes (%d files).\", n)))`. The fix inserts
        the `noun` conditional right before line 150 and edits the format string."
  pattern: "fmt + u (ui.UI) + stderr (io.Writer) + n (int) are ALL already in scope at line 150 — NO new
            import or symbol. The u.Yellow(colorizer) wrapper is preserved around the Sprintf."
  gotcha: "The em-dash `—` (U+2014) in 'staged — staging' MUST be preserved (NOT a hyphen `-`). The
           trailing comment `// FR18 (text verbatim, em-dash; colorized)` is updated to note the
           singular/plural conditional (keep the FR18/em-dash/colorized notes)."

# MUST READ — the test file: the existing n=2 test to mirror (line 439) + the helpers it uses.
- file: internal/cmd/default_action_test.go
  why: "TestRunDefault_AutoStageNotice_FR18 (line 439): setupStubRepo(t, \"feat: auto\") + writeFile(t, repo,
        \"u.txt\", ...) + writeFile(t, repo, \"v.txt\", ...) [2 files] → rootCmd.SetArgs([\"--provider\",
        \"stub\", \"--single\"]) → Execute → assert strings.Contains(stderr, '... (2 files).'). Mirror this
        EXACTLY but with ONE writeFile and the singular assertion. saveRootState/restoreRootState (line 440)
        + setupStubRepo/writeFile/gitOut are the helpers (all in this package)."
  pattern: "saveRootState(t)/defer restoreRootState(...); rootCmd.SetOut/SetErr(&buf); rootCmd.SetArgs(...);
            Execute(ctx); assert on errBuf.String() via strings.Contains. Imports bytes/context/strings/
            testing ALL already present (the existing test uses them) — NO new import."
  gotcha: "u.Yellow colorizes ONLY for a TTY; the test's bytes.Buffer is NOT a TTY → u.Yellow is a no-op →
           the captured stderr is the PLAIN string → strings.Contains matches exactly. PROVEN by the
           existing _FR18 test passing today with an exact substring incl. the trailing period. The new n=1
           test uses the IDENTICAL assertion shape → safe (no ANSI stripping needed)."

# CONTEXT — the naming convention for issue-pinning tests (follow it for the new test name).
- file: internal/cmd/default_action_test.go
  why: "The codebase names issue-specific tests TestRunDefault_*_Issue<N>: e.g.
        TestRunDefault_CleanTreeNoAutoStageNotice_Issue7 (line 480),
        TestRunDefault_MissingProviderCommand_Issue3 (line 810),
        TestRunDefault_RepoLocalNoticeOnce_Issue5 (line 1227). Name the new test
        TestRunDefault_AutoStageNoticeSingular_Issue6 to match."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  default_action.go         # EDIT — line 150 region (insert noun conditional + edit format string)
  default_action_test.go    # EDIT — ADD TestRunDefault_AutoStageNoticeSingular_Issue6 (after the _FR18 test ~line 462)
  root_test.go              # READ-ONLY — saveRootState/restoreRootState/setupStubRepo/writeFile/gitOut helpers
internal/generate/
  finalize.go               # READ-ONLY — owned by P1.M3.T2.S1 (Issue 5); DO NOT TOUCH
  generate_test.go          # READ-ONLY — owned by P1.M3.T2.S1; DO NOT TOUCH
internal/git/
  tokengate.go              # READ-ONLY — owned by P1.M3.T1.S1 (Issue 4); DO NOT TOUCH
Makefile                    # test (line ~70, -race); lint (line ~103); build (line ~52)
```

### Desired Codebase tree with files to be added/edited

```bash
internal/cmd/default_action.go         # EDIT — +noun conditional (3 lines) before line 150; edit format string; update comment
internal/cmd/default_action_test.go    # EDIT — +TestRunDefault_AutoStageNoticeSingular_Issue6
# NOTHING ELSE. No edit to any other file, package, PRD, or task file. NO new import/type/flag/dependency.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (em-dash, not hyphen): the notice literal contains a Unicode EM DASH '—' (U+2014) in
// "staged — staging". The edit MUST preserve it (recurring codebase gotcha — the parallel P1.M3.T2.S1
// flags the same invariant for ErrEmptyMessage). Do NOT replace with '-' (hyphen-minus).

// CRITICAL (n>=1 is guaranteed at the edit site): the n==0 clean-tree early return (default_action.go
// lines 147-149, the Issue 7 fix) runs BEFORE line 150, so the notice only ever sees n>=1. The singular
// case is EXACTLY n==1. Do NOT add an n<=0 branch (it's unreachable here); the `noun` conditional is
// `if n == 1 { noun = "file" }` — nothing more.

// CRITICAL (u.Yellow is a no-op for the test's bytes.Buffer): u.Yellow colorizes only when the writer is
// a TTY. The tests pass a *bytes.Buffer (not a TTY) → u.Yellow returns the plain string → the captured
// stderr is the PLAIN notice. PROVEN by the existing TestRunDefault_AutoStageNotice_FR18 passing today
// with an exact strings.Contains match including the trailing period. The new test uses the same shape.

// GOTCHA (no new import): fmt, u (*ui.UI), stderr (io.Writer), n (int) are ALL already in scope at line
// 150. The `noun` conditional is a plain string var — no import needed.

// GOTCHA (keep the plural path byte-identical): for n>=2 the noun stays "files" → the output is EXACTLY
// "Nothing staged — staging all changes (2 files)." (the existing _FR18 assertion). Do NOT change the
// wording, spacing, em-dash, or trailing period for the plural case.

// GOTCHA (scope — do NOT touch the parallel siblings' files): internal/generate/finalize.go +
// generate_test.go are P1.M3.T2.S1 (Issue 5); internal/git/tokengate.go is P1.M3.T1.S1 (Issue 4). This
// item is internal/cmd/default_action.go + default_action_test.go ONLY. README/docs sync is P1.M4.T1.
```

## Implementation Blueprint

### Data models and structure

None. No new types, fields, or data. The edit reuses the existing `n int` (from `g.StagedFileCount`) and
introduces one local `noun string`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/default_action.go — the noun conditional + format string (the ONLY production change)
  - LOCATE: line 150 (grep 'Nothing staged — staging all changes' → the single site).
  - CURRENT (line 150):
      fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 (text verbatim, em-dash; colorized)
  - REPLACE WITH (insert the 3-line noun conditional BEFORE the Fprintln; edit the format string; update comment):
      noun := "files"
      if n == 1 {
          noun = "file" // Issue 6: singular grammar for the one-file auto-stage case (n>=1 guaranteed: n==0 returns early above)
      }
      fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d %s).", n, noun))) // FR18 (text verbatim, em-dash; colorized; singular/plural per Issue 6)
  - PRESERVE UNCHANGED:
      - The `u.Yellow(...)` wrapper and `fmt.Fprintln(stderr, ...)` outer call (only the Sprintf format + args change).
      - The em-dash `—` (U+2014) in "staged — staging".
      - The n==0 clean-tree early return (lines 147-149) — do NOT touch it.
      - Everything else in runDefault (the auto-stage branch structure, exit codes, etc.).
  - NAMING: `noun` (a local string). No exported symbol, no new func.
  - FOLLOW pattern: the existing fmt.Sprintf+Fprintln+u.Yellow idiom at the same line.
  - GOTCHA: `fmt` is already imported; `u`, `stderr`, `n` already in scope. NO new import.
  - VERIFY after edit: `gofmt -w internal/cmd/default_action.go` then `gofmt -l` (empty); `go build ./...`.

Task 2: ADD TestRunDefault_AutoStageNoticeSingular_Issue6 to internal/cmd/default_action_test.go
  - PLACEMENT: immediately AFTER TestRunDefault_AutoStageNotice_FR18 (ends ~line 462), before
    TestRunDefault_CleanTreeNoAutoStageNotice_Issue7 (~line 472). Keep the section-separator comment style
    (the `// ---` block) used between tests in this file.
  - IMPORTS: none new (bytes, context, strings, testing already imported; setupStubRepo/writeFile/gitOut
    are package-local helpers).
  - BODY (mirror _FR18 exactly, but ONE file + singular assertion + negative guard):
      // ---------------------------------------------------------------------------
      // TestRunDefault_AutoStageNoticeSingular_Issue6 — one file auto-staged → "(1 file)." (singular)
      // ---------------------------------------------------------------------------

      // TestRunDefault_AutoStageNoticeSingular_Issue6 pins the Issue 6 fix: when exactly ONE file is
      // auto-staged, the FR18 notice uses the singular "file" (not the ungrammatical "(1 files)").
      // Mirrors TestRunDefault_AutoStageNotice_FR18 (n=2) but with a single untracked file.
      func TestRunDefault_AutoStageNoticeSingular_Issue6(t *testing.T) {
          origArgs, origOut, origErr, origRunE := saveRootState(t)
          defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

          repo := setupStubRepo(t, "feat: one")
          writeFile(t, repo, "only.txt", "content") // ONE untracked file → n=1 after AddAll

          var outBuf, errBuf bytes.Buffer
          rootCmd.SetOut(&outBuf)
          rootCmd.SetErr(&errBuf)
          rootCmd.SetArgs([]string{"--provider", "stub", "--single"})

          err := Execute(context.Background())
          if err != nil {
              t.Fatalf("Execute err=%v, want nil", err)
          }

          stderr := errBuf.String()
          // Issue 6 fix: singular "file" for n=1.
          if !strings.Contains(stderr, "Nothing staged — staging all changes (1 file).") {
              t.Errorf("stderr = %q, want to contain singular FR18 notice '... (1 file).'", stderr)
          }
          // Regression guard: the ungrammatical plural must be ABSENT for n=1.
          if strings.Contains(stderr, "(1 files)") {
              t.Errorf("stderr = %q, must NOT contain ungrammatical '(1 files)' (Issue 6 regression)", stderr)
          }

          // HEAD moved (proves the run completed through the notice path — mirrors _FR18).
          logMsg := gitOut(t, repo, "log", "--format=%s", "-n1")
          if logMsg != "feat: one" {
              t.Errorf("git log subject = %q, want 'feat: one'", logMsg)
          }
      }
  - NAMING: TestRunDefault_AutoStageNoticeSingular_Issue6 (follows the TestRunDefault_*_Issue<N> convention).
  - FOLLOW pattern: TestRunDefault_AutoStageNotice_FR18 (line 439) — identical setup/assertion shape.
  - GOTCHA: u.Yellow is a no-op for errBuf (non-TTY) → strings.Contains sees the plain string (proven by
    _FR18). The em-dash in the assertion string MUST be `—` (U+2014), not a hyphen.

Task 3: VERIFY — the existing n=2 test is UNCHANGED + green; build/vet/format/regression/lint
  - Confirm TestRunDefault_AutoStageNotice_FR18 (line 439-461) was NOT edited (n=2 → plural → still green).
  - gofmt -l internal/cmd/default_action.go internal/cmd/default_action_test.go   # empty
  - go vet ./internal/cmd/...
  - go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice   # BOTH _FR18 (n=2) + _Issue6 (n=1) PASS
  - go test ./internal/cmd/ -race                                     # full cmd regression
  - make test && make lint && make build
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the noun conditional — the entire production change; verbatim from architecture/minor_fixes.md):
noun := "files"
if n == 1 {
	noun = "file" // Issue 6: singular for the one-file case (n>=1 guaranteed: n==0 returns early above)
}
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d %s).", n, noun))) // FR18 (em-dash; colorized; singular/plural)

// PATTERN (the new test — mirror of _FR18 with ONE file + singular + negative guard):
func TestRunDefault_AutoStageNoticeSingular_Issue6(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)
	repo := setupStubRepo(t, "feat: one")
	writeFile(t, repo, "only.txt", "content") // ONE file → n=1
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "--single"})
	if err := Execute(context.Background()); err != nil { t.Fatalf("Execute err=%v", err) }
	stderr := errBuf.String()
	if !strings.Contains(stderr, "Nothing staged — staging all changes (1 file).") { t.Errorf("...singular...") }
	if strings.Contains(stderr, "(1 files)") { t.Errorf("...Issue 6 regression...") }
	// + HEAD-moved check via gitOut (mirror _FR18)
}
```

### Integration Points

```yaml
CLI SURFACE (user-facing stderr):
  - The FR18 auto-stage notice now reads "(1 file)." for n=1 and "(N files)." for n>=2 (UNCHANGED).
  - No new subcommand, flag, exit code, config key, or env var. No behavior change beyond the noun.

CODE (internal/cmd/default_action.go):
  - ONE edit site (line 150 region): +3 lines (noun conditional) + 1 format-string token + comment update.
  - NO new import (fmt/u/stderr/n in scope). NO signature change. NO new type/func/export.

NO database / migration / routes / config / docs / root.go / main.go / go.mod change.
SCOPE FENCES (zero overlap with siblings):
  - Touches ONLY: internal/cmd/default_action.go + internal/cmd/default_action_test.go.
  - Does NOT touch: internal/generate/* (P1.M3.T2.S1 — Issue 5), internal/git/tokengate.go (P1.M3.T1.S1 —
    Issue 4), main.go, exitcode.go, root.go, go.mod, any PRD/task file.
  - README/docs sync is P1.M4.T1 (the contract says "DOCS: none — minor UI text polish").
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the edit is pure stdlib fmt — no platform tags — but build all anyway).
go build ./...
# Expected: clean.

# Vet the changed package.
go vet ./internal/cmd/...
# Expected: clean.

# Format the 2 touched files.
gofmt -l internal/cmd/default_action.go internal/cmd/default_action_test.go
# Expected: empty. If listed: gofmt -w internal/cmd/default_action.go internal/cmd/default_action_test.go

# Lint.
make lint   # errcheck/gosimple/govet/ineffassign/staticcheck/unused
# Expected: zero errors. (noun is used; the new test func is referenced by `go test`; no unused symbols.)

# Scope guard: ONLY the 2 files changed.
git status --porcelain
# Expected: internal/cmd/default_action.go, internal/cmd/default_action_test.go. ZERO changes elsewhere
#           (esp. NOT internal/generate/*, internal/git/tokengate.go, main.go, root.go).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new + existing auto-stage-notice tests (the -run substring matches BOTH).
go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice
# Expected: 2 PASS —
#   TestRunDefault_AutoStageNotice_FR18           (n=2 → "...(2 files)."  UNCHANGED, still green)
#   TestRunDefault_AutoStageNoticeSingular_Issue6 (n=1 → "...(1 file)."   NEW, singular + no "(1 files)")

# Full cmd-package regression (the auto-stage branch is shared — ensure no sibling test broke).
go test ./internal/cmd/ -race
# Expected: green.

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
make build

# Manual: the n=1 case (one untracked file + default auto_stage_all).
cd "$(mktemp -d)" && git init -q && bin="$(pwd)/../../stagecoach"
# create a repo with one commit (so it's a "mature" repo) + one new untracked file:
printf 'x\n' > only.txt
"$bin" 2>&1 | grep "staging all changes"
# Expected (AFTER fix): "Nothing staged — staging all changes (1 file)."   (singular "file")

# Manual: the n=2 case (regression — plural UNCHANGED).
printf 'y\n' > second.txt
# (with only.txt already committed by the prior run, now second.txt + a new third file untracked → n=2)
printf 'z\n' > third.txt
"$bin" 2>&1 | grep "staging all changes"
# Expected: "Nothing staged — staging all changes (2 files)."   (plural "files" — UNCHANGED)

# Manual: the n==0 clean-tree case (Issue 7 — NO notice).
"$bin" 2>&1 | grep "staging all changes" || echo "(no notice printed — correct for clean tree)"
# Expected: no notice (the n==0 early return skips it); exit 2 "Nothing to commit."

# Expected: n=1 → "(1 file)."; n>=2 → "(N files)." (unchanged); n==0 → no notice.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the OLD hardcoded plural format string is GONE.
grep -n '(%d files)' internal/cmd/default_action.go
# Expect: ZERO hits.

# Guard 2: the NEW conditional format string is present.
grep -n '(%d %s)' internal/cmd/default_action.go
grep -n 'noun := "files"' internal/cmd/default_action.go
grep -n 'if n == 1' internal/cmd/default_action.go
grep -n 'noun = "file"' internal/cmd/default_action.go
# Expect: 1 hit each.

# Guard 3: the em-dash (U+2014) is preserved (NOT a hyphen). The line still contains "staged — staging".
grep -n 'staged — staging' internal/cmd/default_action.go
# Expect: 1 hit (with the U+2014 em-dash, not '-').

# Guard 4: the u.Yellow + Fprintln wrappers are preserved (only the Sprintf changed).
grep -n 'fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged' internal/cmd/default_action.go
# Expect: 1 hit.

# Guard 5: the n==0 clean-tree early return is UNCHANGED (Issue 7 fix preserved).
grep -n 'if n == 0' internal/cmd/default_action.go
grep -n 'Nothing to commit.' internal/cmd/default_action.go
# Expect: the n==0 branch + the early return still present (untouched).

# Guard 6: the new singular test exists + the existing plural test is untouched.
grep -n 'func TestRunDefault_AutoStageNoticeSingular_Issue6' internal/cmd/default_action_test.go
grep -n 'staging all changes (1 file).' internal/cmd/default_action_test.go
grep -n 'staging all changes (2 files).' internal/cmd/default_action_test.go   # existing n=2 test STILL there
# Expect: the new test func + the (1 file). singular assertion + the (2 files). plural assertion all present.

# Guard 7: scope — only the 2 cmd files (zero overlap with parallel siblings).
git status --porcelain
# Expect: internal/cmd/default_action.go + internal/cmd/default_action_test.go ONLY.
git diff --name-only | grep -E 'internal/generate/|internal/git/tokengate|main\.go|root\.go|go\.mod' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l` empty on the 2 touched files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/cmd/ -v -run TestRunDefault_AutoStageNotice` green (BOTH n=2 and n=1)
- [ ] `go test ./internal/cmd/ -race` green (full cmd regression)
- [ ] `make test` + `make build` clean

### Feature Validation
- [ ] n=1 → notice reads "Nothing staged — staging all changes (1 file)." (singular) (grep guards 2,6)
- [ ] n≥2 → notice reads "(N files)." UNCHANGED (the existing _FR18 test still passes) (grep guard 6)
- [ ] n==0 → no notice (clean-tree early return preserved) (grep guard 5)
- [ ] em-dash `—` preserved (grep guard 3)
- [ ] u.Yellow + Fprintln wrappers preserved (grep guard 4)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY internal/cmd/default_action.go + internal/cmd/default_action_test.go (guard 7)
- [ ] NO edit to internal/generate/* (P1.M3.T2.S1 — Issue 5), internal/git/tokengate.go (P1.M3.T1.S1 — Issue 4),
      main.go, exitcode.go, root.go, go.mod, or any PRD/task file
- [ ] NO new import, type, flag, exported symbol, or dependency
- [ ] NO README/docs sync (P1.M4.T1 — contract: "DOCS: none")

### Code Quality & Docs
- [ ] The `noun` conditional is minimal (`if n == 1`), with a comment citing Issue 6
- [ ] The trailing comment on the Fprintln notes the singular/plural conditional (FR18/em-dash/colorized kept)
- [ ] The new test mirrors the existing _FR18 idiom (saveRootState/restoreRootState/setupStubRepo/writeFile/gitOut)
- [ ] The new test includes a negative regression guard (`!strings.Contains(stderr, "(1 files)")`)

---

## Anti-Patterns to Avoid

- ❌ Don't replace the em-dash `—` (U+2014) with a hyphen `-`. The notice literal and the test assertion
  both use the em-dash ("staged — staging"). A hyphen breaks the assertion and the FR18 "text verbatim"
  invariant. (Recurring codebase gotcha — the parallel P1.M3.T2.S1 flags the same for ErrEmptyMessage.)
- ❌ Don't add an `n <= 0` or `n == 0` branch to the noun conditional. The n==0 clean-tree early return
  (lines 147-149, the Issue 7 fix) runs BEFORE line 150, so the notice only ever sees n>=1. The conditional
  is exactly `if n == 1 { noun = "file" }` — adding an n==0 case is dead code and conflates Issue 6 with
  Issue 7 (which is already fixed).
- ❌ Don't change the plural path. For n>=2 the output MUST stay byte-identical ("(2 files).") — the
  existing `TestRunDefault_AutoStageNotice_FR18` (n=2) asserts exactly that and must stay green. Only the
  n==1 case differs.
- ❌ Don't strip or special-case `u.Yellow`. It's a no-op for the test's `bytes.Buffer` (non-TTY) — PROVEN
  by the existing _FR18 test passing with an exact `strings.Contains` match. Keep the wrapper; assert on
  the plain substring (no ANSI handling needed).
- ❌ Don't introduce a new import, helper, or package. `fmt`, `u`, `stderr`, `n` are all in scope at line
  150; `noun` is a plain local string. The test reuses `bytes/context/strings/testing` (already imported)
  + the package-local `setupStubRepo/writeFile/gitOut` helpers.
- ❌ Don't edit the existing `TestRunDefault_AutoStageNotice_FR18` (n=2). The contract says it "continues
  to assert '(2 files)'" — ADD a sibling test for n=1, don't parameterize/rewrite the existing one (that
  would needlessly risk the green n=2 coverage).
- ❌ Don't touch the parallel siblings' files. internal/generate/finalize.go + generate_test.go are
  P1.M3.T2.S1 (Issue 5); internal/git/tokengate.go is P1.M3.T1.S1 (Issue 4). This item is internal/cmd
  ONLY (different package — zero overlap, zero merge conflict).
- ❌ Don't sync README/docs here. The contract says "DOCS: none — minor UI text polish, no config/API
  surface." The README/docs review is P1.M4.T1.
- ❌ Don't skip the negative regression guard in the new test. Asserting `strings.Contains(stderr,
  "(1 file).")` alone would pass even if someone reverted to "(1 files)" via a different code path; the
  `!strings.Contains(stderr, "(1 files)")` guard LOCKS the fix.
