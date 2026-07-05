# Research: docs/how-it-works.md Arbiter Freeze Narrative (Mode A edit)

> **Purpose:** Pin the exact doc edits for P1.M1.T2.S2 — bringing `docs/how-it-works.md`'s arbiter
> narrative into line with the freeze-safe arbiter behavior landed by P1.M1.T2.S1 (FR-M1d/M9/M10).
> Pure documentation edit (Mode A — rides with S1). All line numbers from the live tree on 2026-07-04.

---

## 1. The behavior to document (the S1 CONTRACT — assume landed)

S1 (P1.M1.T2.S1, parallel) rewrites the arbiter to be freeze-safe on all three resolution paths. The
doc must reflect:

- **GATE:** the arbiter gate is the **frozen leftover** `DiffTreeNames(tipTree, T_start)` — NOT live
  `StatusPorcelain`. The arbiter runs iff `len(leftoverPaths) > 0 && len(commits) > 0`.
- **DIFF:** the arbiter is shown `TreeDiff(tipTree, T_start)` (already frozen in `runArbiterPhase`).
- **STAGING (resolution):** every arbiter commit's tree is built from `T_start` — paths A/B
  (`null`/new + tip-amend): `treePrime := tStart` (NO `AddAll`/`WriteTree`); path C (mid-chain):
  per-j `OverlayTreePaths(tree[j], T_start, leftoverPaths)`. NO path reads `git status` or stages
  against the live tree.
- **INDEX SYNC:** after each path's CAS, `ReadTree(T_start)` syncs the index so `git status` is clean
  for the committed set; concurrent working-tree changes remain, unstaged/untracked.
- **THE FREEZE PROPERTY:** a file written after `T_start` capture is NOT in `diff-names(tipTree, T_start)`
  → cannot trip the gate → cannot enter any arbiter commit → left untouched. This closes the v2.0–v2.1
  loophole FR-M1d names.

Authoritative sources: PRD §9.14 FR-M1d / FR-M9 / FR-M10 + §13.6.5; S1's PRP (the contract).

## 2. The current doc state (verified — docs/how-it-works.md, 284 lines)

The decompose section spans ~lines 55-128. Three spots are stale/imprecise wrt the freeze-safe arbiter:

### Spot A — STALE: the "Arbiter leftover reconciliation" paragraph (line 117)
Current verbatim:
> **Arbiter leftover reconciliation.** After all N concepts are committed, if `git status --porcelain` shows remaining changes, the arbiter decides whether they belong to an existing commit (amend) or warrant a new (N+1)th commit.

**Why stale:** gates on LIVE `git status --porcelain` — exactly the behavior S1 removes (FR-M1d).
This is the PRIMARY edit. (`git status --porcelain` appears EXACTLY ONCE in how-it-works.md — here.
Confirmed by grep: the only `porcelain` hit in the file.)

### Spot B — IMPRECISE: the diagram gate label (line 91, inside the ASCII pipeline)
Current verbatim:
```
         git status clean? ──yes──▶ done
                  │ no
                  ▼
            ┌────────────┐  commits made + leftover diff   target SHA or null
            │  arbiter   │◀───────────────────────────▶  (stagehand does all git)
```
**Why imprecise:** the "git status clean?" gate label contradicts the rewritten paragraph (the gate is
the frozen leftover, not live git status). A consistency-driven refinement (the "leftover diff" label
on the arrow is already frozen-correct — unchanged). Scope note: the contract names the paragraph +
line 111; the diagram label is in the SAME arbiter narrative and must stay consistent with the
paragraph rewrite, so refining it is in-scope editorial consistency, not scope creep.

### Spot C — CONFIRM/REFINE: the "Start-of-run freeze (T_start)" paragraph (line 111)
Current verbatim (the arbiter clause):
> ... every stager, the arbiter's leftover staging, and the one-file/single shortcuts stage content drawn strictly from T_start. ...

**Status:** already true after S1 (the arbiter's staging IS from T_start). But FR-M1d names the
arbiter as the "third freeze surface" with THREE frozen aspects — **gate, diff, and staging** — and
the current text mentions only "leftover staging." Refine to name all three so line 111 is internally
consistent with the rewritten line 117 and forwards-compatible with FR-M1d's framing.

### Not stale (verified — no edit needed)
- Line 62 (four-roles table, arbiter row): "Decide which just-made commit any leftover changes belong
  to, or create a new commit" — still accurate (the arbiter's JOB is unchanged; only the gate mechanism
  changed). Leave as-is.
- Line 236 (format modes): "the arbiter's leftover-commit message" — about format applying to the
  arbiter's message, not the gate. Leave as-is.
- Line 124/126 (Safety bullets "Frozen content" / "Start-of-run freeze"): "concurrent edits never enter
  any commit" is now MORE true (loophole closed). Accurate; leave as-is.
- Line 173 (lock no-op fast path): mentions T_start correctly. Leave as-is.

## 3. The exact edits (current → target)

### Edit 1 — line 117 (the PRIMARY rewrite)
**Current:**
> **Arbiter leftover reconciliation.** After all N concepts are committed, if `git status --porcelain` shows remaining changes, the arbiter decides whether they belong to an existing commit (amend) or warrant a new (N+1)th commit.

**Target:**
> **Arbiter leftover reconciliation.** After all N concepts are committed, stagehand computes the **frozen leftover** = `diff-names(tipTree, T_start)` — the `T_start` content no stager claimed (`tipTree` is the last committed tree) — and runs the arbiter **iff it is non-empty**. The live working tree is never consulted for the gate (not `git status --porcelain`), so a file written after `T_start` was captured cannot trigger the arbiter or enter any arbiter commit. Given `TreeDiff(tipTree, T_start)`, the arbiter decides whether the leftovers belong to an existing commit (a plumbing amend that rebuilds the chain from the frozen per-concept `tree[j]` and `T_start`) or warrant a new (N+1)th commit (committing `T_start` directly); stagehand performs all git from frozen trees, then syncs the index to `T_start`, and the arbiter only decides.

Covers (concisely, 3 sentences): gate (frozen diff-names), the freeze property (concurrent change
can't trigger/enter), the diff (TreeDiff frozen), the resolution (amend/new, tree-only from T_start),
and the index sync. Matches the existing bullets' symbol density (`tree[i]`, `T_start`,
`diff(tree[i-1], tree[i])`).

### Edit 2 — line 111 (refine the arbiter clause)
**Current clause:**
> ... every stager, the arbiter's leftover staging, and the one-file/single shortcuts stage content drawn strictly from T_start. ...

**Target clause:**
> ... every stager, the arbiter (its gate, its diff, and its leftover staging), and the one-file/single shortcuts draw strictly from T_start. ...

(Generalizes "stage content drawn strictly from T_start" → "draw strictly from T_start" so the arbiter's
gate/diff reads are covered, and names the three freeze surfaces per FR-M1d.)

### Edit 3 — line 91 (diagram gate label, consistency)
**Current:**
```
         git status clean? ──yes──▶ done
```
**Target:**
```
     frozen leftover empty? ──yes──▶ done
```
(Keeps the diagram consistent with the rewritten paragraph. The "no → arbiter" branch and the
"leftover diff" arrow label are unchanged.)

## 4. Scope discipline (do NOT do)

- Do NOT touch `docs/cli.md` or `docs/configuration.md` (contract: "No new flags/keys — do NOT touch
  docs/cli.md or docs/configuration.md").
- Do NOT edit the four-roles table (line 62), the format-modes paragraph (line 236), the Safety bullets
  (line 124/126), or the lock paragraph (line 173) — none are stale.
- Do NOT add new flags, config keys, or FR citations beyond what the existing bullets carry.
- Do NOT edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, source code, or `plan/*`.
- Do NOT describe the behavior as future/conditional ("will") — write present tense; by the time this
  Mode-A edit lands, S1 IS the behavior.
- Do NOT duplicate the PRD's §13.6.5 verbatim — how-it-works.md is the plain-English companion; keep
  the existing voice (concise bullets, moderate code-symbol use).

## 5. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | Edit only line 117, or also 111 + the diagram? | All three (117 + 111 + diagram label) | 117 is the stale paragraph (contract). 111's "leftover staging" under-names FR-M1d's three surfaces (contract: "confirm/refine"). The diagram's "git status clean?" label would CONTRADICT the rewritten paragraph — internal consistency requires refining it. All three are within the arbiter narrative. |
| D2 | Voice/tense | Present tense, plain-English + moderate symbols | Mode A rides with S1 — by landing, S1 IS the behavior. how-it-works.md uses `T_start`/`tree[i]`/`diff(...)` freely; match that density. |
| D3 | Mention OverlayTreePaths by name? | No — keep "rebuilds the chain from frozen tree[j] and T_start" | how-it-works.md is the plain-English doc; the primitive's mechanics belong in §13.6.5 / the code. Naming it would over-specify for this audience. |
| D4 | Mention ReadTree(T_start) index sync? | Yes, briefly ("syncs the index to T_start") | It's user-observable (git status clean post-run) and part of FR-M1d (3). One phrase, no mechanism. |
| D5 | Validation approach | Editorial grep + scope grep (no compile/test for docs) | Docs-only edit. Grep proves the stale phrase is gone + the new phrasing is present; `git diff --stat` proves scope. |
