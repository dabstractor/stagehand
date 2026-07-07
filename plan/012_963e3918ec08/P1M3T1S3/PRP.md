---
name: "P1.M3.T1.S3 — Rename CI workflow module paths and .gitignore entries: stagehand → stagecoach (project rename Layer 4.3–4.4)"
description: |

  The CI-workflow + .gitignore rename layer of the stagehand→stagecoach project rename (plan 012). The Go
  module (`github.com/dustin/stagecoach`), `cmd/stagecoach/`, the Makefile (S1), and `.goreleaser.yaml`
  (S2, parallel) are already renamed. This task renames the LAST build/CI-surface files that still reference
  `stagehand`: `.github/workflows/ci.yml` (5 refs) and `.gitignore` (4 refs); reviews `release.yml`
  (0 refs — no change); and removes the stale untracked `./stagehand` binary.

  THE ACTUAL CODE IS AT `/home/dustin/projects/stagehand/` (the project root — go.mod is already
  `github.com/dustin/stagecoach`). Run ALL commands from there. The plan artifacts live at
  `/home/dustin/projects/stagecoach/plan/012_…/`.

  ⚠️ **#1 — ci.yml: use the GLOBAL two-branch sed, NOT the work item's narrow module-path-only sed.** The
      work item suggests `sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' ci.yml`.
      That fixes L102-105 but MISSES L1's `Stagehand CI` comment (which would then fail the final grep
      audit P1.M5.T2.S1). Use: `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .github/workflows/ci.yml`
      (fixes ALL 5 refs). (research §1)

  ⚠️ **#2 — ci.yml L102-105 are FUNCTIONAL, not cosmetic (THE load-bearing detail).** They are the
      coverage-gate package paths in the awk script. coverage.out (emitted by Go) now uses the REAL module
      path `github.com/dustin/stagecoach/...`. If L102-105 still say `stagehand`, the awk's `t[i]` matches
      NOTHING in coverage.out ⇒ `::error::... no coverage data` ⇒ `fail=1` ⇒ **the CI coverage gate FAILS
      on the next run**. This is a CI-breaking bug left by M1.T1.S1's Go-only sed. (research §2)

  ⚠️ **#3 — KEEP `dustin/` (do NOT change to `dabstractor/`).** The sed changes ONLY `stagehand`→`stagecoach`;
      `github.com/dustin/` is preserved (it contains no `stagehand` substring). Matches go.mod + S2's
      namespace decision (3 sources: go.mod `github.com/dustin/stagecoach`, PRD §21.2/§21.3 `dustin/`,
      goreleaser `owner: dustin`). (research §2/§4)

  ⚠️ **#4 — .gitignore L4 `/stagehand`: DELETE it, do NOT rename it.** `/stagecoach` is ALREADY on L5
      (verified). The work item's naive global sed would turn L4 `/stagehand` → `/stagecoach`, creating a
      DUPLICATE `/stagecoach` (L4 and L5). The stale `/stagehand` is pure cruft (the binary is now
      `stagecoach`); delete it, then sed the remaining refs (L22 comment, L23 `.stagehand.toml`, L40 comment).
      (research §3)

  ⚠️ **#5 — release.yml: NO CHANGE.** 0 stagehand refs (verified). Its `dustin/homebrew-tap` + `dustin/scoop-bucket`
      SECRETS comments (L5-6) are CORRECT per S2 (the org is `dustin/`). Review it, make NO edit. (research §4)

  ⚠️ **#6 — stale binary cleanup: the work item targets the WRONG path.** `bin/` already has only
      `stagecoach` + `stagecoach-test` (S1's Makefile rebuild); `bin/stagehand`/`bin/stagehand-test` DO NOT
      EXIST (the work item's `rm -f bin/...` is a no-op). The ACTUAL stale binary is at the REPO ROOT:
      `./stagehand` (gitignored, untracked). Remove it: `rm -f stagehand stagehand-test` (from the project root).
      (research §5)

  Deliverable: MODIFIED `.github/workflows/ci.yml` (5 refs → stagecoach) + MODIFIED `.gitignore` (delete
  stale `/stagehand`; rename L22/23/40) + REVIEWED `.github/workflows/release.yml` (no edit) + REMOVED
  stale untracked `./stagehand` binary. NO go.mod/go.sum/Makefile/.goreleaser/.go/docs change. OUTPUT:
  `grep -rni stagehand .github/workflows/ci.yml .gitignore` → 0; ci.yml coverage-gate paths match go.mod's
  `github.com/dustin/stagecoach/...`; .gitignore has a single `/stagecoach` + `.stagecoach.toml`; no stale
  `./stagehand` binary. DOCS: none — CI and gitignore are internal.

---

## Goal

**Feature Goal**: Complete the stagehand→stagecoach rename on the CI/build-surface files: rename every
`stagehand`/`Stagehand` reference in `.github/workflows/ci.yml` and `.gitignore` to `stagecoach`, confirm
`release.yml` needs no change, and remove the stale untracked `./stagehand` binary. The functional
load-bearing fix is ci.yml L102-105 — the coverage-gate package paths must match the renamed Go module
(`github.com/dustin/stagecoach/...`) or the CI coverage gate fails.

**Deliverable** (MODIFY/REVIEW only — no new files):
1. `.github/workflows/ci.yml` — global `stagehand`→`stagecoach` + `Stagehand`→`Stagecoach` (5 refs: L1
   comment + L102-105 coverage-gate paths). `dustin/` preserved.
2. `.gitignore` — DELETE the stale L4 `/stagehand` (L5 `/stagecoach` already present); rename L22 comment +
   L23 `.stagehand.toml`→`.stagecoach.toml` + L40 comment.
3. `.github/workflows/release.yml` — REVIEW only (0 refs; `dustin/` SECRETS comments correct). NO edit.
4. Remove the stale untracked `./stagehand` binary at the repo root.

**Success Definition**: `grep -rni stagehand .github/workflows/ci.yml .gitignore` → 0; ci.yml L102-105 =
`github.com/dustin/stagecoach/internal/{git,provider,generate,config}` (match go.mod + coverage.out);
release.yml unchanged (0 refs, `dustin/` preserved); .gitignore has exactly one `/stagecoach` (no duplicate)
+ `.stagecoach.toml`; no `./stagehand` binary; only ci.yml + .gitignore changed among TRACKED files;
go.mod/go.sum/Makefile/.goreleaser.yaml/.go/docs byte-unchanged.

## User Persona

**Target User**: The maintainer pushing to `main` (triggering CI) and any contributor whose PR runs the
coverage gate. Transitively: the release pipeline (release.yml is reviewed for consistency).

**Use Case**: After the Go/module rename (M1) + Makefile (S1) + goreleaser (S2), the CI workflow and
.gitignore still reference the old `stagehand` name. On the next push to `main`, the coverage-gate awk
(ci.yml L92-114) would FAIL because its `t[i]` package paths (`.../stagehand/internal/...`) no longer match
coverage.out's `.../stagecoach/internal/...` paths. This task fixes that BEFORE the next CI run.

**User Journey**: maintainer merges the rename → pushes to `main` → CI's build/test/lint/govulncheck pass →
the coverage gate's `t[i]` paths now match coverage.out → coverage gate passes (≥85% on the 4 core
packages) → green CI.

**Pain Points Addressed**: (1) A latent CI-breaking bug (coverage gate fails on the next run). (2) A stale
.gitignore entry (`/stagehand`) + missing `.stagecoach.toml` ignore. (3) A stale `./stagehand` binary a
dev might accidentally run.

## Why

- **Layer 4.3–4.4 of the project rename.** The Go structural rename (M1), config surface (M2), Makefile
  (S1), and goreleaser (S2) are done/parallel. The CI workflow + .gitignore are the last build-surface
  files referencing `stagehand`.
- **Fixes a CI-breaking functional gap (ci.yml L102-105).** This is NOT cosmetic — the coverage-gate awk
  matches its `t[i]` keys against coverage.out's package paths; a `stagehand`/`stagecoach` mismatch makes
  every package report "no coverage data" and fails the gate.
- **Completes the tracked-file rename for the build surface.** After this, the only remaining `stagehand`
  references are in docs/ (M4), providers/ (M4), and plan/ historical artifacts (M5) — all separately owned.

## What

Two text edits (ci.yml global sed; .gitignore delete-then-sed), one review (release.yml, no change), and
one untracked-artifact cleanup (root `./stagehand`). No logic change, no Go code, no structural YAML change.

### Success Criteria
- [ ] `grep -rni stagehand .github/workflows/ci.yml .gitignore` → 0 (zero references remain in either file).
- [ ] ci.yml L1 comment = `Stagecoach CI`; L102-105 = `github.com/dustin/stagecoach/internal/{git,provider,generate,config}`.
- [ ] ci.yml L102-105 paths MATCH go.mod's module prefix (`github.com/dustin/stagecoach`) — the coverage
      gate will now find the packages in coverage.out.
- [ ] `dustin/` is PRESERVED in ci.yml (the sed touched only `stagehand`; `github.com/dustin/` unchanged).
- [ ] .gitignore: L4 `/stagehand` is GONE; exactly ONE `/stagecoach` remains (no duplicate); `.stagecoach.toml`
      present (was `.stagehand.toml`); L22 + L40 comments say `Stagecoach`.
- [ ] release.yml: UNCHANGED (`git diff --exit-code .github/workflows/release.yml` clean); 0 stagehand refs;
      `dustin/homebrew-tap` + `dustin/scoop-bucket` preserved.
- [ ] No stale `./stagehand` binary at the repo root.
- [ ] Only `.github/workflows/ci.yml` + `.gitignore` changed among TRACKED files; go.mod/go.sum/Makefile/
      .goreleaser.yaml/.go/docs/providers/PRD.md byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can do this from: the exact sed/edit commands (below), the
four work-item corrections (§1-§5 of the description + research), the functional reason ci.yml L102-105
matters (the coverage-gate awk), the namespace decision (keep `dustin/`), and the verification commands. No
Go/CI-internals knowledge required beyond reading the awk comment.

### Documentation & References

```yaml
# MUST READ — the design calls (the 4 corrections to the work item's naive plan)
- docfile: plan/012_963e3918ec08/P1M3T1S3/research/design-decisions.md
  why: §0 (verified ref counts per file), §1 (ci.yml GLOBAL sed — catches L1 comment too), §2 (ci.yml L102-105
       are FUNCTIONAL coverage-gate paths — the load-bearing fix; KEEP dustin/), §3 (.gitignore DELETE L4
       /stagehand — avoid duplicating /stagecoach), §4 (release.yml NO change — dustin/ SECRETS correct),
       §5 (stale binary is at REPO ROOT not bin/), §6 (scope boundaries), §7 (validation).
  critical: §2 (the coverage-gate functional fix — without it CI fails on the next run) and §3 (the dedup
       gotcha — the naive sed duplicates /stagecoach) are the things most likely to be implemented wrong.

# The namespace CONTRACT (S2 — the org decision this task inherits)
- docfile: plan/012_963e3918ec08/P1M3T1S2/PRP.md
  why: S2 established KEEP `dustin/` (3 sources: go.mod `github.com/dustin/stagecoach`, PRD §21.2/§21.3
       `dustin/`, goreleaser `owner: dustin`). release.yml's `dustin/homebrew-tap` + `dustin/scoop-bucket`
       SECRETS comments match S2's preserved owners. This task does NOT change the org.
  critical: do NOT change `dustin`→`dabstractor` anywhere (ci.yml paths, release.yml SECRETS comments).

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md §20.4 (h3.96) + h2.30
  section: §20.4 — the CI matrix (build/test on {linux,macos,windows}×{amd64,arm64}, Go 1.22+1.23,
           golangci-lint, govulncheck, goreleaser-on-tag). h2.30 — "All references to 'stagehand' must be
           replaced with 'stagecoach'."
  critical: h2.30 is the comprehensive-rename mandate (so the L1 `Stagehand CI` comment MUST be fixed too,
           not just the functional L102-105 paths — else the final grep audit P1.M5.T2.S1 fails).

# THE FILE BEING EDITED — ci.yml (READ L1 + L88-114 before the sed)
- file: .github/workflows/ci.yml   (at /home/dustin/projects/stagehand/.github/workflows/ci.yml)
  section: L1 `# … — Stagehand CI (PRD §20.4 …)` (the comment); L88-114 the `Enforce >=85% on
           internal/{git,provider,generate,config}` awk step — esp. L102-105 `t[1..4]="github.com/dustin/
           stagehand/internal/..."` (the coverage-gate package keys) + L108 `if(!(t[i] in tot)){ … fail=1 }`.
  why: the ONLY functional concern. The awk matches `t[i]` against coverage.out's package paths; a
       stagehand/stagecoach mismatch ⇒ "no coverage data" ⇒ fail.
  critical: after the sed, L102-105 MUST be `github.com/dustin/stagecoach/internal/...` (matching go.mod +
       coverage.out). KEEP `dustin/`.

# THE FILE BEING EDITED — .gitignore (READ the full 43-line file before editing)
- file: .gitignore   (at /home/dustin/projects/stagehand/.gitignore)
  section: L4 `/stagehand` (STALE — L5 `/stagecoach` already present ⇒ DELETE L4, don't rename); L22 comment
           `# Stagehand repo-local config (per-repo .stagehand.toml; …)`; L23 `.stagehand.toml` (→ .stagecoach.toml);
           L40 comment `# Go build / test artifacts (Stagehand)`.
  why: the stale-binary-ignore dedup (L4/L5) + the config-file rename (.stagehand.toml → .stagecoach.toml,
       which M2.T2.S1 fixed in Go code but not in this non-Go file).
  critical: L4 `/stagehand` → DELETE (not rename). A naive `sed s/stagehand/stagecoach/g` duplicates `/stagecoach`.

# THE FILE BEING REVIEWED — release.yml (0 refs; no edit)
- file: .github/workflows/release.yml   (at /home/dustin/projects/stagehand/.github/workflows/release.yml)
  section: L5-6 SECRETS comments (`contents:write on dustin/homebrew-tap`, `dustin/scoop-bucket`) + L41-43
           the `HOMEBREW_TAP_GITHUB_TOKEN`/`SCOOP_BUCKET_GITHUB_TOKEN`/`AUR_SSH_PRIVATE_KEY` env bindings.
  why: confirms 0 stagehand refs + that the `dustin/` references are CORRECT (match S2's goreleaser owners).
  critical: make NO edit. The work item's caution about "dustin/homebrew-tap" needing "org correction" is a
       RED HERRING — `dustin/` is canonical (S2). Changing to `dabstractor/` breaks go.mod/PRD/goreleaser consistency.

# The module-path authority (read-only — confirms dustin/ + the coverage.out path prefix)
- file: go.mod   (at /home/dustin/projects/stagehand/go.mod)
  section: L1 `module github.com/dustin/stagecoach` (ALREADY renamed M1.T1.S1).
  why: the authoritative module path. ci.yml L102-105 MUST match its `github.com/dustin/stagecoach/` prefix
       (Go emits coverage.out with this prefix). Confirms the org is `dustin`.
```

### Current Codebase tree (relevant slice)

```bash
.github/workflows/
  ci.yml             # 5 stagehand/Stagehand refs (L1 comment + L102-105 coverage-gate paths) — EDIT (global sed)
  release.yml        # 0 refs — REVIEW only (no edit; dustin/ SECRETS correct per S2)
.gitignore           # 4 refs (L4 stale /stagehand; L22 comment; L23 .stagehand.toml; L40 comment) — EDIT (delete L4 + sed)
go.mod               # module github.com/dustin/stagecoach (ALREADY renamed — the module-path authority)
Makefile             # S1 (Complete) — builds ./bin/stagecoach; UNCHANGED
.goreleaser.yaml     # S2 (parallel) — UNCHANGED
stagehand            # STALE untracked root binary (gitignored by .gitignore L4) — REMOVE
bin/{stagecoach,stagecoach-test}  # already renamed (S1's rebuild) — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits (ci.yml + .gitignore) + ONE review (release.yml, no change) + ONE
# untracked-artifact cleanup (rm ./stagehand).
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (#1 — ci.yml GLOBAL sed, not the narrow module-path-only sed): the work item's
#   `sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' ci.yml` fixes L102-105 but MISSES
#   L1's `Stagehand CI` comment (which fails the final grep audit P1.M5.T2.S1). Use
#   `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .github/workflows/ci.yml` (ALL 5 refs). (research §1)
# CRITICAL (#2 — ci.yml L102-105 are FUNCTIONAL): they are the coverage-gate awk's `t[i]` package keys.
#   coverage.out (Go-emitted) now uses `github.com/dustin/stagecoach/...`. A stagehand/stagecoach mismatch ⇒
#   `::error::... no coverage data` ⇒ fail=1 ⇒ CI coverage gate FAILS. This is the load-bearing fix. (research §2)
# CRITICAL (#3 — KEEP dustin/): the sed changes ONLY stagehand→stagecoach. `github.com/dustin/` contains no
#   `stagehand` substring ⇒ preserved. Matches go.mod + S2 + PRD §21.2/§21.3. Do NOT add a dustin→dabstractor step. (research §2/§4)
# CRITICAL (#4 — .gitignore L4 /stagehand: DELETE, don't rename): `/stagecoach` is ALREADY on L5. The naive
#   global sed would DUPLICATE `/stagecoach` (L4 + L5). `sed -i '/^\/stagehand$/d' .gitignore` first, THEN
#   `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .gitignore` for L22/23/40. (research §3)
# CRITICAL (#5 — release.yml NO change): 0 stagehand refs; the `dustin/homebrew-tap`/`dustin/scoop-bucket`
#   SECRETS comments are CORRECT (S2's namespace). Review only; do NOT edit; do NOT change to dabstractor/. (research §4)
# CRITICAL (#6 — stale binary is at REPO ROOT, not bin/): bin/ already has stagecoach/stagecoach-test (S1).
#   `rm -f bin/stagehand bin/stagehand-test` is a no-op. The actual stale binary is `./stagehand` (root).
#   `rm -f stagehand stagehand-test` from the project root. (research §5)
# GOTCHA (project root): run ALL commands from /home/dustin/projects/stagehand (where go.mod/ci.yml live).
#   The plan artifacts are at /home/dustin/projects/stagecoach/plan/012_…/ (separate).
# GOTCHA (.stagecoach.toml is a genuine rename, not a dedup): unlike /stagecoach (already present), .stagecoach.toml
#   did NOT already exist in .gitignore ⇒ renaming L23 .stagehand.toml → .stagecoach.toml is correct (no duplicate).
# GOTCHA (untracked artifacts don't affect the tracked-file audit): ./stagehand is gitignored (untracked). Removing
#   it is a cleanliness step; P1.M5.T2.S1's "zero stagehand in tracked files" audit covers TRACKED files only.
# GOTCHA (actionlint may not be installed): if `actionlint` is absent, validate ci.yml by visual review of the
#   YAML structure (the sed changed only string literals — L1 comment + L102-105 awk strings — not the YAML keys/structure).
# GOTCHA (scope): ci.yml + .gitignore ONLY (+ release.yml review + root-binary cleanup). Makefile=S1,
#   .goreleaser.yaml=S2, README/docs=M4.T1, providers=M4.T2, plan/=M5.T1. Do NOT touch them.
```

## Implementation Blueprint

### Data models and structure

No code. Four commands + verification:

```bash
# ALL commands run from the project root: /home/dustin/projects/stagehand

# (1) ci.yml — global rename (L1 comment + L102-105 coverage-gate paths). dustin/ PRESERVED.
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .github/workflows/ci.yml

# (2) .gitignore — DELETE the stale /stagehand (L5 /stagecoach already present), THEN rename L22/23/40.
sed -i '/^\/stagehand$/d' .gitignore
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .gitignore

# (3) release.yml — REVIEW ONLY (no edit). Confirm 0 refs + dustin/ preserved:
grep -ci stagehand .github/workflows/release.yml          # → 0 (expected; make NO edit)
grep -n 'dustin/homebrew-tap\|dustin/scoop-bucket' .github/workflows/release.yml   # → present (CORRECT — leave alone)

# (4) stale binary cleanup — the ACTUAL stale binary is at the REPO ROOT (gitignored, untracked):
rm -f stagehand stagehand-test                             # -f = no error if absent (stagehand-test doesn't exist)
# (Optional defensive no-op matching the work item's literal instruction — bin/ already has stagecoach*):
rm -f bin/stagehand bin/stagehand-test
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RENAME ci.yml (the functional + cosmetic fix)
  - RUN: sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .github/workflows/ci.yml
  - VERIFY: grep -rni stagehand .github/workflows/ci.yml → no output (0 refs).
  - VERIFY: grep -n 'github.com/dustin/stagecoach/internal/' .github/workflows/ci.yml → L102-105 (match go.mod).
  - VERIFY: grep -n 'Stagecoach CI' .github/workflows/ci.yml → L1 (the comment).
  - VERIFY: grep -c 'github.com/dustin/stagecoach' .github/workflows/ci.yml → 4 (dustin/ PRESERVED — not dabstractor/).
  - GOTCHA: use the GLOBAL sed (not the work item's narrow module-path-only sed) — it also fixes L1's comment.
  - GOTCHA: KEEP dustin/ (the sed touches only stagehand→stagecoach).

Task 2: RENAME .gitignore (delete stale + rename)
  - RUN: sed -i '/^\/stagehand$/d' .gitignore                              # delete stale /stagehand (L5 /stagecoach already present)
  - RUN: sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .gitignore   # rename L22 comment + L23 .stagehand.toml + L40 comment
  - VERIFY: grep -c '^/stagecoach$' .gitignore → 1 (NO duplicate — the delete prevented it).
  - VERIFY: grep -nc '^/stagehand$' .gitignore → 0 (the stale line is gone).
  - VERIFY: grep -n '.stagecoach.toml' .gitignore → the renamed line (was .stagehand.toml).
  - VERIFY: grep -rni stagehand .gitignore → no output (0 refs).
  - GOTCHA: DELETE L4 /stagehand (don't rename — renaming duplicates /stagecoach on L5).

Task 3: REVIEW release.yml (NO edit)
  - RUN: grep -ci stagehand .github/workflows/release.yml → 0 (expected).
  - RUN: grep -n 'dustin/homebrew-tap\|dustin/scoop-bucket' .github/workflows/release.yml → present.
  - CONFIRM: `git diff --exit-code .github/workflows/release.yml` → clean (NO change made).
  - GOTCHA: the `dustin/` SECRETS comments are CORRECT (S2's namespace) — do NOT change to dabstractor/.

Task 4: CLEAN UP the stale untracked binary
  - RUN: rm -f stagehand stagehand-test   (from the project root — the stale binary is ./stagehand, NOT bin/)
  - RUN: rm -f bin/stagehand bin/stagehand-test   (defensive no-op — bin/ already has stagecoach*; -f = safe)
  - VERIFY: ls stagehand stagehand-test 2>/dev/null → no output (gone).
  - GOTCHA: these are UNTRACKED (gitignored) — `git status` won't show their removal; that's expected.

Task 5: VERIFY (no regression + scope)
  - RUN the full Validation Loop (Levels 1-3). go.mod/go.sum/Makefile/.goreleaser.yaml/.go/docs MUST be
      byte-unchanged. Only ci.yml + .gitignore changed among TRACKED files.
```

### Implementation Patterns & Key Details

```yaml
# THE ci.yml coverage-gate fix (L102-105 BEFORE → AFTER):
#   BEFORE: t[1]="github.com/dustin/stagehand/internal/git"      ← MISMATCHES coverage.out → "no coverage data" → fail
#   AFTER:  t[1]="github.com/dustin/stagecoach/internal/git"     ← MATCHES coverage.out → gate works
# (dustin/ PRESERVED; only stagehand→stagecoach.)

# THE .gitignore dedup (BEFORE → AFTER):
#   BEFORE: L4 /stagehand  +  L5 /stagecoach   (stale + current)
#   AFTER:  (L4 deleted)   +  L5 /stagecoach    (single /stagecoach — no duplicate)

# THE release.yml review (NO edit):
#   grep -ci stagehand release.yml → 0; dustin/homebrew-tap + dustin/scoop-bucket preserved (CORRECT per S2).

# THE stale-binary cleanup:
#   rm -f stagehand stagehand-test   (REPO ROOT — not bin/; bin/ already renamed by S1)
```

### Integration Points

```yaml
GO MODULE: go.mod is ALREADY github.com/dustin/stagecoach (M1.T1.S1). ci.yml L102-105 now MATCH its prefix
      ⇒ coverage.out's github.com/dustin/stagecoach/... paths resolve in the awk. No go.mod change.

CI (downstream — NOT this task's code, but this task UNBLOCKS it): the coverage-gate job (ci.yml L77-115)
      will now PASS on the next push to main (was going to FAIL on the stagehand/stagecoach mismatch).

RELEASE PIPELINE: release.yml invokes goreleaser (S2's config). release.yml's dustin/ SECRETS match S2's
      goreleaser owners ⇒ consistent. No release.yml change needed.

FROZEN/LEAVE: go.mod, go.sum, Makefile (S1), .goreleaser.yaml (S2), cmd/, all .go files, docs/, providers/,
      FUTURE_SPEC.md, plan/, PRD.md, release.yml (reviewed — no edit).

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: The renames landed correctly

```bash
# Zero stagehand refs in the EDITED tracked files:
grep -rni stagehand .github/workflows/ci.yml .gitignore    # → no output (0 refs in both)

# ci.yml: coverage-gate paths match go.mod + the comment is fixed + dustin/ preserved:
grep -n 'github.com/dustin/stagecoach/internal/' .github/workflows/ci.yml   # → L102-105
grep -n 'Stagecoach CI' .github/workflows/ci.yml                            # → L1
grep -c 'dabstractor' .github/workflows/ci.yml                              # → 0 (dustin/ preserved)

# .gitignore: single /stagecoach (no duplicate), /stagehand gone, .stagecoach.toml present:
grep -c '^/stagecoach$' .gitignore       # → 1
grep -c '^/stagehand$' .gitignore        # → 0
grep -n '.stagecoach.toml' .gitignore    # → the renamed line
# Expected: 0 stagehand hits in ci.yml + .gitignore; ci.yml L102-105 = github.com/dustin/stagecoach/internal/...;
# .gitignore has exactly one /stagecoach + .stagecoach.toml.
```

### Level 2: release.yml review + YAML sanity

```bash
# release.yml: 0 refs + dustin/ preserved + UNCHANGED:
grep -ci stagehand .github/workflows/release.yml                                       # → 0
grep -n 'dustin/homebrew-tap\|dustin/scoop-bucket' .github/workflows/release.yml      # → present (CORRECT)
git diff --exit-code .github/workflows/release.yml && echo "release.yml UNCHANGED (expected)"

# ci.yml YAML sanity (the sed changed only string literals — L1 comment + L102-105 awk strings — not keys/structure):
actionlint .github/workflows/ci.yml 2>/dev/null || echo "(actionlint not installed — visual review: the sed changed only string literals, not YAML keys/structure)"
# Expected: actionlint clean OR (if absent) the YAML is structurally identical (only string values changed).
```

### Level 3: Scope guard + stale-binary cleanup

```bash
# Stale binary gone (untracked — git status won't show it):
ls stagehand stagehand-test 2>/dev/null || echo "(stale root binaries removed — good)"

# ONLY ci.yml + .gitignore changed among TRACKED files:
git status --short
# Expected: M .github/workflows/ci.yml  +  M .gitignore  (and NOTHING else; release.yml is clean).

# Frozen files UNCHANGED:
git diff --exit-code go.mod go.sum Makefile .goreleaser.yaml cmd/ internal/ pkg/ docs/ providers/ PRD.md .github/workflows/release.yml && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Functional reasoning (the coverage-gate fix)

```bash
# The load-bearing fix is ci.yml L102-105 (the coverage-gate awk's t[i] keys). Verify by reasoning + grep:
#   1. go.mod = github.com/dustin/stagecoach ⇒ Go emits coverage.out with github.com/dustin/stagecoach/... paths.
#   2. ci.yml L102-105 t[i] = github.com/dustin/stagecoach/internal/{git,provider,generate,config} (AFTER the sed).
#   3. The awk's `if(t[i] in tot)` now MATCHES (before the sed, t[i]=.../stagehand/... matched nothing ⇒ fail).
# Confirm the paths line up:
grep '^module ' go.mod                                                # → module github.com/dustin/stagecoach
grep -o 'github.com/dustin/stagecoach/internal/[a-z]*' .github/workflows/ci.yml | sort -u
# Expected: the 4 packages (config, generate, git, provider) — exactly matching go.mod's module prefix.
# (Optional, the ultimate proof — runs the actual coverage gate locally:)
#   go test -coverprofile=coverage.out ./... && (cd .github/workflows && awk -f <(sed -n '/awk ..$/,/^.$/p' ci.yml) ../../coverage.out)
#   ↑ runs the awk from ci.yml against a fresh coverage.out; expect pass (>=85% on the 4 packages). Skip if fiddly;
#   the grep alignment above is sufficient proof.
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep -rni stagehand .github/workflows/ci.yml .gitignore` → 0.
- [ ] ci.yml L102-105 = `github.com/dustin/stagecoach/internal/{git,provider,generate,config}` (match go.mod).
- [ ] ci.yml L1 = `Stagecoach CI`; `dustin/` preserved (grep `dabstractor` → 0).
- [ ] .gitignore: single `/stagecoach` (grep -c → 1); `/stagehand` gone; `.stagecoach.toml` present.
- [ ] release.yml UNCHANGED (0 refs; `dustin/` SECRETS preserved).
- [ ] Stale `./stagehand` binary removed.
- [ ] Only `.github/workflows/ci.yml` + `.gitignore` changed among tracked files; all frozen files byte-unchanged.

### Feature Validation
- [ ] The coverage-gate awk's `t[i]` paths now match coverage.out's `github.com/dustin/stagecoach/...` prefix
      (the CI-breaking bug is fixed — the gate will pass on the next push).
- [ ] release.yml reviewed, no edit (the work item's `dustin/` caution was a red herring — `dustin/` is canonical).
- [ ] .gitignore has no duplicate `/stagecoach` (the dedup gotcha was handled).

### Code Quality Validation
- [ ] The global sed is safe (ci.yml: 5 refs, all the product name/comment, zero partial-word collisions).
- [ ] The .gitignore delete-then-sed avoids the `/stagecoach` duplicate (the naive sed would have created it).
- [ ] The `dustin/` org is deliberately preserved (3 sources: go.mod + S2 + PRD §21.2/§21.3).
- [ ] Scope respected (Makefile=S1, .goreleaser.yaml=S2, docs/providers=M4, plan/=M5 — untouched).

### Documentation
- [ ] No docs change (CI + gitignore are internal; the README install instructions are M4.T1.S1).
- [ ] The release.yml review is documented (commit message / PR note: "reviewed, 0 refs, dustin/ correct per S2").

---

## Anti-Patterns to Avoid

- ❌ **Don't use the work item's narrow ci.yml sed** (`s|github.com/dustin/stagehand|...|g`). It misses L1's
      `Stagehand CI` comment (fails the final grep audit). Use the global `s/stagehand/stagecoach/g;
      s/Stagehand/Stagecoach/g`. (research §1)
- ❌ **Don't treat ci.yml L102-105 as cosmetic.** They are the coverage-gate package keys; a stagehand/stagecoach
      mismatch makes the awk report "no coverage data" for all 4 packages ⇒ CI fails. (research §2)
- ❌ **Don't change `dustin` → `dabstractor`.** The sed touches only `stagehand`; `github.com/dustin/` is
      preserved. go.mod + S2 + PRD §21.2/§21.3 ALL mandate `dustin/`. (research §2/§4)
- ❌ **Don't rename .gitignore L4 `/stagehand` → `/stagecoach`.** `/stagecoach` is already on L5; renaming
      duplicates it. DELETE L4 instead (`sed -i '/^\/stagehand$/d'`), then sed L22/23/40. (research §3)
- ❌ **Don't edit release.yml.** It has 0 stagehand refs; the `dustin/homebrew-tap`/`dustin/scoop-bucket`
      SECRETS comments are CORRECT (S2). The work item's caution is a red herring. Review only. (research §4)
- ❌ **Don't `rm -f bin/stagehand` expecting it to do something.** bin/ already has `stagecoach`/`stagecoach-test`
      (S1). The stale binary is at the REPO ROOT (`./stagehand`). `rm -f stagehand stagehand-test`. (research §5)
- ❌ **Don't touch the Makefile, .goreleaser.yaml, go.mod, .go files, docs/, providers/, or plan/.** Those are
      S1/S2/M1/M2/M4/M5. This task is ci.yml + .gitignore (+ release.yml review + root-binary cleanup) ONLY.
- ❌ **Don't change the YAML structure of ci.yml.** The sed replaces STRING LITERALS only (the L1 comment +
      the L102-105 awk strings), not YAML keys/structure. The `name:`, `on:`, `jobs:`, `steps:` structure is unchanged.

---

## Confidence Score

**10/10** — a text-substitution rename on two static files, with every reference count re-verified against the
actual files (ci.yml: 5 refs; .gitignore: 4 refs; release.yml: 0 refs) and four work-item inaccuracies
corrected with documented evidence: (1) ci.yml needs the global sed (the narrow one misses L1); (2) ci.yml
L102-105 are the functional coverage-gate keys (CI-breaking if unfixed — load-bearing); (3) .gitignore L4
must be DELETED not renamed (`/stagecoach` already on L5 — dedup); (4) the stale binary is at the repo root,
not bin/. The namespace decision (KEEP `dustin/`) is inherited from S2 (3 sources: go.mod + PRD + goreleaser).
Zero file overlap with S1 (Makefile) or S2 (.goreleaser.yaml). Validation is deterministic grep + a YAML
sanity check + optional local coverage-gate run. No residual risk.
