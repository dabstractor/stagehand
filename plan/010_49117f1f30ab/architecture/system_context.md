# System Context — Hook execution on the commit path (v2.4, FR-V1–V8)

## What this feature is

This delta adds **hook execution on the plumbing commit path** (PRD §9.25, → G22). Until now,
stagecoach created commits via git *plumbing* (`write-tree` → `commit-tree` → `update-ref` CAS),
which runs **no hooks** — so a user's `pre-commit` (husky, lint-staged, a formatter),
`commit-msg` (conventional-commit lint), and `post-commit` never fired on a `stagecoach` commit.

The v2.4 feature **threads the repository's standard commit hooks** between generation and
`commit-tree`/`update-ref`, in git's documented order, **without surrendering the snapshot-based
atomic-commit core** (§8, §13). The new `--no-verify` flag mirrors `git commit --no-verify`.

This is a single cohesive feature, entirely unimplemented (verified: zero `NoVerify`/`hook_timeout`
symbols in `internal/`, `cmd/`, or `docs/`).

## The two invariants the feature must NOT break

1. **The stage-while-generating freeze (§5 property 1, FR-M1b).** Files the user stages *during*
   generation must NEVER be swept into the in-flight commit. `pre-commit` traditionally reads the
   live `.git/index`; by hook-time the live index may contain those later-staged files. So `pre-commit`
   must run against the **frozen snapshot tree**, materialized into a throwaway index — never the live
   one. This is the single hardest mechanism problem (see `external_deps.md` §7 and
   `codebase_reality.md` §3).

2. **The CAS atomic-commit core (§13.2, §18.1).** HEAD moves only at the final `update-ref` CAS; a
   hook abort (pre/prepare/commit-msg non-zero) must be a **pre-`update-ref` rescue** — HEAD and the
   index byte-for-byte unchanged, the existing `*generate.RescueError{Kind: ErrRescue}` (exit 3)
   fires unchanged. No new exit code, no new rescue variant.

## The exact insertion points (verified by reading source)

There are **three** commit chokepoints across the two commit paths:

| Chokepoint | File:line | Path | Role |
|---|---|---|---|
| `generate.CommitStaged` | `internal/generate/generate.go:389→399→410` | single-commit, COMMIT | EditMessage → [HOOKS] → CommitTree → UpdateRefCAS |
| `pkg/stagecoach.runPipeline` | `pkg/stagecoach/stagecoach.go:411` (self-contained DryRun/SystemExtra path) | single-commit, DRY-RUN + SystemExtra | mirrors CommitStaged; needs commit-msg only (FR-V8a) |
| `decompose.publishCommit` | `internal/decompose/message.go:219` | multi-commit (every per-concept + arbiter new/tip commit) | receives (tree, parentSHA, msg) → CommitTree → UpdateRefCAS |

`generate.CommitStaged` is the canonical pipeline (lines 389 = EditMessage gate, 399 = CommitTree,
410 = UpdateRefCAS). The hooks insert **between 389 and 399** (pre→prepare→commit-msg), with
**post-commit after 410 succeeds** (line 428 = `signal.ClearSnapshot`, best-effort).

`decompose.publishCommit` is the single chokepoint for ALL decompose commits: the main loop
(`decompose.go:484`), `runSingleShortcut` (`:390`), and `runOneFileShortcut` (`:336`) all funnel
through it. The arbiter resolution (`resolveNewCommit` chain.go:81, `resolveTipAmend` :137) produces
user-facing commits and SHOULD run hooks; **`resolveMidChain` (chain.go:186) is the silent
deterministic rebuild that reuses `msg[j]` verbatim and MUST SKIP hooks** (open Q#2 — see
`open_questions.md` — the fidelity risk is confirmed by research: a foreign prepare-commit-msg on a
verbatim-reused message would break the §20.2 "mid-chain amend fidelity" invariant).

`runSingleEscape` delegates to `generate.CommitStaged`, so it is covered by the CommitStaged wiring
(do not double-run).

## What is reused UNCHANGED (do not re-implement)

- **`*generate.RescueError{Kind: ErrRescue}`** + `FormatRescue` + `exitcode.For` → exit 3. A hook
  failure is byte-identical to a generation failure (FR-V7). No new exit code.
- **`git.HooksPath(ctx)`** (`git.go:1752`) — resolves the hooks dir via `git rev-parse --git-path
  hooks` (verified to honor `core.hooksPath` on git 2.54.0; see `external_deps.md` §4 for the
  historical gotcha). Same resolver as hook-mode install (FR-H1).
- **`hook.Marker` / `hook.Detect(hooksDir)` / `hook.HookFilename`** (`internal/hook/hook.go:62`,
  `script.go:15`) — detect stagecoach's OWN `prepare-commit-msg` for recursion prevention (FR-V4: skip
  it on the plumbing path so the message isn't regenerated/recurse).
- **`git.ReadTree` / `WriteTree` / `OverlayTreePaths` / `DiffTreeNameStatus` / `DiffTreeNames`**
  (`git.go:1222/492/1643/1813/1582`) — the index/object primitives. `DiffTreeNames` is the subset
  check (FR-V3 hard error). These operate on `.git/index` by default and need scoped variants (see
  `codebase_reality.md` §3).
- **Config patterns**: `Push` (5-layer bool, `config.go`) is the exact template for `NoVerify`;
  `Timeout` (5-layer duration) is the template for `HookTimeout`. The `--push`/`--edit` flag wiring
  (`internal/cmd/root.go:200-212`) is the template for `--no-verify`.
- **The run lock, signal handler, and CAS** — untouched. Hooks thread between generation and
  commit-tree; the core commit mechanics are unchanged.

## Module placement decision

The §9.20 hook *MODE* lives in `internal/hook/` (install/uninstall/`exec` — the prepare-commit-msg
bridge for plain `git commit`). This new feature is **commit-path hook execution** — a distinct
concern (running the repo's hooks around a plumbing commit, not installing a message-filling hook).

**Recommendation: new module `internal/hooks/`** (distinct from `internal/hook/`) to avoid confusing
the two. The runner imports `internal/hook` only for `Marker`/`Detect` (recursion prevention) and
`internal/git` for the plumbing primitives + the new scoped-index helper. This keeps the
install/execute-mode concerns (`internal/hook`) cleanly separate from the
run-the-repo's-hooks-around-our-commit concern (`internal/hooks`).

(Alternative — a new file `internal/hook/run.go` inside the existing `hook` package — is viable but
risks muddying the package's single responsibility. The `internal/hooks` plural is the clearer
choice and matches the PRD's "the §9.20 *hook mode*" vs "commit-path hooks" distinction.)
