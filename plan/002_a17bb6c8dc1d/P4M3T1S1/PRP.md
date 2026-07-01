---
name: "P4.M3.T1.S1 — Sync changeset-level documentation for all v2.0 features. EDIT README.md (the marketing surface, PRD §21.5) + docs/how-it-works.md + docs/cli.md + docs/configuration.md + docs/providers.md + docs/README.md so every user-facing doc reflects the complete v2.0 feature set: multi-commit decomposition (§13.6, the headline), per-role provider/model config (§16.4/§9.15), binary/non-text filtering (§9.1), the agy provider (§12.5.1), cascading provider priority (§9.16), and the populated config bootstrap + schema versioning (§9.17). This is the FINAL Mode B 'Sync changeset-level documentation' task (plan/002 §5 Phase 6); it depends on all P1–P4 implementation being complete."
description: |

  EDIT (no new files) six Markdown files: README.md, docs/README.md, docs/how-it-works.md,
  docs/cli.md, docs/configuration.md, docs/providers.md. These are the user-facing marketing +
  overview surfaces; the PRD (PRD.md) and the shipped binary are the AUTHORITATIVE sources — these
  docs are DERIVED from them. The whole job is: reconcile each doc with the shipped v2.0 binary and
  the PRD, fixing stale v1-only statements and adding the v2 features.

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  CURRENT STATE (verified 2026-07-01; see research/cli_surface.md + research/providers_and_config.md):
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  docs/configuration.md and docs/providers.md are ALREADY MOSTLY v2-correct (they were edited in P1/P3
  for per-role config, config_version, populated bootstrap, agy, tooled_flags). They need SMALL fixes:
  providers.md has a STALE auto-detect order ("pi, claude, gemini, opencode, codex, cursor, agy" — the
  binary's real order is "pi, opencode, cursor, agy, gemini, codex, claude"); configuration.md is
  MISSING the config upgrade section + per-role env/flag reference + decompose config keys.

  README.md, docs/how-it-works.md, docs/cli.md, docs/README.md are STILL v1-ONLY and MATERIALLY STALE
  (delta_prd §5: "README.md is materially stale after this changeset"). The three headline failures:
  (1) README's FAQ literally says multi-commit is "Not in v1 … planned for v2" — now FALSE; (2) the
  provider preference order quoted in README.md and docs/cli.md ("pi, claude, gemini, opencode, codex,
  cursor") and docs/providers.md (agy appended at the end) is WRONG vs the binary; (3) none mention
  the --commits/--single/--max-commits/per-role flags, the decompose pipeline, binary filtering, or
  the populated config init / config upgrade.

  SCOPE BOUNDARY (frozen — do NOT edit):
    - PRD.md, plan/002/**/prd_snapshot.md, plan/002/**/tasks.json — NEVER modify (orchestrator-owned).
    - .gitignore — never modify.
    - All Go source (.go files) — this is a DOCS task; the binary is the input, not the output.
    - providers/*.toml — these are manifest reference files, NOT docs (leave them; they are accurate).
    - docs/README.md note about "binary is authoritative" — keep that contract (docs must match binary).

  DELIVERABLE: 6 Markdown files edited so that a NEW USER can discover and use multi-commit
  decomposition, per-role models, binary filtering, agy, the cascading default, and the populated config
  bootstrap / upgrade — from the docs ALONE — with ZERO stale v1 statements remaining.

---

## Goal

**Feature Goal**: Reconcile all six user-facing documentation files (README.md + 5 docs/*.md) with the
shipped v2.0 binary and the PRD, so that every doc reflects the complete v2.0 feature set and contains
no stale v1-only statements. The headline is **multi-commit decomposition**: a new user reading the
README or docs/how-it-works.md must be able to discover it, understand the four-role pipeline, and run
`stagehand --commits 3` / `--single` / `--planner-model …` from the docs alone.

**Deliverable**: Six EDITED Markdown files (no new files):
1. `README.md` — hero pitch mentions multi-commit; new "Multi-commit decomposition" section; updated
   feature list; populated `config init` + `config upgrade` in the config section; v2.0 example
   invocations (PRD §15.5); FIXED provider preference order; FIXED "Can it write multiple commits?" FAQ.
2. `docs/how-it-works.md` — new "Multi-commit decomposition" section with the §11.4 pipeline data-flow
   diagram + the four roles + overlapped staging/generation; binary-filtering note.
3. `docs/cli.md` — all new flags (--commits/--single/--no-decompose/--max-commits + per-role) in the
   global-flags table + flag↔env↔git-config map; updated default-action description (decompose routing);
   `config upgrade` subcommand; updated `config init` (populated); v2 examples; FIXED auto-detect order.
4. `docs/configuration.md` — ADD `config upgrade` section + per-role env/flag reference + decompose
   config keys (commits/single/max_commits). (Per-role config + config_version + populated bootstrap
   are ALREADY present — verify, don't rewrite.)
5. `docs/providers.md` — FIX the auto-detect order to `pi, opencode, cursor, agy, gemini, codex, claude`.
   (agy + tooled_flags/stager column + per-role model table are ALREADY present — verify, don't rewrite.)
6. `docs/README.md` — update the index descriptions (cli.md "11 global flags" → count all; providers.md
   "6 built-in providers" → 7) and the v1.0 version note.

**Success Definition**:
- **No stale v1 statements**: `grep` for the known-stale strings (below) returns ZERO hits across all
  six files.
- **Binary is authoritative**: every CLI flag, env var, provider name, and config key documented MATCHES
  the shipped binary (verified by running `stagehand --help` / `config init --help` / `config upgrade
  --help` / `providers list` and diffing against the docs). In particular the provider preference order
  is exactly `pi, opencode, cursor, agy, gemini, codex, claude` (7 providers) everywhere it appears.
- **New-user discoverability**: the README + docs/how-it-works.md explain multi-commit decomposition
  (trigger, the four roles, the overlapped loop) and the docs/cli.md + docs/configuration.md document
  every new flag/env/config-key with at least one example.
- **Markdown validity**: all six files pass `markdownlint-cli2` against the repo's `.markdownlint.json`
  (MD013/MD033/MD060 disabled, so `<details>`/`<summary>` and long lines are fine); all internal doc
  links resolve.

## User Persona

**Target User**: a NEW Stagehand user (the PRD §7.1 "plan-holder") reading the docs to discover and use
v2.0 features. Secondary: a returning v1 user surprised that `pi`'s default changed / that bare
`stagehand` with a dirty tree now decomposes.

**Use Case**: the user clones the repo, reads README.md to understand what stagehand does, runs
`config init`, and wants to (a) commit a dirty working tree as multiple logical commits, (b) route
planning to a big model, (c) know which provider is auto-selected and why pi's default changed.

**User Journey**: README hero → "Multi-commit decomposition" section → `stagehand --commits 3` →
"Configure your agent" → `config init` (populated) → docs/cli.md for the full flag table →
docs/how-it-works.md for the pipeline → docs/configuration.md for per-role models.

**Pain Points Addressed**: (a) the FAQ LIE that multi-commit is "planned for v2" — users who want it
today are told it's missing; (b) a WRONG provider order that makes `providers list` output contradict
the docs; (c) v2 flags that exist in `--help` but not in any doc.

## Why

- **Business value**: this is the changeset-level documentation sync the delta_prd (§5 Phase 6)
  explicitly designates as the FINAL task, depending on all implementation being done. The v2.0
  headline feature (multi-commit) is invisible in the docs today — the marketing surface actively
  DENIES it exists. Shipping v2 with stale docs = shipping a feature no one can discover.
- **Integration with existing features**: documents the outputs of every P1–P4 milestone — per-role
  config (P1.M3), populated bootstrap + upgrade + versioning (P1.M4), binary filtering (P2.M1), the
  decompose pipeline + arbiter (P3), and the CLI flags + public API (P4.M1/M2). The docs are the
  *aggregation* of the whole changeset.
- **Problems this solves and for whom**: removes the three headline stale-statement failures (FAQ lie,
  wrong provider order, missing v2 flags) so the docs stop misleading users. For contributors: makes
  docs/README.md's "binary is authoritative" contract actually true (docs match the binary).

## What

**User-visible behavior** (the rendered docs):
- **README.md** hero mentions multi-commit decomposition as a workflow property; a new section shows
  the dirty-tree → `stagehand` → N commits flow with a short example; the competitive table shows
  multi-commit = Yes; the FAQ answers "Can it write multiple commits?" with YES + how; the provider
  list shows agy; the config section shows `config init` (populated) + `config upgrade`.
- **docs/how-it-works.md** has a "Multi-commit decomposition" section with the planner→stager→message→
  arbiter pipeline (the §11.4 diagram, rendered as a fenced code block), the four roles, the
  overlapped staging/generation, and the safety story; plus a binary-filtering note.
- **docs/cli.md** global-flags table + flag↔env↔git-config map include every new flag; the default-
  action description explains decompose routing; a `config upgrade` subcommand section exists; `config
  init` is described as populated; v2 examples are present; the auto-detect order is correct.
- **docs/configuration.md** documents `config upgrade`, per-role env vars + flags, and the decompose
  config keys.
- **docs/providers.md** auto-detect order is `pi, opencode, cursor, agy, gemini, codex, claude`.
- **docs/README.md** index descriptions are accurate (7 providers, correct flag count, v2.0 note).

**Technical requirements**: 6 Markdown edits. The doc CONTENT is constrained to match the shipped binary
(exact flag names/env vars/provider order/config keys — see the ground-truth tables in §All Needed
Context). NO prose is invented that contradicts the binary; where the binary's flag description strings
are themselves inaccurate (e.g. `--commits`/`--max-commits` cite env/git keys that don't exist — see
G-FLAG-DESC-ACCURACY), the docs describe the ACTUAL implemented behavior, not the flag's text.

### Success Criteria

- [ ] `grep -rn "Not in v1\|planned for v2\|single commit per invocation"` across README.md + docs/
      returns ZERO hits (the FAQ lie is gone).
- [ ] `grep -rn "pi, claude, gemini, opencode, codex, cursor"` (the stale 6-provider order, in any
      form) across README.md + docs/ returns ZERO hits; the order everywhere is
      `pi, opencode, cursor, agy, gemini, codex, claude`.
- [ ] README.md has a "Multi-commit decomposition" section + the hero mentions it; the "Can it write
      multiple commits?" FAQ answer is YES.
- [ ] docs/how-it-works.md has a "Multi-commit decomposition" section with the four roles + pipeline
      diagram + overlapped staging/generation.
- [ ] docs/cli.md global-flags table lists `--commits`, `--single`/`--no-decompose`, `--max-commits`,
      `--planner-provider/--planner-model`, `--stager-provider/--stager-model`,
      `--arbiter-provider/--arbiter-model`; the flag↔env↔git-config map has the per-role + commits rows;
      a `config upgrade` subcommand section exists; `config init` is described as populated; v2 examples
      exist.
- [ ] docs/configuration.md has a `config upgrade` section + per-role env/flag reference + decompose
      config keys.
- [ ] docs/providers.md auto-detect order is correct; agy is marked experimental + not stager-capable.
- [ ] docs/README.md index descriptions match (7 providers; v2.0 note; correct flag count wording).
- [ ] All six files pass `npx --yes markdownlint-cli2` (repo `.markdownlint.json`); internal doc links
      resolve (no broken `docs/*.md` / `../README.md` links).
- [ ] Cross-checked against the binary: `stagehand --help` / `config init --help` / `config upgrade
      --help` / `providers list` flag/provider names match the docs exactly.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ — YES. The exact current state of each doc is given (what's stale, what's already v2-
correct). The exact ground-truth the docs must match (flag names, env vars, provider order, config keys,
the decompose pipeline, the per-provider model table) is embedded verbatim from the source so the
implementer never has to re-read the Go code. The exact stale strings to eliminate are named. The PRD
sections to draw prose from are cited with their anchors.

### Documentation & References

```yaml
# MUST READ — PRD sections the prose is DERIVED from (PRD.md is read-only; quote/paraphrase, don't edit it)
- url: PRD.md §5 (Unique value proposition) + §21.5 (README structure)
  why: "§5 #6 is the 'right model for the right job' value prop to add to positioning; §5 lists the
        hero pitch candidate (mentions multi-commit). §21.5 is the 10-point README section ORDER — the
        new 'Multi-commit decomposition' content slots in as item 5b (after the single-commit quick
        start). The README hero pitch ALREADY has the multi-commit-aware candidate in §5."
  critical: "README must follow §21.5's 10-point structure; do not invent a new structure. The hero
        pitch in the current README.md is the §5 candidate minus multi-commit — ADD multi-commit to it."

- url: PRD.md §13.6 (Multi-commit decomposition) + §11.4 (Multi-commit pipeline data flow) + §9.14 (FR-M*)
  why: "The decompose pipeline for docs/how-it-works.md: the four roles (§13.6.2), the trigger
        (§13.6.1: nothing staged + dirty tree), the loop (§13.6.3), the arbiter (§13.6.5). §11.4 is the
        ASCII pipeline diagram to render. §9.14 FR-M1–M12 are the behavioral requirements."
  critical: "The trigger is NOTHING STAGED + dirty tree — if something is staged, the single-commit path
        runs unchanged (§13.6.1 / N1). The stager is the ONLY role that mutates the index (git add); all
        ref mutations are stagehand's (commit-tree/update-ref). Generation overlaps staging but
        publication is serialized."

- url: PRD.md §15.5 (Example invocations) + §9.1 (FR3a-c binary filtering) + §16.4 (per-role config)
  why: "§15.5 has the ready-made v2 example invocations (--commits 3 / --single / --planner-model /
        per-repo .stagehand.toml) — copy/adapt these into README + cli.md examples. §9.1 FR3a-c is the
        binary-filtering behavior to add a one-line note for in how-it-works.md. §16.4 is the per-role
        config block."
  critical: "§15.5's per-repo example uses .stagehand.toml with [role.planner] provider+model — keep
        that exact shape. FR3c: binary filtering applies in EVERY diff path (staged, working-tree
        snapshot, concept diff) with an identical '<status>\t[binary] <path>' placeholder."

- url: PRD.md §9.16 (FR-D1/D2/D3/D4 cascading provider priority + per-role tier defaults)
  why: "The provider auto-detect order (pi, opencode, cursor, agy, gemini, codex, claude), the
        decoupled-from-z.ai posture (pi default_model is now ''/OpenAI, NOT glm/zai), the tier→role
        strategy (planner=smart, stager=mid, message=fast, arbiter=mid)."
  critical: "FR-D2: pi no longer ships glm/zai as default — the README/providers docs must NOT claim pi
        defaults to glm/zai (the current README's `git config stagehand.model glm-5.2` example is a
        PERSONAL override, fine to keep as an example but do not call it 'the default')."

# CODEBASE FILES — edit targets (all 6 are Markdown; verified current state per file below)
- file: README.md   # EDIT (the marketing surface, PRD §21.5) — MATERIALLY STALE
  why: "The headline file. Currently v1-only: hero omits multi-commit; FAQ §'Can it write multiple
        commits?' says 'Not in v1 — planned for v2' (FALSE); provider order 'pi, claude, gemini,
        opencode, codex, cursor' (WRONG); 'git config stagehand.model glm-5.2' example; config init
        described as 'fully-commented global config file' (v1 inert, now populated); no v2 flags;
        competitive table has no multi-commit row."
  pattern: "Follow the §21.5 10-point structure that the file ALREADY uses (hero → demo → why-not →
        install → quick start → configure → snapshot workflow → full-ref → adding-agent → FAQ). INSERT
        a 'Multi-commit decomposition' subsection inside the Quick start section (after the single-commit
        steps, before lazygit binding). ADD multi-commit to the hero pitch (§5). UPDATE the FAQ answer.
        UPDATE the provider list example to include agy + the correct order. UPDATE the config section
        (config init populated + config upgrade). ADD a v2 examples block."
  gotcha: "README uses <details>/<summary> (inline HTML) + GitHub admonitions '> [!NOTE]' — these are
        fine because .markdownlint.json disables MD033 (inline HTML) and MD013 (line length). Keep the
        existing tone (concise, developer-facing). Do NOT touch the LICENSE TODO comment or the CI badge."

- file: docs/how-it-works.md   # EDIT — MATERIALLY STALE (v1-only)
  why: "The architecture overview. Currently covers only the single-commit snapshot flow + safety +
        prompt engineering. MISSING: the decompose pipeline (§11.4 diagram + four roles + overlapped
        staging/generation + arbiter), binary filtering. The existing 'Snapshot invariants' and
        'Safety' sections remain correct — EXTEND, don't rewrite."
  pattern: "ADD a top-level '## Multi-commit decomposition' section AFTER '## Stage-while-generating'
        (it composes the snapshot machinery N times). Render the §11.4 ASCII pipeline as a fenced code
        block. Explain the four roles (planner/stager/message/arbiter), the trigger (nothing staged +
        dirty tree), the overlapped loop (stager[i+1] ∥ message[i], tree[i] frozen before stager[i+1]
        starts, concept diff is tree[i-1]→tree[i] never index-vs-HEAD), serialized publication (CAS),
        and the arbiter leftover reconciliation. ADD a short 'Binary/non-text filtering' note (FR3a-c)
        under the diff-capture context."
  gotcha: "The decompose section must emphasize the SAME safety invariants as the single-commit section
        (snapshot-based, CAS update-ref, stagehand owns all ref mutations). The stager mutating the
        index is the ONE exception — frame it as 'scoped strictly to staging; stagehand owns commit/
        update-ref/push'. Cross-link to configuration.md for per-role models and cli.md for the flags."

- file: docs/cli.md   # EDIT — MATERIALLY STALE (v1-only)
  why: "The CLI reference. Currently lists 11 global flags with NO decompose/per-role flags; the
        providers-list auto-detect order is 'pi, claude, gemini, opencode, codex, cursor' (WRONG);
        config init described as 'Write a fully-commented example config' (v1 inert); no config upgrade
        subcommand; no v2 examples; the Synopsis says nothing staged → auto-stage-all (omits decompose
        routing)."
  pattern: "ADD the new flags to the Global flags table (exact rows in the GROUND-TRUTH table below).
        UPDATE the Synopsis + default-action description to explain: nothing staged + dirty tree + not
        opted out → DECOMPOSE (multi-commit); --single/--no-decompose/--commits 1 → v1 single commit;
        --dry-run forces the single-commit preview. ADD a 'config upgrade' subcommand section. UPDATE
        the 'config init' section to populated (flags --provider/--force/--template). FIX the providers
        list auto-detect order. ADD v2 rows to the flag↔env↔git-config map. ADD v2 examples (§15.5)."
  gotcha: "G-FLAG-DESC-ACCURACY: the binary's --commits/--max-commits flag DESCRIPTION STRINGS cite
        '(env/git stagehand.commits)' and '(env/git stagehand.max_commits)' but those env/git keys DO
        NOT EXIST in the implementation (only STAGEHAND_COMMITS exists for --commits; --max-commits has
        NO env and NO git key, only the [generation].max_commits config-file field). Document the ACTUAL
        behavior (see GROUND-TRUTH table), NOT the flag's aspirational description string. There is NO
        --message-provider/--message-model flag (message role inherits the global default) — do not
        invent one; only document the env (STAGEHAND_MESSAGE_*) for it."

- file: docs/configuration.md   # EDIT — SMALL ADDITIONS (mostly already v2-correct)
  why: "ALREADY has: 7-layer precedence, populated bootstrap (config init writes config_version=2 +
        [defaults] + [role.*]), per-role config explanation. MISSING: a 'config upgrade' section, the
        per-role ENV vars (STAGEHAND_<ROLE>_*) + per-role FLAGS (--<role>-provider/--<role>-model), and
        the decompose config keys (commits/single/max_commits under [generation])."
  pattern: "VERIFY the populated-bootstrap + config_version content is accurate (it is — from P1.M4).
        ADD a '### Schema versioning (`config upgrade`)' subsection under Bootstrap, documenting the
        config_version advisory + the upgrade command (no flags; 'already at version 2 (no changes)' /
        'Upgraded … to version 2' / errors if no file). ADD the per-role env/flag reference (see
        GROUND-TRUTH). ADD the decompose config keys note. Keep the existing per-role prose."
  gotcha: "Do NOT re-describe per-role config from scratch — it's present and correct. Only ADD the
        missing pieces. The git-config layer has NO role/commits keys — document per-role as env+flag+
        config-file ONLY (no git-config analog). config_version = 2 is the current schema (PRD §9.17)."

- file: docs/providers.md   # EDIT — ONE FIX (mostly already v2-correct)
  why: "ALREADY has: agy, the tooled_flags/stager-capable column, the per-provider per-role model
        table, the bare-vs-tooled mode explanation. ONE STALE FACT: the auto-detection order is quoted
        as 'pi, claude, gemini, opencode, codex, cursor, agy' — the binary's real order is
        'pi, opencode, cursor, agy, gemini, codex, claude'. Also confirm agy is marked experimental +
        not stager-capable, and pi's default_model is '' (decoupled)."
  pattern: "FIX the auto-detect order sentence (appears in '## The 7 built-in providers' intro).
        VERIFY the per-builtin table + per-role model table match the GROUND-TRUTH below (they do).
        VERIFY agy experimental + stager=no. Minimal edit."
  gotcha: "The table says '6 built-in providers' nowhere here (that's docs/README.md) — but double-check
        the '7 built-in providers' header is correct (it is). stager-capable = ONLY pi + claude today."

- file: docs/README.md   # EDIT — MINOR index refresh
  why: "The docs index. The table describes cli.md as 'all 11 global flags' (now ~18+ with per-role) and
        providers.md as '6 built-in providers' (now 7); the v1.0 version note is stale."
  pattern: "Update the index descriptions: cli.md → 'all global flags … subcommands … per-role + decompose
        flags'; providers.md → '7 built-in providers … agy'. Update the 'new in v1.0' note to v2.0.
        KEEP the 'binary is authoritative' contract line — that is a correctness guarantee the docs must
        now actually satisfy."
  gotcha: "Do NOT remove the 'If anything here disagrees with stagehand --help, the binary is
        authoritative' note — that is the contract this whole task exists to honor."

# RESEARCH NOTES (written by this PRP's research phase — read for full ground truth)
- file: plan/002_a17bb6c8dc1d/P4M3T1S1/research/cli_surface.md
  why: "The EXACT flag declarations (verbatim description strings), the exact default-action routing
        predicate (shouldDecompose), the exact env-var list, the exact config subcommand tree. The
        authoritative source for docs/cli.md + docs/configuration.md accuracy."
- file: plan/002_a17bb6c8dc1d/P4M3T1S1/research/providers_and_config.md
  why: "The exact preferredBuiltins order, the exact per-builtin table (detect/command/default_model/
        tooled_flags/experimental), the exact GenerateBootstrapConfig output, the exact config upgrade
        output strings, the four roles + flow + public Decompose() signature. The authoritative source
        for docs/providers.md + docs/how-it-works.md + README accuracy."

# ARCHITECTURE NOTES (plan-level design docs — read for the decompose pipeline detail)
- file: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  why: "The decompose pipeline design at plan level — use for the how-it-works.md prose if §13.6/§11.4
        need elaboration. (The PRD §11.4 diagram is the primary source; this is supplemental.)"
```

### GROUND-TRUTH TABLES (the docs MUST match these exactly — sourced from the binary, 2026-07-01)

**Provider auto-detect priority (7 providers)** — `internal/provider/registry.go:preferredBuiltins`:
```
pi, opencode, cursor, agy, gemini, codex, claude
```
(first installed = default; user-defined providers NEVER auto-selected.)

**Per-builtin provider facts** (`internal/provider/builtin.go`):

| Name | detect/command | default_model | stager-capable (tooled_flags non-empty)? | experimental |
|---|---|---|---|---|
| `pi` | `pi` | `""` (decoupled from z.ai) | **YES** | false |
| `opencode` | `opencode` | `""` (user must set) | no | false |
| `cursor` | `agent` (≠ name!) | `""` | no | false |
| `agy` | `agy` | `gemini-2.5-pro` | no | **true** |
| `gemini` | `gemini` | `gemini-2.5-pro` | no | false |
| `codex` | `codex` | `""` (user must set) | no | false |
| `claude` | `claude` | `sonnet` | **YES** | false |

(stager-capable today = **only pi + claude**. cursor is the ONLY provider where detect≠name — the binary is `agent`.)

**Decompose + per-role flags** (`internal/cmd/root.go`) — EXACT:

| Flag | Type | Default | Env var | Git config | Notes |
|---|---|---|---|---|---|
| `--commits` | int | `0` | `STAGEHAND_COMMITS` | *(none)* | 0=auto-decompose; ≥2=force exactly N; 1≡`--single` |
| `--single` | bool | `false` | *(none)* | *(none)* | bypass decompose → v1 single-commit |
| `--no-decompose` | bool | `false` | *(none)* | *(none)* | alias for `--single` |
| `--max-commits` | int | `12` | *(none)* | *(none)* | safety cap; also `[generation].max_commits` in config |
| `--planner-provider` | string | `""` | `STAGEHAND_PLANNER_PROVIDER` | *(none)* | per-role |
| `--planner-model` | string | `""` | `STAGEHAND_PLANNER_MODEL` | *(none)* | per-role |
| `--stager-provider` | string | `""` | `STAGEHAND_STAGER_PROVIDER` | *(none)* | per-role |
| `--stager-model` | string | `""` | `STAGEHAND_STAGER_MODEL` | *(none)* | per-role |
| `--arbiter-provider` | string | `""` | `STAGEHAND_ARBITER_PROVIDER` | *(none)* | per-role |
| `--arbiter-model` | string | `""` | `STAGEHAND_ARBITER_MODEL` | *(none)* | per-role |

**There is NO `--message-provider`/`--message-model` FLAG.** The message role has env vars
(`STAGEHAND_MESSAGE_PROVIDER`/`STAGEHAND_MESSAGE_MODEL`) + config (`[role.message]`) but NO CLI flag —
it inherits the global `--provider`/`--model`. Document the env+config for message, NOT a flag.

**Default-action routing** (`internal/cmd/default_action.go:shouldDecompose`) — decompose runs IFF:
nothing is staged AND `AutoStageAll` is on AND the user did NOT opt out
(`--single`/`--no-decompose`/`--commits 1` ⇒ `cfg.Single`) AND `--dry-run` is not set AND
`--no-auto-stage` is not set. If something IS staged → always the single-commit path. `--dry-run`
forces the single-commit preview (decompose commits, so dry-run honors the single preview).

**Per-provider per-role default models** (`internal/config/role_defaults.go`) — the compiled-in table:

| Provider | planner (smart) | stager (mid) | message (fast) | arbiter (mid) |
|---|---|---|---|---|
| `pi` | `gpt-5.4` | `gpt-5.4-mini` | `gpt-5.4-nano` | `gpt-5.4-mini` |
| `claude` | `opus` | `sonnet` | `haiku` | `sonnet` |
| `gemini` | `gemini-3.5-pro` | *(cannot)* | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| `agy` | `gemini-3.5-pro` | *(cannot)* | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| `opencode` | `openai/gpt-5.4` | *(cannot)* | `openai/gpt-5.4-nano` | `openai/gpt-5.4-mini` |
| `codex` | `gpt-5.1-codex-max` | *(cannot)* | `gpt-5.4-nano` | `gpt-5.1-codex-mini` |
| `cursor` | `gpt-5.4` ⚠️ | *(cannot)* | `gpt-5.4-nano` ⚠️ | `gpt-5.4-mini` ⚠️ |

*(cannot) = no tooled_flags ⇒ cannot serve as stager; bootstrap falls back to the first stager-capable
provider (pi or claude). ⚠️ cursor = best-guess OpenAI tokens (FR-D5: verify against `agent --help`).*

**config init** (`internal/cmd/config.go` + `internal/config/bootstrap.go`): POPULATED. Writes
`config_version = 2` (uncommented) + `[defaults] provider = "<auto-detected>"` + four `[role.*]` blocks
(planner/stager/message/arbiter) for the detected provider UNCOMMENTED; other installed providers as
commented-out `[role.*]` blocks. Flags: `--provider <name>` (target specific), `--force` (overwrite),
`--template` (v1 inert all-commented reference). Defaults to `"pi"` if nothing detected. Refuses
overwrite unless `--force` (exit 1).

**config upgrade** (`internal/cmd/config.go`): NO flags. Rewrites the GLOBAL config's top-level
`config_version` line to the current version (2) in place; preserves every other line byte-for-byte.
Output: no file → `no config file at <path> (run 'stagehand config init' first)` exit 1; invalid TOML →
exit 1 untouched; already current → `Config at <path> is already at version 2 (no changes).` exit 0;
upgraded → `Upgraded config at <path> to version 2.` exit 0. Idempotent. The load-time advisory
(when config_version is missing/older/newer) points users at `config upgrade` or `config init --force`.

### Current Codebase tree (relevant subset)

```bash
README.md                 # EDIT — marketing surface (§21.5), materially stale
docs/
  README.md               # EDIT — docs index (minor refresh)
  how-it-works.md         # EDIT — add decompose pipeline + binary filtering
  cli.md                  # EDIT — add v2 flags + config upgrade + fix order
  configuration.md        # EDIT — add config upgrade + per-role env/flag + decompose keys
  providers.md            # EDIT — fix auto-detect order (one fact)
providers/*.toml          # NOT EDITED — manifest reference files (accurate, not docs)
PRD.md                    # READ-ONLY — authoritative source
plan/002.../architecture/ # READ-ONLY — design context
```

### Desired Codebase tree (this task EDITS 6 Markdown files; NO new files, NO source changes)

```bash
README.md                 # +hero multi-commit +decompose section +fixed order +v2 examples +FAQ +config
docs/README.md            # +accurate index (7 providers, v2.0 note)
docs/how-it-works.md      # +"Multi-commit decomposition" section +binary filtering note
docs/cli.md               # +v2 flags table +map rows +config upgrade +populated init +v2 examples +fixed order
docs/configuration.md     # +config upgrade section +per-role env/flag +decompose keys
docs/providers.md         # ~fixed auto-detect order (one sentence)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- G-BINARY-IS-AUTHORITATIVE (the master constraint): docs/README.md declares "If anything here
     disagrees with stagehand --help, the binary is authoritative." EVERY flag name, env var, provider
     name, config key, and the provider ORDER in the docs must match the shipped binary. Before
     finalizing, run: `stagehand --help`, `stagehand config init --help`, `stagehand config upgrade
     --help`, `stagehand providers list` — and diff the docs against them. The GROUND-TRUTH tables above
     are pre-extracted from the binary; trust them over the binary's own flag-description strings (next). -->

<!-- G-FLAG-DESC-ACCURACY (IMPORTANT — a real accuracy trap): the binary's --commits flag description
     string says "(env/git stagehand.commits)" and --max-commits says "(env/git stagehand.max_commits)".
     These are ASPIRATIONAL/INACCURATE vs the implementation: STAGEHAND_COMMITS exists, but there is NO
     STAGEHAND_MAX_COMMITS env var and NO git `stagehand.commits` / `stagehand.max_commits` key. The docs
     must describe the ACTUAL behavior (see the GROUND-TRUTH table), NOT copy the flag's description
     string. If you `stagehand --help` and copy the --commits line verbatim, you'll reintroduce a lie.
     Document: --commits = flag + STAGEHAND_COMMITS env (no git key); --max-commits = flag +
     [generation].max_commits config (NO env, NO git key). -->

<!-- G-NO-MESSAGE-FLAG (IMPORTANT): there is NO --message-provider / --message-model CLI flag. Only
     planner/stager/arbiter have per-role flags. The message role is reachable via env
     (STAGEHAND_MESSAGE_PROVIDER/MODEL) + config ([role.message]) but NOT a flag — it inherits the global
     --provider/--model. Do NOT invent a --message-* flag in the cli.md table. Document the env+config
     for message, and note "no flag; inherits the global default". -->

<!-- G-STALE-ORDER-VARIANTS (the order appears in MANY places and is WRONG in several): grep for all of
     these stale forms and replace each with `pi, opencode, cursor, agy, gemini, codex, claude`:
       - "pi, claude, gemini, opencode, codex, cursor" (README.md "Configure your agent"; docs/cli.md
         "providers list" note)
       - "pi, claude, gemini, opencode, codex, cursor, agy" (docs/providers.md intro — agy wrongly last)
       - any "(preference order: pi, claude, …)" parenthetical
     README.md also says the auto-detect order is "pi, claude, gemini, opencode, codex, cursor" in the
     "Configure your agent" prose — fix it. Note: this is 7 providers now (agy added), not 6. -->

<!-- G-FAQ-LIE (the single most damaging stale statement): README.md FAQ "Can it write multiple
     commits?" answers "Not in v1 — v1 creates a single commit per invocation. Multi-commit hunk
     decomposition is planned for v2." This is now FALSE. Rewrite to YES: bare `stagehand` with a dirty
     tree + nothing staged auto-decomposes; `--commits N` forces the count; `--single` keeps v1
     behavior. This is the headline v2 feature. -->

<!-- G-CONFIG-INIT-NOW-POPULATED: README.md + docs/cli.md describe `config init` as writing a "fully-
     commented example config" (the v1 INERT template). It now writes a POPULATED working config
     (config_version=2 + [defaults] + [role.*] for the detected provider). The inert template survives
     behind `config init --template`. Update both files. Also add `config upgrade` (new subcommand). -->

<!-- G-PI-DEFAULT-DECOUPLED (FR-D2): do NOT describe pi as defaulting to glm/zai. pi's default_model is
     now "" (decoupled from the author's personal z.ai subscription). The README's `git config
     stagehand.model glm-5.2` example is fine as a *per-repo override example* but must not be framed as
     "the default". If you keep it, add a one-line note that it's an override. -->

<!-- G-MARKDOWNLINT-RULES: .markdownlint.json disables MD013 (line length — long lines/tables OK),
     MD033 (inline HTML — <details>/<summary> OK), MD060 (heading punctuation OK). Default rules still
     ON include: MD001 (heading increment by one), MD009 (trailing whitespace), MD012 (multiple blank
     lines), MD031 (blanks around fenced code blocks), MD040 (fenced code blocks need a language tag —
     ALWAYS tag fences: ```text, ```bash, ```toml, never bare ```), MD041 (first line = top-level
     heading #), MD036 (no bold-as-heading in some cases). The §11.4 pipeline diagram MUST be in a
     ```text fence. Watch MD040 especially when adding code blocks. -->

<!-- G-ADMIRALTY-LINKS: README.md links to docs/ via `docs/` and `docs/*.md`; docs/*.md link back via
     `../README.md` and sideways via `cli.md`/`configuration.md`/`providers.md`/`how-it-works.md`. Keep
     these relative-link conventions. The new how-it-works.md decompose section should cross-link to
     configuration.md (per-role models) and cli.md (flags). Verify no broken links after editing. -->

<!-- G-DONT-EDIT-PROVIDERS-TOML: providers/*.toml are MANIFEST REFERENCE FILES, not docs. They are
     accurate (P1.M2). Do not edit them. The docs REFERENCE them ("see providers/pi.toml"). -->

<!-- G-SCOPE-NO-SOURCE: this task edits ONLY the 6 Markdown files. Do NOT touch any .go file, the
     Makefile, .markdownlint.json, go.mod, or the PRD. The parallel task P4.M2.T2.S1 touches only
     internal/cmd/*_test.go (Go tests) — DISJOINT from these docs; no conflict. -->
```

## Implementation Blueprint

### Data models and structure

No data models. This is a documentation task. The "data" is the GROUND-TRUTH tables above (provider
order, per-builtin facts, flags, default-model table, config init/upgrade behavior) which must appear
accurately in the prose/tables of the edited Markdown.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: RE-VERIFY ground truth against the binary (no edits — do first, ~2 min)
  - BUILD: go build -o /tmp/sh-doc ./cmd/stagehand
  - RUN: /tmp/sh-doc --help                          → capture the exact flag list/descriptions
  - RUN: /tmp/sh-doc config init --help              → confirm --provider/--force/--template
  - RUN: /tmp/sh-doc config upgrade --help           → confirm it exists, exits 0
  - RUN: /tmp/sh-doc providers list                  → confirm order + (default) marker
  - DIFF the captured --help against the GROUND-TRUTH tables above. If the binary disagrees with the
    tables, the BINARY WINS — update the doc text to match the binary (not the table), and note the
    discrepancy. (Per 2026-07-01 research the tables are accurate; re-confirm flag descriptions especially.)

Task 1: EDIT docs/providers.md — fix the auto-detect order (smallest, isolated edit first)
  - FIND: the sentence in "## The 7 built-in providers" that lists the auto-detection order.
    Current (WRONG): "pi, claude, gemini, opencode, codex, cursor, agy"
    Replace with: "pi, opencode, cursor, agy, gemini, codex, claude"
  - VERIFY: the per-builtin table (agy present + experimental + not stager-capable; pi default_model
    ""/decoupled; cursor detect=agent) and the per-role model table match the GROUND-TRUTH. Fix any drift.
  - WHY FIRST: it's a one-sentence fix; doing it first clears the most-cited wrong fact.

Task 2: EDIT docs/README.md — refresh the index (small, isolated)
  - UPDATE the Documentation index table:
      cli.md description: "all 11 global flags" → "all global flags (incl. decompose + per-role)".
      providers.md description: "6 built-in providers" → "7 built-in providers … agy".
  - UPDATE the "new in v1.0" NOTE to v2.0 (multi-commit, per-role models, binary filtering, agy).
  - KEEP the "binary is authoritative" line.

Task 3: EDIT docs/cli.md — the largest single-file edit (flags table + routing + subcommands + examples)
  - 3a. UPDATE the Synopsis + add a "Default action" note: nothing staged + dirty tree + not opted out →
       DECOMPOSE (multi-commit); --single/--no-decompose/--commits 1 → v1 single commit; --dry-run forces
       the single-commit preview. (Ground-truth: shouldDecompose predicate.)
  - 3b. ADD these rows to the Global flags table (verbatim names from GROUND-TRUTH): --commits, --single,
       --no-decompose, --max-commits, --planner-provider, --planner-model, --stager-provider,
       --stager-model, --arbiter-provider, --arbiter-model. Use the GROUND-TRUTH defaults + descriptions.
       DO NOT add --message-provider/--message-model (G-NO-MESSAGE-FLAG).
  - 3c. UPDATE the "config init" subcommand section: it now writes a POPULATED config (config_version=2 +
       [defaults] + [role.*]); flags --provider/--force/--template; refuses overwrite unless --force.
  - 3d. ADD a "### config upgrade" subcommand section: no flags; rewrites config_version in place;
       "already at version 2 (no changes)" / "Upgraded … to version 2" / errors if no file; idempotent;
       the load-time advisory points here.
  - 3e. FIX the "providers list" auto-detect order note → pi, opencode, cursor, agy, gemini, codex, claude.
  - 3f. ADD rows to the flag↔env↔git-config map: the per-role flags (env STAGEHAND_<ROLE>_*; no git key),
       --commits (env STAGEHAND_COMMITS; no git key), --single/--no-decompose/--max-commits (no env, no
       git — note max-commits config field). Document message role as env+config only (no flag).
  - 3g. ADD v2 examples (from PRD §15.5): --commits 3; --single; --planner-model/--planner-provider.

Task 4: EDIT docs/configuration.md — ADD missing pieces (per-role env/flag + config upgrade + decompose keys)
  - VERIFY the existing populated-bootstrap + config_version + per-role content is accurate (it is).
  - 4a. ADD a "### Schema versioning (`config upgrade`)" subsection under Bootstrap: config_version
       advisory (missing/older/newer → warning pointing at upgrade or init --force) + the upgrade command
       behavior (GROUND-TRUTH strings). config_version = 2 is current.
  - 4b. ADD the per-role env vars (STAGEHAND_PLANNER_*/STAGER_*/MESSAGE_*/ARBITER_*) + per-role flags
       (--planner-provider/--planner-model/etc.) to the env/flag reference. Note message has no flag.
  - 4c. ADD the decompose config keys note: commits/single are flag/env (no config-file block of their
       own; single is reached via --single/--no-decompose/STAGEHAND_COMMITS=1); max_commits lives under
       [generation] (default 12). NO git-config keys for any of these.

Task 5: EDIT docs/how-it-works.md — ADD the decompose pipeline + binary filtering (the architectural centerpiece)
  - 5a. ADD a top-level "## Multi-commit decomposition" section AFTER "## Stage-while-generating".
       Content: the trigger (nothing staged + dirty tree; --single opt-out; §13.6.1); the four roles
       (planner/stager/message/arbiter — §13.6.2); the §11.4 ASCII pipeline diagram in a ```text fence;
       the overlapped loop (stager[i+1] ∥ message[i]; tree[i] frozen before stager[i+1] starts; concept
       diff is tree[i-1]→tree[i], never index-vs-HEAD); serialized publication (CAS update-ref); the
       arbiter leftover reconciliation (§13.6.5). Emphasize the SAME safety invariants (snapshot-based,
       CAS, stagehand owns all ref mutations; stager is the one index-mutating exception, scoped to add).
       Cross-link to configuration.md (per-role models) + cli.md (flags).
  - 5b. ADD a short "### Binary and non-text file filtering" note (FR3a-c): binary/lock/snapshot/
       sourcemap/vendor files are excluded from every diff payload (staged, working-tree snapshot,
       concept diff) and replaced with a "<status>\t[binary] <path>" placeholder so the agent sees what
       changed without the useless binary hunk.

Task 6: EDIT README.md — the marketing surface (largest narrative edit)
  - 6a. HERO pitch: add multi-commit decomposition (PRD §5 — the hero candidate mentions it). Keep the
       existing "stage while it thinks / never corrupt / atomic" framing; add "and can split a dirty
       tree into a sequence of clean commits" (paraphrase §1 item 3 / §5 #6).
  - 6b. QUICK START: after the single-commit steps, INSERT a "### Multi-commit decomposition"
       subsection: dirty tree + nothing staged → bare `stagehand` auto-decomposes; `--commits 3` forces;
       `--single` keeps v1. One short example block (PRD §15.5).
  - 6c. "Configure your agent": FIX the preference order (→ pi, opencode, cursor, agy, gemini, codex,
       claude); UPDATE the providers list example to show agy; UPDATE config init to POPULATED + mention
       config upgrade; add the decoupled-from-z.ai note (pi default changed). Keep the glm-5.2 example
       as a per-repo override but note it's an override (G-PI-DEFAULT-DECOUPLED).
  - 6d. COMPETITIVE table (the opencommit/aicommits comparison): add a "Multi-commit decomposition" row
       = No / Yes (PRD §5 + delta_prd).
  - 6e. FEATURE positioning: ensure "right model for the right job" (§5 #6) is reflected (link to docs).
  - 6f. FAQ "Can it write multiple commits?": REWRITE to YES (the G-FAQ-LIE fix) — auto-decompose,
       --commits N, --single.
  - 6g. ADD a short v2 examples block (or fold into quick start) from §15.5.

Task 7: VALIDATE (no edits)
  - 7a. markdownlint: `npx --yes markdownlint-cli2 README.md docs/` (uses .markdownlint.json). Fix any
       MD040 (fence language), MD009 (trailing ws), MD031 (blank around fences) issues.
  - 7b. Stale-string grep (must be ZERO hits):
         grep -rn "Not in v1\|planned for v2\|single commit per invocation\|pi, claude, gemini, opencode, codex, cursor\|6 built-in providers\|fully-commented example config" README.md docs/
  - 7c. Binary cross-check: re-run Task 0's commands; confirm every doc flag/provider/order matches.
  - 7d. Link check: confirm docs/ internal links resolve (README↔docs, docs↔docs, docs→providers/).
```

### Implementation Patterns & Key Details

```markdown
<!-- Pattern: render the §11.4 pipeline as a fenced code block (how-it-works.md). Use ```text and keep
     the ASCII art from PRD §11.4 verbatim (it is the canonical diagram). The two invariants
     (tree[i] frozen before stager[i+1]; concept diff is tree[i-1]→tree[i]) are the key correctness
     points — call them out in prose under the diagram. -->

<!-- Pattern: flag tables in cli.md. The existing Global flags table has columns
     Flag | Type | Default | Env var | Git config | Description. ADD the v2 rows with the SAME columns.
     For flags with no env/git analog, put "—" in those cells (matching the existing --all/--no-auto-stage
     style). For --commits put "STAGEHAND_COMMITS" in env and "—" in git. NEVER copy the binary's
     "(env/git stagehand.commits)" description string (G-FLAG-DESC-ACCURACY). -->

<!-- Pattern: the "binary is authoritative" contract. docs/README.md already states docs defer to
     stagehand --help. Honor it: if --help shows a flag, document it; if a doc claims a flag/env/key
     that --help (or the source) doesn't have, remove it. The GROUND-TRUTH tables are the verified
     intersection of "what the docs should say" and "what the binary actually does." -->

<!-- Pattern: per-role precedence one-liner (configuration.md + cli.md). Precedence (highest wins):
     flag > env (STAGEHAND_<ROLE>_*) > per-role config ([role.<role>]) > global config ([defaults]) >
     built-in manifest default. The message role omits the flag layer. Stager falls back to the next
     stager-capable provider if its resolved provider can't serve (tooled_flags empty). -->

<!-- Pattern: the FAQ rewrite (README.md). Replace the lie with a 3-line answer:
     "Yes. Run `stagehand` with a dirty working tree and nothing staged, and it decomposes the changes
     into a sequence of logically-coherent commits automatically. Force a count with `--commits 3`, or
     keep the one-commit behavior with `--single`. See [Multi-commit decomposition](#…)." -->
```

### Integration Points

```yaml
DOCS CROSS-LINKS (Markdown relative links — keep the existing conventions):
  - README.md → docs/ via "docs/" and "docs/cli.md" etc. (existing pattern).
  - docs/how-it-works.md → configuration.md (per-role models), cli.md (flags). NEW cross-links.
  - docs/cli.md → configuration.md (config detail), providers.md (manifests). Existing + extend.
  - docs/configuration.md → cli.md (flags), providers.md (manifests). Existing.
  - docs/providers.md → ../providers/ (toml files), configuration.md (per-role). Existing.
  - docs/README.md → ../README.md (quick start), each docs/*.md (index). Existing + refresh text.
BINARY CONTRACT (the docs must match):
  - stagehand --help          → the global flags table (incl. v2 flags).
  - stagehand config init --help → the config init flags (--provider/--force/--template).
  - stagehand config upgrade --help → exists (the new subcommand section).
  - stagehand providers list  → the provider order + (default) marker + agy.
PARALLEL TASK (P4.M2.T2.S1):
  - Touches ONLY internal/cmd/{config,providers}_test.go (Go tests). DISJOINT from these 6 Markdown
    files → NO merge conflict. It LOCKS IN config upgrade registration + help dedup + config init flags
    — which this task DOCUMENTS. The two tasks are complementary (one verifies, one documents).
```

## Validation Loop

### Level 1: Markdown Lint (Immediate Feedback)

```bash
# After editing each file (and at the end). The repo pins rules in .markdownlint.json (MD013/MD033/MD060 off).
npx --yes markdownlint-cli2 README.md 'docs/**/*.md'
# Per-file quick check during editing:
npx --yes markdownlint-cli2 docs/cli.md

# Common fixes the linter will flag:
#  - MD040: fenced code block needs a language tag → use ```text / ```bash / ```toml / ```yaml (never bare ```).
#  - MD031: blank line required before/after a fenced code block.
#  - MD009: trailing whitespace on a line.
#  - MD012: multiple consecutive blank lines → collapse to one.
#  - MD041: first line of the file must be a top-level heading (# …). README.md and each docs/*.md already start with "#".
# Expected: zero errors. If markdownlint-cli2 can't fetch offline, fall back to a careful manual check of
# the rules above (fence tags, trailing ws, blanks around fences) — these are the high-frequency ones.
```

### Level 2: Stale-Statement Elimination (Content Gate)

```bash
# These strings are the KNOWN stale v1 statements. Each MUST return ZERO hits after editing.
grep -rn "Not in v1" README.md docs/                      # the FAQ lie (README.md)
grep -rn "planned for v2" README.md docs/                 # the FAQ lie
grep -rn "single commit per invocation" README.md docs/   # the FAQ lie
grep -rn "pi, claude, gemini, opencode, codex, cursor" README.md docs/   # the stale 6-provider order
grep -rn "codex, cursor, agy" README.md docs/             # any agy-appended-last variant
grep -rn "6 built-in providers" README.md docs/           # docs/README.md index (now 7)
grep -rn "fully-commented example config" README.md docs/ # v1 inert config init description
grep -rn "all 11 global flags" README.md docs/            # docs/README.md index (flag count grew)
grep -rn "new in v1.0" docs/README.md                     # the version note (→ v2.0)

# Expected: ZERO matches for every grep above. If any matches, that stale statement survives — fix it.
```

### Level 3: Binary Cross-Check (the "binary is authoritative" gate)

```bash
# Build and capture the binary's actual surface; diff against the docs.
go build -o /tmp/sh-doc ./cmd/stagehand

# (1) Global flags — every flag in --help must appear in docs/cli.md's table (and vice versa).
/tmp/sh-doc --help > /tmp/help.txt
# Manually confirm: --commits, --single, --no-decompose, --max-commits, --planner-provider/--planner-model,
#   --stager-provider/--stager-model, --arbiter-provider/--arbiter-model are in docs/cli.md.
#   Confirm NO --message-provider/--message-model is claimed in the docs (it doesn't exist).

# (2) config init flags.
/tmp/sh-doc config init --help | grep -E -- "--provider|--force|--template"
#   → all three present; docs/cli.md + docs/configuration.md describe them.

# (3) config upgrade exists.
/tmp/sh-doc config upgrade --help >/dev/null 2>&1 && echo "OK: config upgrade exists"
#   → docs/cli.md + docs/configuration.md document it.

# (4) Provider order + agy.
/tmp/sh-doc providers list
#   → confirm the order is pi, opencode, cursor, agy, gemini, codex, claude and agy appears;
#     confirm README.md + docs/cli.md + docs/providers.md quote the SAME order.

# Expected: every documented flag/provider/order matches the binary. The docs/README.md contract
# "binary is authoritative" is now actually true.
```

### Level 4: Discoverability & Link Integrity (Domain Validation)

```bash
# Discoverability: a new user can find + use multi-commit from the docs alone.
grep -n -i "multi-commit\|decompose\|--commits" README.md docs/how-it-works.md docs/cli.md docs/configuration.md
# Expected: hits in all four (README headline + how-it-works pipeline + cli flags + config keys).

# Binary filtering discoverable.
grep -n -i "binary\|non-text" docs/how-it-works.md
# Expected: at least the FR3a-c note added in Task 5b.

# Per-role models discoverable.
grep -n -i "per-role\|role.planner\|--planner-model" README.md docs/cli.md docs/configuration.md docs/providers.md
# Expected: hits in all four.

# Link integrity: every docs/ internal link resolves to an existing file.
for f in docs/*.md README.md; do
  grep -oE '\]\([^)]+\.md[^)]*\)' "$f" | sed 's/^](//; s/)$//' | while read -r link; do
    # resolve relative to the file's dir
    base=$(dirname "$f")
    target="$base/$link"
    target=$(realpath -m --relative-to=. "$target" 2>/dev/null)
    [ -f "$target" ] || echo "BROKEN LINK in $f: $link"
  done
done
# Expected: no BROKEN LINK lines.

# agy discoverable + flagged experimental.
grep -n "agy" README.md docs/providers.md docs/cli.md
grep -n -i "experimental" docs/providers.md
# Expected: agy in all three; experimental note in providers.md.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 green: `npx --yes markdownlint-cli2 README.md 'docs/**/*.md'` reports zero errors (or, if
      offline, manual check of fence tags / trailing ws / blanks-around-fences passes).
- [ ] Level 2 green: every stale-statement grep returns ZERO hits.
- [ ] Level 3 green: every documented flag/provider/order matches `stagehand --help` / `config init
      --help` / `config upgrade --help` / `providers list`.
- [ ] Level 4 green: discoverability greps hit; link check reports no BROKEN LINKs; agy + experimental
      present.
- [ ] `git diff --name-only` shows ONLY the 6 Markdown files (README.md + docs/*.md). NO .go, NO PRD.md,
      NO tasks.json, NO providers/*.toml, NO Makefile/.markdownlint.json/go.mod.

### Feature Validation

- [ ] README.md hero mentions multi-commit; a "Multi-commit decomposition" subsection exists; the FAQ
      answers YES; the competitive table has a multi-commit = Yes row; the provider order is correct;
      config init is populated + config upgrade mentioned.
- [ ] docs/how-it-works.md has the "Multi-commit decomposition" section (four roles + §11.4 diagram +
      overlapped loop + arbiter + safety) + the binary-filtering note.
- [ ] docs/cli.md global-flags table + flag↔env↔git-config map include all v2 flags (no invented
      --message-* flag); default-action describes decompose routing; config upgrade section exists;
      config init is populated; auto-detect order is correct; v2 examples present.
- [ ] docs/configuration.md has the config upgrade section + per-role env/flag reference + decompose keys.
- [ ] docs/providers.md auto-detect order is `pi, opencode, cursor, agy, gemini, codex, claude`; agy is
      experimental + not stager-capable; pi default_model decoupled.
- [ ] docs/README.md index descriptions are accurate (7 providers; v2.0 note; correct flag-count wording).

### Code Quality Validation

- [ ] All edits follow the existing doc tone (concise, developer-facing) and Markdown conventions.
- [ ] Fenced code blocks are tagged (```text / ```bash / ```toml / ```yaml).
- [ ] Internal doc links use the existing relative-link conventions and all resolve.
- [ ] No invented behavior: every claim is traceable to the binary or the PRD.
- [ ] The "binary is authoritative" contract in docs/README.md is honored (docs match the binary).

### Documentation & Deployment

- [ ] A new user can discover and use multi-commit decomposition, per-role models, binary filtering, agy,
      the cascading default, and config init/upgrade — from the docs ALONE.
- [ ] No stale v1 statements remain anywhere in README.md or docs/.
- [ ] Cross-references between the six files are consistent (e.g. the provider order is identical in
      README.md, docs/cli.md, docs/providers.md).

---

## Anti-Patterns to Avoid

- ❌ Don't copy the binary's flag DESCRIPTION STRINGS verbatim for `--commits`/`--max-commits` — they cite
  env/git keys that don't exist (G-FLAG-DESC-ACCURACY). Document the ACTUAL behavior from the GROUND-TRUTH table.
- ❌ Don't invent a `--message-provider`/`--message-model` flag — it doesn't exist (G-NO-MESSAGE-FLAG).
  The message role is env+config only.
- ❌ Don't leave the README FAQ lie ("Not in v1 … planned for v2") — it's the most damaging stale statement.
- ❌ Don't quote the stale 6-provider order anywhere — the order is now 7 providers: `pi, opencode, cursor,
  agy, gemini, codex, claude`.
- ❌ Don't describe `config init` as "fully-commented/inert" — it's populated now (inert is `--template`).
- ❌ Don't describe pi as defaulting to glm/zai — it's decoupled (default_model = "").
- ❌ Don't rewrite docs/configuration.md or docs/providers.md from scratch — they're already mostly v2-
  correct; only ADD the missing pieces (config upgrade, per-role env/flag, decompose keys) and FIX the one
  stale order.
- ❌ Don't edit any non-doc file (.go, PRD.md, tasks.json, providers/*.toml, Makefile, .markdownlint.json).
- ❌ Don't add untagged fenced code blocks (MD040) — always tag them (```text / ```bash / etc.).
- ❌ Don't break internal doc links — verify they resolve after editing.
- ❌ Don't touch the parallel task P4.M2.T2.S1's files (internal/cmd/*_test.go) — disjoint scope.
