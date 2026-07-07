# P1.M5.T1.S1 — Research Findings
## Bulk rename stagehand → stagecoach in plan/ historical artifacts (Layer 5.5; excludes plan/012)

---

## 0. Task contract (verbatim from item_description)

Mode B (historical-artifacts) mechanical rename. The plan/ directory's PRIOR changesets (plan/001_* –
plan/011_*) contain ~622 tracked files (task breakdowns, PRPs, architecture research) with `stagehand`
references. They are git-tracked but never compiled/executed/shipped. With M1–M4 complete (source, build,
CI, docs renamed), this task sweeps the historical plan/ surface.

LOGIC (the contract's literal command): `find plan/ -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name
'*.txt' | xargs grep -l 'stagehand' | xargs sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g;
s/STAGEHAND/STAGECOACH/g'`.

CAUTION: do NOT modify plan/012_963e3918ec08/ (it intentionally references BOTH names as the rename
documentation). OUTPUT: all plan/ files except 012_* use 'stagecoach' throughout.

---

## 1. CRITICAL: the contract's literal command has THREE correctness gaps

The contract's command is a starting sketch, NOT safe to run verbatim. Three gaps, each independently
breaks the task if unfixed:

### Gap 1 — plan/012 exclusion is MISSING (would corrupt the rename documentation)
The contract's `find plan/ …` includes plan/012_963e3918ec08/. Running the sed on it would transform
"rename **stagehand** → **stagecoach**" into "rename **stagecoach** → **stagecoach**" — destroying the very
rename record this changeset produces. **VERIFIED**: plan/012 contains stagehand references (grep found 11
files; spot-checked: "part of the stagehand→stagecoach project rename", "github.com/dustin/stagehand …
404 occurrences", "Rename stagehand.* git-config keys → stagecoach.*"). These MUST be preserved.
**FIX**: prune plan/012 from the find: `-path plan/012_963e3918ec08 -prune -o …`.

### Gap 2 — `grep -l 'stagehand'` is case-SENSITIVE (misses Stagehand/STAGEHAND-only files)
The grep filter lists files containing lowercase `stagehand` only. A historical file whose ONLY old-name
refs are capitalized (`Stagehand` title, `STAGEHAND_*` env var docs) would be EXCLUDED from the sed list
and left stale. **FIX**: `grep -li 'stagehand'` (case-insensitive) so all three case variants are caught
and renamed.

### Gap 3 — `xargs sed -i …` with empty input (no-input guard)
If grep lists zero files (already renamed, or the historical dirs absent in the run env), the second xargs
receives no input and `sed -i` would error (sed with no file args) or, on some xargs, run sed on stdin.
**FIX**: `xargs -r sed …` (GNU `--no-run-if-empty`); BSD fallback: guard with a test or accept the no-op.

### Bonus — find `-o` grouping
Empirically (tested on GNU find in this env): the UNGROUPED form `find . -name '*.md' -o -name '*.go' -o
-name '*.txt'` DOES print all three extensions (GNU find's implicit `-print` covers the whole OR
expression when no explicit action is present). BUT once `-path … -prune -o` is added (Gap 1's fix) plus
an explicit `-print`, the `-name` conditions MUST be grouped `\( … \)` for the prune/print logic to bind
correctly. Grouping is also portable to BSD find. **FIX**: always use the grouped form in the corrected
command.

---

## 2. The CORRECTED command (the heart of this PRP)

```bash
# Linux / GNU sed + GNU xargs (the CI environment):
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null \
  | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
```
- `-path plan/012_963e3918ec08 -prune -o` → excludes the current changeset's docs (Gap 1).
- `grep -li 'stagehand'` → case-insensitive (Gap 2).
- `xargs -r` → no-op on empty input (Gap 3).
- grouped `\( -name … \)` → prune logic + portability (bonus).
- THREE sed arms → lowercase / Capitalized / ALL-CAPS (the contract's three).

**macOS / BSD sed variant** (if run locally on a Mac): `sed -i '' 's/.../.../g; ...'` (empty backup-ext
arg). BSD xargs lacks `-r`; replace the last pipe with `| xargs sed -i '' …` and accept that an empty
grep result is an error-free no-op only if grep found ≥1 file (verify the count first).

---

## 3. Why the blanket sed is SAFE across all historical files (no compound-token risk)

The previous single-file rename (P1.M4.T2.S2, FUTURE_SPEC.md) verified no compound tokens before trusting
the blanket sed. Across ~622 historical files that exhaustive check is infeasible, BUT the analysis shows
EVERY `stagehand` substring in these artifacts is a token that SHOULD be renamed:
- `.stagehand.toml` → `.stagecoach.toml` (config filename, renamed P1.M2.T2.S1) ✓ desired
- `.stagehandignore` → `.stagecoachignore` (ignore file, renamed P1.M2.T2.S2) ✓ desired
- `stagehand.no_verify` / `stagehand.noVerify` → `stagecoach.*` (git-config keys, renamed P1.M2.T1.S2) ✓
- `STAGEHAND_PROVIDER` / `STAGEHAND_MODEL` / etc. → `STAGECOACH_*` (env vars, renamed P1.M2.T1.S1) ✓
- `github.com/dustin/stagehand` → `github.com/dustin/stagecoach` (module/import path, renamed P1.M1.T1) ✓
  — the contract explicitly says "the sed handles those too."
- `cmd/stagehand`, `pkg/stagehand`, `stagehand` (the binary/prose name) → `stagecoach` ✓
- `commit-pi` (the originating tool) → NOT touched (no `stagehand` substring; sed arms don't match it) ✓

There is NO token in this codebase where a `stagehand` substring should be partially preserved. The three
sed arms are CASE-DISJOINT and order-safe (verified by reasoning): arm 1 `s/stagehand/` matches only
lowercase-initial; arm 2 `s/Stagehand/` needs lowercase `tagehand` after `S` so it does NOT match
`STAGEHAND` (which has `TAGEHAND`); arm 3 `s/STAGEHAND/` matches all-caps. None re-creates another's
pattern (`stagecoach` has no `hand`). **DEFENSIVE**: still run a post-rename sanity grep for any unexpected
residual compound token (Level 4 check) — vanishingly unlikely but cheap to confirm.

---

## 4. The plan/012 preserve target (VERIFIED)

plan/012_963e3918ec08/ is the CURRENT changeset (the stagehand→stagecoach rename project). Its files
INTENTIONALLY reference both names — they DOCUMENT the rename (e.g. "rename stagehand.* git-config keys →
stagecoach.*", "the prefix appears as 404 occurrences", "github.com/dustin/stagehand"). Transforming these
would erase the historical record. Confirmed: 11 files in plan/012 contain `stagehand`. **The corrected
command prunes plan/012 so it is untouched.** Post-rename validation CONFIRMS plan/012 still has stagehand
refs (the exclusion worked).

NOTE: the PRP being written (this file) and its sibling PRPs live in plan/012 — they MUST keep both names.
This is why the exclusion is non-negotiable.

---

## 5. Research-environment caveat (transparency)

The research clone at `/home/dustin/projects/stagecoach` is an EXCERPT: it contains ONLY plan/012_963e3918ec08/
and is NOT a git repository (`git rev-parse` → "fatal: not a git repository"; `git ls-files plan/` → 0;
no `.git`). The historical dirs plan/001_*–plan/011_* described by the contract (~622 files) are ABSENT
here. Therefore:
- The "~622 files" figure is taken from the contract; the implementing agent MUST verify the live file set
  as the FIRST task (the count + a sample), in the full repo where the rename runs.
- The validation gates below are given in TWO forms: git-based (for the full tracked repo the contract
  describes) AND git-independent grep-based (work in any clone, including this excerpt).

The contract is the source of truth for the implementation environment ("tracked in git but never
compiled, executed, or shipped"). The PRP assumes the full repo matches that description.

---

## 6. Validation (two forms — git-based + git-independent)

```bash
# ── BEFORE: verify the scope (run in the full repo) ──
# Count historical plan files with any case-variant stagehand ref (must EXCLUDE 012):
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l
# (Record this N; the contract says ~622 files total, fewer have stagehand. Sample a few to confirm.)

# ── AFTER: zero residue OUTSIDE plan/012 (git-independent) ──
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l     # expect: 0

# ── AFTER: plan/012 PRESERVED (still has stagehand refs — the exclusion worked) ──
grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l   # expect: > 0 (the rename docs intact)

# ── AFTER (git-based, if tracked): scope of the diff ──
git diff --name-only | grep -E '^plan/' | grep -v '^plan/012_963e3918ec08/' | wc -l   # the renamed files
git diff --name-only | grep -E '^plan/012_963e3918ec08/'                              # expect: EMPTY (012 untouched)

# ── AFTER (sanity, defensive): no unexpected compound-token residue repo-wide ──
# (the final whole-repo zero-residue audit is P1.M5.T2.S1; this is a quick sanity grep)
grep -rli 'stagehand' plan/ --include='*.md' --include='*.go' --include='*.toml' --include='*.txt' \
  | grep -v '^plan/012_963e3918ec08/' | wc -l    # expect: 0

# ── Regression check (historical plan/ is NOT compiled) ──
go build ./... && go test ./...   # expect: green, unaffected (plan/ is not in the module build)
```

---

## 7. Confidence & risks

**Confidence: 9/10.** The corrected command is deterministic and the three gaps are precisely identified
and fixed. The preserve-target (plan/012) is verified. The no-compound-token safety is established by
analysis (every substring is a desired rename) with a defensive post-check.

**Risks:**
- **Environment mismatch.** The research clone lacks plan/001–011 + git; the implementing agent must
  verify the live file set first (Task 1). If the historical dirs are absent in the run env too, the task
  is a verified no-op (document it — the contract allows "reviewed, no changes needed" framing).
- **Case-only-variant files missed by a naive grep.** Mitigated by `grep -li` (Gap 2). If the contract's
  literal `grep -l` is used instead, capitalized-only files stay stale — the PRP forbids this.
- **plan/012 corruption.** Mitigated by the `-prune` exclusion (Gap 1). The PRP's validation asserts
  plan/012 STILL contains stagehand refs post-rename.
- **BSD vs GNU sed (local macOS runs).** `sed -i ''` + no `xargs -r`. The CI is Linux (GNU); local Mac
  runs need the BSD variant. PRP notes both.
