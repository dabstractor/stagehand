# Delta PRD — Hook execution on the commit path (v2.4 → G22, FR-V1–V8)

## Diff analysis & sizing

**Document diff (current PRD vs prior v2.3 snapshot):** a focused, single-feature revision. The changes are:

1. **New revision-history row (v2.4)** — adds hook execution on the commit path (§9.25; FR-V1–V8, → G22) and the `--no-verify` flag.
2. **New functional section §9.25** — eight new requirements (FR-V1 through FR-V8), the entire body of the feature.
3. **New goal G22** (§6.1); **new user story US27** + reframed **US19** (§8).
4. **Reframed §9.20 (hook mode)** intro + **FR-H7** — hook mode is no longer the *only* way to get hooks; the plumbing path runs them itself (§9.25). The two modes now compose.
5. **§1 (exec summary)** point 1 — notes hooks run on the plumbing path, scoped to the snapshot; `--no-verify` is the one-off opt-out.
6. **§10.4 (version scope)** — the accepted-features list adds §9.25 and reframes the pre-commit-hooks caveat as directly closed.
7. **New CLI flag** `--no-verify` (env `STAGECOACH_NO_VERIFY`, git config `stagecoach.no_verify`); **new config default** `hook_timeout = 10m` (§16.1, FR-V6).
8. **Appendix F** — one new decision-log entry (hooks scoped to the snapshot, v2.4).

The spec body is ~8 dense FRs in one new section — materially larger than the v2.3 multi-turn *document* delta (which was a 1-line revision row over an already-specced section), but a single cohesive feature, not a multi-area rework.

**Implementation reality (entirely unimplemented):** verified by grep — no `NoVerify`/`no_verify`, `HookTimeout`/`hook_timeout`, or `--no-verify` symbols exist anywhere in `internal/`, `cmd/`, or `docs/`. Specifically:
- `internal/config/config.go` `Config` struct has **no** `NoVerify` / `HookTimeout` fields; `Defaults()` sets neither.
- `internal/cmd/root.go` registers **no** `--no-verify` flag (the `--push`/`--edit` flags at lines 200–212 are the closest analogs).
- There is **no** commit-hook-sequence runner. `internal/hook/` is exclusively the §9.20 *hook mode* (install/uninstall/`exec` — the prepare-commit-msg bridge for plain `git commit`); it has no code that runs a repo's `pre-commit`/`commit-msg`/`post-commit`.
- `generate.CommitStaged` (`internal/generate/generate.go`) goes straight from `EditMessage` (line 389) → `CommitTree` (line 399) → `UpdateRefCAS` (line 410) with **no** hook invocation between them.
- `decompose.publishCommit` (`internal/decompose/message.go:219`) does `CommitTree` → `UpdateRefCAS` with **no** hook invocation.
- `docs/how-it-works.md` §"Hook mode vs the snapshot-based flow" (line 303) **explicitly states the plumbing path "Bypasses pre-commit hooks"** — this is now **false** and must be rewritten.

**Prior session (009)** implemented the multi-turn generation fallback (FR-T1–T12) — **unrelated to this feature**. `plan/009_5c53066d64b3/architecture/` holds no reusable research for commit-hook execution (a genuinely new topic). No web research is needed: the feature is git-hook semantics (well-documented) composed onto the existing plumbing core.

**Sizing verdict: medium feature.** One new runner module + two config knobs + one CLI flag + new git plumbing (scoped-index materialization) + wiring into **two** commit chokepoints (`CommitStaged` and `publishCommit`) + a doc reframe that contradicts the current `how-it-works.md`. It reuses the existing rescue (§9.10 → `*RescueError`), CAS, lock, and signal paths unchanged (FR-V7: a hook abort is a pre-`update-ref` rescue). One phase, one milestone, four implementing tasks + a Mode B doc sync. Slightly more surface than multi-turn (which touched one commit path), because this touches the single-commit path **and** every decompose commit point, plus requires snapshot-scoped pre-commit materialization (the one genuinely subtle new git plumbing).

---

## Scope delta

The authoritative requirement is **PRD §9.25, FR-V1–FR-V8** (reference it by number; do not re-spec). This delta PRD scopes its **implementation**. The v2.4 revision-history row already in PRD.md is the governance signal; no further PRD.md edit is required.

**In scope:**
- Run the repo's commit hooks (`pre-commit` → `prepare-commit-msg` → `commit-msg`, then `post-commit`) around **every** plumbing-path commit, in git's documented order (FR-V1, FR-V2).
- **Snapshot-scoped `pre-commit`** (FR-V3): `pre-commit` runs against the frozen tree content, **not** the live index, so the stage-while-generating freeze holds. A hook may mutate `T_start`/snapshot content (formatter re-stages) → re-tree and commit the hook's output (git-commit parity). A hook that stages a path **not** in the snapshot is a hard error (subset enforcement, mirroring the stager's FR-M1c).
- **Recursion prevention** (FR-V4): if stagecoach's own `prepare-commit-msg` hook is installed (FR-H1, `Marker`-detected), **skip** it on the plumbing path (the message is already generated; invoking it would recurse). Foreign `prepare-commit-msg`/`commit-msg` hooks run and may annotate the message; stagecoach reads the file back as the final message.
- **`--no-verify`** (FR-V5): skips `pre-commit` and `commit-msg` ONLY (mirrors `git commit --no-verify`); `prepare-commit-msg` and `post-commit` still run. Full 5-layer precedence (flag/env/git-config/file/default=false).
- **Hook execution environment + per-hook timeout** (FR-V6): `GIT_DIR`, `GIT_INDEX_FILE` (the snapshot-scoped index), worktree as CWD, message-file path as arg 1 for `*-commit-msg` hooks, stdin `/dev/null`; stdout/stderr pass through; per-hook timeout `[generation].hook_timeout` (default `10m`).
- **Failure is a rescue** (FR-V7): non-zero/timeout from `pre-commit`/`prepare-commit-msg`/`commit-msg` aborts the run before any ref mutation → existing rescue recipe (`*RescueError`, exit 3), HEAD + index byte-for-byte unchanged. `post-commit` is best-effort (runs after `update-ref`; its exit is a warning, never undoes a landed commit — git parity).
- **Composition** (FR-V8): `--dry-run` (skip pre/post-commit, run commit-msg on the would-be message); `--edit` (`*-commit-msg` hooks run on the post-editor message); decompose (full sequence per per-concept commit, scoped to that concept's tree); hook mode (§9.20) unchanged — still the bridge for plain `git commit` from IDEs.

**Out of scope (explicit):**
- No change to the snapshot/atomic-commit core, the CAS, the run lock, or signal handling. Hooks are **threaded between** generation and `commit-tree`/`update-ref`; the core is untouched.
- No switch to `git commit` (FR-V2: stagecoach stays on plumbing). Hook mode (§9.20) is unchanged as a *mode* — only its *positioning* in the docs changes (no longer the sole way to get hooks).
- No new exit codes, no rescue-message changes. A hook failure is the existing `*RescueError{Kind: ErrRescue}` (exit 3).

**Removed requirements:** none. (Note for awareness: the prior §5 caveat / US19 / FR-H7 framing that "the plumbing path bypasses hooks" is **superseded** — those passages are reframed in the current PRD, and the matching `docs/how-it-works.md` text must be rewritten in Mode B.)

---

## Open engineering questions (resolve during breakdown research, not blocking this PRD)

These are architecture-research items for the breakdown agent. The §9.25 spec is decisive on *behavior*; these are *mechanism* questions.

1. **Snapshot-scoped pre-commit materialization (the hard part).** `pre-commit` traditionally runs against the live `.git/index`. On the single-commit path the live index at hook-time may contain files the user staged **during** generation (violating the freeze if `pre-commit` saw them); on the decompose per-concept path the live index holds the full accumulation, not just concept *i*'s tree. So `pre-commit` must run against a **throwaway index materialized from the frozen tree** (`treeSHA` / `tree[i]` / `T_start`). Candidate mechanism: a temp index file via `GIT_INDEX_FILE`, `git read-tree <tree>` into it, run `pre-commit` with that env, then `git write-tree` to capture mutations and `DiffTreeNameStatus` to enforce the subset (no path outside the snapshot added). The existing `git.ReadTree`/`WriteTree`/`OverlayTreePaths`/`DiffTreeNameStatus` primitives (`internal/git/git.go`) operate on `.git/index` by default — confirm whether a `GIT_INDEX_FILE`-scoped variant is needed or whether the runner can manage the env directly for the child process. **This is the single biggest open mechanism question.**
2. **Arbiter mid-chain rebuild hooks (ambiguity in FR-V1).** FR-V1 says hooks run on "every commit produced by the plumbing path," and FR-V8c says the sequence runs "around each per-concept commit." The arbiter's mid-chain rebuild (FR-M10) produces commits via `commit-tree` reusing `msg[j]` **verbatim** and trees via deterministic `OverlayTreePaths` reconstruction — these are not user-facing "new" commits. Running a foreign `prepare-commit-msg` on a verbatim-reused message risks violating the §20.2 "mid-chain amend fidelity" invariant (non-target commits must be byte-identical). **Recommended interpretation (confirm):** hooks run on user-facing commits (single, per-concept, arbiter new-commit N+1, arbiter tip-amend) but **not** on the silent mid-chain rebuild. Flag for breakdown confirmation; the §13.6.5 owner's input matters.
3. **`prepare-commit-msg` source argument.** FR-V2 specifies `prepare-commit-msg <msg-file> ""` with an **empty source** (identical to a plain `git commit`). Confirm this is the right invocation so foreign hooks behave as on a normal commit (not `merge`/`template`/etc.).
4. **Message-file lifecycle.** stagecoach writes the generated+deduped+`--edit`-finalized message to a temp file, runs `prepare-commit-msg` then `commit-msg` over it, reads it back as the final message, then passes it to `commit-tree -m`. Confirm the temp file location (`os.TempDir` or `.git/`) and that the read-back strips nothing git wouldn't.

---

## Requirements (implementation work, grouped for the breakdown agent)

### R1 — Config + CLI surface: `NoVerify`, `HookTimeout`, `--no-verify` flag (FR-V5, FR-V6)
- Add `NoVerify bool` to the resolved `Config` (`internal/config/config.go`) with full 5-layer precedence (flag > env > git-config > file > default). Mirror the **`Push`** field exactly — it is the established 5-layer bool (`toml:"push"`, `config.Defaults()` sets `false`, `file.go` `Push bool` + overlay `if g.Push { c.Push = true }` + merge `if src.Push { dst.Push = true }`). `NoVerify` carries the same only-true-propagates limitation as `Push`/`AutoStageAll` (accepted v1 convention; surface in `docs/configuration.md`).
- Add `HookTimeout time.Duration` (TOML `hook_timeout`) to the `[generation]` block. Mirror the **`Timeout`** field's plumbing (it is a `time.Duration` resolved through the 5 layers). `config.Defaults()` sets `10 * time.Minute` (FR-V6). Add the file-config struct field, the load/overlay/merge guards (duration-zero sentinel, mirroring `Timeout`'s `!= 0` handling).
- **CLI flag** `--no-verify` in `internal/cmd/root.go` (next to `--push`/`--edit` at lines 200–212): `pf.BoolVar(&flagNoVerify, "no-verify", false, "…mirrors git commit --no-verify; skips pre-commit and commit-msg (prepare-commit-msg and post-commit still run). (env STAGECOACH_NO_VERIFY, git stagecoach.no_verify; default false.) (§9.25 FR-V5)")`. Wire `flagNoVerify` into `cfg.NoVerify` resolution the same way `flagPush` → `cfg.Push` is wired (via `fs.Changed` / the config.Load path).
- *Mode A doc (rides with this requirement):* `docs/cli.md` — add the `--no-verify` flag row (env `STAGECOACH_NO_VERIFY`, git `stagecoach.no_verify`, default false) to the global-flags table, with the precise "skips pre-commit + commit-msg only" semantics.
- *Mode A doc (rides with this requirement):* `docs/configuration.md` `[generation]` table — add `hook_timeout` (duration, default `10m`, one-line purpose: per-hook execution timeout). Update the commented `[generation]` template block. Add `no_verify` to the `[defaults]`/flag surface docs if that section lists bool flags.

### R2 — Git primitives for snapshot-scoped hook execution (FR-V3)
- The commit-hook runner needs to materialize a throwaway index from a frozen tree, run a hook against it, and detect/accept mutations. Add the minimal git primitives to `internal/git` (or confirm the existing ones suffice with a `GIT_INDEX_FILE` env):
  - **Scoped read-tree + write-tree:** materialize `<tree>` into a temp index (`GIT_INDEX_FILE=<tmpfile> git read-tree <tree>`), and after the hook, `GIT_INDEX_FILE=<tmpfile> git write-tree` to capture the post-hook tree. If the existing `ReadTree`/`WriteTree` (`internal/git/git.go:492,1222`) hardcode `.git/index`, add a `GIT_INDEX_FILE`-aware variant (e.g. an options struct or a dedicated `ReadTreeInto`/`WriteTreeFrom` pair) rather than mutating global env.
  - **Subset enforcement (FR-V3 hard error):** after `pre-commit` runs, compare the post-hook tree's path set against the snapshot's path set via the existing `DiffTreeNameStatus(ctx, treeA, treeB)` (`git.go:1813`). Any path present in the post-hook tree but **not** traceable to the snapshot (`treeSHA` on single, `tree[i]` on decompose) is a concurrent-work sweep → hard error (abort, non-rescue-ish: the snapshot tree is orphaned, HEAD/index untouched). This mirrors the stager's `FR-M1c` discipline; reuse the same error-reporting shape.
  - **Re-tree on permitted mutation (FR-V3):** if `pre-commit` mutated only snapshot paths (formatter re-staged), the post-hook tree differs from the snapshot but is subset-valid → use the post-hook tree as the commit's tree (git-commit parity: the commit reflects the hook's output).
- This is pure git plumbing (no agent, no network); unit-test with a temp repo + a script hook that `git add`s a file (permitted if in snapshot, rejected if not).

### R3 — The commit-hooks runner + wiring into both commit paths (FR-V1, V2, V3, V4, V6, V7, V8)
- **New module** (e.g. `internal/hooks`, distinct from `internal/hook` which is the §9.20 *hook mode*; or a new file `internal/hook/run.go` — the breakdown agent picks): a `RunCommitHooks(ctx, ...) (finalTree, finalMsg string, err error)` that executes the sequence `pre-commit` → `prepare-commit-msg` → `commit-msg` (returns before `commit-tree`), designed to be called by **both** commit chokepoints. Discover hooks via the existing `git.HooksPath(ctx)` (`git.go:1752`); an absent/non-executable hook is silently skipped (git parity, FR-V1).
  - **`--no-verify`** (FR-V5): when `cfg.NoVerify`, skip `pre-commit` and `commit-msg`; `prepare-commit-msg` and `post-commit` still run.
  - **Recursion prevention** (FR-V4): before running `prepare-commit-msg`, detect stagecoach's own hook via `hook.Detect(hooksDir)` / `hook.Marker` (`internal/hook/hook.go`); if it is stagecoach's, **skip** it on the plumbing path (the message is already generated). Foreign `prepare-commit-msg` runs and may annotate; stagecoach reads the message file back as the final message.
  - **Environment + timeout** (FR-V6): each hook exec'd with `GIT_DIR`, `GIT_INDEX_FILE` (the R2 scoped index for `pre-commit`; unset/stdin for the `*-msg` hooks per git convention), worktree as CWD, message-file as arg 1 for `*-commit-msg`, stdin `/dev/null`; stdout/stderr pass through verbatim; bounded by `cfg.HookTimeout`.
  - **Failure → rescue** (FR-V7): non-zero exit / timeout from `pre-commit`/`prepare-commit-msg`/`commit-msg` returns a `*generate.RescueError{Kind: ErrRescue, TreeSHA, ParentSHA, Candidate, Cause}` — **byte-identical** to a generation-failure rescue (the existing `FormatRescue` / exit-3 path fires unchanged, because no `update-ref` ran). The hook's stderr is surfaced (the user's hook's own diagnostic).
- **Wire into `generate.CommitStaged`** (`internal/generate/generate.go`): insert `RunCommitHooks` **between** the `EditMessage` gate (line 389) and `CommitTree` (line 399). Pass the frozen `treeSHA` (scoped pre-commit per FR-V3) and the finalized `msg`. Use the returned `finalTree`/`finalMsg` for `CommitTree`. **`post-commit`** runs after `UpdateRefCAS` succeeds (line 410, in the success path before `signal.ClearSnapshot` at line 428), best-effort: a non-zero exit is logged as a warning via `deps.Verbose`, never undoes the commit (FR-V7). Gate `pre-commit`/`post-commit` out under `--dry-run` (FR-V8a); run `commit-msg` on the would-be message so the user still sees lint results. (`--dry-run` is known at the cmd layer via `flagDryRun`; thread it into `CommitStaged`'s path or the cmd layer's dry-run branch — confirm where the cleanest seam is.)
- **Wire into `decompose.publishCommit`** (`internal/decompose/message.go:219`): `publishCommit` is the single chokepoint for all decompose commits — `runSingleShortcut` (decompose.go:390), `runOneFileShortcut` (:336), the main loop publish closure (:484), and the arbiter resolution. Extend `publishCommit` (or add a wrapping helper it calls) to run `RunCommitHooks` between receiving `(tree, parentSHA, msg)` and `CommitTree`, scoped to that commit's `tree` (per-concept pre-commit, FR-V8c). `post-commit` after `UpdateRefCAS`. (`runSingleEscape` delegates to `generate.CommitStaged`, so it is covered by the `CommitStaged` wiring — do not double-run.) **Resolve open question #2** (arbiter mid-chain rebuild): per the recommended interpretation, the silent mid-chain rebuild skips hooks (fidelity risk); the arbiter's new-commit/tip-amend paths run them.
- *Mode A doc (rides with this requirement):* `docs/how-it-works.md` — add a new subsection "Commit hooks on the plumbing path" under the snapshot-flow section: hooks run in git's order around every `stagecoach` commit, scoped to the frozen snapshot (freeze holds); `--no-verify` skips pre-commit/commit-msg; a hook abort is a rescue; hook mode (§9.20) is still the bridge for plain `git commit`. Cross-link §9.25.

### R4 — Tests (acceptance proof)
- **Unit (`internal/hooks` or `internal/hook/run_test.go`):** hook ordering (pre → prepare-msg → commit-msg, then post after publication); `--no-verify` skips pre-commit + commit-msg only (prepare-msg + post still run); recursion prevention (stagecoach's own `prepare-commit-msg` Marker-detected → skipped; a foreign one runs); absent/non-executable hook silently skipped; per-hook timeout → failure.
- **Snapshot-freeze unit (the FR-V3 invariant, temp repo + script hooks):** (a) a `pre-commit` that `git add`s a file **in** the snapshot (formatter) → commit lands with the mutated tree (git-commit parity); (b) a `pre-commit` that stages a path **not** in the snapshot → hard error, HEAD + index unchanged; (c) a sentinel file the user "staged during generation" (written to the live index after `write-tree`) is **not** swept into the commit (the scoped index excludes it) — the stage-while-generating property holds under hooks.
- **Integration (stub-free, temp repo + real script hooks):** full sequence around a `generate.CommitStaged` commit (hooks run, commit lands, `post-commit` fires); a `pre-commit` exiting non-zero → `*RescueError{Kind: ErrRescue}` (exit 3), `FormatRescue` prints the frozen `TREE_SHA` recipe, HEAD/index byte-for-byte unchanged (assert via `git diff --cached --name-only` before/after — the existing idempotent-index pattern from `generate_test.go`); decompose per-concept (each concept's `pre-commit` sees only that concept's tree subset); `--dry-run` (pre/post-commit skipped, commit-msg runs); `--edit` (hooks run on the post-editor message). Reuse the existing `internal/stubtest` temp-repo harness shape where applicable (the hooks here are real shell scripts, not the agent stub).

---

## Documentation impact

**Mode A (doc-with-work, rides with the implementing requirement — noted as sub-bullets above):**
- `docs/cli.md` ← R1 (the `--no-verify` flag row).
- `docs/configuration.md` `[generation]` table + `[defaults]`/flag surface ← R1 (`hook_timeout` + `no_verify`).
- `docs/how-it-works.md` snapshot-flow section ← R3 (new "Commit hooks on the plumbing path" subsection).

**Mode B (changeset-level, depends on R1–R4 — the breakdown agent should emit this as a final Task):**
- **`docs/how-it-works.md` §"Hook mode vs the snapshot-based flow" (line 303) — REWRITE (this is the headline doc change).** The current text explicitly says the snapshot flow *"Bypasses pre-commit hooks"* and frames hook mode as the way to get hooks — **both now false**. Rewrite to: the snapshot flow is atomic + stage-while-generating **and honors the repo's hooks** (`--no-verify` skips pre-commit/commit-msg); hook mode (§9.20) remains for plain `git commit` from IDEs (hooks honored there too, but no snapshot guarantees). Update "When to use which": the two modes now compose (hook mode covers `git commit`; the snapshot flow covers `stagecoach`), and a user who always invokes `stagecoach` gets hook coverage without installing hook mode.
- **`README.md`** — add a one-line feature mention (hooks run on every `stagecoach` commit; `--no-verify` mirrors git) in the feature surface / safety area. Keep it to a sentence.
- **`docs/cli.md` global-flags table** is covered under R1 (Mode A); the Mode B task only needs to confirm consistency, not duplicate.

---

## Reference to completed work (do not re-implement)

- **Rescue / CAS / lock / signal are DONE and must be reused unchanged.** A hook failure returns the existing `*generate.RescueError{Kind: ErrRescue}` (`internal/generate/generate.go`); `exitcode.For(err)` already maps it to exit 3; `FormatRescue` prints the frozen-`TREE_SHA` recipe unchanged. **Do not** add a new exit code or rescue variant (FR-V7).
- **`git.HooksPath(ctx)`** (`internal/git/git.go:1752`) resolves the hooks dir via `git rev-parse --git-path hooks` (honors `core.hooksPath` + worktrees) — the same resolver `hook` mode's installer uses (FR-H1). Reuse it; do not re-derive.
- **`hook.Marker` / `hook.Detect(hooksDir)` / `hook.HookFilename`** (`internal/hook/hook.go`) detect stagecoach's own `prepare-commit-msg` — reuse for FR-V4 recursion prevention (skip our hook on the plumbing path).
- **`git.ReadTree` / `WriteTree` / `OverlayTreePaths` / `DiffTreeNameStatus`** (`internal/git/git.go:492,1222,1643,1813`) are the scoped-index + subset-check primitives (R2 builds on them). `DiffTreeNameStatus` is the subset-enforcement check (FR-V3 hard error), mirroring the stager's `FR-M1c`.
- **Config patterns:** `Push` (5-layer bool) is the exact template for `NoVerify`; `Timeout` (5-layer duration) is the template for `HookTimeout`. The `--push`/`--edit` flag wiring in `internal/cmd/root.go` (lines 200–212) is the template for `--no-verify`.
- **`generate.CommitStaged`** (`internal/generate/generate.go`) and **`decompose.publishCommit`** (`internal/decompose/message.go:219`) are the two commit chokepoints — wire `RunCommitHooks` into them; do not fork the commit logic. `runSingleEscape` already delegates to `CommitStaged`, so it is covered once.

## Risks / notes for the implementer
- **The scoped-index mechanism (open question #1) is the single hardest part and the most likely to ship a silent freeze-violation.** If `pre-commit` accidentally sees the live index, a file the user staged during generation gets swept into the commit — the exact bug the snapshot core exists to prevent. The subset check (R2) is the backstop, but the scoped index must be correct by construction. Test the "staged-during-generation" sentinel case explicitly (R4).
- **`--no-verify` parity must be exact.** `git commit --no-verify` skips `pre-commit` and `commit-msg` but **not** `prepare-commit-msg` or `post-commit`. Getting this wrong (e.g. skipping all four) breaks husky/lint-staged users who expect `prepare-commit-msg` to still run. Assert the exact skip set in R4.
- **Arbiter mid-chain rebuild (open question #2) has a fidelity risk.** Running a foreign `prepare-commit-msg` on a verbatim-reused `msg[j]` could mangle it, breaking the §20.2 "mid-chain amend fidelity" invariant. Lean toward skipping hooks on the silent rebuild; confirm with the §13.6.5 owner.
- **No new exit codes, no rescue-message changes, no CLI surface beyond `--no-verify`.** The feature is on by default (hooks run); `--no-verify` is the deliberate exception. The only other user-visible surfaces are `hook_timeout` (config) and the `post-commit` warning (verbose).
