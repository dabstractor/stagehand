# Stagecoach — Delta PRD: Config v3 (model-prefix inference provider) + Reasoning + Decompose Hardening

| Field | Value |
|---|---|
| **Delta scope** | Config schema v3, reasoning level (FR-R6), qwen-code provider, decompose start-of-run freeze (FR-M1b/M1c) + one-file short-circuit (FR-M2b), progress label (FR51b), E2E harness (§20.5) |
| **Base** | v2.0 specification — **fully implemented** in session 002 (P1–P4 Complete). This delta layers onto a working v2. |
| **Last updated** | 2026-07-01 |

---

## 0. Diff analysis & sizing

The diff between `plan/002_a17bb6c8dc1d/prd_snapshot.md` and the current `PRD.md` is a **medium-large, coherent feature set** (not a one-line tweak). It is dominated by **one architectural change** — folding the inference provider into the model string — plus several smaller additive features. Sizing is set accordingly: **4 phases, ~4 milestones, ~9 tasks**. This is *not* an inflated 9-phase PRD.

### What actually changed (grouped)

1. **Config v3 — inference provider folds into the model string (the headline).** The separate `default_provider` manifest/config field is **removed**. For multi-backend providers (pi, opencode) the inference backend becomes a slash-prefix on `model` (`zai/glm-5.2`). `provider` reverts to its original meaning: the **agent platform** (pi, claude, opencode, …). At `Render`, a `provider_flag` provider (pi) splits the model on `/` into `--provider <prefix> --model <rest>`; a bare model (no `/`) on such a provider is a **hard error**, never a silent bare `--model`. Authored as **FR-R5b (rewritten)**, **FR-B7 (new migration)**, and a §12 terminology rewrite. This *replaces* the v2 `(provider, model)` coupling that currently lives in role resolution (`internal/decompose/roles.go`, see tests `TestResolveRoles_FR5b_*`).

2. **Reasoning level per role (FR-R6, new).** New normalized `reasoning` level `off|low|medium|high`; new `reasoning_levels` manifest table; rendered at `Render`; new global + per-role config/flag/env; shipped defaults `planner=high`, `stager=message=arbiter=off`; **graceful no-op** when the agent/model lacks reasoning control.

3. **qwen-code provider (§12.5.2, new) + model-token refresh.** New single-backend built-in (Gemini-CLI fork for Qwen3-Coder); inserted into `preferredBuiltins` and the FR-D4 tier table. Several model tokens bumped to current (`gemini-2.5-pro`→`gemini-3.1-pro`, etc.). FR-D3 gains free-tier rationale.

4. **Decompose concurrency hardening (FR-M1b/M1c/M2b, new).** A **start-of-run freeze** captures the entire working-tree change set as `T_start`; the planner, every stager, the arbiter, and all shortcuts operate strictly on `T_start` so concurrent working-tree changes are excluded from every commit. **Freeze enforcement** makes any staged path/content not traceable to `T_start` a hard abort. A **one-file short-circuit** skips the planner entirely when exactly one file changed.

5. **Progress label format (FR51b, new) + E2E harness (§20.5, new).** The `↳` progress line becomes `<Verb> with <model> in <provider>…`. A throwaway-repo scenario harness is added as the regression net for the concurrency/routing invariants.

### Removed requirements (awareness only — no tasks)

- The manifest/config field **`default_provider`** is removed in v3 (its job moves to the model-string prefix). The `[provider.<name>] default_provider = "X"` v2 field and the `provider` parameter's "inference backend" meaning are gone. Migration (FR-B7) handles existing files.

---

## 1. Scope delta — requirements

Each requirement below names the **Mode A docs** it carries inline. A final **Mode B** changeset-level doc-sync requirement closes the delta (§2).

### Phase 1 — Manifest, Render & Config v3 (inference provider in model string + reasoning)

**R1.1 (FR-R5b rewritten, FR-B7) — Inference provider folds into the model string.** `provider` = agent platform (original meaning); for multi-backend providers the `model` carries the inference backend as a slash-prefix (`zai/glm-5.2`). The `default_provider` field is **removed** from the manifest schema and from config. At `Render` (`internal/provider/render.go`, the single command-emission chokepoint every call path flows through): if `provider_flag != ""` (pi — the only one today) and `model` is non-empty, split on the first `/` and emit `--provider <prefix> --model <rest>`; if `model` is non-empty but has no `/`, **return an error** ("include the inference provider, e.g. `zai/glm-5.2`") rather than emitting a bare `--model`. Providers without `provider_flag` (opencode, and every single-backend provider) pass the model verbatim. Role resolution re-checks the same invariant earlier for a role-named error. This **replaces** the current `DefaultProvider`-param logic in `render.go` (lines 96, 107–118) and the `(provider, model)` coupling in `internal/decompose/roles.go`.
  - *Mode A docs:* doc comments on `Manifest.Render` and the manifest struct; `providers/*.toml` reference files (remove `default_provider`); `internal/config` comments.

**R1.2 (FR-B7) — Config →v3 migration.** Bump `CurrentConfigVersion` to **3**. On load of a `config_version < 3` file, auto-migrate **in memory**: keep `[provider.<name>]`/`[defaults]`/`[role.*]` `provider` fields unchanged (platform names); for a multi-backend provider, prepend its former `default_provider` value to its model → `model="Y"` + `default_provider="X"` becomes `model="X/Y"` (global + per-role); drop `default_provider`. Emit a one-time deprecation notice pointing at `config upgrade`, which performs the same rewrite on disk. **No value is invented** — a pi model with no resolvable prefix stays bare and becomes an FR-R5b error. Files using the abandoned `agent`/`[agent.*]` intermediate terminology are mapped back to `provider`/`[provider.*]` first.
  - *Mode A docs:* `internal/config` comments; `config upgrade --help` text.

**R1.3 (FR-D2 update) — pi ships a blank default model.** `builtinPi()` `DefaultModel` is already `""` from v2; confirm it stays blank and that the manifest **omits** `default_provider` entirely (FR-R5b). The personal `zai/glm-5.2` setup is a documented *override* only. `config init` (FR-B1 update) must not invent a prefix — it detects or leaves blank with guidance.
  - *Mode A docs:* `providers/pi.toml`; pi manifest doc comment.

**R1.4 (FR-R6, new) — Reasoning level per role.** Add a `reasoning_levels` table to the manifest (`off=[]` plus optional `low`/`medium`/`high` token lists; absent/empty tokens ⇒ silent no-op). `Render` receives the role's resolved `reasoning` level and appends the declared tokens after the model flag (never an error if absent). Add `reasoning` to the config model: global `[defaults].reasoning` (`--reasoning` / `STAGECOACH_REASONING` / `stagecoach.reasoning`) and per-role (`--<role>-reasoning` / `STAGECOACH_<ROLE>_REASONING` / `[role.<role>].reasoning`). Extend role resolution to return `(provider, model, reasoning)` with the same per-field precedence, applied independently. **Every role exposes all three flags, including `message`** (corrects the v2 gap that had no `--message-*` flags). Shipped defaults: `planner=high`; `stager=message=arbiter=off`. `Roles` config field gains a `Reasoning` member; `fileRoleConfig` and `overlay()` follow.
  - *Mode A docs:* manifest struct doc comment (`reasoning_levels`); `internal/config` `RoleConfig` comment; CLI flag help.

### Phase 2 — qwen-code provider + model-token refresh

**R2.1 (§12.5.2, FR-D1, FR-D4 — new provider).** Add `builtinQwenCode()` (single-backend Gemini-CLI fork for Qwen3-Coder; mirrors `gemini`/`agy` flag surface; `provider_flag=""`; mark `experimental` until a real end-to-end run clears it). Insert `"qwen-code"` into `preferredBuiltins` between `gemini` and `codex` → `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`. Add the qwen-code row to the FR-D4 tier table. Create `providers/qwen-code.toml`. Update the `TestPreferredBuiltins_MatchesBuiltinKeys` test and the `providers list` default-order doc string.
  - *Mode A docs:* `providers/qwen-code.toml`; `docs/providers.md` reference table (note experimental + DashScope backend); Appendix D quick-reference.

**R2.2 — Model-token refresh + FR-D3 rationale.** Refresh FR-D4 tier-table tokens to current per FR-D5 (e.g. agy/gemini flagship `gemini-3.1-pro`; keep `# TO CONFIRM` discipline for qwen-code/cursor/codex). FR-D3 `message` tier gains the explicit "cheapest / free-tier-eligible" rationale. Update the FR-D5 open-questions list (qwen-code token, pi routing now "ship blank").
  - *Mode A docs:* manifest `default_model` tokens in `builtin.go` + matching `providers/*.toml`; `docs/providers.md`.

### Phase 3 — Decompose concurrency hardening

**R3.1 (FR-M1b/M1c, new) — Start-of-run freeze `T_start`.** The instant decomposition activates, capture an immutable snapshot of the entire working-tree change set (every modified/added/deleted/untracked path **and its byte content**) as a tree object `T_start` (the index is empty per FR-M1, so the change set is captured against HEAD). The planner partitions `T_start`'s diff — **never** a fresh re-read of the live tree; every stager, the arbiter's leftover staging, and the shortcuts stage content drawn **strictly** from `T_start`. Any file created/modified after `T_start` is captured is invisible to the run. **Freeze enforcement (defense-in-depth):** because the stager is an external agent running git against the live tree, after each staging step stagecoach verifies the resulting tree is a subset of `T_start` (only `T_start` paths, `T_start` content); any deviation (concurrent change swept in, or a stager that ran a bare `git add -A`) is a **hard abort** (non-rescue; already-landed commits stand per FR-M12). Update FR-M6 (per-concept loop draws from `T_start`) and FR-M11 (single-shortcut stages `T_start`, not `git add -A`). The orchestrator owns the freeze boundary (mirroring the HEAD-movement guard, §19).
  - *Implementation impact:* `internal/decompose/decompose.go` (orchestrator: capture `T_start` first), `planner.go` (diff `T_start`, not live), `stager.go` (enforce subset after each step). Likely a new git helper to build `T_start` (transient `add -A` → `write-tree` → restore index) — may extend `internal/git`.
  - *Mode A docs:* doc comments in `internal/decompose` documenting the freeze boundary.

**R3.2 (FR-M2b, new) — One-file short-circuit (auto mode).** In auto-decompose, if `git status --porcelain` shows **exactly one** changed path, bypass the planner entirely: stage that one file's `T_start` content, generate one message via the message role, commit. Deterministic (changed-path count), not model judgment. An explicit `--commits N` (N ≥ 2) overrides this and is honored even for a single file.
  - *Implementation impact:* `internal/decompose/decompose.go` (gate before planner call, using existing `StatusPorcelain`).
  - *Mode A docs:* `internal/decompose` comment.

### Phase 4 — Integration: progress label, E2E harness, changeset-level docs

**R4.1 (FR51b, new) — Progress label names the resolved invocation.** The `↳` progress line (stderr) becomes `<Verb> with <model> in <provider>…` — e.g. `Generating with zai/glm-5.2 in pi…`, `Generating with sonnet in claude…`, `Decomposing with anthropic/claude-sonnet-4 in opencode…`. The model string already carries the inference prefix (FR-R5b), so no special formatting is needed; when `model` is empty, show `<provider>` alone. On the single-commit path the surfaced role is `message`; for decompose the label surfaces the **planner** role's resolved config, and `--verbose` prints all four roles. This updates `internal/ui` progress usage and its call sites in `generate`/`decompose`.
  - *Mode A docs:* `internal/ui` `Progress` doc comment example.

**R4.2 (§20.5, new — strongly encouraged) — End-to-end scenario harness.** A throwaway-repo harness (`//go:build e2e` test or script) that, per scenario, creates a fresh `git init` temp repo, seeds it, runs `stagecoach`, and asserts the resulting history (driving the real agent where feasible, else a stub). Must-cover set: nothing-staged→N commits (auto *and* `--commits N`); exactly-one-file→single commit, **no planner call** (FR-M2b); concurrent mid-run file→excluded from every commit, left in working tree (FR-M1b/M1c); bare model on a multi-backend agent→**hard error**, not empty output (FR-R5b); arbiter reconciliation (new/tip-amend/mid-chain); rescue mid-loop; CAS abort. Extend the v2 stub decompose suite with the concurrent-change-exclusion assertion. **Every bug found in the wild becomes a scenario here.**
  - *Mode A docs:* none (test-only).

---

## 2. Documentation impact

**Mode A (ride with implementing work)** — named inline under each requirement above: manifest/Render/config doc comments, `providers/*.toml` reference files, `docs/providers.md`, Appendix D, CLI help text, decompose doc comments, UI `Progress` comment.

**Mode B (changeset-level — final sync task):**

**R-DOC. Sync changeset-level documentation** (depends on R1–R4). The config v3 model-prefix terminology, the reasoning feature, and the qwen-code provider change the user-visible surface described in `README.md` (hero pitch + the quick-start / config examples, which now use `model = "zai/glm-5.2"`, `reasoning`, and the model-prefix examples) and any top-level capability/config overview docs. Update:
- `README.md` — config examples and the model-prefix/`--reasoning` quick-start; provider list (add qwen-code).
- Top-level config/provider docs referenced from the README.
- **Reconcile a doc inconsistency introduced in the v3 edit:** §16.4's CLI example uses `--planner-agent agy` and the config snippet uses `agent = "pi"` / `agent = "agy"`, but FR-R3, FR-B7, and the §12 terminology table all standardize on **`provider`** (the `agent`/`[agent.*]` terminology is the abandoned intermediate that FR-B7 maps *back* to `provider`). Align those examples to `--planner-provider` / `provider = "…"` so the shipped docs are self-consistent.

---

## 3. Reference to completed work & prior research

- **v2 is implemented.** Build on the existing `Manifest` struct (`internal/provider/manifest.go`), `Render(model, provider, sysPrompt, userPayload, mode ...RenderMode)` (`render.go`), per-role `ResolveRoles` (`internal/decompose/roles.go`), `Config.Roles`/`RoleConfig` (`internal/config/config.go`), `CurrentConfigVersion=2` (`config.go`), decompose pipeline (`internal/decompose/{decompose,planner,stager,arbiter,chain}.go`), and the full v2 git method set including `RevParseTree`/`TreeDiff`/`ReadTree`/`StatusPorcelain`/`WorkingTreeDiff` (`internal/git/git.go`).
- **R1.1 reuses, not rewrites:** the current `Render` already has the `provider_flag`/`provider` branch — this delta repurposes it to split the model prefix and turns the bare-model case into a hard error. The current FR-R5b coupling tests (`TestResolveRoles_FR5b_*`) move/rewrite to assert the prefix form.
- **R3.1 reuses** the per-concept `freezeSnapshot` primitive (`internal/decompose/stager.go`) as a pattern for the start-of-run `T_start`, and generalizes the existing CAS/HEAD-movement guard philosophy.
- **Prior research applies** — `plan/002_a17bb6c8dc1d/architecture/{manifest_v2_delta,config_v2_delta,decompose_architecture,system_context}.md` describe the exact structs, signatures, and call sites this delta modifies. No new architectural research is needed; the FR-D5 model-token confirmation is the only live lookups required (qwen-code lineup; current gemini/claude/codex/cursor tokens).

## 4. Dependency summary

```
P1 (Manifest/Render/Config v3 + reasoning) ──┐
                                             ├──▶ P4 (progress label, E2E, docs)
P2 (qwen-code + token refresh) ──────────────┤
                                             │
P3 (decompose freeze + short-circuit) ───────┘
```

P1 is foundational (Render/config chokepoint). P2 and P3 are independent of each other and of P1's migration (though P3's decompose exercises the v3 render path). P4 integrates and closes docs.
