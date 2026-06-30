---
name: "P1.M3.T1.S2 — Replace the dry-run single-pass with the bounded dedupe/retry loop; skip commit-tree/update-ref ONLY (PRD Issue 2 / FR49)"
description: |

  THE BUG (PRD Issue 2, Severity Major): `pkg/stagehand.runPipeline`'s dry-run branch is a **third,
  degraded** implementation of the generate loop — a SINGLE attempt with **no duplicate check, no
  parse-retry (FR29), no bounded retries (FR30–FR33)**. So `--dry-run` can print a DIFFERENT message
  than a real commit would produce (it shows the first attempt even when that attempt duplicates a
  recent subject that a real run would reject-and-retry past). FR49 ("run the full
  diff→snapshot→generate→parse→**duplicate-check** pipeline, print the resulting message, do not create
  the commit") is violated. Dry-run also returns a bare `ErrTimeout` sentinel (no `TreeSHA`, no
  candidate) on timeout instead of a `*RescueError`.

  THE FIX (binding decision **D3** in `architecture/decisions.md`; recon in
  `architecture/seam_dryrun.md` §"Start Here"): the SAME bounded generate→parse→dedupe loop already
  exists one block further down in `runPipeline` (the SystemExtra commit path) — it is a faithful copy
  of `generate.CommitStaged`'s loop. **Delete the dry-run single-pass block; let that existing loop run
  for BOTH `dryRun` and `!dryRun`; then on success branch — `dryRun` returns `Result{CommitSHA:""}`
  (skipping ONLY `commit-tree`/`update-ref`), `!dryRun` does the commit as today.** This collapses
  three near-duplicate code paths into one loop with two tails and fixes Issue 2.

  ⚠️ **S1 (Issue 6) is ALREADY COMPLETE** — `WriteTree` + `signal.SetSnapshot` are now unconditional
  (the `if !dryRun` gate is gone). So `treeSHA` is ALWAYS set when execution reaches the dry-run block,
  and rescue is already armed in dry-run. S2 does NOT touch the snapshot; it only replaces the
  generation strategy below it. (See research design-decisions.md §0.)

  ⚠️ **THE change is minimal and mechanical:** (1) DELETE the `// ---- DryRun: single pass, no commit.
  ----` block (`if dryRun { … }`), (2) INSERT a dry-run success early-return AFTER the existing
  `if !success { return *RescueError }` and BEFORE the `CommitTree` tail, (3) leave the existing loop
  body and the commit tail UNCHANGED. Every primitive the loop needs (`diff`, `recent`, `sysPrompt`,
  `resolved`, `model`, `treeSHA`, `parentSHA`, `candidate`) is already in scope. No new import, no new
  type, no new dep. (§1/§2/§3.)

  ⚠️ **THE one test S2 MUST update** — `TestGenerateCommit_Timeout` / subtest `"dryrun"` currently
  asserts dry-run timeout is NOT a `*RescueError` (`errors.As(err, &re)` must be false). After S2,
  dry-run timeout returns `*RescueError{Kind:ErrTimeout, TreeSHA}`. S2 flips that ONE assertion to
  mirror the `"commit_path"` subtest (expect `*RescueError` + non-empty `TreeSHA`). Leaving it red
  would break the tree; S2 owns this single flip. The NET-NEW dry-run tests (dup-retry, parse-retry,
  exhaustion, snapshot-exists) are **S3** — S2 does not add them. (§4.)

  ⚠️ **THE signal disarm on dry-run success** — S1 armed rescue for dry-run (`SetSnapshot`). On dry-run
  SUCCESS, S2 must DISARM via `signal.ClearSnapshot()` (sets `snapTree=""` → no rescue print; see
  `internal/signal/signal.go` `handle()` — `if tree != ""` guards the rescue). Do NOT call
  `RestoreDefault()` in dry-run (it neutering the handler is for the `update-ref` window, which dry-run
  skips). Dry-run timeout/exhaustion returns `*RescueError` and does NOT disarm (the CLI prints the
  §18.3 rescue block from it; the dangling tree is intentional). (§5.)

  ⚠️ **DO NOT perturb `generate.CommitStaged`** — D3 rejected adding a "no-commit" flag to the frozen,
  heavily-tested orchestrator. The dedup lives INSIDE `runPipeline` (which already held the second
  copy of the loop). S2 touches ONLY `pkg/stagehand/stagehand.go` + `pkg/stagehand/stagehand_test.go`
  + (Mode A) `docs/cli.md`. (§7.)

  Deliverable: EDIT `pkg/stagehand/stagehand.go` `runPipeline` (delete dry-run short-circuit; add
  dry-run success early-return with `signal.ClearSnapshot()`); EDIT `pkg/stagehand/stagehand_test.go`
  (flip the ONE `TestGenerateCommit_Timeout`/`dryrun` assertion); EDIT `docs/cli.md:26` (affirm
  `--dry-run` runs the full dup-check/retry pipeline). No other files. `go build ./...`, `go vet
  ./...`, `gofmt -l`, and `go test -race ./...` all green.

---

## Goal

**Feature Goal**: Make `pkg/stagehand.runPipeline`'s dry-run path run the **same** bounded
generate→parse→dedupe loop as the commit path (FR29 parse-retry, FR30–FR33 duplicate rejection +
bounded retries), then stop **immediately before** `commit-tree`/`update-ref` — so `--dry-run`
previews the EXACT message a real commit would produce (including retrying past a duplicate first
attempt). Satisfies PRD §9.12 FR49 ("run the full diff→snapshot→generate→parse→duplicate-check
pipeline, print the resulting message, but do not create the commit or move HEAD. Exit 0.") and
collapses three near-duplicate loop implementations into one.

**Deliverable**:
1. **EDIT** `pkg/stagehand/stagehand.go` (`runPipeline`):
   - **DELETE** the entire `// ---- DryRun: single pass, no commit. ----` block (the `if dryRun { … }`
     short-circuit).
   - **INSERT** a dry-run success early-return after `if !success { return &generate.RescueError{…} }`
     and before the `CommitTree` tail: `if dryRun { signal.ClearSnapshot(); return Result{CommitSHA:"",
     Subject: generate.ExtractSubject(msg), Message: msg, Provider: deps.Manifest.Name, Model: model}, nil }`.
   - Leave the existing loop body and the commit tail (CommitTree → RestoreDefault → UpdateRefCAS →
     CASError → ClearSnapshot → DiffTree → return) **unchanged** — they now serve only the `!dryRun`
     path. Update the now-stale loop/section comments.
2. **EDIT** `pkg/stagehand/stagehand_test.go`: flip the ONE assertion in `TestGenerateCommit_Timeout`
   / `"dryrun"` from "must NOT be `*RescueError`" to "must BE `*RescueError{Kind:ErrTimeout}` with
   non-empty `TreeSHA`" (mirror the `"commit_path"` subtest).
3. **EDIT** `docs/cli.md` (Mode A): the `--dry-run` table row (line ~26) affirms it runs the full
   duplicate-check/retry pipeline (not just "Generate and print the message; do not commit").

No other files. **No new import, no new type, no new dep** (`go.mod`/`go.sum` unchanged).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...`
green (incl. the updated timeout subtest and all pre-existing tests). A dry run whose FIRST attempt
duplicates a recent subject now retries to a unique message (verifiable in S3's net-new tests, but
the behavior is in place after S2). Dry-run timeout now returns `*RescueError{Kind:ErrTimeout,
TreeSHA: <non-empty>}`. Dry-run still returns `CommitSHA:""`, leaves HEAD unchanged, and exits 0 on
success. The loop appears exactly once in `runPipeline` (no third copy).

## User Persona

**Target User**: A developer evaluating "should I trust this AI commit message?" via `--dry-run`
before committing (PRD US9 — "judge quality before trusting it"). Transitively the CLI default action
(`internal/cmd/default_action.go` → `pkg/stagehand.GenerateCommit` with `DryRun:true`) and any
library consumer calling `GenerateCommit(ctx, Options{DryRun:true})`.

**Use Case**: `stagehand --dry-run` (or `stagehand --provider X --dry-run`) prints the commit message
a real run WOULD produce — same dedupe, same retry, same parse-failure handling — without creating the
commit. The printed message is byte-identical to what `stagehand` (no `--dry-run`) would commit.

**User Journey**: `--dry-run` → `GenerateCommit(DryRun:true)` → `runPipeline(dryRun=true)` →
RevParseHEAD → StagedDiff → WriteTree (S1, unconditional) → SetSnapshot → buildSysPrompt →
RecentSubjects(50) → **[S2] the full bounded loop** → on success: ClearSnapshot + return
`Result{CommitSHA:""}` (no CommitTree, no UpdateRefCAS) → CLI prints the message, exit 0.

**Pain Points Addressed**: A user who previews with `--dry-run`, sees "feat: init", trusts it, then
runs for real and gets a DIFFERENT message ("feat: unique after retry") — the preview lied about
quality. After S2 the preview is faithful: it runs the identical pipeline and shows the message that
would actually be committed.

## Why

- **FR49 compliance.** PRD §9.12 FR49 requires the "full … **duplicate-check** pipeline" in dry-run.
  Today dry-run omits the duplicate-check (and parse-retry, and bounded retries). S2 makes dry-run run
  the real loop.
- **Faithful preview (PRD US9).** Dry-run exists so users can judge quality before trusting the model.
  A preview that can diverge from the real commit defeats its purpose. S2 makes them identical up to
  the commit step.
- **Collapses duplication (architecture smell).** `runPipeline` already had TWO copies of the loop
  (the SystemExtra commit path) plus the degraded dry-run single-pass = three implementations. S2
  deletes the degraded one and routes dry-run through the shared loop. (D3; seam_dryrun.md §6.)
- **Consistent rescue contract.** Dry-run timeout/exhaustion now returns `*RescueError{TreeSHA}`
  (real TreeSHA from S1's unconditional snapshot), matching the commit path — instead of a bare
  `ErrTimeout` with no recovery context.
- **Boundaries respected.** S2 does NOT touch the frozen `generate.CommitStaged`, does NOT redo S1's
  snapshot work, and leaves net-new test coverage to S3.

## What

A surgical edit to `pkg/stagehand/stagehand.go` `runPipeline`: remove the dry-run single-pass
short-circuit, let the already-present bounded loop serve both paths, and add a dry-run success
early-return that skips only `commit-tree`/`update-ref`. Plus one flipped test assertion and one doc
row. No new types, no new functions, no new imports, no I/O changes, no config changes. The loop's
behavior on the commit path is byte-identical to before (it is the same code, now also reached when
`dryRun` is true — minus the commit tail).

### Success Criteria

- [ ] `runPipeline` contains the generate→parse→dedupe loop **exactly once** (no second/third copy).
      The `// ---- DryRun: single pass, no commit. ----` block is GONE.
- [ ] The loop runs for both `dryRun` and `!dryRun`. After `if !success { return *RescueError }`, a
      `if dryRun { … return Result{CommitSHA:""} }` early-return precedes the `CommitTree` tail.
- [ ] Dry-run success returns `Result{CommitSHA:"", Subject: generate.ExtractSubject(msg), Message:
      msg, Provider: deps.Manifest.Name, Model: model}` and calls `signal.ClearSnapshot()` first.
- [ ] Dry-run success does NOT call `CommitTree`, `UpdateRefCAS`, `RestoreDefault`, or `DiffTree`.
- [ ] Dry-run timeout returns `&generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeSHA,
      ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}` (real, non-empty `TreeSHA` — S1's
      unconditional snapshot). NOT the bare `ErrTimeout` sentinel.
- [ ] Dry-run exhaustion (`!success`) returns `&generate.RescueError{Kind: generate.ErrRescue,
      TreeSHA: treeSHA, …}`. Dry-run parse-retry uses `parseFail`+`retryInstr`; dry-run dup-retry
      uses `rejected`+`IsDuplicate` — identical to the commit path.
- [ ] The commit path (`!dryRun`) is byte-for-byte unchanged: same loop body, same
      CommitTree→RestoreDefault→UpdateRefCAS→CASError→ClearSnapshot→DiffTree→return tail.
- [ ] `TestGenerateCommit_Timeout`/`"dryrun"` now asserts the error IS a `*RescueError` with
      `errors.Is(err, ErrTimeout)` true AND `re.TreeSHA != ""` (mirrors `"commit_path"`).
- [ ] `docs/cli.md` `--dry-run` row affirms the full dup-check/retry pipeline runs.
- [ ] `generate.CommitStaged` is UNCHANGED (frozen). S1's snapshot code is UNCHANGED.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/... pkg/...` clean; `go test -race ./...`
      green; `git diff --exit-code go.mod go.sum` empty.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
current dry-run block to delete (quoted verbatim below + in `stagehand.go`), the exact loop that stays
(visible in `stagehand.go` and quoted in `architecture/seam_dryrun.md` §1), the exact early-return to
insert (copy-ready in §Implementation Blueprint), the one test assertion to flip (quoted verbatim),
the signal disarm rationale (`internal/signal/signal.go` `handle`/`ClearSnapshot`), and the binding
decision D3. No external research needed — this is an in-repo refactor of existing, tested code.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: the BINDING decision D3 (and the rejected alternative "add a no-commit flag to CommitStaged").
  section: "## D3 — Fix Issues 2 & 6 by routing dry-run through the full loop"
  critical: D3 item 2 (replace the dry-run single-pass with the loop already present for SystemExtra),
       item 3 (on dry-run success skip ONLY commit-tree/update-ref), item 4 (dry-run timeout now
       *RescueError with real TreeSHA → the locked-in timeout test must flip). D3 explicitly REJECTS
       perturbing generate.CommitStaged — dedup inside runPipeline.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_dryrun.md
  why: the recon — exact code quotes of the real loop (§1), the dry-run divergence (§2), the reusable
       seams (§3), the FR status table (§4), the dry-run tests (§5), the architecture diagram (§6),
       and the "Start Here" implementation recipe.
  section: "## Start Here" + §2(b) (the dry-run short-circuit to delete) + §5 (the locked-in timeout
       test).
  critical: the "Start Here" recipe is EXACTLY this subtask's plan. Note the recon's line numbers are
       slightly stale (S1 already un-gated the snapshot); trust the LIVE source for exact lines.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M3T1S2/research/design-decisions.md
  why: this subtask's own design note — current-state-after-S1 (§0), the exact block to delete (§1),
       the loop that stays (§2), the early-return to insert (§3), the one test to flip (§4), the signal
       disarm rationale (§5), primitives already imported (§6), scope boundary (§7), FR coverage (§8),
       docs (§9).
  critical: §0 (S1 is DONE — do not redo the snapshot), §4 (S2 MUST flip the timeout assertion or the
       tree breaks), §5 (ClearSnapshot not RestoreDefault on dry-run success).

- file: pkg/stagehand/stagehand.go   (THE file you EDIT — runPipeline)
  section: `runPipeline` — the `// ---- DryRun: single pass, no commit. ----` block (DELETE) and the
           commit-path loop immediately after it (KEEP, now serves both paths).
  why: this is the entire change surface. The loop is already a faithful copy of generate.CommitStaged;
       you are deleting the degraded third copy and routing dry-run through it.
  pattern: mirror the loop body EXACTLY as it already exists (do not "improve" it — it is tested). The
           only addition is the dry-run early-return + `signal.ClearSnapshot()`.
  gotcha: `retryInstr := *resolved.RetryInstruction` is declared just above the loop — keep it there
          (the loop uses it for FR29). `model` is resolved before the loop — reuse it in the dry-run
          early-return. Do NOT move these.

- file: internal/generate/generate.go   (READ ONLY — the authoritative loop, do NOT edit)
  section: `CommitStaged` step 5 (the generate+dedupe loop) and step 7-9 (commit tail).
  why: this is the FROZEN, tested reference the runPipeline loop mirrors. Read it to confirm the
       runPipeline loop is faithful (it is). Do NOT add a no-commit flag here (D3 rejected it).
  gotcha: generate.go's loop uses unexported `recentSubjects`/`buildSystemPrompt`; runPipeline uses the
          exported `deps.Git.RecentSubjects` + its own `buildSysPrompt` — that difference is correct
          and pre-existing; do not "fix" it.

- file: internal/signal/signal.go   (READ ONLY — confirm disarm semantics)
  section: `SetSnapshot` (line ~164), `ClearSnapshot` (~184), `RestoreDefault` (~195), and `handle`
           (~113 — the `if tree != ""` rescue guard).
  why: S1 armed rescue for dry-run. On dry-run SUCCESS you must DISARM so a post-return signal does not
       print a rescue block. `ClearSnapshot()` sets `snapTree=""` → `handle` skips rescue. Confirmed.
  gotcha: call `ClearSnapshot()` on dry-run success. Do NOT call `RestoreDefault()` (its job is the
          update-ref window, which dry-run skips). Dry-run timeout/exhaustion must NOT disarm (the
          *RescueError carries the context; dangling tree is intentional).

- file: pkg/stagehand/stagehand_test.go   (THE file you EDIT — one assertion)
  section: `TestGenerateCommit_Timeout` / subtest `"dryrun"` (lines ~224-250).
  why: this test LOCKS IN the current divergent behavior (bare `ErrTimeout`, `errors.As(&re)` false).
       S2 changes that behavior, so S2 must flip this assertion. The `"commit_path"` subtest right
       below it is the template for the new assertion.
  pattern: the new `"dryrun"` assertion should read identically to `"commit_path"`:
       `errors.As(err, &re)` TRUE, `errors.Is(err, ErrTimeout)` TRUE, `re.TreeSHA != ""`.
  gotcha: do NOT add net-new tests here (dup-retry/parse-retry/exhaustion/snapshot) — that is S3. S2
          flips ONLY the `errors.As` line (and adds the TreeSHA check to match commit_path).

- url: (PRD §9.12 FR49 — already in your context as selected_prd_content h3.1; the authoritative req)
  why: FR49 verbatim — "run the full diff→snapshot→generate→parse→duplicate-check pipeline, print the
       resulting message, but do not create the commit or move HEAD. Exit 0." This is the acceptance bar.
  critical: the "full … duplicate-check pipeline" clause is what S2 satisfies; the "do not create the
       commit or move HEAD" clause is preserved (dry-run still returns CommitSHA:"").
```

### Current Codebase tree (relevant slice)

```bash
go.mod / go.sum                              # UNCHANGED — S2 adds NO dep (all primitives already imported)
pkg/stagehand/
  stagehand.go          # EDIT — runPipeline: delete dry-run short-circuit; add dry-run success early-return
  stagehand_test.go     # EDIT — flip ONE assertion in TestGenerateCommit_Timeout/"dryrun"
internal/
  generate/generate.go      # FROZEN (CommitStaged) — READ ONLY
  generate/dedupe.go        # ExtractSubject / IsDuplicate — READ ONLY (already used)
  git/git.go                # WriteTree/RecentSubjects/etc — READ ONLY
  provider/{parse,executor,manifest}.go  # ParseOutput/Execute/Render — READ ONLY
  prompt/payload.go         # BuildUserPayload — READ ONLY
  signal/signal.go          # SetSnapshot/ClearSnapshot/RestoreDefault — READ ONLY
  cmd/default_action.go     # CLI — UNCHANGED
docs/cli.md             # EDIT (Mode A) — --dry-run row affirms full pipeline
docs/how-it-works.md    # VERIFY ONLY (no dry-run mention found; no edit unless a divergence claim exists)
Makefile                # UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
pkg/stagehand/
  stagehand.go          # EDITED — runPipeline: one loop (was: dry-run short-circuit + commit loop)
  stagehand_test.go     # EDITED — TestGenerateCommit_Timeout/"dryrun" assertion flipped to *RescueError
docs/cli.md             # EDITED — --dry-run row affirms dup-check/retry pipeline
# NO new files. NO new types/functions. go.mod/go.sum UNCHANGED. After S2: dry-run runs the full loop;
# S3 will add the net-new dry-run dup/parse/retry/snapshot tests.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 is DONE — treeSHA is always set): do NOT re-un-gate WriteTree or re-add
// signal.SetSnapshot. Both are already unconditional as of P1.M3.T1.S1. S2 starts BELOW the snapshot.
// The dry-run block you delete sits AFTER model resolution and BEFORE the loop. (design-decisions §0)

// CRITICAL (delete the WHOLE dry-run short-circuit, not parts of it): the `if dryRun { … }` block is
// the third, degraded loop copy. Remove it entirely; route dry-run through the existing loop below it.
// Every variable the loop needs (diff, recent, sysPrompt, resolved, model, treeSHA, parentSHA,
// candidate, retryInstr) is already in scope. Do NOT move declarations. (§1/§2)

// CRITICAL (insert the dry-run early-return in the RIGHT place): AFTER `if !success { return
// *RescueError{…} }` and BEFORE the `CommitTree` tail. If you put it before the `!success` check, a
// dry-run that exhausts retries will wrongly return CommitSHA:"" success instead of *RescueError.
// (§3)

// CRITICAL (the one test S2 MUST flip — or the tree is red): TestGenerateCommit_Timeout/"dryrun"
// currently asserts dry-run timeout is NOT *RescueError. After S2 it IS *RescueError{Kind:ErrTimeout}.
// Flip `errors.As(err,&re)` from "must be false" to "must be true" + assert re.TreeSHA != "" (mirror
// the "commit_path" subtest). S2 owns this single flip; S3 owns net-new tests. (§4)

// CRITICAL (disarm on dry-run success via ClearSnapshot, NOT RestoreDefault): S1 armed rescue for
// dry-run (SetSnapshot). On dry-run SUCCESS call signal.ClearSnapshot() (snapTree="" → handle() skips
// rescue). RestoreDefault() is for the update-ref window, which dry-run skips — do NOT call it.
// Dry-run timeout/exhaustion returns *RescueError and does NOT disarm (dangling tree intentional).
// (§5; internal/signal/signal.go handle(): `if tree != ""` guards the rescue print)

// GOTCHA (do NOT perturb generate.CommitStaged): D3 rejected adding a no-commit flag to the frozen
// orchestrator. The dedup happens inside runPipeline, which already held a second copy of the loop.
// generate.go is READ-ONLY for S2. (§7)

// GOTCHA (the loop body is byte-identical to the commit path — do not "improve" it): it is already a
// faithful, tested mirror of CommitStaged. The ONLY changes in runPipeline are: delete the dry-run
// short-circuit, add the dry-run early-return, and update stale comments. (§2)

// GOTCHA (retryInstr / model / recent are declared ABOVE the loop — reuse, don't redeclare):
// `resolved := deps.Manifest.Resolve()`, `model := …`, and `retryInstr := *resolved.RetryInstruction`
// are all in scope. `recent` is built once (nil on unborn → vacuous dup check). Do not shadow them.

// GOTCHA (no new import): generate.*, prompt.BuildUserPayload, provider.{Execute,ParseOutput},
// signal.{SetSnapshot,SetCandidate,ClearSnapshot,RestoreDefault}, context, errors, fmt, strings — ALL
// already imported and used by the existing commit-path loop. `go mod tidy` is a no-op; `git diff
// --exit-code go.mod go.sum` is empty.

// GOTCHA (the dry-run Result drops Changes): pkg/stagehand.Result has no Changes field (only the
// internal generate.Result does). The dry-run early-return returns the 5-field Result like the old
// single-pass did (CommitSHA, Subject, Message, Provider, Model). Do not invent a Changes field.
```

## Implementation Blueprint

### Data models and structure

```go
// NO new types. The existing types drive everything:
//   pkg/stagehand.Result  { CommitSHA, Subject, Message, Provider, Model string }
//   generate.RescueError  { Kind, TreeSHA, ParentSHA, Candidate string; Cause error }
//   generate.Deps, config.Config, provider.Manifest — unchanged.
// The dry-run success Result reuses the exact 5 fields the old single-pass returned.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT pkg/stagehand/stagehand.go — runPipeline: collapse the dry-run path into the shared loop
  - FILE: pkg/stagehand/stagehand.go, function runPipeline. NO new import.
  - DELETE the entire `// ---- DryRun: single pass, no commit. ----` block (the `if dryRun { … }`
      short-circuit: BuildUserPayload(diff,nil) → Render → Execute → ErrTimeout-bare / errors.New /
      ParseOutput → return Result{CommitSHA:""}). Remove it wholesale.
  - The loop immediately below it (currently the SystemExtra commit path) now serves BOTH paths. Update
      its leading comment from "Commit path (SystemExtra set): full generate→dedupe loop + commit" to
      something like "Generation+dedupe loop (FR29/FR32) — runs for both dry-run and commit paths".
      DO NOT change the loop BODY (it is tested as-is).
  - AFTER the existing `if !success { return &generate.RescueError{Kind: generate.ErrRescue, TreeSHA:
      treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause} }` block and BEFORE the
      `// Commit (mirror CommitStaged steps 7-8).` tail, INSERT:
        if dryRun {
            signal.ClearSnapshot() // disarm — no rescue on dry-run success (§18.4 belt-and-suspenders)
            return Result{
                CommitSHA: "",
                Subject:   generate.ExtractSubject(msg),
                Message:   msg,
                Provider:  deps.Manifest.Name,
                Model:     model,
            }, nil
        }
  - The existing CommitTree → RestoreDefault → UpdateRefCAS → CASError → ClearSnapshot → DiffTree →
      return tail is UNCHANGED and now runs only for !dryRun.
  - GOTCHA: place the early-return AFTER the !success check (else dry-run exhaustion wrongly succeeds).
  - GOTCHA: ClearSnapshot() (not RestoreDefault()) on dry-run success.
  - GOTCHA: do NOT touch WriteTree / signal.SetSnapshot (S1 already made them unconditional).

Task 2: EDIT pkg/stagehand/stagehand_test.go — flip the ONE locked-in timeout assertion
  - FILE: pkg/stagehand/stagehand_test.go, TestGenerateCommit_Timeout, subtest "dryrun".
  - REPLACE the block:
        var re *RescueError
        if errors.As(err, &re) {
            t.Error("DryRun timeout should return bare ErrTimeout, not *RescueError")
        }
    WITH (mirror the "commit_path" subtest):
        var re *RescueError
        if !errors.As(err, &re) {
            t.Fatalf("dryrun: error type = %T, want *RescueError", err)
        }
        if !errors.Is(err, ErrTimeout) {
            t.Errorf("dryrun: errors.Is(err, ErrTimeout) = false, error = %v", err)
        }
        if re.TreeSHA == "" {
            t.Error("dryrun: RescueError.TreeSHA empty, want non-empty (snapshot was taken — S1)")
        }
  - KEEP the existing `if !errors.Is(err, ErrTimeout)` guard at the top of the subtest (still true).
  - GOTCHA: do NOT add net-new tests (dup-retry/parse-retry/exhaustion/snapshot) — that is S3.

Task 3: EDIT docs/cli.md (Mode A) — affirm the --dry-run full pipeline
  - FILE: docs/cli.md, line ~26 (the `--dry-run` table row).
  - The row currently reads: "| `--dry-run` | bool | false | — | — | Generate and print the message;
      do not commit |". AFFIRM it runs the full duplicate-check/retry pipeline — e.g. expand the
      description to "Run the full generate→parse→duplicate-check pipeline (same as a real commit,
      including retry) and print the message; do not commit." (or add a one-line note beneath the
      flags table). Keep exit-0 semantics (line ~80) unchanged.
  - VERIFY docs/how-it-works.md: grep found no dry-run mention and the prompt/retry section (~line 102)
      documents the loop generically → NO edit unless a divergence claim is found.

Task 4: VERIFY (no further file change)
  - RUN the Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. generate.go,
      internal/signal/*, internal/generate/dedupe.go, internal/cmd/* MUST be byte-unchanged. S1's
      snapshot code in runPipeline MUST be byte-unchanged. `go test -race ./...` MUST be green.
```

### Implementation Patterns & Key Details

```go
// THE structural change in runPipeline (delete + insert; loop body & commit tail unchanged):

// ── BEFORE (three paths: dry-run short-circuit + commit loop) ──────────────────────────────
//	resolved := deps.Manifest.Resolve()
//	model := cfg.Model
//	if model == "" { model = *resolved.DefaultModel }
//
//	if dryRun {                                   // ← DELETE this entire block (the degraded 3rd copy)
//		… single attempt, bare ErrTimeout, errors.New on parse-fail, return Result{CommitSHA:""} …
//	}
//
//	retryInstr := *resolved.RetryInstruction
//	… loop (rejected, parseFail, IsDuplicate, MaxDuplicateRetries+1, *RescueError) …   // ← KEEP (body)
//	if !success { return &generate.RescueError{Kind: ErrRescue, TreeSHA: treeSHA, …} } // ← KEEP
//	// Commit tail: CommitTree → RestoreDefault → UpdateRefCAS → CASError → ClearSnapshot → DiffTree → return

// ── AFTER (one loop, two tails) ────────────────────────────────────────────────────────────
//	resolved := deps.Manifest.Resolve()
//	model := cfg.Model
//	if model == "" { model = *resolved.DefaultModel }
//
//	retryInstr := *resolved.RetryInstruction
//	… loop (UNCHANGED body — now runs for dryRun AND !dryRun) …
//	if !success { return &generate.RescueError{Kind: ErrRescue, TreeSHA: treeSHA, …} } // both paths
//
//	if dryRun {                                                                   // ← INSERT
//		signal.ClearSnapshot() // disarm — no rescue on dry-run success
//		return Result{CommitSHA: "", Subject: generate.ExtractSubject(msg),
//			Message: msg, Provider: deps.Manifest.Name, Model: model}, nil
//	}
//
//	// Commit tail (UNCHANGED — runs only for !dryRun):
//	// CommitTree → RestoreDefault → UpdateRefCAS → CASError → ClearSnapshot → DiffTree → return

// THE dry-run timeout path (inside the loop, UNCHANGED — now reached by dry-run too):
//	if errors.Is(execErr, context.DeadlineExceeded) {
//		return Result{}, &generate.RescueError{
//			Kind: generate.ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
//			Candidate: candidate, Cause: execErr,   // ← real TreeSHA (S1), not bare ErrTimeout
//		}
//	}

// THE test flip (TestGenerateCommit_Timeout/"dryrun") — mirror "commit_path":
//	var re *RescueError
//	if !errors.As(err, &re)            { t.Fatalf("dryrun: want *RescueError, got %T", err) }
//	if !errors.Is(err, ErrTimeout)     { t.Errorf("dryrun: not ErrTimeout: %v", err) }
//	if re.TreeSHA == ""                { t.Error("dryrun: TreeSHA empty, want non-empty") }
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. Every primitive the loop uses is already imported by stagehand.go (generate.*,
        prompt.BuildUserPayload, provider.{Execute,ParseOutput}, signal.*, context, errors, fmt,
        strings). `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` is empty.

PACKAGE EDGES (import graph):
  - pkg/stagehand → (internal: config, generate, git, prompt, provider, signal, ui; stdlib). UNCHANGED.
  - No new import edge. internal/generate, internal/signal, internal/provider, internal/prompt are
        READ-ONLY consumers here.

UPSTREAM (already in place — consume, do not build):
  - S1 (P1.M3.T1.S1): WriteTree + signal.SetSnapshot are UNCONDITIONAL → treeSHA always set, rescue
        armed for dry-run. S2 consumes treeSHA; it does NOT re-do the snapshot.
  - P1.M1.T3.S3 git.RecentSubjects(50): the `recent` slice, built once in runPipeline (nil on unborn →
        vacuous dup check). Already consumed by the loop.
  - config.Config.MaxDuplicateRetries (default 3 → 4 attempts): bounds the loop. Already consumed.

DOWNSTREAM (the contracts S2 preserves/changes):
  - CLI (internal/cmd/default_action.go): on dry-run SUCCESS prints the message + exit 0 (unchanged);
        on dry-run *RescueError, handleGenError prints the §18.3 rescue block (exit 3) — NOW possible
        in dry-run too, which is correct (the snapshot was taken). No CLI code change needed.
  - pkg/stagehand.GenerateCommit dispatch is UNCHANGED: `!DryRun && SystemExtra=="" → CommitStaged`;
        `DryRun || SystemExtra != "" → runPipeline`. S2 only changes runPipeline's internals.

FROZEN FILES (do NOT edit):
  - internal/generate/generate.go (CommitStaged — D3 rejected a no-commit flag), internal/signal/*,
        internal/generate/dedupe.go, internal/provider/*, internal/prompt/*, internal/git/*,
        internal/cmd/*, internal/config/*, go.mod, go.sum, Makefile, AND S1's snapshot lines in
        runPipeline (the WriteTree + signal.SetSnapshot block).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the edited files
gofmt -w pkg/stagehand/stagehand.go pkg/stagehand/stagehand_test.go

# Vet the package (compiles stagehand.go + test)
go vet ./pkg/stagehand/

# Confirm NO new import leaked and the dry-run short-circuit is gone
grep -n "DryRun: single pass" pkg/stagehand/stagehand.go   # → no match (block deleted)
grep -n "if dryRun {" pkg/stagehand/stagehand.go           # → exactly ONE match (the new early-return)

# Confirm go.mod/go.sum unchanged
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Confirm generate.go / signal.go untouched (frozen)
git diff --exit-code internal/generate/generate.go internal/signal/signal.go && echo "frozen files untouched ✓"

# Expected: Zero errors. The loop must appear once; the dry-run short-circuit must be gone.
```

### Level 2: Unit Tests (THE KEYSTONE — the flipped assertion + full suite)

```bash
# The edited timeout subtest specifically
go test -race -v ./pkg/stagehand/ -run 'TestGenerateCommit_Timeout'

# The whole stagehand package (incl. the happy-path DryRun, SystemExtra, MissingProvider tests)
go test -race -v ./pkg/stagehand/

# Full repo regression (S2 must not break any other package)
go test -race ./...

# Expected: ALL green. TestGenerateCommit_Timeout/"dryrun" now passes with the *RescueError assertion;
# TestGenerateCommit_Timeout/"commit_path" still passes (unchanged); TestGenerateCommit_DryRun still
# passes (happy-path stub succeeds first attempt → CommitSHA:"" still returned via the new early-return).
# If TestGenerateCommit_DryRun fails, the early-return is misplaced (likely before the !success check).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build + vet
go build ./... && go vet ./...

# Manual end-to-end proof of the bug fix (needs a built binary + a stub script):
#   repo history: "feat: init"; stub attempt 1 = "feat: init" (dup), attempt 2 = "feat: unique".
#   Before S2: dry-run printed "feat: init" (the dup). After S2: dry-run must print "feat: unique".
# (This is exactly the repro in PRD Issue 2's Steps to Reproduce. S3 will encode it as a unit test;
#  S2 can spot-check it manually with bin/stagehand + a STAGEHAND_STUB_OUT-scripted stub.)

# Expected: `go build ./...` clean; the manual repro now shows dry-run retrying past the duplicate.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for S2 beyond the above (pure in-repo refactor; no DB/network/UI/service).
# Strongest creative check: confirm the loop appears EXACTLY once in runPipeline and that the dry-run
# success path neither commits nor moves HEAD:
#   - grep -c "for attempt := 0; attempt <= cfg.MaxDuplicateRetries" pkg/stagehand/stagehand.go  → 1
#   - In a dry run, `git rev-parse HEAD` is unchanged AND `git count-objects -v` shows the dangling
#     tree (S1) but NO new commit object.
# (The net-new dup-retry/parse-retry/snapshot unit tests are S3 — S2 ships the behavior, S3 pins it.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` succeeds; `go vet ./...` clean; `gofmt -l pkg/... internal/...` empty.
- [ ] `go test -race ./...` green (the flipped `TestGenerateCommit_Timeout/"dryrun"` + every other test).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.
- [ ] Frozen files byte-unchanged: `internal/generate/generate.go`, `internal/signal/*`,
      `internal/provider/*`, `internal/prompt/*`, `internal/git/*`, `internal/cmd/*`, `internal/config/*`.

### Feature Validation

- [ ] `runPipeline` contains the generate→parse→dedupe loop **exactly once** (the dry-run
      short-circuit is deleted).
- [ ] Dry-run success returns `Result{CommitSHA:""}` via the new early-return (after `!success` check).
- [ ] Dry-run success calls `signal.ClearSnapshot()` and skips CommitTree/UpdateRefCAS/RestoreDefault/DiffTree.
- [ ] Dry-run timeout returns `*RescueError{Kind:ErrTimeout, TreeSHA: <non-empty>}` (flipped test passes).
- [ ] Dry-run exhaustion returns `*RescueError{Kind:ErrRescue, …}`; dry-run uses `parseFail`+`retryInstr`
      (FR29) and `rejected`+`IsDuplicate` (FR30–FR33), identical to the commit path.
- [ ] The commit path (`!dryRun`) is byte-for-byte unchanged (same loop body + commit tail).
- [ ] `docs/cli.md` `--dry-run` row affirms the full dup-check/retry pipeline.

### Code Quality Validation

- [ ] Follows the existing loop pattern verbatim (no "improvements" to the tested loop body).
- [ ] Three near-duplicate paths collapsed to one loop + two tails.
- [ ] Anti-patterns avoided (see below): no CommitStaged perturbation, no RestoreDefault in dry-run,
      no early-return before `!success`, no net-new tests (S3's job), no new import.
- [ ] Comments updated (the stale "Commit path (SystemExtra set)" loop comment now reflects both paths).

### Documentation & Deployment

- [ ] `docs/cli.md` `--dry-run` row updated (Mode A).
- [ ] `docs/how-it-works.md` verified (no divergence claim; no edit needed unless one is found).
- [ ] No new env vars, no new config fields, no new exit codes.

---

## Anti-Patterns to Avoid

- ❌ Don't add a "no-commit" flag to `generate.CommitStaged` — D3 explicitly rejected it (frozen,
  tested orchestrator). Dedup inside `runPipeline`, which already held the loop copy.
- ❌ Don't re-do S1's work — `WriteTree` + `signal.SetSnapshot` are already unconditional. S2 starts
  below the snapshot.
- ❌ Don't place the dry-run early-return BEFORE the `if !success` check — that would make a dry-run
  that exhausts retries wrongly return `CommitSHA:""` success instead of `*RescueError`.
- ❌ Don't call `signal.RestoreDefault()` on dry-run success — its job is the `update-ref` window, which
  dry-run skips. Use `ClearSnapshot()` to disarm.
- ❌ Don't leave the `TestGenerateCommit_Timeout/"dryrun"` assertion un-flipped — S2's behavior change
  makes the old assertion fail; S2 owns this single flip (S3 owns net-new tests).
- ❌ Don't "improve" the loop body — it is a faithful, tested mirror of `CommitStaged`. Change only the
  surrounding structure (delete short-circuit, add early-return, update comments).
- ❌ Don't add net-new dry-run tests (dup-retry/parse-retry/exhaustion/snapshot) — that is S3.
- ❌ Don't add a new import or change go.mod — all primitives are already imported.
- ❌ Don't touch the frozen files (`generate.go`, `signal.go`, `provider/*`, `prompt/*`, `git/*`).
