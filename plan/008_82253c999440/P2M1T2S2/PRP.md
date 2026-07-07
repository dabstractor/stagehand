---
name: "P2.M1.T2.S2 — Mode A doc edit: docs/how-it-works.md planner mode-conditional + files + soft target"
description: |
  Pure documentation edit (Mode A — rides with P2.M1.T2.S1, the mode-conditional planner prompt builder).
  Bring `docs/how-it-works.md`'s planner narrative into line with FR-M3/M3b/M4 (the v2.2 planner changes:
  per-concept `files`, the auto-vs-forced mode-conditional `Rules:` block, the soft `max_commits/2`
  target, and the deterministic non-fatal coverage check). TWO surgical edits in the decompose section:
  (1) UPDATE the four-roles table planner OUTPUT cell (line 59) from `commits:[...]` to
  `commits:[{title,description,files}]`; (2) ADD one tight paragraph after "One-file short-circuit"
  (line 115) covering the mode-conditional rules block (auto leans toward SEVERAL + soft target;
  forced-count fixes the count), the per-concept `files` partition, and the coverage-check-logs-not-
  errors behavior. No new flags/keys; do NOT touch docs/cli.md or docs/configuration.md. No code, no
  tests, no mocks. Docs-only. Present tense (Mode A — by landing, S1 IS the behavior).
---

## Goal

**Feature Goal**: Make `docs/how-it-works.md`'s planner narrative consistent with the v2.2 planner
behavior (FR-M3/M3b/M4): the planner emits a per-concept `files` list (not just `title`/`description`),
its `Rules:` block is **mode-conditional** (auto-decompose leans toward SEVERAL with a soft
`max_commits / 2` target; forced-count `--commits N` fixes the count and drops the soft target), and a
deterministic **coverage check** logs (never errors on) any changed path no concept claimed — the
arbiter reconciles the leftovers. After this edit, the four-roles table shows `files` in the planner
output contract, and the "Key design points" section states the three behaviors the current doc omits.

**Deliverable**: ONE file modified — `docs/how-it-works.md` — with exactly two surgical edits in the
decompose section:
1. **Update** the four-roles table planner OUTPUT cell (line 59): `commits:[...]` → `commits:[{title,description,files}]`.
2. **Add** one tight paragraph after "One-file short-circuit" (line 115): mode-conditional rules +
   soft target + files partition + coverage check.

No other file touched. No code, no tests, no config keys, no new flags.

**Success Definition**: the four-roles table planner output cell reads
`JSON {count, single, commits:[{title,description,files}], message?}`; a new "**Mode-conditional planner
rules.**" paragraph sits between "One-file short-circuit" and "Arbiter leftover reconciliation" and
names (a) the mode-conditional `Rules:` block — auto leans toward SEVERAL + soft `max_commits/2` target
(default 6), forced-count fixes the count, only the hard cap (`max_commits`, default 12) errors;
(b) every concept carries a `files` list; (c) the deterministic coverage check logs-not-errors unclaimed
paths and the arbiter reconciles them; `git diff --stat -- docs/` shows ONLY `docs/how-it-works.md`;
`docs/cli.md` and `docs/configuration.md` are untouched.

## User Persona

**Target User**: The reader of `docs/how-it-works.md` — a stagecoach user (or contributor) trying to
understand the decompose pipeline's *planning* step. They run `stagecoach` (default auto-decompose) on a
mixed working tree and want to know: how does the planner decide how many commits, what stops it from
fanning a 3-concept tree into a dozen micro-commits, how does each stager know which files to touch,
and what happens to a file the planner didn't assign to any concept.

**Use Case**: A user with a 5-file changeset (a refactor + its test + an unrelated docs tweak) reads
the four-roles table + "Key design points" to set expectations. Today the table hides `files` (showing
`commits:[...]`), and the section never mentions the soft target, the auto-vs-forced rules, or the
coverage check — so the user can't predict the commit count or the leftover behavior. The edit surfaces
all three.

**User Journey**: open `docs/how-it-works.md` → the four-roles table shows the planner emits
`files` per concept → the "Key design points" paragraph explains the soft target (≈6 commits for a
typical tree), that `--commits N` overrides it, and that unassigned files are reconciled by the arbiter
(not silently dropped).

**Pain Points Addressed**: Removes the doc/code drift where the doc's planner contract omits `files`
(the central new field, FR-M3) and is silent on the soft target (FR-M4), the auto-vs-forced distinction
(FR-M2/§17.5), and the coverage check (FR-M3b) — the four v2.2 planner behaviors P2.M1.T1/T2 land.

## Why

- **FR-M3/M3b/M4 are the mandate.** PRD §9.14: FR-M3 (planner emits per-concept `files`), FR-M3b
  (deterministic non-fatal coverage check), FR-M4 (hard cap `max_commits` + soft target `max_commits/2`).
  §17.5 specifies the mode-conditional `Rules:` block (auto leans toward SEVERAL; forced-count fixes the
  count). P2.M1.T1.S1 (Files field — COMPLETE), P2.M1.T1.S2 (coverage check — COMPLETE), and
  P2.M1.T2.S1 (the mode-conditional prompt builder — PARALLEL) implement exactly these. This doc edit is
  the Mode A ride-with-the-work documentation of them (item §5).
- **The doc currently hides `files` and is silent on the three behaviors.** Line 59 shows
  `commits:[...]` — the new `files` field (the contract the model now emits and the stager now reads) is
  invisible. The "Key design points" section (lines 101–117) mentions the freeze, the overlap, the
  tree-to-tree diffs, the one-file short-circuit, and the arbiter — but never the soft target, the
  auto-vs-forced rules, the `files` partition, or the coverage check. Doc/code drift of this kind leaves
  users unable to predict decompose's commit granularity or its leftover handling.
- **Consistency within the doc.** The stager row already implies per-file staging ("Stage one concept's
  subset of files"); the planner output cell should name the `files` field that drives it. The new
  paragraph closes the loop between the planner (assigns files + count guidance) and the arbiter
  (reconciles unclaimed paths) — the two ends of the leftover story.
- **No code, no flags, no config.** Pure narrative addition. Mode A = the doc edit rides WITH the work
  (S1); there is no separate docs subtask beyond this one. The soft target is DERIVED from the existing
  `max_commits` (no new flag/key), so `cli.md`/`configuration.md` need no change.

## What

Two surgical edits to `docs/how-it-works.md` (decompose section), in present tense (by landing, the
S1/T1 behavior IS the live behavior):

1. **Update** the four-roles table planner OUTPUT cell (line 59): replace `commits:[...]` with
   `commits:[{title,description,files}]` so the contract shows the per-concept `files` field.
2. **Add** one new paragraph — bold-led "**Mode-conditional planner rules.**" — immediately after the
   "One-file short-circuit" bullet (line 115) and before "Arbiter leftover reconciliation" (line 117).
   The paragraph covers, in one tight block: the `Rules:` block is mode-conditional (auto leans toward
   SEVERAL, tempered by a soft `max_commits / 2` target — default 6 — so ordinary trees don't fan into
   micro-commits; only the hard cap `max_commits`, default 12, ever errors; forced-count `--commits N`
   fixes the count and omits the soft target); every concept carries a `files` list naming each path it
   touches (a single file split across two concepts is named in both, with the description saying which
   part belongs where), so each stager knows where to look; after the planner returns, a deterministic
   coverage check logs (but never errors on) any changed path no concept claimed, and the arbiter
   reconciles those leftovers.

No edits to: the planner Job cell, the other three table rows, lines 101–113, line 115 (One-file
short-circuit), line 117 (Arbiter leftover reconciliation — already freeze-correct from P1.M1.T2.S2),
the Safety bullets, the lock section, any other section, `docs/cli.md`, `docs/configuration.md`,
`docs/providers.md`, any code, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Success Criteria

- [ ] The four-roles table planner OUTPUT cell (line 59) reads `JSON `{count, single, commits:[{title,description,files}], message?}``
      (the `commits:[...]` shorthand is gone).
- [ ] A new "**Mode-conditional planner rules.**" paragraph sits between "One-file short-circuit" and
      "Arbiter leftover reconciliation" and is ONE paragraph (bold lead + prose, matching the
      neighboring bullet voice).
- [ ] The new paragraph names: the mode-conditional `Rules:` block; auto leans toward SEVERAL; the soft
      `max_commits / 2` target (default 6); only the hard cap `max_commits` (default 12) errors;
      forced-count `--commits N` fixes the count; every concept carries a `files` list; a file split
      across two concepts is named in both; the deterministic coverage check logs-not-errors unclaimed
      paths; the arbiter reconciles leftovers.
- [ ] `git diff --stat -- docs/` shows ONLY `docs/how-it-works.md`.
- [ ] `docs/cli.md` and `docs/configuration.md` are UNCHANGED (`git diff --stat` empty for both).
- [ ] No code files changed; no new flags/keys.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes both current passages verbatim (with exact line numbers — 59 and
115) and gives the exact target text for each (ready to paste), plus the behavior contract (soft target
= `max_commits/2` default 6; hard cap = `max_commits` default 12; forced-count omits the soft target;
`files` is per-concept; coverage check logs-not-errors; arbiter reconciles leftovers) and the scope
fence (which passages/files to leave alone). A docs edit needs no toolchain — only the two quoted
changes. No inference required.

### Documentation & References

```yaml
# MUST READ — the behavior contract + the authoritative PRD narrative
- file: PRD.md
  why: "§9.14 FR-M3 (planner emits per-concept `files` listing every path the concept touches; a file
        split across two concepts is named in both with the description saying which part belongs where),
        FR-M3b (deterministic non-fatal coverage check: union concept.Files, compare to the frozen
        changed-path set DiffTreeNames(baseTree, T_start), log unclaimed paths verbose, the arbiter
        reconciles — never aborts), FR-M4 (hard cap = max_commits default 12, the ONLY error; soft target
        = max_commits/2 default 6, guidance interpolated into the prompt, never errors). §17.5 is the
        mode-conditional prompt: opener + UNSTAGED framing + JSON-contract are SHARED; only the `Rules:`
        block changes (auto leans toward SEVERAL; forced-count fixes the count and has NO soft-target line)."
  critical: "FR-M4's 'soft target of max_commits / 2 (default 6) ... guidance, not enforcement — it never
             errors; only the hard cap does' is the single fact the new paragraph must NOT misstate. The
             doc must NOT imply the soft target errors or that forced-count carries a soft target."

# MUST READ — the architecture scout brief (verbatim PRD §17.5 rules + the Mode-A doc-edit spec)
- docfile: plan/008_82253c999440/docs/architecture/planner_prompt.md
  section: §2.5 (Mode-A doc edit) — "Add a short paragraph in the 'Key design points' section (after
          'One-file short-circuit', ~line 115) covering: the planner's rules block is mode-conditional
          (auto leans toward SEVERAL with a soft max_commits/2 target; forced-count fixes the count);
          every concept carries a files list naming the paths it touches, and a deterministic coverage
          check logs (not errors) any path no concept claimed — the arbiter reconciles leftovers. Update
          the roles table (line 59) planner-output cell to JSON {count, single, commits:[{title,description,files}], message?}."
  why: the verbatim spec for BOTH edits (the table cell target + the paragraph's three required points).
  gotcha: §2.5 is the scout brief; the paragraph wording is THIS task's to write (the brief gives the
          points, not the prose). Match the doc's existing bold-led single-paragraph voice.

# MUST READ — the S1 PRP (the mode-conditional builder this doc rides with; PARALLEL — assume landed)
- docfile: plan/008_82253c999440/P2M1T2S1/PRP.md
  why: "S1 lands the mode-conditional BuildPlannerSystemPrompt (forcedCount>0 ⇒ forced rules, NO soft
        target; forcedCount<=0 ⇒ auto rules with fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)).
        Confirms: the soft target uses Go integer division (12→6, 10→5, 20→10, 4→2); the hard cap stays
        in decompose/planner.go:132 (unchanged, guidance-only for the soft target); the JSON-contract
        const now references `files` to match PlannerCommit.Files. THIS doc edit describes the
        user-visible behavior S1 produces — write PRESENT tense."
  critical: "Treat S1 as landed. Write the doc in PRESENT tense — by the time this Mode A edit lands, S1
             IS the behavior. Do NOT say 'will' or 'planned'. The ASCII em-dash rule in S1 (substitute
             ' -- ' in the Go CONSTS) does NOT apply here — this is a Markdown doc; use real em-dashes."

# MUST READ — the FILE TO EDIT
- file: docs/how-it-works.md   (EDIT)
  section: "Multi-commit decomposition" → "The four roles" table (line 59 planner row) and "Key design
          points" (lines 101–117 bullet sequence). The two spots: line 59 (table output cell) and the
          gap between line 115 (One-file short-circuit) and line 117 (Arbiter leftover reconciliation).
  why: the two load-bearing edits live here. The doc convention for "Key design points" is a bold-led
          single paragraph per point (`**Title.** <prose>`) with moderate inline code (`T_start`,
          `tree[i]`, `max_commits`, `--commits N`) and real em-dashes (—) in prose.
  pattern: mirror the neighboring bullets' voice/density — "One-file short-circuit" (line 115) and
          "Arbiter leftover reconciliation" (line 117) are the immediate neighbors; match their length
          and symbol density.
  gotcha: the table is a GitHub Markdown table (pipe-delimited). Only the planner OUTPUT cell (4th
          column) changes; keep the pipe structure and the other three rows byte-identical. The new
          paragraph must NOT cite raw Go primitive names (DiffTreeNames, VerboseRawOutput) — mechanics
          live in the PRD/code; this doc states the user-visible behavior.

# MUST READ — THIS task's research (verbatim current text + exact target text + scope fence + voice)
- docfile: plan/008_82253c999440/P2M1T2S2/research/findings.md
  section: §1 (verbatim current text of both spots, line 59 + 115), §2 (exact copy-paste-ready target
          text for both edits), §3 (accuracy facts: soft target 6 / hard cap 12 / forced omits soft /
          files per-concept / coverage logs-not-errors), §4 (scope fence), §5 (voice — em-dashes OK in
          the doc, bold-led single paragraph, no Go primitive names), §6 (validation greps).
  critical: §2 (the exact target text — copy/paste) and §4 (what NOT to touch — cli.md/configuration.md
          are READ-ONLY; the other table rows / lines 101–113 / 115 / 117 are accurate and unchanged).

# Read-only cross-refs (do NOT edit — scope check)
- file: docs/cli.md
  why: "READ-ONLY scope check. Mentions --max-commits ONLY (lines 36, 399) — the hard cap flag; NO
        soft-target/files/coverage surface. FR-M3/M3b/M4 add no flags, so it needs no change."
- file: docs/configuration.md
  why: "READ-ONLY scope check. Mentions [generation].max_commits ONLY (lines 209, 217, default 12); NO
        soft-target/files/coverage keys. The soft target is DERIVED from max_commits (no new key), so it
        needs no change."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── docs/
    ├── how-it-works.md   # EDIT TARGET — 2 edits in the decompose section (line 59 table cell; new paragraph after line 115)
    ├── cli.md            # READ-ONLY (do NOT touch — contract)
    ├── configuration.md  # READ-ONLY (do NOT touch — contract)
    └── providers.md      # unaffected (no planner-rules narrative)
# (no code, no tests touched — docs-only)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    docs/how-it-works.md   # planner output cell shows files; new "Mode-conditional planner rules" paragraph (FR-M3/M3b/M4)
```

| Path | Action | Responsibility |
|---|---|---|
| `docs/how-it-works.md` | MODIFY | 2 edits: update the table planner output cell (`commits:[...]` → `commits:[{title,description,files}]`); add one "Mode-conditional planner rules" paragraph (mode-conditional rules + soft target + files partition + coverage check). |

**Explicitly NOT touched**: `docs/cli.md`, `docs/configuration.md`, `docs/providers.md` (contract:
no new flags/keys; unaffected), any Go source / tests (S1/T1/T3 own the code; this is the Mode A doc
ride), `README.md` (P3.M1.T1.S1 owns the README changeset sync), `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```markdown
<!-- CRITICAL (G1 — only the OUTPUT cell changes; the Job cell and the other rows stay): the item scopes
     the table edit to the planner OUTPUT cell ("Keep it tight (one paragraph + the table cell)"). Do NOT
     edit the planner Job cell ("decide how many commits and what each covers") — even though forced-count
     mode skips the count decision, the Job cell is a fair summary of the planner's role across modes and
     is OUT OF SCOPE. Do NOT touch the stager/message/arbiter rows. -->

<!-- CRITICAL (G2 — the soft target NEVER errors; only the hard cap does): FR-M4 is explicit — the soft
     target (max_commits/2, default 6) is "guidance, not enforcement — it never errors; only the hard cap
     does." The new paragraph MUST say the soft target is guidance/never-errors and that only the hard cap
     (max_commits, default 12) errors. Do NOT write "the soft target caps the count" or imply enforcement. -->

<!-- CRITICAL (G3 — forced-count OMITS the soft target): PRD §17.5 forced-count rules block has NO
     soft-target line; S1's builder emits forced rules (no soft target) when forcedCount>0. The paragraph
     MUST state forced-count "treats the count as fixed and omits/drops the soft target" — do NOT imply
     forced-count also has a soft target. -->

<!-- CRITICAL (G4 — write PRESENT tense; S1 IS the behavior): Mode A rides with the work. By the time this
     edit lands, the mode-conditional prompt + files + coverage check ARE the live behavior. Do NOT write
     "will lean" / "once S1 lands" / "planned" — write "leans" / "carries" / "logs". Past/conditional
     wording re-introduces drift. -->

<!-- GOTCHA (G5 — em-dashes are OK in THIS doc; the ASCII rule is for Go consts only): docs/how-it-works.md
     uses real em-dashes (—) throughout its prose (e.g. line 113, line 117). The new paragraph MAY use
     em-dashes to match the voice. ⚠️ This is DIFFERENT from P2.M1.T2.S1's ASCII rule — that rule applies
     ONLY to the Go prompt CONSTS in internal/prompt/planner.go (the prompt BYTES must be ASCII). Do NOT
     substitute " -- " in the doc paragraph. -->

<!-- GOTCHA (G6 — no raw Go primitive names in the prose): the doc states USER-VISIBLE behavior, not
     implementation. Do NOT name DiffTreeNames, VerboseRawOutput, fmt.Sprintf, plannerAutoRules, or
     OverlayTreePaths. Use plain English: "a deterministic coverage check logs ... any changed path no
     concept claimed". Mechanics live in the PRD/code. (The neighboring bullets follow this — they say
     `write-tree`, `commit-tree`, `T_start`, not Go identifiers.) -->

<!-- GOTCHA (G7 — the new paragraph is ONE paragraph, bold-led, matching the neighbors): every "Key design
     point" is a single `**Title.** <prose>` paragraph. Do NOT split into sub-bullets or multiple
     paragraphs — the item explicitly says "Keep it tight (one paragraph + the table cell)". Place it
     AFTER "One-file short-circuit" (line 115) and BEFORE "Arbiter leftover reconciliation" (line 117). -->

<!-- GOTCHA (G8 — leave the accurate passages alone): the four-roles table rows other than the planner
     output cell; lines 101–113 (Overlapped / Stage-while-editing / Frozen tree snapshots / Tree-to-tree
     diffs / Serialized publication / Start-of-run freeze / Freeze enforcement); line 115 (One-file
     short-circuit); line 117 (Arbiter leftover reconciliation — already freeze-correct from P1.M1.T2.S2);
     the Safety bullets; the lock section — none are stale. Editing them is scope creep. Only the two
     spots in §findings §1/§2 are in scope. -->

<!-- GOTCHA (G9 — scope fence: cli.md + configuration.md are READ-ONLY): the contract explicitly forbids
     touching them. FR-M3/M3b/M4 add no flags and no config keys (the soft target is DERIVED from the
     existing max_commits), so those files need no change. `git diff --stat` must show ONLY
     docs/how-it-works.md. -->
```

## Implementation Blueprint

### Data models and structure

None. This is a prose edit to one Markdown file. The "model" is the two current→target passages below.

### The two edits (exact — current → target)

**Edit 1 — update the four-roles table planner OUTPUT cell (line 59).**

Current row (line 59):
```markdown
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[...], message?}` |
```

Target row (line 59) — ONLY the OUTPUT (4th) cell changes:
```markdown
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[{title,description,files}], message?}` |
```

**Edit 2 — add the new paragraph after "One-file short-circuit" (line 115), before "Arbiter leftover
reconciliation" (line 117).**

Insert this exact paragraph (with one blank line before and one blank line after, matching the
inter-bullet spacing in the section):

```markdown
**Mode-conditional planner rules.** The planner's `Rules:` block is mode-conditional. In auto-decompose (the default) it leans toward splitting unrelated changes — *lean toward SEVERAL* — tempered by a soft target of `max_commits / 2` (default 6) so an ordinary mixed tree lands at or below it rather than fanning into micro-commits; only the hard cap (`max_commits`, default 12) ever errors. Forced-count (`--commits N`) treats the count as fixed and omits the soft target. Every concept carries a `files` list naming each path it touches — a single file split across two concepts is named in both, with the description saying which part belongs where — so each stager knows where to look. After the planner returns, a deterministic coverage check logs (but never errors on) any changed path no concept claimed; the arbiter reconciles those leftovers.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md — Edit 1 (the table planner output cell, line 59)
  - FILE: docs/how-it-works.md
  - LOCATE the four-roles table row starting "| **planner** | bare | Analyze the full working-tree diff; ...".
  - REPLACE the OUTPUT cell `JSON `{count, single, commits:[...], message?}`` with
    `JSON `{count, single, commits:[{title,description,files}], message?}`` (keep the Job cell and the
    pipe structure byte-identical; the other three rows are untouched).
  - VERIFY after: `grep -n 'commits:\[\.\.\.\]' docs/how-it-works.md` → NO output (old shorthand gone);
    `grep -n 'commits:\[{title,description,files}\]' docs/how-it-works.md` → exactly 1 hit (line 59).
  - DO NOT: edit the planner Job cell, the stager/message/arbiter rows, or any other table.

Task 2: EDIT docs/how-it-works.md — Edit 2 (insert the new paragraph after line 115)
  - LOCATE the "One-file short-circuit." bullet (line 115). The new paragraph goes AFTER it (and its
    trailing blank line) and BEFORE the "Arbiter leftover reconciliation." bullet (line 117).
  - INSERT the Edit-2 target paragraph (bold lead "**Mode-conditional planner rules.**") with one blank
    line before and one blank line after (matching the section's inter-bullet spacing).
  - VERIFY after: `grep -n 'Mode-conditional planner rules' docs/how-it-works.md` → exactly 1 hit;
    `grep -n 'soft target' docs/how-it-works.md` → exactly 1 hit;
    `grep -n 'coverage check' docs/how-it-works.md` → exactly 1 hit;
    `grep -n 'lean toward SEVERAL' docs/how-it-works.md` → exactly 1 hit.
  - DO NOT: alter the One-file short-circuit bullet (line 115) or the Arbiter bullet (line 117); do NOT
    split the new paragraph into sub-bullets; do NOT cite Go primitive names or FR numbers in the prose.

Task 3: VALIDATE (editorial + scope — no compile/test for docs)
  - RUN: grep -n 'commits:\[\.\.\.\]' docs/how-it-works.md                       → expect NO output
  - RUN: grep -n 'commits:\[{title,description,files}\]' docs/how-it-works.md    → expect 1 hit (line 59)
  - RUN: grep -n 'Mode-conditional planner rules' docs/how-it-works.md           → expect 1 hit
  - RUN: grep -n 'soft target' docs/how-it-works.md                              → expect 1 hit
  - RUN: grep -n 'coverage check' docs/how-it-works.md                           → expect 1 hit
  - RUN: grep -n 'lean toward SEVERAL' docs/how-it-works.md                      → expect 1 hit
  - RUN: grep -n 'max_commits / 2' docs/how-it-works.md                          → expect 1 hit
  - RUN: git diff --stat -- docs/                                                → expect ONLY docs/how-it-works.md
  - RUN: git diff --stat -- docs/cli.md docs/configuration.md                    → expect EMPTY
  - RUN: git diff --stat -- internal/ pkg/ cmd/ README.md                        → expect EMPTY
  - READ-THROUGH: the decompose section (lines 55–120) end-to-end once — confirm the table cell, the
    One-file short-circuit bullet, the new paragraph, and the Arbiter bullet are mutually consistent
    (planner assigns files + count guidance → stager stages per concept → coverage check flags unclaimed
    → arbiter reconciles leftovers).
```

### Implementation Patterns & Key Details

```markdown
<!-- === Why the paragraph leads with "mode-conditional" (the rules block is the structural news) ===
     The three behaviors group naturally: the mode-conditional rules block (auto vs forced + soft target)
     is the lead concept; the per-concept `files` list is the partition contract that flows from planner
     to stager; the coverage check is the diagnostic that closes the loop to the arbiter. Leading with the
     rules block matches the PRD §17.5 framing ("only the Rules: block changes") and sets up the soft
     target (the user-visible count guidance) before the mechanical files/coverage details. -->

<!-- === Why the soft target is stated as guidance, never an error (FR-M4) ===
     The user-facing fact is "decompose won't fan a tree into a dozen micro-commits." That follows from
     the soft target (max_commits/2, default 6) being GUIDANCE, with only the hard cap (max_commits,
     default 12) ever erroring. Stating both numbers + "only the hard cap ever errors" prevents the
     misread that the soft target is enforced. -->

<!-- === Why forced-count "omits the soft target" (not "lowers it") ===
     --commits N means the count is SETTLED by the user; the soft target would be contradictory guidance
     (you can't both "fix the count at N" and "aim for max_commits/2"). S1's builder literally emits the
     forced rules block (no soft-target line). The doc says "omits/drops the soft target" to match. -->

<!-- === Why `files` is named per-concept and a split file is "named in both" (FR-M3) ===
     The files list's real job is telling each concept's stager WHERE to look (FR-M5). A single file split
     across two concepts is the disambiguation case the PRD calls out ("naming it in both and saying which
     part belongs where"). Stating it tells the reader why a path can appear in two concepts' files lists. -->

<!-- === Voice match (how-it-works.md vs PRD §17.5) ===
     how-it-works.md is plainer and shorter than the PRD. It uses `max_commits`, `--commits N`, `files`,
     `Rules:` inline but NOT DiffTreeNames/VerboseRawOutput/fmt.Sprintf. The new paragraph mirrors that:
     states the soft target + hard cap + mode-conditional rules + files partition + coverage check WITHOUT
     the builder mechanics or FR citations (the neighbors cite FRs occasionally, but the item says "keep
     it tight" — the three behaviors are self-explanatory). -->
```

### Integration Points

```yaml
DOCS (docs/how-it-works.md):
  - line 59:    four-roles table planner OUTPUT cell `commits:[...]` → `commits:[{title,description,files}]`
  - after 115:  NEW "**Mode-conditional planner rules.**" paragraph (mode-conditional rules + soft target
                + files partition + coverage check), placed between "One-file short-circuit" and
                "Arbiter leftover reconciliation"

NO-TOUCH (explicitly — contract or sibling ownership):
  - docs/cli.md, docs/configuration.md, docs/providers.md   # contract: no new flags/keys; unaffected
  - README.md                                                # P3.M1.T1.S1 (changeset sync)
  - internal/*, pkg/*, cmd/* (any .go)                       # S1/T1/T3 own the code; this is docs-only
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

CROSS-DOC CONSISTENCY (informational — verify, do not edit):
  - PRD §17.5 / FR-M3/M3b/M4 are the authoritative source the new paragraph mirrors (plain-English condensation).
  - docs/cli.md's --max-commits (lines 36, 399) and docs/configuration.md's [generation].max_commits
    (lines 209, 217, default 12) are unchanged by FR-M3/M3b/M4 (no new flags/keys; the soft target is
    derived from the existing max_commits).

DOWNSTREAM HOOKS (informational):
  - P3.M1.T1.S2 (holistic how-it-works.md reconciliation) will review the whole decompose section later;
    this edit's two spots must be consistent and accurate by then (the edit makes them so).
```

## Validation Loop

> Docs-only edit — no compile/test/lint gates apply. Validation is editorial (the old shorthand is gone,
> the new phrasing is present, the section is internally consistent) + scope (`git diff --stat`).

### Level 1: Markdown Sanity (no broken structure)

```bash
cd /home/dustin/projects/stagecoach

# The edits are one table-cell swap + one new bold-led paragraph. Confirm no accidental heading/structure
# breakage and the four-roles table still has 4 data rows:
grep -n '^#' docs/how-it-works.md | head   # heading list unchanged (no new/removed headings)
# Expected: the same heading sequence as before the edit.

grep -nc '^| \*\*' docs/how-it-works.md    # count bold-led table rows in the four-roles table region
sed -n '57,61p' docs/how-it-works.md       # the four-roles table — eyeball 4 rows, planner output cell updated
# Expected: the planner row shows commits:[{title,description,files}]; stager/message/arbiter rows unchanged.
```

### Level 2: Content Assertions (the new contract + the three behaviors are present)

```bash
cd /home/dustin/projects/stagecoach

# THE headline check 1: the old output-cell shorthand is GONE.
grep -n 'commits:\[\.\.\.\]' docs/how-it-works.md
# Expected: NO output. (Before the edit this returned exactly one hit at line 59.)

# The new output-cell contract is present.
grep -n 'commits:\[{title,description,files}\]' docs/how-it-works.md
# Expected: exactly 1 hit (line 59).

# The new paragraph and its three behaviors are present.
grep -n 'Mode-conditional planner rules' docs/how-it-works.md   # Expected: exactly 1 hit
grep -n 'soft target' docs/how-it-works.md                      # Expected: exactly 1 hit
grep -n 'coverage check' docs/how-it-works.md                   # Expected: exactly 1 hit
grep -n 'lean toward SEVERAL' docs/how-it-works.md              # Expected: exactly 1 hit
grep -n 'max_commits / 2' docs/how-it-works.md                  # Expected: exactly 1 hit

# The soft-target-is-guidance fact is stated (only the hard cap errors).
grep -n 'only the hard cap' docs/how-it-works.md                # Expected: exactly 1 hit
```

### Level 3: Scope Discipline (only the one doc changed)

```bash
cd /home/dustin/projects/stagecoach

# ONLY docs/how-it-works.md changed.
git diff --stat -- docs/
# Expected: only docs/how-it-works.md.

# The contract-forbidden docs are untouched.
git diff --stat -- docs/cli.md docs/configuration.md
# Expected: EMPTY.

# No code, no README, no other docs touched.
git diff --stat -- internal/ pkg/ cmd/ README.md docs/providers.md
# Expected: EMPTY.

# Confirm the diff is exactly the 2 intended edits (eyeball the patch).
git diff -- docs/how-it-works.md
# Expected: two changed hunks — (1) the table output cell on line 59, (2) the inserted
# "Mode-conditional planner rules" paragraph between line 115 and line 117. Nothing else.
```

### Level 4: Cross-Reference Consistency (the section tells one story)

```bash
cd /home/dustin/projects/stagecoach

# Read the decompose section end-to-end and confirm the planner→stager→arbiter story is consistent:
sed -n '55,120p' docs/how-it-works.md
# Expected (eyeball): the four-roles table planner cell now shows `files`; the "Mode-conditional planner
# rules" paragraph states the soft target (guidance, default 6) + hard cap (default 12, the only error) +
# forced-count fixing the count + per-concept files + coverage-check-logs-not-errors → arbiter; the
# One-file short-circuit and Arbiter leftover reconciliation bullets are unchanged and consistent.

# Cross-check the soft-target numbers against the PRD's authoritative framing (do NOT edit the PRD):
grep -n 'soft target\|max_commits / 2\|default 6\|hard cap' PRD.md | head
# Expected: PRD §9.14 FR-M4 uses the same soft-target/hard-cap framing the doc now mirrors.
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep 'commits:\[\.\.\.\]' docs/how-it-works.md` → no output (Level 2).
- [ ] `grep 'commits:\[{title,description,files}\]' docs/how-it-works.md` → 1 hit (line 59).
- [ ] `grep 'Mode-conditional planner rules' docs/how-it-works.md` → 1 hit.
- [ ] `grep 'soft target' docs/how-it-works.md` → 1 hit.
- [ ] `grep 'coverage check' docs/how-it-works.md` → 1 hit.
- [ ] `grep 'only the hard cap' docs/how-it-works.md` → 1 hit (the never-errors fact is stated).
- [ ] Markdown structure intact (4 table rows; heading list unchanged) — Level 1 eyeball.

### Feature Validation
- [ ] The four-roles table planner OUTPUT cell shows `commits:[{title,description,files}]`.
- [ ] The new "Mode-conditional planner rules" paragraph is ONE bold-led paragraph between
      "One-file short-circuit" and "Arbiter leftover reconciliation".
- [ ] The paragraph names: mode-conditional `Rules:` block; auto leans toward SEVERAL; soft
      `max_commits / 2` target (default 6); only the hard cap `max_commits` (default 12) errors;
      forced-count `--commits N` fixes the count and omits the soft target; per-concept `files`; a split
      file named in both; coverage check logs-not-errors; arbiter reconciles leftovers.
- [ ] The narrative is in PRESENT tense (no "will"/"once S1 lands" — the behavior is live).

### Scope Discipline Validation
- [ ] `git diff --stat -- docs/` shows ONLY `docs/how-it-works.md`.
- [ ] `git diff --stat -- docs/cli.md docs/configuration.md` is EMPTY (contract: do not touch).
- [ ] `git diff --stat -- internal/ pkg/ cmd/ README.md` is EMPTY (no code/README).
- [ ] The diff is exactly 2 hunks (the table cell + the inserted paragraph).
- [ ] Did NOT edit the planner Job cell, the stager/message/arbiter rows, lines 101–113, line 115,
      line 117, the Safety bullets, or the lock section.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation (editorial)
- [ ] The new paragraph matches the existing voice (concise bold-led single paragraph; moderate inline
      code symbols; real em-dashes — NOT the Go-const " -- " substitution).
- [ ] No raw Go primitive names (`DiffTreeNames`, `VerboseRawOutput`, `fmt.Sprintf`, `plannerAutoRules`)
      in the prose — only user-visible behavior.
- [ ] The four-roles table pipe structure and the other three rows are byte-identical.

---

## Anti-Patterns to Avoid

- ❌ Don't edit the planner Job cell, the stager/message/arbiter rows, or any accurate passage — the item
  scopes the table change to the planner OUTPUT cell only ("one paragraph + the table cell") (gotcha G1/G8).
- ❌ Don't imply the soft target errors or is enforced — FR-M4 is explicit that only the hard cap
  (`max_commits`, default 12) errors; the soft target (`max_commits/2`, default 6) is guidance (G2).
- ❌ Don't imply forced-count carries a soft target — the forced `Rules:` block OMITS it; say "omits/drops
  the soft target" (G3).
- ❌ Don't write in future/conditional tense ("will lean", "once S1 lands"). Mode A rides with the work —
  the behavior IS live when this lands. Present tense only (G4).
- ❌ Don't substitute " -- " for the em-dash in the doc paragraph — that ASCII rule is for the Go PROMPT
  CONSTS (P2.M1.T2.S1) only, NOT this Markdown doc. Use real em-dashes (—) to match the doc's voice (G5).
- ❌ Don't cite raw Go primitive names (`DiffTreeNames`, `VerboseRawOutput`, `OverlayTreePaths`) or heavy
  FR numbering in the prose — the doc states user-visible behavior; mechanics live in the PRD/code (G6).
- ❌ Don't split the new paragraph into sub-bullets or multiple paragraphs — the item says "Keep it tight
  (one paragraph)"; match the bold-led single-paragraph neighbors (G7).
- ❌ Don't touch `docs/cli.md` or `docs/configuration.md` — the contract explicitly forbids it; FR-M3/M3b/M4
  add no flags/keys (the soft target is derived from the existing `max_commits`) (G9).
- ❌ Don't edit code, tests, `README.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`. This is
  a single-file docs edit.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed two-edit prose change to one Markdown file. Both edits are
quoted verbatim (current → target, ready to paste), the line numbers are verified against the live tree
(table row line 59; "One-file short-circuit" line 115; "Arbiter leftover reconciliation" line 117), and
the architecture scout brief (§2.5) independently prescribes exactly these two edits with the same
target table cell and the same three required paragraph points. The behavior contract (FR-M3/M3b/M4 +
§17.5) is exceptionally well-specified in both the PRD and the parallel S1 PRP, so the target phrasing is
unambiguous. The three most likely mistakes — implying the soft target errors (G2), implying forced-count
keeps a soft target (G3), and writing in future tense (G4) — are front-loaded as CRITICAL gotchas and
caught by the deterministic `grep` gates (Level 2: "only the hard cap" present; "soft target" present;
present-tense wording). The one residual uncertainty (not 10/10) is whether to cite FR numbers in the new
paragraph — the item says "keep it tight" and the chosen wording omits them, which a reviewer could
question; this is cosmetic and easily adjusted. Scope is cleanly fenced (`git diff --stat` proves only
`docs/how-it-works.md`; `cli.md`/`configuration.md` explicitly read-only; grep confirms neither mentions
soft-target/files/coverage). No code, no tests, no toolchain — the gates are editorial greps and a scope
check, all deterministic.
