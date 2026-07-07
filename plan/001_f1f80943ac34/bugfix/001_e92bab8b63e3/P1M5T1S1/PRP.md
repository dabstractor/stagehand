---
name: "Sync README.md — config precedence, dry-run, agent configuration (Mode B)"
work_item: P1.M5.T1.S1
kind: documentation (Mode B — changeset-level doc sync)
depends_on:
  - P1.M1.T1 (Issues 1 & 5: CLI↔pkg config double-load eliminated; `--config` honored by default action)  ✅ Complete
  - P1.M2.T1 (Issue 3: provider command pre-flight fail-fast, exit 1)                                   ✅ Complete
  - P1.M3.T1 (Issues 2 & 6: dry-run runs full snapshot→generate→parse→dedupe→retry pipeline)            ✅ Complete
  - P1.M4.T1 (Issue 4: `[generation] output`/`strip_code_fence` applied onto the resolved manifest)      ✅ Complete
  - P1.M4.T2 (Issue 7: clean-tree auto-stage notice)                                                    ✅ Complete (not user-facing in README)
---

## Goal

**Feature Goal**: Make `README.md`'s cross-cutting, product-summarizing blurbs **consistent with the
shipped behavior** of the M1–M4 bugfix changeset. The Mode-A docs (`docs/cli.md`,
`docs/configuration.md`, `docs/how-it-works.md`) and the `config init` template
(`internal/cmd/config.go`) have **already** been synced to ride with the implementing subtasks;
`README.md` is the last Mode-B surface. Every README claim must agree with those already-correct
docs and with the binary.

**Deliverable**: An edited `README.md` (prose only — **no code changes anywhere**) in which five
specific blurbs no longer drift from shipped behavior, verified by (a) `markdownlint-cli2` passing
with 0 errors, and (b) a coherence check against `docs/cli.md` / `docs/configuration.md`.

**Success Definition**:

- `npx markdownlint-cli2 README.md` → `0 error(s)`.
- The five drift points below are resolved with the **canonical wording** already present in the
  shipped docs (quoted inline) — the README must not invent new claims.
- All in-README anchors/badges resolve (CI badge workflow path exists; `#the-snapshot-workflow` and
  `#adding-a-new-agent` anchors still match their headings).
- **Minimal edits only** — per the work-item contract: *"Edit only where text is now inaccurate or can
  be strengthened; do not pad."* Do NOT rewrite whole sections; touch the specific lines below.

## Why

- `README.md` is the project's front door. Its blurbs summarize the whole product in a few sentences;
  once a behavioral changeset lands, any stale summary becomes a **contract lie** to a new user.
- The bug-fix pass resolved four user-journey deviations (`--config`, `--dry-run`, misconfigured
  provider, `[generation]` tuning). The README currently either omits these features or describes the
  old (buggy) behavior.
- This is the **only** documentation task scoped to README (P1.M5.T1.S2 covers `docs/*.md` overview
  sweeps separately). Stay within `README.md`.

## What

Five prose edits to `README.md`, each mapped to a shipped fix. For each: the **current line**, the
**drift**, the **canonical wording to mirror** (copy the substance, do not copy verbatim if a shorter
README phrasing reads better), and whether it is a *fix-inaccuracy* or a *strengthen*.

### Drift map (line numbers from the current `README.md`)

| # | README location (line) | Blurb | Issue(s) | Drift | Type |
|---|------------------------|-------|----------|-------|------|
| 1 | Quick-start `--dry-run` comment (~64–67) | "Preview the message without committing" | 2, 6 | Doesn't reflect FR49: dry-run runs the **full** pipeline (snapshot + generate + parse + **duplicate-check** + retry) and previews the **exact** message a real commit would make | Strengthen |
| 2 | "Configure your agent" — `--config` flag (absent) | README never mentions `--config` at all | 1 | `--config <file>` is now honored by **every** command incl. the default commit action (previously documented-but-broken for the default action) | Fix-inaccuracy |
| 3 | **Config precedence** line (~115) | precedence list is correct but silent on `--config` | 1 | Should state `--config <file>` overrides config-file discovery and is honored by the default action | Strengthen |
| 4 | "Configure your agent" → `config init` (~104–110) | template's `[generation]` knobs not mentioned | 4 | The `config init` template's `[generation] output` / `strip_code_fence` now **apply** (tune parsing across all providers, override per-provider) | Fix-inaccuracy |
| 5 | "Configure your agent" / "Adding a new agent" | no mention of missing-command behavior | 3 | A provider whose `command` is not on `$PATH` **fails fast with exit 1 before the snapshot** | Strengthen |

> The community-agent `[provider.myagent]` block (~154, "Adding a new agent") is referenced in the
> work-item's research note. Its fields stay **valid as-is** (per-provider `output = "raw"` still
> works; it is merely overridden by a `[generation]` value if one is set). No edit required there
> unless you choose to add a one-line cross-reference to `[generation]` for output tuning — keep it
> optional and do not pad.

## All Needed Context

### Context Completeness Check

**Pass**: this PRP quotes the exact README lines, the exact canonical doc wording, the lint command, and
the gotchas. An agent that has never seen this repo can complete it from this file + `README.md` +
`docs/cli.md` + `docs/configuration.md`.

### Documentation & References

```yaml
# MUST READ — the file being edited
- file: README.md
  why: THE edit target. Read it fully before touching anything.
  pattern: GitHub-flavored Markdown with `> [!NOTE]` admonitions and HTML comments.
  gotcha: |
    - markdownlint config (.markdownlint.json) disables MD013 (line length) and MD033 (inline HTML),
      so the existing `<!-- TODO -->` comments and long lines are fine — keep that style.
    - Keep edits minimal/non-padding (contract requirement).

# MUST READ — canonical source of truth for wording (already synced in Mode A)
- file: docs/cli.md
  why: the authoritative CLI reference, already aligned with shipped behavior.
  sections:
    - line 26: `--dry-run` description = "Run the full generate→parse→duplicate-check pipeline (same
      as a real commit, including retry) and print the message; do not commit"  → mirror for drift #1
    - line 30: "`--config` is honored by every command — including the default commit action, so a
      user-defined provider declared under `[provider.<name>]` in that file is usable with
      `--provider <name>` on `stagecoach` directly"  → mirror for drift #2 / #3
    - lines 80–81: exit-code table — `1` = "...**provider command missing on `$PATH` (checked before
      the snapshot)**..."  → mirror for drift #5
  critical: Do not contradict docs/cli.md. If you can't phrase something shorter for the README,
            copy the docs/cli.md substance.

- file: docs/configuration.md
  why: canonical config reference; already aligned with shipped behavior.
  sections:
    - lines 8–9: precedence list (identical to README's, confirms README list is already correct)
    - line 81: "The `output` and `strip_code_fence` settings apply to **parsing** of agent output...
      These `[generation]` values override any per-provider `[provider.<name>]` defaults — the broader
      layer wins."  → mirror for drift #4
  critical: Issue 4 was fixed by APPLYING these knobs (decisions.md D4), NOT by removing them.

# MUST READ — the shipped behavior (proof the docs are right)
- file: internal/cmd/config.go
  why: the `config init` template (`exampleConfigTemplate`). Lines ~154–163 already document the
        `[generation] output` / `strip_code_fence` knobs and the NOTE that they override per-provider
        values. The README's `config init` blurb can safely claim the template documents `[generation]`
        tuning that now applies.
  pattern: const string literal; do not edit it (out of scope — it is already correct).

# SUPPORTING — why each fix was made (rationale + rejected alternatives)
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: D1 (Issue 1/5 resolved-config injection), D2 (Issue 3 pre-flight), D3 (Issue 2/6 dry-run full
        loop), D4 (Issue 4 apply generation knobs — confirms APPLY not REMOVE), D6 (this very doc plan).
  critical: D6 explicitly lists the README blurbs this task owns. Read D6 first.

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/system_context.md
  why: §7 "Documentation surface touched" maps every Mode-A/Mode-B file to its issues.
  section: "## 7. Documentation surface touched"
```

### Current Codebase tree (documentation surface only)

```bash
README.md                      # ← THE edit target (Mode B)
docs/
├── cli.md                     # Mode A — already synced (source of truth for wording)
├── configuration.md           # Mode A — already synced
├── how-it-works.md            # Mode A — already synced (P1.M5.T1.S2 may sweep it)
├── providers.md
└── README.md
internal/cmd/config.go         # config init template — already synced (DO NOT EDIT here)
.markdownlint.json             # lint config: default=true, MD013/MD033/MD060 off
.github/workflows/ci.yml       # the CI badge in README points here (file exists ✓)
```

### Desired Codebase tree

```bash
# No files added, no files moved, no files deleted.
README.md                      # ← EDITED (5 prose touches); everything else untouched
```

### Known Gotchas of our codebase & library quirks

```text
# CRITICAL: markdownlint is NOT wired into the Makefile or CI (ci.yml).
# The ONLY way to validate is the manual command:
#   npx markdownlint-cli2 README.md
# (markdownlint-cli2 v0.22.1 / markdownlint v0.40.0 is cached and available via npx.)
# Do NOT assume CI will catch markdown errors — run it yourself.

# CRITICAL: .markdownlint.json disables MD013 (line-length) and MD033 (inline HTML),
# and lists "MD060" (which is NOT a standard markdownlint rule — it is silently ignored,
# so it has no effect). Keep the existing `> [!NOTE]` and `<!-- TODO -->` style.

# GOTCHA: This is a PROSE-ONLY task. There are no Go unit tests for README content.
# Validation is: (1) markdownlint clean, (2) coherence vs docs/*.md, (3) anchor/badge integrity.
# Do NOT "improve" code, config, or docs/*.md — those are owned by other tasks / already done.

# GOTCHA: The README hero + badge use the path "dustin/stagecoach" and the workflow
# ".github/workflows/ci.yml". That file EXISTS — do not touch the badge.

# GOTCHA: The precedence ORDER in the README line (~115) is ALREADY CORRECT and matches
# docs/configuration.md lines 8-9 AND PRD §16.1. Do NOT reorder it. Only ADD the --config note.

# GOTCHA: Issue 4 was fixed by APPLYING the [generation] knobs onto the manifest (decisions.md D4),
# i.e. the documented capability is now REAL. Do NOT reframe output/strip_code_fence as
# "per-manifest only" — that was the REJECTED alternative (option b). The README must reflect
# that [generation] tuning applies across all providers.
```

## Implementation Blueprint

### Implementation Tasks (ordered — edit README top-to-bottom so line numbers stay predictable)

```yaml
Task 1: STRENGTHEN the Quick-start --dry-run comment  (drift #1, Issues 2/6)
  - FILE: README.md, the "## Quick start" fenced bash block, the 4th item (~lines 64-67).
  - CURRENT:
        # 4. Preview the message without committing
        stagecoach --dry-run
  - CHANGE TO (substance, mirror docs/cli.md line 26): make the comment state that --dry-run runs
    the FULL pipeline — snapshot, generate, parse, duplicate-check, and retry — and prints the exact
    message a real commit would produce; it only skips creating the commit. Keep it ONE compact line.
  - EXAMPLE (adjust phrasing to taste, do not pad):
        # 4. Preview the real message (full pipeline: snapshot→generate→parse→dedupe→retry), no commit
        stagecoach --dry-run
  - GOTCHA: must imply the message is identical to a real commit (FR49). Do NOT claim it is faster
    or partial.

Task 2: FIX-INACCURACY — document --config and that the default action honors it  (drifts #2 & #3, Issue 1)
  - FILE: README.md, "## Configure your agent", near the **Config precedence** line (~115).
  - CURRENT: README never mentions --config. The precedence blurb reads:
        **Config precedence** (highest → lowest): CLI flags > `STAGECOACH_*` env vars > repo
        `git config` (`stagecoach.*`) > repo `.stagecoach.toml` > global config file > provider
        defaults > built-in defaults.
  - CHANGE: Keep the precedence list UNCHANGED (it is already correct). Add ONE short sentence
    (above or below it) stating that `--config <file>` overrides config-file discovery and is honored
    by EVERY command — including the default `stagecoach` commit action — so a provider declared under
    `[provider.<name>]` in that file is usable with `--provider <name>` directly.
    Mirror docs/cli.md line 30 substance.
  - EXAMPLE (compact):
        Point discovery at a specific file with `stagecoach --config path/to/config.toml`. It is
        honored by every command — including the default commit action — so a provider declared under
        `[provider.<name>]` there is usable with `--provider <name>` directly.
  - GOTCHA: `--config` is a discovery override, NOT a config field; `STAGECOACH_CONFIG` is its env
    equivalent. Do not imply it sets any other value.

Task 3: FIX-INACCURACY — note [generation] tuning now applies  (drift #4, Issue 4)
  - FILE: README.md, "## Configure your agent", the `config init` block (~104-110).
  - CURRENT:
        Or write a fully-commented global config file:
        stagecoach config init
        # Wrote example config to ~/.config/stagecoach/config.toml
  - CHANGE: Add a short sentence noting the generated template includes a `[generation]` section
    whose `output` ("raw"|"json") and `strip_code_fence` knobs tune how Stagecoach parses agent output
    across ALL providers (overriding per-provider values). Mirror docs/configuration.md line 81.
  - EXAMPLE:
        The template also documents a `[generation]` section: `output` ("raw"|"json") and
        `strip_code_fence` tune how Stagecoach parses agent output across all providers (overriding
        per-provider values).
  - GOTCHA: This is the APPLY-not-REMOVE outcome (decisions.md D4). Frame it as a working capability.

Task 4: STRENGTHEN — missing provider command fails fast (exit 1)  (drift #5, Issue 3)
  - FILE: README.md, "## Configure your agent" (next to the `providers list` DETECTED column) and/or
          "## Adding a new agent" (next to `stagecoach --provider myagent`).
  - CURRENT: providers list shows a DETECTED column; no statement of what happens if a provider's
             command is missing.
  - CHANGE: Add ONE line: if the resolved provider's command isn't on `$PATH`, Stagecoach fails fast
    with exit 1 BEFORE taking any snapshot (a pre-flight check) — it does not arm a rescue.
    Mirror docs/cli.md exit-code table line 81.
  - EXAMPLE (near the providers list or the --provider example):
        A provider whose command isn't on `$PATH` fails fast with exit 1 before any snapshot — no
        partial state, no rescue recipe.
  - GOTCHA: Pair this with the existing DETECTED column narrative (providers list shows what's
            installed). Do not change exit-code numbers anywhere.

Task 5: VERIFY (no edit unless padding is warranted) — the [provider.myagent] block  (research note)
  - FILE: README.md, "## Adding a new agent" (~148-176).
  - CURRENT: a TOML block including `output = "raw"            # raw | json`.
  - ACTION: Leave AS-IS. The per-provider `output` field is still valid (it is the per-provider
            default; a `[generation] output` would override it — covered by Task 3).
  - OPTIONAL (only if it reads naturally): one-line cross-ref that `[generation] output` tunes output
    mode across all providers. Do NOT force this — "do not pad".

Task 6: NO-OP for Issue 7
  - Issue 7 (clean-tree "(0 files)" notice) is an internal UX detail with no README presence and no
    user-facing contract surface in the README. Do NOT add anything for it.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN: keep README blurbs SHORT and self-consistent with docs/.
# Every behavioral claim in README should be a strict subset of (and worded identically to) docs/.
# If unsure of wording, quote docs/cli.md / docs/configuration.md directly.

# PATTERN: GitHub admonitions are already used (`> [!NOTE]`). Reuse them for the new notes rather
# than introducing a new callout style. markdownlint allows them under the current config.

# CRITICAL: do NOT reorder the Config precedence list — it is verified-correct (matches
# docs/configuration.md:8-9 and PRD §16.1). Only ADD the --config sentence.
```

### Integration Points

```yaml
NONE — this is a single-file prose edit. No config, routes, database, or code integration.
The only "integration" is textual consistency with the already-synced docs/*.md and config init template.
```

## Validation Loop

> This is a documentation task. There are no Go tests, no build step, and no server. Validation is
> lint + coherence + integrity. **markdownlint is NOT in CI** — you must run the commands below manually.

### Level 1: Markdown lint (the primary gate)

```bash
# From the repo root. MUST show "Summary: 0 error(s)".
npx markdownlint-cli2 README.md

# (Optional) lint all docs together to confirm you didn't accidentally need a docs/ change:
npx markdownlint-cli2 "**/*.md"
# Expected: 0 error(s). If docs/*.md fail, that is NOT your task (P1.M5.T1.S2 owns docs/) —
# flag it, do not fix docs/ here.
```

### Level 2: Coherence check vs the already-synced docs

```bash
# Each README behavioral claim must agree with docs/. Quick greps to confirm no contradiction:
# --config honored by default action (Issue 1):
grep -n "default commit action\|every command\|--config" README.md docs/cli.md
# dry-run = full pipeline (Issues 2/6):
grep -n "duplicate-check\|full pipeline\|same as a real commit" README.md docs/cli.md
# missing provider → exit 1 (Issue 3):
grep -n "exit 1\|missing on\|checked before the snapshot\|fails fast" README.md docs/cli.md
# [generation] output/strip_code_fence apply (Issue 4):
grep -n "generation\|override any per-provider\|across all providers\|strip_code_fence" README.md docs/configuration.md
# Expected: README and docs agree in substance. Resolve any disagreement by aligning README to docs/.
```

### Level 3: Anchor & badge integrity

```bash
# CI badge workflow path exists:
test -f .github/workflows/ci.yml && echo "badge target OK"

# In-README anchor targets still resolve to real headings:
grep -n "^## The snapshot workflow$" README.md   # anchor #the-snapshot-workflow
grep -n "^## Adding a new agent$"   README.md   # anchor #adding-a-new-agent
# Any NEW internal links you add must also resolve (lowercase, spaces→hyphens, strip punctuation).
```

### Level 4: Build sanity (no code touched — confirm you didn't break the build by accident)

```bash
# If you ONLY edited README.md, this is a no-op confirmation. Run it to be safe.
go build ./... && go vet ./... && go test ./... 2>&1 | tail -5
# Expected: clean build/vet and all tests pass. (You should not have touched any .go file.)
```

## Final Validation Checklist

### Technical / Lint

- [ ] `npx markdownlint-cli2 README.md` → `0 error(s)`
- [ ] No other files modified (`git status --short` shows ONLY `README.md`)
- [ ] `go build ./... && go vet ./... && go test ./...` still pass (sanity — no code touched)

### Feature / Coherence

- [ ] Drift #1: `--dry-run` comment reflects the FULL pipeline (snapshot→generate→parse→dedupe→retry)
- [ ] Drift #2 & #3: `--config <file>` documented as honored by the default commit action
- [ ] Drift #4: `[generation] output`/`strip_code_fence` noted as applying across all providers
- [ ] Drift #5: missing provider command fails fast (exit 1, before snapshot) documented
- [ ] Config precedence list UNCHANGED (still matches `docs/configuration.md` / PRD §16.1)
- [ ] Every README behavioral claim agrees with `docs/cli.md` / `docs/configuration.md`

### Integrity

- [ ] CI badge workflow path (`.github/workflows/ci.yml`) still exists
- [ ] All in-README anchors (`#the-snapshot-workflow`, `#adding-a-new-agent`) resolve
- [ ] Any newly added internal links resolve

### Style / Discipline

- [ ] Minimal edits only — no padding, no whole-section rewrites (contract requirement)
- [ ] Reuses existing `> [!NOTE]` admonition style; no new callout format introduced
- [ ] Did NOT edit `docs/*.md` (owned by P1.M5.T1.S2), `internal/cmd/config.go`, `PRD.md`, or any code

---

## Anti-Patterns to Avoid

- ❌ Don't reorder or rewrite the Config precedence list — it is verified-correct; only add the `--config` note.
- ❌ Don't reframe `[generation] output`/`strip_code_fence` as "per-manifest only" — that was the REJECTED fix (decisions.md D4 chose APPLY). The README must reflect a working capability.
- ❌ Don't invent behavioral claims not present in `docs/cli.md` / `docs/configuration.md` — the README is a subset/summary, never a new source of truth.
- ❌ Don't edit `docs/*.md`, `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file — out of scope.
- ❌ Don't skip the manual `npx markdownlint-cli2 README.md` run assuming CI will catch it — markdownlint is NOT in CI.
- ❌ Don't pad — the contract explicitly says "edit only where text is now inaccurate or can be strengthened."
- ❌ Don't add anything for Issue 7 — it has no README surface.
