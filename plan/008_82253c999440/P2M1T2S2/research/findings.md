# P2.M1.T2.S2 research findings — `docs/how-it-works.md` planner mode-conditional + files + soft target

Mode A doc edit (rides with P2.M1.T2.S1, the mode-conditional planner prompt builder). Docs-only.
All line numbers verified against the live tree (`docs/how-it-works.md`, 284 lines) by grep on
2026-07-05. Source of truth for behavior: PRD §9.14 FR-M3/M3b/M4 + §17.5; scout brief
`plan/008_82253c999440/docs/architecture/planner_prompt.md` §2.5.

---

## 1. The two edit spots (verbatim current text + exact line numbers)

### 1.1 Edit 1 — the planner OUTPUT cell in the four-roles table (line 59)

Full current row (line 59):
```
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[...], message?}` |
```

Only the OUTPUT cell (4th column) changes:
- CURRENT cell: `` JSON `{count, single, commits:[...], message?}` ``
- TARGET cell:  `` JSON `{count, single, commits:[{title,description,files}], message?}` ``

The Job cell (3rd column: "Analyze the full working-tree diff; decide how many commits and what each
covers") is UNCHANGED — the item scopes the change to the output cell only ("Keep it tight (one
paragraph + the table cell)").

The other three rows (stager / message / arbiter) are UNCHANGED.

### 1.2 Edit 2 — a NEW paragraph after "One-file short-circuit" (line 115), before "Arbiter leftover reconciliation" (line 117)

The "Key design points" bullet sequence is (bold leads):
- 101 Overlapped staging and generation
- 103 Stage-while-editing (FR-E2)
- 105 Frozen tree snapshots
- 107 Tree-to-tree diffs
- 109 Serialized publication
- 111 Start-of-run freeze (T_start)
- 113 Freeze enforcement
- 115 **One-file short-circuit.**  ← the new paragraph goes AFTER this
- 117 **Arbiter leftover reconciliation.**  ← and BEFORE this

So the new paragraph is inserted between line 115 and line 117 (there is one blank line 116 between
them; the new paragraph + its surrounding blank lines slots in there). Current line 115 verbatim:
```
**One-file short-circuit.** In auto-decompose, if exactly one path changed, the planner is bypassed entirely: stage that file's T_start content, generate one message, create one commit (FR-M2b). Deterministic, not model judgment. `--commits N` (N≥2) overrides this shortcut.
```

---

## 2. The exact target text (copy-paste-ready)

### 2.1 Edit 1 target — table row (only the output cell differs)

```
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[{title,description,files}], message?}` |
```

### 2.2 Edit 2 target — the new paragraph (bold lead + one tight paragraph)

```
**Mode-conditional planner rules.** The planner's `Rules:` block is mode-conditional. In auto-decompose (the default) it leans toward splitting unrelated changes — *lean toward SEVERAL* — tempered by a soft target of `max_commits / 2` (default 6) so an ordinary mixed tree lands at or below it rather than fanning into micro-commits; only the hard cap (`max_commits`, default 12) ever errors. Forced-count (`--commits N`) treats the count as fixed and omits the soft target. Every concept carries a `files` list naming each path it touches — a single file split across two concepts is named in both, with the description saying which part belongs where — so each stager knows where to look. After the planner returns, a deterministic coverage check logs (but never errors on) any changed path no concept claimed; the arbiter reconciles those leftovers.
```

This one paragraph covers all three required points (item §3 LOGIC):
1. mode-conditional rules block — auto leans toward SEVERAL + soft `max_commits/2` target; forced-count fixes the count  ✓
2. every concept carries a `files` list naming the paths it touches  ✓
3. deterministic coverage check logs (not errors) unclaimed paths → arbiter reconciles leftovers  ✓

---

## 3. Accuracy facts (verified against PRD §9.14 FR-M3/M3b/M4 + §17.5)

- **Soft target = `max_commits / 2` (default 6).** FR-M4: "the planner is also guided toward a soft
  target of `max_commits / 2` (default 6) ... guidance, not enforcement — it never errors." Integer
  division (12→6).
- **Hard cap = `max_commits` (default 12).** FR-M4: "Refuse to create more than `max_commits` commits
  ... unless the user explicitly sets a higher `--commits` / `--max-commits` (the hard cap)." Only the
  hard cap errors.
- **Forced-count omits the soft target.** PRD §17.5 forced-count rules block has NO soft-target line;
  PRD §17.5 line 1796 "swaps ONLY the `Rules:` block ... the opener, framing line, and JSON contract
  are unchanged." P2.M1.T2.S1 (the builder) branches on `forcedCount > 0` ⇒ forced rules (no soft
  target); `forcedCount <= 0` ⇒ auto rules with interpolated soft target.
- **`files` is per-concept.** FR-M3: "Each commit's `files` lists every path that commit touches, and
  `description` says — per file — WHICH change belongs to that commit ... a single file split across
  two concepts can be disambiguated by naming it in both and saying which part belongs where."
- **Coverage check is deterministic + non-fatal.** FR-M3b: "stagecoach unions the `files` declared
  across all concepts and compares against the frozen changed-path set (`DiffTreeNames(baseTree,
  T_start)`). Any path the planner left unclaimed is logged (verbose) ... the arbiter (FR-M9)
  reconciles it after the loop. This is a diagnostic only: it never aborts the run."
- **Auto-decompose precondition.** PRD §17.5: auto runs only when nothing was staged and the tree is
  dirty — "that precondition is itself the user's signal that they want the changes organized into
  commits for them." (Context for "leans toward SEVERAL"; NOT required in the doc paragraph, kept out
  for tightness.)

---

## 4. Scope fence (do NOT touch — verified)

### 4.1 `docs/cli.md` and `docs/configuration.md` — READ-ONLY (contract)

Confirmed by grep: neither mentions "soft target", "files partition", or "coverage check".
- `docs/cli.md` mentions `--max-commits` ONLY (lines 36, 399) — the hard cap flag; no soft-target/
  files/coverage surface. UNCHANGED by FR-M3/M3b/M4 (no new flags).
- `docs/configuration.md` mentions `max_commits` ONLY (lines 209, 217) — the `[generation].max_commits`
  key (default 12); no soft-target/files/coverage keys. UNCHANGED (no new config keys; the soft target
  is DERIVED from `max_commits`).

The item explicitly forbids touching them: "No new flags/keys — do NOT touch docs/cli.md or
docs/configuration.md." `git diff --stat` for both must be EMPTY.

### 4.2 Within `docs/how-it-works.md` — leave the accurate passages alone

- The four-roles table rows OTHER than the planner output cell (stager/message/arbiter) — unchanged.
- The planner Job cell ("decide how many commits and what each covers") — unchanged (item scopes to
  the output cell only).
- Lines 101–113 bullets (Overlapped / Stage-while-editing / Frozen tree snapshots / Tree-to-tree diffs
  / Serialized publication / Start-of-run freeze / Freeze enforcement) — accurate, unchanged.
- Line 115 (One-file short-circuit) — unchanged (the new paragraph goes AFTER it, not replacing it).
- Line 117 (Arbiter leftover reconciliation) — unchanged (it already describes the frozen leftover
  gate, courtesy of P1.M1.T2.S2; do NOT re-touch).
- All other sections (single-commit flow, prompt engineering, hook mode, lock, etc.) — unchanged.

### 4.3 Code / tests / other docs — out of scope (sibling ownership)

- `internal/prompt/planner.go`, `internal/decompose/planner.go`, `internal/prompt/planner_test.go`,
  `internal/prompt/reserve_test.go` → P2.M1.T2.S1 (the builder this doc rides with; parallel).
- `internal/decompose/decompose.go` (FR-M3b coverage check) → P2.M1.T1.S2 (COMPLETE).
- `internal/prompt/stager.go` (files block + guardrails) → P2.M1.T3.S1.
- `PlannerCommit.Files` + `ParsePlannerOutput` → P2.M1.T1.S1 (COMPLETE).
- `README.md` → P3.M1.T1.S1 (changeset sync).
- `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*` → never (forbidden).

---

## 5. Voice / style notes (verified against the live doc)

- **Em-dashes (—) are USED in this doc's prose.** E.g. line 113 "Any deviation — a concurrent change
  swept in ... — is a hard abort"; line 117 "the `T_start` content no stager claimed ... ". The new
  paragraph MAY use em-dashes to match. ⚠️ This is DIFFERENT from the P2.M1.T2.S1 ASCII rule — that
  rule is for the Go PROMPT CONSTS in `internal/prompt/planner.go` only, NOT for this Markdown doc.
  Do NOT substitute " -- " in the doc paragraph.
- **Bold-led single paragraphs.** Every "Key design point" is `**Title.** <prose>` — one paragraph.
  Match that exactly (the new paragraph is ONE paragraph, bold lead).
- **Moderate code-symbol use.** `T_start`, `tree[i]`, `max_commits`, `--commits N`, `files`,
  `Rules:` — backticked inline. No raw Go primitive names (`DiffTreeNames`, `VerboseRawOutput`,
  `OverlayTreePaths`) — those live in the PRD/code; this doc states the user-visible behavior.
- **FR citations are occasional in this doc.** Neighbors cite FR-M2b (line 115), FR-M12 (line 113),
  FR-E2 (line 103), FR-X5 (line 156). The new paragraph is kept citation-free for tightness (the item
  says "Keep it tight"); the three behaviors are self-explanatory. (Optional: a parenthetical
  "(FR-M3/M3b/M4)" could be appended for consistency, but is NOT required and is omitted here.)
- **Present tense.** Mode A rides with the work — by the time this lands, the mode-conditional prompt
  + files + coverage check ARE the behavior. Do NOT write "will" / "once S1 lands".

---

## 6. Validation gates (editorial greps — no compile/test for docs)

- `grep -n 'commits:\[\.\.\.\]' docs/how-it-works.md` → NO output (the old output cell is gone).
- `grep -n 'commits:\[{title,description,files}\]' docs/how-it-works.md` → exactly 1 hit (line 59).
- `grep -n 'Mode-conditional planner rules' docs/how-it-works.md` → exactly 1 hit (the new paragraph).
- `grep -n 'soft target' docs/how-it-works.md` → exactly 1 hit (the new paragraph).
- `grep -n 'coverage check' docs/how-it-works.md` → exactly 1 hit (the new paragraph).
- `grep -n 'lean toward SEVERAL' docs/how-it-works.md` → exactly 1 hit (the new paragraph).
- `git diff --stat -- docs/` → ONLY `docs/how-it-works.md`.
- `git diff --stat -- docs/cli.md docs/configuration.md` → EMPTY.
- `git diff --stat -- internal/ pkg/ cmd/ README.md` → EMPTY.
- Read-through lines 55–120 once: the table cell, the One-file short-circuit bullet, the new paragraph,
  and the Arbiter bullet are mutually consistent (all describe the planner→stager→arbiter flow with
  files partitioning + frozen leftovers).
