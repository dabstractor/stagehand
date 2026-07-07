---
name: "P1.M3.T2.S2 — pkg/stagecoach.runPipeline dry-run hook wiring (FR-V8a: commit-msg-only under --dry-run, + the SystemExtra-commit path's full sequence): thread S1's CommitHookRunner into runPipeline so --dry-run runs prepare-commit-msg + commit-msg on the would-be message (skipping pre/post-commit) and surfaces a commit-msg rejection as warn-and-print (NOT a rescue) — PRD §9.25 FR-V8a / §9.12 FR49"
description: |

  Land the SECOND subtask of the commit-hooks WIRING (P1.M3.T2): thread S1's `CommitHookRunner`
  (generate.Deps.Hooks → hooks.DefaultRunner, wired in buildDeps by S1) into `pkg/stagecoach.runPipeline`
  — the self-contained path for `opts.DryRun || opts.SystemExtra != ""`. S1 wired generate.CommitStaged
  (the common `!DryRun && SystemExtra==""` path); THIS task wires runPipeline (the dry-run + SystemExtra
  path), which S1 does NOT touch and which no other task owns (T3 is decompose). The runner
  (internal/hooks/runner.go, S1/S2 COMPLETE) ALREADY honors HookOpts.DryRun internally: it skips
  pre-commit + self-skips post-commit under DryRun, and runs prepare-commit-msg (always) + commit-msg
  (FR-V8a). So the caller only passes `dryRun` through — EXCEPT for the one caller-side semantic the
  runner cannot own: a commit-msg REJECTION under --dry-run is INFORMATION (warn-and-print), not a
  failure (the runner intentionally returns the same *RescueError shape under dry-run as on the commit
  path; only the caller knows it is a preview).

  TWO EDITS to runPipeline (pkg/stagecoach/stagecoach.go) + NEW tests. NO other file:
    1. INSERT A (RunCommitHooks) — AFTER generate.EditMessage, BEFORE the `if dryRun {…}` block. Shared
       by the dry-run and SystemExtra-commit paths; passes `dryRun` through. Under dryRun a commit-msg
       *RescueError is caught → warn-and-print (the would-be message + a stderr notice) and the run
       continues to the dry-run Result (exit 0); a NON-RescueError (infrastructure) propagates. Under
       !dryRun a hook error propagates as the rescue (FR-V7, exit 3, mirroring S1's CommitStaged). On
       success reassign `treeSHA, msg = ft, fm`.
    2. INSERT B (RunPostCommit) — in the COMMIT TAIL (CommitTree → … → UpdateRefCAS → ClearSnapshot),
       after UpdateRefCAS success, before signal.ClearSnapshot. Reached ONLY when !dryRun (dry-run
       returns above). Best-effort; return discarded (FR-V7). Mirrors S1's INSERT B.

  ⚠️ **#1 — THE headline design call: commit-msg rejection under --dry-run → WARN-AND-PRINT, not a
      rescue.** The runner returns `("", "", *generate.RescueError{Cause: "commit-msg: …"})` on a
      non-zero commit-msg EVEN under dry-run (it does not special-case the dry-run exit code; only
      pre-commit is dry-run-gated). On the commit path that *RescueError is the correct FR-V7 rescue
      (exit 3); on the DRY-RUN path it is WRONG — FR-V8a's intent is "the user still sees lint results"
      (a dry-run is a preview; a rejection is information). So the dry-run branch CATCHES the
      *RescueError (errors.As), prints the would-be message (RescueError.Candidate) + a stderr notice,
      and continues to exit 0 (FR49). A NON-RescueError (hooks-dir resolve / msg-file / read-back — all
      wrapped fmt.Errorf) is infrastructure → propagate. (design-decisions §2)

  ⚠️ **#2 — INSERT A is ONE shared insert (dryRun threaded), NOT a dry-run-only branch.** runPipeline is
      the commit path for `!DryRun && SystemExtra != ""` too — and FR-V1 says EVERY plumbing-path commit
      runs hooks. No other task wires runPipeline's commit tail (S1 wired CommitStaged; T3 is decompose).
      So INSERT A sits OUTSIDE `if dryRun` (shared): dryRun=true → skip pre-commit + commit-msg-lint;
      dryRun=false → full pre→prepare→commit-msg + INSERT B post-commit. (design-decisions §1/§5)

  ⚠️ **#3 — INSERT B is in the COMMIT TAIL (after UpdateRefCAS), reached ONLY when !dryRun.** The
      `if dryRun {…}` block returns early BEFORE the commit tail, so post-commit is naturally skipped
      under dry-run (FR-V8a) — no extra guard. RunPostCommit also self-guards DryRun. Return discarded
      (FR-V7). (design-decisions §3/§4)

  ⚠️ **#4 — deps.Hooks is wired by S1's buildDeps (DONE); runPipeline does NOT import internal/hooks.**
      runPipeline accesses hooks only via `deps.Hooks CommitHookRunner` (the generate-package interface).
      `errors`/`fmt`/`os`/`generate` are ALREADY imported in stagecoach.go — INSERT A/B add NO imports.
      If S1 has not landed, deps.Hooks is nil → both inserts no-op → byte-identical to today (safe to
      land before/after S1; the dry-run behavior tests require S1's buildDeps wiring via GenerateCommit).
      (design-decisions §7/§8)

  ⚠️ **#5 — stderr/stdout separation (FR51).** The would-be message → stdout (clean, via cmd's
      printDryRunMessage); the "⚠ rejected" notice + the hook's own stderr (runner verbatim passthrough)
      → stderr. The warning is ALWAYS printed (not Verbose-gated) — FR-V8a's whole point. (§6)

  ⚠️ **#6 — Coordination with S1: DISJOINT regions of stagecoach.go.** S1 edits buildDeps (~L325-386) +
      generate.go; THIS task edits runPipeline (~L411-700). Different functions, no overlap. Do NOT touch
      buildDeps/generate.go/runner.go/adapter.go/cmd — all S1/S2-owned or out of scope. (§7)

  Deliverable: MODIFIED pkg/stagecoach/stagecoach.go (runPipeline: INSERT A + INSERT B) + MODIFIED
  pkg/stagecoach/stagecoach_test.go (4 hook tests via GenerateCommit). OUTPUT: --dry-run runs commit-msg
  (and prepare-commit-msg) on the would-be message but skips pre/post-commit; a commit-msg rejection is
  warn-and-print (message + notice, exit 0); a non-zero pre-commit under dry-run is skipped (no abort);
  the SystemExtra-commit path runs the full hook sequence (mirror of S1). `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Thread S1's `CommitHookRunner` into `pkg/stagecoach.runPipeline` so the dry-run +
SystemExtra path honors PRD §9.25 FR-V8a: under `--dry-run`, `pre-commit` and `post-commit` are skipped
(nothing is committed), but `prepare-commit-msg` and `commit-msg` RUN on the would-be message so the user
sees lint results — and a `commit-msg` REJECTION under dry-run is surfaced as warn-and-print (the would-be
message + a stderr notice, exit 0), NOT as a rescue. The SystemExtra real-commit path (!dryRun) gets the
full pre→prepare→commit-msg sequence with a hook-abort mapped to the rescue (FR-V7) and post-commit fired
best-effort — mirroring S1's CommitStaged wiring, since runPipeline's commit tail is the ONLY commit path
for SystemExtra and no other task wires it.

**Deliverable**:
1. **MODIFIED `pkg/stagecoach/stagecoach.go`** — `runPipeline`: INSERT A (nil-guarded `RunCommitHooks` after
   `EditMessage`, before `if dryRun`, passing `dryRun`; dry-run commit-msg *RescueError → warn-and-print;
   !dryRun error → rescue; success → `treeSHA,msg = ft,fm`) + INSERT B (nil-guarded `RunPostCommit` in the
   commit tail after `UpdateRefCAS`, before `ClearSnapshot`; return discarded).
2. **MODIFIED `pkg/stagecoach/stagecoach_test.go`** — 4 tests via the exported `GenerateCommit`: dry-run
   commit-msg reject (warn-and-print + lint surfaced), dry-run pre-commit skipped, dry-run commit-msg
   accepts, and SystemExtra pre-commit abort → *RescueError.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `--dry-run` with a
rejecting `commit-msg` hook returns `err == nil` (exit 0) with `res.Message` non-empty (the would-be
message) + the hook's lint message on stderr; `--dry-run` with a rejecting `pre-commit` hook SUCCEEDS
(pre-commit skipped, no abort); `--dry-run` with an accepting `commit-msg` succeeds; a SystemExtra
real-commit with a rejecting `pre-commit` returns a `*generate.RescueError` with HEAD unchanged; the
existing `TestGenerateCommit_DryRun` (no hook installed → `deps.Hooks` runs commit-msg which is absent →
no-op) stays GREEN; go.mod/go.sum + buildDeps/runner.go/generate.go byte-unchanged.

## User Persona

**Target User**: A user who runs `stagecoach --dry-run` (FR49: preview the message without committing) in a
repo with a `commit-msg` hook (conventional-commit lint). Before this task, `--dry-run` showed the
generated message but NEVER ran `commit-msg` — so the preview could show a message the linter would
reject, giving a false "looks good" signal. After this task, `--dry-run` runs `commit-msg` (and
`prepare-commit-msg`) on the would-be message: if the linter accepts, the preview is the accepted message;
if it rejects, the preview still shows the message AND surfaces the lint result (warn-and-print), so the
user knows to fix the message before committing for real. Transitively: P1.M3.T3 (decompose, which reuses
the same runner) and P1.M4.T1 (the docs Mode B rewrite that will document the dry-run hook composition).

**Use Case**: A user with a conventional-commit `commit-msg` hook runs `stagecoach --dry-run`. The agent
generates "updated the files" (non-conventional). `commit-msg` rejects it (exit 1, stderr "subject must be
conventional"). The dry-run prints the would-be message ("updated the files") to stdout AND the lint
rejection to stderr — so the user sees BOTH what would be committed AND that it would fail the linter, and
re-runs with a better prompt or fixes the hook. Exit 0 (it's a preview).

**User Journey**: `stagecoach --dry-run` → `pkg/stagecoach.GenerateCommit(DryRun:true)` → `buildDeps` (S1
wires `Hooks: DefaultRunner{}`) → `runPipeline(dryRun=true)` → …WriteTree… → generate/dedupe loop →
EditMessage → **INSERT A: RunCommitHooks(dryRun=true)** [pre-commit skipped; prepare-commit-msg + commit-msg
run on the would-be message; a *RescueError (commit-msg reject) → warn-and-print] → `if dryRun` Result
(message to stdout; exit 0). The cmd layer's `printDryRunMessage` writes `res.Message` to stdout (clean);
the runner's verbatim hook-stderr + INSERT A's "⚠ rejected" notice go to stderr.

**Pain Points Addressed**: (1) `--dry-run` not running `commit-msg` — a preview that could silently pass a
message the linter would reject (the FR-V8a gap). (2) A naive "run commit-msg on dry-run" treating a
rejection as a hard failure — wrong for a PREVIEW (FR-V8a: "the user still sees lint results"; a rejection
is information). (3) The SystemExtra-commit path being hook-less — FR-V1 says every plumbing commit runs
hooks; runPipeline's commit tail was un-wired.

## Why

- **Closes FR-V8a (the dry-run hook composition) for the single-commit path.** S1 wired the real-commit
  path (CommitStaged); this wires the dry-run + SystemExtra path (runPipeline). Without it, `--dry-run`
  never runs `commit-msg`, so the preview can mislead.
- **Satisfies PRD §9.25 FR-V8a + §9.12 FR49.** FR-V8a(a): "--dry-run: nothing is committed, so pre-commit
  and post-commit are skipped; commit-msg runs against the would-be message so the user still sees lint
  results." FR49: dry-run prints the message, exit 0 (a commit-msg rejection is information, not a failure —
  exit stays 0, message + lint result both shown).
- **Wires the SystemExtra-commit path (a happy consequence of the shared insert).** Because INSERT A is
  outside `if dryRun`, the `!DryRun && SystemExtra != ""` real-commit path ALSO runs the full hook sequence
  (FR-V1) — closing the last single-commit hook gap S1 didn't cover (CommitStaged is bypassed when
  SystemExtra is set).
- **Faithful to the runner's frozen API.** The runner already honors `HookOpts.DryRun` (skips pre-commit,
  self-skips post-commit, runs prepare+commit-msg); this task only passes `dryRun` through + owns the one
  caller-side dry-run semantic (rejection = information).

## What

Two nil-guarded inserts in `runPipeline` (pkg/stagecoach/stagecoach.go) + four tests in stagecoach_test.go. No
imports, no buildDeps change (S1 did it), no runner/generate/cmd changes, no docs change (M3.T2.S2's how-
it-works subsection is owned by S1 per the item contract point 5).

### Success Criteria

- [ ] `runPipeline` has INSERT A: nil-guarded `deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA,
      parentSHA, msg, dryRun, deps.Verbose)` AFTER `generate.EditMessage` (and its `if err != nil` block)
      and BEFORE the `// ---- Dry-run success ----` / `if dryRun {…}` block.
- [ ] INSERT A error handling: on `herr != nil` AND `dryRun` AND `errors.As(herr, &*generate.RescueError)`
      → print the "⚠ commit hook rejected…" notice to stderr, set `msg = re.Candidate` (fall back to `msg`),
      fall through to the dry-run Result (exit 0); on `herr != nil` AND `dryRun` AND NOT a *RescueError →
      `return Result{}, herr` (infrastructure error propagates); on `herr != nil` AND `!dryRun` →
      `return Result{}, herr` (rescue, FR-V7); on success → `treeSHA, msg = ft, fm`.
- [ ] `runPipeline` has INSERT B: nil-guarded `deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, dryRun,
      deps.Verbose)` (return discarded) in the commit tail, after the `UpdateRefCAS` `if err != nil {…}`
      block, before `signal.ClearSnapshot()`.
- [ ] `stagecoach_test.go` adds: `TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage` (commit-msg
      exit 1 + stderr line; DryRun; err nil + Message non-empty + CommitSHA "" + HEAD unchanged + captured
      stderr contains the lint line); `..._DryRun_SkipsPreCommit` (pre-commit exit 1; DryRun; err nil +
      Message non-empty → pre-commit skipped); `..._DryRun_CommitMsgAccept` (commit-msg exit 0; DryRun;
      err nil + Message non-empty); `..._SystemExtra_PreCommitAbort_Rescue` (pre-commit exit 1;
      SystemExtra set, DryRun false; *RescueError + HEAD unchanged).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on the edited files.
- [ ] go.mod/go.sum byte-unchanged; buildDeps/imports in stagecoach.go byte-unchanged (S1 owns them);
      runner.go/generate.go/cmd/decompose byte-unchanged; the existing `TestGenerateCommit_DryRun` GREEN.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact insert points
(quoted verbatim in the Blueprint, anchored to runPipeline's EditMessage→`if dryRun`→commit-tail flow with
the surrounding code quoted), the headline warn-and-print design (the *RescueError discriminator + the
message-fallback, copy-ready), the runner's frozen DryRun semantics (it already skips pre-commit / runs
prepare+commit-msg / self-skips post-commit — the caller just passes `dryRun`), the buildDeps-is-S1's note
(deps.Hooks is already wired; no import here), and the 4 copy-ready tests (hook install + GenerateCommit +
stderr-capture). No decompose/prompt/provider knowledge required.

### Documentation & References

```yaml
# MUST READ — the design calls (the warn-and-print decision, the shared insert, stderr separation)
- docfile: plan/010_49117f1f30ab/P1M3T2S2/research/design-decisions.md
  why: §0 (scope: 2 inserts in runPipeline + tests; buildDeps is S1's), §1 (INSERT A placement — shared,
       dryRun threaded; why not dry-run-only), §2 (THE headline call: commit-msg reject under dry-run →
       warn-and-print via errors.As(*RescueError); non-RescueError propagates), §3 (INSERT B in commit
       tail, !dryRun-only), §4 (runner honors DryRun internally — just pass dryRun), §5 (Candidate ≈
       post-EditMessage msg under dry-run), §6 (stderr/stdout separation + test stderr-capture is safe:
       no t.Parallel), §7 (coordination: disjoint regions from S1), §8 (no new imports), §9 (test cases).
  critical: §2 (the errors.As discriminator — the load-bearing logic), §1 (shared insert — do NOT put
       RunCommitHooks inside `if dryRun`), §6 (capture os.Stderr; never t.Parallel) are the things most
       likely to go wrong.

# MUST READ — the parallel sibling CONTRACT (S1 wires the seam this task consumes)
- docfile: plan/010_49117f1f30ab/P1M3T2S1/PRP.md
  why: S1 defines generate.CommitHookRunner (the interface on Deps.Hooks) + hooks.DefaultRunner (adapter.go)
       + wires buildDeps (`Hooks: hooks.DefaultRunner{}`, stagecoach.go ~L386). THIS task CONSUMES
       deps.Hooks in runPipeline. S1's INSERT A/B in CommitStaged are the TEMPLATE this task mirrors in
       runPipeline (same nil-guard, same reassign treeSHA/msg, same discarded post-commit return). The
       runner's signatures: RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, dryRun, verbose) →
       (finalTree, finalMsg, err); RunPostCommit(ctx, g, cfg, dryRun, verbose) error.
  critical: deps.Hooks is ALREADY wired by S1's buildDeps — do NOT re-wire it or import internal/hooks in
       runPipeline. S1 edits buildDeps (~L325-386) + generate.go; this task edits runPipeline (~L411-700) —
       DISJOINT, no merge conflict.

# MUST READ — the runner's frozen DryRun semantics (this task CONSUMES, does not modify)
- file: internal/hooks/runner.go   (S1/S2 COMPLETE — read for the DryRun behavior; do NOT edit)
  section: `RunCommitHooks` — pre-commit: `if !(cfg.NoVerify || opts.DryRun)` (SKIPPED under DryRun, FR-V8a);
           prepare-commit-msg: ALWAYS runs (not gated); commit-msg: `if !cfg.NoVerify` (RUNS under DryRun,
           FR-V8a) → non-zero returns *RescueError{Cause:"commit-msg: …"}. On ANY error returns
           ("","",err). `RunPostCommit` — `if opts.DryRun { return nil }` (self-skips under DryRun).
           HookOpts{DryRun, Verbose}. The adapter hooks.DefaultRunner (adapter.go, S1) maps (dryRun,verbose)
           → HookOpts.
  why: confirms the runner ALREADY handles DryRun (skip pre-commit, run prepare+commit-msg, self-skip
       post-commit) — so the caller passes `dryRun` and does NOT re-implement gating. The ONE caller-side
       dry-run semantic the runner can't own: a commit-msg *RescueError under dry-run is INFORMATION
       (warn-and-print), because the runner intentionally returns the same *RescueError shape under dry-run
       as on the commit path.
  critical: do NOT edit runner.go. On a commit-msg rejection under dry-run the runner returns
       ("","",*RescueError{Candidate: finalMsg}) — Candidate ≈ the post-EditMessage msg (pre-commit is
       skipped under dry-run, so finalMsg is unchanged until the post-commit-msg read-back that never runs
       on rejection).

# THE FILE BEING MODIFIED — READ runPipeline fully before editing (the two insert points)
- file: pkg/stagecoach/stagecoach.go
  section: `func runPipeline(ctx, deps generate.Deps, cfg, systemExtra string, dryRun bool) (Result, error)`
           (~L415). The flow: … WriteTree → signal.SetSnapshot → generate/dedupe loop → FR-T1 multi-turn →
           the EditMessage block (`msg, err = generate.EditMessage(ctx, msg, cfg, generate.EditContext{…})`,
           ~L645 + its `if err != nil { return Result{}, err }`) → **INSERT A HERE** → the
           `// ---- Dry-run success: skip commit-tree/update-ref. ----` / `if dryRun { signal.ClearSnapshot();
           return Result{CommitSHA:"", Subject:…, Message: msg, …}, nil }` block (~L651) → the commit tail
           (`CommitTree` → `signal.RestoreDefault` → `UpdateRefCAS` (+ its `if err != nil {…CASError…}`) →
           **INSERT B HERE** → `signal.ClearSnapshot()` → `return Result{CommitSHA: newSHA, …}`).
  why: the EXACT code this task edits. Confirms `treeSHA`/`parentSHA`/`msg`/`cfg`/`model`/`ctx`/`deps` are
       all in scope at INSERT A (after the loop + EditMessage); `model` is declared ~L481; `dryRun` is the
       runPipeline param. Confirms the dry-run Result uses `msg` (so under hook-success `msg=fm` prints the
       annotated message; under rejection `msg=Candidate` prints the would-be message). Confirms the commit
       tail is !dryRun-only (the `if dryRun` block returns above it) → INSERT B is naturally dry-run-skipped.
  critical: INSERT A goes BETWEEN the EditMessage `if err != nil` block and the `// ---- Dry-run success`
       comment; INSERT B goes BETWEEN the UpdateRefCAS `if err != nil {…}` block's close and
       `signal.ClearSnapshot()`. Do NOT move signal.ClearSnapshot / RestoreDefault.

# THE TEST FILE BEING MODIFIED — mirror the existing dry-run test pattern
- file: pkg/stagecoach/stagecoach_test.go
  section: `setupTestRepo(t, stubtest.Options{Out: "feat: preview"})` (L123 — temp repo + repo-local
           .stagecoach.toml stub provider + initRepo + commitRaw("initial") + chdir); `TestGenerateCommit_DryRun`
           (L233 — `GenerateCommit(ctx, Options{Provider:"stub", DryRun:true})`; asserts Message/Subject +
           CommitSHA=="" + HEAD unchanged); `initRepo` (L25 — `git init` + identity); `writeFile`/`stageFile`/
           `headSHA`/`gitOut` helpers.
  why: the test STYLE — route through the EXPORTED GenerateCommit (which calls buildDeps → runPipeline),
       mirror setupTestRepo, install the hook in `repo/.git/hooks/<name>` (0755) AFTER setupTestRepo (CWD is
       the repo → `repoDir, _ := os.Getwd()`). The hook test captures os.Stderr (os.Pipe) to assert the lint
       result surfaced — SAFE because the suite has ZERO t.Parallel() (verified).
  critical: the stub message must be UNIQUE (not "initial") so the dedupe loop accepts it on attempt 0. The
       existing TestGenerateCommit_DryRun has NO hook installed → commit-msg absent → runner no-ops → stays
       GREEN (deps.Hooks runs RunCommitHooks, which finds no commit-msg hook → returns the msg unchanged).

# The chokepoint map (confirms runPipeline is THE dry-run hook site)
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "## 5. The commit chokepoints" — runPipeline (stagecoach.go:411) is the dry-run/SystemExtra path;
           CommitStaged is the common commit path (S1); decompose.publishCommit is T3. The note: "the
           implementing agent must wire commit-msg into runPipeline's dry-run branch (or confirm the
           cmd-layer dry-run handling covers it)" — CONFIRMED: cmd's printDryRunMessage (default_action.go:
           201,438) only writes res.Message to stdout; it runs NO hooks. So runPipeline is the sole site.
  critical: runPipeline is the ONLY place to wire dry-run hooks (cmd does not). §6 (RescueError → exit 3 via
       the existing CLI handleGenError — no new exit code); §7 (signal arms at WriteTree; hooks run inside
       the armed window — no signal change needed).

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/010_…/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" FR-V8(a) ("--dry-run: nothing is committed, so
           pre-commit and post-commit are skipped; commit-msg runs against the would-be message so the user
           still sees lint results"); FR-V1 (every plumbing-path commit runs hooks); FR-V7 (hook failure is
           a rescue on the commit path; post-commit best-effort). "9.12 Dry run" FR49 (print message, exit 0).
  critical: FR-V8a is the spec for THIS task. Note FR49 (exit 0) overrides the commit-path rescue semantics
       under dry-run: a commit-msg rejection is NOT a failure on the dry-run path → warn-and-print + exit 0.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/
  stagecoach.go        # EDIT — runPipeline: INSERT A (RunCommitHooks) + INSERT B (RunPostCommit). buildDeps UNCHANGED (S1 owns it).
  stagecoach_test.go   # EDIT — +4 hook tests (GenerateCommit → buildDeps → runPipeline). Existing tests UNCHANGED + GREEN.
internal/hooks/
  runner.go           # S1/S2 COMPLETE (RunCommitHooks/RunPostCommit/HookOpts; honors DryRun). UNCHANGED.
  adapter.go          # S1 NEW (DefaultRunner → CommitHookRunner). UNCHANGED.
internal/generate/
  generate.go         # S1 EDIT (CommitHookRunner interface + Deps.Hooks + CommitStaged inserts). UNCHANGED here.
internal/cmd/
  default_action.go   # printDryRunMessage(stdout, res.Message) — runs NO hooks (confirms runPipeline is the site). UNCHANGED.
go.mod / go.sum       # UNCHANGED (no new imports in stagecoach.go: errors/fmt/os/generate already imported).
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place: runPipeline (2 inserts) in pkg/stagecoach/stagecoach.go + 4 tests in stagecoach_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — commit-msg rejection under --dry-run → WARN-AND-PRINT, not a rescue): the runner returns
//   ("","",*generate.RescueError{Cause:"commit-msg: …"}) on a non-zero commit-msg EVEN under dry-run (only
//   pre-commit is dry-run-gated). On the commit path that's the FR-V7 rescue (exit 3); on DRY-RUN it's
//   WRONG (FR-V8a: "user still sees lint results" — a rejection is information). So CATCH it under dryRun:
//   errors.As(herr, &*generate.RescueError) → print "⚠ rejected" to stderr + msg=Candidate + fall through
//   to the dry-run Result (exit 0). A NON-RescueError (hooks-dir/msg-file/read-back — wrapped fmt.Errorf)
//   is infrastructure → propagate. (design-decisions §2)

// CRITICAL (#2 — INSERT A is ONE shared insert OUTSIDE `if dryRun`, NOT a dry-run-only branch): runPipeline
//   is the commit path for `!DryRun && SystemExtra != ""` too, and FR-V1 says every plumbing commit runs
//   hooks. No other task wires runPipeline's commit tail. So INSERT A sits between EditMessage and `if dryRun`
//   (shared), passing `dryRun`. Putting it INSIDE `if dryRun` would leave the SystemExtra-commit path hook-less.
//   (design-decisions §1/§5)

// CRITICAL (#3 — INSERT B in the commit tail, !dryRun-only): the `if dryRun {…}` block returns BEFORE the
//   commit tail, so post-commit is naturally skipped under dry-run (FR-V8a). INSERT B goes after UpdateRefCAS
//   success, before signal.ClearSnapshot. Return discarded (FR-V7). (design-decisions §3)

// CRITICAL (#4 — deps.Hooks is wired by S1's buildDeps; do NOT import internal/hooks or re-wire here):
//   runPipeline accesses hooks only via deps.Hooks (the generate.CommitHookRunner interface). errors/fmt/os/
//   generate are ALREADY imported in stagecoach.go — INSERT A/B add NO imports. If S1 hasn't landed, deps.Hooks
//   is nil → both inserts no-op → byte-identical to today. (design-decisions §7/§8)

// GOTCHA (the runner honors DryRun INTERNALLY — do NOT re-gate in the caller): HookOpts.DryRun skips
//   pre-commit + self-skips post-commit + runs prepare/commit-msg. Pass `dryRun` through; the ONLY caller-
//   side dry-run logic is the §1 warn-and-print for a *RescueError. (design-decisions §4)

// GOTCHA (stderr/stdout separation, FR51): res.Message → stdout (clean, via cmd printDryRunMessage); the
//   "⚠ rejected" notice + the hook's own stderr (runner verbatim passthrough via cmd.Stderr=os.Stderr) →
//   stderr. The warning is ALWAYS printed (not Verbose-gated) — FR-V8a's whole point. (design-decisions §6)

// GOTCHA (the test stderr-capture is safe — no t.Parallel): the stagecoach_test suite has ZERO t.Parallel()
//   (verified), so a global os.Stderr swap (os.Pipe) during the GenerateCommit call does not race. Restore
//   os.Stderr in t.Cleanup. Never add t.Parallel to these tests. (design-decisions §6/§9)

// GOTCHA (the would-be message under rejection is RescueError.Candidate ≈ the post-EditMessage msg): under
//   dry-run, pre-commit is skipped so finalMsg is unchanged until the post-commit-msg read-back (which never
//   runs on rejection) → Candidate == the passed-in msg. Use `wouldBe := re.Candidate; if wouldBe == "" {
//   wouldBe = msg }`. The SUCCESS path uses fm (read-back, prepare-annotated, comment-stripped). (§5)

// GOTCHA (coordination with S1 — DISJOINT regions): S1 edits buildDeps (~L325-386) + generate.go; this task
//   edits runPipeline (~L411-700). Different functions, no overlap. Do NOT touch buildDeps/generate.go/
//   runner.go/adapter.go/cmd. (design-decisions §7)

// GOTCHA (the existing TestGenerateCommit_DryRun stays GREEN): it has NO hook installed → commit-msg absent
//   → the runner's hookExecutable returns false → runCommitMsg no-ops → RunCommitHooks returns the msg
//   unchanged → the dry-run Result is byte-identical to today. deps.Hooks (S1's DefaultRunner) runs but finds
//   nothing to run. (design-decisions §9)
```

## Implementation Blueprint

### Data models and structure

```go
// pkg/stagecoach/stagecoach.go — NO new types, NO new imports. Two nil-guarded inserts in runPipeline.
// (generate.CommitHookRunner + Deps.Hooks are defined by S1 in generate.go; hooks.DefaultRunner by S1's
//  adapter.go; buildDeps wires Hooks by S1. This task only USES deps.Hooks in runPipeline.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT pkg/stagecoach/stagecoach.go — INSERT A (RunCommitHooks) in runPipeline
  - INSERT (nil-guarded) BETWEEN the EditMessage `if err != nil { return Result{}, err }` block and the
    `// ---- Dry-run success: skip commit-tree/update-ref. ----` comment:
      `ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)`
      then the 4-branch handler (Blueprint below): dryRun+*RescueError → warn-and-print (stderr notice +
      msg=Candidate, fall through); dryRun+non-RescueError → `return Result{}, herr`; !dryRun →
      `return Result{}, herr`; success → `treeSHA, msg = ft, fm`.
  - GOTCHA: shared insert (dryRun threaded) — do NOT place inside `if dryRun`. The warn-and-print FALLS
      THROUGH to the existing `if dryRun` Result (which prints msg) — do not duplicate the Result construction.
  - GOTCHA: the discriminator is errors.As(herr, &*generate.RescueError) — only hook rejections warn-and-print.

Task 2: EDIT pkg/stagecoach/stagecoach.go — INSERT B (RunPostCommit) in runPipeline's commit tail
  - INSERT (nil-guarded) BETWEEN the UpdateRefCAS `if err != nil {…}` block's close and `signal.ClearSnapshot()`:
      `_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, dryRun, deps.Verbose)` (+ best-effort comment).
  - GOTCHA: reached ONLY when !dryRun (the `if dryRun` block returns above). Return discarded (FR-V7).

Task 3: EDIT pkg/stagecoach/stagecoach_test.go — +4 hook tests (Blueprint below)
  - ADD a stderr-capture helper (os.Pipe + t.Cleanup restore) — used by the reject test.
  - ADD TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage (commit-msg exit 1 + stderr "reject: …";
    DryRun:true; assert err nil + Message non-empty + CommitSHA "" + HEAD unchanged + stderr contains the
    lint line).
  - ADD TestGenerateCommit_DryRun_SkipsPreCommit (pre-commit exit 1; DryRun:true; assert err nil + Message
    non-empty → pre-commit skipped under dry-run).
  - ADD TestGenerateCommit_DryRun_CommitMsgAccept (commit-msg exit 0; DryRun:true; assert err nil + Message
    non-empty → commit-msg ran + accepted under dry-run).
  - ADD TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue (pre-commit exit 1; SystemExtra:"…", DryRun:false;
    assert *generate.RescueError + HEAD unchanged → the shared insert's !dryRun branch = FR-V7).
  - GOTCHA: mirror setupTestRepo (stub provider) + install hooks in repo/.git/hooks/<name> (0755) after
    setupTestRepo (repoDir = os.Getwd()). UNIQUE stub message (not "initial"). No t.Parallel.

Task 4: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. buildDeps/runner.go/generate.go/
    cmd byte-unchanged. The existing TestGenerateCommit_DryRun GREEN (no hook → runner no-ops). `go build/vet/
    test ./...` green.
```

### Implementation Patterns & Key Details

```go
// === INSERT A — between EditMessage and the `if dryRun` block (shared; dryRun threaded) ===
	// ---- Commit hooks (PRD §9.25 FR-V1/V2/V8a). Threaded between EditMessage and the commit/dry-run split.
	// Under --dry-run the runner skips pre-commit + self-skips post-commit, but runs prepare-commit-msg +
	// commit-msg on the would-be message (FR-V8a). A commit-msg REJECTION under --dry-run is warn-and-print
	// (the would-be message + a stderr notice, exit 0) — a dry-run is a preview; a lint rejection is
	// information, not a failure. On the commit path (!dryRun) a hook abort is the FR-V7 rescue (exit 3). ===
	if deps.Hooks != nil {
		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)
		if herr != nil {
			if dryRun {
				var re *generate.RescueError
				if errors.As(herr, &re) {
					// FR-V8a: hook rejection under --dry-run → warn-and-print. Keep the would-be message so the
					// dry-run Result (below) carries it; the runner returned "" on error.
					fmt.Fprintf(os.Stderr, "⚠ commit hook rejected the would-be message under --dry-run: %v\n", re.Cause)
					wouldBe := re.Candidate
					if wouldBe == "" {
						wouldBe = msg
					}
					msg = wouldBe // fall through to the `if dryRun` Result (prints msg; exit 0)
				} else {
					return Result{}, herr // infrastructure error (hooks dir / msg file / read-back) → propagate
				}
			} else {
				return Result{}, herr // !dryRun → rescue (FR-V7, exit 3) — mirrors S1's CommitStaged
			}
		} else {
			treeSHA, msg = ft, fm // hook accepted (possibly re-treed + prepare-annotated) → downstream uses these
		}
	}

// === INSERT B — in the commit tail, after UpdateRefCAS success, before ClearSnapshot (!dryRun-only) ===
	if deps.Hooks != nil {
		_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, dryRun, deps.Verbose) // best-effort; exit disregarded (FR-V7)
	}
	signal.ClearSnapshot() // belt-and-suspenders disarm on success
```

```go
// === pkg/stagecoach/stagecoach_test.go — the stderr-capture helper + the 4 hook tests ===

// captureStderr runs fn with os.Stderr swapped to a pipe and returns whatever was written. SAFE only when
// the test is NOT t.Parallel() (the stagecoach_test suite has none). Restores os.Stderr in t.Cleanup.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	fn()
	w.Close()
	return <-done
}

// installHook writes an executable hook script into repo/.git/hooks/<name> (the runner discovers hooks via
// git rev-parse --git-path hooks = .git/hooks for a normal init). mode 0755 ⇒ owner-exec bit set (hookExecutable).
func installHook(t *testing.T, repo, name, body string) {
	t.Helper()
	dir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage — FR-V8a: under --dry-run a commit-msg that
// rejects (exit 1 + a stderr lint line) is warn-and-print: the would-be message is printed (stdout) AND the
// lint result is surfaced (stderr); exit 0 (err nil); nothing committed.
func TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: would-be preview"}) // UNIQUE message (not "initial")
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")
	beforeSHA := headSHA(t, repoDir)

	// commit-msg hook: rejects (exit 1) + echoes a distinctive lint line to stderr.
	installHook(t, repoDir, "commit-msg", "#!/bin/sh\necho 'reject: subject not conventional' >&2\nexit 1\n")

	var res Result
	var err error
	stderr := captureStderr(t, func() {
		res, err = GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun + rejecting commit-msg: err=%v, want nil (warn-and-print, exit 0)", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message printed (FR-V8a)")
	}
	if !strings.Contains(res.Message, "would-be preview") {
		t.Errorf("Message=%q, want it to carry the would-be message", res.Message)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA=%q, want empty (DryRun must not commit)", res.CommitSHA)
	}
	if after := headSHA(t, repoDir); after != beforeSHA {
		t.Errorf("HEAD moved %q→%q (DryRun must not move HEAD)", beforeSHA, after)
	}
	if !strings.Contains(stderr, "reject: subject not conventional") {
		t.Errorf("stderr missing the commit-msg lint result (FR-V8a):\n%s", stderr)
	}
	if !strings.Contains(stderr, "rejected the would-be message") {
		t.Errorf("stderr missing the dry-run rejection notice:\n%s", stderr)
	}
}

// TestGenerateCommit_DryRun_SkipsPreCommit — FR-V8a: pre-commit is SKIPPED under --dry-run. A pre-commit
// that would abort (exit 1) if run must NOT abort the dry-run (it is skipped). The proof is the absence of an
// abort: had pre-commit run, exit 1 → *RescueError → err != nil.
func TestGenerateCommit_DryRun_SkipsPreCommit(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: skip precommit preview"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	installHook(t, repoDir, "pre-commit", "#!/bin/sh\nexit 1\n")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("DryRun + pre-commit exit 1: err=%v, want nil (pre-commit skipped under --dry-run)", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message (pre-commit skipped ⇒ no abort)")
	}
}

// TestGenerateCommit_DryRun_CommitMsgAccept — FR-V8a: commit-msg RUNS under --dry-run and an accepting
// (exit 0) hook lets the dry-run succeed (the message is printed).
func TestGenerateCommit_DryRun_CommitMsgAccept(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: accepted preview"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	installHook(t, repoDir, "commit-msg", "#!/bin/sh\nexit 0\n")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("DryRun + accepting commit-msg: err=%v, want nil", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message (commit-msg accepted under --dry-run)")
	}
}

// TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue — the shared INSERT A's !dryRun branch: a SystemExtra
// real-commit (forces runPipeline's commit tail) with a rejecting pre-commit returns a *RescueError (FR-V7)
// and leaves HEAD unchanged (mirrors S1's CommitStaged).
func TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: systemextra rescue"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")
	beforeSHA := headSHA(t, repoDir)

	installHook(t, repoDir, "pre-commit", "#!/bin/sh\necho 'precommit: blocked' >&2\nexit 1\n")

	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", SystemExtra: "extra instructions"})
	if err == nil {
		t.Fatal("SystemExtra + pre-commit exit 1: err=nil, want a rescue error (FR-V7)")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("expected *generate.RescueError (FR-V7), got %T: %v", err, err)
	}
	if after := headSHA(t, repoDir); after != beforeSHA {
		t.Errorf("HEAD moved %q→%q on pre-commit abort (FR-V7 idempotent)", beforeSHA, after)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. stagecoach.go already imports errors/fmt/os/generate; INSERT A/B add
      NO imports (errors.As, fmt.Fprintf, os.Stderr, *generate.RescueError all already available).
      stagecoach_test.go adds io/filepath (already imported for os/io + filepath elsewhere — verify; if not,
      add). `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. runPipeline accesses hooks only via deps.Hooks (generate.CommitHookRunner). It does
      NOT import internal/hooks (S1's buildDeps does, and the adapter DefaultRunner satisfies the interface
      structurally). pkg/stagecoach → generate (EXISTING) → CommitHookRunner; no new edge.

UPSTREAM CONTRACT (consume, do NOT edit):
  - S1's generate.CommitHookRunner (generate.go) + Deps.Hooks (nil-safe) + hooks.DefaultRunner (adapter.go):
        RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, dryRun, verbose) → (finalTree, finalMsg, err);
        RunPostCommit(ctx, g, cfg, dryRun, verbose) error. S1 wires buildDeps (Hooks: DefaultRunner{}).
  - S1/S2's runner.go: honors HookOpts.DryRun (skip pre-commit + self-skip post-commit; run prepare+commit-msg);
        a commit-msg non-zero under dry-run returns ("","",*RescueError{Candidate: finalMsg, Cause:"commit-msg:…"}).
  - config: cfg.NoVerify (FR-V5), cfg.HookTimeout (FR-V6) — from P1.M1.T1.S1 (COMPLETE).

DOWNSTREAM CONTRACTS (NOT this task):
  - P1.M3.T3 (decompose.publishCommit): wires DefaultRunner into the decompose Deps; reuses the same runner.
  - P1.M4.T1 (docs Mode B): documents the dry-run hook composition (the how-it-works.md subsection is S1's per
        the item contract point 5; this task adds NO docs).

FROZEN/LEAVE (do NOT edit):
  - internal/hooks/runner.go (+_test.go), adapter.go, subset.go (+_test.go) — S1/S2 + P1.M2.T2.S1.
  - internal/generate/generate.go (+hooks_freeze_test.go) — S1. internal/git/*, internal/cmd/* (printDryRunMessage
        runs NO hooks — confirmed), internal/decompose/* (T3), internal/hook/*, internal/signal/*, internal/lock/*.
  - pkg/stagecoach/stagecoach.go's buildDeps + import block (S1's territory).
  - PRD.md, go.mod, Makefile. The existing TestGenerateCommit_DryRun (no hook → runner no-ops → GREEN).

NO NEW DATABASE / ROUTES / CLI COMMANDS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go
go vet ./pkg/stagecoach/
go build ./...
# Confirm the two inserts present + NO new import in stagecoach.go (errors/fmt/os/generate already there):
grep -n 'deps.Hooks.RunCommitHooks\|deps.Hooks.RunPostCommit\|rejected the would-be message' pkg/stagecoach/stagecoach.go
grep -n 'func captureStderr\|func installHook\|TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage' pkg/stagecoach/stagecoach_test.go
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm buildDeps + the import block are byte-unchanged (S1's territory; this task only edits runPipeline + tests):
git diff pkg/stagecoach/stagecoach.go | grep -E '^\+' | grep -i 'import\|buildDeps\|DefaultRunner' && echo "WARN: buildDeps/import touched (S1 owns it — re-check)" || echo "buildDeps/import UNCHANGED (expected)"
# Expected: go vet clean; build clean; both inserts + the warn-and-print notice present; go.mod/go.sum + buildDeps byte-unchanged.
```

### Level 2: The 4 hook tests + the existing dry-run test + the repo suite

```bash
go test ./pkg/stagecoach/ -v -run 'TestGenerateCommit_DryRun_CommitMsgReject|TestGenerateCommit_DryRun_SkipsPreCommit|TestGenerateCommit_DryRun_CommitMsgAccept|TestGenerateCommit_SystemExtra_PreCommitAbort'
# Expected PASS — verify:
#   ...DryRun_CommitMsgReject_PrintsMessage ... err nil + Message carries "would-be preview" + CommitSHA "" + HEAD unchanged + stderr has the lint line + the notice (FR-V8a warn-and-print ✓)
#   ...DryRun_SkipsPreCommit .................. err nil + Message non-empty (pre-commit exit 1 did NOT abort ⇒ skipped under dry-run ✓)
#   ...DryRun_CommitMsgAccept ................. err nil + Message non-empty (commit-msg ran + accepted under dry-run ✓)
#   ...SystemExtra_PreCommitAbort_Rescue ...... *generate.RescueError + HEAD unchanged (shared insert's !dryRun branch = FR-V7 ✓)
# The existing dry-run test (no hook installed → runner no-ops) MUST stay GREEN:
go test ./pkg/stagecoach/ -v -run 'TestGenerateCommit_DryRun$|TestGenerateCommit_DryRun_DedupeRetry|TestGenerateCommit_Success'
go test ./pkg/stagecoach/...   # the full stagecoach suite
# If CommitMsgReject_PrintsMessage fails with "err != nil", the warn-and-print branch isn't catching the
# *RescueError (check errors.As + the dryRun guard + that INSERT A is BEFORE `if dryRun`).
# If SkipsPreCommit fails (err != nil), pre-commit ran under dry-run (the runner's DryRun skip is broken OR
# deps.Hooks isn't wired — confirm S1's buildDeps landed).
# If the existing TestGenerateCommit_DryRun fails, INSERT A broke the no-hook path (commit-msg absent must no-op).
```

### Level 3: Whole-repo + frozen-file check

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS (stagecoach + generate + hooks + repo-wide). S1/S2's runner + freeze tests green.
# Confirm ONLY the two target files changed:
git diff --name-only | grep -E 'pkg/stagecoach/stagecoach\.go|pkg/stagecoach/stagecoach_test\.go' && echo "(expected files)"
# Confirm the frozen files are byte-unchanged:
git diff --exit-code internal/hooks internal/generate/generate.go internal/generate/hooks_freeze_test.go \
  internal/git internal/cmd internal/decompose internal/hook internal/signal internal/lock \
  PRD.md go.mod Makefile && echo "frozen files UNCHANGED (expected)"
# Confirm runPipeline's buildDeps + import block are byte-unchanged (S1 owns them):
git diff pkg/stagecoach/stagecoach.go | grep -E '^[+-].*(import|hooks\.DefaultRunner|Hooks:)' && echo "WARN: buildDeps/import changed" || echo "buildDeps/import UNCHANGED (expected — S1 owns)"
# Expected: only stagecoach.go + stagecoach_test.go modified; everything else byte-unchanged.
```

### Level 4: FR-V8a + the warn-and-print semantic correctness reasoning

```bash
# Verify by reasoning + the tests:
#   1. FR-V8a skip: under --dry-run the runner skips pre-commit (HookOpts.DryRun) and self-skips post-commit;
#      INSERT B is unreachable (the `if dryRun` block returns before the commit tail). (SkipsPreCommit.)
#   2. FR-V8a run: prepare-commit-msg + commit-msg run on the would-be message; an accepting commit-msg lets
#      the dry-run succeed; a rejecting commit-msg is caught. (CommitMsgAccept + CommitMsgReject.)
#   3. Warn-and-print (THE headline call): a *RescueError under dry-run → "⚠ rejected" on stderr + msg=Candidate
#      + fall through to the dry-run Result (exit 0, err nil). A non-RescueError propagates. The hook's OWN
#      stderr passes through verbatim (runner cmd.Stderr=os.Stderr) → the lint result is surfaced on stderr.
#      (CommitMsgReject_PrintsMessage asserts both the lint line + the notice on captured stderr.)
#   4. Shared insert (!dryRun): a SystemExtra real-commit runs the full sequence; a pre-commit abort →
#      *RescueError + HEAD unchanged (FR-V7, mirrors S1). (SystemExtra_PreCommitAbort_Rescue.)
#   5. No-hook no-op: the existing TestGenerateCommit_DryRun has no commit-msg → hookExecutable false →
#      runCommitMsg no-ops → RunCommitHooks returns the msg unchanged → byte-identical dry-run Result. (GREEN.)
#   6. stderr/stdout separation: res.Message → stdout (cmd printDryRunMessage); notice + hook stderr → stderr.
#      (CommitMsgReject captures os.Stderr; no t.Parallel in the suite → safe.)
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (the 4 new hook tests + the existing dry-run/success tests + S1/S2's runner/freeze
      tests + repo-wide).
- [ ] go.mod/go.sum byte-unchanged; stagecoach.go's buildDeps + import block byte-unchanged (S1 owns them).

### Feature Validation
- [ ] INSERT A (nil-guarded RunCommitHooks) is AFTER EditMessage, BEFORE `if dryRun`; passes `dryRun`.
- [ ] INSERT A error handling: dryRun+*RescueError → warn-and-print (stderr notice + msg=Candidate, exit 0);
      dryRun+non-RescueError → propagate; !dryRun → rescue (FR-V7); success → `treeSHA,msg = ft,fm`.
- [ ] INSERT B (nil-guarded RunPostCommit) is in the commit tail after UpdateRefCAS, before ClearSnapshot;
      return discarded.
- [ ] `--dry-run` + rejecting commit-msg → err nil + Message non-empty + lint result on stderr (FR-V8a).
- [ ] `--dry-run` + rejecting pre-commit → err nil (pre-commit skipped, FR-V8a).
- [ ] `--dry-run` + accepting commit-msg → err nil (commit-msg ran, FR-V8a).
- [ ] SystemExtra real-commit + rejecting pre-commit → `*RescueError` + HEAD unchanged (FR-V7).
- [ ] The existing TestGenerateCommit_DryRun (no hook) stays GREEN.

### Code Quality Validation
- [ ] The warn-and-print discriminator is `errors.As(herr, &*generate.RescueError)` (only hook rejections
      warn-and-print; infrastructure errors propagate).
- [ ] INSERT A is shared (outside `if dryRun`) — the SystemExtra-commit path is not left hook-less.
- [ ] runPipeline adds NO import (errors/fmt/os/generate already imported); does NOT import internal/hooks.
- [ ] Anti-patterns avoided (see below); buildDeps/runner.go/generate.go/cmd byte-unchanged.

### Documentation
- [ ] INSERT A's comment cites PRD §9.25 FR-V1/V2/V8a + the warn-and-print rationale (dry-run is a preview;
      a commit-msg rejection is information, exit 0). The test comments cite FR-V8a. (No docs/how-it-works.md
      change — that subsection is S1's per the item contract point 5.)

---

## Anti-Patterns to Avoid

- ❌ **Don't propagate a commit-msg *RescueError under --dry-run as a rescue.** The runner returns the same
  *RescueError shape under dry-run as on the commit path; only the CALLER knows it is a preview. Catch it
  (errors.As) under dryRun → warn-and-print (msg + stderr notice, exit 0). FR-V8a: "the user still sees lint
  results" — a rejection is information, not a failure. (§2)
- ❌ **Don't put RunCommitHooks INSIDE the `if dryRun` branch.** runPipeline is also the commit path for
  SystemExtra; FR-V1 requires hooks on every plumbing commit, and no other task wires runPipeline's commit
  tail. INSERT A is shared (outside `if dryRun`), with `dryRun` threaded. (§1)
- ❌ **Don't warn-and-print a NON-RescueError under dry-run.** A hooks-dir resolution / message-file / read-
  back error (wrapped fmt.Errorf) is infrastructure, not a lint result → propagate it. Only `*RescueError`
  (hook non-zero/timeout) is warn-and-print. (§2)
- ❌ **Don't re-implement dry-run gating in the caller.** The runner honors HookOpts.DryRun (skip pre-commit,
  run prepare+commit-msg, self-skip post-commit). Pass `dryRun` through; the ONLY caller-side dry-run logic
  is the warn-and-print for a *RescueError. (§4)
- ❌ **Don't add post-commit under dry-run.** INSERT B is in the commit tail, which the `if dryRun` block
  returns before — so post-commit is naturally skipped under dry-run (FR-V8a). Don't add a dry-run post-commit
  call. (§3)
- ❌ **Don't import internal/hooks or re-wire buildDeps in runPipeline.** deps.Hooks is wired by S1's
  buildDeps; runPipeline accesses hooks only via the generate.CommitHookRunner interface (errors/fmt/os/
  generate already imported). (§7/§8)
- ❌ **Don't write the warning to stdout.** stdout must stay clean (the message only — FR51; cmd's
  printDryRunMessage writes res.Message to stdout). The notice + the hook's stderr go to STDERR (fmt.Fprintf
  + the runner's verbatim passthrough). (§6)
- ❌ **Don't add t.Parallel() to the stderr-capture test.** A global os.Stderr swap races under parallel
  execution. The suite has none today; keep it that way for the capture test. (§6/§9)
- ❌ **Don't touch buildDeps/generate.go/runner.go/adapter.go/cmd.** buildDeps+import is S1's; generate.go +
  the freeze test are S1's; runner.go/adapter.go are S1/S2's; cmd's printDryRunMessage runs NO hooks
  (confirmed) and is out of scope. (§7)
