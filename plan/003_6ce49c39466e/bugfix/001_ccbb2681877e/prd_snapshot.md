# Bug Fix Requirements

## Overview

End-to-end validation of the v3 changeset (P1–P4: manifest/Render/config v3, qwen-code provider +
model-token refresh, decompose concurrency hardening, integration docs) against the original PRD scope.

Testing performed:
- Full build + full unit/integration test suite (`go test ./...`) — all green.
- E2E scenario harness (`go test -tags e2e ./internal/e2e/...`) — all green.
- Targeted adversarial probing of the FR-R5b model-prefix fold, FR-R6 reasoning rendering, config v2→v3
  migration, the FR-M11 single shortcut, the FR-M2b one-file short-circuit, FR-M1b/M1c freeze behavior,
  and per-role config resolution — using throwaway probe tests against real temp git repos.

Overall quality assessment: the architecture is sound and the test coverage is strong. The v2→v3
model-prefix migration, the FR-D1 provider priority (incl. qwen-code), the freeze (T_start) capture +
enforcement, and the one-file short-circuit all work correctly. However, three **Major** issues were
found: (1) the headline FR-R6 reasoning feature ships completely inert for every provider, (2) the
single-commit path silently ignores per-role `message` overrides (FR-R3 violation), and (3) the FR-M11
planner-single shortcut leaves the repository in a confusing, inconsistent `git status` state.

---

## Critical Issues (Must Fix)

None found that fully block core commit generation. The three issues below are classified **Major**
because each breaks a documented requirement or invariant on a real user path, but none corrupts
history or loses data.

---

## Major Issues (Should Fix)

### Issue 1: The FR-R6 reasoning feature ships completely inert — no provider defines `reasoning_levels`

**Severity**: Major
**PRD Reference**: §9.15 FR-R6 (P0), §12.1 `reasoning_levels` manifest table, §16.4, §15.2 (`--reasoning`,
`STAGEHAND_REASONING`); task P1.M1.T1.S1 (required: "populate ReasoningLevels for pi/claude with
thinking-effort tokens").
**Expected Behavior**: Each role's resolved `reasoning` level (off|low|medium|high) is rendered via the
provider's `reasoning_levels` manifest table (appended at `Render`). The shipped default
`planner = high` should emit the planner provider's `high` reasoning tokens (e.g. a thinking-effort
flag). `--reasoning high` / `--planner-reasoning high` / `[role.planner] reasoning = "high"` /
`STAGEHAND_REASONING=high` should have an observable effect for providers that support it.
**Actual Behavior**: **None** of the eight built-in providers (`pi`, `claude`, `gemini`, `agy`,
`qwen-code`, `opencode`, `codex`, `cursor`) populate `ReasoningLevels` — every manifest ships the field
as `nil`. As a result `Render`'s reasoning-token branch (`if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`)
is a no-op for every provider. Concretely:
  - The shipped default `planner = high` (`internal/config/roles.go` `defaultRoleReasoning`) is
    **meaningless** — it changes nothing about how the planner is invoked.
  - `stagehand --reasoning high` (advertised in `README.md` line 121–122: "Use reasoning for deeper
    analysis on the planner") does nothing.
  - Every `--<role>-reasoning` flag, `STAGEHAND_<ROLE>_REASONING` env var, and `[role.*].reasoning`
    config key is silently dropped.
  - `docs/cli.md` documents the flags and the "(off; planner: high)" default, but neither has any effect.
**Steps to Reproduce** (probe result): `Manifest.Render("zai/glm-5.2", sys, user, "high")` on `pi`
produces `args=[--provider zai --model glm-5.2 ...]` with **no** reasoning token (e.g. no
`--thinking`). Iterating `provider.BuiltinManifests()` confirms every provider's `ReasoningLevels` is
nil/empty.
**Note**: FR-R6 does permit a *graceful per-invocation no-op* when a provider/model lacks reasoning
control, and FR-D5 defers exact-token verification — so shipping *without* tokens is partially
defensible. However, task P1.M1.T1.S1 explicitly required populating pi/claude tokens (and was marked
Complete without doing so), the feature is advertised in the README/CLI docs, and the headline
`planner=high` default is inert. The reasoning feature as specified is non-functional out of the box.
**Suggested Fix**: Populate `ReasoningLevels` for the providers that support a thinking-effort control —
at minimum `claude` (`{"high": ["--thinking-effort", "high"], "medium": [...], "low": [...]}` per
`claude --help`) and `pi`. Leave genuinely non-reasoning providers nil (the graceful no-op then applies
honestly). Update the README so `--reasoning high` only promises an effect where it has one.

### Issue 2: Per-role `message` overrides (`--message-model`/`--message-provider`/`--message-reasoning`, `[role.message]`) are silently ignored on the single-commit path

**Severity**: Major
**PRD Reference**: §9.15 FR-R3 — "Every role exposes all three flags, including `message` — no role is a
special case that omits a flag (corrects the v2 gap that had no `--message-*` flags)."
**Expected Behavior**: The message role's provider/model/reasoning are resolved per-field
(flag > env > `[role.message]` > global `[defaults]` > manifest default) and used for the message
generation. On the single-commit path the active role is `message`, so a `--message-model X` (or
`[role.message] model = "X"`) must drive the generated commit's model.
**Actual Behavior**: The flag/env/file loaders correctly write `message` overrides into
`cfg.Roles["message"]` (`internal/config/load.go` `setRoleModel`/`setRoleProvider`/`setRoleReasoning`),
and the flags are registered (`internal/cmd/root.go`). But the single-commit path never calls
`config.ResolveRoleModel("message", cfg)`: `generate.CommitStaged` and `pkg/stagehand.runPipeline` both
call `deps.Manifest.Render(cfg.Model, …, cfg.Reasoning)` using the **global** `cfg.Model`/`cfg.Reasoning`
directly (`internal/generate/generate.go`, `pkg/stagehand/stagehand.go`). The CLI
(`internal/cmd/default_action.go` `runGenerate`) likewise passes only `cfg.Model`/`cfg.Provider`.
`ResolveRoleModel("message", …)` is only ever invoked by the *decompose* message role
(`internal/decompose/message.go`), so `--message-*` works on the multi-commit path but is silently
dropped on the default single path.
**Steps to Reproduce**: Probe confirmed that after `loadFlags` with `--message-model haiku` set,
`cfg.Model == ""` while `cfg.Roles["message"].Model == "haiku"`, and `ResolveRoleModel("message", cfg)`
returns `model="haiku"` — but the single path renders `cfg.Model` (`""` → manifest default), never
`"haiku"`. So `stagehand --message-model haiku` (with something staged) uses the default model, not haiku.
**Note**: The common case (no explicit `message` override) is unaffected because the message role
inherits the global. The bug only manifests when a user explicitly configures the message role — exactly
the use case FR-R3 says must work.
**Suggested Fix**: In the single-commit path, resolve the message role before rendering — e.g. compute
`(prov, model, reasoning) = config.ResolveRoleModel("message", cfg)` and pass `model`/`reasoning` to
`Render` (and use `prov`/the role's manifest for `buildDeps`). Apply this in both `generate.CommitStaged`
and `pkg/stagehand.runPipeline`, and have `runGenerate` pass the resolved message provider/model.

### Issue 3: The FR-M11 planner-single shortcut leaves `git status` in a broken/inconsistent state (missing index sync)

**Severity**: Major
**PRD Reference**: §9.14 FR-M11 (single-call shortcut), §13.6; §20.2 invariant "Loop index cleanliness —
after a fully-successful decompose run, `git status --porcelain` is empty (arbiter not needed) … never a
partial leftover"; §18.1 "leaves the repository byte-for-byte unchanged (modulo harmless dangling
objects)".
**Expected Behavior**: After a successful decompose run that commits everything (no leftovers), the
working tree/index should be clean relative to HEAD — `git status --porcelain` empty.
**Actual Behavior**: The T_start freeze (introduced in P3) resets the index to `baseTree`.
`runOneFileShortcut` correctly re-syncs the index to the committed tree with `deps.Git.ReadTree(ctx, tStart)`
after publishing (its comment calls this "CRITICAL (findings §4): … otherwise leave index==baseTree ≠
HEAD.tree ⇒ `git status` shows `MM`"). But the sibling path **`runSingleShortcut`** (FR-M11 — the planner
returns `single: true` with a message) commits `tStart` directly **without** that `ReadTree(tStart)` sync.
So after a successful run, `HEAD.tree == tStart` but `index == baseTree`, and `git status` reports the
just-committed files as **staged deletions plus untracked**:
**Steps to Reproduce** (verified in-process against a real temp repo, default **auto** mode,
2 un-staged files, planner returns `{"single":true,"message":"feat: add a and b"}`):
```
$ stagehand            # auto-decompose; planner judges one commit
↳ Created <sha>  feat: add a and b
$ git status --porcelain
D  a.txt
D  b.txt
?? a.txt
?? b.txt
$ git ls-tree -r --name-only HEAD     # …yet a.txt and b.txt ARE in the commit:
a.txt
b.txt
readme.md
```
The commit itself is correct (the files are in HEAD); only the index is stale. Reachable via the default
auto path whenever the planner returns `single:true` for a multi-file changeset (the one-file
short-circuit and `--single`/`--commits 1` escape-hatch are unaffected — they use `runOneFileShortcut`
and `runSingleEscape`, which both leave a clean index).
**Impact**: A user sees their freshly-committed files reported as "deleted + untracked" immediately after
a "successful" commit — strongly suggesting stagehand broke the repo. A naive user may re-run, re-add, or
panic. It directly violates the §20.2 loop-index-cleanliness invariant.
**Suggested Fix**: Add the same index-sync `runOneFileShortcut` uses to `runSingleShortcut` after
`publishCommit` succeeds:
```go
if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: single shortcut index sync: %w", ErrDecomposeFailed, err)
}
```
Place it before `buildCommitResult`/return, on the success path only (mirror `runOneFileShortcut`). Add a
decompose regression test asserting `git status --porcelain == ""` after a planner-`single` run on a repo
with a base commit (the existing `TestDecompose_SingleShortcut_CleanMessage` uses an unborn repo and does
not assert on status, so it misses this).

---

## Minor Issues (Nice to Fix)

### Issue 4: Bootstrap config header omits the `STAGEHAND_REASONING` / `STAGEHAND_<ROLE>_REASONING` env vars

**Severity**: Minor
**PRD Reference**: §9.8 FR35, §15.2.
**Expected Behavior**: The generated config's header documents all `STAGEHAND_*` env knobs, including the
reasoning ones.
**Actual Behavior**: `internal/config/bootstrap.go` `bootstrapHeader` documents
`STAGEHAND_PLANNER_PROVIDER`/`_MODEL` (and the other three roles' provider/model) and `STAGEHAND_COMMITS`,
but never lists `STAGEHAND_REASONING` (global) or `STAGEHAND_<ROLE>_REASONING`, nor `STAGEHAND_MAX_COMMITS`.
`docs/cli.md` does document them, so this is a header-only inconsistency.
**Suggested Fix**: Add the reasoning (and max-commits) env-var lines to `bootstrapHeader`.

### Issue 5: In-memory v2→v3 migration is a no-op for abandoned `[agent.*]` terminology

**Severity**: Minor
**PRD Reference**: §9.17 FR-B7 — "Files that went through the abandoned intermediate `agent`/`[agent.*]`
terminology are mapped back to `provider`/`[provider.*]` first, then step (b) applies."
**Expected Behavior**: A v2 file using `[defaults] agent = "pi"` / `[agent.pi]` loads with the agent
configuration remapped to `provider`.
**Actual Behavior**: `migrateV2ToV3` (`internal/config/migrate.go`) documents the agent→provider remap as
an explicit **in-memory no-op** (`fileConfig` has no `Agent` field and `toml.Unmarshal` silently drops
unknown `[agent.*]` tables). A file using the abandoned intermediate terminology therefore **loses** its
agent/provider block on load (only the on-disk `config upgrade` command rewrites it). Probe confirmed:
`[defaults] agent = "pi"` + `[agent.pi]` loads with `cfg.Provider == ""` and no `cfg.Providers["pi"]`.
**Impact**: Low — the `agent`/`[agent.*]` terminology was an abandoned intermediate and likely never
shipped in a release; `config upgrade` does handle it on disk. Noted for completeness.
**Suggested Fix**: Either accept the documented limitation (a user hitting it is told to run
`config upgrade`) or, for defense-in-depth, have `loadTOML`/`migrateV2ToV3` detect and remap `[agent.*]`
textually before the typed decode.

---

## Testing Summary

- Total tests performed: ~25 distinct probes/assertions across build, the full unit/integration suites,
  the e2e harness, and bespoke adversarial probes (FR-R5b, FR-R6, v2→v3 migration, agent remap, message-
  role routing, FR-M11/FR-M2b/`--single` post-run `git status`, freeze subset, provider priority/docs).
- Passing: all pre-existing unit/integration/e2e suites green; the three Major issues above are **not**
  caught by any existing test (each was reproduced with a new throwaway probe).
- Failing / newly found: 3 Major (Issues 1–3) + 2 Minor (Issues 4–5).
- Areas with good coverage: the v2→v3 model-prefix fold (global + per-role + raw provider map), FR-D1
  provider priority incl. qwen-code insertion, T_start freeze capture + FR-M1c subset enforcement,
  FR-M2b one-file short-circuit (incl. index sync), CAS-abort, single/loop rescue, arbiter reconciliation
  (tip/mid-chain/new), binary placeholders, e2e subprocess harness.
- Areas needing more attention:
  - `reasoning_levels` population + a render test proving reasoning tokens are emitted (Issue 1).
  - Single-commit path resolving the `message` role via `ResolveRoleModel` (Issue 2).
  - Index-sync parity between `runOneFileShortcut` and `runSingleShortcut`, plus a post-run
    `git status --porcelain == ""` assertion on the planner-`single` path with a base commit (Issue 3).
