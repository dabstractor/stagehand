# Planner `files` + soft target + mode-conditional prompt (FR-M3 / FR-M3b / FR-M4, PRD §17.5 / §17.6)

Scope: v2.2 delta, plan/008, **Phase 2**. This document is the scout brief for the implementing agent.
All line numbers are against the current tree (verified by reading, not assumed).

Source of truth for the verbatim rules blocks: `PRD.md` §17.5 (lines 1758–1814) and `plan/008_82253c999440/delta_prd.md` §4.2.

---

## 1. CURRENT state (verified)

### 1.1 `internal/prompt/planner.go`

- **`plannerSystemPrompt` const (lines 29–49)** — a SINGLE, mode-agnostic block. The load-bearing first rules bullet is literally:
  ```
  - Prefer FEWER commits. A single commit is correct unless the changes clearly span
    unrelated concerns. Do not manufacture tiny commits.
  ```
  There is **no** "UNSTAGED … handed to you to organize" framing line, **no** soft target, **no** "account for every changed path" line.
- **JSON contract line (lines 39–40)** currently has **no `files`**:
  ```
  {"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<precisely which files/hunks belong here, by path>"}, ...]}
  ```
  And the trailing two clauses:
  ```
  - If single is true, set count=1 and ALSO include "message": "<the full commit message>".
  - The "description" must be specific enough that a staging agent can find the exact changes.
  ```
- **`PlannerCommit` struct (lines 57–61)** — has `Title`, `Description`; **NO `Files`**:
  ```go
  type PlannerCommit struct {
      Title       string `json:"title"`
      Description string `json:"description"`
  }
  ```
- **`PlannerOutput` (lines 67–72)** — `Count`, `Single`, `Commits []PlannerCommit`, `Message`. Unchanged by this work.
- **`BuildPlannerSystemPrompt(examples []string, format, locale string) string` (lines 105–125)** — three args; always emits the single `plannerSystemPrompt` const then appends examples (auto) or scaffold (non-auto), then locale. No notion of mode/target.
- **`BuildPlannerUserPayload(diff, context string, forcedCount int) string` (lines ~138–165)** — already mode-aware (forced-count prepend). The delta does **not** change this function's body; only the **system prompt** gains mode-conditional rules. The forced-count directive string is unchanged.
- **`ParsePlannerOutput` (lines ~171–195)** — whole-string `json.Unmarshal` then brace-balanced fallback. Populates the struct fields generically; adding `Files` to the struct makes the parse populate it **for free** — no parse-code change required, just the struct field.

### 1.2 `internal/prompt/stager.go`

- **`BuildStagerTask(title, description string) string` (lines 64–68)** — no `files` param.
- **`stagerInstruction` const (line 20)**: `"Stage, but do NOT commit, all changes in this repository that match this concept:"` — unchanged.
- **`stagerGuardrails` const (lines 38–42)** — five lines. The line that must change wording:
  ```
  only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this
  concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not
  ```
  Target wording (PRD §17.6): "Stage ONLY the changes the description assigns to this concept (the files above are where they live); leave everything else unstaged."
- No `files` block anywhere today.

### 1.3 `internal/decompose/planner.go`

- **`callPlanner` (lines ~52–148)** — builds `sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)` at line 73, then `reserve := prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context, git.EstimateTokens)` at line 74.
- **Hard cap at lines ~129–133** (auto mode only):
  ```go
  if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits {
      return prompt.PlannerOutput{}, fmt.Errorf(
          "planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits",
          parsed.Count, deps.Config.MaxCommits)
  }
  ```
  No soft target. No coverage check.
- **`validatePlannerOutput` (lines ~152–172)** — Count≥1, single⇔message contract. Does NOT inspect `Files` (and should not — files is guidance).

### 1.4 `internal/decompose/decompose.go`

- **Line 182** — the frozen changed-path set the coverage check compares against:
  ```go
  changedPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
  ```
  This `changedPaths` is currently scoped to the one-file short-circuit block (line 175 `if deps.Config.Commits == 0 && !deps.Config.Single`). **FR-M3b needs the same `DiffTreeNames(baseTree, tStart)` set in the planner-acceptance path.** `runLoop` already re-computes it at line 397 as `tStartPaths` for freeze-subset verification — that is a separate concern (do not reuse it; the coverage check belongs right after `callPlanner` returns, before `runLoop`).
- **Line ~199** — `out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)`. The coverage check (FR-M3b) plugs in **between this line and the `if out.Single` check at line ~205**.
- **Line ~213** — `runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)` consumes `concepts []prompt.PlannerCommit`. Adding `Files` to the struct is transparent here.

### 1.5 `internal/decompose/stager.go`

- **Line 99** — `task := prompt.BuildStagerTask(concept.Title, concept.Description)`. This single call site must become `prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)`.

### 1.6 Tests (current shape — all are byte-identity / golden / block-assertion)

- `internal/prompt/planner_test.go`:
  - `TestBuildPlannerSystemPrompt_CanonicalExact` (lines ~14–58) — pins the **full** assembled string byte-for-byte. **Will need a full rewrite** of the `want` constant because the opener, rules, and JSON-contract all change, AND new args (mode/maxCommits) are added to the builder.
  - `TestBuildPlannerSystemPrompt_Properties` (lines ~61–109) — block-presence table; asserts `"Prefer FEWER commits"` is PRESENT (mustBecome auto-vs-forced specific) and `"Target ~"` is ABSENT (still true).
  - `TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact` (lines ~113–155) — compares against `plannerSystemPrompt + "\n\n" + <scaffold>`. **Breaks** because `plannerSystemPrompt` is being split; the shared prefix constant name changes.
  - `TestBuildPlannerSystemPrompt_FormatModes_Properties` (lines ~159–185).
  - `TestParsePlannerOutput` (lines ~226–321) — table-driven; the round-trip cases need a `Files` field added.
  - `TestParsePlannerOutput_RoundTrip` (lines ~364–392) — struct round-trip; add `Files` to the original.
- `internal/prompt/stager_test.go`:
  - `TestBuildStagerTask_CanonicalExact` (lines ~12–34) — full byte-identity `want`. **Breaks** (new `files` arg + guardrails wording).
  - `TestBuildStagerTask_Properties` (lines ~37–135) — many needle assertions, including `"Stage ONLY changes belonging to this\nconcept"` which becomes the new wording.
- `internal/decompose/planner_test.go`:
  - `TestCallPlanner_SafetyCap_Auto` (lines ~260–300) and `TestCallPlanner_SafetyCap_ForcedSkips` (lines ~303–329) — pin the hard-cap behavior. These stay green (cap logic unchanged); they are the regression net for FR-M4 hard-cap preservation.
  - The `callPlanner(...)` signature itself is unchanged; only the system-prompt content it builds changes. Existing stub-JSON fixtures (`validMultiJSON`, `overCapJSON`) do not carry `files` — fine (parse tolerates absence → nil `Files`).
- `internal/prompt/reserve_test.go`:
  - `TestPlannerReserveTokens` (line ~101) calls `PlannerReserveTokens(sysPrompt, ...)`. `PlannerReserveTokens` signature is unchanged, but the `sysPrompt` it receives is now mode-conditional — the test will need a mode-aware `sysPrompt` fixture (or assert both auto and forced variants).

### 1.7 `docs/how-it-works.md`

- The decompose narrative (lines 50–130) currently says:
  - Line 59 (roles table): planner output is `JSON {count, single, commits:[...], message?}` — needs `files` added to the contract sketch.
  - Line 111 ("Start-of-run freeze") and line 115 ("One-file short-circuit") — accurate, unchanged.
  - Line ~127 ("Arbiter leftover reconciliation"): "if `git status --porcelain` shows remaining changes" — **this is the Phase-1 arbiter-freeze change**, not Phase 2; flag but do not edit here.
  - **Missing**: any mention of the soft target, the auto-vs-forced rules distinction, or the `files` partition contract. The Phase-2 Mode-A edit adds a short paragraph (ride with T2).

---

## 2. TARGET state per FR-M3 / FR-M3b / FR-M4 / §17.5 / §17.6

### 2.1 `PlannerCommit.Files` (FR-M3)

`internal/prompt/planner.go` — add the field to the existing struct:

```go
type PlannerCommit struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Files       []string `json:"files"` // FR-M3: every path this concept touches; guidance, not a constraint (FR-M1c is the content guarantee)
}
```

- `ParsePlannerOutput` populates it automatically via `json.Unmarshal` — **no parse-code change**, just the field. Tolerates `"files":null` (→ nil) and absence (→ nil), matching the existing `commits:null` tolerance.
- `validatePlannerOutput` must **NOT** enforce non-empty `Files` — files is guidance (FR-M1c is the sole content guarantee). Leave validation unchanged.

### 2.2 Mode-conditional planner system prompt (FR-M4 soft target + §17.5 rewrite)

The current single `plannerSystemPrompt` const must be **split into shared + swappable pieces** so the builder can emit exactly one rules block. Per PRD §17.5 (line 1762): "the opener, the 'UNSTAGED' framing line, and the JSON contract are shared; only the `Rules:` block changes."

#### SHARED (emitted in both modes — quote these verbatim from PRD §17.5)

**Opener** (PRD §17.5 system-prompt sketch, lines 1766–1768):
```
You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.
```

**UNSTAGED framing line** (PRD §17.5, line 1770):
```
These changes were left UNSTAGED on purpose and handed to you to organize — finding the real
commit boundaries is the job you were asked to do, not a fallback to resist.
```

**JSON contract + trailing clauses** (PRD §17.5, lines 1789–1794) — note `files` is now in the contract:
```
Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- "files" must list every path this commit touches; "description" must say, per file, WHICH
  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or
  line numbers.
```

**Style/format tail** — unchanged mechanism: `BuildPlannerSystemPrompt` appends `\n\n` + (auto: `"---\n"+ex+"\n"` per example) or (non-auto: `formatScaffoldBody(format)`), then `withLocale(...)`.

#### SWAPPING — Auto-decompose `Rules:` block (PRD §17.5, lines 1772–1780)

Verbatim:
```
Rules:
- Split changes that serve DIFFERENT purposes into separate commits. Two changes you would
  describe with different verbs, or explain to a reviewer in separate sentences, almost always
  belong in separate commits. When torn between one commit and several, lean toward SEVERAL.
- Do not manufacture tiny commits. Group changes that only make sense together (a function plus
  its test, a refactor plus the callers it updates). A single commit is correct only when the
  whole changeset pursues ONE purpose.
- Keep the count modest: in ordinary cases at or below 6 (half the max of 12). Only exceed that
  when the changes genuinely span many unrelated concerns; do not approach the max casually.
- Account for every changed path: each file in the diff should appear in some commit's "files".
  A single file may be split across two concepts — name it in both and say, per file, WHICH
  part belongs here.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.
```

The `6` and `12` in the third bullet ("at or below 6 (half the max of 12)") are the **interpolated soft target** — derived as `max_commits/2` and `max_commits` respectively (PRD §17.5 line 1812: "interpolated from `max_commits` at build time (default 12 → '6')").

#### SWAPPING — Forced-count `Rules:` block (PRD §17.5, lines 1798–1805)

Verbatim (PRD §17.5 line 1796: "swaps ONLY the `Rules:` block above for this one — the opener, framing line, and JSON contract are unchanged"):
```
Rules:
- You MUST partition into EXACTLY the requested number of commits. Do not return more or fewer,
  and do not reconsider the count.
- Split changes that serve DIFFERENT purposes into separate commits; group changes that only
  make sense together (a function plus its test, a refactor plus its callers).
- Account for every changed path (each file in the diff in some commit's "files"); name it in
  both if a single file is split across two concepts, and say WHICH part per file.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.
```

The forced-count block has **no soft-target line** (the count is fixed by the user).

#### `BuildPlannerSystemPrompt` signature change

Current: `BuildPlannerSystemPrompt(examples []string, format, locale string) string`

Target — needs **mode** (to pick the rules block) and **maxCommits** (to interpolate the soft target). Two viable shapes; the second is preferred because it keeps the auto/forced distinction explicit and matches the existing `forcedCount int` idiom in `BuildPlannerUserPayload`:

```go
// Preferred: mirror BuildPlannerUserPayload's forcedCount idiom.
func BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string
```

- `forcedCount > 0` ⇒ emit the **forced-count** rules block.
- `forcedCount <= 0` ⇒ emit the **auto** rules block, with the soft-target line interpolated as `maxCommits/2` and `maxCommits`.
- Soft-target interpolation: `fmt.Sprintf` the third auto bullet with `%d` for both numbers, e.g. `"Keep the count modest: in ordinary cases at or below %d (half the max of %d). Only exceed that"`. With `maxCommits=12` ⇒ `6`/`12` (matches PRD default). Integer division `maxCommits/2` is correct (12/2=6, 10/2=5, etc.).

Callers to update:
- `internal/decompose/planner.go:73` — `prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale, forcedCount, deps.Config.MaxCommits)`.
- `internal/prompt/reserve.go:89` `PlannerReserveTokens` takes `sysPrompt` already-built, so its signature is unchanged — but its **callers** (and the test fixture) must pass a mode-appropriate sysPrompt. `internal/decompose/planner.go:74` already builds sysPrompt before calling reserve; just ensure the new args flow through.
- `internal/prompt/reserve_test.go:103–104` — `r0 := PlannerReserveTokens(sysPrompt, 0, "", est)` etc. The `sysPrompt` fixture must be rebuilt with the new builder signature.

#### Topology (exact, mirroring today's `BuildPlannerSystemPrompt`)

```
plannerOpener                       // "…partition them into logical units." (no trailing \n)
'\n' '\n'                           // blank line
plannerUnstagedFraming              // "…not a fallback to resist." (no trailing \n)
'\n' '\n'                           // blank line
<rulesBlock>                        // auto (with interpolated soft target) OR forced-count (no trailing \n)
'\n' '\n'                           // blank line
plannerJSONContract                 // the "Respond with ONLY JSON…" block incl. the two trailing clauses (no trailing \n)
'\n' '\n'                           // blank line before examples/scaffold
auto: for each ex: "---\n" + ex + '\n'
non-auto: formatScaffoldBody(format)
<withLocale>
```

(Confirm the exact blank-line count against PRD §17.5's sketch — the sketch shows one blank line between each major section. The existing test `TestBuildPlannerSystemPrompt_CanonicalExact` is the regression net: rewrite its `want` to the new topology.)

### 2.3 FR-M3b coverage check (deterministic, non-fatal)

**Where it lives:** `internal/decompose/decompose.go`, immediately after `callPlanner` returns successfully and **before** the `if out.Single` short-circuit — i.e. between current lines ~199 and ~205. Reason: the check applies to **partitioned** output (`out.Commits`); when `out.Single==true` there is exactly one concept and the leftover/arbiter path is the single-shortcut, so the check is moot (but harmless to run; simplest is to run it whenever `out.Commits` is non-empty, regardless of Single).

**Algorithm** (PRD §4.2 FR-M3b + PRD §17.5 line 1812):
```go
// FR-M3b: deterministic coverage check. Union concept.Files; compare to the frozen
// changed-path set (DiffTreeNames(baseTree, tStart) — already the run's frozen baseline).
// Any unclaimed path → VerboseRawOutput log as a likely leftover (the arbiter reconciles it).
// NEVER abort, NEVER hard-constrain the stager (FR-M1c stays the sole content guarantee).
if !out.Single && len(out.Commits) > 0 {
    claimed := make(map[string]struct{})
    for _, c := range out.Commits {
        for _, f := range c.Files {
            claimed[f] = struct{}{}
        }
    }
    changed, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
    if err != nil {
        // Best-effort: log and continue (do NOT fail the run on a read error).
        deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: coverage check skipped: %v", err))
    } else {
        for _, p := range changed {
            if _, ok := claimed[p]; !ok {
                deps.Verbose.VerboseRawOutput(fmt.Sprintf(
                    "decompose: path %q not claimed by any concept (likely leftover for the arbiter)", p))
            }
        }
    }
}
```

- **Logs, never errors.** Uses `deps.Verbose.VerboseRawOutput(...)` (already the FR50 raw-output channel; nil-safe per `internal/ui/verbose.go:54`).
- **Not a hard constraint.** Does not influence `runLoop` or the stager; FR-M1c (`verifyFreezeSubset` in `runLoop`) remains the sole content guarantee.
- The `DiffTreeNames(baseTree, tStart)` call duplicates the line-182 computation but at a different control-flow point (line 182 is inside the one-file short-circuit guard; the coverage check is in the post-planner path). Compute it fresh — it is cheap (name-only) and avoids coupling the two branches.
- Consider extracting a small helper `checkPlannerCoverage(deps, baseTree, tStart, concepts)` in `decompose.go` for testability.

### 2.4 Stager `files` block + guardrails wording (§17.6)

`internal/prompt/stager.go`:

**Signature change:**
```go
func BuildStagerTask(title, description string, files []string) string
```

**New topology** (PRD §17.6 task-prompt sketch, lines 1820–1832):
```
stagerInstruction                           // "Stage, but do NOT commit, … match this concept:"
'\n' '\n'
title
'\n'
description
['\n' '\n'                                   // the files block, ONLY when len(files) > 0
 "Files for this concept (where these changes live):\n"
 <one path per line, joined by '\n'>]
'\n' '\n'
stagerGuardrails                            // updated wording (below)
```

- The `Files for this concept (where these changes live):` block is **omitted entirely** when `len(files)==0` (PRD §17.6 line 1818: "an empty list simply omits the files block"). No blank-line artifact.
- Paths rendered one per line (joined `"\n"`). Defensive: nil/empty → omit.

**`stagerGuardrails` wording update** — the second sentence of the block changes. Today (lines 38–39):
```
only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this
concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not
```
Target (PRD §17.6 line 1826):
```
only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description
assigns to this concept (the files above are where they live); leave everything else unstaged.
Do not commit, do not amend, do not push, do not modify file contents — only update the index.
When done, reply with the list of paths you staged and stop.
```
The em-dash "—" (U+2014) in "file contents — only update the index" is preserved (PRD §17.6 fidelity — the existing comment at stager.go:34 already flags this byte). The two backtick commands (`git add <path>`, `git apply --cached`) are unchanged.

**Caller update:** `internal/decompose/stager.go:99`:
```go
task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
```

### 2.5 Mode-A doc edit (`docs/how-it-works.md`, rides with T2)

Add a short paragraph in the "Key design points" section (after "One-file short-circuit", ~line 115) covering: the planner's rules block is mode-conditional (auto leans toward SEVERAL with a soft `max_commits/2` target; forced-count fixes the count); every concept carries a `files` list naming the paths it touches, and a deterministic coverage check logs (not errors) any path no concept claimed — the arbiter reconciles leftovers. Update the roles table (line 59) planner-output cell to `JSON {count, single, commits:[{title,description,files}], message?}`.

---

## 3. Exact test assertions to add / update

### 3.1 `internal/prompt/planner_test.go`

1. **Rewrite `TestBuildPlannerSystemPrompt_CanonicalExact`** — new `want` constant with the full auto-mode topology (opener + UNSTAGED framing + auto rules with `6`/`12` + JSON-contract-with-files + examples). Call `BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)`.
2. **Add `TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact`** — pins the forced-count rules block; call `BuildPlannerSystemPrompt(examples, "auto", "", 3, 12)`. Assert it contains `"You MUST partition into EXACTLY"` and does NOT contain the soft-target line `"Keep the count modest"`. Assert the opener, UNSTAGED framing, and JSON contract are **identical** to the auto variant (byte-compare the shared prefix).
3. **Add `TestBuildPlannerSystemPrompt_SoftTarget_Interpolation`** — table: `maxCommits ∈ {12, 10, 20, 4}` ⇒ soft target `∈ {6, 5, 10, 2}`. Assert the auto prompt contains `"at or below %d (half the max of %d)"` with the right numbers; assert forced-count prompt contains neither number (no soft-target line).
4. **Update `TestBuildPlannerSystemPrompt_Properties`** — drop the `"Prefer FEWER commits"` PRESENT assertion; add `"lean toward SEVERAL"` PRESENT (auto) and `"MUST partition into EXACTLY"` PRESENT-only-in-forced. Add `"files": ["<path>"` PRESENT in the JSON contract line. Keep `"Target ~"` ABSENT.
5. **Update `TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact`** — the shared-prefix constant name changes (split out of `plannerSystemPrompt`); rebuild the `want` strings via the new builder with `forcedCount=0, maxCommits=12`. The scaffold/locale behavior is unchanged.
6. **Update `TestParsePlannerOutput`** — add `Files` to the round-trip cases: e.g. `{"title":"A","description":"d1","files":["a.go","b.go"]}` ⇒ `PlannerCommit{...,Files:[]string{"a.go","b.go"}}`. Add a case for `"files":null` (→ nil) and a case for a concept missing the `files` key (→ nil). Update the comparison loop to check `c.Files`.
7. **Update `TestParsePlannerOutput_RoundTrip`** — add `Files: []string{"x","y"}` to one commit in `original`; assert it survives.

### 3.2 `internal/prompt/stager_test.go`

1. **Update `TestBuildStagerTask_CanonicalExact`** — add a `files` arg and the new `Files for this concept` block + new guardrails wording to `want`. Provide two cases: (a) `files=[]string{"a.go","b.go"}` → block present; (b) `files=nil` → block absent (byte-identity to the no-files rendering, minus the block).
2. **Add `TestBuildStagerTask_FilesBlock_OmittedWhenEmpty`** — `BuildStagerTask(t, d, nil)` and `BuildStagerTask(t, d, []string{})` must NOT contain `"Files for this concept"`.
3. **Update `TestBuildStagerTask_Properties`** — replace the `"Stage ONLY changes belonging to this\nconcept"` needle with the new wording `"Stage ONLY the changes the description\nassigns to this concept (the files above are where they live)"`. Add `"Files for this concept (where these changes live):"` PRESENT (with files) and the per-path rendering. Keep the em-dash, backtick-command, and anti-copy-paste assertions.
4. **Update edge-case tests** (`TestBuildStagerTask_EdgeCases`) to pass the new `files` arg (nil is fine).

### 3.3 `internal/decompose/planner_test.go`

1. **Add `TestCallPlanner_CoverageCheck_LogsNotErrors`** — drive `callPlanner` (auto, `forcedCount=0`) with a stub planner JSON whose concepts' `files` deliberately omit one of the changed paths; assert the run **succeeds** (no error) AND `deps.Verbose` (a capturing writer) received a line naming the unclaimed path. Use the existing `plannerDeps`/`freezeForPlanner` helpers.
   - NOTE: the coverage check lives in `decompose.go` (the orchestrator), NOT in `callPlanner`. So this assertion belongs in `internal/decompose/decompose_test.go`, driving `Decompose(...)` (or a thin extracted helper) — confirm the seam during implementation. If `Decompose` is too heavy, extract `checkPlannerCoverage` and unit-test it directly with a fake `Deps.Verbose`.
2. Existing `TestCallPlanner_SafetyCap_Auto` / `_ForcedSkips` — **unchanged**, green net for the hard cap.

### 3.4 `internal/decompose/stager_test.go` (and `decompose_test.go` stager seam)

- The `deps.stager` test seam (`internal/decompose/roles.go:73`) signature is `func(ctx, Deps, prompt.PlannerCommit) error` — it takes the whole `PlannerCommit`, so it is **transparent** to the `BuildStagerTask` signature change. No test change needed at the seam level.
- If any test calls `BuildStagerTask` directly (grep `internal/decompose/` for it — currently only `stager.go:99`), update the arg.

### 3.5 `internal/prompt/reserve_test.go`

- `TestPlannerReserveTokens` (line ~101) — rebuild the `sysPrompt` fixture via the new builder signature: `sysPrompt := BuildPlannerSystemPrompt(nil, "auto", "", 0, 12)`. The reserve computation itself is unchanged.

---

## 4. Risks / gotchas

1. **Soft-target integer math.** `maxCommits/2` in Go is integer division — correct for even defaults (12→6), and intentional for odd values (e.g. `max_commits=10` → soft target 5, hard cap 10). PRD §17.5 line 1812 only specifies the default-12→6 case; odd `max_commits` is unspecified but integer division is the only sane choice. Document it in the builder's doc comment.
2. **`Files` is guidance, not a constraint (FR-M1c).** Do NOT add `Files` enforcement to `validatePlannerOutput`, and do NOT let the coverage check influence `runLoop` or the stager. The stager may stage paths not in `Files` (or omit paths in `Files`) — `verifyFreezeSubset` in `runLoop` (decompose.go:401) is the sole content guarantee. A concept with `files:[]` or `files:null` must parse and flow through without error.
3. **Shared-prefix constant naming.** Today `plannerSystemPrompt` is one const and is referenced by name in `planner_test.go` (e.g. `TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact` builds `want` as `plannerSystemPrompt + "\n\n" + scaffold`). Splitting it into `plannerOpener` + `plannerUnstagedFraming` + `plannerAutoRules` / `plannerForcedRules` + `plannerJSONContract` will break those references — update them. Consider keeping a private `plannerSharedPrefix` const (= opener + framing) to minimize the diff in the format-mode test.
4. **Coverage check `DiffTreeNames` error is non-fatal.** A git read failure mid-check must NOT fail the run (best-effort: log + continue). The commits are still valid; the arbiter will reconcile regardless.
5. **`VerboseRawOutput` nil-safety.** `deps.Verbose` is `*ui.Verbose`; `VerboseRawOutput` is nil-receiver-safe (`internal/ui/verbose.go:54` guards `v==nil || v.w==nil || !v.on`). No extra nil check needed, but the coverage check should still be guarded by `!out.Single` to avoid noise on the single-shortcut path.
6. **Forced-count mode + soft target.** The forced-count rules block has NO soft-target line. Do not interpolate `maxCommits` into the forced block. The builder branches on `forcedCount > 0` (mirroring `BuildPlannerUserPayload`).
7. **Stager `files` block ordering.** The block goes **between description and guardrails** (PRD §17.6 sketch). Existing test `TestBuildStagerTask_Properties` asserts description is "between title and guardrails" — the files block sits in that same gap; update the ordering assertion to allow the files block (or assert title < description < files-block < guardrails).
8. **`callPlanner` signature unchanged.** The mode/target info reaches `callPlanner` already (via `forcedCount` arg + `deps.Config.MaxCommits`). The only call-site change inside `callPlanner` is the two extra args to `BuildPlannerSystemPrompt` at line 73.
9. **Mode-A docs are a ride-along, not a separate task.** The `docs/how-it-works.md` edit lands with T2 (per delta_prd.md §4.2 / §5 Phase-2 T2). Keep it tight — one paragraph + the roles-table cell.
10. **No new config keys / CLI flags.** FR-M4's soft target is **derived** from existing `max_commits`; FR-M3's `files` is automatic in the planner JSON. Neither adds user-facing surface (delta_prd.md §3 non-goals). Do not touch `internal/config/`, `cmd/stagehand/`, or `docs/cli.md`/`docs/configuration.md`.

---

## 5. Files likely to change (summary)

| File | Change |
|---|---|
| `internal/prompt/planner.go` | Split `plannerSystemPrompt` into shared + auto/forced rules; add `files` to JSON contract; add `Files []string` to `PlannerCommit`; change `BuildPlannerSystemPrompt` signature (+`forcedCount, maxCommits int`); interpolate soft target. |
| `internal/prompt/stager.go` | Add `files []string` param to `BuildStagerTask`; render `Files for this concept` block (omit when empty); update `stagerGuardrails` wording. |
| `internal/decompose/planner.go` | Update `BuildPlannerSystemPrompt` call (line 73) with new args. (Reserve call at line 74 unchanged.) |
| `internal/decompose/decompose.go` | Add FR-M3b coverage check after `callPlanner` returns (~line 200), before `if out.Single`. |
| `internal/decompose/stager.go` | Update `BuildStagerTask` call (line 99) to pass `concept.Files`. |
| `internal/prompt/planner_test.go` | Rewrite canonical-exact; add forced-count + soft-target tests; update properties + parse round-trip for `Files`. |
| `internal/prompt/stager_test.go` | Rewrite canonical-exact; add files-block-omitted test; update properties for new guardrails wording. |
| `internal/prompt/reserve_test.go` | Rebuild `sysPrompt` fixture with new builder signature. |
| `internal/decompose/decompose_test.go` (or `planner_test.go`) | Add coverage-check-logs-not-errors test. |
| `docs/how-it-works.md` | Mode-A ride-along: roles-table cell + one paragraph on mode-conditional rules / soft target / `files` / coverage check. |

---

## 6. Start here

**`internal/prompt/planner.go`** — open it first. The `plannerSystemPrompt` const (lines 29–49) and `PlannerCommit` struct (lines 57–61) are the two load-bearing edits; the `BuildPlannerSystemPrompt` signature (line 113) is the seam that everything else threads through. Once the const is split and the signature is settled, `stager.go` and the three decompose call-sites (`planner.go:73`, `decompose.go:~200`, `stager.go:99`) fall out mechanically, and the tests are a rewrite of the existing canonical-exact fixtures plus the new coverage/soft-target cases.

Quotable source of truth for the rules blocks: **`PRD.md` §17.5 lines 1766–1805** (auto sketch 1766–1788; forced-count swap 1798–1805) and the interpolation note at **line 1812**. The delta spec is **`plan/008_82253c999440/delta_prd.md` §4.2** (FR-M3 / FR-M3b / FR-M4 / §17.5 / §17.6 / tests).
