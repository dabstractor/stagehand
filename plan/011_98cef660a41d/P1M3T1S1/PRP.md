---
name: "P1.M3.T1.S1 — Update README.md and docs/configuration.md for v2.5 reliability changes (closed-loop token-budget guarantee FR3j + stale-lock reaping): Mode B changeset doc sync"
description: |

  The Mode-B changeset-level doc sync for v2.5 (plan 011): two targeted enhancements (the FR3j closed-loop
  token-budget guarantee in README.md + docs/configuration.md) + two no-op confirmations (no stale-lock
  over-claim in README; no v2.5 surface change in cli.md/providers.md). Depends on ALL implementing
  subtasks (P1.M1 for FR3j, P1.M2 for §18.5 stale-lock reaping). The shipped behavior: the assembled
  prompt is GUARANTEED to never exceed `token_limit` (FR3j re-measures and re-trims until it fits), and
  orphaned lock FILES are reaped by pid-liveness on the next Acquire (§18.5).

  THE TWO EDITS:
  (a) README.md L67 (Payload optimization row): append "— a closed-loop guarantee that the assembled prompt
      never exceeds the limit" after "via `token_limit`".
  (b) docs/configuration.md L160 (token_limit bullet): insert the closed-loop sentence after "truncates the
      diff to fit using the ≈4 chars/token estimate" — "After truncation, Stagehand assembles the actual
      full prompt, re-measures it, and re-trims until it fits — a closed-loop guarantee (§9.1 FR3j) that the
      payload never exceeds `token_limit`."

  THE TWO NO-OP CONFIRMATIONS:
  (c) README lock scan: L341 (Safe to run twice) describes CONTENTION behavior (exit 0 vs Busy), NOT
      staleness — no over-claim ("no stale locks") exists → no correction needed. The stale-lock correction
      already landed in docs/how-it-works.md L170/L179 (Mode A, P1.M2.T1.S2).
  (d) cli.md + providers.md: no token_limit/closed-loop/stale-lock mentions → no v2.5 surface change → no
      edit needed (the v2.5 reliability work changed no CLI/config/provider surface).

  ⚠️ **#1 — enhance ONLY (don't restate the full FR3j algorithm).** These are overview/marketing docs
      (README features table + configuration.md knob description). The full FR3j water-fill + closed-loop
      algorithm lives in the PRD (§9.1 FR3i/FR3j) + how-it-works.md. The enhancement is ONE clause per
      doc that surfaces the GUARANTEE ("never exceeds"), not the mechanism.

  ⚠️ **#2 — do NOT duplicate the how-it-works.md stale-lock correction.** docs/how-it-works.md L170/L179
      already has the corrected "lock never stale, FILES reaped by pid-liveness" wording (Mode A,
      P1.M2.T1.S2). README L341 does NOT over-claim staleness (it describes contention behavior). Do NOT
      add a stale-lock paragraph to README — the README has no lock-staleness claim to correct.

  ⚠️ **#3 — respect markdown house style.** `.markdownlint.json`: default true; MD013/MD033/MD060 disabled
      (long lines OK, inline code OK). The README L67 row is a markdown table row (single line, pipes); the
      enhancement must stay on that line. The configuration.md L160 bullet is a `>` blockquote line; the
      enhancement must stay in the blockquote.

  Deliverable: MODIFIED `README.md` (L67 — one clause appended to the Payload optimization row) + MODIFIED
  `docs/configuration.md` (L160 — the closed-loop sentence in the token_limit bullet). NO other file.
  OUTPUT: README.md and docs/configuration.md accurately describe the v2.5 reliability improvements; no
  over-claims remain. DOCS: this IS the documentation sync task (Mode B).

---

## Goal

**Feature Goal**: Surface the v2.5 closed-loop token-budget guarantee (FR3j) in the two overview docs that
mention `token_limit` (README features table + configuration.md knob description), so the docs state the
hard invariant ("the assembled prompt never exceeds the limit") rather than implying a best-effort estimate.
Confirm no stale-lock over-claim remains in README (the how-it-works.md correction is already landed).

**Deliverable** (MODIFY existing docs only):
1. `README.md` L67 — append the closed-loop guarantee clause to the Payload optimization table row.
2. `docs/configuration.md` L160 — insert the closed-loop re-measure/re-trim sentence into the token_limit bullet.
3. (No-op confirmations) README lock scan (L341 — no over-claim) + cli.md/providers.md scan (no v2.5 surface).

**Success Definition**: README L67 + configuration.md L160 state the closed-loop guarantee; the text matches
the shipped FR3j behavior (§9.1 FR3j: "Invariant: EstimateTokens(assembledFullPrompt) ≤ token_limit,
always"); no stale-lock over-claim in README; cli.md/providers.md unchanged; markdownlint-compliant.

## User Persona

**Target User**: The README/docs reader deciding whether to set `token_limit` — who must understand the
guarantee is HARD (the prompt will not exceed the limit), not a best-effort estimate that might drift.

**Use Case**: A user with a 128k-context model sets `token_limit = 120000` and reads the docs to confirm
the payload will fit. The enhanced docs state the closed-loop guarantee explicitly.

**Pain Points Addressed**: The pre-v2.5 docs said "truncates the diff to fit" (implying best-effort); the
shipped v2.5 behavior is a hard invariant (FR3j re-measures and re-trims until it fits). The docs must not
understate the guarantee.

## Why

- **Closes the v2.5 changeset doc sync.** P1.M1 (FR3j closed-loop) + P1.M2 (§18.5 stale-lock reaping)
  landed code + how-it-works.md corrections; the overview docs (README + configuration.md) still describe
  the pre-v2.5 "best-effort" behavior.
- **Surfaces a user-facing reliability guarantee.** The closed-loop is a headline v2.5 improvement (the
  prompt NEVER exceeds token_limit); the overview docs are where a user encounters `token_limit` first.
- **Cheap, surgical.** Two one-sentence enhancements. No code, no config, no API surface change.

## What

Two targeted one-sentence enhancements to existing doc lines, plus two confirmed no-ops. No new sections,
no new features described, no re-algorithm-ization.

### Success Criteria

- [ ] README.md L67: the Payload optimization row mentions the closed-loop guarantee.
- [ ] docs/configuration.md L160: the token_limit bullet states the closed-loop re-measure/re-trim invariant.
- [ ] README L341 (lock): scanned, no stale-lock over-claim found (confirmed no-op).
- [ ] docs/cli.md + docs/providers.md: scanned, no v2.5 surface change needed (confirmed no-op).
- [ ] Both edits are markdownlint-compliant (table row stays on one line; blockquote stays in the `>` block).
- [ ] No other file modified; PRD.md, go.mod, all `.go` files byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior repo knowledge can do this from: the exact current text of both edit targets
(below), the exact enhancement wording, the PRD §9.1 FR3j spec (in your context), and the markdownlint
config. No code knowledge required — this is a prose enhancement.

### Documentation & References

```yaml
# The PRD basis (in your context as selected_prd_content)
- file: PRD.md (or plan/011_…/prd_snapshot.md)
  section: "9.1 Diff capture" FR3j (the closed-loop guarantee spec: "Invariant: EstimateTokens(
           assembledFullPrompt) ≤ token_limit, always") + FR3d (token_limit overview).
  critical: FR3j is the guarantee to surface — "never exceeds token_limit", a hard invariant (not
       best-effort). The full algorithm (water-fill + re-measure loop) stays in the PRD/how-it-works; the
       overview docs get ONE clause.

# The files under edit — exact current text
- file: README.md
  section: L67 — the Payload optimization table row (exact text captured in research). The enhancement is a
           clause appended AFTER "via `token_limit`".
  critical: the row is a markdown TABLE row (pipes) — the enhancement must stay on the SAME line, inside
       the cell, before the link brackets.
- file: docs/configuration.md
  section: L160 — the `**token_limit**` bullet (exact text captured). The enhancement is a sentence
           inserted AFTER "truncates the diff to fit using the ≈4 chars/token estimate".
  critical: the bullet is a `>` blockquote line — the enhancement must stay in the blockquote.

# The already-landed corrections (confirm, don't duplicate)
- file: docs/how-it-works.md
  section: L170/L179 — the stale-lock correction (Mode A, P1.M2.T1.S2): "the LOCK never goes stale.
           Orphaned lock FILES ... are reaped by pid-liveness on the next Acquire."
  why: confirms the lock-staleness correction is ALREADY in how-it-works.md. README L341 does NOT
       over-claim staleness (it describes contention behavior). Do NOT add a stale-lock correction to README.
- file: docs/configuration.md
  section: L160 diff_context bullet — "Valid range is 0–3; an out-of-range value is rejected at config
           load" (already correct — from a prior bugfix). The `no_verify` bullet (~L155) already uses
           `stagehand.noVerify` (the corrected git-valid key). No v2.5 change needed for either.

# House style
- file: .markdownlint.json
  why: default true; MD013 (line length) / MD033 (inline HTML) / MD060 disabled ⇒ long table rows and
       blockquote lines are fine; inline code + plain prose compliant.

# The parallel PRP (no-conflict confirmation)
- file: plan/011_…/P1M2T3S2/PRP.md
  why: confirms P1.M2.T3.S2 (exit-path signal tests) touches ONLY `internal/signal/signal.go` — NOT
       README.md or docs/*. Zero file overlap ⇒ independent.
```

### Current Codebase tree (relevant slice)

```bash
README.md                     # L67 Payload optimization row — EDIT (+closed-loop clause)
docs/configuration.md         # L160 token_limit bullet — EDIT (+closed-loop sentence)
docs/how-it-works.md          # L170/L179 stale-lock correction — ALREADY LANDED (P1.M2.T1.S2) — confirm, don't duplicate
docs/cli.md                   # NO v2.5 surface — confirm, don't edit
docs/providers.md             # NO v2.5 surface — confirm, don't edit
.markdownlint.json            # house style (MD013/MD033/MD060 disabled)
# All .go files, PRD.md — UNCHANGED.
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits: README.md (L67) + docs/configuration.md (L160).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (enhance ONLY, don't re-algorithm-ize): the README row + configuration.md bullet are overview
     surfaces — ONE clause that surfaces the GUARANTEE ("never exceeds"), not the FR3j water-fill mechanism.
     The full algorithm lives in the PRD §9.1 FR3i/FR3j + how-it-works.md. -->

<!-- CRITICAL (don't duplicate how-it-works.md's stale-lock correction): docs/how-it-works.md L170/L179
     already has the "lock never stale, FILES reaped" wording. README L341 describes CONTENTION (exit 0 vs
     Busy), NOT staleness — no over-claim to correct. Do NOT add a stale-lock paragraph to README. -->

<!-- GOTCHA (table row stays on one line): README L67 is a markdown table row (| ... |). The enhancement
     is a clause INSIDE the cell, before the link brackets — the whole row stays on one physical line. -->
<!-- GOTCHA (blockquote stays in the `>` block): configuration.md L160 is a `>` blockquote bullet. The
     inserted sentence must stay on the `>` line (or continue with `>` on the next line if wrapping). -->
<!-- GOTCHA (no other files): docs/cli.md and docs/providers.md have NO token_limit/closed-loop/stale-lock
     mentions — the v2.5 reliability work changed no CLI/config/provider surface. Don't edit them. -->
```

## Implementation Blueprint

### Data models and structure

No code. The two edits, as precise before→after:

```markdown
<!-- ── EDIT 1: README.md L67 (Payload optimization table row) ─────────────────────────── -->
<!-- BEFORE: -->
| Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |
<!-- AFTER: -->
| Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` — a closed-loop guarantee that the assembled prompt never exceeds the limit ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |

<!-- ── EDIT 2: docs/configuration.md L160 (token_limit bullet) ───────────────────────── -->
<!-- BEFORE: -->
> - **`token_limit`** (default `0` = unset) — a holistic token budget over the **whole** agent payload (system prompt + style examples + the concatenated diff). When set (e.g. `120000`), Stagehand reserves room for the prompt/examples and truncates the diff to fit using the ≈4 chars/token estimate, so the payload always fits your model's context window **without Stagehand maintaining a per-model context registry** (§9.1 FR3d). A non-zero `token_limit` **supersedes** the legacy per-section caps `max_diff_bytes` and `max_md_lines` for that run; the two modes are mutually exclusive. When `0`/unset, the legacy caps apply unchanged.
<!-- AFTER (insert ONE sentence after "truncates the diff to fit using the ≈4 chars/token estimate"): -->
> - **`token_limit`** (default `0` = unset) — a holistic token budget over the **whole** agent payload (system prompt + style examples + the concatenated diff). When set (e.g. `120000`), Stagehand reserves room for the prompt/examples and truncates the diff to fit using the ≈4 chars/token estimate; after truncation it assembles the actual full prompt, re-measures it, and re-trims until it fits — a closed-loop guarantee (§9.1 FR3j) that the payload never exceeds `token_limit`. The payload always fits your model's context window **without Stagehand maintaining a per-model context registry** (§9.1 FR3d). A non-zero `token_limit` **supersedes** the legacy per-section caps `max_diff_bytes` and `max_md_lines` for that run; the two modes are mutually exclusive. When `0`/unset, the legacy caps apply unchanged.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT README.md L67 (the closed-loop guarantee clause)
  - EDIT the Payload optimization table row: append " — a closed-loop guarantee that the assembled prompt
      never exceeds the limit" AFTER "via `token_limit`" and BEFORE the link brackets.
  - KEEP the row on one physical line (markdown table row). Do NOT restructure the row.
  - RUN: grep -n 'closed-loop' README.md → 1 hit (the new clause).

Task 2: EDIT docs/configuration.md L160 (the closed-loop sentence)
  - EDIT the token_limit bullet: insert the closed-loop sentence after "truncates the diff to fit using the
      ≈4 chars/token estimate" — "; after truncation it assembles the actual full prompt, re-measures it,
      and re-trims until it fits — a closed-loop guarantee (§9.1 FR3j) that the payload never exceeds
      `token_limit`."
  - STAY in the `>` blockquote block. Do NOT alter the surrounding sentences (the supersedes / mutual
      exclusivity / 0=unset sentences stay verbatim).
  - RUN: grep -n 'closed-loop' docs/configuration.md → 1 hit (the new sentence).

Task 3: CONFIRM the two no-ops
  - README lock scan: grep README.md for "stale\|never stale\|no stale\|lock.*never" — L341 describes
      contention (exit 0 vs Busy), NOT staleness. The stale-lock correction is in how-it-works.md L170/L179
      (already landed). Confirm: no over-claim to correct → no-op.
  - cli.md + providers.md scan: grep for token_limit/closed-loop/stale/reap — expect ZERO relevant hits.
      The v2.5 reliability work changed no CLI/config/provider surface → no-op.

Task 4: VERIFY
  - RUN markdownlint on the two edited files (if available): the edits are inline-code + plain prose
      additions on existing lines → compliant (MD013/MD033/MD060 disabled).
  - CONFIRM: PRD.md, go.mod, all `.go` files, docs/cli.md, docs/providers.md, docs/how-it-works.md
      byte-unchanged. Only README.md + docs/configuration.md modified.
```

### Implementation Patterns & Key Details

```markdown
<!-- THE guarantee to surface (from PRD §9.1 FR3j): "Invariant: EstimateTokens(assembledFullPrompt) ≤
     token_limit, always." The overview docs get ONE clause per doc:
     README: "a closed-loop guarantee that the assembled prompt never exceeds the limit"
     config: "assembles the actual full prompt, re-measures it, and re-trims until it fits — a closed-loop
              guarantee (§9.1 FR3j) that the payload never exceeds token_limit" -->
<!-- THE no-ops: README L341 (contention, not staleness — no over-claim); cli.md/providers.md (no surface). -->
```

### Integration Points

```yaml
DOCUMENTATION (Mode B): this IS the changeset-level doc sync for v2.5 (plan 011). It depends on P1.M1
      (FR3j closed-loop) + P1.M2 (§18.5 stale-lock reaping) — both Complete.

CODE: NONE. No .go file is touched. `go test ./...` stays green by construction (no code change).

FROZEN/LEAVE (do NOT edit):
  - docs/how-it-works.md (the stale-lock correction L170/L179 is already landed by P1.M2.T1.S2 Mode A).
  - docs/cli.md, docs/providers.md (no v2.5 surface change).
  - PRD.md, go.mod, Makefile, all .go files.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG.
```

## Validation Loop

### Level 1: The two edits landed correctly

```bash
# EDIT 1: README L67 — the closed-loop clause is present in the Payload optimization row:
grep -n 'closed-loop' README.md   # → 1 hit (L67)
# EDIT 2: configuration.md L160 — the closed-loop sentence is present in the token_limit bullet:
grep -n 'closed-loop' docs/configuration.md   # → 1 hit (L160)
# Both edits reference the FR3j guarantee; the surrounding text is unchanged.
```

### Level 2: The two no-ops confirmed

```bash
# README lock scan: no stale-lock over-claim (L341 is contention, not staleness):
grep -niE 'stale|never stale|no stale' README.md   # → 0 hits (no over-claim to correct)
# cli.md + providers.md: no v2.5 surface change:
grep -niE 'token_limit|closed.loop|stale|reap' docs/cli.md docs/providers.md   # → 0 relevant hits
```

### Level 3: Byte-unchanged guard (no scope creep)

```bash
git diff --name-only   # Expect ONLY README.md + docs/configuration.md.
git diff --exit-code PRD.md docs/cli.md docs/providers.md docs/how-it-works.md go.mod && echo "frozen files UNCHANGED (expected)"
git diff --name-only -- '*.go' | grep -q . && echo "UNEXPECTED .go change" || echo "no .go changed (expected)"
# markdownlint on the edited files (if available):
npx markdownlint-cli2 README.md docs/configuration.md 2>/dev/null || npx markdownlint README.md docs/configuration.md 2>/dev/null || echo "(markdownlint not installed; visually verify)"
```

### Level 4: Whole-repo still green (doc-only, but confirm no accidental code touch)

```bash
go build ./...   # Expect clean (no code changed).
go test ./...    # Expect all PASS (doc-only task; belt-and-suspenders). If RED, a code file was edited by mistake.
```

## Final Validation Checklist

### Technical Validation
- [ ] README.md L67 mentions the closed-loop guarantee (1 hit for `closed-loop`).
- [ ] docs/configuration.md L160 states the closed-loop re-measure/re-trim invariant (1 hit).
- [ ] `go build ./... && go test ./...` GREEN (doc-only; no code touched).
- [ ] PRD.md / docs/cli.md / docs/providers.md / docs/how-it-works.md / all `.go` files byte-unchanged.

### Feature Validation
- [ ] Both enhancements surface the FR3j guarantee ("never exceeds `token_limit`") without re-stating the algorithm.
- [ ] No stale-lock over-claim in README (L341 is contention, not staleness — confirmed no-op).
- [ ] cli.md + providers.md have no v2.5 surface change (confirmed no-op).

### Code Quality Validation
- [ ] Enhancements are ONE clause/sentence each — surgical, not a re-algorithm-ization.
- [ ] Table row stays on one line; blockquote stays in the `>` block (markdownlint-compliant).
- [ ] No duplication of the how-it-works.md stale-lock correction.

### Documentation
- [ ] The enhancements cite §9.1 FR3j (the closed-loop spec) for the reader who wants the full algorithm.

---

## Anti-Patterns to Avoid

- ❌ **Don't re-state the FR3j algorithm.** The overview docs get ONE clause ("never exceeds the limit"),
      not the water-fill + re-measure loop. The full algorithm lives in the PRD §9.1 FR3i/FR3j + how-it-works.md.
- ❌ **Don't add a stale-lock correction to README.** docs/how-it-works.md L170/L179 already has it (Mode A,
      P1.M2.T1.S2). README L341 describes contention (exit 0 vs Busy), not staleness — no over-claim to correct.
- ❌ **Don't edit cli.md, providers.md, or how-it-works.md.** cli.md/providers.md have no v2.5 surface;
      how-it-works.md's stale-lock correction is already landed. This task is README.md + configuration.md ONLY.
- ❌ **Don't restructure the table row or the blockquote.** The README L67 row is a markdown table row (pipes,
      one line); the configuration.md L160 bullet is a `>` blockquote. The enhancement is a clause/sentence
      INSIDE the existing structure, not a restructure.
- ❌ **Don't touch any code/PRD/go.mod.** Scope is the two docs only.

---

## Confidence Score

**10/10** — a 2-edit doc enhancement whose exact before→after wording is specified verbatim (the closed-loop
clause for README L67 + the closed-loop sentence for configuration.md L160), grounded in the PRD §9.1 FR3j
spec (in your context), with both no-ops pre-verified by grep (README L341 = contention not staleness; cli.md/
providers.md = no v2.5 surface). The parallel P1.M2.T3.S2 touches only `internal/signal/signal.go` (zero file
overlap). The edits are one clause + one sentence on existing lines, markdownlint-compliant (MD013/MD033/MD060
disabled). No residual risk.
