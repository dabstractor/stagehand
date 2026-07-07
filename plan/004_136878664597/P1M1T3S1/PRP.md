---
name: "P1.M1.T3.S1 — Verify README.md and remaining docs/ files have no stale reasoning-default references"
description: |
  Mode B "Sync changeset-level documentation" verification task for the plan/004 "reasoning is
  opt-in everywhere (off for all roles)" changeset. The delta flipped the shipped reasoning default
  from `planner = high; others = off` to `planner = stager = message = arbiter = off` (FR-R6). Two
  sibling subtasks already landed the Mode-A doc edits in their owned files: T1.S2 updated
  `docs/cli.md` (flag help, `--reasoning` default column → `"" (off)`); T2.S1 updated
  `docs/configuration.md` (the `[defaults] reasoning = "off"` example + env-var table). This task
  verifies the FOUR remaining user-facing doc surfaces are free of stale "planner defaults to high"
  (or equivalent) DEFAULT claims: `README.md` (repo root), `docs/how-it-works.md`,
  `docs/providers.md`, `docs/README.md`. **Expected outcome: ZERO edits (verification no-op).** All
  four files are already coherent with the delta at HEAD (commit `9d33b9e`) — README.md's
  `--reasoning high` is a legitimate opt-in INVOCATION EXAMPLE (the contract explicitly blesses
  leaving it); how-it-works.md and docs/README.md contain no reasoning content at all;
  providers.md documents the `reasoning_levels` manifest TABLE SHAPE and uses "reasoning" as a verb
  in model-tier rationale — neither is a default claim. The CRITICAL job of this PRP is to PREVENT
  FALSE-POSITIVE EDITS: a naive grep-and-replace that "fixes" the README `--reasoning high` example
  (or the `reasoning_levels` shape doc, or the verb "reasoning") would itself introduce an error.
  The single repo-wide narrow-grep match (`docs/configuration.md:153`
  `STAGECOACH_PLANNER_REASONING=high`) is (a) a correct opt-in example and (b) OUT OF SCOPE (T2.S1's
  file). The deliverable is the verification itself (grep gates + per-file classification), not a
  doc rewrite. This PRP is a VERIFY-AND-CONFIRM runbook: run the gates, classify each hit against
  the stale-default-vs-correct-example decision criterion, and pass with zero edits (or, in the
  unlikely event a real stale DEFAULT claim is found in an in-scope file, apply ONE surgical line
  edit to that file). Do NOT modify PRD.md, tasks.json, prd_snapshot.md, .gitignore, or any source
  code; do NOT edit docs/cli.md or docs/configuration.md (sibling-owned).
---

## Goal

**Feature Goal**: Verify that no user-facing documentation outside the two already-updated sibling
files contains a stale assertion that the planner (or any role) ships with reasoning `high` by
default — i.e., confirm the plan/004 "off for every role" delta (FR-R6) is reflected coherently
across ALL doc surfaces, not just the two Mode-A files T1.S2/T2.S1 touched.

**Deliverable** (verify-and-confirm; expected zero edits at HEAD):
1. Run the contract's narrow grep (`planner.*high | planner.*default.*high |
   reasoning.*planner.*high`) plus a broader default-claim grep over `README.md docs/`.
2. Classify every hit against the **stale-default-claim vs correct-opt-in-example** decision
   criterion (§Blueprint): only an assertion that `high` IS the shipped default counts as stale.
3. Confirm the four in-scope files (README.md, docs/how-it-works.md, docs/providers.md,
   docs/README.md) are each coherent with the delta.
4. If (unlikely) a real stale DEFAULT claim is found in an in-scope file, apply ONE surgical line
   edit → "off for every role; opt-in per role (FR-R6)". Otherwise: zero edits.

**Success Definition**: All four in-scope files verified; every grep hit correctly classified (no
false-positive edits to legitimate examples/shape-docs/verb-usage); `docs/cli.md` and
`docs/configuration.md` UNCHANGED by this task (sibling-owned); the repo's doc set is internally
consistent with FR-R6's "off for every role." The honest outcome is "verified complete — no doc
edits."

## User Persona

**Target User**: The Stagecoach contributor / reviewer confirming the plan/004 "reasoning opt-in
everywhere" changeset is reflected consistently across the entire documentation surface — so a user
reading ANY doc (README, how-it-works, providers, the docs index) never encounters a contradiction
of the "off by default" behavior, while opt-in examples (`--reasoning high`) remain to teach usage.

**Use Case**: A user reads the README, sees `stagecoach --reasoning high` in the examples, and
understands it as "here's how to turn deeper reasoning on" (opt-in) — NOT as "reasoning defaults to
high." They then read providers.md / how-it-works.md and find no conflicting default claim. The
docs are coherent with what the binary does (off out of the box, per FR-R6) and with what
`config init` writes (`reasoning = "off"` in `[defaults]`, FR-B1).

**Pain Points Addressed**: Prevents doc/code drift where a stale "planner defaults to high" sentence
would mislead a user into thinking reasoning is always on (cost/latency surprise). Equally, prevents
an over-eager doc sweep from DELETING legitimate opt-in examples (which would hide the feature from
users who want it).

## Why

- **The plan/004 delta flipped the shipped reasoning default (FR-R6).** OLD: `planner = high; others
  = off`. NEW: `planner = stager = message = arbiter = off` — opt-in everywhere. Any doc still
  asserting the old default contradicts the binary and `config init` output. This task is the
  changeset-level sweep that catches any such drift the per-file Mode-A subtasks didn't own.
- **critical_findings.md Finding 4 established the conclusion (no separate sweep needed) but flagged
  the docs/ tree exists.** The delta_prd.md's "no docs/ directory exists" claim was wrong (docs/ has
  cli.md, configuration.md, how-it-works.md, providers.md, README.md). Finding 4 re-verified each
  file and concluded the sweep is a no-op — but that conclusion must be CONFIRMED against HEAD, not
  assumed, because the conclusion rests on classifying each reasoning reference correctly.
- **Two of the five doc files are already owned/landed by sibling subtasks.** T1.S2 (cli.md) and
  T2.S1 (configuration.md) did the Mode-A edits. This task covers the remaining three files plus
  the repo-root README — the surfaces no other subtask swept.
- **The false-positive risk is the real hazard.** A mechanical grep for "high" near "reasoning"
  matches the README's `stagecoach --reasoning high` example, the configuration.md env-var examples,
  and providers.md's `reasoning_levels` shape doc. Blindly "fixing" those would damage correct docs.
  This PRP front-loads the classification rule so the implementer edits only genuine default claims.

## What

A verification pass over `README.md` + `docs/` using the contract's grep plus a broader default-claim
grep, with every hit classified. The four in-scope files are each expected to be already-correct:

- **README.md** — `--reasoning high` (L122) is an opt-in invocation example; the `> [!NOTE]`
  (L137–139) describes provider-dependent mechanism (no default claim).
- **docs/how-it-works.md** — no `reasoning` references at all; "planner" appears only as the role
  name in the pipeline description.
- **docs/providers.md** — `reasoning_levels` (L35) documents the manifest table SHAPE; "reasoning"
  in the tier table (L108/L111) is a verb (model-tier rationale); neither is a default claim.
- **docs/README.md** — a doc index; no reasoning/planner content.

The only permissible edit is a single surgical line change IF a gate reveals an actual stale DEFAULT
claim in one of those four files. The two sibling-owned files (cli.md, configuration.md) are
out-of-scope for edits even if the grep re-hits them (and both are already correct).

### Success Criteria

- [ ] The contract grep `grep -rn 'planner.*high\|planner.*default.*high\|reasoning.*planner.*high'
      README.md docs/` has been run and EVERY match classified (stale-default vs correct-example vs
      shape-doc).
- [ ] A broader default-claim grep (§Validation Level 1) has been run and returns no genuine
      "high is the default" assertion in any in-scope file.
- [ ] README.md's `--reasoning high` example (L122) is UNCHANGED (correct opt-in example — D1).
- [ ] docs/providers.md's `reasoning_levels` shape doc (L35) and tier-table "reasoning" verb usage
      (L108/L111) are UNCHANGED (Category 3 — D3).
- [ ] docs/how-it-works.md and docs/README.md are UNCHANGED (no reasoning-default content).
- [ ] docs/cli.md and docs/configuration.md are UNCHANGED BY THIS TASK (sibling-owned — D2).
- [ ] If a stale default claim was found and fixed in an in-scope file: exactly ONE line edited,
      rewritten to "off for every role; opt-in per role (FR-R6)"; `go test ./...` still green; no
      other file touched.
- [ ] If no stale claim was found (expected): ZERO edits; `git diff --stat -- README.md docs/` is
      empty.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, any `*.go` source, or
      anything under `plan/` (except this PRP + research note).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes — and the key context is the classification rule and that the work is expected
to be a no-op.** This PRP quotes the live line-by-line state of every reasoning/planner reference in
`README.md docs/` (so the implementer recognizes each as already-correct rather than blindly editing),
gives the stale-default-vs-correct-example decision criterion with a 3-row table, names the exact
four in-scope files and the two out-of-scope sibling files, and prescribes the single surgical edit
to apply ONLY if a genuine default claim is found. The architecture `critical_findings.md` Finding 4
and the research note pin the provenance and the per-file verdicts.

### Documentation & References

```yaml
# MUST READ — the intended delta, the prior conclusion, and the per-file verdicts
- docfile: plan/004_136878664597/docs/critical_findings.md
  why: "Finding 4 ('docs/ exists but Mode B is still unnecessary') is THE prior conclusion this task
        confirms. It lists all five doc files, states why each needs no edit (cli.md/configuration.md
        are Mode-A sibling-owned; how-it-works.md has no reasoning refs; providers.md documents the
        reasoning_levels SHAPE not defaults; README.md shows --reasoning high as an example not a
        default), and explicitly calls out the delta_prd.md error ('no docs/ directory exists' was
        wrong)."
  critical: "Finding 4's CONCLUSION (no-op) is what this task verifies against HEAD. The implementer
             must re-confirm it (not assume it) by running the grep and classifying each hit."

- docfile: plan/004_136878664597/delta_prd.md
  why: "§3 (FR-R6 change) defines the OLD vs NEW default (planner=high → all=off). §4 ('Mode B')
        contains the WRONG claim 'no docs/ directory exists' that Finding 4 corrected. §1 row 5/6
        list the exact PRD doc sites already updated in PRD.md (§15.2 default column, §16.2/§16.4
        comments) — those are in PRD.md, NOT in docs/, and are already landed."
  critical: "The delta_prd §4 'no docs/' claim is FALSE — docs/ exists. Do not propagate it. The
             delta_prd's CONCLUSION (no sweep needed) survives the correction, but only via the
             per-file classification in Finding 4 / this PRP's research §4."

- docfile: plan/004_136878664597/P1M1T3S1/research/docs_stale_reference_verification.md
  why: "THIS subtask's own research: the scope table (§2 — which files this task owns vs siblings);
        the 3-category decision criterion (§3); the per-file empirical findings with exact line
        citations (§4); and the decisions log (§6). READ THIS FIRST."
  critical: "§3 (stale-default vs correct-example vs shape-doc) is the single most important rule —
             it prevents false-positive edits to README's --reasoning high example. §4 gives the
             verbatim live state of every hit so the implementer confirms rather than guesses."

- docfile: plan/004_136878664597/P1M1T2S1/PRP.md
  why: "The sibling verify-and-confirm PRP (T2.S1, configuration.md). Establishes the SAME pattern
        (work already done at HEAD; verify, don't churn; prescribe a surgical edit ONLY on drift)
        and confirms docs/configuration.md is T2.S1's territory — NOT this task's."
  critical: "T2.S1 owns docs/configuration.md. The one repo-wide narrow-grep match
             (configuration.md:153 STAGECOACH_PLANNER_REASONING=high) is in T2.S1's file AND is a
             correct opt-in example. This task must NOT edit it (D2)."

- docfile: plan/004_136878664597/P1M1T1S2/PRP.md
  why: "The sibling verify-and-confirm PRP (T1.S2, cli.md). Confirms docs/cli.md is T1.S2's
        territory (the --reasoning flag help + default column) and is already correct at HEAD."
  critical: "T1.S2 owns docs/cli.md. This task must NOT edit it (D2). cli.md's --reasoning high
             example (L212–213) is a correct opt-in example."

# The four in-scope files (VERIFY; surgical edit only if a REAL stale default claim is found)
- file: README.md
  why: "Repo-root marketing surface (PRD §21.5). The 'Example invocations' bash block (L115–133)
        includes `stagecoach --reasoning high` (L122) under the comment `# Use reasoning for deeper
        analysis on the planner` (L121) — a CORRECT opt-in example (Category 2). The `> [!NOTE]`
        (L137–139) describes the provider-dependent mechanism (pi --thinking / claude --effort /
        graceful no-op / per-role) — NO default claim."
  pattern: "Examples teach opt-in usage; the NOTE explains mechanism. Neither asserts a default."
  gotcha: "L122 `--reasoning high` is the #1 false-positive risk. The contract EXPLICITLY blesses
           leaving it. Do NOT 'fix' it — it shows the user CAN set high, not that high IS the default."

- file: docs/how-it-works.md
  why: "Architecture + pipeline overview. The word 'reasoning' does NOT appear (broader grep: zero
        matches). 'planner' appears (L59 role table, L71 pipeline diagram, L109 T_start freeze,
        L113 one-file short-circuit) exclusively as the ROLE NAME describing the planner agent's job
        (analyze diff, partition) — never a reasoning-LEVEL default."
  pattern: "Role-name usage of 'planner'; no reasoning-level content."
  gotcha: "Do not confuse the planner ROLE (always exists) with the planner reasoning LEVEL (now off).
           The role references here are correct; leave them."

- file: docs/providers.md
  why: "Manifest schema + built-in providers. L35 documents the `reasoning_levels` manifest TABLE
        (off/low/medium/high token lists; nil⇒graceful no-op; FR-R6) — the SHAPE, not a default.
        L59 documents the Render append rule (mechanism). L108/L111 tier-table rationale uses
        'reasoning' as a VERB ('Needs the strongest model for task decomposition and architecture
        reasoning' / 'Needs reasoning to evaluate diffs') — MODEL-tier justification (FR-D3), a
        separate axis from the reasoning LEVEL (FR-R6)."
  pattern: "Table-shape doc + Render mechanism + verb usage of 'reasoning' for model-tier rationale."
  gotcha: "The tier table (L104–112) is about MODEL sizing (FR-D3), NOT reasoning level (FR-R6).
           'reasoning' as a verb there is correct; do NOT grep-and-replace the word. L35's table is
           the manifest field definition — unchanged by the default flip (only WHICH level is default
           changed, not the level→tokens mapping)."

- file: docs/README.md
  why: "Documentation index page. Broader grep: ZERO matches for both 'reasoning' and 'planner'.
        Pure links (cli.md/configuration.md/providers.md/how-it-works.md) + install + contributing."
  pattern: "Index; no feature-detail content."
  gotcha: "Nothing to verify here beyond the grep returning zero. Do not invent edits."

# External references
- url: https://git-scm.com/docs/git-grep
  why: "git-grep is the deterministic grep engine used for the gates (consistent across environments,
        honors the repo's pathspec). The contract's grep is runnable as `git grep -nE '<pattern>'`."
  critical: "Use `-n` (line numbers) and `-E` (extended regex) for the alternation pattern. Classify
             every hit — do not pipe to a blind edit."
```

### Current Codebase Tree (this task's scope)

```bash
stagecoach/
├── README.md                # IN SCOPE — verify (--reasoning high example, NOTE block)
└── docs/
    ├── README.md            # IN SCOPE — verify (index; no reasoning content)
    ├── how-it-works.md      # IN SCOPE — verify (planner role refs; no reasoning-level content)
    ├── providers.md         # IN SCOPE — verify (reasoning_levels shape; verb usage)
    ├── cli.md               # OUT OF SCOPE — T1.S2 (already correct)
    └── configuration.md     # OUT OF SCOPE — T2.S1 (already correct; in flight)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (expected: ZERO edits at HEAD — all four in-scope files already coherent with FR-R6)
    README.md                # unchanged (--reasoning high is a correct opt-in example)
    docs/README.md           # unchanged (no reasoning content)
    docs/how-it-works.md     # unchanged (planner = role name, not a level default)
    docs/providers.md        # unchanged (reasoning_levels shape + verb usage, not defaults)
```

| Path | Action | Responsibility |
|---|---|---|
| `README.md` | VERIFY (edit ONLY if a real stale default claim found) | Confirm `--reasoning high` is an example, not a default; confirm NOTE describes mechanism. |
| `docs/how-it-works.md` | VERIFY | Confirm zero reasoning-level default content. |
| `docs/providers.md` | VERIFY | Confirm `reasoning_levels` is shape-doc and "reasoning" is verb usage. |
| `docs/README.md` | VERIFY | Confirm zero reasoning/planner content. |
| `docs/cli.md` | NO-TOUCH | T1.S2 territory. |
| `docs/configuration.md` | NO-TOUCH | T2.S1 territory. |

**Explicitly NOT touched**: `docs/cli.md` (T1.S2), `docs/configuration.md` (T2.S1), any `*.go`
source or test (the behavioral flip is S1/S2 of T1, already landed), `pkg/stagecoach/*`,
`providers/*.toml` (the `reasoning_levels` tables are unchanged — only the default level changed),
`PRD.md` (read-only), `tasks.json`, `prd_snapshot.md`, `.gitignore`, anything under `plan/`.

### Known Gotchas of our codebase & toolchain

```markdown
<!-- CRITICAL (G1 — the #1 false positive: README's --reasoning high). The narrow grep does NOT match
this line (it lacks "planner" adjacent), but a broader "reasoning.*high" sweep WILL. The contract
EXPLICITLY states README.md's --reasoning high is a correct opt-in EXAMPLE (Category 2) to be LEFT
AS-IS. It shows the user CAN set reasoning high, not that high IS the default. Editing it would
HIDE the opt-in feature from the README. Do NOT touch README.md:122. (Decision D1.) -->

<!-- CRITICAL (G2 — the one repo-wide narrow-grep match is OUT OF SCOPE and correct). `grep -rn
'planner.*high' README.md docs/` returns exactly ONE hit: docs/configuration.md:153
(`STAGECOACH_PLANNER_REASONING=high stagecoach` in the env-var table's example column). This is (a) a
CORRECT opt-in example (Category 2 — an env-var invocation, not a default claim) and (b) in
docs/configuration.md, which is T2.S1's file (OUT OF SCOPE for this task, D2). The companion default
statement on the SAME file (L80 `reasoning = "off" … off by default for every role`) is the correct
flipped default T2.S1 landed. Do NOT edit configuration.md. Refer, don't fix. -->

<!-- CRITICAL (G3 — "reasoning" is sometimes a VERB, not the level). docs/providers.md:108 ("Needs the
strongest model for task decomposition and architecture reasoning") and :111 ("Needs reasoning to
evaluate diffs") use "reasoning" as a verb justifying the MODEL TIER (FR-D3). The tier table is a
DIFFERENT axis from the reasoning LEVEL (FR-R6). Do NOT grep-and-replace the word "reasoning" — that
would corrupt correct prose. (Decision D3.) -->

<!-- CRITICAL (G4 — the reasoning_levels TABLE is shape, not default). docs/providers.md:35 documents
the manifest `reasoning_levels` field (off/low/medium/high token lists; nil⇒graceful no-op). The
default flip changed WHICH level is default, NOT the level→tokens mapping or the table's existence.
The shape doc is unchanged and correct. Do NOT edit it. (Decision D3 / Category 3.) -->

<!-- GOTCHA (G5 — the delta_prd.md "no docs/ directory" claim is FALSE). plan/004 delta_prd.md §4
says "no docs/ directory exists." It does (5 files). critical_findings.md Finding 4 corrected this.
Do NOT propagate the false claim; do NOT skip the verification on the assumption there's nothing to
check. Run the grep; the conclusion (no-op) survives the correction via per-file classification. -->

<!-- GOTCHA (G6 — "planner" the ROLE vs "planner reasoning" the LEVEL). docs/how-it-works.md
references "planner" as the decomposition role (L59/L71/L109/L113). The planner role ALWAYS exists
(FR-R1); only its reasoning LEVEL flipped to off (FR-R6). These role references are correct; they
are not default claims. Leave them. -->

<!-- GOTCHA (G7 — do not touch sibling-owned files). docs/cli.md is T1.S2's; docs/configuration.md
is T2.S1's. The grep re-hits both incidentally (they contain reasoning examples/docs). Both are
already correct at HEAD. Editing them is scope creep and risks conflicting with a parallel/landed
subtask. (Decision D2.) -->

<!-- GOTCHA (G8 — do not fabricate a diff). The honest outcome is "verified complete — no doc edits"
(mirroring the P1.M1.T2.S1 verify-and-confirm pattern). Do NOT invent a before/after to justify the
subtask. If every gate passes at HEAD (expected), the outcome is a clean verification with zero
edits and an empty `git diff --stat -- README.md docs/`. -->

<!-- GOTCHA (G9 — the build/test gate is a backstop, not the point). This is a docs task; no .go
files are in scope. `go test ./...` is run only to confirm the repo is still green IF a surgical
edit was needed (it touches markdown, so it won't affect the build — but run it for completeness).
If zero edits (expected), the test gate is informational. -->
```

## Implementation Blueprint

### The decision criterion (the single most important rule)

Every grep hit is classified into exactly one of three categories. **Only Category 1 is stale and
in-scope for an edit.**

| Category | Signal | Example | Verdict |
|---|---|---|---|
| **1. STALE default claim** | asserts `high` IS the shipped/default value for planner (or any role) | "the planner defaults to high"; "planner reasoning is high by default"; "default off (planner: high)"; "reasoning: high (default for planner)" | 🛠 UPDATE → "off for every role; opt-in per role (FR-R6)" |
| **2. CORRECT opt-in example** | shows the user INVOKING reasoning high to turn it on | `stagecoach --reasoning high`; `STAGECOACH_PLANNER_REASONING=high stagecoach`; `STAGECOACH_REASONING=high stagecoach` | ✅ LEAVE — teaches opt-in usage |
| **3. CORRECT shape / mechanism / verb** | documents the `reasoning_levels` TABLE; the Render append rule; or uses "reasoning" as a verb | "`reasoning_levels`: off/low/medium/high token lists; nil⇒no-op"; "needs reasoning to evaluate diffs" | ✅ LEAVE — mechanism, not a default |

**Discriminator:** does the text answer "what is it out of the box?" (default claim) or "how do I
turn it on?" (example) or "how does the field work?" (shape)? Only the first is stale.

### Implementation Tasks (ordered — verify-then-(maybe)-fix)

```yaml
Task 1: RUN the contract grep + classify every hit
  - RUN: grep -rn 'planner.*high\|planner.*default.*high\|reasoning.*planner.*high' README.md docs/
  - EXPECT: exactly ONE match — docs/configuration.md:153 (STAGECOACH_PLANNER_REASONING=high …).
  - CLASSIFY: Category 2 (correct opt-in example) AND out-of-scope (T2.S1's file). → NO EDIT.
  - IF a DIFFERENT match appears in an in-scope file (README/how-it-works/providers/docs-README):
    classify it. Only if it is Category 1 (a genuine "high is the default" assertion) → proceed to
    the surgical fix in Task 4. Otherwise → NO EDIT (it is an example/shape/verb).

Task 2: RUN the broader default-claim grep (catches phrasings the narrow pattern misses)
  - RUN: grep -rniE 'default.*(high|reasoning)|reasoning.*default|high by default|defaults to high|planner.*default' README.md docs/
  - EXPECT matches ONLY in: docs/configuration.md:80 ("off by default for every role" — CORRECT,
    the flipped default), docs/configuration.md:190 (precedence prose, no high claim),
    docs/cli.md:83 (bootstrap prose, no high claim). None assert "high is the default."
  - CLASSIFY each: all are correct (the flipped default or precedence/shape prose). → NO EDIT.
  - IF any hit asserts high-as-default in an in-scope file → Task 4 surgical fix.

Task 3: PER-FILE confirmation of the four in-scope files (the verification deliverable)
  - README.md:
      RUN: grep -n 'reasoning' README.md
      EXPECT: L121 (comment), L122 (--reasoning high EXAMPLE), L137–139 (NOTE mechanism).
      VERDICT: all Category 2/3 — CORRECT. LEAVE. (G1.)
  - docs/how-it-works.md:
      RUN: grep -ni 'reasoning' docs/how-it-works.md   → expect ZERO matches.
      RUN: grep -ni 'planner' docs/how-it-works.md     → expect role-name refs (L59/71/109/113).
      VERDICT: no reasoning-level content; planner = role name. CORRECT. LEAVE. (G6.)
  - docs/providers.md:
      RUN: grep -ni 'reasoning' docs/providers.md
      EXPECT: L35 (reasoning_levels TABLE shape), L59 (Render mechanism), L108/L111 (verb). 
      VERDICT: all Category 3 — CORRECT. LEAVE. (G3/G4.)
  - docs/README.md:
      RUN: grep -ni 'reasoning\|planner' docs/README.md   → expect ZERO matches.
      VERDICT: index page, no content. CORRECT. LEAVE.

Task 4: (CONDITIONAL — only if Task 1/2 found a Category-1 stale default claim in an in-scope file)
  SURGICAL EDIT — exactly ONE line, in exactly ONE in-scope file:
    - FIND the stale assertion (e.g. "the planner defaults to high").
    - REPLACE with: "off for every role; opt-in per role (FR-R6)" (or equivalent wording matching
      docs/configuration.md:80 / docs/cli.md:43).
    - DO NOT touch any other line/file. DO NOT touch cli.md or configuration.md (G7).
    - This task is EXPECTED NOT TO FIRE — at HEAD all four in-scope files are coherent with FR-R6.

Task 5: GATES + scope-check
  - RUN (only if Task 4 fired): go build ./... ; go vet ./... ; gofmt -l . ; go test -race ./
    → EXPECT green (a markdown edit cannot break the build, but confirm the suite is healthy).
  - RUN: git diff --stat -- README.md docs/
    → EXPECT at HEAD: EMPTY (zero edits — the verification no-op). If Task 4 fired, exactly ONE
       in-scope file appears with a one-line diff; cli.md/configuration.md MUST NOT appear.
  - RUN: git diff --stat -- docs/cli.md docs/configuration.md
    → EXPECT: EMPTY (sibling-owned; this task did not touch them).
```

### Implementation Patterns & Key Details

```markdown
<!-- === The live state of every reasoning/planner reference in README.md docs/ (VERIFY) === -->

<!-- README.md:122 (Example invocations bash block) — CORRECT opt-in example (Category 2): -->
# Use reasoning for deeper analysis on the planner
stagecoach --reasoning high
<!-- LEAVE — the contract explicitly blesses this. It shows opt-in, not a default. -->

<!-- README.md:137–139 (NOTE) — CORRECT mechanism doc (Category 3): -->
> `--reasoning` is provider-dependent: it engages deeper reasoning for **pi** (`--thinking`) and
> **claude** (`--effort`). Other providers treat it as a graceful no-op (no error) per FR-R6. It
> applies to any role via `--<role>-reasoning` or `[role.*] reasoning`.
<!-- LEAVE — describes which providers honor it + per-role; no default claim. -->

<!-- docs/configuration.md:80 — the CORRECT flipped default (T2.S1, OUT OF SCOPE): -->
reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)
<!-- LEAVE — this IS the target state; editing it would REVERT the delta. -->

<!-- docs/configuration.md:153 — the one narrow-grep match (T2.S1, OUT OF SCOPE, CORRECT): -->
| `STAGECOACH_PLANNER_REASONING` | `--planner-reasoning` | Per-role: planner reasoning | `STAGECOACH_PLANNER_REASONING=high stagecoach` |
<!-- LEAVE — an env-var invocation EXAMPLE (Category 2), not a default claim. -->

<!-- docs/providers.md:35 — CORRECT shape doc (Category 3): -->
| `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6)… |
<!-- LEAVE — the manifest field DEFINITION; the default flip did not change the table. -->

<!-- docs/providers.md:108/111 — CORRECT verb usage (Category 3, model-tier rationale FR-D3): -->
| **planner** | flagship / smart | Needs the strongest model for task decomposition and architecture reasoning. |
| **arbiter** | mid | Needs reasoning to evaluate diffs, but not the flagship — mid-tier balances quality and cost. |
<!-- LEAVE — "reasoning" as a verb justifying the MODEL tier; a different axis from the LEVEL. -->

<!-- === The surgical fix template (ONLY if a Category-1 claim is found in an in-scope file) === -->
<!-- STALE (hypothetical, does NOT exist at HEAD): "the planner defaults to high reasoning" -->
<!-- FIX  : "reasoning is off for every role by default; opt in per role (FR-R6)"             -->
<!-- Apply to exactly the one line, one file. Match the wording of configuration.md:80.       -->
```

### Integration Points

```yaml
DOCS (this task's scope):
  - README.md, docs/README.md, docs/how-it-works.md, docs/providers.md: verify, (conditionally) edit

SIBLING-OWNED (NO-TOUCH):
  - docs/cli.md                # T1.S2 (flag help, --reasoning default column "" (off))
  - docs/configuration.md      # T2.S1 ([defaults] reasoning="off" example, env-var table)

CONSUMED (already landed at commit 9d33b9e):
  - FR-R6 flip: planner reasoning default high → off (internal/config/roles.go)
  - FR-B1: config init emits reasoning = "off" (internal/config/bootstrap.go)
  - PRD.md §15.2/§16.2/§16.4: default column + comments already updated

GATE: grep over README.md docs/ returns no Category-1 default claim in any in-scope file
      → git diff --stat -- README.md docs/ EMPTY (zero edits, the no-op)

NO-TOUCH (explicitly):
  - any *.go source/test (S1/S2 of T1; the behavioral flip is code, already landed)
  - providers/*.toml (reasoning_levels tables unchanged — only the default level changed)
  - PRD.md, tasks.json, prd_snapshot.md, .gitignore, plan/* (except this PRP + research note)
```

## Validation Loop

### Level 1: The Grep Gates (the core verification)

```bash
cd /home/dustin/projects/stagecoach

# (1) The CONTRACT grep (narrow). Expect exactly ONE match, in configuration.md (out-of-scope, correct).
grep -rn 'planner.*high\|planner.*default.*high\|reasoning.*planner.*high' README.md docs/
# Expected: docs/configuration.md:153:| `STAGECOACH_PLANNER_REASONING` | ... | `STAGECOACH_PLANNER_REASONING=high stagecoach` |
# Classify: Category 2 (opt-in example) + out-of-scope (T2.S1). NO EDIT.

# (2) The BROADER default-claim grep (catches "default(s) ... high" / "high by default" phrasings).
grep -rniE 'default.*(high|reasoning)|reasoning.*default|high by default|defaults to high|planner.*default' README.md docs/
# Expected: matches ONLY in configuration.md:80 ("off by default" — CORRECT), configuration.md:190
# (precedence prose), cli.md:83 (bootstrap prose). NONE assert "high is the default."
# Any hit asserting high-as-default in an in-scope file → Task 4 surgical fix.
```

### Level 2: Per-File Confirmation (the four in-scope files)

```bash
cd /home/dustin/projects/stagecoach

# README.md — the --reasoning high EXAMPLE + NOTE must remain (Category 2/3).
grep -n 'reasoning' README.md
# Expected: L121 (comment), L122 (--reasoning high), L137-139 (NOTE). All correct. LEAVE.

# docs/how-it-works.md — ZERO reasoning-level content; planner = role name.
grep -ci 'reasoning' docs/how-it-works.md   # Expected: 0
grep -ni 'planner' docs/how-it-works.md      # Expected: role-name refs (L59/71/109/113). LEAVE.

# docs/providers.md — reasoning_levels SHAPE (L35) + verb usage (L108/111). Category 3.
grep -ni 'reasoning' docs/providers.md
# Expected: L35 (table shape), L59 (Render rule), L108/L111 (verb). All correct. LEAVE.

# docs/README.md — index; ZERO reasoning/planner content.
grep -ci 'reasoning\|planner' docs/README.md   # Expected: 0
```

### Level 3: Sibling-File Integrity (NO-TOUCH confirmation)

```bash
cd /home/dustin/projects/stagecoach

# This task must NOT have edited the sibling-owned files.
git diff --stat -- docs/cli.md docs/configuration.md
# Expected: EMPTY (T1.S2 / T2.S1 territory; untouched by this task).

# Confirm the sibling files are themselves correct at HEAD (informational — not this task's edit):
grep -n '^reasoning = "off"' docs/configuration.md   # Expected: :80 (T2.S1's correct line)
grep -n '"" (off)' docs/cli.md                        # Expected: the --reasoning default column (T1.S2)
```

### Level 4: Scope Boundary + Build Backstop

```bash
cd /home/dustin/projects/stagecoach

# (A) Scope: ideally ZERO edits at HEAD (all four in-scope files coherent with FR-R6).
git diff --stat -- README.md docs/
# Expected at HEAD: EMPTY (no edits needed). If Task 4 fired, exactly ONE in-scope file, one line.

# (B) Build backstop (only meaningful if Task 4 fired; a markdown edit won't break the build, but
#     confirm the suite is healthy regardless).
go build ./...           # Expected: exit 0
go test -race ./...      # Expected: all packages green
```

## Final Validation Checklist

### Technical Validation

- [ ] The contract grep (Level 1.1) run; every match classified (stale-default vs example vs shape).
- [ ] The broader default-claim grep (Level 1.2) run; no "high is the default" assertion in any
      in-scope file.
- [ ] All four in-scope files confirmed per Level 2.
- [ ] (If Task 4 fired) `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test -race ./...` green.

### Feature Validation

- [ ] README.md's `--reasoning high` (L122) is UNCHANGED — correct opt-in example (G1/D1).
- [ ] docs/providers.md's `reasoning_levels` shape (L35) and verb usage (L108/111) UNCHANGED (G3/G4/D3).
- [ ] docs/how-it-works.md and docs/README.md UNCHANGED (no reasoning-default content).
- [ ] No user-facing doc asserts the planner (or any role) ships with reasoning `high` by default.
- [ ] The doc set is internally consistent with FR-R6 ("off for every role") and FR-B1 (config init
      writes `reasoning = "off"`).

### Scope Discipline Validation

- [ ] At HEAD, ZERO source edits were required (all four in-scope files already coherent). If a
      Category-1 claim was found and fixed, only the ONE in-scope file was touched (one line).
- [ ] `git diff --stat -- README.md docs/` is EMPTY (HEAD) OR a single minimal surgical edit in one
      in-scope file.
- [ ] `docs/cli.md` and `docs/configuration.md` UNCHANGED by this task (sibling-owned — D2/G7).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, any `*.go` source, or
      anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] The implementer recognized no genuine stale default claim exists (no false-positive edits to
      examples / shape docs / verb usage).
- [ ] The classification rule (Category 1 vs 2 vs 3) was applied to every grep hit, not a blind
      grep-and-replace.
- [ ] The honest outcome is reported ("verified complete — no doc edits"), not a fabricated diff.

---

## Anti-Patterns to Avoid

- ❌ Don't edit README.md's `--reasoning high` (L122). It is a CORRECT opt-in invocation example
  (Category 2), explicitly blessed by the contract. "Fixing" it would hide the opt-in feature.
  (G1/D1.)
- ❌ Don't edit docs/configuration.md (any line). It is T2.S1's file (out of scope). Its one
  narrow-grep match (L153 `STAGECOACH_PLANNER_REASONING=high`) is a correct env-var example, and its
  L80 `reasoning = "off"` is the correct flipped default. Editing either reverts/corrupts sibling
  work. (G2/G7/D2.)
- ❌ Don't edit docs/cli.md. It is T1.S2's file (out of scope) and already correct. (G7/D2.)
- ❌ Don't grep-and-replace the word "reasoning". In docs/providers.md it is a VERB (model-tier
  rationale, FR-D3 — a different axis from the reasoning LEVEL, FR-R6) and the `reasoning_levels`
  entry is the manifest field DEFINITION (Category 3). Blind replacement corrupts correct prose.
  (G3/G4/D3.)
- ❌ Don't confuse the planner ROLE (always exists, FR-R1) with the planner reasoning LEVEL (now
  off, FR-R6). docs/how-it-works.md's "planner" references are the role; they are correct. (G6.)
- ❌ Don't propagate the delta_prd.md "no docs/ directory exists" falsehood. docs/ exists (5 files);
  Finding 4 corrected this. Run the grep; the no-op conclusion survives via per-file classification.
  (G5.)
- ❌ Don't fabricate a before/after diff to justify the subtask. The honest outcome is "verified
  complete — zero edits" (mirrors P1.M1.T2.S1). (G8.)
- ❌ Don't touch `providers/*.toml`. The `reasoning_levels` manifest tables are unchanged — only the
  default LEVEL changed, not the level→tokens mapping.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, any `*.go` source, or
  anything under `plan/`.

---

## Confidence Score

**9.5/10** that the verification passes with zero edits at HEAD.

Rationale: Four independent pieces of evidence confirm every in-scope file is already coherent with
the "off for every role" delta: (1) the contract's narrow grep returns exactly ONE repo-wide match,
and it is in docs/configuration.md (T2.S1's out-of-scope file) where it is a correct opt-in env-var
example — zero narrow-grep matches in any in-scope file; (2) the broader reasoning grep's hits in
the in-scope files are all classifiable as Category 2 (README `--reasoning high` example, explicitly
blessed by the contract) or Category 3 (providers.md `reasoning_levels` table shape + the verb
"reasoning" in model-tier rationale; how-it-works.md and docs/README.md have zero reasoning
content); (3) `critical_findings.md` Finding 4 independently reached the same per-file verdicts and
explicitly predicted this task is "a verification no-op"; (4) the implementing commit `9d33b9e`
landed the doc updates in the two sibling-owned files (cli.md, configuration.md) and the behavioral
flip, with a clean working tree. The PRP's primary value is PREVENTING false-positive edits — it
front-loads the 3-category classification rule and quotes the live verbatim state of every hit so
the implementer confirms rather than blindly "fixes" a legitimate example. The residual 0.5
uncertainty is the mechanical possibility that a gate reveals a stale claim the research missed
(e.g., a phrasing neither grep pattern catches), in which case the PRP prescribes the single
surgical line edit to "off for every role; opt-in per role (FR-R6)" — but at HEAD no such claim
exists in any in-scope file.
