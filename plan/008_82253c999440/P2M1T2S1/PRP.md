---
name: "P2.M1.T2.S1 — Split plannerSystemPrompt into shared + auto/forced rules + soft-target interpolation + BuildPlannerSystemPrompt signature + call-site + prompt tests"
description: |

  Implement PRD §17.5 / §9.14 FR-M3/M4: split the single mode-agnostic `plannerSystemPrompt` const into
  SHARED blocks (opener, UNSTAGED framing, JSON-contract — now with `files`) + TWO swappable `Rules:`
  blocks (auto with an interpolated soft target of `max_commits/2`; forced-count with NO soft target),
  change `BuildPlannerSystemPrompt` to take `(forcedCount, maxCommits int)`, wire the single call-site,
  and rewrite/add the prompt tests. The hard cap (`decompose/planner.go:132`) is UNCHANGED; the soft
  target is guidance text only (never errors).

  CONTRACT (P2.M1.T2.S1, verbatim):
    1. RESEARCH NOTE: See planner_prompt.md §2.2 (verbatim rules blocks quoted from PRD §17.5). Current
       plannerSystemPrompt (prompt/planner.go:29-49) is a SINGLE mode-agnostic block. BuildPlannerSystemPrompt
       (examples, format, locale) at line ~87. callPlanner (decompose/planner.go:73) calls it; the hard cap
       is at planner.go:132 (unchanged). The JSON contract line in the current const has no 'files' — must
       add it (shared block). Soft-target interpolation: PRD §17.5 'interpolated from max_commits at build
       time (default 12 -> 6)'.
    2. INPUT: PlannerCommit.Files from P2.M1.T1.S1 (so the JSON contract references files). forcedCount and
       deps.Config.MaxCommits reach callPlanner already.
    3. LOGIC: (a) Split into consts plannerOpener, plannerUnstagedFraming, plannerJSONContract (WITH files
       + the two trailing clauses), plannerAutoRules (soft-target line uses fmt.Sprintf with maxCommits/2
       and maxCommits), plannerForcedRules (NO soft-target line). QUOTE EXACT RULES from PRD §17.5 verbatim
       — ASCII only. (b) BuildPlannerSystemPrompt signature: add `forcedCount, maxCommits int`; forcedCount>0
       => plannerForcedRules; else plannerAutoRules with interpolated soft target. Topology: opener + blank +
       framing + blank + rules + blank + JSON-contract + blank + (auto: examples; non-auto: formatScaffoldBody)
       + withLocale. (c) Update callPlanner (decompose/planner.go:73) to pass (forcedCount, deps.Config.MaxCommits).
       (d) PlannerReserveTokens takes the already-built sysPrompt — signature unchanged. TDD: rewrite
       TestBuildPlannerSystemPrompt_CanonicalExact; add ForcedCount_CanonicalExact; add SoftTarget_Interpolation;
       update Properties; update FormatModes_CanonicalExact; update reserve_test.go fixture.
    4. OUTPUT: A mode-conditional planner prompt builder for consumption by callPlanner; the hard cap
       (planner.go:132) UNCHANGED; soft target is guidance-only. Byte-identical shared prefix between auto
       and forced (opener+framing+contract).
    5. DOCS: none here — the Mode A doc edit is P2.M1.T2.S2.

  ⚠️ §1 — ASCII-ONLY CONSTS (the #1 byte-fidelity trap). The existing comment at prompt/planner.go:27
  mandates "§17.5 is ENTIRELY ASCII — no em-dash, no non-ASCII bytes" (the prompt BYTES; comments may use
  §/—). The NEW PRD §17.5 text has TWO em-dashes (U+2014) in the prompt body: the framing line ("organize
  — finding") and the auto rules ("two concepts — name it"). The work item says "verbatim — ASCII only".
  RESOLUTION (binding): in the CONST VALUES substitute the em-dash with " -- " (space hyphen hyphen space).
  The forced-rules block and the opener/JSON-contract have NO em-dashes. Every `want` string below is given
  VERBATIM with this substitution — copy/paste, do not retype, or byte-identity tests fail.

  ⚠️ §2 — SOFT TARGET IS GUIDANCE, NEVER AN ERROR (FR-M4). Only the hard cap (decompose/planner.go:132)
  errors. The soft target (`max_commits/2`) is interpolated text in the auto rules block; do NOT add any
  cap/clamp/error for it. Integer division (12→6, 10→5, 20→10, 4→2). Document odd values in the doc comment.

  ⚠️ §3 — SHARED-PREFIX IDENTITY. opener+framing is a TRUE contiguous prefix of both modes; plannerJSONContract
  is byte-identical in both but NON-contiguous (the differing rules block sits between). The contract's
  "byte-identical shared prefix (opener+framing+contract)" means: assert the contiguous opener+framing prefix
  is identical AND the JSON-contract substring is identical. See §TestBuildPlannerSystemPrompt_ForcedCount.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/prompt/stager.go` + `stager_test.go` → P2.M1.T3.S1 (stager files block + guardrails). This
      task does NOT touch the stager.
    - `internal/decompose/decompose.go` + `decompose_test.go` → P2.M1.T1.S2 (FR-M3b coverage check), running
      in PARALLEL. Different file; no conflict.
    - `internal/decompose/planner_test.go` → callPlanner signature UNCHANGED; tests call callPlanner only
      (NOT the builder) ⇒ ZERO edits, stays green (the safety-cap regression net).
    - `docs/how-it-works.md` → P2.M1.T2.S2 (Mode A doc). Item §5: NO DOCS here.
    - `internal/config/*`, `cmd/stagehand/*`, `docs/cli.md`, `docs/configuration.md` → no new flags/keys.
    - `internal/prompt/reserve.go` → PlannerReserveTokens signature UNCHANGED.
    - `PlannerCommit.Files` + `ParsePlannerOutput` → already COMPLETE (P2.M1.T1.S1); this task only makes the
      JSON-contract const reference `files` to match that field. Do NOT re-add the field or change parse code.

  DELIVERABLES (0 NEW files, 4 EDITED files):
    EDIT internal/prompt/planner.go        — split plannerSystemPrompt into 5 consts (opener/framing/
                                              JSONContract/autoRules/forcedRules); change BuildPlannerSystemPrompt
                                              signature (+forcedCount, maxCommits int); new mode-conditional
                                              topology + soft-target interpolation.
    EDIT internal/decompose/planner.go     — line 73 call-site: pass (forcedCount, deps.Config.MaxCommits).
    EDIT internal/prompt/planner_test.go   — REWRITE CanonicalExact (auto); ADD ForcedCount_CanonicalExact;
                                              ADD SoftTarget_Interpolation; UPDATE Properties; UPDATE call-sites
                                              in EmptyExamples/FormatModes_*; REBUILD FormatModes_CanonicalExact
                                              want from the new consts.
    EDIT internal/prompt/reserve_test.go   — ADD one focused test threading the new builder output through
                                              PlannerReserveTokens (existing sentinel test UNCHANGED).

  SUCCESS: forcedCount<=0 ⇒ auto rules with soft target "maxCommits/2 (half the max of maxCommits)";
  forcedCount>0 ⇒ forced rules, NO soft-target line; opener+framing+JSON-contract byte-identical across modes;
  the hard cap (planner.go:132) UNCHANGED; `go build/vet/test ./...` green; go.mod/go.sum unchanged; the 4
  files above are the ONLY changes; the stager, decompose.go coverage check, config, CLI, and docs are untouched.

---

## Goal

**Feature Goal**: Implement PRD §17.5 (FR-M3/M4) — replace the single mode-agnostic `plannerSystemPrompt`
const with a SHARED preamble (opener + UNSTAGED framing + JSON-contract-with-`files`) plus a
mode-conditional `Rules:` block: **auto-decompose** leans toward splitting (with an interpolated soft
target of `max_commits / 2`); **forced-count** (`--commits N`) fixes the count (no soft target).
`BuildPlannerSystemPrompt` gains `(forcedCount, maxCommits int)` to select the block and interpolate the
target. The hard cap stays in `decompose/planner.go:132` (unchanged); the soft target is guidance text,
never an error.

**Deliverable** (0 NEW + 4 EDITED):
1. **EDIT `internal/prompt/planner.go`** — delete `plannerSystemPrompt`; add 5 consts
   (`plannerOpener`, `plannerUnstagedFraming`, `plannerJSONContract`, `plannerAutoRules`, `plannerForcedRules`);
   change `BuildPlannerSystemPrompt` to `func BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string`;
   new topology + `fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)`.
2. **EDIT `internal/decompose/planner.go`** — line 73: add `forcedCount, deps.Config.MaxCommits` args.
3. **EDIT `internal/prompt/planner_test.go`** — rewrite `TestBuildPlannerSystemPrompt_CanonicalExact`;
   add `TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact`; add
   `TestBuildPlannerSystemPrompt_SoftTarget_Interpolation`; update `TestBuildPlannerSystemPrompt_Properties`;
   update call-sites in `_EmptyExamples` + `_FormatModes_*`; rebuild `_FormatModes_CanonicalExact` `want`.
4. **EDIT `internal/prompt/reserve_test.go`** — add one test threading the new builder output through
   `PlannerReserveTokens`.

**Success Definition**:
- `BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)` ⇒ opener + UNSTAGED framing + auto rules with
  `"...at or below 6 (half the max of 12)..."` + JSON-contract-with-`files` + style examples.
- `BuildPlannerSystemPrompt(examples, "auto", "", 3, 12)` ⇒ same opener + framing + JSON-contract
  (byte-identical) BUT forced rules (`"You MUST partition into EXACTLY..."`) and NO `"Keep the count modest"`.
- `maxCommits ∈ {12,10,20,4}` ⇒ soft target text `{6,5,10,2}`; forced-count prompt contains NEITHER number.
- `callPlanner` builds the right mode (auto vs forced) from `forcedCount`; the hard cap at line 132 is
  byte-unchanged and still fires only in auto mode.
- The stager, `decompose.go` (coverage check), config, CLI, docs, `PlannerCommit.Files`, `ParsePlannerOutput`,
  `PlannerReserveTokens` signature, and `decompose/planner_test.go` are UNCHANGED.
- `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 4 files change.

## User Persona

**Target User**: a developer who runs `stagehand` (default auto-decompose) on a mixed working tree and
wants the planner to split unrelated changes into coherent commits — but not fan a 3-concept tree out into
a dozen micro-commits. The soft target (`max_commits/2`) is the counterweight to "lean toward SEVERAL".

**Use Case**: a 5-file changeset spanning a refactor + its test + an unrelated docs tweak. Auto-decompose
should produce ~2–3 commits (at/below the soft target of 6), not 8. With `--commits 3`, the planner is told
the count is fixed; the soft-target line is OMITTED (it would be contradictory guidance).

**Pain Points Addressed**: today's single planner prompt says "Prefer FEWER commits" — it actively fights
decomposition (the wrong default for auto mode, which exists precisely to split). Forced-count mode reuses
the same auto-leaning prompt, sending contradictory signals. This task gives each mode the correct rules.

## Why

- **Closes PRD §17.5 / FR-M3 / FR-M4 at the prompt layer.** §17.5 line 1762: "the opener, the 'UNSTAGED'
  framing line, and the JSON contract are shared; only the `Rules:` block changes." Today they are NOT split.
- **Correct mode defaults.** Auto-decompose runs only when nothing is staged and the tree is dirty — that
  precondition IS the user's signal they want commits organized for them, so the prompt names it (the
  UNSTAGED framing line) and leans toward SEVERAL. The current "Prefer FEWER" is the v1 single-commit
  reflex, wrong for v2 auto mode.
- **Soft target = the counterweight (FR-M4).** "Lean toward SEVERAL" without a brake would fan trees into
  micro-commits. `max_commits/2` (default 6) is interpolated guidance (never an error); only the hard cap
  (`max_commits`, default 12) errors.
- **Forced-count is unambiguous.** `--commits N` means the count is settled; the prompt must not re-litigate
  it nor give a soft target that could contradict N. The forced rules block says "MUST partition into
  EXACTLY" and omits the soft-target line.
- **`files` in the contract.** FR-M3 (S1) added `PlannerCommit.Files`; the JSON-contract example line must
  now show `"files": [...]` so the model emits it. (S1 added the struct field + parse; this task adds the
  prompt-side contract line so the model knows to populate it.)

## What

A pure prompt-construction refactor: one const → five; one builder signature gains two ints; one call-site
gains two args; the prompt tests are rewritten/added. No behavior change to staging, the loop, the arbiter,
the freeze, the hard cap, parsing, or validation. No new types, no interface change, no import change.

### Success Criteria

- [ ] `plannerOpener`, `plannerUnstagedFraming`, `plannerJSONContract`, `plannerAutoRules`,
      `plannerForcedRules` consts exist in `internal/prompt/planner.go`, each ASCII-only, no trailing `\n`,
      byte-faithful to §5 below (PRD §17.5 with em-dash → ` -- ` and the soft-target `%d`/`%d` placeholders).
- [ ] The OLD `plannerSystemPrompt` const is DELETED (no orphan reference; `gofmt`/compile clean).
- [ ] `plannerJSONContract` includes `"files": ["<path>", ...]` in the commits array AND the two trailing
      clauses (the `single/message` clause + the `"files" must list...` / `Do NOT emit hunks or line numbers` clause).
- [ ] `BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string`
      assembles topology: opener + `\n\n` + framing + `\n\n` + (forced? forcedRules : Sprintf(autoRules,
      maxCommits/2, maxCommits)) + `\n\n` + JSONContract + `\n\n` + (auto examples | scaffold) + withLocale.
- [ ] `forcedCount > 0` ⇒ forced rules; `forcedCount <= 0` ⇒ auto rules (mirrors `BuildPlannerUserPayload`'s
      `forcedCount <= 0` ⇒ normal idiom). Soft target uses Go integer division `maxCommits/2`.
- [ ] `internal/decompose/planner.go:73` passes `(forcedCount, deps.Config.MaxCommits)`; line 74 (reserve)
      and line 132 (hard cap) are byte-unchanged.
- [ ] `PlannerReserveTokens` signature UNCHANGED; `internal/prompt/reserve.go` NOT edited.
- [ ] Tests: `TestBuildPlannerSystemPrompt_CanonicalExact` rewritten (auto, soft target 6/12);
      `TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact` added (§3 identity assertions);
      `TestBuildPlannerSystemPrompt_SoftTarget_Interpolation` added (table {12,10,20,4}→{6,5,10,2});
      `TestBuildPlannerSystemPrompt_Properties` updated (drop "Prefer FEWER"; add auto/forced/files);
      `_EmptyExamples` + `_FormatModes_*` call-sites pass the new args; `_FormatModes_CanonicalExact` `want`
      rebuilt from the new consts.
- [ ] `internal/prompt/reserve_test.go` gains ONE test threading the new builder output through reserve.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 4 files change.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the EXACT const bytes (§5 —
copy/paste), the ASCII em-dash rule (⚠️§1), the exact builder topology (§Implementation), the single
call-site (§1 of findings — line 73), the soft-target math (⚠️§2 — integer division, guidance-only), the
shared-prefix identity nuance (⚠️§3 — opener+framing contiguous; contract non-contiguous), the exact test
specs (§Validation), the scope fence (⚠️ above — stager/decompose.go/docs out of scope), and the reserve
sentinel correction (§3 of findings — existing test uses "P", add a new threading test). No git/provider/
stager/arbiter knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (exact const bytes + ASCII rule + topology + test inventory + scope)
- docfile: plan/008_82253c999440/P2M1T2S1/research/findings.md
  why: §1 (the ONE call-site at decompose/planner.go:73 + reserve line 74 unchanged); §2 (decompose/
       planner_test.go OUT of scope — calls callPlanner only, signature unchanged); §3 (reserve_test.go uses
       sentinel "P" — existing test unchanged, ADD a threading test); §4 (the ASCII em-dash rule + the 2
       em-dash sites); §5 (the EXACT ASCII const bytes — COPY/PASTE); §6 (soft-target table); §7 (topology);
       §8 (shared-prefix identity); §9 (test inventory current→target); §10 (4 files in scope vs. fence);
       §11 (validation commands).
  critical: §4+§5 (ASCII substitution " -- " — every `want` string must match or byte-identity fails),
       §7 (topology — the blank-line discipline is exact), §8 (shared-prefix is opener+framing contiguous
       PLUS contract as an identical non-contiguous substring).

# MUST READ — the architecture scout brief (verbatim PRD §17.5 rules + target topology + signature rationale)
- docfile: plan/008_82253c999440/docs/architecture/planner_prompt.md
  section: §2.2 (the SHARED blocks + the auto rules + the forced rules, quoted from PRD §17.5) and the
       `BuildPlannerSystemPrompt signature change` subsection (preferred shape mirrors BuildPlannerUserPayload's
       forcedCount idiom) and the Topology block.
  why: the verbatim PRD text (apply ⚠️§1 ASCII substitution when transcribing into consts) + the rationale
       for the (forcedCount, maxCommits) signature shape.
  gotcha: §2.2 quotes the em-dash verbatim ("two concepts — name it", "organize — finding"); the CONST must
       use " -- " (⚠️§1). The doc's §3.5 says reserve_test.go "must rebuild sysPrompt via the new builder" —
       that is STALE (the test uses sentinel "P"); see findings §3 for the correction.

# MUST READ — S1's PRP (PlannerCommit.Files — COMPLETE; this task's JSON-contract const now references files)
- docfile: plan/008_82253c999440/P2M1T1S1/PRP.md
  section: the PlannerCommit.Files field (json:"files"; guidance, not a constraint). S1 ADDS the struct field
       + parse round-trip; THIS task (T2.S1) adds the JSON-contract const line that tells the MODEL to emit it.
  critical: do NOT re-add or redeclare Files (S1 done). The contract line `"files": ["<path>", ...]` in
       plannerJSONContract is the prompt-side mirror of S1's struct field.

# MUST READ — the FILE TO EDIT: the const + the builder
- file: internal/prompt/planner.go   (EDIT)
  section: `plannerSystemPrompt` const (L29–49 — DELETE, replace with the 5 consts in §5); `BuildPlannerSystemPrompt`
       (L87 — signature + topology rewrite). `plannerUserInstruction`, `PlannerRetryInstruction`, `PlannerCommit`,
       `PlannerOutput`, `BuildPlannerUserPayload`, `ParsePlannerOutput`, `extractJSONObject` → UNCHANGED.
  why: the two load-bearing edits live here. The package convention (consts have NO trailing newline; the
       builder owns ALL inter-block `\n` placement) is already documented at L14–16 — follow it verbatim.
  pattern: mirror `BuildPlannerUserPayload`'s `forcedCount <= 0 ⇒ normal` idiom for the rules-block branch.
  gotcha: the `format=="auto"` vs scaffold branch is ORTHOGONAL to the forced/auto rules branch — they are
       independent dimensions (you can have format="conventional" + forcedCount=3). Do NOT conflate them.

# MUST READ — the single call-site
- file: internal/decompose/planner.go   (EDIT — ONE line)
  section: L73 `sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)`.
  why: add `, forcedCount, deps.Config.MaxCommits`. `forcedCount` is already a `callPlanner` param; `deps.Config
       .MaxCommits` is in scope. L74 (reserve) and L132 (hard cap) are byte-unchanged.
  gotcha: do NOT touch the hard cap (L132) — soft target is guidance-only and never errors.

# MUST READ — withLocale + formatScaffoldBody (the tail helpers — UNCHANGED, but the topology relies on them)
- file: internal/prompt/format.go   (READ-ONLY)
  section: `withLocale(s, locale)` (L40 — locale=="" ⇒ s unchanged; else TrimRight(s,"\n")+"\nWrite the commit
       message in <locale>.") and `formatScaffoldBody(format)` (L24 — conventional/gitmoji scaffold, "" for auto/plain).
  why: the builder's tail + locale. The "plain, locale ja" FormatModes case relies on withLocale trimming one
       "\n" of the trailing "\n\n" after plannerJSONContract (plain scaffold ⇒ empty tail).

# MUST READ — MaxCommits config (the interpolation source — READ-ONLY)
- file: internal/config/config.go   (READ-ONLY)
  section: `MaxCommits int toml:"max_commits"` (L101) + `MaxCommits: 12` default (L183).
  why: confirms default 12 (⇒ soft target 6) and that the value reaches callPlanner via deps.Config.MaxCommits.
  gotcha: config uses non-zero-wins overlay; maxCommits<=0 is cosmetic-only in the builder (do NOT clamp — out
       of scope; the hard cap guards runtime).

# MUST READ — reserve signature (UNCHANGED — confirms no reserve.go edit)
- file: internal/prompt/reserve.go   (READ-ONLY)
  section: `func PlannerReserveTokens(sysPrompt string, forcedCount int, context string, est TokenEstimator) int` (L89).
  why: takes the ALREADY-BUILT sysPrompt ⇒ its signature is unaffected by the builder's new args. No edit here.
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  planner.go            # EDIT: plannerSystemPrompt const (L29-49) → 5 consts; BuildPlannerSystemPrompt (L87) signature+topology
  planner_test.go       # EDIT: rewrite CanonicalExact; +ForcedCount; +SoftTarget; update Properties/EmptyExamples/FormatModes_*
  reserve.go            # READ-ONLY: PlannerReserveTokens signature unchanged
  reserve_test.go       # EDIT: +1 threading test (sentinel "P" test unchanged)
  format.go             # READ-ONLY: withLocale + formatScaffoldBody (unchanged helpers)
  payload.go            # UNCHANGED (contextBlock helper)
  stager.go             # FENCE — P2.M1.T3.S1
  stager_test.go        # FENCE — P2.M1.T3.S1
internal/decompose/
  planner.go            # EDIT — L73 call-site (ONE line; L74 reserve + L132 hard cap unchanged)
  planner_test.go       # READ-ONLY (calls callPlanner only; signature unchanged; stays green)
  decompose.go          # FENCE — P2.M1.T1.S2 (parallel)
  decompose_test.go     # FENCE — P2.M1.T1.S2 (parallel)
  stager.go             # FENCE — P2.M1.T3.S1
internal/config/config.go  # READ-ONLY (MaxCommits default 12)
```

### Desired Codebase tree with files to be added and responsibility of file

No NEW files. The 4 edited files and their new responsibilities are described in **Deliverable** above and
the Implementation Tasks below.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: prompt const BYTES must be ASCII-only (existing comment at planner.go:27). PRD §17.5 has TWO
// em-dashes (U+2014) in the prompt body (framing line + auto rules). Substitute "—" → " -- " in the CONST
// VALUES. Comments may keep §/—. Every test `want` string MUST use the same substitution (§5 gives verbatim bytes).

// CRITICAL: the rules-block branch (forcedCount>0 ⇒ forced) and the examples-vs-scaffold branch (format=="auto"
// ⇒ examples) are ORTHOGONAL. A user can run --commits 3 (forced) WITH a conventional format scaffold. Do NOT
// nest them or conflate them. The builder emits the rules block based on forcedCount; the tail based on format.

// CRITICAL: soft target uses Go INTEGER division (maxCommits/2). 12→6, 10→5, 20→10, 4→2, 11→5 (odd is
// unspecified by PRD but integer division is the only sane choice — document it). NEVER clamp, NEVER error.

// CRITICAL: plannerAutoRules is a const with TWO "%d" placeholders (the soft-target line). Render via
// fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits). plannerForcedRules has NO placeholders (plain const).

// GOTCHA: withLocale trims trailing "\n" before appending the locale line. The "plain" scaffold tail is "",
// so the buffer ends with the "\n\n" after plannerJSONContract; withLocale turns the second "\n" into the
// separator ⇒ "...line numbers.\nWrite the commit message in ja." (one "\n", not two). The FormatModes test
// must account for this (its "plain, locale ja" case wants a single "\n" before the locale line).

// GOTCHA: shared-prefix identity is split — opener+framing is a TRUE contiguous prefix of both modes; the
// JSON contract is byte-identical but NON-contiguous (the differing rules block sits between). The byte-identity
// test asserts BOTH: HasPrefix(forced, opener+framing) AND both contain the identical plannerJSONContract.

// GOTCHA: the existing planner.go:27 comment "§17.5 is ENTIRELY ASCII — no em-dash" remains TRUE after the
// ASCII substitution (the consts have no em-dash). Optionally tighten its wording; do NOT delete the invariant.
```

## Implementation Blueprint

### Data models and structure

No data-model change. `PlannerCommit.Files` (S1, COMPLETE) is only READ by this task (the JSON-contract
const now references `files` to match it). `PlannerOutput`, `BuildPlannerUserPayload`, `ParsePlannerOutput`,
`PlannerRetryInstruction`, `plannerUserInstruction` — all UNCHANGED.

### The 5 consts — EXACT ASCII bytes (copy/paste into `internal/prompt/planner.go`)

> Transcribe VERBATIM. Em-dash (U+2014) → ` -- ` (space hyphen hyphen space). The auto-rules soft-target
> `6`/`12` → `%d`/`%d` (rendered via `fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)`). No const
> has a trailing newline (package convention — the builder owns `\n` placement). Source: PRD §17.5
> (selected_prd_content h3.81) / architecture §2.2.

```go
// plannerOpener is the §17.5 opener (lines 1766-1768). ASCII; NO trailing newline.
const plannerOpener = `You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.`

// plannerUnstagedFraming is the §17.5 "UNSTAGED on purpose" framing line (line 1770). The PRD em-dash is
// rendered ASCII (" -- "). ASCII; NO trailing newline.
const plannerUnstagedFraming = `These changes were left UNSTAGED on purpose and handed to you to organize -- finding the real
commit boundaries is the job you were asked to do, not a fallback to resist.`

// plannerJSONContract is the §17.5 "Respond with ONLY JSON" block (lines 1789-1794), now INCLUDING "files"
// in the commits array (FR-M3) and the two trailing clauses. ASCII; NO trailing newline.
const plannerJSONContract = "Respond with ONLY JSON, no prose, no code fences:\n" +
	`{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}` + "\n" +
	`- If single is true, set count=1 and ALSO include "message": "<the full commit message>".` + "\n" +
	`- "files" must list every path this commit touches; "description" must say, per file, WHICH` + "\n" +
	"  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or\n" +
	"  line numbers."

// plannerAutoRules is the §17.5 auto-decompose Rules block (lines 1772-1780). The PRD em-dash is rendered
// ASCII (" -- "). The soft-target line carries TWO "%d" placeholders (arg0 = maxCommits/2 integer division,
// arg1 = maxCommits) interpolated at build time (PRD §17.5 line 1812: "interpolated from max_commits at
// build time (default 12 -> 6)"). ASCII; NO trailing newline.
const plannerAutoRules = `Rules:
- Split changes that serve DIFFERENT purposes into separate commits. Two changes you would
  describe with different verbs, or explain to a reviewer in separate sentences, almost always
  belong in separate commits. When torn between one commit and several, lean toward SEVERAL.
- Do not manufacture tiny commits. Group changes that only make sense together (a function plus
  its test, a refactor plus the callers it updates). A single commit is correct only when the
  whole changeset pursues ONE purpose.
- Keep the count modest: in ordinary cases at or below %d (half the max of %d). Only exceed that
  when the changes genuinely span many unrelated concerns; do not approach the max casually.
- Account for every changed path: each file in the diff should appear in some commit's "files".
  A single file may be split across two concepts -- name it in both and say, per file, WHICH
  part belongs here.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.`

// plannerForcedRules is the §17.5 forced-count Rules block (lines 1798-1805). NO em-dash, NO soft-target
// line (the count is fixed by the user via --commits N). ASCII; NO trailing newline.
const plannerForcedRules = `Rules:
- You MUST partition into EXACTLY the requested number of commits. Do not return more or fewer,
  and do not reconsider the count.
- Split changes that serve DIFFERENT purposes into separate commits; group changes that only
  make sense together (a function plus its test, a refactor plus the callers).
- Account for every changed path (each file in the diff in some commit's "files"); name it in
  both if a single file is split across two concepts, and say WHICH part per file.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.`
```

> NOTE on `plannerJSONContract` rendering: the JSON-contract line and the two clauses contain double-quotes,
> so use the existing file's mixed-literal style (raw `` ` `` for quote-heavy lines, `"` for the rest, `+"\n"`
> joins) — exactly as the CURRENT `plannerSystemPrompt` test `want` already does (see planner_test.go L31–33).
> The bytes above are authoritative; the Go quoting is the implementer's choice as long as the BYTES match.

### BuildPlannerSystemPrompt — new signature + topology

```go
// BuildPlannerSystemPrompt assembles the §17.5 planner system prompt. The opener, the UNSTAGED framing
// line, and the JSON contract are SHARED across modes; only the Rules block is mode-conditional (§17.5
// line 1762). forcedCount > 0 ⇒ the forced-count rules (the count is fixed; NO soft target); forcedCount
// <= 0 ⇒ the auto-decompose rules with a soft target of maxCommits/2 interpolated into the third bullet
// (PRD §17.5 line 1812; FR-M4). The soft target is GUIDANCE (never errors — only the hard cap in
// decompose/planner.go:132 errors). maxCommits/2 is Go integer division (12→6, 10→5, 11→5 for odd values).
//
// The rules-block selection (forcedCount) is ORTHOGONAL to the examples-vs-scaffold selection (format):
// format=="auto" appends the §17.1 style examples ("---\n<msg>\n" each, same as system.go); any other
// format appends formatScaffoldBody(format) instead (FR-F5). locale, when non-empty, appends the FR-F6
// one-line language instruction (withLocale — a no-op when locale=="").
//
// ASSEMBLY TOPOLOGY (§17.5, exact):
//
//	plannerOpener                       // no trailing \n
//	"\n\n"                              // blank line
//	plannerUnstagedFraming              // no trailing \n
//	"\n\n"                              // blank line
//	<rules>                             // forcedCount>0 ⇒ plannerForcedRules;
//	                                    //   else fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)
//	"\n\n"                              // blank line
//	plannerJSONContract                 // no trailing \n
//	"\n\n"                              // blank line before examples/scaffold
//	auto: for each ex: "---\n" + ex + '\n'
//	non-auto: formatScaffoldBody(format)         // "" for plain
//	<withLocale(b.String(), locale)>
//
// Defensive: nil/empty examples ⇒ no "---" lines and no panic. The shared opener+framing+contract are
// byte-identical between auto and forced modes (only the Rules block differs).
func BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string {
	var b strings.Builder
	b.WriteString(plannerOpener)
	b.WriteString("\n\n")
	b.WriteString(plannerUnstagedFraming)
	b.WriteString("\n\n")
	if forcedCount > 0 {
		b.WriteString(plannerForcedRules) // forced-count: fixed count, NO soft target
	} else {
		b.WriteString(fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)) // auto: interpolated soft target
	}
	b.WriteString("\n\n")
	b.WriteString(plannerJSONContract)
	b.WriteString("\n\n") // blank line between the JSON contract and the style examples/scaffold
	if format == "auto" {
		for _, ex := range examples {
			b.WriteString("---\n") // one "---" BEFORE each message (same format as system.go)
			b.WriteString(ex)      // examples are pre-trimmed by RecentMessages
			b.WriteByte('\n')
		}
	} else {
		b.WriteString(formatScaffoldBody(format)) // scaffold REPLACES the examples (FR-F5)
	}
	return withLocale(b.String(), locale)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/prompt/planner.go — replace the const + rewrite the builder
  - DELETE: the `plannerSystemPrompt` const (L29-49).
  - ADD: the 5 consts in §"The 5 consts" ABOVE (plannerOpener, plannerUnstagedFraming, plannerJSONContract,
          plannerAutoRules with %d/%d, plannerForcedRules). ASCII-only; no trailing newlines.
  - REWRITE: BuildPlannerSystemPrompt — new signature (+forcedCount, maxCommits int) + topology in §above.
  - PRESERVE: plannerUserInstruction, PlannerRetryInstruction, PlannerCommit (with Files from S1), PlannerOutput,
          BuildPlannerUserPayload, ParsePlannerOutput, extractJSONObject — byte-unchanged.
  - NAMING: const names EXACTLY plannerOpener/plannerUnstagedFraming/plannerJSONContract/plannerAutoRules/
          plannerForcedRules (tests + future tasks reference them by these names).
  - KEEP the package-convention comment block (L11-27); tighten the "ASCII — no em-dash" line if helpful but
          do NOT delete the invariant (it stays true after the " -- " substitution).
  - DEPENDENCIES: none (leaf edit). `import "fmt"` already present (used by BuildPlannerUserPayload).

Task 2: EDIT internal/decompose/planner.go — the ONE call-site (L73)
  - CHANGE L73 from:
      sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)
    to:
      sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale, forcedCount, deps.Config.MaxCommits)
  - PRESERVE: L74 (reserve call — signature unchanged) and L132 (hard cap — byte-unchanged, soft target is
          guidance-only).
  - DEPENDENCIES: Task 1 (the new signature). forcedCount is already a callPlanner param; deps.Config.MaxCommits
          is in scope.

Task 3: EDIT internal/prompt/planner_test.go — rewrite + add tests + update call-sites
  - REWRITE TestBuildPlannerSystemPrompt_CanonicalExact: new auto-mode `want` (opener + framing + auto rules
          with "6"/"12" + JSON-contract-with-files + 2 examples). Call BuildPlannerSystemPrompt(examples,
          "auto", "", 0, 12). (See §Validation for the exact `want`.)
  - ADD TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact: BuildPlannerSystemPrompt(examples, "auto",
          "", 3, 12); assert (a) opener+framing prefix byte-identical to the auto variant, (b) both contain
          the identical plannerJSONContract substring, (c) contains "You MUST partition into EXACTLY", (d)
          does NOT contain "Keep the count modest".
  - ADD TestBuildPlannerSystemPrompt_SoftTarget_Interpolation: table maxCommits∈{12,10,20,4} → target∈{6,5,10,2};
          for each, assert the AUTO prompt contains fmt.Sprintf("at or below %d (half the max of %d)", n/2, n)
          and the FORCED prompt (forcedCount=3, same maxCommits) contains NEITHER number.
  - UPDATE TestBuildPlannerSystemPrompt_Properties: DROP {"rules section PRESENT","Prefer FEWER commits",true};
          ADD {"auto: lean toward SEVERAL PRESENT","lean toward SEVERAL",true}; ADD {"files in JSON contract
          PRESENT",`"files": ["<path>"`,true}; ADD a forced-mode sub-check that "MUST partition into EXACTLY"
          is ABSENT in the auto prompt. KEEP {"§17.1 subject-target line ABSENT","Target ~",false}. (Call with
          the new args: BuildPlannerSystemPrompt(examples, "auto", "", 0, 12).)
  - UPDATE TestBuildPlannerSystemPrompt_EmptyExamples: pass (ex, "auto", "", 0, 12); keep the no-"---" +
          header assertions; CHANGE the trailing-contract assertion from "find the exact changes." to the NEW
          contract's last line "line numbers." (the contract wording changed).
  - REWRITE TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact: rebuild `want` from the new consts —
          define once `sharedAuto := plannerOpener+"\n\n"+plannerUnstagedFraming+"\n\n"+fmt.Sprintf(
          plannerAutoRules,6,12)+"\n\n"+plannerJSONContract` and reuse for all cases (want = sharedAuto +
          "\n\n" + <scaffold> [+ locale via withLocale]). Pass (examples, tc.format, tc.locale, 0, 12) — note
          forcedCount=0 ⇒ auto rules even in non-auto FORMAT modes (the two dimensions are orthogonal). Add
          `import "fmt"` if not already present in the test file.
  - UPDATE TestBuildPlannerSystemPrompt_FormatModes_Properties: add (, 0, 12) to BOTH builder calls; the
          existing assertions (role line, JSON contract, no "---" non-auto, no example leak, auto has
          "---\nfeat: a") still hold.
  - LEAVE UNCHANGED: TestBuildPlannerUserPayload_*, TestPlannerRetryInstruction, TestParsePlannerOutput*,
          TestExtractJSONObject (they do not call the system-prompt builder; ParsePlannerOutput already has
          Files cases from S1).
  - DEPENDENCIES: Task 1.

Task 4: EDIT internal/prompt/reserve_test.go — ADD one threading test
  - ADD TestPlannerReserveTokens_ModeConditionalBuilderOutput (or similar): build autoSys := BuildPlannerSystemPrompt
          (nil, "auto", "", 0, 12) and forcedSys := BuildPlannerSystemPrompt(nil, "auto", "", 3, 12); assert
          PlannerReserveTokens(autoSys, 0, "", est) > 0 and PlannerReserveTokens(forcedSys, 3, "", est) > 0;
          assert the formula holds for autoSys: == est(autoSys) + est(PlannerRetryInstruction+"\n\n"+
          BuildPlannerUserPayload("", "", 0)) + reserveSafetyMargin. (Sanity: forcedSys ≥ autoSys is NOT
          required — both just must be positive + formula-correct. The POINT is exercising the new builder
          output through reserve without panicking.)
  - PRESERVE: the existing TestPlannerReserveTokens (sentinel "P") — UNCHANGED (it tests the formula in
          isolation and does NOT call the builder; compiles & passes as-is).
  - DEPENDENCIES: Task 1.
```

### Implementation Patterns & Key Details

```go
// PATTERN: mode-conditional rules selection mirrors BuildPlannerUserPayload's forcedCount idiom.
// BuildPlannerUserPayload uses `if forcedCount <= 0 { normal } else { forced prepend }`. The system prompt
// uses the SAME boundary: forcedCount <= 0 ⇒ auto rules; forcedCount > 0 ⇒ forced rules. Keep them consistent.

// PATTERN: the two dimensions (rules-block vs examples/scaffold) are INDEPENDENT — do not nest.
// CORRECT:
//   if forcedCount > 0 { forced } else { auto }
//   ... JSON contract ...
//   if format == "auto" { examples } else { scaffold }
// WRONG (conflates them):
//   if format == "auto" && forcedCount == 0 { ... }   // misses forced+conventional, forced+gitmoji, etc.

// PATTERN: soft-target interpolation is a single fmt.Sprintf over the WHOLE autoRules const (2 placeholders).
//   fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)
// Do NOT split the const or insert the line piecemeal — the %d/%d are the ONLY non-verbatim bytes.

// GOTCHA: the JSON-contract const contains double-quotes (the {"count":...} example + "files"/"description"
// clauses). Use Go's mixed raw/interpreted string literals (backticks for quote-heavy lines, "+" joins) —
// mirror the EXISTING planner_test.go `want` style (L31-33). The BYTES are authoritative, not the quoting.
```

### Integration Points

```yaml
NO DATABASE / NO CONFIG / NO ROUTES / NO CLI surface change.
  - This is a pure prompt-construction refactor. MaxCommits (the interpolation source) ALREADY exists in
    config (default 12); forcedCount ALREADY reaches callPlanner. No new flags, no new config keys.
BUILD:
  - `go build ./...` must pass (the signature change + the call-site).
  - go.mod/go.sum UNCHANGED (no new imports — `fmt`/`strings` already in prompt.go).
TESTS:
  - `go test ./internal/prompt/...` (the in-scope packages).
  - `go test ./internal/decompose/...` (regression net — callPlanner safety-cap tests stay green, untouched).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go internal/decompose/planner.go internal/prompt/reserve_test.go
go vet ./internal/prompt/... ./internal/decompose/...
# Expected: zero issues. gofmt alignment of the const block + the builder is the only stylistic concern.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The in-scope packages — the prompt builder + its tests.
go test ./internal/prompt/... -run 'BuildPlannerSystemPrompt|PlannerReserveTokens' -v
go test ./internal/prompt/... -v

# Regression net: callPlanner's signature is unchanged ⇒ these stay green with ZERO edits.
go test ./internal/decompose/... -run 'TestCallPlanner' -v
# Expected: all green. The safety-cap tests (TestCallPlanner_SafetyCap_Auto/_ForcedSkips) prove the hard cap
# is byte-unchanged and still fires only in auto mode.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-repo build + vet + test (catches any orphan reference to the deleted plannerSystemPrompt const).
go build ./... && go vet ./... && go test ./...
# Expected: GREEN. If a test still references `plannerSystemPrompt` by name, it fails to compile — grep:
grep -rn "plannerSystemPrompt" --include="*.go" .   # MUST return ZERO hits after the edit.
```

### Level 4: Byte-Identity (Domain-Specific Validation)

The canonical-exact tests ARE the byte-identity gate. The exact `want` strings (auto mode, default
maxCommits=12, ASCII em-dash substitution applied):

```go
// TestBuildPlannerSystemPrompt_CanonicalExact — AUTO mode want (examples = ["feat: a", "fix: b\n\nBody."]):
const want = "You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they\n" +
	"form ONE coherent commit or SEVERAL, and partition them into logical units.\n" +
	"\n" +
	"These changes were left UNSTAGED on purpose and handed to you to organize -- finding the real\n" +
	"commit boundaries is the job you were asked to do, not a fallback to resist.\n" +
	"\n" +
	"Rules:\n" +
	"- Split changes that serve DIFFERENT purposes into separate commits. Two changes you would\n" +
	"  describe with different verbs, or explain to a reviewer in separate sentences, almost always\n" +
	"  belong in separate commits. When torn between one commit and several, lean toward SEVERAL.\n" +
	"- Do not manufacture tiny commits. Group changes that only make sense together (a function plus\n" +
	"  its test, a refactor plus the callers it updates). A single commit is correct only when the\n" +
	"  whole changeset pursues ONE purpose.\n" +
	"- Keep the count modest: in ordinary cases at or below 6 (half the max of 12). Only exceed that\n" +
	"  when the changes genuinely span many unrelated concerns; do not approach the max casually.\n" +
	"- Account for every changed path: each file in the diff should appear in some commit's \"files\".\n" +
	"  A single file may be split across two concepts -- name it in both and say, per file, WHICH\n" +
	"  part belongs here.\n" +
	"- Each commit must be independently meaningful and reviewable.\n" +
	"- Respect dependencies: if change B depends on change A, A comes first.\n" +
	"- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.\n" +
	"\n" +
	"Respond with ONLY JSON, no prose, no code fences:\n" +
	`{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}` + "\n" +
	`- If single is true, set count=1 and ALSO include "message": "<the full commit message>".` + "\n" +
	`- "files" must list every path this commit touches; "description" must say, per file, WHICH` + "\n" +
	"  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or\n" +
	"  line numbers.\n" +
	"\n" +
	"---\n" +
	"feat: a\n" +
	"---\n" +
	"fix: b\n" +
	"\n" +
	"Body.\n"
// got := BuildPlannerSystemPrompt([]string{"feat: a", "fix: b\n\nBody."}, "auto", "", 0, 12)
```

```go
// TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact — assert shared identity + forced-only markers.
autoP := BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)
forcedP := BuildPlannerSystemPrompt(examples, "auto", "", 3, 12)
sharedPrefix := plannerOpener + "\n\n" + plannerUnstagedFraming // TRUE contiguous prefix of both modes
if !strings.HasPrefix(forcedP, sharedPrefix) || !strings.HasPrefix(autoP, sharedPrefix) {
	t.Error("opener+framing must be a byte-identical contiguous prefix of both modes")
}
if !strings.Contains(forcedP, plannerJSONContract) || !strings.Contains(autoP, plannerJSONContract) {
	t.Error("plannerJSONContract must be byte-identical (same substring) in both modes")
}
if !strings.Contains(forcedP, "You MUST partition into EXACTLY") {
	t.Error("forced prompt must contain the forced-count directive")
}
if strings.Contains(forcedP, "Keep the count modest") {
	t.Error("forced prompt must NOT contain the soft-target line")
}
if strings.Contains(forcedP, "lean toward SEVERAL") {
	t.Error("forced prompt must NOT contain the auto-only 'lean toward SEVERAL' bullet")
}
```

```go
// TestBuildPlannerSystemPrompt_SoftTarget_Interpolation — table.
for _, tc := range []struct{ max, half int}{{12,6},{10,5},{20,10},{4,2}} {
	auto := BuildPlannerSystemPrompt(nil, "auto", "", 0, tc.max)
	wantLine := fmt.Sprintf("at or below %d (half the max of %d)", tc.half, tc.max)
	if !strings.Contains(auto, wantLine) { t.Errorf("max=%d: want %q", tc.max, wantLine) }
	forced := BuildPlannerSystemPrompt(nil, "auto", "", 3, tc.max)
	if strings.Contains(forced, fmt.Sprintf("half the max of %d", tc.max)) {
		t.Errorf("max=%d: forced prompt must not carry the soft-target line", tc.max)
	}
}
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -w` clean on the 4 files; `go vet ./...` zero issues.
- [ ] `go build ./...` compiles (signature change + call-site).
- [ ] `go test ./...` GREEN (whole repo).
- [ ] `grep -rn "plannerSystemPrompt" --include="*.go" .` ⇒ ZERO hits (old const fully removed).
- [ ] go.mod/go.sum UNCHANGED.

### Feature Validation

- [ ] Auto prompt contains `"lean toward SEVERAL"`, the soft-target `"at or below 6 (half the max of 12)"`,
      and `"files": ["<path>"` in the JSON contract.
- [ ] Forced prompt contains `"You MUST partition into EXACTLY"`, does NOT contain `"Keep the count modest"`
      or `"lean toward SEVERAL"`, and shares the opener+framing+contract byte-for-byte with auto.
- [ ] `maxCommits ∈ {12,10,20,4}` ⇒ soft-target text `{6,5,10,2}`; forced prompt contains neither number.
- [ ] The hard cap at `decompose/planner.go:132` is byte-unchanged (regression: `TestCallPlanner_SafetyCap_*`
      green, untouched).
- [ ] `callPlanner` builds the correct mode from `forcedCount` (auto when ≤0, forced when >0).

### Code Quality Validation

- [ ] Const names EXACTLY `plannerOpener`/`plannerUnstagedFraming`/`plannerJSONContract`/`plannerAutoRules`/`plannerForcedRules`.
- [ ] All 5 consts are ASCII-only (no U+2014 em-dash — substituted with ` -- `) and have NO trailing newline.
- [ ] The two dimensions (rules-block via forcedCount; examples/scaffold via format) are independent, not nested.
- [ ] Package conventions followed (consts without trailing `\n`; builder owns `\n` placement; `forcedCount <= 0`
      boundary matches `BuildPlannerUserPayload`).
- [ ] The ASCII-only invariant comment at the top of the const block is preserved (still true).

### Scope Validation

- [ ] EXACTLY 4 files changed: `internal/prompt/planner.go`, `internal/decompose/planner.go`,
      `internal/prompt/planner_test.go`, `internal/prompt/reserve_test.go`.
- [ ] Stager (`internal/prompt/stager*.go`, `internal/decompose/stager.go`) UNTOUCHED (P2.M1.T3.S1).
- [ ] `internal/decompose/decompose*.go` UNTOUCHED (P2.M1.T1.S2, parallel).
- [ ] `internal/decompose/planner_test.go` UNTOUCHED (callPlanner signature unchanged).
- [ ] `docs/how-it-works.md` UNTOUCHED (P2.M1.T2.S2).
- [ ] `internal/config/*`, `cmd/stagehand/*`, `docs/cli.md`, `docs/configuration.md` UNTOUCHED.

---

## Anti-Patterns to Avoid

- ❌ Don't preserve the em-dash (U+2014) in the const values — the file mandates ASCII-only prompt bytes; use ` -- `.
- ❌ Don't nest the rules-block branch inside the format branch (or vice versa) — they are orthogonal dimensions.
- ❌ Don't add a clamp/error/clamp for the soft target — it is guidance text only; only the hard cap (L132) errors.
- ❌ Don't change `callPlanner`'s signature or the hard cap — only the L73 call-site gains two args.
- ❌ Don't edit the stager, decompose.go, config, CLI, or docs — out of scope (other work items).
- ❌ Don't redeclare `PlannerCommit.Files` or change `ParsePlannerOutput` — S1 is COMPLETE; only the JSON-contract
  const now references `files` to match the existing field.
- ❌ Don't split `plannerAutoRules` to insert the soft-target line piecemeal — use ONE `fmt.Sprintf` over the
  whole const with two `%d` placeholders.
- ❌ Don't leave an orphan reference to the deleted `plannerSystemPrompt` const (grep must return zero hits).
