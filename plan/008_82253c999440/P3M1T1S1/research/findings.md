# Findings — P3.M1.T1.S1 (README.md: surface v2.2 decompose improvements in the feature list)

Source of truth: the work-item contract + README.md (current 385 lines) + PRD §5 / §9.14 (FR-M1d, FR-M3,
FR-M4) + plan/008_82253c999440/docs/architecture/system_context.md (lines 88-89) + the parallel PRPs
(P1.* arbiter freeze parity COMPLETE; P2.* planner files + soft target IMPLEMENTING).

---

## §1 — The two README surfaces that mention decompose TODAY (and the gap)

`grep -n` of README.md finds decompose in exactly FOUR places:

1. **Hero pitch blockquote** (README lines 3-5) — "...it automatically decomposes your changes into a
   sequence of logically-coherent commits." → **FROZEN. Do NOT alter** (work item: "Do NOT alter the hero
   pitch"). Line 6's "v2.1 adds..." tagline is also OUT OF SCOPE (not the feature list; not asked).
2. **Comparison table** (line 32) — "Multi-commit decomposition | No | Yes (auto-decompose dirty tree into
   N logical commits)" + line 32 per-role row mentions "arbiter". → Accurate as-is; NO v2.2 change needed
   (it's a yes/no capability comparison, not an improvement-level blurb). LEAVE UNCHANGED.
3. **`## Features` capability table** (lines 59-71) — rows: Payload exclusions, Payload optimization,
   Message shaping, Git hook mode, Tool integrations, --edit/--push, Discovery. → **NO decompose row
   exists.** This is THE gap: the flagship v2 capability is missing from the feature list. Every other
   v2.x capability is listed; decompose is not. THIS is where v2.2 surfaces.
4. **`### Multi-commit decomposition` narrative** (line 142, under Quick start) — the detailed blurb. It
   says: "A start-of-run freeze (T_start) captures your entire change set up front, so files you change
   mid-run are excluded from every commit — the run only ever commits what existed when it started." This
   is the v2.0/v2.1 wording: it states the stager-loop freeze but does NOT surface that the arbiter is now
   ALSO freeze-safe (the v2.2 FR-M1d completion), nor the per-file planner (FR-M3) or the soft count target
   (FR-M4). → LIGHT REFRESH to surface v2.2 (arbiter-inclusive freeze + per-file + soft target).

## §2 — The v2.2 changeset to surface (from P1 + P2, both COMPLETE or IMPLEMENTING)

Per system_context.md line 89 + the work item, the v2.2 decompose improvements are **internal-quality**
(no new flags/keys — confirmed: P1 and P2 add zero CLI flags and zero config keys):

- **Arbiter fully freeze-safe (FR-M1d — P1 COMPLETE).** The start-of-run freeze (T_start) now holds across
  the leftover-reconciliation arbiter too: the arbiter's GATE, the DIFF it's shown, and the TREES it
  commits are all derived from frozen SHAs (T_start + tipTree) — never a live `git status` re-read, never a
  live `git add -A`. Net user-visible guarantee: **a file created/modified by a concurrent process during
  the run can never enter ANY commit, including an arbiter reconciliation commit.** (v2.0-v2.1 had a
  loophole: the arbiter gate read live `git status --porcelain` and the resolution ran `git add -A` against
  the live tree, so a concurrent change during the planner call could silently land in an arbiter commit.
  FR-M1d closes it.)
- **Planner partitions per-file (FR-M3/M3b — P2.M1.T1 COMPLETE).** The planner now declares, per concept,
  the exact file list it touches (`PlannerCommit.Files`), surfaced to the stager as a guidance block
  ("where these changes live"). Coverage is checked deterministically (FR-M3b logs unclaimed paths as
  likely leftovers — non-fatal diagnostic).
- **Soft count target (FR-M4 — P2.M1.T2 COMPLETE).** In auto-decompose mode the planner is guided toward a
  soft target of `max_commits / 2` (default 6), interpolated into the planner prompt. It's GUIDANCE, not
  enforcement (only the hard cap errors). Ordinary mixed trees land at/below half the cap.

The stager files block + guardrails wording (P2.M1.T3.S1 — IMPLEMENTING in parallel) is INTERNAL prompt
construction (no user-facing surface) — it does NOT need a README mention. Its narrative lives in
docs/how-it-works.md (the sibling S2 task). Do NOT mention stager prompt internals in README.

## §3 — The exact edits (README.md ONLY; 2 coordinated changes)

### Edit A (PRIMARY): ADD a "Multi-commit decomposition" row to the `## Features` table

The Features table (the "feature list") is missing the decompose row. INSERT it as the FIRST data row
(right after the `|---|---|` separator, before `| Payload exclusions |`) — decompose is the headline v2
capability (it's in the hero pitch + comparison table) and deserves top placement. The description is
concise, surfaces BOTH v2.2 improvements, and links to docs (NO per-flag/per-key enumeration — work item:
"do NOT duplicate per-key config reference or per-flag CLI reference").

Match the existing row link format (`([label](docs/...#anchor))` or `... · ...`). The anchor
`docs/how-it-works.md#multi-commit-decomposition` EXISTS (how-it-works.md line 47 `## Multi-commit
decomposition`).

### Edit B (SUPPORTING): LIGHTLY refresh the `### Multi-commit decomposition` narrative freeze sentence

Replace the single sentence "A start-of-run freeze (T_start) ... when it started." with a version that
(a) makes the freeze arbiter-inclusive and (b) adds a concise per-file + soft-target clause. Keep the
surrounding sentences (the four-role-pipeline opener; the stager-constraint sentence; "Stagecoach owns
every commit via git plumbing") byte-unchanged — they are accurate and out of the v2.2 scope. Keep it
concise (net +1 short sentence).

### What NOT to touch
- Hero pitch blockquote (lines 3-5) — FROZEN.
- Line 6 "v2.1 adds..." tagline — out of scope (not the feature list; not requested).
- Comparison table (line 32) — accurate; no v2.2 delta.
- The code examples in `### Multi-commit decomposition` (`--commits 3`, `--single`, `[role.planner]`,
  `--reasoning`) — those ARE the per-flag/per-role reference; they stay (the work item forbids DUPLICATING
  them in the feature-list blurb, not removing them from the how-to subsection).
- docs/how-it-works.md, docs/cli.md, docs/configuration.md — the sibling S2 task owns how-it-works.md; the
  other two are the authoritative per-flag/per-key reference (the README links INTO them). Do NOT edit any
  docs/ file.

## §4 — Validation approach (docs-only: no build/test impact)

This is a docs edit — there is NO Go code change, NO `go build`/`go test` impact. Validation is:
1. **Markdown table integrity**: the new row has exactly 2 cells delimited by `|`, matching the header
   (`| Capability | Description |`). Render-check: the row parses as a table row, not loose text.
2. **Link target exists**: `docs/how-it-works.md#multi-commit-decomposition` (confirmed: line 47).
3. **Hero-pitch intact**: byte-compare README lines 3-5 before/after (grep the verbatim opener).
4. **No flag/config duplication in the Features row**: the row description must NOT contain `--commits`,
   `--single`, `[role.`, `max_commits`, etc. (those live in docs/cli.md + docs/configuration.md + the
   how-to subsection).
5. **v2.2 actually surfaced**: grep README for "arbiter" (now in the narrative freeze clause) and a
   soft-target/per-file phrase in the Features row.
6. **Scope fence**: `git status --short` shows ONLY `M README.md` (no docs/ file, no code file).

## §5 — Scope boundaries (frozen / owned elsewhere)

- **README.md** — the ONLY file this task edits (Edits A + B).
- **docs/how-it-works.md** — the SIBLING task P3.M1.T1.S2 ("holistic decompose-section reconciliation").
  Do NOT touch it. README links INTO its `#multi-commit-decomposition` anchor; S2 owns the target's content.
- **docs/cli.md, docs/configuration.md** — the authoritative per-flag/per-key reference. Do NOT edit. The
  README Features row links to them; it must NOT duplicate their content.
- **Any `.go` file, go.mod, go.sum** — UNCHANGED (docs-only task; zero code impact). The v2.2 code is
  already COMPLETE (P1) or IMPLEMENTING (P2); this task only documents it.
- **PRD.md, tasks.json, prd_snapshot.md** — READ-ONLY (never modify).
- The version tagline (README line 6) — out of scope (the work item scopes narrowly to the "feature list").
