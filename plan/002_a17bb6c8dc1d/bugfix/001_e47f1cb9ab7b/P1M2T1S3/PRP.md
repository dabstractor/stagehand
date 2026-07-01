# PRP — P1.M2.T1.S3: Defensive HEAD-movement guard for stager invocations

## Goal

**Feature Goal**: Add a defense-in-depth guard in the decompose stager path that snapshots `HEAD`
before each stager invocation and ABORTS the run (hard, non-rescue error) if `HEAD` moved after the
stager returns. This closes the safety gap documented in Issue 2 / PRD §19: the stager is contractually
only allowed to mutate the INDEX (`git add` / `git apply --cached`), NEVER refs — but pi's unsoped
`tooled_flags` profile (and any future provider) cannot be structurally prevented from running
`git commit` / `git update-ref` / `git reset`. This guard is the runtime safety net.

**Deliverable**:
1. A new sentinel error `ErrStagerMovedHEAD` in `internal/decompose/stager.go`.
2. A HEAD pre/post-snapshot guard inside `invokeStagerRetry` in `internal/decompose/decompose.go`
   that (a) compares HEAD before/after each stager call, (b) returns `ErrStagerMovedHEAD` on any
   movement, and (c) treats that error as a HARD abort that bypasses the existing
   retry-once-then-empty (FR-M8/M12) logic.
3. Two tests in `internal/decompose/decompose_test.go`: a HEAD-movement violation test (rogue stager
   seam that commits) and a happy-path test (well-behaved stager stages via `git add`).

**Success Definition**: A misbehaving stager that moves HEAD causes `Decompose` to return an error for
which `errors.Is(err, ErrStagerMovedHEAD)` is true; the CLI maps it to exit 1 with the message
`stager moved HEAD from <pre> to <post> — aborting; the stager agent mutated refs which it must not do`.
A correctly-behaved stager (index-only mutation) is unaffected (guard passes). `go test -race ./...`
and `golangci-lint run` are green.

## Why

- **Safety/security claim gap (PRD §19, Issue 2)**: the PRD sells the stager as "structurally
  constrained, cannot commit/amend/push." pi's profile has NO tool allowlist, so the claim is
  instructionally — not structurally — enforced. This guard delivers the structural guarantee at
  runtime for ALL providers (defense-in-depth). Sibling tasks S1 (claude allowlist) and S2 (honest pi
  docs) harden the profiles / docs; S3 is the runtime backstop they explicitly defer to.
- **Scope boundary**: this is the ONLY stager-path guard. It must NOT alter ref-mutation semantics
  (stagehand still owns all ref mutations via `UpdateRefCAS`), must NOT change the retry/empty
  behavior for ordinary stager failures, and must NOT touch the arbiter / message / planner paths.
- **Non-rescue by design**: when the guard fires there is no snapshot to restore — the stager
  corrupted repo state. Abort (exit 1) is the correct outcome (contrast with `*RescueError` → exit 3).

## What

User-visible behavior: NONE (no config/API/flag surface). The guard is internal defense-in-depth.

Internal behavior:
- Before the first stager call for a concept, capture `preStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)`.
- After each stager call (first attempt and retry), capture `postStagerHEAD`; if
  `preStagerHEAD != postStagerHEAD`, return `ErrStagerMovedHEAD` (wrapped with `%w`) carrying both SHAs.
- `ErrStagerMovedHEAD` is a HARD error: `invokeStagerRetry` must return it immediately — it must NOT
  be subject to the retry-once-then-empty (FR-M8/M12) treatment.
- The error propagates unchanged through `runLoop` → `Decompose` → `runDecompose` →
  `handleDecomposeError`, which (because it is not `*RescueError` / `*CASError`) maps it to
  `exitcode.New(exitcode.Error, err)` → main prints `stagehand: <msg>` and exits 1.

### Success Criteria

- [ ] `ErrStagerMovedHEAD` sentinel exists in `stager.go` with a Go doc comment explaining it is
  defense-in-depth for providers that cannot be flag-scoped (pi), that the stager must only mutate the
  index, and that firing means a hard abort (non-rescue).
- [ ] Injecting a stager test-seam that moves HEAD (e.g. `git commit --allow-empty`) makes `Decompose`
  return an error where `errors.Is(err, ErrStagerMovedHEAD)` is true.
- [ ] A well-behaved stager seam (stages via `git add`, no ref mutation) completes the run normally —
  the guard does not false-positive.
- [ ] Ordinary stager failures (non-zero exit, no HEAD movement) STILL get retry-once-then-empty
  (FR-M8/M12) — the guard does not regress that behavior.
- [ ] `go build ./...`, `go test -race ./internal/decompose/...`, `go test -race ./...`, and
  `golangci-lint run` all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ YES — every file
path, function, signature, sentinel pattern, test helper, and error-propagation path is named below
with line anchors. The guard touches exactly ONE function (`invokeStagerRetry`) and adds ONE sentinel.

### Documentation & References

```yaml
# MUST READ — the function being modified (the guard's home)
- file: internal/decompose/decompose.go
  why: contains `invokeStagerRetry` (closure inside `runLoop`, ~L330-347) — the stager invocation
       path the guard wraps; and `invokeStager` (the test-seam dispatcher, L485-491) it calls.
  pattern: `invokeStagerRetry` currently treats ANY `invokeStager` error as retryable (retry once,
           then return nil = empty-skip). The new guard's HARD error MUST bypass this.
  gotcha: do NOT put the guard in `invokeStager` itself — `invokeStagerRetry` would then RETRY the
          corrupted stager and silently treat HEAD movement as empty. The guard + the
          `errors.Is(err, ErrStagerMovedHEAD)` short-circuit BOTH belong in `invokeStagerRetry`.

- file: internal/decompose/stager.go
  why: home of `ErrStagerFailed` sentinel (L40) and `stageConcept` (the real tooled stager).
  pattern: copy the `var Err... = errors.New("decompose: ...")` + rich Go doc-comment style for the
           new `ErrStagerMovedHEAD` sentinel (this is the Go equivalent of the "[Mode A] JSDoc" the
           contract requests).

- file: internal/git/git.go
  why: `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)` — the exact method the guard calls
       twice (pre/post). Returns ("", true, nil) on an unborn repo, so pre/post compare equal unless
       the stager created the first commit (post="<sha>" → caught). Branching on the SHA string is
       correct here (not on isUnborn).
  gotcha: discard the `isUnborn` and `err` returns with `preStagerHEAD, _, _ := ...` — a RevParseHEAD
          infra failure here is not actionable mid-loop; the pre/post string comparison is the signal.

- file: internal/decompose/roles.go
  why: `Deps` struct (L54) carries the unexported `stager` test-seam field
       `func(ctx, Deps, prompt.PlannerCommit) error`, dispatched by `invokeStager`. No change needed —
       the guard wraps `invokeStager` so BOTH the seam and production `stageConcept` paths are guarded.

- file: internal/exitcode/exitcode.go
  why: confirms `exitcode.For` returns `Error` (=1) for any non-nil error that is not NothingToCommit
       / Rescue / Timeout / CAS. `ErrStagerMovedHEAD` therefore maps to exit 1 with NO new wiring.
  pattern: the new sentinel needs NO entry here — it falls through to `return Error`.

- file: internal/cmd/default_action.go
  why: `handleDecomposeError` (~L319) — `ErrStagerMovedHEAD` is not `*RescueError`/`*CASError`, so it
       hits the `exitcode.New(exitcode.Error, err)` branch → main prints `stagehand: <msg>`. No change.

- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md
  why: the architecture note for this issue; section "(c) Add a defensive HEAD-movement guard" is the
       authoritative spec this task implements. Cross-check the abort/no-snapshot contract.

- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M2T1S3/research/guard_design.md
  why: the design rationale — especially the retry-bypass insight (guard must live in
       invokeStagerRetry, not invokeStager) and the verified error-propagation path.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go      # runLoop + invokeStagerRetry (MODIFY) + invokeStager (read-only dispatch)
  stager.go         # ErrStagerFailed sentinel + stageConcept (ADD ErrStagerMovedHEAD here)
  decompose_test.go # stager-seam tests + dcm* helpers (ADD 2 tests here)
internal/git/git.go # RevParseHEAD interface + impl (READ ONLY)
internal/exitcode/  # exit mapping (READ ONLY — no change)
internal/cmd/default_action.go # handleDecomposeError (READ ONLY — no change)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/stager.go        # MODIFY: add `var ErrStagerMovedHEAD` sentinel + doc comment
internal/decompose/decompose.go     # MODIFY: rewrite invokeStagerRetry closure to add the guard
internal/decompose/decompose_test.go# MODIFY: add TestDecompose_StagerMovedHEAD + TestDecompose_StagerGuardHappyPath
# NO new files. NO changes to git.go, exitcode.go, default_action.go, or any provider file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: invokeStagerRetry's retry-once-then-empty (FR-M8/M12) must NOT swallow ErrStagerMovedHEAD.
//   Today it treats every non-nil invokeStager error as "retry once, then empty-skip". The guard's
//   HARD error must short-circuit BOTH branches via errors.Is(err, ErrStagerMovedHEAD).
// CRITICAL: put the guard inside invokeStagerRetry (wrapping invokeStager), NOT inside invokeStager.
//   invokeStager is the shared dispatch for the seam + stageConcept; if the guard lived there, the
//   retry logic in invokeStagerRetry would re-run the corrupted stager.
// GOTCHA: capture preStagerHEAD ONCE at the top of invokeStagerRetry (before the first invokeStager).
//   This is correct: the guard guarantees HEAD never silently moves between the failed first attempt
//   and the retry (an attempt that ALSO moved HEAD is itself caught → HARD abort).
// GOTCHA: use %w wrapping so errors.Is(err, ErrStagerMovedHEAD) is true in the unit test:
//   fmt.Errorf("%w: stager moved HEAD from %s to %s — aborting; the stager agent mutated refs which it must not do", ErrStagerMovedHEAD, pre, post)
// GOTCHA: on an unborn repo RevParseHEAD returns ("", true, nil); pre/post string compare still works
//   (both "" unless the stager created the root commit). Do NOT special-case isUnborn.
// GOTCHA: discard RevParseHEAD's isUnborn/err with `preStagerHEAD, _, _ := ...` — do not abort on a
//   mid-loop RevParseHEAD infra error (non-actionable); the SHA comparison is the safety signal.
```

## Implementation Blueprint

### Data models and structure

None. This task adds one sentinel error and modifies one function closure. No new types, structs,
config, or migrations. The only "data" is the two `string` SHAs captured from `RevParseHEAD`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD sentinel `ErrStagerMovedHEAD` to internal/decompose/stager.go
  - IMPLEMENT: `var ErrStagerMovedHEAD = errors.New("decompose: stager moved HEAD")`
  - PLACE: immediately AFTER the existing `var ErrStagerFailed = errors.New(...)` block (~L40).
  - DOC ([Mode A] Go doc-comment, matching the rich style of ErrStagerFailed + ErrCASFailed in git.go):
      * ErrStagerMovedHEAD is the sentinel for a SAFETY-VIOLATION: a stager moved HEAD (committed /
        amended / update-ref / reset). The stager is contractually allowed to mutate the INDEX only
        (git add / git apply --cached); refs move ONLY at UpdateRefCAS (PRD §18.1/§19).
      * This guard is DEFENSE-IN-DEPTH for providers that cannot be flag-scoped (pi's tooled profile
        has no tool allowlist — Issue 2 / PRD §19). It is NOT a rescue (no snapshot to restore): the
        stager corrupted repo state, so the run aborts (exit 1). Contrast *RescueError (exit 3).
      * Produced by the HEAD pre/post-snapshot guard in invokeStagerRetry (decompose.go). Wrapped with
        %w so errors.Is(err, ErrStagerMovedHEAD) is true for test assertions + exit-code mapping.
  - DEPENDENCIES: `errors` is already imported in stager.go.
  - NAMING: `ErrStagerMovedHEAD` (matches `ErrStagerFailed` / `ErrCASFailed` casing convention).

Task 2: ADD the HEAD guard inside invokeStagerRetry (internal/decompose/decompose.go)
  - FIND: the `invokeStagerRetry` closure inside `runLoop` (~L330-347). Current body:
        if cerr := ctx.Err(); cerr != nil { return cerr }
        err := invokeStager(ctx, deps, concept)
        if err == nil { return nil }
        deps.Verbose.VerboseRetry(1, ...)
        if err2 := invokeStager(ctx, deps, concept); err2 == nil { return nil }
        deps.Verbose.VerboseRetry(2, ...)
        return nil
  - REWRITE to (preserve the ctx.Err() guard, the Verbose calls, and the empty-skip return EXACTLY;
    only ADD the HEAD snapshot + runOnce wrapper + the two HARD-abort short-circuits):
        if cerr := ctx.Err(); cerr != nil {
            return cerr // ctx cancelled → abort (drainMsg + partial), not skip-everything
        }
        // Issue 2 / PRD §19 defense-in-depth: a correctly-behaving stager mutates the INDEX only,
        // never refs. Snapshot HEAD once; abort (HARD, non-rescue) if any stager call moves it.
        preStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)
        // runOnce invokes the stager once and aborts if HEAD moved during that call.
        runOnce := func() error {
            serr := invokeStager(ctx, deps, concept)
            postStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)
            if preStagerHEAD != postStagerHEAD {
                return fmt.Errorf("%w: stager moved HEAD from %s to %s — aborting; the stager agent mutated refs which it must not do", ErrStagerMovedHEAD, preStagerHEAD, postStagerHEAD)
            }
            return serr
        }
        if err := runOnce(); err == nil {
            return nil
        } else if errors.Is(err, ErrStagerMovedHEAD) {
            return err // HARD — safety violation; do NOT retry, do NOT empty-skip
        }
        deps.Verbose.VerboseRetry(1, fmt.Sprintf("stager failed for %q; retrying once", concept.Title))
        if err2 := runOnce(); err2 == nil {
            return nil
        } else if errors.Is(err2, ErrStagerMovedHEAD) {
            return err2 // HARD — safety violation even on the retry
        }
        deps.Verbose.VerboseRetry(2, fmt.Sprintf("stager failed twice for %q; treating concept as empty (FR-M8)", concept.Title))
        return nil // empty: freezeSnapshot yields tree[i]==prevTree → S1's empty-skip (UNCHANGED)
  - NAMING/PLACEMENT: keep `invokeStagerRetry` a closure inside `runLoop`; do not extract a new
    package-level symbol (minimizes diff; matches existing structure).
  - PRESERVE: the runLoop call site (`if err := invokeStagerRetry(concept); err != nil { drainMsg(inflight); return commits, nil, err }`)
    needs NO change — it already propagates any non-nil error (including the new HARD error) and
    drains the in-flight message goroutine. The arbiter does NOT run on this abort (§18.3) — correct.
  - GOTCHA: `errors` and `fmt` are already imported in decompose.go — no import changes.

Task 3: ADD TestDecompose_StagerMovedHEAD to internal/decompose/decompose_test.go
  - PURPOSE: a rogue stager seam that moves HEAD → Decompose returns ErrStagerMovedHEAD.
  - MODEL: the setup of TestDecompose_Overlap / TestDecompose_EmptyConceptSkip (planner JSON +
           stub message manifest + stager seam). Use the existing dcm* helpers.
  - STEPS:
      bin := stubtest.Build(t)
      repo := t.TempDir()
      dcmInitRepo(t, repo)
      dcmCommitRaw(t, repo, "initial")          // BORN repo → HEAD has a real SHA to move away from
      dcmWriteFile(t, repo, "a.txt", "aaa\n")   // untracked → dirty tree (FR-M1 routing satisfied)
      plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
      plannerM := dcmPlannerManifest(t, bin, plannerJSON)
      messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})  // unreached, but populate
      roles   := dcmAllRoles(t, bin, stubtest.Options{Out: ""})              // all-roles stub set
      roles.Planner = plannerM
      roles.Message = messageM
      deps := dcmDeps(t, repo, roles)
      // ROGUE seam: stages nothing, instead COMMITS → moves HEAD.
      deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
          dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "rogue: moved HEAD")
          return nil
      }
      _, err := Decompose(context.Background(), deps)
      if err == nil { t.Fatal("expected ErrStagerMovedHEAD, got nil") }
      if !errors.Is(err, ErrStagerMovedHEAD) {
          t.Fatalf("expected ErrStagerMovedHEAD, got %v", err)
      }
      if !strings.Contains(err.Error(), "stager moved HEAD") {
          t.Errorf("error message missing 'stager moved HEAD'; got: %s", err.Error())
      }
  - NAMING: `TestDecompose_StagerMovedHEAD`. Place near the other stager-seam tests
            (after TestDecompose_EmptyConceptSkip, before the invokeStager unit tests).
  - NOTE: importing `errors`, `context`, `strings`, `prompt` — all already imported in decompose_test.go.

Task 4: ADD TestDecompose_StagerGuardHappyPath to internal/decompose/decompose_test.go
  - PURPOSE: a well-behaved stager (git add, no ref mutation) completes normally — guard passes.
  - MODEL: identical structure to Task 3 but the seam stages the file via `dcmStagerSeam`
           (git add) instead of committing. Reuse dcmStagerSeam directly.
  - STEPS:
      bin := stubtest.Build(t)
      repo := t.TempDir()
      dcmInitRepo(t, repo)
      dcmCommitRaw(t, repo, "initial")
      dcmWriteFile(t, repo, "a.txt", "aaa\n")
      plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
      plannerM := dcmPlannerManifest(t, bin, plannerJSON)
      messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
      roles   := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
      roles.Planner = plannerM
      roles.Message = messageM
      deps := dcmDeps(t, repo, roles)
      deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})  // git add only
      result, err := Decompose(context.Background(), deps)
      if err != nil { t.Fatalf("happy-path guard false-positive: %v", err) }
      if len(result.Commits) != 1 { t.Fatalf("Commits len = %d, want 1", len(result.Commits)) }
      // HEAD advanced exactly once (the published commit), via UpdateRefCAS — NOT via the stager.
      if got := dcmLogCount(t, repo); got != 2 {  // initial + 1 published
          t.Errorf("commit count = %d, want 2", got)
      }
      if status := dcmStatusPorcelain(t, repo); status != "" {
          t.Errorf("status = %q, want empty (clean)", status)
      }
  - NAMING: `TestDecompose_StagerGuardHappyPath`. Place immediately after Task 3's test.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the guard is a runOnce closure that wraps invokeStager (the existing test-seam
// dispatcher). It snapshots HEAD exactly once per concept (preStagerHEAD) and compares after each
// stager call. ErrStagerMovedHEAD short-circuits retry/empty at BOTH error checks.
//
// CRITICAL INVARIANT: a correctly-behaving stager mutates ONLY the index (git add / git apply
// --cached); refs move ONLY at UpdateRefCAS. HEAD movement during a stager call is therefore a
// contract violation → HARD abort (no snapshot to restore → NOT a rescue).
invokeStagerRetry := func(concept prompt.PlannerCommit) error {
    if cerr := ctx.Err(); cerr != nil {
        return cerr
    }
    preStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx) // defense-in-depth (Issue 2 / PRD §19)
    runOnce := func() error {
        serr := invokeStager(ctx, deps, concept)
        postStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)
        if preStagerHEAD != postStagerHEAD {
            return fmt.Errorf("%w: stager moved HEAD from %s to %s — aborting; the stager agent mutated refs which it must not do",
                ErrStagerMovedHEAD, preStagerHEAD, postStagerHEAD)
        }
        return serr
    }
    if err := runOnce(); err == nil {
        return nil
    } else if errors.Is(err, ErrStagerMovedHEAD) {
        return err // HARD — safety violation; bypass retry-once-then-empty
    }
    deps.Verbose.VerboseRetry(1, fmt.Sprintf("stager failed for %q; retrying once", concept.Title))
    if err2 := runOnce(); err2 == nil {
        return nil
    } else if errors.Is(err2, ErrStagerMovedHEAD) {
        return err2 // HARD — even on retry
    }
    deps.Verbose.VerboseRetry(2, fmt.Sprintf("stager failed twice for %q; treating concept as empty (FR-M8)", concept.Title))
    return nil
}

// PATTERN — sentinel + doc (stager.go), mirroring ErrStagerFailed and ErrCASFailed's prose density:
// ErrStagerMovedHEAD is the sentinel for a stager SAFETY VIOLATION (the stager moved HEAD). ...
// defense-in-depth for providers that cannot be flag-scoped (pi). NOT a rescue (exit 1, not 3).
var ErrStagerMovedHEAD = errors.New("decompose: stager moved HEAD")
```

### Integration Points

```yaml
DATABASE: none
CONFIG:   none (no new flag/env; the guard is unconditional defense-in-depth)
ROUTES:   none — the error propagates through the EXISTING path:
            invokeStagerRetry → runLoop → Decompose → runDecompose → handleDecomposeError
            → exitcode.New(exitcode.Error, err) → main (exit 1, prints "stagehand: <msg>")
          No edit is required in exitcode.go or default_action.go.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file edit — fix before proceeding.
go build ./...
go vet ./internal/decompose/...
golangci-lint run ./internal/decompose/...

# Expected: zero errors. The `errors`/`fmt` imports already exist in decompose.go; `errors` already
# exists in stager.go — no new imports needed. errcheck is enabled (the `_, _, _ := RevParseHEAD(...)`
# discards are EXPLICIT underscores, which errcheck accepts).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two new tests, isolated and verbose.
go test -race -run 'TestDecompose_StagerMovedHEAD|TestDecompose_StagerGuardHappyPath' ./internal/decompose/ -v

# Full decompose package — must stay green (no regression to retry-once-then-empty / overlap / empty-skip).
go test -race ./internal/decompose/ -v

# Expected: all pass. If TestDecompose_StagerMovedHEAD fails with "got <nil>", the guard is not firing
# (check the runOnce comparison). If TestDecompose_StagerGuardHappyPath fails, the guard is
# false-positiving (check that the seam stages via git add, not commit).
```

### Level 3: Integration Testing (System Validation)

```bash
# Full repo build + race tests (the PRD's own green bar).
go build ./...
go test -race ./...

# Lint the whole repo (Makefile `lint` target).
golangci-lint run

# Expected: green. No new test flakiness; the guard adds two deterministic git calls per stager
# attempt (RevParseHEAD) — negligible, and the tests use real temp repos (t.TempDir).
```

### Level 4: Manual / Domain-Specific Validation (optional, defense-in-depth confidence)

```bash
# Build the CLI and confirm the error maps to exit 1 with the right message. Requires a real agent
# (pi) OR a stub stager binary that runs `git commit`. The unit test in Task 3 is the primary gate;
# this is a sanity check that the full CLI path surfaces the message.
go build -o /tmp/stagehand ./cmd/stagehand
# (synthetic): point the stager at a script that commits, run `stagehand` in a dirty repo, and confirm:
#   - exit code 1
#   - stderr contains "stager moved HEAD from <pre> to <post> — aborting; the stager agent mutated refs which it must not do"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds.
- [ ] `go test -race ./...` is green (incl. the 2 new tests).
- [ ] `golangci-lint run` is green (errcheck/govet/gosimple on the modified closure).
- [ ] No new imports were needed in decompose.go or stager.go (verify with `git diff`).

### Feature Validation
- [ ] `errors.Is(err, ErrStagerMovedHEAD)` is true when the stager moves HEAD (Task 3 test).
- [ ] A well-behaved stager (git add) completes normally (Task 4 test) — no false positive.
- [ ] Ordinary stager failures still retry-once-then-empty (FR-M8/M12) — verify by re-running the
      existing `TestDecompose_EmptyConceptSkip` and `TestDecompose_Overlap` (they must still pass).
- [ ] The error message contains "stager moved HEAD from <pre> to <post>" and "...mutated refs...".
- [ ] The error maps to exit 1 (falls through `exitcode.For` → `Error`; not rescue/timeout/CAS).

### Code Quality Validation
- [ ] `ErrStagerMovedHEAD` has a Go doc comment explaining: index-only stager contract, defense-in-depth
      for un-scoped providers (pi), HARD/non-rescue semantics (exit 1 vs 3).
- [ ] The guard lives in `invokeStagerRetry` (NOT `invokeStager`) — retry bypass verified.
- [ ] preStagerHEAD is captured once; postStagerHEAD after each `invokeStager` call.
- [ ] `runLoop`'s call site and drain logic are UNCHANGED (the error propagates through the existing path).

### Documentation & Scope
- [ ] No user-facing config / API / flag surface added (internal defense-in-depth only).
- [ ] No edits to PRD.md, git.go, exitcode.go, default_action.go, or any provider/*.go file.
- [ ] Sibling tasks S1 (claude allowlist) / S2 (pi docs) are NOT regressed — this guard is additive.

---

## Anti-Patterns to Avoid

- ❌ Do NOT put the guard inside `invokeStager` — `invokeStagerRetry` would then RETRY the corrupted
  stager and silently empty-skip on a HEAD movement (the exact bug this guard exists to prevent).
- ❌ Do NOT let `ErrStagerMovedHEAD` flow through the retry-once-then-empty branches — it MUST
  short-circuit at every `errors.Is(err, ErrStagerMovedHEAD)` check.
- ❌ Do NOT re-capture preStagerHEAD between the first attempt and the retry (the guard guarantees HEAD
  did not silently move; one snapshot is correct and matches the contract).
- ❌ Do NOT special-case the unborn repo — the `pre != post` string comparison is correct for both born
  and unborn (unborn pre/post are both "" unless the stager created the root commit).
- ❌ Do NOT add an `ErrStagerMovedHEAD` case to `exitcode.For` or `handleDecomposeError` — the existing
  fall-through already maps it to exit 1.
- ❌ Do NOT abort the run on a `RevParseHEAD` infra error mid-loop — discard `isUnborn`/`err`; the SHA
  comparison is the safety signal, and a transient git-read failure is not a stager violation.

---

## Confidence Score

**9 / 10** — The change is tightly scoped: one sentinel + one closure rewrite + two tests. The only
non-obvious risk (guard placement vs. retry logic) is fully resolved by the research note and the
explicit `errors.Is` short-circuits. The error-mapping path is verified end-to-end (no new wiring).
One-pass success is highly likely given the exact code skeletons, line anchors, and modeled tests.
