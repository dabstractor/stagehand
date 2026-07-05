# P2.M1.T2.S1 — Research findings (planner prompt split + soft target + mode-conditional rules)

Scout brief for the implementing agent. All line numbers verified against the current tree by reading,
not assumed. Source of truth for the verbatim rules text: `PRD.md` §17.5 (in `selected_prd_content`) and
`plan/008_82253c999440/docs/architecture/planner_prompt.md` §2.2 (which QUOTES the PRD).

---

## §1 — The ONE production call-site + scope fence

- `grep -rn BuildPlannerSystemPrompt --include=*.go` (excl. tests) ⇒ **exactly one** production caller:
  `internal/decompose/planner.go:73`:
  ```go
  sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)
  ```
  → becomes `prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale, forcedCount, deps.Config.MaxCommits)`.
  `forcedCount` is ALREADY a `callPlanner` param; `deps.Config.MaxCommits` is in scope. NO new plumbing.
- `reserve := prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context, git.EstimateTokens)`
  at `decompose/planner.go:74` ⇒ **signature unchanged** (takes the already-built sysPrompt). It just
  receives a longer/different string now. No edit to that line.
- The **hard cap** at `internal/decompose/planner.go:132` (`if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits`)
  is **UNCHANGED** — soft target is guidance-only and never errors.

## §2 — `decompose/planner_test.go` is OUT OF SCOPE (no builder call)

- `grep BuildPlannerSystemPrompt internal/decompose/planner_test.go` ⇒ the ONLY occurrence is a COMMENT
  at line 375 (`// BuildPlannerSystemPrompt(nil) is nil-safe — should not panic`). Every test calls
  `callPlanner(ctx, deps, forcedCount, isUnborn, baseTree, tStart)` — and **callPlanner's signature is
  unchanged**. So `decompose/planner_test.go` compiles & stays green with **zero edits**. The existing
  `TestCallPlanner_SafetyCap_Auto` / `_ForcedSkips` (lines ~283, ~327) are the regression net for the
  FR-M4 hard cap.
- ⇒ The ONLY test files in scope: `internal/prompt/planner_test.go` + `internal/prompt/reserve_test.go`.

## §3 — `reserve_test.go` uses a SENTINEL, not the builder (important correction to the work item)

- `internal/prompt/reserve_test.go` `TestPlannerReserveTokens` uses `const sysPrompt = "P"` — a 1-rune
  sentinel, NOT `BuildPlannerSystemPrompt(...)` output. It tests the reserve FORMULA in isolation.
- `PlannerReserveTokens(sysPrompt string, forcedCount int, context string, est TokenEstimator) int`
  (`internal/prompt/reserve.go:89`) — **signature unchanged**. So `TestPlannerReserveTokens` compiles &
  passes UNCHANGED. The work item's "reserve_test.go fixtures must rebuild sysPrompt via the new builder"
  is based on a stale assumption; the minimal correct change is **no edit**.
- RECOMMENDED (honors the work item's intent + adds real value): ADD a focused test that threads the NEW
  mode-conditional builder output through `PlannerReserveTokens` — build `autoSys := BuildPlannerSystemPrompt
  (nil, "auto", "", 0, 12)` and `forcedSys := BuildPlannerSystemPrompt(nil, "auto", "", 3, 12)`, assert both
  yield a positive reserve and (sanity) forcedSys ≥ autoSys token-wise is NOT required to assert — just
  assert both > 0 and that the formula holds (`est(sys)+est(overhead)+reserveSafetyMargin`). This is the
  ONLY place the new builder output is exercised end-to-end through reserve.

## §4 — The ASCII-only gotcha (the #1 byte-fidelity trap)

- The existing comment at `internal/prompt/planner.go:27` mandates: "§17.5 is ENTIRELY ASCII — no em-dash,
  no non-ASCII bytes." (This refers to the CONST STRING VALUES sent to the model — the comments themselves
  freely use § (U+00A7) and — (U+2014), which is fine; only the prompt bytes must be ASCII.)
- The NEW PRD §17.5 text contains **two em-dashes (U+2014)** in the prompt body:
  1. The framing line: "...handed to you to organize **—** finding the real commit boundaries..."
  2. The auto-rules block: "A single file may be split across two concepts **—** name it in both..."
  (The forced-rules block uses a COMMA instead — "...split across two concepts, and say WHICH..." — NO
  em-dash. The opener and JSON contract have NO em-dashes.)
- The work item says "QUOTE THE EXACT RULES TEXT ... verbatim — ASCII only." Resolution (binding): in the
  CONST VALUES, substitute the em-dash with the standard ASCII rendering **" -- " (space hyphen hyphen
  space)**. This is unambiguous, ASCII, and a well-known em-dash stand-in. Keep all OTHER bytes verbatim.
  The existing ASCII-only comment then stays TRUE (no edit needed; optionally tighten its wording).
- **Tests must use the SAME substitution** in their `want` constants or they will fail byte-identity.
  This is why every `want` string in the PRP is given verbatim below — copy/paste, do not retype.

## §5 — Exact ASCII const values (copy/paste — these ARE the bytes)

### `plannerOpener` (verbatim PRD §17.5 lines 1766–1768; ASCII; no trailing newline)
```
You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.
```

### `plannerUnstagedFraming` (PRD §17.5 line 1770; em-dash → " -- "; ASCII; no trailing newline)
```
These changes were left UNSTAGED on purpose and handed to you to organize -- finding the real
commit boundaries is the job you were asked to do, not a fallback to resist.
```

### `plannerJSONContract` (PRD §17.5 lines 1789–1794; NOW includes "files"; ASCII; no trailing newline)
```
Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- "files" must list every path this commit touches; "description" must say, per file, WHICH
  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or
  line numbers.
```

### `plannerAutoRules` (PRD §17.5 lines 1772–1780; em-dash → " -- "; soft-target "6"/"12" → "%d"/"%d";
### ASCII; Sprintf args = (maxCommits/2, maxCommits); no trailing newline)
```
Rules:
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
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.
```

### `plannerForcedRules` (PRD §17.5 lines 1798–1805; NO em-dash, NO soft-target; ASCII; no trailing newline)
```
Rules:
- You MUST partition into EXACTLY the requested number of commits. Do not return more or fewer,
  and do not reconsider the count.
- Split changes that serve DIFFERENT purposes into separate commits; group changes that only
  make sense together (a function plus its test, a refactor plus the callers).
- Account for every changed path (each file in the diff in some commit's "files"); name it in
  both if a single file is split across two concepts, and say WHICH part per file.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.
```

## §6 — Soft-target interpolation (the math)

- Auto rules 3rd bullet: `"at or below %d (half the max of %d)"` ⇒ `fmt.Sprintf(plannerAutoRules,
  maxCommits/2, maxCommits)`. Go INTEGER division. Table (from the work item):
  | maxCommits | maxCommits/2 | rendered line |
  |---|---|---|
  | 12 (default) | 6 | "at or below 6 (half the max of 12)" |
  | 10 | 5 | "at or below 5 (half the max of 10)" |
  | 20 | 10 | "at or below 10 (half the max of 20)" |
  | 4 | 2 | "at or below 2 (half the max of 4)" |
- Odd maxCommits (e.g. 11 → 5) is unspecified by PRD but integer division is the only sane choice;
  document it in the builder doc comment. The hard cap (`planner.go:132`) is the real guard; soft
  target is guidance text only — NEVER errors.
- `maxCommits <= 0`: config layer guarantees 12 default (non-zero-wins overlay, `file.go:397`); the
  builder does NOT clamp — it interpolates verbatim (maxCommits=0 → "0"/"0", cosmetic). Do not add a
  clamp (out of scope; the hard cap guards runtime).

## §7 — Builder topology (exact; mirrors today's blank-line discipline)

```
plannerOpener                 // no trailing \n
"\n\n"                        // one blank line
plannerUnstagedFraming        // no trailing \n
"\n\n"
<rules>                       // forcedCount>0 ⇒ plannerForcedRules (verbatim);
                              //   else fmt.Sprintf(plannerAutoRules, maxCommits/2, maxCommits)
"\n\n"
plannerJSONContract           // no trailing \n
"\n\n"
format=="auto": for each ex: "---\n" + ex + "\n"
format!="auto": formatScaffoldBody(format)   // "" for plain
<withLocale(b.String(), locale)>
```
- `withLocale` (`internal/prompt/format.go:40`): locale=="" ⇒ returns s unchanged; else
  `strings.TrimRight(s,"\n") + "\nWrite the commit message in <locale>."`. Because the auto/scaffold
  tail always ends in "\n" (one per example) or "" (plain scaffold ⇒ the buffer ends with the "\n\n"
  after plannerJSONContract), `withLocale`'s TrimRight normalizes to a single "\n" separator before the
  locale line. This is why the existing "plain, locale ja" FormatModes case wants
  `plannerShared + "\nWrite the commit message in ja."` (TrimRight eats one "\n" of the trailing "\n\n").

## §8 — Shared-prefix identity between auto and forced (the byte-identity contract)

- opener + framing is a TRUE contiguous shared prefix (first bytes of both modes).
- plannerJSONContract is byte-identical in both modes BUT is NOT contiguous with the prefix (the rules
  block sits between them and DIFFERS). So "shared prefix" = opener+framing (contiguous) + the JSON
  contract block (identical substring, non-contiguous). The `ForcedCount_CanonicalExact` test asserts:
  (a) `strings.HasPrefix(forcedPrompt, autoPrompt[:len(opener+"\n\n"+framing)])` — i.e. the opener+framing
      prefix is byte-identical; AND
  (b) both prompts contain the IDENTICAL `plannerJSONContract` substring; AND
  (c) forced contains `"You MUST partition into EXACTLY"` and does NOT contain `"Keep the count modest"`.

## §9 — Test inventory (current → target) in `internal/prompt/planner_test.go`

| Test (current) | Action | Why |
|---|---|---|
| `TestBuildPlannerSystemPrompt_CanonicalExact` (L14) | **REWRITE** | opener/rules/contract all change + 2 new args. New `want` = full auto-mode topology with soft target 6/12. Call `BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)`. |
| `TestBuildPlannerSystemPrompt_Properties` (L61) | **UPDATE** | drop `"Prefer FEWER commits"` PRESENT; add `"lean toward SEVERAL"` PRESENT (auto), `"MUST partition into EXACTLY"` ABSENT-in-auto, `"files": ["<path>"` PRESENT (contract). Keep `"Target ~"` ABSENT. |
| `TestBuildPlannerSystemPrompt_EmptyExamples` (L114ish) | **UPDATE call-site only** | add `, 0, 12` args; assertions (`You are a commit-planning assistant.`, no `---`) still hold. NOTE: the "find the exact changes." assertion MUST change to the new contract's last line (`"line numbers."`). |
| `TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact` (L113) | **REWRITE want** | `plannerSystemPrompt` const is GONE. Reconstruct `sharedAuto := plannerOpener+"\n\n"+plannerUnstagedFraming+"\n\n"+fmt.Sprintf(plannerAutoRules,6,12)+"\n\n"+plannerJSONContract` once, reuse for all cases: `want = sharedAuto + "\n\n" + <scaffold>` (+ locale via withLocale). Pass new args `(examples, tc.format, tc.locale, 0, 12)`. |
| `TestBuildPlannerSystemPrompt_FormatModes_Properties` (L159) | **UPDATE call-sites** | add `, 0, 12` to both builder calls; assertions (role line, JSON contract, no `---` non-auto, no example leak, auto still has `---\nfeat: a`) still hold. |
| (NEW) `TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact` | **ADD** | §8 assertions (a)(b)(c). |
| (NEW) `TestBuildPlannerSystemPrompt_SoftTarget_Interpolation` | **ADD** | table maxCommits∈{12,10,20,4}→{6,5,10,2}; assert auto prompt contains the right `"at or below %d (half the max of %d)"`; assert forced prompt contains NEITHER number. |
| `TestBuildPlannerUserPayload_*`, `TestPlannerRetryInstruction`, `TestParsePlannerOutput*`, `TestExtractJSONObject` | **UNCHANGED** | these do not touch the system-prompt builder. (ParsePlannerOutput already has Files cases from S1.) |

## §10 — Files in scope (exactly 4) vs. fence (do NOT touch)

IN SCOPE (4 files, all EDIT):
1. `internal/prompt/planner.go` — split const + signature + topology.
2. `internal/decompose/planner.go` — ONE line (73) call-site args.
3. `internal/prompt/planner_test.go` — rewrite + 2 new tests + call-site updates.
4. `internal/prompt/reserve_test.go` — ADD one focused builder→reserve threading test (existing sentinel test unchanged).

FENCE (owned elsewhere — do NOT edit):
- `internal/prompt/stager.go` / `stager_test.go` → **P2.M1.T3.S1** (stager files block + guardrails). This
  task does NOT touch the stager at all.
- `internal/decompose/decompose.go` / `decompose_test.go` → **P2.M1.T1.S2** (FR-M3b coverage check),
  running in parallel. Different file; no conflict.
- `internal/decompose/planner_test.go` → callPlanner signature unchanged ⇒ no edit (§2).
- `docs/how-it-works.md` → **P2.M1.T2.S2** (Mode A doc edit). Item §5: NO DOCS here.
- `internal/config/*`, `cmd/stagehand/*`, `docs/cli.md`, `docs/configuration.md` → no new flags/keys.
- `internal/prompt/reserve.go` → `PlannerReserveTokens` signature UNCHANGED.
- `PlannerCommit.Files`, `ParsePlannerOutput` → already done by S1 (COMPLETE); this task only READS the
  struct shape (the JSON contract const now references files, matching S1's field).

## §11 — Validation commands (Go project, verified)

```bash
cd /home/dustin/projects/stagehand
go build ./...                         # must compile (signature change + call-site)
go vet ./...                           # no vet issues
go test ./internal/prompt/... -v       # planner_test + reserve_test (the in-scope packages)
go test ./internal/decompose/... -v    # regression net: callPlanner safety-cap tests stay green
go test ./...                          # whole repo green (esp. decompose planner_test untouched)
```
No go.mod/go.sum change (no new deps). `gofmt`/`goimports` are implicit via `go test`/`go vet` hygiene —
run `gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go internal/decompose/planner.go
internal/prompt/reserve_test.go` before committing.
