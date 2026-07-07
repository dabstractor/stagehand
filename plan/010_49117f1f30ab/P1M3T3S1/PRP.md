---
name: "P1.M3.T3.S1 — decompose.publishCommit hook wiring (per-concept scoped; arbiter new/tip run; mid-chain skips) + the §20.2 mid-chain-fidelity invariant: thread P1.M3.T1's hooks runner into the 3 decompose commit sites (publishCommit + resolveNewCommit + resolveTipAmend), leaving resolveMidChain hook-free — PRD §9.25 FR-V1/V3/V7/V8c / §13.6.5 / §20.2"
description: |

  Land the ONLY subtask of "Wire RunCommitHooks into the decompose path" (P1.M3.T3): thread P1.M3.T1's
  `internal/hooks` runner into the decompose MULTI-COMMIT commit sites so every per-concept + arbiter-new
  + arbiter-tip commit runs the repo's commit hooks scoped to its frozen tree (FR-V1/V8c), a hook abort
  maps to the existing rescue (FR-V7), post-commit fires best-effort, AND the arbiter mid-chain rebuild
  stays HOOK-FREE (preserving the §20.2 "non-target commits byte-identical" fidelity invariant). The runner
  is COMPLETE (P1.M3.T1.S1/S2); this task CONSUMES its frozen API.

  THE COMMIT CHOKEPOINTS (codebase_reality.md §5 + open_questions.md §2, verified against the code):
    - `decompose.publishCommit` (message.go:219) — the per-concept chokepoint, called by the main loop
      publish closure (decompose.go:484), runSingleShortcut (:390), runOneFileShortcut (:336). WIRE.
    - `resolveNewCommit` (chain.go) + `resolveTipAmend` (chain.go) — arbiter commits that do their OWN
      CommitTree+UpdateRefCAS (NOT via publishCommit). WIRE (user-facing commits).
    - `resolveMidChain` (chain.go) — the deterministic rebuild reusing msg[j] VERBATIM via
      OverlayTreePaths+CommitTree. DO NOT WIRE (hook-free ⇒ §20.2 fidelity).
    - `runSingleEscape` — delegates to `generate.CommitStaged` (covered by P1.M3.T2.S1; do not double-run).

  ⚠️ **#1 — THE import-graph call: decompose imports `internal/hooks` DIRECTLY (NOT the interface/adapter
      approach generate uses).** `hooks/runner.go` imports `internal/generate` (for RescueError), so
      `generate.go` CANNOT import hooks — THAT is why S1 injected `CommitHookRunner` into `generate.Deps`.
      But `internal/generate` imports NEITHER hooks NOR decompose (verified — only comment-text matches in
      generate.go). So the graph is hooks→generate, decompose→generate, and **decompose→hooks is ACYCLIC**.
      Therefore decompose does `import ".../internal/hooks"` and calls the CONCRETE
      `hooks.RunCommitHooks`/`hooks.RunPostCommit` with `hooks.HookOpts{DryRun:false, Verbose:deps.Verbose}`
      — the contract's literal form. NO `Deps.Hooks` field, NO `adapter.go`, NO interface. Do NOT copy S1's
      interface pattern (it exists ONLY to break the generate↔hooks cycle, which decompose does not have). (§1)

  ⚠️ **#2 — THE headline invariant: `resolveMidChain` stays HOOK-FREE (§20.2 mid-chain fidelity).**
      resolveMidChain reuses msg[j] VERBATIM. Running a foreign prepare-commit-msg would mangle it, breaking
      the "rebuilt non-target commits are byte-identical" invariant. resolveMidChain does its OWN
      CommitTree+UpdateRefCAS (doesn't use publishCommit) → naturally hook-free → DO NOT add the runner
      there. resolveNewCommit + resolveTipAmend DO run hooks. (open_questions §2; contract point 4.) (§2)

  ⚠️ **#3 — publishCommit wiring: RunCommitHooks BEFORE CommitTree (use finalTree/finalMsg); RunPostCommit
      AFTER UpdateRefCAS success (best-effort, discard).** On herr → return ("",herr) — the *RescueError
      propagates (the runLoop's FR-M12 handling already maps *generate.RescueError). Per-concept scoping is
      AUTOMATIC: every publishCommit caller passes THAT concept's tree[i], so pre-commit sees only that
      subset (FR-V8c, via the runner's throwaway index from that tree). No caller edit. (§3)

  ⚠️ **#4 — resolveNewCommit + resolveTipAmend: the SAME 2 inserts at each site's CommitTree/UpdateRefCAS,
      preserving each site's CAS expected-old + ReadTree sync.** resolveNewCommit: hookParent=tipSHA,
      expectedOld=tipSHA. resolveTipAmend: hookParent=tipParent, expectedOld=tipSHA (NOT tipParent — that's
      why they don't route through publishCommit). On herr → return herr (propagate *RescueError DIRECTLY).
      (§4)

  ⚠️ **#5 — resolveTipAmend runs hooks on the verbatim tip message (`git commit --amend` parity).** The
      amended tip's message is the hook INPUT; prepare-commit-msg MAY annotate it. This is correct git
      parity (amend re-runs msg hooks). §20.2 constrains ONLY resolveMidChain's non-target commits; the tip
      is the arbiter's target, so its message may change. (§5)

  ⚠️ **#6 — No Deps.Hooks field; existing tests stay GREEN (absent-hook no-op).** decompose calls hooks.*
      directly (always). On a no-hook repo (the existing decompose/chain tests), the runner no-ops
      (hookExecutable false → returns tree/msg unchanged). The runner's HooksPath/GitDir/TopLevel resolve
      cleanly on the real temp repos those tests use. No nil guard (unlike generate's Deps.Hooks). (§6)

  Deliverable: EDIT internal/decompose/message.go (publishCommit: 2 hook inserts + internal/hooks import) +
  internal/decompose/chain.go (resolveNewCommit + resolveTipAmend: 2 hook inserts each + internal/hooks
  import; resolveMidChain UNCHANGED) + internal/decompose/message_test.go + chain_test.go (hook tests incl.
  the §20.2 mid-chain-fidelity test). NO edit to decompose.go (callers), roles.go (Deps), hooks/*, generate/*,
  git/*, cmd/*, pkg/*, docs/*. NO new dependency (internal/hooks is an internal package; go.mod unchanged).

---

## Goal

**Feature Goal**: Thread P1.M3.T1's commit-hooks runner into the decompose multi-commit path so EVERY
per-concept commit (the main loop + the one-file/single shortcuts via `publishCommit`), the arbiter's N+1
commit (`resolveNewCommit`), and the arbiter's tip amend (`resolveTipAmend`) run the repo's
pre→prepare→commit-msg hooks scoped to that commit's frozen tree (FR-V1/V3/V8c), with a hook abort mapped
to the existing rescue (FR-V7) and post-commit fired best-effort — while the arbiter's mid-chain rebuild
(`resolveMidChain`) stays HOOK-FREE, preserving the §20.2 invariant that rebuilt non-target commits are
byte-identical (msg[j] verbatim). This closes the decompose half of the commit-path-hooks feature
(P1.M3.T2 closed the single-commit half: CommitStaged + runPipeline).

**Deliverable**:
1. **MODIFIED `internal/decompose/message.go`** — `publishCommit`: INSERT A (nil-free
   `hooks.RunCommitHooks` after the `parents` slice, before `CommitTree`; on `herr` → `return "", herr`;
   success → use `finalTree`/`finalMsg` in `CommitTree`) + INSERT B (`hooks.RunPostCommit` after the
   `UpdateRefCAS` success block, before `return newSHA, nil`; return discarded) + add `internal/hooks` to
   the import block.
2. **MODIFIED `internal/decompose/chain.go`** — `resolveNewCommit` + `resolveTipAmend`: the SAME 2 inserts
   at each site's `CommitTree`/`UpdateRefCAS` (hookParent = tipSHA / tipParent respectively; on `herr` →
   `return herr`; success → `finalTree`/`finalMsg` in `CommitTree`; `RunPostCommit` before `ReadTree(tStart)`)
   + add `internal/hooks` to the import block. **`resolveMidChain` UNCHANGED** (hook-free).
3. **MODIFIED `internal/decompose/message_test.go` + `chain_test.go`** — hook tests: publishCommit
   prepare-commit-msg annotates + pre-commit abort→*RescueError; resolveArbiter null/tip run hooks +
   mid-chain skips them (the §20.2 fidelity test).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; a per-concept
`publishCommit` with a `prepare-commit-msg` that appends lands a commit whose message carries the append
(hooks ran); a `publishCommit` with a `pre-commit` exit 1 returns `*generate.RescueError` with HEAD
unchanged (FR-V7); `resolveNewCommit` + `resolveTipAmend` likewise run hooks; `resolveMidChain` (mid-chain
arbiter) rebuilds commits whose messages carry NO append (msg[j] verbatim — §20.2 fidelity); the existing
decompose/chain tests (no hooks installed → runner no-ops) stay GREEN; go.mod/go.sum + decompose.go +
roles.go + hooks/* + generate/* byte-unchanged.

## User Persona

**Target User**: A user who runs `stagecoach` (decompose mode — nothing staged, multiple concepts) in a repo
with commit hooks (husky, lint-staged, a conventional-commit `commit-msg`, a `prepare-commit-msg` that
appends a ticket ref). Before this task, decompose's plumbing path (`commit-tree` + `update-ref`) ran NO
hooks — so each per-concept commit silently bypassed the user's `pre-commit` formatter, `commit-msg` lint,
and `prepare-commit-msg` annotation. After this task, every decompose commit (per-concept + arbiter) runs
the hooks scoped to that concept's tree, and the mid-chain rebuild stays deterministic (hooks don't mangle
the verbatim-reused messages). Transitively: P1.M4.T1 (the docs Mode B rewrite documenting the decompose
hook composition).

**Use Case**: A user with a `prepare-commit-msg` hook that appends `[PROJ-123]` runs `stagecoach` on a
3-concept working tree. Each per-concept commit's message gets `[PROJ-123]` appended (scoped — `pre-commit`
sees only that concept's files). If the arbiter amends the tip, the tip's message is re-annotated (`git
commit --amend` parity). If the arbiter rebuilds mid-chain, the non-target commits keep their ORIGINAL
messages verbatim (the rebuild is a deterministic reconstruction, not a re-commit). A `pre-commit` that
exits 1 aborts the run with the rescue recipe (FR-V7) and leaves HEAD + the index untouched.

**User Journey**: `stagecoach` (decompose) → planner → per-concept stager → `publishCommit(tree[i], …)` →
**RunCommitHooks(tree[i], …)** [pre-commit scoped to tree[i]; prepare/commit-msg on the message] →
`CommitTree(finalTree, …)` → `UpdateRefCAS` → `RunPostCommit` → next concept. Arbiter (if leftovers):
`resolveNewCommit`/`resolveTipAmend` (run hooks) or `resolveMidChain` (hook-free).

**Pain Points Addressed**: (1) Decompose commits bypassing the user's hooks — the §9.25 gap for the
multi-commit path. (2) Hooks seeing the WHOLE accumulated index instead of one concept's subset — solved
by the throwaway-index scoping (FR-V3/V8c, runner.go). (3) The mid-chain rebuild mangling verbatim messages
if hooks ran — solved by resolveMidChain staying hook-free (§20.2). (4) A hook abort leaving a partial
chain — solved by FR-V7 rescue (HEAD/index untouched; already-published commits stand).

## Why

- **Closes FR-V1/V8c for the decompose path.** FR-V1: "every commit produced by the plumbing path … runs
  the repo's commit hooks." FR-V8c: "the full hook sequence runs around each per-concept commit, its
  pre-commit seeing only that concept's staged subset." P1.M3.T2 closed the single-commit path; this closes
  the multi-commit path (the last chokepoint family).
- **Satisfies PRD §9.25 (FR-V1/V3/V7/V8c) + §13.6.5 + §20.2.** The §20.2 "mid-chain amend fidelity"
  invariant is the load-bearing acceptance criterion (contract point 4): non-target commits byte-identical
  after a mid-chain rebuild — guaranteed by resolveMidChain running NO hooks.
- **Faithful to open_questions.md §2's confirmed resolution.** "wire the hook runner into publishCommit …
  and simply do NOT wire it into resolveMidChain. Confirm resolveNewCommit/resolveTipAmend route their
  commits through the hook-bearing path."
- **Acyclic, minimal-surface wiring.** Decompose imports `internal/hooks` directly (no interface/adapter —
  the generate↔hooks cycle doesn't apply). Only 2 production files + their tests change; the callers
  (decompose.go) are untouched (publishCommit now runs hooks internally).

## What

EDIT `message.go` (publishCommit) + `chain.go` (resolveNewCommit + resolveTipAmend) — 2 hook inserts each
+ the `internal/hooks` import. `resolveMidChain` UNCHANGED. Plus hook tests. No new types, no new Deps
fields, no interface, no adapter, no caller edits, no docs.

### Success Criteria

- [ ] `message.go` `publishCommit` has INSERT A: `hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree,
      parentSHA, msg, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})` after the `parents` slice,
      before `deps.Git.CommitTree`; on `herr != nil` → `return "", herr`; success → `CommitTree` uses
      `finalTree`/`finalMsg`.
- [ ] `message.go` `publishCommit` has INSERT B: `hooks.RunPostCommit(ctx, deps.Git, deps.Config,
      hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})` (return discarded) after the `UpdateRefCAS`
      success block, before `return newSHA, nil`.
- [ ] `chain.go` `resolveNewCommit` has the same 2 inserts (hookParent = `tipSHA`; INSERT B before
      `ReadTree(tStart)`); `resolveTipAmend` has the same 2 inserts (hookParent = `tipParent`; INSERT B
      before `ReadTree(tStart)`). On `herr` → `return herr`. Each site's CAS expected-old + `handleUpdateRefErr`
      are UNCHANGED.
- [ ] `chain.go` `resolveMidChain` is byte-unchanged (NO hooks — §20.2 fidelity).
- [ ] `message.go` + `chain.go` import blocks add `"github.com/dustin/stagecoach/internal/hooks"`.
- [ ] `message_test.go` adds: a publishCommit test with a `prepare-commit-msg` that appends → the landed
      commit's message carries the append; a publishCommit test with a `pre-commit` exit 1 →
      `*generate.RescueError` + HEAD unchanged.
- [ ] `chain_test.go` adds: `TestResolveArbiter_NullNewCommit_RunsHooks`, `…_TipAmend_RunsHooks`,
      `…_MidChain_SkipsHooks` (THE §20.2 fidelity test — mid-chain rebuilt commits carry NO append).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on the edited files.
- [ ] go.mod/go.sum byte-unchanged; decompose.go + roles.go + hooks/* + generate/* + git/* + cmd/* + pkg/* +
      docs/* byte-unchanged; the existing decompose/chain tests (no hooks → runner no-ops) GREEN.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact insertion points
(the current publishCommit/resolveNewCommit/resolveTipAmend code quoted verbatim in the Blueprint with the
insert markers), the import-graph decision (decompose→hooks acyclic — direct call, no interface), the
frozen runner API (signatures + HookOpts quoted), the mid-chain-fidelity invariant (resolveMidChain
untouched), the existing test scaffolding (chain_test.go's TestResolveArbiter_* + hooks_freeze_test.go's
hook-install pattern), and the copy-ready hook-insert code. No generate/CommitStaged/signal/registry
knowledge required beyond "publishCommit is the chokepoint".

### Documentation & References

```yaml
# MUST READ — the design calls (the import-graph decision, the mid-chain invariant, per-site wiring)
- docfile: plan/010_49117f1f30ab/P1M3T3S1/research/design-decisions.md
  why: the 9 decisions — scope (§0), THE import-graph call: decompose imports hooks DIRECTLY, no interface
       (§1), THE headline invariant: resolveMidChain hook-free (§2), publishCommit wiring (§3),
       resolveNewCommit+resolveTipAmend wiring (§4), resolveTipAmend amend parity (§5), no Deps.Hooks +
       absent-hook no-op (§6), HookOpts (§7), tests (§8), frozen files (§9).
  critical: §1 (direct hooks import — do NOT copy S1's interface/adapter; decompose has no cycle), §2
       (resolveMidChain UNCHANGED — the §20.2 fidelity invariant), §4 (the per-site CAS expected-old +
       hookParent values) are the things most likely to go wrong.

# MUST READ — the chokepoint map (confirms WHICH sites to wire + which to skip)
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "## 5. The commit chokepoints" — `decompose.publishCommit` (message.go:219) is the per-concept
           chokepoint (main loop :484, runSingleShortcut :390, runOneFileShortcut :336); resolveNewCommit
           + resolveTipAmend produce user-facing commits (WIRE); resolveMidChain is the silent deterministic
           rebuild reusing msg[j] verbatim (MUST SKIP hooks — open Q#2); runSingleEscape → generate.CommitStaged
           (covered by M3.T2.S1).
  critical: resolveMidChain does its OWN CommitTree+UpdateRefCAS (chain.go:203/215) — it does NOT go through
       publishCommit, so skipping hooks is natural (just don't wire it). §5 confirms the chokepoint list.

# MUST READ — open_questions §2 (the confirmed mid-chain resolution)
- docfile: plan/010_49117f1f30ab/architecture/open_questions.md
  section: "## 2. Arbiter mid-chain rebuild hooks — RESOLVED: SKIP hooks" — resolveMidChain reuses msg[j]
           VERBATIM; running a foreign prepare-commit-msg would break the §20.2 "mid-chain amend fidelity"
           invariant. resolveNewCommit + resolveTipAmend DO run hooks. "wire the hook runner into
           publishCommit … and simply do NOT wire it into resolveMidChain."
  critical: this IS the spec for the mid-chain decision. Do not second-guess it.

# MUST READ — the runner's FROZEN API (consume, do NOT edit)
- file: internal/hooks/runner.go   (P1.M3.T1.S1/S2 COMPLETE — read for the signatures; do NOT edit)
  section: `func RunCommitHooks(ctx, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
           opts HookOpts) (finalTree, finalMsg string, err error)` — runs pre→prepare→commit-msg scoped to
           snapshotTree (throwaway index); returns the possibly-re-treed finalTree + possibly-annotated
           finalMsg; on hook non-zero/timeout returns ("","",*generate.RescueError{…}); on absent hooks
           no-ops (returns snapshotTree, msg, nil). `func RunPostCommit(ctx, g, cfg, opts HookOpts) error`
           — best-effort after update-ref; ALWAYS returns nil (exit disregarded, FR-V7); self-skips under
           DryRun. `type HookOpts struct { DryRun bool; Verbose *ui.Verbose }`. The runner honors cfg.NoVerify
           (FR-V5) + cfg.HookTimeout (FR-V6); skips absent/non-exec hooks (hookExecutable).
  why: the EXACT signatures + HookOpts to call. Confirms the absent-hook no-op (§6) that keeps existing
       tests GREEN, and that the runner owns the throwaway-index scoping (FR-V3/V8c — the caller just
       passes the tree).
  critical: do NOT edit runner.go. Use hooks.HookOpts{DryRun: false, Verbose: deps.Verbose} (NOT the
       generate.CommitHookRunner interface — that's generate-specific, §1).

# THE FILES BEING EDITED — READ fully before editing (the insert points)
- file: internal/decompose/message.go   (EDIT — publishCommit)
  section: `func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)`
           (~L219). The flow: build `parents` (nil if parentSHA=="") → INSERT A HERE →
           `deps.Git.CommitTree(ctx, tree, parents, msg)` (use finalTree/finalMsg) → compute expectedOld
           (parentSHA or 40-zeros) → `deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)` (+ its CAS
           block returning *generate.CASError) → INSERT B HERE → `return newSHA, nil`.
  why: the EXACT code this task edits. Confirms `tree`/`parentSHA`/`msg`/`deps` are in scope at INSERT A;
       the CAS block is the success/fail fork for INSERT B placement.
  critical: INSERT A goes BETWEEN the `parents` build and `CommitTree`; INSERT B goes BETWEEN the
       UpdateRefCAS block's close and `return newSHA, nil`. Do NOT move the CAS handling.

- file: internal/decompose/chain.go   (EDIT — resolveNewCommit + resolveTipAmend; resolveMidChain UNCHANGED)
  section: `resolveNewCommit` — after `msg, err := generateMessage(…)` + the `parents` build, BEFORE
           `CommitTree(treePrime, parents, msg)`: INSERT A (hookParent=tipSHA). After `UpdateRefCAS` success,
           BEFORE `ReadTree(tStart)`: INSERT B. `resolveTipAmend` — after the `tipMsg`/`parents` setup, BEFORE
           `CommitTree(treePrime, parents, tipMsg)`: INSERT A (hookParent=tipParent). After `UpdateRefCAS`
           success, BEFORE `ReadTree(tStart)`: INSERT B. `resolveMidChain` — UNCHANGED (the loop's
           `CommitTree(treePrime, parents, chainData[j].Message)` + `UpdateRefCAS` + `ReadTree` stay as-is).
  why: the EXACT code. Confirms each site's CAS expected-old (resolveNewCommit: tipSHA-or-zeros;
       resolveTipAmend: tipSHA) and that they use `handleUpdateRefErr` (preserve it). Confirms
       resolveMidChain is structurally separate (its own loop) — easy to leave untouched.
  critical: resolveTipAmend's hookParent is tipParent (the amend's parent), but its CAS expected-old is
       tipSHA (CURRENT HEAD) — do NOT confuse the two. resolveMidChain: do NOT touch.

# THE TEST STYLE TO MIRROR — resolveArbiter direct-call + hook install
- file: internal/decompose/chain_test.go   (EDIT — +3 hook tests; mirror the existing TestResolveArbiter_*)
  section: `TestResolveArbiter_NullNewCommit` (L156), `…_TipAmend` (L205), `…_MidChainRebuild` (L268) — they
           build `commits []CommitInfo` + `chainData []ChainEntry` + `tStart` + `leftoverPaths`, construct
           `Deps` (stubtest.Build + stubtest.Manifest + Roles), and call `resolveArbiter(ctx, deps, target,
           commits, chainData, tStart, leftoverPaths)` directly. The MidChainRebuild test asserts "Subjects
           should be preserved verbatim" (L333) — the baseline for the §20.2 hook test.
  why: the test STYLE — direct resolveArbiter calls (no full Decompose run needed for the arbiter paths).
       The tip/mid tests set `Out: "SHOULD NOT BE USED"` (the agent is never called — msg reused verbatim).
       Mirror this; add a hook install + assert annotation presence (null/tip) / absence (mid).
  critical: do NOT alter the existing TestResolveArbiter_* tests (no hook → runner no-ops → they stay GREEN).
       ADD new *_Hooks tests alongside them.

# THE HOOK-INSTALL PATTERN TO MIRROR
- file: internal/generate/hooks_freeze_test.go   (read for the hook-install helper; do NOT edit)
  section: `os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookBody), 0o755)` (L91) — writes an
           executable hook script to `<repo>/.git/hooks/<name>`, mode 0755 (owner-exec bit ⇒ hookExecutable
           true). The runner discovers hooks via `git rev-parse --git-path hooks` = `.git/hooks` for a normal init.
  why: the hook-install idiom. Mirror it (a small `installHook(t, repo, name, body)` helper in chain_test.go
       / message_test.go, or inline). The prepare-commit-msg body: `#!/bin/sh\necho '[ANNOT]' >> "$1"\n` (appends
       a marker to the message file $1). The pre-commit reject body: `#!/bin/sh\nexit 1\n`.
  critical: mode 0755 (else hookExecutable false → hook skipped → test is vacuous). The prepare-commit-msg
       appends to "$1" (the message file path, arg 1).

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/010_…/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" FR-V1 (every plumbing-path commit runs hooks), FR-V3
           (pre-commit scoped to the snapshot tree), FR-V7 (hook failure is a rescue), FR-V8c ("Decompose:
           the full hook sequence runs around each per-concept commit, its pre-commit seeing only that
           concept's staged subset"). "20.2 Property/invariant tests" — "Mid-chain amend fidelity (v2):
           after an arbiter-driven mid-chain rebuild, the rebuilt chain's non-target commits are byte-
           identical (same tree, same message) to the originals."
  critical: FR-V8c is the per-concept scoping spec; §20.2 mid-chain fidelity is the acceptance invariant.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # main loop + shortcuts (CALL publishCommit). UNCHANGED (no caller edit — publishCommit runs hooks internally).
  message.go          # EDIT — publishCommit: INSERT A (RunCommitHooks) + INSERT B (RunPostCommit) + internal/hooks import.
  chain.go            # EDIT — resolveNewCommit + resolveTipAmend: INSERT A+B each + internal/hooks import. resolveMidChain UNCHANGED.
  roles.go            # Deps struct (Git/Config/Verbose). UNCHANGED (NO Hooks field — §1).
  arbiter.go/planner.go/stager.go  # UNCHANGED.
  message_test.go     # EDIT — +publishCommit hook tests.
  chain_test.go       # EDIT — +3 resolveArbiter hook tests (null/tip run; mid skips).
internal/hooks/
  runner.go           # P1.M3.T1 COMPLETE (RunCommitHooks/RunPostCommit/HookOpts). UNCHANGED (consumed).
  adapter.go/subset.go # UNCHANGED.
internal/generate/generate.go  # P1.M3.T2.S1 (CommitStaged hooks — the single-commit path). UNCHANGED.
go.mod / go.sum       # UNCHANGED (internal/hooks is internal; no new module).
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place: message.go (publishCommit) + chain.go (resolveNewCommit/resolveTipAmend)
# get 2 hook inserts each + the internal/hooks import; message_test.go + chain_test.go get hook tests.
# resolveMidChain (chain.go) is byte-unchanged.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — decompose imports internal/hooks DIRECTLY, NOT via an interface): the generate↔hooks cycle
//   (hooks→generate for RescueError) does NOT extend to decompose — generate imports NEITHER hooks NOR
//   decompose (verified). So decompose→hooks is ACYCLIC. Call hooks.RunCommitHooks/RunPostCommit with
//   hooks.HookOpts{DryRun:false, Verbose:deps.Verbose}. NO Deps.Hooks field, NO adapter.go, NO interface.
//   Do NOT copy S1's CommitHookRunner pattern (it exists ONLY to break the generate cycle). (§1)

// CRITICAL (#2 — resolveMidChain stays HOOK-FREE; §20.2 fidelity): resolveMidChain reuses msg[j] VERBATIM.
//   Running prepare-commit-msg would mangle it, breaking "non-target commits byte-identical". resolveMidChain
//   does its own CommitTree+UpdateRefCAS (not via publishCommit) → naturally hook-free → DO NOT wire. (§2)

// CRITICAL (#3 — per-site CAS expected-old + hookParent; do NOT confuse them): resolveTipAmend commits
//   against parent=tipParent BUT its CAS expected-old=tipSHA (CURRENT HEAD). hookParent (the RunCommitHooks
//   parentSHA, for rescue context) = tipParent (the amend's parent). resolveNewCommit: hookParent=tipSHA,
//   expectedOld=tipSHA. publishCommit: hookParent=parentSHA, expectedOld=parentSHA. Preserve each site's
//   existing CAS handling (handleUpdateRefErr / the inline CASError). (§4)

// CRITICAL (#4 — on herr, return the *RescueError DIRECTLY): publishCommit returns ("",herr); resolveNewCommit/
//   resolveTipAmend return herr. The runLoop's FR-M12 handling + resolveArbiter already map *generate.RescueError
//   (propagate unwrapped — do NOT wrap in ErrPublicationFailed/ErrArbiterResolutionFailed). (§3/§4)

// GOTCHA (existing tests stay GREEN — absent-hook no-op): the runner's hookExecutable returns false for an
//   absent/non-exec hook → runPreCommitScoped/runPrepareCommitMsg/runCommitMsg skip → RunCommitHooks returns
//   (tree, msg, nil) unchanged. So the existing decompose/chain tests (no hooks) are byte-identical. No nil
//   guard needed (unlike generate's Deps.Hooks). (§6)

// GOTCHA (RunCommitHooks resolves HooksPath/GitDir/TopLevel at entry): these work on the real temp git repos
//   the tests use (git init'd). On a resolution error RunCommitHooks returns ("","",err) — would surface as
//   a commit failure. The real-repo tests do not hit this. (§6)

// GOTCHA (HookOpts{DryRun:false, Verbose:deps.Verbose}): decompose always commits (no dry-run path). DryRun:false
//   ⇒ pre-commit RUNS (not skipped), commit-msg RUNS, post-commit RUNS (the full FR-V1 sequence). cfg.NoVerify
//   (FR-V5) + cfg.HookTimeout (FR-V6) come from deps.Config. (§7)

// GOTCHA (per-concept scoping is automatic — FR-V8c): every publishCommit caller passes THAT concept's tree[i].
//   The runner materializes a throwaway index from that tree (runPreCommitScoped → ReadTreeInto) so pre-commit
//   sees only that subset. No caller edit. (§3)

// GOTCHA (resolveTipAmend amend parity): the tip message is the hook INPUT; prepare-commit-msg MAY annotate it
//   (mirrors `git commit --amend` re-running msg hooks). §20.2 constrains ONLY resolveMidChain's non-target
//   commits; the tip is the target. Do NOT suppress hooks on resolveTipAmend. (§5)

// GOTCHA (import addition): message.go adds internal/hooks to (config/generate/git/prompt/provider); chain.go
//   adds internal/hooks to (generate/git). go mod tidy is a no-op (internal package). (§1)
```

## Implementation Blueprint

### Data models and structure

```go
// NO new types, NO new Deps fields, NO interface. Two hook inserts per commit site + the internal/hooks import.
// (hooks.RunCommitHooks/RunPostCommit/HookOpts are FROZEN in internal/hooks/runner.go — consumed read-only.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/message.go — publishCommit: INSERT A + INSERT B + import
  - ADD "github.com/dustin/stagecoach/internal/hooks" to the import block.
  - INSERT A (between the `parents` build and `deps.Git.CommitTree`):
      finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
          hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
      if herr != nil {
          return "", herr // *generate.RescueError (FR-V7) — propagates; runLoop FR-M12 maps it
      }
    Then change CommitTree to use (finalTree, parents, finalMsg).
  - INSERT B (after the UpdateRefCAS block's success path, before `return newSHA, nil`):
      _ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
  - GOTCHA: use finalTree/finalMsg in CommitTree (NOT the original tree/msg). INSERT B's return is discarded.

Task 2: EDIT internal/decompose/chain.go — resolveNewCommit + resolveTipAmend: INSERT A+B each; import
  - ADD "github.com/dustin/stagecoach/internal/hooks" to the import block.
  - resolveNewCommit INSERT A (before its `CommitTree(treePrime, parents, msg)`):
      finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipSHA, msg,
          hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
      if herr != nil { return herr }
    CommitTree uses (finalTree, parents, finalMsg). INSERT B (after UpdateRefCAS success, before ReadTree(tStart)):
      _ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
  - resolveTipAmend INSERT A (before its `CommitTree(treePrime, parents, tipMsg)`):
      finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipParent, tipMsg,
          hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
      if herr != nil { return herr }
    CommitTree uses (finalTree, parents, finalMsg). INSERT B (after UpdateRefCAS success, before ReadTree(tStart)):
      _ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
  - resolveMidChain: UNCHANGED (do NOT touch its loop / CommitTree / UpdateRefCAS / ReadTree).
  - GOTCHA: resolveTipAmend hookParent=tipParent, but its CAS expected-old stays tipSHA (preserve handleUpdateRefErr).
    On herr → return herr (propagate *RescueError DIRECTLY — do NOT wrap).

Task 3: EDIT internal/decompose/message_test.go + chain_test.go — hook tests
  - ADD a small installHook(t, repo, name, body) helper (writes <repo>/.git/hooks/<name>, mode 0755) — mirror
    hooks_freeze_test.go L91.
  - message_test.go: TestPublishCommit_PrepareCommitMsgAnnotates (install prepare-commit-msg that appends a
    marker to "$1"; call publishCommit; assert `git log -1 --format=%B` carries the marker). TestPublishCommit_
    PreCommitAbort_RescueError (install pre-commit exit 1; call publishCommit; assert errors.As(err, &re
    *generate.RescueError) + HEAD unchanged).
  - chain_test.go: TestResolveArbiter_NullNewCommit_RunsHooks (prepare-commit-msg appends; resolveArbiter(nil,…);
    assert the N+1 commit's message carries the marker). TestResolveArbiter_TipAmend_RunsHooks (appends;
    resolveArbiter(&tipSHA,…); assert the amended tip carries the marker). TestResolveArbiter_MidChain_SkipsHooks
    (THE §20.2 fidelity test — appends; resolveArbiter(&sha[i],…) i<N-1; assert the rebuilt commits carry NO
    marker — msg[j] verbatim).
  - GOTCHA: mirror the existing TestResolveArbiter_* setup (build commits/chainData/tStart/leftoverPaths;
    stubtest.Build + stubtest.Manifest with Out:"SHOULD NOT BE USED" for tip/mid — the agent isn't called).
    Hook mode 0755 (else hookExecutable false → vacuous). Do NOT add t.Parallel (hook tests mutate repo state).

Task 4: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. decompose.go + roles.go + hooks/*
    + generate/* + git/* + cmd/* + pkg/* byte-unchanged. The existing decompose/chain tests (no hooks →
    runner no-ops) GREEN. `go build/vet/test ./...` green.
```

### Implementation Patterns & Key Details

```go
// === message.go publishCommit — INSERT A (before CommitTree) + INSERT B (before return) ===
func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error) {
	var parents []string
	if parentSHA != "" {
		parents = []string{parentSHA}
	}
	// ---- INSERT A: commit hooks (PRD §9.25 FR-V1/V3/V7/V8c). Scoped to THIS concept's tree (the caller
	// passes tree[i]/tStart); pre-commit runs against a throwaway index materialized from it (FR-V3). A hook
	// abort returns *RescueError (FR-V7) — propagates to the runLoop's FR-M12 handling. DryRun:false (decompose
	// has no dry-run path). ----
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return "", herr
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg) // hook-adjusted tree + message
	if err != nil {
		return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err)
	}
	expectedOld := parentSHA
	if parentSHA == "" {
		expectedOld = strings.Repeat("0", 40)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		// … existing CAS handling (re-read actual; return *generate.CASError) UNCHANGED …
	}
	// ---- INSERT B: post-commit (FR-V7 best-effort; exit disregarded — the commit already landed). ----
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	return newSHA, nil
}
```

```go
// === chain.go resolveNewCommit — INSERT A (hookParent=tipSHA) + INSERT B (before ReadTree) ===
	// … after generateMessage + the parents build …
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipSHA, msg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return herr // *generate.RescueError — propagate DIRECTLY (matches generateMessage)
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
	// … existing expectedOld=tipSHA-or-zeros + UpdateRefCAS + handleUpdateRefErr UNCHANGED …
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if err := deps.Git.ReadTree(ctx, tStart); err != nil { … } // existing sync — UNCHANGED

// === chain.go resolveTipAmend — INSERT A (hookParent=tipParent) + INSERT B (before ReadTree) ===
	// … after tipMsg/tipParent + the parents build …
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipParent, tipMsg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return herr
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
	// … existing expectedOld=tipSHA (CURRENT HEAD, NOT tipParent) + UpdateRefCAS + handleUpdateRefErr UNCHANGED …
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if err := deps.Git.ReadTree(ctx, tStart); err != nil { … } // existing sync — UNCHANGED

// === chain.go resolveMidChain — UNCHANGED (NO hooks; msg[j] verbatim; §20.2 fidelity) ===
	for j := i; j < N; j++ {
		treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)
		// … CommitTree(treePrime, parents, chainData[j].Message) — msg[j] VERBATIM, NO hooks …
	}
```

```go
// === chain_test.go — the §20.2 mid-chain-fidelity test (the load-bearing acceptance test) ===
func TestResolveArbiter_MidChain_SkipsHooks(t *testing.T) {
	bin := stubtest.Build(t)
	// … build commits/chainData (N≥3), tStart, leftoverPaths exactly like TestResolveArbiter_MidChainRebuild …
	// install a prepare-commit-msg that APPENDS a marker to the message file:
	installHook(t, repoDir, "prepare-commit-msg", "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")
	target := chainData[1].SHA // i=1 < N-1 ⇒ mid-chain
	deps := Deps{ /* … Roles: stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"}) … */ }
	if err := resolveArbiter(context.Background(), deps, &target, commits, chainData, tStart, leftoverPaths); err != nil {
		t.Fatalf("resolveArbiter(mid-chain): %v", err)
	}
	// Assert the rebuilt commits' messages carry NO '[HOOK-RAN]' (msg[j] verbatim — resolveMidChain is hook-free).
	for j := 1; j < len(chainData); j++ {
		msg := gitOut(t, repoDir, "log", "-1", "--format=%B", rebuiltSHA[j])
		if strings.Contains(msg, "[HOOK-RAN]") {
			t.Errorf("rebuilt commit[%d] message carries the hook marker — resolveMidChain must be hook-free (§20.2): %q", j, msg)
		}
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. internal/hooks is an internal package (already in the module);
      decompose adding the import adds NO module. `go mod tidy` is a no-op.

PACKAGE EDGES: ONE new edge — internal/decompose → internal/hooks (ACYCLIC: hooks→generate;
      generate imports neither hooks nor decompose; verified). NO interface, NO adapter (unlike generate's
      CommitHookRunner — that exists ONLY to break the generate↔hooks cycle, which decompose does not have).

UPSTREAM CONTRACT (consume, do NOT edit):
  - hooks.RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts) (finalTree, finalMsg, err) +
        hooks.RunPostCommit(ctx, g, cfg, HookOpts) error (runner.go, P1.M3.T1.S1/S2 COMPLETE). Honors
        cfg.NoVerify (FR-V5) + cfg.HookTimeout (FR-V6); skips absent hooks; *RescueError on hook non-zero/timeout.
  - deps.Git / deps.Config / deps.Verbose (roles.go Deps — present, no edit).

DOWNSTREAM CONTRACTS: none new. The per-concept scoping (FR-V8c) + mid-chain fidelity (§20.2) are acceptance
      invariants, not new symbols. hooks.RunCommitHooks/RunPostCommit/HookOpts are FROZEN (P1.M3.T1).

FROZEN/LEAVE (do NOT edit):
  - internal/decompose/decompose.go (callers — publishCommit runs hooks internally), roles.go (Deps — NO Hooks
        field), arbiter.go, planner.go, stager.go.
  - internal/hooks/* (runner.go/adapter.go/subset.go — P1.M3.T1/M1.T2), internal/generate/* (P1.M3.T2.S1),
        internal/git/*, internal/cmd/*, pkg/stagecoach/*, internal/signal/*, internal/hook/*, internal/lock/*, docs/*.
  - PRD.md, go.mod, Makefile. resolveMidChain (chain.go) — byte-unchanged. The existing decompose/chain tests.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/decompose/message.go internal/decompose/chain.go internal/decompose/message_test.go internal/decompose/chain_test.go
go vet ./internal/decompose/
go build ./...
# Confirm the inserts + the import:
grep -n 'hooks.RunCommitHooks\|hooks.RunPostCommit\|"github.com/dustin/stagecoach/internal/hooks"' internal/decompose/message.go internal/decompose/chain.go
# Confirm resolveMidChain has NO hooks.RunCommitHooks (the §20.2 invariant):
! grep -n 'hooks.RunCommitHooks' <(sed -n '/func resolveMidChain/,/^func /p' internal/decompose/chain.go) && echo "resolveMidChain hook-free (expected §20.2)" || echo "FAIL: resolveMidChain wired to hooks"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; build clean; publishCommit + resolveNewCommit + resolveTipAmend have the inserts;
# resolveMidChain has none; the internal/hooks import added to both files; go.mod/go.sum byte-unchanged.
```

### Level 2: The hook tests + the existing decompose/chain suites (no-regression)

```bash
go test ./internal/decompose/ -v -run 'TestPublishCommit_PrepareCommitMsgAnnotates|TestPublishCommit_PreCommitAbort|TestResolveArbiter_NullNewCommit_RunsHooks|TestResolveArbiter_TipAmend_RunsHooks|TestResolveArbiter_MidChain_SkipsHooks'
# Expected PASS — verify:
#   TestPublishCommit_PrepareCommitMsgAnnotates .... landed commit message carries the append (hooks ran ✓)
#   TestPublishCommit_PreCommitAbort_RescueError .... *generate.RescueError + HEAD unchanged (FR-V7 ✓)
#   TestResolveArbiter_NullNewCommit_RunsHooks ...... N+1 commit message carries the append (hooks ran ✓)
#   TestResolveArbiter_TipAmend_RunsHooks ........... amended tip message carries the append (§5 amend parity ✓)
#   TestResolveArbiter_MidChain_SkipsHooks .......... rebuilt commits carry NO append (msg[j] verbatim — §20.2 ✓)
# The existing decompose/chain tests (no hooks → runner no-ops) MUST stay GREEN:
go test ./internal/decompose/ -v -run 'TestResolveArbiter_NullNewCommit$|TestResolveArbiter_TipAmend$|TestResolveArbiter_MidChainRebuild|TestDecompose_'
go test ./internal/decompose/...   # the full decompose suite
# If MidChain_SkipsHooks FAILS (a rebuilt commit carries the marker), resolveMidChain was wired to hooks
# (§2 violated — remove the wire). If the existing *_MidChainRebuild FAILS, the runner isn't no-op'ing on the
# no-hook repo (check hookExecutable / that the test repo has no .git/hooks/prepare-commit-msg).
```

### Level 3: Whole-repo + frozen-file check

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS (decompose + hooks + generate + repo-wide). P1.M3.T1/T2 tests green.
# Confirm ONLY the target files changed:
git diff --name-only | grep -E 'internal/decompose/(message|chain)\.go|internal/decompose/(message|chain)_test\.go' && echo "(expected files)"
# Confirm the frozen files are byte-unchanged:
git diff --exit-code internal/decompose/decompose.go internal/decompose/roles.go internal/decompose/arbiter.go \
  internal/hooks internal/generate internal/git internal/cmd pkg/stagecoach internal/signal internal/hook internal/lock \
  docs PRD.md go.mod Makefile && echo "frozen files UNCHANGED (expected)"
# Confirm resolveMidChain is byte-unchanged:
git diff internal/decompose/chain.go | grep -E '^[+-].*(resolveMidChain|OverlayTreePaths)' && echo "WARN: resolveMidChain touched" || echo "resolveMidChain UNCHANGED (expected §20.2)"
# Expected: only message.go + chain.go + their _test.go modified; everything else byte-unchanged.
```

### Level 4: FR-V1/V8c + §20.2 fidelity correctness reasoning

```bash
# Verify by reasoning + the tests:
#   1. FR-V1 (every plumbing commit runs hooks): publishCommit (per-concept + shortcuts) + resolveNewCommit +
#      resolveTipAmend all run RunCommitHooks before CommitTree. (The 4 hook tests + the existing no-hook no-op.)
#   2. FR-V8c (per-concept scoping): each publishCommit caller passes tree[i]; the runner's throwaway index is
#      materialized from THAT tree → pre-commit sees only that concept's subset. (Automatic — no caller edit.)
#   3. FR-V7 (hook abort is a rescue): on herr, publishCommit returns ("",herr) / the arbiter sites return herr;
#      the *RescueError propagates to the runLoop's FR-M12 handling (HEAD + index untouched — no CommitTree ran).
#      (PreCommitAbort_RescueError asserts HEAD unchanged.)
#   4. §20.2 mid-chain fidelity: resolveMidChain is hook-free → msg[j] reused verbatim → rebuilt non-target
#      commits byte-identical. (MidChain_SkipsHooks asserts NO hook marker on rebuilt commits.)
#   5. Amend parity: resolveTipAmend runs hooks on the verbatim tip message (git commit --amend re-runs msg hooks);
#      §20.2 constrains ONLY resolveMidChain's non-target commits, not the tip (the target). (TipAmend_RunsHooks.)
#   6. No-hook no-op: existing tests have no hooks → hookExecutable false → RunCommitHooks returns (tree,msg,nil)
#      unchanged → byte-identical behavior. (Existing *_MidChainRebuild / TestDecompose_* GREEN.)
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (the new hook tests + the existing decompose/chain tests + hooks/generate/repo-wide).
- [ ] go.mod/go.sum byte-unchanged; decompose.go + roles.go + hooks/* + generate/* + git/* + cmd/* + pkg/* byte-unchanged.

### Feature Validation
- [ ] `publishCommit` has INSERT A (RunCommitHooks before CommitTree; uses finalTree/finalMsg) + INSERT B (RunPostCommit
      after UpdateRefCAS; discarded).
- [ ] `resolveNewCommit` (hookParent=tipSHA) + `resolveTipAmend` (hookParent=tipParent) have INSERT A+B; on herr →
      return herr; each site's CAS expected-old + handleUpdateRefErr + ReadTree sync preserved.
- [ ] `resolveMidChain` is byte-unchanged (hook-free — §20.2).
- [ ] A per-concept/arbiter commit with a `prepare-commit-msg` that appends lands a message carrying the append.
- [ ] A `pre-commit` exit 1 → `*generate.RescueError` + HEAD unchanged (FR-V7).
- [ ] The mid-chain rebuild's commits carry NO append (msg[j] verbatim — §20.2 fidelity).
- [ ] The existing decompose/chain tests (no hooks) stay GREEN.

### Code Quality Validation
- [ ] decompose imports `internal/hooks` DIRECTLY (no interface/adapter/Deps.Hooks — §1).
- [ ] HookOpts{DryRun:false, Verbose:deps.Verbose} at every call (decompose always commits).
- [ ] The per-site CAS expected-old is NOT changed by the hook wiring (resolveTipAmend: expectedOld=tipSHA, hookParent=tipParent).
- [ ] Anti-patterns avoided (see below); frozen files byte-unchanged; resolveMidChain byte-unchanged.

### Documentation
- [ ] INSERT A's comment cites PRD §9.25 FR-V1/V3/V7/V8c + the per-concept scoping rationale + the FR-V7 rescue
      mapping. The resolveTipAmend comment notes the amend parity (§5). (No docs/how-it-works.md change — the
      decompose composition is covered by M3.T2.S1's subsection per the item contract point 5.)

---

## Anti-Patterns to Avoid

- ❌ **Don't copy S1's interface/adapter/Deps.Hooks pattern into decompose.** The generate↔hooks cycle does
  NOT extend to decompose (generate imports neither hooks nor decompose). Decompose imports `internal/hooks`
  directly and calls the concrete functions. The interface exists ONLY to break the generate cycle. (§1)
- ❌ **Don't wire hooks into resolveMidChain.** It reuses msg[j] verbatim; hooks would mangle it, breaking
  the §20.2 mid-chain fidelity invariant. resolveMidChain does its own CommitTree+UpdateRefCAS (not via
  publishCommit) — leave it hook-free. (§2)
- ❌ **Don't wrap the hook *RescueError in ErrPublicationFailed/ErrArbiterResolutionFailed.** The runLoop's
  FR-M12 handling + resolveArbiter propagate *generate.RescueError unwrapped. On herr → return ("",herr) /
  return herr DIRECTLY. (§3/§4)
- ❌ **Don't confuse resolveTipAmend's hookParent (tipParent) with its CAS expected-old (tipSHA).** The amend
  commits against tipParent but HEAD is currently tipSHA — that's why it can't route through publishCommit
  (which uses the parent as expected-old). Preserve each site's existing CAS handling. (§4)
- ❌ **Don't use the original tree/msg in CommitTree after RunCommitHooks.** Use finalTree/finalMsg (the hook
  may have re-treed via pre-commit, or annotated the message via prepare-commit-msg). (§3)
- ❌ **Don't add a nil guard around the hooks calls.** decompose calls hooks.* directly (always); the runner's
  own absent-hook skip (hookExecutable) is the no-op mechanism on a no-hook repo. (Unlike generate's Deps.Hooks
  nil guard.) (§6)
- ❌ **Don't suppress hooks on resolveTipAmend to "protect" the tip message.** The tip is the arbiter's target;
  amend re-running msg hooks is git parity. §20.2 constrains ONLY resolveMidChain's non-target commits. (§5)
- ❌ **Don't edit decompose.go (the callers) or roles.go (Deps).** publishCommit runs hooks internally, so its
  callers are unaffected; no Deps.Hooks field is added. (§0/§1)
- ❌ **Don't forget mode 0755 on installed test hooks.** Without the owner-exec bit, hookExecutable returns
  false → the hook is skipped → the test is vacuous (asserts nothing). (§8)
- ❌ **Don't add the hooks import to generate.go or anywhere in internal/generate.** That would re-introduce
  the generate↔hooks cycle. Only decompose (message.go + chain.go) gets the import. (§1)
