name: "P2.M3.T3.S1 — Comprehensive grep audit + build/test verification (Mode B final gate)"
description: |
  Mode B FINAL VERIFICATION — the catch-all gate that confirms the whole P2 provider-lineup
  correction delta (gemini removal + agy v1.1.0 re-verification) is coherent across PRD, code,
  manifests, and docs. This is primarily a READ-ONLY verification task with a CONDITIONAL minimal
  fix: it fixes a stray reference ONLY IF the grep audit reveals one that implies `gemini` is a
  shipped built-in. On the current baseline there are ZERO such references, so the EXPECTED outcome
  is zero new edits — every remaining `gemini`/`Gemini` hit is a MODEL NAME (e.g. "Gemini 3.5 Flash
  (Low)") or a LINEAGE/HISTORY comment (EOL / superseded / fork), all of which are correct and must
  NOT be touched.

  Three verification passes (CONTRACT, from the work item):
    (a) Grep audit: `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/` — classify
        EVERY hit; fix any that imply `gemini` is a shipped built-in provider; leave model names +
        lineage/history comments untouched.
    (b) Count check: confirm docs/cli.md, docs/configuration.md, docs/how-it-works.md, docs/README.md,
        docs/providers.md, README.md do NOT list `gemini` as a built-in and do NOT say 'eight'/'8'
        providers.
    (c) Smoke test: `make build && ./bin/stagecoach providers list` (exactly seven built-ins, no
        gemini) + `./bin/stagecoach providers show agy` (`print_flag = ''`, `model_flag = '--model'`,
        `--mode plan` in bare_flags) + `go test ./internal/provider/... ./internal/config/...
        ./internal/cmd/...` (all pass).

  Inputs from prior siblings (treat as DONE / CONTRACT): P2.M3.T1 (docs/providers.md D1–D7,
  COMMITTED), P2.M3.T2 (docs/README.md D8/D9, COMMITTED), P2.M1 (gemini built-in + fixture removal,
  COMMITTED). The working tree is expected to be CLEAN when this task starts.

  Out of scope: PRD.md (READ-ONLY), tasks.json / prd_snapshot.md / .gitignore (forbidden), and any
  change to the COMPILED provider set / manifest schema / agy manifest (those are the SOURCE OF TRUTH
  this task verifies AGAINST, not edits to make).

---

## Goal

**Feature Goal**: Prove — by deterministic, reproducible checks — that the P2 provider-lineup
correction has fully landed and left no stale `gemini`-as-built-in framing anywhere in
`docs/`, `README.md`, `providers/`, `internal/`, or `pkg/`, and that the shipped binary + the
targeted Go test suites agree (seven built-ins, agy's v1.1.0-corrected manifest, green tests).

**Deliverable**: A completed three-pass verification (grep audit + count check + build/list/show/test
smoke) plus a short written summary of the result. The only code/doc edits, if any, are minimal
in-place corrections of a stray stale reference found by pass (a). On the current baseline, **no
edits are required** — the deliverable is the clean verification itself.

**Success Definition**: ALL of the following hold after the task:
1. Pass (a): every `gemini`/`Gemini` grep hit is classified as a MODEL NAME or a LINEAGE/HISTORY
   comment; none implies `gemini` is a shipped built-in. (Any stale hit, if one exists, is fixed.)
2. Pass (b): none of the six docs files list `gemini` as a built-in or say 'eight'/'8' providers.
3. Pass (c): `make build` exits 0; `./bin/stagecoach providers list` prints exactly seven NAME rows
   with no `gemini`; `./bin/stagecoach providers show agy` emits `print_flag = ''`,
   `model_flag = '--model'`, and a `bare_flags` line containing `--mode` and `plan`; and
   `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...` reports all PASS.
4. Scope guard: `git diff --name-only` shows ONLY the file(s) of any stale reference fixed by pass (a)
   — or NOTHING new if no fix was needed. Never `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer / reviewer responsible for shipping the P2 milestone.

**Use Case**: Before declaring the gemini→agy lineup correction done, run one deterministic catch-all
gate that proves the delta is coherent across PRD ↔ code ↔ manifests ↔ docs and that the binary +
tests reflect it. This task IS that gate.

**User Journey**: Run the three passes → read the classification + smoke output → confirm green →
record the summary. If a stray stale reference appears (regression from an earlier edit), fix it in
place and re-run the gate until clean.

**Pain Points Addressed**: Prevents a "coherent code delta" from shipping with a stale doc sentence
that still calls `gemini` a built-in or still says "eight providers". This is the single check that
guards the whole milestone's cross-surface consistency.

## Why

- **Catch-all coherence gate (Mode B).** P2.M1 (code-side gemini removal) and P2.M2 (agy manifest
  re-verification) landed in code; P2.M3.T1/T2 swept the known residual docs drift in
  `docs/providers.md` and `docs/README.md`. This task is the FINAL sweep that catches anything those
  targeted edits missed and confirms they introduced no regressions — across the full surface
  (`docs/`, `README.md`, `providers/`, `internal/`, `pkg/`), not just the files the audit pre-flagged.
- **Greps lie by omission; classification closes the gap.** A naive "no `gemini` anywhere" rule would
  FALSE-FAIL (model labels like "Gemini 3.5 Flash (Low)" and lineage prose must survive). This task's
  value is the classification rubric: it asserts no hit implies gemini-as-built-in while protecting
  the legitimate model-name + lineage references.
- **Binary + tests are the hard floor.** Docs can be persuasive but wrong; the compiled
  `BuiltinManifests()` / `preferredBuiltins` (7 entries, no gemini), the `providers list/show`
  rendering, and the targeted test suites are the authority this gate confirms against.

## What

A read-mostly verification with a conditional one-line fix. The agent runs three passes and writes a
summary. It edits a file ONLY if pass (a) finds a sentence that states or implies `gemini` is a
current shipped built-in (the rubric below defines this precisely). Model-name labels and
lineage/history comments are NEVER edits — they are the expected, correct residue of a clean removal.

### Success Criteria

- [ ] Pass (a): `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/` run; every hit
      classified (model name | lineage/history | STALE); any STALE hit fixed; none remaining.
- [ ] Pass (b): the six docs files contain no `gemini`-as-built-in and no 'eight'/'8' providers
      framing (negative grep over the six files returns zero).
- [ ] Pass (c)-build: `make build` exits 0 and produces `./bin/stagecoach`.
- [ ] Pass (c)-list: `./bin/stagecoach providers list` shows exactly seven data rows; no `gemini` row.
- [ ] Pass (c)-show: `./bin/stagecoach providers show agy` shows `print_flag = ''`, `model_flag =
      '--model'`, and `bare_flags` containing `--mode` + `plan`.
- [ ] Pass (c)-test: `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...`
      reports `ok` for all three packages.
- [ ] Scope guard: `git diff --name-only` lists only any file fixed in pass (a), or nothing new;
      never a forbidden file.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This is a verification task whose commands, expected output, and decision
rubric are all specified verbatim below. The agent needs no prior codebase knowledge to run the
three passes; the only judgment call (classifying a grep hit) is fully pinned by the rubric + the
baseline hit table in the research notes. The "seven built-ins" / "agy manifest" truths are verified
empirically by the binary itself (`providers list` / `providers show agy`), so the agent does not
need to trust any prose — it asserts on the binary's actual output.

### Documentation & References

```yaml
# MUST READ — the research notes capture the verbatim baseline output + the full hit classification.
- docfile: plan/013_b8a415cc6e79/P2M3T3S1/research/baseline_evidence.md
  why: The single source of truth for this task. Contains: (1) the full table of every gemini/Gemini
       grep hit with its category (model name vs lineage/history) + verdict, (2) the EXACT providers
       list / providers show agy output captured from the binary (incl. go-toml v2 single-quote
       literals), (3) the count-check table for the six docs files, (4) the dependency/working-tree
       state expected at T3 start.
  section: "§1 (grep hit table), §3 (smoke output), §2 (count check), §5 (working-tree state)"

# Source of truth 1 — the compiled built-in set (READ-ONLY; verify AGAINST, do not edit)
- file: internal/provider/builtin.go
  why: BuiltinManifests() returns 7 entries (pi, claude, opencode, codex, cursor, agy, qwen-code);
       comment "Seven providers". No builtinGemini. builtinAgy() defines the corrected v1.1.0
       manifest (PrintFlag=strPtr(""), ModelFlag=strPtr("--model"), BareFlags=["--mode","plan"],
       Experimental=true). This is what `providers list/show agy` render.
  section: "func BuiltinManifests() (line ~18), func builtinAgy() (line ~200)"

# Source of truth 2 — the provider preference order (READ-ONLY)
- file: internal/provider/registry.go
  why: preferredBuiltins = 7 entries (no gemini). MarshalTOML() is what `providers show` calls (go-toml
       v2: emits single-quoted literals, OMITS nil pointers, EMITS non-nil-empty strPtr("") as key='').
  section: "preferredBuiltins (line 11), MarshalTOML (line ~90)"

# The CLI commands under test (READ-ONLY)
- file: internal/cmd/providers.go
  why: `providers list` prints a tabwriter NAME/DETECTED/DEFAULT table over reg.List() (sorted asc by
       Name); `providers show <name>` prints reg.MarshalTOML(name). Explains WHY the DETECTED column
       and (default) marker are environment-variant (depend on $PATH) and must NOT be asserted on.
  section: "printProvidersList (line ~150), runProvidersShow (line ~75)"

# The build entry point (READ-ONLY)
- file: Makefile
  why: `make build` = `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`.
       Confirms the exact binary path the smoke test uses (./bin/stagecoach).

# The six docs files under the count check (pass b) — READ for verification, edit only if stale
- file: docs/cli.md
  why: Pass (b) target — must not list gemini as built-in or say eight/8 providers.
- file: docs/configuration.md
  why: Pass (b) target (same as above).
- file: docs/how-it-works.md
  why: Pass (b) target (same as above).
- file: docs/README.md
  why: Pass (b) target. Line 35 (the "Provider manifests" index row) was fixed by sibling P2.M3.T2
       to read "22-field manifest schema ... the 7 built-in providers" — this task RE-CONFIRMS that.
- file: docs/providers.md
  why: Pass (b) target. Lines 3/7/74/92 + 85/88 were fixed by sibling P2.M3.T1 — this task RE-CONFIRMS
       them. Also the densest source of `Gemini 3.5 Flash` MODEL NAMES (legit) + the qwen-code lineage
       comment (legit) — classify, do not edit.
- file: README.md
  why: Pass (b) target (top-level README). Line 355 carries the explicit "gemini ... no longer shipped
       — superseded by agy" note (LEGIT lineage/history). Confirms "Seven built-ins". Do not edit.

# The audit that scoped the prior residual drift (READ-ONLY context)
- docfile: plan/013_b8a415cc6e79/architecture/docs_drift_audit.md
  why: Established that cli.md / configuration.md / how-it-works.md / README.md were CLEAN and that all
       residual drift (D1–D9) lived in providers.md + docs/README.md (owned by T1/T2). This task is the
       re-verification that the clean files STAYED clean and T1/T2 introduced no regressions.
  section: "§2 (clean files) + §3 (README.md) + Summary table (D1–D9)"

# Sibling contracts (treat as DONE)
- docfile: plan/013_b8a415cc6e79/P2M3T2S1/PRP.md
  why: Defines the docs/README.md line-35 edit (D8/D9) this task re-confirms. Its success criteria
       (line 35 reads "22-field" + "7 built-in") are a subset of this task's pass (b).
- docfile: plan/013_b8a415cc6e79/P2M3T1S2/PRP.md
  why: Defines the docs/providers.md edits (D1–D7) this task re-confirms.
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
docs/
  cli.md                 # pass (b) — count + gemini-as-built-in check (expected CLEAN)
  configuration.md       # pass (b) — same (expected CLEAN)
  how-it-works.md        # pass (b) — same (expected CLEAN)
  providers.md           # pass (b) + pass (a) gemini model-names (legit) — re-confirm T1's D1–D7
  README.md              # pass (b) — re-confirm T2's line-35 (D8/D9)
README.md                # pass (b) — top-level; line 355 lineage note (legit); "Seven built-ins"
providers/               # pass (a) — agy/claude/codex/cursor/opencode/pi/qwen-code .toml (NO gemini.toml)
internal/provider/       # pass (a) — builtin.go (7 built-ins, agy manifest), registry.go, manifest.go
internal/config/         # pass (c) — go test target
internal/cmd/            # pass (c) — providers.go (list/show), go test target
pkg/                     # pass (a) — public lib surface (grep gemini here too)
Makefile                 # `make build` target
plan/013_b8a415cc6e79/
  architecture/docs_drift_audit.md          # prior drift scope (READ-ONLY)
  P2M3T1S2/PRP.md                           # sibling: docs/providers.md (CONTRACT — DONE)
  P2M3T2S1/PRP.md                           # sibling: docs/README.md (CONTRACT — DONE)
  P2M3T3S1/
    PRP.md                                  # ← THIS file
    research/baseline_evidence.md           # verbatim baseline output + hit classification
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NO new files are expected. This is a verification gate.
# IF (and only if) pass (a) finds a stale gemini-as-built-in reference, the agent makes a minimal
# in-place edit to THAT file only (a .go comment, a doc sentence, etc.). The expected outcome on the
# current baseline is: NO file changes at all — the deliverable is the clean verification summary.
#
# The only artifacts this task always produces are:
#   - the console output of the three passes (read it, classify it, summarize it)
#   - a short written summary of the result (can be reported in the task transcript; no file required)
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — A naive "zero gemini matches" rule FALSE-FAILS. Model-name labels and lineage/history
#   prose are the EXPECTED, correct residue of a clean removal and MUST survive. Examples that are
#   NOT stale (do NOT edit):
#     - "Gemini 3.5 Flash (Low/High/Medium)"  → a MODEL display label from `agy models` (agy's models)
#     - "Google's gemini / Gemini CLI is no longer shipped — superseded by agy" (README.md:355)
#     - "a Gemini-CLI fork for Qwen3-Coder" / "gemini-cli lineage" / "DIVERGENCE FROM GEMINI-CLI"
#     - "gemini (removed; superseded by agy)" in a test-fixture comment
#   A hit is STALE only if its sentence STATES/IMPLIES gemini is a CURRENT shipped built-in (see rubric
#   in Task 2). The full baseline hit table is in research/baseline_evidence.md §1.

# GOTCHA 2 — go-toml v2 emits SINGLE-QUOTED literal strings and OMITS nil pointers.
#   `./bin/stagecoach providers show agy` prints (verbatim):
#       print_flag = ''                 ← empty (NON-NIL empty strPtr(""); NOT omitted)
#       model_flag = '--model'          ← single-quoted
#       bare_flags = ['--mode', 'plan'] ← array of single-quoted
#       system_prompt_flag = ''         ← also empty (strPtr("")); not asserted but EXPECTED
#       provider_flag = ''              ← also empty (strPtr("")); not asserted but EXPECTED
#   The contract's loose phrase "print = \"\"" maps to the literal `print_flag = ''`. The contract's
#   "--mode plan in bare essentials" maps to `bare_flags = ['--mode', 'plan']`. Match the SINGLE-QUOTED
#   literals; do NOT expect double quotes.

# GOTCHA 3 — `providers list` DETECTED + DEFAULT columns are ENVIRONMENT-VARIANT.
#   The ✓/✗ marks depend on what's on $PATH at run time; the `(default)` marker is the first DETECTED
#   preferred built-in (often `pi`). DO NOT assert on these columns. Assert ONLY on the NAME column:
#   exactly 7 data rows, names = {agy, claude, codex, cursor, opencode, pi, qwen-code}, no `gemini`.

# GOTCHA 4 — go test may report CACHED results ("(cached)"). That is still a PASS ("ok ... (cached)").
#   Do not treat "(cached)" as a failure. If you want a clean re-run, use `go test -count=1 ...`.

# GOTCHA 5 — The grep SCOPE must include pkg/. The contract's pass (a) is
#   `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/`. Do NOT drop pkg/ (the public
#   library surface). Do NOT grep plan/ (immutable planning snapshots — out of scope, and full of
#   legitimate "gemini" history).

# GOTCHA 6 — `git grep` (not plain `grep`) so the search respects .gitignore and the tracked tree.
#   If `git grep` is unavailable, fall back to `grep -rni --exclude-dir=plan --exclude-dir=.git`.

# GOTCHA 7 — agy is EXPERIMENTAL (`experimental = true`). That is EXPECTED and CORRECT (§12.5.1.1 item
#   4 — tooled/stager flags — still open). Do NOT treat experimental=true as a failure or try to clear it.

# GOTCHA 8 — Two README files exist. README.md (repo root) and docs/README.md are DIFFERENT files; the
#   contract count-check (pass b) names BOTH explicitly. Both are expected clean — do not conflate them.
```

## Implementation Blueprint

### Data models and structure

None. No data models, schemas, or code are produced. This is a verification gate with an optional,
minimal, in-place doc/comment correction. The "data" is the grep output + binary output + test
results, which the agent reads, classifies, and summarizes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ESTABLISH BASELINE — confirm the working tree state and that the binary builds
  - RUN: cd /home/dustin/projects/stagecoach && git status --short
  - NOTE: prior siblings (P2.M1, P2.M3.T1, P2.M3.T2) are expected COMMITTED, so `git status` should be
          clean (or show only unrelated working-tree edits). If it shows an UNCOMMITTED docs/README.md
          or docs/providers.md change, that means a sibling isn't committed yet — still proceed (the
          verification is valid against the working tree), but NOTE it in the summary.
  - RUN: make build
  - EXPECT: exits 0; produces ./bin/stagecoach. If build FAILS, that is a hard regression — STOP and
            report (the P2 delta broke compilation; outside this task's fix scope).

Task 2: PASS (a) — Grep audit + CLASSIFICATION (the judgment-call task)
  - RUN: git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/
    (fallback if git grep missing: grep -rni --exclude-dir=plan --exclude-dir=.git 'gemini' docs/
     README.md providers/ internal/ pkg/)
  - FOR EACH HIT, classify using this RUBRIC:
      MODEL NAME     → a quoted label like "Gemini 3.5 Flash (Low/High/Medium)" or "gemini-3.5-flash"
                       (an agy/qwen model token). LEGIT — leave it.
      LINEAGE/HISTORY → a sentence stating gemini/Gemini CLI was REMOVED / is EOL / was SUPERSEDED by
                        agy / is a FORK lineage / "DIVERGED from gemini-cli". LEGIT — leave it.
      STALE          → a sentence that STATES or IMPLIES gemini is a CURRENT shipped built-in. Signals:
                       - "gemini is built in" / "the N providers ... gemini"
                       - an enumeration LIST of built-ins that includes `gemini`
                       - a reference to a (nonexistent) providers/gemini.toml as shipped
                       - a `"gemini"` entry in a built-in/preferred list in CODE (not a comment)
                       → FIX in place (see Task 3).
  - CROSS-CHECK against research/baseline_evidence.md §1 — every baseline hit is pre-classified. Any
    NEW hit not in that table gets the rubric applied fresh.
  - EXPECT (baseline): ZERO stale hits. All hits are model names or lineage/history.
  - ALSO RUN (sanity): ls providers/ | grep -i gemini   → EXPECT: no output (no providers/gemini.toml).
  - ALSO RUN (code authority): grep -nE '"gemini"' internal/provider/builtin.go internal/provider/registry.go
    → EXPECT: no matches in the built-in map / preferredBuiltins slice. (Comment mentions are fine.)

Task 3: CONDITIONAL FIX (only if Task 2 found a STALE hit — on the baseline, SKIP this task entirely)
  - FOR each stale hit: make the MINIMAL in-place edit that removes the stale implication, preserving
    the surrounding sentence's purpose. Examples of correct minimal fixes:
      - "the eight providers: pi, claude, ..., gemini, ..." → drop gemini from the list AND fix the
        count to seven (mirror docs/providers.md's "Seven providers" wording).
      - "gemini is a built-in provider" → reword to "gemini (Gemini CLI) is no longer shipped; use agy"
        (mirror README.md:355), OR delete the sentence if it is otherwise redundant.
  - DO NOT touch MODEL NAMES ("Gemini 3.5 Flash (Low)") or LEGIT lineage/history prose — even if they
    match the grep. Editing them is a false positive and a scope violation.
  - DO NOT edit the COMPILED provider set, the agy manifest, PRD.md, tasks.json, prd_snapshot.md, or
    .gitignore. A stale hit in CODE is fixed by editing the COMMENT (not the logic) — if the stale hit
    is actual CODE logic (e.g. a real "gemini" built-in entry), that is a P2.M1 regression and must be
    REPORTED, not silently fixed here.
  - AFTER any fix: re-run Task 2 to confirm zero stale hits remain.

Task 4: PASS (b) — Count check across the six docs files
  - RUN (negative — expect ZERO matches):
      grep -rniE 'eight built|the 8 built-in|eight providers|\b8 built' \
        docs/cli.md docs/configuration.md docs/how-it-works.md docs/README.md docs/providers.md README.md
  - RUN (negative — gemini-as-built-in, expect ZERO matches):
      grep -rniE 'gemini[[:space:]]*(is[[:space:]]+)?(a[[:space:]]+)?built' \
        docs/cli.md docs/configuration.md docs/how-it-works.md docs/README.md docs/providers.md README.md
  - RUN (positive — re-confirm T1/T2 landed; expect matches):
      grep -nE 'the 7 built-in providers' docs/README.md          # line 35 (T2's D8)
      grep -nE '22-field manifest schema' docs/README.md          # line 35 (T2's D9)
      grep -nE 'the 7 built-in providers' docs/providers.md       # line 3 (T1's D4)
      grep -nE 'Seven providers are compiled in' docs/providers.md# line 7 (T1's D5)
      grep -nE '## The 7 built-in providers' docs/providers.md    # line 74 (T1's D6)
      grep -nE 'seven built-in providers achieve' docs/providers.md# line 92 (T1's D7)
      grep -nE 'Seven built-ins are auto-detected' README.md      # line ~355
  - EXPECT: both negative greps return zero; all seven positive greps return exactly one match each.

Task 5: PASS (c) — Smoke test: list + show agy
  - RUN: ./bin/stagecoach providers list
  - ASSERT: count data rows (skip the "NAME ... DEFAULT" header). EXPECT exactly 7 rows. Use:
      ./bin/stagecoach providers list | tail -n +2 | wc -l   # → 7
  - ASSERT: no gemini row:
      ./bin/stagecoach providers list | grep -iw gemini     # → no output
  - ASSERT: the seven expected names are present (order is sorted ascending by Name):
      for p in agy claude codex cursor opencode pi qwen-code; do
        ./bin/stagecoach providers list | grep -qw "$p" || echo "MISSING: $p"
      done   # → no MISSING lines
  - RUN: ./bin/stagecoach providers show agy
  - ASSERT (single-quoted go-toml v2 literals — see Gotcha 2):
      ./bin/stagecoach providers show agy | grep -Fx "print_flag = ''"          # empty print flag
      ./bin/stagecoach providers show agy | grep -Fx "model_flag = '--model'"   # model flag
      ./bin/stagecoach providers show agy | grep -F "bare_flags = ['--mode', 'plan']"  # bare essentials
      ./bin/stagecoach providers show agy | grep -Fx "experimental = true"      # expected (item 4 open)
  - NOTE: do NOT assert on the DETECTED column or (default) marker (environment-variant, Gotcha 3).
  - IF any assertion fails: the agy manifest / built-in set does not match §12.5.1 — that is a P2.M2
    regression; REPORT it (do not edit the manifest here — it is the source of truth this task verifies).

Task 6: PASS (c) — Targeted Go test suites
  - RUN: go test ./internal/provider/... ./internal/config/... ./internal/cmd/...
  - EXPECT: three lines, each "ok  github.com/dustin/stagecoach/internal/<pkg>" (optionally "(cached)").
  - IF a package FAILS: read the failure; a test failure here means a P2 edit broke behavior. REPORT it
    with the failing test name + assertion. Fixing test logic is OUT OF SCOPE for this verification
    task unless the failure is directly caused by a stale gemini reference this task already fixed
    (re-run after the fix).

Task 7: WRITE THE SUMMARY + enforce the scope guard
  - RUN: git diff --name-only   (from repo root)
  - EXPECT: either EMPTY (no fix needed — the normal baseline outcome) OR exactly the file(s) fixed in
            Task 3. Run the forbidden-file guard:
      git diff --name-only | grep -E 'PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' \
        && echo "FAIL: forbidden file touched" || echo "ok: no forbidden file touched"
  - SUMMARIZE: a short result block covering (a) grep classification verdict + any fix, (b) count-check
    verdict, (c) build/list/show/test verdicts with the exact counts. This is the task's deliverable.
```

### Implementation Patterns & Key Details

```bash
# PATTERN: classification is the only judgment call. Pin it to the rubric in Task 2 and the
# pre-classified table in research/baseline_evidence.md §1. When in doubt, a hit is LEGIT (model name
# or lineage) unless its sentence explicitly enumerates gemini among current built-ins.

# PATTERN: assert on the BINARY, not on prose. `providers list` (7 rows, no gemini) and
# `providers show agy` (print_flag='', model_flag='--model', bare_flags=[--mode,plan]) ARE the proof
# that the compiled provider set + agy manifest are correct. Docs are secondary confirmation.

# CRITICAL: match go-toml v2's SINGLE-QUOTED output. `print_flag = ''` (two single quotes), not `""`.
# `model_flag = '--model'`. `bare_flags = ['--mode', 'plan']`. Using -Fx/-F exact match guards this.

# CRITICAL: never edit PRD.md, tasks.json, prd_snapshot.md, or .gitignore. Never edit the compiled
# provider set or the agy manifest — they are the source of truth this task VERIFIES, not edits.
```

### Integration Points

```yaml
DATABASE: none
CONFIG:   none   # no config file changed (unless a stale gemini reference is in a config doc/comment)
ROUTES:   none
# The "integration" this task validates is cross-surface COHERENCE: after P2.M1/M2/M3, the same truth
# ("seven built-ins, no gemini; agy = v1.1.0-corrected manifest") holds in PRD §12.5/§12.5.1, the
# compiled builtin.go/registry.go, the rendered `providers list/show`, the providers/*.toml reference
# files, and all six docs files. No new integration is created.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# This is a verification task; no new code is written in the baseline path, so there is nothing to lint.
# IF Task 3 fixed a .go COMMENT, re-confirm the package still compiles + lints:
go build ./...            # EXPECT: clean
golangci-lint run ./... 2>&1 | tail -3   # EXPECT: no issues (or run `make lint`)
# IF Task 3 fixed a docs file, optional markdown sanity (NOT in CI; .markdownlint.json sets MD013/33/60 off):
npx --yes markdownlint-cli2 'docs/*.md' 'README.md' 2>/dev/null | grep -iE 'MD0[0-9]' || echo "ok: no blocking MD issues"
```

### Level 2: The Three Verification Passes (the core gate — run ALL of these)

```bash
cd /home/dustin/projects/stagecoach
echo "===== PASS (a): grep audit ====="
git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/ \
  || echo "(git grep found no matches — also acceptable)"
echo "--- sanity: no providers/gemini.toml ---"
ls providers/ | grep -i gemini && echo "FAIL: gemini.toml exists" || echo "ok: no gemini.toml"
echo "--- sanity: no gemini in compiled built-in/preferred code ---"
grep -nE '"gemini"' internal/provider/builtin.go internal/provider/registry.go \
  && echo "FAIL: gemini in compiled provider code" || echo "ok: no gemini built-in/preferred entry"

echo "===== PASS (b): count check (negatives must be EMPTY) ====="
echo "--- (b1) eight/8 provider framing (expect zero) ---"
grep -rniE 'eight built|the 8 built-in|eight providers|\b8 built' \
  docs/cli.md docs/configuration.md docs/how-it-works.md docs/README.md docs/providers.md README.md \
  && echo "FAIL: stale 8/eight framing" || echo "ok: no 8/eight provider framing"
echo "--- (b2) gemini-as-built-in (expect zero) ---"
grep -rniE 'gemini[[:space:]]*(is[[:space:]]+)?(a[[:space:]]+)?built' \
  docs/cli.md docs/configuration.md docs/how-it-works.md docs/README.md docs/providers.md README.md \
  && echo "FAIL: gemini-as-built-in framing" || echo "ok: no gemini-as-built-in framing"

echo "===== PASS (c): smoke test ====="
make build 2>&1 | tail -2
echo "--- providers list: expect 7 data rows, no gemini ---"
./bin/stagecoach providers list
ROWS=$(./bin/stagecoach providers list | tail -n +2 | wc -l)
echo "data rows = $ROWS  (expect 7)"
./bin/stagecoach providers list | grep -iw gemini && echo "FAIL: gemini row present" || echo "ok: no gemini row"
echo "--- providers show agy: expect print_flag='', model_flag='--model', bare_flags=[--mode,plan] ---"
./bin/stagecoach providers show agy | grep -Fx "print_flag = ''"          && echo "ok: print_flag empty"   || echo "FAIL: print_flag"
./bin/stagecoach providers show agy | grep -Fx "model_flag = '--model'"   && echo "ok: model_flag --model" || echo "FAIL: model_flag"
./bin/stagecoach providers show agy | grep -F "bare_flags = ['--mode', 'plan']" && echo "ok: bare --mode plan" || echo "FAIL: bare_flags"
./bin/stagecoach providers show agy | grep -Fx "experimental = true"      && echo "ok: experimental true"   || echo "FAIL: experimental"
echo "--- targeted go test suites (expect 3x ok) ---"
go test ./internal/provider/... ./internal/config/... ./internal/cmd/...

# Expected: PASS (a) sanity lines print "ok"; PASS (b) both negatives "ok"; PASS (c) build clean,
#           list = 7 rows / no gemini, show agy = all four "ok" lines, go test = 3 "ok" lines.
```

### Level 3: Scope & Regression (System Validation)

```bash
cd /home/dustin/projects/stagecoach
echo "--- scope guard: only files fixed by pass (a), or nothing new ---"
git diff --name-only
echo "--- forbidden-file guard (expect ok) ---"
git diff --name-only | grep -E 'PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' \
  && echo "FAIL: forbidden file touched" || echo "ok: no forbidden file touched"
echo "--- full build sanity (optional, broader than the contract's make build) ---"
go build ./... 2>&1 | tail -3   # EXPECT: clean

# Expected: git diff --name-only is either empty (baseline) or lists only pass-(a) fix files; the
# forbidden-file guard prints "ok"; go build ./... is clean.
```

### Level 4: Cross-Surface Coherence Proof (Domain-Specific Validation)

```bash
cd /home/dustin/projects/stagecoach
echo "=== Prove the SAME truth holds in code, binary, manifests, and docs ==="
echo "--- (1) compiled built-in count = 7 (builtin.go comment) ---"
grep -nE 'Seven providers' internal/provider/builtin.go | head -1
echo "--- (2) compiled preference order = 7 (registry.go) ---"
grep -nE 'preferredBuiltins = \[\]' internal/provider/registry.go
echo "--- (3) binary providers list = 7 rows, no gemini ---"
./bin/stagecoach providers list | tail -n +2 | wc -l
echo "--- (4) providers/ dir = 7 .toml, no gemini ---"
ls providers/*.toml | wc -l
ls providers/ | grep -i gemini || echo "(no gemini.toml — correct)"
echo "--- (5) docs agree on 7 (docs/providers.md + README.md) ---"
grep -nE 'the 7 built-in providers|Seven providers are compiled' docs/providers.md | head -2
grep -nE 'Seven built-ins are auto-detected' README.md | head -1
echo "--- (6) agy manifest coherent across binary + toml + builtin.go ---"
./bin/stagecoach providers show agy | grep -E "print_flag|model_flag|bare_flags"
grep -nE 'model_flag|^print_flag' providers/agy.toml

# Expected: (1) "Seven providers"; (2) 7-entry preferredBuiltins; (3) 7; (4) 7 .toml, no gemini;
#           (5) providers.md + README.md say 7; (6) agy print_flag empty / model_flag --model /
#           bare_flags --mode plan in ALL THREE surfaces (binary, toml, builtin.go).
```

## Final Validation Checklist

### Technical Validation

- [ ] Pass (a): `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/` run and every hit
      classified (model name | lineage/history | STALE); zero STALE hits remaining (any found was fixed).
- [ ] Pass (a) sanity: no `providers/gemini.toml`; no `"gemini"` entry in builtin.go/registry.go code.
- [ ] Pass (b): both negative greps (8/eight framing; gemini-as-built-in) return ZERO across the six docs.
- [ ] Pass (b): the seven positive re-confirm greps each return one match (T1/T2 edits still present).
- [ ] Pass (c)-build: `make build` exits 0; `./bin/stagecoach` exists.
- [ ] Pass (c)-list: `providers list` shows exactly 7 data rows; no `gemini` row.
- [ ] Pass (c)-show: `providers show agy` shows `print_flag = ''`, `model_flag = '--model'`,
      `bare_flags = ['--mode', 'plan']`, `experimental = true`.
- [ ] Pass (c)-test: `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...` = 3× `ok`.
- [ ] Scope guard: `git diff --name-only` lists only any pass-(a) fix file(s) or is empty.
- [ ] Forbidden-file guard: no `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

### Feature Validation

- [ ] The P2 delta is coherent: seven built-ins + no gemini is true in code, binary, manifests, AND docs.
- [ ] agy's v1.1.0-corrected manifest (`print_flag=''`, `--model`, `--mode plan`) renders identically
      from the binary, the reference `providers/agy.toml`, and `builtin.go`.
- [ ] The four docs files the audit called CLEAN (cli.md, configuration.md, how-it-works.md, README.md)
      REMAINED clean — P2.M3.T1/T2 introduced no regressions there.
- [ ] Written summary records the verdict for all three passes (with exact row counts + classification).

### Code Quality Validation

- [ ] (If a fix was made) it is the minimal in-place correction; it preserves the sentence's purpose.
- [ ] (If a fix was made) no MODEL-NAME label or LEGIT lineage/history prose was altered (no false positives).
- [ ] No compiled provider set / agy manifest / schema / logic was changed (they are verified, not edited).
- [ ] Classification reasoning is recorded for any non-baseline hit (transparency for review).

### Documentation & Deployment

- [ ] The verification summary is the deliverable; no new docs files are created.
- [ ] If a stale doc reference was fixed, the corrected sentence now mirrors the canonical wording
      (README.md:355 "no longer shipped — superseded by agy" or providers.md "Seven providers").
- [ ] No new environment variables, config keys, or build steps introduced.

---

## Anti-Patterns to Avoid

- ❌ Don't run a "zero `gemini` matches" rule and call stale anything that matches — model-name labels
  (`Gemini 3.5 Flash (Low)`) and lineage/history prose ("superseded by agy", "Gemini-CLI fork") are
  the EXPECTED, correct residue of a clean removal. Use the Task 2 rubric. See Gotcha 1.
- ❌ Don't assert on the `providers list` DETECTED or DEFAULT columns — they are environment-variant
  (depend on $PATH). Assert only on the NAME column (7 rows, no gemini). See Gotcha 3.
- ❌ Don't expect double-quoted TOML from `providers show agy` — go-toml v2 emits SINGLE-QUOTED literals
  (`print_flag = ''`, `model_flag = '--model'`, `bare_flags = ['--mode', 'plan']`). See Gotcha 2.
- ❌ Don't treat `go test ... (cached)` or `experimental = true` as failures. Cached = PASS; agy is
  intentionally experimental (§12.5.1.1 item 4 open). See Gotchas 4 + 7.
- ❌ Don't drop `pkg/` from the grep scope — the contract's pass (a) is
  `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/`. See Gotcha 5.
- ❌ Don't edit the compiled provider set, the agy manifest (`builtin.go builtinAgy()` / `providers/agy.toml`),
  PRD.md, tasks.json, prd_snapshot.md, or .gitignore. They are the SOURCE OF TRUTH this task verifies
  against (or are forbidden). A stale hit in code is a COMMENT fix (or a reported P2.M1 regression), not
  a logic edit.
- ❌ Don't conflate the two README files — README.md (repo root) and docs/README.md are different; the
  count-check (pass b) names BOTH. See Gotcha 8.
- ❌ Don't skip a pass or assert "it should work" — run all three passes and capture real output. The
  whole point of this task is empirical, reproducible proof.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a deterministic verification gate whose every command,
expected output, and decision is specified verbatim. On the current baseline (verified during research),
all three passes already succeed clean: every `gemini`/`Gemini` grep hit is a model name or a
lineage/history comment (full table in research/baseline_evidence.md §1); the six docs files carry no
8/eight framing and no gemini-as-built-in; `make build` succeeds; `providers list` prints exactly seven
rows (agy, claude, codex, cursor, opencode, pi, qwen-code) with no gemini; `providers show agy` prints
`print_flag = ''`, `model_flag = '--model'`, `bare_flags = ['--mode', 'plan']`, `experimental = true`;
and `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...` reports `ok` for all
three packages. The only judgment call (classifying a grep hit) is fully pinned by the rubric + the
pre-classified baseline table, and the only environment-variant output (DETECTED/DEFAULT columns) is
explicitly excluded from the assertions. The normal outcome is zero new edits (pure verification); the
conditional fix path is mechanical if a stale reference ever appears. No code, schema, or external
dependency is in play.
