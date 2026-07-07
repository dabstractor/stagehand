---
name: "P1.M3.T2.S1 — generate.CommitStaged wiring + post-commit + staged-during-generation sentinel invariant: thread S1/S2's commit-hooks runner into the single-commit plumbing path (FR-V1/V2/V3/V7) — PRD §9.25 / §13.5 / §20.2"
description: |

  Land the FIRST subtask of the commit-hooks WIRING (P1.M3.T2): thread S1/S2's `internal/hooks` runner
  (RunCommitHooks/RunPostCommit — COMPLETE/in-progress) into `generate.CommitStaged` so every single-commit
  plumbing-path commit runs the repo's pre→prepare→commit-msg hooks scoped to the frozen snapshot (FR-V3),
  with a hook-abort mapped to the existing rescue (FR-V7) and post-commit fired best-effort. This is the
  single-commit non-dry-run chokepoint (CommitStaged); the dry-run path is runPipeline (S2, P1.M3.T2.S2);
  the decompose path is publishCommit (T3). The runner is COMPLETE (S1) + being refined (S2, parallel —
  recursion-prevention + message lifecycle); this task CONSUMES its frozen API.

  THE LOAD-BEARING CONSTRAINT — the generate↔hooks IMPORT CYCLE. `internal/hooks/runner.go` imports
  `internal/generate` (for `generate.RescueError` — `rescueErr` builds `*generate.RescueError{…}`). Therefore
  `generate.go` CANNOT `import "internal/hooks"` (Go rejects the cycle). RESOLUTION: inject the hook runner
  into `generate.Deps` as an INTERFACE (`CommitHookRunner`) defined in the generate package, whose methods
  inline `dryRun bool, verbose *ui.Verbose` (NOT `hooks.HookOpts` — that would re-introduce the cycle). The
  CLI (`pkg/stagecoach.buildDeps`) wires a concrete adapter (`hooks.DefaultRunner`) that delegates to
  hooks.RunCommitHooks/RunPostCommit. This mirrors how `Manifest` is injected. The item explicitly anticipates
  this ("inject it, mirroring how Manifest is injected, for testability"). (research §0/§1.)

  FIVE EDITS:
    1. internal/generate/generate.go — ADD `CommitHookRunner` interface (inlined dryRun+verbose) + `Deps.Hooks`
       field (nil-safe) + the two nil-guarded insert points in CommitStaged (pre→prepare→commit-msg between
       EditMessage@389 and CommitTree@399; post-commit between UpdateRefCAS@410 and ClearSnapshot@428).
    2. internal/hooks/adapter.go (NEW file) — `DefaultRunner struct{}` satisfying CommitHookRunner structurally
       (delegates to RunCommitHooks/RunPostCommit; (dryRun,verbose)→HookOpts). NEW file ⇒ zero merge conflict
       with S2's runner.go edits.
    3. pkg/stagecoach/stagecoach.go — buildDeps sets `Hooks: hooks.DefaultRunner{}` (+ the internal/hooks import).
    4. internal/generate/hooks_freeze_test.go (NEW file, package generate_test) — the headline freeze invariant
       (a sentinel staged to the live index AFTER write-tree, during a blocking pre-commit hook, is NOT swept
       into the commit; the live index retains it — §20.2/§20.5) + a hook-abort→rescue test (FR-V7 + idempotent).
    5. docs/how-it-works.md — NEW "## Commit hooks on the plumbing path" subsection before "## Hook mode vs the
       snapshot-based flow" (Mode A, ride-with-work; cross-link §9.25 + the M4.T1 Mode B rewrite).

  ⚠️ **#1 — THE IMPORT CYCLE.** Do NOT `import "internal/hooks"` in generate.go (cycle: hooks→generate for
      RescueError). Inject via the CommitHookRunner interface (inlined dryRun+verbose, NO hooks.HookOpts).
      The adapter (hooks.DefaultRunner) is wired in pkg/stagecoach + used by the external freeze test. (§0/§1)

  ⚠️ **#2 — INSERT A reassigns treeSHA + msg (NOT new vars).** After a successful RunCommitHooks, do
      `treeSHA, msg = ft, fm` so ALL downstream (CommitTree, the CASError recovery recipe, the Result
      Subject/Message) use the hook-adjusted tree + message. The ORIGINAL snapshot is preserved in `signal`
      for the during-hook rescue (RestoreDefault disarms only before UpdateRefCAS). A hook ABORT returns
      BEFORE CommitTree (no dangling commit; HEAD+index untouched). (§3)

  ⚠️ **#3 — INSERT B is best-effort (post-commit exit DISREGARDED).** RunPostCommit always returns nil; the
      commit already landed. Place after UpdateRefCAS success, before ClearSnapshot. Never undoes; a non-zero
      exit is logged via Verbose INSIDE RunPostCommit. (§3)

  ⚠️ **#4 — Deps.Hooks is nil-safe ⇒ the ~40 existing generate_test.go tests stay GREEN.** They construct
      Deps without Hooks → nil → CommitStaged skips hooks → byte-identical behavior. Only buildDeps (CLI) and
      the freeze test wire the real runner. (§2/§5)

  ⚠️ **#5 — the freeze test is `package generate_test` (EXTERNAL), not white-box.** A white-box `package
      generate` test CANNOT import internal/hooks (cycle). The external test imports hooks.DefaultRunner +
      uses a blocking pre-commit hook + a goroutine to stage a sentinel to the LIVE index during the hook's
      window — proving the scoped throwaway index excludes it (FR-V3). (§6)

  ⚠️ **#6 — DefaultRunner is a NEW file (adapter.go), NOT appended to runner.go.** S2 edits runner.go in
      parallel; adapter.go is disjoint (zero merge conflict). It satisfies generate.CommitHookRunner
      STRUCTURALLY (no generate import — Go duck typing). (§4)

  ⚠️ **#7 — DO NOT touch runner.go/subset.go/git.go/decompose/cmd.** S1/S2 own runner.go; subset.go is
      P1.M2.T2.S1; git.go's CommentChar is S2's; decompose (runSingleEscape) is T3; hookexec correctly has
      Hooks:nil (it IS the hook). go.mod unchanged. (§0/§5/§8)

  Deliverable: MODIFIED generate.go + NEW adapter.go + MODIFIED pkg/stagecoach/stagecoach.go + NEW
  hooks_freeze_test.go + MODIFIED docs/how-it-works.md. OUTPUT: every CommitStaged commit (the CLI
  single-commit path) runs the repo's hooks scoped to the frozen snapshot; a hook abort is a rescue (exit 3,
  FR-V7); post-commit fires best-effort; the freeze holds (a concurrent live-index stage is never swept in).
  `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Wire S1/S2's commit-hooks runner into `generate.CommitStaged` (the single-commit plumbing
path, PRD §9.25 FR-V1/V2/V7) so every CLI-produced single commit runs the repo's `pre-commit` →
`prepare-commit-msg` → `commit-msg` hooks scoped to the frozen snapshot (FR-V3 freeze holds), with a hook
abort mapped to the existing rescue (FR-V7, exit 3) and `post-commit` fired best-effort after `update-ref`.
The wiring is via DEPENDENCY INJECTION (a `CommitHookRunner` interface on `Deps`) to break the
generate↔hooks import cycle. Includes the headline freeze-safety invariant test (§20.2/§20.5): a sentinel
staged to the live index during a pre-commit hook is NOT swept into the commit.

**Deliverable**:
1. **MODIFIED `internal/generate/generate.go`** — `CommitHookRunner` interface (inlined `dryRun bool, verbose
   *ui.Verbose`) + `Deps.Hooks CommitHookRunner` (nil-safe) + the two nil-guarded insert points in
   `CommitStaged` (pre→prepare→commit-msg between EditMessage and CommitTree; post-commit between UpdateRefCAS
   success and ClearSnapshot).
2. **NEW `internal/hooks/adapter.go`** — `DefaultRunner struct{}` satisfying `CommitHookRunner` structurally
   (delegates to `RunCommitHooks`/`RunPostCommit`; `(dryRun, verbose)` → `HookOpts`).
3. **MODIFIED `pkg/stagecoach/stagecoach.go`** — `buildDeps` wires `Hooks: hooks.DefaultRunner{}` (+ the import).
4. **NEW `internal/generate/hooks_freeze_test.go`** (`package generate_test`) — the freeze invariant
   (blocking pre-commit hook + concurrent live-index sentinel stage → not swept) + the hook-abort→rescue test.
5. **MODIFIED `docs/how-it-works.md`** — NEW `## Commit hooks on the plumbing path` subsection (Mode A).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; a single-commit CLI run with
a repo pre-commit hook fires the hook scoped to the snapshot; a non-zero pre-commit → CommitStaged returns a
`*RescueError` (exit 3) with HEAD + index idempotent; post-commit fires best-effort; a sentinel staged to the
live index during a pre-commit hook is NOT in the commit's tree and IS retained staged in the live index; the
~40 existing generate_test.go tests stay GREEN (Deps.Hooks nil → skipped); go.mod/go.sum + S1/S2's runner.go
+ subset.go + git.go byte-unchanged.

## User Persona

**Target User**: A user who runs `stagecoach` (the snapshot/plumbing path) with repo commit hooks installed
(husky, lint-staged, a formatter, conventional-commit-lint, a post-commit notify). Before this task, those
hooks never fired on a stagecoach commit (the plumbing path bypassed them); after, they fire in git's order,
scoped to the frozen snapshot, so the stage-while-generating guarantee holds AND hooks run. Transitively:
P1.M3.T3 (decompose wiring, which reuses the same CommitHookRunner/DefaultRunner) and P1.M4.T1 (the docs
Mode B rewrite).

**Use Case**: A user with a `pre-commit` formatter runs `stagecoach`. The formatter reformats a file already
in the snapshot (permitted mutation) → stagecoach re-trees → the commit includes the formatter's fix (exactly
like `git commit`). Meanwhile, the user stages ANOTHER file during the hook → it is NOT swept in (the scoped
throwaway index excludes it; the freeze holds). If the `pre-commit` exits non-zero (a lint failure), the
commit is aborted as a rescue (exit 3, the manual recovery printed) — HEAD and the index are unchanged.

**User Journey**: `stagecoach` (CLI) → `pkg/stagecoach` → `buildDeps` (wires `Hooks: DefaultRunner{}`) →
`generate.CommitStaged` → …WriteTree (snapshot)… → EditMessage → **RunCommitHooks** (pre→prepare→commit-msg,
scoped throwaway index) → CommitTree(hook-adjusted tree) → UpdateRefCAS → **RunPostCommit** (best-effort) →
DiffTree → Result. A Ctrl-C during a hook → the existing signal rescue (armed at WriteTree; RestoreDefault
runs only before UpdateRefCAS).

**Pain Points Addressed**: (1) Hooks were silently bypassed on the plumbing path (the §9.25 / US19 gap) —
now they fire in git's order. (2) Naively running hooks against the LIVE index would sweep in
staging-during-generation (breaking the core §5 freeze) — solved by the scoped throwaway index (S1's
runPreCommitScoped), which this wiring consumes. (3) A hook failure leaving a half-commit — solved by mapping
it to the existing rescue (FR-V7, byte-identical to a generation failure).

## Why

- **Closes the §9.25 wiring for the single-commit path.** S1/S2 built the runner (the sequence + scoped
  pre-commit + message lifecycle + recursion prevention); this task THREADS it into the primary commit
  chokepoint (CommitStaged). Without it, the runner is an unused module.
- **Satisfies PRD §9.25 FR-V1/V2/V3/V7.** FR-V1 (run hooks in git's order around every stagecoach commit);
  FR-V2 (threaded between generation and commit-tree/update-ref); FR-V3 (scoped to the snapshot — the freeze
  holds); FR-V7 (a hook abort is a rescue, never a silent skip; post-commit best-effort).
- **Delivers the headline freeze-safety invariant (§20.2/§20.5, folded here per SOW §3).** The integrated
  test proves a concurrent live-index stage is never swept into the commit — the property that lets users
  keep staging while stagecoach commits.
- **Sets the injection seam T3 (decompose) reuses.** The `CommitHookRunner` interface + `DefaultRunner` are
  the single adapter both the single-commit path (here) and the decompose path (T3's publishCommit) consume.

## What

Modified `generate.go` (interface + Deps field + 2 insert points), new `internal/hooks/adapter.go`
(DefaultRunner), modified `pkg/stagecoach/stagecoach.go` (buildDeps wiring), new
`internal/generate/hooks_freeze_test.go` (the freeze + rescue tests), and modified `docs/how-it-works.md`
(new subsection). No change to the runner (S1/S2), subset.go, git.go, decompose, or cmd. go.mod unchanged.

### Success Criteria

- [ ] `generate.go` defines `CommitHookRunner` interface (methods take `ctx, g git.Git, cfg config.Config,
      snapshotTree, parentSHA, msg string, dryRun bool, verbose *ui.Verbose` — NO `hooks.HookOpts`) and adds
      `Hooks CommitHookRunner` to `Deps` (nil-safe; doc cites the cycle).
- [ ] `CommitStaged` runs `deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false,
      deps.Verbose)` nil-guarded between EditMessage and CommitTree; on success reassigns `treeSHA, msg =
      ft, fm`; on error returns `Result{}, herr` (a `*RescueError` or `ErrHookSweptConcurrentWork`).
- [ ] `CommitStaged` runs `deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, false, deps.Verbose)` nil-guarded
      after UpdateRefCAS success, before ClearSnapshot; the return is discarded (best-effort).
- [ ] `internal/hooks/adapter.go` (NEW) defines `DefaultRunner struct{}` with the 2 methods delegating to
      RunCommitHooks/RunPostCommit (HookOpts{DryRun, Verbose}); it does NOT import generate (structural).
- [ ] `pkg/stagecoach.buildDeps` returns `generate.Deps{…, Hooks: hooks.DefaultRunner{}}`; stagecoach.go imports
      `internal/hooks`.
- [ ] **Freeze test**: a sentinel staged to the LIVE index during a blocking pre-commit hook is NOT in the
      commit's tree (`ls-tree -r --name-only HEAD`) AND IS retained staged in the live index
      (`diff --cached --name-only`).
- [ ] **Rescue test**: a pre-commit exiting non-zero → CommitStaged returns a `*generate.RescueError`
      (`errors.As`); HEAD unchanged; `diff --cached --name-only` unchanged (idempotent, §20.2).
- [ ] `docs/how-it-works.md` has a new `## Commit hooks on the plumbing path` subsection before `## Hook mode
      vs the snapshot-based flow`, documenting scope/freeze/--no-verify/rescue/post-commit + a §9.25 cross-link.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on the edited files.
- [ ] go.mod/go.sum byte-unchanged; runner.go/subset.go/runner_test.go/git.go/decompose/cmd byte-unchanged;
      the ~40 existing generate_test.go tests stay GREEN.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact insert points
(quoted verbatim in the Blueprint, anchored to CommitStaged's EditMessage→CommitTree→UpdateRefCAS→ClearSnapshot
flow), the import-cycle rationale + the injection design (research §0/§1), the S1/S2 runner API (the frozen
RunCommitHooks/RunPostCommit signatures + HookOpts), the DefaultRunner adapter (copy-ready), the buildDeps
wiring (one line), the freeze-test mechanics (blocking hook + goroutine + concurrent live-index stage — copy-
ready), and the docs subsection content. No decompose/git-plumbing/prompt knowledge required.

### Documentation & References

```yaml
# MUST READ — the design calls (the cycle, the interface, the insert points, the freeze test, the docs)
- docfile: plan/010_49117f1f30ab/P1M3T2S1/research/design-decisions.md
  why: §0 (THE import cycle + injection resolution), §1 (CommitHookRunner interface, inlined params), §2
       (Deps.Hooks nil-safe ⇒ existing tests green), §3 (the two insert points + the treeSHA/msg reassign),
       §4 (DefaultRunner adapter, NEW file, structural), §5 (buildDeps wiring + the decompose/hookexec gaps),
       §6 (the freeze test mechanics — blocking hook + goroutine + concurrent live-index stage), §7 (docs
       subsection), §8 (no new deps + frozen files).
  critical: §0/§1 (do NOT import hooks in generate — inject via inlined-param interface), §3 (reassign
       treeSHA,msg not new vars; abort returns before CommitTree), §6 (external package generate_test; the
       blocking-hook window) are the things most likely to go wrong.

# MUST READ — the S1/S2 runner CONTRACT (the frozen API this task consumes)
- docfile: plan/010_49117f1f30ab/P1M3T1S1/PRP.md   (S1 — the runner core: COMPLETE)
- docfile: plan/010_49117f1f30ab/P1M3T1S2/PRP.md   (S2 — recursion prevention + message lifecycle: parallel contract)
  why: the EXACT signatures this task calls — RunCommitHooks(ctx, g git.Git, cfg, snapshotTree, parentSHA, msg,
       opts HookOpts) (finalTree, finalMsg, err); RunPostCommit(ctx, g, cfg, opts HookOpts) error; HookOpts{
       DryRun bool; Verbose *ui.Verbose}. S1's runPreCommitScoped enforces the freeze (throwaway index +
       enforceSubset → ErrHookSweptConcurrentWork on a new path); rescueErr builds *generate.RescueError.
  critical: the runner is the FROZEN API — this task CALLS it, does not modify it. HookOpts is {DryRun, Verbose}.

# MUST READ — the verified chokepoint pipeline + the freeze-test basis
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "## 5. The commit chokepoints" (CommitStaged pipeline: EditMessage@389 → CommitTree@399 →
           UpdateRefCAS@410 → ClearSnapshot@428; runPipeline is the dry-run path; decompose.publishCommit is
           T3); "## 6" (RescueError — FR-V7 byte-identical to a generation failure); "## 7" (signal arms rescue
           at WriteTree; RestoreDefault before UpdateRefCAS; hooks run inside the armed window); "## 8" (the
           how-it-works.md docs that contradict the feature — the Mode B rewrite is M4.T1).
  critical: §5 gives the EXACT insert-point line numbers; §7 confirms NO signal change is needed (hooks run
       inside the armed-snapshot window); §6 confirms a hook *RescueError is handled by the EXISTING CLI
       handleGenError → exit 3 + FormatRescue (no new exit code).

# THE FILE BEING MODIFIED — READ the CommitStaged function fully before editing
- file: internal/generate/generate.go
  section: the `Deps` struct (add Hooks); `CommitStaged` — the EditMessage block (`msg, err = EditMessage(…)`,
           ~L389) → "Step 7: commit-tree" (`newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)`,
           ~L399) → signal.RestoreDefault → UpdateRefCAS (~L410, incl. the CAS-handling `if err != nil {…}`)
           → "Step 9: diff-tree" + signal.ClearSnapshot (~L428); the Result construction (Subject/Message use
           `msg`); `RescueError`/`CASError` (TreeSHA/Message fields).
  why: the EXACT code this task edits. Confirms treeSHA/msg are referenced by CommitTree + CASError + Result
       (so reassigning them after a hook is the minimal-diff correct choice). Confirms signal.SetSnapshot
       (step 4) holds the ORIGINAL snapshot for the during-hook rescue.
  critical: INSERT A goes between the EditMessage `if err != nil` block and `// Step 7: commit-tree`; INSERT B
       goes between the UpdateRefCAS CAS-handling block's close and `// Step 9: diff-tree`. Do NOT move
       signal.RestoreDefault (it stays immediately before UpdateRefCAS).

# THE FILE BEING MODIFIED (wiring) — buildDeps
- file: pkg/stagecoach/stagecoach.go
  section: `func buildDeps(cfg, repoDir) (generate.Deps, error)` (~L325); the return `generate.Deps{Git:…,
           Manifest: m}` (~L386); the import block.
  why: the single wiring point for the CLI single-commit path. Add `Hooks: hooks.DefaultRunner{}` + the
       `internal/hooks` import.
  critical: pkg/stagecoach → hooks is a NEW edge but NOT a cycle (hooks→generate; generate→neither). buildDeps
       is the ONE place the real runner is wired for the single-commit path.

# THE FILES BEING CREATED — copy-ready
- file: internal/hooks/adapter.go   (NEW — DefaultRunner; research §4)
- file: internal/generate/hooks_freeze_test.go   (NEW, package generate_test; research §6)

# The test conventions (mirror)
- file: internal/generate/generate_test.go
  section: `initRepo(t, dir)` (L26 — temp repo + identity); the `stubtest.Build`/`stubtest.Manifest`/
           `stubtest.NewScript` pattern (a stub agent binary returning a fixed message); the
           `CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m}, cfg)` call shape.
  why: the freeze test MIRRORS this (real git.New(repo) + stub Manifest) but is `package generate_test`
       (external — to import hooks) so it CANNOT reuse the white-box initRepo; it defines its own minimal
       `initTempRepo`. Confirms cfg defaults (NoVerify=false) run hooks.
  critical: the freeze test must use a UNIQUE stub message (not duplicating the seed subject) so the dedupe
       loop accepts it on the first attempt.

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/010_…/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" FR-V1 (order), FR-V2 (threaded between gen + commit-tree),
           FR-V3 (scoped to snapshot — freeze holds), FR-V7 (failure is a rescue; post-commit best-effort);
           "20.2/20.5" (the freeze invariant + the e2e harness); "13.5" (edge cases).
  critical: FR-V3 is the freeze (pre-commit against a throwaway index; a new path ⇒ hard error); FR-V7 is the
       rescue mapping (byte-identical to a generation failure; post-commit disregarded).
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go                    # EDIT — CommitHookRunner interface + Deps.Hooks + 2 insert points
  generate_test.go               # package generate (white-box) — UNCHANGED (~40 tests stay green: Deps.Hooks nil ⇒ skipped)
  hooks_freeze_test.go           # NEW — package generate_test (external) — the freeze + rescue tests
internal/hooks/
  runner.go                      # S1/S2 (COMPLETE/in-progress) — RunCommitHooks/RunPostCommit/HookOpts. UNCHANGED.
  adapter.go                     # NEW — DefaultRunner (satisfies CommitHookRunner structurally)
  runner_test.go / subset.go     # S1 / P1.M2.T2.S1 — UNCHANGED
pkg/stagecoach/
  stagecoach.go                   # EDIT — buildDeps wires Hooks: hooks.DefaultRunner{} (+ import)
docs/
  how-it-works.md                # EDIT — NEW "## Commit hooks on the plumbing path" subsection
go.mod / go.sum                  # UNCHANGED (internal/hooks already in-module; no new external dep)
```

### Desired Codebase tree with files to be added

```bash
internal/hooks/adapter.go                  # NEW — DefaultRunner struct{} (2 methods → RunCommitHooks/RunPostCommit)
internal/generate/hooks_freeze_test.go     # NEW (package generate_test) — freeze invariant + hook-abort→rescue tests
# + in-place edits to generate.go (interface+Deps+inserts), pkg/stagecoach/stagecoach.go (buildDeps), docs/how-it-works.md.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — THE IMPORT CYCLE): internal/hooks imports internal/generate (for RescueError). generate.go
//   MUST NOT import internal/hooks (cycle). Inject via CommitHookRunner (interface in generate, inlined
//   dryRun+verbose params — NEVER hooks.HookOpts). The adapter hooks.DefaultRunner is wired in pkg/stagecoach.
//   (research §0/§1)

// CRITICAL (#2 — INSERT A reassigns treeSHA + msg, NOT new vars): after a successful RunCommitHooks, do
//   `treeSHA, msg = ft, fm`. Downstream CommitTree + the CASError recovery recipe + the Result (Subject/Message)
//   ALL reference treeSHA/msg — reassigning makes them use the hook-adjusted values (minimal diff). The original
//   snapshot is preserved in signal for the during-hook rescue. A hook ABORT returns BEFORE CommitTree (no
//   dangling commit; HEAD+index untouched). (research §3)

// CRITICAL (#3 — INSERT B best-effort; post-commit exit DISREGARDED): `_ = deps.Hooks.RunPostCommit(…)`.
//   RunPostCommit always returns nil; the commit already landed. Place after UpdateRefCAS success, before
//   ClearSnapshot. (research §3)

// CRITICAL (#4 — Deps.Hooks nil-safe ⇒ existing tests green): guard EVERY call `if deps.Hooks != nil`. The
//   ~40 generate_test.go tests build Deps without Hooks → nil → skipped → byte-identical behavior. (research §2)

// CRITICAL (#5 — the freeze test is package generate_test, EXTERNAL): a white-box package generate test
//   CANNOT import internal/hooks (cycle). The external test imports hooks.DefaultRunner + uses a blocking
//   pre-commit hook + a goroutine to stage a sentinel to the LIVE index during the hook window. (research §6)

// GOTCHA (DefaultRunner is a NEW file adapter.go, NOT in runner.go): S2 edits runner.go in parallel; adapter.go
//   is disjoint (zero merge conflict). DefaultRunner satisfies CommitHookRunner STRUCTURALLY (no generate import).
//   (research §4)
// GOTCHA (decompose.runSingleEscape + hookexec have Hooks:nil): runSingleEscape (decompose.go:294) constructs
//   Deps without Hooks → skipped (T3 wires decompose); hookexec.go:131 → nil → SKIPPED (correct: it IS the hook).
//   buildDeps is the ONE place the real runner is wired for the single-commit path. (research §5)
// GOTCHA (signal unchanged): hooks run between EditMessage and CommitTree — INSIDE signal's armed-snapshot window
//   (SetSnapshot@step4; RestoreDefault runs only before UpdateRefCAS). A Ctrl-C during a hook → the existing
//   rescue (original snapshot). NO signal change. (reality §7)
// GOTCHA (no new exit code / rescue variant): a hook *RescueError is handled by the EXISTING CLI handleGenError
//   → exit 3 + FormatRescue. ErrHookSweptConcurrentWork is a non-rescue freeze abort. (reality §6)
// GOTCHA (do NOT touch runner.go/subset.go/git.go/decompose/cmd/PRD): S1/S2 own runner.go; subset.go is
//   P1.M2.T2.S1; git.go's CommentChar is S2's; decompose is T3; cmd is untouched. go.mod unchanged. (research §8)
// GOTCHA (DOCS — do NOT rewrite the "Bypasses pre-commit hooks" line at how-it-works.md:312): that's the Mode B
//   headline rewrite owned by P1.M4.T1. This task ADDS a new subsection + a cross-link. (research §7)
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/generate/generate.go — the injection seam (NO new import; git/config/ui already imported) ===

// CommitHookRunner runs the repo's commit hooks around the plumbing commit path (PRD §9.25). Injected into
// Deps (NOT called as hooks.RunCommitHooks) to break the generate↔hooks import cycle (hooks imports generate
// for RescueError). The CLI wires hooks.DefaultRunner; tests inject a stub OR nil (nil ⇒ hooks skipped —
// back-compatible). dryRun+verbose are INLINED (not hooks.HookOpts) so generate need not import hooks.
type CommitHookRunner interface {
	RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
		dryRun bool, verbose *ui.Verbose) (finalTree, finalMsg string, err error)
	RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, dryRun bool, verbose *ui.Verbose) error
}

// Deps += Hooks field (add alongside Git/Manifest/Verbose/Excludes/Progress):
type Deps struct {
	Git      git.Git
	Manifest provider.Manifest
	Verbose  *ui.Verbose
	Excludes []string
	Progress func()
	// Hooks runs the repo's commit hooks around the commit (PRD §9.25). Injected (not hooks.RunCommitHooks)
	// to break the generate↔hooks import cycle. nil ⇒ hooks skipped (no-op) — back-compatible with the
	// legacy no-hooks CommitStaged tests. Wired by pkg/stagecoach.buildDeps (hooks.DefaultRunner{}).
	Hooks CommitHookRunner
}
```

```go
// === internal/hooks/adapter.go — NEW file (package hooks; NO new import; NO generate import) ===
package hooks

import (
	"context"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/ui"
)

// DefaultRunner is the production CommitHookRunner (generate.CommitHookRunner): it delegates to the package-
// level RunCommitHooks/RunPostCommit (S1/S2), translating the inlined (dryRun, verbose) to HookOpts. It
// satisfies generate.CommitHookRunner STRUCTURALLY (Go duck typing) — it does NOT import generate, so it adds
// no edge to the generate↔hooks graph. Wired into generate.Deps by pkg/stagecoach.buildDeps; also used by the
// hooks_freeze_test. (P1.M3.T2.S1 — the injection adapter; a NEW file so it never collides with S1/S2's
// runner.go.)
type DefaultRunner struct{}

func (DefaultRunner) RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config,
	snapshotTree, parentSHA, msg string, dryRun bool, verbose *ui.Verbose) (string, string, error) {
	return RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts{DryRun: dryRun, Verbose: verbose})
}

func (DefaultRunner) RunPostCommit(ctx context.Context, g git.Git, cfg config.Config,
	dryRun bool, verbose *ui.Verbose) error {
	return RunPostCommit(ctx, g, cfg, HookOpts{DryRun: dryRun, Verbose: verbose})
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/generate.go — CommitHookRunner interface + Deps.Hooks
  - ADD the CommitHookRunner interface (inlined dryRun bool + verbose *ui.Verbose; NO hooks.HookOpts).
  - ADD `Hooks CommitHookRunner` to the Deps struct (nil-safe; doc cites the cycle + the back-compat).
  - GOTCHA: NO new import (git.Git/config.Config/*ui.Verbose already imported). Do NOT import internal/hooks.

Task 2: EDIT internal/generate/generate.go — INSERT A (pre→prepare→commit-msg) in CommitStaged
  - INSERT (nil-guarded) between the EditMessage `if err != nil {…}` block and `// Step 7: commit-tree`:
      `ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)`;
      `if herr != nil { return Result{}, herr }`; `treeSHA, msg = ft, fm` (+ comment).
  - GOTCHA: reassign treeSHA + msg (downstream CommitTree/CASError/Result use them). DryRun is false here
      (CommitStaged is the !DryRun path). A hook abort returns BEFORE CommitTree.

Task 3: EDIT internal/generate/generate.go — INSERT B (post-commit) in CommitStaged
  - INSERT (nil-guarded) between the UpdateRefCAS CAS-handling block's close and `// Step 9: diff-tree`:
      `_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, false, deps.Verbose)` (+ best-effort comment).
  - GOTCHA: discard the return (post-commit exit disregarded — FR-V7). After UpdateRefCAS success, before
      signal.ClearSnapshot.

Task 4: CREATE internal/hooks/adapter.go — DefaultRunner
  - NEW file (package hooks). Define `DefaultRunner struct{}` + the 2 methods delegating to RunCommitHooks/
      RunPostCommit (HookOpts{DryRun: dryRun, Verbose: verbose}). NO generate import (structural).
  - GOTCHA: NEW file (disjoint from S2's runner.go). Imports only context/config/git/ui (runner.go already
      imports them).

Task 5: EDIT pkg/stagecoach/stagecoach.go — buildDeps wiring
  - ADD `"github.com/dustin/stagecoach/internal/hooks"` to the imports.
  - In buildDeps's return: `generate.Deps{Git: git.New(repoDir), Manifest: m, Hooks: hooks.DefaultRunner{}}`.
  - GOTCHA: pkg/stagecoach → hooks is a new edge but NOT a cycle. buildDeps is the ONE wiring point for the
      single-commit CLI path.

Task 6: CREATE internal/generate/hooks_freeze_test.go — the freeze + rescue tests (Blueprint below)
  - NEW file, `package generate_test` (external — to import hooks). 
  - Test 1 (freeze): blocking pre-commit hook + goroutine + concurrent live-index sentinel stage → assert
      the commit's tree OMITS the sentinel AND the live index RETAINS it staged.
  - Test 2 (rescue): pre-commit exit 1 → *generate.RescueError (errors.As) + HEAD unchanged + index idempotent.
  - GOTCHA: define a local initTempRepo (can't reuse the white-box initRepo). Use stubtest.Manifest with a
      UNIQUE message (not duplicating the seed). cfg NoVerify=false.

Task 7: EDIT docs/how-it-works.md — NEW subsection (Blueprint below)
  - INSERT `## Commit hooks on the plumbing path` IMMEDIATELY BEFORE `## Hook mode vs the snapshot-based flow`.
  - Document: git-order hooks around every stagecoach commit; scoped to the frozen snapshot (freeze holds);
      --no-verify skips pre-commit+commit-msg; hook abort = rescue (exit 3); post-commit best-effort.
      Cross-link §9.25 + the M4.T1 Mode B rewrite.
  - GOTCHA: do NOT rewrite the "Bypasses pre-commit hooks" line (M4.T1 owns it).

Task 8: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. runner.go/subset.go/git.go/
      decompose/cmd byte-unchanged. The ~40 existing generate_test.go tests GREEN. `go build/vet/test ./...` green.
```

### Implementation Patterns & Key Details

```go
// === INSERT A (between EditMessage and CommitTree) — reassign treeSHA + msg on success ===
	if deps.Hooks != nil {
		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)
		if herr != nil {
			return Result{}, herr // *RescueError (FR-V7 → exit 3) or ErrHookSweptConcurrentWork (FR-V3 freeze)
		}
		treeSHA, msg = ft, fm // hook may have re-treed (permitted mutation) + annotated the msg; all downstream
		                       // (CommitTree, CASError recovery, Result) uses these adjusted values
	}

// === INSERT B (after UpdateRefCAS success, before ClearSnapshot) — best-effort ===
	if deps.Hooks != nil {
		_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, false, deps.Verbose) // exit disregarded (FR-V7)
	}
```

```go
// === internal/generate/hooks_freeze_test.go (package generate_test) — the headline freeze invariant ===
package generate_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/hooks"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// initTempRepo creates a temp git repo with identity + a seed commit, returns its dir.
func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, c := range [][]string{
		{"git", "init", "-q", dir},
		{"git", "-C", dir, "config", "user.email", "t@e.com"},
		{"git", "-C", dir, "config", "user.name", "T"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	seed := filepath.Join(dir, "fileA.txt")
	os.WriteFile(seed, []byte("a\n"), 0o644)
	exec.Command("git", "-C", dir, "add", "fileA.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-q", "-m", "seed: initial").Run()
	return dir
}

// TestCommitStaged_PreCommitFree_HoldsForLiveStagedSentinel — the §20.2/§20.5 freeze invariant: a sentinel
// staged to the LIVE index AFTER write-tree (during a blocking pre-commit hook) is NOT swept into the commit
// (the scoped throwaway index excludes it), and the live index RETAINS it staged.
func TestCommitStaged_PreCommitFree_HoldsForLiveStagedSentinel(t *testing.T) {
	repo := initTempRepo(t)
	// Stage a real change for the snapshot.
	os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-modified\n"), 0o644)
	exec.Command("git", "-C", repo, "add", "fileA.txt").Run()

	// A blocking pre-commit hook: signals READY, waits for PROCEED, exits 0 (no mutation).
	tmp := t.TempDir()
	ready := filepath.Join(tmp, "ready")
	proceed := filepath.Join(tmp, "proceed")
	hookBody := "#!/bin/sh\ntouch " + ready + "\nwhile [ ! -f " + proceed + " ]; do sleep 0.02; done\nexit 0\n"
	hooksDir := filepath.Join(repo, ".git", "hooks")
	os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookBody), 0o755)

	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: unique freeze msg"})
	cfg := config.Defaults()
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}, Verbose: nil}

	// Run CommitStaged in a goroutine (the stub agent is instant → it blocks inside RunCommitHooks).
	done := make(chan struct{})
	var res generate.Result
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() { res, err = generate.CommitStaged(ctx, deps, cfg); close(done) }()

	// Wait for the hook to signal READY (it's running, scoped to the throwaway index).
	deadline := time.Now().Add(5 * time.Second)
	for { if _, e := os.Stat(ready); e == nil || time.Now().After(deadline); { break } ; time.Sleep(20 * time.Millisecond) }
	if _, e := os.Stat(ready); e != nil {
		t.Fatalf("pre-commit hook did not start (no ready file): %v", e)
	}

	// NOW stage the sentinel to the LIVE index (concurrent `git add` → .git/index; NOT the throwaway).
	os.WriteFile(filepath.Join(repo, "sentinel.txt"), []byte("s\n"), 0o644)
	if out, e := exec.Command("git", "-C", repo, "add", "sentinel.txt").CombinedOutput(); e != nil {
		t.Fatalf("stage sentinel: %v %s", e, out)
	}
	// Release the hook.
	os.WriteFile(proceed, []byte{}, 0o644)
	<-done

	if err != nil {
		t.Fatalf("CommitStaged err=%v (hook exited 0 — expected success)", err)
	}

	// ASSERT (a): the commit's tree OMITS the sentinel (the scoped throwaway index excluded it).
	lsTree, _ := exec.Command("git", "-C", repo, "ls-tree", "-r", "--name-only", "HEAD").Output()
	if strings.Contains(string(lsTree), "sentinel.txt") {
		t.Errorf("FREEZE VIOLATED: sentinel swept into the commit:\n%s", lsTree)
	}
	if !strings.Contains(string(lsTree), "fileA.txt") {
		t.Errorf("expected fileA in the commit:\n%s", lsTree)
	}
	// ASSERT (b): the LIVE index RETAINS the sentinel staged.
	diffCached, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if !strings.Contains(string(diffCached), "sentinel.txt") {
		t.Errorf("expected the sentinel to remain staged in the live index:\n%s", diffCached)
	}
	_ = res
}

// TestCommitStaged_PreCommitAbort_IsRescue — FR-V7: a non-zero pre-commit → *RescueError + HEAD/index idempotent.
func TestCommitStaged_PreCommitAbort_IsRescue(t *testing.T) {
	repo := initTempRepo(t)
	os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-mod\n"), 0o644)
	exec.Command("git", "-C", repo, "add", "fileA.txt").Run()

	hooksDir := filepath.Join(repo, ".git", "hooks")
	os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte("#!/bin/sh\necho 'lint failed' 1>&2\nexit 1\n"), 0o755)

	headBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	stagedBefore, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()

	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: unique rescue msg"})
	cfg := config.Defaults()
	deps := generate.Deps{Git: git.New(repo), Manifest: m, Hooks: hooks.DefaultRunner{}}

	_, err := generate.CommitStaged(context.Background(), deps, cfg)
	if err == nil {
		t.Fatal("expected a hook-abort error, got nil")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("expected *generate.RescueError (FR-V7), got %T: %v", err, err)
	}
	// HEAD unchanged (no update-ref ran).
	headAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD moved on hook abort: %s → %s", headBefore, headAfter)
	}
	// Index idempotent (§20.2 property 1).
	stagedAfter, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if string(stagedBefore) != string(stagedAfter) {
		t.Errorf("live index changed on hook abort:\nbefore: %s\nafter:  %s", stagedBefore, stagedAfter)
	}
}
```

```go
// === docs/how-it-works.md — NEW subsection (insert IMMEDIATELY BEFORE "## Hook mode vs the snapshot-based flow") ===

// ## Commit hooks on the plumbing path
//
// As of v2.4, the snapshot-based flow runs your repository's standard commit hooks itself — you no longer
// need hook mode (§9.20) just to get `pre-commit`, `commit-msg`, or `post-commit` to fire on a `stagecoach`
// commit. Hooks run in git's documented order around every commit produced by the plumbing path: `pre-commit`
// → `prepare-commit-msg` → `commit-msg` before the commit object is created, and `post-commit` after it is
// published.
//
// The snapshot freeze still holds: `pre-commit` runs against a throwaway index primed from the frozen
// `write-tree` snapshot, never the live index — so files you stage *while* the hook runs are never swept into
// the in-flight commit (the core stage-while-generating guarantee). A `pre-commit` may modify paths already in
// the snapshot (a formatter re-staging its output) and stagecoach includes those fixes, exactly like `git
// commit`; a `pre-commit` that stages a brand-new path aborts the run (it would sweep in concurrent work).
//
// `--no-verify` mirrors `git commit --no-verify`: it skips `pre-commit` and `commit-msg` only
// (`prepare-commit-msg` and `post-commit` still run). A hook that exits non-zero or times out aborts the run
// as a **rescue** (exit code 3) — no commit is created, HEAD and the index are byte-for-byte unchanged, and
// the rescue recipe is printed. `post-commit` is best-effort: its exit code is logged as a warning but cannot
// undo an already-landed commit (git itself disregards it).
//
// See PRD §9.25 (FR-V1–V8) for the full specification. (The "Hook mode vs the snapshot-based flow" framing
// below is being reconciled in the v2.4 docs rewrite — hook mode remains the bridge for plain `git commit`
// from IDEs, and the two modes now compose.)
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. adapter.go imports only in-module packages (config/git/ui) + stdlib
      context. pkg/stagecoach adds the in-module internal/hooks import. `go mod tidy` is a no-op.

PACKAGE EDGES:
  - generate → (config, git, ui, provider, …) — NO hooks (the cycle is avoided by the interface). NEW type:
        CommitHookRunner (in generate).
  - hooks → generate (RescueError — EXISTING) + config/git/ui. adapter.go (DefaultRunner) adds NO new edge
        (structural satisfaction; no generate import).
  - pkg/stagecoach → hooks (NEW edge — no cycle: hooks→generate; generate→neither; pkg/stagecoach→both).

UPSTREAM CONTRACT (consume, do NOT edit):
  - S1/S2's internal/hooks/runner.go: RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts) →
        (finalTree, finalMsg, err); RunPostCommit(ctx, g, cfg, HookOpts) error; HookOpts{DryRun, Verbose}.
        runPreCommitScoped enforces the freeze (throwaway index + enforceSubset); rescueErr → *RescueError.
  - config: cfg.NoVerify (FR-V5), cfg.HookTimeout (FR-V6) — from P1.M1.T1.S1 (COMPLETE).

DOWNSTREAM CONTRACTS (NOT this task — these CONSUME the seam):
  - P1.M3.T2.S2 (runPipeline dry-run): commit-msg only on the would-be message (FR-V8a) — uses the same seam.
  - P1.M3.T3 (decompose.publishCommit): per-concept scoped hooks; wires DefaultRunner into the decompose Deps
        (incl. runSingleEscape, which currently has Hooks:nil — T3 threads it). resolveMidChain SKIPS hooks.
  - P1.M4.T1 (docs Mode B): rewrites the now-false "Bypasses pre-commit hooks" framing.

FROZEN/LEAVE (do NOT edit):
  - internal/hooks/runner.go (+_test.go), subset.go (+_test.go) — S1/S2 + P1.M2.T2.S1.
  - internal/git/* (CommentChar is S2's), internal/cmd/*, internal/decompose/* (T3), internal/hook/*,
    internal/signal/*, internal/lock/*, internal/generate/{rescue,finalize,dedupe,multiturn}.go.
  - PRD.md, go.mod, Makefile. The ~40 existing generate_test.go tests (Deps.Hooks nil ⇒ skipped ⇒ green).

NO NEW DATABASE / ROUTES / CLI COMMANDS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/generate/generate.go internal/hooks/adapter.go pkg/stagecoach/stagecoach.go \
  internal/generate/hooks_freeze_test.go docs/how-it-works.md 2>/dev/null
go vet ./internal/generate/ ./internal/hooks/ ./pkg/stagecoach/
go build ./...
# Confirm the seam + adapter + wiring present, and NO hooks import in generate:
grep -n 'CommitHookRunner\|Hooks CommitHookRunner' internal/generate/generate.go
grep -n 'DefaultRunner' internal/hooks/adapter.go pkg/stagecoach/stagecoach.go
! grep -n 'internal/hooks' internal/generate/generate.go && echo "generate.go does NOT import hooks (cycle avoided ✓)"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; build clean; seam+adapter+wiring present; generate.go does NOT import hooks; go.mod/go.sum byte-unchanged.
```

### Level 2: The freeze + rescue tests + the runner suite + existing generate tests

```bash
go test ./internal/generate/ -v -run 'TestCommitStaged_PreCommit'
# Expected PASS — verify:
#   ...HoldsForLiveStagedSentinel ... the commit's tree OMITS sentinel.txt; the live index RETAINS it staged (FREEZE ✓)
#   ...Abort_IsRescue ............... a non-zero pre-commit → *generate.RescueError; HEAD unchanged; index idempotent (FR-V7 ✓)
go test ./internal/generate/   # the ~40 existing generate_test.go tests MUST stay GREEN (Deps.Hooks nil ⇒ skipped)
go test ./internal/hooks/...   # S1/S2's runner tests MUST stay green (this task didn't touch runner.go)
go test ./pkg/stagecoach/...    # buildDeps wiring compiles + no regression
# If HoldsForLiveStagedSentinel fails with "sentinel swept into the commit", the scoped throwaway index isn't
# being used (check S1's runPreCommitScoped primed from snapshotTree) OR the hook isn't running (check Hooks wired).
# If Abort_IsRescue fails (not a *RescueError), INSERT A isn't returning the runner's error verbatim — it must
# `return Result{}, herr` (herr IS the *RescueError from rescueErr).
```

### Level 3: Whole-repo + frozen-file check

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS (generate + hooks + stagecoach + the repo-wide suite).
git diff --name-only | grep -E 'internal/generate/generate\.go|internal/hooks/adapter\.go|pkg/stagecoach/stagecoach\.go|internal/generate/hooks_freeze_test\.go|docs/how-it-works\.md' && echo "(expected files)"
git diff --exit-code internal/hooks/runner.go internal/hooks/runner_test.go internal/hooks/subset.go \
  internal/git internal/cmd internal/decompose internal/hook internal/signal internal/lock \
  internal/generate/rescue.go internal/generate/finalize.go internal/generate/dedupe.go internal/generate/multiturn.go \
  PRD.md go.mod Makefile && echo "frozen files UNCHANGED (expected)"
# Confirm the existing generate_test.go is byte-unchanged (the ~40 tests stay green untouched):
git diff --exit-code internal/generate/generate_test.go && echo "generate_test.go UNCHANGED (good — tests stay green)"
# Confirm runSingleEscape + hookexec still construct Deps WITHOUT Hooks (T3/hookexec gaps):
grep -n 'generate.Deps{' internal/decompose/decompose.go internal/cmd/hookexec.go
```

### Level 4: FR-V3 + FR-V7 + freeze correctness reasoning

```bash
# Verify by reasoning + the tests:
#   1. The scoped throwaway index: pre-commit runs against GIT_INDEX_FILE=<tmp> primed from snapshotTree; the live
#      .git/index is never touched. A concurrent live-index stage (the sentinel) therefore cannot enter the commit.
#      (HoldsForLiveStagedSentinel — the headline §20.2/§20.5 invariant.)
#   2. FR-V7: a hook non-zero/timeout → *RescueError{snapshotTree, parentSHA, msg, Cause} (byte-identical to a
#      generation failure) → the EXISTING CLI handleGenError → exit 3 + FormatRescue. HEAD + index idempotent.
#      (Abort_IsRescue.)
#   3. INSERT A reassign: treeSHA/msg become the hook-adjusted values for CommitTree + CASError + Result. The
#      original snapshot is preserved in signal (during-hook Ctrl-C rescue). (Level 3 grep + reasoning.)
#   4. INSERT B: post-commit fires after update-ref, exit disregarded, never undoes. (reasoning.)
#   5. Nil-safety: Deps.Hooks nil ⇒ both inserts skipped ⇒ the legacy no-hooks tests are byte-identical.
#      (generate_test.go UNCHANGED + green.)
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (the freeze + rescue tests + S1/S2's runner tests + the ~40 existing generate tests + repo-wide).
- [ ] go.mod/go.sum byte-unchanged; generate.go does NOT import internal/hooks (cycle avoided).

### Feature Validation
- [ ] `CommitHookRunner` interface (inlined dryRun+verbose) + `Deps.Hooks` (nil-safe) defined.
- [ ] INSERT A: nil-guarded RunCommitHooks between EditMessage and CommitTree; reassigns treeSHA/msg; abort returns before CommitTree.
- [ ] INSERT B: nil-guarded RunPostCommit after UpdateRefCAS success, before ClearSnapshot; return discarded.
- [ ] `DefaultRunner` (NEW adapter.go) delegates to RunCommitHooks/RunPostCommit; structural (no generate import).
- [ ] buildDeps wires `Hooks: hooks.DefaultRunner{}`.
- [ ] **Freeze test**: live-index sentinel during a blocking pre-commit → NOT in the commit tree; RETAINED staged.
- [ ] **Rescue test**: non-zero pre-commit → `*RescueError`; HEAD + index idempotent.
- [ ] docs/how-it-works.md has the new subsection (cross-link §9.25 + M4.T1); the "Bypasses pre-commit hooks" line untouched.

### Code Quality Validation
- [ ] The import cycle is avoided via injection (no `import "internal/hooks"` in generate.go).
- [ ] S1/S2's runner.go + subset.go + git.go byte-unchanged; the ~40 existing generate tests untouched + green.
- [ ] DefaultRunner is a NEW file (disjoint from S2's runner.go); structural interface satisfaction.
- [ ] Anti-patterns avoided (see below); decompose (T3) + hookexec (correctly nil) not touched.

### Documentation
- [ ] generate.go doc comments cite PRD §9.25 FR-V1/V2/V3/V7 + the cycle rationale. The docs subsection documents
      scope/freeze/--no-verify/rescue/post-commit + the §9.25 + M4.T1 cross-links.

---

## Anti-Patterns to Avoid

- ❌ **Don't `import "internal/hooks"` in generate.go.** It closes the generate↔hooks cycle (hooks→generate for
  RescueError). Inject via the CommitHookRunner interface (inlined dryRun+verbose, NOT hooks.HookOpts). (§0/§1)
- ❌ **Don't use new variables for the hook-adjusted tree/msg in INSERT A.** Reassign `treeSHA, msg = ft, fm` so
  CommitTree + the CASError recovery recipe + the Result all use the hook's output (minimal diff, correct).
  (§3)
- ❌ **Don't let a hook abort reach CommitTree.** `if herr != nil { return Result{}, herr }` BEFORE CommitTree —
  no dangling commit; HEAD + index untouched. (§3)
- ❌ **Don't treat post-commit's return as an abort.** `_ = RunPostCommit(…)` — the exit is disregarded (FR-V7);
  the commit already landed. (§3)
- ❌ **Don't forget the nil-guard.** Every `deps.Hooks` call is `if deps.Hooks != nil` — the ~40 existing tests
  pass Deps without Hooks and MUST stay green (byte-identical, hooks skipped). (§2)
- ❌ **Don't write the freeze test as white-box `package generate`.** It must import hooks.DefaultRunner → it's
  `package generate_test` (external) or it won't compile (cycle). Use a blocking pre-commit hook + a goroutine
  to stage the sentinel to the LIVE index during the hook window. (§6)
- ❌ **Don't append DefaultRunner to runner.go.** S2 edits runner.go in parallel — use a NEW adapter.go (disjoint).
- ❌ **Don't rewrite the "Bypasses pre-commit hooks" line in how-it-works.md.** That's the Mode B headline
  rewrite (P1.M4.T1). ADD a new subsection + a cross-link. (§7)
- ❌ **Don't touch runner.go/subset.go/git.go/decompose/cmd/PRD.** S1/S2 own runner.go; subset.go is
  P1.M2.T2.S1; git.go's CommentChar is S2's; decompose is T3 (runSingleEscape flagged); hookexec correctly nil.
- ❌ **Don't add a new exit code / rescue variant.** A hook *RescueError is handled by the EXISTING CLI
  handleGenError → exit 3 + FormatRescue (byte-identical to a generation failure). (reality §6)

---

## Confidence Score

**9/10** — the wiring is mechanical once the import-cycle is recognized (the load-bearing insight, fully
specified: inject via an inlined-param interface; the adapter is copy-ready; buildDeps is one line). The two
insert points are anchored to verified line numbers (EditMessage@389 → CommitTree@399; UpdateRefCAS@410 →
ClearSnapshot@428) and the reassign-treeSHA/msg choice is the minimal-diff correct one (downstream CommitTree/
CASError/Result all reference them). The freeze test is the most intricate piece but its mechanics are sound:
a blocking pre-commit hook opens the only seam to stage a sentinel "after write-tree" (CommitStaged takes the
snapshot internally), the concurrent `git add` hits the LIVE index (distinct from the hook's GIT_INDEX_FILE=
throwaway), and the assertions directly prove the §20.2 invariant. The runner API (S1/S2) is the frozen
contract this task calls — not modifies — so it can't regress S1/S2's tests. The -1 reserves for: (a) the
external-test-package subtlety (the freeze test can't reuse the white-box initRepo — a local helper is
needed, copy-provided); (b) adapting to S2's exact final runner.go shape (S2 lands first; the RunCommitHooks/
RunPostCommit signatures are stable per the S1/S2 PRPs, but the freeze test's HookOpts translation must match
S2's final field set — {DryRun, Verbose}, confirmed). The decompose runSingleEscape gap (Hooks:nil until T3)
is correctly scoped out and flagged.
