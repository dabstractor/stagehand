name: "P1.M1.T5.S1 — Verify README.md and overview docs reflect the four fixes (changeset doc-sync)"
description: |
  [Mode B] This IS the documentation-sync task for the P1.M1 changeset. It sweeps README.md and the
  `docs/*.md` overview docs for drift introduced by the four bug fixes (P1.M1.T1–T4):
    - T1 (Issue 1, Major):   `integrate install lazygit` now prints a WARNING when an UNMARKED entry
                             already binds the target key (commit `0dd57a3`).
    - T2 (Issue 2, Minor):   `hook exec` no longer prints the "Generating…" line for source-gated
                             no-ops or empty diffs (commit `0385f21`).
    - T3 (Issue 3, Minor):   `config init --template` now includes the 5 v2.1 `[generation]` keys
                             (`exclude`,`format`,`locale`,`template`,`push`) (commit `73b84e0`).
    - T4 (Issue 4, Minor):   `ui.IsTerminal` uses a true isatty probe so `/dev/null` → false (PRP
                             P1M1T4S1, in progress in parallel — treat its PRP as a contract).
  These are INTERNAL-BEHAVIOR bug fixes, NOT new features. Pre-verification (research/drift_analysis.md)
  confirms **ZERO documentation drift** — every fix brought the CODE into compliance with docs that were
  already accurate. The EXPECTED outcome is therefore: make ZERO doc file changes and produce a
  verification report. The PRP nonetheless gives the implementer the exact per-fix drift probes so the
  conclusion is reached INDEPENDENTLY and is auditable, not merely asserted. Do NOT invent documentation
  changes (work item 3d). If — and only if — a probe surfaces a genuine contradiction, fix it surgically.

---

## Goal

**Feature Goal**: Confirm that README.md and all `docs/*.md` overview documents accurately describe the
behavior of the codebase AFTER the four P1.M1 fixes land, by running a deterministic, per-fix drift
probe against each doc and recording the evidence. The changeset must leave no stale claim.

**Deliverable**: A **verification report** at
`plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T5S1/VERIFICATION_REPORT.md` that, for each of the
four fixes, records: the doc file(s) + line range(s) inspected, the exact claim checked, and the verdict
(accurate / drifted-then-fixed / drifted-and-impossible). **Zero** changes to `README.md` or `docs/*.md`
unless a probe finds a genuine contradiction (none expected). No code changes anywhere.

**Success Definition**:
- Every per-fix drift probe in "Implementation Tasks" has been executed and its output recorded.
- No `README.md` / `docs/*.md` claim contradicts the post-fix behavior of T1–T4.
- `README.md` and `docs/*.md` are byte-identical to their pre-task state UNLESS a real drift was found
  and surgically corrected (expected: byte-identical).
- No source file under `cmd/` or `internal/` is touched (this is a docs-only task).
- The verification report is written and self-consistent with the actual `git diff` (empty, or surgical).

## User Persona (if applicable)

**Target User**: The maintainer / next reviewer reading README.md or `docs/` after the v2.1 validation
bugfix changeset — who must be able to trust that the prose matches the shipped binary's behavior.

**Use Case**: After the four fixes merge, a user reading `docs/cli.md` should not be misled (e.g. the
"Conflicting key behavior" paragraph must describe the WARNING that T1 actually prints; the FR-H4 no-op
guarantee must not promise a "Generating…" line that T2 removed; etc.).

**Pain Points Addressed**: Documentation/code drift — the silent killer of trust in a CLI whose README
itself directs users to the binary (`stagehand --help`) as authoritative.

## Why

- **Work item contract (3d)**: "If NO documentation drift is found (the expected outcome for a bugfix
  changeset), document this finding in the subtask completion and make zero file changes. Do NOT invent
  documentation changes." This task's value is the AUDIT, not churn.
- **PRD refs**: §9.18–§9.23 (the six v2.1 feature areas the docs surface); the four issues' PRD anchors
  (§9.21 FR-I3/I5 for T1; §9.20 FR-H4 for T2; §9.19/§9.22 for T3; §9.23 FR-L3 & §9.21 FR-I3c for T4).
- **Scope discipline**: T1–T4 are bug fixes to INTERNAL behavior. The docs were written to the intended
  (PRD) behavior, so the fixes reconcile code→docs rather than docs→code. Editing docs would risk
  introducing NEW drift against the now-correct code.

## What

No user-visible change. This task produces EVIDENCE (a verification report) and, only if a genuine
contradiction is found, a surgical doc edit. The four behavioral changes under audit:

1. **T1**: `integrate install lazygit` prints a `WARNING` to stderr when an unmarked `customCommands`
   entry already binds the target key, then proceeds through the no-mangle preview/confirm flow.
2. **T2**: `hook exec` emits the "Generating with <provider>…" progress line ONLY when generation is
   actually about to run (after the source-gate / empty-diff no-op checks); no-op paths are silent.
3. **T3**: `config init --template` writes an inert, all-commented reference config whose `[generation]`
   section includes `exclude`, `format`, `locale`, `template`, and `push` (commented).
4. **T4**: `ui.IsTerminal` returns `false` for `/dev/null`, pipes, files, and redirects (true isatty
   probe); `config init --interactive < /dev/null` now trips the FR-L3 TTY gate; `DefaultConfirm` with
   `/dev/null` stdin now takes the explicit FR-I3c auto-decline path.

### Success Criteria

- [ ] Drift probe for Issue 1 executed; README.md + docs/cli.md claims match T1 behavior.
- [ ] Drift probe for Issue 2 executed; docs/cli.md FR-H4 no-op prose consistent with T2 (silent no-op).
- [ ] Drift probe for Issue 3 executed; docs/configuration.md lists all 5 `[generation]` keys.
- [ ] Drift probe for Issue 4 executed; docs/cli.md + docs/configuration.md FR-L3 prose consistent with T4.
- [ ] Cross-cutting sweep of docs/README.md, docs/how-it-works.md, docs/providers.md executed.
- [ ] `git diff --stat README.md docs/` is EMPTY (expected) OR every change is a justified surgical fix.
- [ ] `git diff --stat cmd/ internal/` is EMPTY (no code touched).
- [ ] VERIFICATION_REPORT.md written with per-fix evidence.

## All Needed Context

### Context Completeness Check

_This PRP names the exact doc files, the exact line ranges, the exact claim each fix could invalidate,
the exact grep/read probe to verify it, and the deterministic expected output. The implementer needs
zero codebase knowledge to run the audit — every probe is copy-paste runnable. The "no drift" conclusion
is reproducible from the probes, not asserted._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/architecture/system_context.md
  why: Authoritative map of the codebase areas each fix touches (integrate/hook/config/ui layers) and the
       dep-free / TDD / no-mangle design constraints that bound what "accurate docs" means.
  critical: "Confirms these are INTERNAL-behavior fixes (not new features) → the docs-vs-code arrow is
             code→docs, so doc edits are unexpected unless a prose claim is now literally false."

- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T5S1/research/drift_analysis.md
  why: The completed per-fix drift analysis: for each of T1–T4, the exact doc claim that could drift, the
       probe, and the verdict (all: NO DRIFT). Use this as the reference answer the implementer must
       RE-DERIVE independently (don't copy it blind — run the probes and confirm).
  critical: "Headline = ZERO drift. If a probe disagrees, surface it rather than papering over it."

- file: README.md
  why: Root overview (~19KB, surfaces all six v2.1 features). Audit targets: the Features table, the
       lazygit & git alias section (L168–L189), the config-init note (L249), the snapshot-workflow ASCII
       diagram (L255–L275). README makes NO mention of 'mangle' / foreign-key / no-mangle guarantee
       (grep-confirmed) → expected drift surface = none.
  pattern: "README's lazygit section is purely the install command + manual YAML block; it makes no claim
            about silent install, foreign-key handling, or TTY detection."
  gotcha: "README L265 '↳ generating with pi…' is the MAIN stagehand snapshot diagram, NOT hook exec — do
           NOT confuse it with the T2 'Generating…' line. README L191 loadingText:'Generating commit
           message…' is the lazygit customCommand spinner, also unrelated to T2."

- file: docs/cli.md
  why: The CLI reference (462 lines). The 4 audit anchors: (1) L288–L290 'No-mangle protocol' + L328
       'Conflicting key behavior' [Issue 1]; (2) L109–L134 'hook exec' incl. L113 'Source-gated no-op
       (FR-H4)' [Issue 2]; (3) L173–L174 + L187 '--template' [Issue 3]; (4) L188 + L190 '--interactive'
       Non-TTY gate [Issue 4].
  pattern: "L328 ALREADY documents the T1 WARNING verbatim ('install prints a WARNING to stderr…').
            L113 ALREADY says 'exits 0 having done nothing' for no-ops (consistent with T2 silence).
            L188/L190 ALREADY say 'Non-TTY → exit 1' (consistent with T4)."
  gotcha: "These docs were written to the intended behavior; the fixes make the code match. Do NOT 'fix'
           docs that already say the right thing."

- file: docs/configuration.md
  why: Configuration reference. Audit targets: built-in defaults table (L125–L134) for the 5 v2.1
       `[generation]` keys [Issue 3]; the populated-config example (L99–L112) [Issue 3]; the
       '--interactive' wizard note (L52) for the FR-L3 Non-TTY gate [Issue 4].
  pattern: "L131 format, L132 locale, L133 template, L134 push all present; exclude at L104 + the
            dedicated 'Exclusion globs' section (L222+). L112 'documents every available option' is now
            SATISFIED by the T3-fixed template."
  gotcha: "The fix arrow for Issue 3 is code→doc: docs/configuration.md was already complete; T3 fixed
           the code template (internal/cmd/config.go exampleConfigTemplate) to match the doc. Do not edit
           the doc here — verify it lists all 5 keys (it does)."

- file: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T4S1/PRP.md
  why: T4 is implemented IN PARALLEL. Its PRP is the contract for what IsTerminal will do after the fix
       (true isatty ioctl; /dev/null → false; signature frozen; 4 callers auto-benefit). Read it to know
       the exact post-T4 behavior to audit the FR-L3/FR-I3c prose against.
  critical: "T4 changes ONLY internal/ui/ (output.go body + 4 isatty_*.go files). It touches NO doc and
             NO caller. The docs describe the EXTERNAL behavior (TTY gate / auto-decline), which T4 makes
             correct for /dev/null — so the docs need no change."

- file: docs/README.md
  why: Documentation index (the 'overview doc'). v2.1 per-page additions table (L33–L36). Sweep for any
       claim that one of the four fixes invalidated (expected: none — these are internal-behavior fixes).
  pattern: "Each row's 'v2.1 additions' note lists FEATURES (hook/integrate/models/shaping), none of which
            the four bug fixes add or remove."
```

### Current Codebase tree (relevant slice)

```bash
# Documentation surface under audit (READ-ONLY unless a genuine drift is found):
README.md                          # root overview (~19KB, all six v2.1 features)
docs/README.md                     # documentation index + capability index
docs/cli.md                        # CLI reference (hook exec L109, integrate L214–L342, --interactive L188)
docs/configuration.md              # config reference (defaults table L125, --interactive L52, exclusions L222)
docs/how-it-works.md               # architecture/pipeline prose (hook-vs-snapshot FR-H7)
docs/providers.md                  # manifest schema + 7 built-ins (unrelated to the four fixes)

# The four fixes (already in tree / in progress — for CONTEXT only, do NOT edit code here):
internal/cmd/integrate_lazygit.go          # T1: foreign-key WARNING in Install()
internal/cmd/hookexec.go + internal/hook/  # T2: progress emission deferred past no-op gates
internal/cmd/config.go (exampleConfigTemplate) # T3: +5 commented [generation] keys
internal/ui/output.go + internal/ui/isatty_*.go  # T4: true isatty probe (parallel PRP)

# Validation tooling:
Makefile                            # build, test, lint, coverage-gate targets
.markdownlint.json                  # MD013/MD033/MD060 disabled; default:true
```

### Desired Codebase tree with files to be added/changed

```bash
# EXPECTED outcome (no drift) — ZERO doc changes; one new work-item artifact:
plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T5S1/VERIFICATION_REPORT.md   # NEW (the deliverable)
# README.md, docs/*.md                            # UNCHANGED (byte-identical)
# cmd/, internal/                                 # UNCHANGED (docs-only task)

# ONLY IF a probe finds a genuine contradiction (unexpected): surgical edit to the single offending
# doc file + the report records the fix. Do NOT broaden into a docs refactor.
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (task nature): this is a VERIFICATION + catch-all task, NOT an implementation task. The
# expected result is "docs already accurate, zero changes" (work item 3d). Treat any impulse to "improve"
# or "clarify" docs as scope creep UNLESS a probe proves a claim is now literally false.

# CRITICAL (fix arrow): all four fixes point code→doc, not doc→code. The docs were written to the PRD's
# intended behavior; the fixes make the code comply. So the docs were ALREADY correct (or aspirationally
# correct) and need no edit. Editing a doc that already states the post-fix behavior would CREATE drift.

# CRITICAL (parallel T4): T4 (P1M1T4S1) is implemented in parallel and may not be committed when this task
# starts. Audit the Issue-4 doc claims against T4's PRP (the contract), not against a not-yet-built binary.
# The relevant docs (FR-L3 Non-TTY gate, FR-I3c auto-decline) describe EXTERNAL behavior unaffected by
# whether T4 is merged, so the audit conclusion does not depend on T4 timing.

# GOTCHA (README 'generating' red herrings): README L265 '↳ generating with pi…' is the MAIN stagehand
# snapshot diagram; README L191 loadingText 'Generating commit message…' is the lazygit customCommand
# spinner. NEITHER is the T2 'hook exec' progress line. Do not flag them as T2 drift.

# GOTCHA (Issue 3 fix arrow): docs/configuration.md L112 ('documents every available option') is a claim
# about the TEMPLATE. The doc itself (docs/configuration.md) already lists all 5 keys. T3 fixed the CODE
# template to satisfy the doc. So for Issue 3 you VERIFY the doc lists the keys (it does) — you do NOT
# edit the doc, and you do NOT need to re-verify the code template (that was T3's job, already committed).

# GOTCHA (markdownlint): the repo has .markdownlint.json (MD013/MD033/MD060 off; default on). If — and
# only if — you edit a doc, validate it. There is no `make` target for markdownlint; run it via npx.
# README.md and docs/* currently contain raw HTML (<details>, <table>, <img>) — MD033 is disabled for them.

# GOTCHA (no new features): the four fixes add NO user-facing capability. docs/README.md's per-page
# 'v2.1 additions' notes (L33–L36) list FEATURES; none are added/removed by this changeset.
```

## Implementation Blueprint

### Data models and structure

None — no code, no structs, no config. The only "data" is the per-fix drift-check evidence captured in
the verification report (a Markdown table: fix → doc file:line → claim → verdict).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ISSUE 1 (lazygit foreign-key WARNING) drift probe — README.md + docs/cli.md
  - READ docs/cli.md L288–L290 (No-mangle protocol) and L326–L328 (Conflicting key behavior).
  - ASSERT: L328 describes install printing a 'WARNING' on an unmarked same-key entry, proceeding through
    the no-mangle preview/confirm flow, outcome 'Updated', with the note that lazygit appends (cannot
    overwrite a sequence key). This MUST match T1's behavior.
  - PROBE: grep -niE "WARNING|conflicting|unmarked|duplicate" docs/cli.md  → confirm L328 present.
  - PROBE README: grep -niE "mangle|silent|foreign|<c-a>" README.md → confirm README makes NO claim about
    silent/no-mangle/foreign-key install (expected: only the benign 'default key <c-a>' + manual YAML).
  - RECORD in the report: file, line range, the claim, verdict (expected: ACCURATE — no edit).

Task 2: ISSUE 2 (hook exec progress noise) drift probe — docs/cli.md
  - READ docs/cli.md L109–L134 (### hook exec), esp. L113 'Source-gated no-op (FR-H4)'.
  - ASSERT: the no-op guarantee says hook exec 'exits 0 having done nothing' for message/template/merge/
    squash/commit sources and empty diffs. It makes NO promise of a 'Generating…' line. After T2 this is
    literally true (no noise).
  - PROBE: grep -niE "Generating|no-op|FR-H4" docs/cli.md → confirm no doc claims the progress line prints
    on no-op sources (expected: the only 'Generating' hit is L301 loadingText for lazygit — unrelated).
  - RECORD verdict (expected: ACCURATE — no edit).

Task 3: ISSUE 3 (config template v2.1 keys) drift probe — docs/configuration.md
  - READ docs/configuration.md L99–L112 (populated + inert template notes) and L125–L134 (defaults table).
  - ASSERT: the defaults table lists format (L131), locale (L132), template (L133), push (L134); exclude is
    in the populated-config example (L104) + the 'Exclusion globs' section (L222+).
  - PROBE: grep -nE "^\| (format|locale|template|push|exclude) " docs/configuration.md → 5 hits expected.
  - PROBE: grep -niE "documents every available option|inert" docs/configuration.md → confirm L112 claim;
    note this is now SATISFIED by the T3-fixed code template (verify in code is T3's job, already done).
  - RECORD verdict (expected: ACCURATE — no doc edit; T3 fixed code→doc, not doc→code).

Task 4: ISSUE 4 (IsTerminal /dev/null) drift probe — docs/cli.md + docs/configuration.md
  - READ docs/cli.md L188 (--interactive flag row) and L190 (wizard paragraph); docs/configuration.md L52.
  - ASSERT: both say 'Non-TTY → exit 1' / 'Non-TTY stdin exits 1 pointing at plain config init' (FR-L3).
    After T4, /dev/null correctly trips this gate — the doc was always the intended behavior.
  - PROBE: grep -niE "Non-TTY|isatty|/dev/null|char device|terminal on stdin" README.md docs/ → confirm no
    doc claims /dev/null is a terminal or bypasses the gate, and no doc mentions isatty internals
    (expected: 0 hits for isatty/dev-null/char-device; the Non-TTY hits are generic FR-L3 prose).
  - REFERENCE T4's PRP (P1M1T4S1/PRP.md) to confirm post-T4 external behavior = what the docs already say.
  - RECORD verdict (expected: ACCURATE — no edit).

Task 5: CROSS-CUTTING sweep — docs/README.md, docs/how-it-works.md, docs/providers.md
  - READ docs/README.md fully (documentation index + capability index). ASSERT its per-page 'v2.1
    additions' notes (L33–L36) list features none of the four fixes add/remove.
  - SCAN docs/how-it-works.md for any lazygit/hook-exec/config-template/TTY claim (expected: the FR-H7
    hook-vs-snapshot trade-off section is unaffected; no isatty/dev-null mention).
  - SCAN docs/providers.md (expected: unrelated; the 'agy non-TTY stdout drop' note L77 is a PROVIDER
    bug, NOT Issue 4's IsTerminal — do not conflate them).
  - RECORD a one-line verdict per file (expected: no drift).

Task 6: WRITE the verification report (the deliverable)
  - FILE: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T5S1/VERIFICATION_REPORT.md
  - STRUCTURE: a table with one row per fix (T1–T4) + a cross-cutting section: columns = Fix | Doc file:line
    | Claim audited | Probe (grep/read) | Verdict | Action taken.
  - CONCLUDE: state the headline result (expected: ZERO drift; README.md + docs/*.md byte-identical; zero
    code touched). Mirror research/drift_analysis.md but DERIVED from the probes the implementer ran.
  - IF (unexpected) a genuine contradiction was found in Tasks 1–5: perform the MINIMAL surgical edit to
    that one doc (see Task 7), then record the before/after in the report.

Task 7 (CONDITIONAL — only if Task 1–5 found a genuine drift): SURGICAL doc edit
  - Edit ONLY the single offending doc file; change ONLY the false sentence; preserve surrounding prose,
    anchors, and formatting. Do NOT refactor, reword for style, or 'improve' unrelated text.
  - RE-RUN the offending probe to confirm the claim now matches post-fix behavior.
  - VALIDATE the edited doc with markdownlint (see Validation Loop Level 1).
  - If you cannot make a truthful minimal edit (e.g. the claim is structural), STOP and report it in the
    verification report rather than forcing a change — do NOT invent documentation.
```

### Implementation Patterns & Key Details

```bash
# === Master drift probe — run once to enumerate every potentially-relevant claim ===
# Run from the repo root (/home/dustin/projects/stagehand-competitor-feature-parity).
grep -rniE "mangle|silent|foreign|duplicate|<c-a>|WARNING" README.md docs/      # Issue 1 surface
grep -rniE "Generating|no-op|FR-H4|having done nothing"        README.md docs/   # Issue 2 surface
grep -rniE "exclude|format|locale|template|push|every available option" docs/configuration.md  # Issue 3
grep -rniE "Non-TTY|isatty|/dev/null|char device|terminal on stdin|interactive" README.md docs/ # Issue 4

# === Expected: every hit is either (a) already-correct post-fix prose, or (b) a benign/unrelated ===
# === mention documented in research/drift_analysis.md. NO hit should be a now-false claim.       ===

# === The "did I touch anything?" guardrails (run at the end) ===
git diff --stat README.md docs/      # EXPECTED: empty. If non-empty, every line is a justified Task-7 fix.
git diff --stat cmd/ internal/       # EXPECTED: empty (this task touches NO source code).
```

### Integration Points

```yaml
SOURCE CODE: NONE. This task edits ZERO files under cmd/ or internal/. T1–T4 own the code; this task owns
  the docs audit. If a probe seems to require a code change, STOP — that belongs to T1–T4, not T5.

DOCUMENTATION: README.md and docs/*.md are READ by default. They are MODIFIED only if Task 1–5 finds a
  genuine now-false claim (expected: never). Any edit must be minimal/surgical (Task 7) and pass markdownlint.

VALIDATION: the project's CI (README badge → .github/workflows/ci.yml) runs build/test/vet. A docs-only
  change cannot break those, but run `make build` + `make test` as a sanity guard that no source was
  accidentally touched. markdownlint is NOT a make target — invoke via npx (see Level 1).

PLAN ARTIFACTS: the verification report lives UNDER plan/ (a work-item artifact), NOT under docs/. It is
  not shipped documentation; it is the auditable record that the changeset introduced no doc drift.
```

## Validation Loop

### Level 1: Documentation Lint (only if a doc was edited)

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity

# If (and only if) Task 7 edited a doc, lint the touched file(s). The repo ships .markdownlint.json
# (default:true; MD013 line-length OFF, MD033 inline-HTML OFF, MD060 OFF — README/docs use raw HTML).
npx --yes markdownlint-cli2 "README.md" "docs/**/*.md" 2>/dev/null || \
  echo "markdownlint-cli2 not cached; falling back to per-file:"
# Per-file fallback (substitute the actually-edited file):
npx --yes markdownlint-cli2 docs/configuration.md
# Expected: zero errors. If MD013/MD033/MD060 fire, they are DISABLED in .markdownlint.json — re-check you
# ran from the repo root so the config is picked up. Fix real violations; do NOT weaken the config.

# If NO doc was edited (the expected case), SKIP this level entirely.
```

### Level 2: Drift Probes (the core audit — deterministic)

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity

# Issue 1 — lazygit foreign-key WARNING (expected: docs/cli.md L328 already documents it).
grep -nE "WARNING to stderr|unmarked entry already binds|duplicate .*customCommands" docs/cli.md
# Expected: ≥1 hit at ~L328. README.md must have ZERO 'mangle'/'silent install'/'foreign-key' claims:
grep -ciE "mangle|silent" README.md   # expected: 0 (the 'silently falling back' at L251 is about --config discovery, unrelated)

# Issue 2 — hook exec no-op silence (expected: FR-H4 prose unchanged; no 'Generating on no-op' claim).
grep -nE "having done nothing|Source-gated no-op" docs/cli.md          # expected: ~L113
grep -nE "Generating with" README.md docs/cli.md docs/how-it-works.md  # expected: hook-exec hit = NONE (T2 removed it)

# Issue 3 — all 5 [generation] keys documented in docs/configuration.md (expected: 5 hits).
grep -cE "^\| (format|locale|template|push|exclude) " docs/configuration.md  # expected: 5

# Issue 4 — FR-L3 Non-TTY gate prose, no isatty/dev-null internals (expected: generic Non-TTY prose only).
grep -nE "Non-TTY stdin exits 1|Non-TTY . exit 1" docs/cli.md docs/configuration.md  # expected: hits at cli L188/L190, config L52
grep -ciE "isatty|/dev/null|char device" README.md docs/   # expected: 0
```

### Level 3: Sanity Build & Test (guard against accidental code edits)

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity

# A docs-only task MUST NOT change build/test results. Run as a guardrail.
make build          # ./bin/stagehand produced; zero errors
make test           # full suite green with -race (T1–T4 tests included)
make lint           # golangci-lint clean
# Expected: all pass and IDENTICAL to a pre-task run (no source changed). If anything newly fails,
# you accidentally edited a .go file — revert it (this task touches ONLY docs + the plan/ report).

# Confirm scope:
git diff --stat README.md docs/ cmd/ internal/   # expected: README.md + docs/ empty; cmd/ + internal/ empty
```

### Level 4: Report Consistency (the deliverable self-check)

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity

# The verification report's claims must MATCH the actual git state.
test -f plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T5S1/VERIFICATION_REPORT.md && echo "report exists"
git diff --stat README.md docs/   # if the report says 'zero changes', this MUST be empty
# Read the report end-to-end: every 'Verdict' cell must be backed by a probe output recorded alongside it.
# Expected: all four fixes = ACCURATE/NO-DRIFT; cross-cutting = no drift; headline = zero doc changes.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 markdownlint passes on any EDITED doc (skipped if no edit — the expected case).
- [ ] Level 2 drift probes all return their expected outputs (recorded in the report).
- [ ] `make build`, `make test`, `make lint` all pass and are unchanged from pre-task (no source touched).
- [ ] `git diff --stat cmd/ internal/` is EMPTY (zero code changes — docs-only task).

### Feature Validation
- [ ] Issue 1 probe: docs/cli.md L328 documents the T1 WARNING; README makes no contradicting claim.
- [ ] Issue 2 probe: docs/cli.md L113 FR-H4 no-op prose consistent with T2 silence; no 'Generating on no-op' claim.
- [ ] Issue 3 probe: docs/configuration.md lists all 5 `[generation]` keys (format/locale/template/push/exclude).
- [ ] Issue 4 probe: docs/cli.md L188/L190 + docs/configuration.md L52 FR-L3 prose consistent with T4.
- [ ] Cross-cutting: docs/README.md, docs/how-it-works.md, docs/providers.md swept — no drift.

### Code Quality & Scope Validation
- [ ] `git diff --stat README.md docs/` is EMPTY (expected) OR every diff line is a justified surgical Task-7 fix.
- [ ] No doc edit was made for style/clarity — only to correct a now-false claim (none expected).
- [ ] The verification report is written at the exact path and records per-fix evidence + the headline result.
- [ ] No `plan/005_*/**/tasks.json`, no `PRD.md`, no `.gitignore` was modified (FORBIDDEN files untouched).

### Documentation & Deployment
- [ ] The report is self-documenting: each row names the file:line, the claim, the probe, and the verdict.
- [ ] The report's headline (zero drift / zero changes) matches the actual `git diff --stat` output.

---

## Anti-Patterns to Avoid
- ❌ Don't invent documentation changes — work item 3d explicitly forbids it when no drift exists (the expected case).
- ❌ Don't "improve" or reword docs for clarity/style — this is an audit, not a docs refresh. Edit ONLY a now-false claim.
- ❌ Don't edit the code arrow the wrong way: for Issue 3, docs/configuration.md was already complete; T3 fixed the CODE template — do NOT edit the doc.
- ❌ Don't confuse README's snapshot-diagram '↳ generating with pi…' (L265) or lazygit's loadingText (L191) with the T2 hook-exec progress line.
- ❌ Don't conflate docs/providers.md's 'agy non-TTY stdout drop' note (L77, a provider bug) with Issue 4's IsTerminal fix.
- ❌ Don't touch any `.go` file, `cmd/`, or `internal/` — this is a docs-only task; code belongs to T1–T4.
- ❌ Don't modify PRD.md, any tasks.json, any prd_snapshot.md, or .gitignore (FORBIDDEN).
- ❌ Don't skip recording probe OUTPUT in the report — a verdict without evidence is an assertion, not an audit.
- ❌ Don't depend on T4 being merged — audit Issue 4 against T4's PRP (the contract); the FR-L3 prose is timing-independent.

---

## Confidence Score

**9/10** for one-pass success. The task is a deterministic audit whose conclusion (zero drift) is
already established by direct inspection (research/drift_analysis.md) and is RE-DERIVABLE from the
copy-paste probes in Level 2. The remaining 1 point of risk is purely procedural: the implementer must
resist the urge to "helpfully" edit docs that already say the right thing (mitigated by the explicit
work-item-3d prohibition, the fix-arrow gotchas, and the `git diff --stat` guardrails). No code changes,
no dependencies, no build risk — the only deliverable is a Markdown report under plan/.
