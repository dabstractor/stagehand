# P1.M3.T2.S1 — Design Decisions

**Subtask**: generate.CommitStaged wiring + post-commit + staged-during-generation sentinel invariant
**Files**: `internal/generate/generate.go` (EDIT — interface + Deps field + 2 insert points),
`internal/hooks/adapter.go` (NEW — DefaultRunner), `pkg/stagecoach/stagecoach.go` (EDIT — buildDeps wires it),
`internal/generate/hooks_freeze_test.go` (NEW — package generate_test, the freeze test + the rescue test),
`docs/how-it-works.md` (EDIT — new subsection).
**PRD**: §9.25 FR-V1/V2/V3/V7 (the plumbing-path hooks), §13.5/§20.2/§20.5 (the freeze invariant).
**Consumes**: S1's `internal/hooks/runner.go` (RunCommitHooks/RunPostCommit/HookOpts — COMPLETE) + S2's
recursion-prevention/message-lifecycle (PARALLEL contract — treat runner.go as the frozen API).

---

## §0 — THE central decision: inject the hook runner (break the generate↔hooks import cycle)

`internal/hooks/runner.go` imports `internal/generate` (for `generate.RescueError` — `rescueErr` builds a
`*generate.RescueError{Kind:ErrRescue, TreeSHA, ParentSHA, Candidate, Cause}`). Therefore `generate.go`
CANNOT import `internal/hooks` — that would close a cycle `generate → hooks → generate` (Go rejects it).

**Resolution**: inject the hook runner into `generate.Deps` as an INTERFACE defined in the `generate`
package, whose method signatures reference ONLY types generate already imports (`git.Git`, `config.Config`,
`*ui.Verbose`, primitives) — NEVER `hooks.HookOpts` (that would re-introduce the cycle). The CLI
(`pkg/stagecoach.buildDeps`) wires a concrete adapter (`hooks.DefaultRunner`) that delegates to
`hooks.RunCommitHooks`/`RunPostCommit`, translating the inlined params to `HookOpts`. This mirrors how
`Manifest` is injected (Deps.Manifest is a struct from a package generate imports; here the runner is an
interface because it has behavior). The item explicitly anticipates this ("inject it, mirroring how Manifest
is injected, for testability").

SCOPE: this task owns generate.go (the seam + insert points) + adapter.go + the buildDeps wiring + the freeze
test + the docs subsection. It does NOT touch internal/hooks/runner.go (S1/S2 — frozen API), internal/hooks/
subset.go (P1.M2.T2.S1), internal/decompose/* (T3's domain — runSingleEscape flagged), internal/cmd/*, or
internal/git/git.go (CommentChar is S2's). go.mod unchanged.

## §1 — CommitHookRunner interface (inlined params; NO hooks type)

In `generate.go`:
```go
// CommitHookRunner runs the repo's commit hooks around the plumbing commit path (PRD §9.25). Injected
// into Deps (NOT called as hooks.RunCommitHooks) to break the generate↔hooks import cycle (hooks imports
// generate for RescueError). The CLI wires hooks.DefaultRunner; tests inject a stub OR nil (nil ⇒ hooks
// skipped — back-compatible with the no-hooks CommitStaged tests). dryRun+verbose are INLINED (not a
// hooks.HookOpts) so generate need not import hooks.
type CommitHookRunner interface {
	RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
		dryRun bool, verbose *ui.Verbose) (finalTree, finalMsg string, err error)
	RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, dryRun bool, verbose *ui.Verbose) error
}
```
WHY inlined `dryRun bool, verbose *ui.Verbose` and NOT `hooks.HookOpts`: a `hooks.HookOpts` parameter would
force `generate` to import `internal/hooks` → cycle. Inlining the two fields (DryRun, Verbose) breaks the
cycle with zero information loss (HookOpts is exactly those two fields). `git.Git`/`config.Config`/
`*ui.Verbose` are all already imported by generate.go (Deps uses them) → NO new import.

## §2 — Deps.Hooks field (nil-safe ⇒ back-compatible)

Add to the `Deps` struct:
```go
	// Hooks runs the repo's commit hooks around the commit (PRD §9.25). Injected (not called as
	// hooks.RunCommitHooks) to break the generate↔hooks import cycle. nil ⇒ hooks skipped (no-op) —
	// back-compatible with the legacy no-hooks CommitStaged tests (which construct Deps without Hooks).
	Hooks CommitHookRunner
```
nil-safe: CommitStaged guards every call `if deps.Hooks != nil`. The ~40 existing generate_test.go tests
construct `Deps{Git:..., Manifest:...}` (no Hooks) → Hooks=nil → CommitStaged skips hooks → byte-identical
behavior → they stay GREEN untouched. The CLI (buildDeps) and the freeze test wire the real runner.

## §3 — The two insert points in CommitStaged

**INSERT A — pre→prepare→commit-msg (between EditMessage L389 and CommitTree L399).** After EditMessage
succeeds, BEFORE "Step 7: commit-tree":
```go
	// §9.25 FR-V1/V2: run the repo's pre→prepare→commit-msg hooks scoped to the frozen snapshot, between
	// the finalized message and commit-tree. Injected via Deps.Hooks (breaks the generate↔hooks cycle).
	// nil ⇒ no hooks (back-compatible). DryRun is always false here (CommitStaged is the !DryRun path;
	// the dry-run path is runPipeline, S2). A hook abort is a *RescueError (FR-V7 → exit 3) or
	// ErrHookSweptConcurrentWork (FR-V3 freeze backstop) — both leave HEAD + the live index untouched.
	if deps.Hooks != nil {
		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)
		if herr != nil {
			return Result{}, herr
		}
		treeSHA, msg = ft, fm // hook may have re-treed (permitted mutation) + annotated the message;
		                       // ALL downstream (CommitTree, CASError recovery, Result) use these adjusted values
	}
```
REASSIGN treeSHA + msg (not new vars): downstream CommitTree, the CASError (TreeSHA/Message for the recovery
recipe), and the Result (Subject/Message) ALL reference `treeSHA`/`msg` — reassigning makes them use the
hook-adjusted values with MINIMAL diff. The ORIGINAL snapshot is preserved in `signal` (SetSnapshot stored it
at step 4) for the during-hook Ctrl-C rescue (signal is still armed here — RestoreDefault runs only before
UpdateRefCAS). If a hook ABORTS (herr != nil), return BEFORE CommitTree → no dangling commit from the hook's
tree; HEAD + live index untouched; signal rescue prints the original snapshot (correct — nothing committed).

**INSERT B — post-commit (after UpdateRefCAS L410 succeeds, before ClearSnapshot L428).** Best-effort:
```go
	}  // (end of the UpdateRefCAS `if err != nil {…}` CAS-handling block)

	// §9.25 FR-V1: post-commit runs AFTER update-ref succeeded (best-effort; exit code DISREGARDED — FR-V7).
	// The commit already landed; RunPostCommit logs a non-zero exit as a --verbose warning and NEVER undoes.
	if deps.Hooks != nil {
		_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, false, deps.Verbose)
	}

	// Step 9: diff-tree
	signal.ClearSnapshot()
```
RunPostCommit ALWAYS returns nil (its impl); the `_ =` discards it explicitly (the contract: post-commit
exit code is never an abort). Placed after UpdateRefCAS success (the commit landed) and before ClearSnapshot
(signal disarmed right after — a Ctrl-C in this tiny window is default-disposition, not rescue; acceptable).

## §4 — DefaultRunner adapter (NEW internal/hooks/adapter.go)

A NEW file (disjoint from S1/S2's runner.go — zero merge conflict) with a struct that satisfies
`generate.CommitHookRunner` STRUCTURALLY (Go duck typing — it need NOT import generate):
```go
package hooks
// (imports: context + the same packages runner.go already imports: config, git, ui — NO new import, NO generate import)

// DefaultRunner is the production CommitHookRunner (generate.CommitHookRunner): it delegates to the
// package-level RunCommitHooks/RunPostCommit, translating the inlined (dryRun, verbose) to HookOpts. It
// satisfies generate.CommitHookRunner structurally (no generate import). Wired into generate.Deps by
// pkg/stagecoach.buildDeps; also used by the hooks_freeze_test.
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
WHY a NEW file (not appended to runner.go): S2 is editing runner.go in parallel (the seam + message
lifecycle). adapter.go is a separate file → no merge collision. DefaultRunner is wiring-layer (the injection
adapter) — M3.T2's concern, not S1/S2's runner core. It imports nothing new (runner.go already imports
config/git/ui); it does NOT import generate (structural satisfaction).

## §5 — Wiring in pkg/stagecoach.buildDeps (the single-commit CLI path)

`pkg/stagecoach/stagecoach.go:buildDeps` (returns `generate.Deps{Git:..., Manifest:...}` at L386) — add the
Hooks field:
```go
	return generate.Deps{Git: git.New(repoDir), Manifest: m, Hooks: hooks.DefaultRunner{}}, nil
```
+ add `"github.com/dustin/stagecoach/internal/hooks"` to stagecoach.go imports (NEW edge — pkg/stagecoach →
hooks; NO cycle: hooks → generate, pkg/stagecoach → generate + hooks, generate → neither). This covers the
single-commit CLI path (internal/cmd/default_action → pkg/stagecoach → buildDeps → CommitStaged).

KNOWN GAPS (NOT this task — flagged):
- `internal/decompose/decompose.go:294` (runSingleEscape → CommitStaged) constructs Deps WITHOUT Hooks →
  Hooks:nil → hooks SKIPPED there. runSingleEscape is a decompose path; P1.M3.T3 owns decompose hook wiring
  (publishCommit is the decompose chokepoint) and should thread the runner into runSingleEscape's Deps. Flag.
- `internal/cmd/hookexec.go:131` (`stagecoach hook exec`) constructs Deps WITHOUT Hooks → Hooks:nil → SKIPPED.
  This is CORRECT: `hook exec` IS the hook itself — running hooks there would recurse. Leave nil.

## §6 — The freeze test (headline acceptance invariant — §20.2/§20.5, folded here per SOW §3)

NEW file `internal/generate/hooks_freeze_test.go` (`package generate_test` — EXTERNAL, so it can import
`internal/hooks` for `DefaultRunner`; a white-box `package generate` test CANNOT import hooks — cycle).

**Test 1 — staged-during-generation is NOT swept (FR-V3 at the integrated CommitStaged level):**
1. temp repo (`git init` + identity + seed commit + stage fileA). Helper: a local `initTempRepo(t)`.
2. Install a BLOCKING pre-commit hook that signals ready then waits for a proceed file:
   `#!/bin/sh\ntouch "$READY"\nwhile [ ! -f "$PROCEED" ]; do sleep 0.02; done\nexit 0`.
3. Run CommitStaged in a goroutine (stub Manifest "feat: unique-msg"; `Hooks: hooks.DefaultRunner{}`;
   `cfg` with NoVerify=false). The stub agent is instant → generation completes → CommitStaged blocks inside
   RunCommitHooks (the hook is spinning on PROCEED).
4. Poll for `$READY` (up to ~2s) — confirms the hook is running (scoped to the throwaway index).
5. NOW stage the sentinel to the LIVE index: write `sentinel.txt` to the WT; `git -C repo add sentinel.txt`
   (a SEPARATE process, no GIT_INDEX_FILE → writes to `.git/index`, the live one).
6. Touch `$PROCEED` → the hook exits 0 (no mutation to the throwaway) → RunCommitHooks returns
   (finalTree == snapshotTree, finalMsg == msg) → CommitTree → UpdateRefCAS → CommitStaged returns.
7. `<-done`. ASSERT: `git -C repo ls-tree -r --name-only HEAD` does NOT contain `sentinel.txt` (the commit's
   tree came from the scoped throwaway index, primed from the snapshot — the live-index sentinel never
   entered it); AND `git -C repo diff --cached --name-only` DOES contain `sentinel.txt` (the live index
   retains the sentinel staged). This is the freeze: staging-during-(hook)-generation is not swept in.

WHY a blocking hook + goroutine: CommitStaged takes WriteTree internally (step 4); the ONLY seam to stage a
sentinel "after write-tree" is DURING the hook's execution. The blocking hook opens that window; the test
stages the sentinel to the live index through it. The stub Manifest makes generation instant (no real agent
delay), so the window is deterministic. (A real pre-commit hook runs scoped to GIT_INDEX_FILE=<throwaway>, so
the test's concurrent `git add` to the live `.git/index` is a DIFFERENT index — exactly the concurrent-user
scenario FR-V3 protects against.)

**Test 2 — a hook abort is a rescue (FR-V7) + HEAD/index idempotent:**
1. temp repo + stage fileA. Install a pre-commit hook `#!/bin/sh\nexit 1`.
2. Run CommitStaged (Hooks: DefaultRunner{}; stub Manifest). It returns an error.
3. ASSERT: `errors.As(err, &re)` for `*generate.RescueError` (a hook failure is byte-identical to a
   generation failure → exit 3); `git -C repo rev-parse HEAD` is UNCHANGED (== the seed); `git -C repo diff
   --cached --name-only` is UNCHANGED (idempotent index — §20.2 property 1). Pins FR-V7 + the §20.2 freeze.

## §7 — DOCS (how-it-works.md — NEW subsection, Mode A ride-with-work)

Insert a NEW `## Commit hooks on the plumbing path` subsection in `docs/how-it-works.md` IMMEDIATELY BEFORE
`## Hook mode vs the snapshot-based flow` (~line 303 — after the multi-turn-fallback section). Content:
- The plumbing path now runs the repo's commit hooks in git's order around every stagecoach commit
  (pre-commit → prepare-commit-msg → commit-msg before commit-tree; post-commit after).
- pre-commit is SCOPED to the frozen snapshot (a throwaway index primed from the write-tree) — the
  stage-while-generating freeze HOLDS: files staged during generation are never swept in. A pre-commit that
  stages a NEW path (not in the snapshot) aborts (freeze backstop).
- `--no-verify` skips pre-commit + commit-msg (mirrors `git commit --no-verify`); prepare-commit-msg +
  post-commit still run.
- A hook abort (non-zero/timeout) is a RESCUE (exit 3) — the commit-tree never ran, HEAD + index are
  byte-for-byte unchanged, the rescue recipe prints. post-commit is best-effort (its exit is disregarded).
- Cross-link: PRD §9.25 (FR-V1–V8); note the "Hook mode vs the snapshot-based flow" section below is the
  Mode B headline rewrite (P1.M4.T1) — its "Bypasses pre-commit hooks" line is now superseded by this.

DO NOT rewrite the existing "Bypasses pre-commit hooks" bullet (L312) — that's the Mode B headline rewrite
owned by P1.M4.T1. This task ADDS the new subsection (Mode A, ride-with-work) + a cross-link noting M4.T1
will reconcile the now-stale framing.

## §8 — No new deps; frozen files; validation

- generate.go: NO new import (CommitHookRunner uses git.Git/config.Config/*ui.Verbose — already imported).
- adapter.go (hooks): imports config/git/ui (runner.go already imports them) — NO new, NO generate.
- pkg/stagecoach: += `internal/hooks` import (new edge, no cycle). go.mod UNCHANGED.
- FROZEN (do NOT edit): internal/hooks/runner.go (S1/S2), internal/hooks/subset.go, internal/hooks/
  runner_test.go, internal/git/* (CommentChar is S2's), internal/cmd/*, internal/decompose/* (T3),
  internal/hook/*, internal/signal/*, internal/lock/*. PRD.md, Makefile, go.mod.
- The ~40 existing generate_test.go tests stay GREEN (Deps without Hooks → nil → skipped).
