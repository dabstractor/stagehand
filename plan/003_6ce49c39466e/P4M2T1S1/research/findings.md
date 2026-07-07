# Research Findings — P4.M2.T1.S1 (Sync README + docs to v3 changeset)

## Task
Mode B changeset-level documentation sync. Update README.md + docs/{providers,configuration,cli,how-it-works}.md
to reflect the v3 changeset (config v3 model-prefix inference provider, reasoning FR-R6, qwen-code provider,
decompose T_start freeze + one-file short-circuit). Reconcile the abandoned `agent`→`provider` terminology
drift and purge `default_provider` references (removed in v3).

## v3 changeset (authoritative: plan/003_6ce49c39466e/delta_prd.md §0–§2)

1. **Inference provider folds into the model string (FR-R5b).** The separate `default_provider` field is
   REMOVED. For multi-backend providers (pi, opencode) the inference backend is a slash-prefix on `model`
   (`zai/glm-5.2`). `provider` = agent platform (original meaning). At Render, a `provider_flag` provider
   (pi) splits `model` on first `/` → `--provider <prefix> --model <rest>`; a bare model (no `/`) on such a
   provider is a HARD error, never a silent bare `--model`.
2. **Reasoning level per role (FR-R6).** `reasoning` = `off|low|medium|high`; new `reasoning_levels` manifest
   table; rendered at Render; global + per-role flag/env/config; shipped defaults planner=high, others=off;
   graceful no-op when absent. EVERY role now has all three flags including message (corrects v2 gap).
3. **qwen-code provider (§12.5.2).** Single-backend Gemini-CLI fork for Qwen3-Coder (DashScope);
   `experimental`; inserted into preferredBuiltins between gemini and codex.
4. **Decompose freeze (FR-M1b/M1c) + one-file short-circuit (FR-M2b).** Start-of-run freeze captures the
   whole working-tree change set as T_start; planner/stagers/arbiter/shortcuts draw strictly from T_start
   (concurrent changes excluded from every commit); freeze enforcement aborts on subset violation. One
   changed path → planner bypassed (single commit, no planner call).
5. **Config version → 3 (FR-B7).** `config upgrade` folds `default_provider` into the model slash-prefix
   (NOT byte-for-byte anymore — v2 docs were wrong), maps abandoned `agent`/`[agent.*]` → `provider`.

## Verified shipped state (source of truth for the docs)
- **manifest.go**: `default_provider` field GONE; `reasoning_levels map[string][]string` PRESENT;
  `experimental *bool` PRESENT; `provider_flag` still present (pi's --provider split). (manifest.go:38-89)
- **root.go**: `--reasoning` (global) + `--planner/stager/message/arbiter-reasoning` (ALL FOUR roles)
  + `--message-provider`/`--message-model` (+ reasoning) NOW EXIST (v3 corrects the v2 no-message-flag gap).
  (root.go:45-57, 135-149)
- **registry.go**: `preferredBuiltins = [pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]` (8).
- **config.go**: `CurrentConfigVersion = 3`.
- **bootstrap.go**: writes pi per-role models EMPTY — CORRECT for v3 (user supplies backend prefix), BUT
  the user-facing annotation COMMENTS still say "pi requires a default_provider ... set [provider.pi]
  default_provider" (lines 133, 148-149) — STALE, must be reconciled to the model-prefix.

## Staleness inventory found in the docs (the work)

### README.md
- Hero: provider list "Claude Code, Codex, Gemini CLI, pi, opencode, agy, or Cursor" — MISSING qwen-code.
- Install prerequisite: "pi, Claude Code, Gemini CLI, opencode, Codex, or Cursor" — MISSING agy, qwen-code.
- "Configure your agent" preference order — MISSING qwen-code (lists 7; must be 8).
- The `pi is a multi-provider agent` NOTE + `config init` note reference `[provider.pi] default_provider`
  (REMOVED) — must describe the model-prefix (`zai/glm-5.2`) + FR-R5b.
- Quick start — no `--reasoning`; config examples use bare models, not the model-prefix form.
- FAQ "Which agents are supported?" — says "Seven built-ins" + list omits qwen-code (must be Eight + qwen-code).
- Decompose section describes v2 stager safety only (claude allowlist / pi HEAD-guard); must add the v3
  T_start freeze (concurrent changes excluded) per contract (d).

### docs/providers.md
- Intro "18-field schema" + header "The 7 built-in providers" — inconsistent; field count changed
  (default_provider removed; reasoning_levels + experimental added).
- Schema table STILL has `default_provider` row (REMOVED); MISSING `reasoning_levels` + `experimental` rows.
- Command rendering section — does NOT describe the model-prefix split (FR-R5b) or reasoning token append.
- "The 7 built-in providers" table — header says 7, table has 8 rows (qwen-code only in a NOTE); needs a
  proper qwen-code row + header→8.
- "Per-role default models (FR-D4)": "pi needs a default_provider to route" — STALE (model-prefix now).

### docs/configuration.md
- Precedence: "Provider defaults — manifest's default_model, default_provider, etc." — STALE.
- Bootstrap: order MISSING qwen-code; "pi needs a default_provider ... set [provider.pi] default_provider" STALE.
- config upgrade: says "version 2" + "preserved byte-for-byte" — STALE (version 3; FR-B7 FOLDS
  default_provider → model prefix, so NOT byte-for-byte).
- "current schema (version 2)" + populated example `config_version = 2` + no `reasoning` in [defaults] — STALE.
- Env table: MISSING STAGECOACH_REASONING + per-role reasoning; MESSAGE_PROVIDER/MODEL marked "(no flag)" STALE.
- Bottom note: "message role has no CLI flag" — STALE (--message-* flags exist in v3).

### docs/cli.md
- providers list / config init prose preference orders — MISSING qwen-code.
- config init: "config_version = 2" + "set [provider.pi] default_provider" — STALE.
- config upgrade: "version 2" messages — STALE (3); no FR-B7 fold mention.
- Global flags table + Flag↔env↔git-config map: MISSING --reasoning + all --<role>-reasoning + --message-*;
  the NOTE "message role has no CLI flag" — STALE.

### docs/how-it-works.md (per contract clause d)
- Multi-commit decomposition section: describes v2 per-concept tree[i] freeze + stager safety, but does NOT
  mention the v3 T_start start-of-run freeze (FR-M1b/M1c), freeze enforcement, or the one-file short-circuit
  (FR-M2b). Must be added so the shipped docs match the T_start freeze behavior.
- No default_provider/agent-terminology residue (clean there). No reasoning mention (optional but coherent).

### bootstrap.go (user-facing config output — coherence)
- Lines 133, 148-149: annotation COMMENTS reference `default_provider` (REMOVED). These are written INTO the
  config file by `config init`, so they violate the contract's "self-consistent (no default_provider)".
  COMMENT-ONLY fix (logic already correct): describe the model-prefix instead.

## Scope boundary (parallel coordination)
- P4.M1.T1.S2 (parallel) is the e2e harness — TEST-ONLY (internal/e2e + internal/decompose/decompose_test.go).
  It does NOT touch any doc or bootstrap.go → NO conflict with this doc-sync task. Safe to proceed.
- All v3 implementing subtasks (P1–P3) are COMPLETE → the v3 behavior is shipped and authoritative.

## Validation approach (docs task — grep-based self-consistency assertions)
No unit tests apply. Validation = (a) deterministic grep assertions proving the staleness is purged and the
new surface is present, (b) `make build`/`make test` green (the bootstrap.go change is comment-only → no
behavior change, but verify), (c) manual coherence review of the 5 doc files.
