name: "P1.M4.T2.S1 — Add n==0 early-return for the auto-stage notice on a clean tree (Issue 7)"
description: |

  Cosmetic UX fix for PRD Issue 7. When nothing is staged, `auto_stage_all` is on, and the working
  tree is already clean, Stagecoach currently prints the misleading line
  `Nothing staged — staging all changes (0 files).` (FR18 template) right before `Nothing to commit.`
  (exit 2). Add a 4-line early return between the `StagedFileCount` error guard and the FR18 notice
  so a clean tree goes straight to the exit-2 "Nothing to commit." path, skipping the "(0 files)" line.
  Exit code + message are IDENTICAL to the existing FR17 path (§15.4), so no contract surface changes.

---

## Goal

**Feature Goal**: In the `cfg.AutoStageAll` branch of `runDefault` (`internal/cmd/default_action.go`),
when the post-`AddAll` staged file count `n == 0`, return `exitcode.New(exitcode.NothingToCommit,
errors.New("Nothing to commit."))` BEFORE printing the FR18 notice. This removes the misleading
"staging all changes (0 files)." line on a clean tree while preserving the exact §15.4 exit-2 outcome.

**Deliverable**: A minimal, localized edit to `internal/cmd/default_action.go` (the `cfg.AutoStageAll`
switch case) plus a regression test in `internal/cmd/default_action_test.go` asserting the "(0 files)"
notice is absent on a clean tree. No other files change.

**Success Definition**:
- On a clean tree with `auto_stage_all=true`, stderr does NOT contain `staging all changes`.
- The process still exits with code 2 (`NothingToCommit`) and main prints `Nothing to commit.`
  (the `err.Error() != ""` guard in main still fires — message is non-empty).
- The `n > 0` happy path (`TestRunDefault_AutoStageNotice_FR18`) is UNCHANGED — the FR18 notice
  with the real file count still prints verbatim on stderr.
- `go build ./...`, `go vet ./...`, `go test -race ./...`, `make lint` all pass.

## User Persona (if applicable)

**Target User**: Stagecoach end user running `stagecoach` on an already-clean working tree with the
default `auto_stage_all = true` config.

**Use Case**: A user runs `stagecoach` again after their previous commit (or after staging nothing),
expecting a clean `Nothing to commit.` message. Today they see a confusing "staging all changes
(0 files)" line that implies work happened when nothing did.

**User Journey**: `stagecoach` (clean tree) → stderr shows only `Nothing to commit.` → exit 2.

**Pain Points Addressed**: The "(0 files)" phrasing suggests a no-op staging action was taken, which
is confusing/misleading. FR18's literal `N` template is correct for `N>0` but reads badly for `N==0`.

## Why

- **Business value**: Polished, honest UX on the empty case — a documented "Minor / Nice to Fix"
  issue (PRD h3.6 / Issue 7) found during the v1.0 QA pass.
- **Integration**: Pure CLI-layer cosmetic change in the auto-stage state machine (PRD §9.4
  FR16–FR20). No effect on the snapshot/atomic-commit pipeline, the public `pkg/stagecoach` API,
  config resolution, or exit-code semantics (§15.4).
- **Problems solved**: Removes a misleading stderr line. For whom: every user with the default
  `auto_stage_all = true` hitting a clean tree.

## What

User-visible behavior: with nothing staged + `auto_stage_all` on + a clean tree, stderr is now just
`Nothing to commit.` (exit 2). The "(0 files)" staging notice is suppressed in this single case.
All other auto-stage behavior (the `N>0` notice, `--all`/`-a`, `--no-auto-stage`) is unchanged.

### Success Criteria

- [ ] Clean tree + `auto_stage_all=true`: stderr does NOT contain `staging all changes` (exit 2).
- [ ] Clean tree: exit code is 2 (`NothingToCommit`) — unchanged from before.
- [ ] Unstaged files present + `auto_stage_all=true`: stderr STILL contains the verbatim
      `Nothing staged — staging all changes (N files).` notice (em-dash) for `N = actual count`.
- [ ] The downstream `if !hasStaged` FR17 re-check at `default_action.go` (current line ~85) is
      LEFT IN PLACE as a belt-and-suspenders guard (it also covers a race between
      StagedFileCount and HasStagedChanges).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact 4-line insertion, its exact location (between two named existing
lines), the exact exit code/message to reuse, the exact test harness helpers, and the exact
validation commands are all specified below.

### Documentation & References

```yaml
# MUST READ - Include these in your context window

- file: internal/cmd/default_action.go
  why: The ONLY file to edit. The cfg.AutoStageAll switch case holds the StagedFileCount call,
        the FR18 notice Fprintln, and the FR17 re-check. Edit goes between the StagedFileCount
        error guard and the Fprintln notice.
  pattern: |
    The current block (runDefault, inside `if !hasStaged { switch { case cfg.AutoStageAll:`):
        if err := g.AddAll(ctx); err != nil { ... }
        n, err := g.StagedFileCount(ctx)
        if err != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
        }
        fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18
        hasStaged, err = g.HasStagedChanges(ctx)
        ...
        if !hasStaged {
            return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))  // FR17 path
        }
  gotcha: The em-dash in the FR18 string is U+2014 (—), NOT a hyphen-minus. Do NOT alter that line.

- file: internal/exitcode/exitcode.go
  why: Defines exitcode.NothingToCommit (=2) and exitcode.New(code, err) (*ExitError). For(err)
        resolves an *ExitError to its Code. main prints err.Error() when it is non-empty, so a
        non-nil err carrying "Nothing to commit." still prints to the user as today.
  critical: Reuse exitcode.NothingToCommit and errors.New("Nothing to commit.") VERBATIM — must
        match the existing FR17 path exactly so §15.4 exit-2 semantics are byte-identical.

- file: internal/git/git.go
  why: StagedFileCount runs `git diff --cached --name-only` and returns the count of non-empty
        lines. On a clean tree AddAll is a documented no-op (git add -A exits 0, index unchanged)
        so StagedFileCount returns 0 — that 0 is what flows into the "(%d files)" template today.
  pattern: StagedFileCount returns (int, error); code != 0 → wrapped error, else non-empty-line count.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_and_autostage.md
  why: PART B (lines ~202–315) is the authoritative design for THIS fix: the exact notice code,
        how N is computed, where to insert, and the recommended "early-return" form (chosen over
        the "gate only the notice on n>0" alternative). §B.3 gives the exact patch to apply.
  section: "PART B — Auto-stage "(0 files)" cosmetic notice (Issue 7)" → B.3 "Where to add the if N == 0 short-circuit"

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: Decision D5 records the chosen fix (early-return form) and the rejected alternative, and
        mandates LEAVING the downstream `if !hasStaged` FR17 re-check as a belt-and-suspenders guard.
  section: "D5 — Fix Issue 7 with an early-return when the post-AddAll staged count is 0"
```

### Current Codebase tree (relevant slice)

```bash
internal/
  cmd/
    default_action.go          # ← EDIT HERE (cfg.AutoStageAll case)
    default_action_test.go     # ← ADD/EXTEND regression test here
  exitcode/
    exitcode.go                # exitcode.NothingToCommit (=2), exitcode.New, exitcode.For
  git/
    git.go                     # StagedFileCount (returns 0 on clean tree), AddAll (no-op on clean)
plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/
  architecture/
    decisions.md               # D5 — chosen fix
    seam_config_and_autostage.md  # PART B — exact patch + tests
Makefile                       # build / test / lint targets
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/cmd/default_action.go       # MODIFIED: +4-line early return in cfg.AutoStageAll case
internal/cmd/default_action_test.go  # MODIFIED: regression test asserting no "(0 files)" notice on clean tree
```
No new files. No new packages. No config/schema/migration changes.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: exit code + message must be IDENTICAL to the existing FR17 path
//   (default_action.go line ~85 / handleGenError generate.ErrNothingToCommit branch).
//   Reuse EXACTLY: exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")).
//   main.go prints err.Error() only when non-empty — "Nothing to commit." is non-empty, so the
//   user-facing message is unchanged (exit 2, "Nothing to commit."). §15.4 preserved.

// GOTCHA: Do NOT delete the downstream `if !hasStaged { return ...Nothing to commit. }` re-check.
//   D5 mandates leaving it as a belt-and-suspenders guard (it also catches a race where the index
//   changes between StagedFileCount and HasStagedChanges).

// GOTCHA: The FR18 string uses an em-dash (—, U+2014), not a hyphen. Do not touch that line.
//   The `n > 0` path (TestRunDefault_AutoStageNotice_FR18) must remain byte-identical.

// GOTCHA: StagedFileCount is NOT HasStagedChanges. StagedFileCount counts non-empty lines of
//   `git diff --cached --name-only` (exit 0 always; never --quiet). On a clean tree it returns 0.
//   That 0 is the trigger condition — no extra git call is needed.
```

## Implementation Blueprint

### Data models and structure

None. No data models, schemas, or types change. This is a 4-line control-flow edit + a test.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/cmd/default_action.go — add the n==0 early return
  - LOCATE: the `case cfg.AutoStageAll:` branch inside `if !hasStaged { switch { ... } }` in runDefault.
  - FIND the existing two adjacent lines (the insertion point sits BETWEEN them):
        n, err := g.StagedFileCount(ctx)
        if err != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
        }
        fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 ...
  - INSERT between the error guard's closing `}` and the Fprintln (verbatim, matches the existing
    FR17 path so the §15.4 exit-2 outcome is byte-identical):
        if n == 0 {
            // Clean tree: AddAll staged nothing. Skip the FR18 "(0 files)" notice and go straight
            // to the FR17 exit-2 path (Issue 7 cosmetic fix; D5).
            return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
        }
  - PRESERVE: the Fprintln notice line (unchanged for n>0) and the downstream
    `hasStaged, err = g.HasStagedChanges(ctx)` ... `if !hasStaged { return ... }` FR17 re-check
    (belt-and-suspenders guard per D5 — do NOT remove).
  - NAMING/IMPORTS: no new imports (errors, fmt, exitcode already imported).
  - PLACEMENT: single localized edit inside the cfg.AutoStageAll case.

Task 2: MODIFY internal/cmd/default_action_test.go — regression test for the absent notice
  - The clean-tree test TestRunDefault_NothingStaged_FR17 (≈line 308) ALREADY sets up exactly the
    scenario (clean tree, AutoStageAll default true, --provider stub) and asserts exit 2 + HEAD
    unchanged. ADD an assertion that stderr does NOT contain the misleading notice:
        stderr := errBuf.String()
        if strings.Contains(stderr, "staging all changes") {
            t.Errorf("stderr = %q, want NO auto-stage notice on a clean tree (Issue 7)", stderr)
        }
    (strings is already imported in this test file.)
  - ALTERNATIVE / ALSO ACCEPTABLE: add a dedicated TestRunDefault_CleanTreeNoAutoStageNotice_Issue7
    mirroring TestRunDefault_NothingStaged_FR17's setup (setupStubRepo with NO uncommitted files →
    clean tree; rootCmd.SetArgs([]string{"--provider","stub"}); Execute(ctx); assert exitcode.For(err)
    == exitcode.NothingToCommit AND stderr does NOT contain "staging all changes" AND HEAD unchanged).
  - FOLLOW pattern: the existing TestRunDefault_NothingStaged_FR17 (saveRootState/restoreRootState,
    outBuf/errBuf, rootCmd.SetOut/SetErr/SetArgs, Execute(context.Background()), exitcode.For).
  - COVERAGE: assert (a) exit 2, (b) NO "staging all changes" in stderr, (c) HEAD unchanged.
  - DO NOT weaken TestRunDefault_AutoStageNotice_FR18 — the n>0 notice (em-dash, "(2 files).") must
    still be asserted present (it is the unchanged happy path).
```

### Implementation Patterns & Key Details

```go
// The complete cfg.AutoStageAll case AFTER the edit (for reference; only the `if n == 0` block is new):
case cfg.AutoStageAll:
    // FR16/FR18: auto-stage all, print the transparent notice, re-check.
    if err := g.AddAll(ctx); err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
    }
    n, err := g.StagedFileCount(ctx)
    if err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
    }
    if n == 0 {
        // Clean tree: AddAll staged nothing. Skip the FR18 "(0 files)" notice and go straight
        // to the FR17 exit-2 path (Issue 7 cosmetic fix; D5).
        return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
    }
    fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 (text verbatim, em-dash; colorized)
    hasStaged, err = g.HasStagedChanges(ctx)
    if err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("staged changes check: %w", err))
    }
    if !hasStaged {
        // FR17: clean tree even after auto-stage (belt-and-suspenders; also catches a race).
        return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
    }

// Why the early-return (not "gate only the notice on n>0"): D5 — n==0 is a strict subset of
// !hasStaged here, and a single early return is cleaner. The §15.4 exit-2 + "Nothing to commit."
// message is byte-identical to the FR17 path, so exit-code semantics and the main-printed message
// are unchanged.
```

### Integration Points

```yaml
DATABASE: none
CONFIG:   none (no config field added/removed/changed; auto_stage_all semantics untouched)
ROUTES:   none
NO new imports, no new files, no schema/migration changes. Pure control-flow edit + test.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet after the edit (Go has no separate format/lint beyond these + golangci-lint)
go build ./...            # Expected: zero errors
go vet ./...              # Expected: zero errors
gofmt -l internal/cmd/default_action.go internal/cmd/default_action_test.go   # Expected: empty output (already formatted)
make lint                 # golangci-lint run — Expected: zero issues
```

### Level 2: Unit / Component Tests

```bash
# Run the affected test file (race detector on, matching `make test`)
go test -race ./internal/cmd/ -run 'RunDefault_(NothingStaged_FR17|AutoStageNotice_FR18|NoAutoStage_FR19|AllFlag)' -v
# Expected: all pass, including the new/extended clean-tree assertion (no "(0 files)" notice).

# Full suite (matches `make test`)
go test -race ./...
# Expected: all packages pass.
```

### Level 3: Integration / Manual Sanity (the actual user journey)

```bash
# Build the binary
make build    # → ./bin/stagecoach

# Reproduce the BEFORE bug (on the unpatched tree) to confirm the scenario, then confirm the AFTER:
cd "$(mktemp -d)" && git init -q && git config user.email t@t && git config user.name t \
  && git commit -q --allow-empty -m "init"
# Clean tree now. Run with auto_stage_all default (true):
./bin/stagecoach --provider <any> ; echo "exit=$?"
# AFTER the fix: stderr contains ONLY "Nothing to commit." (no "staging all changes (0 files)").
# exit=2. HEAD unchanged (git log --format=%s -n1 == "init").

# Positive control: the n>0 notice still prints verbatim on an actually-dirty tree:
echo x > a.txt && ./bin/stagecoach --provider <any>
# stderr should contain: Nothing staged — staging all changes (1 files).   (em-dash)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Confirm exit-code semantics are byte-identical to the pre-fix behavior (regression guard):
#   On the clean-tree path the exit code MUST still be 2 (NothingToCommit) — same as before.
#   main.go's err.Error() != "" guard must still print "Nothing to commit."
# This is covered by TestRunDefault_NothingStaged_FR17 (exit 2) — keep it green.

# Coverage gate (PRD §20.3) is unaffected (internal/cmd is not in the 85%-gate package set), but run:
make coverage    # informational; Expected: no regression in internal/{git,provider,generate,config}
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l` on the two edited files is empty
- [ ] `make lint` (golangci-lint) passes
- [ ] `go test -race ./...` passes (full suite)

### Feature Validation

- [ ] Clean tree + auto_stage_all: stderr does NOT contain `staging all changes` (exit 2)
- [ ] Clean tree: exit code is 2 (`NothingToCommit`) — unchanged
- [ ] Dirty tree + auto_stage_all: the verbatim FR18 notice (em-dash, real N) STILL prints (TestRunDefault_AutoStageNotice_FR18 green)
- [ ] The downstream `if !hasStaged` FR17 re-check is still present (not removed)
- [ ] No new files / packages / imports / config fields

### Code Quality Validation

- [ ] Follows existing exitcode/exitcode.New + errors.New pattern (identical to FR17 path)
- [ ] Edit placement matches PART B.3 / D5 exactly (between StagedFileCount error guard and Fprintln)
- [ ] Anti-patterns avoided: no duplicated exit string literal drift (reuse the exact string),
      no deletion of the belt-and-suspenders re-check, no change to the em-dash FR18 line

### Documentation & Deployment

- [ ] No doc changes required (FR18 template unchanged for n>0; pure cosmetic/UX — see work-item DOCS: none)

---

## Anti-Patterns to Avoid

- ❌ Don't delete the downstream `if !hasStaged` FR17 re-check (D5: keep it as belt-and-suspenders).
- ❌ Don't change the exit code/message to anything other than the exact FR17 values
  (`exitcode.NothingToCommit` + `errors.New("Nothing to commit.")`) — §15.4 semantics must stay identical.
- ❌ Don't touch the FR18 Fprintln line (em-dash, "(%d files)." plural) — the n>0 path is unchanged.
- ❌ Don't add a new git call or recompute the count — `n` from `StagedFileCount` is already in hand.
- ❌ Don't weaken `TestRunDefault_AutoStageNotice_FR18` to "make the new test pass" — both must pass.
- ❌ Don't introduce a new test file; extend `default_action_test.go` (the harness helpers live there).

---

## Confidence Score: 9/10

One-pass success is highly likely: the change is a 4-line localized insertion at a precisely-named
location, reusing an already-imported package and an already-existing exit-code/message literal, with
the exact patch, the exact test harness, and the exact validation commands all specified above. The
single residual risk is a minor line-number drift in the work-item's quoted references (the `:78` /
`:85` line numbers are current-as-of-writing; locate by the code text, not the line number).
