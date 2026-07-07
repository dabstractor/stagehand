# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a Mode-B docs sync. The v2.4 plumbing path now runs the repo's commit hooks
> (§9.25, FR-V1–V8), so the "Hook mode vs the snapshot-based flow" comparison in `docs/how-it-works.md`
> (which still says the snapshot flow *Bypasses pre-commit hooks*) and the README FAQ ("pre-commit hooks do
> not run … bypassed") are FALSE. Rewrite both surfaces so no stale "bypasses" framing remains.

## 0. Scope: TWO files, Mode B, anchor on CONTENT (the task's line numbers are stale)

Edit ONLY `docs/how-it-works.md` and `README.md`. No code, no tests, no other docs. **The task's cited line
numbers (312/316/327-329) are STALE** — the M3.T2.S1 subsection ("## Commit hooks on the plumbing path")
was added ABOVE the comparison section, pushing everything down. The ACTUAL current anchors (verified by
grep + read) are:

- how-it-works.md **L337** — the stale "**Bypasses pre-commit hooks**" bullet (Snapshot-based flow).
- how-it-works.md **L344** — the misleading "**Pre-commit hooks honored**" bullet (Hook mode block).
- how-it-works.md **L353-357** — the "### When to use which" bullets.
- how-it-works.md **L325-326** — the M3.T2.S1 subsection's trailing parenthetical ("…is being reconciled in
  the v2.4 docs rewrite…"), which becomes STALE once this rewrite lands (§3).
- README.md **L368-372** — the "### Does it run my pre-commit hooks?" FAQ (the stale answer).
- README.md **L70** — the "Git hook mode" Features row (cross-links `#trade-off-inversion-fr-h7`).

**Anchor every edit on the unique surrounding CONTENT** (the bullet/sentence text), not line numbers.

## 1. The reframing logic (what's now TRUE)

Pre-§9.25 the comparison was an inversion: **snapshot flow = atomic but bypasses hooks; hook mode = hooks
but no atomicity**. Post-§9.25 (the feature this changeset ships): **snapshot flow = atomic AND honors
hooks** (scoped to the frozen snapshot, so stage-while-generating holds; `--no-verify` skips pre-commit +
commit-msg). Hook mode is NO LONGER "the way to get hooks" — its remaining purpose is to cover **plain
`git commit` from an IDE/tool** when the user does NOT invoke `stagecoach` (hooks honored there too via real
`git commit`, but no snapshot/atomicity/stage-while-generating; generation latency inside the commit). The
two modes COMPOSE: §9.25 covers `stagecoach` commits; hook mode covers `git commit` commits. This matches
PRD §9.25 FR-V8d + the updated §9.20/§9.20-FR-H7 (in your context as `h3.41`/`h3.36`).

## 2. The VERIFY grep — distinguish REAL stale claims from CORRECT unrelated "bypass" hits (critical)

The task's VERIFY step (`grep -rniE "bypass|pre-commit hooks do NOT run|do not run on|hooks.*not.*run"
docs/ README.md`) surfaces BOTH the real stale claims AND ~6 UNRELATED correct usages. The implementer MUST
fix only the former and leave the latter:

**STALE — must fix (this task):**
- how-it-works.md:337 — "Bypasses pre-commit hooks … do NOT run on the generated commit."
- how-it-works.md:344 — "Pre-commit hooks honored: the commit flows through the standard `git commit`
  path" (misleading — implies hook mode is the ONLY way to get hooks). → reframe (§1).
- how-it-works.md:355 — "When to use which … pre-commit hooks run" (reframe; §1).
- README.md:368 — "pre-commit hooks do not run … bypassed" (rewrite the FAQ; §5).

**CORRECT / UNRELATED — do NOT touch (these are accurate, not stale):**
- docs/cli.md:34 — "`--single` … Bypass decomposition" (decompose bypass — correct).
- docs/cli.md:42 — "`--edit` … bypasses the duplicate check" (git parity — correct).
- docs/cli.md:44 — "`--no-verify` … Bypass `pre-commit` and `commit-msg` hooks" (THIS IS the bypass flag,
  accurately described — correct; added by P1.M1.T2.S1).
- docs/configuration.md:155 — "`no_verify` … the `--no-verify` bypass" (correct; added by P1.M1.T1.S2).
- docs/configuration.md:234 — "`--single` … Bypass decompose" (correct).
- docs/how-it-works.md:115 — "the planner is bypassed entirely" (one-file short-circuit — correct).

So "every stale claim is updated" means the 4 STALE entries above; the 6 CORRECT entries stay byte-identical.

## 3. The M3.T2.S1 subsection's "is being reconciled" parenthetical becomes stale — update it

The new "## Commit hooks on the plumbing path" subsection (how-it-works.md:303, added by M3.T2.S1) ENDS
(L325-326): *"See PRD §9.25 (FR-V1–V8) for the full specification. (The 'Hook mode vs the snapshot-based
flow' framing below is being reconciled in the v2.4 docs rewrite — hook mode remains the bridge for plain
`git commit` from IDEs, and the two modes now compose.)"* Once THIS task rewrites the comparison section,
"is being reconciled in the v2.4 docs rewrite" is self-referential/stale (the reconciliation is done).
Replace that sentence with a clean forward cross-link: *"See PRD §9.25 (FR-V1–V8) for the full
specification, and [Hook mode vs the snapshot-based flow](#hook-mode-vs-the-snapshot-based-flow) below for
how the two modes compose."* (drops the "is being reconciled" clause; keeps the compose point).

## 4. Preserve the "### Trade-off inversion (FR-H7)" header (don't break the README cross-link)

README.md:70 (the "Git hook mode" Features row) cross-links `#trade-off-inversion-fr-h7`. That anchor is
the slug of the "### Trade-off inversion (FR-H7)" header at how-it-works.md:330. REFRAME the section's
CONTENT (the bullets) but KEEP that header verbatim — renaming it would orphan the README link. ("Trade-off
inversion" is still apt post-§9.25: hook mode inverts the *atomicity/snapshot* trade-off vs the snapshot
flow; hooks are no longer the differentiator.) If the implementer feels the header MUST change, they must
also update README.md:70's link — but default is: keep the header, reframe the content.

## 5. README: rewrite the FAQ (the stale claim) + add a Features-table row (the feature mention)

The task's README ask is twofold: (a) the VERIFY grep catches the stale FAQ at L368 ("pre-commit hooks do
not run … bypassed") — that MUST be rewritten; (b) "add a one-line feature mention in the feature/safety
surface area." Cleanest delivery:

- **Rewrite the FAQ** (### Does it run my pre-commit hooks?, L368): flip "do not run / bypassed" → "Yes —
  as of v2.4 the default `stagecoach` command runs your repo's standard commit hooks (pre-commit →
  prepare-commit-msg → commit-msg → post-commit) around every commit, scoped to the frozen snapshot
  (atomicity + stage-while-generating preserved); `--no-verify` skips pre-commit + commit-msg (mirrors
  git). Hook mode remains for plain `git commit` from an IDE — the two compose." Cross-link
  `#commit-hooks-on-the-plumbing-path`.
- **Add ONE Features-table row** after the "Git hook mode" row (L70): "| Commit hooks on every
  `stagecoach` commit | As of v2.4 your repo's pre-commit → prepare-commit-msg → commit-msg → post-commit
  hooks run around every `stagecoach` commit, scoped to the frozen snapshot (atomic + stage-while-
  generating preserved); `--no-verify` mirrors git ([how it works](docs/how-it-works.md#commit-hooks-on-
  the-plumbing-path)). |" — one row, cross-links the M3.T2.S1 subsection. (Keeps the existing "Git hook
  mode" row — they're complementary: hook mode = `git commit`; this row = `stagecoach`.)

The README hero line (L4) and the v2.1 version-summary (L6) already cover atomicity/decompose; no edit
needed there (the FAQ + Features row carry the hooks-on-plumbing-path message).

## 6. No conflict with the parallel P1.M3.T3.S1 (or any sibling)

The running P1.M3.T3.S1 wires the hooks runner into the decompose path — CODE in `internal/decompose` +
`internal/hooks`. It does NOT touch docs/how-it-works.md or README.md. No overlap. ✓ This task touches
ONLY those two docs files. The feature it documents is fully implemented by M1–M3 (M3.T3.S1 is the last
code piece, running in parallel); the docs are accurate regardless of decompose-path wiring completion
(the behavior is identical on every path per FR-V8c).

## Sources
- `docs/how-it-works.md` L303-357 — the M3.T2.S1 subsection + the comparison section to rewrite (read in full).
- `README.md` L59-77 (Features table) + L355-374 (the FAQ) — read in full; exact anchors.
- `plan/010…/architecture/codebase_reality.md §8` — "the docs that contradict the feature (Mode B
  headline)" — confirms how-it-works.md is the headline rewrite + the "When to use which" reframe.
- PRD §9.25 (FR-V1–V8) + the updated §9.20/FR-H7 (in context as `h3.41`/`h3.36`) — the authoritative
  behavior: hooks run on the plumbing path scoped to the snapshot; `--no-verify` skips pre-commit +
  commit-msg; hook mode = bridge for `git commit` from IDEs; the two compose.
- The grep (§2) — the stale-claim audit across docs/ + README.md.
- `plan/010…/P1M3T3S1/PRP.md` (parallel) — confirms it's code-only (decompose/hooks), no docs overlap.
