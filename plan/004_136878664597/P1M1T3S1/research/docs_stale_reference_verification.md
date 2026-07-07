# Docs Stale-Reference Verification Research — P1.M1.T3.S1

> Empirically verified against the live repo at commit `9d33b9e` ("make reasoning off by default for
> all roles"). This document is the source of truth for the verify-and-confirm PRP. Every line
> citation below was re-confirmed by running the actual `grep` before writing this file.

## 1. What this task is (and is not)

This is the **Mode B "Sync changeset-level documentation"** task for the plan/004 "reasoning is
opt-in everywhere" changeset. Per `critical_findings.md` Finding 4 and the item description, it is
**expected to be a verification no-op**: the conclusion (no separate cross-cutting doc sweep needed)
holds because each `docs/` file is either (a) owned Mode-A-wise by an implementing sibling subtask,
or (b) genuinely free of reasoning-DEFAULT claims.

The deliverable is the **verification** that no stale "planner defaults to high" (or equivalent)
text remains in user-facing documentation — NOT a batch doc rewrite. The PRP's most important job is
to **prevent false-positive edits**: a naive sweep that "fixes" a legitimate `--reasoning high`
opt-in EXAMPLE would itself introduce an error.

## 2. Scope: which files this task owns

| File | Owner | This task? |
|---|---|---|
| `docs/cli.md` | P1.M1.T1.S2 (Mode A — flag help) | ❌ already touched |
| `docs/configuration.md` | P1.M1.T2.S1 (Mode A — config example) | ❌ already touched (in flight) |
| `README.md` (repo root) | **P1.M1.T3.S1** | ✅ |
| `docs/how-it-works.md` | **P1.M1.T3.S1** | ✅ |
| `docs/providers.md` | **P1.M1.T3.S1** | ✅ |
| `docs/README.md` | **P1.M1.T3.S1** | ✅ |

The contract's grep (`README.md docs/`) spans the whole docs tree (so it incidentally re-hits the
two sibling-owned files), but **edits are only permissible in the four ✅ files.** Any finding in
`cli.md` / `configuration.md` is referred to its owning subtask (and in practice both are already
correct — see §4.2/§4.3).

## 3. The decision criterion: stale DEFAULT claim vs correct EXAMPLE

This is the single most important rule for the implementer. The grep pattern
`planner.*high|planner.*default.*high|reasoning.*planner.*high` matches BOTH categories; the
implementer must classify each hit:

| Category | Example | Verdict |
|---|---|---|
| **STALE default claim** (would need fixing) | "the planner defaults to high", "planner reasoning is high by default", "default off (planner: high)", "reasoning: high (default for planner)" | 🛠 UPDATE → "off for every role; opt-in per role" |
| **CORRECT opt-in example** (leave as-is) | `stagecoach --reasoning high`, `STAGECOACH_PLANNER_REASONING=high stagecoach`, `STAGECOACH_REASONING=high stagecoach` | ✅ LEAVE — shows the user CAN set it, not that it IS the default |
| **CORRECT shape/behavior doc** (leave as-is) | the `reasoning_levels` manifest TABLE definition (off/low/medium/high token lists); the Render append rule; "reasoning" used as a VERB ("needs reasoning to evaluate diffs") | ✅ LEAVE — documents mechanism, not a default |

The discriminator is **"does this text assert that `high` is the shipped default?"**. An invocation
example answers "how do I turn it on"; a default claim answers "what is it out of the box". Only the
latter is stale.

## 4. Per-file empirical findings (run against HEAD)

### 4.1 README.md (repo root) — ✅ CORRECT (no edits)

The narrow grep `planner.*high` returns **ZERO** matches here. The broader `reasoning` grep hits:

- **L121:** `# Use reasoning for deeper analysis on the planner` — a bash COMMENT introducing the
  next line's example.
- **L122:** `stagecoach --reasoning high` — a **usage EXAMPLE** in the "Example invocations" bash
  block. The item description states verbatim: *"README.md only references `--reasoning high` as a
  usage example (correct — leave as-is, it shows the user CAN set it, not that it IS the default)."*
  ✅ LEAVE.
- **L137–139:** the `> [!NOTE]` block: *"`--reasoning` is provider-dependent: it engages deeper
  reasoning for **pi** (`--thinking`) and **claude** (`--effort`). Other providers treat it as a
  graceful no-op (no error) per FR-R6. It applies to any role via `--<role>-reasoning` or
  `[role.*] reasoning`."* — describes **mechanism** (which providers honor it, that it's a graceful
  no-op, that it's per-role). NO default claim. ✅ LEAVE.

**Verdict: README.md is already coherent with the delta. Zero edits.**

### 4.2 docs/configuration.md — ❌ OUT OF SCOPE (T2.S1) — and correct anyway

The narrow grep returns **ONE** match repo-wide, and it is here:

- **L153:** `| STAGECOACH_PLANNER_REASONING | --planner-reasoning | Per-role: planner reasoning |
  STAGECOACH_PLANNER_REASONING=high stagecoach |` — the env-var table's **example column**. This is a
  **CORRECT opt-in example** (Category 2): it shows the user invoking the env var with `=high` to
  turn planner reasoning on. It does NOT claim `high` is the default. ✅ LEAVE.

  (Companion lines L152/154/155/156 show `STAGECOACH_REASONING=high`, `..._STAGER_REASONING=low`,
  etc. — all invocation examples, all correct.) And critically: **L80** —
  `reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)` — is the
  UPDATED, correct default statement landed by T2.S1. So the file is internally consistent.

  **This file is T2.S1's territory. This task must NOT edit it.** If it were somehow stale it would
  be referred to T2.S1; it is not stale.

### 4.3 docs/cli.md — ❌ OUT OF SCOPE (T1.S2) — and correct anyway

- **L43:** `| --reasoning <level> | string | "" (off) | …` — the flag table's **default column**
  reads `"" (off)`. ✅ CORRECT (the flipped default). T1.S2 landed this.
- **L44–49:** per-role `--<role>-reasoning` flags, default `""`. ✅ CORRECT.
- **L212–213:** `# Use reasoning for deeper analysis (pi: --thinking, claude: --effort; others
  no-op)` + `stagecoach --reasoning high` — **usage EXAMPLE**. ✅ CORRECT (Category 2).

  **T1.S2's territory. This task must NOT edit it.** Already correct.

### 4.4 docs/how-it-works.md — ✅ CORRECT (no edits)

The broader `reasoning` grep returns **ZERO** matches in this file (the word "reasoning" does not
appear). The word "planner" appears (L59, L71, L109, L113) but exclusively as the **role name** in
the decomposition-pipeline description — e.g. L59: *"| **planner** | bare | Analyze the full
working-tree diff; decide how many commits and what each covers |"*. These describe the planner
agent's JOB, not a reasoning-level default. The item description confirms: *"docs/how-it-works.md
has no reasoning references."* ✅ LEAVE.

### 4.5 docs/providers.md — ✅ CORRECT (no edits)

- **L35:** the manifest-schema row for `reasoning_levels`: *"Per-level reasoning-effort token lists
  (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6)…"* — documents the TABLE **SHAPE**
  (Category 3). The item description: *"docs/providers.md documents the manifest reasoning_levels
  field SHAPE (not defaults)"*. ✅ LEAVE.
- **L59:** the Render rule: *"When a `reasoning` level resolves to a non-empty token list…those
  tokens are appended…absent/empty ⇒ silent no-op."* — mechanism, not a default. ✅ LEAVE.
- **L108:** *"| **planner** | flagship / smart | Needs the strongest model for task decomposition
  and architecture reasoning. |"* — "reasoning" as a **VERB** (the model reasons about
  architecture), justifying the MODEL TIER, not a reasoning-LEVEL default. ✅ LEAVE (Category 3).
- **L111:** *"| **arbiter** | mid | Needs reasoning to evaluate diffs…"* — same: "reasoning" as a
  verb. ✅ LEAVE.

  Note: the tier table (L104–112) is about MODEL sizing (FR-D3), a separate axis from the reasoning
  LEVEL (FR-R6). Conflating the two would be an error; do not "fix" the word "reasoning" here.

### 4.6 docs/README.md — ✅ CORRECT (no edits)

The broader grep returns **ZERO** matches for both "reasoning" and "planner". The file is a
documentation index (links to cli.md/configuration.md/providers.md/how-it-works.md + install +
contributing). No reasoning content at all. ✅ LEAVE.

## 5. Conclusion: verification no-op

**Expected outcome: ZERO edits.** All four in-scope files (README.md, docs/how-it-works.md,
docs/providers.md, docs/README.md) are already coherent with the "off for every role" delta:

- README.md's `--reasoning high` is a correct opt-in example (the contract explicitly blesses it).
- how-it-works.md / docs/README.md have no reasoning-default content.
- providers.md documents the manifest table SHAPE and uses "reasoning" as a verb (model-tier
  rationale), neither of which is a default claim.

The single repo-wide narrow-grep match (docs/configuration.md:153) is (a) a correct opt-in example
and (b) outside this task's scope (T2.S1). No stale DEFAULT claim exists anywhere.

## 6. Decisions log

- **D1** — Usage examples (`--reasoning high`, `STAGECOACH_*_REASONING=high`) are CORRECT and must
  NOT be edited. The discriminator is "asserts `high` is the shipped default" (stale) vs "shows how
  to opt in" (correct). Only the former is in scope.
- **D2** — `docs/cli.md` (T1.S2) and `docs/configuration.md` (T2.S1) are OUT OF SCOPE for edits.
  The grep re-hits them incidentally; findings are referred to the owning subtask. Both are already
  correct at HEAD.
- **D3** — "reasoning" as a VERB (providers.md model-tier rationale: "needs reasoning to evaluate
  diffs") and the `reasoning_levels` TABLE-SHAPE doc are CORRECT (Category 3), not default claims.
  Do not grep-and-replace the word "reasoning".
- **D4** — The task's honest output is "verified complete — no doc edits", mirroring the sibling
  P1.M1.T2.S1 verify-and-confirm pattern. Do NOT fabricate before/after diffs to justify churn.
- **D5** — If (unlikely) a gate reveals an ACTUAL stale default claim in an in-scope file, the fix
  is a single surgical edit to that one line → "off for every role; opt-in per role (FR-R6)". The
  PRP prescribes this but expects it not to fire.
