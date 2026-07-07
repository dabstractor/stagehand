---
name: "Sweep docs/*.md overview sections for changeset coherence (Mode B)"
work_item: P1.M5.T1.S2
kind: documentation (Mode B — changeset-level doc coherence sweep over docs/*.md)
depends_on:
  - P1.M1.T1 (Issues 1 & 5: `--config` honored by the default action)                         ✅ Complete (Mode-A edit landed in docs/cli.md)
  - P1.M2.T1 (Issue 3: provider command pre-flight fail-fast, exit 1)                          ✅ Complete (Mode-A edits in docs/cli.md + docs/how-it-works.md)
  - P1.M3.T1 (Issues 2 & 6: dry-run runs the full snapshot→generate→parse→dedupe pipeline)      ✅ Complete (Mode-A edit in docs/cli.md)
  - P1.M4.T1 (Issue 4: `[generation] output`/`strip_code_fence` applied onto the manifest)      ✅ Complete (Mode-A edit in docs/configuration.md)
  - P1.M4.T2 (Issue 7: clean-tree auto-stage notice — no docs surface)                          ✅ Complete (no doc edit)
  - P1.M5.T1.S1 (README.md sync — Mode B sibling)                                               ✅ Complete
---

## Goal

**Feature Goal**: Make the `docs/*.md` **overview/reference prose** consistent with the shipped
behavior of the M1–M4 bugfix changeset. The implementing subtasks already made their **per-issue
Mode-A** edits (each touched only the single doc row its issue lives in). This task is the final
**cross-cutting sweep**: verify every overview-level claim still agrees with the shipped binary and
with the already-correct Mode-A rows, and patch the **handful of overview gaps the Mode-A edits did
not reach** — without duplicating their work.

**Deliverable**: Edited `docs/configuration.md` (primary) and, conditionally, `docs/providers.md`
and/or `docs/cli.md` (recommended/optional polish). Prose only — **no `.go` file is touched
anywhere**. Verified by (a) `markdownlint-cli2` clean across `docs/`, (b) coherence greps showing
the docs agree with `docs/cli.md` (the already-synced source of truth) and the binary, and (c) a
`go build/vet/test` sanity pass confirming no code was disturbed.

**Success Definition**:

- `npx markdownlint-cli2 'docs/**/*.md'` → `0 error(s)`.
- The four contract coherence checks (item LOGIC) all hold post-sweep:
  1. failure-modes table lists agent-missing → exit 1 (Issue 3) — *already present; verify.*
  2. dry-run overview states full pipeline + snapshot (Issues 2/6) — *already present by reference;
     optionally made explicit.*
  3. config overview states `[generation]` knobs apply (Issue 4) — *already present; verify.*
  4. config overview states `--config` honored everywhere incl. the default action (Issue 1) —
     **the one real gap: add the prose note to `docs/configuration.md`.**
- No stale claim anywhere that dry-run skips the snapshot or the duplicate-check (verified by grep).
- **Minimal edits only** — per the work-item contract: *"Fix only drift the implementing subtasks
  did not already cover; do not duplicate their edits."* Do NOT re-touch the exact lines the Mode-A
  subtasks already edited (listed below) except for the one sanctioned `--dry-run` clarity polish.

## Why

- `docs/*.md` is the canonical reference set the README's "see docs/" link promises (see
  `docs/README.md` NOTE). Once a behavioral changeset ships, any overview that still omits or
  contradicts the new behavior is a **contract lie** to a user reading the reference.
- The Mode-A edits were deliberately **per-issue / per-row** (each subtask touched only the doc cell
  its bug lives in). They left a few **cross-cutting overview spots** that span the whole changeset
  un-touched — most notably the config-reference prose, which never tells a reader that `--config`
  points discovery at a file honored by the default action (the exact behavior Issue 1 fixed).
- This is the **only** task scoped to `docs/*.md` coherence. S1 owns `README.md` (done). Stay within
  `docs/*.md`; do not edit `README.md`, `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any
  `.go` file.

## What

A verification pass + up to three targeted prose edits. Each maps to a **shipped fix** and to a
specific coherence check the work-item contract enumerates. The table records what Mode-A **already
did** (do not repeat it) vs. what this task adds.

### Mode-A coverage map (DO NOT duplicate these — they are done)

| Issue | File | Mode-A edit (already landed) | Commit |
|-------|------|------------------------------|--------|
| 1 | `docs/cli.md:30` | "`--config` is honored by every command — including the default commit action…" | 1368895 |
| 3 | `docs/cli.md:81` | exit-code row: "**provider command missing on `$PATH` (checked before the snapshot)**" | 9b8dc15 |
| 3 | `docs/how-it-works.md:57` | failure table row: "Agent missing on `$PATH` \| 1 (Error)" | 9b8dc15 |
| 2,6 | `docs/cli.md:26` | `--dry-run` desc: "full generate→parse→duplicate-check pipeline (same as a real commit, including retry)…" | f8db87e |
| 4 | `docs/configuration.md:~81` | "`output` and `strip_code_fence` … apply to parsing … override any per-provider `[provider.<name>]` defaults" + `stagecoach.output`/`stagecoach.stripCodeFence` git-config rows | f1ca18a |

### This task's edits

| # | File (line) | Issue(s) | Drift | Type |
|---|-------------|----------|-------|------|
| A | `docs/configuration.md` (~31, "Config file paths" section) | 1 | Config-reference **overview prose** never mentions `--config <path>`/`STAGECOACH_CONFIG` as a discovery override honored by the default action (only the env-var table row names `STAGECOACH_CONFIG`; the precedence + paths sections a user reads to understand discovery omit it entirely) | **REQUIRED** |
| B | `docs/providers.md` (~122, "Output parsing" section) | 4 | Manifest reference documents `output`/`strip_code_fence` as per-provider manifest fields but never notes a `[generation] output`/`strip_code_fence` value **overrides** them (the exact capability Issue 4 shipped) | RECOMMENDED |
| C | `docs/cli.md` (line 26, `--dry-run` row) | 6 | The dry-run enumeration lists "generate→parse→duplicate-check" but omits the **snapshot** step; covered today only by the phrase "same as a real commit" (accurate by reference, implicit on the snapshot) | OPTIONAL polish |

> **Scope discipline.** A is the primary deliverable and is unambiguously in scope (the contract
> explicitly lists "config overview states `--config` honored everywhere incl. the default action"
> as a coherence check, and Mode-A covered it only in `docs/cli.md`, not the config reference). B
> and C are coherence-strengthening; do them only if they read naturally and do not pad — the
> contract says edit where text "is now inaccurate or can be strengthened."

## All Needed Context

### Context Completeness Check

**Pass**: this PRP quotes the exact current text of every edit target, the exact canonical wording
to mirror (from the already-synced `docs/cli.md`), the lint command, the Mode-A coverage map (so you
don't duplicate), and the gotchas. An agent that has never seen this repo can complete it from this
file + the four `docs/*.md` files.

### Documentation & References

```yaml
# MUST READ — the files being swept (read each fully before editing)
- file: docs/configuration.md
  why: PRIMARY edit target (Edit A). The config reference.
  edit_at: "Config file paths" section, after the "Use `stagecoach config path` … `config init`…" line (~31).

- file: docs/cli.md
  why: (1) OPTIONAL edit target (Edit C, --dry-run row line 26);
       (2) the SOURCE OF TRUTH for wording — already synced by Mode-A. Mirror it, do not contradict it.
  source_of_truth_lines:
    - "30: `--config` is honored by every command — including the default commit action…" → mirror for Edit A
    - "81: provider command missing on `$PATH` (checked before the snapshot)" → coherence target
    - "26: full generate→parse→duplicate-check pipeline (same as a real commit, including retry)" → Edit C target / coherence target

- file: docs/providers.md
  why: RECOMMENDED edit target (Edit B). The manifest reference.
  edit_at: "Output parsing" section, after the final "The v1 default is `output = \"raw\"`…" line (~122).

- file: docs/how-it-works.md
  why: VERIFY ONLY (no edit expected). Confirms the Issue-3 failure-modes row (line 57) and that the
       rescue-protocol / snapshot-invariant prose is internally consistent with the pre-flight.
  no_edit_expected: true

- file: docs/README.md
  why: VERIFY ONLY. The docs index. "11 global flags / 7-layer precedence / 18-field schema" must stay accurate.

# MUST READ — why each fix was made + the doc plan
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: D6 ("Documentation (SOW §5) plan") maps every Mode-A/Mode-B file to its issues and is the
       authority on what was supposed to land where. Read D6 + D1/D4 first.
  section: "## D6 — Documentation (SOW §5) plan"

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/system_context.md
  why: §7 "Documentation surface touched" maps each doc to its issues (drives the Mode-A/Mode-B split).
  section: "## 7. Documentation surface touched"

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M5T1S2/research/mode-a-coverage-and-remaining-drift.md
  why: the git-diff audit of exactly what Mode-A already changed and the resulting drift checklist.

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M5T1S1/PRP.md
  why: the sibling Mode-B task (README). Its wording and "do not pad / minimal edits" discipline apply here too.
```

### Current Codebase tree (documentation surface only)

```bash
docs/
├── README.md            # index — VERIFY ONLY
├── cli.md               # source of truth (Mode-A synced) + OPTIONAL Edit C (line 26)
├── configuration.md     # PRIMARY Edit A (~31)
├── how-it-works.md      # VERIFY ONLY (Mode-A synced Issue 3)
└── providers.md         # RECOMMENDED Edit B (~122)
README.md                # ← DONE by S1 — do NOT edit
internal/cmd/config.go   # config init template — already synced (DO NOT EDIT)
.markdownlint.json       # lint config: default=true, MD013/MD033/MD060 off
.github/workflows/ci.yml # CI (markdownlint is NOT wired in here)
```

### Desired Codebase tree

```bash
# No files added, moved, or deleted.
docs/configuration.md    # ← EDITED (Edit A — one prose note)
docs/providers.md        # ← EDITED (Edit B — one prose note; skip if it pads)
docs/cli.md              # ← EDITED (Edit C — one-word/phrase polish on line 26; optional)
# everything else untouched
```

### Known Gotchas of our codebase & library quirks

```text
# CRITICAL: markdownlint is NOT wired into the Makefile or CI (.github/workflows/ci.yml).
# The ONLY way to validate is the manual command:
#   npx markdownlint-cli2 'docs/**/*.md'
# (markdownlint-cli2 v0.22.1 / markdownlint v0.40.0 is cached and available via npx.)
# Do NOT assume CI will catch markdown errors — run it yourself. Verified: docs/cli.md → 0 errors.

# CRITICAL: .markdownlint.json disables MD013 (line-length) and MD033 (inline HTML), and lists
# "MD060" (NOT a standard markdownlint rule — silently ignored, no effect). Long lines and
# `> [!NOTE]` admonitions are fine; reuse the existing admonition style rather than inventing one.

# CRITICAL: This is a PROSE-ONLY task. There are no Go tests for docs content. Validation is:
# (1) markdownlint clean, (2) coherence greps vs docs/cli.md (source of truth), (3) build sanity.
# Do NOT "improve" code, config init template, README, or PRD.

# GOTCHA: docs/cli.md is the SOURCE OF TRUTH — it was synced by Mode-A for Issues 1/2/3/6.
# Every behavioral claim you add elsewhere must be a strict subset of (and worded identically to)
# docs/cli.md. If unsure, quote docs/cli.md. Do NOT contradict it.

# GOTCHA: Do NOT re-edit the exact Mode-A lines except the sanctioned Edit C. The --config
# honored-by-default-action note already exists at docs/cli.md:30 — Edit A ADDS the equivalent to
# configuration.md's overview (different file, not a duplicate), it does not repeat cli.md.

# GOTCHA: Issue 4 was fixed by APPLYING the [generation] knobs onto the manifest (decisions.md D4),
# i.e. the documented capability is now REAL and BROADER than per-provider. Do NOT reframe
# output/strip_code_fence as "per-manifest only" — that was the REJECTED alternative. Edit B must
# frame [generation] as the overriding, broader setting (consistent with configuration.md:~81).

# GOTCHA: The dry-run snapshot (Issue 6) is a harmless dangling tree object — docs must NOT imply
# dry-run is "cheaper" or "partial". It runs the FULL pipeline and only skips commit-tree/update-ref.

# GOTCHA: docs/how-it-works.md Snapshot invariant #3 ("a failed generation leaves … only orphan
# tree/commit objects") is about POST-snapshot failures. The Issue-3 pre-flight is PRE-snapshot
# (no snapshot, no orphan). These are consistent — do NOT "fix" invariant #3; it is correct.
```

## Implementation Blueprint

### Implementation Tasks (ordered — smallest/required first)

```yaml
Task 1 (REQUIRED, Edit A): configuration.md — document the --config discovery override
  Issue: 1
  FILE: docs/configuration.md
  WHERE: "## Config file paths" section, immediately AFTER the line:
         "Use `stagecoach config path` to print the resolved global path. Use `stagecoach config init` to write a fully-commented example to the global path."  (~line 31)
  DRIFT: the precedence + config-paths overview prose never tells a reader they can point discovery
         at a specific file with --config / STAGECOACH_CONFIG, nor that it is honored by the default
         commit action (the exact behavior Issue 1 fixed). Only the env-var table row names it.
  ADD: ONE short sentence/paragraph. Mirror docs/cli.md:30 substance. Example (adjust phrasing,
       keep it compact — do NOT pad):
         Point discovery at a specific file with `--config <path>` (or `STAGECOACH_CONFIG`). It
         overrides global/repo-local file discovery and is honored by every command — including the
         default commit action — so a provider declared under `[provider.<name>]` there is usable
         with `--provider <name>` directly.
  GOTCHA: `--config` is a discovery override, NOT a Config field and NOT a layer value (it replaces
          which file Layer 3 reads). `STAGECOACH_CONFIG` is its env equivalent. Do not imply it sets
          any other value, and do not reorder the precedence list.

Task 2 (RECOMMENDED, Edit B): providers.md — note [generation] overrides manifest output/strip_code_fence
  Issue: 4
  FILE: docs/providers.md
  WHERE: "## Output parsing" section, immediately AFTER the final line:
         "The v1 default is `output = \"raw\"` — the agent's stdout, after cleanup, is the commit message verbatim."  (~line 122)
         (Alternative acceptable placement: a one-line footnote under the schema table's output /
          strip_code_fence rows — pick whichever reads more naturally; do not do both.)
  DRIFT: the manifest reference documents output/strip_code_fence as per-provider manifest fields but
         never mentions a [generation] value overrides them (Issue 4 capability, now shipped).
  ADD: ONE compact sentence. Cross-reference configuration.md. Example:
         A `[generation] output` / `strip_code_fence` value (config file or git-config) overrides
         these per-provider manifest defaults — the broader layer wins (see [configuration.md]).
  GOTCHA: Frame [generation] as the OVERRIDING/broader setting (configuration.md:~81, decisions.md D4
          chose APPLY, not REMOVE). Keep the per-provider fields valid as defaults. Do NOT duplicate
          the configuration.md paragraph verbatim — one cross-ref sentence is enough.

Task 3 (OPTIONAL, Edit C): cli.md — name the snapshot step in the --dry-run enumeration
  Issue: 6
  FILE: docs/cli.md
  WHERE: the `--dry-run` global-flag row (~line 26).
  CURRENT: "| `--dry-run` | bool | false | — | — | Run the full generate→parse→duplicate-check pipeline (same as a real commit, including retry) and print the message; do not commit |"
  DRIFT: the explicit step enumeration ("generate→parse→duplicate-check") omits the write-tree
         SNAPSHOT step. The phrase "same as a real commit, including retry" covers it by reference,
         so the line is already accurate — this is a clarity polish, not a correctness fix.
  CHANGE TO (minimal — add "snapshot" to the enumeration and to the "including" clause):
         "| `--dry-run` | bool | false | — | — | Run the full snapshot→generate→parse→duplicate-check pipeline (same as a real commit, including the write-tree snapshot and retry) and print the message; do not commit |"
  GOTCHA: This line was already edited by Mode-A (P1.M3.T1, commit f8db87e). Make ONLY the minimal
          addition named above; do NOT rewrite the row. If the change does not read cleanly, SKIP it
          (the current text is correct by reference) — do not force a worse phrasing.

Task 4 (VERIFY-ONLY pass): confirm no OTHER overview drift exists
  - Read each of docs/cli.md, docs/configuration.md, docs/how-it-works.md, docs/providers.md,
    docs/README.md end-to-end.
  - Confirm the four contract coherence checks all hold (see Validation Loop Level 2 greps).
  - Confirm NO stale claim that dry-run skips the snapshot or the duplicate-check (grep: 0 hits for
    "skips the snapshot", "single", "partial", "do not.*snapshot" in a dry-run context).
  - If you find drift beyond A/B/C, fix it minimally (mirror docs/cli.md). If you find none, do not
    invent edits (do not pad).
```

### Implementation Patterns & Key Details

```markdown
# PATTERN: docs/cli.md is the source of truth. Every behavioral claim elsewhere must agree with it.
# When adding prose, mirror docs/cli.md's substance (the --config note at line 30, the dry-run
# description at line 26, the exit-code row at line 81). Do not invent new behavioral claims.

# PATTERN: reuse the existing `> [!NOTE]` admonition style if a callout reads better than inline
# prose (configuration.md and docs/README.md already use it). markdownlint allows it (MD033 off).

# PATTERN: keep edits to ONE sentence/paragraph per drift point. The reference docs are dense;
# padding them harms readability and violates the "do not pad" contract.

# CRITICAL: do NOT reorder or rewrite the Precedence list in configuration.md — it is verified
# correct (matches docs/cli.md flag↔env↔git-config map and PRD §16.1). Only ADD the --config note.
```

### Integration Points

```yaml
NONE — this is a multi-file prose edit. No config, routes, database, or code integration.
The only "integration" is textual consistency: every edit mirrors the already-synced docs/cli.md and
the config init template (internal/cmd/config.go), both of which are correct and out of scope.
```

## Validation Loop

> This is a documentation task. No Go tests, no build step, no server. Validation is lint +
> coherence + integrity. **markdownlint is NOT in CI** — run the commands below manually.

### Level 1: Markdown lint (the primary gate)

```bash
# From the repo root. MUST show "Summary: 0 error(s)".
npx markdownlint-cli2 'docs/**/*.md'

# (Belt-and-suspenders) lint README + docs together to confirm you didn't disturb README:
npx markdownlint-cli2 '**/*.md'
# Expected: 0 error(s). If README.md fails, that is NOT your task (S1 owns it) — flag it, do not fix.
```

### Level 2: Coherence check vs the source of truth (docs/cli.md) + contract checks

```bash
# (1) --config honored by default action (Issue 1) — present in BOTH cli.md AND configuration.md now:
grep -rn "default commit action\|honored by every command\|--config" docs/cli.md docs/configuration.md
# Expected: cli.md:30 (Mode-A) AND a new configuration.md hit (Edit A).

# (2) dry-run = full pipeline incl snapshot (Issues 2/6) — no stale "skips snapshot" claim:
grep -rni "skips the snapshot\|single-pass\|single pass\|do not.*snapshot" docs/
# Expected: 0 hits.
grep -rn "snapshot→generate\|generate→parse→duplicate-check\|same as a real commit" docs/cli.md
# Expected: the --dry-run row; ideally names "snapshot" (Edit C) — acceptable if only "same as a real commit".

# (3) missing provider → exit 1 before snapshot (Issue 3) — present in BOTH tables:
grep -rn "Agent missing on\|checked before the snapshot\|exit 1\|fails fast" docs/cli.md docs/how-it-works.md
# Expected: cli.md:81 + how-it-works.md:57 (both Mode-A, unchanged).

# (4) [generation] output/strip_code_fence apply + override per-provider (Issue 4):
grep -rn "generation.*override\|override any per-provider\|override.*per-provider\|strip_code_fence" docs/configuration.md docs/providers.md
# Expected: configuration.md:~81 (Mode-A) AND a new providers.md hit (Edit B).

# Expected: docs agree in substance. Resolve any disagreement by aligning to docs/cli.md (source of truth).
```

### Level 3: Integrity — Mode-A rows untouched (except sanctioned Edit C), no scope creep

```bash
# Confirm only docs/*.md changed (no README, no .go, no PRD/tasks):
git status --short
# Expected (with all edits): M docs/cli.md  M docs/configuration.md  M docs/providers.md
# Acceptable subset (A only): M docs/configuration.md
# MUST NOT include: README.md, internal/cmd/config.go, PRD.md, tasks.json, *.go

# Confirm the Mode-A lines are intact (Edit C is the only sanctioned re-touch of a Mode-A line):
git show 1368895:docs/cli.md | sed -n '30p'   # the --config note — must still exist verbatim
git show 9b8dc15:docs/how-it-works.md | grep -n "Agent missing on"   # failure-table row intact
git show f1ca18a:docs/configuration.md | grep -n "override any per-provider"  # Issue-4 para intact
```

### Level 4: Build sanity (no code touched — confirm you didn't break the build by accident)

```bash
# If you ONLY edited docs/*.md, this is a no-op confirmation. Run it to be safe.
go build ./... && go vet ./... && go test ./... 2>&1 | tail -5
# Expected: clean build/vet and all tests pass. (You should not have touched any .go file.)
```

## Final Validation Checklist

### Technical / Lint

- [ ] `npx markdownlint-cli2 'docs/**/*.md'` → `0 error(s)`
- [ ] `git status --short` shows ONLY `docs/*.md` files (no README, no `.go`, no `PRD.md`/`tasks.json`)
- [ ] `go build ./... && go vet ./... && go test ./...` still pass (sanity — no code touched)

### Coherence (the four contract checks)

- [ ] **A (REQUIRED):** `docs/configuration.md` overview prose now states `--config`/`STAGECOACH_CONFIG`
      override discovery and are honored by the default commit action (mirrors cli.md:30)
- [ ] failure-modes table lists agent-missing → exit 1, pre-generation (Issue 3) — *verify present, unchanged*
- [ ] dry-run overview states full pipeline incl snapshot, no stale "skips snapshot" claim (Issues 2/6)
- [ ] config overview states `[generation] output`/`strip_code_fence` apply (Issue 4) — *verify present, unchanged*
- [ ] no doc anywhere claims dry-run skips the snapshot or the duplicate-check (grep = 0 hits)
- [ ] **B (RECOMMENDED, if done):** `docs/providers.md` notes `[generation]` overrides manifest output/strip_code_fence
- [ ] **C (OPTIONAL, if done):** `docs/cli.md` `--dry-run` row names the snapshot step explicitly
- [ ] Precedence list in `configuration.md` UNCHANGED (still matches `docs/cli.md` map / PRD §16.1)
- [ ] Every docs behavioral claim agrees with `docs/cli.md` (source of truth)

### Integrity / Discipline

- [ ] Mode-A rows intact (the only sanctioned re-touch of a Mode-A line is Edit C on cli.md:26)
- [ ] No edits to `README.md` (S1 owns it), `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file
- [ ] Minimal edits only — no padding, no whole-section rewrites (contract requirement)
- [ ] Reuses existing prose/admonition style; no new callout format introduced

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate the Mode-A edits — the `--config`-honored note already exists at `docs/cli.md:30`,
  the exit-code/failure rows are in, the dry-run flag desc is in, the `[generation]` paragraph is in.
  This task fills **overview gaps Mode-A did not reach**, primarily the config-reference prose.
- ❌ Don't reframe `[generation] output`/`strip_code_fence` as "per-manifest only" — that was the
  REJECTED fix (decisions.md D4 chose APPLY). Edit B must frame `[generation]` as the overriding,
  broader setting.
- ❌ Don't invent behavioral claims not present in `docs/cli.md` — the docs are a derived reference,
  never a new source of truth. When in doubt, quote `docs/cli.md`.
- ❌ Don't reorder or rewrite the Precedence list — it is verified-correct; only ADD the `--config` note.
- ❌ Don't "fix" how-it-works.md Snapshot invariant #3 or the rescue-protocol prose — they are
  correctly scoped to post-snapshot failures and are consistent with the pre-snapshot pre-flight.
- ❌ Don't edit `README.md`, `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file —
  out of scope (owned by S1 / the config init template / humans / the orchestrator).
- ❌ Don't skip the manual `npx markdownlint-cli2 'docs/**/*.md'` run assuming CI will catch it —
  markdownlint is NOT in CI.
- ❌ Don't pad — the contract explicitly says "edit only where text is now inaccurate or can be
  strengthened." If a verification pass finds no drift beyond A, ship just A (plus B/C only if clean).
