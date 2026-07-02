# Per-Issue Root Cause & Fix Surface

> Grounded in 4 parallel scout reports + direct code reads. All line numbers are current
> as of commit 0b242d5. Each fix includes its exact file:line, the verified code pattern
> to follow, and the test that must accompany it (implicit TDD).

---

## Issue 1 — FR-R6 reasoning feature inert (all providers ship nil ReasoningLevels)

**Severity**: Major | **Root cause**: data-only — the plumbing is correct, only the manifest table is missing.

### Confirmed facts
- `Manifest.ReasoningLevels` type: `map[string][]string` (`internal/provider/manifest.go:89`).
- ALL 8 built-in providers ship it `nil` (`internal/provider/builtin.go`): pi (line 52 TODO),
  claude (line 135 comment), gemini (165), agy (205), qwen-code (249), opencode (279),
  codex (323), cursor (361).
- Render guard (`render.go:126`): `if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`
  — always false with nil map. Dead code for every shipped provider.
- Config resolves `planner = "high"` by default (`roles.go:7-11` defaultRoleReasoning).
  The reasoning value DOES reach Render on the planner path (planner.go:99).

### Verified fix tokens (see external_deps.md)
- **pi**: `--thinking <level>` (off/minimal/low/medium/high/xhigh)
- **claude**: `--effort <level>` (low/medium/high) — NOT `--thinking-effort`
- Others: leave nil (honest no-op).

### Fix locations
1. `internal/provider/builtin.go` — `builtinClaude()` (line 89): add after the `TooledFlags`
   block, before `Output:`:
   ```go
   ReasoningLevels: map[string][]string{
       "high":   {"--effort", "high"},
       "medium": {"--effort", "medium"},
       "low":    {"--effort", "low"},
   },
   ```
   Update the comment at line 135 (remove "ReasoningLevels: nil" from the nil list).
2. `internal/provider/builtin.go` — `builtinPi()` (line 41): add after BareFlags block.
   Remove the TODO comment (lines 52-55). Add:
   ```go
   ReasoningLevels: map[string][]string{
       "high":   {"--thinking", "high"},
       "medium": {"--thinking", "medium"},
       "low":    {"--thinking", "low"},
   },
   ```
3. Test (implicit TDD): assert `BuiltinManifests()["claude"].Render("sonnet", "", "", "high")`
   produces args containing `--effort high`; same for pi with `--thinking high`. Pattern:
   `render_test.go:387` `TestRender_ReasoningTokensAppended` but using real built-ins.
   Assert nil providers (gemini) still no-op.

### No changes needed (confirmed correct)
- `render.go:126` guard — correct.
- `roles.go` defaultRoleReasoning / ResolveRoleModel — correct.
- MergeManifest (merge.go:102-113) — handles ReasoningLevels correctly.

---

## Issue 2 — Message role overrides ignored on single-commit path

**Severity**: Major | **Root cause**: `CommitStaged` and `runPipeline` pass global
`cfg.Model`/`cfg.Reasoning` to Render instead of resolving the message role.

### Confirmed facts
- Loaders correctly write message overrides to `cfg.Roles["message"]` (load.go:33-65).
- `ResolveRoleModel("message", cfg)` (roles.go:41) returns the resolved (provider, model,
  reasoning). When no override: returns `(cfg.Provider, cfg.Model, cfg.Reasoning)` —
  back-compatible.
- The multi-commit path resolves correctly: message.go:103-126.
- The single path does NOT: generate.go:196 and stagehand.go:467 use `cfg.Model`/`cfg.Reasoning`.
- `buildDeps` (stagehand.go:316) selects the manifest from global `cfg.Provider` — a
  `--message-provider` override selects the wrong manifest. buildDeps is called ONLY from
  `GenerateCommit` (single path), confirmed by grep.
- NOTE: PRD references `runGenerate` — actual function is `runDefault` (default_action.go:44).

### Fix locations
1. **generate.go** `CommitStaged` (line 135): before the dedupe loop (~line 194), resolve:
   ```go
   _, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
   ```
   Then at line 196: `deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)`.
   (`config` already imported, generate.go:12.)

2. **stagehand.go** `runPipeline` (line 401): same pattern. Before the loop (~line 446),
   resolve the message role. At line 467 use resolved model/reasoning. ALSO update the
   `model` local var (lines 446-448) and `Result.Model` fields (lines 520, 555) to use the
   resolved model for consistent FR42 reporting.
   (`config` already imported, stagehand.go:16.)

3. **stagehand.go** `buildDeps` (line 316): resolve the message provider for manifest
   selection. Change `name := cfg.Provider` to resolve from the message role:
   ```go
   msgProvider, _, _ := config.ResolveRoleModel("message", cfg)
   name := msgProvider
   if name == "" { /* existing auto-detect */ }
   ```
   When no message-provider override, msgProvider == cfg.Provider (or "" → auto-detect) —
   back-compatible. Needs `config` import (already present at stagehand.go:16).

4. Tests (implicit TDD): stub Manifest that records received model/reasoning; set
   `cfg.Roles["message"] = {Model: "haiku"}` with `cfg.Model == ""`; assert stub receives
   "haiku". For provider: assert buildDeps selects the message-provider's manifest.

### No changes needed
- load.go setRole* writers — correct.
- roles.go ResolveRoleModel — correct.
- default_action.go — does NOT need changes (passes cfg which already carries Roles["message"]).

---

## Issue 3 — runSingleShortcut missing index sync (broken git status)

**Severity**: Major | **Root cause**: `runSingleShortcut` (decompose.go:316) commits tStart
directly via publishCommit but never calls `ReadTree(tStart)` to sync the index.

### Confirmed facts
- `FreezeWorkingTree(baseTree)` resets index to baseTree (git.go:1234).
- `runOneFileShortcut` (decompose.go:280) correctly syncs: `ReadTree(tStart)` at lines 294-300
  with "CRITICAL (findings §4)" comment.
- `runSingleShortcut` (decompose.go:316) is MISSING this sync — after publishCommit (line 332),
  it goes straight to buildCommitResult (line 337).
- `publishCommit` (message.go:197) does CommitTree + UpdateRefCAS — touches HEAD only, NOT index.
- `runSingleEscape` (decompose.go:243) does NOT need sync (uses AddAll + CommitStaged).
- Existing test `TestDecompose_SingleShortcut_CleanMessage` (decompose_test.go:314) does NOT
  assert git status — the gap. Sibling `TestDecompose_OneFileShortcut_PlannerBypassed`
  (decompose_test.go:1507) DOES assert `dcmStatusPorcelain == ""` at line 1549.

### Fix location
Insert between decompose.go lines 336-337 (after publishCommit success, before buildCommitResult):
```go
// CRITICAL (findings §4): sync the index to the committed tree so `git status` is clean.
// The freeze reset index→baseTree; committing treePrime (==tStart) directly would otherwise
// leave index==baseTree ≠ HEAD.tree==tStart ⇒ `git status` shows `MM`. ReadTree(treePrime) ⇒
// index==treePrime==HEAD.tree ⇒ clean. Success-only: on a publish error we returned above.
if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)
}
```
`treePrime` (line 319: `treePrime := tStart`) is never reassigned. `ErrDecomposeFailed`
already in scope (decompose.go:28). `fmt` already imported.

### Test (implicit TDD)
Add `TestDecompose_SingleShortcut_CleanStatus` (or extend the existing CleanMessage test):
assert `dcmStatusPorcelain(t, repo) == ""` after a planner-single run on a BORN repo with
un-staged files. Use the existing `dcmStatusPorcelain` helper (decompose_test.go:111).
The existing CleanMessage test uses an unborn repo and does not assert status — it must be
a SEPARATE test or extended with a born-repo scenario.

---

## Issue 4 — Bootstrap header omits reasoning env vars

**Severity**: Minor | **Root cause**: `bootstrapHeader` constant lists 11 env vars but
omits the 5 reasoning env vars that ARE real, ARE read by loadEnv, and ARE in docs/cli.md.

### Confirmed facts
- `bootstrapHeader` const: bootstrap.go:194-252.
- Missing env vars (all read by load.go, documented in docs/cli.md:43-49, docs/configuration.md:152-156):
  `STAGEHAND_REASONING`, `STAGEHAND_PLANNER_REASONING`, `STAGEHAND_STAGER_REASONING`,
  `STAGEHAND_MESSAGE_REASONING`, `STAGEHAND_ARBITER_REASONING`.
- **CORRECTION**: `STAGEHAND_MAX_COMMITS` is NOT an env var (no LookupEnv in load.go; docs/cli.md
  shows "—" for env). It must NOT be added — it would document a non-existent env var.

### Fix location
In `bootstrapHeader` (bootstrap.go), insert after the per-role `_PROVIDER / _MODEL` lines
and before `STAGEHAND_COMMITS`:
```
#   STAGEHAND_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)
#   STAGEHAND_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
```
Matches the existing `_PROVIDER / _MODEL` shorthand style.

### Test (implicit TDD)
Assert the bootstrap output contains `STAGEHAND_REASONING` and `STAGEHAND_PLANNER_REASONING`.

---

## Issue 5 — In-memory v2→v3 migration no-op for [agent.*] terminology

**Severity**: Minor | **Root cause**: `fileConfig` has no `Agent` field; go-toml/v2 silently
drops unknown `[agent.*]` tables. `migrateV2ToV3` documents this as an intentional no-op.

### Confirmed facts
- `migrateV2ToV3` (migrate.go:35-89): agent→provider remap is comment-only (lines ~42-45).
  The real work is default_provider → model slash-prefix fold (steps 1-3).
- `fileConfig` (file.go:30-36): no `Agent` field (only ConfigVersion/Defaults/Generation/Role/Provider).
- `loadTOML` (file.go:124-150): `toml.Unmarshal(data, &fc)` silently drops `[agent.*]`.
- The on-disk `config upgrade` command handles it, but in-memory load loses the data.
- Impact: Low — the `agent`/`[agent.*]` terminology was an abandoned intermediate, likely
  never shipped in a release.

### Fix approach (defense-in-depth, per PRD suggestion)
Add a textual `[agent.*]`→`[provider.*]` remap in `loadTOML` BEFORE `toml.Unmarshal`.
This is a pre-decode text transformation on the raw `data` bytes:
- Replace `[agent.` → `[provider.` (table headers)
- Replace top-level/defaults `agent =` → `provider =` (the defaults key)
- Also handle `[defaults] agent = "X"` style

This makes in-memory load handle abandoned terminology without changing the typed decode
path. The `config upgrade` on-disk rewrite remains the canonical migration.

Alternative (cleaner, typed): add `Agent` fields to `fileConfig`/`fileDefaults` and remap
in `materialize()`. More robust but more code. The implementing agent should choose.

### Test (implicit TDD)
A v2 TOML with `[defaults] agent = "pi"` + `[agent.pi]` loads with cfg.Provider == "pi" and
cfg.Providers["pi"] populated (currently lost). Write the fixture as a temp file, load via
loadTOML, assert the provider block is preserved.
