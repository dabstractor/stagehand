---
name: "P1.M1.T3.S1 — README.md race-free safety property + stale-claim sweep"
description: |
  Mode B changeset-level documentation task for the plan/006 "Per-repo run lock (FR52)" feature
  (PRD §18.5). Three legs: (3a) the ONE real edit — surface the new race-free / safe-to-double-invoke
  safety property in README.md's safety section (the FAQ "Will it corrupt my repo?" answer,
  README:326–328), appending a short paragraph that names the per-repo run lock, the two contention
  outcomes (exit 0 "nothing to do" / exit 5 Busy), and the per-host caveat (the CAS, not the lock,
  covers cross-host shared-FS); (3b) a stale-claim sweep over README.md + docs/ for language implying
  two stagehand processes can safely race without a lock — empirically a NO-OP (zero grep matches at
  HEAD; the existing "can never corrupt your repo" hero is CAS-defended and accurate, NOT stale); (3c)
  a catch-all verification that docs/cli.md has the Busy=5 row and docs/how-it-works.md has the
  Per-repo run lock subsection — also a NO-OP at HEAD (cli.md:374/379 from P1.M1.T2.S1;
  how-it-works.md:144–157 from P1.M1.T1.S1; configuration.md:233–239 lock-location section — all
  present and correct). So this task makes EXACTLY ONE edit (the README append) and verifies the rest.
  The PRP's critical job is scope discipline: do NOT edit the sibling-owned docs (cli.md /
  how-it-works.md / configuration.md) — verify them only; do NOT rewrite the accurate CAS/atomicity
  sentences; do NOT invent a stale claim to fix. The new README paragraph must stay consistent with
  the landed docs (exit codes 0/5, "nothing to do …", "never-clobber-HEAD") and must NOT overclaim
  shared-filesystem safety (the per-host caveat is mandatory). Doc-only: no .go files, no PRD.md, no
  tasks.json, no prd_snapshot.md, no .gitignore. Parallel-safe with P1.M1.T2.S3 (e2e test file under
  a build tag — touches no docs).
---

## Goal

**Feature Goal**: Make the README's safety story coherent with the FR52 run-lock feature by surfacing
the new race-free / safe-to-double-invoke property, and verify the rest of the docs/ tree carries no
stale concurrency claims and has the lock's exit-code + mechanism docs present — so a user reading
ANY surface (README, cli.md, how-it-works.md, configuration.md) gets a consistent, non-contradictory
picture: the lock prevents the common local double-run; the CAS is the never-clobber-HEAD guarantee
that holds even where the lock can't (cross-host / shared FS).

**Deliverable** (one edit + two verifications; the verifications are no-ops at HEAD):
1. **README.md** — append ONE short paragraph to the FAQ "Will it corrupt my repo?" answer
   (README:326–328): the "Safe to run twice" race-free property (per-repo run lock; exit 0
   "nothing to do" if nothing new staged; exit 5 Busy otherwise; per-host caveat pointing at the CAS).
2. **Stale-claim sweep (3b)** — run the grep; confirm ZERO matches (no stale "no lock"/"safe without"/
   "CAS is the only defense" wording exists). No edit.
3. **Catch-all (3c)** — confirm docs/cli.md has the Busy=5 row (L374) + contention prose (L379) and
   docs/how-it-works.md has the "Per-repo run lock (FR52)" subsection (L144–157). Both present at
   HEAD. No edit.

**Success Definition**: README's FAQ safety answer names the race-free property with the correct
exit codes and the per-host caveat; the grep sweep returns no stale concurrency claim; the two
sibling docs are confirmed present and correct; `git diff` shows ONLY README.md changed (a single
appended paragraph); no sibling doc, no source, no PRD/tasks/snapshot/gitignore touched.

## User Persona

**Target User**: A Stagehand user reading the README to decide whether the tool is safe —
specifically the power user / scripter who runs `stagehand` from two terminals, a shell loop, or a
lazygit keybind mashed twice, and wants to know "what happens if I double-invoke?" before trusting it
in automation.

**Use Case**: The user accidentally hits the stagehand keybind twice (or a shell loop fires two runs
before the first finishes). They want the README to tell them, up front, that this is safe: the
second run either no-ops (nothing new staged) or refuses with a clear "busy, retry" — it never races
on HEAD or produces a duplicate/corrupt commit. They also want to know the limit: on a shared
filesystem across hosts, the lock can't help, but the atomic CAS still guarantees no corruption.

**User Journey**: README → FAQ → "Will it corrupt my repo?" → reads the atomicity sentence (failed
generation = byte-for-byte unchanged) AND the new "Safe to run twice" sentence (lock prevents the
HEAD race; graceful exit 0/5; per-host caveat) → trusts the tool for double-invoke scenarios → (if
they want depth) follows the docs/ links to how-it-works.md's FR52 subsection and cli.md's exit-code
table, which say the same thing consistently.

**Pain Points Addressed**: (1) The README previously didn't mention concurrency at all, so a careful
user had no way to know double-invoke was safe — they'd either avoid the tool in automation or worry
about races. (2) Prevents the opposite failure: an overclaim that the lock covers shared filesystems
(the per-host caveat keeps the README honest, matching the CAS-defended corruption guarantee).

## Why

- **PRD §18.5 (FR52) added a per-repo run lock as the FIRST line of defense** against two concurrent
  commit-producing runs racing on HEAD; the §13.5 CAS is the SECOND (the never-clobber-HEAD guarantee
  that holds even cross-host). The README's safety section (the FAQ "Will it corrupt my repo?") must
  surface this new property so users learn the double-invoke-safe behavior from the marketing surface,
  not just from the deep docs.
- **system_context §5 (defense in depth) is the design rationale.** The new property COMPOSES with the
  existing atomicity/never-corrupt pitch (they are two distinct guarantees: atomicity = no corruption
  on failure; the lock = no HEAD race on double-invoke). Both belong in the safety section. The README
  currently has only the first; this task adds the second.
- **Contract 3a's substance is fixed:** safe-to-invoke-twice + per-repo lock + exit 0/5 outcomes +
  the explicit per-host caveat ("don't overclaim shared-filesystem safety — the CAS covers that, not
  the lock"). The caveat is mandatory because the README hero already says "can never corrupt your
  repo" (CAS-defended, accurate); the lock must NOT be pitched as the source of that cross-host
  guarantee.
- **Closes P1.M1.T3 (the doc-sync milestone).** The implementing subtasks (T1.S1 lock primitive,
  T2.S1 Busy code, T2.S2 wiring) landed their own Mode-A docs (how-it-works subsection, cli.md row,
  configuration.md location). This task is the Mode-B catch-all: surface the property on the README
  and confirm no doc drifted.

## What

**One additive edit to README.md** (the FAQ safety answer) plus two no-op verifications. The README
paragraph (drafted verbatim in §Implementation Blueprint) states: a per-repo run lock prevents two
concurrent commit-producing runs from racing on HEAD; an accidental double-invoke exits `0` ("nothing
to do") if nothing new is staged or `5` (Busy) if genuinely new work is staged (left staged for a
re-run); and on a shared filesystem across hosts the lock can't help — the atomic `update-ref` CAS is
the never-clobber-HEAD guarantee there.

The stale-claim sweep greps README.md + docs/ for language implying two stagehand processes can
safely race without a lock ("no lock", "safe without", "safely race", "CAS is the only/sole defense",
etc.). At HEAD this returns ZERO matches — the README simply doesn't discuss concurrency today, and
how-it-works.md already carries the correct two-stage (lock + CAS) defense. No edit.

The catch-all confirms docs/cli.md's Busy=5 row (L374) + contention prose (L379) and
docs/how-it-works.md's "Per-repo run lock (FR52)" subsection (L144–157) are present. Both are — they
landed with their implementing subtasks. No edit (sibling-owned).

### Success Criteria

- [ ] README.md FAQ "Will it corrupt my repo?" answer has the appended "Safe to run twice" paragraph
      naming: the per-repo run lock; exit `0` (nothing to do) / exit `5` (Busy); the per-host caveat
      (CAS, not the lock, covers cross-host shared FS).
- [ ] The new README paragraph is consistent with the landed docs: exit codes `0`/`5`; "nothing to do"
      fragment; "never-clobber-HEAD" framing; per-host limit (does NOT promise shared-FS lock coverage).
- [ ] `grep -rniE 'no lock|safe without|safely race|without a lock|cas is the (only|sole)|only.*defense' README.md docs/`
      returns ZERO matches (no stale concurrency claim).
- [ ] docs/cli.md Busy=5 row (L374) + contention prose (L379) confirmed present (P1.M1.T2.S1).
- [ ] docs/how-it-works.md "Per-repo run lock (FR52)" subsection (L144–157) confirmed present (P1.M1.T1.S1).
- [ ] `git diff --stat -- README.md` shows ONLY README.md changed (one appended paragraph).
- [ ] `git diff --stat -- docs/` is EMPTY (sibling-owned docs NOT edited by this task).
- [ ] No `*.go`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, or `plan/` file modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact insertion point (README FAQ answer, line 326–328),
the exact current text to append after, the verbatim drafted paragraph (substance fixed by contract
3a), the exact grep command for the sweep with its expected zero-match result, the exact line numbers
of the two sibling docs to verify, the consistency anchors (exit codes / message fragments /
per-host caveat) so the wording doesn't drift from cli.md/how-it-works.md, and the hard scope
boundaries (which docs are NO-TOUCH). The research note and system_context §5 pin the rationale and
the per-host caveat. No inference required.

### Documentation & References

```yaml
# MUST READ — the feature spec, the design rationale, and this task's research
- file: PRD.md
  why: "§18.5 (FR52 per-repo run lock) is the authoritative spec: the lock is the FIRST line of
        defense (prevents the common local double-run); the §13.5 CAS is the SECOND (the
        never-clobber-HEAD guarantee, which holds even cross-host / shared FS). 'Scope' (commit-
        producing actions acquire; read-only bypass), 'Contention behavior' (no-op fast path → exit 0;
        genuine second batch → exit Busy naming pid/host/repo), and 'Limits' (per-host — cross-host is
        the CAS's job). §21.5 (README structure) puts safety in the FAQ."
  critical: "§18.5 'Limits' is WHY the README caveat is mandatory: the lock is per-host; do NOT pitch
             it as the shared-FS guarantee. The 'never-clobber-HEAD' framing and the 'nothing to do' /
             Busy outcomes are the wording the README must mirror."

- docfile: plan/006_c23e6f286ae7/architecture/system_context.md
  why: "§5 (Defense in depth — the two layers) is the table the README property composes with: lock =
        per-host, prevents the local double-run; CAS = universal, never-clobbers-HEAD. §1 confirms the
        feature is code-complete (lock primitive + contention wiring landed)."
  critical: "§5 is the source of the 'don't overclaim shared-filesystem safety' rule (contract 3a).
             The README paragraph's parenthetical caveat is a direct echo of §5's per-host row."

- docfile: plan/006_c23e6f286ae7/P1M1T3S1/research/readme_racefree_safety.md
  why: "THIS subtask's research: §2 (the empirical no-op verdicts for the sweep 3b and the catch-all
        3c, with exact line citations); §3 (the verbatim drafted README paragraph + why it satisfies
        all four contract-3a substance points); §4 (the scope table — which docs are EDIT vs NO-TOUCH);
        §5 (parallel-safety with T2.S3); §6 (decisions D1–D5). READ THIS FIRST."
  critical: "§2.1 (sweep returns ZERO matches) and §2.3 (sibling docs all present at HEAD) are the two
             findings that make 3b/3c no-ops. §3's drafted paragraph is the ONE edit. §4's scope table
             prevents editing sibling-owned docs."

- docfile: plan/006_c23e6f286ae7/P1M1T2S3/PRP.md
  why: "The parallel sibling (e2e contention tests, //go:build e2e). Confirms it touches NO docs and
        NO README — zero overlap with this task. Establishes that the feature's cross-process behavior
        is regression-tested (so the README property is backed by tests, not just claimed)."
  critical: "T2.S3 is TEST-ONLY under a build tag excluded from the default suite. It cannot conflict
             with this doc task. Do NOT touch its file."

# The edit site (EDIT — the one append)
- file: README.md
  why: "The FAQ '### Will it corrupt my repo?' answer (L326–328) is the safety section the contract
        names ('alongside Never corrupt your repo'). Current text: 'No. Stagehand uses `git write-tree`
        + `git commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves
        the repo byte-for-byte unchanged — it never touches the live index during generation.' Append
        the 'Safe to run twice' paragraph after it."
  pattern: "FAQ Q→A blocks separated by blank lines; each answer is 1–3 sentences. The new paragraph is
            a follow-on block under the SAME '### Will it corrupt my repo?' heading (the race-free
            property composes with atomicity — one safety section, two guarantees)."
  gotcha: "Do NOT edit the hero (README:4 'can never corrupt your repo') — it is CAS-defended and
           accurate, NOT stale; editing it would clutter the pitch (D1). Do NOT add a NEW ### heading —
           the contract says 'add a bullet or short paragraph' to the existing safety section. Append
           within the existing FAQ answer."

# The verify-only sibling docs (NO-TOUCH — confirm present, do not edit)
- file: docs/cli.md
  why: "Catch-all 3c target #1. The Busy=5 exit-code row (L374: '| 5 | Busy — another stagehand run
        holds the per-repo lock; retry after it finishes. |') and the contention prose (L379: no-op
        exit 0 vs Busy exit 5) are ALREADY PRESENT (landed by P1.M1.T2.S1)."
  pattern: "Exit-code table L370–376 (0/1/2/3/5/124); the Busy=5 row + the two-behavior prose block."
  gotcha: "This is P1.M1.T2.S1's file. VERIFY the row exists; do NOT edit it. If (impossible at HEAD)
           it were missing, that would be a T2.S1 regression to refer, not this task's edit."

- file: docs/how-it-works.md
  why: "Catch-all 3c target #2. The '### Per-repo run lock (FR52)' subsection (L144–157) is ALREADY
        PRESENT (landed by P1.M1.T1.S1): two-stage defense, per-host limit, never-in-repo location,
        no-op fast path, auto-release, Windows stub."
  pattern: "L144 heading '### Per-repo run lock (FR52)'; L146–157 the two-stage + per-host + location
            + fast-path + auto-release bullets."
  gotcha: "This is P1.M1.T1.S1's file. VERIFY the subsection exists; do NOT edit it. It already states
           the per-host limit and the CAS-as-second-defense — the README paragraph must MATCH it, not
           contradict it."

- file: docs/configuration.md
  why: "Informational. The '## Lock file location' section (L233–239) documents the XDG resolution
        (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagehand/locks; sha256 hash; never in repo).
        Already present and correct."
  gotcha: "NO-TOUCH. The README paragraph does NOT need to duplicate the lock-location detail (the
           README links to docs/ for depth). Keep the README paragraph about the PROPERTY (safe to
           double-invoke), not the mechanism internals."

# External references
- url: https://man7.org/linux/man-pages/man2/flock.2.html
  why: "Documents LOCK_EX|LOCK_NB (non-blocking) and that flock auto-releases on fd/process close —
        the 'no stale lock' / 'safe to double-invoke' property the README names. Underpins the per-host
        scope (flock is local to one host)."
  critical: "Confirms the per-host limit the README caveat must state: flock is per-host; cross-host
             shared-FS contention is NOT covered by the lock (the CAS is). Don't overclaim."
```

### Current Codebase Tree (this task's scope)

```bash
stagehand/
├── README.md                # EDIT — append the "Safe to run twice" paragraph to the FAQ safety answer
└── docs/
    ├── cli.md               # VERIFY-ONLY — Busy=5 row (L374) + contention prose (L379) present
    ├── how-it-works.md      # VERIFY-ONLY — "Per-repo run lock (FR52)" subsection (L144–157) present
    ├── configuration.md     # VERIFY-ONLY — "Lock file location" section (L233–239) present
    ├── providers.md         # NO-TOUCH — no concurrency content (sweep: zero matches)
    └── README.md            # NO-TOUCH — docs index, no concurrency content
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── README.md                # one appended paragraph in the FAQ "Will it corrupt my repo?" answer
    (all docs/ files unchanged; sibling-owned docs verified-present, not edited)
```

| Path | Action | Responsibility |
|---|---|---|
| `README.md` | EDIT (one append) | Append the "Safe to run twice" race-free paragraph to the FAQ safety answer (3a). |
| `docs/cli.md` | VERIFY (NO-TOUCH) | Confirm Busy=5 row + contention prose present (3c). |
| `docs/how-it-works.md` | VERIFY (NO-TOUCH) | Confirm "Per-repo run lock (FR52)" subsection present (3c). |
| `docs/configuration.md`, `docs/providers.md`, `docs/README.md` | VERIFY (NO-TOUCH) | Sweep returns nothing; no concurrency claims. |

**Explicitly NOT touched**: `docs/cli.md`, `docs/how-it-works.md`, `docs/configuration.md`,
`docs/providers.md`, `docs/README.md` (sibling-owned / no content); any `*.go` source or test (the
feature is code-complete — S1/S2 landed, S3 e2e in flight); `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `.gitignore`; anything under `plan/`.

### Known Gotchas of our codebase & toolchain

```markdown
<!-- CRITICAL (G1 — the sweep is a NO-OP; do NOT invent a stale claim). The targeted grep
'no lock|safe without|safely race|without a lock|cas is the (only|sole)|only.*defense' returns ZERO
matches across README.md + docs/ at HEAD. The README does not discuss concurrency today;
how-it-works.md already carries the correct two-stage (lock + CAS) defense. Do NOT rewrite the
accurate CAS/atomicity sentences to "find" something to fix. The honest 3b outcome is "verified — no
stale claims." (Research §2.1, D2.) -->

<!-- CRITICAL (G2 — the hero "can never corrupt your repo" is accurate, NOT stale). README:4's
corruption guarantee is defended by the CAS (§13.5), which is UNIVERSAL (holds cross-host / shared FS).
It is not overclaimed — the CAS genuinely prevents corruption under any concurrency (the loser aborts).
The lock is defense-in-depth for UX (avoid the redundant run / dangling snapshot), not a corruption
guard. Do NOT edit the hero. The new property APPENDS to the FAQ safety answer, not the pitch. (D1.) -->

<!-- CRITICAL (G3 — do NOT edit the sibling-owned docs). docs/cli.md (P1.M1.T2.S1), docs/how-it-works.md
(P1.M1.T1.S1), docs/configuration.md are all present and correct at HEAD. The catch-all (3c) is a
VERIFY, not an edit: if the Busy row or the FR52 subsection were missing you'd refer the regression to
the owning subtask, not edit their file here. Editing them is scope creep and risks conflicting with
landed work. (Research §4, D3.) -->

<!-- CRITICAL (G4 — the per-host caveat is MANDATORY). The README paragraph MUST state that on a shared
filesystem across hosts the lock can't help — the CAS is the never-clobber-HEAD guarantee there.
Omitting the caveat overclaims shared-FS safety (contract 3a: "don't overclaim shared-filesystem
safety — the CAS covers that, not the lock"). The hero's "can never corrupt your repo" is CAS-defended;
the lock must not be pitched as the source of that cross-host guarantee. (D4.) -->

<!-- GOTCHA (G5 — keep the README paragraph about the PROPERTY, not the mechanism internals). The README
links to docs/ for depth. State "a per-repo run lock prevents two concurrent commit-producing runs from
racing on HEAD" + the exit 0/5 outcomes + the per-host caveat. Do NOT inline the XDG lock-location
resolution, the sha256 hash, the flock LOCK_NB flag, or the Windows stub — those live in
configuration.md (L233–239) and how-it-works.md (L144–157). The README is the marketing surface (§21.5):
scannable, not exhaustive. -->

<!-- GOTCHA (G6 — stay consistent with the landed docs' exact tokens). The README paragraph's exit codes
(0, 5), the "nothing to do" fragment, and the "never-clobber-HEAD" framing must match docs/cli.md:379
and how-it-works.md:149/155 so the surfaces don't drift. Use exit `0` and exit `5` (Busy); quote the
no-op message as "nothing to do — an in-progress run already covers your staged changes." (D5.) -->

<!-- GOTCHA (G7 — append WITHIN the existing FAQ answer, do not add a new ### heading). The contract says
"Add a bullet or short paragraph" to the safety section "alongside 'Never corrupt your repo'." The
'### Will it corrupt my repo?' answer is that section. Append the new paragraph as a follow-on block
under the SAME heading (atomicity + race-free are two guarantees of one safety section). A new ###
heading (e.g. '### Is it safe to run twice?') is permissible but NOT required and adds TOC noise; the
contract favors the append. -->

<!-- GOTCHA (G8 — markdown formatting: blank-line separation). The FAQ answer blocks are separated by
blank lines. Append the new paragraph after a blank line following the existing atomicity sentence, and
leave a blank line before the next '### Does it send my code anywhere new?' heading. Run the repo's
markdownlint (`.markdownlint.json` exists) if unsure; the paragraph is plain prose + inline code for
the exit codes, no fences. -->

<!-- GOTCHA (G9 — this is doc-only; the build/test gate is a backstop). No .go file is in scope. Run
`go test -race ./...` only to confirm the repo is still green AFTER the edit (a markdown edit cannot
break the build). If the suite was already green (it is — feature code-complete), it stays green. -->
```

## Implementation Blueprint

### The README edit (the ONE change — verbatim drafted text)

Insert this paragraph immediately after the existing atomicity sentence in the FAQ "Will it corrupt
my repo?" answer (README:326–328), separated by a blank line:

```markdown
### Will it corrupt my repo?

No. Stagehand uses `git write-tree` + `git commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves the repo byte-for-byte unchanged — it never touches the live index during generation.

**Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from racing on HEAD, so an accidental double-invoke degrades gracefully: if nothing new is staged it exits `0` (*nothing to do — an in-progress run already covers your staged changes*); if genuinely new work is staged it exits `5` (Busy) and leaves your changes staged to re-run. (On a shared filesystem across hosts the lock can't help — the atomic `update-ref` CAS is the never-clobber-HEAD guarantee there.)
```

> The implementer may polish prose/length to match the README's voice, but the FOUR substance points
> are fixed by contract 3a and MUST remain: (1) safe to double-invoke / graceful degradation;
> (2) per-repo run lock prevents two concurrent commit-producing runs racing on HEAD; (3) exit 0
> "nothing to do" if nothing new staged, exit 5 (Busy) otherwise; (4) the per-host caveat (CAS, not
> the lock, covers cross-host shared FS). Exit codes `0`/`5` and the "nothing to do" / "never-clobber-
> HEAD" fragments must match docs/cli.md:379 + docs/how-it-works.md:149/155.

### Implementation Tasks (ordered)

```yaml
Task 1: EDIT README.md — append the race-free paragraph (the 3a deliverable)
  - OPEN README.md; locate the FAQ answer '### Will it corrupt my repo?' (L326).
  - FIND the existing atomicity sentence (L328): "No. Stagehand uses `git write-tree` + `git
    commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves the repo
    byte-for-byte unchanged — it never touches the live index during generation."
  - APPEND (after a blank line) the "Safe to run twice" paragraph from §"The README edit" above.
  - KEEP the blank line before the next heading '### Does it send my code anywhere new?' (L330).
  - DO NOT edit the hero (L4), the snapshot-workflow section (L257), or any other line.
  - DO NOT add a new ### heading (append within the existing FAQ answer — G7).
  - VERIFY: grep -n 'Safe to run twice' README.md → exactly ONE match.

Task 2: VERIFY the stale-claim sweep (3b) — expected NO-OP
  - RUN: grep -rniE 'no lock|safe without|safely race|without a lock|don.t need a lock|no need to lock|cas is the (only|sole)|only.*defense' README.md docs/
  - EXPECT: ZERO matches (exit 1). If a match appears, classify it: only a genuine assertion that two
    runs are safe WITHOUT a lock (or that the CAS is the SOLE defense) is stale → fix that one line to
    name the lock as the first defense + the CAS as the second. At HEAD no such claim exists. (G1/D2.)

Task 3: VERIFY the catch-all (3c) — expected NO-OP (sibling docs present)
  - RUN: grep -n '^| `5` | Busy' docs/cli.md
    EXPECT: one match at L374 ('| `5` | Busy — another stagehand run holds the per-repo lock; retry after it finishes. |').
  - RUN: grep -n 'nothing to do.*in-progress run already covers' docs/cli.md
    EXPECT: one match at L379 (the no-op-vs-Busy contention prose).
  - RUN: grep -n '^### Per-repo run lock (FR52)' docs/how-it-works.md
    EXPECT: one match at L144.
  - RUN: grep -n 'never-clobber-HEAD\|Per-host limit\|Never-in-repo location' docs/how-it-works.md
    EXPECT: matches within L146–157 (the two-stage defense + per-host + location).
  - IF any required line is MISSING (it is not at HEAD): do NOT edit the sibling file here — flag it
    as a regression in the owning subtask (cli.md → P1.M1.T2.S1; how-it-works.md → P1.M1.T1.S1). (G3/D3.)

Task 4: GATES + scope-check
  - RUN (backstop — markdown can't break the build, but confirm green): go build ./... ; go test -race ./...
    → EXPECT green (unchanged from HEAD).
  - RUN: git diff --stat -- README.md   → EXPECT: README.md (one appended paragraph).
  - RUN: git diff --stat -- docs/       → EXPECT: EMPTY (sibling docs NOT edited).
  - RUN: git diff --stat -- '*.go' PRD.md tasks.json prd_snapshot.md .gitignore
    → EXPECT: EMPTY (no source / forbidden files touched).
```

### Implementation Patterns & Key Details

```markdown
<!-- === Why the new property COMPOSES with (does not replace) the atomicity pitch === -->
<!-- The FAQ answer now carries TWO distinct safety guarantees:
     1. Atomicity (existing): a FAILED generation leaves the repo byte-for-byte unchanged (CAS is the
        only ref mutation; nothing is committed). Source of "can never corrupt your repo."
     2. Race-free (NEW): two CONCURRENT runs cannot race on HEAD (the lock serializes them; the loser
        no-ops or refuses). Source of "safe to run twice."
     Both belong in the safety section. They are independent: atomicity is about a single run's
     failure mode; the lock is about two runs' overlap. The CAS underpins BOTH (it is why the loser
     aborts cleanly). -->

<!-- === Why the per-host caveat is the load-bearing honesty clause === -->
<!-- The lock is a per-process advisory flock — local to one host. On a shared/network FS mounted by two
     machines, two stagehand processes on different hosts CAN both acquire (their flocks are local) —
     the §13.5 CAS catches that race (the loser's update-ref aborts). The README MUST say this so it
     does not overclaim the lock as a shared-FS guarantee. The hero's "can never corrupt your repo"
     stays accurate because it is CAS-defended (universal), not lock-defended (per-host). (system_context §5.) -->

<!-- === Why the README links to docs/ rather than inlining mechanism internals === -->
<!-- The README is the marketing surface (§21.5): scannable. The lock-location XDG resolution, the sha256
     hash, the flock LOCK_NB flag, the Windows no-op stub, and the full two-stage-defense table live in
     configuration.md (L233–239) and how-it-works.md (L144–157). The README paragraph states the
     PROPERTY (safe to double-invoke) + outcomes (0/5) + the caveat. Users wanting depth follow the
     existing docs/ links. (G5.) -->
```

### Integration Points

```yaml
README (EDIT):
  - FAQ "Will it corrupt my repo?" answer: append the "Safe to run twice" paragraph (3a)

DOCS (VERIFY-ONLY — sibling-owned, present at HEAD):
  - docs/cli.md:374          # Busy=5 exit-code row (P1.M1.T2.S1)
  - docs/cli.md:379          # no-op-vs-Busy contention prose (P1.M1.T2.S1)
  - docs/how-it-works.md:144 # "### Per-repo run lock (FR52)" heading (P1.M1.T1.S1)
  - docs/how-it-works.md:146–157 # two-stage defense + per-host + location + fast-path (P1.M1.T1.S1)
  - docs/configuration.md:233–239 # "## Lock file location" XDG resolution

CONSUMED (code-complete — not this task's concern):
  - internal/lock/* (S1), internal/cmd/default_action.go acquire+handleLockContention (S2),
    internal/exitcode Busy=5 (S2.S1), internal/e2e/lock_scenarios_test.go (S3, in flight)

GATE: grep sweep (3b) → zero matches ; sibling docs (3c) → present ; git diff → ONLY README.md

NO-TOUCH (explicitly):
  - docs/cli.md, docs/how-it-works.md, docs/configuration.md, docs/providers.md, docs/README.md
  - any *.go (production/test) — feature is code-complete
  - PRD.md, tasks.json, prd_snapshot.md, .gitignore, plan/*
```

## Validation Loop

### Level 1: The Edit + Sweep Gates

```bash
cd /home/dustin/projects/stagehand

# (1) The README edit landed (exactly one "Safe to run twice" paragraph).
grep -n 'Safe to run twice' README.md
# Expected: one match, inside the "### Will it corrupt my repo?" FAQ answer.

# (2) The new paragraph carries the four substance points (3a).
grep -c 'per-repo run lock' README.md         # Expected: ≥1 (the mechanism)
grep -c 'nothing to do' README.md             # Expected: ≥1 (exit-0 outcome)
grep -c 'Busy' README.md                      # Expected: ≥1 (exit-5 outcome)
grep -ci 'shared filesystem\|shared fs\|across hosts' README.md  # Expected: ≥1 (per-host caveat)

# (3) The stale-claim sweep (3b) — expected NO-OP.
grep -rniE 'no lock|safe without|safely race|without a lock|don.t need a lock|no need to lock|cas is the (only|sole)|only.*defense' README.md docs/
# Expected: ZERO output (exit 1). No stale concurrency claim.
```

### Level 2: The Catch-All Verification (3c — sibling docs present)

```bash
cd /home/dustin/projects/stagehand

# docs/cli.md: Busy=5 row + contention prose (P1.M1.T2.S1).
grep -n '^| `5` | Busy' docs/cli.md
# Expected: 374:| `5` | Busy — another stagehand run holds the per-repo lock; retry after it finishes. |
grep -n 'nothing to do.*in-progress run already covers' docs/cli.md
# Expected: 379 (the no-op-vs-Busy prose).

# docs/how-it-works.md: the FR52 subsection (P1.M1.T1.S1).
grep -n '^### Per-repo run lock (FR52)' docs/how-it-works.md
# Expected: 144:### Per-repo run lock (FR52)
grep -n 'never-clobber-HEAD' docs/how-it-works.md
# Expected: a match within L146–157 (the two-stage defense).

# docs/configuration.md: the lock-location section (informational).
grep -n '^## Lock file location' docs/configuration.md
# Expected: 233:## Lock file location
```

### Level 3: Scope Boundary + Build Backstop

```bash
cd /home/dustin/projects/stagehand

# (A) Scope: ONLY README.md changed (one appended paragraph).
git diff --stat -- README.md
# Expected: README.md | <small> +<n>  (one paragraph; no other README line changed).

git diff --stat -- docs/
# Expected: EMPTY (sibling-owned docs NOT edited by this task).

git diff --stat -- '*.go' PRD.md tasks.json prd_snapshot.md .gitignore
# Expected: EMPTY (no source / forbidden files touched).

# (B) Build backstop (markdown can't break the build; confirm the suite is still green).
go build ./...           # Expected: exit 0
go test -race ./...      # Expected: all packages green (unchanged from HEAD)
```

### Level 4: Cross-Doc Consistency (the README matches the landed docs)

```bash
cd /home/dustin/projects/stagehand

# The README paragraph's exit codes / fragments must match docs/cli.md + docs/how-it-works.md.
# Exit codes 0 and 5 (Busy):
grep -c '`0`' README.md && grep -c '`5`' README.md
# The "nothing to do" no-op fragment is shared with cli.md:379 / how-it-works.md:155:
diff <(grep -o 'nothing to do[^.]*' README.md | head -1) \
     <(grep -o 'nothing to do[^.]*' docs/cli.md | head -1) || true
# (No hard diff requirement — just confirm the README fragment is consistent in spirit, not
#  contradicting the docs. The README may abbreviate for length.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `grep -n 'Safe to run twice' README.md` → exactly ONE match in the FAQ safety answer.
- [ ] The new paragraph carries all four contract-3a points (lock mechanism; exit 0 nothing-to-do;
      exit 5 Busy; per-host caveat).
- [ ] The stale-claim sweep grep returns ZERO matches (3b no-op).
- [ ] docs/cli.md Busy=5 row (L374) + contention prose (L379) confirmed present (3c).
- [ ] docs/how-it-works.md "Per-repo run lock (FR52)" subsection (L144–157) confirmed present (3c).
- [ ] `go build ./...`, `go test -race ./...` green (backstop — markdown edit can't break the build).

### Feature Validation

- [ ] README's FAQ safety answer now covers BOTH guarantees: atomicity (failed gen = byte-for-byte
      unchanged) AND race-free (double-invoke degrades gracefully).
- [ ] The new paragraph does NOT overclaim shared-filesystem safety (the per-host caveat names the CAS
      as the cross-host guarantee, not the lock).
- [ ] The README wording is consistent with cli.md (exit 0/5, "nothing to do") and how-it-works.md
      ("never-clobber-HEAD", per-host limit) — no contradiction across surfaces.

### Scope Discipline Validation

- [ ] `git diff --stat -- README.md` shows ONLY README.md (one appended paragraph; no other line).
- [ ] `git diff --stat -- docs/` is EMPTY (cli.md / how-it-works.md / configuration.md / others
      UNCHANGED — verified-present, not edited).
- [ ] No `*.go`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, or `plan/` file modified.

### Documentation & Deployment Validation

- [ ] The new README paragraph is self-documenting (names the mechanism + outcomes + caveat).
- [ ] Markdown formatting is clean (blank-line separation; inline code for exit codes; no fence issues).
- [ ] The README links/scope stay marketing-surface-appropriate (property, not mechanism internals).

---

## Anti-Patterns to Avoid

- ❌ Don't invent a stale claim to fix. The sweep (3b) returns ZERO matches at HEAD — the README
  doesn't discuss concurrency today and how-it-works.md already has the correct two-stage defense.
  Rewriting the accurate CAS/atomicity sentences to "find" work is churn, not a fix. (G1/D2.)
- ❌ Don't edit the hero "can never corrupt your repo" (README:4). It is CAS-defended (universal) and
  accurate — NOT stale. The new property APPENDS to the FAQ safety answer, not the pitch. Editing the
  hero clutters the one-sentence pitch. (G2/D1.)
- ❌ Don't edit the sibling-owned docs (cli.md, how-it-works.md, configuration.md). They are present
  and correct at HEAD; the catch-all (3c) is a VERIFY. Editing them is scope creep and risks
  conflicting with landed P1.M1.T1.S1 / P1.M1.T2.S1 work. (G3/D3.)
- ❌ Don't omit the per-host caveat. The README MUST state that on a shared filesystem across hosts the
  lock can't help (the CAS is the guarantee there). Omitting it overclaims shared-FS safety — the
  explicit "don't overclaim" rule in contract 3a. (G4/D4.)
- ❌ Don't inline the lock mechanism internals (XDG resolution, sha256 hash, flock LOCK_NB, Windows
  stub) in the README. Those live in configuration.md / how-it-works.md; the README states the
  PROPERTY + outcomes + caveat and links to docs/ for depth. (G5.)
- ❌ Don't drift the README wording from the landed docs. Use exit codes `0`/`5`, the "nothing to do"
  fragment, and "never-clobber-HEAD" framing consistent with cli.md:379 / how-it-works.md:149/155.
  (G6/D5.)
- ❌ Don't add a new `### Is it safe to run twice?` heading unless the README's structure clearly calls
  for it. The contract says "add a bullet or short paragraph" to the existing safety section; appending
  within "### Will it corrupt my repo?" (atomicity + race-free as two guarantees of one section) is the
  cleaner read and avoids TOC noise. (G7.)
- ❌ Don't touch any `*.go` file, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, or `plan/`.
  This is doc-only.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-paragraph README append plus two no-op verifications, all empirically
pinned against HEAD. (1) The edit site is exact (README FAQ "Will it corrupt my repo?", L326–328) and
the verbatim drafted paragraph satisfies all four contract-3a substance points with consistency
anchors to the landed docs (exit 0/5, "nothing to do", "never-clobber-HEAD", per-host caveat). (2)
The stale-claim sweep (3b) was run and returns ZERO matches — no doc anywhere claims the CAS is the
sole defense or that two runs are safe without a lock; the README simply doesn't discuss concurrency
today. (3) The catch-all (3c) siblings are all confirmed present at HEAD: cli.md Busy row (L374) +
contention prose (L379), how-it-works.md FR52 subsection (L144–157), configuration.md lock-location
(L233–239). The PRP's primary value is scope discipline — front-loading the four gotchas (don't
invent a stale claim; don't edit the accurate hero; don't edit sibling docs; the per-host caveat is
mandatory) so the implementer makes exactly one README append and verifies the rest. The residual 0.5
uncertainty is purely editorial (the implementer may polish the paragraph's prose/length, risking an
accidental dropped substance point or a drifted exit-code token), which the Level-1/4 grep gates and
the consistency anchors catch. Parallel-safe with P1.M1.T2.S3 (an e2e test file under a build tag —
touches no docs). No source/build risk (doc-only).
