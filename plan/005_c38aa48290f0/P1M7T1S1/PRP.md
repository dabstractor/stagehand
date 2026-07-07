name: "P1.M7.T1.S1 — README + docs index coherence sweep for the six v2.1 capabilities"
description: |

  Mode-B (changeset-level docs) subtask. The six v2.1 features (M1–M6: payload exclusions, format/
  locale/context/template shaping, git hook mode, tool integrations git-alias+lazygit, --edit/--push,
  models + interactive init) have ALL landed in code AND already carry Mode-A docs in `docs/`. This
  task makes them DISCOVERABLE: rewrite `README.md` (hero/features/quick-start/FAQ) so no v2.1
  capability is missing from the top-level documentation surface, replace the manual lazygit snippet
  with `stagecoach integrate install lazygit` (keeping the YAML as a collapsible alternative), land the
  FR-H7 hook-vs-snapshot trade-off as a README FAQ entry, reference `FUTURE_SPEC.md` wherever
  deferred/rejected ideas appear, and refresh `docs/README.md`'s index for the new/changed sections.

---

## Goal

**Feature Goal**: A single changeset-coherent pair of edits — `README.md` + `docs/README.md` — such that
a first-time visitor reading ONLY those two files learns that all six v2.1 capabilities exist, where
each one lives in `docs/`, what the hook-vs-snapshot trade-off is, and where deferred/rejected ideas
are tracked. Every v2.1 capability is surfaced; every README link/anchor resolves; the README wording
matches the SHIPPED binary (judged against behavior, not this plan).

**Deliverable**: Two edited Markdown files — `README.md` and `docs/README.md` — passing markdown lint
(`.markdownlint.json`) and a link/anchor-integrity check, with a capability-coverage audit showing all
six capabilities present. NO other files are touched (no code, no other docs, no `FUTURE_SPEC.md`).

**Success Definition**:
- All six capabilities appear by name in `README.md` (grep-verified), each linked to its existing
  `docs/` Mode-A page (anchors resolve).
- The manual lazygit snippet is REPLACED by `stagecoach integrate install lazygit` as the primary path;
  the manual YAML survives as a `<details>` collapsible "manual install" alternative matching the
  shipped `customCommands` shape (incl. `context`, `loadingText`, `output`, `description`, the
  `# stagecoach-integration` marker).
- The README FAQ has an FR-H7 trade-off entry ("Does it run my pre-commit hooks?") discoverable from
  the FAQ section, linking `docs/how-it-works.md#trade-off-inversion-fr-h7`.
- `FUTURE_SPEC.md` is linked from README wherever a deferred/rejected competitor idea is mentioned
  (gitui, PR generation, editor extensions, GitHub Action, API keys, etc.) and from `docs/README.md`.
- `docs/README.md` index table descriptions reflect the v2.1 sections each page now carries; the repo's
  navigation surface lists no stale/missing capability.

## User Persona

**Target User**: The "plan-holder" (PRD §7.1) landing on the GitHub repo for the first time after v2.1
ships, deciding whether to install, and the "multi-agent tinkerer" (§7.3) scanning for a specific v2.1
capability (hook mode, lazygit keybind, exclusions).

**Use Case**: A husky/lint-staged user wants to know if stagecoach will honor their pre-commit hooks.
They read the README FAQ, find the FR-H7 trade-off entry, follow the link to `docs/how-it-works.md`,
and learn hook mode is the answer.

**Pain Points Addressed**: Today's README documents only the v1 + v2.0 (decompose) surface — a visitor
who installed stagecoach for `--exclude` or hook mode cannot discover either from the README, and the
manual lazygit snippet invites copy-paste of a YAML shape that drifted from the shipped one.

## Why

- **Item contract (LOGIC/OUTPUT)**: surface all six capabilities in hero/features/quick-start; replace
  the manual lazygit snippet with the install command (keep YAML as alternative); ensure the FR-H7
  FAQ entry is discoverable; reference FUTURE_SPEC.md for deferred/rejected ideas; update the index.
- **PRD §21.5** defines the README as the marketing surface (hero, demo, why-not, install, quick-start,
  configure, snapshot workflow, full reference, adding an agent, FAQ) — the v2.1 capabilities fold into
  the existing structure (features/quick-start + FAQ), they do not require a restructure.
- **PRD §10.4** lists the six accepted v2.1 features; `FUTURE_SPEC.md` carries everything deferred/
  rejected. The README must point to FUTURE_SPEC.md so visitors don't re-litigate "why no PR generation".
- **architecture/system_context.md §6**: README "currently carries a MANUAL lazygit-snippet section and
  a FAQ"; `docs/README.md` is the docs index; `FUTURE_SPEC.md` exists at the repo root (reference it,
  NEVER author or modify it); all per-feature (Mode-A) docs are already in place from M1–M6.

## What

User-visible behavior: a refreshed README whose hero still leads with the core value prop (uses your
agent, snapshot, decompose), followed by a concise **Features** section that lists all six v2.1
capabilities with one-line descriptions and links into `docs/`; a **Quick start** that adds one or two
v2.1 convenience examples; a **lazygit / git alias** subsection whose primary path is
`stagecoach integrate install lazygit` (with the manual YAML kept behind a collapsible); an expanded
**FAQ** with the FR-H7 trade-off entry and a "what about X?" entry pointing at FUTURE_SPEC.md; and a
`docs/README.md` index whose row descriptions + a capability-pointer note make every v2.1 section
reachable in one hop.

### Success Criteria

- [ ] `README.md` contains a "Features" (or equivalent) section listing all six capabilities by name.
- [ ] Each capability entry links to its existing `docs/` page; every link + anchor resolves.
- [ ] The manual lazygit YAML snippet is replaced by `stagecoach integrate install lazygit` (primary);
      the manual YAML survives as a `<details>` alternative matching the shipped shape.
- [ ] `git stagecoach` alias (`stagecoach integrate install git-alias`) is mentioned alongside lazygit.
- [ ] The FAQ has a pre-commit-hooks / FR-H7 trade-off entry linking
      `docs/how-it-works.md#trade-off-inversion-fr-h7`.
- [ ] `FUTURE_SPEC.md` is linked from README (deferred/rejected ideas) AND from `docs/README.md`.
- [ ] `docs/README.md` index table descriptions reflect v2.1 sections; no capability is missing from
      the navigation surface.
- [ ] `README.md` and `docs/README.md` pass `npx --yes markdownlint-cli2` against `.markdownlint.json`.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT two files to edit (`README.md`, `docs/README.md`), the EXACT existing README
sections to touch (hero, "Quick start", "lazygit binding", "Configure your agent", "FAQ), the EXACT
docs pages + verified anchors each capability links to, the EXACT shipped `customCommands` YAML shape,
the EXACT FR-H7 trade-off wording to mirror, the EXACT scope fence vs P1.M7.T1.S2 (stale-string sweep)
and P1.M6.T2.S1 (wizard docs), and the EXACT lint config + validation commands. An implementer with no
prior codebase knowledge can do this from the document + file access._

### Documentation & References

```yaml
- file: README.md
  why: THE file being rewritten. Current structure (section order): hero pitch → "30-second demo" →
       "Why not opencommit/aicommits?" (+ comparison table + a <details> "Which coding plans gate…") →
       "Install" (Build-from-source primary; "Coming soon" Homebrew/Scoop/go-install/curl) → "Quick start"
       (+ "Multi-commit decomposition" subsection + "--reasoning" note) → "lazygit binding" (MANUAL YAML
       snippet — REPLACE) → "Configure your agent" (providers list, git config, config init/path/upgrade,
       precedence) → "The snapshot workflow" (ascii diagram) → "Full CLI and config reference" →
       "Adding a new agent" → "FAQ" ("not for you if…", "Will it corrupt my repo?", "send my code
       anywhere?", "write multiple commits?", "match my style?", "Which agents?", "see what command?")
       → "Contributing".
  pattern: hero uses "> **bold pitch**" prose; callouts use GitHub alert syntax ("> [!NOTE]", "> [!TIP]");
           collapsibles use "<details><summary>…</summary>…</details>" (MD033 inline-HTML is DISABLED in
           .markdownlint.json, so this is allowed and already in use). Code fences are ```bash / ```yaml /
           ```toml / ```text. Tables use GitHub pipe syntax.
  gotcha: README's comparison table currently has 6 rows (Auth, Architecture, Billing, Stage-while-
          generating, Multi-commit decomposition, Per-role model routing). A "Payload exclusions / Hook
          mode / Tool integrations" row could be added but is OPTIONAL — the Features section is the
          primary surface; don't bloat the comparison table (it's the "why not opencommit" argument).

- file: docs/README.md
  why: THE index being refreshed. Structure: hero line → "See the README" pointer → binary-authoritative
       note → "Install" (the 4 planned paths, incl. an install.sh note) → "Documentation index" (a 4-row
       table: CLI reference, Configuration, Provider manifests, How Stagecoach works) → "Product
       specification" (links PRD.md) → "Contributing".
  pattern: 4-row Markdown table `| Page | Description |`. The Product-spec section links `../PRD.md`.
  gotcha: docs/README.md does NOT yet link FUTURE_SPEC.md — add it under "Product specification"
          (`../FUTURE_SPEC.md`). Keep the "binary is authoritative" note; do not duplicate the README's
          install block verbatim (it's already a pointer).

- docfile: plan/005_c38aa48290f0/P1M7T1S1/research/capability_doc_map.md
  why: THE authoritative capability→doc map + verified anchor slugs + shipped-behavior snapshots
       (lazygit customCommands YAML, FR-H7 trade-off wording, FR-D1 detection order). Cite its anchors
       verbatim; re-verify each renders (the PRP Level-2 gate does this).
  section: "The six v2.1 capabilities" table + "Verified doc anchors" + "Shipped behavior snapshots".

- file: docs/how-it-works.md (§"Hook mode vs the snapshot-based flow", lines ~226-250)
  why: THE FR-H7 source. The README FAQ entry must MIRROR this trade-off (snapshot: atomic +
       stage-while-generating + rescue, but bypasses pre-commit hooks; hook mode: pre-commit hooks
       honored + never-block, but no snapshot/atomicity; the two COMPOSE). Do NOT invent a third mode.
  pattern: two bulleted blocks (Snapshot-based / Hook mode) + a "When to use which" note.
  gotcha: anchor is `#trade-off-inversion-fr-h7` (from "### Trade-off inversion (FR-H7)") — verify it
          resolves before linking; GitHub strips the `(`, `)`, and lowercase/dash-transforms.

- file: docs/cli.md (§"integrate install <target>…", "#### lazygit target", "#### git-alias target",
       "### models [<provider>]", "### hook install/uninstall/status/exec", "### config init")
  why: THE shipped command surface the README points at. The lazygit `customCommands` block at ~L287 is
       the canonical YAML (key/context/command/loadingText/output/description + `# stagecoach-integration`
       marker). `integrate list` shows git-alias + lazygit (gitui blocked upstream → FUTURE_SPEC.md §1.2).
  pattern: `stagecoach integrate install lazygit` (default key <c-a>) / `--key '<c-s>'` / `--yes`;
           `stagecoach integrate install git-alias` / `--alias-name <n>`.
  gotcha: the README's CURRENT manual YAML (lazygit binding section) is MISSING `context`, `description`,
          and the `# stagecoach-integration` marker vs the shipped block — the kept alternative MUST be
          updated to match the shipped shape (or just point readers at docs/cli.md's block).

- file: FUTURE_SPEC.md (repo root — READ-ONLY, NEVER modify)
  why: THE deferred/rejected registry. README links to it for: gitui (blocked, §1.2), GitHub Action
       (§2.1), and the rejected table (§3: API-key HTTP, PR title/body, interactive confirm default,
       generate-N-and-pick, file multiselect, push PROMPT, chunking, clipboard, self-update, config
       describe, locale i18n trees). The README "Why not opencommit/aicommits?" section is the natural
       place to add a one-line "what we deliberately didn't build → FUTURE_SPEC.md" pointer.
  pattern: relative link from README root: `[FUTURE_SPEC.md](FUTURE_SPEC.md)`; from docs/README.md:
           `[FUTURE_SPEC.md](../FUTURE_SPEC.md)`.

- file: .markdownlint.json
  why: THE lint config. `default: true` (all rules on); MD013 (line length), MD033 (inline HTML),
       MD060 disabled. So `<details>`, raw HTML, and long lines are FINE; but heading-increment
       (MD001), no-duplicate-heading (MD024), no-bare-URLs (MD034), required-headings, list-style,
       etc. ARE enforced. Keep headings unique within a file; don't skip levels; wrap bare URLs in
       `<>` or markdown link syntax.
  gotcha: MD024 (no-duplicate-heading) fires on identical heading text in the SAME file — if you add
          a "Features" heading, ensure it isn't a duplicate of an existing one. README's FAQ uses
          "### <question>" headings — keep them unique.

- url: PRD §21.5 (README structure), §10.4 (v2.1 accepted set), §5 (value prop / hero pitch),
       §9.20 FR-H7 (hook trade-off), §9.21 (integrations), §9.23 (models/interactive)
  why: THE product contracts the wording is judged against.
  section: read in plan/005_c38aa48290f0/prd_snapshot.md (the read-only PRD copy).
```

### Current Codebase tree (relevant slice — docs surface only)

```bash
README.md                     # EDIT — hero/features/quick-start/lazygit/FAQ/FUTURE_SPEC pointers
docs/
  README.md                   # EDIT — index table descriptions + capability pointer + FUTURE_SPEC link
  cli.md                      # READ-ONLY reference (hook/integrate/models/config-init landings)
  configuration.md            # READ-ONLY reference (exclusion globs / .stagecoachignore / [generation])
  how-it-works.md             # READ-ONLY reference (FR-H7 trade-off, payload exclusions, format modes)
  providers.md                # READ-ONLY reference (manifests, per-role defaults)
FUTURE_SPEC.md                # READ-ONLY — link target, never modify
```

### Desired Codebase tree with files to be edited

```bash
README.md                     # EDIT — add Features section; rewrite lazygit→integrate; expand FAQ (+FR-H7, +FUTURE_SPEC); light hero touch; quick-start v2.1 examples
docs/README.md                # EDIT — enrich index row descriptions; add capability-pointer note; link FUTURE_SPEC.md
# (no new files; no code; no other docs touched)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (two files, no more): this subtask edits ONLY README.md and docs/README.md. The stale
     `config_version = 2` string lives in internal/cmd/config.go:511 (CODE) — that is P1.M7.T1.S2's
     job (stale-reference sweep), NOT yours. Do not touch any .go file, any docs/*.md except
     docs/README.md, PRD.md, tasks.json, or FUTURE_SPEC.md. -->

<!-- CRITICAL (judge wording against SHIPPED behavior, not this plan): every command, flag default,
     and trade-off in the README must match what `stagecoach --help` / the docs/ pages say. When in
     doubt, quote docs/cli.md and docs/how-it-works.md verbatim rather than paraphrasing. -->

<!-- CRITICAL (anchors resolve, or the discovery goal fails): GitHub's anchor algorithm lowercases the
     heading, replaces spaces with `-`, and strips most punctuation (backticks, parentheses, periods,
     brackets). "### Trade-off inversion (FR-H7)" → #trade-off-inversion-fr-h7. "### `models [<provider>]`"
     → #models-provider. "### Exclusion globs (`[generation].exclude`)" → #exclusion-globs-generationexclude.
     VERIFY each anchor with the Level-2 gate before declaring done — a broken anchor is a silent failure. -->

<!-- CRITICAL (the manual lazygit YAML drifted): README's current snippet is MISSING context/description/
     the # stagecoach-integration marker vs docs/cli.md's canonical block. When keeping the YAML as a
     collapsible alternative, either (a) update it to match docs/cli.md's shape, or (b) keep it minimal
     and point readers at docs/cli.md#lazygit-target for the full block. Don't ship a third YAML variant. -->

<!-- CRITICAL (don't bloat the hero): the v1+v2.0 hero pitch (uses your agent / snapshot / decompose) is
     the README's job-1. The six v2.1 capabilities belong in a Features section + quick-start + FAQ, NOT
     crammed into the hero paragraph. A single sentence in the hero ("plus exclusions, message shaping,
     hook mode, editor/git integrations") is the max; the Features section carries the detail. -->

<!-- CRITICAL (markdownlint MD024 no-duplicate-heading): README already has headings like "## FAQ" and
     "### Configure your agent". If you add "## Features", "### lazygit / git alias", or new FAQ
     questions, ensure no heading text duplicates an existing one in the same file. -->

<!-- CRITICAL (README is a GitHub repo-root doc; docs/README.md is one level down): relative links from
     README use docs/how-it-works.md, FUTURE_SPEC.md (no ../). From docs/README.md use ../README.md,
     ../FUTURE_SPEC.md, and sibling docs use how-it-works.md (no ./). -->

<!-- CRITICAL (scope vs P1.M6.T2.S1 wizard): the interactive wizard's own Mode-A docs (--interactive in
     docs/cli.md + docs/configuration.md) are written by P1.M6.T2.S1, NOT you. Your job is to SURFACE
     `config init --interactive` in the README "Configure your agent" section + index — assume its docs
     landing exists. Do not author the --interactive flag docs yourself. -->
```

## Implementation Blueprint

### Data models and structure

No data models. The two edits are Markdown prose + a verified set of relative links/anchors. The
"structure" deliverable is:

1. A **Features** section in README (ordered M1→M6) with a stable one-liner per capability + a link.
2. A rewritten **lazygit / git alias** subsection (primary = `integrate install`, alternative = YAML).
3. Two new **FAQ** entries (FR-H7 trade-off; "what about X?" → FUTURE_SPEC.md) + FUTURE_SPEC pointer
   in the "Why not opencommit/aicommits?" section.
4. An enriched **docs/README.md** index (row descriptions + a capability-pointer note + FUTURE_SPEC).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT README.md — add a "## Features" section (the primary v2.1 discovery surface)
  - PLACE: directly AFTER "## Why not opencommit/aicommits?" (and its comparison table + the
    <details> "Which coding plans gate…") and BEFORE "## Install". Features sits between the
    positioning argument and the install instructions — visitors who skim learn the capability set
    before they decide to install.
  - IMPLEMENT: a short lead sentence ("Stagecoach does one thing — commit messages — and a few things
    around them.") then a table OR a bulleted list of the six capabilities, each row = capability name
    + one-line description + link into docs/. Order M1→M6:
       1. Payload exclusions — `.stagecoachignore` / `--exclude` hide a file's diff from the model
          (never from the commit). → docs/configuration.md#exclusion-globs-generationexclude
       2. Message shaping — `--format` (auto|conventional|gitmoji|plain), `--locale`, `--context`,
          `--template '$msg'`. → docs/how-it-works.md#format-modes-and-locale
       3. Git hook mode — `stagecoach hook install` fills the message on `git commit` (pre-commit
          hooks honored; never blocks). → docs/how-it-works.md#trade-off-inversion-fr-h7
       4. Tool integrations — `stagecoach integrate install git-alias lazygit` wires `git stagecoach`
          and a lazygit keybind via a no-mangle write protocol. → docs/cli.md#integrate-install-target
       5. `--edit` / `--push` — review in `$EDITOR` before the atomic commit; push after a clean run.
          → docs/cli.md (global flags)
       6. Discovery — `stagecoach models [<provider>]` and `config init --interactive` (guided setup).
          → docs/cli.md#models-provider
  - FOLLOW pattern: the existing README comparison table (pipe syntax) and the "Adding a new agent"
    section's link style ([text](docs/file.md#anchor)).
  - NAMING: "## Features" (verify not a duplicate heading — README has no existing Features heading).
  - GOTCHA: keep each one-liner accurate to SHIPPED behavior (e.g. exclusions are payload-ONLY — say
    "hides from the model, never the commit"; hook mode HONORS pre-commit hooks, not bypasses).

Task 2: EDIT README.md — rewrite the "### lazygit binding" subsection → "### lazygit & git alias"
  - FIND: the current "### lazygit binding" section (a single ```yaml code fence with a 5-field manual
    customCommands block missing context/description/marker).
  - REPLACE primary path with the install command:
        ```bash
        stagecoach integrate install lazygit      # default key <c-a>; --key '<c-s>' to customize
        stagecoach integrate install git-alias     # enables `git stagecoach` everywhere
        stagecoach integrate list                  # see what's installed / detected
        ```
  - KEEP the manual YAML as a <details> collapsible "Manual install (no stagecoach integrate)" — and
    UPDATE it to match the SHIPPED shape from docs/cli.md (add `context: 'files'`, `description:
    'stagecoach: AI commit'`, and the `# stagecoach-integration` marker comment), OR keep it minimal
    and link to docs/cli.md#lazygit-target for the canonical block. Pick ONE; do not ship a third.
  - NOTE gitui is blocked upstream → one line "(gitui isn't supported — see FUTURE_SPEC.md)".
  - FOLLOW pattern: README's existing <details> (the "Which coding plans gate…" block) for the
    collapsible; the docs/cli.md#lazygit-target block for the YAML shape.
  - PLACE: keep this subsection where "lazygit binding" currently sits (under "Quick start", after
    the "Multi-commit decomposition" note). Rename the heading to "lazygit & git alias".
  - GOTCHA: heading rename — ensure "### lazygit & git alias" is unique (no MD024 duplicate).

Task 3: EDIT README.md — Quick start: add 1–2 v2.1 convenience examples
  - FIND: the "## Quick start" block (currently stages, runs `stagecoach`, `-a`, `--dry-run`) +
    the "Multi-commit decomposition" subsection.
  - ADD (after the existing examples, before or within a new "### More options" mini-block) a SMALL
    set showing the v2.1 surface without overwhelming the quick start — e.g.:
        stagecoach --push                 # commit + push after a clean run
        stagecoach --edit                 # review in $EDITOR before the atomic commit
        stagecoach --format conventional  # force conventional-commit style
        stagecoach --exclude '*.snap'     # hide snapshot diffs from the model (still committed)
  - LINK: "See Features above and the CLI reference for the rest."
  - GOTCHA: keep it to ~4 lines — quick start must stay quick. Do NOT duplicate the Features table.

Task 4: EDIT README.md — FAQ: add the FR-H7 trade-off entry + a "what about X?" → FUTURE_SPEC entry
  - FIND: the "## FAQ" section (existing questions: "not for you if…", corrupt?, send code anywhere?,
    multiple commits?, match style?, which agents?, see what command?).
  - ADD question 1 (FR-H7 — pre-commit hooks), e.g. "### Does it run my pre-commit hooks?":
       Answer mirrors docs/how-it-works.md#trade-off-inversion-fr-h7: the default `stagecoach` command
       builds the commit via git PLUMBING (write-tree/commit-tree/update-ref) for atomicity +
       stage-while-generating — so pre-commit hooks (husky, lint-staged, .pre-commit-config.yaml) do
       NOT run on it. For day-to-day commits where pre-commit hooks MUST run, install hook mode
       (`stagecoach hook install`) and use plain `git commit` — generation fills the message and never
       blocks the commit. The two modes compose. Link → docs/how-it-works.md#trade-off-inversion-fr-h7.
  - ADD question 2 (deferred/rejected), e.g. "### What about PR generation / editor extensions / a
       GitHub Action / API-key providers?":
       Answer: "Stagecoach writes commit messages — nothing else (PRD §6.3). Ideas we considered but
       deferred or rejected — VS Code/neovim extensions, a GitHub Action, gitui integration, API-key
       HTTP providers, generate-N-and-pick, diff chunking, self-update, and more — each with its
       reason — live in FUTURE_SPEC.md." Link → FUTURE_SPEC.md.
  - ADD a one-line pointer in the "Why not opencommit/aicommits?" section's tail: "What we
       deliberately didn't build is tracked in FUTURE_SPEC.md." (so FUTURE_SPEC is discoverable from
       the positioning argument, not just the FAQ).
  - FOLLOW pattern: existing FAQ "### <question>?" headings + body prose + link style.
  - GOTCHA: every FAQ heading must be unique (MD024). Verify "### Does it run my pre-commit hooks?"
     isn't a near-duplicate of existing text. Keep the FR-H7 wording faithful to docs/how-it-works.md.

Task 5: EDIT README.md — hero/intro: light-touch v2.1 discoverability (DO NOT bloat)
  - FIND: the hero block-quote (the "> **Stagecoach writes your commit messages…**" pitch + the
    "A snapshot-based AI commit message generator…" line).
  - EDIT: append ONE sentence (max) to the prose line noting the v2.1 additions exist, e.g.
    "v2.1 adds payload exclusions, message shaping, git hook mode, git/lazygit integrations,
    --edit/--push, and model discovery — see Features below." Do NOT rewrite the core pitch.
  - GOTCHA: the hero is job-1 (sell the core value). One sentence only; the Features section carries
     the detail. If the sentence makes the block-quote unwieldy, move it to the line beneath instead.

Task 6: EDIT docs/README.md — refresh the index for v2.1 sections + link FUTURE_SPEC.md
  - FIND: the "## Documentation index" 4-row table and the "## Product specification" section.
  - EDIT the table DESCRIPTIONS so each row names the v2.1 sections it now carries:
       • CLI reference — add: hook (install/uninstall/status/exec), integrate (git-alias/lazygit +
         no-mangle protocol), models, and the v2.1 global flags (--exclude, --format, --locale,
         --context, --template, --edit, --push).
       • Configuration — add: exclusion globs + .stagecoachignore, [generation] shaping keys (format/
         locale/template), STAGECOACH_PUSH, and config init --interactive (per P1.M6.T2.S1).
       • How Stagecoach works — add: payload exclusions, format modes & locale, the hook-vs-snapshot
         trade-off (FR-H7), and stage-while-editing (--edit).
       • Provider manifests — unchanged description (per-role default models FR-D4, list_models_command).
  - ADD a short "Capability index" note (a compact list or a second small table) mapping each of the
    six capabilities → its doc anchor, so the index is the single navigation surface. (This can be a
    bulleted list right after the table.)
  - EDIT "## Product specification": after the PRD.md line, add a FUTURE_SPEC.md line:
    "The [FUTURE_SPEC.md](../FUTURE_SPEC.md) lists deferred and rejected ideas — each with its reason."
  - GOTCHA: keep the "binary is authoritative" note and the "See the README" pointer. Do NOT duplicate
     the README's install block (it's already a pointer). Relative link to FUTURE_SPEC uses ../ (docs/
     is one level down). Verify any new anchors you cite resolve (Level-2 gate).

Task 7: VALIDATE — markdown lint + link/anchor integrity + capability coverage audit
  - RUN (best-effort lint; node+npx are on PATH, may download markdownlint-cli2 once):
        npx --yes markdownlint-cli2 README.md docs/README.md
    Expected: zero violations against .markdownlint.json (default rules ON; MD013/MD033/MD060 off).
  - RUN the link/anchor integrity check (see Validation Loop Level 2 for the exact script). Expected:
    every relative link target file exists AND every `#anchor` matches a heading in the target file.
  - RUN the capability coverage audit grep (see Validation Loop Level 2). Expected: all six capability
    keywords present in README.md, and FUTURE_SPEC.md linked from BOTH README.md and docs/README.md.
```

### Implementation Patterns & Key Details

```markdown
<!-- === The shipped lazygit customCommands (the manual-YAML alternative must match THIS — from
     docs/cli.md ~L287). Keep this exact shape if you retain a manual block: === -->
customCommands:
  - key: '<c-a>'                       # stagecoach-integration
    context: 'files'
    command: 'stagecoach'
    loadingText: 'Generating commit message…'
    output: 'none'
    description: 'stagecoach: AI commit'

<!-- === The FR-H7 FAQ entry must mirror this trade-off (docs/how-it-works.md §228-250) === -->
<!-- Snapshot flow (default `stagecoach`):
     - atomic (write-tree/commit-tree/update-ref; repo byte-for-byte unchanged on failure)
     - stage-while-generating (snapshot decouples staged content from generation time)
     - rescue protocol (frozen tree SHA printed on failure)
     - BUT bypasses pre-commit hooks (husky/lint-staged/.pre-commit-config.yaml) — built via plumbing.
     Hook mode (`stagecoach hook install` + `git commit`):
     - pre-commit hooks honored (flows through real `git commit`)
     - never-block contract (failure → message untouched, exit 0, empty editor)
     - BUT no snapshot/atomicity, latency inside the commit.
     The two COMPOSE: hook for `git commit`, flagship for the atomic path. -->

<!-- === Relative-link discipline === -->
<!-- From README.md (repo root):  docs/how-it-works.md#anchor  |  FUTURE_SPEC.md  |  docs/cli.md#anchor -->
<!-- From docs/README.md (one level down):  ../README.md  |  ../FUTURE_SPEC.md  |  how-it-works.md#anchor (sibling) -->

<!-- === GitHub alert + collapsible patterns (already used in README; allowed by .markdownlint.json) === -->
> [!NOTE]
> Short, accurate callout text.

<details>
<summary><em>Manual install (no <code>stagecoach integrate</code>)</em></summary>
```yaml
customCommands:
  - key: '<c-a>'                       # stagecoach-integration
    ...
```
</details>
```

### Integration Points

```yaml
README.md (repo root):
  - add section: "## Features" between "Why not opencommit/aicommits?" and "Install" (Task 1)
  - rewrite section: "### lazygit binding" → "### lazygit & git alias" (integrate primary + YAML alt) (Task 2)
  - extend section: "## Quick start" (+1 mini-block of v2.1 examples) (Task 3)
  - extend section: "## FAQ" (+FR-H7 entry, +FUTURE_SPEC entry) and "Why not…?" tail (+1 FUTURE_SPEC line) (Task 4)
  - edit block: hero prose (+1 v2.1 sentence) (Task 5)

docs/README.md:
  - edit table: "Documentation index" 4-row descriptions enriched for v2.1 sections (Task 6)
  - add note: "Capability index" list mapping the six capabilities → doc anchors (Task 6)
  - edit section: "Product specification" (+FUTURE_SPEC.md line) (Task 6)

NOT touched (scope fences):
  - internal/** (stale config_version=2 string is P1.M7.T1.S2's job)
  - docs/{cli,configuration,how-it-works,providers}.md (Mode-A docs from M1–M6 / P1.M6.T2.S1)
  - FUTURE_SPEC.md, PRD.md, tasks.json, prd_snapshot.md (read-only / orchestrator-owned)
```

## Validation Loop

### Level 1: Markdown Lint (Immediate Feedback)

```bash
# node + npx are on PATH; markdownlint-cli2 downloads on first use (uses .markdownlint.json in repo root).
npx --yes markdownlint-cli2 README.md docs/README.md
# Expected: no output / exit 0. If MD024 (duplicate heading) fires, rename the offending heading.
# If MD034 (bare URL) fires, wrap the URL in <> or [text](url). MD001 (heading increment): don't skip levels.

# If npx/network is unavailable, fall back to checking the disabled rules are respected by hand and
# running the Level-2 link/anchor + coverage gates (those are the PRIMARY gates for a docs task).
```

### Level 2: Link / Anchor Integrity + Capability Coverage (the PRIMARY gates)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity

# (A) Every relative-link TARGET FILE exists in README.md and docs/README.md.
echo "=== broken file-link targets (should be empty) ==="
grep -oE '\]\((docs/[^)#]+|../[^)#]+|[^)#]+\.md)' README.md docs/README.md \
  | sed -E 's/.*\((.*)/\1/' | sort -u | while read -r p; do
      # resolve relative to each file's dir
      ( cd "$(dirname README.md)" && [ -e "$p" ] ) 2>/dev/null || true
    done
# Simpler, more reliable: extract targets and test existence directly.
for f in README.md docs/README.md; do
  dir=$(dirname "$f")
  grep -oE '\]\([^)]+\)' "$f" | sed -E 's/^\]\(//; s/\)$//' | grep -E '^[^h]' | grep -vE '^#' \
    | while IFS= read -r link; do
        target="${link%%#*}"
        [ -e "$dir/$target" ] || echo "BROKEN in $f: $link (missing $dir/$target)"
      done
done
echo "(end file-link check)"

# (B) Every #anchor matches a heading in the target file (GitHub slug algorithm).
echo "=== broken anchors (should be empty) ==="
check_anchor () {  # args: from_file  target_file  anchor
  local tf="$2" an="$3"
  [ -e "$tf" ] || { echo "BROKEN (no file) in $1: -> $tf#$an"; return; }
  # GitHub anchor algorithm: lowercase, drop every char that is NOT [a-z0-9 space _ -],
  # then spaces -> '-'. (Stripping the COMPLEMENT set is robust to backticks, all bracket
  # types, the ellipsis char, etc. — a denylist of punctuation misses ']' and '…' and would
  # produce false BROKEN results. Keep '_' since GitHub preserves it.)
  grep -oE '^#{1,6} .+' "$tf" \
    | sed -E 's/^#+ //; s/[^a-zA-Z0-9 _-]//g' \
    | tr '[:upper:]' '[:lower:]' | tr ' ' '-' \
    | grep -qxF "$an" || echo "BROKEN ANCHOR in $1: $tf#$an"
}
for f in README.md docs/README.md; do
  dir=$(dirname "$f")
  grep -oE '\]\([^)]+#[^)]+\)' "$f" | sed -E 's/^\]\(//; s/\)$//' \
    | while IFS='#' read -r path anchor; do
        # path may be empty (same-file anchor) or a docs-relative path
        if [ -z "$path" ]; then tf="$f"; else tf="$dir/$path"; fi
        check_anchor "$f" "$tf" "$anchor"
      done
done
echo "(end anchor check)"

# (C) Capability coverage audit — all six keywords present in README.md.
echo "=== capability coverage in README.md (each MUST print >=1) ==="
for kw in 'exclude' 'stagecoachignore' '--format' 'gitmoji' 'hook install' 'integrate install' 'lazygit' 'git-alias' '--edit' '--push' 'stagecoach models' 'interactive'; do
  n=$(grep -ci "$kw" README.md)
  printf '%-24s %s\n' "$kw" "$n"
done

# (D) FUTURE_SPEC linked from BOTH files.
echo "=== FUTURE_SPEC discoverability (each MUST be >=1) ==="
grep -c 'FUTURE_SPEC' README.md docs/README.md

# Expected: (A) and (B) print nothing broken; (C) every keyword >=1; (D) both >=1.
```

### Level 3: Manual Review (the behavior-fidelity gate)

```bash
# Word-for-word fidelity check against the shipped binary/docs (the README must match BEHAVIOR).
# Spot-check the four highest-risk claims:

# 1. lazygit customCommands shape in README matches docs/cli.md's canonical block.
diff <(sed -n '/customCommands:/,/^```/p' README.md) \
     <(sed -n '/customCommands:/,/^```/p' docs/cli.md) || echo "REVIEW: README lazygit YAML differs from docs/cli.md (intentional? minimal variant is OK if pointed at docs)"

# 2. FR-H7 FAQ wording matches the docs/how-it-works.md trade-off (atomic+bypass vs hooks+honored).
grep -A6 'pre-commit\|FR-H7\|hook mode' README.md | head -20
# Manual eyeball: snapshot flow = atomic but BYPASSES pre-commit hooks; hook mode = HONORS them.
# Do NOT claim hook mode is atomic, and do NOT claim the snapshot flow runs pre-commit hooks.

# 3. Detection order in README "Configure your agent" still lists all 8 / cites FR-D1.
grep -oE 'pi.*opencode.*cursor.*agy.*gemini.*qwen-code.*codex.*claude' README.md && echo "OK: FR-D1 order intact" || echo "REVIEW: detection-order sentence changed"

# 4. Install section still says Build-from-source is the only working method today (no false promise).
grep -i 'build from source.*only working\|pre-release' README.md && echo "OK: install honesty intact" || echo "REVIEW: install-section honesty claim may have drifted"
```

### Level 4: Domain-Specific Validation (coherence sweep)

```bash
# (a) No capability is MISSING from the README surface — cross-check against the M1–M6 doc anchors.
echo "=== each capability's doc anchor is LINKED from README.md (each MUST be >=1) ==="
for anchor in 'exclusion-globs-generationexclude' 'format-modes-and-locale' 'trade-off-inversion-fr-h7' 'integrate-install-target' 'models-provider'; do
  printf '%-36s %s\n' "$anchor" "$(grep -c "$anchor" README.md)"
done

# (b) docs/README.md index mentions every v2.1 section keyword (hook/integrate/models/exclude/format).
echo "=== docs/README.md index coverage (each MUST be >=1) ==="
for kw in hook integrate models 'exclude\|exclusion' 'format' 'locale\|template' 'edit\|push' 'interactive' 'FUTURE_SPEC'; do
  printf '%-28s %s\n' "$kw" "$(grep -ciE "$kw" docs/README.md)"
done

# (c) Scope fence — confirm NO code / other docs / read-only files were modified by this task.
echo "=== git status (only README.md + docs/README.md should appear as modified) ==="
git status --porcelain

# (d) Stale-string sweep is NOT this task — confirm the known code stale string is untouched.
grep -n 'supports config_version = 2' internal/cmd/config.go >/dev/null && echo "OK: exampleConfigTemplate untouched (S2 owns it)" || echo "REVIEW: was the code stale string changed? (should be S2)"

# Expected: (a) every anchor linked; (b) every keyword present; (c) only README.md + docs/README.md
# modified; (d) the code stale string still present (S2's job, not S1's).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `npx --yes markdownlint-cli2 README.md docs/README.md` passes (or documented why
      network was unavailable and the rules were checked by hand).
- [ ] Level 2 (A): no broken relative-link file targets in README.md or docs/README.md.
- [ ] Level 2 (B): every `#anchor` matches a heading in its target file.
- [ ] Level 2 (C): all six capability keywords present in README.md (coverage audit prints >=1 each).
- [ ] Level 2 (D): `FUTURE_SPEC` linked from BOTH README.md and docs/README.md.

### Feature Validation

- [ ] README "## Features" section lists all six capabilities (M1→M6), each linked to its docs page.
- [ ] README "### lazygit & git alias" — primary path is `stagecoach integrate install lazygit`; manual
      YAML kept as a collapsible alternative matching the shipped `customCommands` shape; git-alias
      mentioned; gitui noted as blocked (FUTURE_SPEC.md).
- [ ] README FAQ has the FR-H7 pre-commit-hooks trade-off entry (snapshot bypasses / hook honors /
      they compose) linking `docs/how-it-works.md#trade-off-inversion-fr-h7`.
- [ ] README FAQ (and "Why not…?" tail) link FUTURE_SPEC.md for deferred/rejected ideas.
- [ ] README hero carries at most ONE v2.1 sentence (core pitch intact, not bloated).
- [ ] docs/README.md index row descriptions name the v2.1 sections; a capability-pointer note maps
      the six capabilities to their anchors; FUTURE_SPEC.md linked under Product specification.

### Code Quality Validation

- [ ] Wording judged against SHIPPED behavior (docs/cli.md, docs/how-it-works.md), not this plan.
- [ ] No heading duplicates (MD024); no skipped heading levels (MD001); no bare URLs (MD034).
- [ ] Relative links correct per file depth (README root vs docs/README one-level-down).
- [ ] Only README.md and docs/README.md modified (git status confirms; no code/other-docs/PRD/FUTURE_SPEC).

### Documentation & Deployment

- [ ] README's "binary authoritative" framing preserved (`stagecoach --help` is the source of truth).
- [ ] docs/README.md "binary is authoritative" note + "See the README" pointer preserved.
- [ ] No false promises (install section still marks package-managed channels as "Coming soon").

---

## Anti-Patterns to Avoid

- ❌ Don't touch any file other than `README.md` and `docs/README.md` (code stale-strings are S2; Mode-A
  docs are M1–M6 / P1.M6.T2.S1; FUTURE_SPEC/PRD/tasks.json are read-only/orchestrator-owned).
- ❌ Don't bloat the hero — the v2.1 detail belongs in Features + quick-start + FAQ, not the pitch.
- ❌ Don't ship a manual lazygit YAML that has drifted from the shipped `customCommands` shape (either
  match docs/cli.md or point readers at it).
- ❌ Don't invent a third commit mode or misstate the FR-H7 trade-off (snapshot BYPASSES pre-commit
  hooks; hook mode HONORS them — never claim the reverse).
- ❌ Don't add bare URLs or duplicate headings (markdownlint MD024/MD034 will catch them — fix, don't disable).
- ❌ Don't paraphrase command syntax from memory — quote `stagecoach --help` / docs/cli.md verbatim.
- ❌ Don't link an anchor without verifying it resolves (a broken `#anchor` is a silent discovery failure).
