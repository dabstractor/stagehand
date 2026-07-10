# Bug Fix Requirements

## Overview

Comprehensive end-to-end testing of the Stagecoach implementation against the v2.8 PRD was performed using a stub agent harness (deterministic output control) plus real installed agents (pi, claude, agy, opencode) for spot checks. Testing covered: the single-commit path, multi-commit decomposition, config bootstrap/upgrade/precedence, provider rendering (FR-R5b), hooks (§9.25), lock contention, exclusions, token limits, format modes, the `--edit`/`--push` conveniences, the integrate exporter, and the freeze invariants (FR-M1b/c/d).

Overall the implementation is mature and the vast majority of PRD requirements work correctly (snapshot/CAS atomicity, stage-while-generating, duplicate rejection, rescue path, hooks ordering, `--no-verify`, freeze enforcement, lazygit/git-alias integration, work-description payload, reasoning levels, rename detection, binary/exclude placeholders, config v2→v3 migration). Three significant bugs were found, all centered on the **config bootstrap producing invalid pi model strings** (FR-R5b violations) and a missing backup in `config upgrade` (FR-B8). These are high-value because the bootstrap is the first-run experience and pi/agy/opencode are the most common providers.

**Severity summary:** 1 Critical, 2 Major, 3 Minor.

## Critical Issues (Must Fix)

### Issue 1: `config init` stager-fallback writes an invalid bare model for pi, breaking decomposition out-of-the-box for agy/opencode/qwen-code users
**Severity**: Critical
**PRD Reference**: §9.15 FR-R5b (a bare model on a `provider_flag` provider is a hard error); §9.17 FR-B1 (bootstrap writes a working config); §9.16 FR-D4 (stager fallback to a tooled provider); §9.14 FR-M1 (decompose is the DEFAULT action when nothing is staged + dirty tree)

**Expected Behavior**: When `config init --provider <X>` is run for a provider whose manifest has empty `tooled_flags` (agy, opencode, qwen-code — i.e. it cannot serve as the stager), the bootstrap routes the stager role to the next tooled provider (pi, per FR-D4). Because pi is a multi-backend `provider_flag` provider, the stager model MUST be in `inference/model` form (e.g. `zai/glm-5.2`) per FR-R5b, OR left blank with guidance (mirroring FR-D2, which deliberately ships pi's models blank because "there is no universally-correct inference backend"). The resulting config must work for a decompose run.

**Actual Behavior**: The bootstrap writes the stager as `provider = "pi"` with a **bare** `model = "gpt-5.4-mini"` (no inference prefix). This is a hard configuration error under FR-R5b. Decomposition — which is the DEFAULT action when nothing is staged and the working tree is dirty — fails **immediately at role resolution**, before the planner is even invoked, regardless of how many files changed:

```
$ stagecoach config init --provider agy
Wrote config to ~/.config/stagecoach/config.toml
$ echo a > a.go; echo b > b.go   # dirty tree, nothing staged
$ stagecoach
stagecoach: decompose: role "stager": model "gpt-5.4-mini" on pi must be inference/model, e.g. "zai/glm-5.2"
```

The same defect is produced by `config init --provider opencode` and `config init --provider qwen-code` (every provider lacking `tooled_flags`). agy (Google's Gemini-CLI successor) and opencode (a major harness) are two of the most common providers, so this breaks the headline v2.0 feature for a large class of users on the very first decompose attempt.

**Root cause**: `internal/config/role_defaults.go` stores bare pi models in the `roleDefaults` table (`pi.stager = "gpt-5.4-mini"`, etc.) — the comment even says "bare; sub-provider set separately via --provider", but v3 (FR-R5b/FR-B7) made a bare model on pi a HARD ERROR. The bootstrap's multi-backend-blanking logic (which correctly blanks pi's models when pi is the *default* provider, per FR-D2) is NOT applied on the stager-fallback path, so the bare `gpt-5.4-mini` is written verbatim. A test at `internal/config/bootstrap_test.go:87` actually **pins** the buggy value (`assertContains(... [role.stager] ... model = "gpt-5.4-mini")`), so no test catches it.

**Steps to Reproduce**:
1. `stagecoach config init --provider agy` (or `opencode`) into a fresh config file.
2. In any git repo with ≥2 dirty files and nothing staged, run `stagecoach`.
3. Observe the immediate hard error: `decompose: role "stager": model "gpt-5.4-mini" on pi must be inference/model`.

**Suggested Fix**: In the bootstrap's stager-fallback path, when the fallback target is a `provider_flag` provider (pi), write a BLANK stager model with the same multi-backend guidance comment used when pi is the default provider (consistent with FR-D2: "there is no universally-correct inference backend"), OR leave the stager model blank and let the user supply the prefix. Do NOT write a bare `gpt-5.4-mini`. Update `internal/config/bootstrap_test.go:87` accordingly and add a post-bootstrap `ValidateModel` assertion on every active role model so a regression is caught.

---

## Major Issues (Should Fix)

### Issue 2: The commented-out pi provider block in every `config init` output uses invalid bare models — uncommenting (the documented FR-B1 workflow) produces a hard error
**Severity**: Major
**PRD Reference**: §9.17 FR-B1 ("Other installed providers are written as commented-out `[role.*]` blocks so switching platforms is a one-line uncomment"); §9.15 FR-R5b (bare model on pi is a hard error); §9.16 FR-D2 (pi is multi-backend)

**Expected Behavior**: FR-B1 explicitly states that the commented-out provider blocks exist so a user can switch platforms by uncommenting ("a one-line uncomment"). Therefore every commented-out block, when uncommented, must produce a valid, working configuration. For pi (a `provider_flag` provider) the models must be in `inference/model` form or blank-with-guidance — never bare.

**Actual Behavior**: Every `config init` output (regardless of `--provider`) includes a commented-out pi block whose models are bare and therefore invalid under FR-R5b:

```toml
# === pi (installed) — uncomment a [role.*] block to route that role to pi ===
# [role.planner]
# provider = "pi"
# model = "gpt-5.4"          ← bare: INVALID for pi
# [role.stager]
# provider = "pi"
# model = "gpt-5.4-mini"     ← bare: INVALID for pi
# [role.message]
# provider = "pi"
# model = "gpt-5.4-nano"     ← bare: INVALID for pi
# [role.arbiter]
# provider = "pi"
# model = "gpt-5.4-mini"     ← bare: INVALID for pi
```

A user who follows the documented workflow (uncomment the pi block to switch to pi) gets:
```
stagecoach: provider render "pi": model "gpt-5.4-nano" on pi must be inference/model, e.g. "zai/glm-5.2"
```

Because pi is installed on most user machines and is the highest-priority default provider (FR-D1), this commented block appears in essentially every `config init` output, so the footgun is universally reachable. (Contrast: the commented-out opencode block correctly uses `openai/gpt-5.4` — opencode has no `provider_flag`, so a prefixed model is valid and passes verbatim. The bug is specific to pi.)

**Root cause**: Same as Issue 1 — `roleDefaults["pi"]` holds bare models and the commented-out-block generator emits them verbatim without applying pi's multi-backend rule.

**Steps to Reproduce**:
1. `stagecoach config init --provider claude` (any provider).
2. Uncomment the pi block (delete the leading `#` on the `provider`/`model` lines under `# === pi (installed) ===`).
3. Run any `stagecoach` command using that config.
4. Observe: `provider render "pi": model "gpt-5.4-nano" on pi must be inference/model`.

**Suggested Fix**: For the commented-out pi block, either (a) write the models blank with the existing pi guidance comment ("pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\""), or (b) write placeholder-prefixed models clearly marked as examples (e.g. `# model = "zai/gpt-5.4"  # example — replace 'zai' with your pi sub-provider`). Any uncommented value must be FR-R5b-valid.

### Issue 3: `config upgrade` does not create a backup of the prior config file (FR-B8 violation)
**Severity**: Major
**PRD Reference**: §9.17 FR-B8 ("Every command that writes the config file — `config init`, `config init --force` and `--template`, **`config upgrade`**, and the install/first-run bootstrap — must ... also leave a timestamped backup of the prior file alongside it ... so every config change is undoable.")

**Expected Behavior**: `config upgrade` must leave a timestamped backup (e.g. `<file>.bak.<timestamp>`) of the pre-upgrade file alongside the upgraded file, mirroring `config init --force` (which does create `<file>.bak.<timestamp>`). This is the "config analogue of FR-H2's never-clobber rule" and the explicit undoability guarantee for schema migrations.

**Actual Behavior**: `config upgrade` overwrites the config file in place with NO backup. The original (pre-migration) contents are silently lost. `config init --force` correctly creates a backup, but `config upgrade` does not — an asymmetry that contradicts FR-B8's "every config-writing command" scope.

```
$ cat > cfg.toml <<'EOF'      # a v2 config
config_version = 2
[defaults]
provider = "pi"
model = "glm-5-turbo"
[provider.pi]
default_provider = "zai"
EOF
$ ls cfg.toml*
cfg.toml
$ stagecoach config upgrade --config cfg.toml
Upgraded config at cfg.toml to version 3.
$ ls cfg.toml*               # NO backup — the v2 original is gone
cfg.toml
```

(For comparison, `config init --force` produces `cfg.toml.bak.2026-07-10T13:43:52Z`.)

**Root cause**: The `config upgrade` write path omits the backup step that `config init --force` performs. FR-B8 lists `config upgrade` explicitly among the commands that must back up.

**Steps to Reproduce**:
1. Create any `config_version = 2` config file with a `default_provider` field.
2. `stagecoach config upgrade --config <file>`.
3. `ls <file>*` — observe no `.bak.*` backup exists; the pre-upgrade content is unrecoverable.

**Suggested Fix**: Add the same timestamped-backup step used by `config init --force` to the `config upgrade` write path, before the in-place rewrite. Verify with a test that asserts a backup file exists after `config upgrade`.

---

## Minor Issues (Nice to Fix)

### Issue 4: FR3j closed-loop token-limit invariant is violated for very small `token_limit` values (below ~270 tokens)
**Severity**: Minor
**PRD Reference**: §9.1 FR3j ("Invariant: `EstimateTokens(assembledFullPrompt) ≤ token_limit`, **always**; the prompt is never delivered over `token_limit`.")

**Expected Behavior**: The assembled prompt (system prompt + user payload) should never exceed `token_limit`, for any `token_limit` value, per the FR3j hard invariant.

**Actual Behavior**: For `token_limit` values below approximately 270, the assembled prompt floor (irreducible system prompt + numstat skeleton + payload framing + the per-file `minBodyTokens` sliver) exceeds the limit, and the closed-loop gate cannot trim further. The code comments in `internal/git/tokengate.go` acknowledge this as a "best-effort fit" degenerate case, but the PRD states the invariant holds "always". Measured: `token_limit=200` delivers a ~269-token prompt; `token_limit=250` delivers ~269 tokens. For `token_limit ≥ ~270` (and all realistic limits — real model context windows are 4K+), the invariant holds correctly.

**Impact**: Negligible in practice — no real model has a sub-270-token context window, and `token_limit` is documented as "set to your model's context window". The defect is a documentation/invariant-vs-implementation mismatch rather than a user-facing failure.

**Suggested Fix**: Either (a) reject `token_limit` values below the computed irreducible floor with a clear error ("token_limit N is too small for the system prompt + skeleton; raise it"), or (b) soften the FR3j wording to "the prompt never exceeds `token_limit` for any `token_limit` at or above the irreducible floor" and document the floor. Option (a) is preferable (fail loud rather than silently violate a documented invariant).

### Issue 5: Doubled "stagecoach:" prefix in the `--edit` empty-message abort
**Severity**: Minor
**PRD Reference**: §9.22 FR-E1 ("An empty result aborts with exit 1 ('empty commit message — aborted')")

**Expected Behavior**: The abort message should read `stagecoach: empty commit message — aborted` (single prefix).

**Actual Behavior**: When `--edit` produces an empty message, the output is `stagecoach: stagecoach: empty commit message — aborted` (the error string already includes the "stagecoach:" prefix and main prepends another).

**Steps to Reproduce**: Run `stagecoach --edit` with a `$GIT_EDITOR` that empties the message file. Observe the doubled prefix on stderr.

**Suggested Fix**: Drop the "stagecoach:" prefix from the error string returned by the edit-abort path (main already adds it), consistent with how other `exitcode.New(exitcode.Error, err)` paths are constructed.

### Issue 6: Auto-stage notice uses "(1 files)" — minor grammar
**Severity**: Minor
**PRD Reference**: §9.4 FR18 ("Print a transparent notice when auto-staging occurs, e.g. `Nothing staged — staging all changes (3 files).`")

**Expected Behavior**: Grammatically correct count ("1 file" / "N files").

**Actual Behavior**: The notice always pluralizes: `Nothing staged — staging all changes (1 files).` (Note: the PRD's own example hardcodes the plural "(3 files)", so this is arguably spec-faithful; flagged only as polish.)

**Suggested Fix**: Singularize when the count is 1 ("1 file"), or leave as-is to match the PRD verbatim.

---

## Testing Summary

- **Total tests performed**: ~50 distinct scenarios across happy-path, edge-case, workflow, integration, error-handling, state, concurrency, and adversarial categories.
- **Passing**: ~44 (the large majority of PRD requirements verified working — see "Areas with good coverage" below).
- **Failing**: 6 (1 Critical, 2 Major, 3 Minor documented above).

### Areas with good coverage (verified WORKING, no issues found)
- Snapshot-based atomic commit (write-tree → commit-tree → update-ref CAS); root commit (no parent) handling.
- Stage-while-generating freeze (FR-M1b): a concurrent file written mid-run is excluded from every commit and left in the working tree.
- Freeze enforcement (FR-M1c): a stager that stages a path outside `T_start` triggers a hard error; HEAD unchanged.
- Arbiter leftover reconciliation (FR-M9/M10): new-commit path folds `T_start` leftovers correctly; frozen gate is tree-based.
- Provider rendering FR-R5b: `zai/glm-5.2` correctly splits to `--provider zai --model glm-5.2`; bare model on a `provider_flag` provider is a hard error.
- Hooks on the plumbing path (FR-V1): correct ordering pre-commit → prepare-commit-msg → commit-msg → post-commit; `--no-verify` skips pre-commit/commit-msg only (FR-V5); pre-commit failure → rescue (FR-V7); stagecoach skips its own prepare-commit-msg hook (FR-V4).
- Duplicate rejection (FR30-33) with rejection-list retry; rescue path (FR43-45) with tree SHA + recovery recipe; exit codes (0/1/2/3/124/5).
- Config precedence (FR34): flag > env > git-config > file > default, verified end-to-end via rendered argv.
- Format modes (conventional/gitmoji/plain) replace style examples correctly; `--locale`, `--template` (with `$msg` validation), `--context`, `--exclude`/`.stagecoachignore` (with negation-skip warning) all work.
- Binary file placeholders (FR3a/b) and `[excluded]` placeholders (FR-X4); rename detection `-M` (FR3e); numstat skeleton (FR3g).
- Token-limit water-fill + closed-loop gate holds the invariant for all realistic limits (≥ ~270 tokens).
- `--edit` (FR-E1) opens editor, strips comments, empty-message aborts exit 1; `--push` (FR-P1-3) push-failure-is-not-commit-failure with exit 1 and verbatim git stderr.
- Lock contention (FR52) exit 5 with holder diagnostics; `lock status` (FR-K4); hook install/uninstall/status with foreign-hook refusal (FR-H2); lazygit + git-alias integrate (no-mangle, comment-preserving, backup).
- Config v2→v3 upgrade migration (FR-B7) correctly folds `default_provider` into the model prefix; FR-B9 inert-file no-op; FR-B8 active-settings preservation for `config init --force`.
- Unicode and special characters (quotes/backticks/`$`) in commit messages preserved correctly.
- Per-role provider/model/reasoning config (FR-R1-R6) including graceful no-op for reasoning on a provider without `reasoning_levels`.

### Areas needing more attention
- **The config bootstrap's pi model handling** (Issues 1 & 2) — the `roleDefaults` table and the stager-fallback / commented-out-block generators do not honor FR-R5b/FR-D2 for pi. This is the highest-impact defect cluster.
- **`config upgrade` backup** (Issue 3) — a one-line-equivalent omission with an explicit FR-B8 requirement.
- A post-bootstrap validation pass that runs `ValidateModel` over every active role model would have caught Issues 1 & 2 automatically; adding it is recommended as a regression net.
