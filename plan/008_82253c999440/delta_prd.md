# Delta PRD — v2.2: Arbiter freeze parity + planner `files`/soft-target

**Delta from:** `plan/007_b33d310438c6` (implemented the six diff-payload optimizations, FR3d–i — DONE)
**Delta to:** `PRD.md` @ v2.2 (Last updated 2026-07-05)
**Scope verdict:** MEDIUM delta. Two cohesive feature groups, both entirely inside the multi-commit decompose subsystem (`internal/decompose`, `internal/git`, `internal/prompt`). No config, CLI, provider, or commit/rescue/CAS/lock changes. Two further PRD text changes (FR50 verbose, `--version` VCS) are **already implemented** in the tree and need **zero work** — listed below for awareness only.

---

## 1. Diff analysis (what actually changed in the PRD)

The v2.2 PRD diff falls into four groups. Each is dispositioned against the **current codebase** (verified by grep, not assumed):

| Group | PRD change | Code state today | Disposition |
|---|---|---|---|
| **A. Arbiter freeze parity** (FR-M1d new; FR-M9, FR-M10, FR-M1b amended; §13.6.5 narrative; §20.2/§20.5 test invariants; risk-table row) | Arbiter *gate* must be the frozen `diff-names(tipTree, T_start)` (not live `git status --porcelain`); *resolution* must stage from `T_start` only via a new `OverlayTreePaths` primitive (not `git add -A`/`git add` against the live tree); index synced to `T_start` after. | **Loop hole OPEN.** `internal/decompose/decompose.go:217` gates on `StatusPorcelain` (live). `internal/decompose/chain.go` `resolveNewCommit`/`resolveTipAmend`/`resolveMidChain` all call `deps.Git.AddAll`/`Add` (lines 96, 142, 205) against the live working tree — the comment at `decompose.go:563` admits "staging is UNCHANGED — it stages from the working tree". `OverlayTreePaths` does **not** exist. | **NEW WORK — Phase 1** |
| **B. Planner `files` + soft target + mode-conditional prompt** (FR-M3, FR-M4 amended; FR-M3b new; §17.5 planner prompt rewrite; §17.6 stager `files` block; §13.6 role table) | Planner emits per-concept `files[]`; deterministic coverage check vs `DiffTreeNames(baseTree, T_start)` (non-fatal); soft count target `max_commits/2` interpolated into the prompt; auto-decompose rules block now "leans toward SEVERAL", forced-count has its own rules block; stager receives the concept's `files` block. | **Not implemented.** `prompt/planner.go:52` `PlannerCommit` has no `Files` field. No soft target (only the hard cap at `planner.go:132`). Planner prompt has no forced-count rules variant; stager task has no `files` block. | **NEW WORK — Phase 2** |
| **C. FR50 verbose: payload size + raw stderr** (FR50 amended; CLI table row; §19 secret-handling note) | `--verbose` now prints payload size (bytes + `chars/4` token estimate) and the agent's raw **stderr** per attempt. | **Already implemented.** `internal/ui/verbose.go:94` `VerbosePayload(bytes)`, `:74` `VerboseStderr`; `internal/provider/executor.go:57,73,80` captures `cmd.Stderr` separately and surfaces it. | **No work** — spec text caught up to the code |
| **D. `--version` VCS enrichment** (build-section paragraph) | `--version` enriches `dev` from Go's embedded `vcs.revision`/`vcs.modified` via `debug.ReadBuildInfo`. | **Already implemented.** `cmd/stagecoach/main.go:32-48` does exactly this. | **No work** — spec text caught up to the code |

**Removed requirements:** none.
**The previous session's research does NOT apply.** `plan/007_b33d310438c6/architecture/` covers diff-capture (`diff_capture_touchmap.md`, `git_diff_semantics.md`, `system_context.md`) — orthogonal to the arbiter/planner. The implementing agent does **fresh, targeted** research on the decompose code; the concrete seams are listed inline below so that research is a verification pass, not a discovery pass.

---

## 2. Goals (delta-only)

- **G1.** Close the arbiter freeze loophole: a file written to the working tree *after* `T_start` was captured can never land in an arbiter commit, on any of the three resolution paths, even when the loop otherwise committed all of `T_start` and the arbiter gate is reached. (FR-M1d, FR-M9, FR-M10)
- **G2.** Make the planner's partition machine-actionable and count-aware: every concept names its `files`; unclaimed paths are surfaced as likely leftovers; the count is guided by a soft `max_commits/2` target with a forced-count rules variant. (FR-M3, FR-M3b, FR-M4, §17.5)
- **G3.** No regressions to the freeze guarantees already shipped for the planner/stager (FR-M1b/M1c), the single shortcuts (FR-M2b/M11), CAS/lock/rescue, or the legacy `token_limit==0` diff path.

---

## 3. Non-goals (this delta)

- No new config keys, CLI flags, providers, or env vars. (FR-M4's soft target is **derived** from the existing `max_commits`; FR-M3's `files` is automatic in the planner JSON. Neither adds user-facing surface.)
- No changes to commit creation, rescue, CAS, the run lock (FR52), or the diff-capture pipeline (FR3d–i, done in session 007).
- No work on FR50 verbose or `--version` — already shipped (Groups C/D above).
- No interactive rebase, no `git commit --amend`. The mid-chain rebuild stays a deterministic plumbing reconstruction (HEAD only), as today.

---

## 4. Requirements (delta-only)

### 4.1 Phase 1 — Arbiter freeze parity (P0, → G1, G3)

- **FR-M1d (new). Arbiter gate, diff, and staging all derive from `T_start` (frozen) — never the live working tree.**
  - (1) **Gate:** replace the live `git status --porcelain` check (`decompose.go:217`) with the frozen leftover `DiffTreeNames(tipTree, T_start)`. Empty set → arbiter does not run (a concurrent working-tree change cannot make it run).
  - (2) **Diff:** already frozen today (`TreeDiff(tipTree, tStart)` at `decompose.go:613`); verify and keep.
  - (3) **Staging:** the three resolution paths build trees from `T_start` and the frozen per-concept `tree[j]`/`msg[j]` **only** — never `git add`/`AddAll` against the live tree, never `git status`.
  - After all three paths, sync the index to `T_start` (`ReadTree T_start`) so `git status` is clean for the committed set; concurrent working-tree changes remain unstaged/untracked.
  - **Docs (Mode A, ride with the work):** `docs/how-it-works.md` — the decompose/arbiter narrative must state that the arbiter gate/diff/staging are frozen (it currently describes the live-tree behavior Group A replaces).

- **FR-M9 (amended).** Arbiter runs **iff** the frozen leftover `diff-names(tipTree, T_start)` is non-empty; receives `TreeDiff(tipTree, T_start)`; live working tree never consulted for gate or diff. (Code: `decompose.go:214-228`.)

- **FR-M10 (amended, the core rewrite). Arbiter resolution — tree-only staging from `T_start`; new `OverlayTreePaths` primitive for the mid-chain path.** All three paths in `internal/decompose/chain.go` change:
  - `null` / new commit: `tree′ = T_start` (commit `T_start` directly — mirrors FR-M2b/M11; **no `AddAll`, no `WriteTree`**). Message agent runs on `TreeDiff(tipTree, T_start)`.
  - `target == tip`: `tree′ = T_start`; `commit-tree T_start -p <tip's parent>` reusing the tip's message verbatim; `update-ref HEAD` (plumbing amend to `T_start`'s tree — no `git commit --amend`, no live staging).
  - `target == commit[i]` (mid-chain): rebuild `i..N-1`; for each `j`, `tree′[j] = OverlayTreePaths(tree[j], T_start, leftoverPaths)`, then `commit-tree tree′[j] -p rebuiltParent` reusing `msg[j]` verbatim. The rebuilt tip equals `T_start`.
  - Ambiguous → default to `null` (unchanged). Amend restricted to this run's commits (unchanged).
  - **Signature change:** `resolveArbiter` (`chain.go:50`) currently takes `(target, commits, chainData)` and stages live; it must additionally take `tStart string` and `leftoverPaths []string` (= `DiffTreeNames(tipTree, tStart)`, computed once in `runArbiterPhase` `decompose.go:603`, which already holds `tStart` and derives `tipTree = chainData[last].Tree`). The `AddAll`/`Add` calls at `chain.go:96,142,205` are removed; the new paths use `OverlayTreePaths`/`CommitTree`/`UpdateRef`/`ReadTree` only.

- **New git primitive `OverlayTreePaths` (§13.6.5).** Add to `internal/git/git.go` alongside `ReadTree`/`WriteTree`/`FreezeWorkingTree`/`DiffTreeNames` (and the `Git` interface near `Add` at line 140):
  - `OverlayTreePaths(ctx, baseTree, sourceTree string, paths []string) (treeSHA string, err error)` — a new tree equal to `baseTree` with each `path` overwritten by its state in `sourceTree`.
  - Implementation: `read-tree baseTree` (index = baseTree) → for each path: present in `sourceTree` → `update-index --cacheinfo <mode>,<blob>,<path>`; absent (deletion-overlay) → `update-index --force-remove <path>`. The `(mode, blob)` pairs read once via `git ls-tree -r --full-tree <sourceTree> -- <paths...>`. → `write-tree`.
  - Mutates only `.git/index` + object store (same discipline as `FreezeWorkingTree`/`ReadTree`/`WriteTree`); never touches the working tree, never moves a ref. At its sole call site `sourceTree == T_start` and `paths == leftoverPaths`.

- **Test invariants (§20.2/§20.5 — the acceptance proof).** Extend `internal/decompose/decompose_test.go` (and `chain_test.go`):
  - **Concurrent-across-arbiter-gate:** write a sentinel file mid-run (after `T_start`), drive the loop to commit all of `T_start`, reach the arbiter gate, assert (a) sentinel in no commit, (b) sentinel remains in the working tree post-run, (c) no arbiter commit when the frozen leftover is empty.
  - **Arbiter folds only `T_start` content:** paired case with a legitimate frozen leftover (force one empty-skip concept) asserts each arbiter commit's tree is exactly `T_start` (or an `OverlayTreePaths` overlay of it) — the concurrent file still lands in no commit.
  - **`T_start` completeness (replaces "Loop index cleanliness"):** after a fully-successful run every `T_start` leftover is committed; live `git status` may be non-empty only from changes outside `T_start`.

### 4.2 Phase 2 — Planner `files` + soft target + mode-conditional prompt (P0, → G2, G3)

- **FR-M3 (amended).** `PlannerCommit` (`internal/prompt/planner.go:52`) gains `Files []string \`json:"files"\``. Each concept's `files` lists every path it touches; `description` says **per file** which change belongs. Planner does **not** emit hunks/line numbers (the stager resolves hunks mechanically). The parse (`internal/decompose/planner.go`) must populate `Files` and tolerate an empty list.
- **FR-M3b (new, deterministic + non-fatal). Planner coverage check.** After the planner returns, union `files` across all concepts and compare to `DiffTreeNames(baseTree, T_start)` (already computed in `decompose.go:182` as `changedPaths`). Any unclaimed path → `VerboseRawOutput` log as a likely leftover (the arbiter reconciles it); **never abort, never hard-constrain the stager** (FR-M1c stays the sole content guarantee).
- **FR-M4 (amended). Soft target.** Keep the hard cap (`Count > MaxCommits` rejected, `planner.go:132`). Add a soft target of `max_commits / 2` (default 6) interpolated into the auto-decompose planner prompt. Guidance only — never errors; only the hard cap does.
- **§17.5 planner system prompt (rewrite, `internal/prompt/planner.go`).** Rules block becomes **mode-conditional**:
  - Shared: opener, the "UNSTAGED … handed to you to organize" framing line, JSON contract (now `commits:[{title,description,files}]`), "account for every changed path", style/format examples tail.
  - **Auto-decompose** rules: lean toward SEVERAL (split changes serving different purposes); group only changes that only make sense together; keep the count at or below the soft target `<6>` (interpolated), approach the hard cap `<12>` only for genuinely unrelated changes.
  - **Forced-count** (`--commits N`) rules: partition into EXACTLY N; same split/group guidance; account for every path; respect dependencies; never reuse wording.
  - Builder emits exactly one rules block (auto unless `--commits N`), then appends style/format.
  - **Docs (Mode A, ride with the work):** `docs/how-it-works.md` decompose/planner section — the auto-vs-forced count behavior and the `files` partition contract.
- **§17.6 stager task (amended, `internal/prompt/stager.go`).** The stager receives the concept's `files` list as a `Files for this concept (where these changes live):` block; `files` is guidance (where to look), **not** a hard constraint (FR-M1c stays the content guarantee; an empty list omits the block). Update `stagerGuardrails` wording to "stage ONLY the changes the description assigns to this concept (the files above are where they live)".
- **Tests.** `planner_test.go`/`stager_test.go`: assert `Files` round-trips through parse; coverage check logs (not errors) on an unclaimed path; auto prompt contains the soft-target line and forced-count prompt swaps only the rules block; stager task renders the `files` block when non-empty and omits it when empty.

### 4.3 Sync changeset-level documentation (Mode B, depends on 4.1 + 4.2)

- **README.md:** surface the v2.2 decompose improvements in the feature list (arbiter now fully freeze-safe; planner partitions are per-file and count-guided). Keep the hero pitch intact; do not duplicate per-key reference.
- **docs/how-it-works.md:** the cross-cutting decompose narrative — arbiter gate/diff/staging frozen; planner `files` + soft target + mode-conditional rules. (Per-requirement Mode A edits in 4.1/4.2 land with the code; this task reconciles the section as a whole once both phases ship.)
- **Verify no stale claims:** `docs/cli.md` / `docs/configuration.md` need **no** changes (no new flags/keys — confirm and leave alone). `docs/providers.md` unaffected.

---

## 5. Phase / milestone / task shape (for the breakdown agent)

This is a MEDIUM delta — **2 phases, 1 milestone each**, plus a final Mode B docs sync. Do NOT inflate beyond this.

- **Phase 1 — Arbiter freeze parity (FR-M1d).** ~1 milestone, 3 tasks: (T1) `OverlayTreePaths` git primitive + test; (T2) arbiter gate→frozen leftover + the three resolution paths → tree-only-from-`T_start` (signature change to `resolveArbiter`); (T3) the §20.2/§20.5 concurrent-across-gate + arbiter-freeze-parity + `T_start`-completeness tests. Mode A doc edit to `docs/how-it-works.md` rides with T2.
- **Phase 2 — Planner `files`/soft-target/prompt (FR-M3/M3b/M4, §17.5/§17.6).** ~1 milestone, 2-3 tasks: (T1) `PlannerCommit.Files` + parse + FR-M3b coverage check; (T2) soft target + mode-conditional planner prompt (auto vs forced-count rules blocks); (T3) stager `files` block + prompt tests. Mode A doc edit rides with T2.
- **Final task (Mode B):** the §4.3 changeset-level docs sweep, depending on all of the above.

## 6. Acceptance

- A concurrent working-tree change written after `T_start` is captured lands in **no** commit and remains in the working tree — proven by a test that forces the arbiter gate (Phase 1).
- Every arbiter commit's tree is built from `T_start` (or an `OverlayTreePaths` overlay of frozen trees) — no `git add`/`AddAll`/`status` reads the live tree in any resolution path.
- Planner output carries per-concept `files`; an unclaimed path is logged not fatal; auto-decompose prompt contains the soft target and forced-count swaps only the rules block.
- `docs/how-it-works.md` + `README.md` reflect v2.2; `cli.md`/`configuration.md` unchanged.
