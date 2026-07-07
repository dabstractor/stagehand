---
name: "P1.M5.T1.S2 (Mode B docs) — README.md payload-optimization feature row + cli.md/providers.md consistency verification"
description: |

  Mode B changeset-level documentation update. README.md (~20KB, PRD §21.5 marketing surface) has a
  `## Features` table that lists six capabilities (payload exclusions, message shaping, hook mode, tool
  integrations, --edit/--push, discovery) but does NOT mention the FR3d–FR3i diff-payload optimization
  shipped by the M2/M3/M4 work (rename-aware `-M`, reduced-context `-U1`, the numstat skeleton, and the
  optional `token_limit` holistic budget). Add ONE concise row to that table, keeping the hero pitch intact,
  and cross-link to the explainer (how-it-works.md, S1) and the knob reference (configuration.md,
  P1.M1.T1.S4) WITHOUT duplicating the per-key reference. THEN verify docs/cli.md and docs/providers.md are
  already consistent with the new config-only `[generation]` keys (they are — no edit) and record that
  verification.

  CONTRACT (item_description §1–§5; PRD §9.1 FR3d–FR3i, §21.5):
    1. RESEARCH NOTE: "README.md (~20KB) is the marketing surface (PRD §21.5 structure). docs/providers.md
       and docs/cli.md document the provider manifests and CLI. None mention token_limit/diff_context yet."
    2. INPUT: "the new capabilities (FR3d-i) + the config knobs (P1.M1.T1.S4 documented
       token_limit/diff_context in docs/CONFIGURATION.md)."
    3. LOGIC: "(a) In README.md, add a concise mention of payload optimization to the feature list / 'how it
       works' blurb (e.g. 'diffs are trimmed and budgeted: rename-aware, reduced-context, skeleton-headed,
       and optionally capped to your model's context window via token_limit') — keep the hero pitch intact.
       (b) Verify docs/cli.md's global-flags / config tables are consistent with the new [generation] keys
       (they should already be, since the keys are config-only with no new flags — confirm no CLI flag was
       added and the docs don't claim one). (c) Verify docs/providers.md needs no change (it doesn't — the
       transforms are provider-agnostic). Do NOT duplicate the per-key reference (that lives in
       docs/CONFIGURATION.md from P1.M1.T1.S4)."
    4. OUTPUT: "README surfaces the capability; providers/cli docs confirmed consistent."
    5. DOCS: "[Mode B] THIS is the README + overview sweep. Together with S1 and P1.M1.T1.S4, all
       changeset-level docs are covered."

  DELIVERABLE (1 file MODIFIED; 2 files VERIFIED no-change; no code, no tests):
    MODIFY README.md — add ONE row ("Payload optimization") to the `## Features` table, immediately AFTER
      the "Payload exclusions" row. Concise, accurate to the shipped implementation, dual cross-link to
      docs/how-it-works.md#diff-capture-pipeline (S1) + docs/configuration.md#built-in-defaults (knob ref).
      Hero pitch (top blockquote + "v2.1 adds" line) UNTOUCHED.
    VERIFY docs/cli.md — already consistent (token_limit/diff_context are config-only, no CLI flag; the
      global-flags table and flag↔env↔git-config map correctly omit them). NO edit.
    VERIFY docs/providers.md — provider-agnostic transforms; no token_limit/diff_context mention needed. NO edit.

  SCOPE BOUNDARY (do NOT edit):
    - README.md hero pitch (lines 1–6: the top `>` blockquote + the "v2.1 adds…" line) — KEEP INTACT.
    - docs/how-it-works.md (S1 owns it; the `### Diff capture pipeline` section is DONE at line 130) — LINK only.
    - docs/configuration.md (P1.M1.T1.S4 owns the per-knob reference) — LINK only; do NOT duplicate it.
    - docs/cli.md, docs/providers.md — VERIFY only; do NOT add token_limit/diff_context rows.
    - ANY source code / tests (Mode B docs-only).

  SUCCESS: README.md `## Features` has a "Payload optimization" row accurate to the implementation
  (rename-aware `-M`, reduced-context `-U1`, file skeleton, optional `token_limit` budget) with working
  cross-links; hero pitch unchanged; cli.md and providers.md confirmed consistent (zero token_limit/
  diff_context references — correct); ONLY README.md modified; well-formed markdown.

---

## Goal

**Feature Goal**: Surface the FR3d–FR3i diff-payload optimization (shipped by M2/M3/M4) on Stagecoach's
marketing surface. Today the README `## Features` table lists six capabilities but never tells a reader that
stagecoach trims and budgets the diff before the agent sees it — a user whose diff exceeds their model's
context window has no README-level hint that `token_limit` exists, and a user inspecting `--verbose` output
(compact renames, `-U1`, a numstat skeleton header, `... [truncated]` markers) has no README context for any
of it. Add ONE concise, accurate row to the Features table, and confirm the other overview docs (cli.md,
providers.md) are already consistent with the new config-only `[generation]` keys.

**Deliverable** (1 modified file; 2 verified no-change; no code, no tests):
- `README.md` — one new "Payload optimization" row in the `## Features` table, placed immediately after the
  "Payload exclusions" row, dual cross-linking to `docs/how-it-works.md#diff-capture-pipeline` (the S1
  pipeline explainer) and `docs/configuration.md#built-in-defaults` (the knob reference). Hero pitch untouched.
- `docs/cli.md` — VERIFIED consistent (no edit): `token_limit`/`diff_context` are config-only (no CLI flag);
  the global-flags table and flag↔env↔git-config map correctly omit them.
- `docs/providers.md` — VERIFIED provider-agnostic (no edit): the transforms run in `internal/git/` before
  any provider renders; providers.md correctly has no token_limit/diff_context mention.

**Success Definition**:
- A reader scanning README's Features table learns, in one line, that the diff is trimmed and budgeted
  (rename-aware, reduced-context, skeleton-headed, optionally `token_limit`-capped) and can click through to
  the how-it-works explainer or the configuration knobs.
- Every literal in the new row matches the shipped implementation (`-M`, `-U1`, `token_limit`, "file
  skeleton", "model's context window").
- The hero pitch (top blockquote + "v2.1 adds" line) is byte-identical to before.
- `docs/cli.md` and `docs/providers.md` are confirmed already consistent (zero erroneous flag/transform
  claims); they are NOT edited.
- ONLY `README.md` is modified; markdown is well-formed.

## User Persona

**Target User**: the prospective stagecoach user reading the README to decide whether to install/use it —
especially a user on a model with a tight context window (who needs to know `token_limit` exists), or a user
who ran `--verbose`, saw a numstat skeleton / compact renames / `... [truncated]` markers, and wants a
README-level explanation pointer.

**Use Case**: user opens README → scans the Features table → sees "Payload optimization" → understands the
diff is preprocessed (not sent raw), and that `token_limit` lets them fit any model's window → clicks
through to how-it-works.md (what it does) or configuration.md (the knobs) → sets `token_limit = 120000`
informedly.

**User Journey**: Features table row → (optional) how-it-works `#diff-capture-pipeline` for the mechanics →
(optional) configuration.md `#built-in-defaults` for the exact knob values/precedence → informed config.

**Pain Points Addressed**: today the README never mentions the payload is optimized, so (a) a user hitting a
context-window overflow has no idea stagecoach can budget the diff, and (b) `--verbose` output that shows the
skeleton/truncation looks unfamiliar. One Features row + two cross-links closes both gaps without bloating
the marketing surface.

## Why

- **It IS the P1.M5.T1.S2 contract.** Mode B README + overview sweep: the FR3d–FR3i work shipped code
  (M2/M3/M4) and config docs (P1.M1.T1.S4) but never reached the README's feature surface. This adds it.
- **Discovery of `token_limit`.** `token_limit` is the single most user-visible knob from this changeset
  (the "fits my model's context window" lever). The README is the one place users look first; surfacing it
  there (with a link, not a duplicate reference) is the highest-leverage docs change.
- **Complements S1 + P1.M1.T1.S4 without duplication.** S1 owns the explainer (how-it-works.md); P1.M1.T1.S4
  owns the knob reference (configuration.md). This task OWNS the README row and POINTS at both — it does not
  re-document the knobs (contract §d).
- **Cheap, surgical, low-risk.** One table row in one file. No code, no tests, no cross-file churn. The
  cli.md/providers.md verification produces zero edits (they're already consistent).
- **Accuracy over marketing fluff.** The row names the real transforms (`-M`, `-U1`, skeleton, `token_limit`)
  so a curious user recognizes them in `--verbose` output — not a vague "smart diff" claim.

## What

**README.md** — insert ONE new row into the `## Features` table, immediately AFTER the "Payload exclusions"
row (thematically adjacent: both concern the agent payload). The row:

```
| Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |
```

Nothing else in README.md changes. The hero pitch (lines 1–6), the comparison table, the install/quick-start
sections, the snapshot-workflow section, and the FAQ are all UNTOUCHED. In particular, do NOT add the diff
optimization to the "v2.1 adds…" line (it is a changeset-level quality improvement, not a version-flagged
v2.1 feature set — that line must stay accurate).

**docs/cli.md** — VERIFY only (no edit): confirm the global-flags table and the flag↔env↔git-config map
contain NO `token_limit`/`diff_context` entry (correct — they are config-only with no CLI flag). Record the
verification; make no change.

**docs/providers.md** — VERIFY only (no edit): confirm it has no token_limit/diff_context/diff-pipeline
mention (correct — the transforms are provider-agnostic). Record the verification; make no change.

### Success Criteria

- [ ] README.md `## Features` table has a new "Payload optimization" row placed immediately after "Payload
      exclusions", matching the table's existing `| <Capability> | <desc> ([docs](anchor)) |` shape.
- [ ] The row is accurate: names `-M`, `-U1`, a "file skeleton", and `token_limit`; says "optionally" for
      `token_limit` (it is `0`/unset by default ⇒ legacy caps); does NOT claim auto-enabling.
- [ ] The row has working cross-links to `docs/how-it-works.md#diff-capture-pipeline` AND
      `docs/configuration.md#built-in-defaults` (both verified to exist).
- [ ] The hero pitch (README lines 1–6) is byte-identical to before.
- [ ] docs/cli.md is confirmed consistent (no `token_limit`/`diff_context` flag claim) — NOT edited.
- [ ] docs/providers.md is confirmed provider-agnostic (no transform mention) — NOT edited.
- [ ] ONLY `README.md` is modified; well-formed markdown; no code/test changes.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the exact insertion point
(after the "Payload exclusons" row in `## Features`), the ready-to-paste row text (verified accurate), the
two verified cross-link anchors, the explicit "do not touch the hero pitch / v2.1 line / other docs files"
scope, and the verification commands for cli.md/providers.md. No code reading is required (the verified-fact
table in findings §4 already confirms every literal).

### Documentation & References

```yaml
# MUST READ — THE decisive doc (insertion point + verified row text + cross-links + scope)
- docfile: plan/007_b33d310438c6/P1M5T1S2/research/findings.md
  why: §1 the contract restated; §2 README structure + the EXACT insertion point + the ready-to-paste row;
       §3 the two cross-link targets (BOTH verified to exist); §4 the verified-implementation-facts table
       (every README literal confirmed against source); §5 cli.md consistency check (PASS, no edit);
       §6 providers.md consistency check (PASS, no edit); §7 scope vs S1 + P1.M1.T1.S4; §8 validation; §9 risks.
  critical: §2 (the row text + placement), §3 (anchors exist), §5/§6 (verify, don't edit cli/providers).

# MUST READ — the file being EDITED
- file: README.md   (EDIT — add ONE Features-table row)
  section: `## Features` — the 6-row capability table. INSERT the new "Payload optimization" row immediately
       AFTER the "Payload exclusions" row (the table's first row). Mirror that row's exact cell shape.
  why: this is THE insertion point. The Features table is the README's capability surface; payload
       optimization is a capability and belongs here, not in the snapshot-workflow narrative.
  pattern: each row is `| <Capability> | <one-line desc with a [docs](relative-anchor) link> |`. The existing
           "Payload exclusions" row is the closest stylistic match (payload-themed, single docs link). The new
           row uses TWO links (`[how it works](…) · [knobs](…)`) — valid in a markdown table cell.
  gotcha: do NOT touch the hero pitch (lines 1–6: the `>` blockquote + "v2.1 adds…" line). Do NOT add the
          optimization to the "v2.1 adds" list (it is not a v2.1 feature set). Do NOT edit the snapshot-workflow
          section (it covers the concurrency model, not the diff pipeline).

# MUST READ — cross-link TARGET #1 (verify the anchor; do NOT edit — S1 owns it)
- file: docs/how-it-works.md   (READ ONLY)
  section: `### Diff capture pipeline` (line ~130 — VERIFIED PRESENT). The S1 (P1.M5.T1.S1) explainer for the
       five transforms (skeleton, -M, -U1, index-strip, token_limit water-fill).
  why: the README row's "how it works" link points here. The anchor `#diff-capture-pipeline` is derived from
       this H3 and is guaranteed valid.
  gotcha: S1 is implemented/present; do NOT edit how-it-works.md from this task. If the heading text ever
          changes, update the README anchor to match.

# MUST READ — cross-link TARGET #2 + the knob reference (verify the anchor; do NOT edit — P1.M1.T1.S4 owns it)
- file: docs/configuration.md   (READ ONLY)
  section: `## Built-in defaults` (heading → anchor `#built-in-defaults`): the defaults table
       (`max_diff_bytes=300000`, `max_md_lines=100`, `token_limit=0`, `diff_context=1`) AND the
       `> **Token budget & diff context.**` callout (the authoritative per-knob explanation).
  why: the README row's "knobs" link points here. This is where the per-key reference LIVES — the README must
       LINK, not duplicate (contract §d).
  gotcha: the filename is LOWERCASE (`configuration.md`) in all README links — match that (uppercase 404s on
          case-sensitive FS).

# VERIFY ONLY — tasks (b) and (c); do NOT edit
- file: docs/cli.md   (READ ONLY — verify consistency)
  section: `## Global flags` (the flag table) + `## Flag ↔ env ↔ git-config map` (the cross-reference table).
  why: confirm NEITHER table has a `token_limit`/`diff_context` entry (correct — they are config-only, no CLI
       flag). The verification is the deliverable; no edit. Do NOT add a row (there is no flag/env triple to
       document, and the per-key reference lives in configuration.md).
- file: docs/providers.md   (READ ONLY — verify provider-agnostic)
  section: the whole file (manifest schema, rendering, the 8 providers, tools-disable, tooled mode, per-role
       models, output parsing).
  why: confirm it has NO diff-pipeline mention (correct — the transforms run in internal/git/ before any
       provider renders, identically for all 8 providers). The verification is the deliverable; no edit.

# READ — the implementation sources (confirm the README literals; do NOT edit)
- file: internal/config/config.go   (READ ONLY)
  section: `TokenLimit int` (line 81, default 0), `DiffContext *int` (line 82, default intPtr(1)),
       `MaxDiffBytes` (300000), `MaxMdLines` (100). NO flag registration in internal/cmd/ or cmd/.
  why: confirms `token_limit`/`diff_context` are config-only (no CLI flag) — the basis for the cli.md
       consistency PASS and for the row's "optionally" wording (token_limit is 0/unset by default).
- file: internal/git/git.go   (READ ONLY)
  section: `buildDiffArgs` appends `-M` (line 695) and `-U<diff_context>` (line 696).
  why: confirms the `-M` (rename-aware) and `-U1` (reduced-context, diff_context default 1) literals in the row.
- file: internal/git/skeleton.go   (READ ONLY)
  section: `numstatSkeletonHeader` (FR3g numstat skeleton prepended to every payload).
  why: confirms the "compact file skeleton" literal in the row.

- url: (PRD internal) PRD.md §9.1 FR3d–FR3i (selected_prd_content h3.17) + §21.5 (README structure).
      AUTHORITATIVE for WHAT the transforms are and that the README is the marketing surface. Use the FR
      wording to sanity-check the row's claims; the implementation facts (findings §4) are the final arbiter.
```

### Current Codebase tree (relevant slice)

```bash
README.md                    # EDIT — add ONE "Payload optimization" row to ## Features (after "Payload exclusions").
docs/
  how-it-works.md            # READ ONLY — cross-link target #1 (### Diff capture pipeline @ ~line 130; S1 owns it).
  configuration.md           # READ ONLY — cross-link target #2 (## Built-in defaults; P1.M1.T1.S4 owns the knob ref).
  cli.md                     # VERIFY ONLY — confirm no token_limit/diff_context flag claim (consistent; no edit).
  providers.md               # VERIFY ONLY — confirm provider-agnostic (no transform mention; no edit).
internal/config/config.go    # READ ONLY — TokenLimit/DiffContext config-only knobs (confirm accuracy).
internal/git/git.go          # READ ONLY — buildDiffArgs -M/-U<n> (confirm accuracy).
internal/git/skeleton.go     # READ ONLY — numstat skeleton header (confirm accuracy).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 1 MODIFIED docs file + 2 VERIFIED no-change:
README.md                    # + one "Payload optimization" row in ## Features.
# docs/cli.md, docs/providers.md — VERIFIED consistent/provider-agnostic; NOT edited.
# NO code changes. NO tests. NO other docs files.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (hero pitch is FROZEN): do NOT edit README lines 1–6 — the top `>` blockquote ("Stagecoach writes
     your commit messages…") and the "v2.1 adds payload exclusions, message shaping, …" line. The contract says
     "keep the hero pitch intact." The edit is ONLY the Features table row. -->

<!-- CRITICAL (do NOT add to the "v2.1 adds" line): the diff optimization is a changeset-level quality
     improvement (P1 of the "Diff payload optimization" plan), NOT a version-flagged v2.1 feature set. Adding it
     to that line would be inaccurate. The Features TABLE is the correct home. -->

<!-- CRITICAL (token_limit is OPTIONAL / off by default): the row must say "optionally capped" (or equivalent).
     token_limit defaults to 0/unset ⇒ the legacy max_diff_bytes/max_md_lines caps apply. Do NOT imply it is on
     by default or auto-enables. (findings §4, config.go:81/174.) -->

<!-- CRITICAL (accuracy — every literal must match the implementation): `-M` (git.go:695, always-on rename
     detection), `-U1` (diff_context default 1; git.go:696), "file skeleton" (skeleton.go numstat header, FR3g),
     `token_limit` (config.go:81, the holistic budget knob). Do NOT invent transforms (e.g. don't claim `-C` copy
     detection — it is intentionally NOT enabled). -->

<!-- CRITICAL (do NOT duplicate the per-key reference): the row NAMES `token_limit` inline and LINKS to
     configuration.md#built-in-defaults; it must NOT restate the defaults (0/1/300000/100), the water-fill
     algorithm, or the two-mode exclusivity. That detail lives in configuration.md (P1.M1.T1.S4) + how-it-works.md
     (S1). The README is the marketing surface — one line, two links. -->

<!-- GOTCHA (cross-link casing): README uses lowercase `docs/<file>.md` everywhere (e.g.
     `docs/configuration.md#exclusion-globs-generationexclude`). Match that — `docs/CONFIGURATION.md` 404s on
     case-sensitive filesystems. The contract writes "CONFIGURATION.md" but the on-disk file is lowercase. -->

<!-- GOTCHA (cli.md/providers.md are VERIFY-ONLY): they are ALREADY consistent. Do NOT add a token_limit/
     diff_context row to cli.md (no CLI flag exists to document). Do NOT add a transform section to providers.md
     (the transforms are provider-agnostic). Editing them would be scope creep and would DUPLICATE the reference
     the contract says to avoid. -->

<!-- GOTCHA (anchor drift): the "how it works" link depends on S1's heading `### Diff capture pipeline`
     (VERIFIED at how-it-works.md:130). If a future S1 edit renames it, the README anchor must follow. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- No data models — one markdown table row. The "structure" is: a single `| … | … |` line inserted into the
     existing 6-row Features table, between "Payload exclusions" (row 1) and "Message shaping" (row 2), matching
     the table's cell shape exactly. -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: LOCATE the insertion point in README.md (READ, no edit yet)
  - FIND the `## Features` heading and its table. The FIRST data row is "Payload exclusions" (`.
stagecoachignore` / `--exclude` hide a file's diff from the model — never from the commit`).
  - CONFIRM the row shape: `| <Capability> | <desc with a ([docs](docs/…#anchor)) link> |` — three cells,
    pipe-delimited, the description cell ending in a parenthesized docs link.
  - NOTE the hero pitch lives ABOVE `## Features` (the `>` blockquote + "v2.1 adds…" line) — DO NOT touch it.
  - GOTCHA: if the Features table row order differs, locate "Payload exclusions" by its unique cell text
      ("hide a file's diff from the model — never from the commit") and insert the new row immediately after it.

Task 2: INSERT the "Payload optimization" row (THE deliverable)
  - FILE: README.md. INSERT one new table row IMMEDIATELY AFTER the "Payload exclusions" row (so the two
      payload-themed rows sit together: exclusions hide files; optimization trims the rest).
  - CONTENT (paste verbatim — verified accurate to findings §4):

        | Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |

  - WHY: this is the entire README edit. One line, accurate literals, dual cross-link, no per-key duplication.
  - GOTCHA: keep the pipe-delimited shape exact (3 cells). The two links share one cell, separated by ` · `
      (middle dot) — valid markdown in a table cell. Do NOT split into two rows. Do NOT restate the knob values.

Task 3: VERIFY docs/cli.md consistency (NO edit — task b)
  - RUN: `grep -niE 'token.?limit|diff.?context' docs/cli.md` → expect ZERO matches.
  - INSPECT the `## Global flags` table and the `## Flag ↔ env ↔ git-config map`: confirm NEITHER has a
      token_limit/diff_context entry. (They correctly omit them — config-only knobs with no CLI flag.)
  - OUTCOME: PASS — no edit. (Record the verification in the PRP/commit message; make no file change.)
  - GOTCHA: do NOT add a token_limit/diff_context row to cli.md. There is no flag/env/git-config triple to
      document, and the per-key reference lives in configuration.md (contract §d). Adding one would be wrong.

Task 4: VERIFY docs/providers.md provider-agnosticism (NO edit — task c)
  - RUN: `grep -niE 'token.?limit|diff.?context|skeleton|water|numstat' docs/providers.md` → expect ZERO
      matches (the only `--model`/`model_flag` hits are manifest SCHEMA fields, unrelated to the diff pipeline).
  - CONFIRM providers.md's scope (manifest schema, rendering, 8 providers, tools-disable, tooled mode,
      per-role models, output parsing) does not intersect diff capture/truncation — the transforms run in
      internal/git/ before any provider renders, identically for all 8 providers.
  - OUTCOME: PASS — no edit. (Record the verification; make no file change.)

Task 5: FINAL VALIDATION (docs-only gates)
  - RUN the grep checks in "Validation Loop → Level 1" (new row present; literals present; hero pitch
      untouched; both cross-link anchors exist; cli.md/providers.md still consistent).
  - VISUAL review: the Features table renders with 7 rows; the new row sits between "Payload exclusions" and
      "Message shaping"; both links resolve.
  - MARKDOWN sanity: valid table row (pipe-delimited, 3 cells); valid inline-link syntax.
  - `git status --porcelain` → ONLY README.md modified (NOT cli.md, providers.md, or any code).
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: mirror the Features table's existing row shape. "Payload exclusions" is:
     `| Payload exclusions | `.stagecoachignore` / `--exclude` hide a file's diff from the model — never from the commit ([docs](docs/configuration.md#exclusion-globs-generationexclude)). |`
     The new row is the SAME shape: `| <Capability> | <desc with links> |`. -->

<!-- PATTERN: cross-link like the rest of the README. Every Features row links to a specific anchor
     (docs/configuration.md#…, docs/how-it-works.md#…, docs/cli.md#…). The new row links to TWO anchors
     (how-it-works + configuration) — a ` · `-separated pair in one cell, both relative `docs/<file>.md#<anchor>`. -->

<!-- CRITICAL (accuracy): the row's literals are the verified implementation surface, not PRD paraphrase:
      - `-M` = always-on rename detection (git.go:695).
      - `-U1` = reduced context, diff_context default 1 (git.go:696, config.go:82/175).
      - "file skeleton" = the numstat completeness floor (skeleton.go, FR3g).
      - `token_limit` = the optional holistic budget (config.go:81, FR3d/FR3i); "optionally" because default 0.
      Do NOT claim `-C` (copy detection — intentionally off), and do NOT claim token_limit is on by default. -->

<!-- CRITICAL (scope — marketing surface, not reference): the row is ONE line. It names the knob and links out;
     it does NOT restate values, the water-fill algorithm, or the two-mode exclusivity. Those live in
     configuration.md (knobs) and how-it-works.md (mechanics). -->
```

### Integration Points

```yaml
README.FEATURES_TABLE (the only file edited):
  - insert: "one Payload optimization row immediately after the Payload exclusions row"
  - content: "rename-aware -M + reduced-context -U1 + file skeleton + optional token_limit budget; dual
    cross-link to how-it-works.md#diff-capture-pipeline + configuration.md#built-in-defaults"

DOCS.CLI (VERIFY — no edit):
  - check: "global-flags table + flag↔env↔git-config map contain NO token_limit/diff_context entry (correct)"
  - rationale: "config-only knobs; no CLI flag; per-key reference lives in configuration.md"

DOCS.PROVIDERS (VERIFY — no edit):
  - check: "no token_limit/diff_context/diff-pipeline mention (correct — provider-agnostic transforms)"
  - rationale: "transforms run in internal/git/ before any provider renders, identically for all 8 providers"

FROZEN/LEAVE (do NOT edit):
  - README.md hero pitch (lines 1–6) + the "v2.1 adds…" line.
  - docs/how-it-works.md (S1 owns it; LINK only) — ### Diff capture pipeline is DONE at line 130.
  - docs/configuration.md (P1.M1.T1.S4 owns the knob reference; LINK only; do NOT duplicate).
  - All other README sections (comparison table, install, quick start, snapshot workflow, FAQ, contributing).
  - ALL source code and tests (Mode B docs-only).
```

## Validation Loop

### Level 1: Content & cross-link checks (docs-only — the real gate)

```bash
# 1. The new row exists in the Features table:
grep -n 'Payload optimization' README.md
# Expected: one match — the new Features-table row.

# 2. The accurate literals are all present in that row:
grep -nE 'token_limit|`-M`|`-U1`|skeleton' README.md
# Expected: matches in the new row for EACH of: -M, -U1, skeleton, token_limit.

# 3. BOTH cross-link targets EXIST:
grep -n '^### Diff capture pipeline' docs/how-it-works.md   # → ~line 130 (the #diff-capture-pipeline anchor)
grep -n '^## Built-in defaults' docs/configuration.md       # → the #built-in-defaults anchor

# 4. The new row uses the correct LOWERCASE filenames in its links:
grep -n 'docs/how-it-works.md#diff-capture-pipeline' README.md
grep -n 'docs/configuration.md#built-in-defaults' README.md
# Expected: each appears once (in the new row). NOT CONFIGURATION.md (uppercase).

# Expected: row present; all literals present; both anchors exist; lowercase links.
```

### Level 2: Hero-pitch integrity & markdown well-formedness

```bash
# The hero pitch (lines 1–6) is byte-identical to before:
sed -n '1,6p' README.md
# Visual check: the `>` blockquote and the "v2.1 adds…" line are UNCHANGED (no payload-optimization addition
# to the v2.1 list — that line must stay as-is).

# The Features table gained exactly one row (now 7 data rows) and still renders:
sed -n '/^## Features/,/^## /p' README.md
# Visual check: 7 rows; "Payload optimization" sits between "Payload exclusions" and "Message shaping";
# pipe-delimited cells intact; both inline links valid.

# If a markdown linter is available:
markdownlint README.md 2>/dev/null && echo "markdownlint clean" || echo "(no markdownlint — visual review)"
```

### Level 3: Consistency verification (tasks b/c — expected ZERO edits)

```bash
# Task (b): cli.md has NO token_limit/diff_context flag claim (consistent):
grep -niE 'token.?limit|diff.?context' docs/cli.md && echo "WARNING: cli.md mentions the knobs (re-check)" || echo "OK: cli.md consistent — no token_limit/diff_context reference (config-only knobs have no CLI flag)"

# Task (c): providers.md has NO diff-pipeline mention (provider-agnostic):
grep -niE 'token.?limit|diff.?context|skeleton|water|numstat' docs/providers.md && echo "WARNING: providers.md mentions the pipeline (re-check)" || echo "OK: providers.md provider-agnostic — no transform mention"

# Scope audit — ONLY README.md changed:
git status --porcelain
# Expected: exactly one entry — README.md (modified). NOT docs/cli.md, NOT docs/providers.md, nothing under internal/.
```

### Level 4: Accuracy spot-check against the implementation (confidence, no edit)

```bash
# The README literals match the shipped code:
grep -n 'TokenLimit' internal/config/config.go            # the token_limit knob source-of-truth
grep -n 'args = append(args, "-M")' internal/git/git.go   # -M always-on (rename detection)
grep -n 'numstatSkeletonHeader' internal/git/skeleton.go  # the file skeleton (FR3g)
# Expected: each grep matches — the README row references these consistently with the implementation.

# Render check (optional): preview README.md in a markdown renderer; confirm the Features table shows the new
# row and both links navigate (how-it-works → Diff capture pipeline; configuration → Built-in defaults).
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: the "Payload optimization" row exists; all four literals (`-M`, `-U1`, skeleton, `token_limit`)
      present; both cross-link anchors exist; lowercase filenames used.
- [ ] Level 2: hero pitch (lines 1–6) byte-identical; Features table now has 7 rows and renders; markdown valid.
- [ ] Level 3: cli.md consistent (no token_limit/diff_context reference); providers.md provider-agnostic (no
      transform mention); ONLY README.md modified.
- [ ] Level 4: README literals match the implementation (`TokenLimit`, `-M`, `numstatSkeletonHeader`).
- [ ] No source code or test files touched; no other docs files touched.

### Feature Validation
- [ ] README.md `## Features` has a "Payload optimization" row placed immediately after "Payload exclusions".
- [ ] The row is accurate: rename-aware (`-M`), reduced-context (`-U1`), file skeleton, optional `token_limit`
      budget; "optionally" reflects the `0`/unset default; no false claims (no `-C`, no auto-enable).
- [ ] Working dual cross-links to `docs/how-it-works.md#diff-capture-pipeline` and
      `docs/configuration.md#built-in-defaults`.
- [ ] Hero pitch intact; "v2.1 adds" line NOT extended with the optimization.
- [ ] cli.md and providers.md confirmed consistent/provider-agnostic (verification recorded; no edits).

### Code Quality Validation
- [ ] The new row mirrors the Features table's existing cell shape and cross-link idiom.
- [ ] No duplication of configuration.md's per-key reference or how-it-works.md's mechanics.
- [ ] No marketing overstatement (no "smart diff" vagueness; real transform names).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] User-facing, one-line marketing prose; no code pasted; literals rendered readably.
- [ ] The knob is named exactly (`token_limit`) and points to configuration.md rather than restating values.
- [ ] No new env vars / flags invented; the row describes existing shipped behavior only.

---

## Anti-Patterns to Avoid

- ❌ **Don't touch the hero pitch.** The top blockquote + "v2.1 adds…" line (README lines 1–6) are FROZEN. The
  contract says "keep the hero pitch intact." The edit is the Features table row ONLY. (gotcha)
- ❌ **Don't add the optimization to the "v2.1 adds" list.** It is a changeset-level quality improvement, not a
  version-flagged v2.1 feature set. That line must stay accurate. The Features TABLE is the home. (gotcha)
- ❌ **Don't imply `token_limit` is on by default.** It is `0`/unset ⇒ legacy caps. Say "optionally capped".
  (gotcha; config.go:81/174.)
- ❌ **Don't invent transforms.** `-M` (rename) and `-U1` (context) are real; `-C` (copy detection) is
  intentionally OFF — do not claim it. Name only the shipped transforms. (gotcha)
- ❌ **Don't duplicate the per-key reference.** The row NAMES `token_limit` and LINKS to configuration.md; it
  must NOT restate the defaults (0/1/300000/100), the water-fill algorithm, or the two-mode exclusivity.
  (contract §d; gotcha.)
- ❌ **Don't use `CONFIGURATION.md` (uppercase).** README uses lowercase `docs/configuration.md` everywhere;
  uppercase 404s on case-sensitive FS. (gotcha.)
- ❌ **Don't edit cli.md or providers.md.** They are ALREADY consistent (token_limit/diff_context are
  config-only with no CLI flag; the transforms are provider-agnostic). Editing them would be scope creep AND
  would duplicate the reference the contract says to avoid. VERIFY only. (gotcha; tasks b/c.)
- ❌ **Don't edit how-it-works.md or configuration.md.** S1 owns the explainer; P1.M1.T1.S4 owns the knob
  reference. This task LINKS to both — it does not modify them. (scope.)
- ❌ **Don't restructure the README.** One row, one table. No moving sections, no new headings, no FAQ additions.
  The contract is a concise feature-surface mention. (scope.)
