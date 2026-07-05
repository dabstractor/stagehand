# P2.M1.T1.S1 Research Findings — PlannerCommit.Files field + parse round-trip tests

Derived from the live `internal/prompt/planner.go` + `planner_test.go` (plan 008) and the authoritative
scout brief `plan/008_82253c999440/docs/architecture/planner_prompt.md` (§1.1, §2.1, §3.1). This is a
LEAF data-model change: add ONE struct field + update TWO test functions. No parse-code change.

## §0. SCOPE — exactly one field, two test functions, one package

**EDIT `internal/prompt/planner.go`**: add `Files []string \`json:"files"\`` to `PlannerCommit` (lines 57-61).
**EDIT `internal/prompt/planner_test.go`**: update `TestParsePlannerOutput` (table + comparison loop) +
`TestParsePlannerOutput_RoundTrip`.

NOT this task (frozen / other subtasks):
- The `plannerSystemPrompt` const split + mode-conditional rules + soft target → **P2.M1.T2.S1** (T2).
- `BuildStagerTask` files param + guardrails wording → **P2.M1.T3.S1** (T3).
- FR-M3b coverage check (logs unclaimed paths) → **P2.M1.T1.S2** (S2, the sibling in this milestone).
- `docs/how-it-works.md` ride-along → T2.
- `validatePlannerOutput` → lives in `internal/decompose/planner.go:152` (a DIFFERENT package); NOT touched,
  and Files must NOT be validated (guidance only — FR-M1c is the sole content guarantee).

## §1. `ParsePlannerOutput` populates Files FOR FREE (no parse-code change)

`ParsePlannerOutput` (planner.go ~171-195) does a generic `json.Unmarshal([]byte(s), &out)` into a
`PlannerOutput` whose `Commits []PlannerCommit` is decoded generically. Adding `Files []string \`json:"files"\``
to `PlannerCommit` makes `json.Unmarshal` populate it automatically — NO parse-code change, just the field.
This is confirmed by the architecture doc §1.1 + §2.1. The parse already tolerates `"files":null` (→ nil)
and a missing `files` key (→ nil), matching the existing `"commits":null` tolerance.

## §2. THE CRITICAL GOTCHA — adding []string makes PlannerCompile-BREAK the round-trip test

`internal/prompt/planner_test.go:440` (in `TestParsePlannerOutput_RoundTrip`) does a DIRECT struct
comparison:
```go
if out.Commits[i] != original.Commits[i] {
```
A Go struct containing a slice field is **NOT comparable with `==`/`!=`** — the compiler rejects it
("`invalid operation: out.Commits[i] != original.Commits[i] (struct containing []string cannot be
compared)`"). TODAY `PlannerCommit` has only `string` fields (comparable); adding `Files []string`
BREAKS THIS LINE. ⇒ the round-trip test's per-commit comparison MUST be rewritten to
`!reflect.DeepEqual(out.Commits[i], original.Commits[i])`, AND `"reflect"` MUST be added to the test
file's imports (currently only `encoding/json`, `strings`, `testing`).

This is the single most load-bearing implementation detail. Verified: no OTHER direct `PlannerCommit`
struct-`==`/`!=` comparison exists in the repo — `grep -rn "PlannerCommit"` shows production code only
passes it as a parameter / iterates it (stager.go:93, decompose.go:395/474/711, roles.go:74); the other
test hits (`decompose_test.go`) compare `.Subject`/`.SHA` STRING fields on `CommitResult`, not
`PlannerCommit` structs. So planner_test.go:440 is the ONLY compile break.

## §3. The exact field (architecture doc §2.1 — verbatim)

```go
// PlannerCommit is one partitioned concept from the planner (§17.5 JSON contract).
type PlannerCommit struct {
	Title       string   `json:"title"`       // "<short concept>" — a short label for the concept.
	Description string   `json:"description"` // "<precisely which files/hunks belong here, by path>" — staging instructions.
	Files       []string `json:"files"`       // FR-M3: every path this concept touches; guidance, not a constraint (FR-M1c is the content guarantee).
}
```
The doc-comment tail is the contract's mandated wording: "every path this concept touches; guidance,
not a constraint (FR-M1c is the content guarantee)". Preserve it verbatim.

## §4. The exact test changes (architecture doc §3.1 items 6 & 7)

### TestParsePlannerOutput (table + comparison loop)
- The round-trip cases share the `cleanMulti` const (used by 4 cases: clean / prose / code-fenced /
  whitespace). The cleanest, lowest-churn way to "add Files to the round-trip cases" is to put
  `"files":["a.go","b.go"]` on concept A in `cleanMulti` and leave concept B WITHOUT a files key.
  This makes ONE table row exercise BOTH "files present" (A) AND "missing-files-key → nil" (B).
- Update the 4 cases reusing cleanMulti to expect `Files:[]string{"a.go","b.go"}` on concept A
  (concept B's Files stays nil — the zero value, so no change needed for B).
- ADD a dedicated `"files":null` case → `Files: nil`.
- UPDATE the comparison loop (currently checks `c.Title` + `c.Description` at lines 363/366) to ALSO
  check `c.Files` via `reflect.DeepEqual` (handles nil vs []string{} vs []string{"a"} correctly).

### TestParsePlannerOutput_RoundTrip
- ADD `Files: []string{"x", "y"}` to one commit in `original` (e.g. Commits[0]).
- REWRITE line 440 `out.Commits[i] != original.Commits[i]` → `!reflect.DeepEqual(out.Commits[i], original.Commits[i])`.
- ADD `"reflect"` to imports.
- The marshal→parse round-trip preserves Files: `[]string{"x","y"}` → `"files":["x","y"]` → `[]string{"x","y"}`;
  nil → `"files":null` → nil. reflect.DeepEqual passes for both.

## §5. json.Marshal/Unmarshal behavior for []string (verified reasoning)

- `json:"files"` has NO `omitempty` ⇒ a nil slice marshals to `"files":null` (NOT omitted); an empty
  `[]string{}` marshals to `"files":[]`. `json.Unmarshal` of `null` into `[]string` leaves it nil; of
  `[]` leaves `[]string{}` (non-nil empty); of `["x","y"]` gives `[]string{"x","y"}`.
- ⇒ the round-trip is exact for nil / empty / populated. reflect.DeepEqual distinguishes nil from
  `[]string{}`, which is the correct semantics for the test assertions.

## §6. Parallel-sibling coordination — ZERO overlap

The parallel work item P1.M1.T3.S2 touches ONLY `internal/decompose/chain_test.go` +
`internal/decompose/decompose_test.go` (arbiter freeze-parity tests). This task touches ONLY
`internal/prompt/planner.go` + `internal/prompt/planner_test.go`. No shared file. No coordination needed.

## §7. Confidence: 9/10

A one-field struct addition + two test-function updates, with the parse populating the field for free.
The only sharp edge is the `!=` → `reflect.DeepEqual` compile-fix in the round-trip test (§2), which is
explicitly flagged. No new types, no parse-code change, no interface change, no new imports in production
code (only `reflect` in the test file), no dependency change.
