---
name: "P1.M6.T1.S1 (bugfix Issue 1 doc-sync) — Update README.md and docs/cli.md to scope --reasoning and note activation (verify providers.md; guard against --thinking-effort)"
description: |

  Documentation-only changeset-level sync (Mode B). The FR-R6 reasoning feature (`--reasoning`,
  `--<role>-reasoning`, the shipped `planner = high` default, `[role.*] reasoning`, the `STAGECOACH_*`
  env knobs) was made FUNCTIONAL for **pi** (`--thinking`) and **claude** (`--effort`) by the now-complete
  implementing subtasks (P1.M1.T1.S1/S2 populate `ReasoningLevels`; P1.M2.* resolve the message role on
  the single path; P1.M3.* index-sync; P1.M4.* bootstrap header env vars). Before them every provider's
  `ReasoningLevels` was `nil` and the feature was completely inert — which is what README.md and
  docs/cli.md were (inertly) advertising. This task makes the docs match the now-working reality and
  scopes the `--reasoning` claim honestly per the PRD Issue 1 Suggested Fix: "Update the README so
  --reasoning high only promises an effect where it has one."

  CONTRACT (item_description §1–§5, verbatim):
    1. RESEARCH: "README.md:121-122 advertises `stagecoach --reasoning high` ('Use reasoning for deeper
       analysis on the planner'). docs/cli.md:43 documents `--reasoning <level>` with default '(off;
       planner: high)'. Before this changeset, these claims were inert for every provider. After
       P1.M1.T1.S1/S2, reasoning now emits real tokens for claude (--effort) and pi (--thinking)."
    2. INPUT: the completed implementing subtasks + docs/providers.md (already Mode-A-updated) + the
       verified tokens from architecture/external_deps.md (pi=`--thinking`, claude=`--effort` — NOT the
       PRD's wrong `--thinking-effort` guess).
    3. LOGIC:
       (A) README.md:121-122 — KEEP the `stagecoach --reasoning high` example; ADD a note that the effect
           is provider-dependent (pi: --thinking, claude: --effort; other providers: graceful no-op).
           Do NOT remove the example.
       (B) docs/cli.md:43 — the `--reasoning` table row + default '(off; planner: high)' are now
           accurate (planner=high now has effect on pi/claude); optionally add a provider-dependent note.
       (C) docs/providers.md — ensure the provider table / reasoning_levels description is CONSISTENT
           with the Mode A updates from P1.M1.T1.S1/S2 (claude --effort, pi --thinking; others nil).
       (D) Verify no doc claims --thinking-effort (the PRD's wrong guess) — use --effort for claude.
    4. OUTPUT: README.md, docs/cli.md, and docs/providers.md coherently describe the now-functional
       reasoning feature with correct provider-scoped claims. No stale or incorrect flag names remain.
    5. DOCS: this IS the changeset-level documentation task (Mode B).

  DELIVERABLES (2 files MODIFIED for certain; docs/providers.md VERIFY-ONLY — expected NO change):
    1. MODIFY README.md   — insert ONE `> [!NOTE]` provider-dependence callout after the multi-commit
       decomposition code block (lines 121-122 example kept verbatim). ~3-4 new lines.
    2. MODIFY docs/cli.md — append a concise provider-dependent qualifier to the `--reasoning` table-row
       Description cell (line 43, keep the '(off; planner: high)' default cell) AND optionally a 1-line
       inline note in the Examples block (lines 212-213). ~1-2 cells/lines touched.
    3. VERIFY docs/providers.md — re-read line 35 (reasoning_levels schema row) + line 59 (render
       paragraph); confirm both already state pi=`--thinking` / claude=`--effort` / others-nil (they do,
       per Mode A). Leave byte-unchanged UNLESS an inconsistency is found (none expected).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/** (all Go source) — ZERO `.go` files touched by this task.
    - providers/*.toml — shipped reference manifests; already carry [reasoning_levels] from P1.M1.T1.S1/S2.
    - PRD.md, **/prd_snapshot.md, **/tasks.json — human/orchestrator-owned (read-only).
    - docs/how-it-works.md and any other docs/* file — out of scope (this changeset = README + cli + providers).
    - internal/config/bootstrap.go (the P1.M4.T1.S1 header env-var change) — that is SOURCE, not prose;
      its effect on the generated config header is not part of user-facing README/cli prose.

  SUCCESS: README.md and docs/cli.md describe `--reasoning` as provider-dependent (pi=`--thinking`,
  claude=`--effort`, others graceful no-op) while keeping the runnable `stagecoach --reasoning high`
  example; the `(off; planner: high)` default is retained and now accurate; docs/providers.md remains
  consistent; `grep -rn "thinking-effort"` over README.md/docs/providers/ stays empty;
  `npx markdownlint-cli2 README.md docs/cli.md docs/providers.md` reports 0 errors; `go build ./...`
  stays green and byte-identical (no source touched); `git status` shows README.md + docs/cli.md
  (docs/providers.md only if an inconsistency forced a fix — expected none).

---

## Goal

**Feature Goal**: Close the documentation half of PRD Issue 1. The FR-R6 reasoning feature is now
functional for pi/claude (P1.M1.T1.S1/S2 + P1.M2.* are complete), but README.md and docs/cli.md still
present `--reasoning` without telling the user the effect is provider-dependent — and were, before the
fixes, advertising an inert feature. Make the docs coherently describe the now-working, provider-scoped
reasoning feature, per the Issue 1 Suggested Fix: *"`--reasoning high` only promises an effect where it
has one."*

**Deliverable** (2 modified files; 1 verify-only file; NO source, NO tests):
1. `README.md` — one new `> [!NOTE]` provider-dependence callout inserted after the Multi-commit
   decomposition code block (the lines 121–122 `stagecoach --reasoning high` example stays verbatim).
2. `docs/cli.md` — the `--reasoning` Global-flags table-row Description cell (line 43) gets a concise
   provider-dependent qualifier; the default cell `"" (off; planner: high)` is kept; optionally a 1-line
   note in the Examples block (lines 212–213).
3. `docs/providers.md` — VERIFY-ONLY: confirm line 35 (`reasoning_levels` schema row) and line 59
   (render paragraph) already state pi=`--thinking` / claude=`--effort` / others-nil (they do, via Mode
   A). Leave byte-unchanged unless an inconsistency is found.

**Success Definition**:
- README.md keeps the runnable `stagecoach --reasoning high` example and adds a note that the effect is
  provider-dependent (pi: `--thinking`, claude: `--effort`; other providers: graceful no-op).
- docs/cli.md's `--reasoning` row retains the `"" (off; planner: high)` default (now accurate) and adds
  a provider-dependent qualifier to the Description cell.
- docs/providers.md remains consistent with the verified tokens (no edit expected).
- `grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/` returns NOTHING (the PRD's
  wrong `--thinking-effort` guess never appears; claude uses `--effort`).
- `npx markdownlint-cli2 README.md docs/cli.md docs/providers.md` → 0 errors (repo `.markdownlint.json`
  config: MD013/MD033/MD060 off, default on).
- `go build ./...` stays GREEN and byte-identical (no `.go` file touched).
- `git status --porcelain` shows README.md + docs/cli.md (docs/providers.md only if a real
  inconsistency forced a fix — expected none).

## User Persona

**Target User**: a Stagecoach user (PRD §7 personas) who reads the README/CLI docs to learn what
`--reasoning high` does. Before the implementing subtasks, the flag was inert and the docs oversold it;
the user would set it, see no behavior change, and lose trust. After this doc sync, the user knows
exactly when the flag engages (pi/claude) and that it is an honest no-op (not an error) elsewhere.

**Use Case**: user runs `stagecoach --reasoning high` (or relies on the shipped `planner = high`
default) with pi or claude → deeper reasoning actually engages; with another provider → it silently
does nothing (FR-R6 graceful no-op), and the docs now say so.

**User Journey**: user reads the README multi-commit section → sees the `--reasoning high` example +
a `> [!NOTE]` scoping it to pi/claude → knows the flag is real for their provider (or an honest no-op)
→ consults docs/cli.md for the exact table row → the Description cell confirms the provider scope and
points at the FR-R6 no-op policy.

**Pain Points Addressed**: the docs previously either oversold an inert feature (pre-fix) or, post-fix,
failed to scope it honestly. This task makes the promise match the behavior: an effect where there is
one, an honest no-op where there isn't.

## Why

- **It IS the doc half of Issue 1.** The PRD Issue 1 Suggested Fix's last sentence is explicitly:
  "Update the README so `--reasoning high` only promises an effect where it has one." This task is that
  sentence, plus the cli.md sibling and a providers.md consistency check.
- **Closes the changeset documentation (Mode B).** P1.M1.T1.S1/S2 did the *code* + a one-line Mode A
  note in providers.md; P1.M6.T1.S1 is the *user-facing prose* sync (README + cli) that rides with the
  changeset. Without it, the README still presents `--reasoning` as universally effective (or, pre-fix,
  as effective at all) — misleading either way.
- **Removes a latent maintenance trap.** The PRD guessed `--thinking-effort` for claude; the verified
  flag is `--effort`. The docs already use `--effort`; this task adds a grep guard so a future edit
  never reintroduces the wrong name.
- **Cheap, isolated, safe.** Pure prose to two files + a verify-only third. No code, no tests, no
  schema, no precedence. Zero overlap with the parallel P1.M5.T1.S1 (which edits `.go` files only).

## What

Three documentation touchpoints, scoped exactly to the contract (A)/(B)/(C)/(D):

1. **README.md (A)** — after the Multi-commit decomposition `bash` code block (which contains the
   `stagecoach --reasoning high` example at lines 121–122), insert a `> [!NOTE]` callout stating the
   effect is provider-dependent: engages for **pi** (`--thinking`) and **claude** (`--effort`); other
   providers treat it as a graceful no-op (no error). The lines 121–122 example stays VERBATIM.
2. **docs/cli.md (B)** — in the Global flags table, the `--reasoning` row (line 43): KEEP the default
   cell `"" (off; planner: high)` (now accurate — `planner = high` emits real tokens for pi/claude) and
   APPEND a concise provider-dependent qualifier to the Description cell. Optionally add a 1-line inline
   note to the Examples block `# Use reasoning for deeper analysis` (lines 212–213).
3. **docs/providers.md (C)** — VERIFY-ONLY: re-read line 35 (`reasoning_levels` schema row) and line 59
   (render paragraph); confirm they already state pi=`--thinking` / claude=`--effort` / others-nil. They
   do (Mode A). Leave byte-unchanged unless a genuine inconsistency exists.
4. **Anti-guard (D)** — after editing, assert `grep -rn "thinking-effort\|thinking_effort"` over
   `README.md docs/ providers/` is empty so the PRD's wrong `--thinking-effort` guess never reappears.

### Success Criteria

- [ ] README.md keeps the `stagecoach --reasoning high` example verbatim (lines 121–122 content preserved)
      and adds a `> [!NOTE]` provider-dependence callout after the multi-commit code block.
- [ ] docs/cli.md `--reasoning` table row keeps the `"" (off; planner: high)` default cell and adds a
      provider-dependent qualifier to the Description cell (pi: `--thinking`, claude: `--effort`; others
      no-op; FR-R6).
- [ ] docs/providers.md line 35 + line 59 confirmed consistent with pi=`--thinking` / claude=`--effort`
      / others-nil (no edit unless an inconsistency is found — none expected).
- [ ] `grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/` → NO matches.
- [ ] `npx markdownlint-cli2 README.md docs/cli.md docs/providers.md` → 0 errors.
- [ ] `go build ./...` GREEN and byte-identical (no `.go` file touched).
- [ ] `git status --porcelain` shows README.md + docs/cli.md (+ docs/providers.md ONLY if a fix was
      forced — expected none).

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer / developer with no prior repo knowledge can implement this from: the exact
two edit sites (README callout placement + cli.md table-row cell, both quoted), the verified token
table (pi=`--thinking`, claude=`--effort`, others nil — quoted), the docs/providers.md verify targets
(line 35 + line 59, quoted), the README's existing `> [!NOTE]` callout idiom to mirror, the markdownlint
gate command, and the `--thinking-effort` anti-guard grep. No Go/registry/git knowledge needed.

### Documentation & References

```yaml
# MUST READ — THE decisive research (verified tokens + current state of all 3 files)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M6T1S1/research/findings.md
  why: §1 the verified token table (pi=--thinking, claude=--effort, others nil; --thinking-effort is
       WRONG); §2 README exact site + the post-code-block callout idiom; §3 cli.md two sites (line 43 row
       + lines 212-213 example) + the leave-alone mapping table; §4 providers.md verify-only (line 35 +
       line 59 already Mode-A-updated); §5 the --thinking-effort anti-guard; §6 validation (markdownlint
       available, no unit tests); §7 scope/parallel-safety.
  critical: §1 (the verified flags — the single source of truth for what the docs must say), §2 (the
            README callout is INSERTED, lines 121-122 stay verbatim), §4 (providers.md is VERIFY-ONLY).

# MUST READ — the verified-token source of truth (architecture)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/architecture/external_deps.md
  why: the VERIFIED flags from live `pi --help` / `claude --help`. pi: `--thinking <level>`
       (off,minimal,low,medium,high,xhigh). claude: `--effort <level>` (low,medium,high) — NOT
       `--thinking-effort` (the PRD Suggested Fix was wrong). The Stagecoach level→token mapping.
  critical: claude uses `--effort` (NEVER `--thinking-effort`). pi uses `--thinking`. off → no tokens.

# EDIT — README.md (insert ONE callout; keep lines 121-122 verbatim)
- file: README.md   (EDIT: +1 `> [!NOTE]` block after the Multi-commit decomposition code block)
  section: the `### Multi-commit decomposition` section. The `bash` code block contains lines 121-122
       (`# Use reasoning for deeper analysis on the planner` / `stagecoach --reasoning high`). The block
       closes with ``` (~line 130), then a blank line, then line ~132 `See [How Stagecoach works…]`.
       INSERT the `> [!NOTE]` callout in that gap (after the closing fence, before the `See` line).
  why: the contract (A) mandates KEEPING the example and ADDING a provider-dependence note. The
       post-code-block `> [!NOTE]` is the README's established idiom (the file already uses `> [!NOTE]`
       at lines 31, 79, 100 to qualify a preceding code block).
  pattern: mirror an existing README `> [!NOTE]` block exactly (blank line before; `> [!NOTE]` on the
       first line; `> ` continuation lines; blank line after).
  gotcha: do NOT edit lines 121-122 themselves (the contract forbids removing the example). do NOT put
       the note INSIDE the `bash` code block (it would become a shell comment / pollute the runnable
       snippet). Put it AFTER the closing fence.

# EDIT — docs/cli.md (the --reasoning table-row Description cell + optional example note)
- file: docs/cli.md   (EDIT: line 43 Description cell; optionally lines 212-213 example)
  section: the `## Global flags` table, row `--reasoning <level>` (line 43). The row is 6 columns:
       `| Flag | Type | Default | Env var | Git config | Description |`. The Default cell is
       `"" (off; planner: high)` — KEEP it (it is now accurate: planner=high emits real tokens for
       pi/claude). The Description cell is `Global reasoning effort: off|low|medium|high` — APPEND the
       provider-dependent qualifier there. The Examples block (line ~212) has
       `# Use reasoning for deeper analysis` / `stagecoach --reasoning high` — optionally a 1-line note.
  why: the contract (B) keeps the default (now accurate) and "optionally" adds the provider-dependent
       note. The Description cell is the right home for the qualifier.
  pattern: keep the row's 6-column `|` structure EXACTLY (6 pipes per row). Do not add/remove columns.
  gotcha: the `off|low|medium|high` uses literal `|` inside the Description cell — those are CONTENT
       pipes inside a table cell and must remain literal (they render as text, not column separators,
       only because the cell is the LAST column; verify markdownlint stays clean). When you APPEND the
       qualifier, keep it in the same final cell. Do NOT touch the Flag↔env↔git-config map table
       (lines 164-170) — it has no Description column and nothing to fix.

# VERIFY-ONLY — docs/providers.md (expected NO change)
- file: docs/providers.md   (VERIFY: line 35 + line 59; edit ONLY if inconsistent)
  section: line 35 = the `reasoning_levels` row of the 19-field schema table (already says "pi populates
       high/medium/low via `--thinking` (verified `pi --help`); claude via `--effort` (verified `claude
       --help`); all other built-ins are nil (graceful no-op)"). line 59 = the "Command rendering"
       paragraph (already says tokens append after the model flag when present, else silent no-op).
  why: contract (C) is "ensure consistent". Mode A (P1.M1.T1.S1/S2) ALREADY updated line 35. It is
       consistent with the verified tokens. Verify; do not churn.
  pattern: if (and only if) an inconsistency is found, fix ONLY that cell/sentence. Otherwise leave the
       file byte-unchanged (a docs file that already matches should not be edited "just in case").
  gotcha: an edit to providers.md is NOT expected and NOT required for success. The success gate is
       "consistent", not "changed". If you change it without cause, you add risk for no benefit.

# READ ONLY — the parallel subtask (ZERO overlap; safe to run concurrently)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M5T1S1/PRP.md
  why: confirms the in-flight P1.M5.T1.S1 edits ONLY `internal/config/{file,migrate,file_test}.go` —
       pure Go, zero doc files. No merge conflict possible with this task; this task edits no `.go` so
       it cannot perturb P1.M5.T1.S1's `go test` gate.
  gotcha: do NOT duplicate P1.M5.T1.S1's scope; do NOT touch any `internal/config/*.go` file.

# PROJECT CONFIG — the markdownlint gate
- file: .markdownlint.json   (READ: the lint config the gate enforces)
  why: `{"default":true,"MD013":false,"MD033":false,"MD060":false}` — line-length, inline-HTML, and
       nested-list rules OFF; everything else ON. Your edits must satisfy MD001 (heading increments),
       MD009 (trailing whitespace), MD012 (multiple blank lines), MD031 (fenced blocks need surrounding
       blank lines), MD040 (fenced code needs a language), MD047 (file ends with single newline), etc.
  gotcha: the README note callout and the cli.md table row must keep surrounding blank lines (MD031) and
       no trailing spaces (MD009). A `> [!NOTE]` block is fine (GitHub-style alert; markdownlint does not
       flag it under this config).
```

### Current Codebase tree (relevant slice)

```bash
# The 3 doc files in scope (no .go, no toml):
README.md            # EDIT: +1 `> [!NOTE]` callout after the multi-commit code block (lines 121-122 kept).
docs/cli.md          # EDIT: --reasoning table-row Description cell (line 43) + optional example note (212-213).
docs/providers.md    # VERIFY-ONLY: line 35 + line 59 already Mode-A-updated (expected NO change).
# Repo lint config (the gate):
.markdownlint.json   # READ: {default:true, MD013:false, MD033:false, MD060:false}.
# Authoritative token source (read-only):
plan/.../architecture/external_deps.md   # READ: pi=--thinking, claude=--effort, --thinking-effort is WRONG.
# Frozen / out of scope (do NOT touch):
internal/**          # all Go source (parallel P1.M5.T1.S1 owns internal/config/*).
providers/*.toml     # shipped reference manifests (already carry [reasoning_levels] from P1.M1.T1.S1/S2).
docs/how-it-works.md # and all other docs/* — out of scope.
PRD.md, prd_snapshot.md, tasks.json  # human/orchestrator-owned.
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. Expected 2 MODIFIED (providers.md expected UNCHANGED):
README.md            # +1 `> [!NOTE]` provider-dependence callout (lines 121-122 verbatim).
docs/cli.md          # --reasoning row Description cell += provider-dependent qualifier; optional example note.
docs/providers.md    # UNCHANGED (verify-only; edit ONLY if a real inconsistency is found — none expected).
# NO .go changes; NO go.mod/go.sum changes; NO providers/*.toml changes.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (claude flag is --effort, NEVER --thinking-effort): the PRD Issue 1 "Suggested Fix" GUESSED
     `--thinking-effort` for claude — that is WRONG (verified via `claude --help`, see external_deps.md).
     The docs already use `--effort`. Every note you write must say claude uses `--effort`. The §D
     anti-guard grep (`thinking-effort|thinking_effort`) MUST stay empty after your edits. -->

<!-- CRITICAL (KEEP the README example): the contract (A) explicitly forbids removing the
     `stagecoach --reasoning high` example. Edit ONLY by ADDING a `> [!NOTE]` callout AFTER the code
     fence closes — never edit lines 121-122, never move the example, never comment it out. -->

<!-- CRITICAL (KEEP the cli.md default cell): the `"" (off; planner: high)` default is now ACCURATE
     (planner=high emits --thinking/--effort for pi/claude post-fix). Do NOT delete or reword it. The
     edit is to the Description cell only. -->

<!-- GOTCHA (cli.md table is 6 columns): the --reasoning row is `| Flag | Type | Default | Env var |
     Git config | Description |`. When you append the qualifier to the Description cell you must keep
     EXACTLY 6 pipes — do not split the row into 7 columns. markdownlint will catch a malformed table. -->

<!-- GOTCHA (do NOT touch the Flag↔env↔git-config map table): docs/cli.md lines 164-170 is a separate
     3-column mapping table (Flag/Env var/Git config) with NO Description column — there is nothing to
     "scope" there, and editing it is out of contract (B) which names only line 43. Leave it. -->

<!-- GOTCHA (providers.md is VERIFY-ONLY): line 35 was already updated by Mode A of P1.M1.T1.S1/S2 and
     already states pi=--thinking / claude=--effort / others-nil. "Ensure consistent" ≠ "must change".
     Editing a file that already matches adds risk for zero benefit. Only edit if you find an actual
     inconsistency (none expected). -->

<!-- GOTCHA (callout placement, not inside the code block): put the README `> [!NOTE]` AFTER the
     closing ``` fence of the multi-commit bash block, not as a `#` comment inside it. A comment inside
     the runnable snippet pollutes copy-paste output and is NOT a visible doc note. -->

<!-- GOTCHA (markdownlint MD031 / MD009): fenced blocks and blockquotes need a blank line before/after;
     no trailing whitespace on any line. Run `npx markdownlint-cli2` on the 3 files as the gate — it is
     installed (v0.23.0) and configured by .markdownlint.json. -->

<!-- GOTCHA (parallel safety): P1.M5.T1.S1 (in-flight) edits internal/config/{file,migrate,file_test}.go.
     This task edits 0 .go files. There is no overlap and no merge conflict. Do not block on it. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. This is a documentation-only task. The only "structures" are:
  - README.md: one new GitHub-style `> [!NOTE]` callout block (3-4 lines).
  - docs/cli.md: one edited table-row cell + (optional) one edited example comment line.
  - docs/providers.md: zero changes (verify-only).
-->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT README.md — add the provider-dependence NOTE callout (contract A)
  - SITE: the `### Multi-commit decomposition` section. The `bash` code block (containing lines 121-122
      `# Use reasoning for deeper analysis on the planner` / `stagecoach --reasoning high`) closes with
      ``` at ~line 130, followed by a blank line, then `See [How Stagecoach works …` at ~line 132.
  - ACTION: INSERT a `> [!NOTE]` callout in the gap (after the closing fence, before the `See` line),
      blank line before and after (MD031). KEEP lines 121-122 VERBATIM (do not edit the example).
  - CONTENT (suggested wording — match the verified tokens; pi=--thinking, claude=--effort):
      > [!NOTE]
      > `--reasoning` is provider-dependent: it engages deeper reasoning for **pi** (`--thinking`) and
      > **claude** (`--effort`). Other providers treat it as a graceful no-op (no error) per FR-R6. It
      > applies to any role via `--<role>-reasoning` or `[role.*] reasoning`.
  - WHY: contract (A) — keep the example, add the provider-scoped note. The post-code-block `> [!NOTE]`
      is the README's established idiom (used at lines 31/79/100).
  - GOTCHA: never edit lines 121-122; never put the note INSIDE the bash block.

Task 2: EDIT docs/cli.md — the --reasoning table-row Description cell (contract B, primary)
  - SITE: the `## Global flags` table, the `--reasoning <level>` row (line 43), 6 columns.
  - ACTION: KEEP the Default cell `"" (off; planner: high)` (now accurate). APPEND a concise qualifier
      to the Description cell. Keep EXACTLY 6 pipes (do not add a column).
  - CONTENT (suggested — append to the existing `Global reasoning effort: off|low|medium|high`):
      … Provider-dependent: engages for pi (`--thinking`) and claude (`--effort`); other providers are a
      graceful no-op (FR-R6).
  - WHY: contract (B) — default is now accurate; "optionally add a note about provider-dependent
      support" → add it to the Description cell.
  - GOTCHA: the literal `off|low|medium|high` pipes stay (they are content in the final cell). Do not
      touch the Flag↔env↔git-config map table (lines 164-170).

Task 3 (OPTIONAL): EDIT docs/cli.md — the Examples block note (contract B, secondary)
  - SITE: the `## Examples` block, lines 212-213:
      `# Use reasoning for deeper analysis` / `stagecoach --reasoning high`
  - ACTION: optionally tighten the comment to name the provider scope, e.g.:
      `# Use reasoning for deeper analysis (pi: --thinking, claude: --effort; others no-op)`
  - WHY: keeps the Examples block self-consistent with the table-row note (Task 2). OPTIONAL because the
      table row (Task 2) already carries the full qualifier; the contract marks the note as "optional".
  - GOTCHA: keep the `stagecoach --reasoning high` command verbatim. This is a one-line comment edit.

Task 4 (VERIFY-ONLY): docs/providers.md — confirm consistency (contract C)
  - ACTION: RE-READ line 35 (the `reasoning_levels` schema-table row) and line 59 (the "Command
      rendering" paragraph). Confirm they state pi=`--thinking`, claude=`--effort`, other built-ins nil
      → graceful no-op. They ALREADY do (Mode A from P1.M1.T1.S1/S2).
  - DECISION RULE: if consistent → LEAVE the file byte-unchanged (do not churn a matching file). If (and
      only if) you find a genuine inconsistency → fix ONLY that cell/sentence.
  - WHY: contract (C) is "ensure consistent", not "must change". Expected outcome: NO edit.
  - GOTCHA: an edit here is NOT required for success. If `git status` shows providers.md modified, you
      must be able to justify it against a specific inconsistency you found.

Task 5: VERIFY (run all gates; the contract's "TEST: verify by reading for consistency")
  - ANTI-GUARD (contract D): `grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/`
      → MUST be empty (claude uses `--effort`, never `--thinking-effort`).
  - MARKDOWNLINT: `npx markdownlint-cli2 README.md docs/cli.md docs/providers.md` → 0 errors
      (repo `.markdownlint.json` config). If errors, READ them (MD031 missing blank line, MD009 trailing
      whitespace, MD040 missing code language) and fix.
  - CONSISTENCY READ: open README.md (the new callout), docs/cli.md (the edited row), docs/providers.md
      (line 35) — confirm all three say pi=`--thinking`, claude=`--effort`, others no-op, identically.
  - SMOKE CHECK (no source touched): `go build ./...` → GREEN and byte-identical; `git status
      --porcelain` → README.md + docs/cli.md (+ docs/providers.md ONLY if Task 4 found a real issue).
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (README callout — mirror an existing README > [!NOTE] block): -->
<!--   (blank line)                                                        -->
<!--   > [!NOTE]                                                           -->
<!--   > First continuation line.                                          -->
<!--   > Second continuation line.                                         -->
<!--   (blank line)                                                        -->
<!-- Place it AFTER the multi-commit bash fence closes, BEFORE the "See" line. -->

<!-- PATTERN (cli.md table cell edit — keep 6 columns): -->
<!-- | `--reasoning <level>` | string | "" (off; planner: high) | `STAGECOACH_REASONING` | `stagecoach.reasoning` | Global reasoning effort: off\|low\|medium\|high. Provider-dependent: engages for pi (`--thinking`) and claude (`--effort`); other providers are a graceful no-op (FR-R6). | -->
<!--   (the off|low|medium|high pipes are CONTENT; the row still has exactly 6 field pipes) -->

<!-- CRITICAL: claude = `--effort`, NEVER `--thinking-effort` (verified via claude --help; the PRD
     guessed wrong). pi = `--thinking`. The anti-guard grep must stay empty. -->

<!-- CRITICAL: this is DOCUMENTATION-ONLY. No .go file is edited, no test is added/changed, no
     providers/*.toml is touched, go.mod/go.sum are byte-identical. `go build ./...` is a SMOKE check
     that confirms you accidentally touched no source. -->
```

### Integration Points

```yaml
README.md:
  - insert: "> [!NOTE] provider-dependence callout after the Multi-commit decomposition bash code block"
  - preserve: "lines 121-122 (the stagecoach --reasoning high example) VERBATIM"

docs/cli.md:
  - edit: "--reasoning Global-flags table-row Description cell (line 43) += provider-dependent qualifier"
  - preserve: 'Default cell "" (off; planner: high) — now accurate (planner=high emits tokens for pi/claude)'
  - optional: "Examples block comment (lines 212-213) names the provider scope"
  - leave: "Flag↔env↔git-config map table (lines 164-170) — no Description column, nothing to scope"

docs/providers.md:
  - verify: "line 35 (reasoning_levels schema row) + line 59 (render paragraph) already state pi=--thinking / claude=--effort / others-nil"
  - decision: "consistent → leave byte-unchanged; inconsistent → fix ONLY that cell/sentence (none expected)"

FROZEN/LEAVE (do NOT edit):
  - internal/** (all Go source — parallel P1.M5.T1.S1 owns internal/config/*).
  - providers/*.toml (already carry [reasoning_levels] from P1.M1.T1.S1/S2).
  - docs/how-it-works.md and all other docs/* (out of scope).
  - PRD.md, prd_snapshot.md, tasks.json (human/orchestrator-owned).
  - go.mod/go.sum (no source change → no dep change).

GO.MODULE: change NONE (no Go touched).
```

## Validation Loop

### Level 1: Markdown style (the project-configured lint gate)

```bash
# Run markdownlint on the 3 touched/verified files (repo .markdownlint.json config; tool is installed):
npx markdownlint-cli2 README.md docs/cli.md docs/providers.md
# Expected: 0 errors. If errors, READ them and fix:
#   MD031 → add a blank line around the fenced block / blockquote.
#   MD009 → remove trailing whitespace.
#   MD040 → the bash code fence already has a language (unchanged); a new fence must too.
#   MD047 → ensure the file ends with exactly one newline.
#   MD012 → collapse multiple consecutive blank lines to one.
```

### Level 2: Anti-guard + consistency read (the contract's "verify by reading")

```bash
# Contract (D): the PRD's wrong --thinking-effort guess must NEVER appear; claude uses --effort.
grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/
# Expected: NO output (empty). If anything prints, replace it with --effort (claude) / --thinking (pi).

# Cross-file consistency: all three docs must agree on the verified tokens.
grep -n "thinking\|effort\|reasoning" README.md docs/cli.md docs/providers.md
# Expected: pi ↔ `--thinking`, claude ↔ `--effort`, others ↔ graceful no-op, consistently across files.
# Read the edited README callout, the edited cli.md row, and providers.md line 35 by eye — they must
# state the same provider scope in the same words.
```

### Level 3: Source-byte smoke check (no source was touched)

```bash
# Documentation-only task: confirm NO .go file changed and the build is unaffected.
go build ./...
# Expected: GREEN. (No source edited, so this is a smoke check that nothing was accidentally touched.)

git status --porcelain
# Expected: README.md + docs/cli.md (and docs/providers.md ONLY if Task 4 found a real inconsistency —
# expected NONE). No internal/**, no providers/*.toml, no go.mod/go.sum.

git diff --stat
# Expected: 2 files (README.md, docs/cli.md), small line deltas (a few lines each). providers.md absent
# unless justified.
```

### Level 4: (N/A — no creative/runtime validation for a docs task)

```bash
# No server, no DB, no MCP, no integration test. A documentation-only task has no runtime validation.
# Level 2 (consistency read + anti-guard) IS the domain validation for prose.
```

## Final Validation Checklist

### Technical Validation
- [ ] `npx markdownlint-cli2 README.md docs/cli.md docs/providers.md` → 0 errors.
- [ ] `grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/` → empty (contract D).
- [ ] `go build ./...` GREEN and byte-identical (no source touched — smoke check).
- [ ] `git status --porcelain` → README.md + docs/cli.md (+ providers.md ONLY if a real fix was needed).

### Feature Validation
- [ ] README.md keeps the `stagecoach --reasoning high` example verbatim and adds a provider-dependence
      `> [!NOTE]` callout (pi: `--thinking`, claude: `--effort`; others no-op) (contract A).
- [ ] docs/cli.md `--reasoning` row keeps the `"" (off; planner: high)` default and adds the
      provider-dependent qualifier to the Description cell (contract B primary).
- [ ] (Optional) docs/cli.md Examples block comment names the provider scope (contract B secondary).
- [ ] docs/providers.md line 35 + line 59 confirmed consistent (pi=`--thinking`, claude=`--effort`,
      others nil) — edited ONLY if an inconsistency was found (contract C; none expected).
- [ ] All three docs state the SAME provider scope in the SAME words (pi=`--thinking`,
      claude=`--effort`, others graceful no-op).

### Code Quality Validation
- [ ] README callout mirrors the existing `> [!NOTE]` idiom (blank lines around; MD031 clean).
- [ ] cli.md table row keeps EXACTLY 6 columns (no added/removed `|` field separators).
- [ ] No trailing whitespace (MD009); fenced blocks have a language and surrounding blank lines (MD040/031).
- [ ] No source file, no `providers/*.toml`, no `go.mod/go.sum` touched.
- [ ] providers.md not churned without a justified inconsistency.

### Documentation
- [ ] The reasoning feature is described as now-functional and provider-scoped (not universally effective,
      not described as inert).
- [ ] No stale or incorrect flag names remain (`--effort` for claude, `--thinking` for pi; never
      `--thinking-effort`).
- [ ] The `planner = high` shipped default is implicitly accurate (the `(off; planner: high)` cell kept).

---

## Anti-Patterns to Avoid

- ❌ **Don't remove or rewrite the `stagecoach --reasoning high` example.** Contract (A) explicitly says
  KEEP it (it now works for pi/claude). The edit is an ADDED `> [!NOTE]` callout, not a deletion.
- ❌ **Don't use `--thinking-effort` for claude.** The verified flag is `--effort` (`claude --help`);
  `--thinking-effort` is the PRD's wrong guess. The §D anti-guard grep must stay empty. (gotcha)
- ❌ **Don't delete/reword the cli.md `"" (off; planner: high)` default cell.** It is now ACCURATE
  (`planner = high` emits real tokens for pi/claude). Edit the Description cell only. (Task 2 gotcha)
- ❌ **Don't change providers.md "just in case."** Line 35 is already Mode-A-updated and consistent.
  "Ensure consistent" ≠ "must change". Editing a matching file adds risk for zero benefit. (Task 4 gotcha)
- ❌ **Don't touch the Flag↔env↔git-config map table (cli.md:164-170).** It's a pure 3-column mapping
  with no Description column; contract (B) names only line 43. (gotcha)
- ❌ **Don't put the README note INSIDE the bash code block.** It would pollute the runnable snippet.
  Put it AFTER the closing ``` fence. (Task 1 gotcha)
- ❌ **Don't edit any `.go` / `providers/*.toml` / `go.mod`.** This is documentation-only. The parallel
  P1.M5.T1.S1 owns `internal/config/*`; this task edits 0 source files. (scope)
- ❌ **Don't break the cli.md table column count.** The `--reasoning` row is 6 columns. Appending to the
  Description cell must keep exactly 6 field pipes, or markdownlint/the table breaks. (Task 2 gotcha)
