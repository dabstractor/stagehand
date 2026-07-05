# System Context — plan/008 (v2.2 delta)

**Project:** stagehand — Go CLI for AI-generated commit messages via user-installed agent CLIs.
**Codebase state:** v1 single-commit core + v2.0 multi-commit decomposition + v2.1 competitor-parity
features (exclusions, format modes, hook mode, integrate, --edit/--push, discovery) are **implemented
and tested**. This plan targets the **v2.2 delta** only.
**Delta scope (authoritative):** `plan/008_82253c999440/delta_prd.md`. Two feature groups, both
entirely inside `internal/decompose` + `internal/git` + `internal/prompt`. **No config / CLI /
provider / commit-CAS / lock / rescue changes.**

## 1. What v2.2 adds (and what is already done)

| Group | PRD ref | Code state | Plan action |
|---|---|---|---|
| **A. Arbiter freeze parity** | FR-M1d (new); FR-M9/M10/M1b amended; §13.6.5; §20.2/§20.5 | **Loop hole OPEN.** Gate reads live `StatusPorcelain` (`decompose.go:217`); three resolution paths stage via live `AddAll`/`Add` (`chain.go:99,142,184,205,209`). `OverlayTreePaths` does NOT exist. The arbiter's *diff* input is ALREADY frozen (`TreeDiff(tipTree, tStart)` in `runArbiterPhase`). | **Phase 1** |
| **B. Planner `files` + soft target + mode-conditional prompt** | FR-M3/M3b/M4 amended; §17.5/§17.6 | **Not implemented.** `PlannerCommit` has no `Files` (`planner.go:57`); single "Prefer FEWER commits" rules block; no soft target; no coverage check; stager has no `files` block. | **Phase 2** |
| C. FR50 verbose (payload size + raw stderr) | FR50 | Already shipped (`ui/verbose.go`, `provider/executor.go`) | None |
| D. `--version` VCS enrichment | build §21.1 | Already shipped (`cmd/stagehand/main.go:32-48`) | None |

## 2. Architecture at a glance (relevant subsystems)

```
cmd/stagehand/main.go            CLI entrypoint (unchanged by this delta)
internal/git/git.go              Git interface + gitRunner impl (Phase 1: +OverlayTreePaths)
internal/decompose/
  decompose.go                   Decompose() orchestrator; runArbiterPhase; the GATE (Phase 1)
  chain.go                       resolveArbiter + 3 resolution paths (Phase 1 core rewrite)
  planner.go                     callPlanner (Phase 2: signature threading, coverage check)
  stager.go                      tooled stager call (Phase 2: files block)
internal/prompt/
  planner.go                     plannerSystemPrompt + PlannerCommit (Phase 2 core)
  stager.go                      BuildStagerTask (Phase 2)
docs/how-it-works.md             decompose narrative (Mode A edits per phase + Mode B sweep)
```

The freeze boundary already works for the **planner** and **stager** (FR-M1b/M1c, shipped) and the
**single shortcuts** (FR-M2b/M11, shipped). v2.2 extends it into the **arbiter** — the third and final
freeze surface — so the run's entire commit output is derived strictly from `T_start`.

## 3. Key invariants this delta must NOT regress

- **Snapshot atomicity** (§13.2/§18.1): refs move only at `UpdateRefCAS`; index never reset between
  `write-tree` and `update-ref`. The arbiter rewrite preserves this (trees built via plumbing only).
- **Stage-while-generating** (§13.4): the per-concept `tree[i]` freeze + tree-to-tree concept diff
  (unchanged by this delta).
- **Freeze subset** (FR-M1c): `runLoop`'s `verifyFreezeSubset` (stager content ⊆ `T_start`) is the
  sole content guarantee; FR-M3b's coverage check is diagnostic-only and must NOT hard-constrain it.
- **Hard commit cap** (FR-M4): `callPlanner`'s `Count > MaxCommits` rejection stays; the new soft
  target is guidance-only (never errors).
- **Legacy `token_limit==0` diff path**: untouched (FR3d–i, session 007).

## 4. Phase plan (matches delta_prd.md §5)

### Phase 1 — Arbiter freeze parity (FR-M1d) [P0]
- **T1 — `OverlayTreePaths` git primitive.** New plumbing op: `read-tree baseTree` →
  `ls-tree -r --full-tree sourceTree -- <paths>` → per-path `update-index --cacheinfo`/`--force-remove`
  → `write-tree`. Lives in the `Git` interface near `DiffTreeNames` and impl near `FreezeWorkingTree`.
  Mutates only `.git/index` + object store; never the working tree, never a ref. Handles the
  deletion-overlay case (path absent in sourceTree → `--force-remove`).
- **T2 — Freeze-safe arbiter gate + resolution.** Gate: `StatusPorcelain` → frozen
  `DiffTreeNames(tipTree, tStart)` (empty ⇒ arbiter skipped — a concurrent change can't trigger it).
  `resolveArbiter` signature gains `tStart, leftoverPaths`. Three paths: new/tip → `treePrime = T_start`
  (drop `AddAll`/`WriteTree`); mid-chain → `OverlayTreePaths(tree[j], T_start, leftoverPaths)` (drop
  live `ReadTree`/`Add`/`StatusPorcelain`). Each path ends with `ReadTree(T_start)` index sync. Mode A
  doc edit to `docs/how-it-works.md` rides here.
- **T3 — Arbiter freeze-parity invariant scenarios (acceptance).** §20.2/§20.5: concurrent-across-
  arbiter-gate (sentinel in no arbiter commit; no arbiter commit when frozen leftover empty);
  arbiter-folds-only-`T_start`-content; `T_start` completeness. Unit proof in `chain_test.go` + the
  integration upgrade in `decompose_test.go` (the existing `TestDecompose_ConcurrentChangeExclusion`
  comment admits the loophole — it is the upgrade target).

### Phase 2 — Planner `files` + soft target + mode-conditional prompt (FR-M3/M3b/M4) [P0]
- **T1 — `PlannerCommit.Files` + FR-M3b coverage check.** Add `Files []string` to the struct (parse
  populates it for free via `json.Unmarshal`; tolerate `null`/absence). FR-M3b: union concept.Files,
  compare to frozen `DiffTreeNames(baseTree, tStart)`, log unclaimed paths via `VerboseRawOutput`
  (never error, never constrain the stager). Mode A doc edit rides with T2.
- **T2 — Mode-conditional planner prompt + soft target.** Split the single `plannerSystemPrompt` into
  shared (opener + UNSTAGED framing + JSON-contract-with-files + style/format tail) + a swappable
  `Rules:` block: auto-decompose (lean toward SEVERAL; soft target `max_commits/2` interpolated) vs
  forced-count (partition into EXACTLY N; no soft-target line). `BuildPlannerSystemPrompt` gains
  `forcedCount, maxCommits int`. Call-site update in `decompose/planner.go`.
- **T3 — Stager `files` block.** `BuildStagerTask` gains `files []string`; renders a
  `Files for this concept (where these changes live):` block (omitted when empty); guardrails wording
  updated to "Stage ONLY the changes the description assigns to this concept (the files above are where
  they live)".

### Phase 3 — Changeset-level documentation sync (Mode B)
- **T1 — README.md + docs/how-it-works.md cross-cutting sweep.** Depends on every implementing
  subtask. README surfaces v2.2 decompose improvements (arbiter fully freeze-safe; planner per-file +
  count-guided) without duplicating per-key reference. `how-it-works.md` reconciles the decompose
  section as a whole once both phases ship. **Verify** `docs/cli.md`/`docs/configuration.md` need no
  changes (no new flags/keys) and leave them alone.

## 5. Detailed research artifacts

- `architecture/arbiter_freeze_parity.md` — Phase 1: exact seams, line numbers, OverlayTreePaths
  design, signature changes, test invariants, risks.
- `architecture/planner_prompt.md` — Phase 2: Files field, verbatim auto/forced rules blocks from
  PRD §17.5, soft-target interpolation, coverage-check algorithm, stager topology, test assertions.

Both are grounded in direct file reads with line numbers; downstream PRP agents should treat them as
the contract for `context_scope` INPUT/OUTPUT/MOCKING definitions.
