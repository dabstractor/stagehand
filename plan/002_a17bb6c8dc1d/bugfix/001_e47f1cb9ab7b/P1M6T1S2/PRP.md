---
name: "P1.M6.T1.S2 — Verify docs/ directory consistency with all five fixes"
description: "Mode-B changeset-level docs sweep. Read every file in docs/ and correct any claim that contradicts the post-fix behavior of the five QA bug fixes (provider/sub-provider conflation, stager toolset scoping, post-arbiter output, config-override in config subcommands, pi bootstrap models). Documentation-only; no code changes. Build/test must still pass."
---

## Goal

**Feature Goal**: Sweep all five files in `docs/` (README.md, cli.md, configuration.md, providers.md, how-it-works.md) and bring every cross-cutting claim into consistency with the **post-fix** behavior of the five Stagehand v2.0 QA bug fixes, so the documentation makes zero stale claims.

**Deliverable**: An edited set of `docs/*.md` files whose wording matches the already-shipped behavior. Concretely: (a) `config path`/`config init`/`config upgrade` are described as honoring `--config`/`STAGEHAND_CONFIG`; (b) the stager safety model is described honestly (claude structurally scoped via a staging-only git allowlist; pi instructional + a HEAD-movement guard, NOT flag-scoped); (c) the pi bootstrap is described as leaving per-role models empty (pi needs `default_provider` to route); (d) the provider-rendering and post-arbiter-output claims are verified accurate. No new docs files; no code changes.

**Success Definition**: Every stale claim enumerated in the Implementation Blueprint is corrected to match the authoritative post-fix wording in the already-updated `README.md` (root) and `providers/{pi,claude}.toml`; `go build ./...` and `go test ./...` still pass; markdown lints clean; a full re-read of each `docs/` file turns up zero contradictions with `stagehand --help` or the source.

## Why

- The five fixes changed shipped behavior that the `docs/` directory documents. The prior subtask (P1.M6.T1.S1, **Complete**) synced the root `README.md`; this subtask sweeps the derived reference docs in `docs/` for the same cross-cutting claims, which were last edited **before** the fixes (timestamps 2026-07-01 ~10:38–10:41, predating the code changes at ~11:30+).
- `docs/` is the user-facing reference ("tracks the shipped binary; if anything here disagrees with `stagehand --help`, the binary is authoritative"). Stale claims here directly mislead users about the stager safety model, the config override semantics, and the pi out-of-box experience — the exact behaviors the five fixes corrected.
- This is the **final** subtask (Mode B, runs last; depends on all implementing subtasks). It must not regress any prior work and must not touch code.

## What

User-visible (documentation-only) changes to `docs/`:

### Success Criteria

- [ ] `docs/cli.md`: `config path` is described as printing the **override-aware** path (honors `--config`/`STAGEHAND_CONFIG`), not "the resolved global config path".
- [ ] `docs/cli.md`: the `--config` flag is described as honored by the default commit action **and the `config init`, `config path`, and `config upgrade` subcommands** (with the `stagehand --config X config upgrade` example).
- [ ] `docs/cli.md`: `config init` reflects that (a) the target path is override-aware and (b) for **pi** the per-role models are left empty.
- [ ] `docs/configuration.md`: the `config path` reference line and the `--config` NOTE both reflect override-aware behavior incl. config subcommands.
- [ ] `docs/configuration.md`: the Bootstrap section notes pi's per-role models are blanked (pi needs `default_provider` to route).
- [ ] `docs/providers.md`: the stager safety three-layer description is **honest** — claude is structurally scoped via `Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`; pi is NOT flag-scoped (instructional §17.6 prompt + best-effort HEAD-movement guard).
- [ ] `docs/providers.md`: the FR-D4 "source of truth for the config bootstrap" note is corrected (the bootstrap blanks pi's models).
- [ ] `docs/how-it-works.md`: the Multi-commit "Safety" bullet no longer claims the stager is "scoped strictly to `git add`" for both providers; it distinguishes claude (structural) from pi (instructional + HEAD guard).
- [ ] Provider-rendering (Issue 1) and post-arbiter output (Issue 3) are **verified** accurate; any claim that the `--provider` flag carries the manifest name, or that printed SHAs can be stale, is corrected if found.
- [ ] `go build ./...` and `go test ./...` still pass (no code touched).

## All Needed Context

### Context Completeness Check

_Passed._ An agent with no prior knowledge of this codebase can complete this PRP: the post-fix behavior is fully specified here, the exact stale strings are quoted verbatim with their file+line, and the correct replacement wording is given (mirroring the already-updated `README.md` and `providers/*.toml`). The five fixes are confirmed applied in the code; the task is pure prose correction.

### Documentation & References

```yaml
# MUST READ — the authoritative post-fix wording (already updated; mirror these in docs/)
- file: README.md
  why: Root README was synced in P1.M6.T1.S1 (Complete). It is the canonical post-fix wording.
  sections:
    - "Configure your agent" → NOTE that pi's `--provider` carries the SUB-PROVIDER from
      `[provider.pi] default_provider` (NOT the manifest name), and the bootstrap NOTE that
      pi per-role models are left empty.
    - "Multi-commit decomposition" → stager safety: "claude via a staging-only git allowlist
      (git add/apply/status/diff); pi instructionally (its task prompt) plus a HEAD-movement
      guard that aborts the run if the stager moves a ref."
    - "Configure your agent" → `--config` NOTE: "honored by every command — including the
      default commit action AND the config init, config path, and config upgrade subcommands."
  critical: Copy this wording; do not invent alternative phrasings. README and docs/ must agree.

- file: providers/pi.toml
  why: Reference manifest; the "SAFETY MODEL — HONEST" comment block is the exact honest framing.
  section: tooled_flags comment ("pi's stager is NOT structurally/flag-scoped … INSTRUCTIONALLY
    constrained (§17.6 prompt) + BEST-EFFORT guarded by the HEAD-movement defense-in-depth check").
  critical: pi.toml default_provider/default_model are "" (FR-D2); this is why bootstrap blanks pi models.

- file: providers/claude.toml
  why: Reference manifest; tooled_flags = the staging-only allowlist.
  section: tooled_flags = "--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit".

# THE FIVE TARGET DOC FILES (read each fully before editing)
- file: docs/cli.md
  why: CLI reference. Stale: --config honored-by wording (L46), config init (L76), config path (L111).
- file: docs/configuration.md
  why: Config reference. Stale: config path line (L31), Bootstrap step 2 (L38), --config NOTE (L67).
- file: docs/providers.md
  why: Manifest/stager reference. Stale: stager safety Layer 1 (L94), FR-D4 bootstrap note (L109).
- file: docs/how-it-works.md
  why: Architecture. Stale: Multi-commit "Safety" bullet (L115). Verify bare-role invariant (L129).
- file: docs/README.md
  why: docs index. Verify only — no stale claim expected (the "binary is authoritative" note stands).

# ARCHITECTURE / RESEARCH (defines post-fix behavior per fix)
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue1_provider_conflation.md
  why: Issue 1 — callers pass "" for the provider param; Render falls back to DefaultProvider.
  critical: The Render ALGORITHM (providers.md "Command rendering") is UNCHANGED. No doc shows
    `pi --provider pi`. Issue 1 needs at most an optional clarifying note; not strictly stale.
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md
  why: Issue 2 — claude tightened to staging-only allowlist; pi honestly documented + HEAD guard.
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue3_post_arbiter_output.md
  why: Issue 3 — rereadFinalCommits after arbiter makes printed SHAs accurate.
  critical: No doc claims stale SHAs are printed → likely NO docs/ change; verify only.
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md
  why: Issue 4 — config init/upgrade/path use ResolveConfigPath(flagConfig) → honor override.
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue5_bootstrap_config.md
  why: Issue 5 — bootstrap blanks pi per-role models when no default_provider.
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M6T1S2/research/docs_stale_claim_matrix.md
  why: Full fix→docs cross-reference with exact stale strings, line numbers, and replacement wording.

# CODE PROOF the fixes are shipped (read to confirm post-fix behavior; do NOT modify)
- file: internal/cmd/config.go
  why: Confirms config path(L133)/upgrade(L141)/init(L234) use config.ResolveConfigPath(flagConfig).
- file: internal/config/file.go
  why: ResolveConfigPath(flagConfig) at L99 — flag > STAGEHAND_CONFIG env > GlobalConfigPath().
- file: internal/config/bootstrap.go
  why: L131–146 — piBlanked: when target=="pi", all per-role models set to "" with an annotation.
- file: internal/provider/builtin.go
  why: L124 claude TooledFlags = "--allowed-tools","Bash(git add:*,...),Read,Edit"; L66–77 pi honest comment.
- file: internal/decompose/decompose.go
  why: L348–367 HEAD-movement guard (ErrStagerMovedHEAD); L185 rereadFinalCommits after arbiter.
- file: internal/git/git.go
  why: L24 LogEntry, L209 LogRange interface (post-arbiter re-read mechanism for Issue 3).
```

### Current Codebase tree (relevant slice)

```bash
docs/
├── README.md          # index; verify only
├── cli.md             # STALE: --config (L46), config init (L76), config path (L111)
├── configuration.md   # STALE: config path (L31), bootstrap (L38), --config NOTE (L67)
├── providers.md       # STALE: stager safety (L94), FR-D4 note (L109)
└── how-it-works.md    # STALE: multi-commit safety (L115); verify bare-role invariant (L129)
README.md              # already updated (S1) — authoritative wording source
providers/{pi,claude}.toml  # already updated — authoritative stager-safety wording source
internal/...           # code; DO NOT MODIFY (proof of post-fix behavior only)
```

### Desired Codebase tree with files to be MODIFIED

```bash
docs/cli.md            # MODIFY: config path, config init, --config honored-by (Issues 4 & 5)
docs/configuration.md  # MODIFY: config path, bootstrap, --config NOTE (Issues 4 & 5)
docs/providers.md      # MODIFY: stager safety layers, FR-D4 note (Issues 2 & 5)
docs/how-it-works.md   # MODIFY: multi-commit safety bullet (Issue 2)
docs/README.md         # verify only (no change expected)
# No new files. No code changes. No PRD/tasks.json changes.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
# CRITICAL: This is a DOCUMENTATION-ONLY task. The five fixes are ALREADY in the code and
# TESTS PASS. Do NOT touch any .go file, PRD.md, tasks.json, or prd_snapshot.md.

# CRITICAL: docs/ must match the ALREADY-UPDATED README.md (root) and providers/*.toml.
# Those files define the canonical post-fix wording. Do NOT invent different phrasings —
# copy/adapt them so README + docs/ are internally consistent.

# GOTCHA (Issue 2): The honest framing splits the two stager-capable providers —
#   claude IS structurally constrained (staging-only git allowlist);
#   pi is NOT flag-scoped (instructional §17.6 prompt + best-effort HEAD-movement guard).
# Do NOT claim pi is "structurally constrained" or "scoped to git add" — that is the exact
# stale claim Issue 2 corrected.

# GOTCHA (Issue 1): The Manifest.Render algorithm itself did NOT change (it still emits
# --provider <provider> when provider != "" and provider_flag != "", falling back to
# DefaultProvider when ""). . The fix was at the CALL SITES (callers pass ""). So the
# providers.md "Command rendering" pseudocode is STILL ACCURATE. Only add a clarifying
# note if a doc implies --provider carries the manifest name.

# GOTCHA (Issue 3): No doc currently claims printed decompose SHAs can be stale. The fix
# makes the documented success report accurate. Expect NO change for Issue 3 — verify only.

# GOTCHA (markdown lint): .markdownlint.json enables default rules EXCEPT MD013 (line
# length), MD033 (inline HTML), MD060 (no-nested). GitHub-style > [!NOTE] admonitions and
# tables are used throughout docs/ — match that style. npx is available for linting.
```

## Implementation Blueprint

### Data models and structure

N/A — no data models. This task edits Markdown prose only.

### Implementation Tasks (ordered by dependencies)

Tasks are ordered to resolve the cross-cutting fixes across files. Each task is a precise
find-and-correct against the quoted stale string. **Read each target file fully before editing**
to preserve surrounding context and anchors.

```yaml
Task 1: VERIFY the five fixes are shipped (no edits — ground truth)
  - RUN: `go build ./...` and `go test ./...` → both must pass (they do, per research).
  - READ: README.md (root) "Configure your agent" + "Multi-commit decomposition"; providers/pi.toml
    "SAFETY MODEL — HONEST" block; providers/claude.toml tooled_flags block.
  - READ: internal/cmd/config.go L133/141/234, internal/config/file.go L99, internal/config/bootstrap.go
    L131-146, internal/provider/builtin.go L66-77 & L124, internal/decompose/decompose.go L348-367.
  - OUTCOME: You have the exact post-fix behavior and canonical wording in hand.
  - WHY FIRST: Every edit below must mirror this wording; do not proceed without it.

Task 2: MODIFY docs/cli.md — config subcommand override behavior (Issue 4)
  - FIND (L111, `### config path`): "Print the resolved global config path:"
  - REPLACE with: "Print the resolved config path (override-aware: honors `--config` / `STAGEHAND_CONFIG`, falling back to the global path):"
  - FIND (L46, `--config` paragraph, end): "...usable with `--provider <name>` on `stagehand` directly (not just the `providers`/`config` subcommands)."
  - REPLACE the parenthetical to mirror README: "...usable with `--provider <name>` on `stagehand` directly. It is also honored by the `config init`, `config path`, and `config upgrade` subcommands — e.g. `stagehand --config X config upgrade` upgrades file `X`, and `config path` prints the resolved path."
  - CHECK: `config upgrade` and `config init` sections — if they imply a global-only target, add a one-line note that `--config`/`STAGEHAND_CONFIG` select the target file (precedence flag > env > global). Do not over-edit.

Task 3: MODIFY docs/cli.md — pi bootstrap models (Issue 5) [can combine with Task 2 in one edit pass]
  - FIND (L76, `config init`): "Bootstrap a **populated, working config** to the global config path. ... writes `config_version = 2`, `[defaults] provider = \"<detected>\"`, and that provider's per-role model defaults UNCOMMENTED so the tool works immediately. Other installed providers appear as commented-out `[role.*]` blocks. If no agent is detected, defaults to `\"pi\"`."
  - REPLACE with wording that: (a) the written path is override-aware (honors `--config`/`STAGEHAND_CONFIG`, default global); (b) for **pi** (the default), the per-role models are left EMPTY so pi picks its own backend model — set `[provider.pi] default_provider` to pin a backend; other detected providers get their per-role defaults uncommented. Mirror README "Configure your agent" bootstrap NOTE.
  - GOTCHA: claude's bootstrap example block later in the file (opus/sonnet/haiku/sonnet) stays valid — claude models are NOT blanked. Only pi is special.

Task 4: MODIFY docs/configuration.md — config path + --config NOTE (Issue 4)
  - FIND (L31): "Use `stagehand config path` to print the resolved global path."
  - REPLACE: "Use `stagehand config path` to print the resolved config path (override-aware: honors `--config` / `STAGEHAND_CONFIG`, else the global path)."
  - FIND (L67, NOTE): "It overrides global and repo-local file discovery and is honored by every command — including the default commit action — so a provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` directly."
  - REPLACE to match README: "...honored by every command — including the default commit action AND the `config init`, `config path`, and `config upgrade` subcommands (e.g. `stagehand --config X config upgrade` upgrades file `X`; `config path` prints the resolved path) — so a provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` directly."

Task 5: MODIFY docs/configuration.md — Bootstrap section pi models (Issue 5)
  - FIND (L38, Bootstrap numbered step 2): "Writes `[defaults] provider = \"<detected>\"` and that provider's per-role model defaults UNCOMMENTED (from the FR-D4 table)."
  - REPLACE to note the pi exception: "Writes `[defaults] provider = \"<detected>\"` and that provider's per-role model defaults UNCOMMENTED (from the FR-D4 table) — EXCEPT for **pi**, whose per-role models are left EMPTY (pi needs a `default_provider` sub-provider to route its `gpt-5.4*` models; the bootstrap writes none, so pi picks its own backend default). Set `[provider.pi] default_provider` to pin a backend."
  - CHECK the "Populated config" TOML example — if it shows pi with gpt-5.4* models uncommented, either switch the example to claude (already claude in configuration.md) or add a note. The current example uses `provider = "claude"`, which is fine.

Task 6: MODIFY docs/providers.md — honest stager safety model (Issue 2)
  - FIND (L94, the three-layer stager safety list, Layer 1): "1. **`tooled_flags`** — scopes tools to staging (claude: git/read/edit allowlist via `--allowed-tools`; pi: all tools on, chrome stripped)."
  - REPLACE Layer 1 with the honest split (mirror pi.toml/claude.toml): "1. **`tooled_flags`** — claude is **structurally** scoped via a staging-only git allowlist (`--allowed-tools Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`) that makes `git commit`/`push`/`update-ref`/`reset`/`rebase` unreachable. pi is **not** flag-scoped — it has no git-scoped allowlist flag, so its tooled profile enables tools with chrome stripped; a misbehaving pi stager CAN run arbitrary Bash. pi's safety is therefore INSTRUCTIONAL (the §17.6 stager task prompt) + a best-effort HEAD-movement guard, not structural."
  - FIND Layer 2 ("Stagehand's ref-mutation monopoly") and Layer 3 (stager task prompt): augment to note the HEAD-movement guard (P1.M2.T1.S3) snapshots HEAD before each stager call and aborts the run if HEAD moved — this is pi's actual safety net, with claude's structural allowlist as defense-in-depth.
  - GOTCHA: PRD §19's "structurally constrained … cannot commit/amend/push" holds for CLAUDE but NOT pi. Do not state it unqualified for both.

Task 7: MODIFY docs/providers.md — FR-D4 bootstrap note (Issue 5)
  - FIND (L109): "The compiled-in per-provider table (PRD §9.16 FR-D4) lives in `internal/config/role_defaults.go` and is the source of truth for the config bootstrap (`config init`, P1.M4.T2)."
  - REPLACE: "The compiled-in per-provider table (PRD §9.16 FR-D4) lives in `internal/config/role_defaults.go`. The config bootstrap (`config init`) uses these defaults — EXCEPT for **pi**, whose per-role models are written EMPTY (pi needs a `default_provider` to route its `gpt-5.4*` models; see [configuration.md](configuration.md)). The pi row below is the compiled-in default, not the bootstrap output."
  - NOTE: the pi table row (L113, gpt-5.4*) stays as-is — it is the accurate compiled-in FR-D4 default.

Task 8: MODIFY docs/how-it-works.md — multi-commit safety bullet (Issue 2)
  - FIND (L115, "Safety" bullet under Multi-commit decomposition): "- **Atomic and safe** — `update-ref CAS` is the only ref mutation per commit. The stager is the ONE role that touches the index (scoped strictly to `git add`); stagehand owns all `commit-tree`, `update-ref`, and `push` operations."
  - REPLACE to distinguish providers honestly (mirror README "Multi-commit decomposition"): "- **Atomic and safe** — `update-ref CAS` is the only ref mutation per commit; stagehand owns all `commit-tree`, `update-ref`, and `push` operations. The stager is the ONE role that touches the index. Its scoping differs by provider: claude is structurally constrained to a staging-only git allowlist (`git add`/`apply`/`status`/`diff`); pi is constrained instructionally (its task prompt) plus a HEAD-movement guard that aborts the run if the stager moves a ref. See [providers.md](providers.md#tooled-mode-and-the-stager-role)."

Task 9: VERIFY providers rendering (Issue 1) and post-arbiter output (Issue 3) — optional edits only
  - READ docs/providers.md "Command rendering" (the pseudocode) and docs/cli.md / docs/how-it-works.md arbiter sections.
  - EXPECT: no stale claim. The Render pseudocode is unchanged (Issue 1 fixed call sites, not Render). No doc claims printed decompose SHAs can be stale (Issue 3).
  - OPTIONAL (consistency): if docs/providers.md or docs/cli.md would benefit, add a one-line note that for pi the `--provider` flag carries the SUB-PROVIDER from `[provider.pi] default_provider`, not the manifest name (mirror README "Configure your agent" NOTE). Only if it improves clarity; do not force it.
  - If you find any explicit stale claim about either issue, correct it to the post-fix behavior; otherwise make NO change for Issues 1 & 3.

Task 10: FINAL full re-read + lint + build/test gate
  - RE-READ every docs/*.md end-to-end. Confirm zero contradictions with README.md and providers/*.toml.
  - RUN markdown lint (Validation Level 1) and `go build ./...` + `go test ./...` (Validation Level 2).
  - RUN the targeted `stagehand` commands in Validation Level 3 to confirm doc claims match the binary.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN: Prose correction against a canonical source.
# For every stale claim, the CORRECT wording already exists in README.md (root) or
# providers/{pi,claude}.toml. The job is: locate the stale string in docs/, replace it
# with the canonical wording (adapted to the doc's voice), and ensure internal links
# (e.g. docs/providers.md#tooled-mode-and-the-stager-role) still resolve.

# PATTERN: Preserve GitHub admonitions and tables. docs/ uses `> [!NOTE]` blocks and
# Markdown tables heavily. Match that style; do not convert them.

# CRITICAL (Issue 2 honesty): The single most important correctness point is that
# docs/ must NOT claim the stager is "structurally constrained" or "scoped to git add"
# for BOTH providers. That phrasing is correct ONLY for claude. pi is instructional +
# HEAD guard. Mirror pi.toml's "SAFETY MODEL — HONEST" block verbatim in spirit.

# CRITICAL (Issue 4): config path/init/upgrade NOW honor --config/STAGEHAND_CONFIG.
# Any phrase like "the global config path" or "resolved global path" in the context of
# these three subcommands is stale and must become "override-aware" / "resolved config path".

# CRITICAL (Issue 5): The bootstrap blanks pi models. Any phrase like "that provider's
# per-role model defaults UNCOMMENTED" must gain a pi exception. claude/other examples
# are NOT blanked — leave them.
```

### Integration Points

```yaml
DOCUMENTATION (no code/config/route integration):
  - docs/cli.md: 3 corrections (config path, --config honored-by, config init pi models)
  - docs/configuration.md: 3 corrections (config path, --config NOTE, bootstrap pi models)
  - docs/providers.md: 2 corrections (stager safety layers, FR-D4 bootstrap note)
  - docs/how-it-works.md: 1 correction (multi-commit safety bullet)
  - docs/README.md: verify only (no change expected)
  - consistency anchors: README.md (root) and providers/*.toml are the source of truth.
NO CODE CHANGES. NO PRD.md / tasks.json / prd_snapshot.md changes. NO .gitignore changes.
```

## Validation Loop

### Level 1: Markdown Style (Immediate Feedback)

```bash
# docs/ uses .markdownlint.json (default rules on; MD013/MD033/MD060 off). npx is available.
npx --yes markdownlint-cli2 "docs/**/*.md" 2>/dev/null || npx --yes markdownlint-cli2 docs/
# Expected: zero errors. If errors appear, READ them and fix (common: MD024 duplicate headings,
# MD040 fenced code without language). The existing docs/ files already pass — your edits must too.

# Manual style check: ensure > [!NOTE] admonitions and tables render (no broken pipe tables).
```

### Level 2: Build & Test Still Pass (Regression Gate — REQUIRED)

```bash
# This task touches NO code, so the suite MUST stay green. This proves no accidental code change.
go build ./...
go test ./...
# Expected: `ok` for every package (matches the pre-edit baseline). Zero failures.
```

### Level 3: Binary ↔ docs Consistency (Manual Spot-Checks)

```bash
# Build the binary if needed.
make build            # produces ./bin/stagehand

# Issue 4 — config subcommands honor --config / STAGEHAND_CONFIG:
STAGEHAND_CONFIG=/tmp/sentinel.toml ./bin/stagehand config path   # must print /tmp/sentinel.toml (NOT the global path)
./bin/stagehand --config /tmp/sentinel.toml config path          # must print /tmp/sentinel.toml
# docs/cli.md + docs/configuration.md "config path" claims must match this output.

# Issue 2 — stager safety (read the merged manifests; compare to docs/providers.md + how-it-works.md):
./bin/stagehand providers show claude | grep -A3 tooled_flags    # Bash(git add:*,...),Read,Edit
./bin/stagehand providers show pi      | grep -A8 tooled_flags   # honest: tools on, no allowlist
# docs/ must describe claude as structurally scoped and pi as instructional + HEAD guard.

# Issue 5 — bootstrap pi models blanked:
cd "$(mktemp -d)" && git init -q && STAGEHAND_CONFIG=./out.toml stagehand config init && grep -A1 'role.planner' out.toml
# For pi, model lines must be empty/absent (NOT gpt-5.4). docs/configuration.md + providers.md must say so.

# Issue 1 — provider rendering (pi --provider carries default_provider, not the manifest name):
# (needs a pi-shaped stub; see plan/.../architecture/issue1_provider_conflation.md reproduction.)
# Verify docs/providers.md + cli.md do NOT show `pi --provider pi`.

# Expected: every docs/ claim matches the binary output above. No contradictions.
```

### Level 4: Full Re-Read Consistency Pass

```bash
# Read each docs/*.md end-to-end and check against the canonical sources.
# For each file, answer: "Does any sentence contradict README.md (root), providers/*.toml,
# or `stagehand --help`?" If yes, fix; if no, pass.
# Re-check the internal anchor links you may have touched, e.g.:
grep -rn "how-it-works.md#multi-commit-decomposition\|providers.md#tooled-mode\|configuration.md" docs/ README.md
# Expected: all anchor links resolve to real headings.
```

## Final Validation Checklist

### Technical Validation

- [ ] Markdown lints clean: `npx markdownlint-cli2 docs/` (or equivalent) → zero errors.
- [ ] Build still green: `go build ./...` → exit 0.
- [ ] Tests still green: `go test ./...` → all `ok`, zero failures.
- [ ] No code/PRD/tasks.json/prd_snapshot/.gitignore files modified (docs/ only).

### Feature Validation

- [ ] All Success Criteria under "What" met (the 9 checkboxes).
- [ ] `config path` doc matches the override-aware binary output (Validation Level 3).
- [ ] Stager safety doc matches `providers show` output (claude scoped; pi honest).
- [ ] Pi bootstrap doc matches the blanked-models config init output.
- [ ] Provider-rendering (Issue 1) and post-arbiter output (Issue 3) verified accurate.
- [ ] Full re-read of each docs/*.md yields zero contradictions with README.md / providers/*.toml / `stagehand --help`.

### Documentation & Quality

- [ ] Wording mirrors the canonical post-fix phrasing in README.md and providers/*.toml (no divergent phrasings).
- [ ] GitHub admonitions (`> [!NOTE]`) and Markdown tables preserved and well-formed.
- [ ] Internal anchor links still resolve.
- [ ] Honest framing preserved: claude structural vs. pi instructional+HEAD-guard (no unqualified "structurally constrained" claim for both).
- [ ] docs/README.md index remains accurate (verify only).

---

## Anti-Patterns to Avoid

- ❌ Don't edit any `.go` file, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `.gitignore` — documentation only.
- ❌ Don't invent new wording when the canonical post-fix phrasing already exists in README.md / providers/*.toml — mirror it so README and docs/ agree.
- ❌ Don't claim the stager is "structurally constrained" or "scoped to git add" for BOTH providers — that is the exact stale claim Issue 2 corrected (true for claude only; pi is instructional + HEAD guard).
- ❌ Don't describe `config path`/`init`/`upgrade` as operating on "the global config path" unconditionally — they now honor `--config`/`STAGEHAND_CONFIG`.
- ❌ Don't claim the bootstrap writes pi's `gpt-5.4*` per-role models — Issue 5 blanks them (claude and others are unaffected).
- ❌ Don't force edits for Issues 1 & 3 if no stale claim exists — verify first; the Render algorithm and the success-report description may already be accurate.
- ❌ Don't skip the `go build`/`go test` gate — even a docs-only task must prove it didn't accidentally touch code.
- ❌ Don't break Markdown tables or `> [!NOTE]` admonitions in the surrounding text of an edit.
