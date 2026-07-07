# System Context — Stagecoach plan/003 (Config v3 changeset)

**Scope.** This plan delivers a **delta onto a fully-implemented v2** codebase. The v1
single-commit core AND the v2 multi-commit decomposition pipeline are already shipped and
tested. Plan 003 layers the "config v3" changeset described in `delta_prd.md`:

1. **Config v3 / FR-R5b (headline)** — fold the inference provider into the model string as a
   slash-prefix; remove the separate `default_provider` manifest/config field. `provider` reverts
   to its original meaning: the **agent platform** (pi, claude, …).
2. **FR-R6 (new)** — per-role `reasoning` level (`off|low|medium|high`) + a `reasoning_levels`
   manifest table rendered at `Render`, with graceful no-op when absent.
3. **§12.5.2 + FR-D5 (new provider + refresh)** — `qwen-code` built-in + model-token refresh.
4. **FR-M1b/M1c/M2b (new)** — decompose start-of-run freeze `T_start`, freeze enforcement
   (subset check = hard abort), and a one-file short-circuit.
5. **FR51b + §20.5 (new)** — progress label `<Verb> with <model> in <provider>…` + an E2E
   scenario harness.

The codebase is **Go 1.22, stdlib + cobra + go-toml/v2 + spf13/pflag only**. No git library — it
shells out to the real `git` binary. See `delta_prd.md` §0 for sizing rationale (medium-large,
coherent; NOT an inflated multi-phase greenfield).

---

## 1. The load-bearing change: `Manifest.Render` signature + Manifest schema

This is the keystone. Every call path flows through `Render`, so its signature change ripples
through ~6 non-test call sites + ~41 test call sites.

**Current** (`internal/provider/render.go:124`):
```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
```
- The `provider` arg is the literal `""` at **every** non-test call site (the real inference
  backend is resolved internally from `*m.Resolve().DefaultProvider`). So the `provider` param is
  effectively dead at call sites — removing it is low-risk.
- `DefaultProvider *string` field lives on `Manifest` (`manifest.go:62`); merged in `merge.go`;
  set non-nil-`""` only for pi (`builtin.go:50`); consumed at `render.go:113-116,131,139` and by
  decompose's `inferenceProvider` guard (`decompose/roles.go:182-187`).

**Target (v3):**
```go
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```
- `DefaultProvider` field is **removed**. The inference backend is the **slash-prefix on `model`**
  for `provider_flag` providers (pi — the only one). At Render: if `*r.ProviderFlag != ""` and
  `model != ""`: split on first `/` → `--provider <prefix> --model <rest>`; **if `model` has no
  `/`, return a hard error** ("include the inference provider, e.g. `zai/glm-5.2`"), never a bare
  `--model`. Providers without `provider_flag` (opencode, all single-backend) pass `model`
  verbatim.
- `reasoning` is appended after the model flag: `args += m.reasoning_levels[reasoning]` when the
  table declares non-empty tokens for that level; **absent/empty ⇒ silent no-op, never an error**.
- `CmdSpec` (the pure-data return) is **unchanged** → `provider.Execute` needs **no edits**.

**Ripple (non-test call sites, from scout_render_callsites.md §A):**
| file:line | current args | v3 args |
|---|---|---|
| `internal/generate/generate.go:196` | `(cfg.Model, "", sysPrompt, payload)` | `(cfg.Model, sysPrompt, payload, reasoning)` |
| `pkg/stagecoach/stagecoach.go:461` | `(cfg.Model, "", sysPrompt, payload)` | `(cfg.Model, sysPrompt, payload, reasoning)` |
| `internal/decompose/planner.go:98` | `(mdl, "", sysPrompt, payload, RenderBare)` | `(mdl, sysPrompt, payload, reasoning, RenderBare)` |
| `internal/decompose/message.go:129` | `(mdl, "", sysPrompt, payload, RenderBare)` | `(mdl, sysPrompt, payload, reasoning, RenderBare)` |
| `internal/decompose/arbiter.go:97` | `(mdl, "", sysPrompt, payload, RenderBare)` | `(mdl, sysPrompt, payload, reasoning, RenderBare)` |
| `internal/decompose/stager.go:86` | `(mdl, "", "", task, RenderTooled)` | `(mdl, "", task, reasoning, RenderTooled)` |

At each decompose site `mdl` comes from `config.ResolveRoleModel(role, cfg)` (`roles.go:28`, 2-return
today). `reasoning` will come from the **3-return** `ResolveRoleModel` added in P1.M2.

**Manifest struct deltas** (`manifest.go`): remove `DefaultProvider *string`; add
`ReasoningLevels map[string][]string \`toml:"reasoning_levels"\`` (a sub-table; go-toml decodes a
`[reasoning_levels]` block into a map). Update `Resolve()` (drop DefaultProvider default; add
nil-safe ReasoningLevels), `Validate()` (no new constraints — empty map OK), `MergeManifest`
(`merge.go`: drop the DefaultProvider field-merge line; add ReasoningLevels merge — first-layer-wins
or merge per the existing slice/map policy).

**Built-ins** (`builtin.go`, 8 providers): drop `DefaultProvider` from all; populate
`ReasoningLevels` where the agent exposes thinking-effort flags (pi, claude — verify tokens per
FR-D5; gemini/agy/qwen-code/codex/cursor/opencode leave it nil/empty = no-op). Update `providers/*.toml`
reference files: **remove `default_provider`** (only `pi.toml:52` carries it).

---

## 2. Config v3 + per-role reasoning (config package)

`internal/config/config.go:13` → `CurrentConfigVersion = 2` (bump to **3**). The full change map is
in `scout_config_model.md`; the load-bearing touchpoints:

- **`RoleConfig`** (`config.go:43-46`) gains `Reasoning string \`toml:"reasoning"\``.
- **`Config`** gains a global `Reasoning` default (analogous to `Provider`/`Model` in `[defaults]`).
  `Defaults()` sets it `""` (planner overrides to high at role-resolution, not here).
- **`ResolveRoleModel`** (`roles.go:28`) changes from `(provider, model)` to `(provider, model,
  reasoning)` (or returns `RoleConfig`). Applied **per-field** like provider/model.
- **File plumbing**: `fileRoleConfig` (`file.go:21-24`), `materialize` (`file.go:202-208`),
  `overlay` role field-merge (`file.go:284-296`), `fileDefaults` (global reasoning default).
- **Env/flag**: new `setRoleReasoning`; loop body `load.go:190-194` (env) + `load.go:243-252`
  (flags) gain `STAGECOACH_<ROLE>_REASONING` / `--<role>-reasoning`; add `--reasoning` global +
  `STAGECOACH_REASONING`. **Flag registration in `internal/cmd/root.go:108-133`** — register
  `<role>-reasoning` for **all four roles incl. `message`** (the v2 gap: no `--message-*` flags).
- **Shipped role defaults** (FR-R6): `planner=high`, `stager=message=arbiter=off`. Apply these as
  the role-resolution layer (not a hard-coded Config default), so a user's `--reasoning` global still
  propagates when no per-role override exists.

**Migration (FR-B7):** on load of `config_version < 3`, mutate the resolved `Config` **in memory**:
(a) map abandoned `agent`/`[agent.*]` → `provider`/`[provider.*]`; (b) for a multi-backend provider,
prepend its former `default_provider` to its model → `model="Y"` + `default_provider="X"` ⇒
`model="X/Y"` (global `Config.Model`, per-role `Config.Roles[r].Model`, and the raw
`Config.Providers[name]` map); (c) drop the `default_provider` key. Emit a one-time deprecation
notice pointing at `config upgrade`. **No value invented** — a pi model with no resolvable prefix
stays bare → FR-R5b error (the user resolves it). The trigger point is the load-time advisory
(`load.go:259` `configVersionNotice` / `load.go:265-285`).

**`config upgrade` on-disk (FR-B5/B7):** extend `upgradeConfigVersion` (`internal/cmd/config.go:178-204`,
currently a pure textual version-bump) to perform the same →v3 rewrite on disk and bump the version.
Update the many `config_version = 2` test fixtures (`config_test.go`, `default_action_test.go`) → 3.

> Note on the raw-map path: `Config.Providers` is `map[string]map[string]any` and carries
> `default_provider` as a raw entry. The migration reads/rewrites it there (BEFORE
> `provider.DecodeUserOverrides`, which unmarshals into `Manifest` and would silently drop the now-
> removed field). No import cycle.

---

## 3. Decompose roles rework (`internal/decompose/roles.go`)

- `isMultiProvider(m)` (currently `m.ProviderFlag != nil && *m.ProviderFlag != ""`) stays — it still
  correctly classifies pi. But `inferenceProvider(m)` (currently `*m.Resolve().DefaultProvider`) is
  **removed**: the FR-R5b guard becomes "model pinned on a `provider_flag` provider must contain a
  `/`" — check the **model string**, not a manifest field. Role-named error: "model %q on %s must be
  inference/model, e.g. zai/glm-5.2".
- `RoleModels` / `RoleConfig` carry `Reasoning`. `ResolveRoles` returns the 3-tuple per role. The
  4 callers (planner/message/arbiter/stager) pass resolved reasoning into `Render`.
- `pkg/stagecoach` public `RoleModel{Provider, Model}` (`stagecoach.go:64`) gains `Reasoning` (additive;
  zero-value ⇒ off — backward compatible). `applyRoleOverride` (`stagecoach.go:274`) threads it.

---

## 4. qwen-code provider + token refresh (Phase 2)

`internal/provider/registry.go:16` → `preferredBuiltins` is currently
`["pi","opencode","cursor","agy","gemini","codex","claude"]`. v3 inserts `"qwen-code"` between
`gemini` and `codex` → `["pi","opencode","cursor","agy","gemini","qwen-code","codex","claude"]`
(FR-D1). No qwen-code anywhere in the tree today (confirmed).

- `builtinQwenCode()`: single-backend Gemini-CLI fork; `provider_flag=""`; `experimental=true`;
  mirrors gemini/agy flag surface (`-m`, `-p`, stdin delivery, no sys-prompt flag → prepend).
  Mark `# TO CONFIRM` on exact tokens per FR-D5.
- New `providers/qwen-code.toml`. Add qwen-code row to `DefaultModelsForProvider`
  (`role_defaults.go:32-87`).
- Update `TestPreferredBuiltins_MatchesBuiltinKeys` (`registry_test.go:15`) `wantOrder`.
- Model-token refresh (FR-D5): bump agy/gemini flagship → `gemini-3.1-pro`, etc., in `builtin.go`
  `default_model` + matching `providers/*.toml`. Keep `# TO CONFIRM` discipline for qwen/cursor/codex.

---

## 5. Decompose concurrency hardening (Phase 3)

From `scout_decompose_freeze.md`. Entry point `Decompose()` (`decompose.go:139`).

**T_start capture** — insert after the `baseTree` derivation (`decompose.go:152-158`) and BEFORE
`callPlanner` (`decompose.go:150`). Sequence: capture the entire working-tree change set as a tree
(`AddAll` → `WriteTree` → T_start SHA), then **reset the index back to the clean base**
(`ReadTree(baseTree)` — the only index-replace primitive; no reset/restore helper exists) so the
per-concept stager starts from a clean base. `baseTree` (HEAD^{tree}) and T_start (working-tree
change set) are **different objects** — keep `prevTree := baseTree` (`decompose.go:291`) unchanged.

**Freeze invariant** — the planner diffs **T_start** (not a fresh `WorkingTreeDiff`); every stager
stages content drawn strictly from T_start; the arbiter's leftover staging + single-shortcut + one-
file path all draw from T_start. Any file created/modified after T_start is invisible to the run.

**Freeze enforcement (defense-in-depth, FR-M1c)** — after each staging step
(`invokeStagerRetry` → `freezeSnapshot` → tree[i]), verify tree[i] is a **content-subset** of T_start:
every (path, blob) changed in `diff(baseTree, tree[i])` must be present in `diff(baseTree, T_start)`.
A concurrent working-tree change the stager swept in, or a stager that ran a bare `git add -A`,
makes a path/content not traceable to T_start → **hard abort** `ErrFreezeViolation` (non-rescue;
already-landed commits 0..i-1 stand, mirroring the HEAD-movement guard). Needs a git primitive for a
name-status/blob-aware tree-to-tree path-set — add to the `git.Git` interface.

**One-file short-circuit (FR-M2b)** — in **auto** mode (no `--commits`), if T_start has **exactly
one** changed path, bypass the planner entirely: stage that one file's T_start content, generate one
message via the message role, commit. Deterministic (changed-path count), not model judgment.
`--commits N≥2` overrides (honored even for one file). Insert at `decompose.go:~148` before
`callPlanner`.

> Reusable existing primitives: `freezeSnapshot` (`stager.go:108`, a `WriteTree` wrapper),
> `ReadTree` (index replace, `chain.go:201`), `EmptyTreeSHA` (`git.go:500`), `StatusPorcelain`
> (`git.go:1058`). The index has ONLY three mutators: `AddAll`, `Add`, `ReadTree`.

---

## 6. Integration: progress label + E2E (Phase 4)

**Progress label (FR51b)** — `ui.Progress` (`output.go`) currently takes a free string. v3: the `↳`
line becomes `<Verb> with <model> in <provider>…` (e.g. `Generating with zai/glm-5.2 in pi…`). The
model string already carries the inference prefix (FR-R5b) so no special formatting; empty model →
`<provider>` alone. Single-commit path surfaces the **message** role; decompose surfaces the
**planner** role; `--verbose` prints all four roles. Call sites: `generate` (message) and `decompose`
(planner). Add a formatting helper in `ui` (or at the call site) and update both.

**E2E harness (§20.5)** — a `//go:build e2e` throwaway-repo suite: per scenario, `git init` temp
repo → seed → run `stagecoach` (real agent where feasible, else stub) → assert history. Must-cover:
nothing→N commits (auto + `--commits N`); one-file→single **no planner call**; concurrent mid-run
file→excluded from every commit, left in working tree; bare model on multi-backend→**hard error**;
arbiter reconcile (new/tip/mid-chain); rescue mid-loop; CAS abort. Extend the v2 stub decompose suite
with the concurrent-change-exclusion assertion.

---

## 7. Dependency structure

```
P1 (Manifest/Render/Config v3 + reasoning) ──┐
   ├─ P1.M1 Render chokepoint + Manifest schema (keystone)
   ├─ P1.M2 Config reasoning plumbing + ResolveRoles rework
   └─ P1.M3 Config v3 migration (FR-B7)
                                             ├──▶ P4 (progress label, E2E, docs)
P2 (qwen-code + token refresh) ──────────────┤   depends on P1 (v3 manifest schema)
                                             │
P3 (decompose freeze + short-circuit) ───────┘   independent of P2; exercises v3 render (needs P1.M2)
```

P1 is foundational (the Render/config chokepoint every path flows through). P2 and P3 are mutually
independent; P3's decompose callers consume the v3 Render signature so they land after P1.M2. P4
integrates and closes the Mode-B documentation sync.

---

## 8. Documentation handling (per the SOW §5)

- **Mode A (rides with the work):** manifest/Render/config doc comments (P1.M1.S1, P1.M2);
  `providers/*.toml` reference files incl. new `qwen-code.toml` (P1.M1.S1 removes
  `default_provider`; P2 adds qwen-code); `docs/providers.md` (P2); CLI flag help text + `config
  upgrade --help` (P1.M2, P1.M3); decompose freeze doc comments (P3); UI `Progress` doc comment
  (P4.M1.S1).
- **Mode B (final changeset-level task, P4.M2.T1):** `README.md` (config examples →
  `model = "zai/glm-5.2"`, `--reasoning`, qwen-code in the provider list), and reconcile the
  §16.4 terminology drift (`agent`/`[agent.*]` → `provider`/`[provider.*]`; `--planner-agent` →
  `--planner-provider`). Depends on every implementing subtask.

## 9. Test discipline (per SOW §3)

No "write tests" subtasks. Every subtask implies failing-test → implement → pass. Test fixtures that
hardcode `config_version = 2`, the `preferredBuiltins` order, and the ~41 `Render(` call sites are
mechanically updated by the implementing subtasks that change those contracts (P1.M1.S2, P1.M3.S2,
P2.M1.S1).
