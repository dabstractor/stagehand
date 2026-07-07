---
name: "P1.M5.T1.S1 — Bulk rename stagehand→stagecoach in plan/ historical artifacts (Layer 5.5); EXCLUDES plan/012 (the rename-documentation preserve target). Fixes 3 gaps in the contract's literal sed command."
description: |

  Mode B (historical-artifacts) mechanical rename. The plan/ directory's PRIOR changesets (plan/001_* –
  plan/011_*, ~622 tracked files per the contract: task breakdowns, PRPs, architecture research) still carry
  `stagehand` references. With M1–M4 complete (Go source, build/CI, README, docs/, providers/, FUTURE_SPEC
  renamed), this task sweeps the historical plan/ surface so the whole repo presents one `stagecoach`
  identity. These files are git-tracked but never compiled/executed/shipped.

  ⚠️ THE CONTRACT'S LITERAL SED COMMAND IS A SKETCH — DO NOT RUN IT VERBATIM. It has THREE correctness gaps
  that the PRP fixes (research/findings.md §1):
    1. NO plan/012 exclusion → would run on plan/012_963e3918ec08 (the CURRENT changeset, which DOCUMENTS
       the rename and intentionally references BOTH names). It would turn "rename stagehand → stagecoach"
       into "rename stagecoach → stagecoach", destroying the rename record. FIX: `-path plan/012_963e3918ec08 -prune -o`.
    2. `grep -l 'stagehand'` is CASE-SENSITIVE → misses files whose only old-name refs are `Stagehand`/
       `STAGEHAND`, leaving them stale. FIX: `grep -li 'stagehand'`.
    3. `xargs sed -i …` has NO empty-input guard → errors if grep finds nothing. FIX: `xargs -r` (GNU).

  THE CORRECTED COMMAND (Linux/GNU; the CI env):
    find plan -path plan/012_963e3918ec08 -prune -o -type f \
      \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
      | xargs grep -li 'stagehand' 2>/dev/null \
      | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'

  CONTRACT (item_description §1–§5; PRD h2.30 rename directive):
    1. "~622 tracked files across plan/001_* through plan/011_* … historical task breakdowns, PRP files,
       architecture research … tracked in git but never compiled, executed, or shipped."
    3. LOGIC: the bulk sed (sketch above); "may also reference github.com/dustin/stagehand import paths —
       the sed handles those too." CAUTION: "do NOT modify the current plan/012_963e3918ec08/ directory's
       architecture docs (they intentionally reference both names as part of the rename documentation)."
    4. OUTPUT: "All plan/ files (except 012_*) use 'stagecoach' throughout."
    5. DOCS: "none — historical artifacts, no user-facing surface."

  DELIVERABLE: in-place textual rename across plan/001_* – plan/011_* (.md/.go/.toml/.txt), via the corrected
  command. plan/012_963e3918ec08 is PRUNED (untouched). No code, no build, no shipped surface.

  SCOPE BOUNDARY (do NOT touch):
    - plan/012_963e3918ec08/ — the CURRENT changeset; documents the rename; references both names by design.
    - All NON-plan/ surfaces (already renamed M1–M4): Go source, Makefile, .goreleaser.yaml, .github/,
      README.md, docs/*.md, providers/*.toml, FUTURE_SPEC.md (P1.M4.T2.S2), go.mod/go.sum.
    - PRD.md / tasks.json / prd_snapshot.md (orchestrator-owned, READ-ONLY).

  SUCCESS: zero `stagehand` residue in plan/001_*–plan/011_* (.md/.go/.toml/.txt); plan/012_963e3918ec08
  STILL contains stagehand refs (the preserve target is intact); `go build ./... && go test ./...` green
  (historical plan/ is not compiled); the diff is confined to plan/ (excluding 012).

---

## Goal

**Feature Goal**: Complete the stagehand→stagecoach rename on the historical plan/ surface (Layer 5.5) —
every `stagehand`/`Stagehand`/`STAGEHAND` reference in plan/001_*–plan/011_* (.md/.go/.toml/.txt) becomes
`stagecoach`/`Stagecoach`/`STAGECOACH`, while plan/012_963e3918ec08 (the current changeset documenting the
rename) is PRESERVED with both names intact.

**Deliverable**: an in-place textual rename across the historical plan/ files, via the corrected bulk
command (which fixes the contract's three gaps: adds the plan/012 prune, uses case-insensitive grep, and
guards empty xargs input). No code, no build artifacts, no shipped surface.

**Success Definition**:
- `find plan -path plan/012_963e3918ec08 -prune -o -type f \( -name '*.md' -o -name '*.go' -o -name
  '*.toml' -o -name '*.txt' \) -print | xargs grep -li 'stagehand'` → **no output** (zero residue outside 012).
- `grep -rli 'stagehand' plan/012_963e3918ec08/` → **> 0 files** (the preserve target is intact — the
  exclusion worked).
- `go build ./... && go test ./...` green (historical plan/ is not in the module build; regression check).
- The diff (if git-tracked) is confined to plan/ excluding 012; plan/012 is byte-unchanged.

## User Persona

**Target User**: a future contributor (or the primary author) browsing prior changesets in plan/ to recall
how a past feature was built. They should see a single `stagecoach` identity, not a stale mix of `stagehand`
(the dead old name) and `stagecoach`. (The CURRENT changeset, plan/012, is the exception: it intentionally
shows both names to document the rename.)

**Use Case**: contributor opens an old PRP (e.g. plan/003_*/P1M5T1S1/PRP.md) to study a prior pattern →
reads `stagecoach` throughout (the renamed module/binary/keys/env) → no confusion about which name is live.

**Pain Points Addressed**: stale `stagehand` references in historical artifacts that contradict the renamed
binary/module/env/config — a reader copying an old command or import path would reference a dead name.

## Why

- **Closes the historical plan/ rename surface (Layer 5.5).** With Go source (M1), config surface (M2),
  build/CI (M3), and user/provider/spec docs (M4) already renamed, plan/001–011 is the remaining tracked
  surface still saying `stagehand`.
- **Single consistent identity repo-wide.** The final whole-repo zero-residue audit (P1.M5.T2.S1) requires
  plan/ to be clean too; this task is its input.
- **Preserves the rename record.** plan/012 documents the stagehand→stagecoach migration and MUST keep both
  names. The corrected command's `-prune` makes this surgical (rename the history; spare the record).
- **Mechanical, zero-runtime-risk.** Historical plan/ files are never compiled/executed/shipped. A textual
  rename cannot affect behavior or tests; `go build`/`go test` are regression checks only.

## What

Run the **corrected** bulk-rename command over plan/ (pruning plan/012), then verify (a) zero `stagehand`
residue in plan/001–011 (.md/.go/.toml/.txt), (b) plan/012 STILL contains stagehand refs, and (c) the
build/test suite is unaffected. No edits to any non-plan/ surface, no edits to plan/012, no code changes.

The corrected command differs from the contract's sketch in exactly three ways (each fixing a real gap):

| gap | contract (unsafe) | corrected |
|-----|-------------------|-----------|
| plan/012 exclusion | (absent — would corrupt the rename docs) | `-path plan/012_963e3918ec08 -prune -o` |
| grep case | `grep -l 'stagehand'` (misses Stagehand/STAGEHAND-only files) | `grep -li 'stagehand'` |
| empty xargs | `xargs sed -i …` (errors on empty) | `xargs -r sed -i …` (GNU no-run-if-empty) |

### Success Criteria

- [ ] The corrected command (with the plan/012 prune + `grep -li` + `xargs -r`) is run.
- [ ] `find plan -path plan/012_963e3918ec08 -prune -o -type f \( -name '*.md' -o -name '*.go' -o -name
      '*.toml' -o -name '*.txt' \) -print | xargs grep -li 'stagehand'` → **no output** (zero residue).
- [ ] `grep -rli 'stagehand' plan/012_963e3918ec08/ | wc -l` → **> 0** (plan/012 preserved — exclusion worked).
- [ ] `go build ./... && go test ./...` green (regression check; plan/ not compiled).
- [ ] (git-tracked repo) `git diff --name-only | grep -E '^plan/012_963e3918ec08/'` → empty (012 untouched).
- [ ] No non-plan/ file touched; PRD.md/tasks.json/prd_snapshot.md untouched.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the corrected command (verbatim
above), the three gap-fixes explained, the before/after verification commands, the plan/012 preserve
requirement, and the scope boundary (non-plan/ surfaces already renamed; don't touch them). No Go/provider/
git-internals knowledge required — it is a mechanical textual rename with safety gates.

### Documentation & References

```yaml
# MUST READ — THE decisive doc (the 3 gaps + the corrected command + the safety analysis)
- docfile: plan/012_963e3918ec08/P1M5T1S1/research/findings.md
  why: §1 the THREE correctness gaps in the contract's literal sed (plan/012 exclusion missing; grep
       case-sensitive; xargs no-input guard) + the find -o grouping note; §2 the corrected command (the
       heart of the PRP); §3 why the blanket sed is SAFE across all historical files (every substring is a
       desired rename — .stagehand.toml, .stagehandignore, stagehand.* git keys, STAGEHAND_* env, the
       github.com/dustin/stagehand import path; commit-pi is NOT touched); §4 the plan/012 preserve target
       (VERIFIED — 11 files, the rename docs); §5 the research-env caveat (verify the live file set first);
       §6 validation (git-based + git-independent forms).
  critical: §1 (the 3 gaps — running the contract verbatim corrupts plan/012 AND misses capitalized-only
       files), §2 (the corrected command), §4 (plan/012 MUST stay untouched — verify it still has refs).

# MUST READ — the governing rename directive (the WHY)
- docfile: PRD.md (heading h2.30 — in context as selected_prd_content)
  section: "## Note: this project was originally named 'stagehand' and has been renamed. All references to
       'stagehand' must be replaced with 'stagecoach'."
  why: the authoritative project-wide rename directive. This task executes it on the plan/ historical surface.

# MUST READ — the preserve target (do NOT edit; verify it survives)
- file: plan/012_963e3918ec08/   (READ ONLY — the CURRENT changeset; PRUNED by the corrected command)
  section: the PRP/research files across P1M1*/P1M2*/P1M3*/P1M4*/P1M5* subdirs. They DOCUMENT the rename and
       intentionally reference both names (e.g. "rename stagehand.* git-config keys → stagecoach.*",
       "github.com/dustin/stagehand … 404 occurrences", "part of the stagehand→stagecoach project rename").
  why: confirms why plan/012 is EXCLUDED. Running the contract's un-pruned sed here would erase the rename
       record ("stagehand → stagecoach" ⇒ "stagecoach → stagecoach"). The corrected command prunes it.
  critical: after the rename, plan/012 MUST still contain stagehand refs — that is the proof the exclusion
       worked. Assert it (Success Criteria).

# READ — the sibling rename PRPs (establish the pattern + scope boundary)
- docfile: plan/012_963e3918ec08/P1M4T2S2/PRP.md   (FUTURE_SPEC.md rename — the immediately-preceding surface)
  section: the "What" + the corrected sed arms + the "zero-residue grep" gate + "go test is a regression
       check only (pure docs)".
  why: confirms the rename pattern (case-variant sed + zero-residue grep + build/test regression check) this
       task generalizes from one file to ~622. Also confirms FUTURE_SPEC.md is a SEPARATE sibling surface
       (Layer 5.4) already done — NOT this task.
  critical: S2 used only TWO sed arms (FUTURE_SPEC had no STAGEHAND). The historical plan/ surface needs ALL
       THREE (env-var docs `STAGEHAND_*` are common in PRPs). Use three arms.

# READ — the rename-surface map context (if present in the full repo)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md   (REFERENCED by sibling PRPs; may live
       in the full repo even if absent in a research excerpt)
  section: the layer list (5.1 Go, 5.2 docs, 5.3 providers, 5.4 FUTURE_SPEC, 5.5 plan/) + the final
       whole-repo zero-residue gate (gate 5: `grep -ri stagehand … == 0`).
  why: confirms this task = Layer 5.5 (plan/ historical) and that the FINAL whole-repo audit is P1.M5.T2.S1
       (not this task). NOTE: this file was NOT present in the research excerpt — if absent in the run env,
       the PRP stands on the contract + PRD h2.30 alone.
```

### Current Codebase tree (relevant slice)

```bash
plan/
  001_*/ … 011_*/        # *** EDIT (in place) *** — historical changesets; ~622 tracked files (.md/.go/.toml/.txt)
                         #   with stagehand refs. The corrected sed renames them; plan/012 is pruned.
  012_963e3918ec08/      # *** READ ONLY — PRUNED *** — the CURRENT changeset; documents the rename (both names).
                         #   Contains P1M1*/…/P1M5* PRP + research subdirs. Must survive untouched.
# Non-plan/ surfaces (already renamed M1–M4; UNCHANGED by this task):
cmd/stagecoach/ pkg/stagecoach/ internal/ docs/ providers/ README.md FUTURE_SPEC.md Makefile
.goreleaser.yaml .github/ go.mod go.sum
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. IN-PLACE textual edits across plan/001_*/…/plan/011_*/ (.md/.go/.toml/.txt) only.
# plan/012_963e3918ec08/ UNCHANGED. All non-plan/ surfaces UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (plan/012 EXCLUSION — Gap 1): the contract's literal command lacks the prune. Running it verbatim
#   renames plan/012 too, turning "rename stagehand → stagecoach" into "rename stagecoach → stagecoach" and
#   erasing the rename record (this PRP, sibling PRPs, the verified-surface maps). The corrected command
#   prunes plan/012_963e3918ec08. After the run, ASSERT plan/012 still contains stagehand refs (Success Crit).

# CRITICAL (CASE-INSENSITIVE grep — Gap 2): `grep -l 'stagehand'` lists ONLY files with lowercase stagehand.
#   A historical file whose only old-name refs are `Stagehand` (a title) or `STAGEHAND_*` (env-var docs)
#   would be EXCLUDED and left stale. Use `grep -li 'stagehand'` so all three case variants are renamed.

# CRITICAL (EMPTY-INPUT GUARD — Gap 3): if grep finds zero files (already renamed, or historical dirs absent
#   in the run env), `xargs sed -i` either errors (sed with no file args) or reads stdin. Use `xargs -r`
#   (GNU --no-run-if-empty). BSD xargs lacks -r (see macOS note).

# CRITICAL (THREE sed arms, not two): historical PRPs document STAGEHAND_PROVIDER / STAGEHAND_MODEL /
#   STAGEHAND_NO_VERIFY etc. (env vars renamed P1.M2.T1.S1). The arms are CASE-DISJOINT + order-safe:
#   `s/stagehand/` (lowercase s) skips Stagehand/STAGEHAND; `s/Stagehand/` needs lowercase 'tagehand' so it
#   does NOT match all-caps STAGEHAND; `s/STAGEHAND/` matches all-caps. None re-creates another's pattern.

# GOTCHA (every substring is a DESIRED rename — no compound-token preservation needed): .stagehand.toml →
#   .stagecoach.toml; .stagehandignore → .stagecoachignore; stagehand.no_verify/noVerify → stagecoach.*;
#   github.com/dustin/stagehand → github.com/dustin/stagecoach (the contract: "the sed handles those too");
#   cmd/pkg stagehand → stagecoach. There is NO token where a stagehand substring should stay. `commit-pi`
#   (the originating tool) has NO stagehand substring and is untouched. Still run the defensive post-check.

# GOTCHA (find -o grouping): empirically the UNGROUPED `find … -name '*.md' -o -name '*.go' …` works on GNU
#   find (implicit -print covers the whole OR). BUT once you add `-path … -prune -o` + explicit `-print`,
#   the -name conditions MUST be grouped `\( … \)` or the prune/print logic mis-binds. Always use the
#   grouped form (also portable to BSD find).

# GOTCHA (research-env caveat): the research clone may contain ONLY plan/012 and may not be a git repo. The
#   implementing agent runs in the FULL repo where plan/001–011 exist and are git-tracked. VERIFY the live
#   file set FIRST (Task 1). If the historical dirs are absent in the run env too, this is a verified no-op
#   (document "reviewed, no changes needed" — the contract allows it).

# GOTCHA (macOS BSD sed, local runs): `sed -i '' 's/…/…/g; …'` (empty backup-ext arg). BSD xargs has no -r;
#   verify grep found ≥1 file before the sed pipe, or run on Linux/CI. The CI is GNU/Linux.

# GOTCHA (historical plan/ is NOT compiled): `go build ./...` only builds the current module's packages;
#   plan/ .go files are research/examples, not in the module. Build/test is a regression check, not a
#   feature test — expect green, unchanged.
```

## Implementation Blueprint

### Data models and structure

_None._ A mechanical textual rename across existing historical files. No data models, no code, no new types.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the live file set + the plan/012 boundary (READ/count, no edit)
  - RUN (full repo): count historical plan files with any case-variant stagehand ref (EXCLUDING 012):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null | wc -l
    Record N. The contract says ~622 files total; N is the subset with stagehand refs. Sample a few
    (`… | head`) to confirm they are the expected old-name references (prose, import paths, env vars).
  - CONFIRM plan/012 is the preserve target: `grep -rli 'stagehand' plan/012_963e3918ec08/ | wc -l` → > 0.
  - IF N == 0 (historical dirs absent or already clean): STOP — this is a verified no-op. Document "reviewed
    the plan/ surface; no stagehand references found outside plan/012; no changes needed" (the contract's
    allowed framing). Do NOT run the sed.
  - WHY: the research clone lacked plan/001–011; verify the live set before the mechanical pass. The
    plan/012 count is the baseline for the post-rename preservation assertion.
  - GOTCHA: if `find` errors with "permission denied" or similar on some paths, that's an env issue, not a
    logic issue — investigate before proceeding.

Task 2: RUN the corrected bulk rename (THE deliverable)
  - COMMAND (Linux/GNU sed + GNU xargs — the CI env):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null \
          | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
  - macOS/BSD LOCAL variant (if not on CI): replace the last line with
        | xargs sed -i '' 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
    AND first confirm Task 1's N > 0 (BSD xargs has no -r; an empty grep result would make sed read stdin).
  - WHY: this is the whole task. The three fixes (prune / grep -li / xargs -r) make the contract's sketch
    safe and complete.
  - GOTCHA: do NOT drop the prune (Gap 1), do NOT use case-sensitive grep (Gap 2), do NOT drop -r (Gap 3).
    Do NOT add `docs/`, `providers/`, or any non-plan/ path (already renamed M1–M4).

Task 3: VERIFY — zero residue + plan/012 preserved + build/test unaffected
  - ZERO RESIDUE (outside 012):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null | wc -l
    Expected: 0. (Any non-zero output is a missed file — re-run; likely a capitalized-only file the case-
    insensitive grep should have caught, or a file type outside the 4 extensions.)
  - PLAN/012 PRESERVED:
        grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l
    Expected: > 0 (the rename docs intact — the exclusion worked). If 0, the prune FAILED and plan/012 was
    corrupted — restore plan/012 from git and re-run with the prune.
  - BUILD/TEST (regression check; plan/ not compiled):
        go build ./... && go test ./...
    Expected: green, unchanged.
  - SCOPE (git-tracked repo):
        git diff --name-only | grep -E '^plan/012_963e3918ec08/'   # expect: EMPTY (012 untouched)
        git diff --name-only | grep -vE '^plan/' | head            # expect: EMPTY (no non-plan/ file touched)
  - WHY: the three gates prove the rename is complete (zero residue), surgical (012 intact), and safe
    (build green).
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the corrected bulk rename (the contract's sketch + 3 fixes). One pass.
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null \
  | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'

# PATTERN: zero-residue gate (case-insensitive; excludes 012). Same find, grep -li, expect no output.
# PATTERN: plan/012 preservation gate. grep -rli 'stagehand' plan/012_963e3918ec08/ → expect > 0.

# CRITICAL: the THREE gaps vs the contract's literal command —
#   Gap 1 (prune plan/012): without it, the rename docs in plan/012 are corrupted. NON-NEGOTIABLE.
#   Gap 2 (grep -li): case-insensitive; catches Stagehand/STAGEHAND-only files the contract's `grep -l` misses.
#   Gap 3 (xargs -r): no-op on empty input (historical dirs absent ⇒ graceful, not an error).

# GOTCHA: three sed arms (historical PRPs have STAGEHAND_* env-var docs). Case-disjoint + order-safe.
# GOTCHA: every stagehand substring is a desired rename (no compound-token preservation). commit-pi untouched.
# GOTCHA: plan/ is NOT compiled — go build/test are regression checks, not feature tests.
```

### Integration Points

```yaml
PLAN.HISTORICAL (plan/001_*/…/plan/011_* — the edit surface):
  - change: "in-place textual rename across .md/.go/.toml/.txt; three case-variant sed arms."
  - runtime: "NONE — never compiled/executed/shipped (the contract). go build/test unaffected."

PLAN.012 (plan/012_963e3918ec08 — PRUNED, untouched):
  - preserve: "the CURRENT changeset; documents the rename with both names. -prune excludes it."
  - assert: "post-rename, plan/012 STILL contains stagehand refs (the exclusion worked)."

NON-PLAN.SURFACES (unchanged — already renamed M1–M4):
  - Go source / Makefile / .goreleaser.yaml / .github / README.md / docs/ / providers/ / FUTURE_SPEC.md /
    go.mod / go.sum: "UNCHANGED. Do NOT include them in the find."

GO.MODULE / BUILD / TEST: change NONE. Historical plan/ is not in the module build. `go build ./... &&
      go test ./...` is a regression-safety check (expected green, unchanged).

DOWNSTREAM (P1.M5.T2.S1 — the final whole-repo zero-residue audit): this task's output (clean plan/) is its
      input. If this task leaves residue, the audit fails — so the zero-residue gate here is the audit's
      plan/-scoped pre-check.
```

## Validation Loop

### Level 1: The corrected command ran + gates pass (the core verification)

```bash
# ZERO RESIDUE outside plan/012 (git-independent; the primary gate):
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l
# Expected: 0. (Any output = a missed file; investigate + re-run.)

# PLAN/012 PRESERVED (the exclusion worked):
grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l
# Expected: > 0 (the rename docs intact). If 0 → the prune FAILED; restore plan/012 from git + fix the prune.

# (quick alias) whole-plan residue excluding 012, any file type:
grep -rli 'stagehand' plan/ --include='*.md' --include='*.go' --include='*.toml' --include='*.txt' \
  | grep -v '^plan/012_963e3918ec08/' | wc -l
# Expected: 0.
```

### Level 2: Scope (only plan/ changed; 012 + non-plan/ untouched)

```bash
# git-tracked repo:
git diff --name-only | grep -E '^plan/012_963e3918ec08/'        # expect: EMPTY (012 untouched)
git diff --name-only | grep -vE '^plan/' | head                 # expect: EMPTY (no non-plan/ file touched)
git diff --name-only | grep -E '^plan/' | grep -v '^plan/012' | wc -l   # the renamed historical files (>0)

# frozen surfaces byte-unchanged:
git diff --exit-code README.md FUTURE_SPEC.md docs providers cmd pkg internal go.mod go.sum Makefile \
  .goreleaser.yaml 2>/dev/null && echo "non-plan/ surfaces UNCHANGED (expected)"
```

### Level 3: Build/test regression check (plan/ is not compiled)

```bash
go build ./...     # Expect clean (plan/ .go files are not in the module build).
go test ./...      # Expect all PASS — historical plan/ is never imported by tests.
# This is a regression-safety check (the rename should be a no-op for the suite), not a feature test.
```

### Level 4: Defensive sanity (no unexpected compound-token residue; whole-repo pre-audit)

```bash
# Sanity: no stagehand residue of ANY case across the whole repo's tracked text, EXCEPT plan/012.
# (The FINAL whole-repo zero-residue audit is P1.M5.T2.S1; this is a quick confidence grep.)
grep -rli 'stagehand' . --include='*.md' --include='*.go' --include='*.toml' --include='*.txt' \
  --include='*.yaml' --include='*.yml' 2>/dev/null | grep -v '^./plan/012_963e3918ec08/'
# Expected: empty. (If non-plan/ surfaces appear, they are a sibling task's surface, not this task's —
# flag for P1.M5.T2.S1. If plan/001-011 appear, re-run Task 2 — a file was missed.)

# Spot-check a renamed historical file reads cleanly (stagecoach throughout, no double-coach, no broken prose):
# (pick any historical PRP confirmed in Task 1's sample, e.g.)
# grep -c 'stagecoach' plan/001_*/P1M1T1S1/PRP.md   → > 0
# grep -c 'stagehand' plan/001_*/P1M1T1S1/PRP.md   → 0
```

## Final Validation Checklist

### Technical Validation
- [ ] The CORRECTED command ran (with plan/012 prune + `grep -li` + `xargs -r`); the contract's literal
      command was NOT run verbatim.
- [ ] Zero-residue gate: `find plan … -prune … | xargs grep -li 'stagehand'` → 0 files outside plan/012.
- [ ] plan/012 preserved: `grep -rli 'stagehand' plan/012_963e3918ec08/ | wc -l` → > 0.
- [ ] `go build ./... && go test ./...` green (plan/ not compiled; regression check).
- [ ] (git) `git diff` confined to plan/ (excluding 012); plan/012 + all non-plan/ surfaces byte-unchanged.

### Feature Validation
- [ ] All historical plan/ files (.md/.go/.toml/.txt outside 012) use `stagecoach`/`Stagecoach`/`STAGECOACH`.
- [ ] plan/012_963e3918ec08 STILL references `stagehand` (the rename documentation is intact).
- [ ] Every `stagehand` substring was a desired rename (config files, env vars, git keys, import paths,
      binary/prose name); `commit-pi` untouched; no compound-token corruption (defensive grep clean).

### Code Quality Validation
- [ ] The three contract gaps were fixed (prune / case-insensitive grep / xargs -r); the corrected command
      is the one that ran.
- [ ] No prose altered beyond the name swaps (a bulk `s///g` — mechanical, no rewording).
- [ ] Scope respected: non-plan/ surfaces (M1–M4) untouched; plan/012 untouched.
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] [Mode B] historical artifacts — no user-facing surface (DOCS clause: "none").
- [ ] If the historical dirs were absent (verified no-op), the implementation summary records "reviewed the
      plan/ surface; no stagehand references outside plan/012; no changes needed."

---

## Anti-Patterns to Avoid

- ❌ **Don't run the contract's literal sed command verbatim.** It lacks the plan/012 prune (Gap 1 → corrupts
  the rename docs), uses case-sensitive grep (Gap 2 → misses Stagehand/STAGEHAND-only files), and has no
  empty-input guard (Gap 3 → errors on empty). Run the CORRECTED command. (§1)
- ❌ **Don't rename plan/012.** It is the CURRENT changeset documenting the rename; it intentionally shows
  both names. The `-prune` excludes it. After the run, ASSERT it still has stagehand refs. (§4)
- ❌ **Don't use only two sed arms.** Historical PRPs document `STAGEHAND_*` env vars. Use all three arms
  (lowercase / Capitalized / ALL-CAPS); they are case-disjoint and order-safe. (gotcha)
- ❌ **Don't widen scope to non-plan/ surfaces.** docs/, providers/, README.md, FUTURE_SPEC.md, Go source,
  Makefile, .goreleaser.yaml, go.mod — ALL already renamed in M1–M4. The find targets `plan/` only. (scope)
- ❌ **Don't "preserve stagehand for backwards compat" in historical files.** The renamed module/binary/env
  do not answer to `stagehand`. Historical artifacts should reflect the live `stagecoach` identity. The ONLY
  exception is plan/012 (the rename record), handled by the prune. (§3)
- ❌ **Don't expect a test to validate this.** Historical plan/ is not compiled/imported. Validation is
  textual (zero-residue grep + plan/012 preservation). `go test ./...` is a regression check only. (gotcha)
- ❌ **Don't conflate this with the final whole-repo audit.** This task is the plan/ historical surface
  (Layer 5.5). The project-wide `grep -ri stagehand … == 0` gate is P1.M5.T2.S1. (scope)
- ❌ **Don't skip the live file-set verification (Task 1).** The research clone lacked plan/001–011. Verify
  the count + sample in the run env first; if absent, it's a documented no-op, not a silent skip. (§5)
