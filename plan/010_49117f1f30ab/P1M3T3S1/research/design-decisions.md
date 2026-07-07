# P1.M3.T3.S1 — decompose.publishCommit hook wiring + mid-chain-fidelity invariant: design decisions

> The single source of truth for the judgment calls in this subtask. Read BEFORE implementing.
> Every decision is pinned by a test. The contract (item_description point 3) + open_questions.md §2
> are decisive; the code is the source of truth for the insertion points + import graph.

The work-item CONTRACT (item_description, point 3/4) is:

> extend publishCommit (or add a wrapping helper it calls) to run RunCommitHooks between receiving
> (tree, parentSHA, msg) and CommitTree … on herr → return the *RescueError … Then CommitTree(finalTree,
> parents, finalMsg) + UpdateRefCAS. After UpdateRefCAS: RunPostCommit (best-effort). The per-concept
> scoping is automatic: each publishCommit call passes THAT concept's tree[i] … CONFIRM
> resolveNewCommit/resolveTipAmend route through publishCommit (or add the hook runner to their
> CommitTree calls too — they produce user-facing commits). resolveMidChain stays hook-free (it doesn't
> use publishCommit). … ACCEPTANCE INVARIANT (§20.2 mid-chain amend fidelity): after an arbiter mid-chain
> rebuild, the non-target commits are byte-identical … running NO hooks on resolveMidChain guarantees this.

---

## §0 — Scope: wire hooks into 3 commit sites; leave resolveMidChain + everything else alone

This subtask edits TWO production files + their tests:

1. `internal/decompose/message.go` — `publishCommit`: insert `hooks.RunCommitHooks` before `CommitTree`
   + `hooks.RunPostCommit` after `UpdateRefCAS` success.
2. `internal/decompose/chain.go` — `resolveNewCommit` + `resolveTipAmend`: the SAME two inserts at each
   site's `CommitTree`/`UpdateRefCAS`. **`resolveMidChain` is UNCHANGED** (hook-free — §2).
3. Tests: `internal/decompose/message_test.go` + `internal/decompose/chain_test.go` (+1 hook test each, or
   a shared helper).

It does NOT:
- touch `decompose.go` (the main loop publish closure, `runSingleShortcut`, `runOneFileShortcut` all CALL
  `publishCommit` — which now runs hooks internally; no caller edit needed),
- touch `runSingleEscape` (delegates to `generate.CommitStaged`, covered by P1.M3.T2.S1),
- add a `Hooks` field to `decompose.Deps` (decompose calls `hooks.*` DIRECTLY — §1),
- modify `internal/hooks/*`, `internal/generate/*`, `internal/git/*`, `pkg/stagecoach/*`, `internal/cmd/*`,
- change docs (the decompose composition is covered by M3.T2.S1's how-it-works.md subsection — item point 5).

The per-concept scoping (FR-V8c) is AUTOMATIC: each `publishCommit`/arbiter commit call passes THAT
concept's `tree[i]`/`tStart` as the snapshot tree, so `pre-commit` sees only that concept's staged subset
(the runner materializes a throwaway index from that tree — FR-V3, already implemented in runner.go).

---

## §1 — THE import-graph call: decompose imports `internal/hooks` DIRECTLY (NOT the interface approach)

**Decision: `internal/decompose` does `import "github.com/dustin/stagecoach/internal/hooks"` and calls
`hooks.RunCommitHooks` / `hooks.RunPostCommit` with `hooks.HookOpts{DryRun: false, Verbose: deps.Verbose}`
— the CONCRETE functions, NOT an interface on `Deps`.**

Why this differs from generate (P1.M3.T2.S1 used a `CommitHookRunner` interface + `Deps.Hooks` + an
`adapter.go`):

- **The generate↔hooks cycle does NOT extend to decompose.** `internal/hooks/runner.go` imports
  `internal/generate` (for `generate.RescueError`). So `generate.go` CANNOT import `internal/hooks` (Go
  rejects the cycle) — THAT is why S1 injected the interface. But `internal/generate` imports NEITHER
  `internal/hooks` NOR `internal/decompose` (verified: the only matches in generate.go are COMMENT text;
  generate.go has no hooks/decompose import line). So the graph is `hooks → generate`, `decompose → generate`,
  and `decompose → hooks` is **ACYCLIC** (hooks→generate→nothing-back-to-decompose).
- **Therefore decompose can import hooks directly.** No interface, no `adapter.go`, no `Deps.Hooks` field.
  The call is the contract's literal form: `hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree,
  parentSHA, msg, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})`.
- **Do NOT copy S1's interface/adapter pattern into decompose.** That pattern exists ONLY to break the
  generate↔hooks cycle. Decompose has no such cycle; adding an interface + Deps.Hooks + adapter would be
  needless indirection + a new field on `Deps` (roles.go, frozen-ish) + a new file. Direct call is simpler
  and matches the contract.

`message.go` adds `internal/hooks` to its existing imports (config/generate/git/prompt/provider).
`chain.go` adds `internal/hooks` to its existing imports (generate/git). No other import change.

---

## §2 — THE headline invariant: `resolveMidChain` stays HOOK-FREE (§20.2 mid-chain fidelity)

`resolveMidChain` (chain.go) walks `j = i..N-1`, building `treePrime = OverlayTreePaths(tree[j], tStart,
leftoverPaths)` then `CommitTree(treePrime, parents, chainData[j].Message)` — reusing `msg[j]` **verbatim**.
These are deterministic reconstructions, NOT user-facing "new" commits.

**Decision: do NOT wire hooks into `resolveMidChain`.** Running a foreign `prepare-commit-msg` on a
verbatim-reused `msg[j]` would risk annotating/mangling it, breaking the §20.2 "mid-chain amend fidelity"
invariant (the rebuilt non-target commits must be byte-identical — same tree via the deterministic
OverlayTreePaths fold, same message via verbatim reuse). `resolveMidChain` does its OWN `CommitTree` +
`UpdateRefCAS` (chain.go, NOT via `publishCommit`), so it is naturally hook-free — just do not add the
runner there. This is open_questions.md §2's confirmed resolution and the contract's explicit "DO NOT wire
the runner into resolveMidChain".

The load-bearing test (MOCKING point 4): install a `prepare-commit-msg` that appends; drive a mid-chain
arbiter resolution; assert the rebuilt commits' messages carry NO annotation (`msg[j]` verbatim), while the
per-concept + arbiter-new + arbiter-tip commits DO carry it.

**`resolveNewCommit` (N+1) and `resolveTipAmend` (tip amend) DO run hooks** — they produce user-facing
commits (open_questions §2; the contract). They are wired in §4.

---

## §3 — `publishCommit` wiring (message.go)

`publishCommit(ctx, deps Deps, tree, parentSHA, msg string) (newSHA string, err error)` is the serialized
publication primitive (CommitTree → UpdateRefCAS). INSERT two hooks:

- **INSERT A (RunCommitHooks)** — after the `parents` slice is built, BEFORE `deps.Git.CommitTree`:
  ```go
  finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
      hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
  if herr != nil {
      return "", herr // *generate.RescueError (FR-V7) — propagates; the runLoop's FR-M12 handling maps it
  }
  ```
  Then `CommitTree(finalTree, parents, finalMsg)` (use the hook-adjusted tree + message). `hookParent =
  parentSHA` (the param) — threaded into the rescue-error context only.
- **INSERT B (RunPostCommit)** — AFTER the `UpdateRefCAS` success block, BEFORE `return newSHA, nil`:
  ```go
  _ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
  ```
  Best-effort; return discarded (FR-V7 — post-commit exit is disregarded; the commit already landed).

**Per-concept scoping is automatic (FR-V8c):** every `publishCommit` caller (main loop closure
`decompose.go:484`, `runSingleShortcut :390`, `runOneFileShortcut :336`) passes THAT concept's `tree[i]`/
`tStart`. `pre-commit` runs against a throwaway index materialized from that tree (runner.go's
`runPreCommitScoped` → `ReadTreeInto(snapshotTree, tmpIndex)`), so it sees ONLY that concept's subset. No
caller change needed.

`DryRun: false` always — decompose has no dry-run path (dry-run is the single-commit runPipeline, P1.M3.T2.S2).

---

## §4 — `resolveNewCommit` + `resolveTipAmend` wiring (chain.go)

Both do their OWN `CommitTree` + `UpdateRefCAS` + `handleUpdateRefErr` + `ReadTree(tStart)` sync (they do
NOT call `publishCommit` — their CAS expected-old differs from `publishCommit`'s parentSHA-based one). Add
the SAME two inserts at each site, preserving each site's existing CAS handling + ReadTree sync.

**`resolveNewCommit` (path A, null — the N+1 commit):**
- INSERT A — before its `CommitTree(treePrime, parents, msg)`: `hooks.RunCommitHooks(ctx, deps.Git,
  deps.Config, treePrime, tipSHA, msg, HookOpts{…})`. `hookParent = tipSHA` (the new commit's parent =
  current HEAD/tip). On `herr` → `return herr` (propagate `*generate.RescueError` DIRECTLY — matches
  `generateMessage`'s pattern; `resolveArbiter` propagates it unwrapped). Use `finalTree`/`finalMsg` in
  `CommitTree`.
- INSERT B — after `UpdateRefCAS` success, before `ReadTree(tStart)`: `RunPostCommit(…)` (discarded).

**`resolveTipAmend` (path B, target==tip — the plumbing amend):**
- INSERT A — before its `CommitTree(treePrime, parents, tipMsg)`: `hooks.RunCommitHooks(ctx, deps.Git,
  deps.Config, treePrime, tipParent, tipMsg, HookOpts{…})`. `hookParent = tipParent` (the amended commit's
  parent = the original tip's parent). On `herr` → `return herr`. Use `finalTree`/`finalMsg`.
- INSERT B — after `UpdateRefCAS` success, before `ReadTree(tStart)`: `RunPostCommit(…)` (discarded).

**CAS expected-old is UNCHANGED at each site** — `resolveNewCommit` keeps `expectedOld = tipSHA` (or
zeros); `resolveTipAmend` keeps `expectedOld = tipSHA` (NOT tipParent — that is the whole reason these
sites can't route through `publishCommit`, which would use the parent as expected-old). The hooks run
BEFORE `CommitTree`; the CAS logic is untouched.

---

## §5 — `resolveTipAmend` runs hooks on the verbatim tip message (`git commit --amend` parity)

`resolveTipAmend` reuses the tip's message (`tipMsg`) verbatim as the INPUT to `RunCommitHooks`. A foreign
`prepare-commit-msg` MAY annotate it (append a ticket ref, etc.) — so the amended tip's committed message
can DIFFER from the original tip's. This is **correct git parity**: `git commit --amend` re-runs
`prepare-commit-msg` on the amended message (a known git behavior — amend re-runs the msg hooks). The
contract explicitly says `resolveTipAmend` "SHOULD run hooks (it produces a user-facing commit)".

This does NOT violate §20.2: the mid-chain fidelity invariant constrains ONLY the **non-target** commits
(rebuilt by `resolveMidChain`, which is hook-free). The tip IS the arbiter's target (the amend target), so
its message changing is permitted — exactly as an amended commit's message may change. The double-
annotation parity (the tip already went through hooks when first published; amend re-runs them) is git's
own behavior, not a stagecoach bug. Document it in the `resolveTipAmend` comment.

---

## §6 — No `Deps.Hooks` field; existing tests stay GREEN (absent-hook no-op)

Because decompose calls `hooks.*` directly (§1), there is NO `Deps.Hooks` field to add and NO nil guard.
`hooks.RunCommitHooks` ALWAYS runs when `publishCommit`/`resolveNewCommit`/`resolveTipAmend` are called.
But on a repo with NO hooks installed (the existing `decompose_test.go` + `chain_test.go` repos), the
runner **no-ops**: `runPreCommitScoped`/`runPrepareCommitMsg`/`runCommitMsg` each check `hookExecutable`
(absent/non-exec → return early), so `RunCommitHooks` returns `(snapshotTree, msg, nil)` — the inputs
unchanged. `RunPostCommit` likewise no-ops (absent `post-commit` → return nil).

**Therefore the ~existing decompose/chain tests stay GREEN** (no hooks → no-op → byte-identical behavior).
The runner's own `hookExecutable` skip is the no-op mechanism here (NOT a nil-Deps.Hooks guard, as in
generate). The one prerequisite: `RunCommitHooks` calls `g.HooksPath`/`g.GitDir`/`g.TopLevel` at entry —
these work on the real temp git repos the tests use (`git init`'d). Verified the decompose/chain tests use
real repos (they stage/commit/`RevParseTree`), so these resolve cleanly.

**GOTCHA — `RunCommitHooks` returns `("", "", err)` on a HooksPath/GitDir/TopLevel resolution error.** If a
test repo somehow failed these, the commit would error. The real-repo tests do not. (If a future library
test constructs `Deps` with a non-repo Git, it would surface here — but none do today.)

---

## §7 — `HookOpts{DryRun: false, Verbose: deps.Verbose}`

`hooks.HookOpts{DryRun bool, Verbose *ui.Verbose}` (runner.go). For every decompose call:
- `DryRun: false` — decompose always commits (no dry-run path; dry-run is single-commit runPipeline).
- `Verbose: deps.Verbose` — `*ui.Verbose`, nil-safe inside the runner (`if opts.Verbose != nil`).

The runner honors these: `DryRun=false` ⇒ pre-commit RUNS (not skipped), commit-msg RUNS, post-commit RUNS
(the full sequence, FR-V1). `cfg.NoVerify` (FR-V5) + `cfg.HookTimeout` (FR-V6) come from `deps.Config`
(threaded into `RunCommitHooks` as `cfg`).

---

## §8 — Tests

Mirror the existing `chain_test.go` `TestResolveArbiter_*` style (build `commits`/`chainData`/`tStart`/
`leftoverPaths`, call `resolveArbiter` directly) + the hook-install pattern from
`internal/generate/hooks_freeze_test.go` (write an executable script to `<repo>/.git/hooks/<name>`, mode
0755). Add:

1. **`message_test.go` — `TestPublishCommit_*`:**
   - `…_PrepareCommitMsgAnnotates` — install a `prepare-commit-msg` that appends a marker; call
     `publishCommit`; assert the landed commit's message (`git log -1 --format=%B`) carries the marker
     (hooks ran + the annotated `finalMsg` was committed).
   - `…_PreCommitAbort_RescueError` — install a `pre-commit` that exits 1; call `publishCommit`; assert
     `errors.As(err, &*generate.RescueError)` AND HEAD unchanged (FR-V7 idempotent — no `CommitTree`/`update-ref` ran).
   - (post-commit is best-effort; assert it RAN via a `post-commit` that touches a sentinel file — optional.)
2. **`chain_test.go` — `TestResolveArbiter_*_Hooks`:**
   - `…_NullNewCommit_RunsHooks` — `prepare-commit-msg` appends; drive `resolveArbiter(nil, …)`; assert the
     N+1 commit's message carries the marker.
   - `…_TipAmend_RunsHooks` — `prepare-commit-msg` appends; drive `resolveArbiter(&tipSHA, …)`; assert the
     amended tip's message carries the marker (§5 — amend re-runs msg hooks).
   - `…_MidChain_SkipsHooks` (THE fidelity invariant) — `prepare-commit-msg` appends; drive
     `resolveArbiter(&sha[i], …)` for `i < N-1`; assert the rebuilt commits' messages carry NO marker
     (`msg[j]` verbatim) — `resolveMidChain` ran hook-free. This is the §20.2 acceptance test.

Mirror `stubtest.Build`/`stubtest.Manifest` for the message role (the null path's `generateMessage` needs a
stub message; the tip/mid paths reuse `chainData[j].Message` verbatim and never call the agent — like the
existing `TestResolveArbiter_TipAmend`/`_MidChainRebuild` which set `Out: "SHOULD NOT BE USED"`). Install
the hook in the temp repo's `.git/hooks/prepare-commit-msg` (0755) BEFORE calling `resolveArbiter`.

---

## §9 — Frozen files + upstream/downstream contracts

**FROZEN (do NOT edit):**
- `internal/decompose/decompose.go` (main loop / shortcuts CALL `publishCommit` — no edit; `runSingleEscape`
  delegates to `generate.CommitStaged`, P1.M3.T2.S1).
- `internal/decompose/roles.go` (`Deps` — NO `Hooks` field added; §1), `arbiter.go`, `planner.go`, `stager.go`.
- `internal/hooks/*` (runner.go/adapter.go/subset.go — P1.M3.T1/M1.T2 COMPLETE; consumed read-only),
  `internal/generate/*` (P1.M3.T2.S1), `internal/git/*`, `internal/cmd/*`, `pkg/stagecoach/*`, `internal/signal/*`,
  `internal/hook/*`, `internal/lock/*`, `docs/*`.
- `go.mod`/`go.sum` (decompose already imports generate/git; adding `internal/hooks` adds NO module — it's
  an internal package).

**UPSTREAM (consume, do NOT edit):**
- `hooks.RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts) (finalTree, finalMsg, err)` +
  `hooks.RunPostCommit(ctx, g, cfg, HookOpts) error` (runner.go, P1.M3.T1.S1/S2 COMPLETE). The runner honors
  `cfg.NoVerify`/`cfg.HookTimeout`, skips absent/non-exec hooks, returns `*RescueError` on hook non-zero/timeout.
- `deps.Git` (git.Git), `deps.Config` (config.Config: NoVerify, HookTimeout), `deps.Verbose` (*ui.Verbose).

**DOWNSTREAM:** none new. The per-concept scoping (FR-V8c) + the mid-chain fidelity (§20.2) are the
acceptance invariants; no future task depends on a new symbol from this subtask (the wiring is internal).
The `hooks.RunCommitHooks`/`RunPostCommit` + `HookOpts` APIs are FROZEN (P1.M3.T1).

**EDIT (this subtask):** `internal/decompose/message.go` (publishCommit: 2 inserts + the `internal/hooks`
import), `internal/decompose/chain.go` (resolveNewCommit + resolveTipAmend: 2 inserts each + the
`internal/hooks` import), `internal/decompose/message_test.go` + `chain_test.go` (+ hook tests).
