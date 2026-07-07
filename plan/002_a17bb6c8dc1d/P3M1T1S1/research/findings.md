# Findings — P3.M1.T1.S1 (planner system prompt + JSON contract + assembly)

Source of truth: PRD §17.5 (prd_snapshot.md lines 1527–1551) + the existing `internal/prompt`
package conventions (system.go, payload.go, system_test.go, payload_test.go) + provider/parse.go.

---

## §1 — The VERBATIM §17.5 system prompt (the `plannerSystemPrompt` constant)

Captured character-for-character from prd_snapshot.md lines 1533–1549 (the code block under
"System prompt (sketch):"). The §17.1 precedent (system.go) treats the PRD §17.x code blocks as the
authoritative verbatim source; §17.5 is "(sketch)" but the work item says "verbatim from §17.5", so the
same rule applies: copy the sketch's text byte-for-byte.

```
You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.

Rules:
- Prefer FEWER commits. A single commit is correct unless the changes clearly span
  unrelated concerns. Do not manufacture tiny commits.
- Each commit must be independently meaningful and reviewable. Group tightly-coupled
  changes (a function + its test, a refactor + its callers) together.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.

Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<precisely which files/hunks belong here, by path>"}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- The "description" must be specific enough that a staging agent can find the exact changes.
```

**CRITICAL DISTINCTION — what is LITERAL vs what is a PLACEHOLDER:**
- The JSON-contract line `{"count": <int>, ...}` is **LITERAL text** shown to the model as the output-
  format spec. Its `<int>`, `<bool>`, `<short concept>`, `<precisely which files/hunks belong here, by
  path>`, `<the full commit message>` tokens are PART OF THE EXAMPLE (they tell the model the shape) —
  they are NOT replaced at runtime. This is exactly analogous to system.go keeping the literal "(up to
  20, ≤100 lines total)" annotation reasoning OFF (that one IS structural), but here the JSON shape
  tokens are part of the instructive format example, so they stay. The whole JSON-contract line goes
  into `plannerSystemPrompt` verbatim.
- `<style examples>` (the LAST line) is the ONLY runtime placeholder. It is replaced at runtime by the
  §17.1-style examples (the RecentMessages-based list). It is therefore NOT part of the constant;
  `BuildPlannerSystemPrompt(examples)` appends the examples there.

So `plannerSystemPrompt` = the text above, ENDING at "...find the exact changes." with NO trailing
newline (mirrors system.go/payload.go rule: constants carry no trailing newline; the Build function
owns inter-block newline placement).

Non-ASCII check: §17.5 is ENTIRELY ASCII — no em-dash (unlike §17.1's anti-reuse block). The `"`, `{`,
`}`, `<>` are all ASCII. Good (one less footgun than S1).

## §2 — The §17.5 user payload + forced-count + retry (the `BuildPlannerUserPayload` + retry const)

From prd_snapshot.md line 1551:
- Normal user payload: `"Decompose these un-staged changes into commits:\n\n<diff>"`.
- Forced-count mode **prepends**: `"Produce EXACTLY N commits from these changes (do not reconsider the count):"`.
- Retry instruction (unparseable JSON): `"Respond with ONLY the JSON object described, no other text."`.

**ASSEMBLY DECISION (the one ambiguous point — pin it with a test):**
The PRD says forced-count "prepends" the directive. It does NOT specify the separator. The cleanest,
most literal reading consistent with payload.go's block-assembly style:
- `forcedCount == 0` (normal): `plannerUserInstruction + "\n\n" + diff`
    where `plannerUserInstruction = "Decompose these un-staged changes into commits:"` (trailing COLON,
    matching §17.3's normal-instruction-ends-with-colon precedent).
- `forcedCount > 0`: `forcedDirective + "\n" + plannerUserInstruction + "\n\n" + diff`
    where `forcedDirective = fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):", forcedCount)`.
  → the two colon-ending instructions on consecutive lines, then ONE blank line, then the diff. This
  preserves the normal payload's `instruction\n\n<diff>` tail topology verbatim.

`forcedCount <= 0` ⇒ normal path (defensive: negative is treated as "not forced", same as 0).

The retry instruction is NOT part of the user payload; it is the user-payload the caller
(decompose/planner.go, P3.M2.T2.S1) sends on the SECOND planner agent call after a parse failure. To
keep the verbatim §17.5 retry text in exactly one auditable place (the prompt package's philosophy), it
is OWNED by planner.go as an exported const `PlannerRetryInstruction` (so the consumer in another
package can use it without hardcoding). P3.M2.T2.S1 wires the retry; THIS task only defines the text.

## §3 — Existing prompt-package conventions (system.go / payload.go) — MUST mirror

Read in full. The load-bearing conventions:
1. **Constants carry NO trailing newline; Build* functions own ALL inter-block newline placement.**
   (system.go comment line 11: "Constants are defined WITHOUT trailing newlines; BuildSystemPrompt
   owns ALL inter-block newline placement so the §17.1 blank-line topology lives in exactly one
   auditable place".) planner.go MUST follow this exactly.
2. **Doc comments are rich**: cite the PRD section, explain the assembly topology in a literal diagram,
   note any non-ASCII bytes, explain WHY signature params are params (not computed inside). Mirror this.
3. **Style examples are formatted**: for each example, `"---\n" + ex + "\n"` (one `---` BEFORE each
   message; examples are pre-trimmed by RecentMessages). system.go BuildSystemPrompt does this inline
   in a `strings.Builder` loop. planner.go MUST replicate this EXACT format for `<style examples>`
   (§17.5: "the §17.1 style examples" — the SAME format).
4. **Exported Build* take minimal, decouled params** (BuildSystemPrompt takes `examples, hasMultiline,
   subjectTarget`; it does NOT take a config). planner.go: `BuildPlannerSystemPrompt(examples []string)`
   — NO hasMultiline (§17.5 has NO multi-line rule; it has its own rules + "NEVER reuse wording" inline,
   NOT §17.1's anti-reuse block) and NO subjectTarget (§17.5 has NO subject-target line).
5. **Anti-copy-paste discipline** (the #1 risk per S2's test): the planner prompt MUST NOT leak §17.1's
   mature-prompt elements ("You are a commit message generator", the raw-output contract, the anti-reuse
   block, "Target ~N characters"). §17.5 is a SEPARATE, self-contained prompt. A test pins this.

## §4 — PlannerOutput struct (the JSON contract)

From §17.5 + the work item. json tags map 1:1 to the §17.5 contract:
```go
type PlannerCommit struct {
	Title       string `json:"title"`       // "<short concept>"
	Description string `json:"description"` // "<precisely which files/hunks belong here, by path>"
}

type PlannerOutput struct {
	Count   int             `json:"count"`   // N (== len(Commits); ==1 iff Single)
	Single  bool            `json:"single"`  // true ⇒ single-commit shortcut (§13.6.4)
	Commits []PlannerCommit `json:"commits"` // the partition (1..N)
	Message string          `json:"message"` // present iff Single==true (the full commit message)
}
```
- `Message` is ALWAYS present on the struct (zero-value `""` when Single==false). The "present iff
  Single==true" rule is a CALLER contract (the orchestrator checks `out.Single && out.Message != ""`),
  NOT enforced by the struct — json.Unmarshal of a payload that omits "message" simply leaves it "".
  This is the right design: a struct field can't be "conditionally present", and the caller (P3.M2.T2.S1
  / §13.6.4) already owns the single-shortcut decision.
- `Commits` may be `null` in malformed output → unmarshals to `nil` slice (len 0); defensive, no panic.

## §5 — ParsePlannerOutput + the JSON-extraction decision (REUSE THE PATTERN, don't import provider)

provider/parse.go has the EXACT algorithm needed:
- `extractJSONObject(s) (string, bool)` — finds first `{`, scans to the matching `}` at depth 0,
  string/escape-aware (handles `{`/`}`/`"` inside JSON string values, and `\"` escapes). ~30 lines.
- `parseJSON(s, field)` — whole-string json.Unmarshal; on failure, extractJSONObject + retry.

**Import-cycle check (DONE):** provider does NOT import prompt (`grep` confirmed). So prompt COULD
import provider. BUT that couples the low-level prompt-construction leaf package to the higher-level
provider-pipeline package — a layering smell. The prompt package currently has ZERO internal deps
(system.go imports only fmt+strings; payload.go imports only strings). 

**DECISION: reimplement a private `extractJSONObject` in prompt/planner.go** (copy provider's algorithm
verbatim). This (a) matches the work-item wording "reuse provider.parseJSON PATTERN or
provider.extractJSONObject" — "pattern" = same algorithm; (b) keeps prompt a zero-internal-dep leaf;
(c) is fully unit-testable in isolation; (d) does NOT touch provider (out of scope). The duplication is
~30 lines of a pure, well-understood function — acceptable and explicitly sanctioned by the work item.

ParsePlannerOutput algorithm (mirrors provider.parseJSON's two-attempt shape):
1. `s := strings.TrimSpace(raw)`.
2. Attempt 1: `json.Unmarshal([]byte(s), &out)` → success ⇒ return (out, nil).
3. Attempt 2 (fallback): `sub, found := extractJSONObject(s)`; if found, `json.Unmarshal([]byte(sub),
   &out)` → success ⇒ return (out, nil).
4. Both fail ⇒ return `(PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err))` where
   err is the SECOND unmarshal's error (most informative). NOTE: do NOT silently swallow — return a
   non-nil error so the caller (decompose/planner.go) can trigger the one retry (§17.5 + §13.6.6:
   "Planner fails / returns unparseable output: surface the error and exit non-rescue").

## §6 — The consumer (decompose/planner.go, P3.M2.T2.S1) — what it will need

decompose/ does NOT exist yet (confirmed: `ls internal/decompose/` → not found). P3.M2.T2.S1 will:
1. `diff, err := deps.Git.WorkingTreeDiff(ctx, opts)` — P2.M2.T2.S2 (BEING IMPLEMENTED IN PARALLEL;
   its PRP is the contract — `WorkingTreeDiff(ctx, StagedDiffOptions) (string, error)`).
2. `examples := git.RecentMessages(...)` (existing v1 method) + the style examples.
3. `sys := prompt.BuildPlannerSystemPrompt(examples)`.
4. `payload := prompt.BuildPlannerUserPayload(diff, forcedCount)` (forcedCount from `--commits N`).
5. Render (bare mode) + Execute the planner agent → raw stdout.
6. `out, err := prompt.ParsePlannerOutput(raw)`.
7. On err: ONE retry — re-Execute with `payload = prompt.PlannerRetryInstruction` (the §17.5 retry
   text); parse again; if still fails → surface error, exit non-rescue (§13.6.6).
8. On success: if `out.Single` → single-shortcut (§13.6.4) using `out.Message`; else loop over
   `out.Commits` into the stager/message/commit pipeline.

So THIS task's EXPORTED surface (the contract P3.M2.T2.S1 consumes):
- `BuildPlannerSystemPrompt(examples []string) string`
- `BuildPlannerUserPayload(diff string, forcedCount int) string`
- `ParsePlannerOutput(raw string) (PlannerOutput, error)`
- `PlannerRetryInstruction` (const string) — needed for step 7's retry payload.
- `PlannerOutput`, `PlannerCommit` (types) — needed to read the parse result.

## §7 — Test conventions (system_test.go / payload_test.go) — MUST mirror

1. **Canonical-exact test**: independently derive the FULL `want` string from the PRD (not from the
   impl), assert `got == want` with `%q` diff output on failure. (system_test.go
   TestBuildSystemPrompt_CanonicalExact, payload_test.go TestBuildUserPayload_NormalCanonicalExact.)
2. **Properties table**: a slice of `{name, inputs, check func}` asserting structural invariants
   (presence/absence of blocks, ordering, delimiter counts). 
3. **Anti-copy-paste guards**: assert §17.1 mature-prompt elements are ABSENT from the planner prompt
   (the #1 risk). (Mirrors TestBuildFallbackPrompt_Properties's "ABSENT" cases.)
4. **Edge cases**: nil/empty examples (no panic, no `---` lines), empty diff, forcedCount==0 vs >0.
5. **ParsePlannerOutput tests**: whole-JSON success; JSON-in-prose (brace-balanced fallback); JSON in a
   code fence (note: the planner prompt says "no code fences" but models sometimes emit them anyway —
   the brace-balanced fallback handles the ```json\n{...}\n``` case because it finds the first `{`);
   malformed → error; empty → error; `commits: null` → nil slice no panic; missing `message` with
   single:false → Message==""; extra unknown fields → ignored.
6. **Helpers**: `near(s, needle)` and `suffix(s, n)` are ALREADY defined in system_test.go (same
   package). Do NOT redeclare them (compile error). Reuse directly.
7. Package is `prompt` (INTERNAL test file `package prompt`, same as system_test.go — so unexported
   `plannerSystemPrompt`, `plannerUserInstruction`, `extractJSONObject` are visible to tests).

## §8 — Imports / module / lint (verified)

- Module: `github.com/dustin/stagecoach`, go 1.22.
- planner.go imports: `encoding/json`, `fmt`, `strings` (stdlib only — NO new internal dep).
- planner_test.go imports: `encoding/json`, `errors`, `strings`, `testing`.
- Lint (.golangci.yml): errcheck/gosimple/govet/ineffassign/staticcheck/unused. The unexported
  `extractJSONObject` and `plannerSystemPrompt` etc. are USED (by the Build/Parse funcs + tests) so
  `unused` won't flag them. No errcheck concern (no ignored Close()).
- go.mod / go.sum UNCHANGED (stdlib only).

## §9 — Scope boundaries (what NOT to touch — frozen / owned elsewhere)

- `internal/prompt/system.go`, `payload.go` — CONSUMED as the pattern template; UNCHANGED. Do NOT add a
  shared `formatStyleExamples` helper to system.go (would touch a frozen file); replicate the
  `---`-example loop inline in planner.go (3 lines).
- `internal/provider/parse.go` — CONSUMED as the algorithm template; UNCHANGED. Do NOT export its
  `extractJSONObject` (scope creep into provider); reimplement privately in planner.go.
- `internal/git/*` — UNCHANGED (WorkingTreeDiff is P2.M2.T2.S2, parallel; RecentMessages is existing v1).
- `internal/decompose/*` — does NOT exist yet; THIS task does NOT create it (P3.M2.*).
- go.mod / go.sum — UNCHANGED.
- Parallel sibling work items: P3.M1.T1.S2 (stager.go prompt) and P3.M1.T1.S3 (arbiter.go prompt) are
  ALSO creating new files in internal/prompt/. Create planner.go as a NEW standalone file → zero merge
  friction. Do NOT touch any shared test helper (near/suffix are shared across the package test files —
  they live in system_test.go; if S2/S3 also need them they're already there).
