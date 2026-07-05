# Research: token_limit / truncation doc sweep (P1.M2.T1.S1, bugfix 002 Mode B)

Verified against the live docs (README.md, docs/how-it-works.md, docs/configuration.md) and the
now-restored shipped behavior (P1.M1.T1.S1 line-anchored splitDiffSections + P1.M1.T2.S1 E2E coverage).
Source of truth for the Mode-B documentation sweep.

## Shipped behavior AFTER the fix (the contract the docs must match)

Under `token_limit > 0`, a non-markdown file whose content contains the literal `diff --git ` (a test
fixture, golden snapshot, .patch/.diff file, docs quoting a diff) is ONE section — sized and truncated as
a unit. If its body exceeds its water-fill allotment it is truncated to `allotment×4` runes + the
`... [truncated]` sentinel, and the **total payload fits within `token_limit`** (FR3d contract upheld).
Before the fix, such a file was fragmented into bogus tiny sections, defeated truncation, and the payload
silently overflowed. The fix RESTORES the contract; it does NOT change user-facing/config/API surface.

## The default outcome: NO EDIT NEEDED (confirmed surface-by-surface)

system_context.md §7.2 states the most likely outcome is "no edit needed" and the implementing agent
confirms. My read of each surface confirms it:

### Surface 1 — README.md ~63 ("Payload optimization" table row)
> "The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`),
> led by a compact file skeleton, and optionally capped to your model's context window via `token_limit`."

Verdict: **ACCURATE / no edit.** "Optionally capped to your model's context window via `token_limit`" is
the FR3d contract — now TRUE again (the bug broke it only for the trigger class). It does NOT reference the
section-split mechanism and does NOT imply anything now-incorrect.

### Surface 2 — docs/how-it-works.md ~138-146 ("Size budget (FR3d / FR3i)")
> "… every file *larger* than `L` is truncated to `L` (with a `... [truncated]` marker that ends the
> file's section on its own line, so the next file's `diff --git` begins fresh). Small files are never
> penalized for their size; large, substantive files receive the bulk of the budget; no single file can
> monopolize it; and nothing is wasted. The common case — a commit that fits — is left untouched."

Verdict: **ACCURATE / no edit.** Describes the water-fill CONTRACT and the OUTPUT line-shape ("the next
file's `diff --git` begins fresh") — both now TRUE again. It does NOT specify the section-splitting
MECHANISM and does NOT imply content-embedded `diff --git` literals are a problem. Adding a note about the
content-embedded case would be INTERNAL-MECHANISM LEAKAGE (the contract forbids it: "Do NOT … describe
internal implementation details (the split mechanism) in user-facing docs").

### Surface 3 — docs/configuration.md ~107 / ~131 / ~146
- ~107 comment: `# token_limit = 0 # holistic token budget (0 = unset ⇒ use the caps above); FR3d` — accurate.
- ~131 defaults table row: `| token_limit | 0 | config.Defaults() (§9.1 FR3d — unset ⇒ legacy caps) |` — accurate.
- ~146 prose: "**`token_limit`** … so the payload always fits your model's context window **without
  Stagehand maintaining a per-model context registry** (§9.1 FR3d)." — "always fits" is the INTENDED
  behavior the fix RESTORES. Now TRUE again. No edit.

Verdict: **ACCURATE / no edit.** (NOTE: the `diff_context` "range 0–3 — out-of-range rejected at config
load" notes at ~107/131/146 are from bugfix 001's P1.M2.T1.S1 Mode-A work, ALREADY PRESENT — they are NOT
this task's concern. Do not touch them.)

## Why no edit is the CORRECT outcome (not laziness)

- The docs describe the CONTRACT ("payload always fits"; "each file's section ends on its own line so the
  next file's `diff --git` begins fresh"; "every file larger than L is truncated to L"). The fix makes
  reality MATCH the docs — the docs were never made inaccurate by describing new behavior; they described
  the intended behavior all along. A latent doc-vs-reality gap existed ONLY for the trigger class, and it
  is now closed FROM THE IMPLEMENTATION SIDE, not the doc side.
- The fix is a correctness defect in an INTERNAL pure function (`splitDiffSections`). There is no new
  user-facing/config/API surface to document.
- Adding "files containing `diff --git` text in their content are handled correctly" would (a) describe an
  internal robustness/mechanism detail the contract forbids in user-facing docs, and (b) be a non-feature
  (it's just "the documented cap now actually holds"). It would NOT materially improve the user's mental
  model — the contract statement already covers it.

## markdownlint house style (.markdownlint.json)
`{"default": true, "MD013": false, "MD033": false, "MD060": false}` — line length off, inline HTML off,
pseudo-headings off; all other rules at default. IF a minimal edit is warranted (it is not expected), it
must pass `markdownlint-cli2` (or `npx markdownlint-cli2 README.md docs/*.md`). In the expected no-op case,
the docs already lint clean (they are untouched).

## Scope boundary (no conflict / no overlap)
- **P1.M1.T2.S1 (parallel)** is TEST-ONLY in `internal/git/difftokenlimit_test.go` ("Test-only — no
  production/docs change"). ZERO overlap.
- **P1.M1.T1.S1 (Complete)** already did the Mode-A godoc update on `splitDiffSections`
  (`internal/git/truncatediff.go` ~59-73) — that INTERNAL doc rode with the fix. This task is USER-FACING
  docs only (README + docs/*.md); do NOT touch internal godocs.
- This task touches ONLY: README.md (~63), docs/how-it-works.md (~138-146), docs/configuration.md
  (~107/131/146). Expected: confirm consistent, no edit. No source code, no PRD, no internal godocs.

## Edit criteria (strict — default is no-op)
Edit a surface IF AND ONLY IF a statement is GENUINELY STALE or MISLEADING about (a) token_limit /
water-fill truncation / the payload-fits-context-window contract, or (b) how the non-md aggregate becomes
per-file sections. A statement that "could mention" the content-embedded-`diff --git` case is NOT a reason
to edit — that is mechanism detail forbidden in user-facing docs. If no surface meets the bar (the expected
case), the deliverable is the VERIFICATION RECORD + an empty git diff for the three files.
