---
name: "P4.M2.T1.S1 — Mode B changeset-level documentation sync: update README.md + docs/{providers,configuration,cli,how-it-works}.md to the v3 changeset (config v3 model-prefix inference provider + reasoning FR-R6 + qwen-code provider + decompose T_start freeze FR-M1b/M1c/M2b), and reconcile the abandoned agent→provider terminology drift + purge all default_provider references (removed in v3)"
description: |

  Mode B documentation task (the FINAL sync that closes the v3 delta). EDIT 5 documentation files + 1 source
  COMMENT block. NO production-logic changes.

  EDIT README.md                       — hero/quick-start/config examples → model-prefix form + --reasoning;
                                         add qwen-code to every provider list; purge default_provider; add the
                                         v3 decompose T_start freeze to the decompose blurb.
  EDIT docs/providers.md               — drop default_provider from the schema table; add reasoning_levels +
                                         experimental rows; document the model-prefix split + reasoning emit in
                                         command rendering; add a qwen-code table row + header count; fix the
                                         "pi needs default_provider" FR-D4 note to the model-prefix.
  EDIT docs/configuration.md           — precedence/bootstrap: drop default_provider, add qwen-code; rewrite
                                         config upgrade to version 3 + FR-B7 fold; add reasoning + --message-*
                                         to env/flag tables; remove the "message has no flag" STALE note.
  EDIT docs/cli.md                     — add --reasoning + all --<role>-reasoning + --message-* flags to the
                                         global-flags + map tables; fix preference orders + version 2→3 +
                                         config upgrade FR-B7; remove the STALE "message has no flag" note.
  EDIT docs/how-it-works.md            — add the v3 T_start start-of-run freeze + freeze enforcement
                                         (FR-M1b/M1c) + one-file short-circuit (FR-M2b) to the Multi-commit
                                         decomposition section (contract clause d).
  EDIT internal/config/bootstrap.go    — COMMENT-ONLY: the `config init` annotation strings still reference
                                         default_provider (lines 133, 148-149); rewrite them to the model-prefix
                                         (FR-R5b). No logic change — pi per-role models stay empty (already correct).

  CONTRACT (P4.M2.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: delta_prd.md §2 (Mode B). The v3 model-prefix terminology, the reasoning feature, and
       the qwen-code provider change the user-visible surface. The PRD §16.4 CLI example uses `--planner-agent agy`
       and `agent = "pi"` / `agent = "agy"` — but FR-R3/FR-B7 and the §12 terminology table all standardize on
       `provider` (the `agent`/`[agent.*]` terminology is the abandoned intermediate that FR-B7 maps BACK to
       provider). README.md is the marketing surface (PRD §21.5 structure).
    2. INPUT: ALL implementing subtasks (P1.M1.T1.S1 → P4.M1.T1.S2). README.md, docs/providers.md,
       docs/configuration.md, docs/cli.md.
    3. LOGIC: (a) README.md — update the hero/quick-start config examples to use the model-prefix form
       (model = "zai/glm-5.2" for multi-backend providers, bare model for single-backend), add --reasoning to
       the quick-start, and add qwen-code to the provider list. (b) Reconcile the §16.4 terminology drift in any
       README/docs examples: agent/[agent.*] → provider/[provider.*]; --planner-agent → --planner-provider.
       (c) docs/providers.md + docs/configuration.md — sync the provider list (add qwen-code, mark experimental),
       the model-prefix config surface, and the reasoning surface so the shipped docs are self-consistent.
       (d) Verify NO doc still references default_provider as a live field (it is removed in v3). Verify the
       §13.4 stage-while-generating + decompose descriptions still match the T_start freeze behavior where relevant.
    4. OUTPUT: README + docs reflect the v3 changeset coherently and are self-consistent (no agent terminology,
       no default_provider).
    5. DOCS: this IS the documentation task (Mode B changeset-level sync).

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  PARALLEL COORDINATION WITH P4.M1.T1.S2 (e2e harness) — NO CONFLICT:
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  P4.M1.T1.S2 is being implemented IN PARALLEL but is TEST-ONLY: it creates internal/e2e/* and edits only
  internal/decompose/decompose_test.go. It touches NO documentation file and NOT internal/config/bootstrap.go.
  Therefore this documentation task has ZERO file overlap with the parallel task — safe to proceed. All v3
  implementing subtasks (P1–P3) are COMPLETE, so the v3 behavior the docs must describe is already shipped and
  authoritative (verified against the live source in this PRP's references).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - PRD.md, tasks.json, prd_snapshot.md, .gitignore — NEVER modify.
    - Any *.go SOURCE LOGIC — the ONE source edit allowed is bootstrap.go's COMMENT/annotation STRINGS only
      (config init output), because those annotations are user-facing and must be default_provider-free for
      coherence. Do NOT change bootstrap.go logic (pi per-role models already correctly empty for v3).
    - providers/*.toml reference files — already v3-correct (default_provider purged in P1.M1.T1.S1; qwen-code
      present). Do NOT edit (they are the Mode A deliverable of P1/P2, already shipped).

  DELIVERABLES (5 doc EDITS + 1 comment-only source EDIT):
    EDIT README.md
    EDIT docs/providers.md
    EDIT docs/configuration.md
    EDIT docs/cli.md
    EDIT docs/how-it-works.md
    EDIT internal/config/bootstrap.go   (comment/annotation strings ONLY)

  SUCCESS: the docs are self-consistent with the shipped v3 binary: every provider list includes qwen-code (8
  built-ins); no live reference to default_provider (migration-only historical context excepted); no `agent`/
  `[agent.*]`/`--planner-agent` terminology; config_version is 3 everywhere; `config upgrade` describes the
  FR-B7 fold (not byte-for-byte); --reasoning + per-role reasoning + --message-* flags documented; the model-
  prefix form (zai/glm-5.2) shown in all config/model examples for multi-backend providers; the decompose
  description mentions the T_start freeze + one-file short-circuit. `make build`/`make test` green (bootstrap.go
  change is comment-only). go.mod/go.sum UNCHANGED.

---

## Goal

**Feature Goal**: Make Stagecoach's shipped documentation self-consistent with the v3 binary (PRD §21.5 + delta_prd
§2 Mode B). The v3 changeset changed the user-visible surface in four ways that the docs (written for v2,
partially patched) no longer reflect: (1) the inference backend folded into the model string as a slash-prefix
(`model = "zai/glm-5.2"`), removing the `default_provider` field entirely (FR-R5b/FR-B7); (2) a per-role
`reasoning` level with new global + per-role flags/env including a `--message-*` flag set that corrects a v2 gap
(FR-R6); (3) a new qwen-code built-in provider raising the count to eight (§12.5.2/FR-D1); (4) the decompose
start-of-run freeze `T_start` + one-file short-circuit that change what a multi-commit run is guaranteed to
contain (FR-M1b/M1c/M2b). This task edits the 5 user-facing docs (+ 1 comment-only source annotation block) so
every example, table, flag list, and prose description matches what `stagecoach --help` and the source actually
ship — and reconciles the abandoned `agent`/`[agent.*]`/`--planner-agent` terminology back to `provider`.

**Deliverable** (5 doc EDITS + 1 comment-only source EDIT — no logic changes):
1. `README.md` — hero/quick-start/config examples use the model-prefix form + `--reasoning`; qwen-code in every
   provider list; `default_provider` purged; v3 decompose freeze in the blurb; FAQ "Eight built-ins".
2. `docs/providers.md` — schema table: drop `default_provider`, add `reasoning_levels` + `experimental`;
   command-rendering: document the model-prefix split + reasoning emit; qwen-code as a proper table row +
   header→8; FR-D4 note fixed to the model-prefix.
3. `docs/configuration.md` — precedence/bootstrap: drop `default_provider`, add qwen-code; `config upgrade` →
   version 3 + FR-B7 fold (NOT byte-for-byte); add reasoning + `--message-*` to env/flag tables; remove the
   STALE "message has no flag" note.
4. `docs/cli.md` — add `--reasoning` + all `--<role>-reasoning` + `--message-*` to the global-flags + map
   tables; fix preference orders + version 2→3 + config-upgrade FR-B7; remove the STALE "message has no flag" note.
5. `docs/how-it-works.md` — Multi-commit decomposition section gains the `T_start` freeze + freeze enforcement
   (FR-M1b/M1c) + one-file short-circuit (FR-M2b).
6. `internal/config/bootstrap.go` — COMMENT/annotation strings only: the `config init` annotations referencing
   `default_provider` (lines ~133, ~148-149) rewritten to the model-prefix (FR-R5b). No logic change.

**Success Definition** (deterministic grep-assertable):
- **No default_provider as a live field**: `grep -rn "default_provider" README.md docs/` returns nothing that
  presents it as a live config field (only migration/upgrade historical context, if any, is acceptable). The
  bootstrap.go annotation strings are default_provider-free.
- **No agent terminology**: `grep -rn "planner-agent\|\[agent\.\|agent = \"\|--planner-agent\|--stager-agent"
  README.md docs/` returns nothing.
- **Version 3**: `grep -rn "version 2\|config_version = 2\|at version 2\|schema version (2)\|schema (version 2)"
  README.md docs/` returns nothing; `config_version = 3` appears in config examples.
- **qwen-code everywhere**: `grep -rln "qwen-code" README.md docs/providers.md docs/configuration.md docs/cli.md`
  returns all 4 files.
- **reasoning surface documented**: `grep -rn "\-\-reasoning\|STAGECOACH_REASONING\|reasoning_levels" docs/cli.md
  docs/configuration.md docs/providers.md` returns hits.
- **message flags documented**: `grep -rn "\-\-message-provider\|\-\-message-reasoning" docs/cli.md
  docs/configuration.md` returns hits (the v2 "no message flag" gap is closed in the docs).
- **model-prefix in examples**: `grep -rn "zai/glm-5.2\|inference/model\|model.*slash-prefix\|prefix" README.md
  docs/` returns hits (the v3 model form is shown).
- **build/test unaffected**: `make build` + `make test` green; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the end user reading the README or docs/ to install, configure, and use Stagecoach (PRD §7.1
plan-holder + §7.2 API-key refusenik + §7.3 multi-agent tinkerer), and the contributor adding a provider.

**Use Case**: a user installs stagecoach, runs `config init`, and follows the README/docs to set their provider +
model. With v3 they MUST supply a model-prefix for multi-backend providers (pi/opencode) or get a hard error,
CAN tune reasoning per role, and SEE qwen-code as an option. If the docs still show `default_provider` or bare
models on pi, the user mis-configures and gets an FR-R5b error with no doc explanation — a direct trust failure.

**User Journey**: (1) README hero → install → quick-start with `--reasoning`; (2) "Configure your agent" shows
the model-prefix form; (3) `config init` writes a v3 file (annotations explain the prefix, NOT default_provider);
(4) per-role config examples route planner to a reasoning model; (5) docs/cli.md + configuration.md are the
authoritative flag/env reference (now complete incl. reasoning + message-*).

**Pain Points Addressed**: (a) docs no longer contradict the binary (the §16.4 agent-terminology drift, the
`default_provider` ghost, the "version 2" references, the missing flags); (b) the v3 model-prefix is explained
where users will hit it; (c) the decompose safety story matches the freeze guarantee.

## Why

- **Business value**: this is the changeset-level doc close-out (delta_prd §2 Mode B) — the v3 behavior shipped
  across P1–P3 is unreachable-in-good-faith until the docs stop describing v2. Docs are the marketing surface
  (PRD §21.5) and the trust layer: a user who follows the README and gets an error the README doesn't mention
  concludes the tool is broken. The FR-R5b bare-model hard error, in particular, is a guaranteed support incident
  if the docs still show bare models on pi.
- **Integration with existing features**: consumes the COMPLETE v3 implementation — config v3 (P1.M1/M2/M3),
  qwen-code + token refresh (P2), the decompose freeze + one-file short-circuit (P3). This task writes NO code;
  it makes the docs faithfully describe what those tasks shipped.
- **Problems this solves and for whom**: removes the four classes of v2→v3 doc drift (default_provider ghost,
  agent terminology, missing reasoning/message flags + version-2 + qwen-code omissions) so a new user's
  first-run experience matches the README.

## What

**User-visible behavior** (docs only — no runtime change):
- README + docs show `model = "zai/glm-5.2"` (multi-backend) / bare model (single-backend), `--reasoning`, the
  8-provider list incl. qwen-code (experimental), config_version 3, and a `config upgrade` that folds
  default_provider into the model prefix.
- The decompose description promises the T_start freeze (concurrent changes never enter a commit) + the one-file
  short-circuit (one file → one commit, no planner).
- No doc mentions `default_provider` as a live field, `agent`/`[agent.*]`, `--planner-agent`, or "version 2".

**Technical requirements**: 5 markdown doc edits + 1 comment-only Go annotation edit. No new files, no code
logic changes, no schema/config/flag changes, no go.mod changes.

### Success Criteria

- [ ] `grep -rn "default_provider" README.md docs/` — no LIVE-FIELD references (migration/upgrade context only, OK).
- [ ] bootstrap.go `config init` annotations are default_provider-free (describe the model-prefix).
- [ ] `grep -rn "planner-agent\|\[agent\.\|agent = \"\|--planner-agent\|--stager-agent" README.md docs/` — empty.
- [ ] `grep -rn "version 2\|config_version = 2\|at version 2\|schema version (2)\|schema (version 2)" README.md docs/`
      — empty; `config_version = 3` present in config examples.
- [ ] qwen-code appears in README + docs/{providers,configuration,cli}.md (and the preference-order lists are the
      full 8: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude).
- [ ] README + docs use the model-prefix form for multi-backend providers (`zai/glm-5.2`), bare for single-backend.
- [ ] docs/cli.md global-flags + map tables include `--reasoning`, `--<role>-reasoning` (all 4 roles), and
      `--message-provider`/`--message-model`/`--message-reasoning`; the "message role has no CLI flag" note is GONE.
- [ ] docs/configuration.md env table includes `STAGECOACH_REASONING` + `STAGECOACH_<ROLE>_REASONING`; the message
      role "(no flag)" row is corrected.
- [ ] docs/providers.md schema table has NO `default_provider` row, HAS `reasoning_levels` + `experimental` rows;
      command-rendering documents the FR-R5b model-prefix split + reasoning emit; qwen-code is a proper table row.
- [ ] docs/how-it-works.md Multi-commit decomposition section describes the T_start freeze (FR-M1b/M1c) + one-file
      short-circuit (FR-M2b).
- [ ] `make build` + `make test` green; go.mod/go.sum UNCHANGED; bootstrap.go diff is comment/annotation strings only.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ — YES. This PRP embeds the
exact v3 changeset semantics (from delta_prd.md §0–§2 + the verified source), the exact staleness inventory
found in each of the 5 docs + bootstrap.go (line-anchored where possible), the exact verified source facts
(manifest.go fields present/absent, root.go flag names, registry.go preference order, config.go version,
bootstrap.go annotation lines), the exact grep assertions that prove success, and the exact scope boundary
(comment-only source edit; no logic; no conflict with the parallel e2e task). The implementer needs no further
codebase exploration beyond the 6 files listed and the verified source facts below.

### Documentation & References

```yaml
# MUST READ — the v3 changeset contract (what the docs must reflect)
- url: plan/003_6ce49c39466e/delta_prd.md §0 (diff analysis) + §1 (R1.1–R4.2) + §2 (Mode B R-DOC)
  why: "THE authoritative v3 changeset. §0 groups the 5 changes; §1 R1.1 (model-prefix/FR-R5b/FR-B7), R1.4
        (reasoning/FR-R6), R2.1 (qwen-code), R3.1/R3.2 (freeze FR-M1b/M1c + one-file FR-M2b); §2 R-DOC is THIS
        task's contract — including the §16.4 agent→provider terminology reconciliation requirement."
  critical: "R-DOC explicitly lists: README model-prefix/`--reasoning` quick-start + provider list (add qwen-code);
        reconcile §16.4's `--planner-agent agy` / `agent = \"pi\"` → `--planner-provider agy` / `provider = \"pi\"`.
        default_provider is REMOVED (R1.1). `config upgrade` FOLDS default_provider into the model prefix (R1.2),
        so it is NOT byte-for-byte (the v2 docs were wrong)."

# PRD sections selected as context (the authoritative product semantics)
- url: PRD.md §16.4 (per-role provider/model/reasoning, v3) + §9.15 (FR-R1–R6) + §9.16 (FR-D1 preference order)
  why: "§16.4 is the canonical config example (provider/model/reasoning per role; multi-backend model =
        `inference/model`). §9.15 FR-R3 = all four roles expose all three flags INCLUDING message; FR-R5b =
        model-prefix + bare-model-on-pi HARD error; FR-R6 = reasoning levels + shipped defaults (planner=high,
        others=off) + graceful no-op. §9.16 FR-D1 = the 8-provider preference order incl. qwen-code."
  critical: "FR-D1 order is verbatim: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude. Use THIS
        exact order in every preference list. qwen-code is between gemini and codex."

- url: PRD.md §16.2 (full config file example, config_version 3) + §21.5 (README structure)
  why: "§16.2 is the model config example to mirror: `[defaults] model = \"zai/glm-5.2\"`, `reasoning = \"off\"`,
        no default_provider field. §21.5 is the 10-point README structure the marketing surface follows."

# VERIFIED SHIPPED SOURCE — the source of truth the docs must match (paths exact; verified this research pass)
- file: internal/provider/manifest.go   # the schema
  why: "Manifest struct TOML fields (lines 38-89): name, detect, command, subcommand, prompt_delivery,
        prompt_flag, print_flag, model_flag, default_model, system_prompt_flag, provider_flag, bare_flags,
        tooled_flags, experimental, output, json_field, strip_code_fence, retry_instruction, env,
        reasoning_levels. NO default_provider field. reasoning_levels map[string][]string PRESENT."
  pattern: "Mirror this exact field set in the docs/providers.md schema table (drop default_provider; add
        reasoning_levels + experimental). The field count is now 19 (was 18)."

- file: internal/provider/registry.go   # the preference order
  why: "Line 16: `preferredBuiltins = []string{\"pi\", \"opencode\", \"cursor\", \"agy\", \"gemini\",
        \"qwen-code\", \"codex\", \"claude\"}` — 8 providers, qwen-code between gemini and codex. This is THE
        order for every 'auto-detection preference' / 'default provider' list in the docs."

- file: internal/cmd/root.go   # the flag surface
  why: "Lines 45-57: flagReasoning + flagPlannerReasoning + flagStagerReasoning + flagMessageReasoning +
        flagArbiterReasoning. Lines 135-149 register --reasoning (global) + --planner-reasoning /
        --stager-reasoning / --message-reasoning / --arbiter-reasoning. Lines 52-53, 142-144:
        flagMessageProvider / flagMessageModel → --message-provider / --message-model. So ALL FOUR roles
        (incl. message) have provider/model/reasoning flags — the v2 'no --message-* flag' gap is CLOSED in v3."
  pattern: "docs/cli.md global-flags table + Flag↔env↔git-config map MUST include --reasoning + the 4
        --<role>-reasoning + --message-provider/--message-model. The 'message role has no CLI flag' NOTE in
        cli.md + configuration.md is STALE — delete it."

- file: internal/config/config.go   # the version
  why: "Line 18: `const CurrentConfigVersion = 3`. Every docs reference to schema version must say 3 (not 2)."

- file: internal/config/migrate.go   # the v3 upgrade semantics (FR-B7)
  why: "The v3 migration FOLDS a multi-backend provider's former default_provider into a model slash-prefix
        (default_provider=X + model=Y → model=X/Y) and DROPS default_provider. So `config upgrade` is NOT
        'preserved byte-for-byte' (the v2 docs said that — WRONG). It rewrites the model line + deletes the key."
  pattern: "docs/configuration.md + docs/cli.md `config upgrade` sections: version 3; describe the fold (not
        byte-for-byte). Keep the 'idempotent' property (running twice is a no-op)."

- file: internal/config/bootstrap.go   # the comment-only source edit target
  why: "Lines ~133 (a code comment) + ~148-149 (b.WriteString annotation strings written INTO the config file
        by `config init`) STILL say 'pi requires a default_provider (sub-provider) ... set [provider.pi]
        default_provider'. default_provider is REMOVED in v3. These annotations are USER-FACING (they appear
        as comments in the generated config), so they violate 'self-consistent (no default_provider)'."
  pattern: "COMMENT/STRING-ONLY edit: rewrite to the model-prefix (FR-R5b). E.g. 'pi is a multi-backend
        provider: prefix the model with the inference backend, e.g. model = \"zai/glm-5.2\". The shipped
        per-role models are empty so you can supply your own backend/model.' Do NOT change the LOGIC (pi
        per-role models stay empty — that is already correct for v3)."

- file: docs/how-it-works.md   # the T_start freeze section (contract clause d)
  why: "The 'Multi-commit decomposition' section (line 47+) describes the v2 per-concept tree[i] freeze +
        stager safety, but does NOT mention the v3 T_start start-of-run freeze (FR-M1b/M1c: the whole working-
        tree change set is frozen at run start; concurrent changes are excluded from every commit; freeze
        enforcement aborts on subset violation) or the one-file short-circuit (FR-M2b: exactly one changed
        path → single commit, planner bypassed). Contract (d) requires these match."
  pattern: "Add a subsection (or extend 'Pipeline flow'/'Key design points'/'Safety') describing: (1) T_start
        freeze — the instant decomposition activates, the entire working-tree change set is captured as an
        immutable tree; planner/stagers/arbiter/shortcuts draw strictly from it; files changed AFTER capture
        never enter a commit. (2) Freeze enforcement — after each staging step the resulting tree is verified
        to be a content-subset of T_start; a concurrent change swept in (or a bare `git add -A` stager) is a
        hard abort (non-rescue; already-landed commits stand). (3) One-file short-circuit — exactly one changed
        path in auto-mode bypasses the planner entirely (one commit, no planner call)."

# FILES TO EDIT (the staleness inventory — each item below is a concrete fix)
- file: README.md   # EDIT — the marketing surface (PRD §21.5)
  staleness:
    - Hero: provider list omits qwen-code ("Claude Code, Codex, Gemini CLI, pi, opencode, agy, or Cursor") → add qwen-code.
    - Install prerequisite: "pi, Claude Code, Gemini CLI, opencode, Codex, or Cursor" → add agy + qwen-code.
    - "Configure your agent" preference order: 7 providers → 8 (add qwen-code between gemini and codex).
    - The `pi is a multi-provider agent` NOTE + the `config init` note reference `[provider.pi] default_provider`
      (REMOVED) → rewrite to the model-prefix (FR-R5b): the backend is the slash-prefix on `model`
      (`zai/glm-5.2`); a bare model on pi is a hard error.
    - Quick start: add `--reasoning` (e.g. `stagecoach --reasoning high`); show the model-prefix form in the
      "Configure your agent" / git-config examples for pi.
    - Multi-commit decomposition blurb: add the v3 T_start freeze (concurrent changes excluded from every commit)
      + keep the existing stager-safety sentence.
    - FAQ "Which agents are supported?": "Seven built-ins" → "Eight built-ins"; add qwen-code (experimental).
  gotcha: "Keep claude single-backend examples bare (e.g. `model = \"sonnet\"` / `--model sonnet`). Only
    multi-backend providers (pi, opencode) take the prefix. opencode uses `openai/gpt-5.4` token form internally
    (backend/model); pi uses `--provider <prefix> --model <rest>`. The README example for pi is `zai/glm-5.2`."

- file: docs/providers.md   # EDIT — the manifest reference
  staleness:
    - Intro "18-field schema ... the 8 built-in providers" vs header "The 7 built-in providers" — inconsistent;
      field count changed (default_provider removed; reasoning_levels + experimental added → 19 fields); 8 providers.
    - Schema table: HAS a `default_provider` row (REMOVED) → DELETE it; MISSING `reasoning_levels` + `experimental`
      rows → ADD them (reasoning_levels = map of level→token-list; experimental = bool, marks agy/qwen-code).
    - Command rendering: does NOT describe the model-prefix split (FR-R5b: a provider_flag provider splits model
      on first `/` → --provider <prefix> --model <rest>; bare model on such a provider = hard error) or the
      reasoning-level token append → ADD a paragraph.
    - "The 7 built-in providers" header + table: header says 7, qwen-code is only a NOTE → ADD a qwen-code row
      (Delivery stdin; Print -p; Model -m; Default qwen3-coder-plus⚠️; no sys-prompt flag; read-only constraint;
      Stager — no; experimental) and fix the header to 8.
    - "Per-role default models (FR-D4)": "pi needs a default_provider to route" → rewrite to "pi needs an
      inference-provider prefix on the model (FR-R5b); its shipped per-role models are blank so you supply
      backend/model, e.g. zai/gpt-5.4".

- file: docs/configuration.md   # EDIT — the config reference
  staleness:
    - Precedence layer 2: "Provider defaults — the manifest's default_model, default_provider, etc." → drop default_provider.
    - Bootstrap: preference order omits qwen-code → add it; "pi needs a default_provider ... set [provider.pi]
      default_provider" → rewrite to the model-prefix.
    - config upgrade: "current schema version (2)" + "Already at version 2" + "preserved byte-for-byte" → version
      3; describe the FR-B7 fold (default_provider folded into the model slash-prefix, key deleted; NOT byte-for-
      byte); keep idempotent.
    - "current schema (version 2)" → version 3 (adds reasoning, model-prefix).
    - Populated config example: `config_version = 2` → 3; add `# reasoning = "off"` to [defaults].
    - Env table: MISSING `STAGECOACH_REASONING` + `STAGECOACH_<ROLE>_REASONING` (4 roles) → ADD; the
      `STAGECOACH_MESSAGE_PROVIDER/MODEL` rows marked "(no flag)" → correct (now mirror --message-provider/--message-model).
    - Bottom note: "The message role has no CLI flag" → DELETE (v3 adds --message-provider/--message-model/--message-reasoning).

- file: docs/cli.md   # EDIT — the CLI reference
  staleness:
    - providers list / config init prose preference orders: omit qwen-code → add it (8 providers).
    - config init: "config_version = 2" + "set [provider.pi] default_provider" → version 3 + model-prefix.
    - config upgrade: "current schema version (2)" + "version 2" messages → 3; add the FR-B7 fold description.
    - Global flags table: MISSING `--reasoning` + `--planner/stager/message/arbiter-reasoning` + `--message-provider`/
      `--message-model` → ADD them (with env var + git-config + default: reasoning default off / planner high).
    - Flag↔env↔git-config map: MISSING reasoning rows + message rows → ADD.
    - NOTE "The message role has no CLI flag (--message-provider/--message-model do not exist)" → DELETE.

- file: internal/config/bootstrap.go   # EDIT — comment/annotation strings ONLY
  staleness:
    - Line ~133 (code comment): "pi's gpt-5.4* models require a sub-provider (default_provider) to route" →
      rewrite to the model-prefix.
    - Lines ~148-149 (b.WriteString annotation strings written into config init output): "# NOTE: pi requires a
      default_provider (sub-provider) ... set [provider.pi] default_provider" → rewrite to describe the
      inference-provider prefix on the model (FR-R5b), e.g. "# NOTE: pi is a multi-backend provider — prefix the
      model with your inference backend, e.g. model = \"zai/glm-5.2\" (a bare model is a config error, FR-R5b).
      # The shipped per-role models are empty so you can supply your own backend/model."
  gotcha: "COMMENT/STRING-ONLY. The bootstrap LOGIC (pi per-role models written empty) is ALREADY correct for v3
    — do not touch it. Only the misleading default_provider wording changes. Run `make build && make test` after
    to confirm no behavior change (the annotations are comments in the output, not parsed)."
```

### Current Codebase tree (relevant subset)

```bash
README.md                          # EDIT — marketing surface
docs/README.md                     # (verify: provider-count mentions; minor)
docs/providers.md                  # EDIT — manifest reference
docs/configuration.md              # EDIT — config reference
docs/cli.md                        # EDIT — CLI reference
docs/how-it-works.md               # EDIT — add T_start freeze + one-file short-circuit
internal/config/bootstrap.go       # EDIT — comment/annotation strings ONLY (config init output)
internal/provider/{manifest,registry,builtin}.go  # CONSUMED (verified source of truth — schema, order, built-ins)
internal/cmd/root.go               # CONSUMED (verified flag surface)
internal/config/{config,migrate}.go # CONSUMED (version=3, FR-B7 fold semantics)
providers/*.toml                   # CONSUMED (already v3-correct — do NOT edit)
```

### Desired Codebase tree (files this task EDITS — no new files, no logic changes)

```bash
README.md                          # hero/quick-start/config → v3 model-prefix + --reasoning + qwen-code + freeze
docs/providers.md                  # schema table (no default_provider; +reasoning_levels/experimental) + render +
                                   #   qwen-code row + FR-D4 note
docs/configuration.md              # precedence/bootstrap + config upgrade v3 fold + reasoning/--message-* env
docs/cli.md                        # global flags + map (+reasoning/--message-*) + v3 + qwen-code + upgrade fold
docs/how-it-works.md               # decompose section: +T_start freeze (FR-M1b/M1c) + one-file short-circuit (FR-M2b)
internal/config/bootstrap.go       # COMMENT-ONLY: default_provider annotations → model-prefix (no logic change)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- G-DEFAULT-PROVIDER-REMOVED: default_provider is GONE from the manifest schema (P1.M1.T1.S1) and config.
     The ONLY acceptable default_provider mentions in the final docs are HISTORICAL/migration context
     (e.g. "v2's default_provider field was folded into the model prefix by config upgrade, FR-B7"). Any mention
     that presents it as a field you can SET is a bug. Grep-assert: `grep -rn "default_provider" README.md docs/`
     must show only migration context. -->

<!-- G-AGENT-TERMINOLOGY: PRD §16.4 (an intermediate v3 edit) used `agent = "pi"`, `agent = "agy"`, and
     `--planner-agent agy`. This is the ABANDONED intermediate that FR-B7 maps BACK to `provider`. The shipped
     binary uses `provider`/`[provider.*]`/`--<role>-provider` exclusively (verified: root.go registers
     --planner-provider, NOT --planner-agent). Align every example to `provider`. Grep-assert: no
     `--planner-agent`, no `agent = "`, no `[agent.`. -->

<!-- G-MODEL-PREFIX-FORM: multi-backend providers (pi, opencode) REQUIRE the inference backend as a slash-prefix
     on the model (FR-R5b): `zai/glm-5.2`, `openai/gpt-5.4`. A bare model (no `/`) on pi is a HARD ERROR, never a
     silent bare --model. Single-backend providers (claude, codex, cursor, gemini, agy, qwen-code) take a bare
     model. README/docs examples MUST use the prefix form for pi/opencode and bare for the rest. Canonical pi
     example: `model = "zai/glm-5.2"`. -->

<!-- G-VERSION-3: CurrentConfigVersion = 3 (config.go:18). Every docs reference to the schema version, the
     `config_version = N` in examples, and the `config upgrade` "at version N" messages MUST say 3. Grep-assert:
     no "version 2" / "config_version = 2" in README.md docs/. -->

<!-- G-CONFIG-UPGRADE-FOLDS-NOT-BYTEPRESERVED: v2 docs (wrongly) said config upgrade is "preserved byte-for-byte".
     v3 (FR-B7) FOLDS a multi-backend provider's former default_provider into the model slash-prefix and deletes
     the key — so it IS a rewrite (only the model line + the deleted key change; everything else preserved).
     Keep the 'idempotent' property. Do NOT carry over the byte-for-byte claim. -->

<!-- G-MESSAGE-FLAGS-EXIST: v2 had NO --message-* flags (a documented gap). v3 CORRECTS this (FR-R3: every role
     exposes all three flags including message). root.go registers --message-provider/--message-model/
     --message-reasoning. So the cli.md + configuration.md "message role has no CLI flag" NOTE is STALE — delete
     it and ADD the message rows to the flag/env tables. -->

<!-- G-REASONING-DEFAULTS: shipped defaults are planner=high, stager=message=arbiter=off (FR-R6). Reasoning is a
     graceful no-op when the provider/model lacks reasoning control (never an error). Document --reasoning
     (global) + --<role>-reasoning (4 roles) + STAGECOACH_REASONING / STAGECOACH_<ROLE>_REASONING. -->

<!-- G-QWEN-CODE-EXPERIMENTAL: qwen-code is single-backend (Qwen/DashScope), a Gemini-CLI fork, marked
     experimental (§12.5.2), and CANNOT serve as the stager (empty tooled_flags → FR-D4 fallback). It sits
     between gemini and codex in the preference order. Document it as experimental + non-stager. -->

<!-- G-BOOTSTRAP-COMMENT-ONLY: the bootstrap.go edit is COMMENT/ANNOTATION STRINGS ONLY. The logic (pi per-role
     models written empty) is ALREADY correct for v3 (the user supplies the backend prefix). Only the misleading
     default_provider wording in the output annotations changes. A string change in a WriteString does not alter
     runtime behavior — but still run `make build && make test` to confirm. -->

<!-- G-SCOPE-NO-CODE-LOGIC: this is a Mode B doc task. The ONLY source file touched is bootstrap.go, and ONLY its
     comment/annotation strings. Do NOT edit any other .go file, any providers/*.toml, the PRD, tasks.json, or
     .gitignore. The parallel P4.M1.T1.S2 (e2e harness) touches only test files — no overlap. -->
```

## Implementation Blueprint

### Data models and structure

No data models — this is a documentation task. The "data" is the verified v3 source facts the docs must match
(see Documentation & References: manifest.go field set, registry.go preference order, root.go flag surface,
config.go version=3, migrate.go FR-B7 fold, bootstrap.go annotation lines).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/providers.md — the manifest schema + rendering + provider table (the technical reference)
  - SCHEMA TABLE: DELETE the `default_provider` row. ADD two rows:
      `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists (off/low/medium/high);
        nil/empty ⇒ graceful no-op (FR-R6). Rendered at Render after the model flag.
      `experimental`   | bool  | false       | Marks a provider experimental (agy, qwen-code) — surfaced in
        `providers list`/`show`. Absent/false ⇒ stable.
    Update the field COUNT in the intro prose (was "18-field"; now 19 with default_provider removed and
    reasoning_levels + experimental added). Keep all other rows.
  - COMMAND RENDERING section: ADD a paragraph after the token-order block: "For a multi-backend provider
    (one whose manifest sets `provider_flag` — pi today), the model is `inference/model` (e.g. `zai/glm-5.2`):
    Render splits it on the first `/` and emits `--provider <prefix> --model <rest>` (FR-R5b). A model with no
    `/` on such a provider is a HARD configuration error, never a silent bare `--model`. Single-backend
    providers take the model verbatim. When a `reasoning` level resolves to a non-empty token list in
    `reasoning_levels`, those tokens are appended after the model flag (FR-R6); absent/empty ⇒ silent no-op."
  - "The N built-in providers" HEADER + TABLE: change the header to "The 8 built-in providers". ADD a qwen-code
    row: Delivery stdin; Print `-p`; Model `-m`; Default `qwen3-coder-plus` ⚠️; System prompt (prepended);
    Tool-disable read-only constraint (`--approval-mode default`); Stager — no; experimental. Ensure the table
    now lists all 8 (pi, claude, gemini, opencode, codex, cursor, agy, qwen-code).
  - "Per-role default models (FR-D4)": rewrite the pi note from "pi needs a default_provider to route its
    gpt-5.4* models" → "pi needs an inference-provider prefix on the model (FR-R5b); its shipped per-role models
    are blank so you supply backend/model (e.g. `zai/gpt-5.4`)".
  - PRESERVE: the tools-disable asymmetry, tooled-mode/stager, output-parsing sections (still accurate).
  - NAMING/PLACEMENT: keep the existing section order; edit in place.

Task 2: EDIT docs/configuration.md — precedence, bootstrap, upgrade, env/flag tables
  - PRECEDENCE layer 2: "Provider defaults — the manifest's `default_model`, `provider_flag`, etc." (drop default_provider).
  - BOOTSTRAP: add qwen-code to the preference order (8 providers); rewrite the pi note to the model-prefix.
  - SCHEMA VERSIONING (config upgrade): version 2 → 3. Replace "preserved byte-for-byte" with the FR-B7 fold:
    "rewrites the top-level `config_version` to 3 AND, for any multi-backend provider, folds its former
    `default_provider` into a slash-prefix on its model (`default_provider = \"X\"` + `model = \"Y\"` →
    `model = \"X/Y\"`) and deletes the `default_provider` key. Every other line is preserved. Idempotent."
    Update the example messages ("version 3"). Update "current schema (version 2)" → version 3 + the v3 feature
    list (reasoning, model-prefix inference provider).
  - POPULATED CONFIG example: `config_version = 2` → 3; add `# reasoning = "off"  # off|low|medium|high; planner
    defaults to high (FR-R6)` to [defaults].
  - ENVIRONMENT VARIABLES table: ADD rows for `STAGECOACH_REASONING` (→ --reasoning) and
    `STAGECOACH_<ROLE>_REASONING` for planner/stager/message/arbiter. CORRECT the `STAGECOACH_MESSAGE_PROVIDER`/
    `STAGECOACH_MESSAGE_MODEL` rows: they now mirror `--message-provider`/`--message-model` (remove "(no flag)").
  - BOTTOM NOTE: DELETE "The message role has no CLI flag" (v3 adds --message-*); replace with a one-liner:
    "Every role (including message) exposes `--<role>-provider`/`--<role>-model`/`--<role>-reasoning` (FR-R3)."
  - PRESERVE: file paths, git-config-keys table, the [generation] opt-in-override note (accurate).

Task 3: EDIT docs/cli.md — global flags + map tables + subcommand prose
  - GLOBAL FLAGS table: ADD rows — `--reasoning <level>` (string, "" → off / planner high, STAGECOACH_REASONING,
    stagecoach.reasoning, "off|low|medium|high"); `--planner-reasoning`/`--stager-reasoning`/
    `--message-reasoning`/`--arbiter-reasoning` (string, "", STAGECOACH_<ROLE>_REASONING); `--message-provider`
    (string, "", STAGECOACH_MESSAGE_PROVIDER); `--message-model` (string, "", STAGECOACH_MESSAGE_MODEL).
  - FLAG↔ENV↔GIT-CONFIG MAP: ADD rows for --reasoning, the 4 --<role>-reasoning, --message-provider, --message-model.
  - `providers list` + `config init` prose: add qwen-code to the preference order (8 providers).
  - `config init`: `config_version = 2` → 3; rewrite the "set [provider.pi] default_provider" note to the
    model-prefix.
  - `config upgrade`: version 2 → 3; add the FR-B7 fold description (default_provider folded into model prefix,
    not byte-for-byte); keep idempotent; fix the example messages to "version 3".
  - DELETE the NOTE "The message role has no CLI flag (--message-provider/--message-model do not exist)".
  - EXAMPLES: keep `--planner-provider claude --planner-model opus` (claude is single-backend → bare model is
    correct); OPTIONALLY add a reasoning example (`--reasoning high`) and a pi model-prefix example.
  - PRESERVE: synopsis, exit codes, the --config/--dry-run notes (accurate).

Task 4: EDIT docs/how-it-works.md — the v3 decompose freeze (contract clause d)
  - In the "Multi-commit decomposition" section (line 47+), ADD (as a new subsection or an extension of "Pipeline
    flow"/"Key design points"/"Safety"):
      (1) START-OF-RUN FREEZE (T_start) — FR-M1b/M1c: the instant decomposition activates, the entire working-
          tree change set (every modified/added/deleted/untracked path AND its byte content) is captured as an
          immutable tree object T_start. The planner partitions T_start's diff (never a fresh re-read of the live
          tree); every stager, the arbiter's leftover staging, and the one-file/single shortcuts stage content
          drawn STRICTLY from T_start. A file created/modified after T_start is captured is invisible to the run.
      (2) FREEZE ENFORCEMENT — because the stager is an external agent running git against the live tree, after
          each staging step stagecoach verifies the resulting tree is a content-SUBSET of T_start (only T_start
          paths, T_start content). Any deviation (a concurrent change swept in, or a stager that ran a bare
          `git add -A`) is a HARD abort (non-rescue; already-landed commits stand per FR-M12).
      (3) ONE-FILE SHORT-CIRCUIT — FR-M2b: in auto-decompose, if exactly ONE path changed, the planner is
          bypassed entirely (stage that file's T_start content, one message, one commit). Deterministic, not
          model judgment. `--commits N` (N≥2) overrides this.
  - PRESERVE: the existing per-concept tree[i] freeze + stager-safety text (still accurate; T_start is the
    start-of-run analog, complementary). Keep the §13.4 snapshot/stage-while-generating diagram (single-commit
    path — unchanged by the freeze; the freeze is decompose-only).
  - NAMING: keep markdown heading levels consistent with the file's existing structure.

Task 5: EDIT README.md — the marketing surface (PRD §21.5)
  - HERO: add qwen-code to the provider list ("Claude Code, Codex, Gemini CLI, pi, opencode, agy, qwen-code, or Cursor").
  - INSTALL prerequisite: add agy + qwen-code ("a coding-agent CLI already installed ... (pi, Claude Code, Gemini
    CLI, opencode, Codex, Cursor, agy, or qwen-code)").
  - "Configure your agent": preference order → 8 providers (add qwen-code between gemini and codex).
  - The `pi is a multi-provider agent` NOTE + the `config init` note: rewrite to the model-prefix — "pi is a
    multi-backend provider: the inference backend is a slash-prefix on the model (`zai/glm-5.2`); a bare model
    on pi is a config error. Set `git config stagecoach.model zai/glm-5.2` (or `[defaults] model = \"zai/glm-5.2\`").
  - QUICK START: add a `--reasoning` example (e.g. `stagecoach --reasoning high`); in "Configure your agent" show
    the model-prefix form for pi (`stagecoach.model zai/glm-5.2`) alongside the existing bare-model claude example.
  - MULTI-COMMIT DECOMPOSITION blurb: add one sentence on the v3 T_start freeze ("A start-of-run freeze captures
    your entire change set up front, so files you change mid-run are excluded from every commit — the run only
    ever commits what existed when it started."); keep the existing stager-safety sentence.
  - FAQ "Which agents are supported?": "Seven built-ins" → "Eight built-ins"; add "**qwen-code** *(experimental)*".
  - PRESERVE: the snapshot-workflow diagram (§13.4, single-commit path — accurate), the lazygit binding, the
    "Adding a new agent" example (a user-defined single-backend provider — bare `default_model` is correct there).

Task 6: EDIT internal/config/bootstrap.go — COMMENT/ANNOTATION STRINGS ONLY (config init output coherence)
  - Line ~133 (code comment): "pi's gpt-5.4* models require a sub-provider (default_provider) to route" →
    "pi is a multi-backend provider: the model must carry the inference backend as a slash-prefix (FR-R5b)".
  - Lines ~148-149 (the b.WriteString annotation strings written into the generated config): replace the
    default_provider wording with the model-prefix. E.g.:
      "# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n"
      "# e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b).\n"
      "# The shipped per-role models are empty so you can supply your own backend/model.\n"
    Keep the existing multi-line string-builder style (b.WriteString per line) so the generated config reads cleanly.
  - DO NOT touch the LOGIC: the pi per-role models are written EMPTY — that is correct for v3 (the user supplies
    the backend/model prefix). Only the explanatory comment/annotation wording changes.
  - VERIFY after: `make build && make test` green; `stagecoach config init` (in a temp dir) produces a config
    whose comments reference the model-prefix, not default_provider.
```

### Implementation Patterns & Key Details

```markdown
<!-- The model-prefix example to use EVERYWHERE for pi (the canonical v3 form): -->
  model = "zai/glm-5.2"      # [defaults] / [role.planner] / git config stagecoach.model

<!-- The single-backend example (claude) — BARE model, NO prefix: -->
  model = "sonnet"           # claude is single-backend; a prefix here is wrong

<!-- The preference order (verbatim FR-D1 / registry.go preferredBuiltins) — use in EVERY list: -->
  pi, opencode, cursor, agy, gemini, qwen-code, codex, claude   (8 providers; qwen-code between gemini and codex)

<!-- The reasoning flag surface (verbatim root.go) — add to EVERY flag/env table: -->
  --reasoning <off|low|medium|high>      (global; STAGECOACH_REASONING; stagecoach.reasoning; default off, planner: high)
  --planner-reasoning / --stager-reasoning / --message-reasoning / --arbiter-reasoning
                                         (per-role; STAGECOACH_<ROLE>_REASONING; all FOUR roles incl. message)
  --message-provider / --message-model  (per-role message; STAGECOACH_MESSAGE_PROVIDER / _MODEL)
  # NOTE: v2 had NO --message-* flags; v3 CORRECTS this (FR-R3). Delete every "message has no flag" note.

<!-- The config upgrade v3 message (FR-B7 fold, NOT byte-for-byte): -->
  "Upgraded config at <path> to version 3."   (and on the v2→v3 path: default_provider folded into the model prefix)
  Idempotent: a second run is a no-op.

<!-- The T_start freeze one-liner (README decompose blurb + how-it-works): -->
  "A start-of-run freeze (T_start) captures your entire change set up front; files you change mid-run are
   excluded from every commit — the run commits only what existed when it started (FR-M1b/M1c)."

<!-- The bootstrap.go annotation rewrite (COMMENT/STRING-ONLY — do not change logic): -->
  // pi is a multi-backend provider: the model must carry the inference backend as a slash-prefix (FR-R5b).
  ...
  b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
  b.WriteString("# e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
  b.WriteString("# The shipped per-role models are empty so you can supply your own backend/model.\n")
```

### Integration Points

```yaml
DOCS (markdown):
  - README.md, docs/{providers,configuration,cli,how-it-works}.md → synced to v3 (model-prefix, reasoning,
    qwen-code, T_start freeze, version 3, no agent-term, no default_provider).
SOURCE (comment-only):
  - internal/config/bootstrap.go → annotation/comment strings only (default_provider → model-prefix). NO logic.
CONSUMED (read-only source of truth — verified this pass):
  - internal/provider/{manifest,registry,builtin}.go (schema fields, 8-provider order, built-ins)
  - internal/cmd/root.go (flag surface: --reasoning + 4×--<role>-reasoning + --message-*)
  - internal/config/{config,migrate}.go (version=3, FR-B7 fold)
  - providers/*.toml (already v3-correct — do NOT edit)
NO production-logic changes. NO go.mod changes. NO new files. NO PRD/tasks.json edits.
```

## Validation Loop

> This is a Mode B documentation task. There are NO unit tests for prose. Validation = (a) deterministic GREP
> assertions proving the staleness is purged and the new v3 surface is present + self-consistent across the 5
> docs, (b) `make build`/`make test` green (the bootstrap.go edit is comment/annotation strings only → no
> behavior change, but verify), (c) a manual coherence read of the 5 docs.

### Level 1: Purge assertions (the staleness is gone)

```bash
# (1) NO live default_provider references (migration/upgrade historical context is acceptable):
grep -rn "default_provider" README.md docs/ internal/config/bootstrap.go
# Expected: at most migration/upgrade-context lines (e.g. "v2's default_provider was folded ... by config
#   upgrade"). NO line presenting it as a settable field. bootstrap.go annotation lines must be gone.

# (2) NO abandoned agent terminology:
grep -rniE "planner-agent|stager-agent|arbiter-agent|\[agent\.|agent = \"|--planner-agent|--stager-agent" README.md docs/
# Expected: empty.

# (3) NO stale "version 2" schema references:
grep -rniE "version 2|config_version = 2|at version 2|schema version \(2\)|schema \(version 2\)" README.md docs/
# Expected: empty. (config_version = 3 present in examples — see Level 2.)

# (4) NO stale "message role has no CLI flag" note:
grep -rni "message role has no CLI flag\|--message-provider.*do not exist\|message.*no flag" docs/ README.md
# Expected: empty.

# (5) NO "byte-for-byte" claim for config upgrade:
grep -rni "byte-for-byte" docs/configuration.md docs/cli.md
# Expected: empty (v3 config upgrade FOLDS default_provider into the model prefix — not byte-for-byte).
```

### Level 2: Presence assertions (the v3 surface is documented)

```bash
# (6) qwen-code in every relevant doc + the full 8-provider order:
grep -rln "qwen-code" README.md docs/providers.md docs/configuration.md docs/cli.md
# Expected: all 4 files listed.
grep -rniE "pi, opencode, cursor, agy, gemini, qwen-code, codex, claude" README.md docs/
# Expected: ≥1 hit per preference-order mention (the canonical 8 order).

# (7) reasoning flag surface documented:
grep -rn -- "--reasoning" docs/cli.md docs/configuration.md
grep -rn "STAGECOACH_REASONING\|STAGECOACH_.*_REASONING" docs/configuration.md
grep -rn "reasoning_levels" docs/providers.md
# Expected: hits in each.

# (8) message flags documented (v2 gap closed in docs):
grep -rn -- "--message-provider\|--message-reasoning" docs/cli.md docs/configuration.md
# Expected: hits.

# (9) model-prefix form shown for multi-backend providers:
grep -rn "zai/glm-5.2" README.md docs/
grep -rni "slash-prefix\|inference provider is.*prefix\|model.*prefix" docs/providers.md docs/configuration.md README.md
# Expected: hits (the v3 model form + the FR-R5b explanation).

# (10) config_version = 3 in examples + config upgrade version 3:
grep -rn "config_version = 3" docs/configuration.md
grep -rn "version 3" docs/cli.md docs/configuration.md
# Expected: hits.

# (11) T_start freeze documented in how-it-works (+ README blurb):
grep -rni "T_start\|start-of-run freeze\|FR-M1b\|FR-M2b\|one-file" docs/how-it-works.md README.md
# Expected: hits in how-it-works.md (README may use the one-liner form).

# (12) schema table: reasoning_levels + experimental present, default_provider absent:
grep -n "reasoning_levels\|experimental" docs/providers.md   # present
grep -n "^| .default_provider" docs/providers.md             # empty (row removed)
```

### Level 3: Build & test (the bootstrap.go comment-only edit is safe)

```bash
make build     # must succeed (bootstrap.go comment/string change compiles)
make test      # full suite green — the bootstrap annotation change is comment/string-only (no behavior change);
               #   verify no config-init test asserts the OLD default_provider wording (if one does, update the
               #   test's expected-string to the model-prefix wording — the annotation text is part of the
               #   shipped UX, so the test SHOULD follow the new wording). go.mod/go.sum UNCHANGED.

# Smoke the config init output actually reads default_provider-free:
tmp=$(mktemp -d) && STAGECOACH_CONFIG=$tmp/c.toml ./bin/stagecoach config init --force >/dev/null 2>&1
grep -i "default_provider" $tmp/c.toml && echo "STALE annotation present" || echo "OK: no default_provider in generated config"
rm -rf $tmp
```

### Level 4: Coherence review (manual — the docs read as one consistent v3 story)

```bash
# Read the 5 docs end-to-end (or section-by-section) and confirm:
#   - The 8-provider order is identical everywhere it appears (FR-D1 verbatim).
#   - pi/opencode ALWAYS show a prefixed model; single-backend providers ALWAYS show a bare model.
#   - The reasoning levels + shipped defaults (planner=high, others=off) + graceful-no-op are consistent.
#   - config_version is 3 everywhere; config upgrade describes the FR-B7 fold.
#   - The decompose description (README blurb + how-it-works) matches: T_start freeze + one-file short-circuit.
#   - The "Adding a new agent" / user-provider example stays single-backend (bare default_model) — UNCHANGED.
# Cross-check any numbers against `stagecoach --help` (the binary is authoritative per docs/README.md).
```

## Final Validation Checklist

### Technical Validation

- [ ] All Level 1 PURGE greps (1)–(5) return empty (or migration-context-only for default_provider).
- [ ] All Level 2 PRESENCE greps (6)–(12) return the expected hits.
- [ ] `make build` succeeds; `make test` green; go.mod/go.sum UNCHANGED.
- [ ] The bootstrap.go diff is comment/annotation STRINGS only (no logic change) — verified by `git diff`.
- [ ] `stagecoach config init` output is default_provider-free (Level 3 smoke).

### Feature Validation

- [ ] README hero/install/FAQ preference lists include qwen-code (8 providers) and use the FR-D1 order.
- [ ] README + docs config/model examples use `zai/glm-5.2` for pi (multi-backend) and bare models for single-backend.
- [ ] `--reasoning` + 4×`--<role>-reasoning` + `--message-provider`/`--message-model` documented in cli.md + configuration.md.
- [ ] docs/providers.md schema table has NO default_provider row; HAS reasoning_levels + experimental; command-
      rendering documents the FR-R5b split + reasoning emit; qwen-code is a proper table row; header says 8.
- [ ] docs/configuration.md + docs/cli.md: config upgrade = version 3 + FR-B7 fold (not byte-for-byte); version 2 gone.
- [ ] docs/how-it-works.md decompose section describes T_start freeze + freeze enforcement + one-file short-circuit.
- [ ] No `agent`/`[agent.*]`/`--planner-agent` terminology anywhere in README.md docs/.

### Code Quality Validation

- [ ] Markdown is well-formed (tables render; headings consistent; no broken links).
- [ ] Doc edits follow each file's existing tone/structure (PRD §21.5 for README; the existing section order for docs/).
- [ ] The bootstrap.go annotation strings read cleanly as comments in the generated config (multi-line WriteString).
- [ ] No scope creep: only README.md + 5 docs + bootstrap.go(comment-only) edited; no other .go / .toml / PRD touched.

### Documentation & Deployment

- [ ] The docs are internally self-consistent (same 8-provider order, same model-prefix form, same reasoning
      defaults, same version 3) across README + docs/.
- [ ] The docs match the shipped binary (`stagecoach --help` is authoritative; docs/README.md already states this).
- [ ] No new env vars / config keys / flags are INVENTED — only documented (all were shipped in P1–P3).
- [ ] The config init generated-file annotations (bootstrap.go) explain the v3 model-prefix, not default_provider.

---

## Anti-Patterns to Avoid

- ❌ Don't invent doc content not in the v3 changeset — every change maps to a verified source fact (manifest.go,
  registry.go, root.go, config.go, migrate.go) or the delta_prd. Inventing fields/flags/behaviors creates NEW drift.
- ❌ Don't change bootstrap.go LOGIC — pi per-role models are already correctly EMPTY for v3 (user supplies the
  backend/model prefix). Only the misleading default_provider COMMENT/ANNOTATION wording changes. Touching the
  logic risks a regression in a COMPLETED task's (P1.M3.T1) deliverable.
- ❌ Don't edit any other .go file, any providers/*.toml, PRD.md, tasks.json, prd_snapshot.md, or .gitignore —
  this is a Mode B doc task with ONE comment-only source exception (bootstrap.go annotations).
- ❌ Don't carry over the v2 "config upgrade is byte-for-byte" claim — v3 (FR-B7) FOLDS default_provider into the
  model prefix; keeping the old claim is a correctness bug in the docs.
- ❌ Don't use a bare model in a pi/opencode example, and don't use a prefixed model in a single-backend example —
  the model-prefix is the v3 headline (FR-R5b) and a bare model on pi is a HARD error. Get this right everywhere.
- ❌ Don't omit qwen-code or use a non-FR-D1 order — the 8-provider order (pi, opencode, cursor, agy, gemini,
  qwen-code, codex, claude) is verbatim from registry.go preferredBuiltins and must be identical everywhere.
- ❌ Don't keep any "message role has no CLI flag" note — v3 CORRECTS that gap (--message-provider/--message-model/
  --message-reasoning all exist). The note is now false and must be deleted, not amended.
- ❌ Don't forget the T_start freeze in how-it-works.md / README — contract clause (d) explicitly requires the
  decompose description match the v3 freeze behavior (FR-M1b/M1c) + one-file short-circuit (FR-M2b).
- ❌ Don't conflict with the parallel P4.M1.T1.S2 — it's test-only (internal/e2e + decompose_test.go), no doc
  overlap, so just don't touch test files.
- ❌ Don't skip the grep validations — a docs task's "tests" ARE the grep assertions + the build/test green. A
  passing `make test` with a stale default_provider reference in the docs is still a failed deliverable.
