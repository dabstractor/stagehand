# Open Questions & Recommended Resolutions

The §9.25 spec is decisive on *behavior*. These are *mechanism* questions for the implementing
agent, resolved as far as planning allows; the rest carry a clear recommendation + a verification step.

## 1. The scoped-index mechanism (the single hardest part) — RESOLVED approach

**Problem**: `pre-commit` traditionally reads `.git/index`. By hook-time, the live index may contain
files the user staged DURING generation (violating the freeze if pre-commit saw them), and on the
decompose per-concept path the live index holds the full accumulation, not just concept *i*'s tree.
So `pre-commit` must run against a **throwaway index materialized from the frozen tree**.

**Approach (recommended)**: two-layer.
1. **Git layer** (`internal/git`): add an env-passing exec seam + thin scoped variants:
   - `runWithEnv(ctx, repo string, extraEnv []string, args ...string) (...)` — sets
     `cmd.Env = append(os.Environ(), extraEnv...)`. The ONLY new exec seam; mirrors `run()` exactly
     except for the env. (Verify it doesn't break the documented "inherits parent env" guarantee —
     it ADDS to os.Environ(), so it's a superset.)
   - `ReadTreeInto(ctx, tree, indexFile string) error` — `GIT_INDEX_FILE=<indexFile> git read-tree
     <tree>` via runWithEnv. (Or a single `RunHookAgainstTree` helper — see below.)
   - `WriteTreeFrom(ctx, indexFile string) (sha, error)` — `GIT_INDEX_FILE=<indexFile> git write-tree`.
   - Reuse the existing `DiffTreeNames` (read-only, no index) for the subset check — it compares two
     tree SHAs, so it's index-agnostic already.
2. **Runner layer** (`internal/hooks`): a higher-level helper that owns the throwaway index lifecycle:
   `RunPreCommit(ctx, g, treeSHA, hookPath, env, timeout) (postTree string, err error)`:
   - `indexFile := filepath.Join(os.TempDir(), "stagecoach-hook-<random>")`; `defer os.Remove(indexFile)`.
   - `g.ReadTreeInto(ctx, treeSHA, indexFile)`.
   - exec `hookPath` with `env = GIT_INDEX_FILE=<abs indexFile>, GIT_EDITOR=:, GIT_DIR=<gitdir>`,
     CWD=worktree, stdin=/dev/null, stdout/stderr pass-through, bounded by `cfg.HookTimeout`.
   - `postTree, _ := g.WriteTreeFrom(ctx, indexFile)` (capture hook-staged fixes).
   - subset check: `added, _ := g.DiffTreeNames(ctx, treeSHA, postTree)` — any path in `postTree`
     not traceable to `treeSHA` (or its ancestor content)... actually the subset check is: the
     post-hook tree's changed paths must all be ⊆ the snapshot's paths. Use `DiffTreeNameStatus` to
     enumerate what the hook changed; if any ADDED path is not in the snapshot's path set → hard
     error (FR-V3). (A formatter modifying an existing snapshot path is permitted → re-tree.)

**Why two layers**: keeps the git primitives composable + unit-testable (the scoped variants are
thin and obviously correct), and the runner composes them (the lifecycle/subset logic lives in one
place). The alternative — one monolithic helper in the git package — blurs the git/exec boundary.

**The working-tree-coupling tension** (external_deps.md §8) is INHERENT and ACCEPTED. Do not
snapshot the working tree. The subset check is the backstop. Document the divergence in
`docs/how-it-works.md`.

## 2. Arbiter mid-chain rebuild hooks — RESOLVED: SKIP hooks

`resolveMidChain` (chain.go:186) rebuilds the linear chain `i..N-1` via `OverlayTreePaths` +
`CommitTree` reusing `msg[j]` **verbatim**. These are deterministic reconstructions, not
user-facing "new" commits.

**Recommendation (confirmed by research)**: the mid-chain rebuild **SKIPS hooks**. Running a foreign
`prepare-commit-msg` on a verbatim-reused `msg[j]` would risk mangling it, breaking the §20.2
"mid-chain amend fidelity" invariant (non-target commits must be byte-identical). The arbiter's
`resolveNewCommit` (N+1 commit) and `resolveTipAmend` (tip amend) DO run hooks (user-facing commits).

**Implementation**: `resolveMidChain` does its OWN `CommitTree`+`UpdateRefCAS` (chain.go:203/215) —
it does NOT go through `publishCommit`. So the cleanest implementation is: wire the hook runner into
`publishCommit` (which `resolveNewCommit`/`resolveTipAmend` could route through OR call directly
after), and simply do NOT wire it into `resolveMidChain`. Confirm `resolveNewCommit`/`resolveTipAmend`
route their commits through the hook-bearing path.

## 3. `prepare-commit-msg` argv: `""` vs absent — VERIFY at implementation

The PRD FR-V2 says `prepare-commit-msg <msg-file> ""` (2 args, empty source). The researcher flags
this as likely a shell-`$2` fallacy — git probably passes 1 arg (file only) for a plain commit.

**Resolution**: run the 5-second test (external_deps.md §2), emit the matching argv, record the
verified form. Pass `""` if the test shows `argc=2`; pass file-only if `argc=1`. Either is git parity.
**The PRD's `""` is the conservative default** (it's what the FR specifies) — only deviate if the test
definitively shows 1 arg.

## 4. Dry-run hook composition (FR-V8a) — the runPipeline chokepoint

FR-V8a: under `--dry-run`, pre-commit and post-commit are skipped (nothing is committed); commit-msg
runs against the would-be message so the user sees lint results.

**The subtlety**: the single-commit path has TWO implementations:
- `generate.CommitStaged` (commit path, `!DryRun`)
- `pkg/stagecoach.runPipeline` (dry-run + SystemExtra path)

The delta_prd's R3 named only `CommitStaged`. **The implementing agent must handle runPipeline's
dry-run branch**: under DryRun, run `commit-msg` (and prepare-commit-msg for recursion-prevention +
annotation) on the would-be message, but NOT pre-commit or post-commit.

**Cleanest seam**: the hook runner takes a `mode` (commit vs dry-run) or a `skipPreCommit bool` +
`skipPostCommit bool`. Under dry-run: `skipPreCommit=true, skipPostCommit=true`, commit-msg runs.
Confirm runPipeline is the only dry-run single-commit path (it is — `GenerateCommit` delegates to it
for DryRun at stagecoach.go:163).

## 5. `prepare-commit-msg` recursion: detect via `hook.Detect`, skip if StatusStagecoach

Before running `prepare-commit-msg`, call `hook.Detect(hooksDir)`. If `StatusStagecoach`, skip it
(the installed hook would `exec stagecoach hook exec`, regenerating/recursing). If `StatusForeign`,
run it and read the message file back. If `StatusNone`, there's no prepare-commit-msg — skip.

**Edge**: a foreign prepare-commit-msg that exits non-zero → abort the run (git parity, FR-V2). A
stagecoach-owned one is skipped entirely (no exit-code consideration).

## 6. Message-file location + read-back

The runner writes the generated+deduped+`--edit`-finalized message to a temp file, runs
prepare-commit-msg then commit-msg over it, reads it back (stripping `#`-comment lines per
`core.commentChar`), and passes the result to `commit-tree` (via `CommitTree`'s stdin `-F -`).

**Location**: `os.TempDir()` (not `.git/` — keeps the repo clean; matches the throwaway index). Or
`.git/STAGECOACH_HOOKMSG` (like the existing `.git/STAGECOACH_EDITMSG` for --edit). **Recommendation**:
`os.TempDir()` + `defer os.Remove`, mirroring the throwaway index lifecycle. Read-back strips
`#`-comment lines (honor `core.commentChar` — default `#`).

## 7. `hook_timeout` config surface — file + default only

FR-V6 specifies `[generation].hook_timeout` (default 10m). The PRD §15.2 flag table does NOT list a
`--hook-timeout` flag, and there's no `STAGECOACH_HOOK_TIMEOUT` env in FR-V6. **Decision**: `HookTimeout`
is config-file (`[generation].hook_timeout`) + default only (mirrors `multi_turn_chunk_tokens`). No
env/flag/git-config. If a per-run override is wanted later, add a flag then. Keep the surface minimal.
